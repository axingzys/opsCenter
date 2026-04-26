# 数据库管理实施方案

## 文档定位
本文作为 OpsHub 后续数据库管理模块的主计划文档。后续数据库管理相关的表结构、接口、前端页面、权限、审计、验收和分期开发均以本文为基准；若范围或优先级发生变化，应同步更新本文。

## 背景
当前 OpsHub 已具备主机资产、凭据、资产分组、审计、监控、插件和虚拟化平台纳管能力，但数据库资产仍缺少统一入口。实际运维中，数据库不仅是一个连接地址，还包含实例、集群、库表结构、账号权限、慢 SQL、备份、性能指标、变更审计和风险控制等多类运维对象。

数据库管理模块不应只做一个简单 SQL 客户端，而应成为统一的数据库资产纳管和受控运维平台。

## 总体目标
1. 统一纳管多种数据库实例、集群和连接凭据。
2. 自动发现数据库元数据，包括实例版本、库、表、字段、索引、容量、主从或集群拓扑。
3. 提供安全 SQL 控制台，支持只读查询、结果导出、历史记录和执行审计。
4. 提供受控数据变更能力，支持高危 SQL 拦截、二次确认、审批预留和回滚辅助。
5. 提供性能诊断能力，包括慢 SQL、执行计划、锁等待、活跃会话、容量趋势和关键指标。
6. 提供备份恢复任务管理，支持定时备份、备份保留、失败告警和恢复演练。
7. 与现有 RBAC、凭据、审计、监控、告警、任务系统打通。

## 设计原则
1. 数据库资产与主机资产关联但不混用模型，数据库实例应独立建模。
2. 先做只读能力，再逐步开放写能力和恢复能力。
3. 默认安全：新接入实例默认只允许元数据发现和只读查询。
4. 所有查询、导出、变更、备份和恢复操作都必须审计。
5. 密码和密钥统一复用凭据模块，不在数据库实例表中保存明文密码。
6. Provider Adapter 归一化不同数据库差异，前端和业务层不直接依赖具体驱动实现。
7. 同步采集数据与人工维护字段隔离，避免自动采集覆盖人工备注、负责人、业务归属等信息。
8. 高风险操作必须具备权限校验、原因留痕、二次确认和可配置开关。

## 模块入口
建议新增菜单：

1. `资产管理 -> 数据库管理`
2. 路由建议：`/asset/databases`
3. 前端视图目录建议：`web/src/views/database/`
4. 前端接口文件建议：`web/src/api/database.ts`
5. 后端接口前缀建议：`/api/v1/databases`
6. 后端业务目录建议：`internal/biz/database/`
7. 后端服务目录建议：`internal/service/database/`

## 支持范围
### 第一批优先支持
1. MySQL
2. MariaDB
3. PostgreSQL
4. Redis
5. MongoDB

### 第二批支持
1. SQL Server
2. Oracle
3. ClickHouse
4. Elasticsearch
5. OpenSearch

### 第三批支持
1. TiDB
2. OceanBase
3. openGauss
4. 达梦
5. 人大金仓
6. Doris
7. StarRocks
8. InfluxDB
9. TimescaleDB
10. SQLite

## 数据库类型能力矩阵
| 类型 | 连接测试 | 元数据发现 | SQL 查询 | 结构管理 | 性能诊断 | 备份 | 写操作 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MySQL / MariaDB | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 分期开放 | 一期主力 |
| PostgreSQL | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 分期开放 | 一期主力 |
| Redis | 支持 | 支持 | 命令控制台 | Key 浏览 | 支持 | 支持 | 分期开放 | 一期主力 |
| MongoDB | 支持 | 支持 | 查询控制台 | 集合管理 | 支持 | 支持 | 分期开放 | 一期主力 |
| SQL Server | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 分期开放 | 二期 |
| Oracle | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 分期开放 | 二期，驱动和授权需确认 |
| ClickHouse | 支持 | 支持 | 支持 | 支持 | 支持 | 低优先级 | 谨慎开放 | 二期 |
| Elasticsearch / OpenSearch | 支持 | 支持 | DSL 查询 | 索引管理 | 支持 | 低优先级 | 谨慎开放 | 二期 |
| 国产数据库 | 支持 | 支持 | 支持 | 支持 | 视兼容性 | 视兼容性 | 分期开放 | 三期 |

