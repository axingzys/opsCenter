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
