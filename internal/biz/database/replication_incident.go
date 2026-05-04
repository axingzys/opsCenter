package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ReplicaIncidentTypeDelete  = "delete"
	ReplicaIncidentTypeUpdate  = "update"
	ReplicaIncidentTypeRelease = "release"
	ReplicaIncidentTypeOther   = "other"

	ReplicaIncidentRecoveryExportBackfill = "export_backfill"
	ReplicaIncidentRecoveryFullRollback   = "full_rollback"
	ReplicaIncidentRecoveryUnknown        = "unknown"

	ReplicaIncidentStatusGenerated = "generated"
)

func (uc *UseCase) CreateReplicaIncidentGuide(ctx context.Context, req *DatabaseReplicaIncidentGuideRequest, operator QueryOperator) (*DatabaseReplicaIncidentGuideVO, error) {
	if uc.instanceRepo == nil || uc.replicaIncidentRepo == nil {
		return nil, errors.New("事故指引仓储未配置")
	}
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil || instance == nil {
		return nil, errors.New("数据库实例不存在")
	}
	if !isReplicaGovernanceEngine(instance.DBType) {
		return nil, fmt.Errorf("%s 暂不支持副本事故指引", DBTypeText(instance.DBType))
	}
	incidentType := normalizeReplicaIncidentType(req.IncidentType)
	if incidentType == "" {
		return nil, errors.New("事故类型不支持")
	}
	affectedSummary := strings.TrimSpace(req.AffectedSummary)
	if affectedSummary == "" {
		return nil, errors.New("影响范围或 SQL 摘要不能为空")
	}
	reason := strings.TrimSpace(req.IncidentReason)
	if reason == "" {
		return nil, errors.New("事故原因不能为空")
	}
	if !req.ConfirmNoAutoPause {
		return nil, errors.New("必须确认 P4.3 只生成指引，不自动暂停 apply")
	}
	incidentTime, err := parseReplicaIncidentTime(req.IncidentTime)
	if err != nil {
		return nil, err
	}

	protection := uc.loadIncidentProtection(ctx, instance.ID)
	preferredCheck := uc.loadIncidentPreferredCheck(ctx, protection)
	canIntercept := replicaIncidentCanIntercept(protection, preferredCheck)
	guideJSON := buildReplicaIncidentGuideJSON(instance, protection, preferredCheck, incidentType, affectedSummary, reason, normalizeReplicaIncidentRecovery(req.ExpectedRecoveryMethod), incidentTime, canIntercept)
	guideMarkdown := buildReplicaIncidentMarkdown(instance, protection, preferredCheck, incidentType, affectedSummary, reason, normalizeReplicaIncidentRecovery(req.ExpectedRecoveryMethod), incidentTime, canIntercept, operator)

	item := &DatabaseReplicaIncidentGuide{
		InstanceID:             instance.ID,
		IncidentTime:           incidentTime,
		IncidentType:           incidentType,
		AffectedSummary:        trimText(affectedSummary, 4000),
		IncidentReason:         trimText(reason, 4000),
		ExpectedRecoveryMethod: normalizeReplicaIncidentRecovery(req.ExpectedRecoveryMethod),
		CanIntercept:           canIntercept,
		RemainingDelaySeconds:  -1,
		GuideMarkdown:          trimText(guideMarkdown, 60000),
		GuideJSON:              trimText(guideJSON, 60000),
		Status:                 ReplicaIncidentStatusGenerated,
		OperatorID:             operator.ID,
		OperatorName:           trimText(operator.Username, 100),
		ClientIP:               trimText(operator.ClientIP, 64),
	}
	if protection != nil {
		item.PreferredReplicaID = protection.PreferredReplicaID
		item.PreferredReplicaInstanceID = protection.PreferredReplicaInstanceID
		item.PreferredCheckID = protection.LastCheckID
		item.RemainingDelaySeconds = protection.RemainingDelaySeconds
	}
	if preferredCheck != nil && item.PreferredCheckID == 0 {
		item.PreferredCheckID = preferredCheck.ID
	}
	if err := uc.replicaIncidentRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	uc.recordReplicaIncidentAudit(ctx, instance, item, operator)
	return uc.toReplicaIncidentGuideVO(ctx, item), nil
}

