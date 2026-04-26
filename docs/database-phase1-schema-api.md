# 数据库管理一期：表结构与接口草案

## 表结构
### `database_instances`
用途：保存数据库实例的资产信息和连接配置。

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
11. `connection_params`
12. `status`
13. `environment`
14. `business_system`
15. `owner`
16. `tags`
17. `remark`
18. `last_test_at`
19. `last_sync_at`
20. `created_at`
21. `updated_at`
22. `deleted_at`

### `database_schemas`
用途：保存库或 Schema 元数据。

核心字段：

1. `id`
2. `instance_id`
3. `schema_name`
4. `charset`
5. `collation`
6. `size_bytes`
7. `table_count`
8. `last_sync_at`

### `database_tables`
用途：保存表和视图元数据。

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
用途：保存字段元数据。

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
10. `column_key`
11. `comment`
12. `is_sensitive`

### `database_indexes`
用途：保存索引元数据。

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
用途：记录元数据同步任务。

核心字段：

1. `id`
2. `instance_id`
3. `trigger_type`
4. `status`
5. `message`
6. `started_at`
7. `finished_at`
8. `duration_ms`
9. `schemas_count`
10. `tables_count`
11. `columns_count`
12. `indexes_count`

### `database_query_audits`
用途：记录 SQL 查询和命令执行审计。

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

## 接口
### 支持类型
`GET /api/v1/databases/supported-types`

返回支持的数据库类型、默认端口、一期能力标记。

### 实例管理
`GET /api/v1/databases/instances`

查询参数：

1. `page`
2. `pageSize`
3. `keyword`
4. `dbType`
5. `status`
6. `environment`

`POST /api/v1/databases/instances`

创建数据库实例。

`GET /api/v1/databases/instances/{id}`

获取实例详情。

`PUT /api/v1/databases/instances/{id}`

更新实例。

`DELETE /api/v1/databases/instances/{id}`

删除实例。

`POST /api/v1/databases/instances/{id}/enable`

启用实例。

`POST /api/v1/databases/instances/{id}/disable`

禁用实例。

`POST /api/v1/databases/instances/{id}/test`

连接测试。

### 元数据同步
`POST /api/v1/databases/instances/{id}/sync-metadata`

触发元数据同步。

`GET /api/v1/databases/instances/{id}/sync-jobs`

查询同步任务，后续补齐列表接口；当前同步任务已经写入 `database_sync_jobs`。

`GET /api/v1/databases/instances/{id}/schemas`

查询 Schema。

`GET /api/v1/databases/instances/{id}/tables`

查询表，支持 `schemaName` 参数。

`GET /api/v1/databases/instances/{id}/columns`

查询字段，支持 `schemaName` 和 `tableName` 参数。

`GET /api/v1/databases/instances/{id}/indexes`

查询索引，支持 `schemaName` 和 `tableName` 参数。

### 查询控制台
`POST /api/v1/databases/instances/{id}/query`

执行只读 SQL 查询。请求体：

```json
{
  "schemaName": "opshub",
  "sqlText": "SELECT * FROM database_instances LIMIT 100",
  "limit": 500,
  "timeoutSeconds": 30
}
```

当前允许 `SELECT`、`SHOW`、`DESC`、`DESCRIBE`、`EXPLAIN`、`WITH`，禁止多语句和写操作。`EXPLAIN` 直接通过该接口执行。

`POST /api/v1/databases/instances/{id}/explain`

预留独立 explain 接口，当前未单独实现。

### 审计
`GET /api/v1/databases/query-audits`

查询 SQL 审计日志。支持查询参数：

1. `page`
2. `pageSize`
3. `keyword`
4. `instanceId`
5. `status`
6. `riskLevel`
7. `sqlType`
8. `startTime`
9. `endTime`

`GET /api/v1/databases/query-audits/export`

按相同筛选条件导出 SQL 审计 CSV。导出字段：

1. 审计ID
2. 执行时间
3. 实例
4. Schema
5. 操作者
6. SQL类型
7. 风险
8. 状态
9. 返回行
10. 耗时ms
11. 客户端IP
12. SQL
13. 错误信息
14. SQL指纹

## RBAC 权限点
数据库管理菜单编码：`asset_databases`

按钮级权限：

1. `database:instance:view`：查看支持类型、实例列表、实例详情。
2. `database:instance:create`：新增数据库实例。
3. `database:instance:update`：编辑数据库实例。
4. `database:instance:delete`：删除数据库实例。
5. `database:instance:status`：启用、禁用数据库实例。
6. `database:connection:test`：执行连接测试。
7. `database:metadata:view`：查看 Schema、表、字段、索引。
8. `database:metadata:sync`：触发元数据同步。
9. `database:query:execute`：执行只读 SQL 查询。
10. `database:audit:view`：查看 SQL 查询审计。
11. `database:audit:export`：导出 SQL 查询审计 CSV。
