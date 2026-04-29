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
	"sync"
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
	defaultDatabaseArchiverMaxConcurrent   = 2
	defaultDatabaseArchiverBackoffSeconds  = 30
	defaultDatabaseArchiverMaxBackoffSec   = 300
	defaultDatabaseArchiverLogMaxBytes     = 10 * 1024 * 1024
	defaultDatabaseArchiverLogMaxFiles     = 5
	defaultDatabaseArchiverUploadRetryBase = 60
	defaultDatabaseArchiverUploadRetryMax  = 3600
)

type databaseArchiverConfig struct {
	Enabled                       bool                         `json:"enabled"`
	BaseURL                       string                       `json:"baseUrl"`
	RunnerID                      string                       `json:"runnerId"`
	RunnerAuth                    string                       `json:"runnerAuth"`
	IntervalSeconds               int                          `json:"intervalSeconds"`
	LeaseTTLSeconds               int                          `json:"leaseTtlSeconds"`
	MaxFilesPerLoop               int                          `json:"maxFilesPerLoop"`
	MaxConcurrentStreams          int                          `json:"maxConcurrentStreams"`
	FailureBackoffSeconds         int                          `json:"failureBackoffSeconds"`
	MaxFailureBackoffSeconds      int                          `json:"maxFailureBackoffSeconds"`
	StopNeverEnabled              bool                         `json:"stopNeverEnabled"`
	SpoolResumeEnabled            bool                         `json:"spoolResumeEnabled"`
	UploadRetryEnabled            *bool                        `json:"uploadRetryEnabled"`
	UploadRetryBaseSeconds        int                          `json:"uploadRetryBaseSeconds"`
	UploadRetryMaxSeconds         int                          `json:"uploadRetryMaxSeconds"`
	UploadRetryMaxAttempts        int                          `json:"uploadRetryMaxAttempts"`
	UploadBandwidthBytesPerSecond int64                        `json:"uploadBandwidthBytesPerSecond"`
	StreamingLogMaxBytes          int64                        `json:"streamingLogMaxBytes"`
	StreamingLogMaxFiles          int                          `json:"streamingLogMaxFiles"`
	IncludeCurrent                bool                         `json:"includeCurrent"`
	WorkDir                       string                       `json:"workDir"`
	StorageRoot                   string                       `json:"storageRoot"`
	Storage                       databaseArchiverStorage      `json:"storage"`
	MySQLBinlogPath               string                       `json:"mysqlBinlogPath"`
	Credentials                   []databaseArchiverCredential `json:"credentials"`
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
	Enabled                       bool
	OpsHubBaseURL                 string
	EndpointBaseURL               string
	RunnerID                      string
	RunnerAuth                    string
	Interval                      time.Duration
	LeaseTTLSeconds               int
	MaxFilesPerLoop               int
	MaxConcurrentStreams          int
	FailureBackoff                time.Duration
	MaxFailureBackoff             time.Duration
	StopNeverEnabled              bool
	SpoolResumeEnabled            bool
	UploadRetryEnabled            bool
	UploadRetryBase               time.Duration
	UploadRetryMax                time.Duration
	UploadRetryMaxAttempts        int
	UploadBandwidthBytesPerSecond int64
	StreamingLogMaxBytes          int64
	StreamingLogMaxFiles          int
	IncludeCurrent                bool
	WorkDir                       string
	StorageRoot                   string
	Storage                       databaseArchiverStorage
	MySQLBinlogPath               string
	Credentials                   []databaseArchiverCredential
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

type agentMySQLServerIdentity struct {
	ServerUUID string
	ServerID   string
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

type agentSpoolResult struct {
	FileName        string
	Path            string
	Size            int64
	SourceSize      int64
	Reused          bool
	Resumed         bool
	ResumeFrom      int64
	AppendedBytes   int64
	ValidationMode  string
	LastCompletePos int64
}

type agentStreamingProcess struct {
	StreamID     uint
	ActiveFile   string
	PID          int
	process      *os.Process
	OutputDir    string
	StdoutLog    string
	StderrLog    string
	StartedAt    time.Time
	RestartCount int
	Command      string
	cancel       context.CancelFunc
	done         chan error
	expectedStop bool
}

type agentStreamingProcessExit struct {
	StreamID     uint
	ActiveFile   string
	PID          int
	StartedAt    time.Time
	RestartCount int
	OutputDir    string
	StdoutLog    string
	StderrLog    string
	Err          error
	ExpectedStop bool
}

type agentStreamingProcessStatus struct {
	Running      bool
	Started      bool
	PID          int
	ActiveFile   string
	OutputDir    string
	StdoutLog    string
	StderrLog    string
	StartedAt    time.Time
	RestartCount int
	Exit         *agentStreamingProcessExit
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
	maxConcurrent := raw.MaxConcurrentStreams
	if maxConcurrent <= 0 {
		maxConcurrent = defaultDatabaseArchiverMaxConcurrent
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxConcurrent > 16 {
		maxConcurrent = 16
	}
	backoffSeconds := raw.FailureBackoffSeconds
	if backoffSeconds <= 0 {
		backoffSeconds = defaultDatabaseArchiverBackoffSeconds
	}
	if backoffSeconds < 5 {
		backoffSeconds = 5
	}
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}
	maxBackoffSeconds := raw.MaxFailureBackoffSeconds
	if maxBackoffSeconds <= 0 {
		maxBackoffSeconds = defaultDatabaseArchiverMaxBackoffSec
	}
	if maxBackoffSeconds < backoffSeconds {
		maxBackoffSeconds = backoffSeconds
	}
	if maxBackoffSeconds > 3600 {
		maxBackoffSeconds = 3600
	}
	streamingLogMaxBytes := raw.StreamingLogMaxBytes
	if streamingLogMaxBytes <= 0 {
		streamingLogMaxBytes = defaultDatabaseArchiverLogMaxBytes
	}
	if streamingLogMaxBytes < 1024*1024 {
		streamingLogMaxBytes = 1024 * 1024
	}
	if streamingLogMaxBytes > 1024*1024*1024 {
		streamingLogMaxBytes = 1024 * 1024 * 1024
	}
	streamingLogMaxFiles := raw.StreamingLogMaxFiles
	if streamingLogMaxFiles <= 0 {
		streamingLogMaxFiles = defaultDatabaseArchiverLogMaxFiles
	}
	if streamingLogMaxFiles < 1 {
		streamingLogMaxFiles = 1
	}
	if streamingLogMaxFiles > 50 {
		streamingLogMaxFiles = 50
	}
	uploadRetryBaseSeconds := raw.UploadRetryBaseSeconds
	if uploadRetryBaseSeconds <= 0 {
		uploadRetryBaseSeconds = defaultDatabaseArchiverUploadRetryBase
	}
	if uploadRetryBaseSeconds < 5 {
		uploadRetryBaseSeconds = 5
	}
	if uploadRetryBaseSeconds > 3600 {
		uploadRetryBaseSeconds = 3600
	}
	uploadRetryMaxSeconds := raw.UploadRetryMaxSeconds
	if uploadRetryMaxSeconds <= 0 {
		uploadRetryMaxSeconds = defaultDatabaseArchiverUploadRetryMax
	}
	if uploadRetryMaxSeconds < uploadRetryBaseSeconds {
		uploadRetryMaxSeconds = uploadRetryBaseSeconds
	}
	if uploadRetryMaxSeconds > 86400 {
		uploadRetryMaxSeconds = 86400
	}
	uploadRetryEnabled := true
	if raw.UploadRetryEnabled != nil {
		uploadRetryEnabled = *raw.UploadRetryEnabled
	}
	uploadRetryMaxAttempts := raw.UploadRetryMaxAttempts
	if uploadRetryMaxAttempts < 0 {
		uploadRetryMaxAttempts = 0
	}
	if uploadRetryMaxAttempts > 10000 {
		uploadRetryMaxAttempts = 10000
	}
	uploadBandwidthBytesPerSecond := raw.UploadBandwidthBytesPerSecond
	if uploadBandwidthBytesPerSecond < 0 {
		uploadBandwidthBytesPerSecond = 0
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
		Enabled:                       true,
		OpsHubBaseURL:                 strings.TrimRight(opsHubBaseURL, "/"),
		EndpointBaseURL:               endpointBase,
		RunnerID:                      runnerID,
		RunnerAuth:                    runnerAuth,
		Interval:                      time.Duration(intervalSeconds) * time.Second,
		LeaseTTLSeconds:               leaseTTLSeconds,
		MaxFilesPerLoop:               maxFiles,
		MaxConcurrentStreams:          maxConcurrent,
		FailureBackoff:                time.Duration(backoffSeconds) * time.Second,
		MaxFailureBackoff:             time.Duration(maxBackoffSeconds) * time.Second,
		StopNeverEnabled:              raw.StopNeverEnabled,
		SpoolResumeEnabled:            raw.SpoolResumeEnabled,
		UploadRetryEnabled:            uploadRetryEnabled,
		UploadRetryBase:               time.Duration(uploadRetryBaseSeconds) * time.Second,
		UploadRetryMax:                time.Duration(uploadRetryMaxSeconds) * time.Second,
		UploadRetryMaxAttempts:        uploadRetryMaxAttempts,
		UploadBandwidthBytesPerSecond: uploadBandwidthBytesPerSecond,
		StreamingLogMaxBytes:          streamingLogMaxBytes,
		StreamingLogMaxFiles:          streamingLogMaxFiles,
		IncludeCurrent:                raw.IncludeCurrent,
		WorkDir:                       workDir,
		StorageRoot:                   storageRoot,
		Storage:                       storage,
		MySQLBinlogPath:               strings.TrimSpace(raw.MySQLBinlogPath),
		Credentials:                   append([]databaseArchiverCredential{}, raw.Credentials...),
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
	defer a.stopAllDatabaseArchiverStreamingProcesses(context.Background(), cfg, "agent stopped")
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
		a.stopAllDatabaseArchiverStreamingProcesses(ctx, cfg, "heartbeat failed")
		return
	}
	a.processAgentUploadRetryQueue(ctx, cfg)
	streams, err := a.databaseArchiverListStreams(ctx, cfg)
	if err != nil {
		log.Printf("database archiver list streams failed: %v", err)
		a.stopAllDatabaseArchiverStreamingProcesses(ctx, cfg, "list streams failed")
		return
	}
	activeStreamIDs := map[uint]bool{}
	for _, stream := range streams.Streams {
		if stream.Stream.ID > 0 {
			activeStreamIDs[stream.Stream.ID] = true
		}
	}
	a.stopDatabaseArchiverStreamingProcessesNotIn(ctx, cfg, activeStreamIDs, "stream no longer assigned to this runner")
	running := 0
	var wg sync.WaitGroup
	maxConcurrent := cfg.MaxConcurrentStreams
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	sem := make(chan struct{}, maxConcurrent)
	for _, stream := range streams.Streams {
		if stream.Stream.ID == 0 {
			continue
		}
		if remaining := a.databaseArchiverBackoffRemaining(stream.Stream.ID); remaining > 0 {
			log.Printf("database archiver stream %d skipped by failure backoff: remaining=%s", stream.Stream.ID, remaining.Round(time.Second))
			continue
		}
		running++
		wg.Add(1)
		sem <- struct{}{}
		go func(item databaseArchiverAssignedStream) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := a.processDatabaseArchiverStream(ctx, cfg, item); err != nil {
				delay := a.databaseArchiverMarkFailure(cfg, item.Stream.ID)
				log.Printf("database archiver stream %d failed: %v; backoff=%s", item.Stream.ID, err, delay.Round(time.Second))
				return
			}
			a.databaseArchiverMarkSuccess(item.Stream.ID)
		}(stream)
	}
	wg.Wait()
	if err := a.databaseArchiverHeartbeat(ctx, cfg, "online", "", running); err != nil {
		log.Printf("database archiver heartbeat failed: %v", err)
	}
}

func (a *agentApp) databaseArchiverBackoffRemaining(streamID uint) time.Duration {
	if streamID == 0 {
		return 0
	}
	a.databaseArchiverBackoffMu.Lock()
	defer a.databaseArchiverBackoffMu.Unlock()
	if a.databaseArchiverBackoffUntil == nil {
		return 0
	}
	until := a.databaseArchiverBackoffUntil[streamID]
	if until.IsZero() {
		return 0
	}
	remaining := time.Until(until)
	if remaining <= 0 {
		delete(a.databaseArchiverBackoffUntil, streamID)
		return 0
	}
	return remaining
}

func (a *agentApp) databaseArchiverMarkFailure(cfg *resolvedDatabaseArchiverConfig, streamID uint) time.Duration {
	if streamID == 0 {
		return 0
	}
	a.databaseArchiverBackoffMu.Lock()
	defer a.databaseArchiverBackoffMu.Unlock()
	if a.databaseArchiverFailures == nil {
		a.databaseArchiverFailures = map[uint]int{}
	}
	if a.databaseArchiverBackoffUntil == nil {
		a.databaseArchiverBackoffUntil = map[uint]time.Time{}
	}
	a.databaseArchiverFailures[streamID]++
	failures := a.databaseArchiverFailures[streamID]
	delay := cfg.FailureBackoff
	for i := 1; i < failures; i++ {
		delay *= 2
		if delay >= cfg.MaxFailureBackoff {
			delay = cfg.MaxFailureBackoff
			break
		}
	}
	if delay <= 0 {
		delay = time.Duration(defaultDatabaseArchiverBackoffSeconds) * time.Second
	}
	if cfg.MaxFailureBackoff > 0 && delay > cfg.MaxFailureBackoff {
		delay = cfg.MaxFailureBackoff
	}
	a.databaseArchiverBackoffUntil[streamID] = time.Now().Add(delay)
	return delay
}

func (a *agentApp) databaseArchiverMarkSuccess(streamID uint) {
	if streamID == 0 {
		return
	}
	a.databaseArchiverBackoffMu.Lock()
	defer a.databaseArchiverBackoffMu.Unlock()
	delete(a.databaseArchiverFailures, streamID)
	delete(a.databaseArchiverBackoffUntil, streamID)
}

func (a *agentApp) ensureDatabaseArchiverStreamingProcess(
	ctx context.Context,
	cfg *resolvedDatabaseArchiverConfig,
	item databaseArchiverAssignedStream,
	credential databaseArchiverCredential,
	tool string,
	activeFile string,
) (agentStreamingProcessStatus, error) {
	status := agentStreamingProcessStatus{}
	if exit := a.collectDatabaseArchiverStreamingExit(item.Stream.ID); exit != nil {
		status.Exit = exit
	}
	if !isSafeAgentBinlogFileName(activeFile) {
		return status, fmt.Errorf("binlog 文件名不合法: %s", activeFile)
	}
	if running := a.databaseArchiverStreamingProcessStatus(item.Stream.ID); running.Running {
		running.Exit = status.Exit
		return running, nil
	}
	started, err := a.startDatabaseArchiverStreamingProcess(ctx, cfg, item, credential, tool, activeFile)
	started.Exit = status.Exit
	if err != nil {
		return started, err
	}
	return started, nil
}

func (a *agentApp) databaseArchiverStreamingProcessStatus(streamID uint) agentStreamingProcessStatus {
	a.databaseArchiverProcessMu.Lock()
	defer a.databaseArchiverProcessMu.Unlock()
	if a.databaseArchiverProcesses == nil {
		return agentStreamingProcessStatus{}
	}
	proc := a.databaseArchiverProcesses[streamID]
	if proc == nil {
		return agentStreamingProcessStatus{}
	}
	return agentStreamingProcessStatus{
		Running:      true,
		PID:          proc.PID,
		ActiveFile:   proc.ActiveFile,
		OutputDir:    proc.OutputDir,
		StdoutLog:    proc.StdoutLog,
		StderrLog:    proc.StderrLog,
		StartedAt:    proc.StartedAt,
		RestartCount: proc.RestartCount,
	}
}

func (a *agentApp) collectDatabaseArchiverStreamingExit(streamID uint) *agentStreamingProcessExit {
	a.databaseArchiverProcessMu.Lock()
	defer a.databaseArchiverProcessMu.Unlock()
	if a.databaseArchiverProcesses == nil {
		return nil
	}
	proc := a.databaseArchiverProcesses[streamID]
	if proc == nil || proc.done == nil {
		return nil
	}
	select {
	case err := <-proc.done:
		delete(a.databaseArchiverProcesses, streamID)
		return &agentStreamingProcessExit{
			StreamID:     proc.StreamID,
			ActiveFile:   proc.ActiveFile,
			PID:          proc.PID,
			StartedAt:    proc.StartedAt,
			RestartCount: proc.RestartCount,
			OutputDir:    proc.OutputDir,
			StdoutLog:    proc.StdoutLog,
			StderrLog:    proc.StderrLog,
			Err:          err,
			ExpectedStop: proc.expectedStop,
		}
	default:
		return nil
	}
}

func (a *agentApp) startDatabaseArchiverStreamingProcess(
	ctx context.Context,
	cfg *resolvedDatabaseArchiverConfig,
	item databaseArchiverAssignedStream,
	credential databaseArchiverCredential,
	tool string,
	activeFile string,
) (agentStreamingProcessStatus, error) {
	outputDir := agentBinlogStreamingSpoolDir(cfg, item)
	logDir := agentBinlogProcessLogDir(cfg, item)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return agentStreamingProcessStatus{}, err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return agentStreamingProcessStatus{}, err
	}
	defaultsFile, err := writeAgentMySQLDefaultsFile(cfg.WorkDir, item.SourceInstance, credential)
	if err != nil {
		return agentStreamingProcessStatus{}, err
	}
	stdoutPath := filepath.Join(logDir, "mysqlbinlog.stdout.log")
	stderrPath := filepath.Join(logDir, "mysqlbinlog.stderr.log")
	stdoutLog, err := newAgentRollingLogWriter(stdoutPath, cfg.StreamingLogMaxBytes, cfg.StreamingLogMaxFiles)
	if err != nil {
		_ = os.Remove(defaultsFile)
		return agentStreamingProcessStatus{}, err
	}
	stderrLog, err := newAgentRollingLogWriter(stderrPath, cfg.StreamingLogMaxBytes, cfg.StreamingLogMaxFiles)
	if err != nil {
		_ = stdoutLog.Close()
		_ = os.Remove(defaultsFile)
		return agentStreamingProcessStatus{}, err
	}
	processCtx, cancel := context.WithCancel(ctx)
	args := []string{
		"--defaults-extra-file=" + defaultsFile,
		"--read-from-remote-server",
		"--raw",
		"--stop-never",
		"--result-file=" + ensureTrailingPathSeparator(outputDir),
		activeFile,
	}
	cmd := exec.CommandContext(processCtx, tool, args...)
	cmd.Stdout = stdoutLog
	cmd.Stderr = stderrLog
	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdoutLog.Close()
		_ = stderrLog.Close()
		_ = os.Remove(defaultsFile)
		return agentStreamingProcessStatus{}, fmt.Errorf("启动 mysqlbinlog --stop-never 失败: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = stdoutLog.Close()
		_ = stderrLog.Close()
		_ = os.Remove(defaultsFile)
		done <- err
	}()
	a.databaseArchiverProcessMu.Lock()
	if a.databaseArchiverProcesses == nil {
		a.databaseArchiverProcesses = map[uint]*agentStreamingProcess{}
	}
	if a.databaseArchiverProcessStarts == nil {
		a.databaseArchiverProcessStarts = map[uint]int{}
	}
	restartCount := a.databaseArchiverProcessStarts[item.Stream.ID]
	a.databaseArchiverProcessStarts[item.Stream.ID] = restartCount + 1
	proc := &agentStreamingProcess{
		StreamID:     item.Stream.ID,
		ActiveFile:   activeFile,
		PID:          cmd.Process.Pid,
		process:      cmd.Process,
		OutputDir:    outputDir,
		StdoutLog:    stdoutPath,
		StderrLog:    stderrPath,
		StartedAt:    time.Now(),
		RestartCount: restartCount,
		Command:      strings.Join(append([]string{filepath.Base(tool)}, args[1:]...), " "),
		cancel:       cancel,
		done:         done,
	}
	a.databaseArchiverProcesses[item.Stream.ID] = proc
	a.databaseArchiverProcessMu.Unlock()
	return agentStreamingProcessStatus{
		Running:      true,
		Started:      true,
		PID:          proc.PID,
		ActiveFile:   proc.ActiveFile,
		OutputDir:    proc.OutputDir,
		StdoutLog:    proc.StdoutLog,
		StderrLog:    proc.StderrLog,
		StartedAt:    proc.StartedAt,
		RestartCount: proc.RestartCount,
	}, nil
}

