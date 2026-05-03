package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DatabaseBackupChainRecordVO struct {
	ID                   uint   `json:"id"`
	ChainID              string `json:"chainId"`
	BaseRecordID         uint   `json:"baseRecordId"`
	ParentRecordID       uint   `json:"parentRecordId"`
	BackupLevel          string `json:"backupLevel"`
	BackupLevelText      string `json:"backupLevelText"`
	BackupOrigin         string `json:"backupOrigin"`
	CheckpointFromLSN    string `json:"checkpointFromLsn"`
	CheckpointToLSN      string `json:"checkpointToLsn"`
	CheckpointLastLSN    string `json:"checkpointLastLsn"`
	FileName             string `json:"fileName"`
	FileSize             int64  `json:"fileSize"`
	ChecksumSHA256       string `json:"checksumSha256"`
	ArtifactState        string `json:"artifactState"`
	StorageURI           string `json:"storageUri"`
	RecoverableFrom      string `json:"recoverableFrom"`
	RecoverableUntil     string `json:"recoverableUntil"`
	Status               string `json:"status"`
	StatusText           string `json:"statusText"`
	StartedAt            string `json:"startedAt"`
	FinishedAt           string `json:"finishedAt"`
	ServerUUID           string `json:"serverUuid"`
	BackupBinlogFile     string `json:"backupBinlogFile"`
	BackupBinlogPos      int64  `json:"backupBinlogPos"`
	BackupGTIDSet        string `json:"backupGtidSet"`
	SyntheticSourceIDs   string `json:"syntheticSourceRecordIds"`
	SupersededByRecordID uint   `json:"supersededByRecordId"`
}

type DatabaseBackupPolicyChainValidationVO struct {
	PolicyID              uint                           `json:"policyId"`
	Status                string                         `json:"status"`
	StatusText            string                         `json:"statusText"`
	BackupChainStatus     string                         `json:"backupChainStatus"`
	BackupChainStatusText string                         `json:"backupChainStatusText"`
	SelectedRecordIDs     []uint                         `json:"selectedRecordIds"`
	BaseRecord            *DatabaseBackupChainRecordVO   `json:"baseRecord,omitempty"`
	LatestRecord          *DatabaseBackupChainRecordVO   `json:"latestRecord,omitempty"`
	Records               []*DatabaseBackupChainRecordVO `json:"records"`
	BlockingReasons       []string                       `json:"blockingReasons"`
	Warnings              []string                       `json:"warnings"`
	Messages              []string                       `json:"messages"`
	CheckedAt             string                         `json:"checkedAt"`
	Chain                 *DatabaseBackupChainStateVO    `json:"chain,omitempty"`
}

type DatabaseSyntheticFullPreviewVO struct {
	PolicyID                      uint                           `json:"policyId"`
	Status                        string                         `json:"status"`
	StatusText                    string                         `json:"statusText"`
	SelectedBaseRecordID          uint                           `json:"selectedBaseRecordId"`
	SelectedIncrementalRecordIDs  []uint                         `json:"selectedIncrementalRecordIds"`
	SelectedRecordIDs             []uint                         `json:"selectedRecordIds"`
	SelectedRecords               []*DatabaseBackupChainRecordVO `json:"selectedRecords"`
	NewSyntheticFullAfterRecordID uint                           `json:"newSyntheticFullAfterRecordId"`
	EstimatedInputSize            int64                          `json:"estimatedInputSize"`
	EstimatedInputSizeText        string                         `json:"estimatedInputSizeText"`
	EstimatedWorkdirSize          int64                          `json:"estimatedWorkdirSize"`
	EstimatedWorkdirSizeText      string                         `json:"estimatedWorkdirSizeText"`
	MergeIncrementalCount         int                            `json:"mergeIncrementalCount"`
	RequiresRestoreProof          bool                           `json:"requiresRestoreProof"`
	BlockingReasons               []string                       `json:"blockingReasons"`
	Warnings                      []string                       `json:"warnings"`
	Messages                      []string                       `json:"messages"`
	CheckedAt                     string                         `json:"checkedAt"`
}

type DatabaseSyntheticFullJobsVO struct {
	Items []*DatabaseRunnerJobVO `json:"items"`
	Total int64                  `json:"total"`
}

type backupPolicyChainValidation struct {
	Policy          *DatabaseBackupPolicyConfig
	Status          string
	BaseRecord      *DatabaseBackupRecord
	LatestRecord    *DatabaseBackupRecord
	Records         []*DatabaseBackupRecord
	BlockingReasons []string
	Warnings        []string
	Messages        []string
	CheckedAt       time.Time
	State           *DatabaseBackupChainState
}