## 功能模块
### 1. 数据库实例纳管
1. 新增、编辑、删除、启用、禁用数据库实例。
2. 支持实例类型、连接地址、端口、数据库名、连接参数、TLS 参数、超时参数。
3. 支持绑定现有凭据，也支持后续扩展临时凭据和动态凭据。
4. 支持负责人、部门、环境、业务系统、标签、机房、网络区域、备注。
5. 支持批量导入实例，格式可先支持 CSV，后续支持 Excel。
6. 支持连通性测试，返回版本、延迟、当前账号权限摘要。
7. 支持实例健康状态，包含正常、异常、禁用、凭据错误、网络不可达。

### 2. 自动发现与元数据同步
1. 自动识别数据库版本、字符集、时区、运行参数。
2. 同步库、Schema、表、视图、字段、索引、约束、触发器、存储过程。
3. 同步容量信息，包括数据库大小、表大小、索引大小、行数估算。
4. 同步实例角色，包括主库、从库、只读副本、分片节点、集群节点。
5. 同步任务支持手动触发和定时执行。
6. 同步任务必须记录状态、耗时、错误信息、增删改统计。
7. 自动采集字段与人工维护字段分离。

### 3. 拓扑与资产视图
1. 实例列表：按类型、环境、状态、负责人、标签、业务系统筛选。
2. 实例详情：展示连接信息、版本、运行状态、容量、最近同步、最近备份。
3. 拓扑视图：展示实例、主从、集群、分片、只读副本。
4. Redis Cluster 展示节点角色、slots 分布、内存和 key 数。
5. MongoDB 展示 ReplicaSet、Primary、Secondary、Arbiter。
6. Elasticsearch 展示集群、节点、索引、分片和健康颜色。

### 4. SQL / 命令控制台
1. MySQL、PostgreSQL、SQL Server、Oracle、ClickHouse 支持 SQL 查询。
2. Redis 支持命令控制台和 Key 浏览。
3. MongoDB 支持 JSON 查询和集合浏览。
4. 支持最大返回行数、查询超时、只读模式、事务模式。
5. 支持结果分页、复制、CSV 导出、Excel 导出。
6. 支持 SQL 格式化、历史记录、收藏、常用语句模板。
7. 支持执行计划查看，例如 `EXPLAIN`、`EXPLAIN ANALYZE`。
8. 控制台默认只读，高风险写操作由全局开关和权限控制。

### 5. 结构管理
1. 数据库、Schema、表、视图、字段、索引浏览。
2. 展示建表语句、字段类型、默认值、是否可空、注释、索引。
3. 支持表容量、行数估算、最近更新时间。
4. 支持字段注释维护和数据字典导出，写操作需要单独权限。
5. 支持结构对比，后续可扩展两个实例之间的 Schema Diff。

### 6. 数据浏览与导出
1. 按表分页浏览数据。
2. 支持字段筛选、条件筛选、排序和关键字搜索。
3. 支持敏感字段脱敏展示。
4. 支持导出 CSV、Excel、JSON。
5. 导出必须记录审计，并可限制最大导出行数。
6. 导出敏感表或大批量数据应预留审批能力。

### 7. 数据变更
1. 支持 INSERT、UPDATE、DELETE、DDL 等变更操作。
2. 默认关闭写操作，需要系统配置开启。
3. 高危 SQL 需要二次确认和操作原因。
4. 支持影响行数预估和最大影响行数限制。
5. 支持变更前数据快照或回滚 SQL 生成。
6. 支持执行前 SQL 规则检查。
7. 后续支持审批流和定时执行。

### 8. 性能诊断
1. 慢 SQL 列表、执行次数、平均耗时、最大耗时、扫描行数。
2. 活跃会话、连接数、等待事件、锁等待。
3. 执行计划分析。
4. 参数巡检，例如最大连接数、慢查询开关、binlog、WAL、缓存配置。
5. 容量趋势，包括库大小、表大小、索引大小。
6. 关键指标趋势，包括 QPS、TPS、连接数、缓存命中率、复制延迟。
7. 支持一键生成诊断报告。

### 9. 备份与恢复
1. 支持逻辑备份，例如 `mysqldump`、`pg_dump`、`mongodump`、`redis-cli --rdb`。
2. 支持定时备份、手动备份、备份保留策略。
3. 支持备份文件列表、大小、耗时、状态、下载。
4. 支持恢复演练，优先恢复到临时实例或指定测试实例。
5. 生产恢复默认不在一期开放。
6. 备份失败必须告警，备份成功率应进入巡检报告。

