package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

const (
	defaultDatabaseArchiverIntervalSeconds = 30
	defaultDatabaseArchiverLeaseTTLSeconds = 90
	defaultDatabaseArchiverMaxFilesPerLoop = 5
	defaultDatabaseArchiverWorkDir         = "/var/lib/opshub-agent"
	defaultDatabaseArchiverObjectPrefix    = "opshub/database-archives"
)

type databaseArchiverConfig struct {
	Enabled         bool                         `json:"enabled"`
	BaseURL         string                       `json:"baseUrl"`
	RunnerID        string                       `json:"runnerId"`
	RunnerAuth      string                       `json:"runnerAuth"`
	IntervalSeconds int                          `json:"intervalSeconds"`
	LeaseTTLSeconds int                          `json:"leaseTtlSeconds"`
	MaxFilesPerLoop int                          `json:"maxFilesPerLoop"`
	IncludeCurrent  bool                         `json:"includeCurrent"`
	WorkDir         string                       `json:"workDir"`
	StorageRoot     string                       `json:"storageRoot"`
	Storage         databaseArchiverStorage      `json:"storage"`
	MySQLBinlogPath string                       `json:"mysqlBinlogPath"`
	Credentials     []databaseArchiverCredential `json:"credentials"`
}

type databaseArchiverStorage struct {
	Type               string `json:"type"`
	Endpoint           string `json:"endpoint"`
	Bucket             string `json:"bucket"`
	Region             string `json:"region"`
	PathPrefix         string `json:"pathPrefix"`
	StagingPrefix      string `json:"stagingPrefix"`
	AccessKey          string `json:"accessKey"`
	SecretKey          string `json:"secretKey"`
	UseSSL             bool   `json:"useSsl"`
	UsePathStyle       bool   `json:"usePathStyle"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
}

type databaseArchiverCredential struct {
	StreamID   uint   `json:"streamId"`
	InstanceID uint   `json:"instanceId"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type resolvedDatabaseArchiverConfig struct {
	Enabled         bool
	OpsHubBaseURL   string
	EndpointBaseURL string
	RunnerID        string
	RunnerAuth      string
	Interval        time.Duration
	LeaseTTLSeconds int
	MaxFilesPerLoop int
	IncludeCurrent  bool
	WorkDir         string
	StorageRoot     string
	Storage         databaseArchiverStorage
	MySQLBinlogPath string
	Credentials     []databaseArchiverCredential
}

type databaseArchiverAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type databaseArchiverStreamList struct {
	RunnerHostID             uint                             `json:"runnerHostId"`
	RunnerID                 string                           `json:"runnerId"`
	LeaseTTLSeconds          int                              `json:"leaseTtlSeconds"`
	HeartbeatIntervalSeconds int                              `json:"heartbeatIntervalSeconds"`
	Streams                  []databaseArchiverAssignedStream `json:"streams"`
}

type databaseArchiverAssignedStream struct {
	Stream            databaseArchiverStream       `json:"stream"`
	SourceInstance    databaseArchiverSource       `json:"sourceInstance"`
	Runner            databaseArchiverRunnerConfig `json:"runner"`
	LeaseOwner        string                       `json:"leaseOwner"`
	LeaseExpiresAt    string                       `json:"leaseExpiresAt"`
	CheckpointURL     string                       `json:"checkpointUrl"`
	LogArchivePostURL string                       `json:"logArchivePostUrl"`
}

type databaseArchiverStream struct {
	ID                uint   `json:"id"`
	InstanceID        uint   `json:"instanceId"`
	SourceInstanceID  uint   `json:"sourceInstanceId"`
	Engine            string `json:"engine"`
	ArchiveType       string `json:"archiveType"`
	ArchiveMode       string `json:"archiveMode"`
	RPOTargetSeconds  int    `json:"rpoTargetSeconds"`
	CursorFile        string `json:"cursorFile"`
	CursorPos         int64  `json:"cursorPos"`
	CursorGTIDSet     string `json:"cursorGtidSet"`
	ActiveFile        string `json:"activeFile"`
	LastSourceFile    string `json:"lastSourceFile"`
	LastSourcePos     int64  `json:"lastSourcePos"`
	ArchiveLagSeconds int    `json:"archiveLagSeconds"`
	LastArchiveName   string `json:"lastArchiveName"`
}

type databaseArchiverSource struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	DBType          string `json:"dbType"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	DefaultDatabase string `json:"defaultDatabase"`
}

type databaseArchiverRunnerConfig struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	RunnerType       string `json:"runnerType"`
	WorkDir          string `json:"workDir"`
	StorageMountPath string `json:"storageMountPath"`
}

type databaseArchiverHeartbeatRequest struct {
	Version        string `json:"version"`
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	RunningStreams int    `json:"runningStreams"`
}

type databaseArchiverCheckpointRequest struct {
	DaemonStatus        string `json:"daemonStatus"`
	CursorFile          string `json:"cursorFile,omitempty"`
	CursorPos           int64  `json:"cursorPos,omitempty"`
	CursorGTIDSet       string `json:"cursorGtidSet,omitempty"`
	ActiveFile          string `json:"activeFile,omitempty"`
	LastSourceFile      string `json:"lastSourceFile,omitempty"`
	LastSourcePos       int64  `json:"lastSourcePos,omitempty"`
	LastEventTime       string `json:"lastEventTime,omitempty"`
	ArchiveLagSeconds   int    `json:"archiveLagSeconds,omitempty"`
	ConsecutiveFailures int    `json:"consecutiveFailures,omitempty"`
	LastError           string `json:"lastError,omitempty"`
	LeaseTTLSeconds     int    `json:"leaseTtlSeconds,omitempty"`
	ReleaseLease        bool   `json:"releaseLease,omitempty"`
}

type databaseArchiverLogArchiveRequest struct {
	StreamID         uint   `json:"streamId"`
	FileName         string `json:"fileName"`
	StorageURI       string `json:"storageUri"`
	FileSize         int64  `json:"fileSize"`
	ChecksumSHA256   string `json:"checksumSha256"`
	FirstEventTime   string `json:"firstEventTime"`
	LastEventTime    string `json:"lastEventTime"`
	Status           string `json:"status"`
	StartPos         int64  `json:"startPos"`
	EndPos           int64  `json:"endPos"`
	PreviousFileName string `json:"previousFileName,omitempty"`
	NextFileName     string `json:"nextFileName,omitempty"`
}

type databaseArchiverEventRequest struct {
	StreamID          uint   `json:"streamId"`
	EventType         string `json:"eventType"`
	Level             string `json:"level"`
	Message           string `json:"message,omitempty"`
	FileName          string `json:"fileName,omitempty"`
	CursorFile        string `json:"cursorFile,omitempty"`
	CursorPos         int64  `json:"cursorPos,omitempty"`
	ActiveFile        string `json:"activeFile,omitempty"`
	ArchiveLagSeconds int    `json:"archiveLagSeconds,omitempty"`
	PayloadJSON       string `json:"payloadJson,omitempty"`
	OccurredAt        string `json:"occurredAt,omitempty"`
}

type agentMySQLBinaryLog struct {
	Name string
	Size int64
}

type agentMySQLBinaryLogStatus struct {
	File            string
	Position        int64
	ExecutedGTIDSet string
}

type agentBinlogArchiveSelection struct {
	FileName string
	FileSize int64
	Previous string
	Next     string
}

type agentBinlogArtifact struct {
	FileName       string
	Path           string
	StorageURI     string
	FileSize       int64
	ChecksumSHA256 string
	FirstEventTime time.Time
	LastEventTime  time.Time
	Previous       string
	Next           string
}

type agentBinlogPurgeGapError struct {
	LastArchived   string
	FirstAvailable string
	LastAvailable  string
}

func (e *agentBinlogPurgeGapError) Error() string {
	return fmt.Sprintf("归档流最近文件 %s 已不在源库 binlog 列表中，可用范围为 %s..%s，可能已经 purge，需人工确认日志链", e.LastArchived, e.FirstAvailable, e.LastAvailable)
}

func resolveDatabaseArchiverConfig(cfg *agentConfig) (*resolvedDatabaseArchiverConfig, error) {
	if cfg == nil || !cfg.DatabaseArchiver.Enabled {
		return &resolvedDatabaseArchiverConfig{}, nil
	}
	raw := cfg.DatabaseArchiver
	runnerID := strings.TrimSpace(raw.RunnerID)
	if runnerID == "" {
		return nil, errors.New("runnerId is required when databaseArchiver.enabled is true")
	}
	runnerAuth := strings.TrimSpace(raw.RunnerAuth)
	if runnerAuth == "" {
		return nil, errors.New("runnerAuth is required when databaseArchiver.enabled is true")
	}
	opsHubBaseURL := resolveDatabaseArchiverBaseURL(raw.BaseURL, cfg.ReportURL)
	if opsHubBaseURL == "" {
		return nil, errors.New("baseUrl is required when reportUrl cannot be used to derive OpsHub base URL")
	}
	intervalSeconds := raw.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = defaultDatabaseArchiverIntervalSeconds
	}
	if intervalSeconds < 5 {
		intervalSeconds = 5
	}
	leaseTTLSeconds := raw.LeaseTTLSeconds
	if leaseTTLSeconds <= 0 {
		leaseTTLSeconds = defaultDatabaseArchiverLeaseTTLSeconds
	}
	if leaseTTLSeconds < 30 {
		leaseTTLSeconds = 30
	}
	if leaseTTLSeconds > 600 {
		leaseTTLSeconds = 600
	}
	maxFiles := raw.MaxFilesPerLoop
	if maxFiles <= 0 {
		maxFiles = defaultDatabaseArchiverMaxFilesPerLoop
	}
	if maxFiles > 20 {
		maxFiles = 20
	}
	workDir := strings.TrimSpace(raw.WorkDir)
	if workDir == "" {
		workDir = defaultDatabaseArchiverWorkDir
	}
	storageRoot := strings.TrimSpace(raw.StorageRoot)
	if storageRoot == "" {
		storageRoot = filepath.Join(workDir, "database-archives")
	}
	storage, err := resolveDatabaseArchiverStorage(raw.Storage)
	if err != nil {
		return nil, err
	}
	endpointBase := strings.TrimRight(opsHubBaseURL, "/") + "/api/v1/public/databases/runner-agents/" + url.PathEscape(runnerID)
	return &resolvedDatabaseArchiverConfig{
		Enabled:         true,
		OpsHubBaseURL:   strings.TrimRight(opsHubBaseURL, "/"),
		EndpointBaseURL: endpointBase,
		RunnerID:        runnerID,
		RunnerAuth:      runnerAuth,
		Interval:        time.Duration(intervalSeconds) * time.Second,
		LeaseTTLSeconds: leaseTTLSeconds,
		MaxFilesPerLoop: maxFiles,
		IncludeCurrent:  raw.IncludeCurrent,
		WorkDir:         workDir,
		StorageRoot:     storageRoot,
		Storage:         storage,
		MySQLBinlogPath: strings.TrimSpace(raw.MySQLBinlogPath),
		Credentials:     append([]databaseArchiverCredential{}, raw.Credentials...),
	}, nil
}

func resolveDatabaseArchiverStorage(raw databaseArchiverStorage) (databaseArchiverStorage, error) {
	storageType := strings.ToLower(strings.TrimSpace(raw.Type))
	if storageType == "" {
		storageType = "local"
	}
	if storageType != "local" && storageType != "s3" && storageType != "minio" {
		return databaseArchiverStorage{}, fmt.Errorf("databaseArchiver.storage.type 仅支持 local/s3/minio，当前为 %s", raw.Type)
	}
	result := databaseArchiverStorage{Type: storageType}
	if storageType == "local" {
		return result, nil
	}
	result.Endpoint = normalizeAgentObjectStorageEndpoint(raw.Endpoint, raw.UseSSL)
	result.Bucket = strings.TrimSpace(raw.Bucket)
	result.Region = strings.TrimSpace(raw.Region)
	if result.Region == "" {
		result.Region = "us-east-1"
	}
	result.PathPrefix = cleanAgentObjectKey(raw.PathPrefix)
	if result.PathPrefix == "" {
		result.PathPrefix = defaultDatabaseArchiverObjectPrefix
	}
	result.StagingPrefix = cleanAgentObjectKey(raw.StagingPrefix)
	if result.StagingPrefix == "" {
		result.StagingPrefix = joinAgentObjectKey(result.PathPrefix, ".staging")
	}
	result.AccessKey = strings.TrimSpace(raw.AccessKey)
	result.SecretKey = strings.TrimSpace(raw.SecretKey)
	result.UseSSL = raw.UseSSL
	result.UsePathStyle = raw.UsePathStyle || storageType == "minio" || result.Endpoint != ""
	result.InsecureSkipVerify = raw.InsecureSkipVerify
	if result.Bucket == "" {
		return databaseArchiverStorage{}, errors.New("databaseArchiver.storage.bucket 不能为空")
	}
	if result.AccessKey == "" || result.SecretKey == "" {
		return databaseArchiverStorage{}, errors.New("databaseArchiver.storage.accessKey/secretKey 不能为空")
	}
	if storageType == "minio" && result.Endpoint == "" {
		return databaseArchiverStorage{}, errors.New("databaseArchiver.storage.endpoint 不能为空")
	}
	return result, nil
}

func resolveDatabaseArchiverBaseURL(configured, reportURL string) string {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return trimKnownAgentPath(configured)
	}
	return trimKnownAgentPath(reportURL)
}

func trimKnownAgentPath(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	for _, marker := range []string{"/api/v1/public/databases/runner-agents/", "/api/v1/public/agents/report", "/api/v1/public/agents/register"} {
		if idx := strings.Index(parsed.Path, marker); idx >= 0 {
			parsed.Path = strings.TrimRight(parsed.Path[:idx], "/")
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return strings.TrimRight(parsed.String(), "/")
		}
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func (a *agentApp) runDatabaseArchiver(ctx context.Context, cfg *resolvedDatabaseArchiverConfig) {
	log.Printf("database binlog archiver started: runner=%s interval=%s endpoint=%s", cfg.RunnerID, cfg.Interval, cfg.EndpointBaseURL)
	a.runDatabaseArchiverOnce(ctx, cfg)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = a.databaseArchiverHeartbeat(context.Background(), cfg, "failed", "agent stopped", 0)
			return
		case <-ticker.C:
			a.runDatabaseArchiverOnce(ctx, cfg)
		}
	}
}

func (a *agentApp) runDatabaseArchiverOnce(ctx context.Context, cfg *resolvedDatabaseArchiverConfig) {
	if err := a.databaseArchiverHeartbeat(ctx, cfg, "online", "", 0); err != nil {
		log.Printf("database archiver heartbeat failed: %v", err)
		return
	}
	streams, err := a.databaseArchiverListStreams(ctx, cfg)
	if err != nil {
		log.Printf("database archiver list streams failed: %v", err)
		return
	}
	running := 0
	for _, stream := range streams.Streams {
		if stream.Stream.ID == 0 {
			continue
		}
		running++
		if err := a.processDatabaseArchiverStream(ctx, cfg, stream); err != nil {
			log.Printf("database archiver stream %d failed: %v", stream.Stream.ID, err)
		}
	}
	if err := a.databaseArchiverHeartbeat(ctx, cfg, "online", "", running); err != nil {
		log.Printf("database archiver heartbeat failed: %v", err)
	}
}

func (a *agentApp) processDatabaseArchiverStream(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream) (err error) {
	var (
		eventFile    string
		eventActive  string
		eventType    = "archive_failed"
		eventLevel   = "error"
		eventPayload = map[string]any{
			"archiveMode": item.Stream.ArchiveMode,
			"streamId":    item.Stream.ID,
		}
	)
	defer func() {
		if err == nil {
			return
		}
		_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
			EventType:   eventType,
			Level:       eventLevel,
			Message:     err.Error(),
			FileName:    eventFile,
			CursorFile:  item.Stream.CursorFile,
			CursorPos:   item.Stream.CursorPos,
			ActiveFile:  eventActive,
			PayloadJSON: databaseArchiverEventPayloadJSON(eventPayload),
			OccurredAt:  time.Now().Format("2006-01-02 15:04:05"),
		})
	}()
	if strings.ToLower(strings.TrimSpace(item.Stream.ArchiveType)) != "binlog" {
		err = errors.New("Agent 当前仅支持 binlog 日志归档流")
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	mode := strings.ToLower(strings.TrimSpace(item.Stream.ArchiveMode))
	if mode != "polling" && mode != "streaming" {
		err = errors.New("Agent 仅支持 polling/streaming binlog 归档模式")
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	credential, err := cfg.credentialForStream(item)
	if err != nil {
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	db, err := openAgentMySQLDB(item.SourceInstance, credential)
	if err != nil {
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	defer db.Close()
	logs, err := agentListMySQLBinaryLogs(ctx, db)
	if err != nil {
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	status, _ := agentReadMySQLBinaryLogStatus(ctx, db)
	current := currentAgentBinaryLog(logs)
	eventActive = activeFileForMode(mode, current.Name)
	eventPayload["sourceFile"] = firstNonEmptyString(status.File, current.Name)
	eventPayload["sourcePos"] = firstNonZeroInt64Agent(status.Position, current.Size)
	lastArchived := strings.TrimSpace(item.Stream.LastArchiveName)
	if lastArchived == "" {
		lastArchived = strings.TrimSpace(item.Stream.CursorFile)
	}
	includeCurrent := cfg.IncludeCurrent
	if mode == "streaming" {
		includeCurrent = false
	}
	selections, err := selectAgentBinlogsForArchive(logs, lastArchived, cfg.MaxFilesPerLoop, includeCurrent)
	if err != nil {
		var purgeGap *agentBinlogPurgeGapError
		if errors.As(err, &purgeGap) {
			eventType = "purge_gap"
			eventLevel = "warning"
			eventPayload["lastArchived"] = purgeGap.LastArchived
			eventPayload["firstAvailable"] = purgeGap.FirstAvailable
			eventPayload["lastAvailable"] = purgeGap.LastAvailable
		}
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastSourceFile:      firstNonEmptyString(status.File, current.Name),
			LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	tool, err := resolveMySQLBinlogTool(cfg.MySQLBinlogPath)
	if err != nil {
		_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
			DaemonStatus:        "degraded",
			LastSourceFile:      firstNonEmptyString(status.File, current.Name),
			LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
			LastError:           err.Error(),
			ConsecutiveFailures: 1,
			LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
		})
		return err
	}
	if mode == "streaming" && current.Name != "" {
		if err := spoolAgentActiveBinlog(ctx, cfg, item, credential, tool, current.Name); err != nil {
			_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
				DaemonStatus:        "degraded",
				ActiveFile:          current.Name,
				LastSourceFile:      firstNonEmptyString(status.File, current.Name),
				LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
				LastError:           "active binlog spool 失败: " + err.Error(),
				ConsecutiveFailures: 1,
				LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
			})
			return err
		}
		_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
			EventType:   "spool_updated",
			Level:       "info",
			Message:     "active binlog spool 已更新",
			ActiveFile:  current.Name,
			PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{"activeFile": current.Name, "sourcePos": firstNonZeroInt64Agent(status.Position, current.Size)}),
			OccurredAt:  time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	var lastArtifact *agentBinlogArtifact
	for _, selection := range selections {
		eventFile = selection.FileName
		artifact, err := archiveAgentBinlogFile(ctx, cfg, item, credential, tool, selection)
		if err != nil {
			_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
				DaemonStatus:        "degraded",
				ActiveFile:          activeFileForMode(mode, current.Name),
				LastSourceFile:      firstNonEmptyString(status.File, current.Name),
				LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
				LastError:           err.Error(),
				ConsecutiveFailures: 1,
				LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
			})
			return err
		}
		if err := a.databaseArchiverRegisterLogArchive(ctx, cfg, item, artifact); err != nil {
			_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
				DaemonStatus:        "degraded",
				ActiveFile:          activeFileForMode(mode, current.Name),
				LastSourceFile:      firstNonEmptyString(status.File, current.Name),
				LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
				LastError:           err.Error(),
				ConsecutiveFailures: 1,
				LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
			})
			return err
		}
		lastArtifact = &artifact
		eventFile = ""
	}
	checkpoint := databaseArchiverCheckpointRequest{
		DaemonStatus:      "running",
		ActiveFile:        activeFileForMode(mode, current.Name),
		LastSourceFile:    firstNonEmptyString(status.File, current.Name),
		LastSourcePos:     firstNonZeroInt64Agent(status.Position, current.Size),
		CursorGTIDSet:     status.ExecutedGTIDSet,
		LeaseTTLSeconds:   cfg.LeaseTTLSeconds,
		ArchiveLagSeconds: 0,
	}
	if lastArtifact != nil {
		checkpoint.CursorFile = lastArtifact.FileName
		checkpoint.CursorPos = lastArtifact.FileSize
		checkpoint.LastEventTime = lastArtifact.LastEventTime.Format("2006-01-02 15:04:05")
	} else if current.Name != "" {
		checkpoint.LastEventTime = time.Now().Format("2006-01-02 15:04:05")
	}
	return a.databaseArchiverCheckpoint(ctx, cfg, item, checkpoint)
}

func (cfg *resolvedDatabaseArchiverConfig) credentialForStream(item databaseArchiverAssignedStream) (databaseArchiverCredential, error) {
	for _, credential := range cfg.Credentials {
		if credential.StreamID > 0 && credential.StreamID == item.Stream.ID {
			return validateAgentCredential(credential, item)
		}
	}
	sourceID := item.SourceInstance.ID
	if sourceID == 0 {
		sourceID = item.Stream.SourceInstanceID
	}
	if sourceID == 0 {
		sourceID = item.Stream.InstanceID
	}
	for _, credential := range cfg.Credentials {
		if credential.InstanceID > 0 && credential.InstanceID == sourceID {
			return validateAgentCredential(credential, item)
		}
	}
	for _, credential := range cfg.Credentials {
		if strings.EqualFold(strings.TrimSpace(credential.Host), strings.TrimSpace(item.SourceInstance.Host)) && credential.Port == item.SourceInstance.Port {
			return validateAgentCredential(credential, item)
		}
	}
	return databaseArchiverCredential{}, fmt.Errorf("Agent 本地配置缺少 stream %d / instance %d 的数据库凭据", item.Stream.ID, sourceID)
}

func validateAgentCredential(credential databaseArchiverCredential, item databaseArchiverAssignedStream) (databaseArchiverCredential, error) {
	if strings.TrimSpace(credential.Username) == "" {
		return credential, fmt.Errorf("stream %d 的本地数据库用户名为空", item.Stream.ID)
	}
	if credential.Port <= 0 {
		credential.Port = item.SourceInstance.Port
	}
	if strings.TrimSpace(credential.Host) == "" {
		credential.Host = item.SourceInstance.Host
	}
	return credential, nil
}

func openAgentMySQLDB(source databaseArchiverSource, credential databaseArchiverCredential) (*sql.DB, error) {
	host := strings.TrimSpace(credential.Host)
	if host == "" {
		host = strings.TrimSpace(source.Host)
	}
	port := credential.Port
	if port <= 0 {
		port = source.Port
	}
	if host == "" || port <= 0 {
		return nil, errors.New("数据库连接地址不完整")
	}
	cfg := mysqlDriver.NewConfig()
	cfg.User = strings.TrimSpace(credential.Username)
	cfg.Passwd = credential.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	cfg.Timeout = 10 * time.Second
	cfg.ReadTimeout = 30 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.AllowNativePasswords = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接日志来源数据库失败: %w", err)
	}
	return db, nil
}

func agentListMySQLBinaryLogs(ctx context.Context, db *sql.DB) ([]agentMySQLBinaryLog, error) {
	for _, query := range []string{"SHOW BINARY LOGS", "SHOW MASTER LOGS"} {
		logs, err := agentScanMySQLBinaryLogs(ctx, db, query)
		if err == nil && len(logs) > 0 {
			return logs, nil
		}
	}
	return nil, errors.New("无法读取源库 binlog 列表，请确认 log_bin 已开启且账号具备复制/管理权限")
}

func agentScanMySQLBinaryLogs(ctx context.Context, db *sql.DB, query string) ([]agentMySQLBinaryLog, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]agentMySQLBinaryLog, 0)
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := map[string]string{}
		for i, column := range columns {
			row[strings.ToLower(strings.TrimSpace(column))] = strings.TrimSpace(values[i].String)
		}
		name := firstNonEmptyString(row["log_name"], row["file"])
		size, _ := strconv.ParseInt(firstNonEmptyString(row["file_size"], row["size"]), 10, 64)
		if name != "" {
			result = append(result, agentMySQLBinaryLog{Name: name, Size: size})
		}
	}
	return result, rows.Err()
}

func agentReadMySQLBinaryLogStatus(ctx context.Context, db *sql.DB) (agentMySQLBinaryLogStatus, error) {
	var lastErr error
	for _, query := range []string{"SHOW BINARY LOG STATUS", "SHOW MASTER STATUS"} {
		status, err := agentScanMySQLBinaryLogStatus(ctx, db, query)
		if err == nil && status.File != "" {
			return status, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("empty binary log status")
	}
	return agentMySQLBinaryLogStatus{}, lastErr
}

func agentScanMySQLBinaryLogStatus(ctx context.Context, db *sql.DB, query string) (agentMySQLBinaryLogStatus, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return agentMySQLBinaryLogStatus{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return agentMySQLBinaryLogStatus{}, err
	}
	if !rows.Next() {
		return agentMySQLBinaryLogStatus{}, rows.Err()
	}
	values := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return agentMySQLBinaryLogStatus{}, err
	}
	row := map[string]string{}
	for i, column := range columns {
		row[strings.ToLower(strings.TrimSpace(column))] = strings.TrimSpace(values[i].String)
	}
	pos, _ := strconv.ParseInt(firstNonEmptyString(row["position"], row["pos"]), 10, 64)
	return agentMySQLBinaryLogStatus{
		File:            firstNonEmptyString(row["file"], row["log_name"]),
		Position:        pos,
		ExecutedGTIDSet: firstNonEmptyString(row["executed_gtid_set"], row["gtid_executed"]),
	}, nil
}

func currentAgentBinaryLog(logs []agentMySQLBinaryLog) agentMySQLBinaryLog {
	if len(logs) == 0 {
		return agentMySQLBinaryLog{}
	}
	return logs[len(logs)-1]
}

func selectAgentBinlogsForArchive(logs []agentMySQLBinaryLog, lastArchived string, maxFiles int, includeCurrent bool) ([]agentBinlogArchiveSelection, error) {
	if len(logs) == 0 {
		return nil, errors.New("源库没有可归档的 binlog 文件")
	}
	if maxFiles <= 0 {
		maxFiles = defaultDatabaseArchiverMaxFilesPerLoop
	}
	if maxFiles > 20 {
		maxFiles = 20
	}
	start := 0
	if lastArchived = strings.TrimSpace(lastArchived); lastArchived != "" {
		start = -1
		for i, item := range logs {
			if item.Name == lastArchived {
				start = i + 1
				break
			}
		}
		if start < 0 {
			return nil, &agentBinlogPurgeGapError{
				LastArchived:   lastArchived,
				FirstAvailable: logs[0].Name,
				LastAvailable:  logs[len(logs)-1].Name,
			}
		}
	}
	end := len(logs)
	if !includeCurrent {
		end = len(logs) - 1
	}
	if start >= end {
		return nil, nil
	}
	if end-start > maxFiles {
		end = start + maxFiles
	}
	result := make([]agentBinlogArchiveSelection, 0, end-start)
	for i := start; i < end; i++ {
		selection := agentBinlogArchiveSelection{FileName: logs[i].Name, FileSize: logs[i].Size}
		if i > 0 {
			selection.Previous = logs[i-1].Name
		}
		if i+1 < len(logs) {
			selection.Next = logs[i+1].Name
		}
		result = append(result, selection)
	}
	return result, nil
}

func resolveMySQLBinlogTool(configured string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		if _, err := exec.LookPath(configured); err == nil {
			return configured, nil
		}
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("mysqlbinlog 工具不存在: %s", configured)
	}
	for _, candidate := range []string{"mysqlbinlog", "mariadb-binlog"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("mysqlbinlog/mariadb-binlog not found")
}

func archiveAgentBinlogFile(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, tool string, selection agentBinlogArchiveSelection) (agentBinlogArtifact, error) {
	finalDir := agentBinlogFinalDir(cfg, item)
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return agentBinlogArtifact{}, err
	}
	tmpDir, err := os.MkdirTemp(finalDir, ".opshub-tmp-")
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	defer os.RemoveAll(tmpDir)
	if err := runAgentMysqlbinlogRaw(ctx, cfg, item, credential, tool, tmpDir, selection.FileName); err != nil {
		return agentBinlogArtifact{}, err
	}
	tmpPath := filepath.Join(tmpDir, selection.FileName)
	finalPath := filepath.Join(finalDir, selection.FileName)
	if err := commitAgentBinlogFile(tmpPath, finalPath); err != nil {
		return agentBinlogArtifact{}, err
	}
	artifact, err := buildAgentBinlogArtifact(ctx, tool, finalPath, selection, item)
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	if cfg.objectStorageEnabled() {
		artifact.StorageURI = agentObjectStorageURI(cfg, item, artifact.FileName)
	}
	if err := writeAgentBinlogSidecars(artifact); err != nil {
		return agentBinlogArtifact{}, err
	}
	if cfg.objectStorageEnabled() {
		if err := publishAgentBinlogArtifact(ctx, cfg, item, artifact); err != nil {
			return agentBinlogArtifact{}, err
		}
	}
	return artifact, nil
}

func spoolAgentActiveBinlog(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, tool, fileName string) error {
	spoolDir := agentBinlogSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(spoolDir, ".opshub-spool-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	if err := runAgentMysqlbinlogRaw(ctx, cfg, item, credential, tool, tmpDir, fileName); err != nil {
		return err
	}
	tmpPath := filepath.Join(tmpDir, fileName)
	target := filepath.Join(spoolDir, fileName+".partial")
	return replaceAgentBinlogFile(tmpPath, target)
}

func runAgentMysqlbinlogRaw(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, tool, resultDir, fileName string) error {
	if !isSafeAgentBinlogFileName(fileName) {
		return fmt.Errorf("binlog 文件名不合法: %s", fileName)
	}
	defaultsFile, err := writeAgentMySQLDefaultsFile(cfg.WorkDir, item.SourceInstance, credential)
	if err != nil {
		return err
	}
	defer os.Remove(defaultsFile)
	commandCtx, cancel := context.WithTimeout(ctx, maxDuration(cfg.Interval*2, 60*time.Second))
	defer cancel()
	cmd := exec.CommandContext(commandCtx, tool, "--defaults-extra-file="+defaultsFile, "--read-from-remote-server", "--raw", "--result-file="+ensureTrailingPathSeparator(resultDir), fileName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqlbinlog 拉取 %s 失败: %w: %s", fileName, err, strings.TrimSpace(stderr.String()))
	}
	outPath := filepath.Join(resultDir, fileName)
	if info, err := os.Stat(outPath); err != nil || info.Size() <= 0 {
		return fmt.Errorf("mysqlbinlog 未生成有效文件: %s", fileName)
	}
	return nil
}

func writeAgentMySQLDefaultsFile(workDir string, source databaseArchiverSource, credential databaseArchiverCredential) (string, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", err
	}
	host := strings.TrimSpace(credential.Host)
	if host == "" {
		host = source.Host
	}
	port := credential.Port
	if port <= 0 {
		port = source.Port
	}
	file, err := os.CreateTemp(workDir, ".mysqlbinlog-*.cnf")
	if err != nil {
		return "", err
	}
	defer file.Close()
	content := strings.Join([]string{
		"[client]",
		"user=" + credential.Username,
		"password=" + credential.Password,
		"host=" + host,
		"port=" + strconv.Itoa(port),
		"",
	}, "\n")
	if _, err := file.WriteString(content); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func commitAgentBinlogFile(tmpPath, finalPath string) error {
	if info, err := os.Stat(finalPath); err == nil && info.Size() > 0 {
		tmpChecksum, tmpErr := sha256File(tmpPath)
		finalChecksum, finalErr := sha256File(finalPath)
		if tmpErr == nil && finalErr == nil && tmpChecksum == finalChecksum {
			return nil
		}
		return fmt.Errorf("目标归档文件已存在且 checksum 不一致: %s", finalPath)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}
	return os.Chmod(finalPath, 0o600)
}

func replaceAgentBinlogFile(tmpPath, finalPath string) error {
	stagingPath := finalPath + ".new"
	_ = os.Remove(stagingPath)
	if err := os.Rename(tmpPath, stagingPath); err != nil {
		return err
	}
	if err := os.Rename(stagingPath, finalPath); err != nil {
		_ = os.Remove(stagingPath)
		return err
	}
	return os.Chmod(finalPath, 0o600)
}

func buildAgentBinlogArtifact(ctx context.Context, tool, path string, selection agentBinlogArchiveSelection, item databaseArchiverAssignedStream) (agentBinlogArtifact, error) {
	checksum, err := sha256File(path)
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	started := time.Now()
	firstLine, lastLine := extractAgentBinlogEventLines(ctx, tool, path)
	firstEvent := parseAgentMySQLBinlogEventTime(firstLine)
	lastEvent := parseAgentMySQLBinlogEventTime(lastLine)
	if firstEvent == nil {
		firstEvent = &started
	}
	if lastEvent == nil {
		lastEvent = &started
	}
	if lastEvent.Before(*firstEvent) {
		lastEvent = firstEvent
	}
	return agentBinlogArtifact{
		FileName:       selection.FileName,
		Path:           path,
		StorageURI:     agentStorageURI(item.Runner.ID, path),
		FileSize:       info.Size(),
		ChecksumSHA256: checksum,
		FirstEventTime: *firstEvent,
		LastEventTime:  *lastEvent,
		Previous:       selection.Previous,
		Next:           selection.Next,
	}, nil
}

func writeAgentBinlogSidecars(artifact agentBinlogArtifact) error {
	if artifact.Path == "" {
		return nil
	}
	if err := os.WriteFile(artifact.Path+".sha256", []byte(artifact.ChecksumSHA256+"  "+filepath.Base(artifact.Path)+"\n"), 0o600); err != nil {
		return err
	}
	manifest := map[string]any{
		"fileName":       artifact.FileName,
		"fileSize":       artifact.FileSize,
		"checksumSha256": artifact.ChecksumSHA256,
		"firstEventTime": artifact.FirstEventTime.Format("2006-01-02 15:04:05"),
		"lastEventTime":  artifact.LastEventTime.Format("2006-01-02 15:04:05"),
		"storageUri":     artifact.StorageURI,
		"previousFile":   artifact.Previous,
		"nextFile":       artifact.Next,
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	return os.WriteFile(artifact.Path+".manifest.json", append(data, '\n'), 0o600)
}

func extractAgentBinlogEventLines(ctx context.Context, tool, path string) (string, string) {
	commandCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, tool, "--base64-output=decode-rows", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", ""
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", ""
	}
	var first, last string
	scanner := newLineScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if agentMySQLBinlogEventLinePattern.MatchString(strings.TrimSpace(line)) {
			if first == "" {
				first = line
			}
			last = line
		}
	}
	_ = cmd.Wait()
	return first, last
}

func newLineScanner(reader io.Reader) *lineScanner {
	return &lineScanner{reader: reader}
}

type lineScanner struct {
	reader io.Reader
	buf    []byte
}

func (s *lineScanner) Scan() bool {
	if s.reader == nil {
		return false
	}
	s.buf = s.buf[:0]
	tmp := make([]byte, 1)
	for {
		n, err := s.reader.Read(tmp)
		if n > 0 {
			if tmp[0] == '\n' {
				return true
			}
			s.buf = append(s.buf, tmp[0])
			if len(s.buf) > 1024*1024 {
				return true
			}
		}
		if err != nil {
			return len(s.buf) > 0
		}
	}
}

func (s *lineScanner) Text() string {
	return strings.TrimRight(string(s.buf), "\r")
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func (a *agentApp) databaseArchiverHeartbeat(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, status, message string, runningStreams int) error {
	payload := databaseArchiverHeartbeatRequest{
		Version:        a.cfg.Version,
		Status:         status,
		Message:        message,
		RunningStreams: runningStreams,
	}
	return a.databaseArchiverDoJSON(ctx, cfg, http.MethodPost, cfg.EndpointBaseURL+"/heartbeat", payload, nil)
}

func (a *agentApp) databaseArchiverListStreams(ctx context.Context, cfg *resolvedDatabaseArchiverConfig) (*databaseArchiverStreamList, error) {
	var result databaseArchiverStreamList
	if err := a.databaseArchiverDoJSON(ctx, cfg, http.MethodGet, cfg.EndpointBaseURL+"/log-archive-streams", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *agentApp) databaseArchiverCheckpoint(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, payload databaseArchiverCheckpointRequest) error {
	if payload.LeaseTTLSeconds <= 0 {
		payload.LeaseTTLSeconds = cfg.LeaseTTLSeconds
	}
	endpoint := cfg.resolveAgentURL(item.CheckpointURL)
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s/log-archive-streams/%d/checkpoint", cfg.EndpointBaseURL, item.Stream.ID)
	}
	return a.databaseArchiverDoJSON(ctx, cfg, http.MethodPost, endpoint, payload, nil)
}

func (a *agentApp) databaseArchiverRegisterLogArchive(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, artifact agentBinlogArtifact) error {
	endpoint := cfg.resolveAgentURL(item.LogArchivePostURL)
	if endpoint == "" {
		endpoint = cfg.EndpointBaseURL + "/log-archives"
	}
	payload := databaseArchiverLogArchiveRequest{
		StreamID:         item.Stream.ID,
		FileName:         artifact.FileName,
		StorageURI:       artifact.StorageURI,
		FileSize:         artifact.FileSize,
		ChecksumSHA256:   artifact.ChecksumSHA256,
		FirstEventTime:   artifact.FirstEventTime.Format("2006-01-02 15:04:05"),
		LastEventTime:    artifact.LastEventTime.Format("2006-01-02 15:04:05"),
		Status:           "archived",
		StartPos:         4,
		EndPos:           artifact.FileSize,
		PreviousFileName: artifact.Previous,
		NextFileName:     artifact.Next,
	}
	return a.databaseArchiverDoJSON(ctx, cfg, http.MethodPost, endpoint, payload, nil)
}

func (a *agentApp) databaseArchiverPostEvent(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, payload databaseArchiverEventRequest) error {
	if payload.StreamID == 0 {
		payload.StreamID = item.Stream.ID
	}
	if payload.OccurredAt == "" {
		payload.OccurredAt = time.Now().Format("2006-01-02 15:04:05")
	}
	endpoint := cfg.EndpointBaseURL + "/log-archive-events"
	return a.databaseArchiverDoJSON(ctx, cfg, http.MethodPost, endpoint, payload, nil)
}

func (a *agentApp) databaseArchiverDoJSON(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, method, endpoint string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-OpsHub-Runner-Auth", cfg.RunnerAuth)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope databaseArchiverAPIResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		if envelope.Message == "" {
			envelope.Message = "backend returned error"
		}
		return errors.New(envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

func (cfg *resolvedDatabaseArchiverConfig) resolveAgentURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return strings.TrimRight(cfg.OpsHubBaseURL, "/") + value
	}
	return strings.TrimRight(cfg.EndpointBaseURL, "/") + "/" + strings.TrimLeft(value, "/")
}

func agentBinlogFinalDir(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream) string {
	return filepath.Join(cfg.StorageRoot, "mysql-binlog", fmt.Sprintf("instance-%d", item.Stream.InstanceID), fmt.Sprintf("stream-%d", item.Stream.ID), "finalized")
}

func agentBinlogSpoolDir(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream) string {
	return filepath.Join(cfg.StorageRoot, "mysql-binlog", fmt.Sprintf("instance-%d", item.Stream.InstanceID), fmt.Sprintf("stream-%d", item.Stream.ID), "spool")
}

func agentStorageURI(runnerHostID uint, path string) string {
	if runtimePath := filepath.ToSlash(path); strings.HasPrefix(runtimePath, "/") {
		return fmt.Sprintf("runner://runner-host-%d%s", runnerHostID, runtimePath)
	}
	return fmt.Sprintf("runner://runner-host-%d/%s", runnerHostID, filepath.ToSlash(path))
}

func (cfg *resolvedDatabaseArchiverConfig) objectStorageEnabled() bool {
	if cfg == nil {
		return false
	}
	storageType := strings.ToLower(strings.TrimSpace(cfg.Storage.Type))
	return storageType == "s3" || storageType == "minio"
}

func publishAgentBinlogArtifact(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, artifact agentBinlogArtifact) error {
	if !cfg.objectStorageEnabled() {
		return nil
	}
	client := newAgentObjectStorageS3Client(cfg.Storage)
	bucket := cfg.Storage.Bucket
	finalKey := agentObjectStorageFinalKey(cfg, item, artifact.FileName)
	stagingKey := agentObjectStorageStagingKey(cfg, item, artifact.FileName)
	metadata := map[string]string{
		"opshub-artifact":       "mysql-binlog",
		"opshub-stream-id":      strconv.FormatUint(uint64(item.Stream.ID), 10),
		"opshub-instance-id":    strconv.FormatUint(uint64(item.Stream.InstanceID), 10),
		"opshub-source-id":      strconv.FormatUint(uint64(item.Stream.SourceInstanceID), 10),
		"opshub-sha256":         artifact.ChecksumSHA256,
		"opshub-file-name":      artifact.FileName,
		"opshub-first-event-at": artifact.FirstEventTime.Format(time.RFC3339),
		"opshub-last-event-at":  artifact.LastEventTime.Format(time.RFC3339),
	}
	if err := uploadAgentObjectFile(ctx, client, bucket, stagingKey, artifact.Path, metadata); err != nil {
		return fmt.Errorf("上传 binlog staging 对象失败: %w", err)
	}
	if err := verifyAgentObject(ctx, client, bucket, stagingKey, artifact.FileSize, artifact.ChecksumSHA256); err != nil {
		return fmt.Errorf("校验 binlog staging 对象失败: %w", err)
	}
	exists, err := agentObjectExistsAndMatches(ctx, client, bucket, finalKey, artifact.FileSize, artifact.ChecksumSHA256)
	if err != nil {
		return fmt.Errorf("校验已存在 binlog final 对象失败: %w", err)
	}
	if !exists {
		if err := uploadAgentObjectFile(ctx, client, bucket, finalKey, artifact.Path, metadata); err != nil {
			return fmt.Errorf("提交 binlog final 对象失败: %w", err)
		}
		if err := verifyAgentObject(ctx, client, bucket, finalKey, artifact.FileSize, artifact.ChecksumSHA256); err != nil {
			return fmt.Errorf("校验 binlog final 对象失败: %w", err)
		}
	}
	if err := publishAgentBinlogSidecar(ctx, client, bucket, finalKey+".sha256", artifact.Path+".sha256", metadata); err != nil {
		return err
	}
	if err := publishAgentBinlogSidecar(ctx, client, bucket, finalKey+".manifest.json", artifact.Path+".manifest.json", metadata); err != nil {
		return err
	}
	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(stagingKey)}); err != nil {
		log.Printf("database archiver remove staging object failed: bucket=%s key=%s err=%v", bucket, stagingKey, err)
	}
	return nil
}

func publishAgentBinlogSidecar(ctx context.Context, client *s3.Client, bucket, key, filePath string, baseMetadata map[string]string) error {
	metadata := make(map[string]string, len(baseMetadata)+1)
	for k, v := range baseMetadata {
		metadata[k] = v
	}
	metadata["opshub-sidecar"] = filepath.Base(filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	exists, err := agentObjectExistsAndMatches(ctx, client, bucket, key, info.Size(), "")
	if err != nil {
		return fmt.Errorf("校验已存在 binlog sidecar %s 失败: %w", filepath.Base(filePath), err)
	}
	if exists {
		return nil
	}
	if err := uploadAgentObjectFile(ctx, client, bucket, key, filePath, metadata); err != nil {
		return fmt.Errorf("上传 binlog sidecar %s 失败: %w", filepath.Base(filePath), err)
	}
	if err := verifyAgentObject(ctx, client, bucket, key, info.Size(), ""); err != nil {
		return fmt.Errorf("校验 binlog sidecar %s 失败: %w", filepath.Base(filePath), err)
	}
	return nil
}

func newAgentObjectStorageS3Client(storage databaseArchiverStorage) *s3.Client {
	awsCfg := aws.Config{
		Region:      storage.Region,
		Credentials: credentials.NewStaticCredentialsProvider(storage.AccessKey, storage.SecretKey, ""),
	}
	if storage.InsecureSkipVerify {
		awsCfg.HTTPClient = &http.Client{Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Agent 本地显式配置，仅用于内网自签名 MinIO。
		}}
	}
	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = storage.UsePathStyle
		if strings.TrimSpace(storage.Endpoint) != "" {
			options.BaseEndpoint = aws.String(storage.Endpoint)
		}
	})
}

func uploadAgentObjectFile(ctx context.Context, client *s3.Client, bucket, key, filePath string, metadata map[string]string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	uploader := manager.NewUploader(client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(info.Size()),
		Metadata:      metadata,
	})
	return err
}

func verifyAgentObject(ctx context.Context, client *s3.Client, bucket, key string, expectedSize int64, expectedChecksum string) error {
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return err
	}
	if expectedSize >= 0 {
		if head.ContentLength == nil {
			return errors.New("对象存储未返回 ContentLength")
		}
		if *head.ContentLength != expectedSize {
			return fmt.Errorf("对象大小不一致: got=%d expected=%d", *head.ContentLength, expectedSize)
		}
	}
	if strings.TrimSpace(expectedChecksum) != "" {
		value := firstNonEmptyString(head.Metadata["opshub-sha256"], head.Metadata["Opshub-Sha256"])
		if value == "" {
			return errors.New("对象缺少 opshub-sha256 metadata")
		}
		if value != "" && !strings.EqualFold(value, expectedChecksum) {
			return fmt.Errorf("对象 checksum metadata 不一致: got=%s expected=%s", value, expectedChecksum)
		}
	}
	return nil
}

func agentObjectExistsAndMatches(ctx context.Context, client *s3.Client, bucket, key string, expectedSize int64, expectedChecksum string) (bool, error) {
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isAgentObjectNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if expectedSize >= 0 {
		if head.ContentLength == nil {
			return false, errors.New("对象存储未返回 ContentLength")
		}
		if *head.ContentLength != expectedSize {
			return false, fmt.Errorf("对象大小不一致: got=%d expected=%d", *head.ContentLength, expectedSize)
		}
	}
	if strings.TrimSpace(expectedChecksum) != "" {
		value := firstNonEmptyString(head.Metadata["opshub-sha256"], head.Metadata["Opshub-Sha256"])
		if value == "" {
			return false, errors.New("对象缺少 opshub-sha256 metadata")
		}
		if value != "" && !strings.EqualFold(value, expectedChecksum) {
			return false, fmt.Errorf("对象 checksum metadata 不一致: got=%s expected=%s", value, expectedChecksum)
		}
	}
	return true, nil
}

func isAgentObjectNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := strings.ToLower(apiErr.ErrorCode())
		return code == "notfound" || code == "nosuchkey" || code == "404"
	}
	return false
}

func agentObjectStorageURI(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, fileName string) string {
	key := agentObjectStorageFinalKey(cfg, item, fileName)
	scheme := strings.ToLower(strings.TrimSpace(cfg.Storage.Type))
	if scheme == "" {
		scheme = "s3"
	}
	return fmt.Sprintf("%s://%s/%s", scheme, cfg.Storage.Bucket, key)
}

func agentObjectStorageFinalKey(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, fileName string) string {
	return joinAgentObjectKey(cfg.Storage.PathPrefix, agentObjectStorageRelativeKey(item, fileName))
}

func agentObjectStorageStagingKey(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, fileName string) string {
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	return joinAgentObjectKey(cfg.Storage.StagingPrefix, agentObjectStorageRelativeKey(item, fileName)+"."+stamp+".tmp")
}

func agentObjectStorageRelativeKey(item databaseArchiverAssignedStream, fileName string) string {
	return joinAgentObjectKey(
		"mysql-binlog",
		fmt.Sprintf("instance-%d", item.Stream.InstanceID),
		fmt.Sprintf("stream-%d", item.Stream.ID),
		"finalized",
		fileName,
	)
}

func normalizeAgentObjectStorageEndpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Scheme != "" {
		return strings.TrimRight(endpoint, "/")
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return scheme + "://" + strings.TrimRight(endpoint, "/")
}

func cleanAgentObjectKey(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	value = path.Clean(strings.ReplaceAll(value, "\\", "/"))
	if value == "." || value == "/" {
		return ""
	}
	return strings.Trim(value, "/")
}

func joinAgentObjectKey(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, item := range parts {
		if value := cleanAgentObjectKey(item); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return path.Join(cleaned...)
}

func activeFileForMode(mode, current string) string {
	if mode == "streaming" {
		return current
	}
	return ""
}

func ensureTrailingPathSeparator(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZeroInt64Agent(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func databaseArchiverEventPayloadJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if len(text) > 4000 {
		return text[:4000]
	}
	return text
}

func isSafeAgentBinlogFileName(value string) bool {
	if strings.TrimSpace(value) == "" || strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

var agentMySQLBinlogEventLinePattern = regexp.MustCompile(`^#(\d{2})(\d{2})(\d{2})\s+(\d{1,2}):(\d{2}):(\d{2})\s+server id`)

func parseAgentMySQLBinlogEventTime(line string) *time.Time {
	match := agentMySQLBinlogEventLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 7 {
		return nil
	}
	yy, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	second, _ := strconv.Atoi(match[6])
	year := 2000 + yy
	if yy >= 70 {
		year = 1900 + yy
	}
	value := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
	return &value
}
