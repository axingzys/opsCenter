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

package system

import (
	"context"
	"time"

	"github.com/ydcloud-dy/opshub/internal/biz/system"
	"gorm.io/gorm"
)

// ConfigRepo 系统配置仓库
type ConfigRepo struct {
	db *gorm.DB
}

// NewConfigRepo 创建系统配置仓库
func NewConfigRepo(db *gorm.DB) *ConfigRepo {
	return &ConfigRepo{db: db}
}

// GetByKey 根据Key获取配置
func (r *ConfigRepo) GetByKey(ctx context.Context, key string) (*system.SysConfig, error) {
	var config system.SysConfig
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetByGroup 根据分组获取配置
func (r *ConfigRepo) GetByGroup(ctx context.Context, group string) ([]*system.SysConfig, error) {
	var configs []*system.SysConfig
	err := r.db.WithContext(ctx).Where("`group` = ?", group).Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAll 获取所有配置
func (r *ConfigRepo) GetAll(ctx context.Context) ([]*system.SysConfig, error) {
	var configs []*system.SysConfig
	err := r.db.WithContext(ctx).Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// Save 保存配置（存在则更新，不存在则创建）
func (r *ConfigRepo) Save(ctx context.Context, config *system.SysConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// SaveOrUpdate 保存或更新配置
func (r *ConfigRepo) SaveOrUpdate(ctx context.Context, key, value string) error {
	var config system.SysConfig
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		// 配置不存在，从默认配置中获取并创建
		if defaultConfig, ok := system.DefaultConfigs[key]; ok {
			config = defaultConfig
			config.Value = value
			return r.db.WithContext(ctx).Create(&config).Error
		}
		// 默认配置也不存在，直接创建
		config = system.SysConfig{
			Key:   key,
			Value: value,
			Type:  "string",
		}
		return r.db.WithContext(ctx).Create(&config).Error
	}
	if err != nil {
		return err
	}
	// 配置存在，更新值
	return r.db.WithContext(ctx).Model(&config).Update("value", value).Error
}

// BatchSaveOrUpdate 批量保存或更新配置
func (r *ConfigRepo) BatchSaveOrUpdate(ctx context.Context, configs map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range configs {
			var config system.SysConfig
			err := tx.Where("`key` = ?", key).First(&config).Error
			if err == gorm.ErrRecordNotFound {
				// 配置不存在，从默认配置中获取并创建
				if defaultConfig, ok := system.DefaultConfigs[key]; ok {
					config = defaultConfig
					config.Value = value
					if err := tx.Create(&config).Error; err != nil {
						return err
					}
					continue
				}
				// 默认配置也不存在，直接创建
				config = system.SysConfig{
					Key:   key,
					Value: value,
					Type:  "string",
				}
				if err := tx.Create(&config).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			// 配置存在，更新值
			if err := tx.Model(&config).Update("value", value).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// InitDefaultConfigs 初始化默认配置
func (r *ConfigRepo) InitDefaultConfigs(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, config := range system.DefaultConfigs {
			var existing system.SysConfig
			err := tx.Where("`key` = ?", config.Key).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				// 不存在则创建
				if err := tx.Create(&config).Error; err != nil {
					return err
				}
			}
			// 已存在则跳过
		}
		return nil
	})
}

// LoginAttemptRepo 登录尝试记录仓库
type LoginAttemptRepo struct {
	db *gorm.DB
}

// NewLoginAttemptRepo 创建登录尝试记录仓库
func NewLoginAttemptRepo(db *gorm.DB) *LoginAttemptRepo {
	return &LoginAttemptRepo{db: db}
}

// GetByUsername 根据用户名获取登录尝试记录
func (r *LoginAttemptRepo) GetByUsername(ctx context.Context, username string) (*system.SysUserLoginAttempt, error) {
	var attempt system.SysUserLoginAttempt
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// IncrementFailCount 增加失败次数
func (r *LoginAttemptRepo) IncrementFailCount(ctx context.Context, username string, maxAttempts int, lockoutDuration int) error {
	var attempt system.SysUserLoginAttempt
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&attempt).Error

	now := time.Now()
	if err == gorm.ErrRecordNotFound {
		// 记录不存在，创建新记录
		attempt = system.SysUserLoginAttempt{
			Username:   username,
			FailCount:  1,
			LastFailAt: now,
		}
		// 如果达到最大失败次数，设置锁定时间
		if attempt.FailCount >= maxAttempts {
			lockedUntil := now.Add(time.Duration(lockoutDuration) * time.Second)
			attempt.LockedUntil = &lockedUntil
		}
		if createErr := r.db.WithContext(ctx).Create(&attempt).Error; createErr != nil {
			return createErr
		}
		return nil
	}
	if err != nil {
		return err
	}

	// 检查是否已经过了锁定时间
	if attempt.LockedUntil != nil && now.After(*attempt.LockedUntil) {
		// 锁定已过期，重置计数
		attempt.FailCount = 1
		attempt.LockedUntil = nil
	} else {
		// 增加失败次数
		attempt.FailCount++
	}
	attempt.LastFailAt = now

	// 如果达到最大失败次数，设置锁定时间
	if attempt.FailCount >= maxAttempts {
		lockedUntil := now.Add(time.Duration(lockoutDuration) * time.Second)
		attempt.LockedUntil = &lockedUntil
	}

	return r.db.WithContext(ctx).Save(&attempt).Error
}

// ResetFailCount 重置失败次数
func (r *LoginAttemptRepo) ResetFailCount(ctx context.Context, username string) error {
	return r.db.WithContext(ctx).
		Where("username = ?", username).
		Delete(&system.SysUserLoginAttempt{}).Error
}

// IsLocked 检查用户是否被锁定
func (r *LoginAttemptRepo) IsLocked(ctx context.Context, username string) (bool, time.Time, error) {
	var attempt system.SysUserLoginAttempt
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&attempt).Error
	if err == gorm.ErrRecordNotFound {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}

	now := time.Now()
	if attempt.LockedUntil != nil && now.Before(*attempt.LockedUntil) {
		return true, *attempt.LockedUntil, nil
	}
	return false, time.Time{}, nil
}
