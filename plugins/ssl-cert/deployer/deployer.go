package deployer

import (
	"context"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// Deployer 部署器接口
type Deployer interface {
	// Type 部署器类型
	Type() string
	// Deploy 部署证书
	Deploy(ctx context.Context, cert *model.CertBundle, config *model.DeployConfig) error
	// Test 测试部署配置
	Test(ctx context.Context, config *model.DeployConfig) error
}

// Factory 部署器工厂
type Factory struct{}

// NewFactory 创建工厂
func NewFactory() *Factory {
	return &Factory{}
}

// Create 根据配置创建部署器
func (f *Factory) Create(deployType string, deps *Dependencies) (Deployer, error) {
	switch deployType {
	case model.DeployTypeNginxSSH:
		return NewNginxSSHDeployer(deps), nil
	case model.DeployTypeK8sSecret:
		return NewK8sSecretDeployer(deps), nil
	default:
		return nil, ErrUnsupportedDeployType
	}
}

// Dependencies 部署器依赖
type Dependencies struct {
	HostGetter    HostGetter
	ClusterGetter ClusterGetter
}

// HostGetter 主机信息获取接口
type HostGetter interface {
	GetHost(ctx context.Context, hostID uint) (*HostInfo, error)
}

// ClusterGetter K8s集群信息获取接口
type ClusterGetter interface {
	GetClusterClient(ctx context.Context, clusterID uint) (K8sClient, error)
}

// HostInfo 主机信息
type HostInfo struct {
	ID         uint
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey []byte
	Passphrase string
}

// K8sClient K8s客户端接口
type K8sClient interface {
	CreateOrUpdateSecret(ctx context.Context, namespace, name string, data map[string][]byte, labels map[string]string) error
	GetSecret(ctx context.Context, namespace, name string) (map[string][]byte, error)
	DeleteSecret(ctx context.Context, namespace, name string) error
	RolloutDeployment(ctx context.Context, namespace, name string) error
}

// ErrUnsupportedDeployType 不支持的部署类型错误
var ErrUnsupportedDeployType = &DeployError{Message: "unsupported deploy type"}

// DeployError 部署错误
type DeployError struct {
	Message string
}

func (e *DeployError) Error() string {
	return e.Message
}
