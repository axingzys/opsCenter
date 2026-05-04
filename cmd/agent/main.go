package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

const defaultVersion = "opshub-agent/0.1.1"

const (
	maxProcessMetrics              = 10
	maxPortMetrics                 = 64
	defaultPublicIPEchoFallbackURL = "https://api64.ipify.org?format=text"
)

type agentConfig struct {
	AgentID          string                 `json:"agentId"`
	AccessToken      string                 `json:"accessToken"`
	ReportURL        string                 `json:"reportUrl"`
	IntervalSeconds  int                    `json:"intervalSeconds"`
	ListenAddr       string                 `json:"listenAddr"`
	Version          string                 `json:"version"`
	HostID           uint                   `json:"hostId"`
	ServiceName      string                 `json:"serviceName"`
	DatabaseArchiver databaseArchiverConfig `json:"databaseArchiver"`
}

type reportPayload struct {
	Version        string             `json:"version"`
	Hostname       string             `json:"hostname"`
	OS             string             `json:"os"`
	Kernel         string             `json:"kernel"`
	Arch           string             `json:"arch"`
	Uptime         string             `json:"uptime"`
	ListenPort     int                `json:"listenPort"`
	PrivateIPs     []string           `json:"privateIps"`
	PublicIPs      []string           `json:"publicIps"`
	Interfaces     []networkInterface `json:"interfaces"`
	Load           loadInfo           `json:"load"`
	Network        []networkIOInfo    `json:"network"`
	DiskIO         []diskIOInfo       `json:"diskIo"`
	CPU            cpuInfo            `json:"cpu"`
	Memory         memoryInfo         `json:"memory"`
	Disk           []diskInfo         `json:"disk"`
	TopProcesses   []processInfo      `json:"topProcesses"`
	ListeningPorts []portInfo         `json:"listeningPorts"`
	ConfigSummary  configSummary      `json:"configSummary"`
}

type loadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type cpuInfo struct {
	ModelName string  `json:"modelName"`
	Cores     int     `json:"cores"`
	Threads   int     `json:"threads"`
	Usage     float64 `json:"usage"`
	MHz       float64 `json:"mHz"`
	Cache     string  `json:"cache"`
	VendorID  string  `json:"vendorId"`
}

type memoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	Usage     float64 `json:"usage"`
	SwapTotal uint64  `json:"swapTotal"`
	SwapUsed  uint64  `json:"swapUsed"`
}

type diskInfo struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mountPoint"`
	Fstype     string  `json:"fstype"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	Usage      float64 `json:"usage"`
}

type networkInterface struct {
	Name      string   `json:"name"`
	Hardware  string   `json:"hardware"`
	Addresses []string `json:"addresses"`
}

type networkIOInfo struct {
	Name      string `json:"name"`
	BytesRecv uint64 `json:"bytesRecv"`
	BytesSent uint64 `json:"bytesSent"`
}

type diskIOInfo struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
}

type processInfo struct {
	PID           int     `json:"pid"`
	Name          string  `json:"name"`
	Command       string  `json:"command"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
}

type portInfo struct {
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	ListenAddr  string `json:"listenAddr"`
	ProcessName string `json:"processName"`
	PID         int    `json:"pid"`
}

type configSummary struct {
	OSRelease        string   `json:"osRelease"`
	Kernel           string   `json:"kernel"`
	Arch             string   `json:"arch"`
	Timezone         string   `json:"timezone"`
	ServiceManager   string   `json:"serviceManager"`
	ContainerRuntime string   `json:"containerRuntime"`
	Mounts           []string `json:"mounts"`
	AgentVersion     string   `json:"agentVersion"`
}

type fileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"isDir"`
	ModTime string `json:"modTime"`
}

type responseEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type agentMetrics struct {
	up             prometheus.Gauge
	reportTS       prometheus.Gauge
	cpuUsage       prometheus.Gauge
	load1          prometheus.Gauge
	load5          prometheus.Gauge
	load15         prometheus.Gauge
	memoryTotal    prometheus.Gauge
	memoryUsed     prometheus.Gauge
	memoryUsage    prometheus.Gauge
	diskUsage      *prometheus.GaugeVec
	diskTotal      *prometheus.GaugeVec
	diskUsed       *prometheus.GaugeVec
	networkRecv    *prometheus.GaugeVec
	networkSend    *prometheus.GaugeVec
	diskReadBytes  *prometheus.GaugeVec
	diskWriteBytes *prometheus.GaugeVec
	processCPU     *prometheus.GaugeVec
	processMemory  *prometheus.GaugeVec
	portListening  *prometheus.GaugeVec
	processCount   prometheus.Gauge
	listenPortInfo prometheus.Gauge
}

type agentApp struct {
	cfg                           *agentConfig
	httpClient                    *http.Client
	metrics                       *agentMetrics
	databaseArchiverBackoffMu     sync.Mutex
	databaseArchiverFailures      map[uint]int
	databaseArchiverBackoffUntil  map[uint]time.Time
	databaseArchiverProcessMu     sync.Mutex
	databaseArchiverProcesses     map[uint]*agentStreamingProcess
	databaseArchiverProcessStarts map[uint]int
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "unwrap-response":
			mustHandleUnwrapResponse()
			return
		case "json-field":
			mustHandleJSONField()
			return
		}
	}

	handled, err := handlePlatformCommand(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if handled {
		return
	}

	configPath, err := parseConfigPath(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runAgent(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

func parseConfigPath(args []string) (string, error) {
	fs := newFlagSet("agent")
	var configPath string
	fs.StringVar(&configPath, "config", "", "agent config path")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return "", errors.New("missing --config")
	}
	return configPath, nil
}

func runAgent(ctx context.Context, cfg *agentConfig) error {
	app := &agentApp{
		cfg:                           cfg,
		httpClient:                    &http.Client{Timeout: 20 * time.Second},
		metrics:                       newAgentMetrics(),
		databaseArchiverFailures:      map[uint]int{},
		databaseArchiverBackoffUntil:  map[uint]time.Time{},
		databaseArchiverProcesses:     map[uint]*agentStreamingProcess{},
		databaseArchiverProcessStarts: map[uint]int{},
	}

	metricsErrCh, err := app.serveMetrics(ctx)
	if err != nil {
		return fmt.Errorf("serve metrics: %w", err)
	}
	if archiverCfg, err := resolveDatabaseArchiverConfig(cfg); err != nil {
		return fmt.Errorf("database archiver config: %w", err)
	} else if archiverCfg.Enabled {
		go app.runDatabaseArchiver(ctx, archiverCfg)
	}

	runDone := make(chan struct{})
	go func() {
		app.run(ctx)
		close(runDone)
	}()

	select {
	case err, ok := <-metricsErrCh:
		if ok && err != nil {
			return fmt.Errorf("metrics server stopped: %w", err)
		}
		return nil
	case <-runDone:
		return nil
	case <-ctx.Done():
		return nil
	}
}

func loadConfig(path string) (*agentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var cfg agentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ReportURL) == "" {
		return nil, errors.New("reportUrl is required")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return nil, errors.New("accessToken is required")
	}
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 60
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		cfg.ListenAddr = "0.0.0.0:19100"
	}
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = defaultVersion
	}
	return &cfg, nil
}

func newAgentMetrics() *agentMetrics {
	items := &agentMetrics{
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_up",
			Help: "Whether the OpsHub agent process is running.",
		}),
		reportTS: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_last_report_timestamp_seconds",
			Help: "Unix timestamp of the last successful report.",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_cpu_usage_percent",
			Help: "CPU usage percent.",
		}),
		load1: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_load1",
			Help: "1 minute load average.",
		}),
		load5: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_load5",
			Help: "5 minute load average.",
		}),
		load15: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_load15",
			Help: "15 minute load average.",
		}),
		memoryTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_memory_total_bytes",
			Help: "Total memory bytes.",
		}),
		memoryUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_memory_used_bytes",
			Help: "Used memory bytes.",
		}),
		memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_memory_usage_percent",
			Help: "Memory usage percent.",
		}),
		diskUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_disk_usage_percent",
			Help: "Disk usage percent by mount point.",
		}, []string{"mount_point", "device"}),
		diskTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_disk_total_bytes",
			Help: "Disk total bytes by mount point.",
		}, []string{"mount_point", "device"}),
		diskUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_disk_used_bytes",
			Help: "Disk used bytes by mount point.",
		}, []string{"mount_point", "device"}),
		networkRecv: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_network_receive_bytes_total",
			Help: "Network receive bytes by interface since boot.",
		}, []string{"interface"}),
		networkSend: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_network_transmit_bytes_total",
			Help: "Network transmit bytes by interface since boot.",
		}, []string{"interface"}),
		diskReadBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_disk_read_bytes_total",
			Help: "Disk read bytes by device since boot.",
		}, []string{"device"}),
		diskWriteBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_disk_write_bytes_total",
			Help: "Disk write bytes by device since boot.",
		}, []string{"device"}),
		processCPU: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_top_process_cpu_percent",
			Help: "CPU usage percent of top processes, capped by rank.",
		}, []string{"rank", "process_name"}),
		processMemory: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_top_process_memory_percent",
			Help: "Memory usage percent of top processes, capped by rank.",
		}, []string{"rank", "process_name"}),
		portListening: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "opshub_agent_port_listening",
			Help: "Listening ports exposed by the host.",
		}, []string{"port", "protocol", "process_name"}),
		processCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_process_count",
			Help: "Process count reported by the agent.",
		}),
		listenPortInfo: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opshub_agent_listen_port",
			Help: "Listen port exposed by the agent.",
		}),
	}

	prometheus.MustRegister(
		items.up,
		items.reportTS,
		items.cpuUsage,
		items.load1,
		items.load5,
		items.load15,
		items.memoryTotal,
		items.memoryUsed,
		items.memoryUsage,
		items.diskUsage,
		items.diskTotal,
		items.diskUsed,
		items.networkRecv,
		items.networkSend,
		items.diskReadBytes,
		items.diskWriteBytes,
		items.processCPU,
		items.processMemory,
		items.portListening,
		items.processCount,
		items.listenPortInfo,
	)

	items.up.Set(1)
	return items
}

func (a *agentApp) serveMetrics(ctx context.Context) (<-chan error, error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/v1/snapshot", a.requireAgentAuth(a.handleSnapshot))
	mux.HandleFunc("/snapshot", a.requireAgentAuth(a.handleSnapshot))
	mux.HandleFunc("/files", a.requireAgentAuth(a.handleFiles))
	mux.HandleFunc("/files/upload", a.requireAgentAuth(a.handleFileUpload))
	mux.HandleFunc("/files/download", a.requireAgentAuth(a.handleFileDownload))

	listener, err := net.Listen("tcp", a.cfg.ListenAddr)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              a.cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		defer close(errCh)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	return errCh, nil
}

func (a *agentApp) requireAgentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if extractBearerToken(r.Header.Get("Authorization")) != strings.TrimSpace(a.cfg.AccessToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a *agentApp) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := collectPayload(r.Context(), a.cfg, a.httpClient)
	if err != nil {
		http.Error(w, fmt.Sprintf("collect snapshot failed: %v", err), http.StatusInternalServerError)
		return
	}

	a.updateMetrics(payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *agentApp) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleFileList(w, r)
	case http.MethodDelete:
		a.handleFileDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *agentApp) handleFileList(w http.ResponseWriter, r *http.Request) {
	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	fullPath, isRoot, err := resolveAgentRequestedPath(requestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var entries []*fileEntry
	if runtime.GOOS == "windows" && isRoot {
		entries, err = listWindowsDriveEntries()
	} else {
		entries, err = listLocalFileEntries(fullPath)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (a *agentApp) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestPath := strings.TrimSpace(r.FormValue("path"))
	fullPath, isRoot, err := resolveAgentRequestedPath(requestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if runtime.GOOS == "windows" && isRoot {
		http.Error(w, "不能直接上传到驱动器列表，请先进入具体目录", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("访问目录失败: %v", err), http.StatusBadRequest)
		return
	}
	if !info.IsDir() {
		http.Error(w, "上传目标不是目录", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "缺少上传文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := filepath.Base(header.Filename)
	if fileName == "." || fileName == "" {
		http.Error(w, "文件名无效", http.StatusBadRequest)
		return
	}

	destination := filepath.Join(fullPath, fileName)
	target, err := os.Create(destination)
	if err != nil {
		http.Error(w, fmt.Sprintf("创建目标文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer target.Close()

	if _, err := io.Copy(target, file); err != nil {
		http.Error(w, fmt.Sprintf("写入目标文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *agentApp) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	fullPath, isRoot, err := resolveAgentRequestedPath(requestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if runtime.GOOS == "windows" && isRoot {
		http.Error(w, "请指定具体文件路径", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取文件信息失败: %v", err), http.StatusBadRequest)
		return
	}
	if info.IsDir() {
		http.Error(w, "暂不支持下载目录", http.StatusBadRequest)
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("打开文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(fullPath)))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, file)
}

func (a *agentApp) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	fullPath, isRoot, err := resolveAgentRequestedPath(requestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if runtime.GOOS == "windows" && isRoot {
		http.Error(w, "请指定具体文件路径", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取文件信息失败: %v", err), http.StatusBadRequest)
		return
	}
	if info.IsDir() {
		http.Error(w, "当前版本仅支持删除文件", http.StatusBadRequest)
		return
	}

	if err := os.Remove(fullPath); err != nil {
		http.Error(w, fmt.Sprintf("删除文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func resolveAgentRequestedPath(requestPath string) (string, bool, error) {
	if runtime.GOOS == "windows" {
		normalized := normalizeWindowsAgentPath(requestPath)
		if normalized == "" {
			return "", true, nil
		}
		if !isWindowsAbsolutePath(normalized) {
			return "", false, fmt.Errorf("Windows 文件路径必须是绝对路径")
		}
		return normalized, false, nil
	}

	normalized := strings.TrimSpace(requestPath)
	if normalized == "" {
		normalized = "/"
	}
	normalized = filepath.Clean(normalized)
	if !filepath.IsAbs(normalized) {
		return "", false, fmt.Errorf("文件路径必须是绝对路径")
	}
	return normalized, false, nil
}

func normalizeWindowsAgentPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "/", `\`))
	if path == "" {
		return ""
	}
	if len(path) == 2 && path[1] == ':' {
		return path + `\`
	}
	if len(path) >= 3 && path[1] == ':' && path[2] != '\\' {
		return path[:2] + `\` + path[2:]
	}
	return filepath.Clean(path)
}

func isWindowsAbsolutePath(path string) bool {
	return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func listWindowsDriveEntries() ([]*fileEntry, error) {
	entries := make([]*fileEntry, 0, 8)
	for drive := 'A'; drive <= 'Z'; drive++ {
		drivePath := fmt.Sprintf("%c:\\", drive)
		info, err := os.Stat(drivePath)
		if err != nil || !info.IsDir() {
			continue
		}

		modTime := ""
		if !info.ModTime().IsZero() {
			modTime = info.ModTime().Format("2006-01-02 15:04:05")
		}

		entries = append(entries, &fileEntry{
			Name:    fmt.Sprintf("%c:", drive),
			Path:    drivePath,
			Size:    0,
			Mode:    "drive",
			IsDir:   true,
			ModTime: modTime,
		})
	}
	return entries, nil
}

func listLocalFileEntries(directory string) ([]*fileEntry, error) {
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("访问目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", directory)
	}

	items, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	entries := make([]*fileEntry, 0, len(items))
	for _, item := range items {
		itemInfo, err := item.Info()
		if err != nil {
			continue
		}

		entries = append(entries, &fileEntry{
			Name:    item.Name(),
			Path:    filepath.Join(directory, item.Name()),
			Size:    itemInfo.Size(),
			Mode:    itemInfo.Mode().String(),
			IsDir:   item.IsDir(),
			ModTime: itemInfo.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return entries, nil
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func (a *agentApp) run(ctx context.Context) {
	a.reportOnce(ctx)

	ticker := time.NewTicker(time.Duration(a.cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.reportOnce(ctx)
		}
	}
}

func (a *agentApp) reportOnce(ctx context.Context) {
	payload, err := collectPayload(ctx, a.cfg, a.httpClient)
	if err != nil {
		log.Printf("collect payload: %v", err)
		return
	}

	if err := a.sendReport(ctx, payload); err != nil {
		log.Printf("send report: %v", err)
		return
	}

	a.updateMetrics(payload)
}

func collectPayload(ctx context.Context, cfg *agentConfig, httpClient *http.Client) (*reportPayload, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	cpuItems, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	cpuPercent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}

	virtualMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	swapMem, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		swapMem = &mem.SwapMemoryStat{}
	}

	privateIPs, interfacePublicIPs, interfaces, err := collectInterfaces()
	if err != nil {
		return nil, err
	}
	publicIPs := collectReportedPublicIPs(ctx, cfg.ReportURL, httpClient, interfacePublicIPs)
	loadAvg := collectLoad(ctx)
	networkIO := collectNetworkIO(ctx)
	diskIO := collectDiskIO(ctx)
	disks := collectDisks(ctx)
	processes := collectProcesses(ctx)
	ports := collectListeningPorts(ctx)

	payload := &reportPayload{
		Version:        cfg.Version,
		Hostname:       info.Hostname,
		OS:             strings.TrimSpace(strings.Join([]string{info.Platform, info.PlatformVersion}, " ")),
		Kernel:         info.KernelVersion,
		Arch:           info.KernelArch,
		Uptime:         formatUptime(info.Uptime),
		ListenPort:     extractListenPort(cfg.ListenAddr),
		PrivateIPs:     privateIPs,
		PublicIPs:      publicIPs,
		Interfaces:     interfaces,
		Load:           loadAvg,
		Network:        networkIO,
		DiskIO:         diskIO,
		Disk:           disks,
		TopProcesses:   processes,
		ListeningPorts: ports,
		ConfigSummary: configSummary{
			OSRelease:        strings.TrimSpace(strings.Join([]string{info.Platform, info.PlatformFamily, info.PlatformVersion}, " ")),
			Kernel:           info.KernelVersion,
			Arch:             info.KernelArch,
			Timezone:         time.Now().Location().String(),
			ServiceManager:   detectServiceManager(),
			ContainerRuntime: detectContainerRuntime(),
			Mounts:           collectMounts(disks),
			AgentVersion:     cfg.Version,
		},
	}

	if len(cpuItems) > 0 {
		totalCores := 0
		totalThreads := 0
		for _, item := range cpuItems {
			totalCores += int(item.Cores)
			if item.Cores > 0 {
				totalThreads += int(item.Cores)
			}
		}
		payload.CPU = cpuInfo{
			ModelName: cpuItems[0].ModelName,
			Cores:     totalCores,
			Threads:   maxInt(totalThreads, len(cpuItems)),
			Usage:     firstFloat(cpuPercent),
			MHz:       cpuItems[0].Mhz,
			VendorID:  cpuItems[0].VendorID,
		}
	}

	if cpuCounts, err := cpu.CountsWithContext(ctx, true); err == nil && cpuCounts > 0 {
		payload.CPU.Threads = cpuCounts
	}

	payload.Memory = memoryInfo{
		Total:     virtualMem.Total,
		Used:      virtualMem.Used,
		Free:      virtualMem.Free,
		Available: virtualMem.Available,
		Usage:     virtualMem.UsedPercent,
		SwapTotal: swapMem.Total,
		SwapUsed:  swapMem.Used,
	}

	if strings.TrimSpace(payload.OS) == "" {
		payload.OS = info.OS
	}
	if strings.TrimSpace(payload.Arch) == "" {
		payload.Arch = info.KernelArch
	}

	return payload, nil
}

func collectReportedPublicIPs(ctx context.Context, reportURL string, httpClient *http.Client, interfacePublicIPs []string) []string {
	items := make([]string, 0, len(interfacePublicIPs)+2)

	if observedIP, err := queryObservedPublicIP(ctx, httpClient, resolveAgentEchoURL(reportURL)); err == nil {
		items = appendUniquePublicIP(items, observedIP)
	}

	if observedIP, err := queryObservedPublicIP(ctx, httpClient, defaultPublicIPEchoFallbackURL); err == nil {
		items = appendUniquePublicIP(items, observedIP)
	}

	for _, ip := range interfacePublicIPs {
		items = appendUniquePublicIP(items, ip)
	}

	return items
}

func resolveAgentEchoURL(reportURL string) string {
	reportURL = strings.TrimSpace(reportURL)
	if reportURL == "" {
		return ""
	}

	parsed, err := url.Parse(reportURL)
	if err != nil {
		return ""
	}

	if strings.HasSuffix(parsed.Path, "/report") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/report") + "/echo-ip"
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/echo-ip"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func queryObservedPublicIP(ctx context.Context, httpClient *http.Client, endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", errors.New("empty endpoint")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.9, */*;q=0.8")

	client := httpClient
	if baseTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		directTransport := baseTransport.Clone()
		directTransport.Proxy = nil
		client = &http.Client{
			Timeout:   5 * time.Second,
			Transport: directTransport,
		}
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}

	return extractObservedPublicIP(data), nil
}

func extractObservedPublicIP(data []byte) string {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}

	if strings.HasPrefix(text, "{") {
		var payload struct {
			ObservedIP string `json:"observedIp"`
			IP         string `json:"ip"`
		}
		if err := json.Unmarshal([]byte(text), &payload); err == nil {
			if ip := normalizeObservedPublicIP(payload.ObservedIP); ip != "" {
				return ip
			}
			if ip := normalizeObservedPublicIP(payload.IP); ip != "" {
				return ip
			}
		}
	}

	return normalizeObservedPublicIP(text)
}

func appendUniquePublicIP(items []string, candidate string) []string {
	candidate = normalizeObservedPublicIP(candidate)
	if candidate == "" {
		return items
	}
	for _, existing := range items {
		if existing == candidate {
			return items
		}
	}
	return append(items, candidate)
}

func normalizeObservedPublicIP(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(candidate); err == nil {
		candidate = host
	}
	candidate = strings.Trim(candidate, "[]")

	ip := net.ParseIP(candidate)
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

func collectLoad(ctx context.Context) loadInfo {
	avg, err := load.AvgWithContext(ctx)
	if err != nil || avg == nil {
		return loadInfo{}
	}
	return loadInfo{
		Load1:  avg.Load1,
		Load5:  avg.Load5,
		Load15: avg.Load15,
	}
}

func collectInterfaces() ([]string, []string, []networkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, nil, err
	}

	privateSet := make(map[string]struct{})
	publicSet := make(map[string]struct{})
	items := make([]networkInterface, 0, len(ifaces))

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		addresses := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			addrText := ip.String()
			addresses = append(addresses, addrText)
			switch {
			case ip.IsPrivate():
				privateSet[addrText] = struct{}{}
			case ip.IsGlobalUnicast():
				publicSet[addrText] = struct{}{}
			}
		}

		if len(addresses) == 0 {
			continue
		}

		items = append(items, networkInterface{
			Name:      iface.Name,
			Hardware:  iface.HardwareAddr.String(),
			Addresses: addresses,
		})
	}

	return mapKeysSorted(privateSet), mapKeysSorted(publicSet), items, nil
}

func collectDisks(ctx context.Context) []diskInfo {
	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil
	}

	items := make([]diskInfo, 0, len(partitions))
	for _, partition := range partitions {
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			continue
		}
		items = append(items, diskInfo{
			Device:     partition.Device,
			MountPoint: partition.Mountpoint,
			Fstype:     partition.Fstype,
			Total:      usage.Total,
			Used:       usage.Used,
			Free:       usage.Free,
			Usage:      usage.UsedPercent,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].MountPoint < items[j].MountPoint
	})
	return items
}

func collectNetworkIO(ctx context.Context) []networkIOInfo {
	stats, err := gnet.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil
	}

	items := make([]networkIOInfo, 0, len(stats))
	for _, stat := range stats {
		name := strings.TrimSpace(stat.Name)
		if shouldIgnoreMetricInterface(name) {
			continue
		}
		items = append(items, networkIOInfo{
			Name:      name,
			BytesRecv: stat.BytesRecv,
			BytesSent: stat.BytesSent,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func collectDiskIO(ctx context.Context) []diskIOInfo {
	stats, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil
	}

	items := make([]diskIOInfo, 0, len(stats))
	for device, stat := range stats {
		name := strings.TrimSpace(device)
		if shouldIgnoreDiskIODevice(name) {
			continue
		}
		items = append(items, diskIOInfo{
			Device:     name,
			ReadBytes:  stat.ReadBytes,
			WriteBytes: stat.WriteBytes,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Device < items[j].Device
	})
	return items
}

func collectProcesses(ctx context.Context) []processInfo {
	ps, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}

	items := make([]processInfo, 0, len(ps))
	for _, proc := range ps {
		name, _ := proc.NameWithContext(ctx)
		cmdline, _ := proc.CmdlineWithContext(ctx)
		cpuPercent, _ := proc.CPUPercentWithContext(ctx)
		memPercent, _ := proc.MemoryPercentWithContext(ctx)

		items = append(items, processInfo{
			PID:           int(proc.Pid),
			Name:          name,
			Command:       truncate(cmdline, 200),
			CPUPercent:    cpuPercent,
			MemoryPercent: float64(memPercent),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CPUPercent == items[j].CPUPercent {
			return items[i].MemoryPercent > items[j].MemoryPercent
		}
		return items[i].CPUPercent > items[j].CPUPercent
	})

	if len(items) > 20 {
		items = items[:20]
	}
	return items
}

func collectListeningPorts(ctx context.Context) []portInfo {
	conns, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil
	}

	items := make([]portInfo, 0, len(conns))
	seen := make(map[string]struct{})
	for _, conn := range conns {
		if conn.Status != "LISTEN" || conn.Laddr.Port == 0 {
			continue
		}

		key := fmt.Sprintf("%s:%d:%d", conn.Laddr.IP, conn.Laddr.Port, conn.Pid)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		processName := ""
		if conn.Pid > 0 {
			if proc, err := process.NewProcessWithContext(ctx, conn.Pid); err == nil {
				processName, _ = proc.NameWithContext(ctx)
			}
		}

		items = append(items, portInfo{
			Protocol:    "tcp",
			Port:        int(conn.Laddr.Port),
			ListenAddr:  conn.Laddr.IP,
			ProcessName: processName,
			PID:         int(conn.Pid),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Port == items[j].Port {
			return items[i].PID < items[j].PID
		}
		return items[i].Port < items[j].Port
	})

	if len(items) > 100 {
		items = items[:100]
	}
	return items
}

func (a *agentApp) sendReport(ctx context.Context, payload *reportPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.ReportURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (a *agentApp) updateMetrics(payload *reportPayload) {
	now := float64(time.Now().Unix())
	a.metrics.reportTS.Set(now)
	a.metrics.cpuUsage.Set(payload.CPU.Usage)
	a.metrics.load1.Set(payload.Load.Load1)
	a.metrics.load5.Set(payload.Load.Load5)
	a.metrics.load15.Set(payload.Load.Load15)
	a.metrics.memoryTotal.Set(float64(payload.Memory.Total))
	a.metrics.memoryUsed.Set(float64(payload.Memory.Used))
	a.metrics.memoryUsage.Set(payload.Memory.Usage)
	a.metrics.processCount.Set(float64(len(payload.TopProcesses)))
	a.metrics.listenPortInfo.Set(float64(payload.ListenPort))

	a.metrics.diskUsage.Reset()
	a.metrics.diskTotal.Reset()
	a.metrics.diskUsed.Reset()
	for _, item := range payload.Disk {
		labels := prometheus.Labels{
			"mount_point": item.MountPoint,
			"device":      item.Device,
		}
		a.metrics.diskUsage.With(labels).Set(item.Usage)
		a.metrics.diskTotal.With(labels).Set(float64(item.Total))
		a.metrics.diskUsed.With(labels).Set(float64(item.Used))
	}

	a.metrics.networkRecv.Reset()
	a.metrics.networkSend.Reset()
	for _, item := range payload.Network {
		labels := prometheus.Labels{"interface": sanitizeMetricLabel(item.Name, "unknown")}
		a.metrics.networkRecv.With(labels).Set(float64(item.BytesRecv))
		a.metrics.networkSend.With(labels).Set(float64(item.BytesSent))
	}

	a.metrics.diskReadBytes.Reset()
	a.metrics.diskWriteBytes.Reset()
	allowedDiskDevices := buildMountedDiskIODeviceSet(payload.Disk)
	for _, item := range payload.DiskIO {
		if len(allowedDiskDevices) > 0 {
			if _, ok := allowedDiskDevices[normalizeMetricDiskDeviceName(item.Device)]; !ok {
				continue
			}
		}
		labels := prometheus.Labels{"device": sanitizeMetricLabel(item.Device, "unknown")}
		a.metrics.diskReadBytes.With(labels).Set(float64(item.ReadBytes))
		a.metrics.diskWriteBytes.With(labels).Set(float64(item.WriteBytes))
	}

	a.metrics.processCPU.Reset()
	a.metrics.processMemory.Reset()
	for idx, item := range payload.TopProcesses[:minInt(len(payload.TopProcesses), maxProcessMetrics)] {
		labels := prometheus.Labels{
			"rank":         strconv.Itoa(idx + 1),
			"process_name": sanitizeMetricLabel(item.Name, "unknown"),
		}
		a.metrics.processCPU.With(labels).Set(item.CPUPercent)
		a.metrics.processMemory.With(labels).Set(item.MemoryPercent)
	}

	a.metrics.portListening.Reset()
	for _, item := range payload.ListeningPorts[:minInt(len(payload.ListeningPorts), maxPortMetrics)] {
		labels := prometheus.Labels{
			"port":         strconv.Itoa(item.Port),
			"protocol":     sanitizeMetricLabel(item.Protocol, "tcp"),
			"process_name": sanitizeMetricLabel(item.ProcessName, "unknown"),
		}
		a.metrics.portListening.With(labels).Set(1)
	}
}

func mustHandleUnwrapResponse() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return
	}

	var envelope responseEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil || len(bytes.TrimSpace(envelope.Data)) == 0 || string(bytes.TrimSpace(envelope.Data)) == "null" {
		_, _ = os.Stdout.Write(trimmed)
		return
	}

	_, _ = os.Stdout.Write(bytes.TrimSpace(envelope.Data))
}

func mustHandleJSONField() {
	if len(os.Args) < 3 {
		log.Fatal("missing field name")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Fatal(err)
	}

	value, ok := payload[os.Args[2]]
	if !ok || value == nil {
		return
	}

	switch item := value.(type) {
	case string:
		fmt.Print(item)
	case float64:
		if item == math.Trunc(item) {
			fmt.Print(strconv.FormatInt(int64(item), 10))
		} else {
			fmt.Print(strconv.FormatFloat(item, 'f', -1, 64))
		}
	case bool:
		fmt.Print(strconv.FormatBool(item))
	default:
		raw, err := json.Marshal(item)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(string(raw))
	}
}

func extractListenPort(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 19100
	}
	value, err := strconv.Atoi(port)
	if err != nil {
		return 19100
	}
	return value
}

func formatUptime(seconds uint64) string {
	d := time.Duration(seconds) * time.Second
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func detectServiceManager() string {
	if manager := platformServiceManager(); manager != "" {
		return manager
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return "systemd"
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return "openrc"
	}
	return "unknown"
}

func detectContainerRuntime() string {
	switch {
	case fileExists("/var/run/docker.sock") || commandExists("docker"):
		return "docker"
	case fileExists("/run/containerd/containerd.sock") || commandExists("containerd"):
		return "containerd"
	case fileExists("/var/run/crio/crio.sock") || commandExists("crio"):
		return "cri-o"
	case commandExists("podman"):
		return "podman"
	default:
		return "unknown"
	}
}

func collectMounts(disks []diskInfo) []string {
	mounts := make([]string, 0, len(disks))
	for _, item := range disks {
		if item.MountPoint == "" {
			continue
		}
		mounts = append(mounts, item.MountPoint)
	}
	sort.Strings(mounts)
	return mounts
}

func mapKeysSorted(items map[string]struct{}) []string {
	values := make([]string, 0, len(items))
	for key := range items {
		values = append(values, key)
	}
	sort.Strings(values)
	return values
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstFloat(items []float64) float64 {
	if len(items) == 0 {
		return 0
	}
	return items[0]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sanitizeMetricLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func shouldIgnoreMetricInterface(name string) bool {
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

func shouldIgnoreDiskIODevice(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return true
	}
	return strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram")
}

func buildMountedDiskIODeviceSet(disks []diskInfo) map[string]struct{} {
	if len(disks) == 0 {
		return nil
	}

	devices := make(map[string]struct{}, len(disks))
	for _, disk := range disks {
		if shouldIgnoreMountedDiskMetric(disk) {
			continue
		}
		name := normalizeMetricDiskDeviceName(disk.Device)
		if name == "" {
			continue
		}
		devices[name] = struct{}{}
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}

func normalizeMetricDiskDeviceName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.TrimRight(value, `\/`)
	value = strings.TrimPrefix(value, "/dev/")
	value = strings.TrimPrefix(value, `\\.\`)
	return value
}

func shouldIgnoreMountedDiskMetric(disk diskInfo) bool {
	device := strings.ToLower(strings.TrimSpace(disk.Device))
	mountPoint := strings.TrimSpace(disk.MountPoint)

	if mountPoint == "" {
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
		"efivarfs",
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

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
