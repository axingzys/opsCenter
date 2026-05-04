package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ydcloud-dy/opshub/internal/conf"
	"github.com/ydcloud-dy/opshub/pkg/collector"
	"gorm.io/gorm"
)

const (
	agentRegisterTokenType = "asset_agent_register"
	agentAccessTokenType   = "asset_agent_access"

	agentHeartbeatTimeout = 3 * time.Minute
	agentPrometheusGrace  = 2 * time.Minute
	agentRegisterTokenTTL = 30 * time.Minute
	agentAccessTokenTTL   = 365 * 24 * time.Hour
	agentVersionMVP       = "opshub-agent/0.1.1"
	windowsAgentVersion   = agentVersionMVP
	windowsAgentHome      = `C:\ProgramData\OpsHubAgent`
	windowsWinRMPort      = 5985
)

type AgentUseCase struct {
	hostRepo            HostRepo
	credentialRepo      CredentialRepo
	agentRepo           AssetAgentRepo
	inventoryRepo       AssetHostInventoryRepo
	publicIPHistoryRepo AssetHostPublicIPHistoryRepo
	jobRepo             AssetAgentJobRepo
	signingKey          []byte
	agentCfg            conf.AgentConfig
	promCfg             conf.PrometheusConfig
}

type AgentBootstrapClaims struct {
	HostID    uint   `json:"host_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

type AgentAccessClaims struct {
	HostID    uint   `json:"host_id"`
	AgentID   string `json:"agent_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

type AgentBootstrapVO struct {
	HostID                uint   `json:"hostId"`
	HostName              string `json:"hostName"`
	RegistrationToken     string `json:"registrationToken"`
	InstallScriptURL      string `json:"installScriptUrl"`
	InstallCommand        string `json:"installCommand"`
	RegisterURL           string `json:"registerUrl"`
	ReportURL             string `json:"reportUrl"`
	ReportIntervalSeconds int    `json:"reportIntervalSeconds"`
	ExpiresAt             string `json:"expiresAt"`
}

type AgentRegisterRequest struct {
	RegistrationToken string `json:"registrationToken" binding:"required"`
	Version           string `json:"version"`
	Hostname          string `json:"hostname"`
	ListenPort        int    `json:"listenPort"`
}

type AgentRegisterResponse struct {
	AgentID               string `json:"agentId"`
	AccessToken           string `json:"accessToken"`
	ReportURL             string `json:"reportUrl"`
	ReportIntervalSeconds int    `json:"reportIntervalSeconds"`
}

type AgentReportRequest struct {
	Version        string                  `json:"version"`
	Hostname       string                  `json:"hostname"`
	OS             string                  `json:"os"`
	Kernel         string                  `json:"kernel"`
	Arch           string                  `json:"arch"`
	Uptime         string                  `json:"uptime"`
	ListenPort     int                     `json:"listenPort"`
	PrivateIPs     []string                `json:"privateIps"`
	PublicIPs      []string                `json:"publicIps"`
	Interfaces     []AgentNetworkInterface `json:"interfaces"`
	CPU            collector.CPUInfo       `json:"cpu"`
	Memory         collector.MemoryInfo    `json:"memory"`
	Disk           []collector.DiskInfo    `json:"disk"`
	TopProcesses   []AgentProcessInfo      `json:"topProcesses"`
	ListeningPorts []AgentPortInfo         `json:"listeningPorts"`
	ConfigSummary  AgentConfigSummary      `json:"configSummary"`
}

func NewAgentUseCase(
	hostRepo HostRepo,
	credentialRepo CredentialRepo,
	agentRepo AssetAgentRepo,
	inventoryRepo AssetHostInventoryRepo,
	publicIPHistoryRepo AssetHostPublicIPHistoryRepo,
	jobRepo AssetAgentJobRepo,
	jwtSecret string,
	agentCfg conf.AgentConfig,
	promCfg conf.PrometheusConfig,
) *AgentUseCase {
	return &AgentUseCase{
		hostRepo:            hostRepo,
		credentialRepo:      credentialRepo,
		agentRepo:           agentRepo,
		inventoryRepo:       inventoryRepo,
		publicIPHistoryRepo: publicIPHistoryRepo,
		jobRepo:             jobRepo,
		signingKey:          []byte(jwtSecret),
		agentCfg:            agentCfg,
		promCfg:             promCfg,
	}
}

func resolveAgentBaseURL(baseURL string) string {
	return strings.TrimSpace(strings.TrimRight(baseURL, "/"))
}

func (uc *AgentUseCase) GenerateBootstrap(ctx context.Context, hostID uint, baseURL string) (*AgentBootstrapVO, error) {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机信息失败: %w", err)
	}

	mode := NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID)
	if host.OSType == OSTypeWindows && mode != ManagementModeAgent {
		return nil, fmt.Errorf("Windows 主机仅在 Agent 管理模式下支持安装引导")
	}
	if host.OSType == OSTypeLinux && mode == ManagementModeNone {
		return nil, fmt.Errorf("当前主机未配置可用的管理方式")
	}

	token, expiresAt, err := uc.generateRegisterToken(host.ID)
	if err != nil {
		return nil, fmt.Errorf("生成 Agent 注册令牌失败: %w", err)
	}

	baseURL = resolveAgentBaseURL(baseURL)
	registerURL := fmt.Sprintf("%s/api/v1/public/agents/register", baseURL)
	reportURL := fmt.Sprintf("%s/api/v1/public/agents/report", baseURL)

	scriptURL := fmt.Sprintf("%s/api/v1/public/agents/install.sh?token=%s", baseURL, url.QueryEscape(token))
	installCommand := fmt.Sprintf("curl -fsSL '%s' | bash", scriptURL)
	if host.OSType == OSTypeWindows {
		scriptURL = fmt.Sprintf("%s/api/v1/public/agents/install.ps1?token=%s", baseURL, url.QueryEscape(token))
		installCommand = fmt.Sprintf(`powershell.exe -ExecutionPolicy Bypass -NoProfile -Command "Invoke-RestMethod -UseBasicParsing -Uri '%s' | Invoke-Expression"`, scriptURL)
	}

	return &AgentBootstrapVO{
		HostID:                host.ID,
		HostName:              host.Name,
		RegistrationToken:     token,
		InstallScriptURL:      scriptURL,
		InstallCommand:        installCommand,
		RegisterURL:           registerURL,
		ReportURL:             reportURL,
		ReportIntervalSeconds: uc.agentCfg.GetReportIntervalSeconds(),
		ExpiresAt:             expiresAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *AgentUseCase) RenderPowerShellInstallScript(ctx context.Context, registrationToken, baseURL string) (string, error) {
	claims, err := uc.parseRegisterToken(registrationToken)
	if err != nil {
		return "", err
	}

	host, err := uc.hostRepo.GetByID(ctx, claims.HostID)
	if err != nil {
		return "", fmt.Errorf("获取主机信息失败: %w", err)
	}
	if host.OSType != OSTypeWindows {
		return "", fmt.Errorf("当前主机不是 Windows")
	}
	if NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID) != ManagementModeAgent {
		return "", fmt.Errorf("当前主机未启用 Agent 管理方式")
	}

	baseURL = resolveAgentBaseURL(baseURL)
	return renderAgentPowerShellScript(baseURL, registrationToken, host.ID, uc.agentCfg.GetReportIntervalSeconds()), nil
}

