package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type mysqlPhysicalBackupRunnerResult struct {
	PolicyID             uint                  `json:"policyId"`
	BackupRecordID       uint                  `json:"backupRecordId"`
	RunnerJobID          uint                  `json:"runnerJobId"`
	RunnerHostID         uint                  `json:"runnerHostId"`
	RunnerID             string                `json:"runnerId"`
	BackupLevel          string                `json:"backupLevel"`
	WorkDir              string                `json:"workDir"`
	FilePath             string                `json:"filePath"`
	FileName             string                `json:"fileName"`
	StorageURI           string                `json:"storageUri"`
	FileSize             int64                 `json:"fileSize"`
	ChecksumSHA256       string                `json:"checksumSha256"`
	ToolName             string                `json:"toolName"`
	ToolVersion          string                `json:"toolVersion"`
	CheckpointBackupType string                `json:"checkpointBackupType"`
	CheckpointFromLSN    string                `json:"checkpointFromLsn"`
	CheckpointToLSN      string                `json:"checkpointToLsn"`
	CheckpointLastLSN    string                `json:"checkpointLastLsn"`
	BackupBinlogFile     string                `json:"backupBinlogFile"`
	BackupBinlogPos      int64                 `json:"backupBinlogPos"`
	BackupGTIDSet        string                `json:"backupGtidSet"`
	ParentArtifactPath   string                `json:"parentArtifactPath,omitempty"`
	ParentRecordID       uint                  `json:"parentRecordId,omitempty"`
	BaseRecordID         uint                  `json:"baseRecordId,omitempty"`
	Steps                []physicalRestoreStep `json:"steps"`
	Stdout               string                `json:"stdout"`
	Stderr               string                `json:"stderr"`
	ExitCode             int                   `json:"exitCode"`
	StartedAt            string                `json:"startedAt"`
	FinishedAt           string                `json:"finishedAt"`
	DurationMs           int64                 `json:"durationMs"`
	Error                string                `json:"error,omitempty"`
}

func (uc *UseCase) ListBackupPolicies(ctx context.Context, req *DatabaseBackupPolicyListRequest) ([]*DatabaseBackupPolicyVO, int64, error) {
	if uc.backupPolicyConfigRepo == nil {
		return nil, 0, fmt.Errorf("备份策略仓库未配置")
	}
	normalizeBackupPolicyListRequest(req)
	items, total, err := uc.backupPolicyConfigRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseBackupPolicyVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toBackupPolicyVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) GetBackupPolicyInstanceID(ctx context.Context, id uint) (uint, error) {
	if id == 0 {
		return 0, fmt.Errorf("备份策略ID不能为空")
	}
	item, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return 0, err
	}
	return item.InstanceID, nil
}

func (uc *UseCase) CreateBackupPolicy(ctx context.Context, req *DatabaseBackupPolicyRequest) (*DatabaseBackupPolicyVO, error) {
	if err := uc.validateBackupPolicyRequest(ctx, req); err != nil {
		return nil, err
	}
	instance, _ := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	item := &DatabaseBackupPolicyConfig{}
	uc.applyBackupPolicyRequest(item, instance, req)
	if err := uc.backupPolicyConfigRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	uc.refreshBackupPolicyNextRunAt(ctx, item, time.Now())
	return uc.toBackupPolicyVO(ctx, item), nil
}

func (uc *UseCase) UpdateBackupPolicy(ctx context.Context, id uint, req *DatabaseBackupPolicyRequest) (*DatabaseBackupPolicyVO, error) {
	if err := uc.validateBackupPolicyRequest(ctx, req); err != nil {
		return nil, err
	}
	item, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	instance, _ := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	uc.applyBackupPolicyRequest(item, instance, req)
	uc.applyBackupPolicyNextRunAt(item, time.Now())
	if err := uc.backupPolicyConfigRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBackupPolicyVO(ctx, item), nil
}

func (uc *UseCase) DeleteBackupPolicy(ctx context.Context, id uint) error {
	if _, err := uc.getBackupPolicy(ctx, id); err != nil {
		return err
	}
	return uc.backupPolicyConfigRepo.Delete(ctx, id)
}

func (uc *UseCase) RunBackupPolicyFull(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupPolicyRunVO, error) {
	return uc.runBackupPolicy(ctx, id, DatabaseBackupLevelFull, operator, DatabaseBackupTriggerManual)
}

func (uc *UseCase) RunBackupPolicyIncremental(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupPolicyRunVO, error) {
	return uc.runBackupPolicy(ctx, id, DatabaseBackupLevelIncremental, operator, DatabaseBackupTriggerManual)
}

func (uc *UseCase) RunScheduledBackupPolicy(ctx context.Context, id uint, backupLevel string) (*DatabaseBackupPolicyRunVO, error) {
	return uc.runBackupPolicy(ctx, id, normalizeBackupLevel(backupLevel), scheduledBackupOperator(), DatabaseBackupTriggerSchedule)
}

func (uc *UseCase) GetBackupPolicyChain(ctx context.Context, id uint) (*DatabaseBackupChainStateVO, error) {
	if _, err := uc.getBackupPolicy(ctx, id); err != nil {
		return nil, err
	}
	state, err := uc.loadBackupChainState(ctx, id)
	if err != nil {
		return nil, err
	}
	return toBackupChainStateVO(state), nil
}

func (uc *UseCase) ListEnabledBackupPolicies(ctx context.Context) ([]*DatabaseBackupPolicyConfig, error) {
	if uc.backupPolicyConfigRepo == nil {
		return nil, fmt.Errorf("备份策略仓库未配置")
	}
	return uc.backupPolicyConfigRepo.ListEnabled(ctx)
}

