package sslcert

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/deployer"
)

// 凭证加密密钥（与 internal/data/asset/host.go 保持一致）
var encryptionKey = []byte("opshub-enc-key-32-bytes-long!!!!")

// kubeconfig 加密密钥（与 kubernetes 插件保持一致）
const kubeConfigEncryptionKey = "opshub-k8s-encrypt-key-32bytes!!"

// HostGetter 主机信息获取器
type HostGetter struct {
	db *gorm.DB
}

// NewHostGetter 创建主机信息获取器
func NewHostGetter(db *gorm.DB) *HostGetter {
	return &HostGetter{db: db}
}

// Host 主机模型(简化版,与asset.Host对应)
type Host struct {
	ID           uint   `gorm:"primarykey"`
	Name         string `gorm:"type:varchar(100)"`
	IP           string `gorm:"column:ip;type:varchar(50)"`
	Port         int    `gorm:"type:int;default:22"`
	SSHUser      string `gorm:"column:ssh_user;type:varchar(50)"`
	CredentialID uint   `gorm:"column:credential_id"`
}

// TableName 表名
func (Host) TableName() string {
	return "hosts"
}

// Credential 凭证模型
type Credential struct {
	ID         uint   `gorm:"primarykey"`
	Name       string `gorm:"type:varchar(100)"`
	Type       string `gorm:"type:varchar(20)"` // password/key
	Username   string `gorm:"type:varchar(100)"`
	Password   string `gorm:"type:text"`
	PrivateKey string `gorm:"type:text"`
	Passphrase string `gorm:"type:varchar(255)"`
}

// TableName 表名
func (Credential) TableName() string {
	return "credentials"
}

// decrypt 解密凭证数据
func decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GetHost 获取主机信息
func (g *HostGetter) GetHost(ctx context.Context, hostID uint) (*deployer.HostInfo, error) {
	var host Host
	if err := g.db.WithContext(ctx).First(&host, hostID).Error; err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	info := &deployer.HostInfo{
		ID:       host.ID,
		Host:     host.IP,
		Port:     host.Port,
		Username: host.SSHUser,
	}

	// 获取凭证信息
	if host.CredentialID > 0 {
		var cred Credential
		if err := g.db.WithContext(ctx).First(&cred, host.CredentialID).Error; err == nil {
			if cred.Type == "password" {
				// 解密密码
				password, err := decrypt(cred.Password)
				if err != nil {
					return nil, fmt.Errorf("decrypt password failed: %w", err)
				}
				info.Password = password
			} else if cred.Type == "key" {
				// 解密私钥
				privateKey, err := decrypt(cred.PrivateKey)
				if err != nil {
					return nil, fmt.Errorf("decrypt private key failed: %w", err)
				}
				info.PrivateKey = []byte(privateKey)
				// 解密私钥密码
				if cred.Passphrase != "" {
					passphrase, err := decrypt(cred.Passphrase)
					if err != nil {
						return nil, fmt.Errorf("decrypt passphrase failed: %w", err)
					}
					info.Passphrase = passphrase
				}
			}
			// 如果凭证中有用户名，优先使用凭证的用户名
			if cred.Username != "" {
				info.Username = cred.Username
			}
		}
	}

	return info, nil
}

// ClusterGetter K8s集群信息获取器
type ClusterGetter struct {
	db *gorm.DB
}

// NewClusterGetter 创建K8s集群信息获取器
func NewClusterGetter(db *gorm.DB) *ClusterGetter {
	return &ClusterGetter{db: db}
}

// Cluster K8s集群模型(简化版)
type Cluster struct {
	ID         uint   `gorm:"primarykey"`
	Name       string `gorm:"type:varchar(100)"`
	KubeConfig string `gorm:"type:text"`
}

// TableName 表名
func (Cluster) TableName() string {
	return "k8s_clusters"
}

// decryptKubeConfig 解密 kubeconfig
func decryptKubeConfig(cipherText string) (string, error) {
	key := []byte(kubeConfigEncryptionKey)
	ciphertext, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GetClusterClient 获取K8s客户端
func (g *ClusterGetter) GetClusterClient(ctx context.Context, clusterID uint) (deployer.K8sClient, error) {
	var cluster Cluster
	if err := g.db.WithContext(ctx).First(&cluster, clusterID).Error; err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// 解密 kubeconfig
	kubeConfig, err := decryptKubeConfig(cluster.KubeConfig)
	if err != nil {
		// 如果解密失败，尝试直接使用（可能未加密）
		kubeConfig = cluster.KubeConfig
	}

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig failed: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client failed: %w", err)
	}

	return &K8sClientWrapper{clientset: clientset}, nil
}

// K8sClientWrapper K8s客户端包装器
type K8sClientWrapper struct {
	clientset *kubernetes.Clientset
}

// CreateOrUpdateSecret 创建或更新Secret
func (c *K8sClientWrapper) CreateOrUpdateSecret(ctx context.Context, namespace, name string, data map[string][]byte, labels map[string]string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeTLS,
		Data: data,
	}

	// 尝试获取已存在的Secret
	existing, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Secret存在,更新
		existing.Data = data
		existing.Labels = labels
		_, err = c.clientset.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{})
		return err
	}

	// Secret不存在,创建
	_, err = c.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// GetSecret 获取Secret
func (c *K8sClientWrapper) GetSecret(ctx context.Context, namespace, name string) (map[string][]byte, error) {
	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

// DeleteSecret 删除Secret
func (c *K8sClientWrapper) DeleteSecret(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// RolloutDeployment 滚动更新Deployment
func (c *K8sClientWrapper) RolloutDeployment(ctx context.Context, namespace, name string) error {
	// 通过更新annotation触发滚动更新
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["ssl-cert.opshub.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}
