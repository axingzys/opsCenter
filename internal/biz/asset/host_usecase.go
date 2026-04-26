// Copyright (c) 2026 DYCloud J.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	"github.com/xuri/excelize/v2"
	"github.com/ydcloud-dy/opshub/internal/conf"
	sshclient "github.com/ydcloud-dy/opshub/pkg/ssh"
	"github.com/ydcloud-dy/opshub/pkg/utils"
)

type HostUseCase struct {
	hostRepo       HostRepo
	credentialRepo CredentialRepo
	groupRepo      AssetGroupRepo
	cloudRepo      CloudAccountRepo
	agentRepo      AssetAgentRepo
	inventoryRepo  AssetHostInventoryRepo
	promCfg        conf.PrometheusConfig
	agentSecretKey []byte
}

func NewHostUseCase(hostRepo HostRepo, credentialRepo CredentialRepo, groupRepo AssetGroupRepo, cloudRepo CloudAccountRepo, agentRepo AssetAgentRepo, inventoryRepo AssetHostInventoryRepo, promCfg conf.PrometheusConfig, agentSecretKey []byte) *HostUseCase {
	return &HostUseCase{
		hostRepo:       hostRepo,
		credentialRepo: credentialRepo,
		groupRepo:      groupRepo,
		cloudRepo:      cloudRepo,
		agentRepo:      agentRepo,
		inventoryRepo:  inventoryRepo,
		promCfg:        promCfg,
		agentSecretKey: agentSecretKey,
	}
}

// Create 创建主机
func (uc *HostUseCase) Create(ctx context.Context, req *HostRequest) (*Host, error) {
	host := req.ToModel()
	uc.applyManagementConfig(host, req)
	if err := uc.validateCredentialBindings(ctx, host); err != nil {
		return nil, err
	}

	if err := uc.hostRepo.CreateOrUpdate(ctx, host); err != nil {
		return nil, err
	}

	return host, nil
}

// Update 更新主机
func (uc *HostUseCase) Update(ctx context.Context, req *HostRequest) error {
	host, err := uc.hostRepo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("主机不存在")
	}

	// 检查IP是否被其他主机使用
	existHost, err := uc.hostRepo.GetByIP(ctx, req.IP)
	if err == nil && existHost != nil && existHost.ID != req.ID {
		return fmt.Errorf("IP地址 %s 已被其他主机使用", req.IP)
	}

	host.Name = req.Name
	host.GroupID = req.GroupID
	host.Type = req.Type
	host.CloudProvider = req.CloudProvider
	host.CloudInstanceID = req.CloudInstanceID
	host.CloudAccountID = req.CloudAccountID
	if req.OSType != "" {
		host.OSType = req.OSType
	}
	host.IP = req.IP
	uc.applyManagementConfig(host, req)
	host.DesktopEnabled = req.DesktopEnabled
	if req.DesktopProtocol != "" {
		host.DesktopProtocol = req.DesktopProtocol
	}
	if req.DesktopPort > 0 {
		host.DesktopPort = req.DesktopPort
	}
	host.DesktopCredentialID = req.DesktopCredentialID
	if req.DesktopSecurity != "" {
		host.DesktopSecurity = req.DesktopSecurity
	}
	host.DesktopIgnoreCert = req.DesktopIgnoreCert
	host.Tags = req.Tags
	host.Description = req.Description

	if err := uc.validateCredentialBindings(ctx, host); err != nil {
		return err
	}

	return uc.hostRepo.Update(ctx, host)
}

func (uc *HostUseCase) applyManagementConfig(host *Host, req *HostRequest) {
	previousMode := uc.effectiveManagementMode(host)
	existingSSHUser := strings.TrimSpace(host.SSHUser)
	existingCredentialID := host.CredentialID
	existingManagementCredentialID := host.ManagementCredentialID
	osType := host.OSType
	if osType == "" {
		osType = OSTypeLinux
	}

	requestedSSHPort := req.Port
	if requestedSSHPort == 0 {
		requestedSSHPort = host.Port
	}
	if requestedSSHPort == 0 {
		requestedSSHPort = 22
	}

	mode := NormalizeManagementMode(osType, req.ManagementMode, req.CredentialID, req.ManagementCredentialID)
	managementPort := NormalizeManagementPort(mode, req.ManagementPort, requestedSSHPort)
	managementCredentialID := NormalizeManagementCredentialID(mode, req.ManagementCredentialID, req.CredentialID)
	if mode == ManagementModeAgent && managementCredentialID == 0 {
		managementCredentialID = NormalizeManagementCredentialID(ManagementModeAgent, existingManagementCredentialID, existingCredentialID)
	}

	host.ManagementMode = mode
	host.ManagementPort = managementPort
	host.ManagementCredentialID = managementCredentialID

	if osType == OSTypeLinux || mode == ManagementModeSSH {
		sshUser := strings.TrimSpace(req.SSHUser)
		if sshUser == "" {
			sshUser = existingSSHUser
		}
		host.SSHUser = sshUser
		host.Port = requestedSSHPort
		host.CredentialID = managementCredentialID
	} else {
		host.SSHUser = ""
		host.Port = 22
		host.CredentialID = 0
	}

	if host.CollectStatus == "" || previousMode != mode {
		host.CollectStatus = DefaultCollectStatus(mode)
		host.CollectError = ""
		if mode == ManagementModeNone {
			host.Status = -1
		}
	}
}

func normalizeCredentialProtocol(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		return ManagementModeSSH
	}
	return protocol
}

func (uc *HostUseCase) validateCredentialBindings(ctx context.Context, host *Host) error {
	mode := uc.effectiveManagementMode(host)
	managementCredentialID := uc.effectiveManagementCredentialID(host)
	if mode == ManagementModeWinRM && managementCredentialID == 0 {
		return fmt.Errorf("WinRM 管理方式需要选择 WinRM 凭证")
	}
	if managementCredentialID > 0 {
		credential, err := uc.credentialRepo.GetByID(ctx, managementCredentialID)
		if err != nil {
			return fmt.Errorf("管理凭证不存在")
		}

		expectedProtocol := ""
		switch mode {
		case ManagementModeSSH:
			expectedProtocol = ManagementModeSSH
		case ManagementModeWinRM:
			expectedProtocol = ManagementModeWinRM
		}

		if expectedProtocol != "" && normalizeCredentialProtocol(credential.Protocol) != expectedProtocol {
			return fmt.Errorf("%s 管理方式需要 %s 凭证", strings.ToUpper(mode), strings.ToUpper(expectedProtocol))
		}
	}

	if host.DesktopCredentialID > 0 {
		credential, err := uc.credentialRepo.GetByID(ctx, host.DesktopCredentialID)
		if err != nil {
			return fmt.Errorf("桌面凭证不存在")
		}
		if normalizeCredentialProtocol(credential.Protocol) != "rdp" {
			return fmt.Errorf("桌面访问需要 RDP 凭证")
		}
	}

	return nil
}

// Delete 删除主机
func (uc *HostUseCase) Delete(ctx context.Context, id uint) error {
	if err := uc.cleanupHostAgentArtifacts(ctx, id); err != nil {
		return err
	}
	return uc.hostRepo.Delete(ctx, id)
}

