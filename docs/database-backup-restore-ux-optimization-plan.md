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

## 22. 第四轮详细方案：备份恢复告警闭环

方案时间：2026-05-05

第四轮建议把重点从单纯拆分 `DatabaseManagement.vue` 调整为“备份恢复 + 监控告警闭环”。原因是前三轮已经解决了主路径可读性、启用保护表单复杂度和高级资源可理解性，当前最影响生产可用性的不是页面继续微调，而是备份恢复风险仍需要用户主动打开页面查看。

第四轮目标是让备份失败、RPO 超时、恢复演练过期、Runner / Barman / 存储 / 日志归档异常自动进入监控告警体系，支持通知、去重、静默、恢复通知和历史留痕。

### 22.1 改动前需要复查的文档和代码

备份恢复侧：

- `docs/database-backup-restore-ux-optimization-plan.md`
  - 第 14.5 节已把备份失败告警、RPO 超时告警、恢复演练过期提醒、风险快照留存列为生产化增强。
  - 第 19、20、21 节记录了一到三轮已落地内容。
- `web/src/views/asset/DatabaseManagement.vue`
  - 已有 `风险中心`、`保护概览`、`高级资源`、恢复演练、备份任务、备份记录。
  - 第三轮已在前端聚合了高级资源异常，但这些异常还没有主动通知能力。
- `web/src/api/database.ts`
  - 已有 `listDatabaseProtectionProfiles`、`listDatabaseProtectionRisks`、备份记录、Runner、Barman、日志归档、恢复计划等接口类型。
- `internal/biz/database/protection_risk.go`
  - 已能输出结构化风险：缺少全量基线、增量链异常、日志链缺口、Runner 不可用、Runner 工具异常、存储姿态异常、缺少恢复演练、恢复演练失败、延迟副本不可用、归档延迟过高。
- `internal/server/database/backup_alert.go`
  - 已接入备份调度失败通知，但目前更接近“发送通知”，还没有完整告警规则、告警状态和告警日志闭环。
- `internal/biz/database/backup_scheduler.go`
  - 备份调度失败、保留清理失败已经有 `BackupSchedulerNotice`。

监控告警侧：

- `docs/plugins/monitor.md`
  - 已定义告警通道、接收人、告警日志、告警防抖和恢复通知等监控概念。
- `plugins/monitor/model/alert_config.go`
  - 已有 `AlertChannel`、`AlertReceiver`、`AlertReceiverChannel`、`AlertLog`。
- `plugins/monitor/service/alert_dispatcher.go`
  - 已能加载启用通道并发送邮件、Webhook、企业微信、钉钉、飞书。
- `plugins/monitor/service/alert_service.go`
  - 已有统一 `AlertMessage`，但资源标题和指标文案主要覆盖域名、主机，数据库告警需要补充。
- `plugins/monitor/service/host_alert_service.go`
  - 已有主机告警规则、静默间隔和写入 `alert_logs` 的实现，可作为数据库告警规则的参考。
- `web/src/plugins/monitor/components/AlertLogs.vue`
  - 已能筛选域名和主机告警，还需要加入数据库告警类型、指标和资源展示。

### 22.2 当前能力判断

可以复用：

- 复用 Monitor 的告警通道、接收人和发送器，不新增独立通知系统。
- 复用 Monitor 的 `alert_logs` 作为统一告警历史。
- 复用备份恢复已有的保护风险和高级资源状态作为告警输入。
- 复用备份恢复页前三轮形成的风险中心和高级资源健康中心作为告警处理入口。

需要新增：

- 数据库备份恢复专用告警规则。
- 数据库备份恢复专用告警状态，用于去重、静默和恢复通知。
- 数据库告警扫描器。
- 数据库告警日志写入逻辑。
- 前端告警订阅页面。
- Monitor 告警日志对数据库资源类型的展示支持。

不建议新增：

- 不建议新建第二套告警通道、接收人和通知配置。
- 不建议前端每次刷新风险中心就触发告警。
- 不建议只在备份调度失败时发消息，RPO、演练过期、资源异常也要纳入。
- 不建议没有静默间隔，否则高频调度会造成告警刷屏。

### 22.3 第四轮信息架构

备份与恢复区域建议调整为：

1. `风险中心`
2. `保护概览`
3. `告警订阅`
4. `高级资源`

其中：

- `风险中心` 继续用于人工处理当前风险。
- `保护概览` 继续用于看实例保护状态和恢复证明。
- `告警订阅` 用于配置哪些风险需要主动通知谁。
- `高级资源` 用于管理员排查 Runner、Barman、存储、日志归档和恢复计划。

监控中心的 `告警日志` 也要支持数据库资源类型。这样用户既可以从备份恢复页面配置和处理，也可以从监控中心统一追踪发送历史。

### 22.4 后端数据模型建议

#### 22.4.1 数据库告警规则

建议新增模型：`DatabaseBackupAlertRule`

建议表名：`database_backup_alert_rules`

核心字段：

| 字段 | 类型 | 说明 |
|:-----|:-----|:-----|
| `id` | uint | 主键 |
| `name` | varchar(100) | 规则名称 |
| `enabled` | bool | 是否启用 |
| `scope_type` | varchar(30) | 作用范围：all、production、instance、engine、business_system、owner |
| `instance_id` | uint nullable | 指定实例 |
| `engine` | varchar(30) | 指定引擎：mysql、mariadb、postgresql、redis |
| `business_system` | varchar(100) | 指定业务系统 |
| `owner` | varchar(100) | 指定负责人 |
| `issue_types_json` | json/text | 告警问题类型列表 |
| `severity` | varchar(20) | warning、critical |
| `alert_interval` | int | 静默间隔，单位秒 |
| `recovery_notify` | bool | 是否发送恢复通知 |
| `channel_ids_json` | json/text | 指定告警通道；为空表示全部启用通道 |
| `threshold_json` | json/text | 阈值配置 |
| `description` | varchar(255) | 备注 |
| `created_at` | time | 创建时间 |
| `updated_at` | time | 更新时间 |

`threshold_json` 示例：

```json
{
  "noSuccessBackupHours": 24,
  "restoreDrillStaleDays": 30,
  "rpoLagGraceMinutes": 10,
  "includeNonProduction": false,
  "minRiskLevel": "high"
}
```

#### 22.4.2 数据库告警状态

建议新增模型：`DatabaseBackupAlertState`

建议表名：`database_backup_alert_states`

核心字段：

| 字段 | 类型 | 说明 |
|:-----|:-----|:-----|
| `id` | uint | 主键 |
| `rule_id` | uint | 规则 ID |
| `fingerprint` | varchar(180) unique | 告警指纹 |
| `resource_type` | varchar(50) | database_instance、database_backup_policy、database_runner、database_barman、database_storage、database_log_archive、database_restore |
| `resource_id` | uint | 资源 ID |
| `resource_name` | varchar(255) | 资源名称 |
| `resource_target` | varchar(255) | 主机、端口或实例 endpoint |
| `issue_type` | varchar(80) | 问题类型 |
| `severity` | varchar(20) | 告警级别 |
| `status` | varchar(20) | firing、resolved |
| `message` | text | 最近告警内容 |
| `suggestion` | varchar(255) | 建议动作 |
| `first_fired_at` | time | 首次触发时间 |
| `last_fired_at` | time | 最近触发时间 |
| `last_notified_at` | time nullable | 最近通知时间 |
| `resolved_at` | time nullable | 恢复时间 |
| `notify_count` | int | 通知次数 |
| `raw_json` | text | 结构化上下文 |

告警指纹建议：

```text
database_backup_alert:{ruleId}:{resourceType}:{resourceId}:{issueType}
```

