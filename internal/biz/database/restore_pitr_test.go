package database

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizeRestoreValidationSQLRejectsWrite(t *testing.T) {
	_, err := normalizeRestoreValidationSQL(DBTypeMySQL, []string{"SELECT 1", "UPDATE users SET name='x'"})
	if err == nil {
		t.Fatalf("expected write SQL to be rejected")
	}
}

func TestNormalizeRestoreValidationChecksWithAssertions(t *testing.T) {
	expectedRows := 1
	expectedContains := "after-backup"
	expectedScalar := "2"
	checks, err := normalizeRestoreValidationChecks(DBTypeMySQL,
		[]string{"SELECT 1 AS restore_probe"},
		[]string{"SELECT 1 AS restore_probe"},
		[]DatabaseRestoreValidationAssertion{
			{SQL: "SELECT COUNT(*) FROM items", ExpectedScalar: &expectedScalar},
			{SQL: "SELECT note FROM items", ExpectedRows: &expectedRows, ExpectedContains: &expectedContains},
		},
	)
	if err != nil {
		t.Fatalf("normalize checks: %v", err)
	}
	if len(checks) != 3 {
		t.Fatalf("unexpected check count %d: %#v", len(checks), checks)
	}
	if checks[1].ExpectedScalar == nil || *checks[1].ExpectedScalar != "2" {
		t.Fatalf("expected scalar assertion on second check: %#v", checks[1])
	}
	if checks[2].ExpectedRows == nil || *checks[2].ExpectedRows != 1 || checks[2].ExpectedContains == nil || *checks[2].ExpectedContains != "after-backup" {
		t.Fatalf("expected row and contains assertion on third check: %#v", checks[2])
	}
}

func TestNormalizeRestoreValidationChecksRejectsInvalidAssertions(t *testing.T) {
	if _, err := normalizeRestoreValidationChecks(DBTypeMySQL, nil, nil, []DatabaseRestoreValidationAssertion{{SQL: "SELECT 1"}}); err == nil {
		t.Fatalf("expected empty assertion to fail")
	}
	expectedRows := -1
	if _, err := normalizeRestoreValidationChecks(DBTypeMySQL, nil, nil, []DatabaseRestoreValidationAssertion{{SQL: "SELECT 1", ExpectedRows: &expectedRows}}); err == nil {
		t.Fatalf("expected negative expectedRows to fail")
	}
	expectedScalar := "x"
	if _, err := normalizeRestoreValidationChecks(DBTypeMySQL, nil, nil, []DatabaseRestoreValidationAssertion{{SQL: "UPDATE users SET name='x'", ExpectedScalar: &expectedScalar}}); err == nil {
		t.Fatalf("expected write assertion SQL to fail")
	}
}

func TestResolveRunnerReadableArtifactPath(t *testing.T) {
	path, err := resolveRunnerReadableArtifactPath("runner://runner-host-7/backup/base.physical.tar.gz", "", 7)
	if err != nil {
		t.Fatalf("resolve runner URI: %v", err)
	}
	if path != "/backup/base.physical.tar.gz" {
		t.Fatalf("unexpected path %q", path)
	}
	if _, err := resolveRunnerReadableArtifactPath("runner://runner-host-8/backup/base.physical.tar.gz", "", 7); err == nil {
		t.Fatalf("expected mismatched runner URI to fail")
	}
	if _, err := resolveRunnerReadableArtifactPath("metadata://binlog/binlog.000001", "", 7); err == nil {
		t.Fatalf("expected metadata URI to fail")
	}
}

