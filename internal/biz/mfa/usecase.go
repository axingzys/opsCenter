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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

// UseCase MFA用例
type UseCase struct {
	repo      Repository
	issuer    string // TOTP发行者名称
	encryptKey []byte // AES加密密钥（32字节）
}

// NewUseCase 创建MFA用例
func NewUseCase(repo Repository, issuer string, encryptKey []byte) *UseCase {
	// 确保密钥长度为32字节
	if len(encryptKey) > 32 {
		encryptKey = encryptKey[:32]
	} else if len(encryptKey) < 32 {
		padded := make([]byte, 32)
		copy(padded, encryptKey)
		encryptKey = padded
	}
	return &UseCase{
		repo:      repo,
		issuer:    issuer,
		encryptKey: encryptKey,
	}
}

// SetupMFA 设置MFA（生成密钥和二维码URL）
func (uc *UseCase) SetupMFA(ctx context.Context, userID uint, username string) (*MFASetupResponse, error) {
	// 生成新密钥
	secret, err := GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	// 生成备用码
	backupCodes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("生成备用码失败: %w", err)
	}

	// 加密密钥
	encryptedSecret, err := uc.encrypt(secret)
	if err != nil {
		return nil, fmt.Errorf("加密密钥失败: %w", err)
	}

	// 加密备用码
	backupCodesJSON, _ := json.Marshal(backupCodes)
	encryptedBackupCodes, err := uc.encrypt(string(backupCodesJSON))
	if err != nil {
		return nil, fmt.Errorf("加密备用码失败: %w", err)
	}

	// 检查是否已有配置
	existing, _ := uc.repo.GetUserMFA(ctx, userID)
	if existing != nil {
		// 更新现有配置
		existing.TOTPSecret = encryptedSecret
		existing.BackupCodes = encryptedBackupCodes
		existing.TOTPEnabled = false // 需要验证后才启用
		existing.TOTPVerified = false
		if err := uc.repo.UpdateMFA(ctx, existing); err != nil {
			return nil, err
		}
	} else {
		// 创建新配置
		mfa := &UserMFA{
			UserID:       userID,
			TOTPSecret:   encryptedSecret,
			BackupCodes:  encryptedBackupCodes,
			TOTPEnabled:  false,
			TOTPVerified: false,
		}
		if err := uc.repo.CreateMFA(ctx, mfa); err != nil {
			return nil, err
		}
	}

	// 生成二维码图片（base64编码）
	qrCodeURL, err := GenerateQRCodeImage(secret, uc.issuer, username)
	if err != nil {
		return nil, fmt.Errorf("生成二维码图片失败: %w", err)
	}

	return &MFASetupResponse{
		Secret:     secret,
		QRCodeURL:  qrCodeURL,
		ManualCode: secret,
	}, nil
}

// VerifyAndEnableMFA 验证并启用MFA
func (uc *UseCase) VerifyAndEnableMFA(ctx context.Context, userID uint, code, ipAddress, userAgent string) (bool, error) {
	mfa, err := uc.repo.GetUserMFA(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("获取MFA配置失败: %w", err)
	}
	if mfa == nil {
		return false, fmt.Errorf("未找到MFA配置，请先设置")
	}

	// 解密密钥
	secret, err := uc.decrypt(mfa.TOTPSecret)
	if err != nil {
		return false, fmt.Errorf("解密密钥失败: %w", err)
	}

	// 验证TOTP码
	valid := ValidateTOTP(secret, code)

	// 记录日志
	log := &MFALog{
		UserID:    userID,
		MFAType:   MFATypeTOTP,
		Action:    "enable",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   valid,
	}
	if valid {
		log.Message = "MFA验证成功并启用"
	} else {
		log.Message = "MFA验证失败"
	}
	uc.repo.CreateLog(ctx, log)

	if !valid {
		return false, nil
	}

	// 启用MFA
	mfa.TOTPEnabled = true
	mfa.TOTPVerified = true
	if err := uc.repo.UpdateMFA(ctx, mfa); err != nil {
		return false, err
	}

	appLogger.Info("MFA已启用", zap.Uint("userID", userID))
	return true, nil
}

