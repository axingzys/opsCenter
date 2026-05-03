package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const backupCleanupFailedMessage = "保留策略清理失败"

func (uc *UseCase) RunScheduledBackupTask(ctx context.Context, id uint) (*DatabaseBackupRunVO, error) {
	if vo, handled, err := uc.runBarmanBackupTaskIfNeeded(ctx, id, scheduledBackupOperator(), DatabaseBackupTriggerSchedule); handled || err != nil {
		return vo, err
	}
	if vo, handled, err := uc.runPgBaseBackupTaskIfNeeded(ctx, id, scheduledBackupOperator(), DatabaseBackupTriggerSchedule); handled || err != nil {
		return vo, err
	}
	run, err := uc.prepareBackupTaskRun(ctx, id, scheduledBackupOperator(), DatabaseBackupTriggerSchedule)
	if err != nil {
		return nil, err
	}
	return uc.executePreparedBackupRun(ctx, run)
}

func (uc *UseCase) ListEnabledBackupTasks(ctx context.Context) ([]*DatabaseBackupTask, error) {
	if uc.backupTaskRepo == nil {
		return nil, fmt.Errorf("备份任务仓库未配置")
	}
	return uc.backupTaskRepo.ListEnabled(ctx)
}

func (uc *UseCase) ListAllBackupTasks(ctx context.Context) ([]*DatabaseBackupTask, error) {
	if uc.backupTaskRepo == nil {
		return nil, fmt.Errorf("备份任务仓库未配置")
	}
	return uc.backupTaskRepo.ListAll(ctx)
}

type preparedBackupRun struct {
	task         *DatabaseBackupTask
	instance     *DatabaseInstance
	credential   *ConnectionCredential
	spec         *backupCommandSpec
	audit        *DatabaseQueryAudit
	record       *DatabaseBackupRecord
	databaseName string
	outputPath   string
	fileName     string
	startedAt    time.Time
}

func (uc *UseCase) prepareBackupTaskRun(ctx context.Context, id uint, operator QueryOperator, triggerType string) (*preparedBackupRun, error) {
	if uc.backupTaskRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份任务仓库未配置")
	}

	task, err := uc.getBackupTask(ctx, id)
	if err != nil {
		return nil, err
	}
	instance, err := uc.instanceRepo.GetByID(ctx, task.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if err := uc.acquireBackupTaskRun(task.ID, task.InstanceID); err != nil {
		return nil, err
	}
	releaseOnError := true
	defer func() {
		if releaseOnError {
			uc.releaseBackupTaskRun(task.ID, task.InstanceID)
		}
	}()
	if !supportsBackupTask(instance.DBType) {
		return nil, fmt.Errorf("%s 逻辑备份将在后续批次接入", DBTypeText(instance.DBType))
	}
	if strings.TrimSpace(instance.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}

	credential, err := uc.credentialResolver(ctx, instance.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}
	policy, err := uc.resolveBackupPolicy(ctx)
	if err != nil {
		return nil, err
	}
	databaseName, err := resolveBackupDatabaseNameForTask(instance, credential, task)
	if err != nil {
		return nil, err
	}
	spec, err := buildBackupCommandSpecForTask(instance, credential, databaseName, task)
	if err != nil {
		return nil, err
	}
	storageRoot, err := resolveBackupStorageRoot(policy)
	if err != nil {
		return nil, err
	}

	audit, err := uc.startBackupAudit(ctx, instance, task, databaseName, operator)
	if err != nil {
		return nil, fmt.Errorf("创建查询审计失败: %w", err)
	}

	startedAt := time.Now()
	expiresAt := startedAt.AddDate(0, 0, normalizeBackupRetentionDays(task.RetentionDays, policy.DefaultRetentionDays))
	outputPath, fileName, err := buildBackupOutputPath(storageRoot, instance, task, databaseName, startedAt, spec.FileExt)
	if err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, err.Error())
		return nil, err
	}

	record := &DatabaseBackupRecord{
		TaskID:           task.ID,
		InstanceID:       task.InstanceID,
		TriggerType:      normalizeBackupTriggerType(triggerType),
		BackupType:       task.BackupType,
		ChainID:          fmt.Sprintf("logical-%d-%d", task.ID, startedAt.Unix()),
		BackupMethod:     normalizeBackupMethod(task.BackupMethod),
		BackupLevel:      normalizeBackupLevel(task.BackupLevel),
		BackupEngine:     backupEngineForSpec(task, spec),
		ToolName:         backupToolNameForSpec(spec),
		SourceInstanceID: normalizeBackupSourceInstanceID(task.SourceInstanceID, task.InstanceID),
		SourceRole:       normalizeSourceRole(task.SourceRole),
		StorageProfileID: task.StorageProfileID,
		StorageType:      task.StorageType,
		Status:           DatabaseBackupStatusQueued,
		FilePath:         outputPath,
		FileName:         fileName,
		Compression:      detectBackupCompression(fileName),
		ExpiresAt:        &expiresAt,
		VerifyStatus:     DatabaseBackupVerifyStatusPending,
		StartedAt:        &startedAt,
		LastHeartbeatAt:  &startedAt,
		ErrorMessage:     trimText(backupQueuedMessage, 500),
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建备份记录失败: "+err.Error())
		return nil, err
	}

	task.LastRunAt = &startedAt
	task.LastStatus = DatabaseBackupStatusQueued
	task.LastMessage = trimText(backupQueuedMessage, 500)
	task.RestoreCapability = normalizeRestoreCapability(task.RestoreCapability)
	uc.applyBackupTaskNextRunAt(task, startedAt)
	if err := uc.backupTaskRepo.Update(ctx, task); err != nil {
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startedAt, time.Now(), "更新备份任务状态失败: "+err.Error())
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "更新备份任务状态失败: "+err.Error())
		return nil, err
	}

	releaseOnError = false
	return &preparedBackupRun{
		task:         task,
		instance:     instance,
		credential:   credential,
		spec:         spec,
		audit:        audit,
		record:       record,
		databaseName: databaseName,
		outputPath:   outputPath,
		fileName:     fileName,
		startedAt:    startedAt,
	}, nil
}

