package messagequeue

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	MQTypeRabbitMQ = "rabbitmq"
	MQTypeKafka    = "kafka"
	MQTypeRocketMQ = "rocketmq"
	MQTypeActiveMQ = "activemq"
	MQTypePulsar   = "pulsar"

	InstanceStatusEnabled  = "enabled"
	InstanceStatusDisabled = "disabled"

	HealthStatusUnknown  = "unknown"
	HealthStatusHealthy  = "healthy"
	HealthStatusWarning  = "warning"
	HealthStatusCritical = "critical"

	SyncStatusRunning = "running"
	SyncStatusSuccess = "success"
	SyncStatusFailed  = "failed"

	JobTypeSync             = "sync"
	JobTypeMetricCollect    = "metric_collect"
	JobTypeInspection       = "inspection"
	JobTypeOperationRefresh = "operation_refresh"
	JobTypeExport           = "export"

	JobStatusPending = "pending"
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
	JobStatusPartial = "partial_success"
	JobStatusCancel  = "cancelled"
	JobStatusTimeout = "timeout"

	TriggerManual   = "manual"
	TriggerSchedule = "schedule"

	ResourceTypeBroker       = "broker"
	ResourceTypeTenant       = "tenant"
	ResourceTypeNamespace    = "namespace"
	ResourceTypeVHost        = "vhost"
	ResourceTypeExchange     = "exchange"
	ResourceTypeBinding      = "binding"
	ResourceTypeQueue        = "queue"
	ResourceTypeTopic        = "topic"
	ResourceTypeSubscription = "subscription"
	ResourceTypeConsumer     = "consumer"

	AuditActionConnectionTest = "connection_test"
	AuditActionMetadataSync   = "metadata_sync"
	AuditActionMessageSample  = "message_sample"
	AuditActionMessageSchema  = "message_schema_inspect"
	AuditActionMessageReplay  = "message_replay_apply"
	AuditActionMetricSnapshot = "metric_snapshot_collect"
	AuditActionInspectionRun  = "inspection_run"
	AuditActionConfigClone    = "config_clone_plan"
	AuditActionPermissionSet  = "instance_permission_upsert"
	AuditActionPermissionDel  = "instance_permission_delete"

	OperationActionRabbitMQQueueUpsert     = "rabbitmq_queue_upsert"
	OperationActionRabbitMQExchangeUpsert  = "rabbitmq_exchange_upsert"
	OperationActionRabbitMQBindingUpsert   = "rabbitmq_binding_upsert"
	OperationActionRabbitMQQueuePurge      = "rabbitmq_queue_purge"
	OperationActionRabbitMQQueueDelete     = "rabbitmq_queue_delete"
	OperationActionRabbitMQExchangeDelete  = "rabbitmq_exchange_delete"
	OperationActionKafkaTopicCreate        = "kafka_topic_create"
	OperationActionKafkaPartitionsExpand   = "kafka_partitions_expand"
	OperationActionKafkaTopicConfigUpdate  = "kafka_topic_config_update"
	OperationActionKafkaTopicDelete        = "kafka_topic_delete"
	OperationActionPulsarRetentionUpdate   = "pulsar_namespace_retention_update"
	OperationActionPulsarTTLUpdate         = "pulsar_namespace_ttl_update"
	OperationActionPulsarTopicDelete       = "pulsar_topic_delete"
	OperationActionPulsarSubscriptionSkip  = "pulsar_subscription_skip"
	OperationActionPulsarSubscriptionReset = "pulsar_subscription_reset"

	ConfigKeyMessageQueueHighRiskEnabled         = "messageQueueHighRiskEnabled"
	ConfigKeyMessageQueueOperationReasonRequired = "messageQueueOperationReasonRequired"
	ConfigKeyMessageQueueOperationMaxMetadataAge = "messageQueueOperationMaxMetadataAgeMinutes"
	ConfigKeyMessageQueueOperationMaxMetricAge   = "messageQueueOperationMaxMetricAgeMinutes"
	ConfigKeyMessageQueueRequireFreshMetric      = "messageQueueRequireFreshMetricForHighRisk"

	AuditStatusPending = "pending"
	AuditStatusSuccess = "success"
	AuditStatusFailed  = "failed"
	AuditStatusPartial = "partial_success"

	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"

	PermissionView           uint = 1 << 0
	PermissionDiagnose       uint = 1 << 1
	PermissionMessageRead    uint = 1 << 2
	PermissionMessageExport  uint = 1 << 3
	PermissionMessageWrite   uint = 1 << 4
	PermissionResourceManage uint = 1 << 5
	PermissionHighRisk       uint = 1 << 6
	PermissionAudit          uint = 1 << 7
	PermissionManage         uint = 1 << 8
	PermissionAll                 = PermissionView |
		PermissionDiagnose |
		PermissionMessageRead |
		PermissionMessageExport |
		PermissionMessageWrite |
		PermissionResourceManage |
		PermissionHighRisk |
		PermissionAudit |
		PermissionManage
)