这样可以保证同一个问题在静默期内不会重复通知，同时不同规则或不同问题互不影响。

### 22.5 告警类型建议

建议先覆盖这些数据库备份恢复告警类型：

| 告警类型 | issue type | 默认级别 | 说明 |
|:-----|:-----|:-----|:-----|
| `database_backup_failed` | `backup_failed` | critical | 定时备份、策略备份、物理备份或校验失败 |
| `database_backup_no_recent_success` | `no_recent_success_backup` | critical | 生产库超过阈值没有成功备份 |
| `database_backup_missing_full` | `missing_full_backup` | critical | 缺少全量基线，无法形成恢复链 |
| `database_backup_chain_broken` | `incremental_chain_broken` | critical | 增量链、备份链、合成全量链异常 |
| `database_backup_rpo_breached` | `rpo_breached` / `archive_lag_high` | critical | binlog/WAL 归档延迟超过 RPO |
| `database_backup_log_gap` | `log_chain_gap` | critical | 日志链缺口，PITR 覆盖不完整 |
| `database_restore_drill_stale` | `restore_drill_missing` | warning | 恢复演练超过周期未成功 |
| `database_restore_drill_failed` | `restore_drill_failed` | warning | 最近恢复演练失败 |
| `database_backup_runner_unavailable` | `runner_offline` | warning | Runner 不在线、禁用或失败 |
| `database_backup_runner_tool_failed` | `runner_tool_missing` | warning | Runner 工具缺失或版本不兼容 |
| `database_backup_storage_failed` | `storage_posture_failed` | warning | 存储姿态、权限、对象存在性或 checksum 异常 |
| `database_backup_barman_failed` | `barman_failed` | warning | Barman check、catalog、WAL 同步异常 |

第一版建议先做高价值类型：

1. `database_backup_failed`
2. `database_backup_no_recent_success`
3. `database_backup_rpo_breached`
4. `database_restore_drill_stale`
5. `database_backup_runner_unavailable`
6. `database_backup_barman_failed`
7. `database_backup_storage_failed`

### 22.6 后端调度设计

建议新增调度器：`DatabaseBackupAlertScheduler`

建议文件：

- `internal/biz/database/backup_alert_rule.go`
- `internal/biz/database/backup_alert_scheduler.go`
- `internal/data/database/backup_alert_repository.go`
- `internal/server/database/backup_alert.go`

调度器职责：

1. 每 1 分钟或 5 分钟读取启用的数据库备份告警规则。
2. 按规则 scope 筛选实例和资源。
3. 从保护风险、保护概览和高级资源状态中生成候选告警。
4. 用 fingerprint 查找或创建告警状态。
5. 如果是新告警，写状态并发送通知。
6. 如果已有 firing 状态且超过静默间隔，再次发送通知并更新 `last_notified_at`。
7. 如果候选告警消失，把状态改为 resolved。
8. 如果规则启用恢复通知，发送恢复通知并写 `alert_logs`。

调度周期建议：

- 第一版默认 5 分钟，避免高频扫描带来数据库压力。
- 备份调度失败这种事件型告警可以立即触发，不必等扫描器。

伪流程：

```text
for each enabled rule:
  candidates = evaluateRule(rule)
  currentFingerprints = set(candidates)

  for candidate in candidates:
    state = findState(rule, candidate.fingerprint)
    if state not exists:
      create firing state
      dispatch alert
      write alert log
    else if state.status == resolved:
      reopen state
      dispatch alert
      write alert log
    else if now - state.last_notified_at >= rule.alert_interval:
      dispatch alert
      write alert log
    else:
      only update last_fired_at/message/raw_json

  for each firing state of rule not in currentFingerprints:
    mark resolved
    if rule.recovery_notify:
      dispatch recovery notification
      write alert log
```

### 22.7 告警输入来源

#### 22.7.1 保护风险

优先复用 `ListProtectionRisks` 的输出：

- `riskLevel`
- `issueType`
- `message`
- `actionText`
- `blocking`
- `instanceId`
- `instanceName`
- `endpoint`
- `recoverableUntil`
- `lastFullAt`
- `lastLogArchiveAt`

这样前端风险中心和后端告警看到的问题一致。

#### 22.7.2 备份记录

用于发现：

- 最近失败备份。
- 校验失败。
- 运行超时或长时间 pending/running。
- 成功备份间隔超过阈值。

第一版可以按实例检查最近一条成功记录时间，超过 `noSuccessBackupHours` 触发。

#### 22.7.3 保护概览

用于发现：

- 保护等级为 `none`。
- 可恢复窗口为空。
- `recoverableUntil` 落后当前时间过多。
- 最近恢复演练状态为 stale 或 failed。

#### 22.7.4 高级资源

用于发现：

- Runner 不在线。
- Barman check / catalog / WAL 同步失败。
- 存储姿态失败。
- 日志归档流 failed / degraded。
- 归档文件 missing / checksum_failed。
- 恢复计划校验阻断。

第三轮前端已经聚合过这些判断，第四轮应把核心判断沉到后端，避免只在前端可见。

### 22.8 Monitor 集成改造

#### 22.8.1 AlertMessage 文案

`plugins/monitor/service/alert_service.go` 需要识别数据库资源：

- `ResourceType = database` 或 `database_backup`
- category title：`数据库备份恢复告警`
- resource label：`数据库实例`
- alert title：按 `AlertType` 返回中文标题
- metric label：按 `Metric` 返回中文指标

建议指标映射：

| metric | 中文 |
|:-----|:-----|
| `backup_failed` | 备份失败 |
| `backup_success_gap` | 成功备份间隔 |
| `rpo_lag` | RPO 延迟 |
| `restore_drill_stale` | 恢复演练过期 |
| `runner_status` | Runner 状态 |
| `barman_status` | Barman 状态 |
| `storage_posture` | 存储姿态 |
| `log_archive_status` | 日志归档状态 |

#### 22.8.2 AlertLog 写入

当前域名和主机告警已有写入逻辑。数据库告警也应统一写入 `alert_logs`：

- 发送成功：`status = success`
- 发送失败：`status = failed`
- `channel_type` 记录通道摘要
- `error_msg` 记录发送失败原因
- `resource_type` 使用 `database` 或 `database_backup`
- `resource_name` 使用实例名或资源名
- `resource_target` 使用 endpoint
- `domain_monitor_id` 置 0
- `domain` 可填实例名或 endpoint，用于兼容旧字段非空约束

#### 22.8.3 现有备份失败通知补齐

`internal/server/database/backup_alert.go` 当前只调 dispatcher。第四轮应补齐：

- 成功发送也写 `alert_logs`。
- 发送失败也写 `alert_logs`。
- `BackupSchedulerNotice` 映射为数据库告警类型。
- 后续 `RunScheduledBackupPolicy` 失败也应调用同一套通知逻辑。

### 22.9 后端 API 建议

建议新增数据库备份告警规则 API：

| 方法 | 路径 | 说明 |
|:-----|:-----|:-----|
| GET | `/api/v1/databases/backup-alert-rules` | 规则列表 |
| POST | `/api/v1/databases/backup-alert-rules` | 创建规则 |
| GET | `/api/v1/databases/backup-alert-rules/:id` | 规则详情 |
| PUT | `/api/v1/databases/backup-alert-rules/:id` | 更新规则 |
| DELETE | `/api/v1/databases/backup-alert-rules/:id` | 删除规则 |
| POST | `/api/v1/databases/backup-alert-rules/:id/test` | 测试发送 |
| GET | `/api/v1/databases/backup-alert-states` | 当前告警状态 |
| POST | `/api/v1/databases/backup-alert-states/:id/resolve` | 手动标记已处理，可选 |
| GET | `/api/v1/databases/backup-alert-summary` | 告警摘要 |