func (uc *UseCase) executePreparedBackupRun(ctx context.Context, run *preparedBackupRun) (*DatabaseBackupRunVO, error) {
	if run == nil || run.task == nil || run.record == nil || run.spec == nil {
		return nil, fmt.Errorf("备份任务执行上下文无效")
	}
	defer uc.releaseBackupTaskRun(run.task.ID, run.task.InstanceID)

	task := run.task
	record := run.record
	startedAt := run.startedAt

	uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusRunning, backupRunningMessage)
	uc.markBackupTaskStatus(ctx, task, DatabaseBackupStatusRunning, backupRunningMessage)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(normalizeBackupMaxDurationMinutes(task.MaxDurationMinutes))*time.Minute)
	defer cancel()
	stopHeartbeat := uc.startBackupRecordHeartbeat(runCtx, record)
	fileSize, err := runBackupCommand(runCtx, run.spec, run.outputPath)
	finishedAt := time.Now()
	stopHeartbeat()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	if err != nil {
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startedAt, finishedAt, err.Error())
		uc.finishBackupTask(ctx, task, finishedAt, DatabaseBackupStatusFailed, err.Error())
		uc.finishBackupAudit(ctx, run.audit, DatabaseQueryStatusFailed, durationMs, err.Error())
		return nil, err
	}
	checksum, err := calculateFileSHA256(run.outputPath)
	if err != nil {
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startedAt, finishedAt, err.Error())
		uc.finishBackupTask(ctx, task, finishedAt, DatabaseBackupStatusFailed, err.Error())
		uc.finishBackupAudit(ctx, run.audit, DatabaseQueryStatusFailed, durationMs, err.Error())
		return nil, err
	}
	uc.enrichPhysicalBackupRecordMetadata(ctx, record, run.instance, run.credential, run.spec, startedAt, finishedAt, run.outputPath, checksum)

	recordMessage := buildBackupSuccessMessage(run.fileName, fileSize)
	taskMessage := recordMessage
	uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusCleaning, "备份文件已生成，保留策略清理中")
	uc.markBackupTaskStatus(ctx, task, DatabaseBackupStatusCleaning, "备份文件已生成，保留策略清理中")
	cleanedCount, cleanupErr := uc.cleanupExpiredBackupFilesByTask(ctx, task)
	if cleanedCount > 0 {
		taskMessage = recordMessage + "，" + buildBackupCleanupMessage(cleanedCount)
	}
	if cleanupErr != nil {
		taskMessage = trimText(taskMessage+"，"+backupCleanupFailedMessage, 500)
	}
	taskMessage = trimText(taskMessage, 500)

	uc.finishBackupRecordSuccess(ctx, record, startedAt, finishedAt, durationMs, fileSize, checksum, recordMessage)
	uc.finishBackupTask(ctx, task, finishedAt, DatabaseBackupStatusSuccess, taskMessage)
	uc.finishBackupAudit(ctx, run.audit, DatabaseQueryStatusSuccess, durationMs, "")

	return &DatabaseBackupRunVO{
		TaskID:         task.ID,
		TaskName:       task.Name,
		RecordID:       record.ID,
		InstanceID:     task.InstanceID,
		InstanceName:   run.instance.Name,
		Status:         DatabaseBackupStatusSuccess,
		StatusText:     BackupStatusText(DatabaseBackupStatusSuccess),
		FileName:       run.fileName,
		FileSize:       fileSize,
		ChecksumSHA256: checksum,
		DurationMs:     durationMs,
		Message:        taskMessage,
		TriggeredAt:    startedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) CleanupExpiredBackupFiles(ctx context.Context) (int, error) {
	tasks, err := uc.ListAllBackupTasks(ctx)
	if err != nil {
		return 0, err
	}

	totalCleaned := 0
	var messages []string
	for _, task := range tasks {
		cleanedCount, cleanupErr := uc.cleanupExpiredBackupFilesByTask(ctx, task)
		totalCleaned += cleanedCount
		if cleanupErr != nil {
			messages = append(messages, fmt.Sprintf("%s: %s", strings.TrimSpace(task.Name), cleanupErr.Error()))
		}
	}
	if len(messages) > 0 {
		return totalCleaned, fmt.Errorf("%s", strings.Join(messages, "; "))
	}
	return totalCleaned, nil
}