func (uc *UseCase) ListReplicaIncidentGuides(ctx context.Context, req *DatabaseReplicaIncidentGuideListRequest) ([]*DatabaseReplicaIncidentGuideVO, int64, error) {
	if uc.replicaIncidentRepo == nil {
		return nil, 0, errors.New("事故指引仓储未配置")
	}
	items, total, err := uc.replicaIncidentRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*DatabaseReplicaIncidentGuideVO, 0, len(items))
	for _, item := range items {
		result = append(result, uc.toReplicaIncidentGuideVO(ctx, item))
	}
	return result, total, nil
}

func (uc *UseCase) GetReplicaIncidentGuide(ctx context.Context, id uint) (*DatabaseReplicaIncidentGuideVO, error) {
	if uc.replicaIncidentRepo == nil {
		return nil, errors.New("事故指引仓储未配置")
	}
	item, err := uc.replicaIncidentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("事故指引不存在")
	}
	return uc.toReplicaIncidentGuideVO(ctx, item), nil
}

func (uc *UseCase) DeleteReplicaIncidentGuide(ctx context.Context, id uint, operator QueryOperator) error {
	if uc.replicaIncidentRepo == nil {
		return errors.New("事故指引仓储未配置")
	}
	if id == 0 {
		return errors.New("事故指引ID不能为空")
	}
	item, err := uc.replicaIncidentRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("事故指引不存在")
	}
	if err := uc.replicaIncidentRepo.Delete(ctx, id); err != nil {
		return err
	}
	uc.recordReplicaIncidentDeleteAudit(ctx, item, operator)
	return nil
}

func (uc *UseCase) loadIncidentProtection(ctx context.Context, instanceID uint) *DatabaseReplicaProtectionVO {
	list, _, err := uc.ListReplicaProtections(ctx, &DatabaseReplicaProtectionListRequest{
		Page:       1,
		PageSize:   1,
		InstanceID: instanceID,
	})
	if err != nil || len(list) == 0 {
		return nil
	}
	return list[0]
}

