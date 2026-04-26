package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	restoreRunningMessage = "恢复演练执行中"
	restoreSuccessMessage = "恢复演练完成"
	restoreCommandTimeout = 2 * time.Hour
)

func (uc *UseCase) ListRestoreJobs(ctx context.Context, req *DatabaseRestoreJobListRequest) ([]*DatabaseRestoreJobVO, int64, error) {
	if uc.restoreJobRepo == nil {
		return nil, 0, fmt.Errorf("恢复演练仓库未配置")
	}
	normalizeRestoreJobListRequest(req)
	items, total, err := uc.restoreJobRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	uc.reconcileStaleRestoreJobs(ctx, items)
	sourceNames, targetNames, targetEnvs := uc.loadRestoreJobInstanceMeta(ctx, items)
	list := make([]*DatabaseRestoreJobVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toRestoreJobVO(item, sourceNames[item.SourceInstanceID], targetNames[item.TargetInstanceID], targetEnvs[item.TargetInstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) RunRestoreDryRun(ctx context.Context, backupRecordID uint, req *DatabaseRestoreDryRunRequest, operator QueryOperator) (*DatabaseRestoreJobVO, error) {
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	if uc.restoreJobRepo == nil {
		return nil, fmt.Errorf("恢复演练仓库未配置")
	}
	if uc.instanceRepo == nil {
		return nil, fmt.Errorf("实例仓库未配置")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	if req == nil || req.TargetInstanceID == 0 {
		return nil, fmt.Errorf("请选择恢复演练目标实例")
	}

	record, err := uc.backupRecordRepo.GetByID(ctx, backupRecordID)
	if err != nil {
		return nil, fmt.Errorf("备份记录不存在")
	}
	source, err := uc.instanceRepo.GetByID(ctx, record.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("来源数据库实例不存在")
	}
	target, err := uc.instanceRepo.GetByID(ctx, req.TargetInstanceID)
	if err != nil {
		return nil, fmt.Errorf("目标数据库实例不存在")
	}

	if err := validateRestoreDryRunRecord(record); err != nil {
		return nil, err
	}
	if err := validateRestoreDryRunTarget(source, target); err != nil {
		return nil, err
	}

	credential, err := uc.credentialResolver(ctx, target.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("目标实例凭据不存在")
	}
	databaseName, err := resolveRestoreDatabaseName(target, credential)
	if err != nil {
		return nil, err
	}

	strategy := normalizeRestoreStrategy(req.RestoreStrategy)
	if err := validateRestoreStrategy(target.DBType, record.BackupType, strategy); err != nil {
		return nil, err
	}
	spec, err := buildRestoreCommandSpec(target, credential, databaseName, record.BackupType, strategy)
	if err != nil {
		return nil, err
	}
	restoreLockKey := buildRestoreRunKey(target.ID, databaseName)
	if err := uc.acquireRestoreRun(restoreLockKey); err != nil {
		return nil, err
	}
	releaseRestoreLock := true
	defer func() {
		if releaseRestoreLock {
			uc.releaseRestoreRun(restoreLockKey)
		}
	}()

	startedAt := time.Now()
	mode := normalizeRestoreMode(req.RestoreMode)
	if mode != DatabaseRestoreModeDryRun {
		return nil, fmt.Errorf("当前仅支持 dry_run 恢复演练")
	}
	job := &DatabaseRestoreJob{
		BackupRecordID:   record.ID,
		SourceInstanceID: record.InstanceID,
		TargetInstanceID: target.ID,
		RestoreMode:      mode,
		RestoreStrategy:  strategy,
		Status:           DatabaseBackupStatusRunning,
		FileName:         record.FileName,
		FileSize:         record.FileSize,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 100),
		StartedAt:        &startedAt,
		ErrorMessage:     restoreRunningMessage,
	}
	if err := uc.restoreJobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建恢复演练记录失败: %w", err)
	}

	audit, auditErr := uc.startRestoreAudit(ctx, target, record, job, databaseName, operator)
	if auditErr != nil {
		finishedAt := time.Now()
		uc.finishRestoreJob(ctx, job, DatabaseBackupStatusFailed, startedAt, finishedAt, auditErr.Error())
		return nil, auditErr
	}

	vo := uc.toRestoreJobVO(job, source.Name, target.Name, target.Environment)
	go uc.executeRestoreDryRunJob(job, audit, target, record, spec, databaseName, startedAt, restoreLockKey)
	releaseRestoreLock = false
	return vo, nil
}

func (uc *UseCase) executeRestoreDryRunJob(job *DatabaseRestoreJob, audit *DatabaseQueryAudit, target *DatabaseInstance, record *DatabaseBackupRecord, spec *restoreCommandSpec, databaseName string, startedAt time.Time, restoreLockKey string) {
	defer uc.releaseRestoreRun(restoreLockKey)

	runCtx, cancel := context.WithTimeout(context.Background(), restoreCommandTimeout)
	defer cancel()

	runErr := runRestoreCommand(runCtx, spec, record.FilePath)
	finishedAt := time.Now()
	status := DatabaseBackupStatusSuccess
	message := buildRestoreSuccessMessage(record.FileName, target.Name, databaseName)
	if runErr != nil {
		status = DatabaseBackupStatusFailed
		message = runErr.Error()
	}
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer updateCancel()
	uc.finishRestoreJob(updateCtx, job, status, startedAt, finishedAt, message)
	uc.finishRestoreAudit(updateCtx, audit, restoreAuditStatus(status), finishedAt.Sub(startedAt).Milliseconds(), message)
}

func (uc *UseCase) reconcileStaleRestoreJobs(ctx context.Context, items []*DatabaseRestoreJob) {
	if uc.restoreJobRepo == nil {
		return
	}
	now := time.Now()
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Status) != DatabaseBackupStatusRunning || item.StartedAt == nil {
			continue
		}
		if item.StartedAt.After(uc.startedAt) && now.Sub(*item.StartedAt) <= restoreCommandTimeout {
			continue
		}
		item.Status = DatabaseBackupStatusFailed
		item.FinishedAt = &now
		item.DurationMs = now.Sub(*item.StartedAt).Milliseconds()
		item.ErrorMessage = trimText("恢复演练进程已中断、服务已重启或超过 2 小时未完成，已自动标记失败", 500)
		_ = uc.restoreJobRepo.Update(ctx, item)
	}
}

