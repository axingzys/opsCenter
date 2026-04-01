package model

import (
	"time"

	"gorm.io/gorm"
)

// DeployConfig 部署配置
type DeployConfig struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	CertificateID uint   `gorm:"index;not null" json:"certificate_id"`
	Name          string `gorm:"type:varchar(100);not null" json:"name"`
	DeployType    string `gorm:"type:varchar(20);not null" json:"deploy_type"` // nginx_ssh/k8s_secret
	TargetConfig  string `gorm:"type:text;not null" json:"target_config"`      // 配置JSON
	AutoDeploy    bool   `gorm:"default:true" json:"auto_deploy"`              // 续期后自动部署
	Enabled       bool   `gorm:"default:true" json:"enabled"`

	LastDeployAt *time.Time `json:"last_deploy_at"`
	LastDeployOK bool       `json:"last_deploy_ok"`
	LastError    string     `gorm:"type:text" json:"last_error"`

	// 关联
	Certificate *SSLCertificate `gorm:"foreignKey:CertificateID" json:"certificate,omitempty"`
}

// TableName 表名
func (DeployConfig) TableName() string {
	return "ssl_deploy_configs"
}

// 部署类型常量
const (
	DeployTypeNginxSSH  = "nginx_ssh"  // Nginx SSH部署
	DeployTypeK8sSecret = "k8s_secret" // K8s Secret部署
)

// NginxSSHConfig Nginx SSH部署配置
type NginxSSHConfig struct {
	HostID        uint   `json:"host_id"`                  // 主机ID
	CertPath      string `json:"cert_path"`                // 证书路径,如 /etc/nginx/ssl/cert.pem
	KeyPath       string `json:"key_path"`                 // 私钥路径,如 /etc/nginx/ssl/key.pem
	ChainPath     string `json:"chain_path,omitempty"`     // 证书链路径(可选)
	NginxBin      string `json:"nginx_bin,omitempty"`      // nginx二进制路径,默认nginx
	ReloadCommand string `json:"reload_command,omitempty"` // 重载命令,默认 nginx -s reload
	BackupEnabled bool   `json:"backup_enabled"`           // 是否备份旧证书
	BackupPath    string `json:"backup_path,omitempty"`    // 备份路径
}

// K8sSecretConfig K8s Secret部署配置
type K8sSecretConfig struct {
	ClusterID      uint              `json:"cluster_id"`            // K8s集群ID
	Namespace      string            `json:"namespace"`             // 命名空间
	SecretName     string            `json:"secret_name"`           // Secret名称
	CertKey        string            `json:"cert_key,omitempty"`    // 证书key,默认 tls.crt
	KeyKey         string            `json:"key_key,omitempty"`     // 私钥key,默认 tls.key
	Labels         map[string]string `json:"labels,omitempty"`      // 额外标签
	TriggerRollout bool              `json:"trigger_rollout"`       // 是否触发关联Deployment滚动更新
	Deployments    []string          `json:"deployments,omitempty"` // 需要滚动更新的Deployment列表
}