第一版可以只做：

- 规则 CRUD。
- 当前状态列表。
- 告警摘要。
- 测试发送。

手动 resolve 可以放第五轮，因为自动恢复更重要。

### 22.10 前端页面方案

#### 22.10.1 备份恢复页新增告警订阅

位置：`web/src/views/asset/DatabaseManagement.vue`

在 P5 工作区二级 Tab 中新增 `告警订阅`。

顶部摘要卡：

- `规则总数`
- `已启用`
- `当前告警`
- `24h 通知`

规则表字段：

- 规则名称
- 作用范围
- 告警类型
- 严重级别
- 告警通道
- 静默间隔
- 恢复通知
- 状态
- 更新时间
- 操作

行操作：

- 编辑
- 测试发送
- 启用 / 停用
- 删除

当前告警状态表：

- 资源
- 告警类型
- 级别
- 状态
- 最近触发
- 最近通知
- 通知次数
- 建议动作
- 操作：查看风险、打开保护详情、打开高级资源

#### 22.10.2 规则弹窗

默认字段：

- 规则名称
- 作用范围
- 告警类型
- 严重级别
- 告警通道
- 静默间隔
- 恢复通知

高级设置折叠：

- 无成功备份阈值，默认 24 小时。
- 恢复演练过期天数，默认 30 天。
- RPO 宽限分钟，默认 10 分钟。
- 是否包含非生产实例，默认否。
- 最小风险等级，默认 high。

告警类型建议用 checkbox group，按业务语言展示：

- 备份失败
- 超过阈值无成功备份
- RPO 超时
- 缺少全量基线
- 备份链异常
- 日志归档异常
- 恢复演练过期
- 恢复演练失败
- Runner 不可用
- Barman 异常
- 存储异常

#### 22.10.3 Monitor 告警日志页面

位置：`web/src/plugins/monitor/components/AlertLogs.vue`

改动：

- 资源类型筛选增加 `数据库`。
- 告警类型下拉增加数据库备份恢复告警。
- 指标翻译增加数据库指标。
- 详情弹窗中资源标签根据 `resourceType` 显示：域名、主机、数据库实例。
- 统计卡片可以暂时保持全局统计，不必第四轮拆分。

### 22.11 默认规则建议

第四轮可以在页面提供“创建推荐规则”按钮，或后端首次启动时不自动创建，只在前端给出模板。

推荐规则：

| 名称 | 范围 | 类型 | 级别 | 静默 | 恢复通知 |
|:-----|:-----|:-----|:-----|:-----|:-----|
| 生产库备份失败 | 生产实例 | 备份失败 | critical | 30 分钟 | 开启 |
| 生产库 RPO 超时 | 生产实例 | RPO 超时、日志链缺口 | critical | 30 分钟 | 开启 |
| 生产库缺少全量基线 | 生产实例 | 缺少全量基线 | critical | 6 小时 | 开启 |
| 恢复演练过期提醒 | 生产实例 | 恢复演练过期 | warning | 24 小时 | 关闭 |
| 执行资源异常 | 全部实例 | Runner 不可用、Barman 异常 | warning | 1 小时 | 开启 |
| 存储和归档异常 | 全部实例 | 存储异常、日志归档异常 | warning | 6 小时 | 开启 |

前端文案应说明：

> 推荐规则不会替代恢复演练和备份链校验，只负责在风险出现时主动通知负责人。

### 22.12 权限与审计

建议新增或复用权限：

- 查看规则：`database:backup:view`
- 创建 / 更新规则：`database:backup:update`
- 删除规则：`database:backup:delete`
- 测试发送：`database:backup:update`
- 查看告警状态：`database:backup:view`

审计动作建议：

- `database_backup_alert_rule_create`
- `database_backup_alert_rule_update`
- `database_backup_alert_rule_delete`
- `database_backup_alert_rule_test`
- `database_backup_alert_state_resolve`

测试发送必须写审计，避免用户误用告警通道。

### 22.13 第四轮分步落地建议

#### 22.13.1 第一步：告警日志和发送器兼容数据库

目标：

- Monitor 能正确展示数据库告警。
- 数据库备份调度失败写入 `alert_logs`。

改动：

- `plugins/monitor/service/alert_service.go`
- `plugins/monitor/server/alert_handler.go`
- `web/src/plugins/monitor/components/AlertLogs.vue`
- `web/src/api/alert-config.ts`
- `internal/server/database/backup_alert.go`

验收：

- 人工触发一次备份失败或测试告警，Monitor 告警日志能看到数据库资源。

#### 22.13.2 第二步：规则和状态表

目标：

- 可以配置数据库备份告警规则。
- 同一问题有持久化 firing/resolved 状态。

改动：

- `internal/biz/database/model.go`
- `cmd/server/server.go`
- `internal/biz/database/repository.go`
- `internal/data/database/backup_alert_repository.go`
- `internal/service/database/http.go`
- `internal/server/database/http.go`
- `web/src/api/database.ts`

验收：

- 规则 CRUD 正常。
- 状态列表能返回当前 firing 告警。

#### 22.13.3 第三步：后端扫描器

目标：

- 自动扫描保护风险和资源异常。
- 支持静默间隔和恢复通知。

改动：

- `internal/biz/database/backup_alert_scheduler.go`
- `internal/server/database/http.go`

验收：

- 构造一条风险，扫描器能产生 firing 状态。
- 静默期内不重复通知。
- 风险消失后状态变 resolved。

#### 22.13.4 第四步：备份恢复页告警订阅

目标：

- 用户能在备份恢复页配置规则和查看当前告警。

改动：

- `web/src/views/asset/DatabaseManagement.vue`
- 后续可拆到 `web/src/views/asset/database-backup/BackupAlertRules.vue`

验收：

- 能新增推荐规则。
- 能编辑通道、范围、类型、静默间隔。
- 能查看当前告警并跳转风险中心或保护详情。

### 22.14 测试计划

后端测试：

- `go test ./internal/biz/database -run BackupAlert`
- `go test ./plugins/monitor/...`
- `go test ./...`

重点用例：

- 规则 scope 匹配：全部、生产、指定实例、指定引擎。
- issue type 匹配：只触发规则选择的问题类型。
- fingerprint 稳定：同一问题不产生重复状态。
- 静默间隔：静默期不重复通知，过期后可再次通知。
- 恢复通知：firing 变 resolved 时按配置通知。
- alert log：成功和失败都写入。
- channel IDs：指定通道和默认全部通道都可用。

前端测试：

- `npm run typecheck`
- `npm run build`

手工验证：

- 在备份恢复页创建规则。
- 触发测试发送。
- 在 Monitor 告警日志筛选数据库。
- 停用规则后不再产生新通知。
- 删除规则后状态处理符合预期。

### 22.15 验收标准

第四轮完成后，应满足：

- 数据库备份失败能进入 Monitor 告警日志。
- Monitor 告警日志可筛选 `数据库` 资源。
- 备份恢复页可配置数据库备份恢复告警规则。
- RPO 超时、恢复演练过期、Runner / Barman / 存储异常至少能自动扫描并产生告警状态。
- 同一问题在静默期内不重复发通知。
- 问题恢复后状态可以自动变为 resolved。
- 开启恢复通知时，可以发送恢复消息。
- 所有告警发送成功和失败都有 `alert_logs` 记录。
- `go test ./...`、`npm run typecheck`、`npm run build` 通过。

### 22.16 第四轮不做的内容

第四轮先不做：

