package database

import (
	"testing"
	"time"
)

func TestNormalizeArchiveModeForDaemon(t *testing.T) {
	cases := map[string]string{
		"":                    DatabaseArchiveModeExternal,
		"manual":              DatabaseArchiveModeManual,
		"run_once":            DatabaseArchiveModeManual,
		"catchup":             DatabaseArchiveModeCatchUp,
		"high_frequency_poll": DatabaseArchiveModePolling,
		"poll":                DatabaseArchiveModePolling,
		"stream":              DatabaseArchiveModeStreaming,
	}
	for input, want := range cases {
		if got := normalizeArchiveMode(input); got != want {
			t.Fatalf("normalizeArchiveMode(%q) = %q, want %q", input, got, want)
		}
	}
	if got := daemonArchiveModeForStart(DatabaseArchiveModeExternal); got != DatabaseArchiveModePolling {
		t.Fatalf("daemonArchiveModeForStart(external) = %q", got)
	}
}

func TestApplyLogArchiveStreamStartState(t *testing.T) {
	stream := &DatabaseLogArchiveStream{
		Enabled:       true,
		Status:        DatabaseLogArchiveStreamStatusPaused,
		ArchiveMode:   DatabaseArchiveModeExternal,
		ArchiveEngine: "external_binlog",
		PausedReason:  "maintenance",
		LastError:     "old",
	}
	pausedAt := time.Now()
	stream.PausedAt = &pausedAt
	leaseExpiresAt := pausedAt.Add(time.Minute)
	stream.LeaseOwner = "runner-a"
	stream.LeaseExpiresAt = &leaseExpiresAt

	applyLogArchiveStreamStartState(stream, 12, DatabaseArchiveModeStreaming, "")

	if stream.RunnerHostID != 12 {
		t.Fatalf("RunnerHostID = %d", stream.RunnerHostID)
	}
	if stream.ArchiveMode != DatabaseArchiveModeStreaming || stream.ArchiveEngine != "mysqlbinlog_streaming" {
		t.Fatalf("archive mode/engine = %s/%s", stream.ArchiveMode, stream.ArchiveEngine)
	}
	if stream.DesiredState != DatabaseLogArchiveDesiredStateRunning || stream.DaemonStatus != DatabaseLogArchiveDaemonStatusStarting {
		t.Fatalf("state = %s/%s", stream.DesiredState, stream.DaemonStatus)
	}
	if stream.Status != DatabaseLogArchiveStreamStatusPending {
		t.Fatalf("status = %s", stream.Status)
	}
	if stream.PausedAt != nil || stream.PausedReason != "" || stream.LeaseOwner != "" || stream.LeaseExpiresAt != nil {
		t.Fatalf("pause/lease fields were not cleared")
	}
}

func TestApplyLogArchiveStreamPauseAndStopState(t *testing.T) {
	stream := &DatabaseLogArchiveStream{
		Enabled:      true,
		Status:       DatabaseLogArchiveStreamStatusRunning,
		DesiredState: DatabaseLogArchiveDesiredStateRunning,
		DaemonStatus: DatabaseLogArchiveDaemonStatusRunning,
		LeaseOwner:   "runner-a",
	}
	pausedAt := time.Date(2026, 4, 29, 1, 2, 3, 0, time.Local)
	applyLogArchiveStreamPauseState(stream, "maintenance", pausedAt)
	if stream.DesiredState != DatabaseLogArchiveDesiredStatePaused || stream.DaemonStatus != DatabaseLogArchiveDaemonStatusPaused {
		t.Fatalf("pause state = %s/%s", stream.DesiredState, stream.DaemonStatus)
	}
	if stream.Status != DatabaseLogArchiveStreamStatusPaused || stream.PausedAt == nil || stream.PausedReason != "maintenance" {
		t.Fatalf("pause fields = status %s at %v reason %q", stream.Status, stream.PausedAt, stream.PausedReason)
	}
	if stream.LeaseOwner != "" {
		t.Fatalf("lease owner not cleared")
	}

	applyLogArchiveStreamStopState(stream, "done")
	if stream.DesiredState != DatabaseLogArchiveDesiredStateStopped || stream.DaemonStatus != DatabaseLogArchiveDaemonStatusStopped {
		t.Fatalf("stop state = %s/%s", stream.DesiredState, stream.DaemonStatus)
	}
	if stream.Status != DatabaseLogArchiveStreamStatusPending || stream.PausedAt != nil || stream.PausedReason != "" {
		t.Fatalf("stop fields = status %s at %v reason %q", stream.Status, stream.PausedAt, stream.PausedReason)
	}
}