type MQInstance struct {
	gorm.Model
	Name             string     `gorm:"type:varchar(100);not null;comment:实例名称" json:"name"`
	MQType           string     `gorm:"column:mq_type;type:varchar(30);not null;index;comment:消息队列类型" json:"mqType"`
	Engine           string     `gorm:"type:varchar(50);comment:具体引擎" json:"engine"`
	Version          string     `gorm:"type:varchar(120);comment:版本" json:"version"`
	Endpoint         string     `gorm:"type:varchar(500);not null;comment:连接地址，多个地址逗号分隔" json:"endpoint"`
	ManagementURL    string     `gorm:"column:management_url;type:varchar(500);comment:管理API地址" json:"managementUrl"`
	Port             int        `gorm:"type:int;default:0;comment:默认端口" json:"port"`
	CredentialID     uint       `gorm:"column:credential_id;index;comment:凭据ID" json:"credentialId"`
	TLSEnabled       bool       `gorm:"column:tls_enabled;default:false;comment:是否启用TLS" json:"tlsEnabled"`
	ConnectionParams string     `gorm:"column:connection_params;type:text;comment:连接参数JSON" json:"connectionParams"`
	Status           string     `gorm:"type:varchar(20);default:'enabled';index;comment:状态 enabled/disabled" json:"status"`
	HealthStatus     string     `gorm:"column:health_status;type:varchar(20);default:'unknown';index;comment:健康状态" json:"healthStatus"`
	Environment      string     `gorm:"type:varchar(50);index;comment:环境" json:"environment"`
	BusinessSystem   string     `gorm:"column:business_system;type:varchar(100);comment:业务系统" json:"businessSystem"`
	Owner            string     `gorm:"type:varchar(100);comment:负责人" json:"owner"`
	Tags             string     `gorm:"type:varchar(500);comment:标签，逗号分隔" json:"tags"`
	Remark           string     `gorm:"type:varchar(500);comment:备注" json:"remark"`
	LastTestAt       *time.Time `gorm:"column:last_test_at;comment:最近测试时间" json:"lastTestAt,omitempty"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
	LastMetricAt     *time.Time `gorm:"column:last_metric_at;comment:最近指标采集时间" json:"lastMetricAt,omitempty"`
}

func (MQInstance) TableName() string {
	return "mq_instances"
}

type MQInstancePermission struct {
	gorm.Model
	RoleID      uint `gorm:"column:role_id;not null;uniqueIndex:idx_mq_instance_permission;comment:角色ID" json:"roleId"`
	InstanceID  uint `gorm:"column:instance_id;not null;uniqueIndex:idx_mq_instance_permission;index;comment:MQ实例ID" json:"instanceId"`
	Permissions uint `gorm:"column:permissions;type:int unsigned;not null;default:1;comment:权限位图" json:"permissions"`
}

func (MQInstancePermission) TableName() string {
	return "mq_instance_permissions"
}

type MQBroker struct {
	gorm.Model
	InstanceID   uint       `gorm:"column:instance_id;not null;index:idx_mq_broker_unique,unique;comment:实例ID" json:"instanceId"`
	BrokerName   string     `gorm:"column:broker_name;type:varchar(160);not null;index:idx_mq_broker_unique,unique;comment:Broker名称" json:"brokerName"`
	BrokerID     string     `gorm:"column:broker_id;type:varchar(120);comment:Broker ID" json:"brokerId"`
	Host         string     `gorm:"type:varchar(255);comment:主机" json:"host"`
	Port         int        `gorm:"type:int;default:0;comment:端口" json:"port"`
	Role         string     `gorm:"type:varchar(60);comment:角色" json:"role"`
	Status       string     `gorm:"type:varchar(60);comment:状态" json:"status"`
	Version      string     `gorm:"type:varchar(120);comment:版本" json:"version"`
	Rack         string     `gorm:"type:varchar(120);comment:Rack" json:"rack"`
	Zone         string     `gorm:"type:varchar(120);comment:可用区" json:"zone"`
	MetadataJSON string     `gorm:"column:metadata_json;type:text;comment:原始元数据JSON" json:"metadataJson"`
	LastSyncAt   *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (MQBroker) TableName() string {
	return "mq_brokers"
}

type MQResource struct {
	gorm.Model
	InstanceID     uint       `gorm:"column:instance_id;not null;index:idx_mq_resource_unique,unique;comment:实例ID" json:"instanceId"`
	ResourceType   string     `gorm:"column:resource_type;type:varchar(40);not null;index:idx_mq_resource_unique,unique;comment:资源类型" json:"resourceType"`
	Namespace      string     `gorm:"type:varchar(255);index:idx_mq_resource_unique,unique;comment:命名空间/vhost/tenant" json:"namespace"`
	Name           string     `gorm:"type:varchar(255);not null;index:idx_mq_resource_unique,unique;comment:资源名称" json:"name"`
	FullName       string     `gorm:"column:full_name;type:varchar(512);index;comment:完整名称" json:"fullName"`
	Durable        bool       `gorm:"default:false;comment:是否持久化" json:"durable"`
	PartitionCount int        `gorm:"column:partition_count;type:int;default:0;comment:分区数" json:"partitionCount"`
	ReplicaCount   int        `gorm:"column:replica_count;type:int;default:0;comment:副本数" json:"replicaCount"`
	MessageCount   int64      `gorm:"column:message_count;type:bigint;default:0;comment:消息量" json:"messageCount"`
	Backlog        int64      `gorm:"type:bigint;default:0;comment:堆积" json:"backlog"`
	ProducedRate   float64    `gorm:"column:produced_rate;type:double;default:0;comment:生产速率" json:"producedRate"`
	ConsumedRate   float64    `gorm:"column:consumed_rate;type:double;default:0;comment:消费速率" json:"consumedRate"`
	ConsumerCount  int        `gorm:"column:consumer_count;type:int;default:0;comment:消费者数" json:"consumerCount"`
	ConfigJSON     string     `gorm:"column:config_json;type:text;comment:配置JSON" json:"configJson"`
	MetadataJSON   string     `gorm:"column:metadata_json;type:text;comment:元数据JSON" json:"metadataJson"`
	LastSyncAt     *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (MQResource) TableName() string {
	return "mq_resources"
}

type MQBinding struct {
	gorm.Model
	InstanceID      uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	VHost           string     `gorm:"column:vhost;type:varchar(255);index;comment:vhost" json:"vhost"`
	Source          string     `gorm:"type:varchar(255);index;comment:源" json:"source"`
	Destination     string     `gorm:"type:varchar(255);index;comment:目标" json:"destination"`
	DestinationType string     `gorm:"column:destination_type;type:varchar(40);comment:目标类型" json:"destinationType"`
	RoutingKey      string     `gorm:"column:routing_key;type:varchar(255);comment:路由键" json:"routingKey"`
	ArgumentsJSON   string     `gorm:"column:arguments_json;type:text;comment:参数JSON" json:"argumentsJson"`
	LastSyncAt      *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (MQBinding) TableName() string {
	return "mq_bindings"
}

type MQConsumerGroup struct {
	gorm.Model
	InstanceID          uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	ResourceID          uint       `gorm:"column:resource_id;index;comment:资源ID" json:"resourceId"`
	GroupName           string     `gorm:"column:group_name;type:varchar(255);not null;index;comment:消费组/订阅" json:"groupName"`
	ResourceName        string     `gorm:"column:resource_name;type:varchar(512);index;comment:资源名称" json:"resourceName"`
	Namespace           string     `gorm:"type:varchar(255);index;comment:命名空间" json:"namespace"`
	State               string     `gorm:"type:varchar(80);comment:状态" json:"state"`
	ConsumerCount       int        `gorm:"column:consumer_count;type:int;default:0;comment:消费者数" json:"consumerCount"`
	ActiveConsumerCount int        `gorm:"column:active_consumer_count;type:int;default:0;comment:活跃消费者数" json:"activeConsumerCount"`
	CurrentOffset       int64      `gorm:"column:current_offset;type:bigint;default:0;comment:当前位点" json:"currentOffset"`
	EndOffset           int64      `gorm:"column:end_offset;type:bigint;default:0;comment:结束位点" json:"endOffset"`
	Lag                 int64      `gorm:"type:bigint;default:0;comment:Lag" json:"lag"`
	Backlog             int64      `gorm:"type:bigint;default:0;comment:堆积" json:"backlog"`
	LastConsumedAt      *time.Time `gorm:"column:last_consumed_at;comment:最近消费时间" json:"lastConsumedAt,omitempty"`
	MetadataJSON        string     `gorm:"column:metadata_json;type:text;comment:元数据JSON" json:"metadataJson"`
	LastSyncAt          *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (MQConsumerGroup) TableName() string {
	return "mq_consumer_groups"
}

type MQPartition struct {
	gorm.Model
	InstanceID    uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	ResourceID    uint       `gorm:"column:resource_id;index;comment:资源ID" json:"resourceId"`
	ResourceName  string     `gorm:"column:resource_name;type:varchar(512);not null;index;comment:资源名称" json:"resourceName"`
	PartitionID   int        `gorm:"column:partition_id;type:int;not null;index;comment:分区ID" json:"partitionId"`
	Leader        string     `gorm:"type:varchar(255);comment:Leader" json:"leader"`
	ReplicasJSON  string     `gorm:"column:replicas_json;type:text;comment:副本JSON" json:"replicasJson"`
	ISRJSON       string     `gorm:"column:isr_json;type:text;comment:ISR JSON" json:"isrJson"`
	StartOffset   int64      `gorm:"column:start_offset;type:bigint;default:0;comment:起始位点" json:"startOffset"`
	EndOffset     int64      `gorm:"column:end_offset;type:bigint;default:0;comment:结束位点" json:"endOffset"`
	CurrentOffset int64      `gorm:"column:current_offset;type:bigint;default:0;comment:当前位点" json:"currentOffset"`
	Lag           int64      `gorm:"type:bigint;default:0;comment:Lag" json:"lag"`
	Status        string     `gorm:"type:varchar(80);comment:状态" json:"status"`
	LastSyncAt    *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (MQPartition) TableName() string {
	return "mq_partitions"
}

type MQSyncJob struct {
	gorm.Model
	InstanceID         uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	TriggerType        string     `gorm:"column:trigger_type;type:varchar(30);default:'manual';comment:触发类型" json:"triggerType"`
	Status             string     `gorm:"type:varchar(30);default:'running';index;comment:状态" json:"status"`
	StartedAt          *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt         *time.Time `gorm:"column:finished_at;comment:完成时间" json:"finishedAt,omitempty"`
	DurationMs         int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	BrokerCount        int        `gorm:"column:broker_count;type:int;default:0;comment:Broker数" json:"brokerCount"`
	ResourceCount      int        `gorm:"column:resource_count;type:int;default:0;comment:资源数" json:"resourceCount"`
	ConsumerGroupCount int        `gorm:"column:consumer_group_count;type:int;default:0;comment:消费组数" json:"consumerGroupCount"`
	PartitionCount     int        `gorm:"column:partition_count;type:int;default:0;comment:分区数" json:"partitionCount"`
	Message            string     `gorm:"type:varchar(500);comment:消息" json:"message"`
	OperatorID         uint       `gorm:"column:operator_id;comment:操作人ID" json:"operatorId"`
	OperatorName       string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
}

func (MQSyncJob) TableName() string {
	return "mq_sync_jobs"
}

type MQJob struct {
	gorm.Model
	InstanceID      uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	InstanceName    string     `gorm:"column:instance_name;type:varchar(100);comment:实例名称" json:"instanceName"`
	MQType          string     `gorm:"column:mq_type;type:varchar(30);index;comment:MQ类型" json:"mqType"`
	JobType         string     `gorm:"column:job_type;type:varchar(40);not null;index;comment:任务类型" json:"jobType"`
	Status          string     `gorm:"type:varchar(30);default:'pending';index;comment:状态" json:"status"`
	ProgressCurrent int64      `gorm:"column:progress_current;type:bigint;default:0;comment:当前进度" json:"progressCurrent"`
	ProgressTotal   int64      `gorm:"column:progress_total;type:bigint;default:0;comment:总进度" json:"progressTotal"`
	CurrentStage    string     `gorm:"column:current_stage;type:varchar(120);comment:当前阶段" json:"currentStage"`
	TriggerType     string     `gorm:"column:trigger_type;type:varchar(30);default:'manual';comment:触发类型" json:"triggerType"`
	OperatorID      uint       `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName    string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	StartedAt       *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt      *time.Time `gorm:"column:finished_at;comment:完成时间" json:"finishedAt,omitempty"`
	DurationMs      int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	Message         string     `gorm:"type:varchar(500);comment:消息" json:"message"`
	ErrorJSON       string     `gorm:"column:error_json;type:text;comment:错误JSON" json:"errorJson"`
	ResultJSON      string     `gorm:"column:result_json;type:text;comment:结果JSON" json:"resultJson"`
	CorrelationID   string     `gorm:"column:correlation_id;type:varchar(80);index;comment:链路ID" json:"correlationId"`
}

