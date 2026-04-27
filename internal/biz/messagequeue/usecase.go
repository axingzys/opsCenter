package messagequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

var resourceOperationLocks sync.Map

type UseCase struct {
	instanceRepo           InstanceRepo
	permissionRepo         PermissionRepo
	brokerRepo             BrokerRepo
	resourceRepo           ResourceRepo
	bindingRepo            BindingRepo
	consumerGroupRepo      ConsumerGroupRepo
	partitionRepo          PartitionRepo
	metadataRepo           MetadataRepo
	syncJobRepo            SyncJobRepo
	operationAuditRepo     OperationAuditRepo
	messageAuditRepo       MessageAuditRepo
	credentialIDExists     func(ctx context.Context, id uint) error
	credentialResolver     func(ctx context.Context, id uint) (*ConnectionCredential, error)
	highRiskConfigResolver func(ctx context.Context) (*HighRiskOperationConfig, error)
	adapters               *AdapterRegistry
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
	highRiskConfigResolver func(ctx context.Context) (*HighRiskOperationConfig, error),
	adapters *AdapterRegistry,
) *UseCase {
	if adapters == nil {
		adapters = NewDefaultAdapterRegistry()
	}
	return &UseCase{
		instanceRepo:           instanceRepo,
		permissionRepo:         permissionRepo,
		brokerRepo:             brokerRepo,
		resourceRepo:           resourceRepo,
		bindingRepo:            bindingRepo,
		consumerGroupRepo:      consumerGroupRepo,
		partitionRepo:          partitionRepo,
		metadataRepo:           metadataRepo,
		syncJobRepo:            syncJobRepo,
		operationAuditRepo:     operationAuditRepo,
		messageAuditRepo:       messageAuditRepo,
		credentialIDExists:     credentialIDExists,
		credentialResolver:     credentialResolver,
		highRiskConfigResolver: highRiskConfigResolver,
		adapters:               adapters,
	}
}

