package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var postgresBarmanWizardNameCleaner = regexp.MustCompile(`[^A-Za-z0-9_.:-]+`)

type DatabasePostgresBarmanPITRWizardRequest struct {
	InstanceID               uint   `json:"instanceId" binding:"required"`
	RunnerHostID             uint   `json:"runnerHostId" binding:"required"`
	ReuseBarmanServerID      uint   `json:"reuseBarmanServerId"`
	Name                     string `json:"name" binding:"omitempty,max=120"`
	BarmanServerName         string `json:"barmanServerName" binding:"omitempty,max=120"`
	BarmanHome               string `json:"barmanHome" binding:"omitempty,max=500"`
	ConfigPath               string `json:"configPath" binding:"omitempty,max=500"`
	RetentionPolicy          string `json:"retentionPolicy" binding:"omitempty,max=255"`
	BackupMethod             string `json:"backupMethod" binding:"omitempty,max=60"`
	StreamingArchiverEnabled *bool  `json:"streamingArchiverEnabled"`
	ArchiverEnabled          *bool  `json:"archiverEnabled"`
	SlotName                 string `json:"slotName" binding:"omitempty,max=120"`
	ConfigJSON               string `json:"configJson" binding:"omitempty,max=4000"`
	RunCheckNow              *bool  `json:"runCheckNow"`
	SyncCatalogNow           *bool  `json:"syncCatalogNow"`
	SyncWALNow               *bool  `json:"syncWalNow"`
	RunInitialBackupNow      *bool  `json:"runInitialBackupNow"`
}

type DatabasePostgresBarmanPITRWizardResult struct {
	TemplateKey      string                             `json:"templateKey"`
	TemplateName     string                             `json:"templateName"`
	InstanceID       uint                               `json:"instanceId"`
	InstanceName     string                             `json:"instanceName"`
	Engine           string                             `json:"engine"`
	EngineText       string                             `json:"engineText"`
	Version          string                             `json:"version"`
	RunnerHostID     uint                               `json:"runnerHostId"`
	RunnerHostName   string                             `json:"runnerHostName"`
	BarmanServerName string                             `json:"barmanServerName"`
	CanApply         bool                               `json:"canApply"`
	BlockingReasons  []string                           `json:"blockingReasons"`
	Warnings         []string                           `json:"warnings"`
	Messages         []string                           `json:"messages"`
	Actions          []DatabaseProtectionWizardActionVO `json:"actions"`
	BarmanServer     *DatabaseBarmanServerVO            `json:"barmanServer,omitempty"`
	LogArchiveStream *DatabaseLogArchiveStreamVO        `json:"logArchiveStream,omitempty"`
	CheckJob         *DatabaseRunnerJobVO               `json:"checkJob,omitempty"`
	CatalogSyncJob   *DatabaseRunnerJobVO               `json:"catalogSyncJob,omitempty"`
	WALSyncJob       *DatabaseRunnerJobVO               `json:"walSyncJob,omitempty"`
	InitialBackupJob *DatabaseRunnerJobVO               `json:"initialBackupJob,omitempty"`
	Profile          *DatabaseProtectionProfileVO       `json:"profile,omitempty"`
}

type postgresBarmanPITRWizardResolved struct {
	Req                 *DatabasePostgresBarmanPITRWizardRequest
	Instance            *DatabaseInstance
	Runner              *DatabaseRunnerHost
	ExistingServer      *DatabaseBarmanServer
	ExistingStream      *DatabaseLogArchiveStream
	Name                string
	BarmanServerName    string
	RunCheckNow         bool
	SyncCatalogNow      bool
	SyncWALNow          bool
	RunInitialBackupNow bool
	BlockingReasons     []string
	Warnings            []string
	Actions             []DatabaseProtectionWizardActionVO
}

func (uc *UseCase) PreviewPostgresBarmanPITRWizard(ctx context.Context, req *DatabasePostgresBarmanPITRWizardRequest) (*DatabasePostgresBarmanPITRWizardResult, error) {
	resolved, err := uc.resolvePostgresBarmanPITRWizard(ctx, req)
	if err != nil {
		return nil, err
	}
	return uc.postgresBarmanPITRWizardResult(ctx, resolved, nil, nil, nil, nil, nil, nil), nil
}