type syntheticRuleConfig struct {
	Mode                         string `json:"mode"`
	AutoRun                      bool   `json:"autoRun"`
	TriggerAfterIncrementals     int    `json:"triggerAfterIncrementals"`
	MergeOldestIncrementals      int    `json:"mergeOldestIncrementals"`
	RequireRestoreProof          bool   `json:"requireRestoreProof"`
	SupersededKeepDaysAfterProof int    `json:"supersededKeepDaysAfterProof"`
	NeverDeleteWithoutProof      bool   `json:"neverDeleteWithoutProof"`
	MarkSupersededAfterProof     bool   `json:"markSupersededAfterProof"`
}

type syntheticFullArtifactInput struct {
	RecordID       uint
	BackupLevel    string
	BackupOrigin   string
	Path           string
	ChecksumSHA256 string
	FileSize       int64
}

func (uc *UseCase) ValidateBackupPolicyChain(ctx context.Context, id uint) (*DatabaseBackupPolicyChainValidationVO, error) {
	if uc.backupPolicyConfigRepo == nil || uc.backupRecordRepo == nil || uc.backupChainStateRepo == nil || uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("备份策略链路仓库未配置")
	}
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, policy.RunnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	validation, err := uc.validateBackupPolicyChainInternal(ctx, policy, host, true)
	if err != nil {
		return nil, err
	}
	return uc.toBackupPolicyChainValidationVO(ctx, validation), nil
}

func (uc *UseCase) PreviewBackupPolicySyntheticFull(ctx context.Context, id uint) (*DatabaseSyntheticFullPreviewVO, error) {
	if uc.backupPolicyConfigRepo == nil || uc.backupRecordRepo == nil || uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("备份策略仓库未配置")
	}
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, policy.RunnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	validation, err := uc.validateBackupPolicyChainInternal(ctx, policy, host, true)
	if err != nil {
		return nil, err
	}
	preview := uc.buildSyntheticFullPreview(policy, validation)
	return preview, nil
}

func (uc *UseCase) RunBackupPolicySyntheticFull(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupPolicyRunVO, error) {
	return uc.runBackupPolicySyntheticFull(ctx, id, operator, DatabaseBackupTriggerManual, "manual", 0)
}

