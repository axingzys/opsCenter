package database

import (
	"context"
	"strings"

	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	"gorm.io/gorm"
)

type instanceReplicaRepo struct {
	db *gorm.DB
}

func NewInstanceReplicaRepo(db *gorm.DB) dbbiz.InstanceReplicaRepo {
	return &instanceReplicaRepo{db: db}
}

func (r *instanceReplicaRepo) Create(ctx context.Context, item *dbbiz.DatabaseInstanceReplica) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *instanceReplicaRepo) Update(ctx context.Context, item *dbbiz.DatabaseInstanceReplica) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *instanceReplicaRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseInstanceReplica, error) {
	var item dbbiz.DatabaseInstanceReplica
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *instanceReplicaRepo) GetByReplicaInstanceID(ctx context.Context, replicaInstanceID uint) (*dbbiz.DatabaseInstanceReplica, error) {
	var item dbbiz.DatabaseInstanceReplica
	if err := r.db.WithContext(ctx).Where("replica_instance_id = ?", replicaInstanceID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *instanceReplicaRepo) UpsertByReplicaInstance(ctx context.Context, item *dbbiz.DatabaseInstanceReplica) (*dbbiz.DatabaseInstanceReplica, error) {
	if item == nil {
		return nil, nil
	}
	var existing dbbiz.DatabaseInstanceReplica
	err := r.db.WithContext(ctx).Where("replica_instance_id = ?", item.ReplicaInstanceID).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		if item.Status == "" {
			item.Status = dbbiz.DatabaseReplicaHealthUnknown
		}
		if item.ReplicaRole == "" {
			item.ReplicaRole = dbbiz.DatabaseReplicaRoleUnknown
		}
		if item.DiscoverySource == "" {
			item.DiscoverySource = dbbiz.DatabaseReplicaDiscoveryInferred
		}
		if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
			return nil, err
		}
		return item, nil
	}

	existing.PrimaryInstanceID = item.PrimaryInstanceID
	existing.Engine = item.Engine
	existing.ReplicaRole = item.ReplicaRole
	existing.SourceHost = item.SourceHost
	existing.SourcePort = item.SourcePort
	existing.SourceServerUUID = item.SourceServerUUID
	existing.PGSystemIdentifier = item.PGSystemIdentifier
	existing.ApplicationName = item.ApplicationName
	existing.ConfiguredDelaySeconds = item.ConfiguredDelaySeconds
	existing.DiscoverySource = item.DiscoverySource
	existing.Status = item.Status
	existing.LastCheckID = item.LastCheckID
	existing.LastCheckedAt = item.LastCheckedAt
	existing.LastError = item.LastError
	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (r *instanceReplicaRepo) List(ctx context.Context, req *dbbiz.DatabaseInstanceReplicaListRequest) ([]*dbbiz.DatabaseInstanceReplica, int64, error) {
	var (
		items []*dbbiz.DatabaseInstanceReplica
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseInstanceReplica{})
	if req != nil {
		if req.RestrictToAllowed {
			if len(req.AllowedInstanceIDs) == 0 {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("primary_instance_id IN ? OR replica_instance_id IN ?", req.AllowedInstanceIDs, req.AllowedInstanceIDs)
			}
		}
		if req.InstanceID > 0 {
			query = query.Where("primary_instance_id = ? OR replica_instance_id = ?", req.InstanceID, req.InstanceID)
		}
		if req.PrimaryInstanceID > 0 {
			query = query.Where("primary_instance_id = ?", req.PrimaryInstanceID)
		}
		if req.ReplicaInstanceID > 0 {
			query = query.Where("replica_instance_id = ?", req.ReplicaInstanceID)
		}
		if engine := strings.TrimSpace(req.Engine); engine != "" {
			query = query.Where("engine = ?", engine)
		}
		if role := strings.TrimSpace(req.ReplicaRole); role != "" {
			query = query.Where("replica_role = ?", role)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("last_checked_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type replicationCheckRepo struct {
	db *gorm.DB
}

func NewReplicationCheckRepo(db *gorm.DB) dbbiz.ReplicationCheckRepo {
	return &replicationCheckRepo{db: db}
}

func (r *replicationCheckRepo) Create(ctx context.Context, item *dbbiz.DatabaseReplicationCheck) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *replicationCheckRepo) Update(ctx context.Context, item *dbbiz.DatabaseReplicationCheck) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *replicationCheckRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseReplicationCheck, error) {
	var item dbbiz.DatabaseReplicationCheck
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *replicationCheckRepo) LatestByInstanceID(ctx context.Context, instanceID uint) (*dbbiz.DatabaseReplicationCheck, error) {
	var item dbbiz.DatabaseReplicationCheck
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Order("checked_at DESC, id DESC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *replicationCheckRepo) List(ctx context.Context, req *dbbiz.DatabaseReplicationCheckListRequest) ([]*dbbiz.DatabaseReplicationCheck, int64, error) {
	var (
		items []*dbbiz.DatabaseReplicationCheck
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseReplicationCheck{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if req.ReplicaID > 0 {
			query = query.Where("replica_id = ?", req.ReplicaID)
		}
		if engine := strings.TrimSpace(req.Engine); engine != "" {
			query = query.Where("engine = ?", engine)
		}
		if role := strings.TrimSpace(req.RoleDetected); role != "" {
			query = query.Where("role_detected = ?", role)
		}
		if status := strings.TrimSpace(req.HealthStatus); status != "" {
			query = query.Where("health_status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("checked_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type replicaIncidentGuideRepo struct {
	db *gorm.DB
}

func NewReplicaIncidentGuideRepo(db *gorm.DB) dbbiz.ReplicaIncidentGuideRepo {
	return &replicaIncidentGuideRepo{db: db}
}

func (r *replicaIncidentGuideRepo) Create(ctx context.Context, item *dbbiz.DatabaseReplicaIncidentGuide) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *replicaIncidentGuideRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dbbiz.DatabaseReplicaIncidentGuide{}, id).Error
}

func (r *replicaIncidentGuideRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseReplicaIncidentGuide, error) {
	var item dbbiz.DatabaseReplicaIncidentGuide
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *replicaIncidentGuideRepo) List(ctx context.Context, req *dbbiz.DatabaseReplicaIncidentGuideListRequest) ([]*dbbiz.DatabaseReplicaIncidentGuide, int64, error) {
	var (
		items []*dbbiz.DatabaseReplicaIncidentGuide
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseReplicaIncidentGuide{})
	if req != nil {
		query = applyAllowedInstanceFilter(query, "instance_id", req.RestrictToAllowed, req.AllowedInstanceIDs)
		if req.InstanceID > 0 {
			query = query.Where("instance_id = ?", req.InstanceID)
		}
		if typ := strings.TrimSpace(req.IncidentType); typ != "" {
			query = query.Where("incident_type = ?", typ)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if value := strings.TrimSpace(req.CanIntercept); value != "" {
			switch strings.ToLower(value) {
			case "true", "1", "yes":
				query = query.Where("can_intercept = ?", true)
			case "false", "0", "no":
				query = query.Where("can_intercept = ?", false)
			}
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type replicaActionRepo struct {
	db *gorm.DB
}

func NewReplicaActionRepo(db *gorm.DB) dbbiz.ReplicaActionRepo {
	return &replicaActionRepo{db: db}
}

func (r *replicaActionRepo) Create(ctx context.Context, item *dbbiz.DatabaseReplicaAction) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *replicaActionRepo) Update(ctx context.Context, item *dbbiz.DatabaseReplicaAction) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *replicaActionRepo) GetByID(ctx context.Context, id uint) (*dbbiz.DatabaseReplicaAction, error) {
	var item dbbiz.DatabaseReplicaAction
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *replicaActionRepo) List(ctx context.Context, req *dbbiz.DatabaseReplicaActionListRequest) ([]*dbbiz.DatabaseReplicaAction, int64, error) {
	var (
		items []*dbbiz.DatabaseReplicaAction
		total int64
	)
	query := r.db.WithContext(ctx).Model(&dbbiz.DatabaseReplicaAction{})
	if req != nil {
		if req.RestrictToAllowed {
			if len(req.AllowedInstanceIDs) == 0 {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("primary_instance_id IN ? OR replica_instance_id IN ?", req.AllowedInstanceIDs, req.AllowedInstanceIDs)
			}
		}
		if req.ReplicaID > 0 {
			query = query.Where("replica_id = ?", req.ReplicaID)
		}
		if req.InstanceID > 0 {
			query = query.Where("primary_instance_id = ? OR replica_instance_id = ?", req.InstanceID, req.InstanceID)
		}
		if action := strings.TrimSpace(req.Action); action != "" {
			query = query.Where("action = ?", action)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeRepoPage(reqPage(req), reqPageSize(req))
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
