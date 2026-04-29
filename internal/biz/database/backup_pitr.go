package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DatabaseStorageProfileListRequest struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	Keyword     string `form:"keyword"`
	StorageType string `form:"storageType"`
	Status      string `form:"status"`
}

type DatabaseStorageProfileRequest struct {
	Name                string `json:"name" binding:"required,max=120"`
	StorageType         string `json:"storageType" binding:"required,max=30"`
	Endpoint            string `json:"endpoint" binding:"omitempty,max=255"`
	Bucket              string `json:"bucket" binding:"omitempty,max=255"`
	Region              string `json:"region" binding:"omitempty,max=120"`
	PathPrefix          string `json:"pathPrefix" binding:"omitempty,max=500"`
	SecretProfileID     uint   `json:"secretProfileId"`
	VersioningEnabled   bool   `json:"versioningEnabled"`
	ImmutabilityEnabled bool   `json:"immutabilityEnabled"`
	KMSKeyID            string `json:"kmsKeyId" binding:"omitempty,max=255"`
	RetentionLockDays   int    `json:"retentionLockDays" binding:"omitempty,min=0,max=3650"`
	Status              string `json:"status" binding:"omitempty,max=30"`
}

type DatabaseStorageProfilePostureCheckRequest struct {
	AccessKey          string `json:"accessKey" binding:"omitempty,max=255"`
	SecretKey          string `json:"secretKey" binding:"omitempty,max=255"`
	SessionToken       string `json:"sessionToken" binding:"omitempty,max=2048"`
	UseSSL             *bool  `json:"useSsl"`
	UsePathStyle       *bool  `json:"usePathStyle"`
	InsecureSkipVerify *bool  `json:"insecureSkipVerify"`
}

