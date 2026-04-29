package database

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRunnerAgentLeaseTTLSeconds          = 90
	defaultRunnerAgentHeartbeatIntervalSeconds = 30
	maxRunnerAgentLeaseTTLSeconds              = 600
)

type DatabaseRunnerAgentHeartbeatRequest struct {
	Version        string `json:"version" binding:"omitempty,max=120"`
	Status         string `json:"status" binding:"omitempty,max=30"`
	Message        string `json:"message" binding:"omitempty,max=500"`
	RunningStreams int    `json:"runningStreams" binding:"omitempty,min=0,max=1000"`
}

type DatabaseRunnerAgentCheckpointRequest struct {
	DaemonStatus        string `json:"daemonStatus" binding:"omitempty,max=30"`
	CursorFile          string `json:"cursorFile" binding:"omitempty,max=255"`
	CursorPos           int64  `json:"cursorPos"`
	CursorGTIDSet       string `json:"cursorGtidSet"`
	ActiveFile          string `json:"activeFile" binding:"omitempty,max=255"`
	LastSourceFile      string `json:"lastSourceFile" binding:"omitempty,max=255"`
	LastSourcePos       int64  `json:"lastSourcePos"`
	LastEventTime       string `json:"lastEventTime"`
	ArchiveLagSeconds   int    `json:"archiveLagSeconds" binding:"omitempty,min=0"`
	ConsecutiveFailures int    `json:"consecutiveFailures" binding:"omitempty,min=0,max=1000000"`
	LastError           string `json:"lastError" binding:"omitempty,max=1000"`
	LeaseTTLSeconds     int    `json:"leaseTtlSeconds" binding:"omitempty,min=1,max=600"`
	ReleaseLease        bool   `json:"releaseLease"`
}

type DatabaseRunnerAgentHeartbeatVO struct {
	RunnerHostID             uint   `json:"runnerHostId"`
	RunnerID                 string `json:"runnerId"`
	Status                   string `json:"status"`
	StatusText               string `json:"statusText"`
	LastHeartbeatAt          string `json:"lastHeartbeatAt"`
	LeaseTTLSeconds          int    `json:"leaseTtlSeconds"`
	HeartbeatIntervalSeconds int    `json:"heartbeatIntervalSeconds"`
}

type DatabaseRunnerAgentStreamListVO struct {
	RunnerHostID             uint                         `json:"runnerHostId"`
	RunnerID                 string                       `json:"runnerId"`
	LeaseTTLSeconds          int                          `json:"leaseTtlSeconds"`
	HeartbeatIntervalSeconds int                          `json:"heartbeatIntervalSeconds"`
	Streams                  []*DatabaseRunnerAgentStream `json:"streams"`
}

type DatabaseRunnerAgentStream struct {
	Stream            *DatabaseLogArchiveStreamVO      `json:"stream"`
	SourceInstance    *DatabaseRunnerAgentSource       `json:"sourceInstance"`
	Runner            *DatabaseRunnerAgentRunnerConfig `json:"runner"`
	LeaseOwner        string                           `json:"leaseOwner"`
	LeaseExpiresAt    string                           `json:"leaseExpiresAt"`
	CheckpointURL     string                           `json:"checkpointUrl"`
	LogArchivePostURL string                           `json:"logArchivePostUrl"`
}