func (MQJob) TableName() string {
	return "mq_jobs"
}

type MQMetricSnapshot struct {
	gorm.Model
	InstanceID        uint      `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	ResourceID        uint      `gorm:"column:resource_id;index;comment:资源ID" json:"resourceId"`
	ResourceType      string    `gorm:"column:resource_type;type:varchar(40);index;comment:资源类型" json:"resourceType"`
	ResourceName      string    `gorm:"column:resource_name;type:varchar(512);index;comment:资源名称" json:"resourceName"`
	BrokerCount       int       `gorm:"column:broker_count;type:int;default:0;comment:Broker数" json:"brokerCount"`
	OnlineBrokerCount int       `gorm:"column:online_broker_count;type:int;default:0;comment:在线Broker数" json:"onlineBrokerCount"`
	MessageCount      int64     `gorm:"column:message_count;type:bigint;default:0;comment:消息量" json:"messageCount"`
	Backlog           int64     `gorm:"type:bigint;default:0;comment:堆积" json:"backlog"`
	Lag               int64     `gorm:"type:bigint;default:0;comment:Lag" json:"lag"`
	ProducedRate      float64   `gorm:"column:produced_rate;type:double;default:0;comment:生产速率" json:"producedRate"`
	ConsumedRate      float64   `gorm:"column:consumed_rate;type:double;default:0;comment:消费速率" json:"consumedRate"`
	ConsumerCount     int       `gorm:"column:consumer_count;type:int;default:0;comment:消费者数" json:"consumerCount"`
	CollectedAt       time.Time `gorm:"column:collected_at;index;comment:采集时间" json:"collectedAt"`
}

func (MQMetricSnapshot) TableName() string {
	return "mq_metric_snapshots"
}

type MQOperationAudit struct {
	gorm.Model
	InstanceID            uint       `gorm:"column:instance_id;index;comment:实例ID" json:"instanceId"`
	InstanceName          string     `gorm:"column:instance_name;type:varchar(100);comment:实例名称" json:"instanceName"`
	MQType                string     `gorm:"column:mq_type;type:varchar(30);index;comment:MQ类型" json:"mqType"`
	ResourceType          string     `gorm:"column:resource_type;type:varchar(40);index;comment:资源类型" json:"resourceType"`
	ResourceName          string     `gorm:"column:resource_name;type:varchar(512);index;comment:资源名称" json:"resourceName"`
	Namespace             string     `gorm:"type:varchar(255);index;comment:命名空间" json:"namespace"`
	Action                string     `gorm:"type:varchar(80);index;comment:动作" json:"action"`
	RiskLevel             string     `gorm:"column:risk_level;type:varchar(30);index;comment:风险等级" json:"riskLevel"`
	Status                string     `gorm:"type:varchar(30);index;comment:状态" json:"status"`
	OperationID           string     `gorm:"column:operation_id;type:varchar(80);index;comment:操作ID" json:"operationId"`
	IdempotencyKey        string     `gorm:"column:idempotency_key;type:varchar(120);index;comment:幂等键" json:"idempotencyKey"`
	LockKey               string     `gorm:"column:lock_key;type:varchar(255);index;comment:资源锁键" json:"lockKey"`
	ConfirmText           string     `gorm:"column:confirm_text;type:varchar(512);comment:确认文本" json:"confirmText"`
	RequestJSON           string     `gorm:"column:request_json;type:text;comment:请求JSON" json:"requestJson"`
	ResultJSON            string     `gorm:"column:result_json;type:text;comment:结果JSON" json:"resultJson"`
	BeforeSnapshotJSON    string     `gorm:"column:before_snapshot_json;type:text;comment:执行前快照" json:"beforeSnapshotJson"`
	AfterSnapshotJSON     string     `gorm:"column:after_snapshot_json;type:text;comment:执行后快照" json:"afterSnapshotJson"`
	DiffJSON              string     `gorm:"column:diff_json;type:text;comment:配置差异" json:"diffJson"`
	WarningsJSON          string     `gorm:"column:warnings_json;type:text;comment:风险提示" json:"warningsJson"`
	ImpactSummaryJSON     string     `gorm:"column:impact_summary_json;type:text;comment:影响摘要" json:"impactSummaryJson"`
	OperationBackupJSON   string     `gorm:"column:operation_backup_json;type:text;comment:操作备份包" json:"operationBackupJson"`
	RollbackHintJSON      string     `gorm:"column:rollback_hint_json;type:text;comment:回滚建议" json:"rollbackHintJson"`
	RollbackSupported     bool       `gorm:"column:rollback_supported;default:false;comment:是否支持回滚辅助" json:"rollbackSupported"`
	RollbackRiskLevel     string     `gorm:"column:rollback_risk_level;type:varchar(30);comment:回滚风险等级" json:"rollbackRiskLevel"`
	MetadataRefreshStatus string     `gorm:"column:metadata_refresh_status;type:varchar(30);comment:后置元数据刷新状态" json:"metadataRefreshStatus"`
	MetadataRefreshError  string     `gorm:"column:metadata_refresh_error;type:varchar(500);comment:后置元数据刷新错误" json:"metadataRefreshError"`
	Reason                string     `gorm:"type:varchar(500);comment:原因" json:"reason"`
	OperatorID            uint       `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName          string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	ClientIP              string     `gorm:"column:client_ip;type:varchar(80);comment:客户端IP" json:"clientIp"`
	StartedAt             *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt            *time.Time `gorm:"column:finished_at;comment:完成时间" json:"finishedAt,omitempty"`
	DurationMs            int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	Message               string     `gorm:"type:varchar(500);comment:消息" json:"message"`
	PreviousAuditHash     string     `gorm:"column:previous_audit_hash;type:varchar(80);comment:上一条审计hash" json:"previousAuditHash"`
	AuditHash             string     `gorm:"column:audit_hash;type:varchar(80);index;comment:审计hash" json:"auditHash"`
}

func (MQOperationAudit) TableName() string {
	return "mq_operation_audits"
}

type MQMessageAudit struct {
	gorm.Model
	InstanceID        uint   `gorm:"column:instance_id;index;comment:实例ID" json:"instanceId"`
	MQType            string `gorm:"column:mq_type;type:varchar(30);index;comment:MQ类型" json:"mqType"`
	ResourceType      string `gorm:"column:resource_type;type:varchar(40);index;comment:资源类型" json:"resourceType"`
	ResourceName      string `gorm:"column:resource_name;type:varchar(512);index;comment:资源名称" json:"resourceName"`
	Namespace         string `gorm:"type:varchar(255);index;comment:命名空间" json:"namespace"`
	Action            string `gorm:"type:varchar(80);index;comment:动作" json:"action"`
	SampleCount       int    `gorm:"column:sample_count;type:int;default:0;comment:采样条数" json:"sampleCount"`
	PayloadBytes      int    `gorm:"column:payload_bytes;type:int;default:0;comment:payload字节数" json:"payloadBytes"`
	FilterJSON        string `gorm:"column:filter_json;type:text;comment:过滤条件JSON" json:"filterJson"`
	PayloadHash       string `gorm:"column:payload_hash;type:varchar(80);index;comment:payload摘要hash" json:"payloadHash"`
	SensitiveHitCount int    `gorm:"column:sensitive_hit_count;type:int;default:0;comment:敏感字段命中数" json:"sensitiveHitCount"`
	RawPayloadVisible bool   `gorm:"column:raw_payload_visible;default:false;comment:是否查看原文" json:"rawPayloadVisible"`
	DLPResultJSON     string `gorm:"column:dlp_result_json;type:text;comment:DLP处理结果JSON" json:"dlpResultJson"`
	Status            string `gorm:"type:varchar(30);index;comment:状态" json:"status"`
	OperatorID        uint   `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName      string `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	ClientIP          string `gorm:"column:client_ip;type:varchar(80);comment:客户端IP" json:"clientIp"`
	Message           string `gorm:"type:varchar(500);comment:消息" json:"message"`
	PreviousAuditHash string `gorm:"column:previous_audit_hash;type:varchar(80);comment:上一条审计hash" json:"previousAuditHash"`
	AuditHash         string `gorm:"column:audit_hash;type:varchar(80);index;comment:审计hash" json:"auditHash"`
}

