# OpsHub P3.10 PITR E2E

This directory contains a local PostgreSQL + Barman + SSH Runner rehearsal environment for P3.10.

It is intentionally separate from the production `docker-compose.yml`. The environment starts:

- `opshub-pitr-postgres`: PostgreSQL 16 with WAL streaming enabled.
- `opshub-pitr-barman-runner`: Barman + SSH server. It mounts the host Docker socket and `/usr/bin/docker`, so OpsHub can start the isolated restore container from the Runner.

Run the target-time rehearsal from the repository root:

```bash
scripts/pitr-e2e/p3-10-target-time.sh
```

The script will:

1. Start the rehearsal containers.
2. Start Barman `receive-wal`.
3. Login to OpsHub as `admin`.
4. Register temporary credentials, PostgreSQL instance, Runner host, and Barman server.
5. Trigger a real Barman backup through OpsHub.
6. Insert data after the selected target time and force WAL rotation.
7. Sync Barman catalog and WAL metadata back into OpsHub.
8. Create a target-time restore plan.
9. Run Barman restore into an isolated PostgreSQL container.
10. Validate the restored database using SQL assertions.

Useful environment overrides:

```bash
OPSHUB_API_BASE=http://127.0.0.1:9876
OPSHUB_ADMIN_USER=admin
OPSHUB_ADMIN_PASSWORD=123456
PITR_POSTGRES_PORT=55432
PITR_SSH_PORT=2222
PITR_RESTORE_PORT=55433
PITR_POSTGRES_IMAGE=postgres:16
PITR_BARMAN_GET_WAL=false
PITR_RESET_ENV=true
PITR_CLEANUP_RESTORE_CONTAINERS=true
```

The default target-time rehearsal uses `barmanGetWal=false`, so Barman pre-stages WAL during `barman restore` and the isolated PostgreSQL container does not need the `barman` CLI inside the image. Set `PITR_BARMAN_GET_WAL=true` only when the restore image can execute `barman get-wal`.

Runtime output is written under `scripts/pitr-e2e/results/` and is ignored by git.
