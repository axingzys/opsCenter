package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type DatabaseBackupAlertRuleListRequest struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	Keyword    string `form:"keyword"`
	Enabled    string `form:"enabled"`
	ScopeType  string `form:"scopeType"`
	InstanceID uint   `form:"instanceId"`
	Engine     string `form:"engine"`
	IssueType  string `form:"issueType"`
}

type DatabaseBackupAlertStateListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	RuleID             uint   `form:"ruleId"`
	InstanceID         uint   `form:"instanceId"`
	Status             string `form:"status"`
	Severity           string `form:"severity"`
	IssueType          string `form:"issueType"`
	Keyword            string `form:"keyword"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseBackupAlertRuleRequest struct {
	Name           string                       `json:"name" binding:"required,max=120"`
	Enabled        bool                         `json:"enabled"`
	ScopeType      string                       `json:"scopeType" binding:"omitempty,max=40"`
	InstanceID     uint                         `json:"instanceId"`
	Engine         string                       `json:"engine" binding:"omitempty,max=30"`
	BusinessSystem string                       `json:"businessSystem" binding:"omitempty,max=120"`
	Owner          string                       `json:"owner" binding:"omitempty,max=120"`
	IssueTypes     []string                     `json:"issueTypes"`
	Severity       string                       `json:"severity" binding:"omitempty,max=20"`
	AlertInterval  int                          `json:"alertInterval"`
	RecoveryNotify bool                         `json:"recoveryNotify"`
	ChannelIDs     []uint                       `json:"channelIds"`
	Threshold      DatabaseBackupAlertThreshold `json:"threshold"`
	Description    string                       `json:"description" binding:"omitempty,max=500"`
}

type DatabaseBackupAlertThreshold struct {
	NoSuccessBackupHours  int    `json:"noSuccessBackupHours"`
	RestoreDrillStaleDays int    `json:"restoreDrillStaleDays"`
	RPOLagGraceMinutes    int    `json:"rpoLagGraceMinutes"`
	IncludeNonProduction  bool   `json:"includeNonProduction"`
	MinRiskLevel          string `json:"minRiskLevel"`
}

type DatabaseBackupAlertRuleVO struct {
	ID             uint                         `json:"id"`
	Name           string                       `json:"name"`
	Enabled        bool                         `json:"enabled"`
	ScopeType      string                       `json:"scopeType"`
	ScopeTypeText  string                       `json:"scopeTypeText"`
	InstanceID     uint                         `json:"instanceId"`
	Engine         string                       `json:"engine"`
	BusinessSystem string                       `json:"businessSystem"`
	Owner          string                       `json:"owner"`
	IssueTypes     []string                     `json:"issueTypes"`
	IssueTypeTexts []string                     `json:"issueTypeTexts"`
	Severity       string                       `json:"severity"`
	SeverityText   string                       `json:"severityText"`
	AlertInterval  int                          `json:"alertInterval"`
	RecoveryNotify bool                         `json:"recoveryNotify"`
	ChannelIDs     []uint                       `json:"channelIds"`
	Threshold      DatabaseBackupAlertThreshold `json:"threshold"`
	Description    string                       `json:"description"`
	CreatedAt      time.Time                    `json:"createdAt"`
	UpdatedAt      time.Time                    `json:"updatedAt"`
}

type DatabaseBackupAlertStateVO struct {
	ID             uint       `json:"id"`
	RuleID         uint       `json:"ruleId"`
	Fingerprint    string     `json:"fingerprint"`
	InstanceID     uint       `json:"instanceId"`
	ResourceType   string     `json:"resourceType"`
	ResourceID     uint       `json:"resourceId"`
	ResourceName   string     `json:"resourceName"`
	ResourceTarget string     `json:"resourceTarget"`
	IssueType      string     `json:"issueType"`
	IssueTypeText  string     `json:"issueTypeText"`
	AlertType      string     `json:"alertType"`
	Metric         string     `json:"metric"`
	MetricText     string     `json:"metricText"`
	Severity       string     `json:"severity"`
	SeverityText   string     `json:"severityText"`
	Status         string     `json:"status"`
	StatusText     string     `json:"statusText"`
	Message        string     `json:"message"`
	Suggestion     string     `json:"suggestion"`
	FirstFiredAt   time.Time  `json:"firstFiredAt"`
	LastFiredAt    time.Time  `json:"lastFiredAt"`
	LastNotifiedAt *time.Time `json:"lastNotifiedAt,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	NotifyCount    int        `json:"notifyCount"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type DatabaseBackupAlertSummaryVO struct {
	RuleTotal      int64 `json:"ruleTotal"`
	RuleEnabled    int64 `json:"ruleEnabled"`
	FiringTotal    int64 `json:"firingTotal"`
	CriticalFiring int64 `json:"criticalFiring"`
	WarningFiring  int64 `json:"warningFiring"`
	Notified24h    int64 `json:"notified24h"`
}

type DatabaseBackupAlertTestResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Channel string `json:"channel"`
	Error   string `json:"error"`
}

type DatabaseBackupAlertNotifier func(ctx context.Context, notice *DatabaseBackupAlertNotice) (string, error)

type DatabaseBackupAlertNotice struct {
	Rule      *DatabaseBackupAlertRule
	State     *DatabaseBackupAlertState
	Candidate *DatabaseBackupAlertCandidate
	Resolved  bool
	Test      bool
}