func (uc *UseCase) runBackupPolicySyntheticFull(ctx context.Context, id uint, operator QueryOperator, triggerType string, triggerReason string, triggerRecordID uint) (*DatabaseBackupPolicyRunVO, error) {
	if uc.backupPolicyConfigRepo == nil || uc.backupRecordRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("备份策略 Runner 仓库未配置")
	}
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled || policy.Status == DatabaseBackupPolicyStatusDisabled {
		return nil, fmt.Errorf("备份策略已禁用")
	}
	if !policy.SyntheticEnabled {
		return nil, fmt.Errorf("策略未开启合成全量")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, policy.InstanceID)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	sourceInstance := instance
	if policy.SourceInstanceID > 0 && policy.SourceInstanceID != policy.InstanceID {
		sourceInstance, err = uc.instanceRepo.GetByID(ctx, policy.SourceInstanceID)
		if err != nil || sourceInstance == nil {
			return nil, fmt.Errorf("备份源实例不存在")
		}
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, policy.RunnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
	}
	validation, err := uc.validateBackupPolicyChainInternal(ctx, policy, host, true)
	if err != nil {
		return nil, err
	}
	preview := uc.buildSyntheticFullPreview(policy, validation)
	if len(preview.BlockingReasons) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(preview.BlockingReasons, "；"))
	}
	if err := uc.acquireBackupPolicyRun(policy.ID, policy.InstanceID); err != nil {
		return nil, err
	}
	releaseOnError := true
	defer func() {
		if releaseOnError {
			uc.releaseBackupPolicyRun(policy.ID, policy.InstanceID)
		}
	}()
	auditTask := backupPolicyAuditTask(policy, DatabaseBackupLevelFull)
	audit, err := uc.startBackupAudit(ctx, instance, auditTask, "instance", operator)
	if err != nil {
		return nil, fmt.Errorf("创建备份审计失败: %w", err)
	}
	started := time.Now()
	expiresAt := started.AddDate(0, 0, policyRetentionDays(policy))
	fileName := fmt.Sprintf("%s-synthetic-full-%s.physical.tar.gz", safeBackupName(instance.Name), started.Format("20060102150405"))
	chainID := fmt.Sprintf("mysql-policy-%d-synthetic-%d", policy.ID, started.Unix())
	sourceIDs := make([]uint, 0, len(validation.Records))
	for _, item := range preview.SelectedRecords {
		sourceIDs = append(sourceIDs, item.ID)
	}
	sourceIDsJSON, _ := json.Marshal(sourceIDs)
	rule := parseSyntheticRule(policy)
	restoreTestStatus := ""
	if rule.RequireRestoreProof || policy.RestoreDrillRequired || rule.NeverDeleteWithoutProof {
		restoreTestStatus = DatabaseBackupStatusPending
	}
	normalizedTriggerType := normalizeBackupTriggerType(triggerType)
	if strings.TrimSpace(triggerReason) == "" {
		triggerReason = normalizedTriggerType
	}
	queuedMessage := "MySQL/MariaDB 合成全量任务已进入 Runner 队列"
	if triggerReason == "auto_after_incremental" {
		queuedMessage = "自动 Synthetic Full 已进入 Runner 队列"
	}
	record := &DatabaseBackupRecord{
		PolicyID:                 policy.ID,
		InstanceID:               policy.InstanceID,
		TriggerType:              normalizedTriggerType,
		BackupType:               DatabaseBackupTypePhysical,
		ChainID:                  trimText(chainID, 64),
		ParentRecordID:           preview.NewSyntheticFullAfterRecordID,
		BackupMethod:             DatabaseBackupMethodPhysical,
		BackupLevel:              DatabaseBackupLevelFull,
		BackupEngine:             policy.BackupEngine,
		BackupScope:              "instance",
		ToolName:                 mysqlPhysicalBackupToolName(policy.BackupEngine, policy.Engine),
		SourceInstanceID:         sourceInstance.ID,
		SourceRole:               normalizeSourceRole(policy.SourceRole),
		StorageProfileID:         policy.StorageProfileID,
		StorageType:              DatabaseBackupStorageExternal,
		StorageURI:               trimText(fmt.Sprintf("runner://runner-host-%d/pending/synthetic/%d", host.ID, started.Unix()), 1000),
		BackupOrigin:             DatabaseBackupOriginSyntheticFull,
		ArtifactState:            DatabaseBackupArtifactStateRemote,
		Status:                   DatabaseBackupStatusQueued,
		FileName:                 trimText(fileName, 255),
		Compression:              "gzip",
		ExpiresAt:                &expiresAt,
		VerifyStatus:             DatabaseBackupVerifyStatusPending,
		StartedAt:                &started,
		LastHeartbeatAt:          &started,
		RecoverableFrom:          parseTimePtr(firstNonEmpty(preview.SelectedRecords[0].RecoverableFrom, started.Format("2006-01-02 15:04:05"))),
		RecoverableUntil:         parseTimePtr(firstNonEmpty(preview.SelectedRecords[len(preview.SelectedRecords)-1].RecoverableUntil, started.Format("2006-01-02 15:04:05"))),
		PrepareStatus:            "synthetic_pending",
		SyntheticSourceRecordIDs: string(sourceIDsJSON),
		RestoreTestStatus:        restoreTestStatus,
		ErrorMessage:             queuedMessage,
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建合成全量记录失败: "+err.Error())
		return nil, err
	}
	record.BaseRecordID = record.ID
	if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "更新合成全量记录失败: "+err.Error())
		return nil, err
	}
	policy.LastRunAt = &started
	policy.LastStatus = DatabaseBackupStatusQueued
	policy.LastMessage = queuedMessage
	policy.Status = DatabaseBackupPolicyStatusActive
	uc.applyBackupPolicyNextRunAt(policy, started)
	if err := uc.backupPolicyConfigRepo.Update(ctx, policy); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("更新备份策略状态失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeMySQLSyntheticFull,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: sourceInstance.ID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandMySQLSyntheticFull,
		CommandSummary:   syntheticFullCommandSummary(policy, instance, triggerReason),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      mysqlSyntheticFullRequestJSON(policy, record, host, instance, sourceInstance, operator, sourceIDs, triggerReason, triggerRecordID, rule),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("创建 MySQL/MariaDB 合成全量 Runner Job 失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		uc.finishBackupPolicy(ctx, policy, time.Now(), DatabaseBackupStatusFailed, record.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, err
	}
	selectedRecords := make([]*DatabaseBackupRecord, 0, len(preview.SelectedRecordIDs))
	for _, id := range preview.SelectedRecordIDs {
		if record := findBackupRecordByID(validation.Records, id); record != nil {
			selectedRecords = append(selectedRecords, record)
		}
	}
	releaseOnError = false
	go func() {
		defer uc.releaseBackupPolicyRun(policy.ID, policy.InstanceID)
		uc.executeMySQLSyntheticFullPolicyJob(context.Background(), policy.ID, record.ID, job.ID, audit, selectedRecords)
	}()
	return &DatabaseBackupPolicyRunVO{
		PolicyID:     policy.ID,
		PolicyName:   policy.Name,
		RecordID:     record.ID,
		RunnerJobID:  job.ID,
		InstanceID:   policy.InstanceID,
		InstanceName: instance.Name,
		BackupLevel:  DatabaseBackupLevelFull,
		Status:       DatabaseBackupStatusQueued,
		StatusText:   BackupStatusText(DatabaseBackupStatusQueued),
		FileName:     record.FileName,
		Message:      fmt.Sprintf("%s，Runner Job #%d", queuedMessage, job.ID),
		TriggeredAt:  started.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) ListBackupPolicySyntheticJobs(ctx context.Context, id uint, req *DatabaseRunnerJobListRequest) (*DatabaseSyntheticFullJobsVO, error) {
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	if uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("Runner Job 仓库未配置")
	}
	sourceInstanceID := normalizeBackupSourceInstanceID(policy.SourceInstanceID, policy.InstanceID)
	if req == nil {
		req = &DatabaseRunnerJobListRequest{}
	}
	req.SourceInstanceID = sourceInstanceID
	req.JobType = DatabaseRunnerJobTypeMySQLSyntheticFull
	items, total, err := uc.runnerJobRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	vos := make([]*DatabaseRunnerJobVO, 0, len(items))
	for _, item := range items {
		if item == nil || !runnerJobRequestMatchesPolicy(item.RequestJSON, id) {
			continue
		}
		vos = append(vos, uc.toRunnerJobVO(ctx, item))
	}
	return &DatabaseSyntheticFullJobsVO{Items: vos, Total: total}, nil
}

