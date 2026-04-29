package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/database/pgwal"
)

const (
	BackupEnginePgBaseBackup = "pg_basebackup"
	BackupEngineWALG         = "walg"
	BackupEnginePgBackRest   = "pgbackrest"
)

type pgBaseBackupScopeConfig struct {
	RunnerHostID uint     `json:"runnerHostId"`
	ExtraArgs    []string `json:"extraArgs"`
}

type pgBaseBackupRunnerResult struct {
	BackupRecordID         uint                  `json:"backupRecordId"`
	BackupTaskID           uint                  `json:"backupTaskId,omitempty"`
	RunnerHostID           uint                  `json:"runnerHostId"`
	RunnerID               string                `json:"runnerId"`
	WorkDir                string                `json:"workDir"`
	FilePath               string                `json:"filePath"`
	FileName               string                `json:"fileName"`
	StorageURI             string                `json:"storageUri"`
	FileSize               int64                 `json:"fileSize"`
	ChecksumSHA256         string                `json:"checksumSha256"`
	ToolVersion            string                `json:"toolVersion"`
	PGVersion              string                `json:"pgVersion"`
	PGSystemIdentifier     string                `json:"pgSystemIdentifier"`
	TimelineID             string                `json:"timelineId"`
	StartLSN               string                `json:"startLsn"`
	EndLSN                 string                `json:"endLsn"`
	WALStart               string                `json:"walStart"`
	WALEnd                 string                `json:"walEnd"`
	BackupManifestChecksum string                `json:"backupManifestChecksum"`
	Steps                  []physicalRestoreStep `json:"steps"`
	Stdout                 string                `json:"stdout"`
	Stderr                 string                `json:"stderr"`
	ExitCode               int                   `json:"exitCode"`
	StartedAt              string                `json:"startedAt"`
	FinishedAt             string                `json:"finishedAt"`
	DurationMs             int64                 `json:"durationMs"`
	Error                  string                `json:"error,omitempty"`
}

func parsePgBaseBackupScopeConfig(value string) (*pgBaseBackupScopeConfig, error) {
	result := &pgBaseBackupScopeConfig{}
	value = strings.TrimSpace(value)
	if value == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(value), result); err != nil {
		return nil, fmt.Errorf("pg_basebackup 范围配置 JSON 格式错误: %w", err)
	}
	cleaned := make([]string, 0, len(result.ExtraArgs))
	for _, arg := range result.ExtraArgs {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		lower := strings.ToLower(arg)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") {
			return nil, fmt.Errorf("pg_basebackup extraArgs 不允许包含密码或密钥")
		}
		cleaned = append(cleaned, arg)
	}
	result.ExtraArgs = cleaned
	return result, nil
}

func isPostgreSQLPgBaseBackupTask(task *DatabaseBackupTask, instance *DatabaseInstance) bool {
	if task == nil || instance == nil {
		return false
	}
	return normalizeDBType(instance.DBType) == DBTypePostgreSQL &&
		normalizeBackupMethod(task.BackupMethod) == DatabaseBackupMethodPhysical &&
		strings.TrimSpace(task.BackupEngine) == BackupEnginePgBaseBackup
}

func (uc *UseCase) runPgBaseBackupTaskIfNeeded(ctx context.Context, id uint, operator QueryOperator, triggerType string) (*DatabaseBackupRunVO, bool, error) {
	if uc.backupTaskRepo == nil || uc.instanceRepo == nil {
		return nil, false, nil
	}
	task, err := uc.backupTaskRepo.GetByID(ctx, id)
	if err != nil || task == nil {
		return nil, false, nil
	}
	instance, err := uc.instanceRepo.GetByID(ctx, task.InstanceID)
	if err != nil || instance == nil || !isPostgreSQLPgBaseBackupTask(task, instance) {
		return nil, false, nil
	}
	job, record, err := uc.enqueuePgBaseBackup(ctx, task, instance, operator, triggerType)
	if err != nil {
		return nil, true, err
	}
	message := "pg_basebackup 物理备份任务已进入 Runner 队列"
	if job != nil && job.ID > 0 {
		message = fmt.Sprintf("%s，Runner Job #%d", message, job.ID)
	}
	return &DatabaseBackupRunVO{
		TaskID:       task.ID,
		TaskName:     task.Name,
		RecordID:     record.ID,
		InstanceID:   task.InstanceID,
		InstanceName: instance.Name,
		Status:       DatabaseBackupStatusQueued,
		StatusText:   BackupStatusText(DatabaseBackupStatusQueued),
		FileName:     record.FileName,
		Message:      message,
		TriggeredAt:  time.Now().Format("2006-01-02 15:04:05"),
	}, true, nil
}

