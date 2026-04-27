package messagequeue

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type highRiskInstanceRepo struct {
	item *MQInstance
}

func (r highRiskInstanceRepo) Create(ctx context.Context, item *MQInstance) error { return nil }
func (r highRiskInstanceRepo) Update(ctx context.Context, item *MQInstance) error { return nil }
func (r highRiskInstanceRepo) Delete(ctx context.Context, id uint) error          { return nil }
func (r highRiskInstanceRepo) GetByID(ctx context.Context, id uint) (*MQInstance, error) {
	return r.item, nil
}
func (r highRiskInstanceRepo) List(ctx context.Context, req *InstanceListRequest) ([]*MQInstance, int64, error) {
	return nil, 0, nil
}

type highRiskStubAdapter struct {
	applied bool
}

func (a *highRiskStubAdapter) Type() string { return MQTypeRabbitMQ }
func (a *highRiskStubAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	return nil, nil
}
func (a *highRiskStubAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	return nil, nil
}
func (a *highRiskStubAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, nil
}
func (a *highRiskStubAdapter) ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	validation.ResourceType = ResourceTypeQueue
	validation.ResourceName = "orders"
	validation.RequiredPermission = PermissionHighRisk
	validation.Message = "high risk"
	return validation, nil
}
func (a *highRiskStubAdapter) ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error) {
	a.applied = true
	return &ResourceOperationApplyResult{
		ResourceType: ResourceTypeQueue,
		ResourceName: "orders",
		Message:      "done",
	}, nil
}

func TestExecuteHighRiskOperationConfigGate(t *testing.T) {
	tests := []struct {
		name      string
		config    *HighRiskOperationConfig
		reason    string
		confirm   string
		wantErr   string
		wantApply bool
	}{
		{name: "disabled", config: &HighRiskOperationConfig{Enabled: false, ReasonRequired: true}, reason: "change window", wantErr: "未开启"},
		{name: "reason required", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, wantErr: "原因不能为空"},
		{name: "confirm text required", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, reason: "change window", wantErr: "资源名确认"},
		{name: "confirm text mismatch", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, reason: "change window", confirm: "wrong", wantErr: "确认不一致"},
		{name: "allowed", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, reason: "change window", confirm: "orders", wantApply: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &highRiskStubAdapter{}
			now := time.Now()
			uc := NewUseCase(
				highRiskInstanceRepo{item: &MQInstance{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ, Status: InstanceStatusEnabled, LastSyncAt: &now, LastMetricAt: &now}},
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				nil,
				nil,
				func(ctx context.Context) (*HighRiskOperationConfig, error) {
					return tt.config, nil
				},
				NewAdapterRegistry(adapter),
			)
			_, err := uc.ExecuteResourceOperation(context.Background(), 1, &ResourceOperationRequest{
				Action:       OperationActionRabbitMQQueueDelete,
				ResourceName: "orders",
				Reason:       tt.reason,
				ConfirmText:  tt.confirm,
				Confirmed:    true,
			}, Operator{ID: 1, Username: "admin"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if adapter.applied != tt.wantApply {
				t.Fatalf("applied=%v, want %v", adapter.applied, tt.wantApply)
			}
		})
	}
}

func TestExecuteHighRiskRequiresFreshMetadata(t *testing.T) {
	adapter := &highRiskStubAdapter{}
	old := time.Now().Add(-2 * time.Hour)
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:        gorm.Model{ID: 1},
			Name:         "rabbit",
			MQType:       MQTypeRabbitMQ,
			Status:       InstanceStatusEnabled,
			LastSyncAt:   &old,
			LastMetricAt: &old,
		}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		func(ctx context.Context) (*HighRiskOperationConfig, error) {
			return &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, nil
		},
		NewAdapterRegistry(adapter),
	)
	_, err := uc.ExecuteResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQQueueDelete,
		ResourceName: "orders",
		Reason:       "change window",
		ConfirmText:  "orders",
		Confirmed:    true,
	}, Operator{ID: 1, Username: "admin"})
	if err == nil || !strings.Contains(err.Error(), "新鲜元数据") {
		t.Fatalf("expected fresh metadata error, got %v", err)
	}
	if adapter.applied {
		t.Fatalf("operation should not be applied with stale metadata")
	}
}

func TestExecuteHighRiskRequiresFreshMetrics(t *testing.T) {
	adapter := &highRiskStubAdapter{}
	now := time.Now()
	old := now.Add(-30 * time.Minute)
	uc := NewUseCase(
		highRiskInstanceRepo{item: &MQInstance{
			Model:        gorm.Model{ID: 1},
			Name:         "rabbit",
			MQType:       MQTypeRabbitMQ,
			Status:       InstanceStatusEnabled,
			LastSyncAt:   &now,
			LastMetricAt: &old,
		}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil,
		nil,
		func(ctx context.Context) (*HighRiskOperationConfig, error) {
			return &HighRiskOperationConfig{Enabled: true, ReasonRequired: true, OperationMaxMetricAge: 10 * time.Minute, RequireFreshMetricHighRisk: true}, nil
		},
		NewAdapterRegistry(adapter),
	)
	_, err := uc.ExecuteResourceOperation(context.Background(), 1, &ResourceOperationRequest{
		Action:       OperationActionRabbitMQQueueDelete,
		ResourceName: "orders",
		Reason:       "change window",
		ConfirmText:  "orders",
		Confirmed:    true,
	}, Operator{ID: 1, Username: "admin"})
	if err == nil || !strings.Contains(err.Error(), "新鲜指标") {
		t.Fatalf("expected fresh metric error, got %v", err)
	}
	if adapter.applied {
		t.Fatalf("operation should not be applied with stale metrics")
	}
}

func TestHighRiskRollbackHintAndBackupPackage(t *testing.T) {
	now := time.Now()
	instance := &MQInstance{
		Model:        gorm.Model{ID: 1},
		Name:         "kafka-prod",
		MQType:       MQTypeKafka,
		Environment:  "prod",
		LastSyncAt:   &now,
		LastMetricAt: &now,
	}
	validation := &ResourceOperationValidationVO{
		Action:       OperationActionKafkaTopicDelete,
		ActionText:   actionText(OperationActionKafkaTopicDelete),
		RiskLevel:    RiskLevelCritical,
		ResourceType: ResourceTypeTopic,
		ResourceName: "orders",
		Before: map[string]any{
			"partitionCount": 12,
			"replicaCount":   3,
			"config":         map[string]any{"retention.ms": "604800000"},
		},
		Impacts: []string{"当前 topic 删除后消息不可恢复"},
	}
	backup := buildOperationBackupPackage(instance, validation)
	if backup["before"] == nil || !strings.Contains(backup["note"].(string), "不承诺恢复") {
		t.Fatalf("unexpected backup package: %#v", backup)
	}
	hint, supported, risk := buildRollbackHint(validation)
	if !supported || risk != RiskLevelCritical {
		t.Fatalf("supported=%v risk=%s hint=%#v", supported, risk, hint)
	}
	if !strings.Contains(hint["message"].(string), "重建 topic") {
		t.Fatalf("unexpected rollback hint: %#v", hint)
	}
}
