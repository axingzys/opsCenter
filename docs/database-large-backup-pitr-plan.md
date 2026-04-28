# OpsHub 大库备份与 PITR 长期改造方案

更新日期：2026-04-29

## 实施记录

### 2026-04-28 P1 已落地

本次 P1 已完成“模型、登记、预校验”层，不包含真实物理备份执行、日志连续拉取守护进程和恢复库拉起。

已落地内容：

1. 新增备份链、日志归档流、日志归档文件、恢复计划、存储配置、密钥配置和 Runner Job 的后端模型与自动迁移。
2. 扩展备份任务和备份记录，记录 `backup_method`、`backup_level`、`backup_engine`、`chain_id`、`base_record_id`、`recoverable_from`、`recoverable_until`、binlog/GTID、WAL/LSN/timeline 等 P1 元数据。
3. 新增外部备份记录登记接口，用于纳管 XtraBackup、mariadb-backup、Barman、WAL-G、pg_basebackup 等外部工具已生成的备份。
4. 新增日志归档流和外部日志归档文件登记接口，binlog/WAL 作为独立日志链管理，不混入 `backup_method`。
5. 新增恢复计划生成接口，按目标时间选择基础备份、检查日志归档覆盖、存储对象、checksum 和工具状态，输出预校验状态与计划 JSON。
6. 前端数据库管理的“备份任务”页新增 PITR 链路面板，支持归档流、日志归档、恢复计划查看，以及外部备份、日志归档和恢复计划登记/生成。

后续 P2/P3 仍按本文继续推进：真实物理备份 Runner、连续 binlog/WAL 归档守护进程、工具版本兼容矩阵、恢复到隔离库、恢复证明和副本治理。

### 2026-04-28 P2 第一版已落地

本次 P2 已完成 MySQL/MariaDB 物理备份主链路的第一版可用能力。边界是：OpsHub 后端可以编排并执行 XtraBackup / mariadb-backup 命令、登记物理备份元数据、生成 PITR 恢复计划和恢复证明；真正“长期连续运行的 binlog 归档守护进程”和“自动停止/重建目标 MySQL 数据目录的隔离恢复 Runner”仍建议继续按后续 Runner 专项深化，或先通过外部工具执行后登记元数据。

已落地内容：

1. 备份任务支持 MySQL/MariaDB `physical` 方法，包含全量和增量配置入口。
2. Runner 支持调用 `xtrabackup` 和 `mariadb-backup` 执行物理备份，并将结果打包为 `.physical.tar.gz`。
3. 任务创建时按数据库类型和版本做工具兼容校验：MySQL 8.0.x 对应 XtraBackup 8.0，MySQL 8.4.x 对应 XtraBackup 8.4，MariaDB 默认且仅推荐 `mariadb-backup`。
4. 物理备份完成后采集 `server_uuid/server_id/gtid/binlog_format/binlog_row_image` 等 MySQL/MariaDB 元数据。
5. 物理备份归档中如存在 `xtrabackup_binlog_info` 或 `mariadb_backup_binlog_info`，优先读取其中的 binlog 文件、position 和 GTID 作为恢复起点。
6. 物理备份完成后自动登记一条 binlog 元数据归档记录，用于恢复计划预校验；连续归档链仍可通过外部归档登记继续补齐。
7. 恢复计划校验增加增量父链检查、binlog 文件链检查和 GTID 元数据缺口检查。
8. 恢复计划新增 `proof_json`，包含 base backup、incremental chain、binlog 区间、checksum 摘要和恢复校验 SQL。
9. 前端备份任务弹窗支持 MySQL/MariaDB 物理备份方法、备份级别和备份引擎选择；外部备份登记弹窗已放大并优化数字字段宽度。
10. 新增单元测试覆盖工具兼容矩阵、增量父链缺失、binlog/GTID 缺口和恢复证明字段。

后续 P2 深化项：

1. 将 binlog 归档从“备份后元数据采样/外部登记”升级为长期运行的 `mysqlbinlog --read-from-remote-server` 或高频 Runner。
2. 增加真正隔离恢复 Runner：准备物理备份、应用增量、按目标时间应用 binlog、启动隔离库并执行校验 SQL。
3. 增加备份工具部署检测和 Runner 主机能力模型，避免要求 backend 容器直接承担所有数据库主机级操作。

### 2026-04-29 P2.1 + P2.2 实施边界

本次继续完善 P2，但仍遵守一个关键边界：`opshub-api` 普通 backend 容器只负责编排、权限、审计、状态和短生命周期 SSH 调度；真正长期连续 binlog archiver、恢复目标 MySQL datadir 停止/清空/prepare/重建/启动等主机级动作，必须由 Runner 主机或后续 Agent 承载，不能简单塞进 HTTP 请求或 backend 容器内长期运行。

#### P2.1：Runner Host 模型和 Runner Job 扩展

目标是先把“哪些主机可以执行数据库备份/归档/恢复命令”建模清楚，让后续 binlog archiver、隔离恢复 Runner 和 PostgreSQL Barman/WAL-G Runner 都有统一入口。

新增 `database_runner_hosts`：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `name` | varchar(120) | Runner 主机名称 |
| `runner_type` | varchar(30) | `ssh / local / agent`，首版实现 `ssh`，预留 `local` 和 `agent` |
| `host` | varchar(255) | SSH 主机或 Runner 地址 |
| `port` | int | SSH 端口，默认 22 |
| `credential_id` | bigint | 复用资产凭据，不在 Runner 表保存密码、私钥或 token |
| `work_dir` | varchar(500) | Runner 工作目录，例如 `/var/lib/opshub/database-runner` |
| `storage_mount_path` | varchar(500) | 备份仓库在 Runner 主机上的挂载点，例如 `/backup/opshub` |
| `max_concurrent_jobs` | int | 单 Runner 最大并发，首版用于展示和后续调度约束 |
| `cpu_limit` | varchar(60) | 资源策略摘要，例如 `2 cores`、`nice=10` |
| `io_limit` | varchar(60) | IO 策略摘要，例如 `ionice=best-effort:7` |
| `bandwidth_limit` | varchar(60) | 上传/拉取带宽限制摘要，例如 `50MB/s` |
| `timeout_minutes` | int | 默认任务超时 |
| `enabled` | bool | 是否允许调度 |
| `status` | varchar(30) | `pending / online / failed / disabled` |
| `last_heartbeat_at` | datetime | 后续 Agent 心跳；SSH Runner 首版可为空 |
| `last_test_at` | datetime | 最近一次连通性测试时间 |
| `last_error` | varchar(1000) | 最近错误 |
| `config_json` | text | 非敏感配置摘要，禁止保存 password/secret/token/key 等明文 |

扩展 `database_runner_jobs`：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `runner_host_id` | bigint | 关联 Runner Host |
| `runner_id` | varchar(120) | 兼容旧字段，首版可写入 `ssh:<host>:<port>` |
| `job_type` | varchar(60) | `runner_probe / physical_backup / binlog_archive / physical_restore / restore_validate` |
| `allowed_command` | varchar(120) | 命令白名单类别，例如 `runner_probe`、`mysqlbinlog_probe` |
| `command_summary` | varchar(500) | 命令摘要，不保存敏感参数 |
| `work_dir` | varchar(500) | 本次任务工作目录 |
| `log_path` | varchar(1000) | 远端日志路径或后续对象存储日志 URI |
| `exit_code` | int | 进程退出码；SSH 无法稳定解析时失败统一记 1 |
| `operator_id` | bigint | 操作人 |
| `operator_name` | varchar(120) | 操作人 |
| `request_json` | text | 下发请求，必须脱敏 |
| `result_json` | text | 回传结果，包含 stdout/stderr 摘要、工具版本、manifest 摘要 |
| `status` | varchar(30) | `queued / running / success / failed / cancelled` |
| `heartbeat_at` | datetime | 长任务心跳 |
| `started_at` | datetime | 开始时间 |
| `finished_at` | datetime | 完成时间 |
| `duration_ms` | bigint | 耗时 |
| `error_message` | varchar(1000) | 错误摘要 |

P2.1 后端接口：

```text
GET  /api/v1/databases/runner-hosts
POST /api/v1/databases/runner-hosts
PUT  /api/v1/databases/runner-hosts/{id}
GET  /api/v1/databases/runner-jobs
```

P2.1 前端入口：

1. 在数据库管理的“备份任务 / PITR 链路与恢复计划”区域增加 `Runner 主机` 与 `Runner任务` 标签页。
2. Runner 主机列表展示主机、类型、凭据、工作目录、存储挂载点、并发、状态、最近测试和最近错误。
3. 新增/编辑弹窗只允许选择已有凭据；不出现密码、私钥、对象存储密钥明文输入框。
4. Runner Job 列表展示任务类型、Runner、来源实例、目标实例、状态、命令类别、耗时、退出码和错误。

#### P2.2：SSH Runner 执行框架

目标是先打通“Backend 下发一个白名单 Runner Job 到 SSH 主机、异步执行、回写结果”的通用闭环。它不是最终 binlog archiver，也不是最终物理恢复 Runner，但后续二者都必须复用这个执行框架。

首版执行能力：

1. `POST /api/v1/databases/runner-hosts/{id}/test` 创建 `runner_probe` Job。
2. Job 通过 SSH 连接 Runner Host。
3. 只执行内置探测脚本，不接受用户提交任意 shell：

```sh
set -e
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"
printf 'opshub-runner-ok\n'
pwd
whoami
hostname
command -v xtrabackup || true
command -v mariadb-backup || true
command -v mysqlbinlog || true
command -v barman || true
command -v wal-g || true
```

4. 采集 stdout/stderr、耗时、退出状态，写回 `database_runner_jobs.result_json`。
5. 成功则把 Runner Host 标记为 `online`，失败标记为 `failed` 并记录 `last_error`。
6. SSH 凭据从现有资产凭据解密读取，支持密码和私钥字段；Runner Host 表只保存 `credential_id`。
7. `config_json`、`request_json` 和 `result_json` 必须做长度限制和敏感字段校验，不允许保存明文 `password`、`secret`、`token`、`private_key`、`access_key`。

P2.2 明确不做：

1. 不实现长期驻留的 `mysqlbinlog --stop-never` 守护进程。
2. 不自动清空、重建、启动目标 MySQL datadir。
3. 不开放用户自定义任意 shell 命令。
4. 不把对象存储密钥写入 Runner Host 或 Runner Job 明文 JSON。
5. 不承诺物理恢复已自动可执行，只形成后续可复用的执行底座。

P2.2 验收标准：

1. 页面可以新增 SSH Runner Host。
2. 页面可以发起 Runner 连通性/工具探测。
3. Runner Job 异步产生 queued、running、success/failed 状态。
4. Runner Host 最近测试时间、状态和错误随 Job 结果更新。
5. Runner Job 能展示 stdout 摘要、命令类别、耗时、退出码和错误。
6. 后端单元测试覆盖 Runner Host 参数归一化、敏感 JSON 拦截、probe 命令生成和 Job 成功/失败状态回写。

本次 P2.1 + P2.2 已落地内容：

1. 新增 `database_runner_hosts` 模型、仓库、自动迁移和列表/创建/更新接口。
2. 扩展 `database_runner_jobs`，增加 `runner_host_id`、`command_summary`、`work_dir`、`log_path`、`exit_code`、`operator_id`、`operator_name` 等字段。
3. 新增 `POST /api/v1/databases/runner-hosts/{id}/test`，通过 SSH 执行固定白名单探测脚本，异步回写 Runner Job 和 Runner Host 状态。
4. 现有资产凭据解析扩展到 SSH 私钥和 passphrase，Runner Host 只保存 `credential_id`。
5. 前端 PITR 面板新增 `Runner主机` 和 `Runner任务` 标签页，支持 Runner 主机维护、探测任务下发、Job 状态和输出摘要查看。
6. 新增 Runner 单元测试，覆盖默认值、敏感配置拦截、固定探测命令、请求脱敏、成功/失败状态回写。

### 2026-04-29 P2.3 + P2.4 实施边界

P2.1 + P2.2 已经打通了 Runner 主机管理和 SSH 白名单探测，但真正要支撑 PITR，还需要把 binlog 归档从“外部登记/备份后元数据采样”推进到“由 Runner 受控拉取一个真实 binlog 文件”。本阶段继续保持低风险边界：只做一次性归档任务，不做后台常驻守护进程，不自动停止或重建任何数据库 datadir。

#### P2.3：Runner 能力探测增强

目标是让 Runner 探测结果不只是展示原始 stdout，而是逐步沉淀成后续调度可用的能力输入。

能力范围：

1. 继续使用 `runner_probe` 白名单命令探测 Runner 主机。
2. 探测内容覆盖 `xtrabackup`、`mariadb-backup`、`mysqlbinlog`、`mariadb-binlog`、`barman`、`wal-g` 等工具是否存在。
3. 探测结果仍写入 `database_runner_jobs.result_json`，前端 Runner 任务列表展示 stdout 摘要。
4. 本阶段不新增单独的 `database_runner_capabilities` 表，避免过早固化工具版本模型；等 P2.5/P3 接入长期 archiver 或 PostgreSQL WAL-G/Barman 时再把能力表拆出来。

验收标准：

1. Runner 探测任务仍只能执行内置脚本。
2. 探测输出能看出目标主机是否具备 `mysqlbinlog` / `mariadb-binlog`。
3. 后续一次性 binlog 归档任务可以复用同一 SSH 执行底座。

#### P2.4：一次性 MySQL/MariaDB binlog 归档 Runner

目标是增加一个“安全、短生命周期、可审计”的 binlog 拉取任务，作为长期连续归档守护进程前的第一版真实归档能力。

