package database

import (
	"time"

	"gorm.io/gorm"
)

const (
	DBTypeMySQL         = "mysql"
	DBTypeMariaDB       = "mariadb"
	DBTypePostgreSQL    = "postgresql"
	DBTypeSQLServer     = "sqlserver"
	DBTypeClickHouse    = "clickhouse"
	DBTypeOracle        = "oracle"
	DBTypeRedis         = "redis"
	DBTypeMongoDB       = "mongodb"
	DBTypeElasticsearch = "elasticsearch"
	DBTypeOpenSearch    = "opensearch"
	DBTypeTiDB          = "tidb"
	DBTypeOceanBase     = "oceanbase"
	DBTypeOpenGauss     = "opengauss"
	DBTypeDameng        = "dameng"
	DBTypeKingbase      = "kingbase"

	DatabaseInstanceStatusEnabled  = "enabled"
	DatabaseInstanceStatusDisabled = "disabled"

	DatabaseSyncStatusRunning = "running"
	DatabaseSyncStatusSuccess = "success"
	DatabaseSyncStatusFailed  = "failed"

	DatabaseQueryStatusPending = "pending"
	DatabaseQueryStatusSuccess = "success"
	DatabaseQueryStatusFailed  = "failed"
	DatabaseQueryStatusDenied  = "denied"

	DatabaseQueryRiskLow      = "low"
	DatabaseQueryRiskMedium   = "medium"
	DatabaseQueryRiskHigh     = "high"
	DatabaseQueryRiskCritical = "critical"

	DatabaseAuditActionQuery              = "query"
	DatabaseAuditActionExplain            = "explain"
	DatabaseAuditActionWriteExplain       = "write_explain"
	DatabaseAuditActionQueryExport        = "query_export"
	DatabaseAuditActionMetadataExport     = "metadata_export"
	DatabaseAuditActionDiagnosisMetrics   = "diagnosis_metrics"
	DatabaseAuditActionDiagnosisSessions  = "diagnosis_sessions"
	DatabaseAuditActionDiagnosisSlowQuery = "diagnosis_slow_queries"
	DatabaseAuditActionChangeExecute      = "change_execute"
	DatabaseAuditActionDDLExecute         = "ddl_execute"
	DatabaseAuditActionBackupRun          = "backup_run"
	DatabaseAuditActionBackupDownload     = "backup_download"
	DatabaseAuditActionBackupVerify       = "backup_verify"
	DatabaseAuditActionTopologyView       = "topology_view"
	DatabaseAuditActionRestoreDryRun      = "restore_dry_run"
	DatabaseAuditActionCapacityView       = "capacity_view"
	DatabaseAuditActionInspectionGenerate = "inspection_generate"
	DatabaseAuditActionPermissionUpsert   = "instance_permission_upsert"
	DatabaseAuditActionPermissionDelete   = "instance_permission_delete"

	DatabaseBackupTypeLogical         = "logical"
	DatabaseBackupTypeLogicalCustom   = "logical_custom"
	DatabaseBackupStorageLocal        = "local"
	DatabaseBackupStatusPending       = "pending"
	DatabaseBackupStatusQueued        = "queued"
	DatabaseBackupStatusRunning       = "running"
	DatabaseBackupStatusCleaning      = "cleaning"
	DatabaseBackupStatusSuccess       = "success"
	DatabaseBackupStatusFailed        = "failed"
	DatabaseBackupStatusExpired       = "expired"
	DatabaseBackupTriggerManual       = "manual"
	DatabaseBackupTriggerSchedule     = "schedule"
	DatabaseBackupTriggerManualRetry  = "manual_retry"
	DatabaseBackupVerifyStatusPending = "pending"
	DatabaseBackupVerifyStatusSuccess = "success"
	DatabaseBackupVerifyStatusFailed  = "failed"
	DatabaseBackupVerifyStatusExpired = "expired"

	DatabaseRestoreCapabilityNone               = "none"
	DatabaseRestoreCapabilityLogicalRestoreOnly = "logical_restore_only"
	DatabaseRestoreCapabilityPhysicalRestore    = "physical_restore"
	DatabaseRestoreCapabilityPITRCapable        = "pitr_capable"
	DatabaseRestoreCapabilityPITRVerified       = "pitr_verified"

	DatabaseRestoreModeDryRun = "dry_run"

	DatabaseRestoreStrategyObjectReplace = "object_replace"
	DatabaseRestoreStrategyDatabaseClean = "database_clean"

	DatabaseCapacityObjectInstance = "instance"
	DatabaseCapacityObjectSchema   = "schema"
	DatabaseCapacityObjectTable    = "table"

	DatabaseInspectionReportManual = "manual"

	DatabasePermissionView           uint = 1 << 0
	DatabasePermissionQuery          uint = 1 << 1
	DatabasePermissionExport         uint = 1 << 2
	DatabasePermissionWrite          uint = 1 << 3
	DatabasePermissionBackup         uint = 1 << 4
	DatabasePermissionRestore        uint = 1 << 5
	DatabasePermissionDiagnosis      uint = 1 << 6
	DatabasePermissionTopology       uint = 1 << 7
	DatabasePermissionManage         uint = 1 << 8
	DatabasePermissionQueryUnlimited uint = 1 << 9
	DatabasePermissionWriteExplain   uint = 1 << 10
	DatabasePermissionDDL            uint = 1 << 11
	DatabasePermissionAll                 = DatabasePermissionView |
		DatabasePermissionQuery |
		DatabasePermissionExport |
		DatabasePermissionWrite |
		DatabasePermissionBackup |
		DatabasePermissionRestore |
		DatabasePermissionDiagnosis |
		DatabasePermissionTopology |
		DatabasePermissionManage |
		DatabasePermissionQueryUnlimited |
		DatabasePermissionWriteExplain |
		DatabasePermissionDDL
)