func (uc *AgentUseCase) RenderShellInstallScript(ctx context.Context, registrationToken, baseURL string) (string, error) {
	claims, err := uc.parseRegisterToken(registrationToken)
	if err != nil {
		return "", err
	}

	host, err := uc.hostRepo.GetByID(ctx, claims.HostID)
	if err != nil {
		return "", fmt.Errorf("获取主机信息失败: %w", err)
	}
	if host.OSType != OSTypeLinux {
		return "", fmt.Errorf("当前主机不是 Linux")
	}
	if NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID) == ManagementModeNone {
		return "", fmt.Errorf("当前主机未配置可用的管理方式")
	}

	baseURL = resolveAgentBaseURL(baseURL)
	return uc.renderAgentShellScript(baseURL, registrationToken, host.ID), nil
}

func (uc *AgentUseCase) Register(ctx context.Context, baseURL string, req *AgentRegisterRequest) (*AgentRegisterResponse, error) {
	claims, err := uc.parseRegisterToken(req.RegistrationToken)
	if err != nil {
		return nil, err
	}

	host, err := uc.hostRepo.GetByID(ctx, claims.HostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机信息失败: %w", err)
	}

	mode := NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID)
	if host.OSType == OSTypeWindows && mode != ManagementModeAgent {
		return nil, fmt.Errorf("当前主机未启用 Agent 管理方式")
	}
	if host.OSType == OSTypeLinux && mode == ManagementModeNone {
		return nil, fmt.Errorf("当前主机未配置可用的管理方式")
	}

	agentID := uuid.NewString()
	accessToken, err := uc.generateAccessToken(host.ID, agentID)
	if err != nil {
		return nil, fmt.Errorf("生成 Agent 访问令牌失败: %w", err)
	}

	now := time.Now()
	listenPort := req.ListenPort
	if listenPort <= 0 {
		listenPort = host.AgentPort
	}
	if listenPort <= 0 {
		listenPort = uc.agentCfg.GetDefaultListenPort()
	}

	host.AgentID = agentID
	host.AgentVersion = strings.TrimSpace(req.Version)
	host.AgentPort = listenPort
	host.AgentLastHeartbeatAt = &now
	host.LastSeen = &now
	host.Status = 1
	host.CollectStatus = CollectStatusUnknown
	host.CollectError = ""
	host.AgentLastError = ""
	if host.OSType == OSTypeLinux {
		host.ManagementMode = ManagementModeAgent
		host.ManagementPort = listenPort
		if host.ManagementCredentialID == 0 && host.CredentialID > 0 {
			host.ManagementCredentialID = host.CredentialID
		}
	}

	if err := uc.hostRepo.Update(ctx, host); err != nil {
		return nil, fmt.Errorf("保存 Agent 注册信息失败: %w", err)
	}

	agentModel, created, err := uc.getOrCreateAgentModel(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("初始化 Agent 记录失败: %w", err)
	}
	agentModel.AgentID = agentID
	encryptedAccessToken, err := encryptAgentSecret(uc.signingKey, accessToken)
	if err != nil {
		return nil, fmt.Errorf("保存 Agent 访问令牌失败: %w", err)
	}
	agentModel.AccessToken = encryptedAccessToken
	agentModel.Version = strings.TrimSpace(req.Version)
	agentModel.Status = AgentStatusRunning
	agentModel.ListenPort = listenPort
	agentModel.InstallPath = defaultString(agentModel.InstallPath, defaultAgentInstallPath(host, uc.agentCfg))
	agentModel.ServiceName = defaultString(agentModel.ServiceName, defaultAgentServiceName(host, uc.agentCfg))
	agentModel.InstallProgress = 100
	agentModel.InstallStage = "running"
	agentModel.LastHeartbeatAt = &now
	agentModel.LastError = ""
	agentModel.DeployedAt = &now

	if err := uc.saveAgentModel(ctx, agentModel, created); err != nil {
		return nil, fmt.Errorf("保存 Agent 生命周期失败: %w", err)
	}

	if shouldSyncPrometheusTarget(host, agentModel.Version) {
		if err := uc.syncPrometheusTarget(host); err != nil {
			return nil, fmt.Errorf("刷新 Prometheus 目标失败: %w", err)
		}
	} else if err := uc.removePrometheusTarget(host.ID); err != nil {
		return nil, fmt.Errorf("清理 Prometheus 目标失败: %w", err)
	}

	_ = uc.markLatestJobSuccess(ctx, host.ID, "Agent 注册成功")

	baseURL = resolveAgentBaseURL(baseURL)
	return &AgentRegisterResponse{
		AgentID:               agentID,
		AccessToken:           accessToken,
		ReportURL:             fmt.Sprintf("%s/api/v1/public/agents/report", baseURL),
		ReportIntervalSeconds: uc.agentCfg.GetReportIntervalSeconds(),
	}, nil
}

