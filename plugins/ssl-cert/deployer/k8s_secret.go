package deployer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// K8sSecretDeployer K8s Secret部署器
type K8sSecretDeployer struct {
	deps *Dependencies
}

// NewK8sSecretDeployer 创建K8s Secret部署器
func NewK8sSecretDeployer(deps *Dependencies) *K8sSecretDeployer {
	return &K8sSecretDeployer{deps: deps}
}

// Type 返回部署器类型
func (d *K8sSecretDeployer) Type() string {
	return model.DeployTypeK8sSecret
}

// Deploy 部署证书到K8s Secret
func (d *K8sSecretDeployer) Deploy(ctx context.Context, cert *model.CertBundle, config *model.DeployConfig) error {
	// 解析配置
	var k8sConfig model.K8sSecretConfig
	if err := json.Unmarshal([]byte(config.TargetConfig), &k8sConfig); err != nil {
		return fmt.Errorf("parse k8s config failed: %w", err)
	}

	// 获取K8s客户端
	client, err := d.deps.ClusterGetter.GetClusterClient(ctx, k8sConfig.ClusterID)
	if err != nil {
		return fmt.Errorf("get k8s client failed: %w", err)
	}

	// 准备Secret数据
	certKey := k8sConfig.CertKey
	if certKey == "" {
		certKey = "tls.crt"
	}
	keyKey := k8sConfig.KeyKey
	if keyKey == "" {
		keyKey = "tls.key"
	}

	data := map[string][]byte{
		certKey: []byte(cert.Certificate),
		keyKey:  []byte(cert.PrivateKey),
	}

	// 如果有证书链，也加入
	if cert.CertChain != "" {
		// 将证书链追加到证书后面
		data[certKey] = []byte(cert.Certificate + "\n" + cert.CertChain)
	}

	// 准备标签
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "opshub-ssl-cert",
		"ssl-cert.opshub.io/type":      "tls",
	}
	for k, v := range k8sConfig.Labels {
		labels[k] = v
	}

	// 创建或更新Secret
	if err := client.CreateOrUpdateSecret(ctx, k8sConfig.Namespace, k8sConfig.SecretName, data, labels); err != nil {
		return fmt.Errorf("create or update secret failed: %w", err)
	}

	// 触发滚动更新
	if k8sConfig.TriggerRollout && len(k8sConfig.Deployments) > 0 {
		for _, deployment := range k8sConfig.Deployments {
			if err := client.RolloutDeployment(ctx, k8sConfig.Namespace, deployment); err != nil {
				// 滚动更新失败只记录警告，不影响部署结果
				// TODO: 添加日志
			}
		}
	}

	return nil
}

// Test 测试部署配置
func (d *K8sSecretDeployer) Test(ctx context.Context, config *model.DeployConfig) error {
	// 解析配置
	var k8sConfig model.K8sSecretConfig
	if err := json.Unmarshal([]byte(config.TargetConfig), &k8sConfig); err != nil {
		return fmt.Errorf("parse k8s config failed: %w", err)
	}

	// 获取K8s客户端
	client, err := d.deps.ClusterGetter.GetClusterClient(ctx, k8sConfig.ClusterID)
	if err != nil {
		return fmt.Errorf("get k8s client failed: %w", err)
	}

	// 测试是否能访问命名空间
	// 尝试获取Secret（即使不存在也算成功）
	_, err = client.GetSecret(ctx, k8sConfig.Namespace, k8sConfig.SecretName)
	if err != nil {
		// 如果是"not found"错误，说明连接正常，只是Secret不存在
		if !isNotFoundError(err) {
			return fmt.Errorf("k8s connection test failed: %w", err)
		}
	}

	return nil
}

// isNotFoundError 判断是否是NotFound错误
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// 简单判断错误消息中是否包含"not found"
	errMsg := err.Error()
	return contains(errMsg, "not found") || contains(errMsg, "NotFound")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
