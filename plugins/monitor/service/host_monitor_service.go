package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	assetbiz "github.com/ydcloud-dy/opshub/internal/biz/asset"
	"github.com/ydcloud-dy/opshub/internal/conf"
	"github.com/ydcloud-dy/opshub/plugins/monitor/model"
	"gorm.io/gorm"
)

const (
	hostHistoryRange1H  = "1h"
	hostHistoryRange24H = "24h"
	hostHistoryRange7D  = "7d"
	hostHistoryRange15D = "15d"
)

type HostMonitorStats struct {
	TotalHosts    int64 `json:"totalHosts"`
	HealthyHosts  int64 `json:"healthyHosts"`
	WarningHosts  int64 `json:"warningHosts"`
	CriticalHosts int64 `json:"criticalHosts"`
	OfflineHosts  int64 `json:"offlineHosts"`
}

type HostMonitorSummary struct {
	ID                 uint    `json:"id"`
	Name               string  `json:"name"`
	IP                 string  `json:"ip"`
	OSType             string  `json:"osType"`
	PrimaryPrivateIP   string  `json:"primaryPrivateIp"`
	PrimaryPublicIP    string  `json:"primaryPublicIp"`
	AgentID            string  `json:"agentId"`
	AgentVersion       string  `json:"agentVersion"`
	AgentPort          int     `json:"agentPort"`
	CollectStatus      string  `json:"collectStatus"`
	HealthStatus       string  `json:"healthStatus"`
	HealthStatusText   string  `json:"healthStatusText"`
	CPUCores           int     `json:"cpuCores"`
	CPUUsage           float64 `json:"cpuUsage"`
	MemoryTotal        uint64  `json:"memoryTotal"`
	MemoryUsed         uint64  `json:"memoryUsed"`
	MemoryUsage        float64 `json:"memoryUsage"`
	DiskTotal          uint64  `json:"diskTotal"`
	DiskUsed           uint64  `json:"diskUsed"`
	DiskUsage          float64 `json:"diskUsage"`
	OS                 string  `json:"os"`
	Kernel             string  `json:"kernel"`
	Arch               string  `json:"arch"`
	Uptime             string  `json:"uptime"`
	Hostname           string  `json:"hostname"`
	LastSeen           string  `json:"lastSeen,omitempty"`
	LastReportAt       string  `json:"lastReportAt,omitempty"`
	LastHeartbeatAt    string  `json:"lastHeartbeatAt,omitempty"`
	AgentLastError     string  `json:"agentLastError,omitempty"`
	TriggeredRuleCount int     `json:"triggeredRuleCount"`
}

