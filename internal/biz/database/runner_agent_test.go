package database

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestRunnerAgentAuthMatchesHashOnly(t *testing.T) {
	sum := sha256.Sum256([]byte("runner-auth-value"))
	config := `{"runnerAuthSha256":"` + hex.EncodeToString(sum[:]) + `"}`
	if !runnerAgentAuthMatches(config, "runner-auth-value") {
		t.Fatalf("expected runner auth to match sha256 config")
	}
	if runnerAgentAuthMatches(config, "wrong") {
		t.Fatalf("unexpected runner auth match")
	}
	if runnerAgentAuthMatches(`{"runnerAuthSha256":""}`, "runner-auth-value") {
		t.Fatalf("empty hash should not match")
	}
	if strings.Contains(strings.ToLower(config), "token") || strings.Contains(strings.ToLower(config), "secret") {
		t.Fatalf("test config should use non-sensitive key naming: %s", config)
	}
}

func TestApplyRunnerAgentHeartbeat(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	host := &DatabaseRunnerHost{Status: DatabaseRunnerHostStatusPending, LastError: "old"}
	applyRunnerAgentHeartbeat(host, &DatabaseRunnerAgentHeartbeatRequest{Status: DatabaseRunnerHostStatusOnline}, now)
	if host.Status != DatabaseRunnerHostStatusOnline || host.LastHeartbeatAt == nil || !host.LastHeartbeatAt.Equal(now) || host.LastError != "" {
		t.Fatalf("unexpected online heartbeat: %#v", host)
	}

	applyRunnerAgentHeartbeat(host, &DatabaseRunnerAgentHeartbeatRequest{Status: DatabaseRunnerHostStatusFailed, Message: "disk full"}, now.Add(time.Minute))
	if host.Status != DatabaseRunnerHostStatusFailed || host.LastError != "disk full" {
		t.Fatalf("unexpected failed heartbeat: %#v", host)
	}
}

func TestApplyRunnerAgentCheckpointRunningAndLagDegraded(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	lastEvent := now.Add(-2 * time.Minute)
	stream := &DatabaseLogArchiveStream{
		RPOTargetSeconds: 30,
		LeaseOwner:       "agent:runner-a",
		DesiredState:     DatabaseLogArchiveDesiredStateRunning,
	}
	applyRunnerAgentCheckpoint(stream, &DatabaseRunnerAgentCheckpointRequest{
		DaemonStatus:      DatabaseLogArchiveDaemonStatusRunning,
		CursorFile:        "binlog.000010",
		CursorPos:         1200,
		LastSourceFile:    "binlog.000011",
		LastSourcePos:     88,
		ArchiveLagSeconds: 90,
	}, &lastEvent, now, "agent:runner-a")

	if stream.CursorFile != "binlog.000010" || stream.CursorPos != 1200 {
		t.Fatalf("cursor not applied: %#v", stream)
	}
	if stream.Status != DatabaseLogArchiveStreamStatusDegraded || stream.DaemonStatus != DatabaseLogArchiveDaemonStatusDegraded {
		t.Fatalf("lag should degrade stream, status=%s daemon=%s", stream.Status, stream.DaemonStatus)
	}
	if stream.LeaseExpiresAt == nil || !stream.LeaseExpiresAt.After(now) {
		t.Fatalf("lease was not renewed")
	}
}

func TestApplyRunnerAgentCheckpointReleaseLease(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	leaseExpiresAt := now.Add(time.Minute)
	stream := &DatabaseLogArchiveStream{
		Enabled:        true,
		Status:         DatabaseLogArchiveStreamStatusRunning,
		DesiredState:   DatabaseLogArchiveDesiredStateRunning,
		DaemonStatus:   DatabaseLogArchiveDaemonStatusRunning,
		LeaseOwner:     "agent:runner-a",
		LeaseExpiresAt: &leaseExpiresAt,
	}
	applyRunnerAgentCheckpoint(stream, &DatabaseRunnerAgentCheckpointRequest{ReleaseLease: true}, nil, now, "agent:runner-a")
	if stream.LeaseOwner != "" || stream.LeaseExpiresAt != nil {
		t.Fatalf("lease not released: %#v", stream)
	}
	if stream.DaemonStatus != DatabaseLogArchiveDaemonStatusStopped || stream.Status != DatabaseLogArchiveStreamStatusPending {
		t.Fatalf("unexpected stopped state: %s/%s", stream.DaemonStatus, stream.Status)
	}
}
