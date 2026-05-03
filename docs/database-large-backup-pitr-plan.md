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

状态更新：截至 2026-04-29，长期 binlog 归档已经推进到 P2.6.12，对象存储安全姿态检测已落地；隔离恢复 Runner 已完成 P2.7 第一版，支持 MySQL/MariaDB 物理备份链恢复到 SSH Runner 上的隔离 Docker 容器并生成 proof。

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

#### 2026-04-29 P2.6.4 已落地范围

本阶段把 P2.6.3 的“能跑起来”继续补成“能看见、能审计、能交付 Agent 配置”的闭环。重点仍然不是引入真正 `--stop-never` 长连接，也不是让 backend 接管对象存储写入，而是先把长期守护进程的运维观察面补齐。

已实现：

1. 新增 `database_log_archive_events` 事件表：
   - 记录 `stream_id / instance_id / source_instance_id / runner_host_id / runner_id`。
   - 记录 `event_type / level / message / file_name / cursor_file / cursor_pos / active_file / archive_lag_seconds`。
   - 记录 `payload_json`，只保存非敏感摘要，不保存数据库密码、连接串、临时 defaults 文件路径或对象存储密钥。
   - 记录 `occurred_at`，用于还原 Agent 侧事件发生顺序。
2. 后端新增事件查询接口：
   - `GET /api/v1/databases/log-archive-events`
   - 支持按归档流、实例、Runner、级别、事件类型过滤。
   - 继续走数据库备份查看权限和实例级权限收敛。
3. 后端新增 Runner Agent 事件上报接口：
   - `POST /api/v1/public/databases/runner-agents/:runnerId/log-archive-events`
   - 复用 `X-OpsHub-Runner-Auth` / Bearer 鉴权。
   - 如果事件绑定 `streamId`，会校验归档流是否绑定当前 Runner。
4. 后端自动记录关键状态事件：
   - 用户启动、暂停、恢复、停止归档流时记录 `state_changed`。
   - Agent 获取 stream lease 时记录 `lease_acquired`。
   - Agent checkpoint 上报降级、失败、释放租约时记录 `checkpoint`。
   - Agent 登记归档文件成功后记录 `archive_success`。
   - Runner 心跳上报失败状态时记录 `runner_heartbeat`。
5. `opshub-agent` 归档循环补充事件上报：
   - 任意 stream 处理失败会补充上报 `archive_failed`，并带上归档模式、源库当前 binlog、position、当前 file/cursor 等非敏感摘要。
   - streaming 安全 spool 成功后上报 `spool_updated`。
   - 正常归档成功仍以登记 `database_log_archives` 为主，后端会自动补 `archive_success` 事件，避免 Agent 和后端重复写成功事件。
6. 前端 PITR 页新增 `Agent事件` 子页：
   - 可按归档流、Runner、级别、事件类型过滤。
   - 显示事件时间、级别、类型、实例/Runner、文件、游标、延迟、消息和摘要。
   - 归档流行和 Runner 主机行可以直接跳转到对应事件过滤视图。
7. 前端 Runner 主机新增 `配置` 操作：
   - 自动生成 `runner-host-<id>` 形式的 Runner ID。
   - 生成随机 `runnerAuth` 明文，供 Agent 本地配置使用。
   - 生成 `runnerAuthSha256`，供 Runner 主机 `configJson` 保存。
   - 根据已绑定该 Runner 的归档流生成 Agent 本地 `databaseArchiver` 配置片段。
   - 生成内容中的数据库用户名/密码仍是占位符，需要用户在 Agent 本机填入，不从 backend 下发真实数据库密码。

本阶段仍未实现，保留给 P2.6.5+：

1. 真正长连接 `mysqlbinlog --stop-never` 子进程管理。
2. streaming active spool 的增量续传和断点 resume。
3. 对象存储 S3/MinIO staging key、checksum 后提交和远端不可变保留。
4. Agent 侧多 stream 并发度、带宽限制和失败退避策略。
5. 更细的 `purge_gap` 自动诊断：当前 cursor 已被源库 purge 时，除降级外还应明确写入断链事件并阻止后续恢复计划误判。
6. 事件保留策略：高频事件要支持保留天数、归档或压缩，避免事件表无界增长。

### 2026-04-29 P2.6.5：对象存储发布与 purge gap 诊断

目标：在不改变 backend 存储密钥边界、不引入真正 `--stop-never` 长连接的前提下，把 Agent 已经 finalized 的 binlog 产物发布到 S3/MinIO，并把源库 purge 导致的日志断链从普通失败升级为明确诊断事件。

#### P2.6.5 落地范围

1. Agent 本地配置新增 `databaseArchiver.storage`。
   - `type=local` 时保持原有 `runner://` 本地归档语义。
   - `type=s3/minio` 时，Agent 在本机使用本地配置中的对象存储凭据上传归档产物。
   - backend 不下发对象存储明文密钥，前端配置生成器只生成占位模板。
2. finalized binlog 发布到对象存储。
   - 先在 Runner 本地完成 `mysqlbinlog --raw` 下载、checksum、manifest 生成。
   - 再上传到 staging key。
   - 对 staging 对象执行 `HeadObject`，校验对象大小和 `opshub-sha256` metadata。
   - 校验通过后上传到 final key。
   - final 对象再次校验通过后，上传 `.sha256` 与 `.manifest.json` sidecar。
   - 所有对象发布成功后才调用 backend 登记 `database_log_archives`，避免数据库中出现不可读取的归档记录。
3. 对象 key 规范。
   - final key：
     `pathPrefix/mysql-binlog/instance-{instance_id}/stream-{stream_id}/finalized/{binlog_file}`
   - staging key：
     `stagingPrefix/mysql-binlog/instance-{instance_id}/stream-{stream_id}/finalized/{binlog_file}.{unix_nano}.tmp`
   - `storage_uri`：
     `s3://bucket/key` 或 `minio://bucket/key`
4. `purge_gap` 自动诊断。
   - 当 `lastArchiveName/cursorFile` 已经不在 `SHOW BINARY LOGS` 返回列表中时，Agent 返回 typed purge gap。
   - checkpoint 将归档流置为 `degraded` 并写入 `last_error`。
   - 事件表写入 `purge_gap`，级别为 `warning`，payload 包含：
     - `lastArchived`
     - `firstAvailable`
     - `lastAvailable`
     - `sourceFile`
     - `sourcePos`
   - 该状态表示 PITR 日志链已经存在断点，不能再把该流误判为连续可恢复。
5. 前端 Agent 配置生成器补充对象存储模板。
   - 默认仍为 `type: local`，避免用户误以为 backend 已经托管对象存储密钥。
   - 若使用 MinIO/S3，需要用户在 Agent 本机把 `type`、`endpoint`、`bucket`、`accessKey`、`secretKey` 改成真实值。

#### Agent 本地配置示例

```json
{
  "databaseArchiver": {
    "enabled": true,
    "baseUrl": "http://opshub.example.com",
    "runnerId": "runner-host-1",
    "runnerAuth": "<local_secret>",
    "intervalSeconds": 30,
    "leaseTtlSeconds": 90,
    "maxFilesPerLoop": 5,
    "includeCurrent": false,
    "workDir": "/var/lib/opshub-agent",
    "storageRoot": "/var/lib/opshub-agent/database-archives",
    "storage": {
      "type": "minio",
      "endpoint": "http://192.168.1.30:9000",
      "bucket": "opshub-backup",
      "region": "us-east-1",
      "pathPrefix": "opshub/database-archives",
      "stagingPrefix": "opshub/database-archives/.staging",
      "accessKey": "<minio_access_key>",
      "secretKey": "<local_secret>",
      "useSsl": false,
      "usePathStyle": true,
      "insecureSkipVerify": false
    },
    "mysqlBinlogPath": "",
    "credentials": []
  }
}
```

#### P2.6.5 仍不做的事

1. 不实现真正长期 `mysqlbinlog --stop-never` 子进程管理。
2. 不把 active spool 文件登记为 finalized 日志。
3. 不让 backend 容器直接持有对象存储密钥或执行上传。
4. 不自动创建 bucket、调整对象锁、版本化或不可变保留策略；这些属于存储侧基线治理，后续只做检测和提示。
5. 不做自动隔离恢复 Runner；P2.7 仍按独立执行面设计。

#### P2.6.6+ 后续深化

1. streaming active spool 增量续传和断点 resume。
2. 真正 `mysqlbinlog --stop-never` 进程托管、健康检查和自动重启。
3. Agent 多 stream 并发度、带宽限制、失败退避。
4. 事件表保留策略和高频事件压缩。
5. 对象存储版本化、不可变保留、KMS、bucket policy 的只读检测与风险提示。

### 2026-04-29 P2.6.6：Agent 并发、失败退避和事件保留

目标：把 P2.6.5 已能发布对象存储的 Agent 归档器继续推进到长期运行可用状态。此阶段不引入真正 `mysqlbinlog --stop-never` 长连接，也不做二进制 partial append，先解决一个 Runner 同时负责多个归档流时的调度稳定性、失败隔离和事件表增长问题。

#### P2.6.6 落地范围

1. Agent 本地配置新增并发与退避参数。
   - `maxConcurrentStreams`：同一 Agent 每轮最多并发处理多少个归档流，默认 `2`，范围 `1-16`。
   - `failureBackoffSeconds`：单个归档流失败后的基础退避秒数，默认 `30`，最小 `5`。
   - `maxFailureBackoffSeconds`：单个归档流指数退避上限，默认 `300`，最大 `3600`。
2. 多 stream 并发调度。
   - 每轮从 backend 获取分配给当前 Runner 的归档流。
   - 对不在退避窗口内的归档流按 `maxConcurrentStreams` 并发执行。
   - 每个 stream 仍独立获取租约、checkpoint、归档登记和事件上报。
   - 任一 stream 失败不会阻塞其他 stream。
3. 单 stream 失败指数退避。
   - 第一次失败退避 `failureBackoffSeconds`。
   - 后续连续失败按 `2x` 增长，直到 `maxFailureBackoffSeconds`。
   - stream 成功后清空本地失败计数和退避窗口。
   - 退避状态只保存在 Agent 进程内；Agent 重启后重新从 backend 状态和租约恢复。
4. active spool 去重优化。
   - `streaming` 模式仍只把当前活跃 binlog 保存到 `spool/<file>.partial`，不登记为 finalized。
   - 如果本地 partial 文件大小已经大于等于源库当前活跃 binlog 大小，本轮跳过重复拉取，避免每 30 秒重写同一份活跃文件。
   - `spool_updated` 事件 payload 增加：
     - `sourceSize`
     - `spoolSize`
     - `reused`
   - 如果源库活跃 binlog 继续增长，当前阶段仍采用重新拉取并原子替换 partial 的安全策略，不做 append。
5. 事件保留策略。
   - 写入 `database_log_archive_events` 后，按归档流 `retention_days` 清理该流更早的事件。
   - 没有归档流上下文时使用默认 `30` 天。
   - 保留天数最大按 `3650` 天保护，避免异常配置导致事件表无限保留。

#### P2.6.6 不做的事

1. 不实现长期 `mysqlbinlog --stop-never` 子进程。
2. 不对 `.partial` 文件做二进制 append。binlog 事件边界、header 和 checksum 需要专门验证，不能只按文件大小追加。
3. 不把 active spool 纳入 PITR 恢复证明。
4. 不做带宽限制和 IO 限速；本阶段只做并发度控制。
5. 不自动调整对象存储 bucket 的版本化、对象锁、KMS 或 bucket policy。

#### P2.6.7+ 后续深化

1. 真正 `mysqlbinlog --stop-never` 进程托管、健康检查、自动重启和 stdout/stderr 日志滚动。
2. active spool 增量 append 前置验证：
   - 验证 `mysqlbinlog --raw --start-position` 输出是否适合 append。
   - 校验 binlog header、事件边界和 checksum。
   - 只在严格验证通过后开放 resume。
3. Agent 侧带宽限制、IO 限速和对象存储上传重试队列。
4. 事件压缩/归档，把高频 `checkpoint/spool_updated` 合并为小时级摘要。
5. 对象存储版本化、不可变保留、KMS、bucket policy 的只读检测和 UI 风险提示。

### 2026-04-29 P2.6.7：`mysqlbinlog --stop-never` 进程托管

目标：把 P2.6.6 的 `streaming` 归档流从“每轮安全重拉 active binlog 的伪 streaming”推进到“Agent 本地托管长期 `mysqlbinlog --stop-never` 子进程”。本阶段仍不做 active spool append/resume，也不把 active spool 纳入 PITR 恢复证明；重点是进程生命周期、租约安全、健康检查、自动重启和本地日志滚动。

#### P2.6.7 为什么必须单独拆

`mysqlbinlog --stop-never` 是长期进程，不适合放在 backend 容器，也不适合绑定一次 HTTP 请求或 SSH session。它会持续连接源库、持续写 spool 文件，并且可能跨 binlog 轮转运行数天到数月。如果进程托管不可靠，会出现几类高风险问题：

1. 用户在 OpsHub 暂停/停止归档流后，Runner 主机上旧进程仍在运行。
2. backend 租约过期后，旧 Agent 和新 Agent 同时拉同一个 stream，造成重复 spool、误报健康状态或对象存储竞争。
3. Agent 崩溃重启后，不知道旧进程是否还存在，也不知道 active spool 是否可信。
4. `mysqlbinlog` 异常退出后无人重启，UI 仍显示归档流 running。
5. stdout/stderr 无界写入，Runner 本地磁盘被日志打满。

因此 P2.6.7 的原则是：**先把长期进程管住，再讨论 spool 文件如何 finalize，最后才讨论 append/resume**。

#### P2.6.7 前置条件

1. Runner Agent 必须以长期服务方式运行。
   - 推荐 systemd、Docker restart policy 或后续专用 Agent supervisor。
   - 不推荐用临时 shell、一次性 SSH session 或 backend 容器内进程运行。
2. Runner Host 必须有稳定工作目录。
   - `workDir` 用于临时 option file、运行态文件。
   - `storageRoot` 用于 `mysql-binlog/instance-{id}/stream-{id}/spool`、`finalized`、`logs`。
   - 需要预留足够磁盘，后续 P2.6.10 再加最小剩余空间和 spool 容量水位保护。
3. 源库账号必须具备远程读取 binlog 的权限。
   - MySQL/MariaDB 版本差异会影响权限名称，但至少要能执行 `SHOW BINARY LOGS` / `SHOW BINARY LOG STATUS` 或兼容语句。
   - `mysqlbinlog --read-from-remote-server --raw --stop-never` 必须能连接源库。
4. 源库必须开启 binlog。
   - `log_bin=ON`。
   - binlog 保留时间必须覆盖 Agent 故障恢复窗口，否则会出现 purge gap。
5. 系统时间要同步。
   - Agent、backend、数据库时间漂移会影响 `last_event_time`、归档延迟和 RPO 判断。
6. 先接受本阶段不做 append/resume。
   - active spool 是运行态证据，不是 finalized archive。
   - PITR 恢复计划只允许使用 `database_log_archives` 中已登记的 finalized 文件。

#### P2.6.7 落地范围

1. Agent 本地配置扩展：
   - `stopNeverEnabled`：是否允许 `streaming` 归档流启用长期 `mysqlbinlog --stop-never` 子进程。
   - `streamingLogMaxBytes`：单个 stdout/stderr 日志文件最大字节数。
   - `streamingLogMaxFiles`：每个 stream 每类日志最多保留多少个滚动文件。
2. Agent 进程表：
   - Agent 内存维护 `stream_id -> process_state`。
   - 每个状态记录：
     - `stream_id`
     - `pid`
     - `active_file`
     - `started_at`
     - `restart_count`
     - `output_dir`
     - `stdout_log`
     - `stderr_log`
     - `last_exit_error`
3. 每轮 reconcile：
   - Agent 先 heartbeat。
   - 从 backend 获取当前分配给本 Runner 的 runnable streams。
   - 对不在返回列表里的本地 streaming 进程执行停止。
   - 如果 heartbeat 或 list streams 失败，本阶段选择安全优先：停止本地 streaming 进程，避免租约失效后双 Agent 同时拉取。
4. stream 处理：
   - 只有 `archive_mode=streaming` 且 `stopNeverEnabled=true` 时启动长期进程。
   - `archive_mode=polling` 或配置关闭时，确保该 stream 不存在残留长期进程。
   - 已有进程存活时不重复启动。
   - 子进程退出后下一轮自动拉起，重启受 Agent 已有失败退避约束保护。
5. 启动命令：

   ```bash
   mysqlbinlog \
     --defaults-extra-file=<agent-temporary-option-file> \
     --read-from-remote-server \
     --raw \
     --stop-never \
     --result-file=<streaming-spool-dir>/ \
     <current-binlog-file>
   ```

   约束：
   - `--defaults-extra-file` 放在第一个参数位置。
   - option file 权限 `0600`。
   - 不使用 `MYSQL_PWD`。
   - 不在命令行参数中出现明文密码。
   - option file 在子进程退出后删除。
6. spool 目录：
   - 长期进程写入独立目录：

     ```text
     <storageRoot>/mysql-binlog/instance-{id}/stream-{id}/spool/streaming/
     ```

   - 该目录中的文件是 active streaming 输出，不等于 finalized archive。
   - 本阶段不登记 `database_log_archives`。
   - 本阶段不参与 PITR 计划。
7. stdout/stderr 日志滚动：
   - 日志目录：

     ```text
     <storageRoot>/mysql-binlog/instance-{id}/stream-{id}/logs/
     ```

   - 文件：
     - `mysqlbinlog.stdout.log`
     - `mysqlbinlog.stderr.log`
   - 超过 `streamingLogMaxBytes` 后滚动：
     - `.1`
     - `.2`
     - ...
   - 最多保留 `streamingLogMaxFiles` 份历史。
8. 健康检查：
   - 进程是否仍在运行。
   - pid 是否存在。
   - 子进程退出错误。
   - output dir 是否存在。
   - stdout/stderr log path 是否可写。
   - 最近一轮 checkpoint 仍写入源库当前 binlog 文件和 position。
9. 事件：
   - 启动成功写 `agent_message`。
   - 子进程退出写 `agent_message`，异常退出为 warning。
   - 因 pause/stop/租约不可刷新而停止写 `agent_message`。
   - payload 只保存 pid、目录、日志路径、active file、restart count，不保存密码和 option file 内容。
10. 失败策略：
    - 启动失败或异常退出后，stream 本轮返回失败。
    - 外层 P2.6.6 的 failure backoff 负责限制重启频率。
    - 成功运行后清空对应 stream 的本地失败退避。

#### P2.6.7 不做的事

1. 不把 streaming spool 文件改名成 finalized archive。
2. 不从 streaming spool 生成 `database_log_archives` 记录。
3. 不做 `.partial` 文件二进制 append。
4. 不解析 binlog event 边界。
5. 不校验 event checksum。
6. 不实现上传重试队列。
7. 不做带宽或 IO 限速。
8. 不自动 kill Agent 进程外历史遗留的孤儿 `mysqlbinlog`；后续需要用 pidfile + command fingerprint 谨慎处理。

#### P2.6.7 验收标准

1. streaming 归档流启动后，Runner 本地存在且只存在一个对应 stream 的 `mysqlbinlog --stop-never` 子进程。
2. 同一 stream 下一轮不会重复启动第二个长期进程。
3. 用户暂停/停止归档流后，下一轮 Agent reconcile 会停止本地长期进程。
4. backend heartbeat/list streams 失败时，本地长期进程会被停止，避免租约失效后双写。
5. 子进程异常退出后会产生事件，并在下一轮按退避策略自动重启。
6. stdout/stderr 日志会滚动，不会无限增长。
7. 数据库密码不会出现在：
   - command summary
   - event payload
   - stdout/stderr 路径
   - backend JSON
8. PITR 计划仍只使用 finalized archive，不会误用 active streaming spool。

#### 2026-04-29 P2.6.7 已落地范围

本次实现按上面的安全边界落地，不改变 PITR 可用链路的定义：`database_log_archives` 仍只登记 finalized 文件，`spool/streaming` 仍只是长期进程输出目录。

已实现：

1. Agent 配置新增：
   - `stopNeverEnabled`
   - `streamingLogMaxBytes`
   - `streamingLogMaxFiles`
2. `streaming + stopNeverEnabled=true` 时，Agent 启动并托管 `mysqlbinlog --stop-never` 子进程。
3. Agent 内存维护 per-stream 进程表，避免同一 stream 重复启动多个长期进程。
4. 每轮从 backend 获取 runnable streams 后执行 reconcile：
   - 不再分配给当前 Runner 的 stream 会停止本地长期进程。
   - heartbeat 或 list streams 失败时，停止本地长期进程，优先避免租约失效后的双进程拉取。
5. `polling` 模式或 `stopNeverEnabled=false` 时，会清理同 stream 残留长期进程，并继续使用 P2.6.6 的安全 active spool 逻辑。
6. 长期进程 stdout/stderr 使用本地滚动日志：
   - `mysqlbinlog.stdout.log`
   - `mysqlbinlog.stderr.log`
7. 启动、异常退出、手动停止都会写入 `agent_message` 事件，payload 只保存 pid、目录、日志路径、active file、restart count 和错误摘要。
8. 每轮 running streaming 进程都会写入 `checkpoint` 事件，payload 包含：
   - pid
   - startedAt
   - restartCount
   - stdout/stderr log path
   - streaming spool path
   - streaming spool size
   - spoolUpdatedAt / lastGrowthAt
   - source file / source position
   - serverUUID / serverID（能读取到时）
9. 前端 Runner Agent 配置模板默认生成 `stopNeverEnabled: true`，但 Agent 运行时仍要求本地配置显式开启，不对旧配置自动启用。
10. 单元测试覆盖：
   - 配置默认值和边界裁剪。
   - 滚动日志。
   - stop-never 子进程启动、复用和停止。

#### P2.6.8+ 后续衔接

P2.6.7 完成后，后续顺序保持：

1. P2.6.8：streaming spool 轮转 finalize。
   - 当 `binlog.000123` 不再是 active file 且 streaming 输出完整时，校验后转入 finalized。
   - 校验失败时回退到当前已存在的完整拉取逻辑。
2. P2.6.9：active spool append/resume 前置验证。
   - 独立 `binlogvalidator` 包。
   - 校验 binlog magic header、event header、event length、end_log_pos、checksum。
   - 只在严格验证通过后 append。
3. P2.6.10：上传重试队列、带宽限制和 IO 限速。
   - 本地 durable queue。
   - staging key 后提交。
   - 对象存储临时不可用时不丢 finalized archive。
4. P2.6.11：事件 rollup。
   - 高频 `checkpoint/spool_updated/agent_message` 聚合到小时级摘要。
   - error/warning/state_changed/archive_success 保留原始事件。
5. P2.6.12：对象存储安全姿态检测。
   - S3/MinIO 版本化、对象锁、默认加密、KMS、bucket policy 只读检测。
   - UI 展示登记值和检测值差异。

### 2026-04-29 P2.6.8：streaming spool 轮转 finalize

目标：在 `mysqlbinlog --stop-never` 已经由 Agent 托管后，把已经轮转完成的 streaming spool 文件安全提交为 finalized binlog。P2.6.8 只处理“不再是当前活跃 binlog”的文件；当前活跃文件仍只作为运行态 spool，不进入 PITR 恢复链。

#### P2.6.8 落地范围

1. Agent 每轮读取源库 `SHOW BINARY LOGS` 后，继续用现有 `selectAgentBinlogsForArchive` 选择待归档文件。
2. 对 `streaming + stopNeverEnabled=true` 的归档流，优先查找：

   ```text
   <storageRoot>/mysql-binlog/instance-{id}/stream-{id}/spool/streaming/{binlog_file}
   ```

3. 只有满足以下条件才从 streaming spool 提交：
   - 文件名安全。
   - 文件存在且不是目录。
   - 文件大小等于源库 `SHOW BINARY LOGS` 中该文件的大小。
   - binlog magic header 正确。
   - 所有 event header 完整。
   - 所有 event size 合法。
   - 文件末尾落在完整 event 边界。
   - 如果能识别 CRC32 checksum，则逐事件校验 checksum。
4. 提交流程：
   - 复制 streaming spool 到 finalized 临时目录。
   - 通过 `commitAgentBinlogFile` 原子提交到 `finalized/`。
   - 生成 `.sha256` 和 `.manifest.json`。
   - 如启用 S3/MinIO，则沿用 P2.6.5 的 staging + verify + final key 发布。
   - 通过 Runner Agent API 登记 `database_log_archives`。
   - 登记成功后更新归档流 cursor。
5. 校验失败或 streaming spool 不存在时：
   - 写入 `agent_message` warning。
   - 如果本地存在无法校验的 streaming spool，则移动到 `spool/quarantine/`，避免后续继续误用。
   - 回退到已有完整远程拉取逻辑。
   - 回退成功后仍登记 finalized archive。

#### P2.6.8 不做的事

1. 不提交当前 active file。
2. 不提交 `.partial` 文件。
3. 不在 checksum 不确定时伪造 checksum 结论；只能证明 event 边界完整。
4. 不删除源库 binlog。
5. 不改变 PITR 计划选择规则。

#### 2026-04-29 P2.6.8 已落地范围

已实现：

