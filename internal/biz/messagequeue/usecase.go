package messagequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type UseCase struct {
	instanceRepo       InstanceRepo
	permissionRepo     PermissionRepo
	brokerRepo         BrokerRepo
	resourceRepo       ResourceRepo
	bindingRepo        BindingRepo
	consumerGroupRepo  ConsumerGroupRepo
	partitionRepo      PartitionRepo
	metadataRepo       MetadataRepo
	syncJobRepo        SyncJobRepo
	operationAuditRepo OperationAuditRepo
	messageAuditRepo   MessageAuditRepo
	credentialIDExists func(ctx context.Context, id uint) error
	credentialResolver func(ctx context.Context, id uint) (*ConnectionCredential, error)
	adapters           *AdapterRegistry
}

func NewUseCase(
	instanceRepo InstanceRepo,
	permissionRepo PermissionRepo,
	brokerRepo BrokerRepo,
	resourceRepo ResourceRepo,
	bindingRepo BindingRepo,
	consumerGroupRepo ConsumerGroupRepo,
	partitionRepo PartitionRepo,
	metadataRepo MetadataRepo,
	syncJobRepo SyncJobRepo,
	operationAuditRepo OperationAuditRepo,
	messageAuditRepo MessageAuditRepo,
	credentialIDExists func(ctx context.Context, id uint) error,
	credentialResolver func(ctx context.Context, id uint) (*ConnectionCredential, error),
	adapters *AdapterRegistry,
) *UseCase {
	if adapters == nil {
		adapters = NewDefaultAdapterRegistry()
	}
	return &UseCase{
		instanceRepo:       instanceRepo,
		permissionRepo:     permissionRepo,
		brokerRepo:         brokerRepo,
		resourceRepo:       resourceRepo,
		bindingRepo:        bindingRepo,
		consumerGroupRepo:  consumerGroupRepo,
		partitionRepo:      partitionRepo,
		metadataRepo:       metadataRepo,
		syncJobRepo:        syncJobRepo,
		operationAuditRepo: operationAuditRepo,
		messageAuditRepo:   messageAuditRepo,
		credentialIDExists: credentialIDExists,
		credentialResolver: credentialResolver,
		adapters:           adapters,
	}
}

func (uc *UseCase) SupportedTypes() []*SupportedTypeVO {
	return []*SupportedTypeVO{
		{Type: MQTypeRabbitMQ, Name: "RabbitMQ", DefaultPort: 5672, DefaultManagement: 15672, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, Phase: "phase1"},
		{Type: MQTypeKafka, Name: "Kafka", DefaultPort: 9092, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, MessageSample: true, Phase: "phase1"},
		{Type: MQTypeRocketMQ, Name: "RocketMQ", DefaultPort: 9876, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: false, Phase: "phase1-basic"},
		{Type: MQTypeActiveMQ, Name: "ActiveMQ", DefaultPort: 61616, DefaultManagement: 8161, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, Phase: "phase1-basic"},
		{Type: MQTypePulsar, Name: "Pulsar", DefaultPort: 6650, DefaultManagement: 8080, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, Phase: "phase1"},
	}
}

func (uc *UseCase) CreateInstance(ctx context.Context, req *InstanceRequest) (*InstanceVO, error) {
	if err := uc.validateInstanceRequest(ctx, req); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = InstanceStatusEnabled
	}
	mqType := NormalizeType(req.MQType)
	port := req.Port
	if port <= 0 {
		port = DefaultPort(mqType)
	}
	item := &MQInstance{
		Name:             strings.TrimSpace(req.Name),
		MQType:           mqType,
		Engine:           mqType,
		Endpoint:         strings.TrimSpace(req.Endpoint),
		ManagementURL:    strings.TrimSpace(req.ManagementURL),
		Port:             port,
		CredentialID:     req.CredentialID,
		TLSEnabled:       req.TLSEnabled,
		ConnectionParams: strings.TrimSpace(req.ConnectionParams),
		Status:           status,
		HealthStatus:     HealthStatusUnknown,
		Environment:      strings.TrimSpace(req.Environment),
		BusinessSystem:   strings.TrimSpace(req.BusinessSystem),
		Owner:            strings.TrimSpace(req.Owner),
		Tags:             strings.TrimSpace(req.Tags),
		Remark:           strings.TrimSpace(req.Remark),
	}
	if err := uc.instanceRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toInstanceVO(item), nil
}

