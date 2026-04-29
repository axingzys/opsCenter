package database

import (
	"strings"
	"testing"
)

func TestValidatePostgreSQLLSNWALCoverageDetectsMissingSegment(t *testing.T) {
	base := &DatabaseBackupRecord{
		WALEnd:         "000000010000000000000001",
		EndLSN:         "0/1000000",
		TimelineID:     "1",
		WALSegmentSize: 16777216,
	}
	target, err := normalizePostgreSQLRestoreTarget("lsn", "0/3000000", "1")
	if err != nil {
		t.Fatalf("normalize target: %v", err)
	}
	status, ids, message := validatePostgreSQLLSNWALCoverage(base, []*DatabaseLogArchive{
		{FileName: "000000010000000000000001", TimelineID: "00000001", WALSegmentSize: 16777216, Status: DatabaseLogArchiveStatusArchived},
		{FileName: "000000010000000000000003", TimelineID: "00000001", WALSegmentSize: 16777216, Status: DatabaseLogArchiveStatusArchived},
	}, target, "00000001", 16777216)
	if status != DatabaseLogChainStatusMissingWAL {
		t.Fatalf("status=%s, want %s, ids=%v message=%s", status, DatabaseLogChainStatusMissingWAL, ids, message)
	}
	if !strings.Contains(message, "000000010000000000000002") {
		t.Fatalf("missing segment message not specific: %s", message)
	}
}

func TestValidatePostgreSQLTimelineHistoryRequiresHistoryFile(t *testing.T) {
	status, message := validatePostgreSQLTimelineHistory([]*DatabaseLogArchive{
		{FileName: "000000030000000000000001", TimelineID: "00000003", Status: DatabaseLogArchiveStatusArchived},
	}, "3")
	if status != DatabaseLogChainStatusTimelineGap {
		t.Fatalf("status=%s, want %s, message=%s", status, DatabaseLogChainStatusTimelineGap, message)
	}

	status, _ = validatePostgreSQLTimelineHistory([]*DatabaseLogArchive{
		{FileName: "00000003.history", TimelineID: "00000003", Status: DatabaseLogArchiveStatusArchived},
		{FileName: "000000030000000000000001", TimelineID: "00000003", Status: DatabaseLogArchiveStatusArchived},
	}, "3")
	if status != DatabaseLogChainStatusComplete {
		t.Fatalf("status=%s, want complete", status)
	}
}

func TestBuildBarmanRestoreScriptIsConstrained(t *testing.T) {
	script, err := buildBarmanRestoreScript(barmanRestoreScriptInput{
		RestoreJobID:     12,
		RestorePlanID:    7,
		RunnerHostID:     3,
		WorkRoot:         "/var/lib/opshub/database-runner",
		BarmanServerName: "pg-main",
		BackupID:         "20260429T010203",
		TargetType:       "lsn",
		TargetValue:      "A/18000098",
		TargetTimelineID: "3",
		TargetAction:     "pause",
		GetWAL:           true,
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{`--target-lsn "$TARGET_VALUE"`, `--target-tli "$BARMAN_TARGET_TLI"`, `--target-action "$TARGET_ACTION"`, `--get-wal`, `"$WORK_ROOT"/restore/job-"$RESTORE_JOB_ID"/pgdata`} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	if !strings.Contains(script, `BARMAN_TARGET_TLI='3'`) {
		t.Fatalf("script should pass decimal target timeline to barman:\n%s", script)
	}
}
