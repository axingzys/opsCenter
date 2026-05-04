# 数据库备份与恢复模块前端优化方案

更新时间：2026-05-04

## 1. 背景

当前入口位于“项目资产管理 / 数据库管理 / 备份与恢复”。从现有截图、前端代码和后端能力看，这个模块已经覆盖了不少备份恢复能力，但页面把日常操作、底层编排资源、排障信息和研发阶段提示混在一起展示，导致用户第一眼很难判断：

- 哪些数据库已经受保护。
- 哪些数据库存在恢复风险。
- 最近一次备份是否成功。
- 当前能恢复到什么时间点。
- 应该点击哪个按钮完成保护、备份或恢复演练。

本方案的目标不是砍掉功能，而是重新组织信息架构：普通用户默认看到“保护状态”和“下一步动作”，管理员和排障人员仍然可以进入“高级资源”处理 Runner、Barman、WAL、日志归档、存储和恢复计划等底层资源。

## 2. 已检查的相关代码和文档

前端主页面：

- `web/src/views/asset/DatabaseManagement.vue`
  - 备份恢复主 Tab：约 `2024` 行开始。
  - P5 备份恢复卡片和保护概览：约 `2395` 行开始。
  - 高级资源 Tab：约 `2658` 行开始。
  - 备份任务弹窗：约 `4650` 行开始。
  - 备份策略弹窗：约 `4842` 行开始。
  - 存储、日志归档、Runner、Barman、恢复演练相关弹窗：约 `5364` 行之后。
  - 备份恢复样式：约 `17668` 行开始。

后端路由：

- `internal/server/database/http.go`
  - `/backup-tasks`
  - `/backup-records`
  - `/restore-jobs`
  - `/restore-plans`
  - `/protection-profiles`
  - `/protection-risks`
  - `/protection-wizards/mysql-pitr`
  - `/protection-wizards/postgresql-barman`
  - `/backup-policies`
  - `/log-archive-streams`
  - `/log-archives`
  - `/storage-profiles`
  - `/runner-hosts`
  - `/runner-jobs`
  - `/barman-servers`

后端业务能力：

- `internal/biz/database/backup.go`
- `internal/biz/database/backup_policy.go`
- `internal/biz/database/backup_pitr.go`
- `internal/biz/database/restore.go`
- `internal/biz/database/restore_pitr.go`
- `internal/biz/database/protection_wizard.go`
- `internal/biz/database/postgres_barman_wizard.go`
- `internal/biz/database/runner*.go`
- `internal/biz/database/storage_posture.go`
- `internal/biz/database/model.go`

相关文档：

- `docs/database-large-backup-pitr-plan.md`
- `docs/database-management-optimization-plan.md`
- `docs/database-postgresql-custom-backup-plan.md`
- `scripts/pitr-e2e/README.md`

## 3. 当前问题

### 3.1 页面结构问题

当前页面从上到下堆叠了多个大块：

1. 逻辑备份任务。
2. 备份记录。
3. 备份与恢复 P5 区域。
4. 恢复演练记录。

用户需要滚动很长页面才能看到核心信息，而且“备份任务”和“保护概览”之间没有清晰主次。对于普通用户来说，最重要的是数据库是否可恢复，而不是先管理备份任务表。

### 3.2 高级资源暴露过多

当前高级资源里包含：

- 备份策略。
- 存储配置。
- Runner 主机。
- Runner 作业。
- Barman Server。
- 日志归档流。
- 日志归档文件。
- 日志归档事件。
- 恢复计划。
- Barman Catalog。
- WAL 状态。

这些资源对后端是必要的，但它们属于“备份系统内部资源”。如果直接放在主视图里，用户会把它们理解成必须手工配置，学习成本很高。

### 3.3 操作列过重

一些表格操作列同时出现很多链接，例如：

- 立即备份。
- 增量。
- 恢复演练。
- 校验。
- 详情。
- 修复。
- 运行全量。
- 运行增量。
- 校验链。
- Synthetic 预览。
- Synthetic 执行。
- 清理预览。
- 清理执行。
- 编辑。
- 删除。

结果是右侧固定列很宽，主数据列被挤压，页面显得拥挤。

### 3.4 专业术语缺少分层

页面直接暴露了大量底层术语：

- PITR。
- WAL。
- RPO。
- Runner。
- Barman。
- Synthetic Full。
- LSN。
- binlog。
- artifact。
- retention。
- cursor。
- spool。
- tool image。
- digest。

