package messagequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	defaultCapacityForecastHorizonHours = 24
	defaultAuditChainVerifyLimit        = 200
	maxAuditChainVerifyLimit            = 1000
)

func (uc *UseCase) InspectMessageSchema(ctx context.Context, instanceID uint, req *MessageSchemaInspectRequest, operator Operator) (*MessageSchemaInspectVO, error) {
	if req == nil {
		return nil, fmt.Errorf("Schema检查参数不能为空")
	}
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("MQ实例仓库未初始化")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	req.ResourceType = firstNonEmpty(strings.TrimSpace(req.ResourceType), ResourceTypeTopic)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.SchemaJSON = strings.TrimSpace(req.SchemaJSON)

	result := &MessageSchemaInspectVO{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
		MQType:       instance.MQType,
		ResourceType: req.ResourceType,
		Namespace:    req.Namespace,
		ResourceName: req.ResourceName,
		Strict:       req.Strict,
		PayloadBytes: len([]byte(req.Payload)),
		PayloadHash:  sha256Text(req.Payload),
		Valid:        true,
		Fields:       make([]*MessageSchemaFieldVO, 0),
		Issues:       make([]*MessageSchemaValidationIssueVO, 0),
		GeneratedAt:  nowText(),
	}
	redacted, sensitiveHits, sensitiveFields := redactPayload(req.Payload)
	result.SensitiveHitCount = sensitiveHits
	result.SensitiveFields = sensitiveFields
	if sensitiveHits > 0 {
		result.RedactedPreview = redacted
	}

	var payload any
	if err := json.Unmarshal([]byte(req.Payload), &payload); err != nil {
		result.Format = "text"
		result.Message = "payload 不是 JSON，已按纯文本执行 DLP 识别"
		if req.SchemaJSON != "" {
			result.Valid = false
			result.Issues = append(result.Issues, schemaIssue("$", "error", "非 JSON payload 暂不支持 JSON Schema 校验"))
		}
		uc.auditSchemaInspect(ctx, instance, req, result, operator)
		return result, nil
	}

	result.Format = "json"
	result.PrettyPayload = prettyJSON(payload)
	result.InferredSchema = inferJSONSchema(payload)
	result.Fields = collectJSONSchemaFields(payload)
	if req.SchemaJSON != "" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(req.SchemaJSON), &schema); err != nil {
			result.Valid = false
			result.Issues = append(result.Issues, schemaIssue("$schema", "error", "Schema JSON 格式错误: "+err.Error()))
		} else {
			result.Issues = append(result.Issues, validateJSONSchema(payload, schema, "$", req.Strict)...)
			result.Valid = !hasSchemaError(result.Issues)
		}
	}
	if result.Valid {
		result.Message = "Schema 检查完成"
	} else {
		result.Message = "Schema 检查发现不兼容项"
	}
	uc.auditSchemaInspect(ctx, instance, req, result, operator)
	return result, nil
}