func (uc *AgentUseCase) Report(ctx context.Context, accessToken string, req *AgentReportRequest) error {
	claims, err := uc.parseAccessToken(accessToken)
	if err != nil {
		return err
	}

	host, err := uc.hostRepo.GetByID(ctx, claims.HostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}

	if host.AgentID != "" && host.AgentID != claims.AgentID {
		return fmt.Errorf("Agent 身份无效或已失效")
	}

	now := time.Now()
	listenPort := req.ListenPort
	if listenPort <= 0 {
		listenPort = host.AgentPort
	}
	if listenPort <= 0 {
		listenPort = uc.agentCfg.GetDefaultListenPort()
	}

	host.AgentID = claims.AgentID
	host.AgentVersion = strings.TrimSpace(req.Version)
	host.AgentPort = listenPort
	host.AgentLastHeartbeatAt = &now
	host.AgentLastReportAt = &now
	host.AgentLastError = ""
	host.LastSeen = &now
	host.LastCollectAt = &now
	host.Status = 1
	host.CollectStatus = CollectStatusOnline
	host.CollectError = ""
	host.ManagementMode = ManagementModeAgent
	if host.OSType == OSTypeLinux {
		host.ManagementPort = listenPort
	}
	if host.OSType == OSTypeLinux && host.ManagementCredentialID == 0 && host.CredentialID > 0 {
		host.ManagementCredentialID = host.CredentialID
	}

	host.OS = strings.TrimSpace(req.OS)
	host.Kernel = strings.TrimSpace(req.Kernel)
	host.Arch = strings.TrimSpace(req.Arch)
	host.Hostname = strings.TrimSpace(req.Hostname)
	host.Uptime = strings.TrimSpace(req.Uptime)
	host.CPUCores = req.CPU.Threads
	host.CPUUsage = req.CPU.Usage
	host.MemoryTotal = req.Memory.Total
	host.MemoryUsed = req.Memory.Used
	host.MemoryUsage = req.Memory.Usage
	host.PrimaryPrivateIP = selectPrimaryPrivateIP(host.IP, req.PrivateIPs, req.Interfaces)
	detectedPublicIP := selectPrimaryPublicIP(req.PublicIPs)
	if shouldUpdatePrimaryPublicIP(host, detectedPublicIP) {
		host.PrimaryPublicIP = detectedPublicIP
	}

	summaryDisks := summarizeReportedDisks(req.Disk)
	var diskTotal, diskUsed uint64
	for _, disk := range summaryDisks {
		diskTotal += disk.Total
		diskUsed += disk.Used
	}
	host.DiskTotal = diskTotal
	host.DiskUsed = diskUsed
	if diskTotal > 0 {
		host.DiskUsage = float64(diskUsed) / float64(diskTotal) * 100
	} else {
		host.DiskUsage = 0
	}

	if cpuJSON, err := req.CPU.ToJSON(); err == nil {
		host.CPUInfo = cpuJSON
	}

	if err := uc.hostRepo.Update(ctx, host); err != nil {
		return fmt.Errorf("保存主机信息失败: %w", err)
	}
	if uc.publicIPHistoryRepo != nil && isPublicReportedIP(detectedPublicIP) {
		if err := uc.publicIPHistoryRepo.Observe(ctx, host.ID, detectedPublicIP, "agent_detected", now); err != nil {
			return fmt.Errorf("记录公网IP历史失败: %w", err)
		}
	}

	agentModel, created, err := uc.getOrCreateAgentModel(ctx, host)
	if err != nil {
		return fmt.Errorf("初始化 Agent 记录失败: %w", err)
	}
	agentModel.AgentID = claims.AgentID
	encryptedAccessToken, err := encryptAgentSecret(uc.signingKey, accessToken)
	if err != nil {
		return fmt.Errorf("保存 Agent 访问令牌失败: %w", err)
	}
	agentModel.AccessToken = encryptedAccessToken
	agentModel.Version = strings.TrimSpace(req.Version)
	agentModel.Status = AgentStatusRunning
	agentModel.ListenPort = listenPort
	agentModel.InstallPath = defaultString(agentModel.InstallPath, defaultAgentInstallPath(host, uc.agentCfg))
	agentModel.ServiceName = defaultString(agentModel.ServiceName, defaultAgentServiceName(host, uc.agentCfg))
	agentModel.InstallProgress = 100
	agentModel.InstallStage = "running"
	agentModel.LastHeartbeatAt = &now
	agentModel.LastReportAt = &now
	agentModel.LastError = ""
	if agentModel.DeployedAt == nil {
		agentModel.DeployedAt = &now
	}

	if err := uc.saveAgentModel(ctx, agentModel, created); err != nil {
		return fmt.Errorf("保存 Agent 生命周期失败: %w", err)
	}

	inventory := &AssetHostInventory{
		HostID:             host.ID,
		PrivateIPsJSON:     encodeJSON(req.PrivateIPs),
		PublicIPsJSON:      encodeJSON(req.PublicIPs),
		InterfacesJSON:     encodeJSON(req.Interfaces),
		DisksJSON:          encodeJSON(convertDiskInventory(summaryDisks)),
		TopProcessesJSON:   encodeJSON(req.TopProcesses),
		ListeningPortsJSON: encodeJSON(req.ListeningPorts),
		ConfigSummaryJSON:  encodeJSON(req.ConfigSummary),
		CollectedAt:        &now,
	}
	if err := uc.inventoryRepo.Upsert(ctx, inventory); err != nil {
		return fmt.Errorf("保存主机库存失败: %w", err)
	}

	if shouldSyncPrometheusTarget(host, strings.TrimSpace(req.Version)) {
		if err := uc.syncPrometheusTarget(host); err != nil {
			return fmt.Errorf("刷新 Prometheus 目标失败: %w", err)
		}
	} else {
		if err := uc.removePrometheusTarget(host.ID); err != nil {
			return fmt.Errorf("清理 Prometheus 目标失败: %w", err)
		}
	}

	_ = uc.markLatestJobSuccess(ctx, host.ID, "Agent 上报成功")
	return nil
}

