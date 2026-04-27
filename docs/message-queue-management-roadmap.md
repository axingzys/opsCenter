# 消息队列管理实施方案

## 文档定位
本文作为 OpsHub 消息队列管理模块的主计划文档。后续 RabbitMQ、Kafka、RocketMQ、ActiveMQ、Pulsar 等消息队列资产纳管、表结构、接口、权限、审计、前端页面、验收标准和分期开发均以本文为基准。

本文只描述产品与技术方案，不直接代表一次性全部实现。实际开发建议按分期推进，先落地只读纳管、元数据同步、消费诊断和审计，再逐步开放资源变更与高风险操作。

## 一期落地状态
截至 2026-04-27，本仓库已落地一期基础能力：

1. 新增后端模块：`internal/biz/messagequeue/`、`internal/data/messagequeue/`、`internal/service/messagequeue/`、`internal/server/messagequeue/`。
2. 新增接口前缀：`/api/v1/message-queues`，覆盖实例 CRUD、启停、连接测试、元数据同步、资源列表、消费诊断、消息采样、操作审计、对象级实例权限。
3. 新增迁移：`migrations/20260427_message_queue_module.sql`，并在 `migrations/init.sql` 中加入菜单与管理员权限。
4. 新增前端：`web/src/api/messagequeue.ts`、`web/src/views/asset/MessageQueueManagement.vue`，并挂载路由 `/asset/message-queues`。
5. 一期适配器能力：
   - RabbitMQ：Management API 连接测试、节点/vhost/exchange/queue/binding 元数据同步。
   - Kafka：Broker 连接测试、broker/topic/partition 元数据同步、独立 reader 消息采样。
   - RocketMQ：NameServer TCP 连通性与基础 broker 占位同步。
   - ActiveMQ：Jolokia 连接测试与 queue/topic 元数据同步，Jolokia 不可用时降级为 broker 端口连通性。
   - Pulsar：Admin API 连接测试、tenant/namespace/topic/subscription 元数据同步。
6. 新增环境变量控制的集成测试：`OPSHUB_MQ_INTEGRATION=1 go test ./internal/biz/messagequeue -run TestAdaptersIntegration -count=1 -v`。

## 二期落地状态
截至 2026-04-27，已落地二期“受控资源管理”能力。本期只开放低/中风险资源变更，不开放删除、清空、重置位点、跳过订阅等高风险操作。

1. 新增统一资源操作接口：
   - `POST /api/v1/message-queues/instances/:id/operations/validate`：校验动作、资源、参数、风险等级和影响范围。
   - `POST /api/v1/message-queues/instances/:id/operations`：确认后执行资源操作，并写入 `mq_operation_audits`。
2. 新增统一 Adapter 扩展接口：`ValidateOperation`、`ApplyOperation`，执行前复用实例配置、凭据解析、启用状态检查、菜单权限和对象级 `PermissionResourceManage`。
3. RabbitMQ 已支持：
   - `rabbitmq_queue_upsert`：创建或更新 queue。
   - `rabbitmq_exchange_upsert`：创建或更新 exchange。
   - `rabbitmq_binding_upsert`：创建 binding。
4. Kafka 已支持：
   - `kafka_topic_create`：创建 topic。
   - `kafka_partitions_expand`：扩展 topic 分区。
   - `kafka_topic_config_update`：通过 `IncrementalAlterConfigs` 更新 topic 配置项。
5. Pulsar 已支持：
   - `pulsar_namespace_retention_update`：更新 namespace retention。
   - `pulsar_namespace_ttl_update`：更新 namespace message TTL。
6. RocketMQ、ActiveMQ 二期资源变更暂不做不可验证的占位执行，当前返回明确“不支持/后续批次接入”。
7. 前端 `资源管理` Tab 新增“资源操作”入口，按实例类型提供操作模板，支持先校验影响再确认执行，并刷新操作审计。

## 三期落地状态
截至 2026-04-27，已落地三期“高风险操作”能力，默认仍保持关闭，需要配置、菜单权限、对象级高危权限、二次确认、资源名输入确认和操作原因同时满足后才允许执行。

1. 新增系统配置：
   - `messageQueueHighRiskEnabled`：MQ 高危操作总开关，默认 `false`。
   - `messageQueueOperationReasonRequired`：高危操作是否必须填写原因，默认 `true`。
2. 新增高危执行控制：
   - 低/中风险资源操作沿用 `messagequeue:resource:manage` 和对象级 `RESOURCE_MANAGE`。
   - 高风险/严重风险操作额外要求 `messagequeue:operation:high-risk` 菜单权限和对象级 `HIGH_RISK`。
   - 高危操作必须传入 `confirmed=true`，并在原因必填配置开启时填写 `reason`。
   - 高危操作必须传入 `confirmText`，后端校验 `confirmText == resourceName`。
3. RabbitMQ 已支持：
   - `rabbitmq_queue_purge`：清空 queue。
   - `rabbitmq_queue_delete`：删除 queue。
   - `rabbitmq_exchange_delete`：删除 exchange。
4. Kafka 已支持：
   - `kafka_topic_delete`：删除 topic。
5. Pulsar 已支持：
   - `pulsar_topic_delete`：删除 topic。
   - `pulsar_subscription_skip`：跳过 subscription 全部积压消息。
   - `pulsar_subscription_reset`：按时间戳重置 subscription cursor。
6. 高危操作执行前会记录 before snapshot、diff、warnings 和 impact summary，执行后会尽量记录刷新后的 after snapshot。
7. 高危操作执行成功后会自动触发元数据刷新，刷新状态写入本次操作结果；刷新失败不会回滚已提交的 MQ 操作，但会写入操作审计消息并标记为 `partial_success`。
8. RabbitMQ 删除 exchange 前会检查 binding；存在 binding 时默认拒绝，必须显式传入 `force=true`。
9. Pulsar subscription skip/reset 会在 validate 阶段展示当前 backlog、lag、消费者数和在线消费者风险提示。
10. 前端 `资源操作` 弹窗增加高危模板，权限不足时禁用高危选项；高危执行前会展示风险、影响范围、警告、diff、二次确认和资源名输入确认。
11. 集成测试已覆盖 RabbitMQ purge/delete、Kafka delete topic、Pulsar delete topic；单元测试覆盖高危配置、资源名确认、RabbitMQ binding 保护和 Pulsar backlog 影响提示。

## 四期落地状态
截至 2026-04-27，已开始落地四期“治理与巡检”能力，优先补齐告警和容量趋势之前的数据底座。

1. 新增手动指标快照接口：
   - `POST /api/v1/message-queues/instances/:id/metric-snapshots`：按当前元数据汇总 broker 在线数、资源数量、消息量、backlog、lag、生产/消费速率，并写入 `mq_metric_snapshots`。
   - `GET /api/v1/message-queues/instances/:id/metric-snapshots`：分页查询实例指标快照。
2. `GET /api/v1/message-queues/instances/:id/overview` 增加健康算法和异常标签：
   - 返回 `healthReasons`、`anomalyTags`、`lastMetricAt`、无消费者资源数、DLQ/Retry 资源数。
   - 默认识别同步过期、指标过期、Broker 离线、消息堆积、消费延迟、无消费者、DLQ/Retry 资源等治理信号。
3. 新增手动巡检接口：
   - `POST /api/v1/message-queues/instances/:id/inspection-reports`：即时生成巡检结果，覆盖健康、容量、消费和治理四类检查。
   - 巡检结果包含评分、风险等级、分组指标和 findings；当前为 MVP 即时报告，暂不长期落库。
4. 前端 `消费诊断` Tab 增加：
   - “采集指标”按钮，采集后刷新实例健康和审计。
   - “生成巡检”按钮，展示巡检评分、分区块摘要和风险项列表。
   - 健康状态、异常标签、健康原因和最近指标时间展示。
5. 四期单元测试覆盖指标快照采集、健康状态评估、异常标签和巡检治理项。

## 生产化优化补充清单
以下内容为结合当前一二三期落地状态、现有代码结构和后续四期目标整理出的优化项。除上文“落地状态”明确说明的能力外，本节均表示后续建议，不代表当前已全部实现。

