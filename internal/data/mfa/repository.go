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

package mfa

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/mfa"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

// NewRepository 创建MFA数据仓库
func NewRepository(db *gorm.DB) mfa.Repository {
	return &repository{db: db}
}

// GetUserMFA 获取用户的MFA配置
func (r *repository) GetUserMFA(ctx context.Context, userID uint) (*mfa.UserMFA, error) {
	var mfaConfig mfa.UserMFA
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&mfaConfig).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &mfaConfig, nil
}

// CreateMFA 创建MFA配置
func (r *repository) CreateMFA(ctx context.Context, mfaConfig *mfa.UserMFA) error {
	return r.db.WithContext(ctx).Create(mfaConfig).Error
}

// UpdateMFA 更新MFA配置
func (r *repository) UpdateMFA(ctx context.Context, mfaConfig *mfa.UserMFA) error {
	return r.db.WithContext(ctx).Save(mfaConfig).Error
}

// DeleteMFA 删除MFA配置
func (r *repository) DeleteMFA(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&mfa.UserMFA{}).Error
}

// CreateLog 创建MFA日志
func (r *repository) CreateLog(ctx context.Context, log *mfa.MFALog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// MarkBackupCodeUsed 标记备用码已使用
func (r *repository) MarkBackupCodeUsed(ctx context.Context, userID uint, code string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mfaConfig mfa.UserMFA
		if err := tx.Where("user_id = ?", userID).First(&mfaConfig).Error; err != nil {
			return err
		}

		// 解析备用码JSON
		var backupCodes []mfa.BackupCode
		if err := json.Unmarshal([]byte(mfaConfig.BackupCodes), &backupCodes); err != nil {
			return err
		}

		// 标记为已使用
		now := time.Now()
		for i, bc := range backupCodes {
			if bc.Code == code && !bc.Used {
				backupCodes[i].Used = true
				backupCodes[i].UsedAt = &now
				break
			}
		}

		// 保存更新
		updatedJSON, _ := json.Marshal(backupCodes)
		return tx.Model(&mfa.UserMFA{}).Where("user_id = ?", userID).
			Update("backup_codes", string(updatedJSON)).Error
	})
}

// SaveTrustedDevice 保存信任设备
func (r *repository) SaveTrustedDevice(ctx context.Context, device *mfa.TrustedDevice) error {
	return r.db.WithContext(ctx).Create(device).Error
}

// GetTrustedDevice 通过token获取信任设备
func (r *repository) GetTrustedDevice(ctx context.Context, deviceToken string) (*mfa.TrustedDevice, error) {
	var device mfa.TrustedDevice
	err := r.db.WithContext(ctx).Where("device_token = ?", deviceToken).First(&device).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

// DeleteExpiredTrustedDevices 清理过期的信任设备
func (r *repository) DeleteExpiredTrustedDevices(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&mfa.TrustedDevice{}).Error
}

// DeleteUserTrustedDevices 删除用户所有信任设备
func (r *repository) DeleteUserTrustedDevices(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&mfa.TrustedDevice{}).Error
}
