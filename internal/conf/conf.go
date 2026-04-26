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

package conf

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Desktop        DesktopConfig        `mapstructure:"desktop"`
	Terminal       TerminalConfig       `mapstructure:"terminal"`
	Agent          AgentConfig          `mapstructure:"agent"`
	Virtualization VirtualizationConfig `mapstructure:"virtualization"`
	Monitoring     MonitoringConfig     `mapstructure:"monitoring"`
	Log            LogConfig            `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Mode         string `mapstructure:"mode"` // debug, release, test
	HttpPort     int    `mapstructure:"http_port"`
	RPCPort      int    `mapstructure:"rpc_port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`  // 毫秒
	WriteTimeout int    `mapstructure:"write_timeout"` // 毫秒
	JWTSecret    string `mapstructure:"jwt_secret"`    // JWT密钥
	ExternalURL  string `mapstructure:"external_url"`  // 外部访问URL，用于OAuth2 issuer
	FrontendURL  string `mapstructure:"frontend_url"`  // 前端URL，用于OAuth2登录重定向
}

// GetOAuth2Issuer 获取OAuth2 issuer URL
// 本地开发时使用后端端口，因为 Jenkins 服务器端需要直接调用 token endpoint
func (c *ServerConfig) GetOAuth2Issuer() string {
	if c.ExternalURL != "" {
		return c.ExternalURL
	}
	// 本地开发默认使用后端端口
	return fmt.Sprintf("http://localhost:%d", c.HttpPort)
}

// GetFrontendURL 获取前端URL
func (c *ServerConfig) GetFrontendURL() string {
	if c.FrontendURL != "" {
		return c.FrontendURL
	}
	if c.ExternalURL != "" {
		return c.ExternalURL
	}
	// 本地开发默认使用 Vite 端口
	return "http://localhost:5173"
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 秒
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Password    string `mapstructure:"password"`
	DB          int    `mapstructure:"db"`
	PoolSize    int    `mapstructure:"pool_size"`
	MinIdleConn int    `mapstructure:"min_idle_conn"`
}

// DesktopConfig 远程桌面配置
type DesktopConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	Provider         string `mapstructure:"provider"`
	PublicPath       string `mapstructure:"public_path"`
	TokenTTLSeconds  int    `mapstructure:"token_ttl_seconds"`
	JSONSecretKey    string `mapstructure:"json_secret_key"`
	RecordingPath    string `mapstructure:"recording_path"`
	TransferRootPath string `mapstructure:"transfer_root_path"`
	DriveName        string `mapstructure:"drive_name"`
	DisableUpload    bool   `mapstructure:"disable_upload"`
	DisableDownload  bool   `mapstructure:"disable_download"`
}

type TerminalConfig struct {
	RecordingPath string `mapstructure:"recording_path"`
}

func (c *TerminalConfig) GetRecordingPath() string {
	if strings.TrimSpace(c.RecordingPath) == "" {
		return "./data/terminal-recordings"
	}
	return c.RecordingPath
}

type AgentConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	BundleDir             string `mapstructure:"bundle_dir"`
	DefaultInstallPath    string `mapstructure:"default_install_path"`
	DefaultListenPort     int    `mapstructure:"default_listen_port"`
	BinaryName            string `mapstructure:"binary_name"`
	ServicePrefix         string `mapstructure:"service_prefix"`
	ReportIntervalSeconds int    `mapstructure:"report_interval_seconds"`
}

func (c *AgentConfig) GetBundleDir() string {
	if strings.TrimSpace(c.BundleDir) == "" {
		return "./agent-bundles"
	}
	return c.BundleDir
}

func (c *AgentConfig) GetDefaultInstallPath() string {
	if strings.TrimSpace(c.DefaultInstallPath) == "" {
		return "/opt/opshub-agent"
	}
	return c.DefaultInstallPath
}

func (c *AgentConfig) GetDefaultListenPort() int {
	if c.DefaultListenPort <= 0 {
		return 19100
	}
	return c.DefaultListenPort
}

func (c *AgentConfig) GetBinaryName() string {
	if strings.TrimSpace(c.BinaryName) == "" {
		return "opshub-agent"
	}
	return c.BinaryName
}

func (c *AgentConfig) GetServicePrefix() string {
	if strings.TrimSpace(c.ServicePrefix) == "" {
		return "opshub-agent"
	}
	return c.ServicePrefix
}

func (c *AgentConfig) GetReportIntervalSeconds() int {
	if c.ReportIntervalSeconds <= 0 {
		return 60
	}
	return c.ReportIntervalSeconds
}

type VirtualizationConfig struct {
	Sync VirtualizationSyncConfig `mapstructure:"sync"`
}

type VirtualizationSyncConfig struct {
	Enabled                bool `mapstructure:"enabled"`
	IntervalSeconds        int  `mapstructure:"interval_seconds"`
	InitialDelaySeconds    int  `mapstructure:"initial_delay_seconds"`
	PlatformTimeoutSeconds int  `mapstructure:"platform_timeout_seconds"`
	Concurrency            int  `mapstructure:"concurrency"`
}

func (c *VirtualizationSyncConfig) GetIntervalSeconds() int {
	if c.IntervalSeconds <= 0 {
		return 60
	}
	return c.IntervalSeconds
}

func (c *VirtualizationSyncConfig) GetInitialDelaySeconds() int {
	if c.InitialDelaySeconds < 0 {
		return 15
	}
	return c.InitialDelaySeconds
}

func (c *VirtualizationSyncConfig) GetPlatformTimeoutSeconds() int {
	if c.PlatformTimeoutSeconds <= 0 {
		return 300
	}
	return c.PlatformTimeoutSeconds
}

func (c *VirtualizationSyncConfig) GetConcurrency() int {
	if c.Concurrency <= 0 {
		return 4
	}
	return c.Concurrency
}

type MonitoringConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
}

type PrometheusConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	BaseURL             string `mapstructure:"base_url"`
	FileSDDir           string `mapstructure:"file_sd_dir"`
	QueryTimeoutSeconds int    `mapstructure:"query_timeout_seconds"`
}

func (c *PrometheusConfig) GetBaseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return "http://127.0.0.1:19090"
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *PrometheusConfig) GetFileSDDir() string {
	if strings.TrimSpace(c.FileSDDir) == "" {
		return "./data/prometheus/file_sd"
	}
	return c.FileSDDir
}

func (c *PrometheusConfig) GetQueryTimeoutSeconds() int {
	if c.QueryTimeoutSeconds <= 0 {
		return 10
	}
	return c.QueryTimeoutSeconds
}

// GetPublicPath 获取桌面访问路径
func (c *DesktopConfig) GetPublicPath() string {
	if c.PublicPath == "" {
		return "/guacamole/"
	}
	return c.PublicPath
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"` // MB
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"` // days
	Compress   bool   `mapstructure:"compress"`
	Console    bool   `mapstructure:"console"`
}

var globalConfig *Config

// Load 加载配置
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.SetDefault("virtualization.sync.enabled", true)
	v.SetDefault("virtualization.sync.interval_seconds", 60)
	v.SetDefault("virtualization.sync.initial_delay_seconds", 15)
	v.SetDefault("virtualization.sync.platform_timeout_seconds", 300)
	v.SetDefault("virtualization.sync.concurrency", 4)

	// 环境变量前缀
	v.SetEnvPrefix("OPSHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	globalConfig = config
	return config, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
	)
}

// GetRedisAddr 获取Redis地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
