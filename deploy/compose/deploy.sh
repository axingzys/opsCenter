#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
ENV_FILE="${SCRIPT_DIR}/.env"
SQL_FILE="${SCRIPT_DIR}/sql/bootstrap.sql"

WITH_DESKTOP=0
WITH_MONITORING=0
FORCE_REINIT_DB=0
SKIP_HEALTHCHECK=0
IMAGE_TAG_OVERRIDE=""

log() {
  printf '[opshub-deploy] %s\n' "$*"
}

die() {
  printf '[opshub-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./deploy/compose/deploy.sh [options]

Options:
  --with-desktop       Start guacd and guacamole
  --with-monitoring    Start prometheus
  --force-reinit-db    Drop and recreate the database before import
  --image-tag TAG      Override IMAGE_TAG from deploy/compose/.env
  --skip-healthcheck   Skip HTTP health checks after startup
  -h, --help           Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-desktop)
      WITH_DESKTOP=1
      shift
      ;;
    --with-monitoring)
      WITH_MONITORING=1
      shift
      ;;
    --force-reinit-db)
      FORCE_REINIT_DB=1
      shift
      ;;
    --image-tag)
      [[ $# -ge 2 ]] || die "--image-tag requires a value"
      IMAGE_TAG_OVERRIDE="$2"
      shift 2
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
docker compose version >/dev/null 2>&1 || die "docker compose is not available"
command -v curl >/dev/null 2>&1 || die "curl is not installed"

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${SCRIPT_DIR}/.env.example" "${ENV_FILE}"
  log "created ${ENV_FILE} from .env.example"
fi

[[ -f "${SQL_FILE}" ]] || die "missing SQL baseline: ${SQL_FILE}"

mkdir -p \
  "${SCRIPT_DIR}/runtime/logs" \
  "${SCRIPT_DIR}/runtime/terminal-recordings" \
  "${SCRIPT_DIR}/runtime/guacamole/recordings" \
  "${SCRIPT_DIR}/runtime/prometheus/file_sd"

set -a
# shellcheck source=/dev/null
source "${ENV_FILE}"
set +a

if [[ -n "${IMAGE_TAG_OVERRIDE}" ]]; then
  export IMAGE_TAG="${IMAGE_TAG_OVERRIDE}"
fi

if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  export OPSHUB_DESKTOP_ENABLED=true
fi

if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  export OPSHUB_MONITORING_PROMETHEUS_ENABLED=true
fi

COMPOSE_ARGS=(--env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  COMPOSE_ARGS=(--profile desktop "${COMPOSE_ARGS[@]}")
fi
if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  COMPOSE_ARGS=(--profile monitoring "${COMPOSE_ARGS[@]}")
fi

compose() {
  docker compose "${COMPOSE_ARGS[@]}" "$@"
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
  local attempts="${MYSQL_RETRY_ATTEMPTS:-20}"
  local attempt

  if [[ $# -ge 2 && "$1" == "--database" ]]; then
    database_args=("$2")
    shift 2
  fi

  for attempt in $(seq 1 "${attempts}"); do
    if compose exec -T mysql mysql --protocol=TCP -h127.0.0.1 -P3306 -uroot "-p${MYSQL_ROOT_PASSWORD}" "${database_args[@]}" "$@"; then
      return 0
    fi
    sleep 2
  done

  return 1
}

run_mysql_import() {
  local attempts="${MYSQL_RETRY_ATTEMPTS:-20}"
  local attempt

  for attempt in $(seq 1 "${attempts}"); do
    if compose exec -T mysql mysql --protocol=TCP -h127.0.0.1 -P3306 -uroot "-p${MYSQL_ROOT_PASSWORD}" "${MYSQL_DATABASE}" < "${SQL_FILE}"; then
      return 0
    fi
    sleep 2
  done

  return 1
}

create_database_if_needed() {
  run_mysql -e "CREATE DATABASE IF NOT EXISTS \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" ||
    die "failed to create database ${MYSQL_DATABASE}"
}

recreate_database() {
  run_mysql -e "DROP DATABASE IF EXISTS \`${MYSQL_DATABASE}\`; CREATE DATABASE \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" ||
    die "failed to recreate database ${MYSQL_DATABASE}"
}

database_initialized() {
  local count
  count=$(
    run_mysql -N -s -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${MYSQL_DATABASE}' AND table_name='sys_user';" |
      tr -d '\r'
  )
  [[ "${count}" == "1" ]]
}

import_database() {
  log "importing ${SQL_FILE}"
  run_mysql_import || die "failed to import ${SQL_FILE}"
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

log "starting mysql and redis"
compose up -d mysql redis
wait_for_container_health mysql 240
wait_for_container_health redis 120

if [[ "${FORCE_REINIT_DB}" -eq 1 ]]; then
  log "reinitializing database ${MYSQL_DATABASE}"
  recreate_database
  import_database
else
  create_database_if_needed
  if database_initialized; then
    log "database ${MYSQL_DATABASE} already initialized, skipping import"
  else
    import_database
  fi
fi

log "starting application services"
compose up -d

if [[ "${SKIP_HEALTHCHECK}" -eq 0 ]]; then
  wait_for_container_health backend 240
  wait_for_container_health frontend 120
  wait_for_http "$(backend_url)/health" 240
  wait_for_http "$(frontend_url)/" 120
fi

log "deployment completed"
log "frontend: $(frontend_url)"
log "backend:  $(backend_url)"
log "swagger:  $(backend_url)/swagger/index.html"
if [[ "${WITH_DESKTOP}" -eq 1 ]]; then
  log "guacamole: http://127.0.0.1:${GUACAMOLE_PORT}/guacamole/"
fi
if [[ "${WITH_MONITORING}" -eq 1 ]]; then
  log "prometheus: http://127.0.0.1:${PROMETHEUS_PORT}"
fi
log "default admin: admin / 123456"
log "compose file: ${COMPOSE_FILE}"