func (uc *UseCase) enqueuePgBaseBackup(ctx context.Context, task *DatabaseBackupTask, instance *DatabaseInstance, operator QueryOperator, triggerType string) (*DatabaseRunnerJob, *DatabaseBackupRecord, error) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil || uc.credentialResolver == nil {
		return nil, nil, fmt.Errorf("pg_basebackup Runner 仓库未配置")
	}
	scope, err := parsePgBaseBackupScopeConfig(task.ScopeConfig)
	if err != nil {
		return nil, nil, err
	}
	if scope.RunnerHostID == 0 {
		return nil, nil, fmt.Errorf("pg_basebackup 需要在范围配置中提供 {\"runnerHostId\": 1}")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, scope.RunnerHostID)
	if err != nil || host == nil {
		return nil, nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, nil, err
	}
	credential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 PostgreSQL 凭据失败: %w", err)
	}
	if strings.TrimSpace(credential.Username) == "" || credential.Password == "" {
		return nil, nil, fmt.Errorf("pg_basebackup 需要带密码的 PostgreSQL 凭据")
	}
	audit, err := uc.startBackupAudit(ctx, instance, task, "cluster", operator)
	if err != nil {
		return nil, nil, fmt.Errorf("创建备份审计失败: %w", err)
	}
	if err := uc.acquireBackupTaskRun(task.ID, task.InstanceID); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, err.Error())
		return nil, nil, err
	}
	releaseOnError := true
	defer func() {
		if releaseOnError {
			uc.releaseBackupTaskRun(task.ID, task.InstanceID)
		}
	}()
	started := time.Now()
	expiresAt := started.AddDate(0, 0, normalizeBackupRetentionDays(task.RetentionDays, 7))
	fileName := fmt.Sprintf("%s-pg-basebackup-%s.tar.gz", safeBackupName(instance.Name), started.Format("20060102150405"))
	record := &DatabaseBackupRecord{
		TaskID:             task.ID,
		InstanceID:         task.InstanceID,
		TriggerType:        normalizeBackupTriggerType(triggerType),
		BackupType:         DatabaseBackupTypePhysical,
		ChainID:            trimText(fmt.Sprintf("pg_basebackup-%d", task.ID), 64),
		BackupMethod:       DatabaseBackupMethodPhysical,
		BackupLevel:        DatabaseBackupLevelFull,
		BackupEngine:       BackupEnginePgBaseBackup,
		ExternalBackupID:   trimText(fmt.Sprintf("pgbasebackup-%d", started.Unix()), 120),
		ExternalServerName: trimText(runnerIDForHost(host), 120),
		BackupScope:        "cluster",
		ToolName:           BackupEnginePgBaseBackup,
		SourceInstanceID:   task.InstanceID,
		SourceRole:         normalizeSourceRole(task.SourceRole),
		StorageType:        DatabaseBackupStorageExternal,
		StorageURI:         trimText(fmt.Sprintf("runner://runner-host-%d/pending/%d", host.ID, started.Unix()), 1000),
		Status:             DatabaseBackupStatusQueued,
		FileName:           trimText(fileName, 255),
		Compression:        "gzip",
		ExpiresAt:          &expiresAt,
		VerifyStatus:       DatabaseBackupVerifyStatusPending,
		StartedAt:          &started,
		LastHeartbeatAt:    &started,
		RecoverableFrom:    &started,
		RecoverableUntil:   &started,
		ErrorMessage:       "pg_basebackup 物理备份任务已进入 Runner 队列",
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建备份记录失败: "+err.Error())
		return nil, nil, err
	}
	task.LastRunAt = &started
	task.LastStatus = DatabaseBackupStatusQueued
	task.LastMessage = trimText("pg_basebackup 物理备份任务已进入 Runner 队列", 500)
	task.RestoreCapability = DatabaseRestoreCapabilityPhysicalRestore
	uc.applyBackupTaskNextRunAt(task, started)
	if err := uc.backupTaskRepo.Update(ctx, task); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("更新备份任务状态失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypePgBaseBackup,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: task.InstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandPgBaseBackup,
		CommandSummary:   fmt.Sprintf("pg_basebackup %s", instance.Name),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      pgBaseBackupRequestJSON(task, record, host, instance, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("创建 pg_basebackup Runner Job 失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		uc.finishBackupTask(ctx, task, time.Now(), DatabaseBackupStatusFailed, record.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, nil, err
	}
	releaseOnError = false
	go func() {
		defer uc.releaseBackupTaskRun(task.ID, task.InstanceID)
		uc.executePgBaseBackupJob(context.Background(), task.ID, record.ID, job.ID, audit, scope, credential)
	}()
	return job, record, nil
}

func (uc *UseCase) executePgBaseBackupJob(ctx context.Context, taskID, recordID, jobID uint, audit *DatabaseQueryAudit, scope *pgBaseBackupScopeConfig, credential *ConnectionCredential) {
	task, _ := uc.backupTaskRepo.GetByID(ctx, taskID)
	record, _ := uc.backupRecordRepo.GetByID(ctx, recordID)
	job, _ := uc.runnerJobRepo.GetByID(ctx, jobID)
	if task == nil || record == nil || job == nil {
		return
	}
	instance, err := uc.instanceRepo.GetByID(ctx, task.InstanceID)
	if err != nil || instance == nil {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, job.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)
	uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusRunning, "pg_basebackup 物理备份执行中")
	uc.markBackupTaskStatus(ctx, task, DatabaseBackupStatusRunning, "pg_basebackup 物理备份执行中")
	stdout, stderr, exitCode, runErr := uc.runPgBaseBackupCommand(ctx, task, record, instance, host, scope, credential)
	finished := time.Now()
	result := parsePgBaseBackupRunnerResult(record, host, stdout, stderr, exitCode, runErr, started, finished)
	result.BackupTaskID = taskID
	uc.applyPgBaseBackupResult(ctx, task, record, job, audit, result, runErr)
}