- 生产恢复审批流。
- 删除备份审批流。
- SLA 日历。
- 风险历史趋势图。
- 大规模组件拆分。
- 告警升级策略和值班排班。
- WebSocket 实时告警推送。

这些建议放到第五轮或第六轮。第四轮先把“风险能主动通知、能留痕、能去重、能恢复闭环”做扎实。

### 22.17 第四轮建议提交范围

建议实际提交包含：

1. 数据库备份告警规则模型和状态模型。
2. 数据库告警规则 CRUD API。
3. 数据库告警扫描器。
4. 备份调度失败通知补齐 alert log。
5. Monitor 告警日志支持数据库资源。
6. 备份恢复页新增告警订阅入口。
7. 本文档第四轮落地记录。

如果需要进一步控制风险，可以拆成两个提交：

1. 后端告警闭环：模型、API、扫描器、Monitor 日志兼容。
2. 前端配置入口：备份恢复页告警订阅和 Monitor 日志筛选。

## 23. 第四轮落地记录：监控告警闭环

### 23.1 改动前复查范围

第四轮动手前复查了以下代码和文档：

- `docs/database-backup-restore-ux-optimization-plan.md`：确认第四轮目标是“主动通知、状态去重、恢复闭环、Monitor 留痕”。
- `internal/server/database/backup_alert.go`：原有备份调度失败只发送 Monitor 通知，未稳定写入 `alert_logs`，也没有规则、状态和静默间隔。
- `plugins/monitor/service/alert_dispatcher.go`、`plugins/monitor/service/alert_service.go`：确认 Monitor 已有通道加载、指定通道发送、邮件/飞书/钉钉/企业微信/Webhook 发送能力，可复用。
- `plugins/monitor/model` 和告警日志页面：确认 `alert_logs` 已支持 `resource_type`、`resource_name`、`resource_target`、`metric`、`alert_rule_id`，第四轮只需要补数据库资源语义。
- `internal/biz/database/protection_profile.go`、`internal/biz/database/protection_risk.go`：确认风险中心已有缺少全量基线、增量链异常、日志链缺口、Runner、存储、恢复演练、归档延迟等风险来源。
- `internal/biz/database/backup.go`、`backup_policy*.go`、`restore*.go`、`barman.go`：确认备份记录、物理策略、Barman、恢复演练已有状态数据，可以作为告警扫描输入。
- `internal/service/database/http.go`、`internal/server/database/http.go`：确认数据库管理接口统一走实例权限和菜单权限。
- `web/src/views/asset/DatabaseManagement.vue`：确认备份恢复页已拆出风险中心、保护概览、高级资源，第四轮入口适合放在“保护概览”和“高级资源”之间。
- `web/src/api/database.ts`、`web/src/api/alert-config.ts`：确认前端已有数据库 API 和告警通道 API，可直接扩展。

### 23.2 后端已落地内容

新增数据库备份恢复告警规则和状态模型：

- `DatabaseBackupAlertRule`
- `DatabaseBackupAlertState`

新增表：

- `database_backup_alert_rules`
- `database_backup_alert_states`

自动迁移已加入 `cmd/server/server.go`，服务启动时会创建表结构。

规则能力：

- 支持启用/停用。
- 支持范围：全部实例、生产实例、指定实例、指定引擎、业务系统、负责人。
- 支持问题类型过滤。
- 支持级别、静默间隔、恢复通知、指定告警通道。
- 支持阈值 JSON：无成功备份小时数、恢复演练过期天数、RPO 宽限分钟、最低风险等级、是否纳入非生产。

状态能力：

- 使用 fingerprint 去重同一规则、同一资源、同一问题。
- 支持 `firing` 和 `resolved`。
- 记录首次触发、最近触发、最近通知、通知次数。
- 静默期内不重复通知。
- 风险消失后自动恢复为 `resolved`。
- 规则开启恢复通知时发送恢复消息。

扫描器：

- 新增 `BackupAlertScheduler`，默认 5 分钟扫描一次。
- 服务后台启动和停止生命周期已接入 `internal/server/database/http.go`。
- 扫描来源包含保护风险、保护概览和近期失败备份记录。

Monitor 留痕：

- 备份调度失败现在使用 `resource_type=database_backup` 写入 `alert_logs`。
- 新规则产生的告警发送成功或失败都会写入 `alert_logs`。
- 支持指定通道发送；未指定通道时使用全部启用通道。
- Monitor 消息标题、分类、资源标签、指标翻译已支持数据库备份恢复。

审计：

- 规则创建、更新、删除、测试发送会写入数据库查询审计。
- 新增审计动作：`backup_alert_rule_create`、`backup_alert_rule_update`、`backup_alert_rule_delete`、`backup_alert_rule_test`。

### 23.3 前端已落地内容

备份恢复页新增二级页签：

- `告警订阅`

入口位置：

- `备份与恢复 -> 保护概览工作台 -> 告警订阅`
- 位于“保护概览”和“高级资源”之间，避免普通用户直接陷入底层 Runner、Barman、WAL 配置。

告警订阅页包含：

- 汇总指标：启用规则、当前告警、严重/高危、24 小时通知。
- 规则列表：规则、范围、问题、级别/静默、通道、阈值、操作。
- 当前告警状态列表：实例/对象、问题/级别、消息、建议、首次/最近、通知次数。
- 规则弹窗：名称、启用、范围、问题类型、级别、静默间隔、恢复通知、告警通道、关键阈值、备注。

Monitor 告警日志页面增强：

- 资源类型筛选增加“数据库备份”。
- 告警类型增加数据库备份恢复告警。
- 指标翻译增加备份失败、成功备份间隔、RPO 延迟、恢复演练、Runner 状态、存储姿态、日志归档状态。
- 详情弹窗资源标签可显示“数据库备份”。

### 23.4 本轮实际未做内容

第四轮没有做以下内容，建议后续轮次处理：

- 告警升级策略和值班排班。
- 告警状态手动确认、认领、关闭。
- 告警历史趋势图。
- WebSocket 实时推送。
- 默认规则自动初始化。
- 从告警状态一键跳转到具体风险修复动作。

当前先保证规则、扫描、发送、去重、恢复、日志和审计闭环。

### 23.5 验收命令

已执行：

```bash
go test ./...
npm run typecheck
```

后续完整发布前继续执行：

```bash
npm run build
git diff --check
docker build -t opshub-api:latest -f Dockerfile .
docker build -f Dockerfile.frontend -t opshub-web:latest .
docker compose up -d --force-recreate frontend backend
```

## 24. 第五轮落地记录：副本角色与继承保护

本轮目标是解决“从库 / 延迟从库被保护概览误判为未备份严重风险”的问题，并给页面补一个明确的人工标记入口。用户看到的现象是 `mysql-delay-replica-2`、`opshub-mysql-replica` 这类实例在备份恢复页被提示“当前实例没有启用备份策略或成功备份记录”，但这些对象本质上是主库的副本，不一定应该独立承担备份基线责任。

### 24.1 改动前复查范围

本轮动手前复查了以下文档、代码和现有数据链路：