// DatabaseInstance 数据库实例资产
type DatabaseInstance struct {
	gorm.Model
	Name             string     `gorm:"type:varchar(100);not null;comment:实例名称" json:"name"`
	DBType           string     `gorm:"column:db_type;type:varchar(30);not null;index;comment:数据库类型" json:"dbType"`
	Engine           string     `gorm:"type:varchar(50);comment:数据库引擎" json:"engine"`
	Version          string     `gorm:"type:varchar(120);comment:数据库版本" json:"version"`
	Host             string     `gorm:"type:varchar(255);not null;comment:主机地址" json:"host"`
	Port             int        `gorm:"type:int;not null;comment:端口" json:"port"`
	DefaultDatabase  string     `gorm:"column:default_database;type:varchar(120);comment:默认库" json:"defaultDatabase"`
	CredentialID     uint       `gorm:"column:credential_id;not null;index;comment:凭据ID" json:"credentialId"`
	TLSEnabled       bool       `gorm:"column:tls_enabled;default:false;comment:是否启用TLS" json:"tlsEnabled"`
	ConnectionParams string     `gorm:"column:connection_params;type:text;comment:连接参数JSON" json:"connectionParams"`
	Status           string     `gorm:"type:varchar(20);default:'enabled';index;comment:状态 enabled/disabled" json:"status"`
	Environment      string     `gorm:"type:varchar(50);index;comment:环境" json:"environment"`
	BusinessSystem   string     `gorm:"column:business_system;type:varchar(100);comment:业务系统" json:"businessSystem"`
	Owner            string     `gorm:"type:varchar(100);comment:负责人" json:"owner"`
	Tags             string     `gorm:"type:varchar(500);comment:标签，逗号分隔" json:"tags"`
	Remark           string     `gorm:"type:varchar(500);comment:备注" json:"remark"`
	LastTestAt       *time.Time `gorm:"column:last_test_at;comment:最近测试时间" json:"lastTestAt,omitempty"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (DatabaseInstance) TableName() string {
	return "database_instances"
}

// DatabaseInstancePermission 角色到数据库实例的对象级权限。
type DatabaseInstancePermission struct {
	gorm.Model
	RoleID      uint `gorm:"column:role_id;not null;uniqueIndex:idx_database_instance_permission;comment:角色ID" json:"roleId"`
	InstanceID  uint `gorm:"column:instance_id;not null;uniqueIndex:idx_database_instance_permission;index;comment:数据库实例ID" json:"instanceId"`
	Permissions uint `gorm:"column:permissions;type:int unsigned;not null;default:1;comment:权限位图" json:"permissions"`
}

func (DatabaseInstancePermission) TableName() string {
	return "database_instance_permissions"
}

// DatabaseSchema 数据库或 Schema 元数据
type DatabaseSchema struct {
	gorm.Model
	InstanceID uint       `gorm:"column:instance_id;not null;index:idx_database_schema,unique;comment:实例ID" json:"instanceId"`
	SchemaName string     `gorm:"column:schema_name;type:varchar(150);not null;index:idx_database_schema,unique;comment:Schema名称" json:"schemaName"`
	Charset    string     `gorm:"type:varchar(80);comment:字符集" json:"charset"`
	Collation  string     `gorm:"type:varchar(120);comment:排序规则" json:"collation"`
	SizeBytes  int64      `gorm:"column:size_bytes;type:bigint;default:0;comment:容量字节" json:"sizeBytes"`
	TableCount int        `gorm:"column:table_count;type:int;default:0;comment:表数量" json:"tableCount"`
	LastSyncAt *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (DatabaseSchema) TableName() string {
	return "database_schemas"
}

// DatabaseTable 表和视图元数据
type DatabaseTable struct {
	gorm.Model
	InstanceID     uint       `gorm:"column:instance_id;not null;index:idx_database_table,unique;comment:实例ID" json:"instanceId"`
	SchemaName     string     `gorm:"column:schema_name;type:varchar(150);not null;index:idx_database_table,unique;comment:Schema名称" json:"schemaName"`
	Name           string     `gorm:"column:table_name;type:varchar(150);not null;index:idx_database_table,unique;comment:表名" json:"tableName"`
	TableType      string     `gorm:"column:table_type;type:varchar(30);comment:表类型" json:"tableType"`
	Engine         string     `gorm:"type:varchar(50);comment:引擎" json:"engine"`
	RowCount       int64      `gorm:"column:row_count;type:bigint;default:0;comment:行数估算" json:"rowCount"`
	DataSizeBytes  int64      `gorm:"column:data_size_bytes;type:bigint;default:0;comment:数据大小" json:"dataSizeBytes"`
	IndexSizeBytes int64      `gorm:"column:index_size_bytes;type:bigint;default:0;comment:索引大小" json:"indexSizeBytes"`
	Comment        string     `gorm:"type:varchar(500);comment:注释" json:"comment"`
	LastSyncAt     *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (DatabaseTable) TableName() string {
	return "database_tables"
}

// DatabaseColumn 字段元数据
type DatabaseColumn struct {
	gorm.Model
	InstanceID      uint   `gorm:"column:instance_id;not null;index:idx_database_column,unique;comment:实例ID" json:"instanceId"`
	SchemaName      string `gorm:"column:schema_name;type:varchar(150);not null;index:idx_database_column,unique;comment:Schema名称" json:"schemaName"`
	Table           string `gorm:"column:table_name;type:varchar(150);not null;index:idx_database_column,unique;comment:表名" json:"tableName"`
	ColumnName      string `gorm:"column:column_name;type:varchar(150);not null;index:idx_database_column,unique;comment:字段名" json:"columnName"`
	OrdinalPosition int    `gorm:"column:ordinal_position;type:int;default:0;comment:字段序号" json:"ordinalPosition"`
	DataType        string `gorm:"column:data_type;type:varchar(120);comment:字段类型" json:"dataType"`
	IsNullable      bool   `gorm:"column:is_nullable;default:false;comment:是否可空" json:"isNullable"`
	DefaultValue    string `gorm:"column:default_value;type:text;comment:默认值" json:"defaultValue"`
	ColumnKey       string `gorm:"column:column_key;type:varchar(50);comment:键类型" json:"columnKey"`
	Comment         string `gorm:"type:varchar(500);comment:注释" json:"comment"`
	IsSensitive     bool   `gorm:"column:is_sensitive;default:false;comment:是否敏感字段" json:"isSensitive"`
}

func (DatabaseColumn) TableName() string {
	return "database_columns"
}

// DatabaseIndex 索引元数据
type DatabaseIndex struct {
	gorm.Model
	InstanceID  uint   `gorm:"column:instance_id;not null;index:idx_database_index,unique;comment:实例ID" json:"instanceId"`
	SchemaName  string `gorm:"column:schema_name;type:varchar(150);not null;index:idx_database_index,unique;comment:Schema名称" json:"schemaName"`
	Table       string `gorm:"column:table_name;type:varchar(150);not null;index:idx_database_index,unique;comment:表名" json:"tableName"`
	IndexName   string `gorm:"column:index_name;type:varchar(150);not null;index:idx_database_index,unique;comment:索引名" json:"indexName"`
	IndexType   string `gorm:"column:index_type;type:varchar(50);comment:索引类型" json:"indexType"`
	Columns     string `gorm:"type:varchar(500);comment:索引字段，逗号分隔" json:"columns"`
	IsUnique    bool   `gorm:"column:is_unique;default:false;comment:是否唯一" json:"isUnique"`
	Cardinality int64  `gorm:"type:bigint;default:0;comment:基数" json:"cardinality"`
	Comment     string `gorm:"type:varchar(500);comment:注释" json:"comment"`
}

func (DatabaseIndex) TableName() string {
	return "database_indexes"
}

// DatabaseRedisKeyspace Redis 逻辑 DB 摘要
type DatabaseRedisKeyspace struct {
	gorm.Model
	InstanceID   uint       `gorm:"column:instance_id;not null;index:idx_database_redis_keyspace,unique;comment:实例ID" json:"instanceId"`
	DBIndex      int        `gorm:"column:db_index;type:int;not null;index:idx_database_redis_keyspace,unique;comment:逻辑DB编号" json:"dbIndex"`
	KeyspaceName string     `gorm:"column:keyspace_name;type:varchar(32);not null;index;comment:逻辑DB名称" json:"keyspaceName"`
	KeyCount     int64      `gorm:"column:key_count;type:bigint;default:0;comment:Key数量" json:"keyCount"`
	ExpiringKeys int64      `gorm:"column:expiring_keys;type:bigint;default:0;comment:设置过过期时间的Key数量" json:"expiringKeys"`
	AvgTTLMillis int64      `gorm:"column:avg_ttl_millis;type:bigint;default:0;comment:平均TTL毫秒" json:"avgTtlMillis"`
	SampledKeys  int        `gorm:"column:sampled_keys;type:int;default:0;comment:已采样Key数量" json:"sampledKeys"`
	LastSyncAt   *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (DatabaseRedisKeyspace) TableName() string {
	return "database_redis_keyspaces"
}

// DatabaseRedisKeySample Redis Key 采样元数据
type DatabaseRedisKeySample struct {
	gorm.Model
	InstanceID       uint       `gorm:"column:instance_id;not null;index:idx_database_redis_key,unique;comment:实例ID" json:"instanceId"`
	DBIndex          int        `gorm:"column:db_index;type:int;not null;index:idx_database_redis_key,unique;comment:逻辑DB编号" json:"dbIndex"`
	KeyspaceName     string     `gorm:"column:keyspace_name;type:varchar(32);not null;index;comment:逻辑DB名称" json:"keyspaceName"`
	KeyName          string     `gorm:"column:key_name;type:varchar(512);not null;index:idx_database_redis_key,unique;comment:Key名称" json:"keyName"`
	KeyType          string     `gorm:"column:key_type;type:varchar(32);comment:Key类型" json:"keyType"`
	TTLMillis        int64      `gorm:"column:ttl_millis;type:bigint;default:0;comment:TTL毫秒，-1表示不过期，-2表示不存在" json:"ttlMillis"`
	MemoryUsageBytes int64      `gorm:"column:memory_usage_bytes;type:bigint;default:0;comment:MEMORY USAGE 字节数" json:"memoryUsageBytes"`
	ValueSize        int64      `gorm:"column:value_size;type:bigint;default:0;comment:值长度或成员数量" json:"valueSize"`
	Encoding         string     `gorm:"type:varchar(64);comment:底层编码" json:"encoding"`
	Slot             int        `gorm:"type:int;default:0;comment:Cluster slot" json:"slot"`
	NodeID           string     `gorm:"column:node_id;type:varchar(128);comment:所属节点ID" json:"nodeId"`
	NodeAddress      string     `gorm:"column:node_address;type:varchar(255);comment:所属节点地址" json:"nodeAddress"`
	PreviewText      string     `gorm:"column:preview_text;type:text;comment:值预览" json:"previewText"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at;comment:最近同步时间" json:"lastSyncAt,omitempty"`
}

