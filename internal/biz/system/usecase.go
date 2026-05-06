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
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ConfigRepoInterface 系统配置仓库接口
type ConfigRepoInterface interface {
	GetByKey(ctx context.Context, key string) (*SysConfig, error)
	GetByGroup(ctx context.Context, group string) ([]*SysConfig, error)
	GetAll(ctx context.Context) ([]*SysConfig, error)
	Save(ctx context.Context, config *SysConfig) error
	SaveOrUpdate(ctx context.Context, key, value string) error
	BatchSaveOrUpdate(ctx context.Context, configs map[string]string) error
	InitDefaultConfigs(ctx context.Context) error
}

// LoginAttemptRepoInterface 登录尝试记录仓库接口
type LoginAttemptRepoInterface interface {
	GetByUsername(ctx context.Context, username string) (*SysUserLoginAttempt, error)
	IncrementFailCount(ctx context.Context, username string, maxAttempts int, lockoutDuration int) error
	ResetFailCount(ctx context.Context, username string) error
	IsLocked(ctx context.Context, username string) (bool, time.Time, error)
}

// ConfigUseCase 系统配置用例
type ConfigUseCase struct {
	configRepo       ConfigRepoInterface
	loginAttemptRepo LoginAttemptRepoInterface
}

// NewConfigUseCase 创建系统配置用例
func NewConfigUseCase(configRepo ConfigRepoInterface, loginAttemptRepo LoginAttemptRepoInterface) *ConfigUseCase {
	return &ConfigUseCase{
		configRepo:       configRepo,
		loginAttemptRepo: loginAttemptRepo,
	}
}

// GetAllConfig 获取所有配置
func (uc *ConfigUseCase) GetAllConfig(ctx context.Context) (*AllConfig, error) {
	configs, err := uc.configRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为map方便查找
	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	// 获取LDAP配置
	ldapConfig, _ := uc.GetLDAPConfig(ctx)

	// 构建响应
	result := &AllConfig{
		Basic: BasicConfig{
			SystemName:        getStringValue(configMap, ConfigKeySystemName, "OpsHub"),
			SystemLogo:        getStringValue(configMap, ConfigKeySystemLogo, ""),
			SystemDescription: getStringValue(configMap, ConfigKeySystemDescription, "运维管理平台"),
		},
		Security: SecurityConfig{
			PasswordMinLength: getIntValue(configMap, ConfigKeyPasswordMinLength, 8),
			SessionTimeout:    getIntValue(configMap, ConfigKeySessionTimeout, 3600),
			EnableCaptcha:     getBoolValue(configMap, ConfigKeyEnableCaptcha, true),
			MaxLoginAttempts:  getIntValue(configMap, ConfigKeyMaxLoginAttempts, 5),
			LockoutDuration:   getIntValue(configMap, ConfigKeyLockoutDuration, 300),
			// MFA配置
			MFAEnabled:      getBoolValue(configMap, ConfigKeyMFAEnabled, false),
			MFAEnforced:     getBoolValue(configMap, ConfigKeyMFAEnforced, false),
			MFAType:         getStringValue(configMap, ConfigKeyMFAType, "totp"),
			MFASkipDuration: getIntValue(configMap, ConfigKeyMFASkipDuration, 2592000),
		},
		Monitoring: MonitoringConfig{
			PrometheusRetentionDays: getIntValue(configMap, ConfigKeyPrometheusRetentionDays, 15),
		},
		AuditLog: AuditLogConfig{
			Enabled:              getBoolValue(configMap, ConfigKeyAuditLogEnabled, true),
			RetentionDays:        normalizeAuditLogRetentionDays(getIntValue(configMap, ConfigKeyAuditLogRetentionDays, 30)),
			AutoCleanupEnabled:   getBoolValue(configMap, ConfigKeyAuditLogAutoCleanupEnabled, true),
			ExcludedPathPrefixes: parseAuditLogExcludedPathPrefixes(configMap),
		},
		Database: DatabaseConfig{
			WriteEnabled:               getBoolValue(configMap, ConfigKeyDatabaseWriteEnabled, false),
			WriteExplainEnabled:        getBoolValue(configMap, ConfigKeyDatabaseWriteExplainEnabled, false),
			DDLEnabled:                 getBoolValue(configMap, ConfigKeyDatabaseDDLEnabled, false),
			DDLHighRiskRequiresConfirm: getBoolValue(configMap, ConfigKeyDatabaseDDLHighRiskRequiresConfirm, true),
			DDLReasonRequired:          getBoolValue(configMap, ConfigKeyDatabaseDDLReasonRequired, true),
			DDLRequireBackupHint:       getBoolValue(configMap, ConfigKeyDatabaseDDLRequireBackupHint, true),
			HighRiskRequiresConfirm:    getBoolValue(configMap, ConfigKeyDatabaseHighRiskRequiresConfirm, true),
			OperationReasonRequired:    getBoolValue(configMap, ConfigKeyDatabaseOperationReasonRequired, true),
			MaxAffectedRows:            getIntValue(configMap, ConfigKeyDatabaseMaxAffectedRows, 1000),
			DefaultBackupRetentionDays: getIntValue(configMap, ConfigKeyDatabaseDefaultBackupRetentionDays, 7),
			BackupStoragePath:          getStringValue(configMap, ConfigKeyDatabaseBackupStoragePath, "./data/database-backups"),
			InstancePermissionMode:     normalizeDatabaseInstancePermissionMode(getStringValue(configMap, ConfigKeyDatabaseInstancePermissionMode, "compat")),
		},
		LDAP: ldapConfig,
	}

	return result, nil
}

