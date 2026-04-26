#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
ENV_FILE="${SCRIPT_DIR}/.env"
MANIFEST_FILE="${SCRIPT_DIR}/migration.env"

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
SKIP_IMAGE_LOAD=0
SKIP_HEALTHCHECK=0

log() {
  printf '[opshub-migration] %s\n' "$*"
}

die() {
  printf '[opshub-migration] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./deploy/compose/install-migration-bundle.sh [options]

Options:
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
  --skip-image-load        Skip docker load step
  --skip-healthcheck       Skip HTTP health checks after startup
  -h, --help               Show this help message

Examples:
  ./deploy/compose/install-migration-bundle.sh --host 192.168.1.10
  ./deploy/compose/install-migration-bundle.sh --host opshub.example.com --scheme https
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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
    --skip-image-load)
      SKIP_IMAGE_LOAD=1
      shift
      ;;
    --skip-healthcheck)
      SKIP_HEALTHCHECK=1
      shift
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
if [[ "${SKIP_HEALTHCHECK}" -eq 0 ]]; then
  command -v curl >/dev/null 2>&1 || die "curl is not installed"
fi
docker compose version >/dev/null 2>&1 || die "docker compose is not available"

[[ -f "${COMPOSE_FILE}" ]] || die "missing compose file: ${COMPOSE_FILE}"
[[ -f "${ENV_FILE}" ]] || die "missing env file: ${ENV_FILE}"
[[ -f "${MANIFEST_FILE}" ]] || die "missing manifest file: ${MANIFEST_FILE}"

set -a
# shellcheck source=/dev/null
source "${ENV_FILE}"
# shellcheck source=/dev/null
source "${MANIFEST_FILE}"
set +a

SQL_FILE="${SCRIPT_DIR}/${MYSQL_DUMP#./}"
IMAGE_ARCHIVE_FILE="${SCRIPT_DIR}/${IMAGE_ARCHIVE}"

[[ -f "${SQL_FILE}" ]] || die "database dump not found: ${SQL_FILE}"
if [[ "${SKIP_IMAGE_LOAD}" -eq 0 ]]; then
  [[ -f "${IMAGE_ARCHIVE_FILE}" ]] || die "image archive not found: ${IMAGE_ARCHIVE_FILE}"
fi

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

build_public_url() {
  local scheme="$1"
  local host="$2"
  local port="$3"
  local default_port=""

  case "${scheme}" in
    http)
      default_port="80"
      ;;
    https)
      default_port="443"
      ;;
    *)
      die "unsupported scheme: ${scheme}"
      ;;
  esac

  if [[ "${port}" == "${default_port}" ]]; then
    printf '%s://%s\n' "${scheme}" "${host}"
  else
    printf '%s://%s:%s\n' "${scheme}" "${host}" "${port}"
  fi
}

compose_args=(--env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
if [[ "${WITH_DESKTOP:-0}" == "1" ]]; then
  compose_args=(--profile desktop "${compose_args[@]}")
fi
if [[ "${WITH_MONITORING:-0}" == "1" ]]; then
  compose_args=(--profile monitoring "${compose_args[@]}")
fi

compose() {
  docker compose "${compose_args[@]}" "$@"
}

container_id() {
  compose ps -q "$1"
}

wait_for_container_health() {
  local service="$1"
  local timeout_seconds="${2:-180}"
  local elapsed=0
  local container
  local status

  container=$(container_id "${service}")
  [[ -n "${container}" ]] || die "service ${service} is not running"

  while (( elapsed < timeout_seconds )); do
    status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}")
    case "${status}" in
      healthy|running)
        return 0
        ;;
      exited|dead)
        compose logs --no-color "${service}" || true
        die "service ${service} stopped unexpectedly"
        ;;
    esac
    sleep 2
    elapsed=$((elapsed + 2))
  done

  compose logs --no-color "${service}" || true
  die "service ${service} did not become healthy within ${timeout_seconds}s"
}

wait_for_http() {
  local url="$1"
  local timeout_seconds="${2:-180}"
  local elapsed=0

  while (( elapsed < timeout_seconds )); do
    if curl -fsS "${url}" >/dev/null; then
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done

  die "HTTP health check failed for ${url}"
}