### 10. 账号与权限管理
1. 展示数据库账号、角色和授权信息。
2. 支持账号过期、弱口令、过大权限巡检。
3. 后续可支持账号创建、授权、回收、密码轮换。
4. 数据库账号管理属于高风险能力，默认不在一期开放。

### 11. 审计与合规
1. 记录连接测试、元数据同步、SQL 查询、数据导出、数据变更、备份、恢复。
2. SQL 审计记录需要包含实例、库、表、操作类型、风险级别、影响行数、耗时。
3. 敏感 SQL 和敏感字段审计展示时需要脱敏。
4. 支持按用户、实例、时间、操作类型、风险等级检索。
5. 支持审计日志导出。
6. 支持审计日志保留策略。

### 12. 告警联动
1. 连接失败告警。
2. 慢 SQL 突增告警。
3. 复制延迟告警。
4. 磁盘或表空间水位告警。
5. Redis 内存水位告警。
6. Elasticsearch yellow / red 告警。
7. 备份失败告警。
8. 连接数接近上限告警。

## 后端架构
### 分层设计
1. `internal/biz/database/model.go`：数据库实例、元数据、任务、审计、备份模型。
2. `internal/biz/database/repository.go`：Repository 接口定义。
3. `internal/biz/database/usecase.go`：业务编排、权限校验、风险控制。
4. `internal/biz/database/adapter.go`：数据库适配器统一接口。
5. `internal/biz/database/adapter_mysql.go`：MySQL / MariaDB 适配器。
6. `internal/biz/database/adapter_postgres.go`：PostgreSQL 适配器。
7. `internal/biz/database/adapter_redis.go`：Redis 适配器。
8. `internal/biz/database/adapter_mongo.go`：MongoDB 适配器。
9. `internal/service/database/http.go`：HTTP Handler 和 Swagger 注释。
10. `internal/server/database/http.go`：路由注册。

### Adapter 统一接口草案
```go
type DatabaseAdapter interface {
    TestConnection(ctx context.Context, instance *DatabaseInstance, credential *Credential) (*ConnectionTestResult, error)
    DiscoverMetadata(ctx context.Context, instance *DatabaseInstance, credential *Credential) (*DatabaseMetadataSnapshot, error)
    ExecuteQuery(ctx context.Context, req *DatabaseQueryRequest) (*DatabaseQueryResult, error)
    Explain(ctx context.Context, req *DatabaseQueryRequest) (*DatabaseExplainResult, error)
    CollectMetrics(ctx context.Context, instance *DatabaseInstance, credential *Credential) (*DatabaseMetricSnapshot, error)
    BuildBackupCommand(ctx context.Context, req *DatabaseBackupRequest) (*DatabaseBackupCommand, error)
}
```

### 风险控制流程
1. 接收 SQL 或命令请求。
2. 校验登录态和 RBAC 权限。
3. 获取实例状态和写操作开关。
4. 解析 SQL 类型和风险等级。
5. 校验只读模式、最大返回行数、超时时间、最大影响行数。
6. 对高风险操作要求二次确认、操作原因和审批预留。
7. 执行前写入 pending 审计。
8. 执行数据库操作。
9. 更新审计状态、耗时、影响行数和错误信息。

## 数据模型草案
### `database_instances`
用途：数据库实例连接配置和资产属性。

核心字段：

1. `id`
2. `name`
3. `db_type`
4. `engine`
5. `version`
6. `host`
7. `port`
8. `default_database`
9. `credential_id`
10. `tls_enabled`
11. `tls_config`
12. `status`
13. `environment`
14. `business_system`
15. `owner_id`
16. `department_id`
17. `tags`
18. `last_test_at`
19. `last_sync_at`
20. `remark`
21. `created_at`
22. `updated_at`

### `database_schemas`
用途：数据库或 Schema 元数据。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `charset`
5. `collation`
6. `size_bytes`
7. `table_count`
8. `source`
9. `last_sync_at`

### `database_tables`
用途：表和视图元数据。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `table_name`
5. `table_type`
6. `engine`
7. `row_count`
8. `data_size_bytes`
9. `index_size_bytes`
10. `comment`
11. `last_sync_at`

