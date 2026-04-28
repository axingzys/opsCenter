# 数据库管理模块优化闭环实施方案

更新日期：2026-04-27

## 文档定位

本文用于承接数据库管理模块后续优化工作，重点不是继续扩大数据库类型，而是把现有 MySQL、MariaDB、PostgreSQL、Redis 等已接入能力做成可运维、可审计、可恢复、可治理的闭环。

本文基于当前代码实现整理：

1. 后端模型：`internal/biz/database/model.go`
2. 后端路由：`internal/server/database/http.go`
3. 后端服务：`internal/service/database/http.go`
4. 后端业务：`internal/biz/database/`
5. 数据层：`internal/data/database/`
6. 前端页面：`web/src/views/asset/DatabaseManagement.vue`
7. 前端接口：`web/src/api/database.ts`
8. 系统配置：`internal/biz/system/model.go`

## 结论

外部评估建议整体符合当前项目方向，尤其是以下判断：

1. 当前模块功能面已经很宽，下一步应优先补安全边界、运维闭环和前端信息架构。
2. 不应优先继续堆数据库类型，应先把 MySQL、PostgreSQL、Redis 的管理链路做扎实。
3. 风险总览、实例详情工作台、权限闭环、备份可恢复证明、巡检风险处理，是当前高收益方向。
4. 审计表已经成为统一留痕核心，应继续复用 `database_query_audits`，不要平行新增多套审计。

但需要校正两点：

1. 当前只读 SQL 并不是简单地“以 `WITH` 开头就放行”。`AnalyzeReadOnlySQL*` 会先判断首关键字，再通过 `findForbiddenSQLKeyword` 扫描被脱敏后的 token，拦截 `insert/update/delete/drop/truncate/alter/create/grant/revoke/replace/merge` 等写关键字。因此外部评估中关于 `WITH` 的风险判断方向成立，但当前项目不是完全裸露状态。
2. 部分体验能力已经实现，包括 SQL 格式化、查询历史、DDL 预览、数据字典导出、受控写入、Redis 只读命令、拓扑、容量趋势、巡检报告、备份和恢复演练。后续文档应避免把这些列为“从零新增”，而应定义为“增强”。

## 第一阶段落地状态

截至 2026-04-27，第一阶段 P0 已完成以下落地点：

1. SQL 安全专项测试补齐：覆盖可写 CTE、`WITH ... UPDATE/DELETE`、MySQL 可执行注释、字符串内危险关键字等场景。
2. SQL 安全规则加固：只读和写入预检查均拒绝数据库可执行注释 `/*! ... */`。
3. 查询链路数据库侧只读约束：MySQL / MariaDB / TiDB / OceanBase 与 PostgreSQL / openGauss / Kingbase 查询执行进入只读事务；PostgreSQL Schema 切换改为事务内 `SET LOCAL search_path`。
4. 实例对象权限模式显式化：新增 `database_instance_permission_mode`，支持 `compat` 和 `whitelist`；白名单模式下非 admin 默认无实例权限。
5. 权限模式前端提示：系统配置页可切换模式，实例权限页展示当前模式和是否真正收敛。
6. 备份可恢复证明字段补齐：备份记录增加压缩方式、是否加密、过期时间、校验时间、校验状态、校验消息、最近恢复演练时间和状态。
7. 手动备份校验接口：新增 `POST /api/v1/databases/backup-records/{id}/verify`，执行路径安全、文件存在、文件大小和 SHA256 校验，并写入 `backup_verify` 审计。
8. 下载和恢复演练前自动回写校验结果：下载前、恢复演练前都会更新备份记录校验状态。
9. 恢复演练拒绝审计：生产目标、类型不兼容、文件校验失败、目标凭据缺失、恢复策略不支持等拒绝路径写入 `restore_dry_run` 审计。

仍保留到后续阶段：

1. 方言级 AST parser 接入。当前已完成字符串规则加固和数据库侧只读事务兜底，但尚未引入 MySQL/PostgreSQL AST 解析依赖。
2. 完整审批流。当前仍是确认、原因、权限、只读事务、审计闭环，审批表和审批流放到 P2。

## 外部建议匹配度

