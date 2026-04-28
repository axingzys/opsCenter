package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	overviewStatusSuccess = "success"
	overviewStatusInfo    = "info"
	overviewStatusWarning = "warning"
	overviewStatusDanger  = "danger"
)

type DatabaseOverviewRequest struct {
	Environment            string `form:"environment"`
	RestrictToAllowed      bool   `form:"-" json:"-"`
	AllowedInstanceIDs     []uint `form:"-" json:"-"`
	PermissionMode         string `form:"-" json:"-"`
	PermissionRulesEnabled bool   `form:"-" json:"-"`
	PermissionModeEnforced bool   `form:"-" json:"-"`
}

type DatabaseOverviewMetric struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Value       int64  `json:"value"`
	ValueText   string `json:"valueText,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	Link        string `json:"link,omitempty"`
}

type DatabaseOverviewTodoItem struct {
	Key          string `json:"key"`
	Severity     string `json:"severity"`
	SeverityText string `json:"severityText"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	InstanceID   uint   `json:"instanceId,omitempty"`
	InstanceName string `json:"instanceName,omitempty"`
	Link         string `json:"link,omitempty"`
}

type DatabaseOverviewVO struct {
	GeneratedAt        string                      `json:"generatedAt"`
	PermissionMode     string                      `json:"permissionMode"`
	PermissionModeText string                      `json:"permissionModeText"`
	InstanceSummary    []*DatabaseOverviewMetric   `json:"instanceSummary"`
	PermissionSummary  []*DatabaseOverviewMetric   `json:"permissionSummary"`
	SQLRiskSummary     []*DatabaseOverviewMetric   `json:"sqlRiskSummary"`
	BackupSummary      []*DatabaseOverviewMetric   `json:"backupSummary"`
	InspectionSummary  []*DatabaseOverviewMetric   `json:"inspectionSummary"`
	TodoItems          []*DatabaseOverviewTodoItem `json:"todoItems"`
}

func (uc *UseCase) GetOverview(ctx context.Context, req *DatabaseOverviewRequest) (*DatabaseOverviewVO, error) {
	if uc == nil || uc.instanceRepo == nil {
		return nil, fmt.Errorf("数据库实例仓库未配置")
	}
	if req == nil {
		req = &DatabaseOverviewRequest{}
	}
	req.Environment = strings.TrimSpace(req.Environment)
	req.PermissionMode = normalizeOverviewPermissionMode(req.PermissionMode)

	now := time.Now()
	instances, _, err := uc.instanceRepo.List(ctx, &DatabaseInstanceListRequest{
		Page:              1,
		PageSize:          10000,
		Environment:       req.Environment,
		RestrictToAllowed: req.RestrictToAllowed,
		AllowedIDs:        req.AllowedInstanceIDs,
	})
	if err != nil {
		return nil, err
	}

	audits, _ := uc.listOverviewAudits(ctx, req, now.Add(-24*time.Hour))
	backupTasks, _ := uc.listOverviewBackupTasks(ctx, req)
	backupRecords, _ := uc.listOverviewBackupRecords(ctx, req, now.AddDate(0, 0, -30))
	inspectionReports, _ := uc.listOverviewInspectionReports(ctx, req)

	instanceStats := buildOverviewInstanceStats(instances, now)
	backupStats := buildOverviewBackupStats(instances, backupTasks, backupRecords, now)
	sqlStats := buildOverviewSQLStats(audits)
	inspectionStats := buildOverviewInspectionStats(instances, inspectionReports)

	overview := &DatabaseOverviewVO{
		GeneratedAt:        now.Format("2006-01-02 15:04:05"),
		PermissionMode:     req.PermissionMode,
		PermissionModeText: overviewPermissionModeText(req.PermissionMode),
		InstanceSummary:    buildOverviewInstanceMetrics(instanceStats),
		PermissionSummary:  buildOverviewPermissionMetrics(req),
		SQLRiskSummary:     buildOverviewSQLMetrics(sqlStats),
		BackupSummary:      buildOverviewBackupMetrics(backupStats),
		InspectionSummary:  buildOverviewInspectionMetrics(inspectionStats),
	}
	overview.TodoItems = buildOverviewTodoItems(req, instanceStats, sqlStats, backupStats, inspectionStats)
	return overview, nil
}

