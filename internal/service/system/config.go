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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/ydcloud-dy/opshub/internal/biz/system"
	"github.com/ydcloud-dy/opshub/internal/conf"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

// ConfigService 系统配置服务
type ConfigService struct {
	configUseCase *system.ConfigUseCase
	uploadDir     string
}

const defaultPrometheusRetentionDays = 15

// NewConfigService 创建系统配置服务
func NewConfigService(configUseCase *system.ConfigUseCase, uploadDir string) *ConfigService {
	return &ConfigService{
		configUseCase: configUseCase,
		uploadDir:     uploadDir,
	}
}

// GetAllConfig 获取所有配置
// @Summary 获取所有系统配置
// @Description 获取基础配置和安全配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config [get]
func (s *ConfigService) GetAllConfig(c *gin.Context) {
	config, err := s.configUseCase.GetAllConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取配置失败: "+err.Error())
		return
	}
	response.Success(c, config)
}

// GetBasicConfig 获取基础配置
// @Summary 获取基础配置
// @Description 获取系统名称、Logo、描述等基础配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config/basic [get]
func (s *ConfigService) GetBasicConfig(c *gin.Context) {
	config, err := s.configUseCase.GetBasicConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取配置失败: "+err.Error())
		return
	}
	response.Success(c, config)
}

// SaveBasicConfigRequest 保存基础配置请求
type SaveBasicConfigRequest struct {
	SystemName        string `json:"systemName"`
	SystemLogo        string `json:"systemLogo"`
	SystemDescription string `json:"systemDescription"`
}

// SaveBasicConfig 保存基础配置
// @Summary 保存基础配置
// @Description 保存系统名称、Logo、描述等基础配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body SaveBasicConfigRequest true "基础配置"
// @Success 200 {object} response.Response "保存成功"
// @Router /api/v1/system/config/basic [put]
func (s *ConfigService) SaveBasicConfig(c *gin.Context) {
	var req SaveBasicConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	config := &system.BasicConfig{
		SystemName:        req.SystemName,
		SystemLogo:        req.SystemLogo,
		SystemDescription: req.SystemDescription,
	}

	if err := s.configUseCase.SaveBasicConfig(c.Request.Context(), config); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "保存成功", nil)
}

// GetSecurityConfig 获取安全配置
// @Summary 获取安全配置
// @Description 获取密码策略、登录安全等配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config/security [get]
func (s *ConfigService) GetSecurityConfig(c *gin.Context) {
	config, err := s.configUseCase.GetSecurityConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取配置失败: "+err.Error())
		return
	}
	response.Success(c, config)
}

// GetMonitoringConfig 获取监控配置
// @Summary 获取监控配置
// @Description 获取 Prometheus 监控数据保留相关配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config/monitoring [get]
func (s *ConfigService) GetMonitoringConfig(c *gin.Context) {
	config, err := s.configUseCase.GetMonitoringConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取监控配置失败: "+err.Error())
		return
	}

	config.PrometheusBaseURL = s.getPrometheusBaseURL()
	config.CurrentPrometheusRetention = s.getCurrentPrometheusRetention(c.Request.Context())
	response.Success(c, config)
}

// GetDatabaseConfig 获取数据库配置
// @Summary 获取数据库配置
// @Description 获取数据库写开关、确认策略和备份默认参数
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config/database [get]
func (s *ConfigService) GetDatabaseConfig(c *gin.Context) {
	config, err := s.configUseCase.GetDatabaseConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取数据库配置失败: "+err.Error())
		return
	}
	response.Success(c, config)
}

// SaveSecurityConfigRequest 保存安全配置请求
type SaveSecurityConfigRequest struct {
	PasswordMinLength int  `json:"passwordMinLength"`
	SessionTimeout    int  `json:"sessionTimeout"`
	EnableCaptcha     bool `json:"enableCaptcha"`
	MaxLoginAttempts  int  `json:"maxLoginAttempts"`
	LockoutDuration   int  `json:"lockoutDuration"`
	// MFA配置
	MFAEnabled      bool   `json:"mfaEnabled"`
	MFAEnforced     bool   `json:"mfaEnforced"`
	MFAType         string `json:"mfaType"`
	MFASkipDuration int    `json:"mfaSkipDuration"`
}

// SaveMonitoringConfigRequest 保存监控配置请求
type SaveMonitoringConfigRequest struct {
	PrometheusRetentionDays int `json:"prometheusRetentionDays"`
}

// SaveDatabaseConfigRequest 保存数据库配置请求
type SaveDatabaseConfigRequest struct {
	WriteEnabled               bool   `json:"writeEnabled"`
	HighRiskRequiresConfirm    bool   `json:"highRiskRequiresConfirm"`
	OperationReasonRequired    bool   `json:"operationReasonRequired"`
	MaxAffectedRows            int    `json:"maxAffectedRows"`
	DefaultBackupRetentionDays int    `json:"defaultBackupRetentionDays"`
	BackupStoragePath          string `json:"backupStoragePath"`
}