func (uc *UseCase) runPgBaseBackupCommand(ctx context.Context, task *DatabaseBackupTask, record *DatabaseBackupRecord, instance *DatabaseInstance, host *DatabaseRunnerHost, scope *pgBaseBackupScopeConfig, credential *ConnectionCredential) (string, string, int, error) {
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script := buildPgBaseBackupScript(task, record, instance, host, scope, credential)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func buildPgBaseBackupScript(task *DatabaseBackupTask, record *DatabaseBackupRecord, instance *DatabaseInstance, host *DatabaseRunnerHost, scope *pgBaseBackupScopeConfig, credential *ConnectionCredential) string {
	workRoot := firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)
	sslMode := "disable"
	if instance.TLSEnabled {
		sslMode = "require"
	}
	lines := []string{
		"set -eu",
		"WORK_ROOT=" + shellSingleQuote(workRoot),
		fmt.Sprintf("BACKUP_RECORD_ID=%d", record.ID),
		fmt.Sprintf("BACKUP_TASK_ID=%d", task.ID),
		fmt.Sprintf("RUNNER_HOST_ID=%d", host.ID),
		"PGHOST_VALUE=" + shellSingleQuote(instance.Host),
		fmt.Sprintf("PGPORT_VALUE=%d", instance.Port),
		"PGUSER_VALUE=" + shellSingleQuote(credential.Username),
		"PGPASSWORD_VALUE=" + shellSingleQuote(credential.Password),
		"PGSSLMODE_VALUE=" + shellSingleQuote(sslMode),
		"FILE_NAME=" + shellSingleQuote(record.FileName),
		"WORK_DIR=\"$WORK_ROOT/pg-basebackup/record-$BACKUP_RECORD_ID\"",
		"BASE_DIR=\"$WORK_DIR/base\"",
		"ARTIFACT=\"$WORK_DIR/$FILE_NAME\"",
		"LOG_FILE=\"$WORK_DIR/pg_basebackup.log\"",
		`mkdir -p "$WORK_DIR"`,
		`: > "$LOG_FILE"`,
		`step() { printf 'OPSHUB_RESTORE_STEP=%s|%s|%s\n' "$1" "$2" "$(date '+%Y-%m-%d %H:%M:%S')"; printf '%s %s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG_FILE"; }`,
		`fail_step() { step "$1" "failed"; echo "$2" >> "$LOG_FILE"; exit 1; }`,
		`PG_BASEBACKUP="$(command -v pg_basebackup || true)"`,
		`if [ -z "$PG_BASEBACKUP" ]; then fail_step "tool_check" "pg_basebackup not found"; fi`,
		`PG_CONTROLDATA="$(command -v pg_controldata || true)"`,
		`printf 'OPSHUB_WORK_DIR=%s\n' "$WORK_DIR"`,
		`printf 'OPSHUB_FILE_PATH=%s\n' "$ARTIFACT"`,
		`printf 'OPSHUB_FILE_NAME=%s\n' "$FILE_NAME"`,
		`printf 'OPSHUB_LOG_PATH=%s\n' "$LOG_FILE"`,
		`printf 'OPSHUB_TOOL_VERSION=%s\n' "$("$PG_BASEBACKUP" --version 2>/dev/null | head -n 1)"`,
		`step "pg_basebackup" "running"`,
		`rm -rf "$BASE_DIR" "$ARTIFACT"`,
		`mkdir -p "$BASE_DIR"`,
	}
	pgBaseBackupCommand := `PGPASSWORD="$PGPASSWORD_VALUE" PGSSLMODE="$PGSSLMODE_VALUE" "$PG_BASEBACKUP" -h "$PGHOST_VALUE" -p "$PGPORT_VALUE" -U "$PGUSER_VALUE" -D "$BASE_DIR" -Fp -X stream --checkpoint=fast --progress`
	for _, arg := range scope.ExtraArgs {
		pgBaseBackupCommand += " " + shellSingleQuote(arg)
	}
	lines = append(lines,
		pgBaseBackupCommand+` >> "$LOG_FILE" 2>&1 || fail_step "pg_basebackup" "pg_basebackup failed"`,
		`step "pg_basebackup" "success"`,
		`step "package_backup" "running"`,
		`tar -czf "$ARTIFACT" -C "$BASE_DIR" . >> "$LOG_FILE" 2>&1 || fail_step "package_backup" "package basebackup failed"`,
		`size="$(wc -c < "$ARTIFACT" | tr -d ' ')"`,
		`sha="$(sha256sum "$ARTIFACT" | awk '{print $1}')"`,
		`manifest_sha=""; if [ -f "$BASE_DIR/backup_manifest" ]; then manifest_sha="$(sha256sum "$BASE_DIR/backup_manifest" | awk '{print $1}')"; fi`,
		`pg_version=""; if [ -f "$BASE_DIR/PG_VERSION" ]; then pg_version="$(cat "$BASE_DIR/PG_VERSION" | tr -d '\r\n')"; fi`,
		`system_id=""; timeline_id=""; redo_lsn=""; redo_wal=""; if [ -n "$PG_CONTROLDATA" ]; then "$PG_CONTROLDATA" "$BASE_DIR" > "$WORK_DIR/pg_controldata.txt" 2>> "$LOG_FILE" || true; system_id="$(awk -F: '/Database system identifier/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"; timeline_id="$(awk -F: '/Latest checkpoint.s TimeLineID/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"; redo_lsn="$(awk -F: '/Latest checkpoint.s REDO location/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"; redo_wal="$(awk -F: '/Latest checkpoint.s REDO WAL file/ {gsub(/^[ \t]+/,"",$2); print $2; exit}' "$WORK_DIR/pg_controldata.txt" 2>/dev/null || true)"; fi`,
		`printf 'OPSHUB_FILE_SIZE=%s\n' "$size"`,
		`printf 'OPSHUB_CHECKSUM_SHA256=%s\n' "$sha"`,
		`printf 'OPSHUB_STORAGE_URI=runner://runner-host-`+strconv.Itoa(int(host.ID))+`%s\n' "$ARTIFACT"`,
		`printf 'OPSHUB_PG_VERSION=%s\n' "$pg_version"`,
		`printf 'OPSHUB_PG_SYSTEM_IDENTIFIER=%s\n' "$system_id"`,
		`printf 'OPSHUB_TIMELINE_ID=%s\n' "$timeline_id"`,
		`printf 'OPSHUB_END_LSN=%s\n' "$redo_lsn"`,
		`printf 'OPSHUB_WAL_END=%s\n' "$redo_wal"`,
		`printf 'OPSHUB_BACKUP_MANIFEST_CHECKSUM=%s\n' "$manifest_sha"`,
		`step "package_backup" "success"`,
	)
	return strings.Join(lines, "\n")
}

