# 数据库管理四期详细拆分

## 目标
1. 在一二三期的实例、元数据、查询审计、受控写入和备份闭环基础上，补齐复杂数据库形态的只读运维能力。
2. 优先支持拓扑可视化、恢复演练、容量趋势和巡检报告，避免直接扩大高风险写操作面。
3. 对国产数据库先做协议兼容接入，再按真实差异逐步增强元数据、诊断和备份能力。

## 一二三期代码回顾结论
1. 一期主线：
   - `internal/biz/database/model.go` 已定义数据库实例、Schema、表、字段、索引、同步任务和查询审计模型。
   - `internal/biz/database/usecase.go` 已形成实例 CRUD、连接测试、元数据同步、只读查询和审计入口。
   - `internal/server/database/http.go` 已统一接入数据库管理路由和 RBAC 权限。
   - `web/src/views/asset/DatabaseManagement.vue` 已承载实例管理、结构浏览、SQL 控制台和审计页签。
2. 二期主线：
   - `postgresql.go`、`sqlserver.go`、`clickhouse.go`、`oracle.go` 已把多类型 SQL 数据库拆到独立适配文件。
   - `diagnosis.go` 已提供指标、会话、慢 SQL 三类只读诊断入口，当前主要覆盖 MySQL / MariaDB / PostgreSQL。
   - 前端已有诊断页签，可复用实例选择、刷新、卡片和表格展示模式。
3. 三期主线：
   - `query_write.go`、`sql_safety.go` 已形成写操作风险识别、预检查、确认和审计闭环。
   - `backup.go`、`backup_execution.go`、`backup_runtime.go`、`backup_scheduler.go` 已形成备份任务、真实逻辑备份、定时调度、保留清理和失败通知。
   - 备份任务和记录已接入页面、审计和下载链路。
4. 四期应沿用现有边界：
   - 复杂拓扑、容量趋势、恢复演练继续放在 `internal/biz/database/` 下按能力拆文件。
   - 所有用户触发的只读运维动作仍写入 `database_query_audits`，通过 `audit_action` 区分。
   - 前端继续收口在数据库管理页，先做可用视图，再逐步增强交互。

## 范围拆分

### A. 集群拓扑
1. Redis Cluster：
   - 展示节点 ID、地址、主从角色、连接状态、slots 范围。
   - 非 Cluster 模式返回单机 / 主从信息和明确提示。
2. MongoDB ReplicaSet：
   - 展示副本集名称、成员地址、PRIMARY / SECONDARY / ARBITER 等角色、健康状态、延迟信息。
   - 非副本集返回 `hello` 基础信息和明确提示。
3. Elasticsearch / OpenSearch：
   - 展示集群健康、节点角色、版本。
   - 展示索引分片列表，含主副分片、状态、节点、文档数和容量。

### B. 国产数据库接入
1. TiDB：按 MySQL 协议优先接入连接测试、元数据、只读查询、诊断和备份后续增强。
2. OceanBase：先按 MySQL 模式接入，Oracle 模式留到专门批次。
3. openGauss：按 PostgreSQL 协议优先接入连接测试、元数据和只读查询。
4. 人大金仓 Kingbase：按 PostgreSQL 兼容协议优先接入。
5. 达梦 Dameng：先完成类型注册和实例纳管，专用驱动、元数据和查询放到后续批次。

### C. 恢复演练
1. 新增恢复演练任务，不覆盖生产实例。
2. 仅允许选择非生产目标实例。
3. 从成功备份记录发起 dry-run / restore rehearsal。
4. 记录目标实例、备份记录、执行耗时、状态、错误信息和审计。
5. 首批仅支持 MySQL / MariaDB / PostgreSQL 逻辑备份恢复演练。

### D. 容量趋势
1. 定时采集实例、Schema、表级容量快照。
2. 聚合展示近 24 小时、7 天、30 天趋势。
3. 对接巡检报告使用相同采样数据。