- `docs/database-backup-restore-ux-optimization-plan.md`：确认前三轮已经把备份恢复默认入口收敛到保护概览，第四轮补了监控告警闭环；副本角色还没有进入保护概览的风险判断。
- `internal/biz/database/model.go`：确认已有 `DatabaseInstanceReplica`、`DatabaseReplicationCheck`、副本角色、健康状态、发现来源和副本保护相关常量。
- `internal/biz/database/replication.go`：确认已有副本采集、关系 upsert、延迟秒数、Apply 状态、事故向导和副本保护能力。
- `internal/data/database/replication_repository.go`：确认已有 `UpsertByReplicaInstance`，但自动检测会覆盖同一 `replica_instance_id` 的关系信息，本轮需要保护人工标记。
- `internal/biz/database/protection_profile.go`：确认保护概览只按实例自身的备份策略、逻辑任务、备份记录、日志归档、Runner、存储、Barman、恢复演练等判断保护级别，没有读取 `database_instance_replicas`。
- `internal/biz/database/protection_risk.go`：确认风险中心来自保护概览拆解，因此保护概览误判会直接进入风险中心。
- `internal/biz/database/backup_alert_rule.go`：确认第四轮告警扫描会从保护概览生成“长时间无成功备份”候选，因此从库误判会进一步触发告警。
- `internal/service/database/replication.go`、`internal/server/database/http.go`：确认副本关系已有查询和删除接口，可以扩展人工标记接口并复用副本治理权限。
- `web/src/api/database.ts`：确认前端已有副本关系列表、采集、保护和备份恢复 API 定义。
- `web/src/views/asset/DatabaseManagement.vue`：确认保护概览、风险中心、保护详情抽屉、副本治理都集中在同一页面，本轮需要同时调整展示和操作入口。

同时复查了当前库里的副本关系：`opshub-mysql-replica` 已有实时从库关系，`mysql-delay-replica-2` 已有延迟从库关系且配置延迟为 3600 秒；`mysql-tmp-test` 当前检测为主库 / 独立实例，因此仍应按普通实例检查是否有备份保护。

### 24.2 改动前的问题判断

原逻辑的问题不在备份引擎本身，而在“保护责任”没有建模：

- 主库 / 独立实例：应该要求有自己的物理备份、逻辑备份或外部备份记录。
- 实时从库：通常可以继承主库备份保护，不应默认要求单独备份。
- 延迟从库：更偏向恢复缓冲和误操作保护，也可以继承主库备份保护；它自身需要展示延迟窗口和健康状态，而不是被简单标红为“没有备份”。
- Standby：PostgreSQL 场景下也应按来源主库保护状态判断，而不是把 standby 当成普通主库。
- 未识别角色：仍按普通实例处理，避免漏掉真正没有备份的生产库。

因此本轮的核心不是隐藏风险，而是把风险指向正确责任方：从库继承主库保护时不再报“从库没有备份”；如果来源主库没有保护，则从库显示“等待来源主库启用备份保护”。

### 24.3 后端已落地内容

保护概览新增副本角色和备份责任字段：

- `instanceRole` / `instanceRoleText`：主库、从库、延迟从库、Standby、未知。
- `replicaRole` / `replicaRoleText`：副本关系中的具体角色。
- `primaryInstanceId`、`primaryInstanceName`、`primaryEndpoint`：来源主库。
- `roleDiscoverySource` / `roleDiscoverySourceText`：人工登记、副本状态、主库状态或推断。
- `configuredDelaySeconds`、`remainingDelaySeconds`：延迟从库的配置延迟和剩余延迟。
- `backupRequirement` / `backupRequirementText`：需单独保护、继承主库保护或不强制单独备份。
- `inheritedProtection` / `inheritedProtectionText`：是否已从来源主库继承到有效保护。
- `inheritedProtectionMode`、`inheritedProtectionLevel`、`inheritedProtectionRiskLevel`：来源主库的保护摘要。

保护概览评估逻辑调整：

- 构建每个实例保护概览时先读取 `database_instance_replicas`，没有关系时再参考最近一次副本检测。
- 如果实例是实时从库、延迟从库或 Standby，则把备份责任设为 `inherited`。
- 若来源主库已有保护级别，则从库继承主库保护级别、可恢复窗口和最近备份 / 日志时间。
- 已继承保护的从库不再产生“当前实例没有启用备份策略或成功备份记录”的严重风险。
- 若来源主库没有可继承保护，则产生“当前实例是从库，但来源主库尚未启用可继承的备份保护”的严重风险。
- 延迟副本保护窗口风险只对需要自身保护的实例检查，避免和继承保护重复告警。

风险中心同步扩展：

- 风险项返回实例角色、来源主库、备份责任和继承保护文案。
- 风险中心可以直接解释“这是从库继承主库保护”或“来源主库未保护”，而不是只展示一条孤立红色风险。

告警扫描调整：

- `no_recent_success_backup` 候选扫描会跳过“已继承主库保护”的从库。
- 来源主库没有保护时仍会保留风险，告警应该指向保护主库，而不是要求每个从库单独建备份。

人工标记能力：

- 新增 `POST /api/v1/databases/replicas/mark`。
- 支持选择来源主库、从库实例、副本角色和延迟秒数。
- 支持实时从库、延迟从库和 Standby。
- 复用副本治理权限，操作写入查询审计，审计动作是 `replica_relation_upsert`。
- 主从实例必须存在、不能相同、数据库类型必须一致，且当前限制在 MySQL、MariaDB、PostgreSQL。

人工标记保护：

- `UpsertByReplicaInstance` 增加人工登记保护逻辑。
- 当现有关系来源是 `manual` 时，后续自动采集只更新健康状态、最近检测、错误信息等运行态字段。
- 自动采集不会覆盖人工登记的来源主库、副本角色、发现来源和延迟秒数。
- 用户再次通过人工标记保存时，可以主动更新这些关系字段。

### 24.4 前端已落地内容

保护概览表：

- 实例列增加角色标签：主库 / 独立实例、从库、延迟从库、Standby。
- 从库行显示来源主库名称，降低“这个实例为什么不要求单独备份”的理解成本。
- 保护模式列展示“继承主库保护”或“需单独保护”。
- 恢复演练 / 副本列补充副本角色和延迟秒数。
- 汇总卡从“已保护实例”调整为更接近责任口径的“保护对象”，把继承保护数量纳入说明。

风险中心：

- 风险实例列展示副本角色和继承保护文案。
- 从库继承主库保护后不再把无自身备份显示为严重风险。
- 来源主库未保护时，风险文案指向主库保护缺失。

保护详情抽屉：

- 新增“实例角色”和“备份责任”。
- 恢复证明区域展示副本角色和配置延迟。
- 新增“标记角色”按钮，便于从当前保护详情直接修正主从关系。

人工标记弹窗：

- 入口在保护概览更多操作和保护详情抽屉。
- 支持选择从库实例、来源主库、副本角色和延迟秒数。
- 延迟秒数字段只在选择“延迟从库”时显示。
- 保存后刷新保护概览、风险中心、副本关系和副本保护。

### 24.5 当前实例的预期效果

按本轮逻辑，当前几个实例应按以下方式展示：

- `opshub-mysql-replica`：展示为从库，备份责任为继承 `opshub-mysql` 的保护，不再因为自身没有备份策略被标为严重。
- `mysql-delay-replica-2`：展示为延迟从库，显示配置延迟 3600 秒，备份责任为继承来源主库保护。
- `mysql-tmp-test`：如果仍检测为主库 / 独立实例，继续要求自身启用备份保护；除非人工确认它实际是从库并完成标记。

### 24.6 本轮不做的内容

本轮先不做以下扩展：

- 不新增“忽略备份保护”开关，避免用户误把真正的主库风险隐藏掉。
- 不自动修改实例名称或标签，只在保护概览和风险中心展示运行角色。
- 不把从库恢复演练自动转成主库恢复演练任务，恢复流程仍走现有演练入口。
- 不做副本关系审批流；人工标记目前直接生效并写审计。
- 不新增数据库表字段，人工标记复用现有 `database_instance_replicas.discovery_source=manual`。

### 24.7 验收命令

本轮已执行：

```bash
go test ./...
npm run typecheck
npm run build
git diff --check
```

容器发布执行：

