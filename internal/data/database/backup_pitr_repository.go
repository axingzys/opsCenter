package database

import (
	"context"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
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
	case *dbbiz.DatabaseRestorePlanListRequest:
		return item.Page
	case *dbbiz.DatabaseStorageProfileListRequest:
		return item.Page
	case *dbbiz.DatabaseSecretProfileListRequest:
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
	case *dbbiz.DatabaseRestorePlanListRequest:
		return item.PageSize
	case *dbbiz.DatabaseStorageProfileListRequest:
		return item.PageSize
	case *dbbiz.DatabaseSecretProfileListRequest:
		return item.PageSize
	case *dbbiz.DatabaseRunnerJobListRequest:
		return item.PageSize
	default:
		return 10
	}
}
