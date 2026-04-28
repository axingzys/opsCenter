package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMySQLPhysicalBackupCompatibilityMatrix(t *testing.T) {
	ctx := context.Background()
	mysql80 := &DatabaseInstance{DBType: DBTypeMySQL, Version: "8.0.44"}
	if _, err := validateMySQLPhysicalBackupCompatibility(ctx, mysql80, nil, BackupEngineXtraBackup84); err == nil {
		t.Fatalf("expected MySQL 8.0 with XtraBackup 8.4 to be rejected")
	} else if !strings.Contains(err.Error(), "8.0.x") {
		t.Fatalf("unexpected MySQL 8.0 compatibility error: %v", err)
	}
	if got, err := validateMySQLPhysicalBackupCompatibility(ctx, mysql80, nil, BackupEngineXtraBackup80); err != nil {
		t.Fatalf("expected MySQL 8.0 with XtraBackup 8.0 to pass: %v", err)
	} else if got.Engine != BackupEngineXtraBackup80 {
		t.Fatalf("engine = %q, want %q", got.Engine, BackupEngineXtraBackup80)
	}

	mysql84 := &DatabaseInstance{DBType: DBTypeMySQL, Version: "8.4.0"}
	if _, err := validateMySQLPhysicalBackupCompatibility(ctx, mysql84, nil, BackupEngineXtraBackup80); err == nil {
		t.Fatalf("expected MySQL 8.4 with XtraBackup 8.0 to be rejected")
	}
	if got, err := validateMySQLPhysicalBackupCompatibility(ctx, mysql84, nil, ""); err != nil {
		t.Fatalf("expected MySQL 8.4 default engine to pass: %v", err)
	} else if got.Engine != BackupEngineXtraBackup84 {
		t.Fatalf("engine = %q, want %q", got.Engine, BackupEngineXtraBackup84)
	}

	mariadb := &DatabaseInstance{DBType: DBTypeMariaDB, Version: "11.4.2-MariaDB"}
	if got, err := validateMySQLPhysicalBackupCompatibility(ctx, mariadb, nil, ""); err != nil {
		t.Fatalf("expected MariaDB default engine to pass: %v", err)
	} else if got.Engine != BackupEngineMariaDB {
		t.Fatalf("engine = %q, want %q", got.Engine, BackupEngineMariaDB)
	}
	if _, err := validateMySQLPhysicalBackupCompatibility(ctx, mariadb, nil, BackupEngineXtraBackup80); err == nil {
		t.Fatalf("expected MariaDB with XtraBackup to be rejected")
	}
}

func TestValidateIncrementalBackupChainMissingParent(t *testing.T) {
	now := time.Now()
	base := &DatabaseBackupRecord{
		Model:        gormModelForTest(1),
		Status:       DatabaseBackupStatusSuccess,
		BackupMethod: DatabaseBackupMethodPhysical,
		BackupLevel:  DatabaseBackupLevelFull,
		ChainID:      "chain-a",
		FinishedAt:   &now,
	}
	incremental := &DatabaseBackupRecord{
		Model:          gormModelForTest(2),
		Status:         DatabaseBackupStatusSuccess,
		BackupMethod:   DatabaseBackupMethodPhysical,
		BackupLevel:    DatabaseBackupLevelIncremental,
		ChainID:        "chain-a",
		ParentRecordID: 99,
		FinishedAt:     &now,
	}
	selectedIDs := []uint{base.ID}
	messages := []string{}
	status := validateIncrementalBackupChain([]*DatabaseBackupRecord{base, incremental}, base, now.Add(time.Minute), &selectedIDs, &messages)
	if status != DatabaseBackupChainStatusMissingIncremental {
		t.Fatalf("status = %q, want %q", status, DatabaseBackupChainStatusMissingIncremental)
	}
	if len(messages) == 0 || !strings.Contains(messages[0], "父记录") {
		t.Fatalf("expected missing parent message, got %#v", messages)
	}
}

func TestValidateMySQLRestoreLogMetadataDetectsMissingBinlogAndGTIDGap(t *testing.T) {
	base := &DatabaseBackupRecord{
		BackupBinlogFile: "binlog.000002",
		BackupBinlogPos:  4,
		BackupGTIDSet:    "uuid:1-10",
		GTIDMode:         "ON",
	}
	status, _ := validateMySQLRestoreLogMetadata(base, []*DatabaseLogArchive{
		{FileName: "binlog.000001", StartGTIDSet: "uuid:1-1", EndGTIDSet: "uuid:1-10"},
	})
	if status != DatabaseLogChainStatusMissingBinlog {
		t.Fatalf("status = %q, want %q", status, DatabaseLogChainStatusMissingBinlog)
	}

	status, _ = validateMySQLRestoreLogMetadata(base, []*DatabaseLogArchive{
		{FileName: "binlog.000002"},
	})
	if status != DatabaseLogChainStatusGTIDGap {
		t.Fatalf("status = %q, want %q", status, DatabaseLogChainStatusGTIDGap)
	}
}

func TestParsePhysicalBackupBinlogInfo(t *testing.T) {
	info := parsePhysicalBackupBinlogInfo("binlog.000123\t456\tuuid:1-10\n")
	if info == nil {
		t.Fatalf("expected binlog info")
	}
	if info.File != "binlog.000123" || info.Pos != 456 || info.GTID != "uuid:1-10" {
		t.Fatalf("unexpected binlog info: %#v", info)
	}
}

func TestBuildRestoreProofJSONIncludesP2Evidence(t *testing.T) {
	result := &restorePlanValidationResult{
		BackupChainStatus: DatabaseBackupChainStatusComplete,
		LogChainStatus:    DatabaseLogChainStatusComplete,
		StorageStatus:     DatabaseStorageStatusAvailable,
		ToolStatus:        DatabaseToolStatusCompatible,
		ValidationStatus:  DatabasePlanValidationPassed,
		BackupProofs: []restoreProofBackup{
			{ID: 1, BackupLevel: DatabaseBackupLevelFull, ChecksumSHA256: strings.Repeat("a", 64)},
			{ID: 2, BackupLevel: DatabaseBackupLevelIncremental, ChecksumSHA256: strings.Repeat("b", 64)},
		},
		LogProofs: []restoreProofLogArchive{
			{ID: 10, ArchiveType: DatabaseArchiveTypeBinlog, FileName: "binlog.000001"},
		},
		ValidationSQL: []string{"SELECT 1 AS restore_probe"},
	}
	proof := buildRestoreProofJSON(&DatabaseInstance{Model: gormModelForTest(7), Name: "source"}, &DatabaseInstance{Model: gormModelForTest(8), Name: "target"}, result, time.Now(), QueryOperator{ID: 9, Username: "admin"}, "isolated_restore")
	for _, want := range []string{"baseBackup", "incrementalChain", "logArchiveRange", "checksum", "validationSql"} {
		if !strings.Contains(proof, want) {
			t.Fatalf("proof JSON missing %q: %s", want, proof)
		}
	}
}