func (DatabaseRedisKeySample) TableName() string {
	return "database_redis_key_samples"
}

// DatabaseSyncJob 元数据同步任务
type DatabaseSyncJob struct {
	gorm.Model
	InstanceID   uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	TriggerType  string     `gorm:"column:trigger_type;type:varchar(20);default:'manual';comment:触发方式" json:"triggerType"`
	Status       string     `gorm:"type:varchar(20);default:'pending';comment:状态" json:"status"`
	Message      string     `gorm:"type:varchar(500);comment:结果信息" json:"message"`
	StartedAt    *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt   *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
	DurationMs   int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	SchemasCount int        `gorm:"column:schemas_count;type:int;default:0;comment:Schema数量" json:"schemasCount"`
	TablesCount  int        `gorm:"column:tables_count;type:int;default:0;comment:表数量" json:"tablesCount"`
	ColumnsCount int        `gorm:"column:columns_count;type:int;default:0;comment:字段数量" json:"columnsCount"`
	IndexesCount int        `gorm:"column:indexes_count;type:int;default:0;comment:索引数量" json:"indexesCount"`
}

func (DatabaseSyncJob) TableName() string {
	return "database_sync_jobs"
}

// DatabaseQueryAudit SQL 查询审计
type DatabaseQueryAudit struct {
	gorm.Model
	InstanceID        uint   `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	SchemaName        string `gorm:"column:schema_name;type:varchar(150);comment:Schema名称" json:"schemaName"`
	OperatorID        uint   `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName      string `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	AuditAction       string `gorm:"column:audit_action;type:varchar(40);index;default:'query';comment:审计动作" json:"action"`
	Reason            string `gorm:"type:varchar(500);comment:操作原因" json:"reason"`
	ConfirmRequired   bool   `gorm:"column:confirm_required;default:false;comment:是否需要确认" json:"confirmRequired"`
	Confirmed         bool   `gorm:"column:confirmed;default:false;comment:是否已确认" json:"confirmed"`
	SQLText           string `gorm:"column:sql_text;type:text;comment:SQL文本" json:"sqlText"`
	SQLFingerprint    string `gorm:"column:sql_fingerprint;type:varchar(64);index;comment:SQL指纹" json:"sqlFingerprint"`
	SQLType           string `gorm:"column:sql_type;type:varchar(30);comment:SQL类型" json:"sqlType"`
	RiskLevel         string `gorm:"column:risk_level;type:varchar(20);default:'low';comment:风险等级" json:"riskLevel"`
	Status            string `gorm:"type:varchar(20);default:'pending';comment:状态" json:"status"`
	RowsReturned      int    `gorm:"column:rows_returned;type:int;default:0;comment:返回行数" json:"rowsReturned"`
	RowsAffectedLimit int64  `gorm:"column:rows_affected_limit;type:bigint;default:0;comment:影响行数上限" json:"rowsAffectedLimit"`
	RowsAffected      int64  `gorm:"column:rows_affected;type:bigint;default:0;comment:影响行数" json:"rowsAffected"`
	DurationMs        int64  `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	RollbackSQL       string `gorm:"column:rollback_sql;type:text;comment:回滚SQL辅助" json:"rollbackSql"`
	ErrorMessage      string `gorm:"column:error_message;type:varchar(500);comment:错误信息" json:"errorMessage"`
	ClientIP          string `gorm:"column:client_ip;type:varchar(64);comment:客户端IP" json:"clientIp"`
}

