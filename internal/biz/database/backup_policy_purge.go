package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type DatabaseBackupPolicyPurgeRecordVO struct {
	RecordID              uint   `json:"recordId"`
	BackupLevel           string `json:"backupLevel"`
	BackupOrigin          string `json:"backupOrigin"`
	FileName              string `json:"fileName"`
	FileSize              int64  `json:"fileSize"`
	FileSizeText          string `json:"fileSizeText"`
	StorageURI            string `json:"storageUri"`
	FilePath              string `json:"filePath"`
	ChecksumSHA256        string `json:"checksumSha256"`
	Status                string `json:"status"`
	StatusText            string `json:"statusText"`
	ArtifactState         string `json:"artifactState"`
	SupersededByRecordID  uint   `json:"supersededByRecordId"`
	PurgeEligibleAt       string `json:"purgeEligibleAt"`
	ProtectedUntil        string `json:"protectedUntil"`
	RestoreTestStatus     string `json:"restoreTestStatus"`
	RestoreTestStatusText string `json:"restoreTestStatusText"`
	BlockingReason        string `json:"blockingReason,omitempty"`
}

type DatabaseBackupPolicyPurgePreviewVO struct {
	PolicyID                uint                                 `json:"policyId"`
	SyntheticRecordID       uint                                 `json:"syntheticRecordId"`
	RequiredProofRecordID   uint                                 `json:"requiredProofRecordId"`
	ProofStatus             string                               `json:"proofStatus"`
	ProofStatusText         string                               `json:"proofStatusText"`
	BinlogCoverageStatus    string                               `json:"binlogCoverageStatus"`
	BinlogCoverageText      string                               `json:"binlogCoverageText"`
	RetentionDays           int                                  `json:"retentionDays"`
	NeverDeleteWithoutProof bool                                 `json:"neverDeleteWithoutProof"`
	EligibleRecordIDs       []uint                               `json:"eligibleRecordIds"`
	BlockedRecordIDs        []uint                               `json:"blockedRecordIds"`
	StorageDeletePlan       []*DatabaseBackupPolicyPurgeRecordVO `json:"storageDeletePlan"`
	BlockedRecords          []*DatabaseBackupPolicyPurgeRecordVO `json:"blockedRecords"`
	BlockingReasons         []string                             `json:"blockingReasons"`
	Warnings                []string                             `json:"warnings"`
	Messages                []string                             `json:"messages"`
	CheckedAt               string                               `json:"checkedAt"`
}

type DatabaseBackupPolicyPurgeRunVO struct {
	PolicyID          uint                                 `json:"policyId"`
	SyntheticRecordID uint                                 `json:"syntheticRecordId"`
	PurgedRecordIDs   []uint                               `json:"purgedRecordIds"`
	SkippedRecordIDs  []uint                               `json:"skippedRecordIds"`
	StorageDeletePlan []*DatabaseBackupPolicyPurgeRecordVO `json:"storageDeletePlan"`
	Message           string                               `json:"message"`
	ExecutedAt        string                               `json:"executedAt"`
}

type syntheticPurgeInput struct {
	Policy        *DatabaseBackupPolicyConfig
	Synthetic     *DatabaseBackupRecord
	SourceRecords []*DatabaseBackupRecord
	AllRecords    []*DatabaseBackupRecord
	LogArchives   []*DatabaseLogArchive
	Rule          syntheticRuleConfig
	Now           time.Time
}

