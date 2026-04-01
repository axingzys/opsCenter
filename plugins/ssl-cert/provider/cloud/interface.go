package cloud

import (
	"context"
	"time"
)

// Provider 云厂商证书服务接口
type Provider interface {
	// Name 服务商名称
	Name() string
	// ListCertificates 列出证书
	ListCertificates(ctx context.Context) ([]*CertificateInfo, error)
	// GetCertificate 获取证书详情(含证书内容)
	GetCertificate(ctx context.Context, certID string) (*CertificateDetail, error)
	// ApplyCertificate 申请证书(需要DNS验证)
	ApplyCertificate(ctx context.Context, domain string, sanDomains []string) (*ApplyResult, error)
	// DownloadCertificate 下载证书
	DownloadCertificate(ctx context.Context, certID string) (*CertificateBundle, error)
	// CheckCertificateStatus 检查证书状态
	CheckCertificateStatus(ctx context.Context, orderID string) (*CertificateStatus, error)
}

// CertificateInfo 证书基本信息
type CertificateInfo struct {
	CertID      string    // 证书ID
	Domain      string    // 主域名
	SANDomains  []string  // SAN域名
	Status      string    // 状态
	NotBefore   time.Time // 生效时间
	NotAfter    time.Time // 过期时间
	Issuer      string    // 颁发者
	ProductName string    // 产品名称
}

// CertificateDetail 证书详情
type CertificateDetail struct {
	CertificateInfo
	Certificate string // 证书PEM
	PrivateKey  string // 私钥PEM
	CertChain   string // 证书链
}

// CertificateBundle 证书包
type CertificateBundle struct {
	Certificate string // 证书PEM
	PrivateKey  string // 私钥PEM
	CertChain   string // 证书链
}

// ApplyResult 证书申请结果
type ApplyResult struct {
	OrderID        string               // 订单ID
	CertID         string               // 证书ID(如果已签发)
	Status         string               // 状态: pending_validation, issued, failed
	ValidationType string               // 验证类型: DNS, FILE
	DNSRecord      *DNSValidationRecord // DNS验证记录
	Message        string               // 消息
}

// DNSValidationRecord DNS验证记录
type DNSValidationRecord struct {
	RecordName  string // 记录名称
	RecordType  string // 记录类型 (TXT, CNAME等)
	RecordValue string // 记录值
}

// CertificateStatus 证书状态
type CertificateStatus struct {
	Status      string // pending, issued, failed
	Certificate string // 证书内容(已签发时)
	PrivateKey  string // 私钥(已签发时)
	CertChain   string // 证书链(已签发时)
	Message     string // 状态消息
}