type DatabaseStorageProfileVO struct {
	ID                  uint   `json:"id"`
	Name                string `json:"name"`
	StorageType         string `json:"storageType"`
	StorageTypeText     string `json:"storageTypeText"`
	Endpoint            string `json:"endpoint"`
	Bucket              string `json:"bucket"`
	Region              string `json:"region"`
	PathPrefix          string `json:"pathPrefix"`
	SecretProfileID     uint   `json:"secretProfileId"`
	VersioningEnabled   bool   `json:"versioningEnabled"`
	ImmutabilityEnabled bool   `json:"immutabilityEnabled"`
	KMSKeyID            string `json:"kmsKeyId"`
	RetentionLockDays   int    `json:"retentionLockDays"`
	Status              string `json:"status"`
	StatusText          string `json:"statusText"`
	LastTestAt          string `json:"lastTestAt"`
	PostureStatus       string `json:"postureStatus"`
	PostureStatusText   string `json:"postureStatusText"`
	PostureSummary      string `json:"postureSummary"`
	PostureJSON         string `json:"postureJson"`
	LastPostureCheckAt  string `json:"lastPostureCheckAt"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type DatabaseSecretProfileListRequest struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	Keyword    string `form:"keyword"`
	SecretType string `form:"secretType"`
	Status     string `form:"status"`
}

type DatabaseSecretProfileRequest struct {
	Name         string `json:"name" binding:"required,max=120"`
	SecretType   string `json:"secretType" binding:"required,max=30"`
	CredentialID uint   `json:"credentialId"`
	ExternalRef  string `json:"externalRef" binding:"omitempty,max=500"`
	Status       string `json:"status" binding:"omitempty,max=30"`
}

type DatabaseSecretProfileVO struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	SecretType     string `json:"secretType"`
	SecretTypeText string `json:"secretTypeText"`
	CredentialID   uint   `json:"credentialId"`
	ExternalRef    string `json:"externalRef"`
	Status         string `json:"status"`
	StatusText     string `json:"statusText"`
	LastRotatedAt  string `json:"lastRotatedAt"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type DatabaseLogArchiveStreamListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	InstanceID         uint   `form:"instanceId"`
	ArchiveType        string `form:"archiveType"`
	Status             string `form:"status"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseLogArchiveStreamRequest struct {
	InstanceID       uint   `json:"instanceId" binding:"required"`
	SourceInstanceID uint   `json:"sourceInstanceId"`
	Engine           string `json:"engine" binding:"omitempty,max=30"`
	ArchiveType      string `json:"archiveType" binding:"omitempty,max=30"`
	ArchiveMode      string `json:"archiveMode" binding:"omitempty,max=30"`
	ArchiveEngine    string `json:"archiveEngine" binding:"omitempty,max=60"`
	RunnerHostID     uint   `json:"runnerHostId"`
	StorageProfileID uint   `json:"storageProfileId"`
	SecretProfileID  uint   `json:"secretProfileId"`
	RPOTargetSeconds int    `json:"rpoTargetSeconds" binding:"omitempty,min=0,max=86400"`
	RetentionDays    int    `json:"retentionDays" binding:"omitempty,min=1,max=3650"`
	Enabled          bool   `json:"enabled"`
	ConfigJSON       string `json:"configJson" binding:"omitempty,max=4000"`
}

type DatabaseLogArchiveStreamControlRequest struct {
	RunnerHostID uint   `json:"runnerHostId"`
	ArchiveMode  string `json:"archiveMode" binding:"omitempty,max=30"`
	Reason       string `json:"reason" binding:"omitempty,max=500"`
}

type DatabaseLogArchiveStreamVO struct {
	ID                  uint   `json:"id"`
	InstanceID          uint   `json:"instanceId"`
	InstanceName        string `json:"instanceName"`
	SourceInstanceID    uint   `json:"sourceInstanceId"`
	SourceInstanceName  string `json:"sourceInstanceName"`
	Engine              string `json:"engine"`
	EngineText          string `json:"engineText"`
	ArchiveType         string `json:"archiveType"`
	ArchiveTypeText     string `json:"archiveTypeText"`
	ArchiveMode         string `json:"archiveMode"`
	ArchiveModeText     string `json:"archiveModeText"`
	ArchiveEngine       string `json:"archiveEngine"`
	RunnerHostID        uint   `json:"runnerHostId"`
	RunnerHostName      string `json:"runnerHostName"`
	StorageProfileID    uint   `json:"storageProfileId"`
	SecretProfileID     uint   `json:"secretProfileId"`
	RPOTargetSeconds    int    `json:"rpoTargetSeconds"`
	RetentionDays       int    `json:"retentionDays"`
	Enabled             bool   `json:"enabled"`
	Status              string `json:"status"`
	StatusText          string `json:"statusText"`
	DesiredState        string `json:"desiredState"`
	DesiredStateText    string `json:"desiredStateText"`
	DaemonStatus        string `json:"daemonStatus"`
	DaemonStatusText    string `json:"daemonStatusText"`
	CursorFile          string `json:"cursorFile"`
	CursorPos           int64  `json:"cursorPos"`
	CursorGTIDSet       string `json:"cursorGtidSet"`
	ActiveFile          string `json:"activeFile"`
	LastSourceFile      string `json:"lastSourceFile"`
	LastSourcePos       int64  `json:"lastSourcePos"`
	LastEventTime       string `json:"lastEventTime"`
	ArchiveLagSeconds   int    `json:"archiveLagSeconds"`
	LastHeartbeatAt     string `json:"lastHeartbeatAt"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LeaseOwner          string `json:"leaseOwner"`
	LeaseExpiresAt      string `json:"leaseExpiresAt"`
	PausedAt            string `json:"pausedAt"`
	PausedReason        string `json:"pausedReason"`
	LastArchivedAt      string `json:"lastArchivedAt"`
	LastArchiveName     string `json:"lastArchiveName"`
	LastError           string `json:"lastError"`
	ConfigJSON          string `json:"configJson"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type DatabaseLogArchiveListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	StreamID           uint   `form:"streamId"`
	InstanceID         uint   `form:"instanceId"`
	ArchiveType        string `form:"archiveType"`
	Status             string `form:"status"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseExternalLogArchiveRequest struct {
	StreamID           uint   `json:"streamId" binding:"required"`
	FileName           string `json:"fileName" binding:"required,max=255"`
	StorageURI         string `json:"storageUri" binding:"required,max=1000"`
	FileSize           int64  `json:"fileSize" binding:"omitempty,min=0"`
	ChecksumSHA256     string `json:"checksumSha256" binding:"omitempty,max=64"`
	FirstEventTime     string `json:"firstEventTime"`
	LastEventTime      string `json:"lastEventTime"`
	Status             string `json:"status" binding:"omitempty,max=30"`
	ServerUUID         string `json:"serverUuid" binding:"omitempty,max=120"`
	ServerID           string `json:"serverId" binding:"omitempty,max=60"`
	StartPos           int64  `json:"startPos"`
	EndPos             int64  `json:"endPos"`
	StartGTIDSet       string `json:"startGtidSet"`
	EndGTIDSet         string `json:"endGtidSet"`
	PreviousFileName   string `json:"previousFileName" binding:"omitempty,max=255"`
	NextFileName       string `json:"nextFileName" binding:"omitempty,max=255"`
	PGSystemIdentifier string `json:"pgSystemIdentifier" binding:"omitempty,max=120"`
	TimelineID         string `json:"timelineId" binding:"omitempty,max=60"`
	StartLSN           string `json:"startLsn" binding:"omitempty,max=120"`
	EndLSN             string `json:"endLsn" binding:"omitempty,max=120"`
	SegmentNo          string `json:"segmentNo" binding:"omitempty,max=120"`
	TimelineHistoryURI string `json:"timelineHistoryUri" binding:"omitempty,max=1000"`
}

type DatabaseLogArchiveVO struct {
	ID                 uint   `json:"id"`
	StreamID           uint   `json:"streamId"`
	InstanceID         uint   `json:"instanceId"`
	InstanceName       string `json:"instanceName"`
	SourceInstanceID   uint   `json:"sourceInstanceId"`
	SourceInstanceName string `json:"sourceInstanceName"`
	Engine             string `json:"engine"`
	EngineText         string `json:"engineText"`
	ArchiveType        string `json:"archiveType"`
	ArchiveTypeText    string `json:"archiveTypeText"`
	FileName           string `json:"fileName"`
	StorageURI         string `json:"storageUri"`
	FileSize           int64  `json:"fileSize"`
	ChecksumSHA256     string `json:"checksumSha256"`
	FirstEventTime     string `json:"firstEventTime"`
	LastEventTime      string `json:"lastEventTime"`
	Status             string `json:"status"`
	StatusText         string `json:"statusText"`
	ArchivedAt         string `json:"archivedAt"`
	PGSystemIdentifier string `json:"pgSystemIdentifier"`
	TimelineID         string `json:"timelineId"`
	WALSegmentSize     int64  `json:"walSegmentSize"`
	ExternalServerName string `json:"externalServerName"`
	StartLSN           string `json:"startLsn"`
	EndLSN             string `json:"endLsn"`
	SegmentNo          string `json:"segmentNo"`
	TimelineHistoryURI string `json:"timelineHistoryUri"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type DatabaseLogArchiveEventListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	StreamID           uint   `form:"streamId"`
	InstanceID         uint   `form:"instanceId"`
	RunnerHostID       uint   `form:"runnerHostId"`
	Level              string `form:"level"`
	EventType          string `form:"eventType"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseRunnerAgentEventRequest struct {
	StreamID          uint   `json:"streamId"`
	EventType         string `json:"eventType" binding:"omitempty,max=60"`
	Level             string `json:"level" binding:"omitempty,max=20"`
	Message           string `json:"message" binding:"omitempty,max=1000"`
	FileName          string `json:"fileName" binding:"omitempty,max=255"`
	CursorFile        string `json:"cursorFile" binding:"omitempty,max=255"`
	CursorPos         int64  `json:"cursorPos"`
	ActiveFile        string `json:"activeFile" binding:"omitempty,max=255"`
	ArchiveLagSeconds int    `json:"archiveLagSeconds" binding:"omitempty,min=0"`
	PayloadJSON       string `json:"payloadJson" binding:"omitempty,max=4000"`
	OccurredAt        string `json:"occurredAt"`
}

type DatabaseLogArchiveEventVO struct {
	ID                 uint   `json:"id"`
	StreamID           uint   `json:"streamId"`
	InstanceID         uint   `json:"instanceId"`
	InstanceName       string `json:"instanceName"`
	SourceInstanceID   uint   `json:"sourceInstanceId"`
	SourceInstanceName string `json:"sourceInstanceName"`
	RunnerHostID       uint   `json:"runnerHostId"`
	RunnerHostName     string `json:"runnerHostName"`
	RunnerID           string `json:"runnerId"`
	EventType          string `json:"eventType"`
	EventTypeText      string `json:"eventTypeText"`
	Level              string `json:"level"`
	LevelText          string `json:"levelText"`
	Message            string `json:"message"`
	FileName           string `json:"fileName"`
	CursorFile         string `json:"cursorFile"`
	CursorPos          int64  `json:"cursorPos"`
	ActiveFile         string `json:"activeFile"`
	ArchiveLagSeconds  int    `json:"archiveLagSeconds"`
	PayloadJSON        string `json:"payloadJson"`
	OccurredAt         string `json:"occurredAt"`
	CreatedAt          string `json:"createdAt"`
}

type DatabaseExternalBackupRecordRequest struct {
	TaskID                 uint   `json:"taskId"`
	InstanceID             uint   `json:"instanceId" binding:"required"`
	SourceInstanceID       uint   `json:"sourceInstanceId"`
	SourceRole             string `json:"sourceRole" binding:"omitempty,max=30"`
	ChainID                string `json:"chainId" binding:"omitempty,max=64"`
	BaseRecordID           uint   `json:"baseRecordId"`
	ParentRecordID         uint   `json:"parentRecordId"`
	BackupMethod           string `json:"backupMethod" binding:"omitempty,max=30"`
	BackupLevel            string `json:"backupLevel" binding:"omitempty,max=30"`
	BackupEngine           string `json:"backupEngine" binding:"omitempty,max=60"`
	ExternalBackupID       string `json:"externalBackupId" binding:"omitempty,max=120"`
	ExternalServerName     string `json:"externalServerName" binding:"omitempty,max=120"`
	BackupScope            string `json:"backupScope" binding:"omitempty,max=30"`
	ToolName               string `json:"toolName" binding:"omitempty,max=60"`
	ToolVersion            string `json:"toolVersion" binding:"omitempty,max=120"`
	StorageProfileID       uint   `json:"storageProfileId"`
	StorageURI             string `json:"storageUri" binding:"required,max=1000"`
	ManifestJSON           string `json:"manifestJson" binding:"omitempty,max=20000"`
	PrepareStatus          string `json:"prepareStatus" binding:"omitempty,max=30"`
	FileName               string `json:"fileName" binding:"required,max=255"`
	FileSize               int64  `json:"fileSize" binding:"omitempty,min=0"`
	ChecksumSHA256         string `json:"checksumSha256" binding:"omitempty,max=64"`
	Compression            string `json:"compression" binding:"omitempty,max=30"`
	Encrypted              bool   `json:"encrypted"`
	RecoverableFrom        string `json:"recoverableFrom"`
	RecoverableUntil       string `json:"recoverableUntil"`
	StartedAt              string `json:"startedAt"`
	FinishedAt             string `json:"finishedAt"`
	Status                 string `json:"status" binding:"omitempty,max=30"`
	VerifyStatus           string `json:"verifyStatus" binding:"omitempty,max=30"`
	ServerUUID             string `json:"serverUuid" binding:"omitempty,max=120"`
	ServerID               string `json:"serverId" binding:"omitempty,max=60"`
	GTIDMode               string `json:"gtidMode" binding:"omitempty,max=30"`
	ExecutedGTIDSet        string `json:"executedGtidSet"`
	PurgedGTIDSet          string `json:"purgedGtidSet"`
	BinlogFormat           string `json:"binlogFormat" binding:"omitempty,max=30"`
	BinlogRowImage         string `json:"binlogRowImage" binding:"omitempty,max=30"`
	BackupBinlogFile       string `json:"backupBinlogFile" binding:"omitempty,max=255"`
	BackupBinlogPos        int64  `json:"backupBinlogPos"`
	BackupGTIDSet          string `json:"backupGtidSet"`
	PromotionHistory       string `json:"promotionHistory"`
	PGSystemIdentifier     string `json:"pgSystemIdentifier" binding:"omitempty,max=120"`
	TimelineID             string `json:"timelineId" binding:"omitempty,max=60"`
	TimelineHistoryFile    string `json:"timelineHistoryFile" binding:"omitempty,max=500"`
	WALSegmentSize         int64  `json:"walSegmentSize"`
	StartLSN               string `json:"startLsn" binding:"omitempty,max=120"`
	EndLSN                 string `json:"endLsn" binding:"omitempty,max=120"`
	WALStart               string `json:"walStart" binding:"omitempty,max=255"`
	WALEnd                 string `json:"walEnd" binding:"omitempty,max=255"`
	BackupLabelJSON        string `json:"backupLabelJson"`
	BackupManifestChecksum string `json:"backupManifestChecksum" binding:"omitempty,max=128"`
}

type DatabaseRestorePlanListRequest struct {
	Page               int    `form:"page"`
	PageSize           int    `form:"pageSize"`
	SourceInstanceID   uint   `form:"sourceInstanceId"`
	TargetInstanceID   uint   `form:"targetInstanceId"`
	ValidationStatus   string `form:"validationStatus"`
	RestoreStatus      string `form:"restoreStatus"`
	RestrictToAllowed  bool   `form:"-" json:"-"`
	AllowedInstanceIDs []uint `form:"-" json:"-"`
}

type DatabaseRestorePlanRequest struct {
	SourceInstanceID       uint   `json:"sourceInstanceId" binding:"required"`
	TargetInstanceID       uint   `json:"targetInstanceId"`
	RestoreMode            string `json:"restoreMode" binding:"omitempty,max=30"`
	RestoreTargetType      string `json:"restoreTargetType" binding:"omitempty,max=30"`
	RestoreTargetValue     string `json:"restoreTargetValue" binding:"required,max=255"`
	RestoreTargetInclusive bool   `json:"restoreTargetInclusive"`
}

type DatabaseRestorePlanVO struct {
	ID                      uint   `json:"id"`
	SourceInstanceID        uint   `json:"sourceInstanceId"`
	SourceInstanceName      string `json:"sourceInstanceName"`
	TargetInstanceID        uint   `json:"targetInstanceId"`
	TargetInstanceName      string `json:"targetInstanceName"`
	RunnerHostID            uint   `json:"runnerHostId"`
	RunnerHostName          string `json:"runnerHostName"`
	RestoreMode             string `json:"restoreMode"`
	RestoreModeText         string `json:"restoreModeText"`
	RestoreTargetType       string `json:"restoreTargetType"`
	RestoreTargetValue      string `json:"restoreTargetValue"`
	RestoreTargetInclusive  bool   `json:"restoreTargetInclusive"`
	SelectedBaseRecordID    uint   `json:"selectedBaseRecordId"`
	SelectedBackupRecordIDs string `json:"selectedBackupRecordIds"`
	SelectedLogArchiveIDs   string `json:"selectedLogArchiveIds"`
	BackupChainStatus       string `json:"backupChainStatus"`
	BackupChainStatusText   string `json:"backupChainStatusText"`
	LogChainStatus          string `json:"logChainStatus"`
	LogChainStatusText      string `json:"logChainStatusText"`
	StorageStatus           string `json:"storageStatus"`
	StorageStatusText       string `json:"storageStatusText"`
	ToolStatus              string `json:"toolStatus"`
	ToolStatusText          string `json:"toolStatusText"`
	ValidationStatus        string `json:"validationStatus"`
	ValidationStatusText    string `json:"validationStatusText"`
	RestoreStatus           string `json:"restoreStatus"`
	RestoreStatusText       string `json:"restoreStatusText"`
	RequiredToolJSON        string `json:"requiredToolJson"`
	RequiredArtifactJSON    string `json:"requiredArtifactJson"`
	EstimatedRestoreBytes   int64  `json:"estimatedRestoreBytes"`
	EstimatedRestoreMinutes int    `json:"estimatedRestoreMinutes"`
	PlanJSON                string `json:"planJson"`
	ProofJSON               string `json:"proofJson"`
	OperatorID              uint   `json:"operatorId"`
	OperatorName            string `json:"operatorName"`
	StartedAt               string `json:"startedAt"`
	FinishedAt              string `json:"finishedAt"`
	DurationMs              int64  `json:"durationMs"`
	Message                 string `json:"message"`
	CreatedAt               string `json:"createdAt"`
	UpdatedAt               string `json:"updatedAt"`
}

type DatabaseRunnerJobListRequest struct {
	Page             int    `form:"page"`
	PageSize         int    `form:"pageSize"`
	RunnerHostID     uint   `form:"runnerHostId"`
	JobType          string `form:"jobType"`
	Status           string `form:"status"`
	SourceInstanceID uint   `form:"sourceInstanceId"`
	TargetInstanceID uint   `form:"targetInstanceId"`
}

func (uc *UseCase) CreateStorageProfile(ctx context.Context, req *DatabaseStorageProfileRequest) (*DatabaseStorageProfileVO, error) {
	if uc.storageProfileRepo == nil {
		return nil, fmt.Errorf("存储配置仓库未配置")
	}
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("存储配置名称不能为空")
	}
	storageType := normalizeStorageProfileType(req.StorageType)
	if req.SecretProfileID > 0 && uc.secretProfileRepo != nil {
		if _, err := uc.secretProfileRepo.GetByID(ctx, req.SecretProfileID); err != nil {
			return nil, fmt.Errorf("密钥配置不存在")
		}
	}
	item := &DatabaseStorageProfile{
		Name:                trimText(strings.TrimSpace(req.Name), 120),
		StorageType:         storageType,
		Endpoint:            trimText(strings.TrimSpace(req.Endpoint), 255),
		Bucket:              trimText(strings.TrimSpace(req.Bucket), 255),
		Region:              trimText(strings.TrimSpace(req.Region), 120),
		PathPrefix:          trimText(strings.TrimSpace(req.PathPrefix), 500),
		SecretProfileID:     req.SecretProfileID,
		VersioningEnabled:   req.VersioningEnabled,
		ImmutabilityEnabled: req.ImmutabilityEnabled,
		KMSKeyID:            trimText(strings.TrimSpace(req.KMSKeyID), 255),
		RetentionLockDays:   req.RetentionLockDays,
		Status:              normalizeProfileStatus(req.Status),
	}
	if err := uc.storageProfileRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return toStorageProfileVO(item), nil
}

func (uc *UseCase) ListStorageProfiles(ctx context.Context, req *DatabaseStorageProfileListRequest) ([]*DatabaseStorageProfileVO, int64, error) {
	if uc.storageProfileRepo == nil {
		return nil, 0, fmt.Errorf("存储配置仓库未配置")
	}
	items, total, err := uc.storageProfileRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseStorageProfileVO, 0, len(items))
	for _, item := range items {
		list = append(list, toStorageProfileVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) CreateSecretProfile(ctx context.Context, req *DatabaseSecretProfileRequest) (*DatabaseSecretProfileVO, error) {
	if uc.secretProfileRepo == nil {
		return nil, fmt.Errorf("密钥配置仓库未配置")
	}
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("密钥配置名称不能为空")
	}
	if req.CredentialID > 0 && uc.credentialIDExists != nil {
		if err := uc.credentialIDExists(ctx, req.CredentialID); err != nil {
			return nil, fmt.Errorf("连接凭据不存在")
		}
	}
	item := &DatabaseSecretProfile{
		Name:         trimText(strings.TrimSpace(req.Name), 120),
		SecretType:   normalizeSecretProfileType(req.SecretType),
		CredentialID: req.CredentialID,
		ExternalRef:  trimText(strings.TrimSpace(req.ExternalRef), 500),
		Status:       normalizeProfileStatus(req.Status),
	}
	if item.CredentialID == 0 && strings.TrimSpace(item.ExternalRef) == "" {
		return nil, fmt.Errorf("密钥配置需要关联连接凭据或外部密钥引用")
	}
	if err := uc.secretProfileRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return toSecretProfileVO(item), nil
}

func (uc *UseCase) ListSecretProfiles(ctx context.Context, req *DatabaseSecretProfileListRequest) ([]*DatabaseSecretProfileVO, int64, error) {
	if uc.secretProfileRepo == nil {
		return nil, 0, fmt.Errorf("密钥配置仓库未配置")
	}
	items, total, err := uc.secretProfileRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseSecretProfileVO, 0, len(items))
	for _, item := range items {
		list = append(list, toSecretProfileVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) CreateLogArchiveStream(ctx context.Context, req *DatabaseLogArchiveStreamRequest) (*DatabaseLogArchiveStreamVO, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档流仓库未配置")
	}
	if req == nil || req.InstanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	sourceID := req.SourceInstanceID
	if sourceID == 0 {
		sourceID = req.InstanceID
	}
	if sourceID != req.InstanceID {
		if _, err := uc.instanceRepo.GetByID(ctx, sourceID); err != nil {
			return nil, fmt.Errorf("日志来源实例不存在")
		}
	}
	engine := normalizeDBType(req.Engine)
	if engine == "" {
		engine = normalizeDBType(instance.DBType)
	}
	archiveType := normalizeArchiveType(req.ArchiveType)
	if archiveType == "" {
		archiveType = archiveTypeForEngine(engine)
	}
	if archiveType == "" || archiveType == DatabaseArchiveTypeNone {
		return nil, fmt.Errorf("%s 暂不支持日志归档", DBTypeText(engine))
	}
	if expected := archiveTypeForEngine(engine); expected != "" && archiveType != expected {
		return nil, fmt.Errorf("%s 日志归档类型应为 %s", DBTypeText(engine), expected)
	}
	if err := validateBackupStorageConfigSafe(req.ConfigJSON); err != nil {
		return nil, err
	}
	if _, err := uc.validateLogArchiveRunnerHost(ctx, req.RunnerHostID); err != nil {
		return nil, err
	}
	status := DatabaseLogArchiveStreamStatusPending
	if !req.Enabled {
		status = DatabaseLogArchiveStreamStatusDisabled
	}
	archiveMode := normalizeArchiveMode(req.ArchiveMode)
	item := &DatabaseLogArchiveStream{
		InstanceID:       req.InstanceID,
		SourceInstanceID: sourceID,
		Engine:           engine,
		ArchiveType:      archiveType,
		ArchiveMode:      archiveMode,
		ArchiveEngine:    normalizeArchiveEngine(req.ArchiveEngine, archiveType),
		RunnerHostID:     req.RunnerHostID,
		StorageProfileID: req.StorageProfileID,
		SecretProfileID:  req.SecretProfileID,
		RPOTargetSeconds: req.RPOTargetSeconds,
		RetentionDays:    normalizeArchiveRetentionDays(req.RetentionDays),
		Enabled:          req.Enabled,
		Status:           status,
		DesiredState:     DatabaseLogArchiveDesiredStateStopped,
		DaemonStatus:     DatabaseLogArchiveDaemonStatusStopped,
		ConfigJSON:       trimText(strings.TrimSpace(req.ConfigJSON), 4000),
	}
	if err := uc.logArchiveStreamRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return uc.toLogArchiveStreamVO(ctx, item), nil
}

func (uc *UseCase) ListLogArchiveStreams(ctx context.Context, req *DatabaseLogArchiveStreamListRequest) ([]*DatabaseLogArchiveStreamVO, int64, error) {
	if uc.logArchiveStreamRepo == nil {
		return nil, 0, fmt.Errorf("日志归档流仓库未配置")
	}
	items, total, err := uc.logArchiveStreamRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseLogArchiveStreamVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toLogArchiveStreamVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) GetLogArchiveStreamInstanceID(ctx context.Context, id uint) (uint, error) {
	if uc.logArchiveStreamRepo == nil {
		return 0, fmt.Errorf("日志归档流仓库未配置")
	}
	if id == 0 {
		return 0, fmt.Errorf("日志归档流ID不能为空")
	}
	item, err := uc.logArchiveStreamRepo.GetByID(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("日志归档流不存在")
	}
	return item.InstanceID, nil
}

func (uc *UseCase) RegisterExternalLogArchive(ctx context.Context, req *DatabaseExternalLogArchiveRequest) (*DatabaseLogArchiveVO, error) {
	if uc.logArchiveRepo == nil || uc.logArchiveStreamRepo == nil {
		return nil, fmt.Errorf("日志归档仓库未配置")
	}
	if req == nil || req.StreamID == 0 {
		return nil, fmt.Errorf("请选择日志归档流")
	}
	stream, err := uc.logArchiveStreamRepo.GetByID(ctx, req.StreamID)
	if err != nil {
		return nil, fmt.Errorf("日志归档流不存在")
	}
	firstEventTime, err := parseDatabaseTime(req.FirstEventTime)
	if err != nil {
		return nil, fmt.Errorf("首个事件时间格式不正确")
	}
	lastEventTime, err := parseDatabaseTime(req.LastEventTime)
	if err != nil {
		return nil, fmt.Errorf("最后事件时间格式不正确")
	}
	if firstEventTime == nil || lastEventTime == nil {
		return nil, fmt.Errorf("日志归档必须填写事件时间范围")
	}
	if lastEventTime.Before(*firstEventTime) {
		return nil, fmt.Errorf("最后事件时间不能早于首个事件时间")
	}
	now := time.Now()
	status := normalizeLogArchiveStatus(req.Status)
	item := &DatabaseLogArchive{
		StreamID:           stream.ID,
		InstanceID:         stream.InstanceID,
		SourceInstanceID:   stream.SourceInstanceID,
		Engine:             stream.Engine,
		ArchiveType:        stream.ArchiveType,
		FileName:           trimText(strings.TrimSpace(req.FileName), 255),
		StorageURI:         trimText(strings.TrimSpace(req.StorageURI), 1000),
		FileSize:           req.FileSize,
		ChecksumSHA256:     strings.TrimSpace(req.ChecksumSHA256),
		FirstEventTime:     firstEventTime,
		LastEventTime:      lastEventTime,
		Status:             status,
		ArchivedAt:         &now,
		ServerUUID:         trimText(strings.TrimSpace(req.ServerUUID), 120),
		ServerID:           trimText(strings.TrimSpace(req.ServerID), 60),
		StartPos:           req.StartPos,
		EndPos:             req.EndPos,
		StartGTIDSet:       strings.TrimSpace(req.StartGTIDSet),
		EndGTIDSet:         strings.TrimSpace(req.EndGTIDSet),
		PreviousFileName:   trimText(strings.TrimSpace(req.PreviousFileName), 255),
		NextFileName:       trimText(strings.TrimSpace(req.NextFileName), 255),
		PGSystemIdentifier: trimText(strings.TrimSpace(req.PGSystemIdentifier), 120),
		TimelineID:         trimText(strings.TrimSpace(req.TimelineID), 60),
		WALSegmentSize:     streamWALSegmentSize(stream),
		ExternalServerName: streamExternalServerName(stream),
		StartLSN:           trimText(strings.TrimSpace(req.StartLSN), 120),
		EndLSN:             trimText(strings.TrimSpace(req.EndLSN), 120),
		SegmentNo:          trimText(strings.TrimSpace(req.SegmentNo), 120),
		TimelineHistoryURI: trimText(strings.TrimSpace(req.TimelineHistoryURI), 1000),
	}
	if item.FileName == "" || item.StorageURI == "" {
		return nil, fmt.Errorf("日志文件名和存储 URI 不能为空")
	}
	if err := uc.logArchiveRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	stream.LastArchivedAt = &now
	stream.LastArchiveName = item.FileName
	stream.LastError = ""
	stream.CursorFile = item.FileName
	if item.EndPos > 0 {
		stream.CursorPos = item.EndPos
	}
	if strings.TrimSpace(item.EndGTIDSet) != "" {
		stream.CursorGTIDSet = item.EndGTIDSet
	}
	stream.LastEventTime = item.LastEventTime
	stream.ArchiveLagSeconds = 0
	if status == DatabaseLogArchiveStatusArchived {
		stream.Status = DatabaseLogArchiveStreamStatusRunning
		stream.ConsecutiveFailures = 0
	} else {
		stream.Status = DatabaseLogArchiveStreamStatusDegraded
		stream.DaemonStatus = DatabaseLogArchiveDaemonStatusDegraded
		stream.ConsecutiveFailures++
		stream.LastError = LogArchiveStatusText(status)
	}
	_ = uc.logArchiveStreamRepo.Update(ctx, stream)
	return uc.toLogArchiveVO(ctx, item), nil
}

func (uc *UseCase) ListLogArchives(ctx context.Context, req *DatabaseLogArchiveListRequest) ([]*DatabaseLogArchiveVO, int64, error) {
	if uc.logArchiveRepo == nil {
		return nil, 0, fmt.Errorf("日志归档仓库未配置")
	}
	items, total, err := uc.logArchiveRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseLogArchiveVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toLogArchiveVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) ListLogArchiveEvents(ctx context.Context, req *DatabaseLogArchiveEventListRequest) ([]*DatabaseLogArchiveEventVO, int64, error) {
	if uc.logArchiveEventRepo == nil {
		return nil, 0, fmt.Errorf("日志归档事件仓库未配置")
	}
	items, total, err := uc.logArchiveEventRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseLogArchiveEventVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toLogArchiveEventVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) RegisterExternalBackupRecord(ctx context.Context, req *DatabaseExternalBackupRecordRequest) (*DatabaseBackupRecordVO, error) {
	if uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("备份记录仓库未配置")
	}
	if req == nil || req.InstanceID == 0 {
		return nil, fmt.Errorf("请选择数据库实例")
	}
	instance, err := uc.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	sourceID := req.SourceInstanceID
	if sourceID == 0 {
		sourceID = req.InstanceID
	}
	backupMethod := normalizeBackupMethod(req.BackupMethod)
	backupLevel := normalizeBackupLevel(req.BackupLevel)
	startedAt, err := parseDatabaseTime(req.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("开始时间格式不正确")
	}
	finishedAt, err := parseDatabaseTime(req.FinishedAt)
	if err != nil {
		return nil, fmt.Errorf("结束时间格式不正确")
	}
	now := time.Now()
	if startedAt == nil {
		startedAt = &now
	}
	if finishedAt == nil {
		finishedAt = &now
	}
	recoverableFrom, err := parseDatabaseTime(req.RecoverableFrom)
	if err != nil {
		return nil, fmt.Errorf("可恢复窗口起点格式不正确")
	}
	recoverableUntil, err := parseDatabaseTime(req.RecoverableUntil)
	if err != nil {
		return nil, fmt.Errorf("可恢复窗口终点格式不正确")
	}
	if recoverableFrom == nil {
		recoverableFrom = startedAt
	}
	if recoverableUntil == nil {
		recoverableUntil = finishedAt
	}
	if recoverableUntil.Before(*recoverableFrom) {
		return nil, fmt.Errorf("可恢复窗口终点不能早于起点")
	}
	status := normalizeBackupRecordStatus(req.Status)
	verifyStatus := normalizeBackupVerifyStatus(req.VerifyStatus)
	if strings.TrimSpace(req.ChecksumSHA256) != "" && len(strings.TrimSpace(req.ChecksumSHA256)) != 64 {
		verifyStatus = DatabaseBackupVerifyStatusFailed
	}
	chainID := strings.TrimSpace(req.ChainID)
	if chainID == "" {
		chainID = fmt.Sprintf("%s-%d-%d", backupMethod, req.InstanceID, now.Unix())
	}
	durationMs := finishedAt.Sub(*startedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	item := &DatabaseBackupRecord{
		TaskID:                 req.TaskID,
		InstanceID:             req.InstanceID,
		TriggerType:            DatabaseBackupTriggerExternal,
		BackupType:             backupMethod,
		ChainID:                trimText(chainID, 64),
		BaseRecordID:           req.BaseRecordID,
		ParentRecordID:         req.ParentRecordID,
		BackupMethod:           backupMethod,
		BackupLevel:            backupLevel,
		BackupEngine:           normalizeBackupEngine(req.BackupEngine, backupMethod),
		ExternalBackupID:       trimText(strings.TrimSpace(req.ExternalBackupID), 120),
		ExternalServerName:     trimText(strings.TrimSpace(req.ExternalServerName), 120),
		BackupScope:            normalizeBackupScope(req.BackupScope),
		ToolName:               trimText(strings.TrimSpace(req.ToolName), 60),
		ToolVersion:            trimText(strings.TrimSpace(req.ToolVersion), 120),
		SourceInstanceID:       sourceID,
		SourceRole:             normalizeSourceRole(req.SourceRole),
		StorageProfileID:       req.StorageProfileID,
		StorageType:            DatabaseBackupStorageExternal,
		StorageURI:             trimText(strings.TrimSpace(req.StorageURI), 1000),
		ManifestJSON:           trimText(strings.TrimSpace(req.ManifestJSON), 20000),
		PrepareStatus:          trimText(strings.TrimSpace(req.PrepareStatus), 30),
		Status:                 status,
		FileName:               trimText(strings.TrimSpace(req.FileName), 255),
		FileSize:               req.FileSize,
		ChecksumSHA256:         strings.TrimSpace(req.ChecksumSHA256),
		Encrypted:              req.Encrypted,
		Compression:            trimText(strings.TrimSpace(req.Compression), 30),
		VerifiedAt:             finishedAt,
		VerifyStatus:           verifyStatus,
		VerifyMessage:          BackupVerifyStatusText(verifyStatus),
		StartedAt:              startedAt,
		FinishedAt:             finishedAt,
		RecoverableFrom:        recoverableFrom,
		RecoverableUntil:       recoverableUntil,
		DurationMs:             durationMs,
		ErrorMessage:           "外部备份记录已登记",
		ServerUUID:             trimText(strings.TrimSpace(req.ServerUUID), 120),
		ServerID:               trimText(strings.TrimSpace(req.ServerID), 60),
		GTIDMode:               trimText(strings.TrimSpace(req.GTIDMode), 30),
		ExecutedGTIDSet:        strings.TrimSpace(req.ExecutedGTIDSet),
		PurgedGTIDSet:          strings.TrimSpace(req.PurgedGTIDSet),
		BinlogFormat:           trimText(strings.TrimSpace(req.BinlogFormat), 30),
		BinlogRowImage:         trimText(strings.TrimSpace(req.BinlogRowImage), 30),
		BackupBinlogFile:       trimText(strings.TrimSpace(req.BackupBinlogFile), 255),
		BackupBinlogPos:        req.BackupBinlogPos,
		BackupGTIDSet:          strings.TrimSpace(req.BackupGTIDSet),
		PromotionHistory:       strings.TrimSpace(req.PromotionHistory),
		PGSystemIdentifier:     trimText(strings.TrimSpace(req.PGSystemIdentifier), 120),
		TimelineID:             trimText(strings.TrimSpace(req.TimelineID), 60),
		TimelineHistoryFile:    trimText(strings.TrimSpace(req.TimelineHistoryFile), 500),
		WALSegmentSize:         req.WALSegmentSize,
		StartLSN:               trimText(strings.TrimSpace(req.StartLSN), 120),
		EndLSN:                 trimText(strings.TrimSpace(req.EndLSN), 120),
		WALStart:               trimText(strings.TrimSpace(req.WALStart), 255),
		WALEnd:                 trimText(strings.TrimSpace(req.WALEnd), 255),
		BackupLabelJSON:        strings.TrimSpace(req.BackupLabelJSON),
		BackupManifestChecksum: trimText(strings.TrimSpace(req.BackupManifestChecksum), 128),
	}
	if item.FileName == "" || item.StorageURI == "" {
		return nil, fmt.Errorf("外部备份文件名和存储 URI 不能为空")
	}
	if item.BaseRecordID == 0 && backupLevel == DatabaseBackupLevelFull {
		item.BaseRecordID = 0
	}
	if err := uc.backupRecordRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	if req.TaskID > 0 && uc.backupTaskRepo != nil {
		if task, taskErr := uc.backupTaskRepo.GetByID(ctx, req.TaskID); taskErr == nil && task != nil {
			task.LastSuccessAt = finishedAt
			task.RestoreCapability = capabilityForBackupRecord(item)
			_ = uc.backupTaskRepo.Update(ctx, task)
		}
	}
	return uc.toBackupRecordVO(item, "", instance.Name), nil
}

func (uc *UseCase) CreateRestorePlan(ctx context.Context, req *DatabaseRestorePlanRequest, operator QueryOperator) (*DatabaseRestorePlanVO, error) {
	if uc.restorePlanRepo == nil || uc.backupRecordRepo == nil {
		return nil, fmt.Errorf("恢复计划仓库未配置")
	}
	if req == nil || req.SourceInstanceID == 0 {
		return nil, fmt.Errorf("请选择来源实例")
	}
	source, err := uc.instanceRepo.GetByID(ctx, req.SourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("来源实例不存在")
	}
	var target *DatabaseInstance
	if req.TargetInstanceID > 0 {
		target, err = uc.instanceRepo.GetByID(ctx, req.TargetInstanceID)
		if err != nil {
			return nil, fmt.Errorf("目标实例不存在")
		}
		if isProductionEnvironment(target.Environment) {
			return nil, fmt.Errorf("恢复计划目标实例不能是生产环境")
		}
	}
	targetType := strings.ToLower(strings.TrimSpace(req.RestoreTargetType))
	if targetType == "" {
		targetType = "time"
	}
	if targetType != "time" {
		return nil, fmt.Errorf("P1 仅支持按时间生成恢复计划")
	}
	targetTime, err := parseDatabaseTime(req.RestoreTargetValue)
	if err != nil || targetTime == nil {
		return nil, fmt.Errorf("恢复目标时间格式不正确")
	}
	startedAt := time.Now()
	records, err := uc.backupRecordRepo.ListSuccessfulForRestore(ctx, req.SourceInstanceID, targetTime)
	if err != nil {
		return nil, err
	}
	base := selectRestoreBaseRecord(records, *targetTime)
	result := uc.validateRestorePlan(ctx, source, records, base, *targetTime)
	finishedAt := time.Now()
	mode := strings.TrimSpace(req.RestoreMode)
	if mode == "" {
		mode = "isolated_restore"
	}
	planJSON := marshalBackupPlanJSON(result)
	proofJSON := buildRestoreProofJSON(source, target, result, *targetTime, operator, mode)
	item := &DatabaseRestorePlan{
		SourceInstanceID:        req.SourceInstanceID,
		TargetInstanceID:        req.TargetInstanceID,
		RestoreMode:             mode,
		RestoreTargetType:       targetType,
		RestoreTargetValue:      targetTime.Format("2006-01-02 15:04:05"),
		RestoreTargetInclusive:  req.RestoreTargetInclusive,
		SelectedBaseRecordID:    result.BaseRecordID,
		SelectedBackupRecordIDs: marshalUintList(result.BackupRecordIDs),
		SelectedLogArchiveIDs:   marshalUintList(result.LogArchiveIDs),
		BackupChainStatus:       result.BackupChainStatus,
		LogChainStatus:          result.LogChainStatus,
		StorageStatus:           result.StorageStatus,
		ToolStatus:              result.ToolStatus,
		ValidationStatus:        result.ValidationStatus,
		RestoreStatus:           DatabaseRestoreStatusPlanned,
		RequiredToolJSON:        buildRestoreRequiredToolJSON(source, result),
		RequiredArtifactJSON:    buildRestoreRequiredArtifactJSON(result),
		EstimatedRestoreBytes:   estimateRestoreBytes(result),
		EstimatedRestoreMinutes: estimateRestoreMinutes(result),
		PlanJSON:                planJSON,
		ProofJSON:               proofJSON,
		OperatorID:              operator.ID,
		OperatorName:            trimText(operator.Username, 120),
		StartedAt:               &startedAt,
		FinishedAt:              &finishedAt,
		DurationMs:              finishedAt.Sub(startedAt).Milliseconds(),
		ErrorMessage:            trimText(strings.Join(result.Messages, "；"), 1000),
	}
	if err := uc.restorePlanRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	if result.ValidationStatus == DatabasePlanValidationPassed && uc.backupTaskRepo != nil && base != nil && base.TaskID > 0 {
		if task, taskErr := uc.backupTaskRepo.GetByID(ctx, base.TaskID); taskErr == nil && task != nil {
			task.RestoreCapability = DatabaseRestoreCapabilityPITRCapable
			_ = uc.backupTaskRepo.Update(ctx, task)
		}
	}
	sourceName := source.Name
	targetName := ""
	if target != nil {
		targetName = target.Name
	}
	return uc.toRestorePlanVO(item, sourceName, targetName), nil
}

func (uc *UseCase) ListRestorePlans(ctx context.Context, req *DatabaseRestorePlanListRequest) ([]*DatabaseRestorePlanVO, int64, error) {
	if uc.restorePlanRepo == nil {
		return nil, 0, fmt.Errorf("恢复计划仓库未配置")
	}
	items, total, err := uc.restorePlanRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseRestorePlanVO, 0, len(items))
	for _, item := range items {
		sourceName, targetName := uc.restorePlanInstanceNames(ctx, item)
		list = append(list, uc.toRestorePlanVO(item, sourceName, targetName))
	}
	return list, total, nil
}