### E. 巡检报告
1. 报告维度：
   - 容量：总容量、增长、Top 对象。
   - 性能：连接、活跃会话、慢 SQL。
   - 安全：高风险 SQL、失败登录 / 凭据状态可后续纳入。
   - 备份：最近成功、失败次数、保留清理状态。
2. 先提供手动生成接口，后续再做定时生成和导出。

## 批次计划

### 第 1 批：类型注册与只读拓扑入口
目标：让四期有可见入口，先把复杂数据库的只读拓扑能力打通。

后端：
1. 新增数据库类型常量：
   - `elasticsearch`
   - `opensearch`
   - `tidb`
   - `oceanbase`
   - `opengauss`
   - `dameng`
   - `kingbase`
2. 扩展 `SupportedTypes`、`IsSupportedType`、`DefaultPort`、`DBTypeText`。
3. TiDB / OceanBase 先复用 MySQL 协议适配。
4. openGauss / Kingbase 先复用 PostgreSQL 协议适配。
5. 新增拓扑查询接口：
   - `GET /api/v1/databases/instances/{id}/topology`
6. 新增审计动作：
   - `topology_view`
7. 新增权限点：
   - `database:topology:view`
8. Redis / MongoDB / Elasticsearch / OpenSearch 实现真实只读采集；暂不落库，先按实时查询返回。

前端：
1. 数据库管理页新增“拓扑”页签。
2. 支持选择 Redis / MongoDB / Elasticsearch / OpenSearch 实例并刷新。
3. 展示摘要卡片、节点表、链路表和分片表。
4. 审计筛选补 `topology_view`。

验收：
1. 支持类型列表出现四期新增类型。
2. Redis Cluster 能返回节点和 slots。
3. MongoDB ReplicaSet 能返回成员和角色。
4. Elasticsearch / OpenSearch 能返回节点和分片。
5. 拓扑查询有审计记录。

### 第 2 批：国产数据库兼容验证
目标：把协议兼容类型纳入现有实例、元数据和只读查询闭环。

后端：
1. TiDB / OceanBase MySQL 模式实库验证并修正版本识别。
2. openGauss / Kingbase PostgreSQL 兼容模式实库验证并修正元数据 SQL 差异。
3. 达梦调研并接入 Go 驱动，先完成连接测试。

前端：
1. 实例表和类型选择补国产数据库标签样式。
2. 连接参数提示按不同类型补充。

验收：
1. 兼容类型可创建实例、测试连接、同步元数据、执行只读查询。
2. 不兼容能力返回明确错误，不出现空白页或未处理异常。

### 第 3 批：恢复演练任务
目标：从备份记录发起非生产恢复演练。

后端：
1. 新增恢复演练模型和仓储。
2. 新增接口：
   - `POST /api/v1/databases/backup-records/{id}/restore-dry-run`
   - `GET /api/v1/databases/restore-jobs`
3. 强制目标实例环境不为 `prod`。
4. 恢复命令与三期备份执行器共享运行时封装。
5. 审计动作：
   - `restore_dry_run`

前端：
1. 备份记录新增恢复演练入口。
2. 新增恢复演练记录视图。

验收：
1. 不能恢复到生产实例。
2. 成功和失败演练均有记录与审计。

### 第 4 批：容量趋势采样
目标：把容量从单次同步扩展为趋势。

后端：
1. 新增容量快照模型。
2. 后台调度定时采集启用实例容量。
3. 新增趋势查询接口。

前端：
1. 诊断页补容量趋势图。
2. 实例详情 / 列表显示容量增长摘要。

验收：
1. 采样任务可持续写入趋势点。
2. 页面能查看 24 小时、7 天、30 天趋势。

### 第 5 批：巡检报告
目标：形成四期读侧运维报告。

后端：
1. 新增巡检报告生成接口。
2. 聚合容量、性能、安全和备份状态。
3. 报告结果落库并支持列表查询。

