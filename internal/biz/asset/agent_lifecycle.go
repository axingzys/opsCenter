package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/internal/conf"
	sshclient "github.com/ydcloud-dy/opshub/pkg/ssh"
	"gorm.io/gorm"
)

func (uc *AgentUseCase) Deploy(ctx context.Context, hostIDs []uint, operatorID uint, baseURL string) ([]*AgentJobVO, error) {
	baseURL = resolveAgentBaseURL(baseURL)

	results := make([]*AgentJobVO, 0, len(hostIDs))
	for _, hostID := range dedupeHostIDs(hostIDs) {
		jobVO, err := uc.deployOne(ctx, hostID, operatorID, baseURL)
		if err != nil {
			if jobVO != nil {
				results = append(results, jobVO)
				continue
			}
			results = append(results, &AgentJobVO{
				HostID:     hostID,
				JobType:    AgentJobTypeDeploy,
				Status:     AgentJobStatusFailed,
				Progress:   0,
				Stage:      "failed",
				Message:    "部署失败",
				Error:      err.Error(),
				UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
			})
			continue
		}
		results = append(results, jobVO)
	}

	return results, nil
}

func (uc *AgentUseCase) Uninstall(ctx context.Context, hostIDs []uint, operatorID uint) ([]*AgentJobVO, error) {
	results := make([]*AgentJobVO, 0, len(hostIDs))
	for _, hostID := range dedupeHostIDs(hostIDs) {
		jobVO, err := uc.uninstallOne(ctx, hostID, operatorID)
		if err != nil {
			if jobVO != nil {
				results = append(results, jobVO)
				continue
			}
			results = append(results, &AgentJobVO{
				HostID:     hostID,
				JobType:    AgentJobTypeUninstall,
				Status:     AgentJobStatusFailed,
				Progress:   0,
				Stage:      "failed",
				Message:    "卸载失败",
				Error:      err.Error(),
				UpdateTime: time.Now().Format("2006-01-02 15:04:05"),
			})
			continue
		}
		results = append(results, jobVO)
	}

	return results, nil
}

func (uc *AgentUseCase) List(ctx context.Context, page, pageSize int, keyword, status string, accessibleHostIDs []uint) ([]*AgentListItemVO, int64, error) {
	agents, total, err := uc.agentRepo.List(ctx, page, pageSize, keyword, accessibleHostIDs, status)
	if err != nil {
		return nil, 0, err
	}

	list := make([]*AgentListItemVO, 0, len(agents))
	for _, agentModel := range agents {
		host, err := uc.hostRepo.GetByID(ctx, agentModel.HostID)
		if err != nil {
			continue
		}

		displayStatus := resolvedAgentDisplayStatus(agentModel)
		item := &AgentListItemVO{
			HostID:           host.ID,
			HostName:         host.Name,
			IP:               host.IP,
			PrimaryPrivateIP: host.PrimaryPrivateIP,
			PrimaryPublicIP:  host.PrimaryPublicIP,
			Version:          firstNonEmpty([]string{agentModel.Version, host.AgentVersion}),
			Status:           displayStatus,
			StatusText:       AgentStatusText(displayStatus),
			ListenPort:       firstPositive(agentModel.ListenPort, host.AgentPort, uc.agentCfg.GetDefaultListenPort()),
			InstallProgress:  agentModel.InstallProgress,
			InstallStage:     agentModel.InstallStage,
			InstallStageText: AgentStageText(agentModel.InstallStage),
			HealthStatus:     AgentHealthStatus(displayStatus, agentModel.LastHeartbeatAt),
			UpdateTime:       agentModel.UpdatedAt.Format("2006-01-02 15:04:05"),
			LastError:        firstNonEmpty([]string{agentModel.LastError, host.AgentLastError}),
			AgentID:          firstNonEmpty([]string{agentModel.AgentID, host.AgentID}),
		}
		if agentModel.LastHeartbeatAt != nil {
			item.LastHeartbeatAt = agentModel.LastHeartbeatAt.Format("2006-01-02 15:04:05")
		}
		if agentModel.LastReportAt != nil {
			item.LastReportAt = agentModel.LastReportAt.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}

	return list, total, nil
}

func (uc *AgentUseCase) GetInventory(ctx context.Context, hostID uint) (*HostInventoryVO, error) {
	host, _ := uc.hostRepo.GetByID(ctx, hostID)
	item, err := uc.inventoryRepo.GetByHostID(ctx, hostID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			vo := &HostInventoryVO{HostID: hostID}
			applyHostInventoryIPFallback(vo, host)
			return vo, nil
		}
		return nil, err
	}

	vo := &HostInventoryVO{
		HostID: hostID,
	}
	decodeJSON(item.PrivateIPsJSON, &vo.PrivateIPs)
	decodeJSON(item.PublicIPsJSON, &vo.PublicIPs)
	decodeJSON(item.InterfacesJSON, &vo.Interfaces)
	decodeJSON(item.DisksJSON, &vo.Disks)
	decodeJSON(item.TopProcessesJSON, &vo.TopProcesses)
	decodeJSON(item.ListeningPortsJSON, &vo.ListeningPorts)
	decodeJSON(item.ConfigSummaryJSON, &vo.ConfigSummary)
	if item.CollectedAt != nil {
		vo.CollectedAt = item.CollectedAt.Format("2006-01-02 15:04:05")
	}
	vo.PublicIPHistory = uc.listPublicIPHistory(ctx, hostID)
	if len(vo.PublicIPHistory) == 0 {
		vo.PublicIPHistory = buildPublicIPHistoryFallback(host)
	}
	applyHostInventoryIPFallback(vo, host)
	return vo, nil
}

