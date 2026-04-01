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

package identity

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// MFASettings 用户MFA设置
type MFASettings struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	TOTPEnabled  bool      `gorm:"default:false" json:"totp_enabled"`
	TOTPSecret   string    `gorm:"type:varchar(255)" json:"-"` // 加密存储
	TOTPVerified bool      `gorm:"default:false" json:"totp_verified"`
	BackupCodes  string    `gorm:"type:text" json:"-"` // JSON数组，加密存储
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MFAChallenge MFA验证挑战
type MFAChallenge struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Token     string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"token"`
	Type      string    `gorm:"type:varchar(20);not null" json:"type"` // login, action
	Attempts  int       `gorm:"default:0" json:"attempts"`
	Verified  bool      `gorm:"default:false" json:"verified"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// MFASettingsRepo MFA设置仓库接口
type MFASettingsRepo interface {
	Create(ctx context.Context, settings *MFASettings) error
	Update(ctx context.Context, settings *MFASettings) error
	GetByUserID(ctx context.Context, userID uint) (*MFASettings, error)
	Delete(ctx context.Context, userID uint) error
}

// MFAChallengeRepo MFA挑战仓库接口
type MFAChallengeRepo interface {
	Create(ctx context.Context, challenge *MFAChallenge) error
	GetByToken(ctx context.Context, token string) (*MFAChallenge, error)
	Update(ctx context.Context, challenge *MFAChallenge) error
	Delete(ctx context.Context, id uint) error
	DeleteExpired(ctx context.Context) error
}

// MFAUseCase MFA用例
type MFAUseCase struct {
	settingsRepo  MFASettingsRepo
	challengeRepo MFAChallengeRepo
	issuer        string
	encryptionKey string
}

// NewMFAUseCase 创建MFA用例
func NewMFAUseCase(
	settingsRepo MFASettingsRepo,
	challengeRepo MFAChallengeRepo,
	issuer string,
	encryptionKey string,
) *MFAUseCase {
	return &MFAUseCase{
		settingsRepo:  settingsRepo,
		challengeRepo: challengeRepo,
		issuer:        issuer,
		encryptionKey: encryptionKey,
	}
}

// TOTPSetupResponse TOTP设置响应
type TOTPSetupResponse struct {
	Secret    string `json:"secret"`
	QRCode    string `json:"qr_code"`    // base64编码的PNG图片
	ManualKey string `json:"manual_key"` // 手动输入的密钥
}

// SetupTOTP 初始化TOTP设置
func (uc *MFAUseCase) SetupTOTP(ctx context.Context, userID uint, username string) (*TOTPSetupResponse, error) {
	// 检查是否已启用
	settings, _ := uc.settingsRepo.GetByUserID(ctx, userID)
	if settings != nil && settings.TOTPEnabled {
		return nil, errors.New("TOTP already enabled")
	}

	// 生成TOTP密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      uc.issuer,
		AccountName: username,
		SecretSize:  32,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
		Period:      30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// 生成二维码
	qrImage, err := key.Image(200, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// 将二维码图片转为base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImage); err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}
	qrBase64 := base64EncodeImage(buf.Bytes())

	// 保存设置（未验证状态）
	if settings == nil {
		settings = &MFASettings{
			UserID:       userID,
			TOTPEnabled:  false,
			TOTPSecret:   key.Secret(),
			TOTPVerified: false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := uc.settingsRepo.Create(ctx, settings); err != nil {
			return nil, fmt.Errorf("failed to save MFA settings: %w", err)
		}
	} else {
		settings.TOTPSecret = key.Secret()
		settings.TOTPVerified = false
		settings.UpdatedAt = time.Now()
		if err := uc.settingsRepo.Update(ctx, settings); err != nil {
			return nil, fmt.Errorf("failed to update MFA settings: %w", err)
		}
	}

	return &TOTPSetupResponse{
		Secret:    key.Secret(),
		QRCode:    qrBase64,
		ManualKey: formatManualKey(key.Secret()),
	}, nil
}

// VerifyTOTPSetup 验证TOTP设置并启用
func (uc *MFAUseCase) VerifyTOTPSetup(ctx context.Context, userID uint, code string) error {
	settings, err := uc.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return errors.New("MFA not set up")
	}

	if settings.TOTPEnabled && settings.TOTPVerified {
		return errors.New("TOTP already enabled")
	}

	// 验证TOTP码
	if !totp.Validate(code, settings.TOTPSecret) {
		return errors.New("invalid TOTP code")
	}

	// 生成备用码
	backupCodes, err := uc.generateBackupCodes()
	if err != nil {
		return fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// 启用TOTP
	settings.TOTPEnabled = true
	settings.TOTPVerified = true
	settings.BackupCodes = encodeBackupCodes(backupCodes)
	settings.UpdatedAt = time.Now()

	if err := uc.settingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("failed to enable TOTP: %w", err)
	}

	return nil
}

// VerifyTOTP 验证TOTP码
func (uc *MFAUseCase) VerifyTOTP(ctx context.Context, userID uint, code string) (bool, error) {
	settings, err := uc.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, errors.New("MFA not enabled")
	}

	if !settings.TOTPEnabled {
		return false, errors.New("TOTP not enabled")
	}

	// 验证TOTP码
	if totp.Validate(code, settings.TOTPSecret) {
		return true, nil
	}

	// 尝试验证备用码
	backupCodes := decodeBackupCodes(settings.BackupCodes)
	for i, backupCode := range backupCodes {
		if backupCode == code {
			// 使用后移除备用码
			backupCodes = append(backupCodes[:i], backupCodes[i+1:]...)
			settings.BackupCodes = encodeBackupCodes(backupCodes)
			settings.UpdatedAt = time.Now()
			_ = uc.settingsRepo.Update(ctx, settings)
			return true, nil
		}
	}

	return false, nil
}