func (uc *UseCase) listOverviewAudits(ctx context.Context, req *DatabaseOverviewRequest, since time.Time) ([]*DatabaseQueryAudit, error) {
	if uc == nil || uc.auditRepo == nil {
		return nil, nil
	}
	items, _, err := uc.auditRepo.List(ctx, &DatabaseQueryAuditListRequest{
		Page:               1,
		PageSize:           5000,
		StartTime:          since.Format("2006-01-02 15:04:05"),
		RestrictToAllowed:  req.RestrictToAllowed,
		AllowedInstanceIDs: req.AllowedInstanceIDs,
	})
	return items, err
}

func (uc *UseCase) listOverviewBackupTasks(ctx context.Context, req *DatabaseOverviewRequest) ([]*DatabaseBackupTask, error) {
	if uc == nil || uc.backupTaskRepo == nil {
		return nil, nil
	}
	items, _, err := uc.backupTaskRepo.List(ctx, &DatabaseBackupTaskListRequest{
		Page:               1,
		PageSize:           5000,
		RestrictToAllowed:  req.RestrictToAllowed,
		AllowedInstanceIDs: req.AllowedInstanceIDs,
	})
	return items, err
}

func (uc *UseCase) listOverviewBackupRecords(ctx context.Context, req *DatabaseOverviewRequest, since time.Time) ([]*DatabaseBackupRecord, error) {
	if uc == nil || uc.backupRecordRepo == nil {
		return nil, nil
	}
	items, _, err := uc.backupRecordRepo.List(ctx, &DatabaseBackupRecordListRequest{
		Page:               1,
		PageSize:           5000,
		DateFrom:           since.Format("2006-01-02 15:04:05"),
		RestrictToAllowed:  req.RestrictToAllowed,
		AllowedInstanceIDs: req.AllowedInstanceIDs,
	})
	return items, err
}

func (uc *UseCase) listOverviewInspectionReports(ctx context.Context, req *DatabaseOverviewRequest) ([]*DatabaseInspectionReport, error) {
	if uc == nil || uc.inspectionReportRepo == nil {
		return nil, nil
	}
	items, _, err := uc.inspectionReportRepo.List(ctx, &DatabaseInspectionReportListRequest{
		Page:               1,
		PageSize:           5000,
		RestrictToAllowed:  req.RestrictToAllowed,
		AllowedInstanceIDs: req.AllowedInstanceIDs,
	})
	return items, err
}

type overviewInstanceStats struct {
	total       int64
	enabled     int64
	disabled    int64
	production  int64
	staleSync   int64
	neverTested int64
}

func buildOverviewInstanceStats(instances []*DatabaseInstance, now time.Time) overviewInstanceStats {
	cutoff := now.AddDate(0, 0, -7)
	var stats overviewInstanceStats
	for _, item := range instances {
		if item == nil {
			continue
		}
		stats.total++
		if strings.TrimSpace(item.Status) == DatabaseInstanceStatusDisabled {
			stats.disabled++
		} else {
			stats.enabled++
		}
		if isProductionEnvironment(item.Environment) {
			stats.production++
		}
		if item.LastSyncAt == nil || item.LastSyncAt.Before(cutoff) {
			stats.staleSync++
		}
		if item.LastTestAt == nil {
			stats.neverTested++
		}
	}
	return stats
}

type overviewSQLStats struct {
	total        int64
	denied       int64
	failed       int64
	highRisk     int64
	writeActions int64
}

