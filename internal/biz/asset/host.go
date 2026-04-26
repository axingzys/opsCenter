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

package asset

import (
	"time"

	"gorm.io/gorm"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"

	ManagementModeSSH   = "ssh"
	ManagementModeWinRM = "winrm"
	ManagementModeAgent = "agent"
	ManagementModeNone  = "none"

	CollectStatusOnline        = "online"
	CollectStatusOffline       = "offline"
	CollectStatusUnknown       = "unknown"
	CollectStatusNotConfigured = "not_configured"
)

// Host 主机模型
type Host struct {
	gorm.Model
	Name                   string      `gorm:"type:varchar(100);not null;comment:主机名称" json:"name"`
	GroupID                uint        `gorm:"column:group_id;comment:分组ID" json:"groupId"`
	Group                  *AssetGroup `gorm:"-" json:"group,omitempty"`
	Type                   string      `gorm:"type:varchar(20);not null;default:'self';comment:主机类型 self:自建 cloud:云主机" json:"type"`
	CloudProvider          string      `gorm:"type:varchar(50);comment:云厂商 aliyun/tencent/aws" json:"cloudProvider,omitempty"`
	CloudInstanceID        string      `gorm:"type:varchar(100);comment:云实例ID" json:"cloudInstanceId,omitempty"`
	CloudAccountID         uint        `gorm:"column:cloud_account_id;comment:云账号ID" json:"cloudAccountId,omitempty"`
	OSType                 string      `gorm:"column:os_type;type:varchar(20);default:'linux';comment:操作系统类型 linux/windows" json:"osType"`
	SSHUser                string      `gorm:"type:varchar(50);comment:SSH用户名" json:"sshUser"`
	IP                     string      `gorm:"type:varchar(50);not null;comment:IP地址" json:"ip"`
	Port                   int         `gorm:"type:int;default:22;comment:SSH端口" json:"port"`
	CredentialID           uint        `gorm:"column:credential_id;comment:凭证ID" json:"credentialId"`
	ManagementMode         string      `gorm:"column:management_mode;type:varchar(20);default:'ssh';comment:管理方式 ssh/winrm/agent/none" json:"managementMode"`
	ManagementPort         int         `gorm:"column:management_port;default:22;comment:管理端口" json:"managementPort"`
	ManagementCredentialID uint        `gorm:"column:management_credential_id;comment:管理凭证ID" json:"managementCredentialId"`
	DesktopEnabled         bool        `gorm:"column:desktop_enabled;default:false;comment:是否启用桌面访问" json:"desktopEnabled"`
	DesktopProtocol        string      `gorm:"column:desktop_protocol;type:varchar(20);default:'rdp';comment:桌面协议 rdp" json:"desktopProtocol"`
	DesktopPort            int         `gorm:"column:desktop_port;default:3389;comment:桌面端口" json:"desktopPort"`
	DesktopCredentialID    uint        `gorm:"column:desktop_credential_id;comment:桌面凭证ID" json:"desktopCredentialId"`
	DesktopSecurity        string      `gorm:"column:desktop_security;type:varchar(20);default:'nla';comment:桌面安全模式" json:"desktopSecurity"`
	DesktopIgnoreCert      bool        `gorm:"column:desktop_ignore_cert;default:true;comment:桌面是否忽略证书" json:"desktopIgnoreCert"`
	Credential             *Credential `gorm:"-" json:"credential,omitempty"`
	ManagementCredential   *Credential `gorm:"-" json:"managementCredential,omitempty"`
	DesktopCredential      *Credential `gorm:"-" json:"desktopCredential,omitempty"`
	Tags                   string      `gorm:"type:varchar(500);comment:主机标签(逗号分隔)" json:"tags"`
	Description            string      `gorm:"type:varchar(500);comment:备注" json:"description"`
	Status                 int         `gorm:"type:tinyint;default:1;comment:状态 1:在线 0:离线 -1:未知" json:"status"`
	LastSeen               *time.Time  `gorm:"column:last_seen;comment:最后连接时间" json:"lastSeen,omitempty"`
	CollectStatus          string      `gorm:"column:collect_status;type:varchar(20);default:'unknown';comment:采集状态 online/offline/unknown/not_configured" json:"collectStatus"`
	CollectError           string      `gorm:"column:collect_error;type:varchar(500);comment:采集错误" json:"collectError"`
	LastCollectAt          *time.Time  `gorm:"column:last_collect_at;comment:最后采集时间" json:"lastCollectAt,omitempty"`
	PrimaryPrivateIP       string      `gorm:"column:primary_private_ip;type:varchar(50);comment:主内网IP" json:"primaryPrivateIp"`
	PrimaryPublicIP        string      `gorm:"column:primary_public_ip;type:varchar(50);comment:主公网IP" json:"primaryPublicIp"`
	AgentID                string      `gorm:"column:agent_id;type:varchar(100);comment:Agent ID" json:"agentId"`
	AgentVersion           string      `gorm:"column:agent_version;type:varchar(50);comment:Agent 版本" json:"agentVersion"`
	AgentLastHeartbeatAt   *time.Time  `gorm:"column:agent_last_heartbeat_at;comment:Agent 最后心跳时间" json:"agentLastHeartbeatAt,omitempty"`
	AgentPort              int         `gorm:"column:agent_port;default:19100;comment:Agent监听端口" json:"agentPort"`
	AgentLastReportAt      *time.Time  `gorm:"column:agent_last_report_at;comment:Agent最后上报时间" json:"agentLastReportAt,omitempty"`
	AgentLastError         string      `gorm:"column:agent_last_error;type:varchar(500);comment:Agent最近错误" json:"agentLastError"`
	OS                     string      `gorm:"type:varchar(100);comment:操作系统" json:"os"`
	Kernel                 string      `gorm:"type:varchar(100);comment:内核版本" json:"kernel"`
	Arch                   string      `gorm:"type:varchar(50);comment:架构" json:"arch"`
	// 扩展信息字段（JSON存储）
	CPUInfo     string  `gorm:"type:text;comment:CPU信息JSON" json:"-"`
	CPUCores    int     `gorm:"type:int;comment:CPU核心数" json:"cpuCores"`
	CPUUsage    float64 `gorm:"type:float;comment:CPU使用率" json:"cpuUsage"`
	MemoryTotal uint64  `gorm:"type:bigint;comment:内存总容量(字节)" json:"memoryTotal"`
	MemoryUsed  uint64  `gorm:"type:bigint;comment:已用内存(字节)" json:"memoryUsed"`
	MemoryUsage float64 `gorm:"type:float;comment:内存使用率" json:"memoryUsage"`
	DiskTotal   uint64  `gorm:"type:bigint;comment:磁盘总容量(字节)" json:"diskTotal"`
	DiskUsed    uint64  `gorm:"type:bigint;comment:已用磁盘(字节)" json:"diskUsed"`
	DiskUsage   float64 `gorm:"type:float;comment:磁盘使用率" json:"diskUsage"`
	Uptime      string  `gorm:"type:varchar(100);comment:运行时间" json:"uptime"`
	Hostname    string  `gorm:"type:varchar(100);comment:主机名" json:"hostname"`
}

