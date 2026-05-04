# 数据库结构浏览表关系增强方案

## 背景

当前数据库管理的“结构浏览”已经支持 Schema、表、字段、索引、DDL 预览和数据字典导出，但缺少表与表之间的关联关系展示。实际运维中，排查业务数据、编写查询 SQL、评估 DDL 影响、处理误删恢复时，通常需要快速知道：

1. 当前表引用了哪些表。
2. 哪些表引用了当前表。
3. 关联字段是什么。
4. 关系来自数据库真实约束，还是 OpsHub 根据命名和唯一索引推断出来。

因此建议在结构浏览中增加“表关系”能力，让结构浏览从“单表结构查看”扩展为“局部业务对象关系查看”。

## 现有实现预览

现有元数据模型：

1. `database_schemas`：Schema / 数据库元数据。
2. `database_tables`：表和视图元数据。
3. `database_columns`：字段元数据。
4. `database_indexes`：索引元数据。

现有后端链路：

1. `SyncMetadata` 通过 `collectSQLMetadata` 分发不同数据库类型的元数据采集。
2. MySQL / PostgreSQL / SQL Server / Oracle / ClickHouse 分别采集 Schema、表、字段、索引。
3. `metadataRepo.ReplaceAll` 按实例删除旧元数据，再批量写入新元数据。
4. 结构浏览接口包括 `schemas`、`tables`、`columns`、`indexes`、`ddl`、`dictionary/export`。

现有前端链路：

1. “结构浏览”左侧选择实例和 Schema。
2. 中间表格展示表 / 视图。
3. 右侧详情展示字段和索引。
4. 没有表关系 tab、关系图、JOIN 辅助或被引用提示。

## 目标

第一阶段和第二阶段目标：

1. 新增表关系元数据模型。
2. 同步元数据时采集数据库真实外键。
3. 在没有外键的业务库中，根据字段命名、主键和唯一索引推断关系。
4. 结构浏览右侧新增“关系”页签。
5. 支持查看当前表的出向关系和入向关系。
6. 支持从关系记录跳转到目标表。
7. 支持复制基础 JOIN 片段。
8. 明确区分“真实外键”和“推断关系”，避免误导。

第三阶段目标：

1. 在已有关系识别基础上，补齐“当前关系对变更、删除、恢复的影响说明”。
2. 为每条关系生成可复制的 SQL 辅助片段，但不自动执行。
3. 支持查看关系详情，包括字段、规则、可信度、ON UPDATE / ON DELETE、影响等级和检查 SQL。
4. 让结构浏览不仅能“看见关系”，还能辅助判断“这条关系下一步该怎么查”。

## 非目标

第一阶段和第二阶段暂不做：

1. 不做跨实例关系。
2. 不做复杂 ER 建模编辑。
3. 不做人工维护关系。
4. 不做自动 DDL 变更或外键创建。
5. 不对 Redis、MongoDB、Elasticsearch / OpenSearch 做关系推断。
6. 不把推断关系用于安全控制或自动恢复决策。

第三阶段仍然不做：

1. 不自动执行检查 SQL。
2. 不基于推断关系生成或执行 DDL。
3. 不提供在线编辑、确认、删除推断关系。
4. 不把影响等级作为拦截策略，只作为结构浏览提示。

## 数据模型

新增 `database_table_relations`。

核心字段：

1. `instance_id`：数据库实例 ID。
2. `schema_name`：来源表 Schema。
3. `table_name`：来源表名。
4. `column_name`：来源字段名。
5. `referenced_schema_name`：目标表 Schema。
6. `referenced_table_name`：目标表名。
7. `referenced_column_name`：目标字段名。
8. `constraint_name`：真实外键约束名，推断关系为空或生成稳定名称。
9. `relation_type`：`foreign_key` / `inferred`。
10. `relation_source`：`database_constraint` / `naming_rule` / `unique_index`。
11. `confidence`：可信度，真实外键固定 100，推断关系根据匹配质量给 60-90。
12. `on_update`：真实外键 ON UPDATE 行为。
13. `on_delete`：真实外键 ON DELETE 行为。
14. `cardinality`：`many_to_one` / `one_to_one` / `unknown`。
15. `comment`：解释说明。
16. `last_sync_at`：最近同步时间。

唯一键实现：

```text
instance_id + relation_key
```

说明：

1. `relation_key` 由来源表、来源字段、目标表、目标字段、关系类型和关系来源计算 SHA-256。
2. 不直接把所有长字段放入唯一索引，避免 MySQL utf8mb4 下超过 3072 bytes 索引长度。
3. `schema_name/table_name` 和 `referenced_schema_name/referenced_table_name` 分别保留普通索引，便于查询出向和入向关系。