func (uc *UseCase) UpdateInstance(ctx context.Context, req *InstanceRequest) error {
	if req == nil || req.ID == 0 {
		return fmt.Errorf("MQ实例ID不能为空")
	}
	if err := uc.validateInstanceRequest(ctx, req); err != nil {
		return err
	}
	item, err := uc.instanceRepo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("MQ实例不存在")
	}
	mqType := NormalizeType(req.MQType)
	item.Name = strings.TrimSpace(req.Name)
	item.MQType = mqType
	if strings.TrimSpace(item.Engine) == "" {
		item.Engine = mqType
	}
	item.Endpoint = strings.TrimSpace(req.Endpoint)
	item.ManagementURL = strings.TrimSpace(req.ManagementURL)
	item.Port = req.Port
	if item.Port <= 0 {
		item.Port = DefaultPort(mqType)
	}
	item.CredentialID = req.CredentialID
	item.TLSEnabled = req.TLSEnabled
	item.ConnectionParams = strings.TrimSpace(req.ConnectionParams)
	if status := strings.TrimSpace(req.Status); status != "" {
		item.Status = status
	}
	item.Environment = strings.TrimSpace(req.Environment)
	item.BusinessSystem = strings.TrimSpace(req.BusinessSystem)
	item.Owner = strings.TrimSpace(req.Owner)
	item.Tags = strings.TrimSpace(req.Tags)
	item.Remark = strings.TrimSpace(req.Remark)
	return uc.instanceRepo.Update(ctx, item)
}

func (uc *UseCase) DeleteInstance(ctx context.Context, id uint) error {
	if id == 0 {
		return fmt.Errorf("MQ实例ID不能为空")
	}
	return uc.instanceRepo.Delete(ctx, id)
}

func (uc *UseCase) GetInstance(ctx context.Context, id uint) (*InstanceVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	return uc.toInstanceVO(item), nil
}

func (uc *UseCase) ListInstances(ctx context.Context, req *InstanceListRequest) ([]*InstanceVO, int64, error) {
	normalizeInstanceListRequest(req)
	items, total, err := uc.instanceRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*InstanceVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toInstanceVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) SetInstanceStatus(ctx context.Context, id uint, status string) error {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("MQ实例不存在")
	}
	status = strings.TrimSpace(status)
	if status != InstanceStatusEnabled && status != InstanceStatusDisabled {
		return fmt.Errorf("不支持的实例状态")
	}
	item.Status = status
	return uc.instanceRepo.Update(ctx, item)
}