1. Agent 新增 streaming spool finalize 优先路径。
2. finalized 前执行 binlog 文件结构校验。
3. 校验失败自动回退到完整远程拉取。
4. 校验失败时会把已存在的 invalid streaming spool 移入 quarantine，不登记 archive。
5. finalized 成功后仍使用现有对象存储发布和归档登记流程。
6. 单元测试覆盖有效 streaming spool 文件提交、sidecar 生成、spool 清理和 invalid spool quarantine。

### 2026-04-29 P2.6.9：active spool append/resume 前置验证

目标：为 active spool resume 提供严格前置验证。P2.6.9 不默认启用 resume；只有 Agent 本地显式配置 `spoolResumeEnabled=true`，并且现有 partial 与 resume candidate 都通过校验时，才允许 append。任何不确定情况都回退到完整重拉。

#### P2.6.9 校验模型

新增 Agent 侧 binlog validator，校验内容：

1. 文件必须以 binlog magic header 开始：

   ```text
   fe 62 69 6e
   ```

2. event header 必须完整，长度按 MySQL binlog event header 19 字节解析。
3. `event_size` 必须大于等于 19。
4. `event_size` 不能越过文件末尾。
5. 文件末尾必须刚好落在完整 event 边界。
6. 如果第一个 event 可识别 CRC32 checksum，则后续 event 全部按 CRC32 校验。
7. append candidate 必须与现有 partial 连续：
   - 现有 partial 的 `LastCompletePos` 作为 resume 起点。
   - candidate 第一个 event 的 `end_log_pos - event_size` 必须等于 `LastCompletePos`。
   - candidate 如果带 binlog magic header，只追加 magic 之后的 event payload。
   - 合并后的完整文件必须再次通过整文件校验。

#### P2.6.9 落地范围

1. Agent 配置新增：
   - `spoolResumeEnabled`：是否允许 active spool resume，默认 `false`。
2. 非 stop-never 的 active spool 流程中，如果：
   - `spool/<file>.partial` 已存在；
   - 源库 active binlog 比 partial 更大；
   - `spoolResumeEnabled=true`；
   - partial 校验通过；
   - `mysqlbinlog --raw --start-position=<LastCompletePos>` 产物校验通过；
   - 合并后整文件校验通过；

   则使用 append/resume 结果原子替换 partial。
3. 任一校验失败：
   - 不修改现有 partial。
   - 回退到 P2.6.6 的完整重拉并原子替换。
4. `spool_updated` 事件 payload 增加：
   - `resumed`
   - `resumeFrom`
   - `appendBytes`
   - `validationMode`
   - `lastCompletePos`
   - `serverUUID`
   - `serverID`
5. 前端 Runner Agent 配置模板加入 `spoolResumeEnabled: false`，要求用户明确评估后再打开。

#### P2.6.9 不做的事

1. 不让 `spoolResumeEnabled` 默认开启。
2. 不对 `mysqlbinlog --stop-never` 子进程启动参数做 resume；长期进程 resume 需要 pidfile、fingerprint 和输出目录锁，后续单独处理。
3. 不把 active partial 登记为 finalized archive。
4. 不将 partial 纳入 PITR 计划。
5. 不解析所有 MySQL/MariaDB event body 语义，只校验文件结构、event 边界、position 连续性和可识别 checksum。

#### 2026-04-29 P2.6.9 已落地范围

已实现：

1. `binlog_validator`：
   - magic header 校验。
   - event header / event size / 完整边界校验。
   - 可识别 CRC32 checksum 时逐事件校验。
   - append candidate 连续性校验。
2. active spool resume：
   - 显式开关。
   - partial manifest：
     - `fileName`
     - `sourceSize`
     - `lastCompletePos`
     - `checksumMode`
     - `serverUUID`
     - `serverID`
     - `updatedAt`
   - resume 前会校验 manifest 文件名、server_uuid、server_id，发现漂移即回退完整重拉。
   - `--start-position=<LastCompletePos>` 拉取增量候选。
   - candidate 可带 magic header，也可直接是 event stream。
   - 合并后再次整文件校验。
   - 原子替换 partial。
   - 失败回退完整重拉。
3. 单元测试覆盖：
   - CRC binlog 校验。
   - 截断文件拒绝。
   - resume candidate 带 magic header 的 append。
   - position 不连续拒绝。
   - active spool resume 合并结果。
   - partial manifest identity 漂移拒绝。

### 2026-04-29 P2.6.10：对象存储上传重试队列与基础限速

目标：对象存储临时不可用时，不让已经 finalized 的本地 binlog 归档丢失，也不让归档链因为一次 S3/MinIO 上传失败而中断。P2.6.10 的边界是“本地 finalized 是事实来源，远端对象存储发布可以异步补偿”；真正跨主机复制、严格 IO 调度和集中任务队列仍留到后续 Runner 平台化阶段。

#### P2.6.10 配置

Agent 配置新增：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `uploadRetryEnabled` | `true` | 对象存储发布失败时写入本地 durable queue |
| `uploadRetryBaseSeconds` | `60` | 首次重试退避 |
| `uploadRetryMaxSeconds` | `3600` | 最大重试退避 |
| `uploadRetryMaxAttempts` | `0` | 最大重试次数，`0` 表示不限制 |
| `uploadBandwidthBytesPerSecond` | `0` | Go 侧上传 reader 基础限速，`0` 表示不限速 |

#### P2.6.10 落地范围

1. finalized 文件先落地本地 `finalized/`，并生成 sidecar。
2. 对象存储启用时，发布流程为：
   - 上传 staging key。
   - 校验 staging 对象大小和 `opshub-sha256` metadata。
   - final key 不存在或不匹配时，从 staging copy 到 final key。
   - 校验 final 对象。
   - 上传 `.sha256` 和 `.manifest.json` sidecar。
   - 删除 staging key，避免 staging 长期堆积。
3. 对象存储上传失败时：
   - 不删除本地 finalized 文件。
   - 不登记损坏对象。
   - 写入本地 queue：

     ```text
     <storageRoot>/mysql-binlog/upload-queue/*.json
     ```

   - queue item 保存 stream、instance、runner、文件名、本地路径、目标 URI、大小、checksum、事件时间、重试次数、下次重试时间和最后错误。
4. Agent 每轮 heartbeat 后处理 upload queue：
   - 到期才重试。
   - 上传前重新校验本地文件大小和 checksum。
   - 成功后删除 queue item。
   - 通过 `agent_message` 写入“对象存储补传成功”事件。
   - 失败后按指数退避更新 `nextAttemptAt`。
5. 对象存储失败时，backend 仍可登记本地 `runner://` storage URI；补传成功后通过事件证明远端对象已补齐。后续如需要强一致展示，可再增加 archive storage URI 回填接口。

#### P2.6.10 不做的事

1. 不承诺 `mysqlbinlog` 下载流严格限速。
2. 不直接使用系统级 cgroup/ionice 配额；第一版只提供 Go 侧上传 reader 限速。
3. 不自动删除 final 对象。
4. 不在 backend 容器中执行对象存储补偿；补偿由 Agent 侧 durable queue 完成。

#### P2.6.10 验收标准

1. S3/MinIO 临时不可用时，本地 finalized 文件和 sidecar 保留。
2. 上传失败会生成 queue JSON。
3. 存储恢复后 Agent 能自动补传。
4. 同一文件重复上传幂等：final 对象已存在且 size/checksum 匹配时直接通过。
5. staging 对象成功发布后会被删除。

### 2026-04-29 P2.6.11：高频事件小时级 rollup

目标：`checkpoint`、`spool_updated`、部分 `agent_message` 会在长期 streaming 归档中高频产生。P2.6.11 把这些事件同步汇总为小时级摘要，原始明细保留短窗口，避免事件表无限增长，同时保留恢复审计所需的状态轨迹。

#### P2.6.11 数据模型

新增表：

```text
database_log_archive_event_rollups
```

核心字段：

| 字段 | 说明 |
| --- | --- |
| `stream_id` | 归档流 |
| `event_type` | `checkpoint / spool_updated / agent_message` |
| `bucket_start` / `bucket_end` | 小时窗口 |
| `event_count` | 事件总数 |
| `warning_count` / `error_count` | 警告和错误数 |
| `min_lag_seconds` / `max_lag_seconds` | 窗口内延迟范围 |
| `last_cursor_file` / `last_cursor_pos` | 窗口内最后游标 |
| `last_active_file` | 窗口内最后 active file |
| `last_message` | 最后一条消息 |
| `last_payload_json` | 最后一条事件 payload 摘要 |
| `last_occurred_at` | 最后一条事件时间 |

唯一键：

```text
(stream_id, event_type, bucket_start)
```

#### P2.6.11 落地范围

1. `recordLogArchiveEvent` 写入原始事件后，如果事件类型属于高频类型，同步 upsert 小时级 rollup。
2. rollup 更新策略：
   - `event_count` 累加。
   - warning/error 计数累加。
   - level 保存窗口内最高严重等级。
   - lag 保存窗口内 min/max。
   - cursor、active file、message、payload 保存最后发生事件的值。
3. 高频原始事件压缩策略：
   - 归档流整体事件仍按 stream retention 删除。
   - 当 stream retention 大于 7 天时，`info` 级高频原始事件只保留最近 7 天。
   - `warning/error` 级高频原始事件仍按 stream retention 保留，避免压缩掉异常证据。
   - 小时级 rollup 用于长期趋势和运行证明。
4. `archive_success`、`archive_failed`、`purge_gap`、`state_changed` 等关键事件仍按原 retention 保留原始明细。

#### P2.6.11 不做的事

1. 第一版不新增 rollup 列表 UI。
2. 不把 error/warning 全部压缩掉。
3. 不改变现有事件列表接口返回结构。
4. 不把 rollup 当作 PITR 恢复输入；PITR 仍只使用 finalized archive 和日志链元数据。

### 2026-04-29 P2.6.12：对象存储安全姿态检测

目标：让 OpsHub 能主动核验备份仓库的安全姿态，避免 storage profile 里“登记为安全”的值和真实对象存储配置不一致。P2.6.12 第一版只做 S3/MinIO 兼容对象存储的只读检测，输出登记值、检测值、差异和风险摘要；不在 backend 中保存对象存储明文密钥。

#### P2.6.12 为什么需要单独做

P2.6.10 已经把 binlog finalized 文件发布到 S3/MinIO，并通过 staging、checksum 和重试队列保证“对象能补传成功”。但这只能证明文件发布链路可用，不能证明备份仓库本身具备长期抗误删、抗覆盖、抗勒索删除的能力。

大库备份仓库至少要关注：

1. `versioning`：避免对象被覆盖或删除后没有历史版本。
2. `object lock / immutability`：防止备份对象在保留期内被误删或恶意删除。
3. 默认服务端加密：防止对象以明文落入远端存储。
4. KMS key：确认默认加密使用的是期望的 KMS Key，而不是平台默认 key 或空配置。
5. bucket policy / public access block：避免备份仓库被公开读取，或存在公开访问策略。

#### P2.6.12 数据模型

复用 `database_storage_profiles`，新增检测结果字段：

| 字段 | 说明 |
| --- | --- |
| `posture_status` | `unknown / passed / warning / failed / unsupported` |
| `posture_summary` | 最近一次检测摘要，用于列表快速展示 |
| `posture_json` | 最近一次检测明细 JSON，不保存 access key、secret key、session token |
| `last_posture_check_at` | 最近一次检测时间 |

`posture_json` 建议结构：

```json
{
  "checkedAt": "2026-04-29T12:00:00+08:00",
  "provider": "minio",
  "endpoint": "http://192.168.1.30:9000",
  "bucket": "opshub-backup",
  "expected": {
    "versioningEnabled": true,
    "immutabilityEnabled": true,
    "kmsKeyId": "kms-key-1",
    "retentionLockDays": 30
  },
  "actual": {
    "bucketReachable": true,
    "versioningStatus": "Enabled",
    "versioningEnabled": true,
    "objectLockEnabled": true,
    "objectLockMode": "GOVERNANCE",
    "objectLockRetentionDays": 30,
    "encryptionEnabled": true,
    "encryptionAlgorithm": "aws:kms",
    "encryptionKmsKeyId": "kms-key-1",
    "bucketPolicyPublic": "private",
    "publicAccessBlockConfigured": true,
    "blockPublicAcls": true,
    "ignorePublicAcls": true,
    "blockPublicPolicy": true,
    "restrictPublicBuckets": true
  },
  "checks": [
    {
      "key": "versioning",
      "label": "版本化",
      "status": "passed",
      "expected": "启用",
      "actual": "Enabled",
      "message": "Bucket 已启用版本化，可降低误删覆盖后无法找回对象的风险。"
    }
  ],
  "status": "passed",
  "summary": "对象存储安全姿态检测通过，登记值与检测值未发现明显差异。"
}
```

#### P2.6.12 检测输入与密钥边界

1. `database_storage_profiles` 仍只保存 endpoint、bucket、region、path_prefix 和期望安全值。
2. `secret_profile` 仍是密钥引用，不保存明文。
3. 主动检测接口要求用户临时输入：
   - `accessKey`
   - `secretKey`
   - `sessionToken` 可选
   - `useSsl`
   - `usePathStyle`
   - `insecureSkipVerify`
4. 临时凭据只用于本次 HTTP 请求，不写入数据库，不写入 `posture_json`，不写入审计摘要。
5. 以后如果接入 Vault / KMS / 凭据中心，可以让 backend 按 `secret_profile_id` 拉取临时凭据；P2.6.12 不直接扩大密钥存储面。

#### P2.6.12 后端接口

新增接口：

```text
POST /api/v1/databases/storage-profiles/:id/posture-check
```

权限：

```text
permDatabaseBackupRun
```

原因：

1. 检测是只读 Bucket API，但会更新 storage profile 的最近检测结果。
2. 输入临时对象存储凭据，权限应高于普通只读列表。
3. 不归类为 create/update profile，避免用户误以为凭据会被保存。

请求示例：

```json
{
  "accessKey": "temporary-access-key",
  "secretKey": "temporary-secret-key",
  "sessionToken": "",
  "useSsl": false,
  "usePathStyle": true,
  "insecureSkipVerify": false
}
```

返回：

```text
DatabaseStorageProfileVO
```

其中包含：

1. `postureStatus`
2. `postureStatusText`
3. `postureSummary`
4. `postureJson`
5. `lastPostureCheckAt`

#### P2.6.12 只读检测 API

S3/MinIO 第一版使用以下只读 API：

| 能力 | API | 说明 |
| --- | --- | --- |
| 可访问性 | `HeadBucket` | 失败则整体 `failed` |
| 区域 | `GetBucketLocation` | 可选辅助信息 |
| 版本化 | `GetBucketVersioning` | 检测 `Enabled / Suspended / 空` |
| 对象锁 | `GetObjectLockConfiguration` | 检测 Object Lock、mode 和默认保留期 |
| 默认加密 | `GetBucketEncryption` | 检测 SSE-S3 / SSE-KMS 和 KMS key |
| 公开策略 | `GetBucketPolicyStatus` | 检测 AWS S3 policy 是否 public；MinIO 可能不支持 |
| 公开访问阻断 | `GetPublicAccessBlock` | AWS S3 优先；MinIO 可能不支持 |

兼容策略：

1. `HeadBucket` 失败：整体 `failed`。
2. 可选 API 不支持：单项 `unsupported`，整体通常为 `warning`，提示人工核验。
3. 可选 API 返回“未配置”：按能力判断为 `warning` 或 `unsupported`。
4. 登记值要求开启，但检测值未开启：`warning`。
5. 检测到 bucket policy public：`warning`。
6. KMS Key 登记值和检测值不一致：`warning`。
7. 全部关键检查通过：`passed`。

#### P2.6.12 前端展示

在数据库管理的 `备份任务 -> PITR 链路与恢复计划` 中新增 `存储配置` 子页：

1. 列出 storage profile：
   - 名称
   - 存储类型
   - bucket / endpoint
   - path prefix
   - 登记安全值：版本化、不可变保留、KMS、保留天数
   - 最近姿态状态
   - 最近检测时间
   - 差异/风险摘要
2. 新增存储配置弹窗：
   - 只登记位置和期望安全值。
   - 明确提示不保存明文密钥。
3. 检测弹窗：
   - 临时输入 access key / secret key / session token。
   - 可选择 HTTPS、Path Style、跳过 TLS 校验。
   - 检测完成后自动打开详情。
4. 详情弹窗：
   - 展示整体状态、摘要、检查项表格。
   - 展示登记值和检测值。
   - 展示原始 `posture_json`，便于审计和问题排查。

#### P2.6.12 不做的事

1. 不保存对象存储 access key、secret key、session token。
2. 不自动修改 bucket 配置。
3. 不自动开启 versioning、object lock、default encryption 或 bucket policy。
4. 不删除或修复已有对象。
5. 不把 `posture_json` 当作 PITR 恢复输入；PITR 仍以 finalized archive 和日志链元数据为准。
6. 不对 OSS/COS 做主动检测；第一版标记为 `unsupported`，后续按厂商 SDK 单独接入。

#### P2.6.12 验收标准

1. S3/MinIO profile 可执行安全姿态检测。
2. 检测凭据不落库。
3. `HeadBucket` 失败时返回明确失败摘要。
4. 版本化、对象锁、默认加密、KMS、公开访问能力能分别展示登记值和检测值。
5. MinIO 不支持的 AWS 专属 API 不导致整体崩溃，而是标记为需人工核验。
6. 非 S3/MinIO 存储类型显示 `unsupported`。
7. 前端列表能展示最近检测状态、时间和摘要。
8. 前端详情能展示检查项和原始 JSON。

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
| `validationAssertions` | array | 否 | 带断言的校验 SQL，断言失败会让 proof 标记 `validation_status=failed` |
| `cleanupOnFailure` | bool | 否 | 失败后是否自动清理临时目录 |

`validationAssertions` 元素结构：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `sql` | string | 是 | 只读校验 SQL，仍受 SQL 安全分析和默认 LIMIT 保护 |
| `expectedRows` | int | 否 | 期望结果行数，不包含表头；适合校验明细查询返回几行 |
| `expectedContains` | string | 否 | 期望校验输出包含固定文本；按原始 tabular 输出做 fixed-string 匹配 |
| `expectedScalar` | string | 否 | 期望首行首列值；适合 `SELECT COUNT(*)`、`SELECT MAX(id)` 等聚合校验 |

断言规则：

1. `validationSql` 只判断 SQL 是否成功执行，并把输出写入 proof。
2. `validationAssertions` 至少配置 `expectedRows / expectedContains / expectedScalar` 之一。
3. 同一条断言可以同时配置多个 expected 字段，必须全部满足才算通过。
4. `expectedRows` 统计结果集数据行数，不包含表头；如果要判断 `COUNT(*) = 2`，应使用 `expectedScalar=2`。
5. SQL 执行失败或任一断言失败，恢复任务仍可能已经成功拉起隔离库，但 proof 的 `validationStatus` 会标记为 `failed`，恢复计划状态保持为“已恢复但未验证通过”。

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
9. `run_validation_sql`：执行默认、用户配置的只读校验 SQL，以及带 `expectedRows / expectedContains / expectedScalar` 的断言校验。
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

`validationResults` 中每条校验结果建议包含：

| 字段 | 说明 |
| --- | --- |
| `sql` | 实际执行的校验 SQL |
| `status` | SQL 执行与断言综合状态，`success / failed` |
| `outputPreview` | 校验输出预览，用于人工复核 |
| `expectedRows / expectedContains / expectedScalar` | 用户配置的断言 |
| `actualRows` | 实际结果行数，不含表头 |
| `actualScalar` | 实际首行首列 |
| `assertionStatus` | `not_configured / passed / failed / unknown` |
| `assertionMessage` | 断言失败原因 |

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

#### P2.7 当前落地边界

本期第一版已按 SSH Runner 方式落地隔离恢复闭环：

1. 恢复计划预校验通过后，可以通过 `POST /api/v1/databases/restore-plans/{id}/run` 下发隔离恢复任务。
2. 后端创建 `database_restore_jobs` 和 `database_runner_jobs`，记录 Runner、工作目录、容器、监听端口、步骤、校验结果和 proof。
3. Runner 执行受控 shell 模板，恢复输入只允许来自 `runner://runner-host-<id>/...`、`local://...`、`file://...` 或绝对路径；`metadata://` 只允许用于预校验，不允许作为真实恢复输入。
4. 物理备份准备使用 `xtrabackup` 或 `mariadb-backup`，先 prepare base backup，再按顺序 apply incremental chain。
5. 隔离实例以 Docker 容器启动，端口默认绑定 `127.0.0.1`，不覆盖生产 datadir，不自动切换业务连接，不自动回填生产数据。
6. binlog 使用 `mysqlbinlog` 或 `mariadb-binlog` 回放到目标时间；P2.7 第一版优先支持按时间恢复。
7. 校验 SQL 继续使用只读白名单，只允许 `SELECT / SHOW / DESC / DESCRIBE / EXPLAIN` 类语句。
8. 恢复完成后生成 `proof_json`，包含恢复计划、base backup、incremental chain、binlog chain、Runner 输出、步骤状态、校验摘要和最终状态。
9. 前端已提供恢复计划执行入口、隔离恢复弹窗、恢复任务列表、隔离库连接信息、proof 查看和清理按钮。
10. 清理动作只允许删除安全工作目录格式 `/restore/job-<id>` 下的恢复目录，并尝试移除对应隔离容器。

本期仍不做：

1. 不在 backend 容器里直接操作 datadir。
2. 不做生产库覆盖恢复、自动切流或自动回填。
3. 不做对象存储 artifact 直拉；对象存储文件需要先由 Runner 可访问的挂载路径、`runner://` 路径或后续下载器提供。
4. 不做长期运行恢复 Worker 池；本期由后端下发 SSH Runner 任务并记录审计。
5. 不承诺 PostgreSQL 隔离恢复；PostgreSQL 仍放在后续 Barman/WAL-G/pg_basebackup 专项。

### P2.8-P2.13：MySQL/MariaDB 自动增量链和 Synthetic Full 策略

本章节是 P2.7 之后的 MySQL/MariaDB 物理备份深化规划。当前落地状态：

| 阶段 | 状态 | 说明 |
| --- | --- | --- |
| P2.8 备份策略模型 | 已落地 | 新增策略表、链状态表、策略 API、前端“备份策略”页签、调度器接入 |
| P2.9 Runner 化 MySQL/MariaDB 物理备份 | 已落地 | 策略可手动/定时下发 full 或 incremental 到 SSH Runner，后端自动选择增量父记录 |
| P2.10 自动增量链选择和链路校验 | 已落地 | 增量自动选择上一条成功物理备份作为父记录；链路校验覆盖父链、策略归属、备份引擎、来源实例、checksum、checkpoint、Runner 可读 URI、链状态持久化和前端校验入口 |
| P2.11 Synthetic Full | 已落地 | 支持合成预览、SSH Runner 白名单执行、全量+增量 prepare/merge、生成 synthetic full 记录、更新链状态和前端合成入口 |
| P2.12 Purge 门禁 | 已落地 | 支持 proof/binlog/artifact/依赖/purge 窗口清理预览；proof 通过后旧链标记 superseded；执行清理仅标记 expired/purged 并保留 URI/checksum 审计 |
| P2.13 自动 Synthetic Full 调度 | 已落地 | 每次 incremental 成功后按规则检查链长，满阈值自动触发 synthetic full，把当前 base + N 条 incremental 合成为新全量基线 |

目标是把“单个物理备份任务”升级为“可长期运行的备份策略编排”，让 OpsHub 可以自动回答并执行：

1. 今天应该跑 full 还是 incremental。
2. incremental 应该依赖哪一条上一备份记录。
3. 上一备份 artifact 是否仍可读、可校验、可作为 `incrementalBaseDir`。
4. full + incremental chain + binlog 是否仍可 PITR。
5. 运行一段时间后，能否把一段旧 incremental 安全合成新的 synthetic full。
6. 旧 full / incremental 何时才允许进入清理窗口。

#### 为什么不能只改现有 backup task

当前 `database_backup_tasks` 已能表达 `backup_method=physical` 和 `backup_level=incremental`，但 MySQL/MariaDB 增量备份有几个额外约束，不能只靠一个固定 cron 和一个固定 `incrementalBaseDir` 长期运行：

1. `xtrabackup --incremental-basedir` 必须指向上一份可读的 full 或 incremental 目录。
2. OpsHub 当前物理备份完成后会打包为 `.physical.tar.gz`，临时目录会被删除；下一次增量前需要先把 parent artifact 解包到 Runner staging。
3. 增量链必须校验 LSN 连续性，不能只按时间选择上一条记录。
4. 备份源发生主从切换、server UUID 变化、备份工具版本变化时，链路可能不能继续复用。
5. synthetic full 不是简单修改元数据，而是需要 Runner 真实执行 prepare/apply-log，生成新的 artifact，并做隔离恢复证明。

因此本能力应该新增“策略编排层”，由策略自动派生 full / incremental / synthetic full / restore drill，而不是让用户手工维护每天变化的 `incrementalBaseDir`。

#### 推荐生产策略

默认推荐策略：