新增后端接口：

```text
POST /api/v1/databases/log-archive-streams/{id}/run-once
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `runnerHostId` | bigint | 是 | 执行归档的 Runner Host |
| `fileName` | string | 否 | 指定要拉取的 binlog 文件；为空时由 OpsHub 根据归档流最近文件和源库 `SHOW BINARY LOGS` 自动选择 |

执行流程：

1. 读取日志归档流，要求 `archive_type=binlog`，数据库类型必须是 MySQL 或 MariaDB。
2. 读取归档来源实例，使用实例凭据连接源库执行 `SHOW BINARY LOGS` / `SHOW MASTER LOGS`，获取当前 binlog 文件列表和大小。
3. 选择待归档文件：
   - 如果请求指定 `fileName`，必须在源库当前 binlog 列表中存在。
   - 如果未指定且归档流有 `last_archive_name`，优先选择它后面的下一个文件。
   - 如果未指定且没有下一个文件，选择当前最新 binlog 文件。
4. 创建 `database_runner_jobs`，`job_type=binlog_archive`，`allowed_command=mysqlbinlog_archive_once`。
5. 通过 SSH Runner 执行固定脚本，不接受前端传入 shell。
6. Runner 脚本在远端创建临时 `defaults-extra-file`，用实例凭据连接源库，执行：

```sh
mysqlbinlog --defaults-extra-file=/tmp/opshub.cnf \
  --read-from-remote-server \
  --raw \
  --result-file="$DEST_DIR/" \
  "$BINLOG_FILE"
```

7. Runner 对拉取到的 binlog 文件计算大小和 SHA256，并用本地 `mysqlbinlog` 解码文件头尾事件行，输出可解析的元数据。
8. 后端解析 Runner 输出，登记 `database_log_archives`：
   - `file_name`
   - `storage_uri`
   - `file_size`
   - `checksum_sha256`
   - `first_event_time`
   - `last_event_time`
   - `start_pos`
   - `end_pos`
   - `previous_file_name`
   - `next_file_name`
9. 更新 `database_log_archive_streams.last_archived_at`、`last_archive_name`、`status` 和 `last_error`。
10. 更新 Runner Job 的状态、退出码、耗时、stdout/stderr 摘要和错误。

安全边界：

1. 前端和 API 不允许提交任意 shell。
2. `database_runner_jobs.request_json` 不保存数据库密码、Runner 密码、私钥或 token。
3. `database_runner_jobs.command_summary` 只保存命令类别和 binlog 文件名，不保存连接串。
4. 数据库密码只通过 SSH stdin 中的临时脚本写入 Runner 主机的临时 option file；脚本退出后立即删除。
5. 不使用 `MYSQL_PWD`，不在远端进程参数中出现 `--password=...`。
6. 本阶段只拉取一个 binlog 文件，不常驻 `mysqlbinlog --stop-never`。
7. 本阶段不自动 purge 源库 binlog，不修改源库复制配置。

前端入口：

1. 在 `归档流` 表格操作列新增“归档一次”。
2. 点击后弹出确认框，选择 Runner Host，可选指定 binlog 文件名。
3. 下发成功后自动刷新 Runner 任务和日志归档列表。
4. `Runner任务` 中可以看到 `binlog 归档` 类型任务，输出摘要包含 binlog 文件、大小和 SHA256。

P2.4 验收标准：

1. 可以对 MySQL/MariaDB binlog 归档流发起一次性归档。
2. 成功后新增一条 `database_log_archives` 记录，归档流最近文件和状态更新。
3. 失败时 Runner Job 失败，归档流标记 degraded/failed 并记录错误。
4. 生成恢复计划时可以利用新登记的真实 binlog 文件元数据参与日志链校验。
5. 单元测试覆盖 binlog 文件选择、事件时间解析、脚本安全约束和 request_json 脱敏。

本次 P2.3 + P2.4 已落地内容：

1. 新增 `POST /api/v1/databases/log-archive-streams/{id}/run-once`。
2. 支持 SSH Runner 使用 `mysqlbinlog` / `mariadb-binlog` 拉取指定或自动选择的一份 binlog。
3. Runner 归档成功后自动登记 `database_log_archives`，并更新归档流最近文件、最近归档时间和状态。
4. 前端归档流列表新增“归档一次”入口，Runner 任务列表展示 binlog 文件、大小和 SHA256 摘要。
5. 保持安全边界：不开放任意 shell，不在 Job JSON 中保存数据库密码，不使用 `MYSQL_PWD`，不把密码放入远端进程参数，不常驻 `--stop-never`。

### 2026-04-29 P2.5 实施边界

P2.4 只能一次拉取一个 binlog 文件，适合作为验证闭环，但实际归档链经常需要把多个已轮转文件追平。本阶段新增“受控批量 catch-up 归档”，仍不是长期守护进程。

新增后端接口：

```text
POST /api/v1/databases/log-archive-streams/{id}/catch-up
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `runnerHostId` | bigint | 是 | 执行归档的 Runner Host |
| `maxFiles` | int | 否 | 单次最多归档文件数，默认 5，最大 20 |
| `includeCurrent` | bool | 否 | 是否允许归档当前活跃 binlog，默认 false |

选择规则：

1. 先从源库读取 `SHOW BINARY LOGS` / `SHOW MASTER LOGS`。
2. 如果归档流已有 `last_archive_name`：
   - 若该文件仍在源库列表中，从它的下一份开始归档。
   - 若该文件已不在源库列表中，判定源库可能已经 purge，直接失败，避免静默断链。
3. 如果归档流没有 `last_archive_name`，从源库当前可见列表第一份开始归档。
4. 默认 `includeCurrent=false`，即不拉取最后一份当前活跃 binlog；只有用户显式开启才会包含当前活跃文件。
5. 每次最多归档 `maxFiles` 份，防止单个 HTTP 触发过长任务。

执行规则：

1. 一个 catch-up 请求创建一个 `database_runner_jobs`，`job_type=binlog_archive`，`allowed_command=mysqlbinlog_archive_catch_up`。
2. 后端生成固定白名单脚本，脚本内循环拉取多个 binlog 文件。
3. 任何一个文件拉取失败，整个 Runner Job 标记失败，归档流标记 degraded。
4. 所有文件成功后，逐个登记 `database_log_archives`，并将归档流 `last_archive_name` 更新为最后成功文件。
5. `result_json` 同时保留 `binlog` 首文件摘要和 `binlogs` 批量摘要，方便前端快速显示。

安全边界：

1. 仍不实现后台常驻 `mysqlbinlog --stop-never`。
2. 仍不自动 purge 源库 binlog。
3. 仍不修改源库复制配置。
4. 仍不碰恢复目标 datadir。
5. 密码仍只进入 Runner 临时 option file，脚本退出后删除，不进入 Job JSON 和命令摘要。

P2.5 验收标准：

1. 页面可以对 binlog 归档流发起“追平归档”。
2. 默认只拉取已轮转 binlog，不拉取当前活跃 binlog。
3. 如果 `last_archive_name` 已被源库 purge，任务拒绝执行并提示断链风险。
4. 成功后可一次登记多条 `database_log_archives`。
5. 单元测试覆盖 catch-up 文件选择、purge 断链保护、多文件输出解析和 request_json 脱敏。

### 2026-04-29 P2.6 / P2.7 外部建议复核结论

外部建议的主线适合 OpsHub 当前方向：P2.6 做长期 binlog 归档，P2.7 做隔离恢复 Runner，这正好补齐 P2 第一版剩余的两个关键缺口。

但实施时必须收紧以下边界：

1. `mysqlbinlog --stop-never` 不能由 `opshub-api` backend 容器直接长期运行，也不应绑定在一次 HTTP 请求生命周期内。长期进程必须由 Runner Agent、系统服务或后续专用 Runner 托管，backend 只负责策略、租约、审计、状态和恢复计划。
2. SSH Runner 可以继续用于探测、一次性归档和受控追平；如果要运行真正常驻 archiver，SSH 更适合做“安装/启动/停止/查看状态”的管理通道，而不是让 backend 用 SSH session 挂住一个永久命令。
3. P2.6 不能只记录 `cursor_file/cursor_pos`。MySQL/MariaDB 发生主从切换、GTID 变化、purge、server_uuid 改变时，只靠文件名和 position 容易误判链路连续性，还必须记录 `server_uuid/server_id/gtid_set/binlog_format/binlog_row_image/source_role/promotion_history` 等上下文。
4. “拉取当前活跃 binlog”必须和“已完成归档文件”区分。活跃文件可以作为 streaming spool 或 checkpoint，但不应在未 finalized 前作为可恢复链路的完整文件参与 PITR 证明。
5. P2.7 的物理恢复顺序要严格拆开：先准备 base backup 和 incremental chain，再启动隔离实例，最后把 binlog 应用到隔离实例。不能把 `xtrabackup --prepare` 阶段和 `mysqlbinlog` 回放阶段混为一个动作。
6. P2.7 第一版只允许恢复到隔离库或临时容器，不支持直接覆盖生产库，不自动切流，不自动回填生产数据。
7. 生产可用性判断不应只看“任务执行成功”，还要看恢复证明、校验 SQL、checksum、日志链连续性、工具版本兼容和可恢复窗口。

因此，P2.6/P2.7 的落地策略是：

```text
P2.6: 从手动 run-once/catch-up 升级为 Runner 托管的持续归档能力
  ├─ P2.6.1 归档流运行态、游标、租约、监控和 API
  ├─ P2.6.2 高频轮询归档，默认只归档已轮转 binlog
  └─ P2.6.3 streaming archiver，支持低 RPO，但活跃文件必须 spool/finalize

P2.7: 从恢复计划升级为可执行的隔离恢复 Runner
  ├─ P2.7.1 restore job 模型、步骤状态和 Runner 执行框架
  ├─ P2.7.2 物理备份链 prepare 和隔离 MySQL/MariaDB 容器启动
  ├─ P2.7.3 binlog 回放到目标时间/GTID
  └─ P2.7.4 校验 SQL、恢复证明、日志和清理
```

### 2026-04-29 P2.6 详细方案：长期 binlog 归档 Runner

目标：让 MySQL/MariaDB binlog 归档从“手动归档一次 / 手动追平”升级为可长期运行、可观测、可审计、可恢复验证的归档链路。

#### P2.6 核心原则

1. backend 不长期持有数据库复制连接，不长期运行 `mysqlbinlog`。
2. 长期归档执行体必须在 Runner 主机或 Runner Agent 上运行。
3. backend 保存期望状态和审计记录，Runner 保存执行现场并回传心跳。
4. 所有执行命令仍必须白名单化，不开放用户自定义 shell。
5. 密钥只通过凭据解析和临时文件进入 Runner，不写入 `request_json/result_json/config_json`。
6. 归档文件必须先写入临时路径，checksum 通过后再原子进入最终路径。
7. PITR 只使用状态为 `archived/verified/finalized` 的日志文件，不使用未完成的 active spool 文件。
8. 源库 binlog purge 不由 P2.6 自动执行；后续可以基于“已归档且超过安全窗口”提供单独建议或审批动作。

#### P2.6 归档模式

P2.6 支持两种模式，默认先实现安全模式，再实现低 RPO 模式。

模式一：高频轮询归档，默认推荐第一版。

```text
Runner Agent / 调度器每 30-60 秒：
  1. 连接源库读取 SHOW BINARY LOGS / SHOW MASTER STATUS
  2. 根据 stream cursor 找出尚未归档的已轮转 binlog
  3. 调用 mysqlbinlog/mariadb-binlog --read-from-remote-server --raw 拉取文件
  4. 计算 SHA256、文件大小、首尾事件时间、GTID 摘要
  5. 上传到 storage profile 指定仓库
  6. 登记 database_log_archives
  7. 更新 stream cursor 和 archive_lag_seconds
```

特点：

1. 实现风险低，复用 P2.4/P2.5 的文件拉取和登记逻辑。
2. 默认不拉取当前活跃 binlog，避免把未闭合文件当作完整归档。
3. RPO 取决于 binlog rotate 频率和业务写入量，不适合强 1 分钟 RPO 的核心库。
4. 可以作为 P2.6.2 的第一版生产能力。

模式二：streaming 归档，作为增强模式。

```text
Runner Agent 常驻：
  mysqlbinlog/mariadb-binlog
    --read-from-remote-server
    --raw
    --stop-never
    --result-file=<spool_dir>/
```

特点：

1. 用于 RPO <= 1-5 分钟的核心库。
2. 需要为每个 stream 分配唯一 replication client 标识，避免和已有复制链路冲突。
3. 当前活跃 binlog 写入 spool 区，只更新 checkpoint，不立即登记为 finalized。
4. 当源库 rotate 到下一文件后，Runner 对上一文件计算 checksum、补齐元数据、上传并登记为 `archived`。
5. 如果 Runner 断开，重启时根据 checkpoint 从上次 cursor 继续，并先执行 catch-up 修补缺口。
6. 如果发现 cursor 文件已被 purge，必须把 stream 标记为 `broken_chain` 或 `degraded`，不能静默从最新文件继续。

#### P2.6 需要扩展的数据模型

