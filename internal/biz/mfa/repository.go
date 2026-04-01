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

import "context"

// Repository MFA数据仓库接口
type Repository interface {
	// GetUserMFA 获取用户的MFA配置
	GetUserMFA(ctx context.Context, userID uint) (*UserMFA, error)

	// CreateMFA 创建MFA配置
	CreateMFA(ctx context.Context, mfa *UserMFA) error

	// UpdateMFA 更新MFA配置
	UpdateMFA(ctx context.Context, mfa *UserMFA) error

	// DeleteMFA 删除MFA配置
	DeleteMFA(ctx context.Context, userID uint) error

	// CreateLog 创建MFA日志
	CreateLog(ctx context.Context, log *MFALog) error

	// MarkBackupCodeUsed 标记备用码已使用
	MarkBackupCodeUsed(ctx context.Context, userID uint, code string) error

	// SaveTrustedDevice 保存信任设备
	SaveTrustedDevice(ctx context.Context, device *TrustedDevice) error

	// GetTrustedDevice 通过token获取信任设备
	GetTrustedDevice(ctx context.Context, deviceToken string) (*TrustedDevice, error)

	// DeleteExpiredTrustedDevices 清理过期的信任设备
	DeleteExpiredTrustedDevices(ctx context.Context) error

	// DeleteUserTrustedDevices 删除用户所有信任设备
	DeleteUserTrustedDevices(ctx context.Context, userID uint) error
}