这些术语对备份管理员有意义，但对普通资产管理用户不友好。主界面应该用业务语言表达，高级信息放到详情抽屉或高级资源里。

### 3.5 表单复杂度过高

MySQL/MariaDB PITR、PostgreSQL Barman、备份策略、日志归档流等表单暴露了太多底层字段。用户容易不知道哪些必填、哪些保持默认即可。

当前应该把表单拆成：

- 普通模式：只保留业务决策字段。
- 高级设置：保留工具、路径、JSON、镜像、Runner 参数等底层字段。

### 3.6 研发阶段标签进入用户界面

页面中有 `P1`、`P5` 一类阶段标签和实现边界说明。这类信息适合文档或研发内部，不适合产品界面。用户界面应该改为产品化提示，例如：

- “生产大库建议使用物理备份 + 日志归档。”
- “当前实例尚未启用可恢复性保护。”
- “最近一次恢复演练已过期。”

### 3.7 已发现的前端表单绑定问题

备份任务弹窗中，部分字段显示在“备份任务”表单里，但绑定到了 `backupPolicyForm`，不是 `backupTaskForm`。这会导致任务弹窗和策略弹窗状态串联，属于需要优先修复的问题。

涉及字段包括：

- `toolExecutionMode`
- `toolImage`
- `toolImageDigest`
- `dataDirMount`
- `workDirMount`
- `networkMode`
- `dataDirReadOnly`

建议第一阶段直接修复。

## 4. 优化目标

### 4.1 用户目标

普通用户进入页面后，应能在 10 秒内判断：

- 当前有多少数据库已保护。
- 哪些数据库没有保护。
- 哪些数据库有严重风险。
- 最近备份是否成功。
- 可恢复窗口是什么。
- 是否有恢复演练证明。
- 下一步应该点击什么。

### 4.2 管理员目标

备份管理员应能完成：

- 为数据库启用保护。
- 创建或调整备份策略。
- 检查 Runner、Barman、存储、日志归档状态。
- 手工触发备份。
- 执行恢复演练。
- 查看恢复证明。
- 排查日志归档、备份链、Runner 作业失败。

### 4.3 产品目标

界面需要从“资源表堆叠”改为“任务驱动”：

- 主页面展示保护结果。
- 向导负责配置保护。
- 详情抽屉展示链路和证明。
- 高级资源用于管理和排障。

## 5. 目标信息架构

备份与恢复页面建议改为 3 个主 Tab：

1. `保护概览`
2. `任务与记录`
3. `高级资源`

其中 `保护概览` 为默认 Tab。

### 5.1 保护概览

面向日常用户和资产负责人。

展示内容：

- 顶部摘要。
- 风险提示。
- 保护状态表。
- 实例详情抽屉。
- 快捷操作。

核心操作：

- 启用保护。
- 立即备份。
- 恢复演练。
- 修复风险。
- 查看详情。

### 5.2 任务与记录

面向备份操作人员。

建议拆成二级 Tab：

- `备份任务`
- `备份记录`
- `恢复演练记录`

这部分不再抢占首屏，只作为历史和任务管理入口。

### 5.3 高级资源

面向备份管理员和排障人员。

建议分组展示，而不是横向塞很多 Tab：

- `执行资源`
  - Runner 主机
  - Runner 作业
  - 工具探测
  - Agent 状态
- `存储与保留`
  - 存储配置
  - 存储姿态检查
  - 保留策略
  - 清理预览和清理执行
- `备份链路`
  - 备份策略
  - 备份链校验
  - Synthetic Full
  - 备份记录链路
- `日志归档`
  - 日志归档流
  - 日志归档文件
  - 日志归档事件
  - WAL/binlog 连续性
- `PostgreSQL Barman`
  - Barman Server
  - Barman Catalog
  - WAL 状态
- `恢复编排`
  - 恢复计划
  - 恢复演练
  - 恢复证明
  - 清理恢复环境

高级资源入口需要有说明：

> 高级资源用于备份管理员配置 Runner、存储、日志归档、Barman 和恢复计划。普通备份、保护启用和恢复演练建议从“保护概览”操作。

## 6. 首屏设计

### 6.1 顶部区域

标题：`备份与恢复`

副标题：

> 管理数据库备份保护、可恢复窗口和恢复演练证明。

右侧主按钮：

- `启用保护`
- `立即备份`
- `恢复演练`
- `刷新`

其中：

- `启用保护` 是最高优先级按钮。
- `立即备份` 可以在选择实例后使用。
- `恢复演练` 可以在选择实例或记录后使用。
- `刷新` 使用图标按钮即可。