前端：
1. 新增巡检报告页签。
2. 支持手动生成、查看详情。

验收：
1. 报告包含容量、性能、安全、备份四类结论。
2. 异常项能追溯到实例、任务或审计记录。

### 第 6 批：Redis 深度兼容
目标：把 Redis 从“可纳管、可测试、可看拓扑”补齐到“可同步、可浏览、可查询、可诊断、可巡检”的读侧闭环。

范围原则：
1. 不把 Redis 强行按 SQL 结构抽象为 Schema / Table / Column 语义。
2. 单机、主从、Cluster、Sentinel 共用一套入口，兼容 ACL 与无认证凭据。
3. 第 6 批只做读侧能力，不把高风险写入、备份恢复混在同一批。

后端：
1. 元数据同步：
   - `sync-metadata` 对 Redis 不再返回占位错误。
   - 同步 `INFO keyspace`、`TYPE`、`PTTL`、`MEMORY USAGE`、`OBJECT ENCODING`、`CLUSTER NODES / SLOTS`。
   - 逻辑 DB 保存 Key 数、过期 Key 数、平均 TTL。
   - Key 目录保存 Key 名称、类型、TTL、内存占用、长度、编码、slot、所属节点和值预览。
   - Cluster 模式按 master 节点扫描，避免只读取入口节点局部数据。
2. 结构浏览：
   - 前端展示改成 `逻辑 DB -> Key 列表 -> Key 属性`。
   - 不再展示 Redis 的 DDL 预览和数据字典导出。
3. 查询控制台：
   - Redis 实例切换为“命令控制台”模式。
   - 首批只开放白名单只读命令：
     - `PING`
     - `INFO`
     - `DBSIZE`
     - `TYPE`
     - `TTL / PTTL`
     - `EXISTS`
     - `GET / MGET / STRLEN`
     - `MEMORY USAGE`
     - `HGET / HGETALL / HLEN`
     - `LRANGE / LLEN`
     - `SMEMBERS / SCARD`
     - `ZRANGE / ZCARD`
     - `XRANGE / XLEN`
     - `SCAN`
     - `CLUSTER INFO / NODES / SLOTS`
   - 明确禁止写命令和高危命令，如 `SET / DEL / FLUSH* / CONFIG SET / EVAL / SCRIPT / MODULE / MIGRATE / CLUSTER MEET`。
   - Cluster 模式支持读命令路由；`SCAN` 聚合 master 节点结果；`DBSIZE` 聚合全局总量。
   - 所有命令继续落到统一查询审计表，沿用现有权限和审计筛选。
4. 诊断与巡检：
   - 诊断卡片改成 Redis 指标：内存、客户端、命中率、驱逐、拒绝连接、复制延迟、Cluster 状态。
   - 会话诊断基于 `CLIENT LIST`，慢日志基于 `SLOWLOG GET`。
   - 巡检报告补 Redis 专项结论：内存压力、碎片率、主从健康、慢日志、大 Key 风险、持久化配置。
5. 容量趋势：
   - Redis 不再沿用关系库表容量语义。
   - 采样口径改为 `used_memory`、`used_memory_dataset`、`maxmemory`、Key 数和节点内存分布。
6. 安全与性能限制：
   - `SCAN` 自动带 `COUNT` 上限。
   - 值预览做截断和二进制保护。
   - 所有只读命令走统一超时控制。

拆分建议：
1. 第 6.1 子阶段：Redis 元数据同步 + 结构浏览。
2. 第 6.2 子阶段：Redis 只读命令控制台 + 审计。
3. 第 6.3 子阶段：Redis 诊断、会话、慢日志。
4. 第 6.4 子阶段：Redis 巡检报告和 Redis 容量趋势。
5. 第 6.5 子阶段：Redis 逻辑备份和恢复演练。

