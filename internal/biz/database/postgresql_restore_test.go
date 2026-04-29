package database

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func TestBuildPgBaseBackupRestoreScriptDirectoryOnly(t *testing.T) {
	script, err := buildPgBaseBackupRestoreScript(pgBaseBackupRestoreScriptInput{
		RestoreJobID:  21,
		RestorePlanID: 9,
		RunnerHostID:  3,
		WorkRoot:      "/var/lib/opshub/database-runner",
		Base: physicalRestoreArtifact{
			Kind:           "base",
			FileName:       "pg-base.tar.gz",
			SourcePath:     "/var/lib/opshub/database-runner/backup/pg-base.tar.gz",
			ChecksumSHA256: strings.Repeat("a", 64),
			FileSize:       1024,
		},
		TargetType:       "time",
		TargetValue:      "2026-04-29 10:00:00",
		TargetTimelineID: "1",
		StartInstance:    false,
		CleanupOnFailure: true,
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`START_INSTANCE=0`,
		`CLEANUP_ON_FAILURE=1`,
		`copy_artifact 'base' '/var/lib/opshub/database-runner/backup/pg-base.tar.gz' "$ARTIFACT_DIR/base.pg_basebackup.tar.gz"`,
		`tar -xzf "$ARTIFACT_DIR/base.pg_basebackup.tar.gz" -C "$PGDATA_DIR"`,
		`PG_VERIFYBACKUP="$(command -v pg_verifybackup || true)"`,
		`printf 'OPSHUB_VALIDATION_STATUS=warning\n'`,
		`OPSHUB_ARTIFACT_URI=runner://runner-host-3`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, `run_pg_validation 1`) {
		t.Fatalf("directory-only restore should not schedule validation SQL:\n%s", script)
	}
}

func TestPgBaseBackupRestoreDirectoryScriptSmoke(t *testing.T) {
	for _, tool := range []string{"bash", "tar", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source-pgdata")
	for _, dir := range []string{"global", "base", "pg_wal"} {
		if err := os.MkdirAll(filepath.Join(sourceDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "PG_VERSION"), []byte("16\n"), 0o644); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}
	artifact := filepath.Join(root, "pg-base.tar.gz")
	if out, err := exec.Command("tar", "-czf", artifact, "-C", sourceDir, ".").CombinedOutput(); err != nil {
		t.Fatalf("package artifact: %v\n%s", err, out)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	info, err := os.Stat(artifact)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	workRoot := filepath.Join(root, "runner")
	script, err := buildPgBaseBackupRestoreScript(pgBaseBackupRestoreScriptInput{
		RestoreJobID:  31,
		RestorePlanID: 13,
		RunnerHostID:  3,
		WorkRoot:      workRoot,
		Base: physicalRestoreArtifact{
			Kind:           "base",
			FileName:       "pg-base.tar.gz",
			SourcePath:     artifact,
			ChecksumSHA256: checksum,
			FileSize:       info.Size(),
		},
		TargetType:    "time",
		TargetValue:   "2026-04-29 10:00:00",
		StartInstance: false,
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("run restore script: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "OPSHUB_VALIDATION_STATUS=warning") {
		t.Fatalf("script output missing directory-only validation status:\n%s", out)
	}
	pgVersion := filepath.Join(workRoot, "restore", "job-31", "pgdata", "PG_VERSION")
	if _, err := os.Stat(pgVersion); err != nil {
		t.Fatalf("restored PG_VERSION missing: %v\n%s", err, out)
	}
	proof := filepath.Join(workRoot, "restore", "job-31", "proof.json")
	if _, err := os.Stat(proof); err != nil {
		t.Fatalf("proof missing: %v\n%s", err, out)
	}
}

func TestBuildPgBaseBackupRestoreScriptStartsPITRContainerAndValidates(t *testing.T) {
	expectedRows := 1
	expectedScalar := "1"
	script, err := buildPgBaseBackupRestoreScript(pgBaseBackupRestoreScriptInput{
		RestoreJobID:     22,
		RestorePlanID:    10,
		RunnerHostID:     3,
		WorkRoot:         "/var/lib/opshub/database-runner",
		ContainerName:    "opshub-pgbase-restore-22",
		ContainerImage:   "postgres:16",
		ListenPort:       25433,
		DatabaseName:     "postgres",
		DBUsername:       "postgres",
		DBPassword:       "secret",
		TargetType:       "lsn",
		TargetValue:      "0/3000000",
		TargetTimelineID: "00000003",
		TargetAction:     "pause",
		StartInstance:    true,
		Base: physicalRestoreArtifact{
			Kind:           "base",
			FileName:       "pg-base.tar.gz",
			SourcePath:     "/var/lib/opshub/database-runner/backup/pg-base.tar.gz",
			ChecksumSHA256: strings.Repeat("b", 64),
			FileSize:       1024,
		},
		Logs: []physicalRestoreArtifact{
			{
				Kind:           "wal",
				FileName:       "000000030000000000000001",
				SourcePath:     "/var/lib/opshub/database-runner/wal/000000030000000000000001",
				ChecksumSHA256: strings.Repeat("c", 64),
				FileSize:       16 * 1024 * 1024,
			},
		},
		ValidationChecks: []restoreValidationCheck{
			{SQL: "SELECT 1", ExpectedRows: &expectedRows, ExpectedScalar: &expectedScalar},
		},
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`START_INSTANCE=1`,
		`WAL_COUNT=1`,
		`copy_artifact 'wal_1' '/var/lib/opshub/database-runner/wal/000000030000000000000001' "$WAL_DIR/000000030000000000000001"`,
		`restore_command = 'cp ''%s/%%f'' ''%%p'''`,
		`recovery_target_lsn = '%s'`,
		`recovery_target_timeline = '%s'`,
		`"$DOCKER_BIN" run -d --name "$CONTAINER_NAME"`,
		`-p "127.0.0.1:$LISTEN_PORT:5432"`,
		`pg_wal_lsn_diff(pg_last_wal_replay_lsn(), '$TARGET_VALUE') >= 0`,
		`run_pg_validation 1 'SELECT 1' '1' '1' '0' '' '1' '1'`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildPgBaseBackupRestoreScriptCombinesIncrementals(t *testing.T) {
	script, err := buildPgBaseBackupRestoreScript(pgBaseBackupRestoreScriptInput{
		RestoreJobID:     23,
		RestorePlanID:    11,
		RunnerHostID:     3,
		WorkRoot:         "/var/lib/opshub/database-runner",
		ContainerName:    "opshub-pgbase-restore-23",
		ContainerImage:   "postgres:16",
		ListenPort:       25434,
		DatabaseName:     "postgres",
		DBUsername:       "postgres",
		TargetType:       "time",
		TargetValue:      "2026-04-29 10:00:00",
		TargetTimelineID: "00000001",
		TargetAction:     "pause",
		StartInstance:    false,
		Base: physicalRestoreArtifact{
			Kind:           "base",
			FileName:       "pg-base.tar.gz",
			SourcePath:     "/var/lib/opshub/database-runner/backup/pg-base.tar.gz",
			ChecksumSHA256: strings.Repeat("d", 64),
			FileSize:       1024,
		},
		Incrementals: []physicalRestoreArtifact{
			{
				Kind:           "incremental",
				FileName:       "pg-inc-1.tar.gz",
				SourcePath:     "/var/lib/opshub/database-runner/backup/pg-inc-1.tar.gz",
				ChecksumSHA256: strings.Repeat("e", 64),
				FileSize:       512,
			},
		},
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`INCREMENTAL_COUNT=1`,
		`copy_artifact 'incremental_1' '/var/lib/opshub/database-runner/backup/pg-inc-1.tar.gz' "$ARTIFACT_DIR/inc-1.pg_basebackup.tar.gz"`,
		`PG_COMBINEBACKUP="$(command -v pg_combinebackup || true)"`,
		`OPSHUB_COMBINEBACKUP_STATUS=missing_tool`,
		`"$PG_COMBINEBACKUP" -o "$PGDATA_DIR" "$COMBINE_BASE_DIR" "$COMBINE_DIR/inc-1"`,
		`OPSHUB_SYNTHETIC_FULL_PATH`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestParsePgBaseBackupRestoreOutputIncludesProofAndValidation(t *testing.T) {
	stdout := strings.Join([]string{
		"OPSHUB_WORK_DIR=/var/lib/opshub/database-runner/restore/job-22",
		"OPSHUB_PREPARED_DATADIR=/var/lib/opshub/database-runner/restore/job-22/pgdata",
		"OPSHUB_CONTAINER_NAME=opshub-pgbase-restore-22",
		"OPSHUB_CONTAINER_IMAGE=postgres:16",
		"OPSHUB_LISTEN_HOST=127.0.0.1",
		"OPSHUB_LISTEN_PORT=25433",
		"OPSHUB_PG_VERSION=16",
		"OPSHUB_VERIFYBACKUP_STATUS=success",
		"OPSHUB_COMBINEBACKUP_STATUS=success",
		"OPSHUB_PG_COMBINEBACKUP_VERSION=pg_combinebackup (PostgreSQL) 17.0",
		"OPSHUB_SYNTHETIC_FULL_PATH=/var/lib/opshub/database-runner/restore/job-22/pgdata",
		"OPSHUB_VALIDATION_STATUS=passed",
		"OPSHUB_RECOVERY_SUMMARY=t|0/3000000|2026-04-29 10:00:00+08",
		"OPSHUB_RESTORE_STEP=verify_pgdata|success|2026-04-29 10:00:00",
		"OPSHUB_RESTORE_VALIDATION=1|success|abcdef|MQo=|passed|1|MQ==|",
	}, "\n")
	result := parsePgBaseBackupRestoreOutput(stdout)
	if result.ContainerName != "opshub-pgbase-restore-22" || result.ListenPort != 25433 {
		t.Fatalf("container metadata not parsed: %+v", result)
	}
	if result.PGVersion != "16" || result.VerifyBackupStatus != "success" {
		t.Fatalf("backup verification metadata not parsed: %+v", result)
	}
	if result.CombineBackupStatus != "success" || !strings.Contains(result.CombineBackupVersion, "pg_combinebackup") {
		t.Fatalf("combine metadata not parsed: %+v", result)
	}
	if result.ValidationStatus != "passed" || len(result.ValidationResults) != 1 {
		t.Fatalf("validation not parsed: %+v", result)
	}
	if result.ValidationResults[0].AssertionStatus != "passed" || result.ValidationResults[0].ActualScalar != "1" {
		t.Fatalf("assertion metadata not parsed: %+v", result.ValidationResults[0])
	}
	if len(result.Steps) != 1 || result.Steps[0].Name != "verify_pgdata" {
		t.Fatalf("steps not parsed: %+v", result.Steps)
	}
}
