package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type DatabaseRunnerAgentLifecycleRequest struct {
	ServerURL               string `json:"serverUrl" binding:"omitempty,max=500"`
	InstallPath             string `json:"installPath" binding:"omitempty,max=500"`
	ServiceName             string `json:"serviceName" binding:"omitempty,max=120"`
	ListenAddr              string `json:"listenAddr" binding:"omitempty,max=120"`
	IntervalSeconds         int    `json:"intervalSeconds" binding:"omitempty,min=5,max=3600"`
	DatabaseArchiverEnabled bool   `json:"databaseArchiverEnabled"`
	DryRun                  bool   `json:"dryRun"`
	RegenerateAuth          bool   `json:"regenerateAuth"`
	Confirm                 bool   `json:"confirm"`
	Reason                  string `json:"reason" binding:"omitempty,max=1000"`
}

type DatabaseRunnerAgentConfigSnippetVO struct {
	RunnerHostID     uint   `json:"runnerHostId"`
	RunnerID         string `json:"runnerId"`
	RunnerAuthSha256 string `json:"runnerAuthSha256"`
	ConfigJSON       string `json:"configJson"`
	ServiceName      string `json:"serviceName"`
	InstallPath      string `json:"installPath"`
	GeneratedAt      string `json:"generatedAt"`
	Message          string `json:"message"`
}

