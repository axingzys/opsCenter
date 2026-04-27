package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func (a *RabbitMQAdapter) ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	normalizeResourceOperationRequest(req)
	switch req.Action {
	case OperationActionRabbitMQQueueUpsert:
		return a.validateQueueUpsert(instance, req)
	case OperationActionRabbitMQExchangeUpsert:
		return a.validateExchangeUpsert(instance, req)
	case OperationActionRabbitMQBindingUpsert:
		return a.validateBindingUpsert(instance, req)
	case OperationActionRabbitMQQueuePurge:
		return a.validateQueuePurge(instance, req)
	case OperationActionRabbitMQQueueDelete:
		return a.validateQueueDelete(instance, req)
	case OperationActionRabbitMQExchangeDelete:
		return a.validateExchangeDelete(instance, req)
	default:
		return unsupportedOperationValidation(instance, req, "RabbitMQ 不支持该资源操作"), nil
	}
}

func (a *RabbitMQAdapter) ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error) {
	validation, err := a.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		return nil, err
	}
	if !validation.Supported {
		return nil, fmt.Errorf("%s", validation.Message)
	}
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypeRabbitMQ), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	vhost := validation.Namespace
	name := validation.ResourceName
	params := validation.NormalizedParams
	switch req.Action {
	case OperationActionRabbitMQQueueUpsert:
		body := map[string]any{
			"durable":     operationBoolParam(params, true, "durable"),
			"auto_delete": operationBoolParam(params, false, "autoDelete", "auto_delete"),
			"arguments":   operationAnyMapParam(params, "arguments"),
		}
		target := baseURL + "/api/queues/" + escapePath(vhost) + "/" + escapePath(name)
		if err := doJSONRequestWithBody(ctx, client, http.MethodPut, target, credential, body, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeQueue,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Queue 创建/更新已提交",
			Result:       map[string]any{"vhost": vhost, "queue": name, "body": body},
		}, nil
	case OperationActionRabbitMQExchangeUpsert:
		body := map[string]any{
			"type":        operationStringParam(params, "type"),
			"durable":     operationBoolParam(params, true, "durable"),
			"auto_delete": operationBoolParam(params, false, "autoDelete", "auto_delete"),
			"internal":    operationBoolParam(params, false, "internal"),
			"arguments":   operationAnyMapParam(params, "arguments"),
		}
		target := baseURL + "/api/exchanges/" + escapePath(vhost) + "/" + escapePath(name)
		if err := doJSONRequestWithBody(ctx, client, http.MethodPut, target, credential, body, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeExchange,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Exchange 创建/更新已提交",
			Result:       map[string]any{"vhost": vhost, "exchange": name, "body": body},
		}, nil
	case OperationActionRabbitMQBindingUpsert:
		source := operationStringParam(params, "source")
		destinationType := normalizeRabbitDestinationType(operationStringParam(params, "destinationType", "destination_type"))
		destinationSegment := "q"
		if destinationType == ResourceTypeExchange {
			destinationSegment = "e"
		}
		body := map[string]any{
			"routing_key": operationStringParam(params, "routingKey", "routing_key"),
			"arguments":   operationAnyMapParam(params, "arguments"),
		}
		target := baseURL + "/api/bindings/" + escapePath(vhost) + "/e/" + escapePath(source) + "/" + destinationSegment + "/" + escapePath(name)
		if err := doJSONRequestWithBody(ctx, client, http.MethodPost, target, credential, body, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeBinding,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Binding 创建/更新已提交",
			Result:       map[string]any{"vhost": vhost, "source": source, "destinationType": destinationType, "destination": name, "body": body},
		}, nil
	case OperationActionRabbitMQQueuePurge:
		target := baseURL + "/api/queues/" + escapePath(vhost) + "/" + escapePath(name) + "/contents"
		if err := doJSONRequest(ctx, client, http.MethodDelete, target, credential, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeQueue,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Queue 已清空",
			Result:       map[string]any{"vhost": vhost, "queue": name},
		}, nil
	case OperationActionRabbitMQQueueDelete:
		query := url.Values{}
		query.Set("if-unused", fmt.Sprintf("%t", operationBoolParam(params, false, "ifUnused", "if_unused")))
		query.Set("if-empty", fmt.Sprintf("%t", operationBoolParam(params, false, "ifEmpty", "if_empty")))
		target := baseURL + "/api/queues/" + escapePath(vhost) + "/" + escapePath(name) + "?" + query.Encode()
		if err := doJSONRequest(ctx, client, http.MethodDelete, target, credential, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeQueue,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Queue 已删除",
			Result:       map[string]any{"vhost": vhost, "queue": name, "ifUnused": query.Get("if-unused"), "ifEmpty": query.Get("if-empty")},
		}, nil
	case OperationActionRabbitMQExchangeDelete:
		query := url.Values{}
		query.Set("if-unused", fmt.Sprintf("%t", operationBoolParam(params, false, "ifUnused", "if_unused")))
		target := baseURL + "/api/exchanges/" + escapePath(vhost) + "/" + escapePath(name) + "?" + query.Encode()
		if err := doJSONRequest(ctx, client, http.MethodDelete, target, credential, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeExchange,
			Namespace:    vhost,
			ResourceName: name,
			Message:      "RabbitMQ Exchange 已删除",
			Result:       map[string]any{"vhost": vhost, "exchange": name, "ifUnused": query.Get("if-unused")},
		}, nil
	default:
		return nil, adapterNotSupported(instance.MQType, "资源操作")
	}
}