继续复用 `database_log_archive_streams` 作为归档流主表，新增运行态字段：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `runner_host_id` | bigint | 负责该归档流的 Runner Host |
| `desired_state` | varchar(30) | `running / paused / stopped`，用户期望状态 |
| `daemon_status` | varchar(30) | `starting / running / paused / degraded / failed / stopped` |
| `cursor_file` | varchar(255) | 已确认归档或当前 checkpoint 的 binlog 文件 |
| `cursor_pos` | bigint | 当前 checkpoint position |
| `cursor_gtid_set` | text | 当前已覆盖 GTID 集合摘要 |
| `active_file` | varchar(255) | streaming 模式下正在 spool 的活跃 binlog |
| `last_source_file` | varchar(255) | 最近一次源库显示的当前 binlog |
| `last_source_pos` | bigint | 最近一次源库当前 position |
| `last_event_time` | datetime | 最近解析到的 binlog 事件时间 |
| `archive_lag_seconds` | int | 归档延迟，按源库当前时间或事件时间估算 |
| `last_heartbeat_at` | datetime | Runner 最近心跳 |
| `consecutive_failures` | int | 连续失败次数 |
| `lease_owner` | varchar(120) | 当前持有该 stream 的 Runner ID |
| `lease_expires_at` | datetime | 租约过期时间，防双 Runner 同时归档 |
| `paused_at` | datetime | 暂停时间 |
| `paused_reason` | varchar(500) | 暂停原因 |

`archive_mode` 建议规范为：

```text
external        外部系统归档，OpsHub 只登记元数据
manual_once     手动单文件归档
catch_up        手动批量追平
polling         Runner 高频轮询归档
streaming       Runner streaming 常驻归档
```

`database_log_archives` 建议补充或严格使用以下字段：

| 字段 | 说明 |
| --- | --- |
| `file_name` | binlog 文件名 |
| `storage_uri` | 最终归档地址 |
| `file_size` | 文件大小 |
| `checksum_sha256` | 文件 checksum |
| `first_event_time / last_event_time` | 首尾事件时间 |
| `start_pos / end_pos` | 文件位置范围 |
| `start_gtid_set / end_gtid_set` | GTID 范围或摘要 |
| `server_uuid / server_id` | 来源 server 标识 |
| `previous_file_name / next_file_name` | 文件链 |
| `status` | `archived / verified / missing / checksum_failed / expired / partial` |

是否新增表：

1. 第一版可以不新增 `database_log_archiver_runs`，归档循环状态先落在 stream 和 runner job。
2. 如果要做完整运维审计，建议后续新增 `database_log_archive_events`，记录 daemon start、stop、heartbeat missed、purge gap、checksum failed、upload retry 等事件。

#### P2.6 Runner Agent 行为

Runner Agent 获取任务有两种实现方式：

1. 后端主动下发：适合 SSH Runner 的短任务，不适合 streaming。
2. Agent 主动拉取：适合长期 archiver。Agent 定期调用 backend 获取自己负责的 stream 列表，并续租。

推荐 P2.6 使用 Agent 主动拉取模型：

```text
Agent loop:
  1. heartbeat runner host
  2. acquire/renew stream lease
  3. load stream config
  4. run polling or streaming archiver
  5. upload archive artifacts
  6. report checkpoint and metrics
  7. release lease on stop/pause/failure
```

租约规则：

1. 一个归档流同一时间只能被一个 Runner 持有。
2. Runner 每 10-30 秒续租。
3. backend 发现 `lease_expires_at < now` 后允许其他 Runner 接管。
4. 接管前必须重新读取源库 binlog 列表并验证 cursor 是否仍存在。
5. 如果 cursor 不存在，stream 进入 degraded，提示需要人工确认是否从备份链补洞。

#### P2.6 后端接口

新增或扩展接口：

```text
POST /api/v1/databases/log-archive-streams/{id}/start
POST /api/v1/databases/log-archive-streams/{id}/pause
POST /api/v1/databases/log-archive-streams/{id}/resume
POST /api/v1/databases/log-archive-streams/{id}/stop
GET  /api/v1/databases/log-archive-streams/{id}/status

POST /api/v1/databases/runner-agents/{runnerId}/heartbeat
GET  /api/v1/databases/runner-agents/{runnerId}/log-archive-streams
POST /api/v1/databases/runner-agents/{runnerId}/log-archive-streams/{id}/checkpoint
POST /api/v1/databases/runner-agents/{runnerId}/log-archives
```

权限建议：

| 操作 | 权限 |
| --- | --- |
| 查看归档流状态 | `database:backup:view` |
| 启动、暂停、恢复、停止 archiver | `database:backup:run` |
| 修改归档流 Runner、模式、RPO、存储 | `database:backup:update` |
| Agent 心跳和 checkpoint | Runner token 或 mTLS，不使用用户 token |

#### P2.6 存储与文件布局

本地或挂载存储建议：

```text
<storage_root>/mysql-binlog/
  instance-<instance_id>/
    stream-<stream_id>/
      finalized/
        binlog.000001
        binlog.000001.sha256
        binlog.000001.manifest.json
      spool/
        binlog.000002.partial
      logs/
        archiver-20260429.log
```

对象存储建议：

```text
s3://<bucket>/<prefix>/mysql-binlog/instance-<id>/stream-<id>/finalized/binlog.000001
s3://<bucket>/<prefix>/mysql-binlog/instance-<id>/stream-<id>/manifest/binlog.000001.json
```

上传规则：

1. 先上传到 `.tmp` 或 staging key。
2. checksum 通过后写入最终 key。
3. `database_log_archives` 只登记最终 key。
4. 如果上传失败，保留本地 spool 并重试，不更新 cursor 到已归档状态。
5. 如果数据库文件已下载但 checksum 或解析失败，登记 `checksum_failed` 或只记录 runner event，不进入可恢复链。

#### P2.6 监控指标

前端和后端至少展示：

1. `desired_state`
2. `daemon_status`
3. `archive_mode`
4. `runner_host`
5. `cursor_file / cursor_pos`
6. `active_file`
7. `last_source_file / last_source_pos`
8. `archive_lag_seconds`
9. `last_heartbeat_at`
10. `consecutive_failures`
11. `last_error`
12. 最近 10 条归档文件
13. 最近 10 条 Runner 事件或 Job

告警建议：

| 条件 | 风险 |
| --- | --- |
| `archive_lag_seconds > rpo_target_seconds * 2` | RPO 超标 |
| `last_heartbeat_at` 超过 2 个心跳周期 | Runner 可能失联 |
| `cursor_file` 已不在源库 binlog 列表 | 源库可能 purge，日志链断裂 |
| `checksum_failed` | 归档文件不可用 |
| `consecutive_failures >= 3` | 归档流 degraded |
| `server_uuid` 变化但无 promotion 记录 | 可能发生主从切换，需人工确认 |

#### P2.6 与 PITR 的集成规则

恢复计划只能使用满足以下条件的 binlog：

1. `status in ('archived', 'verified')`
2. `checksum_sha256` 存在且校验通过
3. `first_event_time/last_event_time` 能覆盖目标时间
4. 文件链 `previous_file_name/next_file_name` 连续
5. GTID 模式下 `start_gtid_set/end_gtid_set` 不出现缺口
6. source server 变化时能找到 promotion history 或人工确认记录

如果目标时间落在 active spool 文件内：

1. polling 模式：默认判定不可恢复到该时间点，提示等待 rotate 或手动归档当前活跃文件。
2. streaming 模式：只有当 spool 文件已经生成可校验 manifest，并且策略允许 `partial` 参与恢复时，才可作为实验性能力使用；第一版不建议默认开启。

#### P2.6 前端改造

归档流列表增加：

1. 运行模式：外部、手动、追平、轮询、streaming。
2. Runner 主机。
3. 期望状态和实际状态。
4. 当前 cursor。
5. 归档延迟。
6. 最近心跳。
7. 操作：启动、暂停、恢复、停止、追平、查看事件。

归档流详情增加：

1. 源库当前 binlog 列表快照。
2. 已归档文件时间线。
3. RPO 趋势。
4. 断链诊断。
5. 与恢复计划的覆盖关系。

#### P2.6 验收标准

1. 可以把一个 MySQL/MariaDB binlog 归档流绑定到 Runner Host。
2. 可以启动、暂停、恢复、停止归档流。
3. polling 模式下可以自动持续归档已轮转 binlog。
4. Runner 失联后 stream 状态变为 degraded，并显示最近心跳和错误。
5. cursor 文件被源库 purge 时必须拒绝继续静默归档，并提示日志链断裂。
6. 成功归档的文件自动登记到 `database_log_archives`，并参与 PITR 恢复计划。
7. 归档文件有 checksum、大小、首尾事件时间、position 和链路关系。
8. request/result/config 不保存数据库密码、SSH 私钥、对象存储密钥。
9. 单元测试覆盖 cursor 选择、租约接管、purge 断链、checksum 失败、状态流转。
10. 集成测试覆盖至少 MySQL 8.0、MySQL 8.4、MariaDB 的轮询归档。

#### 2026-04-29 P2.6.1 已落地范围

本次先落地长期 binlog 归档的控制面和运行态模型，不在 backend 容器内直接运行常驻 `mysqlbinlog --stop-never`。

已实现：

1. `database_log_archive_streams` 扩展运行态字段：
   - `runner_host_id`
   - `desired_state`
   - `daemon_status`
   - `cursor_file / cursor_pos / cursor_gtid_set`
   - `active_file`
   - `last_source_file / last_source_pos`
   - `last_event_time`
   - `archive_lag_seconds`
   - `last_heartbeat_at`
   - `consecutive_failures`
   - `lease_owner / lease_expires_at`
   - `paused_at / paused_reason`
2. 新增归档模式常量：
   - `external`
   - `manual_once`
   - `catch_up`
   - `polling`
   - `streaming`
3. 新增归档流控制接口：
   - `GET /api/v1/databases/log-archive-streams/{id}/status`
   - `POST /api/v1/databases/log-archive-streams/{id}/start`
   - `POST /api/v1/databases/log-archive-streams/{id}/pause`
   - `POST /api/v1/databases/log-archive-streams/{id}/resume`
   - `POST /api/v1/databases/log-archive-streams/{id}/stop`
4. 权限边界：
   - 查看状态使用备份查看权限。
   - 启动、暂停、恢复、停止使用备份执行权限。
   - 仍沿用实例级 `DatabasePermissionBackup` 校验。
5. 启动/恢复行为：
   - 仅允许 MySQL/MariaDB binlog 归档流。
   - 必须选择已启用 Runner Host。
   - 写入 `desired_state=running`。
   - 写入 `daemon_status=starting`。
   - 保留为 `status=pending`，表示等待 Runner Agent 接管。
6. 暂停/停止行为：
   - 暂停写入 `desired_state=paused`、`daemon_status=paused`、`status=paused`。
   - 停止写入 `desired_state=stopped`、`daemon_status=stopped`。
   - 暂停和停止都会清理租约字段。
7. 前端归档流列表增加：
   - Runner 主机。
   - 期望状态。
   - 守护状态。
   - cursor。
   - 归档延迟和心跳。
   - 启动、暂停、恢复、停止按钮。
8. 前端新增启动长期归档流弹窗：
   - 选择 Runner Host。
   - 选择 `polling` 或 `streaming` 模式。
   - 明确提示 P2.6.1 只保存控制面状态，等待 Runner Agent 接管。
9. 一次性 binlog 归档和追平归档成功后会回写：
   - `cursor_file`
   - `cursor_pos`
   - `last_event_time`
   - `last_heartbeat_at`
   - `consecutive_failures=0`
   - `archive_lag_seconds=0`
10. 一次性 binlog 归档或追平归档失败后会进入 `degraded`，并增加 `consecutive_failures`。

P2.6.1 阶段未实现，保留给 P2.6.2+：

1. Runner Agent 主动拉取 stream 列表。
2. Agent lease acquire/renew/release。
3. Agent checkpoint API。
4. 常驻 polling archiver。
5. 常驻 streaming archiver。
6. 源库 binlog purge 断链自动诊断。
7. long-running archiver 事件表。
8. 对象存储 staging、checksum 后提交、spool 重试。

#### 2026-04-29 P2.6.2 已落地范围

本阶段落地 Runner Agent 接入面，让长期 binlog archiver 有稳定的后端协议。仍然不在 backend 容器中直接运行长期归档进程。

新增公开接口：

```text
POST /api/v1/public/databases/runner-agents/{runnerId}/heartbeat
GET  /api/v1/public/databases/runner-agents/{runnerId}/log-archive-streams
POST /api/v1/public/databases/runner-agents/{runnerId}/log-archive-streams/{id}/checkpoint
POST /api/v1/public/databases/runner-agents/{runnerId}/log-archives
```

鉴权规则：

1. Runner Host 的 `config_json` 必须配置 `runnerAuthSha256`。
2. Agent 请求必须带 `X-OpsHub-Runner-Auth`，或 `Authorization: Bearer <value>`。
3. 后端只比较请求值的 SHA256，不保存明文 Runner 认证值。
4. `runnerAuthSha256` 这个键名不包含 `token/secret/password`，可以通过现有敏感配置校验。
5. 如果 Runner Host 没有配置 hash，公开接口拒绝访问。

示例：

```bash
AUTH='a-long-random-runner-auth'
printf '%s' "$AUTH" | sha256sum
```

把输出的 64 位 hash 写入 Runner Host：

```json
{
  "runnerAuthSha256": "..."
}
```

Agent 请求：

```bash
curl -H "X-OpsHub-Runner-Auth: $AUTH" \
  http://opshub/api/v1/public/databases/runner-agents/runner-host-1/log-archive-streams
```

已实现：

1. Runner Agent 心跳：
   - 更新 `database_runner_hosts.last_heartbeat_at`。
   - 正常心跳把 Runner Host 标记为 `online`。
   - 失败心跳把 Runner Host 标记为 `failed` 并记录 `last_error`。