func buildRestoreRunKey(targetInstanceID uint, databaseName string) string {
	return fmt.Sprintf("%d:%s", targetInstanceID, strings.ToLower(strings.TrimSpace(databaseName)))
}

func (uc *UseCase) acquireRestoreRun(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("恢复演练目标不存在")
	}
	uc.restoreRunMu.Lock()
	defer uc.restoreRunMu.Unlock()
	if uc.restoreRunningTargets == nil {
		uc.restoreRunningTargets = make(map[string]struct{})
	}
	if _, exists := uc.restoreRunningTargets[key]; exists {
		return fmt.Errorf("目标实例恢复演练正在执行中，请稍后重试")
	}
	uc.restoreRunningTargets[key] = struct{}{}
	return nil
}

func (uc *UseCase) releaseRestoreRun(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	uc.restoreRunMu.Lock()
	defer uc.restoreRunMu.Unlock()
	delete(uc.restoreRunningTargets, key)
}

func validateRestoreDryRunRecord(record *DatabaseBackupRecord) error {
	if record == nil {
		return fmt.Errorf("备份记录不存在")
	}
	if strings.TrimSpace(record.Status) != DatabaseBackupStatusSuccess {
		return fmt.Errorf("仅允许从成功备份记录发起恢复演练")
	}
	if !isRestorableBackupType(record.BackupType) {
		return fmt.Errorf("当前仅支持 logical / logical_custom 逻辑备份恢复演练")
	}
	if normalizeBackupStorageType(record.StorageType) != DatabaseBackupStorageLocal {
		return fmt.Errorf("当前仅支持 local 本地备份恢复演练")
	}
	if strings.TrimSpace(record.FilePath) == "" || strings.TrimSpace(record.FileName) == "" {
		return fmt.Errorf("备份文件不存在")
	}
	info, err := os.Stat(record.FilePath)
	if err != nil || info == nil || info.IsDir() {
		return fmt.Errorf("备份文件不存在")
	}
	return nil
}

func validateRestoreDryRunTarget(source, target *DatabaseInstance) error {
	if source == nil {
		return fmt.Errorf("来源数据库实例不存在")
	}
	if target == nil {
		return fmt.Errorf("目标数据库实例不存在")
	}
	if strings.TrimSpace(target.Status) != DatabaseInstanceStatusEnabled {
		return fmt.Errorf("目标数据库实例已禁用")
	}
	if source.ID > 0 && source.ID == target.ID {
		return fmt.Errorf("恢复演练目标实例不能与来源实例相同")
	}
	if isProductionEnvironment(target.Environment) {
		return fmt.Errorf("恢复演练不能选择生产目标实例")
	}
	if !supportsRestoreDryRun(source.DBType) {
		return fmt.Errorf("%s 恢复演练将在后续批次接入", DBTypeText(source.DBType))
	}
	if !supportsRestoreDryRun(target.DBType) {
		return fmt.Errorf("%s 恢复演练将在后续批次接入", DBTypeText(target.DBType))
	}
	if !isRestoreCompatibleType(source.DBType, target.DBType) {
		return fmt.Errorf("来源 %s 与目标 %s 类型不兼容", DBTypeText(source.DBType), DBTypeText(target.DBType))
	}
	return nil
}

