package repository

import (
	"context"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"gorm.io/gorm"
)

// DNSProviderRepository DNS服务商仓库
type DNSProviderRepository struct {
	db *gorm.DB
}

// NewDNSProviderRepository 创建DNS服务商仓库
func NewDNSProviderRepository(db *gorm.DB) *DNSProviderRepository {
	return &DNSProviderRepository{db: db}
}

// Create 创建DNS服务商
func (r *DNSProviderRepository) Create(ctx context.Context, provider *model.DNSProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// Update 更新DNS服务商
func (r *DNSProviderRepository) Update(ctx context.Context, provider *model.DNSProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// Delete 删除DNS服务商
func (r *DNSProviderRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.DNSProvider{}, id).Error
}

// GetByID 根据ID获取DNS服务商
func (r *DNSProviderRepository) GetByID(ctx context.Context, id uint) (*model.DNSProvider, error) {
	var provider model.DNSProvider
	err := r.db.WithContext(ctx).First(&provider, id).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// List DNS服务商列表
func (r *DNSProviderRepository) List(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.DNSProvider, int64, error) {
	var providers []model.DNSProvider
	var total int64

	query := r.db.WithContext(ctx).Model(&model.DNSProvider{})

	// 应用过滤条件
	if name, ok := filters["name"].(string); ok && name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if providerType, ok := filters["provider"].(string); ok && providerType != "" {
		query = query.Where("provider = ?", providerType)
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
		Find(&providers).Error
	if err != nil {
		return nil, 0, err
	}

	return providers, total, nil
}

// ListAll 获取所有DNS服务商
func (r *DNSProviderRepository) ListAll(ctx context.Context) ([]model.DNSProvider, error) {
	var providers []model.DNSProvider
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("name ASC").
		Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// UpdateTestResult 更新测试结果
func (r *DNSProviderRepository) UpdateTestResult(ctx context.Context, id uint, ok bool, testAt interface{}) error {
	updates := map[string]interface{}{
		"last_test_ok": ok,
		"last_test_at": testAt,
	}
	return r.db.WithContext(ctx).Model(&model.DNSProvider{}).Where("id = ?", id).Updates(updates).Error
}