2. Agent 主动拉取归档流：
   - 仅返回绑定到当前 Runner Host 的流。
   - 仅返回 `desired_state=running`、`enabled=true`、`archive_type=binlog`、`archive_mode in (polling, streaming)` 的流。
   - 后端尝试为每个流获取或续租。
   - 租约被其他 Runner 持有且未过期时不会返回。
3. 租约规则：
   - 默认租约 TTL 为 90 秒。
   - Agent 拉取 stream 时写入 `lease_owner` 和 `lease_expires_at`。
   - 同一 Runner 可续租。
   - 租约为空、过期或属于同一 Runner 时允许接管。
4. 拉取结果：
   - 返回 stream 运行态。
   - 返回源实例基础连接元数据：实例 ID、类型、host、port、默认库。
   - 返回 Runner 的 work dir 和 storage mount。
   - 不返回数据库密码、SSH 私钥、对象存储密钥。
5. checkpoint：
   - 更新 `daemon_status`。
   - 更新 `cursor_file / cursor_pos / cursor_gtid_set`。
   - 更新 `active_file`。
   - 更新 `last_source_file / last_source_pos`。
   - 更新 `last_event_time`。
   - 更新 `archive_lag_seconds`。
   - 更新 `last_heartbeat_at`。
   - 更新并续租 `lease_expires_at`。
   - 支持 `releaseLease=true` 释放租约。
6. checkpoint 状态映射：
   - `daemon_status=running` 时 stream 进入 `running`。
   - `daemon_status=degraded` 时 stream 进入 `degraded`。
   - `daemon_status=failed` 时 stream 进入 `failed`。
   - `daemon_status=paused` 时 stream 进入 `paused`。
   - 如果 `archive_lag_seconds > rpo_target_seconds * 2`，即使 daemon 上报 running，也会标记为 `degraded`。
7. Agent 登记归档文件：
   - Agent 通过公开接口登记已完成归档的 binlog。
   - 仍复用 `database_log_archives` 元数据模型。
   - 仍要求文件名、存储 URI、首尾事件时间。
   - 登记成功后更新 stream 的最近归档、cursor 和可恢复链路元数据。

本阶段仍未实现，保留给 P2.6.3+：

1. 独立 Runner Agent 二进制中的常驻 polling archiver loop。
2. `mysqlbinlog --stop-never` streaming archiver loop。
3. Agent 侧对象存储上传、staging key、checksum 后提交。
4. Agent 侧 spool 目录重试。
5. 源库 binlog 列表校验和 purge 断链自动诊断。
6. `database_log_archive_events` 事件表。
7. 前端展示 Agent 接入命令、hash 生成和最近 Agent 事件。

#### 2026-04-29 P2.6.3 已落地范围

本阶段把 P2.6.2 的 Runner Agent 协议接入到真实 `opshub-agent` 二进制，先落地可长期运行的 Agent 侧 binlog 归档循环。关键边界保持不变：`opshub-api` backend 不运行长期 `mysqlbinlog`，数据库密码不从公开接口下发，Agent 使用本机配置中的数据库凭据执行归档。

已实现：

1. `cmd/agent` 新增可选 `databaseArchiver` 配置：

   ```json
   {
     "databaseArchiver": {
       "enabled": true,
       "baseUrl": "http://opshub.example.com",
       "runnerId": "runner-host-1",
       "runnerAuth": "本机保存的随机认证值",
       "intervalSeconds": 30,
       "leaseTtlSeconds": 90,
       "maxFilesPerLoop": 5,
       "includeCurrent": false,
       "workDir": "/var/lib/opshub-agent",
       "storageRoot": "/data/opshub-archives",
       "credentials": [
         {
           "instanceId": 1,
           "username": "repl_backup",
           "password": "只保存在 Runner Agent 本机"
         }
       ]
     }
   }
   ```

2. Agent 启动后会并行运行原有资产监控上报和数据库 binlog archiver loop。
3. Agent 每轮执行：
   - 调用 Runner Agent 心跳接口。
   - 拉取绑定给当前 Runner 的归档流。
   - 自动获取或续租 stream lease。
   - 按 stream 模式执行归档。
   - 登记已完成归档文件。
   - 回传 checkpoint、cursor、active file、source file、source pos 和错误状态。
4. `polling` 模式：
   - 连接源库执行 `SHOW BINARY LOGS` / `SHOW MASTER LOGS`。
   - 默认只选择已轮转 binlog，不归档当前活跃文件。
   - 根据 `last_archive_name` / `cursor_file` 继续追平。
   - 如果 cursor 已被源库 purge，Agent 上报 `degraded`，不静默从最新文件继续。
   - 每轮最多归档 `maxFilesPerLoop` 个文件，默认 5，最大 20。
5. `streaming` 模式第一版采用安全 spool/finalize 语义：
   - 每轮仍先归档已轮转文件。
   - 当前活跃 binlog 只下载到 `spool/<file>.partial`。
   - 活跃文件只通过 checkpoint 写入 `active_file`，不登记为 `database_log_archives`。
   - 等源库 rotate 后，上一份文件进入 finalized 并登记为 PITR 可用归档。
   - 这样可以避免把未闭合 active file 错当成完整恢复链。
6. Agent 侧本地文件布局：

   ```text
   <storageRoot>/mysql-binlog/
     instance-<instance_id>/
       stream-<stream_id>/
         finalized/
           binlog.000001
           binlog.000001.sha256
           binlog.000001.manifest.json
         spool/
           binlog.000002.partial
   ```

7. 归档文件写入规则：
   - 先下载到临时目录。
   - `sha256` 计算成功后再原子提交到 finalized。
   - 写入 `.sha256` 和 `.manifest.json` sidecar。
   - `database_log_archives.storage_uri` 使用 `runner://runner-host-<id>/<path>` 指向 Runner 本地归档。
8. Agent 侧使用 `mysqlbinlog` 或 `mariadb-binlog`：
   - `--read-from-remote-server`
   - `--raw`
   - `--result-file=<staging_dir>/`
9. Agent 本地解析 binlog 首尾事件时间：
   - 使用本机 `mysqlbinlog --base64-output=decode-rows <file>`。
   - 能解析则登记 `first_event_time / last_event_time`。
   - 解析失败时不把文件内容上传到 backend，也不泄露 binlog 内容。
10. 安全边界：
   - 后端公开接口仍只返回源实例 host/port 等非敏感元数据。
   - 数据库用户名和密码只保存在 Agent 本机配置。
   - Agent 请求 backend 时只发送 `runnerAuth` 对应的认证头和归档元数据。
   - 归档登记不包含数据库密码、连接串或临时 defaults 文件路径。
11. 新增单元测试覆盖：
   - `baseUrl` 从原 Agent `reportUrl` 推导。
   - polling 默认跳过当前活跃 binlog。
   - cursor purge 断链检测。
   - 显式允许 includeCurrent 的选择逻辑。
   - binlog 事件时间解析。
   - stream 级凭据优先于 instance 级凭据。

本阶段仍未实现，保留给 P2.6.4+：

1. 真正长连接 `mysqlbinlog --stop-never` 子进程管理。
2. streaming active spool 的增量续传和断点 resume。
3. 对象存储 S3/MinIO staging key、checksum 后提交和远端不可变保留。
4. `database_log_archive_events` 事件表。
5. 前端展示 Agent 配置生成器、runnerAuth hash 生成和最近 Agent 事件。
6. Agent 侧多 stream 并发度、带宽限制和失败退避策略。

### 2026-04-29 P2.7 详细方案：隔离恢复 Runner

目标：把 P1/P2 已有的“恢复计划和恢复证明预生成”升级为可执行的隔离恢复流程，真正把物理备份链和 binlog 归档链恢复到一个隔离 MySQL/MariaDB 实例，并执行校验 SQL。

#### P2.7 核心原则

1. 第一版只允许恢复到隔离库、临时容器或隔离目录。
2. 不支持直接覆盖生产库。
3. 不自动切换业务连接。
4. 不自动把数据回填生产库。
5. 所有恢复命令由 Runner 执行，backend 不直接操作 datadir。
6. 恢复用到的 backup、incremental、binlog、工具版本、checksum 和校验结果必须写入 proof。
7. 恢复工作目录必须可清理、可过期、可审计。

#### P2.7 需要扩展的数据模型

继续复用 `database_restore_jobs`，但需要扩展为 PITR 执行任务：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `restore_plan_id` | bigint | 关联 `database_restore_plans` |
| `runner_host_id` | bigint | 执行恢复的 Runner Host |
| `runner_job_id` | bigint | 关联底层 Runner Job |
| `restore_target_type` | varchar(30) | `time / gtid` |
| `restore_target_value` | varchar(255) | 目标时间或 GTID |
| `work_dir` | varchar(1000) | Runner 恢复工作目录 |
| `prepared_datadir` | varchar(1000) | prepare 后 datadir |
| `container_name` | varchar(255) | 临时容器名称 |
| `container_image` | varchar(255) | MySQL/MariaDB 镜像 |
| `listen_host` | varchar(255) | 隔离实例访问地址 |
| `listen_port` | int | 隔离实例端口 |
| `step_json` | text | 步骤状态和耗时 |
| `validation_json` | text | 校验 SQL 结果 |
| `proof_json` | text | 最终恢复证明 |
| `log_path` | varchar(1000) | Runner 日志路径或 URI |
| `artifact_uri` | varchar(1000) | proof、日志、manifest 打包地址 |
| `expires_at` | datetime | 临时恢复库过期时间 |
| `cleanup_status` | varchar(30) | `pending / cleaned / failed` |

`database_restore_plans` 也建议补充：

| 字段 | 说明 |
| --- | --- |
| `runner_host_id` | 推荐执行 Runner |
| `required_tool_json` | 恢复需要的工具和版本 |
| `required_artifact_json` | 备份和 binlog 对象清单 |
| `estimated_restore_bytes` | 预计恢复数据量 |
| `estimated_restore_minutes` | 粗略恢复耗时 |

#### P2.7 后端接口

新增接口：

```text
POST /api/v1/databases/restore-plans/{id}/run
GET  /api/v1/databases/restore-jobs
GET  /api/v1/databases/restore-jobs/{id}
POST /api/v1/databases/restore-jobs/{id}/cancel
POST /api/v1/databases/restore-jobs/{id}/cleanup
GET  /api/v1/databases/restore-jobs/{id}/proof
```

`POST /restore-plans/{id}/run` 请求建议：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `runnerHostId` | bigint | 是 | 执行恢复的 Runner |
| `containerImage` | string | 否 | 指定 MySQL/MariaDB 镜像，默认按源库类型和版本推断 |
| `listenPort` | int | 否 | 隔离实例端口，不填则随机分配 |
| `expiresInHours` | int | 否 | 临时恢复库保留时间 |
| `validationSql` | array | 否 | 额外校验 SQL，只允许只读语句 |
| `cleanupOnFailure` | bool | 否 | 失败后是否自动清理临时目录 |

权限建议：

| 操作 | 权限 |
| --- | --- |
| 发起隔离恢复 | `database:restore:run` |
| 查看恢复任务和 proof | `database:restore:view` |
| 清理恢复环境 | `database:restore:run` 或更高 |
| 下载 proof | `database:restore:view` |

#### P2.7 执行步骤

恢复 Runner Job 按固定步骤执行，每一步写入 `step_json`。

```text
1. lock_restore_plan
2. validate_plan_again
3. prepare_workdir
4. fetch_base_backup
5. verify_base_checksum
6. fetch_incremental_chain
7. verify_incremental_checksums
8. prepare_physical_backup
9. fetch_binlog_chain
10. verify_binlog_chain
11. create_isolated_datadir
12. start_isolated_instance
13. apply_binlog_to_target
14. run_validation_sql
15. generate_proof
16. mark_success_or_failed
```

步骤说明：

1. `validate_plan_again`：执行前必须重新校验恢复计划，因为备份文件、日志文件、对象存储和源库 purge 状态可能已经变化。
2. `prepare_workdir`：在 Runner 主机创建独立目录，例如 `/var/lib/opshub/database-runner/restore/job-<id>`。
3. `fetch_base_backup`：从 local/NFS/S3/MinIO 拉取 base backup，校验大小和 SHA256。
4. `fetch_incremental_chain`：按 `base_record_id/parent_record_id/chain_id` 顺序拉取增量备份。
5. `prepare_physical_backup`：按工具要求准备 datadir。
6. `fetch_binlog_chain`：拉取恢复目标需要的 binlog 文件，校验 checksum 和连续性。
7. `start_isolated_instance`：启动隔离 MySQL/MariaDB 实例，只暴露给 Runner 或受控网络。
8. `apply_binlog_to_target`：用 mysqlbinlog 回放到目标时间或 GTID。
9. `run_validation_sql`：执行默认和用户配置的只读校验 SQL。
10. `generate_proof`：生成最终 proof_json 和 proof 文件。

#### P2.7 物理备份 prepare 规则

XtraBackup / mariadb-backup 的物理 prepare 必须在隔离工作目录完成。

全量备份：

```text
1. 解压 full backup 到 base_dir
2. xtrabackup/mariadb-backup --prepare --target-dir=base_dir
3. prepared datadir = base_dir
```

全量 + 增量链：

```text
1. 解压 full backup 到 base_dir
2. 对 full 执行 prepare apply-log-only
3. 按顺序解压 incremental backup 到 inc_dir_1, inc_dir_2, ...
4. 对每个增量执行 prepare --incremental-dir=inc_dir_n
5. 最后一次 prepare 不再使用 apply-log-only，完成崩溃恢复
6. prepared datadir = base_dir
```

注意：