func (MQMessageAudit) TableName() string {
	return "mq_message_audits"
}

type MQDLQRecord struct {
	gorm.Model
	InstanceID     uint   `gorm:"column:instance_id;not null;index:idx_mq_dlq_record_unique,unique;comment:实例ID" json:"instanceId"`
	ResourceID     uint   `gorm:"column:resource_id;index;comment:资源ID" json:"resourceId"`
	ResourceType   string `gorm:"column:resource_type;type:varchar(40);not null;index:idx_mq_dlq_record_unique,unique;comment:资源类型" json:"resourceType"`
	Namespace      string `gorm:"type:varchar(255);index:idx_mq_dlq_record_unique,unique;comment:命名空间" json:"namespace"`
	ResourceName   string `gorm:"column:resource_name;type:varchar(512);not null;index:idx_mq_dlq_record_unique,unique;comment:资源名称" json:"resourceName"`
	Kind           string `gorm:"type:varchar(30);index;comment:dlq/retry" json:"kind"`
	HandlingStatus string `gorm:"column:handling_status;type:varchar(40);default:'untriaged';index;comment:处理状态" json:"handlingStatus"`
	Owner          string `gorm:"type:varchar(100);comment:负责人" json:"owner"`
	Remark         string `gorm:"type:varchar(1000);comment:处理备注" json:"remark"`
	LastBacklog    int64  `gorm:"column:last_backlog;type:bigint;default:0;comment:最近积压" json:"lastBacklog"`
	LastMessage    string `gorm:"column:last_message;type:varchar(500);comment:最近处理说明" json:"lastMessage"`
	UpdatedByID    uint   `gorm:"column:updated_by_id;index;comment:更新人ID" json:"updatedById"`
	UpdatedByName  string `gorm:"column:updated_by_name;type:varchar(100);comment:更新人" json:"updatedByName"`
}

func (MQDLQRecord) TableName() string {
	return "mq_dlq_records"
}

type ConnectionCredential struct {
	Username   string
	Password   string
	PrivateKey string
	Passphrase string
}

type Operator struct {
	ID       uint
	Username string
	ClientIP string
}

type InstanceRequest struct {
	ID               uint   `json:"id"`
	Name             string `json:"name" binding:"required,min=2,max=100"`
	MQType           string `json:"mqType" binding:"required"`
	Endpoint         string `json:"endpoint" binding:"required,max=500"`
	ManagementURL    string `json:"managementUrl" binding:"omitempty,max=500"`
	Port             int    `json:"port" binding:"omitempty,min=0,max=65535"`
	CredentialID     uint   `json:"credentialId"`
	TLSEnabled       bool   `json:"tlsEnabled"`
	ConnectionParams string `json:"connectionParams"`
	Status           string `json:"status" binding:"omitempty,oneof=enabled disabled"`
	Environment      string `json:"environment" binding:"omitempty,max=50"`
	BusinessSystem   string `json:"businessSystem" binding:"omitempty,max=100"`
	Owner            string `json:"owner" binding:"omitempty,max=100"`
	Tags             string `json:"tags" binding:"omitempty,max=500"`
	Remark           string `json:"remark" binding:"omitempty,max=500"`
}

type InstanceListRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	Keyword           string `form:"keyword"`
	MQType            string `form:"mqType"`
	Status            string `form:"status"`
	HealthStatus      string `form:"healthStatus"`
	Environment       string `form:"environment"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type ResourceListRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	Keyword      string `form:"keyword"`
	ResourceType string `form:"resourceType"`
	Namespace    string `form:"namespace"`
	HasBacklog   string `form:"hasBacklog"`
}

type ConsumerGroupListRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	Keyword      string `form:"keyword"`
	ResourceName string `form:"resourceName"`
	Namespace    string `form:"namespace"`
	HasLag       string `form:"hasLag"`
}

type PartitionListRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	ResourceName string `form:"resourceName"`
}

type AuditListRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	Keyword           string `form:"keyword"`
	InstanceID        uint   `form:"instanceId"`
	MQType            string `form:"mqType"`
	Action            string `form:"action"`
	Status            string `form:"status"`
	RiskLevel         string `form:"riskLevel"`
	StartTime         string `form:"startTime"`
	EndTime           string `form:"endTime"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type MetricSnapshotListRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	ResourceType string `form:"resourceType"`
	ResourceName string `form:"resourceName"`
	StartTime    string `form:"startTime"`
	EndTime      string `form:"endTime"`
}

type JobListRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	Keyword           string `form:"keyword"`
	InstanceID        uint   `form:"instanceId"`
	MQType            string `form:"mqType"`
	JobType           string `form:"jobType"`
	Status            string `form:"status"`
	StartTime         string `form:"startTime"`
	EndTime           string `form:"endTime"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type GovernanceReportRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	InstanceID        uint   `form:"instanceId"`
	Severity          string `form:"severity"`
	Category          string `form:"category"`
	Keyword           string `form:"keyword"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type DLQAnalysisRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	InstanceID        uint   `form:"instanceId"`
	Keyword           string `form:"keyword"`
	Kind              string `form:"kind"`
	HasBacklog        string `form:"hasBacklog"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type DLQRecordRequest struct {
	InstanceID     uint   `json:"instanceId" binding:"required"`
	ResourceID     uint   `json:"resourceId"`
	ResourceType   string `json:"resourceType" binding:"required,max=40"`
	Namespace      string `json:"namespace" binding:"omitempty,max=255"`
	ResourceName   string `json:"resourceName" binding:"required,max=512"`
	Kind           string `json:"kind" binding:"omitempty,max=30"`
	HandlingStatus string `json:"handlingStatus" binding:"required,max=40"`
	Owner          string `json:"owner" binding:"omitempty,max=100"`
	Remark         string `json:"remark" binding:"omitempty,max=1000"`
	LastBacklog    int64  `json:"lastBacklog"`
	LastMessage    string `json:"lastMessage" binding:"omitempty,max=500"`
}

type InstancePermissionListRequest struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	RoleID     uint   `form:"roleId"`
	InstanceID uint   `form:"instanceId"`
	Keyword    string `form:"keyword"`
}

type InstancePermissionRequest struct {
	RoleID      uint `json:"roleId" binding:"required"`
	InstanceID  uint `json:"instanceId" binding:"required"`
	Permissions uint `json:"permissions" binding:"required"`
}

type MessageSampleRequest struct {
	ResourceType string `json:"resourceType" binding:"omitempty,max=40"`
	Namespace    string `json:"namespace" binding:"omitempty,max=255"`
	ResourceName string `json:"resourceName" binding:"required,max=512"`
	GroupName    string `json:"groupName" binding:"omitempty,max=255"`
	PartitionID  *int   `json:"partitionId"`
	Offset       *int64 `json:"offset"`
	Key          string `json:"key" binding:"omitempty,max=255"`
	Limit        int    `json:"limit" binding:"omitempty,min=1,max=10"`
	MaxBytes     int    `json:"maxBytes" binding:"omitempty,min=1,max=262144"`
}

type MessageReplayApplicationRequest struct {
	ResourceType       string `json:"resourceType" binding:"omitempty,max=40"`
	Namespace          string `json:"namespace" binding:"omitempty,max=255"`
	ResourceName       string `json:"resourceName" binding:"required,max=512"`
	TargetInstanceID   uint   `json:"targetInstanceId"`
	TargetResourceName string `json:"targetResourceName" binding:"omitempty,max=512"`
	MaxMessages        int    `json:"maxMessages" binding:"omitempty,min=1,max=10000"`
	RateLimitPerSecond int    `json:"rateLimitPerSecond" binding:"omitempty,min=1,max=10000"`
	Reason             string `json:"reason" binding:"required,max=500"`
}

type ResourceOperationRequest struct {
	Action         string         `json:"action" binding:"required,max=80"`
	ResourceType   string         `json:"resourceType" binding:"omitempty,max=40"`
	Namespace      string         `json:"namespace" binding:"omitempty,max=255"`
	ResourceName   string         `json:"resourceName" binding:"omitempty,max=512"`
	Reason         string         `json:"reason" binding:"omitempty,max=500"`
	ConfirmText    string         `json:"confirmText" binding:"omitempty,max=512"`
	Confirmed      bool           `json:"confirmed"`
	IdempotencyKey string         `json:"idempotencyKey" binding:"omitempty,max=120"`
	Params         map[string]any `json:"params"`
}

type SupportedTypeVO struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	DefaultPort       int    `json:"defaultPort"`
	DefaultManagement int    `json:"defaultManagementPort"`
	TestEnabled       bool   `json:"testEnabled"`
	MetadataEnabled   bool   `json:"metadataEnabled"`
	DiagnosisEnabled  bool   `json:"diagnosisEnabled"`
	MessageSample     bool   `json:"messageSampleEnabled"`
	ResourceManage    bool   `json:"resourceManageEnabled"`
	Phase             string `json:"phase"`
}

type InstanceVO struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	MQType           string `json:"mqType"`
	MQTypeText       string `json:"mqTypeText"`
	Engine           string `json:"engine"`
	Version          string `json:"version"`
	Endpoint         string `json:"endpoint"`
	ManagementURL    string `json:"managementUrl"`
	Port             int    `json:"port"`
	CredentialID     uint   `json:"credentialId"`
	TLSEnabled       bool   `json:"tlsEnabled"`
	ConnectionParams string `json:"connectionParams"`
	Status           string `json:"status"`
	StatusText       string `json:"statusText"`
	HealthStatus     string `json:"healthStatus"`
	HealthText       string `json:"healthText"`
	Environment      string `json:"environment"`
	BusinessSystem   string `json:"businessSystem"`
	Owner            string `json:"owner"`
	Tags             string `json:"tags"`
	Remark           string `json:"remark"`
	LastTestAt       string `json:"lastTestAt,omitempty"`
	LastSyncAt       string `json:"lastSyncAt,omitempty"`
	LastMetricAt     string `json:"lastMetricAt,omitempty"`
	Permissions      uint   `json:"permissions"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type InstancePermissionVO struct {
	ID           uint   `json:"id"`
	RoleID       uint   `json:"roleId"`
	RoleName     string `json:"roleName"`
	RoleCode     string `json:"roleCode"`
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	Permissions  uint   `json:"permissions"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type ConnectionTestResultVO struct {
	InstanceID      uint   `json:"instanceId"`
	Name            string `json:"name"`
	MQType          string `json:"mqType"`
	Version         string `json:"version"`
	ClusterName     string `json:"clusterName"`
	BrokerCount     int    `json:"brokerCount"`
	ManagementReady bool   `json:"managementReady"`
	LatencyMs       int64  `json:"latencyMs"`
	Message         string `json:"message"`
	TestedAt        string `json:"testedAt"`
}

type MetadataSyncResultVO struct {
	JobID              uint   `json:"jobId"`
	InstanceID         uint   `json:"instanceId"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	Message            string `json:"message"`
	Version            string `json:"version"`
	DurationMs         int64  `json:"durationMs"`
	BrokerCount        int    `json:"brokerCount"`
	ResourceCount      int    `json:"resourceCount"`
	ConsumerGroupCount int    `json:"consumerGroupCount"`
	PartitionCount     int    `json:"partitionCount"`
	SyncedAt           string `json:"syncedAt"`
}