func (uc *UseCase) ApplyPostgresBarmanPITRWizard(ctx context.Context, req *DatabasePostgresBarmanPITRWizardRequest, operator QueryOperator) (*DatabasePostgresBarmanPITRWizardResult, error) {
	resolved, err := uc.resolvePostgresBarmanPITRWizard(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resolved.BlockingReasons) > 0 {
		return nil, fmt.Errorf("PostgreSQL Barman 保护向导预校验未通过: %s", strings.Join(resolved.BlockingReasons, "；"))
	}
	serverVO, err := uc.applyPostgresBarmanWizardServer(ctx, resolved)
	if err != nil {
		return nil, err
	}
	serverID := serverVO.ID
	var checkJob, catalogJob, walJob, backupJob *DatabaseRunnerJobVO
	if resolved.RunCheckNow {
		if job, jobErr := uc.CheckBarmanServer(ctx, serverID, operator); jobErr != nil {
			resolved.Warnings = append(resolved.Warnings, "Barman check 下发失败: "+jobErr.Error())
		} else {
			checkJob = job
			resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_server", serverID, resolved.Name, DatabaseProtectionWizardStatusDone, "已下发 Barman check Runner Job", false))
		}
	}
	if resolved.SyncCatalogNow {
		if job, jobErr := uc.SyncBarmanCatalog(ctx, serverID, operator); jobErr != nil {
			resolved.Warnings = append(resolved.Warnings, "Barman catalog 同步下发失败: "+jobErr.Error())
		} else {
			catalogJob = job
			resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_catalog", serverID, resolved.Name, DatabaseProtectionWizardStatusDone, "已下发 Barman catalog sync Runner Job", false))
		}
	}
	var streamVO *DatabaseLogArchiveStreamVO
	if resolved.SyncWALNow {
		if job, jobErr := uc.SyncBarmanWAL(ctx, serverID, operator); jobErr != nil {
			resolved.Warnings = append(resolved.Warnings, "Barman WAL 同步下发失败: "+jobErr.Error())
		} else {
			walJob = job
			resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "wal_archive_stream", 0, resolved.BarmanServerName, DatabaseProtectionWizardStatusDone, "已下发 Barman WAL sync Runner Job", false))
			if stream := uc.findPostgresBarmanWizardStream(ctx, resolved.Instance.ID, serverID); stream != nil {
				streamVO = uc.toLogArchiveStreamVO(ctx, stream)
			}
		}
	}
	if streamVO == nil {
		if stream := uc.findPostgresBarmanWizardStream(ctx, resolved.Instance.ID, serverID); stream != nil {
			streamVO = uc.toLogArchiveStreamVO(ctx, stream)
		}
	}
	if resolved.RunInitialBackupNow {
		if job, jobErr := uc.BackupBarmanServer(ctx, serverID, operator); jobErr != nil {
			resolved.Warnings = append(resolved.Warnings, "Barman backup 下发失败: "+jobErr.Error())
		} else {
			backupJob = job
			resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_backup", serverID, resolved.Name, DatabaseProtectionWizardStatusDone, "已下发 Barman cluster 级 backup Runner Job", false))
		}
	}
	result := uc.postgresBarmanPITRWizardResult(ctx, resolved, serverVO, streamVO, checkJob, catalogJob, walJob, backupJob)
	result.Profile, _ = uc.GetProtectionProfile(ctx, fmt.Sprintf("%s%d", protectionProfileIDPrefix, resolved.Instance.ID))
	return result, nil
}

