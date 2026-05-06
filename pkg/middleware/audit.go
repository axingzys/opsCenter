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

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/internal/biz/audit"
	"github.com/ydcloud-dy/opshub/internal/service/rbac"
	appLogger "github.com/ydcloud-dy/opshub/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	auditLogConfigKeyEnabled              = "audit_log_enabled"
	auditLogConfigKeyRetentionDays        = "audit_log_retention_days"
	auditLogConfigKeyAutoCleanupEnabled   = "audit_log_auto_cleanup_enabled"
	auditLogConfigKeyExcludedPathPrefixes = "audit_log_excluded_path_prefixes"

	auditLogConfigCacheTTL = time.Minute
	auditLogCleanupBatch   = 5000
)

var hardSkipLogPathPrefixes = []string{
	"/health",
	"/swagger",
	"/uploads",
	"/assets",
	"/api/v1/captcha",
}

var defaultAuditLogExcludedPathPrefixes = []string{
	"/metrics",
	"/api/v1/public/agents/report",
	"/api/v1/public/agents/echo-ip",
	"/api/v1/public/databases/runner-agents/",
}

type auditLogRuntimeConfig struct {
	Enabled              bool
	RetentionDays        int
	AutoCleanupEnabled   bool
	ExcludedPathPrefixes []string
}

type auditLogConfigCache struct {
	db *gorm.DB

	mu        sync.RWMutex
	cfg       auditLogRuntimeConfig
	expiresAt time.Time
}

type sysConfigRecord struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}

// AuditLogOperation 操作审计日志中间件
func AuditLogOperation(db *gorm.DB) gin.HandlerFunc {
	configCache := newAuditLogConfigCache(db)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if shouldSkipLog(path, configCache.Get()) {
			c.Next()
			return
		}

		// 开始时间
		start := time.Now()

		// 读取请求体（因为可能需要记录参数）
		var bodyBytes []byte
		if c.Request.Body != nil && c.Request.Method != "GET" {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			// 重新设置请求体，以便后续处理器可以读取
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// 使用响应写入器包装器来捕获响应状态码
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			status:         200,
		}
		c.Writer = writer

		// 处理请求
		c.Next()

		// 计算耗时
		costTime := time.Since(start).Milliseconds()

		// 获取用户信息
		userID, username, realName := getUserInfo(c)

		// 获取模块和操作类型
		module, action, description := getOperationInfo(path, c.Request.Method)

		// 获取请求参数
		params := getRequestParams(c, bodyBytes)

		// 构建操作日志
		log := &audit.SysOperationLog{
			UserID:      userID,
			Username:    username,
			RealName:    realName,
			Module:      module,
			Action:      action,
			Description: description,
			Method:      c.Request.Method,
			Path:        path,
			Params:      params,
			Status:      writer.status,
			CostTime:    costTime,
			IP:          c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
		}

		// 如果有错误，记录错误信息
		if len(c.Errors) > 0 {
			log.ErrorMsg = c.Errors.String()
		}

		// 异步保存日志
		go func() {
			if err := db.Create(log).Error; err != nil {
				appLogger.Error("保存操作日志失败",
					zap.Error(err),
					zap.String("path", path),
					zap.String("username", username),
				)
			}
		}()
	}
}