type OverviewVO struct {
	InstanceID          uint          `json:"instanceId"`
	InstanceName        string        `json:"instanceName"`
	MQType              string        `json:"mqType"`
	HealthStatus        string        `json:"healthStatus"`
	HealthText          string        `json:"healthText"`
	HealthReasons       []string      `json:"healthReasons"`
	AnomalyTags         []string      `json:"anomalyTags"`
	BrokerCount         int64         `json:"brokerCount"`
	OnlineBrokerCount   int64         `json:"onlineBrokerCount"`
	ResourceCount       int64         `json:"resourceCount"`
	ConsumerGroupCount  int64         `json:"consumerGroupCount"`
	PartitionCount      int64         `json:"partitionCount"`
	MessageCount        int64         `json:"messageCount"`
	Backlog             int64         `json:"backlog"`
	Lag                 int64         `json:"lag"`
	ProducedRate        float64       `json:"producedRate"`
	ConsumedRate        float64       `json:"consumedRate"`
	NoConsumerResources int64         `json:"noConsumerResources"`
	DLQResources        int64         `json:"dlqResources"`
	RetryResources      int64         `json:"retryResources"`
	TopBacklogResources []*ResourceVO `json:"topBacklogResources"`
	LastSyncAt          string        `json:"lastSyncAt,omitempty"`
	LastMetricAt        string        `json:"lastMetricAt,omitempty"`
}

type CountStatVO struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type DashboardResourceVO struct {
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	MQType       string `json:"mqType"`
	ResourceType string `json:"resourceType"`
	Namespace    string `json:"namespace"`
	ResourceName string `json:"resourceName"`
	Backlog      int64  `json:"backlog"`
	MessageCount int64  `json:"messageCount"`
	Consumer     int    `json:"consumerCount"`
	LastSyncAt   string `json:"lastSyncAt,omitempty"`
}

type DashboardConsumerGroupVO struct {
	InstanceID          uint   `json:"instanceId"`
	InstanceName        string `json:"instanceName"`
	MQType              string `json:"mqType"`
	Namespace           string `json:"namespace"`
	ResourceName        string `json:"resourceName"`
	GroupName           string `json:"groupName"`
	Lag                 int64  `json:"lag"`
	Backlog             int64  `json:"backlog"`
	ConsumerCount       int    `json:"consumerCount"`
	ActiveConsumerCount int    `json:"activeConsumerCount"`
	LastSyncAt          string `json:"lastSyncAt,omitempty"`
}

type ProductionDashboardVO struct {
	InstanceTotal           int64                       `json:"instanceTotal"`
	HealthyInstances        int64                       `json:"healthyInstances"`
	WarningInstances        int64                       `json:"warningInstances"`
	CriticalInstances       int64                       `json:"criticalInstances"`
	UnknownInstances        int64                       `json:"unknownInstances"`
	ResourceTotal           int64                       `json:"resourceTotal"`
	ConsumerGroupTotal      int64                       `json:"consumerGroupTotal"`
	BacklogTotal            int64                       `json:"backlogTotal"`
	LagTotal                int64                       `json:"lagTotal"`
	DLQResourceTotal        int64                       `json:"dlqResourceTotal"`
	RetryResourceTotal      int64                       `json:"retryResourceTotal"`
	NoOwnerResourceTotal    int64                       `json:"noOwnerResourceTotal"`
	NoOwnerInstanceTotal    int64                       `json:"noOwnerInstanceTotal"`
	NoBusinessInstanceTotal int64                       `json:"noBusinessInstanceTotal"`
	TypeStats               []*CountStatVO              `json:"typeStats"`
	EnvironmentStats        []*CountStatVO              `json:"environmentStats"`
	HealthStats             []*CountStatVO              `json:"healthStats"`
	TopBacklogResources     []*DashboardResourceVO      `json:"topBacklogResources"`
	TopLagConsumerGroups    []*DashboardConsumerGroupVO `json:"topLagConsumerGroups"`
	RecentHighRiskAudits    []*AuditVO                  `json:"recentHighRiskAudits"`
	RecentFailedJobs        []*JobVO                    `json:"recentFailedJobs"`
	AlertCandidates         []*GovernanceViolationVO    `json:"alertCandidates"`
	GeneratedAt             string                      `json:"generatedAt"`
}

type GovernanceViolationVO struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Suggestion   string `json:"suggestion"`
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	MQType       string `json:"mqType"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
	MetricValue  string `json:"metricValue,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

type GovernanceReportVO struct {
	Total              int64                    `json:"total"`
	CriticalCount      int64                    `json:"criticalCount"`
	WarningCount       int64                    `json:"warningCount"`
	InfoCount          int64                    `json:"infoCount"`
	OwnerMissingCount  int64                    `json:"ownerMissingCount"`
	BaselineDriftCount int64                    `json:"baselineDriftCount"`
	LifecycleRiskCount int64                    `json:"lifecycleRiskCount"`
	Violations         []*GovernanceViolationVO `json:"violations"`
	GeneratedAt        string                   `json:"generatedAt"`
}

type DLQAnalysisSummaryVO struct {
	DLQTotal          int64 `json:"dlqTotal"`
	RetryTotal        int64 `json:"retryTotal"`
	BacklogTotal      int64 `json:"backlogTotal"`
	NoConsumerTotal   int64 `json:"noConsumerTotal"`
	ReplayRequestable int64 `json:"replayRequestable"`
}

