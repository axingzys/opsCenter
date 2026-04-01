package dns

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// AWSRoute53Provider AWS Route53 DNS服务商
type AWSRoute53Provider struct {
	cfg    *model.AWSRoute53Config
	client *route53.Client
}

// NewAWSRoute53Provider 创建AWS Route53 DNS Provider
func NewAWSRoute53Provider(cfg *model.AWSRoute53Config) (*AWSRoute53Provider, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config failed: %w", err)
	}

	client := route53.NewFromConfig(awsCfg)
	return &AWSRoute53Provider{
		cfg:    cfg,
		client: client,
	}, nil
}

// Name 返回服务商名称
func (p *AWSRoute53Provider) Name() string {
	return "aws_route53"
}

// CreateTXTRecord 创建TXT记录
func (p *AWSRoute53Provider) CreateTXTRecord(ctx context.Context, domain, name, value string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Hosted Zone ID
	hostedZoneID, err := p.getHostedZoneID(ctx, rootDomain)
	if err != nil {
		return err
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, "."+rootDomain) {
		recordName = name + "." + rootDomain
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &hostedZoneID,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: &recordName,
						Type: types.RRTypeTxt,
						TTL:  int64Ptr(600),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: stringPtrAws("\"" + value + "\""),
							},
						},
					},
				},
			},
		},
	}

	_, err = p.client.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		return fmt.Errorf("create TXT record failed: %w", err)
	}
	return nil
}

// DeleteTXTRecord 删除TXT记录
func (p *AWSRoute53Provider) DeleteTXTRecord(ctx context.Context, domain, name string) error {
	rootDomain := ExtractRootDomain(domain)

	// 获取Hosted Zone ID
	hostedZoneID, err := p.getHostedZoneID(ctx, rootDomain)
	if err != nil {
		return err
	}

	// 构建完整记录名
	recordName := name
	if !strings.HasSuffix(name, "."+rootDomain) {
		recordName = name + "." + rootDomain
	}

	// 先查询记录
	listInput := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    &hostedZoneID,
		StartRecordName: &recordName,
		StartRecordType: types.RRTypeTxt,
		MaxItems:        int32PtrAws(1),
	}

	listOutput, err := p.client.ListResourceRecordSets(ctx, listInput)
	if err != nil {
		return fmt.Errorf("list resource record sets failed: %w", err)
	}

	// 删除匹配的记录
	for _, recordSet := range listOutput.ResourceRecordSets {
		if recordSet.Name != nil && strings.TrimSuffix(*recordSet.Name, ".") == recordName && recordSet.Type == types.RRTypeTxt {
			deleteInput := &route53.ChangeResourceRecordSetsInput{
				HostedZoneId: &hostedZoneID,
				ChangeBatch: &types.ChangeBatch{
					Changes: []types.Change{
						{
							Action:            types.ChangeActionDelete,
							ResourceRecordSet: &recordSet,
						},
					},
				},
			}

			_, err = p.client.ChangeResourceRecordSets(ctx, deleteInput)
			if err != nil {
				return fmt.Errorf("delete TXT record failed: %w", err)
			}
		}
	}
	return nil
}

// TestConnection 测试连接
func (p *AWSRoute53Provider) TestConnection(ctx context.Context) error {
	input := &route53.ListHostedZonesInput{
		MaxItems: int32PtrAws(1),
	}

	_, err := p.client.ListHostedZones(ctx, input)
	if err != nil {
		return fmt.Errorf("test connection failed: %w", err)
	}
	return nil
}

// getHostedZoneID 获取Hosted Zone ID
func (p *AWSRoute53Provider) getHostedZoneID(ctx context.Context, domain string) (string, error) {
	dnsName := domain
	if !strings.HasSuffix(dnsName, ".") {
		dnsName = dnsName + "."
	}

	input := &route53.ListHostedZonesByNameInput{
		DNSName:  &dnsName,
		MaxItems: int32PtrAws(1),
	}

	output, err := p.client.ListHostedZonesByName(ctx, input)
	if err != nil {
		return "", fmt.Errorf("list hosted zones failed: %w", err)
	}

	for _, zone := range output.HostedZones {
		if zone.Name != nil && strings.TrimSuffix(*zone.Name, ".") == domain {
			// Zone ID格式: /hostedzone/XXXXX
			zoneID := *zone.Id
			if strings.HasPrefix(zoneID, "/hostedzone/") {
				zoneID = strings.TrimPrefix(zoneID, "/hostedzone/")
			}
			return zoneID, nil
		}
	}

	return "", fmt.Errorf("hosted zone not found for domain: %s", domain)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtrAws(s string) *string {
	return &s
}

func int32PtrAws(i int32) *int32 {
	return &i
}
