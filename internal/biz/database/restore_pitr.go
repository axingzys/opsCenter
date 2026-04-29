package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/database/pgwal"
)

const (
	defaultRestoreJobExpiresHours = 24
	maxRestoreJobExpiresHours     = 168
	defaultRestoreValidationLimit = 100
)

type DatabaseRestorePlanRunRequest struct {
	RunnerHostID          uint                                 `json:"runnerHostId" binding:"required"`
	ContainerImage        string                               `json:"containerImage" binding:"omitempty,max=255"`
	ListenPort            int                                  `json:"listenPort" binding:"omitempty,min=0,max=65535"`
	ExpiresInHours        int                                  `json:"expiresInHours" binding:"omitempty,min=1,max=168"`
	ValidationSQL         []string                             `json:"validationSql" binding:"omitempty,max=20"`
	ValidationAssertions  []DatabaseRestoreValidationAssertion `json:"validationAssertions" binding:"omitempty,max=20"`
	CleanupOnFailure      bool                                 `json:"cleanupOnFailure"`
	TargetTimelineID      string                               `json:"targetTimelineId" binding:"omitempty,max=60"`
	TargetAction          string                               `json:"targetAction" binding:"omitempty,max=30"`
	BarmanGetWAL          *bool                                `json:"barmanGetWal,omitempty"`
	PostgresStartInstance *bool                                `json:"postgresStartInstance,omitempty"`
}

type DatabaseRestoreValidationAssertion struct {
	SQL              string  `json:"sql" binding:"required,max=4000"`
	ExpectedRows     *int    `json:"expectedRows,omitempty"`
	ExpectedContains *string `json:"expectedContains,omitempty" binding:"omitempty,max=1000"`
	ExpectedScalar   *string `json:"expectedScalar,omitempty" binding:"omitempty,max=1000"`
}

type physicalRestoreArtifact struct {
	Kind           string
	ID             uint
	FileName       string
	SourcePath     string
	StorageURI     string
	ChecksumSHA256 string
	FileSize       int64
	BackupLevel    string
}

type physicalRestoreScriptInput struct {
	RestoreJobID     uint
	RestorePlanID    uint
	RunnerHostID     uint
	WorkRoot         string
	ContainerName    string
	ContainerImage   string
	ListenPort       int
	ToolName         string
	TargetTime       string
	BackupBinlogPos  int64
	Base             physicalRestoreArtifact
	Incrementals     []physicalRestoreArtifact
	Logs             []physicalRestoreArtifact
	ValidationChecks []restoreValidationCheck
	CleanupOnFailure bool
}

type restoreValidationCheck struct {
	SQL              string  `json:"sql"`
	ExpectedRows     *int    `json:"expectedRows,omitempty"`
	ExpectedContains *string `json:"expectedContains,omitempty"`
	ExpectedScalar   *string `json:"expectedScalar,omitempty"`
}

type physicalRestoreStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	OccurredAt string `json:"occurredAt"`
}

type physicalRestoreValidationResult struct {
	Index            int     `json:"index"`
	SQL              string  `json:"sql,omitempty"`
	Status           string  `json:"status"`
	SHA256           string  `json:"sha256"`
	OutputPreview    string  `json:"outputPreview"`
	ExpectedRows     *int    `json:"expectedRows,omitempty"`
	ExpectedContains *string `json:"expectedContains,omitempty"`
	ExpectedScalar   *string `json:"expectedScalar,omitempty"`
	ActualRows       *int    `json:"actualRows,omitempty"`
	ActualScalar     string  `json:"actualScalar,omitempty"`
	AssertionStatus  string  `json:"assertionStatus,omitempty"`
	AssertionMessage string  `json:"assertionMessage,omitempty"`
}

type physicalRestoreRunnerResult struct {
	RestoreJobID       uint                              `json:"restoreJobId"`
	RestorePlanID      uint                              `json:"restorePlanId"`
	RunnerHostID       uint                              `json:"runnerHostId"`
	RunnerID           string                            `json:"runnerId"`
	WorkDir            string                            `json:"workDir"`
	PreparedDatadir    string                            `json:"preparedDatadir"`
	ContainerName      string                            `json:"containerName"`
	ContainerImage     string                            `json:"containerImage"`
	ListenHost         string                            `json:"listenHost"`
	ListenPort         int                               `json:"listenPort"`
	LogPath            string                            `json:"logPath"`
	ProofPath          string                            `json:"proofPath"`
	ArtifactURI        string                            `json:"artifactUri"`
	Steps              []physicalRestoreStep             `json:"steps"`
	ValidationResults  []physicalRestoreValidationResult `json:"validationResults"`
	Stdout             string                            `json:"stdout"`
	Stderr             string                            `json:"stderr"`
	ExitCode           int                               `json:"exitCode"`
	StartedAt          string                            `json:"startedAt"`
	FinishedAt         string                            `json:"finishedAt"`
	DurationMs         int64                             `json:"durationMs"`
	ValidationStatus   string                            `json:"validationStatus"`
	ApplyBinlogSummary string                            `json:"applyBinlogSummary"`
	Error              string                            `json:"error,omitempty"`
}

type barmanRestoreScriptInput struct {
	RestoreJobID     uint
	RestorePlanID    uint
	RunnerHostID     uint
	WorkRoot         string
	ContainerName    string
	ContainerImage   string
	ListenPort       int
	DatabaseName     string
	DBUsername       string
	DBPassword       string
	BarmanServerName string
	ConfigPath       string
	BackupID         string
	TargetType       string
	TargetValue      string
	TargetTimelineID string
	TargetAction     string
	GetWAL           bool
	StartInstance    bool
	ValidationChecks []restoreValidationCheck
	CleanupOnFailure bool
}

type barmanRestoreRunnerResult struct {
	RestoreJobID      uint                              `json:"restoreJobId"`
	RestorePlanID     uint                              `json:"restorePlanId"`
	RunnerHostID      uint                              `json:"runnerHostId"`
	RunnerID          string                            `json:"runnerId"`
	BarmanServerName  string                            `json:"barmanServerName"`
	BackupID          string                            `json:"backupId"`
	WorkDir           string                            `json:"workDir"`
	PreparedDatadir   string                            `json:"preparedDatadir"`
	ContainerName     string                            `json:"containerName"`
	ContainerImage    string                            `json:"containerImage"`
	ListenHost        string                            `json:"listenHost"`
	ListenPort        int                               `json:"listenPort"`
	LogPath           string                            `json:"logPath"`
	ProofPath         string                            `json:"proofPath"`
	ArtifactURI       string                            `json:"artifactUri"`
	TargetType        string                            `json:"targetType"`
	TargetValue       string                            `json:"targetValue"`
	TargetTimelineID  string                            `json:"targetTimelineId"`
	TargetAction      string                            `json:"targetAction"`
	GetWAL            bool                              `json:"getWal"`
	StartInstance     bool                              `json:"startInstance"`
	Steps             []physicalRestoreStep             `json:"steps"`
	ValidationResults []physicalRestoreValidationResult `json:"validationResults"`
	ValidationStatus  string                            `json:"validationStatus"`
	RecoverySummary   string                            `json:"recoverySummary"`
	Stdout            string                            `json:"stdout"`
	Stderr            string                            `json:"stderr"`
	ExitCode          int                               `json:"exitCode"`
	StartedAt         string                            `json:"startedAt"`
	FinishedAt        string                            `json:"finishedAt"`
	DurationMs        int64                             `json:"durationMs"`
	Error             string                            `json:"error,omitempty"`
}

func (uc *UseCase) RunRestorePlan(ctx context.Context, planID uint, req *DatabaseRestorePlanRunRequest, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if uc.restorePlanRepo == nil || uc.restoreJobRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("PITR 恢复 Runner 仓库未配置")
	}
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("实例仓库未配置")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	if planID == 0 {
		return nil, fmt.Errorf("恢复计划ID不能为空")
	}
	if req == nil || req.RunnerHostID == 0 {
		return nil, fmt.Errorf("请选择 Runner 主机")
	}
	plan, err := uc.restorePlanRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("恢复计划不存在")
	}
	if plan.ValidationStatus != DatabasePlanValidationPassed {
		return nil, fmt.Errorf("仅允许执行预校验通过的恢复计划")
	}
	if plan.RestoreStatus == DatabaseRestoreStatusRunning || plan.RestoreStatus == DatabaseRestoreStatusQueued {
		return nil, fmt.Errorf("恢复计划已有执行中的任务")
	}
	source, err := uc.instanceRepo.GetByID(ctx, plan.SourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("来源实例不存在")
	}
	dbType := normalizeDBType(source.DBType)
	if dbType == DBTypePostgreSQL {
		return uc.runPostgreSQLBarmanRestorePlan(ctx, plan, source, req, operator)
	}
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return nil, fmt.Errorf("P2.7 首版仅支持 MySQL/MariaDB 物理备份隔离恢复")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("P2.7 首版仅支持 SSH Runner 执行隔离恢复")
	}
	base, err := uc.backupRecordRepo.GetByID(ctx, plan.SelectedBaseRecordID)
	if err != nil || base == nil {
		return nil, fmt.Errorf("恢复计划缺少 base backup")
	}
	targetTime, err := parseDatabaseTime(plan.RestoreTargetValue)
	if err != nil || targetTime == nil {
		return nil, fmt.Errorf("恢复目标时间格式不正确")
	}
	records, err := uc.backupRecordRepo.ListSuccessfulForRestore(ctx, plan.SourceInstanceID, targetTime)
	if err != nil {
		return nil, err
	}
	recheck := uc.validateRestorePlan(ctx, source, records, base, *targetTime)
	if recheck.ValidationStatus != DatabasePlanValidationPassed {
		plan.ValidationStatus = recheck.ValidationStatus
		plan.BackupChainStatus = recheck.BackupChainStatus
		plan.LogChainStatus = recheck.LogChainStatus
		plan.StorageStatus = recheck.StorageStatus
		plan.ToolStatus = recheck.ToolStatus
		plan.ErrorMessage = trimText(strings.Join(recheck.Messages, "；"), 1000)
		_ = uc.restorePlanRepo.Update(ctx, plan)
		return nil, fmt.Errorf("执行前恢复计划复检未通过: %s", plan.ErrorMessage)
	}
	validationChecks, err := normalizeRestoreValidationChecks(source.DBType, validationSQLForRestorePlan(source), req.ValidationSQL, req.ValidationAssertions)
	if err != nil {
		return nil, err
	}
	expiresHours := req.ExpiresInHours
	if expiresHours <= 0 {
		expiresHours = defaultRestoreJobExpiresHours
	}
	if expiresHours > maxRestoreJobExpiresHours {
		expiresHours = maxRestoreJobExpiresHours
	}
	expiresAt := time.Now().Add(time.Duration(expiresHours) * time.Hour)
	listenPort := normalizeRestoreListenPort(req.ListenPort)
	containerImage := defaultRestoreContainerImage(source, req.ContainerImage)
	containerName := fmt.Sprintf("opshub-restore-%d-%d", plan.ID, time.Now().Unix())
	workRoot := firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)

	job := &DatabaseRestoreJob{
		BackupRecordID:     base.ID,
		RestorePlanID:      plan.ID,
		RunnerHostID:       host.ID,
		SourceInstanceID:   plan.SourceInstanceID,
		TargetInstanceID:   plan.TargetInstanceID,
		RestoreMode:        DatabaseRestoreModeIsolatedRestore,
		RestoreStrategy:    "isolated_container",
		RestoreTargetType:  plan.RestoreTargetType,
		RestoreTargetValue: plan.RestoreTargetValue,
		Status:             DatabaseBackupStatusQueued,
		FileName:           base.FileName,
		FileSize:           base.FileSize,
		WorkDir:            filepath.Join(workRoot, "restore", fmt.Sprintf("job-%d", time.Now().UnixNano())),
		ContainerName:      containerName,
		ContainerImage:     containerImage,
		ListenHost:         "127.0.0.1",
		ListenPort:         listenPort,
		ExpiresAt:          &expiresAt,
		CleanupStatus:      "pending",
		OperatorID:         operator.ID,
		OperatorName:       trimText(operator.Username, 100),
		ErrorMessage:       "隔离恢复任务已排队，等待 Runner 执行",
	}
	if err := uc.restoreJobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建恢复任务失败: %w", err)
	}
	job.WorkDir = filepath.Join(workRoot, "restore", fmt.Sprintf("job-%d", job.ID))
	job.PreparedDatadir = filepath.Join(job.WorkDir, "prepared", "base")
	job.LogPath = filepath.Join(job.WorkDir, "restore.log")
	job.ArtifactURI = fmt.Sprintf("runner://runner-host-%d%s", host.ID, filepath.ToSlash(filepath.Join(job.WorkDir, "proof.json")))
	_ = uc.restoreJobRepo.Update(ctx, job)

	runnerJob := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypePhysicalRestore,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: plan.SourceInstanceID,
		TargetInstanceID: plan.TargetInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandPhysicalRestore,
		CommandSummary:   fmt.Sprintf("隔离恢复计划 #%d 到 %s", plan.ID, plan.RestoreTargetValue),
		WorkDir:          job.WorkDir,
		LogPath:          job.LogPath,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      physicalRestoreRequestJSON(plan, job, host, validationChecks, req.CleanupOnFailure, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, runnerJob); err != nil {
		job.Status = DatabaseBackupStatusFailed
		job.ErrorMessage = trimText("创建 Runner Job 失败: "+err.Error(), 500)
		_ = uc.restoreJobRepo.Update(ctx, job)
		return nil, err
	}
	job.RunnerJobID = runnerJob.ID
	_ = uc.restoreJobRepo.Update(ctx, job)
	plan.RestoreStatus = DatabaseRestoreStatusQueued
	plan.RunnerHostID = host.ID
	plan.ErrorMessage = "隔离恢复任务已下发 Runner"
	_ = uc.restorePlanRepo.Update(ctx, plan)

	go uc.executePhysicalRestoreJob(context.Background(), plan.ID, job.ID, runnerJob.ID, validationChecks, req.CleanupOnFailure)
	return uc.toRestoreJobVO(job, source.Name, "", "", host.Name), nil
}

