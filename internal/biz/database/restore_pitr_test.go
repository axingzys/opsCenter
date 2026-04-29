package database

import (
	"strings"
	"testing"
)

func TestNormalizeRestoreValidationSQLRejectsWrite(t *testing.T) {
	_, err := normalizeRestoreValidationSQL(DBTypeMySQL, []string{"SELECT 1", "UPDATE users SET name='x'"})
	if err == nil {
		t.Fatalf("expected write SQL to be rejected")
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
		ValidationSQL: []string{"SELECT 1 AS restore_probe"},
	})
	if err != nil {
		t.Fatalf("build script: %v", err)
	}
	for _, want := range []string{
		`copy_artifact 'base' '/backup/base.physical.tar.gz' "$ARTIFACT_DIR/base.physical.tar.gz"`,
		`step "prepare_physical_backup" "running"`,
		`step "start_isolated_instance" "running"`,
		`--stop-datetime="$TARGET_TIME" "$BINLOG_DIR/binlog.000001"`,
		`run_validation 1 'SELECT 1 AS restore_probe'`,
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
