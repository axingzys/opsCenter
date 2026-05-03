package database

import (
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestValidatePolicyIncrementalParent(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:            gorm.Model{ID: 7},
		InstanceID:       10,
		SourceInstanceID: 20,
		BackupEngine:     "xtrabackup_8_0",
	}
	host := &DatabaseRunnerHost{Model: gorm.Model{ID: 3}}
	parent := &DatabaseBackupRecord{
		Model:            gorm.Model{ID: 11},
		PolicyID:         7,
		SourceInstanceID: 20,
		Status:           DatabaseBackupStatusSuccess,
		BackupMethod:     DatabaseBackupMethodPhysical,
		BackupLevel:      DatabaseBackupLevelFull,
		BackupEngine:     "xtrabackup_8_0",
		StorageURI:       "runner://runner-host-3/backup/base.physical.tar.gz",
		ChecksumSHA256:   "abc123",
		CheckpointToLSN:  "1200",
		ArtifactState:    DatabaseBackupArtifactStateRemote,
	}
	if err := validatePolicyIncrementalParent(policy, host, parent); err != nil {
		t.Fatalf("expected valid parent, got %v", err)
	}

	parent.CheckpointToLSN = ""
	if err := validatePolicyIncrementalParent(policy, host, parent); err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("expected checkpoint error, got %v", err)
	}

	parent.CheckpointToLSN = "1200"
	parent.StorageURI = "runner://runner-host-9/backup/base.physical.tar.gz"
	if err := validatePolicyIncrementalParent(policy, host, parent); err == nil || !strings.Contains(err.Error(), "Runner") {
		t.Fatalf("expected runner-readable artifact error, got %v", err)
	}
}

func TestBuildMySQLPhysicalBackupPolicyScriptIncrementalUsesCnfAndParent(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:        gorm.Model{ID: 7},
		Engine:       DBTypeMySQL,
		BackupEngine: "xtrabackup_8_0",
	}
	record := &DatabaseBackupRecord{
		Model:       gorm.Model{ID: 99},
		BackupLevel: DatabaseBackupLevelIncremental,
		FileName:    "inc.physical.tar.gz",
	}
	instance := &DatabaseInstance{
		Host: "10.0.0.8",
		Port: 3306,
	}
	host := &DatabaseRunnerHost{
		Model:            gorm.Model{ID: 3},
		WorkDir:          "/var/lib/opshub-runner",
		StorageMountPath: "/backup/opshub",
	}
	credential := &ConnectionCredential{
		Username: "backup_user",
		Password: "secret-pass",
	}

	script := buildMySQLPhysicalBackupPolicyScript(policy, record, "/backup/opshub/base.physical.tar.gz", instance, host, credential)
	for _, expected := range []string{
		`--defaults-extra-file="$MYSQL_CNF"`,
		`--incremental-basedir="$PARENT_DIR"`,
		`tar -xzf "$PARENT_ARTIFACT"`,
		`OPSHUB_CHECKPOINT_TO_LSN`,
		`OPSHUB_STORAGE_URI=runner://runner-host-3`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script missing %q\n%s", expected, script)
		}
	}
	if strings.Contains(script, "--password=") || strings.Contains(script, "-psecret-pass") {
		t.Fatalf("script should not pass database password via process arguments")
	}
}

func TestValidateBackupPolicyChainRecordsCompleteAndBrokenLSN(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:            gorm.Model{ID: 7},
		InstanceID:       10,
		SourceInstanceID: 10,
		BackupEngine:     "xtrabackup_8_0",
		BinlogStreamID:   2,
	}
	host := &DatabaseRunnerHost{Model: gorm.Model{ID: 3}}
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc1 := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	inc2 := testPolicyBackupRecord(3, DatabaseBackupLevelIncremental, 2, 1, "2000", "3000")
	state := &DatabaseBackupChainState{
		PolicyID:            7,
		InstanceID:          10,
		ChainID:             "chain-a",
		CurrentBaseRecordID: 1,
		LatestRecordID:      3,
	}

	validation := validateBackupPolicyChainRecords(policy, host, state, []*DatabaseBackupRecord{base, inc1, inc2})
	if validation.Status != DatabaseBackupChainStatusComplete {
		t.Fatalf("expected complete chain, got %s: %v", validation.Status, validation.BlockingReasons)
	}
	if validation.LatestRecord == nil || validation.LatestRecord.ID != 3 {
		t.Fatalf("expected latest record #3, got %#v", validation.LatestRecord)
	}

	inc2.CheckpointFromLSN = "1999"
	validation = validateBackupPolicyChainRecords(policy, host, state, []*DatabaseBackupRecord{base, inc1, inc2})
	if validation.Status != DatabaseBackupChainStatusBrokenChain {
		t.Fatalf("expected broken chain, got %s", validation.Status)
	}
	if len(validation.BlockingReasons) == 0 || !strings.Contains(strings.Join(validation.BlockingReasons, ","), "不连续") {
		t.Fatalf("expected LSN continuity reason, got %v", validation.BlockingReasons)
	}
}