// HostRequest 主机请求
type HostRequest struct {
	ID                     uint   `json:"id"`
	Name                   string `json:"name" binding:"required,min=2,max=100"`
	GroupID                uint   `json:"groupId"`
	Type                   string `json:"type" binding:"required,oneof=self cloud"`
	CloudProvider          string `json:"cloudProvider,omitempty"`
	CloudInstanceID        string `json:"cloudInstanceId,omitempty"`
	CloudAccountID         uint   `json:"cloudAccountId,omitempty"`
	OSType                 string `json:"osType" binding:"omitempty,oneof=linux windows"`
	SSHUser                string `json:"sshUser"`
	IP                     string `json:"ip" binding:"required,ip"`
	Port                   int    `json:"port" binding:"omitempty,min=1,max=65535"`
	CredentialID           uint   `json:"credentialId"`
	ManagementMode         string `json:"managementMode" binding:"omitempty,oneof=ssh winrm agent none"`
	ManagementPort         int    `json:"managementPort" binding:"omitempty,min=0,max=65535"`
	ManagementCredentialID uint   `json:"managementCredentialId"`
	DesktopEnabled         bool   `json:"desktopEnabled"`
	DesktopProtocol        string `json:"desktopProtocol" binding:"omitempty,oneof=rdp"`
	DesktopPort            int    `json:"desktopPort" binding:"omitempty,min=1,max=65535"`
	DesktopCredentialID    uint   `json:"desktopCredentialId"`
	DesktopSecurity        string `json:"desktopSecurity" binding:"omitempty,oneof=nla tls any"`
	DesktopIgnoreCert      bool   `json:"desktopIgnoreCert"`
	Tags                   string `json:"tags"`
	Description            string `json:"description"`
}