| 外部建议 | 当前项目状态 | 是否符合 | 落地判断 |
| --- | --- | --- | --- |
| 修正 `WITH` 只读 SQL 风险 | 当前已扫描禁用写关键字，但仍是字符串规则 | 符合，但需校正表述 | 作为 P0 加固项，升级 AST/只读账号/只读事务，并补绕过测试 |
| 增加数据库风险总览 | 当前有 9 个页签，但缺统一风险入口 | 符合 | 作为 P1 默认首屏或新增首个页签 |
| 增加实例详情工作台 | 当前实例上下文分散在多个页签 | 符合 | 作为 P1，优先做抽屉或详情页 |
| 能力标记后端化 | 当前 `SupportedTypeVO` 只有部分能力字段 | 符合 | 作为 P1，扩展结构化能力矩阵 |
| 诊断页升级性能分析中心 | 已有诊断入口，但深度有限 | 符合 | 作为 P2，先做 MySQL/PostgreSQL/Redis |
| 备份恢复证明 | 已有备份、记录、恢复演练和校验和，但证明字段不足 | 符合 | 作为 P0/P1，补校验、过期、恢复演练状态 |
| 权限授权闭环 | 已有菜单权限和实例位图权限，但模式不显式 | 符合 | 作为 P0，增加兼容/白名单模式提示和配置 |
| SQL 控制台增强 | 已有格式化、历史、导出、执行计划、写前检查 | 部分符合 | 聚焦自动补全、收藏、影响行数预估、异步导出和审批 |
| 新增 `database_findings` | 当前 findings 在报告 JSON 中 | 符合 | 作为 P2 风险治理表 |
| 新增审批表 | 当前高危操作靠确认和原因，没有审批流 | 符合 | 作为 P2，先接生产写入/恢复/授权 |
| 新增告警规则和事件表 | 当前调度器有任务，但没有数据库告警闭环 | 符合 | 作为 P2，接通知渠道 |
| 继续增加数据库类型 | 当前类型面已经较宽 | 不建议优先 | 保持后置，先补闭环 |

## 当前能力基线

### 页面基线

当前 `DatabaseManagement.vue` 已经包含 9 个页签：

1. 实例管理
2. 实例权限
3. 结构浏览
4. SQL 控制台
5. 诊断
6. 拓扑
7. 备份任务
8. 巡检报告
9. 查询审计

这说明模块已经从“资产台账”扩展为“数据库运维入口”。下一步重点应是让这些页签之间形成闭环，而不是继续增加平行页签。

### 后端能力基线

当前后端已具备：

1. 实例 CRUD、启用、禁用、连接测试。
2. 元数据同步：Schema、表、字段、索引，Redis keyspace 和 key sample。
3. SQL 查询：只读 SQL、Redis 白名单命令、自动限行、超时控制。
4. SQL 安全：禁止多语句、写关键字拦截、敏感字段脱敏、结果大小限制。
5. 受控写入：MySQL、MariaDB、PostgreSQL 支持 `INSERT / UPDATE / DELETE`；DDL 结构变更独立通道默认关闭，第一批仅开放 `CREATE TABLE / CREATE INDEX`。
6. 审计：查询、导出、写入、诊断、拓扑、备份、恢复、权限变更统一写入 `database_query_audits`。
7. 备份：MySQL、MariaDB、PostgreSQL、Redis 逻辑备份。
8. 恢复演练：只允许非生产目标实例，支持 dry run 记录。
9. 容量采样：后台定时采集和手动采集。
10. 巡检报告：容量、性能、安全、备份四类摘要，findings 当前以 JSON 落在报告表中。
11. 权限：菜单权限 + 实例级对象权限位图。
12. 调度器：备份调度器、容量采样调度器。

### 当前数据模型基线

当前数据库管理模块已在 `cmd/server/server.go` 中加入自动迁移，核心模型集中在 `internal/biz/database/model.go`。

#### `database_instances`

数据库实例资产主表。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键，来自 `gorm.Model` |
| `created_at` / `updated_at` / `deleted_at` | 创建、更新、软删除时间 |
| `name` | 实例名称 |
| `db_type` | 数据库类型，如 `mysql/postgresql/redis` |
| `engine` | 引擎标识或发行版 |
| `version` | 数据库版本 |
| `host` | 主机地址 |
| `port` | 端口 |
| `default_database` | 默认库 |
| `credential_id` | 凭据 ID |
| `tls_enabled` | 是否启用 TLS |
| `connection_params` | 连接参数 JSON |
| `status` | 状态，如 `enabled/disabled` |
| `environment` | 环境，如生产、测试、预发 |
| `business_system` | 所属业务系统 |
| `owner` | 负责人 |
| `tags` | 标签 JSON |
| `remark` | 备注 |
| `last_test_at` | 最近连接测试时间 |
| `last_sync_at` | 最近元数据同步时间 |

#### `database_instance_permissions`

实例级对象权限表，用角色和权限位图控制实例访问。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `role_id` | 角色 ID |
| `instance_id` | 实例 ID |
| `permissions` | 权限位图 |

当前权限位：

| 权限 | 位值 | 说明 |
| --- | --- | --- |
| `VIEW` | 1 | 查看实例 |
| `QUERY` | 2 | 查询 |
| `EXPORT` | 4 | 导出 |
| `WRITE` | 8 | 受控写入 |
| `BACKUP` | 16 | 备份 |
| `RESTORE` | 32 | 恢复演练 |
| `DIAGNOSIS` | 64 | 诊断 |
| `TOPOLOGY` | 128 | 拓扑 |
| `MANAGE` | 256 | 管理 |
| `QUERY_UNLIMITED` | 512 | 查询不限最大行数 |
| `WRITE_EXPLAIN` | 1024 | 写 SQL 执行计划 |
| `DDL` | 2048 | DDL 结构变更 |
| `ALL` | 4095 | 全部权限 |

#### `database_schemas`

