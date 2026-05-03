package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type ConnectionCredential struct {
	Username   string
	Password   string
	PrivateKey string
	Passphrase string
}

type DatabaseWritePolicy struct {
	WriteEnabled            bool
	WriteExplainEnabled     bool
	DDLEnabled              bool
	DDLHighRiskConfirm      bool
	DDLReasonRequired       bool
	DDLRequireBackupHint    bool
	HighRiskRequiresConfirm bool
	OperationReasonRequired bool
	MaxAffectedRows         int64
}

type DatabaseBackupPolicy struct {
	DefaultRetentionDays int
	StoragePath          string
}

type UseCase struct {
	instanceRepo           InstanceRepo
	schemaRepo             SchemaRepo
	tableRepo              TableRepo
	columnRepo             ColumnRepo
	indexRepo              IndexRepo
	metadataRepo           MetadataRepo
	redisMetadataRepo      RedisMetadataRepo
	syncJobRepo            SyncJobRepo
	auditRepo              QueryAuditRepo
	backupTaskRepo         BackupTaskRepo
	backupRecordRepo       BackupRecordRepo
	restoreJobRepo         RestoreJobRepo
	capacitySnapshotRepo   CapacitySnapshotRepo
	inspectionReportRepo   InspectionReportRepo
	logArchiveStreamRepo   LogArchiveStreamRepo
	logArchiveRepo         LogArchiveRepo
	logArchiveEventRepo    LogArchiveEventRepo
	restorePlanRepo        RestorePlanRepo
	storageProfileRepo     StorageProfileRepo
	secretProfileRepo      SecretProfileRepo
	runnerHostRepo         RunnerHostRepo
	runnerJobRepo          RunnerJobRepo
	barmanServerRepo       BarmanServerRepo
	instanceReplicaRepo    InstanceReplicaRepo
	replicationCheckRepo   ReplicationCheckRepo
	credentialIDExists     func(ctx context.Context, id uint) error
	credentialResolver     func(ctx context.Context, id uint) (*ConnectionCredential, error)
	writePolicyResolver    func(ctx context.Context) (*DatabaseWritePolicy, error)
	backupPolicyResolver   func(ctx context.Context) (*DatabaseBackupPolicy, error)
	backupRunMu            sync.Mutex
	backupRunningTasks     map[uint]struct{}
	backupRunningInstances map[uint]struct{}
	restoreRunMu           sync.Mutex
	restoreRunningTargets  map[string]struct{}
	startedAt              time.Time
}

func NewUseCase(
	instanceRepo InstanceRepo,
	schemaRepo SchemaRepo,
	tableRepo TableRepo,
	columnRepo ColumnRepo,
	indexRepo IndexRepo,
	metadataRepo MetadataRepo,
	redisMetadataRepo RedisMetadataRepo,
	syncJobRepo SyncJobRepo,
	auditRepo QueryAuditRepo,
	backupTaskRepo BackupTaskRepo,
	backupRecordRepo BackupRecordRepo,
	restoreJobRepo RestoreJobRepo,
	capacitySnapshotRepo CapacitySnapshotRepo,
	inspectionReportRepo InspectionReportRepo,
	credentialIDExists func(ctx context.Context, id uint) error,
	credentialResolver func(ctx context.Context, id uint) (*ConnectionCredential, error),
	writePolicyResolver func(ctx context.Context) (*DatabaseWritePolicy, error),
	backupPolicyResolver func(ctx context.Context) (*DatabaseBackupPolicy, error),
) *UseCase {
	return &UseCase{
		instanceRepo:           instanceRepo,
		schemaRepo:             schemaRepo,
		tableRepo:              tableRepo,
		columnRepo:             columnRepo,
		indexRepo:              indexRepo,
		metadataRepo:           metadataRepo,
		redisMetadataRepo:      redisMetadataRepo,
		syncJobRepo:            syncJobRepo,
		auditRepo:              auditRepo,
		backupTaskRepo:         backupTaskRepo,
		backupRecordRepo:       backupRecordRepo,
		restoreJobRepo:         restoreJobRepo,
		capacitySnapshotRepo:   capacitySnapshotRepo,
		inspectionReportRepo:   inspectionReportRepo,
		credentialIDExists:     credentialIDExists,
		credentialResolver:     credentialResolver,
		writePolicyResolver:    writePolicyResolver,
		backupPolicyResolver:   backupPolicyResolver,
		backupRunningTasks:     make(map[uint]struct{}),
		backupRunningInstances: make(map[uint]struct{}),
		restoreRunningTargets:  make(map[string]struct{}),
		startedAt:              time.Now(),
	}
}

func (uc *UseCase) SetBackupGovernanceRepos(
	logArchiveStreamRepo LogArchiveStreamRepo,
	logArchiveRepo LogArchiveRepo,
	logArchiveEventRepo LogArchiveEventRepo,
	restorePlanRepo RestorePlanRepo,
	storageProfileRepo StorageProfileRepo,
	secretProfileRepo SecretProfileRepo,
	runnerHostRepo RunnerHostRepo,
	runnerJobRepo RunnerJobRepo,
	barmanServerRepo BarmanServerRepo,
) {
	if uc == nil {
		return
	}
	uc.logArchiveStreamRepo = logArchiveStreamRepo
	uc.logArchiveRepo = logArchiveRepo
	uc.logArchiveEventRepo = logArchiveEventRepo
	uc.restorePlanRepo = restorePlanRepo
	uc.storageProfileRepo = storageProfileRepo
	uc.secretProfileRepo = secretProfileRepo
	uc.runnerHostRepo = runnerHostRepo
	uc.runnerJobRepo = runnerJobRepo
	uc.barmanServerRepo = barmanServerRepo
}

func (uc *UseCase) SetReplicaGovernanceRepos(
	instanceReplicaRepo InstanceReplicaRepo,
	replicationCheckRepo ReplicationCheckRepo,
) {
	if uc == nil {
		return
	}
	uc.instanceReplicaRepo = instanceReplicaRepo
	uc.replicationCheckRepo = replicationCheckRepo
}

type DatabaseInstanceRequest struct {
	ID               uint   `json:"id"`
	Name             string `json:"name" binding:"required,min=2,max=100"`
	DBType           string `json:"dbType" binding:"required"`
	Host             string `json:"host" binding:"required,max=255"`
	Port             int    `json:"port" binding:"omitempty,min=1,max=65535"`
	DefaultDatabase  string `json:"defaultDatabase" binding:"omitempty,max=120"`
	CredentialID     uint   `json:"credentialId" binding:"required"`
	TLSEnabled       bool   `json:"tlsEnabled"`
	ConnectionParams string `json:"connectionParams"`
	Status           string `json:"status" binding:"omitempty,oneof=enabled disabled"`
	Environment      string `json:"environment" binding:"omitempty,max=50"`
	BusinessSystem   string `json:"businessSystem" binding:"omitempty,max=100"`
	Owner            string `json:"owner" binding:"omitempty,max=100"`
	Tags             string `json:"tags" binding:"omitempty,max=500"`
	Remark           string `json:"remark" binding:"omitempty,max=500"`
}

type DatabaseInstanceListRequest struct {
	Page              int    `form:"page"`
	PageSize          int    `form:"pageSize"`
	Keyword           string `form:"keyword"`
	DBType            string `form:"dbType"`
	Status            string `form:"status"`
	Environment       string `form:"environment"`
	RestrictToAllowed bool   `form:"-" json:"-"`
	AllowedIDs        []uint `form:"-" json:"-"`
}

type DatabaseQueryAuditListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	Keyword            string `form:"keyword"`
	InstanceID         uint   `form:"instanceId"`
	Action             string `form:"action"`
	Status             string `form:"status"`
	RiskLevel          string `form:"riskLevel"`
	SQLType            string `form:"sqlType"`
	StartTime          string `form:"startTime"`
	EndTime            string `form:"endTime"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseBackupTaskListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	Keyword            string `form:"keyword"`
	InstanceID         uint   `form:"instanceId"`
	Enabled            string `form:"enabled"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseBackupRecordListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	TaskID             uint   `form:"taskId"`
	InstanceID         uint   `form:"instanceId"`
	Status             string `form:"status"`
	TriggerType        string `form:"triggerType"`
	BackupMethod       string `form:"backupMethod"`
	BackupEngine       string `form:"backupEngine"`
	BackupScope        string `form:"backupScope"`
	DateFrom           string `form:"dateFrom"`
	DateTo             string `form:"dateTo"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseRestoreJobListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	BackupRecordID     uint   `form:"backupRecordId"`
	RestorePlanID      uint   `form:"restorePlanId"`
	RunnerHostID       uint   `form:"runnerHostId"`
	SourceInstanceID   uint   `form:"sourceInstanceId"`
	TargetInstanceID   uint   `form:"targetInstanceId"`
	Status             string `form:"status"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseInstancePermissionListRequest struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	RoleID     uint   `form:"roleId"`
	InstanceID uint   `form:"instanceId"`
	Keyword    string `form:"keyword"`
}

type DatabaseInstanceReplicaListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	InstanceID         uint   `form:"instanceId"`
	PrimaryInstanceID  uint   `form:"primaryInstanceId"`
	ReplicaInstanceID  uint   `form:"replicaInstanceId"`
	Engine             string `form:"engine"`
	ReplicaRole        string `form:"replicaRole"`
	Status             string `form:"status"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseReplicationCheckListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	InstanceID         uint   `form:"instanceId"`
	ReplicaID          uint   `form:"replicaId"`
	Engine             string `form:"engine"`
	RoleDetected       string `form:"roleDetected"`
	HealthStatus       string `form:"healthStatus"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseReplicaProtectionListRequest struct {
	Page                         int    `form:"page"`
	PageSize                     int    `form:"pageSize"`
	InstanceID                   uint   `form:"instanceId"`
	Engine                       string `form:"engine"`
	RiskLevel                    string `form:"riskLevel"`
	ProtectionStatus             string `form:"protectionStatus"`
	LagWarningSeconds            int    `form:"lagWarningSeconds"`
	LagCriticalSeconds           int    `form:"lagCriticalSeconds"`
	RemainingDelayWarningSeconds int    `form:"remainingDelayWarningSeconds"`
	RelayLogBacklogWarningBytes  int64  `form:"relayLogBacklogWarningBytes"`
	WALBacklogWarningBytes       int64  `form:"walBacklogWarningBytes"`
	RestrictToAllowed            bool   `form:"-" json:"-"`
	AllowedInstanceIDs           []uint `form:"-" json:"-"`
}

type DatabaseInstancePermissionRequest struct {
	RoleID      uint `json:"roleId" binding:"required"`
	InstanceID  uint `json:"instanceId" binding:"required"`
	Permissions uint `json:"permissions" binding:"required"`
}

