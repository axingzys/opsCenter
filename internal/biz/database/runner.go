package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	defaultRunnerWorkDir        = "/var/lib/opshub/database-runner"
	defaultRunnerTimeoutMinutes = 30
	maxRunnerTimeoutMinutes     = 1440
	maxRunnerJSONLength         = 4000
	maxRunnerOutputLength       = 6000
)

type DatabaseRunnerHostListRequest struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	Keyword    string `form:"keyword"`
	RunnerType string `form:"runnerType"`
	Status     string `form:"status"`
	Enabled    string `form:"enabled"`
}

type DatabaseRunnerHostRequest struct {
	Name              string `json:"name" binding:"required,max=120"`
	RunnerType        string `json:"runnerType" binding:"omitempty,max=30"`
	Host              string `json:"host" binding:"omitempty,max=255"`
	Port              int    `json:"port" binding:"omitempty,min=1,max=65535"`
	CredentialID      uint   `json:"credentialId"`
	WorkDir           string `json:"workDir" binding:"omitempty,max=500"`
	StorageMountPath  string `json:"storageMountPath" binding:"omitempty,max=500"`
	MaxConcurrentJobs int    `json:"maxConcurrentJobs" binding:"omitempty,min=1,max=100"`
	CPULimit          string `json:"cpuLimit" binding:"omitempty,max=60"`
	IOLimit           string `json:"ioLimit" binding:"omitempty,max=60"`
	BandwidthLimit    string `json:"bandwidthLimit" binding:"omitempty,max=60"`
	TimeoutMinutes    int    `json:"timeoutMinutes" binding:"omitempty,min=1,max=1440"`
	Enabled           bool   `json:"enabled"`
	ConfigJSON        string `json:"configJson" binding:"omitempty,max=4000"`
}

type DatabaseRunnerHostVO struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	RunnerType        string `json:"runnerType"`
	RunnerTypeText    string `json:"runnerTypeText"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	CredentialID      uint   `json:"credentialId"`
	WorkDir           string `json:"workDir"`
	StorageMountPath  string `json:"storageMountPath"`
	MaxConcurrentJobs int    `json:"maxConcurrentJobs"`
	CPULimit          string `json:"cpuLimit"`
	IOLimit           string `json:"ioLimit"`
	BandwidthLimit    string `json:"bandwidthLimit"`
	TimeoutMinutes    int    `json:"timeoutMinutes"`
	Enabled           bool   `json:"enabled"`
	Status            string `json:"status"`
	StatusText        string `json:"statusText"`
	LastHeartbeatAt   string `json:"lastHeartbeatAt"`
	LastTestAt        string `json:"lastTestAt"`
	LastError         string `json:"lastError"`
	ConfigJSON        string `json:"configJson"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type DatabaseRunnerJobVO struct {
	ID               uint   `json:"id"`
	JobType          string `json:"jobType"`
	JobTypeText      string `json:"jobTypeText"`
	RunnerHostID     uint   `json:"runnerHostId"`
	RunnerHostName   string `json:"runnerHostName"`
	RunnerID         string `json:"runnerId"`
	SourceInstanceID uint   `json:"sourceInstanceId"`
	TargetInstanceID uint   `json:"targetInstanceId"`
	Status           string `json:"status"`
	StatusText       string `json:"statusText"`
	AllowedCommand   string `json:"allowedCommand"`
	CommandSummary   string `json:"commandSummary"`
	WorkDir          string `json:"workDir"`
	LogPath          string `json:"logPath"`
	ExitCode         int    `json:"exitCode"`
	OperatorID       uint   `json:"operatorId"`
	OperatorName     string `json:"operatorName"`
	RequestJSON      string `json:"requestJson"`
	ResultJSON       string `json:"resultJson"`
	HeartbeatAt      string `json:"heartbeatAt"`
	StartedAt        string `json:"startedAt"`
	FinishedAt       string `json:"finishedAt"`
	DurationMs       int64  `json:"durationMs"`
	ErrorMessage     string `json:"errorMessage"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type runnerProbeResult struct {
	RunnerHostID uint   `json:"runnerHostId"`
	RunnerID     string `json:"runnerId"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
}

