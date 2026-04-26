# 数据库管理一期：后端实施细化

## 包结构
建议新增：

1. `internal/biz/database/model.go`
2. `internal/biz/database/repository.go`
3. `internal/biz/database/usecase.go`
4. `internal/biz/database/adapter.go`
5. `internal/biz/database/adapter_sql.go`
6. `internal/biz/database/sql_safety.go`
7. `internal/service/database/http.go`
8. `internal/server/database/http.go`
9. `web/src/api/database.ts`
10. `web/src/views/asset/DatabaseManagement.vue`

## 一期后端能力
### 实例管理
1. 创建实例。
2. 更新实例。
3. 删除实例。
4. 查询实例详情。
5. 查询实例列表。
6. 启用实例。
7. 禁用实例。

### 支持类型
1. 返回类型编码。
2. 返回显示名称。
3. 返回默认端口。
4. 返回一期能力标记。

### 连接测试
1. MySQL / MariaDB：使用 Go SQL driver 连接并查询版本。
2. PostgreSQL：使用 pgx 或兼容 driver 连接并查询版本。
3. Redis / MongoDB：一期先保留适配器占位，接入驱动后补齐。

### 元数据同步
1. MySQL / MariaDB 查询 `information_schema`。
2. PostgreSQL 查询 `information_schema` 和 `pg_catalog`。
3. 同步过程写入 `database_sync_jobs`。
4. 同步失败写入错误信息。

### 查询控制台
1. 校验 SQL 是否只读。
2. 拒绝多语句。
3. 增加 LIMIT 保护。
4. 执行查询。
5. 返回列名、行数据、耗时和截断标记。
6. 写入 `database_query_audits`。

## SQL 安全规则
1. 去除注释和空白后判断首个关键字。
2. 仅允许 `SELECT`、`SHOW`、`DESC`、`DESCRIBE`、`EXPLAIN`、`WITH`。
3. 禁止出现 `INSERT`、`UPDATE`、`DELETE`、`DROP`、`TRUNCATE`、`ALTER`、`CREATE`、`GRANT`、`REVOKE`、`REPLACE`、`CALL`、`EXEC`、`MERGE`、`LOAD`、`LOCK`、`UNLOCK`、`SET`、`USE`。
4. 禁止多语句分号。
5. 限制 SQL 长度。
6. 查询超时使用 context 控制。

## 错误处理
1. 参数错误返回明确中文提示。
2. 凭据不存在返回 `凭据不存在`。
3. 实例不存在返回 `数据库实例不存在`。
4. 实例禁用时禁止测试、同步和查询。
5. SQL 不安全返回 `仅允许执行只读查询`。

## 审计要求
1. 查询发起前创建审计记录。
2. 执行成功更新状态为 success。
3. 执行失败更新状态为 failed。
4. 被安全策略拦截更新状态为 denied。
5. 记录操作者、实例、库、SQL、风险等级、耗时、返回行数和错误信息。

## 第 1 批落地范围
为了快速启动一期，第 1 批先完成：

1. 数据模型和自动迁移。
2. 实例 CRUD。
3. 支持类型接口。
4. 后端路由注册。
5. 前端实例列表和新增编辑表单。

## 当前落地状态
1. 已新增 `internal/biz/database`、`internal/data/database`、`internal/service/database`、`internal/server/database`。
2. 已接入 AutoMigrate，包含实例、Schema、表、字段、索引、同步任务和查询审计模型。
3. 已实现实例 CRUD、启用、禁用、支持类型列表。
4. 已实现 MySQL / MariaDB 连接测试，返回版本并更新最近测试时间。
5. 已实现 MySQL / MariaDB 元数据同步，采集 `information_schema` 中的 Schema、表、字段、索引。
6. 已实现元数据原子替换写入和同步任务状态记录。
7. 已实现结构浏览接口：Schema、表、字段、索引查询。
8. 已实现 MySQL / MariaDB 只读 SQL 查询，默认最大 500 行、最长 30 秒。
9. 已实现 SQL 安全校验，包含允许关键字、禁止关键字、单语句和自动 LIMIT。
10. 已实现查询审计写入，成功、失败和安全拦截都会记录状态。
11. 已实现查询审计分页列表，支持实例、状态、风险、SQL 类型、时间和关键字筛选。
12. 已实现查询审计 CSV 导出，复用审计筛选条件，默认最多导出 5000 条。
13. 已实现数据库管理按钮级 RBAC 权限点，接口不再只依赖管理员硬限制，admin 角色自动放行。
14. 已实现 PostgreSQL 连接测试、Schema / 表 / 字段 / 索引元数据同步和只读 SQL 查询适配。
15. Redis、MongoDB 仍为后续适配器接入项。
