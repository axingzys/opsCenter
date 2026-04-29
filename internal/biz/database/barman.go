package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/database/pgwal"
	"gorm.io/gorm"
)

const (
	defaultBarmanCatalogMaxBackups = 200
)

var (
	barmanServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)
	barmanBackupIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_.:+-]+$`)
)

type DatabaseBarmanServerListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	Keyword            string `form:"keyword"`
	SourceInstanceID   uint   `form:"sourceInstanceId"`
	RunnerHostID       uint   `form:"runnerHostId"`
	Status             string `form:"status"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseBarmanServerRequest struct {
	SourceInstanceID         uint   `json:"sourceInstanceId" binding:"required"`
	RunnerHostID             uint   `json:"runnerHostId" binding:"required"`
	Name                     string `json:"name" binding:"required,max=120"`
	BarmanServerName         string `json:"barmanServerName" binding:"required,max=120"`
	BarmanHome               string `json:"barmanHome" binding:"omitempty,max=500"`
	ConfigPath               string `json:"configPath" binding:"omitempty,max=500"`
	RetentionPolicy          string `json:"retentionPolicy" binding:"omitempty,max=255"`
	BackupMethod             string `json:"backupMethod" binding:"omitempty,max=60"`
	StreamingArchiverEnabled bool   `json:"streamingArchiverEnabled"`
	ArchiverEnabled          bool   `json:"archiverEnabled"`
	SlotName                 string `json:"slotName" binding:"omitempty,max=120"`
	Status                   string `json:"status" binding:"omitempty,max=30"`
	ConfigJSON               string `json:"configJson" binding:"omitempty,max=4000"`
}

type DatabaseBarmanServerVO struct {
	ID                       uint   `json:"id"`
	SourceInstanceID         uint   `json:"sourceInstanceId"`
	SourceInstanceName       string `json:"sourceInstanceName"`
	RunnerHostID             uint   `json:"runnerHostId"`
	RunnerHostName           string `json:"runnerHostName"`
	Name                     string `json:"name"`
	BarmanServerName         string `json:"barmanServerName"`
	BarmanHome               string `json:"barmanHome"`
	ConfigPath               string `json:"configPath"`
	RetentionPolicy          string `json:"retentionPolicy"`
	BackupMethod             string `json:"backupMethod"`
	StreamingArchiverEnabled bool   `json:"streamingArchiverEnabled"`
	ArchiverEnabled          bool   `json:"archiverEnabled"`
	SlotName                 string `json:"slotName"`
	BarmanVersion            string `json:"barmanVersion"`
	PGVersion                string `json:"pgVersion"`
	PGSystemIdentifier       string `json:"pgSystemIdentifier"`
	WALSegmentSize           int64  `json:"walSegmentSize"`
	Status                   string `json:"status"`
	StatusText               string `json:"statusText"`
	LastCheckAt              string `json:"lastCheckAt"`
	LastCheckStatus          string `json:"lastCheckStatus"`
	LastCheckStatusText      string `json:"lastCheckStatusText"`
	LastCatalogSyncAt        string `json:"lastCatalogSyncAt"`
	LastWALSyncAt            string `json:"lastWalSyncAt"`
	LastError                string `json:"lastError"`
	ConfigJSON               string `json:"configJson"`
	CreatedAt                string `json:"createdAt"`
	UpdatedAt                string `json:"updatedAt"`
}

type barmanCheckRunnerResult struct {
	RunnerHostID     uint   `json:"runnerHostId"`
	RunnerID         string `json:"runnerId"`
	BarmanServerID   uint   `json:"barmanServerId"`
	BarmanServerName string `json:"barmanServerName"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitCode         int    `json:"exitCode"`
	StartedAt        string `json:"startedAt"`
	FinishedAt       string `json:"finishedAt"`
	DurationMs       int64  `json:"durationMs"`
	Version          string `json:"version"`
	CheckExitCode    int    `json:"checkExitCode"`
	StatusExitCode   int    `json:"statusExitCode"`
	CheckJSON        string `json:"checkJson"`
	CheckOutput      string `json:"checkOutput"`
	CheckError       string `json:"checkError"`
	StatusJSON       string `json:"statusJson"`
	StatusError      string `json:"statusError"`
	DerivedStatus    string `json:"derivedStatus"`
}

type barmanCatalogSyncRunnerResult struct {
	RunnerHostID     uint                            `json:"runnerHostId"`
	RunnerID         string                          `json:"runnerId"`
	BarmanServerID   uint                            `json:"barmanServerId"`
	BarmanServerName string                          `json:"barmanServerName"`
	Stdout           string                          `json:"stdout"`
	Stderr           string                          `json:"stderr"`
	ExitCode         int                             `json:"exitCode"`
	StartedAt        string                          `json:"startedAt"`
	FinishedAt       string                          `json:"finishedAt"`
	DurationMs       int64                           `json:"durationMs"`
	ListExitCode     int                             `json:"listExitCode"`
	ListJSON         string                          `json:"listJson"`
	ListOutput       string                          `json:"listOutput"`
	ListError        string                          `json:"listError"`
	BackupIDs        []string                        `json:"backupIds"`
	SyncedRecords    int                             `json:"syncedRecords"`
	CreatedRecords   int                             `json:"createdRecords"`
	UpdatedRecords   int                             `json:"updatedRecords"`
	FailedRecords    int                             `json:"failedRecords"`
	RecordResults    []barmanCatalogSyncRecordResult `json:"recordResults"`
}

type barmanCatalogSyncRecordResult struct {
	BackupID uintOrString `json:"backupId"`
	Status   string       `json:"status"`
	Action   string       `json:"action"`
	Message  string       `json:"message"`
}

type uintOrString string

type barmanBackupRunnerResult struct {
	RunnerHostID     uint   `json:"runnerHostId"`
	RunnerID         string `json:"runnerId"`
	BarmanServerID   uint   `json:"barmanServerId"`
	BarmanServerName string `json:"barmanServerName"`
	BackupRecordID   uint   `json:"backupRecordId,omitempty"`
	BackupTaskID     uint   `json:"backupTaskId,omitempty"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitCode         int    `json:"exitCode"`
	StartedAt        string `json:"startedAt"`
	FinishedAt       string `json:"finishedAt"`
	DurationMs       int64  `json:"durationMs"`
	BackupExitCode   int    `json:"backupExitCode"`
	CheckExitCode    int    `json:"checkExitCode"`
	ListExitCode     int    `json:"listExitCode"`
	BackupID         string `json:"backupId"`
	BackupOutput     string `json:"backupOutput"`
	BackupError      string `json:"backupError"`
	CheckOutput      string `json:"checkOutput"`
	CheckError       string `json:"checkError"`
	ListJSON         string `json:"listJson"`
	ListOutput       string `json:"listOutput"`
	ListError        string `json:"listError"`
	ShowJSON         string `json:"showJson"`
	ShowError        string `json:"showError"`
	SyncedRecord     bool   `json:"syncedRecord"`
}

type barmanWALSyncRunnerResult struct {
	RunnerHostID          uint                        `json:"runnerHostId"`
	RunnerID              string                      `json:"runnerId"`
	BarmanServerID        uint                        `json:"barmanServerId"`
	BarmanServerName      string                      `json:"barmanServerName"`
	StreamID              uint                        `json:"streamId"`
	Stdout                string                      `json:"stdout"`
	Stderr                string                      `json:"stderr"`
	ExitCode              int                         `json:"exitCode"`
	StartedAt             string                      `json:"startedAt"`
	FinishedAt            string                      `json:"finishedAt"`
	DurationMs            int64                       `json:"durationMs"`
	ListExitCode          int                         `json:"listExitCode"`
	ListJSON              string                      `json:"listJson"`
	ListOutput            string                      `json:"listOutput"`
	ListError             string                      `json:"listError"`
	BackupIDs             []string                    `json:"backupIds"`
	SyncedArchives        int                         `json:"syncedArchives"`
	CreatedArchives       int                         `json:"createdArchives"`
	UpdatedArchives       int                         `json:"updatedArchives"`
	FailedArchives        int                         `json:"failedArchives"`
	LogChainStatus        string                      `json:"logChainStatus"`
	TimelineHistoryStatus string                      `json:"timelineHistoryStatus"`
	LastArchiveName       string                      `json:"lastArchiveName"`
	ArchiveResults        []barmanWALSyncRecordResult `json:"archiveResults"`
}

type barmanWALSyncRecordResult struct {
	FileName string `json:"fileName"`
	Status   string `json:"status"`
	Action   string `json:"action"`
	Message  string `json:"message"`
}

type barmanBackupShowOutput struct {
	BackupID string
	ShowJSON string
	ShowErr  string
	ExitCode int
}

type barmanListFilesOutput struct {
	BackupID string
	Output   string
	Error    string
	ExitCode int
}

type barmanBackupCatalogRecord struct {
	BackupID           string
	Status             string
	BackupLevel        string
	FileSize           int64
	ChecksumSHA256     string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	RecoverableFrom    *time.Time
	RecoverableUntil   *time.Time
	PGSystemIdentifier string
	TimelineID         string
	WALSegmentSize     int64
	StartLSN           string
	EndLSN             string
	WALStart           string
	WALEnd             string
	ManifestJSON       string
}

type barmanWALCatalogRecord struct {
	FileName           string
	StorageURI         string
	FileSize           int64
	ChecksumSHA256     string
	FirstEventTime     *time.Time
	LastEventTime      *time.Time
	Status             string
	ArchivedAt         *time.Time
	PGSystemIdentifier string
	TimelineID         string
	WALSegmentSize     int64
	ExternalServerName string
	StartLSN           string
	EndLSN             string
	SegmentNo          string
	TimelineHistoryURI string
}

func (uc *UseCase) ListBarmanServers(ctx context.Context, req *DatabaseBarmanServerListRequest) ([]*DatabaseBarmanServerVO, int64, error) {
	if uc.barmanServerRepo == nil {
		return nil, 0, fmt.Errorf("Barman Server 仓库未配置")
	}
	normalizeBarmanServerListRequest(req)
	items, total, err := uc.barmanServerRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseBarmanServerVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toBarmanServerVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) CreateBarmanServer(ctx context.Context, req *DatabaseBarmanServerRequest) (*DatabaseBarmanServerVO, error) {
	if uc.barmanServerRepo == nil {
		return nil, fmt.Errorf("Barman Server 仓库未配置")
	}
	item, err := uc.buildBarmanServerFromRequest(ctx, nil, req)
	if err != nil {
		return nil, err
	}
	if err := uc.barmanServerRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBarmanServerVO(ctx, item), nil
}

func (uc *UseCase) UpdateBarmanServer(ctx context.Context, id uint, req *DatabaseBarmanServerRequest) (*DatabaseBarmanServerVO, error) {
	if uc.barmanServerRepo == nil {
		return nil, fmt.Errorf("Barman Server 仓库未配置")
	}
	if id == 0 {
		return nil, fmt.Errorf("Barman Server ID不能为空")
	}
	item, err := uc.barmanServerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Barman Server 不存在")
	}
	item, err = uc.buildBarmanServerFromRequest(ctx, item, req)
	if err != nil {
		return nil, err
	}
	if err := uc.barmanServerRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return uc.toBarmanServerVO(ctx, item), nil
}

func (uc *UseCase) DeleteBarmanServer(ctx context.Context, id uint) error {
	if uc.barmanServerRepo == nil {
		return fmt.Errorf("Barman Server 仓库未配置")
	}
	if id == 0 {
		return fmt.Errorf("Barman Server ID不能为空")
	}
	if _, err := uc.barmanServerRepo.GetByID(ctx, id); err != nil {
		return fmt.Errorf("Barman Server 不存在")
	}
	return uc.barmanServerRepo.Delete(ctx, id)
}

func (uc *UseCase) GetBarmanServerSourceInstanceID(ctx context.Context, id uint) (uint, error) {
	if uc.barmanServerRepo == nil {
		return 0, fmt.Errorf("Barman Server 仓库未配置")
	}
	if id == 0 {
		return 0, fmt.Errorf("Barman Server ID不能为空")
	}
	item, err := uc.barmanServerRepo.GetByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("Barman Server 不存在")
	}
	return item.SourceInstanceID, nil
}