// HostInfoVO 主机信息VO
type HostInfoVO struct {
	ID                     uint          `json:"id"`
	Name                   string        `json:"name"`
	GroupName              string        `json:"groupName"`
	GroupID                uint          `json:"groupId"`
	Type                   string        `json:"type"`
	TypeText               string        `json:"typeText"`
	CloudProvider          string        `json:"cloudProvider,omitempty"`
	CloudProviderText      string        `json:"cloudProviderText,omitempty"`
	CloudInstanceID        string        `json:"cloudInstanceId,omitempty"`
	OSType                 string        `json:"osType"`
	SSHUser                string        `json:"sshUser"`
	IP                     string        `json:"ip"`
	Port                   int           `json:"port"`
	CredentialID           uint          `json:"credentialId"`
	Credential             *CredentialVO `json:"credential,omitempty"`
	ManagementMode         string        `json:"managementMode"`
	ManagementModeText     string        `json:"managementModeText"`
	ManagementPort         int           `json:"managementPort"`
	ManagementCredentialID uint          `json:"managementCredentialId"`
	ManagementCredential   *CredentialVO `json:"managementCredential,omitempty"`
	DesktopEnabled         bool          `json:"desktopEnabled"`
	DesktopProtocol        string        `json:"desktopProtocol"`
	DesktopPort            int           `json:"desktopPort"`
	DesktopCredentialID    uint          `json:"desktopCredentialId"`
	DesktopCredential      *CredentialVO `json:"desktopCredential,omitempty"`
	DesktopSecurity        string        `json:"desktopSecurity"`
	DesktopIgnoreCert      bool          `json:"desktopIgnoreCert"`
	Tags                   []string      `json:"tags"`
	Description            string        `json:"description"`
	Status                 int           `json:"status"`
	StatusText             string        `json:"statusText"`
	LastSeen               string        `json:"lastSeen,omitempty"`
	CollectStatus          string        `json:"collectStatus"`
	CollectStatusText      string        `json:"collectStatusText"`
	CollectError           string        `json:"collectError"`
	LastCollectAt          string        `json:"lastCollectAt,omitempty"`
	PrimaryPrivateIP       string        `json:"primaryPrivateIp,omitempty"`
	PrimaryPublicIP        string        `json:"primaryPublicIp,omitempty"`
	AgentID                string        `json:"agentId,omitempty"`
	AgentVersion           string        `json:"agentVersion,omitempty"`
	AgentLastHeartbeatAt   string        `json:"agentLastHeartbeatAt,omitempty"`
	AgentPort              int           `json:"agentPort"`
	AgentLastReportAt      string        `json:"agentLastReportAt,omitempty"`
	AgentLastError         string        `json:"agentLastError,omitempty"`
	OS                     string        `json:"os"`
	Kernel                 string        `json:"kernel"`
	Arch                   string        `json:"arch"`
	CreateTime             string        `json:"createTime"`
	UpdateTime             string        `json:"updateTime"`
	// 扩展信息
	CPUCores    int     `json:"cpuCores"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryTotal uint64  `json:"memoryTotal"`
	MemoryUsed  uint64  `json:"memoryUsed"`
	MemoryUsage float64 `json:"memoryUsage"`
	DiskTotal   uint64  `json:"diskTotal"`
	DiskUsed    uint64  `json:"diskUsed"`
	DiskUsage   float64 `json:"diskUsage"`
	Uptime      string  `json:"uptime"`
	Hostname    string  `json:"hostname"`
}

// HostListVO 主机列表VO（用于分组下的主机列表）
type HostListVO struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	IP               string `json:"ip"`
	Status           int    `json:"status"`
	Port             int    `json:"port"`
	SSHUser          string `json:"sshUser"`
	OSType           string `json:"osType"`
	ManagementMode   string `json:"managementMode"`
	CollectStatus    string `json:"collectStatus"`
	DesktopEnabled   bool   `json:"desktopEnabled"`
	DesktopPort      int    `json:"desktopPort"`
	OS               string `json:"os"`
	PrimaryPrivateIP string `json:"primaryPrivateIp,omitempty"`
	PrimaryPublicIP  string `json:"primaryPublicIp,omitempty"`
}

// ToModel 转换为模型
func (req *HostRequest) ToModel() *Host {
	osType := req.OSType
	if osType == "" {
		osType = OSTypeLinux
	}

	port := req.Port
	if port == 0 {
		port = 22
	}

	managementMode := NormalizeManagementMode(osType, req.ManagementMode, req.CredentialID, req.ManagementCredentialID)
	managementPort := NormalizeManagementPort(managementMode, req.ManagementPort, port)
	managementCredentialID := NormalizeManagementCredentialID(managementMode, req.ManagementCredentialID, req.CredentialID)

	desktopProtocol := req.DesktopProtocol
	if desktopProtocol == "" {
		desktopProtocol = "rdp"
	}

	desktopPort := req.DesktopPort
	if desktopPort == 0 {
		desktopPort = 3389
	}

	desktopSecurity := req.DesktopSecurity
	if desktopSecurity == "" {
		desktopSecurity = "nla"
	}

	return &Host{
		Name:                   req.Name,
		GroupID:                req.GroupID,
		Type:                   req.Type,
		CloudProvider:          req.CloudProvider,
		CloudInstanceID:        req.CloudInstanceID,
		CloudAccountID:         req.CloudAccountID,
		OSType:                 osType,
		SSHUser:                req.SSHUser,
		IP:                     req.IP,
		Port:                   port,
		CredentialID:           req.CredentialID,
		ManagementMode:         managementMode,
		ManagementPort:         managementPort,
		ManagementCredentialID: managementCredentialID,
		DesktopEnabled:         req.DesktopEnabled,
		DesktopProtocol:        desktopProtocol,
		DesktopPort:            desktopPort,
		DesktopCredentialID:    req.DesktopCredentialID,
		DesktopSecurity:        desktopSecurity,
		DesktopIgnoreCert:      req.DesktopIgnoreCert,
		Tags:                   req.Tags,
		Description:            req.Description,
		Status:                 -1, // 初始状态未知
		CollectStatus:          DefaultCollectStatus(managementMode),
	}
}

// Credential 凭证模型
type Credential struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
	Name        string         `gorm:"type:varchar(100);not null;comment:凭证名称" json:"name"`
	Protocol    string         `gorm:"type:varchar(20);not null;default:'ssh';comment:连接协议 ssh/winrm/rdp" json:"protocol"`
	Type        string         `gorm:"type:varchar(20);not null;comment:认证方式 password/key" json:"type"`
	Username    string         `gorm:"type:varchar(100);comment:用户名" json:"username"`
	Domain      string         `gorm:"type:varchar(100);comment:Windows域名" json:"domain,omitempty"`
	Password    string         `gorm:"type:varchar(500);comment:密码(加密)" json:"password,omitempty"`
	PrivateKey  string         `gorm:"type:text;comment:私钥(加密)" json:"privateKey,omitempty"`
	Passphrase  string         `gorm:"type:varchar(500);comment:私钥密码(加密)" json:"passphrase,omitempty"`
	Description string         `gorm:"type:varchar(500);comment:备注" json:"description"`
}

// CredentialRequest 凭证请求
type CredentialRequest struct {
	ID              uint   `json:"id"`
	Name            string `json:"name" binding:"required,min=2,max=100"`
	Protocol        string `json:"protocol" binding:"omitempty,oneof=ssh winrm rdp"`
	Type            string `json:"type" binding:"required,oneof=password key"`
	Username        string `json:"username"`
	Domain          string `json:"domain,omitempty"`
	Password        string `json:"password"`
	ClearPassword   bool   `json:"clearPassword"`
	PrivateKey      string `json:"privateKey"`
	ClearPrivateKey bool   `json:"clearPrivateKey"`
	Passphrase      string `json:"passphrase"`
	ClearPassphrase bool   `json:"clearPassphrase"`
	Description     string `json:"description"`
}

// CredentialVO 凭证VO
type CredentialVO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Type        string `json:"type"`
	TypeText    string `json:"typeText"`
	Username    string `json:"username"`
	Domain      string `json:"domain,omitempty"`
	Description string `json:"description"`
	CreateTime  string `json:"createTime"`
	HostCount   int64  `json:"hostCount"` // 使用该凭证的主机数量
}

// ToModel 转换为模型
func (req *CredentialRequest) ToModel() *Credential {
	protocol := req.Protocol
	if protocol == "" {
		protocol = ManagementModeSSH
	}

	return &Credential{
		Name:        req.Name,
		Protocol:    protocol,
		Type:        req.Type,
		Username:    req.Username,
		Domain:      req.Domain,
		Password:    req.Password,
		PrivateKey:  req.PrivateKey,
		Passphrase:  req.Passphrase,
		Description: req.Description,
	}
}

func NormalizeManagementMode(osType, requestedMode string, credentialID, managementCredentialID uint) string {
	if requestedMode != "" {
		return requestedMode
	}
	if osType == OSTypeWindows {
		if managementCredentialID > 0 || credentialID > 0 {
			return ManagementModeSSH
		}
		return ManagementModeAgent
	}
	return ManagementModeSSH
}

func NormalizeManagementPort(mode string, requestedPort, sshPort int) int {
	if mode == ManagementModeSSH {
		if sshPort > 0 {
			return sshPort
		}
		if requestedPort > 0 {
			return requestedPort
		}
		return 22
	}
	if requestedPort > 0 {
		return requestedPort
	}
	switch mode {
	case ManagementModeWinRM:
		return 5985
	case ManagementModeAgent, ManagementModeNone:
		return 0
	default:
		if sshPort > 0 {
			return sshPort
		}
		return 22
	}
}

func NormalizeManagementCredentialID(mode string, managementCredentialID, sshCredentialID uint) uint {
	if mode == ManagementModeSSH {
		if sshCredentialID > 0 {
			return sshCredentialID
		}
		return managementCredentialID
	}
	if mode == ManagementModeWinRM {
		return managementCredentialID
	}
	if mode == ManagementModeAgent {
		if managementCredentialID > 0 {
			return managementCredentialID
		}
		return sshCredentialID
	}
	if mode == ManagementModeNone {
		return 0
	}
	if managementCredentialID > 0 {
		return managementCredentialID
	}
	return 0
}

func DefaultCollectStatus(mode string) string {
	if mode == ManagementModeNone {
		return CollectStatusNotConfigured
	}
	return CollectStatusUnknown
}

func ManagementModeText(mode string) string {
	switch mode {
	case ManagementModeWinRM:
		return "WinRM"
	case ManagementModeAgent:
		return "Agent"
	case ManagementModeNone:
		return "仅桌面"
	default:
		return "SSH"
	}
}

func CollectStatusText(status string) string {
	switch status {
	case CollectStatusOnline:
		return "在线"
	case CollectStatusOffline:
		return "离线"
	case CollectStatusNotConfigured:
		return "未配置采集"
	default:
		return "未知"
	}
}

// CloudAccount 云平台账号模型
type CloudAccount struct {
	gorm.Model
	Name        string `gorm:"type:varchar(100);not null;comment:账号名称" json:"name"`
	Provider    string `gorm:"type:varchar(50);not null;comment:云厂商 aliyun/tencent/aws/huawei" json:"provider"`
	AccessKey   string `gorm:"type:varchar(200);not null;comment:AccessKey" json:"-"`
	SecretKey   string `gorm:"type:varchar(500);not null;comment:SecretKey" json:"-"`
	Region      string `gorm:"type:varchar(100);comment:默认区域" json:"region"`
	Description string `gorm:"type:varchar(500);comment:备注" json:"description"`
	Status      int    `gorm:"type:tinyint;default:1;comment:状态 1:启用 0:禁用" json:"status"`
}

// CloudAccountRequest 云平台账号请求
type CloudAccountRequest struct {
	ID          uint   `json:"id"`
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Provider    string `json:"provider" binding:"required,oneof=aliyun tencent aws huawei jdcloud"`
	AccessKey   string `json:"accessKey"`
	SecretKey   string `json:"secretKey"`
	Region      string `json:"region"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}

// CloudAccountVO 云平台账号VO
type CloudAccountVO struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	ProviderText string `json:"providerText"`
	Region       string `json:"region"`
	Description  string `json:"description"`
	Status       int    `json:"status"`
	CreateTime   string `json:"createTime"`
}

// ToModel 转换为模型
func (req *CloudAccountRequest) ToModel() *CloudAccount {
	return &CloudAccount{
		Name:        req.Name,
		Provider:    req.Provider,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		Region:      req.Region,
		Description: req.Description,
		Status:      req.Status,
	}
}

// CloudImportRequest 云主机导入请求
type CloudImportRequest struct {
	AccountID   uint     `json:"accountId" binding:"required"`
	AccountName string   `json:"accountName"`
	Region      string   `json:"region"`
	GroupID     uint     `json:"groupId"`
	InstanceIDs []string `json:"instanceIds"` // 要导入的实例ID列表
}

// CloudInstanceVO 云主机实例VO（用于前端展示）
type CloudInstanceVO struct {
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	PublicIP   string `json:"publicIp"`
	PrivateIP  string `json:"privateIp"`
	OS         string `json:"os"`
	Status     string `json:"status"`
}

// CloudRegionVO 云区域VO
type CloudRegionVO struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