// GetMonitoringConfig 获取监控配置
func (uc *ConfigUseCase) GetMonitoringConfig(ctx context.Context) (*MonitoringConfig, error) {
	configs, err := uc.configRepo.GetByGroup(ctx, ConfigGroupMonitoring)
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	return &MonitoringConfig{
		PrometheusRetentionDays: getIntValue(configMap, ConfigKeyPrometheusRetentionDays, 15),
	}, nil
}

// GetAuditLogConfig 获取操作日志策略配置
func (uc *ConfigUseCase) GetAuditLogConfig(ctx context.Context) (*AuditLogConfig, error) {
	configs, err := uc.configRepo.GetByGroup(ctx, ConfigGroupAuditLog)
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	return &AuditLogConfig{
		Enabled:              getBoolValue(configMap, ConfigKeyAuditLogEnabled, true),
		RetentionDays:        normalizeAuditLogRetentionDays(getIntValue(configMap, ConfigKeyAuditLogRetentionDays, 30)),
		AutoCleanupEnabled:   getBoolValue(configMap, ConfigKeyAuditLogAutoCleanupEnabled, true),
		ExcludedPathPrefixes: parseAuditLogExcludedPathPrefixes(configMap),
	}, nil
}

// GetDatabaseConfig 获取数据库配置
func (uc *ConfigUseCase) GetDatabaseConfig(ctx context.Context) (*DatabaseConfig, error) {
	configs, err := uc.configRepo.GetByGroup(ctx, ConfigGroupDatabase)
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	return &DatabaseConfig{
		WriteEnabled:               getBoolValue(configMap, ConfigKeyDatabaseWriteEnabled, false),
		WriteExplainEnabled:        getBoolValue(configMap, ConfigKeyDatabaseWriteExplainEnabled, false),
		DDLEnabled:                 getBoolValue(configMap, ConfigKeyDatabaseDDLEnabled, false),
		DDLHighRiskRequiresConfirm: getBoolValue(configMap, ConfigKeyDatabaseDDLHighRiskRequiresConfirm, true),
		DDLReasonRequired:          getBoolValue(configMap, ConfigKeyDatabaseDDLReasonRequired, true),
		DDLRequireBackupHint:       getBoolValue(configMap, ConfigKeyDatabaseDDLRequireBackupHint, true),
		HighRiskRequiresConfirm:    getBoolValue(configMap, ConfigKeyDatabaseHighRiskRequiresConfirm, true),
		OperationReasonRequired:    getBoolValue(configMap, ConfigKeyDatabaseOperationReasonRequired, true),
		MaxAffectedRows:            getIntValue(configMap, ConfigKeyDatabaseMaxAffectedRows, 1000),
		DefaultBackupRetentionDays: getIntValue(configMap, ConfigKeyDatabaseDefaultBackupRetentionDays, 7),
		BackupStoragePath:          getStringValue(configMap, ConfigKeyDatabaseBackupStoragePath, "./data/database-backups"),
		InstancePermissionMode:     normalizeDatabaseInstancePermissionMode(getStringValue(configMap, ConfigKeyDatabaseInstancePermissionMode, "compat")),
	}, nil
}