func (uc *UseCase) CheckBarmanServer(ctx context.Context, id uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("Barman Runner 仓库未配置")
	}
	server, err := uc.barmanServerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Barman Server 不存在")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBarmanCheck,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: server.SourceInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBarmanCheck,
		CommandSummary:   fmt.Sprintf("Barman check %s", server.BarmanServerName),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      barmanRunnerRequestJSON(server, host, operator, DatabaseRunnerAllowedCommandBarmanCheck),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeBarmanCheckJob(context.Background(), server.ID, job.ID)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) SyncBarmanCatalog(ctx context.Context, id uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("Barman catalog 同步仓库未配置")
	}
	server, err := uc.barmanServerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Barman Server 不存在")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBarmanCatalogSync,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: server.SourceInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBarmanCatalogSync,
		CommandSummary:   fmt.Sprintf("Barman catalog sync %s", server.BarmanServerName),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      barmanRunnerRequestJSON(server, host, operator, DatabaseRunnerAllowedCommandBarmanCatalogSync),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeBarmanCatalogSyncJob(context.Background(), server.ID, job.ID)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) SyncBarmanWAL(ctx context.Context, id uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.logArchiveStreamRepo == nil || uc.logArchiveRepo == nil {
		return nil, fmt.Errorf("Barman WAL 同步仓库未配置")
	}
	server, err := uc.barmanServerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Barman Server 不存在")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
	}
	stream, err := uc.ensureBarmanWALArchiveStream(ctx, server)
	if err != nil {
		return nil, err
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBarmanWALSync,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: server.SourceInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBarmanWALSync,
		CommandSummary:   fmt.Sprintf("Barman WAL sync %s", server.BarmanServerName),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      barmanRunnerRequestJSON(server, host, operator, DatabaseRunnerAllowedCommandBarmanWALSync),
	}
	if stream != nil {
		job.RequestJSON = barmanRunnerRequestJSONWithExtra(server, host, operator, DatabaseRunnerAllowedCommandBarmanWALSync, map[string]any{"streamId": stream.ID})
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeBarmanWALSyncJob(context.Background(), server.ID, job.ID)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) BackupBarmanServer(ctx context.Context, id uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	job, _, err := uc.enqueueBarmanBackup(ctx, id, nil, operator, DatabaseBackupTriggerManual)
	if err != nil {
		return nil, err
	}
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) enqueueBarmanBackup(ctx context.Context, serverID uint, task *DatabaseBackupTask, operator QueryOperator, triggerType string) (*DatabaseRunnerJob, *DatabaseBackupRecord, error) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil {
		return nil, nil, fmt.Errorf("Barman backup 仓库未配置")
	}
	server, err := uc.barmanServerRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("Barman Server 不存在")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil {
		return nil, nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, nil, err
	}
	instance, err := uc.instanceRepo.GetByID(ctx, server.SourceInstanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("PostgreSQL 实例不存在")
	}
	if normalizeDBType(instance.DBType) != DBTypePostgreSQL {
		return nil, nil, fmt.Errorf("Barman backup 仅支持 PostgreSQL 实例")
	}
	audit, err := uc.startBackupAudit(ctx, instance, task, "cluster", operator)
	if err != nil {
		return nil, nil, fmt.Errorf("创建备份审计失败: %w", err)
	}
	releaseOnError := false
	if task != nil {
		if err := uc.acquireBackupTaskRun(task.ID, task.InstanceID); err != nil {
			uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, err.Error())
			return nil, nil, err
		}
		releaseOnError = true
		defer func() {
			if releaseOnError {
				uc.releaseBackupTaskRun(task.ID, task.InstanceID)
			}
		}()
	}
	started := time.Now()
	record := &DatabaseBackupRecord{
		TaskID:             taskID(task),
		InstanceID:         server.SourceInstanceID,
		TriggerType:        normalizeBackupTriggerType(triggerType),
		BackupType:         DatabaseBackupTypePhysical,
		ChainID:            trimText(fmt.Sprintf("barman-%d", server.ID), 64),
		BackupMethod:       DatabaseBackupMethodPhysical,
		BackupLevel:        DatabaseBackupLevelFull,
		BackupEngine:       "barman",
		ExternalServerName: trimText(server.BarmanServerName, 120),
		BackupScope:        "cluster",
		ToolName:           "barman",
		ToolVersion:        trimText(server.BarmanVersion, 120),
		SourceInstanceID:   server.SourceInstanceID,
		SourceRole:         "external",
		StorageType:        DatabaseBackupStorageExternal,
		StorageURI:         trimText(fmt.Sprintf("barman://%s/pending/%d", server.BarmanServerName, started.Unix()), 1000),
		Status:             DatabaseBackupStatusQueued,
		FileName:           trimText(fmt.Sprintf("%s-pending-%s", server.BarmanServerName, started.Format("20060102150405")), 255),
		Compression:        "external",
		VerifyStatus:       DatabaseBackupVerifyStatusPending,
		StartedAt:          &started,
		LastHeartbeatAt:    &started,
		RecoverableFrom:    &started,
		RecoverableUntil:   &started,
		ErrorMessage:       "Barman 物理备份任务已进入 Runner 队列",
		PGSystemIdentifier: trimText(server.PGSystemIdentifier, 120),
		WALSegmentSize:     server.WALSegmentSize,
	}
	if err := uc.backupRecordRepo.Create(ctx, record); err != nil {
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, "创建备份记录失败: "+err.Error())
		return nil, nil, err
	}
	if task != nil && uc.backupTaskRepo != nil {
		task.LastRunAt = &started
		task.LastStatus = DatabaseBackupStatusQueued
		task.LastMessage = trimText("Barman 物理备份任务已进入 Runner 队列", 500)
		task.RestoreCapability = DatabaseRestoreCapabilityPhysicalRestore
		uc.applyBackupTaskNextRunAt(task, started)
		if err := uc.backupTaskRepo.Update(ctx, task); err != nil {
			record.Status = DatabaseBackupStatusFailed
			record.ErrorMessage = trimText("更新备份任务状态失败: "+err.Error(), 500)
			_ = uc.backupRecordRepo.Update(ctx, record)
			uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
			return nil, nil, err
		}
	}
	requestExtra := map[string]any{"backupRecordId": record.ID}
	if task != nil {
		requestExtra["backupTaskId"] = task.ID
	}
	job := &DatabaseRunnerJob{
		JobType:          DatabaseRunnerJobTypeBarmanBackup,
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		SourceInstanceID: server.SourceInstanceID,
		Status:           DatabaseRunnerJobStatusQueued,
		AllowedCommand:   DatabaseRunnerAllowedCommandBarmanBackup,
		CommandSummary:   fmt.Sprintf("Barman backup %s", server.BarmanServerName),
		WorkDir:          host.WorkDir,
		OperatorID:       operator.ID,
		OperatorName:     trimText(operator.Username, 120),
		RequestJSON:      barmanRunnerRequestJSONWithExtra(server, host, operator, DatabaseRunnerAllowedCommandBarmanBackup, requestExtra),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		record.Status = DatabaseBackupStatusFailed
		record.ErrorMessage = trimText("创建 Barman Runner Job 失败: "+err.Error(), 500)
		_ = uc.backupRecordRepo.Update(ctx, record)
		if task != nil {
			uc.finishBackupTask(ctx, task, time.Now(), DatabaseBackupStatusFailed, record.ErrorMessage)
		}
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, 0, record.ErrorMessage)
		return nil, nil, err
	}
	releaseOnError = false
	go func() {
		defer func() {
			if task != nil {
				uc.releaseBackupTaskRun(task.ID, task.InstanceID)
			}
		}()
		uc.executeBarmanBackupJob(context.Background(), server.ID, job.ID, record.ID, taskID(task), audit)
	}()
	return job, record, nil
}

func taskID(task *DatabaseBackupTask) uint {
	if task == nil {
		return 0
	}
	return task.ID
}

func (uc *UseCase) runBarmanBackupTaskIfNeeded(ctx context.Context, id uint, operator QueryOperator, triggerType string) (*DatabaseBackupRunVO, bool, error) {
	if uc.backupTaskRepo == nil || uc.instanceRepo == nil {
		return nil, false, nil
	}
	task, err := uc.backupTaskRepo.GetByID(ctx, id)
	if err != nil || task == nil {
		return nil, false, nil
	}
	instance, err := uc.instanceRepo.GetByID(ctx, task.InstanceID)
	if err != nil || instance == nil || !isPostgreSQLBarmanBackupTask(task, instance) {
		return nil, false, nil
	}
	serverID, err := barmanServerIDFromBackupTask(task)
	if err != nil {
		return nil, true, err
	}
	job, record, err := uc.enqueueBarmanBackup(ctx, serverID, task, operator, triggerType)
	if err != nil {
		return nil, true, err
	}
	message := "Barman 物理备份任务已进入 Runner 队列"
	if job != nil && job.ID > 0 {
		message = fmt.Sprintf("%s，Runner Job #%d", message, job.ID)
	}
	return &DatabaseBackupRunVO{
		TaskID:       task.ID,
		TaskName:     task.Name,
		RecordID:     record.ID,
		InstanceID:   task.InstanceID,
		InstanceName: instance.Name,
		Status:       DatabaseBackupStatusQueued,
		StatusText:   BackupStatusText(DatabaseBackupStatusQueued),
		FileName:     record.FileName,
		Message:      message,
		TriggeredAt:  time.Now().Format("2006-01-02 15:04:05"),
	}, true, nil
}

func isPostgreSQLBarmanBackupTask(task *DatabaseBackupTask, instance *DatabaseInstance) bool {
	if task == nil || instance == nil {
		return false
	}
	return normalizeDBType(instance.DBType) == DBTypePostgreSQL &&
		normalizeBackupMethod(task.BackupMethod) == DatabaseBackupMethodPhysical &&
		strings.TrimSpace(task.BackupEngine) == "barman"
}

func barmanServerIDFromBackupTask(task *DatabaseBackupTask) (uint, error) {
	if task == nil {
		return 0, fmt.Errorf("备份任务不能为空")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(task.ScopeConfig)), &payload); err != nil {
		return 0, fmt.Errorf("PostgreSQL Barman 物理备份需要在范围配置中提供 {\"barmanServerId\": 1}")
	}
	switch value := payload["barmanServerId"].(type) {
	case float64:
		if value > 0 {
			return uint(value), nil
		}
	case string:
		parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if parsed > 0 {
			return uint(parsed), nil
		}
	}
	return 0, fmt.Errorf("PostgreSQL Barman 物理备份需要有效的 barmanServerId")
}

