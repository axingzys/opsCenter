package model

import (
	"time"

	"gorm.io/gorm"
)

// DNSProvider DNS服务商配置
type DNSProvider struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name     string `gorm:"type:varchar(100);not null" json:"name"`
	Provider string `gorm:"type:varchar(50);not null" json:"provider"` // aliyun/cloudflare/huawei/aws_route53
	Config   string `gorm:"type:text;not null" json:"-"`               // 配置JSON(加密) - 列表不返回
	Email    string `gorm:"type:varchar(255)" json:"email"`            // 联系邮箱
	Phone    string `gorm:"type:varchar(50)" json:"phone"`             // 联系电话
	Enabled  bool   `gorm:"default:true" json:"enabled"`

	LastTestAt *time.Time `json:"last_test_at"`
	LastTestOK bool       `json:"last_test_ok"`
}

// TableName 表名
func (DNSProvider) TableName() string {
	return "ssl_dns_providers"
}

// DNS服务商类型常量
const (
	DNSProviderAliyun     = "aliyun"      // 阿里云DNS
	DNSProviderCloudflare = "cloudflare"  // Cloudflare
	DNSProviderHuawei     = "huawei"      // 华为云DNS
	DNSProviderAWSRoute53 = "aws_route53" // AWS Route53
)

// AliyunDNSConfig 阿里云DNS配置
type AliyunDNSConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	RegionID        string `json:"region_id,omitempty"`
}

// CloudflareDNSConfig Cloudflare DNS配置
type CloudflareDNSConfig struct {
	APIToken string `json:"api_token"` // 推荐使用API Token
	Email    string `json:"email,omitempty"`
	APIKey   string `json:"api_key,omitempty"` // 全局API Key(不推荐)
}

// HuaweiDNSConfig 华为云DNS配置
type HuaweiDNSConfig struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region,omitempty"`
}

// AWSRoute53Config AWS Route53配置
type AWSRoute53Config struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region,omitempty"`
}