func TestBuildSyntheticFullPreviewSelectsOldestIncrementals(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:             gorm.Model{ID: 7},
		SyntheticEnabled:  true,
		SyntheticRuleJSON: `{"mergeOldestIncrementals":2}`,
		BinlogStreamID:    2,
	}
	validation := &backupPolicyChainValidation{
		Policy:       policy,
		Status:       DatabaseBackupChainStatusComplete,
		BaseRecord:   testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000"),
		LatestRecord: testPolicyBackupRecord(4, DatabaseBackupLevelIncremental, 3, 1, "3000", "4000"),
		Records: []*DatabaseBackupRecord{
			testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000"),
			testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000"),
			testPolicyBackupRecord(3, DatabaseBackupLevelIncremental, 2, 1, "2000", "3000"),
			testPolicyBackupRecord(4, DatabaseBackupLevelIncremental, 3, 1, "3000", "4000"),
		},
	}
	uc := &UseCase{}
	preview := uc.buildSyntheticFullPreview(policy, validation)
	if preview.Status != DatabasePlanValidationPassed {
		t.Fatalf("expected preview passed, got %s: %v", preview.Status, preview.BlockingReasons)
	}
	if preview.NewSyntheticFullAfterRecordID != 3 {
		t.Fatalf("expected synthetic after #3, got #%d", preview.NewSyntheticFullAfterRecordID)
	}
	if got := preview.SelectedIncrementalRecordIDs; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("unexpected selected incrementals: %v", got)
	}
}

func TestBuildMySQLSyntheticFullScriptVerifiesArtifactsAndPrepares(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:        gorm.Model{ID: 7},
		Engine:       DBTypeMySQL,
		BackupEngine: "xtrabackup_8_0",
	}
	record := &DatabaseBackupRecord{
		Model:    gorm.Model{ID: 99},
		FileName: "synthetic.physical.tar.gz",
	}
	host := &DatabaseRunnerHost{
		Model:            gorm.Model{ID: 3},
		WorkDir:          "/var/lib/opshub-runner",
		StorageMountPath: "/backup/opshub",
	}
	script := buildMySQLSyntheticFullScript(policy, record, host, []syntheticFullArtifactInput{
		{RecordID: 1, BackupLevel: DatabaseBackupLevelFull, Path: "/backup/base.tar.gz", ChecksumSHA256: "base-sha"},
		{RecordID: 2, BackupLevel: DatabaseBackupLevelIncremental, Path: "/backup/inc1.tar.gz", ChecksumSHA256: "inc-sha"},
	})
	for _, expected := range []string{
		`verify_artifact`,
		`--prepare --apply-log-only --target-dir="$BASE_DIR"`,
		`--incremental-dir="$INC_DIR"`,
		`OPSHUB_CHECKPOINT_TO_LSN`,
		`OPSHUB_STORAGE_URI=runner://runner-host-3`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script missing %q\n%s", expected, script)
		}
	}
	if strings.Contains(script, "--password=") {
		t.Fatalf("synthetic script should not include database password arguments")
	}
}

func testPolicyBackupRecord(id uint, level string, parentID, baseID uint, fromLSN, toLSN string) *DatabaseBackupRecord {
	return &DatabaseBackupRecord{
		Model:             gormModelForTest(id),
		PolicyID:          7,
		InstanceID:        10,
		SourceInstanceID:  10,
		Status:            DatabaseBackupStatusSuccess,
		BackupMethod:      DatabaseBackupMethodPhysical,
		BackupLevel:       level,
		BackupEngine:      "xtrabackup_8_0",
		ChainID:           "chain-a",
		BaseRecordID:      baseID,
		ParentRecordID:    parentID,
		StorageURI:        "runner://runner-host-3/backup/record-" + strconv.FormatUint(uint64(id), 10) + ".physical.tar.gz",
		FilePath:          "/backup/record-" + strconv.FormatUint(uint64(id), 10) + ".physical.tar.gz",
		FileSize:          int64(id) * 100,
		ChecksumSHA256:    "sha" + strconv.FormatUint(uint64(id), 10),
		CheckpointFromLSN: fromLSN,
		CheckpointToLSN:   toLSN,
		ArtifactState:     DatabaseBackupArtifactStateRemote,
		BackupOrigin:      backupOriginForLevel(level),
	}
}