// SaveSecurityConfig 保存安全配置
// @Summary 保存安全配置
// @Description 保存密码策略、登录安全等配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body SaveSecurityConfigRequest true "安全配置"
// @Success 200 {object} response.Response "保存成功"
// @Router /api/v1/system/config/security [put]
func (s *ConfigService) SaveSecurityConfig(c *gin.Context) {
	var req SaveSecurityConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 验证参数
	if req.PasswordMinLength < 6 || req.PasswordMinLength > 20 {
		response.ErrorCode(c, http.StatusBadRequest, "密码最小长度必须在6-20之间")
		return
	}
	if req.MaxLoginAttempts < 3 || req.MaxLoginAttempts > 10 {
		response.ErrorCode(c, http.StatusBadRequest, "最大登录失败次数必须在3-10之间")
		return
	}

	config := &system.SecurityConfig{
		PasswordMinLength: req.PasswordMinLength,
		SessionTimeout:    req.SessionTimeout,
		EnableCaptcha:     req.EnableCaptcha,
		MaxLoginAttempts:  req.MaxLoginAttempts,
		LockoutDuration:   req.LockoutDuration,
		// MFA配置
		MFAEnabled:      req.MFAEnabled,
		MFAEnforced:     req.MFAEnforced,
		MFAType:         req.MFAType,
		MFASkipDuration: req.MFASkipDuration,
	}

	if err := s.configUseCase.SaveSecurityConfig(c.Request.Context(), config); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "保存成功", nil)
}

// SaveMonitoringConfig 保存监控配置
// @Summary 保存监控配置
// @Description 保存 Prometheus 监控数据保留天数
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body SaveMonitoringConfigRequest true "监控配置"
// @Success 200 {object} response.Response "保存成功"
// @Router /api/v1/system/config/monitoring [put]
func (s *ConfigService) SaveMonitoringConfig(c *gin.Context) {
	var req SaveMonitoringConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if req.PrometheusRetentionDays < 1 || req.PrometheusRetentionDays > 3650 {
		response.ErrorCode(c, http.StatusBadRequest, "监控数据保留天数必须在1-3650之间")
		return
	}

	config := &system.MonitoringConfig{
		PrometheusRetentionDays: req.PrometheusRetentionDays,
	}
	if err := s.configUseCase.SaveMonitoringConfig(c.Request.Context(), config); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存监控配置失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "监控配置保存成功，Prometheus 侧需同步应用后生效", nil)
}

// SaveDatabaseConfig 保存数据库配置
// @Summary 保存数据库配置
// @Description 保存数据库写开关、确认策略和备份默认参数
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body SaveDatabaseConfigRequest true "数据库配置"
// @Success 200 {object} response.Response "保存成功"
// @Router /api/v1/system/config/database [put]
func (s *ConfigService) SaveDatabaseConfig(c *gin.Context) {
	var req SaveDatabaseConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if req.MaxAffectedRows < 1 || req.MaxAffectedRows > 1000000 {
		response.ErrorCode(c, http.StatusBadRequest, "最大影响行数必须在1-1000000之间")
		return
	}
	if req.DefaultBackupRetentionDays < 1 || req.DefaultBackupRetentionDays > 3650 {
		response.ErrorCode(c, http.StatusBadRequest, "默认备份保留天数必须在1-3650之间")
		return
	}

	backupStoragePath := strings.TrimSpace(req.BackupStoragePath)
	if backupStoragePath == "" {
		backupStoragePath = "./data/database-backups"
	}

	config := &system.DatabaseConfig{
		WriteEnabled:               req.WriteEnabled,
		HighRiskRequiresConfirm:    req.HighRiskRequiresConfirm,
		OperationReasonRequired:    req.OperationReasonRequired,
		MaxAffectedRows:            req.MaxAffectedRows,
		DefaultBackupRetentionDays: req.DefaultBackupRetentionDays,
		BackupStoragePath:          backupStoragePath,
	}
	if err := s.configUseCase.SaveDatabaseConfig(c.Request.Context(), config); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存数据库配置失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "数据库配置保存成功", nil)
}

