package repository

import (
	"context"
	"time"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"gorm.io/gorm"
)

// CertificateRepository 证书仓库
type CertificateRepository struct {
	db *gorm.DB
}

// NewCertificateRepository 创建证书仓库
func NewCertificateRepository(db *gorm.DB) *CertificateRepository {
	return &CertificateRepository{db: db}
}

// Create 创建证书
func (r *CertificateRepository) Create(ctx context.Context, cert *model.SSLCertificate) error {
	return r.db.WithContext(ctx).Create(cert).Error
}

// Update 更新证书
func (r *CertificateRepository) Update(ctx context.Context, cert *model.SSLCertificate) error {
	return r.db.WithContext(ctx).Save(cert).Error
}

// Delete 删除证书
func (r *CertificateRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.SSLCertificate{}, id).Error
}

// GetByID 根据ID获取证书
func (r *CertificateRepository) GetByID(ctx context.Context, id uint) (*model.SSLCertificate, error) {
	var cert model.SSLCertificate
	err := r.db.WithContext(ctx).First(&cert, id).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetByIDWithRelations 根据ID获取证书(含关联)
func (r *CertificateRepository) GetByIDWithRelations(ctx context.Context, id uint) (*model.SSLCertificate, error) {
	var cert model.SSLCertificate
	err := r.db.WithContext(ctx).
		Preload("DNSProvider").
		Preload("DeployConfigs").
		First(&cert, id).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetByDomain 根据域名获取证书
func (r *CertificateRepository) GetByDomain(ctx context.Context, domain string) (*model.SSLCertificate, error) {
	var cert model.SSLCertificate
	err := r.db.WithContext(ctx).Where("domain = ?", domain).First(&cert).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// List 证书列表
func (r *CertificateRepository) List(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]model.SSLCertificate, int64, error) {
	var certs []model.SSLCertificate
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SSLCertificate{})

	// 应用过滤条件
	if domain, ok := filters["domain"].(string); ok && domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain+"%")
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if sourceType, ok := filters["source_type"].(string); ok && sourceType != "" {
		query = query.Where("source_type = ?", sourceType)
	}

	// 计数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Preload("DNSProvider").
		Find(&certs).Error
	if err != nil {
		return nil, 0, err
	}

	return certs, total, nil
}

// ListExpiring 获取即将过期的证书
func (r *CertificateRepository) ListExpiring(ctx context.Context) ([]model.SSLCertificate, error) {
	var certs []model.SSLCertificate
	err := r.db.WithContext(ctx).
		Where("auto_renew = ? AND status != ?", true, model.CertStatusError).
		Where("not_after IS NOT NULL").
		Where("not_after <= DATE_ADD(NOW(), INTERVAL renew_days_before DAY)").
		Find(&certs).Error
	if err != nil {
		return nil, err
	}
	return certs, nil
}

// ListExpiringCustom 自定义条件获取即将过期的证书
func (r *CertificateRepository) ListExpiringCustom(ctx context.Context, beforeDays int) ([]model.SSLCertificate, error) {
	var certs []model.SSLCertificate
	expireTime := time.Now().AddDate(0, 0, beforeDays)
	err := r.db.WithContext(ctx).
		Where("auto_renew = ?", true).
		Where("status IN ?", []string{model.CertStatusActive, model.CertStatusExpiring}).
		Where("not_after IS NOT NULL AND not_after <= ?", expireTime).
		Find(&certs).Error
	if err != nil {
		return nil, err
	}
	return certs, nil
}

// UpdateStatus 更新证书状态
func (r *CertificateRepository) UpdateStatus(ctx context.Context, id uint, status string, lastError string) error {
	updates := map[string]interface{}{
		"status":     status,
		"last_error": lastError,
	}
	return r.db.WithContext(ctx).Model(&model.SSLCertificate{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateCertContent 更新证书内容
func (r *CertificateRepository) UpdateCertContent(ctx context.Context, id uint, cert, key, chain string, notBefore, notAfter *time.Time, fingerprint string) error {
	updates := map[string]interface{}{
		"certificate":   cert,
		"private_key":   key,
		"cert_chain":    chain,
		"not_before":    notBefore,
		"not_after":     notAfter,
		"fingerprint":   fingerprint,
		"status":        model.CertStatusActive,
		"last_renew_at": time.Now(),
		"last_error":    "",
	}
	return r.db.WithContext(ctx).Model(&model.SSLCertificate{}).Where("id = ?", id).Updates(updates).Error
}

// CountByStatus 按状态统计证书数量
func (r *CertificateRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	result := make(map[string]int64)
	var counts []struct {
		Status string
		Count  int64
	}
	err := r.db.WithContext(ctx).Model(&model.SSLCertificate{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&counts).Error
	if err != nil {
		return nil, err
	}
	for _, c := range counts {
		result[c.Status] = c.Count
	}
	return result, nil
}