func (uc *UseCase) CreateRunnerHost(ctx context.Context, req *DatabaseRunnerHostRequest) (*DatabaseRunnerHostVO, error) {
	if uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("Runner 主机仓库未配置")
	}
	item, err := uc.buildRunnerHostFromRequest(ctx, nil, req)
	if err != nil {
		return nil, err
	}
	if err := uc.runnerHostRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return toRunnerHostVO(item), nil
}

func (uc *UseCase) UpdateRunnerHost(ctx context.Context, id uint, req *DatabaseRunnerHostRequest) (*DatabaseRunnerHostVO, error) {
	if uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("Runner 主机仓库未配置")
	}
	if id == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	item, err := uc.runnerHostRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	item, err = uc.buildRunnerHostFromRequest(ctx, item, req)
	if err != nil {
		return nil, err
	}
	if err := uc.runnerHostRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return toRunnerHostVO(item), nil
}

func (uc *UseCase) ListRunnerHosts(ctx context.Context, req *DatabaseRunnerHostListRequest) ([]*DatabaseRunnerHostVO, int64, error) {
	if uc.runnerHostRepo == nil {
		return nil, 0, fmt.Errorf("Runner 主机仓库未配置")
	}
	items, total, err := uc.runnerHostRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseRunnerHostVO, 0, len(items))
	for _, item := range items {
		list = append(list, toRunnerHostVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) TestRunnerHost(ctx context.Context, id uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("Runner 仓库未配置")
	}
	if id == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	job := &DatabaseRunnerJob{
		JobType:        DatabaseRunnerJobTypeProbe,
		RunnerHostID:   host.ID,
		RunnerID:       runnerIDForHost(host),
		Status:         DatabaseRunnerJobStatusQueued,
		AllowedCommand: DatabaseRunnerAllowedCommandProbe,
		CommandSummary: "Runner 主机连通性和工具探测",
		WorkDir:        host.WorkDir,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 120),
		RequestJSON:    runnerRequestJSON(host, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeRunnerProbe(context.Background(), host.ID, job.ID)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) ListRunnerJobs(ctx context.Context, req *DatabaseRunnerJobListRequest) ([]*DatabaseRunnerJobVO, int64, error) {
	if uc.runnerJobRepo == nil {
		return nil, 0, fmt.Errorf("Runner Job 仓库未配置")
	}
	items, total, err := uc.runnerJobRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseRunnerJobVO, 0, len(items))
	for _, item := range items {
		list = append(list, uc.toRunnerJobVO(ctx, item))
	}
	return list, total, nil
}

func (uc *UseCase) buildRunnerHostFromRequest(ctx context.Context, item *DatabaseRunnerHost, req *DatabaseRunnerHostRequest) (*DatabaseRunnerHost, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("Runner 主机名称不能为空")
	}
	runnerType := normalizeRunnerType(req.RunnerType)
	host := strings.TrimSpace(req.Host)
	port := req.Port
	if port <= 0 {
		port = 22
	}
	if runnerType == DatabaseRunnerTypeSSH {
		if host == "" {
			return nil, fmt.Errorf("SSH Runner 主机地址不能为空")
		}
		if req.CredentialID == 0 {
			return nil, fmt.Errorf("SSH Runner 必须选择连接凭据")
		}
	}
	if req.CredentialID > 0 && uc.credentialIDExists != nil {
		if err := uc.credentialIDExists(ctx, req.CredentialID); err != nil {
			return nil, fmt.Errorf("Runner 连接凭据不存在")
		}
	}
	if err := validateBackupStorageConfigSafe(req.ConfigJSON); err != nil {
		return nil, err
	}
	workDir := trimText(strings.TrimSpace(req.WorkDir), 500)
	if workDir == "" {
		workDir = defaultRunnerWorkDir
	}
	timeoutMinutes := normalizeRunnerTimeoutMinutes(req.TimeoutMinutes)
	status := DatabaseRunnerHostStatusPending
	if !req.Enabled {
		status = DatabaseRunnerHostStatusDisabled
	}
	if item == nil {
		item = &DatabaseRunnerHost{}
	} else if item.Status != "" && item.Status != DatabaseRunnerHostStatusDisabled {
		status = item.Status
	}
	item.Name = trimText(strings.TrimSpace(req.Name), 120)
	item.RunnerType = runnerType
	item.Host = trimText(host, 255)
	item.Port = port
	item.CredentialID = req.CredentialID
	item.WorkDir = workDir
	item.StorageMountPath = trimText(strings.TrimSpace(req.StorageMountPath), 500)
	item.MaxConcurrentJobs = normalizeRunnerMaxConcurrentJobs(req.MaxConcurrentJobs)
	item.CPULimit = trimText(strings.TrimSpace(req.CPULimit), 60)
	item.IOLimit = trimText(strings.TrimSpace(req.IOLimit), 60)
	item.BandwidthLimit = trimText(strings.TrimSpace(req.BandwidthLimit), 60)
	item.TimeoutMinutes = timeoutMinutes
	item.Enabled = req.Enabled
	item.Status = status
	item.ConfigJSON = trimText(strings.TrimSpace(req.ConfigJSON), maxRunnerJSONLength)
	if !req.Enabled {
		item.LastError = ""
	}
	return item, nil
}