type restorePlanValidationResult struct {
	BaseRecordID      uint                     `json:"baseRecordId"`
	BackupRecordIDs   []uint                   `json:"backupRecordIds"`
	LogArchiveIDs     []uint                   `json:"logArchiveIds"`
	BackupChainStatus string                   `json:"backupChainStatus"`
	LogChainStatus    string                   `json:"logChainStatus"`
	StorageStatus     string                   `json:"storageStatus"`
	ToolStatus        string                   `json:"toolStatus"`
	ValidationStatus  string                   `json:"validationStatus"`
	RecoverableFrom   string                   `json:"recoverableFrom,omitempty"`
	RecoverableUntil  string                   `json:"recoverableUntil,omitempty"`
	BackupProofs      []restoreProofBackup     `json:"backupProofs,omitempty"`
	LogProofs         []restoreProofLogArchive `json:"logProofs,omitempty"`
	ValidationSQL     []string                 `json:"validationSql,omitempty"`
	Messages          []string                 `json:"messages"`
}

type restoreProofBackup struct {
	ID               uint   `json:"id"`
	BackupMethod     string `json:"backupMethod"`
	BackupLevel      string `json:"backupLevel"`
	BackupEngine     string `json:"backupEngine"`
	FileName         string `json:"fileName"`
	StorageURI       string `json:"storageUri"`
	FileSize         int64  `json:"fileSize"`
	ChecksumSHA256   string `json:"checksumSha256"`
	RecoverableFrom  string `json:"recoverableFrom,omitempty"`
	RecoverableUntil string `json:"recoverableUntil,omitempty"`
	ToolName         string `json:"toolName,omitempty"`
	ToolVersion      string `json:"toolVersion,omitempty"`
	BinlogFile       string `json:"backupBinlogFile,omitempty"`
	BinlogPos        int64  `json:"backupBinlogPos,omitempty"`
	GTIDSet          string `json:"backupGtidSet,omitempty"`
	ServerUUID       string `json:"serverUuid,omitempty"`
	ServerID         string `json:"serverId,omitempty"`
}