func (uc *AgentUseCase) generateRegisterToken(hostID uint) (string, time.Time, error) {
	expiresAt := time.Now().Add(agentRegisterTokenTTL)
	claims := AgentBootstrapClaims{
		HostID:    hostID,
		TokenType: agentRegisterTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(uc.signingKey)
	return signed, expiresAt, err
}

func (uc *AgentUseCase) generateAccessToken(hostID uint, agentID string) (string, error) {
	claims := AgentAccessClaims{
		HostID:    hostID,
		AgentID:   agentID,
		TokenType: agentAccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(agentAccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(uc.signingKey)
}

func (uc *AgentUseCase) parseRegisterToken(tokenString string) (*AgentBootstrapClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AgentBootstrapClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return uc.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Agent 注册令牌无效: %w", err)
	}

	claims, ok := token.Claims.(*AgentBootstrapClaims)
	if !ok || !token.Valid || claims.TokenType != agentRegisterTokenType {
		return nil, fmt.Errorf("Agent 注册令牌无效")
	}

	return claims, nil
}

func (uc *AgentUseCase) parseAccessToken(tokenString string) (*AgentAccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AgentAccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return uc.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Agent 访问令牌无效: %w", err)
	}

	claims, ok := token.Claims.(*AgentAccessClaims)
	if !ok || !token.Valid || claims.TokenType != agentAccessTokenType {
		return nil, fmt.Errorf("Agent 访问令牌无效")
	}

	return claims, nil
}

func (uc *AgentUseCase) renderAgentShellScript(baseURL, registrationToken string, hostID uint) string {
	installPath := uc.agentCfg.GetDefaultInstallPath()
	binaryName := uc.agentCfg.GetBinaryName()
	serviceName := fmt.Sprintf("%s-%d", uc.agentCfg.GetServicePrefix(), hostID)
	registerURL := fmt.Sprintf("%s/api/v1/public/agents/register", baseURL)
	reportURL := fmt.Sprintf("%s/api/v1/public/agents/report", baseURL)
	downloadBaseURL := fmt.Sprintf("%s/api/v1/public/agents/download/linux", baseURL)
	listenPort := uc.agentCfg.GetDefaultListenPort()
	reportInterval := uc.agentCfg.GetReportIntervalSeconds()

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

agent_home=%q
binary_name=%q
binary_path="${agent_home}/${binary_name}"
binary_tmp_path="${binary_path}.tmp"
config_path="${agent_home}/config.json"
service_name=%q
registration_token=%q
register_url=%q
report_url_default=%q
download_base_url=%q
version=%q
default_listen_port=%d
default_interval=%d

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "unsupported" ;;
  esac
}

download_file() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
    return
  fi
  echo "curl 或 wget 不存在" >&2
  exit 1
}