func (uc *UseCase) buildBarmanServerFromRequest(ctx context.Context, item *DatabaseBarmanServer, req *DatabaseBarmanServerRequest) (*DatabaseBarmanServer, error) {
	if req == nil {
		return nil, fmt.Errorf("Barman Server 参数不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("Barman Server 名称不能为空")
	}
	if req.SourceInstanceID == 0 {
		return nil, fmt.Errorf("请选择 PostgreSQL 实例")
	}
	if req.RunnerHostID == 0 {
		return nil, fmt.Errorf("请选择 Runner 主机")
	}
	barmanName := strings.TrimSpace(req.BarmanServerName)
	if barmanName == "" {
		return nil, fmt.Errorf("Barman server name 不能为空")
	}
	if !barmanServerNamePattern.MatchString(barmanName) {
		return nil, fmt.Errorf("Barman server name 只能包含字母、数字、下划线、点、冒号和短横线")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.SourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQL 实例不存在")
	}
	if normalizeDBType(instance.DBType) != DBTypePostgreSQL {
		return nil, fmt.Errorf("P3 第一版 Barman 只支持 PostgreSQL 实例")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return nil, err
	}
	if err := validateBackupStorageConfigSafe(req.ConfigJSON); err != nil {
		return nil, err
	}
	if item == nil {
		item = &DatabaseBarmanServer{
			Status: DatabaseBarmanServerStatusPending,
		}
	}
	item.SourceInstanceID = req.SourceInstanceID
	item.RunnerHostID = req.RunnerHostID
	item.Name = trimText(strings.TrimSpace(req.Name), 120)
	item.BarmanServerName = trimText(barmanName, 120)
	item.BarmanHome = trimText(strings.TrimSpace(req.BarmanHome), 500)
	item.ConfigPath = trimText(strings.TrimSpace(req.ConfigPath), 500)
	item.RetentionPolicy = trimText(strings.TrimSpace(req.RetentionPolicy), 255)
	item.BackupMethod = trimText(strings.TrimSpace(req.BackupMethod), 60)
	item.StreamingArchiverEnabled = req.StreamingArchiverEnabled
	item.ArchiverEnabled = req.ArchiverEnabled
	item.SlotName = trimText(strings.TrimSpace(req.SlotName), 120)
	item.ConfigJSON = trimText(strings.TrimSpace(req.ConfigJSON), maxRunnerJSONLength)
	if status := normalizeBarmanServerStatus(req.Status); status != "" {
		item.Status = status
	} else if strings.TrimSpace(item.Status) == "" {
		item.Status = DatabaseBarmanServerStatusPending
	}
	if item.Status == DatabaseBarmanServerStatusDisabled {
		item.LastError = ""
	}
	return item, nil
}

func validateBarmanRunnerHost(host *DatabaseRunnerHost) error {
	if host == nil {
		return fmt.Errorf("Runner 主机不能为空")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return fmt.Errorf("Runner 主机已禁用")
	}
	if normalizeRunnerType(host.RunnerType) != DatabaseRunnerTypeSSH {
		return fmt.Errorf("P3.1 第一版 Barman 仅支持 SSH Runner")
	}
	if host.CredentialID == 0 {
		return fmt.Errorf("SSH Runner 必须配置连接凭据")
	}
	return nil
}

func (uc *UseCase) executeBarmanCheckJob(ctx context.Context, serverID, jobID uint) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return
	}
	server, serverErr := uc.barmanServerRepo.GetByID(ctx, serverID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if serverErr != nil || jobErr != nil || server == nil || job == nil {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)

	stdout, stderr, exitCode, runErr := uc.runBarmanCheckCommand(ctx, server, host)
	finished := time.Now()
	result := parseBarmanCheckRunnerResult(server, host, stdout, stderr, exitCode, runErr, started, finished)
	uc.applyBarmanCheckResult(ctx, server, job, result, runErr)
}