func (uc *UseCase) runPostgreSQLBarmanRestorePlan(ctx context.Context, plan *DatabaseRestorePlan, source *DatabaseInstance, req *DatabaseRestorePlanRunRequest, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if uc.barmanServerRepo == nil {
		return nil, fmt.Errorf("Barman Server 仓库未配置")
	}
	base, err := uc.backupRecordRepo.GetByID(ctx, plan.SelectedBaseRecordID)
	if err != nil || base == nil {
		return nil, fmt.Errorf("恢复计划缺少 base backup")
	}
	if !isPostgreSQLBarmanBaseRecord(base) {
		return nil, fmt.Errorf("PostgreSQL P3.6 仅支持 Barman full backup 恢复")
	}
	target, err := normalizePostgreSQLRestoreTarget(plan.RestoreTargetType, plan.RestoreTargetValue, req.TargetTimelineID)
	if err != nil {
		return nil, err
	}
	server, err := uc.findBarmanServerForBackup(ctx, base)
	if err != nil {
		return nil, err
	}
	if server.RunnerHostID == 0 {
		return nil, fmt.Errorf("Barman Server 缺少 Runner 主机")
	}
	if req.RunnerHostID != server.RunnerHostID {
		return nil, fmt.Errorf("Barman restore 必须在登记的 Barman Runner 上执行：runnerHostId=%d", server.RunnerHostID)
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("P3.6 首版仅支持 SSH Runner 执行 Barman restore")
	}
	records, err := uc.backupRecordRepo.ListSuccessfulForRestore(ctx, plan.SourceInstanceID, target.Time)
	if err != nil {
		return nil, err
	}
	recheck := uc.validatePostgreSQLRestorePlan(ctx, source, records, base, target)
	if recheck.ValidationStatus != DatabasePlanValidationPassed {
		plan.ValidationStatus = recheck.ValidationStatus
		plan.BackupChainStatus = recheck.BackupChainStatus
		plan.LogChainStatus = recheck.LogChainStatus
		plan.StorageStatus = recheck.StorageStatus
		plan.ToolStatus = recheck.ToolStatus
		plan.ErrorMessage = trimText(strings.Join(recheck.Messages, "；"), 1000)
		_ = uc.restorePlanRepo.Update(ctx, plan)
		return nil, fmt.Errorf("执行前 PostgreSQL 恢复计划复检未通过: %s", plan.ErrorMessage)
	}
	startInstance := true
	if req.PostgresStartInstance != nil {
		startInstance = *req.PostgresStartInstance
	}
	validationChecks := []restoreValidationCheck{}
	if startInstance {
		validationChecks, err = normalizeRestoreValidationChecks(source.DBType, validationSQLForRestorePlan(source), req.ValidationSQL, req.ValidationAssertions)
		if err != nil {
			return nil, err
		}
	}
	expiresHours := req.ExpiresInHours
	if expiresHours <= 0 {
		expiresHours = defaultRestoreJobExpiresHours
	}
	if expiresHours > maxRestoreJobExpiresHours {
		expiresHours = maxRestoreJobExpiresHours
	}
	expiresAt := time.Now().Add(time.Duration(expiresHours) * time.Hour)
	workRoot := firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)
	job := &DatabaseRestoreJob{
		BackupRecordID:     base.ID,
		RestorePlanID:      plan.ID,
		RunnerHostID:       host.ID,
		SourceInstanceID:   plan.SourceInstanceID,
		TargetInstanceID:   plan.TargetInstanceID,
		RestoreMode:        DatabaseRestoreModeIsolatedRestore,
		RestoreStrategy:    "barman_restore_directory",
		RestoreTargetType:  plan.RestoreTargetType,
		RestoreTargetValue: plan.RestoreTargetValue,
		Status:             DatabaseBackupStatusQueued,
		FileName:           base.FileName,
		FileSize:           base.FileSize,
		WorkDir:            filepath.Join(workRoot, "restore", fmt.Sprintf("job-%d", time.Now().UnixNano())),
		PreparedDatadir:    "",
		LogPath:            "",
		ExpiresAt:          &expiresAt,
		CleanupStatus:      "pending",
		OperatorID:         operator.ID,
		OperatorName:       trimText(operator.Username, 100),
		ErrorMessage:       "Barman restore 任务已排队，等待 Runner 执行",
	}
	if err := uc.restoreJobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建恢复任务失败: %w", err)
	}
	job.WorkDir = filepath.Join(workRoot, "restore", fmt.Sprintf("job-%d", job.ID))
	job.PreparedDatadir = filepath.Join(job.WorkDir, "pgdata")
	job.LogPath = filepath.Join(job.WorkDir, "restore.log")
	if startInstance {
		job.ContainerName = fmt.Sprintf("opshub-pg-restore-%d", job.ID)
		job.ContainerImage = defaultPostgreSQLRestoreContainerImage(source, req.ContainerImage)
		job.ListenHost = "127.0.0.1"
		job.ListenPort = normalizeRestoreListenPort(req.ListenPort)
	}
	job.ArtifactURI = fmt.Sprintf("runner://runner-host-%d%s", host.ID, filepath.ToSlash(filepath.Join(job.WorkDir, "proof.json")))
	_ = uc.restoreJobRepo.Update(ctx, job)

	runnerJob := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBarmanRestore,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: plan.SourceInstanceID,
		TargetInstanceID: plan.TargetInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBarmanRestore,
		CommandSummary:   fmt.Sprintf("Barman restore %s 到隔离目录", firstNonEmpty(base.ExternalBackupID, base.FileName)),
		WorkDir:          job.WorkDir,
		LogPath:          job.LogPath,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      barmanRestoreRequestJSON(plan, job, host, server, base, req, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, runnerJob); err != nil {
		job.Status = DatabaseBackupStatusFailed
		job.ErrorMessage = trimText("创建 Runner Job 失败: "+err.Error(), 500)
		_ = uc.restoreJobRepo.Update(ctx, job)
		return nil, err
	}
	job.RunnerJobID = runnerJob.ID
	_ = uc.restoreJobRepo.Update(ctx, job)
	plan.RestoreStatus = DatabaseRestoreStatusQueued
	plan.RunnerHostID = host.ID
	plan.ErrorMessage = "Barman restore 任务已下发 Runner"
	_ = uc.restorePlanRepo.Update(ctx, plan)
	getWAL := true
	if req.BarmanGetWAL != nil {
		getWAL = *req.BarmanGetWAL
	}
	go uc.executeBarmanRestoreJob(context.Background(), plan.ID, job.ID, runnerJob.ID, validationChecks, req.CleanupOnFailure, pgwal.NormalizeTimelineID(firstNonEmpty(req.TargetTimelineID, recheck.TargetTimelineID)), normalizeBarmanRestoreTargetAction(req.TargetAction), getWAL, startInstance)
	return uc.toRestoreJobVO(job, source.Name, "", "", host.Name), nil
}

func (uc *UseCase) GetRestorePlanSourceInstanceID(ctx context.Context, planID uint) (uint, error) {
	if uc.restorePlanRepo == nil {
		return 0, fmt.Errorf("恢复计划仓库未配置")
	}
	plan, err := uc.restorePlanRepo.GetByID(ctx, planID)
	if err != nil {
		return 0, fmt.Errorf("恢复计划不存在")
	}
	return plan.SourceInstanceID, nil
}

func (uc *UseCase) GetRestoreJobSourceInstanceID(ctx context.Context, jobID uint) (uint, error) {
	if uc.restoreJobRepo == nil {
		return 0, fmt.Errorf("恢复任务仓库未配置")
	}
	job, err := uc.restoreJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return 0, fmt.Errorf("恢复任务不存在")
	}
	return job.SourceInstanceID, nil
}

func (uc *UseCase) GetRestoreJob(ctx context.Context, jobID uint) (*DatabaseRestoreJobVO, error) {
	if uc.restoreJobRepo == nil {
		return nil, fmt.Errorf("恢复任务仓库未配置")
	}
	job, err := uc.restoreJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("恢复任务不存在")
	}
	sourceNames, targetNames, targetEnvs := uc.loadRestoreJobInstanceMeta(ctx, []*DatabaseRestoreJob{job})
	runnerNames := uc.loadRestoreJobRunnerMeta(ctx, []*DatabaseRestoreJob{job})
	return uc.toRestoreJobVO(job, sourceNames[job.SourceInstanceID], targetNames[job.TargetInstanceID], targetEnvs[job.TargetInstanceID], runnerNames[job.RunnerHostID]), nil
}

func (uc *UseCase) GetRestoreJobProof(ctx context.Context, jobID uint) (string, error) {
	if uc.restoreJobRepo == nil {
		return "", fmt.Errorf("恢复任务仓库未配置")
	}
	job, err := uc.restoreJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return "", fmt.Errorf("恢复任务不存在")
	}
	if strings.TrimSpace(job.ProofJSON) == "" {
		return "", fmt.Errorf("恢复证明尚未生成")
	}
	return job.ProofJSON, nil
}

func (uc *UseCase) CancelRestoreJob(ctx context.Context, jobID uint, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if uc.restoreJobRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("恢复任务仓库未配置")
	}
	job, err := uc.restoreJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("恢复任务不存在")
	}
	if job.Status == DatabaseBackupStatusRunning {
		return nil, fmt.Errorf("运行中的 SSH Runner 恢复任务暂不支持远程强制取消，请先在 Runner 主机停止恢复进程后再清理")
	}
	if job.Status == DatabaseBackupStatusSuccess || job.Status == DatabaseBackupStatusFailed {
		return nil, fmt.Errorf("恢复任务已结束，不能取消")
	}
	now := time.Now()
	job.Status = DatabaseRestoreStatusCancelled
	job.FinishedAt = &now
	job.ErrorMessage = trimText("恢复任务已取消: "+operator.Username, 500)
	if job.RunnerJobID > 0 {
		if runnerJob, err := uc.runnerJobRepo.GetByID(ctx, job.RunnerJobID); err == nil && runnerJob != nil {
			runnerJob.Status = DatabaseRunnerJobStatusCancelled
			runnerJob.FinishedAt = &now
			runnerJob.ErrorMessage = "恢复任务已取消"
			_ = uc.runnerJobRepo.Update(ctx, runnerJob)
		}
	}
	if job.RestorePlanID > 0 && uc.restorePlanRepo != nil {
		if plan, err := uc.restorePlanRepo.GetByID(ctx, job.RestorePlanID); err == nil && plan != nil {
			plan.RestoreStatus = DatabaseRestoreStatusCancelled
			plan.FinishedAt = &now
			plan.ErrorMessage = "恢复任务已取消"
			_ = uc.restorePlanRepo.Update(ctx, plan)
		}
	}
	if err := uc.restoreJobRepo.Update(ctx, job); err != nil {
		return nil, err
	}
	return uc.GetRestoreJob(ctx, job.ID)
}

func (uc *UseCase) CleanupRestoreJob(ctx context.Context, jobID uint, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if uc.restoreJobRepo == nil || uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("恢复任务仓库未配置")
	}
	job, err := uc.restoreJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("恢复任务不存在")
	}
	if job.Status == DatabaseBackupStatusRunning {
		return nil, fmt.Errorf("恢复任务仍在运行，不能清理")
	}
	if job.RunnerHostID == 0 {
		return nil, fmt.Errorf("恢复任务缺少 Runner")
	}
	if !isSafeRestoreWorkDir(job.WorkDir) {
		return nil, fmt.Errorf("恢复工作目录不安全，拒绝清理")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, job.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("P2.7 首版仅支持 SSH Runner 清理")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script, err := buildRestoreCleanupScript(job)
	if err != nil {
		return nil, err
	}
	_, stderr, _, runErr := executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	now := time.Now()
	if runErr != nil {
		job.CleanupStatus = "failed"
		job.ErrorMessage = trimText("恢复环境清理失败: "+firstNonEmpty(stderr, runErr.Error()), 500)
		_ = uc.restoreJobRepo.Update(ctx, job)
		return nil, fmt.Errorf("%s", job.ErrorMessage)
	}
	job.CleanupStatus = "cleaned"
	job.ErrorMessage = trimText("恢复环境已清理: "+operator.Username, 500)
	job.FinishedAt = firstNonNilTime(job.FinishedAt, &now)
	if err := uc.restoreJobRepo.Update(ctx, job); err != nil {
		return nil, err
	}
	return uc.GetRestoreJob(ctx, job.ID)
}