func (uc *UseCase) GetCapacityForecast(ctx context.Context, instanceID uint, req *CapacityForecastRequest) (*CapacityForecastVO, error) {
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("MQ实例仓库未初始化")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	horizon := defaultCapacityForecastHorizonHours
	if req != nil && req.HorizonHours > 0 {
		horizon = req.HorizonHours
	}
	if horizon > 24*30 {
		horizon = 24 * 30
	}
	result := &CapacityForecastVO{
		InstanceID:           instance.ID,
		InstanceName:         instance.Name,
		MQType:               instance.MQType,
		HorizonHours:         horizon,
		Recommendations:      make([]*CapacityRecommendationVO, 0),
		TopBacklogResources:  make([]*DashboardResourceVO, 0),
		TopLagConsumerGroups: make([]*DashboardConsumerGroupVO, 0),
		RiskLevel:            RiskLevelLow,
		GeneratedAt:          nowText(),
	}

	if uc.metricSnapshotRepo != nil {
		snapshots, _, _ := uc.metricSnapshotRepo.List(ctx, instance.ID, &MetricSnapshotListRequest{Page: 1, PageSize: 100, ResourceType: "instance"})
		result.SampleCount = len(snapshots)
		if len(snapshots) > 0 {
			newest := snapshots[0]
			result.CurrentBacklog = newest.Backlog
			result.CurrentLag = newest.Lag
			if len(snapshots) > 1 {
				oldest := snapshots[len(snapshots)-1]
				hours := newest.CollectedAt.Sub(oldest.CollectedAt).Hours()
				if hours > 0 {
					result.BacklogGrowthPerHour = float64(newest.Backlog-oldest.Backlog) / hours
					result.LagGrowthPerHour = float64(newest.Lag-oldest.Lag) / hours
				}
			}
		}
	}
	if result.SampleCount == 0 {
		uc.fillCapacityCurrentFromMetadata(ctx, instance, result)
	}
	result.ProjectedBacklog = nonNegativeInt64(float64(result.CurrentBacklog) + result.BacklogGrowthPerHour*float64(horizon))
	result.ProjectedLag = nonNegativeInt64(float64(result.CurrentLag) + result.LagGrowthPerHour*float64(horizon))

	if uc.resourceRepo != nil {
		topResources, _ := uc.resourceRepo.TopBacklog(ctx, instance.ID, 10)
		for _, item := range topResources {
			result.TopBacklogResources = append(result.TopBacklogResources, toDashboardResource(instance, item))
		}
	}
	if uc.consumerGroupRepo != nil {
		groups, _, _ := uc.consumerGroupRepo.List(ctx, instance.ID, &ConsumerGroupListRequest{Page: 1, PageSize: 10, HasLag: "true"})
		for _, item := range groups {
			result.TopLagConsumerGroups = append(result.TopLagConsumerGroups, toDashboardConsumerGroup(instance, item))
		}
	}
	result.RiskLevel, result.TrendMessage = capacityRiskAndMessage(result)
	result.Recommendations = buildCapacityRecommendations(instance, result)
	return result, nil
}

func (uc *UseCase) BuildConfigClonePlan(ctx context.Context, sourceInstanceID uint, req *ConfigClonePlanRequest, operator Operator) (*ConfigClonePlanVO, error) {
	if req == nil {
		return nil, fmt.Errorf("配置克隆参数不能为空")
	}
	if uc.instanceRepo == nil || uc.resourceRepo == nil {
		return nil, fmt.Errorf("MQ实例或资源仓库未初始化")
	}
	sourceInstance, err := uc.instanceRepo.GetByID(ctx, sourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("源MQ实例不存在")
	}
	targetInstance, err := uc.instanceRepo.GetByID(ctx, req.TargetInstanceID)
	if err != nil {
		return nil, fmt.Errorf("目标MQ实例不存在")
	}
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.TargetNamespace = firstNonEmpty(strings.TrimSpace(req.TargetNamespace), req.Namespace)
	req.TargetResourceName = firstNonEmpty(strings.TrimSpace(req.TargetResourceName), req.ResourceName)
	sourceResource := uc.findCloneResource(ctx, sourceInstance.ID, req.ResourceType, req.Namespace, req.ResourceName)
	if sourceResource == nil {
		return nil, fmt.Errorf("源资源不存在")
	}
	targetResource := uc.findCloneResource(ctx, targetInstance.ID, req.ResourceType, req.TargetNamespace, req.TargetResourceName)
	sourceSnapshot := resourceSnapshot(sourceResource)
	targetSnapshot := resourceSnapshot(targetResource)
	proposed := buildProposedCloneConfig(sourceInstance, sourceResource, req)
	warnings, risk := configCloneWarnings(sourceInstance, targetInstance, targetResource)
	plan := &ConfigClonePlanVO{
		SourceInstanceID:   sourceInstance.ID,
		SourceInstanceName: sourceInstance.Name,
		SourceMQType:       sourceInstance.MQType,
		TargetInstanceID:   targetInstance.ID,
		TargetInstanceName: targetInstance.Name,
		TargetMQType:       targetInstance.MQType,
		ResourceType:       req.ResourceType,
		Namespace:          req.Namespace,
		ResourceName:       req.ResourceName,
		TargetNamespace:    req.TargetNamespace,
		TargetResourceName: req.TargetResourceName,
		Executable:         false,
		RequiresApproval:   isProductionEnvironment(sourceInstance.Environment) || isProductionEnvironment(targetInstance.Environment),
		RiskLevel:          risk,
		Action:             cloneActionSuggestion(targetInstance.MQType, req.ResourceType, targetResource != nil),
		SourceSnapshot:     sourceSnapshot,
		TargetSnapshot:     targetSnapshot,
		ProposedConfig:     proposed,
		Diff:               buildOperationDiff(targetSnapshot, proposed),
		Warnings:           warnings,
		Suggestions:        configCloneSuggestions(targetInstance.MQType, req.ResourceType, targetResource != nil),
		Message:            "配置克隆计划已生成，当前不会直接修改目标集群或迁移消息",
		GeneratedAt:        nowText(),
	}
	uc.auditConfigClonePlan(ctx, sourceInstance, targetInstance, plan, operator)
	return plan, nil
}

