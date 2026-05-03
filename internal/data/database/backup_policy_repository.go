package database

import (
	"context"
	"strings"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type backupPolicyConfigRepo struct {
	db *gorm.DB
}

func NewBackupPolicyConfigRepo(db *gorm.DB) dbbiz.BackupPolicyConfigRepo {
	return &backupPolicyConfigRepo{db: db}
}

func (r *backupPolicyConfigRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupPolicyConfig) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupPolicyConfigRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupPolicyConfig) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupPolicyConfigRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dbbiz.DatabaseBackupPolicyConfig{}, id).Error
}

func (r *backupPolicyConfigRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseBackupPolicyConfig, error) {
	var item dbbiz.DatabaseBackupPolicyConfig
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupPolicyConfigRepo) List(ctx context.Context, req *dbbiz.DatabaseBackupPolicyListRequest) ([]*dbbiz.DatabaseBackupPolicyConfig, int64, error) {
	var (
		items []*dbbiz.DatabaseBackupPolicyConfig
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseBackupPolicyConfig{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if req.RunnerHostID > 0 {
			query = query.Where("runner_host_id = ?", req.RunnerHostID)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ?", like)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
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
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *backupPolicyConfigRepo) ListEnabled(ctx context.Context) ([]*dbbiz.DatabaseBackupPolicyConfig, error) {
	var items []*dbbiz.DatabaseBackupPolicyConfig
	if err := r.db.WithContext(ctx).
		Model(&dbbiz.DatabaseBackupPolicyConfig{}).
		Where("enabled = ?", true).
		Where("status <> ?", dbbiz.DatabaseBackupPolicyStatusDisabled).
		Where("(full_schedule <> '' OR incremental_schedule <> '')").
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type backupChainStateRepo struct {
	db *gorm.DB
}

func NewBackupChainStateRepo(db *gorm.DB) dbbiz.BackupChainStateRepo {
	return &backupChainStateRepo{db: db}
}

func (r *backupChainStateRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupChainState) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupChainStateRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupChainState) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupChainStateRepo) GetByPolicyID(ctx context.Context, policyID uint) (*dbbiz.DatabaseBackupChainState, error) {
	var item dbbiz.DatabaseBackupChainState
	if err := r.db.WithContext(ctx).Where("policy_id = ?", policyID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