func (a *RabbitMQAdapter) validateQueueUpsert(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelLow)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	name := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "queue", "name"))
	if err := ensureOperationRequired(name, "Queue名称"); err != nil {
		return nil, err
	}
	params := map[string]any{
		"durable":    operationBoolParam(req.Params, true, "durable"),
		"autoDelete": operationBoolParam(req.Params, false, "autoDelete", "auto_delete"),
		"arguments":  operationAnyMapParam(req.Params, "arguments"),
	}
	req.ResourceType = ResourceTypeQueue
	req.Namespace = vhost
	req.ResourceName = name
	validation.ResourceType = ResourceTypeQueue
	validation.Namespace = vhost
	validation.ResourceName = name
	validation.Message = "将通过 RabbitMQ Management API 创建或更新 Queue"
	validation.Impacts = []string{"目标 vhost: " + vhost, "Queue: " + name}
	validation.Warnings = []string{"如果 Queue 已存在，RabbitMQ 会校验 durable/auto_delete/arguments 是否兼容"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *RabbitMQAdapter) validateExchangeUpsert(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelLow)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	name := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "exchange", "name"))
	exchangeType := firstNonEmpty(operationStringParam(req.Params, "type"), "direct")
	if err := ensureOperationRequired(name, "Exchange名称"); err != nil {
		return nil, err
	}
	params := map[string]any{
		"type":       exchangeType,
		"durable":    operationBoolParam(req.Params, true, "durable"),
		"autoDelete": operationBoolParam(req.Params, false, "autoDelete", "auto_delete"),
		"internal":   operationBoolParam(req.Params, false, "internal"),
		"arguments":  operationAnyMapParam(req.Params, "arguments"),
	}
	req.ResourceType = ResourceTypeExchange
	req.Namespace = vhost
	req.ResourceName = name
	validation.ResourceType = ResourceTypeExchange
	validation.Namespace = vhost
	validation.ResourceName = name
	validation.Message = "将通过 RabbitMQ Management API 创建或更新 Exchange"
	validation.Impacts = []string{"目标 vhost: " + vhost, "Exchange: " + name, "类型: " + exchangeType}
	validation.Warnings = []string{"如果 Exchange 已存在，RabbitMQ 会校验 type/durable/arguments 是否兼容"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *RabbitMQAdapter) validateBindingUpsert(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	source := operationStringParam(req.Params, "source")
	destinationType := normalizeRabbitDestinationType(operationStringParam(req.Params, "destinationType", "destination_type"))
	destination := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "destination", "name"))
	if err := ensureOperationRequired(source, "源Exchange"); err != nil {
		return nil, err
	}
	if err := ensureOperationRequired(destination, "目标资源"); err != nil {
		return nil, err
	}
	params := map[string]any{
		"source":          source,
		"destinationType": destinationType,
		"routingKey":      operationStringParam(req.Params, "routingKey", "routing_key"),
		"arguments":       operationAnyMapParam(req.Params, "arguments"),
	}
	req.ResourceType = ResourceTypeBinding
	req.Namespace = vhost
	req.ResourceName = destination
	validation.ResourceType = ResourceTypeBinding
	validation.Namespace = vhost
	validation.ResourceName = destination
	validation.Message = "将通过 RabbitMQ Management API 创建 Binding"
	validation.Impacts = []string{"源 Exchange: " + source, "目标 " + destinationType + ": " + destination, "RoutingKey: " + stringValue(params["routingKey"])}
	validation.Warnings = []string{"Binding 会改变后续消息路由关系，请确认路由键和目标资源"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *RabbitMQAdapter) validateQueuePurge(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	name := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "queue", "name"))
	if err := ensureOperationRequired(name, "Queue名称"); err != nil {
		return nil, err
	}
	params := map[string]any{"vhost": vhost, "queue": name}
	req.ResourceType = ResourceTypeQueue
	req.Namespace = vhost
	req.ResourceName = name
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeQueue
	validation.Namespace = vhost
	validation.ResourceName = name
	validation.Message = "将清空 RabbitMQ Queue 中所有 ready 消息"
	validation.Impacts = []string{"目标 vhost: " + vhost, "Queue: " + name, "清空后未消费消息不可恢复"}
	validation.Warnings = []string{"Purge 不会删除 Queue 本身，但会丢弃当前堆积消息，请确认消费者可接受"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *RabbitMQAdapter) validateQueueDelete(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	name := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "queue", "name"))
	if err := ensureOperationRequired(name, "Queue名称"); err != nil {
		return nil, err
	}
	params := map[string]any{
		"vhost":    vhost,
		"queue":    name,
		"ifUnused": operationBoolParam(req.Params, false, "ifUnused", "if_unused"),
		"ifEmpty":  operationBoolParam(req.Params, false, "ifEmpty", "if_empty"),
	}
	req.ResourceType = ResourceTypeQueue
	req.Namespace = vhost
	req.ResourceName = name
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeQueue
	validation.Namespace = vhost
	validation.ResourceName = name
	validation.Message = "将删除 RabbitMQ Queue"
	validation.Impacts = []string{"目标 vhost: " + vhost, "Queue: " + name, "Queue 删除后绑定关系和未消费消息会丢失"}
	validation.Warnings = []string{"删除 Queue 会影响生产者路由和消费者订阅，请先确认业务已停用或完成迁移"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *RabbitMQAdapter) validateExchangeDelete(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	vhost := firstNonEmpty(req.Namespace, operationStringParam(req.Params, "vhost"), "/")
	name := firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "exchange", "name"))
	if err := ensureOperationRequired(name, "Exchange名称"); err != nil {
		return nil, err
	}
	params := map[string]any{
		"vhost":    vhost,
		"exchange": name,
		"ifUnused": operationBoolParam(req.Params, false, "ifUnused", "if_unused"),
		"force":    operationBoolParam(req.Params, false, "force"),
	}
	req.ResourceType = ResourceTypeExchange
	req.Namespace = vhost
	req.ResourceName = name
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeExchange
	validation.Namespace = vhost
	validation.ResourceName = name
	validation.Message = "将删除 RabbitMQ Exchange"
	validation.Impacts = []string{"目标 vhost: " + vhost, "Exchange: " + name, "Exchange 删除后相关路由会失效"}
	validation.Warnings = []string{"删除 Exchange 会影响后续消息路由，请确认没有生产者继续写入；存在 binding 时必须显式 force=true"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func normalizeRabbitDestinationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exchange", "e":
		return ResourceTypeExchange
	default:
		return ResourceTypeQueue
	}
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