// ValidateMFACode 验证MFA码（登录时使用）
func (uc *UseCase) ValidateMFACode(ctx context.Context, userID uint, code, ipAddress, userAgent string) (bool, error) {
	mfa, err := uc.repo.GetUserMFA(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("获取MFA配置失败: %w", err)
	}
	if mfa == nil || !mfa.IsEnabled() {
		return false, fmt.Errorf("MFA未启用")
	}

	// 解密密钥
	secret, err := uc.decrypt(mfa.TOTPSecret)
	if err != nil {
		return false, fmt.Errorf("解密密钥失败: %w", err)
	}

	// 验证TOTP码
	valid := ValidateTOTP(secret, code)

	// 如果TOTP验证失败，尝试备用码
	if !valid {
		valid = uc.validateBackupCode(ctx, mfa, code)
	}

	// 记录日志
	log := &MFALog{
		UserID:    userID,
		MFAType:   MFATypeTOTP,
		Action:    "verify",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   valid,
	}
	if valid {
		log.Message = "MFA验证成功"
	} else {
		log.Message = "MFA验证失败"
	}
	uc.repo.CreateLog(ctx, log)

	return valid, nil
}

// validateBackupCode 验证备用码
func (uc *UseCase) validateBackupCode(ctx context.Context, mfa *UserMFA, code string) bool {
	// 解密备用码
	backupCodesJSON, err := uc.decrypt(mfa.BackupCodes)
	if err != nil {
		return false
	}

	var backupCodes []BackupCode
	if err := json.Unmarshal([]byte(backupCodesJSON), &backupCodes); err != nil {
		return false
	}

	// 查找匹配的备用码
	for i, bc := range backupCodes {
		if bc.Code == code && !bc.Used {
			// 标记为已使用
			backupCodes[i].Used = true
			now := time.Now()
			backupCodes[i].UsedAt = &now

			// 更新数据库
			updatedJSON, _ := json.Marshal(backupCodes)
			encryptedBackupCodes, err := uc.encrypt(string(updatedJSON))
			if err == nil {
				mfa.BackupCodes = encryptedBackupCodes
				uc.repo.UpdateMFA(ctx, mfa)
			}

			return true
		}
	}
	return false
}

// GetMFAStatus 获取MFA状态
func (uc *UseCase) GetMFAStatus(ctx context.Context, userID uint) (*UserMFA, error) {
	mfa, err := uc.repo.GetUserMFA(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mfa, nil
}

// DisableMFA 禁用MFA
func (uc *UseCase) DisableMFA(ctx context.Context, userID uint, code, ipAddress, userAgent string) (bool, error) {
	// 先验证MFA码
	valid, err := uc.ValidateMFACode(ctx, userID, code, ipAddress, userAgent)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, nil
	}

	// 禁用MFA
	mfa, _ := uc.repo.GetUserMFA(ctx, userID)
	if mfa != nil {
		mfa.TOTPEnabled = false
		mfa.TOTPVerified = false
		uc.repo.UpdateMFA(ctx, mfa)
	}

	// 清除所有信任设备
	_ = uc.repo.DeleteUserTrustedDevices(ctx, userID)

	// 记录日志
	uc.repo.CreateLog(ctx, &MFALog{
		UserID:    userID,
		MFAType:   MFATypeTOTP,
		Action:    "disable",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		Message:   "MFA已禁用",
	})

	appLogger.Info("MFA已禁用", zap.Uint("userID", userID))
	return true, nil
}