type HostMonitorListResult struct {
	List     []*HostMonitorSummary `json:"list"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Stats    HostMonitorStats      `json:"stats"`
}

type HostMonitorOverview struct {
	Host       *HostMonitorSummary       `json:"host"`
	Inventory  *assetbiz.HostInventoryVO `json:"inventory"`
	AlertRules []model.HostAlertRule     `json:"alertRules"`
}

type HostMetricHistory struct {
	HostID      uint               `json:"hostId"`
	Range       string             `json:"range"`
	StepSeconds int64              `json:"stepSeconds"`
	Start       string             `json:"start"`
	End         string             `json:"end"`
	Points      []*HostMetricPoint `json:"points"`
}

type HostMetricPoint struct {
	Timestamp       int64    `json:"timestamp"`
	Time            string   `json:"time"`
	CPUUsage        *float64 `json:"cpuUsage"`
	MemoryUsage     *float64 `json:"memoryUsage"`
	DiskUsage       *float64 `json:"diskUsage"`
	Load1           *float64 `json:"load1"`
	NetworkRecvRate *float64 `json:"networkRecvRate"`
	NetworkSendRate *float64 `json:"networkSendRate"`
	DiskReadRate    *float64 `json:"diskReadRate"`
	DiskWriteRate   *float64 `json:"diskWriteRate"`
}

type hostHistoryWindow struct {
	Key      string
	Duration time.Duration
	Step     time.Duration
}

type HostMonitorService struct {
	db      *gorm.DB
	promCfg conf.PrometheusConfig
}

func NewHostMonitorService(db *gorm.DB) *HostMonitorService {
	var promCfg conf.PrometheusConfig
	if cfg := conf.Get(); cfg != nil {
		promCfg = cfg.Monitoring.Prometheus
	}
	return &HostMonitorService{
		db:      db,
		promCfg: promCfg,
	}
}

func (s *HostMonitorService) ListHosts(ctx context.Context, keyword, health, osType string, page, pageSize int) (*HostMonitorListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var hosts []assetbiz.Host
	if err := applyAgentManagedHostScope(s.db.WithContext(ctx), osType).
		Order("updated_at DESC").
		Find(&hosts).Error; err != nil {
		return nil, err
	}

	rulesByHost, err := s.countTriggeredRules(ctx, hosts)
	if err != nil {
		return nil, err
	}

	items := make([]*HostMonitorSummary, 0, len(hosts))
	stats := HostMonitorStats{}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	health = strings.TrimSpace(health)

	for i := range hosts {
		item := s.toSummary(&hosts[i], rulesByHost[hosts[i].ID])
		stats.TotalHosts++
		switch item.HealthStatus {
		case "healthy":
			stats.HealthyHosts++
		case "warning":
			stats.WarningHosts++
		case "critical":
			stats.CriticalHosts++
		case "offline":
			stats.OfflineHosts++
		}

		if keyword != "" {
			match := strings.Contains(strings.ToLower(item.Name), keyword) ||
				strings.Contains(strings.ToLower(item.IP), keyword) ||
				strings.Contains(strings.ToLower(item.PrimaryPrivateIP), keyword) ||
				strings.Contains(strings.ToLower(item.PrimaryPublicIP), keyword)
			if !match {
				continue
			}
		}
		if health != "" && item.HealthStatus != health {
			continue
		}
		items = append(items, item)
	}

	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	return &HostMonitorListResult{
		List:     items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Stats:    stats,
	}, nil
}

func (s *HostMonitorService) GetOverview(ctx context.Context, hostID uint) (*HostMonitorOverview, error) {
	host, err := s.getHost(ctx, hostID)
	if err != nil {
		return nil, err
	}

	inventory, err := s.getInventory(ctx, hostID)
	if err != nil {
		return nil, err
	}

	var rules []model.HostAlertRule
	if err := s.db.WithContext(ctx).
		Where("host_id IS NULL OR host_id = ?", hostID).
		Order("enabled DESC, updated_at DESC").
		Find(&rules).Error; err != nil {
		return nil, err
	}

	triggered, err := s.countTriggeredRules(ctx, []assetbiz.Host{*host})
	if err != nil {
		return nil, err
	}

	return &HostMonitorOverview{
		Host:       s.toSummary(host, triggered[host.ID]),
		Inventory:  inventory,
		AlertRules: rules,
	}, nil
}

func (s *HostMonitorService) GetProcesses(ctx context.Context, hostID uint) ([]assetbiz.AgentProcessInfo, error) {
	inventory, err := s.getInventoryModel(ctx, hostID)
	if err != nil {
		return nil, err
	}

	items := make([]assetbiz.AgentProcessInfo, 0)
	if inventory == nil {
		return items, nil
	}
	decodeJSONString(inventory.TopProcessesJSON, &items)
	return items, nil
}

func (s *HostMonitorService) GetPorts(ctx context.Context, hostID uint) ([]assetbiz.AgentPortInfo, error) {
	inventory, err := s.getInventoryModel(ctx, hostID)
	if err != nil {
		return nil, err
	}

	items := make([]assetbiz.AgentPortInfo, 0)
	if inventory == nil {
		return items, nil
	}
	decodeJSONString(inventory.ListeningPortsJSON, &items)
	return items, nil
}

func (s *HostMonitorService) GetHistory(ctx context.Context, hostID uint, rangeKey string) (*HostMetricHistory, error) {
	host, err := s.getHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	diskIODeviceSet := s.resolveMonitorDiskIODeviceSet(ctx, hostID)

	window := resolveHostHistoryWindow(rangeKey)
	end := time.Now()
	start := end.Add(-window.Duration)
	result := &HostMetricHistory{
		HostID:      hostID,
		Range:       window.Key,
		StepSeconds: int64(window.Step / time.Second),
		Start:       start.Format("2006-01-02 15:04:05"),
		End:         end.Format("2006-01-02 15:04:05"),
		Points:      make([]*HostMetricPoint, 0),
	}

	if strings.TrimSpace(host.AgentID) == "" && host.AgentLastReportAt == nil {
		return result, nil
	}

	hostLabel := strconv.FormatUint(uint64(hostID), 10)
	cpuSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_cpu_usage_percent{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	memorySeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_memory_usage_percent{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskUsedSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_used_bytes{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskTotalSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_total_bytes{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	loadSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_load1{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	netRecvSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_network_receive_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	netSendSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_network_transmit_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskReadSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_read_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskWriteSeries, err := s.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_write_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskReadSeries = filterMonitorDiskIOSeries(diskReadSeries, diskIODeviceSet)
	diskWriteSeries = filterMonitorDiskIOSeries(diskWriteSeries, diskIODeviceSet)

	cpuValues := flattenPrometheusSeries(cpuSeries)
	memoryValues := flattenPrometheusSeries(memorySeries)
	diskValues := aggregateDiskUsageSeries(diskUsedSeries, diskTotalSeries, true)
	if len(diskValues) == 0 {
		diskValues = aggregateDiskUsageSeries(diskUsedSeries, diskTotalSeries, false)
	}
	loadValues := flattenPrometheusSeries(loadSeries)
	netRecvRate := counterRateSeries(sumPrometheusSeries(netRecvSeries))
	netSendRate := counterRateSeries(sumPrometheusSeries(netSendSeries))
	diskReadRate := counterRateSeries(sumPrometheusSeries(diskReadSeries))
	diskWriteRate := counterRateSeries(sumPrometheusSeries(diskWriteSeries))

	for ts := start.Unix(); ts <= end.Unix(); ts += int64(window.Step / time.Second) {
		point := &HostMetricPoint{
			Timestamp: ts,
			Time:      time.Unix(ts, 0).Format("2006-01-02 15:04"),
		}
		if value, ok := cpuValues[ts]; ok {
			v := roundMetricValue(value)
			point.CPUUsage = &v
		}
		if value, ok := memoryValues[ts]; ok {
			v := roundMetricValue(value)
			point.MemoryUsage = &v
		}
		if value, ok := diskValues[ts]; ok {
			v := roundMetricValue(value)
			point.DiskUsage = &v
		}
		if value, ok := loadValues[ts]; ok {
			v := roundMetricValue(value)
			point.Load1 = &v
		}
		if value, ok := netRecvRate[ts]; ok {
			v := roundMetricValue(value)
			point.NetworkRecvRate = &v
		}
		if value, ok := netSendRate[ts]; ok {
			v := roundMetricValue(value)
			point.NetworkSendRate = &v
		}
		if value, ok := diskReadRate[ts]; ok {
			v := roundMetricValue(value)
			point.DiskReadRate = &v
		}
		if value, ok := diskWriteRate[ts]; ok {
			v := roundMetricValue(value)
			point.DiskWriteRate = &v
		}
		result.Points = append(result.Points, point)
	}

	return result, nil
}
func (s *HostMonitorService) getHost(ctx context.Context, hostID uint) (*assetbiz.Host, error) {
	var host assetbiz.Host
	if err := s.db.WithContext(ctx).First(&host, hostID).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *HostMonitorService) getInventory(ctx context.Context, hostID uint) (*assetbiz.HostInventoryVO, error) {
	host, err := s.getHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	item, err := s.getInventoryModel(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		vo := &assetbiz.HostInventoryVO{HostID: hostID}
		applyMonitorInventoryIPFallback(vo, host)
		vo.PublicIPHistory = s.getPublicIPHistory(ctx, hostID, host)
		return vo, nil
	}

	vo := &assetbiz.HostInventoryVO{
		HostID: hostID,
	}
	decodeJSONString(item.PrivateIPsJSON, &vo.PrivateIPs)
	decodeJSONString(item.PublicIPsJSON, &vo.PublicIPs)
	decodeJSONString(item.InterfacesJSON, &vo.Interfaces)
	decodeJSONString(item.DisksJSON, &vo.Disks)
	decodeJSONString(item.TopProcessesJSON, &vo.TopProcesses)
	decodeJSONString(item.ListeningPortsJSON, &vo.ListeningPorts)
	decodeJSONString(item.ConfigSummaryJSON, &vo.ConfigSummary)
	if item.CollectedAt != nil {
		vo.CollectedAt = item.CollectedAt.Format("2006-01-02 15:04:05")
	}
	applyMonitorInventoryIPFallback(vo, host)
	vo.PublicIPHistory = s.getPublicIPHistory(ctx, hostID, host)
	return vo, nil
}

func (s *HostMonitorService) getInventoryModel(ctx context.Context, hostID uint) (*assetbiz.AssetHostInventory, error) {
	var item assetbiz.AssetHostInventory
	if err := s.db.WithContext(ctx).Where("host_id = ?", hostID).First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *HostMonitorService) getPublicIPHistory(ctx context.Context, hostID uint, host *assetbiz.Host) []assetbiz.HostPublicIPHistoryVO {
	var items []assetbiz.AssetHostPublicIPHistory
	if err := s.db.WithContext(ctx).
		Where("host_id = ?", hostID).
		Order("is_current DESC, last_seen_at DESC, id DESC").
		Limit(10).
		Find(&items).Error; err != nil {
		return buildMonitorPublicIPHistoryFallback(host)
	}
	if len(items) == 0 {
		return buildMonitorPublicIPHistoryFallback(host)
	}

	result := make([]assetbiz.HostPublicIPHistoryVO, 0, len(items))
	for _, item := range items {
		result = append(result, assetbiz.HostPublicIPHistoryVO{
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

func applyMonitorInventoryIPFallback(vo *assetbiz.HostInventoryVO, host *assetbiz.Host) {
	if vo == nil || host == nil {
		return
	}
	vo.PrivateIPs = prependMonitorInventoryIP(vo.PrivateIPs, host.PrimaryPrivateIP)
	vo.PublicIPs = prependMonitorInventoryIP(vo.PublicIPs, host.PrimaryPublicIP)
}

func prependMonitorInventoryIP(items []string, preferred string) []string {
	preferred = strings.TrimSpace(preferred)
	result := make([]string, 0, len(items)+1)
	seen := make(map[string]struct{}, len(items)+1)
	if preferred != "" {
		seen[preferred] = struct{}{}
		result = append(result, preferred)
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func buildMonitorPublicIPHistoryFallback(host *assetbiz.Host) []assetbiz.HostPublicIPHistoryVO {
	if host == nil || strings.TrimSpace(host.PrimaryPublicIP) == "" {
		return []assetbiz.HostPublicIPHistoryVO{}
	}

	observedAt := host.UpdatedAt
	for _, item := range []*time.Time{host.AgentLastReportAt, host.LastCollectAt, host.LastSeen} {
		if item != nil {
			observedAt = *item
			break
		}
	}
	timeText := observedAt.Format("2006-01-02 15:04:05")
	return []assetbiz.HostPublicIPHistoryVO{{
		IP:          strings.TrimSpace(host.PrimaryPublicIP),
		Source:      "host_snapshot",
		FirstSeenAt: timeText,
		LastSeenAt:  timeText,
		SeenCount:   1,
		IsCurrent:   true,
	}}
}

func (s *HostMonitorService) toSummary(host *assetbiz.Host, triggeredRuleCount int) *HostMonitorSummary {
	healthStatus, healthText := evaluateHostHealth(host)
	return &HostMonitorSummary{
		ID:                 host.ID,
		Name:               host.Name,
		IP:                 host.IP,
		OSType:             host.OSType,
		PrimaryPrivateIP:   host.PrimaryPrivateIP,
		PrimaryPublicIP:    host.PrimaryPublicIP,
		AgentID:            host.AgentID,
		AgentVersion:       host.AgentVersion,
		AgentPort:          host.AgentPort,
		CollectStatus:      host.CollectStatus,
		HealthStatus:       healthStatus,
		HealthStatusText:   healthText,
		CPUCores:           host.CPUCores,
		CPUUsage:           host.CPUUsage,
		MemoryTotal:        host.MemoryTotal,
		MemoryUsed:         host.MemoryUsed,
		MemoryUsage:        host.MemoryUsage,
		DiskTotal:          host.DiskTotal,
		DiskUsed:           host.DiskUsed,
		DiskUsage:          host.DiskUsage,
		OS:                 host.OS,
		Kernel:             host.Kernel,
		Arch:               host.Arch,
		Uptime:             host.Uptime,
		Hostname:           host.Hostname,
		LastSeen:           formatNullableTime(host.LastSeen),
		LastReportAt:       formatNullableTime(host.AgentLastReportAt),
		LastHeartbeatAt:    formatNullableTime(host.AgentLastHeartbeatAt),
		AgentLastError:     host.AgentLastError,
		TriggeredRuleCount: triggeredRuleCount,
	}
}

func (s *HostMonitorService) countTriggeredRules(ctx context.Context, hosts []assetbiz.Host) (map[uint]int, error) {
	result := make(map[uint]int, len(hosts))
	if len(hosts) == 0 {
		return result, nil
	}

	var rules []model.HostAlertRule
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return result, nil
	}

	for _, host := range hosts {
		count := 0
		for _, rule := range rules {
			if rule.HostID != nil && *rule.HostID > 0 && *rule.HostID != host.ID {
				continue
			}
			if evaluateHostAlertRule(&host, &rule) {
				count++
			}
		}
		result[host.ID] = count
	}
	return result, nil
}

func resolveHostHistoryWindow(rangeKey string) hostHistoryWindow {
	switch strings.TrimSpace(rangeKey) {
	case hostHistoryRange24H:
		return hostHistoryWindow{
			Key:      hostHistoryRange24H,
			Duration: 24 * time.Hour,
			Step:     10 * time.Minute,
		}
	case hostHistoryRange7D:
		return hostHistoryWindow{
			Key:      hostHistoryRange7D,
			Duration: 7 * 24 * time.Hour,
			Step:     time.Hour,
		}
	case hostHistoryRange15D:
		return hostHistoryWindow{
			Key:      hostHistoryRange15D,
			Duration: 15 * 24 * time.Hour,
			Step:     2 * time.Hour,
		}
	default:
		return hostHistoryWindow{
			Key:      hostHistoryRange1H,
			Duration: time.Hour,
			Step:     time.Minute,
		}
	}
}

type prometheusQueryRangeResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string                   `json:"resultType"`
		Result     []prometheusMatrixSeries `json:"result"`
	} `json:"data"`
}

type prometheusMatrixSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

type prometheusSample struct {
	Timestamp int64
	Value     float64
}

func (s *HostMonitorService) queryPrometheusRange(ctx context.Context, expr string, start, end time.Time, step time.Duration) ([]prometheusMatrixSeries, error) {
	baseURL := s.promCfg.GetBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		return nil, nil
	}

	values := url.Values{}
	values.Set("query", expr)
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", strconv.FormatInt(int64(step/time.Second), 10))

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(s.promCfg.GetQueryTimeoutSeconds())*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(queryCtx, http.MethodGet, baseURL+"/api/v1/query_range?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Prometheus 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prometheus 返回异常状态 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result prometheusQueryRangeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 Prometheus 响应失败: %w", err)
	}
	if result.Status != "success" {
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = "unknown error"
		}
		return nil, fmt.Errorf("Prometheus 查询失败: %s", errText)
	}

	return result.Data.Result, nil
}

func flattenPrometheusSeries(series []prometheusMatrixSeries) map[int64]float64 {
	grouped := make(map[int64][]float64)
	for _, item := range series {
		for _, sample := range parsePrometheusSamples(item.Values) {
			grouped[sample.Timestamp] = append(grouped[sample.Timestamp], sample.Value)
		}
	}

	values := make(map[int64]float64, len(grouped))
	for ts, samples := range grouped {
		if len(samples) == 0 {
			continue
		}
		var sum float64
		for _, sample := range samples {
			sum += sample
		}
		values[ts] = sum / float64(len(samples))
	}
	return values
}

func sumPrometheusSeries(series []prometheusMatrixSeries) map[int64]float64 {
	values := make(map[int64]float64)
	for _, item := range series {
		for _, sample := range parsePrometheusSamples(item.Values) {
			values[sample.Timestamp] += sample.Value
		}
	}
	return values
}

func counterRateSeries(values map[int64]float64) map[int64]float64 {
	if len(values) == 0 {
		return map[int64]float64{}
	}
	keys := make([]int64, 0, len(values))
	for ts := range values {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	rates := make(map[int64]float64, len(keys))
	for i := 1; i < len(keys); i++ {
		prevTS := keys[i-1]
		currTS := keys[i]
		deltaTime := currTS - prevTS
		if deltaTime <= 0 {
			continue
		}
		deltaValue := values[currTS] - values[prevTS]
		if deltaValue < 0 {
			deltaValue = 0
		}
		rates[currTS] = deltaValue / float64(deltaTime)
	}
	return rates
}

func aggregateDiskUsageSeries(usedSeries, totalSeries []prometheusMatrixSeries, filtered bool) map[int64]float64 {
	usedValues := make(map[int64]float64)
	totalValues := make(map[int64]float64)

	for _, item := range usedSeries {
		if filtered && shouldIgnoreTrendDiskMetric(item.Metric) {
			continue
		}
		for _, sample := range parsePrometheusSamples(item.Values) {
			usedValues[sample.Timestamp] += sample.Value
		}
	}

	for _, item := range totalSeries {
		if filtered && shouldIgnoreTrendDiskMetric(item.Metric) {
			continue
		}
		for _, sample := range parsePrometheusSamples(item.Values) {
			totalValues[sample.Timestamp] += sample.Value
		}
	}

	values := make(map[int64]float64, len(totalValues))
	for ts, total := range totalValues {
		if total <= 0 {
			continue
		}
		values[ts] = usedValues[ts] / total * 100
	}
	return values
}

func shouldIgnoreTrendDiskMetric(metric map[string]string) bool {
	device := strings.ToLower(strings.TrimSpace(metric["device"]))
	mountPoint := strings.TrimSpace(metric["mount_point"])
	if mountPoint == "" {
		return false
	}

	ignoredPrefixes := []string{"/run", "/var/lib/docker", "/snap", "/proc", "/sys"}
	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(mountPoint, prefix) {
			return true
		}
	}

	return strings.HasPrefix(device, "overlay") ||
		strings.HasPrefix(device, "tmpfs") ||
		strings.HasPrefix(device, "shm") ||
		strings.HasPrefix(device, "loop")
}

func parsePrometheusSamples(values [][]interface{}) []prometheusSample {
	samples := make([]prometheusSample, 0, len(values))
	for _, item := range values {
		if len(item) != 2 {
			continue
		}
		timestamp, ok := asFloat64(item[0])
		if !ok {
			continue
		}
		value, ok := asString(item[1])
		if !ok {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			continue
		}
		samples = append(samples, prometheusSample{
			Timestamp: int64(timestamp),
			Value:     parsed,
		})
	}
	return samples
}

func roundMetricValue(value float64) float64 {
	return math.Round(value*100) / 100
}

func asFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func asString(value interface{}) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

func decodeJSONString(raw string, target interface{}) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	_ = json.Unmarshal([]byte(raw), target)
}

func formatNullableTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func evaluateHostHealth(host *assetbiz.Host) (string, string) {
	if host == nil {
		return "unknown", "未知"
	}
	if strings.TrimSpace(host.AgentID) == "" || host.AgentLastHeartbeatAt == nil {
		return "offline", "离线"
	}
	if !assetbiz.IsAgentHeartbeatFresh(host) {
		return "offline", "离线"
	}

	maxUsage := math.Max(host.CPUUsage, math.Max(host.MemoryUsage, host.DiskUsage))
	switch {
	case maxUsage >= 90:
		return "critical", "严重"
	case maxUsage >= 75:
		return "warning", "告警"
	default:
		return "healthy", "健康"
	}
}

func evaluateHostAlertRule(host *assetbiz.Host, rule *model.HostAlertRule) bool {
	if host == nil || rule == nil || !rule.Enabled {
		return false
	}
	if strings.TrimSpace(host.AgentID) == "" && rule.Metric != "agent_offline" {
		return false
	}

	switch rule.Metric {
	case "cpu_usage":
		return host.CPUUsage >= rule.Threshold
	case "memory_usage":
		return host.MemoryUsage >= rule.Threshold
	case "disk_usage":
		return host.DiskUsage >= rule.Threshold
	case "agent_offline":
		timeout := time.Duration(rule.Threshold) * time.Second
		if timeout <= 0 {
			timeout = 3 * time.Minute
		}
		if host.AgentLastHeartbeatAt == nil {
			return true
		}
		return time.Since(*host.AgentLastHeartbeatAt) > timeout
	default:
		return false
	}
}