明确不纳入本批：
1. Redis 写命令执行。
2. 基于 RDB / AOF 文件级别的物理备份切换与回放。
3. 跨大版本、跨模块类型的兼容性校验和自动转换。

验收：
1. Redis 点击“同步元数据”不再报“后续批次接入”。
2. 结构浏览能看到逻辑 DB、Key 列表和 Key 属性。
3. Redis 查询控制台可执行白名单只读命令并写审计。
4. 单机、主从、Cluster、Sentinel、无认证场景都能跑通首批读侧能力。

## 默认策略
1. 四期新增能力默认只读。
2. 恢复演练必须选择非生产目标实例。
3. 拓扑实时查询不长期保存敏感连接信息。
4. 所有用户触发动作都写入统一审计。
5. 国产数据库先按兼容协议接入，不把兼容模式误标为完全支持。

## 当前启动项
1. 已完成第 1 批第一阶段：
   - 新增四期数据库类型注册：Elasticsearch、OpenSearch、TiDB、OceanBase、openGauss、达梦、人大金仓。
   - TiDB / OceanBase 已先按 MySQL 兼容协议接入连接测试、元数据、只读查询和诊断分发。
   - openGauss / Kingbase 已先按 PostgreSQL 兼容协议接入连接测试、元数据、只读查询和诊断分发。
   - 新增拓扑查询接口 `GET /api/v1/databases/instances/{id}/topology`。
   - Redis 已支持 Cluster 节点、slots、主从关系读取；非 Cluster 模式返回单机 / 主从信息；Sentinel 模式可读取 master、replicas、sentinels 拓扑。
   - MongoDB 已支持 ReplicaSet 成员、角色、健康状态和复制关系读取；非副本集返回 `hello` 基础信息。
   - Elasticsearch / OpenSearch 已支持 cluster health、cat nodes 和 cat shards 读取，分片列表最多返回 500 条。
   - 新增统一审计动作 `topology_view` 和权限点 `database:topology:view`。
   - 前端数据库管理页新增“拓扑”页签，可查看摘要卡片、节点、复制关系和索引分片。
   - 已补 Redis 拓扑解析和搜索集群字段读取单元测试。
2. 已完成回归：
   - `go test ./...` 通过。
   - `npm run build` 通过。
3. 已完成第 2 批第一阶段：
   - TiDB 连接测试和元数据同步已优先读取 `tidb_version()`，避免只显示 MySQL 兼容版本。
   - OceanBase MySQL 模式连接测试和元数据同步已组合 `VERSION()` 与 `@@version_comment`。
   - openGauss / Kingbase 已复用 PostgreSQL 兼容协议的版本识别、元数据、只读查询和诊断分发。
   - MySQL 兼容元数据和容量诊断已扩展系统库排除范围，过滤 TiDB / OceanBase 常见系统 Schema。
   - 前端拓扑实例筛选已优先使用后端 `topologyEnabled` 能力标记。
   - 已补兼容版本文本处理、四期类型注册和兼容读侧能力单元测试。
4. 第 2 批剩余项待具备实库环境后继续：
   - 做 TiDB / OceanBase / openGauss / Kingbase 实库兼容验证。
   - 补达梦专用驱动调研和连接测试。
   - 根据实库差异继续修正元数据 SQL 和版本识别。
5. 已完成第 3 批第一阶段：
   - 新增恢复演练记录模型 `database_restore_jobs`，随服务启动自动迁移。
   - 新增接口 `POST /api/v1/databases/backup-records/{id}/restore-dry-run` 和 `GET /api/v1/databases/restore-jobs`。
   - 恢复演练仅允许从成功、local、logical 备份记录发起。
   - 目标实例必须启用、非生产，且不能与来源实例相同。
   - 首批仅支持 MySQL / MariaDB 互通恢复演练和 PostgreSQL 同类型恢复演练。
   - 恢复演练复用备份运行时命令查找和执行封装，支持 `.sql` 与 `.sql.gz` 输入。
   - 新增审计动作 `restore_dry_run` 和权限点 `database:restore:view` / `database:restore:run`。
   - 前端备份页新增恢复演练入口、目标实例选择弹窗和恢复演练记录表。
