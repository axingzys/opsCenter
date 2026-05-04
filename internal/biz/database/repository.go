package database

import (
	"context"
	"time"
)

type InstanceRepo interface {
	Create(ctx context.Context, item *DatabaseInstance) error
	Update(ctx context.Context, item *DatabaseInstance) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseInstance, error)
	List(ctx context.Context, req *DatabaseInstanceListRequest) ([]*DatabaseInstance, int64, error)
	ListEnabled(ctx context.Context) ([]*DatabaseInstance, error)
}

type InstanceReplicaRepo interface {
	Create(ctx context.Context, item *DatabaseInstanceReplica) error
	Update(ctx context.Context, item *DatabaseInstanceReplica) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseInstanceReplica, error)
	GetByReplicaInstanceID(ctx context.Context, replicaInstanceID uint) (*DatabaseInstanceReplica, error)
	UpsertByReplicaInstance(ctx context.Context, item *DatabaseInstanceReplica) (*DatabaseInstanceReplica, error)
	List(ctx context.Context, req *DatabaseInstanceReplicaListRequest) ([]*DatabaseInstanceReplica, int64, error)
}

type ReplicationCheckRepo interface {
	Create(ctx context.Context, item *DatabaseReplicationCheck) error
	Update(ctx context.Context, item *DatabaseReplicationCheck) error
	GetByID(ctx context.Context, id uint) (*DatabaseReplicationCheck, error)
	LatestByInstanceID(ctx context.Context, instanceID uint) (*DatabaseReplicationCheck, error)
	List(ctx context.Context, req *DatabaseReplicationCheckListRequest) ([]*DatabaseReplicationCheck, int64, error)
}

type ReplicaIncidentGuideRepo interface {
	Create(ctx context.Context, item *DatabaseReplicaIncidentGuide) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseReplicaIncidentGuide, error)
	List(ctx context.Context, req *DatabaseReplicaIncidentGuideListRequest) ([]*DatabaseReplicaIncidentGuide, int64, error)
}

type ReplicaActionRepo interface {
	Create(ctx context.Context, item *DatabaseReplicaAction) error
	Update(ctx context.Context, item *DatabaseReplicaAction) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseReplicaAction, error)
	List(ctx context.Context, req *DatabaseReplicaActionListRequest) ([]*DatabaseReplicaAction, int64, error)
}

type DatabasePermissionRepo interface {
	HasAnyRules(ctx context.Context) (bool, error)
	IsAdmin(ctx context.Context, userID uint) (bool, error)
	GetUserInstancePermissions(ctx context.Context, userID, instanceID uint) (uint, error)
	GetUserAccessibleInstanceIDs(ctx context.Context, userID uint, required uint) ([]uint, error)
	List(ctx context.Context, req *DatabaseInstancePermissionListRequest) ([]*DatabaseInstancePermissionVO, int64, error)
	GetByID(ctx context.Context, id uint) (*DatabaseInstancePermissionVO, error)
	GetByRoleInstance(ctx context.Context, roleID, instanceID uint) (*DatabaseInstancePermissionVO, error)
	ValidateTarget(ctx context.Context, roleID, instanceID uint) error
	Upsert(ctx context.Context, item *DatabaseInstancePermission) error
	Delete(ctx context.Context, id uint) error
}

type SchemaRepo interface {
	ListByInstanceID(ctx context.Context, instanceID uint) ([]*DatabaseSchema, error)
}

type TableRepo interface {
	List(ctx context.Context, instanceID uint, schemaName string) ([]*DatabaseTable, error)
	Get(ctx context.Context, instanceID uint, schemaName, tableName string) (*DatabaseTable, error)
}

type ColumnRepo interface {
	List(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*DatabaseColumn, error)
}

type IndexRepo interface {
	List(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*DatabaseIndex, error)
}

type TableRelationRepo interface {
	List(ctx context.Context, req *DatabaseTableRelationListRequest) ([]*DatabaseTableRelation, error)
}

type MetadataRepo interface {
	ReplaceAll(ctx context.Context, instanceID uint, schemas []*DatabaseSchema, tables []*DatabaseTable, columns []*DatabaseColumn, indexes []*DatabaseIndex, relations []*DatabaseTableRelation) error
}

type RedisMetadataRepo interface {
	ReplaceAll(ctx context.Context, instanceID uint, keyspaces []*DatabaseRedisKeyspace, keys []*DatabaseRedisKeySample) error
	ListKeyspaces(ctx context.Context, instanceID uint) ([]*DatabaseRedisKeyspace, error)
	ListKeys(ctx context.Context, instanceID uint, dbIndex int) ([]*DatabaseRedisKeySample, error)
	GetKey(ctx context.Context, instanceID uint, dbIndex int, keyName string) (*DatabaseRedisKeySample, error)
}

type SyncJobRepo interface {
	Create(ctx context.Context, item *DatabaseSyncJob) error
	Update(ctx context.Context, item *DatabaseSyncJob) error
}

