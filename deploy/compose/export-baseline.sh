#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)
OUTPUT_DIR="${SCRIPT_DIR}/sql"
SCHEMA_FILE="${OUTPUT_DIR}/schema.sql"
SEED_FILE="${OUTPUT_DIR}/seed.sql"
BOOTSTRAP_FILE="${OUTPUT_DIR}/bootstrap.sql"

MYSQL_CONTAINER="opshub-baseline-mysql"
REDIS_CONTAINER="opshub-baseline-redis"
MYSQL_PASSWORD="123456"
MYSQL_DATABASE="opshub"
REDIS_PASSWORD="123456"
MYSQL_IMAGE="mysql:8.0"
REDIS_IMAGE="redis:7"
MYSQL_PORT=""
REDIS_PORT=""
BACKEND_PORT=""
TMP_CONFIG_FILE=""
TMP_LOG_FILE=""
SERVER_PID=""

log() {
  printf '[opshub-export] %s\n' "$*"
}

port_in_use() {
  local port="$1"
  ss -ltn "( sport = :${port} )" | tail -n +2 | grep -q .
}

find_free_port() {
  local start_port="$1"
  local end_port=$((start_port + 200))
  local port

  for ((port=start_port; port<=end_port; port++)); do
    if ! port_in_use "${port}"; then
      printf '%s\n' "${port}"
      return 0
    fi
  done

  return 1
}

cleanup() {
  if [[ -n "${SERVER_PID}" ]]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  rm -f "${TMP_CONFIG_FILE}" "${TMP_LOG_FILE}" >/dev/null 2>&1 || true
  docker rm -f "${MYSQL_CONTAINER}" "${REDIS_CONTAINER}" >/dev/null 2>&1 || true
}

trap cleanup EXIT

command -v docker >/dev/null 2>&1 || {
  printf '[opshub-export] ERROR: docker is not installed\n' >&2
  exit 1
}
command -v go >/dev/null 2>&1 || {
  printf '[opshub-export] ERROR: go is not installed\n' >&2
  exit 1
}
command -v curl >/dev/null 2>&1 || {
  printf '[opshub-export] ERROR: curl is not installed\n' >&2
  exit 1
}
command -v ss >/dev/null 2>&1 || {
  printf '[opshub-export] ERROR: ss is not installed\n' >&2
  exit 1
}

mkdir -p "${OUTPUT_DIR}"

cleanup

MYSQL_PORT=$(find_free_port 33306) || {
  printf '[opshub-export] ERROR: no free mysql port found\n' >&2
  exit 1
}
REDIS_PORT=$(find_free_port 36379) || {
  printf '[opshub-export] ERROR: no free redis port found\n' >&2
  exit 1
}
BACKEND_PORT=$(find_free_port 39876) || {
  printf '[opshub-export] ERROR: no free backend port found\n' >&2
  exit 1
}

log "starting temporary mysql"
docker run -d \
  --name "${MYSQL_CONTAINER}" \
  -p "${MYSQL_PORT}:3306" \
  -e MYSQL_ROOT_PASSWORD="${MYSQL_PASSWORD}" \
  -e MYSQL_DATABASE="${MYSQL_DATABASE}" \
  -e TZ=Asia/Shanghai \
  "${MYSQL_IMAGE}" \
  --default-authentication-plugin=mysql_native_password \
  --character-set-server=utf8mb4 \
  --collation-server=utf8mb4_unicode_ci >/dev/null

log "starting temporary redis"
docker run -d \
  --name "${REDIS_CONTAINER}" \
  -p "${REDIS_PORT}:6379" \
  -e REDIS_PASSWORD="${REDIS_PASSWORD}" \
  "${REDIS_IMAGE}" \
  sh -c 'exec redis-server --appendonly yes --requirepass "$REDIS_PASSWORD"' >/dev/null

for _ in $(seq 1 120); do
  if docker exec "${MYSQL_CONTAINER}" mysqladmin ping -h localhost -uroot "-p${MYSQL_PASSWORD}" --silent >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

docker exec "${MYSQL_CONTAINER}" mysqladmin ping -h localhost -uroot "-p${MYSQL_PASSWORD}" --silent >/dev/null