func (uc *UseCase) runBackupPolicy(ctx context.Context, id uint, backupLevel string, operator QueryOperator, triggerType string) (*DatabaseBackupPolicyRunVO, error) {
	if uc.backupPolicyConfigRepo == nil || uc.backupChainStateRepo == nil || uc.backupRecordRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("备份策略 Runner 仓库未配置")
	}
	policy, err := uc.getBackupPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled || policy.Status == DatabaseBackupPolicyStatusDisabled {
		return nil, fmt.Errorf("备份策略已禁用")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, policy.InstanceID)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(instance.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	sourceInstance := instance
	if policy.SourceInstanceID > 0 && policy.SourceInstanceID != policy.InstanceID {
		sourceInstance, err = uc.instanceRepo.GetByID(ctx, policy.SourceInstanceID)
		if err != nil || sourceInstance == nil {
			return nil, fmt.Errorf("备份源实例不存在")
		}
		if strings.TrimSpace(sourceInstance.Status) != DatabaseInstanceStatusEnabled {
			return nil, fmt.Errorf("备份源实例已禁用")
		}
		sourceDBType := normalizeDBType(sourceInstance.DBType)
		if sourceDBType != DBTypeMySQL && sourceDBType != DBTypeMariaDB {
			return nil, fmt.Errorf("备份源实例必须是 MySQL/MariaDB")
		}
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, policy.RunnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
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
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, sourceInstance.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("解析数据库凭据失败: %w", err)
	}
	if strings.TrimSpace(credential.Username) == "" || credential.Password == "" {
		return nil, fmt.Errorf("MySQL/MariaDB 物理备份需要带密码的数据库凭据")
	}
	level := normalizeBackupLevel(backupLevel)
	if level != DatabaseBackupLevelFull && level != DatabaseBackupLevelIncremental {
		return nil, fmt.Errorf("备份策略第一版仅支持全量和增量")
	}
	var parent *DatabaseBackupRecord
	if level == DatabaseBackupLevelIncremental {
		validation, err := uc.validateBackupPolicyChainInternal(ctx, policy, host, true)
		if err != nil {
			return nil, err
		}
		if validation == nil || validation.Status != DatabaseBackupChainStatusComplete || validation.LatestRecord == nil || validation.BaseRecord == nil {
			reasons := []string{"增量备份链路校验未通过"}
			if validation != nil && len(validation.BlockingReasons) > 0 {
				reasons = validation.BlockingReasons
			}
			return nil, fmt.Errorf("%s", strings.Join(reasons, "；"))
		}
		parent = validation.LatestRecord
		if err := validatePolicyIncrementalParent(policy, host, parent); err != nil {
			return nil, err
		}
	}
	auditTask := backupPolicyAuditTask(policy, level)
	audit, err := uc.startBackupAudit(ctx, instance, auditTask, "instance", operator)
	if err != nil {
		return nil, fmt.Errorf("创建备份审计失败: %w", err)
	}
	started := time.Now()
	expiresAt := started.AddDate(0, 0, policyRetentionDays(policy))
	fileName := fmt.Sprintf("%s-%s-%s.physical.tar.gz", safeBackupName(instance.Name), level, started.Format("20060102150405"))
	chainID := fmt.Sprintf("mysql-policy-%d-%d", policy.ID, started.Unix())
	baseRecordID := uint(0)
	parentRecordID := uint(0)
	if level == DatabaseBackupLevelIncremental {
		chainID = parent.ChainID
		baseRecordID = parent.BaseRecordID
		if baseRecordID == 0 {
			baseRecordID = parent.ID
		}
		parentRecordID = parent.ID
	}
	record := &DatabaseBackupRecord{
		PolicyID:         policy.ID,
		InstanceID:       policy.InstanceID,
		TriggerType:      normalizeBackupTriggerType(triggerType),
		BackupType:       DatabaseBackupTypePhysical,
		ChainID:          trimText(chainID, 64),
		BaseRecordID:     baseRecordID,
		ParentRecordID:   parentRecordID,
		BackupMethod:     DatabaseBackupMethodPhysical,
		BackupLevel:      level,
		BackupEngine:     policy.BackupEngine,
		BackupScope:      "instance",
		ToolName:         mysqlPhysicalBackupToolName(policy.BackupEngine, policy.Engine),
		SourceInstanceID: sourceInstance.ID,
		SourceRole:       normalizeSourceRole(policy.SourceRole),
		StorageProfileID: policy.StorageProfileID,
		StorageType:      DatabaseBackupStorageExternal,
		StorageURI:       trimText(fmt.Sprintf("runner://runner-host-%d/pending/%d", host.ID, started.Unix()), 1000),
		BackupOrigin:     backupOriginForLevel(level),
		ArtifactState:    DatabaseBackupArtifactStateRemote,
		Status:           DatabaseBackupStatusQueued,
		FileName:         trimText(fileName, 255),
		Compression:      "gzip",
		ExpiresAt:        &expiresAt,
		VerifyStatus:     DatabaseBackupVerifyStatusPending,
		StartedAt:        &started,
		LastHeartbeatAt:  &started,
		RecoverableFrom:  &started,
		RecoverableUntil: &started,
		PrepareStatus:    "pending_prepare",
		ErrorMessage:     "MySQL/MariaDB 物理备份策略任务已进入 Runner 队列",
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建备份记录失败: "+err.Error())
		return nil, err
	}
	if level == DatabaseBackupLevelFull {
		record.BaseRecordID = record.ID
		_ = uc.backupRecordRepo.Update(ctx, record)
	}
	policy.LastRunAt = &started
	policy.LastStatus = DatabaseBackupStatusQueued
	policy.LastMessage = "MySQL/MariaDB 物理备份策略任务已进入 Runner 队列"
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
		JobType:          DatabaseRunnerJobTypePhysicalBackup,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: sourceInstance.ID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandMySQLPhysicalBackup,
		CommandSummary:   fmt.Sprintf("%s %s %s", policy.BackupEngine, instance.Name, BackupLevelText(level)),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      mysqlPhysicalBackupRequestJSON(policy, record, host, instance, sourceInstance, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("创建 MySQL/MariaDB 物理备份 Runner Job 失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		uc.finishBackupPolicy(ctx, policy, time.Now(), DatabaseBackupStatusFailed, record.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, err
	}
	releaseOnError = false
	go func() {
		defer uc.releaseBackupPolicyRun(policy.ID, policy.InstanceID)
		uc.executeMySQLPhysicalBackupPolicyJob(context.Background(), policy.ID, record.ID, job.ID, parentRecordID, audit, credential)
	}()
	return &DatabaseBackupPolicyRunVO{
		PolicyID:     policy.ID,
		PolicyName:   policy.Name,
		RecordID:     record.ID,
		RunnerJobID:  job.ID,
		InstanceID:   policy.InstanceID,
		InstanceName: instance.Name,
		BackupLevel:  level,
		Status:       DatabaseBackupStatusQueued,
		StatusText:   BackupStatusText(DatabaseBackupStatusQueued),
		FileName:     record.FileName,
		Message:      fmt.Sprintf("MySQL/MariaDB %s备份已进入 Runner 队列，Runner Job #%d", BackupLevelText(level), job.ID),
		TriggeredAt:  started.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) executeMySQLPhysicalBackupPolicyJob(ctx context.Context, policyID, recordID, jobID, parentRecordID uint, audit *DatabaseQueryAudit, credential *ConnectionCredential) {
	policy, _ := uc.backupPolicyConfigRepo.GetByID(ctx, policyID)
	record, _ := uc.backupRecordRepo.GetByID(ctx, recordID)
	job, _ := uc.runnerJobRepo.GetByID(ctx, jobID)
	if policy == nil || record == nil || job == nil {
		return
	}
	instance, err := uc.instanceRepo.GetByID(ctx, policy.InstanceID)
	if err != nil || instance == nil {
		return
	}
	sourceInstance := instance
	if policy.SourceInstanceID > 0 && policy.SourceInstanceID != policy.InstanceID {
		if item, err := uc.instanceRepo.GetByID(ctx, policy.SourceInstanceID); err == nil && item != nil {
			sourceInstance = item
		}
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, job.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	var parent *DatabaseBackupRecord
	if parentRecordID > 0 {
		parent, _ = uc.backupRecordRepo.GetByID(ctx, parentRecordID)
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)
	uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusRunning, "MySQL/MariaDB 物理备份执行中")
	uc.markBackupPolicyStatus(ctx, policy, DatabaseBackupStatusRunning, "MySQL/MariaDB 物理备份执行中")
	stdout, stderr, exitCode, runErr := uc.runMySQLPhysicalBackupPolicyCommand(ctx, policy, record, parent, sourceInstance, host, credential)
	finished := time.Now()
	result := parseMySQLPhysicalBackupRunnerResult(policy, record, parent, host, stdout, stderr, exitCode, runErr, started, finished)
	result.RunnerJobID = jobID
	uc.applyMySQLPhysicalBackupPolicyResult(ctx, policy, record, job, audit, result, runErr)
}

func (uc *UseCase) runMySQLPhysicalBackupPolicyCommand(ctx context.Context, policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, parent *DatabaseBackupRecord, instance *DatabaseInstance, host *DatabaseRunnerHost, credential *ConnectionCredential) (string, string, int, error) {
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	parentPath := ""
	if parent != nil {
		parentPath, err = resolveRunnerReadableArtifactPath(parent.StorageURI, parent.FilePath, host.ID)
		if err != nil {
			return "", "", 1, err
		}
	}
	script := buildMySQLPhysicalBackupPolicyScript(policy, record, parentPath, instance, host, credential)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func buildMySQLPhysicalBackupPolicyScript(policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, parentArtifactPath string, instance *DatabaseInstance, host *DatabaseRunnerHost, credential *ConnectionCredential) string {
	workRoot := firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)
	toolName := mysqlPhysicalBackupToolName(policy.BackupEngine, policy.Engine)
	lines := []string{
		"set -eu",
		"WORK_ROOT=" + shellSingleQuote(workRoot),
		fmt.Sprintf("POLICY_ID=%d", policy.ID),
		fmt.Sprintf("BACKUP_RECORD_ID=%d", record.ID),
		fmt.Sprintf("RUNNER_HOST_ID=%d", host.ID),
		"BACKUP_LEVEL=" + shellSingleQuote(record.BackupLevel),
		"TOOL_NAME=" + shellSingleQuote(toolName),
		"DB_HOST_VALUE=" + shellSingleQuote(instance.Host),
		fmt.Sprintf("DB_PORT_VALUE=%d", instance.Port),
		"DB_USER_VALUE=" + shellSingleQuote(credential.Username),
		"DB_PASSWORD_VALUE=" + shellSingleQuote(credential.Password),
		"FILE_NAME=" + shellSingleQuote(record.FileName),
		"PARENT_ARTIFACT=" + shellSingleQuote(parentArtifactPath),
		"WORK_DIR=\"$WORK_ROOT/mysql-physical/policy-$POLICY_ID/record-$BACKUP_RECORD_ID\"",
		"TARGET_DIR=\"$WORK_DIR/backup\"",
		"PARENT_DIR=\"$WORK_DIR/parent\"",
		"ARTIFACT=\"$WORK_DIR/$FILE_NAME\"",
		"LOG_FILE=\"$WORK_DIR/mysql_physical_backup.log\"",
		"MYSQL_CNF=\"$WORK_DIR/mysql-client.cnf\"",
		`mkdir -p "$WORK_DIR"`,
		`: > "$LOG_FILE"`,
		`step() { printf 'OPSHUB_RESTORE_STEP=%s|%s|%s\n' "$1" "$2" "$(date '+%Y-%m-%d %H:%M:%S')"; printf '%s %s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG_FILE"; }`,
		`fail_step() { step "$1" "failed"; echo "$2" >> "$LOG_FILE"; exit 1; }`,
		`TOOL_PATH="$(command -v "$TOOL_NAME" || true)"`,
		`if [ -z "$TOOL_PATH" ]; then fail_step "tool_check" "$TOOL_NAME not found"; fi`,
		`printf 'OPSHUB_WORK_DIR=%s\n' "$WORK_DIR"`,
		`printf 'OPSHUB_FILE_PATH=%s\n' "$ARTIFACT"`,
		`printf 'OPSHUB_FILE_NAME=%s\n' "$FILE_NAME"`,
		`printf 'OPSHUB_LOG_PATH=%s\n' "$LOG_FILE"`,
		`printf 'OPSHUB_TOOL_NAME=%s\n' "$TOOL_NAME"`,
		`printf 'OPSHUB_TOOL_VERSION=%s\n' "$("$TOOL_PATH" --version 2>/dev/null | head -n 1)"`,
		`rm -rf "$TARGET_DIR" "$PARENT_DIR" "$ARTIFACT"`,
		`mkdir -p "$TARGET_DIR"`,
		`{ printf '[client]\n'; printf 'user=%s\n' "$DB_USER_VALUE"; printf 'password=%s\n' "$DB_PASSWORD_VALUE"; printf 'host=%s\n' "$DB_HOST_VALUE"; printf 'port=%s\n' "$DB_PORT_VALUE"; } > "$MYSQL_CNF"`,
		`chmod 600 "$MYSQL_CNF"`,
		`trap 'rm -f "$MYSQL_CNF"' EXIT`,
		`step "physical_backup" "running"`,
		`if [ "$BACKUP_LEVEL" = "incremental" ]; then`,
		`  if [ -z "$PARENT_ARTIFACT" ] || [ ! -f "$PARENT_ARTIFACT" ]; then fail_step "prepare_parent" "parent artifact not found"; fi`,
		`  step "prepare_parent" "running"`,
		`  mkdir -p "$PARENT_DIR"`,
		`  tar -xzf "$PARENT_ARTIFACT" -C "$PARENT_DIR" >> "$LOG_FILE" 2>&1 || fail_step "prepare_parent" "unpack parent artifact failed"`,
		`  if [ ! -f "$PARENT_DIR/xtrabackup_checkpoints" ]; then fail_step "prepare_parent" "parent xtrabackup_checkpoints not found"; fi`,
		`  step "prepare_parent" "success"`,
		`  "$TOOL_PATH" --defaults-extra-file="$MYSQL_CNF" --backup --target-dir="$TARGET_DIR" --incremental-basedir="$PARENT_DIR" >> "$LOG_FILE" 2>&1 || fail_step "physical_backup" "incremental physical backup failed"`,
		`else`,
		`  "$TOOL_PATH" --defaults-extra-file="$MYSQL_CNF" --backup --target-dir="$TARGET_DIR" >> "$LOG_FILE" 2>&1 || fail_step "physical_backup" "full physical backup failed"`,
		`fi`,
		`step "physical_backup" "success"`,
		`step "package_backup" "running"`,
		`tar -czf "$ARTIFACT" -C "$TARGET_DIR" . >> "$LOG_FILE" 2>&1 || fail_step "package_backup" "package physical backup failed"`,
		`size="$(wc -c < "$ARTIFACT" | tr -d ' ')"`,
		`sha="$(sha256sum "$ARTIFACT" | awk '{print $1}')"`,
		`checkpoint_value() { awk -F= -v key="$1" '{gsub(/^[ \t]+|[ \t]+$/,"",$1); if ($1==key) {gsub(/^[ \t]+|[ \t]+$/,"",$2); print $2; exit}}' "$TARGET_DIR/xtrabackup_checkpoints" 2>/dev/null || true; }`,
		`checkpoint_backup_type="$(checkpoint_value backup_type)"`,
		`checkpoint_from_lsn="$(checkpoint_value from_lsn)"`,
		`checkpoint_to_lsn="$(checkpoint_value to_lsn)"`,
		`checkpoint_last_lsn="$(checkpoint_value last_lsn)"`,
		`binlog_info="$TARGET_DIR/xtrabackup_binlog_info"; if [ ! -f "$binlog_info" ]; then binlog_info="$TARGET_DIR/mariadb_backup_binlog_info"; fi`,
		`binlog_file=""; binlog_pos="0"; binlog_gtid=""; if [ -f "$binlog_info" ]; then binlog_file="$(awk 'NR==1 {print $1}' "$binlog_info")"; binlog_pos="$(awk 'NR==1 {print $2}' "$binlog_info")"; binlog_gtid="$(awk 'NR==1 {$1=\"\"; $2=\"\"; sub(/^[ \t]+/,\"\"); print}' "$binlog_info")"; fi`,
		`printf 'OPSHUB_FILE_SIZE=%s\n' "$size"`,
		`printf 'OPSHUB_CHECKSUM_SHA256=%s\n' "$sha"`,
		`printf 'OPSHUB_STORAGE_URI=runner://runner-host-` + strconv.Itoa(int(host.ID)) + `%s\n' "$ARTIFACT"`,
		`printf 'OPSHUB_CHECKPOINT_BACKUP_TYPE=%s\n' "$checkpoint_backup_type"`,
		`printf 'OPSHUB_CHECKPOINT_FROM_LSN=%s\n' "$checkpoint_from_lsn"`,
		`printf 'OPSHUB_CHECKPOINT_TO_LSN=%s\n' "$checkpoint_to_lsn"`,
		`printf 'OPSHUB_CHECKPOINT_LAST_LSN=%s\n' "$checkpoint_last_lsn"`,
		`printf 'OPSHUB_BACKUP_BINLOG_FILE=%s\n' "$binlog_file"`,
		`printf 'OPSHUB_BACKUP_BINLOG_POS=%s\n' "$binlog_pos"`,
		`printf 'OPSHUB_BACKUP_GTID_SET=%s\n' "$binlog_gtid"`,
		`printf 'OPSHUB_PARENT_ARTIFACT_PATH=%s\n' "$PARENT_ARTIFACT"`,
		`step "package_backup" "success"`,
	}
	return strings.Join(lines, "\n")
}

func parseMySQLPhysicalBackupRunnerResult(policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, parent *DatabaseBackupRecord, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) mysqlPhysicalBackupRunnerResult {
	kv := parseRunnerKeyValueOutput(stdout)
	fileSize, _ := strconv.ParseInt(kv["OPSHUB_FILE_SIZE"], 10, 64)
	binlogPos, _ := strconv.ParseInt(kv["OPSHUB_BACKUP_BINLOG_POS"], 10, 64)
	result := mysqlPhysicalBackupRunnerResult{
		PolicyID:             policy.ID,
		BackupRecordID:       record.ID,
		RunnerHostID:         host.ID,
		RunnerID:             runnerIDForHost(host),
		BackupLevel:          record.BackupLevel,
		WorkDir:              kv["OPSHUB_WORK_DIR"],
		FilePath:             kv["OPSHUB_FILE_PATH"],
		FileName:             firstNonEmpty(kv["OPSHUB_FILE_NAME"], record.FileName),
		StorageURI:           kv["OPSHUB_STORAGE_URI"],
		FileSize:             fileSize,
		ChecksumSHA256:       kv["OPSHUB_CHECKSUM_SHA256"],
		ToolName:             firstNonEmpty(kv["OPSHUB_TOOL_NAME"], mysqlPhysicalBackupToolName(policy.BackupEngine, policy.Engine)),
		ToolVersion:          kv["OPSHUB_TOOL_VERSION"],
		CheckpointBackupType: kv["OPSHUB_CHECKPOINT_BACKUP_TYPE"],
		CheckpointFromLSN:    kv["OPSHUB_CHECKPOINT_FROM_LSN"],
		CheckpointToLSN:      kv["OPSHUB_CHECKPOINT_TO_LSN"],
		CheckpointLastLSN:    kv["OPSHUB_CHECKPOINT_LAST_LSN"],
		BackupBinlogFile:     kv["OPSHUB_BACKUP_BINLOG_FILE"],
		BackupBinlogPos:      binlogPos,
		BackupGTIDSet:        kv["OPSHUB_BACKUP_GTID_SET"],
		ParentArtifactPath:   kv["OPSHUB_PARENT_ARTIFACT_PATH"],
		Stdout:               trimText(stdout, maxRunnerOutputLength),
		Stderr:               trimText(stderr, maxRunnerOutputLength),
		ExitCode:             exitCode,
		StartedAt:            started.Format("2006-01-02 15:04:05"),
		FinishedAt:           finished.Format("2006-01-02 15:04:05"),
		DurationMs:           finished.Sub(started).Milliseconds(),
	}
	if parent != nil {
		result.ParentRecordID = parent.ID
		result.BaseRecordID = parent.BaseRecordID
	}
	if runErr != nil {
		result.Error = runErr.Error()
	}
	for _, line := range strings.Split(stdout, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "OPSHUB_RESTORE_STEP="); ok {
			parts := strings.SplitN(value, "|", 3)
			if len(parts) == 3 {
				result.Steps = append(result.Steps, physicalRestoreStep{Name: parts[0], Status: parts[1], OccurredAt: parts[2]})
			}
		}
	}
	return result
}

func (uc *UseCase) applyMySQLPhysicalBackupPolicyResult(ctx context.Context, policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, job *DatabaseRunnerJob, audit *DatabaseQueryAudit, result mysqlPhysicalBackupRunnerResult, runErr error) {
	now := time.Now()
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	durationMs := result.DurationMs
	if runErr != nil || result.ExitCode != 0 {
		message := trimText(firstNonEmpty(result.Error, result.Stderr, "MySQL/MariaDB 物理备份执行失败"), 1000)
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startTimeFromRecord(record, now, durationMs), now, message)
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusFailed, message)
		uc.markBackupChainStateError(ctx, policy.ID, message)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, message)
	} else if result.FileSize <= 0 || strings.TrimSpace(result.ChecksumSHA256) == "" || strings.TrimSpace(result.StorageURI) == "" || strings.TrimSpace(result.CheckpointToLSN) == "" {
		message := "MySQL/MariaDB 物理备份已执行，但未生成完整 artifact/checkpoint metadata"
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startTimeFromRecord(record, now, durationMs), now, message)
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusFailed, message)
		uc.markBackupChainStateError(ctx, policy.ID, message)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, message)
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		record.FileName = trimText(result.FileName, 255)
		record.FilePath = trimText(result.FilePath, 500)
		record.StorageURI = trimText(result.StorageURI, 1000)
		record.FileSize = result.FileSize
		record.ChecksumSHA256 = trimText(result.ChecksumSHA256, 64)
		record.ToolName = trimText(result.ToolName, 60)
		record.ToolVersion = trimText(result.ToolVersion, 120)
		record.CheckpointBackupType = trimText(result.CheckpointBackupType, 60)
		record.CheckpointFromLSN = trimText(result.CheckpointFromLSN, 120)
		record.CheckpointToLSN = trimText(result.CheckpointToLSN, 120)
		record.CheckpointLastLSN = trimText(result.CheckpointLastLSN, 120)
		record.BackupBinlogFile = trimText(result.BackupBinlogFile, 255)
		record.BackupBinlogPos = result.BackupBinlogPos
		record.BackupGTIDSet = strings.TrimSpace(result.BackupGTIDSet)
		record.ArtifactState = DatabaseBackupArtifactStateRemote
		record.ArtifactCacheURI = trimText(fmt.Sprintf("runner://runner-host-%d%s", result.RunnerHostID, result.WorkDir), 1000)
		record.PrepareStatus = "ready"
		record.ManifestJSON = buildMySQLPolicyPhysicalBackupManifest(record, policy, result)
		uc.enrichMySQLPolicyBackupRecordMetadata(ctx, record, policy, result)
		started := startTimeFromRecord(record, now, durationMs)
		uc.finishBackupRecordSuccess(ctx, record, started, now, durationMs, result.FileSize, result.ChecksumSHA256, "MySQL/MariaDB 物理备份策略执行完成")
		if normalizeBackupLevel(record.BackupLevel) == DatabaseBackupLevelIncremental {
			policy.LastIncrementalAt = &now
		} else {
			policy.LastFullAt = &now
		}
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusSuccess, "MySQL/MariaDB 物理备份策略执行完成")
		uc.updateBackupChainStateAfterSuccess(ctx, policy, record, now)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
}

