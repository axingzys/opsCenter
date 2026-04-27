package messagequeue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

func (a *PulsarAdapter) ValidateOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	normalizeResourceOperationRequest(req)
	switch req.Action {
	case OperationActionPulsarRetentionUpdate:
		return a.validateRetentionUpdate(instance, req)
	case OperationActionPulsarTTLUpdate:
		return a.validateTTLUpdate(instance, req)
	case OperationActionPulsarTopicDelete:
		return a.validateTopicDelete(instance, req)
	case OperationActionPulsarSubscriptionSkip:
		return a.validateSubscriptionSkip(instance, req)
	case OperationActionPulsarSubscriptionReset:
		return a.validateSubscriptionReset(instance, req)
	default:
		return unsupportedOperationValidation(instance, req, "Pulsar 不支持该资源操作"), nil
	}
}

func (a *PulsarAdapter) ApplyOperation(ctx context.Context, instance *MQInstance, credential *ConnectionCredential, req *ResourceOperationRequest) (*ResourceOperationApplyResult, error) {
	validation, err := a.ValidateOperation(ctx, instance, credential, req)
	if err != nil {
		return nil, err
	}
	if !validation.Supported {
		return nil, fmt.Errorf("%s", validation.Message)
	}
	baseURL := managementBaseURL(instance, DefaultManagementPort(MQTypePulsar), instance.TLSEnabled)
	client := httpClient(instance.TLSEnabled)
	params := validation.NormalizedParams
	tenant := operationStringParam(params, "tenant")
	namespace := operationStringParam(params, "namespace")
	namespacePath := baseURL + "/admin/v2/namespaces/" + escapePath(tenant) + "/" + escapePath(namespace)
	fullName := tenant + "/" + namespace
	switch req.Action {
	case OperationActionPulsarRetentionUpdate:
		body := map[string]int{
			"retentionTimeInMinutes": operationIntParam(params, 0, "retentionTimeInMinutes"),
			"retentionSizeInMB":      operationIntParam(params, 0, "retentionSizeInMB"),
		}
		if err := doJSONRequestWithBody(ctx, client, http.MethodPost, namespacePath+"/retention", credential, body, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeNamespace,
			Namespace:    tenant,
			ResourceName: fullName,
			Message:      "Pulsar Namespace Retention 更新已提交",
			Result:       map[string]any{"namespace": fullName, "retention": body},
		}, nil
	case OperationActionPulsarTTLUpdate:
		ttlSeconds := operationIntParam(params, 0, "messageTTLInSeconds", "ttlSeconds")
		if err := doJSONRequestWithBody(ctx, client, http.MethodPost, namespacePath+"/messageTTL", credential, ttlSeconds, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeNamespace,
			Namespace:    tenant,
			ResourceName: fullName,
			Message:      "Pulsar Namespace TTL 更新已提交",
			Result:       map[string]any{"namespace": fullName, "messageTTLInSeconds": ttlSeconds},
		}, nil
	case OperationActionPulsarTopicDelete:
		topic := operationStringParam(params, "topic")
		force := operationBoolParam(params, false, "force")
		target := baseURL + "/admin/v2/" + pulsarTopicPath(topic) + "?force=" + strconv.FormatBool(force)
		if err := doJSONRequest(ctx, client, http.MethodDelete, target, credential, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeTopic,
			ResourceName: topic,
			Message:      "Pulsar Topic 已删除",
			Result:       map[string]any{"topic": topic, "force": force},
		}, nil
	case OperationActionPulsarSubscriptionSkip:
		topic := operationStringParam(params, "topic")
		subscription := operationStringParam(params, "subscription")
		target := baseURL + "/admin/v2/" + pulsarTopicPath(topic) + "/subscription/" + escapePath(subscription) + "/skip_all"
		if err := doJSONRequestWithBody(ctx, client, http.MethodPost, target, credential, nil, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeSubscription,
			ResourceName: subscription,
			Message:      "Pulsar Subscription 已跳过全部积压消息",
			Result:       map[string]any{"topic": topic, "subscription": subscription},
		}, nil
	case OperationActionPulsarSubscriptionReset:
		topic := operationStringParam(params, "topic")
		subscription := operationStringParam(params, "subscription")
		timestampMs := int64(operationIntParam(params, 0, "timestampMs", "timestamp"))
		target := baseURL + "/admin/v2/" + pulsarTopicPath(topic) + "/subscription/" + escapePath(subscription) + "/resetcursor/" + strconv.FormatInt(timestampMs, 10)
		if err := doJSONRequestWithBody(ctx, client, http.MethodPost, target, credential, nil, nil); err != nil {
			return nil, err
		}
		return &ResourceOperationApplyResult{
			ResourceType: ResourceTypeSubscription,
			ResourceName: subscription,
			Message:      "Pulsar Subscription Cursor 已重置",
			Result:       map[string]any{"topic": topic, "subscription": subscription, "timestampMs": timestampMs},
		}, nil
	default:
		return nil, adapterNotSupported(instance.MQType, "资源操作")
	}
}