func (uc *AgentUseCase) GetJob(ctx context.Context, id uint) (*AgentJobVO, error) {
	job, err := uc.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toAgentJobVO(job), nil
}

func (uc *AgentUseCase) deployOne(ctx context.Context, hostID uint, operatorID uint, baseURL string) (*AgentJobVO, error) {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机失败: %w", err)
	}
	switch host.OSType {
	case OSTypeLinux:
	case OSTypeWindows:
		return uc.deployWindowsOne(ctx, host, operatorID, baseURL)
	default:
		return nil, fmt.Errorf("当前操作系统暂不支持自动部署 Agent")
	}

	now := time.Now()
	job := &AssetAgentJob{
		HostID:     host.ID,
		JobType:    AgentJobTypeDeploy,
		Status:     AgentJobStatusRunning,
		Progress:   5,
		Stage:      "building",
		Message:    "准备生成安装命令",
		OperatorID: operatorID,
		StartedAt:  &now,
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建部署任务失败: %w", err)
	}

	agentModel, created, err := uc.getOrCreateAgentModel(ctx, host)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "初始化 Agent 记录失败", err)
		return toAgentJobVO(job), err
	}
	agentModel.Status = AgentStatusDeploying
	agentModel.InstallProgress = 5
	agentModel.InstallStage = "building"
	agentModel.LastError = ""
	agentModel.DeployedBy = operatorID
	agentModel.DeployedAt = &now
	if err := uc.saveAgentModel(ctx, agentModel, created); err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "保存 Agent 记录失败", err)
		return toAgentJobVO(job), err
	}

	bootstrap, err := uc.GenerateBootstrap(ctx, host.ID, baseURL)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "生成安装命令失败", err)
		agentModel.Status = AgentStatusError
		agentModel.LastError = err.Error()
		agentModel.InstallStage = "building"
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}

	targetHost, credential, err := uc.resolveLinuxDeployTarget(ctx, host)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "校验 SSH 信息失败", err)
		agentModel.Status = AgentStatusError
		agentModel.LastError = err.Error()
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}

	sshConn, err := uc.createSSHClient(targetHost, credential)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "创建 SSH 连接失败", err)
		agentModel.Status = AgentStatusError
		agentModel.LastError = err.Error()
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}
	defer sshConn.Close()

	if err := uc.markJobRunning(ctx, job, 20, "uploading", "执行远程安装命令"); err != nil {
		return toAgentJobVO(job), err
	}
	agentModel.Status = AgentStatusDeploying
	agentModel.InstallProgress = 20
	agentModel.InstallStage = "uploading"
	if err := uc.agentRepo.Update(ctx, agentModel); err != nil {
		return toAgentJobVO(job), err
	}

	if _, err := sshConn.ExecuteWithTimeout(bootstrap.InstallCommand, 5*time.Minute); err != nil {
		_ = uc.markJobFailed(ctx, job, "uploading", "远程安装命令执行失败", err)
		agentModel.Status = AgentStatusError
		agentModel.LastError = err.Error()
		agentModel.InstallStage = "uploading"
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}

	refreshedAgent, activated := uc.waitForAgentActivity(ctx, host.ID, now, 12*time.Second)
	if activated {
		agentModel = refreshedAgent
		agentModel.Status = AgentStatusRunning
		agentModel.InstallProgress = 100
		agentModel.InstallStage = "running"
		agentModel.LastError = ""
		if err := uc.agentRepo.Update(ctx, agentModel); err != nil {
			return toAgentJobVO(job), err
		}
		if err := uc.markLatestJobSuccess(ctx, host.ID, "Agent 部署成功"); err != nil {
			return toAgentJobVO(job), nil
		}
		latestJob, err := uc.jobRepo.GetLatestByHostID(ctx, host.ID)
		if err == nil {
			return toAgentJobVO(latestJob), nil
		}
		return toAgentJobVO(job), nil
	}

	if err := uc.markJobRunning(ctx, job, 90, "waiting_register", "安装命令执行完成，等待首次上报"); err != nil {
		return toAgentJobVO(job), err
	}
	agentModel.Status = AgentStatusWaiting
	agentModel.InstallProgress = 90
	agentModel.InstallStage = "waiting_register"
	agentModel.LastError = ""
	if err := uc.agentRepo.Update(ctx, agentModel); err != nil {
		return toAgentJobVO(job), err
	}

	latestJob, err := uc.jobRepo.GetLatestByHostID(ctx, host.ID)
	if err == nil {
		return toAgentJobVO(latestJob), nil
	}
	return toAgentJobVO(job), nil
}