数据库 Schema 元数据。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `schema_name` | Schema 名称 |
| `charset` | 字符集 |
| `collation` | 排序规则 |
| `size_bytes` | 容量 |
| `table_count` | 表数量 |
| `last_sync_at` | 最近同步时间 |

#### `database_tables`

表元数据。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `schema_name` | Schema 名称 |
| `table_name` | 表名 |
| `table_type` | 表类型 |
| `engine` | 存储引擎 |
| `row_count` | 行数估算 |
| `data_size_bytes` | 数据大小 |
| `index_size_bytes` | 索引大小 |
| `comment` | 表注释 |
| `last_sync_at` | 最近同步时间 |

#### `database_columns`

字段元数据。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `schema_name` | Schema 名称 |
| `table_name` | 表名 |
| `column_name` | 字段名 |
| `ordinal_position` | 字段顺序 |
| `data_type` | 数据类型 |
| `is_nullable` | 是否可空 |
| `default_value` | 默认值 |
| `column_key` | 键类型 |
| `comment` | 字段注释 |
| `is_sensitive` | 是否敏感字段 |

#### `database_indexes`

索引元数据。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `schema_name` | Schema 名称 |
| `table_name` | 表名 |
| `index_name` | 索引名 |
| `index_type` | 索引类型 |
| `columns` | 索引字段 JSON |
| `is_unique` | 是否唯一索引 |
| `cardinality` | 基数 |
| `comment` | 索引备注 |

#### `database_redis_keyspaces`

Redis keyspace 元数据。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `db_index` | Redis DB 编号 |
| `keyspace_name` | keyspace 名称 |
| `key_count` | Key 数量 |
| `expiring_keys` | 设置过期时间的 Key 数量 |
| `avg_ttl_millis` | 平均 TTL |
| `sampled_keys` | 采样 Key 数 |
| `last_sync_at` | 最近同步时间 |

#### `database_redis_key_samples`

Redis Key 采样记录。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `db_index` | Redis DB 编号 |
| `keyspace_name` | keyspace 名称 |
| `key_name` | Key 名称 |
| `key_type` | Key 类型 |
| `ttl_millis` | TTL |
| `memory_usage_bytes` | 内存占用 |
| `value_size` | 值大小 |
| `encoding` | 编码 |
| `slot` | Cluster slot |
| `node_id` | 节点 ID |
| `node_address` | 节点地址 |
| `preview_text` | 预览文本 |
| `last_sync_at` | 最近同步时间 |

#### `database_sync_jobs`

元数据同步任务记录。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `trigger_type` | 触发方式 |
| `status` | 状态 |
| `message` | 消息 |
| `started_at` | 开始时间 |
| `finished_at` | 结束时间 |
| `duration_ms` | 耗时 |
| `schemas_count` | 同步 Schema 数 |
| `tables_count` | 同步表数 |
| `columns_count` | 同步字段数 |
| `indexes_count` | 同步索引数 |

#### `database_query_audits`

数据库操作统一审计表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `schema_name` | Schema 名称 |
| `operator_id` | 操作人 ID |
| `operator_name` | 操作人名称 |
| `audit_action` | 审计动作，如查询、导出、写入、诊断、备份、恢复 |
| `reason` | 操作原因 |
| `confirm_required` | 是否需要确认 |
| `confirmed` | 是否已确认 |
| `sql_text` | SQL 或命令文本 |
| `sql_fingerprint` | SQL 指纹 |
| `sql_type` | SQL 类型 |
| `risk_level` | 风险等级 |
| `status` | 状态 |
| `rows_returned` | 返回行数 |
| `rows_affected_limit` | 影响行数上限 |
| `rows_affected` | 实际影响行数 |
| `duration_ms` | 耗时 |
| `rollback_sql` | 回滚 SQL |
| `error_message` | 错误信息 |
| `client_ip` | 客户端 IP |

#### `database_backup_tasks`

备份任务表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `name` | 任务名称 |
| `backup_type` | 备份类型 |
| `schedule` | 调度表达式 |
| `storage_type` | 存储类型 |
| `storage_config` | 存储配置 JSON |
| `retention_days` | 保留天数 |
| `enabled` | 是否启用 |
| `last_run_at` | 最近运行时间 |
| `last_status` | 最近状态 |
| `last_message` | 最近消息 |

#### `database_backup_records`

备份执行记录表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `task_id` | 备份任务 ID |
| `instance_id` | 实例 ID |
| `trigger_type` | 触发方式 |
| `backup_type` | 备份类型 |
| `storage_type` | 存储类型 |
| `status` | 状态 |
| `file_path` | 文件路径 |
| `file_name` | 文件名 |
| `file_size` | 文件大小 |
| `checksum_sha256` | SHA256 校验和 |
| `started_at` | 开始时间 |
| `finished_at` | 结束时间 |
| `duration_ms` | 耗时 |
| `error_message` | 错误信息 |

#### `database_restore_jobs`