type restoreProofLogArchive struct {
	ID               uint   `json:"id"`
	ArchiveType      string `json:"archiveType"`
	FileName         string `json:"fileName"`
	StorageURI       string `json:"storageUri"`
	FileSize         int64  `json:"fileSize,omitempty"`
	ChecksumSHA256   string `json:"checksumSha256"`
	FirstEventTime   string `json:"firstEventTime,omitempty"`
	LastEventTime    string `json:"lastEventTime,omitempty"`
	StartPos         int64  `json:"startPos,omitempty"`
	EndPos           int64  `json:"endPos,omitempty"`
	StartGTIDSet     string `json:"startGtidSet,omitempty"`
	EndGTIDSet       string `json:"endGtidSet,omitempty"`
	PreviousFileName string `json:"previousFileName,omitempty"`
	NextFileName     string `json:"nextFileName,omitempty"`
	ServerUUID       string `json:"serverUuid,omitempty"`
	ServerID         string `json:"serverId,omitempty"`
}

func (uc *UseCase) validateRestorePlan(ctx context.Context, instance *DatabaseInstance, records []*DatabaseBackupRecord, base *DatabaseBackupRecord, targetTime time.Time) *restorePlanValidationResult {
	result := &restorePlanValidationResult{
		BackupChainStatus: DatabaseBackupChainStatusMissingBase,
		LogChainStatus:    DatabaseLogChainStatusUnsupported,
		StorageStatus:     DatabaseStorageStatusUnsupported,
		ToolStatus:        DatabaseToolStatusUnsupported,
		ValidationStatus:  DatabasePlanValidationFailed,
		ValidationSQL:     validationSQLForRestorePlan(instance),
		Messages:          []string{},
	}
	if base == nil {
		result.Messages = append(result.Messages, "未找到目标时间之前可用的 base backup")
		return result
	}
	result.BaseRecordID = base.ID
	result.BackupRecordIDs = []uint{base.ID}
	if base.RecoverableFrom != nil {
		result.RecoverableFrom = base.RecoverableFrom.Format("2006-01-02 15:04:05")
	}
	if base.RecoverableUntil != nil {
		result.RecoverableUntil = base.RecoverableUntil.Format("2006-01-02 15:04:05")
	}
	method := normalizeBackupMethod(base.BackupMethod)
	if method == DatabaseBackupMethodLogical {
		result.BackupChainStatus = DatabaseBackupChainStatusUnsupported
		result.LogChainStatus = DatabaseLogChainStatusUnsupported
		result.StorageStatus = DatabaseStorageStatusAvailable
		result.ToolStatus = DatabaseToolStatusUnsupported
		result.Messages = append(result.Messages, "逻辑备份不支持 PITR，只能做逻辑恢复或对象级回填")
		return result
	}
	result.BackupChainStatus = validateIncrementalBackupChain(records, base, targetTime, &result.BackupRecordIDs, &result.Messages)
	result.BackupProofs = buildSelectedBackupProofs(records, result.BackupRecordIDs)
	result.StorageStatus = storageStatusForBackupRecord(base)
	result.ToolStatus = toolStatusForBackupRecord(base)
	if result.StorageStatus != DatabaseStorageStatusAvailable {
		result.Messages = append(result.Messages, StorageStatusText(result.StorageStatus))
	}
	if result.ToolStatus != DatabaseToolStatusCompatible {
		result.Messages = append(result.Messages, ToolStatusText(result.ToolStatus))
	}
	logStart := base.FinishedAt
	if base.RecoverableUntil != nil {
		logStart = base.RecoverableUntil
	}
	if logStart == nil || !targetTime.After(*logStart) {
		result.LogChainStatus = DatabaseLogChainStatusComplete
	} else {
		archiveType := archiveTypeForEngine(normalizeDBType(instance.DBType))
		if archiveType == "" {
			result.LogChainStatus = DatabaseLogChainStatusUnsupported
		} else if uc.logArchiveRepo == nil {
			result.LogChainStatus = DatabaseLogChainStatusUnsupported
			result.Messages = append(result.Messages, "日志归档仓库未配置")
		} else {
			logs, err := uc.logArchiveRepo.ListCoveringTimeRange(ctx, base.InstanceID, archiveType, *logStart, targetTime)
			if err != nil {
				result.LogChainStatus = DatabaseLogChainStatusUnsupported
				result.Messages = append(result.Messages, err.Error())
			} else {
				status, ids, message := validateLogArchiveCoverage(logs, archiveType, *logStart, targetTime)
				result.LogChainStatus = status
				result.LogArchiveIDs = ids
				result.LogProofs = buildSelectedLogProofs(logs, ids)
				if message != "" {
					result.Messages = append(result.Messages, message)
				}
				if archiveType == DatabaseArchiveTypeBinlog && status == DatabaseLogChainStatusComplete {
					mysqlStatus, mysqlMessage := validateMySQLRestoreLogMetadata(base, logs)
					if mysqlStatus != DatabaseLogChainStatusComplete {
						result.LogChainStatus = mysqlStatus
						result.Messages = append(result.Messages, mysqlMessage)
					}
				}
				for _, item := range logs {
					if item != nil && item.Status == DatabaseLogArchiveStatusChecksumFailed {
						result.StorageStatus = DatabaseStorageStatusChecksumFailed
					}
				}
			}
		}
	}
	if result.BackupChainStatus == DatabaseBackupChainStatusComplete &&
		result.LogChainStatus == DatabaseLogChainStatusComplete &&
		result.StorageStatus == DatabaseStorageStatusAvailable &&
		result.ToolStatus == DatabaseToolStatusCompatible {
		result.ValidationStatus = DatabasePlanValidationPassed
		result.Messages = append(result.Messages, "恢复计划预校验通过，可作为后续隔离恢复输入")
		return result
	}
	result.ValidationStatus = DatabasePlanValidationFailed
	if len(result.Messages) == 0 {
		result.Messages = append(result.Messages, "恢复计划预校验未通过")
	}
	return result
}

