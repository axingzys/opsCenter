# 数据库管理三期：表结构与接口草案

## 1. 统一审计扩展
### `database_query_audits`
三期继续沿用现有统一审计表，新增字段建议：

1. `audit_action`
2. `reason`
3. `confirm_required`
4. `confirmed`
5. `rows_affected_limit`
6. `rollback_sql`

说明：

1. 一期、二期已有的查询、导出、诊断动作继续保留。
2. 三期新增写操作、备份动作继续写入同一张表，前端按 `audit_action` 区分。

## 2. 备份任务表
### `database_backup_tasks`
字段建议：

1. `instance_id`
2. `name`
3. `backup_type`
4. `schedule`
5. `storage_type`
6. `storage_config`
7. `retention_days`
8. `enabled`
9. `last_run_at`
10. `last_status`
11. `last_message`

建议约束：

1. `backup_type` 先支持 `logical`
2. `storage_type` 三期先支持 `local`
3. `schedule` 允许为空，表示仅手动执行

## 3. 备份记录表
### `database_backup_records`
字段建议：

1. `task_id`
2. `instance_id`
3. `trigger_type`
4. `backup_type`
5. `storage_type`
6. `status`
7. `file_path`
8. `file_name`
9. `file_size`
10. `started_at`
11. `finished_at`
12. `duration_ms`
13. `error_message`

## 4. 数据库三期配置
建议继续复用 `sys_config`，新增键：

1. `database_write_enabled`
2. `database_high_risk_requires_confirm`
3. `database_operation_reason_required`
4. `database_max_affected_rows`
5. `database_default_backup_retention_days`
6. `database_backup_storage_path`

## 5. 写操作预检查接口
### `POST /api/v1/databases/instances/{id}/query/write/validate`
请求：
```json
{
  "schemaName": "app",
  "sqlText": "update users set status = 'disabled' where id = 10"
}
```

返回：
```json
{
  "instanceId": 12,
  "schemaName": "app",
  "sqlType": "UPDATE",
  "riskLevel": "high",
  "allowed": true,
  "confirmRequired": true,
  "reasonRequired": true,
  "rowsAffectedLimit": 1000,
  "message": "通过写操作预检查"
}
```

## 6. 写操作执行接口
### `POST /api/v1/databases/instances/{id}/query/write`
请求：
```json
{
  "schemaName": "app",
  "sqlText": "delete from users where id = 10",
  "reason": "清理无效测试数据",
  "confirmed": true
}
```

返回：
```json
{
  "auditId": 2201,
  "instanceId": 12,
  "schemaName": "app",
  "sqlType": "DELETE",
  "riskLevel": "high",
  "rowsAffected": 1,
  "durationMs": 45,
  "rollbackSql": "insert into users (...) values (...)",
  "executedAt": "2026-04-24 17:30:00",
  "message": "执行成功"
}
```

## 7. 数据库三期设置接口
### `GET /api/v1/system/config/database`

返回：
```json
{
  "writeEnabled": false,
  "highRiskRequiresConfirm": true,
  "operationReasonRequired": true,
  "maxAffectedRows": 1000,
  "defaultBackupRetentionDays": 7,
  "backupStoragePath": "./data/database-backups"
}
```

### `PUT /api/v1/system/config/database`
请求：
```json
{
  "writeEnabled": false,
  "highRiskRequiresConfirm": true,
  "operationReasonRequired": true,
  "maxAffectedRows": 1000,
  "defaultBackupRetentionDays": 7,
  "backupStoragePath": "./data/database-backups"
}
```

## 8. 备份任务接口
### `GET /api/v1/databases/backup-tasks`
筛选参数：

1. `instanceId`
2. `keyword`
3. `enabled`

### `POST /api/v1/databases/backup-tasks`
请求：
```json
{
  "instanceId": 12,
  "name": "app-db-nightly",
  "backupType": "logical",
  "schedule": "0 2 * * *",
  "storageType": "local",
  "retentionDays": 7,
  "enabled": true
}
```

### `PUT /api/v1/databases/backup-tasks/{id}`
### `DELETE /api/v1/databases/backup-tasks/{id}`
### `POST /api/v1/databases/backup-tasks/{id}/run`

## 9. 备份记录接口
### `GET /api/v1/databases/backup-records`
筛选参数：

1. `taskId`
2. `instanceId`
3. `status`
4. `triggerType`
5. `dateFrom`
6. `dateTo`

### `GET /api/v1/databases/backup-records/{id}/download`

返回头建议：

1. `Content-Disposition`
2. `Content-Type: application/octet-stream`

## 10. 审计动作筛选补充
查询审计继续使用现有接口：

### `GET /api/v1/databases/query-audits`
三期新增筛选值：

1. `change_execute`
2. `backup_run`
3. `backup_download`