type DatabaseRunnerAgentLogsVO struct {
	RunnerHostID uint   `json:"runnerHostId"`
	RunnerID     string `json:"runnerId"`
	ServiceName  string `json:"serviceName"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	FetchedAt    string `json:"fetchedAt"`
}

func (uc *UseCase) GenerateRunnerAgentConfigSnippet(ctx context.Context, runnerHostID uint, req *DatabaseRunnerAgentLifecycleRequest) (*DatabaseRunnerAgentConfigSnippetVO, error) {
	host, err := uc.getRunnerHostForAgentLifecycle(ctx, runnerHostID)
	if err != nil {
		return nil, err
	}
	resolved := normalizeRunnerAgentLifecycleRequest(req, host)
	auth, authHash, err := generateRunnerAgentAuthPair()
	if err != nil {
		return nil, err
	}
	if err := uc.persistRunnerAgentAuthHash(ctx, host, authHash); err != nil {
		return nil, err
	}
	configJSON := buildRunnerAgentConfigJSON(host, resolved, auth)
	return &DatabaseRunnerAgentConfigSnippetVO{
		RunnerHostID:     host.ID,
		RunnerID:         runnerIDForHost(host),
		RunnerAuthSha256: authHash,
		ConfigJSON:       configJSON,
		ServiceName:      resolved.ServiceName,
		InstallPath:      resolved.InstallPath,
		GeneratedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Message:          "已生成一次性 Runner Agent 配置，并只在 Runner 主机配置中保存 runnerAuthSha256",
	}, nil
}

func (uc *UseCase) InstallRunnerAgent(ctx context.Context, runnerHostID uint, req *DatabaseRunnerAgentLifecycleRequest, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	return uc.startRunnerAgentLifecycleJob(ctx, runnerHostID, req, operator, DatabaseRunnerJobTypeAgentInstall, DatabaseRunnerAllowedCommandAgentInstall, "Runner Agent 安装")
}

func (uc *UseCase) UpgradeRunnerAgent(ctx context.Context, runnerHostID uint, req *DatabaseRunnerAgentLifecycleRequest, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	return uc.startRunnerAgentLifecycleJob(ctx, runnerHostID, req, operator, DatabaseRunnerJobTypeAgentUpgrade, DatabaseRunnerAllowedCommandAgentUpgrade, "Runner Agent 升级")
}

func (uc *UseCase) RestartRunnerAgent(ctx context.Context, runnerHostID uint, req *DatabaseRunnerAgentLifecycleRequest, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	return uc.startRunnerAgentLifecycleJob(ctx, runnerHostID, req, operator, DatabaseRunnerJobTypeAgentRestart, DatabaseRunnerAllowedCommandAgentRestart, "Runner Agent 重启")
}

func (uc *UseCase) GetRunnerAgentLogs(ctx context.Context, runnerHostID uint, lines int) (*DatabaseRunnerAgentLogsVO, error) {
	host, err := uc.getRunnerHostForAgentLifecycle(ctx, runnerHostID)
	if err != nil {
		return nil, err
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("Runner Agent 日志读取仅支持 SSH Runner")
	}
	if uc.credentialResolver == nil {
		return nil, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}
	serviceName := runnerAgentServiceName(host, nil)
	script := strings.Join([]string{
		"set +e",
		"SERVICE_NAME=" + shellSingleQuote(serviceName),
		fmt.Sprintf("LINES=%d", lines),
		`if command -v journalctl >/dev/null 2>&1; then journalctl -u "$SERVICE_NAME" -n "$LINES" --no-pager 2>&1; fi`,
		`LOG_FILE="/var/log/${SERVICE_NAME}.log"; [ -f "$LOG_FILE" ] && { echo "--- $LOG_FILE ---"; tail -n "$LINES" "$LOG_FILE"; }`,
		"exit 0",
	}, "\n")
	stdout, stderr, exitCode, err := executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
	if err != nil {
		return nil, err
	}
	return &DatabaseRunnerAgentLogsVO{
		RunnerHostID: host.ID,
		RunnerID:     runnerIDForHost(host),
		ServiceName:  serviceName,
		Stdout:       trimText(stdout, maxRunnerOutputLength),
		Stderr:       trimText(stderr, maxRunnerOutputLength),
		ExitCode:     exitCode,
		FetchedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) startRunnerAgentLifecycleJob(ctx context.Context, runnerHostID uint, req *DatabaseRunnerAgentLifecycleRequest, operator QueryOperator, jobType, allowedCommand, summary string) (*DatabaseRunnerJobVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil {
		return nil, fmt.Errorf("Runner 仓库未配置")
	}
	host, err := uc.getRunnerHostForAgentLifecycle(ctx, runnerHostID)
	if err != nil {
		return nil, err
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return nil, fmt.Errorf("%s 仅支持 SSH Runner", summary)
	}
	resolved := normalizeRunnerAgentLifecycleRequest(req, host)
	if !resolved.Confirm {
		return nil, fmt.Errorf("请确认 Runner Agent 生命周期操作")
	}
	if strings.TrimSpace(resolved.Reason) == "" {
		return nil, fmt.Errorf("操作原因不能为空")
	}
	auth := ""
	authHash := runnerAgentAuthHash(host.ConfigJSON)
	if jobType == DatabaseRunnerJobTypeAgentInstall || resolved.RegenerateAuth || authHash == "" {
		var err error
		auth, authHash, err = generateRunnerAgentAuthPair()
		if err != nil {
			return nil, err
		}
		if !resolved.DryRun {
			if err := uc.persistRunnerAgentAuthHash(ctx, host, authHash); err != nil {
				return nil, err
			}
		}
	}
	requestJSON, _ := json.Marshal(map[string]any{
		"runnerHostId":              host.ID,
		"runnerId":                  runnerIDForHost(host),
		"serverUrl":                 resolved.ServerURL,
		"installPath":               resolved.InstallPath,
		"serviceName":               resolved.ServiceName,
		"listenAddr":                resolved.ListenAddr,
		"intervalSeconds":           resolved.IntervalSeconds,
		"databaseArchiverEnabled":   resolved.DatabaseArchiverEnabled,
		"dryRun":                    resolved.DryRun,
		"regenerateAuth":            resolved.RegenerateAuth,
		"runnerAuthSha256Updated":   auth != "",
		"reason":                    resolved.Reason,
		"allowedCommand":            allowedCommand,
		"plaintextAuthNotPersisted": true,
	})
	job := &DatabaseRunnerJob{
		JobType:        jobType,
		RunnerHostID:   host.ID,
		RunnerID:       runnerIDForHost(host),
		Status:         DatabaseRunnerJobStatusQueued,
		AllowedCommand: allowedCommand,
		CommandSummary: trimText(fmt.Sprintf("%s: %s / dryRun=%t", summary, resolved.ServiceName, resolved.DryRun), 500),
		WorkDir:        host.WorkDir,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 120),
		RequestJSON:    trimText(string(requestJSON), maxRunnerJSONLength),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeRunnerAgentLifecycleJob(context.Background(), host.ID, job.ID, resolved, auth)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) executeRunnerAgentLifecycleJob(ctx context.Context, runnerHostID, jobID uint, req *DatabaseRunnerAgentLifecycleRequest, auth string) {
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
	stdout, stderr, exitCode, err := uc.runRunnerAgentLifecycleCommand(ctx, host, job, req, auth)
	finished := time.Now()
	resultJSON, _ := json.Marshal(map[string]any{
		"runnerHostId": host.ID,
		"runnerId":     runnerIDForHost(host),
		"serviceName":  req.ServiceName,
		"installPath":  req.InstallPath,
		"stdout":       trimText(stdout, maxRunnerOutputLength),
		"stderr":       trimText(stderr, maxRunnerOutputLength),
		"exitCode":     exitCode,
		"startedAt":    started.Format("2006-01-02 15:04:05"),
		"finishedAt":   finished.Format("2006-01-02 15:04:05"),
	})
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.ExitCode = exitCode
	job.FinishedAt = &finished
	job.DurationMs = finished.Sub(started).Milliseconds()
	job.HeartbeatAt = &finished
	host.LastTestAt = &finished
	if err != nil || exitCode != 0 {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(firstNonEmpty(errorString(err), stderr, "Runner Agent 生命周期命令执行失败"), 1000)
		host.Status = DatabaseRunnerHostStatusFailed
		host.LastError = job.ErrorMessage
	} else {
		job.Status = DatabaseRunnerJobStatusSuccess
		job.ErrorMessage = ""
		host.Status = DatabaseRunnerHostStatusOnline
		host.LastError = ""
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.runnerHostRepo.Update(ctx, host)
}

func (uc *UseCase) runRunnerAgentLifecycleCommand(ctx context.Context, host *DatabaseRunnerHost, job *DatabaseRunnerJob, req *DatabaseRunnerAgentLifecycleRequest, auth string) (string, string, int, error) {
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	binaryRemotePath := ""
	configJSON := ""
	if job.JobType == DatabaseRunnerJobTypeAgentInstall || job.JobType == DatabaseRunnerJobTypeAgentUpgrade {
		if !req.DryRun {
			localBinary, err := findRunnerAgentBundlePath(ctx, uc, host)
			if err != nil {
				return "", "", 1, err
			}
			workDir := normalizeRunnerWorkDir(host.WorkDir)
			binaryRemotePath = path.Join(workDir, "agent-lifecycle", fmt.Sprintf("job-%d", job.ID), "opshub-agent")
			mkdirScript := fmt.Sprintf("mkdir -p %s", shellSingleQuote(path.Dir(binaryRemotePath)))
			if _, _, _, err := executeSSHRunnerScript(ctx, host.Host, host.Port, credential, mkdirScript, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute); err != nil {
				return "", "", 1, err
			}
			if err := uploadSSHRunnerFile(ctx, host.Host, host.Port, credential, localBinary, binaryRemotePath, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute); err != nil {
				return "", "", 1, fmt.Errorf("上传 Runner Agent 二进制失败: %w", err)
			}
		}
		if auth != "" {
			configJSON = buildRunnerAgentConfigJSON(host, req, auth)
		}
	}
	script := buildRunnerAgentLifecycleScript(host, job.JobType, req, binaryRemotePath, configJSON)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func buildRunnerAgentLifecycleScript(host *DatabaseRunnerHost, jobType string, req *DatabaseRunnerAgentLifecycleRequest, binaryRemotePath, configJSON string) string {
	installPath := req.InstallPath
	serviceName := req.ServiceName
	configB64 := base64.StdEncoding.EncodeToString([]byte(configJSON))
	dryRunFlag := "0"
	if req.DryRun {
		dryRunFlag = "1"
	}
	lines := []string{
		"set -eu",
		"DRY_RUN=" + shellSingleQuote(dryRunFlag),
		"INSTALL_PATH=" + shellSingleQuote(installPath),
		"SERVICE_NAME=" + shellSingleQuote(serviceName),
		"BINARY_SOURCE=" + shellSingleQuote(binaryRemotePath),
		`CONFIG_PATH="$INSTALL_PATH/config.json"`,
		`BINARY_PATH="$INSTALL_PATH/opshub-agent"`,
		`LOG_FILE="/var/log/${SERVICE_NAME}.log"`,
		`if [ "$(id -u 2>/dev/null)" != "0" ]; then SUDO="sudo"; else SUDO=""; fi`,
		`if [ "$(id -u 2>/dev/null)" != "0" ]; then sudo -n true >/dev/null 2>&1 || { echo "sudo unavailable"; exit 12; }; fi`,
		`echo "Runner Agent lifecycle: $SERVICE_NAME / $INSTALL_PATH / job=` + jobType + `"`,
	}
	if jobType == DatabaseRunnerJobTypeAgentInstall || jobType == DatabaseRunnerJobTypeAgentUpgrade {
		lines = append(lines,
			`if [ "$DRY_RUN" = "1" ]; then echo "dry-run: skip install/upgrade"; exit 0; fi`,
			`[ -f "$BINARY_SOURCE" ] || { echo "agent binary not uploaded: $BINARY_SOURCE"; exit 20; }`,
			`$SUDO mkdir -p "$INSTALL_PATH"`,
			`$SUDO cp "$BINARY_SOURCE" "$BINARY_PATH"`,
			`$SUDO chmod 755 "$BINARY_PATH"`,
		)
		if configJSON != "" {
			lines = append(lines,
				`TMP_CONFIG="$(mktemp)"`,
				"base64 -d > \"$TMP_CONFIG\" <<'OPSHUB_AGENT_CONFIG_B64'",
				configB64,
				"OPSHUB_AGENT_CONFIG_B64",
				`$SUDO cp "$TMP_CONFIG" "$CONFIG_PATH"`,
				`rm -f "$TMP_CONFIG"`,
				`$SUDO chmod 600 "$CONFIG_PATH"`,
			)
		}
		lines = append(lines, buildRunnerAgentStartShellLines()...)
	} else {
		lines = append(lines,
			`if [ "$DRY_RUN" = "1" ]; then echo "dry-run: skip restart"; exit 0; fi`,
		)
		lines = append(lines, buildRunnerAgentStartShellLines()...)
	}
	return strings.Join(lines, "\n")
}

func buildRunnerAgentStartShellLines() []string {
	return []string{
		`if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then`,
		`  SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"`,
		`  TMP_SERVICE="$(mktemp)"`,
		`  cat > "$TMP_SERVICE" <<EOF`,
		`[Unit]`,
		`Description=OpsHub Runner Agent`,
		`After=network-online.target`,
		`Wants=network-online.target`,
		``,
		`[Service]`,
		`Type=simple`,
		`ExecStart=${BINARY_PATH} --config ${CONFIG_PATH}`,
		`Restart=always`,
		`RestartSec=5`,
		``,
		`[Install]`,
		`WantedBy=multi-user.target`,
		`EOF`,
		`  $SUDO cp "$TMP_SERVICE" "$SERVICE_FILE"`,
		`  rm -f "$TMP_SERVICE"`,
		`  $SUDO systemctl daemon-reload`,
		`  $SUDO systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true`,
		`  $SUDO systemctl restart "$SERVICE_NAME"`,
		`  sleep 2`,
		`  $SUDO systemctl --no-pager --full status "$SERVICE_NAME" || true`,
		`else`,
		`  $SUDO pkill -f "${BINARY_PATH} --config ${CONFIG_PATH}" >/dev/null 2>&1 || true`,
		`  $SUDO sh -c "nohup '${BINARY_PATH}' --config '${CONFIG_PATH}' >> '${LOG_FILE}' 2>&1 &"`,
		`  sleep 1`,
		`  pgrep -af "${BINARY_PATH} --config ${CONFIG_PATH}" || true`,
		`fi`,
	}
}

