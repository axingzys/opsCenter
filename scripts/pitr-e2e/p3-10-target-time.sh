#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
RESULT_DIR="${SCRIPT_DIR}/results"

API_BASE="${OPSHUB_API_BASE:-http://127.0.0.1:9876}"
ADMIN_USER="${OPSHUB_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${OPSHUB_ADMIN_PASSWORD:-123456}"
MYSQL_CONTAINER="${OPSHUB_MYSQL_CONTAINER:-opshub-mysql}"
MYSQL_ROOT_PASSWORD="${OPSHUB_MYSQL_ROOT_PASSWORD:-123456}"
PITR_POSTGRES_PORT="${PITR_POSTGRES_PORT:-55432}"
PITR_SSH_PORT="${PITR_SSH_PORT:-2222}"
PITR_SSH_PASSWORD="${PITR_SSH_PASSWORD:-opshub}"
PITR_RESTORE_PORT="${PITR_RESTORE_PORT:-55433}"
PITR_POSTGRES_IMAGE="${PITR_POSTGRES_IMAGE:-postgres:16}"
PITR_BARMAN_GET_WAL="${PITR_BARMAN_GET_WAL:-false}"
PITR_RUN_ID="${PITR_RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
BARMAN_SERVER_NAME="${PITR_BARMAN_SERVER_NAME:-pg-main}"
RUNNER_WORK_DIR="${PITR_RUNNER_WORK_DIR:-/var/lib/opshub-pitr-e2e/runner}"

export PITR_POSTGRES_PORT PITR_SSH_PORT PITR_SSH_PASSWORD PITR_POSTGRES_IMAGE

mkdir -p "${RESULT_DIR}"
RESULT_JSON="${RESULT_DIR}/p3-10-target-time-${PITR_RUN_ID}.json"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >&2
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

api_raw() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local tmp http_code
  tmp="$(mktemp)"
  if [ -n "$body" ]; then
    http_code="$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" "${API_BASE}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN:-}" \
      --data "$body")"
  else
    http_code="$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" "${API_BASE}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN:-}")"
  fi
  if [ "$http_code" -lt 200 ] || [ "$http_code" -ge 300 ]; then
    cat "$tmp" >&2
    rm -f "$tmp"
    echo "HTTP ${http_code} for ${method} ${path}" >&2
    exit 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

api_data() {
  local raw
  raw="$(api_raw "$@")"
  local code message
  code="$(jq -r '.code // empty' <<<"$raw")"
  message="$(jq -r '.message // empty' <<<"$raw")"
  if [ "$code" != "0" ] && [ "$code" != "200" ]; then
    echo "$raw" >&2
    echo "API error: ${message:-unknown}" >&2
    exit 1
  fi
  jq -c '.data' <<<"$raw"
}

mysql_exec() {
  docker exec "$MYSQL_CONTAINER" mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" opshub -N -B -e "$1"
}

restore_captcha_on_exit() {
  if [ -n "${CAPTCHA_ORIGINAL+x}" ] && [ -n "${CAPTCHA_ORIGINAL}" ]; then
    mysql_exec "update sys_config set value='${CAPTCHA_ORIGINAL}' where \`key\`='enable_captcha';" >/dev/null 2>&1 || true
  fi
}

detect_backend_host() {
  if [ -n "${PITR_BACKEND_CONNECT_HOST:-}" ]; then
    printf '%s\n' "$PITR_BACKEND_CONNECT_HOST"
    return
  fi
  local gateway
  gateway="$(docker inspect opshub-backend --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{end}}' 2>/dev/null || true)"
  if [ -n "$gateway" ]; then
    printf '%s\n' "$gateway"
    return
  fi
  printf '127.0.0.1\n'
}

wait_runner_job() {
  local job_id="$1"
  local label="$2"
  local deadline=$((SECONDS + ${3:-900}))
  local item status error_message
  log "等待 Runner Job ${job_id} (${label}) 完成"
  while [ "$SECONDS" -lt "$deadline" ]; do
    item="$(api_data GET "/api/v1/databases/runner-jobs?page=1&pageSize=100" | jq -c --argjson id "$job_id" '.list[]? | select(.id == $id)' | head -n 1)"
    if [ -n "$item" ]; then
      status="$(jq -r '.status // ""' <<<"$item")"
      if [ "$status" = "success" ]; then
        log "Runner Job ${job_id} (${label}) 成功"
        printf '%s\n' "$item"
        return
      fi
      if [ "$status" = "failed" ] || [ "$status" = "cancelled" ]; then
        error_message="$(jq -r '.errorMessage // ""' <<<"$item")"
        echo "$item" | jq . >&2
        echo "Runner Job ${job_id} (${label}) failed: ${error_message}" >&2
        exit 1
      fi
    fi
    sleep 5
  done
  echo "Runner Job ${job_id} (${label}) timed out" >&2
  exit 1
}