func (uc *UseCase) cleanupExpiredBackupFilesByTask(ctx context.Context, task *DatabaseBackupTask) (int, error) {
	if uc.backupRecordRepo == nil || task == nil || task.ID == 0 {
		return 0, nil
	}
	retentionDays := normalizeBackupRetentionDays(task.RetentionDays, 7)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	records, err := uc.backupRecordRepo.ListExpiredSuccessByTask(ctx, task.ID, cutoff)
	if err != nil {
		return 0, err
	}

	cleanedCount := 0
	var messages []string
	for _, record := range records {
		cleaned, cleanupErr := uc.pruneBackupRecordFile(ctx, record)
		if cleaned {
			cleanedCount++
		}
		if cleanupErr != nil {
			label := fmt.Sprintf("#%d", record.ID)
			if strings.TrimSpace(record.FileName) != "" {
				label = strings.TrimSpace(record.FileName)
			}
			messages = append(messages, fmt.Sprintf("%s: %s", label, cleanupErr.Error()))
		}
	}
	if len(messages) > 0 {
		return cleanedCount, fmt.Errorf("%s", strings.Join(messages, "; "))
	}
	return cleanedCount, nil
}

func (uc *UseCase) pruneBackupRecordFile(ctx context.Context, record *DatabaseBackupRecord) (bool, error) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return false, nil
	}
	filePath := strings.TrimSpace(record.FilePath)
	if filePath == "" {
		return false, nil
	}

	securePath, err := uc.secureBackupFilePath(ctx, filePath)
	if err != nil {
		return false, err
	}

	if err := os.Remove(securePath); err != nil && !os.IsNotExist(err) {
		return false, err
	}

	record.FilePath = ""
	record.Status = DatabaseBackupStatusExpired
	record.VerifyStatus = DatabaseBackupVerifyStatusExpired
	record.VerifyMessage = trimText(buildBackupPrunedMessage(record.FileName), 500)
	record.ErrorMessage = trimText(buildBackupPrunedMessage(record.FileName), 500)
	if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
		return false, err
	}
	return true, nil
}

