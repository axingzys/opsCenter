package asset

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolvePrometheusTargetIP(t *testing.T) {
	tests := []struct {
		name string
		host *Host
		want string
	}{
		{
			name: "cloud host prefers primary public ip",
			host: &Host{
				Type:             "cloud",
				IP:               "8.138.94.218",
				PrimaryPrivateIP: "172.24.154.111",
				PrimaryPublicIP:  "8.138.94.218",
			},
			want: "8.138.94.218",
		},
		{
			name: "cloud host falls back to public management ip",
			host: &Host{
				Type:             "cloud",
				IP:               "47.96.10.20",
				PrimaryPrivateIP: "172.24.154.111",
			},
			want: "47.96.10.20",
		},
		{
			name: "cloud host falls back to private ip when no public address exists",
			host: &Host{
				Type:             "cloud",
				IP:               "172.24.154.111",
				PrimaryPrivateIP: "172.24.154.111",
			},
			want: "172.24.154.111",
		},
		{
			name: "non cloud host keeps private ip priority",
			host: &Host{
				Type:             "self",
				IP:               "203.132.61.87",
				PrimaryPrivateIP: "192.168.1.12",
				PrimaryPublicIP:  "203.132.61.87",
			},
			want: "192.168.1.12",
		},
		{
			name: "non cloud host falls back to management ip",
			host: &Host{
				Type: "self",
				IP:   "192.168.1.15",
			},
			want: "192.168.1.15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePrometheusTargetIP(tt.host); got != tt.want {
				t.Fatalf("resolvePrometheusTargetIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldCleanupWindowsAgentWithoutRemote(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name               string
		host               *Host
		agent              *AssetAgent
		missingAgentRecord bool
		want               bool
	}{
		{
			name: "failed deploy with no runtime identity can be cleaned locally",
			host: &Host{OSType: OSTypeWindows},
			agent: &AssetAgent{
				Status:          AgentStatusError,
				InstallProgress: 25,
				InstallStage:    "configuring",
			},
			want: true,
		},
		{
			name:               "missing record with no host runtime identity can be cleaned locally",
			host:               &Host{OSType: OSTypeWindows},
			want:               true,
			missingAgentRecord: true,
		},
		{
			name: "waiting registration still needs remote uninstall attempt",
			host: &Host{OSType: OSTypeWindows},
			agent: &AssetAgent{
				Status:          AgentStatusWaiting,
				InstallProgress: 90,
				InstallStage:    "waiting_register",
			},
			want: false,
		},
		{
			name: "registered agent id prevents local-only cleanup",
			host: &Host{OSType: OSTypeWindows},
			agent: &AssetAgent{
				AgentID: "agent-1",
				Status:  AgentStatusError,
			},
			want: false,
		},
		{
			name: "host heartbeat prevents local-only cleanup even when record is missing",
			host: &Host{
				OSType:               OSTypeWindows,
				AgentLastHeartbeatAt: &now,
			},
			want:               false,
			missingAgentRecord: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCleanupWindowsAgentWithoutRemote(tt.host, tt.agent, tt.missingAgentRecord); got != tt.want {
				t.Fatalf("shouldCleanupWindowsAgentWithoutRemote() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldCleanupWindowsAgentAfterRemoteUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "missing WinRM credential can be cleaned locally",
			err:  errors.New("获取 WinRM 凭证失败: record not found"),
			want: true,
		},
		{
			name: "connection refused can be cleaned locally",
			err:  errors.New(`WinRM执行失败: dial tcp 192.168.1.9:5589: connect: connection refused`),
			want: true,
		},
		{
			name: "unrelated script failure should not be cleaned locally",
			err:  errors.New("PowerShell 脚本退出码 1"),
			want: false,
		},
		{
			name: "nil error should not be cleaned locally",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCleanupWindowsAgentAfterRemoteUnavailable(tt.err); got != tt.want {
				t.Fatalf("shouldCleanupWindowsAgentAfterRemoteUnavailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRevokedAgentID(t *testing.T) {
	got := buildRevokedAgentID(&Host{AgentID: "host-agent"}, &AssetAgent{AgentID: "agent-record"})
	if got != "revoked:agent-record" {
		t.Fatalf("buildRevokedAgentID() = %q, want %q", got, "revoked:agent-record")
	}

	got = buildRevokedAgentID(&Host{AgentID: "host-agent"}, nil)
	if got != "revoked:host-agent" {
		t.Fatalf("buildRevokedAgentID() = %q, want %q", got, "revoked:host-agent")
	}

	got = buildRevokedAgentID(nil, nil)
	if !strings.HasPrefix(got, "revoked:") {
		t.Fatalf("buildRevokedAgentID() = %q, want revoked prefix", got)
	}
}