func parsePgBaseBackupRunnerResult(record *DatabaseBackupRecord, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) pgBaseBackupRunnerResult {
	kv := parseRunnerKeyValueOutput(stdout)
	fileSize, _ := strconv.ParseInt(kv["OPSHUB_FILE_SIZE"], 10, 64)
	result := pgBaseBackupRunnerResult{
		BackupRecordID:         record.ID,
		RunnerHostID:           host.ID,
		RunnerID:               runnerIDForHost(host),
		WorkDir:                kv["OPSHUB_WORK_DIR"],
		FilePath:               kv["OPSHUB_FILE_PATH"],
		FileName:               firstNonEmpty(kv["OPSHUB_FILE_NAME"], record.FileName),
		StorageURI:             kv["OPSHUB_STORAGE_URI"],
		FileSize:               fileSize,
		ChecksumSHA256:         kv["OPSHUB_CHECKSUM_SHA256"],
		ToolVersion:            kv["OPSHUB_TOOL_VERSION"],
		PGVersion:              kv["OPSHUB_PG_VERSION"],
		PGSystemIdentifier:     kv["OPSHUB_PG_SYSTEM_IDENTIFIER"],
		TimelineID:             pgwal.NormalizeTimelineID(kv["OPSHUB_TIMELINE_ID"]),
		EndLSN:                 kv["OPSHUB_END_LSN"],
		WALEnd:                 kv["OPSHUB_WAL_END"],
		BackupManifestChecksum: kv["OPSHUB_BACKUP_MANIFEST_CHECKSUM"],
		Stdout:                 trimText(stdout, maxRunnerOutputLength),
		Stderr:                 trimText(stderr, maxRunnerOutputLength),
		ExitCode:               exitCode,
		StartedAt:              started.Format("2006-01-02 15:04:05"),
		FinishedAt:             finished.Format("2006-01-02 15:04:05"),
		DurationMs:             finished.Sub(started).Milliseconds(),
	}
	for _, line := range strings.Split(stdout, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "OPSHUB_RESTORE_STEP="); ok {
			parts := strings.SplitN(value, "|", 3)
			if len(parts) == 3 {
				result.Steps = append(result.Steps, physicalRestoreStep{Name: parts[0], Status: parts[1], OccurredAt: parts[2]})
			}
		}
	}
	if runErr != nil {
		result.Error = runErr.Error()
	}
	return result
}

