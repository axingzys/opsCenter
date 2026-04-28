package messagequeue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type advancedResourceRepo struct {
	items map[uint][]*MQResource
}

func (r advancedResourceRepo) List(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*MQResource, int64, error) {
	items := r.items[instanceID]
	return items, int64(len(items)), nil
}

func (r advancedResourceRepo) GetByUnique(ctx context.Context, instanceID uint, resourceType, namespace, name string) (*MQResource, error) {
	for _, item := range r.items[instanceID] {
		if item.ResourceType == resourceType && item.Namespace == namespace && item.Name == name {
			return item, nil
		}
	}
	return nil, nil
}

func (r advancedResourceRepo) TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*MQResource, error) {
	items := r.items[instanceID]
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r advancedResourceRepo) Summary(ctx context.Context, instanceID uint) (*ResourceSummary, error) {
	summary := &ResourceSummary{}
	for _, item := range r.items[instanceID] {
		summary.Count++
		summary.Backlog += item.Backlog
		summary.MessageCount += item.MessageCount
	}
	return summary, nil
}

type advancedMetricRepo struct {
	items []*MQMetricSnapshot
}

func (r advancedMetricRepo) Create(ctx context.Context, item *MQMetricSnapshot) error {
	return nil
}

func (r advancedMetricRepo) List(ctx context.Context, instanceID uint, req *MetricSnapshotListRequest) ([]*MQMetricSnapshot, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r advancedMetricRepo) Latest(ctx context.Context, instanceID uint) (*MQMetricSnapshot, error) {
	if len(r.items) == 0 {
		return nil, nil
	}
	return r.items[0], nil
}

type advancedOperationAuditRepo struct {
	items []*MQOperationAudit
}

func (r *advancedOperationAuditRepo) Create(ctx context.Context, item *MQOperationAudit) error {
	item.ID = uint(len(r.items) + 1)
	r.items = append(r.items, item)
	return nil
}

func (r *advancedOperationAuditRepo) Update(ctx context.Context, item *MQOperationAudit) error {
	return nil
}

func (r *advancedOperationAuditRepo) GetByIdempotencyKey(ctx context.Context, instanceID uint, idempotencyKey string) (*MQOperationAudit, error) {
	return nil, nil
}

func (r *advancedOperationAuditRepo) List(ctx context.Context, req *AuditListRequest) ([]*MQOperationAudit, int64, error) {
	result := make([]*MQOperationAudit, len(r.items))
	copy(result, r.items)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, int64(len(result)), nil
}

type configCloneApplyAdapter struct {
	applied *ResourceOperationRequest
}

func (a *configCloneApplyAdapter) Type() string {
	return MQTypeKafka
}

func (a *configCloneApplyAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	return &MQConnectionTestResult{Message: "ok"}, nil
}

func (a *configCloneApplyAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	return &MQMetadataSnapshot{HealthStatus: HealthStatusHealthy, SyncedAt: time.Now()}, nil
}

func (a *configCloneApplyAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, adapterNotSupported(instance.MQType, "消息采样")
}

func (a *configCloneApplyAdapter) ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "name"))
	validation.Message = "stub validation"
	validation.Impacts = []string{"Topic: " + validation.ResourceName}
	setNormalizedParams(req, validation, cloneParams(req.Params))
	return validation, nil
}

func (a *configCloneApplyAdapter) ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error) {
	a.applied = &ResourceOperationRequest{
		Action:         req.Action,
		ResourceType:   req.ResourceType,
		Namespace:      req.Namespace,
		ResourceName:   req.ResourceName,
		Reason:         req.Reason,
		ConfirmText:    req.ConfirmText,
		Confirmed:      req.Confirmed,
		IdempotencyKey: req.IdempotencyKey,
		Params:         cloneParams(req.Params),
	}
	return &ResourceOperationApplyResult{
		ResourceType: ResourceTypeTopic,
		ResourceName: req.ResourceName,
		Message:      "stub applied",
		Result:       map[string]any{"applied": true},
	}, nil
}

