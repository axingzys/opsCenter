package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

type barmanBackupShowOutput struct {
	BackupID string
	ShowJSON string
	ShowErr  string
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
	status := normalizeBackupRecordStatus(catalog.Status)
	record.InstanceID = server.SourceInstanceID
	record.TriggerType = DatabaseBackupTriggerExternal
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
	if created {
		return true, uc.backupRecordRepo.Create(ctx, record)
	}
	return false, uc.backupRecordRepo.Update(ctx, record)
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