func (a *agentApp) stopDatabaseArchiverStreamingProcessesNotIn(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, active map[uint]bool, reason string) {
	for _, streamID := range a.databaseArchiverStreamingProcessIDs() {
		if !active[streamID] {
			a.stopDatabaseArchiverStreamingProcess(ctx, cfg, streamID, reason)
		}
	}
}

func (a *agentApp) stopAllDatabaseArchiverStreamingProcesses(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, reason string) {
	for _, streamID := range a.databaseArchiverStreamingProcessIDs() {
		a.stopDatabaseArchiverStreamingProcess(ctx, cfg, streamID, reason)
	}
}

func (a *agentApp) databaseArchiverStreamingProcessIDs() []uint {
	a.databaseArchiverProcessMu.Lock()
	defer a.databaseArchiverProcessMu.Unlock()
	ids := make([]uint, 0, len(a.databaseArchiverProcesses))
	for id := range a.databaseArchiverProcesses {
		ids = append(ids, id)
	}
	return ids
}

func (a *agentApp) stopDatabaseArchiverStreamingProcess(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, streamID uint, reason string) {
	proc := a.detachDatabaseArchiverStreamingProcess(streamID)
	if proc == nil {
		return
	}
	proc.expectedStop = true
	if proc.cancel != nil {
		proc.cancel()
	}
	var waitErr error
	timedOut := false
	select {
	case waitErr = <-proc.done:
	case <-time.After(10 * time.Second):
		if proc.process != nil {
			_ = proc.process.Kill()
		}
		select {
		case waitErr = <-proc.done:
		case <-time.After(5 * time.Second):
			waitErr = errors.New("停止 mysqlbinlog --stop-never 进程超时")
			timedOut = true
		}
	}
	level := "info"
	message := "mysqlbinlog --stop-never 进程已停止"
	if strings.TrimSpace(reason) != "" {
		message += ": " + strings.TrimSpace(reason)
	}
	if timedOut && waitErr != nil {
		level = "warning"
	}
	item := databaseArchiverAssignedStream{Stream: databaseArchiverStream{ID: streamID}}
	_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
		EventType:  "agent_message",
		Level:      level,
		Message:    message,
		ActiveFile: proc.ActiveFile,
		PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
			"pid":          proc.PID,
			"activeFile":   proc.ActiveFile,
			"outputDir":    proc.OutputDir,
			"stdoutLog":    proc.StdoutLog,
			"stderrLog":    proc.StderrLog,
			"startedAt":    proc.StartedAt.Format(time.RFC3339),
			"restartCount": proc.RestartCount,
			"reason":       strings.TrimSpace(reason),
		}),
		OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (a *agentApp) postDatabaseArchiverStreamingCheckpoint(
	ctx context.Context,
	cfg *resolvedDatabaseArchiverConfig,
	item databaseArchiverAssignedStream,
	processStatus agentStreamingProcessStatus,
	current agentMySQLBinaryLog,
	sourceStatus agentMySQLBinaryLogStatus,
	identity agentMySQLServerIdentity,
) {
	if !processStatus.Running || item.Stream.ID == 0 {
		return
	}
	snapshot := agentStreamingSpoolSnapshot(cfg, item, firstNonEmptyString(processStatus.ActiveFile, current.Name))
	payload := map[string]any{
		"pid":              processStatus.PID,
		"activeFile":       firstNonEmptyString(processStatus.ActiveFile, current.Name),
		"outputDir":        processStatus.OutputDir,
		"stdoutLog":        processStatus.StdoutLog,
		"stderrLog":        processStatus.StderrLog,
		"startedAt":        processStatus.StartedAt.Format(time.RFC3339),
		"restartCount":     processStatus.RestartCount,
		"sourceFile":       firstNonEmptyString(sourceStatus.File, current.Name),
		"sourcePos":        firstNonZeroInt64Agent(sourceStatus.Position, current.Size),
		"sourceSize":       current.Size,
		"serverUUID":       identity.ServerUUID,
		"serverID":         identity.ServerID,
		"spoolPath":        snapshot.Path,
		"spoolExists":      snapshot.Exists,
		"spoolSize":        snapshot.Size,
		"spoolUpdatedAt":   snapshot.UpdatedAt,
		"lastGrowthAt":     snapshot.UpdatedAt,
		"streamingManaged": true,
	}
	_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
		EventType:   "checkpoint",
		Level:       "info",
		Message:     "streaming mysqlbinlog 进程 checkpoint",
		ActiveFile:  firstNonEmptyString(processStatus.ActiveFile, current.Name),
		PayloadJSON: databaseArchiverEventPayloadJSON(payload),
		OccurredAt:  time.Now().Format("2006-01-02 15:04:05"),
	})
}

