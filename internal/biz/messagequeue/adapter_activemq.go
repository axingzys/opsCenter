package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ActiveMQAdapter struct{}

func NewActiveMQAdapter() *ActiveMQAdapter {
	return &ActiveMQAdapter{}
}

func (a *ActiveMQAdapter) Type() string {
	return MQTypeActiveMQ
}

func (a *ActiveMQAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypeActiveMQ), instance.TLSEnabled)
	var version map[string]any
	err := doJSONRequest(ctx, httpClient(instance.TLSEnabled), http.MethodGet, baseURL+"/api/jolokia/version", credential, &version)
	if err == nil {
		return &MQConnectionTestResult{
			Version:         stringValue(version["agent"]),
			Engine:          MQTypeActiveMQ,
			BrokerCount:     1,
			ManagementReady: true,
			Message:         "ActiveMQ Jolokia连接成功",
		}, nil
	}
	address := firstEndpoint(instance, DefaultPort(MQTypeActiveMQ))
	if address == "" {
		return nil, err
	}
	if tcpErr := tcpProbe(ctx, address, 5*time.Second); tcpErr != nil {
		return nil, fmt.Errorf("管理接口和Broker端口均不可用: %v; %v", err, tcpErr)
	}
	return &MQConnectionTestResult{
		Version:         "ActiveMQ",
		Engine:          MQTypeActiveMQ,
		BrokerCount:     1,
		ManagementReady: false,
		Message:         "ActiveMQ Broker端口可连接，Jolokia不可用或未授权",
	}, nil
}

func (a *ActiveMQAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypeActiveMQ), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	now := time.Now()

	var brokerRead map[string]any
	if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/jolokia/read/org.apache.activemq:type=Broker,brokerName=*", credential, &brokerRead); err != nil {
		address := firstEndpoint(instance, DefaultPort(MQTypeActiveMQ))
		if address == "" {
			return nil, err
		}
		if tcpErr := tcpProbe(ctx, address, 5*time.Second); tcpErr != nil {
			return nil, fmt.Errorf("ActiveMQ元数据同步失败: %v; %v", err, tcpErr)
		}
		return &MQMetadataSnapshot{
			Version:      "ActiveMQ",
			Engine:       MQTypeActiveMQ,
			HealthStatus: HealthStatusHealthy,
			Message:      "ActiveMQ Broker端口可连接，Jolokia元数据不可用",
			Brokers: []*MQBroker{
				{
					InstanceID: instance.ID,
					BrokerName: "broker",
					BrokerID:   address,
					Host:       address,
					Role:       "broker",
					Status:     "online",
					Version:    "ActiveMQ",
					LastSyncAt: &now,
				},
			},
			SyncedAt: now,
		}, nil
	}

	snapshot := &MQMetadataSnapshot{
		Version:      "ActiveMQ",
		Engine:       MQTypeActiveMQ,
		HealthStatus: HealthStatusHealthy,
		Message:      "ActiveMQ Jolokia元数据同步成功",
		SyncedAt:     now,
	}
	values, _ := brokerRead["value"].(map[string]any)
	for objectName, raw := range values {
		item, _ := raw.(map[string]any)
		brokerName := stringValue(item["BrokerName"])
		if brokerName == "" {
			brokerName = objectName
		}
		snapshot.Brokers = append(snapshot.Brokers, &MQBroker{
			InstanceID:   instance.ID,
			BrokerName:   brokerName,
			BrokerID:     brokerName,
			Host:         firstEndpoint(instance, DefaultPort(MQTypeActiveMQ)),
			Role:         "broker",
			Status:       "online",
			Version:      stringValue(item["BrokerVersion"]),
			MetadataJSON: mustJSON(item),
			LastSyncAt:   &now,
		})
	}

	for _, destinationType := range []string{"Queue", "Topic"} {
		var search map[string]any
		searchURL := baseURL + "/api/jolokia/search/" + "org.apache.activemq:type=Broker,brokerName=*,destinationType=" + destinationType + ",destinationName=*"
		if err := doJSONRequest(ctx, client, http.MethodGet, searchURL, credential, &search); err != nil {
			continue
		}
		list, _ := search["value"].([]any)
		for _, rawName := range list {
			objectName := stringValue(rawName)
			name := extractJMXValue(objectName, "destinationName")
			if name == "" {
				continue
			}
			resourceType := ResourceTypeQueue
			if destinationType == "Topic" {
				resourceType = ResourceTypeTopic
			}
			snapshot.Resources = append(snapshot.Resources, &MQResource{
				InstanceID:   instance.ID,
				ResourceType: resourceType,
				Name:         name,
				FullName:     name,
				MetadataJSON: mustJSON(map[string]any{"objectName": objectName}),
				LastSyncAt:   &now,
			})
		}
	}

	return snapshot, nil
}

func (a *ActiveMQAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, adapterNotSupported(instance.MQType, "消息采样")
}

func extractJMXValue(objectName, key string) string {
	prefix := key + "="
	parts := strings.Split(objectName, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}