func (uc *UseCase) getRunnerHostForAgentLifecycle(ctx context.Context, runnerHostID uint) (*DatabaseRunnerHost, error) {
	if uc.runnerHostRepo == nil {
		return nil, fmt.Errorf("Runner 主机仓库未配置")
	}
	if runnerHostID == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil || host == nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	return host, nil
}

func normalizeRunnerAgentLifecycleRequest(req *DatabaseRunnerAgentLifecycleRequest, host *DatabaseRunnerHost) *DatabaseRunnerAgentLifecycleRequest {
	if req == nil {
		req = &DatabaseRunnerAgentLifecycleRequest{}
	}
	installPath := strings.TrimSpace(req.InstallPath)
	if installPath == "" {
		installPath = "/opt/opshub-agent"
	}
	serviceName := strings.TrimSpace(req.ServiceName)
	if serviceName == "" {
		serviceName = runnerAgentServiceName(host, req)
	}
	listenAddr := strings.TrimSpace(req.ListenAddr)
	if listenAddr == "" {
		listenAddr = "0.0.0.0:19100"
	}
	intervalSeconds := req.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	return &DatabaseRunnerAgentLifecycleRequest{
		ServerURL:               strings.TrimRight(strings.TrimSpace(req.ServerURL), "/"),
		InstallPath:             installPath,
		ServiceName:             serviceName,
		ListenAddr:              listenAddr,
		IntervalSeconds:         intervalSeconds,
		DatabaseArchiverEnabled: req.DatabaseArchiverEnabled,
		DryRun:                  req.DryRun,
		RegenerateAuth:          req.RegenerateAuth,
		Confirm:                 req.Confirm,
		Reason:                  trimText(strings.TrimSpace(req.Reason), 1000),
	}
}