```bash
docker build -t opshub-api:latest -f Dockerfile .
docker build -f Dockerfile.frontend -t opshub-web:latest .
docker compose up -d --force-recreate frontend backend
```

说明：`npm run build` 仍有 Sass legacy JS API 和大 chunk 体积提示，这是项目现有打包提示，不影响本轮构建结果。

## 25. 第六轮详细方案：副本自动采集与拓扑新鲜度

本轮建议解决“副本关系和拓扑必须手动点采集才变新鲜”的问题。当前用户在拓扑页看到“副本状态采集已过期”，需要先点 `采集相关实例` 或到副本治理里点 `全量采集`，再刷新拓扑，使用成本偏高，也容易让保护概览和拓扑状态长时间停留在旧数据。

### 25.1 改动前复查结论

本轮方案基于以下代码和文档复查：

- `web/src/views/asset/DatabaseManagement.vue`
  - 拓扑页目前有 `采集当前实例`、`采集相关实例`、`刷新拓扑` 三个按钮。
  - `采集相关实例` 会从当前拓扑节点中提取实例 ID，再逐个调用 `checkDatabaseReplication`。
  - 副本治理页的 `全量采集` 也是前端循环所有 MySQL / MariaDB / PostgreSQL 实例逐个调用采集接口。
- `web/src/api/database.ts`
  - 当前只有单实例采集 API：`POST /api/v1/databases/instances/{id}/replication-check`。
  - 没有批量采集、只采过期实例、采集任务状态等后端接口。
- `internal/biz/database/replication.go`
  - `CheckInstanceReplication` 会真正采集并落库。
  - 成功采集后会写入 `database_replication_checks`。
  - 当检测到当前实例是 replica / standby 时，会更新 `database_instance_replicas`。
  - 上一轮已经加了人工登记保护，自动采集不会覆盖 `discovery_source=manual` 的主从关系、角色和延迟秒数。
- `internal/biz/database/topology.go`
  - `GetTopology` 会实时采集当前选中实例，用于返回拓扑结果。
  - 相关副本节点仍依赖最近一次已落库的 `database_replication_checks` 和 `database_instance_replicas.last_checked_at`。
  - 新鲜度阈值当前写死为：
    - 5 分钟内：`fresh`
    - 30 分钟内：`warning`
    - 超过 30 分钟：`stale`
  - `stale` 会生成“副本状态采集已过期”的拓扑风险。
- `internal/server/database/http.go`
  - 当前后台调度器只有备份调度器、备份告警调度器、容量采样调度器。
  - 没有副本状态自动采集调度器。
- `docs/database-large-backup-pitr-plan.md`
  - P4 副本治理原设计只要求“手动刷新单实例和全量刷新”。
  - 没有补齐自动采集、批量采集和拓扑自动补采。

因此，当前问题不是单个按钮文案问题，而是副本运行态数据没有后台保鲜机制。拓扑页实时采集当前实例，但不会把相关副本都补采并落库；副本关系表和保护概览依赖落库结果，所以用户必须手动采集。

### 25.2 本轮目标

第六轮目标：

- 用户进入拓扑页、备份恢复页、保护概览时，不需要先理解“采集”和“刷新”的区别。
- 系统后台自动保持 MySQL / MariaDB / PostgreSQL 副本状态新鲜。
- 拓扑页刷新时自动补采过期节点。
- 手动采集保留为排障动作，而不是主路径必选动作。
- 自动采集必须受控，不能对所有数据库造成突发连接压力。
- 自动采集不能破坏人工标记的副本关系。
- 采集失败要可见、可审计、可用于告警，但不能影响其他实例采集。

### 25.3 总体方案

建议分三层落地：

1. 后端新增 `ReplicationCheckScheduler`，定时自动采集副本状态。
2. 后端新增批量采集接口，给拓扑页、副本治理页和后续告警扫描复用。
3. 前端把拓扑页主流程改成“自动补采 + 刷新拓扑”，手动按钮降级为排障入口。

这三层需要一起做。只做前端自动点按钮，仍然会把批量循环、失败聚合、并发控制留在浏览器里；只做后台调度，用户刚添加实例或刚修复链路时仍可能要等下一轮。因此推荐后台调度和页面触发补采都做。

### 25.4 后端自动采集调度器

新增文件建议：

- `internal/biz/database/replication_check_scheduler.go`

新增结构：

```go
type ReplicationCheckSchedulerOptions struct {
    Interval              time.Duration
    StaleAfter            time.Duration
    MaxConcurrency        int
    BatchSize             int
    IncludePrimary        bool
    IncludeNonProduction  bool
}

type ReplicationCheckScheduler struct {
    useCase *UseCase
    interval time.Duration
    staleAfter time.Duration
    maxConcurrency int
    batchSize int
}
```

推荐默认值：

- `Interval`: 5 分钟。
- `StaleAfter`: 5 分钟。
- `MaxConcurrency`: 2。
- `BatchSize`: 200。
- `IncludePrimary`: true。
- `IncludeNonProduction`: true，首版建议纳入所有启用实例，后续再配置化。

调度器行为：

1. 服务启动后先延迟 30 秒再跑第一轮，避免启动期和迁移、缓存预热抢资源。
2. 每 5 分钟扫描一次启用中的 MySQL / MariaDB / PostgreSQL 实例。
3. 只采集以下实例：
   - 没有最近采集记录。
   - 最近采集超过 `StaleAfter`。
   - 当前已有 `database_instance_replicas` 关系且关系过期。
   - 最近检测为 replica / standby 的实例。
   - 已人工标记为 replica / delayed / standby 的实例。
4. 每个实例独立超时，沿用现有 `replicationCheckTimeout=15s`。
5. 单个实例采集失败不终止整轮。
6. 自动采集写入审计时使用系统操作人，例如：
   - `OperatorID=0`
   - `OperatorName=system`
   - `ClientIP=replication-scheduler`
7. 日志里输出本轮汇总：
   - 成功数量。
   - 失败数量。
   - 跳过数量。
   - 耗时。

需要拆出的 UseCase 内部方法：

```go
func (uc *UseCase) RunReplicationCheck(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseReplicationCheck, error)
```

原因：现在 `CheckInstanceReplication` 直接返回 VO 给 HTTP 使用。调度器和批量接口都应该复用同一条“采集、落库、更新关系、审计”的业务链路，避免出现三套采集逻辑。

### 25.5 后端批量采集接口

新增 API：

```text
POST /api/v1/databases/replication-checks/batch
```

权限：

- `database:replica:check`

请求建议：

```json
{
  "instanceIds": [1, 45, 46],
  "onlyStale": true,
  "staleSeconds": 300,
  "includeRelated": true,
  "maxConcurrency": 2
}
```

字段说明：

- `instanceIds`：指定采集实例。为空时可按权限范围采集全部支持实例。
- `onlyStale`：只采集过期或无采集记录的实例。
- `staleSeconds`：过期阈值，默认 300 秒。
- `includeRelated`：自动扩展采集相关主库和副本。
- `maxConcurrency`：本次请求并发上限，不能超过后端硬上限。

返回建议：

```json
{
  "success": 2,
  "failed": 0,
  "skipped": 1,
  "items": [
    {
      "instanceId": 1,
      "instanceName": "opshub-mysql",
      "status": "success",
      "checkedAt": "2026-05-05 03:20:00",
      "message": "采集完成"
    }
  ]
}
```

批量接口的价值：

- 前端不再逐个循环请求。
- 后端统一做权限过滤、并发限制、失败聚合。
- 后续备份保护、告警扫描、拓扑刷新都可以复用。
- 批量采集可以只采 stale 节点，减少数据库压力。