```text
full: 每周 1 次
incremental: 每天 1 次
binlog archive: 连续归档
synthetic full: 可选，按链长或增量数量触发
restore drill: synthetic full 成功后必须演练
purge: 只有恢复证明成功后才允许清理旧链
```

如果用户明确希望每月全量：

```text
full: 每月 1 次
incremental: 每天 1 次
binlog archive: 连续归档，保留期必须覆盖整个月度链
synthetic full: 建议每周滚动合成，避免恢复时应用过长增量链
restore drill: 至少每周一次
purge: synthetic full + 后续增量 + binlog 恢复演练成功后再清理
```

月度 full 的风险：

1. 最坏恢复需要应用接近 30 条 incremental 和对应 binlog。
2. 任意一个关键 incremental artifact 损坏都会影响后续恢复。
3. 恢复时间、临时磁盘、Runner CPU/IO 压力会明显增加。
4. 对 binlog 归档连续性的要求更高。

#### P2.8：备份策略模型

目标：新增策略层，统一描述 full / incremental / synthetic full / restore drill / purge。

新增表建议：

```text
database_backup_policies
```

核心字段：

| 字段 | 说明 |
| --- | --- |
| `instance_id` | 主实例 |
| `source_instance_id` | 实际备份源，可为主库、实时从库或专用备份从库 |
| `source_role` | `primary / replica / delayed_replica / backup_replica` |
| `name` | 策略名称 |
| `engine` | `mysql / mariadb` |
| `backup_engine` | `xtrabackup_8_0 / xtrabackup_8_4 / xtrabackup_2_4 / mariadb_backup` |
| `runner_host_id` | 执行物理备份和 synthetic full 的 Runner |
| `storage_profile_id` | 备份 artifact 存储配置 |
| `secret_profile_id` | 可选，外部凭据引用 |
| `full_schedule` | full cron，例如 `0 2 1 * *` |
| `incremental_schedule` | incremental cron，例如 `0 3 * * *` |
| `synthetic_enabled` | 是否启用 synthetic full |
| `synthetic_rule_json` | synthetic full 触发规则 |
| `restore_drill_required` | synthetic full 或链路切换前是否必须恢复演练 |
| `binlog_stream_id` | 绑定的 binlog 归档流 |
| `retention_json` | full、incremental、binlog、synthetic 的保留策略 |
| `enabled` | 是否启用 |
| `status` | `pending / active / degraded / failed / disabled` |
| `last_full_at` | 最近 full 成功时间 |
| `last_incremental_at` | 最近 incremental 成功时间 |
| `last_synthetic_at` | 最近 synthetic full 成功时间 |
| `last_restore_drill_at` | 最近恢复演练成功时间 |
| `last_error` | 最近错误 |

`synthetic_rule_json` 示例：

```json
{
  "mode": "weekly_consolidation",
  "triggerAfterDays": 7,
  "mergeOldestIncrementals": 4,
  "maxIncrementalsBeforeSynthetic": 10,
  "requireRestoreProof": true,
  "markSupersededAfterProof": true
}
```

`retention_json` 示例：

```json
{
  "fullKeepMonths": 6,
  "incrementalKeepDays": 45,
  "syntheticKeepMonths": 6,
  "binlogKeepDays": 45,
  "supersededKeepDaysAfterProof": 7,
  "neverDeleteWithoutProof": true
}
```

新增链状态表建议：

```text
database_backup_chain_states
```

核心字段：

| 字段 | 说明 |
| --- | --- |
| `policy_id` | 所属策略 |
| `instance_id` | 主实例 |
| `chain_id` | 当前链 ID |
| `current_base_record_id` | 当前有效 base full 或 synthetic full |
| `latest_record_id` | 当前链最新成功记录 |
| `latest_full_record_id` | 最近原生 full |
| `latest_synthetic_record_id` | 最近 synthetic full |
| `incremental_count` | 当前 base 后累计 incremental 数 |
| `chain_started_at` | 当前链开始时间 |
| `last_success_at` | 最近成功时间 |
| `recoverable_until` | 当前链理论可恢复终点 |
| `status` | `healthy / degraded / broken / consolidating` |
| `last_validation_status` | 最近链路校验结果 |
| `last_error` | 最近错误 |

设计原则：

1. `database_backup_tasks` 继续保留，用于单次或简单 cron 任务。
2. `database_backup_policies` 负责长期策略编排。
3. 策略可以自动生成内部运行记录，但不要求暴露为多个用户手工维护的 backup task。
4. 全量、增量、synthetic full 的最终事实仍落在 `database_backup_records`。
5. PITR 恢复计划仍只读取 `database_backup_records` 和 `database_log_archives`，不直接依赖策略表。

#### P2.9：Runner 化 MySQL/MariaDB 物理备份

目标：MySQL/MariaDB 物理备份不再依赖 `opshub-api` 容器本地安装 `xtrabackup`，改由 Runner 执行。

新增 Runner job 类型建议：

```text
mysql_physical_backup
mysql_physical_backup_prepare_parent
mysql_physical_backup_pack
```

Runner 前置条件：

1. 能访问源库地址和端口。
2. 安装匹配数据库版本的工具：
   - MySQL 5.7：`xtrabackup` 2.4。
   - MySQL 8.0.x：`xtrabackup` 8.0。
   - MySQL 8.4.x：`xtrabackup` 8.4。
   - MariaDB：`mariadb-backup`。
3. 有足够本地 staging 空间，至少能容纳：
   - 当前备份输出。
   - parent 解包目录。
   - synthetic full 临时目录。
4. 能读取或下载 parent artifact。
5. 能把最终 artifact 发布到 `storage_profile` 指定存储。

full 执行步骤：

```text
1. 后端根据策略创建 backup record，状态 running。
2. 下发 Runner job。
3. Runner 创建 staging workdir。
4. Runner 执行 xtrabackup/mariadb-backup --backup。
5. Runner 读取 xtrabackup_checkpoints。
6. Runner 读取 xtrabackup_binlog_info 或 mariadb_backup_binlog_info。
7. Runner 打包 artifact。
8. Runner 计算 SHA256。
9. Runner 上传或落地 storage URI。
10. 后端登记 record 为 success，更新 chain state。
```

incremental 执行步骤：

```text
1. 后端锁定 policy + instance，避免并发执行。
2. 后端选择 parent record。
3. 后端校验 parent 状态、checksum、server UUID、backup engine、tool version、LSN。
4. Runner 准备 parent basedir：
   - 优先复用已校验 staging cache。
   - 否则解包 parent artifact。
   - 校验 SHA256 和 checkpoints。
5. Runner 执行 xtrabackup/mariadb-backup --backup --incremental-basedir=<parent_basedir>。
6. Runner 读取 incremental checkpoints。
7. 校验 incremental.from_lsn == parent.to_lsn 或符合工具允许的连续性规则。
8. Runner 打包 incremental artifact。
9. 后端登记 incremental record：
   - `base_record_id`
   - `parent_record_id`
   - `chain_id`
   - `backup_level=incremental`
10. 更新 chain state。
```

失败保护：

1. parent artifact 不可读时，不执行增量。
2. parent checksum 不匹配时，不执行增量。
3. LSN 不连续时，record 标记 failed，chain state 标记 degraded 或 broken。
4. server UUID 不一致时，默认阻止继续链路，除非存在明确 promotion history。
5. 工具版本不兼容时阻止执行。

P2.8/P2.9 已落地实现边界：

1. 后端新增 `database_backup_policies` 和 `database_backup_chain_states` 自动迁移。
2. 策略 API：
   - `GET /api/v1/databases/backup-policies`
   - `POST /api/v1/databases/backup-policies`
   - `PUT /api/v1/databases/backup-policies/:id`
   - `DELETE /api/v1/databases/backup-policies/:id`
   - `GET /api/v1/databases/backup-policies/:id/chain`
   - `POST /api/v1/databases/backup-policies/:id/run-full`
   - `POST /api/v1/databases/backup-policies/:id/run-incremental`
3. 策略只支持 MySQL/MariaDB 物理备份，PostgreSQL 仍走 P3 的 Barman / pg_basebackup 链路。
4. 调度器会读取启用策略：
   - full schedule 到期时优先跑 full；
   - incremental schedule 到期时跑 incremental；
   - 同一策略和同一实例有互斥锁，避免并发备份打断链。
5. 增量 parent 第一版自动选择策略链状态里的 `latest_record_id`。
6. parent 第一版必须满足：
   - 同一策略；
   - 成功状态；
   - 物理备份；
   - full 或 incremental；
   - 备份引擎一致；
   - 来源实例一致；
   - checksum 存在；
   - checkpoint `to_lsn` 存在；
   - artifact URI 可被当前 Runner 读取。
7. Runner 执行方式：
   - 后端通过 SSH Runner 下发内置白名单脚本；
   - 使用临时 `mysql-client.cnf` 传递数据库凭据，避免把密码放进 `xtrabackup --password` 参数；
   - full 执行 `xtrabackup/mariadb-backup --backup --target-dir=...`；
   - incremental 先解包 parent artifact，再执行 `--incremental-basedir=<parent_dir>`；
   - 成功后打包 `.physical.tar.gz`，登记 SHA256、checkpoint、binlog info、manifest 和 Runner URI。
8. 前端在 PITR 区域新增“备份策略”页签：
   - 创建/编辑策略；
   - 查看 full / incremental cron；
   - 查看当前 base/latest/incremental count；
   - 手动触发 full / incremental；
   - 查看最近运行状态。
9. 当前暂不自动上传策略物理备份 artifact 到对象存储。第一版登记 `runner://runner-host-<id>/<path>`，要求恢复 Runner 或后续同步机制可读该路径。
10. P2.8/P2.9 阶段不执行 synthetic full，不清理 superseded 链，不自动要求恢复演练后 purge；P2.11 已补 synthetic full 第一版，P2.12 继续处理恢复证明后的 superseded/purge 门禁。

#### P2.10：自动增量链选择和链路校验

目标：让 incremental 不需要用户手工填写 `incrementalBaseDir`。

parent 选择规则：

1. 优先使用同一策略、同一 `chain_id`、状态 `success` 的最新物理记录。
2. 只允许选择 `backup_level=full`、`backup_level=incremental` 或 `backup_origin=synthetic_full` 的记录。
3. 必须满足：
   - `backup_method=physical`
   - `backup_engine` 与策略一致
   - `instance_id` 与策略一致
   - `source_instance_id` 与策略一致或在允许的备份源集合内
   - `server_uuid` 一致
   - `to_lsn` 可读
   - artifact 可读且 checksum 通过
4. 如果没有 parent：
   - 若允许自动 full，则本次改跑 full。
   - 若不允许自动 full，则任务失败并提示需要先跑 full。

需要扩展 `database_backup_records` 或 `manifest_json` 的字段：

```text
backup_origin: native_full / native_incremental / synthetic_full / external
checkpoint_from_lsn
checkpoint_to_lsn
checkpoint_last_lsn
checkpoint_backup_type
artifact_state: local / remote / cached / missing / checksum_failed
artifact_cache_uri
synthetic_source_record_ids
superseded_by_record_id
purge_eligible_at
protected_until
```

如果不想立刻扩表，第一版可以先把上述内容放入 `manifest_json`，但用于查询和链路校验的字段建议最终落列。

链路校验内容：

1. base 是否存在。
2. parent 是否存在。
3. incremental 顺序是否连续。
4. 每个 artifact 是否可读。
5. 每个 artifact checksum 是否匹配。
6. 每个 record 的 `from_lsn / to_lsn` 是否连续。
7. binlog 归档是否覆盖 base 之后的恢复窗口。
8. 是否存在 server UUID / GTID / binlog 文件断链。

新增 API 建议：

```text
GET  /api/v1/databases/backup-policies
POST /api/v1/databases/backup-policies
PUT  /api/v1/databases/backup-policies/:id
POST /api/v1/databases/backup-policies/:id/run-full
POST /api/v1/databases/backup-policies/:id/run-incremental
POST /api/v1/databases/backup-policies/:id/validate-chain
GET  /api/v1/databases/backup-policies/:id/chain
```

验收标准：

1. full 成功后，chain state 指向该 full。
2. incremental 自动选择 full 作为 parent。
3. 第二次 incremental 自动选择上一次 incremental 作为 parent。
4. 删除或破坏 parent artifact 后，incremental 被阻止。
5. LSN 不连续时，链路标记 broken。
6. 同一实例不会同时跑两个物理备份。
7. full 到期日不会重复执行 full 和 incremental。

P2.10 已落地实现边界：

1. 新增链路校验 API：
   - `POST /api/v1/databases/backup-policies/:id/validate-chain`
   - `GET /api/v1/databases/backup-policies/:id/chain` 仍返回当前链状态。
2. 后端新增链路校验模型输出：
   - `status / statusText`
   - `backupChainStatus / backupChainStatusText`
   - `baseRecord`
   - `latestRecord`
   - `records`
   - `selectedRecordIds`
   - `blockingReasons`
   - `warnings`
   - `messages`
   - `checkedAt`
   - `chain`
3. 增量备份触发前不再盲信 `database_backup_chain_states.latest_record_id`：
   - 先读取同一策略下成功的物理备份记录；
   - 以当前 `current_base_record_id` 为优先基线；
   - 如果链状态缺失但存在成功 full，则选择最近成功 full 作为校验基线；
   - 从 base 开始按 `parent_record_id` 追溯连续子链；
   - 校验通过后，自动选择链上最新记录作为本次 incremental parent。
4. 链路阻断条件：
   - 缺少 base；
   - 链状态中的 latest 不能从 base 连续追溯；
   - 父子 `parent_record_id/base_record_id` 不连续；
   - `checkpoint_from_lsn` 与父记录 `checkpoint_to_lsn` 不一致；
   - artifact 状态为 `missing/checksum_failed`；
   - 缺少 checksum；
   - 缺少 checkpoint `to_lsn`；
   - artifact URI 不能被当前 Runner 读取；
   - 同一链内存在明确的 `server_uuid` 不一致。
5. 链路提示但不阻断的场景：
   - 策略未绑定 binlog 归档流；
   - 同一父记录存在多个子增量时，按时间选择最早子链并提示风险。
6. 校验结果会同步写回 `database_backup_chain_states`：
   - 通过时：`status=healthy`，`last_validation_status=complete`，更新 base/latest/incremental count；
   - 失败时：`status=broken`，`last_validation_status` 写入具体断链类型，`last_error` 保存阻断摘要。
7. 前端备份策略页新增“校验链”操作：
   - 成功显示通过消息；
   - 有 warning 显示第一条风险；
   - 有 blocking reason 显示失败原因；
   - 操作后刷新策略链状态。
8. 单测已覆盖：
   - full + incremental + incremental 完整链；
   - LSN 不连续导致 `broken_chain`；
   - 旧的 incremental parent 校验继续保留。

#### P2.11：Synthetic Full / 合成全量

目标：把一段旧 full + incremental 合成为新的 synthetic full，缩短恢复链长度，但不破坏旧链。

示例：

```text
full_0 -> inc_1 -> inc_2 -> inc_3 -> inc_4 -> inc_5 -> inc_6
```

合成前四条增量：

```text
full_0 + inc_1 + inc_2 + inc_3 + inc_4 => synthetic_full_4
synthetic_full_4 -> inc_5 -> inc_6
```

触发条件建议：

1. 链运行满 `triggerAfterDays`。
2. 当前 base 后 incremental 数量超过阈值。
3. 最早 N 条 incremental 已经超过用户配置的压缩窗口。
4. 所有待合成记录 artifact 可读且 checksum 通过。
5. binlog 归档覆盖 synthetic full 之后的恢复窗口。
6. 当前没有正在运行的备份、恢复、合成或清理任务。

Runner 执行步骤：

```text
1. 创建 synthetic workdir。
2. 解包 full_0 到 base dir。
3. 解包 inc_1..inc_4 到 incremental dirs。
4. 校验所有 artifact SHA256。
5. 校验所有 xtrabackup_checkpoints。
6. prepare base：
   xtrabackup --prepare --apply-log-only --target-dir=<base>
7. 依次 apply incremental：
   xtrabackup --prepare --apply-log-only --target-dir=<base> --incremental-dir=<inc_1>
   xtrabackup --prepare --apply-log-only --target-dir=<base> --incremental-dir=<inc_2>
   xtrabackup --prepare --apply-log-only --target-dir=<base> --incremental-dir=<inc_3>
   xtrabackup --prepare --apply-log-only --target-dir=<base> --incremental-dir=<inc_4>
8. 读取最终 checkpoints。
9. 打包 synthetic full artifact。
10. 计算 checksum。
11. 登记新的 backup record。
```

synthetic full 记录建议：

```text
backup_method = physical
backup_level = full
backup_origin = synthetic_full
backup_engine = xtrabackup_8_0 / xtrabackup_8_4 / mariadb_backup
base_record_id = self
parent_record_id = inc_4
chain_id = 新链 ID 或原链延续 ID
synthetic_source_record_ids = [full_0, inc_1, inc_2, inc_3, inc_4]
checkpoint_from_lsn = full_0.from_lsn
checkpoint_to_lsn = inc_4.to_lsn
prepare_status = synthetic_prepared
restore_test_status = pending
```

关键边界：

1. 不直接修改原 full 目录或原 artifact。
2. synthetic full 是新 artifact，不是覆盖旧 artifact。
3. synthetic full 成功不代表旧链可删。
4. 必须经过隔离恢复演练后，旧链才允许进入 superseded 状态。
5. 如果 synthetic full 失败，旧链继续保持可用。

新增 Runner job 类型建议：

```text
mysql_synthetic_full
mysql_synthetic_full_validate
mysql_synthetic_full_pack
```

新增 API 建议：

```text
POST /api/v1/databases/backup-policies/:id/synthetic-full/preview
POST /api/v1/databases/backup-policies/:id/synthetic-full/run
GET  /api/v1/databases/backup-policies/:id/synthetic-full/jobs
```

preview 返回内容：

```json
{
  "selectedBaseRecordId": 100,
  "selectedIncrementalRecordIds": [101, 102, 103, 104],
  "newSyntheticFullAfterRecordId": 104,
  "estimatedInputSize": 1234567890,
  "estimatedWorkdirSize": 2469135780,
  "requiresRestoreProof": true,
  "blockingReasons": []
}
```

验收标准：

1. 可以从 full + 多条 incremental 生成 synthetic full。
2. synthetic full 登记为新的 full 记录。
3. synthetic full 不覆盖原始 full。
4. 合成过程中任一 artifact 缺失或 checksum 错误都会失败。
5. 合成后可以用 synthetic full + 后续 incremental + binlog 创建 PITR 恢复计划。
6. synthetic full 未恢复演练前，旧链不能自动删除。

P2.11 已落地实现边界：

1. 新增 API：
   - `POST /api/v1/databases/backup-policies/:id/synthetic-full/preview`
   - `POST /api/v1/databases/backup-policies/:id/synthetic-full/run`
   - `GET /api/v1/databases/backup-policies/:id/synthetic-full/jobs`
2. `preview` 会先复用 P2.10 链路校验，再按策略 `synthetic_rule_json` 选择待合成窗口。
3. 第一版规则字段：
   - `mergeOldestIncrementals`：从当前 base 后选择最早 N 条增量合并，默认 `4`；
   - `requireRestoreProof`：提示合成后需要恢复证明；真正清理门禁仍在 P2.12。
4. `preview` 返回：
   - `selectedBaseRecordId`
   - `selectedIncrementalRecordIds`
   - `selectedRecordIds`
   - `selectedRecords`
   - `newSyntheticFullAfterRecordId`
   - `estimatedInputSize`
   - `estimatedWorkdirSize`
   - `requiresRestoreProof`
   - `blockingReasons`
   - `warnings`
5. `run` 阻断条件：
   - 策略未启用；
   - 未开启 `synthetic_enabled`；
   - P2.10 备份链校验失败；
   - 可合并增量数量小于 `mergeOldestIncrementals`；
   - 同一策略或同一实例已有备份任务运行。
6. Runner job：
   - 新增 `job_type=mysql_synthetic_full`；
   - 新增 `allowed_command=mysql_synthetic_full`；
   - 使用同一 Runner 互斥机制；
   - 只通过 SSH Runner 下发内置白名单脚本；
   - 不需要数据库在线连接，不读取数据库密码，只读取 Runner 可访问的物理备份 artifact。
7. Runner 脚本步骤：
   - 检查 `xtrabackup/mariadb-backup` 工具存在；
   - 校验所有输入 artifact 文件存在；
   - 对每个输入 artifact 做 SHA256 校验；
   - 解包 base；
   - 解包每条 incremental；
   - 检查每个目录的 `xtrabackup_checkpoints`；
   - `--prepare --apply-log-only` 准备 base；
   - 依次 `--prepare --apply-log-only --incremental-dir` 应用增量；
   - 最后执行一次 `--prepare --target-dir=<base>`；
   - 打包新的 synthetic full artifact；
   - 输出 artifact size、SHA256、checkpoint、binlog info 和 Runner URI。
8. synthetic full 记录：
   - `backup_method=physical`
   - `backup_level=full`
   - `backup_origin=synthetic_full`
   - `base_record_id=self`
   - `parent_record_id=<最后一条被合并的 incremental>`
   - `synthetic_source_record_ids=[base, inc...]`
   - `prepare_status=synthetic_ready`
   - `verify_status=pending`
   - `artifact_state=remote`
9. synthetic full 成功后：
   - 写入新的 backup record；
   - 更新 `database_backup_chain_states.current_base_record_id` 到 synthetic full；
   - `latest_record_id` 指向 synthetic full；
   - `latest_synthetic_record_id` 指向 synthetic full；
   - `incremental_count` 重置为 `0`；
   - 策略 `last_synthetic_at` 更新；
   - 旧 full/incremental 不自动删除、不自动 supersede。
10. synthetic full 失败后：
   - 当前 synthetic record 标记 failed；
   - Runner job 标记 failed；
   - 不覆盖旧链；
   - 不修改旧 artifact；
   - 不执行自动清理。
11. 前端备份策略页新增：
   - “合成预览”；
   - “合成Full”；
   - 预览弹窗展示 selected records、LSN、artifact、大小、可恢复时间、阻断原因和 warning。
12. 单测已覆盖：
   - preview 选择最早 N 条 incremental；
   - synthetic 脚本包含 artifact checksum 校验；
   - synthetic 脚本包含 prepare/apply incremental/输出 checkpoint；
   - synthetic 脚本不携带数据库密码参数。

#### P2.12：Synthetic Full 后的恢复演练和清理

目标：确保合成全量可恢复后，再把旧链标记为可清理。

P2.12 已落地实现边界：

1. synthetic full 创建/完成时，如果策略启用了 `requireRestoreProof`、`restoreDrillRequired` 或 `neverDeleteWithoutProof`，新 synthetic full 记录会进入 `restore_test_status=pending`，前端显示为待演练。
2. 恢复证明复用 P2.7 已实现的隔离恢复 Runner：用户基于 synthetic full 生成恢复计划并执行恢复任务，恢复任务 `verified` 后会把 synthetic full 备份记录更新为 `restore_test_status=success`。
3. 新增策略级清理预览接口：

```text
POST /api/v1/databases/backup-policies/:id/purge-preview
```

4. 新增策略级清理执行接口：

```text
POST /api/v1/databases/backup-policies/:id/purge
```

5. 清理预览会先找到当前策略最新成功的 `backup_origin=synthetic_full` 记录，再解析其 `synthetic_source_record_ids` 作为候选旧链。
6. 如果 synthetic full 已通过恢复证明，预览会对来源旧链做状态调和：
   - 设置 `superseded_by_record_id=<synthetic full record id>`；
   - 首次设置 `purge_eligible_at=now + supersededKeepDaysAfterProof`；
   - 不改变未通过 proof 的旧链。
7. 清理预览门禁包括：
   - synthetic full 必须是 `success`；
   - synthetic full artifact 不能是 `missing/checksum_failed`；
   - synthetic full 必须有 checksum；
   - `neverDeleteWithoutProof=true` 时 synthetic full 必须 `restore_test_status=success`；
   - 策略必须绑定 binlog 归档流；
   - binlog catalog 必须能从 synthetic full 的 `backup_binlog_file` 起点证明文件链连续；
   - 若有后续成功增量仍以待清理旧链记录为 `parent_record_id`，阻止清理；
   - 单条旧链记录必须已到达 `purge_eligible_at`，且不在 `protected_until` 保护期内。
8. `purge-preview` 返回：
   - `eligibleRecordIds`；
   - `blockedRecordIds`；
   - `requiredProofRecordId`；
   - `proofStatus`；
   - `binlogCoverageStatus`；
   - `storageDeletePlan`；
   - `blockedRecords`；
   - `blockingReasons`、`warnings`、`messages`。
9. `storageDeletePlan` 明确展示：
   - `recordId`；
   - `backupLevel`；
   - `backupOrigin`；
   - `fileName`；
   - `fileSize/fileSizeText`；
   - `storageUri`；
   - `filePath`；
   - `checksumSha256`；
   - `purgeEligibleAt`。
10. `purge` 执行是第一版安全清理：不在 backend 容器中硬删除 Runner 文件或对象存储对象，只把可清理旧链记录标记为：
    - `status=expired`；
    - `verify_status=expired`；
    - 保留 `storage_uri/file_path/checksum_sha256` 作为审计证据；
    - `verify_message/error_message` 记录 synthetic proof 和清理边界。