func (uc *UseCase) runBarmanCheckCommand(ctx context.Context, server *DatabaseBarmanServer, host *DatabaseRunnerHost) (string, string, int, error) {
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return "", "", 1, err
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script := buildBarmanCheckScript(server)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func (uc *UseCase) applyBarmanCheckResult(ctx context.Context, server *DatabaseBarmanServer, job *DatabaseRunnerJob, result barmanCheckRunnerResult, runErr error) {
	if server == nil || job == nil {
		return
	}
	now := time.Now()
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	server.LastCheckAt = &now
	server.LastCheckStatus = result.DerivedStatus
	server.BarmanVersion = trimText(result.Version, 120)
	if result.StatusJSON != "" {
		metadata := parseBarmanServerMetadata(result.StatusJSON, server.BarmanServerName)
		if metadata.RetentionPolicy != "" {
			server.RetentionPolicy = trimText(metadata.RetentionPolicy, 255)
		}
		if metadata.BackupMethod != "" {
			server.BackupMethod = trimText(metadata.BackupMethod, 60)
		}
		if metadata.SlotName != "" {
			server.SlotName = trimText(metadata.SlotName, 120)
		}
		if metadata.PGVersion != "" {
			server.PGVersion = trimText(metadata.PGVersion, 120)
		}
		if metadata.PGSystemIdentifier != "" {
			server.PGSystemIdentifier = trimText(metadata.PGSystemIdentifier, 120)
		}
		if metadata.WALSegmentSize > 0 {
			server.WALSegmentSize = metadata.WALSegmentSize
		}
		if metadata.StreamingArchiverKnown {
			server.StreamingArchiverEnabled = metadata.StreamingArchiverEnabled
		}
		if metadata.ArchiverKnown {
			server.ArchiverEnabled = metadata.ArchiverEnabled
		}
	}
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.CheckExitCode != 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		message := strings.TrimSpace(result.CheckError)
		if message == "" {
			message = strings.TrimSpace(result.CheckOutput)
		}
		if message == "" {
			message = fmt.Sprintf("barman check 退出码 %d", result.CheckExitCode)
		}
		job.ErrorMessage = trimText(message, 1000)
		server.Status = DatabaseBarmanServerStatusDegraded
		server.LastError = job.ErrorMessage
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		server.Status = DatabaseBarmanServerStatusHealthy
		server.LastError = ""
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.barmanServerRepo.Update(ctx, server)
}

func (uc *UseCase) executeBarmanCatalogSyncJob(ctx context.Context, serverID, jobID uint) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil {
		return
	}
	server, serverErr := uc.barmanServerRepo.GetByID(ctx, serverID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if serverErr != nil || jobErr != nil || server == nil || job == nil {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)

	stdout, stderr, exitCode, runErr := uc.runBarmanCatalogSyncCommand(ctx, server, host)
	finished := time.Now()
	result := parseBarmanCatalogSyncRunnerResult(server, host, stdout, stderr, exitCode, runErr, started, finished)
	uc.applyBarmanCatalogSyncResult(ctx, server, job, result, runErr)
}

func (uc *UseCase) runBarmanCatalogSyncCommand(ctx context.Context, server *DatabaseBarmanServer, host *DatabaseRunnerHost) (string, string, int, error) {
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return "", "", 1, err
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script := buildBarmanCatalogSyncScript(server, defaultBarmanCatalogMaxBackups)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func (uc *UseCase) applyBarmanCatalogSyncResult(ctx context.Context, server *DatabaseBarmanServer, job *DatabaseRunnerJob, result barmanCatalogSyncRunnerResult, runErr error) {
	if server == nil || job == nil {
		return
	}
	now := time.Now()
	result.RecordResults = make([]barmanCatalogSyncRecordResult, 0, len(result.BackupIDs))
	for _, show := range parseBarmanShowOutputs(result.Stdout) {
		if show.ExitCode != 0 {
			result.FailedRecords++
			result.RecordResults = append(result.RecordResults, barmanCatalogSyncRecordResult{
				BackupID: uintOrString(show.BackupID),
				Status:   DatabaseRunnerJobStatusFailed,
				Action:   "skip",
				Message:  trimText(show.ShowErr, 500),
			})
			continue
		}
		catalog := parseBarmanBackupCatalog(show.BackupID, server.BarmanServerName, show.ShowJSON)
		created, err := uc.upsertBarmanBackupRecord(ctx, server, catalog)
		if err != nil {
			result.FailedRecords++
			result.RecordResults = append(result.RecordResults, barmanCatalogSyncRecordResult{
				BackupID: uintOrString(show.BackupID),
				Status:   DatabaseRunnerJobStatusFailed,
				Action:   "upsert_failed",
				Message:  trimText(err.Error(), 500),
			})
			continue
		}
		result.SyncedRecords++
		action := "updated"
		if created {
			action = "created"
			result.CreatedRecords++
		} else {
			result.UpdatedRecords++
		}
		result.RecordResults = append(result.RecordResults, barmanCatalogSyncRecordResult{
			BackupID: uintOrString(show.BackupID),
			Status:   DatabaseRunnerJobStatusSuccess,
			Action:   action,
			Message:  "已同步到备份记录",
		})
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	server.LastCatalogSyncAt = &now
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.ListExitCode != 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		message := strings.TrimSpace(result.ListError)
		if message == "" {
			message = strings.TrimSpace(result.ListOutput)
		}
		if message == "" {
			message = fmt.Sprintf("barman list-backup 退出码 %d", result.ListExitCode)
		}
		job.ErrorMessage = trimText(message, 1000)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.FailedRecords > 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(fmt.Sprintf("Barman catalog 部分同步失败：成功 %d，失败 %d", result.SyncedRecords, result.FailedRecords), 1000)
		server.Status = DatabaseBarmanServerStatusDegraded
		server.LastError = job.ErrorMessage
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		server.Status = DatabaseBarmanServerStatusHealthy
		server.LastError = ""
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.barmanServerRepo.Update(ctx, server)
}

func (uc *UseCase) executeBarmanWALSyncJob(ctx context.Context, serverID, jobID uint) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.logArchiveStreamRepo == nil || uc.logArchiveRepo == nil {
		return
	}
	server, serverErr := uc.barmanServerRepo.GetByID(ctx, serverID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if serverErr != nil || jobErr != nil || server == nil || job == nil {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)

	stdout, stderr, exitCode, runErr := uc.runBarmanWALSyncCommand(ctx, server, host)
	finished := time.Now()
	result := parseBarmanWALSyncRunnerResult(server, host, stdout, stderr, exitCode, runErr, started, finished)
	uc.applyBarmanWALSyncResult(ctx, server, job, result, runErr)
}

func (uc *UseCase) runBarmanWALSyncCommand(ctx context.Context, server *DatabaseBarmanServer, host *DatabaseRunnerHost) (string, string, int, error) {
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return "", "", 1, err
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script := buildBarmanWALSyncScript(server, defaultBarmanCatalogMaxBackups)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func (uc *UseCase) applyBarmanWALSyncResult(ctx context.Context, server *DatabaseBarmanServer, job *DatabaseRunnerJob, result barmanWALSyncRunnerResult, runErr error) {
	if server == nil || job == nil {
		return
	}
	now := time.Now()
	stream, streamErr := uc.ensureBarmanWALArchiveStream(ctx, server)
	if streamErr == nil && stream != nil {
		result.StreamID = stream.ID
	}
	result.ArchiveResults = make([]barmanWALSyncRecordResult, 0)
	walRecords := collectBarmanWALCatalogRecords(server, result.Stdout)
	if streamErr == nil && stream != nil {
		for _, record := range walRecords {
			created, err := uc.upsertBarmanWALArchive(ctx, stream, server, record)
			if err != nil {
				result.FailedArchives++
				result.ArchiveResults = append(result.ArchiveResults, barmanWALSyncRecordResult{
					FileName: record.FileName,
					Status:   DatabaseRunnerJobStatusFailed,
					Action:   "upsert_failed",
					Message:  trimText(err.Error(), 500),
				})
				continue
			}
			result.SyncedArchives++
			action := "updated"
			if created {
				action = "created"
				result.CreatedArchives++
			} else {
				result.UpdatedArchives++
			}
			result.LastArchiveName = record.FileName
			result.ArchiveResults = append(result.ArchiveResults, barmanWALSyncRecordResult{
				FileName: record.FileName,
				Status:   DatabaseRunnerJobStatusSuccess,
				Action:   action,
				Message:  "已同步到 WAL 归档记录",
			})
		}
		result.LogChainStatus, result.TimelineHistoryStatus = classifyBarmanWALCatalog(walRecords)
		updateBarmanWALStreamAfterSync(ctx, uc, stream, result, now)
	} else if streamErr != nil {
		result.FailedArchives++
		result.LogChainStatus = DatabaseLogChainStatusUnsupported
		result.ArchiveResults = append(result.ArchiveResults, barmanWALSyncRecordResult{
			Status:  DatabaseRunnerJobStatusFailed,
			Action:  "stream_failed",
			Message: trimText(streamErr.Error(), 500),
		})
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	server.LastWALSyncAt = &now
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.ListExitCode != 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		message := strings.TrimSpace(result.ListError)
		if message == "" {
			message = strings.TrimSpace(result.ListOutput)
		}
		if message == "" {
			message = fmt.Sprintf("barman list-backup 退出码 %d", result.ListExitCode)
		}
		job.ErrorMessage = trimText(message, 1000)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.FailedArchives > 0 || result.LogChainStatus == DatabaseLogChainStatusMissingWAL || result.TimelineHistoryStatus == DatabaseLogChainStatusTimelineGap {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(fmt.Sprintf("Barman WAL 同步异常：成功 %d，失败 %d，链路状态 %s", result.SyncedArchives, result.FailedArchives, result.LogChainStatus), 1000)
		server.Status = DatabaseBarmanServerStatusDegraded
		server.LastError = job.ErrorMessage
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		server.Status = DatabaseBarmanServerStatusHealthy
		server.LastError = ""
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.barmanServerRepo.Update(ctx, server)
}

func (uc *UseCase) executeBarmanBackupJob(ctx context.Context, serverID, jobID, recordID, taskID uint, audit *DatabaseQueryAudit) {
	if uc.barmanServerRepo == nil || uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.backupRecordRepo == nil {
		return
	}
	server, serverErr := uc.barmanServerRepo.GetByID(ctx, serverID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if serverErr != nil || jobErr != nil || server == nil || job == nil {
		return
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, server.RunnerHostID)
	if err != nil || host == nil {
		return
	}
	var record *DatabaseBackupRecord
	if recordID > 0 {
		record, _ = uc.backupRecordRepo.GetByID(ctx, recordID)
	}
	var task *DatabaseBackupTask
	if taskID > 0 && uc.backupTaskRepo != nil {
		task, _ = uc.backupTaskRepo.GetByID(ctx, taskID)
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)
	if record != nil {
		uc.markBackupRecordStatus(ctx, record, DatabaseBackupStatusRunning, "Barman 物理备份执行中")
	}
	if task != nil {
		uc.markBackupTaskStatus(ctx, task, DatabaseBackupStatusRunning, "Barman 物理备份执行中")
	}

	stdout, stderr, exitCode, runErr := uc.runBarmanBackupCommand(ctx, server, host)
	finished := time.Now()
	result := parseBarmanBackupRunnerResult(server, host, stdout, stderr, exitCode, runErr, started, finished)
	result.BackupRecordID = recordID
	result.BackupTaskID = taskID
	uc.applyBarmanBackupResult(ctx, server, job, record, task, audit, result, runErr)
}

func (uc *UseCase) runBarmanBackupCommand(ctx context.Context, server *DatabaseBarmanServer, host *DatabaseRunnerHost) (string, string, int, error) {
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	if err := validateBarmanRunnerHost(host); err != nil {
		return "", "", 1, err
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	script := buildBarmanBackupScript(server)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func (uc *UseCase) applyBarmanBackupResult(ctx context.Context, server *DatabaseBarmanServer, job *DatabaseRunnerJob, record *DatabaseBackupRecord, task *DatabaseBackupTask, audit *DatabaseQueryAudit, result barmanBackupRunnerResult, runErr error) {
	if server == nil || job == nil {
		return
	}
	now := time.Now()
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = result.ExitCode
	job.FinishedAt = &now
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &now
	server.LastCatalogSyncAt = &now
	durationMs := result.DurationMs
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, job.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, job.ErrorMessage)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if result.BackupExitCode != 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		message := firstNonEmpty(strings.TrimSpace(result.BackupError), strings.TrimSpace(result.BackupOutput), fmt.Sprintf("barman backup 退出码 %d", result.BackupExitCode))
		job.ErrorMessage = trimText(message, 1000)
		finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, job.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, job.ErrorMessage)
		server.Status = DatabaseBarmanServerStatusFailed
		server.LastError = job.ErrorMessage
	} else if strings.TrimSpace(result.BackupID) == "" || strings.TrimSpace(result.ShowJSON) == "" {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = "Barman backup 已执行，但未能解析 backup ID 或 show-backup 元数据"
		finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, job.ErrorMessage)
		uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, job.ErrorMessage)
		server.Status = DatabaseBarmanServerStatusDegraded
		server.LastError = job.ErrorMessage
	} else {
		catalog := parseBarmanBackupCatalog(result.BackupID, server.BarmanServerName, result.ShowJSON)
		if record != nil {
			applyBarmanCatalogToBackupRecord(server, record, catalog)
			record.TaskID = result.BackupTaskID
			record.TriggerType = normalizeBackupTriggerType(record.TriggerType)
			record.Status = normalizeBackupRecordStatus(catalog.Status)
			if record.Status == "" {
				record.Status = DatabaseBackupStatusSuccess
			}
			record.ErrorMessage = "Barman 物理备份完成并已同步 catalog"
			record.FinishedAt = firstNonNilTime(record.FinishedAt, &now)
			if record.StartedAt != nil && record.FinishedAt != nil {
				record.DurationMs = record.FinishedAt.Sub(*record.StartedAt).Milliseconds()
			}
			if err := uc.backupRecordRepo.Update(ctx, record); err != nil {
				job.Status = DatabaseRunnerJobStatusFailed
				job.ErrorMessage = trimText("更新 Barman 备份记录失败: "+err.Error(), 1000)
				finishBarmanBackupRecordAndTask(ctx, uc, record, task, DatabaseBackupStatusFailed, now, durationMs, job.ErrorMessage)
				uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, job.ErrorMessage)
				server.Status = DatabaseBarmanServerStatusDegraded
				server.LastError = job.ErrorMessage
			} else {
				result.SyncedRecord = true
				job.Status = DatabaseRunnerJobStatusSuccess
				job.ErrorMessage = ""
				if task != nil {
					uc.finishBackupTask(ctx, task, now, DatabaseBackupStatusSuccess, "Barman 物理备份完成并已同步 catalog")
				}
				uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")
				server.Status = DatabaseBarmanServerStatusHealthy
				server.LastError = ""
			}
		} else if _, err := uc.upsertBarmanBackupRecord(ctx, server, catalog); err != nil {
			job.Status = DatabaseRunnerJobStatusFailed
			job.ErrorMessage = trimText("同步 Barman 备份记录失败: "+err.Error(), 1000)
			uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusFailed, durationMs, job.ErrorMessage)
			server.Status = DatabaseBarmanServerStatusDegraded
			server.LastError = job.ErrorMessage
		} else {
			result.SyncedRecord = true
			job.Status = DatabaseRunnerJobStatusSuccess
			job.ErrorMessage = ""
			uc.finishBackupAudit(ctx, audit, DatabaseQueryStatusSuccess, durationMs, "")
			server.Status = DatabaseBarmanServerStatusHealthy
			server.LastError = ""
		}
		resultJSON, _ = json.Marshal(result)
		job.ResultJSON = string(resultJSON)
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.barmanServerRepo.Update(ctx, server)
}

func (uc *UseCase) upsertBarmanBackupRecord(ctx context.Context, server *DatabaseBarmanServer, catalog barmanBackupCatalogRecord) (bool, error) {
	if server == nil {
		return false, fmt.Errorf("Barman Server 不能为空")
	}
	if strings.TrimSpace(catalog.BackupID) == "" {
		return false, fmt.Errorf("Barman backup_id 不能为空")
	}
	record, err := uc.backupRecordRepo.GetByExternalBackup(ctx, server.SourceInstanceID, "barman", server.BarmanServerName, catalog.BackupID)
	created := false
	if err != nil || record == nil {
		created = true
		record = &DatabaseBackupRecord{}
	}
	applyBarmanCatalogToBackupRecord(server, record, catalog)
	if created {
		return true, uc.backupRecordRepo.Create(ctx, record)
	}
	return false, uc.backupRecordRepo.Update(ctx, record)
}

func applyBarmanCatalogToBackupRecord(server *DatabaseBarmanServer, record *DatabaseBackupRecord, catalog barmanBackupCatalogRecord) {
	if server == nil || record == nil {
		return
	}
	status := normalizeBackupRecordStatus(catalog.Status)
	record.InstanceID = server.SourceInstanceID
	if strings.TrimSpace(record.TriggerType) == "" {
		record.TriggerType = DatabaseBackupTriggerExternal
	}
	record.BackupType = DatabaseBackupTypePhysical
	record.BackupMethod = DatabaseBackupMethodPhysical
	record.BackupLevel = normalizeBackupLevel(catalog.BackupLevel)
	record.BackupEngine = "barman"
	record.ExternalBackupID = trimText(catalog.BackupID, 120)
	record.ExternalServerName = trimText(server.BarmanServerName, 120)
	record.BackupScope = "cluster"
	record.ToolName = "barman"
	record.ToolVersion = trimText(server.BarmanVersion, 120)
	record.SourceInstanceID = server.SourceInstanceID
	record.SourceRole = "external"
	record.StorageType = DatabaseBackupStorageExternal
	record.StorageURI = trimText(fmt.Sprintf("barman://%s/%s", server.BarmanServerName, catalog.BackupID), 1000)
	record.ManifestJSON = trimText(catalog.ManifestJSON, 60000)
	record.Status = status
	record.FileName = trimText(catalog.BackupID, 255)
	record.FileSize = catalog.FileSize
	record.ChecksumSHA256 = trimText(catalog.ChecksumSHA256, 64)
	record.Compression = "external"
	record.VerifyStatus = DatabaseBackupVerifyStatusPending
	record.StartedAt = catalog.StartedAt
	record.LastHeartbeatAt = catalog.FinishedAt
	record.FinishedAt = catalog.FinishedAt
	record.RecoverableFrom = firstNonNilTime(catalog.RecoverableFrom, catalog.StartedAt)
	record.RecoverableUntil = firstNonNilTime(catalog.RecoverableUntil, catalog.FinishedAt)
	record.PGSystemIdentifier = trimText(firstNonEmpty(catalog.PGSystemIdentifier, server.PGSystemIdentifier), 120)
	record.TimelineID = trimText(catalog.TimelineID, 60)
	record.WALSegmentSize = catalog.WALSegmentSize
	if record.WALSegmentSize == 0 {
		record.WALSegmentSize = server.WALSegmentSize
	}
	record.StartLSN = trimText(catalog.StartLSN, 120)
	record.EndLSN = trimText(catalog.EndLSN, 120)
	record.WALStart = trimText(catalog.WALStart, 255)
	record.WALEnd = trimText(catalog.WALEnd, 255)
	record.ChainID = trimText(fmt.Sprintf("barman-%d", server.ID), 64)
	record.ErrorMessage = ""
	if record.StartedAt != nil && record.FinishedAt != nil {
		record.DurationMs = record.FinishedAt.Sub(*record.StartedAt).Milliseconds()
	}
}

func (uc *UseCase) ensureBarmanWALArchiveStream(ctx context.Context, server *DatabaseBarmanServer) (*DatabaseLogArchiveStream, error) {
	if server == nil {
		return nil, fmt.Errorf("Barman Server 不能为空")
	}
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	items, _, err := uc.logArchiveStreamRepo.List(ctx, &DatabaseLogArchiveStreamListRequest{
		Page:        1,
		PageSize:    100,
		InstanceID:  server.SourceInstanceID,
		ArchiveType: DatabaseArchiveTypeWAL,
	})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.ArchiveEngine) != "barman" {
			continue
		}
		if barmanStreamMatchesServer(item, server) {
			return item, nil
		}
	}
	config := map[string]any{
		"barmanServerId":   server.ID,
		"barmanServerName": server.BarmanServerName,
		"walSegmentSize":   server.WALSegmentSize,
		"source":           "barman_wal_sync",
	}
	configJSON, _ := json.Marshal(config)
	now := time.Now()
	stream := &DatabaseLogArchiveStream{
		InstanceID:       server.SourceInstanceID,
		SourceInstanceID: server.SourceInstanceID,
		Engine:           DBTypePostgreSQL,
		ArchiveType:      DatabaseArchiveTypeWAL,
		ArchiveMode:      DatabaseArchiveModeExternal,
		ArchiveEngine:    "barman",
		RunnerHostID:     server.RunnerHostID,
		RPOTargetSeconds: 300,
		RetentionDays:    30,
		Enabled:          true,
		Status:           DatabaseLogArchiveStreamStatusPending,
		DesiredState:     DatabaseLogArchiveDesiredStateStopped,
		DaemonStatus:     DatabaseLogArchiveDaemonStatusStopped,
		LastHeartbeatAt:  &now,
		ConfigJSON:       trimText(string(configJSON), 4000),
	}
	if err := uc.logArchiveStreamRepo.Create(ctx, stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func barmanStreamMatchesServer(stream *DatabaseLogArchiveStream, server *DatabaseBarmanServer) bool {
	if stream == nil || server == nil {
		return false
	}
	if strings.TrimSpace(stream.ConfigJSON) == "" {
		return strings.TrimSpace(stream.ArchiveEngine) == "barman" && stream.RunnerHostID == server.RunnerHostID
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stream.ConfigJSON), &payload); err != nil {
		return false
	}
	if id, ok := payload["barmanServerId"].(float64); ok && uint(id) == server.ID {
		return true
	}
	if name, ok := payload["barmanServerName"].(string); ok && strings.TrimSpace(name) == server.BarmanServerName {
		return true
	}
	return false
}

func (uc *UseCase) upsertBarmanWALArchive(ctx context.Context, stream *DatabaseLogArchiveStream, server *DatabaseBarmanServer, catalog barmanWALCatalogRecord) (bool, error) {
	if uc.logArchiveRepo == nil {
		return false, fmt.Errorf("日志归档仓库未配置")
	}
	if stream == nil || stream.ID == 0 {
		return false, fmt.Errorf("WAL 归档流不能为空")
	}
	if strings.TrimSpace(catalog.FileName) == "" {
		return false, fmt.Errorf("WAL 文件名不能为空")
	}
	item, err := uc.logArchiveRepo.GetByStreamFile(ctx, stream.ID, catalog.FileName)
	created := false
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
		created = true
		item = &DatabaseLogArchive{}
	}
	item.StreamID = stream.ID
	item.InstanceID = stream.InstanceID
	item.SourceInstanceID = stream.SourceInstanceID
	item.Engine = DBTypePostgreSQL
	item.ArchiveType = DatabaseArchiveTypeWAL
	item.FileName = trimText(catalog.FileName, 255)
	item.StorageURI = trimText(catalog.StorageURI, 1000)
	item.FileSize = catalog.FileSize
	item.ChecksumSHA256 = trimText(catalog.ChecksumSHA256, 64)
	item.FirstEventTime = catalog.FirstEventTime
	item.LastEventTime = catalog.LastEventTime
	item.Status = normalizeLogArchiveStatus(catalog.Status)
	item.ArchivedAt = catalog.ArchivedAt
	item.PGSystemIdentifier = trimText(firstNonEmpty(catalog.PGSystemIdentifier, server.PGSystemIdentifier), 120)
	item.TimelineID = trimText(catalog.TimelineID, 60)
	item.WALSegmentSize = catalog.WALSegmentSize
	item.ExternalServerName = trimText(catalog.ExternalServerName, 120)
	item.StartLSN = trimText(catalog.StartLSN, 120)
	item.EndLSN = trimText(catalog.EndLSN, 120)
	item.SegmentNo = trimText(catalog.SegmentNo, 120)
	item.TimelineHistoryURI = trimText(catalog.TimelineHistoryURI, 1000)
	if created {
		return true, uc.logArchiveRepo.Create(ctx, item)
	}
	return false, uc.logArchiveRepo.Update(ctx, item)
}

func updateBarmanWALStreamAfterSync(ctx context.Context, uc *UseCase, stream *DatabaseLogArchiveStream, result barmanWALSyncRunnerResult, now time.Time) {
	if uc == nil || uc.logArchiveStreamRepo == nil || stream == nil {
		return
	}
	stream.LastArchivedAt = &now
	stream.LastHeartbeatAt = &now
	stream.LastArchiveName = trimText(result.LastArchiveName, 255)
	stream.CursorFile = stream.LastArchiveName
	stream.LastEventTime = &now
	stream.ArchiveLagSeconds = 0
	if result.SyncedArchives > 0 && result.FailedArchives == 0 && result.LogChainStatus == DatabaseLogChainStatusComplete && result.TimelineHistoryStatus != DatabaseLogChainStatusTimelineGap {
		stream.Status = DatabaseLogArchiveStreamStatusRunning
		stream.DaemonStatus = DatabaseLogArchiveDaemonStatusStopped
		stream.ConsecutiveFailures = 0
		stream.LastError = ""
	} else {
		stream.Status = DatabaseLogArchiveStreamStatusDegraded
		stream.DaemonStatus = DatabaseLogArchiveDaemonStatusDegraded
		stream.ConsecutiveFailures++
		stream.LastError = trimText(fmt.Sprintf("Barman WAL 同步异常：链路=%s，timeline=%s，失败=%d", result.LogChainStatus, result.TimelineHistoryStatus, result.FailedArchives), 1000)
	}
	_ = uc.logArchiveStreamRepo.Update(ctx, stream)
}

func finishBarmanBackupRecordAndTask(ctx context.Context, uc *UseCase, record *DatabaseBackupRecord, task *DatabaseBackupTask, status string, finishedAt time.Time, durationMs int64, message string) {
	if uc == nil {
		return
	}
	if record != nil && uc.backupRecordRepo != nil {
		if record.StartedAt == nil {
			started := finishedAt
			record.StartedAt = &started
		}
		record.Status = status
		record.FinishedAt = &finishedAt
		record.LastHeartbeatAt = &finishedAt
		record.DurationMs = durationMs
		record.ErrorMessage = trimText(message, 500)
		if status == DatabaseBackupStatusFailed {
			record.VerifyStatus = DatabaseBackupVerifyStatusFailed
			record.VerifyMessage = trimText(message, 500)
		}
		_ = uc.backupRecordRepo.Update(ctx, record)
	}
	if task != nil && uc.backupTaskRepo != nil {
		uc.finishBackupTask(ctx, task, finishedAt, status, message)
	}
}

func buildBarmanCheckScript(server *DatabaseBarmanServer) string {
	return strings.Join([]string{
		"set -u",
		"SERVER=" + shellSingleQuote(server.BarmanServerName),
		"CONFIG_PATH=" + shellSingleQuote(server.ConfigPath),
		`tmp="$(mktemp -d "${TMPDIR:-/tmp}/opshub-barman-check.XXXXXX")"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		`b64() { if [ -f "$1" ]; then base64 "$1" | tr -d '\n'; fi; }`,
		`run_barman() { if [ -n "$CONFIG_PATH" ]; then barman -c "$CONFIG_PATH" "$@"; else barman "$@"; fi; }`,
		"set +e",
		`run_barman --version > "$tmp/version.txt" 2>&1`,
		`version_exit="$?"`,
		`run_barman -f json check "$SERVER" > "$tmp/check.json" 2> "$tmp/check.err"`,
		`check_exit="$?"`,
		`run_barman check "$SERVER" > "$tmp/check.txt" 2>&1`,
		`run_barman -f json status "$SERVER" > "$tmp/status.json" 2> "$tmp/status.err"`,
		`status_exit="$?"`,
		`printf 'OPSHUB_BARMAN_VERSION_EXIT=%s\n' "$version_exit"`,
		`printf 'OPSHUB_BARMAN_VERSION_B64=%s\n' "$(b64 "$tmp/version.txt")"`,
		`printf 'OPSHUB_BARMAN_CHECK_EXIT=%s\n' "$check_exit"`,
		`printf 'OPSHUB_BARMAN_CHECK_JSON_B64=%s\n' "$(b64 "$tmp/check.json")"`,
		`printf 'OPSHUB_BARMAN_CHECK_OUTPUT_B64=%s\n' "$(b64 "$tmp/check.txt")"`,
		`printf 'OPSHUB_BARMAN_CHECK_ERROR_B64=%s\n' "$(b64 "$tmp/check.err")"`,
		`printf 'OPSHUB_BARMAN_STATUS_EXIT=%s\n' "$status_exit"`,
		`printf 'OPSHUB_BARMAN_STATUS_JSON_B64=%s\n' "$(b64 "$tmp/status.json")"`,
		`printf 'OPSHUB_BARMAN_STATUS_ERROR_B64=%s\n' "$(b64 "$tmp/status.err")"`,
		"exit 0",
	}, "\n")
}

func buildBarmanCatalogSyncScript(server *DatabaseBarmanServer, maxBackups int) string {
	if maxBackups <= 0 {
		maxBackups = defaultBarmanCatalogMaxBackups
	}
	return strings.Join([]string{
		"set -u",
		"SERVER=" + shellSingleQuote(server.BarmanServerName),
		"CONFIG_PATH=" + shellSingleQuote(server.ConfigPath),
		"MAX_BACKUPS=" + shellSingleQuote(strconv.Itoa(maxBackups)),
		`tmp="$(mktemp -d "${TMPDIR:-/tmp}/opshub-barman-catalog.XXXXXX")"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		`b64() { if [ -f "$1" ]; then base64 "$1" | tr -d '\n'; fi; }`,
		`run_barman() { if [ -n "$CONFIG_PATH" ]; then barman -c "$CONFIG_PATH" "$@"; else barman "$@"; fi; }`,
		`run_list_json() { run_barman -f json list-backup "$SERVER" > "$tmp/list.json" 2> "$tmp/list.err"; code="$?"; if [ "$code" != "0" ]; then run_barman -f json list-backups "$SERVER" > "$tmp/list.json" 2>> "$tmp/list.err"; code="$?"; fi; return "$code"; }`,
		`run_list_text() { run_barman list-backup "$SERVER" > "$tmp/list.txt" 2> "$tmp/list-text.err"; code="$?"; if [ "$code" != "0" ]; then run_barman list-backups "$SERVER" > "$tmp/list.txt" 2>> "$tmp/list-text.err"; code="$?"; fi; return "$code"; }`,
		"set +e",
		`run_list_json`,
		`list_exit="$?"`,
		`run_list_text`,
		`list_text_exit="$?"`,
		`printf 'OPSHUB_BARMAN_LIST_EXIT=%s\n' "$list_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_TEXT_EXIT=%s\n' "$list_text_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_JSON_B64=%s\n' "$(b64 "$tmp/list.json")"`,
		`printf 'OPSHUB_BARMAN_LIST_OUTPUT_B64=%s\n' "$(b64 "$tmp/list.txt")"`,
		`printf 'OPSHUB_BARMAN_LIST_ERROR_B64=%s\n' "$(b64 "$tmp/list.err")"`,
		`awk '{print $1}' "$tmp/list.txt" | grep -E '^[A-Za-z0-9_.:+-]+$' | head -n "$MAX_BACKUPS" > "$tmp/ids.txt" || true`,
		`while IFS= read -r backup_id; do`,
		`  [ -n "$backup_id" ] || continue`,
		`  case "$backup_id" in *[!A-Za-z0-9_.:+-]* ) continue ;; esac`,
		`  safe_id="$(printf '%s' "$backup_id" | tr -c 'A-Za-z0-9_.:+-' '_')"`,
		`  show_json="$tmp/show_${safe_id}.json"`,
		`  show_err="$tmp/show_${safe_id}.err"`,
		`  run_barman -f json show-backup "$SERVER" "$backup_id" > "$show_json" 2> "$show_err"`,
		`  show_exit="$?"`,
		`  printf 'OPSHUB_BARMAN_SHOW_BACKUP=%s|%s|%s|%s\n' "$backup_id" "$show_exit" "$(b64 "$show_json")" "$(b64 "$show_err")"`,
		`done < "$tmp/ids.txt"`,
		"exit 0",
	}, "\n")
}

func buildBarmanWALSyncScript(server *DatabaseBarmanServer, maxBackups int) string {
	if maxBackups <= 0 {
		maxBackups = defaultBarmanCatalogMaxBackups
	}
	return strings.Join([]string{
		"set -u",
		"SERVER=" + shellSingleQuote(server.BarmanServerName),
		"CONFIG_PATH=" + shellSingleQuote(server.ConfigPath),
		"MAX_BACKUPS=" + shellSingleQuote(strconv.Itoa(maxBackups)),
		`tmp="$(mktemp -d "${TMPDIR:-/tmp}/opshub-barman-wal.XXXXXX")"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		`b64() { if [ -f "$1" ]; then base64 "$1" | tr -d '\n'; fi; }`,
		`run_barman() { if [ -n "$CONFIG_PATH" ]; then barman -c "$CONFIG_PATH" "$@"; else barman "$@"; fi; }`,
		`run_list_json() { run_barman -f json list-backup "$SERVER" > "$tmp/list.json" 2> "$tmp/list.err"; code="$?"; if [ "$code" != "0" ]; then run_barman -f json list-backups "$SERVER" > "$tmp/list.json" 2>> "$tmp/list.err"; code="$?"; fi; return "$code"; }`,
		`run_list_text() { run_barman list-backup "$SERVER" > "$tmp/list.txt" 2> "$tmp/list-text.err"; code="$?"; if [ "$code" != "0" ]; then run_barman list-backups "$SERVER" > "$tmp/list.txt" 2>> "$tmp/list-text.err"; code="$?"; fi; return "$code"; }`,
		"set +e",
		`run_list_json`,
		`list_exit="$?"`,
		`run_list_text`,
		`list_text_exit="$?"`,
		`printf 'OPSHUB_BARMAN_LIST_EXIT=%s\n' "$list_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_TEXT_EXIT=%s\n' "$list_text_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_JSON_B64=%s\n' "$(b64 "$tmp/list.json")"`,
		`printf 'OPSHUB_BARMAN_LIST_OUTPUT_B64=%s\n' "$(b64 "$tmp/list.txt")"`,
		`printf 'OPSHUB_BARMAN_LIST_ERROR_B64=%s\n' "$(b64 "$tmp/list.err")"`,
		`awk '{print $1}' "$tmp/list.txt" | grep -E '^[A-Za-z0-9_.:+-]+$' | head -n "$MAX_BACKUPS" > "$tmp/ids.txt" || true`,
		`while IFS= read -r backup_id; do`,
		`  [ -n "$backup_id" ] || continue`,
		`  case "$backup_id" in *[!A-Za-z0-9_.:+-]* ) continue ;; esac`,
		`  safe_id="$(printf '%s' "$backup_id" | tr -c 'A-Za-z0-9_.:+-' '_')"`,
		`  show_json="$tmp/show_${safe_id}.json"`,
		`  show_err="$tmp/show_${safe_id}.err"`,
		`  files_txt="$tmp/files_${safe_id}.txt"`,
		`  files_err="$tmp/files_${safe_id}.err"`,
		`  run_barman -f json show-backup "$SERVER" "$backup_id" > "$show_json" 2> "$show_err"`,
		`  show_exit="$?"`,
		`  printf 'OPSHUB_BARMAN_SHOW_BACKUP=%s|%s|%s|%s\n' "$backup_id" "$show_exit" "$(b64 "$show_json")" "$(b64 "$show_err")"`,
		`  run_barman list-files "$SERVER" "$backup_id" > "$files_txt" 2> "$files_err"`,
		`  files_exit="$?"`,
		`  printf 'OPSHUB_BARMAN_LIST_FILES=%s|%s|%s|%s\n' "$backup_id" "$files_exit" "$(b64 "$files_txt")" "$(b64 "$files_err")"`,
		`done < "$tmp/ids.txt"`,
		"exit 0",
	}, "\n")
}

func buildBarmanBackupScript(server *DatabaseBarmanServer) string {
	return strings.Join([]string{
		"set -u",
		"SERVER=" + shellSingleQuote(server.BarmanServerName),
		"CONFIG_PATH=" + shellSingleQuote(server.ConfigPath),
		`tmp="$(mktemp -d "${TMPDIR:-/tmp}/opshub-barman-backup.XXXXXX")"`,
		`trap 'rm -rf "$tmp"' EXIT`,
		`b64() { if [ -f "$1" ]; then base64 "$1" | tr -d '\n'; fi; }`,
		`run_barman() { if [ -n "$CONFIG_PATH" ]; then barman -c "$CONFIG_PATH" "$@"; else barman "$@"; fi; }`,
		`run_list_json() { run_barman -f json list-backup "$SERVER" > "$tmp/list.json" 2> "$tmp/list.err"; code="$?"; if [ "$code" != "0" ]; then run_barman -f json list-backups "$SERVER" > "$tmp/list.json" 2>> "$tmp/list.err"; code="$?"; fi; return "$code"; }`,
		`run_list_text() { run_barman list-backup "$SERVER" > "$tmp/list.txt" 2> "$tmp/list-text.err"; code="$?"; if [ "$code" != "0" ]; then run_barman list-backups "$SERVER" > "$tmp/list.txt" 2>> "$tmp/list-text.err"; code="$?"; fi; return "$code"; }`,
		"set +e",
		`run_barman backup "$SERVER" > "$tmp/backup.txt" 2> "$tmp/backup.err"`,
		`backup_exit="$?"`,
		`run_list_json`,
		`list_exit="$?"`,
		`run_list_text`,
		`list_text_exit="$?"`,
		`backup_id="$(awk '{print $1}' "$tmp/list.txt" | grep -E '^[A-Za-z0-9_.:+-]+$' | head -n 1)"`,
		`printf 'OPSHUB_BARMAN_BACKUP_EXIT=%s\n' "$backup_exit"`,
		`printf 'OPSHUB_BARMAN_BACKUP_OUTPUT_B64=%s\n' "$(b64 "$tmp/backup.txt")"`,
		`printf 'OPSHUB_BARMAN_BACKUP_ERROR_B64=%s\n' "$(b64 "$tmp/backup.err")"`,
		`printf 'OPSHUB_BARMAN_LIST_EXIT=%s\n' "$list_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_TEXT_EXIT=%s\n' "$list_text_exit"`,
		`printf 'OPSHUB_BARMAN_LIST_JSON_B64=%s\n' "$(b64 "$tmp/list.json")"`,
		`printf 'OPSHUB_BARMAN_LIST_OUTPUT_B64=%s\n' "$(b64 "$tmp/list.txt")"`,
		`printf 'OPSHUB_BARMAN_LIST_ERROR_B64=%s\n' "$(b64 "$tmp/list.err")"`,
		`if [ -n "$backup_id" ]; then`,
		`  show_json="$tmp/show_${backup_id}.json"`,
		`  show_err="$tmp/show_${backup_id}.err"`,
		`  check_txt="$tmp/check_${backup_id}.txt"`,
		`  check_err="$tmp/check_${backup_id}.err"`,
		`  run_barman -f json show-backup "$SERVER" "$backup_id" > "$show_json" 2> "$show_err"`,
		`  show_exit="$?"`,
		`  run_barman check-backup "$SERVER" "$backup_id" > "$check_txt" 2> "$check_err"`,
		`  check_exit="$?"`,
		`  printf 'OPSHUB_BARMAN_BACKUP_ID=%s\n' "$backup_id"`,
		`  printf 'OPSHUB_BARMAN_SHOW_BACKUP=%s|%s|%s|%s\n' "$backup_id" "$show_exit" "$(b64 "$show_json")" "$(b64 "$show_err")"`,
		`  printf 'OPSHUB_BARMAN_CHECK_BACKUP_EXIT=%s\n' "$check_exit"`,
		`  printf 'OPSHUB_BARMAN_CHECK_BACKUP_OUTPUT_B64=%s\n' "$(b64 "$check_txt")"`,
		`  printf 'OPSHUB_BARMAN_CHECK_BACKUP_ERROR_B64=%s\n' "$(b64 "$check_err")"`,
		`fi`,
		"exit 0",
	}, "\n")
}

func parseBarmanCheckRunnerResult(server *DatabaseBarmanServer, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) barmanCheckRunnerResult {
	markers := parseBarmanMarkers(stdout)
	checkExit := markerInt(markers, "OPSHUB_BARMAN_CHECK_EXIT", exitCode)
	statusExit := markerInt(markers, "OPSHUB_BARMAN_STATUS_EXIT", 0)
	version := trimBarmanVersion(markerString(markers, "OPSHUB_BARMAN_VERSION_B64"))
	result := barmanCheckRunnerResult{
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		BarmanServerID:   server.ID,
		BarmanServerName: server.BarmanServerName,
		Stdout:           trimText(stdout, maxRunnerOutputLength),
		Stderr:           trimText(stderr, maxRunnerOutputLength),
		ExitCode:         exitCode,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
		FinishedAt:       finished.Format("2006-01-02 15:04:05"),
		DurationMs:       finished.Sub(started).Milliseconds(),
		Version:          version,
		CheckExitCode:    checkExit,
		StatusExitCode:   statusExit,
		CheckJSON:        trimText(markerString(markers, "OPSHUB_BARMAN_CHECK_JSON_B64"), 60000),
		CheckOutput:      trimText(markerString(markers, "OPSHUB_BARMAN_CHECK_OUTPUT_B64"), maxRunnerOutputLength),
		CheckError:       trimText(markerString(markers, "OPSHUB_BARMAN_CHECK_ERROR_B64"), maxRunnerOutputLength),
		StatusJSON:       trimText(markerString(markers, "OPSHUB_BARMAN_STATUS_JSON_B64"), 60000),
		StatusError:      trimText(markerString(markers, "OPSHUB_BARMAN_STATUS_ERROR_B64"), maxRunnerOutputLength),
		DerivedStatus:    DatabaseBarmanServerStatusHealthy,
	}
	if runErr != nil {
		result.DerivedStatus = DatabaseBarmanServerStatusFailed
	} else if checkExit != 0 {
		result.DerivedStatus = DatabaseBarmanServerStatusDegraded
	}
	return result
}

func parseBarmanCatalogSyncRunnerResult(server *DatabaseBarmanServer, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) barmanCatalogSyncRunnerResult {
	markers := parseBarmanMarkers(stdout)
	listJSON := markerString(markers, "OPSHUB_BARMAN_LIST_JSON_B64")
	listOutput := markerString(markers, "OPSHUB_BARMAN_LIST_OUTPUT_B64")
	result := barmanCatalogSyncRunnerResult{
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		BarmanServerID:   server.ID,
		BarmanServerName: server.BarmanServerName,
		Stdout:           trimText(stdout, maxRunnerOutputLength),
		Stderr:           trimText(stderr, maxRunnerOutputLength),
		ExitCode:         exitCode,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
		FinishedAt:       finished.Format("2006-01-02 15:04:05"),
		DurationMs:       finished.Sub(started).Milliseconds(),
		ListExitCode:     markerInt(markers, "OPSHUB_BARMAN_LIST_EXIT", exitCode),
		ListJSON:         trimText(listJSON, 60000),
		ListOutput:       trimText(listOutput, maxRunnerOutputLength),
		ListError:        trimText(markerString(markers, "OPSHUB_BARMAN_LIST_ERROR_B64"), maxRunnerOutputLength),
		BackupIDs:        extractBarmanBackupIDs(listJSON, listOutput),
	}
	if runErr != nil && result.ListExitCode == 0 {
		result.ListExitCode = exitCode
	}
	return result
}

func parseBarmanWALSyncRunnerResult(server *DatabaseBarmanServer, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) barmanWALSyncRunnerResult {
	markers := parseBarmanMarkers(stdout)
	listJSON := markerString(markers, "OPSHUB_BARMAN_LIST_JSON_B64")
	listOutput := markerString(markers, "OPSHUB_BARMAN_LIST_OUTPUT_B64")
	result := barmanWALSyncRunnerResult{
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		BarmanServerID:   server.ID,
		BarmanServerName: server.BarmanServerName,
		Stdout:           trimText(stdout, maxRunnerOutputLength),
		Stderr:           trimText(stderr, maxRunnerOutputLength),
		ExitCode:         exitCode,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
		FinishedAt:       finished.Format("2006-01-02 15:04:05"),
		DurationMs:       finished.Sub(started).Milliseconds(),
		ListExitCode:     markerInt(markers, "OPSHUB_BARMAN_LIST_EXIT", exitCode),
		ListJSON:         trimText(listJSON, 60000),
		ListOutput:       trimText(listOutput, maxRunnerOutputLength),
		ListError:        trimText(markerString(markers, "OPSHUB_BARMAN_LIST_ERROR_B64"), maxRunnerOutputLength),
		BackupIDs:        extractBarmanBackupIDs(listJSON, listOutput),
		LogChainStatus:   DatabaseLogChainStatusComplete,
	}
	if runErr != nil && result.ListExitCode == 0 {
		result.ListExitCode = exitCode
	}
	return result
}

func parseBarmanBackupRunnerResult(server *DatabaseBarmanServer, host *DatabaseRunnerHost, stdout, stderr string, exitCode int, runErr error, started, finished time.Time) barmanBackupRunnerResult {
	markers := parseBarmanMarkers(stdout)
	listJSON := markerString(markers, "OPSHUB_BARMAN_LIST_JSON_B64")
	listOutput := markerString(markers, "OPSHUB_BARMAN_LIST_OUTPUT_B64")
	showOutputs := parseBarmanShowOutputs(stdout)
	backupID := markerString(markers, "OPSHUB_BARMAN_BACKUP_ID")
	if strings.TrimSpace(backupID) == "" {
		ids := extractBarmanBackupIDs(listJSON, listOutput)
		if len(ids) > 0 {
			backupID = ids[0]
		}
	}
	result := barmanBackupRunnerResult{
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		BarmanServerID:   server.ID,
		BarmanServerName: server.BarmanServerName,
		Stdout:           trimText(stdout, maxRunnerOutputLength),
		Stderr:           trimText(stderr, maxRunnerOutputLength),
		ExitCode:         exitCode,
		StartedAt:        started.Format("2006-01-02 15:04:05"),
		FinishedAt:       finished.Format("2006-01-02 15:04:05"),
		DurationMs:       finished.Sub(started).Milliseconds(),
		BackupExitCode:   markerInt(markers, "OPSHUB_BARMAN_BACKUP_EXIT", exitCode),
		CheckExitCode:    markerInt(markers, "OPSHUB_BARMAN_CHECK_BACKUP_EXIT", 0),
		ListExitCode:     markerInt(markers, "OPSHUB_BARMAN_LIST_EXIT", 0),
		BackupID:         trimText(backupID, 120),
		BackupOutput:     trimText(markerString(markers, "OPSHUB_BARMAN_BACKUP_OUTPUT_B64"), maxRunnerOutputLength),
		BackupError:      trimText(markerString(markers, "OPSHUB_BARMAN_BACKUP_ERROR_B64"), maxRunnerOutputLength),
		CheckOutput:      trimText(markerString(markers, "OPSHUB_BARMAN_CHECK_BACKUP_OUTPUT_B64"), maxRunnerOutputLength),
		CheckError:       trimText(markerString(markers, "OPSHUB_BARMAN_CHECK_BACKUP_ERROR_B64"), maxRunnerOutputLength),
		ListJSON:         trimText(listJSON, 60000),
		ListOutput:       trimText(listOutput, maxRunnerOutputLength),
		ListError:        trimText(markerString(markers, "OPSHUB_BARMAN_LIST_ERROR_B64"), maxRunnerOutputLength),
	}
	for _, show := range showOutputs {
		if show.BackupID == result.BackupID || result.ShowJSON == "" {
			result.ShowJSON = trimText(show.ShowJSON, 60000)
			result.ShowError = trimText(show.ShowErr, maxRunnerOutputLength)
			break
		}
	}
	if runErr != nil && result.BackupExitCode == 0 {
		result.BackupExitCode = exitCode
	}
	return result
}

func parseBarmanMarkers(stdout string) map[string]string {
	markers := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "OPSHUB_BARMAN_") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.HasSuffix(key, "_B64") {
			if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value)); err == nil {
				markers[key] = string(decoded)
			} else {
				markers[key] = ""
			}
		} else {
			markers[key] = strings.TrimSpace(value)
		}
	}
	return markers
}