func (uc *UseCase) validateBackupPolicyChainInternal(ctx context.Context, policy *DatabaseBackupPolicyConfig, host *DatabaseRunnerHost, persist bool) (*backupPolicyChainValidation, error) {
	if policy == nil {
		return nil, fmt.Errorf("备份策略不存在")
	}
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	records, err := uc.backupRecordRepo.ListSuccessfulPhysicalByPolicy(ctx, policy.ID)
	if err != nil {
		return nil, err
	}
	state, _ := uc.loadBackupChainState(ctx, policy.ID)
	validation := validateBackupPolicyChainRecords(policy, host, state, records)
	if persist {
		uc.persistBackupPolicyChainValidation(ctx, policy, validation)
	}
	return validation, nil
}

func validateBackupPolicyChainRecords(policy *DatabaseBackupPolicyConfig, host *DatabaseRunnerHost, state *DatabaseBackupChainState, records []*DatabaseBackupRecord) *backupPolicyChainValidation {
	now := time.Now()
	validation := &backupPolicyChainValidation{
		Policy:    policy,
		Status:    DatabaseBackupChainStatusComplete,
		CheckedAt: now,
		State:     state,
	}
	sort.SliceStable(records, func(i, j int) bool {
		ti := backupRecordSortTime(records[i])
		tj := backupRecordSortTime(records[j])
		if ti.Equal(tj) {
			return records[i].ID < records[j].ID
		}
		return ti.Before(tj)
	})
	byID := make(map[uint]*DatabaseBackupRecord, len(records))
	for _, item := range records {
		if item != nil {
			byID[item.ID] = item
		}
	}
	var base *DatabaseBackupRecord
	if state != nil && state.CurrentBaseRecordID > 0 {
		base = byID[state.CurrentBaseRecordID]
		if base == nil {
			validation.fail(DatabaseBackupChainStatusMissingBase, fmt.Sprintf("当前基础备份记录 #%d 不存在或不是成功物理备份", state.CurrentBaseRecordID))
		}
	}
	if base == nil {
		for i := len(records) - 1; i >= 0; i-- {
			if normalizeBackupLevel(records[i].BackupLevel) == DatabaseBackupLevelFull {
				base = records[i]
				validation.message(fmt.Sprintf("未找到链状态中的基础记录，自动选择最近成功全量 #%d 做校验基线", base.ID))
				break
			}
		}
	}
	if base == nil {
		validation.fail(DatabaseBackupChainStatusMissingBase, "没有可用的成功全量物理备份")
		return validation
	}
	validation.BaseRecord = base
	if err := validatePolicyChainRecord(policy, host, base); err != nil {
		validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("基础备份 #%d 不可用: %v", base.ID, err))
	}
	validation.Records = append(validation.Records, base)
	current := base
	children := make(map[uint][]*DatabaseBackupRecord)
	for _, item := range records {
		if item == nil || item.ParentRecordID == 0 || item.ID == base.ID {
			continue
		}
		if strings.TrimSpace(base.ChainID) != "" && strings.TrimSpace(item.ChainID) != strings.TrimSpace(base.ChainID) {
			continue
		}
		children[item.ParentRecordID] = append(children[item.ParentRecordID], item)
	}
	for parentID := range children {
		sort.SliceStable(children[parentID], func(i, j int) bool {
			ti := backupRecordSortTime(children[parentID][i])
			tj := backupRecordSortTime(children[parentID][j])
			if ti.Equal(tj) {
				return children[parentID][i].ID < children[parentID][j].ID
			}
			return ti.Before(tj)
		})
	}
	visited := map[uint]bool{base.ID: true}
	for {
		nextItems := children[current.ID]
		if len(nextItems) == 0 {
			break
		}
		if len(nextItems) > 1 {
			validation.warn(fmt.Sprintf("父记录 #%d 存在多个子增量，已按时间选择最早子链", current.ID))
		}
		next := nextItems[0]
		if visited[next.ID] {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("备份链出现循环引用: #%d", next.ID))
			break
		}
		visited[next.ID] = true
		if normalizeBackupLevel(next.BackupLevel) != DatabaseBackupLevelIncremental {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("记录 #%d 是子节点但层级不是增量", next.ID))
		}
		if next.BaseRecordID != base.ID {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("增量记录 #%d 的基础记录 #%d 与当前基础 #%d 不一致", next.ID, next.BaseRecordID, base.ID))
		}
		if err := validatePolicyChainRecord(policy, host, next); err != nil {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("增量记录 #%d 不可用: %v", next.ID, err))
		}
		if fromLSN := strings.TrimSpace(next.CheckpointFromLSN); fromLSN != "" {
			if toLSN := strings.TrimSpace(current.CheckpointToLSN); toLSN != "" && fromLSN != toLSN {
				validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("增量记录 #%d from_lsn=%s 与父记录 #%d to_lsn=%s 不连续", next.ID, fromLSN, current.ID, toLSN))
			}
		} else {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("增量记录 #%d 缺少 checkpoint from_lsn", next.ID))
		}
		if strings.TrimSpace(current.ServerUUID) != "" && strings.TrimSpace(next.ServerUUID) != "" && strings.TrimSpace(current.ServerUUID) != strings.TrimSpace(next.ServerUUID) {
			validation.fail(DatabaseBackupChainStatusBrokenChain, fmt.Sprintf("记录 #%d server_uuid 与父记录不一致", next.ID))
		}
		validation.Records = append(validation.Records, next)
		current = next
	}
	validation.LatestRecord = current
	if state != nil && state.LatestRecordID > 0 && state.LatestRecordID != current.ID {
		if latest := byID[state.LatestRecordID]; latest == nil {
			validation.fail(DatabaseBackupChainStatusMissingIncremental, fmt.Sprintf("链状态最新记录 #%d 不存在或不是成功物理备份", state.LatestRecordID))
		} else {
			validation.fail(DatabaseBackupChainStatusMissingIncremental, fmt.Sprintf("链状态最新记录 #%d 无法从基础记录 #%d 连续追溯", state.LatestRecordID, base.ID))
		}
	}
	if policy != nil && policy.BinlogStreamID == 0 {
		validation.warn("策略未绑定 binlog 归档流；备份链可用于增量父链，但 PITR 到任意时间点仍依赖额外 binlog 归档")
	}
	if len(validation.BlockingReasons) == 0 {
		validation.Status = DatabaseBackupChainStatusComplete
		validation.message(fmt.Sprintf("备份链校验通过：基础记录 #%d，最新记录 #%d，增量数 %d", base.ID, current.ID, len(validation.Records)-1))
	}
	return validation
}

