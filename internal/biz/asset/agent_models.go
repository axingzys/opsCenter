package asset

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	AgentStatusPending      = "pending"
	AgentStatusDeploying    = "deploying"
	AgentStatusWaiting      = "waiting_register"
	AgentStatusRunning      = "running"
	AgentStatusOffline      = "offline"
	AgentStatusError        = "error"
	AgentStatusRestarting   = "restarting"
	AgentStatusUninstalling = "uninstalling"
	AgentStatusUninstalled  = "uninstalled"

	AgentJobTypeDeploy    = "deploy"
	AgentJobTypeReinstall = "reinstall"
	AgentJobTypeRestart   = "restart"
	AgentJobTypeUninstall = "uninstall"

	AgentJobStatusPending = "pending"
	AgentJobStatusRunning = "running"
	AgentJobStatusSuccess = "success"
	AgentJobStatusFailed  = "failed"
)

// AssetAgent Linux Agent 生命周期记录。
type AssetAgent struct {
	gorm.Model
	HostID          uint       `gorm:"column:host_id;not null;uniqueIndex;comment:主机ID" json:"hostId"`
	AgentID         string     `gorm:"column:agent_id;type:varchar(100);comment:Agent ID" json:"agentId"`
	AccessToken     string     `gorm:"column:access_token_ciphertext;type:text;comment:Agent访问令牌密文" json:"-"`
	Version         string     `gorm:"type:varchar(50);comment:Agent版本" json:"version"`
	Status          string     `gorm:"type:varchar(32);default:'pending';comment:Agent状态" json:"status"`
	ListenPort      int        `gorm:"column:listen_port;default:19100;comment:Agent监听端口" json:"listenPort"`
	InstallPath     string     `gorm:"column:install_path;type:varchar(255);comment:安装目录" json:"installPath"`
	ServiceName     string     `gorm:"column:service_name;type:varchar(64);comment:服务名" json:"serviceName"`
	InstallProgress int        `gorm:"column:install_progress;default:0;comment:安装进度" json:"installProgress"`
	InstallStage    string     `gorm:"column:install_stage;type:varchar(64);comment:安装阶段" json:"installStage"`
	LastHeartbeatAt *time.Time `gorm:"column:last_heartbeat_at;comment:最后心跳时间" json:"lastHeartbeatAt,omitempty"`
	LastReportAt    *time.Time `gorm:"column:last_report_at;comment:最后上报时间" json:"lastReportAt,omitempty"`
	LastError       string     `gorm:"column:last_error;type:text;comment:最近错误" json:"lastError"`
	DeployedBy      uint       `gorm:"column:deployed_by;comment:部署人" json:"deployedBy"`
	DeployedAt      *time.Time `gorm:"column:deployed_at;comment:部署时间" json:"deployedAt,omitempty"`
}

func (AssetAgent) TableName() string {
	return "asset_agents"
}

// AssetAgentJob Agent 操作任务。
type AssetAgentJob struct {
	gorm.Model
	HostID     uint       `gorm:"column:host_id;not null;index;comment:主机ID" json:"hostId"`
	JobType    string     `gorm:"column:job_type;type:varchar(32);not null;comment:任务类型" json:"jobType"`
	Status     string     `gorm:"type:varchar(32);not null;default:'pending';comment:任务状态" json:"status"`
	Progress   int        `gorm:"default:0;comment:任务进度" json:"progress"`
	Stage      string     `gorm:"type:varchar(64);comment:当前阶段" json:"stage"`
	Message    string     `gorm:"type:varchar(255);comment:当前提示" json:"message"`
	Error      string     `gorm:"type:text;comment:错误信息" json:"error"`
	OperatorID uint       `gorm:"column:operator_id;comment:操作人" json:"operatorId"`
	StartedAt  *time.Time `gorm:"column:started_at;comment:开始时间" json:"startedAt,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;comment:结束时间" json:"finishedAt,omitempty"`
}

