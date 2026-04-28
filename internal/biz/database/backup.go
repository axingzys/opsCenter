package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	backupRunningMessage = "逻辑备份执行中"
	backupSuccessMessage = "逻辑备份完成"
	backupQueuedMessage  = "备份任务已进入执行队列"
	backupStaleMessage   = "备份进程已中断、心跳超时或超过任务最大运行时长，已自动标记失败"

	defaultBackupMaxDurationMinutes  = 24 * 60
	maxBackupMaxDurationMinutes      = 7 * 24 * 60
	backupHeartbeatTimeout           = 3 * time.Minute
	logicalBackupLargeThresholdBytes = 50 * 1024 * 1024 * 1024
)

func (uc *UseCase) ListBackupTasks(ctx context.Context, req *DatabaseBackupTaskListRequest) ([]*DatabaseBackupTaskVO, int64, error) {
	if uc.backupTaskRepo == nil {
		return nil, 0, fmt.Errorf("备份任务仓库未配置")
	}

	normalizeBackupTaskListRequest(req)
	items, total, err := uc.backupTaskRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	instanceNames, instanceTypes := uc.loadBackupInstanceMeta(ctx, items)
	list := make([]*DatabaseBackupTaskVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toBackupTaskVO(ctx, item, instanceNames[item.InstanceID], instanceTypes[item.InstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) GetBackupTaskInstanceID(ctx context.Context, id uint) (uint, error) {
	if id == 0 {
		return 0, fmt.Errorf("备份任务ID不能为空")
	}
	item, err := uc.backupTaskRepo.GetByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("备份任务不存在")
	}
	return item.InstanceID, nil
}

func (uc *UseCase) CreateBackupTask(ctx context.Context, req *DatabaseBackupTaskRequest) (*DatabaseBackupTaskVO, error) {
	if err := uc.validateBackupTaskRequest(ctx, req); err != nil {
		return nil, err
	}
	policy, err := uc.resolveBackupPolicy(ctx)
	if err != nil {
		return nil, err
	}

	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}

	item := &DatabaseBackupTask{
		InstanceID:         req.InstanceID,
		Name:               trimText(strings.TrimSpace(req.Name), 120),
		BackupType:         normalizeBackupType(req.BackupType),
		Schedule:           strings.TrimSpace(req.Schedule),
		StorageType:        normalizeBackupStorageType(req.StorageType),
		StorageConfig:      trimText(strings.TrimSpace(req.StorageConfig), 2000),
		RetentionDays:      normalizeBackupRetentionDays(req.RetentionDays, policy.DefaultRetentionDays),
		MaxDurationMinutes: normalizeBackupMaxDurationMinutes(req.MaxDurationMinutes),
		Enabled:            req.Enabled,
		RestoreCapability:  DatabaseRestoreCapabilityLogicalRestoreOnly,
	}
	if err := uc.backupTaskRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	uc.refreshBackupTaskNextRunAt(ctx, item, time.Now())
	return uc.toBackupTaskVO(ctx, item, instance.Name, instance.DBType), nil
}

func (uc *UseCase) UpdateBackupTask(ctx context.Context, id uint, req *DatabaseBackupTaskRequest) (*DatabaseBackupTaskVO, error) {
	if err := uc.validateBackupTaskRequest(ctx, req); err != nil {
		return nil, err
	}
	policy, err := uc.resolveBackupPolicy(ctx)
	if err != nil {
		return nil, err
	}

	item, err := uc.getBackupTask(ctx, id)
	if err != nil {
		return nil, err
	}

	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}

	item.InstanceID = req.InstanceID
	item.Name = trimText(strings.TrimSpace(req.Name), 120)
	item.BackupType = normalizeBackupType(req.BackupType)
	item.Schedule = strings.TrimSpace(req.Schedule)
	item.StorageType = normalizeBackupStorageType(req.StorageType)
	item.StorageConfig = trimText(strings.TrimSpace(req.StorageConfig), 2000)
	item.RetentionDays = normalizeBackupRetentionDays(req.RetentionDays, policy.DefaultRetentionDays)
	item.MaxDurationMinutes = normalizeBackupMaxDurationMinutes(req.MaxDurationMinutes)
	item.Enabled = req.Enabled
	item.RestoreCapability = normalizeRestoreCapability(item.RestoreCapability)
	uc.applyBackupTaskNextRunAt(item, time.Now())

	if err := uc.backupTaskRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBackupTaskVO(ctx, item, instance.Name, instance.DBType), nil
}

