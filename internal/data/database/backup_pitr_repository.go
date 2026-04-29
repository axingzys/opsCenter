package database

import (
	"context"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type logArchiveStreamRepo struct {
	db *gorm.DB
}

func NewLogArchiveStreamRepo(db *gorm.DB) dbbiz.LogArchiveStreamRepo {
	return &logArchiveStreamRepo{db: db}
}

func (r *logArchiveStreamRepo) Create(ctx context.Context, item *dbbiz.DatabaseLogArchiveStream) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *logArchiveStreamRepo) Update(ctx context.Context, item *dbbiz.DatabaseLogArchiveStream) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *logArchiveStreamRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseLogArchiveStream, error) {
	var item dbbiz.DatabaseLogArchiveStream
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *logArchiveStreamRepo) List(ctx context.Context, req *dbbiz.DatabaseLogArchiveStreamListRequest) ([]*dbbiz.DatabaseLogArchiveStream, int64, error) {
	var (
		items []*dbbiz.DatabaseLogArchiveStream
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseLogArchiveStream{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if archiveType := strings.TrimSpace(req.ArchiveType); archiveType != "" {
			query = query.Where("archive_type = ?", archiveType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *logArchiveStreamRepo) ListRunnableForRunner(ctx context.Context, runnerHostID uint) ([]*dbbiz.DatabaseLogArchiveStream, error) {
	var items []*dbbiz.DatabaseLogArchiveStream
	if runnerHostID == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseLogArchiveStream{}).
		Where("runner_host_id = ?", runnerHostID).
		Where("enabled = ?", true).
		Where("desired_state = ?", dbbiz.DatabaseLogArchiveDesiredStateRunning).
		Where("archive_type = ?", dbbiz.DatabaseArchiveTypeBinlog).
		Where("archive_mode IN ?", []string{dbbiz.DatabaseArchiveModePolling, dbbiz.DatabaseArchiveModeStreaming}).
		Where("status <> ?", dbbiz.DatabaseLogArchiveStreamStatusDisabled).
		Order("id ASC").
		Limit(100).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *logArchiveStreamRepo) TryAcquireLease(ctx context.Context, streamID, runnerHostID uint, runnerID string, now, leaseExpiresAt time.Time) (bool, error) {
	if streamID == 0 || runnerHostID == 0 || strings.TrimSpace(runnerID) == "" {
		return false, nil
	}
	result := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseLogArchiveStream{}).
		Where("id = ? AND runner_host_id = ?", streamID, runnerHostID).
		Where("enabled = ? AND desired_state = ?", true, dbbiz.DatabaseLogArchiveDesiredStateRunning).
		Where("status <> ?", dbbiz.DatabaseLogArchiveStreamStatusDisabled).
		Where("(lease_owner = '' OR lease_owner = ? OR lease_expires_at IS NULL OR lease_expires_at < ?)", runnerID, now).
		Updates(map[string]any{
			"lease_owner":       runnerID,
			"lease_expires_at":  leaseExpiresAt,
			"last_heartbeat_at": now,
			"daemon_status":     dbbiz.DatabaseLogArchiveDaemonStatusStarting,
			"status":            dbbiz.DatabaseLogArchiveStreamStatusPending,
			"last_error":        "Runner Agent 已获取租约，等待归档 checkpoint",
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

type logArchiveRepo struct {
	db *gorm.DB
}

func NewLogArchiveRepo(db *gorm.DB) dbbiz.LogArchiveRepo {
	return &logArchiveRepo{db: db}
}

func (r *logArchiveRepo) Create(ctx context.Context, item *dbbiz.DatabaseLogArchive) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *logArchiveRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseLogArchive, error) {
	var item dbbiz.DatabaseLogArchive
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *logArchiveRepo) List(ctx context.Context, req *dbbiz.DatabaseLogArchiveListRequest) ([]*dbbiz.DatabaseLogArchive, int64, error) {
	var (
		items []*dbbiz.DatabaseLogArchive
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseLogArchive{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.StreamID > 0 {
			query = query.Where("stream_id = ?", req.StreamID)
		}
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if archiveType := strings.TrimSpace(req.ArchiveType); archiveType != "" {
			query = query.Where("archive_type = ?", archiveType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("first_event_time DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *logArchiveRepo) ListCoveringTimeRange(ctx context.Context, instanceID uint, archiveType string, startTime, endTime time.Time) ([]*dbbiz.DatabaseLogArchive, error) {
	var items []*dbbiz.DatabaseLogArchive
	if err := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseLogArchive{}).
		Where("instance_id = ?", instanceID).
		Where("archive_type = ?", archiveType).
		Where("status IN ?", []string{dbbiz.DatabaseLogArchiveStatusArchived, dbbiz.DatabaseLogArchiveStatusChecksumFailed, dbbiz.DatabaseLogArchiveStatusMissing}).
		Where("last_event_time >= ? AND first_event_time <= ?", startTime, endTime).
		Order("first_event_time ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type logArchiveEventRepo struct {
	db *gorm.DB
}

func NewLogArchiveEventRepo(db *gorm.DB) dbbiz.LogArchiveEventRepo {
	return &logArchiveEventRepo{db: db}
}

func (r *logArchiveEventRepo) Create(ctx context.Context, item *dbbiz.DatabaseLogArchiveEvent) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *logArchiveEventRepo) List(ctx context.Context, req *dbbiz.DatabaseLogArchiveEventListRequest) ([]*dbbiz.DatabaseLogArchiveEvent, int64, error) {
	var (
		items []*dbbiz.DatabaseLogArchiveEvent
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseLogArchiveEvent{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.StreamID > 0 {
			query = query.Where("stream_id = ?", req.StreamID)
		}
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if req.RunnerHostID > 0 {
			query = query.Where("runner_host_id = ?", req.RunnerHostID)
		}
		if level := strings.TrimSpace(req.Level); level != "" {
			query = query.Where("level = ?", level)
		}
		if eventType := strings.TrimSpace(req.EventType); eventType != "" {
			query = query.Where("event_type = ?", eventType)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("occurred_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *logArchiveEventRepo) UpsertRollup(ctx context.Context, item *dbbiz.DatabaseLogArchiveEventRollup) error {
	if item == nil || item.StreamID == 0 || item.EventType == "" || item.BucketStart.IsZero() {
		return nil
	}
	now := time.Now()
	item.UpdatedAt = now
	updates := map[string]any{
		"instance_id":        item.InstanceID,
		"source_instance_id": item.SourceInstanceID,
		"runner_host_id":     item.RunnerHostID,
		"runner_id":          item.RunnerID,
		"bucket_end":         item.BucketEnd,
		"event_count":        gorm.Expr("event_count + ?", item.EventCount),
		"warning_count":      gorm.Expr("warning_count + ?", item.WarningCount),
		"error_count":        gorm.Expr("error_count + ?", item.ErrorCount),
		"level":              rollupLevelExpr(item.Level),
		"min_lag_seconds":    gorm.Expr("CASE WHEN min_lag_seconds = 0 OR (? > 0 AND ? < min_lag_seconds) THEN ? ELSE min_lag_seconds END", item.MinLagSeconds, item.MinLagSeconds, item.MinLagSeconds),
		"max_lag_seconds":    gorm.Expr("CASE WHEN ? > max_lag_seconds THEN ? ELSE max_lag_seconds END", item.MaxLagSeconds, item.MaxLagSeconds),
		"last_cursor_file":   rollupLatestValueExpr("last_cursor_file", item.LastCursorFile, item.LastOccurredAt),
		"last_cursor_pos":    rollupLatestValueExpr("last_cursor_pos", item.LastCursorPos, item.LastOccurredAt),
		"last_active_file":   rollupLatestValueExpr("last_active_file", item.LastActiveFile, item.LastOccurredAt),
		"last_message":       rollupLatestValueExpr("last_message", item.LastMessage, item.LastOccurredAt),
		"last_payload_json":  rollupLatestValueExpr("last_payload_json", item.LastPayloadJSON, item.LastOccurredAt),
		"last_occurred_at":   rollupLatestValueExpr("last_occurred_at", item.LastOccurredAt, item.LastOccurredAt),
		"updated_at":         now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "stream_id"},
			{Name: "event_type"},
			{Name: "bucket_start"},
		},
		DoUpdates: clause.Assignments(updates),
	}).Create(item).Error
}

func rollupLevelExpr(level string) clause.Expr {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		level = dbbiz.DatabaseLogArchiveEventLevelInfo
	}
	return gorm.Expr(
		"CASE WHEN level = ? OR ? = ? THEN ? WHEN level = ? OR ? = ? THEN ? ELSE ? END",
		dbbiz.DatabaseLogArchiveEventLevelError,
		level,
		dbbiz.DatabaseLogArchiveEventLevelError,
		dbbiz.DatabaseLogArchiveEventLevelError,
		dbbiz.DatabaseLogArchiveEventLevelWarning,
		level,
		dbbiz.DatabaseLogArchiveEventLevelWarning,
		dbbiz.DatabaseLogArchiveEventLevelWarning,
		dbbiz.DatabaseLogArchiveEventLevelInfo,
	)
}

func rollupLatestValueExpr(column string, value any, occurredAt time.Time) clause.Expr {
	return gorm.Expr("CASE WHEN last_occurred_at IS NULL OR ? >= last_occurred_at THEN ? ELSE "+column+" END", occurredAt, value)
}

func (r *logArchiveEventRepo) DeleteBefore(ctx context.Context, before time.Time, streamID uint) (int64, error) {
	if before.IsZero() {
		return 0, nil
	}
	query := r.db.WithContext(ctx).Where("occurred_at < ?", before)
	if streamID > 0 {
		query = query.Where("stream_id = ?", streamID)
	} else {
		query = query.Where("stream_id = 0")
	}
	result := query.Unscoped().Delete(&dbbiz.DatabaseLogArchiveEvent{})
	return result.RowsAffected, result.Error
}

func (r *logArchiveEventRepo) DeleteHighFrequencyBefore(ctx context.Context, before time.Time, streamID uint, eventTypes []string) (int64, error) {
	if before.IsZero() || len(eventTypes) == 0 {
		return 0, nil
	}
	query := r.db.WithContext(ctx).
		Where("occurred_at < ?", before).
		Where("event_type IN ?", eventTypes).
		Where("(level = ? OR level = '' OR level IS NULL)", dbbiz.DatabaseLogArchiveEventLevelInfo)
	if streamID > 0 {
		query = query.Where("stream_id = ?", streamID)
	} else {
		query = query.Where("stream_id = 0")
	}
	result := query.Unscoped().Delete(&dbbiz.DatabaseLogArchiveEvent{})
	return result.RowsAffected, result.Error
}

type restorePlanRepo struct {
	db *gorm.DB
}

func NewRestorePlanRepo(db *gorm.DB) dbbiz.RestorePlanRepo {
	return &restorePlanRepo{db: db}
}

func (r *restorePlanRepo) Create(ctx context.Context, item *dbbiz.DatabaseRestorePlan) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *restorePlanRepo) Update(ctx context.Context, item *dbbiz.DatabaseRestorePlan) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *restorePlanRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseRestorePlan, error) {
	var item dbbiz.DatabaseRestorePlan
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *restorePlanRepo) List(ctx context.Context, req *dbbiz.DatabaseRestorePlanListRequest) ([]*dbbiz.DatabaseRestorePlan, int64, error) {
	var (
		items []*dbbiz.DatabaseRestorePlan
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseRestorePlan{})
	if req != nil {
		query = applyAllowedRestoreInstanceFilter(query, req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.SourceInstanceID > 0 {
			query = query.Where("source_instance_id = ?", req.SourceInstanceID)
		}
		if req.TargetInstanceID > 0 {
			query = query.Where("target_instance_id = ?", req.TargetInstanceID)
		}
		if status := strings.TrimSpace(req.ValidationStatus); status != "" {
			query = query.Where("validation_status = ?", status)
		}
		if status := strings.TrimSpace(req.RestoreStatus); status != "" {
			query = query.Where("restore_status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type storageProfileRepo struct {
	db *gorm.DB
}

func NewStorageProfileRepo(db *gorm.DB) dbbiz.StorageProfileRepo {
	return &storageProfileRepo{db: db}
}

func (r *storageProfileRepo) Create(ctx context.Context, item *dbbiz.DatabaseStorageProfile) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *storageProfileRepo) Update(ctx context.Context, item *dbbiz.DatabaseStorageProfile) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *storageProfileRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseStorageProfile, error) {
	var item dbbiz.DatabaseStorageProfile
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *storageProfileRepo) List(ctx context.Context, req *dbbiz.DatabaseStorageProfileListRequest) ([]*dbbiz.DatabaseStorageProfile, int64, error) {
	var (
		items []*dbbiz.DatabaseStorageProfile
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseStorageProfile{})
	if req != nil {
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ? OR bucket LIKE ? OR path_prefix LIKE ?", like, like, like)
		}
		if storageType := strings.TrimSpace(req.StorageType); storageType != "" {
			query = query.Where("storage_type = ?", storageType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type secretProfileRepo struct {
	db *gorm.DB
}

func NewSecretProfileRepo(db *gorm.DB) dbbiz.SecretProfileRepo {
	return &secretProfileRepo{db: db}
}

func (r *secretProfileRepo) Create(ctx context.Context, item *dbbiz.DatabaseSecretProfile) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *secretProfileRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseSecretProfile, error) {
	var item dbbiz.DatabaseSecretProfile
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *secretProfileRepo) List(ctx context.Context, req *dbbiz.DatabaseSecretProfileListRequest) ([]*dbbiz.DatabaseSecretProfile, int64, error) {
	var (
		items []*dbbiz.DatabaseSecretProfile
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseSecretProfile{})
	if req != nil {
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ? OR external_ref LIKE ?", like, like)
		}
		if secretType := strings.TrimSpace(req.SecretType); secretType != "" {
			query = query.Where("secret_type = ?", secretType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type runnerJobRepo struct {
	db *gorm.DB
}

type runnerHostRepo struct {
	db *gorm.DB
}

func NewRunnerHostRepo(db *gorm.DB) dbbiz.RunnerHostRepo {
	return &runnerHostRepo{db: db}
}

func (r *runnerHostRepo) Create(ctx context.Context, item *dbbiz.DatabaseRunnerHost) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *runnerHostRepo) Update(ctx context.Context, item *dbbiz.DatabaseRunnerHost) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *runnerHostRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseRunnerHost, error) {
	var item dbbiz.DatabaseRunnerHost
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *runnerHostRepo) List(ctx context.Context, req *dbbiz.DatabaseRunnerHostListRequest) ([]*dbbiz.DatabaseRunnerHost, int64, error) {
	var (
		items []*dbbiz.DatabaseRunnerHost
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseRunnerHost{})
	if req != nil {
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ? OR host LIKE ?", like, like)
		}
		if runnerType := strings.TrimSpace(req.RunnerType); runnerType != "" {
			query = query.Where("runner_type = ?", runnerType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if req.Enabled != "" {
			query = query.Where("enabled = ?", req.Enabled == "true" || req.Enabled == "1")
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func NewRunnerJobRepo(db *gorm.DB) dbbiz.RunnerJobRepo {
	return &runnerJobRepo{db: db}
}

func (r *runnerJobRepo) Create(ctx context.Context, item *dbbiz.DatabaseRunnerJob) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *runnerJobRepo) Update(ctx context.Context, item *dbbiz.DatabaseRunnerJob) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *runnerJobRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseRunnerJob, error) {
	var item dbbiz.DatabaseRunnerJob
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *runnerJobRepo) List(ctx context.Context, req *dbbiz.DatabaseRunnerJobListRequest) ([]*dbbiz.DatabaseRunnerJob, int64, error) {
	var (
		items []*dbbiz.DatabaseRunnerJob
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseRunnerJob{})
	if req != nil {
		if req.RunnerHostID > 0 {
			query = query.Where("runner_host_id = ?", req.RunnerHostID)
		}
		if req.SourceInstanceID > 0 {
			query = query.Where("source_instance_id = ?", req.SourceInstanceID)
		}
		if req.TargetInstanceID > 0 {
			query = query.Where("target_instance_id = ?", req.TargetInstanceID)
		}
		if jobType := strings.TrimSpace(req.JobType); jobType != "" {
			query = query.Where("job_type = ?", jobType)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type pageable interface {
	GetPage() int
	GetPageSize() int
}

func normalizeRepoPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return page, pageSize
}

func reqPage(req any) int {
	switch item := req.(type) {
	case *dbbiz.DatabaseLogArchiveStreamListRequest:
		return item.Page
	case *dbbiz.DatabaseLogArchiveListRequest:
		return item.Page
	case *dbbiz.DatabaseLogArchiveEventListRequest:
		return item.Page
	case *dbbiz.DatabaseRestorePlanListRequest:
		return item.Page
	case *dbbiz.DatabaseStorageProfileListRequest:
		return item.Page
	case *dbbiz.DatabaseSecretProfileListRequest:
		return item.Page
	case *dbbiz.DatabaseRunnerHostListRequest:
		return item.Page
	case *dbbiz.DatabaseRunnerJobListRequest:
		return item.Page
	default:
		return 1
	}
}

func reqPageSize(req any) int {
	switch item := req.(type) {
	case *dbbiz.DatabaseLogArchiveStreamListRequest:
		return item.PageSize
	case *dbbiz.DatabaseLogArchiveListRequest:
		return item.PageSize
	case *dbbiz.DatabaseLogArchiveEventListRequest:
		return item.PageSize
	case *dbbiz.DatabaseRestorePlanListRequest:
		return item.PageSize
	case *dbbiz.DatabaseStorageProfileListRequest:
		return item.PageSize
	case *dbbiz.DatabaseSecretProfileListRequest:
		return item.PageSize
	case *dbbiz.DatabaseRunnerHostListRequest:
		return item.PageSize
	case *dbbiz.DatabaseRunnerJobListRequest:
		return item.PageSize
	default:
		return 10
	}
}