func (uc *UseCase) executePhysicalRestoreJob(ctx context.Context, planID, restoreJobID, runnerJobID uint, validationChecks []restoreValidationCheck, cleanupOnFailure bool) {
	plan, planErr := uc.restorePlanRepo.GetByID(ctx, planID)
	restoreJob, restoreErr := uc.restoreJobRepo.GetByID(ctx, restoreJobID)
	runnerJob, runnerErr := uc.runnerJobRepo.GetByID(ctx, runnerJobID)
	if planErr != nil || restoreErr != nil || runnerErr != nil || plan == nil || restoreJob == nil || runnerJob == nil {
		return
	}
	started := time.Now()
	restoreJob.Status = DatabaseBackupStatusRunning
	restoreJob.StartedAt = &started
	restoreJob.ErrorMessage = "隔离恢复 Runner 执行中"
	runnerJob.Status = DatabaseRunnerJobStatusRunning
	runnerJob.StartedAt = &started
	runnerJob.HeartbeatAt = &started
	plan.RestoreStatus = DatabaseRestoreStatusRunning
	plan.StartedAt = &started
	_ = uc.restoreJobRepo.Update(ctx, restoreJob)
	_ = uc.runnerJobRepo.Update(ctx, runnerJob)
	_ = uc.restorePlanRepo.Update(ctx, plan)

	result, exitCode, runErr := uc.runPhysicalRestoreRunnerScript(ctx, plan, restoreJob, validationChecks, cleanupOnFailure, started)
	finished := time.Now()
	status := DatabaseBackupStatusSuccess
	restoreStatus := DatabaseRestoreStatusVerified
	message := "隔离恢复完成并通过校验"
	if runErr != nil {
		status = DatabaseBackupStatusFailed
		restoreStatus = DatabaseRestoreStatusFailed
		message = runErr.Error()
	} else if result.ValidationStatus == DatabasePlanValidationFailed {
		restoreStatus = DatabaseRestoreStatusRestored
		message = "隔离库已恢复，但校验 SQL 存在失败"
	}
	result.ExitCode = exitCode
	result.FinishedAt = finished.Format("2006-01-02 15:04:05")
	result.DurationMs = finished.Sub(started).Milliseconds()
	if runErr != nil {
		result.Error = runErr.Error()
	}
	resultJSON, _ := json.Marshal(result)
	proofJSON := buildPhysicalRestoreProofJSON(plan, restoreJob, result, status, message)

	restoreJob.Status = status
	restoreJob.FinishedAt = &finished
	restoreJob.DurationMs = finished.Sub(started).Milliseconds()
	restoreJob.ErrorMessage = trimText(message, 500)
	restoreJob.WorkDir = result.WorkDir
	restoreJob.PreparedDatadir = result.PreparedDatadir
	restoreJob.ContainerName = result.ContainerName
	restoreJob.ContainerImage = result.ContainerImage
	restoreJob.ListenHost = result.ListenHost
	restoreJob.ListenPort = result.ListenPort
	restoreJob.LogPath = result.LogPath
	restoreJob.ArtifactURI = firstNonEmpty(result.ArtifactURI, restoreJob.ArtifactURI)
	restoreJob.StepJSON = marshalBackupPlanJSON(result.Steps)
	restoreJob.ValidationJSON = marshalBackupPlanJSON(result.ValidationResults)
	restoreJob.ProofJSON = proofJSON
	runnerJob.Status = runnerStatusForRestoreStatus(status)
	runnerJob.ExitCode = exitCode
	runnerJob.FinishedAt = &finished
	runnerJob.DurationMs = finished.Sub(started).Milliseconds()
	runnerJob.HeartbeatAt = &finished
	runnerJob.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	runnerJob.ErrorMessage = ""
	if runErr != nil {
		runnerJob.ErrorMessage = trimText(runErr.Error(), 1000)
	}
	plan.RestoreStatus = restoreStatus
	plan.FinishedAt = &finished
	plan.DurationMs = finished.Sub(started).Milliseconds()
	plan.ProofJSON = proofJSON
	plan.ErrorMessage = trimText(message, 1000)
	_ = uc.restoreJobRepo.Update(ctx, restoreJob)
	_ = uc.runnerJobRepo.Update(ctx, runnerJob)
	_ = uc.restorePlanRepo.Update(ctx, plan)
	if status == DatabaseBackupStatusSuccess && restoreStatus == DatabaseRestoreStatusVerified && uc.backupRecordRepo != nil {
		if base, err := uc.backupRecordRepo.GetByID(ctx, restoreJob.BackupRecordID); err == nil && base != nil {
			now := finished
			base.RestoreTestedAt = &now
			base.RestoreTestStatus = DatabaseBackupStatusSuccess
			_ = uc.backupRecordRepo.Update(ctx, base)
		}
	}
}

func (uc *UseCase) executeBarmanRestoreJob(ctx context.Context, planID, restoreJobID, runnerJobID uint, validationChecks []restoreValidationCheck, cleanupOnFailure bool, targetTimelineID, targetAction string, getWAL, startInstance bool) {
	plan, planErr := uc.restorePlanRepo.GetByID(ctx, planID)
	restoreJob, restoreErr := uc.restoreJobRepo.GetByID(ctx, restoreJobID)
	runnerJob, runnerErr := uc.runnerJobRepo.GetByID(ctx, runnerJobID)
	if planErr != nil || restoreErr != nil || runnerErr != nil || plan == nil || restoreJob == nil || runnerJob == nil {
		return
	}
	started := time.Now()
	restoreJob.Status = DatabaseBackupStatusRunning
	restoreJob.StartedAt = &started
	restoreJob.ErrorMessage = "Barman restore Runner 执行中"
	runnerJob.Status = DatabaseRunnerJobStatusRunning
	runnerJob.StartedAt = &started
	runnerJob.HeartbeatAt = &started
	plan.RestoreStatus = DatabaseRestoreStatusRunning
	plan.StartedAt = &started
	_ = uc.restoreJobRepo.Update(ctx, restoreJob)
	_ = uc.runnerJobRepo.Update(ctx, runnerJob)
	_ = uc.restorePlanRepo.Update(ctx, plan)

	result, exitCode, runErr := uc.runBarmanRestoreRunnerScript(ctx, plan, restoreJob, validationChecks, cleanupOnFailure, targetTimelineID, targetAction, getWAL, startInstance, started)
	finished := time.Now()
	status := DatabaseBackupStatusSuccess
	restoreStatus := DatabaseRestoreStatusVerified
	message := "Barman restore 已恢复到隔离 PostgreSQL 实例并通过校验"
	if runErr != nil {
		status = DatabaseBackupStatusFailed
		restoreStatus = DatabaseRestoreStatusFailed
		message = runErr.Error()
	} else if !startInstance {
		restoreStatus = DatabaseRestoreStatusRestored
		message = "Barman restore 已恢复到隔离目录"
	} else if result.ValidationStatus == DatabasePlanValidationFailed {
		restoreStatus = DatabaseRestoreStatusRestored
		message = "隔离 PostgreSQL 已启动，但校验 SQL 存在失败"
	}
	result.ExitCode = exitCode
	result.FinishedAt = finished.Format("2006-01-02 15:04:05")
	result.DurationMs = finished.Sub(started).Milliseconds()
	if runErr != nil {
		result.Error = runErr.Error()
	}
	resultJSON, _ := json.Marshal(result)
	proofJSON := buildBarmanRestoreProofJSON(plan, restoreJob, result, status, message)

	restoreJob.Status = status
	restoreJob.FinishedAt = &finished
	restoreJob.DurationMs = finished.Sub(started).Milliseconds()
	restoreJob.ErrorMessage = trimText(message, 500)
	restoreJob.WorkDir = firstNonEmpty(result.WorkDir, restoreJob.WorkDir)
	restoreJob.PreparedDatadir = firstNonEmpty(result.PreparedDatadir, restoreJob.PreparedDatadir)
	restoreJob.ContainerName = firstNonEmpty(result.ContainerName, restoreJob.ContainerName)
	restoreJob.ContainerImage = firstNonEmpty(result.ContainerImage, restoreJob.ContainerImage)
	restoreJob.ListenHost = firstNonEmpty(result.ListenHost, restoreJob.ListenHost)
	restoreJob.ListenPort = firstNonZeroInt(result.ListenPort, restoreJob.ListenPort)
	restoreJob.LogPath = firstNonEmpty(result.LogPath, restoreJob.LogPath)
	restoreJob.ArtifactURI = firstNonEmpty(result.ArtifactURI, restoreJob.ArtifactURI)
	restoreJob.StepJSON = marshalBackupPlanJSON(result.Steps)
	restoreJob.ValidationJSON = marshalBackupPlanJSON(result.ValidationResults)
	restoreJob.ProofJSON = proofJSON
	runnerJob.Status = runnerStatusForRestoreStatus(status)
	runnerJob.ExitCode = exitCode
	runnerJob.FinishedAt = &finished
	runnerJob.DurationMs = finished.Sub(started).Milliseconds()
	runnerJob.HeartbeatAt = &finished
	runnerJob.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	runnerJob.ErrorMessage = ""
	if runErr != nil {
		runnerJob.ErrorMessage = trimText(runErr.Error(), 1000)
	}
	plan.RestoreStatus = restoreStatus
	plan.FinishedAt = &finished
	plan.DurationMs = finished.Sub(started).Milliseconds()
	plan.ProofJSON = proofJSON
	plan.ErrorMessage = trimText(message, 1000)
	_ = uc.restoreJobRepo.Update(ctx, restoreJob)
	_ = uc.runnerJobRepo.Update(ctx, runnerJob)
	_ = uc.restorePlanRepo.Update(ctx, plan)
	if status == DatabaseBackupStatusSuccess && restoreStatus == DatabaseRestoreStatusVerified && uc.backupRecordRepo != nil {
		if base, err := uc.backupRecordRepo.GetByID(ctx, restoreJob.BackupRecordID); err == nil && base != nil {
			now := finished
			base.RestoreTestedAt = &now
			base.RestoreTestStatus = DatabaseBackupStatusSuccess
			_ = uc.backupRecordRepo.Update(ctx, base)
		}
	}
}

func (uc *UseCase) runPhysicalRestoreRunnerScript(ctx context.Context, plan *DatabaseRestorePlan, restoreJob *DatabaseRestoreJob, validationChecks []restoreValidationCheck, cleanupOnFailure bool, started time.Time) (physicalRestoreRunnerResult, int, error) {
	result := physicalRestoreRunnerResult{
		RestorePlanID:    plan.ID,
		RestoreJobID:     restoreJob.ID,
		RunnerHostID:     restoreJob.RunnerHostID,
		ContainerName:    restoreJob.ContainerName,
		ContainerImage:   restoreJob.ContainerImage,
		ListenHost:       "127.0.0.1",
		ListenPort:       restoreJob.ListenPort,
		WorkDir:          restoreJob.WorkDir,
		PreparedDatadir:  restoreJob.PreparedDatadir,
		LogPath:          restoreJob.LogPath,
		ArtifactURI:      restoreJob.ArtifactURI,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
		ValidationStatus: DatabasePlanValidationPending,
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, restoreJob.RunnerHostID)
	if err != nil {
		return result, 1, fmt.Errorf("Runner 主机不存在")
	}
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return result, 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	base, err := uc.backupRecordRepo.GetByID(ctx, restoreJob.BackupRecordID)
	if err != nil || base == nil {
		return result, 1, fmt.Errorf("base backup 不存在")
	}
	incrementals, err := uc.restoreBackupArtifacts(ctx, plan.SelectedBackupRecordIDs, restoreJob.RunnerHostID, base.ID)
	if err != nil {
		return result, 1, err
	}
	logs, err := uc.restoreLogArtifacts(ctx, plan.SelectedLogArchiveIDs, restoreJob.RunnerHostID)
	if err != nil {
		return result, 1, err
	}
	baseArtifact, err := restoreArtifactFromBackupRecord(base, restoreJob.RunnerHostID, "base")
	if err != nil {
		return result, 1, err
	}
	toolName := firstNonEmpty(base.ToolName, mysqlPhysicalBackupToolName(base.BackupEngine, ""))
	if toolName == "" {
		return result, 1, fmt.Errorf("base backup 缺少物理恢复工具")
	}
	input := physicalRestoreScriptInput{
		RestoreJobID:     restoreJob.ID,
		RestorePlanID:    plan.ID,
		RunnerHostID:     restoreJob.RunnerHostID,
		WorkRoot:         filepath.Dir(filepath.Dir(restoreJob.WorkDir)),
		ContainerName:    restoreJob.ContainerName,
		ContainerImage:   restoreJob.ContainerImage,
		ListenPort:       restoreJob.ListenPort,
		ToolName:         toolName,
		TargetTime:       plan.RestoreTargetValue,
		BackupBinlogPos:  base.BackupBinlogPos,
		Base:             baseArtifact,
		Incrementals:     incrementals,
		Logs:             logs,
		ValidationChecks: validationChecks,
		CleanupOnFailure: cleanupOnFailure,
	}
	script, err := buildPhysicalRestoreScript(input)
	if err != nil {
		return result, 1, err
	}
	stdout, stderr, exitCode, runErr := executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	parsed := parsePhysicalRestoreOutput(stdout)
	parsed.ValidationResults = attachRestoreValidationChecks(parsed.ValidationResults, validationChecks)
	parsed.RestorePlanID = plan.ID
	parsed.RestoreJobID = restoreJob.ID
	parsed.RunnerHostID = host.ID
	parsed.RunnerID = runnerIDForHost(host)
	parsed.Stdout = trimText(stdout, maxRunnerOutputLength)
	parsed.Stderr = trimText(stderr, maxRunnerOutputLength)
	parsed.ExitCode = exitCode
	parsed.StartedAt = started.Format("2006-01-02 15:04:05")
	parsed.WorkDir = firstNonEmpty(parsed.WorkDir, result.WorkDir)
	parsed.PreparedDatadir = firstNonEmpty(parsed.PreparedDatadir, result.PreparedDatadir)
	parsed.ContainerName = firstNonEmpty(parsed.ContainerName, result.ContainerName)
	parsed.ContainerImage = firstNonEmpty(parsed.ContainerImage, result.ContainerImage)
	parsed.ListenHost = firstNonEmpty(parsed.ListenHost, result.ListenHost)
	parsed.ListenPort = firstNonZeroInt(parsed.ListenPort, result.ListenPort)
	parsed.LogPath = firstNonEmpty(parsed.LogPath, result.LogPath)
	parsed.ArtifactURI = firstNonEmpty(parsed.ArtifactURI, result.ArtifactURI)
	if parsed.ValidationStatus == "" || parsed.ValidationStatus == DatabasePlanValidationPending {
		parsed.ValidationStatus = validationStatusFromResults(parsed.ValidationResults)
	}
	if runErr != nil {
		return parsed, exitCode, fmt.Errorf("隔离恢复 Runner 执行失败: %s", trimText(firstNonEmpty(stderr, runErr.Error()), 1000))
	}
	return parsed, exitCode, nil
}

