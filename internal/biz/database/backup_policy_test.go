package database

import (
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