func buildSelectedBackupProofs(records []*DatabaseBackupRecord, ids []uint) []restoreProofBackup {
	if len(records) == 0 || len(ids) == 0 {
		return nil
	}
	byID := make(map[uint]*DatabaseBackupRecord, len(records))
	for _, item := range records {
		if item != nil {
			byID[item.ID] = item
		}
	}
	result := make([]restoreProofBackup, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if item := byID[id]; item != nil {
			result = append(result, backupProofFromRecord(item))
		}
	}
	return result
}

func backupProofFromRecord(item *DatabaseBackupRecord) restoreProofBackup {
	if item == nil {
		return restoreProofBackup{}
	}
	proof := restoreProofBackup{
		ID:             item.ID,
		BackupMethod:   item.BackupMethod,
		BackupLevel:    item.BackupLevel,
		BackupEngine:   item.BackupEngine,
		FileName:       item.FileName,
		StorageURI:     firstNonEmpty(item.StorageURI, item.FilePath),
		FileSize:       item.FileSize,
		ChecksumSHA256: item.ChecksumSHA256,
		ToolName:       item.ToolName,
		ToolVersion:    item.ToolVersion,
		BinlogFile:     item.BackupBinlogFile,
		BinlogPos:      item.BackupBinlogPos,
		GTIDSet:        item.BackupGTIDSet,
		ServerUUID:     item.ServerUUID,
		ServerID:       item.ServerID,
	}
	if item.RecoverableFrom != nil {
		proof.RecoverableFrom = item.RecoverableFrom.Format("2006-01-02 15:04:05")
	}
	if item.RecoverableUntil != nil {
		proof.RecoverableUntil = item.RecoverableUntil.Format("2006-01-02 15:04:05")
	}
	return proof
}

func buildSelectedLogProofs(logs []*DatabaseLogArchive, ids []uint) []restoreProofLogArchive {
	if len(logs) == 0 || len(ids) == 0 {
		return nil
	}
	selected := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	result := make([]restoreProofLogArchive, 0, len(ids))
	for _, item := range logs {
		if item == nil {
			continue
		}
		if _, ok := selected[item.ID]; !ok {
			continue
		}
		proof := restoreProofLogArchive{
			ID:               item.ID,
			ArchiveType:      item.ArchiveType,
			FileName:         item.FileName,
			StorageURI:       item.StorageURI,
			FileSize:         item.FileSize,
			ChecksumSHA256:   item.ChecksumSHA256,
			StartPos:         item.StartPos,
			EndPos:           item.EndPos,
			StartGTIDSet:     item.StartGTIDSet,
			EndGTIDSet:       item.EndGTIDSet,
			PreviousFileName: item.PreviousFileName,
			NextFileName:     item.NextFileName,
			ServerUUID:       item.ServerUUID,
			ServerID:         item.ServerID,
		}
		if item.FirstEventTime != nil {
			proof.FirstEventTime = item.FirstEventTime.Format("2006-01-02 15:04:05")
		}
		if item.LastEventTime != nil {
			proof.LastEventTime = item.LastEventTime.Format("2006-01-02 15:04:05")
		}
		result = append(result, proof)
	}
	return result
}

func validationSQLForRestorePlan(instance *DatabaseInstance) []string {
	if instance == nil {
		return []string{"SELECT 1 AS restore_probe"}
	}
	switch normalizeDBType(instance.DBType) {
	case DBTypeMySQL, DBTypeMariaDB:
		return []string{
			"SELECT 1 AS restore_probe",
			"SELECT @@version AS version",
			"SHOW DATABASES",
		}
	case DBTypePostgreSQL:
		return []string{
			"SELECT 1 AS restore_probe",
			"SELECT version()",
			"SELECT datname FROM pg_database WHERE datistemplate = false",
		}
	default:
		return []string{"SELECT 1 AS restore_probe"}
	}
}

func buildRestoreProofJSON(source, target *DatabaseInstance, result *restorePlanValidationResult, targetTime time.Time, operator QueryOperator, mode string) string {
	if result == nil {
		return "{}"
	}
	proof := map[string]any{
		"generatedAt":        time.Now().Format("2006-01-02 15:04:05"),
		"sourceInstanceId":   uint(0),
		"sourceInstanceName": "",
		"targetInstanceId":   uint(0),
		"targetInstanceName": "",
		"restoreMode":        mode,
		"restoreTargetType":  "time",
		"restoreTargetValue": targetTime.Format("2006-01-02 15:04:05"),
		"validationStatus":   result.ValidationStatus,
		"backupChainStatus":  result.BackupChainStatus,
		"logChainStatus":     result.LogChainStatus,
		"storageStatus":      result.StorageStatus,
		"toolStatus":         result.ToolStatus,
		"baseBackup":         nil,
		"incrementalChain":   []restoreProofBackup{},
		"logArchiveRange":    result.LogProofs,
		"checksum":           map[string]any{"backupRecords": backupChecksumProofs(result.BackupProofs), "logArchives": logChecksumProofs(result.LogProofs)},
		"validationSql":      result.ValidationSQL,
		"operatorId":         operator.ID,
		"operatorName":       operator.Username,
		"messages":           result.Messages,
	}
	if source != nil {
		proof["sourceInstanceId"] = source.ID
		proof["sourceInstanceName"] = source.Name
	}
	if target != nil {
		proof["targetInstanceId"] = target.ID
		proof["targetInstanceName"] = target.Name
	}
	if len(result.BackupProofs) > 0 {
		proof["baseBackup"] = result.BackupProofs[0]
		if len(result.BackupProofs) > 1 {
			proof["incrementalChain"] = result.BackupProofs[1:]
		}
	}
	return marshalBackupPlanJSON(proof)
}

func buildRestoreRequiredToolJSON(source *DatabaseInstance, result *restorePlanValidationResult) string {
	if result == nil {
		return "[]"
	}
	tools := make([]map[string]any, 0, 3)
	seen := map[string]struct{}{}
	for _, item := range result.BackupProofs {
		tool := strings.TrimSpace(item.ToolName)
		if tool == "" {
			tool = mysqlPhysicalBackupToolName(item.BackupEngine, "")
		}
		if tool == "" {
			continue
		}
		key := tool + "|" + item.ToolVersion
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tools = append(tools, map[string]any{
			"name":       tool,
			"version":    item.ToolVersion,
			"requiredBy": "physical_backup_prepare",
		})
	}
	engine := ""
	if source != nil {
		engine = normalizeDBType(source.DBType)
	}
	if engine == DBTypeMySQL || engine == DBTypeMariaDB {
		tools = append(tools, map[string]any{"name": "docker", "requiredBy": "isolated_instance"})
		tools = append(tools, map[string]any{"name": "mysqlbinlog", "requiredBy": "pitr_log_replay"})
		tools = append(tools, map[string]any{"name": "mysql", "requiredBy": "validation_sql"})
	}
	data, _ := json.Marshal(tools)
	return string(data)
}

func buildRestoreRequiredArtifactJSON(result *restorePlanValidationResult) string {
	if result == nil {
		return "[]"
	}
	artifacts := make([]map[string]any, 0, len(result.BackupProofs)+len(result.LogProofs))
	for _, item := range result.BackupProofs {
		artifacts = append(artifacts, map[string]any{
			"type":           "backup",
			"id":             item.ID,
			"fileName":       item.FileName,
			"storageUri":     item.StorageURI,
			"fileSize":       item.FileSize,
			"checksumSha256": item.ChecksumSHA256,
			"level":          item.BackupLevel,
		})
	}
	for _, item := range result.LogProofs {
		artifacts = append(artifacts, map[string]any{
			"type":           item.ArchiveType,
			"id":             item.ID,
			"fileName":       item.FileName,
			"storageUri":     item.StorageURI,
			"fileSize":       item.FileSize,
			"checksumSha256": item.ChecksumSHA256,
		})
	}
	data, _ := json.Marshal(artifacts)
	return string(data)
}

func estimateRestoreBytes(result *restorePlanValidationResult) int64 {
	if result == nil {
		return 0
	}
	var total int64
	for _, item := range result.BackupProofs {
		if item.FileSize > 0 {
			total += item.FileSize
		}
	}
	for _, item := range result.LogProofs {
		if item.FileSize > 0 {
			total += item.FileSize
		}
	}
	return total
}

func estimateRestoreMinutes(result *restorePlanValidationResult) int {
	total := estimateRestoreBytes(result)
	if total <= 0 {
		return 0
	}
	// 粗略按 2 GiB/min 估算，实际耗时以后由 Runner 历史样本校准。
	minutes := int((total + (2*1024*1024*1024 - 1)) / (2 * 1024 * 1024 * 1024))
	if minutes < 1 {
		return 1
	}
	return minutes
}

func backupChecksumProofs(items []restoreProofBackup) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":             item.ID,
			"fileName":       item.FileName,
			"storageUri":     item.StorageURI,
			"checksumSha256": item.ChecksumSHA256,
		})
	}
	return result
}

func logChecksumProofs(items []restoreProofLogArchive) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":             item.ID,
			"fileName":       item.FileName,
			"storageUri":     item.StorageURI,
			"checksumSha256": item.ChecksumSHA256,
		})
	}
	return result
}

func validateIncrementalBackupChain(records []*DatabaseBackupRecord, base *DatabaseBackupRecord, targetTime time.Time, selectedIDs *[]uint, messages *[]string) string {
	if base == nil {
		return DatabaseBackupChainStatusMissingBase
	}
	status := DatabaseBackupChainStatusComplete
	incrementals := make([]*DatabaseBackupRecord, 0)
	for _, item := range records {
		if item == nil || item.ID == base.ID || item.Status != DatabaseBackupStatusSuccess {
			continue
		}
		if normalizeBackupMethod(item.BackupMethod) != normalizeBackupMethod(base.BackupMethod) {
			continue
		}
		if normalizeBackupLevel(item.BackupLevel) != DatabaseBackupLevelIncremental {
			continue
		}
		if item.ChainID != "" && base.ChainID != "" && item.ChainID != base.ChainID {
			continue
		}
		if item.FinishedAt != nil && item.FinishedAt.After(targetTime) {
			continue
		}
		incrementals = append(incrementals, item)
	}
	sort.SliceStable(incrementals, func(i, j int) bool {
		left := incrementals[i].FinishedAt
		right := incrementals[j].FinishedAt
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.Before(*right)
	})
	selected := map[uint]struct{}{base.ID: {}}
	for _, item := range incrementals {
		if item.ParentRecordID > 0 {
			if _, ok := selected[item.ParentRecordID]; !ok {
				if messages != nil {
					*messages = append(*messages, fmt.Sprintf("增量备份 #%d 的父记录 #%d 不在当前链路内", item.ID, item.ParentRecordID))
				}
				status = DatabaseBackupChainStatusMissingIncremental
			}
		}
		selected[item.ID] = struct{}{}
		if selectedIDs != nil {
			*selectedIDs = append(*selectedIDs, item.ID)
		}
	}
	return status
}

