package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (uc *UseCase) executeMySQLSyntheticFullPolicyJob(ctx context.Context, policyID, recordID, jobID uint, audit *DatabaseQueryAudit, selectedRecords []*DatabaseBackupRecord) {
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
	host, err := uc.runnerHostRepo.GetByID(ctx, job.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)
	uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusRunning, "MySQL/MariaDB 合成全量执行中")
	uc.markBackupPolicyStatus(ctx, policy, DatabaseBackupStatusRunning, "MySQL/MariaDB 合成全量执行中")
	stdout, stderr, exitCode, runErr := uc.runMySQLSyntheticFullCommand(ctx, policy, record, selectedRecords, host)
	finished := time.Now()
	parent := latestSelectedBackupRecord(selectedRecords)
	result := parseMySQLPhysicalBackupRunnerResult(policy, record, parent, host, stdout, stderr, exitCode, runErr, started, finished)
	result.RunnerJobID = jobID
	uc.applyMySQLSyntheticFullResult(ctx, policy, record, job, audit, selectedRecords, result, runErr)
}

func (uc *UseCase) runMySQLSyntheticFullCommand(ctx context.Context, policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, selectedRecords []*DatabaseBackupRecord, host *DatabaseRunnerHost) (string, string, int, error) {
	runnerCredential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	inputs := make([]syntheticFullArtifactInput, 0, len(selectedRecords))
	for _, item := range selectedRecords {
		if item == nil {
			continue
		}
		path, err := resolveRunnerReadableArtifactPath(item.StorageURI, item.FilePath, host.ID)
		if err != nil {
			return "", "", 1, fmt.Errorf("记录 #%d artifact 不可由 Runner 读取: %w", item.ID, err)
		}
		inputs = append(inputs, syntheticFullArtifactInput{
			RecordID:       item.ID,
			BackupLevel:    item.BackupLevel,
			BackupOrigin:   item.BackupOrigin,
			Path:           path,
			ChecksumSHA256: strings.TrimSpace(item.ChecksumSHA256),
			FileSize:       item.FileSize,
		})
	}
	if len(inputs) < 2 {
		return "", "", 1, fmt.Errorf("合成全量至少需要一个基础全量和一个增量")
	}
	script := buildMySQLSyntheticFullScript(policy, record, host, inputs)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, runnerCredential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func buildMySQLSyntheticFullScript(policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, host *DatabaseRunnerHost, inputs []syntheticFullArtifactInput) string {
	workRoot := firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)
	toolName := mysqlPhysicalBackupToolName(policy.BackupEngine, policy.Engine)
	lines := []string{
		"set -eu",
		"WORK_ROOT=" + shellSingleQuote(workRoot),
		fmt.Sprintf("POLICY_ID=%d", policy.ID),
		fmt.Sprintf("BACKUP_RECORD_ID=%d", record.ID),
		fmt.Sprintf("RUNNER_HOST_ID=%d", host.ID),
		"TOOL_NAME=" + shellSingleQuote(toolName),
		"FILE_NAME=" + shellSingleQuote(record.FileName),
		"WORK_DIR=\"$WORK_ROOT/mysql-physical/policy-$POLICY_ID/synthetic-$BACKUP_RECORD_ID\"",
		"BASE_DIR=\"$WORK_DIR/base\"",
		"INC_ROOT=\"$WORK_DIR/incrementals\"",
		"ARTIFACT=\"$WORK_DIR/$FILE_NAME\"",
		"LOG_FILE=\"$WORK_DIR/mysql_synthetic_full.log\"",
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
		`rm -rf "$BASE_DIR" "$INC_ROOT" "$ARTIFACT"`,
		`mkdir -p "$BASE_DIR" "$INC_ROOT"`,
		`verify_artifact() { path="$1"; expected="$2"; record_id="$3"; if [ ! -f "$path" ]; then fail_step "verify_artifact" "artifact not found for record $record_id"; fi; actual="$(sha256sum "$path" | awk '{print $1}')"; if [ "$actual" != "$expected" ]; then fail_step "verify_artifact" "checksum mismatch for record $record_id"; fi; }`,
		`checkpoint_value() { awk -F= -v key="$1" '{gsub(/^[ \t]+|[ \t]+$/,"",$1); if ($1==key) {gsub(/^[ \t]+|[ \t]+$/,"",$2); print $2; exit}}' "$BASE_DIR/xtrabackup_checkpoints" 2>/dev/null || true; }`,
		`step "verify_artifact" "running"`,
	}
	for _, input := range inputs {
		lines = append(lines,
			"verify_artifact "+shellSingleQuote(input.Path)+" "+shellSingleQuote(input.ChecksumSHA256)+" "+shellSingleQuote(strconv.FormatUint(uint64(input.RecordID), 10)),
		)
	}
	lines = append(lines,
		`step "verify_artifact" "success"`,
		`step "unpack_base" "running"`,
		"tar -xzf "+shellSingleQuote(inputs[0].Path)+` -C "$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "unpack_base" "unpack base artifact failed"`,
		`if [ ! -f "$BASE_DIR/xtrabackup_checkpoints" ]; then fail_step "unpack_base" "base xtrabackup_checkpoints not found"; fi`,
		`step "unpack_base" "success"`,
		`step "prepare_base" "running"`,
		`"$TOOL_PATH" --prepare --apply-log-only --target-dir="$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "prepare_base" "prepare base failed"`,
		`step "prepare_base" "success"`,
	)
	for _, input := range inputs[1:] {
		lines = append(lines,
			fmt.Sprintf(`step "apply_incremental_%d" "running"`, input.RecordID),
			fmt.Sprintf(`INC_DIR="$INC_ROOT/inc-%d"`, input.RecordID),
			`mkdir -p "$INC_DIR"`,
			"tar -xzf "+shellSingleQuote(input.Path)+` -C "$INC_DIR" >> "$LOG_FILE" 2>&1 || fail_step "apply_incremental_`+strconv.FormatUint(uint64(input.RecordID), 10)+`" "unpack incremental failed"`,
			`if [ ! -f "$INC_DIR/xtrabackup_checkpoints" ]; then fail_step "apply_incremental_`+strconv.FormatUint(uint64(input.RecordID), 10)+`" "incremental checkpoints not found"; fi`,
			`"$TOOL_PATH" --prepare --apply-log-only --target-dir="$BASE_DIR" --incremental-dir="$INC_DIR" >> "$LOG_FILE" 2>&1 || fail_step "apply_incremental_`+strconv.FormatUint(uint64(input.RecordID), 10)+`" "apply incremental failed"`,
			fmt.Sprintf(`step "apply_incremental_%d" "success"`, input.RecordID),
		)
	}
	lines = append(lines,
		`step "final_prepare" "running"`,
		`"$TOOL_PATH" --prepare --target-dir="$BASE_DIR" >> "$LOG_FILE" 2>&1 || fail_step "final_prepare" "final prepare failed"`,
		`step "final_prepare" "success"`,
		`step "package_backup" "running"`,
		`tar -czf "$ARTIFACT" -C "$BASE_DIR" . >> "$LOG_FILE" 2>&1 || fail_step "package_backup" "package synthetic full failed"`,
		`size="$(wc -c < "$ARTIFACT" | tr -d ' ')"`,
		`sha="$(sha256sum "$ARTIFACT" | awk '{print $1}')"`,
		`checkpoint_backup_type="$(checkpoint_value backup_type)"`,
		`checkpoint_from_lsn="$(checkpoint_value from_lsn)"`,
		`checkpoint_to_lsn="$(checkpoint_value to_lsn)"`,
		`checkpoint_last_lsn="$(checkpoint_value last_lsn)"`,
		`binlog_info="$BASE_DIR/xtrabackup_binlog_info"; if [ ! -f "$binlog_info" ]; then binlog_info="$BASE_DIR/mariadb_backup_binlog_info"; fi`,
		`binlog_file=""; binlog_pos="0"; binlog_gtid=""; if [ -f "$binlog_info" ]; then binlog_file="$(awk 'NR==1 {print $1}' "$binlog_info")"; binlog_pos="$(awk 'NR==1 {print $2}' "$binlog_info")"; binlog_gtid="$(awk 'NR==1 {$1=""; $2=""; sub(/^[ \t]+/,""); print}' "$binlog_info")"; fi`,
		`printf 'OPSHUB_FILE_SIZE=%s\n' "$size"`,
		`printf 'OPSHUB_CHECKSUM_SHA256=%s\n' "$sha"`,
		`printf 'OPSHUB_STORAGE_URI=runner://runner-host-`+strconv.Itoa(int(host.ID))+`%s\n' "$ARTIFACT"`,
		`printf 'OPSHUB_CHECKPOINT_BACKUP_TYPE=%s\n' "$checkpoint_backup_type"`,
		`printf 'OPSHUB_CHECKPOINT_FROM_LSN=%s\n' "$checkpoint_from_lsn"`,
		`printf 'OPSHUB_CHECKPOINT_TO_LSN=%s\n' "$checkpoint_to_lsn"`,
		`printf 'OPSHUB_CHECKPOINT_LAST_LSN=%s\n' "$checkpoint_last_lsn"`,
		`printf 'OPSHUB_BACKUP_BINLOG_FILE=%s\n' "$binlog_file"`,
		`printf 'OPSHUB_BACKUP_BINLOG_POS=%s\n' "$binlog_pos"`,
		`printf 'OPSHUB_BACKUP_GTID_SET=%s\n' "$binlog_gtid"`,
		`step "package_backup" "success"`,
	)
	return strings.Join(lines, "\n")
}