func buildOverviewSQLStats(audits []*DatabaseQueryAudit) overviewSQLStats {
	var stats overviewSQLStats
	for _, item := range audits {
		if item == nil {
			continue
		}
		stats.total++
		switch strings.TrimSpace(item.Status) {
		case DatabaseQueryStatusDenied:
			stats.denied++
		case DatabaseQueryStatusFailed:
			stats.failed++
		}
		if isHighOverviewRisk(item.RiskLevel) {
			stats.highRisk++
		}
		if isDatabaseChangeAuditAction(item.AuditAction) {
			stats.writeActions++
		}
	}
	return stats
}

func isDatabaseChangeAuditAction(action string) bool {
	switch normalizeAuditAction(action) {
	case DatabaseAuditActionChangeExecute, DatabaseAuditActionDDLExecute:
		return true
	default:
		return false
	}
}

type overviewBackupStats struct {
	taskTotal              int64
	enabledTaskTotal       int64
	supportedInstances     int64
	instancesWithoutTask   int64
	noRecentSuccess        int64
	recentFailedRecords    int64
	verifyFailedRecords    int64
	verifyPendingRecords   int64
	restoreUntestedRecords int64
}

func buildOverviewBackupStats(instances []*DatabaseInstance, tasks []*DatabaseBackupTask, records []*DatabaseBackupRecord, now time.Time) overviewBackupStats {
	taskInstanceIDs := make(map[uint]struct{})
	recentSuccessInstanceIDs := make(map[uint]struct{})
	cutoff := now.AddDate(0, 0, -7)
	var stats overviewBackupStats

	for _, task := range tasks {
		if task == nil {
			continue
		}
		stats.taskTotal++
		taskInstanceIDs[task.InstanceID] = struct{}{}
		if task.Enabled {
			stats.enabledTaskTotal++
		}
	}

	for _, record := range records {
		if record == nil {
			continue
		}
		if record.Status == DatabaseBackupStatusFailed && record.CreatedAt.After(cutoff) {
			stats.recentFailedRecords++
		}
		if record.Status != DatabaseBackupStatusSuccess {
			continue
		}
		if record.CreatedAt.After(cutoff) || (record.FinishedAt != nil && record.FinishedAt.After(cutoff)) {
			recentSuccessInstanceIDs[record.InstanceID] = struct{}{}
		}
		switch strings.TrimSpace(record.VerifyStatus) {
		case DatabaseBackupVerifyStatusFailed:
			stats.verifyFailedRecords++
		case "", DatabaseBackupVerifyStatusPending:
			stats.verifyPendingRecords++
		}
		if strings.TrimSpace(record.FileName) != "" && strings.TrimSpace(record.RestoreTestStatus) != DatabaseBackupStatusSuccess {
			stats.restoreUntestedRecords++
		}
	}

	for _, item := range instances {
		if item == nil || strings.TrimSpace(item.Status) == DatabaseInstanceStatusDisabled || !supportsBackupTask(item.DBType) {
			continue
		}
		stats.supportedInstances++
		if _, ok := taskInstanceIDs[item.ID]; !ok {
			stats.instancesWithoutTask++
			continue
		}
		if _, ok := recentSuccessInstanceIDs[item.ID]; !ok {
			stats.noRecentSuccess++
		}
	}
	return stats
}

type overviewInspectionStats struct {
	reportTotal            int64
	failedReports          int64
	highRiskReports        int64
	lowScoreReports        int64
	instancesWithoutReport int64
}

func buildOverviewInspectionStats(instances []*DatabaseInstance, reports []*DatabaseInspectionReport) overviewInspectionStats {
	latestByInstance := make(map[uint]*DatabaseInspectionReport)
	var stats overviewInspectionStats
	for _, report := range reports {
		if report == nil {
			continue
		}
		stats.reportTotal++
		if strings.TrimSpace(report.Status) == DatabaseBackupStatusFailed {
			stats.failedReports++
		}
		if isHighOverviewRisk(report.RiskLevel) {
			stats.highRiskReports++
		}
		if report.Status == DatabaseBackupStatusSuccess && report.HealthScore > 0 && report.HealthScore < 80 {
			stats.lowScoreReports++
		}
		if existing := latestByInstance[report.InstanceID]; existing == nil || report.CreatedAt.After(existing.CreatedAt) {
			latestByInstance[report.InstanceID] = report
		}
	}
	for _, item := range instances {
		if item == nil || strings.TrimSpace(item.Status) == DatabaseInstanceStatusDisabled {
			continue
		}
		if latestByInstance[item.ID] == nil {
			stats.instancesWithoutReport++
		}
	}
	return stats
}

