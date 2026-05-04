package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	monitormodel "github.com/ydcloud-dy/opshub/plugins/monitor/model"
	monitorservice "github.com/ydcloud-dy/opshub/plugins/monitor/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func dispatchBackupSchedulerAlert(ctx context.Context, db *gorm.DB, notice *dbbiz.BackupSchedulerNotice) error {
	if db == nil || notice == nil || notice.Error == nil {
		return nil
	}

	message := monitorservice.AlertMessage{
		AlertType:      mapBackupAlertType(notice.Operation),
		ResourceType:   "database_backup",
		ResourceID:     backupTaskID(notice),
		ResourceName:   backupTaskName(notice),
		ResourceTarget: backupResourceTarget(notice),
		Metric:         "backup_failed",
		Severity:       mapBackupAlertSeverity(notice.Operation),
		Status:         "failed",
		Message:        buildBackupAlertMessage(notice),
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
	}

	channelSummary, err := dispatchDatabaseAlertMessage(ctx, db, message)
	if err != nil {
		return err
	}

	appLogger.Info("数据库备份失败通知已发送",
		zap.String("operation", notice.Operation),
		zap.String("channels", channelSummary),
		zap.Uint("taskID", backupTaskID(notice)),
	)
	return nil
}

func dispatchDatabaseBackupAlertNotice(ctx context.Context, db *gorm.DB, notice *dbbiz.DatabaseBackupAlertNotice) (string, error) {
	if db == nil || notice == nil || notice.Candidate == nil {
		return "", nil
	}
	candidate := notice.Candidate
	status := candidate.Severity
	messageText := candidate.Message
	if notice.Resolved {
		status = "normal"
		if !strings.HasPrefix(messageText, "已恢复") {
			messageText = "已恢复：" + messageText
		}
	}
	var ruleID *uint
	var channelIDs []uint
	if notice.Rule != nil {
		id := notice.Rule.ID
		ruleID = &id
		channelIDs = notice.Rule.ChannelIDs
	}
	message := monitorservice.AlertMessage{
		AlertType:      candidate.AlertType,
		ResourceType:   "database_backup",
		ResourceID:     candidate.ResourceID,
		ResourceName:   candidate.ResourceName,
		ResourceTarget: candidate.ResourceTarget,
		Metric:         candidate.Metric,
		Severity:       candidate.Severity,
		Status:         status,
		Message:        messageText,
		AlertRuleID:    ruleID,
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
	}
	return dispatchDatabaseAlertMessage(ctx, db, message, channelIDs...)
}

func dispatchDatabaseAlertMessage(ctx context.Context, db *gorm.DB, message monitorservice.AlertMessage, channelIDs ...uint) (string, error) {
	if db == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dispatcher := monitorservice.NewAlertDispatcher(db)
	channelSummary, err := dispatcher.Dispatch(message, channelIDs...)
	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}
	writeDatabaseAlertLog(ctx, db, message, status, channelSummary, errorMsg)
	return channelSummary, err
}

func writeDatabaseAlertLog(ctx context.Context, db *gorm.DB, message monitorservice.AlertMessage, status, channel, errorMsg string) {
	if db == nil {
		return
	}
	resourceName := firstNonEmptyString(message.ResourceName, message.Domain, "database")
	resourceTarget := firstNonEmptyString(message.ResourceTarget, message.Domain, resourceName)
	alertLog := &monitormodel.AlertLog{
		AlertType:       message.AlertType,
		ResourceType:    "database_backup",
		ResourceID:      message.ResourceID,
		ResourceName:    resourceName,
		ResourceTarget:  resourceTarget,
		Metric:          message.Metric,
		Severity:        message.Severity,
		CurrentValue:    message.CurrentValue,
		ThresholdValue:  message.ThresholdValue,
		AlertRuleID:     message.AlertRuleID,
		DomainMonitorID: 0,
		Domain:          resourceTarget,
		Status:          status,
		Message:         message.Message,
		ChannelType:     channel,
		ErrorMsg:        errorMsg,
		SentAt:          time.Now(),
	}
	_ = db.WithContext(ctx).Create(alertLog).Error
}

func mapBackupAlertType(operation string) string {
	switch strings.TrimSpace(operation) {
	case "retention_cleanup":
		return "database_backup_cleanup_failed"
	default:
		return "database_backup_schedule_failed"
	}
}

func mapBackupAlertSeverity(operation string) string {
	switch strings.TrimSpace(operation) {
	case "retention_cleanup":
		return "warning"
	default:
		return "critical"
	}
}

func buildBackupAlertMessage(notice *dbbiz.BackupSchedulerNotice) string {
	if notice == nil {
		return ""
	}
	prefix := strings.TrimSpace(notice.Message)
	if prefix == "" {
		prefix = "数据库备份任务执行失败"
	}
	if notice.Task != nil && strings.TrimSpace(notice.Task.Name) != "" {
		prefix = prefix + " [" + strings.TrimSpace(notice.Task.Name) + "]"
	}
	if notice.Instance != nil && strings.TrimSpace(notice.Instance.Name) != "" {
		prefix = prefix + " @" + strings.TrimSpace(notice.Instance.Name)
	}
	if notice.Error == nil {
		return prefix
	}
	return prefix + "，原因：" + notice.Error.Error()
}

func backupTaskID(notice *dbbiz.BackupSchedulerNotice) uint {
	if notice != nil && notice.Task != nil {
		return notice.Task.ID
	}
	return 0
}

func backupTaskName(notice *dbbiz.BackupSchedulerNotice) string {
	if notice != nil && notice.Task != nil && strings.TrimSpace(notice.Task.Name) != "" {
		return strings.TrimSpace(notice.Task.Name)
	}
	return "database-backup-task"
}

func backupResourceTarget(notice *dbbiz.BackupSchedulerNotice) string {
	if notice == nil || notice.Instance == nil {
		return ""
	}
	if strings.TrimSpace(notice.Instance.Name) != "" {
		return fmt.Sprintf("%s (%s:%d)", strings.TrimSpace(notice.Instance.Name), strings.TrimSpace(notice.Instance.Host), notice.Instance.Port)
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(notice.Instance.Host), notice.Instance.Port)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
