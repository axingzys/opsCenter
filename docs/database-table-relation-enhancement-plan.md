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

第四阶段目标：

1. 把当前表关系从右侧详情页签中独立出来，放到结构浏览底部全宽区域。
2. 让关系列表拥有足够横向空间，减少来源字段、目标字段、规则、影响说明被截断的情况。
3. 保留当前的关系统计、过滤、跳转、JOIN 复制和详情弹窗能力。
4. 不改后端接口和数据模型，仅优化前端信息架构与展示密度。

第五阶段目标：

1. 优化结构浏览上方三栏高度，让“库 / Schema”“表 / 视图”“字段 / 索引”在桌面端形成稳定、统一的工作区。
2. 修复“表 / 视图”固定表格高度导致面板底部出现大块空白的问题。
3. 让中间表格和右侧字段 / 索引表格在面板内自适应剩余高度，表多时内部滚动，表少时视觉上仍然填满工作区。
4. 保持底部“表关系”面板位置不变，不改变关系数据、过滤、跳转和详情逻辑。

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

第四阶段仍然不做：

1. 不新增 schema 级全局关系接口。
2. 不做跨实例关系图。
3. 不做关系编辑、人工维护或删除。
4. 不改变现有关系采集、推断和影响分析逻辑。

第五阶段仍然不做：

1. 不调整后端接口和元数据采集逻辑。
2. 不改变表 / 字段 / 索引的查询参数。
3. 不引入虚拟列表或新的表格组件。
4. 不改变移动端为单列浏览的基本结构。

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

## 第四阶段：底部全宽关系视图

### 设计动机

第三阶段已经把关系信息补齐到“可分析”的程度，但关系页签仍放在右侧表详情面板里。右侧面板宽度有限，实际使用时会出现几个问题：

1. 来源字段和目标字段经常是 `schema.table.column`，文本较长，容易被省略。
2. 影响说明、规则名称、ON UPDATE / ON DELETE 等列无法同时展示，只能依赖详情弹窗。
3. 关系图和关系表挤在同一个窄面板中，图占用纵向空间，表格可读性下降。
4. 用户在结构浏览中通常先看字段和索引，再在页面下方做关系分析，把关系独立出来更符合“单表详情 + 下游分析”的阅读顺序。

因此第四阶段建议把“关系”从右侧 `字段 / 索引 / 关系` 页签中移出，作为结构浏览底部独立面板。上方继续保留三栏结构，下方提供全宽关系分析区。

### 布局方案

结构浏览调整为两层：

1. 上层仍为三栏：
   - 左侧：库 / Schema。
   - 中间：表 / 视图。
   - 右侧：当前表基础信息、字段、索引。
2. 下层新增全宽“表关系”面板：
   - 仅在已选择非 Redis 表时展示。
   - 横跨整个结构浏览内容宽度。
   - 标题显示当前表名和关系总数。
   - 顶部保留外键、推断、引用外部、被引用统计。
   - 右侧保留关系来源过滤和“仅外键 / 含推断”开关。

### 关系图处理

底部面板保留轻量关系图，但定位从“主展示”调整为“快速方向提示”：

1. 中心节点仍为当前表。
2. 左侧展示引用当前表的来源表。
3. 右侧展示当前表引用的目标表。
4. 节点点击仍然跳转到对应表。
5. 关系较多时，表格作为主要信息承载，关系图不再强行展示所有细节。

### 宽表字段

底部关系列表改为宽表，优先减少省略。建议列如下：

1. `方向`：引用外部 / 被引用。
2. `来源表`：来源 `schema.table`。
3. `来源字段`：来源字段名。
4. `目标表`：目标 `schema.table`。
5. `目标字段`：目标字段名。
6. `类型`：外键 / 推断。
7. `来源`：数据库约束 / 命名规则 / 唯一索引。
8. `可信度`：推断可信度，真实外键为 100。
9. `影响`：影响等级和简短说明。
10. `ON UPDATE`：真实外键更新规则。
11. `ON DELETE`：真实外键删除规则。
12. `规则`：约束名或推断规则。
13. `操作`：跳转、JOIN、详情，固定在右侧。

### 前端改动范围

只需要修改前端：

