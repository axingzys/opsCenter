package asset

import (
	"context"

	"github.com/ydcloud-dy/opshub/internal/biz/asset"
	"gorm.io/gorm"
)

type desktopSessionRepo struct {
	db *gorm.DB
}

// NewDesktopSessionRepo 创建桌面会话仓库
func NewDesktopSessionRepo(db *gorm.DB) asset.DesktopSessionRepo {
	return &desktopSessionRepo{db: db}
}

func (r *desktopSessionRepo) Create(ctx context.Context, session *asset.DesktopSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *desktopSessionRepo) Update(ctx context.Context, session *asset.DesktopSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *desktopSessionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&asset.DesktopSession{}, id).Error
}

func (r *desktopSessionRepo) GetByID(ctx context.Context, id uint) (*asset.DesktopSession, error) {
	var session asset.DesktopSession
	if err := r.db.WithContext(ctx).First(&session, id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *desktopSessionRepo) GetBySessionUUID(ctx context.Context, sessionUUID string) (*asset.DesktopSession, error) {
	var session asset.DesktopSession
	if err := r.db.WithContext(ctx).Where("session_uuid = ?", sessionUUID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *desktopSessionRepo) List(ctx context.Context, page, pageSize int, keyword, status string, userID uint) ([]*asset.DesktopSession, int64, error) {
	var (
		list  []*asset.DesktopSession
		total int64
	)

	query := r.db.WithContext(ctx).Model(&asset.DesktopSession{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"host_name LIKE ? OR host_ip LIKE ? OR username LIKE ? OR session_uuid LIKE ?",
			like, like, like, like,
		)
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
