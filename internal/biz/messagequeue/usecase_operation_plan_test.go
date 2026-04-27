package messagequeue

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type operationPlanResourceRepo struct {
	item *MQResource
}

func (r operationPlanResourceRepo) List(ctx context.Context, instanceID uint, req *ResourceListRequest) ([]*MQResource, int64, error) {
	return nil, 0, nil
}

func (r operationPlanResourceRepo) GetByUnique(ctx context.Context, instanceID uint, resourceType, namespace, name string) (*MQResource, error) {
	if r.item == nil {
		return nil, nil
	}
	if r.item.InstanceID == instanceID && r.item.ResourceType == resourceType && r.item.Namespace == namespace && r.item.Name == name {
		return r.item, nil
	}
	return nil, nil
}

func (r operationPlanResourceRepo) TopBacklog(ctx context.Context, instanceID uint, limit int) ([]*MQResource, error) {
	return nil, nil
}

func (r operationPlanResourceRepo) Summary(ctx context.Context, instanceID uint) (*ResourceSummary, error) {
	return nil, nil
}

type operationPlanBindingRepo struct {
	items []*MQBinding
}

func (r operationPlanBindingRepo) ListByInstanceID(ctx context.Context, instanceID uint) ([]*MQBinding, error) {
	return r.items, nil
}

type operationPlanConsumerGroupRepo struct {
	item *MQConsumerGroup
}

func (r operationPlanConsumerGroupRepo) List(ctx context.Context, instanceID uint, req *ConsumerGroupListRequest) ([]*MQConsumerGroup, int64, error) {
	return nil, 0, nil
}

func (r operationPlanConsumerGroupRepo) GetByUnique(ctx context.Context, instanceID uint, namespace, resourceName, groupName string) (*MQConsumerGroup, error) {
	if r.item == nil {
		return nil, nil
	}
	if r.item.InstanceID == instanceID && r.item.GroupName == groupName && (resourceName == "" || r.item.ResourceName == resourceName) {
		return r.item, nil
	}
	return nil, nil
}

func (r operationPlanConsumerGroupRepo) Summary(ctx context.Context, instanceID uint) (*ConsumerGroupSummary, error) {
	return nil, nil
}

func TestValidateKafkaConfigDecreaseEscalatesHighRisk(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:      gorm.Model{ID: 1},
			Name:       "kafka",
			MQType:     MQTypeKafka,
			Status:     InstanceStatusEnabled,
			LastSyncAt: &now,
		}},
		nil, nil,
		operationPlanResourceRepo{item: &MQResource{
			Model:        gorm.Model{ID: 10},
			InstanceID:   1,
			ResourceType: ResourceTypeTopic,
			Name:         "orders",
			ConfigJSON:   `{"retention.ms":"604800000","cleanup.policy":"delete"}`,
			LastSyncAt:   &now,
		}},
		nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		nil,
		NewAdapterRegistry(NewKafkaAdapter()),
	)

	validation, err := uc.ValidateResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionKafkaTopicConfigUpdate,
		ResourceName: "orders",
		Params:       map[string]any{"configs": map[string]any{"retention.ms": "86400000"}},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.RiskLevel != RiskLevelHigh || validation.RequiredPermission != PermissionHighRisk || !validation.RequiresHighRiskAck {
		t.Fatalf("risk=%s permission=%d highAck=%v", validation.RiskLevel, validation.RequiredPermission, validation.RequiresHighRiskAck)
	}
	if len(validation.Diff) == 0 {
		t.Fatalf("expected config diff")
	}
	found := false
	for _, item := range validation.Diff {
		if item.Key == "config.retention.ms" && item.Risk == RiskLevelHigh {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected high risk retention.ms diff, got %#v", validation.Diff)
	}
}

