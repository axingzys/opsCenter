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
	metricSnapshotRepo     MetricSnapshotRepo
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
	metricSnapshotRepo MetricSnapshotRepo,
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
		metricSnapshotRepo:     metricSnapshotRepo,
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
	return uc.buildOverview(ctx, instanceID)
}

func (uc *UseCase) CollectMetricSnapshot(ctx context.Context, instanceID uint, operator Operator) (*MetricCollectResultVO, error) {
	if uc.metricSnapshotRepo == nil {
		return nil, fmt.Errorf("MQ指标快照仓库未配置")
	}
	overview, err := uc.buildOverview(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	collectedAt := time.Now()
	snapshot := &MQMetricSnapshot{
		InstanceID:        overview.InstanceID,
		ResourceType:      "instance",
		ResourceName:      overview.InstanceName,
		BrokerCount:       int(overview.BrokerCount),
		OnlineBrokerCount: int(overview.OnlineBrokerCount),
		MessageCount:      overview.MessageCount,
		Backlog:           overview.Backlog,
		Lag:               overview.Lag,
		ProducedRate:      overview.ProducedRate,
		ConsumedRate:      overview.ConsumedRate,
		ConsumerCount:     int(overview.ConsumerGroupCount),
		CollectedAt:       collectedAt,
	}
	if err := uc.metricSnapshotRepo.Create(ctx, snapshot); err != nil {
		return nil, err
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	instance.LastMetricAt = &collectedAt
	overview.LastMetricAt = formatTimePtr(instance.LastMetricAt)
	overview.HealthStatus, overview.HealthReasons, overview.AnomalyTags = evaluateMQHealth(
		instance,
		int64(snapshot.BrokerCount),
		int64(snapshot.OnlineBrokerCount),
		&ResourceSummary{
			Count:                   overview.ResourceCount,
			MessageCount:            overview.MessageCount,
			Backlog:                 overview.Backlog,
			ProducedRate:            overview.ProducedRate,
			ConsumedRate:            overview.ConsumedRate,
			NoConsumerResourceCount: overview.NoConsumerResources,
			DLQResourceCount:        overview.DLQResources,
			RetryResourceCount:      overview.RetryResources,
		},
		&ConsumerGroupSummary{Count: overview.ConsumerGroupCount, Lag: snapshot.Lag},
		snapshot.Backlog,
	)
	overview.HealthText = HealthText(overview.HealthStatus)
	instance.HealthStatus = overview.HealthStatus
	if err := uc.instanceRepo.Update(ctx, instance); err != nil {
		return nil, err
	}
	audit := uc.startOperationAudit(ctx, instance, AuditActionMetricSnapshot, RiskLevelLow, operator, map[string]any{"source": "manual"})
	uc.finishOperationAudit(ctx, audit, AuditStatusSuccess, "MQ指标快照采集成功", map[string]any{
		"snapshotId":    snapshot.ID,
		"healthStatus":  overview.HealthStatus,
		"anomalyTags":   overview.AnomalyTags,
		"healthReasons": overview.HealthReasons,
	}, collectedAt, time.Now())
	return &MetricCollectResultVO{
		InstanceID:    overview.InstanceID,
		InstanceName:  overview.InstanceName,
		MQType:        overview.MQType,
		HealthStatus:  overview.HealthStatus,
		HealthText:    overview.HealthText,
		HealthReasons: overview.HealthReasons,
		AnomalyTags:   overview.AnomalyTags,
		Snapshot:      toMetricSnapshotVO(snapshot),
		Message:       "MQ指标快照采集成功",
		CollectedAt:   collectedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) ListMetricSnapshots(ctx context.Context, instanceID uint, req *MetricSnapshotListRequest) ([]*MetricSnapshotVO, int64, error) {
	if uc.metricSnapshotRepo == nil {
		return nil, 0, fmt.Errorf("MQ指标快照仓库未配置")
	}
	normalizeMetricSnapshotListRequest(req)
	items, total, err := uc.metricSnapshotRepo.List(ctx, instanceID, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*MetricSnapshotVO, 0, len(items))
	for _, item := range items {
		list = append(list, toMetricSnapshotVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) GenerateInspectionReport(ctx context.Context, instanceID uint, operator Operator) (*InspectionReportVO, error) {
	overview, err := uc.buildOverview(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	now := time.Now()
	findings := uc.buildInspectionFindings(ctx, instance, overview)
	sections := buildInspectionSections(instance, overview, findings)
	score, riskLevel := calculateInspectionScore(findings)
	report := &InspectionReportVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		MQType:       instance.MQType,
		Score:        score,
		RiskLevel:    riskLevel,
		Summary:      buildInspectionSummary(score, riskLevel, findings),
		Sections:     sections,
		Findings:     findings,
		GeneratedAt:  now.Format("2006-01-02 15:04:05"),
	}
	audit := uc.startOperationAudit(ctx, instance, AuditActionInspectionRun, RiskLevelLow, operator, map[string]any{"source": "manual"})
	uc.finishOperationAudit(ctx, audit, AuditStatusSuccess, "MQ巡检报告生成成功", map[string]any{
		"score":        report.Score,
		"riskLevel":    report.RiskLevel,
		"findingCount": len(report.Findings),
	}, now, time.Now())
	return report, nil
}

func (uc *UseCase) buildOverview(ctx context.Context, instanceID uint) (*OverviewVO, error) {
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
	backlog := resourceSummary.Backlog + groupSummary.Backlog
	healthStatus, healthReasons, anomalyTags := evaluateMQHealth(instance, brokerCount, onlineBrokerCount, resourceSummary, groupSummary, backlog)
	return &OverviewVO{
		InstanceID:          instance.ID,
		InstanceName:        instance.Name,
		MQType:              instance.MQType,
		HealthStatus:        healthStatus,
		HealthText:          HealthText(healthStatus),
		HealthReasons:       healthReasons,
		AnomalyTags:         anomalyTags,
		BrokerCount:         brokerCount,
		OnlineBrokerCount:   onlineBrokerCount,
		ResourceCount:       resourceSummary.Count,
		ConsumerGroupCount:  groupSummary.Count,
		PartitionCount:      partitionCount,
		MessageCount:        resourceSummary.MessageCount,
		Backlog:             backlog,
		Lag:                 groupSummary.Lag,
		ProducedRate:        resourceSummary.ProducedRate,
		ConsumedRate:        resourceSummary.ConsumedRate,
		NoConsumerResources: resourceSummary.NoConsumerResourceCount,
		DLQResources:        resourceSummary.DLQResourceCount,
		RetryResources:      resourceSummary.RetryResourceCount,
		TopBacklogResources: topVO,
		LastSyncAt:          formatTimePtr(instance.LastSyncAt),
		LastMetricAt:        formatTimePtr(instance.LastMetricAt),
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
		if err := validateHighRiskConfirmText(req, validation); err != nil {
			return nil, err
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
		if snapshot, snapErr := uc.afterOperationSnapshot(ctx, instance.ID, validation, resourceType, namespace, resourceName); snapErr == nil && len(snapshot) > 0 {
			audit.AfterSnapshotJSON = mustJSON(snapshot)
		} else if len(validation.After) > 0 {
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
	if validation.ResourceType == ResourceTypeSubscription {
		consumerGroup, err := uc.lookupOperationConsumerGroup(ctx, instance.ID, validation)
		if err != nil {
			return err
		}
		if consumerGroup != nil {
			validation.Before = consumerGroupSnapshot(consumerGroup)
		}
	}
	validation.After = operationDesiredSnapshot(validation)
	validation.Diff = buildOperationDiff(validation.Before, validation.After)

	applyRabbitMQImmutableGuards(validation)
	applyDestructiveConfigRisk(validation)
	uc.applyHighRiskOperationGuards(ctx, instance, validation, req)
	return nil
}

func (uc *UseCase) lookupOperationResource(ctx context.Context, instanceID uint, validation *ResourceOperationValidationVO) (*MQResource, error) {
	if uc.resourceRepo == nil || validation == nil || validation.ResourceType == "" || validation.ResourceName == "" {
		return nil, nil
	}
	return uc.resourceRepo.GetByUnique(ctx, instanceID, validation.ResourceType, validation.Namespace, validation.ResourceName)
}

func (uc *UseCase) lookupOperationConsumerGroup(ctx context.Context, instanceID uint, validation *ResourceOperationValidationVO) (*MQConsumerGroup, error) {
	if uc.consumerGroupRepo == nil || validation == nil {
		return nil, nil
	}
	params := cloneParams(validation.NormalizedParams)
	resourceName := firstNonEmpty(operationStringParam(params, "topic", "topicName", "name"), validation.ResourceName)
	groupName := firstNonEmpty(operationStringParam(params, "subscription", "subscriptionName", "groupName"), validation.ResourceName)
	return uc.consumerGroupRepo.GetByUnique(ctx, instanceID, validation.Namespace, resourceName, groupName)
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

func consumerGroupSnapshot(item *MQConsumerGroup) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"id":                  item.ID,
		"resourceType":        ResourceTypeSubscription,
		"namespace":           item.Namespace,
		"name":                item.GroupName,
		"groupName":           item.GroupName,
		"resourceName":        item.ResourceName,
		"state":               item.State,
		"consumerCount":       item.ConsumerCount,
		"activeConsumerCount": item.ActiveConsumerCount,
		"currentOffset":       item.CurrentOffset,
		"endOffset":           item.EndOffset,
		"lag":                 item.Lag,
		"backlog":             item.Backlog,
		"lastConsumedAt":      formatTimePtr(item.LastConsumedAt),
		"lastSyncAt":          formatTimePtr(item.LastSyncAt),
		"metadata":            parseJSONMap(item.MetadataJSON),
	}
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
	case OperationActionPulsarSubscriptionSkip:
		snapshot["topic"] = operationStringParam(params, "topic", "topicName", "name")
		snapshot["subscription"] = operationStringParam(params, "subscription", "subscriptionName", "groupName")
		snapshot["backlog"] = 0
		snapshot["lag"] = 0
	case OperationActionPulsarSubscriptionReset:
		snapshot["topic"] = operationStringParam(params, "topic", "topicName", "name")
		snapshot["subscription"] = operationStringParam(params, "subscription", "subscriptionName", "groupName")
		snapshot["targetTimestampMs"] = operationIntParam(params, 0, "timestampMs", "timestamp")
	default:
		snapshot["params"] = params
	}
	return snapshot
}

func (uc *UseCase) applyHighRiskOperationGuards(ctx context.Context, instance *MQInstance, validation *ResourceOperationValidationVO, req *ResourceOperationRequest) {
	if uc == nil || instance == nil || validation == nil || req == nil {
		return
	}
	switch validation.Action {
	case OperationActionRabbitMQExchangeDelete:
		bindingCount, err := uc.countRabbitMQExchangeBindings(ctx, instance.ID, validation.Namespace, validation.ResourceName)
		if err != nil {
			validation.Warnings = appendUniqueString(validation.Warnings, "检查 Exchange 绑定关系失败: "+err.Error())
			return
		}
		if bindingCount <= 0 {
			return
		}
		impact := fmt.Sprintf("该 Exchange 当前存在 %d 条 binding，删除后相关消息路由会失效", bindingCount)
		validation.Impacts = appendUniqueString(validation.Impacts, impact)
		validation.Warnings = appendUniqueString(validation.Warnings, impact)
		if !operationBoolParam(req.Params, false, "force") {
			markOperationUnsupported(validation, "Exchange 存在 binding，需在参数中显式设置 force=true 后才允许删除")
		}
	case OperationActionPulsarSubscriptionSkip, OperationActionPulsarSubscriptionReset:
		if len(validation.Before) == 0 {
			return
		}
		backlog := int64Value(validation.Before["backlog"])
		consumerCount := intValue(validation.Before["consumerCount"])
		activeConsumerCount := intValue(validation.Before["activeConsumerCount"])
		validation.Impacts = appendUniqueString(validation.Impacts, fmt.Sprintf("当前 backlog: %d", backlog))
		validation.Impacts = appendUniqueString(validation.Impacts, fmt.Sprintf("当前消费者数: %d，活跃消费者数: %d", consumerCount, activeConsumerCount))
		if activeConsumerCount > 0 {
			validation.Warnings = appendUniqueString(validation.Warnings, "当前 subscription 存在在线消费者，reset/skip 后可能立即影响消费结果")
		}
	}
}

func (uc *UseCase) countRabbitMQExchangeBindings(ctx context.Context, instanceID uint, vhost, exchange string) (int, error) {
	if uc.bindingRepo == nil {
		return 0, nil
	}
	bindings, err := uc.bindingRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		if vhost != "" && binding.VHost != vhost {
			continue
		}
		if binding.Source == exchange || (binding.DestinationType == ResourceTypeExchange && binding.Destination == exchange) {
			count++
		}
	}
	return count, nil
}

func validateHighRiskConfirmText(req *ResourceOperationRequest, validation *ResourceOperationValidationVO) error {
	if req == nil || validation == nil {
		return nil
	}
	expected := strings.TrimSpace(validation.ResourceName)
	actual := strings.TrimSpace(req.ConfirmText)
	if expected == "" {
		return nil
	}
	if actual == "" {
		return fmt.Errorf("高风险操作必须输入资源名确认")
	}
	if actual != expected {
		return fmt.Errorf("高风险操作资源名确认不一致")
	}
	return nil
}

func (uc *UseCase) afterOperationSnapshot(ctx context.Context, instanceID uint, validation *ResourceOperationValidationVO, resourceType, namespace, resourceName string) (map[string]any, error) {
	if validation == nil {
		return nil, nil
	}
	if validation.ResourceType == ResourceTypeSubscription {
		consumerGroup, err := uc.lookupOperationConsumerGroup(ctx, instanceID, validation)
		if err != nil {
			return nil, err
		}
		if consumerGroup != nil {
			return consumerGroupSnapshot(consumerGroup), nil
		}
		return map[string]any{
			"resourceType": ResourceTypeSubscription,
			"namespace":    validation.Namespace,
			"name":         validation.ResourceName,
			"action":       validation.Action,
			"exists":       false,
		}, nil
	}
	if uc.resourceRepo != nil && resourceType != "" && resourceName != "" {
		resource, err := uc.resourceRepo.GetByUnique(ctx, instanceID, resourceType, namespace, resourceName)
		if err != nil {
			return nil, err
		}
		if resource != nil {
			return resourceSnapshot(resource), nil
		}
	}
	if operationDeletesResource(validation.Action) {
		return map[string]any{
			"resourceType": resourceType,
			"namespace":    namespace,
			"name":         resourceName,
			"action":       validation.Action,
			"exists":       false,
			"deleted":      true,
		}, nil
	}
	return validation.After, nil
}

func operationDeletesResource(action string) bool {
	switch action {
	case OperationActionRabbitMQQueueDelete, OperationActionRabbitMQExchangeDelete, OperationActionKafkaTopicDelete, OperationActionPulsarTopicDelete:
		return true
	default:
		return false
	}
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

func normalizeMetricSnapshotListRequest(req *MetricSnapshotListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
}

const (
	mqWarningBacklogThreshold  = int64(1000)
	mqCriticalBacklogThreshold = int64(100000)
	mqWarningLagThreshold      = int64(1000)
	mqCriticalLagThreshold     = int64(100000)
	mqStaleSyncAge             = 30 * time.Minute
	mqStaleMetricAge           = 10 * time.Minute
)

func evaluateMQHealth(instance *MQInstance, brokerCount, onlineBrokerCount int64, resourceSummary *ResourceSummary, groupSummary *ConsumerGroupSummary, backlog int64) (string, []string, []string) {
	if instance == nil {
		return HealthStatusUnknown, []string{"实例信息不可用"}, []string{"实例未知"}
	}
	reasons := make([]string, 0)
	tags := make([]string, 0)
	severity := 0
	mark := func(level int, reason, tag string) {
		if level > severity {
			severity = level
		}
		if reason != "" {
			reasons = appendUniqueString(reasons, reason)
		}
		if tag != "" {
			tags = appendUniqueString(tags, tag)
		}
	}
	if strings.TrimSpace(instance.Status) != InstanceStatusEnabled {
		mark(1, "实例当前处于禁用状态", "实例禁用")
	}
	switch strings.TrimSpace(instance.HealthStatus) {
	case HealthStatusCritical:
		mark(3, "最近连接测试或同步结果异常", "连接异常")
	case HealthStatusWarning:
		mark(2, "实例已有警告状态", "实例警告")
	}
	if brokerCount > 0 && onlineBrokerCount == 0 {
		mark(3, "全部 Broker 均非在线状态", "Broker 离线")
	} else if brokerCount > 0 && onlineBrokerCount < brokerCount {
		mark(2, fmt.Sprintf("Broker 在线数 %d/%d", onlineBrokerCount, brokerCount), "Broker 部分离线")
	}
	if instance.LastSyncAt == nil || instance.LastSyncAt.IsZero() {
		mark(1, "尚未完成元数据同步", "同步过期")
	} else if age := time.Since(*instance.LastSyncAt); age > mqStaleSyncAge {
		mark(2, fmt.Sprintf("元数据同步已超过 %d 分钟", int(mqStaleSyncAge.Minutes())), "同步过期")
	}
	if instance.LastMetricAt == nil || instance.LastMetricAt.IsZero() {
		mark(1, "尚未采集指标快照", "未采集指标")
	} else if age := time.Since(*instance.LastMetricAt); age > mqStaleMetricAge {
		mark(2, fmt.Sprintf("指标快照已超过 %d 分钟", int(mqStaleMetricAge.Minutes())), "指标过期")
	}
	if backlog >= mqCriticalBacklogThreshold {
		mark(3, fmt.Sprintf("Backlog 已达到 %d", backlog), "堆积严重")
	} else if backlog > 0 {
		mark(2, fmt.Sprintf("Backlog 当前为 %d", backlog), "堆积增长")
	}
	lag := int64(0)
	if groupSummary != nil {
		lag = groupSummary.Lag
	}
	if lag >= mqCriticalLagThreshold {
		mark(3, fmt.Sprintf("Lag 已达到 %d", lag), "消费严重延迟")
	} else if lag > 0 {
		mark(2, fmt.Sprintf("Lag 当前为 %d", lag), "消费延迟")
	}
	if resourceSummary != nil {
		if resourceSummary.NoConsumerResourceCount > 0 {
			mark(2, fmt.Sprintf("存在 %d 个无消费者 Topic/Queue", resourceSummary.NoConsumerResourceCount), "无消费者")
		}
		if resourceSummary.DLQResourceCount > 0 {
			mark(2, fmt.Sprintf("识别到 %d 个 DLQ 资源", resourceSummary.DLQResourceCount), "DLQ 资源")
		}
		if resourceSummary.RetryResourceCount > 0 {
			mark(2, fmt.Sprintf("识别到 %d 个 Retry 资源", resourceSummary.RetryResourceCount), "Retry 资源")
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "指标状态正常")
	}
	switch severity {
	case 3:
		return HealthStatusCritical, reasons, tags
	case 2:
		return HealthStatusWarning, reasons, tags
	case 1:
		return HealthStatusUnknown, reasons, tags
	default:
		return HealthStatusHealthy, reasons, tags
	}
}

func (uc *UseCase) buildInspectionFindings(ctx context.Context, instance *MQInstance, overview *OverviewVO) []*InspectionFindingVO {
	findings := make([]*InspectionFindingVO, 0)
	add := func(severity, category, title, description, resourceType, resourceName string) {
		findings = append(findings, &InspectionFindingVO{
			Severity:     severity,
			Category:     category,
			Title:        title,
			Description:  description,
			ResourceType: resourceType,
			ResourceName: resourceName,
		})
	}
	if overview == nil || instance == nil {
		add("critical", "health", "巡检数据不可用", "实例或概览数据为空", "instance", "")
		return findings
	}
	switch overview.HealthStatus {
	case HealthStatusCritical:
		add("critical", "health", "实例健康异常", strings.Join(overview.HealthReasons, "；"), "instance", instance.Name)
	case HealthStatusWarning:
		add("warning", "health", "实例存在健康警告", strings.Join(overview.HealthReasons, "；"), "instance", instance.Name)
	case HealthStatusUnknown:
		add("warning", "health", "实例健康未知", strings.Join(overview.HealthReasons, "；"), "instance", instance.Name)
	}
	if overview.BrokerCount > 0 && overview.OnlineBrokerCount < overview.BrokerCount {
		severity := "warning"
		if overview.OnlineBrokerCount == 0 {
			severity = "critical"
		}
		add(severity, "health", "Broker 在线状态异常", fmt.Sprintf("Broker 在线数 %d/%d", overview.OnlineBrokerCount, overview.BrokerCount), "broker", instance.Name)
	}
	if overview.Backlog >= mqCriticalBacklogThreshold {
		add("critical", "capacity", "消息堆积严重", fmt.Sprintf("当前 Backlog %d", overview.Backlog), "instance", instance.Name)
	} else if overview.Backlog >= mqWarningBacklogThreshold || overview.Backlog > 0 {
		add("warning", "capacity", "存在消息堆积", fmt.Sprintf("当前 Backlog %d", overview.Backlog), "instance", instance.Name)
	}
	if overview.Lag >= mqCriticalLagThreshold {
		add("critical", "consumer", "消费延迟严重", fmt.Sprintf("当前 Lag %d", overview.Lag), "consumer_group", instance.Name)
	} else if overview.Lag >= mqWarningLagThreshold || overview.Lag > 0 {
		add("warning", "consumer", "存在消费延迟", fmt.Sprintf("当前 Lag %d", overview.Lag), "consumer_group", instance.Name)
	}
	if overview.NoConsumerResources > 0 {
		add("warning", "consumer", "存在无消费者资源", fmt.Sprintf("%d 个 Topic/Queue 当前消费者数为 0", overview.NoConsumerResources), "resource", instance.Name)
	}
	if overview.DLQResources > 0 {
		add("warning", "governance", "识别到 DLQ 资源", fmt.Sprintf("%d 个资源命中 DLQ 命名规则，需要关注堆积趋势", overview.DLQResources), "resource", instance.Name)
	}
	if overview.RetryResources > 0 {
		add("info", "governance", "识别到 Retry 资源", fmt.Sprintf("%d 个资源命中 Retry 命名规则", overview.RetryResources), "resource", instance.Name)
	}
	if strings.TrimSpace(instance.Owner) == "" {
		add("warning", "governance", "实例未绑定负责人", "四期告警和巡检治理需要明确负责人", "instance", instance.Name)
	}
	if strings.TrimSpace(instance.BusinessSystem) == "" {
		add("warning", "governance", "实例未绑定业务系统", "建议补充业务系统，用于告警路由和影响面判断", "instance", instance.Name)
	}
	if uc.operationAuditRepo != nil {
		start := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
		_, highTotal, _ := uc.operationAuditRepo.List(ctx, &AuditListRequest{Page: 1, PageSize: 1, InstanceID: instance.ID, RiskLevel: RiskLevelHigh, Status: AuditStatusFailed, StartTime: start})
		_, criticalTotal, _ := uc.operationAuditRepo.List(ctx, &AuditListRequest{Page: 1, PageSize: 1, InstanceID: instance.ID, RiskLevel: RiskLevelCritical, Status: AuditStatusFailed, StartTime: start})
		if highTotal+criticalTotal > 0 {
			add("warning", "governance", "近期存在高危操作失败", fmt.Sprintf("近 7 天高危/严重操作失败 %d 次", highTotal+criticalTotal), "audit", instance.Name)
		}
	}
	return findings
}

func buildInspectionSections(instance *MQInstance, overview *OverviewVO, findings []*InspectionFindingVO) []*InspectionSectionVO {
	if overview == nil {
		return nil
	}
	return []*InspectionSectionVO{
		{
			Key:     "health",
			Title:   "健康",
			Status:  inspectionSectionStatus(findings, "health"),
			Summary: strings.Join(overview.HealthReasons, "；"),
			Metrics: []*InspectionMetricVO{
				{Name: "健康状态", Value: overview.HealthText, Status: overview.HealthStatus},
				{Name: "Broker 在线", Value: fmt.Sprintf("%d/%d", overview.OnlineBrokerCount, overview.BrokerCount), Status: metricStatus(overview.BrokerCount == 0 || overview.OnlineBrokerCount == overview.BrokerCount)},
				{Name: "最近同步", Value: fallbackDash(overview.LastSyncAt), Status: metricStatus(overview.LastSyncAt != "")},
				{Name: "最近指标", Value: fallbackDash(overview.LastMetricAt), Status: metricStatus(overview.LastMetricAt != "")},
			},
		},
		{
			Key:     "capacity",
			Title:   "容量",
			Status:  inspectionSectionStatus(findings, "capacity"),
			Summary: fmt.Sprintf("消息数 %d，Backlog %d", overview.MessageCount, overview.Backlog),
			Metrics: []*InspectionMetricVO{
				{Name: "消息数", Value: overview.MessageCount, Status: "success"},
				{Name: "Backlog", Value: overview.Backlog, Status: metricRiskStatus(overview.Backlog, mqWarningBacklogThreshold, mqCriticalBacklogThreshold)},
				{Name: "生产/s", Value: overview.ProducedRate, Status: "success"},
				{Name: "消费/s", Value: overview.ConsumedRate, Status: "success"},
			},
		},
		{
			Key:     "consumer",
			Title:   "消费",
			Status:  inspectionSectionStatus(findings, "consumer"),
			Summary: fmt.Sprintf("消费组 %d，Lag %d", overview.ConsumerGroupCount, overview.Lag),
			Metrics: []*InspectionMetricVO{
				{Name: "消费组/订阅", Value: overview.ConsumerGroupCount, Status: "success"},
				{Name: "Lag", Value: overview.Lag, Status: metricRiskStatus(overview.Lag, mqWarningLagThreshold, mqCriticalLagThreshold)},
				{Name: "无消费者资源", Value: overview.NoConsumerResources, Status: metricStatus(overview.NoConsumerResources == 0)},
			},
		},
		{
			Key:     "governance",
			Title:   "治理",
			Status:  inspectionSectionStatus(findings, "governance"),
			Summary: fmt.Sprintf("负责人：%s，业务系统：%s", fallbackDash(instance.Owner), fallbackDash(instance.BusinessSystem)),
			Metrics: []*InspectionMetricVO{
				{Name: "负责人", Value: fallbackDash(instance.Owner), Status: metricStatus(strings.TrimSpace(instance.Owner) != "")},
				{Name: "业务系统", Value: fallbackDash(instance.BusinessSystem), Status: metricStatus(strings.TrimSpace(instance.BusinessSystem) != "")},
				{Name: "DLQ 资源", Value: overview.DLQResources, Status: metricStatus(overview.DLQResources == 0)},
				{Name: "Retry 资源", Value: overview.RetryResources, Status: "info"},
			},
		},
	}
}

func calculateInspectionScore(findings []*InspectionFindingVO) (int, string) {
	score := 100
	critical := 0
	warning := 0
	for _, item := range findings {
		if item == nil {
			continue
		}
		switch item.Severity {
		case "critical":
			score -= 20
			critical++
		case "warning":
			score -= 8
			warning++
		case "info":
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	switch {
	case critical > 0 || score < 60:
		return score, HealthStatusCritical
	case warning > 0 || score < 90:
		return score, HealthStatusWarning
	default:
		return score, HealthStatusHealthy
	}
}

func buildInspectionSummary(score int, riskLevel string, findings []*InspectionFindingVO) string {
	if len(findings) == 0 {
		return fmt.Sprintf("巡检评分 %d，当前未发现明显风险", score)
	}
	return fmt.Sprintf("巡检评分 %d，风险等级 %s，发现 %d 个关注项", score, HealthText(riskLevel), len(findings))
}

func inspectionSectionStatus(findings []*InspectionFindingVO, category string) string {
	status := "success"
	for _, item := range findings {
		if item == nil || item.Category != category {
			continue
		}
		if item.Severity == "critical" {
			return "critical"
		}
		if item.Severity == "warning" {
			status = "warning"
		} else if item.Severity == "info" && status == "success" {
			status = "info"
		}
	}
	return status
}

func metricStatus(ok bool) string {
	if ok {
		return "success"
	}
	return "warning"
}

func metricRiskStatus(value, warningThreshold, criticalThreshold int64) string {
	switch {
	case value >= criticalThreshold:
		return "critical"
	case value >= warningThreshold || value > 0:
		return "warning"
	default:
		return "success"
	}
}

func fallbackDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
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

func toMetricSnapshotVO(item *MQMetricSnapshot) *MetricSnapshotVO {
	if item == nil {
		return nil
	}
	return &MetricSnapshotVO{
		ID:                item.ID,
		InstanceID:        item.InstanceID,
		ResourceID:        item.ResourceID,
		ResourceType:      item.ResourceType,
		ResourceName:      item.ResourceName,
		BrokerCount:       item.BrokerCount,
		OnlineBrokerCount: item.OnlineBrokerCount,
		MessageCount:      item.MessageCount,
		Backlog:           item.Backlog,
		Lag:               item.Lag,
		ProducedRate:      item.ProducedRate,
		ConsumedRate:      item.ConsumedRate,
		ConsumerCount:     item.ConsumerCount,
		CollectedAt:       item.CollectedAt.Format("2006-01-02 15:04:05"),
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
	case AuditActionMetricSnapshot:
		return "指标快照采集"
	case AuditActionInspectionRun:
		return "巡检报告生成"
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
