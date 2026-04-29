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

func TestBuildRestoreRequiredToolJSONPgBaseBackupIncludesExecutionTools(t *testing.T) {
	payload := buildRestoreRequiredToolJSON(&DatabaseInstance{DBType: DBTypePostgreSQL}, &restorePlanValidationResult{
		BackupProofs: []restoreProofBackup{
			{ID: 1, BackupMethod: DatabaseBackupMethodPhysical, BackupLevel: DatabaseBackupLevelFull, BackupEngine: BackupEnginePgBaseBackup},
			{ID: 2, BackupMethod: DatabaseBackupMethodPhysical, BackupLevel: DatabaseBackupLevelIncremental, BackupEngine: BackupEnginePgBaseBackup},
		},
	})
	var tools []map[string]any
	if err := json.Unmarshal([]byte(payload), &tools); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}
	names := map[string]bool{}
	for _, item := range tools {
		names[item["name"].(string)] = true
	}
	for _, want := range []string{"pg_basebackup", "pg_combinebackup", "docker", "psql", "tar", "sha256sum", "pg_verifybackup"} {
		if !names[want] {
			t.Fatalf("required tool %s not found in %#v", want, tools)
		}
	}
}

func TestBuildRestoreRequiredArtifactJSONIncludesRunnerReadiness(t *testing.T) {
	payload := buildRestoreRequiredArtifactJSON(&restorePlanValidationResult{
		RunnerHostID: 3,
		BackupProofs: []restoreProofBackup{
			{
				ID:           1,
				FileName:     "pg-base.tar.gz",
				StorageURI:   "runner://runner-host-3/backups/pg-base.tar.gz",
				BackupLevel:  DatabaseBackupLevelFull,
				BackupEngine: BackupEnginePgBaseBackup,
			},
		},
		LogProofs: []restoreProofLogArchive{
			{
				ID:          9,
				ArchiveType: DatabaseArchiveTypeWAL,
				FileName:    "000000010000000000000001",
				StorageURI:  "runner://runner-host-3/wal/000000010000000000000001",
			},
		},
	})
	var artifacts []map[string]any
	if err := json.Unmarshal([]byte(payload), &artifacts); err != nil {
		t.Fatalf("unmarshal artifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected two artifacts, got %#v", artifacts)
	}
	for _, item := range artifacts {
		if item["runnerReadable"] != true || item["readinessStatus"] != "ready" {
			t.Fatalf("artifact should be runner ready: %#v", item)
		}
		if !strings.HasPrefix(item["runnerPath"].(string), "/") {
			t.Fatalf("runner path should be absolute: %#v", item)
		}
	}
}

