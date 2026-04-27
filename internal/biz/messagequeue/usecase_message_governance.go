package messagequeue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	mqDLQResourceScanPageSize = 200
	mqDLQResourceScanMaxPages = 20
)

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|access_token|refresh_token|authorization|api[_-]?key)\s*[:=]\s*["']?[^,"'\s}]+`)
var emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
var phonePattern = regexp.MustCompile(`\b1[3-9]\d{9}\b`)

type messageAuditMeta struct {
	PayloadHash       string   `json:"payloadHash,omitempty"`
	SensitiveHitCount int      `json:"sensitiveHitCount,omitempty"`
	SensitiveFields   []string `json:"sensitiveFields,omitempty"`
	Redacted          bool     `json:"redacted"`
}

func (uc *UseCase) GetDLQAnalysis(ctx context.Context, req *DLQAnalysisRequest) (*DLQAnalysisVO, error) {
	if req == nil {
		req = &DLQAnalysisRequest{}
	}
	normalizeDLQAnalysisRequest(req)
	instances, err := uc.listDLQAnalysisInstances(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([]*DLQResourceIssueVO, 0)
	for _, instance := range instances {
		resources, err := uc.scanDLQResources(ctx, instance.ID, req)
		if err != nil {
			return nil, err
		}
		for _, resource := range resources {
			if resource == nil || !isDLQOrRetry(resource.Name) {
				continue
			}
			if req.Kind != "" && dlqResourceKind(resource.Name) != req.Kind {
				continue
			}
			if req.HasBacklog == "true" && resource.Backlog <= 0 {
				continue
			}
			if !matchesDLQKeyword(instance, resource, req.Keyword) {
				continue
			}
			result = append(result, uc.buildDLQIssue(ctx, instance, resource))
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Severity != result[j].Severity {
			return dlqSeverityRank(result[i].Severity) > dlqSeverityRank(result[j].Severity)
		}
		if result[i].Backlog != result[j].Backlog {
			return result[i].Backlog > result[j].Backlog
		}
		return result[i].ResourceName < result[j].ResourceName
	})
	report := &DLQAnalysisVO{
		Total:         int64(len(result)),
		Page:          req.Page,
		PageSize:      req.PageSize,
		Items:         paginateDLQIssues(result, req.Page, req.PageSize),
		ReplayEnabled: false,
		GeneratedAt:   nowText(),
	}
	for _, item := range result {
		if item.Kind == "dlq" {
			report.Summary.DLQTotal++
		}
		if item.Kind == "retry" {
			report.Summary.RetryTotal++
		}
		report.Summary.BacklogTotal += item.Backlog
		if item.ConsumerCount == 0 {
			report.Summary.NoConsumerTotal++
		}
		report.Summary.ReplayRequestable++
	}
	return report, nil
}

func (uc *UseCase) PrepareMessageReplayApplication(ctx context.Context, instanceID uint, req *MessageReplayApplicationRequest, operator Operator) (*MessageReplayPlanVO, error) {
	if req == nil {
		return nil, fmt.Errorf("重放申请参数不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("MQ实例不存在")
	}
	normalizeReplayApplicationRequest(req)
	resource := uc.findReplaySourceResource(ctx, instance.ID, req)
	warnings := []string{
		"当前仅创建重放申请评估，不会执行真实消息重放",
		"生产重放需要审批、限速、小批量试跑和业务幂等确认",
	}
	if resource != nil && !isDLQOrRetry(resource.Name) {
		warnings = append(warnings, "源资源未命中 DLQ/Retry 命名规则，请确认是否确实需要重放")
	}
	if resource == nil {
		warnings = append(warnings, "未在当前元数据中找到源资源，影响评估仅基于申请参数")
	}
	if req.TargetResourceName == "" {
		req.TargetResourceName = relatedOriginalResourceName(req.ResourceName)
	}
	impact := map[string]any{
		"sourceResourceName": req.ResourceName,
		"targetResourceName": req.TargetResourceName,
		"maxMessages":        req.MaxMessages,
		"rateLimitPerSecond": req.RateLimitPerSecond,
		"requiresApproval":   true,
		"replayExecutable":   false,
	}
	if resource != nil {
		impact["backlog"] = resource.Backlog
		impact["messageCount"] = resource.MessageCount
		impact["consumerCount"] = resource.ConsumerCount
		impact["resourceType"] = resource.ResourceType
	}
	auditID := uc.createMessageReplayAudit(ctx, instance, req, impact, operator)
	return &MessageReplayPlanVO{
		InstanceID:         instance.ID,
		InstanceName:       instance.Name,
		MQType:             instance.MQType,
		ResourceType:       firstNonEmpty(req.ResourceType, resourceTypeOf(resource), ResourceTypeTopic),
		Namespace:          req.Namespace,
		ResourceName:       req.ResourceName,
		TargetInstanceID:   firstNonZero(req.TargetInstanceID, instance.ID),
		TargetResourceName: req.TargetResourceName,
		MaxMessages:        req.MaxMessages,
		RateLimitPerSecond: req.RateLimitPerSecond,
		Status:             AuditStatusPending,
		ReplayExecutable:   false,
		RequiresApproval:   true,
		Warnings:           warnings,
		Impact:             impact,
		Message:            "重放申请评估已生成，当前版本不会直接执行消息重放",
		AuditID:            auditID,
		CreatedAt:          time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func applyMessageSampleDLP(result *MessageSampleResultVO) messageAuditMeta {
	meta := messageAuditMeta{}
	if result == nil {
		return meta
	}
	hash := sha256.New()
	fieldSet := map[string]struct{}{}
	for i := range result.Samples {
		sample := &result.Samples[i]
		hash.Write([]byte(sample.Payload))
		hash.Write([]byte{0})
		sample.PayloadHash = sha256Text(sample.Payload)
		redacted, hits, fields := redactPayload(sample.Payload)
		if hits > 0 {
			sample.Payload = redacted
			sample.Redacted = true
			sample.SensitiveHitCount = hits
			sample.SensitiveFields = fields
			meta.Redacted = true
			meta.SensitiveHitCount += hits
			for _, field := range fields {
				fieldSet[field] = struct{}{}
			}
		}
	}
	meta.PayloadHash = hex.EncodeToString(hash.Sum(nil))
	for field := range fieldSet {
		meta.SensitiveFields = append(meta.SensitiveFields, field)
	}
	sort.Strings(meta.SensitiveFields)
	result.PayloadHash = meta.PayloadHash
	result.SensitiveHitCount = meta.SensitiveHitCount
	result.Redacted = meta.Redacted
	return meta
}

func redactPayload(payload string) (string, int, []string) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return payload, 0, nil
	}
	var value any
	if json.Unmarshal([]byte(payload), &value) == nil {
		fields := make([]string, 0)
		hits := redactJSONValue(value, "$", &fields)
		if hits > 0 {
			data, err := json.MarshalIndent(value, "", "  ")
			if err == nil {
				return string(data), hits, uniqueSorted(fields)
			}
		}
	}
	redacted := payload
	hits := 0
	fields := make([]string, 0)
	redacted = sensitiveKeyPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		hits++
		key := strings.TrimSpace(strings.Split(strings.NewReplacer(":", "=", " ", "=").Replace(match), "=")[0])
		fields = append(fields, strings.ToLower(key))
		pair := strings.SplitN(match, "=", 2)
		if len(pair) == 2 {
			return pair[0] + "=***"
		}
		pair = strings.SplitN(match, ":", 2)
		if len(pair) == 2 {
			return pair[0] + ":***"
		}
		return "***"
	})
	redacted = emailPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		hits++
		fields = append(fields, "email")
		return maskEmail(match)
	})
	redacted = phonePattern.ReplaceAllStringFunc(redacted, func(match string) string {
		hits++
		fields = append(fields, "phone")
		return maskPhone(match)
	})
	return redacted, hits, uniqueSorted(fields)
}

