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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/mfa"
	"github.com/ydcloud-dy/opshub/internal/biz/rbac"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
)

// Service MFA服务
type Service struct {
	useCase     *mfa.UseCase
	authService *rbacservice.AuthService
	userUseCase *rbac.UserUseCase
}

// NewService 创建MFA服务
func NewService(useCase *mfa.UseCase, authService *rbacservice.AuthService, userUseCase *rbac.UserUseCase) *Service {
	return &Service{
		useCase:     useCase,
		authService: authService,
		userUseCase: userUseCase,
	}
}

// SetupRequest MFA设置请求
type SetupRequest struct {
	Username string `json:"username" binding:"required"`
}

// VerifySetupRequest 验证设置请求
type VerifySetupRequest struct {
	Username string `json:"username" binding:"required"`
	Code     string `json:"code" binding:"required,len=6"`
}

// MFALoginRequest MFA登录请求
type MFALoginRequest struct {
	MFAToken       string `json:"mfaToken" binding:"required"`
	Code           string `json:"code" binding:"required"`
	RememberDevice bool   `json:"rememberDevice"`
}

// EnableRequest 启用MFA请求
type EnableRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// DisableRequest 禁用MFA请求
type DisableRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// ValidateRequest 验证码验证请求
type ValidateRequest struct {
	Code string `json:"code" binding:"required"`
}

// RegenerateBackupRequest 重新生成备用码请求
type RegenerateBackupRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// getClientIP 获取客户端IP
func getClientIP(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		ip = c.GetHeader("X-Real-IP")
	}
	if ip == "" {
		ip = c.GetHeader("X-Forwarded-For")
		if ip != "" {
			// 取第一个IP
			for i, r := range ip {
				if r == ',' {
					ip = ip[:i]
					break
				}
			}
		}
	}
	return ip
}

// GetClientIP 导出获取客户端IP函数
func GetClientIP(c *gin.Context) string {
	return getClientIP(c)
}

// MFAStatusResponse MFA状态响应
type MFAStatusResponse struct {
	IsEnabled  bool   `json:"isEnabled"`
	MFAType    string `json:"mfaType"`
	HasBackup  bool   `json:"hasBackup"`
	VerifiedAt string `json:"verifiedAt,omitempty"`
}

// GetStatus 获取MFA状态
func (s *Service) GetStatus(ctx context.Context, userID uint) (*MFAStatusResponse, error) {
	mfa, err := s.useCase.GetMFAStatus(ctx, userID)
	if err != nil {
		return nil, err
	}

	if mfa == nil {
		return &MFAStatusResponse{IsEnabled: false}, nil
	}

	return &MFAStatusResponse{
		IsEnabled:  mfa.IsEnabled(),
		MFAType:    "totp",
		HasBackup:  mfa.BackupCodes != "",
		VerifiedAt: "",
	}, nil
}

// EnableMFA 启用MFA
func (s *Service) EnableMFA(ctx context.Context, userID uint, code, ipAddress, userAgent string) error {
	_, err := s.useCase.VerifyAndEnableMFA(ctx, userID, code, ipAddress, userAgent)
	return err
}

// DisableMFA 禁用MFA
func (s *Service) DisableMFA(ctx context.Context, userID uint, code, ipAddress, userAgent string) error {
	_, err := s.useCase.DisableMFA(ctx, userID, code, ipAddress, userAgent)
	return err
}

// RegenerateBackupCodes 重新生成备用码
func (s *Service) RegenerateBackupCodes(ctx context.Context, userID uint, code, ipAddress, userAgent string) ([]string, error) {
	return s.useCase.RegenerateBackupCodes(ctx, userID, code, ipAddress, userAgent)
}

// MFALoginResponse MFA登录响应
type MFALoginResponse struct {
	Token       string      `json:"token"`
	User        interface{} `json:"user"`
	DeviceToken string      `json:"deviceToken,omitempty"` // 信任设备令牌
}

// MFALogin MFA登录验证
func (s *Service) MFALogin(ctx context.Context, mfaToken, code, ipAddress, userAgent string, rememberDevice bool, skipDuration int) (*MFALoginResponse, error) {
	// 1. 解析mfaToken获取userID
	claims, err := s.authService.ParseMFAToken(mfaToken)
	if err != nil {
		return nil, fmt.Errorf("无效的MFA Token: %w", err)
	}

	// 2. 验证MFA码
	valid, err := s.useCase.ValidateMFACode(ctx, claims.UserID, code, ipAddress, userAgent)
	if err != nil {
		return nil, fmt.Errorf("MFA验证失败: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("验证码错误")
	}

	// 3. 生成正式JWT token
	token, err := s.authService.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %w", err)
	}

	// 4. 获取用户信息
	user, err := s.userUseCase.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	resp := &MFALoginResponse{
		Token: token,
		User:  user,
	}

	// 5. 如果选择记住设备且配置了跳过时长
	if rememberDevice && skipDuration > 0 {
		deviceToken, err := s.useCase.SaveTrustedDevice(ctx, claims.UserID, userAgent, ipAddress, skipDuration)
		if err != nil {
			// 保存失败不影响登录，只记录日志
			fmt.Printf("保存信任设备失败: %v\n", err)
		} else {
			resp.DeviceToken = deviceToken
		}
	}

	return resp, nil
}

// SetupMFA 设置MFA
func (s *Service) SetupMFA(ctx context.Context, userID uint, username string) (*mfa.MFASetupResponse, error) {
	return s.useCase.SetupMFA(ctx, userID, username)
}
