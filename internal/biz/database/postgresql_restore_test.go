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

func TestBuildBarmanRestoreScriptCanStartIsolatedPostgreSQLAndValidate(t *testing.T) {
	expectedRows := 1
	expectedScalar := "1"
	script, err := buildBarmanRestoreScript(barmanRestoreScriptInput{
		RestoreJobID:     12,
		RestorePlanID:    7,
		RunnerHostID:     3,
		WorkRoot:         "/var/lib/opshub/database-runner",
		ContainerName:    "opshub-pg-restore-12",
		ContainerImage:   "postgres:16",
		ListenPort:       25432,
		DatabaseName:     "postgres",
		DBUsername:       "postgres",
		DBPassword:       "secret",
		BarmanServerName: "pg-main",
		BackupID:         "20260429T010203",
		TargetType:       "time",
		TargetValue:      "2026-04-29 10:00:00",
		TargetAction:     "pause",
		GetWAL:           true,
		StartInstance:    true,
		ValidationChecks: []restoreValidationCheck{
			{SQL: "SELECT 1", ExpectedRows: &expectedRows, ExpectedScalar: &expectedScalar},
		},
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`START_INSTANCE=1`,
		`"$DOCKER_BIN" run -d --name "$CONTAINER_NAME"`,
		`-p "127.0.0.1:$LISTEN_PORT:5432"`,
		`run_pg_validation 1 'SELECT 1' '1' '1' '0' '' '1' '1'`,
		`OPSHUB_VALIDATION_STATUS=passed`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestParseBarmanRestoreOutputIncludesPostgreSQLValidationAssertions(t *testing.T) {
	stdout := strings.Join([]string{
		"OPSHUB_CONTAINER_NAME=opshub-pg-restore-12",
		"OPSHUB_CONTAINER_IMAGE=postgres:16",
		"OPSHUB_LISTEN_HOST=127.0.0.1",
		"OPSHUB_LISTEN_PORT=25432",
		"OPSHUB_VALIDATION_STATUS=passed",
		"OPSHUB_RECOVERY_SUMMARY=t|0/3000000",
		"OPSHUB_RESTORE_VALIDATION=1|success|abcdef|MQo=|passed|1|MQ==|",
	}, "\n")
	result := parseBarmanRestoreOutput(stdout)
	if result.ContainerName != "opshub-pg-restore-12" || result.ListenPort != 25432 {
		t.Fatalf("container metadata not parsed: %+v", result)
	}
	if result.ValidationStatus != "passed" || len(result.ValidationResults) != 1 {
		t.Fatalf("validation not parsed: %+v", result)
	}
	if result.ValidationResults[0].AssertionStatus != "passed" || result.ValidationResults[0].ActualScalar != "1" {
		t.Fatalf("assertion metadata not parsed: %+v", result.ValidationResults[0])
	}
}
