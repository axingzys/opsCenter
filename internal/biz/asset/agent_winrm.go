package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	winrm "github.com/masterzen/winrm"
	"gorm.io/gorm"
)

func (uc *AgentUseCase) deployWindowsOne(ctx context.Context, host *Host, operatorID uint, baseURL string) (*AgentJobVO, error) {
	mode := NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID)
	if mode != ManagementModeAgent {
		return nil, fmt.Errorf("Windows 主机需先切换为 Agent 管理方式")
	}

	now := time.Now()
	job := &AssetAgentJob{
		HostID:     host.ID,
		JobType:    AgentJobTypeDeploy,
		Status:     AgentJobStatusRunning,
		Progress:   5,
		Stage:      "building",
		Message:    "准备生成安装脚本",
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
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}

	targetHost, credential, err := uc.resolveWindowsDeployTarget(ctx, host)
	if err != nil {
		_ = uc.markJobFailed(ctx, job, "building", "校验 WinRM 信息失败", err)
		agentModel.Status = AgentStatusError
		agentModel.LastError = err.Error()
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), err
	}

	if err := uc.markJobRunning(ctx, job, 25, "configuring", "通过 WinRM 执行远程安装脚本"); err != nil {
		return toAgentJobVO(job), err
	}
	agentModel.Status = AgentStatusDeploying
	agentModel.InstallProgress = 25
	agentModel.InstallStage = "configuring"
	if err := uc.agentRepo.Update(ctx, agentModel); err != nil {
		return toAgentJobVO(job), err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	stdout, stderr, exitCode, err := uc.runWinRMPS(cmdCtx, targetHost, credential, bootstrap.InstallCommand)
	if err != nil {
		runErr := formatWinRMExecutionError(err, stdout, stderr, exitCode)
		_ = uc.markJobFailed(ctx, job, "configuring", "远程安装脚本执行失败", runErr)
		agentModel.Status = AgentStatusError
		agentModel.LastError = runErr.Error()
		agentModel.InstallStage = "configuring"
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), runErr
	}
	if exitCode != 0 {
		runErr := formatWinRMExecutionError(fmt.Errorf("PowerShell 脚本退出码 %d", exitCode), stdout, stderr, exitCode)
		_ = uc.markJobFailed(ctx, job, "configuring", "远程安装脚本执行失败", runErr)
		agentModel.Status = AgentStatusError
		agentModel.LastError = runErr.Error()
		agentModel.InstallStage = "configuring"
		_ = uc.agentRepo.Update(ctx, agentModel)
		return toAgentJobVO(job), runErr
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

	if err := uc.markJobRunning(ctx, job, 90, "waiting_register", "安装脚本执行完成，等待首次上报"); err != nil {
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

func (uc *AgentUseCase) uninstallWindowsOne(ctx context.Context, host *Host, operatorID uint) (*AgentJobVO, error) {
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
			InstallPath: defaultAgentInstallPath(host, uc.agentCfg),
			ServiceName: defaultAgentServiceName(host, uc.agentCfg),
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

	if shouldCleanupWindowsAgentWithoutRemote(host, agentModel, missingAgentRecord) {
		return uc.cleanupWindowsAgentPlatformState(ctx, host, agentModel, missingAgentRecord, job, "未检测到已注册 Agent，直接清理平台侧记录")
	}

	targetHost, credential, err := uc.resolveWindowsDeployTarget(ctx, host)
	if err != nil {
		if shouldCleanupWindowsAgentAfterRemoteUnavailable(err) {
			return uc.cleanupWindowsAgentPlatformState(ctx, host, agentModel, missingAgentRecord, job, "WinRM 凭证不可用，已清理平台侧 Agent 记录")
		}
		_ = uc.markJobFailed(ctx, job, "removing", "校验 WinRM 信息失败", err)
		return toAgentJobVO(job), err
	}

	if err := uc.markJobRunning(ctx, job, 35, "removing", "通过 WinRM 执行远程卸载脚本"); err != nil {
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

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	stdout, stderr, exitCode, err := uc.runWinRMPS(cmdCtx, targetHost, credential, renderAgentWindowsUninstallScript(host.ID))
	if err != nil {
		runErr := formatWinRMExecutionError(err, stdout, stderr, exitCode)
		if shouldCleanupWindowsAgentAfterRemoteUnavailable(runErr) {
			return uc.cleanupWindowsAgentPlatformState(ctx, host, agentModel, missingAgentRecord, job, "远程卸载不可用，已清理平台侧 Agent 记录")
		}
		_ = uc.markJobFailed(ctx, job, "removing", "远程卸载脚本执行失败", runErr)
		if !missingAgentRecord {
			agentModel.Status = AgentStatusError
			agentModel.LastError = runErr.Error()
			agentModel.InstallStage = "removing"
			_ = uc.agentRepo.Update(ctx, agentModel)
		}
		return toAgentJobVO(job), runErr
	}
	if exitCode != 0 {
		runErr := formatWinRMExecutionError(fmt.Errorf("PowerShell 脚本退出码 %d", exitCode), stdout, stderr, exitCode)
		_ = uc.markJobFailed(ctx, job, "removing", "远程卸载脚本执行失败", runErr)
		if !missingAgentRecord {
			agentModel.Status = AgentStatusError
			agentModel.LastError = runErr.Error()
			agentModel.InstallStage = "removing"
			_ = uc.agentRepo.Update(ctx, agentModel)
		}
		return toAgentJobVO(job), runErr
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

func shouldCleanupWindowsAgentAfterRemoteUnavailable(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	patterns := []string{
		"主机未配置 winrm 凭证",
		"获取 winrm 凭证失败",
		"windows 自动部署需要 winrm 凭证",
		"当前管理方式需要 winrm 凭证",
		"winrm 凭证未配置",
		"winrm认证失败",
		"winrm 认证失败",
		"connection refused",
		"connect: connection refused",
		"no route to host",
		"i/o timeout",
		"connection timed out",
		"context deadline exceeded",
		"connectex:",
		"actively refused",
		"dial tcp",
	}
	for _, pattern := range patterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

func shouldCleanupWindowsAgentWithoutRemote(host *Host, agentModel *AssetAgent, missingAgentRecord bool) bool {
	if hasWindowsAgentRuntimeEvidence(host, agentModel) {
		return false
	}
	if missingAgentRecord || agentModel == nil {
		return true
	}

	switch agentModel.Status {
	case AgentStatusPending, AgentStatusError, AgentStatusUninstalling, AgentStatusUninstalled:
		return true
	default:
		return false
	}
}

func hasWindowsAgentRuntimeEvidence(host *Host, agentModel *AssetAgent) bool {
	if host != nil {
		if strings.TrimSpace(host.AgentID) != "" || strings.TrimSpace(host.AgentVersion) != "" {
			return true
		}
		if host.AgentLastHeartbeatAt != nil || host.AgentLastReportAt != nil {
			return true
		}
	}
	if agentModel != nil {
		if strings.TrimSpace(agentModel.AgentID) != "" || strings.TrimSpace(agentModel.Version) != "" {
			return true
		}
		if agentModel.LastHeartbeatAt != nil || agentModel.LastReportAt != nil {
			return true
		}
	}
	return false
}

func (uc *AgentUseCase) cleanupWindowsAgentPlatformState(ctx context.Context, host *Host, agentModel *AssetAgent, missingAgentRecord bool, job *AssetAgentJob, message string) (*AgentJobVO, error) {
	revokeAgentIdentity := hasWindowsAgentRuntimeEvidence(host, agentModel)
	revokedAgentID := buildRevokedAgentID(host, agentModel)

	if err := uc.markJobRunning(ctx, job, 75, "removing", message); err != nil {
		return toAgentJobVO(job), err
	}

	if err := uc.inventoryRepo.DeleteByHostID(ctx, host.ID); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "清理库存快照失败", err)
		return toAgentJobVO(job), err
	}
	if !missingAgentRecord && agentModel != nil {
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
	if revokeAgentIdentity {
		host.AgentID = revokedAgentID
		host.CollectError = "Agent 已从平台清理"
	}
	if err := uc.hostRepo.Update(ctx, host); err != nil {
		_ = uc.markJobFailed(ctx, job, "removing", "更新主机状态失败", err)
		return toAgentJobVO(job), err
	}

	if err := uc.markJobSuccess(ctx, job, "removed", "Agent 记录已清理"); err != nil {
		return toAgentJobVO(job), err
	}
	return toAgentJobVO(job), nil
}

func buildRevokedAgentID(host *Host, agentModel *AssetAgent) string {
	for _, value := range []string{
		stringFromAgentModel(agentModel),
		stringFromHostAgent(host),
	} {
		value = strings.TrimSpace(value)
		if value != "" {
			return "revoked:" + value
		}
	}
	if host == nil {
		return fmt.Sprintf("revoked:%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("revoked:%d:%d", host.ID, time.Now().Unix())
}

func stringFromAgentModel(agentModel *AssetAgent) string {
	if agentModel == nil {
		return ""
	}
	return agentModel.AgentID
}

func stringFromHostAgent(host *Host) string {
	if host == nil {
		return ""
	}
	return host.AgentID
}

func (uc *AgentUseCase) resolveWindowsDeployTarget(ctx context.Context, host *Host) (*Host, *Credential, error) {
	if host == nil {
		return nil, nil, fmt.Errorf("主机不存在")
	}
	if host.OSType != OSTypeWindows {
		return nil, nil, fmt.Errorf("当前主机不是 Windows")
	}

	credentialID := host.ManagementCredentialID
	if credentialID == 0 {
		credentialID = host.CredentialID
	}
	if credentialID == 0 {
		return nil, nil, fmt.Errorf("主机未配置 WinRM 凭证")
	}

	credential, err := uc.credentialRepo.GetByIDDecrypted(ctx, credentialID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取 WinRM 凭证失败: %w", err)
	}
	if normalizeCredentialProtocol(credential.Protocol) != ManagementModeWinRM {
		return nil, nil, fmt.Errorf("Windows 自动部署需要 WinRM 凭证")
	}

	target := *host
	target.ManagementPort = host.ManagementPort
	if target.ManagementPort <= 0 || target.ManagementPort == host.AgentPort || target.ManagementPort == uc.agentCfg.GetDefaultListenPort() {
		target.ManagementPort = windowsWinRMPort
	}

	return &target, credential, nil
}

func (uc *AgentUseCase) createWinRMClient(host *Host, credential *Credential, authMode winRMAuthMode) (*winrm.Client, error) {
	if normalizeCredentialProtocol(credential.Protocol) != ManagementModeWinRM {
		return nil, fmt.Errorf("当前管理方式需要 WinRM 凭证")
	}
	if strings.TrimSpace(credential.Username) == "" {
		return nil, fmt.Errorf("WinRM 凭证未配置用户名")
	}
	if credential.Type != "password" {
		return nil, fmt.Errorf("WinRM 凭证仅支持密码认证")
	}
	if credential.Password == "" {
		return nil, fmt.Errorf("WinRM 凭证未配置密码")
	}

	port := host.ManagementPort
	if port <= 0 {
		port = windowsWinRMPort
	}
	useHTTPS := port == 5986
	endpoint := winrm.NewEndpoint(host.IP, port, useHTTPS, true, nil, nil, nil, 30*time.Second)

	params := *winrm.DefaultParameters
	switch authMode {
	case winRMAuthBasic:
		params.TransportDecorator = func() winrm.Transporter {
			return winrm.NewClientWithProxyFunc(winRMNoProxyFunc)
		}
	default:
		params.TransportDecorator = func() winrm.Transporter {
			return winrm.NewClientNTLMWithProxyFunc(winRMNoProxyFunc)
		}
	}

	client, err := winrm.NewClientWithParameters(endpoint, formatWinRMUsername(credential), credential.Password, &params)
	if err != nil {
		return nil, fmt.Errorf("创建WinRM连接失败: %w", err)
	}
	return client, nil
}

func (uc *AgentUseCase) runWinRMPS(ctx context.Context, host *Host, credential *Credential, command string) (string, string, int, error) {
	authModes := []winRMAuthMode{winRMAuthNTLM, winRMAuthBasic}
	var firstErr error
	var tried []string

	for _, authMode := range authModes {
		client, err := uc.createWinRMClient(host, credential, authMode)
		if err != nil {
			return "", "", 0, err
		}

		stdout, stderr, exitCode, err := client.RunPSWithContext(ctx, command)
		if err == nil {
			return stdout, stderr, exitCode, nil
		}

		fallbackToBasic := authMode == winRMAuthNTLM && shouldFallbackToWinRMBasic(err)
		err = normalizeWinRMError(err, credential)
		if firstErr == nil {
			firstErr = err
		}
		tried = append(tried, string(authMode))
		if fallbackToBasic {
			continue
		}
		return stdout, stderr, exitCode, err
	}

	if firstErr != nil {
		return "", "", 0, fmt.Errorf("%w，已尝试认证方式: %s", firstErr, strings.Join(tried, ", "))
	}
	return "", "", 0, fmt.Errorf("WinRM执行失败")
}

func renderAgentWindowsUninstallScript(hostID uint) string {
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$serviceName = '%s'
$taskName = '%s'
$agentHome = '%s'
$binaryPath = Join-Path $agentHome 'opshub-agent.exe'

if (Test-Path -LiteralPath $binaryPath) {
  try { & $binaryPath service stop --name $serviceName | Out-Null } catch {}
  try { & $binaryPath service uninstall --name $serviceName | Out-Null } catch {}
}

$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($service) {
  try {
    if ($service.Status -ne 'Stopped') {
      Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue | Out-Null
      Start-Sleep -Seconds 2
    }
  } catch {}
  sc.exe delete $serviceName | Out-Null
}

$running = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
  $_.CommandLine -like '*OpsHubAgent\report.ps1*' -or $_.CommandLine -like '*OpsHubAgent\opshub-agent.exe*'
}
if ($running) {
  $running | ForEach-Object {
    try { Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop } catch {}
  }
}

$task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($task) {
  try { Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue | Out-Null } catch {}
  Unregister-ScheduledTask -TaskName $taskName -Confirm:$false | Out-Null
}

if (Test-Path -LiteralPath $agentHome) {
  Remove-Item -LiteralPath $agentHome -Recurse -Force
}

Write-Host 'OpsHub Windows Agent 已卸载'
`, windowsAgentTaskName(hostID), windowsAgentTaskName(hostID), windowsAgentHome)
}

func formatWinRMExecutionError(err error, stdout, stderr string, exitCode int) error {
	parts := []string{fmt.Sprintf("命令执行失败: %v", err)}
	if exitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit code: %d", exitCode))
	}
	if trimmed := trimWinRMOutput(stderr); trimmed != "" {
		parts = append(parts, fmt.Sprintf("stderr: %s", trimmed))
	}
	if trimmed := trimWinRMOutput(stdout); trimmed != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", trimmed))
	}
	return errors.New(strings.Join(parts, ", "))
}