func (uc *UseCase) startRestoreAudit(ctx context.Context, target *DatabaseInstance, record *DatabaseBackupRecord, job *DatabaseRestoreJob, databaseName string, operator QueryOperator) (*DatabaseQueryAudit, error) {
	if uc.auditRepo == nil {
		return nil, nil
	}
	sqlText := buildRestoreAuditText(record, job, databaseName)
	audit := &DatabaseQueryAudit{
		InstanceID:     target.ID,
		SchemaName:     resolveBackupSchemaName(target, databaseName),
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionRestoreDryRun,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "RESTORE",
		RiskLevel:      DatabaseQueryRiskHigh,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil, err
	}
	return audit, nil
}

func (uc *UseCase) finishRestoreAudit(ctx context.Context, audit *DatabaseQueryAudit, status string, durationMs int64, message string) {
	if uc.auditRepo == nil || audit == nil || audit.ID == 0 {
		return
	}
	audit.Status = status
	audit.DurationMs = durationMs
	if status == DatabaseQueryStatusFailed {
		audit.ErrorMessage = trimText(message, 500)
	}
	_ = uc.auditRepo.Update(ctx, audit)
}

func (uc *UseCase) finishRestoreJob(ctx context.Context, job *DatabaseRestoreJob, status string, startedAt, finishedAt time.Time, message string) {
	if uc.restoreJobRepo == nil || job == nil || job.ID == 0 {
		return
	}
	job.Status = status
	job.StartedAt = &startedAt
	job.FinishedAt = &finishedAt
	job.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	job.ErrorMessage = trimText(message, 500)
	_ = uc.restoreJobRepo.Update(ctx, job)
}

func (uc *UseCase) loadRestoreJobInstanceMeta(ctx context.Context, items []*DatabaseRestoreJob) (map[uint]string, map[uint]string, map[uint]string) {
	sourceNames := make(map[uint]string)
	targetNames := make(map[uint]string)
	targetEnvs := make(map[uint]string)
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.SourceInstanceID > 0 {
			if _, ok := sourceNames[item.SourceInstanceID]; !ok {
				instance, err := uc.instanceRepo.GetByID(ctx, item.SourceInstanceID)
				if err == nil && instance != nil {
					sourceNames[item.SourceInstanceID] = instance.Name
				}
			}
		}
		if item.TargetInstanceID > 0 {
			if _, ok := targetNames[item.TargetInstanceID]; !ok {
				instance, err := uc.instanceRepo.GetByID(ctx, item.TargetInstanceID)
				if err == nil && instance != nil {
					targetNames[item.TargetInstanceID] = instance.Name
					targetEnvs[item.TargetInstanceID] = instance.Environment
				}
			}
		}
	}
	return sourceNames, targetNames, targetEnvs
}

func (uc *UseCase) toRestoreJobVO(item *DatabaseRestoreJob, sourceName, targetName, targetEnvironment string) *DatabaseRestoreJobVO {
	if item == nil {
		return nil
	}
	return &DatabaseRestoreJobVO{
		ID:                  item.ID,
		BackupRecordID:      item.BackupRecordID,
		SourceInstanceID:    item.SourceInstanceID,
		SourceInstanceName:  sourceName,
		TargetInstanceID:    item.TargetInstanceID,
		TargetInstanceName:  targetName,
		TargetEnvironment:   targetEnvironment,
		RestoreMode:         item.RestoreMode,
		RestoreModeText:     RestoreModeText(item.RestoreMode),
		RestoreStrategy:     normalizeRestoreStrategy(item.RestoreStrategy),
		RestoreStrategyText: RestoreStrategyText(item.RestoreStrategy),
		Status:              item.Status,
		StatusText:          BackupStatusText(item.Status),
		FileName:            item.FileName,
		FileSize:            item.FileSize,
		OperatorID:          item.OperatorID,
		OperatorName:        item.OperatorName,
		StartedAt:           formatTime(item.StartedAt),
		FinishedAt:          formatTime(item.FinishedAt),
		DurationMs:          item.DurationMs,
		Message:             buildRestoreJobMessage(item),
		CreatedAt:           item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:           item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func normalizeRestoreMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return DatabaseRestoreModeDryRun
	}
	return mode
}

func normalizeRestoreStrategy(strategy string) string {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "" {
		return DatabaseRestoreStrategyObjectReplace
	}
	return strategy
}