post_json() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -H 'Content-Type: application/json' -d "$2" "$1"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO- --header='Content-Type: application/json' --post-data "$2" "$1"
    return
  fi
  echo "curl 或 wget 不存在" >&2
  exit 1
}

arch="$(detect_arch)"
if [[ "$arch" == "unsupported" ]]; then
  echo "不支持的 CPU 架构: $(uname -m)" >&2
  exit 1
fi

mkdir -p "$agent_home"
download_file "${download_base_url}/${arch}" "$binary_tmp_path"
chmod +x "$binary_tmp_path"
mv -f "$binary_tmp_path" "$binary_path"

hostname_value="$(hostname)"
register_body=$(cat <<JSON
{"registrationToken":"%s","version":"%s","hostname":"${hostname_value}","listenPort":%d}
JSON
)

register_response="$(post_json "$register_url" "$register_body")"
register_payload="$(printf '%%s' "$register_response" | "$binary_path" unwrap-response)"

agent_id="$(printf '%%s' "$register_payload" | "$binary_path" json-field agentId)"
access_token="$(printf '%%s' "$register_payload" | "$binary_path" json-field accessToken)"
report_url="$(printf '%%s' "$register_payload" | "$binary_path" json-field reportUrl)"
interval_value="$(printf '%%s' "$register_payload" | "$binary_path" json-field reportIntervalSeconds)"

if [[ -z "$report_url" ]]; then
  report_url="$report_url_default"
fi
if [[ -z "$interval_value" ]]; then
  interval_value="$default_interval"
fi

cat > "$config_path" <<JSON
{
  "agentId": "$agent_id",
  "accessToken": "$access_token",
  "reportUrl": "$report_url",
  "intervalSeconds": $interval_value,
  "listenAddr": "0.0.0.0:${default_listen_port}",
  "version": "%s",
  "hostId": %d
}
JSON

if command -v systemctl >/dev/null 2>&1; then
  cat > "/etc/systemd/system/${service_name}.service" <<UNIT
[Unit]
Description=OpsHub Agent (%d)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${binary_path} --config ${config_path}
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable "${service_name}"
  systemctl restart "${service_name}"
else
  pkill -f "$binary_path --config $config_path" >/dev/null 2>&1 || true
  nohup "$binary_path" --config "$config_path" >/var/log/${service_name}.log 2>&1 &
fi

