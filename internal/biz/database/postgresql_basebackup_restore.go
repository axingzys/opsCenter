package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/database/pgwal"
)

type pgBaseBackupRestoreScriptInput struct {
	RestoreJobID        uint
	RestorePlanID       uint
	RunnerHostID        uint
	WorkRoot            string
	Base                physicalRestoreArtifact
	Logs                []physicalRestoreArtifact
	ContainerName       string
	ContainerImage      string
	ListenPort          int
	DatabaseName        string
	DBUsername          string
	DBPassword          string
	TargetType          string
	TargetValue         string
	TargetTimelineID    string
	TargetTimelineValue string
	TargetAction        string
	StartInstance       bool
	ValidationChecks    []restoreValidationCheck
	CleanupOnFailure    bool
}

type pgBaseBackupRestoreRunnerResult struct {
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
	TargetType         string                            `json:"targetType"`
	TargetValue        string                            `json:"targetValue"`
	TargetTimelineID   string                            `json:"targetTimelineId"`
	TargetAction       string                            `json:"targetAction"`
	StartInstance      bool                              `json:"startInstance"`
	PGVersion          string                            `json:"pgVersion"`
	ManifestChecksum   string                            `json:"manifestChecksum"`
	VerifyBackupStatus string                            `json:"verifyBackupStatus"`
	RecoverySummary    string                            `json:"recoverySummary"`
	Steps              []physicalRestoreStep             `json:"steps"`
	ValidationResults  []physicalRestoreValidationResult `json:"validationResults"`
	ValidationStatus   string                            `json:"validationStatus"`
	Stdout             string                            `json:"stdout"`
	Stderr             string                            `json:"stderr"`
	ExitCode           int                               `json:"exitCode"`
	StartedAt          string                            `json:"startedAt"`
	FinishedAt         string                            `json:"finishedAt"`
	DurationMs         int64                             `json:"durationMs"`
	Error              string                            `json:"error,omitempty"`
}

func (uc *UseCase) runPostgreSQLPgBaseBackupRestorePlan(ctx context.Context, plan *DatabaseRestorePlan, source *DatabaseInstance, base *DatabaseBackupRecord, req *DatabaseRestorePlanRunRequest, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if !isPostgreSQLPgBaseBackupBaseRecord(base) {
		return nil, fmt.Errorf("PostgreSQL P3.8.1/2 仅支持 pg_basebackup full backup 恢复")
	}
	target, err := normalizePostgreSQLRestoreTarget(plan.RestoreTargetType, plan.RestoreTargetValue, req.TargetTimelineID)
	if err != nil {
		return nil, err
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("P3.8.1/2 首版仅支持 SSH Runner 执行 pg_basebackup restore")
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
		return nil, fmt.Errorf("执行前 PostgreSQL pg_basebackup 恢复计划复检未通过: %s", plan.ErrorMessage)
	}
	if recheck.RunnerHostID > 0 && recheck.RunnerHostID != host.ID {
		return nil, fmt.Errorf("pg_basebackup artifact 属于 runnerHostId=%d，当前选择 Runner 为 %d", recheck.RunnerHostID, host.ID)
	}
	if _, err := restoreArtifactFromBackupRecord(base, host.ID, "base"); err != nil {
		return nil, err
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
	strategy := "pg_basebackup_restore_directory"
	if startInstance {
		strategy = "pg_basebackup_pitr_container"
	}
	job := &DatabaseRestoreJob{
		BackupRecordID:     base.ID,
		RestorePlanID:      plan.ID,
		RunnerHostID:       host.ID,
		SourceInstanceID:   plan.SourceInstanceID,
		TargetInstanceID:   plan.TargetInstanceID,
		RestoreMode:        DatabaseRestoreModeIsolatedRestore,
		RestoreStrategy:    strategy,
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
		ErrorMessage:       "pg_basebackup restore 任务已排队，等待 Runner 执行",
	}
	if err := uc.restoreJobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建恢复任务失败: %w", err)
	}
	job.WorkDir = filepath.Join(workRoot, "restore", fmt.Sprintf("job-%d", job.ID))
	job.PreparedDatadir = filepath.Join(job.WorkDir, "pgdata")
	job.LogPath = filepath.Join(job.WorkDir, "restore.log")
	if startInstance {
		job.ContainerName = fmt.Sprintf("opshub-pgbase-restore-%d", job.ID)
		job.ContainerImage = defaultPostgreSQLRestoreContainerImage(source, req.ContainerImage)
		job.ListenHost = "127.0.0.1"
		job.ListenPort = normalizeRestoreListenPort(req.ListenPort)
	}
	job.ArtifactURI = fmt.Sprintf("runner://runner-host-%d%s", host.ID, filepath.ToSlash(filepath.Join(job.WorkDir, "proof.json")))
	_ = uc.restoreJobRepo.Update(ctx, job)

	runnerJob := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypePgBaseBackupRestore,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: plan.SourceInstanceID,
		TargetInstanceID: plan.TargetInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandPgBaseBackupRestore,
		CommandSummary:   fmt.Sprintf("pg_basebackup restore %s", firstNonEmpty(base.FileName, base.ExternalBackupID)),
		WorkDir:          job.WorkDir,
		LogPath:          job.LogPath,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      pgBaseBackupRestoreRequestJSON(plan, job, host, base, req, validationChecks, operator),
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
	plan.ErrorMessage = "pg_basebackup restore 任务已下发 Runner"
	_ = uc.restorePlanRepo.Update(ctx, plan)

	go uc.executePgBaseBackupRestoreJob(context.Background(), plan.ID, job.ID, runnerJob.ID, validationChecks, req.CleanupOnFailure, pgwal.NormalizeTimelineID(firstNonEmpty(req.TargetTimelineID, recheck.TargetTimelineID)), normalizeBarmanRestoreTargetAction(req.TargetAction), startInstance)
	return uc.toRestoreJobVO(job, source.Name, "", "", host.Name), nil
}