### 二期补强：受控资源管理生产化
二期已经具备低/中风险资源操作主流程，后续重点不应继续扩大高危操作范围，而应把“能执行”补强为“可预览、可约束、可审计、可复盘”。

1. 将 `operations/validate` 升级为 operation plan/diff。
   - 返回 `before`、`after`、`diff`、`warnings`、`impact`、`risk_level`。
   - 前端在执行前展示操作前配置、操作后配置、变更项、影响范围、当前 backlog/lag、消费者数量和最近同步时间。
   - 执行成功后在审计中保存本次 plan 或 diff 摘要。
2. 引入操作幂等和资源锁。
   - 请求建议携带 `operation_id` 或 `idempotency_key`。
   - 锁粒度建议为 `instance_id + resource_type + namespace + resource_name + action`。
   - 防止重复点击、前端超时重试、多人同时修改同一 topic/queue。
3. 对 retention、TTL、max-length 等配置按变更方向重新评估风险。
   - 增大 retention/TTL 通常可视为中风险。
   - 降低 retention/TTL/max-length 可能导致消息提前清理或丢弃，应升级为高风险或在二期直接拒绝。
   - Kafka `cleanup.policy`、`min.insync.replicas`、`max.message.bytes` 等也应按值变化给出风险提示。
4. Kafka topic config update 增加白名单、只读名单和高风险名单。
   - 二期建议只开放 `retention.ms`、`retention.bytes`、`cleanup.policy`、`compression.type`、`max.message.bytes`、`min.insync.replicas`、`segment.ms` 等经过校验的配置。
   - `unclean.leader.election.enable`、关键时间戳策略、可能破坏数据保留语义的配置默认不开放。
5. RabbitMQ upsert 拆分为创建和安全更新语义。
   - 已存在 queue 的 `durable`、`exclusive`、`auto_delete`、`x-queue-type` 等不可变或高风险字段不应被普通 upsert 隐式修改。
   - 已存在 exchange 的 `type`、`durable`、`auto_delete`、`internal` 等字段不一致时，应提示需要删除重建，转入高风险流程。
   - binding 已存在且参数一致时返回 `already_exists`，不作为失败。
6. 增加资源模板和标准创建向导。
   - 建议新增 `mq_resource_templates`，按 MQ 类型、资源类型、环境维护默认配置。
   - Kafka topic 创建模板可覆盖普通业务事件、订单链路、日志采集、compact 状态表、测试环境等场景。
   - 生产环境创建资源应要求负责人、业务系统、环境、标签和模板来源。
7. 补充平台侧治理字段维护。
   - 负责人、业务系统、环境、重要等级、SLA、数据敏感等级、告警联系人、成本中心、标签和备注属于低风险平台字段。
   - 这些字段不应被同步任务覆盖，后续四期巡检、告警和容量治理会依赖这些字段。
8. Adapter 能力矩阵改为实例级动态能力。
   - 同一种 MQ 在不同版本、不同权限、不同插件开启状态下能力不同。
   - 连接测试或同步后建议生成 `capabilities_json`，前端按实例真实能力展示按钮。
   - 例如 Kafka 账号没有 delete 权限、RabbitMQ 未开启 Management Plugin、Pulsar token 无 reset 权限时，应明确返回禁用原因。
9. 操作后置同步支持 `partial_success`。
   - MQ 操作成功但元数据刷新失败时，不应把整体显示为完全失败。
   - 审计应区分 MQ 操作结果和后置同步结果。

### 三期收口：高危操作安全闭环
三期已经具备高危操作开关、权限、原因、二次确认和审计。上线生产前建议补齐以下安全闭环。

1. 高危操作前置快照。
   - RabbitMQ 删除 queue 前保存 queue 参数、durable、arguments、消息数、consumer 数、binding 信息。
   - RabbitMQ purge 前保存 ready、unacked、message、consumer 状态。
   - Kafka 删除 topic 前保存分区数、副本数、topic config、consumer group lag。
   - Pulsar reset/skip/delete 前保存 backlog、cursor、subscription、partition 信息。
   - 建议审计增加 `before_snapshot_json`、`after_snapshot_json`、`impact_summary_json`。
2. 资源名输入确认。
   - 高危操作除 `confirmed=true` 外，前端要求手动输入资源名。
   - 后端校验 `confirm_text == resource_name`，防止误点或误选。
3. 审批或双人复核预留。
   - 生产环境高危操作建议从 `validate -> execute` 演进为 `validate -> create pending operation -> approve -> execute`。
   - 审批人不能是申请人。
   - backlog、consumer group 数、分区数或环境达到阈值时强制审批。
4. 高危维护窗口。
   - 生产环境高危操作可限制在维护窗口内执行。
   - 紧急场景允许管理员 `emergency_override`，但必须填写原因并加强审计。
5. Kafka reset consumer group offset 作为三期缺口补齐。
   - 建议优先支持 `timestamp` 和 `explicit` 两种模式。
   - `earliest`、`latest`、`shift` 可在风险提示成熟后开放。
   - validate 阶段必须展示每个 partition 的当前 offset、目标 offset、end offset、lag 和变更差值。
6. RabbitMQ exchange delete 增加 binding 检查。
   - 存在 binding 时风险自动提升。
   - 默认拒绝删除，除非显式传入 `force=true` 并完成高危确认。
7. Pulsar subscription reset/skip 增加影响提示。
   - validate 阶段返回当前 backlog、cursor、目标时间、预计跳过数量、connected consumer 数和 active consumer 状态。
   - 生产环境如存在在线 consumer，可要求先停消费者或走审批。

### 四期前置：指标、同步和治理底座
四期告警、巡检、DLQ 治理和容量趋势依赖稳定的数据底座，建议在三期收尾时提前补齐。

1. 定时指标快照。
   - 复用 `mq_metric_snapshots`，按 1 分钟或 5 分钟采集实例健康、broker 在线数、资源 backlog/lag、生产/消费速率。
   - 没有稳定快照数据前，不建议直接做复杂容量预测。
2. 健康状态算法明确化。
   - `unknown`：从未同步、从未测试或指标超过阈值未更新。
   - `healthy`：连接正常、broker 正常、backlog 未超过阈值。
   - `warning`：同步过期、部分 broker 异常、backlog 增长、消费者为 0。
   - `critical`：连接失败、全部 broker 异常、backlog 严重超阈值或消费完全停滞。
   - 阈值应支持实例级或资源级覆盖，如 warning/critical lag、backlog、stale metric minutes。
3. 先做异常标签，再做告警。
   - 页面先标记无消费者、消费停滞、堆积增长、单分区热点、同步过期、Broker 离线、DLQ 增长、Retry Topic 增长、消费者频繁上下线。
   - 四期告警规则可直接复用这些诊断标签。
4. 元数据同步异步化。
   - 大集群同步建议改为 `POST sync -> job_id`，再通过 `GET sync-jobs/:id` 查询进度。
   - 状态建议支持 `pending`、`running`、`success`、`failed`、`partial_success`、`timeout`、`cancelled`。
5. 同步失败不覆盖旧数据。
   - 同步任务应支持分项结果：broker、resource、consumer、partition、metric。
   - 单项失败时整体可为 `partial_success`。
   - 未扫到的资源先标记 `is_stale`、`stale_since`、`last_seen_at`、`sync_generation`，连续多次未出现后再转为 inactive/deleted。
6. 巡检报告 MVP。
   - 手动触发实例巡检，检查连接、broker 在线、无消费者、backlog 超阈值、长期未消费、DLQ/retry 增长、同步过期、高危失败记录、未绑定负责人、未配置阈值等。
7. DLQ 和 retry 识别。
   - 资源层先统一标记 `is_dlq`、`is_retry`、`related_resource_name`。
   - 优先展示数量、backlog、增长趋势、关联原始资源、消费者状态、负责人和业务系统。
   - 消息重放属于更高风险工作流，不建议在四期前提前开放。

### 消息数据安全补强
消息查看和导出比普通元数据查询风险更高，建议尽快把脱敏和导出限制从“预留”补成真实能力。