sleep 2
echo "OpsHub Agent 安装完成"
echo "Agent ID: ${agent_id}"
echo "Report URL: ${report_url}"
`, installPath, binaryName, serviceName, registrationToken, registerURL, reportURL, downloadBaseURL, agentVersionMVP, listenPort, reportInterval, registrationToken, agentVersionMVP, listenPort, agentVersionMVP, hostID, hostID)
}

func renderAgentPowerShellScript(baseURL, registrationToken string, hostID uint, reportIntervalSeconds int) string {
	registerURL := fmt.Sprintf("%s/api/v1/public/agents/register", baseURL)
	reportURL := fmt.Sprintf("%s/api/v1/public/agents/report", baseURL)
	downloadBaseURL := fmt.Sprintf("%s/api/v1/public/agents/download/windows", baseURL)
	agentHome := windowsAgentHome
	configPath := `C:\ProgramData\OpsHubAgent\config.json`
	serviceName := windowsAgentTaskName(hostID)
	binaryPath := `C:\ProgramData\OpsHubAgent\opshub-agent.exe`
	legacyTaskName := windowsAgentTaskName(hostID)
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$agentHome = '%s'
$configPath = '%s'
$binaryPath = '%s'
$binaryTmpPath = $binaryPath + '.tmp'
$serviceName = '%s'
$legacyTaskName = '%s'
$registrationToken = '%s'
$registerUrl = '%s'
$reportUrl = '%s'
$downloadBaseUrl = '%s'
$version = '%s'
$defaultListenPort = 19100
$defaultInterval = %d

function Get-AgentArch {
  $arch = $env:PROCESSOR_ARCHITEW6432
  if ([string]::IsNullOrWhiteSpace($arch)) {
    $arch = $env:PROCESSOR_ARCHITECTURE
  }
  switch (($arch | ForEach-Object { $_.ToUpperInvariant() })) {
    'AMD64' { return 'amd64' }
    'ARM64' { return 'arm64' }
    default { throw ('不支持的 CPU 架构: ' + $arch) }
  }
}

function Download-File([string]$Uri, [string]$Path) {
  Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Path
}

New-Item -ItemType Directory -Force -Path $agentHome | Out-Null

$legacyTask = Get-ScheduledTask -TaskName $legacyTaskName -ErrorAction SilentlyContinue
if ($legacyTask) {
  try { Stop-ScheduledTask -TaskName $legacyTaskName -ErrorAction SilentlyContinue | Out-Null } catch {}
  Unregister-ScheduledTask -TaskName $legacyTaskName -Confirm:$false | Out-Null
}

$running = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
  $_.CommandLine -like '*OpsHubAgent\report.ps1*' -or $_.CommandLine -like '*OpsHubAgent\opshub-agent.exe*'
}
if ($running) {
  $running | ForEach-Object {
    try { Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop } catch {}
  }
}

$arch = Get-AgentArch
Download-File ($downloadBaseUrl + '/' + $arch) $binaryTmpPath
Move-Item -Force -Path $binaryTmpPath -Destination $binaryPath

$registerBody = @{
  registrationToken = $registrationToken
  version = $version
  hostname = $env:COMPUTERNAME
  listenPort = $defaultListenPort
} | ConvertTo-Json

$registerResponse = Invoke-RestMethod -Method POST -Uri $registerUrl -Body $registerBody -ContentType 'application/json'
$registerResult = if ($registerResponse.data) { $registerResponse.data } else { $registerResponse }
$reportInterval = if ($registerResult.reportIntervalSeconds) { [int]$registerResult.reportIntervalSeconds } else { $defaultInterval }

$config = [ordered]@{
  agentId = $registerResult.agentId
  accessToken = $registerResult.accessToken
  reportUrl = if ($registerResult.reportUrl) { $registerResult.reportUrl } else { $reportUrl }
  intervalSeconds = $reportInterval
  listenAddr = ('0.0.0.0:' + $defaultListenPort)
  version = $version
  hostId = %d
  serviceName = $serviceName
}
$config | ConvertTo-Json -Depth 4 | Set-Content -Encoding UTF8 -Path $configPath

$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
  try { & $binaryPath service stop --name $serviceName | Out-Null } catch {}
  try { & $binaryPath service uninstall --name $serviceName | Out-Null } catch {}
}

& $binaryPath service install --name $serviceName --config $configPath
& $binaryPath service start --name $serviceName
Start-Sleep -Seconds 2

$service = Get-Service -Name $serviceName -ErrorAction Stop
if ($service.Status -ne 'Running') {
  throw ('Windows Service 启动失败，当前状态: ' + $service.Status)
}

Write-Host ''
Write-Host 'OpsHub Agent 安装完成。'
Write-Host ('Agent ID: ' + $registerResult.agentId)
Write-Host ('上报地址: ' + $config.reportUrl)
Write-Host ('上报间隔: ' + $reportInterval + ' 秒')
Write-Host ('Windows Service: ' + $serviceName)
	`, agentHome, configPath, binaryPath, serviceName, legacyTaskName, registrationToken, registerURL, reportURL, downloadBaseURL, windowsAgentVersion, reportIntervalSeconds, hostID)
}

func IsAgentHeartbeatFresh(host *Host) bool {
	if host == nil || host.AgentLastHeartbeatAt == nil {
		return false
	}
	return time.Since(*host.AgentLastHeartbeatAt) <= agentHeartbeatTimeout
}