type DLQAnalysisVO struct {
	Total         int64                 `json:"total"`
	Page          int                   `json:"page"`
	PageSize      int                   `json:"pageSize"`
	Summary       DLQAnalysisSummaryVO  `json:"summary"`
	Items         []*DLQResourceIssueVO `json:"items"`
	ReplayEnabled bool                  `json:"replayEnabled"`
	GeneratedAt   string                `json:"generatedAt"`
}

type DLQResourceIssueVO struct {
	InstanceID          uint                 `json:"instanceId"`
	InstanceName        string               `json:"instanceName"`
	MQType              string               `json:"mqType"`
	MQTypeText          string               `json:"mqTypeText"`
	Environment         string               `json:"environment"`
	BusinessSystem      string               `json:"businessSystem"`
	Owner               string               `json:"owner"`
	ResourceID          uint                 `json:"resourceId"`
	ResourceType        string               `json:"resourceType"`
	Namespace           string               `json:"namespace"`
	ResourceName        string               `json:"resourceName"`
	FullName            string               `json:"fullName"`
	Kind                string               `json:"kind"`
	Severity            string               `json:"severity"`
	MessageCount        int64                `json:"messageCount"`
	Backlog             int64                `json:"backlog"`
	ConsumerCount       int                  `json:"consumerCount"`
	ProducedRate        float64              `json:"producedRate"`
	ConsumedRate        float64              `json:"consumedRate"`
	RelatedResourceName string               `json:"relatedResourceName"`
	HandlingStatus      string               `json:"handlingStatus"`
	HandlingRemark      string               `json:"handlingRemark,omitempty"`
	HandlingUpdatedBy   string               `json:"handlingUpdatedBy,omitempty"`
	HandlingUpdatedAt   string               `json:"handlingUpdatedAt,omitempty"`
	Suggestion          string               `json:"suggestion"`
	LastSyncAt          string               `json:"lastSyncAt,omitempty"`
	ConsumerGroups      []*ConsumerGroupVO   `json:"consumerGroups"`
	RecentAudits        []*AuditVO           `json:"recentAudits"`
	ErrorSummary        []*DLQErrorSummaryVO `json:"errorSummary"`
}

type DLQErrorSummaryVO struct {
	Type        string `json:"type"`
	Count       int64  `json:"count"`
	Description string `json:"description"`
}

type TopologyNodeVO struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Type     string         `json:"type"`
	Status   string         `json:"status,omitempty"`
	Metrics  map[string]any `json:"metrics,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type TopologyEdgeVO struct {
	ID     string         `json:"id"`
	From   string         `json:"from"`
	To     string         `json:"to"`
	Type   string         `json:"type"`
	Label  string         `json:"label,omitempty"`
	Metric map[string]any `json:"metric,omitempty"`
}

type TopologyVO struct {
	InstanceID   uint              `json:"instanceId"`
	InstanceName string            `json:"instanceName"`
	MQType       string            `json:"mqType"`
	Nodes        []*TopologyNodeVO `json:"nodes"`
	Edges        []*TopologyEdgeVO `json:"edges"`
	Warnings     []string          `json:"warnings"`
	GeneratedAt  string            `json:"generatedAt"`
}

type MetricSnapshotVO struct {
	ID                uint    `json:"id"`
	InstanceID        uint    `json:"instanceId"`
	ResourceID        uint    `json:"resourceId"`
	ResourceType      string  `json:"resourceType"`
	ResourceName      string  `json:"resourceName"`
	BrokerCount       int     `json:"brokerCount"`
	OnlineBrokerCount int     `json:"onlineBrokerCount"`
	MessageCount      int64   `json:"messageCount"`
	Backlog           int64   `json:"backlog"`
	Lag               int64   `json:"lag"`
	ProducedRate      float64 `json:"producedRate"`
	ConsumedRate      float64 `json:"consumedRate"`
	ConsumerCount     int     `json:"consumerCount"`
	CollectedAt       string  `json:"collectedAt"`
}

type MetricCollectResultVO struct {
	InstanceID    uint              `json:"instanceId"`
	InstanceName  string            `json:"instanceName"`
	MQType        string            `json:"mqType"`
	HealthStatus  string            `json:"healthStatus"`
	HealthText    string            `json:"healthText"`
	HealthReasons []string          `json:"healthReasons"`
	AnomalyTags   []string          `json:"anomalyTags"`
	Snapshot      *MetricSnapshotVO `json:"snapshot"`
	Message       string            `json:"message"`
	CollectedAt   string            `json:"collectedAt"`
}

type JobVO struct {
	ID              uint   `json:"id"`
	InstanceID      uint   `json:"instanceId"`
	InstanceName    string `json:"instanceName"`
	MQType          string `json:"mqType"`
	MQTypeText      string `json:"mqTypeText"`
	JobType         string `json:"jobType"`
	JobTypeText     string `json:"jobTypeText"`
	Status          string `json:"status"`
	StatusText      string `json:"statusText"`
	ProgressCurrent int64  `json:"progressCurrent"`
	ProgressTotal   int64  `json:"progressTotal"`
	ProgressPercent int    `json:"progressPercent"`
	CurrentStage    string `json:"currentStage"`
	TriggerType     string `json:"triggerType"`
	OperatorID      uint   `json:"operatorId"`
	OperatorName    string `json:"operatorName"`
	StartedAt       string `json:"startedAt,omitempty"`
	FinishedAt      string `json:"finishedAt,omitempty"`
	DurationMs      int64  `json:"durationMs"`
	Message         string `json:"message"`
	ErrorJSON       string `json:"errorJson,omitempty"`
	ResultJSON      string `json:"resultJson,omitempty"`
	CorrelationID   string `json:"correlationId"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type InspectionMetricVO struct {
	Name   string `json:"name"`
	Value  any    `json:"value"`
	Status string `json:"status"`
}

type InspectionSectionVO struct {
	Key     string                `json:"key"`
	Title   string                `json:"title"`
	Status  string                `json:"status"`
	Summary string                `json:"summary"`
	Metrics []*InspectionMetricVO `json:"metrics"`
}

type InspectionFindingVO struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
}

type InspectionReportVO struct {
	InstanceID   uint                   `json:"instanceId"`
	InstanceName string                 `json:"instanceName"`
	MQType       string                 `json:"mqType"`
	Score        int                    `json:"score"`
	RiskLevel    string                 `json:"riskLevel"`
	Summary      string                 `json:"summary"`
	Sections     []*InspectionSectionVO `json:"sections"`
	Findings     []*InspectionFindingVO `json:"findings"`
	GeneratedAt  string                 `json:"generatedAt"`
}