## 第一阶段：真实外键关系

### 采集范围

MySQL / MariaDB / TiDB / OceanBase：

1. 从 `information_schema.KEY_COLUMN_USAGE` 读取 `REFERENCED_TABLE_SCHEMA`、`REFERENCED_TABLE_NAME`、`REFERENCED_COLUMN_NAME`。
2. 关联 `information_schema.REFERENTIAL_CONSTRAINTS` 获取 `UPDATE_RULE` 和 `DELETE_RULE`。

PostgreSQL / openGauss / Kingbase：

1. 从 `pg_constraint` 读取 `contype='f'` 的外键。
2. 通过 `conkey`、`confkey` 和 `pg_attribute` 展开字段。
3. 读取 `confupdtype`、`confdeltype` 映射 ON UPDATE / ON DELETE 行为。

SQL Server：

1. 从 `sys.foreign_keys`、`sys.foreign_key_columns`、`sys.tables`、`sys.schemas`、`sys.columns` 采集。

Oracle：

1. 从 `ALL_CONSTRAINTS`、`ALL_CONS_COLUMNS` 采集 `R` 类型约束。
2. 关联主键 / 唯一约束找到目标表和目标字段。

ClickHouse：

1. 第一阶段不支持外键，返回空关系。

### 后端接口

新增：

```http
GET /api/v1/databases/instances/:id/table-relations
```

参数：

1. `schemaName`：可选，限定 Schema。
2. `tableName`：可选，限定当前表。
3. `direction`：`all` / `outgoing` / `incoming`，默认 `all`。
4. `source`：`all` / `foreign_key` / `inferred`，默认 `all`。
5. `includeInferred`：是否包含推断关系，默认 `true`。

返回：

```json
{
  "relations": [],
  "nodes": [],
  "links": [],
  "summary": {
    "total": 0,
    "foreignKey": 0,
    "inferred": 0
  }
}
```

## 第二阶段：推断关系

很多生产库为了写入性能、分库分表、历史兼容或 ORM 习惯，不创建数据库外键。只采集真实外键会导致关系图经常为空，所以第二阶段加入推断关系。

### 推断规则

候选字段：

1. 字段名等于 `xxx_id`。
2. 字段名等于 `xxxId`。
3. 字段名等于 `xxxID`。
4. 字段名等于 `xxx_uuid`。
5. 排除明显不是关系的字段，例如 `trace_id`、`request_id`、`session_id`、`batch_id`。

候选目标表：

1. 与 `xxx` 同名的表。
2. `xxx` 的复数形式，例如 `user_id -> users.id`。
3. 去掉常见前缀后匹配，例如 `sys_user_id -> sys_users.id`。
4. 同一 Schema 优先，不跨 Schema 时可信度更高。

候选目标字段：

1. `id`。
2. `uuid`。
3. 与候选字段相同的字段。

可信度：

1. 真实外键：100。
2. 同 Schema，`xxx_id -> xxx.id`，目标字段为主键：90。
3. 同 Schema，`xxx_id -> xxx.id`，目标字段为唯一索引：85。
4. 同 Schema，`xxx_id -> xxxs.id`，目标字段为主键或唯一索引：80。
5. 跨 Schema 命中主键或唯一索引：70。
6. 仅命名命中但目标字段不是主键 / 唯一索引：不生成。

去重规则：

1. 如果真实外键已经存在同一组字段关系，不再生成推断关系。
2. 如果多个目标表命中，只保留可信度最高的一条。
3. 同可信度冲突时不生成，避免误导。

### UI 标识

1. 真实外键显示为 `外键`，使用实线。
2. 推断关系显示为 `推断`，使用虚线。
3. 低于 80 的推断关系使用普通提示，不做高亮。
4. 关系说明明确展示 `基于字段命名和唯一索引推断`。

## 前端设计

结构浏览右侧详情增加第三个页签：

1. `字段`
2. `索引`
3. `关系`

关系页签展示：

1. 方向：`引用外部表` / `被其他表引用`。
2. 来源表字段。
3. 目标表字段。
4. 来源：外键 / 推断。
5. 可信度。
6. ON UPDATE / ON DELETE。
7. 操作：跳转表、复制 JOIN。

表详情卡片增加：

1. 出向关系数。
2. 入向关系数。
3. 真实外键数。
4. 推断关系数。

关系图：

1. 当前表为中心节点。
2. 出向关系放右侧。
3. 入向关系放左侧。
4. 实线表示真实外键。
5. 虚线表示推断关系。
6. 点击节点切换选中表。

第一阶段和第二阶段可以先实现“关系列表 + 简单图”，后续再优化布局和复杂交互。