func (uc *UseCase) VerifyAuditChain(ctx context.Context, req *AuditChainVerifyRequest) (*AuditChainVerifyVO, error) {
	if req == nil {
		req = &AuditChainVerifyRequest{}
	}
	auditType := strings.ToLower(strings.TrimSpace(req.AuditType))
	if auditType != "message" {
		auditType = "operation"
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultAuditChainVerifyLimit
	}
	if limit > maxAuditChainVerifyLimit {
		limit = maxAuditChainVerifyLimit
	}
	result := &AuditChainVerifyVO{AuditType: auditType, Valid: true, Findings: make([]*AuditChainFindingVO, 0), GeneratedAt: nowText()}
	listReq := &AuditListRequest{Page: 1, PageSize: limit, InstanceID: req.InstanceID, RestrictToAllowed: req.RestrictToAllowed, AllowedIDs: req.AllowedIDs}
	linkCheck := req.InstanceID == 0 && !req.RestrictToAllowed
	if auditType == "message" {
		if uc.messageAuditRepo == nil {
			return nil, fmt.Errorf("消息审计仓库未初始化")
		}
		items, _, err := uc.messageAuditRepo.List(ctx, listReq)
		if err != nil {
			return nil, err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		verifyMessageAuditItems(items, linkCheck, result)
		return result, nil
	}
	if uc.operationAuditRepo == nil {
		return nil, fmt.Errorf("操作审计仓库未初始化")
	}
	items, _, err := uc.operationAuditRepo.List(ctx, listReq)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	verifyOperationAuditItems(items, linkCheck, result)
	return result, nil
}

func (uc *UseCase) fillCapacityCurrentFromMetadata(ctx context.Context, instance *MQInstance, result *CapacityForecastVO) {
	if result == nil || instance == nil {
		return
	}
	if uc.resourceRepo != nil {
		summary, _ := uc.resourceRepo.Summary(ctx, instance.ID)
		if summary != nil {
			result.CurrentBacklog = summary.Backlog
		}
	}
	if uc.consumerGroupRepo != nil {
		summary, _ := uc.consumerGroupRepo.Summary(ctx, instance.ID)
		if summary != nil {
			result.CurrentLag = summary.Lag
		}
	}
}

func (uc *UseCase) findCloneResource(ctx context.Context, instanceID uint, resourceType, namespace, name string) *MQResource {
	if uc.resourceRepo == nil {
		return nil
	}
	if item, err := uc.resourceRepo.GetByUnique(ctx, instanceID, resourceType, namespace, name); err == nil && item != nil {
		return item
	}
	items, _, err := uc.resourceRepo.List(ctx, instanceID, &ResourceListRequest{Page: 1, PageSize: 20, ResourceType: resourceType, Keyword: name})
	if err != nil {
		return nil
	}
	for _, item := range items {
		if item != nil && item.Name == name && (namespace == "" || item.Namespace == namespace) {
			return item
		}
	}
	return nil
}

func (uc *UseCase) auditSchemaInspect(ctx context.Context, instance *MQInstance, req *MessageSchemaInspectRequest, result *MessageSchemaInspectVO, operator Operator) {
	if uc.messageAuditRepo == nil || instance == nil || req == nil || result == nil {
		return
	}
	status := AuditStatusSuccess
	if !result.Valid {
		status = AuditStatusPartial
	}
	_ = uc.messageAuditRepo.Create(ctx, &MQMessageAudit{
		InstanceID:        instance.ID,
		MQType:            instance.MQType,
		ResourceType:      req.ResourceType,
		ResourceName:      req.ResourceName,
		Namespace:         req.Namespace,
		Action:            AuditActionMessageSchema,
		SampleCount:       1,
		PayloadBytes:      result.PayloadBytes,
		FilterJSON:        mustJSON(map[string]any{"resourceType": req.ResourceType, "namespace": req.Namespace, "resourceName": req.ResourceName, "schemaHash": sha256Text(req.SchemaJSON), "strict": req.Strict}),
		PayloadHash:       result.PayloadHash,
		SensitiveHitCount: result.SensitiveHitCount,
		RawPayloadVisible: false,
		DLPResultJSON:     mustJSON(map[string]any{"format": result.Format, "valid": result.Valid, "sensitiveFields": result.SensitiveFields, "issues": result.Issues, "rawPayloadStored": false}),
		Status:            status,
		OperatorID:        operator.ID,
		OperatorName:      operator.Username,
		ClientIP:          operator.ClientIP,
		Message:           result.Message,
	})
}

func (uc *UseCase) auditConfigClonePlan(ctx context.Context, source, target *MQInstance, plan *ConfigClonePlanVO, operator Operator) {
	if uc.operationAuditRepo == nil || source == nil || target == nil || plan == nil {
		return
	}
	_ = uc.operationAuditRepo.Create(ctx, &MQOperationAudit{
		InstanceID:         source.ID,
		InstanceName:       source.Name,
		MQType:             source.MQType,
		ResourceType:       plan.ResourceType,
		ResourceName:       plan.ResourceName,
		Namespace:          plan.Namespace,
		Action:             AuditActionConfigClone,
		RiskLevel:          plan.RiskLevel,
		Status:             AuditStatusSuccess,
		RequestJSON:        mustJSON(map[string]any{"targetInstanceId": target.ID, "targetResourceName": plan.TargetResourceName, "targetNamespace": plan.TargetNamespace}),
		ResultJSON:         mustJSON(map[string]any{"executable": false, "action": plan.Action, "targetInstanceName": target.Name}),
		BeforeSnapshotJSON: mustJSON(plan.SourceSnapshot),
		AfterSnapshotJSON:  mustJSON(plan.TargetSnapshot),
		DiffJSON:           mustJSON(plan.Diff),
		WarningsJSON:       mustJSON(plan.Warnings),
		Reason:             "生成跨实例配置克隆计划",
		OperatorID:         operator.ID,
		OperatorName:       operator.Username,
		ClientIP:           operator.ClientIP,
		StartedAt:          ptrTime(time.Now()),
		FinishedAt:         ptrTime(time.Now()),
		Message:            plan.Message,
	})
}

func inferJSONSchema(value any) map[string]any {
	switch item := value.(type) {
	case map[string]any:
		props := map[string]any{}
		required := make([]string, 0, len(item))
		for key, child := range item {
			props[key] = inferJSONSchema(child)
			required = append(required, key)
		}
		sort.Strings(required)
		return map[string]any{"type": "object", "properties": props, "required": required}
	case []any:
		schema := map[string]any{"type": "array"}
		if len(item) > 0 {
			schema["items"] = inferJSONSchema(item[0])
		}
		return schema
	case string:
		return map[string]any{"type": "string"}
	case bool:
		return map[string]any{"type": "boolean"}
	case float64:
		if math.Trunc(item) == item {
			return map[string]any{"type": "integer"}
		}
		return map[string]any{"type": "number"}
	case nil:
		return map[string]any{"type": "null"}
	default:
		return map[string]any{"type": fmt.Sprintf("%T", value)}
	}
}

func collectJSONSchemaFields(value any) []*MessageSchemaFieldVO {
	fields := make([]*MessageSchemaFieldVO, 0)
	var walk func(path string, value any, required bool)
	walk = func(path string, value any, required bool) {
		field := &MessageSchemaFieldVO{Path: path, Type: jsonValueType(value), Required: required, Sensitive: pathHasSensitiveName(path), Example: jsonExample(value)}
		fields = append(fields, field)
		switch item := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(item))
			for key := range item {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(path+"."+key, item[key], true)
			}
		case []any:
			if len(item) > 0 {
				walk(path+"[]", item[0], false)
			}
		}
	}
	walk("$", value, true)
	return fields
}