不建议保留：

- `P1`
- `P5`
- `保护向导`
- `PostgreSQL Barman 向导`

`保护向导` 和 `PostgreSQL Barman 向导` 应合并进 `启用保护` 流程。

### 6.2 摘要卡片

顶部建议 4 张卡片：

1. `已保护实例`
   - 数值：已启用保护的实例数量。
   - 辅助信息：PITR 可用数量。
2. `存在风险`
   - 数值：严重风险数量。
   - 辅助信息：中低风险数量。
3. `最近成功备份`
   - 数值：最近一次成功备份时间。
   - 辅助信息：失败任务数量。
4. `恢复演练`
   - 数值：最近一次成功演练时间。
   - 辅助信息：演练过期数量。

卡片颜色不要过重：

- 正常：中性背景。
- 风险：浅红色提示。
- 警告：浅橙色提示。
- 成功：浅绿色标签。

### 6.3 风险提示条

只展示真正需要用户行动的风险。

示例：

- “3 个实例尚未启用备份保护。”
- “1 个 MySQL 实例 binlog 归档延迟超过 RPO。”
- “2 个实例最近 7 天没有恢复演练证明。”

每条风险后面给动作：

- `查看实例`
- `修复`
- `发起演练`

## 7. 保护概览表

### 7.1 表格列

建议列：

| 列名 | 展示内容 | 说明 |
| --- | --- | --- |
| 实例 | 实例名、地址、引擎标签 | 保留引擎和版本，但不要挤太多字段 |
| 保护状态 | 未保护、逻辑备份、物理备份、PITR 已就绪 | 用业务标签表达 |
| 可恢复窗口 | 起止时间或 `-` | 替代散落的 full/inc/log 字段 |
| 最近备份 | 最近成功时间、备份类型 | 失败时显示失败原因摘要 |
| 日志归档 | 正常、延迟、断档、不支持 | MySQL 显示 binlog，PostgreSQL 显示 WAL |
| 恢复演练 | 最近成功时间、是否过期 | 没有演练显示“未演练” |
| 风险 | 严重、高、中、低、正常 | 风险原因放摘要 |
| 操作 | 主操作 + 更多 | 不再横向铺很多链接 |

### 7.2 操作设计

每行最多展示 3 个可见动作：

- 首要动作：
  - 未保护：`启用保护`
  - 有严重风险：`修复风险`
  - 正常：`立即备份`
- 次要动作：
  - `恢复演练`
  - `详情`
- 更多动作：
  - 校验
  - 下载
  - 编辑策略
  - 查看日志
  - 查看 Runner 作业
  - 高级修复

### 7.3 风险文案

风险文案需要从技术描述改成行动描述。

不推荐：

- “RPO: - / 不支持”
- “存储: 未检测”
- “副本: 未知”
- “当前实例没有启用备份策略或成功备份”

推荐：

- “尚未启用备份保护。”
- “尚未检测备份存储可用性。”
- “最近没有恢复演练证明。”
- “日志归档未启动，暂不能按时间点恢复。”
- “备份链缺少最近全量备份。”

## 8. 实例详情抽屉

点击 `详情` 后从右侧打开实例详情抽屉，避免把所有字段塞进表格。

建议分 5 个区块：

### 8.1 保护摘要

- 实例名称。
- 引擎和版本。
- 保护等级。
- 当前风险等级。
- 推荐动作。

### 8.2 可恢复窗口

用时间线展示：

- 最近全量备份。
- 最近增量备份。
- 最近 Synthetic Full。
- 日志归档起点。
- 日志归档终点。
- 当前可恢复范围。

如果日志断档，时间线上标出断档位置。

### 8.3 最近任务

展示最近 5 条：

- 备份任务。
- 日志归档任务。
- 校验任务。
- 恢复演练任务。

字段只保留：

- 时间。
- 类型。
- 状态。
- 耗时。
- 结果摘要。

### 8.4 恢复证明

展示：

- 最近演练时间。
- 目标实例或隔离环境。
- 恢复模式。
- 校验结果。
- Proof 下载或查看入口。
- 是否已过期。

### 8.5 高级信息

默认折叠：

- Runner。
- Barman Server。
- Storage Profile。
- Backup Policy。
- Log Archive Stream。
- WAL/binlog cursor。
- 原始 JSON。

## 9. 任务与记录设计

### 9.1 备份任务

用途：管理传统逻辑备份任务和少量手工任务。

列表建议列：

- 任务名称。
- 实例。
- 类型。
- 计划。
- 最近执行。
- 最近结果。
- 保留天数。
- 状态。
- 操作。