func validatePolicyChainRecord(policy *DatabaseBackupPolicyConfig, host *DatabaseRunnerHost, record *DatabaseBackupRecord) error {
	if policy == nil || host == nil || record == nil {
		return fmt.Errorf("备份记录不存在")
	}
	if record.PolicyID != policy.ID {
		return fmt.Errorf("记录不属于当前策略")
	}
	if record.Status != DatabaseBackupStatusSuccess {
		return fmt.Errorf("记录不是成功状态")
	}
	if normalizeBackupMethod(record.BackupMethod) != DatabaseBackupMethodPhysical {
		return fmt.Errorf("记录不是物理备份")
	}
	if strings.TrimSpace(record.BackupEngine) != strings.TrimSpace(policy.BackupEngine) {
		return fmt.Errorf("备份引擎与策略不一致")
	}
	if record.SourceInstanceID != normalizeBackupSourceInstanceID(policy.SourceInstanceID, policy.InstanceID) {
		return fmt.Errorf("来源实例与策略不一致")
	}
	level := normalizeBackupLevel(record.BackupLevel)
	if level != DatabaseBackupLevelFull && level != DatabaseBackupLevelIncremental {
		return fmt.Errorf("备份层级不支持")
	}
	if record.ArtifactState == DatabaseBackupArtifactStateMissing || record.ArtifactState == DatabaseBackupArtifactStateChecksumFailed {
		return fmt.Errorf("artifact 状态不可用")
	}
	if strings.TrimSpace(record.ChecksumSHA256) == "" {
		return fmt.Errorf("缺少 checksum")
	}
	if strings.TrimSpace(record.CheckpointToLSN) == "" {
		return fmt.Errorf("缺少 checkpoint to_lsn")
	}
	if _, err := resolveRunnerReadableArtifactPath(record.StorageURI, record.FilePath, host.ID); err != nil {
		return err
	}
	return nil
}