func (uc *UseCase) applyPgBaseBackupResult(ctx context.Context, task *DatabaseBackupTask, record *DatabaseBackupRecord, job *DatabaseRunnerJob, audit *DatabaseQueryAudit, result pgBaseBackupRunnerResult, runErr error) {
	now := time.Now()
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	durationMs := result.DurationMs
	if runErr != nil || result.ExitCode != 0 {
		message := trimText(firstNonEmpty(result.Error, result.Stderr, "pg_basebackup 执行失败"), 1000)
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, message)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, message)
	} else if result.FileSize <= 0 || strings.TrimSpace(result.ChecksumSHA256) == "" || strings.TrimSpace(result.StorageURI) == "" {
		message := "pg_basebackup 已执行，但未能生成完整 artifact metadata"
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, message)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, message)
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		record.FileName = trimText(result.FileName, 255)
		record.FilePath = trimText(result.FilePath, 500)
		record.StorageURI = trimText(result.StorageURI, 1000)
		record.FileSize = result.FileSize
		record.ChecksumSHA256 = trimText(result.ChecksumSHA256, 64)
		record.ToolName = BackupEnginePgBaseBackup
		record.ToolVersion = trimText(result.ToolVersion, 120)
		record.PGSystemIdentifier = trimText(result.PGSystemIdentifier, 120)
		record.TimelineID = trimText(result.TimelineID, 60)
		record.EndLSN = trimText(result.EndLSN, 120)
		record.WALEnd = trimText(result.WALEnd, 255)
		record.BackupManifestChecksum = trimText(result.BackupManifestChecksum, 128)
		record.ManifestJSON = buildPgBaseBackupManifest(record, result)
		record.PrepareStatus = "ready"
		started := now.Add(-time.Duration(durationMs) * time.Millisecond)
		if record.StartedAt != nil {
			started = *record.StartedAt
		}
		uc.finishBackupRecordSuccess(ctx, record, started, now, durationMs, result.FileSize, result.ChecksumSHA256, "pg_basebackup 物理备份完成")
		if task != nil {
			uc.finishBackupTask(ctx, task, now, DatabaseBackupStatusSuccess, "pg_basebackup 物理备份完成")
		}
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
}