func (uc *UseCase) resolvePostgresBarmanPITRWizard(ctx context.Context, req *DatabasePostgresBarmanPITRWizardRequest) (*postgresBarmanPITRWizardResolved, error) {
	if req == nil || req.InstanceID == 0 {
		return nil, fmt.Errorf("请选择 PostgreSQL 实例")
	}
	if uc.instanceRepo == nil || uc.runnerHostRepo == nil || uc.barmanServerRepo == nil {
		return nil, fmt.Errorf("PostgreSQL Barman 向导依赖仓库未配置")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("PostgreSQL 实例不存在")
	}
	if normalizeDBType(instance.DBType) != DBTypePostgreSQL {
		return nil, fmt.Errorf("PostgreSQL Barman 向导不支持 %s", DBTypeText(instance.DBType))
	}
	runner, err := uc.runnerHostRepo.GetByID(ctx, req.RunnerHostID)
	if err != nil || runner == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	resolved := &postgresBarmanPITRWizardResolved{
		Req:                 req,
		Instance:            instance,
		Runner:              runner,
		Name:                postgresBarmanWizardDisplayName(req, instance),
		BarmanServerName:    postgresBarmanWizardServerName(req, instance),
		RunCheckNow:         boolDefault(req.RunCheckNow, true),
		SyncCatalogNow:      boolDefault(req.SyncCatalogNow, true),
		SyncWALNow:          boolDefault(req.SyncWALNow, true),
		RunInitialBackupNow: boolDefault(req.RunInitialBackupNow, false),
		BlockingReasons:     []string{},
		Warnings:            []string{},
		Actions:             []DatabaseProtectionWizardActionVO{},
	}
	if err := validateBarmanRunnerHost(runner); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	} else if runner.Status != DatabaseRunnerHostStatusOnline {
		resolved.Warnings = append(resolved.Warnings, "Runner 主机未确认在线，apply 后可能无法立即执行 Barman 命令")
	}
	if err := validateBackupStorageConfigSafe(req.ConfigJSON); err != nil {
		resolved.BlockingReasons = append(resolved.BlockingReasons, err.Error())
	}
	if !barmanServerNamePattern.MatchString(resolved.BarmanServerName) {
		resolved.BlockingReasons = append(resolved.BlockingReasons, "Barman server name 只能包含字母、数字、下划线、点、冒号和短横线")
	}
	resolved.ExistingServer = uc.findPostgresBarmanWizardServer(ctx, req)
	if resolved.ExistingServer == nil {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionCreate, "barman_server", 0, resolved.Name, DatabaseProtectionWizardStatusPlanned, "登记 PostgreSQL Barman Server", false))
	} else {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionUpdate, "barman_server", resolved.ExistingServer.ID, resolved.ExistingServer.Name, DatabaseProtectionWizardStatusPlanned, "复用并更新 Barman Server 配置", false))
		if resolved.ExistingServer.Status == DatabaseBarmanServerStatusFailed {
			resolved.Warnings = append(resolved.Warnings, "已有 Barman Server 当前为失败状态，建议先执行 check")
		}
	}
	if resolved.RunCheckNow {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_check", idFromBarmanServer(resolved.ExistingServer), resolved.Name, DatabaseProtectionWizardStatusPlanned, "执行 Barman check", false))
	}
	if resolved.SyncCatalogNow {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_catalog", idFromBarmanServer(resolved.ExistingServer), resolved.Name, DatabaseProtectionWizardStatusPlanned, "同步 Barman catalog 到备份记录", false))
	}
	if resolved.SyncWALNow {
		resolved.ExistingStream = uc.findPostgresBarmanWizardStream(ctx, instance.ID, idFromBarmanServer(resolved.ExistingServer))
		action := DatabaseProtectionWizardActionCreate
		message := "创建或复用 WAL 归档流并同步 WAL 状态"
		streamID := uint(0)
		if resolved.ExistingStream != nil {
			action = DatabaseProtectionWizardActionReuse
			streamID = resolved.ExistingStream.ID
			message = "复用 WAL 归档流并同步 WAL 状态"
		}
		resolved.Actions = append(resolved.Actions, wizardAction(action, "wal_archive_stream", streamID, resolved.BarmanServerName, DatabaseProtectionWizardStatusPlanned, message, false))
	}
	if resolved.RunInitialBackupNow {
		resolved.Actions = append(resolved.Actions, wizardAction(DatabaseProtectionWizardActionRun, "barman_backup", idFromBarmanServer(resolved.ExistingServer), resolved.Name, DatabaseProtectionWizardStatusPlanned, "立即下发一次 Barman cluster 级 full backup", false))
	}
	return resolved, nil
}

func (uc *UseCase) applyPostgresBarmanWizardServer(ctx context.Context, resolved *postgresBarmanPITRWizardResolved) (*DatabaseBarmanServerVO, error) {
	req := resolved.Req
	payload := &DatabaseBarmanServerRequest{
		SourceInstanceID:         resolved.Instance.ID,
		RunnerHostID:             resolved.Runner.ID,
		Name:                     resolved.Name,
		BarmanServerName:         resolved.BarmanServerName,
		BarmanHome:               strings.TrimSpace(req.BarmanHome),
		ConfigPath:               strings.TrimSpace(req.ConfigPath),
		RetentionPolicy:          strings.TrimSpace(req.RetentionPolicy),
		BackupMethod:             strings.TrimSpace(req.BackupMethod),
		StreamingArchiverEnabled: boolDefault(req.StreamingArchiverEnabled, true),
		ArchiverEnabled:          boolDefault(req.ArchiverEnabled, true),
		SlotName:                 strings.TrimSpace(req.SlotName),
		Status:                   DatabaseBarmanServerStatusPending,
		ConfigJSON:               strings.TrimSpace(req.ConfigJSON),
	}
	if resolved.ExistingServer == nil {
		return uc.CreateBarmanServer(ctx, payload)
	}
	payload.Status = resolved.ExistingServer.Status
	return uc.UpdateBarmanServer(ctx, resolved.ExistingServer.ID, payload)
}

