# 数据库管理一期：RBAC、审计导出与 PostgreSQL 适配

## 本批范围
1. 将数据库管理接口从管理员硬限制细化为按钮级 RBAC 权限。
2. 查询审计支持按筛选条件导出 CSV。
3. PostgreSQL 接入连接测试、元数据同步和只读 SQL 查询。
4. 调整新增 / 编辑实例弹窗端口布局。

## RBAC 权限点
菜单入口仍使用 `asset_databases` 控制导航可见性。

按钮权限隐藏在数据库管理菜单下，角色授权树可勾选：

1. `database:instance:view`：查看支持类型、实例列表、实例详情。
2. `database:instance:create`：新增实例。
3. `database:instance:update`：编辑实例。
4. `database:instance:delete`：删除实例。
5. `database:instance:status`：启用、禁用实例。
6. `database:connection:test`：连接测试。
7. `database:metadata:view`：查看 Schema、表、字段、索引。
8. `database:metadata:sync`：同步元数据。
9. `database:query:execute`：执行只读 SQL。
10. `database:audit:view`：查看查询审计。
11. `database:audit:export`：导出查询审计。

## 接口权限映射
1. `GET /api/v1/databases/supported-types`：`database:instance:view`
2. `GET /api/v1/databases/instances`：`database:instance:view`
3. `POST /api/v1/databases/instances`：`database:instance:create`
4. `GET /api/v1/databases/instances/{id}`：`database:instance:view`
5. `PUT /api/v1/databases/instances/{id}`：`database:instance:update`
6. `DELETE /api/v1/databases/instances/{id}`：`database:instance:delete`
7. `POST /api/v1/databases/instances/{id}/enable`：`database:instance:status`
8. `POST /api/v1/databases/instances/{id}/disable`：`database:instance:status`
9. `POST /api/v1/databases/instances/{id}/test`：`database:connection:test`
10. `POST /api/v1/databases/instances/{id}/sync-metadata`：`database:metadata:sync`
11. `GET /api/v1/databases/instances/{id}/schemas`：`database:metadata:view`
12. `GET /api/v1/databases/instances/{id}/tables`：`database:metadata:view`
13. `GET /api/v1/databases/instances/{id}/columns`：`database:metadata:view`
14. `GET /api/v1/databases/instances/{id}/indexes`：`database:metadata:view`
15. `POST /api/v1/databases/instances/{id}/query`：`database:query:execute`
16. `GET /api/v1/databases/query-audits`：`database:audit:view`
17. `GET /api/v1/databases/query-audits/export`：`database:audit:export`

admin 角色自动放行，并在服务启动时自动补齐数据库管理菜单和按钮权限授权。

## 查询审计导出
导出接口：`GET /api/v1/databases/query-audits/export`

导出规则：

1. 复用审计列表筛选条件。
2. CSV 使用 UTF-8 BOM，便于 Excel 打开中文。
3. 默认最多导出 5000 条。
4. 对以 `=`、`+`、`-`、`@` 开头的文本字段做 CSV 注入防护。

## PostgreSQL 适配
驱动：`github.com/jackc/pgx/v5/stdlib`

能力：

1. 连接测试：`SELECT version()`。
2. Schema 同步：读取 `pg_namespace`、`pg_database`、`pg_class`。
3. 表同步：读取普通表、分区表、视图、物化视图、外部表。
4. 字段同步：读取 `information_schema.columns`，识别主键和敏感字段。
5. 索引同步：读取 `pg_index`、`pg_class`、`pg_namespace`、`pg_am`。
6. 查询控制台：复用只读 SQL 校验，执行前按选择的 Schema 设置 `search_path`。

限制：

1. PostgreSQL 元数据同步范围为当前连接数据库内的 Schema。
2. 默认库为空时使用凭据用户名作为连接数据库名。
3. `DESC` / `DESCRIBE` 在 PostgreSQL 中不是标准 SQL，接口允许通过但数据库可能返回语法错误。