1. 统一脱敏规则。
   - 系统配置建议支持 JSONPath 和正则两类规则。
   - 默认脱敏字段包括 `password`、`passwd`、`secret`、`token`、`access_token`、`refresh_token`、`authorization`、`phone`、`email`、`id_card`、`bank_card`。
   - 查看原文需要更高权限，并单独写审计。
2. 消息导出默认关闭。
   - 导出需要系统配置开启、对象级 `MESSAGE_EXPORT`、条数限制、总大小限制、强制脱敏和审计。
   - 审计只保存导出条件、条数、大小、hash 和摘要，不保存完整 payload。
3. 采样结果不长期保存完整 payload。
   - 如需记录，只保存 `payload_hash`、`payload_size`、`is_truncated`、`encoding`、`sample_count`、`filter_json`。
   - 避免 OpsHub 自身变成业务敏感数据存储点。

### 大集群稳定性和索引检查
RabbitMQ/Kafka/Pulsar 大集群场景下，资源数量和审计数量会快速增长，以下索引和查询约束应作为四期前的稳定性检查项。

1. 重点索引建议：
   - `mq_resources(instance_id, resource_type, namespace, name)`
   - `mq_resources(instance_id, backlog)`
   - `mq_resources(instance_id, last_sync_at)`
   - `mq_consumer_groups(instance_id, group_name)`
   - `mq_consumer_groups(instance_id, resource_name)`
   - `mq_consumer_groups(instance_id, lag)`
   - `mq_partitions(instance_id, resource_id)`
   - `mq_partitions(instance_id, lag)`
   - `mq_operation_audits(instance_id, action, risk_level, status, started_at)`
   - `mq_message_audits(instance_id, action, created_at)`
   - `mq_metric_snapshots(instance_id, collected_at)`
2. MySQL 8 下 `lag` 属于易触发语法问题的字段名，查询、聚合、排序中应使用反引号或统一改名为非保留语义字段。
3. 列表接口必须服务端分页，禁止前端一次性拉取大规模 topic、partition、consumer group 或审计记录。

### 建议优先级
P0：三期生产可用前优先补齐。

1. 高危操作 before/after/impact 快照。
2. 资源名输入确认。
3. 幂等 key 和资源锁。
4. Kafka reset offset 或明确延期。
5. 消息采样脱敏规则。
6. 最近同步时间校验。
7. Kafka 配置白名单和 retention/TTL 风险重分类。
8. 大集群分页、索引和 `lag` 字段查询检查。

P1：三期收尾和四期前置。

1. 审批流或审批接口预留。
2. 高危维护窗口。
3. 动态 capabilities/action schema。
4. 同步任务异步化和 partial success。
5. 定时 metric snapshots。
6. 健康状态算法和异常标签。
7. DLQ/retry 识别。
8. 资源模板和平台侧治理字段。

P2：四期治理增强。

1. 巡检报告 MVP。
2. 告警规则、静默和维护窗口。
3. 容量趋势和容量预测。
4. 死信消息分析。
5. 消息重放审批工作流。
6. RocketMQ、ActiveMQ 更完整资源变更能力。
7. 审计导出和合规报表。

## 背景
当前 OpsHub 已具备主机资产、凭据、资产分组、数据库管理、Kubernetes 管理、监控告警、审计和插件体系。消息队列在实际运维中通常承载核心业务链路，问题集中在以下方面：

1. MQ 实例分散在不同团队和环境中，缺少统一资产入口。
2. RabbitMQ、Kafka、RocketMQ、ActiveMQ、Pulsar 的概念差异较大，排障人员需要切换多个控制台。
3. 消息堆积、消费延迟、消费者异常、分区不均衡等问题需要快速定位。
4. 删除 Topic、Purge Queue、Reset Offset、Skip Subscription 等操作风险高，需要权限、确认、审计和全局开关。
5. 消息查看、导出、重放涉及业务数据安全，必须限制范围并留下审计。

消息队列管理模块不应只是一个简单的 MQ 控制台，而应成为统一的消息中间件资产纳管、消费诊断和受控运维平台。

## 总体目标
1. 统一纳管 RabbitMQ、Kafka、RocketMQ、ActiveMQ、Pulsar 实例或集群。
2. 复用 OpsHub 现有凭据管理，避免在 MQ 模块中保存明文密码、Token 或证书私钥。
3. 通过统一 Adapter 屏蔽不同 MQ 产品差异，前端和业务层使用统一模型。
4. 自动同步 broker、topic、queue、exchange、binding、consumer group、subscription、partition 等元数据。
5. 提供消费诊断能力，重点展示消息堆积、消费延迟、消费速率、异常消费者和分区状态。
6. 提供受控消息采样能力，默认不影响业务消费位点，并限制条数、大小和敏感内容展示。
7. 提供受控资源变更能力，逐步开放创建、修改、删除、Purge、Reset Offset 等操作。
8. 所有连接测试、同步、诊断、消息查看、导出和变更操作必须审计。
9. 与现有 RBAC、对象级权限、审计、监控告警、任务中心逐步打通。

## 设计原则
1. MQ 资产独立建模，不混入主机资产或数据库实例模型。
2. 一期优先只读，先解决纳管、可见性、消费诊断和审计问题。
3. 高风险操作默认关闭，必须由系统配置、菜单权限、对象级权限和二次确认共同控制。
4. 凭据统一复用资产凭据模块，MQ 实例只保存 `credential_id`、连接参数和 TLS 开关。
5. 自动同步字段与人工维护字段分离，避免采集任务覆盖负责人、业务系统、标签和备注。
6. 不在默认路径中消费业务消息，不提交业务消费组 offset。
7. 消息 payload 默认限制大小，敏感字段和二进制内容不直接全量展示。
8. Provider Adapter 归一化不同 MQ 差异，核心业务层不直接依赖具体 SDK。
9. 所有变更操作先做校验、风险评估、审计 pending，再执行，最后更新审计结果。
10. 前端以运维排障效率为中心，优先提供列表筛选、实例详情、消费诊断和审计检索。

## 模块入口
建议新增菜单：

1. `资产管理 -> 消息队列管理`
2. 路由建议：`/asset/message-queues`
3. 前端视图建议：`web/src/views/asset/MessageQueueManagement.vue`
4. 前端接口建议：`web/src/api/messagequeue.ts`
5. 后端接口前缀：`/api/v1/message-queues`
6. 后端业务目录：`internal/biz/messagequeue/`
7. 后端数据目录：`internal/data/messagequeue/`
8. 后端服务目录：`internal/service/messagequeue/`
9. 后端路由目录：`internal/server/messagequeue/`
10. 迁移文件建议：`migrations/202604xx_message_queue_module.sql`

## 与现有系统的关系
### 推荐做成核心资产模块
消息队列管理建议放入核心后端与资产管理菜单，而不是第一阶段做成插件，原因如下：

1. 与数据库管理类似，MQ 管理需要对象级权限、审计、凭据解密、后台同步、全局安全配置。
2. 需要和 `资产管理` 菜单下的主机、数据库、虚拟化平台形成统一资产入口。
3. 高风险操作需要统一 RBAC、审计和系统配置，不适合完全孤立在插件中。
4. 后续如果要开放为插件，也可以在核心模型稳定后拆出 Adapter 或页面。

### 可复用能力
1. 凭据管理：复用现有资产凭据，支持用户名密码、Token、证书等。
2. 菜单权限：复用 `RequireMenuPermission` 风格。
3. 对象级权限：参考数据库实例权限，按角色配置某个 MQ 实例的权限位图。
4. 操作审计：复用操作审计中间件，同时 MQ 操作单独落业务审计表。
5. 系统配置：新增 MQ 高风险操作开关、消息采样限制、导出限制等。
6. 告警能力：后续联动监控插件或告警配置。

## 支持范围
### 第一批支持类型
1. RabbitMQ
2. Kafka
3. RocketMQ
4. ActiveMQ
5. Pulsar