6. 已完成第 4 批第一阶段：
   - 新增容量快照模型 `database_capacity_snapshots`，保存实例、Schema、表级采样点。
   - 新增后台容量采集调度，服务启动后立即采集一次，后续默认每 6 小时采集启用实例。
   - 新增接口 `GET /api/v1/databases/instances/{id}/capacity-trend` 和 `POST /api/v1/databases/instances/{id}/capacity-snapshots`。
   - 新增审计动作 `capacity_view` 和权限点 `database:capacity:view` / `database:capacity:collect`。
   - 前端诊断页新增 24 小时、7 天、30 天容量趋势、Top 表和手动采集入口。
   - 实例列表新增近 7 天容量摘要，读取已有快照，不触发实时采集。
   - 已补容量聚合和趋势范围单元测试。
7. 已完成第 5 批第一阶段：
   - 新增巡检报告模型 `database_inspection_reports`，报告内容落库留痕。
   - 新增接口 `GET /api/v1/databases/inspection-reports`、`POST /api/v1/databases/inspection-reports` 和 `GET /api/v1/databases/inspection-reports/{id}`。
   - 报告生成会聚合容量、性能、安全审计和备份状态，输出健康分、风险等级、摘要和关注项。
   - 新增审计动作 `inspection_generate` 和权限点 `database:inspection:view` / `database:inspection:run`。
   - 前端数据库管理页新增“巡检报告”页签，支持筛选、生成和详情查看。
   - 已补巡检评分和容量增长计算单元测试。
8. 第 6 批已推进到第 6.5 子阶段：
   - Redis 支持元数据同步，后端会采集逻辑 DB 摘要和 Key 样本，不再返回“Redis 元数据同步将在后续批次接入”。
   - 结构浏览首批改成 Redis 语义，展示逻辑 DB、Key 列表和 Key 属性，不再展示 Redis DDL / 字典导出入口。
   - Redis 查询控制台首批切换为白名单只读命令模式，支持常用读命令、`SCAN`、`INFO` 和 `CLUSTER INFO / NODES / SLOTS`，并复用统一查询审计。
   - Redis 新增 Sentinel 连接与拓扑支持，可通过 `masterName + sentinelAddrs` 纳管 Sentinel 场景，并读取 master、replicas、sentinels 关系。
   - Redis 诊断已支持指标卡片、`CLIENT LIST` 会话视图和 `SLOWLOG GET` 慢日志，单机和 Cluster 统一走 Redis 读侧诊断链路。
   - Redis 巡检报告已补容量、性能、安全、持久化专项结论，容量部分按 `used_memory` 采样，性能部分输出内存压力、碎片率、Cluster 状态、阻塞客户端、驱逐和复制延迟，备份部分检查 AOF / RDB 持久化配置和最近落盘状态。
   - Redis 容量趋势已按内存趋势展示，支持手动采集、近 24 小时 / 7 天 / 30 天趋势、Top Key 样本和采样空状态提示。
   - Redis 备份任务已接入现有逻辑备份框架，支持手动触发、Cron 调度、保留策略清理、文件下载和统一审计；备份文件采用 `DUMP / RESTORE` 逻辑备份格式，单机、Cluster、Sentinel 和无认证场景共用同一条执行链路。
   - Redis 恢复演练已接入现有 dry-run 恢复链路，仅允许导入到非生产 Redis 目标实例；恢复时按 Key 做 `RESTORE REPLACE`，会覆盖同名 Key，但不会清空目标实例中的无关数据。
   - 第 6 批后续仍待继续：Redis 写命令执行、更细粒度的持久化策略巡检和跨版本备份兼容性校验。