// GetBasicConfig 获取基础配置
func (uc *ConfigUseCase) GetBasicConfig(ctx context.Context) (*BasicConfig, error) {
	configs, err := uc.configRepo.GetByGroup(ctx, ConfigGroupBasic)
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	return &BasicConfig{
		SystemName:        getStringValue(configMap, ConfigKeySystemName, "OpsHub"),
		SystemLogo:        getStringValue(configMap, ConfigKeySystemLogo, ""),
		SystemDescription: getStringValue(configMap, ConfigKeySystemDescription, "运维管理平台"),
	}, nil
}

// GetSecurityConfig 获取安全配置
func (uc *ConfigUseCase) GetSecurityConfig(ctx context.Context) (*SecurityConfig, error) {
	configs, err := uc.configRepo.GetByGroup(ctx, ConfigGroupSecurity)
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, c := range configs {
		configMap[c.Key] = c.Value
	}

	return &SecurityConfig{
		PasswordMinLength: getIntValue(configMap, ConfigKeyPasswordMinLength, 8),
		SessionTimeout:    getIntValue(configMap, ConfigKeySessionTimeout, 3600),
		EnableCaptcha:     getBoolValue(configMap, ConfigKeyEnableCaptcha, true),
		MaxLoginAttempts:  getIntValue(configMap, ConfigKeyMaxLoginAttempts, 5),
		LockoutDuration:   getIntValue(configMap, ConfigKeyLockoutDuration, 300),
		// MFA配置
		MFAEnabled:      getBoolValue(configMap, ConfigKeyMFAEnabled, false),
		MFAEnforced:     getBoolValue(configMap, ConfigKeyMFAEnforced, false),
		MFAType:         getStringValue(configMap, ConfigKeyMFAType, "totp"),
		MFASkipDuration: getIntValue(configMap, ConfigKeyMFASkipDuration, 2592000),
	}, nil
}

// SaveBasicConfig 保存基础配置
func (uc *ConfigUseCase) SaveBasicConfig(ctx context.Context, config *BasicConfig) error {
	configs := map[string]string{
		ConfigKeySystemName:        config.SystemName,
		ConfigKeySystemLogo:        config.SystemLogo,
		ConfigKeySystemDescription: config.SystemDescription,
	}
	return uc.configRepo.BatchSaveOrUpdate(ctx, configs)
}

// SaveSecurityConfig 保存安全配置
func (uc *ConfigUseCase) SaveSecurityConfig(ctx context.Context, config *SecurityConfig) error {
	configs := map[string]string{
		ConfigKeyPasswordMinLength: strconv.Itoa(config.PasswordMinLength),
		ConfigKeySessionTimeout:    strconv.Itoa(config.SessionTimeout),
		ConfigKeyEnableCaptcha:     strconv.FormatBool(config.EnableCaptcha),
		ConfigKeyMaxLoginAttempts:  strconv.Itoa(config.MaxLoginAttempts),
		ConfigKeyLockoutDuration:   strconv.Itoa(config.LockoutDuration),
		// MFA配置
		ConfigKeyMFAEnabled:      strconv.FormatBool(config.MFAEnabled),
		ConfigKeyMFAEnforced:     strconv.FormatBool(config.MFAEnforced),
		ConfigKeyMFAType:         config.MFAType,
		ConfigKeyMFASkipDuration: strconv.Itoa(config.MFASkipDuration),
	}
	return uc.configRepo.BatchSaveOrUpdate(ctx, configs)
}

// SaveMonitoringConfig 保存监控配置
func (uc *ConfigUseCase) SaveMonitoringConfig(ctx context.Context, config *MonitoringConfig) error {
	configs := map[string]string{
		ConfigKeyPrometheusRetentionDays: strconv.Itoa(config.PrometheusRetentionDays),
	}
	return uc.configRepo.BatchSaveOrUpdate(ctx, configs)
}

