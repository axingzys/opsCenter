package database

import (
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestBuildMySQLPhysicalBackupPolicyScriptContainerMode(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:                gorm.Model{ID: 7},
		Engine:               DBTypeMySQL,
		BackupEngine:         "xtrabackup_8_0",
		ToolExecutionMode:    DatabaseToolExecutionModeContainer,
		ToolImage:            "opshub-runner-tools:mysql80",
		ContainerDatadirPath: "/var/lib/mysql",
		ContainerNetworkMode: "host",
		ContainerDatadirRO:   true,
	}
	record := &DatabaseBackupRecord{
		Model:       gorm.Model{ID: 99},
		BackupLevel: DatabaseBackupLevelFull,
		FileName:    "full.physical.tar.gz",
	}
	instance := &DatabaseInstance{Host: "10.0.0.8", Port: 3306}
	host := &DatabaseRunnerHost{Model: gorm.Model{ID: 3}, WorkDir: "/var/lib/opshub-runner"}
	credential := &ConnectionCredential{Username: "backup_user", Password: "secret-pass"}

	script := buildMySQLPhysicalBackupPolicyScript(policy, record, "", instance, host, credential)
	for _, expected := range []string{
		`TOOL_EXECUTION_MODE='container_tools'`,
		`CONTAINER_TOOL_IMAGE='opshub-runner-tools:mysql80'`,
		`CONTAINER_DATADIR='/var/lib/mysql'`,
		`CONTAINER_DATADIR_MODE='ro'`,
		`"$DOCKER_BIN" image inspect "$CONTAINER_TOOL_IMAGE"`,
		`--datadir=/var/lib/mysql --target-dir=/work/backup`,
		`OPSHUB_STORAGE_URI=runner://runner-host-3`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("container script missing %q\n%s", expected, script)
		}
	}
}

func TestValidateContainerToolImageRejectsUnsafeTags(t *testing.T) {
	for _, image := range []string{"mysql", "mysql:latest", "repo/mysql:latest@sha256:abc", "mysql:8.0 bad"} {
		if err := validateContainerToolImage(image); err == nil {
			t.Fatalf("expected invalid container image %q", image)
		}
	}
	for _, image := range []string{"opshub-runner-tools:mysql80", "registry.local/opshub-runner-tools@sha256:abcdef"} {
		if err := validateContainerToolImage(image); err != nil {
			t.Fatalf("expected valid container image %q: %v", image, err)
		}
	}
}

func TestBuildMySQLPhysicalBackupPolicyScriptBinlogGTIDAwkIsShellValid(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:        gorm.Model{ID: 7},
		Engine:       DBTypeMySQL,
		BackupEngine: "xtrabackup_8_0",
	}
	record := &DatabaseBackupRecord{
		Model:       gorm.Model{ID: 99},
		BackupLevel: DatabaseBackupLevelFull,
		FileName:    "full.physical.tar.gz",
	}
	instance := &DatabaseInstance{Host: "10.0.0.8", Port: 3306}
	host := &DatabaseRunnerHost{
		Model:   gorm.Model{ID: 3},
		WorkDir: "/var/lib/opshub-runner",
	}
	credential := &ConnectionCredential{Username: "backup_user", Password: "secret-pass"}

	script := buildMySQLPhysicalBackupPolicyScript(policy, record, "", instance, host, credential)
	expected := `binlog_gtid="$(awk 'NR==1 {$1=""; $2=""; sub(/^[ \t]+/,""); print}' "$binlog_info")"`
	if !strings.Contains(script, expected) {
		t.Fatalf("script should parse GTID without escaped shell quotes, missing %q\n%s", expected, script)
	}
	if strings.Contains(script, `{$1=\"\"; $2=\"\"`) {
		t.Fatalf("script contains escaped quotes that break awk execution\n%s", script)
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
		SyntheticRuleJSON: `{"mergeOldestIncrementals":2,"requireRestoreProof":false}`,
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

func TestParseSyntheticRuleDefaultsForRollingAutoPolicy(t *testing.T) {
	rule := parseSyntheticRule(&DatabaseBackupPolicyConfig{})
	if rule.Mode != "rolling_synthetic_full" {
		t.Fatalf("unexpected default mode: %s", rule.Mode)
	}
	if rule.AutoRun {
		t.Fatalf("auto run must be opt-in")
	}
	if rule.TriggerAfterIncrementals != 5 || rule.MergeOldestIncrementals != 5 {
		t.Fatalf("unexpected default thresholds: trigger=%d merge=%d", rule.TriggerAfterIncrementals, rule.MergeOldestIncrementals)
	}
	if !rule.RequireRestoreProof || !rule.NeverDeleteWithoutProof || !rule.MarkSupersededAfterProof {
		t.Fatalf("safe synthetic rule gates should default to true: %#v", rule)
	}
}

func TestBuildSyntheticAutoDecisionRequiresExactRollingWindow(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:             gorm.Model{ID: 7},
		InstanceID:        10,
		Enabled:           true,
		Status:            DatabaseBackupPolicyStatusActive,
		SyntheticEnabled:  true,
		BinlogStreamID:    2,
		SyntheticRuleJSON: `{"autoRun":true,"triggerAfterIncrementals":2,"mergeOldestIncrementals":2,"requireRestoreProof":true}`,
	}
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc1 := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	inc2 := testPolicyBackupRecord(3, DatabaseBackupLevelIncremental, 2, 1, "2000", "3000")
	validation := &backupPolicyChainValidation{
		Policy:       policy,
		Status:       DatabaseBackupChainStatusComplete,
		BaseRecord:   base,
		LatestRecord: inc2,
		Records:      []*DatabaseBackupRecord{base, inc1, inc2},
	}
	rule := syntheticRuleConfig{
		Mode:                     "rolling_synthetic_full",
		AutoRun:                  true,
		TriggerAfterIncrementals: 2,
		MergeOldestIncrementals:  2,
		RequireRestoreProof:      true,
	}
	preview := (&UseCase{}).buildSyntheticFullPreview(policy, validation)
	decision := buildSyntheticAutoDecision(policy, rule, validation, preview, inc2)
	if !decision.ShouldRun {
		t.Fatalf("expected auto synthetic to run, got %#v", decision)
	}

	rule.MergeOldestIncrementals = 1
	decision = buildSyntheticAutoDecision(policy, rule, validation, preview, inc2)
	if decision.ShouldRun || !decision.Degraded || !strings.Contains(decision.Reason, "mergeOldestIncrementals") {
		t.Fatalf("partial merge should be blocked and degraded, got %#v", decision)
	}
}

