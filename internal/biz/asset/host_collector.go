package asset

import (
	"context"
	"fmt"

	"github.com/ydcloud-dy/opshub/pkg/collector"
)

type HostCollector interface {
	Mode() string
	Test(ctx context.Context, host *Host) error
	Collect(ctx context.Context, host *Host) (*collector.SystemInfo, error)
}

type sshHostCollector struct {
	useCase *HostUseCase
}

type winRMHostCollector struct {
	useCase *HostUseCase
}

type agentHostCollector struct {
	useCase *HostUseCase
}

func (uc *HostUseCase) effectiveManagementMode(host *Host) string {
	return NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID)
}

func (uc *HostUseCase) effectiveManagementPort(host *Host) int {
	return NormalizeManagementPort(uc.effectiveManagementMode(host), host.ManagementPort, host.Port)
}

func (uc *HostUseCase) effectiveManagementCredentialID(host *Host) uint {
	return NormalizeManagementCredentialID(uc.effectiveManagementMode(host), host.ManagementCredentialID, host.CredentialID)
}

func (uc *HostUseCase) selectCollector(host *Host) HostCollector {
	switch uc.effectiveManagementMode(host) {
	case ManagementModeWinRM:
		return &winRMHostCollector{useCase: uc}
	case ManagementModeAgent:
		return &agentHostCollector{useCase: uc}
	default:
		return &sshHostCollector{useCase: uc}
	}
}

func (uc *HostUseCase) resolveSSHHost(host *Host) (*Host, uint, error) {
	credentialID := uc.effectiveManagementCredentialID(host)
	if credentialID == 0 {
		return nil, 0, fmt.Errorf("主机未配置 SSH 凭证")
	}
	if host.SSHUser == "" {
		return nil, 0, fmt.Errorf("主机未配置 SSH 用户名")
	}

	target := *host
	target.Port = uc.effectiveManagementPort(host)
	if target.Port == 0 {
		target.Port = 22
	}

	return &target, credentialID, nil
}

func (c *sshHostCollector) Mode() string {
	return ManagementModeSSH
}

func (c *sshHostCollector) Test(ctx context.Context, host *Host) error {
	target, credentialID, err := c.useCase.resolveSSHHost(host)
	if err != nil {
		return err
	}

	credential, err := c.useCase.credentialRepo.GetByIDDecrypted(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("获取凭证失败: %w", err)
	}

	sshClient, err := c.useCase.createSSHClient(target, credential)
	if err != nil {
		return fmt.Errorf("创建SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	if err := sshClient.TestConnection(); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}

	return nil
}

func (c *sshHostCollector) Collect(ctx context.Context, host *Host) (*collector.SystemInfo, error) {
	target, credentialID, err := c.useCase.resolveSSHHost(host)
	if err != nil {
		return nil, err
	}

	credential, err := c.useCase.credentialRepo.GetByIDDecrypted(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("获取凭证失败: %w", err)
	}

	sshClient, err := c.useCase.createSSHClient(target, credential)
	if err != nil {
		return nil, fmt.Errorf("创建SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	return collector.NewCollector(sshClient).CollectAll()
}

func (c *winRMHostCollector) Mode() string {
	return ManagementModeWinRM
}

func (c *agentHostCollector) Mode() string {
	return ManagementModeAgent
}

func (c *agentHostCollector) Test(ctx context.Context, host *Host) error {
	if host.AgentID == "" || host.AgentLastHeartbeatAt == nil {
		return fmt.Errorf("Agent 未注册或尚未上报")
	}
	if !IsAgentHeartbeatFresh(host) {
		return fmt.Errorf("Agent 心跳超时")
	}
	return nil
}

func (c *agentHostCollector) Collect(ctx context.Context, host *Host) (*collector.SystemInfo, error) {
	if err := c.Test(ctx, host); err != nil {
		return nil, err
	}
	return SystemInfoFromHost(host)
}