func (uc *UseCase) DeleteBackupTask(ctx context.Context, id uint) error {
	if _, err := uc.getBackupTask(ctx, id); err != nil {
		return err
	}
	return uc.backupTaskRepo.Delete(ctx, id)
}

func (uc *UseCase) RunBackupTask(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupRunVO, error) {
	run, err := uc.prepareBackupTaskRun(ctx, id, operator, DatabaseBackupTriggerManual)
	if err != nil {
		return nil, err
	}
	go func() {
		_, _ = uc.executePreparedBackupRun(context.Background(), run)
	}()
	return backupRunQueuedVO(run), nil
}

func (uc *UseCase) ListBackupRecords(ctx context.Context, req *DatabaseBackupRecordListRequest) ([]*DatabaseBackupRecordVO, int64, error) {
	if uc.backupRecordRepo == nil {
		return nil, 0, fmt.Errorf("备份记录仓库未配置")
	}

	normalizeBackupRecordListRequest(req)
	items, total, err := uc.backupRecordRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	uc.reconcileStaleBackupRecords(ctx, items)

	taskNames := uc.loadBackupTaskNames(ctx, items)
	instanceNames, _ := uc.loadBackupInstanceMetaByRecord(ctx, items)
	list := make([]*DatabaseBackupRecordVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toBackupRecordVO(item, taskNames[item.TaskID], instanceNames[item.InstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) GetBackupRecordInstanceID(ctx context.Context, id uint) (uint, error) {
	if id == 0 {
		return 0, fmt.Errorf("备份记录ID不能为空")
	}
	item, err := uc.backupRecordRepo.GetByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("备份记录不存在")
	}
	return item.InstanceID, nil
}

func (uc *UseCase) reconcileStaleBackupRecords(ctx context.Context, items []*DatabaseBackupRecord) {
	if uc.backupRecordRepo == nil {
		return
	}
	now := time.Now()
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Status) != DatabaseBackupStatusRunning || item.StartedAt == nil {
			continue
		}
		task := uc.loadBackupTaskForRecord(ctx, item.TaskID)
		if !isBackupRecordStale(item, task, uc.startedAt, now) {
			continue
		}
		item.Status = DatabaseBackupStatusFailed
		item.FinishedAt = &now
		item.DurationMs = now.Sub(*item.StartedAt).Milliseconds()
		item.ErrorMessage = trimText(backupStaleMessage, 500)
		_ = uc.backupRecordRepo.Update(ctx, item)
		uc.markBackupTaskStale(ctx, item.TaskID, now)
	}
}

func (uc *UseCase) markBackupTaskStale(ctx context.Context, taskID uint, finishedAt time.Time) {
	if uc.backupTaskRepo == nil || taskID == 0 {
		return
	}
	task, err := uc.backupTaskRepo.GetByID(ctx, taskID)
	if err != nil || task == nil || strings.TrimSpace(task.LastStatus) != DatabaseBackupStatusRunning {
		return
	}
	task.LastStatus = DatabaseBackupStatusFailed
	task.LastMessage = trimText(backupStaleMessage, 500)
	task.UpdatedAt = finishedAt
	_ = uc.backupTaskRepo.Update(ctx, task)
}

func (uc *UseCase) DownloadBackupRecord(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupDownloadVO, error) {
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	record, err := uc.backupRecordRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("备份记录不存在")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, record.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}

	if strings.TrimSpace(record.Status) != DatabaseBackupStatusSuccess {
		err = fmt.Errorf("仅允许下载成功的备份文件")
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusDenied, err.Error())
		return nil, err
	}
	if strings.TrimSpace(record.FilePath) == "" || strings.TrimSpace(record.FileName) == "" {
		err = fmt.Errorf("备份文件不存在")
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	filePath, err := uc.secureBackupFilePath(ctx, record.FilePath)
	if err != nil {
		err = fmt.Errorf("备份文件路径无效: %w", err)
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	if _, err := uc.verifyBackupRecordFile(ctx, record, filePath); err != nil {
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusSuccess, "下载前 checksum 校验通过")
	uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusSuccess, "")
	return &DatabaseBackupDownloadVO{
		FilePath:    filePath,
		FileName:    record.FileName,
		ContentType: detectBackupContentType(record.FileName),
	}, nil
}

