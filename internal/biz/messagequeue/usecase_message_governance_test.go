package messagequeue

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"
)

type messageGovernanceAuditRepo struct {
	items []*MQMessageAudit
}

func (r *messageGovernanceAuditRepo) Create(ctx context.Context, item *MQMessageAudit) error {
	item.ID = uint(len(r.items) + 1)
	r.items = append(r.items, item)
	return nil
}

func (r *messageGovernanceAuditRepo) List(ctx context.Context, req *AuditListRequest) ([]*MQMessageAudit, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func TestMessageSampleDLPRedactsSensitivePayload(t *testing.T) {
	result := &MessageSampleResultVO{Samples: []MessageSampleVO{{
		Payload:     `{"username":"alice","password":"secret","phone":"13800138000","email":"alice@example.com"}`,
		PayloadSize: 90,
	}}}
	meta := applyMessageSampleDLP(result)
	if !meta.Redacted || meta.SensitiveHitCount < 3 {
		t.Fatalf("expected redaction hits, got %#v", meta)
	}
	payload := result.Samples[0].Payload
	if strings.Contains(payload, "secret") || strings.Contains(payload, "13800138000") || strings.Contains(payload, "alice@example.com") {
		t.Fatalf("payload was not redacted: %s", payload)
	}
	if result.PayloadHash == "" || result.Samples[0].PayloadHash == "" {
		t.Fatalf("expected payload hashes")
	}
}

func TestDLQAnalysisIdentifiesDeadLetterResources(t *testing.T) {
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "rabbit-prod", MQType: MQTypeRabbitMQ, Environment: "prod", Owner: "ops"}}},
		nil, nil,
		productionResourceRepo{items: map[uint][]*MQResource{1: {
			{Model: gorm.Model{ID: 10}, InstanceID: 1, ResourceType: ResourceTypeQueue, Name: "orders.dlq", Backlog: 1200, ConsumerCount: 0},
			{Model: gorm.Model{ID: 11}, InstanceID: 1, ResourceType: ResourceTypeQueue, Name: "orders.normal", Backlog: 0, ConsumerCount: 1},
		}}},
		nil,
		productionConsumerGroupRepo{items: map[uint][]*MQConsumerGroup{1: {{Model: gorm.Model{ID: 20}, InstanceID: 1, GroupName: "dlq-worker", ResourceName: "orders.dlq"}}}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	report, err := uc.GetDLQAnalysis(context.Background(), &DLQAnalysisRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("dlq analysis: %v", err)
	}
	if report.Total != 1 || report.Summary.DLQTotal != 1 || len(report.Items) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Items[0].RelatedResourceName != "orders" || report.Items[0].Severity != "warning" {
		t.Fatalf("unexpected issue: %#v", report.Items[0])
	}
}

func TestPrepareMessageReplayApplicationOnlyCreatesPendingAudit(t *testing.T) {
	audits := &messageGovernanceAuditRepo{}
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "kafka-prod", MQType: MQTypeKafka}}},
		nil, nil,
		productionResourceRepo{items: map[uint][]*MQResource{1: {{Model: gorm.Model{ID: 10}, InstanceID: 1, ResourceType: ResourceTypeTopic, Name: "orders.retry", Backlog: 50}}}},
		nil, nil, nil, nil, nil, nil, nil, audits, nil, nil, nil, nil,
	)
	plan, err := uc.PrepareMessageReplayApplication(context.Background(), 1, &MessageReplayApplicationRequest{
		ResourceType:       ResourceTypeTopic,
		ResourceName:       "orders.retry",
		MaxMessages:        10,
		RateLimitPerSecond: 5,
		Reason:             "业务修复后申请小批量重放",
	}, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("prepare replay: %v", err)
	}
	if plan.ReplayExecutable || !plan.RequiresApproval || plan.Status != AuditStatusPending {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if len(audits.items) != 1 || audits.items[0].Action != AuditActionMessageReplay || audits.items[0].Status != AuditStatusPending {
		t.Fatalf("expected pending audit, got %#v", audits.items)
	}
}