func buildMySQLPolicyPhysicalBackupManifest(record *DatabaseBackupRecord, policy *DatabaseBackupPolicyConfig, result mysqlPhysicalBackupRunnerResult) string {
	payload := map[string]any{
		"engine":               policy.BackupEngine,
		"method":               DatabaseBackupMethodPhysical,
		"level":                record.BackupLevel,
		"origin":               record.BackupOrigin,
		"backupScope":          "instance",
		"storageUri":           result.StorageURI,
		"checksumSha256":       result.ChecksumSHA256,
		"checkpointBackupType": result.CheckpointBackupType,
		"checkpointFromLsn":    result.CheckpointFromLSN,
		"checkpointToLsn":      result.CheckpointToLSN,
		"checkpointLastLsn":    result.CheckpointLastLSN,
		"backupBinlogFile":     result.BackupBinlogFile,
		"backupBinlogPos":      result.BackupBinlogPos,
		"backupGtidSet":        result.BackupGTIDSet,
		"toolName":             result.ToolName,
		"toolVersion":          result.ToolVersion,
		"runnerHostId":         result.RunnerHostID,
		"parentRecordId":       record.ParentRecordID,
		"baseRecordId":         record.BaseRecordID,
		"generatedAt":          time.Now().Format("2006-01-02 15:04:05"),
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func (uc *UseCase) enrichMySQLPolicyBackupRecordMetadata(ctx context.Context, record *DatabaseBackupRecord, policy *DatabaseBackupPolicyConfig, result mysqlPhysicalBackupRunnerResult) {
	if uc.credentialResolver == nil || record == nil || policy == nil || uc.instanceRepo == nil {
		return
	}
	instanceID := normalizeBackupSourceInstanceID(policy.SourceInstanceID, policy.InstanceID)
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil || instance == nil {
		return
	}
	credential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		return
	}
	meta, err := collectMySQLBackupMetadata(ctx, instance, credential, result.ToolName, result.ToolVersion)
	if err != nil || meta == nil {
		return
	}
	record.ServerUUID = meta.ServerUUID
	record.ServerID = meta.ServerID
	record.GTIDMode = meta.GTIDMode
	record.ExecutedGTIDSet = meta.ExecutedGTIDSet
	record.PurgedGTIDSet = meta.PurgedGTIDSet
	record.BinlogFormat = meta.BinlogFormat
	record.BinlogRowImage = meta.BinlogRowImage
	record.BackupGTIDSet = firstNonEmpty(record.BackupGTIDSet, meta.ExecutedGTIDSet, meta.PurgedGTIDSet)
}

func mysqlPhysicalBackupRequestJSON(policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, host *DatabaseRunnerHost, instance *DatabaseInstance, sourceInstance *DatabaseInstance, operator QueryOperator) string {
	sourceInstanceID := instance.ID
	if sourceInstance != nil {
		sourceInstanceID = sourceInstance.ID
	}
	payload := map[string]any{
		"backupPolicyId":   policy.ID,
		"backupRecordId":   record.ID,
		"instanceId":       instance.ID,
		"sourceInstanceId": sourceInstanceID,
		"runnerHostId":     host.ID,
		"runnerType":       host.RunnerType,
		"allowedCommand":   DatabaseRunnerAllowedCommandMySQLPhysicalBackup,
		"backupEngine":     policy.BackupEngine,
		"backupScope":      "instance",
		"backupLevel":      record.BackupLevel,
		"operatorId":       operator.ID,
		"operatorName":     operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func (uc *UseCase) validateBackupPolicyRequest(ctx context.Context, req *DatabaseBackupPolicyRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.InstanceID == 0 {
		return fmt.Errorf("请选择数据库实例")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("策略名称不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil || instance == nil {
		return fmt.Errorf("数据库实例不存在")
	}
	dbType := normalizeDBType(instance.DBType)
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return fmt.Errorf("%s 暂不支持 MySQL/MariaDB 物理备份策略", DBTypeText(instance.DBType))
	}
	if req.RunnerHostID == 0 {
		return fmt.Errorf("请选择 Runner 主机")
	}
	if uc.runnerHostRepo == nil {
		return fmt.Errorf("Runner 主机仓库未配置")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil || host == nil {
		return fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return err
	}
	if uc.credentialResolver == nil {
		return fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		return fmt.Errorf("凭据不存在")
	}
	if _, err := validateMySQLPhysicalBackupCompatibility(ctx, instance, credential, req.BackupEngine); err != nil {
		return err
	}
	if !isValidBackupSchedule(req.FullSchedule) {
		return fmt.Errorf("全量 Cron 表达式格式不正确")
	}
	if !isValidBackupSchedule(req.IncrementalSchedule) {
		return fmt.Errorf("增量 Cron 表达式格式不正确")
	}
	if err := validateJSONText(req.SyntheticRuleJSON, "合成全量规则"); err != nil {
		return err
	}
	if err := validateJSONText(req.RetentionJSON, "保留策略"); err != nil {
		return err
	}
	if req.SourceInstanceID > 0 {
		sourceInstance, err := uc.instanceRepo.GetByID(ctx, req.SourceInstanceID)
		if err != nil || sourceInstance == nil {
			return fmt.Errorf("备份来源实例不存在")
		}
		sourceDBType := normalizeDBType(sourceInstance.DBType)
		if sourceDBType != DBTypeMySQL && sourceDBType != DBTypeMariaDB {
			return fmt.Errorf("备份来源实例必须是 MySQL/MariaDB")
		}
	}
	if req.BinlogStreamID > 0 && uc.logArchiveStreamRepo != nil {
		stream, err := uc.logArchiveStreamRepo.GetByID(ctx, req.BinlogStreamID)
		if err != nil || stream == nil || stream.InstanceID != req.InstanceID || stream.ArchiveType != DatabaseArchiveTypeBinlog {
			return fmt.Errorf("binlog 归档流不存在或不属于当前实例")
		}
	}
	return nil
}

func (uc *UseCase) applyBackupPolicyRequest(item *DatabaseBackupPolicyConfig, instance *DatabaseInstance, req *DatabaseBackupPolicyRequest) {
	if item == nil || req == nil || instance == nil {
		return
	}
	engine := normalizeMySQLPhysicalBackupEngine(req.BackupEngine, instance.DBType, instance.Version)
	status := DatabaseBackupPolicyStatusActive
	if !req.Enabled {
		status = DatabaseBackupPolicyStatusDisabled
	} else if item.Status != "" && item.Status != DatabaseBackupPolicyStatusDisabled {
		status = item.Status
	}
	item.InstanceID = req.InstanceID
	item.SourceInstanceID = normalizeBackupSourceInstanceID(req.SourceInstanceID, req.InstanceID)
	item.SourceRole = normalizeSourceRole(req.SourceRole)
	item.Name = trimText(strings.TrimSpace(req.Name), 120)
	item.Engine = normalizeDBType(instance.DBType)
	item.BackupEngine = engine
	item.RunnerHostID = req.RunnerHostID
	item.StorageProfileID = req.StorageProfileID
	item.SecretProfileID = req.SecretProfileID
	item.BinlogStreamID = req.BinlogStreamID
	item.FullSchedule = strings.TrimSpace(req.FullSchedule)
	item.IncrementalSchedule = strings.TrimSpace(req.IncrementalSchedule)
	item.SyntheticEnabled = req.SyntheticEnabled
	item.SyntheticRuleJSON = trimText(strings.TrimSpace(req.SyntheticRuleJSON), 4000)
	item.RestoreDrillRequired = req.RestoreDrillRequired
	item.RetentionJSON = trimText(strings.TrimSpace(req.RetentionJSON), 4000)
	item.Enabled = req.Enabled
	item.Status = status
	if !req.Enabled {
		item.LastError = ""
	}
}

func validatePolicyIncrementalParent(policy *DatabaseBackupPolicyConfig, host *DatabaseRunnerHost, parent *DatabaseBackupRecord) error {
	if policy == nil || host == nil || parent == nil {
		return fmt.Errorf("增量备份父记录不存在")
	}
	if parent.PolicyID != policy.ID {
		return fmt.Errorf("增量备份父记录不属于当前策略")
	}
	if parent.Status != DatabaseBackupStatusSuccess {
		return fmt.Errorf("增量备份父记录不是成功状态")
	}
	if normalizeBackupMethod(parent.BackupMethod) != DatabaseBackupMethodPhysical {
		return fmt.Errorf("增量备份父记录不是物理备份")
	}
	if strings.TrimSpace(parent.BackupEngine) != strings.TrimSpace(policy.BackupEngine) {
		return fmt.Errorf("增量备份父记录备份引擎与策略不一致")
	}
	if parent.SourceInstanceID != normalizeBackupSourceInstanceID(policy.SourceInstanceID, policy.InstanceID) {
		return fmt.Errorf("增量备份父记录来源实例与策略不一致")
	}
	if normalizeBackupLevel(parent.BackupLevel) != DatabaseBackupLevelFull && normalizeBackupLevel(parent.BackupLevel) != DatabaseBackupLevelIncremental {
		return fmt.Errorf("增量备份父记录层级不支持")
	}
	if parent.ArtifactState == DatabaseBackupArtifactStateMissing || parent.ArtifactState == DatabaseBackupArtifactStateChecksumFailed {
		return fmt.Errorf("增量备份父记录 artifact 不可用")
	}
	if strings.TrimSpace(parent.ChecksumSHA256) == "" {
		return fmt.Errorf("增量备份父记录缺少 checksum")
	}
	if strings.TrimSpace(parent.CheckpointToLSN) == "" {
		return fmt.Errorf("增量备份父记录缺少 checkpoint to_lsn")
	}
	if _, err := resolveRunnerReadableArtifactPath(parent.StorageURI, parent.FilePath, host.ID); err != nil {
		return err
	}
	return nil
}

func backupPolicyAuditTask(policy *DatabaseBackupPolicyConfig, level string) *DatabaseBackupTask {
	return &DatabaseBackupTask{
		Name:         policy.Name,
		BackupType:   DatabaseBackupTypePhysical,
		BackupMethod: DatabaseBackupMethodPhysical,
		BackupLevel:  normalizeBackupLevel(level),
		BackupEngine: policy.BackupEngine,
		StorageType:  DatabaseBackupStorageExternal,
	}
}

func (uc *UseCase) getBackupPolicy(ctx context.Context, id uint) (*DatabaseBackupPolicyConfig, error) {
	if uc.backupPolicyConfigRepo == nil {
		return nil, fmt.Errorf("备份策略仓库未配置")
	}
	item, err := uc.backupPolicyConfigRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("备份策略不存在")
	}
	return item, nil
}

func (uc *UseCase) loadBackupChainState(ctx context.Context, policyID uint) (*DatabaseBackupChainState, error) {
	if uc.backupChainStateRepo == nil || policyID == 0 {
		return nil, fmt.Errorf("备份链状态仓库未配置")
	}
	state, err := uc.backupChainStateRepo.GetByPolicyID(ctx, policyID)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (uc *UseCase) updateBackupChainStateAfterSuccess(ctx context.Context, policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, finishedAt time.Time) {
	if uc.backupChainStateRepo == nil || policy == nil || record == nil {
		return
	}
	state, err := uc.backupChainStateRepo.GetByPolicyID(ctx, policy.ID)
	if err != nil {
		state = &DatabaseBackupChainState{
			PolicyID:   policy.ID,
			InstanceID: policy.InstanceID,
			Status:     DatabaseBackupChainStateHealthy,
		}
	}
	if normalizeBackupLevel(record.BackupLevel) == DatabaseBackupLevelFull {
		state.ChainID = record.ChainID
		state.CurrentBaseRecordID = record.ID
		state.LatestRecordID = record.ID
		if record.BackupOrigin == DatabaseBackupOriginSyntheticFull {
			state.LatestSyntheticRecordID = record.ID
		} else {
			state.LatestFullRecordID = record.ID
		}
		state.IncrementalCount = 0
		if record.StartedAt != nil {
			state.ChainStartedAt = record.StartedAt
		}
	} else {
		state.LatestRecordID = record.ID
		state.IncrementalCount++
	}
	state.InstanceID = policy.InstanceID
	state.LastSuccessAt = &finishedAt
	state.RecoverableUntil = record.RecoverableUntil
	state.Status = DatabaseBackupChainStateHealthy
	state.LastValidationStatus = DatabaseBackupChainStatusComplete
	state.LastError = ""
	if state.ID == 0 {
		_ = uc.backupChainStateRepo.Create(ctx, state)
		return
	}
	_ = uc.backupChainStateRepo.Update(ctx, state)
}

func (uc *UseCase) markBackupChainStateError(ctx context.Context, policyID uint, message string) {
	if uc.backupChainStateRepo == nil || policyID == 0 {
		return
	}
	state, err := uc.backupChainStateRepo.GetByPolicyID(ctx, policyID)
	if err != nil || state == nil {
		return
	}
	state.Status = DatabaseBackupChainStateDegraded
	state.LastValidationStatus = DatabaseBackupChainStatusBrokenChain
	state.LastError = trimText(message, 1000)
	_ = uc.backupChainStateRepo.Update(ctx, state)
}

func (uc *UseCase) finishBackupPolicy(ctx context.Context, policy *DatabaseBackupPolicyConfig, finishedAt time.Time, status, message string) {
	if uc.backupPolicyConfigRepo == nil || policy == nil || policy.ID == 0 {
		return
	}
	policy.LastRunAt = &finishedAt
	policy.LastStatus = status
	policy.LastMessage = trimText(message, 500)
	if status == DatabaseBackupStatusSuccess {
		if policy.LastFullAt == nil || strings.Contains(message, "全量") {
			// The precise field is corrected in applyMySQLPhysicalBackupPolicyResult after record inspection.
		}
		policy.Status = DatabaseBackupPolicyStatusActive
		policy.LastError = ""
	} else if status == DatabaseBackupStatusFailed {
		policy.Status = DatabaseBackupPolicyStatusDegraded
		policy.LastError = trimText(message, 1000)
	}
	uc.applyBackupPolicyNextRunAt(policy, finishedAt)
	_ = uc.backupPolicyConfigRepo.Update(ctx, policy)
}

func (uc *UseCase) markBackupPolicyStatus(ctx context.Context, policy *DatabaseBackupPolicyConfig, status, message string) {
	if uc.backupPolicyConfigRepo == nil || policy == nil || policy.ID == 0 {
		return
	}
	policy.LastStatus = status
	policy.LastMessage = trimText(message, 500)
	_ = uc.backupPolicyConfigRepo.Update(ctx, policy)
}

func (uc *UseCase) acquireBackupPolicyRun(policyID, instanceID uint) error {
	if policyID == 0 {
		return fmt.Errorf("备份策略不存在")
	}
	uc.backupRunMu.Lock()
	defer uc.backupRunMu.Unlock()
	if uc.backupRunningPolicies == nil {
		uc.backupRunningPolicies = make(map[uint]struct{})
	}
	if uc.backupRunningInstances == nil {
		uc.backupRunningInstances = make(map[uint]struct{})
	}
	if _, exists := uc.backupRunningPolicies[policyID]; exists {
		return fmt.Errorf("备份策略正在执行中，请稍后重试")
	}
	if instanceID > 0 {
		if _, exists := uc.backupRunningInstances[instanceID]; exists {
			return fmt.Errorf("数据库实例已有备份任务正在执行中，请稍后重试")
		}
	}
	uc.backupRunningPolicies[policyID] = struct{}{}
	if instanceID > 0 {
		uc.backupRunningInstances[instanceID] = struct{}{}
	}
	return nil
}

func (uc *UseCase) releaseBackupPolicyRun(policyID, instanceID uint) {
	if policyID == 0 {
		return
	}
	uc.backupRunMu.Lock()
	delete(uc.backupRunningPolicies, policyID)
	if instanceID > 0 {
		delete(uc.backupRunningInstances, instanceID)
	}
	uc.backupRunMu.Unlock()
}

func backupOriginForLevel(level string) string {
	if normalizeBackupLevel(level) == DatabaseBackupLevelIncremental {
		return DatabaseBackupOriginNativeIncremental
	}
	return DatabaseBackupOriginNativeFull
}

func policyRetentionDays(policy *DatabaseBackupPolicyConfig) int {
	if policy == nil {
		return 30
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(policy.RetentionJSON), &payload); err == nil {
		for _, key := range []string{"incrementalKeepDays", "fullKeepDays", "binlogKeepDays"} {
			if value, ok := payload[key]; ok {
				switch typed := value.(type) {
				case float64:
					if typed > 0 {
						return int(typed)
					}
				case int:
					if typed > 0 {
						return typed
					}
				}
			}
		}
	}
	return 45
}

func startTimeFromRecord(record *DatabaseBackupRecord, fallback time.Time, durationMs int64) time.Time {
	if record != nil && record.StartedAt != nil {
		return *record.StartedAt
	}
	return fallback.Add(-time.Duration(durationMs) * time.Millisecond)
}

func validateJSONText(value, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return fmt.Errorf("%s JSON 格式错误: %w", label, err)
	}
	if err := validateBackupStorageConfigSafe(value); err != nil {
		return err
	}
	return nil
}

func normalizeBackupPolicyListRequest(req *DatabaseBackupPolicyListRequest) {
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
	req.Status = strings.TrimSpace(req.Status)
	req.Enabled = strings.ToLower(strings.TrimSpace(req.Enabled))
}

func BackupPolicyStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupPolicyStatusActive:
		return "启用"
	case DatabaseBackupPolicyStatusDegraded:
		return "降级"
	case DatabaseBackupPolicyStatusFailed:
		return "失败"
	case DatabaseBackupPolicyStatusDisabled:
		return "已禁用"
	default:
		return "待配置"
	}
}

func BackupChainStateText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupChainStateHealthy:
		return "健康"
	case DatabaseBackupChainStateDegraded:
		return "降级"
	case DatabaseBackupChainStateBroken:
		return "断链"
	case DatabaseBackupChainStateConsolidating:
		return "合成中"
	default:
		return "未知"
	}
}