隐藏或移入详情：

- PITR 是否支持。
- 工具镜像。
- Digest。
- datadir 挂载。
- 网络模式。
- 工作目录。

这些字段不应该默认出现在任务列表。

### 9.2 备份记录

用途：查看和使用备份产物。

列表建议列：

- 创建时间。
- 实例。
- 类型。
- 备份级别。
- 文件大小。
- 可恢复窗口。
- 校验状态。
- 过期时间。
- 操作。

操作建议：

- `校验`
- `恢复演练`
- `下载`
- `更多`

`更多` 包括：

- 查看详情。
- 查看链路。
- 查看存储。
- 清理。

### 9.3 恢复演练记录

用途：审计和证明。

列表建议列：

- 创建时间。
- 来源实例。
- 目标环境。
- 模式。
- 状态。
- 耗时。
- 操作人。
- 证明状态。
- 操作。

操作建议：

- `详情`
- `Proof`
- `清理`

失败记录需要展示失败阶段：

- 准备 Runner。
- 拉取备份文件。
- 启动隔离环境。
- 导入数据。
- 校验失败。
- 清理失败。

## 10. 向导优化

### 10.1 统一入口

保留一个入口：`启用保护`。

点击后进入统一向导：

1. 选择实例。
2. 系统识别数据库引擎。
3. 根据引擎推荐保护方案。
4. 用户确认计划、RPO、保留天数。
5. 预检查。
6. 应用配置。
7. 可选立即执行首次备份或恢复演练。

不再在主页面暴露多个向导按钮。

### 10.2 MySQL/MariaDB 推荐向导

普通模式只展示：

- 实例。
- 保护方案：
  - `逻辑备份`
  - `物理备份`
  - `物理备份 + binlog PITR`
- 备份计划。
- RPO 目标。
- 保留天数。
- Runner。
- 存储位置。
- 是否立即执行首次全量备份。

高级设置折叠展示：

- xtrabackup/mariadb-backup 工具选择。
- 工具执行模式。
- 工具镜像。
- 镜像 Digest。
- datadir 挂载。
- 工作目录挂载。
- 网络模式。
- binlog 归档目录。
- Synthetic Full 规则。
- retention JSON。
- archive config JSON。

默认值建议：

- 生产大库优先推荐 `物理备份 + binlog PITR`。
- 非生产或小库可推荐 `逻辑备份`。
- 首次启用保护建议立即执行全量备份。
- RPO 默认 5 分钟或 15 分钟，根据现有文档策略选择。
- 恢复演练建议启用后立即执行一次。

### 10.3 PostgreSQL 推荐向导

普通模式只展示：

- 实例。
- 保护方案：
  - `逻辑备份`
  - `pg_basebackup`
  - `Barman`
- Runner。
- Barman Server。
- 保留天数。
- 是否立即检查 Barman。
- 是否立即同步 catalog。
- 是否立即执行首次备份。

高级设置折叠展示：

- Barman home。
- Barman config path。
- streaming user。
- replication slot。
- WAL archive mode。
- Barman raw config JSON。
- SSH 参数。
- catalog 同步参数。

默认值建议：

- 如果目标是 PostgreSQL PITR，优先引导 Barman。
- 如果用户没有 Barman 基础设施，先提示需要 Runner 和 Barman Server。
- WAL-G/pgBackRest 可保留为外部登记能力，不在普通向导里作为默认推荐。

### 10.4 恢复演练向导

恢复演练应该从“证明可恢复”角度设计。

步骤：

1. 选择来源实例。
2. 选择恢复点：
   - 最新可恢复点。
   - 指定时间点。
   - 指定备份记录。
3. 选择目标：
   - 隔离容器。
   - 临时实例。
   - 指定测试实例。
4. 预检查：
   - 备份链完整性。
   - 日志连续性。
   - Runner 可用性。
   - 存储可访问性。
   - 目标环境安全性。
5. 执行演练。
6. 生成 Proof。

风险限制：

- 默认禁止直接恢复到生产实例。
- 如果目标是已启用或生产标记实例，需要二次确认或审批。
- 恢复前必须展示将要清理或覆盖的目标信息。

## 11. 高级资源重组

### 11.1 展示方式

高级资源建议不再使用一排很多 Tab，而是使用分组卡片或左侧二级导航。

推荐结构：

- 左侧导航：资源组。
- 右侧内容：当前资源表和详情。

资源组：