func (uc *UseCase) executeRunnerProbe(ctx context.Context, runnerHostID, jobID uint) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return
	}
	host, hostErr := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	job, jobErr := uc.runnerJobRepo.GetByID(ctx, jobID)
	if hostErr != nil || jobErr != nil || host == nil || job == nil {
		return
	}
	started := time.Now()
	job.Status = DatabaseRunnerJobStatusRunning
	job.StartedAt = &started
	job.HeartbeatAt = &started
	_ = uc.runnerJobRepo.Update(ctx, job)

	stdout, stderr, exitCode, err := uc.runRunnerProbeCommand(ctx, host)
	finished := time.Now()
	applyRunnerProbeResult(host, job, started, finished, stdout, stderr, exitCode, err)
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.runnerHostRepo.Update(ctx, host)
}

func applyRunnerProbeResult(host *DatabaseRunnerHost, job *DatabaseRunnerJob, started, finished time.Time, stdout, stderr string, exitCode int, runErr error) {
	if host == nil || job == nil {
		return
	}
	result := runnerProbeResult{
		RunnerHostID: host.ID,
		RunnerID:     runnerIDForHost(host),
		Stdout:       trimText(stdout, maxRunnerOutputLength),
		Stderr:       trimText(stderr, maxRunnerOutputLength),
		ExitCode:     exitCode,
		StartedAt:    started.Format("2006-01-02 15:04:05"),
		FinishedAt:   finished.Format("2006-01-02 15:04:05"),
		DurationMs:   finished.Sub(started).Milliseconds(),
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = exitCode
	job.FinishedAt = &finished
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &finished
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		host.Status = DatabaseRunnerHostStatusFailed
		host.LastError = job.ErrorMessage
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		host.Status = DatabaseRunnerHostStatusOnline
		host.LastError = ""
	}
	host.LastTestAt = &finished
}