type DatabaseInstancePermissionAuditRequest struct {
	RoleID            uint   `json:"roleId"`
	RoleName          string `json:"roleName"`
	RoleCode          string `json:"roleCode"`
	InstanceID        uint   `json:"instanceId"`
	InstanceName      string `json:"instanceName"`
	BeforePermissions uint   `json:"beforePermissions"`
	AfterPermissions  uint   `json:"afterPermissions"`
}

type DatabaseQueryRequest struct {
	SchemaName             string `json:"schemaName" binding:"omitempty,max=150"`
	SQLText                string `json:"sqlText" binding:"required"`
	Limit                  int    `json:"limit" binding:"omitempty,min=1,max=5000"`
	TimeoutSeconds         int    `json:"timeoutSeconds" binding:"omitempty,min=1,max=30"`
	UnlimitedRows          bool   `json:"unlimitedRows"`
	UnlimitedRowsPermitted bool   `json:"-"`
}

type DatabaseWriteValidateRequest struct {
	SchemaName string `json:"schemaName" binding:"omitempty,max=150"`
	SQLText    string `json:"sqlText" binding:"required"`
}

type DatabaseWriteExecuteRequest struct {
	SchemaName     string `json:"schemaName" binding:"omitempty,max=150"`
	SQLText        string `json:"sqlText" binding:"required"`
	Reason         string `json:"reason" binding:"omitempty,max=500"`
	Confirmed      bool   `json:"confirmed"`
	TimeoutSeconds int    `json:"timeoutSeconds" binding:"omitempty,min=1,max=30"`
}

type DatabaseDDLValidateRequest struct {
	SchemaName string `json:"schemaName" binding:"omitempty,max=150"`
	SQLText    string `json:"sqlText" binding:"required"`
}

type DatabaseBackupTaskRequest struct {
	InstanceID         uint   `json:"instanceId" binding:"required"`
	Name               string `json:"name" binding:"required,min=2,max=120"`
	BackupType         string `json:"backupType" binding:"omitempty,max=30"`
	BackupMethod       string `json:"backupMethod" binding:"omitempty,max=30"`
	BackupLevel        string `json:"backupLevel" binding:"omitempty,max=30"`
	BackupEngine       string `json:"backupEngine" binding:"omitempty,max=60"`
	SourceInstanceID   uint   `json:"sourceInstanceId"`
	SourceRole         string `json:"sourceRole" binding:"omitempty,max=30"`
	StorageProfileID   uint   `json:"storageProfileId"`
	SecretProfileID    uint   `json:"secretProfileId"`
	BackupScope        string `json:"backupScope" binding:"omitempty,max=30"`
	ScopeConfig        string `json:"scopeConfig" binding:"omitempty,max=4000"`
	RPOMinutes         int    `json:"rpoMinutes" binding:"omitempty,min=0,max=10080"`
	RTOMinutes         int    `json:"rtoMinutes" binding:"omitempty,min=0,max=10080"`
	Schedule           string `json:"schedule" binding:"omitempty,max=120"`
	StorageType        string `json:"storageType" binding:"omitempty,max=30"`
	StorageConfig      string `json:"storageConfig" binding:"omitempty,max=2000"`
	RetentionDays      int    `json:"retentionDays" binding:"omitempty,min=1,max=3650"`
	MaxDurationMinutes int    `json:"maxDurationMinutes" binding:"omitempty,min=1,max=10080"`
	Compression        string `json:"compression" binding:"omitempty,max=30"`
	EncryptionEnabled  bool   `json:"encryptionEnabled"`
	Enabled            bool   `json:"enabled"`
}

type DatabaseRestoreDryRunRequest struct {
	TargetInstanceID uint   `json:"targetInstanceId" binding:"required"`
	RestoreMode      string `json:"restoreMode" binding:"omitempty,max=30"`
	RestoreStrategy  string `json:"restoreStrategy" binding:"omitempty,max=30"`
}

type QueryOperator struct {
	ID       uint
	Username string
	ClientIP string
}

