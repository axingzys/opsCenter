package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	maxRunnerToolJSONLength = 60000
	runnerToolProbeMarker   = "OPSHUB_PROFILE_FIELD "
)

type RunnerToolInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

type RunnerToolCapabilitySummary struct {
	Available []string `json:"available"`
	Missing   []string `json:"missing"`
	Warnings  []string `json:"warnings"`
}

type RunnerToolCompatibilitySummary struct {
	OS        string   `json:"os"`
	Arch      string   `json:"arch"`
	Supported []string `json:"supported"`
	Warnings  []string `json:"warnings"`
}

type DatabaseRunnerToolProfileVO struct {
	ID             uint                           `json:"id"`
	RunnerHostID   uint                           `json:"runnerHostId"`
	RunnerName     string                         `json:"runnerName"`
	OSFamily       string                         `json:"osFamily"`
	OSVersion      string                         `json:"osVersion"`
	OSPrettyName   string                         `json:"osPrettyName"`
	Arch           string                         `json:"arch"`
	PackageManager string                         `json:"packageManager"`
	IsRoot         bool                           `json:"isRoot"`
	HasSudo        bool                           `json:"hasSudo"`
	HasSystemd     bool                           `json:"hasSystemd"`
	HasDocker      bool                           `json:"hasDocker"`
	NetworkAccess  string                         `json:"networkAccess"`
	Tools          map[string]RunnerToolInfo      `json:"tools"`
	Capability     RunnerToolCapabilitySummary    `json:"capability"`
	Compatibility  RunnerToolCompatibilitySummary `json:"compatibility"`
	LastProbeAt    string                         `json:"lastProbeAt"`
	LastStatus     string                         `json:"lastStatus"`
	LastError      string                         `json:"lastError"`
	CreatedAt      string                         `json:"createdAt"`
	UpdatedAt      string                         `json:"updatedAt"`
}

type DatabaseRunnerToolInstallScriptRequest struct {
	Profiles         []string          `json:"profiles"`
	TargetDBVersions map[string]string `json:"targetDbVersions"`
	InstallMode      string            `json:"installMode"`
	ExecutionMode    string            `json:"executionMode"`
	ToolImage        string            `json:"toolImage"`
	ToolImageDigest  string            `json:"toolImageDigest"`
	DatadirMount     string            `json:"datadirMount"`
	WorkdirMount     string            `json:"workdirMount"`
	NetworkMode      string            `json:"networkMode"`
	ReadOnlyDatadir  bool              `json:"readOnlyDatadir"`
	DryRun           bool              `json:"dryRun"`
}

type DatabaseRunnerToolInstallScriptVO struct {
	RunnerHostID     uint              `json:"runnerHostId"`
	RunnerHostName   string            `json:"runnerHostName"`
	OSFamily         string            `json:"osFamily"`
	OSVersion        string            `json:"osVersion"`
	OSPrettyName     string            `json:"osPrettyName"`
	Arch             string            `json:"arch"`
	PackageManager   string            `json:"packageManager"`
	Profiles         []string          `json:"profiles"`
	TargetDBVersions map[string]string `json:"targetDbVersions"`
	InstallMode      string            `json:"installMode"`
	ExecutionMode    string            `json:"executionMode"`
	ToolImage        string            `json:"toolImage"`
	ToolImageDigest  string            `json:"toolImageDigest"`
	DatadirMount     string            `json:"datadirMount"`
	WorkdirMount     string            `json:"workdirMount"`
	NetworkMode      string            `json:"networkMode"`
	ReadOnlyDatadir  bool              `json:"readOnlyDatadir"`
	DryRun           bool              `json:"dryRun"`
	Warnings         []string          `json:"warnings"`
	Unsupported      []string          `json:"unsupported"`
	Script           string            `json:"script"`
	GeneratedAt      string            `json:"generatedAt"`
}