// UploadLogo 上传系统Logo
// @Summary 上传系统Logo
// @Description 上传系统Logo图片
// @Tags 系统配置
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formance file true "Logo图片文件"
// @Success 200 {object} response.Response{} "上传成功"
// @Router /api/v1/system/config/logo [post]
func (s *ConfigService) UploadLogo(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "获取文件失败: "+err.Error())
		return
	}
	defer file.Close()

	// 验证文件类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".ico": true, ".svg": true}
	if !allowedExts[ext] {
		response.ErrorCode(c, http.StatusBadRequest, "不支持的文件格式，仅支持 png/jpg/jpeg/ico/svg")
		return
	}

	// 验证文件大小 (最大2MB)
	if header.Size > 2*1024*1024 {
		response.ErrorCode(c, http.StatusBadRequest, "文件大小不能超过2MB")
		return
	}

	// 创建目录
	logoDir := filepath.Join(s.uploadDir, "logo")
	if err := os.MkdirAll(logoDir, 0755); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "创建目录失败")
		return
	}

	// 生成文件名
	filename := fmt.Sprintf("logo_%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(logoDir, filename)

	// 保存文件
	dst, err := os.Create(filePath)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "创建文件失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存文件失败")
		return
	}

	// 返回文件路径
	logoURL := "/uploads/logo/" + filename

	// 保存到配置
	basicConfig := &system.BasicConfig{
		SystemLogo: logoURL,
	}
	// 只更新Logo字段
	if err := s.configUseCase.SaveBasicConfig(c.Request.Context(), basicConfig); err != nil {
		// 忽略错误，文件已经上传成功
	}

	response.Success(c, gin.H{
		"url": logoURL,
	})
}

// GetPublicConfig 获取公开配置（无需认证）
// @Summary 获取公开配置
// @Description 获取登录页面需要的配置（验证码开关等）
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/public/config [get]
func (s *ConfigService) GetPublicConfig(c *gin.Context) {
	// 获取基础配置
	basicConfig, _ := s.configUseCase.GetBasicConfig(c.Request.Context())
	if basicConfig == nil {
		basicConfig = &system.BasicConfig{
			SystemName:        "OpsHub",
			SystemLogo:        "",
			SystemDescription: "运维管理平台",
		}
	}

	// 获取安全配置
	securityConfig, _ := s.configUseCase.GetSecurityConfig(c.Request.Context())
	if securityConfig == nil {
		securityConfig = &system.SecurityConfig{
			EnableCaptcha: true,
		}
	}

	// 获取LDAP启用状态
	ldapEnabled := s.configUseCase.IsLDAPEnabled(c.Request.Context())

	response.Success(c, gin.H{
		"systemName":        basicConfig.SystemName,
		"systemLogo":        basicConfig.SystemLogo,
		"systemDescription": basicConfig.SystemDescription,
		"enableCaptcha":     securityConfig.EnableCaptcha,
		"ldapEnabled":       ldapEnabled,
	})
}

// GetConfigUseCase 获取配置用例（供其他服务使用）
func (s *ConfigService) GetConfigUseCase() *system.ConfigUseCase {
	return s.configUseCase
}

func (s *ConfigService) getPrometheusBaseURL() string {
	if cfg := conf.Get(); cfg != nil {
		return cfg.Monitoring.Prometheus.GetBaseURL()
	}
	return (&conf.PrometheusConfig{}).GetBaseURL()
}

func (s *ConfigService) getCurrentPrometheusRetention(ctx context.Context) string {
	baseURL := s.getPrometheusBaseURL()
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/api/v1/status/flags", nil)
	if err != nil {
		return fmt.Sprintf("%dd（未能读取运行时状态）", defaultPrometheusRetentionDays)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("%dd（未能连接 Prometheus）", defaultPrometheusRetentionDays)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Sprintf("%dd（Prometheus 返回 %d）", defaultPrometheusRetentionDays, resp.StatusCode)
	}

	var payload struct {
		Status string                 `json:"status"`
		Data   map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Sprintf("%dd（状态解析失败）", defaultPrometheusRetentionDays)
	}

	retentionTime := strings.TrimSpace(fmt.Sprint(payload.Data["storage.tsdb.retention.time"]))
	retentionSize := strings.TrimSpace(fmt.Sprint(payload.Data["storage.tsdb.retention.size"]))
	if retentionTime == "" || retentionTime == "<nil>" {
		retentionTime = fmt.Sprintf("%dd", defaultPrometheusRetentionDays)
	}
	if retentionSize != "" && retentionSize != "<nil>" && retentionSize != "0B" && retentionSize != "0" {
		return fmt.Sprintf("%s（同时受 %s 限制）", retentionTime, retentionSize)
	}
	return retentionTime
}

// GetLDAPConfig 获取LDAP配置
// @Summary 获取LDAP配置
// @Description 获取LDAP认证配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response{} "获取成功"
// @Router /api/v1/system/config/ldap [get]
func (s *ConfigService) GetLDAPConfig(c *gin.Context) {
	config, err := s.configUseCase.GetLDAPConfig(c.Request.Context())
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取LDAP配置失败: "+err.Error())
		return
	}
	// 返回时隐藏密码
	safeCfg := *config
	if safeCfg.BindPassword != "" {
		safeCfg.BindPassword = "******"
	}
	response.Success(c, safeCfg)
}