## 第三阶段：影响分析与 SQL 辅助

### 设计动机

第一阶段和第二阶段解决了“有没有关系”和“关系怎么连”的问题，但实际使用时还会遇到三个问题：

1. 做误删恢复时，需要知道当前表是否被其他表引用，以及应该先查哪些子表。
2. 做 DDL 或字段清理时，需要快速检查是否存在孤儿记录或依赖记录。
3. 编写查询 SQL 时，复制一个裸 JOIN 片段还不够，需要更具体的检查 SQL 模板。

第三阶段建议把关系页签从“展示型信息”增强为“只读分析型信息”，核心是详情面板和 SQL 辅助。

### 后端增强

`DatabaseTableRelationVO` 增加字段：

1. `reverseJoinSql`：反向 JOIN 片段，用于从目标表回查来源表。
2. `orphanCheckSql`：孤儿记录检查 SQL，用于查来源表中找不到目标表的记录。
3. `dependencyCheckSql`：依赖数量检查 SQL，用于估算当前关系下的依赖行数。
4. `impactLevel`：`high` / `warning` / `info`。
5. `impactText`：面向操作者的影响说明。

SQL 生成规则：

1. MySQL / MariaDB / TiDB / OceanBase 使用反引号。
2. PostgreSQL / openGauss / Kingbase / Oracle 使用双引号。
3. SQL Server 使用方括号。
4. JOIN SQL 使用固定别名，避免 `schema.table.column` 在不同数据库中的兼容问题：

```sql
LEFT JOIN <target_table> ref ON src.<source_column> = ref.<target_column>
```

5. 孤儿检查 SQL 示例：

```sql
SELECT src.*
FROM <source_table> src
LEFT JOIN <target_table> ref ON src.<source_column> = ref.<target_column>
WHERE src.<source_column> IS NOT NULL
  AND ref.<target_column> IS NULL
LIMIT 100
```

6. SQL Server 使用 `TOP (100)`，Oracle 使用 `FETCH FIRST 100 ROWS ONLY`。

影响等级建议：

1. 真实外键 + 当前表被引用：`high`，删除或修改当前表主键 / 唯一键前必须先检查子表依赖。
2. 真实外键 + 当前表引用外部表：`warning`，写入或变更来源字段前应避免孤儿记录。
3. 推断关系 + 可信度大于等于 80：`warning`，建议人工确认后用于排查。
4. 推断关系 + 可信度低于 80：`info`，仅作为线索。

### 前端增强

关系页签增加：

1. “影响”列：展示 `impactLevel` 和 `impactText` 摘要。
2. “详情”操作：打开关系详情弹窗。
3. 详情弹窗展示：
   - 来源字段。
   - 目标字段。
   - 关系类型和来源。
   - 可信度。
   - ON UPDATE / ON DELETE。
   - 影响说明。
   - JOIN 片段。
   - 反向 JOIN 片段。
   - 孤儿检查 SQL。
   - 依赖数量检查 SQL。
4. 每段 SQL 都提供复制按钮。

### 用户使用路径

1. 在结构浏览中选中一张表。
2. 进入“关系”页签。
3. 根据“影响”列判断当前表是依赖别人，还是被别人依赖。
4. 点“详情”查看检查 SQL。
5. 复制 SQL 到 SQL 控制台，人工确认后再执行。

### 验收标准

1. 关系接口返回新增的 SQL 辅助字段和影响字段。
2. 关系表格能展示影响等级。
3. 点击详情能看到完整关系信息和 SQL 模板。
4. SQL 模板按数据库类型正确引用标识符。
5. 前端复制 JOIN、孤儿检查、依赖检查均可用。
6. 所有 SQL 只展示和复制，不自动执行。
7. 后端测试、前端类型检查和生产构建通过。

## 安全与权限

1. 复用 `database:metadata:view` 菜单权限。
2. 复用数据库实例对象权限 `DatabasePermissionView`。
3. 关系查询只读，不写审计。
4. 同步元数据仍需要 `database:metadata:sync` 和实例 Manage 权限。
5. 推断关系不参与权限判断、不参与自动化动作。

## 验收标准

1. 同步 MySQL / PostgreSQL 带外键的库后，结构浏览能看到真实外键关系。
2. 没有外键但存在 `user_id -> users.id` 这类命名时，能看到推断关系。
3. UI 能区分外键和推断关系。
4. 选中表后能看到出向和入向关系。
5. 点击关系目标表能跳转到目标表。
6. 复制 JOIN 能生成基础 SQL 片段。
7. Redis 实例不显示关系页签或显示空状态，不报错。
8. `go test ./...`、前端类型检查和生产构建通过。
