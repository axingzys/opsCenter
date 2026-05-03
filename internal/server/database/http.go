package database

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	systembiz "github.com/ydcloud-dy/opshub/internal/biz/system"
	assetdata "github.com/ydcloud-dy/opshub/internal/data/asset"
	dbdata "github.com/ydcloud-dy/opshub/internal/data/database"
	systemdata "github.com/ydcloud-dy/opshub/internal/data/system"
	dbservice "github.com/ydcloud-dy/opshub/internal/service/database"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
	"gorm.io/gorm"
)

const (
	permDatabaseInstanceView    = "database:instance:view"
	permDatabaseInstanceCreate  = "database:instance:create"
	permDatabaseInstanceUpdate  = "database:instance:update"
	permDatabaseInstanceDelete  = "database:instance:delete"
	permDatabaseInstanceStatus  = "database:instance:status"
	permDatabaseConnectionTest  = "database:connection:test"
	permDatabaseMetadataView    = "database:metadata:view"
	permDatabaseMetadataSync    = "database:metadata:sync"
	permDatabaseMetadataExport  = "database:metadata:export"
	permDatabaseQueryExecute    = "database:query:execute"
	permDatabaseQueryWrite      = "database:query:write"
	permDatabaseQueryExplain    = "database:query:explain"
	permDatabaseQueryWritePlan  = "database:query:write-explain"
	permDatabaseQueryDDL        = "database:query:ddl"
	permDatabaseQueryExport     = "database:query:export"
	permDatabaseQueryHistory    = "database:query:history:view"
	permDatabaseDiagnosisView   = "database:diagnosis:view"
	permDatabaseTopologyView    = "database:topology:view"
	permDatabaseAuditView       = "database:audit:view"
	permDatabaseAuditExport     = "database:audit:export"
	permDatabaseBackupView      = "database:backup:view"
	permDatabaseBackupCreate    = "database:backup:create"
	permDatabaseBackupUpdate    = "database:backup:update"
	permDatabaseBackupDelete    = "database:backup:delete"
	permDatabaseBackupRun       = "database:backup:run"
	permDatabaseBackupDownload  = "database:backup:download"
	permDatabaseRestoreView     = "database:restore:view"
	permDatabaseRestoreRun      = "database:restore:run"
	permDatabaseCapacityView    = "database:capacity:view"
	permDatabaseCapacityCollect = "database:capacity:collect"
	permDatabaseInspectionView  = "database:inspection:view"
	permDatabaseInspectionRun   = "database:inspection:run"
	permDatabaseReplicaView     = "database:replica:view"
	permDatabaseReplicaCheck    = "database:replica:check"
	permDatabaseReplicaIncident = "database:replica:incident-guide"
	permDatabaseReplicaPause    = "database:replica:pause-apply"
	permDatabaseReplicaResume   = "database:replica:resume-apply"
)

var databaseUIPermissionCodes = map[string]string{
	"instanceCreate":             permDatabaseInstanceCreate,
	"instanceUpdate":             permDatabaseInstanceUpdate,
	"instanceDelete":             permDatabaseInstanceDelete,
	"instanceStatus":             permDatabaseInstanceStatus,
	"connectionTest":             permDatabaseConnectionTest,
	"metadataSync":               permDatabaseMetadataSync,
	"metadataExport":             permDatabaseMetadataExport,
	"queryExecute":               permDatabaseQueryExecute,
	"queryWrite":                 permDatabaseQueryWrite,
	"queryExplain":               permDatabaseQueryExplain,
	"queryWriteExplain":          permDatabaseQueryWritePlan,
	"queryDDL":                   permDatabaseQueryDDL,
	"queryExport":                permDatabaseQueryExport,
	"auditExport":                permDatabaseAuditExport,
	"backupCreate":               permDatabaseBackupCreate,
	"backupUpdate":               permDatabaseBackupUpdate,
	"backupDelete":               permDatabaseBackupDelete,
	"backupRun":                  permDatabaseBackupRun,
	"backupDownload":             permDatabaseBackupDownload,
	"restoreRun":                 permDatabaseRestoreRun,
	"capacityCollect":            permDatabaseCapacityCollect,
	"inspectionRun":              permDatabaseInspectionRun,
	"replicaView":                permDatabaseReplicaView,
	"replicaCheck":               permDatabaseReplicaCheck,
	"replicaIncidentGuide":       permDatabaseReplicaIncident,
	"replicaPauseApply":          permDatabaseReplicaPause,
	"replicaResumeApply":         permDatabaseReplicaResume,
	"instancePermissionView":     permDatabaseInstanceView,
	"instancePermissionManage":   permDatabaseInstanceUpdate,
	"instanceObjectPermissionUI": permDatabaseInstanceUpdate,
}