// SaveLDAPConfig 保存LDAP配置
// @Summary 保存LDAP配置
// @Description 保存LDAP认证配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body system.LDAPConfig true "LDAP配置"
// @Success 200 {object} response.Response "保存成功"
// @Router /api/v1/system/config/ldap [put]
func (s *ConfigService) SaveLDAPConfig(c *gin.Context) {
	var req system.LDAPConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 如果密码是掩码，保留原密码
	if req.BindPassword == "******" {
		oldCfg, _ := s.configUseCase.GetLDAPConfig(c.Request.Context())
		if oldCfg != nil {
			req.BindPassword = oldCfg.BindPassword
		}
	}

	// 参数验证
	if req.Enabled {
		if req.Host == "" {
			response.ErrorCode(c, http.StatusBadRequest, "启用LDAP时服务器地址不能为空")
			return
		}
		if req.BindDN == "" {
			response.ErrorCode(c, http.StatusBadRequest, "启用LDAP时Bind DN不能为空")
			return
		}
		if req.BaseDN == "" {
			response.ErrorCode(c, http.StatusBadRequest, "启用LDAP时Base DN不能为空")
			return
		}
	}

	// 设置默认值
	if req.Port == 0 {
		if req.UseTLS {
			req.Port = 636
		} else {
			req.Port = 389
		}
	}
	if req.UserFilter == "" {
		req.UserFilter = "(uid=%s)"
	}
	if req.AttrUsername == "" {
		req.AttrUsername = "uid"
	}
	if req.AttrEmail == "" {
		req.AttrEmail = "mail"
	}
	if req.AttrRealName == "" {
		req.AttrRealName = "cn"
	}
	if req.AttrPhone == "" {
		req.AttrPhone = "telephoneNumber"
	}

	if err := s.configUseCase.SaveLDAPConfig(c.Request.Context(), &req); err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "保存LDAP配置失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "LDAP配置保存成功", nil)
}

// TestLDAPConnection 测试LDAP连接
// @Summary 测试LDAP连接
// @Description 使用提供的配置测试LDAP服务器连接
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body system.LDAPConfig true "LDAP配置"
// @Success 200 {object} response.Response "连接成功"
// @Router /api/v1/system/config/ldap/test [post]
func (s *ConfigService) TestLDAPConnection(c *gin.Context) {
	var req system.LDAPConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 如果密码是掩码，使用已保存的密码
	if req.BindPassword == "******" {
		oldCfg, _ := s.configUseCase.GetLDAPConfig(c.Request.Context())
		if oldCfg != nil {
			req.BindPassword = oldCfg.BindPassword
		}
	}

	if req.Host == "" || req.BindDN == "" || req.BaseDN == "" {
		response.ErrorCode(c, http.StatusBadRequest, "服务器地址、Bind DN、Base DN不能为空")
		return
	}

	if req.Port == 0 {
		if req.UseTLS {
			req.Port = 636
		} else {
			req.Port = 389
		}
	}

	// 测试连接
	address := fmt.Sprintf("%s:%d", req.Host, req.Port)
	var conn *ldap.Conn
	var err error

	if req.UseTLS {
		tlsConfig := &tls.Config{InsecureSkipVerify: req.SkipVerify}
		conn, err = ldap.DialTLS("tcp", address, tlsConfig)
	} else {
		conn, err = ldap.Dial("tcp", address)
		if err == nil && req.StartTLS {
			tlsConfig := &tls.Config{InsecureSkipVerify: req.SkipVerify}
			err = conn.StartTLS(tlsConfig)
		}
	}

	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "连接LDAP服务器失败: "+err.Error())
		return
	}
	defer conn.Close()

	// 测试绑定
	if err := conn.Bind(req.BindDN, req.BindPassword); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "LDAP绑定失败（请检查Bind DN和密码）: "+err.Error())
		return
	}

	// 测试搜索BaseDN
	searchReq := ldap.NewSearchRequest(
		req.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1, 10, false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)
	if _, err := conn.Search(searchReq); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "Base DN搜索失败（请检查Base DN是否正确）: "+err.Error())
		return
	}

	// 统计用户数
	userFilter := req.UserFilter
	if userFilter == "" {
		userFilter = "(uid=%s)"
	}
	userFilter = strings.Replace(userFilter, "%s", "*", -1)
	userSearchReq := ldap.NewSearchRequest(
		req.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 10, false,
		userFilter,
		[]string{"dn"},
		nil,
	)
	userResult, err := conn.Search(userSearchReq)
	userCount := 0
	if err == nil {
		userCount = len(userResult.Entries)
	}

	response.Success(c, gin.H{
		"message":   "LDAP连接测试成功",
		"userCount": userCount,
	})
}