func (uc *AgentUseCase) uninstallOne(ctx context.Context, hostID uint, operatorID uint) (*AgentJobVO, error) {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uc.cleanupOrphanAgentPlatformState(ctx, hostID, operatorID, "主机不存在，清理平台侧 Agent 记录")
		}
		return nil, fmt.Errorf("获取主机失败: %w", err)
	}
	switch host.OSType {
	case OSTypeLinux:
	case OSTypeWindows:
		return uc.uninstallWindowsOne(ctx, host, operatorID)
	default:
		return nil, fmt.Errorf("当前操作系统暂不支持自动卸载 Agent")
	}

	agentModel, err := uc.agentRepo.GetByHostID(ctx, host.ID)
	missingAgentRecord := false
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("获取 Agent 记录失败: %w", err)
		}
		missingAgentRecord = true
		now := time.Now()
		agentModel = &AssetAgent{
			HostID:      host.ID,
			ListenPort:  uc.agentCfg.GetDefaultListenPort(),
			InstallPath: uc.agentCfg.GetDefaultInstallPath(),
			ServiceName: fmt.Sprintf("%s-%d", uc.agentCfg.GetServicePrefix(), host.ID),
			Status:      AgentStatusUninstalling,
			DeployedAt:  &now,
		}
	}

	now := time.Now()
	job := &AssetAgentJob{
		HostID:     host.ID,
		JobType:    AgentJobTypeUninstall,
		Status:     AgentJobStatusRunning,
		Progress:   10,
		Stage:      "removing",
		Message:    "准备卸载 Agent",
		OperatorID: operatorID,
		StartedAt:  &now,
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建卸载任务失败: %w", err)
	}

	targetHost, credential, err := uc.resolveLinuxDeployTarget(ctx, host)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "校验 SSH 信息失败", err)
		return toAgentJobVO(job), err
	}

	sshConn, err := uc.createSSHClient(targetHost, credential)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "创建 SSH 连接失败", err)
		return toAgentJobVO(job), err
	}
	defer sshConn.Close()

	if err := uc.markJobRunning(ctx, job, 35, "removing", "执行远程卸载命令"); err != nil {
		return toAgentJobVO(job), err
	}

	agentModel.Status = AgentStatusUninstalling
	agentModel.InstallProgress = 35
	agentModel.InstallStage = "removing"
	agentModel.LastError = ""
	if !missingAgentRecord {
		if err := uc.agentRepo.Update(ctx, agentModel); err != nil {
			return toAgentJobVO(job), err
		}
	}

	if _, err := sshConn.ExecuteWithTimeout(renderAgentUninstallShellScript(agentModel.ServiceName, agentModel.InstallPath), 3*time.Minute); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "远程卸载命令执行失败", err)
		if !missingAgentRecord {
			agentModel.Status = AgentStatusError
			agentModel.LastError = err.Error()
			agentModel.InstallStage = "removing"
			_ = uc.agentRepo.Update(ctx, agentModel)
		}
		return toAgentJobVO(job), err
	}

	if err := uc.markJobRunning(ctx, job, 75, "removing", "清理平台侧 Agent 记录"); err != nil {
		return toAgentJobVO(job), err
	}

	if err := uc.inventoryRepo.DeleteByHostID(ctx, host.ID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理库存快照失败", err)
		return toAgentJobVO(job), err
	}
	if !missingAgentRecord {
		if err := uc.agentRepo.DeleteByHostID(ctx, host.ID); err != nil {
			_ = uc.markJobFailed(ctx, job, "removing", "清理 Agent 记录失败", err)
			return toAgentJobVO(job), err
		}
	}
	if err := uc.removePrometheusTarget(host.ID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理 Prometheus 目标失败", err)
		return toAgentJobVO(job), err
	}

	uc.resetHostAgentState(host)
	if err := uc.hostRepo.Update(ctx, host); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "更新主机状态失败", err)
		return toAgentJobVO(job), err
	}

	if err := uc.markJobSuccess(ctx, job, "removed", "Agent 卸载成功"); err != nil {
		return toAgentJobVO(job), err
	}
	return toAgentJobVO(job), nil
}