func buildPgBaseBackupManifest(record *DatabaseBackupRecord, result pgBaseBackupRunnerResult) string {
	payload := map[string]any{
		"engine":                 BackupEnginePgBaseBackup,
		"method":                 DatabaseBackupMethodPhysical,
		"level":                  record.BackupLevel,
		"backupScope":            "cluster",
		"storageUri":             result.StorageURI,
		"checksumSha256":         result.ChecksumSHA256,
		"pgVersion":              result.PGVersion,
		"pgSystemIdentifier":     result.PGSystemIdentifier,
		"timelineId":             result.TimelineID,
		"endLsn":                 result.EndLSN,
		"walEnd":                 result.WALEnd,
		"backupManifestChecksum": result.BackupManifestChecksum,
		"generatedAt":            time.Now().Format("2006-01-02 15:04:05"),
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func pgBaseBackupRequestJSON(task *DatabaseBackupTask, record *DatabaseBackupRecord, host *DatabaseRunnerHost, instance *DatabaseInstance, operator QueryOperator) string {
	payload := map[string]any{
		"backupTaskId":   task.ID,
		"backupRecordId": record.ID,
		"instanceId":     instance.ID,
		"runnerHostId":   host.ID,
		"runnerType":     host.RunnerType,
		"allowedCommand": DatabaseRunnerAllowedCommandPgBaseBackup,
		"backupEngine":   BackupEnginePgBaseBackup,
		"backupScope":    "cluster",
		"backupLevel":    task.BackupLevel,
		"operatorId":     operator.ID,
		"operatorName":   operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func safeBackupName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "postgresql"
	}
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-_.")
	if result == "" {
		return "postgresql"
	}
	return result
}