func validateMySQLRestoreLogMetadata(base *DatabaseBackupRecord, logs []*DatabaseLogArchive) (string, string) {
	if base == nil {
		return DatabaseLogChainStatusMissingBinlog, "缺少 base backup"
	}
	if strings.TrimSpace(base.BackupBinlogFile) == "" && strings.TrimSpace(base.BackupGTIDSet) == "" {
		return DatabaseLogChainStatusMissingBinlog, "base backup 缺少 backup_binlog_file/binlog_pos 或 GTID 起点"
	}
	gtidEnabled := strings.EqualFold(strings.TrimSpace(base.GTIDMode), "ON") ||
		strings.Contains(strings.ToLower(base.GTIDMode), "mariadb") ||
		strings.TrimSpace(base.BackupGTIDSet) != ""
	if gtidEnabled && strings.TrimSpace(base.BackupGTIDSet) == "" && strings.TrimSpace(base.ExecutedGTIDSet) == "" {
		return DatabaseLogChainStatusGTIDGap, "GTID 模式已开启，但 base backup 缺少 GTID 集合"
	}
	if len(logs) == 0 {
		return DatabaseLogChainStatusMissingBinlog, "缺少 binlog 归档"
	}
	if strings.TrimSpace(base.BackupBinlogFile) != "" {
		foundStart := false
		for index, item := range logs {
			if item == nil {
				continue
			}
			if item.FileName == base.BackupBinlogFile || item.PreviousFileName == base.BackupBinlogFile {
				foundStart = true
			}
			if index > 0 {
				prev := logs[index-1]
				if prev != nil && strings.TrimSpace(prev.NextFileName) != "" && prev.NextFileName != item.FileName {
					return DatabaseLogChainStatusMissingBinlog, fmt.Sprintf("binlog 文件链断裂：%s 的下一个文件不是 %s", prev.FileName, item.FileName)
				}
				if prev != nil && strings.TrimSpace(item.PreviousFileName) != "" && item.PreviousFileName != prev.FileName {
					return DatabaseLogChainStatusMissingBinlog, fmt.Sprintf("binlog 文件链断裂：%s 的上一个文件不是 %s", item.FileName, prev.FileName)
				}
			}
			if gtidEnabled && strings.TrimSpace(item.StartGTIDSet) == "" && strings.TrimSpace(item.EndGTIDSet) == "" {
				return DatabaseLogChainStatusGTIDGap, fmt.Sprintf("binlog %s 缺少 GTID 起止集合", item.FileName)
			}
		}
		if !foundStart {
			return DatabaseLogChainStatusMissingBinlog, "日志归档中未找到 base backup 对应的 binlog 起点"
		}
	}
	return DatabaseLogChainStatusComplete, ""
}

func selectRestoreBaseRecord(records []*DatabaseBackupRecord, targetTime time.Time) *DatabaseBackupRecord {
	candidates := make([]*DatabaseBackupRecord, 0, len(records))
	for _, item := range records {
		if item == nil || item.Status != DatabaseBackupStatusSuccess {
			continue
		}
		if item.RecoverableFrom != nil && item.RecoverableFrom.After(targetTime) {
			continue
		}
		if item.FinishedAt != nil && item.FinishedAt.After(targetTime) {
			continue
		}
		candidates = append(candidates, item)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i].FinishedAt
		right := candidates[j].FinishedAt
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.After(*right)
	})
	for _, item := range candidates {
		if normalizeBackupMethod(item.BackupMethod) != DatabaseBackupMethodLogical && normalizeBackupLevel(item.BackupLevel) == DatabaseBackupLevelFull {
			return item
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return nil
}

func validateLogArchiveCoverage(items []*DatabaseLogArchive, archiveType string, startTime, endTime time.Time) (string, []uint, string) {
	if len(items) == 0 {
		if archiveType == DatabaseArchiveTypeWAL {
			return DatabaseLogChainStatusMissingWAL, nil, "缺少覆盖目标时间段的 WAL 归档"
		}
		return DatabaseLogChainStatusMissingBinlog, nil, "缺少覆盖目标时间段的 binlog 归档"
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].FirstEventTime == nil {
			return false
		}
		if items[j].FirstEventTime == nil {
			return true
		}
		return items[i].FirstEventTime.Before(*items[j].FirstEventTime)
	})
	ids := make([]uint, 0, len(items))
	cursor := startTime
	tolerance := time.Minute
	for _, item := range items {
		if item == nil || item.FirstEventTime == nil || item.LastEventTime == nil {
			continue
		}
		if item.Status == DatabaseLogArchiveStatusMissing {
			return missingLogStatus(archiveType), ids, "日志归档文件被标记为缺失"
		}
		if item.Status == DatabaseLogArchiveStatusChecksumFailed {
			return DatabaseLogChainStatusTimeRangeGap, ids, "日志归档文件 checksum 异常"
		}
		if item.FirstEventTime.After(cursor.Add(tolerance)) {
			return DatabaseLogChainStatusTimeRangeGap, ids, "日志归档时间范围不连续"
		}
		if item.LastEventTime.After(cursor) {
			cursor = *item.LastEventTime
		}
		ids = append(ids, item.ID)
		if !cursor.Before(endTime) {
			return DatabaseLogChainStatusComplete, ids, ""
		}
	}
	if cursor.Before(endTime) {
		return missingLogStatus(archiveType), ids, "日志归档未覆盖到目标恢复时间"
	}
	return DatabaseLogChainStatusComplete, ids, ""
}

func missingLogStatus(archiveType string) string {
	if archiveType == DatabaseArchiveTypeWAL {
		return DatabaseLogChainStatusMissingWAL
	}
	return DatabaseLogChainStatusMissingBinlog
}

func storageStatusForBackupRecord(item *DatabaseBackupRecord) string {
	if item == nil {
		return DatabaseStorageStatusMissingObject
	}
	if item.VerifyStatus == DatabaseBackupVerifyStatusFailed {
		return DatabaseStorageStatusChecksumFailed
	}
	if strings.TrimSpace(item.StorageURI) == "" && strings.TrimSpace(item.FilePath) == "" {
		return DatabaseStorageStatusMissingObject
	}
	if checksum := strings.TrimSpace(item.ChecksumSHA256); checksum != "" && len(checksum) != 64 {
		return DatabaseStorageStatusChecksumFailed
	}
	return DatabaseStorageStatusAvailable
}

func toolStatusForBackupRecord(item *DatabaseBackupRecord) string {
	if item == nil {
		return DatabaseToolStatusUnsupported
	}
	if strings.TrimSpace(item.BackupEngine) == "" && strings.TrimSpace(item.ToolName) == "" {
		return DatabaseToolStatusUnsupported
	}
	return DatabaseToolStatusCompatible
}

func capabilityForBackupRecord(item *DatabaseBackupRecord) string {
	if item == nil {
		return DatabaseRestoreCapabilityNone
	}
	switch normalizeBackupMethod(item.BackupMethod) {
	case DatabaseBackupMethodPhysical, DatabaseBackupMethodExternal:
		return DatabaseRestoreCapabilityPhysicalRestore
	default:
		return DatabaseRestoreCapabilityLogicalRestoreOnly
	}
}

func (uc *UseCase) restorePlanInstanceNames(ctx context.Context, item *DatabaseRestorePlan) (string, string) {
	if item == nil || uc.instanceRepo == nil {
		return "", ""
	}
	sourceName := ""
	targetName := ""
	if item.SourceInstanceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, item.SourceInstanceID); err == nil && instance != nil {
			sourceName = instance.Name
		}
	}
	if item.TargetInstanceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, item.TargetInstanceID); err == nil && instance != nil {
			targetName = instance.Name
		}
	}
	return sourceName, targetName
}