type BrokerVO struct {
	ID         uint   `json:"id"`
	InstanceID uint   `json:"instanceId"`
	BrokerName string `json:"brokerName"`
	BrokerID   string `json:"brokerId"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	Rack       string `json:"rack"`
	Zone       string `json:"zone"`
	LastSyncAt string `json:"lastSyncAt,omitempty"`
}

type ResourceVO struct {
	ID               uint    `json:"id"`
	InstanceID       uint    `json:"instanceId"`
	ResourceType     string  `json:"resourceType"`
	ResourceTypeText string  `json:"resourceTypeText"`
	Namespace        string  `json:"namespace"`
	Name             string  `json:"name"`
	FullName         string  `json:"fullName"`
	Durable          bool    `json:"durable"`
	PartitionCount   int     `json:"partitionCount"`
	ReplicaCount     int     `json:"replicaCount"`
	MessageCount     int64   `json:"messageCount"`
	Backlog          int64   `json:"backlog"`
	ProducedRate     float64 `json:"producedRate"`
	ConsumedRate     float64 `json:"consumedRate"`
	ConsumerCount    int     `json:"consumerCount"`
	LastSyncAt       string  `json:"lastSyncAt,omitempty"`
}

type BindingVO struct {
	ID              uint   `json:"id"`
	InstanceID      uint   `json:"instanceId"`
	VHost           string `json:"vhost"`
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	DestinationType string `json:"destinationType"`
	RoutingKey      string `json:"routingKey"`
	LastSyncAt      string `json:"lastSyncAt,omitempty"`
}

type ConsumerGroupVO struct {
	ID                  uint   `json:"id"`
	InstanceID          uint   `json:"instanceId"`
	ResourceID          uint   `json:"resourceId"`
	GroupName           string `json:"groupName"`
	ResourceName        string `json:"resourceName"`
	Namespace           string `json:"namespace"`
	State               string `json:"state"`
	ConsumerCount       int    `json:"consumerCount"`
	ActiveConsumerCount int    `json:"activeConsumerCount"`
	CurrentOffset       int64  `json:"currentOffset"`
	EndOffset           int64  `json:"endOffset"`
	Lag                 int64  `json:"lag"`
	Backlog             int64  `json:"backlog"`
	LastConsumedAt      string `json:"lastConsumedAt,omitempty"`
	LastSyncAt          string `json:"lastSyncAt,omitempty"`
}

type PartitionVO struct {
	ID            uint   `json:"id"`
	InstanceID    uint   `json:"instanceId"`
	ResourceID    uint   `json:"resourceId"`
	ResourceName  string `json:"resourceName"`
	PartitionID   int    `json:"partitionId"`
	Leader        string `json:"leader"`
	StartOffset   int64  `json:"startOffset"`
	EndOffset     int64  `json:"endOffset"`
	CurrentOffset int64  `json:"currentOffset"`
	Lag           int64  `json:"lag"`
	Status        string `json:"status"`
	LastSyncAt    string `json:"lastSyncAt,omitempty"`
}

type AuditVO struct {
	ID                  uint   `json:"id"`
	InstanceID          uint   `json:"instanceId"`
	InstanceName        string `json:"instanceName,omitempty"`
	MQType              string `json:"mqType"`
	ResourceType        string `json:"resourceType"`
	ResourceName        string `json:"resourceName"`
	Namespace           string `json:"namespace"`
	Action              string `json:"action"`
	ActionText          string `json:"actionText"`
	RiskLevel           string `json:"riskLevel"`
	Status              string `json:"status"`
	StatusText          string `json:"statusText"`
	Reason              string `json:"reason,omitempty"`
	OperationBackupJson string `json:"operationBackupJson,omitempty"`
	RollbackHintJson    string `json:"rollbackHintJson,omitempty"`
	RollbackSupported   bool   `json:"rollbackSupported"`
	RollbackRiskLevel   string `json:"rollbackRiskLevel,omitempty"`
	OperatorID          uint   `json:"operatorId"`
	OperatorName        string `json:"operatorName"`
	ClientIP            string `json:"clientIp"`
	DurationMs          int64  `json:"durationMs"`
	SampleCount         int    `json:"sampleCount,omitempty"`
	PayloadBytes        int    `json:"payloadBytes,omitempty"`
	PayloadHash         string `json:"payloadHash,omitempty"`
	SensitiveHitCount   int    `json:"sensitiveHitCount,omitempty"`
	RawPayloadVisible   bool   `json:"rawPayloadVisible,omitempty"`
	PreviousAuditHash   string `json:"previousAuditHash,omitempty"`
	AuditHash           string `json:"auditHash,omitempty"`
	Message             string `json:"message"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type MessageSampleResultVO struct {
	InstanceID        uint              `json:"instanceId"`
	MQType            string            `json:"mqType"`
	ResourceName      string            `json:"resourceName"`
	Namespace         string            `json:"namespace"`
	Samples           []MessageSampleVO `json:"samples"`
	SampleCount       int               `json:"sampleCount"`
	SensitiveHitCount int               `json:"sensitiveHitCount"`
	PayloadHash       string            `json:"payloadHash"`
	Redacted          bool              `json:"redacted"`
	Truncated         bool              `json:"truncated"`
	Message           string            `json:"message"`
	SampledAt         string            `json:"sampledAt"`
}

type MessageSampleVO struct {
	Topic             string            `json:"topic"`
	PartitionID       int               `json:"partitionId"`
	Offset            int64             `json:"offset"`
	Key               string            `json:"key"`
	Timestamp         string            `json:"timestamp"`
	Headers           map[string]string `json:"headers"`
	Payload           string            `json:"payload"`
	PayloadSize       int               `json:"payloadSize"`
	Truncated         bool              `json:"truncated"`
	Encoding          string            `json:"encoding"`
	PayloadHash       string            `json:"payloadHash"`
	Redacted          bool              `json:"redacted"`
	SensitiveHitCount int               `json:"sensitiveHitCount"`
	SensitiveFields   []string          `json:"sensitiveFields"`
}

type MessageReplayPlanVO struct {
	InstanceID         uint           `json:"instanceId"`
	InstanceName       string         `json:"instanceName"`
	MQType             string         `json:"mqType"`
	ResourceType       string         `json:"resourceType"`
	Namespace          string         `json:"namespace"`
	ResourceName       string         `json:"resourceName"`
	TargetInstanceID   uint           `json:"targetInstanceId"`
	TargetResourceName string         `json:"targetResourceName"`
	MaxMessages        int            `json:"maxMessages"`
	RateLimitPerSecond int            `json:"rateLimitPerSecond"`
	Status             string         `json:"status"`
	ReplayExecutable   bool           `json:"replayExecutable"`
	RequiresApproval   bool           `json:"requiresApproval"`
	Warnings           []string       `json:"warnings"`
	Impact             map[string]any `json:"impact"`
	Message            string         `json:"message"`
	AuditID            uint           `json:"auditId"`
	CreatedAt          string         `json:"createdAt"`
}

type MessageSchemaInspectRequest struct {
	ResourceType string `json:"resourceType" binding:"omitempty,max=40"`
	Namespace    string `json:"namespace" binding:"omitempty,max=255"`
	ResourceName string `json:"resourceName" binding:"omitempty,max=512"`
	Payload      string `json:"payload" binding:"required,max=262144"`
	SchemaJSON   string `json:"schemaJson" binding:"omitempty,max=262144"`
	Strict       bool   `json:"strict"`
}

type MessageSchemaFieldVO struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	Required  bool   `json:"required"`
	Sensitive bool   `json:"sensitive"`
	Example   string `json:"example,omitempty"`
}