// SaveAuditLogConfig 保存操作日志策略配置
func (uc *ConfigUseCase) SaveAuditLogConfig(ctx context.Context, config *AuditLogConfig) error {
	prefixes := normalizeAuditLogExcludedPathPrefixes(config.ExcludedPathPrefixes)
	data, err := json.Marshal(prefixes)
	if err != nil {
		return err
	}
	configs := map[string]string{
		ConfigKeyAuditLogEnabled:              strconv.FormatBool(config.Enabled),
		ConfigKeyAuditLogRetentionDays:        strconv.Itoa(normalizeAuditLogRetentionDays(config.RetentionDays)),
		ConfigKeyAuditLogAutoCleanupEnabled:   strconv.FormatBool(config.AutoCleanupEnabled),
		ConfigKeyAuditLogExcludedPathPrefixes: string(data),
	}
	return uc.configRepo.BatchSaveOrUpdate(ctx, configs)
}

// SaveDatabaseConfig 保存数据库配置
func (uc *ConfigUseCase) SaveDatabaseConfig(ctx context.Context, config *DatabaseConfig) error {
	configs := map[string]string{
		ConfigKeyDatabaseWriteEnabled:               strconv.FormatBool(config.WriteEnabled),
		ConfigKeyDatabaseWriteExplainEnabled:        strconv.FormatBool(config.WriteExplainEnabled),
		ConfigKeyDatabaseDDLEnabled:                 strconv.FormatBool(config.DDLEnabled),
		ConfigKeyDatabaseDDLHighRiskRequiresConfirm: strconv.FormatBool(config.DDLHighRiskRequiresConfirm),
		ConfigKeyDatabaseDDLReasonRequired:          strconv.FormatBool(config.DDLReasonRequired),
		ConfigKeyDatabaseDDLRequireBackupHint:       strconv.FormatBool(config.DDLRequireBackupHint),
		ConfigKeyDatabaseHighRiskRequiresConfirm:    strconv.FormatBool(config.HighRiskRequiresConfirm),
		ConfigKeyDatabaseOperationReasonRequired:    strconv.FormatBool(config.OperationReasonRequired),
		ConfigKeyDatabaseMaxAffectedRows:            strconv.Itoa(config.MaxAffectedRows),
		ConfigKeyDatabaseDefaultBackupRetentionDays: strconv.Itoa(config.DefaultBackupRetentionDays),
		ConfigKeyDatabaseBackupStoragePath:          config.BackupStoragePath,
		ConfigKeyDatabaseInstancePermissionMode:     normalizeDatabaseInstancePermissionMode(config.InstancePermissionMode),
	}
	return uc.configRepo.BatchSaveOrUpdate(ctx, configs)
}

func normalizeDatabaseInstancePermissionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "whitelist":
		return "whitelist"
	default:
		return "compat"
	}
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

func parseAuditLogExcludedPathPrefixes(configMap map[string]string) []string {
	value, ok := configMap[ConfigKeyAuditLogExcludedPathPrefixes]
	if !ok || strings.TrimSpace(value) == "" {
		value = defaultAuditLogExcludedPathPrefixesJSON
	}
	var prefixes []string
	if err := json.Unmarshal([]byte(value), &prefixes); err != nil {
		return normalizeAuditLogExcludedPathPrefixes(defaultAuditLogExcludedPathPrefixes())
	}
	return normalizeAuditLogExcludedPathPrefixes(prefixes)
}

func defaultAuditLogExcludedPathPrefixes() []string {
	var prefixes []string
	if err := json.Unmarshal([]byte(defaultAuditLogExcludedPathPrefixesJSON), &prefixes); err != nil {
		return []string{
			"/metrics",
			"/api/v1/public/agents/report",
			"/api/v1/public/agents/echo-ip",
			"/api/v1/public/databases/runner-agents/",
		}
	}
	return prefixes
}