func TestBuildSyntheticAutoDecisionBlocksWithoutBinlogAtThreshold(t *testing.T) {
	policy := &DatabaseBackupPolicyConfig{
		Model:             gorm.Model{ID: 7},
		InstanceID:        10,
		Enabled:           true,
		Status:            DatabaseBackupPolicyStatusActive,
		SyntheticEnabled:  true,
		SyntheticRuleJSON: `{"autoRun":true,"triggerAfterIncrementals":1,"mergeOldestIncrementals":1,"requireRestoreProof":true}`,
	}
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	validation := &backupPolicyChainValidation{
		Policy:       policy,
		Status:       DatabaseBackupChainStatusComplete,
		BaseRecord:   base,
		LatestRecord: inc,
		Records:      []*DatabaseBackupRecord{base, inc},
	}
	rule := syntheticRuleConfig{
		Mode:                     "rolling_synthetic_full",
		AutoRun:                  true,
		TriggerAfterIncrementals: 1,
		MergeOldestIncrementals:  1,
	}
	preview := (&UseCase{}).buildSyntheticFullPreview(policy, validation)
	decision := buildSyntheticAutoDecision(policy, rule, validation, preview, inc)
	if decision.ShouldRun || !decision.Degraded || !strings.Contains(decision.Reason, "binlog") {
		t.Fatalf("missing binlog stream should block auto synthetic, got %#v", decision)
	}
}

func TestBuildSyntheticPurgePreviewRequiresProof(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.Local)
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	synthetic := testPolicyBackupRecord(9, DatabaseBackupLevelFull, 2, 9, "", "2000")
	synthetic.BackupOrigin = DatabaseBackupOriginSyntheticFull
	synthetic.SyntheticSourceRecordIDs = `[1,2]`
	synthetic.BackupBinlogFile = "binlog.000001"
	synthetic.BackupBinlogPos = 4
	synthetic.RestoreTestStatus = DatabaseBackupStatusPending

	preview := buildSyntheticPurgePreview(&syntheticPurgeInput{
		Policy: &DatabaseBackupPolicyConfig{
			Model:          gorm.Model{ID: 7},
			InstanceID:     10,
			BinlogStreamID: 3,
		},
		Synthetic:     synthetic,
		SourceRecords: []*DatabaseBackupRecord{base, inc},
		AllRecords:    []*DatabaseBackupRecord{base, inc, synthetic},
		LogArchives: []*DatabaseLogArchive{{
			Model:    gorm.Model{ID: 20},
			FileName: "binlog.000001",
			Status:   DatabaseLogArchiveStatusArchived,
		}},
		Rule: syntheticRuleConfig{
			MergeOldestIncrementals:      1,
			SupersededKeepDaysAfterProof: 0,
			NeverDeleteWithoutProof:      true,
			MarkSupersededAfterProof:     true,
		},
		Now: now,
	})
	if len(preview.EligibleRecordIDs) != 0 {
		t.Fatalf("proof pending should not produce eligible records: %#v", preview.EligibleRecordIDs)
	}
	if len(preview.BlockingReasons) == 0 || !strings.Contains(strings.Join(preview.BlockingReasons, " "), "恢复演练") {
		t.Fatalf("expected restore proof block, got %v", preview.BlockingReasons)
	}
}