1. 执行资源。
2. 存储与保留。
3. 备份链路。
4. 日志归档。
5. PostgreSQL Barman。
6. 恢复编排。

### 11.2 操作收敛

每个高级资源表的行操作最多展示两个直接按钮：

- 主动作。
- 详情。

其他动作全部放入 `更多`：

- 编辑。
- 删除。
- 校验。
- 同步。
- 运行。
- 清理。
- 查看作业。
- 查看事件。

### 11.3 字段说明

高级字段需要增加说明，而不是让用户猜。

示例：

- Runner：执行备份、归档和恢复命令的主机。
- Barman Server：PostgreSQL 物理备份和 WAL 管理服务。
- WAL：PostgreSQL 事务日志，用于按时间点恢复。
- binlog：MySQL/MariaDB 二进制日志，用于按时间点恢复。
- Synthetic Full：由全量和增量合成的新全量，用于缩短恢复链。
- RPO：允许丢失数据的最长时间窗口。

说明可以通过表头 tooltip、字段旁 help icon、详情抽屉里的解释块实现。

## 12. 后端配合建议

第一阶段不强依赖后端改动，可以先用现有接口完成前端重排。

后续建议补充以下后端能力，让前端更简单。

### 12.1 保护概览聚合接口

建议新增：

`GET /api/database/protection-overview`

返回每个实例的聚合状态：

- instanceId
- instanceName
- engine
- version
- address
- protectionLevel
- protectionStatus
- recoverableWindowStart
- recoverableWindowEnd
- lastFullBackupAt
- lastIncrementalBackupAt
- lastLogArchiveAt
- lastSuccessfulBackupAt
- lastRestoreDrillAt
- restoreProofStatus
- riskLevel
- riskReasons
- recommendedAction
- availableActions

这样前端不用同时拼 `protection-profiles`、`backup-records`、`restore-jobs`、`log-archive-streams`、`barman-servers`。

### 12.2 推荐保护模板接口

建议新增：

`GET /api/database/instances/:id/protection-recommendation`

返回：

- 推荐方案。
- 可选方案。
- 默认 RPO。
- 默认保留天数。
- 默认 Runner。
- 默认存储。
- 是否支持 PITR。
- 是否需要 Barman。
- 是否需要 binlog/WAL。
- 不支持原因。
- 风险说明。

### 12.3 风险动作接口

建议让风险返回结构化修复动作：

- actionType
- actionLabel
- actionTarget
- severity
- reason
- canAutoFix
- requiresConfirmation

示例：

```json
{
  "riskLevel": "critical",
  "reason": "未启用备份保护",
  "recommendedAction": {
    "actionType": "open_protection_wizard",
    "actionLabel": "启用保护",
    "requiresConfirmation": false
  }
}
```

### 12.4 表单 schema 或枚举接口

建议后端统一返回当前引擎支持的：

- backupType
- backupMethod
- restoreMode
- storageType
- runnerCapability
- pitrCapability
- barmanCapability

这样前端可以少写硬编码判断。

### 12.5 审批和高危动作

生产化阶段建议增加：

- 删除备份审批。
- 生产恢复审批。
- 清理备份链审批。
- 修改保护策略审计。
- 恢复演练证明过期提醒。

## 13. 视觉优化建议

### 13.1 页面密度

当前页面表格密度高、右侧操作列宽、视觉层级弱。建议：

- 首屏只保留一个主表。
- 摘要卡片高度保持紧凑。
- 表格默认每页 10 条。
- 长文本统一省略，详情抽屉展示完整内容。
- 固定操作列宽度控制在 180px 以内。

### 13.2 状态标签

状态标签统一颜色：

- 成功：绿色。
- 运行中：蓝色。
- 警告：橙色。
- 失败/严重：红色。
- 未配置/未知：灰色。

标签文案统一：

- `未保护`
- `已保护`
- `PITR 已就绪`
- `日志延迟`
- `链路断档`
- `演练过期`
- `校验通过`
- `校验失败`

### 13.3 空状态

空状态需要给动作。

示例：

- 没有保护实例：展示 `启用保护`。
- 没有备份记录：展示 `立即备份`。
- 没有恢复演练：展示 `发起恢复演练`。
- 没有 Runner：展示 `添加 Runner`。
- 没有 Barman Server：展示 `配置 Barman Server`。

### 13.4 加载和失败状态

保护概览加载失败时，不应该整页空白。

建议：

- 摘要卡片显示骨架。
- 表格显示局部错误。
- 保留刷新按钮。
- 错误信息给出接口或资源类型。

## 14. 实施计划

### 14.1 P0：立即修复