func (uc *UseCase) VerifyBackupRecord(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupRecordVO, error) {
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	record, err := uc.backupRecordRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("备份记录不存在")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, record.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(record.Status) != DatabaseBackupStatusSuccess {
		err = fmt.Errorf("仅允许校验成功的备份记录")
		uc.recordBackupVerifyAudit(ctx, instance, record, operator, DatabaseQueryStatusDenied, err.Error())
		return nil, err
	}
	if strings.TrimSpace(record.FilePath) == "" || strings.TrimSpace(record.FileName) == "" {
		err = fmt.Errorf("备份文件不存在或已清理")
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupVerifyAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}
	filePath, err := uc.secureBackupFilePath(ctx, record.FilePath)
	if err != nil {
		err = fmt.Errorf("备份文件路径无效: %w", err)
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupVerifyAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}
	info, err := uc.verifyBackupRecordFile(ctx, record, filePath)
	if err != nil {
		uc.markBackupRecordVerifyResult(ctx, record, DatabaseBackupVerifyStatusFailed, err.Error())
		uc.recordBackupVerifyAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	now := time.Now()
	record.VerifiedAt = &now
	record.VerifyStatus = DatabaseBackupVerifyStatusSuccess
	record.VerifyMessage = "备份文件 checksum 校验通过"
	if info != nil {
		record.FileSize = info.Size()
	}
	if strings.TrimSpace(record.Compression) == "" {
		record.Compression = detectBackupCompression(record.FileName)
	}
	if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
		return nil, err
	}
	uc.recordBackupVerifyAudit(ctx, instance, record, operator, DatabaseQueryStatusSuccess, "")

	taskName := ""
	if uc.backupTaskRepo != nil && record.TaskID > 0 {
		if task, taskErr := uc.backupTaskRepo.GetByID(ctx, record.TaskID); taskErr == nil && task != nil {
			taskName = task.Name
		}
	}
	return uc.toBackupRecordVO(record, taskName, instance.Name), nil
}

func (uc *UseCase) getBackupTask(ctx context.Context, id uint) (*DatabaseBackupTask, error) {
	if uc.backupTaskRepo == nil {
		return nil, fmt.Errorf("备份任务仓库未配置")
	}
	item, err := uc.backupTaskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("备份任务不存在")
	}
	return item, nil
}

func (uc *UseCase) validateBackupTaskRequest(ctx context.Context, req *DatabaseBackupTaskRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.InstanceID == 0 {
		return fmt.Errorf("请选择数据库实例")
	}
	if _, err := uc.instanceRepo.GetByID(ctx, req.InstanceID); err != nil {
		return fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("任务名称不能为空")
	}

	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return fmt.Errorf("数据库实例不存在")
	}
	if !supportsBackupTask(instance.DBType) {
		return fmt.Errorf("%s 逻辑备份将在后续批次接入", DBTypeText(instance.DBType))
	}

	backupType := normalizeBackupType(req.BackupType)
	if !supportsBackupType(instance.DBType, backupType) {
		if backupType == DatabaseBackupTypeLogicalCustom {
			return fmt.Errorf("%s 当前不支持 Custom 备份", DBTypeText(instance.DBType))
		}
		return fmt.Errorf("当前不支持备份类型 %s", backupType)
	}
	storageType := normalizeBackupStorageType(req.StorageType)
	if storageType != DatabaseBackupStorageLocal {
		return fmt.Errorf("当前仅支持 local 本地存储")
	}
	if err := validateBackupStorageConfigSafe(req.StorageConfig); err != nil {
		return err
	}
	if !isValidBackupSchedule(req.Schedule) {
		return fmt.Errorf("Cron 表达式格式不正确")
	}
	policy, err := uc.resolveBackupPolicy(ctx)
	if err != nil {
		return err
	}
	if retentionDays := normalizeBackupRetentionDays(req.RetentionDays, policy.DefaultRetentionDays); retentionDays <= 0 || retentionDays > 3650 {
		return fmt.Errorf("保留天数范围必须为 1-3650")
	}
	if req.MaxDurationMinutes < 0 || req.MaxDurationMinutes > maxBackupMaxDurationMinutes {
		return fmt.Errorf("最大运行时长范围必须为 1-%d 分钟", maxBackupMaxDurationMinutes)
	}
	return nil
}

