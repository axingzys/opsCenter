package database

import (
	"context"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type backupTaskRepo struct {
	db *gorm.DB
}

func NewBackupTaskRepo(db *gorm.DB) dbbiz.BackupTaskRepo {
	return &backupTaskRepo{db: db}
}

func (r *backupTaskRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupTask) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupTaskRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupTask) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupTaskRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dbbiz.DatabaseBackupTask{}, id).Error
}

func (r *backupTaskRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseBackupTask, error) {
	var item dbbiz.DatabaseBackupTask
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupTaskRepo) List(ctx context.Context, req *dbbiz.DatabaseBackupTaskListRequest) ([]*dbbiz.DatabaseBackupTask, int64, error) {
	var (
		items []*dbbiz.DatabaseBackupTask
		total int64
	)

	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseBackupTask{})
	if req != nil {
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ?", like)
		}
		switch strings.ToLower(strings.TrimSpace(req.Enabled)) {
		case "true", "1", "enabled":
			query = query.Where("enabled = ?", true)
		case "false", "0", "disabled":
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		page = req.Page
		pageSize = req.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *backupTaskRepo) ListAll(ctx context.Context) ([]*dbbiz.DatabaseBackupTask, error) {
	var items []*dbbiz.DatabaseBackupTask
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *backupTaskRepo) ListEnabled(ctx context.Context) ([]*dbbiz.DatabaseBackupTask, error) {
	var items []*dbbiz.DatabaseBackupTask
	if err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("schedule <> ''").
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type backupRecordRepo struct {
	db *gorm.DB
}

func NewBackupRecordRepo(db *gorm.DB) dbbiz.BackupRecordRepo {
	return &backupRecordRepo{db: db}
}

func (r *backupRecordRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupRecordRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupRecordRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseBackupRecord, error) {
	var item dbbiz.DatabaseBackupRecord
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupRecordRepo) List(ctx context.Context, req *dbbiz.DatabaseBackupRecordListRequest) ([]*dbbiz.DatabaseBackupRecord, int64, error) {
	var (
		items []*dbbiz.DatabaseBackupRecord
		total int64
	)

	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseBackupRecord{})
	if req != nil {
		if req.TaskID > 0 {
			query = query.Where("task_id = ?", req.TaskID)
		}
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if triggerType := strings.TrimSpace(req.TriggerType); triggerType != "" {
			query = query.Where("trigger_type = ?", triggerType)
		}
		if dateFrom := strings.TrimSpace(req.DateFrom); dateFrom != "" {
			query = query.Where("created_at >= ?", dateFrom)
		}
		if dateTo := strings.TrimSpace(req.DateTo); dateTo != "" {
			query = query.Where("created_at <= ?", dateTo)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		page = req.Page
		pageSize = req.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *backupRecordRepo) ListExpiredSuccessByTask(ctx context.Context, taskID uint, before time.Time) ([]*dbbiz.DatabaseBackupRecord, error) {
	var items []*dbbiz.DatabaseBackupRecord
	query := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseBackupRecord{}).
		Where("task_id = ?", taskID).
		Where("status = ?", dbbiz.DatabaseBackupStatusSuccess).
		Where("file_path <> ''").
		Where("(finished_at IS NOT NULL AND finished_at < ?) OR (finished_at IS NULL AND created_at < ?)", before, before).
		Order("id ASC")
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type restoreJobRepo struct {
	db *gorm.DB
}

func NewRestoreJobRepo(db *gorm.DB) dbbiz.RestoreJobRepo {
	return &restoreJobRepo{db: db}
}

func (r *restoreJobRepo) Create(ctx context.Context, item *dbbiz.DatabaseRestoreJob) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *restoreJobRepo) Update(ctx context.Context, item *dbbiz.DatabaseRestoreJob) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *restoreJobRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseRestoreJob, error) {
	var item dbbiz.DatabaseRestoreJob
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *restoreJobRepo) List(ctx context.Context, req *dbbiz.DatabaseRestoreJobListRequest) ([]*dbbiz.DatabaseRestoreJob, int64, error) {
	var (
		items []*dbbiz.DatabaseRestoreJob
		total int64
	)

	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseRestoreJob{})
	if req != nil {
		if req.BackupRecordID > 0 {
			query = query.Where("backup_record_id = ?", req.BackupRecordID)
		}
		if req.SourceInstanceID > 0 {
			query = query.Where("source_instance_id = ?", req.SourceInstanceID)
		}
		if req.TargetInstanceID > 0 {
			query = query.Where("target_instance_id = ?", req.TargetInstanceID)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		page = req.Page
		pageSize = req.PageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