func validateJSONSchema(value any, schema map[string]any, path string, strict bool) []*MessageSchemaValidationIssueVO {
	issues := make([]*MessageSchemaValidationIssueVO, 0)
	expectedType := schemaString(schema, "type")
	if expectedType != "" && !jsonTypeMatches(value, expectedType) {
		issues = append(issues, schemaIssue(path, "error", fmt.Sprintf("类型不匹配，期望 %s，实际 %s", expectedType, jsonValueType(value))))
		return issues
	}
	switch item := value.(type) {
	case map[string]any:
		required := schemaStringSlice(schema, "required")
		props := schemaMap(schema, "properties")
		for _, key := range required {
			if _, ok := item[key]; !ok {
				issues = append(issues, schemaIssue(path+"."+key, "error", "必填字段缺失"))
			}
		}
		for key, child := range item {
			childSchema, ok := props[key].(map[string]any)
			if !ok {
				if strict && len(props) > 0 {
					issues = append(issues, schemaIssue(path+"."+key, "error", "严格模式下不允许 Schema 外字段"))
				}
				continue
			}
			issues = append(issues, validateJSONSchema(child, childSchema, path+"."+key, strict)...)
		}
	case []any:
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for i, child := range item {
				issues = append(issues, validateJSONSchema(child, itemSchema, fmt.Sprintf("%s[%d]", path, i), strict)...)
				if i >= 20 {
					issues = append(issues, schemaIssue(path, "warning", "数组超过 20 个元素，后续元素未逐项校验"))
					break
				}
			}
		}
	}
	return issues
}

