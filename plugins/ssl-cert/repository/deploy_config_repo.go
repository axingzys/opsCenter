package repository

import (
	"context"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"gorm.io/gorm"
)

// DeployConfigRepository 部署配置仓库
type DeployConfigRepository struct {
	db *gorm.DB
}

// NewDeployConfigRepository 创建部署配置仓库
func NewDeployConfigRepository(db *gorm.DB) *DeployConfigRepository {
	return &DeployConfigRepository{db: db}
}

// Create 创建部署配置
func (r *DeployConfigRepository) Create(ctx context.Context, config *model.DeployConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// Update 更新部署配置
func (r *DeployConfigRepository) Update(ctx context.Context, config *model.DeployConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// Delete 删除部署配置
func (r *DeployConfigRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.DeployConfig{}, id).Error
}

// GetByID 根据ID获取部署配置
func (r *DeployConfigRepository) GetByID(ctx context.Context, id uint) (*model.DeployConfig, error) {
	var config model.DeployConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetByIDWithCert 根据ID获取部署配置(含证书)
func (r *DeployConfigRepository) GetByIDWithCert(ctx context.Context, id uint) (*model.DeployConfig, error) {
	var config model.DeployConfig
	err := r.db.WithContext(ctx).
		Preload("Certificate").
		First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// List 部署配置列表
func (r *DeployConfigRepository) List(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.DeployConfig, int64, error) {
	var configs []model.DeployConfig
	var total int64

	query := r.db.WithContext(ctx).Model(&model.DeployConfig{})

	// 应用过滤条件
	if certID, ok := filters["certificate_id"].(uint); ok && certID > 0 {
		query = query.Where("certificate_id = ?", certID)
	}
	if deployType, ok := filters["deploy_type"].(string); ok && deployType != "" {
		query = query.Where("deploy_type = ?", deployType)
	}
	if enabled, ok := filters["enabled"].(bool); ok {
		query = query.Where("enabled = ?", enabled)
	}

	// 计数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Preload("Certificate").
		Find(&configs).Error
	if err != nil {
		return nil, 0, err
	}

	return configs, total, nil
}

// ListByCertificateID 根据证书ID获取部署配置列表
func (r *DeployConfigRepository) ListByCertificateID(ctx context.Context, certID uint) ([]model.DeployConfig, error) {
	var configs []model.DeployConfig
	err := r.db.WithContext(ctx).
		Where("certificate_id = ? AND enabled = ?", certID, true).
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// ListAutoDeploy 获取自动部署配置列表
func (r *DeployConfigRepository) ListAutoDeploy(ctx context.Context, certID uint) ([]model.DeployConfig, error) {
	var configs []model.DeployConfig
	err := r.db.WithContext(ctx).
		Where("certificate_id = ? AND auto_deploy = ? AND enabled = ?", certID, true, true).
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// UpdateDeployResult 更新部署结果
func (r *DeployConfigRepository) UpdateDeployResult(ctx context.Context, id uint, ok bool, deployAt interface{}, lastError string) error {
	updates := map[string]interface{}{
		"last_deploy_ok": ok,
		"last_deploy_at": deployAt,
		"last_error":     lastError,
	}
	return r.db.WithContext(ctx).Model(&model.DeployConfig{}).Where("id = ?", id).Updates(updates).Error
}