### `database_columns`
用途：字段元数据。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `table_name`
5. `column_name`
6. `ordinal_position`
7. `data_type`
8. `is_nullable`
9. `default_value`
10. `comment`
11. `is_primary_key`
12. `is_sensitive`

### `database_indexes`
用途：索引元数据。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `table_name`
5. `index_name`
6. `index_type`
7. `columns`
8. `is_unique`
9. `cardinality`
10. `comment`

### `database_sync_jobs`
用途：元数据同步任务。

核心字段：

1. `id`
2. `instance_id`
3. `trigger_type`
4. `status`
5. `started_at`
6. `finished_at`
7. `duration_ms`
8. `schemas_count`
9. `tables_count`
10. `columns_count`
11. `error_message`

### `database_query_audits`
用途：SQL 查询和命令执行审计。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `operator_id`
5. `operator_name`
6. `sql_text`
7. `sql_fingerprint`
8. `sql_type`
9. `risk_level`
10. `status`
11. `rows_returned`
12. `rows_affected`
13. `duration_ms`
14. `error_message`
15. `client_ip`
16. `created_at`

### `database_backup_tasks`
用途：备份任务配置。

核心字段：

1. `id`
2. `instance_id`
3. `name`
4. `backup_type`
5. `schedule`
6. `storage_type`
7. `storage_config`
8. `retention_days`
9. `enabled`
10. `created_at`
11. `updated_at`

### `database_backup_records`
用途：备份执行记录。

核心字段：

1. `id`
2. `task_id`
3. `instance_id`
4. `status`
5. `file_path`
6. `file_size`
7. `started_at`
8. `finished_at`
9. `duration_ms`
10. `error_message`

### `database_metric_snapshots`
用途：实例性能和容量趋势快照。

核心字段：

1. `id`
2. `instance_id`
3. `metric_time`
4. `connections`
5. `qps`
6. `tps`
7. `slow_queries`
8. `cache_hit_rate`
9. `replication_lag_seconds`
10. `size_bytes`
11. `raw_metrics`

## 接口草案
### 实例管理
1. `GET /api/v1/databases/instances`
2. `POST /api/v1/databases/instances`
3. `GET /api/v1/databases/instances/{id}`
4. `PUT /api/v1/databases/instances/{id}`
5. `DELETE /api/v1/databases/instances/{id}`
6. `POST /api/v1/databases/instances/{id}/enable`
7. `POST /api/v1/databases/instances/{id}/disable`
8. `POST /api/v1/databases/instances/{id}/test`

### 元数据
1. `POST /api/v1/databases/instances/{id}/sync`
2. `GET /api/v1/databases/instances/{id}/sync-jobs`
3. `GET /api/v1/databases/instances/{id}/schemas`
4. `GET /api/v1/databases/instances/{id}/tables`
5. `GET /api/v1/databases/instances/{id}/tables/{table}/columns`
6. `GET /api/v1/databases/instances/{id}/tables/{table}/indexes`
7. `GET /api/v1/databases/instances/{id}/tables/{table}/ddl`

### 查询与数据浏览
1. `POST /api/v1/databases/instances/{id}/query`
2. `POST /api/v1/databases/instances/{id}/explain`
3. `POST /api/v1/databases/instances/{id}/export`
4. `GET /api/v1/databases/query-audits`
5. `GET /api/v1/databases/query-history`
6. `POST /api/v1/databases/sql/validate`

### 性能诊断
1. `GET /api/v1/databases/instances/{id}/metrics`
2. `GET /api/v1/databases/instances/{id}/slow-queries`
3. `GET /api/v1/databases/instances/{id}/sessions`
4. `GET /api/v1/databases/instances/{id}/locks`
5. `POST /api/v1/databases/instances/{id}/diagnosis-report`

### 备份恢复
1. `GET /api/v1/databases/backup-tasks`
2. `POST /api/v1/databases/backup-tasks`
3. `PUT /api/v1/databases/backup-tasks/{id}`
4. `DELETE /api/v1/databases/backup-tasks/{id}`
5. `POST /api/v1/databases/backup-tasks/{id}/run`
6. `GET /api/v1/databases/backup-records`
7. `GET /api/v1/databases/backup-records/{id}/download`
8. `POST /api/v1/databases/backup-records/{id}/restore-dry-run`

### 配置
1. `GET /api/v1/databases/settings`
2. `PUT /api/v1/databases/settings`
3. `GET /api/v1/databases/supported-types`

