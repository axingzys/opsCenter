#!/usr/bin/env bash
set -Eeuo pipefail

cat >> "${PGDATA}/pg_hba.conf" <<'EOF'

# OpsHub P3.10 Barman streaming rehearsal.
host replication barman 0.0.0.0/0 scram-sha-256
host replication barman ::/0 scram-sha-256
EOF