目标：不改变后端，不大改业务逻辑，先修明显问题。

任务：

1. 修复备份任务弹窗错误绑定 `backupPolicyForm` 的字段。
2. 移除用户界面中的 `P1`、`P5` 阶段标签。
3. 将默认视图调整为 `保护概览`。
4. 合并重复按钮：
   - `启用数据库保护`
   - `保护向导`
   - `PostgreSQL Barman 向导`
5. 表格操作列初步收敛，非主操作放入 `更多`。

验收标准：

- 创建备份任务时不会污染备份策略表单。
- 用户进入页面后首先看到保护概览。
- 首屏不再出现研发阶段标签。
- 主操作不超过 4 个。

### 14.2 P1：页面结构重排

目标：完成主要 UX 重构。

任务：

1. 新增顶部摘要卡片。
2. 重构保护概览表列。
3. 增加实例详情抽屉。
4. 将逻辑备份任务、备份记录、恢复演练记录移动到 `任务与记录`。
5. 将高级资源移动到独立分组区域。
6. 优化风险文案和状态标签。

验收标准：

- 普通用户可以从保护概览完成启用保护、立即备份、恢复演练、修复风险。
- 备份记录和恢复记录不再挤占首屏。
- 高级资源仍可访问，但不会干扰日常操作。

### 14.3 P2：向导简化

目标：降低配置门槛。

任务：

1. 统一 `启用保护` 向导。
2. MySQL/MariaDB 向导拆分普通模式和高级设置。
3. PostgreSQL Barman 向导拆分普通模式和高级设置。
4. JSON 配置改成结构化表单生成。
5. 增加预检查结果页。
6. 应用成功后引导首次备份或恢复演练。

验收标准：

- 普通模式下用户不需要理解工具镜像、Digest、挂载、Synthetic JSON、Barman raw config。
- 高级用户仍可展开高级设置修改底层参数。
- 向导应用前能看到预检查结果和风险提示。

### 14.4 P3：后端聚合接口

目标：减少前端拼装状态，提高一致性。

任务：

1. 增加保护概览聚合接口。
2. 增加实例保护推荐接口。
3. 风险结果返回结构化动作。
4. 统一返回引擎能力枚举。

验收标准：

- 前端保护概览主要依赖一个聚合接口。
- 风险修复按钮由后端推荐动作驱动。
- 新增数据库引擎或保护方式时，前端改动减少。

### 14.5 P4：生产化增强

目标：提升生产灾备治理能力。

任务：

1. 恢复演练过期提醒。
2. 备份失败告警。
3. RPO 超时告警。
4. 高危操作审批。
5. 风险快照留存。
6. 备份 SLA 日历。

验收标准：

- 可以追踪风险历史。
- 可以证明恢复演练是否持续有效。
- 生产恢复、删除、清理等动作有审批和审计。

## 15. 具体改动清单

### 15.1 前端文件

主要改动：

- `web/src/views/asset/DatabaseManagement.vue`

建议后续拆分：

- `web/src/views/asset/database-backup/BackupRestorePage.vue`
- `web/src/views/asset/database-backup/ProtectionOverview.vue`
- `web/src/views/asset/database-backup/ProtectionSummaryCards.vue`
- `web/src/views/asset/database-backup/ProtectionDetailDrawer.vue`
- `web/src/views/asset/database-backup/BackupTaskList.vue`
- `web/src/views/asset/database-backup/BackupRecordList.vue`
- `web/src/views/asset/database-backup/RestoreDrillList.vue`
- `web/src/views/asset/database-backup/AdvancedResources.vue`
- `web/src/views/asset/database-backup/EnableProtectionWizard.vue`

如果当前项目不适合立即拆文件，可以第一阶段先在现有 SFC 内局部重排，第二阶段再拆组件。

### 15.2 API 文件

可能改动：

- `web/src/api/database.ts`

新增或整理：

- `getProtectionOverview`
- `getProtectionRecommendation`
- `previewProtectionWizard`
- `applyProtectionWizard`
- `runRestoreDrill`
- `getRestoreProof`

### 15.3 后端文件

后续可能改动：

- `internal/server/database/http.go`
- `internal/service/database/http.go`
- `internal/biz/database/protection_profile.go`
- `internal/biz/database/protection_wizard.go`
- `internal/biz/database/postgres_barman_wizard.go`
- `internal/biz/database/restore_pitr.go`

第一阶段不要求后端改动。

## 16. 不建议做的事

不建议：