### 类型能力矩阵
| 类型 | 连接测试 | 元数据同步 | 消费诊断 | 消息采样 | 资源管理 | 高风险操作 | 一期建议 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| RabbitMQ | 支持 | 支持 vhost、exchange、queue、binding、consumer | 支持 ready、unacked、consumer | 谨慎支持 queue peek 或采样 | 已支持 queue/exchange/binding 创建或更新 | purge、delete queue、delete exchange | 二期主力 |
| Kafka | 支持 | 支持 broker、topic、partition、config | 支持 consumer group lag | 通过独立 reader 采样，不提交业务 offset | 已支持 topic 创建、分区扩容、topic 配置更新 | delete topic、reset offset | 二期主力 |
| RocketMQ | 支持 | 支持 cluster、broker、topic、consumer group | 支持 offset、lag、retry、DLQ 视图 | 视客户端能力分期 | 后续批次开放 topic/group | delete topic、reset offset | 一期只读优先 |
| ActiveMQ | 支持 | 支持 queue、topic、consumer、connection | 支持 pending、enqueue、dequeue | 谨慎支持 browse | 后续批次开放 destination | purge、delete destination | 一期只读优先 |
| Pulsar | 支持 | 支持 tenant、namespace、topic、subscription | 支持 backlog、cursor、rate | 通过 reader 采样 | 已支持 namespace retention/TTL 更新 | skip、reset subscription、delete topic | 二期基础 |

## 统一概念模型
不同 MQ 产品术语差异较大，后端建议使用统一模型对齐。

| 统一概念 | RabbitMQ | Kafka | RocketMQ | ActiveMQ | Pulsar |
| --- | --- | --- | --- | --- | --- |
| Instance | RabbitMQ cluster 或单节点 | Kafka cluster | RocketMQ cluster | Broker 或 broker group | Pulsar cluster |
| Broker | Node | Broker | Broker | Broker | Broker |
| Resource | vhost、exchange、queue | topic | topic | queue、topic | tenant、namespace、topic |
| Route | binding | partition assignment | queue route | destination route | topic lookup / bundle |
| Consumer | consumer | consumer group member | consumer group | consumer | subscription consumer |
| Subscription | queue consumer | consumer group | consumer group | durable subscription | subscription |
| Partition | 无或 quorum/stream 内部维度 | partition | message queue | destination internal | partitioned topic |
| Backlog | ready/unacked | lag | diff / lag | pending | backlog |

## 功能模块
### 1. MQ 实例纳管
实例管理是模块基础，负责维护连接配置和资产属性。

功能范围：

1. 新增、编辑、删除、启用、禁用 MQ 实例。
2. 支持实例类型：RabbitMQ、Kafka、RocketMQ、ActiveMQ、Pulsar。
3. 支持连接地址：管理地址、broker 地址、bootstrap servers、nameserver 地址等。
4. 支持端口、TLS 开关、认证方式、连接参数 JSON。
5. 支持绑定凭据 `credential_id`。
6. 支持环境、业务系统、负责人、标签、备注。
7. 支持连接超时、读取超时、采集超时等参数。
8. 支持批量导入，初期可支持 CSV，后续支持 Excel。
9. 支持实例状态：启用、禁用、异常、凭据错误、网络不可达。

核心字段建议：

1. `name`：实例名称。
2. `mq_type`：消息队列类型。
3. `endpoint`：主连接地址，多个地址可用逗号分隔。
4. `management_url`：管理 API 地址，可为空。
5. `port`：默认端口。
6. `credential_id`：凭据 ID。
7. `tls_enabled`：是否启用 TLS。
8. `connection_params`：连接参数 JSON。
9. `status`：启用或禁用。
10. `environment`：环境。
11. `business_system`：业务系统。
12. `owner`：负责人。
13. `tags`：标签。
14. `remark`：备注。

### 2. 连接测试
连接测试用于验证配置可用性，并返回基础能力信息。

统一返回：

1. 是否连接成功。
2. 实例类型。
3. 版本信息。
4. 集群名称或 cluster id。
5. broker/node 数量。
6. 管理 API 是否可用。
7. 认证方式摘要。
8. 当前账号权限摘要。
9. 往返延迟。
10. 错误类型和错误消息。

产品差异：

1. RabbitMQ：优先测试 Management HTTP API，再测试 AMQP 端口。
2. Kafka：测试 bootstrap servers、metadata 获取、broker 列表。
3. RocketMQ：测试 nameserver、broker route、topic route 能力。
4. ActiveMQ：测试 Jolokia/JMX 或管理 API，再测试 broker 连接。
5. Pulsar：测试 Admin REST API，再测试 broker/service URL。

### 3. 元数据同步
元数据同步支持手动触发，一期可先不做定时任务，后续增加后台调度。

同步对象：

1. 实例基础信息：版本、集群名称、集群 ID。
2. broker/node 信息：节点名、地址、角色、状态。
3. 资源信息：topic、queue、exchange、vhost、namespace、tenant。
4. 路由信息：binding、partition、message queue、bundle。
5. 消费组或订阅：consumer group、subscription、durable subscription。
6. 分区和队列维度：partition id、leader、replicas、min offset、max offset。
7. 消费位点：current offset、log end offset、lag、backlog。
8. 采集时间、采集耗时、错误信息。

同步要求：

1. 每次同步写入 `mq_sync_jobs`，记录状态、耗时、错误。
2. 同步采用 replace 或 upsert 策略，保留人工字段。
3. 同步失败不应清空上一次成功的元数据。
4. 大集群同步需要分页、限流和超时控制。
5. 对于资源数量特别大的集群，前端列表必须服务端分页。

### 4. 健康概览
健康概览用于快速判断实例是否可用、是否有消费堆积。

展示指标：

1. broker/node 总数、在线数、异常数。
2. topic/queue 数量。
3. consumer group/subscription 数量。
4. 总 backlog/lag。
5. 最大 backlog/lag 资源。
6. 生产速率。
7. 消费速率。
8. 消息堆积增长趋势。
9. 最近连接测试时间。
10. 最近同步时间。

健康等级建议：

1. `healthy`：连接正常，无明显堆积。
2. `warning`：存在增长中的堆积、部分消费者异常或同步过期。
3. `critical`：连接失败、broker 异常、大规模消费停滞或 backlog 超阈值。
4. `unknown`：未同步、未测试或数据不足。

### 5. 资源管理
资源管理一期以只读展示为主，二期开放受控变更。

统一资源字段：

1. 资源名称。
2. 资源类型。
3. 命名空间或 vhost。
4. 分区数或队列数。
5. 副本数。
6. durable / transient。
7. retention 配置。
8. TTL 配置。
9. 最大消息大小。
10. 当前消息量。
11. backlog / lag。
12. 生产速率、消费速率。
13. 创建时间、更新时间。

RabbitMQ 资源：

1. vhost。
2. exchange。
3. queue。
4. binding。
5. consumer。

Kafka 资源：

1. topic。
2. partition。
3. consumer group。
4. group member。
5. topic config。

RocketMQ 资源：

1. topic。
2. broker。
3. consumer group。
4. message queue。
5. retry topic。
6. DLQ topic。

ActiveMQ 资源：

1. queue。
2. topic。
3. durable subscription。
4. connection。
5. consumer。

Pulsar 资源：

1. tenant。
2. namespace。
3. topic。
4. partitioned topic。
5. subscription。
6. cursor。

### 6. 消费诊断
消费诊断是一期核心能力，目标是快速定位堆积来源。

诊断维度：

1. 按实例查看总 backlog/lag。
2. 按 topic/queue 查看 backlog/lag。
3. 按 consumer group/subscription 查看 backlog/lag。
4. 按 partition/message queue 查看 offset 差异。
5. 查看消费者在线状态、客户端地址、消费速率。
6. 查看最近消费时间、最近生产时间。
7. 标记消费停滞、消费速率低于生产速率、单分区热点。
8. 展示 Top 堆积资源。

统一诊断指标：