func (uc *UseCase) runBarmanRestoreRunnerScript(ctx context.Context, plan *DatabaseRestorePlan, restoreJob *DatabaseRestoreJob, validationChecks []restoreValidationCheck, cleanupOnFailure bool, targetTimelineID, targetAction string, getWAL, startInstance bool, started time.Time) (barmanRestoreRunnerResult, int, error) {
	result := barmanRestoreRunnerResult{
		RestorePlanID:   plan.ID,
		RestoreJobID:    restoreJob.ID,
		RunnerHostID:    restoreJob.RunnerHostID,
		WorkDir:         restoreJob.WorkDir,
		PreparedDatadir: restoreJob.PreparedDatadir,
		ContainerName:   restoreJob.ContainerName,
		ContainerImage:  restoreJob.ContainerImage,
		ListenHost:      "127.0.0.1",
		ListenPort:      restoreJob.ListenPort,
		LogPath:         restoreJob.LogPath,
		ArtifactURI:     restoreJob.ArtifactURI,
		TargetType:      plan.RestoreTargetType,
		TargetValue:     plan.RestoreTargetValue,
		StartInstance:   startInstance,
		StartedAt:       started.Format("2006-01-02 15:04:05"),
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, restoreJob.RunnerHostID)
	if err != nil {
		return result, 1, fmt.Errorf("Runner 主机不存在")
	}
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return result, 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	base, err := uc.backupRecordRepo.GetByID(ctx, restoreJob.BackupRecordID)
	if err != nil || base == nil {
		return result, 1, fmt.Errorf("base backup 不存在")
	}
	server, err := uc.findBarmanServerForBackup(ctx, base)
	if err != nil {
		return result, 1, err
	}
	databaseName := "postgres"
	dbUsername := "postgres"
	dbPassword := ""
	if startInstance {
		source, err := uc.instanceRepo.GetByID(ctx, plan.SourceInstanceID)
		if err != nil || source == nil {
			return result, 1, fmt.Errorf("来源 PostgreSQL 实例不存在")
		}
		credential, err := uc.credentialResolver(ctx, source.CredentialID)
		if err != nil {
			return result, 1, fmt.Errorf("解析 PostgreSQL 校验凭据失败: %w", err)
		}
		databaseName = firstNonEmpty(strings.TrimSpace(source.DefaultDatabase), "postgres")
		dbUsername = firstNonEmpty(strings.TrimSpace(credential.Username), "postgres")
		dbPassword = credential.Password
	}
	targetTimelineID = pgwal.NormalizeTimelineID(firstNonEmpty(targetTimelineID, extractTargetTimelineFromPlan(plan)))
	targetAction = normalizeBarmanRestoreTargetAction(targetAction)
	input := barmanRestoreScriptInput{
		RestoreJobID:     restoreJob.ID,
		RestorePlanID:    plan.ID,
		RunnerHostID:     restoreJob.RunnerHostID,
		WorkRoot:         filepath.Dir(filepath.Dir(restoreJob.WorkDir)),
		ContainerName:    restoreJob.ContainerName,
		ContainerImage:   restoreJob.ContainerImage,
		ListenPort:       restoreJob.ListenPort,
		DatabaseName:     databaseName,
		DBUsername:       dbUsername,
		DBPassword:       dbPassword,
		BarmanServerName: server.BarmanServerName,
		ConfigPath:       server.ConfigPath,
		BackupID:         base.ExternalBackupID,
		TargetType:       plan.RestoreTargetType,
		TargetValue:      plan.RestoreTargetValue,
		TargetTimelineID: targetTimelineID,
		TargetAction:     targetAction,
		GetWAL:           getWAL,
		StartInstance:    startInstance,
		ValidationChecks: validationChecks,
		CleanupOnFailure: cleanupOnFailure,
	}
	script, err := buildBarmanRestoreScript(input)
	if err != nil {
		return result, 1, err
	}
	stdout, stderr, exitCode, runErr := executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	parsed := parseBarmanRestoreOutput(stdout)
	parsed.RestorePlanID = plan.ID
	parsed.RestoreJobID = restoreJob.ID
	parsed.RunnerHostID = host.ID
	parsed.RunnerID = runnerIDForHost(host)
	parsed.BarmanServerName = server.BarmanServerName
	parsed.BackupID = base.ExternalBackupID
	parsed.Stdout = trimText(stdout, maxRunnerOutputLength)
	parsed.Stderr = trimText(stderr, maxRunnerOutputLength)
	parsed.ExitCode = exitCode
	parsed.StartedAt = started.Format("2006-01-02 15:04:05")
	parsed.WorkDir = firstNonEmpty(parsed.WorkDir, result.WorkDir)
	parsed.PreparedDatadir = firstNonEmpty(parsed.PreparedDatadir, result.PreparedDatadir)
	parsed.ContainerName = firstNonEmpty(parsed.ContainerName, result.ContainerName)
	parsed.ContainerImage = firstNonEmpty(parsed.ContainerImage, result.ContainerImage)
	parsed.ListenHost = firstNonEmpty(parsed.ListenHost, result.ListenHost)
	parsed.ListenPort = firstNonZeroInt(parsed.ListenPort, result.ListenPort)
	parsed.LogPath = firstNonEmpty(parsed.LogPath, result.LogPath)
	parsed.ArtifactURI = firstNonEmpty(parsed.ArtifactURI, result.ArtifactURI)
	parsed.TargetType = firstNonEmpty(parsed.TargetType, result.TargetType)
	parsed.TargetValue = firstNonEmpty(parsed.TargetValue, result.TargetValue)
	parsed.TargetTimelineID = firstNonEmpty(parsed.TargetTimelineID, targetTimelineID)
	parsed.TargetAction = firstNonEmpty(parsed.TargetAction, targetAction)
	parsed.GetWAL = getWAL
	parsed.StartInstance = startInstance
	parsed.ValidationResults = attachRestoreValidationChecks(parsed.ValidationResults, validationChecks)
	if startInstance && (parsed.ValidationStatus == "" || parsed.ValidationStatus == DatabasePlanValidationPending) {
		parsed.ValidationStatus = validationStatusFromResults(parsed.ValidationResults)
	}
	if runErr != nil {
		return parsed, exitCode, fmt.Errorf("Barman restore Runner 执行失败: %s", trimText(firstNonEmpty(stderr, runErr.Error()), 1000))
	}
	return parsed, exitCode, nil
}

