package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DatabaseInspectionReportListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	InstanceID         uint   `form:"instanceId"`
	Status             string `form:"status"`
	RiskLevel          string `form:"riskLevel"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseInspectionReportRequest struct {
	InstanceID uint `json:"instanceId" binding:"required"`
}

type DatabaseInspectionMetricVO struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

type DatabaseInspectionSectionVO struct {
	Key     string                        `json:"key"`
	Label   string                        `json:"label"`
	Status  string                        `json:"status"`
	Summary string                        `json:"summary"`
	Metrics []*DatabaseInspectionMetricVO `json:"metrics"`
}

type DatabaseInspectionFindingVO struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Message      string `json:"message"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
}

type DatabaseInspectionReportVO struct {
	ID                 uint                           `json:"id"`
	InstanceID         uint                           `json:"instanceId"`
	InstanceName       string                         `json:"instanceName"`
	DBType             string                         `json:"dbType"`
	DBTypeText         string                         `json:"dbTypeText"`
	ReportType         string                         `json:"reportType"`
	Status             string                         `json:"status"`
	StatusText         string                         `json:"statusText"`
	HealthScore        int                            `json:"healthScore"`
	RiskLevel          string                         `json:"riskLevel"`
	RiskLevelText      string                         `json:"riskLevelText"`
	Summary            string                         `json:"summary"`
	CapacitySummary    *DatabaseInspectionSectionVO   `json:"capacitySummary"`
	PerformanceSummary *DatabaseInspectionSectionVO   `json:"performanceSummary"`
	SecuritySummary    *DatabaseInspectionSectionVO   `json:"securitySummary"`
	BackupSummary      *DatabaseInspectionSectionVO   `json:"backupSummary"`
	Findings           []*DatabaseInspectionFindingVO `json:"findings"`
	OperatorID         uint                           `json:"operatorId"`
	OperatorName       string                         `json:"operatorName"`
	GeneratedAt        string                         `json:"generatedAt"`
	DurationMs         int64                          `json:"durationMs"`
	ErrorMessage       string                         `json:"errorMessage"`
	CreatedAt          string                         `json:"createdAt"`
	UpdatedAt          string                         `json:"updatedAt"`
}

func (uc *UseCase) ListInspectionReports(ctx context.Context, req *DatabaseInspectionReportListRequest) ([]*DatabaseInspectionReportVO, int64, error) {
	if uc.inspectionReportRepo == nil {
		return nil, 0, fmt.Errorf("巡检报告仓库未配置")
	}
	normalizeInspectionReportListRequest(req)
	items, total, err := uc.inspectionReportRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	instanceNames, instanceTypes := uc.loadInspectionReportInstanceMeta(ctx, items)
	list := make([]*DatabaseInspectionReportVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toInspectionReportVO(item, instanceNames[item.InstanceID], instanceTypes[item.InstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) GetInspectionReport(ctx context.Context, id uint) (*DatabaseInspectionReportVO, error) {
	if uc.inspectionReportRepo == nil {
		return nil, fmt.Errorf("巡检报告仓库未配置")
	}
	item, err := uc.inspectionReportRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("巡检报告不存在")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
	if err != nil || instance == nil {
		return uc.toInspectionReportVO(item, "", ""), nil
	}
	return uc.toInspectionReportVO(item, instance.Name, instance.DBType), nil
}

func (uc *UseCase) GetInspectionReportInstanceID(ctx context.Context, id uint) (uint, error) {
	if uc.inspectionReportRepo == nil {
		return 0, fmt.Errorf("巡检报告仓库未配置")
	}
	if id == 0 {
		return 0, fmt.Errorf("巡检报告ID不能为空")
	}
	item, err := uc.inspectionReportRepo.GetByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("巡检报告不存在")
	}
	return item.InstanceID, nil
}

func (uc *UseCase) GenerateInspectionReport(ctx context.Context, req *DatabaseInspectionReportRequest, operator QueryOperator) (*DatabaseInspectionReportVO, error) {
	if uc.inspectionReportRepo == nil {
		return nil, fmt.Errorf("巡检报告仓库未配置")
	}
	if req == nil || req.InstanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}

	start := time.Now()
	report := &DatabaseInspectionReport{
		InstanceID:   instance.ID,
		ReportType:   DatabaseInspectionReportManual,
		Status:       DatabaseBackupStatusRunning,
		OperatorID:   operator.ID,
		OperatorName: trimText(operator.Username, 100),
		GeneratedAt:  &start,
		ErrorMessage: "巡检报告生成中",
		HealthScore:  0,
		RiskLevel:    DatabaseQueryRiskLow,
	}
	if err := uc.inspectionReportRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("创建巡检报告失败: %w", err)
	}

	audit := uc.startInspectionAudit(ctx, instance, report, operator)
	sections, findings := uc.buildInspectionSections(ctx, instance)
	score, riskLevel := calculateInspectionScore(findings)
	finishedAt := time.Now()
	report.Status = DatabaseBackupStatusSuccess
	report.HealthScore = score
	report.RiskLevel = riskLevel
	report.Summary = buildInspectionSummary(score, riskLevel, findings)
	report.CapacitySummary = marshalInspectionSection(sections.capacity)
	report.PerformanceSummary = marshalInspectionSection(sections.performance)
	report.SecuritySummary = marshalInspectionSection(sections.security)
	report.BackupSummary = marshalInspectionSection(sections.backup)
	report.Findings = marshalInspectionFindings(findings)
	report.DurationMs = finishedAt.Sub(start).Milliseconds()
	report.ErrorMessage = ""
	if err := uc.inspectionReportRepo.Update(ctx, report); err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, report.DurationMs, err.Error())
		return nil, fmt.Errorf("更新巡检报告失败: %w", err)
	}
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, len(findings), report.DurationMs, "")
	return uc.toInspectionReportVO(report, instance.Name, instance.DBType), nil
}

