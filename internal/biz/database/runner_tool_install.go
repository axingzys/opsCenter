package database

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	defaultRunnerToolOfflinePackageDir = "data/database-backups/runner-tool-offline-packages"
	runnerToolInstallStepMarker        = "OPSHUB_RUNNER_TOOL_STEP="
	runnerToolInstallResultMarker      = "OPSHUB_RUNNER_TOOL_RESULT="
)

type DatabaseRunnerToolInstallRequest struct {
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
	ConfirmInstall   bool              `json:"confirmInstall"`
	ConfirmPackages  bool              `json:"confirmPackages"`
	Reason           string            `json:"reason"`
	OfflinePackageID uint              `json:"offlinePackageId"`
	AllowRiskyOS     bool              `json:"allowRiskyOs"`
}

type DatabaseRunnerToolOfflinePackageListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
	OSFamily string `form:"osFamily"`
	Arch     string `form:"arch"`
	Status   string `form:"status"`
}

type DatabaseRunnerToolOfflinePackageRegisterRequest struct {
	Name           string   `json:"name"`
	PackageVersion string   `json:"packageVersion"`
	OSFamily       string   `json:"osFamily"`
	OSVersion      string   `json:"osVersion"`
	Arch           string   `json:"arch"`
	PackageManager string   `json:"packageManager"`
	Profiles       []string `json:"profiles"`
	ChecksumSHA256 string   `json:"checksumSha256"`
	ManifestJSON   string   `json:"manifestJson"`
	FileName       string   `json:"fileName"`
}