11. 前端备份策略页新增：
    - `清理预览`；
    - `清理旧链`；
    - 清理预览弹窗展示 proof、binlog 链、保护天数、可清理数、阻断数、清理计划和受保护记录。
12. 本阶段不做 artifact 物理删除。真正删除 Runner 本地文件、对象存储对象、S3 Object Lock/retention 过期后的 delete marker 管理，后续必须放到 Runner/对象存储保留策略中做，不能由普通 backend HTTP 请求直接执行。

清理状态建议：

```text
active
synthetic_created
restore_proof_pending
restore_proof_passed
superseded
purge_eligible
purged
purge_blocked
```

流程：

```text
1. synthetic full 生成成功。
2. 自动创建恢复计划：
   synthetic_full_4 + inc_5..inc_N + binlog。
3. 下发隔离恢复 Runner。
4. 执行 validation SQL 断言。
5. proof 成功后：
   - synthetic full 标记 restore_test_status=success。
   - full_0 + inc_1..inc_4 标记 superseded。
   - 设置 superseded_by_record_id=synthetic_full_4。
   - 设置 purge_eligible_at=now + supersededKeepDaysAfterProof。
6. 到达 purge_eligible_at 后，按保留策略提示或执行清理。
```

清理原则：

1. `neverDeleteWithoutProof=true` 时，没有恢复证明绝不删除。
2. 如果 binlog 归档断链，不允许清理旧链。
3. 如果 synthetic full artifact 丢失或 checksum 失败，旧链重新保护。
4. 如果后续 incremental 仍依赖旧 parent，不允许清理。
5. 清理动作必须审计，并记录删除的 artifact URI、checksum 和 record ID。

API 规划与当前落地：

```text
POST /api/v1/databases/backup-policies/:id/synthetic-full/:jobId/run-restore-proof
POST /api/v1/databases/backup-policies/:id/purge-preview
POST /api/v1/databases/backup-policies/:id/purge
```

当前 P2.12 已落地后两个清理 API；`run-restore-proof` 不新增独立入口，恢复证明复用现有恢复计划和隔离恢复 Runner：

```text
POST /api/v1/databases/restore-plans/:id/run
GET  /api/v1/databases/restore-jobs/:id/proof
```

后续如果要把“为某个 synthetic full 自动创建恢复计划并执行 proof”做成一键按钮，再补 `run-restore-proof` 编排接口。

purge preview 必须返回：

```json
{
  "eligibleRecordIds": [100, 101, 102, 103, 104],
  "blockedRecordIds": [],
  "requiredProofRecordId": 150,
  "proofStatus": "success",
  "binlogCoverageStatus": "complete",
  "storageDeletePlan": [
    {
      "recordId": 100,
      "storageUri": "s3://bucket/mysql/full_0.tar.gz",
      "checksumSha256": "..."
    }
  ]
}
```

验收标准：

1. synthetic full 成功后自动进入 `restore_proof_pending`。
2. 恢复演练成功后，旧链进入 `superseded`。
3. 恢复演练失败时，旧链保持 active/protected。
4. binlog 断链时 purge preview 阻止删除。
5. purge 删除前必须展示 record、storage URI、大小和 checksum。
6. purge 后 records 不直接硬删除，先标记 `expired/purged` 并保留审计。

#### P2.13：自动 Synthetic Full 调度

目标：把 P2.11 的手动 synthetic full 升级为长期自动调度能力，让 MySQL/MariaDB 物理增量链不会无限增长。典型生产策略是“先全量一次，之后每天增量；当前 base 后累计 N 条增量后，自动把 base + N 条增量合成为新的 synthetic full，第二天继续以新 synthetic full 为父记录跑增量”。

推荐链路：

```text
native_full_0
  -> inc_1
  -> inc_2
  -> inc_3
  -> inc_4
  -> inc_5
      => synthetic_full_5

synthetic_full_5
  -> inc_6
  -> inc_7
  -> inc_8
  -> inc_9
  -> inc_10
      => synthetic_full_10
```

推荐策略 JSON：

```json
{
  "mode": "rolling_synthetic_full",
  "autoRun": true,
  "triggerAfterIncrementals": 5,
  "mergeOldestIncrementals": 5,
  "requireRestoreProof": true,
  "neverDeleteWithoutProof": true,
  "markSupersededAfterProof": true,
  "supersededKeepDaysAfterProof": 7
}
```

字段语义：

| 字段 | 说明 |
| --- | --- |
| `mode` | `rolling_synthetic_full` 表示滚动合成全量 |
| `autoRun` | 是否允许调度器自动触发 synthetic full |
| `triggerAfterIncrementals` | 当前 base 后成功增量数达到该值时触发自动合成 |
| `mergeOldestIncrementals` | 本次合成纳入的最早增量数；推荐与 `triggerAfterIncrementals` 一致 |
| `requireRestoreProof` | synthetic full 成功后必须恢复演练 |
| `neverDeleteWithoutProof` | 没有 proof 成功时绝不清理旧链 |
| `markSupersededAfterProof` | proof 成功后自动把来源旧链标记为 superseded |
| `supersededKeepDaysAfterProof` | proof 成功后旧链继续保留的天数 |

为什么推荐 `triggerAfterIncrementals=5` 且 `mergeOldestIncrementals=5`：

1. 链路最清晰：每次合成都把当前 base 后的一整段增量合并掉。
2. 合成后 `database_backup_chain_states.current_base_record_id` 可以直接切换到新的 synthetic full。
3. 第二天的 incremental 自动以 synthetic full 为 parent。
4. 旧 base + 5 条 incremental 在 proof 成功后可以整体进入 superseded/purge 门禁。
5. 不需要做复杂的 incremental rebase / reparent。

不推荐第一版做 `triggerAfterIncrementals=5`、`mergeOldestIncrementals=3`：

1. 只合并前 3 条时，剩下 inc_4/inc_5 仍依赖旧 parent。
2. 要让 inc_4/inc_5 接到新的 synthetic full 后面，需要重写父链或重新生成增量。
3. 这属于 incremental rebase/reparent，风险高于滚动整段合成。
4. P2.12 purge 门禁会因为“后续增量仍依赖旧 parent”阻止清理旧链。

自动触发时机：

1. 调度器跑完 full 成功后：
   - 重置 chain state；
   - 不立即触发 synthetic full。
2. 调度器跑完 incremental 成功后：
   - 更新 chain state；
   - 检查 `synthetic_enabled=true`；
   - 解析 `synthetic_rule_json`；
   - 如果 `autoRun=true` 且 `incremental_count >= triggerAfterIncrementals`，进入自动 synthetic full 检查。
3. 手动跑 incremental 成功后：
   - 同样允许检查自动 synthetic full；
   - 但需记录触发来源为 `auto_after_incremental`。
4. synthetic full 成功后：
   - 更新当前 base 为 synthetic full；
   - `incremental_count=0`；
   - `restore_test_status=pending`；
   - 不自动清理旧链。

自动调度门禁：

1. 策略必须启用。
2. `synthetic_enabled=true`。
3. `synthetic_rule_json.autoRun=true`。
4. 当前没有同一策略、同一实例的 full/incremental/synthetic 正在运行。
5. 当前备份链校验必须通过。
6. 当前 base 后成功 incremental 数必须大于等于 `triggerAfterIncrementals`。
7. `mergeOldestIncrementals` 必须大于 0，且不能大于当前可合成增量数。
8. 推荐第一版要求 `mergeOldestIncrementals == triggerAfterIncrementals`；如果不相等，前端提示高风险，后端可直接拒绝自动执行。
9. 策略必须绑定 binlog 归档流；未绑定时可以允许 synthetic full 手动预览，但不允许自动执行。
10. Runner 主机必须在线或最近探测成功，且具备 `xtrabackup/mariadb-backup` 能力。
11. 旧 synthetic full 如果仍有运行中的 proof 或 purge，不阻止新一轮增量，但不允许清理旧链。

后端改造：

1. 扩展 `syntheticRuleConfig`：

```go
type syntheticRuleConfig struct {
    Mode                         string `json:"mode"`
    AutoRun                      bool   `json:"autoRun"`
    TriggerAfterIncrementals     int    `json:"triggerAfterIncrementals"`
    MergeOldestIncrementals      int    `json:"mergeOldestIncrementals"`
    RequireRestoreProof          bool   `json:"requireRestoreProof"`
    NeverDeleteWithoutProof      bool   `json:"neverDeleteWithoutProof"`
    MarkSupersededAfterProof     bool   `json:"markSupersededAfterProof"`
    SupersededKeepDaysAfterProof int    `json:"supersededKeepDaysAfterProof"`
}
```

2. `parseSyntheticRule` 默认值：

```text
mode=rolling_synthetic_full
autoRun=false
triggerAfterIncrementals=5
mergeOldestIncrementals=5
requireRestoreProof=true
neverDeleteWithoutProof=true
markSupersededAfterProof=true
supersededKeepDaysAfterProof=7
```

3. 新增内部方法：

```go
func (uc *UseCase) maybeRunBackupPolicySyntheticAfterSuccess(ctx context.Context, policyID uint, triggerRecordID uint, triggerType string)
```

职责：
   - 加载策略；
   - 加载 Runner；
   - 校验规则；
   - 校验链状态；
   - 调用 synthetic full preview；
   - 满足门禁时调用 synthetic full run 的内部实现；
   - 记录触发来源。

4. 拆分 `RunBackupPolicySyntheticFull`：
   - HTTP 手动入口继续保留；
   - 内部新增 `runBackupPolicySyntheticFull(ctx, id, operator, triggerType)`，支持手动和自动复用。

5. 自动 synthetic 的 `DatabaseBackupRecord`：

```text
trigger_type=schedule 或 auto_synthetic
backup_origin=synthetic_full
backup_level=full
restore_test_status=pending
synthetic_source_record_ids=[base, inc_1..inc_N]
```

如不新增枚举，也可以先用 `trigger_type=schedule`，在 `manifest_json/request_json` 里记录：

```json
{
  "triggerReason": "auto_after_incremental",
  "triggerRecordId": 123,
  "triggerAfterIncrementals": 5,
  "mergeOldestIncrementals": 5
}
```

6. 调度器接入点：
   - `executeScheduledPolicy` full/incremental 成功后；
   - 手动 full/incremental 成功后；
   - 只在 incremental 成功后触发 synthetic full 检查。

7. 互斥与防重复：
   - 复用现有 `acquireBackupPolicyRun(policy.ID, policy.InstanceID)`；
   - 增加“最近 synthetic full 是否已经覆盖当前 latest record”的检查；
   - 若存在 queued/running 的 `mysql_synthetic_full` Runner job，跳过；
   - 若 `last_synthetic_at` 晚于最新 incremental finished_at，跳过。

8. 策略状态：
   - 自动 synthetic 下发成功：`last_status=queued/running`，`last_message=自动 Synthetic Full 已下发`；
   - synthetic 成功：`last_synthetic_at` 更新，chain state 重置；
   - synthetic 失败：策略 `status=degraded`，旧链保持 active；
   - proof 未完成：策略不失败，但前端展示 “Synthetic Full 待恢复演练”。

前端改造：

1. 备份策略弹窗的 Synthetic 规则默认值改为：

```json
{
  "mode": "rolling_synthetic_full",
  "autoRun": true,
  "triggerAfterIncrementals": 5,
  "mergeOldestIncrementals": 5,
  "requireRestoreProof": true,
  "neverDeleteWithoutProof": true,
  "markSupersededAfterProof": true,
  "supersededKeepDaysAfterProof": 7
}
```

2. 在 `Synthetic Full` 开关旁展示：
   - 自动合成：开/关；
   - 触发增量数；
   - 合并增量数；
   - 是否要求 proof。
3. 如果 `mergeOldestIncrementals != triggerAfterIncrementals`，显示风险提示：
   - “第一版自动合成建议整段合并，否则旧链清理可能被后续增量依赖阻断。”
4. 备份策略表增加提示：
   - 当前 base；
   - 当前增量数；
   - 自动合成阈值；
   - 距离下次 synthetic 还差几条增量。
5. Runner 任务表支持筛选 `mysql_synthetic_full`，展示触发来源 `auto_after_incremental`。

恢复证明与清理关系：

1. 自动 synthetic full 成功后，新的 synthetic full 可以作为后续增量 parent。
2. 即使 proof 未完成，后续增量仍可以继续接在 synthetic full 后面，以避免链继续增长。
3. proof 未成功前，旧链不能进入 purge。
4. proof 成功后，P2.12 会把旧链标记为 superseded，并设置 `purge_eligible_at`。
5. purge preview 仍必须检查 binlog 链、artifact 状态、checksum 和后续依赖。

这种设计的取舍：

1. 优点：备份链短、恢复快、长期维护成本低。
2. 风险：如果 synthetic full 自动生成后长期不做 proof，系统会积累多条待证明 synthetic full 和旧链。
3. 控制方式：前端风险提示、策略 degraded 标记、定期 restore drill 和 purge preview。

验收标准：

1. 创建策略：月度 full、每日 incremental、`autoRun=true`、`triggerAfterIncrementals=5`、`mergeOldestIncrementals=5`。
2. 手动跑一次 full 成功，chain state 指向 full，`incremental_count=0`。
3. 连续跑 5 次 incremental，每次自动选择上一条成功记录作为 parent。
4. 第 5 次 incremental 成功后，系统自动下发 `mysql_synthetic_full` Runner job。
5. synthetic full 成功后：
   - 新 record 为 `backup_origin=synthetic_full`；
   - `base_record_id=self`；
   - `synthetic_source_record_ids` 包含 full + 5 条 incremental；
   - chain state `current_base_record_id` 指向 synthetic full；
   - `incremental_count=0`；
   - `restore_test_status=pending`。
6. 第 6 天 incremental 自动以 synthetic full 为 parent。
7. synthetic full 失败时：
   - 旧链仍保持 current；
   - 第 6 天 incremental 仍可按旧链继续，或策略 degraded 后等待人工处理；
   - 不清理任何旧 artifact。
8. synthetic full proof 成功后，旧 full + 5 条 incremental 可以通过 purge preview。
9. synthetic full proof 未成功时，purge preview 阻止清理。
10. 未绑定 binlog 归档流时，自动 synthetic 不触发或直接进入 warning/blocked。

`opshub-mysql` 推荐配置：

```text
backup_engine: xtrabackup_8_0
full_schedule: 0 2 1 * *
incremental_schedule: 0 3 * * *
binlog_archive: streaming 或 polling 连续归档流
synthetic_enabled: true
restore_drill_required: true
```

```json
{
  "mode": "rolling_synthetic_full",
  "autoRun": true,
  "triggerAfterIncrementals": 5,
  "mergeOldestIncrementals": 5,
  "requireRestoreProof": true,
  "neverDeleteWithoutProof": true,
  "markSupersededAfterProof": true,
  "supersededKeepDaysAfterProof": 7
}
```

生产使用流程：

1. 绑定并启动 binlog 归档流。
2. 保存备份策略。
3. 手动执行一次 full。
4. 确认第一天 incremental 成功。
5. 到第 5 条 incremental 成功后，观察自动 synthetic full Runner job。
6. synthetic full 成功后生成恢复计划并执行 proof。
7. proof 成功后通过清理预览确认旧链可清理。

#### 前端改造

新增或扩展页面：

1. `备份策略`
   - 创建 full/incremental/synthetic 策略。
   - 选择 Runner。
   - 选择 binlog 归档流。
   - 配置 full cron、incremental cron、synthetic rule 和 retention。
2. `备份链路`
   - 展示 full、incremental、synthetic full 的链路图。
   - 展示 base、parent、LSN、binlog 起点、恢复窗口。
3. `合成全量`
   - 预览将合成哪些记录。
   - 展示预计工作目录大小。
   - 展示阻塞原因。
   - 发起 synthetic full。
4. `恢复证明`
   - synthetic full 生成后提示必须做恢复演练。
   - 展示 proof 状态和校验 SQL 断言。
5. `清理预览`
   - 只展示可清理记录。
   - 阻塞项必须明确原因。

链路图示例：

```text
native_full_0
  -> inc_1
  -> inc_2
  -> inc_3
  -> inc_4
       => synthetic_full_4
            -> inc_5
            -> inc_6
```

风险提示：

1. 未绑定 binlog 归档流：只能恢复到备份点，不能 PITR。
2. binlog 归档延迟过大：RPO 不达标。
3. incremental 数量过多：恢复时间可能过长。
4. synthetic full 未恢复演练：旧链不能清理。
5. Runner staging 空间不足：不允许发起 synthetic full。
6. 当前 source 是 primary：提示优先使用备份从库或实时从库卸载压力。

#### 权限和审计

新增权限建议：

```text
database:backup-policy:view
database:backup-policy:create
database:backup-policy:update
database:backup-policy:delete
database:backup-policy:run-full
database:backup-policy:run-incremental
database:backup-policy:run-synthetic
database:backup-policy:purge-preview
database:backup-policy:purge
```

审计事件：

1. 创建、更新、禁用策略。
2. 手动触发 full。
3. 手动触发 incremental。
4. 手动触发 synthetic full。
5. synthetic full preview。
6. 恢复证明执行。
7. purge preview。
8. purge 执行。
9. 链路 broken/degraded。
10. artifact checksum 失败。

#### 总体验收矩阵

必须覆盖：

1. MySQL 8.0 monthly full + daily incremental。
2. MySQL 8.0 weekly full + daily incremental。
3. MariaDB full + incremental。
4. parent artifact 缺失。
5. parent checksum 错误。
6. LSN 不连续。
7. server UUID 不一致。
8. synthetic full 成功。
9. synthetic full 失败后旧链仍可恢复。
10. synthetic full + 后续 incremental + binlog PITR 成功。
11. synthetic full 未恢复证明时 purge 被阻止。
12. 恢复证明成功后 purge preview 正确。
13. purge 执行后审计完整。

#### 对 `opshub-mysql` 的建议落地策略

`opshub-mysql` 当前是 MySQL 8.0.44，后续策略建议：

```text
backup_engine: xtrabackup_8_0
full_schedule: 0 2 1 * *
incremental_schedule: 0 3 * * *
binlog_archive: 必须启用
synthetic_enabled: true
synthetic_rule:
  triggerAfterDays: 7
  mergeOldestIncrementals: 4
  requireRestoreProof: true
retention:
  fullKeepMonths: 6
  syntheticKeepMonths: 6
  binlogKeepDays: 45
  supersededKeepDaysAfterProof: 7
```

生产上线顺序：

1. 先准备安装 `xtrabackup_8_0` 的 Runner。
2. 开启 `opshub-mysql` binlog 归档流。
3. 手动跑一次 full。
4. 手动跑一次 incremental，验证 parent 自动选择。
5. 连续跑 3-7 天 incremental。
6. 执行 synthetic full preview。
7. 执行 synthetic full。
8. 用 synthetic full 创建并运行隔离恢复演练。
9. 恢复 proof 成功后，才允许旧链进入清理窗口。

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
| `posture_status` | varchar(30) | 对象存储安全姿态：`unknown / passed / warning / failed / unsupported` |
| `posture_summary` | varchar(1000) | 最近一次姿态检测摘要 |
| `posture_json` | text | 最近一次姿态检测明细 JSON，不包含临时密钥 |
| `last_posture_check_at` | datetime | 最近一次安全姿态检测时间 |

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

P3 的核心判断：

1. PostgreSQL 物理备份和 PITR 是 cluster 级能力，不是单 database/schema/table 能力。
2. 第一版深接 Barman，不同时深接 Barman、WAL-G、pg_basebackup 三套引擎。
3. WAL-G 先只做 external metadata registration，后续独立阶段再深接。
4. pgBackRest 只作为 legacy external 纳管，不作为新方案默认推荐。
5. `pg_basebackup` 作为轻量备用方案，但原生增量必须按 PostgreSQL 版本和 `pg_combinebackup` 能力门控。
6. PostgreSQL 恢复计划必须校验 `pg_system_identifier`、timeline、timeline history、WAL segment 连续性和 LSN 覆盖范围。
7. Barman restore 和隔离 PostgreSQL 启动要分阶段实现：先恢复到隔离目录，再启动隔离实例并执行校验 SQL。

#### P3 技术边界

本阶段做：

1. Barman server 登记和健康检查。
2. Barman backup catalog 同步到 OpsHub 备份记录。
3. Barman WAL 状态和 WAL catalog 同步到 OpsHub 日志归档记录。
4. Barman 物理备份任务触发。
5. PostgreSQL PITR 恢复计划生成和预校验。
6. Barman restore 到隔离目录。
7. 隔离 PostgreSQL 实例启动、target time / target LSN 恢复、校验 SQL 和恢复证明。
8. `pg_basebackup` 轻量备用方案的记录、触发和计划步骤。
9. WAL-G / pgBackRest 外部元数据登记和 UI 风险提示。

本阶段不做：

1. 不做生产库自动切换。
2. 不做 PostgreSQL 表级物理恢复。
3. 不做 WAL-G 深度执行链路。
4. 不把 pgBackRest 作为新建策略推荐。
5. 不在 backend 容器里直接跑 Barman/pg_basebackup/pg_ctl；这类命令仍走 Runner 主机。
6. 不承诺所有 PostgreSQL 版本都支持原生增量。
7. 不默认支持复杂 tablespace remap；检测到 tablespace 且没有明确 remap 配置时，恢复计划应阻断。

#### P3 数据模型补充

现有 `database_backup_records`、`database_log_archive_streams`、`database_log_archives`、`database_restore_plans` 已经具备 P1/P2 的大部分字段，P3 不重建一套备份系统，只补 PostgreSQL/Barman 必需模型。

新增表：

```text
database_barman_servers
```

建议字段：

| 字段 | 说明 |
| --- | --- |
| `source_instance_id` | 关联的 PostgreSQL 生产实例 |
| `runner_host_id` | 执行 Barman 命令的 Runner 主机 |
| `name` | OpsHub 内部展示名称 |
| `barman_server_name` | Barman 配置里的 server name |
| `barman_home` | Barman home/catalog 根目录，可选 |
| `config_path` | Barman 配置路径，可选 |
| `retention_policy` | Barman retention policy 展示值 |
| `backup_method` | Barman 配置中的 backup method 摘要 |
| `streaming_archiver_enabled` | 是否启用 WAL streaming |
| `archiver_enabled` | 是否启用 archive_command/put-wal 链路 |
| `slot_name` | 物理复制 slot 名称，可选 |
| `barman_version` | Barman 版本 |
| `pg_version` | 源 PostgreSQL 版本 |
| `pg_system_identifier` | PostgreSQL cluster system identifier |
| `wal_segment_size` | WAL segment size |
| `status` | pending / healthy / degraded / failed / disabled |
| `last_check_at` | 最近一次 `barman check` 时间 |
| `last_check_status` | 最近一次 check 结果 |
| `last_catalog_sync_at` | 最近 catalog 同步时间 |
| `last_wal_sync_at` | 最近 WAL 同步时间 |
| `last_error` | 最近错误 |
| `config_json` | 脱敏配置摘要，不保存明文密钥 |

`database_backup_records` 建议补充字段：

| 字段 | 说明 |
| --- | --- |
| `external_backup_id` | Barman backup ID / WAL-G backup name / pg_basebackup id |
| `external_server_name` | Barman server name 或外部工具 server name |
| `backup_scope` | `cluster / database / schema / table`，PostgreSQL 物理备份固定为 `cluster` |

已有 PostgreSQL 字段继续使用：

1. `pg_system_identifier`
2. `timeline_id`
3. `timeline_history_file`
4. `wal_segment_size`
5. `start_lsn`
6. `end_lsn`
7. `wal_start`
8. `wal_end`
9. `backup_label_json`
10. `backup_manifest_checksum`

`database_log_archives` 建议补充字段：

| 字段 | 说明 |
| --- | --- |
| `wal_segment_size` | WAL segment size，便于 LSN 到 segment 覆盖计算 |
| `external_server_name` | Barman server name |

已有 PostgreSQL 字段继续使用：

1. `pg_system_identifier`
2. `timeline_id`
3. `start_lsn`
4. `end_lsn`
5. `segment_no`
6. `timeline_history_uri`

#### P3 API 设计

Barman server：

```text
GET    /api/v1/databases/barman-servers
POST   /api/v1/databases/barman-servers
PUT    /api/v1/databases/barman-servers/:id
DELETE /api/v1/databases/barman-servers/:id
POST   /api/v1/databases/barman-servers/:id/check
POST   /api/v1/databases/barman-servers/:id/sync-catalog
POST   /api/v1/databases/barman-servers/:id/sync-wal
```

Barman backup：

```text
POST /api/v1/databases/barman-servers/:id/backup
```

也可以复用现有备份任务手动触发接口，但任务配置里要能选择：

1. `backup_method=physical`
2. `backup_engine=barman`
3. `backup_scope=cluster`
4. `barman_server_id`

PostgreSQL PITR 计划：

```text
POST /api/v1/databases/restore-plans
POST /api/v1/databases/restore-plans/:id/run
```

P3 要扩展 `restoreTargetType`：

1. `time`
2. `lsn`

后续再扩展：

1. `xid`
2. `restore_point`
3. `immediate`

#### P3 Runner 命令边界

Barman Runner 允许命令：