恢复演练任务表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `backup_record_id` | 备份记录 ID |
| `source_instance_id` | 来源实例 ID |
| `target_instance_id` | 目标实例 ID |
| `restore_mode` | 恢复模式，当前重点是 dry run |
| `restore_strategy` | 恢复策略 |
| `status` | 状态 |
| `file_name` | 文件名 |
| `file_size` | 文件大小 |
| `operator_id` | 操作人 ID |
| `operator_name` | 操作人名称 |
| `started_at` | 开始时间 |
| `finished_at` | 结束时间 |
| `duration_ms` | 耗时 |
| `error_message` | 错误信息 |

#### `database_capacity_snapshots`

容量快照表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `object_type` | 对象类型，如实例、schema、table、redis_key |
| `schema_name` | Schema 名称 |
| `table_name` | 表名 |
| `schema_count` | Schema 数 |
| `table_count` | 表数 |
| `row_count` | 行数 |
| `data_size_bytes` | 数据大小 |
| `index_size_bytes` | 索引大小 |
| `total_size_bytes` | 总大小 |
| `collected_at` | 采集时间 |

#### `database_inspection_reports`

巡检报告表。

| 字段 | 说明 |
| --- | --- |
| `id` / `created_at` / `updated_at` / `deleted_at` | 通用字段 |
| `instance_id` | 实例 ID |
| `report_type` | 报告类型 |
| `status` | 状态 |
| `health_score` | 健康分 |
| `risk_level` | 风险等级 |
| `summary` | 汇总 |
| `capacity_summary` | 容量摘要 JSON |
| `performance_summary` | 性能摘要 JSON |
| `security_summary` | 安全摘要 JSON |
| `backup_summary` | 备份摘要 JSON |
| `findings` | 风险发现 JSON |
| `operator_id` | 操作人 ID |
| `operator_name` | 操作人名称 |
| `generated_at` | 生成时间 |
| `duration_ms` | 耗时 |
| `error_message` | 错误信息 |

### 当前系统配置基线

数据库模块当前依赖以下系统配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `database_write_enabled` | `false` | 是否允许数据库写入能力 |
| `database_write_explain_enabled` | `false` | 是否允许写 SQL 执行计划 |
| `database_ddl_enabled` | `false` | 是否允许 DDL 结构变更能力 |
| `database_ddl_high_risk_requires_confirm` | `true` | DDL 是否要求二次确认 |
| `database_ddl_reason_required` | `true` | DDL 是否要求填写原因 |
| `database_ddl_require_backup_hint` | `true` | DDL 是否提示备份或回滚方案 |
| `database_high_risk_requires_confirm` | `true` | 高风险操作是否需要确认 |
| `database_operation_reason_required` | `true` | 操作原因是否必填 |
| `database_max_affected_rows` | `1000` | 单次写入最大影响行数 |
| `database_default_backup_retention_days` | `7` | 默认备份保留天数 |
| `database_backup_storage_path` | `./data/database-backups` | 本地备份根目录 |

### 支持类型基线

当前能力矩阵由 `SupportedTypes()` 和前端 `supportedTypes` 驱动，大致如下：

| 类型 | 当前主要能力 |
| --- | --- |
| MySQL / MariaDB | 连接测试、元数据、查询、执行计划、诊断、备份、恢复演练、受控写入、受控 DDL |
| PostgreSQL | 连接测试、元数据、查询、执行计划、诊断、备份、恢复演练、受控写入、受控 DDL |
| Redis | 连接测试、元数据、只读命令、诊断、拓扑、备份、恢复演练 |
| SQL Server | 连接测试、元数据、查询 |
| ClickHouse | 连接测试、元数据、查询 |
| Oracle | 连接测试、元数据、查询 |
| MongoDB | 拓扑 |
| Elasticsearch / OpenSearch | 连接测试、拓扑 |
| TiDB / OceanBase | 按 MySQL 兼容协议接入 |
| openGauss / Kingbase | 按 PostgreSQL 兼容协议接入 |
| 达梦 | 类型注册，专用能力待接入 |

## 优先级总览

### P0：安全边界和闭环

1. SQL 只读判定加固。
2. 对象权限模式显式化。
3. 备份可恢复证明字段补齐。
4. 生产写入和恢复演练的高危确认进一步收口。
5. 所有被拒绝、失败、跳过的关键操作保持统一审计。

### P1：产品完整度和体验

1. 数据库风险总览页。
2. 实例详情工作台。
3. 后端能力矩阵实例化。
4. SQL 控制台体验增强。
5. 审计从流水账升级为聚合分析。
6. 结构浏览补字段搜索、敏感字段治理和变更差异。

### P2：运维价值增强

1. 巡检 findings 独立成表。
2. 操作审批流。
3. 数据库告警规则和事件。
4. 慢查询中心、锁等待、长事务、Big Key、Hot Key。
5. 定期恢复演练任务。
6. 更多数据库类型备份和诊断能力。

## P0 详细方案

### 1. SQL 只读判定加固

#### 当前状态

当前查询链路是：

1. `ExecuteQuery / ExplainQuery / ExportQuery`
2. `AnalyzeReadOnlySQLByDB / AnalyzeExplainSQLByDB`
3. `runAuditedQuery`
4. `executeSQLQuery`

