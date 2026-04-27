package messagequeue

import (
	"context"
	"strings"
	"testing"

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
		wantErr   string
		wantApply bool
	}{
		{name: "disabled", config: &HighRiskOperationConfig{Enabled: false, ReasonRequired: true}, reason: "change window", wantErr: "未开启"},
		{name: "reason required", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, wantErr: "原因不能为空"},
		{name: "allowed", config: &HighRiskOperationConfig{Enabled: true, ReasonRequired: true}, reason: "change window", wantApply: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &highRiskStubAdapter{}
			uc := NewUseCase(
				highRiskInstanceRepo{item: &MQInstance{Model: gorm.Model{ID: 1}, Name: "rabbit", MQType: MQTypeRabbitMQ, Status: InstanceStatusEnabled}},
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
