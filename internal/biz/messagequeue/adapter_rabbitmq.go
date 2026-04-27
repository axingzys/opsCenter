package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type RabbitMQAdapter struct{}

func NewRabbitMQAdapter() *RabbitMQAdapter {
	return &RabbitMQAdapter{}
}

func (a *RabbitMQAdapter) Type() string {
	return MQTypeRabbitMQ
}

func (a *RabbitMQAdapter) TestConnection(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQConnectionTestResult, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypeRabbitMQ), instance.TLSEnabled)
	var overview map[string]any
	if err := doJSONRequest(ctx, httpClient(instance.TLSEnabled), http.MethodGet, baseURL+"/api/overview", credential, &overview); err != nil {
		address := firstEndpoint(instance, DefaultPort(MQTypeRabbitMQ))
		if address == "" {
			return nil, err
		}
		if tcpErr := tcpProbe(ctx, address, 5*time.Second); tcpErr != nil {
			return nil, fmt.Errorf("管理接口和AMQP端口均不可用: %v; %v", err, tcpErr)
		}
		return &MQConnectionTestResult{
			Engine:          MQTypeRabbitMQ,
			ManagementReady: false,
			BrokerCount:     1,
			Message:         "AMQP端口可连接，Management API不可用或未授权",
		}, nil
	}

	brokerCount := 0
	if listeners, ok := overview["listeners"].([]any); ok {
		brokerCount = len(listeners)
	}
	return &MQConnectionTestResult{
		Version:         stringValue(overview["rabbitmq_version"]),
		Engine:          MQTypeRabbitMQ,
		ClusterName:     stringValue(overview["cluster_name"]),
		BrokerCount:     brokerCount,
		ManagementReady: true,
		Message:         "RabbitMQ Management API 连接成功",
	}, nil
}

func (a *RabbitMQAdapter) DiscoverMetadata(ctx context.Context, instance *MQInstance, credential *ConnectionCredential) (*MQMetadataSnapshot, error) {
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypeRabbitMQ), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	now := time.Now()

	var overview map[string]any
	if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/overview", credential, &overview); err != nil {
		return nil, err
	}

	var nodes []map[string]any
	if err := doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/nodes", credential, &nodes); err != nil {
		return nil, err
	}

	var vhosts []map[string]any
	_ = doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/vhosts", credential, &vhosts)

	var exchanges []map[string]any
	_ = doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/exchanges", credential, &exchanges)

	var queues []map[string]any
	_ = doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/queues", credential, &queues)

	var bindings []map[string]any
	_ = doJSONRequest(ctx, client, http.MethodGet, baseURL+"/api/bindings", credential, &bindings)

	snapshot := &MQMetadataSnapshot{
		Version:      stringValue(overview["rabbitmq_version"]),
		Engine:       MQTypeRabbitMQ,
		ClusterName:  stringValue(overview["cluster_name"]),
		HealthStatus: HealthStatusHealthy,
		Message:      "RabbitMQ 元数据同步成功",
		SyncedAt:     now,
	}

	for _, node := range nodes {
		name := stringValue(node["name"])
		snapshot.Brokers = append(snapshot.Brokers, &MQBroker{
			InstanceID:   instance.ID,
			BrokerName:   name,
			BrokerID:     name,
			Host:         stringValue(node["hostname"]),
			Role:         "node",
			Status:       rabbitNodeStatus(node),
			Version:      stringValue(node["rabbitmq_version"]),
			MetadataJSON: mustJSON(node),
			LastSyncAt:   &now,
		})
	}

	for _, vhost := range vhosts {
		name := stringValue(vhost["name"])
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:   instance.ID,
			ResourceType: ResourceTypeVHost,
			Namespace:    name,
			Name:         name,
			FullName:     name,
			MetadataJSON: mustJSON(vhost),
			LastSyncAt:   &now,
		})
	}

	for _, exchange := range exchanges {
		name := stringValue(exchange["name"])
		vhost := stringValue(exchange["vhost"])
		if name == "" {
			name = "(default)"
		}
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:   instance.ID,
			ResourceType: ResourceTypeExchange,
			Namespace:    vhost,
			Name:         name,
			FullName:     vhost + "/" + name,
			Durable:      boolValue(exchange["durable"]),
			ConfigJSON:   mustJSON(map[string]any{"type": exchange["type"], "auto_delete": exchange["auto_delete"], "internal": exchange["internal"]}),
			MetadataJSON: mustJSON(exchange),
			LastSyncAt:   &now,
		})
	}

	for _, queue := range queues {
		name := stringValue(queue["name"])
		vhost := stringValue(queue["vhost"])
		messageCount := int64Value(queue["messages"])
		backlog := int64Value(queue["messages_ready"]) + int64Value(queue["messages_unacknowledged"])
		stats, _ := queue["message_stats"].(map[string]any)
		snapshot.Resources = append(snapshot.Resources, &MQResource{
			InstanceID:    instance.ID,
			ResourceType:  ResourceTypeQueue,
			Namespace:     vhost,
			Name:          name,
			FullName:      vhost + "/" + name,
			Durable:       boolValue(queue["durable"]),
			MessageCount:  messageCount,
			Backlog:       backlog,
			ProducedRate:  nestedRate(stats, "publish_details"),
			ConsumedRate:  nestedRate(stats, "deliver_get_details"),
			ConsumerCount: intValue(queue["consumers"]),
			ConfigJSON:    mustJSON(map[string]any{"type": queue["type"], "auto_delete": queue["auto_delete"], "exclusive": queue["exclusive"]}),
			MetadataJSON:  mustJSON(queue),
			LastSyncAt:    &now,
		})
	}

	for _, binding := range bindings {
		snapshot.Bindings = append(snapshot.Bindings, &MQBinding{
			InstanceID:      instance.ID,
			VHost:           stringValue(binding["vhost"]),
			Source:          stringValue(binding["source"]),
			Destination:     stringValue(binding["destination"]),
			DestinationType: stringValue(binding["destination_type"]),
			RoutingKey:      stringValue(binding["routing_key"]),
			ArgumentsJSON:   mustJSON(binding["arguments"]),
			LastSyncAt:      &now,
		})
	}

	return snapshot, nil
}

func (a *RabbitMQAdapter) SampleMessages(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *MessageSampleRequest) (*MessageSampleResultVO, error) {
	return nil, adapterNotSupported(instance.MQType, "消息采样")
}

func rabbitNodeStatus(node map[string]any) string {
	if boolValue(node["running"]) {
		return "running"
	}
	return "stopped"
}

func boolValue(v any) bool {
	switch item := v.(type) {
	case bool:
		return item
	case string:
		return item == "true" || item == "1"
	default:
		return false
	}
}

func nestedRate(parent map[string]any, key string) float64 {
	if parent == nil {
		return 0
	}
	child, _ := parent[key].(map[string]any)
	return floatValue(child["rate"])
}

func escapePath(value string) string {
	return url.PathEscape(value)
}