```text
barman --version
barman check <server>
barman status <server>
barman list-backups <server>
barman show-backup <server> <backup_id>
barman backup <server>
barman check-backup <server> <backup_id>
barman restore <server> <backup_id|auto> <destination_dir> [target options]
barman receive-wal <server> --create-slot --if-not-exists
barman receive-wal <server> --reset
barman terminate-process <server> receive-wal
```

首版不建议直接开放任意命令输入。所有命令由 OpsHub 生成，参数必须白名单校验：

1. `server` 只能来自已登记 Barman server。
2. `backup_id` 只能来自已同步 catalog，或在恢复阶段使用 `auto`。
3. `destination_dir` 必须位于 Runner 配置的 work root 下。
4. `target_time` 必须是可解析时间，且必须在可恢复窗口内。
5. `target_lsn` 必须符合 PostgreSQL LSN 格式。
6. `target_tli` 必须来自计划选择或 catalog 已知 timeline。

#### P3 PostgreSQL WAL 解析和校验

需要新增 PostgreSQL WAL 工具包，建议包名：

```text
internal/biz/database/pgwal
```

职责：

1. 解析 WAL segment 文件名。
2. 解析 timeline ID。
3. 计算 segment 序号。
4. 校验 segment 文件名合法性。
5. 比较 segment 连续性。
6. 比较 LSN 大小。
7. 根据 WAL segment size 判断 LSN 是否落在某个 segment 范围内。
8. 识别 `.history` timeline history 文件。

恢复计划校验必须覆盖：

| 校验项 | 失败状态 |
| --- | --- |
| base backup 缺失 | `missing_base` |
| 原生增量依赖缺失 | `missing_incremental` |
| `pg_system_identifier` 不一致 | `system_identifier_mismatch` |
| timeline 不匹配 | `timeline_mismatch` |
| timeline history 缺失 | `timeline_gap` |
| WAL segment 不连续 | `missing_wal` |
| target LSN 不在 WAL 覆盖范围内 | `lsn_not_covered` |
| target time 不在 WAL 时间窗口内 | `time_not_covered` |
| storage object 缺失 | `missing_object` |
| checksum 不匹配 | `checksum_failed` |
| Barman/pg tools 不兼容 | `incompatible_tool` |

#### P3 恢复计划内容

`plan_json` 需要包含：

```json
{
  "engine": "postgresql",
  "backupEngine": "barman",
  "backupScope": "cluster",
  "barmanServerId": 1,
  "barmanServerName": "prod-pg",
  "target": {
    "type": "time",
    "value": "2026-04-29 02:30:00",
    "inclusive": true,
    "timelineId": "1"
  },
  "baseBackup": {
    "recordId": 100,
    "externalBackupId": "20260429T010001",
    "pgSystemIdentifier": "7400000000000000000",
    "timelineId": "1",
    "startLsn": "0/3000028",
    "endLsn": "0/5000000",
    "walStart": "000000010000000000000003",
    "walEnd": "000000010000000000000005"
  },
  "walChain": {
    "status": "complete",
    "archiveIds": [201, 202, 203],
    "timelineHistoryRequired": false,
    "coverage": {
      "fromLsn": "0/5000000",
      "toLsn": "0/9000000"
    }
  },
  "checks": {
    "systemIdentifier": "passed",
    "timeline": "passed",
    "walContinuity": "passed",
    "storage": "passed",
    "tool": "passed"
  },
  "restoreSteps": [
    "barman restore",
    "start isolated postgres",
    "wait recovery target",
    "run validation sql",
    "generate proof"
  ]
}
```

如使用 `pg_basebackup --incremental`，`restoreSteps` 必须包含：

```text
pg_combinebackup
```

且 `plan_json` 必须列出 full + incremental 依赖链。

#### P3 前端改造

PostgreSQL 物理备份任务页面：

1. 明确显示“PostgreSQL 物理备份是 cluster 级，不能选择单库/Schema/表”。
2. 当选择 PostgreSQL + physical 时，只允许 `backup_scope=cluster`。
3. `backup_engine` 首选 Barman。
4. `pg_basebackup` 显示为轻量备用。
5. WAL-G 显示为 external registration。
6. pgBackRest 显示 legacy external，并提示“不作为新方案默认推荐”。

Barman server 页面：

1. Barman server 列表。
2. server 健康状态。
3. 最近 `barman check` 结果。
4. retention policy。
5. streaming archiver / archive_command 状态。
6. 最近 catalog sync 时间。
7. 最近 WAL sync 时间。
8. 最近错误。

Barman catalog 页面：

1. backup ID。
2. backup 状态。
3. backup 开始/结束时间。
4. PostgreSQL 版本。
5. system identifier。
6. timeline。
7. start/end LSN。
8. WAL start/end。
9. backup size。
10. retention 状态。
11. 是否已同步到 OpsHub backup record。

WAL 状态页面：

1. WAL archive stream。
2. timeline。
3. segment 起止。
4. LSN 起止。
5. 最近归档时间。
6. gap 状态。
7. RPO 风险提示。

恢复计划页面：

1. PostgreSQL 支持 `target time` 和 `target LSN`。
2. 展示 cluster 级恢复提示。
3. 展示 base backup。
4. 展示 WAL chain。
5. 展示 timeline/system identifier 校验。
6. timeline mismatch / WAL gap 时明确失败原因。
7. 恢复执行前要求选择 Runner 主机、隔离端口、容器镜像或本机 PostgreSQL 路径。

#### P3 测试矩阵

单元测试：

1. WAL segment 文件名解析。
2. timeline ID 解析。
3. LSN 比较。
4. LSN 到 WAL segment 覆盖判断。
5. WAL segment 连续性判断。
6. timeline mismatch 阻断恢复计划。
7. `pg_system_identifier` mismatch 阻断恢复计划。
8. 缺 WAL segment 阻断恢复计划。
9. target LSN 超出覆盖范围阻断恢复计划。
10. 原生增量依赖缺失阻断恢复计划。
11. `pg_combinebackup` 步骤生成。
12. Barman `list-backups` / `show-backup` 输出解析。

集成测试：

1. PostgreSQL + Barman server 登记。
2. `barman check` 成功和失败。
3. 触发 Barman backup。
4. 同步 Barman backup catalog。
5. 同步 WAL catalog。
6. 手工制造 WAL gap 后恢复计划失败。
7. 手工制造 timeline mismatch 后恢复计划失败。
8. 按 target time 恢复到隔离 PostgreSQL。
9. 按 target LSN 恢复到隔离 PostgreSQL。
10. 校验 SQL 通过。
11. 校验 SQL 断言失败时恢复证明记录 actual/expected。

真实环境演练：

1. 准备一个 PostgreSQL 主库。
2. 准备一个 Barman server。
3. 生成测试表和测试数据。
4. 做 Barman full backup。
5. 插入 backup 之后的数据。
6. 确认 WAL 被归档。
7. 选择插入后时间点恢复。
8. 选择指定 LSN 恢复。
9. 启动隔离 PostgreSQL。
10. 执行 SQL 断言。
11. 生成恢复证明。

#### P3 分期拆分

##### P3.0：P3 设计和兼容矩阵

目标：把 PostgreSQL 物理备份边界、工具矩阵、测试拓扑和风险提示固化。

实现内容：

1. 补充 P3 文档。
2. 补充 Barman / pg_basebackup / WAL-G / pgBackRest 选择说明。
3. 明确 PostgreSQL 物理备份 cluster 级 UI 文案。
4. 明确 Runner 工具探测字段：
   - `barman --version`
   - `pg_basebackup --version`
   - `pg_combinebackup --version`
   - `pg_ctl --version`
   - `postgres --version`
   - `psql --version`
5. 明确测试环境和验收脚本。

验收：

1. 文档能直接指导 P3.1-P3.10 实施。
2. 不修改线上行为。

##### P3.1：Barman server 登记和健康检查

目标：OpsHub 能登记 Barman server，并通过 Runner 执行 `barman check`。

当前落地状态（2026-04-29）：

1. 已落地 `database_barman_servers` 模型、仓库和 AutoMigrate。
2. 已新增 Barman server CRUD API 与前端管理页签。
3. 已新增 `barman_check` Runner job 类型和白名单命令。
4. 已通过 SSH Runner 执行受控脚本：
   - `barman --version`
   - `barman -f json check <server>`
   - `barman check <server>`
   - `barman -f json status <server>`
5. 已解析 `barman_version`、`retention_policy`、`backup_method`、`slot_name`、`pg_version`、`pg_system_identifier`、`wal_segment_size`、archiver/streaming 状态。
6. 已校验 Barman server name 只能使用安全字符，命令参数不接受前端自由拼接。

实现内容：

1. 新增 `database_barman_servers` 模型、仓库、迁移。
2. 新增 Barman server CRUD API。
3. 新增 `check` API。
4. Runner job 新增 `barman_check` 类型。
5. 后端生成受控脚本执行：
   - `barman --version`
   - `barman check <server>`
   - 可选 `barman status <server>`
6. 解析 check 输出，写入：
   - `status`
   - `last_check_at`
   - `last_check_status`
   - `last_error`
   - `barman_version`
   - `pg_version`
7. 前端增加 Barman server 管理页面。

验收：

1. 能创建 Barman server。
2. 能手动触发 check。
3. check 成功显示 healthy。
4. check 失败显示失败项和错误。
5. 无效 server name 不会进入命令行。

##### P3.2：Barman backup catalog 同步

目标：把 Barman catalog 同步成 OpsHub backup record。

当前落地状态（2026-04-29）：

1. 已新增 `barman_catalog_sync` Runner job 类型和白名单命令。
2. 已通过 SSH Runner 执行受控脚本：
   - `barman -f json list-backup <server>`，失败时兼容 `list-backups`
   - `barman list-backup <server>`，用于提取 backup ID，失败时兼容 `list-backups`
   - `barman -f json show-backup <server> <backup_id>`
3. 已解析 Barman backup ID、状态、开始/结束时间、大小、PostgreSQL system identifier、timeline、WAL segment、LSN。
4. 已按 `source_instance_id + backup_engine + external_server_name + external_backup_id` 幂等 upsert `database_backup_records`。
5. 已写入 `backup_method=physical`、`backup_engine=barman`、`backup_scope=cluster`、`storage_uri=barman://<server>/<backup_id>`。
6. 已在前端 Barman Server 页签提供“检查”和“同步”操作，并在 Runner 任务里展示 Barman 任务类型。

实现内容：

1. Runner 执行：
   - `barman list-backups <server>`
   - `barman show-backup <server> <backup_id>`
2. 解析 Barman backup ID、状态、开始/结束时间、大小、WAL 范围、timeline、LSN。
3. 写入或更新 `database_backup_records`：
   - `backup_method=physical`
   - `backup_engine=barman`
   - `backup_scope=cluster`
   - `external_backup_id=<barman backup id>`
   - `external_server_name=<barman server name>`
   - `pg_system_identifier`
   - `timeline_id`
   - `start_lsn`
   - `end_lsn`
   - `wal_start`
   - `wal_end`
4. 同步幂等：同一个 Barman server + backup ID 不重复创建。
5. 前端 catalog 页面显示“已同步/未同步”。

验收：

1. Barman 备份记录能同步到 OpsHub。
2. 重复同步不会产生重复记录。
3. 备份记录显示 cluster 级。
4. 缺关键元数据时标记 degraded，不当作 PITR 可用。

##### P3.3：Barman WAL 状态和 WAL catalog 同步

目标：OpsHub 能看到 PostgreSQL WAL 链路，并识别 WAL gap。

当前落地状态（2026-04-29）：

1. 已新增 `barman_wal_sync` Runner job 类型和白名单命令。
2. 已新增接口：
   - `POST /api/v1/databases/barman-servers/:id/sync-wal`
3. 已通过 SSH Runner 执行受控脚本：
   - `barman -f json list-backup <server>`，失败时兼容 `list-backups`
   - `barman list-backup <server>`，失败时兼容 `list-backups`
   - `barman -f json show-backup <server> <backup_id>`
   - `barman list-files <server> <backup_id>`，用于读取 Barman catalog 中实际可见的 WAL / timeline history 文件
4. 已自动创建或复用 `database_log_archive_streams`：
   - `engine=postgresql`
   - `archive_type=wal`
   - `archive_engine=barman`
   - `archive_mode=external`
   - `config_json` 记录 `barmanServerId`、`barmanServerName`、`walSegmentSize`
5. 已新增 PostgreSQL WAL segment 解析工具包：
   - `internal/biz/database/pgwal`
   - 支持 WAL segment 文件名解析、timeline 解析、segment 序号计算、segment range 展开、`.history` 文件识别。
6. WAL 同步优先使用 `barman list-files` 的真实文件列表；只有当 `list-files` 不可用或无 WAL 文件输出时，才从 `show-backup` 的 `begin_wal/end_wal` 范围做兜底展开。
7. 已写入或更新 `database_log_archives`：
   - `file_name`
   - `storage_uri=barman://<server>/wal/<segment>`
   - `archive_type=wal`
   - `pg_system_identifier`
   - `timeline_id`
   - `wal_segment_size`
   - `segment_no`
   - `external_server_name`
   - `timeline_history_uri`
8. 已对同步结果做第一版链路判断：
   - 同 timeline 内 segment 序号不连续时返回 `missing_wal`
   - timeline 大于 1 但缺少对应 `.history` 时返回 `timeline_gap`
   - 异常时把归档流标记为 `degraded`
9. 前端 Barman Server 页签已增加“同步WAL”操作；日志归档列表已展示 WAL timeline 和 segment 序号。

本阶段边界：

1. P3.3 只同步 Barman catalog 元数据，不直接复制 WAL 文件。
2. `list-files` 不可用时的 `begin_wal/end_wal` 兜底范围只能证明“备份元数据要求的 WAL 范围”，不能证明对象文件真实存在。
3. 精确 target time / target LSN 覆盖校验仍归入 P3.5。
4. Barman WAL streaming / receive-wal 进程治理仍放在后续阶段。

实现内容：

1. 自动创建或绑定 `database_log_archive_streams`：
   - `engine=postgresql`
   - `archive_type=wal`
   - `archive_engine=barman`
2. 同步 Barman WAL catalog 或 xlog metadata。
3. 解析 WAL segment 文件名和 timeline。
4. 写入 `database_log_archives`。
5. 记录 timeline history 文件。
6. 计算归档窗口和 gap 状态。
7. 前端显示 WAL 状态。

验收：

1. WAL segment 能登记到 OpsHub。
2. 连续 WAL 显示 complete。
3. 删除一个中间 segment 后恢复计划能识别 `missing_wal`。
4. timeline history 缺失时恢复计划能识别 `timeline_gap`。

##### P3.4：Barman backup 触发

目标：OpsHub 能触发 PostgreSQL cluster 级 Barman 物理备份。

当前落地状态（2026-04-29）：

1. 已新增 `barman_backup` Runner job 类型和白名单命令。
2. 已新增接口：
   - `POST /api/v1/databases/barman-servers/:id/backup`
3. 已通过 SSH Runner 执行受控脚本：
   - `barman backup <server>`
   - `barman list-backup/list-backups <server>` 获取最新 backup ID
   - `barman -f json show-backup <server> <backup_id>`
   - `barman check-backup <server> <backup_id>` 作为附加校验输出采集
4. 备份触发时会先创建 `database_backup_records` 排队记录，状态为 `queued/running`，完成后用 `show-backup` metadata 更新为 Barman catalog 记录。
5. 已写入：
   - `backup_method=physical`
   - `backup_engine=barman`
   - `backup_scope=cluster`
   - `external_backup_id=<barman backup id>`
   - `external_server_name=<barman server name>`
   - `storage_uri=barman://<server>/<backup_id>`
   - `pg_system_identifier`
   - `timeline_id`
   - `start_lsn/end_lsn`
   - `wal_start/wal_end`
6. Barman backup 成功后等价完成一次对应 backup record 的 catalog 同步；后续仍可手动执行完整 catalog sync 做 reconciliation。
7. 备份任务已支持 PostgreSQL + `physical` + `barman`：
   - 后端校验 PostgreSQL 物理备份必须 `backup_scope=cluster`
   - `scope_config` 必须提供 `{"barmanServerId": 1}`
   - 手动/调度触发都会下发 `barman_backup` Runner job
8. 前端备份任务弹窗已允许 PostgreSQL 选择物理备份，并固定引擎为 Barman、范围为 cluster；Barman Server 页签已增加“备份”操作。
9. 失败时会记录 Runner job、备份记录、任务状态和备份审计。

本阶段边界：

1. Barman backup 仍依赖 Runner 主机已正确安装并配置 Barman。
2. 不在 backend 容器里执行 Barman。
3. 不自动执行 PostgreSQL restore；恢复到隔离目录和隔离实例仍属于 P3.6/P3.7。
4. 不把 Barman backup 产物复制到 OpsHub 本地存储，OpsHub 记录 Barman catalog URI 和元数据。

实现内容：

1. 备份任务支持 PostgreSQL + physical + Barman。
2. API 校验 PostgreSQL 物理备份必须 `backup_scope=cluster`。
3. Runner job 新增 `barman_backup` 类型。
4. Runner 执行：
   - `barman backup <server>`
   - 可选 `barman check-backup <server> <backup_id>`
5. 备份完成后自动触发 catalog sync。
6. 前端禁用 database/schema/table 选择。

验收：

1. PostgreSQL 物理备份任务页面明确显示 cluster 级。
2. 能手动触发 Barman backup。
3. 完成后能同步出 backup record。
4. Barman backup 失败时记录 Runner job、错误和审计。

##### P3.5：PostgreSQL PITR 恢复计划校验

目标：恢复计划能针对 PostgreSQL 做真实预校验。

当前落地状态（2026-04-29）：

1. `CreateRestorePlan` 已按来源实例类型分支；PostgreSQL 走 Barman 专用预校验链路。
2. PostgreSQL 目标类型已支持：
   - `time`
   - `lsn`
3. 新增 PostgreSQL WAL/LSN 工具能力：
   - LSN 解析、格式化和比较。
   - 按 LSN 计算所属 WAL segment。
   - timeline ID 规范化。
   - WAL segment LSN 覆盖范围计算。
4. base backup 选择策略：
   - 仅选择 `backup_method=physical`、`backup_engine=barman`、`backup_level=full`、带 `external_backup_id` 的 Barman backup record。
   - `targetType=time` 选择目标时间前最近的可用 Barman full backup。
   - `targetType=lsn` 优先选择 `EndLSN <= target LSN` 的最近 Barman full backup。
5. 预校验已覆盖：
   - Barman server 是否能由 backup record 唯一定位。
   - `pg_system_identifier` 是否一致。
   - target timeline 与 base backup timeline 是否一致。
   - timeline 大于 1 时是否存在对应 `.history` 文件。
   - WAL segment 是否连续。
   - target LSN 所需 WAL segment 是否完整。
   - target time 所需 WAL 时间窗口是否覆盖。
   - WAL/backup checksum 或 missing 状态。
   - Barman runner host 和工具状态。
6. `plan_json` 已生成 PostgreSQL 专用结构，包含：
   - Barman server ID/name。
   - runner host。
   - target type/value/timeline。
   - base backup。
   - WAL archive IDs。
   - checks。
   - restore steps。
7. `required_tool_json` 已加入 `barman`。
8. `required_artifact_json` 已保留 PostgreSQL/Barman 元数据，包括 `externalBackupId`、`externalServerName`、`pgSystemIdentifier`、timeline、LSN、WAL segment 信息。
9. 前端恢复计划弹窗已对 PostgreSQL 开放 `按 LSN`，并支持填写目标 timeline。

当前边界：

1. `targetType=time` 的覆盖能力依赖已同步 WAL archive 的 `first_event_time / last_event_time`，Barman catalog 粒度不足时只能按现有元数据判断。
2. `targetType=lsn` 能精确校验 WAL segment 连续性，但不解析 WAL 内部 event，不判断事务级精确停止点。
3. `storage object` 仍以 OpsHub 已登记 metadata 为准，不在 backend 容器里读取 Barman 仓库文件。
4. timeline 历史文件要求基于已同步的 WAL catalog；如果 Barman 中存在 `.history` 但尚未同步，计划会失败，需先执行 WAL 同步。

实现内容：

1. `CreateRestorePlan` 支持 PostgreSQL `targetType=time|lsn`。
2. 选择合适 base backup。
3. 校验 system identifier。
4. 校验 timeline。
5. 校验 timeline history。
6. 校验 WAL segment 连续性。
7. 校验 target LSN/time 覆盖。
8. 校验 storage object 和 checksum。
9. 生成 PostgreSQL 专用 plan_json、required_tool_json、required_artifact_json。

验收：

1. WAL 缺口能被恢复计划识别。
2. timeline 不匹配时恢复计划失败。
3. system identifier 不匹配时恢复计划失败。
4. target LSN 超出范围时恢复计划失败。
5. 正常链路恢复计划为 validated。

##### P3.6：Barman restore 到隔离目录

目标：Runner 能执行 Barman restore，把 PGDATA 恢复到隔离目录。

当前落地状态（2026-04-29）：

1. Runner Job 已新增：
   - `job_type=barman_restore`
   - `allowed_command=barman_restore`
2. `RunRestorePlan` 已支持 PostgreSQL 分支：
   - 来源实例为 PostgreSQL 时不再走 MySQL/MariaDB 物理恢复容器逻辑。
   - 仅允许预校验通过的 Barman PITR 计划执行。
   - 执行前会重新做 PostgreSQL/Barman 恢复计划复检。
3. Barman restore 执行边界：
   - 必须在 Barman server 登记的 SSH Runner 主机上执行。
   - 不允许用户自由输入命令。
   - `server` 来自已登记 Barman server。
   - `backup_id` 来自已同步 backup record 的 `external_backup_id`。
   - `destination_dir` 固定为：
     `runnerWorkRoot/restore/job-<restore_job_id>/pgdata`
   - 脚本内再次校验 destination 必须匹配安全目录模式。
4. 已支持受控 target 参数：
   - `--target-time`
   - `--target-lsn`
   - `--target-tli`，OpsHub 内部保存规范化 8 位十六进制 timeline，执行 Barman CLI 时转换为数字 timeline。
   - `--target-action pause|shutdown|promote`
   - `--get-wal / --no-get-wal`
5. Runner 脚本执行后会校验恢复目录结构：
   - `PG_VERSION`
   - `global/`
   - `base/`
6. 恢复任务会记录：
   - `database_restore_jobs.work_dir`
   - `prepared_datadir`
   - `log_path`
   - `artifact_uri`
   - `step_json`
   - `proof_json`
7. 恢复成功后：
   - restore job 状态为 `success`。
   - restore plan 状态为 `restored`。
   - 备份记录 `restore_test_status` 标记为 `restored`。
8. 前端恢复执行弹窗已针对 PostgreSQL 切换为 Barman restore 模式：
   - 不显示 MySQL/MariaDB 容器镜像、端口和校验 SQL。
   - 显示 target timeline、target action、get-wal 开关。
   - 风险确认文案改为“恢复到 Runner 隔离目录”。

当前边界：

1. P3.6 不启动隔离 PostgreSQL 实例。
2. P3.6 不执行校验 SQL 和断言，SQL 校验放到 P3.7。
3. P3.6 不做生产切换、不回填数据、不自动清理成功恢复目录。
4. 需要 Runner 主机已经正确安装并配置 Barman，且能访问 Barman backup/WAL 仓库。
5. `barman restore` 的 stdout/stderr 只作为 Runner 结果和 proof 摘要保存，不把完整恢复目录内容纳管进 OpsHub。

实现内容：

1. Runner job 新增 `barman_restore` 类型。
2. 后端生成 `barman restore` 脚本。
3. 支持：
   - `--target-time`
   - `--target-lsn`
   - `--target-tli`
   - `--target-action pause|shutdown|promote`
   - `--get-wal` / `--no-get-wal`
4. destination_dir 固定在 Runner work root。
5. 校验恢复后的目录结构。
6. 记录恢复目录、日志、命令摘要到 proof。

验收：

1. 能恢复到隔离目录。
2. 目录不与生产 PGDATA 重叠。
3. target 参数受控。
4. Barman restore 失败时 proof 记录失败步骤。

##### P3.7：隔离 PostgreSQL 实例启动和验证

目标：能把恢复目录启动为隔离 PostgreSQL，并执行校验 SQL。

实现内容：

1. 根据源 PostgreSQL major version 选择容器镜像或本机 postgres。
2. 配置隔离端口和 listen address。
3. 修正 PGDATA 权限。
4. 启动隔离 PostgreSQL。
5. 等待 recovery 到目标点。
6. 查询恢复状态。
7. 执行 validation SQL。
8. 支持 P2.7 已有断言：
   - `expectedRows`
   - `expectedContains`
   - `expectedScalar`
9. 生成恢复证明。
10. 支持失败清理和手动保留隔离库。

验收：

1. 能恢复到指定 target time 的隔离 PostgreSQL 实例。
2. 能恢复到指定 target LSN 的隔离 PostgreSQL 实例。
3. 校验 SQL 成功时 plan/job 标记成功。
4. 校验断言失败时 proof 记录 expected/actual。
5. cleanup 开启时失败后清理隔离容器。

2026-04-29 已落地：

