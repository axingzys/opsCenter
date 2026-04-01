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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/identity"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

// MFAService MFA服务
type MFAService struct {
	useCase *identity.MFAUseCase
}

// NewMFAService 创建MFA服务
func NewMFAService(useCase *identity.MFAUseCase) *MFAService {
	return &MFAService{useCase: useCase}
}

// SetupTOTP 初始化TOTP设置
func (s *MFAService) SetupTOTP(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	username, _ := c.Get("username")
	usernameStr := ""
	if username != nil {
		usernameStr = username.(string)
	}

	result, err := s.useCase.SetupTOTP(c.Request.Context(), userID.(uint), usernameStr)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, result)
}

// VerifyTOTP 验证TOTP并启用
func (s *MFAService) VerifyTOTP(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := s.useCase.VerifyTOTPSetup(c.Request.Context(), userID.(uint), req.Code); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "TOTP已启用"})
}

// EnableTOTP 启用TOTP（验证后启用）
func (s *MFAService) EnableTOTP(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := s.useCase.VerifyTOTPSetup(c.Request.Context(), userID.(uint), req.Code); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "TOTP已启用"})
}

// DisableTOTP 禁用TOTP
func (s *MFAService) DisableTOTP(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := s.useCase.DisableTOTP(c.Request.Context(), userID.(uint), req.Code); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "TOTP已禁用"})
}

// GetStatus 获取MFA状态
func (s *MFAService) GetStatus(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	enabled, err := s.useCase.GetMFAStatus(c.Request.Context(), userID.(uint))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{
		"totp_enabled": enabled,
	})
}

// GetBackupCodes 获取备用码
func (s *MFAService) GetBackupCodes(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.ErrorCode(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	codes, err := s.useCase.GetBackupCodes(c.Request.Context(), userID.(uint), req.Code)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{
		"backup_codes": codes,
	})
}

// VerifyMFALogin 验证MFA登录
func (s *MFAService) VerifyMFALogin(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	challenge, err := s.useCase.VerifyMFAChallenge(c.Request.Context(), req.Token, req.Code)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	// 返回已验证的用户ID供登录流程使用
	response.Success(c, gin.H{
		"verified": true,
		"userId":   challenge.UserID,
	})
}

// CreateChallenge 创建MFA挑战（登录时调用）
func (s *MFAService) CreateChallenge(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	// 检查用户是否启用了MFA
	enabled, err := s.useCase.GetMFAStatus(c.Request.Context(), req.UserID)
	if err != nil || !enabled {
		response.Success(c, gin.H{
			"mfa_required": false,
		})
		return
	}

	challenge, err := s.useCase.CreateMFAChallenge(c.Request.Context(), req.UserID, "login")
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{
		"mfa_required": true,
		"token":        challenge.Token,
		"expires_at":   challenge.ExpiresAt,
	})
}