type QueryAuditRepo interface {
	Create(ctx context.Context, item *DatabaseQueryAudit) error
	Update(ctx context.Context, item *DatabaseQueryAudit) error
	List(ctx context.Context, req *DatabaseQueryAuditListRequest) ([]*DatabaseQueryAudit, int64, error)
	ListHistory(ctx context.Context, operatorID uint, req *DatabaseQueryHistoryRequest) ([]*DatabaseQueryAudit, error)
}

type BackupTaskRepo interface {
	Create(ctx context.Context, item *DatabaseBackupTask) error
	Update(ctx context.Context, item *DatabaseBackupTask) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseBackupTask, error)
	List(ctx context.Context, req *DatabaseBackupTaskListRequest) ([]*DatabaseBackupTask, int64, error)
	ListAll(ctx context.Context) ([]*DatabaseBackupTask, error)
	ListEnabled(ctx context.Context) ([]*DatabaseBackupTask, error)
}

type BackupRecordRepo interface {
	Create(ctx context.Context, item *DatabaseBackupRecord) error
	Update(ctx context.Context, item *DatabaseBackupRecord) error
	GetByID(ctx context.Context, id uint) (*DatabaseBackupRecord, error)
	GetByExternalBackup(ctx context.Context, instanceID uint, backupEngine, externalServerName, externalBackupID string) (*DatabaseBackupRecord, error)
	List(ctx context.Context, req *DatabaseBackupRecordListRequest) ([]*DatabaseBackupRecord, int64, error)
	ListExpiredSuccessByTask(ctx context.Context, taskID uint, before time.Time) ([]*DatabaseBackupRecord, error)
	ListSuccessfulForRestore(ctx context.Context, instanceID uint, targetTime *time.Time) ([]*DatabaseBackupRecord, error)
	ListSuccessfulPhysicalByPolicy(ctx context.Context, policyID uint) ([]*DatabaseBackupRecord, error)
}

type BackupPolicyConfigRepo interface {
	Create(ctx context.Context, item *DatabaseBackupPolicyConfig) error
	Update(ctx context.Context, item *DatabaseBackupPolicyConfig) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseBackupPolicyConfig, error)
	List(ctx context.Context, req *DatabaseBackupPolicyListRequest) ([]*DatabaseBackupPolicyConfig, int64, error)
	ListEnabled(ctx context.Context) ([]*DatabaseBackupPolicyConfig, error)
}

type BackupChainStateRepo interface {
	Create(ctx context.Context, item *DatabaseBackupChainState) error
	Update(ctx context.Context, item *DatabaseBackupChainState) error
	GetByPolicyID(ctx context.Context, policyID uint) (*DatabaseBackupChainState, error)
}

type BackupAlertRuleRepo interface {
	Create(ctx context.Context, item *DatabaseBackupAlertRule) error
	Update(ctx context.Context, item *DatabaseBackupAlertRule) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseBackupAlertRule, error)
	List(ctx context.Context, req *DatabaseBackupAlertRuleListRequest) ([]*DatabaseBackupAlertRule, int64, error)
	ListEnabled(ctx context.Context) ([]*DatabaseBackupAlertRule, error)
}

type BackupAlertStateRepo interface {
	Create(ctx context.Context, item *DatabaseBackupAlertState) error
	Update(ctx context.Context, item *DatabaseBackupAlertState) error
	GetByID(ctx context.Context, id uint) (*DatabaseBackupAlertState, error)
	GetByFingerprint(ctx context.Context, fingerprint string) (*DatabaseBackupAlertState, error)
	List(ctx context.Context, req *DatabaseBackupAlertStateListRequest) ([]*DatabaseBackupAlertState, int64, error)
	ListFiringByRuleID(ctx context.Context, ruleID uint) ([]*DatabaseBackupAlertState, error)
}

type RestoreJobRepo interface {
	Create(ctx context.Context, item *DatabaseRestoreJob) error
	Update(ctx context.Context, item *DatabaseRestoreJob) error
	GetByID(ctx context.Context, id uint) (*DatabaseRestoreJob, error)
	List(ctx context.Context, req *DatabaseRestoreJobListRequest) ([]*DatabaseRestoreJob, int64, error)
}

type CapacitySnapshotRepo interface {
	CreateBatch(ctx context.Context, items []*DatabaseCapacitySnapshot) error
	ListInstanceTrend(ctx context.Context, instanceID uint, since time.Time) ([]*DatabaseCapacitySnapshot, error)
	LatestInstance(ctx context.Context, instanceID uint) (*DatabaseCapacitySnapshot, error)
	LatestTopObjects(ctx context.Context, instanceID uint, objectType string, limit int) ([]*DatabaseCapacitySnapshot, error)
}

type InspectionReportRepo interface {
	Create(ctx context.Context, item *DatabaseInspectionReport) error
	Update(ctx context.Context, item *DatabaseInspectionReport) error
	GetByID(ctx context.Context, id uint) (*DatabaseInspectionReport, error)
	List(ctx context.Context, req *DatabaseInspectionReportListRequest) ([]*DatabaseInspectionReport, int64, error)
}