// GetByID 根据ID获取主机详情
func (uc *HostUseCase) GetByID(ctx context.Context, id uint) (*HostInfoVO, error) {
	host, err := uc.hostRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	vo := uc.toInfoVO(host)

	// 加载分组信息
	if host.GroupID > 0 {
		group, err := uc.groupRepo.GetByID(ctx, host.GroupID)
		if err == nil && group != nil {
			vo.GroupName = group.Name
		}
	}

	// 加载凭证信息
	if host.CredentialID > 0 {
		credential, err := uc.credentialRepo.GetByID(ctx, host.CredentialID)
		if err == nil && credential != nil {
			vo.Credential = uc.toCredentialVO(credential)
		}
	}
	if host.ManagementCredentialID > 0 {
		credential, err := uc.credentialRepo.GetByID(ctx, host.ManagementCredentialID)
		if err == nil && credential != nil {
			vo.ManagementCredential = uc.toCredentialVO(credential)
		}
	}
	if host.DesktopCredentialID > 0 {
		credential, err := uc.credentialRepo.GetByID(ctx, host.DesktopCredentialID)
		if err == nil && credential != nil {
			vo.DesktopCredential = uc.toCredentialVO(credential)
		}
	}

	return vo, nil
}

// List 分页查询主机列表
func (uc *HostUseCase) List(ctx context.Context, page, pageSize int, keyword string, groupID *uint, accessibleHostIDs []uint, status *int) ([]*HostInfoVO, int64, error) {
	// 如果指定了分组ID，获取所有子孙分组ID
	var groupIDs []uint
	if groupID != nil && *groupID > 0 {
		groupIDs = append(groupIDs, *groupID)
		// 获取所有子孙分组ID
		descendantIDs, err := uc.groupRepo.GetDescendantIDs(ctx, *groupID)
		if err == nil {
			groupIDs = append(groupIDs, descendantIDs...)
		}
	}

	hosts, total, err := uc.hostRepo.List(ctx, page, pageSize, keyword, groupIDs, accessibleHostIDs, status)
	if err != nil {
		return nil, 0, err
	}

	var vos []*HostInfoVO
	for _, host := range hosts {
		vo := uc.toInfoVO(host)

		// 加载分组信息
		if host.GroupID > 0 {
			group, err := uc.groupRepo.GetByID(ctx, host.GroupID)
			if err == nil && group != nil {
				vo.GroupName = group.Name
			}
		}

		// 加载凭证信息
		if host.CredentialID > 0 {
			credential, err := uc.credentialRepo.GetByID(ctx, host.CredentialID)
			if err == nil && credential != nil {
				vo.Credential = uc.toCredentialVO(credential)
			}
		}
		if host.ManagementCredentialID > 0 {
			credential, err := uc.credentialRepo.GetByID(ctx, host.ManagementCredentialID)
			if err == nil && credential != nil {
				vo.ManagementCredential = uc.toCredentialVO(credential)
			}
		}
		if host.DesktopCredentialID > 0 {
			credential, err := uc.credentialRepo.GetByID(ctx, host.DesktopCredentialID)
			if err == nil && credential != nil {
				vo.DesktopCredential = uc.toCredentialVO(credential)
			}
		}

		vos = append(vos, vo)
	}

	return vos, total, nil
}

// toInfoVO 转换为InfoVO
func (uc *HostUseCase) toInfoVO(host *Host) *HostInfoVO {
	status := host.Status
	managementMode := uc.effectiveManagementMode(host)
	collectStatus := host.CollectStatus
	collectError := host.CollectError
	if collectStatus == "" {
		collectStatus = DefaultCollectStatus(managementMode)
	}
	if managementMode == ManagementModeAgent {
		status, collectStatus, collectError = AgentRuntimeState(host)
	}

	statusText := "未知"
	if status == 1 {
		statusText = "在线"
	} else if status == 0 {
		statusText = "离线"
	}

	typeText := "自建主机"
	if host.Type == "cloud" {
		typeText = "云主机"
	}

	var cloudProviderText string
	if host.CloudProvider != "" {
		switch host.CloudProvider {
		case "aliyun":
			cloudProviderText = "阿里云"
		case "tencent":
			cloudProviderText = "腾讯云"
		case "aws":
			cloudProviderText = "AWS"
		case "huawei":
			cloudProviderText = "华为云"
		default:
			cloudProviderText = host.CloudProvider
		}
	}

	var tags []string
	if host.Tags != "" {
		tags = strings.Split(host.Tags, ",")
	}

	var lastSeen string
	if host.LastSeen != nil {
		lastSeen = host.LastSeen.Format("2006-01-02 15:04:05")
	}
	var lastCollectAt string
	if host.LastCollectAt != nil {
		lastCollectAt = host.LastCollectAt.Format("2006-01-02 15:04:05")
	}
	var agentLastHeartbeatAt string
	if host.AgentLastHeartbeatAt != nil {
		agentLastHeartbeatAt = host.AgentLastHeartbeatAt.Format("2006-01-02 15:04:05")
	}
	var agentLastReportAt string
	if host.AgentLastReportAt != nil {
		agentLastReportAt = host.AgentLastReportAt.Format("2006-01-02 15:04:05")
	}

	return &HostInfoVO{
		ID:                     host.ID,
		Name:                   host.Name,
		GroupID:                host.GroupID,
		Type:                   host.Type,
		TypeText:               typeText,
		CloudProvider:          host.CloudProvider,
		CloudProviderText:      cloudProviderText,
		CloudInstanceID:        host.CloudInstanceID,
		OSType:                 host.OSType,
		SSHUser:                host.SSHUser,
		IP:                     host.IP,
		Port:                   host.Port,
		CredentialID:           host.CredentialID,
		ManagementMode:         managementMode,
		ManagementModeText:     ManagementModeText(managementMode),
		ManagementPort:         uc.effectiveManagementPort(host),
		ManagementCredentialID: uc.effectiveManagementCredentialID(host),
		DesktopEnabled:         host.DesktopEnabled,
		DesktopProtocol:        host.DesktopProtocol,
		DesktopPort:            host.DesktopPort,
		DesktopCredentialID:    host.DesktopCredentialID,
		DesktopSecurity:        host.DesktopSecurity,
		DesktopIgnoreCert:      host.DesktopIgnoreCert,
		Tags:                   tags,
		Description:            host.Description,
		Status:                 status,
		StatusText:             statusText,
		LastSeen:               lastSeen,
		CollectStatus:          collectStatus,
		CollectStatusText:      CollectStatusText(collectStatus),
		CollectError:           collectError,
		LastCollectAt:          lastCollectAt,
		PrimaryPrivateIP:       host.PrimaryPrivateIP,
		PrimaryPublicIP:        host.PrimaryPublicIP,
		AgentID:                host.AgentID,
		AgentVersion:           host.AgentVersion,
		AgentLastHeartbeatAt:   agentLastHeartbeatAt,
		AgentPort:              host.AgentPort,
		AgentLastReportAt:      agentLastReportAt,
		AgentLastError:         host.AgentLastError,
		OS:                     host.OS,
		Kernel:                 host.Kernel,
		Arch:                   host.Arch,
		CreateTime:             host.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdateTime:             host.UpdatedAt.Format("2006-01-02 15:04:05"),
		// 扩展信息
		CPUCores:    host.CPUCores,
		CPUUsage:    host.CPUUsage,
		MemoryTotal: host.MemoryTotal,
		MemoryUsed:  host.MemoryUsed,
		MemoryUsage: host.MemoryUsage,
		DiskTotal:   host.DiskTotal,
		DiskUsed:    host.DiskUsed,
		DiskUsage:   host.DiskUsage,
		Uptime:      host.Uptime,
		Hostname:    host.Hostname,
	}
}