- 删除高级资源。
- 把所有数据库引擎强行合成一个黑盒流程。
- 前端绕过 Runner、存储、权限、审计和恢复证明。
- 允许默认恢复到生产库。
- 让用户手写 JSON 作为普通配置方式。
- 在主界面继续展示研发阶段标签。
- 将所有操作继续横向塞进固定操作列。

## 17. 推荐优先级

建议按下面顺序执行：

1. 修复表单绑定 bug。
2. 保护概览默认首屏。
3. 顶部摘要卡片。
4. 操作列收敛。
5. 高级资源分组和降级展示。
6. 详情抽屉。
7. 统一启用保护向导。
8. 后端聚合接口。
9. 告警、审批和风险历史。

## 18. 最小可交付版本

如果只做一版最小可交付，建议包含：

- 默认进入 `保护概览`。
- 顶部 4 个摘要卡片。
- 保护概览表重排。
- 行操作收敛。
- 高级资源折叠或移到独立 Tab。
- 修复备份任务弹窗绑定 bug。
- 移除 P1/P5 阶段标签。

这部分可以明显改善观感和使用路径，而且风险低，不需要大改后端。

## 19. 本次落地范围

落地时间：2026-05-05

本次先按 P0/P1 的低风险路径实现，不改后端接口，主要改动集中在 `web/src/views/asset/DatabaseManagement.vue`。

已落地：

- 将备份恢复页的主工作区改为 `保护概览`，通过样式顺序让它优先出现在备份任务、备份记录和恢复演练记录之前。
- 移除备份恢复页中的 `P1`、`P5` 研发阶段文案，替换为面向用户的保护建议和能力说明。
- 新增保护概览摘要区，展示已保护实例、风险数量、最近成功备份和恢复演练状态。
- 新增风险快捷条，展示当前最需要处理的风险并提供处理入口。
- 将 `启用数据库保护`、`保护向导`、`PostgreSQL Barman 向导` 合并成一个 `启用保护` 下拉入口。
- 收敛保护概览行操作，保留主动作、备份、恢复演练，把增量、校验、高级资源和修复放入 `更多`。
- 新增保护详情抽屉，展示实例保护等级、可恢复窗口、最近备份、日志归档、恢复证明、风险和高级资源入口。
- 将高级资源 Tab 文案按职责分组，例如 `备份链路 / 策略`、`执行资源 / Runner`、`日志归档 / 流`、`PostgreSQL / WAL 状态`。
- 收敛备份策略表的操作列，把 Synthetic、清理、编辑、删除等低频动作放入 `更多`。
- 修复备份任务弹窗误绑定 `backupPolicyForm` 的问题：任务弹窗不再展示策略级工具镜像、容器挂载和 digest 字段。

未在本次落地：

- 未新增后端聚合接口，保护概览仍使用现有 `protection-profiles`、`protection-risks` 等接口数据。
- 未重写 MySQL/MariaDB 和 PostgreSQL Barman 向导为完整分步向导。
- 未把备份恢复相关代码从大 SFC 中拆分为独立组件。
- 未增加生产恢复审批、告警、风险快照和 SLA 日历。

## 20. 第二轮落地范围

落地时间：2026-05-05

第二轮继续保持低风险改动，不调整后端 API 契约。改动目标是降低“启用保护”和“恢复演练”的理解成本，把高频必填项放在默认视图，把底层工具、路径、归档和 JSON 配置收进高级设置。

改动前复查的相关文档和代码：

- `docs/database-backup-restore-ux-optimization-plan.md`：第 3.3、4、5、8、10 节已明确指出“高级资源配置太多”和“启用保护路径分散”的问题。
- `docs/database-large-backup-pitr-plan.md`：P5 方案要求 MySQL/MariaDB 以“物理备份 + binlog PITR + 滚动合成全量”为主路径，PostgreSQL 以 Barman Server、catalog 和 WAL 状态为主路径。
- `web/src/views/asset/DatabaseManagement.vue`：备份恢复页、保护概览、MySQL/MariaDB PITR 向导、PostgreSQL Barman 向导、恢复演练对话框均集中在该大 SFC 内。
- `internal/biz/database/protection_wizard.go`：MySQL/MariaDB 启用保护请求已支持工具、容器、binlog、Synthetic Full、保留策略和归档 JSON。
- `internal/biz/database/postgres_barman_wizard.go`：PostgreSQL Barman 启用保护请求已支持 Barman Server、路径、归档器、slot、catalog/WAL 同步和配置 JSON。

已落地：