func (uc *UseCase) toLogArchiveStreamVO(ctx context.Context, item *DatabaseLogArchiveStream) *DatabaseLogArchiveStreamVO {
	if item == nil {
		return nil
	}
	instanceName, sourceName := uc.archiveInstanceNames(ctx, item.InstanceID, item.SourceInstanceID)
	runnerHostName := uc.archiveRunnerHostName(ctx, item.RunnerHostID)
	return &DatabaseLogArchiveStreamVO{
		ID:                  item.ID,
		InstanceID:          item.InstanceID,
		InstanceName:        instanceName,
		SourceInstanceID:    item.SourceInstanceID,
		SourceInstanceName:  sourceName,
		Engine:              item.Engine,
		EngineText:          DBTypeText(item.Engine),
		ArchiveType:         item.ArchiveType,
		ArchiveTypeText:     ArchiveTypeText(item.ArchiveType),
		ArchiveMode:         item.ArchiveMode,
		ArchiveModeText:     ArchiveModeText(item.ArchiveMode),
		ArchiveEngine:       item.ArchiveEngine,
		RunnerHostID:        item.RunnerHostID,
		RunnerHostName:      runnerHostName,
		StorageProfileID:    item.StorageProfileID,
		SecretProfileID:     item.SecretProfileID,
		RPOTargetSeconds:    item.RPOTargetSeconds,
		RetentionDays:       item.RetentionDays,
		Enabled:             item.Enabled,
		Status:              item.Status,
		StatusText:          LogArchiveStreamStatusText(item.Status),
		DesiredState:        normalizeLogArchiveDesiredState(item.DesiredState),
		DesiredStateText:    LogArchiveDesiredStateText(item.DesiredState),
		DaemonStatus:        normalizeLogArchiveDaemonStatus(item.DaemonStatus),
		DaemonStatusText:    LogArchiveDaemonStatusText(item.DaemonStatus),
		CursorFile:          item.CursorFile,
		CursorPos:           item.CursorPos,
		CursorGTIDSet:       item.CursorGTIDSet,
		ActiveFile:          item.ActiveFile,
		LastSourceFile:      item.LastSourceFile,
		LastSourcePos:       item.LastSourcePos,
		LastEventTime:       formatTime(item.LastEventTime),
		ArchiveLagSeconds:   item.ArchiveLagSeconds,
		LastHeartbeatAt:     formatTime(item.LastHeartbeatAt),
		ConsecutiveFailures: item.ConsecutiveFailures,
		LeaseOwner:          item.LeaseOwner,
		LeaseExpiresAt:      formatTime(item.LeaseExpiresAt),
		PausedAt:            formatTime(item.PausedAt),
		PausedReason:        item.PausedReason,
		LastArchivedAt:      formatTime(item.LastArchivedAt),
		LastArchiveName:     item.LastArchiveName,
		LastError:           item.LastError,
		ConfigJSON:          item.ConfigJSON,
		CreatedAt:           item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:           item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) toLogArchiveVO(ctx context.Context, item *DatabaseLogArchive) *DatabaseLogArchiveVO {
	if item == nil {
		return nil
	}
	instanceName, sourceName := uc.archiveInstanceNames(ctx, item.InstanceID, item.SourceInstanceID)
	return &DatabaseLogArchiveVO{
		ID:                 item.ID,
		StreamID:           item.StreamID,
		InstanceID:         item.InstanceID,
		InstanceName:       instanceName,
		SourceInstanceID:   item.SourceInstanceID,
		SourceInstanceName: sourceName,
		Engine:             item.Engine,
		EngineText:         DBTypeText(item.Engine),
		ArchiveType:        item.ArchiveType,
		ArchiveTypeText:    ArchiveTypeText(item.ArchiveType),
		FileName:           item.FileName,
		StorageURI:         item.StorageURI,
		FileSize:           item.FileSize,
		ChecksumSHA256:     item.ChecksumSHA256,
		FirstEventTime:     formatTime(item.FirstEventTime),
		LastEventTime:      formatTime(item.LastEventTime),
		Status:             item.Status,
		StatusText:         LogArchiveStatusText(item.Status),
		ArchivedAt:         formatTime(item.ArchivedAt),
		PGSystemIdentifier: item.PGSystemIdentifier,
		TimelineID:         item.TimelineID,
		WALSegmentSize:     item.WALSegmentSize,
		ExternalServerName: item.ExternalServerName,
		StartLSN:           item.StartLSN,
		EndLSN:             item.EndLSN,
		SegmentNo:          item.SegmentNo,
		TimelineHistoryURI: item.TimelineHistoryURI,
		CreatedAt:          item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:          item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) toLogArchiveEventVO(ctx context.Context, item *DatabaseLogArchiveEvent) *DatabaseLogArchiveEventVO {
	if item == nil {
		return nil
	}
	instanceName, sourceName := uc.archiveInstanceNames(ctx, item.InstanceID, item.SourceInstanceID)
	return &DatabaseLogArchiveEventVO{
		ID:                 item.ID,
		StreamID:           item.StreamID,
		InstanceID:         item.InstanceID,
		InstanceName:       instanceName,
		SourceInstanceID:   item.SourceInstanceID,
		SourceInstanceName: sourceName,
		RunnerHostID:       item.RunnerHostID,
		RunnerHostName:     uc.archiveRunnerHostName(ctx, item.RunnerHostID),
		RunnerID:           item.RunnerID,
		EventType:          normalizeLogArchiveEventType(item.EventType),
		EventTypeText:      LogArchiveEventTypeText(item.EventType),
		Level:              normalizeLogArchiveEventLevel(item.Level),
		LevelText:          LogArchiveEventLevelText(item.Level),
		Message:            item.Message,
		FileName:           item.FileName,
		CursorFile:         item.CursorFile,
		CursorPos:          item.CursorPos,
		ActiveFile:         item.ActiveFile,
		ArchiveLagSeconds:  item.ArchiveLagSeconds,
		PayloadJSON:        item.PayloadJSON,
		OccurredAt:         formatTime(item.OccurredAt),
		CreatedAt:          item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) recordLogArchiveEvent(ctx context.Context, item *DatabaseLogArchiveEvent) error {
	if uc.logArchiveEventRepo == nil || item == nil {
		return nil
	}
	if item.OccurredAt == nil || item.OccurredAt.IsZero() {
		item.OccurredAt = ptrTime(time.Now())
	}
	item.EventType = normalizeLogArchiveEventType(item.EventType)
	item.Level = normalizeLogArchiveEventLevel(item.Level)
	item.Message = trimText(strings.TrimSpace(item.Message), 1000)
	item.FileName = trimText(strings.TrimSpace(item.FileName), 255)
	item.CursorFile = trimText(strings.TrimSpace(item.CursorFile), 255)
	item.ActiveFile = trimText(strings.TrimSpace(item.ActiveFile), 255)
	item.RunnerID = trimText(strings.TrimSpace(item.RunnerID), 120)
	item.PayloadJSON = trimText(strings.TrimSpace(item.PayloadJSON), 4000)
	if err := uc.logArchiveEventRepo.Create(ctx, item); err != nil {
		return err
	}
	if isHighFrequencyLogArchiveEvent(item.EventType) {
		_ = uc.logArchiveEventRepo.UpsertRollup(ctx, buildLogArchiveEventRollup(item))
	}
	uc.pruneLogArchiveEvents(ctx, item.StreamID)
	return nil
}

func (uc *UseCase) pruneLogArchiveEvents(ctx context.Context, streamID uint) {
	if uc.logArchiveEventRepo == nil {
		return
	}
	retentionDays := 30
	if streamID > 0 && uc.logArchiveStreamRepo != nil {
		if stream, err := uc.logArchiveStreamRepo.GetByID(ctx, streamID); err == nil && stream != nil {
			retentionDays = normalizeArchiveRetentionDays(stream.RetentionDays)
		}
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if retentionDays > 3650 {
		retentionDays = 3650
	}
	before := time.Now().AddDate(0, 0, -retentionDays)
	highFrequencyRetentionDays := retentionDays
	if highFrequencyRetentionDays > 7 {
		highFrequencyRetentionDays = 7
	}
	if highFrequencyRetentionDays > 0 && highFrequencyRetentionDays < retentionDays {
		_, _ = uc.logArchiveEventRepo.DeleteHighFrequencyBefore(ctx, time.Now().AddDate(0, 0, -highFrequencyRetentionDays), streamID, highFrequencyLogArchiveEventTypes())
	}
	_, _ = uc.logArchiveEventRepo.DeleteBefore(ctx, before, streamID)
}

func highFrequencyLogArchiveEventTypes() []string {
	return []string{
		DatabaseLogArchiveEventCheckpoint,
		DatabaseLogArchiveEventSpoolUpdated,
		DatabaseLogArchiveEventAgentMessage,
	}
}

func isHighFrequencyLogArchiveEvent(eventType string) bool {
	eventType = normalizeLogArchiveEventType(eventType)
	for _, candidate := range highFrequencyLogArchiveEventTypes() {
		if eventType == candidate {
			return true
		}
	}
	return false
}

func buildLogArchiveEventRollup(item *DatabaseLogArchiveEvent) *DatabaseLogArchiveEventRollup {
	if item == nil {
		return nil
	}
	occurredAt := time.Now()
	if item.OccurredAt != nil && !item.OccurredAt.IsZero() {
		occurredAt = *item.OccurredAt
	}
	bucketStart := occurredAt.Truncate(time.Hour)
	level := normalizeLogArchiveEventLevel(item.Level)
	rollup := &DatabaseLogArchiveEventRollup{
		StreamID:         item.StreamID,
		InstanceID:       item.InstanceID,
		SourceInstanceID: item.SourceInstanceID,
		RunnerHostID:     item.RunnerHostID,
		RunnerID:         item.RunnerID,
		EventType:        normalizeLogArchiveEventType(item.EventType),
		Level:            level,
		BucketStart:      bucketStart,
		BucketEnd:        bucketStart.Add(time.Hour),
		EventCount:       1,
		MinLagSeconds:    item.ArchiveLagSeconds,
		MaxLagSeconds:    item.ArchiveLagSeconds,
		LastCursorFile:   item.CursorFile,
		LastCursorPos:    item.CursorPos,
		LastActiveFile:   item.ActiveFile,
		LastMessage:      item.Message,
		LastPayloadJSON:  item.PayloadJSON,
		LastOccurredAt:   occurredAt,
	}
	if level == DatabaseLogArchiveEventLevelWarning {
		rollup.WarningCount = 1
	}
	if level == DatabaseLogArchiveEventLevelError {
		rollup.ErrorCount = 1
	}
	return rollup
}

func (uc *UseCase) archiveInstanceNames(ctx context.Context, instanceID, sourceID uint) (string, string) {
	instanceName := ""
	sourceName := ""
	if uc.instanceRepo == nil {
		return instanceName, sourceName
	}
	if instanceID > 0 {
		if instance, err := uc.instanceRepo.GetByID(ctx, instanceID); err == nil && instance != nil {
			instanceName = instance.Name
		}
	}
	if sourceID > 0 {
		if source, err := uc.instanceRepo.GetByID(ctx, sourceID); err == nil && source != nil {
			sourceName = source.Name
		}
	}
	return instanceName, sourceName
}

func streamWALSegmentSize(stream *DatabaseLogArchiveStream) int64 {
	if stream == nil || strings.TrimSpace(stream.ConfigJSON) == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stream.ConfigJSON), &payload); err != nil {
		return 0
	}
	switch value := payload["walSegmentSize"].(type) {
	case float64:
		return int64(value)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed
	default:
		return 0
	}
}

func streamExternalServerName(stream *DatabaseLogArchiveStream) string {
	if stream == nil || strings.TrimSpace(stream.ConfigJSON) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stream.ConfigJSON), &payload); err != nil {
		return ""
	}
	if value, ok := payload["barmanServerName"].(string); ok {
		return trimText(strings.TrimSpace(value), 120)
	}
	if value, ok := payload["externalServerName"].(string); ok {
		return trimText(strings.TrimSpace(value), 120)
	}
	return ""
}

func (uc *UseCase) toRestorePlanVO(item *DatabaseRestorePlan, sourceName, targetName string) *DatabaseRestorePlanVO {
	if item == nil {
		return nil
	}
	runnerHostName := ""
	if item.RunnerHostID > 0 && uc != nil && uc.runnerHostRepo != nil {
		if host, err := uc.runnerHostRepo.GetByID(context.Background(), item.RunnerHostID); err == nil && host != nil {
			runnerHostName = host.Name
		}
	}
	return &DatabaseRestorePlanVO{
		ID:                      item.ID,
		SourceInstanceID:        item.SourceInstanceID,
		SourceInstanceName:      sourceName,
		TargetInstanceID:        item.TargetInstanceID,
		TargetInstanceName:      targetName,
		RunnerHostID:            item.RunnerHostID,
		RunnerHostName:          runnerHostName,
		RestoreMode:             item.RestoreMode,
		RestoreModeText:         RestoreModeText(item.RestoreMode),
		RestoreTargetType:       item.RestoreTargetType,
		RestoreTargetValue:      item.RestoreTargetValue,
		RestoreTargetInclusive:  item.RestoreTargetInclusive,
		SelectedBaseRecordID:    item.SelectedBaseRecordID,
		SelectedBackupRecordIDs: item.SelectedBackupRecordIDs,
		SelectedLogArchiveIDs:   item.SelectedLogArchiveIDs,
		BackupChainStatus:       item.BackupChainStatus,
		BackupChainStatusText:   BackupChainStatusText(item.BackupChainStatus),
		LogChainStatus:          item.LogChainStatus,
		LogChainStatusText:      LogChainStatusText(item.LogChainStatus),
		StorageStatus:           item.StorageStatus,
		StorageStatusText:       StorageStatusText(item.StorageStatus),
		ToolStatus:              item.ToolStatus,
		ToolStatusText:          ToolStatusText(item.ToolStatus),
		ValidationStatus:        item.ValidationStatus,
		ValidationStatusText:    ValidationStatusText(item.ValidationStatus),
		RestoreStatus:           item.RestoreStatus,
		RestoreStatusText:       RestoreStatusText(item.RestoreStatus),
		RequiredToolJSON:        item.RequiredToolJSON,
		RequiredArtifactJSON:    item.RequiredArtifactJSON,
		EstimatedRestoreBytes:   item.EstimatedRestoreBytes,
		EstimatedRestoreMinutes: item.EstimatedRestoreMinutes,
		PlanJSON:                item.PlanJSON,
		ProofJSON:               item.ProofJSON,
		OperatorID:              item.OperatorID,
		OperatorName:            item.OperatorName,
		StartedAt:               formatTime(item.StartedAt),
		FinishedAt:              formatTime(item.FinishedAt),
		DurationMs:              item.DurationMs,
		Message:                 item.ErrorMessage,
		CreatedAt:               item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:               item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toStorageProfileVO(item *DatabaseStorageProfile) *DatabaseStorageProfileVO {
	if item == nil {
		return nil
	}
	return &DatabaseStorageProfileVO{
		ID:                  item.ID,
		Name:                item.Name,
		StorageType:         item.StorageType,
		StorageTypeText:     StorageProfileTypeText(item.StorageType),
		Endpoint:            item.Endpoint,
		Bucket:              item.Bucket,
		Region:              item.Region,
		PathPrefix:          item.PathPrefix,
		SecretProfileID:     item.SecretProfileID,
		VersioningEnabled:   item.VersioningEnabled,
		ImmutabilityEnabled: item.ImmutabilityEnabled,
		KMSKeyID:            item.KMSKeyID,
		RetentionLockDays:   item.RetentionLockDays,
		Status:              item.Status,
		StatusText:          ProfileStatusText(item.Status),
		LastTestAt:          formatTime(item.LastTestAt),
		PostureStatus:       normalizeStoragePostureStatus(item.PostureStatus),
		PostureStatusText:   StoragePostureStatusText(item.PostureStatus),
		PostureSummary:      item.PostureSummary,
		PostureJSON:         item.PostureJSON,
		LastPostureCheckAt:  formatTime(item.LastPostureCheckAt),
		CreatedAt:           item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:           item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toSecretProfileVO(item *DatabaseSecretProfile) *DatabaseSecretProfileVO {
	if item == nil {
		return nil
	}
	return &DatabaseSecretProfileVO{
		ID:             item.ID,
		Name:           item.Name,
		SecretType:     item.SecretType,
		SecretTypeText: SecretProfileTypeText(item.SecretType),
		CredentialID:   item.CredentialID,
		ExternalRef:    item.ExternalRef,
		Status:         item.Status,
		StatusText:     ProfileStatusText(item.Status),
		LastRotatedAt:  formatTime(item.LastRotatedAt),
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func normalizeBackupMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseBackupMethodPhysical:
		return DatabaseBackupMethodPhysical
	case DatabaseBackupMethodExternal:
		return DatabaseBackupMethodExternal
	default:
		return DatabaseBackupMethodLogical
	}
}

func normalizeBackupLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseBackupLevelIncremental:
		return DatabaseBackupLevelIncremental
	case DatabaseBackupLevelDifferential:
		return DatabaseBackupLevelDifferential
	default:
		return DatabaseBackupLevelFull
	}
}

func normalizeBackupEngine(value, method string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		return trimText(value, 60)
	}
	switch normalizeBackupMethod(method) {
	case DatabaseBackupMethodPhysical:
		return "external_physical"
	case DatabaseBackupMethodExternal:
		return "external"
	default:
		return "logical"
	}
}

func normalizeBackupRecordStatus(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupStatusFailed:
		return DatabaseBackupStatusFailed
	case DatabaseBackupStatusExpired:
		return DatabaseBackupStatusExpired
	default:
		return DatabaseBackupStatusSuccess
	}
}