type agentStreamingSpoolSnapshotResult struct {
	Path      string
	Exists    bool
	Size      int64
	UpdatedAt string
}

func agentStreamingSpoolSnapshot(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, activeFile string) agentStreamingSpoolSnapshotResult {
	if !isSafeAgentBinlogFileName(activeFile) {
		return agentStreamingSpoolSnapshotResult{}
	}
	path := filepath.Join(agentBinlogStreamingSpoolDir(cfg, item), activeFile)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return agentStreamingSpoolSnapshotResult{Path: path}
	}
	return agentStreamingSpoolSnapshotResult{
		Path:      path,
		Exists:    true,
		Size:      info.Size(),
		UpdatedAt: info.ModTime().Format(time.RFC3339),
	}
}

func (a *agentApp) detachDatabaseArchiverStreamingProcess(streamID uint) *agentStreamingProcess {
	a.databaseArchiverProcessMu.Lock()
	defer a.databaseArchiverProcessMu.Unlock()
	if a.databaseArchiverProcesses == nil {
		return nil
	}
	proc := a.databaseArchiverProcesses[streamID]
	if proc == nil {
		return nil
	}
	proc.expectedStop = true
	delete(a.databaseArchiverProcesses, streamID)
	return proc
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
	identity := agentReadMySQLServerIdentity(ctx, db)
	current := currentAgentBinaryLog(logs)
	eventActive = activeFileForMode(mode, current.Name)
	eventPayload["sourceFile"] = firstNonEmptyString(status.File, current.Name)
	eventPayload["sourcePos"] = firstNonZeroInt64Agent(status.Position, current.Size)
	if identity.ServerUUID != "" {
		eventPayload["serverUUID"] = identity.ServerUUID
	}
	if identity.ServerID != "" {
		eventPayload["serverID"] = identity.ServerID
	}
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
	if mode != "streaming" || !cfg.StopNeverEnabled {
		a.stopDatabaseArchiverStreamingProcess(ctx, cfg, item.Stream.ID, "streaming process disabled or mode changed")
	}
	if mode == "streaming" && current.Name != "" {
		if cfg.StopNeverEnabled {
			processStatus, processErr := a.ensureDatabaseArchiverStreamingProcess(ctx, cfg, item, credential, tool, current.Name)
			if processStatus.Exit != nil {
				level := "warning"
				message := "mysqlbinlog --stop-never 进程异常退出，等待自动重启"
				if processStatus.Exit.ExpectedStop {
					level = "info"
					message = "mysqlbinlog --stop-never 进程已停止"
				}
				if processStatus.Exit.Err != nil && !processStatus.Exit.ExpectedStop {
					message += ": " + processStatus.Exit.Err.Error()
				}
				_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
					EventType:  "agent_message",
					Level:      level,
					Message:    message,
					ActiveFile: processStatus.Exit.ActiveFile,
					PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
						"pid":          processStatus.Exit.PID,
						"activeFile":   processStatus.Exit.ActiveFile,
						"outputDir":    processStatus.Exit.OutputDir,
						"stdoutLog":    processStatus.Exit.StdoutLog,
						"stderrLog":    processStatus.Exit.StderrLog,
						"startedAt":    processStatus.Exit.StartedAt.Format(time.RFC3339),
						"restartCount": processStatus.Exit.RestartCount,
						"expectedStop": processStatus.Exit.ExpectedStop,
						"exitError":    agentErrorString(processStatus.Exit.Err),
					}),
					OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
				})
			}
			if processErr != nil {
				_ = a.databaseArchiverCheckpoint(ctx, cfg, item, databaseArchiverCheckpointRequest{
					DaemonStatus:        "degraded",
					ActiveFile:          current.Name,
					LastSourceFile:      firstNonEmptyString(status.File, current.Name),
					LastSourcePos:       firstNonZeroInt64Agent(status.Position, current.Size),
					LastError:           "stop-never streaming 进程失败: " + processErr.Error(),
					ConsecutiveFailures: 1,
					LeaseTTLSeconds:     cfg.LeaseTTLSeconds,
				})
				return processErr
			}
			if processStatus.Started {
				_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
					EventType:  "agent_message",
					Level:      "info",
					Message:    "mysqlbinlog --stop-never 进程已启动",
					ActiveFile: processStatus.ActiveFile,
					PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
						"pid":          processStatus.PID,
						"activeFile":   processStatus.ActiveFile,
						"outputDir":    processStatus.OutputDir,
						"stdoutLog":    processStatus.StdoutLog,
						"stderrLog":    processStatus.StderrLog,
						"startedAt":    processStatus.StartedAt.Format(time.RFC3339),
						"restartCount": processStatus.RestartCount,
					}),
					OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
				})
			}
			a.postDatabaseArchiverStreamingCheckpoint(ctx, cfg, item, processStatus, current, status, identity)
		} else {
			spool, err := spoolAgentActiveBinlog(ctx, cfg, item, credential, identity, tool, current.Name, current.Size)
			if err != nil {
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
			message := "active binlog spool 已更新"
			if spool.Reused {
				message = "active binlog spool 未变化，跳过重复拉取"
			}
			_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
				EventType:  "spool_updated",
				Level:      "info",
				Message:    message,
				ActiveFile: current.Name,
				PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
					"activeFile":      current.Name,
					"sourcePos":       firstNonZeroInt64Agent(status.Position, current.Size),
					"sourceSize":      spool.SourceSize,
					"spoolSize":       spool.Size,
					"reused":          spool.Reused,
					"resumed":         spool.Resumed,
					"resumeFrom":      spool.ResumeFrom,
					"appendBytes":     spool.AppendedBytes,
					"validationMode":  spool.ValidationMode,
					"lastCompletePos": spool.LastCompletePos,
					"serverUUID":      identity.ServerUUID,
					"serverID":        identity.ServerID,
				}),
				OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
			})
		}
	}
	var lastArtifact *agentBinlogArtifact
	for _, selection := range selections {
		eventFile = selection.FileName
		artifact, err := a.archiveAgentBinlogSelection(ctx, cfg, item, credential, tool, selection, mode == "streaming" && cfg.StopNeverEnabled)
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

func agentReadMySQLServerIdentity(ctx context.Context, db *sql.DB) agentMySQLServerIdentity {
	if db == nil {
		return agentMySQLServerIdentity{}
	}
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var uuidValue, idValue sql.NullString
	if err := db.QueryRowContext(queryCtx, "SELECT @@server_uuid, @@server_id").Scan(&uuidValue, &idValue); err == nil {
		return agentMySQLServerIdentity{
			ServerUUID: strings.TrimSpace(uuidValue.String),
			ServerID:   strings.TrimSpace(idValue.String),
		}
	}
	queryCtx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel2()
	if err := db.QueryRowContext(queryCtx2, "SELECT @@server_id").Scan(&idValue); err == nil {
		return agentMySQLServerIdentity{ServerID: strings.TrimSpace(idValue.String)}
	}
	return agentMySQLServerIdentity{}
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

func (a *agentApp) archiveAgentBinlogSelection(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, tool string, selection agentBinlogArchiveSelection, preferStreamingSpool bool) (agentBinlogArtifact, error) {
	if preferStreamingSpool {
		artifact, err := archiveAgentStreamingSpoolFile(ctx, cfg, item, tool, selection)
		if err == nil {
			_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
				EventType:  "agent_message",
				Level:      "info",
				Message:    "streaming spool 已校验并提交为 finalized binlog",
				FileName:   selection.FileName,
				ActiveFile: item.Stream.ActiveFile,
				PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
					"fileName":       artifact.FileName,
					"fileSize":       artifact.FileSize,
					"checksumSha256": artifact.ChecksumSHA256,
					"storageUri":     artifact.StorageURI,
					"source":         "streaming_spool",
				}),
				OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
			})
			return artifact, nil
		}
		payload := map[string]any{
			"fileName": selection.FileName,
			"source":   "streaming_spool",
			"fallback": "remote_full_download",
		}
		if quarantinePath, quarantineErr := quarantineAgentStreamingSpoolFile(cfg, item, selection.FileName); quarantineErr == nil && quarantinePath != "" {
			payload["quarantinePath"] = quarantinePath
		} else if quarantineErr != nil {
			payload["quarantineError"] = quarantineErr.Error()
		}
		_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
			EventType:   "agent_message",
			Level:       "warning",
			Message:     "streaming spool 无法直接提交，回退为完整拉取: " + err.Error(),
			FileName:    selection.FileName,
			ActiveFile:  item.Stream.ActiveFile,
			PayloadJSON: databaseArchiverEventPayloadJSON(payload),
			OccurredAt:  time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	return archiveAgentBinlogFile(ctx, cfg, item, credential, tool, selection)
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
	return finalizeAgentBinlogArtifact(ctx, cfg, item, artifact)
}

