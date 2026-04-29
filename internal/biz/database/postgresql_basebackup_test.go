package database

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildPgBaseBackupScriptKeepsCommandAndRedirectTogether(t *testing.T) {
	task := &DatabaseBackupTask{ScopeConfig: `{"runnerHostId":3}`}
	task.ID = 2
	record := &DatabaseBackupRecord{FileName: "pg-base.tar.gz"}
	record.ID = 8
	host := &DatabaseRunnerHost{WorkDir: "/var/lib/opshub/database-runner"}
	host.ID = 3
	script := buildPgBaseBackupScript(
		task,
		record,
		&DatabaseInstance{Host: "127.0.0.1", Port: 5432, Name: "pg-main"},
		host,
		&pgBaseBackupScopeConfig{RunnerHostID: 3, ExtraArgs: []string{"--max-rate=20M"}},
		&ConnectionCredential{Username: "backup", Password: "secret"},
	)
	for _, want := range []string{
		`"$PG_BASEBACKUP" -h "$PGHOST_VALUE" -p "$PGPORT_VALUE" -U "$PGUSER_VALUE" -D "$BASE_DIR" -Fp -X stream --checkpoint=fast --progress '--max-rate=20M' >> "$LOG_FILE" 2>&1`,
		`OPSHUB_STORAGE_URI=runner://runner-host-3`,
		`OPSHUB_PG_SYSTEM_IDENTIFIER`,
		`OPSHUB_BACKUP_MANIFEST_CHECKSUM`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildPostgreSQLRestorePlanJSONIncludesPgCombinebackup(t *testing.T) {
	result := &restorePlanValidationResult{
		BackupChainStatus: DatabaseBackupChainStatusComplete,
		LogChainStatus:    DatabaseLogChainStatusComplete,
		StorageStatus:     DatabaseStorageStatusAvailable,
		ToolStatus:        DatabaseToolStatusCompatible,
		ValidationStatus:  DatabasePlanValidationPassed,
		BackupProofs: []restoreProofBackup{
			{ID: 1, BackupMethod: DatabaseBackupMethodPhysical, BackupLevel: DatabaseBackupLevelFull, BackupEngine: BackupEnginePgBaseBackup},
			{ID: 2, BackupMethod: DatabaseBackupMethodPhysical, BackupLevel: DatabaseBackupLevelIncremental, BackupEngine: BackupEnginePgBaseBackup},
		},
	}
	targetTime := time.Date(2026, 4, 29, 10, 0, 0, 0, time.Local)
	planJSON := buildPostgreSQLRestorePlanJSON(&DatabaseInstance{DBType: DBTypePostgreSQL}, result, postgreSQLRestoreTarget{
		Type:  "time",
		Value: targetTime.Format("2006-01-02 15:04:05"),
		Time:  &targetTime,
	}, "isolated_restore")
	var payload map[string]any
	if err := json.Unmarshal([]byte(planJSON), &payload); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if payload["backupEngine"] != BackupEnginePgBaseBackup {
		t.Fatalf("backup engine=%v", payload["backupEngine"])
	}
	tools, _ := payload["requiredTools"].([]any)
	found := false
	for _, item := range tools {
		if item == "pg_combinebackup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("required tools should include pg_combinebackup: %v", tools)
	}
}

func TestValidatePgBaseBackupIncrementalMetadataRequiresParentAndManifest(t *testing.T) {
	base := &DatabaseBackupRecord{BackupLevel: DatabaseBackupLevelFull, BackupEngine: BackupEnginePgBaseBackup, BackupManifestChecksum: strings.Repeat("b", 64)}
	base.ID = 1
	inc := &DatabaseBackupRecord{
		BaseRecordID: 1,
		BackupLevel:  DatabaseBackupLevelIncremental,
		BackupEngine: BackupEnginePgBaseBackup,
	}
	inc.ID = 2
	status, message := validatePgBaseBackupIncrementalMetadata([]*DatabaseBackupRecord{base, inc})
	if status != DatabaseBackupChainStatusMissingIncremental || !strings.Contains(message, "parent_record_id") {
		t.Fatalf("expected missing parent, got status=%s message=%s", status, message)
	}

	inc.ParentRecordID = 1
	status, message = validatePgBaseBackupIncrementalMetadata([]*DatabaseBackupRecord{base, inc})
	if status != DatabaseBackupChainStatusMissingIncremental || !strings.Contains(message, "backup manifest") {
		t.Fatalf("expected missing manifest, got status=%s message=%s", status, message)
	}

	inc.BackupManifestChecksum = strings.Repeat("a", 64)
	status, message = validatePgBaseBackupIncrementalMetadata([]*DatabaseBackupRecord{base, inc})
	if status != DatabaseBackupChainStatusComplete || message != "" {
		t.Fatalf("expected complete, got status=%s message=%s", status, message)
	}
}

func TestValidateRequestedPostgreSQLBaseRecordRejectsWrongTargetTime(t *testing.T) {
	recoverableFrom := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local)
	targetTime := recoverableFrom.Add(-time.Minute)
	record := &DatabaseBackupRecord{
		InstanceID:      7,
		Status:          DatabaseBackupStatusSuccess,
		BackupMethod:    DatabaseBackupMethodPhysical,
		BackupLevel:     DatabaseBackupLevelFull,
		BackupEngine:    BackupEnginePgBaseBackup,
		StorageURI:      "runner://runner-host-1/pg-base.tar.gz",
		RecoverableFrom: &recoverableFrom,
	}
	record.ID = 5
	err := validateRequestedPostgreSQLBaseRecord(record, 7, postgreSQLRestoreTarget{
		Type:  "time",
		Value: targetTime.Format("2006-01-02 15:04:05"),
		Time:  &targetTime,
	})
	if err == nil || !strings.Contains(err.Error(), "可恢复起点晚于目标时间") {
		t.Fatalf("expected target time rejection, got %v", err)
	}
}