func (uc *UseCase) SupportedTypes() []*SupportedTypeVO {
	return []*SupportedTypeVO{
		{Type: MQTypeRabbitMQ, Name: "RabbitMQ", DefaultPort: 5672, DefaultManagement: 15672, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, ResourceManage: true, Phase: "phase2"},
		{Type: MQTypeKafka, Name: "Kafka", DefaultPort: 9092, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, MessageSample: true, ResourceManage: true, Phase: "phase2"},
		{Type: MQTypeRocketMQ, Name: "RocketMQ", DefaultPort: 9876, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: false, Phase: "phase1-basic"},
		{Type: MQTypeActiveMQ, Name: "ActiveMQ", DefaultPort: 61616, DefaultManagement: 8161, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, Phase: "phase1-basic"},
		{Type: MQTypePulsar, Name: "Pulsar", DefaultPort: 6650, DefaultManagement: 8080, TestEnabled: true, MetadataEnabled: true, DiagnosisEnabled: true, ResourceManage: true, Phase: "phase2-basic"},
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

func (uc *UseCase) ValidateResourceOperation(ctx context.Context, instanceID uint, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	instance, operationAdapter, credential, err := uc.resolveResourceOperationAdapter(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	normalizeResourceOperationRequest(req)
	if req == nil || req.Action == "" {
		return nil, fmt.Errorf("操作动作不能为空")
	}
	validation, err := operationAdapter.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		return nil, err
	}
	if validation == nil {
		return nil, fmt.Errorf("操作校验结果为空")
	}
	fillOperationValidationDefaults(validation, instance, req)
	if err := uc.enrichResourceOperationPlan(ctx, instance, validation, req); err != nil {
		return nil, err
	}
	fillOperationValidationDefaults(validation, instance, req)
	return validation, nil
}

func (uc *UseCase) ExecuteResourceOperation(ctx context.Context, instanceID uint, req *ResourceOperationRequest, operator Operator) (*ResourceOperationResultVO, error) {
	instance, operationAdapter, credential, err := uc.resolveResourceOperationAdapter(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	normalizeResourceOperationRequest(req)
	if req == nil || req.Action == "" {
		return nil, fmt.Errorf("操作动作不能为空")
	}
	if !req.Confirmed {
		return nil, fmt.Errorf("请确认操作影响后再执行")
	}
	validation, err := operationAdapter.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		return nil, err
	}
	fillOperationValidationDefaults(validation, instance, req)
	if err := uc.enrichResourceOperationPlan(ctx, instance, validation, req); err != nil {
		return nil, err
	}
	fillOperationValidationDefaults(validation, instance, req)
	if !validation.Supported {
		return nil, fmt.Errorf("%s", validation.Message)
	}
	if strings.EqualFold(instance.Environment, "prod") && validation.MetadataStale && !operationBoolParam(req.Params, false, "allowStaleMetadata") {
		return nil, fmt.Errorf("生产环境 MQ 元数据已过期，请先同步后再执行资源操作")
	}
	if IsHighRiskLevel(validation.RiskLevel) {
		cfg, err := uc.resolveHighRiskOperationConfig(ctx)
		if err != nil {
			return nil, err
		}
		if !cfg.Enabled {
			return nil, fmt.Errorf("MQ高风险操作未开启")
		}
		if cfg.ReasonRequired && strings.TrimSpace(req.Reason) == "" {
			return nil, fmt.Errorf("高风险操作原因不能为空")
		}
	}
	if result, ok, err := uc.idempotentResourceOperationResult(ctx, instance, validation, req); err != nil {
		return nil, err
	} else if ok {
		return result, nil
	}
	if validation.LockKey != "" {
		if !acquireResourceOperationLock(validation.LockKey) {
			return nil, fmt.Errorf("资源正在执行相同操作，请稍后再试")
		}
		defer releaseResourceOperationLock(validation.LockKey)
	}

	startedAt := time.Now()
	audit := uc.startResourceOperationAudit(ctx, instance, validation, req, operator)
	applyResult, err := operationAdapter.ApplyOperation(ctx, instance, credential, req)
	finishedAt := time.Now()
	if err != nil {
		uc.finishOperationAudit(ctx, audit, AuditStatusFailed, err.Error(), nil, startedAt, finishedAt)
		return nil, err
	}
	if applyResult == nil {
		applyResult = &ResourceOperationApplyResult{}
	}
	resourceType := firstNonEmpty(applyResult.ResourceType, validation.ResourceType)
	namespace := firstNonEmpty(applyResult.Namespace, validation.Namespace)
	resourceName := firstNonEmpty(applyResult.ResourceName, validation.ResourceName)
	message := firstNonEmpty(applyResult.Message, validation.Message, "资源操作执行成功")
	result := applyResult.Result
	if result == nil {
		result = map[string]any{}
	}
	status := AuditStatusSuccess
	metadataRefreshStatus := ""
	metadataRefreshError := ""
	if metadataAdapter, ok := operationAdapter.(Adapter); ok {
		if syncErr := uc.refreshMetadataAfterResourceOperation(ctx, instance, metadataAdapter, credential); syncErr != nil {
			status = AuditStatusPartial
			metadataRefreshStatus = SyncStatusFailed
			metadataRefreshError = syncErr.Error()
			result["metadataSyncStatus"] = SyncStatusFailed
			result["metadataSyncError"] = syncErr.Error()
			message = trimText(message+"；元数据同步失败: "+syncErr.Error(), 500)
		} else {
			metadataRefreshStatus = SyncStatusSuccess
			result["metadataSyncStatus"] = SyncStatusSuccess
		}
	}
	finishedAt = time.Now()
	if audit != nil {
		audit.MetadataRefreshStatus = metadataRefreshStatus
		audit.MetadataRefreshError = trimText(metadataRefreshError, 500)
		if len(validation.After) > 0 {
			audit.AfterSnapshotJSON = mustJSON(validation.After)
		}
	}
	uc.finishOperationAudit(ctx, audit, status, message, result, startedAt, finishedAt)
	auditID := uint(0)
	if audit != nil {
		auditID = audit.ID
	}
	return &ResourceOperationResultVO{
		AuditID:      auditID,
		InstanceID:   instance.ID,
		MQType:       instance.MQType,
		Action:       validation.Action,
		ActionText:   actionText(validation.Action),
		RiskLevel:    validation.RiskLevel,
		ResourceType: resourceType,
		Namespace:    namespace,
		ResourceName: resourceName,
		Status:       status,
		Message:      message,
		DurationMs:   finishedAt.Sub(startedAt).Milliseconds(),
		Result:       result,
		ExecutedAt:   finishedAt.Format("2006-01-02 15:04:05"),
	}, nil
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

func (uc *UseCase) resolveResourceOperationAdapter(ctx context.Context, instanceID uint) (*MQInstance, ResourceOperationAdapter, *ConnectionCredential, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("MQ实例不存在")
	}
	if strings.TrimSpace(instance.Status) != InstanceStatusEnabled {
		return nil, nil, nil, fmt.Errorf("MQ实例已禁用")
	}
	adapter, credential, err := uc.resolveAdapterAndCredential(ctx, instance)
	if err != nil {
		return nil, nil, nil, err
	}
	operationAdapter, ok := adapter.(ResourceOperationAdapter)
	if !ok {
		return nil, nil, nil, adapterNotSupported(instance.MQType, "资源管理")
	}
	return instance, operationAdapter, credential, nil
}

func (uc *UseCase) resolveHighRiskOperationConfig(ctx context.Context) (*HighRiskOperationConfig, error) {
	cfg := &HighRiskOperationConfig{Enabled: false, ReasonRequired: true}
	if uc.highRiskConfigResolver == nil {
		return cfg, nil
	}
	resolved, err := uc.highRiskConfigResolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取MQ高风险操作配置失败: %w", err)
	}
	if resolved == nil {
		return cfg, nil
	}
	cfg.Enabled = resolved.Enabled
	cfg.ReasonRequired = resolved.ReasonRequired
	return cfg, nil
}

func (uc *UseCase) refreshMetadataAfterResourceOperation(ctx context.Context, item *MQInstance, adapter Adapter, credential *ConnectionCredential) error {
	if uc.metadataRepo == nil || item == nil || adapter == nil {
		return nil
	}
	snapshot, err := adapter.DiscoverMetadata(ctx, item, credential)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	now := time.Now()
	if snapshot.SyncedAt.IsZero() {
		snapshot.SyncedAt = now
	}
	if snapshot.HealthStatus == "" {
		snapshot.HealthStatus = HealthStatusHealthy
	}
	if err := uc.metadataRepo.ReplaceAll(ctx, item.ID, snapshot); err != nil {
		return err
	}
	item.Version = strings.TrimSpace(snapshot.Version)
	if strings.TrimSpace(snapshot.Engine) != "" {
		item.Engine = strings.TrimSpace(snapshot.Engine)
	}
	item.HealthStatus = snapshot.HealthStatus
	item.LastSyncAt = &now
	if uc.instanceRepo != nil {
		return uc.instanceRepo.Update(ctx, item)
	}
	return nil
}

func (uc *UseCase) enrichResourceOperationPlan(ctx context.Context, instance *MQInstance, validation *ResourceOperationValidationVO, req *ResourceOperationRequest) error {
	if instance == nil || validation == nil || req == nil {
		return nil
	}
	validation.LockKey = buildResourceOperationLockKey(instance.ID, validation)
	applyMetadataFreshnessWarning(instance, validation)

	resource, err := uc.lookupOperationResource(ctx, instance.ID, validation)
	if err != nil {
		return err
	}
	if resource != nil {
		validation.Before = resourceSnapshot(resource)
	}
	validation.After = operationDesiredSnapshot(validation)
	validation.Diff = buildOperationDiff(validation.Before, validation.After)

	applyRabbitMQImmutableGuards(validation)
	applyDestructiveConfigRisk(validation)
	return nil
}

func (uc *UseCase) lookupOperationResource(ctx context.Context, instanceID uint, validation *ResourceOperationValidationVO) (*MQResource, error) {
	if uc.resourceRepo == nil || validation == nil || validation.ResourceType == "" || validation.ResourceName == "" {
		return nil, nil
	}
	return uc.resourceRepo.GetByUnique(ctx, instanceID, validation.ResourceType, validation.Namespace, validation.ResourceName)
}

func applyMetadataFreshnessWarning(instance *MQInstance, validation *ResourceOperationValidationVO) {
	if instance == nil || validation == nil {
		return
	}
	const maxAge = 30 * time.Minute
	if instance.LastSyncAt == nil || instance.LastSyncAt.IsZero() {
		validation.MetadataStale = true
		validation.MetadataStaleReason = "该实例尚未完成元数据同步，影响范围可能不准确"
		validation.Warnings = appendUniqueString(validation.Warnings, validation.MetadataStaleReason)
		return
	}
	if age := time.Since(*instance.LastSyncAt); age > maxAge {
		validation.MetadataStale = true
		validation.MetadataStaleReason = fmt.Sprintf("最近元数据同步已超过 %d 分钟，影响范围可能不准确", int(maxAge.Minutes()))
		validation.Warnings = appendUniqueString(validation.Warnings, validation.MetadataStaleReason)
	}
}

func resourceSnapshot(resource *MQResource) map[string]any {
	if resource == nil {
		return nil
	}
	snapshot := map[string]any{
		"id":             resource.ID,
		"resourceType":   resource.ResourceType,
		"namespace":      resource.Namespace,
		"name":           resource.Name,
		"fullName":       resource.FullName,
		"durable":        resource.Durable,
		"partitionCount": resource.PartitionCount,
		"replicaCount":   resource.ReplicaCount,
		"messageCount":   resource.MessageCount,
		"backlog":        resource.Backlog,
		"consumerCount":  resource.ConsumerCount,
		"config":         parseJSONMap(resource.ConfigJSON),
		"lastSyncAt":     formatTimePtr(resource.LastSyncAt),
	}
	return snapshot
}

func operationDesiredSnapshot(validation *ResourceOperationValidationVO) map[string]any {
	if validation == nil {
		return nil
	}
	params := cloneParams(validation.NormalizedParams)
	snapshot := map[string]any{
		"resourceType": validation.ResourceType,
		"namespace":    validation.Namespace,
		"name":         validation.ResourceName,
		"action":       validation.Action,
	}
	switch validation.Action {
	case OperationActionKafkaTopicCreate:
		snapshot["partitionCount"] = operationIntParam(params, 0, "partitions", "numPartitions")
		snapshot["replicaCount"] = operationIntParam(params, 0, "replicationFactor")
		snapshot["config"] = operationStringMapParam(params, "configs")
	case OperationActionKafkaPartitionsExpand:
		snapshot["partitionCount"] = operationIntParam(params, 0, "partitions", "count")
	case OperationActionKafkaTopicConfigUpdate:
		snapshot["config"] = operationStringMapParam(params, "configs")
	case OperationActionRabbitMQQueueUpsert:
		snapshot["durable"] = operationBoolParam(params, true, "durable")
		snapshot["config"] = map[string]any{
			"auto_delete": operationBoolParam(params, false, "autoDelete", "auto_delete"),
			"arguments":   operationAnyMapParam(params, "arguments"),
		}
	case OperationActionRabbitMQExchangeUpsert:
		snapshot["durable"] = operationBoolParam(params, true, "durable")
		snapshot["config"] = map[string]any{
			"type":        operationStringParam(params, "type"),
			"auto_delete": operationBoolParam(params, false, "autoDelete", "auto_delete"),
			"internal":    operationBoolParam(params, false, "internal"),
			"arguments":   operationAnyMapParam(params, "arguments"),
		}
	case OperationActionPulsarRetentionUpdate:
		snapshot["config"] = map[string]any{
			"retentionTimeInMinutes": operationIntParam(params, 0, "retentionTimeInMinutes"),
			"retentionSizeInMB":      operationIntParam(params, 0, "retentionSizeInMB"),
		}
	case OperationActionPulsarTTLUpdate:
		snapshot["config"] = map[string]any{
			"messageTTLInSeconds": operationIntParam(params, 0, "messageTTLInSeconds", "ttlSeconds"),
		}
	default:
		snapshot["params"] = params
	}
	return snapshot
}

func buildOperationDiff(before, after map[string]any) []OperationDiffItem {
	if len(after) == 0 {
		return nil
	}
	diff := make([]OperationDiffItem, 0)
	for _, key := range []string{"resourceType", "namespace", "name", "durable", "partitionCount", "replicaCount"} {
		afterValue, ok := after[key]
		if !ok {
			continue
		}
		var beforeValue any
		if before != nil {
			beforeValue = before[key]
		}
		if fmt.Sprint(beforeValue) != fmt.Sprint(afterValue) {
			diff = append(diff, OperationDiffItem{Key: key, Before: beforeValue, After: afterValue, Risk: RiskLevelLow})
		}
	}
	afterConfig := snapshotConfig(after)
	beforeConfig := snapshotConfig(before)
	for key, afterValue := range afterConfig {
		beforeValue := beforeConfig[key]
		if fmt.Sprint(beforeValue) != fmt.Sprint(afterValue) {
			diff = append(diff, OperationDiffItem{Key: "config." + key, Before: beforeValue, After: afterValue, Risk: configDiffRisk(key, beforeValue, afterValue)})
		}
	}
	return diff
}

func applyRabbitMQImmutableGuards(validation *ResourceOperationValidationVO) {
	if validation == nil || len(validation.Before) == 0 {
		return
	}
	beforeConfig := snapshotConfig(validation.Before)
	afterConfig := snapshotConfig(validation.After)
	switch validation.Action {
	case OperationActionRabbitMQQueueUpsert:
		if snapshotBool(validation.Before["durable"]) != snapshotBool(validation.After["durable"]) {
			markOperationUnsupported(validation, "Queue 已存在，durable 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
			return
		}
		if snapshotBool(beforeConfig["auto_delete"]) != snapshotBool(afterConfig["auto_delete"]) {
			markOperationUnsupported(validation, "Queue 已存在，auto_delete 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
			return
		}
		beforeType := stringValue(beforeConfig["type"])
		afterArguments := snapshotConfig(afterConfig["arguments"])
		afterType := firstNonEmpty(stringValue(afterArguments["x-queue-type"]), stringValue(afterArguments["type"]))
		if beforeType != "" && afterType != "" && beforeType != afterType {
			markOperationUnsupported(validation, "Queue 已存在，x-queue-type 不支持直接变更；classic/quorum/stream 类型切换需要删除重建")
			return
		}
	case OperationActionRabbitMQExchangeUpsert:
		if stringValue(beforeConfig["type"]) != "" && stringValue(afterConfig["type"]) != "" && stringValue(beforeConfig["type"]) != stringValue(afterConfig["type"]) {
			markOperationUnsupported(validation, "Exchange 已存在，type 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
			return
		}
		if snapshotBool(validation.Before["durable"]) != snapshotBool(validation.After["durable"]) {
			markOperationUnsupported(validation, "Exchange 已存在，durable 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
			return
		}
		if snapshotBool(beforeConfig["auto_delete"]) != snapshotBool(afterConfig["auto_delete"]) {
			markOperationUnsupported(validation, "Exchange 已存在，auto_delete 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
			return
		}
		if snapshotBool(beforeConfig["internal"]) != snapshotBool(afterConfig["internal"]) {
			markOperationUnsupported(validation, "Exchange 已存在，internal 不支持通过二期 upsert 直接变更；如需变更请走高风险删除重建流程")
		}
	}
}

func applyDestructiveConfigRisk(validation *ResourceOperationValidationVO) {
	if validation == nil || len(validation.Before) == 0 {
		return
	}
	beforeConfig := snapshotConfig(validation.Before)
	afterConfig := snapshotConfig(validation.After)
	switch validation.Action {
	case OperationActionKafkaTopicConfigUpdate:
		for key, afterValue := range afterConfig {
			beforeValue := beforeConfig[key]
			if configValueDecrease(key, beforeValue, afterValue) || cleanupPolicyBecomesDeleteOnly(key, beforeValue, afterValue) {
				elevateOperationRisk(validation, fmt.Sprintf("%s 从 %s 调整为 %s 可能导致历史消息更快清理或写入行为变化，已升级为高风险", key, stringValue(beforeValue), stringValue(afterValue)))
			}
		}
	case OperationActionPulsarRetentionUpdate, OperationActionPulsarTTLUpdate:
		for key, afterValue := range afterConfig {
			beforeValue := beforeConfig[key]
			if configValueDecrease(key, beforeValue, afterValue) {
				elevateOperationRisk(validation, fmt.Sprintf("%s 从 %s 调整为 %s 可能导致消息更快过期或清理，已升级为高风险", key, stringValue(beforeValue), stringValue(afterValue)))
			}
		}
	}
}

func elevateOperationRisk(validation *ResourceOperationValidationVO, warning string) {
	if validation == nil {
		return
	}
	validation.RiskLevel = RiskLevelHigh
	validation.RequiredPermission = PermissionHighRisk
	validation.RequiresHighRiskAck = true
	validation.Warnings = appendUniqueString(validation.Warnings, warning)
}

func markOperationUnsupported(validation *ResourceOperationValidationVO, message string) {
	validation.Supported = false
	validation.Message = message
	validation.Warnings = appendUniqueString(validation.Warnings, message)
}

func configDiffRisk(key string, before, after any) string {
	if configValueDecrease(key, before, after) || cleanupPolicyBecomesDeleteOnly(key, before, after) {
		return RiskLevelHigh
	}
	switch key {
	case "retention.ms", "retention.bytes", "cleanup.policy", "max.message.bytes", "min.insync.replicas", "messageTTLInSeconds", "retentionTimeInMinutes", "retentionSizeInMB":
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

func configValueDecrease(key string, before, after any) bool {
	switch key {
	case "retention.ms", "retention.bytes", "max.message.bytes", "retentionTimeInMinutes", "retentionSizeInMB", "messageTTLInSeconds":
		beforeInt, beforeOK := parseConfigInt64(before)
		afterInt, afterOK := parseConfigInt64(after)
		return beforeOK && afterOK && beforeInt >= 0 && afterInt >= 0 && afterInt < beforeInt
	default:
		return false
	}
}

func cleanupPolicyBecomesDeleteOnly(key string, before, after any) bool {
	if key != "cleanup.policy" {
		return false
	}
	beforePolicy := strings.ToLower(stringValue(before))
	afterPolicy := strings.ToLower(stringValue(after))
	return strings.Contains(beforePolicy, "compact") && afterPolicy == "delete"
}

func parseConfigInt64(value any) (int64, bool) {
	text := strings.TrimSpace(stringValue(value))
	if text == "" {
		return 0, false
	}
	var number json.Number = json.Number(text)
	parsed, err := number.Int64()
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseJSONMap(value string) map[string]any {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func snapshotConfig(snapshot any) map[string]any {
	if snapshot == nil {
		return map[string]any{}
	}
	switch item := snapshot.(type) {
	case map[string]any:
		if config, ok := item["config"]; ok {
			return snapshotConfig(config)
		}
		return item
	case map[string]string:
		result := make(map[string]any, len(item))
		for key, value := range item {
			result[key] = value
		}
		return result
	default:
		return map[string]any{}
	}
}

func snapshotBool(value any) bool {
	return boolValue(value)
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func buildResourceOperationLockKey(instanceID uint, validation *ResourceOperationValidationVO) string {
	if validation == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s:%s:%s:%s", instanceID, validation.ResourceType, validation.Namespace, validation.ResourceName, validation.Action)
}

func acquireResourceOperationLock(lockKey string) bool {
	lockKey = strings.TrimSpace(lockKey)
	if lockKey == "" {
		return true
	}
	_, loaded := resourceOperationLocks.LoadOrStore(lockKey, time.Now())
	return !loaded
}

func releaseResourceOperationLock(lockKey string) {
	lockKey = strings.TrimSpace(lockKey)
	if lockKey == "" {
		return
	}
	resourceOperationLocks.Delete(lockKey)
}

func (uc *UseCase) idempotentResourceOperationResult(ctx context.Context, instance *MQInstance, validation *ResourceOperationValidationVO, req *ResourceOperationRequest) (*ResourceOperationResultVO, bool, error) {
	if uc.operationAuditRepo == nil || instance == nil || validation == nil || req == nil || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, false, nil
	}
	existing, err := uc.operationAuditRepo.GetByIdempotencyKey(ctx, instance.ID, req.IdempotencyKey)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, nil
	}
	switch existing.Status {
	case AuditStatusPending:
		return nil, false, fmt.Errorf("相同幂等键的操作正在执行，请稍后查询审计结果")
	case AuditStatusSuccess, AuditStatusPartial:
		result := map[string]any{}
		if strings.TrimSpace(existing.ResultJSON) != "" {
			_ = json.Unmarshal([]byte(existing.ResultJSON), &result)
		}
		return &ResourceOperationResultVO{
			AuditID:      existing.ID,
			InstanceID:   existing.InstanceID,
			MQType:       existing.MQType,
			Action:       existing.Action,
			ActionText:   actionText(existing.Action),
			RiskLevel:    existing.RiskLevel,
			ResourceType: existing.ResourceType,
			Namespace:    existing.Namespace,
			ResourceName: existing.ResourceName,
			Status:       existing.Status,
			Message:      firstNonEmpty(existing.Message, "重复提交已返回上次操作结果"),
			DurationMs:   existing.DurationMs,
			Result:       result,
			ExecutedAt:   formatTimePtr(existing.FinishedAt),
		}, true, nil
	default:
		return nil, false, fmt.Errorf("相同幂等键已有失败操作记录，请更换幂等键后重试")
	}
}

func fillOperationValidationDefaults(validation *ResourceOperationValidationVO, instance *MQInstance, req *ResourceOperationRequest) {
	if validation == nil || instance == nil || req == nil {
		return
	}
	if validation.InstanceID == 0 {
		validation.InstanceID = instance.ID
	}
	if validation.MQType == "" {
		validation.MQType = instance.MQType
	}
	if validation.Action == "" {
		validation.Action = req.Action
	}
	if validation.ActionText == "" {
		validation.ActionText = actionText(validation.Action)
	}
	if validation.RiskLevel == "" {
		validation.RiskLevel = RiskLevelLow
	}
	if validation.RequiredPermission == 0 {
		validation.RequiredPermission = PermissionResourceManage
	}
	if validation.ResourceType == "" {
		validation.ResourceType = req.ResourceType
	}
	if validation.Namespace == "" {
		validation.Namespace = req.Namespace
	}
	if validation.ResourceName == "" {
		validation.ResourceName = req.ResourceName
	}
	if validation.NormalizedParams == nil {
		validation.NormalizedParams = cloneParams(req.Params)
	}
	if IsHighRiskLevel(validation.RiskLevel) {
		validation.RequiresHighRiskAck = true
	}
	validation.RequiresConfirm = true
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

func (uc *UseCase) startResourceOperationAudit(ctx context.Context, item *MQInstance, validation *ResourceOperationValidationVO, req *ResourceOperationRequest, operator Operator) *MQOperationAudit {
	if uc.operationAuditRepo == nil || item == nil || validation == nil {
		return nil
	}
	now := time.Now()
	reason := ""
	idempotencyKey := ""
	confirmText := ""
	if req != nil {
		reason = req.Reason
		idempotencyKey = req.IdempotencyKey
		confirmText = req.ConfirmText
	}
	operationID := fmt.Sprintf("mqop-%d-%d", item.ID, time.Now().UnixNano())
	audit := &MQOperationAudit{
		InstanceID:         item.ID,
		InstanceName:       item.Name,
		MQType:             item.MQType,
		ResourceType:       validation.ResourceType,
		ResourceName:       validation.ResourceName,
		Namespace:          validation.Namespace,
		Action:             validation.Action,
		RiskLevel:          validation.RiskLevel,
		Status:             AuditStatusPending,
		OperationID:        operationID,
		IdempotencyKey:     strings.TrimSpace(idempotencyKey),
		LockKey:            validation.LockKey,
		ConfirmText:        strings.TrimSpace(confirmText),
		RequestJSON:        mustJSON(req),
		BeforeSnapshotJSON: mustJSON(validation.Before),
		AfterSnapshotJSON:  mustJSON(validation.After),
		DiffJSON:           mustJSON(validation.Diff),
		WarningsJSON:       mustJSON(validation.Warnings),
		ImpactSummaryJSON:  mustJSON(validation.Impacts),
		Reason:             trimText(reason, 500),
		OperatorID:         operator.ID,
		OperatorName:       operator.Username,
		ClientIP:           operator.ClientIP,
		StartedAt:          &now,
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
	case OperationActionRabbitMQQueueUpsert:
		return "RabbitMQ Queue 创建/更新"
	case OperationActionRabbitMQExchangeUpsert:
		return "RabbitMQ Exchange 创建/更新"
	case OperationActionRabbitMQBindingUpsert:
		return "RabbitMQ Binding 创建/更新"
	case OperationActionRabbitMQQueuePurge:
		return "RabbitMQ Queue 清空"
	case OperationActionRabbitMQQueueDelete:
		return "RabbitMQ Queue 删除"
	case OperationActionRabbitMQExchangeDelete:
		return "RabbitMQ Exchange 删除"
	case OperationActionKafkaTopicCreate:
		return "Kafka Topic 创建"
	case OperationActionKafkaPartitionsExpand:
		return "Kafka 分区扩容"
	case OperationActionKafkaTopicConfigUpdate:
		return "Kafka Topic 配置更新"
	case OperationActionKafkaTopicDelete:
		return "Kafka Topic 删除"
	case OperationActionPulsarRetentionUpdate:
		return "Pulsar Namespace Retention 更新"
	case OperationActionPulsarTTLUpdate:
		return "Pulsar Namespace TTL 更新"
	case OperationActionPulsarTopicDelete:
		return "Pulsar Topic 删除"
	case OperationActionPulsarSubscriptionSkip:
		return "Pulsar Subscription 跳过"
	case OperationActionPulsarSubscriptionReset:
		return "Pulsar Subscription Cursor 重置"
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
	case AuditStatusPartial:
		return "部分成功"
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapToJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
