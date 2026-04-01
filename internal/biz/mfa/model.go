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
	"time"
)

// MFAType MFA类型
type MFAType string

const (
	MFATypeTOTP MFAType = "totp" // 基于时间的一次性密码
)

// UserMFA 用户MFA配置（对应 mfa_settings 表）
type UserMFA struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"uniqueIndex;not null" json:"userId"`
	TOTPEnabled  bool       `gorm:"column:totp_enabled;default:false" json:"totpEnabled"`
	TOTPSecret   string     `gorm:"column:totp_secret;type:varchar(255)" json:"-"` // 加密存储
	TOTPVerified bool       `gorm:"column:totp_verified;default:false" json:"totpVerified"`
	BackupCodes  string     `gorm:"type:text" json:"-"` // 加密存储的备用码JSON
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (UserMFA) TableName() string {
	return "mfa_settings"
}

// IsEnabled 是否启用MFA
func (m *UserMFA) IsEnabled() bool {
	return m.TOTPEnabled && m.TOTPVerified
}

// MFALog MFA验证日志
type MFALog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	MFAType    MFAType   `gorm:"type:varchar(20)" json:"mfaType"`
	Action     string    `gorm:"type:varchar(20);not null" json:"action"` // verify/enable/disable/setup
	IPAddress  string    `gorm:"type:varchar(45)" json:"ipAddress"`
	UserAgent  string    `gorm:"type:varchar(500)" json:"userAgent"`
	Success    bool      `gorm:"default:false" json:"success"`
	Message    string    `gorm:"type:varchar(255)" json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

// TableName 指定表名
func (MFALog) TableName() string {
	return "sys_mfa_log"
}

// MFASetupResponse MFA设置响应
type MFASetupResponse struct {
	Secret     string `json:"secret"`
	QRCodeURL  string `json:"qrCodeUrl"`
	ManualCode string `json:"manualCode"`
}

// MFAStatusResponse MFA状态响应
type MFAStatusResponse struct {
	IsEnabled  bool      `json:"isEnabled"`
	MFAType    MFAType   `json:"mfaType"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	HasBackup  bool      `json:"hasBackup"`
}

// BackupCode 备用码
type BackupCode struct {
	Code    string `json:"code"`
	Used    bool   `json:"used"`
	UsedAt  *time.Time `json:"usedAt,omitempty"`
}

// TrustedDevice MFA信任设备（对应 mfa_trusted_devices 表）
type TrustedDevice struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"index;not null" json:"userId"`
	DeviceToken     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"`
	DeviceName      string    `gorm:"type:varchar(255)" json:"deviceName"`
	IPAddress       string    `gorm:"type:varchar(45)" json:"ipAddress"`
	LastVerifiedAt  time.Time `json:"lastVerifiedAt"`
	ExpiresAt       time.Time `gorm:"index" json:"expiresAt"`
	CreatedAt       time.Time `json:"createdAt"`
}

// TableName 指定表名
func (TrustedDevice) TableName() string {
	return "mfa_trusted_devices"
}

// IsValid 检查信任设备是否仍然有效
func (d *TrustedDevice) IsValid() bool {
	return time.Now().Before(d.ExpiresAt)
}