func buildBarmanRestoreScript(input barmanRestoreScriptInput) (string, error) {
	if input.RestoreJobID == 0 || input.RunnerHostID == 0 {
		return "", fmt.Errorf("恢复任务上下文不完整")
	}
	if !barmanServerNamePattern.MatchString(strings.TrimSpace(input.BarmanServerName)) {
		return "", fmt.Errorf("Barman server name 不合法")
	}
	if !barmanBackupIDPattern.MatchString(strings.TrimSpace(input.BackupID)) {
		return "", fmt.Errorf("Barman backup ID 不合法")
	}
	targetType := strings.ToLower(strings.TrimSpace(input.TargetType))
	if targetType != "time" && targetType != "lsn" {
		return "", fmt.Errorf("Barman restore 仅支持 targetType=time/lsn")
	}
	if strings.TrimSpace(input.TargetValue) == "" {
		return "", fmt.Errorf("恢复目标不能为空")
	}
	targetAction := normalizeBarmanRestoreTargetAction(input.TargetAction)
	barmanTargetTimelineID := ""
	if input.TargetTimelineID != "" {
		input.TargetTimelineID = pgwal.NormalizeTimelineID(input.TargetTimelineID)
		timelineNo, err := pgwal.TimelineNumber(input.TargetTimelineID)
		if err != nil {
			return "", fmt.Errorf("target timeline 不合法")
		}
		barmanTargetTimelineID = strconv.FormatUint(timelineNo, 10)
	}
	if input.StartInstance {
		if strings.TrimSpace(input.ContainerName) == "" || !isSafeRestoreContainerName(input.ContainerName) {
			return "", fmt.Errorf("隔离 PostgreSQL 容器名称不合法")
		}
		if strings.TrimSpace(input.ContainerImage) == "" || !isSafeRestoreContainerImage(input.ContainerImage) {
			return "", fmt.Errorf("隔离 PostgreSQL 容器镜像不合法")
		}
		if input.ListenPort <= 0 || input.ListenPort > 65535 {
			return "", fmt.Errorf("隔离 PostgreSQL 端口不合法")
		}
		if strings.TrimSpace(input.DatabaseName) == "" || strings.ContainsAny(input.DatabaseName, "\x00\r\n") {
			return "", fmt.Errorf("PostgreSQL 校验数据库名不合法")
		}
		if strings.TrimSpace(input.DBUsername) == "" || strings.ContainsAny(input.DBUsername, "\x00\r\n") {
			return "", fmt.Errorf("PostgreSQL 校验用户名不合法")
		}
	}
	lines := []string{
		"set -eu",
		"WORK_ROOT=" + shellSingleQuote(input.WorkRoot),
		fmt.Sprintf("RESTORE_JOB_ID=%d", input.RestoreJobID),
		fmt.Sprintf("RESTORE_PLAN_ID=%d", input.RestorePlanID),
		"WORK_DIR=\"$WORK_ROOT/restore/job-$RESTORE_JOB_ID\"",
		"DEST_DIR=\"$WORK_DIR/pgdata\"",
		"LOG_FILE=\"$WORK_DIR/restore.log\"",
		"PROOF_FILE=\"$WORK_DIR/proof.json\"",
		"CONTAINER_NAME=" + shellSingleQuote(input.ContainerName),
		"CONTAINER_IMAGE=" + shellSingleQuote(input.ContainerImage),
		fmt.Sprintf("LISTEN_PORT=%d", input.ListenPort),
		"PGDATABASE_NAME=" + shellSingleQuote(input.DatabaseName),
		"PGUSER_NAME=" + shellSingleQuote(input.DBUsername),
		"PGPASSWORD_VALUE=" + shellSingleQuote(input.DBPassword),
		"SERVER=" + shellSingleQuote(input.BarmanServerName),
		"BACKUP_ID=" + shellSingleQuote(input.BackupID),
		"CONFIG_PATH=" + shellSingleQuote(input.ConfigPath),
		"TARGET_TYPE=" + shellSingleQuote(targetType),
		"TARGET_VALUE=" + shellSingleQuote(input.TargetValue),
		"TARGET_TLI=" + shellSingleQuote(input.TargetTimelineID),
		"BARMAN_TARGET_TLI=" + shellSingleQuote(barmanTargetTimelineID),
		"TARGET_ACTION=" + shellSingleQuote(targetAction),
		"GET_WAL=" + boolShellValue(input.GetWAL),
		"START_INSTANCE=" + boolShellValue(input.StartInstance),
		"CLEANUP_ON_FAILURE=" + boolShellValue(input.CleanupOnFailure),
		`mkdir -p "$WORK_DIR"`,
		`: > "$LOG_FILE"`,
		`step() { printf 'OPSHUB_RESTORE_STEP=%s|%s|%s\n' "$1" "$2" "$(date '+%Y-%m-%d %H:%M:%S')"; printf '%s %s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG_FILE"; }`,
		`cleanup_failure() { code="$?"; if [ "$code" != "0" ] && [ "$CLEANUP_ON_FAILURE" = "1" ]; then DOCKER_BIN="$(command -v docker || true)"; if [ -n "$DOCKER_BIN" ] && [ -n "$CONTAINER_NAME" ]; then "$DOCKER_BIN" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fi; rm -rf "$DEST_DIR" >> "$LOG_FILE" 2>&1 || true; printf '%s cleanup_on_failure restored_dir_removed\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_FILE" || true; fi; exit "$code"; }`,
		`trap cleanup_failure EXIT`,
		`fail_step() { step "$1" "failed"; echo "$2" >> "$LOG_FILE"; exit 1; }`,
		`run_barman() { if [ -n "$CONFIG_PATH" ]; then barman -c "$CONFIG_PATH" "$@"; else barman "$@"; fi; }`,
		`run_pg_validation() { idx="$1"; sql="$2"; expected_rows_enabled="$3"; expected_rows="$4"; expected_contains_enabled="$5"; expected_contains="$6"; expected_scalar_enabled="$7"; expected_scalar="$8"; out="$WORK_DIR/validation-$idx.out"; step "validation_${idx}" "running"; set +e; PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -F "$(printf '\t')" -v ON_ERROR_STOP=1 -c "$sql" > "$out" 2>&1; code="$?"; set -e; sha="$(sha256sum "$out" | awk '{print $1}')"; preview="$(head -c 1200 "$out" | base64 | tr -d '\n')"; actual_rows="$(grep -v '^$' "$out" | wc -l | tr -d ' ')"; actual_scalar="$(grep -v '^$' "$out" | head -n 1 | awk -F '\t' '{print $1}')"; status="success"; assertion_status="not_configured"; assertion_message=""; if [ "$code" != "0" ]; then status="failed"; assertion_status="failed"; assertion_message="SQL execution failed"; else if [ "$expected_rows_enabled" = "1" ] || [ "$expected_contains_enabled" = "1" ] || [ "$expected_scalar_enabled" = "1" ]; then assertion_status="passed"; fi; if [ "$expected_rows_enabled" = "1" ] && [ "$actual_rows" != "$expected_rows" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedRows=$expected_rows actualRows=$actual_rows; "; fi; if [ "$expected_contains_enabled" = "1" ] && ! grep -F -- "$expected_contains" "$out" >/dev/null 2>&1; then assertion_status="failed"; assertion_message="${assertion_message}expectedContains not found; "; fi; if [ "$expected_scalar_enabled" = "1" ] && [ "$actual_scalar" != "$expected_scalar" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedScalar=$expected_scalar actualScalar=$actual_scalar; "; fi; if [ "$assertion_status" = "failed" ]; then status="failed"; fi; fi; actual_scalar_b64="$(printf '%s' "$actual_scalar" | base64 | tr -d '\n')"; assertion_message_b64="$(printf '%s' "$assertion_message" | base64 | tr -d '\n')"; if [ "$status" = "success" ]; then step "validation_${idx}" "success"; else step "validation_${idx}" "failed"; validation_failed=1; fi; printf 'OPSHUB_RESTORE_VALIDATION=%s|%s|%s|%s|%s|%s|%s|%s\n' "$idx" "$status" "$sha" "$preview" "$assertion_status" "$actual_rows" "$actual_scalar_b64" "$assertion_message_b64"; }`,
		`printf 'OPSHUB_WORK_DIR=%s\n' "$WORK_DIR"`,
		`printf 'OPSHUB_PREPARED_DATADIR=%s\n' "$DEST_DIR"`,
		`printf 'OPSHUB_CONTAINER_NAME=%s\n' "$CONTAINER_NAME"`,
		`printf 'OPSHUB_CONTAINER_IMAGE=%s\n' "$CONTAINER_IMAGE"`,
		`printf 'OPSHUB_LISTEN_HOST=127.0.0.1\n'`,
		`printf 'OPSHUB_LISTEN_PORT=%s\n' "$LISTEN_PORT"`,
		`printf 'OPSHUB_LOG_PATH=%s\n' "$LOG_FILE"`,
		`printf 'OPSHUB_PROOF_PATH=%s\n' "$PROOF_FILE"`,
		`printf 'OPSHUB_BARMAN_SERVER=%s\n' "$SERVER"`,
		`printf 'OPSHUB_BARMAN_BACKUP_ID=%s\n' "$BACKUP_ID"`,
		`printf 'OPSHUB_TARGET_TYPE=%s\n' "$TARGET_TYPE"`,
		`printf 'OPSHUB_TARGET_VALUE=%s\n' "$TARGET_VALUE"`,
		`printf 'OPSHUB_TARGET_TLI=%s\n' "$TARGET_TLI"`,
		`printf 'OPSHUB_TARGET_ACTION=%s\n' "$TARGET_ACTION"`,
		`printf 'OPSHUB_GET_WAL=%s\n' "$GET_WAL"`,
		`step "prepare_restore_directory" "running"`,
		`case "$DEST_DIR" in "$WORK_ROOT"/restore/job-"$RESTORE_JOB_ID"/pgdata) ;; *) fail_step "prepare_restore_directory" "unsafe restore destination: $DEST_DIR";; esac`,
		`rm -rf "$DEST_DIR"`,
		`mkdir -p "$DEST_DIR"`,
		`step "prepare_restore_directory" "success"`,
		`step "barman_restore" "running"`,
		`set -- restore "$SERVER" "$BACKUP_ID" "$DEST_DIR"`,
		`if [ "$TARGET_TYPE" = "time" ]; then set -- "$@" --target-time "$TARGET_VALUE"; else set -- "$@" --target-lsn "$TARGET_VALUE"; fi`,
		`if [ -n "$BARMAN_TARGET_TLI" ]; then set -- "$@" --target-tli "$BARMAN_TARGET_TLI"; fi`,
		`if [ -n "$TARGET_ACTION" ]; then set -- "$@" --target-action "$TARGET_ACTION"; fi`,
		`if [ "$GET_WAL" = "1" ]; then set -- "$@" --get-wal; else set -- "$@" --no-get-wal; fi`,
		`printf '%s barman command: barman %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG_FILE"`,
		`run_barman "$@" >> "$LOG_FILE" 2>&1 || fail_step "barman_restore" "barman restore failed"`,
		`step "barman_restore" "success"`,
		`step "verify_pgdata" "running"`,
		`test -f "$DEST_DIR/PG_VERSION" || fail_step "verify_pgdata" "PG_VERSION not found in restored PGDATA"`,
		`test -d "$DEST_DIR/global" || fail_step "verify_pgdata" "global directory not found in restored PGDATA"`,
		`test -d "$DEST_DIR/base" || fail_step "verify_pgdata" "base directory not found in restored PGDATA"`,
		`step "verify_pgdata" "success"`,
		`validation_failed=0`,
		`if [ "$START_INSTANCE" = "1" ]; then`,
		`  step "start_isolated_postgres" "running"`,
		`  DOCKER_BIN="$(command -v docker || true)"`,
		`  PSQL_BIN="$(command -v psql || true)"`,
		`  if [ -z "$DOCKER_BIN" ]; then fail_step "start_isolated_postgres" "docker not found"; fi`,
		`  if [ -z "$PSQL_BIN" ]; then fail_step "start_isolated_postgres" "psql not found"; fi`,
		`  "$DOCKER_BIN" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true`,
		`  chown -R 999:999 "$DEST_DIR" >> "$LOG_FILE" 2>&1 || true`,
		`  "$DOCKER_BIN" run -d --name "$CONTAINER_NAME" -p "127.0.0.1:$LISTEN_PORT:5432" -v "$DEST_DIR:/var/lib/postgresql/data" "$CONTAINER_IMAGE" -c listen_addresses='*' -c port=5432 >> "$LOG_FILE" 2>&1 || fail_step "start_isolated_postgres" "docker run failed"`,
		`  ready=0; for i in $(seq 1 90); do if PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -v ON_ERROR_STOP=1 -c "SELECT 1" >/dev/null 2>&1; then ready=1; break; fi; sleep 2; done`,
		`  if [ "$ready" != "1" ]; then "$DOCKER_BIN" logs "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fail_step "start_isolated_postgres" "isolated postgres not ready"; fi`,
		`  recovery_summary="$(PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -F '|' -c "SELECT pg_is_in_recovery(), pg_last_wal_replay_lsn()" 2>/dev/null || true)"`,
		`  printf 'OPSHUB_RECOVERY_SUMMARY=%s\n' "$recovery_summary"`,
		`  step "start_isolated_postgres" "success"`,
	}
	for idx, check := range input.ValidationChecks {
		expectedRowsEnabled, expectedRowsValue := validationExpectedRowsArgs(check.ExpectedRows)
		expectedContainsEnabled, expectedContainsValue := validationExpectedStringArgs(check.ExpectedContains)
		expectedScalarEnabled, expectedScalarValue := validationExpectedStringArgs(check.ExpectedScalar)
		lines = append(lines, fmt.Sprintf("  run_pg_validation %d %s %s %s %s %s %s %s",
			idx+1,
			shellSingleQuote(check.SQL),
			shellSingleQuote(expectedRowsEnabled),
			shellSingleQuote(expectedRowsValue),
			shellSingleQuote(expectedContainsEnabled),
			shellSingleQuote(expectedContainsValue),
			shellSingleQuote(expectedScalarEnabled),
			shellSingleQuote(expectedScalarValue),
		))
	}
	lines = append(lines,
		`  if [ "$validation_failed" = "0" ]; then printf 'OPSHUB_VALIDATION_STATUS=passed\n'; else printf 'OPSHUB_VALIDATION_STATUS=failed\n'; fi`,
		`else`,
		`  printf 'OPSHUB_VALIDATION_STATUS=warning\n'`,
		`fi`,
		`GENERATED_AT="$(date '+%Y-%m-%d %H:%M:%S')"`,
		`printf '{"restoreJobId":%s,"restorePlanId":%s,"workDir":"%s","preparedDatadir":"%s","containerName":"%s","containerImage":"%s","listenHost":"127.0.0.1","listenPort":%s,"barmanServerName":"%s","backupId":"%s","targetType":"%s","targetValue":"%s","targetTimelineId":"%s","targetAction":"%s","getWal":"%s","startInstance":"%s","generatedAt":"%s"}\n' "$RESTORE_JOB_ID" "$RESTORE_PLAN_ID" "$WORK_DIR" "$DEST_DIR" "$CONTAINER_NAME" "$CONTAINER_IMAGE" "$LISTEN_PORT" "$SERVER" "$BACKUP_ID" "$TARGET_TYPE" "$TARGET_VALUE" "$TARGET_TLI" "$TARGET_ACTION" "$GET_WAL" "$START_INSTANCE" "$GENERATED_AT" > "$PROOF_FILE"`,
		`printf 'OPSHUB_ARTIFACT_URI=runner://runner-host-`+strconv.Itoa(int(input.RunnerHostID))+`%s\n' "$PROOF_FILE"`,
		`step "generate_proof" "success"`,
	)
	return strings.Join(lines, "\n"), nil
}

func parseBarmanRestoreOutput(stdout string) barmanRestoreRunnerResult {
	result := barmanRestoreRunnerResult{}
	kv := parseRunnerKeyValueOutput(stdout)
	result.WorkDir = kv["OPSHUB_WORK_DIR"]
	result.PreparedDatadir = kv["OPSHUB_PREPARED_DATADIR"]
	result.ContainerName = kv["OPSHUB_CONTAINER_NAME"]
	result.ContainerImage = kv["OPSHUB_CONTAINER_IMAGE"]
	result.ListenHost = kv["OPSHUB_LISTEN_HOST"]
	result.ListenPort, _ = strconv.Atoi(kv["OPSHUB_LISTEN_PORT"])
	result.LogPath = kv["OPSHUB_LOG_PATH"]
	result.ProofPath = kv["OPSHUB_PROOF_PATH"]
	result.ArtifactURI = kv["OPSHUB_ARTIFACT_URI"]
	result.BarmanServerName = kv["OPSHUB_BARMAN_SERVER"]
	result.BackupID = kv["OPSHUB_BARMAN_BACKUP_ID"]
	result.TargetType = kv["OPSHUB_TARGET_TYPE"]
	result.TargetValue = kv["OPSHUB_TARGET_VALUE"]
	result.TargetTimelineID = kv["OPSHUB_TARGET_TLI"]
	result.TargetAction = kv["OPSHUB_TARGET_ACTION"]
	result.GetWAL = kv["OPSHUB_GET_WAL"] == "1"
	result.ValidationStatus = kv["OPSHUB_VALIDATION_STATUS"]
	result.RecoverySummary = kv["OPSHUB_RECOVERY_SUMMARY"]
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if value, ok := strings.CutPrefix(line, "OPSHUB_RESTORE_STEP="); ok {
			parts := strings.SplitN(value, "|", 3)
			if len(parts) == 3 {
				result.Steps = append(result.Steps, physicalRestoreStep{Name: parts[0], Status: parts[1], OccurredAt: parts[2]})
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "OPSHUB_RESTORE_VALIDATION="); ok {
			parts := strings.SplitN(value, "|", 8)
			if len(parts) >= 4 {
				idx, _ := strconv.Atoi(parts[0])
				previewBytes, _ := base64.StdEncoding.DecodeString(parts[3])
				item := physicalRestoreValidationResult{
					Index:         idx,
					Status:        parts[1],
					SHA256:        parts[2],
					OutputPreview: trimText(string(previewBytes), 1200),
				}
				if len(parts) == 8 {
					item.AssertionStatus = parts[4]
					if actualRows, err := strconv.Atoi(strings.TrimSpace(parts[5])); err == nil {
						item.ActualRows = &actualRows
					}
					if scalarBytes, err := base64.StdEncoding.DecodeString(parts[6]); err == nil {
						item.ActualScalar = string(scalarBytes)
					}
					if messageBytes, err := base64.StdEncoding.DecodeString(parts[7]); err == nil {
						item.AssertionMessage = trimText(string(messageBytes), 1000)
					}
				}
				result.ValidationResults = append(result.ValidationResults, item)
			}
		}
	}
	return result
}

func normalizeBarmanRestoreTargetAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shutdown", "promote":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "pause"
	}
}