func (uc *UseCase) executePgBaseBackupRestoreJob(ctx context.Context, planID, restoreJobID, runnerJobID uint, validationChecks []restoreValidationCheck, cleanupOnFailure bool, targetTimelineID, targetAction string, startInstance bool) {
	plan, planErr := uc.restorePlanRepo.GetByID(ctx, planID)
	restoreJob, restoreErr := uc.restoreJobRepo.GetByID(ctx, restoreJobID)
	runnerJob, runnerErr := uc.runnerJobRepo.GetByID(ctx, runnerJobID)
	if planErr != nil || restoreErr != nil || runnerErr != nil || plan == nil || restoreJob == nil || runnerJob == nil {
		return
	}
	started := time.Now()
	restoreJob.Status = DatabaseBackupStatusRunning
	restoreJob.StartedAt = &started
	restoreJob.ErrorMessage = "pg_basebackup restore Runner 执行中"
	runnerJob.Status = DatabaseRunnerJobStatusRunning
	runnerJob.StartedAt = &started
	runnerJob.HeartbeatAt = &started
	plan.RestoreStatus = DatabaseRestoreStatusRunning
	plan.StartedAt = &started
	_ = uc.restoreJobRepo.Update(ctx, restoreJob)
	_ = uc.runnerJobRepo.Update(ctx, runnerJob)
	_ = uc.restorePlanRepo.Update(ctx, plan)

	result, exitCode, runErr := uc.runPgBaseBackupRestoreRunnerScript(ctx, plan, restoreJob, validationChecks, cleanupOnFailure, targetTimelineID, targetAction, startInstance, started)
	finished := time.Now()
	status := DatabaseBackupStatusSuccess
	restoreStatus := DatabaseRestoreStatusVerified
	message := "pg_basebackup 已恢复到隔离 PostgreSQL 实例并通过校验"
	if runErr != nil {
		status = DatabaseBackupStatusFailed
		restoreStatus = DatabaseRestoreStatusFailed
		message = runErr.Error()
	} else if !startInstance {
		restoreStatus = DatabaseRestoreStatusRestored
		message = "pg_basebackup 已恢复到 Runner 隔离目录"
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
	proofJSON := buildPgBaseBackupRestoreProofJSON(plan, restoreJob, result, status, message)

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

func (uc *UseCase) runPgBaseBackupRestoreRunnerScript(ctx context.Context, plan *DatabaseRestorePlan, restoreJob *DatabaseRestoreJob, validationChecks []restoreValidationCheck, cleanupOnFailure bool, targetTimelineID, targetAction string, startInstance bool, started time.Time) (pgBaseBackupRestoreRunnerResult, int, error) {
	result := pgBaseBackupRestoreRunnerResult{
		RestorePlanID:    plan.ID,
		RestoreJobID:     restoreJob.ID,
		RunnerHostID:     restoreJob.RunnerHostID,
		WorkDir:          restoreJob.WorkDir,
		PreparedDatadir:  restoreJob.PreparedDatadir,
		ContainerName:    restoreJob.ContainerName,
		ContainerImage:   restoreJob.ContainerImage,
		ListenHost:       "127.0.0.1",
		ListenPort:       restoreJob.ListenPort,
		LogPath:          restoreJob.LogPath,
		ArtifactURI:      restoreJob.ArtifactURI,
		TargetType:       plan.RestoreTargetType,
		TargetValue:      plan.RestoreTargetValue,
		TargetTimelineID: targetTimelineID,
		TargetAction:     targetAction,
		StartInstance:    startInstance,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
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
	baseArtifact, err := restoreArtifactFromBackupRecord(base, restoreJob.RunnerHostID, "base")
	if err != nil {
		return result, 1, err
	}
	logs := []physicalRestoreArtifact{}
	if startInstance {
		logs, err = uc.restoreLogArtifacts(ctx, plan.SelectedLogArchiveIDs, restoreJob.RunnerHostID)
		if err != nil {
			return result, 1, err
		}
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
	targetTimelineValue, err := postgreSQLRecoveryTargetTimelineValue(targetTimelineID)
	if err != nil {
		return result, 1, err
	}
	targetAction = normalizeBarmanRestoreTargetAction(targetAction)
	input := pgBaseBackupRestoreScriptInput{
		RestoreJobID:        restoreJob.ID,
		RestorePlanID:       plan.ID,
		RunnerHostID:        restoreJob.RunnerHostID,
		WorkRoot:            filepath.Dir(filepath.Dir(restoreJob.WorkDir)),
		Base:                baseArtifact,
		Logs:                logs,
		ContainerName:       restoreJob.ContainerName,
		ContainerImage:      restoreJob.ContainerImage,
		ListenPort:          restoreJob.ListenPort,
		DatabaseName:        databaseName,
		DBUsername:          dbUsername,
		DBPassword:          dbPassword,
		TargetType:          plan.RestoreTargetType,
		TargetValue:         plan.RestoreTargetValue,
		TargetTimelineID:    targetTimelineID,
		TargetTimelineValue: targetTimelineValue,
		TargetAction:        targetAction,
		StartInstance:       startInstance,
		ValidationChecks:    validationChecks,
		CleanupOnFailure:    cleanupOnFailure,
	}
	script, err := buildPgBaseBackupRestoreScript(input)
	if err != nil {
		return result, 1, err
	}
	stdout, stderr, exitCode, runErr := executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	parsed := parsePgBaseBackupRestoreOutput(stdout)
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
	parsed.TargetType = firstNonEmpty(parsed.TargetType, result.TargetType)
	parsed.TargetValue = firstNonEmpty(parsed.TargetValue, result.TargetValue)
	parsed.TargetTimelineID = firstNonEmpty(parsed.TargetTimelineID, targetTimelineID)
	parsed.TargetAction = firstNonEmpty(parsed.TargetAction, targetAction)
	parsed.StartInstance = startInstance
	parsed.ValidationResults = attachRestoreValidationChecks(parsed.ValidationResults, validationChecks)
	if startInstance && (parsed.ValidationStatus == "" || parsed.ValidationStatus == DatabasePlanValidationPending) {
		parsed.ValidationStatus = validationStatusFromResults(parsed.ValidationResults)
	}
	if runErr != nil {
		return parsed, exitCode, fmt.Errorf("pg_basebackup restore Runner 执行失败: %s", trimText(firstNonEmpty(stderr, runErr.Error()), 1000))
	}
	return parsed, exitCode, nil
}

func buildPgBaseBackupRestoreScript(input pgBaseBackupRestoreScriptInput) (string, error) {
	if input.RestoreJobID == 0 || input.RunnerHostID == 0 {
		return "", fmt.Errorf("恢复任务上下文不完整")
	}
	if err := validateRestoreArtifactForScript(input.Base); err != nil {
		return "", err
	}
	for _, item := range input.Logs {
		if err := validateRestoreArtifactForScript(item); err != nil {
			return "", err
		}
	}
	targetType := strings.ToLower(strings.TrimSpace(input.TargetType))
	if targetType != "time" && targetType != "lsn" {
		return "", fmt.Errorf("pg_basebackup restore 仅支持 targetType=time/lsn")
	}
	if strings.TrimSpace(input.TargetValue) == "" {
		return "", fmt.Errorf("恢复目标不能为空")
	}
	targetAction := normalizeBarmanRestoreTargetAction(input.TargetAction)
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
		"ARTIFACT_DIR=\"$WORK_DIR/artifacts\"",
		"WAL_DIR=\"$WORK_DIR/wal\"",
		"PGDATA_DIR=\"$WORK_DIR/pgdata\"",
		"LOG_FILE=\"$WORK_DIR/restore.log\"",
		"PROOF_FILE=\"$WORK_DIR/proof.json\"",
		"CONTAINER_NAME=" + shellSingleQuote(input.ContainerName),
		"CONTAINER_IMAGE=" + shellSingleQuote(input.ContainerImage),
		fmt.Sprintf("LISTEN_PORT=%d", input.ListenPort),
		"PGDATABASE_NAME=" + shellSingleQuote(input.DatabaseName),
		"PGUSER_NAME=" + shellSingleQuote(input.DBUsername),
		"PGPASSWORD_VALUE=" + shellSingleQuote(input.DBPassword),
		"TARGET_TYPE=" + shellSingleQuote(targetType),
		"TARGET_VALUE=" + shellSingleQuote(input.TargetValue),
		"TARGET_TLI=" + shellSingleQuote(input.TargetTimelineID),
		"TARGET_TLI_VALUE=" + shellSingleQuote(input.TargetTimelineValue),
		"TARGET_ACTION=" + shellSingleQuote(targetAction),
		"START_INSTANCE=" + boolShellValue(input.StartInstance),
		"CLEANUP_ON_FAILURE=" + boolShellValue(input.CleanupOnFailure),
		fmt.Sprintf("WAL_COUNT=%d", len(input.Logs)),
		`mkdir -p "$ARTIFACT_DIR" "$WAL_DIR" "$WORK_DIR"`,
		`: > "$LOG_FILE"`,
		`step() { printf 'OPSHUB_RESTORE_STEP=%s|%s|%s\n' "$1" "$2" "$(date '+%Y-%m-%d %H:%M:%S')"; printf '%s %s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG_FILE"; }`,
		`cleanup_failure() { code="$?"; if [ "$code" != "0" ] && [ "$CLEANUP_ON_FAILURE" = "1" ]; then DOCKER_BIN="$(command -v docker || true)"; if [ -n "$DOCKER_BIN" ] && [ -n "$CONTAINER_NAME" ]; then "$DOCKER_BIN" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fi; rm -rf "$PGDATA_DIR" >> "$LOG_FILE" 2>&1 || true; printf '%s cleanup_on_failure pgdata_removed\n' "$(date '+%Y-%m-%d %H:%M:%S')" >> "$LOG_FILE" || true; fi; exit "$code"; }`,
		`trap cleanup_failure EXIT`,
		`fail_step() { step "$1" "failed"; echo "$2" >> "$LOG_FILE"; exit 1; }`,
		`copy_artifact() { label="$1"; src="$2"; dest="$3"; expected_sha="$4"; expected_size="$5"; step "fetch_${label}" "running"; if [ ! -f "$src" ]; then fail_step "fetch_${label}" "artifact not found: $src"; fi; cp "$src" "$dest"; actual_size="$(wc -c < "$dest" | tr -d ' ')"; if [ "$expected_size" != "0" ] && [ "$actual_size" != "$expected_size" ]; then fail_step "fetch_${label}" "size mismatch: $actual_size != $expected_size"; fi; if [ -n "$expected_sha" ]; then actual_sha="$(sha256sum "$dest" | awk '{print $1}')"; if [ "$actual_sha" != "$expected_sha" ]; then fail_step "fetch_${label}" "sha256 mismatch"; fi; fi; step "fetch_${label}" "success"; }`,
		`run_pg_validation() { idx="$1"; sql="$2"; expected_rows_enabled="$3"; expected_rows="$4"; expected_contains_enabled="$5"; expected_contains="$6"; expected_scalar_enabled="$7"; expected_scalar="$8"; out="$WORK_DIR/validation-$idx.out"; step "validation_${idx}" "running"; set +e; PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -F "$(printf '\t')" -v ON_ERROR_STOP=1 -c "$sql" > "$out" 2>&1; code="$?"; set -e; sha="$(sha256sum "$out" | awk '{print $1}')"; preview="$(head -c 1200 "$out" | base64 | tr -d '\n')"; actual_rows="$(grep -v '^$' "$out" | wc -l | tr -d ' ')"; actual_scalar="$(grep -v '^$' "$out" | head -n 1 | awk -F '\t' '{print $1}')"; status="success"; assertion_status="not_configured"; assertion_message=""; if [ "$code" != "0" ]; then status="failed"; assertion_status="failed"; assertion_message="SQL execution failed"; else if [ "$expected_rows_enabled" = "1" ] || [ "$expected_contains_enabled" = "1" ] || [ "$expected_scalar_enabled" = "1" ]; then assertion_status="passed"; fi; if [ "$expected_rows_enabled" = "1" ] && [ "$actual_rows" != "$expected_rows" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedRows=$expected_rows actualRows=$actual_rows; "; fi; if [ "$expected_contains_enabled" = "1" ] && ! grep -F -- "$expected_contains" "$out" >/dev/null 2>&1; then assertion_status="failed"; assertion_message="${assertion_message}expectedContains not found; "; fi; if [ "$expected_scalar_enabled" = "1" ] && [ "$actual_scalar" != "$expected_scalar" ]; then assertion_status="failed"; assertion_message="${assertion_message}expectedScalar=$expected_scalar actualScalar=$actual_scalar; "; fi; if [ "$assertion_status" = "failed" ]; then status="failed"; fi; fi; actual_scalar_b64="$(printf '%s' "$actual_scalar" | base64 | tr -d '\n')"; assertion_message_b64="$(printf '%s' "$assertion_message" | base64 | tr -d '\n')"; if [ "$status" = "success" ]; then step "validation_${idx}" "success"; else step "validation_${idx}" "failed"; validation_failed=1; fi; printf 'OPSHUB_RESTORE_VALIDATION=%s|%s|%s|%s|%s|%s|%s|%s\n' "$idx" "$status" "$sha" "$preview" "$assertion_status" "$actual_rows" "$actual_scalar_b64" "$assertion_message_b64"; }`,
		`printf 'OPSHUB_WORK_DIR=%s\n' "$WORK_DIR"`,
		`printf 'OPSHUB_PREPARED_DATADIR=%s\n' "$PGDATA_DIR"`,
		`printf 'OPSHUB_CONTAINER_NAME=%s\n' "$CONTAINER_NAME"`,
		`printf 'OPSHUB_CONTAINER_IMAGE=%s\n' "$CONTAINER_IMAGE"`,
		`printf 'OPSHUB_LISTEN_HOST=127.0.0.1\n'`,
		`printf 'OPSHUB_LISTEN_PORT=%s\n' "$LISTEN_PORT"`,
		`printf 'OPSHUB_LOG_PATH=%s\n' "$LOG_FILE"`,
		`printf 'OPSHUB_PROOF_PATH=%s\n' "$PROOF_FILE"`,
		`printf 'OPSHUB_TARGET_TYPE=%s\n' "$TARGET_TYPE"`,
		`printf 'OPSHUB_TARGET_VALUE=%s\n' "$TARGET_VALUE"`,
		`printf 'OPSHUB_TARGET_TLI=%s\n' "$TARGET_TLI"`,
		`printf 'OPSHUB_TARGET_ACTION=%s\n' "$TARGET_ACTION"`,
		`step "prepare_restore_directory" "running"`,
		`case "$PGDATA_DIR" in "$WORK_ROOT"/restore/job-"$RESTORE_JOB_ID"/pgdata) ;; *) fail_step "prepare_restore_directory" "unsafe pgdata destination: $PGDATA_DIR";; esac`,
		`rm -rf "$PGDATA_DIR"`,
		`mkdir -p "$PGDATA_DIR"`,
		`step "prepare_restore_directory" "success"`,
	}
	lines = append(lines, buildRestoreCopyArtifactLine("base", input.Base, `$ARTIFACT_DIR/base.pg_basebackup.tar.gz`))
	lines = append(lines,
		`step "extract_base_backup" "running"`,
		`tar -xzf "$ARTIFACT_DIR/base.pg_basebackup.tar.gz" -C "$PGDATA_DIR" >> "$LOG_FILE" 2>&1 || fail_step "extract_base_backup" "extract pg_basebackup artifact failed"`,
		`step "extract_base_backup" "success"`,
		`step "verify_pgdata" "running"`,
		`test -f "$PGDATA_DIR/PG_VERSION" || fail_step "verify_pgdata" "PG_VERSION not found in restored PGDATA"`,
		`test -d "$PGDATA_DIR/global" || fail_step "verify_pgdata" "global directory not found in restored PGDATA"`,
		`test -d "$PGDATA_DIR/base" || fail_step "verify_pgdata" "base directory not found in restored PGDATA"`,
		`if [ ! -d "$PGDATA_DIR/pg_wal" ] && [ ! -d "$PGDATA_DIR/pg_xlog" ]; then fail_step "verify_pgdata" "pg_wal/pg_xlog directory not found in restored PGDATA"; fi`,
		`pg_version="$(cat "$PGDATA_DIR/PG_VERSION" | tr -d '\r\n')"`,
		`manifest_sha=""; if [ -f "$PGDATA_DIR/backup_manifest" ]; then manifest_sha="$(sha256sum "$PGDATA_DIR/backup_manifest" | awk '{print $1}')"; fi`,
		`printf 'OPSHUB_PG_VERSION=%s\n' "$pg_version"`,
		`printf 'OPSHUB_BACKUP_MANIFEST_CHECKSUM=%s\n' "$manifest_sha"`,
		`PG_VERIFYBACKUP="$(command -v pg_verifybackup || true)"`,
		`verify_status="not_available"; if [ -n "$PG_VERIFYBACKUP" ] && [ -f "$PGDATA_DIR/backup_manifest" ]; then if "$PG_VERIFYBACKUP" "$PGDATA_DIR" >> "$LOG_FILE" 2>&1; then verify_status="success"; else printf 'OPSHUB_VERIFYBACKUP_STATUS=failed\n'; fail_step "verify_pgdata" "pg_verifybackup failed"; fi; fi`,
		`printf 'OPSHUB_VERIFYBACKUP_STATUS=%s\n' "$verify_status"`,
		`step "verify_pgdata" "success"`,
	)
	for idx, logArtifact := range input.Logs {
		label := fmt.Sprintf("wal_%d", idx+1)
		dest := fmt.Sprintf(`$WAL_DIR/%s`, logArtifact.FileName)
		lines = append(lines, buildRestoreCopyArtifactLine(label, logArtifact, dest))
	}
	lines = append(lines,
		`validation_failed=0`,
		`if [ "$START_INSTANCE" = "1" ]; then`,
		`  step "configure_recovery" "running"`,
		`  rm -f "$PGDATA_DIR/postmaster.pid" "$PGDATA_DIR/standby.signal"`,
		`  if [ "$WAL_COUNT" != "0" ]; then`,
		`    escaped_wal_dir="$(printf '%s' "$WAL_DIR" | sed "s/'/''/g")"`,
		`    escaped_target="$(printf '%s' "$TARGET_VALUE" | sed "s/'/''/g")"`,
		`    touch "$PGDATA_DIR/recovery.signal"`,
		`    { printf '\n# OpsHub pg_basebackup PITR restore\n'; printf "restore_command = 'cp ''%s/%%f'' ''%%p'''\n" "$escaped_wal_dir"; if [ "$TARGET_TYPE" = "time" ]; then printf "recovery_target_time = '%s'\n" "$escaped_target"; else printf "recovery_target_lsn = '%s'\n" "$escaped_target"; fi; if [ -n "$TARGET_TLI_VALUE" ]; then printf "recovery_target_timeline = '%s'\n" "$TARGET_TLI_VALUE"; fi; printf "recovery_target_action = '%s'\n" "$TARGET_ACTION"; } >> "$PGDATA_DIR/postgresql.auto.conf"`,
		`    printf 'OPSHUB_RECOVERY_CONFIG=%s\n' "archive_recovery"`,
		`  else`,
		`    printf 'OPSHUB_RECOVERY_CONFIG=%s\n' "no_external_wal_required"`,
		`  fi`,
		`  step "configure_recovery" "success"`,
		`  step "start_isolated_postgres" "running"`,
		`  DOCKER_BIN="$(command -v docker || true)"`,
		`  PSQL_BIN="$(command -v psql || true)"`,
		`  if [ -z "$DOCKER_BIN" ]; then fail_step "start_isolated_postgres" "docker not found"; fi`,
		`  if [ -z "$PSQL_BIN" ]; then fail_step "start_isolated_postgres" "psql not found"; fi`,
		`  "$DOCKER_BIN" rm -f "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true`,
		`  chown -R 999:999 "$PGDATA_DIR" >> "$LOG_FILE" 2>&1 || true`,
		`  "$DOCKER_BIN" run -d --name "$CONTAINER_NAME" -p "127.0.0.1:$LISTEN_PORT:5432" -v "$PGDATA_DIR:/var/lib/postgresql/data" "$CONTAINER_IMAGE" -c listen_addresses='*' -c port=5432 >> "$LOG_FILE" 2>&1 || fail_step "start_isolated_postgres" "docker run failed"`,
		`  ready=0; for i in $(seq 1 120); do if PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -v ON_ERROR_STOP=1 -c "SELECT 1" >/dev/null 2>&1; then ready=1; break; fi; sleep 2; done`,
		`  if [ "$ready" != "1" ]; then "$DOCKER_BIN" logs "$CONTAINER_NAME" >> "$LOG_FILE" 2>&1 || true; fail_step "start_isolated_postgres" "isolated postgres not ready"; fi`,
		`  recovery_summary="$(PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -F '|' -c "SELECT pg_is_in_recovery(), pg_last_wal_replay_lsn(), pg_last_xact_replay_timestamp()" 2>/dev/null || true)"`,
		`  printf 'OPSHUB_RECOVERY_SUMMARY=%s\n' "$recovery_summary"`,
		`  if [ "$TARGET_TYPE" = "lsn" ] && [ "$WAL_COUNT" != "0" ]; then reached="$(PGPASSWORD="$PGPASSWORD_VALUE" "$PSQL_BIN" -h 127.0.0.1 -p "$LISTEN_PORT" -U "$PGUSER_NAME" -d "$PGDATABASE_NAME" -At -c "SELECT pg_wal_lsn_diff(pg_last_wal_replay_lsn(), '$TARGET_VALUE') >= 0" 2>/dev/null || true)"; if [ "$reached" = "f" ]; then fail_step "start_isolated_postgres" "replay LSN did not reach target"; fi; fi`,
		`  step "start_isolated_postgres" "success"`,
	)
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
		`printf '{"restoreJobId":%s,"restorePlanId":%s,"workDir":"%s","preparedDatadir":"%s","containerName":"%s","containerImage":"%s","listenHost":"127.0.0.1","listenPort":%s,"targetType":"%s","targetValue":"%s","targetTimelineId":"%s","targetAction":"%s","startInstance":"%s","generatedAt":"%s"}\n' "$RESTORE_JOB_ID" "$RESTORE_PLAN_ID" "$WORK_DIR" "$PGDATA_DIR" "$CONTAINER_NAME" "$CONTAINER_IMAGE" "$LISTEN_PORT" "$TARGET_TYPE" "$TARGET_VALUE" "$TARGET_TLI" "$TARGET_ACTION" "$START_INSTANCE" "$GENERATED_AT" > "$PROOF_FILE"`,
		`printf 'OPSHUB_ARTIFACT_URI=runner://runner-host-`+strconv.Itoa(int(input.RunnerHostID))+`%s\n' "$PROOF_FILE"`,
		`step "generate_proof" "success"`,
	)
	return strings.Join(lines, "\n"), nil
}

func parsePgBaseBackupRestoreOutput(stdout string) pgBaseBackupRestoreRunnerResult {
	result := pgBaseBackupRestoreRunnerResult{}
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
	result.TargetType = kv["OPSHUB_TARGET_TYPE"]
	result.TargetValue = kv["OPSHUB_TARGET_VALUE"]
	result.TargetTimelineID = kv["OPSHUB_TARGET_TLI"]
	result.TargetAction = kv["OPSHUB_TARGET_ACTION"]
	result.PGVersion = kv["OPSHUB_PG_VERSION"]
	result.ManifestChecksum = kv["OPSHUB_BACKUP_MANIFEST_CHECKSUM"]
	result.VerifyBackupStatus = kv["OPSHUB_VERIFYBACKUP_STATUS"]
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

func buildPgBaseBackupRestoreProofJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, result pgBaseBackupRestoreRunnerResult, finalStatus, message string) string {
	restoreStatus := DatabaseRestoreStatusFailed
	if finalStatus == DatabaseBackupStatusSuccess {
		restoreStatus = DatabaseRestoreStatusRestored
		if result.StartInstance && result.ValidationStatus == DatabasePlanValidationPassed {
			restoreStatus = DatabaseRestoreStatusVerified
		}
	}
	proof := map[string]any{
		"restoreJobId":          job.ID,
		"restorePlanId":         plan.ID,
		"sourceInstanceId":      plan.SourceInstanceID,
		"targetInstanceId":      plan.TargetInstanceID,
		"backupEngine":          BackupEnginePgBaseBackup,
		"restoreTargetType":     plan.RestoreTargetType,
		"restoreTargetValue":    plan.RestoreTargetValue,
		"targetTimelineId":      result.TargetTimelineID,
		"targetAction":          result.TargetAction,
		"runnerHostId":          job.RunnerHostID,
		"runnerJobId":           job.RunnerJobID,
		"baseBackupRecordId":    job.BackupRecordID,
		"selectedLogArchiveIds": plan.SelectedLogArchiveIDs,
		"workDir":               result.WorkDir,
		"preparedDatadir":       result.PreparedDatadir,
		"containerName":         result.ContainerName,
		"containerImage":        result.ContainerImage,
		"listenHost":            result.ListenHost,
		"listenPort":            result.ListenPort,
		"logPath":               result.LogPath,
		"artifactUri":           result.ArtifactURI,
		"startInstance":         result.StartInstance,
		"pgVersion":             result.PGVersion,
		"manifestChecksum":      result.ManifestChecksum,
		"verifyBackupStatus":    result.VerifyBackupStatus,
		"recoverySummary":       result.RecoverySummary,
		"steps":                 result.Steps,
		"validationResults":     result.ValidationResults,
		"validationStatus":      result.ValidationStatus,
		"startedAt":             result.StartedAt,
		"finishedAt":            result.FinishedAt,
		"durationMs":            result.DurationMs,
		"operatorId":            job.OperatorID,
		"operatorName":          job.OperatorName,
		"finalStatus":           finalStatus,
		"restoreStatus":         restoreStatus,
		"message":               message,
	}
	return marshalBackupPlanJSON(proof)
}

func pgBaseBackupRestoreRequestJSON(plan *DatabaseRestorePlan, job *DatabaseRestoreJob, host *DatabaseRunnerHost, base *DatabaseBackupRecord, req *DatabaseRestorePlanRunRequest, validationChecks []restoreValidationCheck, operator QueryOperator) string {
	startInstance := true
	targetTimelineID := extractTargetTimelineFromPlan(plan)
	targetAction := "pause"
	cleanupOnFailure := false
	var validationSQL []string
	var validationAssertions []DatabaseRestoreValidationAssertion
	if req != nil {
		if req.PostgresStartInstance != nil {
			startInstance = *req.PostgresStartInstance
		}
		if strings.TrimSpace(req.TargetTimelineID) != "" {
			targetTimelineID = pgwal.NormalizeTimelineID(req.TargetTimelineID)
		}
		targetAction = normalizeBarmanRestoreTargetAction(req.TargetAction)
		cleanupOnFailure = req.CleanupOnFailure
		validationSQL = req.ValidationSQL
		validationAssertions = req.ValidationAssertions
	}
	payload := map[string]any{
		"restorePlanId":         plan.ID,
		"restoreJobId":          job.ID,
		"runnerHostId":          host.ID,
		"runnerType":            host.RunnerType,
		"allowedCommand":        DatabaseRunnerAllowedCommandPgBaseBackupRestore,
		"backupRecordId":        base.ID,
		"backupEngine":          BackupEnginePgBaseBackup,
		"workDir":               job.WorkDir,
		"destinationDir":        job.PreparedDatadir,
		"containerName":         job.ContainerName,
		"containerImage":        job.ContainerImage,
		"listenHost":            job.ListenHost,
		"listenPort":            job.ListenPort,
		"targetType":            job.RestoreTargetType,
		"targetValue":           job.RestoreTargetValue,
		"targetTimelineId":      targetTimelineID,
		"targetAction":          targetAction,
		"startInstance":         startInstance,
		"selectedLogArchiveIds": plan.SelectedLogArchiveIDs,
		"validationSql":         validationSQL,
		"validationAssertions":  validationAssertions,
		"validationChecks":      validationChecks,
		"cleanupOnFailure":      cleanupOnFailure,
		"operatorId":            operator.ID,
		"operatorName":          operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func isPostgreSQLPgBaseBackupBaseRecord(item *DatabaseBackupRecord) bool {
	return item != nil &&
		normalizeBackupMethod(item.BackupMethod) == DatabaseBackupMethodPhysical &&
		normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelFull &&
		normalizePostgreSQLPhysicalBackupEngine(item.BackupEngine) == BackupEnginePgBaseBackup &&
		(strings.TrimSpace(item.StorageURI) != "" || strings.TrimSpace(item.FilePath) != "")
}

func postgreSQLRecoveryTargetTimelineValue(timelineID string) (string, error) {
	timelineID = pgwal.NormalizeTimelineID(timelineID)
	if timelineID == "" {
		return "", nil
	}
	timelineNo, err := pgwal.TimelineNumber(timelineID)
	if err != nil {
		return "", fmt.Errorf("target timeline 不合法")
	}
	return strconv.FormatUint(timelineNo, 10), nil
}