当前规则已经做到：

1. 只允许 `SELECT / SHOW / DESC / DESCRIBE / EXPLAIN / WITH`。
2. 禁止多语句。
3. 对 SQL 文本去注释、屏蔽字符串和标识符后扫描禁用关键字。
4. `WITH` 中如果包含 `UPDATE / DELETE / INSERT / MERGE` 等 token，当前会被拒绝。
5. 被拒绝查询会通过 `recordDeniedQuery` 写入 `database_query_audits`。

#### 仍需优化

字符串规则仍有维护风险，建议增加以下加固：

1. 引入方言级 SQL AST parser，优先覆盖 MySQL、PostgreSQL。
2. AST 解析失败时按拒绝处理，除非在兼容模式中明确允许。
3. 对 CTE 内部语句、顶层语句、子查询语句统一判断是否只读。
4. 查询连接尽量使用只读账号。
5. MySQL / PostgreSQL 查询链路尝试增加数据库侧只读约束：
   - PostgreSQL：查询事务中设置只读事务。
   - MySQL：尽量使用只读账号；会话级只读能力需谨慎验证兼容性。
6. 单测补齐可写 CTE、注释绕过、字符串绕过、标识符绕过、方言语法边界。

#### 验收标准

1. `WITH c AS (DELETE FROM t WHERE id = 1 RETURNING *) SELECT * FROM c` 被拒绝。
2. `WITH c AS (SELECT 1) SELECT * FROM c` 被允许。
3. `WITH c AS (...) UPDATE ...` 被拒绝。
4. `EXPLAIN ANALYZE` 默认被拒绝。
5. 被拒绝 SQL 审计状态为 `denied`，记录 `sql_text`、`sql_fingerprint`、`client_ip`、`operator_id`。
6. AST 解析失败默认拒绝，并返回明确错误。

### 2. 对象权限模式显式化

#### 当前状态

当前对象权限规则在 `database_instance_permissions` 中。逻辑是：

1. 如果没有任何对象权限规则，则不启用实例级权限收敛。
2. 一旦存在规则，非 admin 用户只能访问授权实例。
3. Admin 角色拥有全部实例权限。

这个策略便于平滑升级，但 UI 上必须明确提示，否则管理员容易误以为对象权限已经生效。

#### 建议方案

新增系统配置：

1. `database_instance_permission_mode`
   - `compat`: 兼容模式，没有规则时不收敛。
   - `whitelist`: 白名单模式，非 admin 默认无实例权限。
2. `database_instance_permission_mode_changed_at`
3. `database_instance_permission_mode_changed_by`

前端实例权限页增加明显状态：

1. 当前权限模式。
2. 未启用白名单时展示风险提示。
3. 切换到白名单前展示影响范围：
   - 当前用户数。
   - 当前角色数。
   - 已配置实例授权数。
   - 可能失去访问的非 admin 用户。

#### 验收标准

1. 白名单模式下，无对象权限的非 admin 用户看不到任何实例。
2. 兼容模式下，保持现有行为。
3. 模式切换写入 `database_query_audits` 或系统操作审计。
4. 前端顶部明确显示权限模式，不允许隐藏在文案中。

### 3. 备份可恢复证明补齐

#### 当前状态

当前已有：

1. `database_backup_tasks`
2. `database_backup_records`
3. `database_restore_jobs`
4. 文件 SHA256 校验和 `checksum_sha256`
5. 本地备份目录安全校验
6. 恢复演练限制非生产目标实例

#### 建议补充字段

在 `database_backup_records` 上补：

1. `encrypted`：是否加密。
2. `compression`：压缩方式，如 `gzip`、`zstd`、`none`。
3. `expires_at`：按保留策略计算的过期时间。
4. `verified_at`：最近一次文件校验时间。
5. `verify_status`：`pending/success/failed`。
6. `restore_tested_at`：最近一次恢复演练时间。
7. `restore_test_status`：最近恢复演练结果。
8. `backup_scope`：`database/schema/table/all_keys`。
9. `scope_config`：备份范围配置 JSON。

在 `database_backup_tasks` 上补：

1. `rpo_minutes`
2. `rto_minutes`
3. `backup_scope`
4. `scope_config`
5. `next_run_at`
6. `last_success_at`
7. `last_restore_test_at`
8. `last_restore_test_status`

#### 验收标准

1. 备份记录能展示：文件是否存在、校验是否通过、是否过期、是否做过恢复演练。
2. 备份任务能展示：最近成功备份、最近恢复演练、下次运行时间。
3. 文件下载前会校验路径仍在备份根目录内。
4. 备份清理后记录仍保留，但文件路径和状态应能表达“已清理”。

### 4. 高危操作统一门禁

#### 当前状态

受控 DML / DDL 已有：

1. 全局写开关 `database_write_enabled`。
2. DDL 全局开关 `database_ddl_enabled`。
3. 高危确认 `database_high_risk_requires_confirm` / `database_ddl_high_risk_requires_confirm`。
4. 操作原因 `database_operation_reason_required` / `database_ddl_reason_required`。
5. DML 最大影响行数 `database_max_affected_rows`。
6. DML 执行时事务包裹，超过影响行数回滚。
7. DDL 首批只开放 `CREATE TABLE / CREATE INDEX`，并提示备份或回滚方案。