type MessageSchemaValidationIssueVO struct {
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type MessageSchemaInspectVO struct {
	InstanceID        uint                              `json:"instanceId"`
	InstanceName      string                            `json:"instanceName"`
	MQType            string                            `json:"mqType"`
	ResourceType      string                            `json:"resourceType"`
	Namespace         string                            `json:"namespace"`
	ResourceName      string                            `json:"resourceName"`
	Format            string                            `json:"format"`
	Valid             bool                              `json:"valid"`
	Strict            bool                              `json:"strict"`
	PayloadBytes      int                               `json:"payloadBytes"`
	PayloadHash       string                            `json:"payloadHash"`
	PrettyPayload     string                            `json:"prettyPayload,omitempty"`
	InferredSchema    map[string]any                    `json:"inferredSchema,omitempty"`
	Fields            []*MessageSchemaFieldVO           `json:"fields"`
	Issues            []*MessageSchemaValidationIssueVO `json:"issues"`
	SensitiveHitCount int                               `json:"sensitiveHitCount"`
	SensitiveFields   []string                          `json:"sensitiveFields"`
	RedactedPreview   string                            `json:"redactedPreview,omitempty"`
	Message           string                            `json:"message"`
	GeneratedAt       string                            `json:"generatedAt"`
}

type ConfigClonePlanRequest struct {
	ResourceType            string `json:"resourceType" binding:"required,max=40"`
	Namespace               string `json:"namespace" binding:"omitempty,max=255"`
	ResourceName            string `json:"resourceName" binding:"required,max=512"`
	TargetInstanceID        uint   `json:"targetInstanceId" binding:"required"`
	TargetNamespace         string `json:"targetNamespace" binding:"omitempty,max=255"`
	TargetResourceName      string `json:"targetResourceName" binding:"omitempty,max=512"`
	IncludeGovernanceFields bool   `json:"includeGovernanceFields"`
}

type ConfigClonePlanVO struct {
	SourceInstanceID   uint                `json:"sourceInstanceId"`
	SourceInstanceName string              `json:"sourceInstanceName"`
	SourceMQType       string              `json:"sourceMqType"`
	TargetInstanceID   uint                `json:"targetInstanceId"`
	TargetInstanceName string              `json:"targetInstanceName"`
	TargetMQType       string              `json:"targetMqType"`
	ResourceType       string              `json:"resourceType"`
	Namespace          string              `json:"namespace"`
	ResourceName       string              `json:"resourceName"`
	TargetNamespace    string              `json:"targetNamespace"`
	TargetResourceName string              `json:"targetResourceName"`
	Executable         bool                `json:"executable"`
	RequiresApproval   bool                `json:"requiresApproval"`
	RiskLevel          string              `json:"riskLevel"`
	Action             string              `json:"action"`
	SourceSnapshot     map[string]any      `json:"sourceSnapshot"`
	TargetSnapshot     map[string]any      `json:"targetSnapshot,omitempty"`
	ProposedConfig     map[string]any      `json:"proposedConfig"`
	Diff               []OperationDiffItem `json:"diff"`
	Warnings           []string            `json:"warnings"`
	Suggestions        []string            `json:"suggestions"`
	Message            string              `json:"message"`
	GeneratedAt        string              `json:"generatedAt"`
}

type CapacityForecastRequest struct {
	HorizonHours int `form:"horizonHours"`
}

type CapacityRecommendationVO struct {
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Title      string `json:"title"`
	Suggestion string `json:"suggestion"`
	Metric     string `json:"metric,omitempty"`
}

type CapacityForecastVO struct {
	InstanceID           uint                        `json:"instanceId"`
	InstanceName         string                      `json:"instanceName"`
	MQType               string                      `json:"mqType"`
	HorizonHours         int                         `json:"horizonHours"`
	SampleCount          int                         `json:"sampleCount"`
	CurrentBacklog       int64                       `json:"currentBacklog"`
	CurrentLag           int64                       `json:"currentLag"`
	BacklogGrowthPerHour float64                     `json:"backlogGrowthPerHour"`
	LagGrowthPerHour     float64                     `json:"lagGrowthPerHour"`
	ProjectedBacklog     int64                       `json:"projectedBacklog"`
	ProjectedLag         int64                       `json:"projectedLag"`
	RiskLevel            string                      `json:"riskLevel"`
	TrendMessage         string                      `json:"trendMessage"`
	Recommendations      []*CapacityRecommendationVO `json:"recommendations"`
	TopBacklogResources  []*DashboardResourceVO      `json:"topBacklogResources"`
	TopLagConsumerGroups []*DashboardConsumerGroupVO `json:"topLagConsumerGroups"`
	GeneratedAt          string                      `json:"generatedAt"`
}

type AuditChainVerifyRequest struct {
	AuditType         string `form:"auditType"`
	InstanceID        uint   `form:"instanceId"`
	Limit             int    `form:"limit"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type AuditChainFindingVO struct {
	AuditID  uint   `json:"auditId"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type AuditChainVerifyVO struct {
	AuditType   string                 `json:"auditType"`
	Checked     int                    `json:"checked"`
	Valid       bool                   `json:"valid"`
	BrokenCount int                    `json:"brokenCount"`
	HeadHash    string                 `json:"headHash,omitempty"`
	TailHash    string                 `json:"tailHash,omitempty"`
	Findings    []*AuditChainFindingVO `json:"findings"`
	GeneratedAt string                 `json:"generatedAt"`
}

type HighRiskOperationConfig struct {
	Enabled                    bool
	ReasonRequired             bool
	OperationMaxMetadataAge    time.Duration
	OperationMaxMetricAge      time.Duration
	RequireFreshMetricHighRisk bool
}

type OperationDiffItem struct {
	Key    string `json:"key"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Risk   string `json:"risk"`
}

type ResourceOperationValidationVO struct {
	InstanceID          uint                `json:"instanceId"`
	MQType              string              `json:"mqType"`
	Action              string              `json:"action"`
	ActionText          string              `json:"actionText"`
	RiskLevel           string              `json:"riskLevel"`
	RequiredPermission  uint                `json:"requiredPermission"`
	ResourceType        string              `json:"resourceType"`
	Namespace           string              `json:"namespace"`
	ResourceName        string              `json:"resourceName"`
	Supported           bool                `json:"supported"`
	Warnings            []string            `json:"warnings"`
	Impacts             []string            `json:"impacts"`
	Before              map[string]any      `json:"before,omitempty"`
	After               map[string]any      `json:"after,omitempty"`
	Diff                []OperationDiffItem `json:"diff,omitempty"`
	NormalizedParams    map[string]any      `json:"normalizedParams"`
	LockKey             string              `json:"lockKey,omitempty"`
	MetadataStale       bool                `json:"metadataStale"`
	MetadataStaleReason string              `json:"metadataStaleReason,omitempty"`
	MetricStale         bool                `json:"metricStale"`
	MetricStaleReason   string              `json:"metricStaleReason,omitempty"`
	Message             string              `json:"message"`
	RequiresConfirm     bool                `json:"requiresConfirm"`
	RequiresHighRiskAck bool                `json:"requiresHighRiskAck"`
}

type MQCapabilityItemVO struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	DisabledReason string `json:"disabledReason,omitempty"`
}

type MQCapabilitiesVO struct {
	InstanceID   uint                   `json:"instanceId"`
	InstanceName string                 `json:"instanceName"`
	MQType       string                 `json:"mqType"`
	MQTypeText   string                 `json:"mqTypeText"`
	Capabilities []*MQCapabilityItemVO  `json:"capabilities"`
	Actions      []*MQOperationActionVO `json:"actions"`
}

type MQOperationActionVO struct {
	Action              string         `json:"action"`
	ActionText          string         `json:"actionText"`
	ResourceType        string         `json:"resourceType"`
	RiskLevel           string         `json:"riskLevel"`
	RequiredPermission  uint           `json:"requiredPermission"`
	RequiresConfirm     bool           `json:"requiresConfirm"`
	RequiresHighRiskAck bool           `json:"requiresHighRiskAck"`
	Enabled             bool           `json:"enabled"`
	DisabledReason      string         `json:"disabledReason,omitempty"`
	DefaultParams       map[string]any `json:"defaultParams"`
	FormSchema          map[string]any `json:"formSchema"`
}

type ResourceOperationApplyResult struct {
	ResourceType string         `json:"resourceType"`
	Namespace    string         `json:"namespace"`
	ResourceName string         `json:"resourceName"`
	Message      string         `json:"message"`
	Result       map[string]any `json:"result"`
}

type ResourceOperationResultVO struct {
	AuditID      uint           `json:"auditId"`
	InstanceID   uint           `json:"instanceId"`
	MQType       string         `json:"mqType"`
	Action       string         `json:"action"`
	ActionText   string         `json:"actionText"`
	RiskLevel    string         `json:"riskLevel"`
	ResourceType string         `json:"resourceType"`
	Namespace    string         `json:"namespace"`
	ResourceName string         `json:"resourceName"`
	Status       string         `json:"status"`
	Message      string         `json:"message"`
	DurationMs   int64          `json:"durationMs"`
	Result       map[string]any `json:"result"`
	ExecutedAt   string         `json:"executedAt"`
}

func NormalizeType(mqType string) string {
	return strings.ToLower(strings.TrimSpace(mqType))
}

func IsSupportedType(mqType string) bool {
	switch NormalizeType(mqType) {
	case MQTypeRabbitMQ, MQTypeKafka, MQTypeRocketMQ, MQTypeActiveMQ, MQTypePulsar:
		return true
	default:
		return false
	}
}

func TypeText(mqType string) string {
	switch NormalizeType(mqType) {
	case MQTypeRabbitMQ:
		return "RabbitMQ"
	case MQTypeKafka:
		return "Kafka"
	case MQTypeRocketMQ:
		return "RocketMQ"
	case MQTypeActiveMQ:
		return "ActiveMQ"
	case MQTypePulsar:
		return "Pulsar"
	default:
		return strings.TrimSpace(mqType)
	}
}

func DefaultPort(mqType string) int {
	switch NormalizeType(mqType) {
	case MQTypeRabbitMQ:
		return 5672
	case MQTypeKafka:
		return 9092
	case MQTypeRocketMQ:
		return 9876
	case MQTypeActiveMQ:
		return 61616
	case MQTypePulsar:
		return 6650
	default:
		return 0
	}
}

func DefaultManagementPort(mqType string) int {
	switch NormalizeType(mqType) {
	case MQTypeRabbitMQ:
		return 15672
	case MQTypeActiveMQ:
		return 8161
	case MQTypePulsar:
		return 8080
	default:
		return 0
	}
}

func StatusText(status string) string {
	if status == InstanceStatusEnabled {
		return "启用"
	}
	if status == InstanceStatusDisabled {
		return "禁用"
	}
	return status
}

func HealthText(status string) string {
	switch status {
	case HealthStatusHealthy:
		return "健康"
	case HealthStatusWarning:
		return "警告"
	case HealthStatusCritical:
		return "异常"
	default:
		return "未知"
	}
}

func ResourceTypeText(resourceType string) string {
	switch resourceType {
	case ResourceTypeBroker:
		return "Broker"
	case ResourceTypeTenant:
		return "租户"
	case ResourceTypeNamespace:
		return "命名空间"
	case ResourceTypeVHost:
		return "VHost"
	case ResourceTypeExchange:
		return "Exchange"
	case ResourceTypeBinding:
		return "Binding"
	case ResourceTypeQueue:
		return "Queue"
	case ResourceTypeTopic:
		return "Topic"
	case ResourceTypeSubscription:
		return "订阅"
	case ResourceTypeConsumer:
		return "消费者"
	default:
		return resourceType
	}
}
