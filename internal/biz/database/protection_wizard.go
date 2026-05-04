package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DatabaseProtectionWizardActionCreate = "create"
	DatabaseProtectionWizardActionReuse  = "reuse"
	DatabaseProtectionWizardActionUpdate = "update"
	DatabaseProtectionWizardActionStart  = "start"
	DatabaseProtectionWizardActionRun    = "run"

	DatabaseProtectionWizardStatusPlanned = "planned"
	DatabaseProtectionWizardStatusDone    = "done"
	DatabaseProtectionWizardStatusBlocked = "blocked"
)

type DatabaseMySQLPITRWizardRequest struct {
	InstanceID                        uint    `json:"instanceId" binding:"required"`
	SourceInstanceID                  uint    `json:"sourceInstanceId"`
	SourceRole                        string  `json:"sourceRole" binding:"omitempty,max=40"`
	RunnerHostID                      uint    `json:"runnerHostId" binding:"required"`
	StorageProfileID                  uint    `json:"storageProfileId"`
	SecretProfileID                   uint    `json:"secretProfileId"`
	TemplateKey                       string  `json:"templateKey" binding:"omitempty,max=80"`
	PolicyName                        string  `json:"policyName" binding:"omitempty,max=120"`
	BackupEngine                      string  `json:"backupEngine" binding:"omitempty,max=60"`
	ToolExecutionMode                 string  `json:"toolExecutionMode" binding:"omitempty,max=30"`
	ToolImage                         string  `json:"toolImage" binding:"omitempty,max=255"`
	ToolImageDigest                   string  `json:"toolImageDigest" binding:"omitempty,max=255"`
	ContainerDatadirPath              string  `json:"containerDatadirPath" binding:"omitempty,max=500"`
	ContainerWorkdirPath              string  `json:"containerWorkdirPath" binding:"omitempty,max=500"`
	ContainerNetworkMode              string  `json:"containerNetworkMode" binding:"omitempty,max=60"`
	ContainerDatadirRO                *bool   `json:"containerDatadirRo"`
	ReuseLogArchiveStreamID           uint    `json:"reuseLogArchiveStreamId"`
	ReuseBackupPolicyID               uint    `json:"reuseBackupPolicyId"`
	FullSchedule                      string  `json:"fullSchedule" binding:"omitempty,max=120"`
	IncrementalSchedule               string  `json:"incrementalSchedule" binding:"omitempty,max=120"`
	RunInitialFullNow                 *bool   `json:"runInitialFullNow"`
	BinlogArchiveMode                 string  `json:"binlogArchiveMode" binding:"omitempty,max=30"`
	BinlogRPOTargetSeconds            int     `json:"binlogRpoTargetSeconds" binding:"omitempty,min=0,max=86400"`
	BinlogRetentionDays               int     `json:"binlogRetentionDays" binding:"omitempty,min=1,max=3650"`
	SyntheticEnabled                  *bool   `json:"syntheticEnabled"`
	SyntheticAutoRun                  *bool   `json:"syntheticAutoRun"`
	SyntheticTriggerAfterIncrementals int     `json:"syntheticTriggerAfterIncrementals" binding:"omitempty,min=1,max=365"`
	SyntheticMergeOldestIncrementals  int     `json:"syntheticMergeOldestIncrementals" binding:"omitempty,min=1,max=365"`
	SyntheticRequireRestoreProof      *bool   `json:"syntheticRequireRestoreProof"`
	SyntheticNeverDeleteWithoutProof  *bool   `json:"syntheticNeverDeleteWithoutProof"`
	SyntheticMarkSupersededAfterProof *bool   `json:"syntheticMarkSupersededAfterProof"`
	SyntheticSupersededKeepDays       int     `json:"syntheticSupersededKeepDays" binding:"omitempty,min=1,max=3650"`
	RestoreDrillRequired              *bool   `json:"restoreDrillRequired"`
	RetentionFullKeepMonths           int     `json:"retentionFullKeepMonths" binding:"omitempty,min=1,max=120"`
	RetentionIncrementalKeepDays      int     `json:"retentionIncrementalKeepDays" binding:"omitempty,min=1,max=3650"`
	RetentionBinlogKeepDays           int     `json:"retentionBinlogKeepDays" binding:"omitempty,min=1,max=3650"`
	RetentionNeverDeleteWithoutProof  *bool   `json:"retentionNeverDeleteWithoutProof"`
	ArchiveConfigJSON                 string  `json:"archiveConfigJson" binding:"omitempty,max=4000"`
	ExpectedMonthlyDataChangeGB       float64 `json:"expectedMonthlyDataChangeGb"`
}

