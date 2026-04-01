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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/system"
	mfaservice "github.com/ydcloud-dy/opshub/internal/service/mfa"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
)

// HTTPServer MFA HTTP服务
type HTTPServer struct {
	svc           *mfaservice.Service
	configUseCase *system.ConfigUseCase
}

// NewHTTPServer 创建HTTP服务
func NewHTTPServer(svc *mfaservice.Service) *HTTPServer {
	return &HTTPServer{
		svc: svc,
	}
}

// SetConfigUseCase 设置配置用例（通过依赖注入）
func (s *HTTPServer) SetConfigUseCase(configUseCase *system.ConfigUseCase) {
	s.configUseCase = configUseCase
}

// RegisterRoutes 注册需要认证的路由
func (s *HTTPServer) RegisterRoutes(r *gin.RouterGroup) {
	mfaGroup := r.Group("/mfa")
	{
		mfaGroup.GET("/status", s.GetStatus)
		mfaGroup.POST("/setup", s.SetupMFA)
		mfaGroup.POST("/enable", s.EnableMFA)
		mfaGroup.POST("/disable", s.DisableMFA)
		mfaGroup.POST("/regenerate-backup", s.RegenerateBackup)
	}
}

// RegisterPublicRoutes 注册公开路由（MFA登录验证）
func (s *HTTPServer) RegisterPublicRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth/mfa")
	{
		auth.POST("/login", s.MFALogin)
	}
}

// GetStatus 获取MFA状态
// @Summary 获取MFA状态
// @Description 获取当前用户的MFA配置状态
// @Tags MFA
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} mfa.MFAStatusResponse
// @Router /api/v1/mfa/status [get]
func (s *HTTPServer) GetStatus(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未授权"})
		return
	}

	status, err := s.svc.GetStatus(c.Request.Context(), userID)
	if err != nil {
		appLogger.Error("获取MFA状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取MFA状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// SetupMFA 设置MFA（生成二维码和密钥）
// @Summary 设置MFA
// @Description 生成MFA二维码和密钥
// @Tags MFA
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} mfa.MFASetupResponse
// @Router /api/v1/mfa/setup [post]
func (s *HTTPServer) SetupMFA(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未授权"})
		return
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	setupData, err := s.svc.SetupMFA(c.Request.Context(), userID, usernameStr)
	if err != nil {
		appLogger.Error("设置MFA失败", zap.Error(err), zap.Uint("userID", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "设置MFA失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": setupData})
}

// EnableMFA 启用MFA
// @Summary 启用MFA
// @Description 启用当前用户的多因素认证
// @Tags MFA
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.EnableRequest true "启用请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/mfa/enable [post]
func (s *HTTPServer) EnableMFA(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未授权"})
		return
	}

	var req mfaservice.EnableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误: " + err.Error()})
		return
	}

	ip := mfaservice.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	if err := s.svc.EnableMFA(c.Request.Context(), userID, req.Code, ip, userAgent); err != nil {
		appLogger.Error("启用MFA失败", zap.Error(err), zap.Uint("userID", userID))
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "MFA已启用"})
}

// DisableMFA 禁用MFA
// @Summary 禁用MFA
// @Description 禁用当前用户的多因素认证
// @Tags MFA
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.DisableRequest true "禁用请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/mfa/disable [post]
func (s *HTTPServer) DisableMFA(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未授权"})
		return
	}

	var req mfaservice.DisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误: " + err.Error()})
		return
	}

	ip := mfaservice.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	if err := s.svc.DisableMFA(c.Request.Context(), userID, req.Code, ip, userAgent); err != nil {
		appLogger.Error("禁用MFA失败", zap.Error(err), zap.Uint("userID", userID))
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "MFA已禁用"})
}

// RegenerateBackup 重新生成备用码
// @Summary 重新生成备用码
// @Description 重新生成MFA备用码
// @Tags MFA
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.RegenerateBackupRequest true "请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/mfa/regenerate-backup [post]
func (s *HTTPServer) RegenerateBackup(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未授权"})
		return
	}

	var req mfaservice.RegenerateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误: " + err.Error()})
		return
	}

	ip := mfaservice.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	codes, err := s.svc.RegenerateBackupCodes(c.Request.Context(), userID, req.Code, ip, userAgent)
	if err != nil {
		appLogger.Error("重新生成备用码失败", zap.Error(err), zap.Uint("userID", userID))
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"message": "备用码已重新生成",
		"data": gin.H{"backupCodes": codes},
	})
}

// MFALogin MFA登录
// @Summary MFA登录
// @Description 使用MFA验证码完成登录
// @Tags MFA
// @Accept json
// @Produce json
// @Param request body service.MFALoginRequest true "登录请求"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/public/auth/mfa/login [post]
func (s *HTTPServer) MFALogin(c *gin.Context) {
	var req mfaservice.MFALoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误: " + err.Error()})
		return
	}

	ip := mfaservice.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	// 获取MFA记住设备时长配置
	skipDuration := 0
	if req.RememberDevice && s.configUseCase != nil {
		securityConfig, err := s.configUseCase.GetSecurityConfig(c.Request.Context())
		if err == nil {
			skipDuration = securityConfig.MFASkipDuration
		}
	}

	result, err := s.svc.MFALogin(c.Request.Context(), req.MFAToken, req.Code, ip, userAgent, req.RememberDevice, skipDuration)
	if err != nil {
		appLogger.Error("MFA登录失败", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
		return
	}

	// 如果有设备令牌，通过httpOnly cookie设置
	if result.DeviceToken != "" && skipDuration > 0 {
		c.SetCookie(
			"mfa_trusted_device",
			result.DeviceToken,
			skipDuration,
			"/",
			"",
			false,
			true,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"token": result.Token,
			"user":  result.User,
		},
	})
}