func extractTargetTimelineFromPlan(plan *DatabaseRestorePlan) string {
	if plan == nil || strings.TrimSpace(plan.PlanJSON) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(plan.PlanJSON), &payload); err != nil {
		return ""
	}
	if target, ok := payload["target"].(map[string]any); ok {
		if value, ok := target["timelineId"].(string); ok {
			return pgwal.NormalizeTimelineID(value)
		}
	}
	if value, ok := payload["targetTimelineId"].(string); ok {
		return pgwal.NormalizeTimelineID(value)
	}
	return ""
}

func buildPhysicalRestoreScript(input physicalRestoreScriptInput) (string, error) {
	if input.RestoreJobID == 0 || input.RunnerHostID == 0 {
		return "", fmt.Errorf("恢复任务上下文不完整")
	}
	if strings.TrimSpace(input.Base.SourcePath) == "" {
		return "", fmt.Errorf("base backup 缺少可访问路径")
	}
	if strings.TrimSpace(input.ToolName) == "" {
		return "", fmt.Errorf("恢复工具不能为空")
	}
	if input.ListenPort <= 0 || input.ListenPort > 65535 {
		return "", fmt.Errorf("隔离实例端口不合法")
	}
	if strings.TrimSpace(input.ContainerImage) == "" || !isSafeRestoreContainerImage(input.ContainerImage) {
		return "", fmt.Errorf("隔离容器镜像不合法")
	}
	if strings.TrimSpace(input.ContainerName) == "" || !isSafeRestoreContainerName(input.ContainerName) {
		return "", fmt.Errorf("隔离容器名称不合法")
	}
	if err := validateRestoreArtifactForScript(input.Base); err != nil {
		return "", err
	}
	for _, item := range input.Incrementals {
		if err := validateRestoreArtifactForScript(item); err != nil {
			return "", err
		}
	}
	for _, item := range input.Logs {
		if err := validateRestoreArtifactForScript(item); err != nil {
			return "", err
		}
	}
	lines := []string{
		"set -eu",
		"WORK_ROOT=" + shellSingleQuote(input.WorkRoot),
		fmt.Sprintf("RESTORE_JOB_ID=%d", input.RestoreJobID),
		fmt.Sprintf("RESTORE_PLAN_ID=%d", input.RestorePlanID),
		"WORK_DIR=\"$WORK_ROOT/restore/job-$RESTORE_JOB_ID\"",
		"ARTIFACT_DIR=\"$WORK_DIR/artifacts\"",
		"BINLOG_DIR=\"$WORK_DIR/binlogs\"",
		"PREPARE_ROOT=\"$WORK_DIR/prepared\"",
		"BASE_DIR=\"$PREPARE_ROOT/base\"",
		"LOG_FILE=\"$WORK_DIR/restore.log\"",
		"PROOF_FILE=\"$WORK_DIR/proof.json\"",
		"CONTAINER_NAME=" + shellSingleQuote(input.ContainerName),
		"CONTAINER_IMAGE=" + shellSingleQuote(input.ContainerImage),
		fmt.Sprintf("LISTEN_PORT=%d", input.ListenPort),
		"TARGET_TIME=" + shellSingleQuote(input.TargetTime),
		fmt.Sprintf("BACKUP_BINLOG_POS=%d", input.BackupBinlogPos),
		"PREPARE_TOOL_NAME=" + shellSingleQuote(input.ToolName),
		"CLEANUP_ON_FAILURE=" + boolShellValue(input.CleanupOnFailure),
		`mkdir -p "$ARTIFACT_DIR" "$BINLOG_DIR" "$PREPARE_ROOT"`,
		`: > "$LOG_FILE"`,
		`step() { printf 'OPSHUB_RESTORE_STEP=%s|%s|%s\n' "$1" "$2" "$(date '+%Y-%m-%d %H:%M:%S')"; printf '%s %s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG_FILE"; }`,
		`cleanup_failure() { code="$?"; if [ "$code" != "0" ] && [ "$CLEANUP_ON_FAILURE" = "1" ]; then DOCKER_BIN="$(command -v docker || true)"; if [ -n "$DOCKER_BIN" ]; then "$DOCKER_BIN" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fi; printf '%s cleanup_on_failure container_removed\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_FILE" || true; fi; exit "$code"; }`,
		`trap cleanup_failure EXIT`,
		`fail_step() { step "$1" "failed"; echo "$2" >> "$LOG_FILE"; exit 1; }`,
		`copy_artifact() { label="$1"; src="$2"; dest="$3"; expected_sha="$4"; expected_size="$5"; step "fetch_${label}" "running"; if [ ! -f "$src" ]; then fail_step "fetch_${label}" "artifact not found: $src"; fi; cp "$src" "$dest"; actual_size="$(wc -c < "$dest" | tr -d ' ')"; if [ "$expected_size" != "0" ] && [ "$actual_size" != "$expected_size" ]; then fail_step "fetch_${label}" "size mismatch: $actual_size != $expected_size"; fi; if [ -n "$expected_sha" ]; then actual_sha="$(sha256sum "$dest" | awk '{print $1}')"; if [ "$actual_sha" != "$expected_sha" ]; then fail_step "fetch_${label}" "sha256 mismatch"; fi; fi; step "fetch_${label}" "success"; }`,
		`run_validation() { idx="$1"; sql="$2"; expected_rows_enabled="$3"; expected_rows="$4"; expected_contains_enabled="$5"; expected_contains="$6"; expected_scalar_enabled="$7"; expected_scalar="$8"; out="$WORK_DIR/validation-$idx.out"; clean="$WORK_DIR/validation-$idx.data"; step "validation_${idx}" "running"; set +e; if command -v timeout >/dev/null 2>&1; then timeout 30 "$MYSQL_CLI" -h127.0.0.1 -P "$LISTEN_PORT" -uroot --batch --raw -e "$sql" < /dev/null > "$out" 2>&1; else "$MYSQL_CLI" -h127.0.0.1 -P "$LISTEN_PORT" -uroot --batch --raw -e "$sql" < /dev/null > "$out" 2>&1; fi; code="$?"; set -e; grep -v '^WARNING:' "$out" > "$clean" || true; sha="$(sha256sum "$out" | awk '{print $1}')"; preview="$(head -c 1200 "$out" | base64 | tr -d '\n')"; actual_rows="$(awk 'NR>1 {count++} END {print count+0}' "$clean")"; actual_scalar="$(awk 'NR>1 {print; exit}' "$clean" | awk -F '\t' '{print $1}')"; status="success"; assertion_status="not_configured"; assertion_message=""; if [ "$code" != "0" ]; then status="failed"; assertion_status="failed"; assertion_message="SQL execution failed"; else if [ "$expected_rows_enabled" = "1" ] || [ "$expected_contains_enabled" = "1" ] || [ "$expected_scalar_enabled" = "1" ]; then assertion_status="passed"; fi; if [ "$expected_rows_enabled" = "1" ] && [ "$actual_rows" != "$expected_rows" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedRows=$expected_rows actualRows=$actual_rows; "; fi; if [ "$expected_contains_enabled" = "1" ] && ! grep -F -- "$expected_contains" "$clean" >/dev/null 2>&1; then assertion_status="failed"; assertion_message="${assertion_message}expectedContains not found; "; fi; if [ "$expected_scalar_enabled" = "1" ] && [ "$actual_scalar" != "$expected_scalar" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedScalar=$expected_scalar actualScalar=$actual_scalar; "; fi; if [ "$assertion_status" = "failed" ]; then status="failed"; fi; fi; actual_scalar_b64="$(printf '%s' "$actual_scalar" | base64 | tr -d '\n')"; assertion_message_b64="$(printf '%s' "$assertion_message" | base64 | tr -d '\n')"; if [ "$status" = "success" ]; then step "validation_${idx}" "success"; else step "validation_${idx}" "failed"; validation_failed=1; fi; printf 'OPSHUB_RESTORE_VALIDATION=%s|%s|%s|%s|%s|%s|%s|%s\n' "$idx" "$status" "$sha" "$preview" "$assertion_status" "$actual_rows" "$actual_scalar_b64" "$assertion_message_b64"; }`,
		`printf 'OPSHUB_WORK_DIR=%s\n' "$WORK_DIR"`,
		`printf 'OPSHUB_PREPARED_DATADIR=%s\n' "$BASE_DIR"`,
		`printf 'OPSHUB_CONTAINER_NAME=%s\n' "$CONTAINER_NAME"`,
		`printf 'OPSHUB_CONTAINER_IMAGE=%s\n' "$CONTAINER_IMAGE"`,
		`printf 'OPSHUB_LISTEN_HOST=127.0.0.1\n'`,
		`printf 'OPSHUB_LISTEN_PORT=%s\n' "$LISTEN_PORT"`,
		`printf 'OPSHUB_LOG_PATH=%s\n' "$LOG_FILE"`,
		`printf 'OPSHUB_PROOF_PATH=%s\n' "$PROOF_FILE"`,
	}
	lines = append(lines, buildRestoreCopyArtifactLine("base", input.Base, `$ARTIFACT_DIR/base.physical.tar.gz`))
	lines = append(lines,
		`step "extract_base_backup" "running"`,
		`rm -rf "$BASE_DIR"`,
		`mkdir -p "$BASE_DIR"`,
		`tar -xzf "$ARTIFACT_DIR/base.physical.tar.gz" -C "$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "extract_base_backup" "extract base backup failed"`,
		`step "extract_base_backup" "success"`,
	)
	for idx, inc := range input.Incrementals {
		label := fmt.Sprintf("incremental_%d", idx+1)
		dest := fmt.Sprintf(`$ARTIFACT_DIR/inc-%d.physical.tar.gz`, idx+1)
		incDir := fmt.Sprintf(`$PREPARE_ROOT/inc-%d`, idx+1)
		lines = append(lines, buildRestoreCopyArtifactLine(label, inc, dest))
		lines = append(lines,
			fmt.Sprintf(`rm -rf "%s"`, incDir),
			fmt.Sprintf(`mkdir -p "%s"`, incDir),
			fmt.Sprintf(`tar -xzf "%s" -C "%s" >> "$LOG_FILE" 2>&1 || fail_step "extract_%s" "extract incremental failed"`, dest, incDir, label),
		)
	}
	lines = append(lines,
		`step "prepare_physical_backup" "running"`,
		`PREPARE_TOOL="$(command -v "$PREPARE_TOOL_NAME" || true)"`,
		`if [ -z "$PREPARE_TOOL" ] && [ "$PREPARE_TOOL_NAME" = "xtrabackup" ]; then PREPARE_TOOL="$(command -v xtrabackup || true)"; fi`,
		`if [ -z "$PREPARE_TOOL" ] && [ "$PREPARE_TOOL_NAME" = "mariadb-backup" ]; then PREPARE_TOOL="$(command -v mariadb-backup || true)"; fi`,
		`if [ -z "$PREPARE_TOOL" ]; then fail_step "prepare_physical_backup" "physical prepare tool not found"; fi`,
	)
	if len(input.Incrementals) == 0 {
		lines = append(lines, `"$PREPARE_TOOL" --prepare --target-dir="$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "prepare_physical_backup" "prepare full backup failed"`)
	} else {
		lines = append(lines, `"$PREPARE_TOOL" --prepare --apply-log-only --target-dir="$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "prepare_physical_backup" "prepare base apply-log-only failed"`)
		for idx := range input.Incrementals {
			incDir := fmt.Sprintf(`$PREPARE_ROOT/inc-%d`, idx+1)
			if idx+1 < len(input.Incrementals) {
				lines = append(lines, fmt.Sprintf(`"$PREPARE_TOOL" --prepare --apply-log-only --target-dir="$BASE_DIR" --incremental-dir="%s" >> "$LOG_FILE" 2>&1 || fail_step "prepare_physical_backup" "prepare incremental %d failed"`, incDir, idx+1))
			} else {
				lines = append(lines, fmt.Sprintf(`"$PREPARE_TOOL" --prepare --target-dir="$BASE_DIR" --incremental-dir="%s" >> "$LOG_FILE" 2>&1 || fail_step "prepare_physical_backup" "prepare final incremental failed"`, incDir))
			}
		}
	}
	lines = append(lines, `step "prepare_physical_backup" "success"`)
	for idx, logArtifact := range input.Logs {
		label := fmt.Sprintf("binlog_%d", idx+1)
		dest := fmt.Sprintf(`$BINLOG_DIR/%s`, logArtifact.FileName)
		lines = append(lines, buildRestoreCopyArtifactLine(label, logArtifact, dest))
	}
	lines = append(lines,
		`step "start_isolated_instance" "running"`,
		`DOCKER="$(command -v docker || true)"`,
		`if [ -z "$DOCKER" ]; then fail_step "start_isolated_instance" "docker not found"; fi`,
		`MYSQL_CLI="$(command -v mysql || command -v mariadb || true)"`,
		`if [ -z "$MYSQL_CLI" ]; then fail_step "start_isolated_instance" "mysql/mariadb client not found"; fi`,
		`MYSQLBINLOG="$(command -v mysqlbinlog || command -v mariadb-binlog || true)"`,
		`if [ -z "$MYSQLBINLOG" ]; then fail_step "start_isolated_instance" "mysqlbinlog/mariadb-binlog not found"; fi`,
		`"$DOCKER" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true`,
		`chown -R 999:999 "$BASE_DIR" >> "$LOG_FILE" 2>&1 || true`,
		`"$DOCKER" run -d --name "$CONTAINER_NAME" -p "127.0.0.1:$LISTEN_PORT:3306" -v "$BASE_DIR:/var/lib/mysql" "$CONTAINER_IMAGE" --skip-grant-tables --skip-networking=0 >> "$LOG_FILE" 2>&1 || fail_step "start_isolated_instance" "docker run failed"`,
		`ready=0; for i in $(seq 1 60); do if "$MYSQL_CLI" -h127.0.0.1 -P "$LISTEN_PORT" -uroot -e "SELECT 1" < /dev/null >/dev/null 2>&1; then ready=1; break; fi; sleep 2; done; if [ "$ready" != "1" ]; then "$DOCKER" logs "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fail_step "start_isolated_instance" "isolated mysql not ready"; fi`,
		`step "start_isolated_instance" "success"`,
	)
	if len(input.Logs) > 0 {
		logFiles := make([]string, 0, len(input.Logs))
		for _, item := range input.Logs {
			logFiles = append(logFiles, fmt.Sprintf(`"$BINLOG_DIR/%s"`, item.FileName))
		}
		startPositionArg := ""
		if input.BackupBinlogPos > 0 {
			startPositionArg = fmt.Sprintf("--start-position=%d ", input.BackupBinlogPos)
		}
		lines = append(lines,
			`step "apply_binlog_to_target" "running"`,
			`set +e`,
			fmt.Sprintf(`"$MYSQLBINLOG" %s--stop-datetime="$TARGET_TIME" %s | "$MYSQL_CLI" -h127.0.0.1 -P "$LISTEN_PORT" -uroot >> "$LOG_FILE" 2>&1`, startPositionArg, strings.Join(logFiles, " ")),
			`apply_code="$?"`,
			`set -e`,
			`if [ "$apply_code" != "0" ]; then fail_step "apply_binlog_to_target" "mysqlbinlog replay failed"; fi`,
			`printf 'OPSHUB_APPLY_BINLOG_SUMMARY=%s\n' "applied archived binlogs to $TARGET_TIME"`,
			`step "apply_binlog_to_target" "success"`,
		)
	} else {
		lines = append(lines,
			`step "apply_binlog_to_target" "success"`,
			`printf 'OPSHUB_APPLY_BINLOG_SUMMARY=%s\n' "no archived binlog required for target time"`,
		)
	}
	lines = append(lines, `validation_failed=0`)
	for idx, check := range input.ValidationChecks {
		expectedRowsEnabled, expectedRowsValue := validationExpectedRowsArgs(check.ExpectedRows)
		expectedContainsEnabled, expectedContainsValue := validationExpectedStringArgs(check.ExpectedContains)
		expectedScalarEnabled, expectedScalarValue := validationExpectedStringArgs(check.ExpectedScalar)
		lines = append(lines, fmt.Sprintf("run_validation %d %s %s %s %s %s %s %s",
			idx+1,
			shellSingleQuote(check.SQL),
			shellSingleQuote(expectedRowsEnabled),
			shellSingleQuote(expectedRowsValue),
			shellSingleQuote(expectedContainsEnabled),
			shellSingleQuote(expectedContainsValue),
			shellSingleQuote(expectedScalarEnabled),
			shellSingleQuote(expectedScalarValue),
		))
	}
	lines = append(lines,
		`if [ "$validation_failed" = "0" ]; then printf 'OPSHUB_VALIDATION_STATUS=passed\n'; else printf 'OPSHUB_VALIDATION_STATUS=failed\n'; fi`,
		`GENERATED_AT="$(date '+%Y-%m-%d %H:%M:%S')"`,
		`printf '{"restoreJobId":%s,"restorePlanId":%s,"workDir":"%s","containerName":"%s","containerImage":"%s","listenHost":"127.0.0.1","listenPort":%s,"generatedAt":"%s"}\n' "$RESTORE_JOB_ID" "$RESTORE_PLAN_ID" "$WORK_DIR" "$CONTAINER_NAME" "$CONTAINER_IMAGE" "$LISTEN_PORT" "$GENERATED_AT" > "$PROOF_FILE"`,
		`printf 'OPSHUB_ARTIFACT_URI=runner://runner-host-`+strconv.Itoa(int(input.RunnerHostID))+`%s\n' "$PROOF_FILE"`,
		`step "generate_proof" "success"`,
	)
	return strings.Join(lines, "\n"), nil
}