func (uc *UseCase) startBackupAudit(ctx context.Context, instance *DatabaseInstance, task *DatabaseBackupTask, databaseName string, operator QueryOperator) (*DatabaseQueryAudit, error) {
	if uc.auditRepo == nil {
		return nil, nil
	}
	audit := &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		SchemaName:     resolveBackupSchemaName(instance, databaseName),
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionBackupRun,
		SQLText:        trimText(buildBackupAuditText(task, databaseName), 20000),
		SQLFingerprint: sqlFingerprint(buildBackupAuditText(task, databaseName)),
		SQLType:        "BACKUP",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil, err
	}
	return audit, nil
}

func (uc *UseCase) finishBackupAudit(ctx context.Context, audit *DatabaseQueryAudit, status string, durationMs int64, errorMessage string) {
	if uc.auditRepo == nil || audit == nil || audit.ID == 0 {
		return
	}
	audit.Status = status
	audit.DurationMs = durationMs
	audit.ErrorMessage = trimText(errorMessage, 500)
	_ = uc.auditRepo.Update(ctx, audit)
}

func (uc *UseCase) recordBackupDownloadAudit(ctx context.Context, instance *DatabaseInstance, record *DatabaseBackupRecord, operator QueryOperator, status, errorMessage string) {
	if uc.auditRepo == nil || instance == nil || record == nil {
		return
	}
	sqlText := fmt.Sprintf("DOWNLOAD BACKUP %s", strings.TrimSpace(record.FileName))
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		SchemaName:     resolveBackupSchemaName(instance, strings.TrimSpace(instance.DefaultDatabase)),
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionBackupDownload,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "BACKUP",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         status,
		ErrorMessage:   trimText(errorMessage, 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func (uc *UseCase) recordBackupVerifyAudit(ctx context.Context, instance *DatabaseInstance, record *DatabaseBackupRecord, operator QueryOperator, status, errorMessage string) {
	if uc.auditRepo == nil || instance == nil || record == nil {
		return
	}
	sqlText := fmt.Sprintf("VERIFY BACKUP %s", strings.TrimSpace(record.FileName))
	_ = uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     instance.ID,
		SchemaName:     resolveBackupSchemaName(instance, strings.TrimSpace(instance.DefaultDatabase)),
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionBackupVerify,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "BACKUP",
		RiskLevel:      DatabaseQueryRiskMedium,
		Status:         status,
		ErrorMessage:   trimText(errorMessage, 500),
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func (uc *UseCase) markBackupRecordVerifyResult(ctx context.Context, record *DatabaseBackupRecord, status, message string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	now := time.Now()
	record.VerifiedAt = &now
	record.VerifyStatus = status
	record.VerifyMessage = trimText(message, 500)
	_ = uc.backupRecordRepo.Update(ctx, record)
}

func (uc *UseCase) finishBackupRecord(ctx context.Context, record *DatabaseBackupRecord, status string, startedAt, finishedAt time.Time, errorMessage string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	record.Status = status
	record.StartedAt = &startedAt
	record.FinishedAt = &finishedAt
	record.LastHeartbeatAt = &finishedAt
	record.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	record.ErrorMessage = trimText(errorMessage, 500)
	_ = uc.backupRecordRepo.Update(ctx, record)
}

func (uc *UseCase) markBackupRecordStatus(ctx context.Context, record *DatabaseBackupRecord, status, message string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	now := time.Now()
	record.Status = status
	record.LastHeartbeatAt = &now
	record.ErrorMessage = trimText(message, 500)
	_ = uc.backupRecordRepo.Update(ctx, record)
}

func (uc *UseCase) finishBackupRecordSuccess(ctx context.Context, record *DatabaseBackupRecord, startedAt, finishedAt time.Time, durationMs, fileSize int64, checksum, message string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	record.Status = DatabaseBackupStatusSuccess
	record.StartedAt = &startedAt
	record.FinishedAt = &finishedAt
	record.LastHeartbeatAt = &finishedAt
	record.DurationMs = durationMs
	record.FileSize = fileSize
	record.ChecksumSHA256 = strings.TrimSpace(checksum)
	record.Compression = detectBackupCompression(record.FileName)
	record.VerifiedAt = &finishedAt
	record.VerifyStatus = DatabaseBackupVerifyStatusSuccess
	record.VerifyMessage = "备份文件已生成并完成 checksum 计算"
	record.ErrorMessage = trimText(message, 500)
	_ = uc.backupRecordRepo.Update(ctx, record)
}

func (uc *UseCase) finishBackupTask(ctx context.Context, task *DatabaseBackupTask, finishedAt time.Time, status, message string) {
	if uc.backupTaskRepo == nil || task == nil || task.ID == 0 {
		return
	}
	task.LastRunAt = &finishedAt
	task.LastStatus = status
	task.LastMessage = trimText(message, 500)
	task.RestoreCapability = normalizeRestoreCapability(task.RestoreCapability)
	if status == DatabaseBackupStatusSuccess {
		task.LastSuccessAt = &finishedAt
	}
	uc.applyBackupTaskNextRunAt(task, finishedAt)
	_ = uc.backupTaskRepo.Update(ctx, task)
}

func (uc *UseCase) markBackupTaskStatus(ctx context.Context, task *DatabaseBackupTask, status, message string) {
	if uc.backupTaskRepo == nil || task == nil || task.ID == 0 {
		return
	}
	task.LastStatus = status
	task.LastMessage = trimText(message, 500)
	_ = uc.backupTaskRepo.Update(ctx, task)
}

func (uc *UseCase) loadBackupInstanceMeta(ctx context.Context, items []*DatabaseBackupTask) (map[uint]string, map[uint]string) {
	instanceNames := make(map[uint]string)
	instanceTypes := make(map[uint]string)
	for _, item := range items {
		if item == nil || item.InstanceID == 0 {
			continue
		}
		if _, ok := instanceNames[item.InstanceID]; ok {
			continue
		}
		instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
		if err != nil || instance == nil {
			instanceNames[item.InstanceID] = ""
			instanceTypes[item.InstanceID] = ""
			continue
		}
		instanceNames[item.InstanceID] = instance.Name
		instanceTypes[item.InstanceID] = instance.DBType
	}
	return instanceNames, instanceTypes
}

func (uc *UseCase) loadBackupInstanceMetaByRecord(ctx context.Context, items []*DatabaseBackupRecord) (map[uint]string, map[uint]string) {
	instanceNames := make(map[uint]string)
	instanceTypes := make(map[uint]string)
	for _, item := range items {
		if item == nil || item.InstanceID == 0 {
			continue
		}
		if _, ok := instanceNames[item.InstanceID]; ok {
			continue
		}
		instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
		if err != nil || instance == nil {
			instanceNames[item.InstanceID] = ""
			instanceTypes[item.InstanceID] = ""
			continue
		}
		instanceNames[item.InstanceID] = instance.Name
		instanceTypes[item.InstanceID] = instance.DBType
	}
	return instanceNames, instanceTypes
}

func (uc *UseCase) loadBackupTaskNames(ctx context.Context, items []*DatabaseBackupRecord) map[uint]string {
	taskNames := make(map[uint]string)
	for _, item := range items {
		if item == nil || item.TaskID == 0 {
			continue
		}
		if _, ok := taskNames[item.TaskID]; ok {
			continue
		}
		task, err := uc.backupTaskRepo.GetByID(ctx, item.TaskID)
		if err != nil || task == nil {
			taskNames[item.TaskID] = ""
			continue
		}
		taskNames[item.TaskID] = task.Name
	}
	return taskNames
}

func (uc *UseCase) toBackupTaskVO(ctx context.Context, item *DatabaseBackupTask, instanceName, instanceDBType string) *DatabaseBackupTaskVO {
	if item == nil {
		return nil
	}
	capability := normalizeRestoreCapability(item.RestoreCapability)
	nextRunAt := item.NextRunAt
	if nextRunAt == nil || nextRunAt.IsZero() {
		nextRunAt, _ = upcomingBackupRunAt(item, time.Now())
	}
	vo := &DatabaseBackupTaskVO{
		ID:                    item.ID,
		InstanceID:            item.InstanceID,
		InstanceName:          instanceName,
		InstanceDBType:        instanceDBType,
		InstanceDBTypeText:    DBTypeText(instanceDBType),
		Name:                  item.Name,
		BackupType:            item.BackupType,
		BackupTypeText:        BackupTypeText(item.BackupType),
		Schedule:              item.Schedule,
		StorageType:           item.StorageType,
		StorageTypeText:       BackupStorageTypeText(item.StorageType),
		StorageConfig:         item.StorageConfig,
		RetentionDays:         item.RetentionDays,
		MaxDurationMinutes:    normalizeBackupMaxDurationMinutes(item.MaxDurationMinutes),
		Enabled:               item.Enabled,
		NextRunAt:             formatTime(nextRunAt),
		LastRunAt:             formatTime(item.LastRunAt),
		LastSuccessAt:         formatTime(item.LastSuccessAt),
		LastStatus:            item.LastStatus,
		LastStatusText:        backupTaskLastStatusText(item.LastStatus),
		LastMessage:           item.LastMessage,
		RestoreCapability:     capability,
		RestoreCapabilityText: RestoreCapabilityText(capability),
		PITRSupported:         capability == DatabaseRestoreCapabilityPITRCapable || capability == DatabaseRestoreCapabilityPITRVerified,
		PITRStatusText:        PITRStatusText(capability),
		StrategyText:          backupTaskStrategyText(item),
		CreatedAt:             item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:             item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	uc.applyBackupTaskCapacityWarning(ctx, item, vo)
	return vo
}

func (uc *UseCase) toBackupRecordVO(item *DatabaseBackupRecord, taskName, instanceName string) *DatabaseBackupRecordVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupRecordVO{
		ID:                    item.ID,
		TaskID:                item.TaskID,
		TaskName:              taskName,
		InstanceID:            item.InstanceID,
		InstanceName:          instanceName,
		TriggerType:           item.TriggerType,
		TriggerTypeText:       BackupTriggerTypeText(item.TriggerType),
		BackupType:            item.BackupType,
		BackupTypeText:        BackupTypeText(item.BackupType),
		StorageType:           item.StorageType,
		StorageTypeText:       BackupStorageTypeText(item.StorageType),
		Status:                item.Status,
		StatusText:            BackupStatusText(item.Status),
		FileName:              item.FileName,
		FileSize:              item.FileSize,
		ChecksumSHA256:        item.ChecksumSHA256,
		Encrypted:             item.Encrypted,
		Compression:           item.Compression,
		ExpiresAt:             formatTime(item.ExpiresAt),
		VerifiedAt:            formatTime(item.VerifiedAt),
		VerifyStatus:          item.VerifyStatus,
		VerifyStatusText:      BackupVerifyStatusText(item.VerifyStatus),
		VerifyMessage:         item.VerifyMessage,
		RestoreTestedAt:       formatTime(item.RestoreTestedAt),
		RestoreTestStatus:     item.RestoreTestStatus,
		RestoreTestStatusText: RestoreTestStatusText(item.RestoreTestStatus),
		StartedAt:             formatTime(item.StartedAt),
		LastHeartbeatAt:       formatTime(item.LastHeartbeatAt),
		FinishedAt:            formatTime(item.FinishedAt),
		DurationMs:            item.DurationMs,
		Message:               buildBackupRecordMessage(item),
		CreatedAt:             item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:             item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func BackupVerifyStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseBackupVerifyStatusPending:
		return "待校验"
	case DatabaseBackupVerifyStatusSuccess:
		return "校验通过"
	case DatabaseBackupVerifyStatusFailed:
		return "校验失败"
	case DatabaseBackupVerifyStatusExpired:
		return "文件已清理"
	default:
		return ""
	}
}

func RestoreTestStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseBackupStatusSuccess:
		return "演练成功"
	case DatabaseBackupStatusFailed:
		return "演练失败"
	case DatabaseBackupStatusRunning:
		return "演练中"
	case DatabaseBackupStatusPending:
		return "待演练"
	default:
		return ""
	}
}

func RestoreCapabilityText(value string) string {
	switch normalizeRestoreCapability(value) {
	case DatabaseRestoreCapabilityLogicalRestoreOnly:
		return "仅逻辑恢复"
	case DatabaseRestoreCapabilityPhysicalRestore:
		return "物理恢复"
	case DatabaseRestoreCapabilityPITRCapable:
		return "PITR 可用"
	case DatabaseRestoreCapabilityPITRVerified:
		return "PITR 已演练"
	default:
		return "无恢复能力"
	}
}

func PITRStatusText(value string) string {
	switch normalizeRestoreCapability(value) {
	case DatabaseRestoreCapabilityPITRCapable:
		return "支持 PITR，待恢复演练"
	case DatabaseRestoreCapabilityPITRVerified:
		return "支持 PITR，已通过恢复演练"
	default:
		return "不支持 PITR"
	}
}

func supportsBackupTask(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeRedis:
		return true
	default:
		return false
	}
}