func (a *PulsarAdapter) validateRetentionUpdate(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	tenant, namespace, fullName, err := normalizePulsarNamespace(req)
	if err != nil {
		return nil, err
	}
	if !operationParamExists(req.Params, "retentionTimeInMinutes") || !operationParamExists(req.Params, "retentionSizeInMB") {
		return nil, fmt.Errorf("Retention时间和大小不能为空")
	}
	retentionTime := operationIntParam(req.Params, 0, "retentionTimeInMinutes")
	retentionSize := operationIntParam(req.Params, 0, "retentionSizeInMB")
	if retentionTime < -1 || retentionSize < -1 {
		return nil, fmt.Errorf("Retention时间和大小不能小于-1")
	}
	params := map[string]any{
		"tenant":                 tenant,
		"namespace":              namespace,
		"retentionTimeInMinutes": retentionTime,
		"retentionSizeInMB":      retentionSize,
	}
	req.ResourceType = ResourceTypeNamespace
	req.Namespace = tenant
	req.ResourceName = fullName
	validation.ResourceType = ResourceTypeNamespace
	validation.Namespace = tenant
	validation.ResourceName = fullName
	validation.Message = "将通过 Pulsar Admin API 更新 Namespace Retention"
	validation.Impacts = []string{"Namespace: " + fullName, fmt.Sprintf("保留时间(分钟): %d", retentionTime), fmt.Sprintf("保留大小(MB): %d", retentionSize)}
	validation.Warnings = []string{"Retention 配置会影响已消费消息的保留策略，请确认业务合规要求"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *PulsarAdapter) validateTTLUpdate(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelMedium)
	tenant, namespace, fullName, err := normalizePulsarNamespace(req)
	if err != nil {
		return nil, err
	}
	if !operationParamExists(req.Params, "messageTTLInSeconds", "ttlSeconds") {
		return nil, fmt.Errorf("TTL秒数不能为空")
	}
	ttlSeconds := operationIntParam(req.Params, 0, "messageTTLInSeconds", "ttlSeconds")
	if ttlSeconds < 0 {
		return nil, fmt.Errorf("TTL秒数不能小于0")
	}
	params := map[string]any{
		"tenant":              tenant,
		"namespace":           namespace,
		"messageTTLInSeconds": ttlSeconds,
	}
	req.ResourceType = ResourceTypeNamespace
	req.Namespace = tenant
	req.ResourceName = fullName
	validation.ResourceType = ResourceTypeNamespace
	validation.Namespace = tenant
	validation.ResourceName = fullName
	validation.Message = "将通过 Pulsar Admin API 更新 Namespace TTL"
	validation.Impacts = []string{"Namespace: " + fullName, fmt.Sprintf("TTL秒数: %d", ttlSeconds)}
	validation.Warnings = []string{"TTL 会影响消息过期时间，设置过小可能导致消费者读取不到历史消息"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *PulsarAdapter) validateTopicDelete(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelHigh)
	topic := normalizePulsarTopicName(firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "topicName", "name")))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	force := operationBoolParam(req.Params, false, "force")
	params := map[string]any{"topic": topic, "force": force}
	req.ResourceType = ResourceTypeTopic
	req.ResourceName = topic
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeTopic
	validation.ResourceName = topic
	validation.Message = "将通过 Pulsar Admin API 删除 Topic"
	validation.Impacts = []string{"Topic: " + topic, "Topic 删除后消息、分区和订阅关系会失效"}
	validation.Warnings = []string{"force=true 会强制删除仍有订阅或连接的 Topic，请谨慎使用"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *PulsarAdapter) validateSubscriptionSkip(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelCritical)
	topic := normalizePulsarTopicName(firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "topicName", "name")))
	subscription := operationStringParam(req.Params, "subscription", "subscriptionName", "groupName")
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	if err := ensureOperationRequired(subscription, "Subscription名称"); err != nil {
		return nil, err
	}
	params := map[string]any{"topic": topic, "subscription": subscription}
	req.ResourceType = ResourceTypeSubscription
	req.ResourceName = subscription
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeSubscription
	validation.ResourceName = subscription
	validation.Message = "将跳过 Pulsar Subscription 的全部积压消息"
	validation.Impacts = []string{"Topic: " + topic, "Subscription: " + subscription, "当前积压消息会被标记为已消费"}
	validation.Warnings = []string{"skip_all 会导致消费者不再收到被跳过的历史消息"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func (a *PulsarAdapter) validateSubscriptionReset(instance *MQInstance, req *ResourceOperationRequest) (*ResourceOperationValidationVO, error) {
	validation := newOperationValidation(instance, req, RiskLevelCritical)
	topic := normalizePulsarTopicName(firstNonEmpty(req.ResourceName, operationStringParam(req.Params, "topic", "topicName", "name")))
	subscription := operationStringParam(req.Params, "subscription", "subscriptionName", "groupName")
	timestampMs := int64(operationIntParam(req.Params, 0, "timestampMs", "timestamp"))
	if err := ensureOperationRequired(topic, "Topic名称"); err != nil {
		return nil, err
	}
	if err := ensureOperationRequired(subscription, "Subscription名称"); err != nil {
		return nil, err
	}
	if timestampMs <= 0 {
		return nil, fmt.Errorf("重置时间戳不能为空")
	}
	params := map[string]any{"topic": topic, "subscription": subscription, "timestampMs": timestampMs}
	req.ResourceType = ResourceTypeSubscription
	req.ResourceName = subscription
	validation.RequiredPermission = PermissionHighRisk
	validation.ResourceType = ResourceTypeSubscription
	validation.ResourceName = subscription
	validation.Message = "将重置 Pulsar Subscription Cursor"
	validation.Impacts = []string{"Topic: " + topic, "Subscription: " + subscription, fmt.Sprintf("目标时间戳(ms): %d", timestampMs)}
	validation.Warnings = []string{"Cursor 重置可能造成历史消息重放或跳过，请确认目标时间点"}
	setNormalizedParams(req, validation, params)
	return validation, nil
}

func normalizePulsarNamespace(req *ResourceOperationRequest) (string, string, string, error) {
	raw := firstNonEmpty(operationStringParam(req.Params, "fullName", "namespaceFullName"), req.ResourceName, req.Namespace, operationStringParam(req.Params, "namespace"))
	tenant := operationStringParam(req.Params, "tenant")
	namespace := operationStringParam(req.Params, "namespace")
	if strings.Count(raw, "/") == 1 {
		parts := strings.Split(raw, "/")
		tenant = firstNonEmpty(tenant, parts[0])
		namespace = parts[1]
	} else if strings.Count(req.Namespace, "/") == 1 {
		parts := strings.Split(req.Namespace, "/")
		tenant = firstNonEmpty(tenant, parts[0])
		namespace = parts[1]
	}
	if err := ensureOperationRequired(tenant, "Tenant"); err != nil {
		return "", "", "", err
	}
	if err := ensureOperationRequired(namespace, "Namespace"); err != nil {
		return "", "", "", err
	}
	return tenant, namespace, tenant + "/" + namespace, nil
}

func normalizePulsarTopicName(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" || strings.Contains(topic, "://") {
		return topic
	}
	if strings.Count(topic, "/") == 2 {
		return "persistent://" + topic
	}
	return topic
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