type DatabaseInstanceVO struct {
	ID                    uint    `json:"id"`
	Name                  string  `json:"name"`
	DBType                string  `json:"dbType"`
	DBTypeText            string  `json:"dbTypeText"`
	Engine                string  `json:"engine"`
	Version               string  `json:"version"`
	Host                  string  `json:"host"`
	Port                  int     `json:"port"`
	Endpoint              string  `json:"endpoint"`
	DefaultDatabase       string  `json:"defaultDatabase"`
	CredentialID          uint    `json:"credentialId"`
	TLSEnabled            bool    `json:"tlsEnabled"`
	ConnectionParams      string  `json:"connectionParams"`
	Status                string  `json:"status"`
	StatusText            string  `json:"statusText"`
	Environment           string  `json:"environment"`
	BusinessSystem        string  `json:"businessSystem"`
	Owner                 string  `json:"owner"`
	Tags                  string  `json:"tags"`
	Remark                string  `json:"remark"`
	LastTestAt            string  `json:"lastTestAt,omitempty"`
	LastSyncAt            string  `json:"lastSyncAt,omitempty"`
	CapacitySizeText      string  `json:"capacitySizeText,omitempty"`
	CapacityGrowthText    string  `json:"capacityGrowthText,omitempty"`
	CapacityGrowthPercent float64 `json:"capacityGrowthPercent"`
	CapacityCollectedAt   string  `json:"capacityCollectedAt,omitempty"`
	Permissions           uint    `json:"permissions"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
}

type DatabaseInstancePermissionVO struct {
	ID           uint   `json:"id"`
	RoleID       uint   `json:"roleId"`
	RoleName     string `json:"roleName"`
	RoleCode     string `json:"roleCode"`
	InstanceID   uint   `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	Permissions  uint   `json:"permissions"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type DatabaseInstanceReplicaVO struct {
	ID                     uint   `json:"id"`
	PrimaryInstanceID      uint   `json:"primaryInstanceId"`
	PrimaryInstanceName    string `json:"primaryInstanceName"`
	PrimaryEndpoint        string `json:"primaryEndpoint"`
	ReplicaInstanceID      uint   `json:"replicaInstanceId"`
	ReplicaInstanceName    string `json:"replicaInstanceName"`
	ReplicaEndpoint        string `json:"replicaEndpoint"`
	Engine                 string `json:"engine"`
	EngineText             string `json:"engineText"`
	ReplicaRole            string `json:"replicaRole"`
	ReplicaRoleText        string `json:"replicaRoleText"`
	SourceHost             string `json:"sourceHost"`
	SourcePort             int    `json:"sourcePort"`
	SourceServerUUID       string `json:"sourceServerUuid"`
	PGSystemIdentifier     string `json:"pgSystemIdentifier"`
	ApplicationName        string `json:"applicationName"`
	ConfiguredDelaySeconds int    `json:"configuredDelaySeconds"`
	DiscoverySource        string `json:"discoverySource"`
	DiscoverySourceText    string `json:"discoverySourceText"`
	Status                 string `json:"status"`
	StatusText             string `json:"statusText"`
	LastCheckID            uint   `json:"lastCheckId"`
	LastCheckedAt          string `json:"lastCheckedAt"`
	LastError              string `json:"lastError"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

type DatabaseReplicationCheckVO struct {
	ID                        uint   `json:"id"`
	InstanceID                uint   `json:"instanceId"`
	InstanceName              string `json:"instanceName"`
	InstanceEndpoint          string `json:"instanceEndpoint"`
	ReplicaID                 uint   `json:"replicaId"`
	Engine                    string `json:"engine"`
	EngineText                string `json:"engineText"`
	RoleDetected              string `json:"roleDetected"`
	RoleDetectedText          string `json:"roleDetectedText"`
	SourceInstanceID          uint   `json:"sourceInstanceId"`
	SourceInstanceName        string `json:"sourceInstanceName"`
	ReplicaIORunning          string `json:"replicaIoRunning"`
	ReplicaSQLRunning         string `json:"replicaSqlRunning"`
	SecondsBehindSource       int    `json:"secondsBehindSource"`
	ConfiguredDelaySeconds    int    `json:"configuredDelaySeconds"`
	RemainingDelaySeconds     int    `json:"remainingDelaySeconds"`
	RelayLogBytes             int64  `json:"relayLogBytes"`
	PGWriteLagMs              int64  `json:"pgWriteLagMs"`
	PGFlushLagMs              int64  `json:"pgFlushLagMs"`
	PGReplayLagMs             int64  `json:"pgReplayLagMs"`
	PGLastWALReplayLSN        string `json:"pgLastWalReplayLsn"`
	PGLastXactReplayTimestamp string `json:"pgLastXactReplayTimestamp"`
	WALBacklogBytes           int64  `json:"walBacklogBytes"`
	HealthStatus              string `json:"healthStatus"`
	HealthStatusText          string `json:"healthStatusText"`
	RiskFlagsJSON             string `json:"riskFlagsJson"`
	RawStatusJSON             string `json:"rawStatusJson"`
	CheckedAt                 string `json:"checkedAt"`
	ErrorMessage              string `json:"errorMessage"`
	CreatedAt                 string `json:"createdAt"`
}

type DatabaseReplicationStatusVO struct {
	Instance  *DatabaseInstanceVO           `json:"instance,omitempty"`
	Replicas  []*DatabaseInstanceReplicaVO  `json:"replicas"`
	Checks    []*DatabaseReplicationCheckVO `json:"checks"`
	LastCheck *DatabaseReplicationCheckVO   `json:"lastCheck,omitempty"`
	Message   string                        `json:"message"`
}

type DatabaseReplicaProtectionVO struct {
	PrimaryInstanceID            uint     `json:"primaryInstanceId"`
	PrimaryInstanceName          string   `json:"primaryInstanceName"`
	PrimaryEndpoint              string   `json:"primaryEndpoint"`
	Engine                       string   `json:"engine"`
	EngineText                   string   `json:"engineText"`
	HasReplica                   bool     `json:"hasReplica"`
	HasDelayedReplica            bool     `json:"hasDelayedReplica"`
	DelayedReplicaCount          int      `json:"delayedReplicaCount"`
	PreferredReplicaID           uint     `json:"preferredReplicaId"`
	PreferredReplicaInstanceID   uint     `json:"preferredReplicaInstanceId"`
	PreferredReplicaInstanceName string   `json:"preferredReplicaInstanceName"`
	PreferredReplicaEndpoint     string   `json:"preferredReplicaEndpoint"`
	PreferredReplicaStatus       string   `json:"preferredReplicaStatus"`
	PreferredReplicaStatusText   string   `json:"preferredReplicaStatusText"`
	ConfiguredDelaySeconds       int      `json:"configuredDelaySeconds"`
	RemainingDelaySeconds        int      `json:"remainingDelaySeconds"`
	RemainingDelayEstimated      bool     `json:"remainingDelayEstimated"`
	ApplyLagSeconds              int      `json:"applyLagSeconds"`
	ApplyTime                    string   `json:"applyTime"`
	LastCheckID                  uint     `json:"lastCheckId"`
	LastCheckedAt                string   `json:"lastCheckedAt"`
	ProtectionStatus             string   `json:"protectionStatus"`
	ProtectionStatusText         string   `json:"protectionStatusText"`
	RiskLevel                    string   `json:"riskLevel"`
	RiskLevelText                string   `json:"riskLevelText"`
	RiskMessages                 []string `json:"riskMessages"`
	RiskFlagsJSON                string   `json:"riskFlagsJson"`
	LagWarningSeconds            int      `json:"lagWarningSeconds"`
	LagCriticalSeconds           int      `json:"lagCriticalSeconds"`
	RemainingDelayWarningSeconds int      `json:"remainingDelayWarningSeconds"`
	RelayLogBacklogWarningBytes  int64    `json:"relayLogBacklogWarningBytes"`
	WALBacklogWarningBytes       int64    `json:"walBacklogWarningBytes"`
}

type SupportedTypeVO struct {
	Type            string `json:"type"`
	Name            string `json:"name"`
	DefaultPort     int    `json:"defaultPort"`
	MetadataEnabled bool   `json:"metadataEnabled"`
	QueryEnabled    bool   `json:"queryEnabled"`
	TestEnabled     bool   `json:"testEnabled"`
	TopologyEnabled bool   `json:"topologyEnabled"`
	Phase           string `json:"phase"`
}

type ConnectionTestResultVO struct {
	InstanceID uint   `json:"instanceId"`
	Name       string `json:"name"`
	DBType     string `json:"dbType"`
	Version    string `json:"version"`
	LatencyMs  int64  `json:"latencyMs"`
	Message    string `json:"message"`
	TestedAt   string `json:"testedAt"`
}

type MetadataSyncResultVO struct {
	JobID        uint   `json:"jobId"`
	InstanceID   uint   `json:"instanceId"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Version      string `json:"version"`
	DurationMs   int64  `json:"durationMs"`
	SchemasCount int    `json:"schemasCount"`
	TablesCount  int    `json:"tablesCount"`
	ColumnsCount int    `json:"columnsCount"`
	IndexesCount int    `json:"indexesCount"`
	SyncedAt     string `json:"syncedAt"`
}

type DatabaseSchemaVO struct {
	ID           uint   `json:"id"`
	InstanceID   uint   `json:"instanceId"`
	SchemaName   string `json:"schemaName"`
	Charset      string `json:"charset"`
	Collation    string `json:"collation"`
	SizeBytes    int64  `json:"sizeBytes"`
	TableCount   int    `json:"tableCount"`
	ExpiresCount int64  `json:"expiresCount,omitempty"`
	AvgTTLText   string `json:"avgTtlText,omitempty"`
	LastSyncAt   string `json:"lastSyncAt,omitempty"`
}

type DatabaseTableVO struct {
	ID             uint   `json:"id"`
	InstanceID     uint   `json:"instanceId"`
	SchemaName     string `json:"schemaName"`
	TableName      string `json:"tableName"`
	TableType      string `json:"tableType"`
	Engine         string `json:"engine"`
	RowCount       int64  `json:"rowCount"`
	DataSizeBytes  int64  `json:"dataSizeBytes"`
	IndexSizeBytes int64  `json:"indexSizeBytes"`
	TTLMillis      int64  `json:"ttlMillis,omitempty"`
	TTLText        string `json:"ttlText,omitempty"`
	Encoding       string `json:"encoding,omitempty"`
	NodeAddress    string `json:"nodeAddress,omitempty"`
	Slot           int    `json:"slot,omitempty"`
	Preview        string `json:"preview,omitempty"`
	Comment        string `json:"comment"`
	LastSyncAt     string `json:"lastSyncAt,omitempty"`
}

type DatabaseColumnVO struct {
	ID              uint   `json:"id"`
	InstanceID      uint   `json:"instanceId"`
	SchemaName      string `json:"schemaName"`
	TableName       string `json:"tableName"`
	ColumnName      string `json:"columnName"`
	OrdinalPosition int    `json:"ordinalPosition"`
	DataType        string `json:"dataType"`
	IsNullable      bool   `json:"isNullable"`
	DefaultValue    string `json:"defaultValue"`
	ColumnKey       string `json:"columnKey"`
	Comment         string `json:"comment"`
	IsSensitive     bool   `json:"isSensitive"`
}

type DatabaseIndexVO struct {
	ID          uint   `json:"id"`
	InstanceID  uint   `json:"instanceId"`
	SchemaName  string `json:"schemaName"`
	TableName   string `json:"tableName"`
	IndexName   string `json:"indexName"`
	IndexType   string `json:"indexType"`
	Columns     string `json:"columns"`
	IsUnique    bool   `json:"isUnique"`
	Cardinality int64  `json:"cardinality"`
	Comment     string `json:"comment"`
}

type DatabaseQueryResultVO struct {
	AuditID         uint             `json:"auditId"`
	InstanceID      uint             `json:"instanceId"`
	SchemaName      string           `json:"schemaName"`
	SQLType         string           `json:"sqlType"`
	ExecutedSQL     string           `json:"executedSql"`
	Columns         []string         `json:"columns"`
	ColumnTypes     []string         `json:"columnTypes"`
	Rows            []map[string]any `json:"rows"`
	RowsReturned    int              `json:"rowsReturned"`
	DurationMs      int64            `json:"durationMs"`
	Limit           int              `json:"limit"`
	Truncated       bool             `json:"truncated"`
	CellTruncated   bool             `json:"cellTruncated"`
	CellsMasked     bool             `json:"cellsMasked"`
	BinaryPreviewed bool             `json:"binaryPreviewed"`
	ResultBytes     int              `json:"resultBytes"`
	Message         string           `json:"message"`
	ExecutedAt      string           `json:"executedAt"`
}

type DatabaseWriteValidateVO struct {
	InstanceID        uint   `json:"instanceId"`
	InstanceName      string `json:"instanceName"`
	DBType            string `json:"dbType"`
	DBTypeText        string `json:"dbTypeText"`
	SchemaName        string `json:"schemaName"`
	SQLType           string `json:"sqlType"`
	RiskLevel         string `json:"riskLevel"`
	RiskLevelText     string `json:"riskLevelText"`
	Allowed           bool   `json:"allowed"`
	ConfirmRequired   bool   `json:"confirmRequired"`
	ReasonRequired    bool   `json:"reasonRequired"`
	RowsAffectedLimit int64  `json:"rowsAffectedLimit"`
	Message           string `json:"message"`
}

type DatabaseDDLValidateVO struct {
	InstanceID      uint   `json:"instanceId"`
	InstanceName    string `json:"instanceName"`
	DBType          string `json:"dbType"`
	DBTypeText      string `json:"dbTypeText"`
	SchemaName      string `json:"schemaName"`
	SQLType         string `json:"sqlType"`
	RiskLevel       string `json:"riskLevel"`
	RiskLevelText   string `json:"riskLevelText"`
	Allowed         bool   `json:"allowed"`
	ConfirmRequired bool   `json:"confirmRequired"`
	ReasonRequired  bool   `json:"reasonRequired"`
	BackupRequired  bool   `json:"backupRequired"`
	Message         string `json:"message"`
}

type DatabaseWriteExecuteVO struct {
	AuditID           uint   `json:"auditId"`
	AuditAction       string `json:"auditAction"`
	InstanceID        uint   `json:"instanceId"`
	InstanceName      string `json:"instanceName"`
	DBType            string `json:"dbType"`
	DBTypeText        string `json:"dbTypeText"`
	SchemaName        string `json:"schemaName"`
	SQLType           string `json:"sqlType"`
	RiskLevel         string `json:"riskLevel"`
	RiskLevelText     string `json:"riskLevelText"`
	ExecutedSQL       string `json:"executedSql"`
	RowsAffected      int64  `json:"rowsAffected"`
	RowsAffectedLimit int64  `json:"rowsAffectedLimit"`
	Reason            string `json:"reason"`
	ConfirmRequired   bool   `json:"confirmRequired"`
	Confirmed         bool   `json:"confirmed"`
	RollbackSQL       string `json:"rollbackSql"`
	DurationMs        int64  `json:"durationMs"`
	Message           string `json:"message"`
	ExecutedAt        string `json:"executedAt"`
}

type DatabaseBackupTaskVO struct {
	ID                        uint   `json:"id"`
	InstanceID                uint   `json:"instanceId"`
	InstanceName              string `json:"instanceName"`
	InstanceDBType            string `json:"instanceDbType"`
	InstanceDBTypeText        string `json:"instanceDbTypeText"`
	Name                      string `json:"name"`
	BackupType                string `json:"backupType"`
	BackupTypeText            string `json:"backupTypeText"`
	BackupMethod              string `json:"backupMethod"`
	BackupMethodText          string `json:"backupMethodText"`
	BackupLevel               string `json:"backupLevel"`
	BackupLevelText           string `json:"backupLevelText"`
	BackupEngine              string `json:"backupEngine"`
	SourceInstanceID          uint   `json:"sourceInstanceId"`
	SourceRole                string `json:"sourceRole"`
	StorageProfileID          uint   `json:"storageProfileId"`
	SecretProfileID           uint   `json:"secretProfileId"`
	BackupScope               string `json:"backupScope"`
	ScopeConfig               string `json:"scopeConfig"`
	RPOMinutes                int    `json:"rpoMinutes"`
	RTOMinutes                int    `json:"rtoMinutes"`
	Schedule                  string `json:"schedule"`
	StorageType               string `json:"storageType"`
	StorageTypeText           string `json:"storageTypeText"`
	StorageConfig             string `json:"storageConfig"`
	RetentionDays             int    `json:"retentionDays"`
	MaxDurationMinutes        int    `json:"maxDurationMinutes"`
	Compression               string `json:"compression"`
	EncryptionEnabled         bool   `json:"encryptionEnabled"`
	Enabled                   bool   `json:"enabled"`
	NextRunAt                 string `json:"nextRunAt"`
	LastRunAt                 string `json:"lastRunAt"`
	LastSuccessAt             string `json:"lastSuccessAt"`
	LastRestoreTestAt         string `json:"lastRestoreTestAt"`
	LastStatus                string `json:"lastStatus"`
	LastStatusText            string `json:"lastStatusText"`
	LastMessage               string `json:"lastMessage"`
	RestoreCapability         string `json:"restoreCapability"`
	RestoreCapabilityText     string `json:"restoreCapabilityText"`
	PITRSupported             bool   `json:"pitrSupported"`
	PITRStatusText            string `json:"pitrStatusText"`
	StrategyText              string `json:"strategyText"`
	InstanceCapacitySizeBytes int64  `json:"instanceCapacitySizeBytes"`
	InstanceCapacitySizeText  string `json:"instanceCapacitySizeText"`
	LargeDataWarning          bool   `json:"largeDataWarning"`
	LargeDataWarningText      string `json:"largeDataWarningText"`
	CreatedAt                 string `json:"createdAt"`
	UpdatedAt                 string `json:"updatedAt"`
}

type DatabaseBackupRecordVO struct {
	ID                    uint   `json:"id"`
	TaskID                uint   `json:"taskId"`
	TaskName              string `json:"taskName"`
	InstanceID            uint   `json:"instanceId"`
	InstanceName          string `json:"instanceName"`
	TriggerType           string `json:"triggerType"`
	TriggerTypeText       string `json:"triggerTypeText"`
	BackupType            string `json:"backupType"`
	BackupTypeText        string `json:"backupTypeText"`
	ChainID               string `json:"chainId"`
	BaseRecordID          uint   `json:"baseRecordId"`
	ParentRecordID        uint   `json:"parentRecordId"`
	BackupMethod          string `json:"backupMethod"`
	BackupMethodText      string `json:"backupMethodText"`
	BackupLevel           string `json:"backupLevel"`
	BackupLevelText       string `json:"backupLevelText"`
	BackupEngine          string `json:"backupEngine"`
	ExternalBackupID      string `json:"externalBackupId"`
	ExternalServerName    string `json:"externalServerName"`
	BackupScope           string `json:"backupScope"`
	ToolName              string `json:"toolName"`
	ToolVersion           string `json:"toolVersion"`
	SourceInstanceID      uint   `json:"sourceInstanceId"`
	SourceRole            string `json:"sourceRole"`
	StorageProfileID      uint   `json:"storageProfileId"`
	StorageType           string `json:"storageType"`
	StorageTypeText       string `json:"storageTypeText"`
	StorageURI            string `json:"storageUri"`
	ManifestJSON          string `json:"manifestJson"`
	PrepareStatus         string `json:"prepareStatus"`
	Status                string `json:"status"`
	StatusText            string `json:"statusText"`
	FileName              string `json:"fileName"`
	FileSize              int64  `json:"fileSize"`
	ChecksumSHA256        string `json:"checksumSha256"`
	Encrypted             bool   `json:"encrypted"`
	Compression           string `json:"compression"`
	ExpiresAt             string `json:"expiresAt"`
	VerifiedAt            string `json:"verifiedAt"`
	VerifyStatus          string `json:"verifyStatus"`
	VerifyStatusText      string `json:"verifyStatusText"`
	VerifyMessage         string `json:"verifyMessage"`
	RestoreTestedAt       string `json:"restoreTestedAt"`
	RestoreTestStatus     string `json:"restoreTestStatus"`
	RestoreTestStatusText string `json:"restoreTestStatusText"`
	StartedAt             string `json:"startedAt"`
	LastHeartbeatAt       string `json:"lastHeartbeatAt"`
	FinishedAt            string `json:"finishedAt"`
	RecoverableFrom       string `json:"recoverableFrom"`
	RecoverableUntil      string `json:"recoverableUntil"`
	DurationMs            int64  `json:"durationMs"`
	Message               string `json:"message"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`

	ServerUUID             string `json:"serverUuid,omitempty"`
	ServerID               string `json:"serverId,omitempty"`
	GTIDMode               string `json:"gtidMode,omitempty"`
	BinlogFormat           string `json:"binlogFormat,omitempty"`
	BackupBinlogFile       string `json:"backupBinlogFile,omitempty"`
	BackupBinlogPos        int64  `json:"backupBinlogPos,omitempty"`
	BackupGTIDSet          string `json:"backupGtidSet,omitempty"`
	PGSystemIdentifier     string `json:"pgSystemIdentifier,omitempty"`
	TimelineID             string `json:"timelineId,omitempty"`
	WALSegmentSize         int64  `json:"walSegmentSize,omitempty"`
	StartLSN               string `json:"startLsn,omitempty"`
	EndLSN                 string `json:"endLsn,omitempty"`
	WALStart               string `json:"walStart,omitempty"`
	WALEnd                 string `json:"walEnd,omitempty"`
	BackupManifestChecksum string `json:"backupManifestChecksum,omitempty"`
}

type DatabaseBackupRunVO struct {
	TaskID         uint   `json:"taskId"`
	TaskName       string `json:"taskName"`
	RecordID       uint   `json:"recordId"`
	InstanceID     uint   `json:"instanceId"`
	InstanceName   string `json:"instanceName"`
	Status         string `json:"status"`
	StatusText     string `json:"statusText"`
	FileName       string `json:"fileName"`
	FileSize       int64  `json:"fileSize"`
	ChecksumSHA256 string `json:"checksumSha256"`
	DurationMs     int64  `json:"durationMs"`
	Message        string `json:"message"`
	TriggeredAt    string `json:"triggeredAt"`
}

type DatabaseBackupDownloadVO struct {
	FilePath    string
	FileName    string
	ContentType string
}

type DatabaseRestoreJobVO struct {
	ID                  uint   `json:"id"`
	BackupRecordID      uint   `json:"backupRecordId"`
	RestorePlanID       uint   `json:"restorePlanId"`
	RunnerHostID        uint   `json:"runnerHostId"`
	RunnerHostName      string `json:"runnerHostName"`
	RunnerJobID         uint   `json:"runnerJobId"`
	SourceInstanceID    uint   `json:"sourceInstanceId"`
	SourceInstanceName  string `json:"sourceInstanceName"`
	TargetInstanceID    uint   `json:"targetInstanceId"`
	TargetInstanceName  string `json:"targetInstanceName"`
	TargetEnvironment   string `json:"targetEnvironment"`
	RestoreMode         string `json:"restoreMode"`
	RestoreModeText     string `json:"restoreModeText"`
	RestoreStrategy     string `json:"restoreStrategy"`
	RestoreStrategyText string `json:"restoreStrategyText"`
	RestoreTargetType   string `json:"restoreTargetType"`
	RestoreTargetValue  string `json:"restoreTargetValue"`
	Status              string `json:"status"`
	StatusText          string `json:"statusText"`
	FileName            string `json:"fileName"`
	FileSize            int64  `json:"fileSize"`
	WorkDir             string `json:"workDir"`
	PreparedDatadir     string `json:"preparedDatadir"`
	ContainerName       string `json:"containerName"`
	ContainerImage      string `json:"containerImage"`
	ListenHost          string `json:"listenHost"`
	ListenPort          int    `json:"listenPort"`
	StepJSON            string `json:"stepJson"`
	ValidationJSON      string `json:"validationJson"`
	ProofJSON           string `json:"proofJson"`
	LogPath             string `json:"logPath"`
	ArtifactURI         string `json:"artifactUri"`
	ExpiresAt           string `json:"expiresAt"`
	CleanupStatus       string `json:"cleanupStatus"`
	OperatorID          uint   `json:"operatorId"`
	OperatorName        string `json:"operatorName"`
	StartedAt           string `json:"startedAt"`
	FinishedAt          string `json:"finishedAt"`
	DurationMs          int64  `json:"durationMs"`
	Message             string `json:"message"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type DatabaseQueryAuditVO struct {
	ID                uint   `json:"id"`
	InstanceID        uint   `json:"instanceId"`
	InstanceName      string `json:"instanceName"`
	SchemaName        string `json:"schemaName"`
	OperatorID        uint   `json:"operatorId"`
	OperatorName      string `json:"operatorName"`
	Action            string `json:"action"`
	ActionText        string `json:"actionText"`
	SQLText           string `json:"sqlText"`
	SQLSummary        string `json:"sqlSummary"`
	SQLFingerprint    string `json:"sqlFingerprint"`
	SQLType           string `json:"sqlType"`
	RiskLevel         string `json:"riskLevel"`
	RiskLevelText     string `json:"riskLevelText"`
	Status            string `json:"status"`
	StatusText        string `json:"statusText"`
	RowsReturned      int    `json:"rowsReturned"`
	RowsAffectedLimit int64  `json:"rowsAffectedLimit"`
	RowsAffected      int64  `json:"rowsAffected"`
	Reason            string `json:"reason"`
	ConfirmRequired   bool   `json:"confirmRequired"`
	Confirmed         bool   `json:"confirmed"`
	RollbackSQL       string `json:"rollbackSql"`
	DurationMs        int64  `json:"durationMs"`
	ErrorMessage      string `json:"errorMessage"`
	ClientIP          string `json:"clientIp"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

func (uc *UseCase) SupportedTypes() []*SupportedTypeVO {
	return []*SupportedTypeVO{
		{Type: DBTypeMySQL, Name: "MySQL", DefaultPort: 3306, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase1"},
		{Type: DBTypeMariaDB, Name: "MariaDB", DefaultPort: 3306, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase1"},
		{Type: DBTypePostgreSQL, Name: "PostgreSQL", DefaultPort: 5432, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase1"},
		{Type: DBTypeSQLServer, Name: "SQL Server", DefaultPort: 1433, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase2"},
		{Type: DBTypeClickHouse, Name: "ClickHouse", DefaultPort: 9000, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase2"},
		{Type: DBTypeOracle, Name: "Oracle", DefaultPort: 1521, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase2-research"},
		{Type: DBTypeRedis, Name: "Redis", DefaultPort: 6379, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, TopologyEnabled: true, Phase: "phase1-redis"},
		{Type: DBTypeMongoDB, Name: "MongoDB", DefaultPort: 27017, TopologyEnabled: true, Phase: "phase4-topology-only"},
		{Type: DBTypeElasticsearch, Name: "Elasticsearch", DefaultPort: 9200, TestEnabled: true, TopologyEnabled: true, Phase: "phase4-topology"},
		{Type: DBTypeOpenSearch, Name: "OpenSearch", DefaultPort: 9200, TestEnabled: true, TopologyEnabled: true, Phase: "phase4-topology"},
		{Type: DBTypeTiDB, Name: "TiDB", DefaultPort: 4000, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase4-compatible"},
		{Type: DBTypeOceanBase, Name: "OceanBase MySQL", DefaultPort: 2881, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase4-compatible"},
		{Type: DBTypeOpenGauss, Name: "openGauss", DefaultPort: 5432, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase4-compatible"},
		{Type: DBTypeKingbase, Name: "人大金仓 Kingbase", DefaultPort: 54321, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, Phase: "phase4-compatible"},
		{Type: DBTypeDameng, Name: "达梦 Dameng", DefaultPort: 5236, Phase: "phase4-research-only"},
	}
}

func (uc *UseCase) CreateInstance(ctx context.Context, req *DatabaseInstanceRequest) (*DatabaseInstanceVO, error) {
	if err := uc.validateInstanceRequest(ctx, req); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = DatabaseInstanceStatusEnabled
	}
	port := req.Port
	if port <= 0 {
		port = DefaultPort(req.DBType)
	}
	item := &DatabaseInstance{
		Name:             strings.TrimSpace(req.Name),
		DBType:           normalizeDBType(req.DBType),
		Engine:           normalizeDBType(req.DBType),
		Host:             strings.TrimSpace(req.Host),
		Port:             port,
		DefaultDatabase:  strings.TrimSpace(req.DefaultDatabase),
		CredentialID:     req.CredentialID,
		TLSEnabled:       req.TLSEnabled,
		ConnectionParams: strings.TrimSpace(req.ConnectionParams),
		Status:           status,
		Environment:      strings.TrimSpace(req.Environment),
		BusinessSystem:   strings.TrimSpace(req.BusinessSystem),
		Owner:            strings.TrimSpace(req.Owner),
		Tags:             strings.TrimSpace(req.Tags),
		Remark:           strings.TrimSpace(req.Remark),
	}
	if err := uc.instanceRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toInstanceVO(item), nil
}

func (uc *UseCase) UpdateInstance(ctx context.Context, req *DatabaseInstanceRequest) error {
	if req.ID == 0 {
		return fmt.Errorf("数据库实例ID不能为空")
	}
	if err := uc.validateInstanceRequest(ctx, req); err != nil {
		return err
	}
	item, err := uc.instanceRepo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("数据库实例不存在")
	}
	item.Name = strings.TrimSpace(req.Name)
	item.DBType = normalizeDBType(req.DBType)
	item.Engine = normalizeDBType(req.DBType)
	item.Host = strings.TrimSpace(req.Host)
	item.Port = req.Port
	if item.Port <= 0 {
		item.Port = DefaultPort(req.DBType)
	}
	item.DefaultDatabase = strings.TrimSpace(req.DefaultDatabase)
	item.CredentialID = req.CredentialID
	item.TLSEnabled = req.TLSEnabled
	item.ConnectionParams = strings.TrimSpace(req.ConnectionParams)
	if status := strings.TrimSpace(req.Status); status != "" {
		item.Status = status
	}
	item.Environment = strings.TrimSpace(req.Environment)
	item.BusinessSystem = strings.TrimSpace(req.BusinessSystem)
	item.Owner = strings.TrimSpace(req.Owner)
	item.Tags = strings.TrimSpace(req.Tags)
	item.Remark = strings.TrimSpace(req.Remark)
	return uc.instanceRepo.Update(ctx, item)
}

func (uc *UseCase) DeleteInstance(ctx context.Context, id uint) error {
	if id == 0 {
		return fmt.Errorf("数据库实例ID不能为空")
	}
	return uc.instanceRepo.Delete(ctx, id)
}

func (uc *UseCase) GetInstance(ctx context.Context, id uint) (*DatabaseInstanceVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	return uc.toInstanceVO(item), nil
}

func (uc *UseCase) ListInstances(ctx context.Context, req *DatabaseInstanceListRequest) ([]*DatabaseInstanceVO, int64, error) {
	normalizeListRequest(req)
	items, total, err := uc.instanceRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseInstanceVO, 0, len(items))
	for _, item := range items {
		vo := uc.toInstanceVO(item)
		uc.applyInstanceCapacitySummary(ctx, item, vo)
		list = append(list, vo)
	}
	return list, total, nil
}

func (uc *UseCase) SetInstanceStatus(ctx context.Context, id uint, status string) error {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("数据库实例不存在")
	}
	status = strings.TrimSpace(status)
	if status != DatabaseInstanceStatusEnabled && status != DatabaseInstanceStatusDisabled {
		return fmt.Errorf("不支持的实例状态")
	}
	item.Status = status
	return uc.instanceRepo.Update(ctx, item)
}

func (uc *UseCase) TestInstance(ctx context.Context, id uint) (*ConnectionTestResultVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(item.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}

	start := time.Now()
	version, err := testSQLConnection(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item.Version = version
	item.LastTestAt = &now
	if strings.TrimSpace(item.Engine) == "" {
		item.Engine = item.DBType
	}
	if err := uc.instanceRepo.Update(ctx, item); err != nil {
		return nil, err
	}

	return &ConnectionTestResultVO{
		InstanceID: item.ID,
		Name:       item.Name,
		DBType:     item.DBType,
		Version:    version,
		LatencyMs:  time.Since(start).Milliseconds(),
		Message:    "连接测试成功",
		TestedAt:   now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) SyncMetadata(ctx context.Context, id uint) (*MetadataSyncResultVO, error) {
	item, err := uc.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if strings.TrimSpace(item.Status) != DatabaseInstanceStatusEnabled {
		return nil, fmt.Errorf("数据库实例已禁用")
	}
	if uc.metadataRepo == nil {
		return nil, fmt.Errorf("元数据仓储未配置")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, item.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}

	start := time.Now()
	job := &DatabaseSyncJob{
		InstanceID:  item.ID,
		TriggerType: "manual",
		Status:      DatabaseSyncStatusRunning,
		StartedAt:   &start,
	}
	if uc.syncJobRepo != nil {
		if err := uc.syncJobRepo.Create(ctx, job); err != nil {
			return nil, fmt.Errorf("创建同步任务失败: %w", err)
		}
	}

	if normalizeDBType(item.DBType) == DBTypeRedis {
		return uc.syncRedisMetadata(ctx, item, credential, job, start)
	}

	version, schemas, tables, columns, indexes, err := collectSQLMetadata(ctx, item, credential)
	now := time.Now()
	if err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, err.Error(), start, now, 0, 0, 0, 0)
		return nil, err
	}
	for _, schema := range schemas {
		schema.InstanceID = item.ID
		schema.LastSyncAt = &now
	}
	for _, table := range tables {
		table.InstanceID = item.ID
		table.LastSyncAt = &now
	}
	for _, column := range columns {
		column.InstanceID = item.ID
	}
	for _, index := range indexes {
		index.InstanceID = item.ID
	}

	if err := uc.metadataRepo.ReplaceAll(ctx, item.ID, schemas, tables, columns, indexes); err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, "写入元数据失败: "+err.Error(), start, now, 0, 0, 0, 0)
		return nil, fmt.Errorf("写入元数据失败: %w", err)
	}

	item.Version = version
	item.LastSyncAt = &now
	if strings.TrimSpace(item.Engine) == "" {
		item.Engine = item.DBType
	}
	if err := uc.instanceRepo.Update(ctx, item); err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, "更新实例同步时间失败: "+err.Error(), start, now, 0, 0, 0, 0)
		return nil, err
	}

	duration := now.Sub(start).Milliseconds()
	message := "元数据同步成功"
	uc.finishSyncJob(ctx, job, DatabaseSyncStatusSuccess, message, start, now, len(schemas), len(tables), len(columns), len(indexes))
	return &MetadataSyncResultVO{
		JobID:        job.ID,
		InstanceID:   item.ID,
		Name:         item.Name,
		Status:       DatabaseSyncStatusSuccess,
		Message:      message,
		Version:      version,
		DurationMs:   duration,
		SchemasCount: len(schemas),
		TablesCount:  len(tables),
		ColumnsCount: len(columns),
		IndexesCount: len(indexes),
		SyncedAt:     now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) ListSchemas(ctx context.Context, instanceID uint) ([]*DatabaseSchemaVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.listRedisSchemas(ctx, instanceID)
	}
	items, err := uc.schemaRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseSchemaVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toSchemaVO(item))
	}
	return list, nil
}

func (uc *UseCase) ListTables(ctx context.Context, instanceID uint, schemaName string) ([]*DatabaseTableVO, error) {
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.listRedisKeys(ctx, instanceID, schemaName)
	}
	items, err := uc.tableRepo.List(ctx, instanceID, strings.TrimSpace(schemaName))
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseTableVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toTableVO(item))
	}
	return list, nil
}

func (uc *UseCase) ListColumns(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*DatabaseColumnVO, error) {
	if strings.TrimSpace(tableName) == "" {
		return nil, fmt.Errorf("表名不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return uc.listRedisKeyDetails(ctx, instanceID, schemaName, tableName)
	}
	items, err := uc.columnRepo.List(ctx, instanceID, strings.TrimSpace(schemaName), strings.TrimSpace(tableName))
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseColumnVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toColumnVO(item))
	}
	return list, nil
}

func (uc *UseCase) ListIndexes(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*DatabaseIndexVO, error) {
	if strings.TrimSpace(tableName) == "" {
		return nil, fmt.Errorf("表名不能为空")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if normalizeDBType(instance.DBType) == DBTypeRedis {
		return []*DatabaseIndexVO{}, nil
	}
	items, err := uc.indexRepo.List(ctx, instanceID, strings.TrimSpace(schemaName), strings.TrimSpace(tableName))
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseIndexVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toIndexVO(item))
	}
	return list, nil
}

func (uc *UseCase) ExecuteQuery(ctx context.Context, instanceID uint, req *DatabaseQueryRequest, operator QueryOperator) (*DatabaseQueryResultVO, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	item, err := uc.getEnabledQueryInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	if req.UnlimitedRows && !req.UnlimitedRowsPermitted {
		return nil, fmt.Errorf("无不限行数查询权限")
	}
	if req.UnlimitedRows && normalizeDBType(item.DBType) == DBTypeRedis {
		return nil, fmt.Errorf("Redis 查询控制台暂不支持不限行数")
	}

	limit := normalizeQueryLimit(req.Limit, req.UnlimitedRows)
	timeout := normalizeQueryTimeout(req.TimeoutSeconds)
	schemaName := resolveQuerySchemaName(item, req.SchemaName)
	sqlText := strings.TrimSpace(req.SQLText)
	if normalizeDBType(item.DBType) == DBTypeRedis {
		safety := analyzeReadOnlyRedisCommand(sqlText, limit)
		if !safety.Allowed {
			uc.recordDeniedQuery(ctx, item, schemaName, DatabaseAuditActionQuery, sqlText, safety.SQLType, safety.Message, operator)
			return nil, errors.New(safety.Message)
		}
		return uc.runAuditedQuery(ctx, item, schemaName, DatabaseAuditActionQuery, sqlText, safety.SQLType, safety.SQLText, limit, timeout, operator)
	}
	safety := AnalyzeReadOnlySQLByDB(item.DBType, sqlText, limit)
	if !safety.Allowed {
		uc.recordDeniedQuery(ctx, item, schemaName, DatabaseAuditActionQuery, sqlText, safety.SQLType, safety.Message, operator)
		return nil, errors.New(safety.Message)
	}
	return uc.runAuditedQuery(ctx, item, schemaName, DatabaseAuditActionQuery, sqlText, safety.SQLType, safety.SQLText, limit, timeout, operator)
}

func (uc *UseCase) ListQueryAudits(ctx context.Context, req *DatabaseQueryAuditListRequest) ([]*DatabaseQueryAuditVO, int64, error) {
	normalizeAuditListRequest(req)
	items, total, err := uc.auditRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	instanceNames := make(map[uint]string)
	list := make([]*DatabaseQueryAuditVO, 0, len(items))
	for _, item := range items {
		if item.InstanceID > 0 {
			if _, ok := instanceNames[item.InstanceID]; !ok {
				instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
				if err == nil && instance != nil {
					instanceNames[item.InstanceID] = instance.Name
				} else {
					instanceNames[item.InstanceID] = ""
				}
			}
		}
		list = append(list, uc.toQueryAuditVO(item, instanceNames[item.InstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) ExportQueryAudits(ctx context.Context, req *DatabaseQueryAuditListRequest) ([]*DatabaseQueryAuditVO, int64, error) {
	exportReq := DatabaseQueryAuditListRequest{}
	if req != nil {
		exportReq = *req
	}
	normalizeAuditListRequest(&exportReq)
	exportReq.Page = 1
	exportReq.PageSize = 5000

	items, total, err := uc.auditRepo.List(ctx, &exportReq)
	if err != nil {
		return nil, 0, err
	}
	instanceNames := make(map[uint]string)
	list := make([]*DatabaseQueryAuditVO, 0, len(items))
	for _, item := range items {
		if item.InstanceID > 0 {
			if _, ok := instanceNames[item.InstanceID]; !ok {
				instance, err := uc.instanceRepo.GetByID(ctx, item.InstanceID)
				if err == nil && instance != nil {
					instanceNames[item.InstanceID] = instance.Name
				} else {
					instanceNames[item.InstanceID] = ""
				}
			}
		}
		list = append(list, uc.toQueryAuditVO(item, instanceNames[item.InstanceID]))
	}
	return list, total, nil
}

func (uc *UseCase) finishQueryAudit(ctx context.Context, audit *DatabaseQueryAudit, status string, rowsReturned int, durationMs int64, errorMessage string) {
	if uc.auditRepo == nil || audit == nil || audit.ID == 0 {
		return
	}
	audit.Status = status
	audit.RowsReturned = rowsReturned
	audit.DurationMs = durationMs
	audit.ErrorMessage = trimText(errorMessage, 500)
	_ = uc.auditRepo.Update(ctx, audit)
}

func (uc *UseCase) RecordInstancePermissionAudit(ctx context.Context, action string, req *DatabaseInstancePermissionAuditRequest, operator QueryOperator) error {
	if uc == nil || uc.auditRepo == nil || req == nil {
		return nil
	}
	action = normalizeAuditAction(action)
	sqlText := buildInstancePermissionAuditSQL(action, req)
	return uc.auditRepo.Create(ctx, &DatabaseQueryAudit{
		InstanceID:     req.InstanceID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    action,
		SQLText:        trimText(sqlText, 20000),
		SQLFingerprint: sqlFingerprint(sqlText),
		SQLType:        "PERMISSION",
		RiskLevel:      DatabaseQueryRiskHigh,
		Status:         DatabaseQueryStatusSuccess,
		ClientIP:       trimText(operator.ClientIP, 64),
	})
}

func buildInstancePermissionAuditSQL(action string, req *DatabaseInstancePermissionAuditRequest) string {
	if req == nil {
		return "{}"
	}
	payload := map[string]any{
		"action":            normalizeAuditAction(action),
		"roleId":            req.RoleID,
		"roleName":          strings.TrimSpace(req.RoleName),
		"roleCode":          strings.TrimSpace(req.RoleCode),
		"instanceId":        req.InstanceID,
		"instanceName":      strings.TrimSpace(req.InstanceName),
		"beforePermissions": req.BeforePermissions,
		"afterPermissions":  req.AfterPermissions,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (uc *UseCase) finishSyncJob(ctx context.Context, job *DatabaseSyncJob, status, message string, startedAt, finishedAt time.Time, schemasCount, tablesCount, columnsCount, indexesCount int) {
	if uc.syncJobRepo == nil || job == nil {
		return
	}
	job.Status = status
	job.Message = trimText(message, 500)
	job.StartedAt = &startedAt
	job.FinishedAt = &finishedAt
	job.DurationMs = finishedAt.Sub(startedAt).Milliseconds()
	job.SchemasCount = schemasCount
	job.TablesCount = tablesCount
	job.ColumnsCount = columnsCount
	job.IndexesCount = indexesCount
	_ = uc.syncJobRepo.Update(ctx, job)
}

func (uc *UseCase) validateInstanceRequest(ctx context.Context, req *DatabaseInstanceRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("实例名称不能为空")
	}
	if strings.TrimSpace(req.Host) == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	dbType := normalizeDBType(req.DBType)
	if !IsSupportedType(dbType) {
		return fmt.Errorf("不支持的数据库类型: %s", req.DBType)
	}
	if req.CredentialID == 0 {
		return fmt.Errorf("请选择连接凭据")
	}
	if uc.credentialIDExists != nil {
		if err := uc.credentialIDExists(ctx, req.CredentialID); err != nil {
			return fmt.Errorf("凭据不存在")
		}
	}
	if req.Port < 0 || req.Port > 65535 {
		return fmt.Errorf("端口范围必须为 1-65535")
	}
	if status := strings.TrimSpace(req.Status); status != "" && status != DatabaseInstanceStatusEnabled && status != DatabaseInstanceStatusDisabled {
		return fmt.Errorf("不支持的实例状态")
	}
	return nil
}

func testSQLConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return testMySQLConnection(ctx, item, credential)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return testPostgreSQLConnection(ctx, item, credential)
	case DBTypeSQLServer:
		return testSQLServerConnection(ctx, item, credential)
	case DBTypeClickHouse:
		return testClickHouseConnection(ctx, item, credential)
	case DBTypeOracle:
		return testOracleConnection(ctx, item, credential)
	case DBTypeRedis:
		return testRedisConnection(ctx, item, credential)
	case DBTypeElasticsearch, DBTypeOpenSearch:
		return testSearchConnection(ctx, item, credential)
	default:
		return "", fmt.Errorf("%s 连接测试将在后续批次接入", DBTypeText(item.DBType))
	}
}

func testMySQLConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return "", err
	}
	defer db.Close()

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(testCtx); err != nil {
		return "", fmt.Errorf("连接数据库失败: %w", err)
	}

	return readMySQLCompatibleVersion(testCtx, db, item.DBType)
}

func collectSQLMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return collectMySQLMetadata(ctx, item, credential)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return collectPostgreSQLMetadata(ctx, item, credential)
	case DBTypeSQLServer:
		return collectSQLServerMetadata(ctx, item, credential)
	case DBTypeClickHouse:
		return collectClickHouseMetadata(ctx, item, credential)
	case DBTypeOracle:
		return collectOracleMetadata(ctx, item, credential)
	default:
		return "", nil, nil, nil, nil, fmt.Errorf("%s 元数据同步将在后续批次接入", DBTypeText(item.DBType))
	}
}

func executeSQLQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return executeMySQLQuery(ctx, item, credential, schemaName, sqlType, sqlText, limit, timeoutSeconds)
	case DBTypePostgreSQL, DBTypeOpenGauss, DBTypeKingbase:
		return executePostgreSQLQuery(ctx, item, credential, schemaName, sqlType, sqlText, limit, timeoutSeconds)
	case DBTypeSQLServer:
		return executeSQLServerQuery(ctx, item, credential, schemaName, sqlType, sqlText, limit, timeoutSeconds)
	case DBTypeClickHouse:
		return executeClickHouseQuery(ctx, item, credential, schemaName, sqlType, sqlText, limit, timeoutSeconds)
	case DBTypeOracle:
		return executeOracleQuery(ctx, item, credential, schemaName, sqlType, sqlText, limit, timeoutSeconds)
	case DBTypeRedis:
		return executeRedisCommand(ctx, item, credential, schemaName, sqlText, limit, timeoutSeconds)
	default:
		return nil, fmt.Errorf("%s 查询控制台将在后续批次接入", DBTypeText(item.DBType))
	}
}

func executeMySQLQuery(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, schemaName, sqlType, sqlText string, limit, timeoutSeconds int) (*DatabaseQueryResultVO, error) {
	queryItem := *item
	if strings.TrimSpace(schemaName) != "" {
		queryItem.DefaultDatabase = strings.TrimSpace(schemaName)
	}
	db, err := openMySQLDB(&queryItem, credential)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("开启只读事务失败: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(queryCtx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	result, err := readSQLQueryResult(rows, sqlType, sqlText, limit)
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, fmt.Errorf("关闭查询结果失败: %w", closeErr)
	}
	return result, nil
}

func openMySQLDB(item *DatabaseInstance, credential *ConnectionCredential) (*sql.DB, error) {
	if credential == nil || strings.TrimSpace(credential.Username) == "" {
		return nil, fmt.Errorf("凭据用户名不能为空")
	}
	if credential.Password == "" {
		return nil, fmt.Errorf("凭据密码不能为空")
	}

	cfg := mysqlDriver.NewConfig()
	cfg.User = strings.TrimSpace(credential.Username)
	cfg.Passwd = credential.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
	cfg.DBName = strings.TrimSpace(item.DefaultDatabase)
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	if item.TLSEnabled {
		cfg.TLSConfig = "true"
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func collectMySQLMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseSchema, []*DatabaseTable, []*DatabaseColumn, []*DatabaseIndex, error) {
	db, err := openMySQLDB(item, credential)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := db.PingContext(queryCtx); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	version, err := readMySQLCompatibleVersion(queryCtx, db, item.DBType)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	schemas, err := collectMySQLSchemas(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	tables, err := collectMySQLTables(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	columns, err := collectMySQLColumns(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	indexes, err := collectMySQLIndexes(queryCtx, db)
	if err != nil {
		return "", nil, nil, nil, nil, err
	}
	return version, schemas, tables, columns, indexes, nil
}

func collectMySQLSchemas(ctx context.Context, db *sql.DB) ([]*DatabaseSchema, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	s.SCHEMA_NAME,
	COALESCE(s.DEFAULT_CHARACTER_SET_NAME, ''),
	COALESCE(s.DEFAULT_COLLATION_NAME, ''),
	COALESCE(SUM(COALESCE(t.DATA_LENGTH, 0) + COALESCE(t.INDEX_LENGTH, 0)), 0),
	COUNT(t.TABLE_NAME)
FROM information_schema.SCHEMATA s
LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
WHERE UPPER(s.SCHEMA_NAME) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')
GROUP BY s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME
ORDER BY s.SCHEMA_NAME`)
	if err != nil {
		return nil, fmt.Errorf("读取 Schema 元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseSchema, 0)
	for rows.Next() {
		var (
			item       DatabaseSchema
			sizeBytes  int64
			tableCount int64
		)
		if err := rows.Scan(&item.SchemaName, &item.Charset, &item.Collation, &sizeBytes, &tableCount); err != nil {
			return nil, fmt.Errorf("解析 Schema 元数据失败: %w", err)
		}
		item.SizeBytes = sizeBytes
		item.TableCount = int(tableCount)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Schema 元数据失败: %w", err)
	}
	return items, nil
}

func collectMySQLTables(ctx context.Context, db *sql.DB) ([]*DatabaseTable, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	TABLE_SCHEMA,
	TABLE_NAME,
	COALESCE(TABLE_TYPE, ''),
	COALESCE(ENGINE, ''),
	COALESCE(TABLE_ROWS, 0),
	COALESCE(DATA_LENGTH, 0),
	COALESCE(INDEX_LENGTH, 0),
	COALESCE(TABLE_COMMENT, '')
FROM information_schema.TABLES
WHERE UPPER(TABLE_SCHEMA) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')
ORDER BY TABLE_SCHEMA, TABLE_NAME`)
	if err != nil {
		return nil, fmt.Errorf("读取表元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseTable, 0)
	for rows.Next() {
		var item DatabaseTable
		if err := rows.Scan(
			&item.SchemaName,
			&item.Name,
			&item.TableType,
			&item.Engine,
			&item.RowCount,
			&item.DataSizeBytes,
			&item.IndexSizeBytes,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析表元数据失败: %w", err)
		}
		item.Comment = trimText(item.Comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历表元数据失败: %w", err)
	}
	return items, nil
}

func collectMySQLColumns(ctx context.Context, db *sql.DB) ([]*DatabaseColumn, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	TABLE_SCHEMA,
	TABLE_NAME,
	COLUMN_NAME,
	ORDINAL_POSITION,
	COALESCE(COLUMN_TYPE, ''),
	COALESCE(IS_NULLABLE, ''),
	COLUMN_DEFAULT,
	COALESCE(COLUMN_KEY, ''),
	COALESCE(COLUMN_COMMENT, '')
FROM information_schema.COLUMNS
WHERE UPPER(TABLE_SCHEMA) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`)
	if err != nil {
		return nil, fmt.Errorf("读取字段元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseColumn, 0)
	for rows.Next() {
		var (
			item         DatabaseColumn
			isNullable   string
			defaultValue sql.NullString
		)
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.ColumnName,
			&item.OrdinalPosition,
			&item.DataType,
			&isNullable,
			&defaultValue,
			&item.ColumnKey,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析字段元数据失败: %w", err)
		}
		item.IsNullable = strings.EqualFold(isNullable, "YES")
		item.DefaultValue = nullString(defaultValue)
		item.Comment = trimText(item.Comment, 500)
		item.IsSensitive = isSensitiveColumn(item.ColumnName)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历字段元数据失败: %w", err)
	}
	return items, nil
}

func collectMySQLIndexes(ctx context.Context, db *sql.DB) ([]*DatabaseIndex, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	TABLE_SCHEMA,
	TABLE_NAME,
	INDEX_NAME,
	COALESCE(INDEX_TYPE, ''),
	COALESCE(GROUP_CONCAT(COALESCE(COLUMN_NAME, '') ORDER BY SEQ_IN_INDEX SEPARATOR ','), ''),
	MIN(NON_UNIQUE),
	MAX(CARDINALITY),
	COALESCE(MAX(INDEX_COMMENT), '')
FROM information_schema.STATISTICS
WHERE UPPER(TABLE_SCHEMA) NOT IN ('INFORMATION_SCHEMA', 'MYSQL', 'PERFORMANCE_SCHEMA', 'SYS', 'METRICS_SCHEMA', 'INSPECTION_SCHEMA', 'OCEANBASE', '__RECYCLEBIN')
GROUP BY TABLE_SCHEMA, TABLE_NAME, INDEX_NAME, INDEX_TYPE
ORDER BY TABLE_SCHEMA, TABLE_NAME, INDEX_NAME`)
	if err != nil {
		return nil, fmt.Errorf("读取索引元数据失败: %w", err)
	}
	defer rows.Close()

	items := make([]*DatabaseIndex, 0)
	for rows.Next() {
		var (
			item        DatabaseIndex
			nonUnique   int
			cardinality sql.NullInt64
		)
		if err := rows.Scan(
			&item.SchemaName,
			&item.Table,
			&item.IndexName,
			&item.IndexType,
			&item.Columns,
			&nonUnique,
			&cardinality,
			&item.Comment,
		); err != nil {
			return nil, fmt.Errorf("解析索引元数据失败: %w", err)
		}
		item.Columns = trimText(item.Columns, 500)
		item.IsUnique = nonUnique == 0
		item.Cardinality = nullInt64(cardinality)
		item.Comment = trimText(item.Comment, 500)
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引元数据失败: %w", err)
	}
	return items, nil
}

func (uc *UseCase) toInstanceVO(item *DatabaseInstance) *DatabaseInstanceVO {
	if item == nil {
		return nil
	}
	return &DatabaseInstanceVO{
		ID:               item.ID,
		Name:             item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		Engine:           item.Engine,
		Version:          item.Version,
		Host:             item.Host,
		Port:             item.Port,
		Endpoint:         fmt.Sprintf("%s:%d", item.Host, item.Port),
		DefaultDatabase:  item.DefaultDatabase,
		CredentialID:     item.CredentialID,
		TLSEnabled:       item.TLSEnabled,
		ConnectionParams: item.ConnectionParams,
		Status:           item.Status,
		StatusText:       InstanceStatusText(item.Status),
		Environment:      item.Environment,
		BusinessSystem:   item.BusinessSystem,
		Owner:            item.Owner,
		Tags:             item.Tags,
		Remark:           item.Remark,
		LastTestAt:       formatTime(item.LastTestAt),
		LastSyncAt:       formatTime(item.LastSyncAt),
		CreatedAt:        item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) applyInstanceCapacitySummary(ctx context.Context, item *DatabaseInstance, vo *DatabaseInstanceVO) {
	if uc.capacitySnapshotRepo == nil || item == nil || vo == nil {
		return
	}
	points, err := uc.capacitySnapshotRepo.ListInstanceTrend(ctx, item.ID, time.Now().Add(-7*24*time.Hour))
	if err != nil || len(points) == 0 {
		return
	}
	last := points[len(points)-1]
	if last == nil {
		return
	}
	growthBytes, growthPercent := calculateCapacityGrowth(points)
	vo.CapacitySizeText = humanizeBytes(last.TotalSizeBytes)
	vo.CapacityGrowthText = humanizeSignedBytes(growthBytes)
	vo.CapacityGrowthPercent = growthPercent
	vo.CapacityCollectedAt = last.CollectedAt.Format("2006-01-02 15:04:05")
}

func (uc *UseCase) toSchemaVO(item *DatabaseSchema) *DatabaseSchemaVO {
	if item == nil {
		return nil
	}
	return &DatabaseSchemaVO{
		ID:         item.ID,
		InstanceID: item.InstanceID,
		SchemaName: item.SchemaName,
		Charset:    item.Charset,
		Collation:  item.Collation,
		SizeBytes:  item.SizeBytes,
		TableCount: item.TableCount,
		LastSyncAt: formatTime(item.LastSyncAt),
	}
}

func (uc *UseCase) toTableVO(item *DatabaseTable) *DatabaseTableVO {
	if item == nil {
		return nil
	}
	return &DatabaseTableVO{
		ID:             item.ID,
		InstanceID:     item.InstanceID,
		SchemaName:     item.SchemaName,
		TableName:      item.Name,
		TableType:      item.TableType,
		Engine:         item.Engine,
		RowCount:       item.RowCount,
		DataSizeBytes:  item.DataSizeBytes,
		IndexSizeBytes: item.IndexSizeBytes,
		Comment:        item.Comment,
		LastSyncAt:     formatTime(item.LastSyncAt),
	}
}

func (uc *UseCase) toColumnVO(item *DatabaseColumn) *DatabaseColumnVO {
	if item == nil {
		return nil
	}
	return &DatabaseColumnVO{
		ID:              item.ID,
		InstanceID:      item.InstanceID,
		SchemaName:      item.SchemaName,
		TableName:       item.Table,
		ColumnName:      item.ColumnName,
		OrdinalPosition: item.OrdinalPosition,
		DataType:        item.DataType,
		IsNullable:      item.IsNullable,
		DefaultValue:    item.DefaultValue,
		ColumnKey:       item.ColumnKey,
		Comment:         item.Comment,
		IsSensitive:     item.IsSensitive,
	}
}

func (uc *UseCase) toIndexVO(item *DatabaseIndex) *DatabaseIndexVO {
	if item == nil {
		return nil
	}
	return &DatabaseIndexVO{
		ID:          item.ID,
		InstanceID:  item.InstanceID,
		SchemaName:  item.SchemaName,
		TableName:   item.Table,
		IndexName:   item.IndexName,
		IndexType:   item.IndexType,
		Columns:     item.Columns,
		IsUnique:    item.IsUnique,
		Cardinality: item.Cardinality,
		Comment:     item.Comment,
	}
}

func (uc *UseCase) toQueryAuditVO(item *DatabaseQueryAudit, instanceName string) *DatabaseQueryAuditVO {
	if item == nil {
		return nil
	}
	action := resolveAuditAction(item)
	return &DatabaseQueryAuditVO{
		ID:                item.ID,
		InstanceID:        item.InstanceID,
		InstanceName:      instanceName,
		SchemaName:        item.SchemaName,
		OperatorID:        item.OperatorID,
		OperatorName:      item.OperatorName,
		Action:            action,
		ActionText:        QueryAuditActionText(action),
		SQLText:           item.SQLText,
		SQLSummary:        sqlSummary(item.SQLText),
		SQLFingerprint:    item.SQLFingerprint,
		SQLType:           item.SQLType,
		RiskLevel:         item.RiskLevel,
		RiskLevelText:     QueryRiskLevelText(item.RiskLevel),
		Status:            item.Status,
		StatusText:        QueryStatusText(item.Status),
		RowsReturned:      item.RowsReturned,
		RowsAffectedLimit: item.RowsAffectedLimit,
		RowsAffected:      item.RowsAffected,
		Reason:            item.Reason,
		ConfirmRequired:   item.ConfirmRequired,
		Confirmed:         item.Confirmed,
		RollbackSQL:       item.RollbackSQL,
		DurationMs:        item.DurationMs,
		ErrorMessage:      item.ErrorMessage,
		ClientIP:          item.ClientIP,
		CreatedAt:         item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:         item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func normalizeListRequest(req *DatabaseInstanceListRequest) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.DBType = normalizeDBType(req.DBType)
	req.Status = strings.TrimSpace(req.Status)
	req.Environment = strings.TrimSpace(req.Environment)
	req.Keyword = strings.TrimSpace(req.Keyword)
}

func normalizeAuditListRequest(req *DatabaseQueryAuditListRequest) {
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
	req.Action = normalizeAuditActionFilter(req.Action)
	req.Status = strings.TrimSpace(req.Status)
	req.RiskLevel = strings.TrimSpace(req.RiskLevel)
	req.SQLType = strings.ToUpper(strings.TrimSpace(req.SQLType))
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
}

func normalizeBackupTaskListRequest(req *DatabaseBackupTaskListRequest) {
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
	req.Enabled = strings.ToLower(strings.TrimSpace(req.Enabled))
}

func normalizeBackupRecordListRequest(req *DatabaseBackupRecordListRequest) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Status = strings.TrimSpace(req.Status)
	req.TriggerType = strings.TrimSpace(req.TriggerType)
	req.BackupMethod = strings.TrimSpace(req.BackupMethod)
	req.BackupEngine = strings.TrimSpace(req.BackupEngine)
	req.BackupScope = strings.TrimSpace(req.BackupScope)
	req.DateFrom = strings.TrimSpace(req.DateFrom)
	req.DateTo = strings.TrimSpace(req.DateTo)
}

func normalizeRestoreJobListRequest(req *DatabaseRestoreJobListRequest) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	req.Status = strings.TrimSpace(req.Status)
}

func normalizeDBType(dbType string) string {
	return strings.ToLower(strings.TrimSpace(dbType))
}

func IsSupportedType(dbType string) bool {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL, DBTypeSQLServer, DBTypeClickHouse, DBTypeOracle,
		DBTypeRedis, DBTypeMongoDB, DBTypeElasticsearch, DBTypeOpenSearch, DBTypeTiDB, DBTypeOceanBase,
		DBTypeOpenGauss, DBTypeDameng, DBTypeKingbase:
		return true
	default:
		return false
	}
}

func DefaultPort(dbType string) int {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL, DBTypeMariaDB:
		return 3306
	case DBTypePostgreSQL:
		return 5432
	case DBTypeSQLServer:
		return 1433
	case DBTypeClickHouse:
		return 9000
	case DBTypeOracle:
		return 1521
	case DBTypeRedis:
		return 6379
	case DBTypeMongoDB:
		return 27017
	case DBTypeElasticsearch, DBTypeOpenSearch:
		return 9200
	case DBTypeTiDB:
		return 4000
	case DBTypeOceanBase:
		return 2881
	case DBTypeOpenGauss:
		return 5432
	case DBTypeDameng:
		return 5236
	case DBTypeKingbase:
		return 54321
	default:
		return 0
	}
}

func DBTypeText(dbType string) string {
	switch normalizeDBType(dbType) {
	case DBTypeMySQL:
		return "MySQL"
	case DBTypeMariaDB:
		return "MariaDB"
	case DBTypePostgreSQL:
		return "PostgreSQL"
	case DBTypeSQLServer:
		return "SQL Server"
	case DBTypeClickHouse:
		return "ClickHouse"
	case DBTypeOracle:
		return "Oracle"
	case DBTypeRedis:
		return "Redis"
	case DBTypeMongoDB:
		return "MongoDB"
	case DBTypeElasticsearch:
		return "Elasticsearch"
	case DBTypeOpenSearch:
		return "OpenSearch"
	case DBTypeTiDB:
		return "TiDB"
	case DBTypeOceanBase:
		return "OceanBase MySQL"
	case DBTypeOpenGauss:
		return "openGauss"
	case DBTypeDameng:
		return "达梦 Dameng"
	case DBTypeKingbase:
		return "人大金仓 Kingbase"
	default:
		return dbType
	}
}

func InstanceStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseInstanceStatusEnabled:
		return "启用"
	case DatabaseInstanceStatusDisabled:
		return "禁用"
	default:
		return "未知"
	}
}

func QueryStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseQueryStatusPending:
		return "执行中"
	case DatabaseQueryStatusSuccess:
		return "成功"
	case DatabaseQueryStatusFailed:
		return "失败"
	case DatabaseQueryStatusDenied:
		return "已拦截"
	default:
		return "未知"
	}
}

func QueryRiskLevelText(riskLevel string) string {
	switch strings.TrimSpace(riskLevel) {
	case DatabaseQueryRiskLow:
		return "低"
	case DatabaseQueryRiskMedium:
		return "中"
	case DatabaseQueryRiskHigh:
		return "高"
	case DatabaseQueryRiskCritical:
		return "严重"
	default:
		return "未知"
	}
}

func BackupTypeText(backupType string) string {
	switch strings.TrimSpace(backupType) {
	case DatabaseBackupTypeLogical:
		return "逻辑备份"
	case DatabaseBackupTypeLogicalCustom:
		return "逻辑备份（Custom）"
	case DatabaseBackupTypePhysical:
		return "物理备份"
	case DatabaseBackupTypeExternal:
		return "外部备份"
	default:
		return strings.TrimSpace(backupType)
	}
}

func BackupStorageTypeText(storageType string) string {
	switch strings.TrimSpace(storageType) {
	case DatabaseBackupStorageLocal:
		return "本地存储"
	case DatabaseBackupStorageExternal:
		return "外部存储"
	default:
		return strings.TrimSpace(storageType)
	}
}

func BackupStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case DatabaseBackupStatusPending:
		return "待执行"
	case DatabaseBackupStatusQueued:
		return "队列中"
	case DatabaseBackupStatusRunning:
		return "执行中"
	case DatabaseBackupStatusCleaning:
		return "清理中"
	case DatabaseBackupStatusSuccess:
		return "成功"
	case DatabaseBackupStatusFailed:
		return "失败"
	case DatabaseBackupStatusExpired:
		return "已过期"
	case DatabaseRestoreStatusCancelled:
		return "已取消"
	default:
		return "未知"
	}
}

func BackupTriggerTypeText(triggerType string) string {
	switch strings.TrimSpace(triggerType) {
	case DatabaseBackupTriggerManual:
		return "手动触发"
	case DatabaseBackupTriggerSchedule:
		return "定时触发"
	case DatabaseBackupTriggerManualRetry:
		return "手动重试"
	case DatabaseBackupTriggerExternal:
		return "外部登记"
	default:
		return strings.TrimSpace(triggerType)
	}
}

func RestoreModeText(mode string) string {
	switch strings.TrimSpace(mode) {
	case DatabaseRestoreModeDryRun:
		return "恢复演练"
	case DatabaseRestoreModeIsolatedRestore:
		return "隔离恢复"
	default:
		return strings.TrimSpace(mode)
	}
}

func QueryAuditActionText(action string) string {
	switch normalizeAuditAction(action) {
	case DatabaseAuditActionQuery:
		return "只读查询"
	case DatabaseAuditActionExplain:
		return "执行计划"
	case DatabaseAuditActionWriteExplain:
		return "写 SQL 执行计划"
	case DatabaseAuditActionQueryExport:
		return "查询结果导出"
	case DatabaseAuditActionMetadataExport:
		return "数据字典导出"
	case DatabaseAuditActionDiagnosisMetrics:
		return "诊断指标查看"
	case DatabaseAuditActionDiagnosisSessions:
		return "活跃会话查看"
	case DatabaseAuditActionDiagnosisSlowQuery:
		return "慢 SQL 查看"
	case DatabaseAuditActionChangeExecute:
		return "写操作执行"
	case DatabaseAuditActionDDLExecute:
		return "DDL 结构变更"
	case DatabaseAuditActionBackupRun:
		return "逻辑备份执行"
	case DatabaseAuditActionBackupDownload:
		return "备份文件下载"
	case DatabaseAuditActionBackupVerify:
		return "备份文件校验"
	case DatabaseAuditActionTopologyView:
		return "拓扑查看"
	case DatabaseAuditActionRestoreDryRun:
		return "恢复演练"
	case DatabaseAuditActionCapacityView:
		return "容量趋势查看"
	case DatabaseAuditActionInspectionGenerate:
		return "巡检报告生成"
	case DatabaseAuditActionPermissionUpsert:
		return "实例权限保存"
	case DatabaseAuditActionPermissionDelete:
		return "实例权限删除"
	case DatabaseAuditActionReplicaStatusView:
		return "副本状态查看"
	case DatabaseAuditActionReplicaCheckRun:
		return "副本状态采集"
	default:
		return strings.TrimSpace(action)
	}
}

func normalizeAuditAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", DatabaseAuditActionQuery:
		return DatabaseAuditActionQuery
	case DatabaseAuditActionExplain:
		return DatabaseAuditActionExplain
	case DatabaseAuditActionWriteExplain:
		return DatabaseAuditActionWriteExplain
	case DatabaseAuditActionQueryExport:
		return DatabaseAuditActionQueryExport
	case DatabaseAuditActionMetadataExport:
		return DatabaseAuditActionMetadataExport
	case DatabaseAuditActionDiagnosisMetrics:
		return DatabaseAuditActionDiagnosisMetrics
	case DatabaseAuditActionDiagnosisSessions:
		return DatabaseAuditActionDiagnosisSessions
	case DatabaseAuditActionDiagnosisSlowQuery:
		return DatabaseAuditActionDiagnosisSlowQuery
	case DatabaseAuditActionChangeExecute:
		return DatabaseAuditActionChangeExecute
	case DatabaseAuditActionDDLExecute:
		return DatabaseAuditActionDDLExecute
	case DatabaseAuditActionBackupRun:
		return DatabaseAuditActionBackupRun
	case DatabaseAuditActionBackupDownload:
		return DatabaseAuditActionBackupDownload
	case DatabaseAuditActionBackupVerify:
		return DatabaseAuditActionBackupVerify
	case DatabaseAuditActionTopologyView:
		return DatabaseAuditActionTopologyView
	case DatabaseAuditActionRestoreDryRun:
		return DatabaseAuditActionRestoreDryRun
	case DatabaseAuditActionCapacityView:
		return DatabaseAuditActionCapacityView
	case DatabaseAuditActionInspectionGenerate:
		return DatabaseAuditActionInspectionGenerate
	case DatabaseAuditActionPermissionUpsert:
		return DatabaseAuditActionPermissionUpsert
	case DatabaseAuditActionPermissionDelete:
		return DatabaseAuditActionPermissionDelete
	case DatabaseAuditActionReplicaStatusView:
		return DatabaseAuditActionReplicaStatusView
	case DatabaseAuditActionReplicaCheckRun:
		return DatabaseAuditActionReplicaCheckRun
	default:
		return strings.ToLower(strings.TrimSpace(action))
	}
}

func normalizeAuditActionFilter(action string) string {
	return strings.ToLower(strings.TrimSpace(action))
}

func resolveAuditAction(item *DatabaseQueryAudit) string {
	if item == nil {
		return DatabaseAuditActionQuery
	}
	if action := strings.TrimSpace(item.AuditAction); action != "" {
		return normalizeAuditAction(action)
	}
	if strings.EqualFold(strings.TrimSpace(item.SQLType), "EXPLAIN") {
		return DatabaseAuditActionExplain
	}
	return DatabaseAuditActionQuery
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func normalizeQueryLimit(limit int, unlimited bool) int {
	if unlimited {
		return 0
	}
	if limit <= 0 {
		return 500
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func normalizeQueryTimeout(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return 30
	}
	if timeoutSeconds > 30 {
		return 30
	}
	return timeoutSeconds
}

func normalizeSQLValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return queryRawBytes(append([]byte(nil), v...))
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return v
	}
}

func sqlFingerprint(sqlText string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(sqlText)), " "))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func sqlSummary(sqlText string) string {
	return trimText(strings.Join(strings.Fields(sqlText), " "), 160)
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullInt64(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func trimText(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

func isSensitiveColumn(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	keywords := []string{
		"password",
		"passwd",
		"pwd",
		"secret",
		"token",
		"credential",
		"auth",
		"mobile",
		"phone",
		"email",
		"id_card",
		"idcard",
		"identity",
		"bank_card",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
