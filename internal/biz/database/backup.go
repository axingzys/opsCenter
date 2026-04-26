package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	backupRunningMessage = "逻辑备份执行中"
	backupSuccessMessage = "逻辑备份完成"
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
		list = append(list, uc.toBackupTaskVO(item, instanceNames[item.InstanceID], instanceTypes[item.InstanceID]))
	}
	return list, total, nil
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
		InstanceID:    req.InstanceID,
		Name:          trimText(strings.TrimSpace(req.Name), 120),
		BackupType:    normalizeBackupType(req.BackupType),
		Schedule:      strings.TrimSpace(req.Schedule),
		StorageType:   normalizeBackupStorageType(req.StorageType),
		StorageConfig: trimText(strings.TrimSpace(req.StorageConfig), 2000),
		RetentionDays: normalizeBackupRetentionDays(req.RetentionDays, policy.DefaultRetentionDays),
		Enabled:       req.Enabled,
	}
	if err := uc.backupTaskRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBackupTaskVO(item, instance.Name, instance.DBType), nil
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
	item.Enabled = req.Enabled

	if err := uc.backupTaskRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBackupTaskVO(item, instance.Name, instance.DBType), nil
}

func (uc *UseCase) DeleteBackupTask(ctx context.Context, id uint) error {
	if _, err := uc.getBackupTask(ctx, id); err != nil {
		return err
	}
	return uc.backupTaskRepo.Delete(ctx, id)
}

func (uc *UseCase) RunBackupTask(ctx context.Context, id uint, operator QueryOperator) (*DatabaseBackupRunVO, error) {
	return uc.runBackupTask(ctx, id, operator, DatabaseBackupTriggerManual)
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

	taskNames := uc.loadBackupTaskNames(ctx, items)
	instanceNames, _ := uc.loadBackupInstanceMetaByRecord(ctx, items)
	list := make([]*DatabaseBackupRecordVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toBackupRecordVO(item, taskNames[item.TaskID], instanceNames[item.InstanceID]))
	}
	return list, total, nil
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
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	filePath, err := uc.secureBackupFilePath(ctx, record.FilePath)
	if err != nil {
		err = fmt.Errorf("备份文件路径无效: %w", err)
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		err = fmt.Errorf("备份文件不存在")
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}
	if info.IsDir() {
		err = fmt.Errorf("备份文件不存在")
		uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusFailed, err.Error())
		return nil, err
	}

	uc.recordBackupDownloadAudit(ctx, instance, record, operator, DatabaseQueryStatusSuccess, "")
	return &DatabaseBackupDownloadVO{
		FilePath:    filePath,
		FileName:    record.FileName,
		ContentType: detectBackupContentType(record.FileName),
	}, nil
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

func (uc *UseCase) finishBackupRecord(ctx context.Context, record *DatabaseBackupRecord, status string, startedAt, finishedAt time.Time, errorMessage string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	record.Status = status
	record.StartedAt = &startedAt
	record.FinishedAt = &finishedAt
	record.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	record.ErrorMessage = trimText(errorMessage, 500)
	_ = uc.backupRecordRepo.Update(ctx, record)
}

func (uc *UseCase) finishBackupRecordSuccess(ctx context.Context, record *DatabaseBackupRecord, startedAt, finishedAt time.Time, durationMs, fileSize int64, message string) {
	if uc.backupRecordRepo == nil || record == nil || record.ID == 0 {
		return
	}
	record.Status = DatabaseBackupStatusSuccess
	record.StartedAt = &startedAt
	record.FinishedAt = &finishedAt
	record.DurationMs = durationMs
	record.FileSize = fileSize
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

func (uc *UseCase) toBackupTaskVO(item *DatabaseBackupTask, instanceName, instanceDBType string) *DatabaseBackupTaskVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupTaskVO{
		ID:                 item.ID,
		InstanceID:         item.InstanceID,
		InstanceName:       instanceName,
		InstanceDBType:     instanceDBType,
		InstanceDBTypeText: DBTypeText(instanceDBType),
		Name:               item.Name,
		BackupType:         item.BackupType,
		BackupTypeText:     BackupTypeText(item.BackupType),
		Schedule:           item.Schedule,
		StorageType:        item.StorageType,
		StorageTypeText:    BackupStorageTypeText(item.StorageType),
		StorageConfig:      item.StorageConfig,
		RetentionDays:      item.RetentionDays,
		Enabled:            item.Enabled,
		LastRunAt:          formatTime(item.LastRunAt),
		LastStatus:         item.LastStatus,
		LastStatusText:     backupTaskLastStatusText(item.LastStatus),
		LastMessage:        item.LastMessage,
		CreatedAt:          item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:          item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) toBackupRecordVO(item *DatabaseBackupRecord, taskName, instanceName string) *DatabaseBackupRecordVO {
	if item == nil {
		return nil
	}
	return &DatabaseBackupRecordVO{
		ID:              item.ID,
		TaskID:          item.TaskID,
		TaskName:        taskName,
		InstanceID:      item.InstanceID,
		InstanceName:    instanceName,
		TriggerType:     item.TriggerType,
		TriggerTypeText: BackupTriggerTypeText(item.TriggerType),
		BackupType:      item.BackupType,
		BackupTypeText:  BackupTypeText(item.BackupType),
		StorageType:     item.StorageType,
		StorageTypeText: BackupStorageTypeText(item.StorageType),
		Status:          item.Status,
		StatusText:      BackupStatusText(item.Status),
		FileName:        item.FileName,
		FileSize:        item.FileSize,
		StartedAt:       formatTime(item.StartedAt),
		FinishedAt:      formatTime(item.FinishedAt),
		DurationMs:      item.DurationMs,
		Message:         buildBackupRecordMessage(item),
		CreatedAt:       item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:       item.UpdatedAt.Format("2006-01-02 15:04:05"),
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

func normalizeBackupStorageType(storageType string) string {
	storageType = strings.ToLower(strings.TrimSpace(storageType))
	if storageType == "" {
		return DatabaseBackupStorageLocal
	}
	return storageType
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