func (uc *UseCase) applyMySQLSyntheticFullResult(ctx context.Context, policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, job *DatabaseRunnerJob, audit *DatabaseQueryAudit, selectedRecords []*DatabaseBackupRecord, result mysqlPhysicalBackupRunnerResult, runErr error) {
	now := time.Now()
	sourceIDs := selectedBackupRecordIDs(selectedRecords)
	resultJSON, _ := json.Marshal(map[string]any{
		"result":          result,
		"sourceRecordIds": sourceIDs,
	})
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	durationMs := result.DurationMs
	if runErr != nil || result.ExitCode != 0 {
		message := trimText(firstNonEmpty(result.Error, result.Stderr, "MySQL/MariaDB 合成全量执行失败"), 1000)
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startTimeFromRecord(record, now, durationMs), now, message)
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusFailed, message)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, message)
	} else if result.FileSize <= 0 || strings.TrimSpace(result.ChecksumSHA256) == "" || strings.TrimSpace(result.StorageURI) == "" || strings.TrimSpace(result.CheckpointToLSN) == "" {
		message := "MySQL/MariaDB 合成全量已执行，但未生成完整 artifact/checkpoint metadata"
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = message
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startTimeFromRecord(record, now, durationMs), now, message)
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusFailed, message)
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
		record.CheckpointBackupType = trimText(firstNonEmpty(result.CheckpointBackupType, "synthetic_full"), 60)
		record.CheckpointFromLSN = trimText(result.CheckpointFromLSN, 120)
		record.CheckpointToLSN = trimText(result.CheckpointToLSN, 120)
		record.CheckpointLastLSN = trimText(result.CheckpointLastLSN, 120)
		record.BackupBinlogFile = trimText(result.BackupBinlogFile, 255)
		record.BackupBinlogPos = result.BackupBinlogPos
		record.BackupGTIDSet = strings.TrimSpace(result.BackupGTIDSet)
		record.BackupOrigin = DatabaseBackupOriginSyntheticFull
		record.BackupLevel = DatabaseBackupLevelFull
		record.BaseRecordID = record.ID
		if parent := latestSelectedBackupRecord(selectedRecords); parent != nil {
			record.ParentRecordID = parent.ID
			record.RecoverableUntil = parent.RecoverableUntil
			record.ServerUUID = parent.ServerUUID
			record.ServerID = parent.ServerID
			record.GTIDMode = parent.GTIDMode
			record.ExecutedGTIDSet = parent.ExecutedGTIDSet
			record.PurgedGTIDSet = parent.PurgedGTIDSet
			record.BinlogFormat = parent.BinlogFormat
			record.BinlogRowImage = parent.BinlogRowImage
		}
		if base := firstSelectedBackupRecord(selectedRecords); base != nil {
			record.RecoverableFrom = base.RecoverableFrom
			if record.CheckpointFromLSN == "" {
				record.CheckpointFromLSN = base.CheckpointFromLSN
			}
		}
		sourceIDsJSON, _ := json.Marshal(sourceIDs)
		record.SyntheticSourceRecordIDs = string(sourceIDsJSON)
		record.ArtifactState = DatabaseBackupArtifactStateRemote
		record.ArtifactCacheURI = trimText(fmt.Sprintf("runner://runner-host-%d%s", result.RunnerHostID, result.WorkDir), 1000)
		record.PrepareStatus = "synthetic_ready"
		record.ManifestJSON = buildMySQLPolicySyntheticFullManifest(record, policy, selectedRecords, result)
		rule := parseSyntheticRule(policy)
		if strings.TrimSpace(record.RestoreTestStatus) == "" && (rule.RequireRestoreProof || policy.RestoreDrillRequired || rule.NeverDeleteWithoutProof) {
			record.RestoreTestStatus = DatabaseBackupStatusPending
		}
		started := startTimeFromRecord(record, now, durationMs)
		uc.finishBackupRecordSuccess(ctx, record, started, now, durationMs, result.FileSize, result.ChecksumSHA256, "MySQL/MariaDB 合成全量执行完成")
		policy.LastSyntheticAt = &now
		uc.finishBackupPolicy(ctx, policy, now, DatabaseBackupStatusSuccess, "MySQL/MariaDB 合成全量执行完成")
		uc.updateBackupChainStateAfterSuccess(ctx, policy, record, now)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
}

