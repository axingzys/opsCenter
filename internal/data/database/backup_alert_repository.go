package database

import (
	"context"
	"strings"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type backupAlertRuleRepo struct {
	db *gorm.DB
}

func NewBackupAlertRuleRepo(db *gorm.DB) dbbiz.BackupAlertRuleRepo {
	return &backupAlertRuleRepo{db: db}
}

func (r *backupAlertRuleRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupAlertRule) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupAlertRuleRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupAlertRule) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupAlertRuleRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dbbiz.DatabaseBackupAlertRule{}, id).Error
}

func (r *backupAlertRuleRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseBackupAlertRule, error) {
	var item dbbiz.DatabaseBackupAlertRule
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupAlertRuleRepo) List(ctx context.Context, req *dbbiz.DatabaseBackupAlertRuleListRequest) ([]*dbbiz.DatabaseBackupAlertRule, int64, error) {
	var (
		items []*dbbiz.DatabaseBackupAlertRule
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseBackupAlertRule{})
	if req != nil {
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("name LIKE ? OR description LIKE ? OR business_system LIKE ? OR owner LIKE ?", like, like, like, like)
		}
		switch strings.ToLower(strings.TrimSpace(req.Enabled)) {
		case "true", "1", "enabled":
			query = query.Where("enabled = ?", true)
		case "false", "0", "disabled":
			query = query.Where("enabled = ?", false)
		}
		if scopeType := strings.TrimSpace(req.ScopeType); scopeType != "" {
			query = query.Where("scope_type = ?", scopeType)
		}
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if engine := strings.TrimSpace(req.Engine); engine != "" {
			query = query.Where("engine = ?", engine)
		}
		if issueType := strings.TrimSpace(req.IssueType); issueType != "" {
			query = query.Where("issue_types_json LIKE ?", "%"+issueType+"%")
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeBackupAlertPage(reqRulePage(req), reqRulePageSize(req))
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *backupAlertRuleRepo) ListEnabled(ctx context.Context) ([]*dbbiz.DatabaseBackupAlertRule, error) {
	var items []*dbbiz.DatabaseBackupAlertRule
	if err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type backupAlertStateRepo struct {
	db *gorm.DB
}

func NewBackupAlertStateRepo(db *gorm.DB) dbbiz.BackupAlertStateRepo {
	return &backupAlertStateRepo{db: db}
}

func (r *backupAlertStateRepo) Create(ctx context.Context, item *dbbiz.DatabaseBackupAlertState) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *backupAlertStateRepo) Update(ctx context.Context, item *dbbiz.DatabaseBackupAlertState) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *backupAlertStateRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseBackupAlertState, error) {
	var item dbbiz.DatabaseBackupAlertState
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupAlertStateRepo) GetByFingerprint(ctx context.Context, fingerprint string) (*dbbiz.DatabaseBackupAlertState, error) {
	var item dbbiz.DatabaseBackupAlertState
	if err := r.db.WithContext(ctx).Where("fingerprint = ?", strings.TrimSpace(fingerprint)).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *backupAlertStateRepo) List(ctx context.Context, req *dbbiz.DatabaseBackupAlertStateListRequest) ([]*dbbiz.DatabaseBackupAlertState, int64, error) {
	var (
		items []*dbbiz.DatabaseBackupAlertState
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseBackupAlertState{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.RuleID > 0 {
			query = query.Where("rule_id = ?", req.RuleID)
		}
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if severity := strings.TrimSpace(req.Severity); severity != "" {
			query = query.Where("severity = ?", severity)
		}
		if issueType := strings.TrimSpace(req.IssueType); issueType != "" {
			query = query.Where("issue_type = ?", issueType)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("resource_name LIKE ? OR resource_target LIKE ? OR message LIKE ? OR suggestion LIKE ?", like, like, like, like)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeBackupAlertPage(reqStatePage(req), reqStatePageSize(req))
	if err := query.Order("last_fired_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *backupAlertStateRepo) ListFiringByRuleID(ctx context.Context, ruleID uint) ([]*dbbiz.DatabaseBackupAlertState, error) {
	var items []*dbbiz.DatabaseBackupAlertState
	if err := r.db.WithContext(ctx).
		Where("rule_id = ? AND status = ?", ruleID, dbbiz.DatabaseBackupAlertStateFiring).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func normalizeBackupAlertPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func reqRulePage(req *dbbiz.DatabaseBackupAlertRuleListRequest) int {
	if req == nil {
		return 1
	}
	return req.Page
}

func reqRulePageSize(req *dbbiz.DatabaseBackupAlertRuleListRequest) int {
	if req == nil {
		return 10
	}
	return req.PageSize
}

func reqStatePage(req *dbbiz.DatabaseBackupAlertStateListRequest) int {
	if req == nil {
		return 1
	}
	return req.Page
}

func reqStatePageSize(req *dbbiz.DatabaseBackupAlertStateListRequest) int {
	if req == nil {
		return 10
	}
	return req.PageSize
}