type DatabaseBackupAlertCandidate struct {
	Fingerprint    string `json:"fingerprint"`
	InstanceID     uint   `json:"instanceId"`
	ResourceType   string `json:"resourceType"`
	ResourceID     uint   `json:"resourceId"`
	ResourceName   string `json:"resourceName"`
	ResourceTarget string `json:"resourceTarget"`
	IssueType      string `json:"issueType"`
	AlertType      string `json:"alertType"`
	Metric         string `json:"metric"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	Suggestion     string `json:"suggestion"`
	RawJSON        string `json:"rawJson"`
}

func (uc *UseCase) ListBackupAlertRules(ctx context.Context, req *DatabaseBackupAlertRuleListRequest) ([]*DatabaseBackupAlertRuleVO, int64, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return nil, 0, fmt.Errorf("备份告警规则仓储未初始化")
	}
	normalizeBackupAlertRuleListRequest(req)
	items, total, err := uc.backupAlertRuleRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*DatabaseBackupAlertRuleVO, 0, len(items))
	for _, item := range items {
		result = append(result, toBackupAlertRuleVO(item))
	}
	return result, total, nil
}

func (uc *UseCase) GetBackupAlertRule(ctx context.Context, id uint) (*DatabaseBackupAlertRuleVO, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return nil, fmt.Errorf("备份告警规则仓储未初始化")
	}
	item, err := uc.backupAlertRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toBackupAlertRuleVO(item), nil
}

func (uc *UseCase) CreateBackupAlertRule(ctx context.Context, req *DatabaseBackupAlertRuleRequest) (*DatabaseBackupAlertRuleVO, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return nil, fmt.Errorf("备份告警规则仓储未初始化")
	}
	item, err := buildBackupAlertRuleFromRequest(nil, req)
	if err != nil {
		return nil, err
	}
	if err := uc.backupAlertRuleRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return toBackupAlertRuleVO(item), nil
}

func (uc *UseCase) UpdateBackupAlertRule(ctx context.Context, id uint, req *DatabaseBackupAlertRuleRequest) (*DatabaseBackupAlertRuleVO, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return nil, fmt.Errorf("备份告警规则仓储未初始化")
	}
	item, err := uc.backupAlertRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item, err = buildBackupAlertRuleFromRequest(item, req)
	if err != nil {
		return nil, err
	}
	if err := uc.backupAlertRuleRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return toBackupAlertRuleVO(item), nil
}

func (uc *UseCase) DeleteBackupAlertRule(ctx context.Context, id uint) error {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return fmt.Errorf("备份告警规则仓储未初始化")
	}
	if _, err := uc.backupAlertRuleRepo.GetByID(ctx, id); err != nil {
		return err
	}
	return uc.backupAlertRuleRepo.Delete(ctx, id)
}

func (uc *UseCase) ListBackupAlertStates(ctx context.Context, req *DatabaseBackupAlertStateListRequest) ([]*DatabaseBackupAlertStateVO, int64, error) {
	if uc == nil || uc.backupAlertStateRepo == nil {
		return nil, 0, fmt.Errorf("备份告警状态仓储未初始化")
	}
	normalizeBackupAlertStateListRequest(req)
	items, total, err := uc.backupAlertStateRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*DatabaseBackupAlertStateVO, 0, len(items))
	for _, item := range items {
		result = append(result, toBackupAlertStateVO(item))
	}
	return result, total, nil
}

func (uc *UseCase) GetBackupAlertSummary(ctx context.Context) (*DatabaseBackupAlertSummaryVO, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil || uc.backupAlertStateRepo == nil {
		return &DatabaseBackupAlertSummaryVO{}, nil
	}
	rules, ruleTotal, err := uc.backupAlertRuleRepo.List(ctx, &DatabaseBackupAlertRuleListRequest{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	var enabled int64
	for _, rule := range rules {
		if rule.Enabled {
			enabled++
		}
	}
	states, firingTotal, err := uc.backupAlertStateRepo.List(ctx, &DatabaseBackupAlertStateListRequest{Page: 1, PageSize: 200, Status: DatabaseBackupAlertStateFiring})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	summary := &DatabaseBackupAlertSummaryVO{RuleTotal: ruleTotal, RuleEnabled: enabled, FiringTotal: firingTotal}
	for _, state := range states {
		switch strings.TrimSpace(state.Severity) {
		case DatabaseQueryRiskCritical, DatabaseQueryRiskHigh:
			summary.CriticalFiring++
		default:
			summary.WarningFiring++
		}
		if state.LastNotifiedAt != nil && state.LastNotifiedAt.After(now.Add(-24*time.Hour)) {
			summary.Notified24h++
		}
	}
	return summary, nil
}

func (uc *UseCase) TestBackupAlertRule(ctx context.Context, id uint) (*DatabaseBackupAlertTestResult, error) {
	if uc == nil || uc.backupAlertRuleRepo == nil {
		return nil, fmt.Errorf("备份告警规则仓储未初始化")
	}
	rule, err := uc.backupAlertRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	decodeBackupAlertRule(rule)
	candidate := &DatabaseBackupAlertCandidate{
		Fingerprint:    fmt.Sprintf("database_backup_alert_test:%d:%d", rule.ID, time.Now().UnixNano()),
		ResourceType:   "database_backup_alert_rule",
		ResourceID:     rule.ID,
		ResourceName:   rule.Name,
		ResourceTarget: "告警通道测试",
		IssueType:      "test",
		AlertType:      "database_backup_alert_test",
		Metric:         "backup_alert_test",
		Severity:       firstNonEmpty(rule.Severity, DatabaseQueryRiskMedium),
		Message:        fmt.Sprintf("数据库备份恢复告警规则「%s」测试通知", rule.Name),
		Suggestion:     "如果收到该消息，说明告警通道可用。",
	}
	if uc.backupAlertNotifier == nil {
		return &DatabaseBackupAlertTestResult{Status: "failed", Message: "未配置数据库备份告警通知器", Error: "notifier not configured"}, nil
	}
	channel, notifyErr := uc.backupAlertNotifier(ctx, &DatabaseBackupAlertNotice{Rule: rule, Candidate: candidate, Test: true})
	if notifyErr != nil {
		return &DatabaseBackupAlertTestResult{Status: "failed", Message: "测试发送失败", Channel: channel, Error: notifyErr.Error()}, nil
	}
	return &DatabaseBackupAlertTestResult{Status: "success", Message: "测试发送成功", Channel: channel}, nil
}

func (uc *UseCase) RunBackupAlertScan(ctx context.Context) error {
	if uc == nil || uc.backupAlertRuleRepo == nil || uc.backupAlertStateRepo == nil {
		return nil
	}
	rules, err := uc.backupAlertRuleRepo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		decodeBackupAlertRule(rule)
		if err := uc.runBackupAlertRuleScan(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

func (uc *UseCase) runBackupAlertRuleScan(ctx context.Context, rule *DatabaseBackupAlertRule) error {
	if rule == nil || rule.ID == 0 || !rule.Enabled {
		return nil
	}
	now := time.Now()
	candidates, err := uc.evaluateBackupAlertRule(ctx, rule, now)
	if err != nil {
		return err
	}
	current := make(map[string]*DatabaseBackupAlertCandidate, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || strings.TrimSpace(candidate.Fingerprint) == "" {
			continue
		}
		current[candidate.Fingerprint] = candidate
		if err := uc.upsertBackupAlertState(ctx, rule, candidate, now); err != nil {
			return err
		}
	}
	firingStates, err := uc.backupAlertStateRepo.ListFiringByRuleID(ctx, rule.ID)
	if err != nil {
		return err
	}
	for _, state := range firingStates {
		if _, ok := current[state.Fingerprint]; ok {
			continue
		}
		state.Status = DatabaseBackupAlertStateResolved
		state.ResolvedAt = &now
		state.LastFiredAt = now
		if rule.RecoveryNotify && uc.backupAlertNotifier != nil {
			candidate := candidateFromAlertState(state)
			candidate.Message = "已恢复：" + state.Message
			if _, notifyErr := uc.backupAlertNotifier(ctx, &DatabaseBackupAlertNotice{Rule: rule, State: state, Candidate: candidate, Resolved: true}); notifyErr == nil {
				state.LastNotifiedAt = &now
				state.NotifyCount++
			}
		}
		if err := uc.backupAlertStateRepo.Update(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (uc *UseCase) upsertBackupAlertState(ctx context.Context, rule *DatabaseBackupAlertRule, candidate *DatabaseBackupAlertCandidate, now time.Time) error {
	state, err := uc.backupAlertStateRepo.GetByFingerprint(ctx, candidate.Fingerprint)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	isNew := state == nil || state.ID == 0
	if isNew {
		state = &DatabaseBackupAlertState{
			RuleID:       rule.ID,
			Fingerprint:  candidate.Fingerprint,
			FirstFiredAt: now,
			Status:       DatabaseBackupAlertStateFiring,
		}
	}
	reopened := state.Status == DatabaseBackupAlertStateResolved
	state.RuleID = rule.ID
	state.InstanceID = candidate.InstanceID
	state.ResourceType = candidate.ResourceType
	state.ResourceID = candidate.ResourceID
	state.ResourceName = candidate.ResourceName
	state.ResourceTarget = candidate.ResourceTarget
	state.IssueType = candidate.IssueType
	state.AlertType = candidate.AlertType
	state.Metric = candidate.Metric
	state.Severity = firstNonEmpty(candidate.Severity, rule.Severity, DatabaseQueryRiskMedium)
	state.Status = DatabaseBackupAlertStateFiring
	state.Message = candidate.Message
	state.Suggestion = candidate.Suggestion
	state.LastFiredAt = now
	state.ResolvedAt = nil
	state.RawJSON = candidate.RawJSON

	shouldNotify := isNew || reopened || state.LastNotifiedAt == nil || now.Sub(*state.LastNotifiedAt) >= time.Duration(normalizedBackupAlertInterval(rule.AlertInterval))*time.Second
	if shouldNotify && uc.backupAlertNotifier != nil {
		if _, notifyErr := uc.backupAlertNotifier(ctx, &DatabaseBackupAlertNotice{Rule: rule, State: state, Candidate: candidate}); notifyErr == nil {
			state.LastNotifiedAt = &now
			state.NotifyCount++
		} else if state.LastNotifiedAt == nil || now.Sub(*state.LastNotifiedAt) >= time.Minute {
			state.LastNotifiedAt = &now
			state.NotifyCount++
		}
	}
	if isNew {
		return uc.backupAlertStateRepo.Create(ctx, state)
	}
	return uc.backupAlertStateRepo.Update(ctx, state)
}

func (uc *UseCase) evaluateBackupAlertRule(ctx context.Context, rule *DatabaseBackupAlertRule, now time.Time) ([]*DatabaseBackupAlertCandidate, error) {
	threshold := backupAlertThreshold(rule)
	candidates := make([]*DatabaseBackupAlertCandidate, 0, 32)
	risks, err := uc.collectProtectionRisksForAlertRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	for _, risk := range risks {
		if !backupAlertRuleMatchesRisk(rule, threshold, risk) {
			continue
		}
		candidates = append(candidates, candidateFromProtectionRisk(rule, risk))
	}
	profiles, err := uc.collectProtectionProfilesForAlertRule(ctx, rule)
	if err == nil {
		for _, profile := range profiles {
			if !backupAlertRuleAcceptsIssue(rule, DatabaseBackupAlertIssueNoRecentSuccessBackup) || !backupAlertRuleMatchesProfileScope(rule, profile) {
				continue
			}
			if candidate := candidateFromNoRecentSuccessProfile(rule, profile, threshold, now); candidate != nil {
				candidates = append(candidates, candidate)
			}
		}
	}
	failedRecords, err := uc.collectFailedBackupRecordCandidates(ctx, rule, threshold, now)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, failedRecords...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return backupAlertSeverityRank(candidates[i].Severity) > backupAlertSeverityRank(candidates[j].Severity)
	})
	return candidates, nil
}

func (uc *UseCase) collectProtectionRisksForAlertRule(ctx context.Context, rule *DatabaseBackupAlertRule) ([]*DatabaseProtectionRiskVO, error) {
	result := make([]*DatabaseProtectionRiskVO, 0, 128)
	req := &DatabaseProtectionRiskListRequest{
		Page:       1,
		PageSize:   100,
		InstanceID: scopedAlertInstanceID(rule),
		Engine:     scopedAlertEngine(rule),
	}
	if rule.ScopeType == DatabaseBackupAlertScopeProduction {
		req.ProductionOnly = "true"
	}
	for {
		items, _, err := uc.ListProtectionRisks(ctx, req)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(items) < req.PageSize || req.Page >= 200 {
			break
		}
		req.Page++
	}
	return result, nil
}

func (uc *UseCase) collectProtectionProfilesForAlertRule(ctx context.Context, rule *DatabaseBackupAlertRule) ([]*DatabaseProtectionProfileVO, error) {
	result := make([]*DatabaseProtectionProfileVO, 0, 128)
	req := &DatabaseProtectionProfileListRequest{
		Page:       1,
		PageSize:   100,
		InstanceID: scopedAlertInstanceID(rule),
		Engine:     scopedAlertEngine(rule),
	}
	for {
		items, _, err := uc.ListProtectionProfiles(ctx, req)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(items) < req.PageSize || req.Page >= 200 {
			break
		}
		req.Page++
	}
	return result, nil
}

func (uc *UseCase) collectFailedBackupRecordCandidates(ctx context.Context, rule *DatabaseBackupAlertRule, threshold DatabaseBackupAlertThreshold, now time.Time) ([]*DatabaseBackupAlertCandidate, error) {
	if uc == nil || uc.backupRecordRepo == nil || !backupAlertRuleAcceptsIssue(rule, DatabaseBackupAlertIssueBackupFailed) {
		return nil, nil
	}
	since := now.Add(-time.Duration(defaultInt(threshold.NoSuccessBackupHours, 24)) * time.Hour)
	req := &DatabaseBackupRecordListRequest{Page: 1, PageSize: 100, Status: DatabaseBackupStatusFailed, InstanceID: scopedAlertInstanceID(rule)}
	candidates := make([]*DatabaseBackupAlertCandidate, 0, 16)
	for {
		records, _, err := uc.backupRecordRepo.List(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if record == nil || record.CreatedAt.Before(since) {
				continue
			}
			instance, _ := uc.loadBackupAlertInstance(ctx, record.InstanceID)
			if !backupAlertRuleMatchesInstance(rule, instance) {
				continue
			}
			candidates = append(candidates, candidateFromFailedBackupRecord(rule, record, instance))
		}
		if len(records) < req.PageSize || req.Page >= 5 {
			break
		}
		req.Page++
	}
	return candidates, nil
}

func (uc *UseCase) loadBackupAlertInstance(ctx context.Context, instanceID uint) (*DatabaseInstance, error) {
	if uc == nil || uc.instanceRepo == nil || instanceID == 0 {
		return nil, nil
	}
	return uc.instanceRepo.GetByID(ctx, instanceID)
}

func buildBackupAlertRuleFromRequest(item *DatabaseBackupAlertRule, req *DatabaseBackupAlertRuleRequest) (*DatabaseBackupAlertRule, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	if item == nil {
		item = &DatabaseBackupAlertRule{Enabled: true}
	}
	item.Name = strings.TrimSpace(req.Name)
	item.Enabled = req.Enabled
	item.ScopeType = normalizeBackupAlertScope(req.ScopeType)
	item.InstanceID = req.InstanceID
	item.Engine = strings.TrimSpace(req.Engine)
	item.BusinessSystem = strings.TrimSpace(req.BusinessSystem)
	item.Owner = strings.TrimSpace(req.Owner)
	item.IssueTypes = sanitizeBackupAlertIssueTypes(req.IssueTypes)
	item.IssueTypesJSON = mustMarshalStringSlice(item.IssueTypes)
	item.Severity = normalizeBackupAlertSeverity(req.Severity)
	item.AlertInterval = normalizedBackupAlertInterval(req.AlertInterval)
	item.RecoveryNotify = req.RecoveryNotify
	item.ChannelIDs = sanitizeUintIDs(req.ChannelIDs)
	item.ChannelIDsJSON = mustMarshalUintSlice(item.ChannelIDs)
	item.ThresholdJSON = mustMarshalThreshold(normalizeBackupAlertThreshold(req.Threshold))
	item.Description = strings.TrimSpace(req.Description)
	if item.ScopeType == DatabaseBackupAlertScopeInstance && item.InstanceID == 0 {
		return nil, fmt.Errorf("指定实例范围必须选择实例")
	}
	if item.ScopeType == DatabaseBackupAlertScopeEngine && item.Engine == "" {
		return nil, fmt.Errorf("指定引擎范围必须选择数据库引擎")
	}
	return item, nil
}

func normalizeBackupAlertRuleListRequest(req *DatabaseBackupAlertRuleListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
}

func normalizeBackupAlertStateListRequest(req *DatabaseBackupAlertStateListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
}

func toBackupAlertRuleVO(item *DatabaseBackupAlertRule) *DatabaseBackupAlertRuleVO {
	if item == nil {
		return nil
	}
	decodeBackupAlertRule(item)
	issueTexts := make([]string, 0, len(item.IssueTypes))
	for _, issue := range item.IssueTypes {
		issueTexts = append(issueTexts, BackupAlertIssueTypeText(issue))
	}
	threshold := backupAlertThreshold(item)
	return &DatabaseBackupAlertRuleVO{
		ID:             item.ID,
		Name:           item.Name,
		Enabled:        item.Enabled,
		ScopeType:      item.ScopeType,
		ScopeTypeText:  BackupAlertScopeText(item.ScopeType),
		InstanceID:     item.InstanceID,
		Engine:         item.Engine,
		BusinessSystem: item.BusinessSystem,
		Owner:          item.Owner,
		IssueTypes:     item.IssueTypes,
		IssueTypeTexts: issueTexts,
		Severity:       item.Severity,
		SeverityText:   BackupAlertSeverityText(item.Severity),
		AlertInterval:  item.AlertInterval,
		RecoveryNotify: item.RecoveryNotify,
		ChannelIDs:     item.ChannelIDs,
		Threshold:      threshold,
		Description:    item.Description,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toBackupAlertStateVO(item *DatabaseBackupAlertState) *DatabaseBackupAlertStateVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupAlertStateVO{
		ID:             item.ID,
		RuleID:         item.RuleID,
		Fingerprint:    item.Fingerprint,
		InstanceID:     item.InstanceID,
		ResourceType:   item.ResourceType,
		ResourceID:     item.ResourceID,
		ResourceName:   item.ResourceName,
		ResourceTarget: item.ResourceTarget,
		IssueType:      item.IssueType,
		IssueTypeText:  BackupAlertIssueTypeText(item.IssueType),
		AlertType:      item.AlertType,
		Metric:         item.Metric,
		MetricText:     BackupAlertMetricText(item.Metric),
		Severity:       item.Severity,
		SeverityText:   BackupAlertSeverityText(item.Severity),
		Status:         item.Status,
		StatusText:     BackupAlertStateText(item.Status),
		Message:        item.Message,
		Suggestion:     item.Suggestion,
		FirstFiredAt:   item.FirstFiredAt,
		LastFiredAt:    item.LastFiredAt,
		LastNotifiedAt: item.LastNotifiedAt,
		ResolvedAt:     item.ResolvedAt,
		NotifyCount:    item.NotifyCount,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func decodeBackupAlertRule(item *DatabaseBackupAlertRule) {
	if item == nil {
		return
	}
	if len(item.IssueTypes) == 0 && strings.TrimSpace(item.IssueTypesJSON) != "" {
		_ = json.Unmarshal([]byte(item.IssueTypesJSON), &item.IssueTypes)
	}
	if len(item.ChannelIDs) == 0 && strings.TrimSpace(item.ChannelIDsJSON) != "" {
		_ = json.Unmarshal([]byte(item.ChannelIDsJSON), &item.ChannelIDs)
	}
	item.ScopeType = normalizeBackupAlertScope(item.ScopeType)
	item.Severity = normalizeBackupAlertSeverity(item.Severity)
	item.AlertInterval = normalizedBackupAlertInterval(item.AlertInterval)
}

func backupAlertThreshold(rule *DatabaseBackupAlertRule) DatabaseBackupAlertThreshold {
	threshold := DatabaseBackupAlertThreshold{}
	if rule != nil && strings.TrimSpace(rule.ThresholdJSON) != "" {
		_ = json.Unmarshal([]byte(rule.ThresholdJSON), &threshold)
	}
	return normalizeBackupAlertThreshold(threshold)
}

func normalizeBackupAlertThreshold(item DatabaseBackupAlertThreshold) DatabaseBackupAlertThreshold {
	if item.NoSuccessBackupHours <= 0 {
		item.NoSuccessBackupHours = 24
	}
	if item.RestoreDrillStaleDays <= 0 {
		item.RestoreDrillStaleDays = 30
	}
	if item.RPOLagGraceMinutes <= 0 {
		item.RPOLagGraceMinutes = 10
	}
	if strings.TrimSpace(item.MinRiskLevel) == "" {
		item.MinRiskLevel = DatabaseQueryRiskMedium
	}
	return item
}

func normalizeBackupAlertScope(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupAlertScopeProduction, DatabaseBackupAlertScopeInstance, DatabaseBackupAlertScopeEngine, DatabaseBackupAlertScopeBusinessSystem, DatabaseBackupAlertScopeOwner:
		return strings.TrimSpace(value)
	default:
		return DatabaseBackupAlertScopeAll
	}
}

func normalizeBackupAlertSeverity(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseQueryRiskCritical, DatabaseQueryRiskHigh, DatabaseQueryRiskMedium, DatabaseQueryRiskLow:
		return strings.TrimSpace(value)
	default:
		return DatabaseQueryRiskMedium
	}
}

func normalizedBackupAlertInterval(value int) int {
	if value <= 0 {
		return 1800
	}
	if value < 60 {
		return 60
	}
	return value
}

func sanitizeBackupAlertIssueTypes(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func sanitizeUintIDs(items []uint) []uint {
	seen := make(map[uint]struct{}, len(items))
	result := make([]uint, 0, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mustMarshalStringSlice(items []string) string {
	data, _ := json.Marshal(items)
	return string(data)
}

func mustMarshalUintSlice(items []uint) string {
	data, _ := json.Marshal(items)
	return string(data)
}

func mustMarshalThreshold(item DatabaseBackupAlertThreshold) string {
	data, _ := json.Marshal(item)
	return string(data)
}

func candidateFromProtectionRisk(rule *DatabaseBackupAlertRule, risk *DatabaseProtectionRiskVO) *DatabaseBackupAlertCandidate {
	issueType := strings.TrimSpace(risk.IssueType)
	if issueType == DatabaseProtectionIssueArchiveLagHigh {
		issueType = DatabaseBackupAlertIssueRPOBreached
	}
	candidate := &DatabaseBackupAlertCandidate{
		InstanceID:     risk.InstanceID,
		ResourceType:   "database_instance",
		ResourceID:     risk.InstanceID,
		ResourceName:   firstNonEmpty(risk.InstanceName, fmt.Sprintf("#%d", risk.InstanceID)),
		ResourceTarget: risk.Endpoint,
		IssueType:      issueType,
		AlertType:      backupAlertTypeForIssue(issueType),
		Metric:         backupAlertMetricForIssue(issueType),
		Severity:       firstNonEmpty(rule.Severity, risk.RiskLevel, DatabaseQueryRiskMedium),
		Message:        risk.Message,
		Suggestion:     firstNonEmpty(risk.ActionText, risk.Action),
	}
	candidate.Fingerprint = backupAlertFingerprint(rule.ID, candidate.ResourceType, candidate.ResourceID, candidate.IssueType)
	candidate.RawJSON = marshalCandidateContext(risk)
	return candidate
}

func candidateFromNoRecentSuccessProfile(rule *DatabaseBackupAlertRule, profile *DatabaseProtectionProfileVO, threshold DatabaseBackupAlertThreshold, now time.Time) *DatabaseBackupAlertCandidate {
	if profile == nil || profile.InstanceID == 0 || profile.ProtectionLevel == "none" {
		return nil
	}
	lastFullAt := parseBackupAlertTime(profile.LastFullAt)
	if !lastFullAt.IsZero() && now.Sub(lastFullAt) < time.Duration(threshold.NoSuccessBackupHours)*time.Hour {
		return nil
	}
	message := "当前实例超过阈值没有成功全量备份"
	if lastFullAt.IsZero() {
		message = "当前实例没有成功全量备份记录"
	}
	candidate := &DatabaseBackupAlertCandidate{
		InstanceID:     profile.InstanceID,
		ResourceType:   "database_instance",
		ResourceID:     profile.InstanceID,
		ResourceName:   firstNonEmpty(profile.InstanceName, fmt.Sprintf("#%d", profile.InstanceID)),
		ResourceTarget: profile.Endpoint,
		IssueType:      DatabaseBackupAlertIssueNoRecentSuccessBackup,
		AlertType:      backupAlertTypeForIssue(DatabaseBackupAlertIssueNoRecentSuccessBackup),
		Metric:         backupAlertMetricForIssue(DatabaseBackupAlertIssueNoRecentSuccessBackup),
		Severity:       firstNonEmpty(rule.Severity, DatabaseQueryRiskHigh),
		Message:        fmt.Sprintf("%s，阈值 %d 小时", message, threshold.NoSuccessBackupHours),
		Suggestion:     "检查备份策略并执行一次全量备份",
	}
	candidate.Fingerprint = backupAlertFingerprint(rule.ID, candidate.ResourceType, candidate.ResourceID, candidate.IssueType)
	candidate.RawJSON = marshalCandidateContext(profile)
	return candidate
}

func candidateFromFailedBackupRecord(rule *DatabaseBackupAlertRule, record *DatabaseBackupRecord, instance *DatabaseInstance) *DatabaseBackupAlertCandidate {
	name := fmt.Sprintf("备份记录 #%d", record.ID)
	target := ""
	if instance != nil {
		name = firstNonEmpty(instance.Name, name)
		target = databaseInstanceEndpoint(instance)
	}
	message := firstNonEmpty(record.ErrorMessage, "备份执行失败")
	if record.FileName != "" {
		message = fmt.Sprintf("%s，文件 %s", message, record.FileName)
	}
	candidate := &DatabaseBackupAlertCandidate{
		InstanceID:     record.InstanceID,
		ResourceType:   "database_backup_record",
		ResourceID:     record.ID,
		ResourceName:   name,
		ResourceTarget: target,
		IssueType:      DatabaseBackupAlertIssueBackupFailed,
		AlertType:      backupAlertTypeForIssue(DatabaseBackupAlertIssueBackupFailed),
		Metric:         backupAlertMetricForIssue(DatabaseBackupAlertIssueBackupFailed),
		Severity:       firstNonEmpty(rule.Severity, DatabaseQueryRiskCritical),
		Message:        message,
		Suggestion:     "查看备份记录和 Runner 日志，修复后重新执行备份",
	}
	candidate.Fingerprint = backupAlertFingerprint(rule.ID, candidate.ResourceType, candidate.ResourceID, candidate.IssueType)
	candidate.RawJSON = marshalCandidateContext(record)
	return candidate
}

func candidateFromAlertState(state *DatabaseBackupAlertState) *DatabaseBackupAlertCandidate {
	return &DatabaseBackupAlertCandidate{
		Fingerprint:    state.Fingerprint,
		InstanceID:     state.InstanceID,
		ResourceType:   state.ResourceType,
		ResourceID:     state.ResourceID,
		ResourceName:   state.ResourceName,
		ResourceTarget: state.ResourceTarget,
		IssueType:      state.IssueType,
		AlertType:      state.AlertType,
		Metric:         state.Metric,
		Severity:       state.Severity,
		Message:        state.Message,
		Suggestion:     state.Suggestion,
		RawJSON:        state.RawJSON,
	}
}

func backupAlertFingerprint(ruleID uint, resourceType string, resourceID uint, issueType string) string {
	return fmt.Sprintf("database_backup_alert:%d:%s:%d:%s", ruleID, strings.TrimSpace(resourceType), resourceID, strings.TrimSpace(issueType))
}

func backupAlertRuleMatchesRisk(rule *DatabaseBackupAlertRule, threshold DatabaseBackupAlertThreshold, risk *DatabaseProtectionRiskVO) bool {
	if rule == nil || risk == nil {
		return false
	}
	if !backupAlertRuleAcceptsIssue(rule, normalizedRiskIssueType(risk.IssueType)) {
		return false
	}
	if backupAlertSeverityRank(risk.RiskLevel) < backupAlertSeverityRank(threshold.MinRiskLevel) {
		return false
	}
	switch rule.ScopeType {
	case DatabaseBackupAlertScopeProduction:
		return isBackupAlertProductionEnvironment(risk.Environment)
	case DatabaseBackupAlertScopeInstance:
		return risk.InstanceID == rule.InstanceID
	case DatabaseBackupAlertScopeEngine:
		return strings.EqualFold(risk.Engine, rule.Engine)
	case DatabaseBackupAlertScopeBusinessSystem:
		return strings.TrimSpace(rule.BusinessSystem) == "" || strings.EqualFold(risk.BusinessSystem, rule.BusinessSystem)
	case DatabaseBackupAlertScopeOwner:
		return strings.TrimSpace(rule.Owner) == "" || strings.EqualFold(risk.Owner, rule.Owner)
	default:
		return true
	}
}

func backupAlertRuleMatchesProfileScope(rule *DatabaseBackupAlertRule, profile *DatabaseProtectionProfileVO) bool {
	if rule == nil || profile == nil {
		return false
	}
	switch rule.ScopeType {
	case DatabaseBackupAlertScopeProduction:
		return isBackupAlertProductionEnvironment(profile.Environment)
	case DatabaseBackupAlertScopeInstance:
		return profile.InstanceID == rule.InstanceID
	case DatabaseBackupAlertScopeEngine:
		return strings.EqualFold(profile.Engine, rule.Engine)
	case DatabaseBackupAlertScopeBusinessSystem:
		return strings.TrimSpace(rule.BusinessSystem) == "" || strings.EqualFold(profile.BusinessSystem, rule.BusinessSystem)
	case DatabaseBackupAlertScopeOwner:
		return strings.TrimSpace(rule.Owner) == "" || strings.EqualFold(profile.Owner, rule.Owner)
	default:
		return true
	}
}

func backupAlertRuleMatchesInstance(rule *DatabaseBackupAlertRule, instance *DatabaseInstance) bool {
	if rule == nil {
		return false
	}
	if instance == nil {
		return rule.ScopeType == DatabaseBackupAlertScopeAll
	}
	switch rule.ScopeType {
	case DatabaseBackupAlertScopeProduction:
		return isBackupAlertProductionEnvironment(instance.Environment)
	case DatabaseBackupAlertScopeInstance:
		return instance.ID == rule.InstanceID
	case DatabaseBackupAlertScopeEngine:
		return strings.EqualFold(instance.DBType, rule.Engine)
	case DatabaseBackupAlertScopeBusinessSystem:
		return strings.TrimSpace(rule.BusinessSystem) == "" || strings.EqualFold(instance.BusinessSystem, rule.BusinessSystem)
	case DatabaseBackupAlertScopeOwner:
		return strings.TrimSpace(rule.Owner) == "" || strings.EqualFold(instance.Owner, rule.Owner)
	default:
		return true
	}
}

func backupAlertRuleAcceptsIssue(rule *DatabaseBackupAlertRule, issueType string) bool {
	if rule == nil {
		return false
	}
	decodeBackupAlertRule(rule)
	if len(rule.IssueTypes) == 0 {
		return true
	}
	issueType = normalizedRiskIssueType(issueType)
	for _, item := range rule.IssueTypes {
		if normalizedRiskIssueType(item) == issueType {
			return true
		}
	}
	return false
}

func normalizedRiskIssueType(issueType string) string {
	switch strings.TrimSpace(issueType) {
	case DatabaseProtectionIssueArchiveLagHigh:
		return DatabaseBackupAlertIssueRPOBreached
	default:
		return strings.TrimSpace(issueType)
	}
}

func backupAlertTypeForIssue(issueType string) string {
	switch strings.TrimSpace(issueType) {
	case DatabaseBackupAlertIssueBackupFailed:
		return "database_backup_failed"
	case DatabaseBackupAlertIssueNoRecentSuccessBackup:
		return "database_backup_no_recent_success"
	case DatabaseProtectionIssueMissingFullBackup:
		return "database_backup_missing_full"
	case DatabaseProtectionIssueIncrementalChainBroken:
		return "database_backup_chain_broken"
	case DatabaseBackupAlertIssueRPOBreached, DatabaseProtectionIssueArchiveLagHigh:
		return "database_backup_rpo_breached"
	case DatabaseProtectionIssueLogChainGap:
		return "database_backup_log_gap"
	case DatabaseProtectionIssueRestoreDrillMissing:
		return "database_restore_drill_stale"
	case DatabaseProtectionIssueRestoreDrillFailed:
		return "database_restore_drill_failed"
	case DatabaseProtectionIssueRunnerOffline:
		return "database_backup_runner_unavailable"
	case DatabaseProtectionIssueRunnerToolMissing:
		return "database_backup_runner_tool_failed"
	case DatabaseProtectionIssueStoragePostureFailed:
		return "database_backup_storage_failed"
	default:
		return "database_backup_risk"
	}
}

func backupAlertMetricForIssue(issueType string) string {
	switch strings.TrimSpace(issueType) {
	case DatabaseBackupAlertIssueBackupFailed:
		return "backup_failed"
	case DatabaseBackupAlertIssueNoRecentSuccessBackup, DatabaseProtectionIssueMissingFullBackup, DatabaseProtectionIssueIncrementalChainBroken:
		return "backup_success_gap"
	case DatabaseBackupAlertIssueRPOBreached, DatabaseProtectionIssueArchiveLagHigh, DatabaseProtectionIssueLogChainGap:
		return "rpo_lag"
	case DatabaseProtectionIssueRestoreDrillMissing, DatabaseProtectionIssueRestoreDrillFailed:
		return "restore_drill_stale"
	case DatabaseProtectionIssueRunnerOffline, DatabaseProtectionIssueRunnerToolMissing:
		return "runner_status"
	case DatabaseProtectionIssueStoragePostureFailed:
		return "storage_posture"
	default:
		return "backup_risk"
	}
}

func backupAlertSeverityRank(value string) int {
	switch strings.TrimSpace(value) {
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

func scopedAlertInstanceID(rule *DatabaseBackupAlertRule) uint {
	if rule != nil && rule.ScopeType == DatabaseBackupAlertScopeInstance {
		return rule.InstanceID
	}
	return 0
}

func scopedAlertEngine(rule *DatabaseBackupAlertRule) string {
	if rule != nil && rule.ScopeType == DatabaseBackupAlertScopeEngine {
		return rule.Engine
	}
	return ""
}

func isBackupAlertProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production", "生产", "生产环境":
		return true
	default:
		return false
	}
}

func marshalCandidateContext(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func parseBackupAlertTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func databaseInstanceEndpoint(instance *DatabaseInstance) string {
	if instance == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(instance.Host), instance.Port)
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func BackupAlertScopeText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupAlertScopeProduction:
		return "生产实例"
	case DatabaseBackupAlertScopeInstance:
		return "指定实例"
	case DatabaseBackupAlertScopeEngine:
		return "指定引擎"
	case DatabaseBackupAlertScopeBusinessSystem:
		return "业务系统"
	case DatabaseBackupAlertScopeOwner:
		return "负责人"
	default:
		return "全部实例"
	}
}

func BackupAlertSeverityText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseQueryRiskCritical:
		return "严重"
	case DatabaseQueryRiskHigh:
		return "高"
	case DatabaseQueryRiskLow:
		return "低"
	default:
		return "中"
	}
}

func BackupAlertStateText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupAlertStateResolved:
		return "已恢复"
	default:
		return "告警中"
	}
}

func BackupAlertIssueTypeText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupAlertIssueBackupFailed:
		return "备份失败"
	case DatabaseBackupAlertIssueNoRecentSuccessBackup:
		return "无近期成功备份"
	case DatabaseBackupAlertIssueRPOBreached:
		return "RPO 超时"
	default:
		return ProtectionIssueTypeText(value)
	}
}

func BackupAlertMetricText(value string) string {
	switch strings.TrimSpace(value) {
	case "backup_failed":
		return "备份失败"
	case "backup_success_gap":
		return "成功备份间隔"
	case "rpo_lag":
		return "RPO 延迟"
	case "restore_drill_stale":
		return "恢复演练"
	case "runner_status":
		return "Runner 状态"
	case "barman_status":
		return "Barman 状态"
	case "storage_posture":
		return "存储姿态"
	case "log_archive_status":
		return "日志归档状态"
	default:
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return value
	}
}
