package database

import (
	"fmt"
	"strings"
)

const (
	DatabaseToolExecutionModeHost      = "host_tools"
	DatabaseToolExecutionModeContainer = "container_tools"
)

func normalizeToolExecutionMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "container", "docker", "container_tool", "container_tools":
		return DatabaseToolExecutionModeContainer
	default:
		return DatabaseToolExecutionModeHost
	}
}

func normalizeContainerNetworkMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "host"
	}
	return value
}

func validateContainerToolImage(image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return fmt.Errorf("容器工具镜像不能为空")
	}
	if strings.Contains(image, " ") || strings.ContainsAny(image, "\n\r\t") {
		return fmt.Errorf("容器工具镜像包含非法空白字符")
	}
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	hasTag := lastColon > lastSlash
	hasDigest := strings.Contains(image, "@sha256:")
	if !hasTag && !hasDigest {
		return fmt.Errorf("容器工具镜像必须使用显式 tag 或 sha256 digest，禁止使用隐式 latest")
	}
	if strings.HasSuffix(image, ":latest") || strings.Contains(image, ":latest@") {
		return fmt.Errorf("容器工具镜像禁止使用 latest，请固定具体版本 tag 或 digest")
	}
	return nil
}

func validateContainerNetworkMode(value string) error {
	value = normalizeContainerNetworkMode(value)
	if strings.ContainsAny(value, " \n\r\t;|&`$") {
		return fmt.Errorf("容器网络模式包含非法字符")
	}
	return nil
}

func backupPolicyContainerToolOptions(policy *DatabaseBackupPolicyConfig, host *DatabaseRunnerHost) containerToolOptions {
	if policy == nil || normalizeToolExecutionMode(policy.ToolExecutionMode) != DatabaseToolExecutionModeContainer {
		return containerToolOptions{}
	}
	workdirMount := strings.TrimSpace(policy.ContainerWorkdirPath)
	if workdirMount == "" && host != nil {
		workdirMount = firstNonEmpty(host.StorageMountPath, host.WorkDir, defaultRunnerWorkDir)
	}
	return containerToolOptions{
		Enabled:         true,
		Image:           strings.TrimSpace(policy.ToolImage),
		ImageDigest:     strings.TrimSpace(policy.ToolImageDigest),
		DatadirPath:     strings.TrimSpace(policy.ContainerDatadirPath),
		WorkdirPath:     workdirMount,
		NetworkMode:     normalizeContainerNetworkMode(policy.ContainerNetworkMode),
		DatadirReadOnly: policy.ContainerDatadirRO,
	}
}

type containerToolOptions struct {
	Enabled         bool
	Image           string
	ImageDigest     string
	DatadirPath     string
	WorkdirPath     string
	NetworkMode     string
	DatadirReadOnly bool
}

func appendContainerToolShellPrelude(lines *[]string, opts containerToolOptions, requireDatadir bool) {
	if lines == nil || !opts.Enabled {
		return
	}
	mountMode := "ro"
	if !opts.DatadirReadOnly {
		mountMode = "rw"
	}
	*lines = append(*lines,
		"CONTAINER_TOOL_IMAGE="+shellSingleQuote(opts.Image),
		"CONTAINER_TOOL_DIGEST="+shellSingleQuote(opts.ImageDigest),
		"CONTAINER_DATADIR="+shellSingleQuote(opts.DatadirPath),
		"CONTAINER_WORKDIR_MOUNT="+shellSingleQuote(opts.WorkdirPath),
		"CONTAINER_NETWORK_MODE="+shellSingleQuote(normalizeContainerNetworkMode(opts.NetworkMode)),
		"CONTAINER_DATADIR_MODE="+shellSingleQuote(mountMode),
		`DOCKER_BIN="$(command -v docker || true)"`,
		`if [ -z "$DOCKER_BIN" ]; then fail_step "container_precheck" "docker not found"; fi`,
		`case "$CONTAINER_TOOL_IMAGE" in ""|*:latest) fail_step "container_precheck" "container image must use explicit non-latest tag or digest" ;; esac`,
		`if [ -n "$CONTAINER_TOOL_DIGEST" ]; then "$DOCKER_BIN" image inspect --format='{{range .RepoDigests}}{{println .}}{{end}}' "$CONTAINER_TOOL_IMAGE" 2>/dev/null | grep -q "$CONTAINER_TOOL_DIGEST" || fail_step "container_precheck" "container image digest not matched or image not pulled"; fi`,
		`"$DOCKER_BIN" image inspect "$CONTAINER_TOOL_IMAGE" >/dev/null 2>&1 || "$DOCKER_BIN" pull "$CONTAINER_TOOL_IMAGE" >> "$LOG_FILE" 2>&1 || fail_step "container_precheck" "container image not available and pull failed"`,
		`if [ -n "$CONTAINER_WORKDIR_MOUNT" ] && [ ! -d "$CONTAINER_WORKDIR_MOUNT" ]; then mkdir -p "$CONTAINER_WORKDIR_MOUNT" || true; fi`,
	)
	if requireDatadir {
		*lines = append(*lines,
			`[ -n "$CONTAINER_DATADIR" ] || fail_step "container_precheck" "container datadir mount path is required"`,
			`[ -d "$CONTAINER_DATADIR" ] || fail_step "container_precheck" "container datadir mount path does not exist"`,
		)
	}
}