func markerString(markers map[string]string, key string) string {
	if markers == nil {
		return ""
	}
	return markers[key]
}

func markerInt(markers map[string]string, key string, fallback int) int {
	value := strings.TrimSpace(markerString(markers, key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBarmanShowOutputs(stdout string) []barmanBackupShowOutput {
	results := make([]barmanBackupShowOutput, 0)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "OPSHUB_BARMAN_SHOW_BACKUP=") {
			continue
		}
		raw := strings.TrimPrefix(line, "OPSHUB_BARMAN_SHOW_BACKUP=")
		parts := strings.SplitN(raw, "|", 4)
		if len(parts) != 4 {
			continue
		}
		exitCode, _ := strconv.Atoi(parts[1])
		showJSON, _ := base64.StdEncoding.DecodeString(parts[2])
		showErr, _ := base64.StdEncoding.DecodeString(parts[3])
		results = append(results, barmanBackupShowOutput{
			BackupID: strings.TrimSpace(parts[0]),
			ExitCode: exitCode,
			ShowJSON: string(showJSON),
			ShowErr:  string(showErr),
		})
	}
	return results
}

func parseBarmanListFilesOutputs(stdout string) []barmanListFilesOutput {
	results := make([]barmanListFilesOutput, 0)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "OPSHUB_BARMAN_LIST_FILES=") {
			continue
		}
		raw := strings.TrimPrefix(line, "OPSHUB_BARMAN_LIST_FILES=")
		parts := strings.SplitN(raw, "|", 4)
		if len(parts) != 4 {
			continue
		}
		exitCode, _ := strconv.Atoi(parts[1])
		output, _ := base64.StdEncoding.DecodeString(parts[2])
		outputErr, _ := base64.StdEncoding.DecodeString(parts[3])
		results = append(results, barmanListFilesOutput{
			BackupID: strings.TrimSpace(parts[0]),
			ExitCode: exitCode,
			Output:   string(output),
			Error:    string(outputErr),
		})
	}
	return results
}

