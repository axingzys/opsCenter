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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

const (
	// TOTPDigits TOTP码位数
	TOTPDigits = 6
	// TOTPInterval TOTP时间间隔（秒）
	TOTPInterval = 30
	// TOTPSkew 允许的时间窗口偏移
	TOTPSkew = 1
	// SecretLength 密钥长度（字节）
	SecretLength = 20
)

// GenerateSecret 生成随机密钥
func GenerateSecret() (string, error) {
	bytes := make([]byte, SecretLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}
	// Base32编码，移除填充符
	secret := base32.StdEncoding.EncodeToString(bytes)
	return strings.TrimRight(secret, "="), nil
}

// GenerateQRCodeURL 生成OTP Auth URL（用于二维码）
func GenerateQRCodeURL(secret, issuer, accountName string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		url.QueryEscape(issuer),
		url.QueryEscape(accountName),
		secret,
		url.QueryEscape(issuer),
		TOTPDigits,
		TOTPInterval,
	)
}

// GenerateQRCodeImage 生成二维码图片（返回base64编码的PNG图片）
func GenerateQRCodeImage(secret, issuer, accountName string) (string, error) {
	// 生成OTP Auth URL
	otpURL := GenerateQRCodeURL(secret, issuer, accountName)

	// 生成二维码图片（256x256像素）
	qr, err := qrcode.New(otpURL, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}

	// 设置二维码大小
	qr.DisableBorder = false

	// 生成PNG图片字节
	pngBytes, err := qr.PNG(256)
	if err != nil {
		return "", fmt.Errorf("生成PNG图片失败: %w", err)
	}

	// 转换为base64编码的Data URL
	base64Str := base64.StdEncoding.EncodeToString(pngBytes)
	return fmt.Sprintf("data:image/png;base64,%s", base64Str), nil
}

// ValidateTOTP 验证TOTP码
func ValidateTOTP(secret, code string) bool {
	if len(code) != TOTPDigits {
		return false
	}

	// 解码密钥（添加填充符）
	secret = strings.ToUpper(secret)
	secret = strings.TrimRight(secret, "=")
	if pad := 8 - len(secret)%8; pad != 8 {
		secret += strings.Repeat("=", pad)
	}

	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false
	}

	// 获取当前时间窗口
	now := time.Now().UTC().Unix() / TOTPInterval

	// 允许时间偏移（前后各TOTPSkew个窗口）
	for i := -TOTPSkew; i <= TOTPSkew; i++ {
		if generateTOTP(key, now+int64(i)) == code {
			return true
		}
	}
	return false
}

// generateTOTP 生成TOTP码
func generateTOTP(key []byte, counter int64) string {
	// 将counter转换为大端序字节
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	// HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(buf)
	sum := h.Sum(nil)

	// 动态截断
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code = code % uint32(math.Pow10(TOTPDigits))

	return fmt.Sprintf("%06d", code)
}

// GenerateBackupCodes 生成备用码
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		code, err := generateBackupCode()
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// generateBackupCode 生成单个备用码（8位字母数字）
func generateBackupCode() (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = chars[int(bytes[i])%len(chars)]
	}
	return string(bytes), nil
}