// shouldSkipLog 判断是否跳过记录日志
func shouldSkipLog(path string, cfg auditLogRuntimeConfig) bool {
	if !cfg.Enabled {
		return true
	}
	for _, skip := range hardSkipLogPathPrefixes {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	for _, skip := range cfg.ExcludedPathPrefixes {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

func newAuditLogConfigCache(db *gorm.DB) *auditLogConfigCache {
	return &auditLogConfigCache{
		db:  db,
		cfg: defaultAuditLogRuntimeConfig(),
	}
}

func (c *auditLogConfigCache) Get() auditLogRuntimeConfig {
	if c == nil || c.db == nil {
		return defaultAuditLogRuntimeConfig()
	}
	now := time.Now()
	c.mu.RLock()
	if now.Before(c.expiresAt) {
		cfg := c.cfg
		c.mu.RUnlock()
		return cfg
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Before(c.expiresAt) {
		return c.cfg
	}
	cfg := loadAuditLogRuntimeConfig(c.db)
	c.cfg = cfg
	c.expiresAt = now.Add(auditLogConfigCacheTTL)
	return cfg
}

func defaultAuditLogRuntimeConfig() auditLogRuntimeConfig {
	return auditLogRuntimeConfig{
		Enabled:              true,
		RetentionDays:        30,
		AutoCleanupEnabled:   true,
		ExcludedPathPrefixes: append([]string(nil), defaultAuditLogExcludedPathPrefixes...),
	}
}

func loadAuditLogRuntimeConfig(db *gorm.DB) auditLogRuntimeConfig {
	cfg := defaultAuditLogRuntimeConfig()
	if db == nil {
		return cfg
	}
	keys := []string{
		auditLogConfigKeyEnabled,
		auditLogConfigKeyRetentionDays,
		auditLogConfigKeyAutoCleanupEnabled,
		auditLogConfigKeyExcludedPathPrefixes,
	}
	var rows []sysConfigRecord
	if err := db.Table("sys_config").
		Select("`key`, `value`").
		Where("`key` IN ?", keys).
		Where("deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		appLogger.Warn("读取操作日志策略失败，使用默认策略", zap.Error(err))
		return cfg
	}

	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	if value, ok := values[auditLogConfigKeyEnabled]; ok {
		cfg.Enabled = parseAuditLogBool(value, true)
	}
	if value, ok := values[auditLogConfigKeyRetentionDays]; ok {
		cfg.RetentionDays = normalizeAuditLogRetentionDays(parseAuditLogInt(value, 30))
	}
	if value, ok := values[auditLogConfigKeyAutoCleanupEnabled]; ok {
		cfg.AutoCleanupEnabled = parseAuditLogBool(value, true)
	}
	if value, ok := values[auditLogConfigKeyExcludedPathPrefixes]; ok {
		cfg.ExcludedPathPrefixes = parseAuditLogPathPrefixes(value, defaultAuditLogExcludedPathPrefixes)
	}
	return cfg
}

func parseAuditLogBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseAuditLogInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeAuditLogRetentionDays(days int) int {
	if days < 1 {
		return 30
	}
	if days > 3650 {
		return 3650
	}
	return days
}

func parseAuditLogPathPrefixes(value string, fallback []string) []string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return normalizeAuditLogPathPrefixes(fallback)
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		items = strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ','
		})
	}
	return normalizeAuditLogPathPrefixes(items)
}

func normalizeAuditLogPathPrefixes(items []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		prefix := strings.TrimSpace(item)
		if prefix == "" {
			continue
		}
		prefix = "/" + strings.TrimLeft(prefix, "/")
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		result = append(result, prefix)
	}
	return result
}

