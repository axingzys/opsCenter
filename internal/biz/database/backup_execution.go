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
	return uc.runBackupTask(ctx, id, scheduledBackupOperator(), DatabaseBackupTriggerSchedule)
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

func (uc *UseCase) runBackupTask(ctx context.Context, id uint, operator QueryOperator, triggerType string) (*DatabaseBackupRunVO, error) {
	if uc.backupTaskRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份任务仓库未配置")
	}

	task, err := uc.getBackupTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := uc.acquireBackupTaskRun(task.ID); err != nil {
		return nil, err
	}
	defer uc.releaseBackupTaskRun(task.ID)

	instance, err := uc.instanceRepo.GetByID(ctx, task.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
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
	databaseName, err := resolveBackupDatabaseName(instance, credential)
	if err != nil {
		return nil, err
	}
	spec, err := buildBackupCommandSpec(instance, credential, databaseName, task.BackupType)
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
	outputPath, fileName, err := buildBackupOutputPath(storageRoot, instance, task, databaseName, startedAt, spec.FileExt)
	if err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, err.Error())
		return nil, err
	}

	record := &DatabaseBackupRecord{
		TaskID:       task.ID,
		InstanceID:   task.InstanceID,
		TriggerType:  normalizeBackupTriggerType(triggerType),
		BackupType:   task.BackupType,
		StorageType:  task.StorageType,
		Status:       DatabaseBackupStatusRunning,
		FilePath:     outputPath,
		FileName:     fileName,
		StartedAt:    &startedAt,
		ErrorMessage: trimText(backupRunningMessage, 500),
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建备份记录失败: "+err.Error())
		return nil, err
	}

	task.LastRunAt = &startedAt
	task.LastStatus = DatabaseBackupStatusRunning
	task.LastMessage = trimText(backupRunningMessage, 500)
	if err := uc.backupTaskRepo.Update(ctx, task); err != nil {
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startedAt, time.Now(), "更新备份任务状态失败: "+err.Error())
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "更新备份任务状态失败: "+err.Error())
		return nil, err
	}

	fileSize, err := runBackupCommand(ctx, spec, outputPath)
	finishedAt := time.Now()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	if err != nil {
		uc.finishBackupRecord(ctx, record, DatabaseBackupStatusFailed, startedAt, finishedAt, err.Error())
		uc.finishBackupTask(ctx, task, finishedAt, DatabaseBackupStatusFailed, err.Error())
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, err.Error())
		return nil, err
	}

	recordMessage := buildBackupSuccessMessage(fileName, fileSize)
	taskMessage := recordMessage
	cleanedCount, cleanupErr := uc.cleanupExpiredBackupFilesByTask(ctx, task)
	if cleanedCount > 0 {
		taskMessage = recordMessage + "，" + buildBackupCleanupMessage(cleanedCount)
	}
	if cleanupErr != nil {
		taskMessage = trimText(taskMessage+"，"+backupCleanupFailedMessage, 500)
	}
	taskMessage = trimText(taskMessage, 500)

	uc.finishBackupRecordSuccess(ctx, record, startedAt, finishedAt, durationMs, fileSize, recordMessage)
	uc.finishBackupTask(ctx, task, finishedAt, DatabaseBackupStatusSuccess, taskMessage)
	uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")

	return &DatabaseBackupRunVO{
		TaskID:       task.ID,
		TaskName:     task.Name,
		RecordID:     record.ID,
		InstanceID:   task.InstanceID,
		InstanceName: instance.Name,
		Status:       DatabaseBackupStatusSuccess,
		StatusText:   BackupStatusText(DatabaseBackupStatusSuccess),
		FileName:     fileName,
		FileSize:     fileSize,
		DurationMs:   durationMs,
		Message:      taskMessage,
		TriggeredAt:  startedAt.Format("2006-01-02 15:04:05"),
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
	record.ErrorMessage = trimText(buildBackupPrunedMessage(record.FileName), 500)
	if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
		return false, err
	}
	return true, nil
}

func (uc *UseCase) acquireBackupTaskRun(taskID uint) error {
	if taskID == 0 {
		return fmt.Errorf("备份任务不存在")
	}
	uc.backupRunMu.Lock()
	defer uc.backupRunMu.Unlock()
	if _, exists := uc.backupRunningTasks[taskID]; exists {
		return fmt.Errorf("备份任务正在执行中，请稍后重试")
	}
	uc.backupRunningTasks[taskID] = struct{}{}
	return nil
}

func (uc *UseCase) releaseBackupTaskRun(taskID uint) {
	if taskID == 0 {
		return
	}
	uc.backupRunMu.Lock()
	delete(uc.backupRunningTasks, taskID)
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
	case DatabaseBackupTriggerSchedule, DatabaseBackupTriggerManualRetry:
		return triggerType
	default:
		return DatabaseBackupTriggerManual
	}
}

func isBackupTaskRunningError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "备份任务正在执行中")
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