### 25.6 拓扑页交互优化

当前按钮：

- `采集当前实例`
- `采集相关实例`
- `刷新拓扑`

建议调整为：

- 主按钮：`刷新拓扑`
- 次按钮：`采集状态`
- 更多菜单：
  - `采集当前实例`
  - `采集相关实例`
  - `查看最近采集`

主按钮 `刷新拓扑` 的内部流程：

1. 调用 `GET /instances/{id}/topology`。
2. 读取返回节点和边里的 `metrics.check_freshness`。
3. 如果存在 `stale` 或 `unknown`，并且用户有 `replicaCheck` 权限：
   - 调用批量采集接口。
   - 采集对象包括当前实例、拓扑节点中的实例、边上的 replica instance。
   - 只采过期节点。
4. 采集成功后再次调用 `GET /instances/{id}/topology`。
5. 页面显示最终拓扑。

如果用户没有 `replicaCheck` 权限：

- 仍允许刷新拓扑。
- 不自动采集。
- 风险提示文案改为：
  - `副本状态已过期，需要副本采集权限才能刷新运行态。`

加载体验：

- 第一阶段 loading：`读取拓扑`
- 第二阶段 loading：`补采过期副本状态`
- 第三阶段 loading：`刷新拓扑`

这样用户感知上仍然是一个按钮，而不是必须理解两个动作。

### 25.7 副本治理页优化

副本治理页建议保留手动能力，但定位改成“排障和立即验证”：

- `全量采集` 改名为 `立即采集全部`。
- 增加 `只采过期` 开关，默认开启。
- 副本关系表增加自动采集状态：
  - 最近自动采集时间。
  - 最近采集来源：`manual / scheduler / topology_auto_refresh`。
  - 采集新鲜度：`新鲜 / 即将过期 / 已过期 / 无记录`。
- 最近采集表增加触发来源字段：
  - `manual`
  - `scheduler`
  - `batch`
  - `topology_auto_refresh`

如果不想改表结构，可以先从审计和 `raw_status_json` 里弱表达；但推荐后续给 `database_replication_checks` 增加 `trigger_source` 字段，查询和排障更直接。

### 25.8 数据模型建议

首版可以不新增表，只复用：

- `database_replication_checks`
- `database_instance_replicas`

但为了区分自动采集和手动采集，建议新增字段：

在 `database_replication_checks` 增加：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `trigger_source` | varchar(40) | `manual / scheduler / batch / topology_auto_refresh` |
| `operator_id` | uint | 手动触发时的用户 ID，系统触发为 0 |
| `operator_name` | varchar(100) | 操作人，系统触发为 `system` |

不建议新增“自动采集任务表”，除非后续要做长时间任务队列、任务取消、任务进度。如果第六轮只做轻量后台调度和批量接口，直接写入采集记录即可。

### 25.9 配置建议

自动副本采集需要配置化，但第六轮可以先后端默认开启，再把配置放到数据库管理系统配置中。

建议配置项：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `replicationAutoCollectEnabled` | true | 是否启用自动副本采集 |
| `replicationAutoCollectIntervalSeconds` | 300 | 自动采集间隔 |
| `replicationStaleAfterSeconds` | 300 | 超过多久视为需要补采 |
| `replicationTopologyStaleSeconds` | 1800 | 超过多久拓扑展示为过期 |
| `replicationMaxConcurrency` | 2 | 自动采集最大并发 |
| `replicationBatchSize` | 200 | 单轮最多扫描实例数 |
| `replicationCollectTimeoutSeconds` | 15 | 单实例采集超时 |

当前 `topologyCheckFreshSeconds` 和 `topologyCheckStaleSeconds` 是硬编码，建议第六轮先保留默认值，同时把文案和调度器阈值对齐；第七轮再做系统配置页面。

### 25.10 告警和风险联动

第四轮已经有备份恢复告警闭环。第六轮可先只做采集保鲜，不立即新增告警类型。但建议预留以下候选：

- `replica_check_failed`：副本状态采集失败。
- `replica_check_stale`：副本状态长时间未刷新。
- `replica_lag_high`：复制延迟超过阈值。
- `replica_apply_stopped`：MySQL SQL 线程停止或 PostgreSQL replay 暂停。
- `replica_source_unmatched`：来源主库无法匹配。

第六轮至少要做到：

- 采集失败写入 `database_replication_checks.error_message`。
- `database_instance_replicas.last_error` 更新为最近错误。
- 拓扑风险和副本治理页能看到失败原因。

第七轮再把这些接入 `BackupAlertScheduler` 或新增数据库副本告警规则。

### 25.11 权限和安全

自动采集本质上会连接数据库并执行只读状态查询，需要注意：

- 调度器只能采集 OpsHub 已启用实例。
- 仍然复用实例凭据，不新增数据库账号管理逻辑。
- 自动采集执行的 SQL 必须保持只读：
  - MySQL / MariaDB: `SHOW REPLICA STATUS`、`SHOW SLAVE STATUS`、必要变量读取。
  - PostgreSQL: `pg_is_in_recovery()`、`pg_stat_replication`、`pg_stat_wal_receiver`、LSN 和 replay timestamp 查询。
- 自动采集不执行：
  - `STOP REPLICA`
  - `START REPLICA`
  - `pg_wal_replay_pause`
  - promote / failover / switchover
- 采集原始结果继续脱敏：
  - password
  - conninfo password
  - token / secret 类字段

人工触发批量采集时，仍然检查 `database:replica:check` 权限；自动调度器以系统身份执行，不受单个用户权限影响，但只扫描已纳管实例。

### 25.12 失败处理

自动采集失败策略：

- 单实例失败：记录失败，不影响其他实例。
- 某一轮失败数量过多：后端日志 warning。
- 数据库连接超时：写入采集记录，健康状态 `unknown` 或 `warning`。
- 凭据缺失：写入错误，提示修复凭据。
- 不支持引擎：跳过，不写错误。
- 实例禁用：跳过。
- 手工标记关系存在但采集失败：保留人工关系，只更新 `last_error` 和健康状态。

批量接口返回时要区分：

- `success`
- `failed`
- `skipped`

跳过不应该算失败，例如实例未过期、实例禁用、数据库类型不支持。

### 25.13 前端文案建议

拓扑风险文案调整：

当前：

- `副本状态采集已过期`
- `点击采集相关实例刷新当前拓扑中的主库和副本状态。`

建议：

- 自动采集已开启但本节点过期：
  - 标题：`副本状态等待自动刷新`
  - 描述：`最近一次采集距今约 2h55m，系统会自动刷新；也可以立即采集。`
  - 动作：`立即采集`
- 自动采集失败：
  - 标题：`副本状态自动采集失败`
  - 描述：展示最近错误。
  - 动作：`查看采集详情`
- 用户无采集权限：
  - 标题：`副本状态已过期`
  - 描述：`当前账号没有副本采集权限，只能查看最近一次采集结果。`
  - 动作：无。

按钮文案：

- `刷新拓扑`：主按钮，自动补采必要运行态。
- `立即采集`：只在风险卡片或更多菜单里出现。
- `查看最近采集`：跳转副本治理最近采集表。

### 25.14 分阶段落地建议

第六轮建议按 4 个小步走：

1. 后端调度器
   - 新增 `ReplicationCheckScheduler`。
   - 接入 `HTTPServer.StartBackground` 和 `StopBackground`。
   - 每 5 分钟采集过期实例。
   - 补单测。

2. 批量采集接口
   - 新增 request / response。
   - 后端统一并发、权限、失败聚合。
   - 前端副本治理页 `全量采集` 改用批量接口。

