package messagequeue

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

type productionInstanceRepo struct {
	items []*MQInstance
}

func (r productionInstanceRepo) Create(ctx context.Context, item *MQInstance) error { return nil }
func (r productionInstanceRepo) Update(ctx context.Context, item *MQInstance) error { return nil }
func (r productionInstanceRepo) Delete(ctx context.Context, id uint) error          { return nil }
func (r productionInstanceRepo) GetByID(ctx context.Context, id uint) (*MQInstance, error) {
	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (r productionInstanceRepo) List(ctx context.Context, req *InstanceListRequest) ([]*MQInstance, int64, error) {
	result := make([]*MQInstance, 0, len(r.items))
	for _, item := range r.items {
		if req != nil && req.RestrictToAllowed && !containsUint(req.AllowedIDs, item.ID) {
			continue
		}
		result = append(result, item)
	}
	return result, int64(len(result)), nil
}

type productionResourceRepo struct {
	items map[uint][]*MQResource
}

func (r productionResourceRepo) List(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*MQResource, int64, error) {
	items := r.items[instanceID]
	return items, int64(len(items)), nil
}
func (r productionResourceRepo) GetByUnique(ctx context.Context, instanceID uint, resourceType, namespace, name string) (*MQResource, error) {
	return nil, nil
}
func (r productionResourceRepo) TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*MQResource, error) {
	items := r.items[instanceID]
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
func (r productionResourceRepo) Summary(ctx context.Context, instanceID uint) (*ResourceSummary, error) {
	summary := &ResourceSummary{}
	for _, item := range r.items[instanceID] {
		summary.Count++
		summary.Backlog += item.Backlog
		summary.MessageCount += item.MessageCount
		if isQueueOrTopic(item.ResourceType) && item.ConsumerCount == 0 {
			summary.NoConsumerResourceCount++
		}
		if isDLQName(item.Name) {
			summary.DLQResourceCount++
		}
		if isRetryName(item.Name) {
			summary.RetryResourceCount++
		}
	}
	return summary, nil
}

type productionConsumerGroupRepo struct {
	items map[uint][]*MQConsumerGroup
}

func (r productionConsumerGroupRepo) List(ctx context.Context, instanceID uint, req *ConsumerGroupListRequest) ([]*MQConsumerGroup, int64, error) {
	items := r.items[instanceID]
	return items, int64(len(items)), nil
}
func (r productionConsumerGroupRepo) GetByUnique(ctx context.Context, instanceID uint, namespace, resourceName, groupName string) (*MQConsumerGroup, error) {
	return nil, nil
}
func (r productionConsumerGroupRepo) Summary(ctx context.Context, instanceID uint) (*ConsumerGroupSummary, error) {
	summary := &ConsumerGroupSummary{}
	for _, item := range r.items[instanceID] {
		summary.Count++
		summary.Lag += item.Lag
		summary.Backlog += item.Backlog
	}
	return summary, nil
}

type productionBindingRepo struct {
	items []*MQBinding
}

func (r productionBindingRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*MQBinding, error) {
	return r.items, nil
}

func TestProductionDashboardAggregatesGovernanceSignals(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "kafka-prod", MQType: MQTypeKafka, Status: InstanceStatusEnabled, HealthStatus: HealthStatusWarning, Environment: "prod", LastSyncAt: &now, LastMetricAt: &now}}},
		nil, nil,
		productionResourceRepo{items: map[uint][]*MQResource{1: {{Model: gorm.Model{ID: 11}, InstanceID: 1, ResourceType: ResourceTypeTopic, Name: "orders", Backlog: 2000, ConsumerCount: 0, ReplicaCount: 1, PartitionCount: 3}}}},
		nil,
		productionConsumerGroupRepo{items: map[uint][]*MQConsumerGroup{1: {{Model: gorm.Model{ID: 21}, InstanceID: 1, GroupName: "orders-cg", ResourceName: "orders", Lag: 3000, ActiveConsumerCount: 0}}}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	dashboard, err := uc.GetProductionDashboard(context.Background(), &GovernanceReportRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dashboard.InstanceTotal != 1 || dashboard.BacklogTotal != 2000 || dashboard.LagTotal != 3000 {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
	if len(dashboard.AlertCandidates) == 0 {
		t.Fatalf("expected alert candidates")
	}
}

func TestTopologyBuildsBindingAndConsumerEdges(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		productionInstanceRepo{items: []*MQInstance{{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ, Status: InstanceStatusEnabled, HealthStatus: HealthStatusHealthy, LastSyncAt: &now, LastMetricAt: &now}}},
		nil, nil,
		productionResourceRepo{items: map[uint][]*MQResource{1: {
			{Model: gorm.Model{ID: 11}, InstanceID: 1, ResourceType: ResourceTypeExchange, Namespace: "/", Name: "orders.ex"},
			{Model: gorm.Model{ID: 12}, InstanceID: 1, ResourceType: ResourceTypeQueue, Namespace: "/", Name: "orders.q", ConsumerCount: 1},
		}}},
		productionBindingRepo{items: []*MQBinding{{InstanceID: 1, VHost: "/", Source: "orders.ex", Destination: "orders.q", DestinationType: ResourceTypeQueue, RoutingKey: "orders.*"}}},
		productionConsumerGroupRepo{items: map[uint][]*MQConsumerGroup{1: {{Model: gorm.Model{ID: 21}, InstanceID: 1, GroupName: "worker", ResourceName: "orders.q", Namespace: "/"}}}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	topology, err := uc.GetTopology(context.Background(), 1)
	if err != nil {
		t.Fatalf("topology: %v", err)
	}
	hasBinding := false
	hasConsume := false
	for _, edge := range topology.Edges {
		if edge.Type == ResourceTypeBinding {
			hasBinding = true
		}
		if edge.Type == "consume" {
			hasConsume = true
		}
	}
	if !hasBinding || !hasConsume {
		t.Fatalf("expected binding and consume edges: %#v", topology.Edges)
	}
}
