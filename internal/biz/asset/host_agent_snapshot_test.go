package asset

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ydcloud-dy/opshub/pkg/collector"
)

type fakeSnapshotAgentRepo struct {
	item *AssetAgent
	err  error
}

func (r *fakeSnapshotAgentRepo) Create(ctx context.Context, agent *AssetAgent) error {
	return nil
}

func (r *fakeSnapshotAgentRepo) Update(ctx context.Context, agent *AssetAgent) error {
	return nil
}

func (r *fakeSnapshotAgentRepo) GetByHostID(ctx context.Context, hostID uint) (*AssetAgent, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.item == nil {
		return nil, errors.New("not found")
	}
	return r.item, nil
}

func (r *fakeSnapshotAgentRepo) GetByAgentID(ctx context.Context, agentID string) (*AssetAgent, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeSnapshotAgentRepo) List(ctx context.Context, page, pageSize int, keyword string, accessibleHostIDs []uint, status string) ([]*AssetAgent, int64, error) {
	return nil, 0, nil
}

func (r *fakeSnapshotAgentRepo) DeleteByHostID(ctx context.Context, hostID uint) error {
	return nil
}

func TestCollectAgentSnapshotUsesLiveAgentEndpoint(t *testing.T) {
	const token = "agent-access-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != agentSnapshotPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, agentSnapshotPath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("authorization = %q, want bearer token", got)
		}

		_ = json.NewEncoder(w).Encode(AgentReportRequest{
			Hostname: "agent-host",
			OS:       "ubuntu 25.04",
			Kernel:   "6.14.0",
			Arch:     "x86_64",
			Uptime:   "1d 2h",
			CPU: collector.CPUInfo{
				Threads: 8,
				Usage:   42.5,
			},
			Memory: collector.MemoryInfo{
				Total: 16,
				Used:  8,
				Usage: 50,
			},
			Disk: []collector.DiskInfo{
				{Device: "/dev/sda1", MountPoint: "/", Fstype: "ext4", Total: 100, Used: 40, Usage: 40},
			},
		})
	}))
	defer server.Close()

	host, port := splitTestServerHostPort(t, server.URL)
	secretKey := []byte("jwt-secret")
	encryptedToken, err := encryptAgentSecret(secretKey, token)
	if err != nil {
		t.Fatalf("encryptAgentSecret() error = %v", err)
	}

	uc := &HostUseCase{
		agentRepo:      &fakeSnapshotAgentRepo{item: &AssetAgent{ListenPort: port, AccessToken: encryptedToken}},
		agentSecretKey: secretKey,
	}
	info, err := uc.collectAgentSnapshot(context.Background(), &Host{PrimaryPrivateIP: host})
	if err != nil {
		t.Fatalf("collectAgentSnapshot() error = %v", err)
	}
	if info.CPU.Usage != 42.5 || info.Memory.Usage != 50 {
		t.Fatalf("snapshot usages = cpu %.2f memory %.2f, want 42.5/50", info.CPU.Usage, info.Memory.Usage)
	}
	if info.Hostname != "agent-host" || len(info.Disk) != 1 {
		t.Fatalf("unexpected system info: %#v", info)
	}
}

func TestCollectAgentSnapshotReportsUnsupportedAgent(t *testing.T) {
	const token = "agent-access-token"

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	host, port := splitTestServerHostPort(t, server.URL)
	secretKey := []byte("jwt-secret")
	encryptedToken, err := encryptAgentSecret(secretKey, token)
	if err != nil {
		t.Fatalf("encryptAgentSecret() error = %v", err)
	}

	uc := &HostUseCase{
		agentRepo:      &fakeSnapshotAgentRepo{item: &AssetAgent{ListenPort: port, AccessToken: encryptedToken}},
		agentSecretKey: secretKey,
	}
	_, err = uc.collectAgentSnapshot(context.Background(), &Host{PrimaryPrivateIP: host})
	if err == nil || !strings.Contains(err.Error(), "版本过低") {
		t.Fatalf("collectAgentSnapshot() error = %v, want unsupported version", err)
	}
}

func splitTestServerHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split server host: %v", err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	return host, port
}
