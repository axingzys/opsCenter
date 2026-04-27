package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PulsarAdapter struct{}

func NewPulsarAdapter() *PulsarAdapter {
	return &PulsarAdapter{}
}

func (a *PulsarAdapter) Type() string {
	return MQTypePulsar
}

func (a *PulsarAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypePulsar), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	var clusters []string
	if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/admin/v2/clusters", credential, &clusters); err != nil {
		address := firstEndpoint(instance, DefaultPort(MQTypePulsar))
		if address == "" {
			return nil, err
		}
		if tcpErr := tcpProbe(ctx, address, 5*time.Second); tcpErr != nil {
			return nil, fmt.Errorf("管理接口和Broker端口均不可用: %v; %v", err, tcpErr)
		}
		return &MQConnectionTestResult{
			Version:         "Pulsar",
			Engine:          MQTypePulsar,
			BrokerCount:     1,
			ManagementReady: false,
			Message:         "Pulsar Broker端口可连接，Admin API不可用或未授权",
		}, nil
	}
	clusterName := ""
	if len(clusters) > 0 {
		clusterName = clusters[0]
	}
	return &MQConnectionTestResult{
		Version:         "Pulsar",
		Engine:          MQTypePulsar,
		ClusterName:     clusterName,
		BrokerCount:     1,
		ManagementReady: true,
		Message:         "Pulsar Admin API连接成功",
	}, nil
}

func (a *PulsarAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypePulsar), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	now := time.Now()

	var tenants []string
	if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/admin/v2/tenants", credential, &tenants); err != nil {
		return nil, err
	}
	snapshot := &MQMetadataSnapshot{
		Version:      "Pulsar",
		Engine:       MQTypePulsar,
		HealthStatus: HealthStatusHealthy,
		Message:      "Pulsar 元数据同步成功",
		SyncedAt:     now,
		Brokers: []*MQBroker{
			{
				InstanceID: instance.ID,
				BrokerName: "pulsar-admin",
				BrokerID:   managementBaseURL(instance, DefaultManagementPort(MQTypePulsar), instance.TLSEnabled),
				Role:       "admin",
				Status:     "online",
				Version:    "Pulsar",
				LastSyncAt: &now,
			},
		},
	}

	for _, tenant := range limitStrings(tenants, 50) {
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:   instance.ID,
			ResourceType: ResourceTypeTenant,
			Name:         tenant,
			FullName:     tenant,
			LastSyncAt:   &now,
		})
		var namespaces []string
		if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/admin/v2/namespaces/"+escapePath(tenant), credential, &namespaces); err != nil {
			continue
		}
		for _, namespaceFull := range limitStrings(namespaces, 100) {
			namespaceName := namespaceFull
			snapshot.Resources = append(snapshot.Resources, &MQResource{
				InstanceID:   instance.ID,
				ResourceType: ResourceTypeNamespace,
				Namespace:    tenant,
				Name:         namespaceName,
				FullName:     namespaceFull,
				LastSyncAt:   &now,
			})
			parts := strings.Split(namespaceFull, "/")
			if len(parts) != 2 {
				continue
			}
			topicsURL := baseURL + "/admin/v2/persistent/" + escapePath(parts[0]) + "/" + escapePath(parts[1])
			var topics []string
			if err := doJSONRequest(ctx, client, http.MethodGet, topicsURL, credential, &topics); err != nil {
				continue
			}
			for _, topic := range limitStrings(topics, 500) {
				resource := &MQResource{
					InstanceID:   instance.ID,
					ResourceType: ResourceTypeTopic,
					Namespace:    namespaceFull,
					Name:         shortPulsarTopicName(topic),
					FullName:     topic,
					LastSyncAt:   &now,
				}
				statsURL := baseURL + "/admin/v2/" + pulsarTopicPath(topic) + "/stats"
				var stats map[string]any
				if err := doJSONRequest(ctx, client, http.MethodGet, statsURL, credential, &stats); err == nil {
					resource.MessageCount = int64Value(stats["msgInCounter"])
					resource.Backlog = int64Value(stats["backlogSize"])
					resource.ProducedRate = floatValue(stats["msgRateIn"])
					resource.ConsumedRate = floatValue(stats["msgRateOut"])
					resource.MetadataJSON = mustJSON(stats)
					if subscriptions, ok := stats["subscriptions"].(map[string]any); ok {
						for subName, subRaw := range subscriptions {
							sub, _ := subRaw.(map[string]any)
							snapshot.ConsumerGroups = append(snapshot.ConsumerGroups, &MQConsumerGroup{
								InstanceID:          instance.ID,
								GroupName:           subName,
								ResourceName:        topic,
								Namespace:           namespaceFull,
								State:               "active",
								ConsumerCount:       lenAnySlice(sub["consumers"]),
								ActiveConsumerCount: lenAnySlice(sub["consumers"]),
								Backlog:             int64Value(sub["msgBacklog"]),
								MetadataJSON:        mustJSON(sub),
								LastSyncAt:          &now,
							})
						}
					}
				}
				snapshot.Resources = append(snapshot.Resources, resource)
			}
		}
	}
	return snapshot, nil
}

func (a *PulsarAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, adapterNotSupported(instance.MQType, "消息采样")
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func shortPulsarTopicName(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) == 0 {
		return topic
	}
	return parts[len(parts)-1]
}

func pulsarTopicPath(topic string) string {
	topic = strings.TrimPrefix(topic, "persistent://")
	topic = strings.TrimPrefix(topic, "non-persistent://")
	parts := strings.Split(topic, "/")
	if len(parts) != 3 {
		return "persistent/" + topic
	}
	return "persistent/" + escapePath(parts[0]) + "/" + escapePath(parts[1]) + "/" + escapePath(parts[2])
}

func lenAnySlice(v any) int {
	list, _ := v.([]any)
	return len(list)
}