func normalizeBackupVerifyStatus(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupVerifyStatusFailed:
		return DatabaseBackupVerifyStatusFailed
	case DatabaseBackupVerifyStatusExpired:
		return DatabaseBackupVerifyStatusExpired
	case DatabaseBackupVerifyStatusPending:
		return DatabaseBackupVerifyStatusPending
	default:
		return DatabaseBackupVerifyStatusSuccess
	}
}

func normalizeSourceRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "replica", "delayed_replica", "external":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "primary"
	}
}

func normalizeArchiveType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseArchiveTypeBinlog:
		return DatabaseArchiveTypeBinlog
	case DatabaseArchiveTypeWAL:
		return DatabaseArchiveTypeWAL
	case DatabaseArchiveTypeNone:
		return DatabaseArchiveTypeNone
	default:
		return ""
	}
}

func archiveTypeForEngine(engine string) string {
	switch normalizeDBType(engine) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeTiDB, DBTypeOceanBase:
		return DatabaseArchiveTypeBinlog
	case DBTypePostgreSQL, DBTypeOpenGauss:
		return DatabaseArchiveTypeWAL
	default:
		return ""
	}
}

func normalizeArchiveMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", DatabaseArchiveModeExternal:
		return DatabaseArchiveModeExternal
	case DatabaseArchiveModeManual, "manual", "once", "run_once":
		return DatabaseArchiveModeManual
	case DatabaseArchiveModeCatchUp, "catchup":
		return DatabaseArchiveModeCatchUp
	case DatabaseArchiveModePolling, "poll", "high_frequency_poll", "high_frequency_polling":
		return DatabaseArchiveModePolling
	case DatabaseArchiveModeStreaming, "stream":
		return DatabaseArchiveModeStreaming
	default:
		return trimText(value, 30)
	}
}

func normalizeArchiveEngine(value, archiveType string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		return trimText(value, 60)
	}
	if archiveType == DatabaseArchiveTypeWAL {
		return "external_wal"
	}
	return "external_binlog"
}

func normalizeArchiveRetentionDays(value int) int {
	if value <= 0 {
		return 30
	}
	return value
}

func normalizeLogArchiveStatus(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseLogArchiveStatusMissing, DatabaseLogArchiveStatusChecksumFailed, DatabaseLogArchiveStatusExpired:
		return strings.TrimSpace(value)
	default:
		return DatabaseLogArchiveStatusArchived
	}
}

func normalizeLogArchiveEventLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseLogArchiveEventLevelWarning:
		return DatabaseLogArchiveEventLevelWarning
	case DatabaseLogArchiveEventLevelError:
		return DatabaseLogArchiveEventLevelError
	default:
		return DatabaseLogArchiveEventLevelInfo
	}
}

func normalizeLogArchiveEventType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseLogArchiveEventRunnerHeartbeat:
		return DatabaseLogArchiveEventRunnerHeartbeat
	case DatabaseLogArchiveEventLeaseAcquired:
		return DatabaseLogArchiveEventLeaseAcquired
	case DatabaseLogArchiveEventCheckpoint:
		return DatabaseLogArchiveEventCheckpoint
	case DatabaseLogArchiveEventStateChanged:
		return DatabaseLogArchiveEventStateChanged
	case DatabaseLogArchiveEventArchiveSuccess:
		return DatabaseLogArchiveEventArchiveSuccess
	case DatabaseLogArchiveEventArchiveFailed:
		return DatabaseLogArchiveEventArchiveFailed
	case DatabaseLogArchiveEventSpoolUpdated:
		return DatabaseLogArchiveEventSpoolUpdated
	case DatabaseLogArchiveEventPurgeGap:
		return DatabaseLogArchiveEventPurgeGap
	default:
		return DatabaseLogArchiveEventAgentMessage
	}
}

func normalizeStorageProfileType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return DatabaseBackupStorageLocal
	}
	return trimText(value, 30)
}

func normalizeLogArchiveDesiredState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseLogArchiveDesiredStateRunning:
		return DatabaseLogArchiveDesiredStateRunning
	case DatabaseLogArchiveDesiredStatePaused:
		return DatabaseLogArchiveDesiredStatePaused
	default:
		return DatabaseLogArchiveDesiredStateStopped
	}
}

func normalizeLogArchiveDaemonStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseLogArchiveDaemonStatusStarting:
		return DatabaseLogArchiveDaemonStatusStarting
	case DatabaseLogArchiveDaemonStatusRunning:
		return DatabaseLogArchiveDaemonStatusRunning
	case DatabaseLogArchiveDaemonStatusPaused:
		return DatabaseLogArchiveDaemonStatusPaused
	case DatabaseLogArchiveDaemonStatusDegraded:
		return DatabaseLogArchiveDaemonStatusDegraded
	case DatabaseLogArchiveDaemonStatusFailed:
		return DatabaseLogArchiveDaemonStatusFailed
	default:
		return DatabaseLogArchiveDaemonStatusStopped
	}
}

func normalizeSecretProfileType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "external_ref"
	}
	return trimText(value, 30)
}

func normalizeProfileStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return DatabaseLogArchiveStreamStatusPending
	}
	return trimText(value, 30)
}

func normalizeStoragePostureStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "passed":
		return "passed"
	case "warning":
		return "warning"
	case "failed":
		return "failed"
	case "unsupported":
		return "unsupported"
	default:
		return "unknown"
	}
}

func parseDatabaseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		var (
			parsed time.Time
			err    error
		)
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			return &parsed, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func marshalUintList(values []uint) string {
	data, _ := json.Marshal(values)
	return string(data)
}

func marshalBackupPlanJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func BackupMethodText(value string) string {
	switch normalizeBackupMethod(value) {
	case DatabaseBackupMethodPhysical:
		return "物理备份"
	case DatabaseBackupMethodExternal:
		return "外部备份"
	default:
		return "逻辑备份"
	}
}

func BackupLevelText(value string) string {
	switch normalizeBackupLevel(value) {
	case DatabaseBackupLevelIncremental:
		return "增量"
	case DatabaseBackupLevelDifferential:
		return "差异"
	default:
		return "全量"
	}
}

func ArchiveTypeText(value string) string {
	switch normalizeArchiveType(value) {
	case DatabaseArchiveTypeWAL:
		return "WAL"
	case DatabaseArchiveTypeBinlog:
		return "binlog"
	default:
		return "无"
	}
}

func ArchiveModeText(value string) string {
	switch normalizeArchiveMode(value) {
	case DatabaseArchiveModeManual:
		return "手动一次"
	case DatabaseArchiveModeCatchUp:
		return "追平"
	case DatabaseArchiveModePolling:
		return "轮询"
	case DatabaseArchiveModeStreaming:
		return "Streaming"
	default:
		return "外部登记"
	}
}

func LogArchiveStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseLogArchiveStatusMissing:
		return "缺失"
	case DatabaseLogArchiveStatusChecksumFailed:
		return "校验失败"
	case DatabaseLogArchiveStatusExpired:
		return "已过期"
	default:
		return "已归档"
	}
}

func LogArchiveStreamStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseLogArchiveStreamStatusRunning:
		return "运行中"
	case DatabaseLogArchiveStreamStatusPaused:
		return "已暂停"
	case DatabaseLogArchiveStreamStatusDegraded:
		return "降级"
	case DatabaseLogArchiveStreamStatusFailed:
		return "失败"
	case DatabaseLogArchiveStreamStatusDisabled:
		return "已禁用"
	default:
		return "待接入"
	}
}

func LogArchiveDesiredStateText(value string) string {
	switch normalizeLogArchiveDesiredState(value) {
	case DatabaseLogArchiveDesiredStateRunning:
		return "期望运行"
	case DatabaseLogArchiveDesiredStatePaused:
		return "期望暂停"
	default:
		return "期望停止"
	}
}

func LogArchiveDaemonStatusText(value string) string {
	switch normalizeLogArchiveDaemonStatus(value) {
	case DatabaseLogArchiveDaemonStatusStarting:
		return "启动中"
	case DatabaseLogArchiveDaemonStatusRunning:
		return "运行中"
	case DatabaseLogArchiveDaemonStatusPaused:
		return "已暂停"
	case DatabaseLogArchiveDaemonStatusDegraded:
		return "降级"
	case DatabaseLogArchiveDaemonStatusFailed:
		return "失败"
	default:
		return "已停止"
	}
}

func LogArchiveEventLevelText(value string) string {
	switch normalizeLogArchiveEventLevel(value) {
	case DatabaseLogArchiveEventLevelWarning:
		return "警告"
	case DatabaseLogArchiveEventLevelError:
		return "错误"
	default:
		return "信息"
	}
}

func LogArchiveEventTypeText(value string) string {
	switch normalizeLogArchiveEventType(value) {
	case DatabaseLogArchiveEventRunnerHeartbeat:
		return "Runner 心跳"
	case DatabaseLogArchiveEventLeaseAcquired:
		return "租约获取"
	case DatabaseLogArchiveEventCheckpoint:
		return "Checkpoint"
	case DatabaseLogArchiveEventStateChanged:
		return "状态变更"
	case DatabaseLogArchiveEventArchiveSuccess:
		return "归档成功"
	case DatabaseLogArchiveEventArchiveFailed:
		return "归档失败"
	case DatabaseLogArchiveEventSpoolUpdated:
		return "Spool 更新"
	case DatabaseLogArchiveEventPurgeGap:
		return "日志断链"
	default:
		return "Agent 消息"
	}
}

func BackupChainStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseBackupChainStatusComplete:
		return "备份链完整"
	case DatabaseBackupChainStatusMissingBase:
		return "缺少 Base Backup"
	case DatabaseBackupChainStatusMissingIncremental:
		return "缺少增量"
	case DatabaseBackupChainStatusBrokenChain:
		return "备份链断裂"
	default:
		return "不支持"
	}
}

func LogChainStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseLogChainStatusComplete:
		return "日志链完整"
	case DatabaseLogChainStatusMissingWAL:
		return "缺少 WAL"
	case DatabaseLogChainStatusMissingBinlog:
		return "缺少 binlog"
	case DatabaseLogChainStatusTimelineGap:
		return "timeline 缺口"
	case DatabaseLogChainStatusGTIDGap:
		return "GTID 缺口"
	case DatabaseLogChainStatusTimeRangeGap:
		return "时间范围缺口"
	default:
		return "不支持"
	}
}

func StorageStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseStorageStatusAvailable:
		return "存储可用"
	case DatabaseStorageStatusMissingObject:
		return "对象缺失"
	case DatabaseStorageStatusChecksumFailed:
		return "校验失败"
	case DatabaseStorageStatusPermissionDenied:
		return "无存储权限"
	default:
		return "不支持"
	}
}

func ToolStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseToolStatusCompatible:
		return "工具兼容"
	case DatabaseToolStatusIncompatibleVersion:
		return "版本不兼容"
	case DatabaseToolStatusMissingTool:
		return "缺少工具"
	case DatabaseToolStatusPermissionDenied:
		return "无工具权限"
	default:
		return "不支持"
	}
}

func ValidationStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabasePlanValidationPassed:
		return "校验通过"
	case DatabasePlanValidationWarning:
		return "有风险"
	case DatabasePlanValidationFailed:
		return "校验失败"
	default:
		return "待校验"
	}
}

func RestoreStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseRestoreStatusQueued:
		return "排队中"
	case DatabaseRestoreStatusRunning:
		return "恢复中"
	case DatabaseRestoreStatusRestored:
		return "已恢复"
	case DatabaseRestoreStatusVerified:
		return "已验证"
	case DatabaseRestoreStatusFailed:
		return "失败"
	case DatabaseRestoreStatusCancelled:
		return "已取消"
	default:
		return "已计划"
	}
}

func StorageProfileTypeText(value string) string {
	switch normalizeStorageProfileType(value) {
	case "nfs":
		return "NFS"
	case "s3":
		return "S3"
	case "minio":
		return "MinIO"
	case "oss":
		return "OSS"
	case "cos":
		return "COS"
	case DatabaseBackupStorageExternal:
		return "外部存储"
	default:
		return "本地"
	}
}

func StoragePostureStatusText(value string) string {
	switch normalizeStoragePostureStatus(value) {
	case "passed":
		return "通过"
	case "warning":
		return "有风险"
	case "failed":
		return "检测失败"
	case "unsupported":
		return "不支持"
	default:
		return "未检测"
	}
}

func SecretProfileTypeText(value string) string {
	switch normalizeSecretProfileType(value) {
	case "db_credential":
		return "数据库凭据"
	case "ssh_key":
		return "SSH Key"
	case "object_storage_key":
		return "对象存储密钥"
	case "encryption_key":
		return "加密密钥"
	default:
		return "外部引用"
	}
}

func ProfileStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseQueryStatusSuccess:
		return "可用"
	case DatabaseQueryStatusFailed:
		return "失败"
	case DatabaseLogArchiveStreamStatusDisabled:
		return "已禁用"
	default:
		return "待验证"
	}
}