func TestBuildPhysicalRestoreScriptGuardsAndSteps(t *testing.T) {
	script, err := buildPhysicalRestoreScript(physicalRestoreScriptInput{
		RestoreJobID:    11,
		RestorePlanID:   22,
		RunnerHostID:    7,
		WorkRoot:        "/var/lib/opshub/database-runner",
		ContainerName:   "opshub-restore-22",
		ContainerImage:  "mysql:8.0",
		ListenPort:      24306,
		ToolName:        "xtrabackup",
		TargetTime:      "2026-04-29 10:00:00",
		BackupBinlogPos: 4,
		Base: physicalRestoreArtifact{
			Kind:           "base",
			ID:             1,
			FileName:       "base.physical.tar.gz",
			SourcePath:     "/backup/base.physical.tar.gz",
			ChecksumSHA256: strings.Repeat("a", 64),
			FileSize:       1024,
		},
		Logs: []physicalRestoreArtifact{{
			Kind:           "binlog",
			ID:             3,
			FileName:       "binlog.000001",
			SourcePath:     "/backup/binlog.000001",
			ChecksumSHA256: strings.Repeat("b", 64),
			FileSize:       2048,
		}},
		ValidationChecks: []restoreValidationCheck{{SQL: "SELECT 1 AS restore_probe", ExpectedScalar: stringPtrForRestoreTest("1")}},
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`copy_artifact 'base' '/backup/base.physical.tar.gz' "$ARTIFACT_DIR/base.physical.tar.gz"`,
		`step "prepare_physical_backup" "running"`,
		`step "start_isolated_instance" "running"`,
		`-e "SELECT 1" < /dev/null >/dev/null 2>&1`,
		`--stop-datetime="$TARGET_TIME" "$BINLOG_DIR/binlog.000001"`,
		`-e "$sql" < /dev/null > "$out" 2>&1`,
		`run_validation 1 'SELECT 1 AS restore_probe' '0' '' '0' '' '1' '1'`,
		`cleanup_failure()`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	for _, forbidden := range []string{"MYSQL_PWD", "--password"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("script exposes forbidden token %q", forbidden)
		}
	}
}

func TestParsePhysicalRestoreOutputWithAssertionFields(t *testing.T) {
	stdout := "OPSHUB_RESTORE_VALIDATION=2|failed|abc|" +
		base64.StdEncoding.EncodeToString([]byte("count\n1\n")) +
		"|failed|1|" +
		base64.StdEncoding.EncodeToString([]byte("1")) +
		"|" +
		base64.StdEncoding.EncodeToString([]byte("expectedScalar=2 actualScalar=1; ")) + "\n"
	result := parsePhysicalRestoreOutput(stdout)
	if len(result.ValidationResults) != 1 {
		t.Fatalf("expected one validation result: %#v", result.ValidationResults)
	}
	item := result.ValidationResults[0]
	if item.Index != 2 || item.Status != DatabaseBackupStatusFailed || item.AssertionStatus != DatabaseBackupStatusFailed {
		t.Fatalf("unexpected parsed validation item: %#v", item)
	}
	if item.ActualRows == nil || *item.ActualRows != 1 || item.ActualScalar != "1" {
		t.Fatalf("unexpected actual values: %#v", item)
	}
	if !strings.Contains(item.AssertionMessage, "expectedScalar=2") {
		t.Fatalf("unexpected assertion message: %#v", item)
	}
}

func TestBuildRestoreCleanupScriptRequiresRestoreWorkdir(t *testing.T) {
	if _, err := buildRestoreCleanupScript(&DatabaseRestoreJob{WorkDir: "/var/lib/mysql", ContainerName: "opshub-restore-1"}); err == nil {
		t.Fatalf("expected unsafe work dir to fail")
	}
	script, err := buildRestoreCleanupScript(&DatabaseRestoreJob{WorkDir: "/var/lib/opshub/database-runner/restore/job-9", ContainerName: "opshub-restore-9"})
	if err != nil {
		t.Fatalf("build cleanup script: %v", err)
	}
	if !strings.Contains(script, `rm -rf "$WORK_DIR"`) || !strings.Contains(script, `docker`) {
		t.Fatalf("cleanup script missing expected cleanup commands:\n%s", script)
	}
}

func stringPtrForRestoreTest(value string) *string {
	return &value
}
