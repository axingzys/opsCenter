// Copyright (c) 2026 DYCloud J.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package asset

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/asset"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type hostRepo struct {
	db *gorm.DB
}

// NewHostRepo 创建主机仓库
func NewHostRepo(db *gorm.DB) asset.HostRepo {
	return &hostRepo{db: db}
}

// Create 创建主机
func (r *hostRepo) Create(ctx context.Context, host *asset.Host) error {
	return r.db.WithContext(ctx).Create(host).Error
}

// CreateOrUpdate 创建或恢复主机（如果IP已被软删除则恢复）
func (r *hostRepo) CreateOrUpdate(ctx context.Context, host *asset.Host) error {
	// 先查找是否存在该IP的记录（包括软删除的）
	var existing asset.Host
	err := r.db.WithContext(ctx).Unscoped().Where("ip = ?", host.IP).First(&existing).Error

	if err == nil {
		// 找到了记录
		if existing.DeletedAt.Valid {
			// 记录已被软删除，恢复它
			existing.Name = host.Name
			existing.GroupID = host.GroupID
			existing.OSType = host.OSType
			existing.SSHUser = host.SSHUser
			existing.IP = host.IP
			existing.Port = host.Port
			existing.CredentialID = host.CredentialID
			existing.ManagementMode = host.ManagementMode
			existing.ManagementPort = host.ManagementPort
			existing.ManagementCredentialID = host.ManagementCredentialID
			existing.DesktopEnabled = host.DesktopEnabled
			existing.DesktopProtocol = host.DesktopProtocol
			existing.DesktopPort = host.DesktopPort
			existing.DesktopCredentialID = host.DesktopCredentialID
			existing.DesktopSecurity = host.DesktopSecurity
			existing.DesktopIgnoreCert = host.DesktopIgnoreCert
			existing.Tags = host.Tags
			existing.Description = host.Description
			existing.Status = host.Status
			existing.CollectStatus = host.CollectStatus
			existing.CollectError = host.CollectError
			existing.LastCollectAt = host.LastCollectAt
			existing.DeletedAt.Time = *new(time.Time) // 清除删除时间
			existing.DeletedAt.Valid = false
			return r.db.WithContext(ctx).Unscoped().Save(&existing).Error
		}
		// 记录未被删除，返回错误
		return fmt.Errorf("IP地址 %s 已存在", host.IP)
	}

	// 没找到记录，创建新的
	return r.db.WithContext(ctx).Create(host).Error
}

// Update 更新主机
func (r *hostRepo) Update(ctx context.Context, host *asset.Host) error {
	return r.db.WithContext(ctx).Save(host).Error
}

// Delete 删除主机
func (r *hostRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&asset.Host{}, id).Error
}

