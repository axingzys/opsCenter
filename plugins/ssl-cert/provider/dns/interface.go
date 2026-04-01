package dns

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// Provider DNS服务商接口
type Provider interface {
	// Name 服务商名称
	Name() string
	// CreateTXTRecord 创建TXT记录
	CreateTXTRecord(ctx context.Context, domain, name, value string) error
	// DeleteTXTRecord 删除TXT记录
	DeleteTXTRecord(ctx context.Context, domain, name string) error
	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// Factory DNS Provider工厂
type Factory struct{}

// NewFactory 创建工厂
func NewFactory() *Factory {
	return &Factory{}
}

// Create 根据配置创建DNS Provider
func (f *Factory) Create(provider *model.DNSProvider) (Provider, error) {
	switch provider.Provider {
	case model.DNSProviderAliyun:
		var config model.AliyunDNSConfig
		if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
			return nil, fmt.Errorf("parse aliyun config failed: %w", err)
		}
		return NewAliyunProvider(&config)
	case model.DNSProviderCloudflare:
		var config model.CloudflareDNSConfig
		if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
			return nil, fmt.Errorf("parse cloudflare config failed: %w", err)
		}
		return NewCloudflareProvider(&config)
	case model.DNSProviderHuawei:
		var config model.HuaweiDNSConfig
		if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
			return nil, fmt.Errorf("parse huawei config failed: %w", err)
		}
		return NewHuaweiProvider(&config)
	case model.DNSProviderAWSRoute53:
		var config model.AWSRoute53Config
		if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
			return nil, fmt.Errorf("parse aws route53 config failed: %w", err)
		}
		return NewAWSRoute53Provider(&config)
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", provider.Provider)
	}
}

// ExtractRootDomain 从域名中提取根域名
// 例如: _acme-challenge.www.example.com -> example.com
func ExtractRootDomain(domain string) string {
	// 简单实现：取最后两段
	// 实际生产中应该使用 publicsuffix 库
	parts := splitDomain(domain)
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return domain
}

// ExtractSubdomain 提取子域名部分
// 例如: _acme-challenge.www.example.com, example.com -> _acme-challenge.www
func ExtractSubdomain(fullDomain, rootDomain string) string {
	if len(fullDomain) <= len(rootDomain) {
		return ""
	}
	// 移除根域名和末尾的点
	sub := fullDomain[:len(fullDomain)-len(rootDomain)-1]
	return sub
}

func splitDomain(domain string) []string {
	var parts []string
	var current string
	for _, c := range domain {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
