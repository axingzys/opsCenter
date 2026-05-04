package database

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DatabaseProtectionModeNone                     = "none"
	DatabaseProtectionModeLogicalBackup            = "logical_backup"
	DatabaseProtectionModeMySQLPhysicalPITR        = "mysql_physical_pitr"
	DatabaseProtectionModeMariaDBPhysicalPITR      = "mariadb_physical_pitr"
	DatabaseProtectionModePostgresBarmanPITR       = "postgres_barman_pitr"
	DatabaseProtectionModePostgresPgBaseBackupPITR = "postgres_pg_basebackup_pitr"
	DatabaseProtectionModeExternalPITR             = "external_pitr"

	DatabaseProtectionLevelNone              = "none"
	DatabaseProtectionLevelBackupOnly        = "backup_only"
	DatabaseProtectionLevelPITRCapable       = "pitr_capable"
	DatabaseProtectionLevelPITRVerified      = "pitr_verified"
	DatabaseProtectionLevelHAAndPITRVerified = "ha_and_pitr_verified"

	DatabaseProtectionHealthHealthy  = "healthy"
	DatabaseProtectionHealthWarning  = "warning"
	DatabaseProtectionHealthCritical = "critical"
	DatabaseProtectionHealthUnknown  = "unknown"

	DatabaseRestoreDrillStatusNone    = "none"
	DatabaseRestoreDrillStatusSuccess = "success"
	DatabaseRestoreDrillStatusFailed  = "failed"
	DatabaseRestoreDrillStatusStale   = "stale"

	protectionProfileIDPrefix       = "instance-"
	protectionRestoreDrillFreshDays = 30
)

type DatabaseProtectionProfileListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	Keyword            string `form:"keyword"`
	InstanceID         uint   `form:"instanceId"`
	Engine             string `form:"engine"`
	ProtectionMode     string `form:"protectionMode"`
	ProtectionLevel    string `form:"protectionLevel"`
	RiskLevel          string `form:"riskLevel"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseProtectionActionVO struct {
	Level    string `json:"level"`
	Action   string `json:"action"`
	Text     string `json:"text"`
	Blocking bool   `json:"blocking"`
}

type DatabaseProtectionProfileVO struct {
	ProfileID               string                       `json:"profileId"`
	InstanceID              uint                         `json:"instanceId"`
	InstanceName            string                       `json:"instanceName"`
	Engine                  string                       `json:"engine"`
	EngineText              string                       `json:"engineText"`
	Version                 string                       `json:"version"`
	Endpoint                string                       `json:"endpoint"`
	Environment             string                       `json:"environment"`
	BusinessSystem          string                       `json:"businessSystem"`
	Owner                   string                       `json:"owner"`
	ProtectionMode          string                       `json:"protectionMode"`
	ProtectionModeText      string                       `json:"protectionModeText"`
	ProtectionLevel         string                       `json:"protectionLevel"`
	ProtectionLevelText     string                       `json:"protectionLevelText"`
	HealthStatus            string                       `json:"healthStatus"`
	HealthStatusText        string                       `json:"healthStatusText"`
	RiskLevel               string                       `json:"riskLevel"`
	RiskLevelText           string                       `json:"riskLevelText"`
	RiskMessages            []string                     `json:"riskMessages"`
	RecoverableFrom         string                       `json:"recoverableFrom"`
	RecoverableUntil        string                       `json:"recoverableUntil"`
	RPOLagSeconds           int                          `json:"rpoLagSeconds"`
	LastFullAt              string                       `json:"lastFullAt"`
	LastIncrementalAt       string                       `json:"lastIncrementalAt"`
	LastSyntheticAt         string                       `json:"lastSyntheticAt"`
	LastLogArchiveAt        string                       `json:"lastLogArchiveAt"`
	LastRestoreDrillAt      string                       `json:"lastRestoreDrillAt"`
	RestoreDrillStatus      string                       `json:"restoreDrillStatus"`
	RestoreDrillStatusText  string                       `json:"restoreDrillStatusText"`
	RunnerStatus            string                       `json:"runnerStatus"`
	RunnerStatusText        string                       `json:"runnerStatusText"`
	StorageStatus           string                       `json:"storageStatus"`
	StorageStatusText       string                       `json:"storageStatusText"`
	BackupChainStatus       string                       `json:"backupChainStatus"`
	BackupChainStatusText   string                       `json:"backupChainStatusText"`
	LogChainStatus          string                       `json:"logChainStatus"`
	LogChainStatusText      string                       `json:"logChainStatusText"`
	ReplicaProtectionStatus string                       `json:"replicaProtectionStatus"`
	ReplicaProtectionText   string                       `json:"replicaProtectionText"`
	BackupPolicy            *DatabaseBackupPolicyVO      `json:"backupPolicy,omitempty"`
	LogicalTask             *DatabaseBackupTaskVO        `json:"logicalTask,omitempty"`
	LatestBackupRecord      *DatabaseBackupRecordVO      `json:"latestBackupRecord,omitempty"`
	LatestLogArchive        *DatabaseLogArchiveVO        `json:"latestLogArchive,omitempty"`
	LatestRestoreJob        *DatabaseRestoreJobVO        `json:"latestRestoreJob,omitempty"`
	LogArchiveStream        *DatabaseLogArchiveStreamVO  `json:"logArchiveStream,omitempty"`
	RunnerHost              *DatabaseRunnerHostVO        `json:"runnerHost,omitempty"`
	StorageProfile          *DatabaseStorageProfileVO    `json:"storageProfile,omitempty"`
	ReplicaProtection       *DatabaseReplicaProtectionVO `json:"replicaProtection,omitempty"`
	RecommendedActions      []DatabaseProtectionActionVO `json:"recommendedActions"`
	ValidatedAt             string                       `json:"validatedAt"`
}

func (uc *UseCase) ListProtectionProfiles(ctx context.Context, req *DatabaseProtectionProfileListRequest) ([]*DatabaseProtectionProfileVO, int64, error) {
	if uc.instanceRepo == nil {
		return nil, 0, fmt.Errorf("实例仓库未配置")
	}
	if req == nil {
		req = &DatabaseProtectionProfileListRequest{}
	}
	normalizeProtectionProfileListRequest(req)
	instanceReq := &DatabaseInstanceListRequest{
		Page:              1,
		PageSize:          10000,
		Keyword:           req.Keyword,
		DBType:            req.Engine,
		RestrictToAllowed: req.RestrictToAllowed,
		AllowedIDs:        req.AllowedInstanceIDs,
	}
	if req.InstanceID > 0 {
		instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
		if err != nil {
			return []*DatabaseProtectionProfileVO{}, 0, nil
		}
		if !profileInstanceAllowed(req, instance.ID) || !profileInstanceMatches(instance, req) {
			return []*DatabaseProtectionProfileVO{}, 0, nil
		}
		profile := uc.buildProtectionProfile(ctx, instance)
		if !profileMatchesRequest(profile, req) {
			return []*DatabaseProtectionProfileVO{}, 0, nil
		}
		return []*DatabaseProtectionProfileVO{profile}, 1, nil
	}
	instances, _, err := uc.instanceRepo.List(ctx, instanceReq)
	if err != nil {
		return nil, 0, err
	}
	profiles := make([]*DatabaseProtectionProfileVO, 0, len(instances))
	for _, instance := range instances {
		if instance == nil || !profileInstanceMatches(instance, req) {
			continue
		}
		profile := uc.buildProtectionProfile(ctx, instance)
		if profileMatchesRequest(profile, req) {
			profiles = append(profiles, profile)
		}
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		left, right := profiles[i], profiles[j]
		if protectionRiskRank(left.RiskLevel) != protectionRiskRank(right.RiskLevel) {
			return protectionRiskRank(left.RiskLevel) > protectionRiskRank(right.RiskLevel)
		}
		if protectionLevelRank(left.ProtectionLevel) != protectionLevelRank(right.ProtectionLevel) {
			return protectionLevelRank(left.ProtectionLevel) < protectionLevelRank(right.ProtectionLevel)
		}
		return left.InstanceID > right.InstanceID
	})
	total := int64(len(profiles))
	page, pageSize := normalizeListPage(req.Page, req.PageSize)
	start := (page - 1) * pageSize
	if start >= len(profiles) {
		return []*DatabaseProtectionProfileVO{}, total, nil
	}
	end := start + pageSize
	if end > len(profiles) {
		end = len(profiles)
	}
	return profiles[start:end], total, nil
}

func (uc *UseCase) GetProtectionProfile(ctx context.Context, profileID string) (*DatabaseProtectionProfileVO, error) {
	instanceID, err := ProtectionProfileInstanceID(profileID)
	if err != nil {
		return nil, err
	}
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("实例仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	return uc.buildProtectionProfile(ctx, instance), nil
}

func (uc *UseCase) ValidateProtectionProfile(ctx context.Context, profileID string) (*DatabaseProtectionProfileVO, error) {
	return uc.GetProtectionProfile(ctx, profileID)
}

func ProtectionProfileInstanceID(profileID string) (uint, error) {
	value := strings.TrimSpace(profileID)
	if value == "" {
		return 0, fmt.Errorf("保护策略ID不能为空")
	}
	if strings.HasPrefix(value, protectionProfileIDPrefix) {
		value = strings.TrimPrefix(value, protectionProfileIDPrefix)
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) >= 2 {
			value = parts[1]
		}
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("保护策略ID格式不正确")
	}
	return uint(id), nil
}

func (uc *UseCase) buildProtectionProfile(ctx context.Context, instance *DatabaseInstance) *DatabaseProtectionProfileVO {
	now := time.Now()
	engine := normalizeDBType(instance.DBType)
	profile := &DatabaseProtectionProfileVO{
		ProfileID:               fmt.Sprintf("%s%d", protectionProfileIDPrefix, instance.ID),
		InstanceID:              instance.ID,
		InstanceName:            instance.Name,
		Engine:                  engine,
		EngineText:              DBTypeText(engine),
		Version:                 instance.Version,
		Endpoint:                netJoinHostPort(instance.Host, instance.Port),
		Environment:             instance.Environment,
		BusinessSystem:          instance.BusinessSystem,
		Owner:                   instance.Owner,
		ProtectionMode:          DatabaseProtectionModeNone,
		ProtectionLevel:         DatabaseProtectionLevelNone,
		HealthStatus:            DatabaseProtectionHealthUnknown,
		RiskLevel:               DatabaseQueryRiskCritical,
		RPOLagSeconds:           -1,
		RestoreDrillStatus:      DatabaseRestoreDrillStatusNone,
		BackupChainStatus:       DatabaseBackupChainStatusMissingBase,
		LogChainStatus:          DatabaseLogChainStatusUnsupported,
		RunnerStatus:            DatabaseRunnerHostStatusPending,
		StorageStatus:           "unknown",
		ReplicaProtectionStatus: DatabaseReplicaProtectionUnknown,
		ValidatedAt:             now.Format("2006-01-02 15:04:05"),
	}

	policy := uc.selectProtectionPolicy(ctx, instance.ID)
	if policy != nil {
		profile.BackupPolicy = uc.toBackupPolicyVO(ctx, policy)
		profile.ProtectionMode = protectionModeForPolicy(engine, policy)
		profile.LastFullAt = profile.BackupPolicy.LastFullAt
		profile.LastIncrementalAt = profile.BackupPolicy.LastIncrementalAt
		profile.LastSyntheticAt = profile.BackupPolicy.LastSyntheticAt
		profile.LastRestoreDrillAt = profile.BackupPolicy.LastRestoreDrillAt
		if profile.BackupPolicy.Chain != nil {
			profile.BackupChainStatus = backupChainStatusForProfile(profile.BackupPolicy.Chain)
			profile.BackupChainStatusText = BackupChainStatusText(profile.BackupChainStatus)
			profile.RecoverableFrom = profile.BackupPolicy.Chain.ChainStartedAt
			profile.RecoverableUntil = profile.BackupPolicy.Chain.RecoverableUntil
		}
	}
	if profile.BackupPolicy == nil {
		if task := uc.selectLogicalBackupTask(ctx, instance.ID); task != nil {
			profile.LogicalTask = task
			profile.ProtectionMode = DatabaseProtectionModeLogicalBackup
			profile.ProtectionLevel = DatabaseProtectionLevelBackupOnly
			profile.LastFullAt = task.LastSuccessAt
			profile.LastRestoreDrillAt = task.LastRestoreTestAt
			profile.BackupChainStatus = DatabaseBackupChainStatusUnsupported
			profile.BackupChainStatusText = BackupChainStatusText(profile.BackupChainStatus)
		}
	}

	latestRecord := uc.latestSuccessfulBackupRecord(ctx, instance.ID)
	if latestRecord != nil {
		profile.LatestBackupRecord = uc.toBackupRecordVO(latestRecord, "", instance.Name)
		profile.LastFullAt = firstNonEmpty(profile.LastFullAt, latestBackupRecordTime(latestRecord, DatabaseBackupLevelFull))
		profile.RecoverableFrom = firstNonEmpty(profile.RecoverableFrom, formatTime(latestRecord.RecoverableFrom), formatTime(latestRecord.StartedAt))
		profile.RecoverableUntil = firstNonEmpty(profile.RecoverableUntil, formatTime(latestRecord.RecoverableUntil), formatTime(latestRecord.FinishedAt))
	}

	stream := uc.selectProtectionLogArchiveStream(ctx, instance.ID, engine, profile.BackupPolicy)
	if stream != nil {
		profile.LogArchiveStream = uc.toLogArchiveStreamVO(ctx, stream)
		profile.LogChainStatus = logChainStatusForStream(stream)
		profile.LogChainStatusText = LogChainStatusText(profile.LogChainStatus)
		profile.RPOLagSeconds = stream.ArchiveLagSeconds
		profile.LastLogArchiveAt = firstNonEmpty(formatTime(stream.LastArchivedAt), formatTime(stream.LastEventTime), formatTime(stream.LastHeartbeatAt))
		if profile.RecoverableUntil == "" && stream.LastEventTime != nil {
			profile.RecoverableUntil = formatTime(stream.LastEventTime)
		}
	}
	latestArchive := uc.latestArchivedLog(ctx, instance.ID, archiveTypeForEngine(engine))
	if latestArchive != nil {
		profile.LatestLogArchive = uc.toLogArchiveVO(ctx, latestArchive)
		profile.LastLogArchiveAt = firstNonEmpty(profile.LastLogArchiveAt, formatTime(latestArchive.ArchivedAt), formatTime(latestArchive.LastEventTime))
		if profile.RecoverableUntil == "" && latestArchive.LastEventTime != nil {
			profile.RecoverableUntil = formatTime(latestArchive.LastEventTime)
		}
	}

	profile.RunnerHost = uc.resolveProtectionRunner(ctx, profile.BackupPolicy, profile.LogArchiveStream)
	if profile.RunnerHost != nil {
		profile.RunnerStatus = profile.RunnerHost.Status
		profile.RunnerStatusText = profile.RunnerHost.StatusText
	}
	profile.StorageProfile = uc.resolveProtectionStorage(ctx, profile.BackupPolicy, profile.LogArchiveStream, profile.LogicalTask)
	if profile.StorageProfile != nil {
		profile.StorageStatus = normalizeStoragePostureStatus(profile.StorageProfile.PostureStatus)
		profile.StorageStatusText = StoragePostureStatusText(profile.StorageProfile.PostureStatus)
	}

	if job := uc.latestRestoreJob(ctx, instance.ID); job != nil {
		profile.LatestRestoreJob = job
		profile.RestoreDrillStatus = restoreDrillStatusForJob(job, now)
		if profile.LastRestoreDrillAt == "" && (job.Status == DatabaseRestoreStatusVerified || job.Status == DatabaseBackupStatusSuccess) {
			profile.LastRestoreDrillAt = job.FinishedAt
		}
	}
	if profile.RestoreDrillStatus == DatabaseRestoreDrillStatusNone && profile.LastRestoreDrillAt != "" {
		profile.RestoreDrillStatus = restoreDrillStatusFromTime(profile.LastRestoreDrillAt, now)
	}

	profile.ReplicaProtection = uc.protectionReplicaStatus(ctx, instance.ID, engine)
	if profile.ReplicaProtection != nil {
		profile.ReplicaProtectionStatus = profile.ReplicaProtection.ProtectionStatus
		profile.ReplicaProtectionText = profile.ReplicaProtection.ProtectionStatusText
	}

	profile.applyProtectionAssessment(now)
	return profile
}

func (profile *DatabaseProtectionProfileVO) applyProtectionAssessment(now time.Time) {
	actions := make([]DatabaseProtectionActionVO, 0, 8)
	risks := make([]string, 0, 8)
	critical := false
	high := false
	medium := false
	pitrCandidate := isPITRProtectionMode(profile.ProtectionMode)

	if profile.BackupPolicy == nil && profile.LogicalTask == nil && profile.LatestBackupRecord == nil {
		critical = true
		risks = append(risks, "当前实例没有启用备份策略或成功备份记录")
		actions = append(actions, protectionAction(DatabaseQueryRiskCritical, "enable_protection", "请先启用数据库保护策略或创建备份任务", true))
	}
	if profile.BackupPolicy != nil {
		if !profile.BackupPolicy.Enabled || profile.BackupPolicy.Status == DatabaseBackupPolicyStatusDisabled {
			high = true
			risks = append(risks, "物理备份策略已禁用")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "enable_backup_policy", "启用物理备份策略", true))
		}
		if profile.BackupPolicy.LastStatus == DatabaseBackupStatusFailed || profile.BackupPolicy.Status == DatabaseBackupPolicyStatusFailed {
			critical = true
			risks = append(risks, "最近一次物理备份失败")
			actions = append(actions, protectionAction(DatabaseQueryRiskCritical, "inspect_backup_failure", "检查最近一次物理备份失败原因", true))
		}
		if profile.BackupPolicy.Chain == nil || profile.BackupPolicy.Chain.CurrentBaseRecordID == 0 {
			critical = true
			risks = append(risks, "当前物理备份链缺少可用全量基线")
			actions = append(actions, protectionAction(DatabaseQueryRiskCritical, "run_full_backup", "先执行一次 full 备份建立基线", true))
		}
		switch profile.ChainStatus() {
		case DatabaseBackupChainStateBroken:
			critical = true
			risks = append(risks, "当前物理备份链已断链")
			actions = append(actions, protectionAction(DatabaseQueryRiskCritical, "validate_backup_chain", "校验并修复物理备份链", true))
		case DatabaseBackupChainStateDegraded:
			high = true
			risks = append(risks, "当前物理备份链处于降级状态")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "validate_backup_chain", "校验物理备份链并查看降级原因", true))
		case DatabaseBackupChainStateConsolidating:
			medium = true
			risks = append(risks, "当前物理备份链正在合成全量")
		}
	}
	if profile.LogicalTask != nil && profile.LogicalTask.LastStatus == DatabaseBackupStatusFailed {
		high = true
		risks = append(risks, "最近一次逻辑备份失败")
		actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "inspect_logical_backup_failure", "检查逻辑备份任务失败原因", true))
	}
	if pitrCandidate {
		if profile.LogArchiveStream == nil {
			high = true
			risks = append(risks, "未绑定 binlog/WAL 归档流，PITR 只能恢复到备份点")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "start_log_archive_stream", "创建并启动日志归档流", true))
		} else {
			switch profile.LogArchiveStream.Status {
			case DatabaseLogArchiveStreamStatusFailed:
				critical = true
				risks = append(risks, "日志归档流失败")
				actions = append(actions, protectionAction(DatabaseQueryRiskCritical, "inspect_log_archive_stream", "检查日志归档流失败原因", true))
			case DatabaseLogArchiveStreamStatusDegraded:
				high = true
				risks = append(risks, "日志归档流降级")
			case DatabaseLogArchiveStreamStatusPaused, DatabaseLogArchiveStreamStatusDisabled:
				high = true
				risks = append(risks, "日志归档流未运行")
				actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "resume_log_archive_stream", "启动或恢复日志归档流", true))
			}
			if profile.LogArchiveStream.RPOTargetSeconds > 0 && profile.RPOLagSeconds > profile.LogArchiveStream.RPOTargetSeconds {
				medium = true
				risks = append(risks, "日志归档延迟超过目标 RPO")
				actions = append(actions, protectionAction(DatabaseQueryRiskMedium, "inspect_archive_lag", "检查日志归档延迟和 Agent 心跳", false))
			}
			if strings.TrimSpace(profile.LogArchiveStream.LastError) != "" {
				high = true
				risks = append(risks, "日志归档流存在最近错误")
			}
		}
	}
	if profile.RunnerHost == nil && (profile.BackupPolicy != nil || profile.LogArchiveStream != nil) {
		high = true
		risks = append(risks, "当前保护策略缺少 Runner 主机")
		actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "configure_runner", "配置并绑定 Runner 主机", true))
	}
	if profile.RunnerHost != nil {
		if !profile.RunnerHost.Enabled || profile.RunnerHost.Status == DatabaseRunnerHostStatusDisabled || profile.RunnerHost.Status == DatabaseRunnerHostStatusFailed {
			high = true
			risks = append(risks, "Runner 主机不可用")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "repair_runner", "修复 Runner 主机或切换到可用 Runner", true))
		} else if profile.RunnerHost.Status != DatabaseRunnerHostStatusOnline {
			medium = true
			risks = append(risks, "Runner 主机未确认在线")
		}
	}
	if profile.StorageProfile != nil {
		switch profile.StorageStatus {
		case "failed":
			high = true
			risks = append(risks, "备份存储安全姿态检测失败")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "check_storage_posture", "重新检测备份存储安全姿态", true))
		case "warning", "unknown":
			medium = true
			risks = append(risks, "备份存储安全姿态未完全验证")
		}
	}
	if pitrCandidate {
		switch profile.RestoreDrillStatus {
		case DatabaseRestoreDrillStatusFailed:
			high = true
			risks = append(risks, "最近一次恢复演练失败")
			actions = append(actions, protectionAction(DatabaseQueryRiskHigh, "run_restore_drill", "重新执行隔离恢复演练", true))
		case DatabaseRestoreDrillStatusNone:
			medium = true
			risks = append(risks, "尚未完成恢复演练 proof")
			actions = append(actions, protectionAction(DatabaseQueryRiskMedium, "run_restore_drill", "执行一次隔离恢复演练并生成 proof", false))
		case DatabaseRestoreDrillStatusStale:
			medium = true
			risks = append(risks, "恢复演练 proof 已超过 30 天")
			actions = append(actions, protectionAction(DatabaseQueryRiskMedium, "run_restore_drill", "刷新恢复演练 proof", false))
		}
	}
	if profile.ReplicaProtection != nil && profile.ReplicaProtection.ProtectionStatus != DatabaseReplicaProtectionProtected {
		medium = true
		risks = append(risks, "延迟副本保护窗口不可用或降级")
	}

	profile.RiskMessages = uniqueStrings(risks)
	profile.RecommendedActions = dedupeProtectionActions(actions)
	if critical {
		profile.RiskLevel = DatabaseQueryRiskCritical
		profile.HealthStatus = DatabaseProtectionHealthCritical
	} else if high {
		profile.RiskLevel = DatabaseQueryRiskHigh
		profile.HealthStatus = DatabaseProtectionHealthCritical
	} else if medium {
		profile.RiskLevel = DatabaseQueryRiskMedium
		profile.HealthStatus = DatabaseProtectionHealthWarning
	} else {
		profile.RiskLevel = DatabaseQueryRiskLow
		profile.HealthStatus = DatabaseProtectionHealthHealthy
	}
	profile.ProtectionLevel = profile.deriveProtectionLevel()
	profile.ProtectionModeText = ProtectionModeText(profile.ProtectionMode)
	profile.ProtectionLevelText = ProtectionLevelText(profile.ProtectionLevel)
	profile.HealthStatusText = ProtectionHealthStatusText(profile.HealthStatus)
	profile.RiskLevelText = RiskLevelText(profile.RiskLevel)
	profile.RestoreDrillStatusText = RestoreDrillStatusText(profile.RestoreDrillStatus)
	profile.BackupChainStatusText = BackupChainStatusText(profile.BackupChainStatus)
	profile.LogChainStatusText = LogChainStatusText(profile.LogChainStatus)
	if profile.RunnerStatusText == "" {
		profile.RunnerStatusText = RunnerHostStatusText(profile.RunnerStatus)
	}
	if profile.StorageStatusText == "" {
		profile.StorageStatusText = StoragePostureStatusText(profile.StorageStatus)
	}
	if profile.ReplicaProtectionText == "" {
		profile.ReplicaProtectionText = ReplicaProtectionStatusText(profile.ReplicaProtectionStatus)
	}
}

func (profile *DatabaseProtectionProfileVO) ChainStatus() string {
	if profile == nil || profile.BackupPolicy == nil || profile.BackupPolicy.Chain == nil {
		return ""
	}
	return profile.BackupPolicy.Chain.Status
}

func (profile *DatabaseProtectionProfileVO) deriveProtectionLevel() string {
	if profile == nil || (profile.BackupPolicy == nil && profile.LogicalTask == nil && profile.LatestBackupRecord == nil) {
		return DatabaseProtectionLevelNone
	}
	if !isPITRProtectionMode(profile.ProtectionMode) {
		return DatabaseProtectionLevelBackupOnly
	}
	chainOK := profile.BackupPolicy != nil && profile.BackupPolicy.Chain != nil &&
		profile.BackupPolicy.Chain.CurrentBaseRecordID > 0 &&
		(profile.BackupPolicy.Chain.Status == "" ||
			profile.BackupPolicy.Chain.Status == DatabaseBackupChainStateHealthy ||
			profile.BackupPolicy.Chain.Status == DatabaseBackupChainStateConsolidating)
	logOK := profile.LogArchiveStream != nil &&
		profile.LogArchiveStream.Enabled &&
		profile.LogArchiveStream.Status != DatabaseLogArchiveStreamStatusFailed &&
		profile.LogArchiveStream.Status != DatabaseLogArchiveStreamStatusDisabled &&
		profile.LogArchiveStream.Status != DatabaseLogArchiveStreamStatusPaused
	if !chainOK || !logOK {
		return DatabaseProtectionLevelBackupOnly
	}
	if profile.RestoreDrillStatus == DatabaseRestoreDrillStatusSuccess {
		if profile.ReplicaProtection != nil && profile.ReplicaProtection.ProtectionStatus == DatabaseReplicaProtectionProtected {
			return DatabaseProtectionLevelHAAndPITRVerified
		}
		return DatabaseProtectionLevelPITRVerified
	}
	return DatabaseProtectionLevelPITRCapable
}

func (uc *UseCase) selectProtectionPolicy(ctx context.Context, instanceID uint) *DatabaseBackupPolicyConfig {
	if uc == nil || uc.backupPolicyConfigRepo == nil || instanceID == 0 {
		return nil
	}
	items, _, err := uc.backupPolicyConfigRepo.List(ctx, &DatabaseBackupPolicyListRequest{Page: 1, PageSize: 100, InstanceID: instanceID})
	if err != nil {
		return nil
	}
	var selected *DatabaseBackupPolicyConfig
	for _, item := range items {
		if item == nil {
			continue
		}
		if selected == nil {
			selected = item
		}
		if item.Enabled && item.Status != DatabaseBackupPolicyStatusDisabled {
			if selected == nil || !selected.Enabled || item.ID > selected.ID {
				selected = item
			}
		}
	}
	return selected
}

func (uc *UseCase) selectLogicalBackupTask(ctx context.Context, instanceID uint) *DatabaseBackupTaskVO {
	if uc == nil || uc.backupTaskRepo == nil || instanceID == 0 {
		return nil
	}
	items, _, err := uc.backupTaskRepo.List(ctx, &DatabaseBackupTaskListRequest{Page: 1, PageSize: 100, InstanceID: instanceID})
	if err != nil {
		return nil
	}
	var selected *DatabaseBackupTask
	for _, item := range items {
		if item == nil {
			continue
		}
		if normalizeBackupMethod(item.BackupMethod) != DatabaseBackupMethodLogical && item.BackupType != DatabaseBackupTypeLogical {
			continue
		}
		if selected == nil || betterProtectionTask(item, selected) {
			selected = item
		}
	}
	if selected == nil {
		return nil
	}
	instanceName, instanceDBType := "", ""
	if uc.instanceRepo != nil {
		if instance, err := uc.instanceRepo.GetByID(ctx, selected.InstanceID); err == nil && instance != nil {
			instanceName = instance.Name
			instanceDBType = instance.DBType
		}
	}
	return uc.toBackupTaskVO(ctx, selected, instanceName, instanceDBType)
}

func (uc *UseCase) latestSuccessfulBackupRecord(ctx context.Context, instanceID uint) *DatabaseBackupRecord {
	if uc == nil || uc.backupRecordRepo == nil || instanceID == 0 {
		return nil
	}
	items, _, err := uc.backupRecordRepo.List(ctx, &DatabaseBackupRecordListRequest{Page: 1, PageSize: 1, InstanceID: instanceID, Status: DatabaseBackupStatusSuccess})
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (uc *UseCase) selectProtectionLogArchiveStream(ctx context.Context, instanceID uint, engine string, policy *DatabaseBackupPolicyVO) *DatabaseLogArchiveStream {
	if uc == nil || uc.logArchiveStreamRepo == nil || instanceID == 0 {
		return nil
	}
	if policy != nil && policy.BinlogStreamID > 0 {
		if stream, err := uc.logArchiveStreamRepo.GetByID(ctx, policy.BinlogStreamID); err == nil && stream != nil {
			return stream
		}
	}
	archiveType := archiveTypeForEngine(engine)
	if archiveType == "" || archiveType == DatabaseArchiveTypeNone {
		return nil
	}
	items, _, err := uc.logArchiveStreamRepo.List(ctx, &DatabaseLogArchiveStreamListRequest{Page: 1, PageSize: 20, InstanceID: instanceID, ArchiveType: archiveType})
	if err != nil {
		return nil
	}
	var selected *DatabaseLogArchiveStream
	for _, item := range items {
		if item == nil {
			continue
		}
		if selected == nil {
			selected = item
		}
		if item.Enabled && item.Status == DatabaseLogArchiveStreamStatusRunning {
			return item
		}
		if item.Enabled && selected != nil && !selected.Enabled {
			selected = item
		}
	}
	return selected
}

func (uc *UseCase) latestArchivedLog(ctx context.Context, instanceID uint, archiveType string) *DatabaseLogArchive {
	if uc == nil || uc.logArchiveRepo == nil || instanceID == 0 || archiveType == "" || archiveType == DatabaseArchiveTypeNone {
		return nil
	}
	items, _, err := uc.logArchiveRepo.List(ctx, &DatabaseLogArchiveListRequest{Page: 1, PageSize: 1, InstanceID: instanceID, ArchiveType: archiveType, Status: DatabaseLogArchiveStatusArchived})
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (uc *UseCase) resolveProtectionRunner(ctx context.Context, policy *DatabaseBackupPolicyVO, stream *DatabaseLogArchiveStreamVO) *DatabaseRunnerHostVO {
	if uc == nil || uc.runnerHostRepo == nil {
		return nil
	}
	runnerID := uint(0)
	if policy != nil && policy.RunnerHostID > 0 {
		runnerID = policy.RunnerHostID
	} else if stream != nil && stream.RunnerHostID > 0 {
		runnerID = stream.RunnerHostID
	}
	if runnerID == 0 {
		return nil
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerID)
	if err != nil {
		return nil
	}
	return toRunnerHostVO(host)
}

func (uc *UseCase) resolveProtectionStorage(ctx context.Context, policy *DatabaseBackupPolicyVO, stream *DatabaseLogArchiveStreamVO, logicalTask *DatabaseBackupTaskVO) *DatabaseStorageProfileVO {
	if uc == nil || uc.storageProfileRepo == nil {
		return nil
	}
	storageID := uint(0)
	if policy != nil && policy.StorageProfileID > 0 {
		storageID = policy.StorageProfileID
	} else if stream != nil && stream.StorageProfileID > 0 {
		storageID = stream.StorageProfileID
	} else if logicalTask != nil && logicalTask.StorageProfileID > 0 {
		storageID = logicalTask.StorageProfileID
	}
	if storageID == 0 {
		return nil
	}
	item, err := uc.storageProfileRepo.GetByID(ctx, storageID)
	if err != nil {
		return nil
	}
	return toStorageProfileVO(item)
}

func (uc *UseCase) latestRestoreJob(ctx context.Context, instanceID uint) *DatabaseRestoreJobVO {
	if uc == nil || uc.restoreJobRepo == nil || instanceID == 0 {
		return nil
	}
	items, _, err := uc.restoreJobRepo.List(ctx, &DatabaseRestoreJobListRequest{Page: 1, PageSize: 1, SourceInstanceID: instanceID})
	if err != nil || len(items) == 0 {
		return nil
	}
	job := items[0]
	sourceName := ""
	if uc.instanceRepo != nil {
		if instance, err := uc.instanceRepo.GetByID(ctx, job.SourceInstanceID); err == nil && instance != nil {
			sourceName = instance.Name
		}
	}
	runnerName := ""
	if uc.runnerHostRepo != nil && job.RunnerHostID > 0 {
		if host, err := uc.runnerHostRepo.GetByID(ctx, job.RunnerHostID); err == nil && host != nil {
			runnerName = host.Name
		}
	}
	return uc.toRestoreJobVO(job, sourceName, "", "", runnerName)
}

func (uc *UseCase) protectionReplicaStatus(ctx context.Context, instanceID uint, engine string) *DatabaseReplicaProtectionVO {
	if uc == nil || instanceID == 0 || !isReplicaGovernanceEngine(engine) {
		return nil
	}
	list, _, err := uc.ListReplicaProtections(ctx, &DatabaseReplicaProtectionListRequest{Page: 1, PageSize: 1, InstanceID: instanceID})
	if err != nil || len(list) == 0 {
		return nil
	}
	return list[0]
}

func normalizeProtectionProfileListRequest(req *DatabaseProtectionProfileListRequest) {
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
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.Engine = normalizeDBType(req.Engine)
	req.ProtectionMode = strings.TrimSpace(req.ProtectionMode)
	req.ProtectionLevel = strings.TrimSpace(req.ProtectionLevel)
	req.RiskLevel = strings.TrimSpace(req.RiskLevel)
}

func profileInstanceAllowed(req *DatabaseProtectionProfileListRequest, instanceID uint) bool {
	if req == nil || !req.RestrictToAllowed {
		return true
	}
	for _, allowedID := range req.AllowedInstanceIDs {
		if allowedID == instanceID {
			return true
		}
	}
	return false
}

func profileInstanceMatches(instance *DatabaseInstance, req *DatabaseProtectionProfileListRequest) bool {
	if instance == nil || req == nil {
		return true
	}
	if req.Engine != "" && normalizeDBType(instance.DBType) != req.Engine {
		return false
	}
	if req.Keyword != "" {
		kw := strings.ToLower(req.Keyword)
		text := strings.ToLower(strings.Join([]string{instance.Name, instance.Host, instance.DefaultDatabase, instance.BusinessSystem, instance.Owner}, " "))
		if !strings.Contains(text, kw) {
			return false
		}
	}
	return true
}

func profileMatchesRequest(profile *DatabaseProtectionProfileVO, req *DatabaseProtectionProfileListRequest) bool {
	if profile == nil || req == nil {
		return true
	}
	if req.ProtectionMode != "" && profile.ProtectionMode != req.ProtectionMode {
		return false
	}
	if req.ProtectionLevel != "" && profile.ProtectionLevel != req.ProtectionLevel {
		return false
	}
	if req.RiskLevel != "" && profile.RiskLevel != req.RiskLevel {
		return false
	}
	return true
}

func protectionModeForPolicy(engine string, policy *DatabaseBackupPolicyConfig) string {
	if policy == nil {
		return DatabaseProtectionModeNone
	}
	backupEngine := normalizeProtectionBackupEngine(policy.BackupEngine)
	switch normalizeDBType(engine) {
	case DBTypeMySQL:
		return DatabaseProtectionModeMySQLPhysicalPITR
	case DBTypeMariaDB:
		return DatabaseProtectionModeMariaDBPhysicalPITR
	case DBTypePostgreSQL:
		if backupEngine == "barman" {
			return DatabaseProtectionModePostgresBarmanPITR
		}
		if backupEngine == "pg_basebackup" {
			return DatabaseProtectionModePostgresPgBaseBackupPITR
		}
		return DatabaseProtectionModePostgresBarmanPITR
	default:
		if backupEngine == "external" {
			return DatabaseProtectionModeExternalPITR
		}
		return DatabaseProtectionModeExternalPITR
	}
}

func normalizeProtectionBackupEngine(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func backupChainStatusForProfile(chain *DatabaseBackupChainStateVO) string {
	if chain == nil || chain.CurrentBaseRecordID == 0 {
		return DatabaseBackupChainStatusMissingBase
	}
	switch chain.Status {
	case DatabaseBackupChainStateHealthy, DatabaseBackupChainStateConsolidating:
		return DatabaseBackupChainStatusComplete
	case DatabaseBackupChainStateDegraded:
		return DatabaseBackupChainStatusMissingIncremental
	case DatabaseBackupChainStateBroken:
		return DatabaseBackupChainStatusBrokenChain
	default:
		return DatabaseBackupChainStatusComplete
	}
}

func logChainStatusForStream(stream *DatabaseLogArchiveStream) string {
	if stream == nil {
		return DatabaseLogChainStatusUnsupported
	}
	if stream.ArchiveType == DatabaseArchiveTypeBinlog {
		if stream.Status == DatabaseLogArchiveStreamStatusFailed || stream.Status == DatabaseLogArchiveStreamStatusDisabled || stream.Status == DatabaseLogArchiveStreamStatusPaused {
			return DatabaseLogChainStatusMissingBinlog
		}
	}
	if stream.ArchiveType == DatabaseArchiveTypeWAL {
		if stream.Status == DatabaseLogArchiveStreamStatusFailed || stream.Status == DatabaseLogArchiveStreamStatusDisabled || stream.Status == DatabaseLogArchiveStreamStatusPaused {
			return DatabaseLogChainStatusMissingWAL
		}
	}
	if stream.Status == DatabaseLogArchiveStreamStatusDegraded {
		return DatabaseLogChainStatusTimeRangeGap
	}
	return DatabaseLogChainStatusComplete
}

func isPITRProtectionMode(mode string) bool {
	switch mode {
	case DatabaseProtectionModeMySQLPhysicalPITR, DatabaseProtectionModeMariaDBPhysicalPITR, DatabaseProtectionModePostgresBarmanPITR, DatabaseProtectionModePostgresPgBaseBackupPITR, DatabaseProtectionModeExternalPITR:
		return true
	default:
		return false
	}
}

func betterProtectionTask(candidate, current *DatabaseBackupTask) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	if candidate.Enabled != current.Enabled {
		return candidate.Enabled
	}
	if candidate.LastSuccessAt != nil && current.LastSuccessAt != nil {
		return candidate.LastSuccessAt.After(*current.LastSuccessAt)
	}
	if candidate.LastSuccessAt != nil {
		return true
	}
	return candidate.ID > current.ID
}

func latestBackupRecordTime(record *DatabaseBackupRecord, level string) string {
	if record == nil || normalizeBackupLevel(record.BackupLevel) != level {
		return ""
	}
	return firstNonEmpty(formatTime(record.FinishedAt), formatTime(record.StartedAt))
}

func restoreDrillStatusForJob(job *DatabaseRestoreJobVO, now time.Time) string {
	if job == nil || job.ID == 0 {
		return DatabaseRestoreDrillStatusNone
	}
	switch job.Status {
	case DatabaseRestoreStatusVerified, DatabaseBackupStatusSuccess, DatabaseRestoreStatusRestored:
		return restoreDrillStatusFromTime(firstNonEmpty(job.FinishedAt, job.UpdatedAt, job.CreatedAt), now)
	case DatabaseRestoreStatusFailed:
		return DatabaseRestoreDrillStatusFailed
	default:
		return DatabaseRestoreDrillStatusNone
	}
}

func restoreDrillStatusFromTime(value string, now time.Time) string {
	t, err := parseProtectionProfileTime(value)
	if err != nil || t.IsZero() {
		return DatabaseRestoreDrillStatusSuccess
	}
	if now.Sub(t) > protectionRestoreDrillFreshDays*24*time.Hour {
		return DatabaseRestoreDrillStatusStale
	}
	return DatabaseRestoreDrillStatusSuccess
}

func parseProtectionProfileTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time")
}

func protectionAction(level, action, text string, blocking bool) DatabaseProtectionActionVO {
	return DatabaseProtectionActionVO{Level: level, Action: action, Text: text, Blocking: blocking}
}

func dedupeProtectionActions(actions []DatabaseProtectionActionVO) []DatabaseProtectionActionVO {
	seen := map[string]bool{}
	result := make([]DatabaseProtectionActionVO, 0, len(actions))
	for _, action := range actions {
		key := action.Action + ":" + action.Text
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, action)
	}
	return result
}

func protectionRiskRank(value string) int {
	switch value {
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

func protectionLevelRank(value string) int {
	switch value {
	case DatabaseProtectionLevelHAAndPITRVerified:
		return 5
	case DatabaseProtectionLevelPITRVerified:
		return 4
	case DatabaseProtectionLevelPITRCapable:
		return 3
	case DatabaseProtectionLevelBackupOnly:
		return 2
	case DatabaseProtectionLevelNone:
		return 1
	default:
		return 0
	}
}

func ProtectionModeText(value string) string {
	switch value {
	case DatabaseProtectionModeLogicalBackup:
		return "逻辑备份"
	case DatabaseProtectionModeMySQLPhysicalPITR:
		return "MySQL 物理 PITR"
	case DatabaseProtectionModeMariaDBPhysicalPITR:
		return "MariaDB 物理 PITR"
	case DatabaseProtectionModePostgresBarmanPITR:
		return "PostgreSQL Barman PITR"
	case DatabaseProtectionModePostgresPgBaseBackupPITR:
		return "PostgreSQL pg_basebackup PITR"
	case DatabaseProtectionModeExternalPITR:
		return "外部 PITR"
	default:
		return "未保护"
	}
}

func ProtectionLevelText(value string) string {
	switch value {
	case DatabaseProtectionLevelBackupOnly:
		return "仅备份"
	case DatabaseProtectionLevelPITRCapable:
		return "PITR 可用"
	case DatabaseProtectionLevelPITRVerified:
		return "PITR 已演练"
	case DatabaseProtectionLevelHAAndPITRVerified:
		return "HA + PITR 已演练"
	default:
		return "未保护"
	}
}

func ProtectionHealthStatusText(value string) string {
	switch value {
	case DatabaseProtectionHealthHealthy:
		return "健康"
	case DatabaseProtectionHealthWarning:
		return "有风险"
	case DatabaseProtectionHealthCritical:
		return "严重"
	default:
		return "未知"
	}
}

func RiskLevelText(value string) string {
	switch value {
	case DatabaseQueryRiskLow:
		return "低"
	case DatabaseQueryRiskMedium:
		return "中"
	case DatabaseQueryRiskHigh:
		return "高"
	case DatabaseQueryRiskCritical:
		return "严重"
	default:
		return "未知"
	}
}

func RestoreDrillStatusText(value string) string {
	switch value {
	case DatabaseRestoreDrillStatusSuccess:
		return "成功"
	case DatabaseRestoreDrillStatusFailed:
		return "失败"
	case DatabaseRestoreDrillStatusStale:
		return "已过期"
	default:
		return "无"
	}
}
