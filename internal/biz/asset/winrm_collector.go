package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	winrm "github.com/masterzen/winrm"
	"github.com/ydcloud-dy/opshub/pkg/collector"
)

const winRMCollectScript = `
$ErrorActionPreference = 'Stop'

$os = Get-CimInstance Win32_OperatingSystem
$cpu = @(Get-CimInstance Win32_Processor)
$disks = @(
  Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ForEach-Object {
    $total = [int64]$_.Size
    $free = [int64]$_.FreeSpace
    $used = $total - $free
    [ordered]@{
      device = $_.DeviceID
      mountPoint = $_.DeviceID
      fstype = $_.FileSystem
      total = $total
      used = if ($used -gt 0) { $used } else { 0 }
      free = if ($free -gt 0) { $free } else { 0 }
      usage = if ($total -gt 0) { [math]::Round(($used * 100.0) / $total, 2) } else { 0 }
    }
  }
)

$totalMemory = [int64]$os.TotalVisibleMemorySize * 1024
$freeMemory = [int64]$os.FreePhysicalMemory * 1024
$usedMemory = $totalMemory - $freeMemory
if ($usedMemory -lt 0) { $usedMemory = 0 }

$cpuModel = ''
$cpuVendor = ''
$cpuMHz = 0
$cpuCores = 0
$cpuThreads = 0
$cpuUsage = 0

if ($cpu.Count -gt 0) {
  $cpuModel = [string]$cpu[0].Name
  $cpuVendor = [string]$cpu[0].Manufacturer
  $cpuMHz = [double]$cpu[0].MaxClockSpeed
  $cpuCores = [int](($cpu | Measure-Object -Property NumberOfCores -Sum).Sum)
  $cpuThreads = [int](($cpu | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum)
  $cpuUsageAvg = ($cpu | Measure-Object -Property LoadPercentage -Average).Average
  if ($null -ne $cpuUsageAvg) {
    $cpuUsage = [math]::Round([double]$cpuUsageAvg, 2)
  }
}

$bootTime = [datetime]$os.LastBootUpTime
$uptimeSpan = (Get-Date) - $bootTime
$uptimeText = '{0}d {1}h {2}m' -f [int][math]::Floor($uptimeSpan.TotalDays), $uptimeSpan.Hours, $uptimeSpan.Minutes

$result = [ordered]@{
  os = [string]$os.Caption
  kernel = ('{0} Build {1}' -f $os.Version, $os.BuildNumber)
  arch = [string]$os.OSArchitecture
  hostname = [string]$os.CSName
  uptime = $uptimeText
  cpu = [ordered]@{
    modelName = $cpuModel
    cores = $cpuCores
    threads = $cpuThreads
    usage = $cpuUsage
    mHz = $cpuMHz
    cache = ''
    vendorId = $cpuVendor
  }
  memory = [ordered]@{
    total = $totalMemory
    used = $usedMemory
    free = $freeMemory
    available = $freeMemory
    usage = if ($totalMemory -gt 0) { [math]::Round(($usedMemory * 100.0) / $totalMemory, 2) } else { 0 }
    swapTotal = 0
    swapUsed = 0
  }
  disk = $disks
}

$result | ConvertTo-Json -Depth 5 -Compress
`

type winRMCollectOutput struct {
	OS       string               `json:"os"`
	Kernel   string               `json:"kernel"`
	Arch     string               `json:"arch"`
	Hostname string               `json:"hostname"`
	Uptime   string               `json:"uptime"`
	CPU      collector.CPUInfo    `json:"cpu"`
	Memory   collector.MemoryInfo `json:"memory"`
	Disk     []collector.DiskInfo `json:"disk"`
}

type winRMAuthMode string

const (
	winRMAuthNTLM  winRMAuthMode = "ntlm"
	winRMAuthBasic winRMAuthMode = "basic"
)

func (uc *HostUseCase) resolveWinRMHost(host *Host) (*Host, uint, error) {
	credentialID := uc.effectiveManagementCredentialID(host)
	if credentialID == 0 {
		return nil, 0, fmt.Errorf("主机未配置 WinRM 凭证")
	}

	target := *host
	target.ManagementPort = uc.effectiveManagementPort(host)
	if target.ManagementPort == 0 {
		target.ManagementPort = 5985
	}

	return &target, credentialID, nil
}