func supportsBackupType(dbType, backupType string) bool {
	backupType = normalizeBackupType(backupType)
	switch normalizeDBType(dbType) {
	case DBTypePostgreSQL:
		return backupType == DatabaseBackupTypeLogical || backupType == DatabaseBackupTypeLogicalCustom
	case DBTypeMySQL, DBTypeMariaDB, DBTypeRedis:
		return backupType == DatabaseBackupTypeLogical
	default:
		return false
	}
}

func normalizeBackupType(backupType string) string {
	backupType = strings.ToLower(strings.TrimSpace(backupType))
	if backupType == "" {
		return DatabaseBackupTypeLogical
	}
	return backupType
}

func normalizeRestoreCapability(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseRestoreCapabilityPhysicalRestore,
		DatabaseRestoreCapabilityPITRCapable,
		DatabaseRestoreCapabilityPITRVerified,
		DatabaseRestoreCapabilityNone:
		return strings.TrimSpace(value)
	default:
		return DatabaseRestoreCapabilityLogicalRestoreOnly
	}
}

func normalizeBackupStorageType(storageType string) string {
	storageType = strings.ToLower(strings.TrimSpace(storageType))
	if storageType == "" {
		return DatabaseBackupStorageLocal
	}
	return storageType
}

func detectBackupCompression(fileName string) string {
	name := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(name, ".gz") || strings.Contains(name, ".gz."):
		return "gzip"
	case strings.HasSuffix(name, ".zst") || strings.HasSuffix(name, ".zstd"):
		return "zstd"
	default:
		return "none"
	}
}