func buildOverviewInstanceMetrics(stats overviewInstanceStats) []*DatabaseOverviewMetric {
	return []*DatabaseOverviewMetric{
		overviewMetric("instance_total", "实例总数", stats.total, overviewStatusInfo, "当前可见数据库实例数", "instances"),
		overviewMetric("instance_enabled", "启用实例", stats.enabled, overviewStatusSuccess, "处于启用状态的实例", "instances"),
		overviewMetric("instance_prod", "生产实例", stats.production, overviewStatusWarning, "环境标记为生产的实例", "instances"),
		overviewMetric("instance_stale_sync", "结构未同步", stats.staleSync, overviewCountStatus(stats.staleSync), "从未同步或超过 7 天未同步结构", "schemas"),
		overviewMetric("instance_never_tested", "未测试连接", stats.neverTested, overviewCountStatus(stats.neverTested), "从未执行连接测试的实例", "instances"),
	}
}

func buildOverviewPermissionMetrics(req *DatabaseOverviewRequest) []*DatabaseOverviewMetric {
	modeStatus := overviewStatusWarning
	if req.PermissionMode == "whitelist" {
		modeStatus = overviewStatusSuccess
	}
	rulesStatus := overviewStatusWarning
	rulesValue := int64(0)
	rulesText := "未配置"
	if req.PermissionRulesEnabled {
		rulesStatus = overviewStatusSuccess
		rulesValue = 1
		rulesText = "已配置"
	}
	enforcedStatus := overviewStatusWarning
	enforcedValue := int64(0)
	enforcedText := "未收敛"
	if req.PermissionModeEnforced {
		enforcedStatus = overviewStatusSuccess
		enforcedValue = 1
		enforcedText = "已收敛"
	}
	return []*DatabaseOverviewMetric{
		overviewTextMetric("permission_mode", "权限模式", overviewPermissionModeText(req.PermissionMode), modeStatus, "实例对象权限模式", "permissions"),
		overviewTextMetricWithValue("permission_rules", "授权规则", rulesValue, rulesText, rulesStatus, "是否已配置实例级授权规则", "permissions"),
		overviewTextMetricWithValue("permission_enforced", "权限收敛", enforcedValue, enforcedText, enforcedStatus, "非 admin 用户是否按实例权限收敛", "permissions"),
	}
}

func buildOverviewSQLMetrics(stats overviewSQLStats) []*DatabaseOverviewMetric {
	return []*DatabaseOverviewMetric{
		overviewMetric("sql_total_24h", "24h 审计", stats.total, overviewStatusInfo, "最近 24 小时数据库审计动作", "audit"),
		overviewMetric("sql_denied_24h", "24h 拦截", stats.denied, overviewCountStatus(stats.denied), "最近 24 小时被拦截的 SQL / 高危动作", "audit"),
		overviewMetric("sql_failed_24h", "24h 失败", stats.failed, overviewCountStatus(stats.failed), "最近 24 小时执行失败的数据库动作", "audit"),
		overviewMetric("sql_high_risk_24h", "24h 高风险", stats.highRisk, overviewCountStatus(stats.highRisk), "最近 24 小时高 / 严重风险动作", "audit"),
		overviewMetric("sql_write_24h", "24h 变更", stats.writeActions, overviewCountStatus(stats.writeActions), "最近 24 小时受控 DML / DDL 变更动作", "audit"),
	}
}