func (DatabaseQueryAudit) TableName() string {
	return "database_query_audits"
}

// DatabaseBackupTask 备份任务配置
type DatabaseBackupTask struct {
	gorm.Model
	InstanceID         uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	Name               string     `gorm:"type:varchar(120);not null;comment:任务名称" json:"name"`
	BackupType         string     `gorm:"column:backup_type;type:varchar(30);default:'logical';comment:备份类型" json:"backupType"`
	Schedule           string     `gorm:"type:varchar(120);comment:Cron表达式" json:"schedule"`
	StorageType        string     `gorm:"column:storage_type;type:varchar(30);default:'local';comment:存储类型" json:"storageType"`
	StorageConfig      string     `gorm:"column:storage_config;type:text;comment:存储配置JSON" json:"storageConfig"`
	RetentionDays      int        `gorm:"column:retention_days;type:int;default:7;comment:保留天数" json:"retentionDays"`
	MaxDurationMinutes int        `gorm:"column:max_duration_minutes;type:int;default:1440;comment:最大运行时长分钟" json:"maxDurationMinutes"`
	Enabled            bool       `gorm:"default:true;comment:是否启用" json:"enabled"`
	NextRunAt          *time.Time `gorm:"column:next_run_at;comment:下次预计执行时间" json:"nextRunAt,omitempty"`
	LastRunAt          *time.Time `gorm:"column:last_run_at;comment:最近执行时间" json:"lastRunAt,omitempty"`
	LastSuccessAt      *time.Time `gorm:"column:last_success_at;comment:最近成功备份时间" json:"lastSuccessAt,omitempty"`
	LastStatus         string     `gorm:"column:last_status;type:varchar(20);comment:最近执行状态" json:"lastStatus"`
	LastMessage        string     `gorm:"column:last_message;type:varchar(500);comment:最近执行结果" json:"lastMessage"`
	RestoreCapability  string     `gorm:"column:restore_capability;type:varchar(30);default:'logical_restore_only';comment:恢复能力" json:"restoreCapability"`
}