func TestValidateRabbitMQExchangeDeleteRequiresForceWhenBound(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:      gorm.Model{ID: 1},
			Name:       "rabbit",
			MQType:     MQTypeRabbitMQ,
			Status:     InstanceStatusEnabled,
			LastSyncAt: &now,
		}},
		nil, nil, nil,
		operationPlanBindingRepo{items: []*MQBinding{{InstanceID: 1, VHost: "/", Source: "orders.ex", Destination: "orders.q", DestinationType: ResourceTypeQueue}}},
		nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		nil,
		NewAdapterRegistry(NewRabbitMQAdapter()),
	)

	validation, err := uc.ValidateResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQExchangeDelete,
		Namespace:    "/",
		ResourceName: "orders.ex",
		Params:       map[string]any{"ifUnused": false},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.Supported {
		t.Fatalf("expected delete exchange with bindings to require force")
	}
	if !strings.Contains(validation.Message, "force=true") {
		t.Fatalf("expected force message, got %q", validation.Message)
	}

	validation, err = uc.ValidateResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQExchangeDelete,
		Namespace:    "/",
		ResourceName: "orders.ex",
		Params:       map[string]any{"ifUnused": false, "force": true},
	})
	if err != nil {
		t.Fatalf("validate force: %v", err)
	}
	if !validation.Supported {
		t.Fatalf("expected force delete supported, got %q", validation.Message)
	}
	if len(validation.Impacts) < 3 {
		t.Fatalf("expected binding impact, got %#v", validation.Impacts)
	}
}

func TestValidatePulsarSubscriptionSkipIncludesBacklogImpact(t *testing.T) {
	now := time.Now()
	topic := "persistent://public/default/orders"
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:      gorm.Model{ID: 1},
			Name:       "pulsar",
			MQType:     MQTypePulsar,
			Status:     InstanceStatusEnabled,
			LastSyncAt: &now,
		}},
		nil, nil, nil, nil,
		operationPlanConsumerGroupRepo{item: &MQConsumerGroup{
			Model:               gorm.Model{ID: 10},
			InstanceID:          1,
			GroupName:           "sub-a",
			ResourceName:        topic,
			State:               "active",
			ConsumerCount:       2,
			ActiveConsumerCount: 1,
			Backlog:             123,
			Lag:                 123,
			LastSyncAt:          &now,
		}},
		nil, nil, nil, nil, nil,
		nil,
		nil,
		nil,
		NewAdapterRegistry(NewPulsarAdapter()),
	)

	validation, err := uc.ValidateResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action: OperationActionPulsarSubscriptionSkip,
		Params: map[string]any{"topic": topic, "subscription": "sub-a"},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.Before["backlog"] != int64(123) {
		t.Fatalf("expected backlog snapshot, got %#v", validation.Before)
	}
	if !containsText(validation.Impacts, "当前 backlog: 123") {
		t.Fatalf("expected backlog impact, got %#v", validation.Impacts)
	}
	if !containsText(validation.Warnings, "在线消费者") {
		t.Fatalf("expected online consumer warning, got %#v", validation.Warnings)
	}
}

func containsText(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func TestValidateRabbitMQQueueImmutableChangeRejected(t *testing.T) {
	now := time.Now()
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:      gorm.Model{ID: 1},
			Name:       "rabbit",
			MQType:     MQTypeRabbitMQ,
			Status:     InstanceStatusEnabled,
			LastSyncAt: &now,
		}},
		nil, nil,
		operationPlanResourceRepo{item: &MQResource{
			Model:        gorm.Model{ID: 10},
			InstanceID:   1,
			ResourceType: ResourceTypeQueue,
			Namespace:    "/",
			Name:         "orders",
			Durable:      true,
			ConfigJSON:   `{"type":"classic","auto_delete":false}`,
			LastSyncAt:   &now,
		}},
		nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		nil,
		NewAdapterRegistry(NewRabbitMQAdapter()),
	)

	validation, err := uc.ValidateResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQQueueUpsert,
		Namespace:    "/",
		ResourceName: "orders",
		Params:       map[string]any{"durable": false, "autoDelete": false, "arguments": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.Supported {
		t.Fatalf("expected unsupported immutable queue change")
	}
	if !strings.Contains(validation.Message, "durable") {
		t.Fatalf("expected durable message, got %q", validation.Message)
	}
}