func normalizeBackupRetentionDays(retentionDays int, defaultValue int) int {
	if retentionDays <= 0 {
		if defaultValue <= 0 {
			return 7
		}
		return defaultValue
	}
	return retentionDays
}

func normalizeBackupMaxDurationMinutes(value int) int {
	if value <= 0 {
		return defaultBackupMaxDurationMinutes
	}
	if value > maxBackupMaxDurationMinutes {
		return maxBackupMaxDurationMinutes
	}
	return value
}

func validateBackupStorageConfigSafe(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	blocked := []string{
		"password",
		"passwd",
		"secret",
		"access_key",
		"accesskey",
		"secret_key",
		"secretkey",
		"token",
		"private_key",
		"privatekey",
		"ssh_key",
		"credential",
		"kms_key",
	}
	for _, keyword := range blocked {
		if strings.Contains(lower, keyword) {
			return fmt.Errorf("存储配置不允许保存密码、密钥、Token 或凭据内容，请使用后续 storage profile / secret profile")
		}
	}
	return nil
}

func isValidBackupSchedule(schedule string) bool {
	_, err := parseBackupSchedule(schedule)
	return err == nil
}

func buildBackupAuditText(task *DatabaseBackupTask, databaseName string) string {
	if task == nil {
		return "BACKUP TASK"
	}
	if strings.TrimSpace(databaseName) == "" {
		return fmt.Sprintf("BACKUP TASK %s TYPE %s STORAGE %s", strings.TrimSpace(task.Name), strings.ToUpper(task.BackupType), strings.ToUpper(task.StorageType))
	}
	return fmt.Sprintf("BACKUP TASK %s DATABASE %s TYPE %s STORAGE %s", strings.TrimSpace(task.Name), strings.TrimSpace(databaseName), strings.ToUpper(task.BackupType), strings.ToUpper(task.StorageType))
}

