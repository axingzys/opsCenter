package cloud

import (
	"fmt"
)

// Factory 云厂商证书服务工厂
type Factory struct{}

// NewFactory 创建工厂
func NewFactory() *Factory {
	return &Factory{}
}

// Create 根据云账号创建Provider
func (f *Factory) Create(provider, accessKey, secretKey string) (Provider, error) {
	switch provider {
	case "aliyun":
		return NewAliyunProvider(&AliyunCertConfig{
			AccessKeyID:     accessKey,
			AccessKeySecret: secretKey,
		})
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

// CreateFromConfig 从配置创建Provider
func (f *Factory) CreateFromConfig(provider, configJSON string) (Provider, error) {
	switch provider {
	case "aliyun":
		return NewAliyunProviderFromJSON(configJSON)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}