func archiveAgentStreamingSpoolFile(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, tool string, selection agentBinlogArchiveSelection) (agentBinlogArtifact, error) {
	if !isSafeAgentBinlogFileName(selection.FileName) {
		return agentBinlogArtifact{}, fmt.Errorf("binlog 文件名不合法: %s", selection.FileName)
	}
	sourcePath := filepath.Join(agentBinlogStreamingSpoolDir(cfg, item), selection.FileName)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return agentBinlogArtifact{}, fmt.Errorf("streaming spool 文件不存在: %w", err)
	}
	if info.IsDir() || info.Size() <= 0 {
		return agentBinlogArtifact{}, fmt.Errorf("streaming spool 文件无效: %s", sourcePath)
	}
	if selection.FileSize > 0 && info.Size() != selection.FileSize {
		return agentBinlogArtifact{}, fmt.Errorf("streaming spool 文件大小 %d 与源库记录 %d 不一致", info.Size(), selection.FileSize)
	}
	validation, err := validateAgentBinlogFile(sourcePath)
	if err != nil {
		return agentBinlogArtifact{}, fmt.Errorf("streaming spool binlog 校验失败: %w", err)
	}
	if validation.LastCompletePos != info.Size() {
		return agentBinlogArtifact{}, fmt.Errorf("streaming spool 末尾不是完整事件边界: complete=%d size=%d", validation.LastCompletePos, info.Size())
	}
	finalDir := agentBinlogFinalDir(cfg, item)
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return agentBinlogArtifact{}, err
	}
	tmpDir, err := os.MkdirTemp(finalDir, ".opshub-streaming-finalize-")
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, selection.FileName)
	if err := copyAgentFile(sourcePath, tmpPath, 0o600); err != nil {
		return agentBinlogArtifact{}, err
	}
	finalPath := filepath.Join(finalDir, selection.FileName)
	if err := commitAgentBinlogFile(tmpPath, finalPath); err != nil {
		return agentBinlogArtifact{}, err
	}
	artifact, err := buildAgentBinlogArtifact(ctx, tool, finalPath, selection, item)
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	artifact, err = finalizeAgentBinlogArtifact(ctx, cfg, item, artifact)
	if err != nil {
		return agentBinlogArtifact{}, err
	}
	_ = os.Remove(sourcePath)
	return artifact, nil
}