func (v *backupPolicyChainValidation) fail(status, reason string) {
	if v == nil || strings.TrimSpace(reason) == "" {
		return
	}
	if v.Status == "" || v.Status == DatabaseBackupChainStatusComplete {
		v.Status = status
	}
	v.BlockingReasons = append(v.BlockingReasons, reason)
}

func (v *backupPolicyChainValidation) warn(message string) {
	if v == nil || strings.TrimSpace(message) == "" {
		return
	}
	v.Warnings = append(v.Warnings, message)
}

func (v *backupPolicyChainValidation) message(message string) {
	if v == nil || strings.TrimSpace(message) == "" {
		return
	}
	v.Messages = append(v.Messages, message)
}

func (uc *UseCase) persistBackupPolicyChainValidation(ctx context.Context, policy *DatabaseBackupPolicyConfig, validation *backupPolicyChainValidation) {
	if uc.backupChainStateRepo == nil || policy == nil || validation == nil {
		return
	}
	state := validation.State
	if state == nil {
		state = &DatabaseBackupChainState{
			PolicyID:   policy.ID,
			InstanceID: policy.InstanceID,
		}
	}
	state.InstanceID = policy.InstanceID
	state.LastValidationStatus = validation.Status
	if len(validation.BlockingReasons) > 0 {
		state.Status = DatabaseBackupChainStateBroken
		state.LastError = trimText(strings.Join(validation.BlockingReasons, "；"), 1000)
	} else {
		state.Status = DatabaseBackupChainStateHealthy
		state.LastError = ""
		if validation.BaseRecord != nil {
			state.ChainID = validation.BaseRecord.ChainID
			state.CurrentBaseRecordID = validation.BaseRecord.ID
			if validation.BaseRecord.StartedAt != nil {
				state.ChainStartedAt = validation.BaseRecord.StartedAt
			}
		}
		if validation.LatestRecord != nil {
			state.LatestRecordID = validation.LatestRecord.ID
			state.RecoverableUntil = validation.LatestRecord.RecoverableUntil
			if validation.LatestRecord.FinishedAt != nil {
				state.LastSuccessAt = validation.LatestRecord.FinishedAt
			}
		}
		state.IncrementalCount = 0
		for _, item := range validation.Records {
			if item == nil {
				continue
			}
			if normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelIncremental {
				state.IncrementalCount++
			}
			if normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelFull {
				if item.BackupOrigin == DatabaseBackupOriginSyntheticFull {
					state.LatestSyntheticRecordID = item.ID
				} else {
					state.LatestFullRecordID = item.ID
				}
			}
		}
	}
	if state.ID == 0 {
		_ = uc.backupChainStateRepo.Create(ctx, state)
		return
	}
	_ = uc.backupChainStateRepo.Update(ctx, state)
}

func (uc *UseCase) toBackupPolicyChainValidationVO(ctx context.Context, validation *backupPolicyChainValidation) *DatabaseBackupPolicyChainValidationVO {
	if validation == nil {
		return nil
	}
	selectedIDs := make([]uint, 0, len(validation.Records))
	records := make([]*DatabaseBackupChainRecordVO, 0, len(validation.Records))
	for _, item := range validation.Records {
		if item == nil {
			continue
		}
		selectedIDs = append(selectedIDs, item.ID)
		records = append(records, toBackupChainRecordVO(item))
	}
	state := validation.State
	if validation.Policy != nil && uc != nil && uc.backupChainStateRepo != nil {
		if item, err := uc.backupChainStateRepo.GetByPolicyID(ctx, validation.Policy.ID); err == nil {
			state = item
		}
	}
	policyID := uint(0)
	if validation.Policy != nil {
		policyID = validation.Policy.ID
	}
	return &DatabaseBackupPolicyChainValidationVO{
		PolicyID:              policyID,
		Status:                validation.Status,
		StatusText:            BackupChainStatusText(validation.Status),
		BackupChainStatus:     validation.Status,
		BackupChainStatusText: BackupChainStatusText(validation.Status),
		SelectedRecordIDs:     selectedIDs,
		BaseRecord:            toBackupChainRecordVO(validation.BaseRecord),
		LatestRecord:          toBackupChainRecordVO(validation.LatestRecord),
		Records:               records,
		BlockingReasons:       append([]string(nil), validation.BlockingReasons...),
		Warnings:              append([]string(nil), validation.Warnings...),
		Messages:              append([]string(nil), validation.Messages...),
		CheckedAt:             validation.CheckedAt.Format("2006-01-02 15:04:05"),
		Chain:                 toBackupChainStateVO(state),
	}
}