func winRMNoProxyFunc(*http.Request) (*url.URL, error) {
	return nil, nil
}

func (uc *HostUseCase) createWinRMClient(host *Host, credential *Credential, authMode winRMAuthMode) (*winrm.Client, error) {
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

	port := uc.effectiveManagementPort(host)
	if port == 0 {
		port = 5985
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

func formatWinRMUsername(credential *Credential) string {
	username := strings.TrimSpace(credential.Username)
	if username == "" {
		return username
	}

	if credential.Domain == "" || strings.Contains(username, "\\") || strings.Contains(username, "@") {
		return username
	}

	return credential.Domain + `\` + username
}

func trimWinRMOutput(output string) string {
	return strings.TrimSpace(strings.TrimPrefix(output, "\ufeff"))
}

func normalizeWinRMError(err error, credential *Credential) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "http response error: 401 - invalid content type") {
		return fmt.Errorf("WinRM认证失败: 用户 %q 未通过认证，请检查密码，并确认该用户属于 Administrators 或 Remote Management Users", credential.Username)
	}

	return err
}

func shouldFallbackToWinRMBasic(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "http response error: 401 - invalid content type")
}

func (uc *HostUseCase) runWinRMPS(ctx context.Context, host *Host, credential *Credential, command string) (string, string, int, error) {
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
		return "", "", 0, err
	}

	if firstErr != nil {
		return "", "", 0, fmt.Errorf("%w，已尝试认证方式: %s", firstErr, strings.Join(tried, ", "))
	}

	return "", "", 0, fmt.Errorf("WinRM执行失败")
}

func (c *winRMHostCollector) Test(ctx context.Context, host *Host) error {
	target, credentialID, err := c.useCase.resolveWinRMHost(host)
	if err != nil {
		return err
	}

	credential, err := c.useCase.credentialRepo.GetByIDDecrypted(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("获取凭证失败: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stdout, stderr, exitCode, err := c.useCase.runWinRMPS(cmdCtx, target, credential, "Write-Output 'ok'")
	if err != nil {
		return fmt.Errorf("WinRM连接测试失败: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("WinRM连接测试失败: exit=%d stderr=%s", exitCode, trimWinRMOutput(stderr))
	}
	if trimWinRMOutput(stdout) == "" {
		return fmt.Errorf("WinRM连接测试失败: 未收到有效响应")
	}

	return nil
}

func (c *winRMHostCollector) Collect(ctx context.Context, host *Host) (*collector.SystemInfo, error) {
	target, credentialID, err := c.useCase.resolveWinRMHost(host)
	if err != nil {
		return nil, err
	}

	credential, err := c.useCase.credentialRepo.GetByIDDecrypted(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("获取凭证失败: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	stdout, stderr, exitCode, err := c.useCase.runWinRMPS(cmdCtx, target, credential, winRMCollectScript)
	if err != nil {
		return nil, fmt.Errorf("WinRM执行失败: %w", err)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("WinRM执行失败: exit=%d stderr=%s", exitCode, trimWinRMOutput(stderr))
	}

	output := trimWinRMOutput(stdout)
	if output == "" {
		return nil, fmt.Errorf("WinRM执行失败: 未返回采集数据")
	}

	var payload winRMCollectOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, fmt.Errorf("解析WinRM采集结果失败: %w", err)
	}

	if payload.CPU.Threads == 0 && payload.CPU.Cores > 0 {
		payload.CPU.Threads = payload.CPU.Cores
	}
	if payload.CPU.Cores == 0 && payload.CPU.Threads > 0 {
		payload.CPU.Cores = payload.CPU.Threads
	}

	return &collector.SystemInfo{
		OS:       strings.TrimSpace(payload.OS),
		Kernel:   strings.TrimSpace(payload.Kernel),
		Arch:     strings.TrimSpace(payload.Arch),
		CPU:      payload.CPU,
		Memory:   payload.Memory,
		Disk:     payload.Disk,
		Uptime:   strings.TrimSpace(payload.Uptime),
		Hostname: strings.TrimSpace(payload.Hostname),
	}, nil
}