func buildRestoreCleanupScript(job *DatabaseRestoreJob) (string, error) {
	if job == nil {
		return "", fmt.Errorf("恢复任务不存在")
	}
	if !isSafeRestoreWorkDir(job.WorkDir) {
		return "", fmt.Errorf("恢复工作目录不安全")
	}
	if strings.TrimSpace(job.ContainerName) != "" && !isSafeRestoreContainerName(job.ContainerName) {
		return "", fmt.Errorf("隔离容器名称不合法")
	}
	lines := []string{
		"set -eu",
		"WORK_DIR=" + shellSingleQuote(job.WorkDir),
		"CONTAINER_NAME=" + shellSingleQuote(job.ContainerName),
		`if [ -n "$CONTAINER_NAME" ]; then DOCKER="$(command -v docker || true)"; if [ -n "$DOCKER" ]; then "$DOCKER" rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true; fi; fi`,
		`rm -rf "$WORK_DIR"`,
		`printf 'OPSHUB_CLEANUP_STATUS=cleaned\n'`,
	}
	return strings.Join(lines, "\n"), nil
}

func buildRestoreCopyArtifactLine(label string, artifact physicalRestoreArtifact, dest string) string {
	return fmt.Sprintf("copy_artifact %s %s \"%s\" %s %s",
		shellSingleQuote(label),
		shellSingleQuote(artifact.SourcePath),
		dest,
		shellSingleQuote(artifact.ChecksumSHA256),
		shellSingleQuote(strconv.FormatInt(maxInt64(artifact.FileSize, 0), 10)),
	)
}

func validateRestoreArtifactForScript(item physicalRestoreArtifact) error {
	if strings.TrimSpace(item.FileName) == "" || !isSafeRunnerFileName(item.FileName) {
		return fmt.Errorf("恢复 artifact 文件名不合法: %s", item.FileName)
	}
	if strings.TrimSpace(item.SourcePath) == "" || strings.ContainsAny(item.SourcePath, "\x00\r\n") {
		return fmt.Errorf("恢复 artifact 路径不合法: %s", item.FileName)
	}
	if checksum := strings.TrimSpace(item.ChecksumSHA256); checksum != "" && len(checksum) != 64 {
		return fmt.Errorf("恢复 artifact checksum 不合法: %s", item.FileName)
	}
	return nil
}

func parsePhysicalRestoreOutput(stdout string) physicalRestoreRunnerResult {
	result := physicalRestoreRunnerResult{}
	kv := parseRunnerKeyValueOutput(stdout)
	result.WorkDir = kv["OPSHUB_WORK_DIR"]
	result.PreparedDatadir = kv["OPSHUB_PREPARED_DATADIR"]
	result.ContainerName = kv["OPSHUB_CONTAINER_NAME"]
	result.ContainerImage = kv["OPSHUB_CONTAINER_IMAGE"]
	result.ListenHost = kv["OPSHUB_LISTEN_HOST"]
	result.ListenPort, _ = strconv.Atoi(kv["OPSHUB_LISTEN_PORT"])
	result.LogPath = kv["OPSHUB_LOG_PATH"]
	result.ProofPath = kv["OPSHUB_PROOF_PATH"]
	result.ArtifactURI = kv["OPSHUB_ARTIFACT_URI"]
	result.ValidationStatus = kv["OPSHUB_VALIDATION_STATUS"]
	result.ApplyBinlogSummary = kv["OPSHUB_APPLY_BINLOG_SUMMARY"]
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if value, ok := strings.CutPrefix(line, "OPSHUB_RESTORE_STEP="); ok {
			parts := strings.SplitN(value, "|", 3)
			if len(parts) == 3 {
				result.Steps = append(result.Steps, physicalRestoreStep{Name: parts[0], Status: parts[1], OccurredAt: parts[2]})
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "OPSHUB_RESTORE_VALIDATION="); ok {
			parts := strings.SplitN(value, "|", 8)
			if len(parts) >= 4 {
				idx, _ := strconv.Atoi(parts[0])
				previewBytes, _ := base64.StdEncoding.DecodeString(parts[3])
				item := physicalRestoreValidationResult{
					Index:         idx,
					Status:        parts[1],
					SHA256:        parts[2],
					OutputPreview: trimText(string(previewBytes), 1200),
				}
				if len(parts) == 8 {
					item.AssertionStatus = parts[4]
					if actualRows, err := strconv.Atoi(strings.TrimSpace(parts[5])); err == nil {
						item.ActualRows = &actualRows
					}
					if scalarBytes, err := base64.StdEncoding.DecodeString(parts[6]); err == nil {
						item.ActualScalar = string(scalarBytes)
					}
					if messageBytes, err := base64.StdEncoding.DecodeString(parts[7]); err == nil {
						item.AssertionMessage = trimText(string(messageBytes), 1000)
					}
				}
				result.ValidationResults = append(result.ValidationResults, item)
			}
		}
	}
	return result
}

func (uc *UseCase) restoreBackupArtifacts(ctx context.Context, selectedJSON string, runnerHostID, baseID uint) ([]physicalRestoreArtifact, error) {
	ids := parseUintListJSON(selectedJSON)
	result := make([]physicalRestoreArtifact, 0, len(ids))
	for _, id := range ids {
		if id == 0 || id == baseID {
			continue
		}
		record, err := uc.backupRecordRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("增量备份 #%d 不存在", id)
		}
		artifact, err := restoreArtifactFromBackupRecord(record, runnerHostID, "incremental")
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	return result, nil
}

func (uc *UseCase) restoreLogArtifacts(ctx context.Context, selectedJSON string, runnerHostID uint) ([]physicalRestoreArtifact, error) {
	ids := parseUintListJSON(selectedJSON)
	result := make([]physicalRestoreArtifact, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		item, err := uc.logArchiveRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("日志归档 #%d 不存在", id)
		}
		path, err := resolveRunnerReadableArtifactPath(item.StorageURI, "", runnerHostID)
		if err != nil {
			return nil, fmt.Errorf("日志归档 %s 不可由 Runner 读取: %w", item.FileName, err)
		}
		result = append(result, physicalRestoreArtifact{
			Kind:           item.ArchiveType,
			ID:             item.ID,
			FileName:       item.FileName,
			SourcePath:     path,
			StorageURI:     item.StorageURI,
			ChecksumSHA256: strings.TrimSpace(item.ChecksumSHA256),
			FileSize:       item.FileSize,
		})
	}
	return result, nil
}

func restoreArtifactFromBackupRecord(record *DatabaseBackupRecord, runnerHostID uint, kind string) (physicalRestoreArtifact, error) {
	if record == nil {
		return physicalRestoreArtifact{}, fmt.Errorf("备份记录不存在")
	}
	path, err := resolveRunnerReadableArtifactPath(record.StorageURI, record.FilePath, runnerHostID)
	if err != nil {
		return physicalRestoreArtifact{}, fmt.Errorf("备份记录 #%d 不可由 Runner 读取: %w", record.ID, err)
	}
	return physicalRestoreArtifact{
		Kind:           kind,
		ID:             record.ID,
		FileName:       firstNonEmpty(record.FileName, filepath.Base(path)),
		SourcePath:     path,
		StorageURI:     firstNonEmpty(record.StorageURI, record.FilePath),
		ChecksumSHA256: strings.TrimSpace(record.ChecksumSHA256),
		FileSize:       record.FileSize,
		BackupLevel:    record.BackupLevel,
	}, nil
}

func resolveRunnerReadableArtifactPath(storageURI, filePath string, runnerHostID uint) (string, error) {
	for _, candidate := range []string{storageURI, filePath} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "runner://runner-host-") {
			rest := strings.TrimPrefix(candidate, "runner://runner-host-")
			idText, pathPart, ok := strings.Cut(rest, "/")
			if !ok {
				return "", fmt.Errorf("runner URI 缺少路径")
			}
			id64, _ := strconv.ParseUint(idText, 10, 64)
			if uint(id64) != runnerHostID {
				return "", fmt.Errorf("runner URI 属于 runner-host-%d，当前 Runner 为 %d", id64, runnerHostID)
			}
			return "/" + pathPart, nil
		}
		if strings.HasPrefix(candidate, "local://") {
			return strings.TrimPrefix(candidate, "local://"), nil
		}
		if strings.HasPrefix(candidate, "file://") {
			u, err := url.Parse(candidate)
			if err != nil {
				return "", err
			}
			return u.Path, nil
		}
		if strings.HasPrefix(candidate, "/") {
			return candidate, nil
		}
		if strings.HasPrefix(candidate, "metadata://") {
			return "", fmt.Errorf("metadata URI 仅用于预校验，不能作为实际恢复输入")
		}
	}
	return "", fmt.Errorf("缺少 runner/local/file 可访问路径")
}

func normalizeRestoreValidationSQL(dbType string, values []string) ([]string, error) {
	checks, err := normalizeRestoreValidationChecks(dbType, nil, values, nil)
	if err != nil {
		return nil, err
	}
	return validationCheckSQLs(checks), nil
}

