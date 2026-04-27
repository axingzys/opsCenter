package messagequeue

import (
	"context"
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

func TestConfigClonePlanIsNonExecutable(t *testing.T) {
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
	if plan.Executable || plan.Action != OperationActionKafkaTopicConfigUpdate || len(plan.Diff) == 0 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if len(audits.items) != 1 || audits.items[0].Action != AuditActionConfigClone {
		t.Fatalf("expected config clone audit, got %#v", audits.items)
	}
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