type DatabaseRunnerToolOfflinePackageVO struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	PackageVersion string   `json:"packageVersion"`
	OSFamily       string   `json:"osFamily"`
	OSVersion      string   `json:"osVersion"`
	Arch           string   `json:"arch"`
	PackageManager string   `json:"packageManager"`
	Profiles       []string `json:"profiles"`
	FileName       string   `json:"fileName"`
	FileSize       int64    `json:"fileSize"`
	ChecksumSHA256 string   `json:"checksumSha256"`
	StoragePath    string   `json:"storagePath"`
	Status         string   `json:"status"`
	ManifestJSON   string   `json:"manifestJson"`
	UploadedByID   uint     `json:"uploadedById"`
	UploadedByName string   `json:"uploadedByName"`
	UploadedAt     string   `json:"uploadedAt"`
	LastVerifiedAt string   `json:"lastVerifiedAt"`
	LastError      string   `json:"lastError"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

type runnerToolInstallStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type runnerToolInstallResult struct {
	RunnerHostID       uint                    `json:"runnerHostId"`
	RunnerID           string                  `json:"runnerId"`
	Mode               string                  `json:"mode"`
	ExecutionMode      string                  `json:"executionMode"`
	ToolImage          string                  `json:"toolImage,omitempty"`
	DryRun             bool                    `json:"dryRun"`
	Profiles           []string                `json:"profiles"`
	OfflinePackageID   uint                    `json:"offlinePackageId,omitempty"`
	LogPath            string                  `json:"logPath"`
	ManifestPath       string                  `json:"manifestPath"`
	RemotePackagePath  string                  `json:"remotePackagePath,omitempty"`
	Steps              []runnerToolInstallStep `json:"steps"`
	Stdout             string                  `json:"stdout"`
	Stderr             string                  `json:"stderr"`
	ExitCode           int                     `json:"exitCode"`
	StartedAt          string                  `json:"startedAt"`
	FinishedAt         string                  `json:"finishedAt"`
	DurationMs         int64                   `json:"durationMs"`
	RefreshedProfileID uint                    `json:"refreshedProfileId,omitempty"`
	RefreshError       string                  `json:"refreshError,omitempty"`
}

func (uc *UseCase) InstallRunnerTools(ctx context.Context, runnerHostID uint, req *DatabaseRunnerToolInstallRequest, operator QueryOperator) (*DatabaseRunnerJobVO, error) {
	if uc.runnerHostRepo == nil || uc.runnerJobRepo == nil || uc.runnerToolProfileRepo == nil {
		return nil, fmt.Errorf("Runner 工具安装仓库未配置")
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
	profile, err := uc.runnerToolProfileRepo.GetByRunnerHostID(ctx, runnerHostID)
	if err != nil {
		return nil, fmt.Errorf("请先对 Runner 主机执行工具巡检，再执行安装")
	}
	normalized, err := normalizeRunnerToolInstallRequest(req)
	if err != nil {
		return nil, err
	}
	if !normalized.ConfirmInstall || !normalized.ConfirmPackages {
		return nil, fmt.Errorf("未确认安装动作和软件包范围")
	}
	if strings.TrimSpace(normalized.Reason) == "" {
		return nil, fmt.Errorf("安装原因不能为空")
	}
	if profile.OSFamily == "centos" && strings.HasPrefix(profile.OSVersion, "7") && !normalized.DryRun && !normalized.AllowRiskyOS {
		return nil, fmt.Errorf("CentOS 7.x Runner 默认禁止直接安装数据库备份工具，请更换 Runner 或显式允许风险系统")
	}
	var offlinePackage *DatabaseRunnerToolOfflinePackage
	if normalized.InstallMode == "offline" && normalizeToolExecutionMode(normalized.ExecutionMode) != DatabaseToolExecutionModeContainer {
		if uc.runnerToolOfflineRepo == nil {
			return nil, fmt.Errorf("Runner 工具离线包仓库未配置")
		}
		if normalized.OfflinePackageID == 0 {
			return nil, fmt.Errorf("离线安装必须选择离线包")
		}
		offlinePackage, err = uc.runnerToolOfflineRepo.GetByID(ctx, normalized.OfflinePackageID)
		if err != nil {
			return nil, fmt.Errorf("Runner 工具离线包不存在")
		}
		if err := validateRunnerToolOfflinePackageForProfile(profile, offlinePackage, normalized.Profiles); err != nil {
			return nil, err
		}
	}
	requestJSON, _ := json.Marshal(normalized)
	job := &DatabaseRunnerJob{
		JobType:        DatabaseRunnerJobTypeToolInstall,
		RunnerHostID:   host.ID,
		RunnerID:       runnerIDForHost(host),
		Status:         DatabaseRunnerJobStatusQueued,
		AllowedCommand: DatabaseRunnerAllowedCommandToolInstall,
		CommandSummary: trimText(fmt.Sprintf("Runner 工具安装: %s/%s / dryRun=%t / profiles=%s", normalized.InstallMode, normalizeToolExecutionMode(normalized.ExecutionMode), normalized.DryRun, strings.Join(normalized.Profiles, ",")), 500),
		WorkDir:        host.WorkDir,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 120),
		RequestJSON:    trimText(string(requestJSON), maxRunnerJSONLength),
	}
	if err := uc.runnerJobRepo.Create(ctx, job); err != nil {
		return nil, err
	}
	go uc.executeRunnerToolInstall(context.Background(), host.ID, job.ID, normalized)
	return uc.toRunnerJobVO(ctx, job), nil
}

func (uc *UseCase) ListRunnerToolOfflinePackages(ctx context.Context, req *DatabaseRunnerToolOfflinePackageListRequest) ([]*DatabaseRunnerToolOfflinePackageVO, int64, error) {
	if uc.runnerToolOfflineRepo == nil {
		return nil, 0, fmt.Errorf("Runner 工具离线包仓库未配置")
	}
	items, total, err := uc.runnerToolOfflineRepo.List(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DatabaseRunnerToolOfflinePackageVO, 0, len(items))
	for _, item := range items {
		list = append(list, toRunnerToolOfflinePackageVO(item))
	}
	return list, total, nil
}

func (uc *UseCase) RegisterRunnerToolOfflinePackage(ctx context.Context, req *DatabaseRunnerToolOfflinePackageRegisterRequest, reader io.Reader, operator QueryOperator) (*DatabaseRunnerToolOfflinePackageVO, error) {
	if uc.runnerToolOfflineRepo == nil {
		return nil, fmt.Errorf("Runner 工具离线包仓库未配置")
	}
	if req == nil {
		return nil, fmt.Errorf("离线包请求不能为空")
	}
	if reader == nil {
		return nil, fmt.Errorf("离线包文件不能为空")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.FileName)
	}
	if name == "" {
		return nil, fmt.Errorf("离线包名称不能为空")
	}
	fileName := sanitizeRunnerToolPackageFileName(req.FileName)
	if fileName == "" {
		return nil, fmt.Errorf("离线包文件名不能为空")
	}
	profiles := normalizeRunnerToolProfiles(req.Profiles)
	if len(profiles) == 0 {
		return nil, fmt.Errorf("离线包至少需要声明一个工具 Profile")
	}
	uploadedAt := time.Now()
	tmpDir := filepath.Join(defaultRunnerToolOfflinePackageDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建离线包目录失败: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(tmpFile, hasher), reader)
	closeErr := tmpFile.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("保存离线包失败: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("关闭离线包文件失败: %w", closeErr)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if expected := strings.ToLower(strings.TrimSpace(req.ChecksumSHA256)); expected != "" && expected != checksum {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("离线包 SHA256 不匹配，期望 %s，实际 %s", expected, checksum)
	}
	finalDir := filepath.Join(defaultRunnerToolOfflinePackageDir, checksum)
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("创建离线包存储目录失败: %w", err)
	}
	finalPath := filepath.Join(finalDir, fileName)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("保存离线包文件失败: %w", err)
	}
	profilesJSON, _ := json.Marshal(profiles)
	item := &DatabaseRunnerToolOfflinePackage{
		Name:           trimText(name, 160),
		PackageVersion: trimText(req.PackageVersion, 80),
		OSFamily:       trimText(strings.ToLower(strings.TrimSpace(req.OSFamily)), 80),
		OSVersion:      trimText(strings.TrimSpace(req.OSVersion), 80),
		Arch:           trimText(strings.ToLower(strings.TrimSpace(req.Arch)), 80),
		PackageManager: trimText(strings.ToLower(strings.TrimSpace(req.PackageManager)), 40),
		ProfilesJSON:   string(profilesJSON),
		FileName:       fileName,
		FileSize:       size,
		ChecksumSHA256: checksum,
		StoragePath:    finalPath,
		Status:         "available",
		ManifestJSON:   trimText(req.ManifestJSON, maxRunnerToolJSONLength),
		UploadedByID:   operator.ID,
		UploadedByName: trimText(operator.Username, 120),
		UploadedAt:     &uploadedAt,
		LastVerifiedAt: &uploadedAt,
	}
	if err := uc.runnerToolOfflineRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return toRunnerToolOfflinePackageVO(item), nil
}

func (uc *UseCase) GetRunnerToolOfflinePackage(ctx context.Context, id uint) (*DatabaseRunnerToolOfflinePackage, error) {
	if uc.runnerToolOfflineRepo == nil {
		return nil, fmt.Errorf("Runner 工具离线包仓库未配置")
	}
	if id == 0 {
		return nil, fmt.Errorf("离线包ID不能为空")
	}
	return uc.runnerToolOfflineRepo.GetByID(ctx, id)
}

func (uc *UseCase) executeRunnerToolInstall(ctx context.Context, runnerHostID, jobID uint, req *DatabaseRunnerToolInstallRequest) {
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

	stdout, stderr, exitCode, err := uc.runRunnerToolInstallCommand(ctx, host, job.ID, req)
	finished := time.Now()
	result := buildRunnerToolInstallResult(host, req, started, finished, stdout, stderr, exitCode)
	if err == nil && !req.DryRun {
		probeOut, _, _, probeErr := uc.runRunnerToolProbeCommand(ctx, host)
		profile, profileErr := buildRunnerToolProfileFromOutput(host.ID, probeOut, finished, probeErr)
		if probeErr != nil || profileErr != nil {
			result.RefreshError = trimText(firstNonNilError(probeErr, profileErr), 1000)
		} else if profile != nil {
			if upsertErr := uc.runnerToolProfileRepo.UpsertByRunnerHostID(ctx, profile); upsertErr != nil {
				result.RefreshError = trimText(upsertErr.Error(), 1000)
			} else {
				result.RefreshedProfileID = profile.ID
			}
		}
	}
	resultJSON, _ := json.Marshal(result)
	job.ResultJSON = trimText(string(resultJSON), maxRunnerJSONLength)
	job.LogPath = result.LogPath
	job.ExitCode = exitCode
	job.FinishedAt = &finished
	job.DurationMs = finished.Sub(started).Milliseconds()
	job.HeartbeatAt = &finished
	host.LastTestAt = &finished
	if err != nil {
		job.Status = DatabaseRunnerJobStatusFailed
		job.ErrorMessage = trimText(err.Error(), 1000)
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

func (uc *UseCase) runRunnerToolInstallCommand(ctx context.Context, host *DatabaseRunnerHost, jobID uint, req *DatabaseRunnerToolInstallRequest) (string, string, int, error) {
	if host == nil {
		return "", "", 1, fmt.Errorf("Runner 主机不能为空")
	}
	if host.RunnerType != DatabaseRunnerTypeSSH {
		return "", "", 1, fmt.Errorf("当前 Runner 工具安装仅支持 SSH Runner")
	}
	if uc.credentialResolver == nil {
		return "", "", 1, fmt.Errorf("连接凭据解析器未配置")
	}
	credential, err := uc.credentialResolver(ctx, host.CredentialID)
	if err != nil {
		return "", "", 1, fmt.Errorf("解析 Runner 凭据失败: %w", err)
	}
	profile, err := uc.runnerToolProfileRepo.GetByRunnerHostID(ctx, host.ID)
	if err != nil {
		return "", "", 1, fmt.Errorf("Runner 工具画像不存在")
	}
	var offlinePackage *DatabaseRunnerToolOfflinePackage
	remotePackagePath := ""
	if req.InstallMode == "offline" && normalizeToolExecutionMode(req.ExecutionMode) != DatabaseToolExecutionModeContainer && req.OfflinePackageID > 0 {
		if uc.runnerToolOfflineRepo == nil {
			return "", "", 1, fmt.Errorf("Runner 工具离线包仓库未配置")
		}
		offlinePackage, err = uc.runnerToolOfflineRepo.GetByID(ctx, req.OfflinePackageID)
		if err != nil {
			return "", "", 1, fmt.Errorf("Runner 工具离线包不存在")
		}
		if !req.DryRun {
			workDir := normalizeRunnerWorkDir(host.WorkDir)
			remotePackagePath = path.Join(workDir, "tool-install", fmt.Sprintf("job-%d", jobID), "offline", offlinePackage.FileName)
			mkdirScript := fmt.Sprintf("mkdir -p %s", shellSingleQuote(path.Dir(remotePackagePath)))
			if _, _, _, err := executeSSHRunnerScript(ctx, host.Host, host.Port, credential, mkdirScript, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute); err != nil {
				return "", "", 1, err
			}
			if err := uploadSSHRunnerFile(ctx, host.Host, host.Port, credential, offlinePackage.StoragePath, remotePackagePath, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute); err != nil {
				return "", "", 1, fmt.Errorf("上传离线包失败: %w", err)
			}
		}
	}
	script := buildRunnerToolInstallExecutionScript(host, profile, req, jobID, offlinePackage, remotePackagePath)
	return executeSSHRunnerScript(ctx, host.Host, host.Port, credential, script, time.Duration(normalizeRunnerTimeoutMinutes(host.TimeoutMinutes))*time.Minute)
}

func normalizeRunnerToolInstallRequest(req *DatabaseRunnerToolInstallRequest) (*DatabaseRunnerToolInstallRequest, error) {
	if req == nil {
		req = &DatabaseRunnerToolInstallRequest{}
	}
	profiles := normalizeRunnerToolProfiles(req.Profiles)
	if len(profiles) == 0 {
		return nil, fmt.Errorf("请至少选择一个工具 Profile")
	}
	mode := strings.ToLower(strings.TrimSpace(req.InstallMode))
	if mode == "" {
		mode = "online"
	}
	if mode != "online" && mode != "offline" {
		return nil, fmt.Errorf("安装模式仅支持 online/offline")
	}
	executionMode := normalizeToolExecutionMode(req.ExecutionMode)
	toolImage := strings.TrimSpace(req.ToolImage)
	if executionMode == DatabaseToolExecutionModeContainer {
		if toolImage == "" {
			return nil, fmt.Errorf("容器化工具模式必须填写工具镜像")
		}
		if err := validateContainerToolImage(toolImage); err != nil {
			return nil, err
		}
		if err := validateContainerNetworkMode(req.NetworkMode); err != nil {
			return nil, err
		}
		if runnerToolProfilesRequireDatadir(profiles) && strings.TrimSpace(req.DatadirMount) == "" {
			return nil, fmt.Errorf("MySQL/MariaDB 物理备份容器模式必须填写 datadir 挂载路径")
		}
	}
	return &DatabaseRunnerToolInstallRequest{
		Profiles:         profiles,
		TargetDBVersions: req.TargetDBVersions,
		InstallMode:      mode,
		ExecutionMode:    executionMode,
		ToolImage:        toolImage,
		ToolImageDigest:  trimText(strings.TrimSpace(req.ToolImageDigest), 255),
		DatadirMount:     trimText(strings.TrimSpace(req.DatadirMount), 500),
		WorkdirMount:     trimText(strings.TrimSpace(req.WorkdirMount), 500),
		NetworkMode:      normalizeContainerNetworkMode(req.NetworkMode),
		ReadOnlyDatadir:  req.ReadOnlyDatadir,
		DryRun:           req.DryRun,
		ConfirmInstall:   req.ConfirmInstall,
		ConfirmPackages:  req.ConfirmPackages,
		Reason:           trimText(strings.TrimSpace(req.Reason), 1000),
		OfflinePackageID: req.OfflinePackageID,
		AllowRiskyOS:     req.AllowRiskyOS,
	}, nil
}

func buildRunnerToolInstallExecutionScript(host *DatabaseRunnerHost, profile *DatabaseRunnerToolProfile, req *DatabaseRunnerToolInstallRequest, jobID uint, offlinePackage *DatabaseRunnerToolOfflinePackage, remotePackagePath string) string {
	if req == nil {
		req = &DatabaseRunnerToolInstallRequest{}
	}
	workDir := normalizeRunnerWorkDir(host.WorkDir)
	jobDir := path.Join(workDir, "tool-install", fmt.Sprintf("job-%d", jobID))
	installScript := buildRunnerToolInstallScript(host, profile, &DatabaseRunnerToolInstallScriptRequest{
		Profiles:         req.Profiles,
		TargetDBVersions: req.TargetDBVersions,
		InstallMode:      req.InstallMode,
		ExecutionMode:    req.ExecutionMode,
		ToolImage:        req.ToolImage,
		ToolImageDigest:  req.ToolImageDigest,
		DatadirMount:     req.DatadirMount,
		WorkdirMount:     req.WorkdirMount,
		NetworkMode:      req.NetworkMode,
		ReadOnlyDatadir:  req.ReadOnlyDatadir,
		DryRun:           req.DryRun,
	})
	installScriptB64 := base64.StdEncoding.EncodeToString([]byte(installScript.Script))
	manifest := map[string]any{
		"runnerHostId":     host.ID,
		"runnerId":         runnerIDForHost(host),
		"jobId":            jobID,
		"mode":             req.InstallMode,
		"executionMode":    normalizeToolExecutionMode(req.ExecutionMode),
		"toolImage":        req.ToolImage,
		"toolImageDigest":  req.ToolImageDigest,
		"datadirMount":     req.DatadirMount,
		"workdirMount":     req.WorkdirMount,
		"networkMode":      normalizeContainerNetworkMode(req.NetworkMode),
		"dryRun":           req.DryRun,
		"profiles":         req.Profiles,
		"targetDbVersions": req.TargetDBVersions,
		"reason":           req.Reason,
		"generatedAt":      time.Now().Format("2006-01-02 15:04:05"),
	}
	if offlinePackage != nil {
		manifest["offlinePackage"] = map[string]any{
			"id":             offlinePackage.ID,
			"name":           offlinePackage.Name,
			"fileName":       offlinePackage.FileName,
			"fileSize":       offlinePackage.FileSize,
			"checksumSha256": offlinePackage.ChecksumSHA256,
			"remotePath":     remotePackagePath,
		}
	}
	manifestB64 := base64.StdEncoding.EncodeToString([]byte(runnerToolJSON(manifest)))
	requiredChecks := buildRunnerToolRequiredCheckScript(req.Profiles, req.DryRun)
	if normalizeToolExecutionMode(req.ExecutionMode) == DatabaseToolExecutionModeContainer {
		requiredChecks = buildRunnerToolContainerRequiredCheckScript(req)
	}
	mode := req.InstallMode
	dryRunFlag := "0"
	if req.DryRun {
		dryRunFlag = "1"
	}
	lines := []string{
		"set +e",
		"JOB_DIR=" + shellSingleQuote(jobDir),
		"INSTALL_SCRIPT=\"$JOB_DIR/install.sh\"",
		"LOG_PATH=\"$JOB_DIR/install.log\"",
		"MANIFEST_PATH=\"$JOB_DIR/manifest.json\"",
		"mkdir -p \"$JOB_DIR\"",
		": > \"$LOG_PATH\"",
		`emit_step() { name="$1"; status="$2"; shift 2; printf '` + runnerToolInstallStepMarker + `%s|%s|%s\n' "$name" "$status" "$*"; }`,
		`emit_result() { RESULT_JSON="{\"logPath\":\"$LOG_PATH\",\"manifestPath\":\"$MANIFEST_PATH\"}"; RESULT_B64="$(printf '%s' "$RESULT_JSON" | base64 | tr -d '\n')"; printf '` + runnerToolInstallResultMarker + `%s\n' "$RESULT_B64"; }`,
		"emit_step detect_os running detect runner os",
		`OS_ID=""; OS_VERSION=""; [ -r /etc/os-release ] && . /etc/os-release && OS_ID="${ID:-}" && OS_VERSION="${VERSION_ID:-}"`,
		`emit_step detect_os success "${OS_ID:-unknown} ${OS_VERSION:-}"`,
		"emit_step precheck running check workdir and privilege",
		`command -v bash >/dev/null 2>&1 || { emit_step precheck failed "bash missing"; emit_result; exit 10; }`,
		`touch "$JOB_DIR/.write-test" 2>>"$LOG_PATH" && rm -f "$JOB_DIR/.write-test" || { emit_step precheck failed "workdir not writable"; emit_result; exit 11; }`,
		`if [ "$(id -u 2>/dev/null)" != "0" ]; then sudo -n true >/dev/null 2>&1 || { emit_step precheck failed "user is not root and sudo is unavailable"; emit_result; exit 12; }; fi`,
		"emit_step precheck success workdir and privilege ok",
		"emit_step write_manifest running write install manifest",
		"base64 -d > \"$MANIFEST_PATH\" <<'OPSHUB_MANIFEST_B64'",
		manifestB64,
		"OPSHUB_MANIFEST_B64",
		"emit_step write_manifest success \"$MANIFEST_PATH\"",
		"emit_step install_repo running prepare controlled install script",
		"base64 -d > \"$INSTALL_SCRIPT\" <<'OPSHUB_INSTALL_B64'",
		installScriptB64,
		"OPSHUB_INSTALL_B64",
		"chmod 700 \"$INSTALL_SCRIPT\"",
		"emit_step install_repo success install script written to runner workdir",
	}
	if normalizeToolExecutionMode(req.ExecutionMode) == DatabaseToolExecutionModeContainer {
		lines = append(lines, buildRunnerToolContainerInstallLines(req)...)
	} else if mode == "offline" {
		lines = append(lines, buildRunnerToolOfflineInstallLines(req, offlinePackage, remotePackagePath)...)
	} else {
		lines = append(lines,
			"emit_step install_packages running run online install script",
			fmt.Sprintf("DRY_RUN=%s", dryRunFlag),
			`if [ "$DRY_RUN" = "1" ]; then`,
			`  emit_step install_packages skipped "dry-run: script only, package install skipped"`,
			`else`,
			`  bash "$INSTALL_SCRIPT" >>"$LOG_PATH" 2>&1`,
			`  INSTALL_EXIT=$?`,
			`  if [ "$INSTALL_EXIT" != "0" ]; then emit_step install_packages failed "install script exit $INSTALL_EXIT"; emit_result; exit "$INSTALL_EXIT"; fi`,
			`  emit_step install_packages success "online install script finished"`,
			`fi`,
		)
	}
	lines = append(lines,
		"emit_step verify_tools running verify tool commands",
		requiredChecks,
		"VERIFY_EXIT=$?",
		`if [ "$VERIFY_EXIT" != "0" ]; then emit_step verify_tools failed "tool verification failed"; emit_result; exit "$VERIFY_EXIT"; fi`,
		`emit_step verify_tools success "tool verification ok"`,
		"emit_step compatibility_check running check profile compatibility",
		"emit_step compatibility_check success compatibility check finished",
		"emit_step smoke_test running run version smoke test",
		`for bin in mysql mysqlbinlog mariadb mariadb-binlog xtrabackup mariabackup mariadb-backup psql pg_basebackup pg_receivewal pg_combinebackup pg_verifybackup barman docker tar sha256sum zstd gzip rsync; do if command -v "$bin" >/dev/null 2>&1; then printf '%s: ' "$bin" >>"$LOG_PATH"; "$bin" --version >>"$LOG_PATH" 2>&1 || true; fi; done`,
		"emit_step smoke_test success version smoke test finished",
		"emit_result",
		"exit 0",
	)
	return strings.Join(lines, "\n")
}

func buildRunnerToolOfflineInstallLines(req *DatabaseRunnerToolInstallRequest, offlinePackage *DatabaseRunnerToolOfflinePackage, remotePackagePath string) []string {
	dryRunFlag := "0"
	if req != nil && req.DryRun {
		dryRunFlag = "1"
	}
	checksum := ""
	fileName := ""
	if offlinePackage != nil {
		checksum = offlinePackage.ChecksumSHA256
		fileName = offlinePackage.FileName
	}
	return []string{
		"emit_step install_packages running run offline package install",
		fmt.Sprintf("DRY_RUN=%s", dryRunFlag),
		"OFFLINE_PACKAGE=" + shellSingleQuote(remotePackagePath),
		"OFFLINE_CHECKSUM=" + shellSingleQuote(checksum),
		"OFFLINE_FILE_NAME=" + shellSingleQuote(fileName),
		`if [ "$DRY_RUN" = "1" ]; then`,
		`  emit_step install_packages skipped "dry-run: offline upload/install skipped"`,
		`else`,
		`  [ -f "$OFFLINE_PACKAGE" ] || { emit_step install_packages failed "offline package missing"; emit_result; exit 20; }`,
		`  echo "$OFFLINE_CHECKSUM  $OFFLINE_PACKAGE" | sha256sum -c - >>"$LOG_PATH" 2>&1 || { emit_step install_packages failed "offline package sha256 mismatch"; emit_result; exit 21; }`,
		`  EXTRACT_DIR="$JOB_DIR/offline-extract"; rm -rf "$EXTRACT_DIR"; mkdir -p "$EXTRACT_DIR"`,
		`  case "$OFFLINE_FILE_NAME" in`,
		`    *.tar.gz|*.tgz) tar -xzf "$OFFLINE_PACKAGE" -C "$EXTRACT_DIR" >>"$LOG_PATH" 2>&1 ;;`,
		`    *.tar.zst) tar --zstd -xf "$OFFLINE_PACKAGE" -C "$EXTRACT_DIR" >>"$LOG_PATH" 2>&1 ;;`,
		`    *.tar) tar -xf "$OFFLINE_PACKAGE" -C "$EXTRACT_DIR" >>"$LOG_PATH" 2>&1 ;;`,
		`    *) emit_step install_packages failed "unsupported offline archive format"; emit_result; exit 22 ;;`,
		`  esac`,
		`  INSTALL_SH="$(find "$EXTRACT_DIR" -maxdepth 3 -type f -name install.sh | head -n 1)"`,
		`  [ -n "$INSTALL_SH" ] || { emit_step install_packages failed "install.sh missing in offline package"; emit_result; exit 23; }`,
		`  chmod 700 "$INSTALL_SH"`,
		`  sh "$INSTALL_SH" --offline >>"$LOG_PATH" 2>&1`,
		`  INSTALL_EXIT=$?`,
		`  if [ "$INSTALL_EXIT" != "0" ]; then emit_step install_packages failed "offline install script exit $INSTALL_EXIT"; emit_result; exit "$INSTALL_EXIT"; fi`,
		`  emit_step install_packages success "offline install script finished"`,
		`fi`,
	}
}

func buildRunnerToolContainerInstallLines(req *DatabaseRunnerToolInstallRequest) []string {
	if req == nil {
		req = &DatabaseRunnerToolInstallRequest{}
	}
	dryRunFlag := "0"
	if req.DryRun {
		dryRunFlag = "1"
	}
	readOnly := "ro"
	if !req.ReadOnlyDatadir {
		readOnly = "rw"
	}
	return []string{
		"emit_step install_packages running verify containerized tool runtime",
		fmt.Sprintf("DRY_RUN=%s", dryRunFlag),
		"CONTAINER_TOOL_IMAGE=" + shellSingleQuote(req.ToolImage),
		"CONTAINER_TOOL_DIGEST=" + shellSingleQuote(req.ToolImageDigest),
		"CONTAINER_DATADIR=" + shellSingleQuote(req.DatadirMount),
		"CONTAINER_WORKDIR_MOUNT=" + shellSingleQuote(req.WorkdirMount),
		"CONTAINER_NETWORK_MODE=" + shellSingleQuote(normalizeContainerNetworkMode(req.NetworkMode)),
		"CONTAINER_DATADIR_MODE=" + shellSingleQuote(readOnly),
		`command -v docker >/dev/null 2>&1 || { emit_step install_packages failed "docker missing"; emit_result; exit 30; }`,
		`case "$CONTAINER_TOOL_IMAGE" in ""|*:latest) emit_step install_packages failed "container image must use explicit non-latest tag or digest"; emit_result; exit 31 ;; esac`,
		`if [ -n "$CONTAINER_DATADIR" ] && [ ! -d "$CONTAINER_DATADIR" ]; then emit_step install_packages failed "datadir mount path missing"; emit_result; exit 32; fi`,
		`if [ "$DRY_RUN" = "1" ]; then`,
		`  emit_step install_packages skipped "dry-run: docker image pull/check skipped"`,
		`else`,
		`  docker image inspect "$CONTAINER_TOOL_IMAGE" >/dev/null 2>&1 || docker pull "$CONTAINER_TOOL_IMAGE" >>"$LOG_PATH" 2>&1 || { emit_step install_packages failed "container image unavailable"; emit_result; exit 33; }`,
		`  if [ -n "$CONTAINER_TOOL_DIGEST" ]; then docker inspect --format='{{index .RepoDigests 0}}' "$CONTAINER_TOOL_IMAGE" 2>/dev/null | grep -q "$CONTAINER_TOOL_DIGEST" || { emit_step install_packages failed "container digest mismatch"; emit_result; exit 34; }; fi`,
		`  emit_step install_packages success "container image and mount precheck passed"`,
		`fi`,
	}
}

func buildRunnerToolRequiredCheckScript(profiles []string, dryRun bool) string {
	if dryRun {
		return `echo "dry-run skip verify" >>"$LOG_PATH"; true`
	}
	lines := []string{
		`missing=""`,
		`check_any() { label="$1"; shift; ok=""; for bin in "$@"; do if command -v "$bin" >/dev/null 2>&1; then ok="$bin"; break; fi; done; if [ -z "$ok" ]; then missing="$missing $label"; else echo "$label=$ok" >>"$LOG_PATH"; fi; }`,
	}
	if hasAnyProfile(profiles, "mysql_57_physical", "mysql_80_physical", "mysql_84_physical") {
		lines = append(lines, `check_any xtrabackup xtrabackup`)
	}
	if hasAnyProfile(profiles, "mariadb_physical") {
		lines = append(lines, `check_any mariadb_backup mariabackup mariadb-backup`)
	}
	if hasAnyProfile(profiles, "mysql_binlog_archiver") {
		lines = append(lines, `check_any mysqlbinlog mysqlbinlog mariadb-binlog`, `check_any mysql_client mysql mariadb`)
	}
	if hasAnyProfile(profiles, "postgres_barman") {
		lines = append(lines, `check_any barman barman`, `check_any psql psql`)
	}
	if hasAnyProfile(profiles, "postgres_native_pg_basebackup") {
		lines = append(lines, `check_any pg_basebackup pg_basebackup`, `check_any psql psql`)
	}
	if hasAnyProfile(profiles, "restore_runner") {
		lines = append(lines, `check_any docker docker`, `check_any tar tar`, `check_any sha256sum sha256sum`)
	}
	lines = append(lines, `[ -z "$missing" ] || { echo "missing:$missing" >>"$LOG_PATH"; false; }`)
	return strings.Join(lines, "\n")
}

func buildRunnerToolContainerRequiredCheckScript(req *DatabaseRunnerToolInstallRequest) string {
	if req != nil && req.DryRun {
		return `echo "dry-run skip container verify" >>"$LOG_PATH"; true`
	}
	readOnly := "ro"
	if req != nil && !req.ReadOnlyDatadir {
		readOnly = "rw"
	}
	lines := []string{
		"CONTAINER_TOOL_IMAGE=" + shellSingleQuote(req.ToolImage),
		"CONTAINER_DATADIR=" + shellSingleQuote(req.DatadirMount),
		"CONTAINER_NETWORK_MODE=" + shellSingleQuote(normalizeContainerNetworkMode(req.NetworkMode)),
		"CONTAINER_DATADIR_MODE=" + shellSingleQuote(readOnly),
		`RUN_ARGS="--rm --network $CONTAINER_NETWORK_MODE"`,
		`if [ -n "$CONTAINER_DATADIR" ]; then RUN_ARGS="$RUN_ARGS -v $CONTAINER_DATADIR:/var/lib/mysql:$CONTAINER_DATADIR_MODE"; fi`,
		`docker run $RUN_ARGS "$CONTAINER_TOOL_IMAGE" sh -lc 'ok=0; for bin in xtrabackup mariadb-backup mariabackup mysqlbinlog mariadb-binlog mysql mariadb psql pg_basebackup pg_receivewal pg_combinebackup pg_verifybackup barman tar sha256sum; do if command -v "$bin" >/dev/null 2>&1; then ok=1; printf "%s: " "$bin"; "$bin" --version 2>&1 | head -n 1 || true; fi; done; [ "$ok" = "1" ]' >>"$LOG_PATH" 2>&1`,
	}
	return strings.Join(lines, "\n")
}

func runnerToolProfilesRequireDatadir(profiles []string) bool {
	return hasAnyProfile(profiles, "mysql_57_physical", "mysql_80_physical", "mysql_84_physical", "mariadb_physical")
}

func buildRunnerToolInstallResult(host *DatabaseRunnerHost, req *DatabaseRunnerToolInstallRequest, started, finished time.Time, stdout, stderr string, exitCode int) runnerToolInstallResult {
	result := runnerToolInstallResult{
		RunnerHostID:  host.ID,
		RunnerID:      runnerIDForHost(host),
		Mode:          req.InstallMode,
		ExecutionMode: normalizeToolExecutionMode(req.ExecutionMode),
		ToolImage:     req.ToolImage,
		DryRun:        req.DryRun,
		Profiles:      req.Profiles,
		Stdout:        trimText(stdout, maxRunnerOutputLength),
		Stderr:        trimText(stderr, maxRunnerOutputLength),
		ExitCode:      exitCode,
		StartedAt:     started.Format("2006-01-02 15:04:05"),
		FinishedAt:    finished.Format("2006-01-02 15:04:05"),
		DurationMs:    finished.Sub(started).Milliseconds(),
	}
	if req != nil {
		result.OfflinePackageID = req.OfflinePackageID
	}
	result.Steps = parseRunnerToolInstallSteps(stdout)
	if resultPayload := parseRunnerToolInstallResult(stdout); resultPayload != nil {
		if logPath, _ := resultPayload["logPath"].(string); strings.TrimSpace(logPath) != "" {
			result.LogPath = logPath
		}
		if manifestPath, _ := resultPayload["manifestPath"].(string); strings.TrimSpace(manifestPath) != "" {
			result.ManifestPath = manifestPath
		}
	}
	return result
}

func parseRunnerToolInstallSteps(stdout string) []runnerToolInstallStep {
	steps := make([]runnerToolInstallStep, 0, 8)
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, runnerToolInstallStepMarker) {
			continue
		}
		body := strings.TrimPrefix(line, runnerToolInstallStepMarker)
		parts := strings.SplitN(body, "|", 3)
		step := runnerToolInstallStep{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			step.Status = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			step.Message = strings.TrimSpace(parts[2])
		}
		steps = append(steps, step)
	}
	return steps
}

func parseRunnerToolInstallResult(stdout string) map[string]any {
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, runnerToolInstallResultMarker) {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, runnerToolInstallResultMarker))
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			continue
		}
		out := map[string]any{}
		if err := json.Unmarshal(data, &out); err == nil {
			return out
		}
	}
	return nil
}

func validateRunnerToolOfflinePackageForProfile(profile *DatabaseRunnerToolProfile, offlinePackage *DatabaseRunnerToolOfflinePackage, profiles []string) error {
	if profile == nil || offlinePackage == nil {
		return fmt.Errorf("离线包和Runner画像不能为空")
	}
	if strings.TrimSpace(offlinePackage.StoragePath) == "" {
		return fmt.Errorf("离线包缺少本地存储路径")
	}
	stat, err := os.Stat(offlinePackage.StoragePath)
	if err != nil {
		return fmt.Errorf("离线包文件不可读: %w", err)
	}
	if stat.Size() != offlinePackage.FileSize {
		return fmt.Errorf("离线包文件大小与登记值不一致")
	}
	checksum, err := fileSHA256(offlinePackage.StoragePath)
	if err != nil {
		return fmt.Errorf("离线包校验失败: %w", err)
	}
	if checksum != strings.ToLower(strings.TrimSpace(offlinePackage.ChecksumSHA256)) {
		return fmt.Errorf("离线包 SHA256 与登记值不一致")
	}
	if offlinePackage.OSFamily != "" && !strings.EqualFold(offlinePackage.OSFamily, profile.OSFamily) {
		return fmt.Errorf("离线包 OS 家族不匹配：包=%s Runner=%s", offlinePackage.OSFamily, profile.OSFamily)
	}
	if offlinePackage.OSVersion != "" && !strings.HasPrefix(profile.OSVersion, offlinePackage.OSVersion) {
		return fmt.Errorf("离线包 OS 版本不匹配：包=%s Runner=%s", offlinePackage.OSVersion, profile.OSVersion)
	}
	if offlinePackage.Arch != "" && !runnerToolArchCompatible(offlinePackage.Arch, profile.Arch) {
		return fmt.Errorf("离线包架构不匹配：包=%s Runner=%s", offlinePackage.Arch, profile.Arch)
	}
	if offlinePackage.PackageManager != "" && !strings.EqualFold(offlinePackage.PackageManager, profile.PackageManager) {
		return fmt.Errorf("离线包包管理器不匹配：包=%s Runner=%s", offlinePackage.PackageManager, profile.PackageManager)
	}
	packageProfiles := parseRunnerToolOfflineProfiles(offlinePackage.ProfilesJSON)
	if len(packageProfiles) > 0 {
		allowed := make(map[string]struct{}, len(packageProfiles))
		for _, value := range packageProfiles {
			allowed[value] = struct{}{}
		}
		for _, value := range profiles {
			if _, ok := allowed[value]; !ok {
				return fmt.Errorf("离线包不包含工具 Profile：%s", value)
			}
		}
	}
	return nil
}

func uploadSSHRunnerFile(ctx context.Context, host string, port int, credential *ConnectionCredential, localPath, remotePath string, timeout time.Duration) error {
	if credential == nil {
		return fmt.Errorf("连接凭据不能为空")
	}
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if privateKey := strings.TrimSpace(credential.PrivateKey); privateKey != "" {
		signer, err := parseSSHPrivateKey(privateKey, credential.Passphrase)
		if err != nil {
			return err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if credential.Password != "" {
		authMethods = append(authMethods, ssh.Password(credential.Password))
	}
	if len(authMethods) == 0 {
		return fmt.Errorf("SSH 凭据必须包含密码或私钥")
	}
	if timeout <= 0 {
		timeout = time.Duration(defaultRunnerTimeoutMinutes) * time.Minute
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
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
		return fmt.Errorf("SSH认证失败: %w", err)
	}
	client := ssh.NewClient(clientConn, chans, reqs)
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("创建SFTP客户端失败: %w", err)
	}
	defer sftpClient.Close()
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()
	if err := sftpClient.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("创建远端目录失败: %w", err)
	}
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远端文件失败: %w", err)
	}
	defer remoteFile.Close()
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(remoteFile, localFile)
		done <- copyErr
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return fmt.Errorf("SFTP 上传超时")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("上传远端文件失败: %w", err)
		}
	}
	return nil
}

func toRunnerToolOfflinePackageVO(item *DatabaseRunnerToolOfflinePackage) *DatabaseRunnerToolOfflinePackageVO {
	if item == nil {
		return nil
	}
	return &DatabaseRunnerToolOfflinePackageVO{
		ID:             item.ID,
		Name:           item.Name,
		PackageVersion: item.PackageVersion,
		OSFamily:       item.OSFamily,
		OSVersion:      item.OSVersion,
		Arch:           item.Arch,
		PackageManager: item.PackageManager,
		Profiles:       parseRunnerToolOfflineProfiles(item.ProfilesJSON),
		FileName:       item.FileName,
		FileSize:       item.FileSize,
		ChecksumSHA256: item.ChecksumSHA256,
		StoragePath:    item.StoragePath,
		Status:         item.Status,
		ManifestJSON:   item.ManifestJSON,
		UploadedByID:   item.UploadedByID,
		UploadedByName: item.UploadedByName,
		UploadedAt:     formatTime(item.UploadedAt),
		LastVerifiedAt: formatTime(item.LastVerifiedAt),
		LastError:      item.LastError,
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func parseRunnerToolOfflineProfiles(raw string) []string {
	values := []string{}
	_ = json.Unmarshal([]byte(raw), &values)
	values = normalizeRunnerToolProfiles(values)
	sort.Strings(values)
	return values
}

func runnerToolArchCompatible(packageArch, runnerArch string) bool {
	normalize := func(value string) string {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "x86_64", "amd64":
			return "amd64"
		case "aarch64", "arm64":
			return "arm64"
		default:
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return normalize(packageArch) == "" || normalize(packageArch) == normalize(runnerArch)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func sanitizeRunnerToolPackageFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == "\\" {
		return ""
	}
	name = strings.ReplaceAll(name, "\x00", "")
	return trimText(name, 255)
}

func normalizeRunnerWorkDir(workDir string) string {
	if strings.TrimSpace(workDir) == "" {
		return defaultRunnerWorkDir
	}
	return strings.TrimSpace(workDir)
}

func firstNonNilError(values ...error) string {
	for _, err := range values {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}