func normalizeRestoreValidationChecks(dbType string, defaultSQL, values []string, assertions []DatabaseRestoreValidationAssertion) ([]restoreValidationCheck, error) {
	checks := make([]restoreValidationCheck, 0, len(defaultSQL)+len(values)+len(assertions))
	seen := map[string]struct{}{}
	for _, sqlText := range append(defaultSQL, values...) {
		cleaned, err := normalizeSingleRestoreValidationSQL(dbType, sqlText)
		if err != nil {
			return nil, err
		}
		if cleaned == "" {
			continue
		}
		key := "sql:" + strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		checks = append(checks, restoreValidationCheck{SQL: cleaned})
	}
	for _, assertion := range assertions {
		check, err := normalizeRestoreValidationAssertion(dbType, assertion)
		if err != nil {
			return nil, err
		}
		key := restoreValidationCheckKey(check)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		checks = append(checks, check)
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("校验 SQL 不能为空")
	}
	if len(checks) > 20 {
		return nil, fmt.Errorf("校验 SQL 最多 20 条")
	}
	return checks, nil
}

func normalizeSingleRestoreValidationSQL(dbType, sqlText string) (string, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return "", nil
	}
	safety := AnalyzeReadOnlySQLByDB(dbType, sqlText, defaultRestoreValidationLimit)
	if !safety.Allowed {
		return "", fmt.Errorf("校验 SQL 不安全: %s", safety.Message)
	}
	return strings.TrimSpace(safety.SQLText), nil
}

func normalizeRestoreValidationAssertion(dbType string, assertion DatabaseRestoreValidationAssertion) (restoreValidationCheck, error) {
	cleaned, err := normalizeSingleRestoreValidationSQL(dbType, assertion.SQL)
	if err != nil {
		return restoreValidationCheck{}, err
	}
	if cleaned == "" {
		return restoreValidationCheck{}, fmt.Errorf("断言校验 SQL 不能为空")
	}
	check := restoreValidationCheck{SQL: cleaned}
	if assertion.ExpectedRows != nil {
		if *assertion.ExpectedRows < 0 {
			return restoreValidationCheck{}, fmt.Errorf("expectedRows 不能小于 0")
		}
		value := *assertion.ExpectedRows
		check.ExpectedRows = &value
	}
	if assertion.ExpectedContains != nil {
		value := trimText(strings.TrimSpace(*assertion.ExpectedContains), 1000)
		if value == "" {
			return restoreValidationCheck{}, fmt.Errorf("expectedContains 不能为空")
		}
		check.ExpectedContains = &value
	}
	if assertion.ExpectedScalar != nil {
		value := trimText(strings.TrimSpace(*assertion.ExpectedScalar), 1000)
		check.ExpectedScalar = &value
	}
	if check.ExpectedRows == nil && check.ExpectedContains == nil && check.ExpectedScalar == nil {
		return restoreValidationCheck{}, fmt.Errorf("断言校验至少需要 expectedRows、expectedContains 或 expectedScalar 之一")
	}
	return check, nil
}

func restoreValidationCheckKey(check restoreValidationCheck) string {
	parts := []string{strings.ToLower(check.SQL)}
	if check.ExpectedRows != nil {
		parts = append(parts, "rows="+strconv.Itoa(*check.ExpectedRows))
	}
	if check.ExpectedContains != nil {
		parts = append(parts, "contains="+*check.ExpectedContains)
	}
	if check.ExpectedScalar != nil {
		parts = append(parts, "scalar="+*check.ExpectedScalar)
	}
	return strings.Join(parts, "\x00")
}

func validationCheckSQLs(checks []restoreValidationCheck) []string {
	values := make([]string, 0, len(checks))
	for _, check := range checks {
		if strings.TrimSpace(check.SQL) != "" {
			values = append(values, check.SQL)
		}
	}
	return values
}

func validationExpectedRowsArgs(value *int) (string, string) {
	if value == nil {
		return "0", ""
	}
	return "1", strconv.Itoa(*value)
}

func validationExpectedStringArgs(value *string) (string, string) {
	if value == nil {
		return "0", ""
	}
	return "1", *value
}

func attachRestoreValidationChecks(results []physicalRestoreValidationResult, checks []restoreValidationCheck) []physicalRestoreValidationResult {
	if len(results) == 0 || len(checks) == 0 {
		return results
	}
	for idx := range results {
		checkIdx := results[idx].Index - 1
		if checkIdx < 0 || checkIdx >= len(checks) {
			continue
		}
		check := checks[checkIdx]
		results[idx].SQL = check.SQL
		results[idx].ExpectedRows = check.ExpectedRows
		results[idx].ExpectedContains = check.ExpectedContains
		results[idx].ExpectedScalar = check.ExpectedScalar
		if results[idx].AssertionStatus == "" {
			if check.ExpectedRows != nil || check.ExpectedContains != nil || check.ExpectedScalar != nil {
				results[idx].AssertionStatus = "unknown"
			} else {
				results[idx].AssertionStatus = "not_configured"
			}
		}
	}
	return results
}

func defaultRestoreContainerImage(source *DatabaseInstance, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	dbType := DBTypeMySQL
	version := ""
	if source != nil {
		dbType = normalizeDBType(source.DBType)
		version = source.Version
	}
	major, minor := parseMajorMinorVersion(version)
	if dbType == DBTypeMariaDB {
		if major > 0 {
			return fmt.Sprintf("mariadb:%d.%d", major, minor)
		}
		return "mariadb:latest"
	}
	if major == 8 && minor >= 4 {
		return "mysql:8.4"
	}
	if major == 5 {
		return "mysql:5.7"
	}
	return "mysql:8.0"
}

func defaultPostgreSQLRestoreContainerImage(source *DatabaseInstance, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return configured
	}
	version := ""
	if source != nil {
		version = source.Version
	}
	major, _ := parseMajorMinorVersion(version)
	if major > 0 {
		return fmt.Sprintf("postgres:%d", major)
	}
	return "postgres:latest"
}

func normalizeRestoreListenPort(value int) int {
	if value >= 1024 && value <= 65535 {
		return value
	}
	return 24000 + int(time.Now().UnixNano()%20000)
}

func parseUintListJSON(value string) []uint {
	var values []uint
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &values); err != nil {
		return nil
	}
	return values
}

func validationStatusFromResults(values []physicalRestoreValidationResult) string {
	if len(values) == 0 {
		return DatabasePlanValidationWarning
	}
	for _, item := range values {
		if item.Status != DatabaseBackupStatusSuccess {
			return DatabasePlanValidationFailed
		}
	}
	return DatabasePlanValidationPassed
}

func runnerStatusForRestoreStatus(status string) string {
	if status == DatabaseBackupStatusSuccess {
		return DatabaseRunnerJobStatusSuccess
	}
	return DatabaseRunnerJobStatusFailed
}

func buildPhysicalRestoreProofJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, result physicalRestoreRunnerResult, finalStatus, message string) string {
	proof := map[string]any{
		"restoreJobId":            job.ID,
		"restorePlanId":           plan.ID,
		"sourceInstanceId":        plan.SourceInstanceID,
		"targetInstanceId":        plan.TargetInstanceID,
		"restoreTargetType":       plan.RestoreTargetType,
		"restoreTargetValue":      plan.RestoreTargetValue,
		"runnerHostId":            job.RunnerHostID,
		"runnerJobId":             job.RunnerJobID,
		"baseBackupRecordId":      job.BackupRecordID,
		"selectedBackupRecordIds": plan.SelectedBackupRecordIDs,
		"selectedLogArchiveIds":   plan.SelectedLogArchiveIDs,
		"workDir":                 result.WorkDir,
		"preparedDatadir":         result.PreparedDatadir,
		"containerName":           result.ContainerName,
		"containerImage":          result.ContainerImage,
		"listenHost":              result.ListenHost,
		"listenPort":              result.ListenPort,
		"logPath":                 result.LogPath,
		"artifactUri":             result.ArtifactURI,
		"steps":                   result.Steps,
		"validationResults":       result.ValidationResults,
		"validationStatus":        result.ValidationStatus,
		"applyBinlogSummary":      result.ApplyBinlogSummary,
		"startedAt":               result.StartedAt,
		"finishedAt":              result.FinishedAt,
		"durationMs":              result.DurationMs,
		"operatorId":              job.OperatorID,
		"operatorName":            job.OperatorName,
		"finalStatus":             finalStatus,
		"message":                 message,
	}
	return marshalBackupPlanJSON(proof)
}

func buildBarmanRestoreProofJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, result barmanRestoreRunnerResult, finalStatus, message string) string {
	restoreStatus := DatabaseRestoreStatusFailed
	if finalStatus == DatabaseBackupStatusSuccess {
		restoreStatus = DatabaseRestoreStatusRestored
	}
	proof := map[string]any{
		"restoreJobId":       job.ID,
		"restorePlanId":      plan.ID,
		"sourceInstanceId":   plan.SourceInstanceID,
		"targetInstanceId":   plan.TargetInstanceID,
		"restoreTargetType":  plan.RestoreTargetType,
		"restoreTargetValue": plan.RestoreTargetValue,
		"targetTimelineId":   result.TargetTimelineID,
		"targetAction":       result.TargetAction,
		"getWal":             result.GetWAL,
		"runnerHostId":       job.RunnerHostID,
		"runnerJobId":        job.RunnerJobID,
		"baseBackupRecordId": job.BackupRecordID,
		"barmanServerName":   result.BarmanServerName,
		"backupId":           result.BackupID,
		"workDir":            result.WorkDir,
		"preparedDatadir":    result.PreparedDatadir,
		"containerName":      result.ContainerName,
		"containerImage":     result.ContainerImage,
		"listenHost":         result.ListenHost,
		"listenPort":         result.ListenPort,
		"logPath":            result.LogPath,
		"artifactUri":        result.ArtifactURI,
		"steps":              result.Steps,
		"validationResults":  result.ValidationResults,
		"validationStatus":   result.ValidationStatus,
		"recoverySummary":    result.RecoverySummary,
		"startedAt":          result.StartedAt,
		"finishedAt":         result.FinishedAt,
		"durationMs":         result.DurationMs,
		"operatorId":         job.OperatorID,
		"operatorName":       job.OperatorName,
		"finalStatus":        finalStatus,
		"restoreStatus":      restoreStatus,
		"message":            message,
	}
	return marshalBackupPlanJSON(proof)
}

func physicalRestoreRequestJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, host *DatabaseRunnerHost, validationChecks []restoreValidationCheck, cleanupOnFailure bool, operator QueryOperator) string {
	payload := map[string]any{
		"restorePlanId":    plan.ID,
		"restoreJobId":     job.ID,
		"runnerHostId":     host.ID,
		"runnerType":       host.RunnerType,
		"allowedCommand":   DatabaseRunnerAllowedCommandPhysicalRestore,
		"containerImage":   job.ContainerImage,
		"listenHost":       job.ListenHost,
		"listenPort":       job.ListenPort,
		"workDir":          job.WorkDir,
		"targetType":       job.RestoreTargetType,
		"targetValue":      job.RestoreTargetValue,
		"validationSql":    validationCheckSQLs(validationChecks),
		"validationChecks": validationChecks,
		"cleanupOnFailure": cleanupOnFailure,
		"operatorId":       operator.ID,
		"operatorName":     operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func barmanRestoreRequestJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, host *DatabaseRunnerHost, server *DatabaseBarmanServer, base *DatabaseBackupRecord, req *DatabaseRestorePlanRunRequest, operator QueryOperator) string {
	getWAL := true
	if req != nil && req.BarmanGetWAL != nil {
		getWAL = *req.BarmanGetWAL
	}
	startInstance := true
	if req != nil && req.PostgresStartInstance != nil {
		startInstance = *req.PostgresStartInstance
	}
	targetTimelineID := ""
	targetAction := "pause"
	cleanupOnFailure := false
	var validationSQL []string
	var validationAssertions []DatabaseRestoreValidationAssertion
	if req != nil {
		targetTimelineID = pgwal.NormalizeTimelineID(req.TargetTimelineID)
		targetAction = normalizeBarmanRestoreTargetAction(req.TargetAction)
		cleanupOnFailure = req.CleanupOnFailure
		validationSQL = req.ValidationSQL
		validationAssertions = req.ValidationAssertions
	}
	payload := map[string]any{
		"restorePlanId":        plan.ID,
		"restoreJobId":         job.ID,
		"runnerHostId":         host.ID,
		"runnerType":           host.RunnerType,
		"allowedCommand":       DatabaseRunnerAllowedCommandBarmanRestore,
		"barmanServerId":       server.ID,
		"barmanServerName":     server.BarmanServerName,
		"backupRecordId":       base.ID,
		"backupId":             base.ExternalBackupID,
		"workDir":              job.WorkDir,
		"destinationDir":       job.PreparedDatadir,
		"containerName":        job.ContainerName,
		"containerImage":       job.ContainerImage,
		"listenHost":           job.ListenHost,
		"listenPort":           job.ListenPort,
		"targetType":           job.RestoreTargetType,
		"targetValue":          job.RestoreTargetValue,
		"targetTimelineId":     targetTimelineID,
		"targetAction":         targetAction,
		"getWal":               getWAL,
		"startInstance":        startInstance,
		"validationSql":        validationSQL,
		"validationAssertions": validationAssertions,
		"cleanupOnFailure":     cleanupOnFailure,
		"operatorId":           operator.ID,
		"operatorName":         operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func isSafeRestoreContainerImage(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n;$`|&<>") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._/:@-+", r) {
			continue
		}
		return false
	}
	return true
}

func isSafeRestoreContainerName(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isSafeRestoreWorkDir(value string) bool {
	value = filepath.Clean(strings.TrimSpace(value))
	if value == "" || value == "." || value == "/" {
		return false
	}
	return strings.Contains(value, "/restore/job-")
}

func boolShellValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func firstNonNilTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