type HTTPServer struct {
	service           *dbservice.Service
	backupScheduler   *dbbiz.BackupScheduler
	capacityScheduler *dbbiz.CapacityScheduler
	authMiddleware    *rbacservice.AuthMiddleware
}

func NewHTTPServer(db *gorm.DB, authMiddleware *rbacservice.AuthMiddleware) *HTTPServer {
	instanceRepo := dbdata.NewInstanceRepo(db)
	permissionRepo := dbdata.NewDatabasePermissionRepo(db)
	schemaRepo := dbdata.NewSchemaRepo(db)
	tableRepo := dbdata.NewTableRepo(db)
	columnRepo := dbdata.NewColumnRepo(db)
	indexRepo := dbdata.NewIndexRepo(db)
	metadataRepo := dbdata.NewMetadataRepo(db)
	redisMetadataRepo := dbdata.NewRedisMetadataRepo(db)
	syncJobRepo := dbdata.NewSyncJobRepo(db)
	auditRepo := dbdata.NewQueryAuditRepo(db)
	backupTaskRepo := dbdata.NewBackupTaskRepo(db)
	backupRecordRepo := dbdata.NewBackupRecordRepo(db)
	backupPolicyConfigRepo := dbdata.NewBackupPolicyConfigRepo(db)
	backupChainStateRepo := dbdata.NewBackupChainStateRepo(db)
	restoreJobRepo := dbdata.NewRestoreJobRepo(db)
	capacitySnapshotRepo := dbdata.NewCapacitySnapshotRepo(db)
	inspectionReportRepo := dbdata.NewInspectionReportRepo(db)
	logArchiveStreamRepo := dbdata.NewLogArchiveStreamRepo(db)
	logArchiveRepo := dbdata.NewLogArchiveRepo(db)
	logArchiveEventRepo := dbdata.NewLogArchiveEventRepo(db)
	restorePlanRepo := dbdata.NewRestorePlanRepo(db)
	storageProfileRepo := dbdata.NewStorageProfileRepo(db)
	secretProfileRepo := dbdata.NewSecretProfileRepo(db)
	runnerHostRepo := dbdata.NewRunnerHostRepo(db)
	runnerJobRepo := dbdata.NewRunnerJobRepo(db)
	barmanServerRepo := dbdata.NewBarmanServerRepo(db)
	instanceReplicaRepo := dbdata.NewInstanceReplicaRepo(db)
	replicationCheckRepo := dbdata.NewReplicationCheckRepo(db)
	replicaIncidentGuideRepo := dbdata.NewReplicaIncidentGuideRepo(db)
	replicaActionRepo := dbdata.NewReplicaActionRepo(db)
	credentialRepo := assetdata.NewCredentialRepo(db)
	configRepo := systemdata.NewConfigRepo(db)
	loginAttemptRepo := systemdata.NewLoginAttemptRepo(db)
	configUseCase := systembiz.NewConfigUseCase(configRepo, loginAttemptRepo)

	useCase := dbbiz.NewUseCase(
		instanceRepo,
		schemaRepo,
		tableRepo,
		columnRepo,
		indexRepo,
		metadataRepo,
		redisMetadataRepo,
		syncJobRepo,
		auditRepo,
		backupTaskRepo,
		backupRecordRepo,
		restoreJobRepo,
		capacitySnapshotRepo,
		inspectionReportRepo,
		func(ctx context.Context, id uint) error {
			_, err := credentialRepo.GetByID(ctx, id)
			return err
		},
		func(ctx context.Context, id uint) (*dbbiz.ConnectionCredential, error) {
			credential, err := credentialRepo.GetByIDDecrypted(ctx, id)
			if err != nil {
				return nil, err
			}
			return &dbbiz.ConnectionCredential{
				Username:   credential.Username,
				Password:   credential.Password,
				PrivateKey: credential.PrivateKey,
				Passphrase: credential.Passphrase,
			}, nil
		},
		func(ctx context.Context) (*dbbiz.DatabaseWritePolicy, error) {
			cfg, err := configUseCase.GetDatabaseConfig(ctx)
			if err != nil {
				return nil, err
			}
			return &dbbiz.DatabaseWritePolicy{
				WriteEnabled:            cfg.WriteEnabled,
				WriteExplainEnabled:     cfg.WriteExplainEnabled,
				DDLEnabled:              cfg.DDLEnabled,
				DDLHighRiskConfirm:      cfg.DDLHighRiskRequiresConfirm,
				DDLReasonRequired:       cfg.DDLReasonRequired,
				DDLRequireBackupHint:    cfg.DDLRequireBackupHint,
				HighRiskRequiresConfirm: cfg.HighRiskRequiresConfirm,
				OperationReasonRequired: cfg.OperationReasonRequired,
				MaxAffectedRows:         int64(cfg.MaxAffectedRows),
			}, nil
		},
		func(ctx context.Context) (*dbbiz.DatabaseBackupPolicy, error) {
			cfg, err := configUseCase.GetDatabaseConfig(ctx)
			if err != nil {
				return nil, err
			}
			return &dbbiz.DatabaseBackupPolicy{
				DefaultRetentionDays: cfg.DefaultBackupRetentionDays,
				StoragePath:          cfg.BackupStoragePath,
			}, nil
		},
	)
	useCase.SetBackupGovernanceRepos(
		logArchiveStreamRepo,
		logArchiveRepo,
		logArchiveEventRepo,
		restorePlanRepo,
		storageProfileRepo,
		secretProfileRepo,
		runnerHostRepo,
		runnerJobRepo,
		barmanServerRepo,
		backupPolicyConfigRepo,
		backupChainStateRepo,
	)
	useCase.SetReplicaGovernanceRepos(instanceReplicaRepo, replicationCheckRepo, replicaIncidentGuideRepo, replicaActionRepo)

	backupScheduler := dbbiz.NewBackupScheduler(useCase, dbbiz.BackupSchedulerOptions{
		Interval:        time.Minute,
		CleanupInterval: 6 * time.Hour,
		NotifyFailure: func(ctx context.Context, notice *dbbiz.BackupSchedulerNotice) error {
			return dispatchBackupSchedulerAlert(ctx, db, notice)
		},
	})
	capacityScheduler := dbbiz.NewCapacityScheduler(useCase, dbbiz.CapacitySchedulerOptions{
		Interval: 6 * time.Hour,
	})

	return &HTTPServer{
		service: dbservice.NewService(useCase, permissionRepo, func(ctx context.Context) (string, error) {
			cfg, err := configUseCase.GetDatabaseConfig(ctx)
			if err != nil {
				return "", err
			}
			return cfg.InstancePermissionMode, nil
		}),
		backupScheduler:   backupScheduler,
		capacityScheduler: capacityScheduler,
		authMiddleware:    authMiddleware,
	}
}