## 前端页面规划
### `DatabaseInstances.vue`
1. 实例列表。
2. 新增、编辑、删除、启停。
3. 连通性测试。
4. 手动同步。
5. 按类型、环境、状态、负责人、标签筛选。

### `DatabaseInstanceDetail.vue`
1. 基础信息。
2. 健康状态。
3. 容量和性能概览。
4. 拓扑信息。
5. 最近同步任务。
6. 最近审计日志。

### `DatabaseSchemas.vue`
1. 库和 Schema 列表。
2. 表、视图、字段、索引浏览。
3. 建表语句查看。
4. 数据字典导出。

### `DatabaseQueryConsole.vue`
1. SQL 编辑器。
2. 实例和库选择。
3. 执行、解释、格式化。
4. 结果表格。
5. 历史记录。
6. 收藏语句。

### `DatabasePerformance.vue`
1. 指标趋势。
2. 慢 SQL。
3. 活跃会话。
4. 锁等待。
5. 诊断报告。

### `DatabaseBackups.vue`
1. 备份任务列表。
2. 备份记录列表。
3. 手动备份。
4. 下载备份。
5. 恢复演练。

### `DatabaseAudit.vue`
1. 查询审计。
2. 导出审计。
3. 变更审计。
4. 备份恢复审计。
5. 按用户、实例、时间、风险等级筛选。

### `DatabaseSettings.vue`
1. 写操作总开关。
2. 最大查询超时。
3. 最大返回行数。
4. 最大导出行数。
5. 高风险 SQL 策略。
6. 审计保留天数。
7. 备份默认保留天数。

## 权限设计
建议新增权限点：

1. `database:instance:list`
2. `database:instance:create`
3. `database:instance:update`
4. `database:instance:delete`
5. `database:instance:test`
6. `database:metadata:sync`
7. `database:metadata:view`
8. `database:query:readonly`
9. `database:query:write`
10. `database:query:export`
11. `database:performance:view`
12. `database:backup:list`
13. `database:backup:create`
14. `database:backup:run`
15. `database:restore:dryrun`
16. `database:restore:execute`
17. `database:audit:view`
18. `database:settings:update`

权限边界：

1. 支持按数据库实例授权。
2. 后续支持按库、Schema、表授权。
3. 导出权限必须独立于查询权限。
4. 写操作权限必须独立于只读查询权限。
5. 恢复执行权限必须独立于备份权限。

## 风险等级
1. `low`：元数据查看、只读查询、执行计划。
2. `medium`：数据导出、手动备份、字段注释变更。
3. `high`：INSERT、UPDATE、DELETE、DDL、账号权限调整。
4. `critical`：DROP、TRUNCATE、批量 DELETE、生产恢复、账号删除。

## SQL 安全策略
1. 默认只读。
2. 默认禁止多语句执行。
3. 默认禁止无 WHERE 的 UPDATE / DELETE。
4. 默认禁止 DROP / TRUNCATE。
5. 默认限制 SELECT 最大返回行数。
6. 默认限制查询超时时间。
7. 默认限制导出最大行数。
8. 默认记录完整审计。
9. 高风险 SQL 必须二次确认。
10. critical 级别 SQL 预留审批能力。

## 敏感数据保护
1. 支持配置敏感字段规则，例如手机号、身份证、邮箱、密码、token。
2. 支持按字段名自动识别敏感字段。
3. 支持按数据样本正则识别敏感字段。
4. 页面展示默认脱敏。
5. 导出敏感字段需要独立权限。
6. 审计日志中的 SQL 参数和结果摘要需要脱敏。

## 监控指标建议
### MySQL / MariaDB
1. 连接数。
2. QPS。
3. TPS。
4. 慢查询数。
5. InnoDB Buffer Pool 命中率。
6. 主从延迟。
7. 死锁数。
8. 表空间大小。

### PostgreSQL
1. 连接数。
2. TPS。
3. 缓存命中率。
4. 慢查询。
5. 锁等待。
6. WAL 大小。
7. replication lag。
8. 表膨胀估算。

### Redis
1. used_memory。
2. connected_clients。
3. ops_per_sec。
4. keyspace_hits / keyspace_misses。
5. evicted_keys。
6. expired_keys。
7. replication lag。
8. cluster_state。

### MongoDB
1. connections。
2. opcounters。
3. replication lag。
4. lock percentage。
5. index size。
6. collection size。
7. slow operations。