func (uc *UseCase) postgresBarmanPITRWizardResult(ctx context.Context, resolved *postgresBarmanPITRWizardResolved, server *DatabaseBarmanServerVO, stream *DatabaseLogArchiveStreamVO, checkJob, catalogJob, walJob, backupJob *DatabaseRunnerJobVO) *DatabasePostgresBarmanPITRWizardResult {
	if server == nil && resolved.ExistingServer != nil {
		server = uc.toBarmanServerVO(ctx, resolved.ExistingServer)
	}
	if stream == nil && resolved.ExistingStream != nil {
		stream = uc.toLogArchiveStreamVO(ctx, resolved.ExistingStream)
	}
	messages := make([]string, 0, 4)
	if len(resolved.BlockingReasons) > 0 {
		messages = append(messages, resolved.BlockingReasons...)
	} else {
		messages = append(messages, "预检通过，可以启用 PostgreSQL Barman PITR 保护")
	}
	messages = append(messages, resolved.Warnings...)
	return &DatabasePostgresBarmanPITRWizardResult{
		TemplateKey:      "postgres_barman_pitr",
		TemplateName:     "PostgreSQL Barman PITR",
		InstanceID:       resolved.Instance.ID,
		InstanceName:     resolved.Instance.Name,
		Engine:           DBTypePostgreSQL,
		EngineText:       DBTypeText(DBTypePostgreSQL),
		Version:          resolved.Instance.Version,
		RunnerHostID:     resolved.Runner.ID,
		RunnerHostName:   resolved.Runner.Name,
		BarmanServerName: resolved.BarmanServerName,
		CanApply:         len(resolved.BlockingReasons) == 0,
		BlockingReasons:  uniqueStrings(resolved.BlockingReasons),
		Warnings:         uniqueStrings(resolved.Warnings),
		Messages:         uniqueStrings(messages),
		Actions:          resolved.Actions,
		BarmanServer:     server,
		LogArchiveStream: stream,
		CheckJob:         checkJob,
		CatalogSyncJob:   catalogJob,
		WALSyncJob:       walJob,
		InitialBackupJob: backupJob,
	}
}

func (uc *UseCase) findPostgresBarmanWizardServer(ctx context.Context, req *DatabasePostgresBarmanPITRWizardRequest) *DatabaseBarmanServer {
	if uc == nil || uc.barmanServerRepo == nil || req == nil {
		return nil
	}
	if req.ReuseBarmanServerID > 0 {
		if server, err := uc.barmanServerRepo.GetByID(ctx, req.ReuseBarmanServerID); err == nil && server != nil && server.SourceInstanceID == req.InstanceID {
			return server
		}
	}
	items, _, err := uc.barmanServerRepo.List(ctx, &DatabaseBarmanServerListRequest{Page: 1, PageSize: 100, SourceInstanceID: req.InstanceID})
	if err != nil {
		return nil
	}
	var selected *DatabaseBarmanServer
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(req.BarmanServerName) != "" && item.BarmanServerName == strings.TrimSpace(req.BarmanServerName) {
			return item
		}
		if selected == nil || barmanServerProtectionRank(item) > barmanServerProtectionRank(selected) {
			selected = item
		}
	}
	return selected
}

func (uc *UseCase) findPostgresBarmanWizardStream(ctx context.Context, instanceID, serverID uint) *DatabaseLogArchiveStream {
	if uc == nil || uc.logArchiveStreamRepo == nil || instanceID == 0 {
		return nil
	}
	items, _, err := uc.logArchiveStreamRepo.List(ctx, &DatabaseLogArchiveStreamListRequest{Page: 1, PageSize: 100, InstanceID: instanceID, ArchiveType: DatabaseArchiveTypeWAL})
	if err != nil {
		return nil
	}
	var selected *DatabaseLogArchiveStream
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.ArchiveEngine) != "barman" {
			continue
		}
		if serverID > 0 && strings.Contains(item.ConfigJSON, fmt.Sprintf(`"barmanServerId":%d`, serverID)) {
			return item
		}
		if selected == nil {
			selected = item
		}
	}
	return selected
}

func postgresBarmanWizardDisplayName(req *DatabasePostgresBarmanPITRWizardRequest, instance *DatabaseInstance) string {
	if req != nil && strings.TrimSpace(req.Name) != "" {
		return trimText(strings.TrimSpace(req.Name), 120)
	}
	if instance == nil || strings.TrimSpace(instance.Name) == "" {
		return "PostgreSQL-Barman-PITR"
	}
	return trimText(fmt.Sprintf("%s-Barman-PITR保护", safeBackupName(instance.Name)), 120)
}

func postgresBarmanWizardServerName(req *DatabasePostgresBarmanPITRWizardRequest, instance *DatabaseInstance) string {
	if req != nil && strings.TrimSpace(req.BarmanServerName) != "" {
		return trimText(strings.TrimSpace(req.BarmanServerName), 120)
	}
	name := "postgres"
	if instance != nil {
		name = fmt.Sprintf("pg_%d_%s", instance.ID, strings.ToLower(instance.Name))
	}
	name = postgresBarmanWizardNameCleaner.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_.:-")
	if name == "" {
		name = "postgres"
	}
	return trimText(name, 120)
}

func idFromBarmanServer(item *DatabaseBarmanServer) uint {
	if item == nil {
		return 0
	}
	return item.ID
}
