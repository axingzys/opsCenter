package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ydcloud-dy/opshub/pkg/collector"
)

const (
	agentSnapshotPath     = "/api/v1/snapshot"
	maxAgentSnapshotBytes = 16 << 20
)

func (uc *HostUseCase) collectAgentSnapshot(ctx context.Context, host *Host) (*collector.SystemInfo, error) {
	endpoint, token, err := uc.resolveAgentEndpoint(ctx, host)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+agentSnapshotPath, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 实时采集请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := newAgentFileHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 Agent 实时采集接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Agent 版本过低，不支持实时采集，请重新部署 Agent")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Agent 访问令牌无效，请重新部署 Agent")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Agent 实时采集接口返回 %d: %s", resp.StatusCode, readAgentFileError(resp.Body))
	}

	var snapshot AgentReportRequest
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAgentSnapshotBytes)).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("解析 Agent 实时采集结果失败: %w", err)
	}

	return SystemInfoFromAgentReport(&snapshot)
}

func (uc *HostUseCase) resolveAgentEndpoint(ctx context.Context, host *Host) (string, string, error) {
	if uc.agentRepo == nil {
		return "", "", fmt.Errorf("Agent 仓储未初始化")
	}
	if host == nil {
		return "", "", fmt.Errorf("主机不存在")
	}

	agentModel, err := uc.agentRepo.GetByHostID(ctx, host.ID)
	if err != nil {
		return "", "", fmt.Errorf("获取 Agent 记录失败: %w", err)
	}

	token, err := decryptAgentSecret(uc.agentSecretKey, agentModel.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("解密 Agent 访问令牌失败: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return "", "", fmt.Errorf("当前 Agent 缺少访问令牌，请重新部署 Agent")
	}

	targetHost := firstNonEmpty([]string{
		normalizeReportedIP(host.PrimaryPrivateIP),
		normalizeReportedIP(host.IP),
		normalizeReportedIP(host.PrimaryPublicIP),
	})
	if targetHost == "" {
		return "", "", fmt.Errorf("当前主机缺少可访问的 Agent 地址")
	}

	port := firstPositive(agentModel.ListenPort, host.AgentPort, 19100)
	if port <= 0 {
		return "", "", fmt.Errorf("当前主机缺少可访问的 Agent 端口")
	}

	return fmt.Sprintf("http://%s", net.JoinHostPort(targetHost, strconv.Itoa(port))), strings.TrimSpace(token), nil
}

func SystemInfoFromAgentReport(req *AgentReportRequest) (*collector.SystemInfo, error) {
	if req == nil {
		return nil, errors.New("Agent 实时采集结果为空")
	}
	if strings.TrimSpace(req.OS) == "" && req.CPU.Threads == 0 && req.Memory.Total == 0 && len(req.Disk) == 0 {
		return nil, errors.New("Agent 实时采集结果为空")
	}

	return &collector.SystemInfo{
		OS:       strings.TrimSpace(req.OS),
		Kernel:   strings.TrimSpace(req.Kernel),
		Arch:     strings.TrimSpace(req.Arch),
		CPU:      req.CPU,
		Memory:   req.Memory,
		Disk:     summarizeReportedDisks(req.Disk),
		Uptime:   strings.TrimSpace(req.Uptime),
		Hostname: strings.TrimSpace(req.Hostname),
	}, nil
}