1. MySQL XtraBackup 和 MariaDB mariadb-backup 参数细节可能不同，Runner 必须按 `backup_engine/tool_name/tool_version` 选择命令模板。
2. 如果备份链缺少任一增量，不能尝试跳过，恢复任务必须失败。
3. 如果工具版本和备份元数据不兼容，恢复任务必须在 prepare 前失败。
4. prepare 后的 datadir 不得覆盖生产路径。

#### P2.7 隔离实例启动方式

第一版推荐 Docker 隔离实例：

```text
docker run --rm -d
  --name opshub-restore-<job_id>
  -v <prepared_datadir>:/var/lib/mysql
  -p 127.0.0.1:<random_port>:3306
  mysql:<matched_version>
```

MariaDB 使用匹配的 `mariadb:<version>` 镜像。

启动规则：

1. 镜像版本必须尽量匹配源库 major/minor。
2. 默认只绑定 `127.0.0.1` 或 Runner 内部网络。
3. 生成临时 root 密码，只写入 Runner 临时文件和 OpsHub secret 引用，不写入 job 明文。
4. 启动前处理 datadir ownership。
5. 失败时保留日志，按配置决定是否清理 datadir。
6. UI 必须显示“隔离库”，不能让用户误以为已恢复生产。

如果目标环境不允许 Docker：

1. P2.7 可以预留 host process 模式。
2. host process 模式必须指定 `mysqld` 路径、独立 datadir、独立 socket、独立 port 和独立 config。
3. host process 模式默认不开启，必须由管理员显式允许。

#### P2.7 binlog 回放规则

回放前置条件：

1. 恢复计划已经定位 `backup_binlog_file/backup_binlog_pos` 或 `backup_gtid_set`。
2. 所需 binlog 文件全部已归档且 checksum 通过。
3. 文件链连续。
4. GTID 模式下 GTID 范围连续。
5. source server 切换有 promotion history 或人工确认。

按时间恢复：

```text
mysqlbinlog
  --start-position=<backup_binlog_pos>
  --stop-datetime='<target_time>'
  <binlog files...> | mysql --host=127.0.0.1 --port=<isolated_port>
```

按 GTID 恢复：

```text
mysqlbinlog
  --include-gtids='<target_gtid_set>'
  <binlog files...> | mysql --host=127.0.0.1 --port=<isolated_port>
```

注意：

1. MySQL 和 MariaDB 的 GTID 语义不同，不能共用一套简单参数。
2. 第一版必须优先支持按时间恢复；GTID 恢复可以作为 P2.7 后半段增强。
3. 如果 base backup 本身已经包含某些事务，必须从备份记录的 binlog 起点开始，避免重复回放。
4. `mysqlbinlog` 回放必须记录实际使用的文件列表、start/stop 参数、退出码和 stderr 摘要。

#### P2.7 校验 SQL

默认校验：

```sql
SELECT VERSION();
SELECT @@server_uuid;
SHOW DATABASES;
```

MySQL/MariaDB 可选校验：

1. 指定库的表数量。
2. 关键表行数。
3. 指定业务 SQL 的只读查询结果。
4. 恢复目标时间附近的关键数据是否存在。

安全规则：

1. 只允许 `SELECT / SHOW / DESC / DESCRIBE / EXPLAIN`。
2. 禁止多语句。
3. 单条 SQL 必须有超时。
4. 结果只保存摘要，不保存大结果集。
5. 校验失败不一定代表恢复失败，但 proof 必须标记 `validation_status=failed`。

#### P2.7 恢复证明 proof_json

proof 必须包含：

1. `restoreJobId`
2. `restorePlanId`
3. `sourceInstanceId`
4. `targetTime` 或 `targetGtid`
5. `runnerHostId`
6. `toolVersions`
7. `baseBackup`
8. `incrementalChain`
9. `binlogChain`
10. `checksumResults`
11. `prepareSteps`
12. `applyBinlogCommandSummary`
13. `isolatedInstance`
14. `validationSql`
15. `validationResults`
16. `startedAt / finishedAt / durationMs`
17. `operator`
18. `finalStatus`

proof 文件建议同时保存：

```text
proof.json
restore.log
tool-versions.txt
selected-artifacts.manifest.json
validation-results.json
```

#### P2.7 前端改造

恢复计划列表增加：

1. `执行恢复` 按钮。
2. 推荐 Runner。
3. 预计恢复数据量。
4. 预计耗时。
5. 是否可执行。
6. 最近一次恢复任务状态。

恢复执行弹窗：

1. 选择 Runner Host。
2. 选择容器镜像或使用自动推荐。
3. 选择临时库保留时间。
4. 填写额外校验 SQL。
5. 风险确认：只恢复到隔离库，不覆盖生产。

恢复任务详情：

1. 步骤时间线。
2. 当前日志摘要。
3. 已下载 artifacts。
4. 隔离库连接信息。
5. 校验 SQL 结果。
6. proof 下载。
7. 清理按钮。

#### P2.7 安全边界

1. 不允许选择生产实例作为直接覆盖目标。
2. 不允许 Runner 使用生产 datadir 路径作为恢复目录。
3. Docker 端口默认只绑定本机。
4. 临时 root 密码必须自动生成并脱敏。
5. 恢复任务所有命令必须来自白名单模板。
6. 用户输入只允许出现在受控字段中，不拼接任意 shell。
7. 恢复环境必须有过期时间和清理动作。
8. 清理动作也必须审计。

#### P2.7 验收标准

1. 可以从一个预校验通过的 MySQL/MariaDB 恢复计划发起隔离恢复任务。
2. Runner 能拉取 base backup、增量链和 binlog 链，并逐项 checksum 校验。
3. 缺少任一备份或 binlog 时任务失败，错误信息能指向缺失对象。
4. Runner 能 prepare 物理备份链。
5. Runner 能启动隔离 MySQL/MariaDB 容器。
6. Runner 能按目标时间回放 binlog。
7. 校验 SQL 能执行并保存摘要。
8. proof_json 包含 base backup、incremental chain、binlog chain、checksum、工具版本、校验 SQL 和最终状态。
9. 前端能查看恢复步骤、状态、错误、隔离库连接信息和 proof。
10. 恢复环境能手动清理，并在过期后提示清理。
11. 单元测试覆盖计划状态判断、命令模板生成、危险路径拒绝、校验 SQL 白名单、proof 字段。
12. 集成测试至少覆盖 MySQL 8.0 full restore、MySQL 8.0 full+incremental restore、MariaDB full restore。

## 文档定位

本文是 OpsHub 数据库管理模块在“大库备份、日志归档、延迟副本、PITR 恢复演练”方向的长期改造基准。后续分期实施、表结构扩展、接口设计、前端页面、Runner 执行边界、权限和验收标准均以本文为准。

本文不替代现有数据库管理总路线文档，而是专门约束备份恢复方向：

1. 现有逻辑备份继续保留，用于小库、迁移、临时导出和对象级恢复辅助。
2. 大库生产备份不再以“每天凌晨逻辑全量”为主方案。
3. 大库长期方案以“物理备份链 + binlog/WAL 连续归档 + 恢复计划校验 + 隔离库恢复演练”为核心。
4. 实时从库和延迟从库纳入灾备视图，但不把从库当成长期备份。
5. OpsHub 做编排、审计、元数据、链路校验、可视化和恢复证明，不自创数据库备份格式。

## 当前代码基线

当前实现已经具备备份任务、备份记录、下载、校验和恢复演练的基础闭环，但仍属于逻辑备份能力。

关键代码位置：

1. 后端模型：`internal/biz/database/model.go`
2. 备份任务管理：`internal/biz/database/backup.go`
3. 备份执行：`internal/biz/database/backup_execution.go`
4. 备份命令构造：`internal/biz/database/backup_runtime.go`
5. 备份调度器：`internal/biz/database/backup_scheduler.go`
6. 文件完整性校验：`internal/biz/database/backup_integrity.go`
7. 恢复演练：`internal/biz/database/restore.go`
8. 恢复命令构造：`internal/biz/database/restore_runtime.go`
9. 前端入口：`web/src/views/asset/DatabaseManagement.vue`
10. 前端 API：`web/src/api/database.ts`

当前能力：

1. MySQL / MariaDB 使用 `mysqldump` 或 `mariadb-dump` 生成 `.sql.gz`。
2. PostgreSQL 使用 `pg_dump`，支持 plain SQL 和 custom archive 两种逻辑格式。
3. Redis 当前也接入备份，但本文后续改造先不展开 Redis。
4. 备份文件默认落到本地目录 `./data/database-backups`。
5. `storage_config` 当前只是任务字段，不代表对象存储和密钥已经真正接入。
6. 恢复演练基于单个逻辑备份文件，只允许非生产目标实例。
7. 运行中备份记录存在固定 6 小时 stale 判定，大库场景可能误判。
8. 调度器按 Cron 拉起任务，缺少全局并发、单实例并发、Runner 资源和存储带宽治理。

当前主要缺口：

1. 没有物理全量备份。
2. 没有物理增量或差异备份。
3. 没有 MySQL/MariaDB binlog 连续归档链。
4. 没有 PostgreSQL WAL 连续归档链。
5. 没有 PITR 恢复计划和目标时间点恢复。
6. 没有备份链、日志链、GTID、LSN、timeline 连续性检查。
7. 没有备份工具版本兼容矩阵。
8. 没有对象存储、异地、不可变保留、加密和密钥托管闭环。
9. 没有延迟副本状态、暂停 apply 指引和误删事故剧本。
10. 备份成功不等于可恢复，当前页面还不能直接回答“能恢复到哪个时间点”。

## 事实依据和约束

以下事实作为设计约束：

1. MySQL PITR 的信息来源是全量备份之后生成的 binary log，所以只做全量或增量备份不能恢复到任意时间点。参考：<https://dev.mysql.com/doc/refman/8.4/en/point-in-time-recovery-binlog.html>
2. MySQL delayed replication 可以通过 `SOURCE_DELAY` 让副本延迟执行事务，可用于防源库误操作，但它不是长期备份。参考：<https://dev.mysql.com/doc/refman/8.4/en/replication-delayed.html>
3. PostgreSQL PITR 需要 base backup 和从 base backup 起连续可用的 WAL 文件链。参考：<https://www.postgresql.org/docs/current/continuous-archiving.html>
4. PostgreSQL `pg_basebackup` 是 cluster 级物理备份，不能单独备份某个 database、schema 或 table；选择性对象备份仍需 `pg_dump`。参考：<https://www.postgresql.org/docs/current/app-pgbasebackup.html>
5. PostgreSQL 原生增量备份不能直接恢复，需要 `pg_combinebackup` 将依赖链合成可恢复的 synthetic full backup。参考：<https://www.postgresql.org/docs/current/app-pgbasebackup.html> 和 <https://www.postgresql.org/docs/current/app-pgcombinebackup.html>
6. Percona XtraBackup 存在强版本兼容约束，例如 XtraBackup 8.4 只支持 MySQL/Percona 8.4 系，不支持 MySQL 8.0 或 9.x。参考：<https://docs.percona.com/percona-server/8.4/8.4-compatibility-and-removed-items.html>
7. Barman 适合 PostgreSQL 集中备份管理和恢复窗口保留策略，可保留 base backup 与 WAL 以满足恢复窗口。参考：<https://docs.pgbarman.org/release/3.17.0/user_guide/retention_policies.html>
8. WAL-G 可作为 PostgreSQL、MySQL/MariaDB 等数据库的云端归档恢复工具，适合对象存储场景。参考：<https://wal-g.readthedocs.io/PostgreSQL/>
9. pgBackRest 官方站点已在 2026-04-27 标注项目不再维护。新接入 PostgreSQL 不以 pgBackRest 作为默认推荐引擎；已有 pgBackRest 链路仅按 legacy external 接入。参考：<https://pgbackrest.org/>

## 总体结论

大库生产场景不能继续依赖“每天凌晨全量逻辑备份”作为主备份方案。最终架构应是：

```text
生产主库
  ├─ 实时从库 / 热备库
  │    └─ 用于故障切换、读扩展、备份卸载
  ├─ 延迟从库
  │    └─ 延迟 1-6 小时，用于误删误更新后的快速截停
  └─ 备份仓库
       ├─ 物理全量备份
       ├─ 物理增量 / 差异备份
       ├─ binlog / WAL 连续归档
       ├─ 异地 / 对象存储 / 不可变保留 / 加密
       └─ 定期恢复演练和恢复证明
```

核心原则：

1. 实时从库解决主库坏了能不能尽快顶上，属于 HA 和低 RTO。
2. 延迟从库解决刚发生的误删误更新能不能快速截停，属于短窗口防误操作。
3. 物理备份链和 binlog/WAL 归档解决延迟发现后能不能回到指定时间点，属于备份和 PITR。
4. 三者不能互相替代。
5. 所有恢复必须先到隔离库，校验通过后再决定导回数据或整体切换。

## 能力分层

OpsHub 后续把备份能力分为 5 层。

### L1 逻辑备份

适用场景：

1. 小库。
2. 临时导出。
3. 迁移。
4. 对象级恢复辅助。
5. 测试环境备份。

限制：

1. 不支持 PITR。
2. 大库备份和恢复耗时长。
3. 对复杂对象、权限、函数、触发器兼容性依赖工具参数。
4. 不能作为核心大库唯一备份。

### L2 物理备份

适用场景：

1. 大库主备份。
2. 快速恢复。
3. 保留数据文件级一致快照。

候选工具：

1. MySQL：Percona XtraBackup 或 MySQL Enterprise Backup。
2. MariaDB：mariadb-backup 或 MariaDB Enterprise Backup。
3. PostgreSQL：Barman、WAL-G、pg_basebackup。