func AgentRuntimeState(host *Host) (int, string, string) {
	collectStatus := host.CollectStatus
	if collectStatus == "" {
		collectStatus = DefaultCollectStatus(ManagementModeAgent)
	}
	collectError := host.CollectError
	status := host.Status

	if host.AgentID == "" || host.AgentLastHeartbeatAt == nil {
		if collectStatus == CollectStatusUnknown {
			collectStatus = CollectStatusNotConfigured
		}
		if collectError == "" {
			collectError = "Agent 未注册或尚未上报"
		}
		return -1, collectStatus, collectError
	}

	if !IsAgentHeartbeatFresh(host) {
		if collectError == "" {
			collectError = "Agent 心跳超时"
		}
		return 0, CollectStatusOffline, collectError
	}

	if status == 0 && collectStatus == CollectStatusOffline && collectError != "" {
		return status, collectStatus, collectError
	}

	return 1, CollectStatusOnline, ""
}

func SystemInfoFromHost(host *Host) (*collector.SystemInfo, error) {
	if host == nil {
		return nil, errors.New("主机不存在")
	}

	info := &collector.SystemInfo{
		OS:       host.OS,
		Kernel:   host.Kernel,
		Arch:     host.Arch,
		Uptime:   host.Uptime,
		Hostname: host.Hostname,
		CPU: collector.CPUInfo{
			Threads: host.CPUCores,
			Cores:   host.CPUCores,
			Usage:   host.CPUUsage,
		},
		Memory: collector.MemoryInfo{
			Total:     host.MemoryTotal,
			Used:      host.MemoryUsed,
			Free:      0,
			Available: 0,
			Usage:     host.MemoryUsage,
		},
	}

	if host.MemoryTotal > host.MemoryUsed {
		info.Memory.Free = host.MemoryTotal - host.MemoryUsed
		info.Memory.Available = info.Memory.Free
	}

	if host.CPUInfo != "" {
		_ = json.Unmarshal([]byte(host.CPUInfo), &info.CPU)
	}

	if host.DiskTotal > 0 {
		free := uint64(0)
		if host.DiskTotal > host.DiskUsed {
			free = host.DiskTotal - host.DiskUsed
		}
		info.Disk = []collector.DiskInfo{
			{
				Device:     "all",
				MountPoint: "all",
				Fstype:     "",
				Total:      host.DiskTotal,
				Used:       host.DiskUsed,
				Free:       free,
				Usage:      host.DiskUsage,
			},
		}
	}

	return info, nil
}

func convertDiskInventory(disks []collector.DiskInfo) []collectorDiskVO {
	items := make([]collectorDiskVO, 0, len(disks))
	for _, disk := range disks {
		items = append(items, collectorDiskVO{
			Device:     disk.Device,
			MountPoint: disk.MountPoint,
			FSType:     disk.Fstype,
			Total:      disk.Total,
			Used:       disk.Used,
			Free:       disk.Free,
			Usage:      disk.Usage,
		})
	}
	return items
}

func selectPrimaryPrivateIP(managementIP string, reported []string, interfaces []AgentNetworkInterface) string {
	managementIP = normalizeReportedIP(managementIP)
	if isPrivateReportedIP(managementIP) {
		return managementIP
	}

	for _, iface := range interfaces {
		if shouldIgnorePrimaryInterface(iface.Name) {
			continue
		}
		for _, address := range iface.Addresses {
			address = normalizeReportedIP(address)
			if isPrivateReportedIP(address) {
				return address
			}
		}
	}

	for _, address := range reported {
		address = normalizeReportedIP(address)
		if isPrivateReportedIP(address) {
			return address
		}
	}

	return ""
}

func selectPrimaryPublicIP(reported []string) string {
	for _, address := range reported {
		address = normalizeReportedIP(address)
		if isPublicReportedIP(address) {
			return address
		}
	}
	return ""
}

func shouldUpdatePrimaryPublicIP(host *Host, detected string) bool {
	detected = normalizeReportedIP(detected)
	if !isPublicReportedIP(detected) {
		return false
	}
	if host == nil {
		return true
	}
	if host.Type == "cloud" && strings.TrimSpace(host.PrimaryPublicIP) != "" {
		return false
	}
	return true
}

func normalizeReportedIP(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		address = host
	}
	if strings.Contains(address, "/") {
		address = strings.SplitN(address, "/", 2)[0]
	}
	return strings.Trim(address, "[]")
}

func isPrivateReportedIP(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && ip.IsPrivate()
}

func isPublicReportedIP(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}

func shouldIgnorePrimaryInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return true
	}

	ignoredPrefixes := []string{
		"lo",
		"docker",
		"br-",
		"veth",
		"cni",
		"flannel",
		"cali",
		"tunl",
		"virbr",
		"podman",
		"kube-ipvs",
		"cbr",
	}
	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func summarizeReportedDisks(disks []collector.DiskInfo) []collector.DiskInfo {
	summary := make([]collector.DiskInfo, 0, len(disks))
	seen := make(map[string]struct{}, len(disks))

	for _, disk := range disks {
		if shouldIgnoreSummaryDisk(disk) {
			continue
		}
		key := strings.TrimSpace(disk.Device) + "|" + strings.TrimSpace(disk.MountPoint)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		summary = append(summary, disk)
	}

	if len(summary) > 0 {
		return summary
	}

	for _, disk := range disks {
		key := strings.TrimSpace(disk.Device) + "|" + strings.TrimSpace(disk.MountPoint)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		summary = append(summary, disk)
	}

	return summary
}