### Elasticsearch / OpenSearch
1. cluster health。
2. node count。
3. shard count。
4. JVM heap。
5. indexing rate。
6. search latency。
7. disk watermark。

## 分期计划
### 一期：MVP 闭环
目标：先完成数据库资产纳管、元数据发现、只读查询和审计闭环。

范围：

1. 新增数据库管理菜单。
2. 数据库实例 CRUD。
3. 凭据绑定。
4. MySQL、PostgreSQL、Redis、MongoDB 连接测试。
5. MySQL、PostgreSQL 元数据同步。
6. Redis 基础信息同步。
7. MongoDB 库和集合同步。
8. 实例详情和结构浏览。
9. 只读 SQL 查询控制台。
10. 查询审计日志。
11. 基础 RBAC 权限点。

不包含：

1. 写操作。
2. 生产恢复。
3. 审批流。
4. 账号授权管理。
5. 自动修复。

验收标准：

1. 能新增至少 1 个 MySQL 实例并完成连通性测试。
2. 能同步 MySQL 库、表、字段、索引。
3. 能执行只读 SELECT 并展示分页结果。
4. 非只读 SQL 默认被拦截。
5. 每次查询都能在审计日志中检索到。
6. 无权限用户不能访问数据库实例和 SQL 控制台。

### 二期：结构、导出和性能诊断
目标：增强结构管理、导出、安全限制和基础性能诊断。

范围：

1. 表结构详情。
2. DDL 查看。
3. 数据字典导出。
4. 查询结果导出。
5. SQL 格式化和历史记录。
6. 执行计划。
7. 慢 SQL 列表。
8. 活跃会话。
9. 基础指标趋势。
10. SQL Server、Oracle、ClickHouse 初步支持。

验收标准：

1. 能查看表字段、索引和 DDL。
2. 能导出受限行数内的数据，并记录导出审计。
3. 能查看慢 SQL 和基础指标趋势。
4. 超过导出行数限制时被拦截。

### 三期：受控写操作和备份
目标：开放可控数据变更和备份任务。

范围：

1. 写操作总开关。
2. SQL 风险识别。
3. 高风险 SQL 二次确认。
4. 操作原因必填。
5. 最大影响行数限制。
6. 回滚 SQL 辅助生成。
7. 逻辑备份任务。
8. 备份记录和下载。
9. 备份失败告警。

验收标准：

1. 写操作总开关关闭时，所有写 SQL 都被拦截。
2. 开关开启后，具备权限且通过确认的写 SQL 可以执行。
3. 高风险 SQL 必须填写原因并记录审计。
4. 备份任务能按计划执行并生成备份记录。

### 四期：集群拓扑、恢复演练和国产数据库
目标：补齐复杂数据库类型、集群拓扑和恢复演练。

范围：

1. Redis Cluster 拓扑。
2. MongoDB ReplicaSet 拓扑。
3. Elasticsearch / OpenSearch 索引和分片视图。
4. TiDB、OceanBase、openGauss、达梦、人大金仓支持。
5. 恢复演练。
6. 容量趋势。
7. 巡检报告。

验收标准：

1. 能展示 Redis Cluster 节点和 slots。
2. 能展示 MongoDB 主从角色。
3. 能执行备份恢复演练到非生产实例。
4. 巡检报告包含容量、性能、安全和备份状态。

## 一到四期收尾优化清单
定位：以下内容属于一到四期已完成功能的安全收口、体验收口和工程质量优化，不进入五期“治理和自动化”范围。实施顺序按风险收益和依赖关系排列，优先补齐安全闭环，再处理可维护性和功能增强。

### 1. 实例权限变更审计
目标：让数据库实例对象级权限的新增、修改、删除可追溯。

状态：已实现。实例权限保存和删除会写入统一审计，动作分别为 `instance_permission_upsert` 和 `instance_permission_delete`，审计内容包含角色、实例、变更前权限和变更后权限。

改造范围：

1. 对 `database_instance_permissions` 的新增、修改和删除写入统一审计。
2. 审计内容应包含角色 ID、角色名称、实例 ID、实例名称、变更前权限、变更后权限、操作人和客户端 IP。
3. 新增审计动作建议：
   - `instance_permission_upsert`
   - `instance_permission_delete`