### L3 日志归档

适用场景：

1. PITR。
2. 降低 RPO。
3. 证明备份链之后的数据变更可重放。

日志类型：

1. MySQL/MariaDB：binlog。
2. PostgreSQL：WAL。

重要约束：

1. binlog/WAL 不是普通 backup method。
2. binlog/WAL 必须单独建归档流和归档文件记录。
3. 归档要尽量连续、低延迟、可校验、独立存储。

### L4 副本治理

适用场景：

1. 实时从库用于 HA、读扩展和备份卸载。
2. 延迟从库用于误操作保护窗口。

首版边界：

1. 先监控和展示状态。
2. 提供事故操作指引。
3. 暂停 apply 这类高风险动作先做命令建议和审计预留，不默认自动执行。

### L5 PITR 恢复演练

适用场景：

1. 恢复到目标时间。
2. 恢复到目标 LSN。
3. 恢复到目标 GTID。
4. 恢复到 named restore point。
5. 生成恢复证明。

必须具备：

1. 自动选择 base backup。
2. 自动选择增量链。
3. 自动选择 binlog/WAL 区间。
4. 校验链路完整性。
5. 校验存储对象和 checksum。
6. 校验工具版本。
7. 拉起隔离恢复库。
8. 执行业务校验。
9. 记录恢复证明。

## 关键概念收敛

### 不把 binlog/WAL 放进 backup_method

错误模型：

```text
backup_method = logical / physical / binlog / wal / external
backup_level = full / incremental / log_archive
```

目标模型：

```text
backup_method = logical / physical / external
backup_level = full / incremental / differential
archive_type = none / binlog / wal
```

原因：

1. `logical`、`physical` 描述备份方法。
2. `full`、`incremental`、`differential` 描述备份层级。
3. `binlog`、`wal` 描述恢复日志归档链，不是备份方法。
4. 恢复计划需要表达“一个 base backup + 若干 incremental backups + 一段连续日志 = 目标时间点恢复”。

### 逻辑备份和物理备份的范围不同

逻辑备份：

1. 可以按 database、schema、table 等对象选择。
2. 适合对象级导出和回填。
3. 恢复速度通常慢于物理备份。

PostgreSQL 物理备份：

1. 是 database cluster 级别。
2. 不承诺 schema/table 级物理恢复。
3. 对象级恢复应先恢复到隔离库，再用逻辑导出回填。

MySQL/MariaDB 物理备份：

1. 主要是实例/datadir 级别。
2. 可以研究部分恢复，但第一版不承诺宽泛对象级物理恢复。
3. 对象级恢复同样先恢复到隔离库再导出回填。

### 备份成功不等于可恢复

页面和 API 不应只展示“备份成功”。更关键的状态是：

1. PITR 是否支持。
2. 理论可恢复窗口。
3. 最近成功备份。
4. 最近日志归档时间。
5. 最近恢复演练时间。
6. 最近恢复演练结果。
7. 当前链路是否有缺口。

建议能力字段：

```text
restore_capability:
  none
  logical_restore_only
  physical_restore
  pitr_capable
  pitr_verified

recoverable_from
recoverable_until
last_restore_test_at
last_restore_test_status
```

## 数据模型设计

### 扩展 `database_backup_tasks`

保留现有字段并新增：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `backup_method` | varchar(30) | `logical / physical / external` |
| `backup_level` | varchar(30) | `full / incremental / differential` |
| `backup_engine` | varchar(60) | `mysqldump / pg_dump / xtrabackup / mariadb_backup / barman / walg / pg_basebackup / external` |
| `source_instance_id` | bigint | 实际执行备份的源实例，可为主库、从库或延迟从库 |
| `source_role` | varchar(30) | `primary / replica / delayed_replica / external` |
| `storage_profile_id` | bigint | 存储配置引用 |
| `secret_profile_id` | bigint | 密钥配置引用 |
| `backup_scope` | varchar(30) | `instance / cluster / database / schema / table` |
| `scope_config` | text | 范围配置 JSON |
| `rpo_minutes` | int | 目标 RPO |
| `rto_minutes` | int | 目标 RTO |
| `max_duration_minutes` | int | 最大允许运行时长，替代固定 6 小时 |
| `compression` | varchar(30) | `none / gzip / zstd / lz4` |
| `encryption_enabled` | bool | 是否加密 |
| `next_run_at` | datetime | 下次预计运行时间 |
| `last_success_at` | datetime | 最近成功备份时间 |
| `last_restore_test_at` | datetime | 最近恢复演练时间 |
| `restore_capability` | varchar(30) | 当前任务能提供的恢复能力 |

兼容策略：

1. 老任务 `backup_type=logical` 映射为 `backup_method=logical`、`backup_level=full`。
2. PostgreSQL `logical_custom` 仍是 `backup_method=logical`，只是 `backup_format=custom`。
3. 现有 `storage_type=local` 迁移到默认 local storage profile。
4. 老任务默认 `restore_capability=logical_restore_only`。

### 扩展 `database_backup_records`

保留现有字段并新增：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `chain_id` | varchar(64) | 备份链 ID |
| `base_record_id` | bigint | 所属 full backup |
| `parent_record_id` | bigint | 直接依赖的上一备份 |
| `backup_method` | varchar(30) | `logical / physical / external` |
| `backup_level` | varchar(30) | `full / incremental / differential` |
| `backup_engine` | varchar(60) | 备份引擎 |
| `tool_name` | varchar(60) | 实际工具名 |
| `tool_version` | varchar(120) | 实际工具版本 |
| `source_instance_id` | bigint | 备份来源实例 |
| `source_role` | varchar(30) | 来源角色 |
| `storage_profile_id` | bigint | 存储配置 |
| `storage_uri` | varchar(1000) | 对象存储或本地 URI |
| `manifest_json` | text | 工具 manifest 或 OpsHub 采集摘要 |
| `prepare_status` | varchar(30) | 物理备份 prepare 状态 |
| `recoverable_from` | datetime | 理论恢复窗口起点 |
| `recoverable_until` | datetime | 理论恢复窗口终点 |

MySQL/MariaDB 追加：

| 字段 | 说明 |
| --- | --- |
| `server_uuid` | 来源 server UUID |
| `server_id` | 来源 server_id |
| `gtid_mode` | GTID 模式 |
| `executed_gtid_set` | 备份时 executed GTID |
| `purged_gtid_set` | 备份时 purged GTID |
| `binlog_format` | binlog_format |
| `binlog_row_image` | binlog_row_image |
| `backup_binlog_file` | 备份对应起点 binlog file |
| `backup_binlog_pos` | 备份对应起点 position |
| `backup_gtid_set` | 备份对应 GTID set |
| `promotion_history` | 主从切换摘要 JSON |

PostgreSQL 追加：

| 字段 | 说明 |
| --- | --- |
| `pg_system_identifier` | PostgreSQL system identifier |
| `timeline_id` | timeline |
| `timeline_history_file` | timeline history 文件名或 URI |
| `wal_segment_size` | WAL segment size |
| `start_lsn` | 备份起始 LSN |
| `end_lsn` | 备份结束 LSN |
| `wal_start` | 起始 WAL segment |
| `wal_end` | 结束 WAL segment |
| `backup_label_json` | backup_label 摘要 |
| `backup_manifest_checksum` | manifest 校验摘要 |

建议索引：

1. `(instance_id, created_at)`
2. `(chain_id, created_at)`
3. `(base_record_id)`
4. `(source_instance_id, created_at)`
5. `(backup_method, backup_level, status)`

### 新增 `database_log_archive_streams`

用途：记录 binlog/WAL 连续归档配置。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `instance_id` | bigint | 归档所属主实例 |
| `source_instance_id` | bigint | 实际拉取日志的实例 |
| `engine` | varchar(30) | `mysql / mariadb / postgresql` |
| `archive_type` | varchar(30) | `binlog / wal` |
| `archive_mode` | varchar(30) | `daemon / high_frequency_poll / archive_command / streaming / external` |
| `archive_engine` | varchar(60) | `mysqlbinlog / mariadb-binlog / barman / walg / pg_receivewal / external` |
| `storage_profile_id` | bigint | 存储配置 |
| `secret_profile_id` | bigint | 密钥配置 |
| `rpo_target_seconds` | int | 目标 RPO |
| `retention_days` | int | 日志保留天数 |
| `enabled` | bool | 是否启用 |
| `status` | varchar(30) | `pending / running / degraded / failed / disabled` |
| `last_archived_at` | datetime | 最近归档成功时间 |
| `last_archive_name` | varchar(255) | 最近归档文件 |
| `last_error` | varchar(1000) | 最近错误 |
| `config_json` | text | 工具配置摘要，不保存明文密钥 |

### 新增 `database_log_archives`

用途：记录每个已归档 binlog/WAL 文件或 segment。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `stream_id` | bigint | 归档流 ID |
| `instance_id` | bigint | 主实例 ID |
| `source_instance_id` | bigint | 来源实例 ID |
| `engine` | varchar(30) | 数据库类型 |
| `archive_type` | varchar(30) | `binlog / wal` |
| `file_name` | varchar(255) | 文件名 |
| `storage_uri` | varchar(1000) | 存储地址 |
| `file_size` | bigint | 文件大小 |
| `checksum_sha256` | varchar(64) | 文件校验 |
| `first_event_time` | datetime | 文件内首个事件时间 |
| `last_event_time` | datetime | 文件内最后事件时间 |
| `status` | varchar(30) | `archived / missing / checksum_failed / expired` |
| `archived_at` | datetime | 归档时间 |

MySQL/MariaDB 追加：

| 字段 | 说明 |
| --- | --- |
| `server_uuid` | 来源 server UUID |
| `server_id` | 来源 server_id |
| `start_pos` | 起始 position |
| `end_pos` | 结束 position |
| `start_gtid_set` | 起始 GTID |
| `end_gtid_set` | 结束 GTID |
| `previous_file_name` | 上一个 binlog 文件 |
| `next_file_name` | 下一个 binlog 文件 |

PostgreSQL 追加：

| 字段 | 说明 |
| --- | --- |
| `pg_system_identifier` | system identifier |
| `timeline_id` | timeline |
| `start_lsn` | 起始 LSN |
| `end_lsn` | 结束 LSN |
| `segment_no` | segment 序号 |
| `timeline_history_uri` | timeline history URI |

建议索引：

1. `(stream_id, file_name)`
2. `(instance_id, archive_type, first_event_time, last_event_time)`
3. `(instance_id, archive_type, status)`
4. MySQL：`(server_uuid, file_name)`
5. PostgreSQL：`(pg_system_identifier, timeline_id, start_lsn)`

### 新增 `database_restore_plans`

用途：恢复前生成计划，执行前做链路校验，执行后生成恢复证明。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `source_instance_id` | bigint | 来源实例 |
| `target_instance_id` | bigint | 目标隔离实例或恢复环境 |
| `restore_mode` | varchar(30) | `dry_run / isolated_restore / production_cutover` |
| `restore_target_type` | varchar(30) | `time / lsn / xid / gtid / restore_point / latest` |
| `restore_target_value` | varchar(255) | 目标值 |
| `restore_target_inclusive` | bool | 是否包含目标事件 |
| `selected_base_record_id` | bigint | 选择的 base backup |
| `selected_backup_record_ids` | text | 选择的增量链 JSON |
| `selected_log_archive_ids` | text | 选择的日志文件 JSON |
| `backup_chain_status` | varchar(30) | 备份链状态 |
| `log_chain_status` | varchar(30) | 日志链状态 |
| `storage_status` | varchar(30) | 存储状态 |
| `tool_status` | varchar(30) | 工具兼容状态 |
| `validation_status` | varchar(30) | 计划校验状态 |
| `restore_status` | varchar(30) | 恢复执行状态 |
| `plan_json` | text | 完整计划 |
| `proof_json` | text | 恢复证明 |
| `operator_id` | bigint | 操作人 |
| `operator_name` | varchar(120) | 操作人 |
| `started_at` | datetime | 开始时间 |
| `finished_at` | datetime | 完成时间 |
| `duration_ms` | bigint | 耗时 |
| `error_message` | varchar(1000) | 错误 |

状态枚举建议：

```text
backup_chain_status:
  complete
  missing_base
  missing_incremental
  broken_chain
  unsupported

log_chain_status:
  complete
  missing_binlog
  missing_wal
  timeline_gap
  gtid_gap
  time_range_gap
  unsupported

storage_status:
  available
  missing_object
  checksum_failed
  permission_denied
  unsupported

tool_status:
  compatible
  incompatible_version
  missing_tool
  permission_denied
  unsupported

validation_status:
  pending
  passed
  warning
  failed

restore_status:
  planned
  queued
  running
  restored
  verified
  failed
  cancelled
```

### 新增 `database_storage_profiles`

用途：统一管理备份存储，不再把密钥塞到 `storage_config`。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `name` | varchar(120) | 名称 |
| `storage_type` | varchar(30) | `local / nfs / s3 / minio / oss / cos / external` |
| `endpoint` | varchar(255) | 端点 |
| `bucket` | varchar(255) | bucket |
| `region` | varchar(120) | region |
| `path_prefix` | varchar(500) | 路径前缀 |
| `secret_profile_id` | bigint | 密钥引用 |
| `versioning_enabled` | bool | 是否启用版本化 |
| `immutability_enabled` | bool | 是否启用不可变保留 |
| `kms_key_id` | varchar(255) | KMS key |
| `retention_lock_days` | int | 锁定保留天数 |
| `status` | varchar(30) | 状态 |
| `last_test_at` | datetime | 最近测试时间 |

### 新增 `database_secret_profiles`