type runnerToolProbeResult struct {
	RunnerHostID uint   `json:"runnerHostId"`
	RunnerID     string `json:"runnerId"`
	ProfileID    uint   `json:"profileId"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
}

func (uc *UseCase) GetRunnerToolProfile(ctx context.Context, runnerHostID uint) (*DatabaseRunnerToolProfileVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerToolProfileRepo == nil {
		return nil, fmt.Errorf("Runner 工具画像仓库未配置")
	}
	if runnerHostID == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	profile, err := uc.runnerToolProfileRepo.GetByRunnerHostID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 工具画像不存在，请先执行工具巡检")
	}
	return uc.toRunnerToolProfileVO(host, profile), nil
}

func (uc *UseCase) ProbeRunnerTools(ctx context.Context, runnerHostID uint, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.runnerToolProfileRepo == nil {
		return nil, fmt.Errorf("Runner 工具巡检仓库未配置")
	}
	if runnerHostID == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	if !host.Enabled || host.Status == DatabaseRunnerHostStatusDisabled {
		return nil, fmt.Errorf("Runner 主机已禁用")
	}
	job := &DatabaseRunnerJob{
		JobType:        DatabaseRunnerJobTypeToolProbe,
		RunnerHostID:   host.ID,
		RunnerID:       runnerIDForHost(host),
		Status:         DatabaseRunnerJobStatusQueued,
		AllowedCommand: DatabaseRunnerAllowedCommandToolProbe,
		CommandSummary: "Runner 数据库备份工具巡检",
		WorkDir:        host.WorkDir,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 120),
		RequestJSON:    runnerToolProbeRequestJSON(host, operator),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeRunnerToolProbe(context.Background(), host.ID, job.ID)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) GenerateRunnerToolInstallScript(ctx context.Context, runnerHostID uint, req *DatabaseRunnerToolInstallScriptRequest) (*DatabaseRunnerToolInstallScriptVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerToolProfileRepo == nil {
		return nil, fmt.Errorf("Runner 工具画像仓库未配置")
	}
	if runnerHostID == 0 {
		return nil, fmt.Errorf("Runner 主机ID不能为空")
	}
	host, err := uc.runnerHostRepo.GetByID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("Runner 主机不存在")
	}
	profile, err := uc.runnerToolProfileRepo.GetByRunnerHostID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("请先对 Runner 主机执行工具巡检，再生成安装脚本")
	}
	return buildRunnerToolInstallScript(host, profile, req), nil
}

func (uc *UseCase) executeRunnerToolProbe(ctx context.Context, runnerHostID, jobID uint) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.runnerToolProfileRepo == nil {
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

	stdout, stderr, exitCode, err := uc.runRunnerToolProbeCommand(ctx, host)
	finished := time.Now()
	profile, profileErr := buildRunnerToolProfileFromOutput(host.ID, stdout, finished, err)
	if err == nil && profileErr != nil {
		err = profileErr
	}
	applyRunnerToolProbeResult(host, job, profile, started, finished, stdout, stderr, exitCode, err)
	if profile != nil {
		_ = uc.runnerToolProfileRepo.UpsertByRunnerHostID(ctx, profile)
	}
	_ = uc.runnerJobRepo.Update(ctx, job)
	_ = uc.runnerHostRepo.Update(ctx, host)
}

func (uc *UseCase) runRunnerToolProbeCommand(ctx context.Context, host *DatabaseRunnerHost) (string, string, int, error) {
	if host == nil {
		return "", "", 1, fmt.Errorf("Runner 主机不能为空")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return "", "", 1, fmt.Errorf("当前工具巡检仅支持 SSH Runner")
	}
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, buildRunnerToolProbeScript(), time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func buildRunnerToolProbeScript() string {
	tools := []string{
		"mysql", "mysqlbinlog", "mariadb", "mariadb-binlog", "xtrabackup", "mariabackup", "mariadb-backup",
		"psql", "pg_basebackup", "pg_receivewal", "pg_combinebackup", "pg_verifybackup", "barman", "wal-g",
		"docker", "tar", "sha256sum", "zstd", "gzip", "rsync", "curl", "wget",
	}
	lines := []string{
		"set +e",
		`emit() { key="$1"; shift; printf 'OPSHUB_PROFILE_FIELD %s=%s\n' "$key" "$*"; }`,
		`os_id=""`,
		`os_version=""`,
		`os_pretty=""`,
		`[ -r /etc/os-release ] && . /etc/os-release && os_id="${ID:-}" && os_version="${VERSION_ID:-}" && os_pretty="${PRETTY_NAME:-}"`,
		`emit os_family "$os_id"`,
		`emit os_version "$os_version"`,
		`emit os_pretty_name "$os_pretty"`,
		`emit arch "$(uname -m 2>/dev/null)"`,
		`pm="unknown"; command -v apt-get >/dev/null 2>&1 && pm="apt"; command -v dnf >/dev/null 2>&1 && pm="dnf"; command -v yum >/dev/null 2>&1 && pm="yum"; emit package_manager "$pm"`,
		`[ "$(id -u 2>/dev/null)" = "0" ] && emit is_root true || emit is_root false`,
		`sudo -n true >/dev/null 2>&1 && emit has_sudo true || emit has_sudo false`,
		`command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ] && emit has_systemd true || emit has_systemd false`,
		`command -v docker >/dev/null 2>&1 && emit has_docker true || emit has_docker false`,
		`emit network_access unknown`,
		`probe_tool() { name="$1"; path="$(command -v "$name" 2>/dev/null || true)"; if [ -n "$path" ]; then version="$("$path" --version 2>&1 | head -n 1 || true)"; emit "tool.$name.path" "$path"; emit "tool.$name.version" "$version"; else emit "tool.$name.path" ""; emit "tool.$name.version" ""; fi; }`,
	}
	for _, tool := range tools {
		lines = append(lines, "probe_tool "+shellSingleQuote(tool))
	}
	lines = append(lines, "exit 0")
	return strings.Join(lines, "\n")
}

func buildRunnerToolProfileFromOutput(runnerHostID uint, stdout string, probedAt time.Time, runErr error) (*DatabaseRunnerToolProfile, error) {
	fields := parseRunnerToolProbeFields(stdout)
	if len(fields) == 0 && runErr == nil {
		return nil, fmt.Errorf("未识别到工具巡检输出")
	}
	tools := parseRunnerTools(fields)
	manifestJSON := runnerToolJSON(map[string]any{
		"tools":       tools,
		"generatedAt": probedAt.Format("2006-01-02 15:04:05"),
	})
	capability := buildRunnerToolCapabilitySummary(tools)
	compatibility := buildRunnerToolCompatibilitySummary(fields, tools)
	capabilityJSON := runnerToolJSON(capability)
	compatibilityJSON := runnerToolJSON(compatibility)
	status := DatabaseRunnerJobStatusSuccess
	lastError := ""
	if runErr != nil {
		status = DatabaseRunnerJobStatusFailed
		lastError = trimText(runErr.Error(), 1000)
	}
	return &DatabaseRunnerToolProfile{
		RunnerHostID:      runnerHostID,
		OSFamily:          trimText(fields["os_family"], 80),
		OSVersion:         trimText(fields["os_version"], 80),
		OSPrettyName:      trimText(fields["os_pretty_name"], 200),
		Arch:              trimText(fields["arch"], 80),
		PackageManager:    trimText(fields["package_manager"], 40),
		IsRoot:            parseBoolText(fields["is_root"]),
		HasSudo:           parseBoolText(fields["has_sudo"]),
		HasSystemd:        parseBoolText(fields["has_systemd"]),
		HasDocker:         parseBoolText(fields["has_docker"]),
		NetworkAccess:     trimText(defaultIfBlank(fields["network_access"], "unknown"), 40),
		ToolManifestJSON:  trimText(manifestJSON, maxRunnerToolJSONLength),
		CapabilityJSON:    trimText(capabilityJSON, maxRunnerToolJSONLength),
		CompatibilityJSON: trimText(compatibilityJSON, maxRunnerToolJSONLength),
		LastProbeAt:       &probedAt,
		LastProbeStatus:   status,
		LastError:         lastError,
	}, nil
}

func parseRunnerToolProbeFields(stdout string) map[string]string {
	fields := make(map[string]string)
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, runnerToolProbeMarker) {
			continue
		}
		body := strings.TrimPrefix(line, runnerToolProbeMarker)
		key, value, ok := strings.Cut(body, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return fields
}

func parseRunnerTools(fields map[string]string) map[string]RunnerToolInfo {
	tools := make(map[string]RunnerToolInfo)
	for key, value := range fields {
		if !strings.HasPrefix(key, "tool.") || !strings.HasSuffix(key, ".path") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "tool."), ".path")
		path := strings.TrimSpace(value)
		version := strings.TrimSpace(fields["tool."+name+".version"])
		tools[name] = RunnerToolInfo{Name: name, Path: path, Version: version, Installed: path != ""}
	}
	return tools
}

func buildRunnerToolCapabilitySummary(tools map[string]RunnerToolInfo) RunnerToolCapabilitySummary {
	available := make([]string, 0, 8)
	missing := make([]string, 0, 8)
	warnings := make([]string, 0, 4)
	check := func(capability string, names ...string) {
		for _, name := range names {
			if tool, ok := tools[name]; ok && tool.Installed {
				available = append(available, capability)
				return
			}
		}
		missing = append(missing, capability)
	}
	check("mysql_physical_backup", "xtrabackup")
	check("mariadb_physical_backup", "mariabackup", "mariadb-backup")
	check("mysql_binlog_archive", "mysqlbinlog", "mariadb-binlog")
	check("postgres_barman", "barman")
	check("postgres_pg_basebackup", "pg_basebackup")
	check("postgres_pg_combinebackup", "pg_combinebackup")
	check("isolated_restore_container", "docker")
	if xtra := tools["xtrabackup"]; xtra.Installed {
		version := strings.ToLower(xtra.Version)
		switch {
		case strings.Contains(version, "8.4"):
			warnings = append(warnings, "XtraBackup 8.4 通常只适配 MySQL/Percona 8.4，请勿直接用于 MySQL 8.0")
		case strings.Contains(version, "8.0"):
			warnings = append(warnings, "XtraBackup 8.0 通常用于 MySQL/Percona 8.0，请为 MySQL 8.4 使用对应工具版本")
		case strings.Contains(version, "2.4"):
			warnings = append(warnings, "XtraBackup 2.4 通常用于 MySQL/Percona 5.7，已不适合作为新 MySQL 8.x 主链路")
		}
	}
	sort.Strings(available)
	sort.Strings(missing)
	sort.Strings(warnings)
	return RunnerToolCapabilitySummary{Available: available, Missing: missing, Warnings: warnings}
}

func buildRunnerToolCompatibilitySummary(fields map[string]string, tools map[string]RunnerToolInfo) RunnerToolCompatibilitySummary {
	osFamily := strings.ToLower(strings.TrimSpace(fields["os_family"]))
	osVersion := strings.TrimSpace(fields["os_version"])
	pm := strings.ToLower(strings.TrimSpace(fields["package_manager"]))
	supported := make([]string, 0, 8)
	warnings := make([]string, 0, 8)
	switch pm {
	case "apt", "dnf", "yum":
		supported = append(supported, "script_generation")
	default:
		warnings = append(warnings, "未识别 apt/dnf/yum 包管理器，第一版不能生成可直接执行的安装脚本")
	}
	if osFamily == "centos" && strings.HasPrefix(osVersion, "7") {
		warnings = append(warnings, "CentOS 7.9 已接近/处于维护尾声，建议优先使用 Rocky/Alma/Ubuntu/Debian Runner 或容器化工具环境")
	}
	if osFamily == "ubuntu" || osFamily == "debian" || osFamily == "rocky" || osFamily == "almalinux" || osFamily == "centos" || osFamily == "rhel" {
		supported = append(supported, "mysql_tools", "postgres_tools", "docker_restore")
	}
	if tool, ok := tools["docker"]; ok && !tool.Installed {
		warnings = append(warnings, "未发现 docker，自动隔离恢复 Runner 需要额外安装容器运行时")
	}
	sort.Strings(supported)
	sort.Strings(warnings)
	return RunnerToolCompatibilitySummary{
		OS:        strings.TrimSpace(fields["os_family"] + " " + fields["os_version"]),
		Arch:      fields["arch"],
		Supported: supported,
		Warnings:  warnings,
	}
}

func applyRunnerToolProbeResult(host *DatabaseRunnerHost, job *DatabaseRunnerJob, profile *DatabaseRunnerToolProfile, started, finished time.Time, stdout, stderr string, exitCode int, runErr error) {
	if host == nil || job == nil {
		return
	}
	result := runnerToolProbeResult{
		RunnerHostID: host.ID,
		RunnerID:     runnerIDForHost(host),
		Stdout:       trimText(stdout, maxRunnerOutputLength),
		Stderr:       trimText(stderr, maxRunnerOutputLength),
		ExitCode:     exitCode,
		StartedAt:    started.Format("2006-01-02 15:04:05"),
		FinishedAt:   finished.Format("2006-01-02 15:04:05"),
		DurationMs:   finished.Sub(started).Milliseconds(),
	}
	if profile != nil {
		result.ProfileID = profile.ID
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = string(resultJSON)
	job.ExitCode = exitCode
	job.FinishedAt = &finished
	job.DurationMs = result.DurationMs
	job.HeartbeatAt = &finished
	host.LastTestAt = &finished
	if runErr != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(runErr.Error(), 1000)
		host.Status = DatabaseRunnerHostStatusFailed
		host.LastError = job.ErrorMessage
		return
	}
	job.Status = DatabaseRunnerJobStatusSuccess
	job.ErrorMessage = ""
	host.Status = DatabaseRunnerHostStatusOnline
	host.LastError = ""
}

func runnerToolProbeRequestJSON(host *DatabaseRunnerHost, operator QueryOperator) string {
	payload := map[string]any{
		"runnerHostId":   host.ID,
		"runnerType":     host.RunnerType,
		"host":           host.Host,
		"port":           host.Port,
		"workDir":        host.WorkDir,
		"allowedCommand": DatabaseRunnerAllowedCommandToolProbe,
		"operatorId":     operator.ID,
		"operatorName":   operator.Username,
	}
	data, _ := json.Marshal(payload)
	return trimText(string(data), maxRunnerJSONLength)
}

func (uc *UseCase) toRunnerToolProfileVO(host *DatabaseRunnerHost, item *DatabaseRunnerToolProfile) *DatabaseRunnerToolProfileVO {
	if item == nil {
		return nil
	}
	tools := map[string]RunnerToolInfo{}
	var manifest struct {
		Tools map[string]RunnerToolInfo `json:"tools"`
	}
	_ = json.Unmarshal([]byte(item.ToolManifestJSON), &manifest)
	if manifest.Tools != nil {
		tools = manifest.Tools
	}
	capability := RunnerToolCapabilitySummary{}
	_ = json.Unmarshal([]byte(item.CapabilityJSON), &capability)
	compatibility := RunnerToolCompatibilitySummary{}
	_ = json.Unmarshal([]byte(item.CompatibilityJSON), &compatibility)
	runnerName := ""
	if host != nil {
		runnerName = host.Name
	}
	return &DatabaseRunnerToolProfileVO{
		ID:             item.ID,
		RunnerHostID:   item.RunnerHostID,
		RunnerName:     runnerName,
		OSFamily:       item.OSFamily,
		OSVersion:      item.OSVersion,
		OSPrettyName:   item.OSPrettyName,
		Arch:           item.Arch,
		PackageManager: item.PackageManager,
		IsRoot:         item.IsRoot,
		HasSudo:        item.HasSudo,
		HasSystemd:     item.HasSystemd,
		HasDocker:      item.HasDocker,
		NetworkAccess:  item.NetworkAccess,
		Tools:          tools,
		Capability:     capability,
		Compatibility:  compatibility,
		LastProbeAt:    formatTime(item.LastProbeAt),
		LastStatus:     item.LastProbeStatus,
		LastError:      item.LastError,
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func buildRunnerToolInstallScript(host *DatabaseRunnerHost, profile *DatabaseRunnerToolProfile, req *DatabaseRunnerToolInstallScriptRequest) *DatabaseRunnerToolInstallScriptVO {
	if req == nil {
		req = &DatabaseRunnerToolInstallScriptRequest{}
	}
	profiles := normalizeRunnerToolProfiles(req.Profiles)
	if len(profiles) == 0 {
		profiles = []string{"mysql_80_physical", "mysql_binlog_archiver", "postgres_barman", "postgres_native_pg_basebackup", "restore_runner"}
	}
	mode := strings.ToLower(strings.TrimSpace(req.InstallMode))
	if mode == "" {
		mode = "online"
	}
	executionMode := normalizeToolExecutionMode(req.ExecutionMode)
	warnings := make([]string, 0, 8)
	unsupported := make([]string, 0, 4)
	lines := []string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		"",
		"# OpsHub Runner 数据库备份工具安装脚本",
		"# 仅由 OpsHub 生成，不会自动执行；执行前请在测试主机验证版本和仓库可达性。",
		fmt.Sprintf("# Runner: %s (%s:%d)", valueOr(host.Name, "-"), valueOr(host.Host, "-"), host.Port),
		fmt.Sprintf("# OS: %s / %s / %s", profile.OSPrettyName, profile.OSFamily, profile.OSVersion),
		fmt.Sprintf("# Package manager: %s", profile.PackageManager),
		fmt.Sprintf("# Generated at: %s", time.Now().Format("2006-01-02 15:04:05")),
		"",
		`if [ "$(id -u)" != "0" ]; then`,
		`  SUDO="sudo"`,
		`else`,
		`  SUDO=""`,
		`fi`,
		"",
	}
	if !profile.IsRoot && !profile.HasSudo {
		warnings = append(warnings, "巡检显示当前用户不是 root 且 sudo -n 不可用，脚本执行前需要准备具备安装权限的账号")
	}
	if mode == "offline" {
		warnings = append(warnings, "离线模式第一版只生成安装清单和校验命令，离线包路径需由运维手动补齐")
		lines = append(lines,
			`echo "OFFLINE 模式：请先准备 Percona/PostgreSQL/Docker 相关离线包，再按本脚本的包名清单安装。"`,
			"",
		)
	}
	if executionMode == DatabaseToolExecutionModeContainer {
		appendContainerRunnerToolScript(&lines, profiles, req, &warnings, &unsupported)
	} else {
		switch strings.ToLower(strings.TrimSpace(profile.PackageManager)) {
		case "apt":
			appendAptRunnerToolScript(&lines, profiles, &warnings, &unsupported, mode)
		case "dnf", "yum":
			appendYumRunnerToolScript(&lines, profiles, &warnings, &unsupported, strings.ToLower(strings.TrimSpace(profile.PackageManager)), profile.OSFamily, profile.OSVersion, mode)
		default:
			unsupported = append(unsupported, "当前包管理器未识别，仅支持 apt/dnf/yum 脚本生成")
			lines = append(lines, `echo "Unsupported package manager; please install tools manually."`)
		}
	}
	lines = append(lines,
		"",
		"echo '--- OpsHub runner tool versions ---'",
		"for bin in mysql mysqlbinlog mariadb mariadb-binlog xtrabackup mariabackup mariadb-backup psql pg_basebackup pg_receivewal pg_combinebackup pg_verifybackup barman docker tar sha256sum zstd gzip rsync; do",
		"  if command -v \"$bin\" >/dev/null 2>&1; then",
		"    printf '%s: ' \"$bin\"; \"$bin\" --version 2>&1 | head -n 1 || true",
		"  fi",
		"done",
		"",
		"echo 'OpsHub Runner tool install script finished.'",
	)
	if profile.OSFamily == "centos" && strings.HasPrefix(profile.OSVersion, "7") {
		warnings = append(warnings, "CentOS 7.9 上新版本数据库工具兼容性和仓库可用性较差，建议优先更换 Rocky/Alma/Ubuntu/Debian Runner")
	}
	return &DatabaseRunnerToolInstallScriptVO{
		RunnerHostID:     host.ID,
		RunnerHostName:   host.Name,
		OSFamily:         profile.OSFamily,
		OSVersion:        profile.OSVersion,
		OSPrettyName:     profile.OSPrettyName,
		Arch:             profile.Arch,
		PackageManager:   profile.PackageManager,
		Profiles:         profiles,
		TargetDBVersions: req.TargetDBVersions,
		InstallMode:      mode,
		ExecutionMode:    executionMode,
		ToolImage:        strings.TrimSpace(req.ToolImage),
		ToolImageDigest:  strings.TrimSpace(req.ToolImageDigest),
		DatadirMount:     strings.TrimSpace(req.DatadirMount),
		WorkdirMount:     strings.TrimSpace(req.WorkdirMount),
		NetworkMode:      normalizeContainerNetworkMode(req.NetworkMode),
		ReadOnlyDatadir:  req.ReadOnlyDatadir,
		DryRun:           req.DryRun,
		Warnings:         uniqueSorted(warnings),
		Unsupported:      uniqueSorted(unsupported),
		Script:           strings.Join(lines, "\n"),
		GeneratedAt:      time.Now().Format("2006-01-02 15:04:05"),
	}
}

func appendAptRunnerToolScript(lines *[]string, profiles []string, warnings, unsupported *[]string, mode string) {
	*lines = append(*lines,
		"$SUDO apt-get update",
		"$SUDO apt-get install -y ca-certificates curl gnupg lsb-release tar gzip zstd jq rsync",
		"",
	)
	if mode != "offline" && hasAnyProfile(profiles, "mysql_57_physical", "mysql_80_physical", "mysql_84_physical") {
		*lines = append(*lines,
			"curl -fsSLO https://repo.percona.com/apt/percona-release_latest.generic_all.deb",
			"$SUDO dpkg -i percona-release_latest.generic_all.deb",
		)
	}
	for _, profile := range profiles {
		switch profile {
		case "mysql_57_physical":
			*warnings = append(*warnings, "XtraBackup 2.4/MySQL 5.7 仅用于遗留链路，新生产库不建议新建")
			*lines = append(*lines, "$SUDO percona-release setup pxb-24 || true", "$SUDO apt-get update", "$SUDO apt-get install -y percona-xtrabackup-24 mysql-client")
		case "mysql_80_physical":
			*lines = append(*lines, "$SUDO percona-release setup pxb-80 || true", "$SUDO apt-get update", "$SUDO apt-get install -y percona-xtrabackup-80 mysql-client")
		case "mysql_84_physical":
			*warnings = append(*warnings, "请确认 Percona 仓库中 MySQL 8.4 对应 XtraBackup 包名；不同发行版可能为 percona-xtrabackup-84 或 percona-xtrabackup")
			*lines = append(*lines, "$SUDO percona-release setup pxb-84 || true", "$SUDO apt-get update", "$SUDO apt-get install -y percona-xtrabackup-84 mysql-client || $SUDO apt-get install -y percona-xtrabackup mysql-client")
		case "mysql_binlog_archiver":
			*lines = append(*lines, "$SUDO apt-get install -y mysql-client || $SUDO apt-get install -y default-mysql-client")
		case "mariadb_physical":
			*lines = append(*lines, "$SUDO apt-get install -y mariadb-client mariadb-backup")
		case "postgres_barman":
			*lines = append(*lines, "$SUDO apt-get install -y postgresql-client barman")
		case "postgres_native_pg_basebackup":
			*lines = append(*lines, "$SUDO apt-get install -y postgresql-client")
		case "restore_runner":
			*lines = append(*lines, "$SUDO apt-get install -y docker.io tar gzip zstd rsync")
		default:
			*unsupported = append(*unsupported, profile)
		}
	}
}

func appendYumRunnerToolScript(lines *[]string, profiles []string, warnings, unsupported *[]string, packageManager, osFamily, osVersion, mode string) {
	pm := "$SUDO " + packageManager
	*lines = append(*lines,
		pm+" install -y ca-certificates curl tar gzip zstd jq rsync",
		"",
	)
	if osFamily == "centos" && strings.HasPrefix(osVersion, "7") {
		*lines = append(*lines, pm+" install -y epel-release || true")
	}
	if mode != "offline" && hasAnyProfile(profiles, "mysql_57_physical", "mysql_80_physical", "mysql_84_physical") {
		*lines = append(*lines, pm+" install -y https://repo.percona.com/yum/percona-release-latest.noarch.rpm")
	}
	for _, profile := range profiles {
		switch profile {
		case "mysql_57_physical":
			*warnings = append(*warnings, "XtraBackup 2.4/MySQL 5.7 仅用于遗留链路，新生产库不建议新建")
			*lines = append(*lines, "$SUDO percona-release setup pxb-24 || true", pm+" install -y percona-xtrabackup-24 mysql")
		case "mysql_80_physical":
			*lines = append(*lines, "$SUDO percona-release setup pxb-80 || true", pm+" install -y percona-xtrabackup-80 mysql")
		case "mysql_84_physical":
			*warnings = append(*warnings, "请确认 Percona 仓库中 MySQL 8.4 对应 XtraBackup 包名；不同 RHEL 系发行版可能存在差异")
			*lines = append(*lines, "$SUDO percona-release setup pxb-84 || true", pm+" install -y percona-xtrabackup-84 mysql || "+pm+" install -y percona-xtrabackup mysql")
		case "mysql_binlog_archiver":
			*lines = append(*lines, pm+" install -y mysql")
		case "mariadb_physical":
			*lines = append(*lines, pm+" install -y MariaDB-client MariaDB-backup || "+pm+" install -y mariadb mariadb-backup")
		case "postgres_barman":
			*lines = append(*lines, pm+" install -y postgresql barman")
		case "postgres_native_pg_basebackup":
			*lines = append(*lines, pm+" install -y postgresql")
		case "restore_runner":
			*lines = append(*lines, pm+" install -y docker tar gzip zstd rsync")
		default:
			*unsupported = append(*unsupported, profile)
		}
	}
}

func appendContainerRunnerToolScript(lines *[]string, profiles []string, req *DatabaseRunnerToolInstallScriptRequest, warnings, unsupported *[]string) {
	image := strings.TrimSpace(req.ToolImage)
	if err := validateContainerToolImage(image); err != nil {
		*unsupported = append(*unsupported, err.Error())
	}
	if image == "" {
		image = "opshub-runner-tools:mysql80"
		*warnings = append(*warnings, "未指定容器镜像，脚本示例使用 opshub-runner-tools:mysql80；生产环境必须固定版本 tag 或 digest")
	}
	datadirMount := strings.TrimSpace(req.DatadirMount)
	if hasAnyProfile(profiles, "mysql_57_physical", "mysql_80_physical", "mysql_84_physical", "mariadb_physical") && datadirMount == "" {
		*warnings = append(*warnings, "MySQL/MariaDB 物理备份容器需要把数据库 datadir 只读挂载到容器；未配置时执行任务会阻断")
	}
	networkMode := normalizeContainerNetworkMode(req.NetworkMode)
	readOnly := "ro"
	if !req.ReadOnlyDatadir {
		readOnly = "rw"
		*warnings = append(*warnings, "datadir 非只读挂载风险较高，仅在明确需要且已隔离 Runner 时使用")
	}
	*warnings = append(*warnings, "容器化工具模式不会在宿主机安装 xtrabackup/mariadb-backup/pg_basebackup，只校验 Docker、镜像、挂载和容器内工具版本")
	*lines = append(*lines,
		"echo '--- OpsHub containerized runner tools ---'",
		"CONTAINER_TOOL_IMAGE="+shellSingleQuote(image),
		"CONTAINER_TOOL_DIGEST="+shellSingleQuote(strings.TrimSpace(req.ToolImageDigest)),
		"CONTAINER_DATADIR="+shellSingleQuote(datadirMount),
		"CONTAINER_NETWORK_MODE="+shellSingleQuote(networkMode),
		"CONTAINER_DATADIR_MODE="+shellSingleQuote(readOnly),
		`command -v docker >/dev/null 2>&1 || { echo "docker not found"; exit 1; }`,
		`docker image inspect "$CONTAINER_TOOL_IMAGE" >/dev/null 2>&1 || docker pull "$CONTAINER_TOOL_IMAGE"`,
		`if [ -n "$CONTAINER_TOOL_DIGEST" ]; then docker inspect --format='{{index .RepoDigests 0}}' "$CONTAINER_TOOL_IMAGE" | grep -q "$CONTAINER_TOOL_DIGEST"; fi`,
		`if [ -n "$CONTAINER_DATADIR" ]; then [ -d "$CONTAINER_DATADIR" ] || { echo "datadir mount not found: $CONTAINER_DATADIR"; exit 1; }; fi`,
		`RUN_ARGS="--rm --network $CONTAINER_NETWORK_MODE"`,
		`if [ -n "$CONTAINER_DATADIR" ]; then RUN_ARGS="$RUN_ARGS -v $CONTAINER_DATADIR:/var/lib/mysql:$CONTAINER_DATADIR_MODE"; fi`,
		`docker run $RUN_ARGS "$CONTAINER_TOOL_IMAGE" sh -lc 'for bin in xtrabackup mariadb-backup mariabackup mysqlbinlog mariadb-binlog mysql mariadb psql pg_basebackup pg_receivewal pg_combinebackup pg_verifybackup barman docker tar sha256sum zstd gzip rsync; do command -v "$bin" >/dev/null 2>&1 && { printf "%s: " "$bin"; "$bin" --version 2>&1 | head -n 1 || true; }; done'`,
	)
}

func normalizeRunnerToolProfiles(values []string) []string {
	allowed := map[string]struct{}{
		"mysql_57_physical":             {},
		"mysql_80_physical":             {},
		"mysql_84_physical":             {},
		"mysql_binlog_archiver":         {},
		"mariadb_physical":              {},
		"postgres_barman":               {},
		"postgres_native_pg_basebackup": {},
		"restore_runner":                {},
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[key]; !ok {
			if key == "" {
				continue
			}
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hasAnyProfile(values []string, wanted ...string) bool {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	for _, value := range wanted {
		if _, ok := set[value]; ok {
			return true
		}
	}
	return false
}

func runnerToolJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseBoolText(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
}

func defaultIfBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