func TestInspectMessageSchemaValidatesAndRedacts(t *testing.T) {
	audits := &messageGovernanceAuditRepo{}
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "kafka", MQType: MQTypeKafka}}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, audits, nil, nil, nil, nil,
	)
	result, err := uc.InspectMessageSchema(context.Background(), 1, &MessageSchemaInspectRequest{
		ResourceType: ResourceTypeTopic,
		ResourceName: "orders",
		Payload:      `{"orderId":"A001","phone":"13800138000","password":"secret"}`,
		SchemaJSON:   `{"type":"object","required":["orderId"],"properties":{"orderId":{"type":"string"},"phone":{"type":"string"},"password":{"type":"string"}}}`,
		Strict:       true,
	}, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
	if !result.Valid || result.Format != "json" || result.SensitiveHitCount < 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Contains(result.RedactedPreview, "secret") || strings.Contains(result.RedactedPreview, "13800138000") {
		t.Fatalf("payload not redacted: %s", result.RedactedPreview)
	}
	if len(audits.items) != 1 || audits.items[0].Action != AuditActionMessageSchema {
		t.Fatalf("expected schema audit, got %#v", audits.items)
	}
}

func TestCapacityForecastBuildsRecommendations(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "kafka-prod", MQType: MQTypeKafka, Environment: "prod"}}},
		nil, nil,
		advancedResourceRepo{items: map[uint][]*MQResource{1: {{Model: gorm.Model{ID: 10}, InstanceID: 1, ResourceType: ResourceTypeTopic, Name: "orders", Backlog: 3000}}}},
		nil,
		productionConsumerGroupRepo{items: map[uint][]*MQConsumerGroup{1: {{Model: gorm.Model{ID: 20}, InstanceID: 1, GroupName: "orders-cg", ResourceName: "orders", Lag: 5000}}}},
		nil, nil, nil,
		advancedMetricRepo{items: []*MQMetricSnapshot{
			{InstanceID: 1, ResourceType: "instance", Backlog: 3000, Lag: 5000, CollectedAt: now},
			{InstanceID: 1, ResourceType: "instance", Backlog: 1000, Lag: 2000, CollectedAt: now.Add(-2 * time.Hour)},
		}},
		nil, nil, nil, nil, nil, nil,
	)
	forecast, err := uc.GetCapacityForecast(context.Background(), 1, &CapacityForecastRequest{HorizonHours: 2})
	if err != nil {
		t.Fatalf("forecast: %v", err)
	}
	if forecast.BacklogGrowthPerHour <= 0 || forecast.ProjectedBacklog <= forecast.CurrentBacklog {
		t.Fatalf("unexpected forecast: %#v", forecast)
	}
	if len(forecast.Recommendations) == 0 {
		t.Fatalf("expected recommendations")
	}
}

func TestConfigClonePlanIsExecutableForSameTypeKafkaTopic(t *testing.T) {
	audits := &advancedOperationAuditRepo{}
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{
			{Model: gorm.Model{ID: 1}, Name: "kafka-a", MQType: MQTypeKafka, Environment: "prod", Owner: "ops"},
			{Model: gorm.Model{ID: 2}, Name: "kafka-b", MQType: MQTypeKafka, Environment: "prod"},
		}},
		nil, nil,
		advancedResourceRepo{items: map[uint][]*MQResource{
			1: {{Model: gorm.Model{ID: 10}, InstanceID: 1, ResourceType: ResourceTypeTopic, Name: "orders", PartitionCount: 12, ReplicaCount: 3, ConfigJSON: `{"retention.ms":"604800000"}`}},
			2: {{Model: gorm.Model{ID: 11}, InstanceID: 2, ResourceType: ResourceTypeTopic, Name: "orders", PartitionCount: 6, ReplicaCount: 3, ConfigJSON: `{"retention.ms":"86400000"}`}},
		}},
		nil, nil, nil, nil, nil, nil, audits, nil, nil, nil, nil, nil,
	)
	plan, err := uc.BuildConfigClonePlan(context.Background(), 1, &ConfigClonePlanRequest{
		ResourceType:            ResourceTypeTopic,
		ResourceName:            "orders",
		TargetInstanceID:        2,
		IncludeGovernanceFields: true,
	}, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("clone plan: %v", err)
	}
	if !plan.Executable || plan.Action != OperationActionKafkaTopicConfigUpdate || len(plan.Diff) == 0 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if len(audits.items) != 1 || audits.items[0].Action != AuditActionConfigClone {
		t.Fatalf("expected config clone audit, got %#v", audits.items)
	}
}