func quarantineAgentStreamingSpoolFile(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, fileName string) (string, error) {
	if !isSafeAgentBinlogFileName(fileName) {
		return "", fmt.Errorf("binlog 文件名不合法: %s", fileName)
	}
	sourcePath := filepath.Join(agentBinlogStreamingSpoolDir(cfg, item), fileName)
	info, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", nil
	}
	quarantineDir := filepath.Join(agentBinlogSpoolDir(cfg, item), "quarantine")
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(quarantineDir, fmt.Sprintf("%s.%s.invalid", fileName, time.Now().UTC().Format("20060102T150405Z")))
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return "", err
	}
	manifestPath := sourcePath + ".manifest.json"
	if _, err := os.Stat(manifestPath); err == nil {
		_ = os.Rename(manifestPath, targetPath+".manifest.json")
	}
	return targetPath, nil
}

func spoolAgentActiveBinlog(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, identity agentMySQLServerIdentity, tool, fileName string, sourceSize int64) (agentSpoolResult, error) {
	if !isSafeAgentBinlogFileName(fileName) {
		return agentSpoolResult{}, fmt.Errorf("binlog 文件名不合法: %s", fileName)
	}
	spoolDir := agentBinlogSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		return agentSpoolResult{}, err
	}
	target := filepath.Join(spoolDir, fileName+".partial")
	if sourceSize > 0 {
		if info, err := os.Stat(target); err == nil && info.Size() >= sourceSize {
			validation, validationErr := validateAgentBinlogFile(target)
			if validationErr == nil {
				if err := writeAgentPartialManifest(target, fileName, sourceSize, validation, identity); err != nil {
					return agentSpoolResult{}, err
				}
				return agentSpoolResult{
					FileName:        fileName,
					Path:            target,
					Size:            info.Size(),
					SourceSize:      sourceSize,
					Reused:          true,
					ValidationMode:  validation.ChecksumMode,
					LastCompletePos: validation.LastCompletePos,
				}, nil
			}
		}
	}
	if cfg.SpoolResumeEnabled && sourceSize > 0 {
		if result, err := resumeAgentActiveBinlogSpool(ctx, cfg, item, credential, identity, tool, fileName, sourceSize, target); err == nil {
			return result, nil
		}
	}
	tmpDir, err := os.MkdirTemp(spoolDir, ".opshub-spool-")
	if err != nil {
		return agentSpoolResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	if err := runAgentMysqlbinlogRaw(ctx, cfg, item, credential, tool, tmpDir, fileName); err != nil {
		return agentSpoolResult{}, err
	}
	tmpPath := filepath.Join(tmpDir, fileName)
	if err := replaceAgentBinlogFile(tmpPath, target); err != nil {
		return agentSpoolResult{}, err
	}
	size := int64(0)
	if info, err := os.Stat(target); err == nil {
		size = info.Size()
	}
	validationMode := ""
	lastCompletePos := int64(0)
	if validation, validationErr := validateAgentBinlogFile(target); validationErr == nil {
		validationMode = validation.ChecksumMode
		lastCompletePos = validation.LastCompletePos
		if err := writeAgentPartialManifest(target, fileName, sourceSize, validation, identity); err != nil {
			return agentSpoolResult{}, err
		}
	}
	return agentSpoolResult{
		FileName:        fileName,
		Path:            target,
		Size:            size,
		SourceSize:      sourceSize,
		ValidationMode:  validationMode,
		LastCompletePos: lastCompletePos,
	}, nil
}