func backupRunQueuedVO(run *preparedBackupRun) *DatabaseBackupRunVO {
	if run == nil || run.task == nil || run.record == nil {
		return nil
	}
	instanceName := ""
	if run.instance != nil {
		instanceName = run.instance.Name
	}
	return &DatabaseBackupRunVO{
		TaskID:       run.task.ID,
		TaskName:     run.task.Name,
		RecordID:     run.record.ID,
		InstanceID:   run.task.InstanceID,
		InstanceName: instanceName,
		Status:       DatabaseBackupStatusQueued,
		StatusText:   BackupStatusText(DatabaseBackupStatusQueued),
		FileName:     run.fileName,
		Message:      backupQueuedMessage,
		TriggeredAt:  run.startedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) startBackupRecordHeartbeat(ctx context.Context, record *DatabaseBackupRecord) func() {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return func() {}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				now := time.Now()
				record.LastHeartbeatAt = &now
				_ = uc.backupRecordRepo.Update(context.Background(), record)
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func (uc *UseCase) acquireBackupTaskRun(taskID, instanceID uint) error {
	if taskID == 0 {
		return fmt.Errorf("备份任务不存在")
	}
	uc.backupRunMu.Lock()
	defer uc.backupRunMu.Unlock()
	if uc.backupRunningTasks == nil {
		uc.backupRunningTasks = make(map[uint]struct{})
	}
	if uc.backupRunningInstances == nil {
		uc.backupRunningInstances = make(map[uint]struct{})
	}
	if _, exists := uc.backupRunningTasks[taskID]; exists {
		return fmt.Errorf("备份任务正在执行中，请稍后重试")
	}
	if instanceID > 0 {
		if _, exists := uc.backupRunningInstances[instanceID]; exists {
			return fmt.Errorf("数据库实例已有备份任务正在执行中，请稍后重试")
		}
	}
	uc.backupRunningTasks[taskID] = struct{}{}
	if instanceID > 0 {
		uc.backupRunningInstances[instanceID] = struct{}{}
	}
	return nil
}

func (uc *UseCase) releaseBackupTaskRun(taskID, instanceID uint) {
	if taskID == 0 {
		return
	}
	uc.backupRunMu.Lock()
	delete(uc.backupRunningTasks, taskID)
	if instanceID > 0 {
		delete(uc.backupRunningInstances, instanceID)
	}
	uc.backupRunMu.Unlock()
}

func scheduledBackupOperator() QueryOperator {
	return QueryOperator{
		ID:       0,
		Username: "system-scheduler",
		ClientIP: "system",
	}
}

func normalizeBackupTriggerType(triggerType string) string {
	switch strings.TrimSpace(triggerType) {
	case DatabaseBackupTriggerSchedule, DatabaseBackupTriggerManualRetry, DatabaseBackupTriggerExternal:
		return triggerType
	default:
		return DatabaseBackupTriggerManual
	}
}

func isBackupTaskRunningError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "备份任务正在执行中") || strings.Contains(err.Error(), "备份策略正在执行中") || strings.Contains(err.Error(), "数据库实例已有备份任务正在执行中"))
}

func backupEngineForSpec(task *DatabaseBackupTask, spec *backupCommandSpec) string {
	if task != nil && strings.TrimSpace(task.BackupEngine) != "" {
		return strings.TrimSpace(task.BackupEngine)
	}
	if spec == nil {
		return "logical"
	}
	if len(spec.Commands) > 0 {
		return strings.TrimSpace(spec.Commands[0])
	}
	if spec.Runner != nil {
		return "opshub_runner"
	}
	return "logical"
}

func backupToolNameForSpec(spec *backupCommandSpec) string {
	if spec == nil {
		return ""
	}
	if len(spec.Commands) > 0 {
		return strings.TrimSpace(spec.Commands[0])
	}
	if spec.Runner != nil {
		return "opshub_runner"
	}
	return ""
}

func buildBackupCleanupMessage(cleanedCount int) string {
	if cleanedCount <= 0 {
		return ""
	}
	return fmt.Sprintf("已清理 %d 个过期备份文件", cleanedCount)
}

func buildBackupPrunedMessage(fileName string) string {
	if strings.TrimSpace(fileName) == "" {
		return "逻辑备份完成，文件已按保留策略清理"
	}
	return fmt.Sprintf("逻辑备份完成，文件 %s 已按保留策略清理", strings.TrimSpace(fileName))
}
