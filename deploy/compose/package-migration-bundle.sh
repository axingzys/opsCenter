#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)
SOURCE_ENV_FILE="${ROOT_DIR}/.env"
ENV_TEMPLATE_FILE="${SCRIPT_DIR}/.env.example"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
SOURCE_NGINX_FILE="${ROOT_DIR}/nginx.conf"
SOURCE_PROMETHEUS_FILE="${ROOT_DIR}/deploy/prometheus/prometheus.yml"

BACKEND_CONTAINER="opshub-backend"
FRONTEND_CONTAINER="opshub-frontend"
MYSQL_CONTAINER="opshub-mysql"
REDIS_CONTAINER="opshub-redis"
GUACD_CONTAINER="opshub-guacd"
GUACAMOLE_CONTAINER="opshub-guacamole"
PROMETHEUS_CONTAINER="opshub-prometheus"
OUTPUT_FILE=""

log() {
  printf '[opshub-migration] %s\n' "$*"
}

die() {
  printf '[opshub-migration] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./deploy/compose/package-migration-bundle.sh [options]

Options:
  --output FILE         Output bundle archive, default: dist/opshub-migration-bundle-<timestamp>.tar.gz
  --source-env FILE     Source env file, default: ./.env
  --backend-container   Backend container name, default: opshub-backend
  --frontend-container  Frontend container name, default: opshub-frontend
  --mysql-container     MySQL container name, default: opshub-mysql
  --redis-container     Redis container name, default: opshub-redis
  --guacd-container     Guacd container name, default: opshub-guacd
  --guacamole-container Guacamole container name, default: opshub-guacamole
  --prometheus-container Prometheus container name, default: opshub-prometheus
  -h, --help            Show this help message

Examples:
  ./deploy/compose/package-migration-bundle.sh
  ./deploy/compose/package-migration-bundle.sh --output ./dist/opshub-prod-migration.tar.gz
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      [[ $# -ge 2 ]] || die "--output requires a value"
      OUTPUT_FILE="$2"
      shift 2
      ;;
    --source-env)
      [[ $# -ge 2 ]] || die "--source-env requires a value"
      SOURCE_ENV_FILE="$2"
      shift 2
      ;;
    --backend-container)
      [[ $# -ge 2 ]] || die "--backend-container requires a value"
      BACKEND_CONTAINER="$2"
      shift 2
      ;;
    --frontend-container)
      [[ $# -ge 2 ]] || die "--frontend-container requires a value"
      FRONTEND_CONTAINER="$2"
      shift 2
      ;;
    --mysql-container)
      [[ $# -ge 2 ]] || die "--mysql-container requires a value"
      MYSQL_CONTAINER="$2"
      shift 2
      ;;
    --redis-container)
      [[ $# -ge 2 ]] || die "--redis-container requires a value"
      REDIS_CONTAINER="$2"
      shift 2
      ;;
    --guacd-container)
      [[ $# -ge 2 ]] || die "--guacd-container requires a value"
      GUACD_CONTAINER="$2"
      shift 2
      ;;
    --guacamole-container)
      [[ $# -ge 2 ]] || die "--guacamole-container requires a value"
      GUACAMOLE_CONTAINER="$2"
      shift 2
      ;;
    --prometheus-container)
      [[ $# -ge 2 ]] || die "--prometheus-container requires a value"
      PROMETHEUS_CONTAINER="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "docker is not installed"
command -v gzip >/dev/null 2>&1 || die "gzip is not installed"
[[ -f "${ENV_TEMPLATE_FILE}" ]] || die "missing env template: ${ENV_TEMPLATE_FILE}"
[[ -f "${COMPOSE_FILE}" ]] || die "missing compose file: ${COMPOSE_FILE}"
[[ -f "${SOURCE_ENV_FILE}" ]] || die "source env file not found: ${SOURCE_ENV_FILE}"

container_exists() {
  local name="$1"
  docker container inspect "${name}" >/dev/null 2>&1
}

container_running() {
  local name="$1"
  [[ "$(docker inspect --format '{{.State.Running}}' "${name}" 2>/dev/null)" == "true" ]]
}

require_running_container() {
  local name="$1"
  container_exists "${name}" || die "container not found: ${name}"
  container_running "${name}" || die "container is not running: ${name}"
}

container_image() {
  local name="$1"
  docker inspect --format '{{.Config.Image}}' "${name}"
}

extract_env_from_file() {
  local key="$1"
  local value

  value=$(awk -F= -v key="${key}" '
    $0 ~ "^[[:space:]]*" key "=" {
      sub("^[^=]+=", "", $0)
      print $0
      exit
    }
  ' "${SOURCE_ENV_FILE}")

  if [[ "${value}" =~ ^\".*\"$ ]]; then
    value="${value:1:${#value}-2}"
  fi

  printf '%s\n' "${value}"
}

upsert_env() {
  local file="$1"
  local key="$2"
  local value="$3"
  local tmp_file

  tmp_file=$(mktemp)
  awk -v key="${key}" -v value="${value}" '
    BEGIN { updated = 0 }
    $0 ~ "^" key "=" {
      print key "=" value
      updated = 1
      next
    }
    { print }
    END {
      if (!updated) {
        print key "=" value
      }
    }
  ' "${file}" > "${tmp_file}"
  mv "${tmp_file}" "${file}"
}

copy_env_overrides() {
  local target_file="$1"
  local key
  local value

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ -n "${line}" ]] || continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" == *=* ]] || continue
    key=${line%%=*}
    value=${line#*=}
    upsert_env "${target_file}" "${key}" "${value}"
  done < "${SOURCE_ENV_FILE}"
}

parse_repo_and_tag() {
  local image="$1"
  local last_segment

  [[ "${image}" != *@* ]] || die "digest image is not supported: ${image}"
  last_segment="${image##*/}"
  [[ "${last_segment}" == *:* ]] || die "image must contain a tag: ${image}"

  printf '%s\n' "${image%:*}"
  printf '%s\n' "${image##*:}"
}

timestamp=$(date +%Y%m%d%H%M%S)
bundle_name="opshub-migration-bundle-${timestamp}"
if [[ -z "${OUTPUT_FILE}" ]]; then
  OUTPUT_FILE="${ROOT_DIR}/dist/${bundle_name}.tar.gz"
fi

require_running_container "${BACKEND_CONTAINER}"
require_running_container "${FRONTEND_CONTAINER}"
require_running_container "${MYSQL_CONTAINER}"
require_running_container "${REDIS_CONTAINER}"

WITH_DESKTOP=0
if container_running "${GUACD_CONTAINER}" && container_running "${GUACAMOLE_CONTAINER}"; then
  WITH_DESKTOP=1
fi

WITH_MONITORING=0
if container_running "${PROMETHEUS_CONTAINER}"; then
  WITH_MONITORING=1
fi

BACKEND_IMAGE=$(container_image "${BACKEND_CONTAINER}")
FRONTEND_IMAGE=$(container_image "${FRONTEND_CONTAINER}")
MYSQL_IMAGE=$(container_image "${MYSQL_CONTAINER}")
REDIS_IMAGE=$(container_image "${REDIS_CONTAINER}")

GUACD_IMAGE=""
GUACAMOLE_IMAGE=""
PROMETHEUS_IMAGE=""

if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  GUACD_IMAGE=$(container_image "${GUACD_CONTAINER}")
  GUACAMOLE_IMAGE=$(container_image "${GUACAMOLE_CONTAINER}")
fi

if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  PROMETHEUS_IMAGE=$(container_image "${PROMETHEUS_CONTAINER}")
fi

MYSQL_DATABASE=$(extract_env_from_file "MYSQL_DATABASE")
MYSQL_ROOT_PASSWORD=$(extract_env_from_file "MYSQL_ROOT_PASSWORD")

[[ -n "${MYSQL_DATABASE}" ]] || die "MYSQL_DATABASE is empty in ${SOURCE_ENV_FILE}"
[[ -n "${MYSQL_ROOT_PASSWORD}" ]] || die "MYSQL_ROOT_PASSWORD is empty in ${SOURCE_ENV_FILE}"

mapfile -t backend_parts < <(parse_repo_and_tag "${BACKEND_IMAGE}")
mapfile -t frontend_parts < <(parse_repo_and_tag "${FRONTEND_IMAGE}")
BACKEND_REPOSITORY="${backend_parts[0]}"
BACKEND_TAG="${backend_parts[1]}"
FRONTEND_REPOSITORY="${frontend_parts[0]}"
FRONTEND_TAG="${frontend_parts[1]}"

[[ "${BACKEND_TAG}" == "${FRONTEND_TAG}" ]] ||
  die "backend and frontend tags differ (${BACKEND_TAG} != ${FRONTEND_TAG}); align image tags before packaging"

bundle_dir=$(mktemp -d)
trap 'rm -rf "${bundle_dir:-}"' EXIT
package_root="${bundle_dir}/${bundle_name}"
compose_bundle_dir="${package_root}/deploy/compose"
image_archive="${package_root}/dist/opshub-migration-images.tar"
sql_dump_file="${compose_bundle_dir}/sql/runtime.sql.gz"
bundle_env_file="${compose_bundle_dir}/.env"
manifest_file="${compose_bundle_dir}/migration.env"

mkdir -p "${compose_bundle_dir}/prometheus" "${compose_bundle_dir}/sql" "${package_root}/dist"

cp "${COMPOSE_FILE}" "${compose_bundle_dir}/docker-compose.yml"
cp "${ENV_TEMPLATE_FILE}" "${compose_bundle_dir}/.env.example"
cp "${SCRIPT_DIR}/README.md" "${compose_bundle_dir}/README.md"
cp "${SCRIPT_DIR}/install-migration-bundle.sh" "${compose_bundle_dir}/install-migration-bundle.sh"

if [[ -f "${SOURCE_NGINX_FILE}" ]]; then
  cp "${SOURCE_NGINX_FILE}" "${compose_bundle_dir}/nginx.conf"
else
  cp "${SCRIPT_DIR}/nginx.conf" "${compose_bundle_dir}/nginx.conf"
fi

if [[ -f "${SOURCE_PROMETHEUS_FILE}" ]]; then
  cp "${SOURCE_PROMETHEUS_FILE}" "${compose_bundle_dir}/prometheus/prometheus.yml"
else
  cp "${SCRIPT_DIR}/prometheus/prometheus.yml" "${compose_bundle_dir}/prometheus/prometheus.yml"
fi

cp "${ENV_TEMPLATE_FILE}" "${bundle_env_file}"
copy_env_overrides "${bundle_env_file}"

upsert_env "${bundle_env_file}" "COMPOSE_PROJECT_NAME" "opshub"
upsert_env "${bundle_env_file}" "BACKEND_REPOSITORY" "${BACKEND_REPOSITORY}"
upsert_env "${bundle_env_file}" "FRONTEND_REPOSITORY" "${FRONTEND_REPOSITORY}"
upsert_env "${bundle_env_file}" "IMAGE_TAG" "${BACKEND_TAG}"
upsert_env "${bundle_env_file}" "MYSQL_IMAGE" "${MYSQL_IMAGE}"
upsert_env "${bundle_env_file}" "REDIS_IMAGE" "${REDIS_IMAGE}"

if [[ -n "${GUACD_IMAGE}" ]]; then
  upsert_env "${bundle_env_file}" "GUACD_IMAGE" "${GUACD_IMAGE}"
fi
if [[ -n "${GUACAMOLE_IMAGE}" ]]; then
  upsert_env "${bundle_env_file}" "GUACAMOLE_IMAGE" "${GUACAMOLE_IMAGE}"
fi
if [[ -n "${PROMETHEUS_IMAGE}" ]]; then
  upsert_env "${bundle_env_file}" "PROMETHEUS_IMAGE" "${PROMETHEUS_IMAGE}"
fi

cat > "${manifest_file}" <<EOF
WITH_DESKTOP=${WITH_DESKTOP}
WITH_MONITORING=${WITH_MONITORING}
IMAGE_ARCHIVE=../../dist/opshub-migration-images.tar
MYSQL_DUMP=./sql/runtime.sql.gz
BUNDLE_CREATED_AT=$(date -Iseconds)
EOF

log "dumping database ${MYSQL_DATABASE} from ${MYSQL_CONTAINER}"
docker exec "${MYSQL_CONTAINER}" mysqldump \
  --single-transaction \
  --set-gtid-purged=OFF \
  --default-character-set=utf8mb4 \
  --no-tablespaces \
  --routines \
  --events \
  --triggers \
  -uroot "-p${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE}" | gzip -c > "${sql_dump_file}"

images=(
  "${BACKEND_IMAGE}"
  "${FRONTEND_IMAGE}"
  "${MYSQL_IMAGE}"
  "${REDIS_IMAGE}"
)

if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  images+=("${GUACD_IMAGE}" "${GUACAMOLE_IMAGE}")
fi

if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  images+=("${PROMETHEUS_IMAGE}")
fi

log "saving image archive ${image_archive}"
docker save -o "${image_archive}" "${images[@]}"

mkdir -p "$(dirname "${OUTPUT_FILE}")"

case "${OUTPUT_FILE}" in
  *.tar.gz|*.tgz)
    tar -C "${bundle_dir}" -czf "${OUTPUT_FILE}" "${bundle_name}"
    ;;
  *.tar)
    tar -C "${bundle_dir}" -cf "${OUTPUT_FILE}" "${bundle_name}"
    ;;
  *)
    die "unsupported output format: ${OUTPUT_FILE} (use .tar, .tar.gz or .tgz)"
    ;;
esac

log "migration bundle created"
log "archive: ${OUTPUT_FILE}"
log "core images: ${BACKEND_IMAGE} ${FRONTEND_IMAGE} ${MYSQL_IMAGE} ${REDIS_IMAGE}"
if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  log "desktop images: ${GUACD_IMAGE} ${GUACAMOLE_IMAGE}"
fi
if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  log "monitoring image: ${PROMETHEUS_IMAGE}"
fi
log "target install command: ./deploy/compose/install-migration-bundle.sh --host <new-node-ip-or-domain>"