func resumeAgentActiveBinlogSpool(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, identity agentMySQLServerIdentity, tool, fileName string, sourceSize int64, target string) (agentSpoolResult, error) {
	info, err := os.Stat(target)
	if err != nil {
		return agentSpoolResult{}, err
	}
	if info.IsDir() || info.Size() <= 4 || info.Size() >= sourceSize {
		return agentSpoolResult{}, fmt.Errorf("partial 文件大小不适合 resume: size=%d source=%d", info.Size(), sourceSize)
	}
	if err := validateAgentPartialManifestForResume(target, fileName, identity); err != nil {
		return agentSpoolResult{}, err
	}
	existingValidation, err := validateAgentBinlogFile(target)
	if err != nil {
		return agentSpoolResult{}, fmt.Errorf("partial binlog 校验失败: %w", err)
	}
	if existingValidation.LastCompletePos != info.Size() {
		return agentSpoolResult{}, fmt.Errorf("partial 末尾不是完整事件边界: complete=%d size=%d", existingValidation.LastCompletePos, info.Size())
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(target), ".opshub-resume-")
	if err != nil {
		return agentSpoolResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	if err := runAgentMysqlbinlogRawFromPosition(ctx, cfg, item, credential, tool, tmpDir, fileName, existingValidation.LastCompletePos); err != nil {
		return agentSpoolResult{}, err
	}
	candidatePath := filepath.Join(tmpDir, fileName)
	appendPlan, err := validateAgentBinlogAppendCandidate(target, candidatePath)
	if err != nil {
		return agentSpoolResult{}, err
	}
	combinedPath := filepath.Join(tmpDir, fileName+".combined")
	if err := combineAgentBinlogAppend(target, candidatePath, appendPlan.PayloadOffset, combinedPath); err != nil {
		return agentSpoolResult{}, err
	}
	combinedValidation, err := validateAgentBinlogFile(combinedPath)
	if err != nil {
		return agentSpoolResult{}, fmt.Errorf("resume 合并后 binlog 校验失败: %w", err)
	}
	if combinedValidation.LastCompletePos <= existingValidation.LastCompletePos {
		return agentSpoolResult{}, fmt.Errorf("resume 未产生新增完整事件: before=%d after=%d", existingValidation.LastCompletePos, combinedValidation.LastCompletePos)
	}
	if err := replaceAgentBinlogFile(combinedPath, target); err != nil {
		return agentSpoolResult{}, err
	}
	if err := writeAgentPartialManifest(target, fileName, sourceSize, combinedValidation, identity); err != nil {
		return agentSpoolResult{}, err
	}
	size := int64(0)
	if info, err := os.Stat(target); err == nil {
		size = info.Size()
	}
	return agentSpoolResult{
		FileName:        fileName,
		Path:            target,
		Size:            size,
		SourceSize:      sourceSize,
		Resumed:         true,
		ResumeFrom:      existingValidation.LastCompletePos,
		AppendedBytes:   appendPlan.PayloadBytes,
		ValidationMode:  combinedValidation.ChecksumMode,
		LastCompletePos: combinedValidation.LastCompletePos,
	}, nil
}

type agentPartialManifest struct {
	FileName        string `json:"fileName"`
	SourceSize      int64  `json:"sourceSize"`
	LastCompletePos int64  `json:"lastCompletePos"`
	ChecksumMode    string `json:"checksumMode"`
	ServerUUID      string `json:"serverUUID,omitempty"`
	ServerID        string `json:"serverID,omitempty"`
	UpdatedAt       string `json:"updatedAt"`
}

func agentPartialManifestPath(partialPath string) string {
	return partialPath + ".manifest.json"
}

func readAgentPartialManifest(partialPath string) (agentPartialManifest, error) {
	data, err := os.ReadFile(agentPartialManifestPath(partialPath))
	if err != nil {
		return agentPartialManifest{}, err
	}
	var manifest agentPartialManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return agentPartialManifest{}, err
	}
	return manifest, nil
}