type inspectionSections struct {
	capacity    *DatabaseInspectionSectionVO
	performance *DatabaseInspectionSectionVO
	security    *DatabaseInspectionSectionVO
	backup      *DatabaseInspectionSectionVO
}

func (uc *UseCase) buildInspectionSections(ctx context.Context, instance *DatabaseInstance) (inspectionSections, []*DatabaseInspectionFindingVO) {
	findings := make([]*DatabaseInspectionFindingVO, 0)
	capacity, capacityFindings := uc.buildCapacityInspectionSection(ctx, instance)
	findings = append(findings, capacityFindings...)
	performance, performanceFindings := uc.buildPerformanceInspectionSection(ctx, instance)
	findings = append(findings, performanceFindings...)
	security, securityFindings := uc.buildSecurityInspectionSection(ctx, instance)
	findings = append(findings, securityFindings...)
	backup, backupFindings := uc.buildBackupInspectionSection(ctx, instance)
	findings = append(findings, backupFindings...)
	return inspectionSections{
		capacity:    capacity,
		performance: performance,
		security:    security,
		backup:      backup,
	}, findings
}

func (uc *UseCase) buildCapacityInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.buildRedisCapacityInspectionSection(ctx, instance)
	}
	findings := make([]*DatabaseInspectionFindingVO, 0)
	if uc.capacitySnapshotRepo == nil {
		return inspectionSection("capacity", "容量", "warning", "容量快照仓库未配置"), append(findings, inspectionFinding("warning", "capacity", "容量快照不可用", "容量快照仓库未配置", "instance", fmt.Sprint(instance.ID)))
	}
	_, _ = uc.CollectCapacitySnapshot(ctx, instance.ID)
	latest, err := uc.capacitySnapshotRepo.LatestInstance(ctx, instance.ID)
	if err != nil || latest == nil {
		return inspectionSection("capacity", "容量", "warning", "暂无容量采样数据"), append(findings, inspectionFinding("warning", "capacity", "暂无容量采样", "请先同步元数据并等待容量采样", "instance", fmt.Sprint(instance.ID)))
	}
	points, _ := uc.capacitySnapshotRepo.ListInstanceTrend(ctx, instance.ID, time.Now().Add(-7*24*time.Hour))
	growthBytes, growthPercent := calculateCapacityGrowth(points)
	topTables, _ := uc.capacitySnapshotRepo.LatestTopObjects(ctx, instance.ID, DatabaseCapacityObjectTable, 5)
	section := inspectionSection("capacity", "容量", "success", "容量采样正常")
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "total_size", Label: "当前容量", Value: humanizeBytes(latest.TotalSizeBytes), Status: "success"},
		{Key: "growth_7d", Label: "近 7 天增长", Value: fmt.Sprintf("%s / %.2f%%", humanizeSignedBytes(growthBytes), growthPercent), Status: capacityGrowthStatus(growthBytes, growthPercent)},
		{Key: "schemas", Label: "Schema 数", Value: fmt.Sprintf("%d", latest.SchemaCount), Status: "info"},
		{Key: "tables", Label: "表数量", Value: fmt.Sprintf("%d", latest.TableCount), Status: "info"},
	}
	if growthBytes > 10*1024*1024*1024 || growthPercent >= 50 {
		findings = append(findings, inspectionFinding("warning", "capacity", "容量增长偏快", fmt.Sprintf("近 7 天增长 %s / %.2f%%", humanizeSignedBytes(growthBytes), growthPercent), "instance", fmt.Sprint(instance.ID)))
		section.Status = "warning"
		section.Summary = "容量增长偏快"
	}
	if len(topTables) > 0 && topTables[0].TotalSizeBytes > latest.TotalSizeBytes/2 && latest.TotalSizeBytes > 0 {
		findings = append(findings, inspectionFinding("info", "capacity", "单表容量占比较高", fmt.Sprintf("%s.%s 占实例容量超过 50%%", topTables[0].SchemaName, topTables[0].Table), "table", topTables[0].Table))
	}
	return section, findings
}