#### 建议补齐

1. 生产环境写入额外确认。
2. 生产环境恢复演练目标继续禁止，来源为生产时也显示强提示。
3. 写入前预估影响行数：
   - MySQL / PostgreSQL 对 `UPDATE / DELETE` 尝试转换为 `SELECT COUNT(*)`。
   - 无法预估时提示“无法预估”，按高风险处理。
4. `UPDATE / DELETE` 执行前生成预览查询建议。
5. 高危操作预留审批流，审批未完成时不能执行。

#### 验收标准

1. 生产实例写入必须展示生产标识和二次确认。
2. 无 WHERE 的 `UPDATE / DELETE` 继续拒绝。
3. 影响行数预估失败不会放行低风险。
4. 所有拒绝和失败均写入审计。

## P1 详细方案

### 1. 数据库风险总览

#### 目标

在 `/asset/databases` 增加“总览”页签，或把总览作为默认首屏。它要回答一个问题：管理员今天最应该处理什么。

#### 指标建议

| 模块 | 指标 |
| --- | --- |
| 实例健康 | 启用实例数、禁用实例数、最近连接失败、超过 N 天未同步结构 |
| 权限风险 | 生产实例写权限角色数、未启用对象权限模式、拥有管理权限的角色 |
| SQL 风险 | 今日高风险写入、被拦截 SQL、失败 SQL、慢 SQL 或慢命令 |
| 备份风险 | 未配置备份实例、最近备份失败、超过 N 天未成功备份、未做恢复演练 |
| 容量风险 | 容量增长最快实例、Top 大表、Redis Top Key 样本 |
| 巡检风险 | 低健康分报告、高危 findings、未处理 findings |

#### API 建议

新增：

`GET /api/v1/databases/overview`

返回建议结构：

```json
{
  "instanceSummary": {},
  "permissionSummary": {},
  "sqlRiskSummary": {},
  "backupSummary": {},
  "capacitySummary": {},
  "inspectionSummary": {},
  "todoItems": []
}
```

#### 验收标准

1. 无需进入 9 个页签即可看到当前最高风险项。
2. 每个风险卡片能跳转到对应实例、审计、备份、巡检记录。
3. 支持按环境过滤，生产环境默认优先。

### 2. 实例详情工作台

#### 目标

当前操作分散在多个页签。建议新增实例详情抽屉或详情页，作为单实例上下文工作台。

#### 内容结构

1. 基本信息：类型、版本、地址、环境、业务系统、负责人、标签、备注。
2. 连接状态：最近测试、延迟、失败原因、最近同步。
3. 能力概览：查询、写入、诊断、拓扑、备份、恢复演练、巡检。
4. 结构概览：Schema 数、表数、字段数、敏感字段数、索引数。
5. 权限概览：拥有查看、查询、导出、写入、备份、恢复、诊断、拓扑、管理权限的角色。
6. 备份概览：最近成功备份、最近失败、最近恢复演练、下次运行。
7. 容量趋势：当前容量、近 7 天增长、Top 表或 Top Key。
8. 最近操作：查询、写入、导出、备份、恢复、诊断审计。
9. 推荐动作：同步结构、配置备份、做恢复演练、处理巡检风险。

#### API 建议

新增：

`GET /api/v1/databases/instances/{id}/workbench`

或拆分：

1. `GET /api/v1/databases/instances/{id}/summary`
2. `GET /api/v1/databases/instances/{id}/recent-audits`
3. `GET /api/v1/databases/instances/{id}/risk-summary`

#### 验收标准

1. 从实例列表点击实例名进入详情。
2. 详情中所有动作复用现有权限和能力矩阵。
3. 不复制已有页签的大表，只聚合摘要和入口。

### 3. 后端能力矩阵实例化

#### 当前状态

当前 `SupportedTypeVO` 包含：

1. `metadataEnabled`
2. `queryEnabled`
3. `testEnabled`
4. `topologyEnabled`
5. `phase`

但备份、恢复、诊断、写入、执行计划、慢查询等能力没有完整结构化返回。

#### 建议结构

新增能力结构：

```go
type DatabaseCapabilities struct {
    TestConnection bool `json:"testConnection"`
    SyncMetadata   bool `json:"syncMetadata"`
    Query          bool `json:"query"`
    Explain        bool `json:"explain"`
    Write          bool `json:"write"`
    Export         bool `json:"export"`
    Diagnose       bool `json:"diagnose"`
    SlowQuery      bool `json:"slowQuery"`
    Sessions       bool `json:"sessions"`
    Topology       bool `json:"topology"`
    Backup         bool `json:"backup"`
    RestoreDrill   bool `json:"restoreDrill"`
    Capacity       bool `json:"capacity"`
    Inspect        bool `json:"inspect"`
}
```

在以下返回中加入：

