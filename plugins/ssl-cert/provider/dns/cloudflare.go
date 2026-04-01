package dns

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// CloudflareProvider Cloudflare DNS服务商
type CloudflareProvider struct {
	config *model.CloudflareDNSConfig
	api    *cloudflare.API
}

// NewCloudflareProvider 创建Cloudflare DNS Provider
func NewCloudflareProvider(config *model.CloudflareDNSConfig) (*CloudflareProvider, error) {
	var api *cloudflare.API
	var err error

	if config.APIToken != "" {
		// 优先使用API Token
		api, err = cloudflare.NewWithAPIToken(config.APIToken)
	} else if config.Email != "" && config.APIKey != "" {
		// 使用全局API Key
		api, err = cloudflare.New(config.APIKey, config.Email)
	} else {
		return nil, fmt.Errorf("cloudflare config requires either api_token or (email + api_key)")
	}

	if err != nil {
		return nil, fmt.Errorf("create cloudflare client failed: %w", err)
	}

	return &CloudflareProvider{
		config: config,
		api:    api,
	}, nil
}

// Name 返回服务商名称
func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

// CreateTXTRecord 创建TXT记录
func (p *CloudflareProvider) CreateTXTRecord(ctx context.Context, domain, name, value string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Zone ID
	zoneID, err := p.api.ZoneIDByName(rootDomain)
	if err != nil {
		return fmt.Errorf("get zone id failed: %w", err)
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, "."+rootDomain) {
		recordName = name + "." + rootDomain
	}

	record := cloudflare.CreateDNSRecordParams{
		Type:    "TXT",
		Name:    recordName,
		Content: value,
		TTL:     600,
	}

	_, err = p.api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), record)
	if err != nil {
		return fmt.Errorf("create TXT record failed: %w", err)
	}
	return nil
}

// DeleteTXTRecord 删除TXT记录
func (p *CloudflareProvider) DeleteTXTRecord(ctx context.Context, domain, name string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Zone ID
	zoneID, err := p.api.ZoneIDByName(rootDomain)
	if err != nil {
		return fmt.Errorf("get zone id failed: %w", err)
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, "."+rootDomain) {
		recordName = name + "." + rootDomain
	}

	// 查询记录
	records, _, err := p.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Type: "TXT",
		Name: recordName,
	})
	if err != nil {
		return fmt.Errorf("list dns records failed: %w", err)
	}

	// 删除匹配的记录
	for _, record := range records {
		err = p.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), record.ID)
		if err != nil {
			return fmt.Errorf("delete TXT record failed: %w", err)
		}
	}
	return nil
}

// TestConnection 测试连接
func (p *CloudflareProvider) TestConnection(ctx context.Context) error {
	_, err := p.api.ListZones(ctx)
	if err != nil {
		return fmt.Errorf("test connection failed: %w", err)
	}
	return nil
}