// StartAuditLogCleanup 启动操作日志清理任务。
func StartAuditLogCleanup(ctx context.Context, db *gorm.DB) {
	if db == nil {
		return
	}
	go func() {
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				runAuditLogCleanup(ctx, db)
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}

func runAuditLogCleanup(ctx context.Context, db *gorm.DB) {
	cfg := loadAuditLogRuntimeConfig(db)
	if !cfg.AutoCleanupEnabled || cfg.RetentionDays < 1 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -cfg.RetentionDays)
	total := int64(0)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var ids []uint
		if err := db.WithContext(ctx).
			Table("sys_operation_log").
			Select("id").
			Where("created_at < ?", cutoff).
			Order("id ASC").
			Limit(auditLogCleanupBatch).
			Pluck("id", &ids).Error; err != nil {
			appLogger.Warn("查询待清理操作日志失败", zap.Error(err))
			return
		}
		if len(ids) == 0 {
			if total > 0 {
				appLogger.Info("操作日志清理完成", zap.Int64("deleted", total), zap.Time("cutoff", cutoff))
			}
			return
		}
		res := db.WithContext(ctx).Exec("DELETE FROM sys_operation_log WHERE id IN ?", ids)
		if res.Error != nil {
			appLogger.Warn("清理操作日志失败", zap.Error(res.Error))
			return
		}
		total += res.RowsAffected
		if len(ids) < auditLogCleanupBatch {
			appLogger.Info("操作日志清理完成", zap.Int64("deleted", total), zap.Time("cutoff", cutoff))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// getUserInfo 从上下文中获取用户信息
func getUserInfo(c *gin.Context) (uint, string, string) {
	// 使用 rbac 包中定义的常量键名（与认证中间件一致）
	if userID, exists := c.Get(rbac.UserIdKey); exists {
		if uid, ok := userID.(uint); ok {
			if username, exists := c.Get(rbac.UsernameKey); exists {
				if uname, ok := username.(string); ok {
					// 尝试获取真实姓名
					realName := ""
					if realNameVal, exists := c.Get("realName"); exists {
						if rname, ok := realNameVal.(string); ok {
							realName = rname
						}
					}
					return uid, uname, realName
				}
			}
		}
	}

	// 如果没有认证信息，返回默认值
	return 0, "", ""
}

// getOperationInfo 根据路径和请求方法获取操作信息
func getOperationInfo(path string, method string) (module, action, description string) {
	// 根据路径前缀确定模块（按照菜单结构）
	switch {
	// 系统管理 - 用户管理
	case strings.HasPrefix(path, "/api/v1/users"):
		module = "系统管理"
		switch method {
		case "POST":
			action = "创建"
			description = "创建用户"
		case "PUT":
			action = "更新"
			description = "更新用户信息"
		case "DELETE":
			action = "删除"
			description = "删除用户"
		default:
			action = "查询"
			description = "查询用户列表"
		}
	// 系统管理 - 角色管理
	case strings.HasPrefix(path, "/api/v1/roles"):
		module = "系统管理"
		switch method {
		case "POST":
			action = "创建"
			description = "创建角色"
		case "PUT":
			action = "更新"
			description = "更新角色信息"
		case "DELETE":
			action = "删除"
			description = "删除角色"
		default:
			action = "查询"
			description = "查询角色列表"
		}
	// 系统管理 - 部门管理
	case strings.HasPrefix(path, "/api/v1/departments"):
		module = "系统管理"
		switch method {
		case "POST":
			action = "创建"
			description = "创建部门"
		case "PUT":
			action = "更新"
			description = "更新部门信息"
		case "DELETE":
			action = "删除"
			description = "删除部门"
		default:
			action = "查询"
			description = "查询部门树"
		}
	// 系统管理 - 菜单管理
	case strings.HasPrefix(path, "/api/v1/menus"):
		module = "系统管理"
		switch method {
		case "POST":
			action = "创建"
			description = "创建菜单"
		case "PUT":
			action = "更新"
			description = "更新菜单信息"
		case "DELETE":
			action = "删除"
			description = "删除菜单"
		default:
			action = "查询"
			description = "查询菜单树"
		}
	// 系统管理 - 岗位管理
	case strings.HasPrefix(path, "/api/v1/positions"):
		module = "系统管理"
		switch method {
		case "POST":
			action = "创建"
			description = "创建岗位"
		case "PUT":
			action = "更新"
			description = "更新岗位信息"
		case "DELETE":
			action = "删除"
			description = "删除岗位"
		default:
			action = "查询"
			description = "查询岗位列表"
		}
	// 个人信息
	case path == "/api/v1/profile" || strings.HasPrefix(path, "/api/v1/profile"):
		module = "个人信息"
		switch method {
		case "PUT":
			action = "更新"
			if strings.Contains(path, "/password") {
				description = "修改密码"
			} else if strings.Contains(path, "/avatar") {
				description = "更新头像"
			} else {
				description = "更新个人信息"
			}
		default:
			action = "查询"
			description = "查询个人信息"
		}
	// 操作审计
	case strings.HasPrefix(path, "/api/v1/audit"):
		module = "操作审计"
		switch {
		case strings.Contains(path, "/operation-logs"):
			description = "操作日志管理"
		case strings.Contains(path, "/login-logs"):
			description = "登录日志管理"
		case strings.Contains(path, "/data-logs"):
			description = "数据日志管理"
		default:
			description = "操作审计"
		}
		action = getActionFromMethod(method)
	// 容器管理
	case strings.HasPrefix(path, "/api/v1/plugins/kubernetes"):
		module = "容器管理"
		action = getActionFromMethod(method)
		description = getK8sOperationDescription(path, method)
	// 任务中心
	case strings.HasPrefix(path, "/api/v1/plugins/task") || strings.HasPrefix(path, "/api/v1/plugins/jobs") || strings.HasPrefix(path, "/api/v1/plugins/templates") || strings.HasPrefix(path, "/api/v1/plugins/ansible"):
		module = "任务中心"
		action = getActionFromMethod(method)
		description = getTaskOperationDescription(path, method)
	// 监控中心
	case strings.HasPrefix(path, "/api/v1/plugins/monitor"):
		module = "监控中心"
		action = getActionFromMethod(method)
		description = getMonitorOperationDescription(path, method)
	// 资产管理
	case strings.HasPrefix(path, "/api/v1/hosts") || strings.HasPrefix(path, "/api/v1/asset") || strings.HasPrefix(path, "/api/v1/terminal") || strings.HasPrefix(path, "/api/v1/cloud-accounts") || strings.HasPrefix(path, "/api/v1/credentials"):
		module = "资产管理"
		action = getActionFromMethod(method)
		description = getAssetOperationDescription(path, method)
	// 登录接口
	case path == "/api/v1/public/login":
		module = "系统管理"
		action = "登录"
		description = "用户登录"
	default:
		module = "系统"
		action = method
		description = method + " " + path
	}

	return module, action, description
}

// getActionFromMethod 根据请求方法获取操作类型
func getActionFromMethod(method string) string {
	switch method {
	case "GET":
		return "查询"
	case "POST":
		return "创建"
	case "PUT", "PATCH":
		return "更新"
	case "DELETE":
		return "删除"
	default:
		return method
	}
}

// getK8sOperationDescription 获取K8s操作描述
func getK8sOperationDescription(path string, method string) string {
	if strings.Contains(path, "/clusters") {
		return "集群管理操作"
	}
	if strings.Contains(path, "/deployments") || strings.Contains(path, "/workloads") {
		return "工作负载操作"
	}
	if strings.Contains(path, "/pods") {
		return "Pod管理操作"
	}
	if strings.Contains(path, "/services") {
		return "服务管理操作"
	}
	if strings.Contains(path, "/ingresses") {
		return "Ingress管理操作"
	}
	if strings.Contains(path, "/configmaps") || strings.Contains(path, "/secrets") {
		return "配置管理操作"
	}
	if strings.Contains(path, "/pv") || strings.Contains(path, "/pvc") || strings.Contains(path, "/storage") {
		return "存储管理操作"
	}
	if strings.Contains(path, "/namespaces") {
		return "命名空间管理"
	}
	if strings.Contains(path, "/nodes") {
		return "节点管理"
	}
	if strings.Contains(path, "/terminal") {
		return "终端操作"
	}
	if strings.Contains(path, "/files") {
		return "文件操作"
	}
	if strings.Contains(path, "/network") {
		return "网络管理操作"
	}
	if strings.Contains(path, "/access") {
		return "访问控制管理"
	}
	return "容器管理操作"
}

// getMonitorOperationDescription 获取监控中心操作描述
func getMonitorOperationDescription(path string, method string) string {
	if strings.Contains(path, "/domains") {
		return "域名监控操作"
	}
	return "监控中心操作"
}

// getTaskOperationDescription 获取任务操作描述
func getTaskOperationDescription(path string, method string) string {
	if strings.Contains(path, "/ansible") {
		return "Ansible任务操作"
	}
	if strings.Contains(path, "/jobs") {
		return "任务管理操作"
	}
	return "任务中心操作"
}

// getAssetOperationDescription 获取资产操作描述
func getAssetOperationDescription(path string, method string) string {
	if strings.Contains(path, "/hosts") {
		return "主机管理操作"
	}
	if strings.Contains(path, "/terminal") {
		return "终端操作"
	}
	if strings.Contains(path, "/asset") {
		return "资产管理操作"
	}
	return "资产管理操作"
}

// getRequestParams 获取请求参数
func getRequestParams(c *gin.Context, bodyBytes []byte) string {
	// 对于GET请求，记录查询参数
	if c.Request.Method == "GET" {
		return c.Request.URL.RawQuery
	}

	// 对于POST/PUT/DELETE请求，记录请求体（但过滤敏感信息）
	if len(bodyBytes) > 0 {
		var params map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &params); err == nil {
			// 过滤敏感字段
			filterSensitiveFields(params)
			if filtered, err := json.Marshal(params); err == nil {
				return string(filtered)
			}
		}
		return string(bodyBytes)
	}

	return ""
}

// filterSensitiveFields 过滤敏感字段
func filterSensitiveFields(params map[string]interface{}) {
	sensitiveFields := []string{"password", "pwd", "secret", "token", "key"}

	for _, field := range sensitiveFields {
		if _, exists := params[field]; exists {
			params[field] = "******"
		}
	}

	// 递归处理嵌套对象
	for _, v := range params {
		if nested, ok := v.(map[string]interface{}); ok {
			filterSensitiveFields(nested)
		}
	}
}

// responseWriter 响应写入器包装器，用于捕获状态码
type responseWriter struct {
	gin.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write implements the io.Writer interface
func (w *responseWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

// WriteString implements the gin.ResponseWriter interface
func (w *responseWriter) WriteString(str string) (int, error) {
	return w.ResponseWriter.WriteString(str)
}