func (uc *UseCase) loadIncidentPreferredCheck(ctx context.Context, protection *DatabaseReplicaProtectionVO) *DatabaseReplicationCheck {
	if protection == nil || protection.LastCheckID == 0 || uc.replicationCheckRepo == nil {
		return nil
	}
	check, err := uc.replicationCheckRepo.GetByID(ctx, protection.LastCheckID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return check
}

func replicaIncidentCanIntercept(protection *DatabaseReplicaProtectionVO, check *DatabaseReplicationCheck) bool {
	if protection == nil || !protection.HasDelayedReplica || protection.PreferredReplicaInstanceID == 0 {
		return false
	}
	if protection.RiskLevel == DatabaseReplicaHealthCritical || strings.EqualFold(protection.PreferredReplicaStatus, DatabaseReplicaHealthCritical) {
		return false
	}
	if check == nil {
		return false
	}
	return protection.RemainingDelaySeconds > 0
}

func buildReplicaIncidentGuideJSON(
	instance *DatabaseInstance,
	protection *DatabaseReplicaProtectionVO,
	check *DatabaseReplicationCheck,
	incidentType, affectedSummary, reason, recovery string,
	incidentTime *time.Time,
	canIntercept bool,
) string {
	payload := map[string]any{
		"instanceId":             instance.ID,
		"instanceName":           instance.Name,
		"engine":                 normalizeDBType(instance.DBType),
		"incidentTime":           formatTime(incidentTime),
		"incidentType":           incidentType,
		"incidentTypeText":       ReplicaIncidentTypeText(incidentType),
		"affectedSummary":        affectedSummary,
		"incidentReason":         reason,
		"expectedRecoveryMethod": recovery,
		"canIntercept":           canIntercept,
		"protection":             protection,
	}
	if check != nil {
		payload["preferredCheckId"] = check.ID
		payload["preferredCheckAt"] = formatTime(check.CheckedAt)
		payload["preferredCheckHealth"] = check.HealthStatus
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func buildReplicaIncidentMarkdown(
	instance *DatabaseInstance,
	protection *DatabaseReplicaProtectionVO,
	check *DatabaseReplicationCheck,
	incidentType, affectedSummary, reason, recovery string,
	incidentTime *time.Time,
	canIntercept bool,
	operator QueryOperator,
) string {
	var b strings.Builder
	add := func(format string, args ...any) {
		b.WriteString(fmt.Sprintf(format, args...))
		b.WriteByte('\n')
	}

	add("# 数据库误操作事故指引")
	add("")
	add("## 事故信息")
	add("")
	add("- 事故实例：%s (#%d, %s, %s)", instance.Name, instance.ID, DBTypeText(instance.DBType), netJoinHostPort(instance.Host, instance.Port))
	add("- 事故时间：%s", firstNonEmpty(formatTime(incidentTime), "-"))
	add("- 事故类型：%s", ReplicaIncidentTypeText(incidentType))
	add("- 影响范围 / SQL 摘要：%s", affectedSummary)
	add("- 事故原因：%s", reason)
	add("- 期望恢复方式：%s", ReplicaIncidentRecoveryText(recovery))
	add("- 指引生成：%s / %s", firstNonEmpty(operator.Username, "-"), time.Now().Format("2006-01-02 15:04:05"))
	add("")
	add("## 当前判断")
	add("")
	if protection == nil {
		add("- 结论：未找到该实例的延迟副本保护窗口记录。")
		add("- 当前是否还有截停机会：否。")
		add("- 建议：立即转 PITR 兜底，选择事故前时间点恢复到隔离库。")
	} else {
		add("- 保护状态：%s", firstNonEmpty(protection.ProtectionStatusText, protection.ProtectionStatus, "-"))
		add("- 风险等级：%s", firstNonEmpty(protection.RiskLevelText, protection.RiskLevel, "-"))
		if protection.HasDelayedReplica {
			add("- 推荐延迟副本：%s (#%d, %s)", firstNonEmpty(protection.PreferredReplicaInstanceName, "-"), protection.PreferredReplicaInstanceID, firstNonEmpty(protection.PreferredReplicaEndpoint, "-"))
			add("- 最近检查：#%d / %s", protection.LastCheckID, firstNonEmpty(protection.LastCheckedAt, "-"))
			add("- 配置延迟：%s", secondsText(protection.ConfiguredDelaySeconds))
			add("- 当前 remaining delay：%s", secondsText(protection.RemainingDelaySeconds))
			add("- 当前 apply/replay 时间：%s", firstNonEmpty(protection.ApplyTime, "-"))
		} else {
			add("- 推荐延迟副本：无。")
		}
		if canIntercept {
			add("- 当前是否还有截停机会：可能有，需要立刻人工确认并在副本侧暂停 apply/replay。")
		} else {
			add("- 当前是否还有截停机会：否或未知，优先准备 PITR 兜底。")
		}
		if len(protection.RiskMessages) > 0 {
			add("- 风险提示：%s", strings.Join(protection.RiskMessages, "；"))
		}
	}
	if check != nil {
		add("- 关联 replication check：#%d，健康状态 %s。", check.ID, ReplicaHealthText(check.HealthStatus))
	}
	add("")
	add("## 只读安全边界")
	add("")
	add("- OpsHub P4.3 只生成指引和审计，不自动执行 pause、resume、promote、failover 或 switchover。")
	add("- 下面命令只是人工模板；执行前必须确认目标实例是 replica/standby，不能在 primary 上执行。")
	add("")
	add("## 暂停 apply/replay 命令模板")
	add("")
	add("### MySQL / MariaDB")
	add("")
	add("```sql")
	add("STOP REPLICA SQL_THREAD;")
	add("-- 旧版本兼容：")
	add("STOP SLAVE SQL_THREAD;")
	add("```")
	add("")
	add("### PostgreSQL")
	add("")
	add("```sql")
	add("SELECT pg_wal_replay_pause();")
	add("```")
	add("")
	add("## 推荐处置流程")
	add("")
	add("1. 立刻确认推荐延迟副本的最近检查时间、apply/replay 时间和 remaining delay。")
	add("2. 如果仍有截停机会，在副本主机或数据库控制台人工暂停 apply/replay，并记录操作人、时间和原因。")
	add("3. 使用只读连接检查事故 SQL 是否尚未应用到延迟副本。")
	add("4. 从延迟副本导出受影响表、行或业务对象。")
	add("5. 回填生产库前走 SQL 审批、预检和审计，避免二次污染。")
	add("6. 如果延迟副本不可用、已追上或无法证明一致性，转 PITR：选择事故前时间点，恢复到隔离库，执行校验 SQL，再导出缺失对象或准备整体切换。")
	return b.String()
}

func (uc *UseCase) toReplicaIncidentGuideVO(ctx context.Context, item *DatabaseReplicaIncidentGuide) *DatabaseReplicaIncidentGuideVO {
	if item == nil {
		return nil
	}
	instanceName, endpoint := uc.instanceNameEndpoint(ctx, item.InstanceID)
	var engine string
	if instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID); err == nil && instance != nil {
		engine = normalizeDBType(instance.DBType)
	}
	replicaName, replicaEndpoint := uc.instanceNameEndpoint(ctx, item.PreferredReplicaInstanceID)
	return &DatabaseReplicaIncidentGuideVO{
		ID:                         item.ID,
		InstanceID:                 item.InstanceID,
		InstanceName:               instanceName,
		InstanceEndpoint:           endpoint,
		Engine:                     engine,
		EngineText:                 DBTypeText(engine),
		IncidentTime:               formatTime(item.IncidentTime),
		IncidentType:               item.IncidentType,
		IncidentTypeText:           ReplicaIncidentTypeText(item.IncidentType),
		AffectedSummary:            item.AffectedSummary,
		IncidentReason:             item.IncidentReason,
		ExpectedRecoveryMethod:     item.ExpectedRecoveryMethod,
		ExpectedRecoveryMethodText: ReplicaIncidentRecoveryText(item.ExpectedRecoveryMethod),
		PreferredReplicaID:         item.PreferredReplicaID,
		PreferredReplicaInstanceID: item.PreferredReplicaInstanceID,
		PreferredReplicaName:       replicaName,
		PreferredReplicaEndpoint:   replicaEndpoint,
		PreferredCheckID:           item.PreferredCheckID,
		CanIntercept:               item.CanIntercept,
		RemainingDelaySeconds:      item.RemainingDelaySeconds,
		GuideMarkdown:              item.GuideMarkdown,
		GuideJSON:                  item.GuideJSON,
		Status:                     item.Status,
		StatusText:                 ReplicaIncidentStatusText(item.Status),
		OperatorID:                 item.OperatorID,
		OperatorName:               item.OperatorName,
		ClientIP:                   item.ClientIP,
		CreatedAt:                  formatTime(&item.CreatedAt),
		UpdatedAt:                  formatTime(&item.UpdatedAt),
	}
}

func (uc *UseCase) recordReplicaIncidentAudit(ctx context.Context, instance *DatabaseInstance, guide *DatabaseReplicaIncidentGuide, operator QueryOperator) {
	if uc == nil || uc.auditRepo == nil || instance == nil || guide == nil {
		return
	}
	auditText := fmt.Sprintf("replica incident guide #%d type=%s canIntercept=%t", guide.ID, guide.IncidentType, guide.CanIntercept)
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionReplicaIncident,
		SQLText:        trimText(auditText, 20000),
		SQLFingerprint: sqlFingerprint(auditText),
		SQLType:        "REPLICA_INCIDENT_GUIDE",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         DatabaseQueryStatusSuccess,
		RowsReturned:   1,
		ErrorMessage:   trimText(fmt.Sprintf("preferredReplica=%d preferredCheck=%d", guide.PreferredReplicaInstanceID, guide.PreferredCheckID), 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func (uc *UseCase) recordReplicaIncidentDeleteAudit(ctx context.Context, guide *DatabaseReplicaIncidentGuide, operator QueryOperator) {
	if uc == nil || uc.auditRepo == nil || guide == nil {
		return
	}
	auditText := fmt.Sprintf("delete replica incident guide #%d type=%s canIntercept=%t", guide.ID, guide.IncidentType, guide.CanIntercept)
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     guide.InstanceID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionReplicaIncidentDel,
		SQLText:        trimText(auditText, 20000),
		SQLFingerprint: sqlFingerprint(auditText),
		SQLType:        "REPLICA_INCIDENT_GUIDE_DELETE",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         DatabaseQueryStatusSuccess,
		RowsReturned:   1,
		ErrorMessage:   trimText(fmt.Sprintf("preferredReplica=%d preferredCheck=%d", guide.PreferredReplicaInstanceID, guide.PreferredCheckID), 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func parseReplicaIncidentTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		now := time.Now()
		return &now, nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	var parsed time.Time
	var err error
	for _, layout := range formats {
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("事故时间格式不正确")
}

func normalizeReplicaIncidentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReplicaIncidentTypeDelete, "误删":
		return ReplicaIncidentTypeDelete
	case ReplicaIncidentTypeUpdate, "误更新":
		return ReplicaIncidentTypeUpdate
	case ReplicaIncidentTypeRelease, "publish", "deploy", "错误发布":
		return ReplicaIncidentTypeRelease
	case ReplicaIncidentTypeOther, "其他":
		return ReplicaIncidentTypeOther
	default:
		return ""
	}
}

func normalizeReplicaIncidentRecovery(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReplicaIncidentRecoveryExportBackfill, "export", "backfill":
		return ReplicaIncidentRecoveryExportBackfill
	case ReplicaIncidentRecoveryFullRollback, "rollback":
		return ReplicaIncidentRecoveryFullRollback
	default:
		return ReplicaIncidentRecoveryUnknown
	}
}

func ReplicaIncidentTypeText(value string) string {
	switch normalizeReplicaIncidentType(value) {
	case ReplicaIncidentTypeDelete:
		return "误删"
	case ReplicaIncidentTypeUpdate:
		return "误更新"
	case ReplicaIncidentTypeRelease:
		return "错误发布"
	default:
		return "其他"
	}
}

func ReplicaIncidentRecoveryText(value string) string {
	switch normalizeReplicaIncidentRecovery(value) {
	case ReplicaIncidentRecoveryExportBackfill:
		return "导出回填"
	case ReplicaIncidentRecoveryFullRollback:
		return "整库回滚"
	default:
		return "暂不确定"
	}
}

func ReplicaIncidentStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case ReplicaIncidentStatusGenerated:
		return "已生成"
	default:
		return strings.TrimSpace(value)
	}
}

func secondsText(value int) string {
	if value < 0 {
		return "-"
	}
	if value < 60 {
		return fmt.Sprintf("%ds", value)
	}
	minutes := value / 60
	seconds := value % 60
	if minutes < 60 {
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	restMinutes := minutes % 60
	if restMinutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, restMinutes)
	}
	return fmt.Sprintf("%dh", hours)
}