用途：保存备份系统需要的密钥引用，不保存明文。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `name` | varchar(120) | 名称 |
| `secret_type` | varchar(30) | `db_credential / ssh_key / object_storage_key / encryption_key / external_ref` |
| `credential_id` | bigint | 复用现有凭据 ID |
| `external_ref` | varchar(500) | 外部密钥系统引用 |
| `status` | varchar(30) | 状态 |
| `last_rotated_at` | datetime | 最近轮换时间 |

### 新增 `database_runner_jobs`

用途：记录由 Runner 执行的长任务。

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | bigint | 主键 |
| `job_type` | varchar(60) | `backup / log_archive / restore / verify / replica_check` |
| `runner_id` | varchar(120) | Runner 标识 |
| `source_instance_id` | bigint | 来源实例 |
| `target_instance_id` | bigint | 目标实例 |
| `status` | varchar(30) | `queued / running / success / failed / cancelled` |
| `allowed_command` | varchar(120) | 命令类别 |
| `request_json` | text | 下发请求 |
| `result_json` | text | 回传结果 |
| `heartbeat_at` | datetime | 最近心跳 |
| `started_at` | datetime | 开始时间 |
| `finished_at` | datetime | 结束时间 |
| `duration_ms` | bigint | 耗时 |
| `error_message` | varchar(1000) | 错误 |

## Runner 执行边界

不建议让 backend 容器直接长期执行所有物理备份命令。原因：

1. XtraBackup、mariadb-backup、Barman、WAL-G 往往需要跑在数据库主机、从库、备份服务器或能访问数据目录的主机上。
2. 大库备份可能持续数小时，HTTP 请求和 backend 进程不应承担执行生命周期。
3. 备份命令涉及高权限、文件系统、对象存储密钥、带宽和 IO 限制。
4. 备份系统本身是高价值攻击面，需要命令白名单和执行策略。

目标架构：

```text
OpsHub Backend
  ├─ 保存策略、任务、记录、审计、恢复计划
  ├─ 校验权限和风险
  ├─ 下发 Runner Job
  └─ 接收 Runner 心跳和结果

OpsHub Runner / Agent / SSH Runner
  ├─ 安装数据库备份工具
  ├─ 执行白名单命令
  ├─ 控制 CPU / IO / 带宽 / 超时
  ├─ 上传备份文件和日志文件
  └─ 回传 manifest、checksum、工具版本和状态
```

Runner 策略字段：

```text
allowed_commands
allowed_hosts
allowed_storage_profiles
cpu_limit
io_limit
bandwidth_limit
timeout_minutes
max_parallel_jobs
working_directory
```

首版可以先支持 SSH Runner 或本地 Runner，但模型要为 Agent 化预留。

## MySQL / MariaDB 设计

### 备份工具选择

MySQL：

1. 默认开源路线：Percona XtraBackup。
2. 企业路线：MySQL Enterprise Backup，作为 external/enterprise engine 预留。

MariaDB：

1. 默认：mariadb-backup。
2. 企业版：MariaDB Enterprise Backup，作为 external/enterprise engine 预留。

### 版本兼容矩阵

OpsHub 创建物理备份任务前必须检测：

1. 数据库类型：MySQL / Percona Server / MariaDB。
2. 数据库版本：5.7 / 8.0 / 8.4 / 9.x / MariaDB 版本。
3. 存储引擎：InnoDB / MyISAM / MyRocks / mixed。
4. 备份工具：xtrabackup 2.4 / 8.0 / 8.4 / mariadb-backup。
5. GTID 模式和 binlog 配置。

建议兼容策略：

| 数据库 | 推荐工具 | 备注 |
| --- | --- | --- |
| MySQL 5.7 | XtraBackup 2.4 legacy | 属于老版本兼容，需标注生命周期风险 |
| MySQL 8.0.x | XtraBackup 8.0 | 不能混用 XtraBackup 8.4 |
| MySQL 8.4.x | XtraBackup 8.4 | 只支持 8.4 系 |
| MySQL 9.x | 暂不默认推荐 XtraBackup | 使用 external/enterprise，待工具明确支持 |
| Percona Server 8.0 | XtraBackup 8.0 | 按 Percona 版本匹配 |
| Percona Server 8.4 | XtraBackup 8.4 | 按 Percona 版本匹配 |
| MariaDB | mariadb-backup | 不使用 XtraBackup 作为默认 |

非 InnoDB 风险：

1. MyISAM 等非事务表需要额外锁策略。
2. mixed engine 需要 UI 强提示。
3. 首版物理备份可以只标注风险，不自动优化复杂锁策略。

### 备份链

推荐策略：

```text
每周 1 次 physical full
每天 1 次 physical incremental
高变更库每 6 小时或每小时增量
binlog 准实时连续归档
从库或备份专用副本执行物理备份
```

备份记录必须记录：

1. `backup_binlog_file`
2. `backup_binlog_pos`
3. `backup_gtid_set`
4. `server_uuid`
5. `executed_gtid_set`
6. `purged_gtid_set`
7. `tool_name`
8. `tool_version`
9. `manifest_json`
10. `checksum`

### binlog 归档

binlog 归档不能只做每天一次 Cron。目标是连续或高频归档。

归档方式：

1. `mysqlbinlog --read-from-remote-server` 持续拉取。
2. Runner 高频检查新 binlog 并上传。
3. 由外部工具或已有备份系统归档，OpsHub 登记元数据。

归档记录必须能回答：

1. 当前最新归档到哪个 binlog 文件和位置。
2. 当前最新归档到哪个 GTID。
3. 归档延迟是多少。
4. 是否跨过主从切换。
5. GTID 是否连续。
6. 是否有文件缺失。
7. 目标时间点是否在可恢复窗口内。

### PITR 恢复流程

```text
1. 用户选择实例和目标时间 / GTID。
2. OpsHub 自动选择目标时间之前最近的 full backup。
3. OpsHub 按 chain_id 选择依赖的 incremental backups。
4. OpsHub 根据 backup_binlog_file/binlog_pos 或 backup_gtid_set 定位日志起点。
5. OpsHub 选择目标时间或 GTID 之前需要的 binlog 文件。
6. 校验 binlog 文件连续性、GTID 连续性、server_uuid 和 promotion history。
7. Runner 准备物理备份。
8. Runner 拉起隔离 MySQL/MariaDB 实例。
9. Runner 使用 mysqlbinlog 应用日志到目标时间或 GTID。
10. OpsHub 执行业务校验 SQL。
11. 生成恢复证明。
12. 用户决定导出回填或整体切换。
```

## PostgreSQL 设计

### 工具选择

新接入默认建议：

1. 第一优先：Barman 深接入。
2. 第二优先：WAL-G，偏对象存储和云原生。
3. 轻量备用：pg_basebackup + WAL archive。
4. legacy external：pgBackRest，仅登记已有链路，不默认推荐新建。

不要第一版同时深做 Barman、WAL-G、pg_basebackup 三套。第一版 PostgreSQL 物理备份建议深接 Barman，因为它更适合集中管理、catalog、retention policy 和恢复计划。

### PostgreSQL 物理备份范围

必须在前端和 API 明确：

1. PostgreSQL 物理备份是 cluster 级。
2. 不能选择单个 database/schema/table 做物理备份。
3. 如果用户需要对象级备份，使用 `pg_dump` 逻辑备份。
4. 如果用户需要对象级恢复，先 PITR 到隔离库，再逻辑导出所需对象回填。

### Barman 接入

OpsHub 负责：

1. 登记 Barman server。
2. 读取 Barman backup catalog。
3. 读取 WAL 归档状态。
4. 展示 retention policy。
5. 发起恢复计划。
6. 记录恢复证明。

Barman Runner 负责：

1. 执行 `barman check`。
2. 执行 `barman backup`。
3. 执行 `barman list-backups` / `show-backup`。
4. 执行 `barman restore` 到隔离目录。
5. 回传 backup ID、WAL 范围、timeline、LSN、manifest。

### WAL-G 接入

WAL-G 更适合：

1. S3 / MinIO / OSS 对象存储。
2. 压缩、加密、远端 push/fetch。
3. 不希望维护较重备份服务器的部署。

首版建议：

1. 先做 external metadata registration。
2. 后续再深接 `backup-push`、`backup-fetch`、`wal-push`、`wal-fetch`。

### pg_basebackup 接入

pg_basebackup 作为轻量原生方案：

1. 适合先打通简单 base backup + WAL archive。
2. catalog、retention、对象存储、恢复编排需要 OpsHub 自己补。
3. 原生 incremental 必须按目标 PostgreSQL 版本和客户端工具版本门控。
4. 使用 incremental 时，恢复计划必须自动加入 `pg_combinebackup` 步骤。
5. 任一依赖备份缺失，不允许发起恢复。

### WAL 归档

WAL 归档方式：

1. `archive_command`。
2. Barman WAL streaming。
3. WAL-G `wal-push`。
4. `pg_receivewal`。
5. external。

RPO 提示：

1. RPO 大于 15 分钟：`archive_command` 可以作为基础方案。
2. RPO 小于等于 5 分钟：提示优先 WAL streaming、Barman streaming 或 pg_receivewal。
3. RPO 小于等于 1 分钟：必须强提示单纯等待 WAL segment 切换可能不满足目标。

### PostgreSQL PITR 恢复流程

```text
1. 用户选择实例和 target time / target lsn / target xid / restore point。
2. OpsHub 自动选择可覆盖目标的 base backup。
3. 如使用原生增量，按顺序选择 full + incrementals。
4. 校验 pg_system_identifier。
5. 校验 timeline 和 timeline history。
6. 校验 WAL segment 连续性。
7. 校验 storage object 和 checksum。
8. 校验工具版本和恢复目标类型。
9. Runner 恢复 base backup。
10. 如有原生增量，先执行 pg_combinebackup。
11. 配置 restore_command 和 recovery_target_*。
12. 启动隔离 PostgreSQL 实例。
13. 等待恢复到目标点。
14. 执行业务校验 SQL。
15. 生成恢复证明。
```

## 延迟副本治理

延迟副本不是长期备份，但对刚发生的误删误更新非常有价值。

目标能力：

1. 发现实时从库和延迟从库。
2. 展示复制状态。
3. 展示配置延迟。
4. 展示 remaining delay。
5. 展示 replay/apply 进度。
6. 展示 relay log 或 WAL 积压大小。
7. 提供事故指引。

事故指引首版：

```text
1. 选择疑似受影响主库。
2. 选择延迟副本。
3. 显示当前 replay/apply 时间和 remaining delay。
4. 提示立即暂停 apply 的命令。
5. 要求填写事故原因。
6. 记录审计。
7. 提供从延迟库导出缺失数据的建议。
```

首版不默认自动执行暂停 apply。后续如果要执行，需要新增独立权限、二次确认、审批和回滚指引。

## 恢复计划校验

恢复计划必须先校验再执行。

### 通用校验

1. 目标恢复时间是否在可恢复窗口内。
2. 是否存在可用 base backup。
3. 增量链是否完整。
4. 日志链是否完整。
5. 存储对象是否存在。
6. checksum 是否匹配。
7. 加密密钥是否可用。
8. 备份工具是否安装。
9. 工具版本是否兼容。
10. 目标恢复环境是否非生产。
11. 目标环境磁盘容量是否足够。

### MySQL/MariaDB 校验

1. `server_uuid` 是否匹配或 promotion history 是否能解释切换。
2. `backup_binlog_file` 和 `backup_binlog_pos` 是否可定位。
3. GTID 是否连续。
4. 是否存在 purged gap。
5. binlog format 是否满足恢复预期。
6. binlog 时间范围是否覆盖目标时间。
7. XtraBackup/mariadb-backup 版本是否兼容。

### PostgreSQL 校验

1. `pg_system_identifier` 是否一致。
2. timeline 是否正确。
3. timeline history 是否存在。
4. WAL segment 是否连续。
5. LSN 范围是否覆盖目标。
6. 原生增量依赖链是否完整。
7. 是否需要 `pg_combinebackup`。
8. target type 是否被当前引擎支持。

## 前端改造

### 备份首页

新增“备份与恢复能力总览”：

1. 当前策略：逻辑全量 / 物理全量 / 物理增量 / PITR。
2. PITR 状态：不支持 / 支持 / 已演练。
3. 可恢复窗口：`recoverable_from` 到 `recoverable_until`。
4. 最近成功备份。
5. 最近日志归档。
6. 最近恢复演练。
7. 当前风险。
8. 推荐动作。

示例：

```text
当前策略：逻辑全量
PITR：不支持
最近成功备份：2026-04-28 02:00
最近恢复演练：无
风险：当前库容量较大，不建议使用每日逻辑全量作为主备份方案
```

### 任务创建向导

步骤：

1. 选择实例。
2. 显示容量、环境、数据库类型、版本、复制角色。
3. 选择策略：
   - 小库逻辑备份
   - 大库物理备份
   - 外部备份接入
4. 选择来源：
   - 主库
   - 实时从库
   - 备份专用从库
5. 选择 RPO/RTO。
6. 选择存储 profile。
7. 选择密钥 profile。
8. 版本兼容检查。
9. 生成任务。

### 日志归档页面

展示：

1. 归档流状态。
2. 归档延迟。
3. 最近归档文件。
4. 最近归档时间。
5. 当前 RPO 估算。
6. 文件缺口。
7. checksum 异常。

### PITR 恢复页面

流程：

1. 选择来源实例。
2. 选择恢复目标类型和值。
3. 点击生成恢复计划。
4. 展示选中的 base backup、incremental chain、binlog/WAL 文件。
5. 展示链路校验结果。
6. 选择隔离恢复目标。
7. 发起恢复演练。
8. 展示恢复日志和校验结果。
9. 生成恢复证明。