func verifyOperationAuditItems(items []*MQOperationAudit, linkCheck bool, result *AuditChainVerifyVO) {
	result.Checked = len(items)
	for i, item := range items {
		expected := operationAuditExpectedHash(item)
		if item.AuditHash == "" {
			appendAuditChainFinding(result, item.ID, "critical", "审计 hash 为空")
		} else if expected != item.AuditHash {
			appendAuditChainFinding(result, item.ID, "critical", "审计 hash 与内容不匹配")
		}
		if linkCheck && i > 0 && item.PreviousAuditHash != items[i-1].AuditHash {
			appendAuditChainFinding(result, item.ID, "critical", "审计链 previous hash 不连续")
		}
	}
	fillAuditChainRange(result, len(items), func(i int) string { return items[i].AuditHash })
}

func verifyMessageAuditItems(items []*MQMessageAudit, linkCheck bool, result *AuditChainVerifyVO) {
	result.Checked = len(items)
	for i, item := range items {
		expected := messageAuditExpectedHash(item)
		if item.AuditHash == "" {
			appendAuditChainFinding(result, item.ID, "critical", "审计 hash 为空")
		} else if expected != item.AuditHash {
			appendAuditChainFinding(result, item.ID, "critical", "审计 hash 与内容不匹配")
		}
		if linkCheck && i > 0 && item.PreviousAuditHash != items[i-1].AuditHash {
			appendAuditChainFinding(result, item.ID, "critical", "审计链 previous hash 不连续")
		}
	}
	fillAuditChainRange(result, len(items), func(i int) string { return items[i].AuditHash })
}

func operationAuditExpectedHash(item *MQOperationAudit) string {
	if item == nil {
		return ""
	}
	return auditContentHash(map[string]any{
		"previous":     item.PreviousAuditHash,
		"instanceId":   item.InstanceID,
		"mqType":       item.MQType,
		"resourceType": item.ResourceType,
		"resourceName": item.ResourceName,
		"namespace":    item.Namespace,
		"action":       item.Action,
		"riskLevel":    item.RiskLevel,
		"status":       item.Status,
		"operationId":  item.OperationID,
		"request":      item.RequestJSON,
		"result":       item.ResultJSON,
		"before":       item.BeforeSnapshotJSON,
		"after":        item.AfterSnapshotJSON,
		"reason":       item.Reason,
		"operatorId":   item.OperatorID,
		"operatorName": item.OperatorName,
		"message":      item.Message,
		"durationMs":   item.DurationMs,
	})
}

func messageAuditExpectedHash(item *MQMessageAudit) string {
	if item == nil {
		return ""
	}
	return auditContentHash(map[string]any{
		"previous":          item.PreviousAuditHash,
		"instanceId":        item.InstanceID,
		"mqType":            item.MQType,
		"resourceType":      item.ResourceType,
		"resourceName":      item.ResourceName,
		"namespace":         item.Namespace,
		"action":            item.Action,
		"sampleCount":       item.SampleCount,
		"payloadBytes":      item.PayloadBytes,
		"filter":            item.FilterJSON,
		"payloadHash":       item.PayloadHash,
		"sensitiveHitCount": item.SensitiveHitCount,
		"rawPayloadVisible": item.RawPayloadVisible,
		"dlp":               item.DLPResultJSON,
		"status":            item.Status,
		"operatorId":        item.OperatorID,
		"operatorName":      item.OperatorName,
		"message":           item.Message,
	})
}

func auditContentHash(value map[string]any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return sha256Text(string(data))
}