func buildOverviewBackupMetrics(stats overviewBackupStats) []*DatabaseOverviewMetric {
	return []*DatabaseOverviewMetric{
		overviewMetric("backup_task_total", "备份任务", stats.taskTotal, overviewStatusInfo, "当前备份任务总数", "backup"),
		overviewMetric("backup_supported_instances", "可备份实例", stats.supportedInstances, overviewStatusInfo, "启用且已接入备份能力的实例", "backup"),
		overviewMetric("backup_without_task", "未配置备份", stats.instancesWithoutTask, overviewCountStatus(stats.instancesWithoutTask), "可备份实例中未配置备份任务的数量", "backup"),
		overviewMetric("backup_no_recent_success", "7天无成功备份", stats.noRecentSuccess, overviewCountStatus(stats.noRecentSuccess), "已配置任务但最近 7 天没有成功备份的实例", "backup"),
		overviewMetric("backup_failed_recent", "7天备份失败", stats.recentFailedRecords, overviewCountStatus(stats.recentFailedRecords), "最近 7 天失败的备份记录", "backup"),
		overviewMetric("backup_verify_failed", "校验失败", stats.verifyFailedRecords, overviewCountStatus(stats.verifyFailedRecords), "最近备份记录中校验失败的数量", "backup"),
		overviewMetric("backup_restore_untested", "未恢复演练", stats.restoreUntestedRecords, overviewCountStatus(stats.restoreUntestedRecords), "成功备份但尚未恢复演练成功的记录", "backup"),
	}
}

func buildOverviewInspectionMetrics(stats overviewInspectionStats) []*DatabaseOverviewMetric {
	return []*DatabaseOverviewMetric{
		overviewMetric("inspection_total", "巡检报告", stats.reportTotal, overviewStatusInfo, "当前可见巡检报告数", "inspection"),
		overviewMetric("inspection_without_report", "未巡检实例", stats.instancesWithoutReport, overviewCountStatus(stats.instancesWithoutReport), "启用实例中从未生成巡检报告的数量", "inspection"),
		overviewMetric("inspection_failed", "巡检失败", stats.failedReports, overviewCountStatus(stats.failedReports), "失败的巡检报告数量", "inspection"),
		overviewMetric("inspection_high_risk", "高风险报告", stats.highRiskReports, overviewCountStatus(stats.highRiskReports), "风险等级为高或严重的巡检报告", "inspection"),
		overviewMetric("inspection_low_score", "健康分低", stats.lowScoreReports, overviewCountStatus(stats.lowScoreReports), "健康分低于 80 的成功巡检报告", "inspection"),
	}
}

