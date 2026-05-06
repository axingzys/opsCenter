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
	"time"

	"gorm.io/gorm"
)

// SysConfig 系统配置表
type SysConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"type:varchar(100);uniqueIndex;not null;comment:配置键" json:"key"`
	Value     string         `gorm:"type:text;comment:配置值" json:"value"`
	Type      string         `gorm:"type:varchar(20);default:'string';comment:配置类型(string/int/bool/json)" json:"type"`
	Group     string         `gorm:"type:varchar(50);index;comment:配置分组(basic/security)" json:"group"`
	Remark    string         `gorm:"type:varchar(200);comment:备注说明" json:"remark"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SysConfig) TableName() string {
	return "sys_config"
}

// SysUserLoginAttempt 用户登录失败记录表
type SysUserLoginAttempt struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Username    string     `gorm:"type:varchar(50);index;not null;comment:用户名" json:"username"`
	FailCount   int        `gorm:"default:0;comment:失败次数" json:"failCount"`
	LastFailAt  time.Time  `gorm:"comment:最后失败时间" json:"lastFailAt"`
	LockedUntil *time.Time `gorm:"comment:锁定截止时间" json:"lockedUntil"`
}

// TableName 指定表名
func (SysUserLoginAttempt) TableName() string {
	return "sys_user_login_attempt"
}

// ConfigKey 配置键常量
const (
	// 基础配置
	ConfigKeySystemName        = "system_name"
	ConfigKeySystemLogo        = "system_logo"
	ConfigKeySystemDescription = "system_description"

	// 安全配置
	ConfigKeyPasswordMinLength = "password_min_length"
	ConfigKeySessionTimeout    = "session_timeout"
	ConfigKeyEnableCaptcha     = "enable_captcha"
	ConfigKeyMaxLoginAttempts  = "max_login_attempts"
	ConfigKeyLockoutDuration   = "lockout_duration"

	// MFA 配置
	ConfigKeyMFAEnabled      = "mfa_enabled"       // 是否启用MFA功能
	ConfigKeyMFAEnforced     = "mfa_enforced"      // 是否强制所有用户启用MFA
	ConfigKeyMFAType         = "mfa_type"          // MFA类型（totp）
	ConfigKeyMFASkipDuration = "mfa_skip_duration" // MFA记住设备时长（秒）

	// LDAP 配置
	ConfigKeyLDAPConfig = "ldap_config" // JSON格式存储完整LDAP配置

	// 监控配置
	ConfigKeyPrometheusRetentionDays = "prometheus_retention_days"

	// 日志策略配置
	ConfigKeyAuditLogEnabled              = "audit_log_enabled"
	ConfigKeyAuditLogRetentionDays        = "audit_log_retention_days"
	ConfigKeyAuditLogAutoCleanupEnabled   = "audit_log_auto_cleanup_enabled"
	ConfigKeyAuditLogExcludedPathPrefixes = "audit_log_excluded_path_prefixes"

	// 数据库配置
	ConfigKeyDatabaseWriteEnabled               = "database_write_enabled"
	ConfigKeyDatabaseWriteExplainEnabled        = "database_write_explain_enabled"
	ConfigKeyDatabaseDDLEnabled                 = "database_ddl_enabled"
	ConfigKeyDatabaseDDLHighRiskRequiresConfirm = "database_ddl_high_risk_requires_confirm"
	ConfigKeyDatabaseDDLReasonRequired          = "database_ddl_reason_required"
	ConfigKeyDatabaseDDLRequireBackupHint       = "database_ddl_require_backup_hint"
	ConfigKeyDatabaseHighRiskRequiresConfirm    = "database_high_risk_requires_confirm"
	ConfigKeyDatabaseOperationReasonRequired    = "database_operation_reason_required"
	ConfigKeyDatabaseMaxAffectedRows            = "database_max_affected_rows"
	ConfigKeyDatabaseDefaultBackupRetentionDays = "database_default_backup_retention_days"
	ConfigKeyDatabaseBackupStoragePath          = "database_backup_storage_path"
	ConfigKeyDatabaseInstancePermissionMode     = "database_instance_permission_mode"

	// 消息队列配置
	ConfigKeyMessageQueueHighRiskEnabled         = "messageQueueHighRiskEnabled"
	ConfigKeyMessageQueueOperationReasonRequired = "messageQueueOperationReasonRequired"
	ConfigKeyMessageQueueOperationMaxMetadataAge = "messageQueueOperationMaxMetadataAgeMinutes"
)