type barmanServerMetadata struct {
	RetentionPolicy          string
	BackupMethod             string
	SlotName                 string
	PGVersion                string
	PGSystemIdentifier       string
	WALSegmentSize           int64
	StreamingArchiverEnabled bool
	StreamingArchiverKnown   bool
	ArchiverEnabled          bool
	ArchiverKnown            bool
}

func parseBarmanServerMetadata(raw, serverName string) barmanServerMetadata {
	root := barmanJSONRoot(raw, serverName)
	return barmanServerMetadata{
		RetentionPolicy:          barmanLookupString(root, "retention_policy", "retentionPolicy", "minimum_redundancy", "retention"),
		BackupMethod:             barmanLookupString(root, "backup_method", "backupMethod"),
		SlotName:                 barmanLookupString(root, "slot_name", "slotName", "streaming_archiver_name"),
		PGVersion:                barmanLookupString(root, "postgres_version", "postgresVersion", "pg_version", "pgVersion"),
		PGSystemIdentifier:       firstNonEmpty(barmanLookupString(root, "system_identifier", "systemIdentifier", "pg_system_identifier"), barmanLookupStringByContains(root, "system", "identifier")),
		WALSegmentSize:           barmanLookupInt64(root, "wal_segment_size", "walSegmentSize"),
		StreamingArchiverEnabled: barmanLookupBool(root, "streaming_archiver", "streaming_archiver_enabled", "streamingArchiverEnabled"),
		StreamingArchiverKnown:   barmanLookupExists(root, "streaming_archiver", "streaming_archiver_enabled", "streamingArchiverEnabled"),
		ArchiverEnabled:          barmanLookupBool(root, "archiver", "archiver_enabled", "archiverEnabled"),
		ArchiverKnown:            barmanLookupExists(root, "archiver", "archiver_enabled", "archiverEnabled"),
	}
}