func (uc *UseCase) TestInstance(ctx context.Context, id uint, operator Operator) (*ConnectionTestResultVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	if strings.TrimSpace(item.Status) != InstanceStatusEnabled {
		return nil, fmt.Errorf("MQ实例已禁用")
	}
	adapter, credential, err := uc.resolveAdapterAndCredential(ctx, item)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	audit := uc.startOperationAudit(ctx, item, AuditActionConnectionTest, RiskLevelLow, operator, nil)
	result, err := adapter.TestConnection(ctx, item, credential)
	finishedAt := time.Now()
	if err != nil {
		item.HealthStatus = HealthStatusCritical
		_ = uc.instanceRepo.Update(ctx, item)
		uc.finishOperationAudit(ctx, audit, AuditStatusFailed, err.Error(), nil, startedAt, finishedAt)
		return nil, err
	}

	item.Version = strings.TrimSpace(result.Version)
	if strings.TrimSpace(result.Engine) != "" {
		item.Engine = strings.TrimSpace(result.Engine)
	}
	item.HealthStatus = HealthStatusHealthy
	item.LastTestAt = &finishedAt
	if err := uc.instanceRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	uc.finishOperationAudit(ctx, audit, AuditStatusSuccess, result.Message, result, startedAt, finishedAt)

	return &ConnectionTestResultVO{
		InstanceID:      item.ID,
		Name:            item.Name,
		MQType:          item.MQType,
		Version:         item.Version,
		ClusterName:     result.ClusterName,
		BrokerCount:     result.BrokerCount,
		ManagementReady: result.ManagementReady,
		LatencyMs:       finishedAt.Sub(startedAt).Milliseconds(),
		Message:         result.Message,
		TestedAt:        finishedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) SyncMetadata(ctx context.Context, id uint, operator Operator) (*MetadataSyncResultVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	if strings.TrimSpace(item.Status) != InstanceStatusEnabled {
		return nil, fmt.Errorf("MQ实例已禁用")
	}
	if uc.metadataRepo == nil || uc.syncJobRepo == nil {
		return nil, fmt.Errorf("MQ元数据仓库未配置")
	}
	adapter, credential, err := uc.resolveAdapterAndCredential(ctx, item)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	job := &MQSyncJob{
		InstanceID:   item.ID,
		TriggerType:  TriggerManual,
		Status:       SyncStatusRunning,
		StartedAt:    &startedAt,
		OperatorID:   operator.ID,
		OperatorName: operator.Username,
	}
	if err := uc.syncJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	audit := uc.startOperationAudit(ctx, item, AuditActionMetadataSync, RiskLevelLow, operator, nil)

	snapshot, err := adapter.DiscoverMetadata(ctx, item, credential)
	finishedAt := time.Now()
	if err != nil {
		uc.finishSyncJob(ctx, job, SyncStatusFailed, err.Error(), startedAt, finishedAt, nil)
		uc.finishOperationAudit(ctx, audit, AuditStatusFailed, err.Error(), nil, startedAt, finishedAt)
		item.HealthStatus = HealthStatusCritical
		_ = uc.instanceRepo.Update(ctx, item)
		return nil, err
	}
	if snapshot.SyncedAt.IsZero() {
		snapshot.SyncedAt = finishedAt
	}
	if snapshot.HealthStatus == "" {
		snapshot.HealthStatus = HealthStatusHealthy
	}
	if err := uc.metadataRepo.ReplaceAll(ctx, item.ID, snapshot); err != nil {
		uc.finishSyncJob(ctx, job, SyncStatusFailed, err.Error(), startedAt, finishedAt, snapshot)
		uc.finishOperationAudit(ctx, audit, AuditStatusFailed, err.Error(), nil, startedAt, finishedAt)
		return nil, err
	}

	item.Version = strings.TrimSpace(snapshot.Version)
	if strings.TrimSpace(snapshot.Engine) != "" {
		item.Engine = strings.TrimSpace(snapshot.Engine)
	}
	item.HealthStatus = snapshot.HealthStatus
	item.LastSyncAt = &finishedAt
	if err := uc.instanceRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	uc.finishSyncJob(ctx, job, SyncStatusSuccess, snapshot.Message, startedAt, finishedAt, snapshot)
	uc.finishOperationAudit(ctx, audit, AuditStatusSuccess, snapshot.Message, snapshotSummary(snapshot), startedAt, finishedAt)

	return &MetadataSyncResultVO{
		JobID:              job.ID,
		InstanceID:         item.ID,
		Name:               item.Name,
		Status:             SyncStatusSuccess,
		Message:            snapshot.Message,
		Version:            item.Version,
		DurationMs:         finishedAt.Sub(startedAt).Milliseconds(),
		BrokerCount:        len(snapshot.Brokers),
		ResourceCount:      len(snapshot.Resources),
		ConsumerGroupCount: len(snapshot.ConsumerGroups),
		PartitionCount:     len(snapshot.Partitions),
		SyncedAt:           finishedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) ListBrokers(ctx context.Context, instanceID uint) ([]*BrokerVO, error) {
	items, err := uc.brokerRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	list := make([]*BrokerVO, 0, len(items))
	for _, item := range items {
		list = append(list, toBrokerVO(item))
	}
	return list, nil
}

func (uc *UseCase) ListResources(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*ResourceVO, int64, error) {
	normalizeResourceListRequest(req)
	items, total, err := uc.resourceRepo.List(ctx, instanceID, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*ResourceVO, 0, len(items))
	for _, item := range items {
		list = append(list, toResourceVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) ListBindings(ctx context.Context, instanceID uint) ([]*BindingVO, error) {
	items, err := uc.bindingRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	list := make([]*BindingVO, 0, len(items))
	for _, item := range items {
		list = append(list, toBindingVO(item))
	}
	return list, nil
}

func (uc *UseCase) ListConsumerGroups(ctx context.Context, instanceID uint, req *ConsumerGroupListRequest) ([]*ConsumerGroupVO, int64, error) {
	normalizeConsumerGroupListRequest(req)
	items, total, err := uc.consumerGroupRepo.List(ctx, instanceID, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*ConsumerGroupVO, 0, len(items))
	for _, item := range items {
		list = append(list, toConsumerGroupVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) ListPartitions(ctx context.Context, instanceID uint, req *PartitionListRequest) ([]*PartitionVO, int64, error) {
	normalizePartitionListRequest(req)
	items, total, err := uc.partitionRepo.List(ctx, instanceID, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*PartitionVO, 0, len(items))
	for _, item := range items {
		list = append(list, toPartitionVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) GetOverview(ctx context.Context, instanceID uint) (*OverviewVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	brokerCount, onlineBrokerCount, err := uc.brokerRepo.CountByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	resourceSummary, err := uc.resourceRepo.Summary(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	groupSummary, err := uc.consumerGroupRepo.Summary(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	partitionCount, err := uc.partitionRepo.CountByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	top, err := uc.resourceRepo.TopBacklog(ctx, instanceID, 10)
	if err != nil {
		return nil, err
	}
	topVO := make([]*ResourceVO, 0, len(top))
	for _, item := range top {
		topVO = append(topVO, toResourceVO(item))
	}
	return &OverviewVO{
		InstanceID:          instance.ID,
		InstanceName:        instance.Name,
		MQType:              instance.MQType,
		HealthStatus:        instance.HealthStatus,
		BrokerCount:         brokerCount,
		OnlineBrokerCount:   onlineBrokerCount,
		ResourceCount:       resourceSummary.Count,
		ConsumerGroupCount:  groupSummary.Count,
		PartitionCount:      partitionCount,
		MessageCount:        resourceSummary.MessageCount,
		Backlog:             resourceSummary.Backlog + groupSummary.Backlog,
		Lag:                 groupSummary.Lag,
		ProducedRate:        resourceSummary.ProducedRate,
		ConsumedRate:        resourceSummary.ConsumedRate,
		TopBacklogResources: topVO,
		LastSyncAt:          formatTimePtr(instance.LastSyncAt),
	}, nil
}

func (uc *UseCase) SampleMessages(ctx context.Context, instanceID uint, req *MessageSampleRequest, operator Operator) (*MessageSampleResultVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	adapter, credential, err := uc.resolveAdapterAndCredential(ctx, instance)
	if err != nil {
		return nil, err
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.MaxBytes <= 0 {
		req.MaxBytes = 64 * 1024
	}
	startedAt := time.Now()
	result, err := adapter.SampleMessages(ctx, instance, credential, req)
	status := AuditStatusSuccess
	message := "消息采样成功"
	sampleCount := 0
	payloadBytes := 0
	if err != nil {
		status = AuditStatusFailed
		message = err.Error()
	} else if result != nil {
		sampleCount = result.SampleCount
		for _, item := range result.Samples {
			payloadBytes += item.PayloadSize
		}
	}
	uc.createMessageAudit(ctx, instance, req, status, sampleCount, payloadBytes, message, operator)
	if err != nil {
		return nil, err
	}
	if result.SampledAt == "" {
		result.SampledAt = startedAt.Format("2006-01-02 15:04:05")
	}
	return result, nil
}

func (uc *UseCase) ListOperationAudits(ctx context.Context, req *AuditListRequest) ([]*AuditVO, int64, error) {
	normalizeAuditListRequest(req)
	items, total, err := uc.operationAuditRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*AuditVO, 0, len(items))
	for _, item := range items {
		list = append(list, operationAuditToVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) ListMessageAudits(ctx context.Context, req *AuditListRequest) ([]*AuditVO, int64, error) {
	normalizeAuditListRequest(req)
	items, total, err := uc.messageAuditRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*AuditVO, 0, len(items))
	for _, item := range items {
		list = append(list, messageAuditToVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) AuditInstancePermission(ctx context.Context, action string, req any, operator Operator) {
	if uc.operationAuditRepo == nil {
		return
	}
	payload := mustJSON(req)
	now := time.Now()
	_ = uc.operationAuditRepo.Create(ctx, &MQOperationAudit{
		Action:       action,
		RiskLevel:    RiskLevelLow,
		Status:       AuditStatusSuccess,
		RequestJSON:  payload,
		OperatorID:   operator.ID,
		OperatorName: operator.Username,
		ClientIP:     operator.ClientIP,
		StartedAt:    &now,
		FinishedAt:   &now,
		Message:      "实例权限已更新",
	})
}

func (uc *UseCase) validateInstanceRequest(ctx context.Context, req *InstanceRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("实例名称不能为空")
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		return fmt.Errorf("连接地址不能为空")
	}
	mqType := NormalizeType(req.MQType)
	if !IsSupportedType(mqType) {
		return fmt.Errorf("不支持的消息队列类型: %s", req.MQType)
	}
	if req.CredentialID > 0 && uc.credentialIDExists != nil {
		if err := uc.credentialIDExists(ctx, req.CredentialID); err != nil {
			return fmt.Errorf("凭据不存在")
		}
	}
	if req.Port < 0 || req.Port > 65535 {
		return fmt.Errorf("端口范围必须为 1-65535")
	}
	if status := strings.TrimSpace(req.Status); status != "" && status != InstanceStatusEnabled && status != InstanceStatusDisabled {
		return fmt.Errorf("不支持的实例状态")
	}
	return nil
}

func (uc *UseCase) resolveAdapterAndCredential(ctx context.Context, item *MQInstance) (Adapter, *ConnectionCredential, error) {
	adapter, ok := uc.adapters.Get(item.MQType)
	if !ok {
		return nil, nil, fmt.Errorf("%s 适配器未注册", TypeText(item.MQType))
	}
	var credential *ConnectionCredential
	if item.CredentialID > 0 {
		if uc.credentialResolver == nil {
			return nil, nil, fmt.Errorf("连接凭据解析器未配置")
		}
		resolved, err := uc.credentialResolver(ctx, item.CredentialID)
		if err != nil {
			return nil, nil, fmt.Errorf("凭据不存在")
		}
		credential = resolved
	}
	return adapter, credential, nil
}

func (uc *UseCase) toInstanceVO(item *MQInstance) *InstanceVO {
	if item == nil {
		return nil
	}
	return &InstanceVO{
		ID:               item.ID,
		Name:             item.Name,
		MQType:           item.MQType,
		MQTypeText:       TypeText(item.MQType),
		Engine:           item.Engine,
		Version:          item.Version,
		Endpoint:         item.Endpoint,
		ManagementURL:    item.ManagementURL,
		Port:             item.Port,
		CredentialID:     item.CredentialID,
		TLSEnabled:       item.TLSEnabled,
		ConnectionParams: item.ConnectionParams,
		Status:           item.Status,
		StatusText:       StatusText(item.Status),
		HealthStatus:     item.HealthStatus,
		HealthText:       HealthText(item.HealthStatus),
		Environment:      item.Environment,
		BusinessSystem:   item.BusinessSystem,
		Owner:            item.Owner,
		Tags:             item.Tags,
		Remark:           item.Remark,
		LastTestAt:       formatTimePtr(item.LastTestAt),
		LastSyncAt:       formatTimePtr(item.LastSyncAt),
		LastMetricAt:     formatTimePtr(item.LastMetricAt),
		CreatedAt:        item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) startOperationAudit(ctx context.Context, item *MQInstance, action, riskLevel string, operator Operator, request any) *MQOperationAudit {
	if uc.operationAuditRepo == nil || item == nil {
		return nil
	}
	now := time.Now()
	audit := &MQOperationAudit{
		InstanceID:   item.ID,
		InstanceName: item.Name,
		MQType:       item.MQType,
		Action:       action,
		RiskLevel:    riskLevel,
		Status:       AuditStatusPending,
		RequestJSON:  mustJSON(request),
		OperatorID:   operator.ID,
		OperatorName: operator.Username,
		ClientIP:     operator.ClientIP,
		StartedAt:    &now,
	}
	if err := uc.operationAuditRepo.Create(ctx, audit); err != nil {
		return nil
	}
	return audit
}

func (uc *UseCase) finishOperationAudit(ctx context.Context, audit *MQOperationAudit, status, message string, result any, startedAt, finishedAt time.Time) {
	if uc.operationAuditRepo == nil || audit == nil {
		return
	}
	audit.Status = status
	audit.Message = trimText(message, 500)
	audit.ResultJSON = mustJSON(result)
	audit.StartedAt = &startedAt
	audit.FinishedAt = &finishedAt
	audit.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	_ = uc.operationAuditRepo.Update(ctx, audit)
}

func (uc *UseCase) finishSyncJob(ctx context.Context, job *MQSyncJob, status, message string, startedAt, finishedAt time.Time, snapshot *MQMetadataSnapshot) {
	if uc.syncJobRepo == nil || job == nil {
		return
	}
	job.Status = status
	job.Message = trimText(message, 500)
	job.StartedAt = &startedAt
	job.FinishedAt = &finishedAt
	job.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	if snapshot != nil {
		job.BrokerCount = len(snapshot.Brokers)
		job.ResourceCount = len(snapshot.Resources)
		job.ConsumerGroupCount = len(snapshot.ConsumerGroups)
		job.PartitionCount = len(snapshot.Partitions)
	}
	_ = uc.syncJobRepo.Update(ctx, job)
}

func (uc *UseCase) createMessageAudit(ctx context.Context, item *MQInstance, req *MessageSampleRequest, status string, sampleCount, payloadBytes int, message string, operator Operator) {
	if uc.messageAuditRepo == nil || item == nil || req == nil {
		return
	}
	_ = uc.messageAuditRepo.Create(ctx, &MQMessageAudit{
		InstanceID:   item.ID,
		MQType:       item.MQType,
		ResourceType: req.ResourceType,
		ResourceName: req.ResourceName,
		Namespace:    req.Namespace,
		Action:       AuditActionMessageSample,
		SampleCount:  sampleCount,
		PayloadBytes: payloadBytes,
		FilterJSON:   mustJSON(req),
		Status:       status,
		OperatorID:   operator.ID,
		OperatorName: operator.Username,
		ClientIP:     operator.ClientIP,
		Message:      trimText(message, 500),
	})
}

func normalizeInstanceListRequest(req *InstanceListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	req.MQType = NormalizeType(req.MQType)
	req.Status = strings.TrimSpace(req.Status)
	req.HealthStatus = strings.TrimSpace(req.HealthStatus)
	req.Environment = strings.TrimSpace(req.Environment)
	req.Keyword = strings.TrimSpace(req.Keyword)
}

func normalizeResourceListRequest(req *ResourceListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.Namespace = strings.TrimSpace(req.Namespace)
}

func normalizeConsumerGroupListRequest(req *ConsumerGroupListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.Namespace = strings.TrimSpace(req.Namespace)
}

func normalizePartitionListRequest(req *PartitionListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.ResourceName = strings.TrimSpace(req.ResourceName)
}

func normalizeAuditListRequest(req *AuditListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.MQType = NormalizeType(req.MQType)
	req.Action = strings.TrimSpace(req.Action)
	req.Status = strings.TrimSpace(req.Status)
	req.RiskLevel = strings.TrimSpace(req.RiskLevel)
}

func toBrokerVO(item *MQBroker) *BrokerVO {
	return &BrokerVO{
		ID:         item.ID,
		InstanceID: item.InstanceID,
		BrokerName: item.BrokerName,
		BrokerID:   item.BrokerID,
		Host:       item.Host,
		Port:       item.Port,
		Role:       item.Role,
		Status:     item.Status,
		Version:    item.Version,
		Rack:       item.Rack,
		Zone:       item.Zone,
		LastSyncAt: formatTimePtr(item.LastSyncAt),
	}
}

func toResourceVO(item *MQResource) *ResourceVO {
	return &ResourceVO{
		ID:               item.ID,
		InstanceID:       item.InstanceID,
		ResourceType:     item.ResourceType,
		ResourceTypeText: ResourceTypeText(item.ResourceType),
		Namespace:        item.Namespace,
		Name:             item.Name,
		FullName:         item.FullName,
		Durable:          item.Durable,
		PartitionCount:   item.PartitionCount,
		ReplicaCount:     item.ReplicaCount,
		MessageCount:     item.MessageCount,
		Backlog:          item.Backlog,
		ProducedRate:     item.ProducedRate,
		ConsumedRate:     item.ConsumedRate,
		ConsumerCount:    item.ConsumerCount,
		LastSyncAt:       formatTimePtr(item.LastSyncAt),
	}
}

func toBindingVO(item *MQBinding) *BindingVO {
	return &BindingVO{
		ID:              item.ID,
		InstanceID:      item.InstanceID,
		VHost:           item.VHost,
		Source:          item.Source,
		Destination:     item.Destination,
		DestinationType: item.DestinationType,
		RoutingKey:      item.RoutingKey,
		LastSyncAt:      formatTimePtr(item.LastSyncAt),
	}
}

func toConsumerGroupVO(item *MQConsumerGroup) *ConsumerGroupVO {
	return &ConsumerGroupVO{
		ID:                  item.ID,
		InstanceID:          item.InstanceID,
		ResourceID:          item.ResourceID,
		GroupName:           item.GroupName,
		ResourceName:        item.ResourceName,
		Namespace:           item.Namespace,
		State:               item.State,
		ConsumerCount:       item.ConsumerCount,
		ActiveConsumerCount: item.ActiveConsumerCount,
		CurrentOffset:       item.CurrentOffset,
		EndOffset:           item.EndOffset,
		Lag:                 item.Lag,
		Backlog:             item.Backlog,
		LastConsumedAt:      formatTimePtr(item.LastConsumedAt),
		LastSyncAt:          formatTimePtr(item.LastSyncAt),
	}
}

func toPartitionVO(item *MQPartition) *PartitionVO {
	return &PartitionVO{
		ID:            item.ID,
		InstanceID:    item.InstanceID,
		ResourceID:    item.ResourceID,
		ResourceName:  item.ResourceName,
		PartitionID:   item.PartitionID,
		Leader:        item.Leader,
		StartOffset:   item.StartOffset,
		EndOffset:     item.EndOffset,
		CurrentOffset: item.CurrentOffset,
		Lag:           item.Lag,
		Status:        item.Status,
		LastSyncAt:    formatTimePtr(item.LastSyncAt),
	}
}

func operationAuditToVO(item *MQOperationAudit) *AuditVO {
	return &AuditVO{
		ID:           item.ID,
		InstanceID:   item.InstanceID,
		InstanceName: item.InstanceName,
		MQType:       item.MQType,
		ResourceType: item.ResourceType,
		ResourceName: item.ResourceName,
		Namespace:    item.Namespace,
		Action:       item.Action,
		ActionText:   actionText(item.Action),
		RiskLevel:    item.RiskLevel,
		Status:       item.Status,
		StatusText:   auditStatusText(item.Status),
		Reason:       item.Reason,
		OperatorID:   item.OperatorID,
		OperatorName: item.OperatorName,
		ClientIP:     item.ClientIP,
		DurationMs:   item.DurationMs,
		Message:      item.Message,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func messageAuditToVO(item *MQMessageAudit) *AuditVO {
	return &AuditVO{
		ID:           item.ID,
		InstanceID:   item.InstanceID,
		MQType:       item.MQType,
		ResourceType: item.ResourceType,
		ResourceName: item.ResourceName,
		Namespace:    item.Namespace,
		Action:       item.Action,
		ActionText:   actionText(item.Action),
		Status:       item.Status,
		StatusText:   auditStatusText(item.Status),
		OperatorID:   item.OperatorID,
		OperatorName: item.OperatorName,
		ClientIP:     item.ClientIP,
		Message:      item.Message,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func snapshotSummary(snapshot *MQMetadataSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	return map[string]any{
		"brokerCount":        len(snapshot.Brokers),
		"resourceCount":      len(snapshot.Resources),
		"bindingCount":       len(snapshot.Bindings),
		"consumerGroupCount": len(snapshot.ConsumerGroups),
		"partitionCount":     len(snapshot.Partitions),
		"message":            snapshot.Message,
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func actionText(action string) string {
	switch action {
	case AuditActionConnectionTest:
		return "连接测试"
	case AuditActionMetadataSync:
		return "元数据同步"
	case AuditActionMessageSample:
		return "消息采样"
	case AuditActionPermissionSet:
		return "实例权限保存"
	case AuditActionPermissionDel:
		return "实例权限删除"
	default:
		return action
	}
}

func auditStatusText(status string) string {
	switch status {
	case AuditStatusPending:
		return "执行中"
	case AuditStatusSuccess:
		return "成功"
	case AuditStatusFailed:
		return "失败"
	default:
		return status
	}
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		return value[:max]
	}
	return value
}

func mapToJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
