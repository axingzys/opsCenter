package database

import (
	"context"
	"strings"
	"time"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type capacitySnapshotRepo struct {
	db *gorm.DB
}

func NewCapacitySnapshotRepo(db *gorm.DB) dbbiz.CapacitySnapshotRepo {
	return &capacitySnapshotRepo{db: db}
}

func (r *capacitySnapshotRepo) CreateBatch(ctx context.Context, items []*dbbiz.DatabaseCapacitySnapshot) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 500).Error
}

func (r *capacitySnapshotRepo) ListInstanceTrend(ctx context.Context, instanceID uint, since time.Time) ([]*dbbiz.DatabaseCapacitySnapshot, error) {
	var items []*dbbiz.DatabaseCapacitySnapshot
	query := r.db.WithContext(ctx).
		Where("instance_id = ? AND object_type = ?", instanceID, dbbiz.DatabaseCapacityObjectInstance).
		Order("collected_at ASC, id ASC")
	if !since.IsZero() {
		query = query.Where("collected_at >= ?", since)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *capacitySnapshotRepo) LatestInstance(ctx context.Context, instanceID uint) (*dbbiz.DatabaseCapacitySnapshot, error) {
	var item dbbiz.DatabaseCapacitySnapshot
	err := r.db.WithContext(ctx).
		Where("instance_id = ? AND object_type = ?", instanceID, dbbiz.DatabaseCapacityObjectInstance).
		Order("collected_at DESC, id DESC").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *capacitySnapshotRepo) LatestTopObjects(ctx context.Context, instanceID uint, objectType string, limit int) ([]*dbbiz.DatabaseCapacitySnapshot, error) {
	var latest dbbiz.DatabaseCapacitySnapshot
	objectType = strings.TrimSpace(objectType)
	if objectType == "" {
		objectType = dbbiz.DatabaseCapacityObjectTable
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	if err := r.db.WithContext(ctx).
		Where("instance_id = ? AND object_type = ?", instanceID, objectType).
		Order("collected_at DESC, id DESC").
		First(&latest).Error; err != nil {
		return nil, err
	}

	var items []*dbbiz.DatabaseCapacitySnapshot
	if err := r.db.WithContext(ctx).
		Where("instance_id = ? AND object_type = ? AND collected_at = ?", instanceID, objectType, latest.CollectedAt).
		Order("total_size_bytes DESC, row_count DESC, id ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type inspectionReportRepo struct {
	db *gorm.DB
}

func NewInspectionReportRepo(db *gorm.DB) dbbiz.InspectionReportRepo {
	return &inspectionReportRepo{db: db}
}

func (r *inspectionReportRepo) Create(ctx context.Context, item *dbbiz.DatabaseInspectionReport) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *inspectionReportRepo) Update(ctx context.Context, item *dbbiz.DatabaseInspectionReport) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *inspectionReportRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseInspectionReport, error) {
	var item dbbiz.DatabaseInspectionReport
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *inspectionReportRepo) List(ctx context.Context, req *dbbiz.DatabaseInspectionReportListRequest) ([]*dbbiz.DatabaseInspectionReport, int64, error) {
	var (
		items []*dbbiz.DatabaseInspectionReport
		total int64
	)

	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseInspectionReport{})
	if req != nil {
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if riskLevel := strings.TrimSpace(req.RiskLevel); riskLevel != "" {
			query = query.Where("risk_level = ?", riskLevel)
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