func runnerAgentServiceName(host *DatabaseRunnerHost, req *DatabaseRunnerAgentLifecycleRequest) string {
	if req != nil && strings.TrimSpace(req.ServiceName) != "" {
		return strings.TrimSpace(req.ServiceName)
	}
	if host != nil && host.ID > 0 {
		return fmt.Sprintf("opshub-agent-runner-%d", host.ID)
	}
	return "opshub-agent-runner"
}

func buildRunnerAgentConfigJSON(host *DatabaseRunnerHost, req *DatabaseRunnerAgentLifecycleRequest, auth string) string {
	serverURL := strings.TrimRight(strings.TrimSpace(req.ServerURL), "/")
	if serverURL == "" {
		serverURL = "http://<opshub-backend>:8080"
	}
	workDir := firstNonEmpty(host.WorkDir, defaultRunnerWorkDir)
	storageRoot := firstNonEmpty(host.StorageMountPath, path.Join(workDir, "database-archives"))
	payload := map[string]any{
		"agentId":         runnerIDForHost(host),
		"accessToken":     "<asset_agent_token_optional>",
		"reportUrl":       serverURL + "/api/v1/public/agents/report",
		"intervalSeconds": req.IntervalSeconds,
		"listenAddr":      req.ListenAddr,
		"serviceName":     req.ServiceName,
		"databaseArchiver": map[string]any{
			"enabled":                       req.DatabaseArchiverEnabled,
			"baseUrl":                       serverURL,
			"runnerId":                      runnerIDForHost(host),
			"runnerAuth":                    auth,
			"intervalSeconds":               30,
			"leaseTtlSeconds":               90,
			"maxFilesPerLoop":               5,
			"maxConcurrentStreams":          2,
			"failureBackoffSeconds":         30,
			"maxFailureBackoffSeconds":      300,
			"stopNeverEnabled":              true,
			"spoolResumeEnabled":            false,
			"uploadRetryEnabled":            true,
			"uploadRetryBaseSeconds":        60,
			"uploadRetryMaxSeconds":         3600,
			"uploadRetryMaxAttempts":        0,
			"uploadBandwidthBytesPerSecond": 0,
			"streamingLogMaxBytes":          10485760,
			"streamingLogMaxFiles":          5,
			"includeCurrent":                false,
			"workDir":                       workDir,
			"storageRoot":                   storageRoot,
			"storage": map[string]any{
				"type":               "local",
				"endpoint":           "http://<minio-host>:9000",
				"bucket":             "opshub-backup",
				"region":             "us-east-1",
				"pathPrefix":         "opshub/database-archives",
				"stagingPrefix":      "opshub/database-archives/.staging",
				"accessKey":          "<local_secret>",
				"secretKey":          "<local_secret>",
				"useSsl":             false,
				"usePathStyle":       true,
				"insecureSkipVerify": false,
			},
			"mysqlBinlogPath": "",
			"credentials": []map[string]any{
				{
					"streamId":   0,
					"instanceId": 0,
					"host":       "<mysql-host>",
					"port":       3306,
					"username":   "<mysql_replication_user>",
					"password":   "<local_secret>",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

func (uc *UseCase) persistRunnerAgentAuthHash(ctx context.Context, host *DatabaseRunnerHost, authHash string) error {
	if host == nil || uc.runnerHostRepo == nil {
		return fmt.Errorf("Runner 主机仓库未配置")
	}
	var data map[string]any
	if strings.TrimSpace(host.ConfigJSON) != "" {
		_ = json.Unmarshal([]byte(host.ConfigJSON), &data)
	}
	if data == nil {
		data = map[string]any{}
	}
	data["runnerAuthSha256"] = authHash
	data["runnerAgentManagedBy"] = "opshub"
	data["runnerAgentUpdatedAt"] = time.Now().Format("2006-01-02 15:04:05")
	encoded, _ := json.Marshal(data)
	host.ConfigJSON = trimText(string(encoded), 4000)
	return uc.runnerHostRepo.Update(ctx, host)
}

func generateRunnerAgentAuthPair() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	auth := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(auth))
	return auth, hex.EncodeToString(sum[:]), nil
}

func findRunnerAgentBundlePath(ctx context.Context, uc *UseCase, host *DatabaseRunnerHost) (string, error) {
	arch := runtime.GOARCH
	if uc != nil && uc.runnerToolProfileRepo != nil && host != nil {
		if profile, err := uc.runnerToolProfileRepo.GetByRunnerHostID(ctx, host.ID); err == nil && profile != nil {
			arch = normalizeAgentBundleArch(profile.Arch)
		}
	}
	fileName := "opshub-agent-linux-" + arch
	candidates := []string{
		filepath.Join("agent-bundles", fileName),
		filepath.Join("/app/agent-bundles", fileName),
		filepath.Join("/work/opshub/opshub-main/agent-bundles", fileName),
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 Runner Agent 二进制包：%s", fileName)
}

func normalizeAgentBundleArch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "arm64", "aarch64":
		return "arm64"
	default:
		return "amd64"
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