func (uc *UseCase) refreshBackupTaskNextRunAt(ctx context.Context, task *DatabaseBackupTask, now time.Time) {
	if task == nil || task.ID == 0 || uc.backupTaskRepo == nil {
		return
	}
	uc.applyBackupTaskNextRunAt(task, now)
	_ = uc.backupTaskRepo.Update(ctx, task)
}

func (uc *UseCase) applyBackupTaskNextRunAt(task *DatabaseBackupTask, now time.Time) {
	if task == nil {
		return
	}
	next, _ := upcomingBackupRunAt(task, now)
	task.NextRunAt = next
}

func upcomingBackupRunAt(task *DatabaseBackupTask, now time.Time) (*time.Time, error) {
	if task == nil {
		return nil, nil
	}
	schedule, err := parseBackupSchedule(task.Schedule)
	if err != nil || schedule == nil {
		return nil, err
	}
	baseTime := task.CreatedAt
	if task.LastRunAt != nil && !task.LastRunAt.IsZero() {
		baseTime = *task.LastRunAt
	}
	next := schedule.Next(baseTime)
	for !next.IsZero() && !next.After(now) {
		next = schedule.Next(next)
	}
	if next.IsZero() {
		return nil, nil
	}
	return &next, nil
}

func (uc *UseCase) loadBackupTaskForRecord(ctx context.Context, taskID uint) *DatabaseBackupTask {
	if uc.backupTaskRepo == nil || taskID == 0 {
		return nil
	}
	task, err := uc.backupTaskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil
	}
	return task
}