func buildOverviewTodoItems(req *DatabaseOverviewRequest, instances overviewInstanceStats, sql overviewSQLStats, backup overviewBackupStats, inspection overviewInspectionStats) []*DatabaseOverviewTodoItem {
	items := make([]*DatabaseOverviewTodoItem, 0)
	if req.PermissionMode != "whitelist" && !req.PermissionRulesEnabled {
		items = append(items, overviewTodo("permission_not_enforced", DatabaseQueryRiskHigh, "实例权限未形成白名单收敛", "当前为兼容模式且没有授权规则，非 admin 用户不会按实例级权限收敛。", "permissions"))
	}
	if instances.staleSync > 0 {
		items = append(items, overviewTodo("instance_stale_sync", DatabaseQueryRiskMedium, "处理结构同步滞后的实例", fmt.Sprintf("%d 个实例从未同步或超过 7 天未同步结构。", instances.staleSync), "schemas"))
	}
	if backup.instancesWithoutTask > 0 {
		items = append(items, overviewTodo("backup_without_task", DatabaseQueryRiskHigh, "为可备份实例配置备份任务", fmt.Sprintf("%d 个启用实例已支持备份但尚未配置备份任务。", backup.instancesWithoutTask), "backup"))
	}
	if backup.noRecentSuccess > 0 {
		items = append(items, overviewTodo("backup_no_recent_success", DatabaseQueryRiskHigh, "检查最近无成功备份的实例", fmt.Sprintf("%d 个实例最近 7 天没有成功备份记录。", backup.noRecentSuccess), "backup"))
	}
	if backup.recentFailedRecords > 0 || backup.verifyFailedRecords > 0 {
		items = append(items, overviewTodo("backup_failed", DatabaseQueryRiskHigh, "处理备份失败或校验失败记录", fmt.Sprintf("最近 7 天备份失败 %d 条，文件校验失败 %d 条。", backup.recentFailedRecords, backup.verifyFailedRecords), "backup"))
	}
	if backup.restoreUntestedRecords > 0 {
		items = append(items, overviewTodo("backup_restore_untested", DatabaseQueryRiskMedium, "补齐备份恢复演练", fmt.Sprintf("%d 条成功备份尚未恢复演练成功。", backup.restoreUntestedRecords), "backup"))
	}
	if sql.denied > 0 || sql.failed > 0 || sql.highRisk > 0 {
		items = append(items, overviewTodo("sql_risk_24h", DatabaseQueryRiskMedium, "复核最近 24 小时 SQL 风险", fmt.Sprintf("拦截 %d 条，失败 %d 条，高风险 %d 条。", sql.denied, sql.failed, sql.highRisk), "audit"))
	}
	if inspection.instancesWithoutReport > 0 {
		items = append(items, overviewTodo("inspection_without_report", DatabaseQueryRiskMedium, "为启用实例生成巡检报告", fmt.Sprintf("%d 个启用实例尚未生成巡检报告。", inspection.instancesWithoutReport), "inspection"))
	}
	if inspection.failedReports > 0 || inspection.highRiskReports > 0 || inspection.lowScoreReports > 0 {
		items = append(items, overviewTodo("inspection_risk", DatabaseQueryRiskHigh, "处理巡检高风险项", fmt.Sprintf("失败报告 %d 份，高风险报告 %d 份，健康分低报告 %d 份。", inspection.failedReports, inspection.highRiskReports, inspection.lowScoreReports), "inspection"))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return overviewSeverityRank(items[i].Severity) > overviewSeverityRank(items[j].Severity)
	})
	return items
}

func overviewMetric(key, label string, value int64, status, description, link string) *DatabaseOverviewMetric {
	return &DatabaseOverviewMetric{
		Key:         key,
		Label:       label,
		Value:       value,
		Status:      status,
		Description: description,
		Link:        link,
	}
}

func overviewTextMetric(key, label, valueText, status, description, link string) *DatabaseOverviewMetric {
	return overviewTextMetricWithValue(key, label, 0, valueText, status, description, link)
}

func overviewTextMetricWithValue(key, label string, value int64, valueText, status, description, link string) *DatabaseOverviewMetric {
	return &DatabaseOverviewMetric{
		Key:         key,
		Label:       label,
		Value:       value,
		ValueText:   valueText,
		Status:      status,
		Description: description,
		Link:        link,
	}
}

func overviewTodo(key, severity, title, description, link string) *DatabaseOverviewTodoItem {
	return &DatabaseOverviewTodoItem{
		Key:          key,
		Severity:     severity,
		SeverityText: QueryRiskLevelText(severity),
		Title:        title,
		Description:  description,
		Link:         link,
	}
}

func overviewCountStatus(value int64) string {
	if value > 0 {
		return overviewStatusWarning
	}
	return overviewStatusSuccess
}

func isHighOverviewRisk(riskLevel string) bool {
	switch strings.TrimSpace(riskLevel) {
	case DatabaseQueryRiskHigh, DatabaseQueryRiskCritical:
		return true
	default:
		return false
	}
}

func normalizeOverviewPermissionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "whitelist":
		return "whitelist"
	default:
		return "compat"
	}
}

func overviewPermissionModeText(mode string) string {
	if normalizeOverviewPermissionMode(mode) == "whitelist" {
		return "白名单模式"
	}
	return "兼容模式"
}

func overviewSeverityRank(severity string) int {
	switch strings.TrimSpace(severity) {
	case DatabaseQueryRiskCritical:
		return 4
	case DatabaseQueryRiskHigh:
		return 3
	case DatabaseQueryRiskMedium:
		return 2
	case DatabaseQueryRiskLow:
		return 1
	default:
		return 0
	}
}
