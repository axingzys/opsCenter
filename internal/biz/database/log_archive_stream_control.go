package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (uc *UseCase) GetLogArchiveStreamStatus(ctx context.Context, streamID uint) (*DatabaseLogArchiveStreamVO, error) {
	stream, err := uc.getLogArchiveStreamForControl(ctx, streamID)
	if err != nil {
		return nil, err
	}
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) StartLogArchiveStream(ctx context.Context, streamID uint, req *DatabaseLogArchiveStreamControlRequest) (*DatabaseLogArchiveStreamVO, error) {
	stream, err := uc.getLogArchiveStreamForControl(ctx, streamID)
	if err != nil {
		return nil, err
	}
	if err := uc.validateLogArchiveStreamDaemonTarget(ctx, stream); err != nil {
		return nil, err
	}
	if !stream.Enabled || stream.Status == DatabaseLogArchiveStreamStatusDisabled {
		return nil, fmt.Errorf("日志归档流已禁用，请先启用后再启动")
	}
	hostID := stream.RunnerHostID
	if req != nil && req.RunnerHostID > 0 {
		hostID = req.RunnerHostID
	}
	host, err := uc.validateRequiredLogArchiveRunnerHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	archiveMode := stream.ArchiveMode
	if req != nil && strings.TrimSpace(req.ArchiveMode) != "" {
		archiveMode = req.ArchiveMode
	}
	archiveMode = daemonArchiveModeForStart(archiveMode)
	applyLogArchiveStreamStartState(stream, host.ID, archiveMode, archiveEngineForDaemonMode(stream.ArchiveEngine, archiveMode))
	if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
		return nil, err
	}
	uc.recordLogArchiveStreamStateEvent(ctx, stream, DatabaseLogArchiveEventLevelInfo, "日志归档流已启动，等待 Runner Agent 接管")
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) PauseLogArchiveStream(ctx context.Context, streamID uint, req *DatabaseLogArchiveStreamControlRequest) (*DatabaseLogArchiveStreamVO, error) {
	stream, err := uc.getLogArchiveStreamForControl(ctx, streamID)
	if err != nil {
		return nil, err
	}
	reason := ""
	if req != nil {
		reason = req.Reason
	}
	applyLogArchiveStreamPauseState(stream, reason, time.Now())
	if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
		return nil, err
	}
	uc.recordLogArchiveStreamStateEvent(ctx, stream, DatabaseLogArchiveEventLevelWarning, stream.LastError)
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) ResumeLogArchiveStream(ctx context.Context, streamID uint, req *DatabaseLogArchiveStreamControlRequest) (*DatabaseLogArchiveStreamVO, error) {
	stream, err := uc.getLogArchiveStreamForControl(ctx, streamID)
	if err != nil {
		return nil, err
	}
	if err := uc.validateLogArchiveStreamDaemonTarget(ctx, stream); err != nil {
		return nil, err
	}
	if !stream.Enabled || stream.Status == DatabaseLogArchiveStreamStatusDisabled {
		return nil, fmt.Errorf("日志归档流已禁用，请先启用后再恢复")
	}
	hostID := stream.RunnerHostID
	if req != nil && req.RunnerHostID > 0 {
		hostID = req.RunnerHostID
	}
	host, err := uc.validateRequiredLogArchiveRunnerHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	archiveMode := stream.ArchiveMode
	if req != nil && strings.TrimSpace(req.ArchiveMode) != "" {
		archiveMode = req.ArchiveMode
	}
	archiveMode = daemonArchiveModeForStart(archiveMode)
	applyLogArchiveStreamStartState(stream, host.ID, archiveMode, archiveEngineForDaemonMode(stream.ArchiveEngine, archiveMode))
	if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
		return nil, err
	}
	uc.recordLogArchiveStreamStateEvent(ctx, stream, DatabaseLogArchiveEventLevelInfo, "日志归档流已恢复，等待 Runner Agent 接管")
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) StopLogArchiveStream(ctx context.Context, streamID uint, req *DatabaseLogArchiveStreamControlRequest) (*DatabaseLogArchiveStreamVO, error) {
	stream, err := uc.getLogArchiveStreamForControl(ctx, streamID)
	if err != nil {
		return nil, err
	}
	reason := ""
	if req != nil {
		reason = req.Reason
	}
	applyLogArchiveStreamStopState(stream, reason)
	if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
		return nil, err
	}
	uc.recordLogArchiveStreamStateEvent(ctx, stream, DatabaseLogArchiveEventLevelWarning, stream.LastError)
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) getLogArchiveStreamForControl(ctx context.Context, streamID uint) (*DatabaseLogArchiveStream, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	if streamID == 0 {
		return nil, fmt.Errorf("日志归档流ID不能为空")
	}
	stream, err := uc.logArchiveStreamRepo.GetByID(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("日志归档流不存在")
	}
	return stream, nil
}