1. `web/src/views/asset/DatabaseManagement.vue`
   - 从右侧详情 `el-tabs` 中移除 `关系` tab。
   - 在三栏 `metadata-content` 下方新增 `relation-panel`。
   - 复用现有 `tableRelations`、`relationSummary`、`incomingTableRelations`、`outgoingTableRelations`、`relationSourceFilter` 和 `includeInferredRelations`。
   - 调整 `handleOpenRelationTable`，跳转后不再切换到已移除的 `relations` tab。
   - 可选增加 `relationPanelRef`，跳转目标表后滚动到底部关系区。
   - 增加底部关系面板和宽表样式。
2. `web/src/api/database.ts`
   - 不需要修改，当前字段已经满足宽表展示。

### 交互细节

1. 用户选中一张表后，上方右侧默认展示字段和索引。
2. 如果当前实例不是 Redis，底部出现“表关系”面板。
3. 用户可以在底部直接过滤外键 / 推断关系。
4. 点击“跳转”后切换到关联表，并滚动到关系面板，便于继续追踪链路。
5. 点击“JOIN”仍复制基础 JOIN 片段。
6. 点击“详情”仍打开第三阶段已有关系详情弹窗。

### 验收标准

1. 结构浏览右侧详情只保留字段和索引。
2. 选中非 Redis 表后，页面底部展示全宽表关系面板。
3. 关系表横向空间更大，来源表、来源字段、目标表、目标字段、影响、规则等列独立展示。
4. 外键 / 推断过滤和含推断开关仍可用。
5. 跳转、JOIN 复制、详情弹窗仍可用。
6. Redis 元数据实例不展示关系面板。
7. 前端类型检查和生产构建通过。

## 第五阶段：结构浏览三栏高度优化

### 设计动机

第四阶段把“表关系”移动到底部后，上方三栏变成主要的结构浏览工作区。实际使用中，“表 / 视图”面板仍然使用固定表格高度，而右侧“字段 / 索引”因为包含基础信息卡片和字段表，会把整行 grid 撑得更高。中间表格没有跟随面板高度增长，就会在表格下方留下明显空白。

这类空白不是数据问题，而是布局问题：

1. `表 / 视图` 表格固定为 `height=520`。
2. `字段 / 索引` 面板内容高度通常更高。
3. CSS grid 默认会把同一行的面板拉伸到一致高度。
4. 中间面板被拉高，但内部表格仍维持固定高度，于是底部出现空白。

### 布局原则

第五阶段建议把结构浏览上方三栏改成“固定工作区 + 内部滚动”的方式：

1. 桌面端三栏容器使用视口相关高度，例如 `clamp(620px, calc(100vh - 300px), 820px)`。
2. 三个面板统一使用纵向 flex 布局。
3. 面板标题固定在顶部。
4. 表格和树区域使用 `flex: 1` 占满剩余空间。
5. 表多、字段多或索引多时，在对应表格内部滚动，不把整页无限撑高。
6. 底部“表关系”面板仍然位于三栏下方，作为独立分析区。

### 前端改动范围

只需要修改 `web/src/views/asset/DatabaseManagement.vue`：

1. 把“表 / 视图”主表格从固定 `height=520` 改为 `height=100%`，并加填充类。
2. 把 Redis 属性表、字段表、索引表从固定 `height=470` 改为 `height=100%`，让其占满详情面板剩余高度。
3. 给右侧详情内容增加一个 body 容器，用于承载基础信息卡片和 tabs。
4. 给 `.metadata-content` 增加桌面端自适应高度。
5. 给 `.schema-panel`、`.table-panel`、`.detail-panel` 增加 flex 布局。
6. 给 `.schema-tree`、`.detail-tabs`、`.metadata-table-fill` 增加 flex 填充和 `min-height: 0`，避免 flex 子项撑破容器。
7. 移动端恢复自然高度，表格使用固定可用高度，避免手机端出现过长的嵌套滚动。

### 交互细节

1. 用户在中间表格滚动时，只滚动“表 / 视图”表格，不影响右侧字段区域。
2. 用户在右侧字段 / 索引表滚动时，只滚动当前表格，不影响中间表列表。
3. 左侧 Schema 数量多时，树区域独立滚动。
4. 切换表、同步元数据、关系跳转的行为不变。
5. 底部关系宽表继续跟随当前选中表刷新。

### 验收标准

1. “表 / 视图”面板底部不再出现大块空白。
2. 中间表格能随面板高度填满剩余空间。
3. 右侧字段 / 索引表格仍可正常滚动。
4. 左侧 Schema 树不撑破面板。
5. 移动端结构浏览仍为单列展示。
6. 前端类型检查和生产构建通过。

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