4. 审计展示应能在现有“查询审计”页按动作筛选，动作文案清晰可读。
5. 权限变更失败时不写成功审计；若后续需要记录失败，可单独补失败审计动作。

验收标准：

1. 新增或修改实例权限后，审计列表能看到权限保存记录。
2. 删除实例权限后，审计列表能看到权限删除记录。
3. 审计记录能定位到目标数据库实例、角色和权限位图变化。
4. 现有查询、备份、恢复、巡检审计不受影响。

### 2. 前端权限入口收口
目标：前端入口和后端权限保持一致，减少无权限用户看到不可用操作的情况。

状态：已实现。数据库管理页会加载后端 `ui-permissions` 能力结果，并按 RBAC 按钮权限隐藏实例权限页签和配置动作。

改造范围：

1. “实例权限”页签只对具备实例管理或权限配置能力的用户展示。
2. 添加、编辑、删除实例权限按钮按菜单权限隐藏或禁用。
3. 实例列表中的查询、写入、导出、备份、恢复、诊断、拓扑等按钮继续按对象级权限位图控制。
4. 权限加载失败时应给出明确提示，不出现空白页或按钮误开放。
5. 保留后端权限校验作为最终边界，前端只做体验收口。

验收标准：

1. 普通无管理权限用户看不到实例权限配置入口。
2. 有查看权限但无管理权限的用户不能从前端触发权限配置动作。
3. 后端直接请求仍按 RBAC 和对象级权限拦截。

### 3. SQL 和导出安全细化
目标：在现有只读查询、受控写入和导出审计基础上，继续降低数据泄露和资源耗尽风险。

状态：已实现。查询结果会统一做单元格安全预览、敏感字段脱敏和二进制预览；导出结果具备独立行数上限、单元格长度限制和总大小拦截，并在导出审计原因中记录行数、列数、截断和脱敏状态。

改造范围：

1. 查询结果增加单字段最大展示长度，超出部分截断并标记。
2. 导出结果增加最大单元格长度和总字节数保护。
3. 导出最大行数继续独立于查询最大行数配置。
4. 对二进制、超长文本、JSON 大字段做安全预览，避免前端渲染和浏览器内存压力。
5. 预留敏感字段脱敏规则入口，先支持按字段名匹配，例如 password、token、secret、phone、email。
6. 审计中记录导出行数、结果列数和截断状态，避免只知道“导出成功”。

验收标准：

1. 超长字段不会撑爆接口响应或前端表格。
2. 导出超过限制时被拦截并返回明确错误。
3. 查询和导出仍保留完整审计。

### 4. 备份和恢复链路增强
目标：在三期备份和四期恢复演练基础上，提升可靠性、可诊断性和误操作防护。

状态：已实现。成功备份记录会保存 SHA256 checksum；下载和恢复演练前会校验路径、文件大小和 checksum；备份运行态补充队列中、执行中、清理中；恢复演练锁收紧到目标实例级别，失败后可再次发起并保留每次记录。

改造范围：

1. 备份文件生成后记录 checksum，下载和恢复演练前校验文件完整性。
2. 恢复演练支持失败重试，并保留每次执行状态。
3. 备份任务运行状态补充队列中、执行中、清理中等更细状态。
4. 备份失败原因聚合展示，方便定位凭据、网络、命令缺失、磁盘空间等问题。
5. 恢复演练继续禁止生产目标实例，且同一目标实例同一时间只允许一个恢复任务。
6. 备份文件下载继续限制在配置的备份根目录内，禁止路径穿越和软链逃逸。

验收标准：

1. 成功备份记录包含可校验的 checksum。
2. 文件缺失、checksum 不一致时不能下载或恢复。
3. 恢复演练重试不会覆盖原始失败记录。
4. 备份失败原因能在页面和审计中定位。

### 5. 数据库管理页面组件拆分
目标：降低 `DatabaseManagement.vue` 的维护成本，为后续迭代提供清晰边界。

状态：进行中。已先拆出实例权限、查询审计、巡检报告和拓扑页签主体为独立组件，保留父组件中的数据加载、权限判断和提交逻辑，后续继续按页签小步拆分。

改造范围：

1. 按业务页签拆分组件：
   - 实例管理
   - 实例权限
   - 结构浏览
   - 查询控制台
   - 审计日志
   - 备份恢复
   - 诊断和巡检
