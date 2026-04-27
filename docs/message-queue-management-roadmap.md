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
| RabbitMQ | 支持 | 支持 vhost、exchange、queue、binding、consumer | 支持 ready、unacked、consumer | 谨慎支持 queue peek 或采样 | 二期开放 | purge、delete queue、delete exchange | 一期主力 |
| Kafka | 支持 | 支持 broker、topic、partition、config | 支持 consumer group lag | 通过独立 reader 采样，不提交业务 offset | 二期开放 topic 配置 | delete topic、reset offset | 一期主力 |
| RocketMQ | 支持 | 支持 cluster、broker、topic、consumer group | 支持 offset、lag、retry、DLQ 视图 | 视客户端能力分期 | 二期开放 topic/group | delete topic、reset offset | 一期只读优先 |
| ActiveMQ | 支持 | 支持 queue、topic、consumer、connection | 支持 pending、enqueue、dequeue | 谨慎支持 browse | 二期开放 destination | purge、delete destination | 一期只读优先 |
| Pulsar | 支持 | 支持 tenant、namespace、topic、subscription | 支持 backlog、cursor、rate | 通过 reader 采样 | 二期开放 topic/namespace | skip、reset subscription、delete topic | 一期主力 |

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
| `POST` | `/api/v1/message-queues/instances/:id/operations` | 执行资源操作 |
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
8. 高风险操作要求原因和二次确认。
9. 调用 Adapter `ValidateOperation` 获取影响范围。
10. 写入 `mq_operation_audits`，状态为 `pending`。
11. 调用 Adapter `ApplyOperation`。
12. 更新审计状态、结果、耗时和错误信息。
13. 必要时触发元数据同步。

### 消息采样流程
1. 校验登录态和权限。
2. 校验实例对象级 `MESSAGE_READ` 权限。
3. 校验采样条数和 payload 大小限制。
4. 获取实例与凭据。
5. 调用 Adapter 采样。
6. 对 payload 做大小截断、编码识别和脱敏预留。
7. 写入 `mq_message_audits`。
8. 返回样本数据和截断标记。

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

1. `messageQueueHighRiskEnabled`：是否允许高风险 MQ 操作，默认 false。
2. `messageQueueOperationReasonRequired`：高风险操作是否必须填写原因，默认 true。
3. `messageQueueMessageSampleEnabled`：是否允许消息采样，默认 true。
4. `messageQueueMaxSampleMessages`：单次最大采样条数，默认 10。
5. `messageQueueMaxPayloadBytes`：单条最大展示字节数，默认 65536。
6. `messageQueueAuditRetentionDays`：MQ 审计保留天数，默认 180。
7. `messageQueueMetricRetentionDays`：指标快照保留天数，默认 30。
8. `messageQueueSyncTimeoutSeconds`：同步超时，默认 60。

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
