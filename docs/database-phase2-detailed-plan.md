# 数据库管理二期：详细实施方案

## 一期回顾
当前一期已形成最小可用闭环，已落地内容如下：

1. 数据库实例纳管：实例 CRUD、启停、凭据绑定、支持类型列表。
2. 一期主力数据库链路：MySQL / MariaDB / PostgreSQL 连接测试。
3. 元数据同步：Schema、表、字段、索引同步与任务记录。
4. 结构浏览：Schema 树、表列表、字段列表、索引列表。
5. 只读 SQL 控制台：只读校验、单语句限制、超时控制、返回行数限制。
6. 查询审计：执行、失败、拦截全量留痕，支持分页检索和 CSV 导出。
7. 按钮级 RBAC：实例、连接测试、元数据、查询、审计导出权限点。
8. 交互补齐：实例弹窗端口布局优化、PostgreSQL Schema 选择修正。

补充说明：

1. PostgreSQL 已完成真实环境联调验收，连接测试、元数据同步、结构浏览、只读查询、DDL 查看链路已验证通过。
2. SQL Server / ClickHouse / Oracle 已完成二期代码接入与统一前端表单适配，但真实实例联调验收仍需分别补齐。

## 二期总目标
二期不做高风险写操作，重点把“一期能用”提升为“二期更适合日常运维使用”的版本，聚焦四件事：

1. 结构增强：补齐表详情、DDL 预览、数据字典导出。
2. 查询增强：补齐 SQL 格式化、历史记录、执行计划、结果导出。
3. 诊断增强：补齐慢 SQL、活跃会话、基础指标趋势。
4. 适配增强：为 SQL Server、Oracle、ClickHouse 做初步接入预留。

## 二期范围拆分
### A. 结构增强
目标：把当前“字段/索引查看”升级为“结构详情工作台”。

子项：

1. 表概览卡片：对象类型、行数估算、数据大小、索引大小、注释。
2. DDL 预览：
   - 表 DDL 查看
   - 视图对象给出定义占位说明
   - 标注“基于同步元数据生成”或“实时读取”的来源
3. 数据字典导出：
   - 单表导出
   - 后续扩到 Schema 级导出
4. 表结构详情补充字段：
   - 引擎
   - 表注释
   - 主键/唯一索引标记
   - 敏感字段标记

接口建议：

1. `GET /api/v1/databases/instances/{id}/ddl?schemaName=&tableName=`
2. `GET /api/v1/databases/instances/{id}/dictionary/export?schemaName=&tableName=`

权限建议：

1. `database:metadata:view`
2. `database:metadata:export`

验收点：

1. 任意已同步表可查看 DDL。
2. 任意已同步表可导出数据字典 CSV。
3. 未同步元数据时给出明确提示，不返回空文件。

### B. 查询增强
目标：提升 SQL 控制台的连续使用体验。

子项：

1. SQL 格式化：
   - 基础缩进与关键字大写
   - 保留只读限制
2. SQL 历史记录：
   - 最近执行记录
   - 快速回填编辑器
   - 区分当前用户历史与实例维度历史
3. 常用 SQL 收藏：
   - 标题
   - 适用实例类型
   - 收藏时间
4. 查询结果导出：
   - CSV
   - 后续扩 Excel
   - 最大导出行数限制
   - 导出审计留痕
5. 执行计划：
   - MySQL：`EXPLAIN`
   - PostgreSQL：`EXPLAIN`
   - 结果结构化展示与原文展示并存

接口建议：

1. `POST /api/v1/databases/instances/{id}/query/format`
2. `GET /api/v1/databases/query-history`
3. `POST /api/v1/databases/query-favorites`
4. `POST /api/v1/databases/instances/{id}/query/export`
5. `POST /api/v1/databases/instances/{id}/query/explain`

权限建议：

1. `database:query:execute`
2. `database:query:export`
3. `database:query:history:view`
4. `database:query:explain`

验收点：

1. 最近执行 SQL 可回填。
2. 结果导出超过限制时被拦截。
3. 执行计划能区分正常计划结果和语法错误。

### C. 诊断增强
目标：从“能查 SQL”走向“能看数据库状态”。

子项：

1. 慢 SQL 列表：
   - SQL 摘要
   - 平均耗时
   - 最大耗时
   - 执行次数
2. 活跃会话：
   - 会话 ID
   - 用户
   - 当前状态
   - 执行中 SQL
   - 持续时长
3. 基础指标趋势：
   - 连接数
   - QPS/TPS 占位
   - 实例容量
   - 慢查询数量
4. 指标抓取策略：
   - 二期先按请求时实时采集
   - 三期再补定时快照表