2. 公共状态和工具函数抽到 composable 或独立 util，例如实例选择、权限位图、时间格式化、状态标签。
3. 拆分过程不改变接口协议和用户可见行为。
4. 每次拆分保持小步提交，避免和业务功能改造混在一起。

验收标准：

1. 页面构建通过，主要页签可正常切换。
2. 拆分后组件职责清晰，单文件体积明显下降。
3. 行为与拆分前保持一致。

### 6. Redis 专项增强
目标：把 Redis 从“可纳管、可测试、可拓扑、可备份”继续补齐到更完整的读侧运维闭环。

改造范围：

1. Redis Key 浏览按逻辑 DB、Key 类型、TTL、内存占用和编码展示。
2. Redis 命令控制台继续保持只读白名单，禁止写命令、高危命令和脚本执行。
3. Redis 诊断补充内存压力、命中率、慢日志、大 Key 风险、客户端连接和复制健康。
4. Redis 容量趋势改用 Redis 语义指标，例如 used_memory、dataset_memory、keys、expires、maxmemory。
5. Redis Cluster 场景下按 master 节点聚合扫描和容量统计，避免只看入口节点。
6. Redis 巡检报告输出内存、持久化、复制、Cluster、慢日志和大 Key 结论。

验收标准：

1. Redis 结构浏览不再套用关系型数据库表结构语义。
2. Redis 只读命令继续写入统一审计。
3. Redis 诊断和巡检能给出可操作的问题定位。

### 五期：治理和自动化
目标：形成数据库治理闭环。

范围：

1. 审批流。
2. 数据库账号生命周期管理。
3. 密码轮换。
4. 敏感数据发现。
5. 数据血缘和影响分析。
6. Schema Diff。
7. SQL 优化建议。
8. 自动巡检和风险评分。
9. 工单系统联动。

验收标准：

1. 高风险变更可走审批。
2. 敏感字段能被识别并脱敏。
3. 能输出实例风险评分。
4. 能生成 SQL 优化建议和巡检报告。

## 一期优先开发顺序
1. 新增数据库模块菜单、路由和空页面。
2. 新增基础表结构和迁移。
3. 实现数据库实例 CRUD。
4. 接入凭据模块。
5. 实现 MySQL Adapter 连接测试。
6. 实现 MySQL 元数据同步。
7. 实现实例列表和实例详情。
8. 实现结构浏览。
9. 实现只读 SQL 控制台。
10. 实现查询审计。
11. 实现 PostgreSQL Adapter。
12. 实现 Redis 和 MongoDB 基础发现。
13. 补 RBAC 和菜单权限。
14. 补 Swagger 和验收文档。

## 测试策略
1. Adapter 单元测试：连接参数、SQL 分类、元数据解析。
2. Usecase 单元测试：权限、风险策略、审计状态流转。
3. API 测试：实例 CRUD、连接测试、同步、查询。
4. 前端构建测试：`npm run build`。
5. 后端测试：`go test ./...`。
6. 集成测试：使用 Docker Compose 启动 MySQL、PostgreSQL、Redis、MongoDB。
7. 安全测试：验证无 WHERE 更新、DROP、TRUNCATE、多语句、超时和超行数限制。

## 关键风险与处理
1. 多数据库驱动依赖膨胀：按阶段引入驱动，避免一期一次性接入所有类型。
2. SQL 解析复杂：一期先做保守规则，后续引入成熟 SQL parser。
3. 权限粒度复杂：一期按实例授权，后续按库表授权。
4. 写操作误伤：默认关闭写操作，高危 SQL 必须二次确认和审计。
5. 大查询拖垮数据库：强制超时、行数限制和只读事务。
6. 备份文件占用磁盘：保留策略、容量告警和外部存储配置。
7. 敏感数据泄露：脱敏、导出独立授权、审计脱敏。
8. 国产数据库兼容性不一致：优先兼容常用协议，再做专有能力。

## 待确认问题
1. 一期是否只开放 MySQL 和 PostgreSQL，还是同时纳入 Redis、MongoDB 基础发现。
2. 数据库实例是否统一归属 `资产管理`，还是后续提升为独立一级菜单。
3. 是否需要接入现有审批或工单系统。
4. 备份文件默认存储在本机、对象存储，还是由用户配置。
5. SQL 控制台是否允许跨库查询。
6. 是否需要支持堡垒机式数据库代理审计。
7. 是否需要支持生产、预发、测试环境的不同安全策略。
