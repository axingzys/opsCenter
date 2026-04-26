package asset

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestPVEUsernameCandidates(t *testing.T) {
	testCases := []struct {
		name     string
		username string
		want     []string
	}{
		{
			name:     "username with realm",
			username: "root@pam",
			want:     []string{"root@pam"},
		},
		{
			name:     "username without realm",
			username: "root",
			want:     []string{"root@pam", "root@pve", "root"},
		},
		{
			name:     "trim username",
			username: "  admin  ",
			want:     []string{"admin@pam", "admin@pve", "admin"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := pveUsernameCandidates(tc.username)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("unexpected candidates: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestPVECollectAuthFallback(t *testing.T) {
	loginAttempts := make([]string, 0, 3)

	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		username := r.Form.Get("username")
		loginAttempts = append(loginAttempts, username)

		if username != "root@pam" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null,"message":"authentication failure\n"}`))
			return
		}

		_, _ = w.Write([]byte(`{"data":{"ticket":"ticket-ok","CSRFPreventionToken":"csrf"}}`))
	})

	mux.HandleFunc("/api2/json/cluster/resources", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("PVEAuthCookie")
		if err != nil || cookie.Value != "ticket-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null,"message":"invalid credentials"}`))
			return
		}

		payload := pveResourcesResponse{
			Data: []pveResource{
				{Type: "cluster", ID: "cluster/prod", Name: "prod"},
				{Type: "node", ID: "node/node1", Node: "node1", Status: "online", MaxCPU: 16, MaxMem: 34359738368, Mem: 8589934592},
				{Type: "qemu", ID: "qemu/100", Name: "vm-100", VMID: 100, Node: "node1", Status: "running", MaxCPU: 4, MaxMem: 4294967296, Tags: "192.168.1.20"},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	})

	mux.HandleFunc("/api2/json/nodes/node1/qemu/100/config", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("PVEAuthCookie")
		if err != nil || cookie.Value != "ticket-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null,"message":"invalid credentials"}`))
			return
		}

		_, _ = w.Write([]byte(`{"data":{"name":"vm-100","ostype":"l26","tags":"192.168.1.20"}}`))
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url failed: %v", err)
	}

	adapter := &pveAdapter{}
	platform := &VirtualizationPlatform{
		Name:               "pve-test",
		Endpoint:           server.URL,
		Port:               443,
		Username:           "root",
		Password:           "pass",
		InsecureSkipVerify: true,
	}

	snapshot, err := adapter.Collect(context.Background(), platform)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(snapshot.Clusters) != 1 || len(snapshot.Hosts) != 1 || len(snapshot.Guests) != 1 {
		t.Fatalf("unexpected snapshot size: clusters=%d hosts=%d guests=%d", len(snapshot.Clusters), len(snapshot.Hosts), len(snapshot.Guests))
	}
	if snapshot.Guests[0].PowerState != "powered_on" {
		t.Fatalf("unexpected guest power state: %s", snapshot.Guests[0].PowerState)
	}
	if snapshot.Guests[0].PrimaryIP != "192.168.1.20" {
		t.Fatalf("unexpected guest ip: %s", snapshot.Guests[0].PrimaryIP)
	}
	if snapshot.Guests[0].OSType != "Linux" {
		t.Fatalf("unexpected guest os type: %s", snapshot.Guests[0].OSType)
	}

	if len(loginAttempts) == 0 {
		t.Fatalf("expected login attempts")
	}
	if loginAttempts[0] != "root@pam" {
		t.Fatalf("unexpected first login attempt: %v", loginAttempts)
	}
	if !slices.Contains(loginAttempts, "root@pam") {
		t.Fatalf("expected root@pam in attempts: %v", loginAttempts)
	}
	if strings.TrimSpace(u.Host) == "" {
		t.Fatalf("invalid server host")
	}
}

func TestExtractPrimaryIPFromPVETags(t *testing.T) {
	if got := extractPrimaryIPFromPVETags("env:prod;192.168.1.12;role:docker"); got != "192.168.1.12" {
		t.Fatalf("unexpected ip from tags: %s", got)
	}
	if got := extractPrimaryIPFromPVETags("role:web,ip=10.0.0.8"); got != "10.0.0.8" {
		t.Fatalf("unexpected ip from tags with prefix: %s", got)
	}
}