wait_restore_job() {
  local job_id="$1"
  local deadline=$((SECONDS + ${2:-1200}))
  local item status message
  log "等待 Restore Job ${job_id} 完成"
  while [ "$SECONDS" -lt "$deadline" ]; do
    item="$(api_data GET "/api/v1/databases/restore-jobs/${job_id}")"
    status="$(jq -r '.status // ""' <<<"$item")"
    if [ "$status" = "success" ] || [ "$status" = "verified" ]; then
      log "Restore Job ${job_id} 成功，状态=${status}"
      printf '%s\n' "$item"
      return
    fi
    if [ "$status" = "failed" ] || [ "$status" = "cancelled" ]; then
      message="$(jq -r '.message // .errorMessage // ""' <<<"$item")"
      echo "$item" | jq . >&2
      echo "Restore Job ${job_id} failed: ${message}" >&2
      exit 1
    fi
    sleep 5
  done
  echo "Restore Job ${job_id} timed out" >&2
  exit 1
}

psql_source() {
  docker exec -e PGPASSWORD=opshub opshub-pitr-postgres psql -U opshub -d opshub_pitr -At -v ON_ERROR_STOP=1 -c "$1"
}

start_rehearsal_environment() {
  log "启动 PostgreSQL + Barman Runner 演练环境"
  if [ "${PITR_RESET_ENV:-true}" = "true" ]; then
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  if [ "${PITR_CLEANUP_RESTORE_CONTAINERS:-true}" = "true" ]; then
    docker ps -aq --filter "name=opshub-pg-restore-" | xargs -r docker rm -f >/dev/null 2>&1 || true
    docker ps -aq --filter "name=opshub-pgbase-restore-" | xargs -r docker rm -f >/dev/null 2>&1 || true
  fi
  if [ "${PITR_BUILD_RUNNER:-auto}" = "true" ] || { [ "${PITR_BUILD_RUNNER:-auto}" = "auto" ] && ! docker image inspect opshub-pitr-barman-runner:latest >/dev/null 2>&1; }; then
    docker compose -f "$COMPOSE_FILE" build barman-runner
  fi
  docker compose -f "$COMPOSE_FILE" up -d
  log "等待 PostgreSQL 就绪"
  for _ in $(seq 1 60); do
    if docker exec opshub-pitr-postgres pg_isready -U postgres -d postgres >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  docker exec opshub-pitr-postgres pg_isready -U postgres -d postgres >/dev/null
  log "初始化 Barman WAL streaming"
  docker exec opshub-pitr-barman-runner sh -lc 'barman receive-wal --create-slot --if-not-exists pg-main >/var/log/barman/receive-wal-create-slot.log 2>&1 || true'
  docker exec opshub-pitr-barman-runner sh -lc 'for pid in $(pgrep -f "[b]arman receive-wal pg-main" 2>/dev/null || true); do kill "$pid" 2>/dev/null || true; done'
  docker exec -d opshub-pitr-barman-runner sh -lc 'barman receive-wal pg-main >>/var/log/barman/receive-wal.log 2>&1'
  sleep 3
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 3
  docker exec opshub-pitr-barman-runner barman check "$BARMAN_SERVER_NAME" || true
}