func (AssetAgentJob) TableName() string {
	return "asset_agent_jobs"
}

// AssetHostInventory 主机最新快照。
type AssetHostInventory struct {
	gorm.Model
	HostID             uint       `gorm:"column:host_id;not null;uniqueIndex;comment:主机ID" json:"hostId"`
	PrivateIPsJSON     string     `gorm:"column:private_ips_json;type:json;comment:内网IP列表" json:"-"`
	PublicIPsJSON      string     `gorm:"column:public_ips_json;type:json;comment:公网IP列表" json:"-"`
	InterfacesJSON     string     `gorm:"column:interfaces_json;type:json;comment:网卡信息" json:"-"`
	DisksJSON          string     `gorm:"column:disks_json;type:json;comment:磁盘详情" json:"-"`
	TopProcessesJSON   string     `gorm:"column:top_processes_json;type:json;comment:进程快照" json:"-"`
	ListeningPortsJSON string     `gorm:"column:listening_ports_json;type:json;comment:监听端口快照" json:"-"`
	ConfigSummaryJSON  string     `gorm:"column:config_summary_json;type:json;comment:配置摘要" json:"-"`
	CollectedAt        *time.Time `gorm:"column:collected_at;comment:采集时间" json:"collectedAt,omitempty"`
}

func (AssetHostInventory) TableName() string {
	return "asset_host_inventory"
}

type AssetHostPublicIPHistory struct {
	gorm.Model
	HostID      uint      `gorm:"column:host_id;not null;index:idx_asset_host_public_ip_history_host_id;comment:主机ID" json:"hostId"`
	IP          string    `gorm:"column:ip;type:varchar(50);not null;comment:公网出口IP" json:"ip"`
	Source      string    `gorm:"column:source;type:varchar(32);comment:来源" json:"source"`
	FirstSeenAt time.Time `gorm:"column:first_seen_at;not null;comment:首次发现时间" json:"firstSeenAt"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at;not null;comment:最近发现时间" json:"lastSeenAt"`
	SeenCount   int       `gorm:"column:seen_count;default:1;comment:观测次数" json:"seenCount"`
	IsCurrent   bool      `gorm:"column:is_current;default:false;index:idx_asset_host_public_ip_history_current;comment:是否当前IP" json:"isCurrent"`
}

func (AssetHostPublicIPHistory) TableName() string {
	return "asset_host_public_ip_history"
}

type AgentNetworkInterface struct {
	Name      string   `json:"name"`
	Hardware  string   `json:"hardware"`
	Addresses []string `json:"addresses"`
}

type AgentProcessInfo struct {
	PID           int     `json:"pid"`
	Name          string  `json:"name"`
	Command       string  `json:"command"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
}

type AgentPortInfo struct {
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	ListenAddr  string `json:"listenAddr"`
	ProcessName string `json:"processName"`
	PID         int    `json:"pid"`
}

type AgentConfigSummary struct {
	OSRelease        string   `json:"osRelease"`
	Kernel           string   `json:"kernel"`
	Arch             string   `json:"arch"`
	Timezone         string   `json:"timezone"`
	ServiceManager   string   `json:"serviceManager"`
	ContainerRuntime string   `json:"containerRuntime"`
	Mounts           []string `json:"mounts"`
	AgentVersion     string   `json:"agentVersion"`
}

type HostInventoryVO struct {
	HostID          uint                    `json:"hostId"`
	PrivateIPs      []string                `json:"privateIps"`
	PublicIPs       []string                `json:"publicIps"`
	PublicIPHistory []HostPublicIPHistoryVO `json:"publicIpHistory"`
	Interfaces      []AgentNetworkInterface `json:"interfaces"`
	Disks           []collectorDiskVO       `json:"disks"`
	TopProcesses    []AgentProcessInfo      `json:"topProcesses"`
	ListeningPorts  []AgentPortInfo         `json:"listeningPorts"`
	ConfigSummary   AgentConfigSummary      `json:"configSummary"`
	CollectedAt     string                  `json:"collectedAt,omitempty"`
}

type HostPublicIPHistoryVO struct {
	IP          string `json:"ip"`
	Source      string `json:"source"`
	FirstSeenAt string `json:"firstSeenAt"`
	LastSeenAt  string `json:"lastSeenAt"`
	SeenCount   int    `json:"seenCount"`
	IsCurrent   bool   `json:"isCurrent"`
}

type HostFileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"isDir"`
	ModTime string `json:"modTime"`
}

