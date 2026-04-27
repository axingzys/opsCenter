package messagequeue

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestRabbitMQOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ}
	req := &ResourceOperationRequest{
		Action:       OperationActionRabbitMQBindingUpsert,
		Namespace:    "/",
		ResourceName: "target.queue",
		Params: map[string]any{
			"source":          "amq.direct",
			"destinationType": "queue",
			"routingKey":      "demo.key",
		},
	}
	result, err := NewRabbitMQAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate binding: %v", err)
	}
	if !result.Supported || result.RiskLevel != RiskLevelMedium {
		t.Fatalf("unexpected validation: %#v", result)
	}
	if req.ResourceType != ResourceTypeBinding || req.ResourceName != "target.queue" {
		t.Fatalf("request not normalized: %#v", req)
	}
	if result.NormalizedParams["source"] != "amq.direct" {
		t.Fatalf("missing normalized source: %#v", result.NormalizedParams)
	}
}

func TestRabbitMQHighRiskOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ}
	req := &ResourceOperationRequest{
		Action:       OperationActionRabbitMQQueuePurge,
		Namespace:    "/",
		ResourceName: "orders.dead",
	}
	result, err := NewRabbitMQAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate purge: %v", err)
	}
	if result.RiskLevel != RiskLevelHigh || !result.RequiresHighRiskAck || result.RequiredPermission != PermissionHighRisk {
		t.Fatalf("unexpected high risk validation: %#v", result)
	}
	if req.ResourceType != ResourceTypeQueue {
		t.Fatalf("request not normalized: %#v", req)
	}
}

func TestKafkaOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 2}, Name: "kafka", MQType: MQTypeKafka}
	req := &ResourceOperationRequest{
		Action:       OperationActionKafkaTopicConfigUpdate,
		ResourceName: "orders",
		Params: map[string]any{
			"configs": map[string]any{"retention.ms": "604800000"},
		},
	}
	result, err := NewKafkaAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate kafka config: %v", err)
	}
	if result.RiskLevel != RiskLevelMedium || result.ResourceType != ResourceTypeTopic {
		t.Fatalf("unexpected validation: %#v", result)
	}
	configs, ok := result.NormalizedParams["configs"].(map[string]string)
	if !ok || configs["retention.ms"] != "604800000" {
		t.Fatalf("configs not normalized: %#v", result.NormalizedParams)
	}
}

func TestKafkaOperationValidationRejectsConfigOutsideAllowList(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 2}, Name: "kafka", MQType: MQTypeKafka}
	req := &ResourceOperationRequest{
		Action:       OperationActionKafkaTopicConfigUpdate,
		ResourceName: "orders",
		Params: map[string]any{
			"configs": map[string]any{"unclean.leader.election.enable": "true"},
		},
	}
	_, err := NewKafkaAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err == nil || !strings.Contains(err.Error(), "白名单") {
		t.Fatalf("expected allow list error, got %v", err)
	}
}

func TestKafkaHighRiskOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 2}, Name: "kafka", MQType: MQTypeKafka}
	req := &ResourceOperationRequest{
		Action:       OperationActionKafkaTopicDelete,
		ResourceName: "orders",
	}
	result, err := NewKafkaAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate kafka delete: %v", err)
	}
	if result.RiskLevel != RiskLevelHigh || result.RequiredPermission != PermissionHighRisk {
		t.Fatalf("unexpected delete validation: %#v", result)
	}
}

func TestPulsarOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 3}, Name: "pulsar", MQType: MQTypePulsar}
	req := &ResourceOperationRequest{
		Action:       OperationActionPulsarRetentionUpdate,
		ResourceName: "public/default",
		Params: map[string]any{
			"retentionTimeInMinutes": 1440,
			"retentionSizeInMB":      1024,
		},
	}
	result, err := NewPulsarAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate pulsar retention: %v", err)
	}
	if result.ResourceType != ResourceTypeNamespace || result.Namespace != "public" || result.ResourceName != "public/default" {
		t.Fatalf("namespace not normalized: %#v", result)
	}
	if result.RiskLevel != RiskLevelMedium {
		t.Fatalf("unexpected risk: %#v", result)
	}
}

func TestPulsarHighRiskOperationValidation(t *testing.T) {
	instance := &MQInstance{Model: gorm.Model{ID: 3}, Name: "pulsar", MQType: MQTypePulsar}
	req := &ResourceOperationRequest{
		Action: OperationActionPulsarSubscriptionReset,
		Params: map[string]any{
			"topic":        "public/default/orders",
			"subscription": "orders-sub",
			"timestampMs":  1710000000000,
		},
	}
	result, err := NewPulsarAdapter().ValidateOperation(context.Background(), instance, nil, req)
	if err != nil {
		t.Fatalf("validate pulsar reset: %v", err)
	}
	if result.RiskLevel != RiskLevelCritical || result.ResourceName != "orders-sub" || result.RequiredPermission != PermissionHighRisk {
		t.Fatalf("unexpected reset validation: %#v", result)
	}
	if topic := result.NormalizedParams["topic"]; topic != "persistent://public/default/orders" {
		t.Fatalf("topic not normalized: %#v", result.NormalizedParams)
	}
}