func parseBarmanBackupCatalog(backupID, serverName, raw string) barmanBackupCatalogRecord {
	root := barmanJSONRoot(raw, serverName)
	started := barmanLookupTime(root, "begin_time", "beginTime", "start_time", "startTime", "started_at", "startedAt")
	finished := barmanLookupTime(root, "end_time", "endTime", "finish_time", "finishTime", "finished_at", "finishedAt", "copy_time")
	level := normalizeBarmanBackupLevel(firstNonEmpty(barmanLookupString(root, "backup_type", "backupType", "type"), "full"))
	status := normalizeBarmanBackupStatus(barmanLookupString(root, "status", "state"))
	return barmanBackupCatalogRecord{
		BackupID:           backupID,
		Status:             status,
		BackupLevel:        level,
		FileSize:           barmanLookupSize(root, "size", "backup_size", "deduplicated_size", "total_size"),
		ChecksumSHA256:     barmanLookupString(root, "checksum_sha256", "checksumSha256", "sha256"),
		StartedAt:          started,
		FinishedAt:         finished,
		RecoverableFrom:    started,
		RecoverableUntil:   finished,
		PGSystemIdentifier: firstNonEmpty(barmanLookupString(root, "system_identifier", "systemIdentifier", "pg_system_identifier"), barmanLookupStringByContains(root, "system", "identifier")),
		TimelineID:         firstNonEmpty(barmanLookupString(root, "timeline", "timeline_id", "timelineId"), barmanLookupStringByContains(root, "timeline")),
		WALSegmentSize:     barmanLookupInt64(root, "wal_segment_size", "walSegmentSize"),
		StartLSN:           firstNonEmpty(barmanLookupString(root, "begin_lsn", "beginLsn", "start_lsn", "startLsn"), barmanLookupStringByContains(root, "begin", "lsn")),
		EndLSN:             firstNonEmpty(barmanLookupString(root, "end_lsn", "endLsn", "stop_lsn", "stopLsn"), barmanLookupStringByContains(root, "end", "lsn")),
		WALStart:           firstNonEmpty(barmanLookupString(root, "begin_wal", "beginWal", "begin_xlog", "beginXlog", "wal_start", "walStart"), barmanLookupStringByContains(root, "begin", "wal")),
		WALEnd:             firstNonEmpty(barmanLookupString(root, "end_wal", "endWal", "end_xlog", "endXlog", "wal_end", "walEnd"), barmanLookupStringByContains(root, "end", "wal")),
		ManifestJSON:       raw,
	}
}

func collectBarmanWALCatalogRecords(server *DatabaseBarmanServer, stdout string) []barmanWALCatalogRecord {
	if server == nil {
		return nil
	}
	recordsByName := make(map[string]barmanWALCatalogRecord)
	showByID := make(map[string]barmanBackupCatalogRecord)
	listFilesProvided := make(map[string]bool)
	for _, show := range parseBarmanShowOutputs(stdout) {
		if show.ExitCode != 0 || strings.TrimSpace(show.ShowJSON) == "" {
			continue
		}
		showByID[show.BackupID] = parseBarmanBackupCatalog(show.BackupID, server.BarmanServerName, show.ShowJSON)
	}
	for _, files := range parseBarmanListFilesOutputs(stdout) {
		if files.ExitCode != 0 {
			continue
		}
		catalog := showByID[files.BackupID]
		fileNames := extractPostgreSQLWALFileNames(files.Output)
		if len(fileNames) > 0 {
			listFilesProvided[files.BackupID] = true
		}
		for _, fileName := range fileNames {
			record := buildBarmanWALRecord(server, catalog, fileName)
			if record.FileName != "" {
				recordsByName[record.FileName] = record
			}
		}
	}
	for backupID, catalog := range showByID {
		if listFilesProvided[backupID] {
			continue
		}
		for _, record := range expandBarmanWALRange(server, catalog) {
			if _, exists := recordsByName[record.FileName]; !exists {
				recordsByName[record.FileName] = record
			}
		}
	}
	records := make([]barmanWALCatalogRecord, 0, len(recordsByName))
	for _, record := range recordsByName {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if left.TimelineID == right.TimelineID {
			return left.SegmentNo < right.SegmentNo || (left.SegmentNo == right.SegmentNo && left.FileName < right.FileName)
		}
		return left.TimelineID < right.TimelineID
	})
	return records
}

func extractPostgreSQLWALFileNames(output string) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, token := range strings.Fields(strings.ReplaceAll(output, "/", " ")) {
		token = strings.Trim(token, " ,;:()[]{}\"'")
		if idx := strings.LastIndex(token, "/"); idx >= 0 {
			token = token[idx+1:]
		}
		if pgwal.IsSegmentName(token) || pgwal.IsTimelineHistoryFile(token) {
			token = strings.ToUpper(token)
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			names = append(names, token)
		}
	}
	return names
}

func expandBarmanWALRange(server *DatabaseBarmanServer, catalog barmanBackupCatalogRecord) []barmanWALCatalogRecord {
	if strings.TrimSpace(catalog.WALStart) == "" || strings.TrimSpace(catalog.WALEnd) == "" {
		return nil
	}
	segmentSize := catalog.WALSegmentSize
	if segmentSize <= 0 && server != nil {
		segmentSize = server.WALSegmentSize
	}
	segments, err := pgwal.SegmentRange(catalog.WALStart, catalog.WALEnd, segmentSize, 20000)
	if err != nil {
		return nil
	}
	records := make([]barmanWALCatalogRecord, 0, len(segments))
	for _, segment := range segments {
		records = append(records, buildBarmanWALRecord(server, catalog, segment.Name))
	}
	return records
}

func buildBarmanWALRecord(server *DatabaseBarmanServer, catalog barmanBackupCatalogRecord, fileName string) barmanWALCatalogRecord {
	fileName = strings.ToUpper(strings.TrimSpace(fileName))
	if fileName == "" || server == nil {
		return barmanWALCatalogRecord{}
	}
	segmentSize := catalog.WALSegmentSize
	if segmentSize <= 0 {
		segmentSize = server.WALSegmentSize
	}
	if segmentSize <= 0 {
		segmentSize = pgwal.DefaultSegmentSize
	}
	now := time.Now()
	firstTime := firstNonNilTime(catalog.StartedAt, catalog.FinishedAt, &now)
	lastTime := firstNonNilTime(catalog.FinishedAt, catalog.StartedAt, &now)
	record := barmanWALCatalogRecord{
		FileName:           fileName,
		StorageURI:         trimText(fmt.Sprintf("barman://%s/wal/%s", server.BarmanServerName, fileName), 1000),
		FirstEventTime:     firstTime,
		LastEventTime:      lastTime,
		Status:             DatabaseLogArchiveStatusArchived,
		ArchivedAt:         &now,
		PGSystemIdentifier: trimText(firstNonEmpty(catalog.PGSystemIdentifier, server.PGSystemIdentifier), 120),
		WALSegmentSize:     segmentSize,
		ExternalServerName: trimText(server.BarmanServerName, 120),
		StartLSN:           trimText(catalog.StartLSN, 120),
		EndLSN:             trimText(catalog.EndLSN, 120),
	}
	if pgwal.IsTimelineHistoryFile(fileName) {
		record.TimelineID = pgwal.TimelineFromHistoryFile(fileName)
		record.TimelineHistoryURI = record.StorageURI
		return record
	}
	segment, err := pgwal.ParseSegmentName(fileName, segmentSize)
	if err != nil {
		return barmanWALCatalogRecord{}
	}
	record.TimelineID = segment.TimelineID
	record.SegmentNo = strconv.FormatUint(segment.SegmentNo, 10)
	return record
}