func (uc *UseCase) PreviewBackupPolicyPurge(ctx context.Context, id uint) (*DatabaseBackupPolicyPurgePreviewVO, error) {
	input, err := uc.buildSyntheticPurgeInput(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.reconcileSyntheticPurgeState(ctx, input); err != nil {
		return nil, err
	}
	return buildSyntheticPurgePreview(input), nil
}

func (uc *UseCase) RunBackupPolicyPurge(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupPolicyPurgeRunVO, error) {
	input, err := uc.buildSyntheticPurgeInput(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.reconcileSyntheticPurgeState(ctx, input); err != nil {
		return nil, err
	}
	preview := buildSyntheticPurgePreview(input)
	if len(preview.BlockingReasons) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(preview.BlockingReasons, "；"))
	}
	if len(preview.EligibleRecordIDs) == 0 {
		return nil, fmt.Errorf("没有到达清理窗口的旧链记录")
	}
	purged := make([]uint, 0, len(preview.EligibleRecordIDs))
	for _, id := range preview.EligibleRecordIDs {
		record := findBackupRecordByID(input.SourceRecords, id)
		if record == nil {
			continue
		}
		record.Status = DatabaseBackupStatusExpired
		record.VerifyStatus = DatabaseBackupVerifyStatusExpired
		record.VerifyMessage = trimText(fmt.Sprintf("已由 synthetic full #%d 完成恢复证明后替代；P2.12 仅做元数据清理标记，artifact 删除交给 Runner/对象存储保留策略", input.Synthetic.ID), 500)
		record.ErrorMessage = trimText(fmt.Sprintf("合成全量 #%d 已通过恢复证明，旧链记录进入 expired/purged 元数据状态；storage URI/checksum 保留用于审计", input.Synthetic.ID), 500)
		if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
			return nil, fmt.Errorf("更新备份记录 #%d 清理状态失败: %w", record.ID, err)
		}
		purged = append(purged, record.ID)
	}
	if uc.backupPolicyConfigRepo != nil && input.Policy != nil {
		now := input.Now
		input.Policy.LastRunAt = &now
		input.Policy.LastStatus = DatabaseBackupStatusExpired
		input.Policy.LastMessage = trimText(fmt.Sprintf("Synthetic full #%d 恢复证明后已标记清理旧链记录 %d 条", input.Synthetic.ID, len(purged)), 500)
		input.Policy.Status = DatabaseBackupPolicyStatusActive
		_ = uc.backupPolicyConfigRepo.Update(ctx, input.Policy)
	}
	if uc.instanceRepo != nil && input.Policy != nil {
		if instance, err := uc.instanceRepo.GetByID(ctx, input.Policy.InstanceID); err == nil && instance != nil {
			auditTask := backupPolicyAuditTask(input.Policy, "synthetic_purge")
			audit, err := uc.startBackupAudit(ctx, instance, auditTask, "policy", operator)
			if err == nil {
				uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, 0, fmt.Sprintf("Synthetic full #%d purge metadata records=%v", input.Synthetic.ID, purged))
			}
		}
	}
	return &DatabaseBackupPolicyPurgeRunVO{
		PolicyID:          input.Policy.ID,
		SyntheticRecordID: input.Synthetic.ID,
		PurgedRecordIDs:   purged,
		SkippedRecordIDs:  append([]uint(nil), preview.BlockedRecordIDs...),
		StorageDeletePlan: preview.StorageDeletePlan,
		Message:           fmt.Sprintf("已标记清理 %d 条旧链记录；未在 backend 中硬删除 artifact", len(purged)),
		ExecutedAt:        input.Now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) buildSyntheticPurgeInput(ctx context.Context, id uint) (*syntheticPurgeInput, error) {
	if uc.backupPolicyConfigRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份策略仓库未配置")
	}
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	records, err := uc.backupRecordRepo.ListSuccessfulPhysicalByPolicy(ctx, policy.ID)
	if err != nil {
		return nil, err
	}
	synthetic := latestSyntheticFullRecord(records)
	if synthetic == nil {
		return nil, fmt.Errorf("当前策略没有成功的 synthetic full 记录")
	}
	sourceIDs := parseSyntheticSourceRecordIDs(synthetic.SyntheticSourceRecordIDs)
	sourceRecords := make([]*DatabaseBackupRecord, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		record := findBackupRecordByID(records, sourceID)
		if record == nil {
			if item, err := uc.backupRecordRepo.GetByID(ctx, sourceID); err == nil && item != nil {
				record = item
			}
		}
		if record != nil {
			sourceRecords = append(sourceRecords, record)
		}
	}
	logs := []*DatabaseLogArchive(nil)
	if uc.logArchiveRepo != nil && policy.BinlogStreamID > 0 {
		logs, _, _ = uc.logArchiveRepo.List(ctx, &DatabaseLogArchiveListRequest{
			Page:        1,
			PageSize:    1000,
			StreamID:    policy.BinlogStreamID,
			InstanceID:  policy.InstanceID,
			ArchiveType: DatabaseArchiveTypeBinlog,
		})
		sort.SliceStable(logs, func(i, j int) bool {
			if logs[i] == nil || logs[j] == nil {
				return i < j
			}
			if logs[i].FileName == logs[j].FileName {
				return logs[i].ID < logs[j].ID
			}
			return logs[i].FileName < logs[j].FileName
		})
	}
	return &syntheticPurgeInput{
		Policy:        policy,
		Synthetic:     synthetic,
		SourceRecords: sourceRecords,
		AllRecords:    records,
		LogArchives:   logs,
		Rule:          parseSyntheticRule(policy),
		Now:           time.Now(),
	}, nil
}