func TestRestoreArtifactReadinessMarksObjectStoragePendingDownload(t *testing.T) {
	status, runnerPath, message := restoreArtifactReadiness(3, "s3://bucket/path/pg-base.tar.gz", "", BackupEnginePgBaseBackup)
	if status != "pending_download" || runnerPath != "" || !strings.Contains(message, "Runner staging") {
		t.Fatalf("expected pending object storage download, got status=%s path=%s message=%s", status, runnerPath, message)
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

func TestNormalizePostgreSQLExternalBackupEngines(t *testing.T) {
	cases := map[string]string{
		"wal-g":       BackupEngineWALG,
		"wal_g":       BackupEngineWALG,
		"WALG":        BackupEngineWALG,
		"pgBackRest":  BackupEnginePgBackRest,
		"pg-backrest": BackupEnginePgBackRest,
		"pg_backrest": BackupEnginePgBackRest,
	}
	for input, want := range cases {
		if got := normalizePostgreSQLPhysicalBackupEngine(input); got != want {
			t.Fatalf("normalizePostgreSQLPhysicalBackupEngine(%q)=%q, want %q", input, got, want)
		}
	}
	if got := normalizeArchiveEngine("wal-g", DatabaseArchiveTypeWAL); got != BackupEngineWALG {
		t.Fatalf("normalizeArchiveEngine wal-g=%q", got)
	}
}

func TestPostgreSQLExternalEnginesCanBePITRBaseRecords(t *testing.T) {
	for _, engine := range []string{BackupEngineWALG, BackupEnginePgBackRest} {
		record := &DatabaseBackupRecord{
			BackupMethod: DatabaseBackupMethodExternal,
			BackupLevel:  DatabaseBackupLevelFull,
			BackupEngine: engine,
			StorageURI:   "s3://bucket/pg/base",
		}
		if !isPostgreSQLPhysicalBaseRecord(record) {
			t.Fatalf("%s external metadata record should be usable as PostgreSQL PITR base", engine)
		}
	}
}

func TestPostgreSQLWALGExternalPlanIsWarningAndMetadataOnly(t *testing.T) {
	result := &restorePlanValidationResult{
		BackupChainStatus: DatabaseBackupChainStatusComplete,
		LogChainStatus:    DatabaseLogChainStatusComplete,
		StorageStatus:     DatabaseStorageStatusAvailable,
		ToolStatus:        DatabaseToolStatusCompatible,
		ValidationStatus:  DatabasePlanValidationPassed,
		BackupProofs: []restoreProofBackup{
			{ID: 1, BackupMethod: DatabaseBackupMethodPhysical, BackupLevel: DatabaseBackupLevelFull, BackupEngine: BackupEngineWALG, StorageURI: "s3://bucket/base"},
		},
	}
	finalized := finalizePostgreSQLRestoreValidation(result)
	if finalized.ValidationStatus != DatabasePlanValidationWarning {
		t.Fatalf("WAL-G external plan should be warning, got %s", finalized.ValidationStatus)
	}

	planJSON := buildPostgreSQLRestorePlanJSON(&DatabaseInstance{DBType: DBTypePostgreSQL}, finalized, postgreSQLRestoreTarget{
		Type:  "time",
		Value: "2026-04-30 10:00:00",
	}, "isolated_restore")
	var payload map[string]any
	if err := json.Unmarshal([]byte(planJSON), &payload); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if payload["backupEngine"] != BackupEngineWALG || payload["externalMetadataOnly"] != true {
		t.Fatalf("unexpected WAL-G plan payload: %#v", payload)
	}
	tools, _ := payload["requiredTools"].([]any)
	foundWALG := false
	for _, item := range tools {
		if item == "wal-g" {
			foundWALG = true
		}
	}
	if !foundWALG {
		t.Fatalf("WAL-G plan should require wal-g metadata tool: %#v", tools)
	}

	status, runnerPath, message := restoreArtifactReadiness(0, "s3://bucket/base", "", BackupEngineWALG)
	if status != "managed_by_walg" || runnerPath != "" || !strings.Contains(message, "WAL-G external") {
		t.Fatalf("unexpected WAL-G artifact readiness status=%s path=%s message=%s", status, runnerPath, message)
	}
}

func TestPostgreSQLPgBackRestExternalPlanIsLegacyWarning(t *testing.T) {
	result := finalizePostgreSQLRestoreValidation(&restorePlanValidationResult{
		BackupChainStatus: DatabaseBackupChainStatusComplete,
		LogChainStatus:    DatabaseLogChainStatusComplete,
		StorageStatus:     DatabaseStorageStatusAvailable,
		ToolStatus:        DatabaseToolStatusCompatible,
		BackupProofs: []restoreProofBackup{
			{ID: 1, BackupMethod: DatabaseBackupMethodExternal, BackupLevel: DatabaseBackupLevelFull, BackupEngine: BackupEnginePgBackRest, StorageURI: "s3://bucket/pgbackrest/base"},
		},
	})
	if result.ValidationStatus != DatabasePlanValidationWarning {
		t.Fatalf("pgBackRest external plan should be warning, got %s", result.ValidationStatus)
	}
	planJSON := buildPostgreSQLRestorePlanJSON(&DatabaseInstance{DBType: DBTypePostgreSQL}, result, postgreSQLRestoreTarget{
		Type:  "time",
		Value: "2026-04-30 10:00:00",
	}, "isolated_restore")
	var payload map[string]any
	if err := json.Unmarshal([]byte(planJSON), &payload); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if payload["backupEngine"] != BackupEnginePgBackRest || payload["legacyExternal"] != true {
		t.Fatalf("unexpected pgBackRest plan payload: %#v", payload)
	}
	status, _, message := restoreArtifactReadiness(0, "s3://bucket/pgbackrest/base", "", BackupEnginePgBackRest)
	if status != "legacy_pgbackrest" || !strings.Contains(message, "legacy external") {
		t.Fatalf("unexpected pgBackRest readiness status=%s message=%s", status, message)
	}
}