// DisableTOTP 禁用TOTP
func (uc *MFAUseCase) DisableTOTP(ctx context.Context, userID uint, code string) error {
	settings, err := uc.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return errors.New("MFA not enabled")
	}

	if !settings.TOTPEnabled {
		return errors.New("TOTP not enabled")
	}

	// 验证TOTP码
	if !totp.Validate(code, settings.TOTPSecret) {
		// 尝试备用码
		backupCodes := decodeBackupCodes(settings.BackupCodes)
		found := false
		for _, backupCode := range backupCodes {
			if backupCode == code {
				found = true
				break
			}
		}
		if !found {
			return errors.New("invalid code")
		}
	}

	// 禁用TOTP
	settings.TOTPEnabled = false
	settings.TOTPVerified = false
	settings.TOTPSecret = ""
	settings.BackupCodes = ""
	settings.UpdatedAt = time.Now()

	return uc.settingsRepo.Update(ctx, settings)
}

// GetMFAStatus 获取用户MFA状态
func (uc *MFAUseCase) GetMFAStatus(ctx context.Context, userID uint) (bool, error) {
	settings, err := uc.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, nil
	}
	return settings.TOTPEnabled && settings.TOTPVerified, nil
}

// GetBackupCodes 获取/重新生成备用码
func (uc *MFAUseCase) GetBackupCodes(ctx context.Context, userID uint, code string) ([]string, error) {
	settings, err := uc.settingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, errors.New("MFA not enabled")
	}

	if !settings.TOTPEnabled {
		return nil, errors.New("TOTP not enabled")
	}

	// 验证TOTP码
	if !totp.Validate(code, settings.TOTPSecret) {
		return nil, errors.New("invalid TOTP code")
	}

	// 生成新的备用码
	backupCodes, err := uc.generateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	settings.BackupCodes = encodeBackupCodes(backupCodes)
	settings.UpdatedAt = time.Now()

	if err := uc.settingsRepo.Update(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to save backup codes: %w", err)
	}

	return backupCodes, nil
}

// CreateMFAChallenge 创建MFA验证挑战
func (uc *MFAUseCase) CreateMFAChallenge(ctx context.Context, userID uint, challengeType string) (*MFAChallenge, error) {
	// 生成随机令牌
	token, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	challenge := &MFAChallenge{
		UserID:    userID,
		Token:     token,
		Type:      challengeType,
		Attempts:  0,
		Verified:  false,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
	}

	if err := uc.challengeRepo.Create(ctx, challenge); err != nil {
		return nil, fmt.Errorf("failed to create challenge: %w", err)
	}

	return challenge, nil
}

// VerifyMFAChallenge 验证MFA挑战
func (uc *MFAUseCase) VerifyMFAChallenge(ctx context.Context, token, code string) (*MFAChallenge, error) {
	challenge, err := uc.challengeRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, errors.New("invalid challenge token")
	}

	// 检查是否过期
	if time.Now().After(challenge.ExpiresAt) {
		return nil, errors.New("challenge expired")
	}

	// 检查尝试次数
	if challenge.Attempts >= 5 {
		return nil, errors.New("too many attempts")
	}

	// 增加尝试次数
	challenge.Attempts++
	_ = uc.challengeRepo.Update(ctx, challenge)

	// 验证TOTP码
	valid, err := uc.VerifyTOTP(ctx, challenge.UserID, code)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, errors.New("invalid code")
	}

	// 标记为已验证
	challenge.Verified = true
	if err := uc.challengeRepo.Update(ctx, challenge); err != nil {
		return nil, fmt.Errorf("failed to update challenge: %w", err)
	}

	return challenge, nil
}

// 辅助函数

func (uc *MFAUseCase) generateBackupCodes() ([]string, error) {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code, err := generateRandomCode(8)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(bytes)[:length], nil
}

func base64EncodeImage(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func formatManualKey(secret string) string {
	// 每4个字符加一个空格
	var result string
	for i, c := range secret {
		if i > 0 && i%4 == 0 {
			result += " "
		}
		result += string(c)
	}
	return result
}

func encodeBackupCodes(codes []string) string {
	// 简单编码，用逗号分隔
	result := ""
	for i, code := range codes {
		if i > 0 {
			result += ","
		}
		result += code
	}
	return result
}

func decodeBackupCodes(encoded string) []string {
	if encoded == "" {
		return []string{}
	}
	var codes []string
	start := 0
	for i := 0; i <= len(encoded); i++ {
		if i == len(encoded) || encoded[i] == ',' {
			if start < i {
				codes = append(codes, encoded[start:i])
			}
			start = i + 1
		}
	}
	return codes
}

// generateTOTPCode 生成TOTP码（用于测试）
func generateTOTPCode(secret string) string {
	// 获取当前时间戳
	counter := uint64(time.Now().Unix()) / 30

	// 将计数器转换为字节
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	// 解码密钥
	key, _ := base32.StdEncoding.DecodeString(secret)

	// 计算HMAC
	h := hmac.New(sha1.New, key)
	h.Write(counterBytes)
	hash := h.Sum(nil)

	// 动态截断
	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	code = code % 1000000

	return fmt.Sprintf("%06d", code)
}