func (uc *UseCase) buildSyntheticFullPreview(policy *DatabaseBackupPolicyConfig, validation *backupPolicyChainValidation) *DatabaseSyntheticFullPreviewVO {
	now := time.Now()
	rule := parseSyntheticRule(policy)
	preview := &DatabaseSyntheticFullPreviewVO{
		PolicyID:              policy.ID,
		Status:                DatabasePlanValidationPassed,
		StatusText:            ValidationStatusText(DatabasePlanValidationPassed),
		MergeIncrementalCount: rule.MergeOldestIncrementals,
		RequiresRestoreProof:  rule.RequireRestoreProof || policy.RestoreDrillRequired,
		CheckedAt:             now.Format("2006-01-02 15:04:05"),
	}
	if !policy.SyntheticEnabled {
		preview.BlockingReasons = append(preview.BlockingReasons, "策略未开启合成全量")
	}
	if validation == nil || validation.Status != DatabaseBackupChainStatusComplete {
		preview.BlockingReasons = append(preview.BlockingReasons, "备份链校验未通过")
		if validation != nil {
			preview.BlockingReasons = append(preview.BlockingReasons, validation.BlockingReasons...)
			preview.Warnings = append(preview.Warnings, validation.Warnings...)
		}
	}
	if validation == nil || validation.BaseRecord == nil {
		preview.BlockingReasons = append(preview.BlockingReasons, "没有可合成的基础全量")
		preview.finalizeSyntheticPreviewStatus()
		return preview
	}
	preview.SelectedBaseRecordID = validation.BaseRecord.ID
	selected := []*DatabaseBackupRecord{validation.BaseRecord}
	incrementals := make([]*DatabaseBackupRecord, 0)
	for _, item := range validation.Records {
		if item != nil && normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelIncremental {
			incrementals = append(incrementals, item)
		}
	}
	if len(incrementals) < rule.MergeOldestIncrementals {
		preview.BlockingReasons = append(preview.BlockingReasons, fmt.Sprintf("当前可合成增量数 %d，小于规则要求 %d", len(incrementals), rule.MergeOldestIncrementals))
	}
	limit := rule.MergeOldestIncrementals
	if limit > len(incrementals) {
		limit = len(incrementals)
	}
	for i := 0; i < limit; i++ {
		selected = append(selected, incrementals[i])
		preview.SelectedIncrementalRecordIDs = append(preview.SelectedIncrementalRecordIDs, incrementals[i].ID)
	}
	var estimatedInput int64
	for _, item := range selected {
		if item == nil {
			continue
		}
		preview.SelectedRecordIDs = append(preview.SelectedRecordIDs, item.ID)
		preview.SelectedRecords = append(preview.SelectedRecords, toBackupChainRecordVO(item))
		estimatedInput += item.FileSize
	}
	if len(selected) > 0 {
		preview.NewSyntheticFullAfterRecordID = selected[len(selected)-1].ID
	}
	preview.EstimatedInputSize = estimatedInput
	preview.EstimatedInputSizeText = humanizeBytes(estimatedInput)
	preview.EstimatedWorkdirSize = estimatedInput * 3
	preview.EstimatedWorkdirSizeText = humanizeBytes(preview.EstimatedWorkdirSize)
	if preview.RequiresRestoreProof {
		preview.Warnings = append(preview.Warnings, "策略要求恢复证明；合成全量完成后仍建议发起隔离恢复演练再进入长期保留")
	}
	preview.finalizeSyntheticPreviewStatus()
	return preview
}

func (p *DatabaseSyntheticFullPreviewVO) finalizeSyntheticPreviewStatus() {
	if p == nil {
		return
	}
	if len(p.BlockingReasons) > 0 {
		p.Status = DatabasePlanValidationFailed
		p.StatusText = ValidationStatusText(DatabasePlanValidationFailed)
		return
	}
	if len(p.Warnings) > 0 {
		p.Status = DatabasePlanValidationWarning
		p.StatusText = ValidationStatusText(DatabasePlanValidationWarning)
		return
	}
	p.Status = DatabasePlanValidationPassed
	p.StatusText = ValidationStatusText(DatabasePlanValidationPassed)
}