func (uc *UseCase) toBackupPolicyVO(ctx context.Context, item *DatabaseBackupPolicyConfig) *DatabaseBackupPolicyVO {
	if item == nil {
		return nil
	}
	instanceName := ""
	instanceDBType := item.Engine
	if uc != nil && uc.instanceRepo != nil && item.InstanceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID); err == nil && instance != nil {
			instanceName = instance.Name
			instanceDBType = instance.DBType
		}
	}
	runnerName := ""
	if uc != nil && uc.runnerHostRepo != nil && item.RunnerHostID > 0 {
		if host, err := uc.runnerHostRepo.GetByID(ctx, item.RunnerHostID); err == nil && host != nil {
			runnerName = host.Name
		}
	}
	var chain *DatabaseBackupChainStateVO
	if uc != nil && uc.backupChainStateRepo != nil && item.ID > 0 {
		if state, err := uc.backupChainStateRepo.GetByPolicyID(ctx, item.ID); err == nil {
			chain = toBackupChainStateVO(state)
		}
	}
	return &DatabaseBackupPolicyVO{
		ID:                   item.ID,
		InstanceID:           item.InstanceID,
		InstanceName:         instanceName,
		InstanceDBType:       instanceDBType,
		SourceInstanceID:     item.SourceInstanceID,
		SourceRole:           item.SourceRole,
		Name:                 item.Name,
		Engine:               item.Engine,
		BackupEngine:         item.BackupEngine,
		RunnerHostID:         item.RunnerHostID,
		RunnerHostName:       runnerName,
		StorageProfileID:     item.StorageProfileID,
		SecretProfileID:      item.SecretProfileID,
		BinlogStreamID:       item.BinlogStreamID,
		FullSchedule:         item.FullSchedule,
		IncrementalSchedule:  item.IncrementalSchedule,
		SyntheticEnabled:     item.SyntheticEnabled,
		SyntheticRuleJSON:    item.SyntheticRuleJSON,
		RestoreDrillRequired: item.RestoreDrillRequired,
		RetentionJSON:        item.RetentionJSON,
		Enabled:              item.Enabled,
		Status:               item.Status,
		StatusText:           BackupPolicyStatusText(item.Status),
		NextFullRunAt:        formatTime(item.NextFullRunAt),
		NextIncrementalRunAt: formatTime(item.NextIncrementalRunAt),
		LastRunAt:            formatTime(item.LastRunAt),
		LastFullAt:           formatTime(item.LastFullAt),
		LastIncrementalAt:    formatTime(item.LastIncrementalAt),
		LastSyntheticAt:      formatTime(item.LastSyntheticAt),
		LastRestoreDrillAt:   formatTime(item.LastRestoreDrillAt),
		LastStatus:           item.LastStatus,
		LastStatusText:       BackupStatusText(item.LastStatus),
		LastMessage:          item.LastMessage,
		LastError:            item.LastError,
		Chain:                chain,
		CreatedAt:            item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:            item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toBackupChainStateVO(item *DatabaseBackupChainState) *DatabaseBackupChainStateVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupChainStateVO{
		ID:                      item.ID,
		PolicyID:                item.PolicyID,
		InstanceID:              item.InstanceID,
		ChainID:                 item.ChainID,
		CurrentBaseRecordID:     item.CurrentBaseRecordID,
		LatestRecordID:          item.LatestRecordID,
		LatestFullRecordID:      item.LatestFullRecordID,
		LatestSyntheticRecordID: item.LatestSyntheticRecordID,
		IncrementalCount:        item.IncrementalCount,
		ChainStartedAt:          formatTime(item.ChainStartedAt),
		LastSuccessAt:           formatTime(item.LastSuccessAt),
		RecoverableUntil:        formatTime(item.RecoverableUntil),
		Status:                  item.Status,
		StatusText:              BackupChainStateText(item.Status),
		LastValidationStatus:    item.LastValidationStatus,
		LastError:               item.LastError,
	}
}