func (uc *UseCase) runRunnerProbeCommand(ctx context.Context, host *DatabaseRunnerHost) (string, string, int, error) {
	if host == nil {
		return "", "", 1, fmt.Errorf("Runner 主机不能为空")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return "", "", 1, fmt.Errorf("当前 P2.2 仅支持 SSH Runner 探测")
	}
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	command := buildRunnerProbeCommand(host.WorkDir)
	return executeSSHRunnerCommand(ctx, host.Host, host.Port, credential, command, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func executeSSHRunnerCommand(ctx context.Context, host string, port int, credential *ConnectionCredential, command string, timeout time.Duration) (string, string, int, error) {
	return executeSSHRunnerSession(ctx, host, port, credential, command, nil, timeout)
}

func executeSSHRunnerScript(ctx context.Context, host string, port int, credential *ConnectionCredential, script string, timeout time.Duration) (string, string, int, error) {
	return executeSSHRunnerSession(ctx, host, port, credential, "sh -s", strings.NewReader(script), timeout)
}

func executeSSHRunnerSession(ctx context.Context, host string, port int, credential *ConnectionCredential, command string, stdin io.Reader, timeout time.Duration) (string, string, int, error) {
	if credential == nil {
		return "", "", 1, fmt.Errorf("连接凭据不能为空")
	}
	if strings.TrimSpace(credential.Username) == "" {
		return "", "", 1, fmt.Errorf("SSH 凭据用户名不能为空")
	}
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if privateKey := strings.TrimSpace(credential.PrivateKey); privateKey != "" {
		signer, err := parseSSHPrivateKey(privateKey, credential.Passphrase)
		if err != nil {
			return "", "", 1, err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if credential.Password != "" {
		authMethods = append(authMethods, ssh.Password(credential.Password))
	}
	if len(authMethods) == 0 {
		return "", "", 1, fmt.Errorf("SSH 凭据必须包含密码或私钥")
	}
	if timeout <= 0 {
		timeout = time.Duration(defaultRunnerTimeoutMinutes) * time.Minute
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return "", "", 1, fmt.Errorf("SSH连接失败: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            strings.TrimSpace(credential.Username),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return "", "", 1, fmt.Errorf("SSH认证失败: %w", err)
	}
	client := ssh.NewClient(clientConn, chans, reqs)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", "", 1, fmt.Errorf("创建SSH session失败: %w", err)
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = stdin
	}
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return stdout.String(), stderr.String(), 1, ctx.Err()
	case <-time.After(timeout):
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return stdout.String(), stderr.String(), 1, fmt.Errorf("Runner 命令执行超时")
	case err := <-done:
		if err != nil {
			exitCode := 1
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			}
			return stdout.String(), stderr.String(), exitCode, fmt.Errorf("Runner 命令执行失败: %w", err)
		}
		return stdout.String(), stderr.String(), 0, nil
	}
}

func parseSSHPrivateKey(privateKey, passphrase string) (ssh.Signer, error) {
	if strings.TrimSpace(passphrase) != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("解析SSH私钥失败: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("解析SSH私钥失败: %w", err)
	}
	return signer, nil
}

func buildRunnerProbeCommand(workDir string) string {
	if strings.TrimSpace(workDir) == "" {
		workDir = defaultRunnerWorkDir
	}
	quotedWorkDir := shellSingleQuote(workDir)
	return strings.Join([]string{
		"set -e",
		"WORK_DIR=" + quotedWorkDir,
		`mkdir -p "$WORK_DIR"`,
		`cd "$WORK_DIR"`,
		`printf 'opshub-runner-ok\n'`,
		"pwd",
		"whoami",
		"hostname",
		"command -v xtrabackup || true",
		"command -v mariadb-backup || true",
		"command -v mysqlbinlog || true",
		"command -v barman || true",
		"command -v wal-g || true",
	}, "\n")
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func runnerRequestJSON(host *DatabaseRunnerHost, operator QueryOperator) string {
	payload := map[string]any{
		"runnerHostId":   host.ID,
		"runnerType":     host.RunnerType,
		"host":           host.Host,
		"port":           host.Port,
		"workDir":        host.WorkDir,
		"allowedCommand": DatabaseRunnerAllowedCommandProbe,
		"operatorId":     operator.ID,
		"operatorName":   operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func runnerIDForHost(host *DatabaseRunnerHost) string {
	if host == nil {
		return ""
	}
	switch host.RunnerType {
	case DatabaseRunnerTypeLocal:
		return fmt.Sprintf("local:%d", host.ID)
	case DatabaseRunnerTypeAgent:
		return fmt.Sprintf("agent:%s", host.Host)
	default:
		return fmt.Sprintf("ssh:%s:%d", host.Host, host.Port)
	}
}

func normalizeRunnerType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DatabaseRunnerTypeLocal:
		return DatabaseRunnerTypeLocal
	case DatabaseRunnerTypeAgent:
		return DatabaseRunnerTypeAgent
	default:
		return DatabaseRunnerTypeSSH
	}
}

func normalizeRunnerMaxConcurrentJobs(value int) int {
	if value <= 0 {
		return 1
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeRunnerTimeoutMinutes(value int) int {
	if value <= 0 {
		return defaultRunnerTimeoutMinutes
	}
	if value > maxRunnerTimeoutMinutes {
		return maxRunnerTimeoutMinutes
	}
	return value
}

func toRunnerHostVO(item *DatabaseRunnerHost) *DatabaseRunnerHostVO {
	if item == nil {
		return nil
	}
	return &DatabaseRunnerHostVO{
		ID:                item.ID,
		Name:              item.Name,
		RunnerType:        item.RunnerType,
		RunnerTypeText:    RunnerTypeText(item.RunnerType),
		Host:              item.Host,
		Port:              item.Port,
		CredentialID:      item.CredentialID,
		WorkDir:           item.WorkDir,
		StorageMountPath:  item.StorageMountPath,
		MaxConcurrentJobs: item.MaxConcurrentJobs,
		CPULimit:          item.CPULimit,
		IOLimit:           item.IOLimit,
		BandwidthLimit:    item.BandwidthLimit,
		TimeoutMinutes:    item.TimeoutMinutes,
		Enabled:           item.Enabled,
		Status:            item.Status,
		StatusText:        RunnerHostStatusText(item.Status),
		LastHeartbeatAt:   formatTime(item.LastHeartbeatAt),
		LastTestAt:        formatTime(item.LastTestAt),
		LastError:         item.LastError,
		ConfigJSON:        item.ConfigJSON,
		CreatedAt:         item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:         item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (uc *UseCase) toRunnerJobVO(ctx context.Context, item *DatabaseRunnerJob) *DatabaseRunnerJobVO {
	if item == nil {
		return nil
	}
	runnerHostName := ""
	if uc != nil && uc.runnerHostRepo != nil && item.RunnerHostID > 0 {
		if host, err := uc.runnerHostRepo.GetByID(ctx, item.RunnerHostID); err == nil && host != nil {
			runnerHostName = host.Name
		}
	}
	return &DatabaseRunnerJobVO{
		ID:               item.ID,
		JobType:          item.JobType,
		JobTypeText:      RunnerJobTypeText(item.JobType),
		RunnerHostID:     item.RunnerHostID,
		RunnerHostName:   runnerHostName,
		RunnerID:         item.RunnerID,
		SourceInstanceID: item.SourceInstanceID,
		TargetInstanceID: item.TargetInstanceID,
		Status:           item.Status,
		StatusText:       RunnerJobStatusText(item.Status),
		AllowedCommand:   item.AllowedCommand,
		CommandSummary:   item.CommandSummary,
		WorkDir:          item.WorkDir,
		LogPath:          item.LogPath,
		ExitCode:         item.ExitCode,
		OperatorID:       item.OperatorID,
		OperatorName:     item.OperatorName,
		RequestJSON:      item.RequestJSON,
		ResultJSON:       item.ResultJSON,
		HeartbeatAt:      formatTime(item.HeartbeatAt),
		StartedAt:        formatTime(item.StartedAt),
		FinishedAt:       formatTime(item.FinishedAt),
		DurationMs:       item.DurationMs,
		ErrorMessage:     item.ErrorMessage,
		CreatedAt:        item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func RunnerTypeText(value string) string {
	switch normalizeRunnerType(value) {
	case DatabaseRunnerTypeLocal:
		return "本地 Runner"
	case DatabaseRunnerTypeAgent:
		return "Agent Runner"
	default:
		return "SSH Runner"
	}
}

func RunnerHostStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseRunnerHostStatusOnline:
		return "在线"
	case DatabaseRunnerHostStatusFailed:
		return "失败"
	case DatabaseRunnerHostStatusDisabled:
		return "已禁用"
	default:
		return "待测试"
	}
}

func RunnerJobTypeText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseRunnerJobTypePhysicalBackup:
		return "物理备份"
	case DatabaseRunnerJobTypeBinlogArchive:
		return "binlog 归档"
	case DatabaseRunnerJobTypePhysicalRestore:
		return "物理恢复"
	case DatabaseRunnerJobTypeRestoreValidate:
		return "恢复校验"
	default:
		return "Runner 探测"
	}
}

func RunnerJobStatusText(value string) string {
	switch strings.TrimSpace(value) {
	case DatabaseRunnerJobStatusRunning:
		return "运行中"
	case DatabaseRunnerJobStatusSuccess:
		return "成功"
	case DatabaseRunnerJobStatusFailed:
		return "失败"
	case DatabaseRunnerJobStatusCancelled:
		return "已取消"
	default:
		return "排队中"
	}
}