func isBackupRecordStale(record *DatabaseBackupRecord, task *DatabaseBackupTask, processStartedAt, now time.Time) bool {
	if record == nil || record.StartedAt == nil {
		return false
	}
	if record.StartedAt.Before(processStartedAt) && record.LastHeartbeatAt == nil {
		return true
	}
	maxDuration := time.Duration(defaultBackupMaxDurationMinutes) * time.Minute
	if task != nil {
		maxDuration = time.Duration(normalizeBackupMaxDurationMinutes(task.MaxDurationMinutes)) * time.Minute
	}
	if maxDuration > 0 && now.Sub(*record.StartedAt) > maxDuration {
		return true
	}
	if record.LastHeartbeatAt != nil && !record.LastHeartbeatAt.IsZero() && now.Sub(*record.LastHeartbeatAt) > backupHeartbeatTimeout {
		return true
	}
	return false
}

func backupTaskStrategyText(task *DatabaseBackupTask) string {
	if task != nil && normalizeBackupType(task.BackupType) == DatabaseBackupTypeLogicalCustom {
		return "逻辑全量（PostgreSQL Custom），不支持 PITR"
	}
	return "逻辑全量，不支持 PITR"
}

func (uc *UseCase) applyBackupTaskCapacityWarning(ctx context.Context, task *DatabaseBackupTask, vo *DatabaseBackupTaskVO) {
	if uc.capacitySnapshotRepo == nil || task == nil || vo == nil {
		return
	}
	latest, err := uc.capacitySnapshotRepo.LatestInstance(ctx, task.InstanceID)
	if err != nil || latest == nil {
		return
	}
	vo.InstanceCapacitySizeBytes = latest.TotalSizeBytes
	vo.InstanceCapacitySizeText = humanizeBytes(latest.TotalSizeBytes)
	if latest.TotalSizeBytes >= logicalBackupLargeThresholdBytes {
		vo.LargeDataWarning = true
		vo.LargeDataWarningText = fmt.Sprintf("当前实例最近容量约 %s，不建议把每日逻辑全量作为大库主备份方案", humanizeBytes(latest.TotalSizeBytes))
	}
}

func resolveBackupSchemaName(instance *DatabaseInstance, databaseName string) string {
	if strings.TrimSpace(databaseName) != "" {
		return strings.TrimSpace(databaseName)
	}
	if instance == nil {
		return ""
	}
	return strings.TrimSpace(instance.DefaultDatabase)
}

func buildBackupSuccessMessage(fileName string, fileSize int64) string {
	if strings.TrimSpace(fileName) == "" {
		return backupSuccessMessage
	}
	if fileSize > 0 {
		return fmt.Sprintf("%s，文件 %s，大小 %d 字节", backupSuccessMessage, strings.TrimSpace(fileName), fileSize)
	}
	return fmt.Sprintf("%s，文件 %s", backupSuccessMessage, strings.TrimSpace(fileName))
}

func backupTaskLastStatusText(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "-"
	}
	return BackupStatusText(status)
}