// CollectHostInfo 采集主机信息
func (uc *HostUseCase) CollectHostInfo(ctx context.Context, hostID uint) error {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	mode := uc.effectiveManagementMode(host)
	if mode == ManagementModeNone {
		uc.persistCollectFailure(ctx, host, CollectStatusNotConfigured, fmt.Errorf("主机未配置采集通道"), -1)
		return fmt.Errorf("主机未配置采集通道")
	}

	hostCollector := uc.selectCollector(host)
	info, err := hostCollector.Collect(ctx, host)
	if err != nil {
		collectStatus, runtimeStatus := uc.inferCollectFailure(host, err)
		uc.persistCollectFailure(ctx, host, collectStatus, err, runtimeStatus)
		return fmt.Errorf("采集主机信息失败: %w", err)
	}

	// 更新主机信息
	host.OS = info.OS
	host.Kernel = info.Kernel
	host.Arch = info.Arch
	// 使用Threads作为CPU核心数，因为这是总CPU数量
	host.CPUCores = info.CPU.Threads
	host.CPUUsage = info.CPU.Usage
	host.MemoryTotal = info.Memory.Total
	host.MemoryUsed = info.Memory.Used
	host.MemoryUsage = info.Memory.Usage
	host.Uptime = info.Uptime
	host.Hostname = info.Hostname

	// 计算磁盘总容量和使用量
	var diskTotal, diskUsed uint64
	if len(info.Disk) > 0 {
		for _, disk := range info.Disk {
			diskTotal += disk.Total
			diskUsed += disk.Used
		}
	}
	host.DiskTotal = diskTotal
	host.DiskUsed = diskUsed
	if diskTotal > 0 {
		host.DiskUsage = float64(diskUsed) / float64(diskTotal) * 100
	}

	// 保存CPU详细信息为JSON
	if cpuJSON, err := info.CPU.ToJSON(); err == nil {
		host.CPUInfo = cpuJSON
	}

	now := time.Now()
	host.Status = 1
	host.LastSeen = &now
	host.LastCollectAt = &now
	host.CollectStatus = CollectStatusOnline
	host.CollectError = ""

	return uc.hostRepo.Update(ctx, host)
}

// createSSHClient 创建SSH客户端
func (uc *HostUseCase) createSSHClient(host *Host, credential *Credential) (*sshclient.Client, error) {
	var privateKey []byte

	// 检查凭证信息是否完整
	if credential.Type == "password" && credential.Password == "" {
		return nil, fmt.Errorf("凭证类型为密码认证，但未填写密码")
	}
	if credential.Type == "key" && credential.PrivateKey == "" {
		return nil, fmt.Errorf("凭证类型为密钥认证，但未填写私钥")
	}

	// 如果是密钥认证，需要解密私钥
	if credential.Type == "key" && credential.PrivateKey != "" {
		// 这里私钥是从数据库读取的加密数据，需要先解密
		// 但在credentialRepo中已经返回了解密后的数据
		// 实际上我们需要修改CredentialRepo的GetByID方法来返回解密后的数据
		// 或者在这里解密
		privateKey = []byte(credential.PrivateKey)
	}

	client, err := sshclient.NewClient(
		host.IP,
		host.Port,
		host.SSHUser,
		credential.Password,
		privateKey,
		credential.Passphrase,
	)
	if err != nil {
		return nil, fmt.Errorf("创建SSH客户端失败: %w", err)
	}

	return client, nil
}

func (uc *HostUseCase) inferCollectFailure(host *Host, err error) (string, int) {
	mode := uc.effectiveManagementMode(host)
	errMsg := err.Error()

	switch mode {
	case ManagementModeNone:
		return CollectStatusNotConfigured, -1
	case ManagementModeWinRM:
		if uc.effectiveManagementCredentialID(host) == 0 || strings.Contains(errMsg, "未配置 WinRM 凭证") || strings.Contains(errMsg, "WinRM 凭证未配置") || strings.Contains(errMsg, "当前管理方式需要 WinRM 凭证") {
			return CollectStatusNotConfigured, -1
		}
		if strings.Contains(errMsg, "创建WinRM连接失败") || strings.Contains(errMsg, "WinRM连接测试失败") || strings.Contains(errMsg, "WinRM执行失败") {
			return CollectStatusOffline, 0
		}
		return CollectStatusUnknown, -1
	case ManagementModeAgent:
		if strings.Contains(errMsg, "Agent 未注册") || strings.Contains(errMsg, "尚未上报") {
			return CollectStatusNotConfigured, -1
		}
		if strings.Contains(errMsg, "Agent 心跳超时") {
			return CollectStatusOffline, 0
		}
		return CollectStatusUnknown, -1
	default:
		if strings.Contains(errMsg, "未配置 SSH 凭证") || strings.Contains(errMsg, "未配置 SSH 用户名") {
			return CollectStatusNotConfigured, -1
		}
		if strings.Contains(errMsg, "创建SSH连接失败") || strings.Contains(errMsg, "连接测试失败") {
			return CollectStatusOffline, 0
		}
		return CollectStatusUnknown, -1
	}
}

