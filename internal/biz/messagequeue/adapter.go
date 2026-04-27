package messagequeue

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Adapter interface {
	Type() string
	TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error)
	DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error)
	SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error)
}

type ResourceOperationAdapter interface {
	ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error)
	ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error)
}

type AdapterRegistry struct {
	adapters map[string]Adapter
}

func NewAdapterRegistry(adapters ...Adapter) *AdapterRegistry {
	result := &AdapterRegistry{adapters: make(map[string]Adapter)}
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		result.adapters[NormalizeType(adapter.Type())] = adapter
	}
	return result
}

func NewDefaultAdapterRegistry() *AdapterRegistry {
	return NewAdapterRegistry(
		NewRabbitMQAdapter(),
		NewKafkaAdapter(),
		NewRocketMQAdapter(),
		NewActiveMQAdapter(),
		NewPulsarAdapter(),
	)
}

func (r *AdapterRegistry) Get(mqType string) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[NormalizeType(mqType)]
	return adapter, ok
}

type MQConnectionTestResult struct {
	Version         string
	Engine          string
	ClusterName     string
	BrokerCount     int
	ManagementReady bool
	Message         string
}

type MQMetadataSnapshot struct {
	Version        string
	Engine         string
	ClusterName    string
	HealthStatus   string
	Message        string
	Brokers        []*MQBroker
	Resources      []*MQResource
	Bindings       []*MQBinding
	ConsumerGroups []*MQConsumerGroup
	Partitions     []*MQPartition
	SyncedAt       time.Time
}

func adapterNotSupported(mqType, capability string) error {
	return fmt.Errorf("%s %s 能力将在后续批次接入", TypeText(mqType), capability)
}

func splitEndpoints(endpoint string) []string {
	parts := strings.Split(endpoint, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