func (uc *AgentUseCase) cleanupOrphanAgentPlatformState(ctx context.Context, hostID uint, operatorID uint, message string) (*AgentJobVO, error) {
	now := time.Now()
	job := &AssetAgentJob{
		HostID:     hostID,
		JobType:    AgentJobTypeUninstall,
		Status:     AgentJobStatusRunning,
		Progress:   10,
		Stage:      "removing",
		Message:    "准备清理 Agent 记录",
		OperatorID: operatorID,
		StartedAt:  &now,
	}
	if err := uc.jobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("创建卸载任务失败: %w", err)
	}

	if err := uc.markJobRunning(ctx, job, 75, "removing", message); err != nil {
		return toAgentJobVO(job), err
	}
	if err := uc.inventoryRepo.DeleteByHostID(ctx, hostID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理库存快照失败", err)
		return toAgentJobVO(job), err
	}
	if err := uc.agentRepo.DeleteByHostID(ctx, hostID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理 Agent 记录失败", err)
		return toAgentJobVO(job), err
	}
	if err := uc.removePrometheusTarget(hostID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理 Prometheus 目标失败", err)
		return toAgentJobVO(job), err
	}
	if err := uc.markJobSuccess(ctx, job, "removed", "Agent 记录已清理"); err != nil {
		return toAgentJobVO(job), err
	}
	return toAgentJobVO(job), nil
}

func (uc *AgentUseCase) resolveLinuxDeployTarget(ctx context.Context, host *Host) (*Host, *Credential, error) {
	if strings.TrimSpace(host.SSHUser) == "" {
		return nil, nil, fmt.Errorf("主机未配置 SSH 用户")
	}

	credential, credentialID, err := uc.resolveLinuxCredential(ctx, host)
	if err != nil {
		return nil, nil, err
	}

	target := *host
	if target.Port <= 0 {
		target.Port = 22
	}
	if host.CredentialID == 0 {
		host.CredentialID = credentialID
	}
	if host.ManagementCredentialID == 0 {
		host.ManagementCredentialID = credentialID
	}
	return &target, credential, nil
}