// GetByID 根据ID获取主机
func (r *hostRepo) GetByID(ctx context.Context, id uint) (*asset.Host, error) {
	var host asset.Host
	err := r.db.WithContext(ctx).First(&host, id).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// List 列表查询
func (r *hostRepo) List(ctx context.Context, page, pageSize int, keyword string, groupIDs []uint, accessibleHostIDs []uint, status *int) ([]*asset.Host, int64, error) {
	var hosts []*asset.Host
	var total int64

	query := r.db.WithContext(ctx).Model(&asset.Host{})

	if keyword != "" {
		query = query.Where("name LIKE ? OR ip LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 添加分组ID筛选（支持多个分组ID）
	if len(groupIDs) > 0 {
		query = query.Where("group_id IN ?", groupIDs)
	}

	// 添加状态筛选
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 添加可访问主机ID筛选
	// 如果 accessibleHostIDs 为空切片（非nil），表示用户没有任何权限，应该返回空列表
	// 如果 accessibleHostIDs 为nil，表示不进行权限筛选（管理员或未启用权限控制）
	if accessibleHostIDs != nil {
		if len(accessibleHostIDs) == 0 {
			// 用户没有任何主机访问权限，返回空列表
			return []*asset.Host{}, 0, nil
		}
		query = query.Where("id IN ?", accessibleHostIDs)
	}

	err := query.Order("id DESC").Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&hosts).Error
	if err != nil {
		return nil, 0, err
	}

	return hosts, total, nil
}

// GetByGroupID 根据分组ID获取主机列表
func (r *hostRepo) GetByGroupID(ctx context.Context, groupID uint) ([]*asset.Host, error) {
	var hosts []*asset.Host
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

// GetByIP 根据IP获取主机
func (r *hostRepo) GetByIP(ctx context.Context, ip string) (*asset.Host, error) {
	var host asset.Host
	err := r.db.WithContext(ctx).Where("ip = ?", ip).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// GetByCloudInstanceID 根据云实例ID获取主机
func (r *hostRepo) GetByCloudInstanceID(ctx context.Context, instanceID string) (*asset.Host, error) {
	var host asset.Host
	err := r.db.WithContext(ctx).Where("cloud_instance_id = ?", instanceID).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

// CountByCredentialID 统计使用指定凭证的主机数量
func (r *hostRepo) CountByCredentialID(ctx context.Context, credentialID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&asset.Host{}).
		Where("credential_id = ? OR management_credential_id = ? OR desktop_credential_id = ?", credentialID, credentialID, credentialID).
		Count(&count).Error
	return count, err
}

// credentialRepo 凭证仓库
type credentialRepo struct {
	db            *gorm.DB
	encryptionKey []byte
}

// NewCredentialRepo 创建凭证仓库
func NewCredentialRepo(db *gorm.DB) asset.CredentialRepo {
	// AES-256要求密钥长度必须是32字节（256位）
	encryptionKey := []byte("opshub-enc-key-32-bytes-long!!!!")
	return &credentialRepo{
		db:            db,
		encryptionKey: encryptionKey,
	}
}

// encrypt 加密
func (r *credentialRepo) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(r.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密
func (r *credentialRepo) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(r.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Create 创建凭证
func (r *credentialRepo) Create(ctx context.Context, credential *asset.Credential) error {
	// 加密敏感信息
	if credential.Password != "" {
		encrypted, err := r.encrypt(credential.Password)
		if err != nil {
			return fmt.Errorf("加密密码失败: %w", err)
		}
		credential.Password = encrypted
	}

	if credential.PrivateKey != "" {
		encrypted, err := r.encrypt(credential.PrivateKey)
		if err != nil {
			return fmt.Errorf("加密私钥失败: %w", err)
		}
		credential.PrivateKey = encrypted
	}

	if credential.Passphrase != "" {
		encrypted, err := r.encrypt(credential.Passphrase)
		if err != nil {
			return fmt.Errorf("加密私钥密码失败: %w", err)
		}
		credential.Passphrase = encrypted
	}

	return r.db.WithContext(ctx).Create(credential).Error
}

// Update 更新凭证
func (r *credentialRepo) Update(ctx context.Context, credential *asset.Credential) error {
	// 加密敏感信息
	if credential.Password != "" {
		encrypted, err := r.encrypt(credential.Password)
		if err != nil {
			return fmt.Errorf("加密密码失败: %w", err)
		}
		credential.Password = encrypted
	}

	if credential.PrivateKey != "" {
		encrypted, err := r.encrypt(credential.PrivateKey)
		if err != nil {
			return fmt.Errorf("加密私钥失败: %w", err)
		}
		credential.PrivateKey = encrypted
	}

	if credential.Passphrase != "" {
		encrypted, err := r.encrypt(credential.Passphrase)
		if err != nil {
			return fmt.Errorf("加密私钥密码失败: %w", err)
		}
		credential.Passphrase = encrypted
	}

	return r.db.WithContext(ctx).Save(credential).Error
}

// Delete 删除凭证
func (r *credentialRepo) Delete(ctx context.Context, id uint) error {
	// 检查是否有主机使用此凭证
	var count int64
	if err := r.db.WithContext(ctx).Model(&asset.Host{}).
		Where("credential_id = ? OR management_credential_id = ? OR desktop_credential_id = ?", id, id, id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该凭证正在被 %d 个主机使用，无法删除", count)
	}

	return r.db.WithContext(ctx).Delete(&asset.Credential{}, id).Error
}

// GetByID 根据ID获取凭证
func (r *credentialRepo) GetByID(ctx context.Context, id uint) (*asset.Credential, error) {
	var credential asset.Credential
	err := r.db.WithContext(ctx).First(&credential, id).Error
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

// GetByIDDecrypted 根据ID获取凭证（解密后的）
func (r *credentialRepo) GetByIDDecrypted(ctx context.Context, id uint) (*asset.Credential, error) {
	credential, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 解密敏感信息
	if credential.Password != "" {
		decrypted, err := r.decrypt(credential.Password)
		if err != nil {
			return nil, fmt.Errorf("解密密码失败: %w", err)
		}
		credential.Password = decrypted
	}

	if credential.PrivateKey != "" {
		decrypted, err := r.decrypt(credential.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("解密私钥失败: %w", err)
		}
		credential.PrivateKey = decrypted
	}

	if credential.Passphrase != "" {
		decrypted, err := r.decrypt(credential.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("解密私钥密码失败: %w", err)
		}
		credential.Passphrase = decrypted
	}

	return credential, nil
}

// List 列表查询
func (r *credentialRepo) List(ctx context.Context, page, pageSize int, keyword string) ([]*asset.Credential, int64, error) {
	var credentials []*asset.Credential
	var total int64

	query := r.db.WithContext(ctx).Model(&asset.Credential{})

	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	err := query.Order("id DESC").Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&credentials).Error
	if err != nil {
		return nil, 0, err
	}

	return credentials, total, nil
}

// GetAll 获取所有凭证
func (r *credentialRepo) GetAll(ctx context.Context) ([]*asset.Credential, error) {
	var credentials []*asset.Credential
	err := r.db.WithContext(ctx).Order("id DESC").Find(&credentials).Error
	if err != nil {
		return nil, err
	}
	return credentials, nil
}

// cloudAccountRepo 云平台账号仓库
type cloudAccountRepo struct {
	db *gorm.DB
}

// NewCloudAccountRepo 创建云平台账号仓库
func NewCloudAccountRepo(db *gorm.DB) asset.CloudAccountRepo {
	return &cloudAccountRepo{db: db}
}

// Create 创建云平台账号
func (r *cloudAccountRepo) Create(ctx context.Context, account *asset.CloudAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

// Update 更新云平台账号
func (r *cloudAccountRepo) Update(ctx context.Context, account *asset.CloudAccount) error {
	return r.db.WithContext(ctx).Save(account).Error
}

// Delete 删除云平台账号
func (r *cloudAccountRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&asset.CloudAccount{}, id).Error
}

// GetByID 根据ID获取云平台账号
func (r *cloudAccountRepo) GetByID(ctx context.Context, id uint) (*asset.CloudAccount, error) {
	var account asset.CloudAccount
	err := r.db.WithContext(ctx).First(&account, id).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// List 列表查询
func (r *cloudAccountRepo) List(ctx context.Context, page, pageSize int) ([]*asset.CloudAccount, int64, error) {
	var accounts []*asset.CloudAccount
	var total int64

	query := r.db.WithContext(ctx).Model(&asset.CloudAccount{})

	err := query.Order("id DESC").Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&accounts).Error
	if err != nil {
		return nil, 0, err
	}

	return accounts, total, nil
}

// GetAll 获取所有启用的云平台账号
func (r *cloudAccountRepo) GetAll(ctx context.Context) ([]*asset.CloudAccount, error) {
	var accounts []*asset.CloudAccount
	err := r.db.WithContext(ctx).Order("id DESC").Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

type virtualizationPlatformRepo struct {
	db *gorm.DB
}

func NewVirtualizationPlatformRepo(db *gorm.DB) asset.VirtualizationPlatformRepo {
	return &virtualizationPlatformRepo{db: db}
}

func (r *virtualizationPlatformRepo) Create(ctx context.Context, platform *asset.VirtualizationPlatform) error {
	return r.db.WithContext(ctx).Create(platform).Error
}

func (r *virtualizationPlatformRepo) Update(ctx context.Context, platform *asset.VirtualizationPlatform) error {
	return r.db.WithContext(ctx).Save(platform).Error
}

func (r *virtualizationPlatformRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&asset.VirtualizationPlatform{}, id).Error
}

func (r *virtualizationPlatformRepo) GetByID(ctx context.Context, id uint) (*asset.VirtualizationPlatform, error) {
	var item asset.VirtualizationPlatform
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *virtualizationPlatformRepo) List(ctx context.Context, page, pageSize int, keyword string) ([]*asset.VirtualizationPlatform, int64, error) {
	var (
		items []*asset.VirtualizationPlatform
		total int64
	)

	query := r.db.WithContext(ctx).Model(&asset.VirtualizationPlatform{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("name LIKE ? OR endpoint LIKE ? OR provider LIKE ?", like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *virtualizationPlatformRepo) ListEnabled(ctx context.Context) ([]*asset.VirtualizationPlatform, error) {
	var items []*asset.VirtualizationPlatform
	if err := r.db.WithContext(ctx).
		Where("status = ?", "enabled").
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationClusterRepo struct {
	db *gorm.DB
}

func NewVirtualizationClusterRepo(db *gorm.DB) asset.VirtualizationClusterRepo {
	return &virtualizationClusterRepo{db: db}
}

func (r *virtualizationClusterRepo) UpsertBatch(ctx context.Context, items []*asset.VirtualizationCluster) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "platform_id"}, {Name: "external_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name",
				"datacenter",
				"host_count",
				"guest_count",
				"status",
				"last_collected_at",
				"raw_payload_digest",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(&items).Error
}

func (r *virtualizationClusterRepo) GetByID(ctx context.Context, id uint) (*asset.VirtualizationCluster, error) {
	var item asset.VirtualizationCluster
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *virtualizationClusterRepo) ListByPlatformID(ctx context.Context, platformID uint) ([]*asset.VirtualizationCluster, error) {
	var items []*asset.VirtualizationCluster
	if err := r.db.WithContext(ctx).
		Where("platform_id = ?", platformID).
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationHostRepo struct {
	db *gorm.DB
}

func NewVirtualizationHostRepo(db *gorm.DB) asset.VirtualizationHostRepo {
	return &virtualizationHostRepo{db: db}
}

func (r *virtualizationHostRepo) UpsertBatch(ctx context.Context, items []*asset.VirtualizationHost) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "platform_id"}, {Name: "external_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"cluster_id",
				"name",
				"management_ip",
				"cpu_model",
				"cpu_cores",
				"memory_total_mb",
				"memory_used_mb",
				"guest_count",
				"status",
				"last_collected_at",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(&items).Error
}

func (r *virtualizationHostRepo) GetByID(ctx context.Context, id uint) (*asset.VirtualizationHost, error) {
	var item asset.VirtualizationHost
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *virtualizationHostRepo) ListByPlatformID(ctx context.Context, platformID uint) ([]*asset.VirtualizationHost, error) {
	var items []*asset.VirtualizationHost
	if err := r.db.WithContext(ctx).
		Where("platform_id = ?", platformID).
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationGuestRepo struct {
	db *gorm.DB
}

func NewVirtualizationGuestRepo(db *gorm.DB) asset.VirtualizationGuestRepo {
	return &virtualizationGuestRepo{db: db}
}

func (r *virtualizationGuestRepo) UpsertBatch(ctx context.Context, items []*asset.VirtualizationGuest) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "platform_id"}, {Name: "external_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"cluster_id",
				"host_id",
				"name",
				"os_type",
				"power_state",
				"cpu_count",
				"memory_mb",
				"primary_ip",
				"tools_status",
				"binding_status",
				"last_collected_at",
				"updated_at",
				"deleted_at",
			}),
		}).
		Create(&items).Error
}

func (r *virtualizationGuestRepo) GetByID(ctx context.Context, id uint) (*asset.VirtualizationGuest, error) {
	var item asset.VirtualizationGuest
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *virtualizationGuestRepo) List(ctx context.Context, req *asset.VirtualizationGuestListRequest) ([]*asset.VirtualizationGuest, int64, error) {
	var (
		items []*asset.VirtualizationGuest
		total int64
	)

	query := r.db.WithContext(ctx).Model(&asset.VirtualizationGuest{})

	if req != nil {
		if req.PlatformID > 0 {
			query = query.Where("platform_id = ?", req.PlatformID)
		}
		if req.ClusterID > 0 {
			query = query.Where("cluster_id = ?", req.ClusterID)
		}
		if state := strings.TrimSpace(req.PowerState); state != "" {
			query = query.Where("power_state = ?", state)
		}
		switch strings.TrimSpace(req.Bound) {
		case "bound":
			query = query.Where("binding_status = ?", "bound")
		case "unbound":
			query = query.Where("binding_status <> ?", "bound")
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("name LIKE ? OR external_id LIKE ? OR primary_ip LIKE ?", like, like, like)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *virtualizationGuestRepo) ListByPlatformID(ctx context.Context, platformID uint) ([]*asset.VirtualizationGuest, error) {
	var items []*asset.VirtualizationGuest
	if err := r.db.WithContext(ctx).
		Where("platform_id = ?", platformID).
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationGuestBindingRepo struct {
	db *gorm.DB
}

func NewVirtualizationGuestBindingRepo(db *gorm.DB) asset.VirtualizationGuestBindingRepo {
	return &virtualizationGuestBindingRepo{db: db}
}

func (r *virtualizationGuestBindingRepo) Create(ctx context.Context, binding *asset.VirtualizationGuestBinding) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

func (r *virtualizationGuestBindingRepo) Update(ctx context.Context, binding *asset.VirtualizationGuestBinding) error {
	return r.db.WithContext(ctx).Save(binding).Error
}

func (r *virtualizationGuestBindingRepo) GetActiveByGuestID(ctx context.Context, guestID uint) (*asset.VirtualizationGuestBinding, error) {
	var binding asset.VirtualizationGuestBinding
	if err := r.db.WithContext(ctx).
		Where("guest_id = ? AND status = ?", guestID, "active").
		First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *virtualizationGuestBindingRepo) GetByGuestID(ctx context.Context, guestID uint) (*asset.VirtualizationGuestBinding, error) {
	var binding asset.VirtualizationGuestBinding
	if err := r.db.WithContext(ctx).
		Where("guest_id = ?", guestID).
		Order("id DESC").
		First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

type virtualizationSyncJobRepo struct {
	db *gorm.DB
}

func NewVirtualizationSyncJobRepo(db *gorm.DB) asset.VirtualizationSyncJobRepo {
	return &virtualizationSyncJobRepo{db: db}
}

func (r *virtualizationSyncJobRepo) Create(ctx context.Context, job *asset.VirtualizationSyncJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *virtualizationSyncJobRepo) Update(ctx context.Context, job *asset.VirtualizationSyncJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *virtualizationSyncJobRepo) ListByPlatformID(ctx context.Context, platformID uint, page, pageSize int) ([]*asset.VirtualizationSyncJob, int64, error) {
	var (
		items []*asset.VirtualizationSyncJob
		total int64
	)
	query := r.db.WithContext(ctx).Model(&asset.VirtualizationSyncJob{}).Where("platform_id = ?", platformID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type virtualizationPlatformMetricRepo struct {
	db *gorm.DB
}

func NewVirtualizationPlatformMetricRepo(db *gorm.DB) asset.VirtualizationPlatformMetricRepo {
	return &virtualizationPlatformMetricRepo{db: db}
}

func (r *virtualizationPlatformMetricRepo) Create(ctx context.Context, item *asset.VirtualizationPlatformMetric) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *virtualizationPlatformMetricRepo) ListByPlatformID(ctx context.Context, platformID uint, start, end time.Time) ([]*asset.VirtualizationPlatformMetric, error) {
	var items []*asset.VirtualizationPlatformMetric
	query := r.db.WithContext(ctx).
		Where("platform_id = ?", platformID).
		Where("collected_at >= ?", start).
		Where("collected_at <= ?", end).
		Order("collected_at ASC")
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationClusterMetricRepo struct {
	db *gorm.DB
}

func NewVirtualizationClusterMetricRepo(db *gorm.DB) asset.VirtualizationClusterMetricRepo {
	return &virtualizationClusterMetricRepo{db: db}
}

func (r *virtualizationClusterMetricRepo) CreateBatch(ctx context.Context, items []*asset.VirtualizationClusterMetric) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *virtualizationClusterMetricRepo) ListByClusterID(ctx context.Context, clusterID uint, start, end time.Time) ([]*asset.VirtualizationClusterMetric, error) {
	var items []*asset.VirtualizationClusterMetric
	query := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Where("collected_at >= ?", start).
		Where("collected_at <= ?", end).
		Order("collected_at ASC")
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type virtualizationActionLogRepo struct {
	db *gorm.DB
}

func NewVirtualizationActionLogRepo(db *gorm.DB) asset.VirtualizationActionLogRepo {
	return &virtualizationActionLogRepo{db: db}
}

func (r *virtualizationActionLogRepo) Create(ctx context.Context, item *asset.VirtualizationActionLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *virtualizationActionLogRepo) Update(ctx context.Context, item *asset.VirtualizationActionLog) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *virtualizationActionLogRepo) List(ctx context.Context, req *asset.VirtualizationActionLogListRequest) ([]*asset.VirtualizationActionLog, int64, error) {
	var (
		items []*asset.VirtualizationActionLog
		total int64
	)

	query := r.db.WithContext(ctx).Model(&asset.VirtualizationActionLog{})
	if req != nil {
		if req.PlatformID > 0 {
			query = query.Where("platform_id = ?", req.PlatformID)
		}
		if req.GuestID > 0 {
			query = query.Where("guest_id = ?", req.GuestID)
		}
		if action := strings.TrimSpace(req.Action); action != "" {
			query = query.Where("action = ?", action)
		}
		if status := strings.TrimSpace(req.Status); status != "" {
			query = query.Where("status = ?", status)
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("target_name LIKE ? OR operator_name LIKE ? OR reason LIKE ? OR result_message LIKE ?", like, like, like, like)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

const (
	virtualizationOnboardConflictPolicyKey  = "virtualization_onboard_conflict_policy"
	virtualizationWriteOperationsEnabledKey = "virtualization_write_operations_enabled"
)

type sysConfigRecord struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}

func (sysConfigRecord) TableName() string {
	return "sys_config"
}

type virtualizationPolicyRepo struct {
	db *gorm.DB
}

func NewVirtualizationPolicyRepo(db *gorm.DB) asset.VirtualizationPolicyRepo {
	return &virtualizationPolicyRepo{db: db}
}

func (r *virtualizationPolicyRepo) GetOnboardConflictPolicy(ctx context.Context) (string, error) {
	var item sysConfigRecord
	if err := r.db.WithContext(ctx).
		Where("`key` = ?", virtualizationOnboardConflictPolicyKey).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return asset.VirtualizationOnboardPolicyStrict, nil
		}
		return "", err
	}

	policy := strings.TrimSpace(item.Value)
	if policy == "" {
		return asset.VirtualizationOnboardPolicyStrict, nil
	}
	return policy, nil
}

func (r *virtualizationPolicyRepo) SaveOnboardConflictPolicy(ctx context.Context, policy string) error {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = asset.VirtualizationOnboardPolicyStrict
	}

	var item sysConfigRecord
	err := r.db.WithContext(ctx).
		Where("`key` = ?", virtualizationOnboardConflictPolicyKey).
		First(&item).Error
	if err == nil {
		item.Value = policy
		return r.db.WithContext(ctx).Model(&sysConfigRecord{}).Where("id = ?", item.ID).Updates(map[string]any{
			"value": policy,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return r.db.WithContext(ctx).Create(&sysConfigRecord{
		Key:   virtualizationOnboardConflictPolicyKey,
		Value: policy,
	}).Error
}

func (r *virtualizationPolicyRepo) GetWriteOperationsEnabled(ctx context.Context) (bool, error) {
	var item sysConfigRecord
	if err := r.db.WithContext(ctx).
		Where("`key` = ?", virtualizationWriteOperationsEnabledKey).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	value := strings.TrimSpace(strings.ToLower(item.Value))
	switch value {
	case "1", "true", "yes", "on":
		return true, nil
	default:
		return false, nil
	}
}

func (r *virtualizationPolicyRepo) SaveWriteOperationsEnabled(ctx context.Context, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}

	var item sysConfigRecord
	err := r.db.WithContext(ctx).
		Where("`key` = ?", virtualizationWriteOperationsEnabledKey).
		First(&item).Error
	if err == nil {
		item.Value = value
		return r.db.WithContext(ctx).Model(&sysConfigRecord{}).Where("id = ?", item.ID).Updates(map[string]any{
			"value": value,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return r.db.WithContext(ctx).Create(&sysConfigRecord{
		Key:   virtualizationWriteOperationsEnabledKey,
		Value: value,
	}).Error
}