// collectorDiskVO 避免在前端返回时暴露内部 collector 包类型。
type collectorDiskVO struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mountPoint"`
	FSType     string  `json:"fstype"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	Usage      float64 `json:"usage"`
}

type AgentListItemVO struct {
	HostID           uint   `json:"hostId"`
	HostName         string `json:"hostName"`
	IP               string `json:"ip"`
	HostIP           string `json:"hostIp"`
	PrimaryPrivateIP string `json:"primaryPrivateIp"`
	PrimaryPublicIP  string `json:"primaryPublicIp"`
	Version          string `json:"version"`
	Status           string `json:"status"`
	StatusText       string `json:"statusText"`
	ListenPort       int    `json:"listenPort"`
	InstallProgress  int    `json:"installProgress"`
	InstallStage     string `json:"installStage"`
	InstallStageText string `json:"installStageText"`
	HealthStatus     string `json:"healthStatus"`
	LastHeartbeatAt  string `json:"lastHeartbeatAt,omitempty"`
	LastReportAt     string `json:"lastReportAt,omitempty"`
	UpdateTime       string `json:"updateTime"`
	LastError        string `json:"lastError,omitempty"`
	AgentID          string `json:"agentId,omitempty"`
}

type AgentDeployRequest struct {
	HostIDs []uint `json:"hostIds" binding:"required"`
}

type AgentReinstallRequest struct {
	HostIDs []uint `json:"hostIds" binding:"required"`
}

type AgentUninstallRequest struct {
	HostIDs []uint `json:"hostIds" binding:"required"`
}

type AgentJobVO struct {
	ID         uint   `json:"id"`
	HostID     uint   `json:"hostId"`
	JobType    string `json:"jobType"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Stage      string `json:"stage"`
	Message    string `json:"message"`
	Error      string `json:"error"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	UpdateTime string `json:"updateTime"`
}

func AgentStatusText(status string) string {
	switch status {
	case AgentStatusDeploying:
		return "部署中"
	case AgentStatusWaiting:
		return "等待注册"
	case AgentStatusRunning:
		return "运行中"
	case AgentStatusOffline:
		return "离线"
	case AgentStatusError:
		return "异常"
	case AgentStatusRestarting:
		return "重启中"
	case AgentStatusUninstalling:
		return "卸载中"
	case AgentStatusUninstalled:
		return "已卸载"
	default:
		return "待部署"
	}
}

func AgentHealthStatus(status string, lastHeartbeatAt *time.Time) string {
	if status == AgentStatusRunning && lastHeartbeatAt != nil && time.Since(*lastHeartbeatAt) <= agentHeartbeatTimeout {
		return "healthy"
	}
	if status == AgentStatusRunning || status == AgentStatusOffline {
		return "degraded"
	}
	return "unknown"
}

func AgentStageText(stage string) string {
	switch stage {
	case "building":
		return "构建中"
	case "uploading":
		return "传输中"
	case "configuring":
		return "配置中"
	case "starting":
		return "启动中"
	case "waiting_register":
		return "等待注册"
	case "running":
		return "运行中"
	case "removing":
		return "卸载中"
	default:
		return stage
	}
}

func encodeJSON(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeJSON(data string, target any) {
	data = strings.TrimSpace(data)
	if data == "" {
		return
	}
	_ = json.Unmarshal([]byte(data), target)
}
