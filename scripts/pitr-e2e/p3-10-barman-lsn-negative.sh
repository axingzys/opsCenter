#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export PITR_RESTORE_PORT="${PITR_RESTORE_PORT:-55434}"

# Reuse the P3.10 environment/API helpers without running the target-time rehearsal.
# shellcheck source=./p3-10-target-time.sh
source "${SCRIPT_DIR}/p3-10-target-time.sh"

RESULT_JSON="${RESULT_DIR}/p3-10-barman-lsn-negative-${PITR_RUN_ID}.json"

trigger_barman_backup() {
  local job
  job="$(api_data POST "/api/v1/databases/barman-servers/${BARMAN_ID}/backup")"
  wait_runner_job "$(jq -r '.id' <<<"$job")" "barman-backup" 900 >/dev/null
}

future_lsn() {
  local current_bytes future_bytes
  current_bytes="$(psql_source "select pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0')::bigint;")"
  future_bytes=$((current_bytes + 268435456))
  printf '%X/%X\n' $((future_bytes / 4294967296)) $((future_bytes % 4294967296))
}

segment_add() {
  local name="$1"
  local offset="$2"
  local tli="${name:0:8}"
  local log_hex="${name:8:8}"
  local seg_hex="${name:16:8}"
  local log_dec=$((16#$log_hex))
  local seg_dec=$((16#$seg_hex + offset))
  while [ "$seg_dec" -ge 256 ]; do
    seg_dec=$((seg_dec - 256))
    log_dec=$((log_dec + 1))
  done
  printf '%s%08X%08X\n' "$tli" "$log_dec" "$seg_dec"
}

lsn_from_bytes() {
  local value="$1"
  printf '%X/%X\n' $((value / 4294967296)) $((value % 4294967296))
}

register_barman_wal_catalog_metadata() {
  local logs stream_id files now_iso later_iso line name path size timeline log_hex seg_hex log_dec seg_dec start_bytes end_bytes start_lsn end_lsn
  logs="$(api_data GET "/api/v1/databases/log-archives?instanceId=${INSTANCE_ID}&archiveType=wal&page=1&pageSize=500")"
  stream_id="$(jq -r '.list[]? | .streamId' <<<"$logs" | head -n 1)"
  if [ -z "$stream_id" ]; then
    echo "$logs" | jq . >&2
    echo "missing Barman WAL archive stream" >&2
    exit 1
  fi
  files="$(docker exec opshub-pitr-barman-runner sh -lc 'find /var/lib/barman/pg-main/wals -type f -regextype posix-extended -regex ".*/[0-9A-F]{24}$" | sort | while read -r p; do n="$(basename "$p")"; s="$(wc -c < "$p" | tr -d " ")"; printf "%s\t%s\t%s\n" "$n" "$p" "$s"; done')"
  now_iso="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  later_iso="$(date -u -d '+60 seconds' '+%Y-%m-%dT%H:%M:%SZ')"
  while IFS=$'\t' read -r name path size; do
    [ -n "$name" ] || continue
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
      --argjson streamId "$stream_id" \
      --arg fileName "$name" \
      --arg storageUri "barman://${BARMAN_SERVER_NAME}/wal/${name}" \
      --argjson fileSize "$size" \
      --arg first "$now_iso" \
      --arg last "$later_iso" \
      --arg timeline "$timeline" \
      --arg startLsn "$start_lsn" \
      --arg endLsn "$end_lsn" \
      --arg segmentNo "$name" \
      '{streamId:$streamId,fileName:$fileName,storageUri:$storageUri,fileSize:$fileSize,firstEventTime:$first,lastEventTime:$last,status:"archived",timelineId:$timeline,startLsn:$startLsn,endLsn:$endLsn,segmentNo:$segmentNo}')" >/dev/null
  done <<<"$files"
}

assert_plan_failed() {
  local plan="$1"
  local label="$2"
  local expected="$3"
  local status log_status message
  status="$(jq -r '.validationStatus // ""' <<<"$plan")"
  log_status="$(jq -r '.logChainStatus // ""' <<<"$plan")"
  message="$(jq -r '.message // ""' <<<"$plan")"
  if [ "$status" = "passed" ]; then
    echo "$plan" | jq . >&2
    echo "${label} should not pass validation" >&2
    exit 1
  fi
  if [ "$log_status" != "$expected" ]; then
    echo "$plan" | jq . >&2
    echo "${label} expected logChainStatus=${expected}, got ${log_status}" >&2
    exit 1
  fi
  if [ -z "$message" ]; then
    echo "$plan" | jq . >&2
    echo "${label} should include an explicit failure message" >&2
    exit 1
  fi
}

register_fake_wal_evidence() {
  local logs stream_id last_segment system_id gap_segment timeline_segment now_iso later_iso
  logs="$(api_data GET "/api/v1/databases/log-archives?instanceId=${INSTANCE_ID}&archiveType=wal&page=1&pageSize=500")"
  stream_id="$(jq -r '.list[]? | select(.fileName | test("^[0-9A-F]{24}$")) | .streamId' <<<"$logs" | head -n 1)"
  last_segment="$(jq -r '.list[]? | select(.fileName | test("^[0-9A-F]{24}$")) | .fileName' <<<"$logs" | sort | tail -n 1)"
  system_id="$(jq -r '.list[]? | select(.pgSystemIdentifier != "") | .pgSystemIdentifier' <<<"$logs" | head -n 1)"
  if [ -z "$stream_id" ] || [ -z "$last_segment" ]; then
    echo "$logs" | jq . >&2
    echo "missing WAL archive stream for UI gap evidence" >&2
    exit 1
  fi
  gap_segment="$(segment_add "$last_segment" 5)"
  timeline_segment="00000002${gap_segment:8:16}"
  now_iso="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  later_iso="$(date -u -d '+60 seconds' '+%Y-%m-%dT%H:%M:%SZ')"

  api_data POST /api/v1/databases/log-archives/external "$(jq -cn \
    --argjson streamId "$stream_id" \
    --arg name "$gap_segment" \
    --arg uri "metadata://p3-10/wal-gap/${gap_segment}" \
    --arg first "$now_iso" \
    --arg last "$later_iso" \
    --arg systemId "$system_id" \
    '{streamId:$streamId,fileName:$name,storageUri:$uri,firstEventTime:$first,lastEventTime:$last,status:"missing",pgSystemIdentifier:$systemId,timelineId:"1",segmentNo:$name}')" >/dev/null

  api_data POST /api/v1/databases/log-archives/external "$(jq -cn \
    --argjson streamId "$stream_id" \
    --arg name "$timeline_segment" \
    --arg uri "metadata://p3-10/timeline-switch/${timeline_segment}" \
    --arg first "$now_iso" \
    --arg last "$later_iso" \
    --arg systemId "$system_id" \
    '{streamId:$streamId,fileName:$name,storageUri:$uri,firstEventTime:$first,lastEventTime:$last,status:"archived",pgSystemIdentifier:$systemId,timelineId:"2",segmentNo:$name}')" >/dev/null

  api_data GET "/api/v1/databases/log-archives?instanceId=${INSTANCE_ID}&archiveType=wal&page=1&pageSize=500"
}

run_barman_lsn_and_negative_rehearsal() {
  log "准备 Barman target LSN 演练数据"
  psql_source "truncate table pitr_marker restart identity;" >/dev/null
  psql_source "insert into pitr_marker(label,payload) values ('before_backup','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null

  trigger_barman_backup
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 5
  sync_barman_catalog_and_wal

  psql_source "insert into pitr_marker(label,payload) values ('lsn_target','${PITR_RUN_ID}');" >/dev/null
  TARGET_LSN="$(psql_source "select pg_current_wal_lsn();")"
  log "目标恢复 LSN：${TARGET_LSN}"
  psql_source "insert into pitr_marker(label,payload) values ('after_lsn','${PITR_RUN_ID}');" >/dev/null
  psql_source "select pg_switch_wal();" >/dev/null
  sleep 5
  sync_barman_catalog_and_wal
  register_barman_wal_catalog_metadata

  local plan restore restored_labels future plan_gap plan_timeline wal_evidence
  plan="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --arg target "$TARGET_LSN" \
    '{sourceInstanceId:$sourceInstanceId,restoreMode:"isolated_restore",restoreTargetType:"lsn",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  PLAN_ID="$(jq -r '.id' <<<"$plan")"
  if [ "$(jq -r '.validationStatus' <<<"$plan")" != "passed" ]; then
    echo "$plan" | jq . >&2
    echo "target LSN restore plan did not pass" >&2
    exit 1
  fi

  log "执行 Barman target LSN 隔离恢复"
  restore="$(api_data POST "/api/v1/databases/restore-plans/${PLAN_ID}/run" "$(jq -cn \
    --argjson runnerHostId "$RUNNER_ID" \
    --arg image "$PITR_POSTGRES_IMAGE" \
    --argjson listenPort "$PITR_RESTORE_PORT" \
    --argjson barmanGetWal "$([ "$PITR_BARMAN_GET_WAL" = "true" ] && printf true || printf false)" \
    '{runnerHostId:$runnerHostId,containerImage:$image,listenPort:$listenPort,postgresStartInstance:true,targetAction:"pause",barmanGetWal:$barmanGetWal,cleanupOnFailure:false,validationAssertions:[
      {sql:"select count(*) from pitr_marker where label = '\''before_backup'\''",expectedScalar:"1"},
      {sql:"select count(*) from pitr_marker where label = '\''lsn_target'\''",expectedScalar:"1"},
      {sql:"select count(*) from pitr_marker where label = '\''after_lsn'\''",expectedScalar:"0"}
    ]}')")"
  RESTORE_JOB_ID="$(jq -r '.id' <<<"$restore")"
  RESTORE_RESULT="$(wait_restore_job "$RESTORE_JOB_ID" 1200)"

  restored_labels="$(docker exec -e PGPASSWORD=opshub opshub-pitr-barman-runner psql -h 127.0.0.1 -p "$PITR_RESTORE_PORT" -U opshub -d opshub_pitr -At -v ON_ERROR_STOP=1 -c "select string_agg(label, ',' order by id) from pitr_marker;")"
  if [ "$restored_labels" != "before_backup,lsn_target" ]; then
    echo "unexpected restored labels for target LSN: ${restored_labels}" >&2
    exit 1
  fi

  future="$(future_lsn)"
  log "验证 WAL gap 不能误判为可恢复：${future}"
  plan_gap="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --arg target "$future" \
    '{sourceInstanceId:$sourceInstanceId,restoreMode:"isolated_restore",restoreTargetType:"lsn",restoreTargetValue:$target,restoreTargetInclusive:true}')")"
  assert_plan_failed "$plan_gap" "wal-gap" "missing_wal"

  log "验证 timeline mismatch 不能执行"
  plan_timeline="$(api_data POST /api/v1/databases/restore-plans "$(jq -cn \
    --argjson sourceInstanceId "$INSTANCE_ID" \
    --arg target "$TARGET_LSN" \
    '{sourceInstanceId:$sourceInstanceId,restoreMode:"isolated_restore",restoreTargetType:"lsn",restoreTargetValue:$target,targetTimelineId:"2",restoreTargetInclusive:true}')")"
  assert_plan_failed "$plan_timeline" "timeline-mismatch" "timeline_mismatch"

  log "登记 WAL gap/timeline switch 元数据，用于前端 WAL 状态页可见性检查"
  wal_evidence="$(register_fake_wal_evidence)"

  jq -n \
    --arg runId "$PITR_RUN_ID" \
    --arg targetLsn "$TARGET_LSN" \
    --arg futureLsn "$future" \
    --arg restoredLabels "$restored_labels" \
    --argjson instanceId "$INSTANCE_ID" \
    --argjson runnerHostId "$RUNNER_ID" \
    --argjson barmanServerId "$BARMAN_ID" \
    --argjson restorePlan "$plan" \
    --argjson restoreJob "$RESTORE_RESULT" \
    --argjson walGapPlan "$plan_gap" \
    --argjson timelineMismatchPlan "$plan_timeline" \
    --argjson walEvidence "$wal_evidence" \
    '{runId:$runId,targetLsn:$targetLsn,futureLsn:$futureLsn,restoredLabels:$restoredLabels,instanceId:$instanceId,runnerHostId:$runnerHostId,barmanServerId:$barmanServerId,restorePlan:$restorePlan,restoreJob:$restoreJob,walGapPlan:$walGapPlan,timelineMismatchPlan:$timelineMismatchPlan,walEvidence:{total:$walEvidence.total,gapCount:([ $walEvidence.list[]? | select(.status=="missing") ] | length),timelineIds:([ $walEvidence.list[]?.timelineId ] | unique)}}' \
    > "$RESULT_JSON"
  log "P3.10.3 Barman target LSN / 失败演练通过：${RESULT_JSON}"
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
  run_barman_lsn_and_negative_rehearsal
}

main "$@"