1. `producedRate`：生产速率。
2. `consumedRate`：消费速率。
3. `backlog`：堆积数量。
4. `lag`：位点差异。
5. `readyMessages`：可消费消息数。
6. `unackedMessages`：未确认消息数。
7. `consumerCount`：消费者数量。
8. `activeConsumerCount`：活跃消费者数量。
9. `lastProducedAt`：最近生产时间。
10. `lastConsumedAt`：最近消费时间。

### 7. 消息查看与采样
消息查看风险较高，一期只做受控采样，不做批量拉取和重放。

设计要求：

1. 必须有 `MESSAGE_READ` 对象级权限。
2. 必须记录 `mq_message_audits`。
3. 默认限制最多查看 10 条。
4. 默认限制单条 payload 最大展示 64 KB。
5. 二进制消息默认展示摘要、编码和大小，不直接渲染完整内容。
6. JSON 文本可格式化展示。
7. 支持按 key、partition、offset、时间范围等条件过滤，视 MQ 类型而定。
8. 不提交业务消费组 offset。
9. 不使用业务 consumer group 拉取消息。
10. 敏感字段后续接入统一脱敏规则。

各产品策略：

1. Kafka：使用独立 reader 指定 topic、partition、offset 或时间位置采样。
2. Pulsar：使用 reader 模式，不绑定业务 subscription。
3. RabbitMQ：优先采用 Management API 的受控 get/peek 能力，必须确认 requeue 策略，不允许默认消费删除。
4. ActiveMQ：优先使用 browse 能力，不做 destructive consume。
5. RocketMQ：优先使用按 offset 或 message id 查询能力，具体能力按部署版本 PoC 后确认。

### 8. 资源变更
资源变更二期开放，所有操作必须先走风险校验。

低风险操作：

1. 创建 topic。
2. 创建 queue。
3. 创建 exchange。
4. 创建 binding。
5. 修改描述、标签、备注等平台侧字段。

中风险操作：

1. 修改 topic retention。
2. 修改 queue TTL。
3. 修改 partition 数。
4. 修改 namespace/topic 配置。
5. 暂停或恢复某些非核心消费者。

高风险操作：

1. 删除 topic。
2. 删除 queue。
3. 删除 exchange。
4. purge queue。
5. reset consumer group offset。
6. skip subscription。
7. unload namespace。
8. 删除 subscription。
9. 发布测试消息到生产 topic。
10. 消息重放。

高风险操作要求：

1. 系统配置开启 MQ 高风险操作。
2. 用户具备菜单权限。
3. 用户具备该实例对象级 `HIGH_RISK` 权限。
4. 操作前展示影响范围。
5. 操作人填写原因。
6. 操作人二次确认。
7. 写入 pending 审计。
8. 执行完成后更新结果、耗时和错误信息。

### 9. 审计与合规
业务审计单独建表，便于按 MQ 实例、资源、操作类型查询。

审计范围：

1. 连接测试。
2. 元数据同步。
3. 健康指标采集。
4. 消费诊断查看。
5. 消息采样查看。
6. 消息导出。
7. 资源创建、修改、删除。
8. purge。
9. reset offset。
10. skip subscription。
11. 发布测试消息。
12. 消息重放。
13. 实例权限变更。

审计字段：

1. 操作人 ID、用户名、客户端 IP。
2. 实例 ID、实例名称、MQ 类型。
3. 资源类型、资源名称、命名空间。
4. 操作动作。
5. 风险等级。
6. 操作参数摘要。
7. 操作原因。
8. 执行状态。
9. 影响范围。
10. 错误信息。
11. 开始时间、结束时间、耗时。

### 10. 告警联动
告警联动可放到二期或三期，先在 MQ 模块内部保留指标快照。

建议告警项：

1. 实例连接失败。
2. broker/node 离线。
3. topic/queue backlog 超阈值。
4. consumer group lag 超阈值。
5. backlog 持续增长。
6. 消费者数为 0。
7. 生产速率突增。
8. 消费速率突降。
9. 同步任务连续失败。
10. 高风险操作执行失败。

## 后端架构
### 分层设计
建议后端目录：

1. `internal/biz/messagequeue/model.go`：模型、常量、请求、VO。
2. `internal/biz/messagequeue/repository.go`：Repository 接口。
3. `internal/biz/messagequeue/usecase.go`：业务编排、权限前置数据、风险控制。
4. `internal/biz/messagequeue/adapter.go`：统一 Adapter 接口。
5. `internal/biz/messagequeue/adapter_rabbitmq.go`：RabbitMQ 适配器。
6. `internal/biz/messagequeue/adapter_kafka.go`：Kafka 适配器。
7. `internal/biz/messagequeue/adapter_rocketmq.go`：RocketMQ 适配器。
8. `internal/biz/messagequeue/adapter_activemq.go`：ActiveMQ 适配器。
9. `internal/biz/messagequeue/adapter_pulsar.go`：Pulsar 适配器。
10. `internal/data/messagequeue/repository.go`：GORM repository 实现。
11. `internal/data/messagequeue/permission_repository.go`：对象级权限实现。
12. `internal/data/messagequeue/operations_repository.go`：审计、采样、操作记录实现。
13. `internal/service/messagequeue/http.go`：HTTP handler。
14. `internal/server/messagequeue/http.go`：依赖组装和路由注册。

### Adapter 接口草案
```go
type MessageQueueAdapter interface {
	Type() string

	TestConnection(
		ctx context.Context,
		instance *MQInstance,
		credential *ConnectionCredential,
	) (*MQConnectionTestResult, error)

	DiscoverMetadata(
		ctx context.Context,
		instance *MQInstance,
		credential *ConnectionCredential,
	) (*MQMetadataSnapshot, error)

	CollectMetrics(
		ctx context.Context,
		instance *MQInstance,
		credential *ConnectionCredential,
	) (*MQMetricSnapshot, error)

	SampleMessages(
		ctx context.Context,
		req *MQMessageSampleRequest,
	) (*MQMessageSampleResult, error)

	ValidateOperation(
		ctx context.Context,
		req *MQOperationRequest,
	) (*MQOperationValidationResult, error)

	ApplyOperation(
		ctx context.Context,
		req *MQOperationRequest,
	) (*MQOperationResult, error)
}
```

### Adapter 注册
```go
type AdapterRegistry struct {
	adapters map[string]MessageQueueAdapter
}

func (r *AdapterRegistry) Get(mqType string) (MessageQueueAdapter, bool) {
	adapter, ok := r.adapters[mqType]
	return adapter, ok
}
```

### 凭据解析
复用资产凭据仓库，MQ UseCase 只依赖一个 resolver：

```go
type ConnectionCredential struct {
	Username     string
	Password     string
	Token        string
	AccessKey    string
	SecretKey    string
	CertPEM      string
	KeyPEM       string
	CACertPEM    string
	Extra        map[string]string
}
```

一期可以先按现有凭据字段承载用户名密码；Token、证书和 SASL 参数放入 `connection_params` 或后续扩展凭据类型。

## 数据模型草案
### `mq_instances`
用途：保存 MQ 实例连接配置与资产属性。

核心字段：

1. `id`
2. `name`
3. `mq_type`
4. `engine`
5. `version`
6. `endpoint`
7. `management_url`
8. `port`
9. `credential_id`
10. `tls_enabled`
11. `connection_params`
12. `status`
13. `health_status`
14. `environment`
15. `business_system`
16. `owner`
17. `tags`
18. `remark`
19. `last_test_at`
20. `last_sync_at`
21. `last_metric_at`
22. `created_at`
23. `updated_at`
24. `deleted_at`

建议索引：

1. `idx_mq_instances_type`
2. `idx_mq_instances_status`
3. `idx_mq_instances_environment`
4. `idx_mq_instances_deleted_at`

### `mq_instance_permissions`
用途：角色到 MQ 实例的对象级权限。

核心字段：

1. `id`
2. `role_id`
3. `instance_id`
4. `permissions`
5. `created_at`
6. `updated_at`
7. `deleted_at`

唯一索引：

1. `uk_mq_instance_permission_role_instance(role_id, instance_id, deleted_at)`

### `mq_brokers`
用途：保存 broker/node 元数据。

核心字段：

