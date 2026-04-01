package dns

import (
	"context"
	"fmt"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	huaweidns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	dnsregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
	sslmodel "github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// HuaweiProvider 华为云DNS服务商
type HuaweiProvider struct {
	config *sslmodel.HuaweiDNSConfig
	client *huaweidns.DnsClient
}

// NewHuaweiProvider 创建华为云DNS Provider
func NewHuaweiProvider(config *sslmodel.HuaweiDNSConfig) (*HuaweiProvider, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(config.AccessKey).
		WithSk(config.SecretKey).
		Build()

	region := config.Region
	if region == "" {
		region = "cn-north-4"
	}

	client := huaweidns.NewDnsClient(
		huaweidns.DnsClientBuilder().
			WithRegion(dnsregion.ValueOf(region)).
			WithCredential(auth).
			Build())

	return &HuaweiProvider{
		config: config,
		client: client,
	}, nil
}

// Name 返回服务商名称
func (p *HuaweiProvider) Name() string {
	return "huawei"
}

// CreateTXTRecord 创建TXT记录
func (p *HuaweiProvider) CreateTXTRecord(ctx context.Context, domain, name, value string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Zone ID
	zoneID, err := p.getZoneID(rootDomain)
	if err != nil {
		return err
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, ".") {
		recordName = name + "."
	}
	if !strings.HasSuffix(recordName, rootDomain+".") {
		recordName = recordName + rootDomain + "."
	}

	request := &model.CreateRecordSetRequest{
		ZoneId: zoneID,
		Body: &model.CreateRecordSetRequestBody{
			Name:    recordName,
			Type:    "TXT",
			Records: []string{"\"" + value + "\""},
			Ttl:     int32Ptr(600),
		},
	}

	_, err = p.client.CreateRecordSet(request)
	if err != nil {
		return fmt.Errorf("create TXT record failed: %w", err)
	}
	return nil
}

// DeleteTXTRecord 删除TXT记录
func (p *HuaweiProvider) DeleteTXTRecord(ctx context.Context, domain, name string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Zone ID
	zoneID, err := p.getZoneID(rootDomain)
	if err != nil {
		return err
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, ".") {
		recordName = name + "."
	}
	if !strings.HasSuffix(recordName, rootDomain+".") {
		recordName = recordName + rootDomain + "."
	}

	// 查询记录
	listRequest := &model.ListRecordSetsByZoneRequest{
		ZoneId: zoneID,
		Name:   &recordName,
		Type:   stringPtr("TXT"),
	}

	response, err := p.client.ListRecordSetsByZone(listRequest)
	if err != nil {
		return fmt.Errorf("list record sets failed: %w", err)
	}

	// 删除匹配的记录
	if response.Recordsets != nil {
		for _, record := range *response.Recordsets {
			deleteRequest := &model.DeleteRecordSetRequest{
				ZoneId:      zoneID,
				RecordsetId: *record.Id,
			}
			_, err = p.client.DeleteRecordSet(deleteRequest)
			if err != nil {
				return fmt.Errorf("delete TXT record failed: %w", err)
			}
		}
	}
	return nil
}

// TestConnection 测试连接
func (p *HuaweiProvider) TestConnection(ctx context.Context) error {
	request := &model.ListPublicZonesRequest{
		Limit: int32Ptr(1),
	}

	_, err := p.client.ListPublicZones(request)
	if err != nil {
		return fmt.Errorf("test connection failed: %w", err)
	}
	return nil
}

// getZoneID 获取Zone ID
func (p *HuaweiProvider) getZoneID(domain string) (string, error) {
	zoneName := domain
	if !strings.HasSuffix(zoneName, ".") {
		zoneName = zoneName + "."
	}

	request := &model.ListPublicZonesRequest{
		Name: &zoneName,
	}

	response, err := p.client.ListPublicZones(request)
	if err != nil {
		return "", fmt.Errorf("list public zones failed: %w", err)
	}

	if response.Zones == nil || len(*response.Zones) == 0 {
		return "", fmt.Errorf("zone not found for domain: %s", domain)
	}

	return *(*response.Zones)[0].Id, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