func RestoreStrategyText(strategy string) string {
	switch normalizeRestoreStrategy(strategy) {
	case DatabaseRestoreStrategyDatabaseClean:
		return "清空目标库后导入"
	case DatabaseRestoreStrategyObjectReplace:
		return "覆盖备份中的对象"
	default:
		return strategy
	}
}

func validateRestoreStrategy(dbType, backupType, strategy string) error {
	strategy = normalizeRestoreStrategy(strategy)
	switch strategy {
	case DatabaseRestoreStrategyObjectReplace:
	case DatabaseRestoreStrategyDatabaseClean:
	default:
		return fmt.Errorf("不支持的恢复目标处理策略: %s", strategy)
	}

	dbType = normalizeDBType(dbType)
	backupType = normalizeBackupType(backupType)
	if strategy == DatabaseRestoreStrategyDatabaseClean {
		switch dbType {
		case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
			return nil
		default:
			return fmt.Errorf("%s 暂不支持清空目标库后导入", DBTypeText(dbType))
		}
	}
	if dbType == DBTypePostgreSQL && backupType == DatabaseBackupTypeLogical {
		return fmt.Errorf("PostgreSQL Plain SQL 备份暂不支持覆盖备份中的对象，请选择清空目标库后导入，或使用 Custom 备份")
	}
	return nil
}

func supportsRestoreDryRun(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeRedis:
		return true
	default:
		return false
	}
}

func isRestorableBackupType(backupType string) bool {
	switch normalizeBackupType(backupType) {
	case DatabaseBackupTypeLogical, DatabaseBackupTypeLogicalCustom:
		return true
	default:
		return false
	}
}

func isRestoreCompatibleType(sourceType, targetType string) bool {
	sourceType = normalizeDBType(sourceType)
	targetType = normalizeDBType(targetType)
	if sourceType == DBTypeRedis || targetType == DBTypeRedis {
		return sourceType == DBTypeRedis && targetType == DBTypeRedis
	}
	if sourceType == DBTypePostgreSQL || targetType == DBTypePostgreSQL {
		return sourceType == DBTypePostgreSQL && targetType == DBTypePostgreSQL
	}
	return isMySQLFamily(sourceType) && isMySQLFamily(targetType)
}

func isMySQLFamily(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB:
		return true
	default:
		return false
	}
}

func isProductionEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production", "prd", "生产", "生产环境":
		return true
	default:
		return false
	}
}

func buildRestoreAuditText(record *DatabaseBackupRecord, job *DatabaseRestoreJob, databaseName string) string {
	recordID := uint(0)
	fileName := ""
	if record != nil {
		recordID = record.ID
		fileName = strings.TrimSpace(record.FileName)
	}
	targetID := uint(0)
	if job != nil {
		targetID = job.TargetInstanceID
	}
	strategyText := ""
	if job != nil {
		strategyText = RestoreStrategyText(job.RestoreStrategy)
	}
	if strings.TrimSpace(databaseName) == "" {
		return fmt.Sprintf("RESTORE DRY RUN BACKUP RECORD #%d FILE %s TARGET INSTANCE #%d STRATEGY %s", recordID, fileName, targetID, strategyText)
	}
	return fmt.Sprintf("RESTORE DRY RUN BACKUP RECORD #%d FILE %s TARGET INSTANCE #%d DATABASE %s STRATEGY %s", recordID, fileName, targetID, strings.TrimSpace(databaseName), strategyText)
}

func buildRestoreSuccessMessage(fileName, targetName, databaseName string) string {
	message := restoreSuccessMessage
	if strings.TrimSpace(fileName) != "" {
		message += "，文件 " + strings.TrimSpace(fileName)
	}
	if strings.TrimSpace(targetName) != "" {
		message += "，目标 " + strings.TrimSpace(targetName)
	}
	if strings.TrimSpace(databaseName) != "" {
		message += " / " + strings.TrimSpace(databaseName)
	}
	return message
}

func buildRestoreJobMessage(item *DatabaseRestoreJob) string {
	if item == nil {
		return ""
	}
	if message := strings.TrimSpace(item.ErrorMessage); message != "" {
		return message
	}
	switch strings.TrimSpace(item.Status) {
	case DatabaseBackupStatusSuccess:
		return restoreSuccessMessage
	case DatabaseBackupStatusRunning:
		return restoreRunningMessage
	case DatabaseBackupStatusPending:
		return "恢复演练等待执行"
	case DatabaseBackupStatusFailed:
		return "恢复演练失败"
	default:
		return ""
	}
}

func restoreAuditStatus(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseBackupStatusSuccess:
		return DatabaseQueryStatusSuccess
	case DatabaseBackupStatusFailed:
		return DatabaseQueryStatusFailed
	default:
		return DatabaseQueryStatusPending
	}
}