func (uc *HostUseCase) persistCollectFailure(ctx context.Context, host *Host, collectStatus string, err error, runtimeStatus int) {
	now := time.Now()
	host.CollectStatus = collectStatus
	host.CollectError = err.Error()
	host.LastCollectAt = &now
	host.Status = runtimeStatus
	_ = uc.hostRepo.Update(ctx, host)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestConnection 测试主机连接
func (uc *HostUseCase) TestConnection(ctx context.Context, hostID uint) error {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	mode := uc.effectiveManagementMode(host)
	if mode == ManagementModeNone {
		return fmt.Errorf("主机未配置采集通道")
	}

	return uc.selectCollector(host).Test(ctx, host)
}

// BatchCollectHostInfo 批量采集主机信息
func (uc *HostUseCase) BatchCollectHostInfo(ctx context.Context, hostIDs []uint) error {
	for _, hostID := range hostIDs {
		// 采集失败时继续处理其他主机
		_ = uc.CollectHostInfo(ctx, hostID)
	}
	return nil
}

// BatchDelete 批量删除主机
func (uc *HostUseCase) BatchDelete(ctx context.Context, hostIDs []uint) error {
	for _, hostID := range hostIDs {
		if err := uc.Delete(ctx, hostID); err != nil {
			return fmt.Errorf("删除主机 %d 失败: %w", hostID, err)
		}
	}
	return nil
}

func (uc *HostUseCase) cleanupHostAgentArtifacts(ctx context.Context, hostID uint) error {
	if hostID == 0 {
		return nil
	}
	if uc.agentRepo != nil {
		if err := uc.agentRepo.DeleteByHostID(ctx, hostID); err != nil {
			return fmt.Errorf("清理 Agent 记录失败: %w", err)
		}
	}
	if uc.inventoryRepo != nil {
		if err := uc.inventoryRepo.DeleteByHostID(ctx, hostID); err != nil {
			return fmt.Errorf("清理 Agent 库存失败: %w", err)
		}
	}
	if err := removePrometheusTargetFile(uc.promCfg, hostID); err != nil {
		return fmt.Errorf("清理 Prometheus 目标失败: %w", err)
	}
	return nil
}

// toCredentialVO 转换为CredentialVO
func (uc *HostUseCase) toCredentialVO(credential *Credential) *CredentialVO {
	typeText := "密码"
	if credential.Type == "key" {
		typeText = "密钥"
	}

	return &CredentialVO{
		ID:          credential.ID,
		Name:        credential.Name,
		Protocol:    credential.Protocol,
		Type:        credential.Type,
		TypeText:    typeText,
		Username:    credential.Username,
		Domain:      credential.Domain,
		Description: credential.Description,
		CreateTime:  credential.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// GetCredentialRepo 获取凭证Repo（用于终端功能）
func (uc *HostUseCase) GetCredentialRepo() CredentialRepo {
	return uc.credentialRepo
}

// GetByIDDecrypted 根据ID获取凭证（解密后的，用于编辑时回显）
func (uc *CredentialUseCase) GetByIDDecrypted(ctx context.Context, id uint) (*Credential, error) {
	credential, err := uc.repo.GetByIDDecrypted(ctx, id)
	if err != nil {
		return nil, err
	}

	return credential, nil
}

// CredentialUseCase 凭证用例
type CredentialUseCase struct {
	repo     CredentialRepo
	hostRepo HostRepo
}

func NewCredentialUseCase(repo CredentialRepo, hostRepo HostRepo) *CredentialUseCase {
	return &CredentialUseCase{
		repo:     repo,
		hostRepo: hostRepo,
	}
}

// Create 创建凭证
func (uc *CredentialUseCase) Create(ctx context.Context, req *CredentialRequest) (*Credential, error) {
	if req.Protocol == "" {
		req.Protocol = "ssh"
	}
	if (req.Protocol == "rdp" || req.Protocol == "winrm") && req.Type != "password" {
		return nil, fmt.Errorf("%s 凭证仅支持密码认证", strings.ToUpper(req.Protocol))
	}

	credential := req.ToModel()

	if err := uc.repo.Create(ctx, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// Update 更新凭证
func (uc *CredentialUseCase) Update(ctx context.Context, req *CredentialRequest) error {
	credential, err := uc.repo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("凭证不存在")
	}

	credential.Name = req.Name
	if req.Protocol != "" {
		credential.Protocol = req.Protocol
	}
	credential.Type = req.Type
	credential.Username = req.Username
	credential.Domain = req.Domain
	credential.Description = req.Description

	if (credential.Protocol == "rdp" || credential.Protocol == "winrm") && credential.Type != "password" {
		return fmt.Errorf("%s 凭证仅支持密码认证", strings.ToUpper(credential.Protocol))
	}

	// 如果提供了新的密码或私钥，更新它们
	if req.Password != "" {
		credential.Password = req.Password
	} else if req.ClearPassword {
		credential.Password = ""
	}
	if req.PrivateKey != "" {
		credential.PrivateKey = req.PrivateKey
	} else if req.ClearPrivateKey {
		credential.PrivateKey = ""
	}
	if req.Passphrase != "" {
		credential.Passphrase = req.Passphrase
	} else if req.ClearPassphrase {
		credential.Passphrase = ""
	}

	return uc.repo.Update(ctx, credential)
}

// Delete 删除凭证
func (uc *CredentialUseCase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

// GetByID 根据ID获取凭证
func (uc *CredentialUseCase) GetByID(ctx context.Context, id uint) (*Credential, error) {
	return uc.repo.GetByID(ctx, id)
}

// List 分页查询凭证列表
func (uc *CredentialUseCase) List(ctx context.Context, page, pageSize int, keyword string) ([]*CredentialVO, int64, error) {
	credentials, total, err := uc.repo.List(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, 0, err
	}

	var vos []*CredentialVO
	for _, cred := range credentials {
		typeText := "密码"
		if cred.Type == "key" {
			typeText = "密钥"
		}

		// 统计使用该凭证的主机数量
		usedCount, _ := uc.hostRepo.CountByCredentialID(ctx, cred.ID)

		vo := &CredentialVO{
			ID:          cred.ID,
			Name:        cred.Name,
			Protocol:    cred.Protocol,
			Type:        cred.Type,
			TypeText:    typeText,
			Username:    cred.Username,
			Domain:      cred.Domain,
			Description: cred.Description,
			CreateTime:  cred.CreatedAt.Format("2006-01-02 15:04:05"),
			HostCount:   usedCount,
		}
		vos = append(vos, vo)
	}

	return vos, total, nil
}

// GetAll 获取所有凭证（用于下拉选择）
func (uc *CredentialUseCase) GetAll(ctx context.Context) ([]*CredentialVO, error) {
	credentials, err := uc.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var vos []*CredentialVO
	for _, cred := range credentials {
		typeText := "密码"
		if cred.Type == "key" {
			typeText = "密钥"
		}

		// 统计使用该凭证的主机数量
		usedCount, _ := uc.hostRepo.CountByCredentialID(ctx, cred.ID)

		vo := &CredentialVO{
			ID:          cred.ID,
			Name:        cred.Name,
			Protocol:    cred.Protocol,
			Type:        cred.Type,
			TypeText:    typeText,
			Username:    cred.Username,
			Domain:      cred.Domain,
			Description: cred.Description,
			CreateTime:  cred.CreatedAt.Format("2006-01-02 15:04:05"),
			HostCount:   usedCount,
		}
		vos = append(vos, vo)
	}

	return vos, nil
}

// CloudAccountUseCase 云平台账号用例
type CloudAccountUseCase struct {
	repo CloudAccountRepo
}

func NewCloudAccountUseCase(repo CloudAccountRepo) *CloudAccountUseCase {
	return &CloudAccountUseCase{repo: repo}
}

// Create 创建云平台账号
func (uc *CloudAccountUseCase) Create(ctx context.Context, req *CloudAccountRequest) (*CloudAccount, error) {
	// 创建时必须提供 AccessKey 和 SecretKey
	if req.AccessKey == "" {
		return nil, fmt.Errorf("AccessKey 不能为空")
	}
	if req.SecretKey == "" {
		return nil, fmt.Errorf("SecretKey 不能为空")
	}

	account := req.ToModel()

	if err := uc.repo.Create(ctx, account); err != nil {
		return nil, err
	}

	return account, nil
}

// Update 更新云平台账号
func (uc *CloudAccountUseCase) Update(ctx context.Context, req *CloudAccountRequest) error {
	account, err := uc.repo.GetByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("云平台账号不存在")
	}

	account.Name = req.Name
	account.Provider = req.Provider
	// 只有提供了新的值才更新 AccessKey 和 SecretKey
	if req.AccessKey != "" {
		account.AccessKey = req.AccessKey
	}
	if req.SecretKey != "" {
		account.SecretKey = req.SecretKey
	}
	account.Region = req.Region
	account.Description = req.Description
	account.Status = req.Status

	return uc.repo.Update(ctx, account)
}

// Delete 删除云平台账号
func (uc *CloudAccountUseCase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

// GetByID 根据ID获取云平台账号
func (uc *CloudAccountUseCase) GetByID(ctx context.Context, id uint) (*CloudAccount, error) {
	return uc.repo.GetByID(ctx, id)
}

// List 分页查询云平台账号列表
func (uc *CloudAccountUseCase) List(ctx context.Context, page, pageSize int) ([]*CloudAccountVO, int64, error) {
	accounts, total, err := uc.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []*CloudAccountVO
	for _, acc := range accounts {
		vo := uc.toVO(acc)
		vos = append(vos, vo)
	}

	return vos, total, nil
}

// GetAll 获取所有启用的云平台账号
func (uc *CloudAccountUseCase) GetAll(ctx context.Context) ([]*CloudAccountVO, error) {
	accounts, err := uc.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var vos []*CloudAccountVO
	for _, acc := range accounts {
		vo := uc.toVO(acc)
		vos = append(vos, vo)
	}

	return vos, nil
}

// GetRegions 获取云平台的区域列表
func (uc *CloudAccountUseCase) GetRegions(ctx context.Context, accountID uint) ([]*CloudRegionVO, error) {
	account, err := uc.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("云平台账号不存在")
	}

	// 根据不同的云厂商调用不同的SDK获取区域列表
	var regions []CloudRegion

	switch account.Provider {
	case "aliyun":
		regions, err = uc.listAliyunRegions(account)
	case "tencent":
		regions, err = uc.listTencentRegions(account)
	case "aws":
		regions, err = uc.listAWSRegions(account)
	case "jdcloud":
		regions, err = uc.listJDCloudRegions(account)
	default:
		return nil, fmt.Errorf("暂不支持该云平台")
	}

	if err != nil {
		return nil, fmt.Errorf("获取区域列表失败: %w", err)
	}

	// 转换为VO
	var vos []*CloudRegionVO
	for _, region := range regions {
		vos = append(vos, &CloudRegionVO{
			Value: region.Value,
			Label: region.Label,
		})
	}

	return vos, nil
}

// GetInstances 获取云平台的实例列表
func (uc *CloudAccountUseCase) GetInstances(ctx context.Context, accountID uint, region string) ([]*CloudInstanceVO, error) {
	account, err := uc.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("云平台账号不存在")
	}

	// 根据不同的云厂商调用不同的SDK获取实例列表
	var instances []CloudInstance

	switch account.Provider {
	case "aliyun":
		instances, err = uc.listAliyunInstances(account, region)
	case "tencent":
		instances, err = uc.listTencentInstances(account, region)
	case "aws":
		instances, err = uc.listAWSInstances(account, region)
	default:
		return nil, fmt.Errorf("暂不支持该云平台")
	}

	if err != nil {
		return nil, fmt.Errorf("获取云主机列表失败: %w", err)
	}

	// 转换为VO
	var vos []*CloudInstanceVO
	for _, inst := range instances {
		vos = append(vos, &CloudInstanceVO{
			InstanceID: inst.InstanceID,
			Name:       inst.Name,
			PublicIP:   inst.PublicIP,
			PrivateIP:  inst.PrivateIP,
			OS:         inst.OS,
			Status:     inst.Status,
		})
	}

	return vos, nil
}

// toVO 转换为VO
func (uc *CloudAccountUseCase) toVO(account *CloudAccount) *CloudAccountVO {
	providerText := account.Provider
	switch account.Provider {
	case "aliyun":
		providerText = "阿里云"
	case "tencent":
		providerText = "腾讯云"
	case "aws":
		providerText = "AWS"
	case "huawei":
		providerText = "华为云"
	case "jdcloud":
		providerText = "京东云"
	}

	return &CloudAccountVO{
		ID:           account.ID,
		Name:         account.Name,
		Provider:     account.Provider,
		ProviderText: providerText,
		Region:       account.Region,
		Description:  account.Description,
		Status:       account.Status,
		CreateTime:   account.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// ImportFromCloud 从云平台导入主机
func (uc *CloudAccountUseCase) ImportFromCloud(ctx context.Context, req *CloudImportRequest, hostUseCase *HostUseCase) error {
	account, err := uc.repo.GetByID(ctx, req.AccountID)
	if err != nil {
		return fmt.Errorf("云平台账号不存在")
	}

	// 根据不同的云厂商调用不同的SDK获取实例列表
	var instances []CloudInstance

	switch account.Provider {
	case "aliyun":
		instances, err = uc.listAliyunInstances(account, req.Region)
	case "tencent":
		instances, err = uc.listTencentInstances(account, req.Region)
	case "aws":
		instances, err = uc.listAWSInstances(account, req.Region)
	default:
		return fmt.Errorf("暂不支持该云平台")
	}

	if err != nil {
		return fmt.Errorf("获取云主机列表失败: %w", err)
	}

	// 批量导入主机
	successCount := 0
	var importErrors []string

	for _, instance := range instances {
		// 如果指定了实例ID列表，只导入指定的实例
		if len(req.InstanceIDs) > 0 && !utils.Contains(req.InstanceIDs, instance.InstanceID) {
			continue
		}

		// 检查是否已存在（通过云实例ID）
		existHost, err := hostUseCase.hostRepo.GetByCloudInstanceID(ctx, instance.InstanceID)
		if err == nil && existHost != nil {
			// 主机已存在，更新分组
			existHost.GroupID = req.GroupID
			existHost.Name = instance.Name
			if err := hostUseCase.hostRepo.Update(ctx, existHost); err != nil {
				importErrors = append(importErrors, fmt.Sprintf("实例 %s 更新失败: %v", instance.InstanceID, err))
			} else {
				successCount++
			}
			continue
		}

		// 确定使用的IP地址（优先公网IP，如果没有则使用私网IP）
		ip := instance.PublicIP
		if ip == "" {
			ip = instance.PrivateIP
		}

		// 如果都没有IP，跳过该实例
		if ip == "" {
			importErrors = append(importErrors, fmt.Sprintf("实例 %s (%s) 没有IP地址，跳过", instance.Name, instance.InstanceID))
			continue
		}

		// 检查IP是否已被其他主机使用
		existByIP, _ := hostUseCase.hostRepo.GetByIP(ctx, ip)
		if existByIP != nil {
			// IP已被使用，但不是同一个云实例，更新该主机为云主机
			existByIP.Type = "cloud"
			existByIP.CloudProvider = account.Provider
			existByIP.CloudInstanceID = instance.InstanceID
			existByIP.CloudAccountID = req.AccountID
			existByIP.GroupID = req.GroupID
			existByIP.Name = instance.Name
			if instance.OS != "" {
				existByIP.OS = instance.OS
			}
			if err := hostUseCase.hostRepo.Update(ctx, existByIP); err != nil {
				importErrors = append(importErrors, fmt.Sprintf("实例 %s 关联IP失败: %v", instance.InstanceID, err))
			} else {
				successCount++
			}
			continue
		}

		// 创建新主机
		hostReq := &HostRequest{
			Name:            instance.Name,
			GroupID:         req.GroupID,
			Type:            "cloud",
			CloudProvider:   account.Provider,
			CloudInstanceID: instance.InstanceID,
			CloudAccountID:  req.AccountID,
			SSHUser:         "root", // 默认使用root
			IP:              ip,
			Port:            22,
			Description:     fmt.Sprintf("从%s导入", account.Name),
		}

		host := hostReq.ToModel()
		host.Status = -1 // 初始状态未知
		host.OS = instance.OS

		if err := hostUseCase.hostRepo.Create(ctx, host); err != nil {
			importErrors = append(importErrors, fmt.Sprintf("实例 %s 创建失败: %v", instance.InstanceID, err))
		} else {
			successCount++
		}
	}

	if len(importErrors) > 0 {
		return fmt.Errorf("导入完成，成功 %d 个，失败: %s", successCount, strings.Join(importErrors, "; "))
	}

	return nil
}

// CloudInstance 云主机实例
type CloudInstance struct {
	InstanceID string
	Name       string
	PublicIP   string
	PrivateIP  string
	OS         string
	Status     string
}

// CloudRegion 云区域
type CloudRegion struct {
	Value string
	Label string
}

var awsRegionLabels = map[string]string{
	"us-east-1":      "美国东部 (弗吉尼亚北部)",
	"us-east-2":      "美国东部 (俄亥俄)",
	"us-west-1":      "美国西部 (加利福尼亚北部)",
	"us-west-2":      "美国西部 (俄勒冈)",
	"ca-central-1":   "加拿大 (中部)",
	"sa-east-1":      "南美洲 (圣保罗)",
	"eu-west-1":      "欧洲 (爱尔兰)",
	"eu-west-2":      "欧洲 (伦敦)",
	"eu-west-3":      "欧洲 (巴黎)",
	"eu-central-1":   "欧洲 (法兰克福)",
	"eu-central-2":   "欧洲 (苏黎世)",
	"eu-north-1":     "欧洲 (斯德哥尔摩)",
	"eu-south-1":     "欧洲 (米兰)",
	"eu-south-2":     "欧洲 (西班牙)",
	"ap-east-1":      "亚太地区 (香港)",
	"ap-south-1":     "亚太地区 (孟买)",
	"ap-south-2":     "亚太地区 (海得拉巴)",
	"ap-southeast-1": "亚太地区 (新加坡)",
	"ap-southeast-2": "亚太地区 (悉尼)",
	"ap-southeast-3": "亚太地区 (雅加达)",
	"ap-southeast-4": "亚太地区 (墨尔本)",
	"ap-northeast-1": "亚太地区 (东京)",
	"ap-northeast-2": "亚太地区 (首尔)",
	"ap-northeast-3": "亚太地区 (大阪)",
	"ap-east-2":      "亚太地区 (台北)",
	"me-south-1":     "中东 (巴林)",
	"me-central-1":   "中东 (阿联酋)",
	"af-south-1":     "非洲 (开普敦)",
	"il-central-1":   "以色列 (特拉维夫)",
}

// listAliyunRegions 获取阿里云区域列表
func (uc *CloudAccountUseCase) listAliyunRegions(account *CloudAccount) ([]CloudRegion, error) {
	// 使用杭州区域创建客户端（DescribeRegions API 可以使用任意区域）
	client, err := ecs.NewClientWithAccessKey(
		"cn-hangzhou",
		account.AccessKey,
		account.SecretKey,
	)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云客户端失败: %w", err)
	}

	// 创建 DescribeRegions 请求
	request := ecs.CreateDescribeRegionsRequest()
	request.Scheme = "https"

	// 调用 API
	response, err := client.DescribeRegions(request)
	if err != nil {
		return nil, fmt.Errorf("获取阿里云区域列表失败: %w", err)
	}

	// 转换为 CloudRegion 格式
	var regions []CloudRegion
	for _, region := range response.Regions.Region {
		// 只返回可用的区域
		if region.LocalName != "" && region.RegionId != "" {
			regions = append(regions, CloudRegion{
				Value: region.RegionId,
				Label: fmt.Sprintf("%s (%s)", region.LocalName, region.RegionId),
			})
		}
	}

	return regions, nil
}

// listTencentRegions 获取腾讯云区域列表
func (uc *CloudAccountUseCase) listTencentRegions(account *CloudAccount) ([]CloudRegion, error) {
	// 创建腾讯云客户端
	cred := common.NewCredential(account.AccessKey, account.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "cvm.tencentcloudapi.com"

	// 使用默认区域 ap-guangzhou 来查询区域列表
	client, err := v20170312.NewClient(cred, "ap-guangzhou", cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云客户端失败: %w", err)
	}

	// DescribeRegions 请求
	request := v20170312.NewDescribeRegionsRequest()

	response, err := client.DescribeRegions(request)
	if err != nil {
		return nil, fmt.Errorf("获取腾讯云区域列表失败: %w", err)
	}

	var regions []CloudRegion
	if response.Response.RegionSet != nil {
		for _, region := range response.Response.RegionSet {
			regionName := ""
			if region.RegionName != nil {
				regionName = *region.RegionName
			}
			regionID := ""
			if region.Region != nil {
				regionID = *region.Region
			}

			// 只返回可用的区域
			if region.RegionState != nil && *region.RegionState == "AVAILABLE" {
				regions = append(regions, CloudRegion{
					Value: regionID,
					Label: regionName,
				})
			}
		}
	}

	return regions, nil
}

// listJDCloudRegions 获取京东云区域列表
func (uc *CloudAccountUseCase) listJDCloudRegions(account *CloudAccount) ([]CloudRegion, error) {
	// 京东云区域列表
	return []CloudRegion{
		{Value: "cn-north-1", Label: "华北-北京"},
		{Value: "cn-east-1", Label: "华东-宿迁"},
		{Value: "cn-south-1", Label: "华南-广州"},
		{Value: "cn-southwest-1", Label: "西南-成都"},
		{Value: "ap-southeast-1", Label: "中国香港"},
	}, nil
}

func (uc *CloudAccountUseCase) listAWSRegions(account *CloudAccount) ([]CloudRegion, error) {
	client, err := uc.newAWSEC2Client(account, defaultAWSRegion(account.Region))
	if err != nil {
		return nil, err
	}

	response, err := client.DescribeRegions(context.Background(), &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("获取AWS区域列表失败: %w", err)
	}

	regions := make([]CloudRegion, 0, len(response.Regions))
	for _, region := range response.Regions {
		regionID := aws.ToString(region.RegionName)
		if regionID == "" {
			continue
		}

		optInStatus := aws.ToString(region.OptInStatus)
		if optInStatus == "not-opted-in" {
			continue
		}

		label := regionID
		if localName, ok := awsRegionLabels[regionID]; ok {
			label = fmt.Sprintf("%s (%s)", localName, regionID)
		}

		regions = append(regions, CloudRegion{
			Value: regionID,
			Label: label,
		})
	}

	sort.Slice(regions, func(i, j int) bool {
		return regions[i].Value < regions[j].Value
	})

	return regions, nil
}

// listAliyunInstances 获取阿里云实例列表
func (uc *CloudAccountUseCase) listAliyunInstances(account *CloudAccount, region string) ([]CloudInstance, error) {
	// 创建ECS客户端
	client, err := ecs.NewClientWithAccessKey(
		region,
		account.AccessKey,
		account.SecretKey,
	)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云客户端失败: %w", err)
	}

	var allInstances []CloudInstance
	pageSize := 100
	pageNumber := 1

	for {
		// 创建请求
		request := ecs.CreateDescribeInstancesRequest()
		request.Scheme = "https"
		request.PageSize = requests.NewInteger(pageSize)
		request.PageNumber = requests.NewInteger(pageNumber)

		// 发送请求
		response, err := client.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("获取阿里云实例失败: %w", err)
		}

		// 转换结果
		for _, instance := range response.Instances.Instance {
			var publicIP, privateIP string
			if len(instance.PublicIpAddress.IpAddress) > 0 {
				publicIP = instance.PublicIpAddress.IpAddress[0]
			}
			if len(instance.InnerIpAddress.IpAddress) > 0 {
				privateIP = instance.InnerIpAddress.IpAddress[0]
			}

			allInstances = append(allInstances, CloudInstance{
				InstanceID: instance.InstanceId,
				Name:       instance.InstanceName,
				PublicIP:   publicIP,
				PrivateIP:  privateIP,
				OS:         instance.OSName,
				Status:     instance.Status,
			})
		}

		// 检查是否还有更多页
		totalCount := int(response.TotalCount)
		if len(allInstances) >= totalCount || pageNumber*pageSize >= totalCount {
			break
		}
		pageNumber++

		// 最多获取10页（1000条）
		if pageNumber > 10 {
			break
		}
	}

	return allInstances, nil
}

// listTencentInstances 获取腾讯云实例列表
func (uc *CloudAccountUseCase) listTencentInstances(account *CloudAccount, region string) ([]CloudInstance, error) {
	// 创建腾讯云CVM客户端
	cred := common.NewCredential(account.AccessKey, account.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "cvm.tencentcloudapi.com"

	client, err := v20170312.NewClient(cred, region, cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云客户端失败: %w", err)
	}

	// DescribeInstances 请求
	request := v20170312.NewDescribeInstancesRequest()
	limit := int64(100)
	request.Limit = &limit

	var allInstances []CloudInstance
	offset := int64(0)

	for {
		request.Offset = &offset
		response, err := client.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("获取腾讯云实例失败: %w", err)
		}

		if response.Response.InstanceSet == nil || len(response.Response.InstanceSet) == 0 {
			break
		}

		for _, inst := range response.Response.InstanceSet {
			var publicIP, privateIP string
			if len(inst.PublicIpAddresses) > 0 && inst.PublicIpAddresses[0] != nil {
				publicIP = *inst.PublicIpAddresses[0]
			}
			if len(inst.PrivateIpAddresses) > 0 && inst.PrivateIpAddresses[0] != nil {
				privateIP = *inst.PrivateIpAddresses[0]
			}

			var osName string
			if inst.OsName != nil {
				osName = *inst.OsName
			}

			var instanceID, instanceName, status string
			if inst.InstanceId != nil {
				instanceID = *inst.InstanceId
			}
			if inst.InstanceName != nil {
				instanceName = *inst.InstanceName
			}
			if inst.InstanceState != nil {
				status = *inst.InstanceState
			}

			allInstances = append(allInstances, CloudInstance{
				InstanceID: instanceID,
				Name:       instanceName,
				PublicIP:   publicIP,
				PrivateIP:  privateIP,
				OS:         osName,
				Status:     status,
			})
		}

		if len(response.Response.InstanceSet) < 100 {
			break
		}
		offset += 100
		if offset >= 1000 { // 最多获取1000个实例
			break
		}
	}

	return allInstances, nil
}

// listJDCloudInstances 获取京东云实例列表
func (uc *CloudAccountUseCase) listJDCloudInstances(account *CloudAccount, region string) ([]CloudInstance, error) {
	// 京东云使用 DescribeInstances API
	url := fmt.Sprintf("https://vm.%s.jdcloud-api.com/v2/regions/%s/instances?pageSize=100", region, region)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+account.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用京东云API失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("京东云API返回错误: %s", string(body))
	}

	// 解析京东云API响应
	var result struct {
		Result struct {
			Instances []struct {
				InstanceID string `json:"instanceId"`
				Name       string `json:"name"`
				Status     string `json:"status"`
				PrimaryIP  string `json:"primaryIpAddress"`
				PrivateIP  string `json:"privateIpAddress"`
				OSName     string `json:"osName"`
			} `json:"instances"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析京东云响应失败: %w", err)
	}

	var instances []CloudInstance
	for _, inst := range result.Result.Instances {
		instances = append(instances, CloudInstance{
			InstanceID: inst.InstanceID,
			Name:       inst.Name,
			PublicIP:   inst.PrimaryIP,
			PrivateIP:  inst.PrivateIP,
			OS:         inst.OSName,
			Status:     inst.Status,
		})
	}

	return instances, nil
}

func (uc *CloudAccountUseCase) listAWSInstances(account *CloudAccount, region string) ([]CloudInstance, error) {
	awsRegion := strings.TrimSpace(region)
	if awsRegion == "" {
		awsRegion = account.Region
	}

	client, err := uc.newAWSEC2Client(account, defaultAWSRegion(awsRegion))
	if err != nil {
		return nil, err
	}

	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(100),
	})

	allInstances := make([]CloudInstance, 0, 100)
	pageCount := 0
	for paginator.HasMorePages() {
		response, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("获取AWS实例失败: %w", err)
		}

		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				instanceID := aws.ToString(instance.InstanceId)
				if instanceID == "" {
					continue
				}

				name := instanceID
				for _, tag := range instance.Tags {
					if aws.ToString(tag.Key) == "Name" && aws.ToString(tag.Value) != "" {
						name = aws.ToString(tag.Value)
						break
					}
				}

				osName := aws.ToString(instance.PlatformDetails)
				if osName == "" {
					if strings.EqualFold(string(instance.Platform), "windows") {
						osName = "Windows"
					} else {
						osName = "Linux/UNIX"
					}
				}

				allInstances = append(allInstances, CloudInstance{
					InstanceID: instanceID,
					Name:       name,
					PublicIP:   aws.ToString(instance.PublicIpAddress),
					PrivateIP:  aws.ToString(instance.PrivateIpAddress),
					OS:         osName,
					Status:     strings.ToLower(string(instance.State.Name)),
				})
			}
		}

		pageCount++
		if pageCount >= 20 {
			break
		}
	}

	return allInstances, nil
}

func (uc *CloudAccountUseCase) newAWSEC2Client(account *CloudAccount, region string) (*ec2.Client, error) {
	awsCfg, err := awscfg.LoadDefaultConfig(context.Background(),
		awscfg.WithRegion(defaultAWSRegion(region)),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			account.AccessKey,
			account.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("创建AWS客户端失败: %w", err)
	}

	return ec2.NewFromConfig(awsCfg), nil
}

func defaultAWSRegion(region string) string {
	if strings.TrimSpace(region) != "" {
		return region
	}
	return "us-east-1"
}

// ExcelImportResult Excel导入结果
type ExcelImportResult struct {
	SuccessCount int      `json:"successCount"`
	FailedCount  int      `json:"failedCount"`
	FailedRows   []int    `json:"failedRows,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

// ImportFromExcel 从Excel批量导入主机
func (uc *HostUseCase) ImportFromExcel(ctx context.Context, excelData []byte) (*ExcelImportResult, error) {
	// 使用excelize读取Excel文件
	f, err := excelize.OpenReader(bytes.NewReader(excelData))
	if err != nil {
		return nil, fmt.Errorf("读取Excel文件失败: %w", err)
	}
	defer f.Close()

	// 获取第一个sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel文件中没有工作表")
	}

	// 读取数据（跳过标题行，从第2行开始）
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取Excel数据失败: %w", err)
	}

	result := &ExcelImportResult{
		FailedRows: make([]int, 0),
		Errors:     make([]string, 0),
	}

	// 获取所有分组和凭证映射
	groups, _ := uc.groupRepo.GetAll(ctx)
	groupCodeMap := make(map[string]uint)
	for _, g := range groups {
		groupCodeMap[g.Code] = g.ID
	}

	credentials, _ := uc.credentialRepo.GetAll(ctx)
	credentialNameMap := make(map[string]uint)
	for _, c := range credentials {
		credentialNameMap[c.Name] = c.ID
	}

	// 从第2行开始处理数据（第1行是标题）
	for i, row := range rows {
		rowNum := i + 1
		if i == 0 {
			// 跳过标题行
			continue
		}

		// 检查行是否为空
		if len(row) < 4 {
			continue
		}

		// 解析Excel行数据
		// 列顺序: 主机名称 | 分组编码 | SSH用户名 | IP地址 | SSH端口 | 凭证名称 | 标签 | 备注
		name := strings.TrimSpace(row[0])
		groupCode := strings.TrimSpace(row[1])
		sshUser := strings.TrimSpace(row[2])
		ip := strings.TrimSpace(row[3])
		port := 22
		if len(row) > 4 && row[4] != "" {
			p, err := strconv.Atoi(strings.TrimSpace(row[4]))
			if err == nil {
				port = p
			}
		}
		credentialName := ""
		if len(row) > 5 {
			credentialName = strings.TrimSpace(row[5])
		}
		tags := ""
		if len(row) > 6 {
			tags = strings.TrimSpace(row[6])
		}
		description := ""
		if len(row) > 7 {
			description = strings.TrimSpace(row[7])
		}

		// 验证必填字段
		if name == "" || sshUser == "" || ip == "" {
			result.FailedCount++
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 缺少必填字段", rowNum))
			continue
		}

		// 验证IP格式
		hostReq := &HostRequest{
			Name:        name,
			SSHUser:     sshUser,
			IP:          ip,
			Port:        port,
			Tags:        tags,
			Description: description,
		}

		// 查找分组ID
		if groupCode != "" {
			if groupID, ok := groupCodeMap[groupCode]; ok {
				hostReq.GroupID = groupID
			} else {
				result.FailedCount++
				result.FailedRows = append(result.FailedRows, rowNum)
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 分组编码'%s'不存在", rowNum, groupCode))
				continue
			}
		}

		// 查找凭证ID
		if credentialName != "" {
			if credentialID, ok := credentialNameMap[credentialName]; ok {
				hostReq.CredentialID = credentialID
			} else {
				result.FailedCount++
				result.FailedRows = append(result.FailedRows, rowNum)
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 凭证名称'%s'不存在", rowNum, credentialName))
				continue
			}
		}

		// 创建主机
		host := hostReq.ToModel()
		if err := uc.hostRepo.CreateOrUpdate(ctx, host); err != nil {
			result.FailedCount++
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: %s", rowNum, err.Error()))
		} else {
			result.SuccessCount++
		}
	}

	return result, nil
}

// ImportFromExcelWithType 从Excel批量导入主机（带类型和分组）
func (uc *HostUseCase) ImportFromExcelWithType(ctx context.Context, excelData []byte, hostType string, defaultGroupID uint) (*ExcelImportResult, error) {
	// 使用excelize读取Excel文件
	f, err := excelize.OpenReader(bytes.NewReader(excelData))
	if err != nil {
		return nil, fmt.Errorf("读取Excel文件失败: %w", err)
	}
	defer f.Close()

	// 获取第一个sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel文件中没有工作表")
	}

	// 读取数据（跳过标题行，从第2行开始）
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取Excel数据失败: %w", err)
	}

	result := &ExcelImportResult{
		FailedRows: make([]int, 0),
		Errors:     make([]string, 0),
	}

	// 获取所有分组和凭证映射
	groups, _ := uc.groupRepo.GetAll(ctx)
	groupCodeMap := make(map[string]uint)
	for _, g := range groups {
		groupCodeMap[g.Code] = g.ID
	}

	credentials, _ := uc.credentialRepo.GetAll(ctx)
	credentialNameMap := make(map[string]uint)
	for _, c := range credentials {
		credentialNameMap[c.Name] = c.ID
	}

	// 从第2行开始处理数据（第1行是标题）
	for i, row := range rows {
		rowNum := i + 1
		if i == 0 {
			// 跳过标题行
			continue
		}

		// 检查行是否为空
		if len(row) < 4 {
			continue
		}

		// 解析Excel行数据
		// 列顺序: 主机名称 | 分组编码 | SSH用户名 | IP地址 | SSH端口 | 凭证名称 | 标签 | 备注
		name := strings.TrimSpace(row[0])
		groupCode := strings.TrimSpace(row[1])
		sshUser := strings.TrimSpace(row[2])
		ip := strings.TrimSpace(row[3])
		port := 22
		if len(row) > 4 && row[4] != "" {
			p, err := strconv.Atoi(strings.TrimSpace(row[4]))
			if err == nil {
				port = p
			}
		}
		credentialName := ""
		if len(row) > 5 {
			credentialName = strings.TrimSpace(row[5])
		}
		tags := ""
		if len(row) > 6 {
			tags = strings.TrimSpace(row[6])
		}
		description := ""
		if len(row) > 7 {
			description = strings.TrimSpace(row[7])
		}

		// 验证必填字段
		if name == "" || sshUser == "" || ip == "" {
			result.FailedCount++
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 缺少必填字段", rowNum))
			continue
		}

		// 验证IP格式
		hostReq := &HostRequest{
			Name:        name,
			Type:        hostType,
			SSHUser:     sshUser,
			IP:          ip,
			Port:        port,
			Tags:        tags,
			Description: description,
		}

		// 如果有默认分组ID，使用默认分组
		if defaultGroupID > 0 {
			hostReq.GroupID = defaultGroupID
		}

		// 如果Excel中指定了分组编码，查找分组ID
		if groupCode != "" {
			if groupID, ok := groupCodeMap[groupCode]; ok {
				hostReq.GroupID = groupID
			} else {
				result.FailedCount++
				result.FailedRows = append(result.FailedRows, rowNum)
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 分组编码'%s'不存在", rowNum, groupCode))
				continue
			}
		}

		// 查找凭证ID
		if credentialName != "" {
			if credentialID, ok := credentialNameMap[credentialName]; ok {
				hostReq.CredentialID = credentialID
			} else {
				result.FailedCount++
				result.FailedRows = append(result.FailedRows, rowNum)
				result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 凭证名称'%s'不存在", rowNum, credentialName))
				continue
			}
		}

		// 创建主机
		host := hostReq.ToModel()
		if err := uc.hostRepo.CreateOrUpdate(ctx, host); err != nil {
			result.FailedCount++
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: %s", rowNum, err.Error()))
		} else {
			result.SuccessCount++
		}
	}

	return result, nil
}

// ListFiles 列出主机目录下的文件
func (uc *HostUseCase) ListFiles(ctx context.Context, hostID uint, remotePath string) ([]*HostFileEntry, error) {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机信息失败: %w", err)
	}

	if host.OSType == OSTypeWindows && !uc.shouldUseAgentFileProxy(host) {
		return nil, fmt.Errorf("Windows 文件管理仅支持已部署并在线的 Agent")
	}

	if uc.shouldUseAgentFileProxy(host) {
		return uc.listFilesViaAgent(ctx, host, remotePath)
	}

	return uc.listFilesViaSSH(ctx, host, remotePath)
}

