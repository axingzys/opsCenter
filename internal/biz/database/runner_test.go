package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildRunnerHostFromRequestDefaults(t *testing.T) {
	uc := &UseCase{
		credentialIDExists: func(ctx context.Context, id uint) error {
			if id != 7 {
				t.Fatalf("credential id = %d, want 7", id)
			}
			return nil
		},
	}
	item, err := uc.buildRunnerHostFromRequest(context.Background(), nil, &DatabaseRunnerHostRequest{
		Name:         "backup-runner",
		Host:         "192.168.1.15",
		CredentialID: 7,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("buildRunnerHostFromRequest error = %v", err)
	}
	if item.RunnerType != DatabaseRunnerTypeSSH {
		t.Fatalf("runner type = %s, want ssh", item.RunnerType)
	}
	if item.Port != 22 {
		t.Fatalf("port = %d, want 22", item.Port)
	}
	if item.WorkDir != defaultRunnerWorkDir {
		t.Fatalf("work dir = %s, want %s", item.WorkDir, defaultRunnerWorkDir)
	}
	if item.MaxConcurrentJobs != 1 {
		t.Fatalf("max concurrent = %d, want 1", item.MaxConcurrentJobs)
	}
	if item.TimeoutMinutes != defaultRunnerTimeoutMinutes {
		t.Fatalf("timeout = %d, want %d", item.TimeoutMinutes, defaultRunnerTimeoutMinutes)
	}
	if item.Status != DatabaseRunnerHostStatusPending {
		t.Fatalf("status = %s, want pending", item.Status)
	}
}

func TestBuildRunnerHostFromRequestRejectsSensitiveConfig(t *testing.T) {
	uc := &UseCase{}
	_, err := uc.buildRunnerHostFromRequest(context.Background(), nil, &DatabaseRunnerHostRequest{
		Name:         "backup-runner",
		Host:         "192.168.1.15",
		CredentialID: 7,
		Enabled:      true,
		ConfigJSON:   `{"access_key":"AKIA"}`,
	})
	if err == nil {
		t.Fatalf("expected sensitive config error")
	}
}

func TestBuildRunnerProbeCommandIsFixedAndQuoted(t *testing.T) {
	command := buildRunnerProbeCommand("/tmp/opshub runner's work")
	if !strings.Contains(command, "opshub-runner-ok") {
		t.Fatalf("probe marker missing: %s", command)
	}
	if !strings.Contains(command, "command -v xtrabackup || true") {
		t.Fatalf("tool probe missing: %s", command)
	}
	if !strings.Contains(command, "runner'\\''s") {
		t.Fatalf("work dir was not shell-quoted: %s", command)
	}
	if strings.Contains(command, "password") || strings.Contains(command, "secret") {
		t.Fatalf("probe command should not contain sensitive tokens: %s", command)
	}
}

func TestRunnerRequestJSONDoesNotExposeCredential(t *testing.T) {
	payload := runnerRequestJSON(&DatabaseRunnerHost{
		Name:         "runner",
		RunnerType:   DatabaseRunnerTypeSSH,
		Host:         "192.168.1.15",
		Port:         22,
		CredentialID: 9,
		WorkDir:      defaultRunnerWorkDir,
	}, QueryOperator{ID: 1, Username: "admin"})
	if strings.Contains(payload, "credential") || strings.Contains(payload, "password") || strings.Contains(payload, "secret") {
		t.Fatalf("request json exposes sensitive fields: %s", payload)
	}
}

func TestApplyRunnerProbeResultUpdatesSuccessAndFailure(t *testing.T) {
	started := time.Date(2026, 4, 29, 1, 0, 0, 0, time.Local)
	finished := started.Add(2 * time.Second)
	host := &DatabaseRunnerHost{RunnerType: DatabaseRunnerTypeSSH, Host: "192.168.1.15", Port: 22}
	job := &DatabaseRunnerJob{}
	applyRunnerProbeResult(host, job, started, finished, "ok", "", 0, nil)
	if host.Status != DatabaseRunnerHostStatusOnline || job.Status != DatabaseRunnerJobStatusSuccess {
		t.Fatalf("success status host=%s job=%s", host.Status, job.Status)
	}
	if job.DurationMs != 2000 {
		t.Fatalf("duration = %d, want 2000", job.DurationMs)
	}
	applyRunnerProbeResult(host, job, started, finished, "", "boom", 1, fmt.Errorf("probe failed"))
	if host.Status != DatabaseRunnerHostStatusFailed || job.Status != DatabaseRunnerJobStatusFailed {
		t.Fatalf("failure status host=%s job=%s", host.Status, job.Status)
	}
	if host.LastError == "" || job.ErrorMessage == "" {
		t.Fatalf("failure error was not recorded")
	}
}