func (uc *UseCase) validateLogArchiveStreamDaemonTarget(ctx context.Context, stream *DatabaseLogArchiveStream) error {
	if stream == nil {
		return fmt.Errorf("日志归档流不存在")
	}
	if normalizeArchiveType(stream.ArchiveType) != DatabaseArchiveTypeBinlog {
		return fmt.Errorf("P2.6 第一版仅支持 MySQL/MariaDB binlog 长期归档流")
	}
	sourceInstanceID := stream.SourceInstanceID
	if sourceInstanceID == 0 {
		sourceInstanceID = stream.InstanceID
	}
	if uc.instanceRepo == nil {
		return fmt.Errorf("数据库实例仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, sourceInstanceID)
	if err != nil {
		return fmt.Errorf("日志来源实例不存在")
	}
	dbType := normalizeDBType(instance.DBType)
	if dbType != DBTypeMySQL && dbType != DBTypeMariaDB {
		return fmt.Errorf("%s 暂不支持 P2.6 binlog 长期归档流", DBTypeText(instance.DBType))
	}
	return nil
}

func (uc *UseCase) validateRequiredLogArchiveRunnerHost(ctx context.Context, runnerHostID uint) (*DatabaseRunnerHost, error) {
	if runnerHostID == 0 {
		return nil, fmt.Errorf("请选择 Runner 主机")
	}
	host, err := uc.validateLogArchiveRunnerHost(ctx, runnerHostID)
	if err != nil {
		return nil, err
	}
	return host, nil
}

func (uc *UseCase) validateLogArchiveRunnerHost(ctx context.Context, runnerHostID uint) (*DatabaseRunnerHost, error) {
	if runnerHostID == 0 {
		return nil, nil
	}
	if uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("Runner 主机仓库未配置")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	return host, nil
}

func (uc *UseCase) archiveRunnerHostName(ctx context.Context, runnerHostID uint) string {
	if uc == nil || uc.runnerHostRepo == nil || runnerHostID == 0 {
		return ""
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil || host == nil {
		return ""
	}
	return host.Name
}

func (uc *UseCase) recordLogArchiveStreamStateEvent(ctx context.Context, stream *DatabaseLogArchiveStream, level, message string) {
	if stream == nil {
		return
	}
	uc.recordLogArchiveEvent(ctx, &DatabaseLogArchiveEvent{
		StreamID:          stream.ID,
		InstanceID:        stream.InstanceID,
		SourceInstanceID:  stream.SourceInstanceID,
		RunnerHostID:      stream.RunnerHostID,
		RunnerID:          stream.LeaseOwner,
		EventType:         DatabaseLogArchiveEventStateChanged,
		Level:             level,
		Message:           message,
		FileName:          stream.LastArchiveName,
		CursorFile:        stream.CursorFile,
		CursorPos:         stream.CursorPos,
		ActiveFile:        stream.ActiveFile,
		ArchiveLagSeconds: stream.ArchiveLagSeconds,
	})
}

func daemonArchiveModeForStart(value string) string {
	switch normalizeArchiveMode(value) {
	case DatabaseArchiveModePolling:
		return DatabaseArchiveModePolling
	case DatabaseArchiveModeStreaming:
		return DatabaseArchiveModeStreaming
	default:
		return DatabaseArchiveModePolling
	}
}

func archiveEngineForDaemonMode(current, archiveMode string) string {
	current = strings.ToLower(strings.TrimSpace(current))
	if current != "" && !strings.HasPrefix(current, "external") {
		return trimText(current, 60)
	}
	switch normalizeArchiveMode(archiveMode) {
	case DatabaseArchiveModeStreaming:
		return "mysqlbinlog_streaming"
	default:
		return "mysqlbinlog_polling"
	}
}

func applyLogArchiveStreamStartState(stream *DatabaseLogArchiveStream, runnerHostID uint, archiveMode, archiveEngine string) {
	if stream == nil {
		return
	}
	if runnerHostID > 0 {
		stream.RunnerHostID = runnerHostID
	}
	stream.ArchiveMode = daemonArchiveModeForStart(archiveMode)
	stream.ArchiveEngine = archiveEngineForDaemonMode(archiveEngine, stream.ArchiveMode)
	stream.DesiredState = DatabaseLogArchiveDesiredStateRunning
	stream.DaemonStatus = DatabaseLogArchiveDaemonStatusStarting
	stream.Status = DatabaseLogArchiveStreamStatusPending
	stream.PausedAt = nil
	stream.PausedReason = ""
	stream.LeaseOwner = ""
	stream.LeaseExpiresAt = nil
	stream.LastError = "等待 Runner Agent 接管归档流"
}

func applyLogArchiveStreamPauseState(stream *DatabaseLogArchiveStream, reason string, pausedAt time.Time) {
	if stream == nil {
		return
	}
	if pausedAt.IsZero() {
		pausedAt = time.Now()
	}
	reason = trimText(strings.TrimSpace(reason), 500)
	if reason == "" {
		reason = "手动暂停"
	}
	stream.DesiredState = DatabaseLogArchiveDesiredStatePaused
	stream.DaemonStatus = DatabaseLogArchiveDaemonStatusPaused
	stream.Status = DatabaseLogArchiveStreamStatusPaused
	stream.PausedAt = &pausedAt
	stream.PausedReason = reason
	stream.LeaseOwner = ""
	stream.LeaseExpiresAt = nil
	stream.LastError = "归档流已暂停: " + reason
}

func applyLogArchiveStreamStopState(stream *DatabaseLogArchiveStream, reason string) {
	if stream == nil {
		return
	}
	reason = trimText(strings.TrimSpace(reason), 500)
	stream.DesiredState = DatabaseLogArchiveDesiredStateStopped
	stream.DaemonStatus = DatabaseLogArchiveDaemonStatusStopped
	if stream.Enabled {
		stream.Status = DatabaseLogArchiveStreamStatusPending
	} else {
		stream.Status = DatabaseLogArchiveStreamStatusDisabled
	}
	stream.PausedAt = nil
	stream.PausedReason = ""
	stream.LeaseOwner = ""
	stream.LeaseExpiresAt = nil
	if reason != "" {
		stream.LastError = "归档流已停止: " + reason
	} else {
		stream.LastError = "归档流已停止"
	}
}