func (uc *UseCase) buildPerformanceInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.buildRedisPerformanceInspectionSection(ctx, instance)
	}
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("performance", "性能", "success", "性能诊断采集正常")
	if !isDiagnosableType(instance.DBType) {
		section.Status = "info"
		section.Summary = DBTypeText(instance.DBType) + " 性能诊断后续批次接入"
		return section, findings
	}
	credential, err := uc.resolveDiagnosableCredential(ctx, instance)
	if err != nil {
		section.Status = "warning"
		section.Summary = "性能诊断凭据不可用"
		return section, append(findings, inspectionFinding("warning", "performance", "性能诊断失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	cards, err := collectDatabaseMetrics(ctx, instance, credential)
	if err != nil {
		section.Status = "warning"
		section.Summary = "性能诊断采集失败"
		return section, append(findings, inspectionFinding("warning", "performance", "性能诊断失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	section.Metrics = make([]*DatabaseInspectionMetricVO, 0, len(cards))
	for _, card := range cards {
		if card == nil {
			continue
		}
		status := "info"
		if card.Key == "active_sessions" || card.Key == "connections" {
			status = "success"
		}
		section.Metrics = append(section.Metrics, &DatabaseInspectionMetricVO{
			Key:    card.Key,
			Label:  card.Label,
			Value:  card.Value,
			Status: status,
		})
	}
	return section, findings
}

func (uc *UseCase) buildSecurityInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.buildRedisSecurityInspectionSection(ctx, instance)
	}
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("security", "安全", "success", "SQL 风险审计正常")
	since := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	highRisk := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, RiskLevel: DatabaseQueryRiskHigh, StartTime: since})
	criticalRisk := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, RiskLevel: DatabaseQueryRiskCritical, StartTime: since})
	denied := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, Status: DatabaseQueryStatusDenied, StartTime: since})
	failedWrites := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, Action: DatabaseAuditActionChangeExecute, Status: DatabaseQueryStatusFailed, StartTime: since})
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "high_risk", Label: "高风险 SQL", Value: fmt.Sprintf("%d", highRisk+criticalRisk), Status: countStatus(highRisk + criticalRisk)},
		{Key: "denied", Label: "拦截次数", Value: fmt.Sprintf("%d", denied), Status: countStatus(denied)},
		{Key: "failed_writes", Label: "写操作失败", Value: fmt.Sprintf("%d", failedWrites), Status: countStatus(failedWrites)},
	}
	if criticalRisk > 0 {
		findings = append(findings, inspectionFinding("critical", "security", "存在严重风险 SQL", fmt.Sprintf("近 7 天严重风险 SQL %d 次", criticalRisk), "audit", "critical"))
		section.Status = "danger"
		section.Summary = "存在严重风险 SQL"
	} else if highRisk > 0 || denied > 0 || failedWrites > 0 {
		findings = append(findings, inspectionFinding("warning", "security", "存在 SQL 风险事件", fmt.Sprintf("近 7 天高风险 %d 次、拦截 %d 次、写失败 %d 次", highRisk, denied, failedWrites), "audit", "risk"))
		section.Status = "warning"
		section.Summary = "存在 SQL 风险事件"
	}
	return section, findings
}