func writeAgentPartialManifest(partialPath, fileName string, sourceSize int64, validation agentBinlogValidationResult, identity agentMySQLServerIdentity) error {
	if partialPath == "" {
		return nil
	}
	manifest := agentPartialManifest{
		FileName:        fileName,
		SourceSize:      sourceSize,
		LastCompletePos: validation.LastCompletePos,
		ChecksumMode:    validation.ChecksumMode,
		ServerUUID:      strings.TrimSpace(identity.ServerUUID),
		ServerID:        strings.TrimSpace(identity.ServerID),
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(agentPartialManifestPath(partialPath), append(data, '\n'), 0o600)
}

func validateAgentPartialManifestForResume(partialPath, fileName string, identity agentMySQLServerIdentity) error {
	manifest, err := readAgentPartialManifest(partialPath)
	if err != nil {
		if os.IsNotExist(err) && strings.TrimSpace(identity.ServerUUID) == "" && strings.TrimSpace(identity.ServerID) == "" {
			return nil
		}
		if os.IsNotExist(err) {
			return errors.New("partial manifest 不存在，不能确认 server identity，回退完整拉取")
		}
		return fmt.Errorf("读取 partial manifest 失败: %w", err)
	}
	if strings.TrimSpace(manifest.FileName) != fileName {
		return fmt.Errorf("partial manifest 文件名漂移: manifest=%s current=%s", manifest.FileName, fileName)
	}
	if manifest.ServerUUID != "" && identity.ServerUUID != "" && manifest.ServerUUID != identity.ServerUUID {
		return fmt.Errorf("partial manifest server_uuid 漂移: manifest=%s current=%s", manifest.ServerUUID, identity.ServerUUID)
	}
	if manifest.ServerID != "" && identity.ServerID != "" && manifest.ServerID != identity.ServerID {
		return fmt.Errorf("partial manifest server_id 漂移: manifest=%s current=%s", manifest.ServerID, identity.ServerID)
	}
	return nil
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

func runAgentMysqlbinlogRawFromPosition(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, credential databaseArchiverCredential, tool, resultDir, fileName string, startPosition int64) error {
	if !isSafeAgentBinlogFileName(fileName) {
		return fmt.Errorf("binlog 文件名不合法: %s", fileName)
	}
	if startPosition <= 4 {
		return runAgentMysqlbinlogRaw(ctx, cfg, item, credential, tool, resultDir, fileName)
	}
	defaultsFile, err := writeAgentMySQLDefaultsFile(cfg.WorkDir, item.SourceInstance, credential)
	if err != nil {
		return err
	}
	defer os.Remove(defaultsFile)
	commandCtx, cancel := context.WithTimeout(ctx, maxDuration(cfg.Interval*2, 60*time.Second))
	defer cancel()
	cmd := exec.CommandContext(
		commandCtx,
		tool,
		"--defaults-extra-file="+defaultsFile,
		"--read-from-remote-server",
		"--raw",
		"--start-position="+strconv.FormatInt(startPosition, 10),
		"--result-file="+ensureTrailingPathSeparator(resultDir),
		fileName,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqlbinlog resume 拉取 %s@%d 失败: %w: %s", fileName, startPosition, err, strings.TrimSpace(stderr.String()))
	}
	outPath := filepath.Join(resultDir, fileName)
	if info, err := os.Stat(outPath); err != nil || info.Size() <= 0 {
		return fmt.Errorf("mysqlbinlog resume 未生成有效文件: %s", fileName)
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

type agentRollingLogWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	maxFiles int
	file     *os.File
	size     int64
}

func newAgentRollingLogWriter(path string, maxBytes int64, maxFiles int) (*agentRollingLogWriter, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("log path is empty")
	}
	if maxBytes <= 0 {
		maxBytes = defaultDatabaseArchiverLogMaxBytes
	}
	if maxFiles <= 0 {
		maxFiles = defaultDatabaseArchiverLogMaxFiles
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	writer := &agentRollingLogWriter{path: path, maxBytes: maxBytes, maxFiles: maxFiles}
	if err := writer.openLocked(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *agentRollingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := 0
	for len(p) > 0 {
		if err := w.openLocked(); err != nil {
			return written, err
		}
		if w.maxBytes > 0 && w.size >= w.maxBytes {
			if err := w.rotateLocked(); err != nil {
				return written, err
			}
			continue
		}
		chunk := p
		if w.maxBytes > 0 {
			remaining := w.maxBytes - w.size
			if remaining <= 0 {
				if err := w.rotateLocked(); err != nil {
					return written, err
				}
				continue
			}
			if int64(len(chunk)) > remaining {
				chunk = chunk[:remaining]
			}
		}
		n, err := w.file.Write(chunk)
		w.size += int64(n)
		written += n
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func (w *agentRollingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.size = 0
	return err
}

func (w *agentRollingLogWriter) openLocked() error {
	if w.file != nil {
		return nil
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	if w.maxBytes > 0 && w.size >= w.maxBytes {
		return w.rotateLocked()
	}
	return nil
}

func (w *agentRollingLogWriter) rotateLocked() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	if w.maxFiles <= 1 {
		_ = os.Remove(w.path)
	} else {
		_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.maxFiles))
		for i := w.maxFiles - 1; i >= 1; i-- {
			oldPath := fmt.Sprintf("%s.%d", w.path, i)
			newPath := fmt.Sprintf("%s.%d", w.path, i+1)
			if _, err := os.Stat(oldPath); err == nil {
				_ = os.Rename(oldPath, newPath)
			}
		}
		if _, err := os.Stat(w.path); err == nil {
			_ = os.Rename(w.path, w.path+".1")
		}
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	w.file = file
	w.size = 0
	return nil
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

func finalizeAgentBinlogArtifact(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, artifact agentBinlogArtifact) (agentBinlogArtifact, error) {
	publishArtifact := artifact
	if cfg.objectStorageEnabled() {
		publishArtifact.StorageURI = agentObjectStorageURI(cfg, item, artifact.FileName)
	}
	if err := writeAgentBinlogSidecars(publishArtifact); err != nil {
		return agentBinlogArtifact{}, err
	}
	if !cfg.objectStorageEnabled() {
		return artifact, nil
	}
	if err := publishAgentBinlogArtifact(ctx, cfg, item, publishArtifact); err != nil {
		if cfg.UploadRetryEnabled {
			if queueErr := enqueueAgentUploadRetry(cfg, item, publishArtifact, err); queueErr != nil {
				return agentBinlogArtifact{}, fmt.Errorf("对象存储上传失败且写入重试队列失败: upload=%v queue=%w", err, queueErr)
			}
			log.Printf("database archiver object upload queued for retry: stream=%d file=%s err=%v", item.Stream.ID, artifact.FileName, err)
			return artifact, nil
		}
		return agentBinlogArtifact{}, err
	}
	return publishArtifact, nil
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

func agentBinlogStreamingSpoolDir(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream) string {
	return filepath.Join(agentBinlogSpoolDir(cfg, item), "streaming")
}

func agentBinlogProcessLogDir(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream) string {
	return filepath.Join(cfg.StorageRoot, "mysql-binlog", fmt.Sprintf("instance-%d", item.Stream.InstanceID), fmt.Sprintf("stream-%d", item.Stream.ID), "logs")
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

type agentUploadQueueItem struct {
	ID               string `json:"id"`
	StreamID         uint   `json:"streamId"`
	InstanceID       uint   `json:"instanceId"`
	SourceInstanceID uint   `json:"sourceInstanceId"`
	RunnerHostID     uint   `json:"runnerHostId"`
	RunnerID         string `json:"runnerId"`
	FileName         string `json:"fileName"`
	LocalPath        string `json:"localPath"`
	ObjectURI        string `json:"objectUri"`
	FileSize         int64  `json:"fileSize"`
	ChecksumSHA256   string `json:"checksumSha256"`
	FirstEventTime   string `json:"firstEventTime"`
	LastEventTime    string `json:"lastEventTime"`
	PreviousFileName string `json:"previousFileName,omitempty"`
	NextFileName     string `json:"nextFileName,omitempty"`
	Attempts         int    `json:"attempts"`
	MaxAttempts      int    `json:"maxAttempts"`
	NextAttemptAt    string `json:"nextAttemptAt"`
	LastError        string `json:"lastError,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func agentUploadQueueDir(cfg *resolvedDatabaseArchiverConfig) string {
	return filepath.Join(cfg.StorageRoot, "mysql-binlog", "upload-queue")
}

func agentUploadQueuePath(cfg *resolvedDatabaseArchiverConfig, id string) string {
	return filepath.Join(agentUploadQueueDir(cfg), sanitizeAgentQueueFileName(id)+".json")
}

func sanitizeAgentQueueFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "upload"
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	return strings.Trim(re.ReplaceAllString(value, "-"), "-")
}

func agentUploadQueueID(item databaseArchiverAssignedStream, artifact agentBinlogArtifact) string {
	sum := artifact.ChecksumSHA256
	if len(sum) > 16 {
		sum = sum[:16]
	}
	return fmt.Sprintf("stream-%d-%s-%s", item.Stream.ID, artifact.FileName, sum)
}

func enqueueAgentUploadRetry(cfg *resolvedDatabaseArchiverConfig, item databaseArchiverAssignedStream, artifact agentBinlogArtifact, cause error) error {
	if cfg == nil || !cfg.objectStorageEnabled() {
		return nil
	}
	if err := os.MkdirAll(agentUploadQueueDir(cfg), 0o755); err != nil {
		return err
	}
	now := time.Now().UTC()
	queueItem := agentUploadQueueItem{
		ID:               agentUploadQueueID(item, artifact),
		StreamID:         item.Stream.ID,
		InstanceID:       item.Stream.InstanceID,
		SourceInstanceID: item.Stream.SourceInstanceID,
		RunnerHostID:     item.Runner.ID,
		RunnerID:         cfg.RunnerID,
		FileName:         artifact.FileName,
		LocalPath:        artifact.Path,
		ObjectURI:        artifact.StorageURI,
		FileSize:         artifact.FileSize,
		ChecksumSHA256:   artifact.ChecksumSHA256,
		FirstEventTime:   artifact.FirstEventTime.Format("2006-01-02 15:04:05"),
		LastEventTime:    artifact.LastEventTime.Format("2006-01-02 15:04:05"),
		PreviousFileName: artifact.Previous,
		NextFileName:     artifact.Next,
		MaxAttempts:      cfg.UploadRetryMaxAttempts,
		CreatedAt:        now.Format(time.RFC3339),
	}
	path := agentUploadQueuePath(cfg, queueItem.ID)
	if existing, err := readAgentUploadQueueItem(path); err == nil {
		queueItem.Attempts = existing.Attempts
		queueItem.CreatedAt = existing.CreatedAt
	}
	queueItem.LastError = agentErrorString(cause)
	queueItem.UpdatedAt = now.Format(time.RFC3339)
	queueItem.NextAttemptAt = now.Add(agentUploadRetryDelay(cfg, queueItem.Attempts)).Format(time.RFC3339)
	return writeAgentUploadQueueItem(path, queueItem)
}

func readAgentUploadQueueItem(path string) (agentUploadQueueItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentUploadQueueItem{}, err
	}
	var item agentUploadQueueItem
	if err := json.Unmarshal(data, &item); err != nil {
		return agentUploadQueueItem{}, err
	}
	return item, nil
}

func writeAgentUploadQueueItem(path string, item agentUploadQueueItem) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func agentUploadRetryDelay(cfg *resolvedDatabaseArchiverConfig, attempts int) time.Duration {
	delay := cfg.UploadRetryBase
	if delay <= 0 {
		delay = time.Duration(defaultDatabaseArchiverUploadRetryBase) * time.Second
	}
	for i := 0; i < attempts; i++ {
		delay *= 2
		if cfg.UploadRetryMax > 0 && delay >= cfg.UploadRetryMax {
			return cfg.UploadRetryMax
		}
	}
	if cfg.UploadRetryMax > 0 && delay > cfg.UploadRetryMax {
		delay = cfg.UploadRetryMax
	}
	return delay
}

func (a *agentApp) processAgentUploadRetryQueue(ctx context.Context, cfg *resolvedDatabaseArchiverConfig) {
	if cfg == nil || !cfg.objectStorageEnabled() || !cfg.UploadRetryEnabled {
		return
	}
	entries, err := os.ReadDir(agentUploadQueueDir(cfg))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("database archiver read upload queue failed: %v", err)
		}
		return
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(agentUploadQueueDir(cfg), entry.Name())
		queueItem, err := readAgentUploadQueueItem(path)
		if err != nil {
			log.Printf("database archiver read upload queue item failed: %s err=%v", path, err)
			continue
		}
		if queueItem.NextAttemptAt != "" {
			nextAttempt, parseErr := time.Parse(time.RFC3339, queueItem.NextAttemptAt)
			if parseErr == nil && nextAttempt.After(now) {
				continue
			}
		}
		if queueItem.MaxAttempts > 0 && queueItem.Attempts >= queueItem.MaxAttempts {
			continue
		}
		if err := a.processAgentUploadQueueItem(ctx, cfg, path, queueItem); err != nil {
			log.Printf("database archiver upload retry failed: stream=%d file=%s err=%v", queueItem.StreamID, queueItem.FileName, err)
		}
	}
}

func (a *agentApp) processAgentUploadQueueItem(ctx context.Context, cfg *resolvedDatabaseArchiverConfig, queuePath string, queueItem agentUploadQueueItem) error {
	item := databaseArchiverAssignedStream{
		Stream: databaseArchiverStream{
			ID:               queueItem.StreamID,
			InstanceID:       queueItem.InstanceID,
			SourceInstanceID: queueItem.SourceInstanceID,
		},
		Runner: databaseArchiverRunnerConfig{ID: queueItem.RunnerHostID},
	}
	firstEvent, _ := time.ParseInLocation("2006-01-02 15:04:05", queueItem.FirstEventTime, time.Local)
	lastEvent, _ := time.ParseInLocation("2006-01-02 15:04:05", queueItem.LastEventTime, time.Local)
	artifact := agentBinlogArtifact{
		FileName:       queueItem.FileName,
		Path:           queueItem.LocalPath,
		StorageURI:     queueItem.ObjectURI,
		FileSize:       queueItem.FileSize,
		ChecksumSHA256: queueItem.ChecksumSHA256,
		FirstEventTime: firstEvent,
		LastEventTime:  lastEvent,
		Previous:       queueItem.PreviousFileName,
		Next:           queueItem.NextFileName,
	}
	if artifact.FirstEventTime.IsZero() {
		artifact.FirstEventTime = time.Now()
	}
	if artifact.LastEventTime.IsZero() {
		artifact.LastEventTime = artifact.FirstEventTime
	}
	if err := validateAgentQueuedArtifact(artifact); err != nil {
		queueItem.Attempts++
		queueItem.LastError = err.Error()
		queueItem.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		queueItem.NextAttemptAt = time.Now().UTC().Add(agentUploadRetryDelay(cfg, queueItem.Attempts)).Format(time.RFC3339)
		_ = writeAgentUploadQueueItem(queuePath, queueItem)
		return err
	}
	if err := publishAgentBinlogArtifact(ctx, cfg, item, artifact); err != nil {
		queueItem.Attempts++
		queueItem.LastError = err.Error()
		queueItem.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		queueItem.NextAttemptAt = time.Now().UTC().Add(agentUploadRetryDelay(cfg, queueItem.Attempts)).Format(time.RFC3339)
		_ = writeAgentUploadQueueItem(queuePath, queueItem)
		return err
	}
	if err := os.Remove(queuePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = a.databaseArchiverPostEvent(ctx, cfg, item, databaseArchiverEventRequest{
		EventType: "agent_message",
		Level:     "info",
		Message:   "对象存储补传成功",
		FileName:  queueItem.FileName,
		PayloadJSON: databaseArchiverEventPayloadJSON(map[string]any{
			"fileName":       queueItem.FileName,
			"storageUri":     queueItem.ObjectURI,
			"attempts":       queueItem.Attempts + 1,
			"checksumSha256": queueItem.ChecksumSHA256,
			"queueId":        queueItem.ID,
		}),
		OccurredAt: time.Now().Format("2006-01-02 15:04:05"),
	})
	return nil
}

func validateAgentQueuedArtifact(artifact agentBinlogArtifact) error {
	info, err := os.Stat(artifact.Path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("queued artifact path is a directory: %s", artifact.Path)
	}
	if artifact.FileSize > 0 && info.Size() != artifact.FileSize {
		return fmt.Errorf("queued artifact size mismatch: got=%d expected=%d", info.Size(), artifact.FileSize)
	}
	if artifact.ChecksumSHA256 != "" {
		checksum, err := sha256File(artifact.Path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(checksum, artifact.ChecksumSHA256) {
			return fmt.Errorf("queued artifact checksum mismatch: got=%s expected=%s", checksum, artifact.ChecksumSHA256)
		}
	}
	return nil
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
	if err := uploadAgentObjectFileLimited(ctx, client, bucket, stagingKey, artifact.Path, metadata, cfg.UploadBandwidthBytesPerSecond); err != nil {
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
		copySource := encodeAgentS3CopySource(bucket, stagingKey)
		if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(finalKey),
			CopySource: aws.String(copySource),
			Metadata:   metadata,
		}); err != nil {
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

func encodeAgentS3CopySource(bucket, key string) string {
	encodedKey := url.PathEscape(key)
	encodedKey = strings.ReplaceAll(encodedKey, "%2F", "/")
	return url.PathEscape(bucket) + "/" + encodedKey
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
	if err := uploadAgentObjectFileLimited(ctx, client, bucket, key, filePath, metadata, 0); err != nil {
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
	return uploadAgentObjectFileLimited(ctx, client, bucket, key, filePath, metadata, 0)
}

func uploadAgentObjectFileLimited(ctx context.Context, client *s3.Client, bucket, key, filePath string, metadata map[string]string, bytesPerSecond int64) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	var body io.Reader = file
	if bytesPerSecond > 0 {
		body = &agentRateLimitedReader{reader: file, bytesPerSecond: bytesPerSecond}
	}
	uploader := manager.NewUploader(client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(info.Size()),
		Metadata:      metadata,
	})
	return err
}

type agentRateLimitedReader struct {
	reader         io.Reader
	bytesPerSecond int64
}

func (r *agentRateLimitedReader) Read(p []byte) (int, error) {
	if r == nil || r.reader == nil {
		return 0, io.EOF
	}
	if r.bytesPerSecond <= 0 {
		return r.reader.Read(p)
	}
	maxChunk := r.bytesPerSecond / 10
	if maxChunk < 1 {
		maxChunk = 1
	}
	if int64(len(p)) > maxChunk {
		p = p[:maxChunk]
	}
	started := time.Now()
	n, err := r.reader.Read(p)
	if n > 0 {
		expected := time.Duration(int64(n) * int64(time.Second) / r.bytesPerSecond)
		if elapsed := time.Since(started); expected > elapsed {
			time.Sleep(expected - elapsed)
		}
	}
	return n, err
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

func agentErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