for _ in $(seq 1 60); do
  if docker exec "${REDIS_CONTAINER}" redis-cli -a "${REDIS_PASSWORD}" ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 2
done

docker exec "${REDIS_CONTAINER}" redis-cli -a "${REDIS_PASSWORD}" ping 2>/dev/null | grep -q PONG

log "importing migrations/init.sql"
docker exec -i "${MYSQL_CONTAINER}" mysql -uroot "-p${MYSQL_PASSWORD}" "${MYSQL_DATABASE}" < "${ROOT_DIR}/migrations/init.sql"

TMP_CONFIG_FILE=$(mktemp)
TMP_LOG_FILE=$(mktemp)

cat > "${TMP_CONFIG_FILE}" <<EOF
server:
  mode: release
  http_port: ${BACKEND_PORT}
  rpc_port: 9090
  read_timeout: 60000
  write_timeout: 60000
  jwt_secret: "baseline-export-secret"
  external_url: "http://127.0.0.1:${BACKEND_PORT}"
  frontend_url: "http://127.0.0.1"

database:
  driver: mysql
  host: 127.0.0.1
  port: ${MYSQL_PORT}
  database: ${MYSQL_DATABASE}
  username: root
  password: "${MYSQL_PASSWORD}"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  host: 127.0.0.1
  port: ${REDIS_PORT}
  password: "${REDIS_PASSWORD}"
  db: 0
  pool_size: 10
  min_idle_conn: 5

desktop:
  enabled: false
  provider: guacamole
  public_path: /guacamole/
  token_ttl_seconds: 30
  json_secret_key: "baseline-export-secret-key"
  recording_path: /tmp/guacamole/recordings
  transfer_root_path: /tmp/guacamole
  drive_name: OpsHub Files
  disable_upload: false
  disable_download: false

agent:
  enabled: true
  bundle_dir: ./agent-bundles
  default_install_path: /opt/opshub-agent
  default_listen_port: 19100
  binary_name: opshub-agent
  service_prefix: opshub-agent
  report_interval_seconds: 60

monitoring:
  prometheus:
    enabled: false
    base_url: http://127.0.0.1:19090
    file_sd_dir: ./data/prometheus/file_sd
    query_timeout_seconds: 10

log:
  level: info
  filename: logs/baseline-export.log
  max_size: 10
  max_backups: 3
  max_age: 7
  compress: false
  console: false
EOF

log "starting temporary backend with local go toolchain"
(cd "${ROOT_DIR}" && go run main.go --config "${TMP_CONFIG_FILE}" server >"${TMP_LOG_FILE}" 2>&1) &
SERVER_PID=$!

for _ in $(seq 1 120); do
  if curl -fsS "http://127.0.0.1:${BACKEND_PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if ! curl -fsS "http://127.0.0.1:${BACKEND_PORT}/health" >/dev/null; then
  cat "${TMP_LOG_FILE}" >&2
  exit 1
fi

log "dumping schema"
docker exec "${MYSQL_CONTAINER}" mysqldump \
  --single-transaction \
  --set-gtid-purged=OFF \
  --default-character-set=utf8mb4 \
  --no-tablespaces \
  --no-data \
  -uroot "-p${MYSQL_PASSWORD}" "${MYSQL_DATABASE}" > "${SCHEMA_FILE}"

log "dumping seed data"
docker exec "${MYSQL_CONTAINER}" mysqldump \
  --single-transaction \
  --set-gtid-purged=OFF \
  --default-character-set=utf8mb4 \
  --no-tablespaces \
  --no-create-info \
  --skip-triggers \
  -uroot "-p${MYSQL_PASSWORD}" "${MYSQL_DATABASE}" \
  sys_department \
  sys_role \
  sys_menu \
  sys_role_menu \
  sys_position \
  sys_user \
  sys_user_role \
  sys_user_position \
  sys_config \
  plugin_states > "${SEED_FILE}"

cat "${SCHEMA_FILE}" "${SEED_FILE}" > "${BOOTSTRAP_FILE}"

log "generated:"
log "  ${SCHEMA_FILE}"
log "  ${SEED_FILE}"
log "  ${BOOTSTRAP_FILE}"
