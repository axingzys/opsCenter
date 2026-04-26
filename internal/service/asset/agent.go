package asset

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	rbacbiz "github.com/ydcloud-dy/opshub/internal/biz/rbac"
	"github.com/ydcloud-dy/opshub/internal/conf"
	rbacService "github.com/ydcloud-dy/opshub/internal/service/rbac"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

type AgentService struct {
	useCase                *assetbiz.AgentUseCase
	assetPermissionUseCase *rbacbiz.AssetPermissionUseCase
	cfg                    *conf.Config
}

func NewAgentService(useCase *assetbiz.AgentUseCase, assetPermissionUseCase *rbacbiz.AssetPermissionUseCase, cfg *conf.Config) *AgentService {
	return &AgentService{
		useCase:                useCase,
		assetPermissionUseCase: assetPermissionUseCase,
		cfg:                    cfg,
	}
}

func (s *AgentService) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	accessibleHostIDs := s.getAccessibleHostIDs(c)
	data, total, err := s.useCase.List(c.Request.Context(), page, pageSize, keyword, status, accessibleHostIDs)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "查询 Agent 列表失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"list":     data,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *AgentService) Deploy(c *gin.Context) {
	var req assetbiz.AgentDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	filteredHostIDs := s.filterHostIDs(c, req.HostIDs)
	if len(filteredHostIDs) == 0 {
		response.ErrorCode(c, http.StatusForbidden, "没有可部署的主机")
		return
	}

	operatorID := rbacService.GetUserID(c)
	data, err := s.useCase.Deploy(c.Request.Context(), filteredHostIDs, operatorID, s.resolveBaseURL(c))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "部署 Agent 失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "部署任务已执行", data)
}

func (s *AgentService) Uninstall(c *gin.Context) {
	var req assetbiz.AgentUninstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	filteredHostIDs := s.filterHostIDs(c, req.HostIDs)
	if len(filteredHostIDs) == 0 {
		response.ErrorCode(c, http.StatusForbidden, "没有可卸载的主机")
		return
	}

	operatorID := rbacService.GetUserID(c)
	data, err := s.useCase.Uninstall(c.Request.Context(), filteredHostIDs, operatorID)
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "卸载 Agent 失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "卸载任务已执行", data)
}

func (s *AgentService) GetJob(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	job, err := s.useCase.GetJob(c.Request.Context(), uint(id64))
	if err != nil {
		response.ErrorCode(c, http.StatusNotFound, "任务不存在")
		return
	}

	if !s.canAccessHost(c, job.HostID) {
		response.ErrorCode(c, http.StatusForbidden, "没有权限查看该任务")
		return
	}

	response.Success(c, job)
}

func (s *AgentService) GetInventory(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的主机ID")
		return
	}

	data, err := s.useCase.GetInventory(c.Request.Context(), uint(id64))
	if err != nil {
		response.ErrorCode(c, http.StatusInternalServerError, "获取库存信息失败: "+err.Error())
		return
	}

	response.Success(c, data)
}

func (s *AgentService) GetHostBootstrap(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "无效的主机ID")
		return
	}

	data, err := s.useCase.GenerateBootstrap(c.Request.Context(), uint(id64), s.resolveBaseURL(c))
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, data)
}

func (s *AgentService) InstallPowerShellScript(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		response.ErrorCode(c, http.StatusBadRequest, "缺少 Agent 注册令牌")
		return
	}

	script, err := s.useCase.RenderPowerShellInstallScript(c.Request.Context(), token, s.resolveBaseURL(c))
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

func (s *AgentService) InstallShellScript(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		response.ErrorCode(c, http.StatusBadRequest, "缺少 Agent 注册令牌")
		return
	}

	script, err := s.useCase.RenderShellInstallScript(c.Request.Context(), token, s.resolveBaseURL(c))
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