func buildMySQLPolicySyntheticFullManifest(record *DatabaseBackupRecord, policy *DatabaseBackupPolicyConfig, selectedRecords []*DatabaseBackupRecord, result mysqlPhysicalBackupRunnerResult) string {
	sourceIDs := selectedBackupRecordIDs(selectedRecords)
	payload := map[string]any{
		"engine":               policy.BackupEngine,
		"method":               DatabaseBackupMethodPhysical,
		"level":                DatabaseBackupLevelFull,
		"origin":               DatabaseBackupOriginSyntheticFull,
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
		"sourceRecordIds":      sourceIDs,
		"parentRecordId":       record.ParentRecordID,
		"baseRecordId":         record.BaseRecordID,
		"generatedAt":          time.Now().Format("2006-01-02 15:04:05"),
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func mysqlSyntheticFullRequestJSON(policy *DatabaseBackupPolicyConfig, record *DatabaseBackupRecord, host *DatabaseRunnerHost, instance *DatabaseInstance, sourceInstance *DatabaseInstance, operator QueryOperator, sourceRecordIDs []uint, triggerReason string, triggerRecordID uint, rule syntheticRuleConfig) string {
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
		"allowedCommand":   DatabaseRunnerAllowedCommandMySQLSyntheticFull,
		"backupEngine":     policy.BackupEngine,
		"backupScope":      "instance",
		"backupLevel":      DatabaseBackupLevelFull,
		"backupOrigin":     DatabaseBackupOriginSyntheticFull,
		"triggerType":      record.TriggerType,
		"triggerReason":    strings.TrimSpace(triggerReason),
		"triggerRecordId":  triggerRecordID,
		"syntheticRule": map[string]any{
			"mode":                         rule.Mode,
			"autoRun":                      rule.AutoRun,
			"triggerAfterIncrementals":     rule.TriggerAfterIncrementals,
			"mergeOldestIncrementals":      rule.MergeOldestIncrementals,
			"requireRestoreProof":          rule.RequireRestoreProof,
			"neverDeleteWithoutProof":      rule.NeverDeleteWithoutProof,
			"markSupersededAfterProof":     rule.MarkSupersededAfterProof,
			"supersededKeepDaysAfterProof": rule.SupersededKeepDaysAfterProof,
		},
		"sourceRecordIds": sourceRecordIDs,
		"operatorId":      operator.ID,
		"operatorName":    operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func selectedBackupRecordIDs(records []*DatabaseBackupRecord) []uint {
	ids := make([]uint, 0, len(records))
	for _, item := range records {
		if item != nil {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func firstSelectedBackupRecord(records []*DatabaseBackupRecord) *DatabaseBackupRecord {
	for _, item := range records {
		if item != nil {
			return item
		}
	}
	return nil
}

func latestSelectedBackupRecord(records []*DatabaseBackupRecord) *DatabaseBackupRecord {
	for i := len(records) - 1; i >= 0; i-- {
		if records[i] != nil {
			return records[i]
		}
	}
	return nil
}
