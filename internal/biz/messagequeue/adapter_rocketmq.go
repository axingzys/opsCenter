package messagequeue

import (
	"context"
	"fmt"
	"time"
)

type RocketMQAdapter struct{}

func NewRocketMQAdapter() *RocketMQAdapter {
	return &RocketMQAdapter{}
}

func (a *RocketMQAdapter) Type() string {
	return MQTypeRocketMQ
}

func (a *RocketMQAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	address := firstEndpoint(instance, DefaultPort(MQTypeRocketMQ))
	if address == "" {
		return nil, fmt.Errorf("RocketMQ NameServer地址不能为空")
	}
	if err := tcpProbe(ctx, address, 5*time.Second); err != nil {
		return nil, fmt.Errorf("连接RocketMQ NameServer失败: %w", err)
	}
	return &MQConnectionTestResult{
		Version:         "RocketMQ",
		Engine:          MQTypeRocketMQ,
		BrokerCount:     0,
		ManagementReady: false,
		Message:         "RocketMQ NameServer端口可连接，完整元数据将在后续适配器增强",
	}, nil
}

func (a *RocketMQAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	address := firstEndpoint(instance, DefaultPort(MQTypeRocketMQ))
	if address == "" {
		return nil, fmt.Errorf("RocketMQ NameServer地址不能为空")
	}
	if err := tcpProbe(ctx, address, 5*time.Second); err != nil {
		return nil, fmt.Errorf("连接RocketMQ NameServer失败: %w", err)
	}
	now := time.Now()
	return &MQMetadataSnapshot{
		Version:      "RocketMQ",
		Engine:       MQTypeRocketMQ,
		HealthStatus: HealthStatusHealthy,
		Message:      "RocketMQ NameServer连通性同步成功，Topic/Group元数据将在后续增强",
		Brokers: []*MQBroker{
			{
				InstanceID: instance.ID,
				BrokerName: "nameserver",
				BrokerID:   address,
				Host:       address,
				Role:       "nameserver",
				Status:     "online",
				Version:    "RocketMQ",
				LastSyncAt: &now,
			},
		},
		SyncedAt: now,
	}, nil
}

func (a *RocketMQAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, adapterNotSupported(instance.MQType, "消息采样")
}