func (s *AgentService) DownloadBinary(c *gin.Context) {
	osName := normalizeBinarySegment(c.Param("os"))
	arch := normalizeAgentArch(c.Param("arch"))
	if osName == "" || arch == "" {
		response.ErrorCode(c, http.StatusBadRequest, "无效的平台参数")
		return
	}

	filePath := filepath.Join(s.cfg.Agent.GetBundleDir(), fmt.Sprintf("%s-%s-%s", s.cfg.Agent.GetBinaryName(), osName, arch))
	if _, err := os.Stat(filePath); err != nil {
		response.ErrorCode(c, http.StatusNotFound, "Agent 二进制不存在")
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}

func (s *AgentService) Register(c *gin.Context) {
	var req assetbiz.AgentRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	data, err := s.useCase.Register(c.Request.Context(), s.resolveBaseURL(c), &req)
	if err != nil {
		response.ErrorCode(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, data)
}

func (s *AgentService) Report(c *gin.Context) {
	accessToken := extractBearerToken(c.GetHeader("Authorization"))
	if accessToken == "" {
		response.ErrorCode(c, http.StatusUnauthorized, "缺少 Agent 访问令牌")
		return
	}

	var req assetbiz.AgentReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorCode(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := s.useCase.Report(c.Request.Context(), accessToken, &req); err != nil {
		response.ErrorCode(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.SuccessWithMessage(c, "Agent 上报成功", nil)
}

func (s *AgentService) EchoIP(c *gin.Context) {
	observedIP, source := resolveObservedIP(c)
	if observedIP == "" {
		response.ErrorCode(c, http.StatusServiceUnavailable, "无法识别来源IP")
		return
	}

	response.Success(c, gin.H{
		"observedIp":   observedIP,
		"source":       source,
		"forwardedFor": strings.TrimSpace(c.GetHeader("X-Forwarded-For")),
	})
}

func (s *AgentService) resolveBaseURL(c *gin.Context) string {
	if baseURL := strings.TrimSpace(c.GetHeader("X-OpsHub-Base-URL")); baseURL != "" {
		return sanitizeAgentBaseURL(baseURL, c.Request.Host, s.cfg.Server.HttpPort)
	}
	if externalURL := strings.TrimSpace(s.cfg.Server.ExternalURL); externalURL != "" {
		return sanitizeAgentBaseURL(externalURL, c.Request.Host, s.cfg.Server.HttpPort)
	}

	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}

	if host != "" {
		return sanitizeAgentBaseURL(fmt.Sprintf("%s://%s", scheme, host), c.Request.Host, s.cfg.Server.HttpPort)
	}

	return sanitizeAgentBaseURL(s.cfg.Server.GetOAuth2Issuer(), c.Request.Host, s.cfg.Server.HttpPort)
}

func sanitizeAgentBaseURL(rawBaseURL, fallbackHost string, defaultPort int) string {
	rawBaseURL = strings.TrimSpace(strings.TrimRight(rawBaseURL, "/"))
	if rawBaseURL == "" {
		return rawBaseURL
	}

	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return rawBaseURL
	}
	if parsed.Host == "" {
		return rawBaseURL
	}

	hostName := parsed.Hostname()
	if !isLocalOnlyHost(hostName) {
		return rawBaseURL
	}

	usedInterfaceHost := false
	advertiseHost := sanitizeFallbackHost(fallbackHost)
	if advertiseHost == "" {
		advertiseHost = firstReachableAdvertiseHost()
		if advertiseHost == "" {
			return rawBaseURL
		}
		usedInterfaceHost = true
	}

	port := extractPort(fallbackHost)
	if port == "" && usedInterfaceHost && defaultPort > 0 {
		port = strconv.Itoa(defaultPort)
	}

	if port != "" {
		parsed.Host = net.JoinHostPort(advertiseHost, port)
	} else {
		parsed.Host = advertiseHost
	}

	return strings.TrimRight(parsed.String(), "/")
}

func isLocalOnlyHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch host {
	case "", "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	default:
		return false
	}
}

func sanitizeFallbackHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err == nil {
			host = parsed.Host
		}
	}
	hostname := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		hostname = parsedHost
	}
	hostname = strings.Trim(hostname, "[]")
	if isLocalOnlyHost(hostname) {
		return ""
	}
	return hostname
}

func extractPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err == nil {
			return parsed.Port()
		}
	}
	_, port, err := net.SplitHostPort(host)
	if err != nil {
		return ""
	}
	return port
}

func firstReachableAdvertiseHost() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var publicCandidates []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtualInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addressToIPv4(addr)
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			publicCandidates = append(publicCandidates, ip.String())
		}
	}

	if len(publicCandidates) > 0 {
		return publicCandidates[0]
	}
	return ""
}

func addressToIPv4(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP.To4()
	case *net.IPAddr:
		return value.IP.To4()
	default:
		return nil
	}
}

func isVirtualInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	prefixes := []string{"lo", "docker", "br-", "veth", "virbr", "cni", "flannel", "zt", "tailscale", "wg", "tun", "tap"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func resolveObservedIP(c *gin.Context) (string, string) {
	if ip := firstForwardedIP(c.GetHeader("X-Forwarded-For")); ip != "" {
		return ip, "x-forwarded-for"
	}
	if ip := normalizeObservedIP(c.GetHeader("X-Real-IP")); ip != "" {
		return ip, "x-real-ip"
	}
	if ip := normalizeObservedIP(c.ClientIP()); ip != "" {
		return ip, "client-ip"
	}
	if ip := normalizeObservedIP(c.Request.RemoteAddr); ip != "" {
		return ip, "remote-addr"
	}
	return "", ""
}

func firstForwardedIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, item := range strings.Split(value, ",") {
		if ip := normalizeObservedIP(item); ip != "" {
			return ip
		}
	}
	return ""
}

func normalizeObservedIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	if ip := net.ParseIP(value); ip != nil {
		return ip.String()
	}
	return ""
}

func (s *AgentService) getAccessibleHostIDs(c *gin.Context) []uint {
	userID := rbacService.GetUserID(c)
	if userID == 0 {
		return nil
	}

	hostIDs, err := s.assetPermissionUseCase.GetUserAccessibleHostIDs(c.Request.Context(), userID)
	if err != nil {
		return []uint{}
	}
	return hostIDs
}

func (s *AgentService) filterHostIDs(c *gin.Context, hostIDs []uint) []uint {
	accessibleHostIDs := s.getAccessibleHostIDs(c)
	if accessibleHostIDs == nil {
		return hostIDs
	}
	if len(accessibleHostIDs) == 0 {
		return []uint{}
	}

	allowed := make(map[uint]struct{}, len(accessibleHostIDs))
	for _, hostID := range accessibleHostIDs {
		allowed[hostID] = struct{}{}
	}

	filtered := make([]uint, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		if _, ok := allowed[hostID]; ok {
			filtered = append(filtered, hostID)
		}
	}
	return filtered
}

func (s *AgentService) canAccessHost(c *gin.Context, hostID uint) bool {
	accessibleHostIDs := s.getAccessibleHostIDs(c)
	if accessibleHostIDs == nil {
		return true
	}
	for _, item := range accessibleHostIDs {
		if item == hostID {
			return true
		}
	}
	return false
}

func normalizeBinarySegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "linux", "windows":
		return value
	default:
		return ""
	}
}

func normalizeAgentArch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return ""
	}
}

func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}
