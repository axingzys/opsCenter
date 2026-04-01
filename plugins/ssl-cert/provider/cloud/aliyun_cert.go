package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
)

// AliyunCertConfig 阿里云证书配置
type AliyunCertConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	RegionID        string `json:"region_id,omitempty"`
}

// AliyunProvider 阿里云证书服务
type AliyunProvider struct {
	config *AliyunCertConfig
	client *sdk.Client
}

// NewAliyunProvider 创建阿里云证书服务Provider
func NewAliyunProvider(config *AliyunCertConfig) (*AliyunProvider, error) {
	regionID := config.RegionID
	if regionID == "" {
		regionID = "cn-hangzhou"
	}
	client, err := sdk.NewClientWithAccessKey(regionID, config.AccessKeyID, config.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create aliyun client failed: %w", err)
	}
	return &AliyunProvider{
		config: config,
		client: client,
	}, nil
}

// NewAliyunProviderFromJSON 从JSON创建Provider
func NewAliyunProviderFromJSON(configJSON string) (*AliyunProvider, error) {
	var config AliyunCertConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("parse aliyun config failed: %w", err)
	}
	return NewAliyunProvider(&config)
}

// Name 返回服务商名称
func (p *AliyunProvider) Name() string {
	return "aliyun"
}

// ListCertificates 列出证书
func (p *AliyunProvider) ListCertificates(ctx context.Context) ([]*CertificateInfo, error) {
	request := requests.NewCommonRequest()
	request.Method = "POST"
	request.Scheme = "https"
	request.Domain = "cas.aliyuncs.com"
	request.Version = "2020-04-07"
	request.ApiName = "ListUserCertificateOrder"
	request.QueryParams["ShowSize"] = "100"
	request.QueryParams["CurrentPage"] = "1"

	response, err := p.client.ProcessCommonRequest(request)
	if err != nil {
		return nil, fmt.Errorf("list certificates failed: %w", err)
	}

	var result struct {
		CertificateOrderList []struct {
			CertificateId int64  `json:"CertificateId"`
			Domain        string `json:"Domain"`
			Sans          string `json:"Sans"`
			StartDate     string `json:"StartDate"`
			EndDate       string `json:"EndDate"`
			Issuer        string `json:"Issuer"`
		} `json:"CertificateOrderList"`
	}
	if err := json.Unmarshal(response.GetHttpContentBytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	var certs []*CertificateInfo
	for _, cert := range result.CertificateOrderList {
		notBefore, _ := time.Parse("2006-01-02", cert.StartDate)
		notAfter, _ := time.Parse("2006-01-02", cert.EndDate)

		certs = append(certs, &CertificateInfo{
			CertID:     fmt.Sprintf("%d", cert.CertificateId),
			Domain:     cert.Domain,
			SANDomains: parseSANs(cert.Sans),
			NotBefore:  notBefore,
			NotAfter:   notAfter,
			Issuer:     cert.Issuer,
		})
	}
	return certs, nil
}

// GetCertificate 获取证书详情
func (p *AliyunProvider) GetCertificate(ctx context.Context, certID string) (*CertificateDetail, error) {
	request := requests.NewCommonRequest()
	request.Method = "POST"
	request.Scheme = "https"
	request.Domain = "cas.aliyuncs.com"
	request.Version = "2020-04-07"
	request.ApiName = "GetUserCertificateDetail"
	request.QueryParams["CertId"] = certID

	response, err := p.client.ProcessCommonRequest(request)
	if err != nil {
		return nil, fmt.Errorf("get certificate detail failed: %w", err)
	}

	var result struct {
		Cert      string `json:"Cert"`
		Key       string `json:"Key"`
		Common    string `json:"Common"`
		Sans      string `json:"Sans"`
		StartDate string `json:"StartDate"`
		EndDate   string `json:"EndDate"`
		Issuer    string `json:"Issuer"`
	}
	if err := json.Unmarshal(response.GetHttpContentBytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	notBefore, _ := time.Parse("2006-01-02", result.StartDate)
	notAfter, _ := time.Parse("2006-01-02", result.EndDate)

	return &CertificateDetail{
		CertificateInfo: CertificateInfo{
			CertID:     certID,
			Domain:     result.Common,
			SANDomains: parseSANs(result.Sans),
			NotBefore:  notBefore,
			NotAfter:   notAfter,
			Issuer:     result.Issuer,
		},
		Certificate: result.Cert,
		PrivateKey:  result.Key,
	}, nil
}

// ApplyCertificate 申请免费证书
func (p *AliyunProvider) ApplyCertificate(ctx context.Context, domain string, sanDomains []string) (*ApplyResult, error) {
	// 阿里云免费证书申请流程：
	// 1. 创建证书请求 CreateCertificateForPackageRequest
	// 2. 获取DNS验证信息
	// 3. 等待验证完成

	// 创建证书订单
	request := requests.NewCommonRequest()
	request.Method = "POST"
	request.Scheme = "https"
	request.Domain = "cas.aliyuncs.com"
	request.Version = "2020-04-07"
	request.ApiName = "CreateCertificateForPackageRequest"
	request.QueryParams["Domain"] = domain
	request.QueryParams["ValidateType"] = "DNS"                 // DNS验证
	request.QueryParams["ProductCode"] = "digicert-free-1-free" // 免费DV证书产品代码

	response, err := p.client.ProcessCommonRequest(request)
	if err != nil {
		return nil, fmt.Errorf("create certificate request failed: %w", err)
	}

	// 打印完整的响应内容用于调试
	responseBody := response.GetHttpContentBytes()

	var result struct {
		OrderId   int64  `json:"OrderId"`
		RequestId string `json:"RequestId"`
		Code      string `json:"Code"`
		Message   string `json:"Message"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w, response: %s", err, string(responseBody))
	}

	// 检查是否有错误
	if result.Code != "" && result.Code != "200" {
		return nil, fmt.Errorf("aliyun api error: code=%s, message=%s, response: %s",
			result.Code, result.Message, string(responseBody))
	}

	if result.OrderId == 0 {
		return nil, fmt.Errorf("create certificate request failed: %s", string(responseBody))
	}

	// 获取DNS验证信息
	dnsInfo, err := p.GetDNSValidation(ctx, fmt.Sprintf("%d", result.OrderId))
	if err != nil {
		return nil, fmt.Errorf("get dns validation failed: %w", err)
	}

	return &ApplyResult{
		OrderID:        fmt.Sprintf("%d", result.OrderId),
		Status:         "pending_validation",
		ValidationType: "DNS",
		DNSRecord:      dnsInfo,
	}, nil
}

// GetDNSValidation 获取DNS验证信息
func (p *AliyunProvider) GetDNSValidation(ctx context.Context, orderID string) (*DNSValidationRecord, error) {
	request := requests.NewCommonRequest()
	request.Method = "POST"
	request.Scheme = "https"
	request.Domain = "cas.aliyuncs.com"
	request.Version = "2020-04-07"
	request.ApiName = "DescribeCertificateState"
	request.QueryParams["OrderId"] = orderID

	response, err := p.client.ProcessCommonRequest(request)
	if err != nil {
		return nil, fmt.Errorf("get certificate state failed: %w", err)
	}

	var result struct {
		Type        string `json:"Type"`        // 验证类型
		Domain      string `json:"Domain"`      // 验证域名
		RecordType  string `json:"RecordType"`  // 记录类型
		RecordValue string `json:"RecordValue"` // 记录值
		Certificate string `json:"Certificate"` // 证书内容(签发后)
		PrivateKey  string `json:"PrivateKey"`  // 私钥(签发后)
	}
	if err := json.Unmarshal(response.GetHttpContentBytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &DNSValidationRecord{
		RecordName:  "_dnsauth." + result.Domain,
		RecordType:  result.RecordType,
		RecordValue: result.RecordValue,
	}, nil
}

// CheckCertificateStatus 检查证书状态
func (p *AliyunProvider) CheckCertificateStatus(ctx context.Context, orderID string) (*CertificateStatus, error) {
	request := requests.NewCommonRequest()
	request.Method = "POST"
	request.Scheme = "https"
	request.Domain = "cas.aliyuncs.com"
	request.Version = "2020-04-07"
	request.ApiName = "DescribeCertificateState"
	request.QueryParams["OrderId"] = orderID

	response, err := p.client.ProcessCommonRequest(request)
	if err != nil {
		return nil, fmt.Errorf("get certificate state failed: %w", err)
	}

	var result struct {
		Type        string `json:"Type"`
		Certificate string `json:"Certificate"`
		PrivateKey  string `json:"PrivateKey"`
	}
	if err := json.Unmarshal(response.GetHttpContentBytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	status := &CertificateStatus{
		Status: "pending",
	}

	// 如果返回了证书内容，说明已签发
	if result.Certificate != "" {
		status.Status = "issued"
		status.Certificate = result.Certificate
		status.PrivateKey = result.PrivateKey
	}

	return status, nil
}

// DownloadCertificate 下载证书
func (p *AliyunProvider) DownloadCertificate(ctx context.Context, certID string) (*CertificateBundle, error) {
	detail, err := p.GetCertificate(ctx, certID)
	if err != nil {
		return nil, err
	}
	return &CertificateBundle{
		Certificate: detail.Certificate,
		PrivateKey:  detail.PrivateKey,
		CertChain:   detail.CertChain,
	}, nil
}

func parseSANs(sans string) []string {
	if sans == "" {
		return nil
	}
	var domains []string
	json.Unmarshal([]byte(sans), &domains)
	return domains
}