func appendAuditChainFinding(result *AuditChainVerifyVO, auditID uint, severity, message string) {
	if result == nil {
		return
	}
	result.Valid = false
	result.BrokenCount++
	result.Findings = append(result.Findings, &AuditChainFindingVO{AuditID: auditID, Severity: severity, Message: message})
}

func fillAuditChainRange(result *AuditChainVerifyVO, count int, hashAt func(int) string) {
	if result == nil || count == 0 {
		return
	}
	result.HeadHash = hashAt(0)
	result.TailHash = hashAt(count - 1)
}

func buildProposedCloneConfig(instance *MQInstance, resource *MQResource, req *ConfigClonePlanRequest) map[string]any {
	result := resourceSnapshot(resource)
	result["targetNamespace"] = req.TargetNamespace
	result["targetResourceName"] = req.TargetResourceName
	result["config"] = parseJSONMap(resource.ConfigJSON)
	result["metadata"] = parseJSONMap(resource.MetadataJSON)
	if req.IncludeGovernanceFields && instance != nil {
		result["governance"] = map[string]any{
			"environment":    instance.Environment,
			"businessSystem": instance.BusinessSystem,
			"owner":          instance.Owner,
			"tags":           instance.Tags,
			"remark":         instance.Remark,
		}
	}
	return result
}

func configCloneWarnings(source, target *MQInstance, targetResource *MQResource) ([]string, string) {
	warnings := []string{
		"当前仅生成配置克隆计划，不执行目标集群变更，不迁移消息内容",
		"跨集群配置克隆前需要确认目标集群版本、权限、容量和业务幂等约束",
	}
	risk := RiskLevelMedium
	if source != nil && target != nil && NormalizeType(source.MQType) != NormalizeType(target.MQType) {
		warnings = append(warnings, "源实例和目标实例 MQ 类型不同，配置只能作为人工参考")
		risk = RiskLevelHigh
	}
	if targetResource != nil {
		warnings = append(warnings, "目标资源已存在，计划会展示配置差异，实际变更前必须再次预检")
	}
	if (source != nil && isProductionEnvironment(source.Environment)) || (target != nil && isProductionEnvironment(target.Environment)) {
		warnings = append(warnings, "生产环境配置克隆建议走审批和维护窗口")
		risk = RiskLevelHigh
	}
	return warnings, risk
}

func configCloneSuggestions(mqType, resourceType string, targetExists bool) []string {
	switch NormalizeType(mqType) {
	case MQTypeKafka:
		if resourceType == ResourceTypeTopic {
			if targetExists {
				return []string{"先执行 topic 配置 diff 预览", "仅通过白名单配置更新目标 topic", "分区扩容需要单独确认 key 顺序影响"}
			}
			return []string{"使用 Kafka topic 创建模板重建配置", "确认 replication.factor 与目标 broker 数匹配", "创建后立即同步元数据并绑定 owner"}
		}
	case MQTypeRabbitMQ:
		if resourceType == ResourceTypeQueue || resourceType == ResourceTypeExchange {
			return []string{"优先通过 queue/exchange 创建流程重放配置", "binding 需要单独确认源和目标是否存在", "不要通过删除重建修改不可变字段"}
		}
	case MQTypePulsar:
		return []string{"先确认 tenant/namespace 存在", "retention/TTL 降低可能升级为高危", "topic 删除或 cursor 操作不属于配置克隆范围"}
	}
	return []string{"目标适配器未提供自动克隆动作，建议按计划人工复核后分步执行"}
}

func cloneActionSuggestion(mqType, resourceType string, targetExists bool) string {
	switch NormalizeType(mqType) {
	case MQTypeKafka:
		if resourceType == ResourceTypeTopic {
			if targetExists {
				return OperationActionKafkaTopicConfigUpdate
			}
			return OperationActionKafkaTopicCreate
		}
	case MQTypeRabbitMQ:
		if resourceType == ResourceTypeQueue {
			return OperationActionRabbitMQQueueUpsert
		}
		if resourceType == ResourceTypeExchange {
			return OperationActionRabbitMQExchangeUpsert
		}
	case MQTypePulsar:
		if resourceType == ResourceTypeNamespace {
			return OperationActionPulsarRetentionUpdate
		}
	}
	return ""
}

