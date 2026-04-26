#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE_FILE="${SCRIPT_DIR}/.env.example"
DEPLOY_SCRIPT="${SCRIPT_DIR}/deploy.sh"

IMAGE_SOURCE=""
BACKEND_IMAGE="opshub-api:latest"
FRONTEND_IMAGE="opshub-web:latest"
PUBLIC_HOST=""
PUBLIC_SCHEME="http"
FRONTEND_PORT_OVERRIDE=""
BACKEND_PORT_OVERRIDE=""
MYSQL_PORT_OVERRIDE=""
REDIS_PORT_OVERRIDE=""
GUACAMOLE_PORT_OVERRIDE=""
PROMETHEUS_PORT_OVERRIDE=""
MYSQL_ROOT_PASSWORD_OVERRIDE=""
REDIS_PASSWORD_OVERRIDE=""
WITH_DESKTOP=0
WITH_MONITORING=0
STRICT_OFFLINE=0
DEPLOY_ARGS=()

log() {
  printf '[opshub-offline] %s\n' "$*"
}

die() {
  printf '[opshub-offline] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./deploy/compose/install-offline.sh [options] [deploy.sh options]

Options:
  --images PATH            Load image archive file or all archives in a directory
  --backend-image IMAGE    Backend image to use, default: opshub-api:latest
  --frontend-image IMAGE   Frontend image to use, default: opshub-web:latest
  --host HOST              Public host or IP written into deploy/compose/.env
  --scheme SCHEME          Public URL scheme, http or https, default: http
  --frontend-port PORT     Override FRONTEND_PORT in deploy/compose/.env
  --backend-port PORT      Override BACKEND_PORT in deploy/compose/.env
  --mysql-port PORT        Override MYSQL_PORT in deploy/compose/.env
  --redis-port PORT        Override REDIS_PORT in deploy/compose/.env
  --guacamole-port PORT    Override GUACAMOLE_PORT in deploy/compose/.env
  --prometheus-port PORT   Override PROMETHEUS_PORT in deploy/compose/.env
  --mysql-root-password P  Override MYSQL_ROOT_PASSWORD in deploy/compose/.env
  --redis-password P       Override REDIS_PASSWORD in deploy/compose/.env
  --strict-offline         Require dependency images to exist locally
  -h, --help               Show this help message

Examples:
  ./deploy/compose/install-offline.sh --images ./dist/opshub-images-latest.tar --host 192.168.1.10
  ./deploy/compose/install-offline.sh --images ./dist --with-monitoring
  ./deploy/compose/install-offline.sh \
    --images ./dist/opshub-images-v1.0.0.tar \
    --backend-image registry.example.com/opshub/opshub-api:v1.0.0 \
    --frontend-image registry.example.com/opshub/opshub-web:v1.0.0
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --images)
      [[ $# -ge 2 ]] || die "--images requires a value"
      IMAGE_SOURCE="$2"
      shift 2
      ;;
    --backend-image)
      [[ $# -ge 2 ]] || die "--backend-image requires a value"
      BACKEND_IMAGE="$2"
      shift 2
      ;;
    --frontend-image)
      [[ $# -ge 2 ]] || die "--frontend-image requires a value"
      FRONTEND_IMAGE="$2"
      shift 2
      ;;
    --host)
      [[ $# -ge 2 ]] || die "--host requires a value"
      PUBLIC_HOST="$2"
      shift 2
      ;;
    --scheme)
      [[ $# -ge 2 ]] || die "--scheme requires a value"
      PUBLIC_SCHEME="$2"
      shift 2
      ;;
    --frontend-port)
      [[ $# -ge 2 ]] || die "--frontend-port requires a value"
      FRONTEND_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --backend-port)
      [[ $# -ge 2 ]] || die "--backend-port requires a value"
      BACKEND_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --mysql-port)
      [[ $# -ge 2 ]] || die "--mysql-port requires a value"
      MYSQL_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --redis-port)
      [[ $# -ge 2 ]] || die "--redis-port requires a value"
      REDIS_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --guacamole-port)
      [[ $# -ge 2 ]] || die "--guacamole-port requires a value"
      GUACAMOLE_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --prometheus-port)
      [[ $# -ge 2 ]] || die "--prometheus-port requires a value"
      PROMETHEUS_PORT_OVERRIDE="$2"
      shift 2
      ;;
    --mysql-root-password)
      [[ $# -ge 2 ]] || die "--mysql-root-password requires a value"
      MYSQL_ROOT_PASSWORD_OVERRIDE="$2"
      shift 2
      ;;
    --redis-password)
      [[ $# -ge 2 ]] || die "--redis-password requires a value"
      REDIS_PASSWORD_OVERRIDE="$2"
      shift 2
      ;;
    --strict-offline)
      STRICT_OFFLINE=1
      shift
      ;;
    --with-desktop)
      WITH_DESKTOP=1
      DEPLOY_ARGS+=("$1")
      shift
      ;;
    --with-monitoring)
      WITH_MONITORING=1
      DEPLOY_ARGS+=("$1")
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      DEPLOY_ARGS+=("$1")
      shift
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "docker is not installed"
docker compose version >/dev/null 2>&1 || die "docker compose is not available"
[[ -x "${DEPLOY_SCRIPT}" ]] || die "missing deploy script: ${DEPLOY_SCRIPT}"

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE_FILE}" "${ENV_FILE}"
  log "created ${ENV_FILE} from .env.example"
fi

load_archive() {
  local archive="$1"
  [[ -f "${archive}" ]] || die "archive not found: ${archive}"

  case "${archive}" in
    *.tar)
      log "loading image archive ${archive}"
      docker load -i "${archive}"
      ;;
    *.tar.gz|*.tgz)
      log "loading image archive ${archive}"
      gzip -dc "${archive}" | docker load
      ;;
    *)
      die "unsupported archive format: ${archive}"
      ;;
  esac
}

load_archives_from_source() {
  local source="$1"
  local archives=()

  if [[ -z "${source}" ]]; then
    return 0
  fi

  if [[ -f "${source}" ]]; then
    load_archive "${source}"
    return 0
  fi

  if [[ -d "${source}" ]]; then
    mapfile -t archives < <(find "${source}" -maxdepth 1 -type f \( -name '*.tar' -o -name '*.tar.gz' -o -name '*.tgz' \) | sort)
    [[ ${#archives[@]} -gt 0 ]] || die "no image archives found in directory: ${source}"
    for archive in "${archives[@]}"; do
      load_archive "${archive}"
    done
    return 0
  fi

  die "image source does not exist: ${source}"
}

require_tagged_image() {
  local image="$1"
  if [[ "${image}" == *@* ]]; then
    die "image with digest is not supported here, use a tagged image: ${image}"
  fi
  if [[ "${image##*/}" != *:* ]]; then
    die "image must include an explicit tag: ${image}"
  fi
}

require_local_image() {
  local image="$1"
  docker image inspect "${image}" >/dev/null 2>&1 ||
    die "local image not found: ${image}"
}

local_image_exists() {
  local image="$1"
  docker image inspect "${image}" >/dev/null 2>&1
}

upsert_env() {
  local key="$1"
  local value="$2"
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
  ' "${ENV_FILE}" > "${tmp_file}"
  mv "${tmp_file}" "${ENV_FILE}"
}

set_image_env() {
  local image="$1"
  local repo_key="$2"
  local tag_key="$3"
  local repo="${image%:*}"
  local tag="${image##*:}"

  upsert_env "${repo_key}" "${repo}"
  upsert_env "${tag_key}" "${tag}"
}

require_tagged_image "${BACKEND_IMAGE}"
require_tagged_image "${FRONTEND_IMAGE}"

if [[ "${BACKEND_IMAGE##*:}" != "${FRONTEND_IMAGE##*:}" ]]; then
  die "backend and frontend images must use the same tag"
fi

load_archives_from_source "${IMAGE_SOURCE}"

require_local_image "${BACKEND_IMAGE}"
require_local_image "${FRONTEND_IMAGE}"

set_image_env "${BACKEND_IMAGE}" "BACKEND_REPOSITORY" "IMAGE_TAG"
set_image_env "${FRONTEND_IMAGE}" "FRONTEND_REPOSITORY" "IMAGE_TAG"

if [[ -n "${FRONTEND_PORT_OVERRIDE}" ]]; then
  upsert_env "FRONTEND_PORT" "${FRONTEND_PORT_OVERRIDE}"
fi
if [[ -n "${BACKEND_PORT_OVERRIDE}" ]]; then
  upsert_env "BACKEND_PORT" "${BACKEND_PORT_OVERRIDE}"
fi
if [[ -n "${MYSQL_PORT_OVERRIDE}" ]]; then
  upsert_env "MYSQL_PORT" "${MYSQL_PORT_OVERRIDE}"
fi
if [[ -n "${REDIS_PORT_OVERRIDE}" ]]; then
  upsert_env "REDIS_PORT" "${REDIS_PORT_OVERRIDE}"
fi
if [[ -n "${GUACAMOLE_PORT_OVERRIDE}" ]]; then
  upsert_env "GUACAMOLE_PORT" "${GUACAMOLE_PORT_OVERRIDE}"
fi
if [[ -n "${PROMETHEUS_PORT_OVERRIDE}" ]]; then
  upsert_env "PROMETHEUS_PORT" "${PROMETHEUS_PORT_OVERRIDE}"
fi
if [[ -n "${MYSQL_ROOT_PASSWORD_OVERRIDE}" ]]; then
  upsert_env "MYSQL_ROOT_PASSWORD" "${MYSQL_ROOT_PASSWORD_OVERRIDE}"
fi
if [[ -n "${REDIS_PASSWORD_OVERRIDE}" ]]; then
  upsert_env "REDIS_PASSWORD" "${REDIS_PASSWORD_OVERRIDE}"
fi

if [[ "${PUBLIC_SCHEME}" != "http" && "${PUBLIC_SCHEME}" != "https" ]]; then
  die "unsupported scheme: ${PUBLIC_SCHEME}"
fi

set -a
# shellcheck source=/dev/null
source "${ENV_FILE}"
set +a

build_url() {
  local scheme="$1"
  local host="$2"
  local port="$3"

  if [[ -z "${port}" ]]; then
    printf '%s://%s' "${scheme}" "${host}"
    return 0
  fi

  if [[ "${scheme}" == "http" && "${port}" == "80" ]]; then
    printf '%s://%s' "${scheme}" "${host}"
    return 0
  fi

  if [[ "${scheme}" == "https" && "${port}" == "443" ]]; then
    printf '%s://%s' "${scheme}" "${host}"
    return 0
  fi

  printf '%s://%s:%s' "${scheme}" "${host}" "${port}"
}

if [[ -n "${PUBLIC_HOST}" ]]; then
  upsert_env "OPSHUB_SERVER_FRONTEND_URL" "$(build_url "${PUBLIC_SCHEME}" "${PUBLIC_HOST}" "${FRONTEND_PORT}")"
  upsert_env "OPSHUB_SERVER_EXTERNAL_URL" "$(build_url "${PUBLIC_SCHEME}" "${PUBLIC_HOST}" "${BACKEND_PORT}")"

  set -a
  # shellcheck source=/dev/null
  source "${ENV_FILE}"
  set +a
fi

runtime_images=(
  "${MYSQL_IMAGE:-mysql:8.0.44}"
  "${REDIS_IMAGE:-redis:7-alpine}"
)

if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  runtime_images+=(
    "${GUACD_IMAGE:-guacamole/guacd:1.6.0}"
    "${GUACAMOLE_IMAGE:-guacamole/guacamole:1.6.0}"
  )
fi

if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  runtime_images+=("${PROMETHEUS_IMAGE:-prom/prometheus:v2.54.1}")
fi

missing_runtime_images=()
for image in "${runtime_images[@]}"; do
  if ! local_image_exists "${image}"; then
    missing_runtime_images+=("${image}")
  fi
done

if [[ ${#missing_runtime_images[@]} -gt 0 ]]; then
  if [[ "${STRICT_OFFLINE}" -eq 1 ]]; then
    die "missing local dependency images: ${missing_runtime_images[*]}"
  fi
  log "dependency images not found locally: ${missing_runtime_images[*]}"
  log "docker compose will try to pull them from registry during deployment"
fi

log "backend image: ${BACKEND_IMAGE}"
log "frontend image: ${FRONTEND_IMAGE}"
if [[ -n "${PUBLIC_HOST}" ]]; then
  log "frontend url: ${OPSHUB_SERVER_FRONTEND_URL}"
  log "backend url: ${OPSHUB_SERVER_EXTERNAL_URL}"
fi
log "starting deploy.sh ${DEPLOY_ARGS[*]:-}"

"${DEPLOY_SCRIPT}" "${DEPLOY_ARGS[@]}"