1. `SupportedTypeVO`
2. `DatabaseInstanceVO`
3. 实例详情工作台

#### 前端变化

前端按钮禁用原因从“权限或能力暂未接入”拆成：

1. 无实例权限。
2. 当前数据库类型不支持。
3. 当前实例已禁用。
4. 当前功能被系统配置关闭。
5. 需要先同步元数据。

#### 验收标准

1. 前后端能力判断一致。
2. 加新数据库能力时只改后端能力表和对应实现，不需要大量前端硬编码。
3. UI 禁用原因可解释。

### 4. SQL 控制台增强

#### 已实现

1. SQL / Redis 命令执行。
2. SQL 格式化。
3. 查询历史。
4. 执行计划。
5. 查询导出。
6. 写前检查。
7. 受控写入和 DDL 结构变更分层治理。
8. 结果脱敏、截断和二进制预览。

#### 建议增强

1. 自动补全：Schema、表、字段、历史 SQL。
2. 收藏 SQL。
3. 最近使用实例和 Schema。
4. 查询结果列宽调整和复制。
5. 环境强提示：生产实例顶部高亮。
6. 写入前影响行数预估。
7. `UPDATE / DELETE` 预览查询生成。
8. DDL 结构变更继续扩展审批、结构 Diff 和更多方言级风险识别。
9. 大结果导出异步化。
10. 审批模式接入。

#### API 建议

1. `GET /api/v1/databases/instances/{id}/completion`
2. `GET /api/v1/databases/query-favorites`
3. `POST /api/v1/databases/query-favorites`
4. `DELETE /api/v1/databases/query-favorites/{id}`
5. `POST /api/v1/databases/instances/{id}/query/write/estimate`
6. `POST /api/v1/databases/instances/{id}/query/export-jobs`

### 5. 审计聚合分析

#### 当前状态

`database_query_audits` 已经记录 `sql_fingerprint`，但审计页主要还是流水账。

#### 建议新增表

`database_query_fingerprints`

字段建议：

1. `id`
2. `instance_id`
3. `schema_name`
4. `sql_fingerprint`
5. `sample_sql`
6. `sql_type`
7. `risk_level`
8. `total_count`
9. `success_count`
10. `error_count`
11. `denied_count`
12. `avg_duration_ms`
13. `max_duration_ms`
14. `rows_returned_total`
15. `rows_affected_total`
16. `first_seen_at`
17. `last_seen_at`

#### 验收标准

1. 审计页支持“按 SQL 指纹聚合”。
2. 可以看到失败最多、最慢、被拦截最多、高风险最多的 SQL 模板。
3. 保留流水账详情。

## P2 详细方案

### 1. 巡检 findings 独立成表

#### 当前状态

当前 `database_inspection_reports.findings` 是 JSON，适合报告详情展示，但不利于风险处理流转、统计、筛选。

#### 新表建议

`database_findings`

字段：

1. `id`
2. `instance_id`
3. `report_id`
4. `finding_type`
5. `category`
6. `severity`
7. `title`
8. `description`
9. `evidence`
10. `recommendation`
11. `resource_type`
12. `resource_id`
13. `status`：`open/ignored/fixed`
14. `owner`
15. `detected_at`
16. `fixed_at`
17. `ignored_at`
18. `ignored_reason`
19. `created_at`
20. `updated_at`
21. `deleted_at`

#### 流转建议

1. 巡检生成报告时同步写入 findings 表。
2. 相同 instance + category + resource + title 的未关闭 finding 可合并更新。
3. 用户可标记为已修复或已忽略。
4. 总览页只统计未关闭 findings。

### 2. 操作审批流

#### 新表建议

`database_operation_approvals`

字段：

1. `id`
2. `operation_type`：`write_sql/restore/grant_permission/backup_run/export`
3. `instance_id`
4. `operator_id`
5. `operator_name`
6. `risk_level`
7. `reason`
8. `request_payload`
9. `status`：`pending/approved/rejected/cancelled/executed/expired`
10. `approver_id`
11. `approver_name`
12. `approved_at`
13. `executed_at`
14. `expired_at`
15. `created_at`
16. `updated_at`
17. `deleted_at`

#### 首批接入范围

1. 生产实例写入。
2. 高风险写入。
3. 生产实例权限授权。
4. 恢复演练。
5. 大批量导出。

### 3. 告警规则和事件

#### 新表建议

`database_alert_rules`

字段：

1. `id`
2. `name`
3. `rule_type`：`backup_failed/capacity_growth/health_score/sync_failed/permission_risk/sql_denied`
4. `condition_json`
5. `enabled`
6. `notify_channels`
7. `created_by`
8. `created_at`
9. `updated_at`
10. `deleted_at`

`database_alert_events`

字段：

1. `id`
2. `rule_id`
3. `instance_id`
4. `severity`
5. `title`
6. `message`
7. `status`：`firing/resolved/silenced`
8. `first_seen_at`
9. `last_seen_at`
10. `resolved_at`
11. `created_at`
12. `updated_at`
13. `deleted_at`

#### 首批规则