func (uc *UseCase) buildBackupInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.buildRedisBackupInspectionSection(ctx, instance)
	}
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("backup", "备份", "success", "备份状态正常")
	since := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	latestSuccess, _ := uc.latestBackupRecord(ctx, instance.ID, DatabaseBackupStatusSuccess)
	failedCount := uc.countBackupRecords(ctx, &DatabaseBackupRecordListRequest{InstanceID: instance.ID, Status: DatabaseBackupStatusFailed, DateFrom: since})
	lastSuccessText := "-"
	if latestSuccess != nil {
		lastSuccessText = formatTime(latestSuccess.FinishedAt)
		if lastSuccessText == "" {
			lastSuccessText = latestSuccess.CreatedAt.Format("2006-01-02 15:04:05")
		}
	}
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "last_success", Label: "最近成功备份", Value: lastSuccessText, Status: backupSuccessStatus(latestSuccess)},
		{Key: "failed_7d", Label: "近 7 天失败", Value: fmt.Sprintf("%d", failedCount), Status: countStatus(failedCount)},
	}
	if latestSuccess == nil {
		findings = append(findings, inspectionFinding("warning", "backup", "缺少成功备份", "当前实例没有成功备份记录", "instance", fmt.Sprint(instance.ID)))
		section.Status = "warning"
		section.Summary = "缺少成功备份"
	} else if isBackupStale(latestSuccess, 7*24*time.Hour) {
		findings = append(findings, inspectionFinding("warning", "backup", "备份时间过旧", "最近成功备份超过 7 天", "backup_record", fmt.Sprint(latestSuccess.ID)))
		section.Status = "warning"
		section.Summary = "最近成功备份过旧"
	}
	if failedCount > 0 {
		findings = append(findings, inspectionFinding("warning", "backup", "存在备份失败", fmt.Sprintf("近 7 天备份失败 %d 次", failedCount), "backup_record", "failed"))
		section.Status = "warning"
	}
	return section, findings
}

func (uc *UseCase) buildRedisCapacityInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	findings := make([]*DatabaseInspectionFindingVO, 0)
	if uc.capacitySnapshotRepo == nil {
		return inspectionSection("capacity", "容量", "warning", "容量快照仓库未配置"), append(findings, inspectionFinding("warning", "capacity", "容量快照不可用", "容量快照仓库未配置", "instance", fmt.Sprint(instance.ID)))
	}
	_, _ = uc.CollectCapacitySnapshot(ctx, instance.ID)
	latest, err := uc.capacitySnapshotRepo.LatestInstance(ctx, instance.ID)
	if err != nil || latest == nil {
		return inspectionSection("capacity", "容量", "warning", "暂无 Redis 内存采样"), append(findings, inspectionFinding("warning", "capacity", "暂无 Redis 内存采样", "请先手动采集或等待后台采样", "instance", fmt.Sprint(instance.ID)))
	}
	points, _ := uc.capacitySnapshotRepo.ListInstanceTrend(ctx, instance.ID, time.Now().Add(-7*24*time.Hour))
	growthBytes, growthPercent := calculateCapacityGrowth(points)
	topKeys, _ := uc.capacitySnapshotRepo.LatestTopObjects(ctx, instance.ID, DatabaseCapacityObjectTable, 5)
	section := inspectionSection("capacity", "容量", "success", "Redis 内存采样正常")
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "used_memory", Label: "当前内存", Value: humanizeBytes(latest.TotalSizeBytes), Status: "success"},
		{Key: "growth_7d", Label: "近 7 天增长", Value: fmt.Sprintf("%s / %.2f%%", humanizeSignedBytes(growthBytes), growthPercent), Status: capacityGrowthStatus(growthBytes, growthPercent)},
		{Key: "logical_dbs", Label: "逻辑 DB 数", Value: fmt.Sprintf("%d", latest.SchemaCount), Status: "info"},
		{Key: "keys", Label: "Key 数", Value: fmt.Sprintf("%d", latest.TableCount), Status: "info"},
	}
	if growthBytes > 2*1024*1024*1024 || growthPercent >= 30 {
		findings = append(findings, inspectionFinding("warning", "capacity", "Redis 内存增长偏快", fmt.Sprintf("近 7 天增长 %s / %.2f%%", humanizeSignedBytes(growthBytes), growthPercent), "instance", fmt.Sprint(instance.ID)))
		section.Status = "warning"
		section.Summary = "Redis 内存增长偏快"
	}
	if len(topKeys) > 0 && latest.TotalSizeBytes > 0 && topKeys[0].TotalSizeBytes > latest.TotalSizeBytes/4 {
		findings = append(findings, inspectionFinding("info", "capacity", "存在大 Key 样本", fmt.Sprintf("%s.%s 占当前已用内存超过 25%%，结果基于采样", topKeys[0].SchemaName, topKeys[0].Table), "key", topKeys[0].Table))
	}
	return section, findings
}

