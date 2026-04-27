package messagequeue

import (
	"context"
	"fmt"
	"strings"
)

func (uc *UseCase) GetInstanceCapabilities(ctx context.Context, instanceID uint) (*MQCapabilitiesVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	actions, err := uc.ListOperationActions(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return &MQCapabilitiesVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		MQType:       instance.MQType,
		MQTypeText:   TypeText(instance.MQType),
		Capabilities: uc.buildCapabilityItems(ctx, instance),
		Actions:      actions,
	}, nil
}

func (uc *UseCase) ListOperationActions(ctx context.Context, instanceID uint) ([]*MQOperationActionVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	cfg, err := uc.resolveHighRiskOperationConfig(ctx)
	if err != nil {
		return nil, err
	}
	defs := operationActionDefinitions(instance.MQType)
	actions := make([]*MQOperationActionVO, 0, len(defs))
	for _, def := range defs {
		action := def.toVO()
		if strings.TrimSpace(instance.Status) != InstanceStatusEnabled {
			action.Enabled = false
			action.DisabledReason = "MQ实例已禁用"
		} else if def.highRisk && !cfg.Enabled {
			action.Enabled = false
			action.DisabledReason = "MQ高风险操作总开关未开启"
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func (uc *UseCase) buildCapabilityItems(ctx context.Context, instance *MQInstance) []*MQCapabilityItemVO {
	enabled := strings.TrimSpace(instance.Status) == InstanceStatusEnabled
	disabledReason := ""
	if !enabled {
		disabledReason = "MQ实例已禁用"
	}
	_, hasAdapter := uc.adapters.Get(instance.MQType)
	operationDefs := operationActionDefinitions(instance.MQType)
	cfg, err := uc.resolveHighRiskOperationConfig(ctx)
	if err != nil || cfg == nil {
		cfg = &HighRiskOperationConfig{}
	}
	return []*MQCapabilityItemVO{
		capabilityItem("test_connection", "连接测试", enabled && hasAdapter, firstDisabledReason(disabledReason, adapterDisabledReason(hasAdapter))),
		capabilityItem("discover_metadata", "元数据同步", enabled && hasAdapter, firstDisabledReason(disabledReason, adapterDisabledReason(hasAdapter))),
		capabilityItem("diagnosis", "消费诊断", enabled && hasAdapter, firstDisabledReason(disabledReason, adapterDisabledReason(hasAdapter))),
		capabilityItem("sample_message", "消息采样", enabled && supportsMessageSample(instance.MQType), firstDisabledReason(disabledReason, unsupportedReason(supportsMessageSample(instance.MQType), "当前 MQ 类型暂未开放消息采样"))),
		capabilityItem("resource_manage", "资源管理", enabled && len(operationDefs) > 0, firstDisabledReason(disabledReason, unsupportedReason(len(operationDefs) > 0, "当前 MQ 类型暂未开放资源管理"))),
		capabilityItem("high_risk_operation", "高危操作", enabled && len(highRiskActionDefinitions(instance.MQType)) > 0 && cfg.Enabled, firstDisabledReason(disabledReason, highRiskCapabilityDisabledReason(instance.MQType, cfg))),
	}
}

type operationActionDefinition struct {
	action        string
	resourceType  string
	riskLevel     string
	permission    uint
	defaultParams map[string]any
	formSchema    map[string]any
	highRisk      bool
}

func (d operationActionDefinition) toVO() *MQOperationActionVO {
	riskLevel := firstNonEmpty(d.riskLevel, RiskLevelLow)
	permission := d.permission
	if permission == 0 {
		permission = PermissionResourceManage
	}
	highRisk := d.highRisk || IsHighRiskLevel(riskLevel)
	if highRisk {
		permission = PermissionHighRisk
	}
	return &MQOperationActionVO{
		Action:              d.action,
		ActionText:          actionText(d.action),
		ResourceType:        d.resourceType,
		RiskLevel:           riskLevel,
		RequiredPermission:  permission,
		RequiresConfirm:     true,
		RequiresHighRiskAck: highRisk,
		Enabled:             true,
		DefaultParams:       cloneAnyMap(d.defaultParams),
		FormSchema:          cloneAnyMap(d.formSchema),
	}
}

func operationActionDefinitions(mqType string) []operationActionDefinition {
	switch NormalizeType(mqType) {
	case MQTypeRabbitMQ:
		return []operationActionDefinition{
			{action: OperationActionRabbitMQQueueUpsert, resourceType: ResourceTypeQueue, riskLevel: RiskLevelLow, defaultParams: map[string]any{"durable": true, "autoDelete": false, "arguments": map[string]any{}}},
			{action: OperationActionRabbitMQExchangeUpsert, resourceType: ResourceTypeExchange, riskLevel: RiskLevelLow, defaultParams: map[string]any{"type": "direct", "durable": true, "autoDelete": false, "internal": false, "arguments": map[string]any{}}},
			{action: OperationActionRabbitMQBindingUpsert, resourceType: ResourceTypeBinding, riskLevel: RiskLevelLow, defaultParams: map[string]any{"source": "amq.direct", "destinationType": "queue", "routingKey": "", "arguments": map[string]any{}}},
			{action: OperationActionRabbitMQQueuePurge, resourceType: ResourceTypeQueue, riskLevel: RiskLevelHigh, highRisk: true, defaultParams: map[string]any{}},
			{action: OperationActionRabbitMQQueueDelete, resourceType: ResourceTypeQueue, riskLevel: RiskLevelHigh, highRisk: true, defaultParams: map[string]any{"ifUnused": false, "ifEmpty": false}},
			{action: OperationActionRabbitMQExchangeDelete, resourceType: ResourceTypeExchange, riskLevel: RiskLevelHigh, highRisk: true, defaultParams: map[string]any{"ifUnused": false, "force": false}},
		}
	case MQTypeKafka:
		return []operationActionDefinition{
			{action: OperationActionKafkaTopicCreate, resourceType: ResourceTypeTopic, riskLevel: RiskLevelLow, defaultParams: map[string]any{"partitions": 1, "replicationFactor": 1, "configs": map[string]any{}}},
			{action: OperationActionKafkaPartitionsExpand, resourceType: ResourceTypeTopic, riskLevel: RiskLevelMedium, defaultParams: map[string]any{"partitions": 3}},
			{action: OperationActionKafkaTopicConfigUpdate, resourceType: ResourceTypeTopic, riskLevel: RiskLevelMedium, defaultParams: map[string]any{"configs": map[string]any{"retention.ms": "604800000"}}},
			{action: OperationActionKafkaTopicDelete, resourceType: ResourceTypeTopic, riskLevel: RiskLevelCritical, highRisk: true, defaultParams: map[string]any{}},
		}
	case MQTypePulsar:
		return []operationActionDefinition{
			{action: OperationActionPulsarRetentionUpdate, resourceType: ResourceTypeNamespace, riskLevel: RiskLevelMedium, defaultParams: map[string]any{"retentionTimeInMinutes": 1440, "retentionSizeInMB": 1024}},
			{action: OperationActionPulsarTTLUpdate, resourceType: ResourceTypeNamespace, riskLevel: RiskLevelMedium, defaultParams: map[string]any{"messageTTLInSeconds": 86400}},
			{action: OperationActionPulsarTopicDelete, resourceType: ResourceTypeTopic, riskLevel: RiskLevelCritical, highRisk: true, defaultParams: map[string]any{"force": false}},
			{action: OperationActionPulsarSubscriptionSkip, resourceType: ResourceTypeSubscription, riskLevel: RiskLevelHigh, highRisk: true, defaultParams: map[string]any{"topic": "persistent://public/default/topic", "subscription": ""}},
			{action: OperationActionPulsarSubscriptionReset, resourceType: ResourceTypeSubscription, riskLevel: RiskLevelHigh, highRisk: true, defaultParams: map[string]any{"topic": "persistent://public/default/topic", "subscription": "", "timestampMs": 0}},
		}
	default:
		return nil
	}
}

func highRiskActionDefinitions(mqType string) []operationActionDefinition {
	defs := operationActionDefinitions(mqType)
	result := make([]operationActionDefinition, 0)
	for _, def := range defs {
		if def.highRisk || IsHighRiskLevel(def.riskLevel) {
			result = append(result, def)
		}
	}
	return result
}

func supportsMessageSample(mqType string) bool {
	return NormalizeType(mqType) == MQTypeKafka
}

func capabilityItem(key, name string, enabled bool, disabledReason string) *MQCapabilityItemVO {
	if enabled {
		disabledReason = ""
	}
	return &MQCapabilityItemVO{Key: key, Name: name, Enabled: enabled, DisabledReason: disabledReason}
}

func adapterDisabledReason(hasAdapter bool) string {
	return unsupportedReason(hasAdapter, "当前 MQ 类型适配器未注册")
}

func unsupportedReason(ok bool, reason string) string {
	if ok {
		return ""
	}
	return reason
}

func highRiskCapabilityDisabledReason(mqType string, cfg *HighRiskOperationConfig) string {
	if len(highRiskActionDefinitions(mqType)) == 0 {
		return "当前 MQ 类型暂未开放高危操作"
	}
	if cfg == nil || !cfg.Enabled {
		return "MQ高风险操作总开关未开启"
	}
	return ""
}

func firstDisabledReason(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneAnyMap(nested)
		} else {
			out[key] = value
		}
	}
	return out
}
