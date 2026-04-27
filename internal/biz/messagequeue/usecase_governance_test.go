package messagequeue

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

type governanceInstanceRepo struct {
	item *MQInstance
}

func (r *governanceInstanceRepo) Create(ctx context.Context, item *MQInstance) error { return nil }
func (r *governanceInstanceRepo) Update(ctx context.Context, item *MQInstance) error {
	r.item = item
	return nil
}
func (r *governanceInstanceRepo) Delete(ctx context.Context, id uint) error { return nil }
func (r *governanceInstanceRepo) GetByID(ctx context.Context, id uint) (*MQInstance, error) {
	return r.item, nil
}
func (r *governanceInstanceRepo) List(ctx context.Context, req *InstanceListRequest) ([]*MQInstance, int64, error) {
	return nil, 0, nil
}

type governanceBrokerRepo struct {
	total  int64
	online int64
}

func (r governanceBrokerRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*MQBroker, error) {
	return nil, nil
}
func (r governanceBrokerRepo) CountByInstanceID(ctx context.Context, instanceID uint) (int64, int64, error) {
	return r.total, r.online, nil
}

type governanceResourceRepo struct {
	summary *ResourceSummary
	top     []*MQResource
}

func (r governanceResourceRepo) List(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*MQResource, int64, error) {
	return nil, 0, nil
}
func (r governanceResourceRepo) GetByUnique(ctx context.Context, instanceID uint, resourceType, namespace, name string) (*MQResource, error) {
	return nil, nil
}
func (r governanceResourceRepo) TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*MQResource, error) {
	return r.top, nil
}
func (r governanceResourceRepo) Summary(ctx context.Context, instanceID uint) (*ResourceSummary, error) {
	if r.summary == nil {
		return &ResourceSummary{}, nil
	}
	return r.summary, nil
}

type governanceConsumerGroupRepo struct {
	summary *ConsumerGroupSummary
}

func (r governanceConsumerGroupRepo) List(ctx context.Context, instanceID uint, req *ConsumerGroupListRequest) ([]*MQConsumerGroup, int64, error) {
	return nil, 0, nil
}
func (r governanceConsumerGroupRepo) GetByUnique(ctx context.Context, instanceID uint, namespace, resourceName, groupName string) (*MQConsumerGroup, error) {
	return nil, nil
}
func (r governanceConsumerGroupRepo) Summary(ctx context.Context, instanceID uint) (*ConsumerGroupSummary, error) {
	if r.summary == nil {
		return &ConsumerGroupSummary{}, nil
	}
	return r.summary, nil
}

type governancePartitionRepo struct {
	count int64
}

func (r governancePartitionRepo) List(ctx context.Context, instanceID uint, req *PartitionListRequest) ([]*MQPartition, int64, error) {
	return nil, 0, nil
}
func (r governancePartitionRepo) CountByInstanceID(ctx context.Context, instanceID uint) (int64, error) {
	return r.count, nil
}

type governanceMetricRepo struct {
	items []*MQMetricSnapshot
}

func (r *governanceMetricRepo) Create(ctx context.Context, item *MQMetricSnapshot) error {
	item.ID = uint(len(r.items) + 1)
	r.items = append(r.items, item)
	return nil
}
func (r *governanceMetricRepo) List(ctx context.Context, instanceID uint, req *MetricSnapshotListRequest) ([]*MQMetricSnapshot, int64, error) {
	return r.items, int64(len(r.items)), nil
}
func (r *governanceMetricRepo) Latest(ctx context.Context, instanceID uint) (*MQMetricSnapshot, error) {
	if len(r.items) == 0 {
		return nil, nil
	}
	return r.items[len(r.items)-1], nil
}

func TestCollectMetricSnapshotUpdatesHealthAndAnomalies(t *testing.T) {
	now := time.Now()
	instanceRepo := &governanceInstanceRepo{item: &MQInstance{
		Model:        gorm.Model{ID: 1},
		Name:         "kafka-prod",
		MQType:       MQTypeKafka,
		Status:       InstanceStatusEnabled,
		HealthStatus: HealthStatusHealthy,
		LastSyncAt:   &now,
	}}
	metricRepo := &governanceMetricRepo{}
	uc := NewUseCase(
		instanceRepo,
		nil,
		governanceBrokerRepo{total: 3, online: 2},
		governanceResourceRepo{summary: &ResourceSummary{
			Count:                   5,
			MessageCount:            2000,
			Backlog:                 1500,
			ProducedRate:            20,
			ConsumedRate:            12,
			NoConsumerResourceCount: 1,
			DLQResourceCount:        1,
		}},
		nil,
		governanceConsumerGroupRepo{summary: &ConsumerGroupSummary{Count: 2, Lag: 20}},
		governancePartitionRepo{count: 6},
		nil,
		nil,
		metricRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	result, err := uc.CollectMetricSnapshot(context.Background(), 1, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(metricRepo.items) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(metricRepo.items))
	}
	if result.HealthStatus != HealthStatusWarning {
		t.Fatalf("health=%s, want warning", result.HealthStatus)
	}
	if !containsText(result.AnomalyTags, "堆积增长") || !containsText(result.AnomalyTags, "无消费者") {
		t.Fatalf("unexpected anomaly tags: %#v", result.AnomalyTags)
	}
	if instanceRepo.item.LastMetricAt == nil {
		t.Fatalf("expected last metric time updated")
	}
}

func TestGenerateInspectionReportFindsGovernanceIssues(t *testing.T) {
	instanceRepo := &governanceInstanceRepo{item: &MQInstance{
		Model:        gorm.Model{ID: 1},
		Name:         "rabbit",
		MQType:       MQTypeRabbitMQ,
		Status:       InstanceStatusEnabled,
		HealthStatus: HealthStatusHealthy,
	}}
	uc := NewUseCase(
		instanceRepo,
		nil,
		governanceBrokerRepo{},
		governanceResourceRepo{summary: &ResourceSummary{}},
		nil,
		governanceConsumerGroupRepo{summary: &ConsumerGroupSummary{}},
		governancePartitionRepo{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	report, err := uc.GenerateInspectionReport(context.Background(), 1, Operator{ID: 1, Username: "admin"})
	if err != nil {
		t.Fatalf("inspection: %v", err)
	}
	if report.Score >= 100 {
		t.Fatalf("expected score deduction, got %d", report.Score)
	}
	foundOwner := false
	for _, item := range report.Findings {
		if item.Title == "实例未绑定负责人" {
			foundOwner = true
		}
	}
	if !foundOwner {
		t.Fatalf("expected owner finding, got %#v", report.Findings)
	}
}