func normalizeAuditLogExcludedPathPrefixes(prefixes []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		item := strings.TrimSpace(prefix)
		if item == "" {
			continue
		}
		item = "/" + strings.TrimLeft(item, "/")
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// GetConfigByKey 根据Key获取配置值
func (uc *ConfigUseCase) GetConfigByKey(ctx context.Context, key string) (string, error) {
	config, err := uc.configRepo.GetByKey(ctx, key)
	if err != nil {
		// 返回默认值
		if defaultConfig, ok := DefaultConfigs[key]; ok {
			return defaultConfig.Value, nil
		}
		return "", err
	}
	return config.Value, nil
}

// GetPasswordMinLength 获取密码最小长度
func (uc *ConfigUseCase) GetPasswordMinLength(ctx context.Context) int {
	value, err := uc.GetConfigByKey(ctx, ConfigKeyPasswordMinLength)
	if err != nil {
		return 8
	}
	length, err := strconv.Atoi(value)
	if err != nil {
		return 8
	}
	return length
}

// IsCaptchaEnabled 检查验证码是否开启
func (uc *ConfigUseCase) IsCaptchaEnabled(ctx context.Context) bool {
	value, err := uc.GetConfigByKey(ctx, ConfigKeyEnableCaptcha)
	if err != nil {
		return true
	}
	return value == "true"
}

// IsDatabaseWriteEnabled 检查数据库写操作是否开启
func (uc *ConfigUseCase) IsDatabaseWriteEnabled(ctx context.Context) bool {
	value, err := uc.GetConfigByKey(ctx, ConfigKeyDatabaseWriteEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// CheckLoginAttempt 检查登录尝试
func (uc *ConfigUseCase) CheckLoginAttempt(ctx context.Context, username string) (bool, int, error) {
	// 检查是否被锁定
	locked, lockedUntil, err := uc.loginAttemptRepo.IsLocked(ctx, username)
	if err != nil {
		return false, 0, err
	}
	if locked {
		remainingSeconds := int(time.Until(lockedUntil).Seconds())
		return true, remainingSeconds, nil
	}
	return false, 0, nil
}

// RecordLoginFailure 记录登录失败
func (uc *ConfigUseCase) RecordLoginFailure(ctx context.Context, username string) error {
	securityConfig, err := uc.GetSecurityConfig(ctx)
	if err != nil {
		// 使用默认值
		securityConfig = &SecurityConfig{
			MaxLoginAttempts: 5,
			LockoutDuration:  300,
		}
	}
	return uc.loginAttemptRepo.IncrementFailCount(ctx, username, securityConfig.MaxLoginAttempts, securityConfig.LockoutDuration)
}

// ResetLoginAttempt 重置登录尝试
func (uc *ConfigUseCase) ResetLoginAttempt(ctx context.Context, username string) error {
	return uc.loginAttemptRepo.ResetFailCount(ctx, username)
}

// InitDefaultConfigs 初始化默认配置
func (uc *ConfigUseCase) InitDefaultConfigs(ctx context.Context) error {
	return uc.configRepo.InitDefaultConfigs(ctx)
}

// GetLDAPConfig 获取LDAP配置
func (uc *ConfigUseCase) GetLDAPConfig(ctx context.Context) (*LDAPConfig, error) {
	value, err := uc.GetConfigByKey(ctx, ConfigKeyLDAPConfig)
	if err != nil || value == "" {
		return GetDefaultLDAPConfig(), nil
	}

	var config LDAPConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return GetDefaultLDAPConfig(), nil
	}

	// 设置默认值
	if config.Port == 0 {
		config.Port = 389
	}
	if config.UserFilter == "" {
		config.UserFilter = "(uid=%s)"
	}
	if config.AttrUsername == "" {
		config.AttrUsername = "uid"
	}
	if config.AttrEmail == "" {
		config.AttrEmail = "mail"
	}
	if config.AttrRealName == "" {
		config.AttrRealName = "cn"
	}
	if config.AttrPhone == "" {
		config.AttrPhone = "telephoneNumber"
	}

	return &config, nil
}

// SaveLDAPConfig 保存LDAP配置
func (uc *ConfigUseCase) SaveLDAPConfig(ctx context.Context, config *LDAPConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return uc.configRepo.SaveOrUpdate(ctx, ConfigKeyLDAPConfig, string(data))
}

// IsLDAPEnabled 检查LDAP是否启用
func (uc *ConfigUseCase) IsLDAPEnabled(ctx context.Context) bool {
	config, err := uc.GetLDAPConfig(ctx)
	if err != nil {
		return false
	}
	return config.Enabled
}

// 辅助函数
func getStringValue(m map[string]string, key, defaultValue string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

func getIntValue(m map[string]string, key string, defaultValue int) int {
	if v, ok := m[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func getBoolValue(m map[string]string, key string, defaultValue bool) bool {
	if v, ok := m[key]; ok {
		return v == "true" || v == "1"
	}
	return defaultValue
}