type DatabaseProtectionWizardActionVO struct {
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   uint   `json:"resourceId,omitempty"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Blocking     bool   `json:"blocking"`
}

type DatabaseMySQLPITRWizardResult struct {
	TemplateKey         string                             `json:"templateKey"`
	TemplateName        string                             `json:"templateName"`
	InstanceID          uint                               `json:"instanceId"`
	InstanceName        string                             `json:"instanceName"`
	Engine              string                             `json:"engine"`
	EngineText          string                             `json:"engineText"`
	Version             string                             `json:"version"`
	RunnerHostID        uint                               `json:"runnerHostId"`
	RunnerHostName      string                             `json:"runnerHostName"`
	BackupEngine        string                             `json:"backupEngine"`
	FullSchedule        string                             `json:"fullSchedule"`
	IncrementalSchedule string                             `json:"incrementalSchedule"`
	RunInitialFullNow   bool                               `json:"runInitialFullNow"`
	SyntheticRuleJSON   string                             `json:"syntheticRuleJson"`
	RetentionJSON       string                             `json:"retentionJson"`
	CanApply            bool                               `json:"canApply"`
	BlockingReasons     []string                           `json:"blockingReasons"`
	Warnings            []string                           `json:"warnings"`
	Messages            []string                           `json:"messages"`
	Actions             []DatabaseProtectionWizardActionVO `json:"actions"`
	LogArchiveStream    *DatabaseLogArchiveStreamVO        `json:"logArchiveStream,omitempty"`
	BackupPolicy        *DatabaseBackupPolicyVO            `json:"backupPolicy,omitempty"`
	InitialFullRun      *DatabaseBackupPolicyRunVO         `json:"initialFullRun,omitempty"`
	Profile             *DatabaseProtectionProfileVO       `json:"profile,omitempty"`
}

type DatabaseProtectionRestoreDrillRequest struct {
	RestoreTargetType      string                               `json:"restoreTargetType" binding:"omitempty,max=30"`
	RestoreTargetValue     string                               `json:"restoreTargetValue" binding:"omitempty,max=255"`
	TargetTimelineID       string                               `json:"targetTimelineId" binding:"omitempty,max=60"`
	RestoreTargetInclusive bool                                 `json:"restoreTargetInclusive"`
	RunnerHostID           uint                                 `json:"runnerHostId"`
	ContainerImage         string                               `json:"containerImage" binding:"omitempty,max=255"`
	ListenPort             int                                  `json:"listenPort" binding:"omitempty,min=0,max=65535"`
	ExpiresInHours         int                                  `json:"expiresInHours" binding:"omitempty,min=1,max=168"`
	ValidationSQL          []string                             `json:"validationSql" binding:"omitempty,max=20"`
	ValidationAssertions   []DatabaseRestoreValidationAssertion `json:"validationAssertions" binding:"omitempty,max=20"`
	CleanupOnFailure       bool                                 `json:"cleanupOnFailure"`
	PostgresStartInstance  *bool                                `json:"postgresStartInstance,omitempty"`
	TargetAction           string                               `json:"targetAction" binding:"omitempty,max=30"`
	BarmanGetWAL           *bool                                `json:"barmanGetWal,omitempty"`
}

type DatabaseProtectionRestoreDrillVO struct {
	ProfileID string                       `json:"profileId"`
	Plan      *DatabaseRestorePlanVO       `json:"plan,omitempty"`
	Job       *DatabaseRestoreJobVO        `json:"job,omitempty"`
	Profile   *DatabaseProtectionProfileVO `json:"profile,omitempty"`
	Status    string                       `json:"status"`
	Message   string                       `json:"message"`
}

type mysqlPITRWizardResolved struct {
	Req                  *DatabaseMySQLPITRWizardRequest
	Instance             *DatabaseInstance
	Runner               *DatabaseRunnerHost
	ExistingStream       *DatabaseLogArchiveStream
	ExistingPolicy       *DatabaseBackupPolicyConfig
	BackupEngine         string
	FullSchedule         string
	IncrementalSchedule  string
	RunInitialFullNow    bool
	ArchiveMode          string
	BinlogRPOTarget      int
	BinlogRetentionDays  int
	SyntheticEnabled     bool
	SyntheticRuleJSON    string
	RetentionJSON        string
	RestoreDrillRequired bool
	BlockingReasons      []string
	Warnings             []string
	Actions              []DatabaseProtectionWizardActionVO
}

func (uc *UseCase) PreviewMySQLPITRWizard(ctx context.Context, req *DatabaseMySQLPITRWizardRequest) (*DatabaseMySQLPITRWizardResult, error) {
	resolved, err := uc.resolveMySQLPITRWizard(ctx, req)
	if err != nil {
		return nil, err
	}
	return uc.mysqlPITRWizardResult(ctx, resolved, nil, nil), nil
}

func (uc *UseCase) ApplyMySQLPITRWizard(ctx context.Context, req *DatabaseMySQLPITRWizardRequest, operator QueryOperator) (*DatabaseMySQLPITRWizardResult, error) {
	resolved, err := uc.resolveMySQLPITRWizard(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resolved.BlockingReasons) > 0 {
		return nil, fmt.Errorf("保护向导预校验未通过: %s", strings.Join(resolved.BlockingReasons, "；"))
	}
	stream, err := uc.applyMySQLPITRWizardLogArchiveStream(ctx, resolved)
	if err != nil {
		return nil, err
	}
	streamVO := uc.toLogArchiveStreamVO(ctx, stream)
	policy, err := uc.applyMySQLPITRWizardBackupPolicy(ctx, resolved, stream.ID)
	if err != nil {
		return nil, err
	}
	policyVO := uc.toBackupPolicyVO(ctx, policy)
	var initialFull *DatabaseBackupPolicyRunVO
	if resolved.RunInitialFullNow {
		run, runErr := uc.RunBackupPolicyFull(ctx, policy.ID, operator)
		if runErr != nil {
			resolved.Warnings = append(resolved.Warnings, "保护资源已保存，但初始 Full 下发失败: "+runErr.Error())
		} else {
			initialFull = run
			resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "backup_policy", policy.ID, policy.Name, DatabaseProtectionWizardStatusDone, "已下发初始 Full 备份 Runner Job", false))
		}
	}
	result := uc.mysqlPITRWizardResult(ctx, resolved, streamVO, policyVO)
	result.InitialFullRun = initialFull
	result.Profile, _ = uc.GetProtectionProfile(ctx, fmt.Sprintf("%s%d", protectionProfileIDPrefix, resolved.Instance.ID))
	return result, nil
}

func (uc *UseCase) RunProtectionProfileRestoreDrill(ctx context.Context, profileID string, req *DatabaseProtectionRestoreDrillRequest, operator QueryOperator) (*DatabaseProtectionRestoreDrillVO, error) {
	profile, err := uc.GetProtectionProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &DatabaseProtectionRestoreDrillRequest{}
	}
	targetType := strings.TrimSpace(req.RestoreTargetType)
	if targetType == "" {
		targetType = "time"
	}
	targetValue := strings.TrimSpace(req.RestoreTargetValue)
	if targetValue == "" && targetType == "time" {
		targetValue = firstNonEmpty(profile.RecoverableUntil, profile.LastLogArchiveAt, profile.LastIncrementalAt, profile.LastFullAt, time.Now().Format("2006-01-02 15:04:05"))
	}
	if targetValue == "" {
		return nil, fmt.Errorf("请填写恢复目标")
	}
	plan, err := uc.CreateRestorePlan(ctx, &DatabaseRestorePlanRequest{
		SourceInstanceID:       profile.InstanceID,
		RestoreMode:            DatabaseRestoreModeIsolatedRestore,
		RestoreTargetType:      targetType,
		RestoreTargetValue:     targetValue,
		TargetTimelineID:       req.TargetTimelineID,
		RestoreTargetInclusive: req.RestoreTargetInclusive,
	}, operator)
	if err != nil {
		return nil, err
	}
	result := &DatabaseProtectionRestoreDrillVO{
		ProfileID: profile.ProfileID,
		Plan:      plan,
		Profile:   profile,
		Status:    "planned",
		Message:   firstNonEmpty(plan.Message, "恢复计划已生成"),
	}
	if plan.ValidationStatus != DatabasePlanValidationPassed {
		result.Status = "plan_failed"
		result.Message = firstNonEmpty(plan.Message, "恢复计划预校验未通过")
		return result, nil
	}
	runnerHostID := req.RunnerHostID
	if runnerHostID == 0 {
		runnerHostID = plan.RunnerHostID
	}
	if runnerHostID == 0 && profile.RunnerHost != nil {
		runnerHostID = profile.RunnerHost.ID
	}
	if runnerHostID == 0 {
		result.Status = "runner_required"
		result.Message = "恢复计划已生成，请选择 Runner 主机后执行恢复演练"
		return result, nil
	}
	job, err := uc.RunRestorePlan(ctx, plan.ID, &DatabaseRestorePlanRunRequest{
		RunnerHostID:          runnerHostID,
		ContainerImage:        req.ContainerImage,
		ListenPort:            req.ListenPort,
		ExpiresInHours:        req.ExpiresInHours,
		ValidationSQL:         req.ValidationSQL,
		ValidationAssertions:  req.ValidationAssertions,
		CleanupOnFailure:      req.CleanupOnFailure,
		TargetTimelineID:      req.TargetTimelineID,
		TargetAction:          req.TargetAction,
		BarmanGetWAL:          req.BarmanGetWAL,
		PostgresStartInstance: req.PostgresStartInstance,
	}, operator)
	if err != nil {
		result.Status = "run_failed"
		result.Message = err.Error()
		return result, nil
	}
	result.Job = job
	result.Status = "running"
	result.Message = "恢复演练任务已下发 Runner"
	result.Profile, _ = uc.GetProtectionProfile(ctx, profileID)
	return result, nil
}

func (uc *UseCase) resolveMySQLPITRWizard(ctx context.Context, req *DatabaseMySQLPITRWizardRequest) (*mysqlPITRWizardResolved, error) {
	if req == nil || req.InstanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	if uc.instanceRepo == nil || uc.backupPolicyConfigRepo == nil || uc.logArchiveStreamRepo == nil || uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("保护向导依赖仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	engine := normalizeDBType(instance.DBType)
	if engine != DBTypeMySQL && engine != DBTypeMariaDB {
		return nil, fmt.Errorf("P5.3 MySQL/MariaDB PITR 向导不支持 %s", DBTypeText(instance.DBType))
	}
	runner, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil || runner == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	resolved := &mysqlPITRWizardResolved{
		Req:                  req,
		Instance:             instance,
		Runner:               runner,
		BackupEngine:         normalizeMySQLPhysicalBackupEngine(req.BackupEngine, instance.DBType, instance.Version),
		FullSchedule:         strings.TrimSpace(req.FullSchedule),
		IncrementalSchedule:  firstNonEmpty(strings.TrimSpace(req.IncrementalSchedule), "0 3 * * *"),
		RunInitialFullNow:    boolDefault(req.RunInitialFullNow, true),
		ArchiveMode:          firstNonEmpty(strings.TrimSpace(req.BinlogArchiveMode), DatabaseArchiveModePolling),
		BinlogRPOTarget:      intDefault(req.BinlogRPOTargetSeconds, 300),
		BinlogRetentionDays:  intDefault(req.BinlogRetentionDays, 45),
		SyntheticEnabled:     boolDefault(req.SyntheticEnabled, true),
		RestoreDrillRequired: boolDefault(req.RestoreDrillRequired, true),
		BlockingReasons:      []string{},
		Warnings:             []string{},
		Actions:              []DatabaseProtectionWizardActionVO{},
	}
	if !isValidBackupSchedule(resolved.FullSchedule) {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "全量 Cron 表达式格式不正确")
	}
	if !isValidBackupSchedule(resolved.IncrementalSchedule) {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "增量 Cron 表达式格式不正确")
	}
	if !runner.Enabled || runner.Status == DatabaseRunnerHostStatusDisabled {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "Runner 主机已禁用")
	} else if runner.Status != DatabaseRunnerHostStatusOnline {
		resolved.Warnings = append(resolved.Warnings, "Runner 主机未确认在线，apply 后可能无法立即执行备份或归档")
	}
	if _, err := uc.validateLogArchiveRunnerHost(ctx, req.RunnerHostID); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	}
	if uc.credentialResolver == nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "连接凭据解析器未配置")
	} else if credential, err := uc.credentialResolver(ctx, instance.CredentialID); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "数据库凭据不存在")
	} else if _, err := validateMySQLPhysicalBackupCompatibility(ctx, instance, credential, resolved.BackupEngine); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	}
	resolved.SyntheticRuleJSON = buildMySQLPITRWizardSyntheticRuleJSON(req, resolved)
	resolved.RetentionJSON = buildMySQLPITRWizardRetentionJSON(req, resolved)
	if err := validateJSONText(resolved.SyntheticRuleJSON, "合成全量规则"); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	}
	if err := validateJSONText(resolved.RetentionJSON, "保留策略"); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	}
	resolved.ExistingStream = uc.findMySQLPITRWizardStream(ctx, req)
	resolved.ExistingPolicy = uc.findMySQLPITRWizardPolicy(ctx, req)
	if resolved.ExistingStream == nil {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionCreate, "log_archive_stream", 0, mysqlPITRWizardStreamName(instance), DatabaseProtectionWizardStatusPlanned, "创建 binlog 连续归档流", false))
	} else {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionUpdate, "log_archive_stream", resolved.ExistingStream.ID, mysqlPITRWizardStreamName(instance), DatabaseProtectionWizardStatusPlanned, "复用并更新 binlog 归档流配置", false))
	}
	resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionStart, "log_archive_stream", idFromLogArchiveStream(resolved.ExistingStream), mysqlPITRWizardStreamName(instance), DatabaseProtectionWizardStatusPlanned, "设置 desired_state=running，等待 Runner Agent 接管", false))
	if resolved.ExistingPolicy == nil {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionCreate, "backup_policy", 0, mysqlPITRWizardPolicyName(req, instance), DatabaseProtectionWizardStatusPlanned, "创建 MySQL/MariaDB 物理 PITR 备份策略", false))
	} else {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionUpdate, "backup_policy", resolved.ExistingPolicy.ID, resolved.ExistingPolicy.Name, DatabaseProtectionWizardStatusPlanned, "复用并更新物理 PITR 备份策略", false))
	}
	if resolved.RunInitialFullNow {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "backup_policy", idFromBackupPolicy(resolved.ExistingPolicy), mysqlPITRWizardPolicyName(req, instance), DatabaseProtectionWizardStatusPlanned, "立即下发一次 Full 建立基线", false))
	}
	return resolved, nil
}

func (uc *UseCase) applyMySQLPITRWizardLogArchiveStream(ctx context.Context, resolved *mysqlPITRWizardResolved) (*DatabaseLogArchiveStream, error) {
	req := resolved.Req
	stream := resolved.ExistingStream
	if stream == nil {
		vo, err := uc.CreateLogArchiveStream(ctx, &DatabaseLogArchiveStreamRequest{
			InstanceID:       req.InstanceID,
			SourceInstanceID: normalizeBackupSourceInstanceID(req.SourceInstanceID, req.InstanceID),
			Engine:           resolved.Instance.DBType,
			ArchiveType:      DatabaseArchiveTypeBinlog,
			ArchiveMode:      resolved.ArchiveMode,
			ArchiveEngine:    archiveEngineForDaemonMode("", resolved.ArchiveMode),
			RunnerHostID:     req.RunnerHostID,
			StorageProfileID: req.StorageProfileID,
			SecretProfileID:  req.SecretProfileID,
			RPOTargetSeconds: resolved.BinlogRPOTarget,
			RetentionDays:    resolved.BinlogRetentionDays,
			Enabled:          true,
			ConfigJSON:       req.ArchiveConfigJSON,
		})
		if err != nil {
			return nil, err
		}
		stream, err = uc.logArchiveStreamRepo.GetByID(ctx, vo.ID)
		if err != nil {
			return nil, err
		}
	} else {
		stream.SourceInstanceID = normalizeBackupSourceInstanceID(req.SourceInstanceID, req.InstanceID)
		stream.Engine = normalizeDBType(resolved.Instance.DBType)
		stream.ArchiveType = DatabaseArchiveTypeBinlog
		stream.ArchiveMode = daemonArchiveModeForStart(resolved.ArchiveMode)
		stream.ArchiveEngine = archiveEngineForDaemonMode(stream.ArchiveEngine, stream.ArchiveMode)
		stream.RunnerHostID = req.RunnerHostID
		stream.StorageProfileID = req.StorageProfileID
		stream.SecretProfileID = req.SecretProfileID
		stream.RPOTargetSeconds = resolved.BinlogRPOTarget
		stream.RetentionDays = normalizeArchiveRetentionDays(resolved.BinlogRetentionDays)
		stream.Enabled = true
		if stream.Status == DatabaseLogArchiveStreamStatusDisabled {
			stream.Status = DatabaseLogArchiveStreamStatusPending
		}
		if strings.TrimSpace(req.ArchiveConfigJSON) != "" {
			if err := validateBackupStorageConfigSafe(req.ArchiveConfigJSON); err != nil {
				return nil, err
			}
			stream.ConfigJSON = trimText(strings.TrimSpace(req.ArchiveConfigJSON), 4000)
		}
		if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
			return nil, err
		}
	}
	if _, err := uc.StartLogArchiveStream(ctx, stream.ID, &DatabaseLogArchiveStreamControlRequest{
		RunnerHostID: req.RunnerHostID,
		ArchiveMode:  resolved.ArchiveMode,
		Reason:       "P5 MySQL/MariaDB PITR 向导启动",
	}); err != nil {
		return nil, err
	}
	return uc.logArchiveStreamRepo.GetByID(ctx, stream.ID)
}

func (uc *UseCase) applyMySQLPITRWizardBackupPolicy(ctx context.Context, resolved *mysqlPITRWizardResolved, streamID uint) (*DatabaseBackupPolicyConfig, error) {
	req := resolved.Req
	policyReq := &DatabaseBackupPolicyRequest{
		InstanceID:           req.InstanceID,
		SourceInstanceID:     normalizeBackupSourceInstanceID(req.SourceInstanceID, req.InstanceID),
		SourceRole:           firstNonEmpty(strings.TrimSpace(req.SourceRole), "primary"),
		Name:                 mysqlPITRWizardPolicyName(req, resolved.Instance),
		BackupEngine:         resolved.BackupEngine,
		ToolExecutionMode:    req.ToolExecutionMode,
		ToolImage:            req.ToolImage,
		ToolImageDigest:      req.ToolImageDigest,
		ContainerDatadirPath: req.ContainerDatadirPath,
		ContainerWorkdirPath: req.ContainerWorkdirPath,
		ContainerNetworkMode: req.ContainerNetworkMode,
		ContainerDatadirRO:   req.ContainerDatadirRO,
		RunnerHostID:         req.RunnerHostID,
		StorageProfileID:     req.StorageProfileID,
		SecretProfileID:      req.SecretProfileID,
		BinlogStreamID:       streamID,
		FullSchedule:         resolved.FullSchedule,
		IncrementalSchedule:  resolved.IncrementalSchedule,
		SyntheticEnabled:     resolved.SyntheticEnabled,
		SyntheticRuleJSON:    resolved.SyntheticRuleJSON,
		RestoreDrillRequired: resolved.RestoreDrillRequired,
		RetentionJSON:        resolved.RetentionJSON,
		Enabled:              true,
	}
	if resolved.ExistingPolicy == nil {
		vo, err := uc.CreateBackupPolicy(ctx, policyReq)
		if err != nil {
			return nil, err
		}
		return uc.getBackupPolicy(ctx, vo.ID)
	}
	if _, err := uc.UpdateBackupPolicy(ctx, resolved.ExistingPolicy.ID, policyReq); err != nil {
		return nil, err
	}
	return uc.getBackupPolicy(ctx, resolved.ExistingPolicy.ID)
}

func (uc *UseCase) mysqlPITRWizardResult(ctx context.Context, resolved *mysqlPITRWizardResolved, stream *DatabaseLogArchiveStreamVO, policy *DatabaseBackupPolicyVO) *DatabaseMySQLPITRWizardResult {
	if stream == nil && resolved.ExistingStream != nil {
		stream = uc.toLogArchiveStreamVO(ctx, resolved.ExistingStream)
	}
	if policy == nil && resolved.ExistingPolicy != nil {
		policy = uc.toBackupPolicyVO(ctx, resolved.ExistingPolicy)
	}
	messages := make([]string, 0, 4)
	if len(resolved.BlockingReasons) > 0 {
		messages = append(messages, resolved.BlockingReasons...)
	} else {
		messages = append(messages, "预检通过，可以启用 MySQL/MariaDB 物理 PITR 保护")
	}
	messages = append(messages, resolved.Warnings...)
	return &DatabaseMySQLPITRWizardResult{
		TemplateKey:         mysqlPITRWizardTemplateKey(resolved.Req.TemplateKey),
		TemplateName:        mysqlPITRWizardTemplateName(resolved.Req.TemplateKey),
		InstanceID:          resolved.Instance.ID,
		InstanceName:        resolved.Instance.Name,
		Engine:              normalizeDBType(resolved.Instance.DBType),
		EngineText:          DBTypeText(resolved.Instance.DBType),
		Version:             resolved.Instance.Version,
		RunnerHostID:        resolved.Runner.ID,
		RunnerHostName:      resolved.Runner.Name,
		BackupEngine:        resolved.BackupEngine,
		FullSchedule:        resolved.FullSchedule,
		IncrementalSchedule: resolved.IncrementalSchedule,
		RunInitialFullNow:   resolved.RunInitialFullNow,
		SyntheticRuleJSON:   resolved.SyntheticRuleJSON,
		RetentionJSON:       resolved.RetentionJSON,
		CanApply:            len(resolved.BlockingReasons) == 0,
		BlockingReasons:     uniqueStrings(resolved.BlockingReasons),
		Warnings:            uniqueStrings(resolved.Warnings),
		Messages:            uniqueStrings(messages),
		Actions:             resolved.Actions,
		LogArchiveStream:    stream,
		BackupPolicy:        policy,
	}
}

func (uc *UseCase) findMySQLPITRWizardStream(ctx context.Context, req *DatabaseMySQLPITRWizardRequest) *DatabaseLogArchiveStream {
	if req.ReuseLogArchiveStreamID > 0 {
		if stream, err := uc.logArchiveStreamRepo.GetByID(ctx, req.ReuseLogArchiveStreamID); err == nil && stream != nil && stream.InstanceID == req.InstanceID && stream.ArchiveType == DatabaseArchiveTypeBinlog {
			return stream
		}
	}
	items, _, err := uc.logArchiveStreamRepo.List(ctx, &DatabaseLogArchiveStreamListRequest{Page: 1, PageSize: 20, InstanceID: req.InstanceID, ArchiveType: DatabaseArchiveTypeBinlog})
	if err != nil {
		return nil
	}
	for _, item := range items {
		if item != nil && item.Enabled && item.Status != DatabaseLogArchiveStreamStatusDisabled {
			return item
		}
	}
	if len(items) > 0 {
		return items[0]
	}
	return nil
}

func (uc *UseCase) findMySQLPITRWizardPolicy(ctx context.Context, req *DatabaseMySQLPITRWizardRequest) *DatabaseBackupPolicyConfig {
	if req.ReuseBackupPolicyID > 0 {
		if policy, err := uc.backupPolicyConfigRepo.GetByID(ctx, req.ReuseBackupPolicyID); err == nil && policy != nil && policy.InstanceID == req.InstanceID {
			return policy
		}
	}
	items, _, err := uc.backupPolicyConfigRepo.List(ctx, &DatabaseBackupPolicyListRequest{Page: 1, PageSize: 20, InstanceID: req.InstanceID})
	if err != nil {
		return nil
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		engine := normalizeDBType(item.Engine)
		if engine == DBTypeMySQL || engine == DBTypeMariaDB || engine == "" {
			return item
		}
	}
	return nil
}

func buildMySQLPITRWizardSyntheticRuleJSON(req *DatabaseMySQLPITRWizardRequest, resolved *mysqlPITRWizardResolved) string {
	trigger := intDefault(req.SyntheticTriggerAfterIncrementals, 5)
	merge := intDefault(req.SyntheticMergeOldestIncrementals, trigger)
	payload := map[string]any{
		"mode":                         "rolling_synthetic_full",
		"autoRun":                      boolDefault(req.SyntheticAutoRun, true),
		"triggerAfterIncrementals":     trigger,
		"mergeOldestIncrementals":      merge,
		"requireRestoreProof":          boolDefault(req.SyntheticRequireRestoreProof, true),
		"neverDeleteWithoutProof":      boolDefault(req.SyntheticNeverDeleteWithoutProof, true),
		"markSupersededAfterProof":     boolDefault(req.SyntheticMarkSupersededAfterProof, true),
		"supersededKeepDaysAfterProof": intDefault(req.SyntheticSupersededKeepDays, 7),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

func buildMySQLPITRWizardRetentionJSON(req *DatabaseMySQLPITRWizardRequest, resolved *mysqlPITRWizardResolved) string {
	payload := map[string]any{
		"fullKeepMonths":          intDefault(req.RetentionFullKeepMonths, 6),
		"incrementalKeepDays":     intDefault(req.RetentionIncrementalKeepDays, 45),
		"binlogKeepDays":          intDefault(req.RetentionBinlogKeepDays, resolved.BinlogRetentionDays),
		"neverDeleteWithoutProof": boolDefault(req.RetentionNeverDeleteWithoutProof, true),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

func mysqlPITRWizardPolicyName(req *DatabaseMySQLPITRWizardRequest, instance *DatabaseInstance) string {
	if name := strings.TrimSpace(req.PolicyName); name != "" {
		return trimText(name, 120)
	}
	return trimText(fmt.Sprintf("%s-物理PITR保护", safeBackupName(instance.Name)), 120)
}

func mysqlPITRWizardStreamName(instance *DatabaseInstance) string {
	if instance == nil {
		return "binlog归档流"
	}
	return fmt.Sprintf("%s-binlog归档流", safeBackupName(instance.Name))
}

func mysqlPITRWizardTemplateKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "rolling_synthetic_full"
	}
	return value
}

func mysqlPITRWizardTemplateName(value string) string {
	switch mysqlPITRWizardTemplateKey(value) {
	case "conservative_pitr":
		return "MySQL/MariaDB 物理 PITR - 保守策略"
	case "logical_small":
		return "小库逻辑备份"
	default:
		return "MySQL/MariaDB 物理 PITR - 滚动合成全量"
	}
}

func wizardAction(action, resourceType string, resourceID uint, name, status, message string, blocking bool) DatabaseProtectionWizardActionVO {
	return DatabaseProtectionWizardActionVO{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Name:         name,
		Status:       status,
		Message:      message,
		Blocking:     blocking,
	}
}

func idFromLogArchiveStream(item *DatabaseLogArchiveStream) uint {
	if item == nil {
		return 0
	}
	return item.ID
}

func idFromBackupPolicy(item *DatabaseBackupPolicyConfig) uint {
	if item == nil {
		return 0
	}
	return item.ID
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func intDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