// ConfigGroup 配置分组常量
const (
	ConfigGroupBasic        = "basic"
	ConfigGroupSecurity     = "security"
	ConfigGroupLDAP         = "ldap"
	ConfigGroupMonitoring   = "monitoring"
	ConfigGroupAuditLog     = "audit_log"
	ConfigGroupDatabase     = "database"
	ConfigGroupMessageQueue = "messagequeue"
)

const defaultAuditLogExcludedPathPrefixesJSON = `["/metrics","/api/v1/public/agents/report","/api/v1/public/agents/echo-ip","/api/v1/public/databases/runner-agents/"]`

// DefaultConfigs 默认配置
var DefaultConfigs = map[string]SysConfig{
	ConfigKeySystemName: {
		Key:    ConfigKeySystemName,
		Value:  "OpsHub",
		Type:   "string",
		Group:  ConfigGroupBasic,
		Remark: "系统名称",
	},
	ConfigKeySystemLogo: {
		Key:    ConfigKeySystemLogo,
		Value:  "",
		Type:   "string",
		Group:  ConfigGroupBasic,
		Remark: "系统Logo路径",
	},
	ConfigKeySystemDescription: {
		Key:    ConfigKeySystemDescription,
		Value:  "运维管理平台",
		Type:   "string",
		Group:  ConfigGroupBasic,
		Remark: "系统描述",
	},
	ConfigKeyPasswordMinLength: {
		Key:    ConfigKeyPasswordMinLength,
		Value:  "8",
		Type:   "int",
		Group:  ConfigGroupSecurity,
		Remark: "密码最小长度",
	},
	ConfigKeySessionTimeout: {
		Key:    ConfigKeySessionTimeout,
		Value:  "3600",
		Type:   "int",
		Group:  ConfigGroupSecurity,
		Remark: "Session超时时间(秒)",
	},
	ConfigKeyEnableCaptcha: {
		Key:    ConfigKeyEnableCaptcha,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupSecurity,
		Remark: "是否开启验证码",
	},
	ConfigKeyMaxLoginAttempts: {
		Key:    ConfigKeyMaxLoginAttempts,
		Value:  "5",
		Type:   "int",
		Group:  ConfigGroupSecurity,
		Remark: "最大登录失败次数",
	},
	ConfigKeyLockoutDuration: {
		Key:    ConfigKeyLockoutDuration,
		Value:  "300",
		Type:   "int",
		Group:  ConfigGroupSecurity,
		Remark: "账户锁定时间(秒)",
	},
	ConfigKeyMFAEnabled: {
		Key:    ConfigKeyMFAEnabled,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupSecurity,
		Remark: "是否启用MFA功能",
	},
	ConfigKeyMFAEnforced: {
		Key:    ConfigKeyMFAEnforced,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupSecurity,
		Remark: "是否强制所有用户启用MFA",
	},
	ConfigKeyMFAType: {
		Key:    ConfigKeyMFAType,
		Value:  "totp",
		Type:   "string",
		Group:  ConfigGroupSecurity,
		Remark: "MFA类型(totp)",
	},
	ConfigKeyMFASkipDuration: {
		Key:    ConfigKeyMFASkipDuration,
		Value:  "2592000",
		Type:   "int",
		Group:  ConfigGroupSecurity,
		Remark: "MFA记住设备时长(秒)，默认30天",
	},
	ConfigKeyLDAPConfig: {
		Key:    ConfigKeyLDAPConfig,
		Value:  `{"enabled":false,"host":"","port":389,"useTls":false,"startTls":false,"skipVerify":false,"bindDn":"","bindPassword":"","baseDn":"","userFilter":"(uid=%s)","attrUsername":"uid","attrEmail":"mail","attrRealName":"cn","attrPhone":"telephoneNumber","defaultRoleId":0,"defaultDeptId":0,"autoCreateUser":true}`,
		Type:   "json",
		Group:  ConfigGroupLDAP,
		Remark: "LDAP配置(JSON)",
	},
	ConfigKeyPrometheusRetentionDays: {
		Key:    ConfigKeyPrometheusRetentionDays,
		Value:  "15",
		Type:   "int",
		Group:  ConfigGroupMonitoring,
		Remark: "Prometheus监控数据目标保留天数",
	},
	ConfigKeyAuditLogEnabled: {
		Key:    ConfigKeyAuditLogEnabled,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupAuditLog,
		Remark: "操作日志记录开关",
	},
	ConfigKeyAuditLogRetentionDays: {
		Key:    ConfigKeyAuditLogRetentionDays,
		Value:  "30",
		Type:   "int",
		Group:  ConfigGroupAuditLog,
		Remark: "操作日志保留天数",
	},
	ConfigKeyAuditLogAutoCleanupEnabled: {
		Key:    ConfigKeyAuditLogAutoCleanupEnabled,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupAuditLog,
		Remark: "操作日志自动清理开关",
	},
	ConfigKeyAuditLogExcludedPathPrefixes: {
		Key:    ConfigKeyAuditLogExcludedPathPrefixes,
		Value:  defaultAuditLogExcludedPathPrefixesJSON,
		Type:   "json",
		Group:  ConfigGroupAuditLog,
		Remark: "操作日志排除路径前缀(JSON数组)",
	},
	ConfigKeyDatabaseWriteEnabled: {
		Key:    ConfigKeyDatabaseWriteEnabled,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库写操作总开关",
	},
	ConfigKeyDatabaseWriteExplainEnabled: {
		Key:    ConfigKeyDatabaseWriteExplainEnabled,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库写 SQL 执行计划开关",
	},
	ConfigKeyDatabaseDDLEnabled: {
		Key:    ConfigKeyDatabaseDDLEnabled,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库 DDL 结构变更开关",
	},
	ConfigKeyDatabaseDDLHighRiskRequiresConfirm: {
		Key:    ConfigKeyDatabaseDDLHighRiskRequiresConfirm,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库 DDL 结构变更是否要求二次确认",
	},
	ConfigKeyDatabaseDDLReasonRequired: {
		Key:    ConfigKeyDatabaseDDLReasonRequired,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库 DDL 结构变更是否要求填写原因",
	},
	ConfigKeyDatabaseDDLRequireBackupHint: {
		Key:    ConfigKeyDatabaseDDLRequireBackupHint,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库 DDL 结构变更是否提示备份",
	},
	ConfigKeyDatabaseHighRiskRequiresConfirm: {
		Key:    ConfigKeyDatabaseHighRiskRequiresConfirm,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库高风险操作是否要求二次确认",
	},
	ConfigKeyDatabaseOperationReasonRequired: {
		Key:    ConfigKeyDatabaseOperationReasonRequired,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupDatabase,
		Remark: "数据库操作是否要求填写原因",
	},
	ConfigKeyDatabaseMaxAffectedRows: {
		Key:    ConfigKeyDatabaseMaxAffectedRows,
		Value:  "1000",
		Type:   "int",
		Group:  ConfigGroupDatabase,
		Remark: "数据库写操作最大影响行数",
	},
	ConfigKeyDatabaseDefaultBackupRetentionDays: {
		Key:    ConfigKeyDatabaseDefaultBackupRetentionDays,
		Value:  "7",
		Type:   "int",
		Group:  ConfigGroupDatabase,
		Remark: "数据库备份默认保留天数",
	},
	ConfigKeyDatabaseBackupStoragePath: {
		Key:    ConfigKeyDatabaseBackupStoragePath,
		Value:  "./data/database-backups",
		Type:   "string",
		Group:  ConfigGroupDatabase,
		Remark: "数据库备份默认本地存储目录",
	},
	ConfigKeyDatabaseInstancePermissionMode: {
		Key:    ConfigKeyDatabaseInstancePermissionMode,
		Value:  "compat",
		Type:   "string",
		Group:  ConfigGroupDatabase,
		Remark: "数据库实例对象权限模式：compat兼容模式，whitelist白名单模式",
	},
	ConfigKeyMessageQueueHighRiskEnabled: {
		Key:    ConfigKeyMessageQueueHighRiskEnabled,
		Value:  "false",
		Type:   "bool",
		Group:  ConfigGroupMessageQueue,
		Remark: "MQ高危操作总开关",
	},
	ConfigKeyMessageQueueOperationReasonRequired: {
		Key:    ConfigKeyMessageQueueOperationReasonRequired,
		Value:  "true",
		Type:   "bool",
		Group:  ConfigGroupMessageQueue,
		Remark: "MQ高危操作是否要求填写原因",
	},
	ConfigKeyMessageQueueOperationMaxMetadataAge: {
		Key:    ConfigKeyMessageQueueOperationMaxMetadataAge,
		Value:  "30",
		Type:   "int",
		Group:  ConfigGroupMessageQueue,
		Remark: "MQ资源操作允许的最大元数据年龄(分钟)",
	},
}