1. Barman restore Runner 增加 `postgresStartInstance` 开关：
   - `true`：恢复到隔离目录后启动 PostgreSQL 容器。
   - `false`：只恢复到 Runner 隔离目录。
2. Restore job 自动生成隔离容器元数据：
   - `container_name=opshub-pg-restore-<job_id>`
   - `container_image=postgres:<source major>`，无法识别时使用 `postgres:latest`
   - `listen_host=127.0.0.1`
   - `listen_port` 沿用隔离恢复端口自动分配逻辑
3. Runner 脚本受控执行：
   - `barman restore` 仍固定写入 Runner work root 下的 `restore/job-<id>/pgdata`
   - 启动容器前校验 `PG_VERSION/global/base`
   - 使用 `docker run -d -p 127.0.0.1:<port>:5432 -v <pgdata>:/var/lib/postgresql/data`
   - 失败清理支持删除隔离容器和恢复目录
4. 校验 SQL 支持 PostgreSQL：
   - 默认校验包含 `SELECT 1`、`SELECT version()` 和数据库列表。
   - 允许用户追加只读 SQL。
   - 复用 P2.7 断言：`expectedRows`、`expectedContains`、`expectedScalar`。
   - proof 记录断言状态、实际行数、实际标量和失败原因。
5. 前端恢复执行弹窗：
   - PostgreSQL 可选择是否启动隔离实例。
   - 可配置容器镜像、监听端口、target timeline、target action、`--get-wal`。
   - PostgreSQL 启动隔离实例时显示校验 SQL 和断言表单。
6. 单元测试覆盖：
   - Barman restore 脚本的 target 参数约束。
   - 隔离 PostgreSQL 容器启动命令。
   - PostgreSQL 校验断言输出解析。

##### P3.8：pg_basebackup 轻量备用

目标：在没有 Barman 的轻量环境下支持 PostgreSQL 原生 base backup。

实现内容：

1. 备份任务支持 PostgreSQL + physical + `pg_basebackup`。
2. 明确 cluster 级。
3. 支持 full base backup。
4. 记录 backup manifest。
5. 支持外部 WAL archive 配合 PITR。
6. 原生 incremental 只在 PostgreSQL 17+ 且客户端工具支持时开放。
7. 使用 incremental 时恢复计划必须包含 `pg_combinebackup`。
8. 任一依赖备份缺失时恢复计划失败。

验收：

1. full pg_basebackup 可登记为 physical backup。
2. incremental 未满足版本条件时不能选择。
3. incremental 恢复计划包含 `pg_combinebackup`。
4. 缺依赖备份时恢复计划失败。

2026-04-29 已落地：

1. 备份任务支持 PostgreSQL `physical + pg_basebackup`：
   - 后端校验 `backup_scope=cluster`。
   - 第一版可执行任务仅支持 `backup_level=full`。
   - `scope_config` 必须提供 SSH Runner：`{"runnerHostId":1}`。
   - `extraArgs` 可透传给 `pg_basebackup`，但禁止包含 password/secret。
2. 手动触发和定时触发均走 Runner Job：
   - `job_type=pg_basebackup`
   - `allowed_command=pg_basebackup`
   - request/result 不保存数据库密码，只在 SSH 脚本执行环境中传入。
3. Runner 脚本能力：
   - 执行 `pg_basebackup -Fp -X stream --checkpoint=fast --progress`
   - 产物打包为 `tar.gz`
   - 记录 artifact 路径、大小、SHA256、工具版本
   - 尝试读取 `PG_VERSION`、`pg_controldata`、`backup_manifest` 摘要
   - storage URI 使用 `runner://runner-host-<id>/<path>`，后续恢复 Runner 可校验归属
4. 备份记录写入：
   - `backup_engine=pg_basebackup`
   - `backup_method=physical`
   - `backup_level=full`
   - `backup_scope=cluster`
   - `tool_name=pg_basebackup`
   - `pg_system_identifier / timeline_id / end_lsn / wal_end / backup_manifest_checksum`
5. PITR 恢复计划识别：
   - PostgreSQL base backup 选择支持 `barman` 和 `pg_basebackup`。
   - `pg_basebackup` 不再强依赖 Barman Server catalog。
   - WAL catalog 仍按 system identifier、timeline、时间或 LSN 覆盖检查。
   - 如果链路包含 `pg_basebackup` 增量记录，计划 JSON 必须包含 `pg_combinebackup` 步骤和 required tool。
6. 前端入口：
   - PostgreSQL 物理备份引擎可选 `Barman` 或 `pg_basebackup`。
   - `pg_basebackup` 任务级别限制为 full。
   - 范围配置提示展示 `runnerHostId` 示例。
7. 单元测试覆盖：
   - `pg_basebackup` SSH 脚本命令和重定向拼接。
   - artifact metadata 输出。
   - 含增量记录的 PostgreSQL 恢复计划会包含 `pg_combinebackup`。

P3.8.0 阶段未做边界：

1. 第一版不自动执行 `pg_basebackup` 产物的隔离恢复；P3.8.1/P3.8.2 负责继续补齐。
2. PostgreSQL 原生 incremental 不在 UI 中开放执行；可通过外部记录登记纳管，恢复计划负责链路校验和 `pg_combinebackup` 步骤证明。
3. WAL-G 仍在 P3.9 按 external metadata registration 处理。

##### P3.8+：pg_basebackup 自动隔离恢复深化拆分

定位：

`pg_basebackup` 自动隔离恢复继续归入 P3.8 深化项，不放入 P3.9 或 P3.10。

原因：

1. P3.8 的主题就是 PostgreSQL 原生 `pg_basebackup` 轻量备用链路。
2. P3.9 只处理 WAL-G / pgBackRest external 纳管，不应该混入原生恢复 Runner。
3. P3.10 负责 P3 总体联调、回归和恢复演练，应该验证 P3.8 已实现能力，而不是承载主要实现。

整体目标：

把 P3.8 从“能生成 pg_basebackup 备份记录和 PITR 计划输入”深化为“能用 pg_basebackup artifact + WAL catalog 自动恢复到隔离 PostgreSQL，并生成可验证 proof”。

通用前置条件：

1. Runner 主机可通过 SSH 执行受控脚本。
2. Runner 主机安装：
   - `docker`
   - `psql`
   - `postgres` 客户端工具
   - `tar`
   - `sha256sum`
   - 如涉及增量：`pg_combinebackup`
3. Runner 主机能读取备份 artifact：
   - `runner://runner-host-<id>/<path>`
   - 或后续对象存储下载到 Runner 本地 staging 目录
4. `database_backup_records` 中 base backup 必须具备：
   - `backup_engine=pg_basebackup`
   - `backup_method=physical`
   - `backup_scope=cluster`
   - `backup_level=full`
   - `storage_uri` 或 `file_path`
   - `checksum_sha256`
   - `pg_system_identifier`
   - `timeline_id`
   - `end_lsn` 或 `wal_end`
5. PITR 目标点超出 base backup 自身范围时，必须存在连续 WAL catalog：
   - `archive_type=wal`
   - `status in ('archived','verified')`
   - `pg_system_identifier` 一致
   - `timeline_id` 一致或具备 timeline history
   - 时间范围或 LSN 范围覆盖目标点
6. 恢复全程只允许写入 Runner work root：
   - 不允许 destination 指向生产 PGDATA
   - 不允许覆盖现有运行容器
   - 不允许默认删除原始 backup/WAL artifact

###### P3.8.1：pg_basebackup 隔离目录恢复

目标：

先把 `pg_basebackup` artifact 恢复到 Runner 隔离目录，并证明目录结构和备份元数据可用；不启动 PostgreSQL，不 replay WAL。

实现内容：

1. 新增 `pg_basebackup_restore` Runner 执行分支，或复用 restore job 的 PostgreSQL native strategy。
2. 根据 restore plan 找到 selected base backup。
3. 校验 base backup：
   - storage URI 属于当前 Runner 或可下载到当前 Runner。
   - 文件存在。
   - 文件大小匹配。
   - SHA256 匹配。
   - `backup_engine=pg_basebackup`。
4. 在 Runner work root 创建隔离目录：

   ```text
   <runner_work_root>/restore/job-<restore_job_id>/
     artifacts/
       base.tar.gz
     pgdata/
     proof.json
     restore.log
   ```

5. 解压 base artifact 到 `pgdata/`。
6. 校验目录结构：
   - `PG_VERSION`
   - `global/`
   - `base/`
   - `pg_wal/` 或 `pg_xlog/`
   - `backup_manifest` 如存在则记录 checksum
7. 如可用，执行 `pg_verifybackup`：
   - 工具不存在时只记录 warning，不阻断第一版恢复。
   - 工具存在但校验失败时标记 restore job failed。
8. 生成 proof：
   - base record ID
   - artifact URI
   - checksum 校验结果
   - PG_VERSION
   - backup manifest checksum
   - pgdata path
   - restore step 列表

不做：

1. 不启动 PostgreSQL。
2. 不写 `recovery.signal`。
3. 不应用 WAL。
4. 不执行 validation SQL。

验收：

1. `pg_basebackup` full artifact 能恢复到隔离目录。
2. checksum 错误时任务失败，不生成成功 proof。
3. 目录缺少 `PG_VERSION/global/base` 时任务失败。
4. proof 能证明 base artifact、pgdata path 和校验结果。
5. cleanup 开启时失败后删除本次 `pgdata/`，不删除原始 artifact。

###### P3.8.2：pg_basebackup + WAL PITR 隔离实例

目标：

在 P3.8.1 的基础上，把 base backup 配合 WAL catalog 恢复到指定 target time / target LSN / timeline，并启动隔离 PostgreSQL 容器执行校验 SQL。

实现内容：

1. 读取 restore plan：
   - base backup
   - selected WAL archives
   - target type：`time / lsn`
   - target value
   - target timeline
2. 校验 WAL 链：
   - system identifier 一致
   - timeline 一致
   - timeline > 1 时必须有 `.history`
   - segment 连续
   - 时间或 LSN 覆盖目标点
   - checksum 可用
3. 将 WAL artifact staged 到 Runner：

   ```text
   <runner_work_root>/restore/job-<id>/wal/
     000000010000000000000001
     000000010000000000000002
     00000002.history
   ```

4. 生成隔离恢复配置：
   - 写入 `recovery.signal`
   - 写入 `postgresql.auto.conf` 或 append 受控配置
   - `restore_command='cp <wal_dir>/%f %p'`
   - target time：`recovery_target_time`
   - target LSN：`recovery_target_lsn`
   - target timeline：`recovery_target_timeline`
   - target action：默认 `pause`
5. 启动隔离 PostgreSQL 容器：
   - 容器名：`opshub-pgbase-restore-<job_id>`
   - 绑定：`127.0.0.1:<listen_port>:5432`
   - 挂载：`pgdata:/var/lib/postgresql/data`
   - 镜像：默认 `postgres:<source major>`，不能识别时 `postgres:latest`
6. 等待恢复：
   - `SELECT pg_is_in_recovery()`
   - `SELECT pg_last_wal_replay_lsn()`
   - `SELECT pg_last_xact_replay_timestamp()`
   - 对 target LSN 校验 replay LSN 是否达到目标
   - 对 target time 校验 replay timestamp 是否不晚于目标，且容器可查询
7. 执行 validation SQL：
   - 默认 `SELECT 1`
   - 默认 `SELECT version()`
   - 用户自定义只读 SQL
   - `expectedRows`
   - `expectedContains`
   - `expectedScalar`
8. 生成 proof：
   - base backup proof
   - WAL chain proof
   - recovery config 摘要
   - replay LSN / replay timestamp
   - validation SQL result
   - assertion expected/actual
   - container name、image、listen port

安全边界：

1. `restore_command` 只能指向本次 job 的 WAL staging 目录。
2. 不允许 `restore_command` 由用户自由输入。
3. 不允许容器监听 `0.0.0.0`，默认只绑定 `127.0.0.1`。
4. 不允许将生产连接串写入 proof。
5. validation SQL 必须走只读白名单。

验收：

1. 可恢复到指定 target time 的隔离 PostgreSQL。
2. 可恢复到指定 target LSN 的隔离 PostgreSQL。
3. WAL 缺段时恢复计划或执行前校验失败。
4. timeline 不匹配时拒绝执行。
5. 校验 SQL 成功时 restore plan/job 标记 verified。
6. 断言失败时 restore job 保留为 restored 或 failed，并在 proof 记录 expected/actual。
7. cleanup 开启时失败后删除隔离容器和本次 pgdata。

2026-04-29 P3.8.1/P3.8.2 已落地：

1. 后端恢复计划执行支持按 PostgreSQL 备份引擎分派：
   - `backup_engine=barman` 继续走 P3.7 的 Barman restore。
   - `backup_engine=pg_basebackup` 走新的 `pg_basebackup_restore` Runner Job。
   - `job_type=pg_basebackup_restore`
   - `allowed_command=pg_basebackup_restore`
2. P3.8.1 隔离目录恢复：
   - 从 `restore plan` 读取 selected base backup。
   - 强制校验 base record 为 `physical + full + pg_basebackup`。
   - 强制 artifact 属于所选 `runner://runner-host-<id>`，避免跨 Runner 读错文件。
   - Runner 脚本复制 artifact 到本次 job `artifacts/`。
   - 校验文件大小和 SHA256。
   - 解压到 `<work_root>/restore/job-<id>/pgdata/`。
   - 校验 `PG_VERSION / global / base / pg_wal|pg_xlog`。
   - 如存在 `backup_manifest` 且 Runner 有 `pg_verifybackup`，执行校验；工具不存在时记录 `not_available`。
   - 支持失败后清理本次 `pgdata/`，不删除原始 artifact。
3. P3.8.2 PITR 隔离实例：
   - 执行前重新调用 PostgreSQL PITR 校验，复检 base、WAL catalog、timeline、system identifier 和覆盖范围。
   - 根据 `selected_log_archive_ids` staged WAL 到本次 job `wal/`。
   - `restore_command` 由系统生成，只指向本次 job WAL 目录，不接受用户自由输入。
   - 支持 `recovery_target_time` 和 `recovery_target_lsn`。
   - 支持 `recovery_target_timeline`，内部把 8 位 timeline 转为 PostgreSQL 配置可用的十进制值。
   - 默认 `recovery_target_action=pause`，也保留 `shutdown/promote` 入口。
   - 启动隔离 PostgreSQL 容器：`opshub-pgbase-restore-<job_id>`，仅绑定 `127.0.0.1:<port>:5432`。
   - 对 target LSN 执行 `pg_wal_lsn_diff(pg_last_wal_replay_lsn(), target) >= 0` 到达性检查。
4. 校验和 proof：
   - 复用 P2.7/P3.7 的 validation SQL 断言模型。
   - 支持 `expectedRows / expectedContains / expectedScalar`。
   - proof 记录 base record、WAL archive IDs、pgdata 路径、容器信息、PG_VERSION、backup manifest checksum、`pg_verifybackup` 状态、recovery summary、步骤、校验结果和 expected/actual。
   - 校验通过时计划和任务标记 `verified`。
   - 不启动实例时标记 `restored`，validation 状态记录为 `warning`。
5. 前端入口：
   - 执行隔离恢复弹窗自动识别 `backupEngine=pg_basebackup`。
   - `pg_basebackup` 恢复不再展示 Barman 专用 `--get-wal` 开关，显示 WAL 来源为恢复计划选择。
   - Runner 任务筛选新增 `pg_basebackup 恢复`。
6. 测试覆盖：
   - `pg_basebackup_restore` 目录恢复脚本生成。
   - `pg_basebackup + WAL` PITR 容器脚本生成。
   - Runner 输出解析，包括 proof 元数据、步骤和断言校验结果。

当前边界：

1. P3.8.1/P3.8.2 已支持 OpsHub `pg_basebackup` full artifact 的自动恢复；原生 incremental 由 P3.8.3 继续补齐。
2. artifact 读取首版依赖 Runner 本地可读 `runner://` 路径；对象存储直接下载到 Runner staging 可在后续存储增强中补齐。
3. 自动隔离恢复不会修改生产实例、不会切换业务连接、不会自动回填数据。
4. `target_action=shutdown/promote` 会改变容器后续校验行为；长期默认建议仍使用 `pause` 做恢复证明。

###### P3.8.3：pg_basebackup incremental / pg_combinebackup

目标：

支持 PostgreSQL 原生增量备份链路的恢复准备。增量不能直接启动，需要先使用 `pg_combinebackup` 合成可用 full backup，再进入 P3.8.2。

版本门控：

1. PostgreSQL server 版本必须支持原生 incremental base backup。
2. Runner 上 `pg_basebackup` 和 `pg_combinebackup` 版本必须兼容。
3. backup manifest 必须存在并可用于校验依赖关系。
4. 不满足条件时：
   - UI 不展示原生增量执行入口。
   - 外部登记的增量记录仍可纳管，但恢复计划必须标记 tool/version 风险。

实现内容：

1. 备份记录层面区分：
   - `backup_level=full`
   - `backup_level=incremental`
   - `base_record_id`
   - `parent_record_id`
2. 恢复计划校验增量链：
   - full base 存在
   - 每个 incremental 的 parent 存在
   - 依赖顺序完整
   - 任一缺失则 `backup_chain_status=missing_incremental`
3. Runner 执行：

   ```text
   pg_combinebackup <full_dir> <inc1_dir> <inc2_dir> ... -o <synthetic_full_dir>
   ```

4. 合成 full 后校验：
   - `PG_VERSION`
   - `global/`
   - `base/`
   - manifest checksum
5. synthetic full 作为 P3.8.2 的输入继续执行 WAL PITR。
6. proof 记录：
   - full backup ID
   - incremental backup IDs
   - combine order
   - pg_combinebackup version
   - synthetic full path
   - synthetic full checksum/manifest 摘要

不做：

1. 不把 synthetic full 自动登记为新的长期 backup record，第一版仅作为 restore job 中间产物。
2. 不跨实例混合增量链。
3. 不在链路不完整时尝试“跳过某个 incremental”。

验收：

1. 含完整增量链的计划包含 `pg_combinebackup` 步骤。
2. 缺 parent incremental 时恢复计划失败。
3. Runner 缺少 `pg_combinebackup` 时 tool status 失败。
4. synthetic full 生成失败时不启动 PostgreSQL。
5. synthetic full 成功后可进入 P3.8.2 执行 PITR。

###### P3.8.4：pg_basebackup 恢复演练模板

目标：

把 pg_basebackup 的恢复执行固化为可重复演练模板，方便后续 P3.10 做全链路演练。

实现内容：

1. 前端增加“从 pg_basebackup 生成恢复演练”入口。
2. 用户选择：
   - base backup record
   - target time / target LSN
   - target timeline
   - Runner host
   - 是否启动隔离实例
   - 校验 SQL/断言模板
3. 后端生成 restore plan：
   - 自动选择 base backup
   - 自动选择 WAL archive
   - 自动判断是否需要 `pg_combinebackup`
   - 自动估算 restore bytes / minutes
4. 执行完成后生成演练记录：
   - restore job
   - proof JSON
   - backup record `restore_tested_at`
   - backup record `restore_test_status`
5. 前端展示：
   - 最近恢复演练时间
   - 演练结果
   - 可恢复窗口
   - proof 下载/查看

验收：

1. 能从任一 pg_basebackup full record 直接发起演练。
2. 演练 proof 能展示 base、WAL、timeline、target、validation、container。
3. 演练失败能明确区分：
   - backup artifact 不可读
   - WAL 缺口
   - timeline mismatch
   - 工具缺失
   - 容器启动失败
   - 校验 SQL/断言失败
4. 演练成功会更新 restore test 状态。

2026-04-30 P3.8.3/P3.8.4 已落地：

1. P3.8.3 增量链路校验：
   - PostgreSQL `pg_basebackup` 恢复计划支持固定 base record。
   - 计划生成会把 base 后续的同链路 incremental 纳入 `selected_backup_record_ids`。
   - `pg_basebackup` incremental 链要求 `parent_record_id` 连续，且每个 incremental 记录必须具备 `backup_manifest_checksum` 或 `manifest_json`。
   - 缺 parent、parent 不在已选链路、manifest 缺失时，计划会标记备份链不可用并给出明确原因。
2. Runner 合成 synthetic full：
   - `pg_basebackup_restore` Runner Job 会把 base 和 incrementals 恢复到隔离工作目录。
   - 存在 incrementals 时先运行 `pg_combinebackup` 合成 synthetic full，再进入原 P3.8.2 WAL PITR 和隔离实例启动流程。
   - Runner 缺少 `pg_combinebackup` 会返回 `tool_status=missing_tool`，不会继续启动 PostgreSQL。
   - proof JSON 增加 `selectedBackupRecordIds`、`incrementalCount`、`combineBackupStatus`、`combineBackupVersion`、`syntheticFullPath`。
3. P3.8.4 前端演练入口：
   - 备份记录列表中 `physical + full + pg_basebackup + success` 记录显示为 `PITR演练`。
   - 从该入口进入会打开 PITR 恢复计划弹窗并固定 `baseRecordId`。
   - 后端仍会重新校验 base 归属、状态、目标时间、WAL、timeline 和增量链，不信任前端选择。
   - 生成计划后可沿用 P3.8.2 的恢复执行弹窗选择 Runner、隔离实例参数和校验 SQL/断言。
4. 记录回写：
   - `pg_basebackup_restore` 执行完成后会更新 base backup record 的 `restore_tested_at`。
   - 成功、失败都会写入 `restore_test_status`，避免演练失败被误看成“未演练”。

当前边界：

1. PostgreSQL 原生 incremental 备份执行入口仍不在 UI 开放；可先通过外部记录登记纳管 incremental artifact。
2. synthetic full 第一版只作为恢复 Job 中间产物，不自动登记为新的长期 backup record。
3. 对象存储 artifact 自动拉取到 Runner staging 仍留到后续存储增强；P3.8.3/P3.8.4 仍依赖 Runner 可读 artifact。
4. 真实 PostgreSQL 环境的 full + incremental + WAL 长链路端到端演练归入 P3.10 总体验收。

###### P3.8.9：pg_basebackup 恢复执行就绪性门禁

目标：

把 P3.8 已能生成和执行的 `pg_basebackup` 恢复链路补上执行前可见的 readiness gate，避免“恢复计划预校验通过，但下发 Runner 后才发现 artifact 不可读或工具清单不完整”。

实现范围：

1. 恢复计划 required tools 补齐 `pg_basebackup` 恢复真实依赖：
   - `pg_basebackup`
   - `tar`
   - `sha256sum`
   - `docker`
   - `psql`
   - `pg_verifybackup`，可选
   - 如有 incremental，增加 `pg_combinebackup`
2. Runner 探测补充 PostgreSQL 恢复工具：
   - `pg_basebackup`
   - `pg_combinebackup`
   - `pg_verifybackup`
   - `psql`
   - `docker`
   - `tar`
   - `sha256sum`
3. `required_artifact_json` 增加 artifact 可读性矩阵：
   - `runnerHostId`
   - `runnerReadable`
   - `runnerPath`
   - `readinessStatus`
   - `readinessMessage`
4. readiness 状态定义：
   - `ready`：当前 Runner 可直接读取 `runner://`、`local://`、`file://` 或绝对路径 artifact。
   - `pending_download`：S3/MinIO/OSS/COS 等对象存储 URI 已登记，但自动拉取到 Runner staging 尚未启用。
   - `metadata_only`：`metadata://` 仅能做计划证明，不能作为实际恢复输入。
   - `missing`：未登记可用路径。
   - `unreadable`：路径属于其它 Runner 或格式不可被当前 Runner 读取。
   - `unknown`：计划尚未绑定 Runner，执行时再确认。
   - `managed_by_barman`：Barman artifact 由 Barman server/catalog 管理，不要求 Runner 直接读单个备份文件。
5. `pg_basebackup_restore` 下发前同步预检：
   - base artifact 必须 Runner 可读。
   - incremental artifact 必须 Runner 可读。
   - 启动隔离实例时，WAL artifact 也必须 Runner 可读。
   - 任一不可读则拒绝下发恢复任务，而不是让后台 Job 排队后失败。
6. 前端恢复计划列表展示 Artifact readiness：
   - 全部 ready 显示“就绪”。
   - 对象存储未拉取显示“待拉取”，阻止执行。
   - missing/unreadable/metadata_only 显示“不可读”，阻止执行。
   - unknown 不阻止旧计划，但提示执行时确认 Runner。

不做：

1. 不在本阶段实现对象存储自动下载到 Runner staging。
2. 不把对象存储密钥下发给 backend 或写入恢复计划。
3. 不新增生产库切换、回填或清理原始 artifact 的动作。
4. 不把 Runner 探测结果做成强约束缓存；实际恢复脚本仍以执行时 `command -v` 和文件校验为准。

验收：

1. 新生成的 pg_basebackup 恢复计划能在 `required_artifact_json` 中看到每个 base/incremental/WAL artifact 的 readiness。
2. `runner://runner-host-N/...` 且 Runner 匹配时显示 ready。
3. `s3://` / `minio://` artifact 显示 pending_download，并阻止直接执行。
4. Runner 探测输出包含 pg_basebackup 恢复所需工具。
5. 执行 pg_basebackup + WAL 隔离恢复前，如果 WAL artifact 不是 Runner 可读路径，会直接拒绝下发并返回清晰错误。