run_mysql() {
  local database_args=()

  if [[ $# -ge 2 && "$1" == "--database" ]]; then
    database_args=("$2")
    shift 2
  fi

  compose exec -T mysql mysql --protocol=TCP -h127.0.0.1 -P3306 -uroot "-p${MYSQL_ROOT_PASSWORD}" "${database_args[@]}" "$@"
}

frontend_url() {
  if [[ "${FRONTEND_PORT:-80}" == "80" ]]; then
    printf 'http://127.0.0.1'
  else
    printf 'http://127.0.0.1:%s' "${FRONTEND_PORT}"
  fi
}

backend_url() {
  printf 'http://127.0.0.1:%s' "${BACKEND_PORT}"
}

mkdir -p \
  "${SCRIPT_DIR}/runtime/logs" \
  "${SCRIPT_DIR}/runtime/guacamole/recordings" \
  "${SCRIPT_DIR}/runtime/prometheus/file_sd"

if [[ -n "${FRONTEND_PORT_OVERRIDE}" ]]; then
  upsert_env "FRONTEND_PORT" "${FRONTEND_PORT_OVERRIDE}"
  FRONTEND_PORT="${FRONTEND_PORT_OVERRIDE}"
fi
if [[ -n "${BACKEND_PORT_OVERRIDE}" ]]; then
  upsert_env "BACKEND_PORT" "${BACKEND_PORT_OVERRIDE}"
  BACKEND_PORT="${BACKEND_PORT_OVERRIDE}"
fi
if [[ -n "${MYSQL_PORT_OVERRIDE}" ]]; then
  upsert_env "MYSQL_PORT" "${MYSQL_PORT_OVERRIDE}"
  MYSQL_PORT="${MYSQL_PORT_OVERRIDE}"
fi
if [[ -n "${REDIS_PORT_OVERRIDE}" ]]; then
  upsert_env "REDIS_PORT" "${REDIS_PORT_OVERRIDE}"
  REDIS_PORT="${REDIS_PORT_OVERRIDE}"
fi
if [[ -n "${GUACAMOLE_PORT_OVERRIDE}" ]]; then
  upsert_env "GUACAMOLE_PORT" "${GUACAMOLE_PORT_OVERRIDE}"
  GUACAMOLE_PORT="${GUACAMOLE_PORT_OVERRIDE}"
fi
if [[ -n "${PROMETHEUS_PORT_OVERRIDE}" ]]; then
  upsert_env "PROMETHEUS_PORT" "${PROMETHEUS_PORT_OVERRIDE}"
  PROMETHEUS_PORT="${PROMETHEUS_PORT_OVERRIDE}"
fi
if [[ -n "${MYSQL_ROOT_PASSWORD_OVERRIDE}" ]]; then
  upsert_env "MYSQL_ROOT_PASSWORD" "${MYSQL_ROOT_PASSWORD_OVERRIDE}"
  MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD_OVERRIDE}"
fi
if [[ -n "${REDIS_PASSWORD_OVERRIDE}" ]]; then
  upsert_env "REDIS_PASSWORD" "${REDIS_PASSWORD_OVERRIDE}"
  REDIS_PASSWORD="${REDIS_PASSWORD_OVERRIDE}"
fi

if [[ -n "${PUBLIC_HOST}" ]]; then
  upsert_env "OPSHUB_SERVER_EXTERNAL_URL" "$(build_public_url "${PUBLIC_SCHEME}" "${PUBLIC_HOST}" "${BACKEND_PORT}")"
  upsert_env "OPSHUB_SERVER_FRONTEND_URL" "$(build_public_url "${PUBLIC_SCHEME}" "${PUBLIC_HOST}" "${FRONTEND_PORT}")"
fi

if [[ "${SKIP_IMAGE_LOAD}" -eq 0 ]]; then
  log "loading image archive ${IMAGE_ARCHIVE_FILE}"
  docker load -i "${IMAGE_ARCHIVE_FILE}"
fi

log "stopping previous compose services in this bundle directory"
compose down --remove-orphans >/dev/null 2>&1 || true

log "starting mysql and redis"
compose up -d mysql redis
wait_for_container_health mysql 240
wait_for_container_health redis 120

log "recreating database ${MYSQL_DATABASE}"
run_mysql -e "DROP DATABASE IF EXISTS \`${MYSQL_DATABASE}\`; CREATE DATABASE \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

log "importing database dump ${SQL_FILE}"
gzip -dc "${SQL_FILE}" | compose exec -T mysql mysql --protocol=TCP -h127.0.0.1 -P3306 -uroot "-p${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE}"

log "starting application services"
compose up -d

if [[ "${SKIP_HEALTHCHECK}" -eq 0 ]]; then
  wait_for_container_health backend 240
  wait_for_container_health frontend 120
  wait_for_http "$(backend_url)/health" 60
  wait_for_http "$(frontend_url)" 60
fi

log "deployment completed"
log "frontend: $(frontend_url)"
log "backend: $(backend_url)"