func TestBuildSyntheticPurgePreviewEligibleAfterProof(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.Local)
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	eligibleAt := now.Add(-time.Minute)
	for _, record := range []*DatabaseBackupRecord{base, inc} {
		record.SupersededByRecordID = 9
		record.PurgeEligibleAt = &eligibleAt
	}
	synthetic := testPolicyBackupRecord(9, DatabaseBackupLevelFull, 2, 9, "", "2000")
	synthetic.BackupOrigin = DatabaseBackupOriginSyntheticFull
	synthetic.SyntheticSourceRecordIDs = `[1,2]`
	synthetic.BackupBinlogFile = "binlog.000001"
	synthetic.BackupBinlogPos = 4
	synthetic.RestoreTestStatus = DatabaseBackupStatusSuccess

	preview := buildSyntheticPurgePreview(&syntheticPurgeInput{
		Policy: &DatabaseBackupPolicyConfig{
			Model:          gorm.Model{ID: 7},
			InstanceID:     10,
			BinlogStreamID: 3,
		},
		Synthetic:     synthetic,
		SourceRecords: []*DatabaseBackupRecord{base, inc},
		AllRecords:    []*DatabaseBackupRecord{base, inc, synthetic},
		LogArchives: []*DatabaseLogArchive{{
			Model:    gorm.Model{ID: 20},
			FileName: "binlog.000001",
			Status:   DatabaseLogArchiveStatusArchived,
		}},
		Rule: syntheticRuleConfig{
			MergeOldestIncrementals:      1,
			SupersededKeepDaysAfterProof: 0,
			NeverDeleteWithoutProof:      true,
			MarkSupersededAfterProof:     true,
		},
		Now: now,
	})
	if len(preview.BlockingReasons) != 0 {
		t.Fatalf("expected no blocking reasons, got %v", preview.BlockingReasons)
	}
	if len(preview.EligibleRecordIDs) != 2 || preview.EligibleRecordIDs[0] != 1 || preview.EligibleRecordIDs[1] != 2 {
		t.Fatalf("unexpected eligible records: %v", preview.EligibleRecordIDs)
	}
	if len(preview.StorageDeletePlan) != 2 || preview.StorageDeletePlan[0].StorageURI == "" || preview.StorageDeletePlan[0].ChecksumSHA256 == "" {
		t.Fatalf("storage delete plan should keep uri and checksum: %#v", preview.StorageDeletePlan)
	}
}

func TestBuildSyntheticPurgePreviewBlocksDependentIncremental(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.Local)
	base := testPolicyBackupRecord(1, DatabaseBackupLevelFull, 0, 1, "", "1000")
	inc := testPolicyBackupRecord(2, DatabaseBackupLevelIncremental, 1, 1, "1000", "2000")
	child := testPolicyBackupRecord(3, DatabaseBackupLevelIncremental, 2, 1, "2000", "3000")
	eligibleAt := now.Add(-time.Minute)
	for _, record := range []*DatabaseBackupRecord{base, inc} {
		record.SupersededByRecordID = 9
		record.PurgeEligibleAt = &eligibleAt
	}
	synthetic := testPolicyBackupRecord(9, DatabaseBackupLevelFull, 2, 9, "", "2000")
	synthetic.BackupOrigin = DatabaseBackupOriginSyntheticFull
	synthetic.SyntheticSourceRecordIDs = `[1,2]`
	synthetic.BackupBinlogFile = "binlog.000001"
	synthetic.BackupBinlogPos = 4
	synthetic.RestoreTestStatus = DatabaseBackupStatusSuccess

	preview := buildSyntheticPurgePreview(&syntheticPurgeInput{
		Policy: &DatabaseBackupPolicyConfig{
			Model:          gorm.Model{ID: 7},
			InstanceID:     10,
			BinlogStreamID: 3,
		},
		Synthetic:     synthetic,
		SourceRecords: []*DatabaseBackupRecord{base, inc},
		AllRecords:    []*DatabaseBackupRecord{base, inc, child, synthetic},
		LogArchives: []*DatabaseLogArchive{{
			Model:    gorm.Model{ID: 20},
			FileName: "binlog.000001",
			Status:   DatabaseLogArchiveStatusArchived,
		}},
		Rule: syntheticRuleConfig{
			MergeOldestIncrementals:      1,
			SupersededKeepDaysAfterProof: 0,
			NeverDeleteWithoutProof:      true,
			MarkSupersededAfterProof:     true,
		},
		Now: now,
	})
	if len(preview.EligibleRecordIDs) != 0 {
		t.Fatalf("dependent incremental should block all cleanup: %v", preview.EligibleRecordIDs)
	}
	if len(preview.BlockingReasons) == 0 || !strings.Contains(strings.Join(preview.BlockingReasons, " "), "parent") {
		t.Fatalf("expected dependency block, got %v", preview.BlockingReasons)
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