2026-04-30 P3.8.9 已落地：

1. 后端 required tools 已补齐 `pg_basebackup` 恢复执行真实依赖。
2. 后端 required artifacts 已生成 Runner readiness 矩阵。
3. `pg_basebackup_restore` 下发前会同步预检 WAL artifact 可读性。
4. Runner 探测命令已补充 PostgreSQL 恢复工具。
5. 前端恢复计划列表已展示 Artifact readiness，并对不可读或待拉取对象阻止直接执行。

建议执行顺序：

1. P3.8.1：先恢复目录，验证 artifact 可用。
2. P3.8.2：再接 WAL PITR 和隔离 PostgreSQL。
3. P3.8.3：最后接原生增量和 `pg_combinebackup`。
4. P3.8.4：把执行流程产品化为演练模板。

与 P3.10 的关系：

P3.10 不再重新设计 pg_basebackup 恢复逻辑，只负责用真实 PostgreSQL 测试环境验证：

1. Barman restore 到 target time。
2. Barman restore 到 target LSN。
3. pg_basebackup full + WAL restore 到 target time。
4. pg_basebackup full + WAL restore 到 target LSN。
5. pg_basebackup incremental + pg_combinebackup + WAL restore。
6. WAL gap / timeline mismatch / checksum failed 的失败证明。

##### P3.9：WAL-G / pgBackRest external 纳管

目标：支持已有 PostgreSQL 外部备份链路纳管，但不深接执行。

实现内容：

1. 外部备份登记支持 `backup_engine=walg`。
2. 外部 WAL 登记支持 `archive_engine=walg`。
3. 外部备份登记支持 `backup_engine=pgbackrest`。
4. pgBackRest UI 标注 legacy external。
5. 新建策略默认不推荐 pgBackRest。
6. 只记录元数据、恢复演练结果和风险提示。

验收：

1. WAL-G 外部记录能进入恢复计划。
2. pgBackRest 记录可登记但有 legacy warning。
3. 不出现“新建 pgBackRest 深接入策略”的默认入口。

落地记录（2026-04-30）：

1. 后端已规范 PostgreSQL external 引擎别名：
   - `wal-g / wal_g / walg` 统一保存为 `backup_engine=walg` 或 `archive_engine=walg`。
   - `pgBackRest / pg-backrest / pg_backrest` 统一保存为 `backup_engine=pgbackrest` 或 `archive_engine=pgbackrest`。
2. 外部备份登记在 PostgreSQL 场景下会按 cluster 级元数据处理；WAL-G / pgBackRest full 记录可作为 PostgreSQL PITR base backup 进入恢复计划。
3. WAL-G / pgBackRest 计划只做元数据链路校验：
   - 校验 base backup、增量链、WAL 时间/LSN 覆盖、timeline、system_identifier、checksum 状态。
   - 计划状态为 `warning`，表示链路可用于外部恢复演练证明，但 OpsHub 当前不自动执行 `wal-g backup-fetch`、WAL replay 或 `pgbackrest restore`。
   - `required_artifact_json` 对 WAL-G 标记为 `managed_by_walg`，对 pgBackRest 标记为 `legacy_pgbackrest`。
4. `run restore plan` 仍只支持：
   - `backup_engine=barman` 走 Barman restore Runner。
   - `backup_engine=pg_basebackup` 走 pg_basebackup artifact restore Runner。
   - `backup_engine=walg/pgbackrest` 会被拒绝执行，并提示当前仅支持 external 元数据纳管。
5. 前端外部备份登记已提供 WAL-G external 和 pgBackRest legacy external 选项，并补充外部备份 ID、外部 Server/Profile/Stanza、工具名字段。
6. 前端新建 PostgreSQL 物理备份任务仍只推荐 Barman 和 pg_basebackup；pgBackRest 不出现在新建深接入策略入口。

##### P3.A1/P3.A2：PITR 可观测性补齐已落地

落地记录（2026-05-02）：

1. 恢复任务详情已补充步骤时间线：
   - 前端恢复任务列表新增“详情”入口。
   - 详情弹窗读取 `database_restore_jobs.step_json`，按 `name/status/occurredAt` 展示恢复步骤、开始/完成时间和步骤耗时。
   - 同一个步骤的 `running` 与后续终态事件会合并展示，方便定位恢复卡在哪一步。
   - 未识别的步骤名称保留原始 `name`，避免后端新增步骤后前端丢失信息。
2. 恢复任务详情已补充校验 SQL 和断言结果：
   - 前端读取 `validation_json`，展示 SQL、执行状态、期望值、实际值、断言状态和输出预览。
   - 支持展示 `expectedRows`、`expectedContains`、`expectedScalar` 对应的实际结果和失败原因。
   - Proof 仍作为完整证据保留，详情弹窗只做排障和快速核验视图。
3. PITR 子页签已新增 Barman Catalog 视图：
   - 复用 `/api/v1/databases/backup-records`，按 PostgreSQL 物理备份、cluster 级范围和备份引擎过滤。
   - 展示 backup ID、实例、引擎、状态、PostgreSQL system identifier、timeline、LSN 范围、WAL 范围、manifest checksum、可恢复窗口和同步状态。
   - 后端 `DatabaseBackupRecordVO` 已补充 MySQL binlog/GTID 字段和 PostgreSQL timeline/LSN/WAL/system identifier 字段，避免前端只能看通用备份字段。
   - 备份记录列表接口已支持 `backupMethod` 和 `backupScope` 查询参数，用于稳定过滤 PostgreSQL cluster 级物理备份 catalog。
4. PITR 子页签已新增 WAL 状态视图：
   - 复用 `/api/v1/databases/log-archives`，固定按 `archiveType=wal` 查询。
   - 前端归档流下拉只展示 WAL stream，避免混入 MySQL/MariaDB binlog stream。
   - WAL 记录按 stream 分组，展示 timeline、segment 序号、LSN 范围、事件时间范围、文件大小和状态。
   - 同一 timeline 内 segment 序号不连续会标记“缺口”；timeline 切换会单独标记，便于恢复计划失败前先从 UI 发现链路风险。
5. 当前边界：
   - A1/A2 是可观测性和 catalog 展示增强，不改变 PITR 恢复计划的链路校验算法。
   - WAL 状态页只使用已登记的 finalized WAL/archive metadata，不把 rollup 事件当作 PITR 输入。
   - WAL-G / pgBackRest 仍只做 external metadata 纳管，自动隔离恢复执行仍不开放。
   - 真实 PostgreSQL+Barman / pg_basebackup 端到端演练脚本和 runbook 仍归入 P3.10。

##### P3.10：P3 联调、回归和恢复演练

目标：把 P3 做成可证明恢复的闭环。

实现内容：

1. 准备 PostgreSQL + Barman 测试环境。
2. 准备真实备份、WAL、恢复脚本。
3. 编写单元测试。
4. 编写集成测试。
5. 执行 target time 恢复演练。
6. 执行 target LSN 恢复演练。
7. 执行 WAL gap 失败演练。
8. 执行 timeline mismatch 失败演练。
9. 更新文档和操作手册。

###### P3.10.1：真实 PostgreSQL + Barman 演练环境

目标：准备一套可重复启动、可由 OpsHub 真实调用的 PostgreSQL + Barman + SSH Runner 环境，不再只依赖 mock / 单元测试证明 P3 链路。

2026-05-02 已落地：

1. 新增 `scripts/pitr-e2e/docker-compose.yml`：
   - `opshub-pitr-postgres`：PostgreSQL 16，开启 `wal_level=replica`、`max_wal_senders`、`max_replication_slots`，暴露 `55432` 给 OpsHub backend 容器访问。
   - `opshub-pitr-barman-runner`：Barman + SSH Server，使用 host network，暴露 SSH `2222`，挂载 `/var/run/docker.sock` 和宿主 `/usr/bin/docker`，用于执行 Barman 命令和启动隔离恢复容器。
   - `/var/lib/opshub-pitr-e2e` 在宿主和 Runner 中保持同路径挂载，保证 Runner 下发的恢复目录可以被宿主 Docker 作为 bind mount 使用。
2. 新增 PostgreSQL 初始化脚本：
   - 创建 `opshub_pitr` 数据库。
   - 创建 `opshub` 校验用户和 `barman` 备份/复制用户。
   - 创建 `pitr_marker` 测试表。
   - 追加 `pg_hba.conf` replication 规则，允许 Barman streaming 连接。
3. 新增 Barman 配置：
   - server name 固定为 `pg-main`。
   - 使用 `backup_method=postgres`。
   - 使用 `streaming_archiver=on` 和 replication slot `opshub_pitr_barman`。
   - 演练环境 `minimum_redundancy=0`，避免第一次备份前健康检查必然失败；生产环境仍应按保留策略设置大于 0 的冗余要求。
4. 新增 `scripts/pitr-e2e/p3-10-target-time.sh`：
   - 自动启动演练环境。
   - 自动启动 `barman receive-wal`。
   - 自动登录 OpsHub。
   - 自动登记临时凭据、PostgreSQL 实例、SSH Runner、Barman Server。
   - 自动触发 Barman check / backup / catalog sync / WAL sync。
   - 默认清理本演练创建的 compose volume 和 `opshub-pg-restore-*` 隔离恢复容器，保证脚本可重复执行。
5. 演练脚本默认使用 `barmanGetWal=false`：
   - Barman restore 会预取所需 WAL 到恢复目录。
   - 隔离 PostgreSQL 容器不需要内置 `barman` CLI。
   - 如果后续要验证 `barman get-wal` 模式，可设置 `PITR_BARMAN_GET_WAL=true`，但恢复镜像必须能执行 `barman get-wal`。

P3.10.1 过程中修复的问题：

1. PostgreSQL 默认 `pg_hba.conf` 没有 replication 规则，导致 Barman `receive-wal` 报 `no pg_hba.conf entry for replication connection`；已通过初始化脚本补齐。
2. 测试脚本原 `pkill -f "barman receive-wal pg-main"` 会匹配到自身 shell 命令并导致脚本 SIGTERM；已改为 `[b]arman receive-wal pg-main` 安全匹配。
3. Debian `docker.io` 包在该基础镜像中没有提供 `/usr/bin/docker` CLI；已改为挂载宿主 `/usr/bin/docker`。
4. Barman `minimum_redundancy=1` 会让第一次备份前的 check 失败；演练环境改为 0，真实生产策略仍按恢复要求配置。

验收标准：

1. `docker compose -f scripts/pitr-e2e/docker-compose.yml up -d` 能启动 PostgreSQL 和 Barman Runner。
2. Runner 可通过 SSH 从 OpsHub backend 连接。
3. Barman `check pg-main` 除首次无备份场景外不应有连接、WAL streaming、slot、工具兼容错误。
4. OpsHub 能登记 PostgreSQL 实例、Runner Host 和 Barman Server。
5. OpsHub 能触发 Barman backup，并同步 catalog / WAL metadata。

###### P3.10.2：Barman target time 真实恢复演练

目标：用真实 PostgreSQL、真实 Barman backup、真实 WAL streaming 和 OpsHub restore plan，恢复到指定 target time 的隔离 PostgreSQL，并用 SQL 断言证明恢复结果。

2026-05-02 已跑通：

1. 源库写入 `before_backup`。
2. 通过 OpsHub 触发 Barman full backup。
3. backup 后强制 `pg_switch_wal()`，让 Barman receive-wal 尽快拿到一致性所需 WAL。
4. 记录 target time。
5. target time 后写入 `after_target`。
6. 再次 `pg_switch_wal()` 并同步 Barman catalog / WAL metadata。
7. 通过 OpsHub 创建 `restoreTargetType=time` 的 PostgreSQL / Barman PITR restore plan。
8. 通过 OpsHub 下发 Barman restore Runner：
   - `targetAction=pause`
   - `postgresStartInstance=true`
   - `barmanGetWal=false`
   - 启动隔离 PostgreSQL 容器。
9. 校验 SQL 断言：
   - `before_backup` 行数必须为 1。
   - `after_target` 行数必须为 0。
   - marker 汇总结果必须包含 `before_backup`。
10. 演练输出保存到 `scripts/pitr-e2e/results/p3-10-target-time-<run_id>.json`，该目录不入库。

实际通过记录：

1. run id：`20260502133755`
2. target time：`2026-05-02T13:38:44Z`
3. restore job：`30`
4. restore status：`success`
5. restored labels：`before_backup`
6. 三个业务断言全部 `passed`。

P3.10.2 过程中修复的问题：

1. Barman `list-backup <server>` 文本输出在当前版本中第一列为 server name，旧逻辑会把 `pg-main` 误当成 backup id；已修复为识别标准 `YYYYMMDDTHHMMSS` backup id，并保留 JSON catalog 解析。
2. Barman restore 生成的 `restore_command` 会使用 Runner 视角的绝对路径；隔离 PostgreSQL 容器内只能看到 `/var/lib/postgresql/data`。已在 Barman restore Runner 启动容器前重写 `postgresql.auto.conf` 中的 restored datadir 前缀。
3. `--get-wal` 模式要求恢复容器内存在 `barman` CLI；P3.10.2 默认改为 `--no-get-wal`，让 Barman 预取 WAL，隔离容器只负责 PostgreSQL replay 和查询校验。

验收标准：

1. 恢复计划 `validationStatus=passed`。
2. Barman restore、PGDATA 校验、隔离 PostgreSQL 启动步骤全部成功。
3. 隔离库处于 target time 之前的数据状态：
   - `before_backup` 存在。
   - `after_target` 不存在。
4. `validationJson` 中断言状态为 `passed`。
5. `proofJson` 写入 restore job，包含 backup id、target time、Runner、容器、步骤和校验结果。

验收：

1. Barman 备份记录能同步到 OpsHub。
2. WAL 缺口能被恢复计划识别。
3. timeline 不匹配时恢复计划失败。
4. 能恢复到指定 target time 的隔离 PostgreSQL 实例。
5. 能恢复到指定 target LSN 的隔离 PostgreSQL 实例。
6. 如使用原生增量，恢复计划必须包含 `pg_combinebackup` 步骤。

###### P3.10.3：Barman target LSN / WAL gap / timeline mismatch 演练

目标：证明 Barman 深接入链路不仅能恢复到 target time，也能恢复到 target LSN，并且不会把 WAL 缺口或 timeline 不匹配误判为可恢复。

演练脚本：

```bash
scripts/pitr-e2e/p3-10-barman-lsn-negative.sh
```

执行内容：

1. 复用 P3.10 PostgreSQL + Barman Runner 环境。
2. 通过 OpsHub 触发 Barman full backup，并同步 Barman catalog/WAL。
3. 在 base backup 之后写入 `lsn_target` marker，记录 `pg_current_wal_lsn()` 作为恢复目标。
4. 在目标 LSN 之后写入 `after_lsn` marker，并强制 `pg_switch_wal()` 使 WAL 可归档。
5. 创建 `restoreTargetType=lsn` 的恢复计划，要求 `validation_status=passed`、`log_chain_status=complete`，并且 `plan_json.target.targetLsn` 与目标 LSN 一致。
6. 下发 Barman restore 到隔离 PostgreSQL 容器。
7. 使用校验 SQL 断言 `before_backup` 存在、`lsn_target` 存在、`after_lsn` 不存在。
8. 构造超前 target LSN，验证恢复计划失败，状态应为 `validation_status=failed`、`log_chain_status=missing_wal`。
9. 构造 `targetTimelineId=2`，验证恢复计划失败，状态应为 `validation_status=failed`、`log_chain_status=timeline_mismatch`。
10. 额外登记一条 `status=missing` 的 WAL segment 和一条 timeline=2 的 WAL metadata，用于前端 WAL 状态页观察缺口和 timeline 切换。

验收标准：

1. target LSN 隔离恢复成功，恢复后的 marker 精确等于 `before_backup,lsn_target`。
2. WAL gap 计划不能通过预校验，不能下发 Runner。
3. timeline mismatch 计划不能通过预校验，不能下发 Runner。
4. 失败原因在恢复计划 `message`、`plan_json.messages`、`proof_json.messages` 中可读。
5. 前端 WAL 状态页能通过 `/api/v1/databases/log-archives?archiveType=wal` 的元数据看到缺口和 timeline 切换。

输出证明：

```text
scripts/pitr-e2e/results/p3-10-barman-lsn-negative-<run_id>.json
```

2026-05-02 实际通过记录：

1. 单项通过：
   - result：`scripts/pitr-e2e/results/p3-10-barman-lsn-negative-20260502142001.json`
   - target LSN 恢复成功，隔离库 marker 为 `before_backup,lsn_target`。
   - WAL gap 预校验失败，`logChainStatus=missing_wal`。
   - timeline mismatch 预校验失败，`logChainStatus=timeline_mismatch`。
2. P3.10 总入口通过：
   - result：`scripts/pitr-e2e/results/p3-10-barman-lsn-negative-20260502150802-lsn.json`
   - 同时登记了 `status=missing` WAL 和 timeline=2 WAL metadata，用于前端 WAL 状态页展示缺口和 timeline 切换。

###### P3.10.4：pg_basebackup full / incremental 演练

目标：证明 `pg_basebackup` 轻量备用链路具备真实 artifact 恢复能力，full + WAL 和 full + incremental + `pg_combinebackup` + WAL 都能恢复到目标时间点。

演练脚本：

```bash
scripts/pitr-e2e/p3-10-pg-basebackup.sh
```

环境要求：

1. PostgreSQL 镜像需要支持原生 incremental；默认使用 `registry.cn-guangzhou.aliyuncs.com/xingcangku/postgres:17`。
2. Runner 侧必须能执行 `pg_basebackup`、`pg_combinebackup`、`pg_verifybackup`、`pg_controldata`、`psql` 和 `docker`。
3. PostgreSQL 17 incremental 需要开启 WAL summarizer；脚本会检测 `SHOW summarize_wal`，未开启时执行 `ALTER SYSTEM SET summarize_wal = on` 并重启测试 PostgreSQL。
4. WAL artifact 必须是 Runner 可直接读取的 `runner://runner-host-<id>/path/to/wal`，不使用 `barman://` 作为 `pg_basebackup` 自动恢复输入。

执行内容：

1. 创建 OpsHub PostgreSQL 实例、SSH Runner、Barman server 和专用 WAL external stream。
2. 写入 `before_full` marker，通过 OpsHub `backup_engine=pg_basebackup` 备份任务生成 full base artifact。
3. 写入 `full_target` marker，记录 target time，再写入 `after_full_target`。
4. 从 Runner 的 Barman WAL 目录登记 finalized WAL 文件，记录 `storage_uri`、`pg_system_identifier`、`timeline_id`、`start_lsn/end_lsn`、`checksum_sha256` 和事件时间窗口。
5. 创建固定 `baseRecordId=<full record id>` 的 target time 恢复计划。
6. 下发 `pg_basebackup_restore` 到隔离 PostgreSQL 容器，并断言 `before_full/full_target` 存在、`after_full_target` 不存在。
7. 构造一个 `runner://` 指向不存在文件的 pg_basebackup full 记录，验证 artifact readiness gate：后端必须在创建恢复任务前通过 SSH Runner 同步检查文件存在性、大小和 checksum，文件不可读时拒绝下发。
8. 在源库继续写入 `before_incremental`，用 `pg_basebackup --incremental=<base backup_manifest>` 生成 incremental artifact。
9. 登记 incremental backup record，要求 `backup_level=incremental`、`backup_engine=pg_basebackup`、`base_record_id=<full>`、`parent_record_id=<full>`、`backup_manifest_checksum` 和 Runner 可读 `storage_uri`。
10. 写入 `incremental_target` marker，记录 target time，再写入 `after_incremental_target`，并登记新的 Runner 可读 WAL。
11. 创建 target time 恢复计划，要求 `selected_backup_record_ids` 包含 full + incremental，`plan_json.restoreSteps` 包含 `pg_combinebackup synthetic full`，`required_tool_json` 包含 `pg_combinebackup`。
12. 下发恢复任务，Runner 解包 full/incremental，执行 `pg_combinebackup` 合成 synthetic full，再配置 WAL `restore_command`，启动隔离 PostgreSQL 并执行校验 SQL 断言。

验收标准：

1. full + WAL 可以恢复到 target time。
2. full + incremental + `pg_combinebackup` + WAL 可以恢复到 target time。
3. 不可读 artifact 在恢复任务创建前被后端拒绝，不进入 Runner 队列。
4. incremental 恢复计划明确展示 `pg_combinebackup` 步骤。
5. incremental proof 明确记录 `combineBackupStatus=success`、`combineBackupVersion` 和 `syntheticFullPath`。
6. 校验 SQL 断言可以自动判断行数、包含内容和标量值，不需要人工翻 proof。

输出证明：

```text
scripts/pitr-e2e/results/p3-10-pg-basebackup-<run_id>.json
```

2026-05-02 过程中修复的问题：

1. Runner 镜像中的 PostgreSQL 17 工具不在默认 `PATH`：
   - `pg_combinebackup`、`pg_verifybackup`、`pg_controldata` 等工具实际位于 `/usr/lib/postgresql/<major>/bin`。
   - Runner Dockerfile 改为把这些工具软链到 `/usr/local/bin`。
   - Barman 配置不再硬编码 PostgreSQL 16 的 `path_prefix`，避免 PostgreSQL 17 演练时工具版本不匹配。
2. `pg_basebackup` 需要 replication 连接：
   - 初始化脚本为 `opshub` 用户补充 `host replication` 规则。
   - 否则 full backup 会报 `no pg_hba.conf entry for replication connection`。
3. WAL 时间窗口登记需要统一时区语义：
   - OpsHub MySQL 连接使用 `loc=Local`，演练脚本登记 WAL `firstEventTime/lastEventTime` 时按本地时间写入。
   - 避免把本地时间用 `date -u -d` 误当 UTC 解析，造成 WAL 时间窗口漂移到未来。
4. Barman streaming WAL 轮转后可能还在 `streaming/` 目录：
   - 演练脚本除了 `wals/` 目录，也会纳入 `streaming/` 中已经完整命名、非 `.partial` 的 WAL segment。
   - PITR 计划仍只使用 finalized/完整 segment，不使用 active partial。
5. Barman `wals/` 中的历史 WAL 可能是 gzip 压缩文件：
   - `pg_basebackup` 自动恢复输入需要 raw WAL segment。
   - 演练脚本会把压缩 WAL 解到 `/var/lib/opshub-pitr-e2e/runner/wal-raw/<run_id>/`，登记 raw 文件的 `runner://` URI、大小和 checksum。
6. `pg_basebackup_restore` Runner 的隔离容器需要读取预取 WAL：
   - 恢复 Runner 启动容器时显式挂载 `$WAL_DIR:$WAL_DIR:ro`。
   - 启动前对 WAL 目录执行只读权限放开，避免容器内 PostgreSQL 无法读取 `restore_command` 文件。
7. 隔离 PostgreSQL 容器失败时不能长时间等待：
   - readiness 循环增加 Docker container running 状态检查。
   - 容器退出时立即采集日志并失败，不再等满 120 次轮询。
8. target time 需要避免秒级截断误差：
   - 演练 marker 写入后使用 `created_at + interval '1 second'` 作为目标时间。
   - 避免 OpsHub 将 RFC3339 归一化到秒时，把刚写入的 marker 排除在恢复目标之外。
9. 脚本通过 heredoc 下发到 `docker exec` 时必须使用 `-i`：
   - WAL raw 转换和 incremental artifact 生成都需要把 shell 脚本传入 Runner 容器。
   - 缺少 `-i` 会导致命令体没有执行，进而出现空 JSON 或无 WAL 文件。

2026-05-02 实际通过记录：

1. 单项通过：
   - result：`scripts/pitr-e2e/results/p3-10-pg-basebackup-20260502150615.json`
   - full + WAL target time 恢复成功。
   - bad artifact gate 成功阻止不可读 `runner://` artifact 下发。
   - full + incremental + `pg_combinebackup` + WAL target time 恢复成功。
   - incremental proof 记录 `combineBackupStatus=success`。
2. P3.10 总入口通过：
   - result：`scripts/pitr-e2e/results/p3-10-pg-basebackup-20260502150802-pgbase.json`
   - full restore job 和 incremental restore job 均为 `success`。
   - validation assertions 自动通过，不需要人工检查 proof 输出。

###### P3.10.5：P3.10 总体验收脚本和运行证明

目标：把 P3.10 的 target time、target LSN、失败链路和 pg_basebackup full/incremental 演练串成可重复回归入口。

总入口：

```bash
scripts/pitr-e2e/p3-10-all.sh
```

执行顺序：

1. `p3-10-target-time.sh`
2. `p3-10-barman-lsn-negative.sh`
3. `p3-10-pg-basebackup.sh`

验收标准：

1. 三个脚本都能独立执行，也能通过总入口顺序执行。
2. 每个脚本使用独立 `PITR_RUN_ID`，避免 OpsHub 实例名、凭据名、恢复端口和结果文件互相覆盖。
3. 结果 JSON 保存在 `scripts/pitr-e2e/results/`，该目录继续由 `.gitignore` 忽略。
4. P3.10.5 不新增生产功能，只作为联调、回归和证明入口。

2026-05-02 实际通过记录：