func shouldIgnoreSummaryDisk(disk collector.DiskInfo) bool {
	if disk.Total == 0 {
		return true
	}

	fsType := strings.ToLower(strings.TrimSpace(disk.Fstype))
	if fsType == "" {
		return false
	}

	ignoredFSTypes := map[string]struct{}{
		"autofs":      {},
		"binfmt_misc": {},
		"bpf":         {},
		"cgroup":      {},
		"cgroup2":     {},
		"configfs":    {},
		"debugfs":     {},
		"devpts":      {},
		"devtmpfs":    {},
		"fusectl":     {},
		"hugetlbfs":   {},
		"mqueue":      {},
		"nsfs":        {},
		"overlay":     {},
		"proc":        {},
		"pstore":      {},
		"ramfs":       {},
		"securityfs":  {},
		"squashfs":    {},
		"sysfs":       {},
		"tmpfs":       {},
		"tracefs":     {},
	}
	_, ignored := ignoredFSTypes[fsType]
	return ignored
}

func firstNonEmpty(items []string) string {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func windowsAgentTaskName(hostID uint) string {
	return fmt.Sprintf("OpsHubAgent-Host-%d", hostID)
}

func defaultAgentInstallPath(host *Host, cfg conf.AgentConfig) string {
	if host != nil && host.OSType == OSTypeWindows {
		return windowsAgentHome
	}
	return cfg.GetDefaultInstallPath()
}

func defaultAgentServiceName(host *Host, cfg conf.AgentConfig) string {
	if host != nil && host.OSType == OSTypeWindows {
		return windowsAgentTaskName(host.ID)
	}
	return fmt.Sprintf("%s-%d", cfg.GetServicePrefix(), host.ID)
}

func shouldSyncPrometheusTarget(host *Host, version string) bool {
	if host != nil && host.OSType == OSTypeWindows && strings.HasPrefix(strings.TrimSpace(version), "powershell-agent/") {
		return false
	}
	return true
}

func (uc *AgentUseCase) getOrCreateAgentModel(ctx context.Context, host *Host) (*AssetAgent, bool, error) {
	agentModel, err := uc.agentRepo.GetByHostID(ctx, host.ID)
	if err == nil {
		return agentModel, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	now := time.Now()
	return &AssetAgent{
		HostID:      host.ID,
		ListenPort:  uc.agentCfg.GetDefaultListenPort(),
		InstallPath: defaultAgentInstallPath(host, uc.agentCfg),
		ServiceName: defaultAgentServiceName(host, uc.agentCfg),
		Status:      AgentStatusPending,
		DeployedAt:  &now,
	}, true, nil
}

func (uc *AgentUseCase) saveAgentModel(ctx context.Context, agentModel *AssetAgent, created bool) error {
	if created || agentModel.ID == 0 {
		return uc.agentRepo.Create(ctx, agentModel)
	}
	return uc.agentRepo.Update(ctx, agentModel)
}

func (uc *AgentUseCase) resetHostAgentState(host *Host) {
	if host == nil {
		return
	}

	previousAgentPort := host.AgentPort
	host.AgentID = ""
	host.AgentVersion = ""
	host.AgentLastHeartbeatAt = nil
	host.AgentLastReportAt = nil
	host.AgentLastError = ""
	host.AgentPort = 0

	managementMode := ManagementModeNone
	if host.OSType == OSTypeLinux {
		if strings.TrimSpace(host.SSHUser) != "" && (host.CredentialID > 0 || host.ManagementCredentialID > 0) {
			managementMode = ManagementModeSSH
		}
	} else {
		managementMode = NormalizeManagementMode(host.OSType, host.ManagementMode, host.CredentialID, host.ManagementCredentialID)
	}
	host.ManagementMode = managementMode
	host.ManagementCredentialID = NormalizeManagementCredentialID(managementMode, host.ManagementCredentialID, host.CredentialID)
	host.ManagementPort = NormalizeManagementPort(managementMode, host.ManagementPort, host.Port)
	if host.OSType == OSTypeWindows && managementMode == ManagementModeAgent {
		if host.ManagementPort <= 0 || host.ManagementPort == previousAgentPort || host.ManagementPort == uc.agentCfg.GetDefaultListenPort() {
			host.ManagementPort = windowsWinRMPort
		}
	}
	host.CollectStatus = DefaultCollectStatus(managementMode)
	if managementMode == ManagementModeAgent {
		host.Status = -1
		host.CollectError = "Agent 已卸载"
	} else {
		if managementMode == ManagementModeNone {
			host.Status = -1
		}
		host.CollectError = ""
	}
}
