package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	monitorservice "github.com/ydcloud-dy/opshub/plugins/monitor/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func dispatchBackupSchedulerAlert(_ context.Context, db *gorm.DB, notice *dbbiz.BackupSchedulerNotice) error {
	if db == nil || notice == nil || notice.Error == nil {
		return nil
	}

	dispatcher := monitorservice.NewAlertDispatcher(db)
	message := monitorservice.AlertMessage{
		AlertType:      mapBackupAlertType(notice.Operation),
		ResourceType:   "database_backup_task",
		ResourceID:     backupTaskID(notice),
		ResourceName:   backupTaskName(notice),
		ResourceTarget: backupResourceTarget(notice),
		Metric:         "backup",
		Severity:       mapBackupAlertSeverity(notice.Operation),
		Status:         "failed",
		Message:        buildBackupAlertMessage(notice),
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
	}

	channelSummary, err := dispatcher.Dispatch(message)
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