func (DatabaseBackupTask) TableName() string {
	return "database_backup_tasks"
}

// DatabaseBackupRecord 备份执行记录
type DatabaseBackupRecord struct {
	gorm.Model
	TaskID            uint       `gorm:"column:task_id;index;comment:任务ID" json:"taskId"`
	InstanceID        uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	TriggerType       string     `gorm:"column:trigger_type;type:varchar(30);default:'manual';comment:触发方式" json:"triggerType"`
	BackupType        string     `gorm:"column:backup_type;type:varchar(30);default:'logical';comment:备份类型" json:"backupType"`
	StorageType       string     `gorm:"column:storage_type;type:varchar(30);default:'local';comment:存储类型" json:"storageType"`
	Status            string     `gorm:"type:varchar(20);default:'pending';index;comment:状态" json:"status"`
	FilePath          string     `gorm:"column:file_path;type:varchar(500);comment:文件路径" json:"filePath"`
	FileName          string     `gorm:"column:file_name;type:varchar(255);comment:文件名" json:"fileName"`
	FileSize          int64      `gorm:"column:file_size;type:bigint;default:0;comment:文件大小" json:"fileSize"`
	ChecksumSHA256    string     `gorm:"column:checksum_sha256;type:varchar(64);comment:文件SHA256校验和" json:"checksumSha256"`
	Encrypted         bool       `gorm:"default:false;comment:备份是否加密" json:"encrypted"`
	Compression       string     `gorm:"type:varchar(30);comment:压缩方式" json:"compression"`
	ExpiresAt         *time.Time `gorm:"column:expires_at;index;comment:过期时间" json:"expiresAt,omitempty"`
	VerifiedAt        *time.Time `gorm:"column:verified_at;comment:最近校验时间" json:"verifiedAt,omitempty"`
	VerifyStatus      string     `gorm:"column:verify_status;type:varchar(20);comment:校验状态" json:"verifyStatus"`
	VerifyMessage     string     `gorm:"column:verify_message;type:varchar(500);comment:校验结果" json:"verifyMessage"`
	RestoreTestedAt   *time.Time `gorm:"column:restore_tested_at;comment:最近恢复演练时间" json:"restoreTestedAt,omitempty"`
	RestoreTestStatus string     `gorm:"column:restore_test_status;type:varchar(20);comment:最近恢复演练状态" json:"restoreTestStatus"`
	StartedAt         *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	LastHeartbeatAt   *time.Time `gorm:"column:last_heartbeat_at;comment:最近心跳时间" json:"lastHeartbeatAt,omitempty"`
	FinishedAt        *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
	DurationMs        int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	ErrorMessage      string     `gorm:"column:error_message;type:varchar(500);comment:错误信息" json:"errorMessage"`
}

