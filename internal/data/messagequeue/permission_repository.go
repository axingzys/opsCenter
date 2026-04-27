package messagequeue

import (
	"context"
	"fmt"
	"strings"
	"time"

	mqbiz "github.com/ydcloud-dy/opshub/internal/biz/messagequeue"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type permissionRepo struct {
	db *gorm.DB
}

type permissionRow struct {
	ID           uint
	RoleID       uint
	RoleName     string
	RoleCode     string
	InstanceID   uint
	InstanceName string
	Permissions  uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewPermissionRepo(db *gorm.DB) mqbiz.PermissionRepo {
	return &permissionRepo{db: db}
}

func (r *permissionRepo) HasAnyRules(ctx context.Context) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&mqbiz.MQInstancePermission{}).Count(&count).Error
	return count > 0, err
}

func (r *permissionRepo) IsAdmin(ctx context.Context, userID uint) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("sys_user_role AS ur").
		Joins("JOIN sys_role AS r ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.code = ? AND r.status = 1 AND r.deleted_at IS NULL", userID, "admin").
		Count(&count).Error
	return count > 0, err
}

func (r *permissionRepo) GetUserInstancePermissions(ctx context.Context, userID, instanceID uint) (uint, error) {
	if userID == 0 || instanceID == 0 {
		return 0, nil
	}
	admin, err := r.IsAdmin(ctx, userID)
	if err != nil {
		return 0, err
	}
	if admin {
		return mqbiz.PermissionAll, nil
	}
	var permissions []uint
	err = r.db.WithContext(ctx).
		Table("mq_instance_permissions AS p").
		Joins("JOIN sys_user_role AS ur ON p.role_id = ur.role_id").
		Joins("JOIN sys_role AS r ON ur.role_id = r.id").
		Where("ur.user_id = ? AND p.instance_id = ? AND p.deleted_at IS NULL AND r.status = 1 AND r.deleted_at IS NULL", userID, instanceID).
		Pluck("p.permissions", &permissions).Error
	if err != nil {
		return 0, err
	}
	var result uint
	for _, permission := range permissions {
		result |= permission
	}
	return result, nil
}

func (r *permissionRepo) GetUserAccessibleInstanceIDs(ctx context.Context, userID uint, required uint) ([]uint, error) {
	if userID == 0 {
		return nil, nil
	}
	admin, err := r.IsAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	if admin {
		var ids []uint
		err := r.db.WithContext(ctx).Model(&mqbiz.MQInstance{}).Pluck("id", &ids).Error
		return ids, err
	}
	query := r.db.WithContext(ctx).
		Table("mq_instance_permissions AS p").
		Joins("JOIN sys_user_role AS ur ON p.role_id = ur.role_id").
		Joins("JOIN sys_role AS r ON ur.role_id = r.id").
		Where("ur.user_id = ? AND p.deleted_at IS NULL AND r.status = 1 AND r.deleted_at IS NULL", userID)
	if required > 0 {
		query = query.Where("(p.permissions & ?) > 0", required)
	} else {
		query = query.Where("p.permissions > 0")
	}
	var ids []uint
	err = query.Distinct("p.instance_id").Pluck("p.instance_id", &ids).Error
	return ids, err
}

func (r *permissionRepo) List(ctx context.Context, req *mqbiz.InstancePermissionListRequest) ([]*mqbiz.InstancePermissionVO, int64, error) {
	query := r.permissionVOQuery(ctx)
	if req != nil {
		if req.RoleID > 0 {
			query = query.Where("p.role_id = ?", req.RoleID)
		}
		if req.InstanceID > 0 {
			query = query.Where("p.instance_id = ?", req.InstanceID)
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			like := "%" + keyword + "%"
			query = query.Where("r.name LIKE ? OR r.code LIKE ? OR i.name LIKE ?", like, like, like)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := pageParams(req)
	var rows []permissionRow
	if err := query.Order("p.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	list := make([]*mqbiz.InstancePermissionVO, 0, len(rows))
	for _, item := range rows {
		list = append(list, permissionRowToVO(item))
	}
	return list, total, nil
}

func (r *permissionRepo) GetByID(ctx context.Context, id uint) (*mqbiz.InstancePermissionVO, error) {
	if id == 0 {
		return nil, nil
	}
	return r.scanPermissionVO(r.permissionVOQuery(ctx).Where("p.id = ?", id))
}

func (r *permissionRepo) GetByRoleInstance(ctx context.Context, roleID, instanceID uint) (*mqbiz.InstancePermissionVO, error) {
	if roleID == 0 || instanceID == 0 {
		return nil, nil
	}
	return r.scanPermissionVO(r.permissionVOQuery(ctx).Where("p.role_id = ? AND p.instance_id = ?", roleID, instanceID))
}

func (r *permissionRepo) ValidateTarget(ctx context.Context, roleID, instanceID uint) error {
	if roleID == 0 {
		return fmt.Errorf("角色不能为空")
	}
	if instanceID == 0 {
		return fmt.Errorf("MQ实例不能为空")
	}
	var roleCount int64
	if err := r.db.WithContext(ctx).Table("sys_role").
		Where("id = ? AND status = 1 AND deleted_at IS NULL", roleID).
		Count(&roleCount).Error; err != nil {
		return fmt.Errorf("校验角色失败: %w", err)
	}
	if roleCount == 0 {
		return fmt.Errorf("角色不存在或已禁用")
	}
	var instanceCount int64
	if err := r.db.WithContext(ctx).Table("mq_instances").
		Where("id = ? AND deleted_at IS NULL", instanceID).
		Count(&instanceCount).Error; err != nil {
		return fmt.Errorf("校验MQ实例失败: %w", err)
	}
	if instanceCount == 0 {
		return fmt.Errorf("MQ实例不存在")
	}
	return nil
}

func (r *permissionRepo) Upsert(ctx context.Context, item *mqbiz.MQInstancePermission) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "role_id"}, {Name: "instance_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"permissions": item.Permissions,
				"updated_at":  now,
				"deleted_at":  nil,
			}),
		}).
		Create(item).Error
}

func (r *permissionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&mqbiz.MQInstancePermission{}, id).Error
}

func (r *permissionRepo) permissionVOQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("mq_instance_permissions AS p").
		Select(`p.id, p.role_id, r.name AS role_name, r.code AS role_code,
			p.instance_id, i.name AS instance_name, p.permissions, p.created_at, p.updated_at`).
		Joins("LEFT JOIN sys_role AS r ON p.role_id = r.id").
		Joins("LEFT JOIN mq_instances AS i ON p.instance_id = i.id AND i.deleted_at IS NULL").
		Where("p.deleted_at IS NULL")
}

func (r *permissionRepo) scanPermissionVO(query *gorm.DB) (*mqbiz.InstancePermissionVO, error) {
	var row permissionRow
	if err := query.Order("p.id DESC").Limit(1).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return permissionRowToVO(row), nil
}

func permissionRowToVO(item permissionRow) *mqbiz.InstancePermissionVO {
	return &mqbiz.InstancePermissionVO{
		ID:           item.ID,
		RoleID:       item.RoleID,
		RoleName:     item.RoleName,
		RoleCode:     item.RoleCode,
		InstanceID:   item.InstanceID,
		InstanceName: item.InstanceName,
		Permissions:  item.Permissions,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