func (uc *UseCase) buildRedisPerformanceInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("performance", "性能", "success", "Redis 诊断采集正常")
	credential, err := uc.resolveDiagnosableCredential(ctx, instance)
	if err != nil {
		section.Status = "warning"
		section.Summary = "Redis 诊断凭据不可用"
		return section, append(findings, inspectionFinding("warning", "performance", "Redis 诊断失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	cards, err := collectDatabaseMetrics(ctx, instance, credential)
	if err != nil {
		section.Status = "warning"
		section.Summary = "Redis 诊断采集失败"
		return section, append(findings, inspectionFinding("warning", "performance", "Redis 诊断失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	cardMap := make(map[string]string, len(cards))
	section.Metrics = make([]*DatabaseInspectionMetricVO, 0, len(cards))
	for _, card := range cards {
		if card == nil {
			continue
		}
		cardMap[card.Key] = card.Value
		section.Metrics = append(section.Metrics, &DatabaseInspectionMetricVO{
			Key:    card.Key,
			Label:  card.Label,
			Value:  card.Value,
			Status: redisInspectionMetricStatus(card.Key, card.Value),
		})
	}

	if state := strings.TrimSpace(cardMap["redis_cluster_state"]); state != "" && state != "正常" {
		findings = append(findings, inspectionFinding("critical", "performance", "Redis Cluster 状态异常", "当前 Cluster 状态不是正常", "instance", fmt.Sprint(instance.ID)))
		section.Status = "danger"
		section.Summary = "Redis Cluster 状态异常"
	}
	if pressure := parsePercentNumber(cardMap["redis_memory_pressure"]); pressure >= 95 {
		findings = append(findings, inspectionFinding("critical", "performance", "Redis 内存压力过高", fmt.Sprintf("当前内存压力 %.2f%%", pressure), "instance", fmt.Sprint(instance.ID)))
		section.Status = "danger"
		section.Summary = "Redis 内存压力过高"
	} else if pressure >= 85 {
		findings = append(findings, inspectionFinding("warning", "performance", "Redis 内存压力偏高", fmt.Sprintf("当前内存压力 %.2f%%", pressure), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "Redis 内存压力偏高"
		}
	}
	if fragmentation := parseFloatNumber(cardMap["redis_fragmentation"]); fragmentation >= 2 {
		findings = append(findings, inspectionFinding("warning", "performance", "Redis 内存碎片率偏高", fmt.Sprintf("当前碎片率 %.2f", fragmentation), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "Redis 内存碎片率偏高"
		}
	}
	if blocked := parseLeadingInt64(cardMap["redis_blocked_clients"]); blocked > 0 {
		findings = append(findings, inspectionFinding("warning", "performance", "存在阻塞客户端", fmt.Sprintf("当前阻塞客户端 %d 个", blocked), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "存在阻塞客户端"
		}
	}
	evicted, rejected := parsePairInt64(cardMap["redis_evicted_rejected"])
	if evicted > 0 || rejected > 0 {
		findings = append(findings, inspectionFinding("warning", "performance", "存在驱逐或拒绝连接", fmt.Sprintf("驱逐 %d 次，拒绝连接 %d 次", evicted, rejected), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "存在驱逐或拒绝连接"
		}
	}
	if lag := parseDurationSeconds(cardMap["redis_replica_lag"]); lag >= 30 {
		findings = append(findings, inspectionFinding("warning", "performance", "复制延迟偏高", fmt.Sprintf("最大复制延迟 %ds", lag), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "复制延迟偏高"
		}
	}
	return section, findings
}

func (uc *UseCase) buildRedisSecurityInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("security", "安全", "success", "Redis 命令审计正常")
	since := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	commandAudits := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, Action: DatabaseAuditActionQuery, StartTime: since})
	denied := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, Action: DatabaseAuditActionQuery, Status: DatabaseQueryStatusDenied, StartTime: since})
	failed := uc.countAudits(ctx, &DatabaseQueryAuditListRequest{InstanceID: instance.ID, Action: DatabaseAuditActionQuery, Status: DatabaseQueryStatusFailed, StartTime: since})
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "command_audits", Label: "近 7 天命令审计", Value: fmt.Sprintf("%d", commandAudits), Status: "info"},
		{Key: "denied", Label: "拦截次数", Value: fmt.Sprintf("%d", denied), Status: countStatus(denied)},
		{Key: "failed_commands", Label: "命令失败", Value: fmt.Sprintf("%d", failed), Status: countStatus(failed)},
	}
	if denied > 0 || failed > 0 {
		findings = append(findings, inspectionFinding("warning", "security", "存在 Redis 命令风险事件", fmt.Sprintf("近 7 天拦截 %d 次、命令失败 %d 次", denied, failed), "audit", "redis-command"))
		section.Status = "warning"
		section.Summary = "存在 Redis 命令风险事件"
	}
	return section, findings
}

func (uc *UseCase) buildRedisBackupInspectionSection(ctx context.Context, instance *DatabaseInstance) (*DatabaseInspectionSectionVO, []*DatabaseInspectionFindingVO) {
	findings := make([]*DatabaseInspectionFindingVO, 0)
	section := inspectionSection("backup", "备份", "success", "Redis 持久化配置正常")
	credential, err := uc.resolveDiagnosableCredential(ctx, instance)
	if err != nil {
		section.Status = "warning"
		section.Summary = "Redis 持久化巡检凭据不可用"
		return section, append(findings, inspectionFinding("warning", "backup", "Redis 持久化巡检失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	aggregate, err := collectRedisPersistenceAggregate(ctx, instance, credential)
	if err != nil {
		section.Status = "warning"
		section.Summary = "Redis 持久化巡检失败"
		return section, append(findings, inspectionFinding("warning", "backup", "Redis 持久化巡检失败", err.Error(), "instance", fmt.Sprint(instance.ID)))
	}
	section.Metrics = []*DatabaseInspectionMetricVO{
		{Key: "nodes", Label: "检查节点", Value: aggregate.nodeScopeText(), Status: "info"},
		{Key: "mode", Label: "持久化模式", Value: aggregate.persistenceModeText(), Status: redisPersistenceModeStatus(aggregate)},
		{Key: "rdb_last_save", Label: "最近 RDB 落盘", Value: aggregate.rdbSaveText(), Status: redisRDBSaveStatus(aggregate)},
		{Key: "aof_status", Label: "AOF 状态", Value: aggregate.aofStatusText(), Status: redisAOFStatus(aggregate)},
	}
	if aggregate.noPersistenceNodes > 0 {
		findings = append(findings, inspectionFinding("warning", "backup", "存在未启用持久化的节点", fmt.Sprintf("%d/%d 个节点未启用 AOF 和 RDB", aggregate.noPersistenceNodes, aggregate.inspectedNodes), "instance", fmt.Sprint(instance.ID)))
		section.Status = "warning"
		section.Summary = "存在未启用持久化的节点"
	}
	if aggregate.rdbFailureNodes > 0 || aggregate.aofRewriteFailureNodes > 0 || aggregate.aofWriteFailureNodes > 0 {
		findings = append(findings, inspectionFinding("warning", "backup", "最近持久化任务异常", fmt.Sprintf("RDB 失败 %d 节点，AOF rewrite 失败 %d 节点，AOF 写入失败 %d 节点", aggregate.rdbFailureNodes, aggregate.aofRewriteFailureNodes, aggregate.aofWriteFailureNodes), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "最近持久化任务异常"
		}
	}
	if aggregate.staleRDBOnlyNodes > 0 {
		findings = append(findings, inspectionFinding("warning", "backup", "RDB 落盘时间过旧", fmt.Sprintf("%d 个仅依赖 RDB 的节点最近成功落盘超过 24 小时或暂无记录", aggregate.staleRDBOnlyNodes), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Status = "warning"
			section.Summary = "RDB 落盘时间过旧"
		}
	}
	if aggregate.appendFsyncNoNodes > 0 {
		findings = append(findings, inspectionFinding("info", "backup", "AOF 落盘策略偏宽松", fmt.Sprintf("%d 个节点 appendfsync=no，异常宕机时可能丢失更多数据", aggregate.appendFsyncNoNodes), "instance", fmt.Sprint(instance.ID)))
		if section.Status == "success" {
			section.Summary = "AOF 落盘策略偏宽松"
		}
	}
	return section, findings
}

func (uc *UseCase) countAudits(ctx context.Context, req *DatabaseQueryAuditListRequest) int64 {
	if uc.auditRepo == nil {
		return 0
	}
	req.Page = 1
	req.PageSize = 1
	_, total, err := uc.auditRepo.List(ctx, req)
	if err != nil {
		return 0
	}
	return total
}

func (uc *UseCase) countBackupRecords(ctx context.Context, req *DatabaseBackupRecordListRequest) int64 {
	if uc.backupRecordRepo == nil {
		return 0
	}
	req.Page = 1
	req.PageSize = 1
	_, total, err := uc.backupRecordRepo.List(ctx, req)
	if err != nil {
		return 0
	}
	return total
}

func (uc *UseCase) latestBackupRecord(ctx context.Context, instanceID uint, status string) (*DatabaseBackupRecord, error) {
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	items, _, err := uc.backupRecordRepo.List(ctx, &DatabaseBackupRecordListRequest{
		Page:       1,
		PageSize:   1,
		InstanceID: instanceID,
		Status:     status,
	})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func (uc *UseCase) startInspectionAudit(ctx context.Context, instance *DatabaseInstance, report *DatabaseInspectionReport, operator QueryOperator) *DatabaseQueryAudit {
	if uc.auditRepo == nil || instance == nil {
		return nil
	}
	sqlText := fmt.Sprintf("GENERATE INSPECTION REPORT INSTANCE #%d", instance.ID)
	if report != nil && report.ID > 0 {
		sqlText = fmt.Sprintf("%s REPORT #%d", sqlText, report.ID)
	}
	audit := &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionInspectionGenerate,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "INSPECTION",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil
	}
	return audit
}

func (uc *UseCase) loadInspectionReportInstanceMeta(ctx context.Context, items []*DatabaseInspectionReport) (map[uint]string, map[uint]string) {
	instanceNames := make(map[uint]string)
	instanceTypes := make(map[uint]string)
	for _, item := range items {
		if item == nil || item.InstanceID == 0 {
			continue
		}
		if _, ok := instanceNames[item.InstanceID]; ok {
			continue
		}
		instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
		if err != nil || instance == nil {
			continue
		}
		instanceNames[item.InstanceID] = instance.Name
		instanceTypes[item.InstanceID] = instance.DBType
	}
	return instanceNames, instanceTypes
}

func (uc *UseCase) toInspectionReportVO(item *DatabaseInspectionReport, instanceName, instanceDBType string) *DatabaseInspectionReportVO {
	if item == nil {
		return nil
	}
	return &DatabaseInspectionReportVO{
		ID:                 item.ID,
		InstanceID:         item.InstanceID,
		InstanceName:       instanceName,
		DBType:             instanceDBType,
		DBTypeText:         DBTypeText(instanceDBType),
		ReportType:         item.ReportType,
		Status:             item.Status,
		StatusText:         BackupStatusText(item.Status),
		HealthScore:        item.HealthScore,
		RiskLevel:          item.RiskLevel,
		RiskLevelText:      QueryRiskLevelText(item.RiskLevel),
		Summary:            item.Summary,
		CapacitySummary:    unmarshalInspectionSection(item.CapacitySummary, "capacity", "容量"),
		PerformanceSummary: unmarshalInspectionSection(item.PerformanceSummary, "performance", "性能"),
		SecuritySummary:    unmarshalInspectionSection(item.SecuritySummary, "security", "安全"),
		BackupSummary:      unmarshalInspectionSection(item.BackupSummary, "backup", "备份"),
		Findings:           unmarshalInspectionFindings(item.Findings),
		OperatorID:         item.OperatorID,
		OperatorName:       item.OperatorName,
		GeneratedAt:        formatTime(item.GeneratedAt),
		DurationMs:         item.DurationMs,
		ErrorMessage:       item.ErrorMessage,
		CreatedAt:          item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:          item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func normalizeInspectionReportListRequest(req *DatabaseInspectionReportListRequest) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Status = strings.TrimSpace(req.Status)
	req.RiskLevel = strings.TrimSpace(req.RiskLevel)
}

func inspectionSection(key, label, status, summary string) *DatabaseInspectionSectionVO {
	return &DatabaseInspectionSectionVO{
		Key:     key,
		Label:   label,
		Status:  status,
		Summary: summary,
		Metrics: []*DatabaseInspectionMetricVO{},
	}
}

func inspectionFinding(severity, category, title, message, resourceType, resourceID string) *DatabaseInspectionFindingVO {
	return &DatabaseInspectionFindingVO{
		Severity:     severity,
		Category:     category,
		Title:        title,
		Message:      trimText(message, 500),
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

func redisInspectionMetricStatus(key, value string) string {
	switch strings.TrimSpace(key) {
	case "redis_cluster_state":
		if strings.TrimSpace(value) == "正常" {
			return "success"
		}
		return "danger"
	case "redis_memory_pressure":
		pressure := parsePercentNumber(value)
		if pressure >= 95 {
			return "danger"
		}
		if pressure >= 85 {
			return "warning"
		}
		return "success"
	case "redis_fragmentation":
		fragmentation := parseFloatNumber(value)
		if fragmentation >= 2 {
			return "warning"
		}
		return "success"
	case "redis_blocked_clients":
		if parseLeadingInt64(value) > 0 {
			return "warning"
		}
		return "success"
	case "redis_replica_lag":
		if parseDurationSeconds(value) >= 30 {
			return "warning"
		}
		return "success"
	case "redis_evicted_rejected":
		evicted, rejected := parsePairInt64(value)
		if evicted > 0 || rejected > 0 {
			return "warning"
		}
		return "success"
	default:
		return "info"
	}
}

func parseLeadingInt64(value string) int64 {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0
	}
	if index := strings.IndexAny(text, " /%s"); index > 0 {
		text = text[:index]
	}
	return parseInt64Default(text, 0)
}

func parsePairInt64(value string) (int64, int64) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseLeadingInt64(parts[0]), parseLeadingInt64(parts[1])
}

func parsePercentNumber(value string) float64 {
	text := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	return parseFloatNumber(text)
}

func parseDurationSeconds(value string) int64 {
	text := strings.TrimSpace(strings.TrimSuffix(value, "s"))
	return parseInt64Default(text, 0)
}

func parseFloatNumber(value string) float64 {
	text := strings.TrimSpace(value)
	if text == "" || text == "未限制" {
		return 0
	}
	return parseRedisFloatDefault(text, 0)
}

func calculateInspectionScore(findings []*DatabaseInspectionFindingVO) (int, string) {
	score := 100
	for _, finding := range findings {
		if finding == nil {
			continue
		}
		switch strings.TrimSpace(finding.Severity) {
		case "critical":
			score -= 30
		case "warning":
			score -= 15
		}
	}
	if score < 0 {
		score = 0
	}
	switch {
	case score < 60:
		return score, DatabaseQueryRiskCritical
	case score < 80:
		return score, DatabaseQueryRiskHigh
	case score < 90:
		return score, DatabaseQueryRiskMedium
	default:
		return score, DatabaseQueryRiskLow
	}
}

func buildInspectionSummary(score int, riskLevel string, findings []*DatabaseInspectionFindingVO) string {
	if len(findings) == 0 {
		return fmt.Sprintf("巡检完成，健康分 %d，风险等级 %s，暂无异常项", score, QueryRiskLevelText(riskLevel))
	}
	return fmt.Sprintf("巡检完成，健康分 %d，风险等级 %s，发现 %d 个关注项", score, QueryRiskLevelText(riskLevel), len(findings))
}

func calculateCapacityGrowth(points []*DatabaseCapacitySnapshot) (int64, float64) {
	if len(points) < 2 {
		return 0, 0
	}
	first := points[0]
	last := points[len(points)-1]
	if first == nil || last == nil {
		return 0, 0
	}
	growthBytes := last.TotalSizeBytes - first.TotalSizeBytes
	if first.TotalSizeBytes <= 0 {
		return growthBytes, 0
	}
	return growthBytes, float64(growthBytes) / float64(first.TotalSizeBytes) * 100
}

func capacityGrowthStatus(growthBytes int64, growthPercent float64) string {
	if growthBytes > 10*1024*1024*1024 || growthPercent >= 50 {
		return "warning"
	}
	return "success"
}

func countStatus(count int64) string {
	if count > 0 {
		return "warning"
	}
	return "success"
}

func backupSuccessStatus(record *DatabaseBackupRecord) string {
	if record == nil {
		return "warning"
	}
	if isBackupStale(record, 7*24*time.Hour) {
		return "warning"
	}
	return "success"
}

func isBackupStale(record *DatabaseBackupRecord, maxAge time.Duration) bool {
	if record == nil {
		return true
	}
	t := record.CreatedAt
	if record.FinishedAt != nil && !record.FinishedAt.IsZero() {
		t = *record.FinishedAt
	}
	return time.Since(t) > maxAge
}

func isDiagnosableType(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase, DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase, DBTypeRedis:
		return true
	default:
		return false
	}
}

func marshalInspectionSection(section *DatabaseInspectionSectionVO) string {
	if section == nil {
		return ""
	}
	data, _ := json.Marshal(section)
	return string(data)
}

func marshalInspectionFindings(findings []*DatabaseInspectionFindingVO) string {
	if len(findings) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(findings)
	return string(data)
}

func unmarshalInspectionSection(raw, key, label string) *DatabaseInspectionSectionVO {
	section := inspectionSection(key, label, "info", "")
	if strings.TrimSpace(raw) == "" {
		return section
	}
	if err := json.Unmarshal([]byte(raw), section); err != nil {
		return inspectionSection(key, label, "warning", "报告内容解析失败")
	}
	return section
}

func unmarshalInspectionFindings(raw string) []*DatabaseInspectionFindingVO {
	if strings.TrimSpace(raw) == "" {
		return []*DatabaseInspectionFindingVO{}
	}
	var findings []*DatabaseInspectionFindingVO
	if err := json.Unmarshal([]byte(raw), &findings); err != nil {
		return []*DatabaseInspectionFindingVO{}
	}
	if findings == nil {
		return []*DatabaseInspectionFindingVO{}
	}
	return findings
}