func capacityRiskAndMessage(result *CapacityForecastVO) (string, string) {
	if result == nil {
		return RiskLevelLow, ""
	}
	if result.SampleCount < 2 {
		return RiskLevelMedium, "指标样本不足，预测基于当前元数据，建议先连续采集指标"
	}
	if result.ProjectedBacklog >= 100000 || result.ProjectedLag >= 100000 || result.BacklogGrowthPerHour >= 10000 || result.LagGrowthPerHour >= 10000 {
		return RiskLevelHigh, "预测周期内堆积或延迟可能快速扩大，需要尽快扩容或处理消费者瓶颈"
	}
	if result.ProjectedBacklog > result.CurrentBacklog || result.ProjectedLag > result.CurrentLag || result.CurrentBacklog > 0 || result.CurrentLag > 0 {
		return RiskLevelMedium, "存在堆积或延迟风险，建议跟踪 Top 资源并补充告警阈值"
	}
	return RiskLevelLow, "当前没有明显容量增长风险"
}

func buildCapacityRecommendations(instance *MQInstance, result *CapacityForecastVO) []*CapacityRecommendationVO {
	items := make([]*CapacityRecommendationVO, 0)
	add := func(severity, category, title, suggestion, metric string) {
		items = append(items, &CapacityRecommendationVO{Severity: severity, Category: category, Title: title, Suggestion: suggestion, Metric: metric})
	}
	if result.SampleCount < 2 {
		add("warning", "metric", "指标样本不足", "先连续采集 1 分钟或 5 分钟指标，再判断容量趋势", fmt.Sprintf("samples=%d", result.SampleCount))
	}
	if result.BacklogGrowthPerHour > 0 {
		add("warning", "capacity", "Backlog 持续增长", "检查消费者吞吐、分区热点和下游依赖，必要时扩容消费者或分区", fmt.Sprintf("%.2f/h", result.BacklogGrowthPerHour))
	}
	if result.LagGrowthPerHour > 0 {
		add("warning", "consumer", "Lag 持续增长", "优先排查消费组 offset 是否停滞、rebalance 是否频繁、单分区是否热点", fmt.Sprintf("%.2f/h", result.LagGrowthPerHour))
	}
	if len(result.TopBacklogResources) > 0 {
		add("info", "governance", "存在 Top Backlog 资源", "为 Top Backlog 资源绑定 owner、业务系统和单独告警阈值", result.TopBacklogResources[0].ResourceName)
	}
	if instance != nil && isProductionEnvironment(instance.Environment) && result.RiskLevel != RiskLevelLow {
		add("warning", "change", "生产容量风险", "生产环境建议先生成巡检报告，确认变更窗口后再执行扩容或配置调整", instance.Name)
	}
	if len(items) == 0 {
		add("info", "baseline", "保持容量基线", "继续保留定时指标快照，用于后续容量预测和成本分析", "")
	}
	return items
}

func nonNegativeInt64(value float64) int64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int64(math.Round(value))
}

func prettyJSON(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func jsonValueType(value any) string {
	switch item := value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		if math.Trunc(item) == item {
			return "integer"
		}
		return "number"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func jsonTypeMatches(value any, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual := jsonValueType(value)
	if expected == actual {
		return true
	}
	if expected == "number" && actual == "integer" {
		return true
	}
	return false
}

func jsonExample(value any) string {
	switch item := value.(type) {
	case map[string]any, []any:
		return ""
	case string:
		return trimText(item, 80)
	case nil:
		return ""
	default:
		return trimText(fmt.Sprint(value), 80)
	}
}

func pathHasSensitiveName(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return false
	}
	return isSensitiveFieldName(parts[len(parts)-1])
}

func schemaString(schema map[string]any, key string) string {
	if schema == nil {
		return ""
	}
	value, _ := schema[key].(string)
	return strings.TrimSpace(value)
}

func schemaStringSlice(schema map[string]any, key string) []string {
	raw, ok := schema[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func schemaMap(schema map[string]any, key string) map[string]any {
	raw, ok := schema[key].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return raw
}

func schemaIssue(path, severity, message string) *MessageSchemaValidationIssueVO {
	return &MessageSchemaValidationIssueVO{Path: path, Severity: severity, Message: message}
}

func hasSchemaError(issues []*MessageSchemaValidationIssueVO) bool {
	for _, item := range issues {
		if item != nil && item.Severity == "error" {
			return true
		}
	}
	return false
}

func isProductionEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production", "prd", "生产", "生产环境":
		return true
	default:
		return false
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