// BasicConfig 基础配置响应结构
type BasicConfig struct {
	SystemName        string `json:"systemName"`
	SystemLogo        string `json:"systemLogo"`
	SystemDescription string `json:"systemDescription"`
}

// SecurityConfig 安全配置响应结构
type SecurityConfig struct {
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

// MonitoringConfig 监控配置响应结构
type MonitoringConfig struct {
	PrometheusRetentionDays    int    `json:"prometheusRetentionDays"`
	PrometheusBaseURL          string `json:"prometheusBaseUrl,omitempty"`
	CurrentPrometheusRetention string `json:"currentPrometheusRetention,omitempty"`
}

// AuditLogConfig 操作日志策略配置响应结构
type AuditLogConfig struct {
	Enabled              bool     `json:"enabled"`
	RetentionDays        int      `json:"retentionDays"`
	AutoCleanupEnabled   bool     `json:"autoCleanupEnabled"`
	ExcludedPathPrefixes []string `json:"excludedPathPrefixes"`
}

// DatabaseConfig 数据库配置响应结构
type DatabaseConfig struct {
	WriteEnabled               bool   `json:"writeEnabled"`
	WriteExplainEnabled        bool   `json:"writeExplainEnabled"`
	DDLEnabled                 bool   `json:"ddlEnabled"`
	DDLHighRiskRequiresConfirm bool   `json:"ddlHighRiskRequiresConfirm"`
	DDLReasonRequired          bool   `json:"ddlReasonRequired"`
	DDLRequireBackupHint       bool   `json:"ddlRequireBackupHint"`
	HighRiskRequiresConfirm    bool   `json:"highRiskRequiresConfirm"`
	OperationReasonRequired    bool   `json:"operationReasonRequired"`
	MaxAffectedRows            int    `json:"maxAffectedRows"`
	DefaultBackupRetentionDays int    `json:"defaultBackupRetentionDays"`
	BackupStoragePath          string `json:"backupStoragePath"`
	InstancePermissionMode     string `json:"instancePermissionMode"`
}

// LDAPConfig LDAP配置结构
type LDAPConfig struct {
	Enabled        bool   `json:"enabled"`        // 是否启用LDAP
	Host           string `json:"host"`           // LDAP服务器地址
	Port           int    `json:"port"`           // 端口（389/636）
	UseTLS         bool   `json:"useTls"`         // 是否使用LDAPS
	StartTLS       bool   `json:"startTls"`       // 是否使用StartTLS
	SkipVerify     bool   `json:"skipVerify"`     // 跳过TLS证书验证
	BindDN         string `json:"bindDn"`         // 管理员DN
	BindPassword   string `json:"bindPassword"`   // 管理员密码
	BaseDN         string `json:"baseDn"`         // 搜索根DN
	UserFilter     string `json:"userFilter"`     // 用户搜索过滤器，如 (uid=%s)
	AttrUsername   string `json:"attrUsername"`   // 用户名属性（uid/sAMAccountName）
	AttrEmail      string `json:"attrEmail"`      // 邮箱属性（mail）
	AttrRealName   string `json:"attrRealName"`   // 姓名属性（cn/displayName）
	AttrPhone      string `json:"attrPhone"`      // 电话属性（telephoneNumber）
	DefaultRoleID  uint   `json:"defaultRoleId"`  // LDAP用户默认角色ID
	DefaultDeptID  uint   `json:"defaultDeptId"`  // LDAP用户默认部门ID
	AutoCreateUser bool   `json:"autoCreateUser"` // 登录时自动创建本地用户
}

// GetDefaultLDAPConfig 获取LDAP默认配置
func GetDefaultLDAPConfig() *LDAPConfig {
	return &LDAPConfig{
		Enabled:        false,
		Port:           389,
		UserFilter:     "(uid=%s)",
		AttrUsername:   "uid",
		AttrEmail:      "mail",
		AttrRealName:   "cn",
		AttrPhone:      "telephoneNumber",
		AutoCreateUser: true,
	}
}

// AllConfig 所有配置响应结构
type AllConfig struct {
	Basic      BasicConfig      `json:"basic"`
	Security   SecurityConfig   `json:"security"`
	Monitoring MonitoringConfig `json:"monitoring"`
	AuditLog   AuditLogConfig   `json:"auditLog"`
	Database   DatabaseConfig   `json:"database"`
	LDAP       *LDAPConfig      `json:"ldap,omitempty"`
}