1. `id`
2. `instance_id`
3. `broker_name`
4. `broker_id`
5. `host`
6. `port`
7. `role`
8. `status`
9. `version`
10. `rack`
11. `zone`
12. `metadata_json`
13. `last_sync_at`

### `mq_resources`
用途：统一保存 topic、queue、exchange、vhost、tenant、namespace 等资源。

核心字段：

1. `id`
2. `instance_id`
3. `resource_type`
4. `namespace`
5. `name`
6. `full_name`
7. `durable`
8. `partition_count`
9. `replica_count`
10. `message_count`
11. `backlog`
12. `produced_rate`
13. `consumed_rate`
14. `config_json`
15. `metadata_json`
16. `last_sync_at`

建议唯一索引：

1. `uk_mq_resource(instance_id, resource_type, namespace, name)`

### `mq_bindings`
用途：保存 RabbitMQ binding 或其他路由关系。

核心字段：

1. `id`
2. `instance_id`
3. `vhost`
4. `source`
5. `destination`
6. `destination_type`
7. `routing_key`
8. `arguments_json`
9. `last_sync_at`

### `mq_consumer_groups`
用途：保存 consumer group、subscription、durable subscription。

核心字段：

1. `id`
2. `instance_id`
3. `resource_id`
4. `group_name`
5. `resource_name`
6. `namespace`
7. `state`
8. `consumer_count`
9. `active_consumer_count`
10. `current_offset`
11. `end_offset`
12. `lag`
13. `backlog`
14. `last_consumed_at`
15. `metadata_json`
16. `last_sync_at`

### `mq_partitions`
用途：保存 Kafka partition、RocketMQ message queue、Pulsar partitioned topic 的分区维度信息。

核心字段：

1. `id`
2. `instance_id`
3. `resource_id`
4. `partition_id`
5. `leader`
6. `replicas_json`
7. `isr_json`
8. `start_offset`
9. `end_offset`
10. `current_offset`
11. `lag`
12. `status`
13. `last_sync_at`

### `mq_sync_jobs`
用途：记录元数据同步任务。

核心字段：

1. `id`
2. `instance_id`
3. `trigger_type`
4. `status`
5. `started_at`
6. `finished_at`
7. `duration_ms`
8. `broker_count`
9. `resource_count`
10. `consumer_group_count`
11. `message`
12. `operator_id`
13. `operator_name`

### `mq_metric_snapshots`
用途：保存实例级或资源级指标快照。

核心字段：

1. `id`
2. `instance_id`
3. `resource_id`
4. `resource_type`
5. `resource_name`
6. `broker_count`
7. `online_broker_count`
8. `message_count`
9. `backlog`
10. `lag`
11. `produced_rate`
12. `consumed_rate`
13. `consumer_count`
14. `collected_at`

### `mq_operation_audits`
用途：记录资源变更和高风险操作。

核心字段：

1. `id`
2. `instance_id`
3. `instance_name`
4. `mq_type`
5. `resource_type`
6. `resource_name`
7. `namespace`
8. `action`
9. `risk_level`
10. `status`
11. `request_json`
12. `result_json`
13. `reason`
14. `operator_id`
15. `operator_name`
16. `client_ip`
17. `started_at`
18. `finished_at`
19. `duration_ms`
20. `message`

后续增强字段建议：

1. `operation_id`：一次操作的全局唯一 ID。
2. `operation_plan_id`：validate/plan 阶段生成的计划 ID。
3. `idempotency_key`：客户端或服务端生成的幂等键。
4. `lock_key`：资源操作锁键。
5. `confirm_text`：高危操作手动输入的资源名确认文本。
6. `before_snapshot_json`：执行前资源状态快照。
7. `after_snapshot_json`：执行后资源状态快照。
8. `diff_json`：配置变更 diff。
9. `warnings_json`：validate 阶段风险提示。
10. `impact_summary_json`：影响范围摘要。
11. `metadata_refresh_status`：后置元数据刷新状态。
12. `metadata_refresh_error`：后置元数据刷新错误。
13. `approval_status`：审批状态，预留。
14. `approved_by`：审批人，预留。
15. `approved_at`：审批时间，预留。
16. `emergency_override`：是否紧急绕过维护窗口或审批。

### `mq_message_audits`
用途：记录消息查看、导出、发布和重放。

核心字段：

1. `id`
2. `instance_id`
3. `mq_type`
4. `resource_type`
5. `resource_name`
6. `namespace`
7. `action`
8. `sample_count`
9. `payload_bytes`
10. `filter_json`
11. `status`
12. `operator_id`
13. `operator_name`
14. `client_ip`
15. `created_at`
16. `message`

## 权限设计
### 菜单权限
建议新增权限码：

1. `messagequeue:instance:view`
2. `messagequeue:instance:create`
3. `messagequeue:instance:update`
4. `messagequeue:instance:delete`
5. `messagequeue:instance:status`
6. `messagequeue:connection:test`
7. `messagequeue:metadata:view`
8. `messagequeue:metadata:sync`
9. `messagequeue:diagnosis:view`
10. `messagequeue:message:read`
11. `messagequeue:message:export`
12. `messagequeue:message:write`
13. `messagequeue:resource:manage`
14. `messagequeue:operation:high-risk`
15. `messagequeue:audit:view`
16. `messagequeue:audit:export`
17. `messagequeue:permission:manage`

### 对象级权限位图
建议定义：

| 权限 | 位值 | 说明 |
| --- | --- | --- |
| `VIEW` | 1 | 查看实例和资源 |
| `DIAGNOSE` | 2 | 查看健康、lag、消费者 |
| `MESSAGE_READ` | 4 | 查看消息采样 |
| `MESSAGE_EXPORT` | 8 | 导出消息样本或资源列表 |
| `MESSAGE_WRITE` | 16 | 发布测试消息、消息重放 |
| `RESOURCE_MANAGE` | 32 | 创建、修改资源 |
| `HIGH_RISK` | 64 | purge、delete、reset offset、skip |
| `AUDIT` | 128 | 查看该实例操作审计 |
| `MANAGE` | 256 | 管理实例配置和实例权限 |
| `ALL` | 511 | 全部权限 |

### 权限校验顺序
1. 校验登录态。
2. 校验菜单权限。
3. 如果系统内已有 MQ 对象权限规则，则启用对象级权限过滤。
4. 管理员角色默认拥有所有实例权限。
5. 普通用户只能看到有 `VIEW` 权限的实例。
6. 执行诊断、消息查看、导出和变更时再校验对应对象级权限。

## API 设计草案
### 基础能力
| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/message-queues/supported-types` | 获取支持类型和能力矩阵 |
| `GET` | `/api/v1/message-queues/ui-permissions` | 获取当前用户前端按钮权限 |
| `GET` | `/api/v1/message-queues/instances` | 实例列表 |
| `POST` | `/api/v1/message-queues/instances` | 创建实例 |
| `GET` | `/api/v1/message-queues/instances/:id` | 实例详情 |
| `PUT` | `/api/v1/message-queues/instances/:id` | 更新实例 |
| `DELETE` | `/api/v1/message-queues/instances/:id` | 删除实例 |
| `POST` | `/api/v1/message-queues/instances/:id/enable` | 启用实例 |
| `POST` | `/api/v1/message-queues/instances/:id/disable` | 禁用实例 |
| `POST` | `/api/v1/message-queues/instances/:id/test` | 连接测试 |

### 元数据与诊断
| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/message-queues/instances/:id/sync-metadata` | 同步元数据 |
| `GET` | `/api/v1/message-queues/sync-jobs/:jobId` | 查询异步同步任务，后续增强 |
| `GET` | `/api/v1/message-queues/instances/:id/brokers` | broker 列表 |
| `GET` | `/api/v1/message-queues/instances/:id/resources` | 资源列表 |
| `GET` | `/api/v1/message-queues/instances/:id/resources/:resourceId` | 资源详情 |
| `GET` | `/api/v1/message-queues/instances/:id/bindings` | binding 列表 |
| `GET` | `/api/v1/message-queues/instances/:id/consumer-groups` | 消费组或订阅列表 |
| `GET` | `/api/v1/message-queues/instances/:id/partitions` | 分区或队列维度 |
| `GET` | `/api/v1/message-queues/instances/:id/overview` | 实例健康概览 |
| `GET` | `/api/v1/message-queues/instances/:id/lag-trend` | lag/backlog 趋势 |
| `POST` | `/api/v1/message-queues/instances/:id/metric-snapshots` | 手动采集指标快照 |