login_opshub() {
  log "登录 OpsHub API"
  CAPTCHA_ORIGINAL="$(mysql_exec "select value from sys_config where \`key\`='enable_captcha' limit 1;" | tr -d '\r' || true)"
  trap restore_captcha_on_exit EXIT
  mysql_exec "update sys_config set value='false' where \`key\`='enable_captcha';" >/dev/null
  local raw
  raw="$(curl -sS -X POST "${API_BASE}/api/v1/public/login" \
    -H 'Content-Type: application/json' \
    --data "$(jq -cn --arg u "$ADMIN_USER" --arg p "$ADMIN_PASSWORD" '{username:$u,password:$p}')")"
  TOKEN="$(jq -r '.data.token // empty' <<<"$raw")"
  if [ -z "$TOKEN" ]; then
    echo "$raw" >&2
    echo "OpsHub login failed" >&2
    exit 1
  fi
}

create_opshub_resources() {
  local backend_host="$1"
  log "登记 OpsHub 凭据、实例、Runner、Barman Server"

  local pg_cred ssh_cred instance runner barman
  pg_cred="$(api_data POST /api/v1/credentials "$(jq -cn --arg n "p3-10-pg-${PITR_RUN_ID}" '{name:$n,protocol:"ssh",type:"password",username:"opshub",password:"opshub",description:"P3.10 e2e PostgreSQL credential"}')")"
  ssh_cred="$(api_data POST /api/v1/credentials "$(jq -cn --arg n "p3-10-runner-${PITR_RUN_ID}" --arg p "$PITR_SSH_PASSWORD" '{name:$n,protocol:"ssh",type:"password",username:"root",password:$p,description:"P3.10 e2e SSH Runner credential"}')")"
  PG_CREDENTIAL_ID="$(jq -r '.id' <<<"$pg_cred")"
  SSH_CREDENTIAL_ID="$(jq -r '.id' <<<"$ssh_cred")"

  instance="$(api_data POST /api/v1/databases/instances "$(jq -cn \
    --arg n "p3-10-pg-${PITR_RUN_ID}" \
    --arg h "$backend_host" \
    --argjson port "$PITR_POSTGRES_PORT" \
    --argjson credentialId "$PG_CREDENTIAL_ID" \
    '{name:$n,dbType:"postgresql",host:$h,port:$port,defaultDatabase:"opshub_pitr",credentialId:$credentialId,status:"enabled",environment:"test",businessSystem:"P3.10 PITR E2E",owner:"opshub"}')")"
  INSTANCE_ID="$(jq -r '.id' <<<"$instance")"
  api_data POST "/api/v1/databases/instances/${INSTANCE_ID}/test" >/dev/null

  runner="$(api_data POST /api/v1/databases/runner-hosts "$(jq -cn \
    --arg n "p3-10-barman-runner-${PITR_RUN_ID}" \
    --arg h "$backend_host" \
    --arg w "$RUNNER_WORK_DIR" \
    --argjson port "$PITR_SSH_PORT" \
    --argjson credentialId "$SSH_CREDENTIAL_ID" \
    '{name:$n,runnerType:"ssh",host:$h,port:$port,credentialId:$credentialId,workDir:$w,maxConcurrentJobs:1,timeoutMinutes:30,enabled:true}')")"
  RUNNER_ID="$(jq -r '.id' <<<"$runner")"
  api_data POST "/api/v1/databases/runner-hosts/${RUNNER_ID}/test" >/dev/null

  barman="$(api_data POST /api/v1/databases/barman-servers "$(jq -cn \
    --arg n "p3-10-barman-${PITR_RUN_ID}" \
    --arg server "$BARMAN_SERVER_NAME" \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    '{sourceInstanceId:$sourceInstanceId,runnerHostId:$runnerHostId,name:$n,barmanServerName:$server,barmanHome:"/var/lib/barman",configPath:"/etc/barman.conf",retentionPolicy:"REDUNDANCY 2",backupMethod:"postgres",streamingArchiverEnabled:true,archiverEnabled:false,slotName:"opshub_pitr_barman",status:"pending"}')")"
  BARMAN_ID="$(jq -r '.id' <<<"$barman")"

  local job
  job="$(api_data POST "/api/v1/databases/barman-servers/${BARMAN_ID}/check")"
  wait_runner_job "$(jq -r '.id' <<<"$job")" "barman-check" 300 >/dev/null
}

sync_barman_catalog_and_wal() {
  local job
  docker exec opshub-pitr-barman-runner barman archive-wal "$BARMAN_SERVER_NAME" >/dev/null 2>&1 || true
  job="$(api_data POST "/api/v1/databases/barman-servers/${BARMAN_ID}/sync-catalog")"
  wait_runner_job "$(jq -r '.id' <<<"$job")" "barman-catalog-sync" 300 >/dev/null
  docker exec opshub-pitr-barman-runner barman archive-wal "$BARMAN_SERVER_NAME" >/dev/null 2>&1 || true
  job="$(api_data POST "/api/v1/databases/barman-servers/${BARMAN_ID}/sync-wal")"
  wait_runner_job "$(jq -r '.id' <<<"$job")" "barman-wal-sync" 300 >/dev/null
}

run_target_time_rehearsal() {
  log "准备源库 marker 数据"
  psql_source "truncate table pitr_marker restart identity;" >/dev/null
  psql_source "insert into pitr_marker(label,payload) values ('before_backup','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null

  log "通过 OpsHub 触发 Barman backup"
  local job
  job="$(api_data POST "/api/v1/databases/barman-servers/${BARMAN_ID}/backup")"
  wait_runner_job "$(jq -r '.id' <<<"$job")" "barman-backup" 900 >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 5
  sync_barman_catalog_and_wal

  TARGET_TIME_RFC3339="$(docker exec -e PGPASSWORD=opshub opshub-pitr-postgres psql -U opshub -d opshub_pitr -At -v ON_ERROR_STOP=1 -c "select to_char(clock_timestamp(), 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"');")"
  log "目标恢复时间：${TARGET_TIME_RFC3339}"
  sleep 2
  psql_source "insert into pitr_marker(label,payload) values ('after_target','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 5
  sync_barman_catalog_and_wal

  log "生成 target time 恢复计划"
  local plan
  plan="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --arg target "$TARGET_TIME_RFC3339" \
    '{sourceInstanceId:$sourceInstanceId,restoreMode:"isolated_restore",restoreTargetType:"time",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  PLAN_ID="$(jq -r '.id' <<<"$plan")"
  PLAN_STATUS="$(jq -r '.validationStatus' <<<"$plan")"
  if [ "$PLAN_STATUS" != "passed" ]; then
    echo "$plan" | jq . >&2
    echo "restore plan validation did not pass: ${PLAN_STATUS}" >&2
    exit 1
  fi

  log "执行隔离 PostgreSQL target time 恢复"
  local restore
  restore="$(api_data POST "/api/v1/databases/restore-plans/${PLAN_ID}/run" "$(jq -cn \
    --argjson runnerHostId "$RUNNER_ID" \
    --arg image "$PITR_POSTGRES_IMAGE" \
    --argjson listenPort "$PITR_RESTORE_PORT" \
    --argjson barmanGetWal "$([ "$PITR_BARMAN_GET_WAL" = "true" ] && printf true || printf false)" \
    '{runnerHostId:$runnerHostId,containerImage:$image,listenPort:$listenPort,postgresStartInstance:true,targetAction:"pause",barmanGetWal:$barmanGetWal,cleanupOnFailure:false,validationAssertions:[
      {sql:"select count(*) from pitr_marker where label = '\''before_backup'\''",expectedScalar:"1"},
      {sql:"select count(*) from pitr_marker where label = '\''after_target'\''",expectedScalar:"0"},
      {sql:"select string_agg(label, '\'','\'' order by id) from pitr_marker",expectedContains:"before_backup"}
    ]}')")"
  RESTORE_JOB_ID="$(jq -r '.id' <<<"$restore")"
  RESTORE_RESULT="$(wait_restore_job "$RESTORE_JOB_ID" 1200)"

  RESTORED_LABELS="$(docker exec -e PGPASSWORD=opshub opshub-pitr-barman-runner psql -h 127.0.0.1 -p "$PITR_RESTORE_PORT" -U opshub -d opshub_pitr -At -v ON_ERROR_STOP=1 -c "select string_agg(label, ',' order by id) from pitr_marker;")"
  if [ "$RESTORED_LABELS" != "before_backup" ]; then
    echo "unexpected restored labels: ${RESTORED_LABELS}" >&2
    exit 1
  fi

  jq -n \
    --arg runId "$PITR_RUN_ID" \
    --arg targetTime "$TARGET_TIME_RFC3339" \
    --arg restoredLabels "$RESTORED_LABELS" \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    --argjson barmanServerId "$BARMAN_ID" \
    --argjson restorePlanId "$PLAN_ID" \
    --argjson restoreJobId "$RESTORE_JOB_ID" \
    --argjson restoreJob "$RESTORE_RESULT" \
    '{runId:$runId,targetTime:$targetTime,restoredLabels:$restoredLabels,instanceId:$instanceId,runnerHostId:$runnerHostId,barmanServerId:$barmanServerId,restorePlanId:$restorePlanId,restoreJobId:$restoreJobId,restoreJob:$restoreJob}' \
    > "$RESULT_JSON"
  log "P3.10 target time 恢复演练通过：${RESULT_JSON}"
}

main() {
  require_cmd docker
  require_cmd curl
  require_cmd jq

  cd "$REPO_ROOT"
  start_rehearsal_environment
  login_opshub
  BACKEND_CONNECT_HOST="$(detect_backend_host)"
  log "OpsHub backend 容器访问演练服务的地址：${BACKEND_CONNECT_HOST}"
  create_opshub_resources "$BACKEND_CONNECT_HOST"
  run_target_time_rehearsal
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
