package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	alidns "github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/ydcloud-dy/opshub/pkg/logger"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
	"go.uber.org/zap"
)

// AliyunProvider 阿里云DNS服务商
type AliyunProvider struct {
	config *model.AliyunDNSConfig
	client *alidns.Client
}

// NewAliyunProvider 创建阿里云DNS Provider
func NewAliyunProvider(config *model.AliyunDNSConfig) (*AliyunProvider, error) {
	regionID := config.RegionID
	if regionID == "" {
		regionID = "cn-hangzhou"
	}
	client, err := alidns.NewClientWithAccessKey(regionID, config.AccessKeyID, config.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create aliyun dns client failed: %w", err)
	}
	// 设置HTTP超时
	client.SetConnectTimeout(30 * time.Second)
	client.SetReadTimeout(30 * time.Second)
	return &AliyunProvider{
		config: config,
		client: client,
	}, nil
}

// Name 返回服务商名称
func (p *AliyunProvider) Name() string {
	return "aliyun"
}

// CreateTXTRecord 创建TXT记录
func (p *AliyunProvider) CreateTXTRecord(ctx context.Context, domain, name, value string) error {
	logger.Info("开始创建DNS TXT记录", zap.String("domain", domain), zap.String("name", name))

	// 提取根域名和子域名
	rootDomain := ExtractRootDomain(domain)
	rr := name
	if strings.HasSuffix(name, "."+rootDomain) {
		rr = strings.TrimSuffix(name, "."+rootDomain)
	}

	logger.Info("提取域名信息", zap.String("rootDomain", rootDomain), zap.String("rr", rr))

	request := alidns.CreateAddDomainRecordRequest()
	request.Scheme = "https"
	request.DomainName = rootDomain
	request.RR = rr
	request.Type = "TXT"
	request.Value = value
	request.TTL = "600"

	logger.Info("正在调用阿里云API添加DNS记录...")
	_, err := p.client.AddDomainRecord(request)
	if err != nil {
		logger.Error("添加DNS TXT记录失败", zap.Error(err))
		return fmt.Errorf("add TXT record failed: %w", err)
	}
	logger.Info("DNS TXT记录创建成功", zap.String("domain", domain), zap.String("name", name))
	return nil
}

// DeleteTXTRecord 删除TXT记录
func (p *AliyunProvider) DeleteTXTRecord(ctx context.Context, domain, name string) error {
	// 提取根域名和子域名
	rootDomain := ExtractRootDomain(domain)
	rr := name
	if strings.HasSuffix(name, "."+rootDomain) {
		rr = strings.TrimSuffix(name, "."+rootDomain)
	}

	// 先查询记录ID
	describeRequest := alidns.CreateDescribeDomainRecordsRequest()
	describeRequest.Scheme = "https"
	describeRequest.DomainName = rootDomain
	describeRequest.RRKeyWord = rr
	describeRequest.Type = "TXT"

	response, err := p.client.DescribeDomainRecords(describeRequest)
	if err != nil {
		return fmt.Errorf("describe domain records failed: %w", err)
	}

	// 删除匹配的记录
	for _, record := range response.DomainRecords.Record {
		if record.RR == rr && record.Type == "TXT" {
			deleteRequest := alidns.CreateDeleteDomainRecordRequest()
			deleteRequest.Scheme = "https"
			deleteRequest.RecordId = record.RecordId
			_, err := p.client.DeleteDomainRecord(deleteRequest)
			if err != nil {
				return fmt.Errorf("delete TXT record failed: %w", err)
			}
		}
	}
	return nil
}

// TestConnection 测试连接
func (p *AliyunProvider) TestConnection(ctx context.Context) error {
	request := alidns.CreateDescribeDomainsRequest()
	request.Scheme = "https"
	request.PageSize = "1"

	_, err := p.client.DescribeDomains(request)
	if err != nil {
		return fmt.Errorf("test connection failed: %w", err)
	}
	return nil
}