func redactJSONValue(value any, path string, fields *[]string) int {
	hits := 0
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			childPath := path + "." + key
			if isSensitiveFieldName(key) {
				item[key] = "***"
				*fields = append(*fields, childPath)
				hits++
				continue
			}
			if text, ok := child.(string); ok {
				redacted, textHits, textFields := redactPayloadText(text)
				if textHits > 0 {
					item[key] = redacted
					*fields = append(*fields, textFields...)
					hits += textHits
					continue
				}
			}
			hits += redactJSONValue(child, childPath, fields)
		}
	case []any:
		for i, child := range item {
			if text, ok := child.(string); ok {
				redacted, textHits, textFields := redactPayloadText(text)
				if textHits > 0 {
					item[i] = redacted
					*fields = append(*fields, textFields...)
					hits += textHits
					continue
				}
			}
			hits += redactJSONValue(child, fmt.Sprintf("%s[%d]", path, i), fields)
		}
	}
	return hits
}

func redactPayloadText(value string) (string, int, []string) {
	hits := 0
	fields := make([]string, 0)
	redacted := emailPattern.ReplaceAllStringFunc(value, func(match string) string {
		hits++
		fields = append(fields, "email")
		return maskEmail(match)
	})
	redacted = phonePattern.ReplaceAllStringFunc(redacted, func(match string) string {
		hits++
		fields = append(fields, "phone")
		return maskPhone(match)
	})
	return redacted, hits, uniqueSorted(fields)
}

