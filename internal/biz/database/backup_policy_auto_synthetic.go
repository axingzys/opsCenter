package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type syntheticAutoDecision struct {
	ShouldRun bool
	Degraded  bool
	Reason    string
}

func (uc *UseCase) maybeRunBackupPolicySyntheticAfterSuccess(ctx context.Context, policyID uint, triggerRecordID uint, triggerReason string) {
	if uc == nil || uc.backupPolicyConfigRepo == nil || uc.backupRecordRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return
	}
	triggerRecord, err := uc.backupRecordRepo.GetByID(ctx, triggerRecordID)
	if err != nil || triggerRecord == nil {
		return
	}
	if triggerRecord.Status != DatabaseBackupStatusSuccess || normalizeBackupLevel(triggerRecord.BackupLevel) != DatabaseBackupLevelIncremental {
		return
	}
	policy, err := uc.getBackupPolicy(ctx, policyID)
	if err != nil || policy == nil {
		return
	}
	rule := parseSyntheticRule(policy)
	if !rule.AutoRun || !policy.SyntheticEnabled || !policy.Enabled || policy.Status == DatabaseBackupPolicyStatusDisabled {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, policy.RunnerHostID)
	if err != nil || host == nil {
		uc.markBackupPolicyAutoSyntheticBlocked(ctx, policy, "自动 Synthetic Full 阻塞：Runner 主机不存在")
		return
	}
	validation, err := uc.validateBackupPolicyChainInternal(ctx, policy, host, true)
	if err != nil {
		uc.markBackupPolicyAutoSyntheticBlocked(ctx, policy, "自动 Synthetic Full 阻塞：备份链校验失败: "+err.Error())
		return
	}
	preview := uc.buildSyntheticFullPreview(policy, validation)
	decision := buildSyntheticAutoDecision(policy, rule, validation, preview, triggerRecord)
	if !decision.ShouldRun {
		if decision.Degraded {
			uc.markBackupPolicyAutoSyntheticBlocked(ctx, policy, decision.Reason)
		}
		return
	}
	if uc.hasActiveBackupPolicySyntheticJob(ctx, policy) {
		return
	}
	operator := scheduledBackupOperator()
	operator.Username = "system-auto-synthetic"
	operator.ClientIP = "system"
	if strings.TrimSpace(triggerReason) == "" {
		triggerReason = "auto_after_incremental"
	}
	if _, err := uc.runBackupPolicySyntheticFull(ctx, policy.ID, operator, DatabaseBackupTriggerSchedule, triggerReason, triggerRecord.ID); err != nil && !isBackupTaskRunningError(err) {
		uc.markBackupPolicyAutoSyntheticBlocked(ctx, policy, "自动 Synthetic Full 下发失败: "+err.Error())
	}
}