1. 最近一次备份失败。
2. 超过 7 天无成功备份。
3. 容量 7 天增长超过 50%。
4. 巡检健康分低于 80。
5. 生产实例存在写权限。
6. SQL 被拦截次数突增。

## 前端优化清单

### 实例管理

1. 批量连接测试。
2. 批量同步元数据。
3. 批量设置负责人、标签、环境。
4. 异常实例置顶。
5. 生产实例高亮。
6. 点击实例名打开详情工作台。
7. 能力标签可解释。

### 实例权限

1. 权限模式展示：兼容模式 / 白名单模式。
2. 角色视角和实例视角切换。
3. 高危授权标识。
4. 临时授权和过期时间。
5. 一键回收角色在某实例上的权限。
6. 生产写权限风险提示。

### 结构浏览

1. 字段搜索。
2. 敏感字段扫描结果。
3. 表容量排行。
4. 表行数排行。
5. 本次同步和上次同步差异。
6. 元数据变更历史。

### SQL 控制台

1. 生产环境强提示。
2. 自动补全。
3. 收藏 SQL。
4. 影响行数预估。
5. 大结果异步导出。
6. 审批流入口。

### 诊断

1. MySQL / PostgreSQL 锁等待和长事务。
2. MySQL / PostgreSQL 慢 SQL 聚合。
3. Redis Big Key / Hot Key。
4. 诊断项直接生成 findings。
5. 每个诊断风险给出建议动作。

### 拓扑

1. Redis Cluster slot 分布可视化。
2. MongoDB ReplicaSet 角色和延迟。
3. Elasticsearch / OpenSearch 节点和分片异常提示。
4. 拓扑异常直接生成 findings。

### 备份任务

1. 下次运行时间。
2. 最近成功备份。
3. 最近恢复演练。
4. 备份文件过期时间。
5. 手动校验备份。
6. 备份文件大小趋势。

### 巡检报告

1. findings 独立列表。
2. findings 处理状态。
3. findings 负责人。
4. 忽略、确认、修复流转。
5. 健康分趋势。
6. 报告对比。

### 查询审计

1. SQL 指纹聚合。
2. 按操作人统计。
3. 按实例统计。
4. 失败 SQL 和被拦截 SQL 快速筛选。
5. 敏感字段访问记录。
6. 审计留存策略展示。

## 分阶段实施建议

### 第一阶段：P0 安全与闭环

建议目标版本：一个小版本内完成。

任务：

1. SQL 安全测试补齐。
2. AST 解析器技术预研和 MySQL / PostgreSQL 首批接入。
3. 查询链路只读事务或只读账号策略文档化。
4. 对象权限模式配置和 UI 提示。
5. 备份记录补 `expires_at/verified_at/verify_status/restore_tested_at/restore_test_status`。
6. 备份校验接口和手动校验按钮。

测试：

1. `go test ./internal/biz/database/...`
2. SQL 绕过用例专项测试。
3. 权限模式回归测试。
4. 备份校验和恢复演练回归测试。

### 第二阶段：P1 产品闭环

任务：

1. 总览页 API 和前端页签。
2. 实例详情工作台。
3. 能力矩阵后端结构化。
4. 审计指纹聚合表和聚合页。
5. SQL 收藏、自动补全、结果复制。

测试：

1. API 权限收敛测试。
2. 前端构建。
3. 核心页面手工回归。

### 第三阶段：P2 风险治理

任务：

1. `database_findings` 表。
2. 巡检 findings 生成和流转。
3. 审批表和首批审批流。
4. 告警规则和事件表。
5. 飞书、钉钉、企业微信、邮件、Webhook 通知。
6. 定期恢复演练任务。

测试：

1. findings 合并和状态流转测试。
2. 审批超时、拒绝、执行后状态测试。
3. 告警触发、恢复、静默测试。

## 暂不建议优先做

1. 继续堆数据库类型。
2. 表级权限。
3. 在线 DDL 平台。
4. 让 SQL 控制台替代完整 DMS 工具。
5. Elasticsearch / OpenSearch 非官方方式备份。

## 实现风险

1. SQL AST parser 和当前手写规则并存期间，要避免行为不一致。
2. 对象权限从兼容模式切换到白名单模式，可能影响现有用户访问。
3. 备份字段扩展需要兼容历史记录。
4. 恢复演练涉及外部命令和目标实例状态，必须保证幂等和失败可见。
5. 总览页聚合查询可能造成慢查询，需要先做简单聚合，后续再缓存。
6. findings 独立成表后，要处理旧报告 JSON findings 的兼容展示。

## 最小可落地版本建议

如果只做一个最小闭环版本，建议包含：

1. 总览页展示 6 类风险。
2. 实例详情工作台。
3. 对象权限模式提示和白名单模式。
4. 备份记录显示校验状态、过期状态、恢复演练状态。
5. SQL 安全专项测试和 AST 预研结果。
6. 巡检 findings 列表化，但处理流转可以后置。

这样能让数据库管理从“功能集合”升级为“运维闭环”，并且不会大幅重构现有代码。
