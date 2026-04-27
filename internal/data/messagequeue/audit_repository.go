package messagequeue

import (
	"context"
	"errors"
	"strings"
	"time"

	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	"gorm.io/gorm"
)

type operationAuditRepo struct {
	db *gorm.DB
}

func NewOperationAuditRepo(db *gorm.DB) mqbiz.OperationAuditRepo {
	return &operationAuditRepo{db: db}
}

func (r *operationAuditRepo) Create(ctx context.Context, item *mqbiz.MQOperationAudit) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *operationAuditRepo) Update(ctx context.Context, item *mqbiz.MQOperationAudit) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *operationAuditRepo) GetByIdempotencyKey(ctx context.Context, instanceID uint, idempotencyKey string) (*mqbiz.MQOperationAudit, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, nil
	}
	var item mqbiz.MQOperationAudit
	err := r.db.WithContext(ctx).
		Where("instance_id = ? AND idempotency_key = ?", instanceID, idempotencyKey).
		Order("id DESC").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *operationAuditRepo) List(ctx context.Context, req *mqbiz.AuditListRequest) ([]*mqbiz.MQOperationAudit, int64, error) {
	var (
		items []*mqbiz.MQOperationAudit
		total int64
	)
	query := r.db.WithContext(ctx).Model(&mqbiz.MQOperationAudit{})
	query = applyOperationAuditFilter(query, req)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type messageAuditRepo struct {
	db *gorm.DB
}

func NewMessageAuditRepo(db *gorm.DB) mqbiz.MessageAuditRepo {
	return &messageAuditRepo{db: db}
}

func (r *messageAuditRepo) Create(ctx context.Context, item *mqbiz.MQMessageAudit) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *messageAuditRepo) List(ctx context.Context, req *mqbiz.AuditListRequest) ([]*mqbiz.MQMessageAudit, int64, error) {
	var (
		items []*mqbiz.MQMessageAudit
		total int64
	)
	query := r.db.WithContext(ctx).Model(&mqbiz.MQMessageAudit{})
	query = applyMessageAuditFilter(query, req)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func applyOperationAuditFilter(query *gorm.DB, req *mqbiz.AuditListRequest) *gorm.DB {
	if req == nil {
		return query
	}
	query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedIDs)
	if req.InstanceID > 0 {
		query = query.Where("instance_id = ?", req.InstanceID)
	}
	if req.MQType != "" {
		query = query.Where("mq_type = ?", req.MQType)
	}
	if req.Action != "" {
		query = query.Where("action = ?", req.Action)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.RiskLevel != "" {
		query = query.Where("risk_level = ?", req.RiskLevel)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("instance_name LIKE ? OR resource_name LIKE ? OR operator_name LIKE ? OR message LIKE ?", like, like, like, like)
	}
	if start, ok := parseTime(req.StartTime); ok {
		query = query.Where("created_at >= ?", start)
	}
	if end, ok := parseTime(req.EndTime); ok {
		query = query.Where("created_at <= ?", end)
	}
	return query
}

func applyMessageAuditFilter(query *gorm.DB, req *mqbiz.AuditListRequest) *gorm.DB {
	if req == nil {
		return query
	}
	query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedIDs)
	if req.InstanceID > 0 {
		query = query.Where("instance_id = ?", req.InstanceID)
	}
	if req.MQType != "" {
		query = query.Where("mq_type = ?", req.MQType)
	}
	if req.Action != "" {
		query = query.Where("action = ?", req.Action)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("resource_name LIKE ? OR operator_name LIKE ? OR message LIKE ?", like, like, like)
	}
	if start, ok := parseTime(req.StartTime); ok {
		query = query.Where("created_at >= ?", start)
	}
	if end, ok := parseTime(req.EndTime); ok {
		query = query.Where("created_at <= ?", end)
	}
	return query
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		t, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
