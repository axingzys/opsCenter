package asset

import (
	"context"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/asset"
	"gorm.io/gorm"
)

type assetAgentRepo struct {
	db *gorm.DB
}

func NewAssetAgentRepo(db *gorm.DB) asset.AssetAgentRepo {
	return &assetAgentRepo{db: db}
}

func (r *assetAgentRepo) Create(ctx context.Context, agentModel *asset.AssetAgent) error {
	return r.db.WithContext(ctx).Create(agentModel).Error
}

func (r *assetAgentRepo) Update(ctx context.Context, agentModel *asset.AssetAgent) error {
	return r.db.WithContext(ctx).Save(agentModel).Error
}

func (r *assetAgentRepo) GetByHostID(ctx context.Context, hostID uint) (*asset.AssetAgent, error) {
	var item asset.AssetAgent
	if err := r.db.WithContext(ctx).Where("host_id = ?", hostID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *assetAgentRepo) GetByAgentID(ctx context.Context, agentID string) (*asset.AssetAgent, error) {
	var item asset.AssetAgent
	if err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *assetAgentRepo) List(ctx context.Context, page, pageSize int, keyword string, accessibleHostIDs []uint, status string) ([]*asset.AssetAgent, int64, error) {
	var (
		list  []*asset.AssetAgent
		total int64
	)

	query := r.db.WithContext(ctx).Model(&asset.AssetAgent{})
	if keyword != "" {
		query = query.Joins("JOIN hosts ON hosts.id = asset_agents.host_id AND hosts.deleted_at IS NULL").
			Where(
				"hosts.name LIKE ? OR hosts.ip LIKE ? OR hosts.primary_private_ip LIKE ? OR hosts.primary_public_ip LIKE ? OR asset_agents.agent_id LIKE ?",
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
			)
	}
	if accessibleHostIDs != nil {
		if len(accessibleHostIDs) == 0 {
			return []*asset.AssetAgent{}, 0, nil
		}
		query = query.Where("host_id IN ?", accessibleHostIDs)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *assetAgentRepo) DeleteByHostID(ctx context.Context, hostID uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("host_id = ?", hostID).Delete(&asset.AssetAgent{}).Error
}

type hostInventoryRepo struct {
	db *gorm.DB
}

func NewAssetHostInventoryRepo(db *gorm.DB) asset.AssetHostInventoryRepo {
	return &hostInventoryRepo{db: db}
}

func (r *hostInventoryRepo) Upsert(ctx context.Context, inventory *asset.AssetHostInventory) error {
	var existing asset.AssetHostInventory
	err := r.db.WithContext(ctx).Where("host_id = ?", inventory.HostID).First(&existing).Error
	if err == nil {
		inventory.ID = existing.ID
		inventory.CreatedAt = existing.CreatedAt
		inventory.DeletedAt = existing.DeletedAt
		return r.db.WithContext(ctx).Save(inventory).Error
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.WithContext(ctx).Create(inventory).Error
}

func (r *hostInventoryRepo) GetByHostID(ctx context.Context, hostID uint) (*asset.AssetHostInventory, error) {
	var item asset.AssetHostInventory
	if err := r.db.WithContext(ctx).Where("host_id = ?", hostID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *hostInventoryRepo) DeleteByHostID(ctx context.Context, hostID uint) error {
	return r.db.WithContext(ctx).Unscoped().Where("host_id = ?", hostID).Delete(&asset.AssetHostInventory{}).Error
}

type hostPublicIPHistoryRepo struct {
	db *gorm.DB
}

func NewAssetHostPublicIPHistoryRepo(db *gorm.DB) asset.AssetHostPublicIPHistoryRepo {
	return &hostPublicIPHistoryRepo{db: db}
}

func (r *hostPublicIPHistoryRepo) Observe(ctx context.Context, hostID uint, ip, source string, observedAt time.Time) error {
	ip = strings.TrimSpace(ip)
	if hostID == 0 || ip == "" {
		return nil
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "agent_detected"
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current asset.AssetHostPublicIPHistory
		err := tx.Where("host_id = ? AND is_current = ?", hostID, true).Order("id DESC").First(&current).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if err == nil && strings.TrimSpace(current.IP) == ip {
			current.LastSeenAt = observedAt
			current.SeenCount++
			current.Source = source
			current.IsCurrent = true
			return tx.Save(&current).Error
		}

		if err == nil {
			if err := tx.Model(&asset.AssetHostPublicIPHistory{}).
				Where("host_id = ? AND is_current = ?", hostID, true).
				Updates(map[string]any{
					"is_current":   false,
					"last_seen_at": observedAt,
				}).Error; err != nil {
				return err
			}
		}

		item := &asset.AssetHostPublicIPHistory{
			HostID:      hostID,
			IP:          ip,
			Source:      source,
			FirstSeenAt: observedAt,
			LastSeenAt:  observedAt,
			SeenCount:   1,
			IsCurrent:   true,
		}
		return tx.Create(item).Error
	})
}

func (r *hostPublicIPHistoryRepo) ListByHostID(ctx context.Context, hostID uint, limit int) ([]*asset.AssetHostPublicIPHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	var list []*asset.AssetHostPublicIPHistory
	if err := r.db.WithContext(ctx).
		Where("host_id = ?", hostID).
		Order("is_current DESC, last_seen_at DESC, id DESC").
		Limit(limit).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

type assetAgentJobRepo struct {
	db *gorm.DB
}

func NewAssetAgentJobRepo(db *gorm.DB) asset.AssetAgentJobRepo {
	return &assetAgentJobRepo{db: db}
}

func (r *assetAgentJobRepo) Create(ctx context.Context, job *asset.AssetAgentJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *assetAgentJobRepo) Update(ctx context.Context, job *asset.AssetAgentJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *assetAgentJobRepo) GetByID(ctx context.Context, id uint) (*asset.AssetAgentJob, error) {
	var item asset.AssetAgentJob
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *assetAgentJobRepo) GetLatestByHostID(ctx context.Context, hostID uint) (*asset.AssetAgentJob, error) {
	var item asset.AssetAgentJob
	if err := r.db.WithContext(ctx).Where("host_id = ?", hostID).Order("id DESC").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
