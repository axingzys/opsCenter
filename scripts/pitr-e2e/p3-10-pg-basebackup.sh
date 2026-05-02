#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export PITR_POSTGRES_IMAGE="${PITR_POSTGRES_IMAGE:-registry.cn-guangzhou.aliyuncs.com/xingcangku/postgres:17}"
export PITR_BUILD_RUNNER="${PITR_BUILD_RUNNER:-true}"
export PITR_RESTORE_PORT="${PITR_RESTORE_PORT:-55435}"
PGBASE_INCREMENTAL_RESTORE_PORT="${PGBASE_INCREMENTAL_RESTORE_PORT:-55436}"

# Reuse the P3.10 environment/API helpers without running the target-time rehearsal.
# shellcheck source=./p3-10-target-time.sh
source "${SCRIPT_DIR}/p3-10-target-time.sh"

RESULT_JSON="${RESULT_DIR}/p3-10-pg-basebackup-${PITR_RUN_ID}.json"
WAL_STREAM_ID=""

api_allow_error() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS -X "$method" "${API_BASE}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN:-}" \
      --data "$body"
  else
    curl -sS -X "$method" "${API_BASE}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN:-}"
  fi
}

enable_pg17_incremental_if_available() {
  local current
  if ! current="$(docker exec opshub-pitr-postgres psql -U postgres -d postgres -At -v ON_ERROR_STOP=1 -c "show summarize_wal;" 2>/dev/null)"; then
    log "当前 PostgreSQL 不支持 summarize_wal，跳过原生 incremental 前置设置"
    return
  fi
  if [ "$current" = "on" ]; then
    return
  fi
  log "开启 PostgreSQL WAL summarizer 以支持 pg_basebackup incremental"
  docker exec opshub-pitr-postgres psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c "alter system set summarize_wal = on;" >/dev/null
  docker restart opshub-pitr-postgres >/dev/null
  for _ in $(seq 1 60); do
    if docker exec opshub-pitr-postgres pg_isready -U postgres -d postgres >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  docker exec opshub-pitr-postgres pg_isready -U postgres -d postgres >/dev/null
  docker exec opshub-pitr-barman-runner sh -lc 'for pid in $(pgrep -f "[b]arman receive-wal pg-main" 2>/dev/null || true); do kill "$pid" 2>/dev/null || true; done'
  docker exec opshub-pitr-barman-runner sh -lc 'barman receive-wal --create-slot --if-not-exists pg-main >/var/log/barman/receive-wal-create-slot.log 2>&1 || true'
  docker exec -d opshub-pitr-barman-runner sh -lc 'barman receive-wal pg-main >>/var/log/barman/receive-wal.log 2>&1'
  sleep 3
}

wait_backup_record() {
  local record_id="$1"
  local deadline=$((SECONDS + ${2:-900}))
  local item status message
  log "等待备份记录 ${record_id} 完成"
  while [ "$SECONDS" -lt "$deadline" ]; do
    item="$(api_data GET "/api/v1/databases/backup-records?instanceId=${INSTANCE_ID}&page=1&pageSize=200" | jq -c --argjson id "$record_id" '.list[]? | select(.id == $id)' | head -n 1)"
    if [ -n "$item" ]; then
      status="$(jq -r '.status // ""' <<<"$item")"
      if [ "$status" = "success" ]; then
        printf '%s\n' "$item"
        return
      fi
      if [ "$status" = "failed" ]; then
        message="$(jq -r '.message // .verifyMessage // ""' <<<"$item")"
        echo "$item" | jq . >&2
        echo "backup record ${record_id} failed: ${message}" >&2
        exit 1
      fi
    fi
    sleep 5
  done
  echo "backup record ${record_id} timed out" >&2
  exit 1
}

create_wal_stream() {
  local stream
  stream="$(api_data POST /api/v1/databases/log-archive-streams "$(jq -cn \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    '{instanceId:$instanceId,sourceInstanceId:$instanceId,engine:"postgresql",archiveType:"wal",archiveMode:"external",archiveEngine:"pg_basebackup_e2e",runnerHostId:$runnerHostId,retentionDays:7,enabled:true,configJson:"{\"purpose\":\"p3.10 pg_basebackup runner-readable WAL artifacts\"}"}')")"
  WAL_STREAM_ID="$(jq -r '.id' <<<"$stream")"
}