```bash
PITR_BUILD_RUNNER=auto scripts/pitr-e2e/p3-10-all.sh
```

通过结果：

```text
P3.10 full rehearsal suite passed: 20260502150802
```

本次总入口生成的证明文件：

1. `scripts/pitr-e2e/results/p3-10-target-time-20260502150802-time.json`
2. `scripts/pitr-e2e/results/p3-10-barman-lsn-negative-20260502150802-lsn.json`
3. `scripts/pitr-e2e/results/p3-10-pg-basebackup-20260502150802-pgbase.json`

配套常规验证：

1. `go test ./internal/biz/database` 通过。
2. `npm run build` 在 `web/` 目录通过。
3. `npm run typecheck` 在 `web/` 目录通过；前端 typecheck CI 门禁通过 `.github/workflows/frontend-typecheck.yml` 开启。

#### P3 总体验收

1. PostgreSQL 物理备份任务页面明确显示 cluster 级。
2. Barman 备份记录能同步到 OpsHub。
3. WAL 缺口能被恢复计划识别。
4. timeline 不匹配时恢复计划失败。
5. 能恢复到指定 target time 或 target LSN 的隔离 PostgreSQL 实例。
6. 如使用原生增量，恢复计划必须包含 pg_combinebackup 步骤。

### P4：副本治理和误操作事故剧本

目标：把实时从库、延迟从库纳入数据库灾备闭环，让 OpsHub 能回答“这个主库有没有可用副本、有没有误删缓冲窗口、当前副本是否健康、事故发生时应该先看哪台延迟副本”。P4.1-P4.3 只读和生成指引，不执行暂停、切换、提升或重建；P4.4 才单独评审暂停 apply 执行。

P4 适配当前代码基线：

1. 后端已有数据库实例、权限、审计、Runner Job、PITR、日志归档和恢复计划模型，P4 应继续放在 `internal/biz/database`、`internal/data/database`、`internal/service/database`、`internal/server/database` 这一组边界内。
2. 前端数据库管理入口仍是 `web/src/views/asset/DatabaseManagement.vue`，建议在数据库管理页新增一级页签“副本治理”，或在 PITR 区域新增“副本状态 / 事故指引”子页签。
3. MySQL/MariaDB 无法可靠地从主库侧直接列出所有 replica；首版应扫描 OpsHub 已登记的数据库实例，对每个实例执行 replica/standby 状态采集，再用 source host、source port、server UUID、application_name、system_identifier 等信息归并拓扑。
4. PostgreSQL primary 侧 `pg_stat_replication` 只能看到当前连接的 standby；standby 侧 `pg_stat_wal_receiver`、`pg_is_in_recovery()`、`pg_last_wal_replay_lsn()`、`pg_last_xact_replay_timestamp()` 能补齐当前实例角色和 replay 状态。
5. 延迟副本 remaining delay 不是所有引擎都能精确给出。MySQL/MariaDB 可优先使用 `SQL_Remaining_Delay`；PostgreSQL 需要结合 `recovery_min_apply_delay`、replay timestamp 和主备时间差做近似展示，并明确标注估算。

#### P4 数据模型

新增 `database_instance_replicas`，记录 OpsHub 归并后的副本关系。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `primary_instance_id` | 推断或人工绑定的主库实例 |
| `replica_instance_id` | 从库 / standby 实例 |
| `engine` | `mysql / mariadb / postgresql` |
| `replica_role` | `realtime_replica / delayed_replica / standby / unknown` |
| `source_host` / `source_port` | MySQL/MariaDB source host/port 或 PostgreSQL primary conninfo 摘要 |
| `source_server_uuid` | MySQL/MariaDB source UUID，可选 |
| `pg_system_identifier` | PostgreSQL cluster system identifier，可选 |
| `application_name` | PostgreSQL standby application_name，可选 |
| `configured_delay_seconds` | 配置延迟，MySQL `SQL_Delay` / PostgreSQL `recovery_min_apply_delay` |
| `discovery_source` | `replica_status / primary_stat / manual / inferred` |
| `status` | `healthy / warning / critical / unknown` |
| `last_check_id` | 最近一次检查记录 |
| `last_checked_at` | 最近检查时间 |
| `last_error` | 最近错误摘要 |

新增 `database_replication_checks`，保存每次采集的原始和标准化状态。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `instance_id` | 被采集实例 |
| `replica_id` | 归并后的副本关系，可为空 |
| `engine` | 数据库类型 |
| `role_detected` | `primary / replica / standby / unknown` |
| `source_instance_id` | 推断来源实例，可为空 |
| `replica_io_running` | MySQL/MariaDB IO 线程状态 |
| `replica_sql_running` | MySQL/MariaDB SQL apply 线程状态 |
| `seconds_behind_source` | MySQL/MariaDB replication lag |
| `configured_delay_seconds` | 配置延迟 |
| `remaining_delay_seconds` | 剩余延迟 |
| `relay_log_bytes` | relay log 积压，能采集时记录 |
| `pg_write_lag_ms` / `pg_flush_lag_ms` / `pg_replay_lag_ms` | PostgreSQL primary 侧 lag |
| `pg_last_wal_replay_lsn` | PostgreSQL standby 最近 replay LSN |
| `pg_last_xact_replay_timestamp` | PostgreSQL standby 最近 replay 事务时间 |
| `wal_backlog_bytes` | PostgreSQL WAL 积压估算，能采集时记录 |
| `health_status` | `healthy / warning / critical / unknown` |
| `risk_flags_json` | 风险标记数组 |
| `raw_status_json` | 原始采集结果，脱敏后保存 |
| `checked_at` | 检查时间 |
| `error_message` | 错误 |

新增 `database_replica_incident_guides`，记录误删事故指引。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `incident_no` | 事故编号 |
| `primary_instance_id` | 事故主库 |
| `preferred_replica_id` | 推荐检查的延迟副本 |
| `incident_time` | 事故发生时间 |
| `incident_type` | `delete / update / release / other` |
| `affected_objects` | 影响表、库或 SQL 摘要 |
| `reason` | 用户填写的事故原因，必填 |
| `can_intercept` | 当前延迟副本是否还有机会截停 |
| `replay_time` | 当前 replay/apply 时间 |
| `remaining_delay_seconds` | 剩余保护窗口 |
| `guide_markdown` | 生成的操作指引 |
| `command_templates_json` | 暂停 apply 命令模板，只展示不执行 |
| `fallback_plan_json` | PITR 兜底建议 |
| `operator_id` / `operator_name` | 操作人 |
| `created_at` | 创建时间 |

P4.4 如进入执行阶段，再新增 `database_replica_actions`，专门记录 pause/resume apply 操作。

#### P4 API 草案

副本发现和状态：

```text
GET  /api/v1/databases/instances/{id}/replicas
GET  /api/v1/databases/instances/{id}/replication-status
POST /api/v1/databases/instances/{id}/replication-check
GET  /api/v1/databases/replicas
GET  /api/v1/databases/replication-checks
```

事故指引：

```text
POST /api/v1/databases/replica-incident-guides
GET  /api/v1/databases/replica-incident-guides
GET  /api/v1/databases/replica-incident-guides/{id}
```

P4.4 执行动作，先预留，不在 P4.1-P4.3 实现：

```text
POST /api/v1/databases/replicas/{id}/pause-apply
POST /api/v1/databases/replicas/{id}/resume-apply
```

#### P4 权限和审计

新增权限：

```text
database:replica:view
database:replica:check
database:replica:incident-guide
database:replica:pause-apply
database:replica:resume-apply
```

P4.1-P4.3 只需要 `view/check/incident-guide`。`pause-apply/resume-apply` 只在 P4.4 单独启用。

新增审计动作：

```text
replica_status_view
replica_check_run
replica_incident_guide
replica_pause_apply
replica_resume_apply
```

审计必须记录实例、操作人、采集来源、事故编号、用户填写原因、生成的命令模板和执行结果摘要。P4.1-P4.3 不记录任何已执行暂停命令，因为它们不执行命令。

#### P4.1：副本发现和状态展示

目标：先知道 OpsHub 已登记数据库实例里哪些是主库、实时从库、延迟从库或 PostgreSQL standby，并展示它们是否健康。

后端采集：

1. MySQL/MariaDB：
   - 优先执行 `SHOW REPLICA STATUS`。
   - 兼容旧版本 `SHOW SLAVE STATUS`。
   - 读取 `Source_Host` / `Master_Host`、`Source_Port` / `Master_Port`。
   - 读取 `Seconds_Behind_Source` / `Seconds_Behind_Master`。
   - 读取 `SQL_Delay`、`SQL_Remaining_Delay`。
   - 读取 `Replica_IO_Running` / `Slave_IO_Running`。
   - 读取 `Replica_SQL_Running` / `Slave_SQL_Running`。
   - 读取 `Retrieved_Gtid_Set`、`Executed_Gtid_Set`、`Relay_Log_File`、`Relay_Log_Pos`，能拿到时记录。
2. PostgreSQL primary：
   - 执行 `SELECT * FROM pg_stat_replication`。
   - 采集 `application_name`、`client_addr`、`state`、`sync_state`、`write_lag`、`flush_lag`、`replay_lag`、`sent_lsn`、`write_lsn`、`flush_lsn`、`replay_lsn`。
3. PostgreSQL standby：
   - 执行 `SELECT pg_is_in_recovery()` 判断角色。
   - 读取 `pg_stat_wal_receiver`。
   - 读取 `pg_last_wal_receive_lsn()`、`pg_last_wal_replay_lsn()`、`pg_last_xact_replay_timestamp()`。
   - 执行 `SHOW recovery_min_apply_delay`，能读取时映射为 configured delay。
4. 归并逻辑：
   - MySQL/MariaDB 先按 source host/port 匹配已登记实例；有 server UUID 时优先 UUID。
   - PostgreSQL 先按 `pg_system_identifier`、primary conninfo、host/port、application_name 归并。
   - 匹配不到主库时仍保存 replica 状态，但 `primary_instance_id` 为空，前端显示“来源未登记或无法匹配”。

前端内容：

1. 数据库管理新增“副本治理”页签。
2. 首屏展示副本关系表：
   - 主库实例。
   - 副本实例。
   - 引擎。
   - 角色：实时 / 延迟 / standby / 未知。
   - 复制状态。
   - lag。
   - remaining delay。
   - replay/apply 时间。
   - 最近检查时间。
   - 最近错误。
3. 支持手动刷新单实例和全量刷新。
4. 异常状态用明确标签展示：
   - IO 线程异常。
   - SQL/apply 线程异常。
   - lag 过大。
   - 来源主库未匹配。
   - PostgreSQL timeline/system identifier 不明确。

验收：

1. 能识别 MySQL/MariaDB 从库。
2. 能识别 PostgreSQL standby。
3. 能展示健康、异常、延迟过大和来源未匹配。
4. 不执行任何 pause、resume、promote、failover、切换动作。
5. 原始状态脱敏后可在详情中查看，便于排障。

P4.1 已落地内容：

1. 后端新增 `database_instance_replicas` 和 `database_replication_checks` 自动迁移模型。
2. 后端新增只读 API：
   - `GET /api/v1/databases/replicas`
   - `GET /api/v1/databases/replication-checks`
   - `GET /api/v1/databases/instances/{id}/replicas`
   - `GET /api/v1/databases/instances/{id}/replication-status`
   - `POST /api/v1/databases/instances/{id}/replication-check`
3. MySQL/MariaDB 采集优先 `SHOW REPLICA STATUS`，失败后兼容 `SHOW SLAVE STATUS`；采集 IO/SQL 线程、source host/port、lag、configured delay、remaining delay、GTID 和 relay 摘要。
4. PostgreSQL 采集使用 `pg_is_in_recovery()` 区分 primary/standby；primary 侧读取 `pg_stat_replication`，standby 侧读取 `pg_stat_wal_receiver`、`pg_last_wal_receive_lsn()`、`pg_last_wal_replay_lsn()`、`pg_last_xact_replay_timestamp()` 和 `recovery_min_apply_delay`。
5. 副本关系归并按已登记实例 host/port 推断主库；匹配不到仍保留采集记录，并在 UI 显示“来源未匹配”。
6. 原始采集结果写入 `raw_status_json` 前会对 `password` / `conninfo password=` 做脱敏。
7. 前端新增“副本治理”页签，包含副本关系表、最近采集表、单实例采集、全量采集和原始状态详情。
8. 新增 UI 权限 `database:replica:view`、`database:replica:check`，并纳入 admin 菜单初始化。
9. P4.1 仍然只读；代码中没有实现 pause/resume/promote/failover/switchover 路由或命令。

#### P4.2：延迟副本监控和风险提示

目标：把延迟副本变成可见的误操作保护能力，让用户能一眼看到当前库是否还有“误删缓冲窗口”。

识别规则：

1. MySQL/MariaDB：
   - `SQL_Delay > 0` 判定为 delayed replica。
   - `SQL_Remaining_Delay > 0` 表示当前还有剩余保护窗口。
   - `Replica_SQL_Running != Yes` 时必须标记 apply 异常。
2. PostgreSQL：
   - `recovery_min_apply_delay > 0` 判定为 delayed standby。
   - `pg_last_xact_replay_timestamp()` 与当前时间、配置延迟结合估算保护窗口。
   - standby 未处于 recovery 状态时必须标记角色异常。

阈值配置：

1. `lag_warning_seconds`：默认 300。
2. `lag_critical_seconds`：默认 1800。
3. `remaining_delay_warning_seconds`：默认 300。
4. `relay_log_backlog_warning_bytes`：可选。
5. `wal_backlog_warning_bytes`：可选。

风险提示：

1. 没有延迟副本：当前主库没有短窗口误删保护。
2. 延迟副本已追上：当前 remaining delay 为 0，无法截停刚才的误操作。
3. apply 线程异常：副本可能已经不可用，需先排障。
4. lag 过大：副本数据太旧，回填风险升高。
5. relay log / WAL 积压过大：磁盘和恢复窗口存在风险。
6. 主从时钟不一致：PostgreSQL remaining delay 估算可能不可靠。

前端内容：

1. 在副本治理页增加“误操作保护窗口”卡片。
2. 对每个主库展示：
   - 是否有延迟副本。
   - 最佳延迟副本。
   - configured delay。
   - remaining delay。
   - replay/apply time。
   - 当前风险。
3. 提供“生成事故指引”入口。

验收：

1. 用户能看到每个主库是否有延迟副本。
2. 用户能看到剩余保护窗口和 replay/apply 时间。
3. 延迟副本不健康时有明确风险提示。
4. 所有状态只读，不触发高风险命令。

P4.2 已落地内容：

1. 新增只读保护窗口汇总接口：
   - `GET /api/v1/databases/replica-protections`
   - 复用 `database:replica:view` 菜单权限和数据库实例拓扑权限范围。
   - 支持按主库实例、引擎、保护状态、风险等级过滤。
2. 后端按已登记副本关系聚合每个主库的误操作保护状态：
   - 排除已识别为 replica/standby 的实例，避免把从库当作主库展示。
   - 识别是否存在延迟副本。
   - 选择最佳延迟副本。
   - 展示 configured delay、remaining delay、apply/replay 时间、最近采集时间和风险消息。
3. P4.2 第一版阈值采用 API 查询参数覆盖，默认值为：
   - `lag_warning_seconds = 300`
   - `lag_critical_seconds = 1800`
   - `remaining_delay_warning_seconds = 300`
   - `relay_log_backlog_warning_bytes = 0`，表示不启用。
   - `wal_backlog_warning_bytes = 0`，表示不启用。
4. MySQL/MariaDB 延迟副本风险判断：
   - `SQL_Delay > 0` 的副本纳入延迟保护窗口。
   - `SQL_Remaining_Delay = 0` 标记“延迟副本已追上，当前没有可截停窗口”。
   - `Replica_IO_Running / Replica_SQL_Running` 异常继续按 P4.1 采集结果标记风险。
   - `Seconds_Behind_Source` 超过阈值时标记复制延迟风险。
5. PostgreSQL 延迟 standby 风险判断：
   - `recovery_min_apply_delay > 0` 的 standby 纳入延迟保护窗口。
   - `remainingDelaySeconds` 基于 `pg_last_xact_replay_timestamp()`、当前时间和配置延迟估算，前端明确显示“估算”。
   - `pg_stat_wal_receiver.status != streaming` 或来源主库无法匹配时继续标记风险。
6. 前端“副本治理”页签新增“误操作保护窗口”表格：
   - 展示主库、保护状态、最佳延迟副本、配置延迟、剩余窗口、apply/replay 时间、风险消息和最近检查。
   - 支持刷新保护窗口和重新采集。
   - 预留“生成事故指引”入口，但只提示 P4.3 启用，不执行任何命令。
7. P4.2 仍然只读；没有实现 pause、resume、promote、failover、switchover，也没有增加 Runner 执行动作。

#### P4.3：误删事故指引

目标：发生误删、误更新或错误发布时，OpsHub 先生成可审计的操作剧本，指导用户判断是否还能通过延迟副本截停；如果不能，则提示转 PITR 兜底。

输入：

1. 事故实例。
2. 事故时间。
3. 事故类型：误删 / 误更新 / 错误发布 / 其他。
4. 影响库、表、SQL 摘要或业务对象。
5. 事故原因，必填。
6. 期望恢复方式：导出回填 / 整库回滚 / 暂不确定。

输出：

1. 推荐立即检查哪个延迟副本。
2. 当前延迟副本 replay/apply 时间。
3. 当前 remaining delay。
4. 是否还有机会截停。
5. MySQL/MariaDB 暂停 apply 命令模板：
   - MySQL 8+：`STOP REPLICA SQL_THREAD;`
   - MySQL 旧版：`STOP SLAVE SQL_THREAD;`
6. PostgreSQL 暂停 replay 命令模板：
   - `SELECT pg_wal_replay_pause();`
7. 导出/回填建议：
   - 先只读连接延迟副本。
   - 校验误操作是否尚未 replay。
   - 导出影响表或影响行。
   - 回填前先在生产执行预检和审计。
8. PITR 兜底建议：
   - 选择事故前时间点。
   - 生成 PITR 恢复计划。
   - 恢复到隔离库。
   - 执行校验 SQL。
   - 导出缺失对象或准备整体切换。

指引格式：

1. 页面详情。
2. 可复制 Markdown。
3. 写入 `database_replica_incident_guides.guide_markdown`。
4. 写入审计 `replica_incident_guide`。

权限边界：

1. 只需要 `database:replica:incident-guide`。
2. 不需要 `database:replica:pause-apply`。
3. 不自动执行命令。
4. 命令模板必须标注“需在确认目标是 replica/standby 后执行”。

验收：

1. 用户必须填写事故原因才能生成。
2. 指引中明确显示是否还有截停机会。
3. 指引写入审计。
4. 指引能关联最近一次 replication check。
5. 不执行任何 pause/resume 命令。

P4.3 已落地内容：

1. 新增只读事故指引模型 `database_replica_incident_guides`：
   - 保存事故实例、事故时间、事故类型、影响摘要、事故原因和期望恢复方式。
   - 绑定推荐延迟副本、推荐副本实例、最近一次 `database_replication_checks`。
   - 记录生成时的 `remaining_delay_seconds` 和 `can_intercept` 判断。
   - 保存可复制 Markdown 指引 `guide_markdown` 和结构化摘要 `guide_json`。
2. 新增 API：
   - `POST /api/v1/databases/replica-incident-guides`
   - `GET /api/v1/databases/replica-incident-guides`
   - `GET /api/v1/databases/replica-incident-guides/{id}`
3. 新增菜单权限：
   - `database:replica:incident-guide`
   - 仅控制“生成事故指引”；列表和详情复用 `database:replica:view`。
   - 实例对象范围仍复用拓扑权限位，避免越权查看副本关系。
4. 指引生成逻辑：
   - 复用 P4.2 误操作保护窗口选择推荐延迟副本。
   - 如果没有延迟副本、没有最近检查、remaining delay 不足或副本严重异常，则明确提示转 PITR 兜底。
   - 如果仍可能截停，则提示立刻人工确认并在副本侧暂停 apply/replay。
5. 指引内容包含：
   - 事故信息。
   - 当前保护状态、风险等级、推荐延迟副本、最近检查、配置延迟、remaining delay、apply/replay 时间。
   - “OpsHub P4.3 不自动执行 pause/resume”的安全边界。
   - MySQL/MariaDB 和 PostgreSQL 暂停 apply/replay 命令模板。
   - 延迟副本导出回填流程和 PITR 兜底流程。
6. 前端“副本治理”页签新增：
   - 保护窗口表格中的“生成事故指引”入口。
   - 事故指引生成弹窗，强制填写事故原因和“不自动暂停 apply/replay”确认。
   - 事故指引列表、筛选和 Markdown 详情查看。
7. 审计：
   - 新增审计动作 `replica_incident_guide`。
   - 每次生成指引写入查询审计，记录推荐副本、关联检查和截停判断。
8. P4.3 仍然只读；没有实现 pause、resume、promote、failover、switchover，也没有通过 Runner 执行任何数据库命令。

#### P4.4：暂停 apply 执行，单独评审

目标：在 P4.1-P4.3 稳定后，再允许 OpsHub 通过 Runner 执行 pause/resume apply。这个阶段风险最高，必须独立评审、独立权限、独立验收，不和只读监控混在一起做。

新增权限：

```text
database:replica:pause-apply
database:replica:resume-apply
```

执行前置条件：

1. 目标实例必须被最近一次检查确认是 replica/standby。
2. 目标实例不能是 primary。
3. 最近检查时间不能超过阈值，例如 60 秒。
4. 用户必须输入事故编号。
5. 用户必须输入暂停或恢复原因。
6. 用户必须确认影响范围。
7. Runner 必须在允许主机和允许命令白名单内。

命令白名单：

1. MySQL 8+ pause：`STOP REPLICA SQL_THREAD;`
2. MySQL 8+ resume：`START REPLICA SQL_THREAD;`
3. MySQL 旧版 pause：`STOP SLAVE SQL_THREAD;`
4. MySQL 旧版 resume：`START SLAVE SQL_THREAD;`
5. PostgreSQL pause：`SELECT pg_wal_replay_pause();`
6. PostgreSQL resume：`SELECT pg_wal_replay_resume();`

执行流程：

```text
1. 用户从事故指引或副本详情进入 pause/resume。
2. 后端重新加载最近 replication check。
3. 校验目标不是 primary。
4. 校验权限和二次确认。
5. 生成 Runner Job，allowed_command 固定为 replica_pause_apply 或 replica_resume_apply。
6. Runner 执行白名单命令。
7. 执行后立即重新采集 replication status。
8. 保存前后状态、stdout/stderr 摘要和审计。
9. 前端显示“apply 已暂停 / 已恢复 / 执行失败”。
```

验收：

1. 只能对确认是 replica/standby 的实例执行。
2. 不能对 primary 执行。
3. 所有命令必须来自白名单模板，不能输入任意 SQL。
4. 执行失败能保留 stdout/stderr 摘要。
5. 前端有明显“已暂停 apply”状态。
6. 恢复 apply 也必须审计。
7. P4.4 上线前必须有真实 MySQL/MariaDB 和 PostgreSQL standby 演练。

P4.4 第一版落地边界：

1. 新增 `database_replica_actions` 表，专门记录 pause/resume apply 的执行记录、事故编号、原因、影响确认、前后检查、白名单命令、stdout/stderr 摘要、执行状态和操作人。
2. 新增菜单权限：
   - `database:replica:pause-apply`
   - `database:replica:resume-apply`
3. API：
   - `GET /api/v1/databases/replica-actions`
   - `POST /api/v1/databases/replicas/:id/pause-apply`
   - `POST /api/v1/databases/replicas/:id/resume-apply`
4. 第一版执行器不开放任意 SQL，也不做 promote/failover。后端复用数据库连接对目标副本执行固定 SQL 白名单；后续如需要更强隔离，可把同一 `allowed_command` 迁移到 Runner 主机执行。
5. 执行前后都会自动运行一次 `CheckInstanceReplication`：
   - 执行前确认目标当前检测角色是 MySQL/MariaDB `replica` 或 PostgreSQL `standby`。
   - 如果检测结果是 `primary`、`unknown` 或采集失败，动作失败并记录。
   - 执行后重新采集并把 `after_check_id` 和状态摘要写入记录。
6. MySQL/MariaDB 优先执行 `STOP/START REPLICA SQL_THREAD`，失败时回退到旧版 `STOP/START SLAVE SQL_THREAD`，最终实际命令写入记录。
7. PostgreSQL 执行 `SELECT pg_wal_replay_pause()` / `SELECT pg_wal_replay_resume()`；状态采集增加 `pg_is_wal_replay_paused()`，前端能显示 `Apply状态=已暂停`。
8. 每次执行都会写入统一查询审计：
   - `replica_pause_apply`
   - `replica_resume_apply`
   - `SQLType=REPLICA_APPLY_CONTROL`
   - `risk_level=high`
9. 前端“副本治理”新增：
   - 副本关系表的 `Apply状态` 列。
   - 暂停 / 恢复按钮，仅在拥有对应菜单权限时显示。
   - 强制填写事故编号、执行原因、影响确认和二次确认的弹窗。
   - `Apply 操作记录` 表格，展示动作、目标副本、命令、状态、原因、错误和操作人。

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
