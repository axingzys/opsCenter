#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN_ID="${PITR_RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"

export PITR_RUN_ID="${RUN_ID}-time"
"${SCRIPT_DIR}/p3-10-target-time.sh"

export PITR_RUN_ID="${RUN_ID}-lsn"
"${SCRIPT_DIR}/p3-10-barman-lsn-negative.sh"

export PITR_RUN_ID="${RUN_ID}-pgbase"
"${SCRIPT_DIR}/p3-10-pg-basebackup.sh"

printf 'P3.10 full rehearsal suite passed: %s\n' "$RUN_ID"