run_pg_basebackup_full() {
  local task run record_id record
  task="$(api_data POST /api/v1/databases/backup-tasks "$(jq -cn \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    --arg runId "$PITR_RUN_ID" \
    '{instanceId:$instanceId,name:("p3-10-pgbase-"+$runId),backupType:"physical",backupMethod:"physical",backupLevel:"full",backupEngine:"pg_basebackup",sourceRole:"primary",backupScope:"cluster",scopeConfig:({runnerHostId:$runnerHostId}|tojson),storageType:"local",retentionDays:7,maxDurationMinutes:30,compression:"gzip",enabled:false}')")"
  PGBASE_TASK_ID="$(jq -r '.id' <<<"$task")"
  run="$(api_data POST "/api/v1/databases/backup-tasks/${PGBASE_TASK_ID}/run")"
  record_id="$(jq -r '.recordId' <<<"$run")"
  record="$(wait_backup_record "$record_id" 900)"
  printf '%s\n' "$record"
}

lsn_from_bytes() {
  local value="$1"
  printf '%X/%X\n' $((value / 4294967296)) $((value % 4294967296))
}

register_runner_wal_archives() {
  local start_iso="$1"
  local target_iso="$2"
  local files count start_epoch target_epoch span index line name path size sha first_epoch last_epoch first_iso last_iso
  local timeline log_hex seg_hex log_dec seg_dec start_bytes end_bytes start_lsn end_lsn system_id

  psql_source "select pg_switch_wal();" >/dev/null
  for _ in $(seq 1 5); do
    docker exec opshub-pitr-barman-runner barman archive-wal "$BARMAN_SERVER_NAME" >/dev/null 2>&1 || true
    sleep 2
  done
  files="$(docker exec -i -e RUN_ID="$PITR_RUN_ID" opshub-pitr-barman-runner bash -se <<'EOS'
set -Eeuo pipefail
RAW_DIR="/var/lib/opshub-pitr-e2e/runner/wal-raw/${RUN_ID}"
rm -rf "$RAW_DIR"
mkdir -p "$RAW_DIR"
find /var/lib/barman/pg-main/wals /var/lib/barman/pg-main/streaming -type f -regextype posix-extended -regex ".*/[0-9A-F]{24}$" 2>/dev/null |
  awk -F/ '{name=$NF; if (!(name in seen) || $0 ~ /\/streaming\//) {seen[name]=1; path[name]=$0}} END {for (name in path) print path[name]}' |
  sort |
  while read -r p; do
    n="$(basename "$p")"
    raw="$RAW_DIR/$n"
    if gzip -t "$p" >/dev/null 2>&1; then
      gzip -dc "$p" > "$raw"
    else
      cp "$p" "$raw"
    fi
    chmod a+r "$raw"
    s="$(wc -c < "$raw" | tr -d " ")"
    h="$(sha256sum "$raw" | cut -d " " -f 1)"
    printf "%s\t%s\t%s\t%s\n" "$n" "$raw" "$s" "$h"
  done
EOS
)"
  count="$(grep -c . <<<"$files" || true)"
  if [ "$count" -le 0 ]; then
    echo "no complete Barman WAL files found" >&2
    docker exec opshub-pitr-barman-runner sh -lc 'find /var/lib/barman/pg-main -maxdepth 4 -type f | sort' >&2 || true
    exit 1
  fi
  start_epoch="$(date -d "$start_iso" '+%s')"
  target_epoch="$(date -d "$target_iso" '+%s')"
  if [ "$target_epoch" -le "$start_epoch" ]; then
    target_epoch=$((start_epoch + count + 2))
  fi
  span=$((target_epoch - start_epoch))
  if [ "$span" -lt "$count" ]; then
    span=$((count + 2))
  fi
  system_id="$(psql_source "select system_identifier from pg_control_system();")"

  index=0
  while IFS=$'\t' read -r name path size sha; do
    [ -n "$name" ] || continue
    first_epoch=$((start_epoch + (index * span / count)))
    last_epoch=$((start_epoch + ((index + 1) * span / count)))
    if [ "$index" -eq $((count - 1)) ]; then
      last_epoch=$((target_epoch + 5))
    fi
    if [ "$last_epoch" -le "$first_epoch" ]; then
      last_epoch=$((first_epoch + 1))
    fi
    first_iso="$(TZ=Asia/Shanghai date -d "@$first_epoch" '+%Y-%m-%d %H:%M:%S')"
    last_iso="$(TZ=Asia/Shanghai date -d "@$last_epoch" '+%Y-%m-%d %H:%M:%S')"
    timeline="${name:0:8}"
    log_hex="${name:8:8}"
    seg_hex="${name:16:8}"
    log_dec=$((16#$log_hex))
    seg_dec=$((16#$seg_hex))
    start_bytes=$((log_dec * 4294967296 + seg_dec * 16777216))
    end_bytes=$((start_bytes + 16777216))
    start_lsn="$(lsn_from_bytes "$start_bytes")"
    end_lsn="$(lsn_from_bytes "$end_bytes")"
    api_data POST /api/v1/databases/log-archives/external "$(jq -cn \
      --argjson streamId "$WAL_STREAM_ID" \
      --arg fileName "$name" \
      --arg storageUri "runner://runner-host-${RUNNER_ID}${path}" \
      --argjson fileSize "$size" \
      --arg checksum "$sha" \
      --arg first "$first_iso" \
      --arg last "$last_iso" \
      --arg systemId "$system_id" \
      --arg timeline "$timeline" \
      --arg startLsn "$start_lsn" \
      --arg endLsn "$end_lsn" \
      --arg segmentNo "$name" \
      '{streamId:$streamId,fileName:$fileName,storageUri:$storageUri,fileSize:$fileSize,checksumSha256:$checksum,firstEventTime:$first,lastEventTime:$last,status:"archived",pgSystemIdentifier:$systemId,timelineId:$timeline,startLsn:$startLsn,endLsn:$endLsn,segmentNo:$segmentNo}')" >/dev/null
    index=$((index + 1))
  done <<<"$files"
}

run_pgbase_restore() {
  local plan_id="$1"
  local listen_port="$2"
  local expected_labels="$3"
  local restore job_id result labels
  restore="$(api_data POST "/api/v1/databases/restore-plans/${plan_id}/run" "$(jq -cn \
    --argjson runnerHostId "$RUNNER_ID" \
    --arg image "$PITR_POSTGRES_IMAGE" \
    --argjson listenPort "$listen_port" \
    '{runnerHostId:$runnerHostId,containerImage:$image,listenPort:$listenPort,postgresStartInstance:true,targetAction:"pause",cleanupOnFailure:false,validationAssertions:[
      {sql:"select count(*) from pitr_marker",expectedRows:1},
      {sql:"select string_agg(label, '\'','\'' order by id) from pitr_marker",expectedContains:"before_full"}
    ]}')")"
  job_id="$(jq -r '.id' <<<"$restore")"
  result="$(wait_restore_job "$job_id" 1200)"
  labels="$(docker exec -e PGPASSWORD=opshub opshub-pitr-barman-runner psql -h 127.0.0.1 -p "$listen_port" -U opshub -d opshub_pitr -At -v ON_ERROR_STOP=1 -c "select string_agg(label, ',' order by id) from pitr_marker;")"
  if [ "$labels" != "$expected_labels" ]; then
    echo "unexpected pg_basebackup restored labels: ${labels}; expected ${expected_labels}" >&2
    exit 1
  fi
  printf '%s\n' "$result"
}

runner_path_from_uri() {
  sed -E 's#^runner://runner-host-[0-9]+##' <<<"$1"
}

create_pgbase_incremental_artifact() {
  local base_uri="$1"
  local base_path
  base_path="$(runner_path_from_uri "$base_uri")"
  docker exec -i \
    -e BASE_ARTIFACT="$base_path" \
    -e RUN_ID="$PITR_RUN_ID" \
    -e PGPORT_VALUE="$PITR_POSTGRES_PORT" \
    opshub-pitr-barman-runner bash -se <<'EOS'
set -Eeuo pipefail
WORK_DIR="/var/lib/opshub-pitr-e2e/runner/pg-basebackup-incremental/${RUN_ID}"
BASE_EXTRACT="$WORK_DIR/base"
INC_DIR="$WORK_DIR/incremental"
INC_TAR="$WORK_DIR/pg-basebackup-incremental-${RUN_ID}.tar.gz"
LOG_FILE="$WORK_DIR/incremental.log"
rm -rf "$WORK_DIR"
mkdir -p "$BASE_EXTRACT" "$INC_DIR"
tar -xzf "$BASE_ARTIFACT" -C "$BASE_EXTRACT" >"$LOG_FILE" 2>&1
test -f "$BASE_EXTRACT/backup_manifest"
PGPASSWORD=opshub pg_basebackup -h 127.0.0.1 -p "$PGPORT_VALUE" -U opshub -D "$INC_DIR" -Fp -X stream --checkpoint=fast --incremental="$BASE_EXTRACT/backup_manifest" >>"$LOG_FILE" 2>&1
tar -czf "$INC_TAR" -C "$INC_DIR" . >>"$LOG_FILE" 2>&1
size="$(wc -c < "$INC_TAR" | tr -d ' ')"
sha="$(sha256sum "$INC_TAR" | awk '{print $1}')"
manifest_sha=""; if [ -f "$INC_DIR/backup_manifest" ]; then manifest_sha="$(sha256sum "$INC_DIR/backup_manifest" | awk '{print $1}')"; fi
pg_version=""; if [ -f "$INC_DIR/PG_VERSION" ]; then pg_version="$(cat "$INC_DIR/PG_VERSION" | tr -d '\r\n')"; fi
system_id=""; timeline_id=""; redo_lsn=""; redo_wal=""
if command -v pg_controldata >/dev/null 2>&1; then
  pg_controldata "$INC_DIR" > "$WORK_DIR/pg_controldata.txt" 2>> "$LOG_FILE" || true
  system_id="$(awk -F: '/Database system identifier/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"
  timeline_id="$(awk -F: '/Latest checkpoint.s TimeLineID/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"
  redo_lsn="$(awk -F: '/Latest checkpoint.s REDO location/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"
  redo_wal="$(awk -F: '/Latest checkpoint.s REDO WAL file/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"
fi
jq -n \
  --arg filePath "$INC_TAR" \
  --arg fileName "$(basename "$INC_TAR")" \
  --argjson fileSize "$size" \
  --arg checksumSha256 "$sha" \
  --arg backupManifestChecksum "$manifest_sha" \
  --arg pgVersion "$pg_version" \
  --arg pgSystemIdentifier "$system_id" \
  --arg timelineId "$timeline_id" \
  --arg endLsn "$redo_lsn" \
  --arg walEnd "$redo_wal" \
  --arg startedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --arg finishedAt "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  '{filePath:$filePath,fileName:$fileName,fileSize:$fileSize,checksumSha256:$checksumSha256,backupManifestChecksum:$backupManifestChecksum,pgVersion:$pgVersion,pgSystemIdentifier:$pgSystemIdentifier,timelineId:$timelineId,endLsn:$endLsn,walEnd:$walEnd,startedAt:$startedAt,finishedAt:$finishedAt}'
EOS
}

register_pgbase_incremental_record() {
  local full_record="$1"
  local inc_meta="$2"
  local record
  record="$(api_data POST /api/v1/databases/backup-records/external "$(jq -cn \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson baseRecordId "$(jq -r '.id' <<<"$full_record")" \
    --argjson parentRecordId "$(jq -r '.id' <<<"$full_record")" \
    --arg chainId "$(jq -r '.chainId' <<<"$full_record")" \
    --arg externalServerName "$(jq -r '.externalServerName // ""' <<<"$full_record")" \
    --arg storageUri "runner://runner-host-${RUNNER_ID}$(jq -r '.filePath' <<<"$inc_meta")" \
    --arg fileName "$(jq -r '.fileName' <<<"$inc_meta")" \
    --argjson fileSize "$(jq -r '.fileSize' <<<"$inc_meta")" \
    --arg checksum "$(jq -r '.checksumSha256' <<<"$inc_meta")" \
    --arg startedAt "$(jq -r '.startedAt' <<<"$inc_meta")" \
    --arg finishedAt "$(jq -r '.finishedAt' <<<"$inc_meta")" \
    --arg recoverableFrom "$(jq -r '.recoverableFrom // .startedAt // ""' <<<"$full_record")" \
    --arg recoverableUntil "$(jq -r '.finishedAt' <<<"$inc_meta")" \
    --arg pgSystemIdentifier "$(jq -r '.pgSystemIdentifier' <<<"$inc_meta")" \
    --arg timelineId "$(jq -r '.timelineId' <<<"$inc_meta")" \
    --arg endLsn "$(jq -r '.endLsn' <<<"$inc_meta")" \
    --arg walEnd "$(jq -r '.walEnd' <<<"$inc_meta")" \
    --arg manifestChecksum "$(jq -r '.backupManifestChecksum' <<<"$inc_meta")" \
    --arg pgVersion "$(jq -r '.pgVersion' <<<"$inc_meta")" \
    --arg runId "$PITR_RUN_ID" \
    '{instanceId:$instanceId,sourceInstanceId:$instanceId,chainId:$chainId,baseRecordId:$baseRecordId,parentRecordId:$parentRecordId,backupMethod:"physical",backupLevel:"incremental",backupEngine:"pg_basebackup",externalBackupId:("pgbase-inc-"+$runId),externalServerName:$externalServerName,backupScope:"cluster",toolName:"pg_basebackup",toolVersion:$pgVersion,storageUri:$storageUri,manifestJson:({engine:"pg_basebackup",level:"incremental",requires:"pg_combinebackup",baseRecordId:$baseRecordId,parentRecordId:$parentRecordId}|tojson),prepareStatus:"ready",fileName:$fileName,fileSize:$fileSize,checksumSha256:$checksum,compression:"gzip",recoverableFrom:$recoverableFrom,recoverableUntil:$recoverableUntil,startedAt:$startedAt,finishedAt:$finishedAt,status:"success",verifyStatus:"success",pgSystemIdentifier:$pgSystemIdentifier,timelineId:$timelineId,endLsn:$endLsn,walEnd:$walEnd,backupManifestChecksum:$manifestChecksum}')")"
  printf '%s\n' "$record"
}

expect_bad_artifact_gate() {
  local target_time="$1"
  local full_record="$2"
  local bad_record bad_plan raw code message
  bad_record="$(api_data POST /api/v1/databases/backup-records/external "$(jq -cn \
    --argjson instanceId "$INSTANCE_ID" \
    --arg chainId "pgbase-bad-${PITR_RUN_ID}" \
    --arg storageUri "runner://runner-host-${RUNNER_ID}/tmp/opshub-missing-pgbase-${PITR_RUN_ID}.tar.gz" \
    --arg startedAt "$(jq -r '.startedAt' <<<"$full_record")" \
    --arg finishedAt "$(jq -r '.finishedAt' <<<"$full_record")" \
    --arg recoverableFrom "$(jq -r '.recoverableFrom // .startedAt // ""' <<<"$full_record")" \
    --arg recoverableUntil "$(jq -r '.recoverableUntil // .finishedAt // ""' <<<"$full_record")" \
    --arg pgSystemIdentifier "$(jq -r '.pgSystemIdentifier // ""' <<<"$full_record")" \
    --arg timelineId "$(jq -r '.timelineId // ""' <<<"$full_record")" \
    --arg endLsn "$(jq -r '.endLsn // ""' <<<"$full_record")" \
    --arg walEnd "$(jq -r '.walEnd // ""' <<<"$full_record")" \
    '{instanceId:$instanceId,sourceInstanceId:$instanceId,chainId:$chainId,backupMethod:"physical",backupLevel:"full",backupEngine:"pg_basebackup",externalBackupId:$chainId,externalServerName:"p3.10-bad-artifact",backupScope:"cluster",toolName:"pg_basebackup",storageUri:$storageUri,fileName:"missing-pgbase.tar.gz",fileSize:1,compression:"gzip",recoverableFrom:$recoverableFrom,recoverableUntil:$recoverableUntil,startedAt:$startedAt,finishedAt:$finishedAt,status:"success",verifyStatus:"success",pgSystemIdentifier:$pgSystemIdentifier,timelineId:$timelineId,endLsn:$endLsn,walEnd:$walEnd,backupManifestChecksum:"metadata-only"}')")"
  bad_plan="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --argjson baseRecordId "$(jq -r '.id' <<<"$bad_record")" \
    --arg target "$target_time" \
    '{sourceInstanceId:$sourceInstanceId,baseRecordId:$baseRecordId,restoreMode:"isolated_restore",restoreTargetType:"time",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  if [ "$(jq -r '.validationStatus' <<<"$bad_plan")" != "passed" ]; then
    echo "$bad_plan" | jq . >&2
    echo "bad artifact plan should pass metadata validation so run gate can reject unreadable file" >&2
    exit 1
  fi
  raw="$(api_allow_error POST "/api/v1/databases/restore-plans/$(jq -r '.id' <<<"$bad_plan")/run" "$(jq -cn \
    --argjson runnerHostId "$RUNNER_ID" \
    --arg image "$PITR_POSTGRES_IMAGE" \
    --argjson listenPort 55439 \
    '{runnerHostId:$runnerHostId,containerImage:$image,listenPort:$listenPort,postgresStartInstance:false,cleanupOnFailure:true}')")"
  code="$(jq -r '.code // empty' <<<"$raw")"
  message="$(jq -r '.message // ""' <<<"$raw")"
  if [ "$code" = "0" ] || [ "$code" = "200" ] || ! grep -qi "artifact" <<<"$message"; then
    echo "$raw" | jq . >&2
    echo "expected unreadable artifact gate to reject restore run" >&2
    exit 1
  fi
  jq -n --argjson record "$bad_record" --argjson plan "$bad_plan" --arg message "$message" '{record:$record,plan:$plan,message:$message}'
}

run_pgbase_rehearsal() {
  log "准备 pg_basebackup full/incremental 演练数据"
  create_wal_stream
  psql_source "truncate table pitr_marker restart identity;" >/dev/null
  psql_source "insert into pitr_marker(label,payload) values ('before_full','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 3

  FULL_RECORD="$(run_pg_basebackup_full)"
  psql_source "insert into pitr_marker(label,payload) values ('full_target','${PITR_RUN_ID}');" >/dev/null
  FULL_TARGET_TIME_RFC3339="$(psql_source "select to_char(max(created_at) + interval '1 second', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') from pitr_marker where label = 'full_target';")"
  sleep 2
  psql_source "insert into pitr_marker(label,payload) values ('after_full_target','${PITR_RUN_ID}');" >/dev/null
  register_runner_wal_archives "$(jq -r '.recoverableUntil // .finishedAt' <<<"$FULL_RECORD")" "$FULL_TARGET_TIME_RFC3339"

  log "生成 pg_basebackup full + WAL target time 恢复计划"
  FULL_PLAN="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --argjson baseRecordId "$(jq -r '.id' <<<"$FULL_RECORD")" \
    --arg target "$FULL_TARGET_TIME_RFC3339" \
    '{sourceInstanceId:$sourceInstanceId,baseRecordId:$baseRecordId,restoreMode:"isolated_restore",restoreTargetType:"time",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  if [ "$(jq -r '.validationStatus' <<<"$FULL_PLAN")" != "passed" ]; then
    echo "$FULL_PLAN" | jq . >&2
    echo "pg_basebackup full restore plan did not pass" >&2
    exit 1
  fi
  FULL_RESTORE="$(run_pgbase_restore "$(jq -r '.id' <<<"$FULL_PLAN")" "$PITR_RESTORE_PORT" "before_full,full_target")"

  BAD_ARTIFACT_GATE="$(expect_bad_artifact_gate "$FULL_TARGET_TIME_RFC3339" "$FULL_RECORD")"

  log "生成 pg_basebackup incremental artifact"
  psql_source "insert into pitr_marker(label,payload) values ('before_incremental','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 3
  INC_META="$(create_pgbase_incremental_artifact "$(jq -r '.storageUri' <<<"$FULL_RECORD")")"
  INC_RECORD="$(register_pgbase_incremental_record "$FULL_RECORD" "$INC_META")"

  psql_source "insert into pitr_marker(label,payload) values ('incremental_target','${PITR_RUN_ID}');" >/dev/null
  INCREMENTAL_TARGET_TIME_RFC3339="$(psql_source "select to_char(max(created_at) + interval '1 second', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') from pitr_marker where label = 'incremental_target';")"
  sleep 2
  psql_source "insert into pitr_marker(label,payload) values ('after_incremental_target','${PITR_RUN_ID}');" >/dev/null
  register_runner_wal_archives "$FULL_TARGET_TIME_RFC3339" "$INCREMENTAL_TARGET_TIME_RFC3339"

  log "生成 pg_basebackup incremental + pg_combinebackup + WAL 恢复计划"
  INC_PLAN="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --argjson baseRecordId "$(jq -r '.id' <<<"$FULL_RECORD")" \
    --arg target "$INCREMENTAL_TARGET_TIME_RFC3339" \
    '{sourceInstanceId:$sourceInstanceId,baseRecordId:$baseRecordId,restoreMode:"isolated_restore",restoreTargetType:"time",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  if [ "$(jq -r '.validationStatus' <<<"$INC_PLAN")" != "passed" ]; then
    echo "$INC_PLAN" | jq . >&2
    echo "pg_basebackup incremental restore plan did not pass" >&2
    exit 1
  fi
  if ! jq -er '.planJson | fromjson | .restoreSteps[] | select(test("pg_combinebackup"))' <<<"$INC_PLAN" >/dev/null; then
    echo "$INC_PLAN" | jq . >&2
    echo "incremental plan does not include pg_combinebackup step" >&2
    exit 1
  fi
  INC_RESTORE="$(run_pgbase_restore "$(jq -r '.id' <<<"$INC_PLAN")" "$PGBASE_INCREMENTAL_RESTORE_PORT" "before_full,full_target,after_full_target,before_incremental,incremental_target")"
  if ! jq -er '.proofJson | fromjson | .combineBackupStatus == "success"' <<<"$INC_RESTORE" >/dev/null; then
    echo "$INC_RESTORE" | jq . >&2
    echo "incremental restore proof does not show pg_combinebackup success" >&2
    exit 1
  fi

  jq -n \
    --arg runId "$PITR_RUN_ID" \
    --arg fullTargetTime "$FULL_TARGET_TIME_RFC3339" \
    --arg incrementalTargetTime "$INCREMENTAL_TARGET_TIME_RFC3339" \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    --argjson walStreamId "$WAL_STREAM_ID" \
    --argjson fullRecord "$FULL_RECORD" \
    --argjson fullPlan "$FULL_PLAN" \
    --argjson fullRestore "$FULL_RESTORE" \
    --argjson badArtifactGate "$BAD_ARTIFACT_GATE" \
    --argjson incrementalMeta "$INC_META" \
    --argjson incrementalRecord "$INC_RECORD" \
    --argjson incrementalPlan "$INC_PLAN" \
    --argjson incrementalRestore "$INC_RESTORE" \
    '{runId:$runId,fullTargetTime:$fullTargetTime,incrementalTargetTime:$incrementalTargetTime,instanceId:$instanceId,runnerHostId:$runnerHostId,walStreamId:$walStreamId,fullRecord:$fullRecord,fullPlan:$fullPlan,fullRestore:$fullRestore,badArtifactGate:$badArtifactGate,incrementalMeta:$incrementalMeta,incrementalRecord:$incrementalRecord,incrementalPlan:$incrementalPlan,incrementalRestore:$incrementalRestore}' \
    > "$RESULT_JSON"
  log "P3.10.4 pg_basebackup full/incremental 演练通过：${RESULT_JSON}"
}

main() {
  require_cmd docker
  require_cmd curl
  require_cmd jq

  cd "$REPO_ROOT"
  start_rehearsal_environment
  enable_pg17_incremental_if_available
  login_opshub
  BACKEND_CONNECT_HOST="$(detect_backend_host)"
  log "OpsHub backend 容器访问演练服务的地址：${BACKEND_CONNECT_HOST}"
  create_opshub_resources "$BACKEND_CONNECT_HOST"
  run_pgbase_rehearsal
}

main "$@"