接口建议：

1. `GET /api/v1/databases/instances/{id}/slow-queries`
2. `GET /api/v1/databases/instances/{id}/sessions`
3. `GET /api/v1/databases/instances/{id}/metrics`

权限建议：

1. `database:diagnosis:view`

验收点：

1. MySQL / PostgreSQL 可看到基础诊断数据。
2. 查询失败时返回明确原因，不影响现有元数据与 SQL 控制台。

### D. 二期数据库类型适配
目标：把二期类型接入从“路线图”变成“可分批实现”。

拆分顺序建议：

1. SQL Server：
   - 连接测试
   - 基础元数据
   - 只读查询
   - DDL / Explain 后补
2. ClickHouse：
   - 连接测试
   - 库表结构
   - 只读查询
3. Oracle：
   - 先做驱动与连接串方案验证
   - 再做元数据与只读查询

要求：

1. 前端不新增分叉页面，全部继续走统一数据库管理页。
2. 后端继续走 Adapter 归一化。
3. 每种类型先做“连接测试 + 元数据 + 只读查询”最小闭环。

### E. 审计与权限补齐
目标：二期新增能力不能绕开权限和留痕。

新增建议权限：

1. `database:metadata:export`
2. `database:query:export`
3. `database:query:history:view`
4. `database:diagnosis:view`
5. `database:query:explain`

新增建议审计类型：

1. 数据字典导出
2. 查询结果导出
3. 执行计划查看
4. 慢 SQL 查看
5. 会话查看

### F. 前端交互增强
目标：避免数据库页继续堆功能导致可用性下降。

交互原则：

1. 结构浏览继续保留三栏布局，但详情区升级为“概览 + Tabs”。
2. 查询控制台增加工具条，不改成多页面，避免切换成本。
3. 新增导出和 DDL 操作必须围绕当前所选表，不增加全局漂浮入口。
4. 结果与错误提示统一使用当前页面反馈，不新增复杂弹层链路。

### G. 测试与验收
后端：

1. DDL 生成单元测试。
2. 数据字典导出字段顺序测试。
3. 查询导出限制测试。
4. Explain / 慢 SQL / 会话接口回归测试。

前端：

1. `npm run build`
2. DDL 弹窗展示验证
3. 数据字典下载验证
4. 查询历史回填验证

联调：

1. MySQL 实例一轮完整验收
2. PostgreSQL 实例一轮完整验收
3. 二期新增类型逐个做连接与元数据抽样验收

## 二期实施批次建议
### 第 1 批：结构增强
1. 表概览卡片
2. DDL 预览
3. 数据字典导出

### 第 2 批：查询体验增强
1. SQL 格式化
2. SQL 历史记录
3. 查询结果导出

### 第 3 批：Explain 与诊断
1. Explain
2. 慢 SQL
3. 活跃会话
4. 基础指标趋势

### 第 4 批：二期数据库类型接入
1. SQL Server
2. ClickHouse
3. Oracle 预研结果落地

### 第 5 批：权限审计收口与验收
1. 新权限点补齐
2. 新审计补齐
3. 二期验收清单与回归

## 当前已启动项
当前进度：

1. 第 1 批已完成：
   - 结构浏览详情区增强
   - DDL 查看接口与前端弹窗
   - 数据字典导出接口与前端下载
2. 第 2 批已完成：
   - SQL 格式化
   - 最近 SQL 历史回填
   - 查询结果 CSV 导出
   - Explain 执行计划查看
3. 第 3 批最小闭环已完成：
   - 慢 SQL
   - 活跃会话
   - 基础指标趋势
4. 第 4 批已完成代码接入与部署验证：
   - SQL Server：连接测试、基础元数据、只读查询接入
   - ClickHouse：连接测试、基础元数据、只读查询接入
   - Oracle：连接测试、基础元数据、只读查询接入
   - 前端统一表单支持额外连接参数录入
   - 正式前后端镜像已更新并完成健康检查
5. 已完成真实环境验收：
   - MySQL 一轮回归通过
   - PostgreSQL 一轮真实实例回归通过
6. 第 5 批已完成权限审计收口：
   - 数据字典导出、查询结果导出、执行计划、诊断指标、活跃会话、慢 SQL 均已写入统一审计表
   - 查询审计新增“审计动作”维度，支持动作筛选与 CSV 导出
   - SQL 历史已排除诊断与元数据导出类记录，避免干扰回填
   - 前端审计页已展示动作标签与详情字段
7. 当前二期剩余事项：
   - SQL Server / ClickHouse / Oracle 分库真实实例联调验收