func classifyBarmanWALCatalog(records []barmanWALCatalogRecord) (string, string) {
	if len(records) == 0 {
		return DatabaseLogChainStatusMissingWAL, ""
	}
	historyByTimeline := make(map[string]struct{})
	segmentsByTimeline := make(map[string][]uint64)
	for _, record := range records {
		if pgwal.IsTimelineHistoryFile(record.FileName) {
			if timeline := pgwal.TimelineFromHistoryFile(record.FileName); timeline != "" {
				historyByTimeline[timeline] = struct{}{}
			}
			continue
		}
		if !pgwal.IsSegmentName(record.FileName) {
			continue
		}
		segmentNo, err := strconv.ParseUint(record.SegmentNo, 10, 64)
		if err != nil {
			continue
		}
		segmentsByTimeline[record.TimelineID] = append(segmentsByTimeline[record.TimelineID], segmentNo)
	}
	for _, segments := range segmentsByTimeline {
		sort.Slice(segments, func(i, j int) bool { return segments[i] < segments[j] })
		for i := 1; i < len(segments); i++ {
			if segments[i] == segments[i-1] {
				continue
			}
			if segments[i] != segments[i-1]+1 {
				return DatabaseLogChainStatusMissingWAL, timelineHistoryStatus(segmentsByTimeline, historyByTimeline)
			}
		}
	}
	return DatabaseLogChainStatusComplete, timelineHistoryStatus(segmentsByTimeline, historyByTimeline)
}

func timelineHistoryStatus(segmentsByTimeline map[string][]uint64, historyByTimeline map[string]struct{}) string {
	for timeline := range segmentsByTimeline {
		parsed, err := strconv.ParseUint(timeline, 16, 64)
		if err == nil && parsed > 1 {
			if _, ok := historyByTimeline[timeline]; !ok {
				return DatabaseLogChainStatusTimelineGap
			}
		}
	}
	return DatabaseLogChainStatusComplete
}

func extractBarmanBackupIDs(listJSON, listText string) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	add := func(value string) {
		value = strings.Trim(strings.TrimSpace(value), ",")
		if value == "" || !barmanBackupIDPattern.MatchString(value) {
			return
		}
		lower := strings.ToLower(value)
		if lower == "server" || lower == "backup" || lower == "backup_id" || lower == "name" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	for _, line := range strings.Split(listText, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		add(fields[0])
	}
	var root any
	if err := json.Unmarshal([]byte(listJSON), &root); err == nil {
		walkBarmanJSON(root, func(key string, value any) {
			normalized := normalizeBarmanKey(key)
			if normalized == "backupid" || normalized == "id" {
				if text := barmanValueToString(value); text != "" {
					add(text)
				}
			}
		})
	}
	return ids
}

func barmanJSONRoot(raw, serverName string) any {
	var root any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &root); err != nil {
		return nil
	}
	if serverName != "" {
		if obj, ok := root.(map[string]any); ok {
			if child, ok := obj[serverName]; ok {
				return child
			}
			for key, child := range obj {
				if strings.EqualFold(key, serverName) {
					return child
				}
			}
		}
	}
	return root
}

func barmanLookupString(root any, keys ...string) string {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[normalizeBarmanKey(key)] = struct{}{}
	}
	found := ""
	walkBarmanJSON(root, func(key string, value any) {
		if found != "" {
			return
		}
		if _, ok := keySet[normalizeBarmanKey(key)]; ok {
			found = barmanValueToString(value)
		}
	})
	return found
}

func barmanLookupStringByContains(root any, fragments ...string) string {
	normalizedFragments := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		normalizedFragments = append(normalizedFragments, normalizeBarmanKey(fragment))
	}
	found := ""
	walkBarmanJSON(root, func(key string, value any) {
		if found != "" {
			return
		}
		normalized := normalizeBarmanKey(key)
		for _, fragment := range normalizedFragments {
			if !strings.Contains(normalized, fragment) {
				return
			}
		}
		found = barmanValueToString(value)
	})
	return found
}

func barmanLookupBool(root any, keys ...string) bool {
	value := strings.ToLower(barmanLookupString(root, keys...))
	switch value {
	case "true", "on", "yes", "enabled", "1", "active":
		return true
	default:
		return false
	}
}

func barmanLookupExists(root any, keys ...string) bool {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[normalizeBarmanKey(key)] = struct{}{}
	}
	found := false
	walkBarmanJSON(root, func(key string, value any) {
		if found {
			return
		}
		if _, ok := keySet[normalizeBarmanKey(key)]; ok {
			found = value != nil
		}
	})
	return found
}

func barmanLookupInt64(root any, keys ...string) int64 {
	value := barmanLookupString(root, keys...)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err == nil {
		return parsed
	}
	return parseBarmanSize(value)
}

func barmanLookupSize(root any, keys ...string) int64 {
	value := barmanLookupString(root, keys...)
	return parseBarmanSize(value)
}

func barmanLookupTime(root any, keys ...string) *time.Time {
	value := barmanLookupString(root, keys...)
	if value == "" {
		return nil
	}
	parsed, _ := parseDatabaseTime(value)
	return parsed
}

func walkBarmanJSON(root any, visit func(key string, value any)) {
	switch item := root.(type) {
	case map[string]any:
		for key, value := range item {
			visit(key, value)
			walkBarmanJSON(value, visit)
		}
	case []any:
		for _, value := range item {
			walkBarmanJSON(value, visit)
		}
	}
}

func barmanValueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		data, _ := json.Marshal(typed)
		return strings.TrimSpace(string(data))
	}
}

func normalizeBarmanKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "")
	return replacer.Replace(value)
}

func normalizeBarmanServerListRequest(req *DatabaseBarmanServerListRequest) {
	if req == nil {
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.Status = normalizeBarmanServerStatus(req.Status)
}

func normalizeBarmanServerStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseBarmanServerStatusHealthy:
		return DatabaseBarmanServerStatusHealthy
	case DatabaseBarmanServerStatusDegraded:
		return DatabaseBarmanServerStatusDegraded
	case DatabaseBarmanServerStatusFailed:
		return DatabaseBarmanServerStatusFailed
	case DatabaseBarmanServerStatusDisabled:
		return DatabaseBarmanServerStatusDisabled
	case DatabaseBarmanServerStatusPending:
		return DatabaseBarmanServerStatusPending
	default:
		return ""
	}
}

func BarmanServerStatusText(value string) string {
	switch normalizeBarmanServerStatus(value) {
	case DatabaseBarmanServerStatusHealthy:
		return "健康"
	case DatabaseBarmanServerStatusDegraded:
		return "降级"
	case DatabaseBarmanServerStatusFailed:
		return "失败"
	case DatabaseBarmanServerStatusDisabled:
		return "已禁用"
	default:
		return "待检测"
	}
}

func normalizeBarmanBackupLevel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case DatabaseBackupLevelIncremental:
		return DatabaseBackupLevelIncremental
	case DatabaseBackupLevelDifferential, "diff":
		return DatabaseBackupLevelDifferential
	default:
		return DatabaseBackupLevelFull
	}
}

func normalizeBarmanBackupStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "failed", "error", "failure":
		return DatabaseBackupStatusFailed
	case "running", "started", "waiting":
		return DatabaseBackupStatusRunning
	default:
		return DatabaseBackupStatusSuccess
	}
}

func parseBarmanSize(value string) int64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0
	}
	fields := strings.Fields(value)
	numberText := value
	unit := ""
	if len(fields) > 0 {
		numberText = fields[0]
	}
	if len(fields) > 1 {
		unit = strings.ToLower(fields[1])
	}
	number, err := strconv.ParseFloat(numberText, 64)
	if err != nil {
		return 0
	}
	multiplier := float64(1)
	switch strings.TrimSuffix(unit, "s") {
	case "kb", "kib", "k":
		multiplier = 1024
	case "mb", "mib", "m":
		multiplier = 1024 * 1024
	case "gb", "gib", "g":
		multiplier = 1024 * 1024 * 1024
	case "tb", "tib", "t":
		multiplier = 1024 * 1024 * 1024 * 1024
	}
	return int64(number * multiplier)
}

func trimBarmanVersion(value string) string {
	value = strings.TrimSpace(value)
	lines := strings.Split(value, "\n")
	if len(lines) > 0 {
		value = strings.TrimSpace(lines[0])
	}
	return trimText(value, 120)
}

func barmanRunnerRequestJSON(server *DatabaseBarmanServer, host *DatabaseRunnerHost, operator QueryOperator, command string) string {
	return barmanRunnerRequestJSONWithExtra(server, host, operator, command, nil)
}

func barmanRunnerRequestJSONWithExtra(server *DatabaseBarmanServer, host *DatabaseRunnerHost, operator QueryOperator, command string, extra map[string]any) string {
	payload := map[string]any{
		"barmanServerId":   server.ID,
		"barmanServerName": server.BarmanServerName,
		"sourceInstanceId": server.SourceInstanceID,
		"runnerHostId":     host.ID,
		"runnerId":         runnerIDForHost(host),
		"allowedCommand":   command,
		"operatorId":       operator.ID,
		"operatorName":     operator.Username,
	}
	for key, value := range extra {
		payload[key] = value
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func (uc *UseCase) toBarmanServerVO(ctx context.Context, item *DatabaseBarmanServer) *DatabaseBarmanServerVO {
	if item == nil {
		return nil
	}
	sourceInstanceName := ""
	if uc != nil && uc.instanceRepo != nil && item.SourceInstanceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, item.SourceInstanceID); err == nil && instance != nil {
			sourceInstanceName = instance.Name
		}
	}
	runnerHostName := ""
	if uc != nil && uc.runnerHostRepo != nil && item.RunnerHostID > 0 {
		if host, err := uc.runnerHostRepo.GetByID(ctx, item.RunnerHostID); err == nil && host != nil {
			runnerHostName = host.Name
		}
	}
	return &DatabaseBarmanServerVO{
		ID:                       item.ID,
		SourceInstanceID:         item.SourceInstanceID,
		SourceInstanceName:       sourceInstanceName,
		RunnerHostID:             item.RunnerHostID,
		RunnerHostName:           runnerHostName,
		Name:                     item.Name,
		BarmanServerName:         item.BarmanServerName,
		BarmanHome:               item.BarmanHome,
		ConfigPath:               item.ConfigPath,
		RetentionPolicy:          item.RetentionPolicy,
		BackupMethod:             item.BackupMethod,
		StreamingArchiverEnabled: item.StreamingArchiverEnabled,
		ArchiverEnabled:          item.ArchiverEnabled,
		SlotName:                 item.SlotName,
		BarmanVersion:            item.BarmanVersion,
		PGVersion:                item.PGVersion,
		PGSystemIdentifier:       item.PGSystemIdentifier,
		WALSegmentSize:           item.WALSegmentSize,
		Status:                   normalizeBarmanServerStatus(item.Status),
		StatusText:               BarmanServerStatusText(item.Status),
		LastCheckAt:              formatTime(item.LastCheckAt),
		LastCheckStatus:          item.LastCheckStatus,
		LastCheckStatusText:      BarmanServerStatusText(item.LastCheckStatus),
		LastCatalogSyncAt:        formatTime(item.LastCatalogSyncAt),
		LastWALSyncAt:            formatTime(item.LastWALSyncAt),
		LastError:                item.LastError,
		ConfigJSON:               item.ConfigJSON,
		CreatedAt:                item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:                item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