func (uc *UseCase) refreshBackupPolicyNextRunAt(ctx context.Context, policy *DatabaseBackupPolicyConfig, now time.Time) {
	if policy == nil || policy.ID == 0 || uc.backupPolicyConfigRepo == nil {
		return
	}
	uc.applyBackupPolicyNextRunAt(policy, now)
	_ = uc.backupPolicyConfigRepo.Update(ctx, policy)
}

func (uc *UseCase) applyBackupPolicyNextRunAt(policy *DatabaseBackupPolicyConfig, now time.Time) {
	if policy == nil {
		return
	}
	if next, _ := upcomingPolicyRunAt(policy.FullSchedule, policy.LastFullAt, policy.CreatedAt, now); !next.IsZero() {
		policy.NextFullRunAt = &next
	} else {
		policy.NextFullRunAt = nil
	}
	if next, _ := upcomingPolicyRunAt(policy.IncrementalSchedule, policy.LastIncrementalAt, policy.CreatedAt, now); !next.IsZero() {
		policy.NextIncrementalRunAt = &next
	} else {
		policy.NextIncrementalRunAt = nil
	}
}

func upcomingPolicyRunAt(scheduleText string, lastRunAt *time.Time, createdAt time.Time, now time.Time) (time.Time, error) {
	schedule, err := parseBackupSchedule(scheduleText)
	if err != nil || schedule == nil {
		return time.Time{}, err
	}
	baseTime := createdAt
	if lastRunAt != nil && !lastRunAt.IsZero() {
		baseTime = *lastRunAt
	}
	next := schedule.Next(baseTime)
	for !next.IsZero() && !next.After(now) {
		next = schedule.Next(next)
	}
	return next, nil
}

func duePolicyRunAt(scheduleText string, lastRunAt *time.Time, createdAt time.Time, now time.Time) (bool, time.Time, error) {
	schedule, err := parseBackupSchedule(scheduleText)
	if err != nil || schedule == nil {
		return false, time.Time{}, err
	}
	baseTime := createdAt
	if lastRunAt != nil && !lastRunAt.IsZero() {
		baseTime = *lastRunAt
	}
	next := schedule.Next(baseTime)
	return !next.IsZero() && !now.Before(next), next, nil
}