func buildSyntheticAutoDecision(policy *DatabaseBackupPolicyConfig, rule syntheticRuleConfig, validation *backupPolicyChainValidation, preview *DatabaseSyntheticFullPreviewVO, triggerRecord *DatabaseBackupRecord) syntheticAutoDecision {
	if policy == nil || !policy.Enabled || policy.Status == DatabaseBackupPolicyStatusDisabled {
		return syntheticAutoDecision{Reason: "策略未启用"}
	}
	if !policy.SyntheticEnabled {
		return syntheticAutoDecision{Reason: "策略未开启合成全量"}
	}
	if !rule.AutoRun {
		return syntheticAutoDecision{Reason: "自动合成未开启"}
	}
	if triggerRecord == nil || triggerRecord.ID == 0 {
		return syntheticAutoDecision{Reason: "触发增量记录不存在"}
	}
	if triggerRecord.Status != DatabaseBackupStatusSuccess || normalizeBackupLevel(triggerRecord.BackupLevel) != DatabaseBackupLevelIncremental {
		return syntheticAutoDecision{Reason: "触发记录不是成功增量备份"}
	}
	if validation == nil || validation.Status != DatabaseBackupChainStatusComplete || validation.LatestRecord == nil {
		reason := "自动 Synthetic Full 阻塞：备份链校验未通过"
		if validation != nil && len(validation.BlockingReasons) > 0 {
			reason += ": " + strings.Join(validation.BlockingReasons, "；")
		}
		return syntheticAutoDecision{Degraded: true, Reason: reason}
	}
	incrementalCount := countValidationIncrementals(validation)
	if incrementalCount < rule.TriggerAfterIncrementals {
		return syntheticAutoDecision{Reason: fmt.Sprintf("当前增量数 %d 未达到自动合成阈值 %d", incrementalCount, rule.TriggerAfterIncrementals)}
	}
	if policy.BinlogStreamID == 0 {
		return syntheticAutoDecision{Degraded: true, Reason: "自动 Synthetic Full 阻塞：策略未绑定 binlog 归档流"}
	}
	if rule.MergeOldestIncrementals != rule.TriggerAfterIncrementals {
		return syntheticAutoDecision{Degraded: true, Reason: "自动 Synthetic Full 阻塞：第一版要求 mergeOldestIncrementals 等于 triggerAfterIncrementals，避免部分合并导致旧父链无法清理"}
	}
	if rule.MergeOldestIncrementals > incrementalCount {
		return syntheticAutoDecision{Degraded: true, Reason: fmt.Sprintf("自动 Synthetic Full 阻塞：可合成增量数 %d 小于规则要求 %d", incrementalCount, rule.MergeOldestIncrementals)}
	}
	if validation.LatestRecord.ID != triggerRecord.ID {
		return syntheticAutoDecision{Degraded: true, Reason: fmt.Sprintf("自动 Synthetic Full 阻塞：触发记录 #%d 不是当前最新记录 #%d", triggerRecord.ID, validation.LatestRecord.ID)}
	}
	if policy.LastSyntheticAt != nil && triggerRecord.FinishedAt != nil && !policy.LastSyntheticAt.Before(*triggerRecord.FinishedAt) {
		return syntheticAutoDecision{Reason: "最新增量已被最近一次 Synthetic Full 覆盖"}
	}
	if preview == nil {
		return syntheticAutoDecision{Degraded: true, Reason: "自动 Synthetic Full 阻塞：无法生成合成预览"}
	}
	if len(preview.BlockingReasons) > 0 {
		return syntheticAutoDecision{Degraded: true, Reason: "自动 Synthetic Full 阻塞：" + strings.Join(preview.BlockingReasons, "；")}
	}
	if len(preview.SelectedIncrementalRecordIDs) != rule.MergeOldestIncrementals {
		return syntheticAutoDecision{Degraded: true, Reason: fmt.Sprintf("自动 Synthetic Full 阻塞：预览选择增量数 %d 与规则要求 %d 不一致", len(preview.SelectedIncrementalRecordIDs), rule.MergeOldestIncrementals)}
	}
	if preview.NewSyntheticFullAfterRecordID != triggerRecord.ID {
		return syntheticAutoDecision{Degraded: true, Reason: fmt.Sprintf("自动 Synthetic Full 阻塞：当前链已有 %d 条增量，第一版只自动合并完整窗口，请先人工处理旧窗口", incrementalCount)}
	}
	return syntheticAutoDecision{ShouldRun: true, Reason: "达到自动 Synthetic Full 阈值"}
}

func countValidationIncrementals(validation *backupPolicyChainValidation) int {
	if validation == nil {
		return 0
	}
	count := 0
	for _, item := range validation.Records {
		if item != nil && normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelIncremental {
			count++
		}
	}
	return count
}

func (uc *UseCase) hasActiveBackupPolicySyntheticJob(ctx context.Context, policy *DatabaseBackupPolicyConfig) bool {
	if uc == nil || uc.runnerJobRepo == nil || policy == nil {
		return false
	}
	sourceInstanceID := normalizeBackupSourceInstanceID(policy.SourceInstanceID, policy.InstanceID)
	items, _, err := uc.runnerJobRepo.List(ctx, &DatabaseRunnerJobListRequest{
		Page:             1,
		PageSize:         100,
		JobType:          DatabaseRunnerJobTypeMySQLSyntheticFull,
		SourceInstanceID: sourceInstanceID,
	})
	if err != nil {
		return false
	}
	for _, item := range items {
		if item == nil || !runnerJobRequestMatchesPolicy(item.RequestJSON, policy.ID) {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status == DatabaseRunnerJobStatusQueued || status == DatabaseRunnerJobStatusRunning {
			return true
		}
	}
	return false
}

func (uc *UseCase) markBackupPolicyAutoSyntheticBlocked(ctx context.Context, policy *DatabaseBackupPolicyConfig, message string) {
	if uc == nil || uc.backupPolicyConfigRepo == nil || policy == nil || policy.ID == 0 {
		return
	}
	now := time.Now()
	policy.LastRunAt = &now
	policy.LastStatus = DatabaseBackupStatusFailed
	policy.LastMessage = trimText(message, 500)
	policy.LastError = trimText(message, 1000)
	policy.Status = DatabaseBackupPolicyStatusDegraded
	uc.applyBackupPolicyNextRunAt(policy, now)
	_ = uc.backupPolicyConfigRepo.Update(ctx, policy)
}