type DatabaseRunnerAgentSource struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	DBType          string `json:"dbType"`
	DBTypeText      string `json:"dbTypeText"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	DefaultDatabase string `json:"defaultDatabase"`
}

type DatabaseRunnerAgentRunnerConfig struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	RunnerType       string `json:"runnerType"`
	WorkDir          string `json:"workDir"`
	StorageMountPath string `json:"storageMountPath"`
}

func (uc *UseCase) RunnerAgentHeartbeat(ctx context.Context, runnerID, auth string, req *DatabaseRunnerAgentHeartbeatRequest) (*DatabaseRunnerAgentHeartbeatVO, error) {
	host, err := uc.authenticateRunnerAgent(ctx, runnerID, auth)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	applyRunnerAgentHeartbeat(host, req, now)
	if err := uc.runnerHostRepo.Update(ctx, host); err != nil {
		return nil, err
	}
	if host.Status == DatabaseRunnerHostStatusFailed {
		uc.recordLogArchiveEvent(ctx, &DatabaseLogArchiveEvent{
			RunnerHostID: host.ID,
			RunnerID:     runnerIDForHost(host),
			EventType:    DatabaseLogArchiveEventRunnerHeartbeat,
			Level:        DatabaseLogArchiveEventLevelError,
			Message:      host.LastError,
			OccurredAt:   &now,
		})
	}
	return runnerAgentHeartbeatVO(host, runnerIDForHost(host)), nil
}

func (uc *UseCase) RunnerAgentListLogArchiveStreams(ctx context.Context, runnerID, auth, basePath string) (*DatabaseRunnerAgentStreamListVO, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	host, err := uc.authenticateRunnerAgent(ctx, runnerID, auth)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	applyRunnerAgentHeartbeat(host, &DatabaseRunnerAgentHeartbeatRequest{Status: DatabaseRunnerHostStatusOnline}, now)
	if err := uc.runnerHostRepo.Update(ctx, host); err != nil {
		return nil, err
	}
	candidates, err := uc.logArchiveStreamRepo.ListRunnableForRunner(ctx, host.ID)
	if err != nil {
		return nil, err
	}
	resolvedRunnerID := runnerIDForHost(host)
	leaseTTL := runnerAgentLeaseTTLSeconds(0)
	leaseExpiresAt := now.Add(time.Duration(leaseTTL) * time.Second)
	streams := make([]*DatabaseRunnerAgentStream, 0, len(candidates))
	for _, stream := range candidates {
		acquired, err := uc.logArchiveStreamRepo.TryAcquireLease(ctx, stream.ID, host.ID, resolvedRunnerID, now, leaseExpiresAt)
		if err != nil {
			return nil, err
		}
		if !acquired {
			continue
		}
		leased, err := uc.logArchiveStreamRepo.GetByID(ctx, stream.ID)
		if err != nil {
			continue
		}
		streams = append(streams, uc.toRunnerAgentStream(ctx, host, leased, basePath))
		uc.recordLogArchiveEvent(ctx, &DatabaseLogArchiveEvent{
			StreamID:          leased.ID,
			InstanceID:        leased.InstanceID,
			SourceInstanceID:  leased.SourceInstanceID,
			RunnerHostID:      host.ID,
			RunnerID:          resolvedRunnerID,
			EventType:         DatabaseLogArchiveEventLeaseAcquired,
			Level:             DatabaseLogArchiveEventLevelInfo,
			Message:           "Runner Agent 已获取日志归档流租约",
			CursorFile:        leased.CursorFile,
			CursorPos:         leased.CursorPos,
			ActiveFile:        leased.ActiveFile,
			ArchiveLagSeconds: leased.ArchiveLagSeconds,
			OccurredAt:        &now,
		})
	}
	return &DatabaseRunnerAgentStreamListVO{
		RunnerHostID:             host.ID,
		RunnerID:                 resolvedRunnerID,
		LeaseTTLSeconds:          leaseTTL,
		HeartbeatIntervalSeconds: defaultRunnerAgentHeartbeatIntervalSeconds,
		Streams:                  streams,
	}, nil
}

func (uc *UseCase) RunnerAgentCheckpointLogArchiveStream(ctx context.Context, runnerID, auth string, streamID uint, req *DatabaseRunnerAgentCheckpointRequest) (*DatabaseLogArchiveStreamVO, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	host, err := uc.authenticateRunnerAgent(ctx, runnerID, auth)
	if err != nil {
		return nil, err
	}
	stream, err := uc.logArchiveStreamRepo.GetByID(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("日志归档流不存在")
	}
	resolvedRunnerID := runnerIDForHost(host)
	if err := validateRunnerAgentStreamOwnership(stream, host.ID, resolvedRunnerID); err != nil {
		return nil, err
	}
	now := time.Now()
	lastEventTime, err := parseDatabaseTime(req.LastEventTime)
	if err != nil {
		return nil, fmt.Errorf("最近事件时间格式不正确")
	}
	applyRunnerAgentCheckpoint(stream, req, lastEventTime, now, resolvedRunnerID)
	if err := uc.logArchiveStreamRepo.Update(ctx, stream); err != nil {
		return nil, err
	}
	if req.ReleaseLease || stream.Status == DatabaseLogArchiveStreamStatusDegraded || stream.Status == DatabaseLogArchiveStreamStatusFailed || strings.TrimSpace(stream.LastError) != "" {
		level := DatabaseLogArchiveEventLevelInfo
		if stream.Status == DatabaseLogArchiveStreamStatusFailed {
			level = DatabaseLogArchiveEventLevelError
		} else if stream.Status == DatabaseLogArchiveStreamStatusDegraded || strings.TrimSpace(stream.LastError) != "" {
			level = DatabaseLogArchiveEventLevelWarning
		}
		message := stream.LastError
		if req.ReleaseLease {
			message = "Runner Agent 已释放日志归档流租约"
		}
		uc.recordLogArchiveEvent(ctx, &DatabaseLogArchiveEvent{
			StreamID:          stream.ID,
			InstanceID:        stream.InstanceID,
			SourceInstanceID:  stream.SourceInstanceID,
			RunnerHostID:      host.ID,
			RunnerID:          resolvedRunnerID,
			EventType:         DatabaseLogArchiveEventCheckpoint,
			Level:             level,
			Message:           message,
			FileName:          stream.LastArchiveName,
			CursorFile:        stream.CursorFile,
			CursorPos:         stream.CursorPos,
			ActiveFile:        stream.ActiveFile,
			ArchiveLagSeconds: stream.ArchiveLagSeconds,
			OccurredAt:        &now,
		})
	}
	return uc.toLogArchiveStreamVO(ctx, stream), nil
}

func (uc *UseCase) RunnerAgentRegisterLogArchive(ctx context.Context, runnerID, auth string, req *DatabaseExternalLogArchiveRequest) (*DatabaseLogArchiveVO, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	if req == nil || req.StreamID == 0 {
		return nil, fmt.Errorf("请选择日志归档流")
	}
	host, err := uc.authenticateRunnerAgent(ctx, runnerID, auth)
	if err != nil {
		return nil, err
	}
	stream, err := uc.logArchiveStreamRepo.GetByID(ctx, req.StreamID)
	if err != nil {
		return nil, fmt.Errorf("日志归档流不存在")
	}
	resolvedRunnerID := runnerIDForHost(host)
	if err := validateRunnerAgentStreamOwnership(stream, host.ID, resolvedRunnerID); err != nil {
		return nil, err
	}
	item, err := uc.RegisterExternalLogArchive(ctx, req)
	if err != nil {
		return nil, err
	}
	stream, err = uc.logArchiveStreamRepo.GetByID(ctx, req.StreamID)
	if err == nil && stream != nil {
		now := time.Now()
		stream.LastHeartbeatAt = &now
		stream.LeaseOwner = resolvedRunnerID
		stream.LeaseExpiresAt = ptrTime(now.Add(time.Duration(runnerAgentLeaseTTLSeconds(0)) * time.Second))
		_ = uc.logArchiveStreamRepo.Update(ctx, stream)
		uc.recordLogArchiveEvent(ctx, &DatabaseLogArchiveEvent{
			StreamID:          stream.ID,
			InstanceID:        stream.InstanceID,
			SourceInstanceID:  stream.SourceInstanceID,
			RunnerHostID:      host.ID,
			RunnerID:          resolvedRunnerID,
			EventType:         DatabaseLogArchiveEventArchiveSuccess,
			Level:             DatabaseLogArchiveEventLevelInfo,
			Message:           "Runner Agent 已登记日志归档文件",
			FileName:          req.FileName,
			CursorFile:        stream.CursorFile,
			CursorPos:         stream.CursorPos,
			ActiveFile:        stream.ActiveFile,
			ArchiveLagSeconds: stream.ArchiveLagSeconds,
			PayloadJSON:       archiveEventPayload(map[string]any{"storageUri": req.StorageURI, "fileSize": req.FileSize, "checksumSha256": req.ChecksumSHA256}),
			OccurredAt:        &now,
		})
	}
	return item, nil
}

func (uc *UseCase) RunnerAgentCreateLogArchiveEvent(ctx context.Context, runnerID, auth string, req *DatabaseRunnerAgentEventRequest) (*DatabaseLogArchiveEventVO, error) {
	if uc.logArchiveEventRepo == nil {
		return nil, fmt.Errorf("日志归档事件仓库未配置")
	}
	if req == nil {
		return nil, fmt.Errorf("事件内容不能为空")
	}
	host, err := uc.authenticateRunnerAgent(ctx, runnerID, auth)
	if err != nil {
		return nil, err
	}
	resolvedRunnerID := runnerIDForHost(host)
	var stream *DatabaseLogArchiveStream
	if req.StreamID > 0 {
		if uc.logArchiveStreamRepo == nil {
			return nil, fmt.Errorf("日志归档流仓库未配置")
		}
		stream, err = uc.logArchiveStreamRepo.GetByID(ctx, req.StreamID)
		if err != nil {
			return nil, fmt.Errorf("日志归档流不存在")
		}
		if stream.RunnerHostID != host.ID {
			return nil, fmt.Errorf("日志归档流未绑定当前 Runner")
		}
	}
	occurredAt, err := parseDatabaseTime(req.OccurredAt)
	if err != nil {
		return nil, fmt.Errorf("事件时间格式不正确")
	}
	if occurredAt == nil {
		now := time.Now()
		occurredAt = &now
	}
	item := &DatabaseLogArchiveEvent{
		RunnerHostID:      host.ID,
		RunnerID:          resolvedRunnerID,
		EventType:         normalizeLogArchiveEventType(req.EventType),
		Level:             normalizeLogArchiveEventLevel(req.Level),
		Message:           req.Message,
		FileName:          req.FileName,
		CursorFile:        req.CursorFile,
		CursorPos:         req.CursorPos,
		ActiveFile:        req.ActiveFile,
		ArchiveLagSeconds: req.ArchiveLagSeconds,
		PayloadJSON:       req.PayloadJSON,
		OccurredAt:        occurredAt,
	}
	if stream != nil {
		item.StreamID = stream.ID
		item.InstanceID = stream.InstanceID
		item.SourceInstanceID = stream.SourceInstanceID
	}
	if err := uc.recordLogArchiveEvent(ctx, item); err != nil {
		return nil, err
	}
	return uc.toLogArchiveEventVO(ctx, item), nil
}

func (uc *UseCase) authenticateRunnerAgent(ctx context.Context, runnerID, auth string) (*DatabaseRunnerHost, error) {
	if uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("Runner 主机仓库未配置")
	}
	runnerID = strings.TrimSpace(runnerID)
	if runnerID == "" {
		return nil, fmt.Errorf("Runner ID 不能为空")
	}
	host, err := uc.findRunnerHostByRunnerID(ctx, runnerID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	if !runnerAgentAuthMatches(host.ConfigJSON, auth) {
		return nil, fmt.Errorf("Runner Agent 鉴权失败，请在 Runner Host configJson 中配置 runnerAuthSha256")
	}
	return host, nil
}

func (uc *UseCase) findRunnerHostByRunnerID(ctx context.Context, runnerID string) (*DatabaseRunnerHost, error) {
	if id, ok := parseRunnerHostIDAlias(runnerID); ok {
		host, err := uc.runnerHostRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("Runner 主机不存在")
		}
		return host, nil
	}
	items, _, err := uc.runnerHostRepo.List(ctx, &DatabaseRunnerHostListRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	for _, host := range items {
		if runnerIDForHost(host) == runnerID {
			return host, nil
		}
	}
	return nil, fmt.Errorf("Runner 主机不存在")
}

func (uc *UseCase) toRunnerAgentStream(ctx context.Context, host *DatabaseRunnerHost, stream *DatabaseLogArchiveStream, basePath string) *DatabaseRunnerAgentStream {
	if host == nil || stream == nil {
		return nil
	}
	sourceID := stream.SourceInstanceID
	if sourceID == 0 {
		sourceID = stream.InstanceID
	}
	var source *DatabaseRunnerAgentSource
	if uc.instanceRepo != nil && sourceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, sourceID); err == nil && instance != nil {
			source = &DatabaseRunnerAgentSource{
				ID:              instance.ID,
				Name:            instance.Name,
				DBType:          normalizeDBType(instance.DBType),
				DBTypeText:      DBTypeText(instance.DBType),
				Host:            instance.Host,
				Port:            instance.Port,
				DefaultDatabase: instance.DefaultDatabase,
			}
		}
	}
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	checkpointURL := fmt.Sprintf("%s/log-archive-streams/%d/checkpoint", basePath, stream.ID)
	logArchivePostURL := fmt.Sprintf("%s/log-archives", basePath)
	return &DatabaseRunnerAgentStream{
		Stream:         uc.toLogArchiveStreamVO(ctx, stream),
		SourceInstance: source,
		Runner: &DatabaseRunnerAgentRunnerConfig{
			ID:               host.ID,
			Name:             host.Name,
			RunnerType:       host.RunnerType,
			WorkDir:          host.WorkDir,
			StorageMountPath: host.StorageMountPath,
		},
		LeaseOwner:        stream.LeaseOwner,
		LeaseExpiresAt:    formatTime(stream.LeaseExpiresAt),
		CheckpointURL:     checkpointURL,
		LogArchivePostURL: logArchivePostURL,
	}
}

func applyRunnerAgentHeartbeat(host *DatabaseRunnerHost, req *DatabaseRunnerAgentHeartbeatRequest, now time.Time) {
	if host == nil {
		return
	}
	status := DatabaseRunnerHostStatusOnline
	message := ""
	if req != nil {
		message = trimText(strings.TrimSpace(req.Message), 500)
		switch strings.ToLower(strings.TrimSpace(req.Status)) {
		case DatabaseRunnerHostStatusFailed:
			status = DatabaseRunnerHostStatusFailed
		case DatabaseRunnerHostStatusDisabled:
			status = DatabaseRunnerHostStatusDisabled
		}
	}
	host.LastHeartbeatAt = &now
	host.Status = status
	if status == DatabaseRunnerHostStatusFailed {
		if message == "" {
			message = "Runner Agent 心跳上报失败状态"
		}
		host.LastError = message
	} else {
		host.LastError = ""
	}
}

func applyRunnerAgentCheckpoint(stream *DatabaseLogArchiveStream, req *DatabaseRunnerAgentCheckpointRequest, lastEventTime *time.Time, now time.Time, runnerID string) {
	if stream == nil || req == nil {
		return
	}
	if req.ReleaseLease {
		stream.LeaseOwner = ""
		stream.LeaseExpiresAt = nil
		stream.LastHeartbeatAt = &now
		if stream.DesiredState == DatabaseLogArchiveDesiredStatePaused {
			stream.DaemonStatus = DatabaseLogArchiveDaemonStatusPaused
			stream.Status = DatabaseLogArchiveStreamStatusPaused
			return
		}
		stream.DaemonStatus = DatabaseLogArchiveDaemonStatusStopped
		stream.Status = DatabaseLogArchiveStreamStatusPending
		return
	}
	stream.LeaseOwner = runnerID
	stream.LeaseExpiresAt = ptrTime(now.Add(time.Duration(runnerAgentLeaseTTLSeconds(req.LeaseTTLSeconds)) * time.Second))
	stream.LastHeartbeatAt = &now
	stream.DaemonStatus = normalizeLogArchiveDaemonStatus(req.DaemonStatus)
	if strings.TrimSpace(req.CursorFile) != "" {
		stream.CursorFile = trimText(strings.TrimSpace(req.CursorFile), 255)
	}
	if req.CursorPos > 0 {
		stream.CursorPos = req.CursorPos
	}
	if strings.TrimSpace(req.CursorGTIDSet) != "" {
		stream.CursorGTIDSet = strings.TrimSpace(req.CursorGTIDSet)
	}
	stream.ActiveFile = trimText(strings.TrimSpace(req.ActiveFile), 255)
	if strings.TrimSpace(req.LastSourceFile) != "" {
		stream.LastSourceFile = trimText(strings.TrimSpace(req.LastSourceFile), 255)
	}
	if req.LastSourcePos > 0 {
		stream.LastSourcePos = req.LastSourcePos
	}
	if lastEventTime != nil {
		stream.LastEventTime = lastEventTime
	}
	stream.ArchiveLagSeconds = req.ArchiveLagSeconds
	stream.LastError = trimText(strings.TrimSpace(req.LastError), 1000)
	switch stream.DaemonStatus {
	case DatabaseLogArchiveDaemonStatusRunning:
		stream.Status = DatabaseLogArchiveStreamStatusRunning
		stream.ConsecutiveFailures = req.ConsecutiveFailures
		if stream.LastError == "" && stream.RPOTargetSeconds > 0 && stream.ArchiveLagSeconds > stream.RPOTargetSeconds*2 {
			stream.Status = DatabaseLogArchiveStreamStatusDegraded
			stream.DaemonStatus = DatabaseLogArchiveDaemonStatusDegraded
			stream.LastError = "归档延迟超过 RPO 目标"
		}
	case DatabaseLogArchiveDaemonStatusPaused:
		stream.Status = DatabaseLogArchiveStreamStatusPaused
	case DatabaseLogArchiveDaemonStatusDegraded:
		stream.Status = DatabaseLogArchiveStreamStatusDegraded
		stream.ConsecutiveFailures = maxInt(req.ConsecutiveFailures, stream.ConsecutiveFailures+1)
		if stream.LastError == "" {
			stream.LastError = "Runner Agent 上报归档流降级"
		}
	case DatabaseLogArchiveDaemonStatusFailed:
		stream.Status = DatabaseLogArchiveStreamStatusFailed
		stream.ConsecutiveFailures = maxInt(req.ConsecutiveFailures, stream.ConsecutiveFailures+1)
		if stream.LastError == "" {
			stream.LastError = "Runner Agent 上报归档流失败"
		}
	default:
		stream.Status = DatabaseLogArchiveStreamStatusPending
	}
}

func validateRunnerAgentStreamOwnership(stream *DatabaseLogArchiveStream, runnerHostID uint, runnerID string) error {
	if stream == nil {
		return fmt.Errorf("日志归档流不存在")
	}
	if stream.RunnerHostID != runnerHostID {
		return fmt.Errorf("日志归档流未绑定当前 Runner")
	}
	if stream.DesiredState != DatabaseLogArchiveDesiredStateRunning {
		return fmt.Errorf("日志归档流当前不期望运行")
	}
	if strings.TrimSpace(stream.LeaseOwner) != runnerID {
		return fmt.Errorf("日志归档流租约不属于当前 Runner，请重新拉取任务")
	}
	return nil
}

func runnerAgentHeartbeatVO(host *DatabaseRunnerHost, runnerID string) *DatabaseRunnerAgentHeartbeatVO {
	return &DatabaseRunnerAgentHeartbeatVO{
		RunnerHostID:             host.ID,
		RunnerID:                 runnerID,
		Status:                   host.Status,
		StatusText:               RunnerHostStatusText(host.Status),
		LastHeartbeatAt:          formatTime(host.LastHeartbeatAt),
		LeaseTTLSeconds:          defaultRunnerAgentLeaseTTLSeconds,
		HeartbeatIntervalSeconds: defaultRunnerAgentHeartbeatIntervalSeconds,
	}
}

func runnerAgentLeaseTTLSeconds(value int) int {
	if value <= 0 {
		return defaultRunnerAgentLeaseTTLSeconds
	}
	if value > maxRunnerAgentLeaseTTLSeconds {
		return maxRunnerAgentLeaseTTLSeconds
	}
	return value
}

func runnerAgentAuthMatches(configJSON, auth string) bool {
	auth = strings.TrimSpace(auth)
	if auth == "" {
		return false
	}
	expected := runnerAgentAuthHash(configJSON)
	if expected == "" {
		return false
	}
	sum := sha256.Sum256([]byte(auth))
	got := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(expected)), []byte(got)) == 1
}

func archiveEventPayload(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return trimText(string(data), 4000)
}

func runnerAgentAuthHash(configJSON string) string {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(configJSON), &data); err != nil {
		return ""
	}
	for _, key := range []string{"runnerAuthSha256", "authSha256"} {
		if raw, ok := data[key]; ok {
			value := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
			if len(value) == 64 {
				return value
			}
		}
	}
	return ""
}

func parseRunnerHostIDAlias(value string) (uint, bool) {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"runner-host-", "runner:", "host:"} {
		value = strings.TrimPrefix(value, prefix)
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