func isSensitiveFieldName(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "password", "passwd", "secret", "token", "access_token", "refresh_token", "authorization", "phone", "mobile", "email", "id_card", "idcard", "bank_card", "card_no", "credential", "private_key", "api_key", "apikey":
		return true
	default:
		return strings.Contains(normalized, "password") || strings.Contains(normalized, "token") || strings.Contains(normalized, "secret")
	}
}

func (uc *UseCase) listDLQAnalysisInstances(ctx context.Context, req *DLQAnalysisRequest) ([]*MQInstance, error) {
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("MQ实例仓库未初始化")
	}
	listReq := &InstanceListRequest{
		Page:              1,
		PageSize:          1000,
		RestrictToAllowed: req.RestrictToAllowed,
		AllowedIDs:        req.AllowedIDs,
	}
	if req.InstanceID > 0 {
		instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
		if err != nil {
			return nil, err
		}
		if req.RestrictToAllowed && !containsUint(req.AllowedIDs, req.InstanceID) {
			return []*MQInstance{}, nil
		}
		return []*MQInstance{instance}, nil
	}
	items, _, err := uc.instanceRepo.List(ctx, listReq)
	return items, err
}

func (uc *UseCase) scanDLQResources(ctx context.Context, instanceID uint, req *DLQAnalysisRequest) ([]*MQResource, error) {
	if uc.resourceRepo == nil {
		return nil, nil
	}
	var result []*MQResource
	for page := 1; page <= mqDLQResourceScanMaxPages; page++ {
		items, _, err := uc.resourceRepo.List(ctx, instanceID, &ResourceListRequest{Page: page, PageSize: mqDLQResourceScanPageSize})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item != nil && isDLQOrRetry(item.Name) {
				result = append(result, item)
			}
		}
		if len(items) < mqDLQResourceScanPageSize {
			break
		}
	}
	return result, nil
}

func (uc *UseCase) buildDLQIssue(ctx context.Context, instance *MQInstance, resource *MQResource) *DLQResourceIssueVO {
	consumerGroups := uc.dlqConsumerGroups(ctx, resource)
	recentAudits := uc.recentMessageAudits(ctx, instance.ID, resource.Name, 5)
	issue := &DLQResourceIssueVO{
		InstanceID:          instance.ID,
		InstanceName:        instance.Name,
		MQType:              instance.MQType,
		MQTypeText:          TypeText(instance.MQType),
		Environment:         instance.Environment,
		BusinessSystem:      instance.BusinessSystem,
		Owner:               instance.Owner,
		ResourceID:          resource.ID,
		ResourceType:        resource.ResourceType,
		Namespace:           resource.Namespace,
		ResourceName:        resource.Name,
		FullName:            resource.FullName,
		Kind:                dlqResourceKind(resource.Name),
		Severity:            dlqIssueSeverity(resource),
		MessageCount:        resource.MessageCount,
		Backlog:             resource.Backlog,
		ConsumerCount:       resource.ConsumerCount,
		ProducedRate:        resource.ProducedRate,
		ConsumedRate:        resource.ConsumedRate,
		RelatedResourceName: relatedOriginalResourceName(resource.Name),
		HandlingStatus:      "untriaged",
		Suggestion:          dlqSuggestion(resource),
		LastSyncAt:          formatTimePtr(resource.LastSyncAt),
		ConsumerGroups:      consumerGroups,
		RecentAudits:        recentAudits,
	}
	issue.ErrorSummary = buildDLQErrorSummary(issue, recentAudits)
	return issue
}

