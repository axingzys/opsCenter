package messagequeue

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestListOperationActionsUsesHighRiskSwitch(t *testing.T) {
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{Model: gorm.Model{ID: 1}, Name: "kafka", MQType: MQTypeKafka, Status: InstanceStatusEnabled}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		func(ctx context.Context) (*HighRiskOperationConfig, error) {
			return &HighRiskOperationConfig{Enabled: false, ReasonRequired: true}, nil
		},
		NewAdapterRegistry(NewKafkaAdapter()),
	)
	actions, err := uc.ListOperationActions(context.Background(), 1)
	if err != nil {
		t.Fatalf("actions: %v", err)
	}
	foundDelete := false
	for _, action := range actions {
		if action.Action == OperationActionKafkaTopicDelete {
			foundDelete = true
			if action.Enabled {
				t.Fatalf("delete action should be disabled when high risk switch is off")
			}
			if action.DisabledReason == "" {
				t.Fatalf("expected disabled reason")
			}
		}
	}
	if !foundDelete {
		t.Fatalf("expected kafka delete topic action")
	}
}

func TestGetInstanceCapabilitiesReportsResourceManagement(t *testing.T) {
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ, Status: InstanceStatusEnabled}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		func(ctx context.Context) (*HighRiskOperationConfig, error) {
			return &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, nil
		},
		NewAdapterRegistry(NewRabbitMQAdapter()),
	)
	capabilities, err := uc.GetInstanceCapabilities(context.Background(), 1)
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if len(capabilities.Actions) == 0 {
		t.Fatalf("expected operation actions")
	}
	hasResourceManage := false
	for _, item := range capabilities.Capabilities {
		if item.Key == "resource_manage" && item.Enabled {
			hasResourceManage = true
		}
	}
	if !hasResourceManage {
		t.Fatalf("expected resource management capability: %#v", capabilities.Capabilities)
	}
}
