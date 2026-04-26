package asset

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
)

const (
	hostMetricTrendRange1H  = "1h"
	hostMetricTrendRange24H = "24h"
	hostMetricTrendRange7D  = "7d"
	hostMetricTrendRange15D = "15d"
)

type HostMetricTrendVO struct {
	HostID      uint                    `json:"hostId"`
	Range       string                  `json:"range"`
	StepSeconds int64                   `json:"stepSeconds"`
	Start       string                  `json:"start"`
	End         string                  `json:"end"`
	Points      []*HostMetricTrendPoint `json:"points"`
}

type HostMetricTrendPoint struct {
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

type hostMetricTrendWindow struct {
	Key      string
	Duration time.Duration
	Step     time.Duration
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

func (uc *HostUseCase) GetMetricTrend(ctx context.Context, hostID uint, rangeKey string) (*HostMetricTrendVO, error) {
	host, err := uc.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	window := resolveHostMetricTrendWindow(rangeKey)
	end := time.Now()
	start := end.Add(-window.Duration)
	trend := &HostMetricTrendVO{
		HostID:      hostID,
		Range:       window.Key,
		StepSeconds: int64(window.Step / time.Second),
		Start:       start.Format("2006-01-02 15:04:05"),
		End:         end.Format("2006-01-02 15:04:05"),
		Points:      make([]*HostMetricTrendPoint, 0),
	}

	if !uc.promCfg.Enabled && strings.TrimSpace(uc.promCfg.BaseURL) == "" {
		return trend, nil
	}
	if strings.TrimSpace(host.AgentID) == "" && host.AgentLastReportAt == nil {
		return trend, nil
	}

	hostLabel := strconv.FormatUint(uint64(hostID), 10)
	cpuSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_cpu_usage_percent{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}

	memorySeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_memory_usage_percent{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}

	diskUsedSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_used_bytes{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}

	diskTotalSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_total_bytes{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	loadSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_load1{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	netRecvSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_network_receive_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	netSendSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_network_transmit_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskReadSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_read_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}
	diskWriteSeries, err := uc.queryPrometheusRange(ctx, fmt.Sprintf(`opshub_agent_disk_write_bytes_total{host_id=%q}`, hostLabel), start, end, window.Step)
	if err != nil {
		return nil, err
	}

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
		point := &HostMetricTrendPoint{
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
		trend.Points = append(trend.Points, point)
	}

	return trend, nil
}
func resolveHostMetricTrendWindow(rangeKey string) hostMetricTrendWindow {
	switch strings.TrimSpace(rangeKey) {
	case hostMetricTrendRange24H:
		return hostMetricTrendWindow{
			Key:      hostMetricTrendRange24H,
			Duration: 24 * time.Hour,
			Step:     10 * time.Minute,
		}
	case hostMetricTrendRange7D:
		return hostMetricTrendWindow{
			Key:      hostMetricTrendRange7D,
			Duration: 7 * 24 * time.Hour,
			Step:     time.Hour,
		}
	case hostMetricTrendRange15D:
		return hostMetricTrendWindow{
			Key:      hostMetricTrendRange15D,
			Duration: 15 * 24 * time.Hour,
			Step:     2 * time.Hour,
		}
	default:
		return hostMetricTrendWindow{
			Key:      hostMetricTrendRange1H,
			Duration: time.Hour,
			Step:     time.Minute,
		}
	}
}

func (uc *HostUseCase) queryPrometheusRange(ctx context.Context, expr string, start, end time.Time, step time.Duration) ([]prometheusMatrixSeries, error) {
	baseURL := uc.promCfg.GetBaseURL()
	values := url.Values{}
	values.Set("query", expr)
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", strconv.FormatInt(int64(step/time.Second), 10))

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(uc.promCfg.GetQueryTimeoutSeconds())*time.Second)
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
	for ts, items := range grouped {
		if len(items) == 0 {
			continue
		}
		var sum float64
		for _, item := range items {
			sum += item
		}
		values[ts] = sum / float64(len(items))
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
	timestamps := make([]int64, 0, len(values))
	for ts := range values {
		timestamps = append(timestamps, ts)
	}
	if len(timestamps) == 0 {
		return map[int64]float64{}
	}

	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	rates := make(map[int64]float64, len(timestamps))
	for index := 1; index < len(timestamps); index++ {
		currentTS := timestamps[index]
		previousTS := timestamps[index-1]
		elapsed := currentTS - previousTS
		if elapsed <= 0 {
			continue
		}
		currentValue := values[currentTS]
		previousValue := values[previousTS]
		delta := currentValue - previousValue
		if delta < 0 {
			delta = 0
		}
		rates[currentTS] = delta / float64(elapsed)
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
	if device == "" && mountPoint == "/" {
		return false
	}

	ignoredDevices := []string{
		"tmpfs",
		"overlay",
		"proc",
		"sysfs",
		"cgroup",
		"cgroup2",
		"devtmpfs",
		"devpts",
		"autofs",
		"mqueue",
		"tracefs",
		"nsfs",
		"ramfs",
		"fusectl",
		"pstore",
		"securityfs",
		"debugfs",
		"configfs",
		"hugetlbfs",
	}
	for _, item := range ignoredDevices {
		if device == item {
			return true
		}
	}

	if strings.Contains(device, "loop") || strings.Contains(device, "overlay") {
		return true
	}

	ignoredMountPrefixes := []string{
		"/dev",
		"/proc",
		"/run",
		"/snap",
		"/sys",
		"/var/lib/docker",
		"/var/lib/containerd",
		"/var/lib/kubelet",
	}
	for _, prefix := range ignoredMountPrefixes {
		if mountPoint == prefix || strings.HasPrefix(mountPoint, prefix+"/") {
			return true
		}
	}

	return false
}

func parsePrometheusSamples(values [][]interface{}) []prometheusSample {
	samples := make([]prometheusSample, 0, len(values))
	for _, item := range values {
		if len(item) < 2 {
			continue
		}

		ts, ok := convertPrometheusFloat(item[0])
		if !ok {
			continue
		}
		value, ok := convertPrometheusFloat(item[1])
		if !ok {
			continue
		}

		samples = append(samples, prometheusSample{
			Timestamp: int64(ts),
			Value:     value,
		})
	}
	return samples
}

func convertPrometheusFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func roundMetricValue(value float64) float64 {
	return math.Round(value*100) / 100
}