func TestConfigCloneApplyExecutesTargetOperation(t *testing.T) {
	now := time.Now()
	audits := &advancedOperationAuditRepo{}
	adapter := &configCloneApplyAdapter{}
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{
			{Model: gorm.Model{ID: 1}, Name: "kafka-a", MQType: MQTypeKafka, Status: InstanceStatusEnabled, LastSyncAt: &now},
			{Model: gorm.Model{ID: 2}, Name: "kafka-b", MQType: MQTypeKafka, Status: InstanceStatusEnabled, LastSyncAt: &now},
		}},
		nil, nil,
		advancedResourceRepo{items: map[uint][]*MQResource{
			1: {{Model: gorm.Model{ID: 10}, InstanceID: 1, ResourceType: ResourceTypeTopic, Name: "orders", PartitionCount: 12, ReplicaCount: 3, ConfigJSON: `{"retention.ms":"604800000","unclean.leader.election.enable":"true"}`}},
			2: {{Model: gorm.Model{ID: 11}, InstanceID: 2, ResourceType: ResourceTypeTopic, Name: "orders-copy", PartitionCount: 6, ReplicaCount: 3, ConfigJSON: `{"retention.ms":"86400000"}`}},
		}},
		nil, nil, nil, nil, nil, nil, audits, nil, nil, nil, nil,
		NewAdapterRegistry(adapter),
	)
	result, err := uc.ApplyConfigClone(context.Background(), 1, &ConfigCloneApplyRequest{
		ResourceType:            ResourceTypeTopic,
		ResourceName:            "orders",
		TargetInstanceID:        2,
		TargetResourceName:      "orders-copy",
		IncludeGovernanceFields: true,
		Reason:                  "clone config for test",
		Confirmed:               true,
		ConfirmText:             "orders-copy",
		IdempotencyKey:          "clone-test-1",
	}, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("apply clone: %v", err)
	}
	if result == nil || !result.Executed || result.Operation == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	if adapter.applied == nil || adapter.applied.Action != OperationActionKafkaTopicConfigUpdate || adapter.applied.ResourceName != "orders-copy" {
		t.Fatalf("target operation not applied: %#v", adapter.applied)
	}
	configs := operationStringMapParam(adapter.applied.Params, "configs")
	if configs["retention.ms"] != "604800000" {
		t.Fatalf("expected retention cloned, got %#v", configs)
	}
	if _, ok := configs["unclean.leader.election.enable"]; ok {
		t.Fatalf("unexpected non-whitelisted kafka config cloned: %#v", configs)
	}
	if len(audits.items) < 2 || audits.items[len(audits.items)-1].Action != AuditActionConfigCloneApply {
		t.Fatalf("expected clone apply audit, got %#v", audits.items)
	}
}

