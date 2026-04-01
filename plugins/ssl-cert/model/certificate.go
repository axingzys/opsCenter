package model

import (
	"time"

	"gorm.io/gorm"
)

// SSLCertificate SSL证书
type SSLCertificate struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name       string `gorm:"type:varchar(100);not null" json:"name"`         // 证书名称
	Domain     string `gorm:"type:varchar(255);not null;index" json:"domain"` // 主域名
	SANDomains string `gorm:"type:text" json:"san_domains"`                   // SAN域名(JSON数组)
	ACMEEmail  string `gorm:"type:varchar(255)" json:"acme_email"`            // ACME注册邮箱(Let's Encrypt)

	// 证书申请配置
	CAProvider   string `gorm:"type:varchar(20)" json:"ca_provider"`   // CA提供商: letsencrypt/zerossl/google/buypass
	KeyAlgorithm string `gorm:"type:varchar(20)" json:"key_algorithm"` // 密钥算法: rsa2048/rsa3072/rsa4096/ec256/ec384

	// 证书来源
	SourceType     string `gorm:"type:varchar(20)" json:"source_type"`    // acme/aliyun/manual
	CloudAccountID *uint  `gorm:"index" json:"cloud_account_id"`          // 云账号ID
	CloudCertID    string `gorm:"type:varchar(100)" json:"cloud_cert_id"` // 云厂商证书ID

	// 证书内容(加密存储)
	Certificate string `gorm:"type:text" json:"-"` // 证书PEM
	PrivateKey  string `gorm:"type:text" json:"-"` // 私钥PEM(加密)
	CertChain   string `gorm:"type:text" json:"-"` // 证书链

	// 证书信息
	Issuer      string     `gorm:"type:varchar(255)" json:"issuer"`
	NotBefore   *time.Time `json:"not_before"`
	NotAfter    *time.Time `gorm:"index" json:"not_after"` // 过期时间
	Fingerprint string     `gorm:"type:varchar(100)" json:"fingerprint"`

	// 续期配置
	Status          string     `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending/active/expiring/expired/error
	AutoRenew       bool       `gorm:"default:true" json:"auto_renew"`
	RenewDaysBefore int        `gorm:"default:30" json:"renew_days_before"` // 提前续期天数
	DNSProviderID   *uint      `gorm:"index" json:"dns_provider_id"`
	LastRenewAt     *time.Time `json:"last_renew_at"`
	LastError       string     `gorm:"type:text" json:"last_error"`

	// 关联
	DNSProvider   *DNSProvider   `gorm:"foreignKey:DNSProviderID" json:"dns_provider,omitempty"`
	DeployConfigs []DeployConfig `gorm:"foreignKey:CertificateID" json:"deploy_configs,omitempty"`
}

// TableName 表名
func (SSLCertificate) TableName() string {
	return "ssl_certificates"
}

// 证书状态常量
const (
	CertStatusPending  = "pending"  // 待申请
	CertStatusActive   = "active"   // 正常
	CertStatusExpiring = "expiring" // 即将过期
	CertStatusExpired  = "expired"  // 已过期
	CertStatusError    = "error"    // 错误
)

// 证书来源类型
const (
	SourceTypeACME   = "acme"   // ACME协议(Let's Encrypt等)
	SourceTypeAliyun = "aliyun" // 阿里云证书服务
	SourceTypeManual = "manual" // 手动导入
)

// CA提供商
const (
	CAProviderLetsEncrypt = "letsencrypt" // Let's Encrypt
	CAProviderZeroSSL     = "zerossl"     // ZeroSSL
	CAProviderGoogle      = "google"      // Google Trust Services
	CAProviderBuyPass     = "buypass"     // BuyPass
)

// 密钥算法
const (
	KeyAlgorithmRSA2048 = "rsa2048" // RSA 2048位
	KeyAlgorithmRSA3072 = "rsa3072" // RSA 3072位
	KeyAlgorithmRSA4096 = "rsa4096" // RSA 4096位
	KeyAlgorithmEC256   = "ec256"   // ECDSA P-256
	KeyAlgorithmEC384   = "ec384"   // ECDSA P-384
)

// CertBundle 证书包(用于部署)
type CertBundle struct {
	Certificate string // 证书PEM
	PrivateKey  string // 私钥PEM
	CertChain   string // 证书链PEM
}
