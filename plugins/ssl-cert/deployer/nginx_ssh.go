package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	sshclient "github.com/ydcloud-dy/opshub/pkg/ssh"
	"github.com/ydcloud-dy/opshub/plugins/ssl-cert/model"
)

// NginxSSHDeployer Nginx SSH部署器
type NginxSSHDeployer struct {
	deps *Dependencies
}

// NewNginxSSHDeployer 创建Nginx SSH部署器
func NewNginxSSHDeployer(deps *Dependencies) *NginxSSHDeployer {
	return &NginxSSHDeployer{deps: deps}
}

// Type 返回部署器类型
func (d *NginxSSHDeployer) Type() string {
	return model.DeployTypeNginxSSH
}

// Deploy 部署证书到Nginx
func (d *NginxSSHDeployer) Deploy(ctx context.Context, cert *model.CertBundle, config *model.DeployConfig) error {
	// 解析配置
	var nginxConfig model.NginxSSHConfig
	if err := json.Unmarshal([]byte(config.TargetConfig), &nginxConfig); err != nil {
		return fmt.Errorf("parse nginx config failed: %w", err)
	}

	// 获取主机信息
	hostInfo, err := d.deps.HostGetter.GetHost(ctx, nginxConfig.HostID)
	if err != nil {
		return fmt.Errorf("get host info failed: %w", err)
	}

	// 创建SSH客户端
	client, err := sshclient.NewClient(
		hostInfo.Host,
		hostInfo.Port,
		hostInfo.Username,
		hostInfo.Password,
		hostInfo.PrivateKey,
		hostInfo.Passphrase,
	)
	if err != nil {
		return fmt.Errorf("create ssh client failed: %w", err)
	}
	defer client.Close()

	// 确保目录存在
	certDir := filepath.Dir(nginxConfig.CertPath)
	if _, err := client.Execute(fmt.Sprintf("mkdir -p %s", certDir)); err != nil {
		return fmt.Errorf("create cert directory failed: %w", err)
	}

	// 备份旧证书
	if nginxConfig.BackupEnabled {
		backupPath := nginxConfig.BackupPath
		if backupPath == "" {
			backupPath = certDir + "/backup"
		}
		timestamp := time.Now().Format("20060102150405")

		// 创建备份目录
		if _, err := client.Execute(fmt.Sprintf("mkdir -p %s", backupPath)); err != nil {
			return fmt.Errorf("create backup directory failed: %w", err)
		}

		// 备份证书文件（保留原文件名和扩展名）
		certFilename := filepath.Base(nginxConfig.CertPath)
		certExt := filepath.Ext(certFilename)
		certNameWithoutExt := strings.TrimSuffix(certFilename, certExt)
		certBackupName := fmt.Sprintf("%s_%s%s", certNameWithoutExt, timestamp, certExt)
		if _, err := client.Execute(fmt.Sprintf("cp %s %s/%s 2>/dev/null || true", nginxConfig.CertPath, backupPath, certBackupName)); err != nil {
			// 备份失败不影响部署
		}

		// 备份私钥文件（保留原文件名和扩展名）
		keyFilename := filepath.Base(nginxConfig.KeyPath)
		keyExt := filepath.Ext(keyFilename)
		keyNameWithoutExt := strings.TrimSuffix(keyFilename, keyExt)
		keyBackupName := fmt.Sprintf("%s_%s%s", keyNameWithoutExt, timestamp, keyExt)
		if _, err := client.Execute(fmt.Sprintf("cp %s %s/%s 2>/dev/null || true", nginxConfig.KeyPath, backupPath, keyBackupName)); err != nil {
			// 备份失败不影响部署
		}
	}

	// 上传证书
	if err := client.UploadFromReader(strings.NewReader(cert.Certificate), nginxConfig.CertPath); err != nil {
		return fmt.Errorf("upload certificate failed: %w", err)
	}

	// 上传私钥
	if err := client.UploadFromReader(strings.NewReader(cert.PrivateKey), nginxConfig.KeyPath); err != nil {
		return fmt.Errorf("upload private key failed: %w", err)
	}

	// 上传证书链(如果有)
	if cert.CertChain != "" && nginxConfig.ChainPath != "" {
		if err := client.UploadFromReader(strings.NewReader(cert.CertChain), nginxConfig.ChainPath); err != nil {
			return fmt.Errorf("upload cert chain failed: %w", err)
		}
	}

	// 设置文件权限
	if _, err := client.Execute(fmt.Sprintf("chmod 644 %s", nginxConfig.CertPath)); err != nil {
		return fmt.Errorf("set cert permission failed: %w", err)
	}
	if _, err := client.Execute(fmt.Sprintf("chmod 600 %s", nginxConfig.KeyPath)); err != nil {
		return fmt.Errorf("set key permission failed: %w", err)
	}

	// 测试Nginx配置
	nginxBin := nginxConfig.NginxBin
	if nginxBin == "" {
		nginxBin = "nginx"
	}
	if _, err := client.Execute(fmt.Sprintf("%s -t", nginxBin)); err != nil {
		return fmt.Errorf("nginx config test failed: %w", err)
	}

	// 重载Nginx
	reloadCmd := nginxConfig.ReloadCommand
	if reloadCmd == "" {
		reloadCmd = fmt.Sprintf("%s -s reload", nginxBin)
	}
	if _, err := client.Execute(reloadCmd); err != nil {
		return fmt.Errorf("nginx reload failed: %w", err)
	}

	return nil
}

// Test 测试部署配置
func (d *NginxSSHDeployer) Test(ctx context.Context, config *model.DeployConfig) error {
	// 解析配置
	var nginxConfig model.NginxSSHConfig
	if err := json.Unmarshal([]byte(config.TargetConfig), &nginxConfig); err != nil {
		return fmt.Errorf("parse nginx config failed: %w", err)
	}

	// 获取主机信息
	hostInfo, err := d.deps.HostGetter.GetHost(ctx, nginxConfig.HostID)
	if err != nil {
		return fmt.Errorf("get host info failed: %w", err)
	}

	// 创建SSH客户端并测试连接
	client, err := sshclient.NewClient(
		hostInfo.Host,
		hostInfo.Port,
		hostInfo.Username,
		hostInfo.Password,
		hostInfo.PrivateKey,
		hostInfo.Passphrase,
	)
	if err != nil {
		return fmt.Errorf("create ssh client failed: %w", err)
	}
	defer client.Close()

	// 测试SSH连接
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("ssh connection test failed: %w", err)
	}

	// 测试Nginx是否存在
	nginxBin := nginxConfig.NginxBin
	if nginxBin == "" {
		nginxBin = "nginx"
	}
	if _, err := client.Execute(fmt.Sprintf("which %s", nginxBin)); err != nil {
		return fmt.Errorf("nginx not found: %w", err)
	}

	// 测试目录是否可写
	certDir := filepath.Dir(nginxConfig.CertPath)
	if _, err := client.Execute(fmt.Sprintf("test -w %s || mkdir -p %s", certDir, certDir)); err != nil {
		return fmt.Errorf("cert directory not writable: %w", err)
	}

	return nil
}