type LogArchiveStreamRepo interface {
	Create(ctx context.Context, item *DatabaseLogArchiveStream) error
	Update(ctx context.Context, item *DatabaseLogArchiveStream) error
	GetByID(ctx context.Context, id uint) (*DatabaseLogArchiveStream, error)
	List(ctx context.Context, req *DatabaseLogArchiveStreamListRequest) ([]*DatabaseLogArchiveStream, int64, error)
	ListRunnableForRunner(ctx context.Context, runnerHostID uint) ([]*DatabaseLogArchiveStream, error)
	TryAcquireLease(ctx context.Context, streamID, runnerHostID uint, runnerID string, now, leaseExpiresAt time.Time) (bool, error)
}

type LogArchiveRepo interface {
	Create(ctx context.Context, item *DatabaseLogArchive) error
	Update(ctx context.Context, item *DatabaseLogArchive) error
	GetByID(ctx context.Context, id uint) (*DatabaseLogArchive, error)
	GetByStreamFile(ctx context.Context, streamID uint, fileName string) (*DatabaseLogArchive, error)
	List(ctx context.Context, req *DatabaseLogArchiveListRequest) ([]*DatabaseLogArchive, int64, error)
	ListCoveringTimeRange(ctx context.Context, instanceID uint, archiveType string, startTime, endTime time.Time) ([]*DatabaseLogArchive, error)
}

type LogArchiveEventRepo interface {
	Create(ctx context.Context, item *DatabaseLogArchiveEvent) error
	List(ctx context.Context, req *DatabaseLogArchiveEventListRequest) ([]*DatabaseLogArchiveEvent, int64, error)
	UpsertRollup(ctx context.Context, item *DatabaseLogArchiveEventRollup) error
	DeleteBefore(ctx context.Context, before time.Time, streamID uint) (int64, error)
	DeleteHighFrequencyBefore(ctx context.Context, before time.Time, streamID uint, eventTypes []string) (int64, error)
}

type RestorePlanRepo interface {
	Create(ctx context.Context, item *DatabaseRestorePlan) error
	Update(ctx context.Context, item *DatabaseRestorePlan) error
	GetByID(ctx context.Context, id uint) (*DatabaseRestorePlan, error)
	List(ctx context.Context, req *DatabaseRestorePlanListRequest) ([]*DatabaseRestorePlan, int64, error)
}

type StorageProfileRepo interface {
	Create(ctx context.Context, item *DatabaseStorageProfile) error
	Update(ctx context.Context, item *DatabaseStorageProfile) error
	GetByID(ctx context.Context, id uint) (*DatabaseStorageProfile, error)
	List(ctx context.Context, req *DatabaseStorageProfileListRequest) ([]*DatabaseStorageProfile, int64, error)
}

type SecretProfileRepo interface {
	Create(ctx context.Context, item *DatabaseSecretProfile) error
	GetByID(ctx context.Context, id uint) (*DatabaseSecretProfile, error)
	List(ctx context.Context, req *DatabaseSecretProfileListRequest) ([]*DatabaseSecretProfile, int64, error)
}

type RunnerHostRepo interface {
	Create(ctx context.Context, item *DatabaseRunnerHost) error
	Update(ctx context.Context, item *DatabaseRunnerHost) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseRunnerHost, error)
	List(ctx context.Context, req *DatabaseRunnerHostListRequest) ([]*DatabaseRunnerHost, int64, error)
	CountDeleteBlockers(ctx context.Context, id uint) (*DatabaseRunnerHostDeleteBlockers, error)
}

type RunnerToolProfileRepo interface {
	UpsertByRunnerHostID(ctx context.Context, item *DatabaseRunnerToolProfile) error
	GetByRunnerHostID(ctx context.Context, runnerHostID uint) (*DatabaseRunnerToolProfile, error)
}

type RunnerToolOfflinePackageRepo interface {
	Create(ctx context.Context, item *DatabaseRunnerToolOfflinePackage) error
	Update(ctx context.Context, item *DatabaseRunnerToolOfflinePackage) error
	GetByID(ctx context.Context, id uint) (*DatabaseRunnerToolOfflinePackage, error)
	List(ctx context.Context, req *DatabaseRunnerToolOfflinePackageListRequest) ([]*DatabaseRunnerToolOfflinePackage, int64, error)
}

type RunnerJobRepo interface {
	Create(ctx context.Context, item *DatabaseRunnerJob) error
	Update(ctx context.Context, item *DatabaseRunnerJob) error
	GetByID(ctx context.Context, id uint) (*DatabaseRunnerJob, error)
	List(ctx context.Context, req *DatabaseRunnerJobListRequest) ([]*DatabaseRunnerJob, int64, error)
}

type BarmanServerRepo interface {
	Create(ctx context.Context, item *DatabaseBarmanServer) error
	Update(ctx context.Context, item *DatabaseBarmanServer) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*DatabaseBarmanServer, error)
	List(ctx context.Context, req *DatabaseBarmanServerListRequest) ([]*DatabaseBarmanServer, int64, error)
}
