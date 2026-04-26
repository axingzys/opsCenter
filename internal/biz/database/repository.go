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

type MetadataRepo interface {
	ReplaceAll(ctx context.Context, instanceID uint, schemas []*DatabaseSchema, tables []*DatabaseTable, columns []*DatabaseColumn, indexes []*DatabaseIndex) error
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
	List(ctx context.Context, req *DatabaseBackupRecordListRequest) ([]*DatabaseBackupRecord, int64, error)
	ListExpiredSuccessByTask(ctx context.Context, taskID uint, before time.Time) ([]*DatabaseBackupRecord, error)
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