### 消息采样
| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/message-queues/instances/:id/messages/sample` | 消息采样 |
| `POST` | `/api/v1/message-queues/instances/:id/messages/export` | 导出消息样本，二期 |
| `POST` | `/api/v1/message-queues/instances/:id/messages/publish` | 发布测试消息，三期 |
| `POST` | `/api/v1/message-queues/instances/:id/messages/replay` | 消息重放，三期 |

### 资源操作
| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/message-queues/instances/:id/operations/validate` | 操作风险校验 |
| `GET` | `/api/v1/message-queues/instances/:id/operations/actions` | 获取实例真实可用操作和表单 schema，后续增强 |
| `POST` | `/api/v1/message-queues/instances/:id/operations/plan` | 生成操作 plan/diff，后续增强 |
| `POST` | `/api/v1/message-queues/instances/:id/operations` | 执行资源操作 |
| `POST` | `/api/v1/message-queues/operations/:operationId/approve` | 审批待执行高危操作，后续增强 |
| `GET` | `/api/v1/message-queues/operation-audits` | 操作审计列表 |
| `GET` | `/api/v1/message-queues/operation-audits/export` | 导出操作审计 |
| `GET` | `/api/v1/message-queues/message-audits` | 消息审计列表 |

### 实例权限
| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/message-queues/instance-permissions` | 实例权限列表 |
| `POST` | `/api/v1/message-queues/instance-permissions` | 新增或更新实例权限 |
| `DELETE` | `/api/v1/message-queues/instance-permissions/:id` | 删除实例权限 |

## 风险控制流程
### 资源变更流程
1. 接收操作请求。
2. 校验登录态和菜单权限。
3. 校验实例对象级权限。
4. 获取实例配置和凭据。
5. 检查实例是否启用。
6. 根据 `action` 判断风险等级。
7. 校验系统配置是否允许该类操作。
8. 校验元数据新鲜度，生产环境可要求先同步再操作。
9. 高风险操作要求原因、二次确认和资源名输入确认。
10. 调用 Adapter `ValidateOperation` 或 plan 接口获取影响范围、风险提示和配置 diff。
11. 获取幂等记录和资源锁，避免重复提交或并发修改同一资源。
12. 写入 `mq_operation_audits`，状态为 `pending`。
13. 如命中审批规则，进入待审批状态，审批通过后再执行。
14. 调用 Adapter `ApplyOperation`。
15. 更新审计状态、结果、耗时、错误信息和 before/after 快照。
16. 必要时触发元数据同步，MQ 操作成功但同步失败时记录为 `partial_success`。

### 消息采样流程
1. 校验登录态和权限。
2. 校验实例对象级 `MESSAGE_READ` 权限。
3. 校验采样条数和 payload 大小限制。
4. 获取实例与凭据。
5. 调用 Adapter 采样。
6. 对 payload 做大小截断、编码识别、二进制摘要和敏感字段脱敏。
7. 如用户申请查看原文，额外校验权限并单独写审计。
8. 写入 `mq_message_audits`，审计中只保存条件、条数、大小、hash 和摘要，不保存完整 payload。
9. 返回样本数据、脱敏状态和截断标记。

## 前端设计
### 页面结构
建议新增 `MessageQueueManagement.vue`，采用单页面多 Tab：

1. `实例管理`
2. `资源管理`
3. `消费诊断`
4. `消息查看`
5. `操作审计`
6. `实例权限`

### 实例管理
列表字段：

1. 实例名称。
2. 类型。
3. 地址。
4. 管理地址。
5. 环境。
6. 业务系统。
7. 负责人。
8. 健康状态。
9. 最近测试。
10. 最近同步。
11. 最大堆积。
12. 操作按钮。

操作按钮：

1. 连接测试。
2. 同步元数据。
3. 健康概览。
4. 启用或禁用。
5. 编辑。
6. 删除。

### 资源管理
筛选条件：

1. 实例。
2. 资源类型。
3. 命名空间、vhost 或 tenant。
4. 关键字。
5. 是否有堆积。
6. 是否有消费者。

表格字段：

1. 资源名称。
2. 类型。
3. 分区或队列数。
4. 消息数。
5. backlog/lag。
6. 生产速率。
7. 消费速率。
8. 消费者数。
9. 最近同步。

### 消费诊断
重点视图：

1. Top 堆积资源。
2. Top lag consumer group。
3. 分区 lag 明细。
4. 消费者在线状态。
5. 生产速率与消费速率对比。
6. 异常诊断标签，例如无消费者、消费停滞、单分区热点。

### 消息查看
表单字段：

1. 实例。
2. 资源类型。
3. topic/queue 名称。
4. consumer group 或 subscription，可选。
5. partition，可选。
6. offset 或时间位置，可选。
7. key，可选。
8. 最大条数。

展示字段：

1. topic/queue。
2. partition。
3. offset 或 message id。
4. key。
5. timestamp。
6. headers/properties。
7. payload 摘要。
8. payload 展示。
9. 是否截断。

### 操作审计
筛选条件：

1. 实例。
2. MQ 类型。
3. 操作人。
4. 操作动作。
5. 风险等级。
6. 执行状态。
7. 时间范围。
8. 资源名关键字。

### 实例权限
复用数据库实例权限页面交互：

1. 按角色和实例筛选。
2. 选择权限位图。
3. 展示角色、实例、权限标签、更新时间。
4. 支持编辑和删除。

## 系统配置建议
建议在系统配置中增加消息队列管理配置：

基础配置：

1. `messageQueueHighRiskEnabled`：是否允许高风险 MQ 操作，默认 false。
2. `messageQueueOperationReasonRequired`：高风险操作是否必须填写原因，默认 true。
3. `messageQueueMessageSampleEnabled`：是否允许消息采样，默认 true。
4. `messageQueueMaxSampleMessages`：单次最大采样条数，默认 10。
5. `messageQueueMaxPayloadBytes`：单条最大展示字节数，默认 65536。
6. `messageQueueAuditRetentionDays`：MQ 审计保留天数，默认 180。
7. `messageQueueMetricRetentionDays`：指标快照保留天数，默认 30。
8. `messageQueueSyncTimeoutSeconds`：同步超时，默认 60。

后续增强配置：

1. `messageQueueOperationMaxMetadataAgeMinutes`：资源变更允许的最大元数据年龄，默认 30。
2. `messageQueueHighRiskAllowedTimeRanges`：生产高危操作维护窗口，例如 `[{"start":"00:00","end":"06:00"}]`。
3. `messageQueueSensitiveRules`：消息采样脱敏规则，支持 JSONPath 和正则。
4. `messageQueueMessageExportEnabled`：是否允许消息导出，默认 false。
5. `messageQueueMaxExportMessages`：单次最大导出条数，默认 100。
6. `messageQueueMaxExportBytes`：单次最大导出总字节数，默认 10485760。
7. `messageQueueMetricCollectIntervalSeconds`：定时指标采集间隔，默认 300。
8. `messageQueueKafkaConfigUpdateAllowList`：Kafka topic 配置更新白名单。
9. `messageQueueKafkaConfigHighRiskKeys`：Kafka topic 配置高风险键列表。
10. `messageQueueKafkaConfigReadonlyKeys`：Kafka topic 配置只读键列表。
11. `messageQueueResourceLockTimeoutSeconds`：资源操作锁超时时间，默认 300。
12. `messageQueueRequireApprovalForProdHighRisk`：生产环境高危操作是否强制审批，默认 true。

## 分期计划
### 一期：只读纳管和消费诊断
目标：把五类 MQ 统一纳管起来，提供连接测试、元数据同步、健康概览、消费诊断、消息采样、权限和审计。

后端：

1. 新增数据模型和迁移。
2. 新增 repository、usecase、service、server。
3. 新增支持类型能力矩阵。
4. 实现实例 CRUD、启停、连接测试。
5. 实现元数据同步和同步任务记录。
6. 实现 broker、resource、consumer group、partition 查询。
7. 实现对象级权限。
8. 实现消息采样审计。
9. 实现 RabbitMQ、Kafka、Pulsar 只读能力。
10. RocketMQ、ActiveMQ 一期可以先完成连接测试和基础元数据，细节按 PoC 补齐。

前端：

1. 新增 API 文件。
2. 新增消息队列管理页面。
3. 实现实例管理、资源管理、消费诊断、消息查看、操作审计、实例权限。
4. 新增菜单和路由。
5. 按 UI 权限控制按钮展示和禁用状态。

验收：

1. 可以新增五种 MQ 实例。
2. 可以测试连接并返回版本或基础信息。
3. 可以同步元数据。
4. 可以查看 topic/queue/consumer group/subscription。
5. 可以查看 lag/backlog。
6. 消息采样不影响业务消费位点。
7. 操作记录可查询。
8. 普通用户只能看到授权实例。

### 二期：受控资源管理
目标：开放低风险和中风险资源变更。

范围：

1. RabbitMQ queue、exchange、binding 创建和编辑。
2. Kafka topic 创建、partition 扩容、配置修改。
3. Pulsar namespace/topic 配置修改。
4. RocketMQ topic/group 管理。
5. ActiveMQ destination 管理。
6. 操作前风险校验。
7. 操作审计和操作结果展示。

### 三期：高风险操作
目标：开放 purge、delete、reset offset、skip subscription 等高风险能力。

要求：

1. 默认关闭。
2. 系统配置开启。
3. 菜单权限和对象级权限都满足。
4. 二次确认。
5. 操作原因必填。
6. 操作前展示影响范围。
7. 全量审计。
8. 操作后自动同步元数据。

### 四期：告警、巡检和治理
目标：从管理控制台升级为消息队列治理平台。

范围：

1. backlog/lag 告警。
2. broker 离线告警。
3. 消费停滞告警。
4. DLQ 和 retry topic 治理。
5. 巡检报告。
6. 容量趋势。
7. 死信消息分析。
8. 消息重放工作流。

## 适配器实现建议
### RabbitMQ
优先接口：

1. Management HTTP API。
2. AMQP 连接测试。

一期能力：

1. 获取 overview。
2. 获取 nodes。
3. 获取 vhosts。
4. 获取 exchanges。
5. 获取 queues。
6. 获取 bindings。
7. 获取 consumers。
8. 读取 queue ready、unacked、consumer count、message rates。

注意事项：

1. Management 插件未开启时只能做有限 AMQP 测试。
2. 消息采样必须确认 requeue，避免误消费删除。
3. Purge Queue 属于高风险操作。

### Kafka
优先接口：

1. Kafka Admin 协议。
2. Consumer group offset 查询。
3. 临时 reader 消息采样。

一期能力：

1. 获取 broker 列表。
2. 获取 topic 列表。
3. 获取 partition leader、replica、ISR。
4. 获取 consumer group。
5. 计算 group lag。
6. 读取 topic config。

注意事项：

1. SASL、TLS 参数差异较大，需要通过 `connection_params` 承载。
2. 消息采样不能使用业务 group 提交 offset。
3. reset offset 和 delete topic 必须放到高风险操作。

### RocketMQ
优先接口：

1. Nameserver 路由信息。
2. Admin 能力。
3. 视部署版本补充命令行或 HTTP 管理接口。

一期能力：

1. 获取 cluster。
2. 获取 broker。
3. 获取 topic route。
4. 获取 consumer group。
5. 获取消费位点和堆积。
6. 识别 retry topic 和 DLQ topic。

注意事项：

1. RocketMQ 版本和部署方式差异较大，建议先做 PoC。
2. 一期优先只读，不先承诺消息重放和 offset 修改。

### ActiveMQ
优先接口：

1. Jolokia/JMX。
2. Web console 管理 API，视版本而定。
3. Broker 协议连接测试。

一期能力：

1. 获取 broker 信息。
2. 获取 queues。
3. 获取 topics。
4. 获取 consumers。
5. 获取 pending、enqueue、dequeue。
6. 获取 durable subscriptions。

注意事项：

1. ActiveMQ Classic 与 Artemis 差异较大，模型中需要保留 `engine` 字段。
2. Browse 消息必须避免 destructive consume。

### Pulsar
优先接口：

1. Pulsar Admin REST API。
2. Reader 模式做消息采样。

一期能力：

1. 获取 tenants。
2. 获取 namespaces。
3. 获取 topics。
4. 获取 partitioned topics。
5. 获取 subscriptions。
6. 获取 backlog、rate、consumer。
7. 获取 broker 信息。

注意事项：

1. Token、TLS 和 mTLS 需要凭据与连接参数共同支持。
2. skip 和 reset subscription 属于高风险操作。

## 测试策略
### 单元测试
1. 支持类型能力矩阵。
2. 请求参数校验。
3. 权限位图归一化。
4. 对象级权限过滤。
5. 风险等级判断。
6. payload 截断和脱敏预留。
7. Adapter registry。

### 集成测试
1. 实例 CRUD。
2. 连接测试失败场景。
3. 同步任务成功和失败。
4. 资源分页查询。
5. 消费诊断查询。
6. 消息采样审计。
7. 对象级权限生效。

### 手工验收环境
建议准备：

1. RabbitMQ 单节点或集群。
2. Kafka 三 broker 测试集群。
3. RocketMQ nameserver + broker。
4. ActiveMQ Classic 或 Artemis。
5. Pulsar standalone 或小集群。

## 验收清单
一期完成标准：

1. 菜单 `资产管理 -> 消息队列管理` 可见。
2. 管理员可新增、编辑、删除、启停 MQ 实例。
3. 管理员可对五类 MQ 执行连接测试。
4. 至少 RabbitMQ、Kafka、Pulsar 可同步完整基础元数据。
5. RocketMQ、ActiveMQ 至少完成连接测试和基础资源同步。
6. 实例列表展示健康状态、最近测试、最近同步。
7. 资源管理页可按实例、类型、关键字筛选。
8. 消费诊断页可展示 lag/backlog Top 列表。
9. 消息查看页具备条数和大小限制，并写入审计。
10. 普通用户只可查看授权实例。
11. 无权限用户不能访问消息采样和诊断接口。
12. 所有错误接口返回明确中文错误信息。
13. 后端测试通过。
14. 前端构建通过。

## 风险与待确认事项
1. RocketMQ 管理能力强依赖部署版本和可用管理接口，需要先做 PoC。
2. ActiveMQ Classic 与 Artemis 差异较大，可能需要拆分适配器。
3. RabbitMQ 消息采样如使用 get 操作，必须确认 requeue 行为，避免误删消息。
4. Kafka 消息采样需要严格避免提交业务 group offset。
5. Pulsar subscription reset/skip 风险高，必须放到三期。
6. 大规模 Kafka/Pulsar 集群元数据量很大，接口必须分页并限制同步超时。
7. 消息 payload 可能包含敏感数据，需要尽早接入脱敏策略。
8. 生产环境资源变更应预留审批流接口。
9. 凭据类型可能需要扩展，以支持 Kafka SASL、Pulsar Token、mTLS 等。
10. 不同 MQ 指标命名差异大，前端展示需要保留原始指标详情。

## 推荐落地顺序
1. 先建文档和一期范围确认。
2. 新增表结构迁移和后端模型。
3. 实现实例 CRUD、权限和路由。
4. 实现 RabbitMQ Adapter，只读打通完整链路。
5. 实现 Kafka Adapter，只读打通完整链路。
6. 实现 Pulsar Adapter，只读打通完整链路。
7. 补 RocketMQ 和 ActiveMQ 基础只读能力。
8. 实现前端实例管理和资源管理。
9. 实现消费诊断和消息采样。
10. 补审计、权限测试和验收文档。