func (uc *AgentUseCase) resolveLinuxCredential(ctx context.Context, host *Host) (*Credential, uint, error) {
	seen := map[uint]struct{}{}
	for _, credentialID := range []uint{host.CredentialID, host.ManagementCredentialID} {
		if credentialID == 0 {
			continue
		}
		if _, ok := seen[credentialID]; ok {
			continue
		}
		seen[credentialID] = struct{}{}

		credential, err := uc.credentialRepo.GetByIDDecrypted(ctx, credentialID)
		if err == nil {
			return credential, credentialID, nil
		}
	}

	candidates, err := uc.credentialRepo.GetAll(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("获取 SSH 凭证失败: %w", err)
	}

	for _, item := range candidates {
		if item == nil || item.ID == 0 {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		if !matchesLinuxCredentialFallback(item, host) {
			continue
		}

		credential, err := uc.credentialRepo.GetByIDDecrypted(ctx, item.ID)
		if err != nil {
			continue
		}
		if err := uc.testLinuxCredential(host, credential); err != nil {
			continue
		}
		return credential, item.ID, nil
	}

	return nil, 0, fmt.Errorf("主机未配置 SSH 凭证")
}

func matchesLinuxCredentialFallback(item *Credential, host *Host) bool {
	if item == nil {
		return false
	}
	if normalizeCredentialProtocol(item.Protocol) != ManagementModeSSH {
		return false
	}
	if sshUser := strings.TrimSpace(host.SSHUser); sshUser != "" && !strings.EqualFold(strings.TrimSpace(item.Username), sshUser) {
		return false
	}

	hostName := strings.TrimSpace(host.Name)
	hostIP := strings.TrimSpace(host.IP)
	credentialName := strings.TrimSpace(item.Name)
	return credentialName != "" && (strings.EqualFold(credentialName, hostName) || strings.EqualFold(credentialName, hostIP))
}

func (uc *AgentUseCase) testLinuxCredential(host *Host, credential *Credential) error {
	target := *host
	if target.Port <= 0 {
		target.Port = 22
	}

	sshConn, err := uc.createSSHClient(&target, credential)
	if err != nil {
		return err
	}
	defer sshConn.Close()
	return nil
}

func (uc *AgentUseCase) createSSHClient(host *Host, credential *Credential) (*sshclient.Client, error) {
	var privateKey []byte
	if credential.Type == "password" && credential.Password == "" {
		return nil, fmt.Errorf("SSH 密码为空")
	}
	if credential.Type == "key" {
		if strings.TrimSpace(credential.PrivateKey) == "" {
			return nil, fmt.Errorf("SSH 私钥为空")
		}
		privateKey = []byte(credential.PrivateKey)
	}

	return sshclient.NewClient(
		host.IP,
		host.Port,
		host.SSHUser,
		credential.Password,
		privateKey,
		credential.Passphrase,
	)
}

func (uc *AgentUseCase) syncPrometheusTarget(host *Host) error {
	fileSDDir := strings.TrimSpace(uc.promCfg.FileSDDir)
	if !uc.promCfg.Enabled && fileSDDir == "" {
		return nil
	}

	targetIP := resolvePrometheusTargetIP(host)
	if targetIP == "" || host.AgentPort <= 0 {
		return nil
	}

	if fileSDDir == "" {
		fileSDDir = uc.promCfg.GetFileSDDir()
	}
	if err := os.MkdirAll(fileSDDir, 0o755); err != nil {
		return err
	}

	payload := []map[string]any{
		{
			"targets": []string{fmt.Sprintf("%s:%d", targetIP, host.AgentPort)},
			"labels": map[string]string{
				"job":       "opshub_agent",
				"host_id":   fmt.Sprintf("%d", host.ID),
				"host_name": host.Name,
				"os_type":   host.OSType,
			},
		},
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(fileSDDir, fmt.Sprintf("agent-%d.json", host.ID))
	return os.WriteFile(filePath, data, 0o644)
}

func resolvePrometheusTargetIP(host *Host) string {
	if host == nil {
		return ""
	}

	publicIP := normalizeReportedIP(host.PrimaryPublicIP)
	privateIP := normalizeReportedIP(host.PrimaryPrivateIP)
	hostIP := normalizeReportedIP(host.IP)

	// Cloud hosts are often only reachable from Prometheus via their public address.
	if strings.EqualFold(strings.TrimSpace(host.Type), "cloud") {
		if isPublicReportedIP(publicIP) {
			return publicIP
		}
		if isPublicReportedIP(hostIP) {
			return hostIP
		}
		if privateIP != "" {
			return privateIP
		}
		return hostIP
	}

	if privateIP != "" {
		return privateIP
	}
	if hostIP != "" {
		return hostIP
	}
	return publicIP
}

func (uc *AgentUseCase) removePrometheusTarget(hostID uint) error {
	return removePrometheusTargetFile(uc.promCfg, hostID)
}

func removePrometheusTargetFile(promCfg conf.PrometheusConfig, hostID uint) error {
	fileSDDir := strings.TrimSpace(promCfg.FileSDDir)
	if fileSDDir == "" {
		fileSDDir = promCfg.GetFileSDDir()
	}
	if fileSDDir == "" {
		return nil
	}

	filePath := filepath.Join(fileSDDir, fmt.Sprintf("agent-%d.json", hostID))
	err := os.Remove(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (uc *AgentUseCase) markLatestJobSuccess(ctx context.Context, hostID uint, message string) error {
	job, err := uc.jobRepo.GetLatestByHostID(ctx, hostID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if job.Status != AgentJobStatusRunning {
		return nil
	}
	switch job.JobType {
	case AgentJobTypeDeploy, AgentJobTypeReinstall, AgentJobTypeRestart:
	default:
		return nil
	}
	return uc.markJobSuccess(ctx, job, "running", message)
}

func (uc *AgentUseCase) markJobRunning(ctx context.Context, job *AssetAgentJob, progress int, stage, message string) error {
	job.Status = AgentJobStatusRunning
	job.Progress = progress
	job.Stage = stage
	job.Message = message
	job.Error = ""
	return uc.jobRepo.Update(ctx, job)
}

func (uc *AgentUseCase) markJobSuccess(ctx context.Context, job *AssetAgentJob, stage, message string) error {
	now := time.Now()
	job.Status = AgentJobStatusSuccess
	job.Progress = 100
	job.Stage = stage
	job.Message = message
	job.Error = ""
	job.FinishedAt = &now
	return uc.jobRepo.Update(ctx, job)
}

func (uc *AgentUseCase) markJobFailed(ctx context.Context, job *AssetAgentJob, stage, message string, err error) error {
	now := time.Now()
	job.Status = AgentJobStatusFailed
	job.Stage = stage
	job.Message = message
	if err != nil {
		job.Error = err.Error()
	}
	job.FinishedAt = &now
	return uc.jobRepo.Update(ctx, job)
}

func toAgentJobVO(job *AssetAgentJob) *AgentJobVO {
	vo := &AgentJobVO{
		ID:         job.ID,
		HostID:     job.HostID,
		JobType:    job.JobType,
		Status:     job.Status,
		Progress:   job.Progress,
		Stage:      job.Stage,
		Message:    job.Message,
		Error:      job.Error,
		UpdateTime: job.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if job.StartedAt != nil {
		vo.StartedAt = job.StartedAt.Format("2006-01-02 15:04:05")
	}
	if job.FinishedAt != nil {
		vo.FinishedAt = job.FinishedAt.Format("2006-01-02 15:04:05")
	}
	return vo
}

func resolvedAgentDisplayStatus(agentModel *AssetAgent) string {
	if agentModel.Status == AgentStatusRunning && agentModel.LastHeartbeatAt != nil && time.Since(*agentModel.LastHeartbeatAt) > agentHeartbeatTimeout {
		return AgentStatusOffline
	}
	if agentModel.Status == AgentStatusRunning && agentModel.LastHeartbeatAt == nil {
		return AgentStatusWaiting
	}
	return agentModel.Status
}

func dedupeHostIDs(hostIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(hostIDs))
	items := make([]uint, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		if hostID == 0 {
			continue
		}
		if _, ok := seen[hostID]; ok {
			continue
		}
		seen[hostID] = struct{}{}
		items = append(items, hostID)
	}
	return items
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func applyHostInventoryIPFallback(vo *HostInventoryVO, host *Host) {
	if vo == nil || host == nil {
		return
	}
	vo.PrivateIPs = prependUniqueReportedIP(vo.PrivateIPs, host.PrimaryPrivateIP, isPrivateReportedIP)
	vo.PublicIPs = prependUniqueReportedIP(vo.PublicIPs, host.PrimaryPublicIP, isPublicReportedIP)
}

func prependUniqueReportedIP(items []string, preferred string, validator func(string) bool) []string {
	preferred = normalizeReportedIP(preferred)
	normalized := make([]string, 0, len(items)+1)
	seen := make(map[string]struct{}, len(items)+1)

	if preferred != "" && (validator == nil || validator(preferred)) {
		seen[preferred] = struct{}{}
		normalized = append(normalized, preferred)
	}

	for _, item := range items {
		item = normalizeReportedIP(item)
		if item == "" {
			continue
		}
		if validator != nil && !validator(item) {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	return normalized
}

func (uc *AgentUseCase) listPublicIPHistory(ctx context.Context, hostID uint) []HostPublicIPHistoryVO {
	if uc.publicIPHistoryRepo == nil {
		return []HostPublicIPHistoryVO{}
	}
	items, err := uc.publicIPHistoryRepo.ListByHostID(ctx, hostID, 10)
	if err != nil || len(items) == 0 {
		return []HostPublicIPHistoryVO{}
	}
	result := make([]HostPublicIPHistoryVO, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, HostPublicIPHistoryVO{
			IP:          strings.TrimSpace(item.IP),
			Source:      strings.TrimSpace(item.Source),
			FirstSeenAt: item.FirstSeenAt.Format("2006-01-02 15:04:05"),
			LastSeenAt:  item.LastSeenAt.Format("2006-01-02 15:04:05"),
			SeenCount:   item.SeenCount,
			IsCurrent:   item.IsCurrent,
		})
	}
	return result
}

func buildPublicIPHistoryFallback(host *Host) []HostPublicIPHistoryVO {
	if host == nil || !isPublicReportedIP(strings.TrimSpace(host.PrimaryPublicIP)) {
		return []HostPublicIPHistoryVO{}
	}

	observedAt := host.UpdatedAt
	for _, item := range []*time.Time{host.AgentLastReportAt, host.LastCollectAt, host.LastSeen} {
		if item != nil {
			observedAt = *item
			break
		}
	}

	timeText := observedAt.Format("2006-01-02 15:04:05")
	return []HostPublicIPHistoryVO{
		{
			IP:          strings.TrimSpace(host.PrimaryPublicIP),
			Source:      "host_snapshot",
			FirstSeenAt: timeText,
			LastSeenAt:  timeText,
			SeenCount:   1,
			IsCurrent:   true,
		},
	}
}

func renderAgentUninstallShellScript(serviceName, installPath string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

service_name=%q
agent_home=%q
binary_path="${agent_home}/opshub-agent"
config_path="${agent_home}/config.json"

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop "${service_name}" >/dev/null 2>&1 || true
  systemctl disable "${service_name}" >/dev/null 2>&1 || true
  rm -f "/etc/systemd/system/${service_name}.service"
  systemctl daemon-reload >/dev/null 2>&1 || true
fi

pkill -f "${binary_path} --config ${config_path}" >/dev/null 2>&1 || true
rm -rf "${agent_home}"
echo "OpsHub Agent 已卸载"
`, defaultString(serviceName, "opshub-agent"), defaultString(installPath, "/opt/opshub-agent"))
}

func hasAgentActivitySince(agentModel *AssetAgent, startedAt time.Time) bool {
	if agentModel == nil {
		return false
	}

	for _, timestamp := range []*time.Time{agentModel.LastReportAt, agentModel.LastHeartbeatAt, agentModel.DeployedAt} {
		if timestamp != nil && !timestamp.Before(startedAt) {
			return true
		}
	}
	return false
}

func (uc *AgentUseCase) waitForAgentActivity(ctx context.Context, hostID uint, startedAt time.Time, timeout time.Duration) (*AssetAgent, bool) {
	deadline := time.Now().Add(timeout)
	for {
		agentModel, err := uc.agentRepo.GetByHostID(ctx, hostID)
		if err == nil && hasAgentActivitySince(agentModel, startedAt) {
			return agentModel, true
		}
		if time.Now().After(deadline) {
			return agentModel, false
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(2 * time.Second):
		}
	}
}