- 将 `启用保护` 从引擎下拉改为“先选实例，再进入推荐向导”的统一对话框，用户不需要先判断 MySQL/MariaDB 或 PostgreSQL 应该走哪个入口。
- MySQL/MariaDB 向导默认只展示实例、保护方案、Runner、存储、首次全量、增量计划、RPO、日志保留、增量保留和恢复证明。
- MySQL/MariaDB 向导把策略名称、来源角色、备份工具、工具模式、容器镜像/digest、datadir、网络、binlog 归档模式、复用资源、Synthetic Full 和 JSON 配置全部收进高级设置。
- PostgreSQL Barman 向导默认只展示实例、Runner、Barman Server 名称、复用 Server、保留窗口以及 check/catalog/WAL/首次备份动作。
- PostgreSQL Barman 向导把策略名称、备份方法、Barman home、配置路径、archiver、streaming archiver、slot 和配置 JSON 收进高级设置。
- 恢复演练对话框默认只展示恢复点、Runner、隔离环境保留时间和风险确认。
- 恢复演练把容器镜像、端口、PostgreSQL timeline/action/get-wal、失败清理、校验 SQL 和断言校验收进高级设置。

后端结论：

- 第二轮暂不需要后端改动。现有后端 payload 已能接收前端高级设置中的所有参数，且已有预览、校验和应用逻辑。
- 后续如果继续优化，建议新增后端推荐接口，由后端按实例引擎、版本、Runner 在线状态和已有备份链生成默认模板，减少前端默认值判断。

第二轮之后建议的后续轮次：

- 第三轮：把高级资源改成“资源健康中心”，用分组摘要、异常优先和详情抽屉替代多张横向表。
- 第四轮：拆分 `DatabaseManagement.vue` 中备份恢复相关组件，降低维护成本。
- 第五轮：新增后端聚合与推荐接口，补齐风险历史、审批、告警和 SLA 视图。

## 21. 第三轮落地范围

落地时间：2026-05-05

第三轮聚焦 `高级资源`，目标是让备份管理员先看到资源健康和异常，而不是先面对一排底层资源表。

改动前复查的相关文档和代码：

- `docs/database-backup-restore-ux-optimization-plan.md`：第 5.3、11 节已定义高级资源应按执行资源、存储与保留、备份链路、日志归档、PostgreSQL Barman、恢复编排分组。
- `web/src/views/asset/DatabaseManagement.vue`：高级资源仍使用横向 `el-tabs` 展示备份策略、存储、Runner、Runner 作业、Barman、归档流、归档文件、事件、恢复计划、Barman Catalog、WAL 状态。
- `web/src/api/database.ts`：现有接口已经返回高级资源列表和状态字段，包括策略状态、存储姿态、Runner 状态、Runner 作业状态、Barman 检查状态、日志归档状态、恢复计划校验状态。
- 后端 `storage_posture.go`、`runner*.go`、`backup_pitr.go`、`postgres_barman_wizard.go`、`restore_pitr.go` 已提供当前健康判断所需基础字段。

已落地：

- 在高级资源顶部新增资源健康摘要，按 `备份链路`、`执行资源`、`存储与保留`、`日志归档`、`PostgreSQL Barman`、`恢复编排` 分组展示总量和异常数。
- 新增 `异常优先` 列表，聚合备份策略异常、存储姿态失败、Runner 不在线、Runner 作业失败、Barman 检查失败、日志归档流异常、归档文件缺失/校验失败、归档事件告警和恢复计划阻断项。
- 新增异常详情抽屉，展示资源组、对象、风险、说明、建议和最近时间，并能跳转到对应资源明细。
- 将高级资源明细从横向 Tab 改为左侧资源导航，原明细表和已有操作能力保留在右侧。
- 导航按职责分组：备份链路、执行资源、存储与保留、日志归档、PostgreSQL Barman、恢复编排。
- 导航项增加资源数量和异常数量，避免用户必须逐个打开表格确认状态。

后端结论：

- 第三轮暂不新增后端接口。前端使用现有资源列表做轻量聚合，风险低且不改变 API 契约。
- 后续第五轮建议补一个后端 `resource-health` 聚合接口，把异常判断、分页和权限过滤沉到后端，减少前端一次刷新时并发拉取多张资源表。

仍未在第三轮处理：

- 未拆分 `DatabaseManagement.vue`，高级资源仍在大 SFC 内实现。
- 未新增风险历史、SLA 趋势、审批流和告警订阅。
- 未把每个高级资源表的行操作全部收敛为 `主动作 + 详情 + 更多`，本轮只改资源入口和异常优先视图。
