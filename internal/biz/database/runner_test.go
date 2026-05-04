package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
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
	for _, want := range []string{"command -v xtrabackup || true", "command -v pg_basebackup || true", "command -v pg_combinebackup || true", "command -v docker || true"} {
		if !strings.Contains(command, want) {
			t.Fatalf("tool probe %q missing: %s", want, command)
		}
	}
	if !strings.Contains(command, "runner'\\''s") {
		t.Fatalf("work dir was not shell-quoted: %s", command)
	}
	if strings.Contains(command, "password") || strings.Contains(command, "secret") {
		t.Fatalf("probe command should not contain sensitive tokens: %s", command)
	}
}

func TestDeleteRunnerHostBlocksActiveReferences(t *testing.T) {
	repo := &runnerHostRepoForDeleteTest{
		host: &DatabaseRunnerHost{Model: gorm.Model{ID: 7}, Name: "runner-7"},
		blockers: &DatabaseRunnerHostDeleteBlockers{
			BackupPolicies:    1,
			LogArchiveStreams: 2,
			BackupArtifacts:   3,
		},
	}
	uc := &UseCase{runnerHostRepo: repo}
	err := uc.DeleteRunnerHost(context.Background(), 7)
	if err == nil {
		t.Fatalf("expected delete blocker error")
	}
	message := err.Error()
	for _, want := range []string{"备份策略", "日志归档流", "本地 artifact"} {
		if !strings.Contains(message, want) {
			t.Fatalf("delete blocker message %q missing %q", message, want)
		}
	}
	if repo.deletedID != 0 {
		t.Fatalf("delete should not be called, deleted id = %d", repo.deletedID)
	}
}

func TestDeleteRunnerHostDeletesWhenNoReferences(t *testing.T) {
	repo := &runnerHostRepoForDeleteTest{
		host:     &DatabaseRunnerHost{Model: gorm.Model{ID: 8}, Name: "runner-8"},
		blockers: &DatabaseRunnerHostDeleteBlockers{},
	}
	uc := &UseCase{runnerHostRepo: repo}
	if err := uc.DeleteRunnerHost(context.Background(), 8); err != nil {
		t.Fatalf("DeleteRunnerHost error = %v", err)
	}
	if repo.deletedID != 8 {
		t.Fatalf("deleted id = %d, want 8", repo.deletedID)
	}
}

type runnerHostRepoForDeleteTest struct {
	host      *DatabaseRunnerHost
	blockers  *DatabaseRunnerHostDeleteBlockers
	deletedID uint
}

func (r *runnerHostRepoForDeleteTest) Create(ctx context.Context, item *DatabaseRunnerHost) error {
	r.host = item
	return nil
}

func (r *runnerHostRepoForDeleteTest) Update(ctx context.Context, item *DatabaseRunnerHost) error {
	r.host = item
	return nil
}

func (r *runnerHostRepoForDeleteTest) Delete(ctx context.Context, id uint) error {
	r.deletedID = id
	return nil
}

func (r *runnerHostRepoForDeleteTest) GetByID(ctx context.Context, id uint) (*DatabaseRunnerHost, error) {
	if r.host == nil || r.host.ID != id {
		return nil, fmt.Errorf("not found")
	}
	return r.host, nil
}

func (r *runnerHostRepoForDeleteTest) List(ctx context.Context, req *DatabaseRunnerHostListRequest) ([]*DatabaseRunnerHost, int64, error) {
	if r.host == nil {
		return nil, 0, nil
	}
	return []*DatabaseRunnerHost{r.host}, 1, nil
}

func (r *runnerHostRepoForDeleteTest) CountDeleteBlockers(ctx context.Context, id uint) (*DatabaseRunnerHostDeleteBlockers, error) {
	if r.blockers == nil {
		return &DatabaseRunnerHostDeleteBlockers{}, nil
	}
	return r.blockers, nil
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