## API 设计草案

备份能力：

```text
GET  /api/v1/databases/instances/{id}/backup-capability
GET  /api/v1/databases/instances/{id}/recoverability
POST /api/v1/databases/instances/{id}/backup-compatibility-check
```

备份任务：

```text
GET    /api/v1/databases/backup-tasks
POST   /api/v1/databases/backup-tasks
PUT    /api/v1/databases/backup-tasks/{id}
DELETE /api/v1/databases/backup-tasks/{id}
POST   /api/v1/databases/backup-tasks/{id}/run
POST   /api/v1/databases/backup-tasks/{id}/disable
POST   /api/v1/databases/backup-tasks/{id}/enable
```

日志归档：

```text
GET    /api/v1/databases/log-archive-streams
POST   /api/v1/databases/log-archive-streams
PUT    /api/v1/databases/log-archive-streams/{id}
DELETE /api/v1/databases/log-archive-streams/{id}
POST   /api/v1/databases/log-archive-streams/{id}/test
GET    /api/v1/databases/log-archives
POST   /api/v1/databases/log-archives/register
```

恢复计划：

```text
POST /api/v1/databases/restore-plans
GET  /api/v1/databases/restore-plans
GET  /api/v1/databases/restore-plans/{id}
POST /api/v1/databases/restore-plans/{id}/validate
POST /api/v1/databases/restore-plans/{id}/run
POST /api/v1/databases/restore-plans/{id}/cancel
```

存储和密钥：

```text
GET    /api/v1/databases/storage-profiles
POST   /api/v1/databases/storage-profiles
PUT    /api/v1/databases/storage-profiles/{id}
DELETE /api/v1/databases/storage-profiles/{id}
POST   /api/v1/databases/storage-profiles/{id}/test

GET    /api/v1/databases/secret-profiles
POST   /api/v1/databases/secret-profiles
PUT    /api/v1/databases/secret-profiles/{id}
DELETE /api/v1/databases/secret-profiles/{id}
```

副本治理：

```text
GET  /api/v1/databases/instances/{id}/replicas
GET  /api/v1/databases/instances/{id}/replication-status
POST /api/v1/databases/instances/{id}/replica-incident-guide
```

## 权限和审计

新增菜单权限建议：

```text
database:backup:logical
database:backup:physical
database:backup:external-register
database:backup:run
database:backup:verify
database:backup:download
database:log-archive:view
database:log-archive:manage
database:restore:plan
database:restore:pitr-run
database:restore:proof
database:storage-profile:manage
database:secret-profile:manage
database:runner:manage
database:replica:view
database:replica:incident-guide
database:replica:pause-apply
```

实例位图权限后续可以增加：

```text
PHYSICAL_BACKUP
LOG_ARCHIVE
PITR_RESTORE
REPLICA_GOVERNANCE
```

审计动作建议：

```text
backup_task_create
backup_task_update
backup_run
backup_verify
backup_download
physical_backup_run
log_archive_stream_create
log_archive_stream_update
log_archive_register
restore_plan_create
restore_plan_validate
pitr_restore_run
restore_proof_generate
storage_profile_test
secret_profile_update
runner_job_dispatch
replica_incident_guide
replica_pause_apply
```

安全要求：

1. 对象存储密钥、SSH 私钥、数据库密码、加密密钥不允许以明文 JSON 保存到任务表。
2. 生产恢复执行权限必须独立于恢复演练权限。
3. 暂停副本 apply 必须独立权限、二次确认和审计。
4. 删除备份对象必须独立权限和审批预留。
5. 恢复计划和恢复执行的 SQL 校验脚本也要审计。

## 分期实施计划

### P0：现有逻辑备份安全收口

目标：不引入物理备份前，先让现有逻辑备份在大库场景下不误导用户，并消除明显执行风险。

范围：

1. UI 明确标注当前是逻辑全量备份。
2. UI 标注当前不支持 PITR。
3. 根据容量采样提示“大库不建议使用每日逻辑全量作为主备份方案”。
4. 备份任务增加 `next_run_at`、`last_success_at`、`restore_capability`。
5. 手动备份改为异步返回 record ID，避免 HTTP 请求等待完整备份。
6. 调度器增加全局并发限制。
7. 调度器增加单实例互斥。
8. 固定 6 小时 stale 改为 `max_duration_minutes` 和 heartbeat。
9. 备份记录状态补充 queued、running、cleaning、success、failed、expired。
10. 禁止 `storage_config` 保存疑似密钥字段，提示使用后续 storage profile。
11. 巡检报告把“逻辑备份成功”与“PITR 可恢复”分开。

验收：

1. 页面能明确看到当前策略是逻辑备份。
2. 大库创建每日逻辑全量任务时有风险提示。
3. 手动触发立即返回运行记录，不阻塞到命令结束。
4. 同一实例同一时间不会跑多个备份。
5. 长时间备份不会因为固定 6 小时被误判失败。
6. 备份任务能展示下次运行、最近成功、当前恢复能力。

### P1：备份链、日志链和恢复计划模型

目标：先建模型和只读校验能力，为物理备份和 PITR 做基础。

范围：

1. 新增 `database_log_archive_streams`。
2. 新增 `database_log_archives`。
3. 新增 `database_restore_plans`。
4. 新增 `database_storage_profiles`。
5. 新增 `database_secret_profiles`。
6. 新增 `database_runner_jobs`。
7. 扩展 backup task 和 record 的链路字段。
8. 支持 external backup record 注册。
9. 支持 external log archive 注册。
10. 支持恢复计划生成和预校验，但不实际恢复。
11. 前端展示 PITR 是否可用、可恢复窗口和缺口原因。

验收：

1. 能登记一条外部物理备份记录。
2. 能登记一段 binlog/WAL 归档元数据。
3. 能为目标时间生成恢复计划。
4. 缺 base backup、缺增量、缺日志、checksum 异常时能明确显示。
5. 页面能展示 `pitr_capable` 和 `pitr_verified` 的差异。

### P2：MySQL/MariaDB 物理备份和 binlog 归档

目标：接入 MySQL/MariaDB 大库主链路。

当前拆分：

1. P2 第一版：物理备份、元数据采集、恢复计划和证明预生成。
2. P2.1-P2.2：Runner Host、Runner Job 和 SSH 白名单执行底座。
3. P2.3-P2.5：一次性 binlog 归档和受控批量追平。
4. P2.6：长期 binlog 归档 Runner。
5. P2.7：隔离恢复 Runner。

范围：

1. Runner 支持 XtraBackup。
2. Runner 支持 mariadb-backup。
3. 任务创建时做版本兼容检查。
4. 支持 physical full。
5. 支持 physical incremental。
6. 支持 binlog 连续归档元数据采集。
7. 支持恢复计划校验 GTID/binlog 连续性。
8. 支持恢复到隔离库。
9. 支持恢复证明。

验收：

1. MySQL 8.0 不允许选择 XtraBackup 8.4。
2. MariaDB 默认推荐 mariadb-backup。
3. 缺少任一增量时恢复计划失败。
4. 缺 binlog 或 GTID 断链时恢复计划失败。
5. 能恢复到指定时间点的隔离 MySQL/MariaDB 实例。
6. 恢复证明包含 base backup、incremental chain、binlog 区间、checksum 和校验 SQL。

P2 完成度判定：

1. 只完成 P2 第一版到 P2.5 时，系统具备“可生成计划、可登记/归档、可证明理论可恢复”的能力。
2. 完成 P2.6 后，系统具备“持续维护 binlog 可恢复窗口”的能力。
3. 完成 P2.7 后，系统才具备“MySQL/MariaDB 物理备份链自动恢复到隔离库并验证”的完整闭环。

### P3：PostgreSQL 物理备份和 WAL 归档

目标：接入 PostgreSQL 大库主链路。第一版深接 Barman。

范围：

1. Barman server 登记。
2. Barman catalog 同步。
3. Barman backup 触发。
4. Barman WAL 状态采集。
5. Barman restore 到隔离目录或隔离实例。
6. timeline、LSN、system_identifier 校验。
7. pg_basebackup 作为轻量备用。
8. WAL-G 先支持 external metadata registration，后续再深接。
9. pgBackRest 仅 legacy external，不作为默认推荐。

验收：

1. PostgreSQL 物理备份任务页面明确显示 cluster 级。
2. Barman 备份记录能同步到 OpsHub。
3. WAL 缺口能被恢复计划识别。
4. timeline 不匹配时恢复计划失败。
5. 能恢复到指定 target time 或 target LSN 的隔离 PostgreSQL 实例。
6. 如使用原生增量，恢复计划必须包含 pg_combinebackup 步骤。

### P4：副本治理和误操作事故剧本

目标：把实时从库、延迟从库纳入灾备闭环。

范围：

1. 实时从库状态采集。
2. 延迟从库状态采集。
3. replication lag 展示。
4. remaining delay 展示。
5. relay log / WAL 积压展示。
6. 误删事故操作指引。
7. 暂停 apply 命令建议。
8. 暂停 apply 执行动作预留权限和审批。

验收：

1. 能识别一个实例是否存在延迟副本。
2. 能展示延迟配置和实际 apply/replay 时间。
3. 用户可以生成误删事故指引。
4. 指引生成写入审计。
5. 第一版不会在未授权情况下自动暂停副本 apply。

## 迁移策略

### 现有任务

迁移规则：

1. `backup_type=logical` 映射为 `backup_method=logical`、`backup_level=full`、`backup_engine=mysqldump/pg_dump/redis`。
2. `backup_type=logical_custom` 映射为 `backup_method=logical`、`backup_format=custom`、`backup_engine=pg_dump`。
3. `storage_type=local` 创建默认 local storage profile。
4. `storage_config` 保留读取，但新建任务不再鼓励填写密钥。
5. `restore_capability=logical_restore_only`。

### 现有记录

迁移规则：

1. 老记录不补物理链字段。
2. 老记录不参与 PITR 计划。
3. 老记录继续支持下载、校验、逻辑恢复演练。
4. 老记录页面明确显示“逻辑备份记录，不支持 PITR”。

### 配置兼容

1. 不删除现有备份目录配置。
2. Docker compose 里的 `./data/database-backups` 继续保留。
3. 新 storage profile 默认指向现有本地目录。
4. 后续对象存储接入不影响老本地备份。

## 非目标

以下内容不在第一轮大库备份改造内：

1. Redis 备份重构。
2. 自动生产恢复切换。
3. 自动暂停延迟副本 apply。
4. 自动删除远端不可变备份。
5. 自研备份文件格式。
6. 第一版同时深接 Barman、WAL-G、pg_basebackup 三套 PostgreSQL 引擎。
7. 第一版承诺 MySQL/PostgreSQL 表级物理恢复。
8. 第一版承诺所有数据库版本和所有存储引擎组合。

## 风险和处理

| 风险 | 影响 | 处理 |
| --- | --- | --- |
| 版本兼容错误 | 备份不可用或恢复失败 | 任务创建前做工具版本和数据库版本检测 |
| 日志链缺口 | PITR 失败 | 恢复计划必须校验 binlog/WAL 连续性 |
| 存储对象丢失 | 恢复失败 | checksum、对象存在性、不可变保留、恢复演练 |
| 备份压垮主库 | 业务性能下降 | 优先从库备份、限速、并发控制、窗口控制 |
| 密钥泄露 | 备份仓库被攻击 | secret profile、凭据引用、禁止明文 storage_config |
| 误恢复到生产 | 数据破坏 | 第一版只允许隔离库，生产切换后置审批 |
| 工具输出差异 | 状态解析失败 | Runner 回传结构化 manifest，保留原始日志摘要 |
| pgBackRest 维护停止 | 长期风险 | 新方案不默认推荐，仅 legacy external |

## 后续执行清单

P0 开始前先完成：

1. 梳理当前备份任务和记录 API 返回结构。
2. 设计迁移 SQL 或 GORM AutoMigrate 字段扩展。
3. 设计异步手动备份返回值。
4. 设计调度器全局并发参数。
5. 设计前端备份风险提示和恢复能力字段。
6. 补 P0 单元测试和前端构建测试。

P1 开始前先完成：

1. 确认 storage profile 和 secret profile 是否复用现有凭据模块。
2. 确认 Runner 首版形态：本地 Runner、SSH Runner 或 Agent。
3. 确认 restore plan 的最小字段和状态机。
4. 确认 external backup/log registration 的接口边界。

P2 开始前先完成：

1. 准备 MySQL 8.0、MySQL 8.4、MariaDB 测试环境。
2. 准备对应 XtraBackup/mariadb-backup 工具版本。
3. 准备 binlog 归档测试链路。
4. 定义恢复到隔离库的 Docker/主机运行方式。

P3 开始前先完成：

1. 决定 PostgreSQL 第一深接引擎，默认 Barman。
2. 准备 Barman 测试服务器。
3. 准备 WAL archive 和 WAL streaming 测试。
4. 准备 timeline 切换和 PITR 测试用例。

## 最终目标验收

一个核心生产大库在 OpsHub 中应能看到：

1. 当前备份策略是物理全量 + 增量 + 日志归档。
2. PITR 状态是 `pitr_verified`。
3. 可恢复窗口明确，例如最近 30 天。
4. 最近成功全量、最近成功增量、最近日志归档时间明确。
5. 最近恢复演练成功并有证明。
6. 恢复计划能展示所需备份链和日志链。
7. 任一链路缺口能在执行前被发现。
8. 恢复只能先到隔离库。
9. 所有操作有权限、二次确认和审计。