func (uc *UseCase) reconcileSyntheticPurgeState(ctx context.Context, input *syntheticPurgeInput) error {
	if input == nil || input.Synthetic == nil || input.Policy == nil || uc.backupRecordRepo == nil {
		return nil
	}
	if input.Rule.NeverDeleteWithoutProof && input.Synthetic.RestoreTestStatus != DatabaseBackupStatusSuccess {
		return nil
	}
	if !input.Rule.MarkSupersededAfterProof {
		return nil
	}
	eligibleAt := input.Now.AddDate(0, 0, input.Rule.SupersededKeepDaysAfterProof)
	for _, record := range input.SourceRecords {
		if record == nil || record.ID == input.Synthetic.ID || record.Status == DatabaseBackupStatusExpired {
			continue
		}
		changed := false
		if record.SupersededByRecordID != input.Synthetic.ID {
			record.SupersededByRecordID = input.Synthetic.ID
			changed = true
		}
		if record.PurgeEligibleAt == nil {
			record.PurgeEligibleAt = &eligibleAt
			changed = true
		}
		if changed {
			record.ErrorMessage = trimText(fmt.Sprintf("Synthetic full #%d 已通过恢复证明，旧链记录标记为 superseded，清理窗口：%s", input.Synthetic.ID, eligibleAt.Format("2006-01-02 15:04:05")), 500)
			if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildSyntheticPurgePreview(input *syntheticPurgeInput) *DatabaseBackupPolicyPurgePreviewVO {
	now := time.Now()
	if input != nil && !input.Now.IsZero() {
		now = input.Now
	}
	preview := &DatabaseBackupPolicyPurgePreviewVO{
		CheckedAt: now.Format("2006-01-02 15:04:05"),
	}
	if input == nil || input.Policy == nil {
		preview.BlockingReasons = append(preview.BlockingReasons, "备份策略不存在")
		return preview
	}
	preview.PolicyID = input.Policy.ID
	preview.RetentionDays = input.Rule.SupersededKeepDaysAfterProof
	preview.NeverDeleteWithoutProof = input.Rule.NeverDeleteWithoutProof
	if input.Synthetic == nil {
		preview.BlockingReasons = append(preview.BlockingReasons, "没有可用 synthetic full 记录")
		return preview
	}
	preview.SyntheticRecordID = input.Synthetic.ID
	preview.RequiredProofRecordID = input.Synthetic.ID
	preview.ProofStatus = firstNonEmpty(input.Synthetic.RestoreTestStatus, DatabaseBackupStatusPending)
	preview.ProofStatusText = RestoreTestStatusText(preview.ProofStatus)
	preview.BinlogCoverageStatus, preview.BinlogCoverageText = syntheticPurgeBinlogStatus(input)
	globalBlocked := make([]string, 0)
	if input.Synthetic.Status != DatabaseBackupStatusSuccess {
		globalBlocked = append(globalBlocked, fmt.Sprintf("synthetic full #%d 不是成功状态", input.Synthetic.ID))
	}
	if input.Synthetic.ArtifactState == DatabaseBackupArtifactStateMissing || input.Synthetic.ArtifactState == DatabaseBackupArtifactStateChecksumFailed {
		globalBlocked = append(globalBlocked, fmt.Sprintf("synthetic full #%d artifact 状态为 %s", input.Synthetic.ID, input.Synthetic.ArtifactState))
	}
	if strings.TrimSpace(input.Synthetic.ChecksumSHA256) == "" {
		globalBlocked = append(globalBlocked, fmt.Sprintf("synthetic full #%d 缺少 checksum", input.Synthetic.ID))
	}
	if input.Rule.NeverDeleteWithoutProof && input.Synthetic.RestoreTestStatus != DatabaseBackupStatusSuccess {
		globalBlocked = append(globalBlocked, fmt.Sprintf("synthetic full #%d 尚未通过恢复演练 proof", input.Synthetic.ID))
	}
	if preview.BinlogCoverageStatus != DatabaseLogChainStatusComplete {
		globalBlocked = append(globalBlocked, firstNonEmpty(preview.BinlogCoverageText, "binlog 归档链未证明连续"))
	}
	if len(input.SourceRecords) == 0 {
		globalBlocked = append(globalBlocked, "synthetic full 缺少来源记录，不能清理旧链")
	}
	dependentReasons := sourceDependencyBlockReasons(input)
	if len(dependentReasons) > 0 {
		globalBlocked = append(globalBlocked, dependentReasons...)
	}
	preview.BlockingReasons = append(preview.BlockingReasons, uniqueStrings(globalBlocked)...)
	for _, record := range input.SourceRecords {
		if record == nil {
			continue
		}
		reason := firstRecordPurgeBlockReason(record, input.Synthetic, now, globalBlocked)
		vo := toBackupPolicyPurgeRecordVO(record, reason)
		if reason == "" {
			preview.EligibleRecordIDs = append(preview.EligibleRecordIDs, record.ID)
			preview.StorageDeletePlan = append(preview.StorageDeletePlan, vo)
			continue
		}
		preview.BlockedRecordIDs = append(preview.BlockedRecordIDs, record.ID)
		preview.BlockedRecords = append(preview.BlockedRecords, vo)
	}
	if len(preview.EligibleRecordIDs) > 0 {
		preview.Messages = append(preview.Messages, fmt.Sprintf("%d 条旧链记录已到达清理窗口；执行清理仅标记 expired/purged 并保留 storage URI/checksum", len(preview.EligibleRecordIDs)))
	}
	if len(preview.BlockedRecordIDs) > 0 {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("%d 条旧链记录仍受保护或未到清理窗口", len(preview.BlockedRecordIDs)))
	}
	return preview
}

func syntheticPurgeBinlogStatus(input *syntheticPurgeInput) (string, string) {
	if input == nil || input.Policy == nil {
		return DatabaseLogChainStatusUnsupported, "备份策略不存在"
	}
	if input.Policy.BinlogStreamID == 0 {
		return DatabaseLogChainStatusMissingBinlog, "策略未绑定 binlog 归档流，不能证明 PITR 清理安全"
	}
	if input.Synthetic == nil {
		return DatabaseLogChainStatusMissingBinlog, "缺少 synthetic full 记录"
	}
	for _, item := range input.LogArchives {
		if item == nil {
			continue
		}
		if item.Status == DatabaseLogArchiveStatusMissing {
			return DatabaseLogChainStatusMissingBinlog, fmt.Sprintf("binlog %s 被标记为缺失", item.FileName)
		}
		if item.Status == DatabaseLogArchiveStatusChecksumFailed {
			return DatabaseLogChainStatusTimeRangeGap, fmt.Sprintf("binlog %s checksum 异常", item.FileName)
		}
	}
	status, message := validateMySQLRestoreLogMetadata(input.Synthetic, input.LogArchives)
	if status == DatabaseLogChainStatusComplete {
		return status, "binlog catalog 起点和文件链校验通过"
	}
	return status, message
}

func sourceDependencyBlockReasons(input *syntheticPurgeInput) []string {
	if input == nil || input.Synthetic == nil {
		return nil
	}
	sourceSet := make(map[uint]struct{}, len(input.SourceRecords))
	for _, record := range input.SourceRecords {
		if record != nil {
			sourceSet[record.ID] = struct{}{}
		}
	}
	reasons := make([]string, 0)
	for _, record := range input.AllRecords {
		if record == nil || record.ID == input.Synthetic.ID || record.Status == DatabaseBackupStatusExpired {
			continue
		}
		if _, isSource := sourceSet[record.ID]; isSource {
			continue
		}
		if _, dependsOnSource := sourceSet[record.ParentRecordID]; dependsOnSource {
			reasons = append(reasons, fmt.Sprintf("记录 #%d 仍以旧链记录 #%d 为 parent，清理会破坏后续增量依赖", record.ID, record.ParentRecordID))
		}
	}
	return uniqueStrings(reasons)
}

func firstRecordPurgeBlockReason(record, synthetic *DatabaseBackupRecord, now time.Time, globalBlocked []string) string {
	if record == nil {
		return "备份记录不存在"
	}
	if record.Status == DatabaseBackupStatusExpired {
		return "记录已是 expired/purged 状态"
	}
	if len(globalBlocked) > 0 {
		return globalBlocked[0]
	}
	if synthetic == nil || record.SupersededByRecordID != synthetic.ID {
		return "记录尚未被当前 synthetic full 标记 superseded"
	}
	if record.ProtectedUntil != nil && record.ProtectedUntil.After(now) {
		return "记录仍在保护期内：" + record.ProtectedUntil.Format("2006-01-02 15:04:05")
	}
	if record.PurgeEligibleAt == nil {
		return "记录未设置 purge_eligible_at"
	}
	if record.PurgeEligibleAt.After(now) {
		return "清理窗口未到：" + record.PurgeEligibleAt.Format("2006-01-02 15:04:05")
	}
	return ""
}

func latestSyntheticFullRecord(records []*DatabaseBackupRecord) *DatabaseBackupRecord {
	var latest *DatabaseBackupRecord
	for _, record := range records {
		if record == nil || record.BackupOrigin != DatabaseBackupOriginSyntheticFull || normalizeBackupLevel(record.BackupLevel) != DatabaseBackupLevelFull {
			continue
		}
		if latest == nil || backupRecordSortTime(latest).Before(backupRecordSortTime(record)) || (backupRecordSortTime(latest).Equal(backupRecordSortTime(record)) && latest.ID < record.ID) {
			latest = record
		}
	}
	return latest
}

func parseSyntheticSourceRecordIDs(value string) []uint {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(value), &ids); err == nil {
		return ids
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	for _, part := range parts {
		id, err := parseUintText(part)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func toBackupPolicyPurgeRecordVO(record *DatabaseBackupRecord, reason string) *DatabaseBackupPolicyPurgeRecordVO {
	if record == nil {
		return nil
	}
	return &DatabaseBackupPolicyPurgeRecordVO{
		RecordID:              record.ID,
		BackupLevel:           record.BackupLevel,
		BackupOrigin:          record.BackupOrigin,
		FileName:              record.FileName,
		FileSize:              record.FileSize,
		FileSizeText:          humanizeBytes(record.FileSize),
		StorageURI:            record.StorageURI,
		FilePath:              record.FilePath,
		ChecksumSHA256:        record.ChecksumSHA256,
		Status:                record.Status,
		StatusText:            BackupStatusText(record.Status),
		ArtifactState:         record.ArtifactState,
		SupersededByRecordID:  record.SupersededByRecordID,
		PurgeEligibleAt:       formatTime(record.PurgeEligibleAt),
		ProtectedUntil:        formatTime(record.ProtectedUntil),
		RestoreTestStatus:     record.RestoreTestStatus,
		RestoreTestStatusText: RestoreTestStatusText(record.RestoreTestStatus),
		BlockingReason:        reason,
	}
}

func parseUintText(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty")
	}
	var parsed uint64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid uint")
		}
		parsed = parsed*10 + uint64(ch-'0')
	}
	return uint(parsed), nil
}