3. 拓扑自动补采
   - `刷新拓扑` 自动识别 stale / unknown 节点。
   - 有权限则调用批量接口补采。
   - 补采后自动刷新拓扑。
   - 手动采集按钮降级。

4. 文案和状态优化
   - 拓扑风险文案区分自动采集开启、失败、无权限。
   - 副本治理表展示采集新鲜度。
   - 文档补充运行说明和验收记录。

### 25.15 验收标准

功能验收：

- 新启动服务后，不手动点击采集，副本关系的 `last_checked_at` 会自动刷新。
- 拓扑页打开后，如果副本节点过期，系统自动补采并刷新拓扑。
- `opshub-mysql-replica` 和 `mysql-delay-replica-2` 不再长期停留在 `2h55m` 这种过期状态。
- 手工标记的延迟从库仍保持 `manual` 关系和 3600 秒延迟配置，不被自动采集覆盖。
- 批量采集接口能返回成功、失败、跳过明细。
- 单个实例凭据错误时，其他实例仍能采集成功。

安全验收：

- 自动采集不执行任何暂停、恢复、切换、提升命令。
- 原始采集结果继续脱敏。
- 批量接口必须校验 `database:replica:check` 权限。

性能验收：

- 默认并发不超过 2。
- 采集 10 个实例时不会阻塞页面请求。
- 后端日志可看到每轮自动采集汇总。

回归验收：

```bash
go test ./...
npm run typecheck
npm run build
git diff --check
```

发布验收：

```bash
docker build -t opshub-api:latest -f Dockerfile .
docker build -f Dockerfile.frontend -t opshub-web:latest .
docker compose up -d --force-recreate frontend backend
docker compose ps backend frontend mysql redis
```

### 25.16 本轮不建议做的内容

第六轮先不做：

- 不做副本切换、提升、自动故障转移。
- 不把采集任务做成长任务中心。
- 不引入复杂 cron 表达式。
- 不做每个实例独立采集计划。
- 不接入值班升级和副本告警规则。
- 不改数据库真实复制配置。

第六轮重点是把“副本状态自动保鲜”和“拓扑刷新不需要手动采集”做扎实。

## 26. 第六轮落地记录：副本自动采集与拓扑自动补采

本轮按第 25 节方案落地，目标是解决“副本状态必须手动点采集”的体验问题，并把采集动作从前端循环请求下沉到后端统一控制。

### 26.1 本轮改动前复查结论

改动前相关入口和问题：

- 拓扑页的 `采集当前实例`、`采集相关实例`、`刷新拓扑` 位于 `web/src/views/asset/DatabaseManagement.vue`。
- `collectTopologyReplicationChecks` 由前端逐个调用 `checkDatabaseReplication(id)`，页面负责串行循环、失败计数和刷新。
- 副本治理页的 `全量采集` 同样遍历 `replicationInstances`，逐个调用单实例采集接口。
- 后端只有单实例接口 `POST /api/v1/databases/instances/:id/replication-check`，没有批量采集接口。
- `internal/biz/database/replication.go` 的采集逻辑只在手动请求时运行，采集结果会写入 `database_replication_checks` 并更新副本关系。
- `internal/biz/database/topology.go` 能判断 `check_freshness=normal/warning/stale/unknown`，但只是展示风险，不会主动触发补采。
- `internal/server/database/http.go` 已有备份调度器、备份告警调度器和容量调度器，但没有副本采集调度器。

因此，界面出现 `副本状态采集已过期` 时，用户必须再手动点 `采集相关实例`；如果忘记点击，拓扑和副本治理长期显示旧数据。

### 26.2 后端落地内容

新增能力：

- 新增 `POST /api/v1/databases/replication-checks/batch` 批量采集接口。
- 新增 `DatabaseReplicationCheckBatchRequest`、`DatabaseReplicationCheckBatchResultVO`、`DatabaseReplicationCheckBatchItemVO`。
- 批量接口支持：
  - 指定实例 ID 列表；
  - 只采过期实例；
  - 自动带上相关主库/从库；
  - 后端并发控制，默认 2，最大 5；
  - 成功、失败、跳过明细返回；
  - 权限作用域过滤，沿用数据库实例权限。
- 单实例采集逻辑抽到 `runReplicationCheck`，批量接口和原手动接口共用同一套采集、落库、关系更新、审计逻辑。
- 新增 `ReplicationCheckScheduler`，服务启动后自动运行：
  - 初始延迟 45 秒；
  - 每 5 分钟扫描一次；
  - 只采集超过 5 分钟新鲜窗口的实例；
  - 只做只读状态采集，不执行 pause/resume/promote/failover。

新增采集来源字段：

- `manual`：手动采集。
- `batch`：副本治理页批量采集。
- `scheduler`：后台自动调度。
- `topology_auto_refresh`：拓扑刷新时自动补采。

数据库模型增加字段：

- `database_replication_checks.trigger_source`
- `database_replication_checks.operator_id`
- `database_replication_checks.operator_name`

这些字段用于区分一条采集记录到底来自人工、页面批量、拓扑自动补采还是后台调度。

### 26.3 前端落地内容

拓扑页：

- `刷新拓扑` 现在会先读取拓扑。
- 如果拓扑节点或链路的 `check_freshness` 是 `stale` 或 `unknown`，并且当前用户有 `database:replica:check` 权限，前端会自动调用批量采集接口。
- 自动补采成功或失败后，拓扑会再刷新一次。
- 原来的 `采集相关实例` 改为 `立即采集相关实例`，作为人工兜底按钮保留。
- 自动补采不再由前端逐个请求实例，而是一次批量请求交给后端并发控制。

副本治理页：

- `重新采集`、`全量采集` 统一改为 `立即采集`。
- 新增 `只采过期 / 采集全部` 开关，默认 `只采过期`，避免每次都打所有实例。
- 最近采集表新增 `触发来源` 列，能看出记录是手动、批量、调度还是拓扑自动补采产生。
- 全局采集不再前端串行循环，改用后端批量接口返回汇总。

### 26.4 本轮涉及主要文件

- `internal/biz/database/model.go`
- `internal/biz/database/usecase.go`
- `internal/biz/database/replication.go`
- `internal/biz/database/replication_check_scheduler.go`
- `internal/service/database/http.go`
- `internal/service/database/replication.go`
- `internal/server/database/http.go`
- `web/src/api/database.ts`
- `web/src/views/asset/DatabaseManagement.vue`

### 26.5 已完成验证

本轮代码验证命令：

```bash
go test ./...
npm run typecheck
npm run build
git diff --check
```

验证结果：

- Go 全量测试通过。
- 前端类型检查通过。
- 前端生产构建通过。
- `git diff --check` 通过。
- 前端构建仍有既有 Sass legacy JS API 提示和 chunk size 提示，不影响构建结果。

### 26.6 预期效果

上线后应该看到：

- 不再必须手动点 `采集` 才能让副本状态更新。
- 后台会周期性刷新过期副本状态。
- 打开拓扑页时，过期节点会自动补采并刷新。
- 副本治理的批量采集速度更稳定，失败不会中断其他实例。
- 最近采集记录能追踪触发来源，排查“是谁触发的采集”更直接。

### 26.7 后续可继续做的内容

第六轮先把自动保鲜打通，后续如果继续优化，可以再做：

- 在保护概览卡片里直接展示“自动采集运行中 / 最近自动采集时间”。
- 给副本状态采集接入监控告警，把调度失败、长期 stale、延迟超阈值推到监控告警中心。
- 增加批量采集历史页，专门查看每轮批量任务的明细。
- 给调度间隔、新鲜窗口、并发数做系统配置项。
- 拓扑页增加“本次自动补采了哪些实例”的轻量提示。