func (uc *UseCase) dlqConsumerGroups(ctx context.Context, resource *MQResource) []*ConsumerGroupVO {
	if uc.consumerGroupRepo == nil || resource == nil {
		return nil
	}
	items, _, err := uc.consumerGroupRepo.List(ctx, resource.InstanceID, &ConsumerGroupListRequest{Page: 1, PageSize: 5, ResourceName: resource.Name})
	if err != nil {
		return nil
	}
	result := make([]*ConsumerGroupVO, 0, len(items))
	for _, item := range items {
		result = append(result, toConsumerGroupVO(item))
	}
	return result
}

func (uc *UseCase) recentMessageAudits(ctx context.Context, instanceID uint, resourceName string, limit int) []*AuditVO {
	if uc.messageAuditRepo == nil || limit <= 0 {
		return nil
	}
	items, _, err := uc.messageAuditRepo.List(ctx, &AuditListRequest{Page: 1, PageSize: limit, InstanceID: instanceID, Keyword: resourceName})
	if err != nil {
		return nil
	}
	result := make([]*AuditVO, 0, len(items))
	for _, item := range items {
		result = append(result, messageAuditToVO(item))
	}
	return result
}

func (uc *UseCase) findReplaySourceResource(ctx context.Context, instanceID uint, req *MessageReplayApplicationRequest) *MQResource {
	if uc.resourceRepo == nil || req == nil {
		return nil
	}
	resourceTypes := []string{req.ResourceType}
	if strings.TrimSpace(req.ResourceType) == "" {
		resourceTypes = []string{ResourceTypeTopic, ResourceTypeQueue}
	}
	for _, resourceType := range resourceTypes {
		item, err := uc.resourceRepo.GetByUnique(ctx, instanceID, resourceType, req.Namespace, req.ResourceName)
		if err == nil && item != nil {
			return item
		}
	}
	items, _, err := uc.resourceRepo.List(ctx, instanceID, &ResourceListRequest{Page: 1, PageSize: 10, Keyword: req.ResourceName})
	if err != nil {
		return nil
	}
	for _, item := range items {
		if item != nil && (item.Name == req.ResourceName || item.FullName == req.ResourceName) {
			return item
		}
	}
	return nil
}

func (uc *UseCase) createMessageReplayAudit(ctx context.Context, instance *MQInstance, req *MessageReplayApplicationRequest, impact map[string]any, operator Operator) uint {
	if uc.messageAuditRepo == nil || instance == nil || req == nil {
		return 0
	}
	audit := &MQMessageAudit{
		InstanceID:        instance.ID,
		MQType:            instance.MQType,
		ResourceType:      firstNonEmpty(req.ResourceType, ResourceTypeTopic),
		ResourceName:      req.ResourceName,
		Namespace:         req.Namespace,
		Action:            AuditActionMessageReplay,
		FilterJSON:        mustJSON(req),
		DLPResultJSON:     mustJSON(map[string]any{"impact": impact, "rawPayloadStored": false}),
		Status:            AuditStatusPending,
		OperatorID:        operator.ID,
		OperatorName:      operator.Username,
		ClientIP:          operator.ClientIP,
		Message:           "消息重放申请评估已创建，等待审批工作流接入后执行",
		RawPayloadVisible: false,
	}
	if err := uc.messageAuditRepo.Create(ctx, audit); err != nil {
		return 0
	}
	return audit.ID
}

func normalizeDLQAnalysisRequest(req *DLQAnalysisRequest) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if req.Kind != "dlq" && req.Kind != "retry" {
		req.Kind = ""
	}
	req.HasBacklog = strings.ToLower(strings.TrimSpace(req.HasBacklog))
	req.Keyword = strings.TrimSpace(req.Keyword)
}