func (s *HTTPServer) StartBackground(ctx context.Context) {
	if s == nil {
		return
	}
	if s.backupScheduler != nil {
		s.backupScheduler.Start(ctx)
	}
	if s.capacityScheduler != nil {
		s.capacityScheduler.Start(ctx)
	}
}

func (s *HTTPServer) StopBackground(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.backupScheduler != nil {
		if err := s.backupScheduler.Stop(ctx); err != nil {
			return err
		}
	}
	if s.capacityScheduler != nil {
		if err := s.capacityScheduler.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *HTTPServer) GetUIPermissions(c *gin.Context) {
	userID := rbacservice.GetUserID(c)
	if userID == 0 {
		response.ErrorCode(c, http.StatusUnauthorized, "未登录")
		return
	}
	result := gin.H{}
	for key, code := range databaseUIPermissionCodes {
		ok, err := s.authMiddleware.HasMenuPermission(c.Request.Context(), userID, code)
		if err != nil {
			response.ErrorCode(c, http.StatusInternalServerError, "权限检查失败")
			return
		}
		result[key] = ok
	}
	response.Success(c, result)
}

func (s *HTTPServer) RegisterRoutes(r *gin.RouterGroup) {
	databases := r.Group("/databases")
	{
		databases.GET("/supported-types", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceView), s.service.GetSupportedTypes)
		databases.GET("/ui-permissions", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceView), s.GetUIPermissions)
		databases.GET("/instance-permissions", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceView), s.service.ListInstancePermissions)
		databases.POST("/instance-permissions", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceUpdate), s.service.UpsertInstancePermission)
		databases.DELETE("/instance-permissions/:id", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceUpdate), s.service.DeleteInstancePermission)
		databases.GET("/query-history", s.authMiddleware.RequireMenuPermission(permDatabaseQueryHistory), s.service.ListQueryHistory)
		databases.GET("/query-audits", s.authMiddleware.RequireMenuPermission(permDatabaseAuditView), s.service.ListQueryAudits)
		databases.GET("/query-audits/export", s.authMiddleware.RequireMenuPermission(permDatabaseAuditExport), s.service.ExportQueryAudits)
		databases.GET("/backup-tasks", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListBackupTasks)
		databases.POST("/backup-tasks", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateBackupTask)
		databases.PUT("/backup-tasks/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupUpdate), s.service.UpdateBackupTask)
		databases.DELETE("/backup-tasks/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupDelete), s.service.DeleteBackupTask)
		databases.POST("/backup-tasks/:id/run", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunBackupTask)
		databases.GET("/backup-policies", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListBackupPolicies)
		databases.POST("/backup-policies", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateBackupPolicy)
		databases.PUT("/backup-policies/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupUpdate), s.service.UpdateBackupPolicy)
		databases.DELETE("/backup-policies/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupDelete), s.service.DeleteBackupPolicy)
		databases.GET("/backup-policies/:id/chain", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.GetBackupPolicyChain)
		databases.POST("/backup-policies/:id/validate-chain", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.ValidateBackupPolicyChain)
		databases.POST("/backup-policies/:id/synthetic-full/preview", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.PreviewBackupPolicySyntheticFull)
		databases.POST("/backup-policies/:id/synthetic-full/run", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunBackupPolicySyntheticFull)
		databases.GET("/backup-policies/:id/synthetic-full/jobs", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListBackupPolicySyntheticJobs)
		databases.POST("/backup-policies/:id/purge-preview", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.PreviewBackupPolicyPurge)
		databases.POST("/backup-policies/:id/purge", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunBackupPolicyPurge)
		databases.POST("/backup-policies/:id/run-full", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunBackupPolicyFull)
		databases.POST("/backup-policies/:id/run-incremental", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunBackupPolicyIncremental)
		databases.GET("/backup-records", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListBackupRecords)
		databases.POST("/backup-records/external", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.RegisterExternalBackupRecord)
		databases.GET("/backup-records/:id/download", s.authMiddleware.RequireMenuPermission(permDatabaseBackupDownload), s.service.DownloadBackupRecord)
		databases.POST("/backup-records/:id/verify", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.VerifyBackupRecord)
		databases.POST("/backup-records/:id/restore-dry-run", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreRun), s.service.RunRestoreDryRun)
		databases.GET("/restore-jobs", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreView), s.service.ListRestoreJobs)
		databases.GET("/restore-jobs/:id", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreView), s.service.GetRestoreJob)
		databases.POST("/restore-jobs/:id/cancel", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreRun), s.service.CancelRestoreJob)
		databases.POST("/restore-jobs/:id/cleanup", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreRun), s.service.CleanupRestoreJob)
		databases.GET("/restore-jobs/:id/proof", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreView), s.service.GetRestoreJobProof)
		databases.GET("/restore-plans", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreView), s.service.ListRestorePlans)
		databases.POST("/restore-plans", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreRun), s.service.CreateRestorePlan)
		databases.POST("/restore-plans/:id/run", s.authMiddleware.RequireMenuPermission(permDatabaseRestoreRun), s.service.RunRestorePlan)
		databases.GET("/log-archive-streams", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListLogArchiveStreams)
		databases.POST("/log-archive-streams", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateLogArchiveStream)
		databases.GET("/log-archive-streams/:id/status", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.GetLogArchiveStreamStatus)
		databases.POST("/log-archive-streams/:id/start", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.StartLogArchiveStream)
		databases.POST("/log-archive-streams/:id/pause", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.PauseLogArchiveStream)
		databases.POST("/log-archive-streams/:id/resume", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.ResumeLogArchiveStream)
		databases.POST("/log-archive-streams/:id/stop", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.StopLogArchiveStream)
		databases.POST("/log-archive-streams/:id/run-once", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunLogArchiveOnce)
		databases.POST("/log-archive-streams/:id/catch-up", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.RunLogArchiveCatchUp)
		databases.GET("/log-archives", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListLogArchives)
		databases.POST("/log-archives/external", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.RegisterExternalLogArchive)
		databases.GET("/log-archive-events", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListLogArchiveEvents)
		databases.GET("/storage-profiles", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListStorageProfiles)
		databases.POST("/storage-profiles", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateStorageProfile)
		databases.POST("/storage-profiles/:id/posture-check", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.CheckStorageProfilePosture)
		databases.GET("/secret-profiles", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListSecretProfiles)
		databases.POST("/secret-profiles", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateSecretProfile)
		databases.GET("/runner-hosts", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListRunnerHosts)
		databases.POST("/runner-hosts", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateRunnerHost)
		databases.PUT("/runner-hosts/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupUpdate), s.service.UpdateRunnerHost)
		databases.POST("/runner-hosts/:id/test", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.TestRunnerHost)
		databases.GET("/runner-jobs", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListRunnerJobs)
		databases.GET("/barman-servers", s.authMiddleware.RequireMenuPermission(permDatabaseBackupView), s.service.ListBarmanServers)
		databases.POST("/barman-servers", s.authMiddleware.RequireMenuPermission(permDatabaseBackupCreate), s.service.CreateBarmanServer)
		databases.PUT("/barman-servers/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupUpdate), s.service.UpdateBarmanServer)
		databases.DELETE("/barman-servers/:id", s.authMiddleware.RequireMenuPermission(permDatabaseBackupDelete), s.service.DeleteBarmanServer)
		databases.POST("/barman-servers/:id/check", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.CheckBarmanServer)
		databases.POST("/barman-servers/:id/sync-catalog", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.SyncBarmanCatalog)
		databases.POST("/barman-servers/:id/sync-wal", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.SyncBarmanWAL)
		databases.POST("/barman-servers/:id/backup", s.authMiddleware.RequireMenuPermission(permDatabaseBackupRun), s.service.BackupBarmanServer)
		databases.GET("/inspection-reports", s.authMiddleware.RequireMenuPermission(permDatabaseInspectionView), s.service.ListInspectionReports)
		databases.POST("/inspection-reports", s.authMiddleware.RequireMenuPermission(permDatabaseInspectionRun), s.service.GenerateInspectionReport)
		databases.GET("/inspection-reports/:id", s.authMiddleware.RequireMenuPermission(permDatabaseInspectionView), s.service.GetInspectionReport)
		databases.GET("/replica-protections", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListReplicaProtections)
		databases.POST("/replica-incident-guides", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaIncident), s.service.CreateReplicaIncidentGuide)
		databases.GET("/replica-incident-guides", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListReplicaIncidentGuides)
		databases.GET("/replica-incident-guides/:id", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.GetReplicaIncidentGuide)
		databases.GET("/replica-actions", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListReplicaActions)
		databases.GET("/replicas", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListReplicas)
		databases.POST("/replicas/:id/pause-apply", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaPause), s.service.PauseReplicaApply)
		databases.POST("/replicas/:id/resume-apply", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaResume), s.service.ResumeReplicaApply)
		databases.GET("/replication-checks", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListReplicationChecks)

		instances := databases.Group("/instances")
		{
			instances.GET("", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceView), s.service.ListInstances)
			instances.POST("", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceCreate), s.service.CreateInstance)
			instances.GET("/:id", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceView), s.service.GetInstance)
			instances.PUT("/:id", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceUpdate), s.service.UpdateInstance)
			instances.DELETE("/:id", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceDelete), s.service.DeleteInstance)
			instances.POST("/:id/enable", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceStatus), s.service.EnableInstance)
			instances.POST("/:id/disable", s.authMiddleware.RequireMenuPermission(permDatabaseInstanceStatus), s.service.DisableInstance)
			instances.POST("/:id/test", s.authMiddleware.RequireMenuPermission(permDatabaseConnectionTest), s.service.TestInstance)
			instances.POST("/:id/sync-metadata", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataSync), s.service.SyncMetadata)
			instances.GET("/:id/schemas", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataView), s.service.ListSchemas)
			instances.GET("/:id/tables", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataView), s.service.ListTables)
			instances.GET("/:id/columns", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataView), s.service.ListColumns)
			instances.GET("/:id/indexes", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataView), s.service.ListIndexes)
			instances.GET("/:id/ddl", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataView), s.service.GetTableDDL)
			instances.GET("/:id/dictionary/export", s.authMiddleware.RequireMenuPermission(permDatabaseMetadataExport), s.service.ExportTableDictionary)
			instances.GET("/:id/metrics", s.authMiddleware.RequireMenuPermission(permDatabaseDiagnosisView), s.service.GetDiagnosisMetrics)
			instances.GET("/:id/sessions", s.authMiddleware.RequireMenuPermission(permDatabaseDiagnosisView), s.service.ListDiagnosisSessions)
			instances.GET("/:id/slow-queries", s.authMiddleware.RequireMenuPermission(permDatabaseDiagnosisView), s.service.ListSlowQueries)
			instances.GET("/:id/topology", s.authMiddleware.RequireMenuPermission(permDatabaseTopologyView), s.service.GetTopology)
			instances.GET("/:id/replicas", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.ListInstanceReplicas)
			instances.GET("/:id/replication-status", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaView), s.service.GetInstanceReplicationStatus)
			instances.POST("/:id/replication-check", s.authMiddleware.RequireMenuPermission(permDatabaseReplicaCheck), s.service.CheckInstanceReplication)
			instances.GET("/:id/capacity-trend", s.authMiddleware.RequireMenuPermission(permDatabaseCapacityView), s.service.GetCapacityTrend)
			instances.POST("/:id/capacity-snapshots", s.authMiddleware.RequireMenuPermission(permDatabaseCapacityCollect), s.service.CollectCapacitySnapshot)
			instances.POST("/:id/query/format", s.authMiddleware.RequireMenuPermission(permDatabaseQueryExecute), s.service.FormatQuerySQL)
			instances.POST("/:id/query/write/validate", s.authMiddleware.RequireMenuPermission(permDatabaseQueryWrite), s.service.ValidateWriteQuery)
			instances.POST("/:id/query/write/explain", s.authMiddleware.RequireMenuPermission(permDatabaseQueryWritePlan), s.service.ExplainWriteQuery)
			instances.POST("/:id/query/write", s.authMiddleware.RequireMenuPermission(permDatabaseQueryWrite), s.service.ExecuteWriteQuery)
			instances.POST("/:id/query/ddl/validate", s.authMiddleware.RequireMenuPermission(permDatabaseQueryDDL), s.service.ValidateDDLQuery)
			instances.POST("/:id/query/ddl", s.authMiddleware.RequireMenuPermission(permDatabaseQueryDDL), s.service.ExecuteDDLQuery)
			instances.POST("/:id/query", s.authMiddleware.RequireMenuPermission(permDatabaseQueryExecute), s.service.ExecuteQuery)
			instances.POST("/:id/query/explain", s.authMiddleware.RequireMenuPermission(permDatabaseQueryExplain), s.service.ExplainQuery)
			instances.POST("/:id/query/export", s.authMiddleware.RequireMenuPermission(permDatabaseQueryExport), s.service.ExportQueryResult)
		}
	}
}

func (s *HTTPServer) RegisterPublicRoutes(r *gin.RouterGroup) {
	agents := r.Group("/databases/runner-agents/:runnerId")
	{
		agents.POST("/heartbeat", s.service.RunnerAgentHeartbeat)
		agents.GET("/log-archive-streams", s.service.RunnerAgentListLogArchiveStreams)
		agents.POST("/log-archive-streams/:id/checkpoint", s.service.RunnerAgentCheckpointLogArchiveStream)
		agents.POST("/log-archives", s.service.RunnerAgentRegisterLogArchive)
		agents.POST("/log-archive-events", s.service.RunnerAgentCreateLogArchiveEvent)
	}
}
