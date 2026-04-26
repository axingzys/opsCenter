package database

import (
	"os"
	"testing"
)

func TestRestoreDryRunGuards(t *testing.T) {
	if !supportsRestoreDryRun(DBTypeMySQL) || !supportsRestoreDryRun(DBTypePostgreSQL) || !supportsRestoreDryRun(DBTypeRedis) {
		t.Fatalf("expected mysql, postgresql and redis restore rehearsal support")
	}
	if supportsRestoreDryRun(DBTypeTiDB) || supportsRestoreDryRun(DBTypeOceanBase) {
		t.Fatalf("compatible database restore rehearsal should remain closed until verified")
	}
	if !isRestoreCompatibleType(DBTypeMySQL, DBTypeMariaDB) {
		t.Fatalf("mysql and mariadb should be restore compatible")
	}
	if isRestoreCompatibleType(DBTypeMySQL, DBTypePostgreSQL) {
		t.Fatalf("mysql and postgresql should not be restore compatible")
	}
	if !isRestoreCompatibleType(DBTypeRedis, DBTypeRedis) {
		t.Fatalf("redis should be restore compatible with redis")
	}
	if isRestoreCompatibleType(DBTypeRedis, DBTypeMySQL) {
		t.Fatalf("redis and mysql should not be restore compatible")
	}
	if !isProductionEnvironment("prod") || !isProductionEnvironment("生产环境") {
		t.Fatalf("expected production aliases to be blocked")
	}
	if isProductionEnvironment("staging") {
		t.Fatalf("staging should be allowed as non-production")
	}
}

func TestValidateRestoreDryRunTarget(t *testing.T) {
	source := &DatabaseInstance{DBType: DBTypeMySQL}
	target := &DatabaseInstance{
		DBType:      DBTypeMySQL,
		Status:      DatabaseInstanceStatusEnabled,
		Environment: "prod",
	}
	if err := validateRestoreDryRunTarget(source, target); err == nil {
		t.Fatalf("expected production target to be rejected")
	}

	target.Environment = "test"
	if err := validateRestoreDryRunTarget(source, target); err != nil {
		t.Fatalf("expected non-production compatible target, got %v", err)
	}

	target.DBType = DBTypePostgreSQL
	if err := validateRestoreDryRunTarget(source, target); err == nil {
		t.Fatalf("expected incompatible target to be rejected")
	}

	source.ID = 10
	target.ID = 10
	target.DBType = DBTypeMySQL
	if err := validateRestoreDryRunTarget(source, target); err == nil {
		t.Fatalf("expected same source and target to be rejected")
	}
}

func TestValidateRestoreDryRunTargetRedis(t *testing.T) {
	source := &DatabaseInstance{DBType: DBTypeRedis}
	target := &DatabaseInstance{
		DBType:      DBTypeRedis,
		Status:      DatabaseInstanceStatusEnabled,
		Environment: "test",
	}
	if err := validateRestoreDryRunTarget(source, target); err != nil {
		t.Fatalf("expected redis target to be allowed, got %v", err)
	}
}

func TestValidateRestoreDryRunRecordAllowsPostgreSQLCustom(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "backup-*.dump")
	if err != nil {
		t.Fatalf("create temp backup: %v", err)
	}
	file.Close()

	record := &DatabaseBackupRecord{
		Status:      DatabaseBackupStatusSuccess,
		BackupType:  DatabaseBackupTypeLogicalCustom,
		StorageType: DatabaseBackupStorageLocal,
		FilePath:    file.Name(),
		FileName:    "backup.dump",
	}
	if err := validateRestoreDryRunRecord(record); err != nil {
		t.Fatalf("expected custom backup record to be restorable, got %v", err)
	}
}