func normalizeReplayApplicationRequest(req *MessageReplayApplicationRequest) {
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.TargetResourceName = strings.TrimSpace(req.TargetResourceName)
	if req.TargetInstanceID == 0 {
		req.TargetInstanceID = 0
	}
	if req.MaxMessages <= 0 {
		req.MaxMessages = 100
	}
	if req.RateLimitPerSecond <= 0 {
		req.RateLimitPerSecond = 10
	}
}

func paginateDLQIssues(items []*DLQResourceIssueVO, page, pageSize int) []*DLQResourceIssueVO {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []*DLQResourceIssueVO{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func buildDLQErrorSummary(issue *DLQResourceIssueVO, audits []*AuditVO) []*DLQErrorSummaryVO {
	result := make([]*DLQErrorSummaryVO, 0)
	add := func(kind string, count int64, desc string) {
		if count > 0 {
			result = append(result, &DLQErrorSummaryVO{Type: kind, Count: count, Description: desc})
		}
	}
	add("backlog", issue.Backlog, "资源存在未处理积压")
	if issue.ConsumerCount == 0 {
		add("no_consumer", 1, "当前没有消费者处理该资源")
	}
	if issue.ProducedRate > issue.ConsumedRate && issue.Backlog > 0 {
		add("consumer_lagging", 1, "生产速率高于消费速率，积压可能继续增长")
	}
	var failedAudits int64
	var sensitiveHits int64
	for _, audit := range audits {
		if audit.Status == AuditStatusFailed {
			failedAudits++
		}
		sensitiveHits += int64(audit.SensitiveHitCount)
	}
	add("sample_failed", failedAudits, "近期消息采样失败，需要确认权限或连接状态")
	add("sensitive_payload", sensitiveHits, "近期采样命中敏感字段，处理时必须脱敏")
	return result
}

func dlqResourceKind(name string) string {
	if isDLQName(name) {
		return "dlq"
	}
	return "retry"
}

func dlqIssueSeverity(resource *MQResource) string {
	switch {
	case resource.Backlog >= mqCriticalBacklogThreshold:
		return "critical"
	case resource.Backlog >= mqWarningBacklogThreshold || resource.ConsumerCount == 0:
		return "warning"
	default:
		return "info"
	}
}

func dlqSuggestion(resource *MQResource) string {
	if resource.Backlog > 0 && resource.ConsumerCount == 0 {
		return "先确认负责人和消费程序状态，再按审批流程评估小批量重放或人工处理"
	}
	if resource.Backlog > 0 {
		return "按错误类型归类样本，确认业务幂等后再申请限速重放"
	}
	return "保持定期巡检，避免 DLQ/Retry 资源长期无人认领"
}

func relatedOriginalResourceName(name string) string {
	value := strings.TrimSpace(name)
	lower := strings.ToLower(value)
	replacements := []string{".dead-letter", "-dead-letter", "_dead_letter", ".deadletter", "-deadletter", "_deadletter", ".dlq", "-dlq", "_dlq", ".retry", "-retry", "_retry", "%retry%"}
	for _, suffix := range replacements {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSuffix(value, value[len(value)-len(suffix):])
		}
	}
	prefixes := []string{"dlq.", "dlq-", "retry.", "retry-", "%retry%"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return value[len(prefix):]
		}
	}
	return value
}

func matchesDLQKeyword(instance *MQInstance, resource *MQResource, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	values := []string{instance.Name, instance.MQType, instance.Environment, instance.BusinessSystem, instance.Owner, resource.Name, resource.FullName, resource.Namespace}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}

func dlqSeverityRank(severity string) int {
	switch severity {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func resourceTypeOf(resource *MQResource) string {
	if resource == nil {
		return ""
	}
	return resource.ResourceType
}

func firstNonZero(values ...uint) uint {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func uniqueSorted(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func maskEmail(value string) string {
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 || len(parts[0]) == 0 {
		return "***"
	}
	prefix := parts[0]
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return prefix + "***@" + parts[1]
}

func maskPhone(value string) string {
	if len(value) < 7 {
		return "***"
	}
	return value[:3] + "****" + value[len(value)-4:]
}