func (DatabaseBackupRecord) TableName() string {
	return "database_backup_records"
}

// DatabaseRestoreJob 备份恢复演练记录
type DatabaseRestoreJob struct {
	gorm.Model
	BackupRecordID   uint       `gorm:"column:backup_record_id;not null;index;comment:备份记录ID" json:"backupRecordId"`
	SourceInstanceID uint       `gorm:"column:source_instance_id;not null;index;comment:来源实例ID" json:"sourceInstanceId"`
	TargetInstanceID uint       `gorm:"column:target_instance_id;not null;index;comment:目标实例ID" json:"targetInstanceId"`
	RestoreMode      string     `gorm:"column:restore_mode;type:varchar(30);default:'dry_run';comment:恢复模式" json:"restoreMode"`
	RestoreStrategy  string     `gorm:"column:restore_strategy;type:varchar(30);default:'object_replace';comment:恢复目标处理策略" json:"restoreStrategy"`
	Status           string     `gorm:"type:varchar(20);default:'pending';index;comment:状态" json:"status"`
	FileName         string     `gorm:"column:file_name;type:varchar(255);comment:备份文件名" json:"fileName"`
	FileSize         int64      `gorm:"column:file_size;type:bigint;default:0;comment:备份文件大小" json:"fileSize"`
	OperatorID       uint       `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName     string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	StartedAt        *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt       *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
	DurationMs       int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	ErrorMessage     string     `gorm:"column:error_message;type:varchar(500);comment:错误信息" json:"errorMessage"`
}

func (DatabaseRestoreJob) TableName() string {
	return "database_restore_jobs"
}

// DatabaseCapacitySnapshot 容量采样快照
type DatabaseCapacitySnapshot struct {
	gorm.Model
	InstanceID     uint      `gorm:"column:instance_id;not null;index:idx_database_capacity_instance_time;comment:实例ID" json:"instanceId"`
	ObjectType     string    `gorm:"column:object_type;type:varchar(30);not null;index;comment:对象类型 instance/schema/table" json:"objectType"`
	SchemaName     string    `gorm:"column:schema_name;type:varchar(150);index;comment:Schema名称" json:"schemaName"`
	Table          string    `gorm:"column:table_name;type:varchar(150);index;comment:表名" json:"tableName"`
	SchemaCount    int       `gorm:"column:schema_count;type:int;default:0;comment:Schema数量" json:"schemaCount"`
	TableCount     int       `gorm:"column:table_count;type:int;default:0;comment:表数量" json:"tableCount"`
	RowCount       int64     `gorm:"column:row_count;type:bigint;default:0;comment:行数估算" json:"rowCount"`
	DataSizeBytes  int64     `gorm:"column:data_size_bytes;type:bigint;default:0;comment:数据大小" json:"dataSizeBytes"`
	IndexSizeBytes int64     `gorm:"column:index_size_bytes;type:bigint;default:0;comment:索引大小" json:"indexSizeBytes"`
	TotalSizeBytes int64     `gorm:"column:total_size_bytes;type:bigint;default:0;comment:总容量" json:"totalSizeBytes"`
	CollectedAt    time.Time `gorm:"column:collected_at;not null;index:idx_database_capacity_instance_time;comment:采样时间" json:"collectedAt"`
}

func (DatabaseCapacitySnapshot) TableName() string {
	return "database_capacity_snapshots"
}

// DatabaseInspectionReport 数据库巡检报告
type DatabaseInspectionReport struct {
	gorm.Model
	InstanceID         uint       `gorm:"column:instance_id;not null;index;comment:实例ID" json:"instanceId"`
	ReportType         string     `gorm:"column:report_type;type:varchar(30);default:'manual';comment:报告类型" json:"reportType"`
	Status             string     `gorm:"type:varchar(20);default:'pending';index;comment:状态" json:"status"`
	HealthScore        int        `gorm:"column:health_score;type:int;default:0;comment:健康分" json:"healthScore"`
	RiskLevel          string     `gorm:"column:risk_level;type:varchar(20);default:'low';index;comment:风险等级" json:"riskLevel"`
	Summary            string     `gorm:"type:varchar(500);comment:摘要" json:"summary"`
	CapacitySummary    string     `gorm:"column:capacity_summary;type:text;comment:容量摘要JSON" json:"capacitySummary"`
	PerformanceSummary string     `gorm:"column:performance_summary;type:text;comment:性能摘要JSON" json:"performanceSummary"`
	SecuritySummary    string     `gorm:"column:security_summary;type:text;comment:安全摘要JSON" json:"securitySummary"`
	BackupSummary      string     `gorm:"column:backup_summary;type:text;comment:备份摘要JSON" json:"backupSummary"`
	Findings           string     `gorm:"type:text;comment:异常项JSON" json:"findings"`
	OperatorID         uint       `gorm:"column:operator_id;index;comment:操作人ID" json:"operatorId"`
	OperatorName       string     `gorm:"column:operator_name;type:varchar(100);comment:操作人" json:"operatorName"`
	GeneratedAt        *time.Time `gorm:"column:generated_at;comment:生成时间" json:"generatedAt,omitempty"`
	DurationMs         int64      `gorm:"column:duration_ms;type:bigint;default:0;comment:耗时毫秒" json:"durationMs"`
	ErrorMessage       string     `gorm:"column:error_message;type:varchar(500);comment:错误信息" json:"errorMessage"`
}

func (DatabaseInspectionReport) TableName() string {
	return "database_inspection_reports"
}