// RegenerateBackupCodes 重新生成备用码
func (uc *UseCase) RegenerateBackupCodes(ctx context.Context, userID uint, code, ipAddress, userAgent string) ([]string, error) {
	// 先验证MFA码
	valid, err := uc.ValidateMFACode(ctx, userID, code, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("验证码错误")
	}

	// 生成新备用码
	backupCodes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, err
	}

	// 加密并保存
	mfa, _ := uc.repo.GetUserMFA(ctx, userID)
	if mfa != nil {
		backupCodesJSON, _ := json.Marshal(backupCodes)
		encryptedBackupCodes, err := uc.encrypt(string(backupCodesJSON))
		if err != nil {
			return nil, err
		}
		mfa.BackupCodes = encryptedBackupCodes
		uc.repo.UpdateMFA(ctx, mfa)
	}

	return backupCodes, nil
}

// encrypt AES-GCM加密
func (uc *UseCase) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(uc.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("%x", ciphertext), nil
}

// decrypt AES-GCM解密
func (uc *UseCase) decrypt(ciphertext string) (string, error) {
	block, err := aes.NewCipher(uc.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	data := []byte(ciphertext)
	if len(data) < nonceSize*2 {
		return "", fmt.Errorf("密文太短")
	}

	// 解析十六进制
	nonce := make([]byte, nonceSize)
	ciphertextBytes := make([]byte, (len(data)-nonceSize*2)/2)

	for i := 0; i < nonceSize; i++ {
		_, err := fmt.Sscanf(string(data[i*2:(i+1)*2]), "%02x", &nonce[i])
		if err != nil {
			return "", err
		}
	}

	for i := 0; i < len(ciphertextBytes); i++ {
		_, err := fmt.Sscanf(string(data[(nonceSize+i)*2:(nonceSize+i+1)*2]), "%02x", &ciphertextBytes[i])
		if err != nil {
			return "", err
		}
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsMFAEnabled 检查用户是否启用了MFA
func (uc *UseCase) IsMFAEnabled(ctx context.Context, userID uint) bool {
	mfa, err := uc.repo.GetUserMFA(ctx, userID)
	if err != nil || mfa == nil {
		return false
	}
	return mfa.IsEnabled()
}

// GenerateDeviceToken 生成设备信任令牌
func (uc *UseCase) GenerateDeviceToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SaveTrustedDevice 保存信任设备
func (uc *UseCase) SaveTrustedDevice(ctx context.Context, userID uint, deviceName, ipAddress string, durationSeconds int) (string, error) {
	token, err := uc.GenerateDeviceToken()
	if err != nil {
		return "", fmt.Errorf("生成设备令牌失败: %w", err)
	}

	// 对token做hash存储，避免明文存储
	hashedToken := hashDeviceToken(token)

	device := &TrustedDevice{
		UserID:         userID,
		DeviceToken:    hashedToken,
		DeviceName:     deviceName,
		IPAddress:      ipAddress,
		LastVerifiedAt: time.Now(),
		ExpiresAt:      time.Now().Add(time.Duration(durationSeconds) * time.Second),
	}

	if err := uc.repo.SaveTrustedDevice(ctx, device); err != nil {
		return "", fmt.Errorf("保存信任设备失败: %w", err)
	}

	// 异步清理过期设备
	go func() {
		_ = uc.repo.DeleteExpiredTrustedDevices(context.Background())
	}()

	appLogger.Info("已保存MFA信任设备", zap.Uint("userID", userID), zap.String("deviceName", deviceName))
	return token, nil
}

// IsTrustedDevice 检查是否为信任设备
func (uc *UseCase) IsTrustedDevice(ctx context.Context, userID uint, deviceToken string) bool {
	if deviceToken == "" {
		return false
	}

	hashedToken := hashDeviceToken(deviceToken)
	device, err := uc.repo.GetTrustedDevice(ctx, hashedToken)
	if err != nil || device == nil {
		return false
	}

	// 检查是否属于该用户且未过期
	if device.UserID != userID || !device.IsValid() {
		return false
	}

	return true
}

// DeleteUserTrustedDevices 删除用户所有信任设备（禁用MFA时调用）
func (uc *UseCase) DeleteUserTrustedDevices(ctx context.Context, userID uint) error {
	return uc.repo.DeleteUserTrustedDevices(ctx, userID)
}

// hashDeviceToken 对设备令牌做SHA256哈希
func hashDeviceToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