func parseSyntheticRule(policy *DatabaseBackupPolicyConfig) syntheticRuleConfig {
	rule := syntheticRuleConfig{
		Mode:                         "rolling_synthetic_full",
		AutoRun:                      false,
		TriggerAfterIncrementals:     5,
		MergeOldestIncrementals:      5,
		RequireRestoreProof:          true,
		SupersededKeepDaysAfterProof: 7,
		NeverDeleteWithoutProof:      true,
		MarkSupersededAfterProof:     true,
	}
	if policy == nil {
		return rule
	}
	if strings.TrimSpace(policy.SyntheticRuleJSON) != "" {
		_ = json.Unmarshal([]byte(policy.SyntheticRuleJSON), &rule)
	}
	if strings.TrimSpace(rule.Mode) == "" {
		rule.Mode = "rolling_synthetic_full"
	}
	if rule.TriggerAfterIncrementals <= 0 {
		rule.TriggerAfterIncrementals = 5
	}
	if rule.MergeOldestIncrementals <= 0 {
		rule.MergeOldestIncrementals = rule.TriggerAfterIncrementals
	}
	if rule.SupersededKeepDaysAfterProof < 0 {
		rule.SupersededKeepDaysAfterProof = 7
	}
	return rule
}

func syntheticFullCommandSummary(policy *DatabaseBackupPolicyConfig, instance *DatabaseInstance, triggerReason string) string {
	engine := ""
	if policy != nil {
		engine = policy.BackupEngine
	}
	instanceName := ""
	if instance != nil {
		instanceName = instance.Name
	}
	if strings.TrimSpace(triggerReason) == "auto_after_incremental" {
		return fmt.Sprintf("%s %s 自动合成全量", engine, instanceName)
	}
	return fmt.Sprintf("%s %s 合成全量", engine, instanceName)
}

func backupRecordSortTime(item *DatabaseBackupRecord) time.Time {
	if item == nil {
		return time.Time{}
	}
	if item.FinishedAt != nil {
		return *item.FinishedAt
	}
	if item.StartedAt != nil {
		return *item.StartedAt
	}
	return item.CreatedAt
}

func toBackupChainRecordVO(item *DatabaseBackupRecord) *DatabaseBackupChainRecordVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupChainRecordVO{
		ID:                   item.ID,
		ChainID:              item.ChainID,
		BaseRecordID:         item.BaseRecordID,
		ParentRecordID:       item.ParentRecordID,
		BackupLevel:          item.BackupLevel,
		BackupLevelText:      BackupLevelText(item.BackupLevel),
		BackupOrigin:         item.BackupOrigin,
		CheckpointFromLSN:    item.CheckpointFromLSN,
		CheckpointToLSN:      item.CheckpointToLSN,
		CheckpointLastLSN:    item.CheckpointLastLSN,
		FileName:             item.FileName,
		FileSize:             item.FileSize,
		ChecksumSHA256:       item.ChecksumSHA256,
		ArtifactState:        item.ArtifactState,
		StorageURI:           item.StorageURI,
		RecoverableFrom:      formatTime(item.RecoverableFrom),
		RecoverableUntil:     formatTime(item.RecoverableUntil),
		Status:               item.Status,
		StatusText:           BackupStatusText(item.Status),
		StartedAt:            formatTime(item.StartedAt),
		FinishedAt:           formatTime(item.FinishedAt),
		ServerUUID:           item.ServerUUID,
		BackupBinlogFile:     item.BackupBinlogFile,
		BackupBinlogPos:      item.BackupBinlogPos,
		BackupGTIDSet:        item.BackupGTIDSet,
		SyntheticSourceIDs:   item.SyntheticSourceRecordIDs,
		SupersededByRecordID: item.SupersededByRecordID,
	}
}

func parseTimePtr(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}

func findBackupRecordByID(records []*DatabaseBackupRecord, id uint) *DatabaseBackupRecord {
	for _, item := range records {
		if item != nil && item.ID == id {
			return item
		}
	}
	return nil
}

func runnerJobRequestMatchesPolicy(requestJSON string, policyID uint) bool {
	var payload map[string]any
	if err := json.Unmarshal([]byte(requestJSON), &payload); err != nil {
		return true
	}
	switch value := payload["backupPolicyId"].(type) {
	case float64:
		return uint(value) == policyID
	case string:
		parsed, _ := strconv.ParseUint(value, 10, 64)
		return uint(parsed) == policyID
	default:
		return true
	}
}