// UploadFile 上传文件到主机
func (uc *HostUseCase) UploadFile(ctx context.Context, hostID uint, reader io.Reader, remotePath, filename string) error {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	if host.OSType == OSTypeWindows && !uc.shouldUseAgentFileProxy(host) {
		return fmt.Errorf("Windows 文件管理仅支持已部署并在线的 Agent")
	}

	if uc.shouldUseAgentFileProxy(host) {
		return uc.uploadFileViaAgent(ctx, host, reader, remotePath, filename)
	}

	return uc.uploadFileViaSSH(ctx, host, reader, remotePath, filename)
}

// DownloadFile 从主机下载文件
func (uc *HostUseCase) DownloadFile(ctx context.Context, hostID uint, remotePath string, writer io.Writer) error {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	if host.OSType == OSTypeWindows && !uc.shouldUseAgentFileProxy(host) {
		return fmt.Errorf("Windows 文件管理仅支持已部署并在线的 Agent")
	}

	if uc.shouldUseAgentFileProxy(host) {
		return uc.downloadFileViaAgent(ctx, host, remotePath, writer)
	}

	return uc.downloadFileViaSSH(ctx, host, remotePath, writer)
}

// DeleteFile 删除主机上的文件
func (uc *HostUseCase) DeleteFile(ctx context.Context, hostID uint, remotePath string) error {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	if host.OSType == OSTypeWindows && !uc.shouldUseAgentFileProxy(host) {
		return fmt.Errorf("Windows 文件管理仅支持已部署并在线的 Agent")
	}

	if uc.shouldUseAgentFileProxy(host) {
		return uc.deleteFileViaAgent(ctx, host, remotePath)
	}

	return uc.deleteFileViaSSH(ctx, host, remotePath)
}