func TestConfigCloneApplyIntegrationRabbitMQ(t *testing.T) {
	if os.Getenv("OPSHUB_MQ_CLONE_INTEGRATION") != "1" {
		t.Skip("set OPSHUB_MQ_CLONE_INTEGRATION=1 to run RabbitMQ config clone integration test")
	}
	targetManagementURL := envString("OPSHUB_TEST_CLONE_TARGET_RABBITMQ_MANAGEMENT_URL", "")
	if strings.TrimSpace(targetManagementURL) == "" {
		t.Skip("set OPSHUB_TEST_CLONE_TARGET_RABBITMQ_MANAGEMENT_URL to run clone integration test")
	}
	noProxy := strings.TrimSpace(os.Getenv("NO_PROXY"))
	for _, host := range []string{"127.0.0.1", "localhost", "192.168.1.30"} {
		if !strings.Contains(noProxy, host) {
			if noProxy != "" {
				noProxy += ","
			}
			noProxy += host
		}
	}
	t.Setenv("NO_PROXY", noProxy)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adapter := NewRabbitMQAdapter()
	credential := &ConnectionCredential{Username: envString("OPSHUB_TEST_RABBITMQ_USERNAME", "guest"), Password: envString("OPSHUB_TEST_RABBITMQ_PASSWORD", "guest")}
	source := &MQInstance{
		Model:         gorm.Model{ID: 1},
		Name:          "rabbit-source",
		MQType:        MQTypeRabbitMQ,
		Status:        InstanceStatusEnabled,
		Endpoint:      envString("OPSHUB_TEST_CLONE_SOURCE_RABBITMQ_ENDPOINT", "127.0.0.1:5672"),
		ManagementURL: envString("OPSHUB_TEST_CLONE_SOURCE_RABBITMQ_MANAGEMENT_URL", "http://127.0.0.1:15672"),
		CredentialID:  1,
	}
	target := &MQInstance{
		Model:         gorm.Model{ID: 2},
		Name:          "rabbit-target",
		MQType:        MQTypeRabbitMQ,
		Status:        InstanceStatusEnabled,
		Endpoint:      envString("OPSHUB_TEST_CLONE_TARGET_RABBITMQ_ENDPOINT", "192.168.1.30:5672"),
		ManagementURL: targetManagementURL,
		CredentialID:  2,
	}
	suffix := time.Now().UnixNano()
	sourceQueue := fmt.Sprintf("opshub.clone.source.%d", suffix)
	targetQueue := fmt.Sprintf("opshub.clone.target.%d", suffix)
	defer func() {
		_, _ = adapter.ApplyOperation(context.Background(), source, credential, &ResourceOperationRequest{Action: OperationActionRabbitMQQueueDelete, Namespace: "/", ResourceName: sourceQueue, Params: map[string]any{"ifUnused": false, "ifEmpty": false}})
		_, _ = adapter.ApplyOperation(context.Background(), target, credential, &ResourceOperationRequest{Action: OperationActionRabbitMQQueueDelete, Namespace: "/", ResourceName: targetQueue, Params: map[string]any{"ifUnused": false, "ifEmpty": false}})
	}()

	_, err := adapter.ApplyOperation(ctx, source, credential, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQQueueUpsert,
		Namespace:    "/",
		ResourceName: sourceQueue,
		Params:       map[string]any{"durable": true, "autoDelete": false, "arguments": map[string]any{"x-message-ttl": 60000}},
	})
	if err != nil {
		t.Fatalf("create source queue: %v", err)
	}
	snapshot, err := adapter.DiscoverMetadata(ctx, source, credential)
	if err != nil {
		t.Fatalf("discover source metadata: %v", err)
	}
	var sourceResource *MQResource
	for _, item := range snapshot.Resources {
		if item.ResourceType == ResourceTypeQueue && item.Name == sourceQueue {
			sourceResource = item
			break
		}
	}
	if sourceResource == nil {
		t.Fatalf("source queue not found in metadata")
	}

	audits := &advancedOperationAuditRepo{}
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{source, target}},
		nil, nil,
		advancedResourceRepo{items: map[uint][]*MQResource{source.ID: {sourceResource}, target.ID: {}}},
		nil, nil, nil, nil, nil, nil, audits, nil, nil,
		func(ctx context.Context, id uint) (*ConnectionCredential, error) { return credential, nil },
		nil,
		NewAdapterRegistry(adapter),
	)
	result, err := uc.ApplyConfigClone(ctx, source.ID, &ConfigCloneApplyRequest{
		ResourceType:       ResourceTypeQueue,
		Namespace:          "/",
		ResourceName:       sourceQueue,
		TargetInstanceID:   target.ID,
		TargetNamespace:    "/",
		TargetResourceName: targetQueue,
		Reason:             "integration clone",
		Confirmed:          true,
		ConfirmText:        targetQueue,
		IdempotencyKey:     fmt.Sprintf("rabbit-clone-%d", suffix),
	}, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("apply config clone: %v", err)
	}
	if result == nil || !result.Executed {
		t.Fatalf("unexpected clone result: %#v", result)
	}
	targetSnapshot, err := adapter.DiscoverMetadata(ctx, target, credential)
	if err != nil {
		t.Fatalf("discover target metadata: %v", err)
	}
	for _, item := range targetSnapshot.Resources {
		if item.ResourceType == ResourceTypeQueue && item.Name == targetQueue {
			metadata := parseJSONMap(item.MetadataJSON)
			args := anyMapFromAny(metadata["arguments"])
			if intValue(args["x-message-ttl"]) != 60000 {
				t.Fatalf("expected cloned queue ttl, metadata=%s", item.MetadataJSON)
			}
			return
		}
	}
	t.Fatalf("target queue not found after clone")
}

func TestVerifyAuditChainDetectsTamper(t *testing.T) {
	first := &MQOperationAudit{Model: gorm.Model{ID: 1}, InstanceID: 1, MQType: MQTypeKafka, Action: AuditActionMetadataSync, Status: AuditStatusSuccess, Message: "ok"}
	first.AuditHash = operationAuditExpectedHash(first)
	second := &MQOperationAudit{Model: gorm.Model{ID: 2}, InstanceID: 1, MQType: MQTypeKafka, Action: AuditActionMetricSnapshot, Status: AuditStatusSuccess, PreviousAuditHash: first.AuditHash, Message: "ok"}
	second.AuditHash = operationAuditExpectedHash(second)
	second.Message = "tampered"
	uc := NewUseCase(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &advancedOperationAuditRepo{items: []*MQOperationAudit{first, second}}, nil, nil, nil, nil, nil)
	result, err := uc.VerifyAuditChain(context.Background(), &AuditChainVerifyRequest{AuditType: "operation", Limit: 10})
	if err != nil {
		t.Fatalf("verify chain: %v", err)
	}
	if result.Valid || result.BrokenCount == 0 {
		t.Fatalf("expected broken audit chain: %#v", result)
	}
}
