package database

import (
	"strings"
	"testing"
)

func TestSupportsBackupTask(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{dbType: DBTypeMySQL, want: true},
		{dbType: DBTypeMariaDB, want: true},
		{dbType: DBTypePostgreSQL, want: true},
		{dbType: DBTypeRedis, want: true},
		{dbType: DBTypeSQLServer, want: false},
		{dbType: DBTypeClickHouse, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			if got := supportsBackupTask(tt.dbType); got != tt.want {
				t.Fatalf("supportsBackupTask(%q) = %v, want %v", tt.dbType, got, tt.want)
			}
		})
	}
}

func TestSupportsBackupType(t *testing.T) {
	tests := []struct {
		name       string
		dbType     string
		backupType string
		want       bool
	}{
		{name: "postgres logical", dbType: DBTypePostgreSQL, backupType: DatabaseBackupTypeLogical, want: true},
		{name: "postgres custom", dbType: DBTypePostgreSQL, backupType: DatabaseBackupTypeLogicalCustom, want: true},
		{name: "mysql logical", dbType: DBTypeMySQL, backupType: DatabaseBackupTypeLogical, want: true},
		{name: "mysql custom", dbType: DBTypeMySQL, backupType: DatabaseBackupTypeLogicalCustom, want: false},
		{name: "redis logical", dbType: DBTypeRedis, backupType: DatabaseBackupTypeLogical, want: true},
		{name: "redis custom", dbType: DBTypeRedis, backupType: DatabaseBackupTypeLogicalCustom, want: false},
		{name: "unsupported", dbType: DBTypeClickHouse, backupType: DatabaseBackupTypeLogical, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsBackupType(tt.dbType, tt.backupType); got != tt.want {
				t.Fatalf("supportsBackupType(%q, %q) = %v, want %v", tt.dbType, tt.backupType, got, tt.want)
			}
		})
	}
}

func TestIsValidBackupSchedule(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		want     bool
	}{
		{name: "empty", schedule: "", want: true},
		{name: "valid standard cron", schedule: "0 2 * * *", want: true},
		{name: "invalid too short", schedule: "0 2 * *", want: false},
		{name: "invalid too long", schedule: "0 2 * * * *", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidBackupSchedule(tt.schedule); got != tt.want {
				t.Fatalf("isValidBackupSchedule(%q) = %v, want %v", tt.schedule, got, tt.want)
			}
		})
	}
}

func TestNormalizeBackupMaxDurationMinutes(t *testing.T) {
	if got := normalizeBackupMaxDurationMinutes(0); got != defaultBackupMaxDurationMinutes {
		t.Fatalf("normalizeBackupMaxDurationMinutes(0) = %d, want %d", got, defaultBackupMaxDurationMinutes)
	}
	if got := normalizeBackupMaxDurationMinutes(30); got != 30 {
		t.Fatalf("normalizeBackupMaxDurationMinutes(30) = %d, want 30", got)
	}
	if got := normalizeBackupMaxDurationMinutes(maxBackupMaxDurationMinutes + 1); got != maxBackupMaxDurationMinutes {
		t.Fatalf("normalizeBackupMaxDurationMinutes(over max) = %d, want %d", got, maxBackupMaxDurationMinutes)
	}
}

func TestValidateBackupStorageConfigSafeRejectsSecrets(t *testing.T) {
	if err := validateBackupStorageConfigSafe(`{"path":"/data/backups"}`); err != nil {
		t.Fatalf("validateBackupStorageConfigSafe(non-secret) error = %v", err)
	}
	err := validateBackupStorageConfigSafe(`{"access_key":"AKIA..."}`)
	if err == nil {
		t.Fatalf("expected storage config with access_key to be rejected")
	}
	if !strings.Contains(err.Error(), "不允许保存密码") {
		t.Fatalf("unexpected storage config error: %v", err)
	}
}

func TestRestoreCapabilityTextAndPITRStatus(t *testing.T) {
	if got := normalizeRestoreCapability(""); got != DatabaseRestoreCapabilityLogicalRestoreOnly {
		t.Fatalf("normalizeRestoreCapability(empty) = %q", got)
	}
	if got := RestoreCapabilityText(DatabaseRestoreCapabilityPITRVerified); got != "PITR 已演练" {
		t.Fatalf("RestoreCapabilityText(pitr_verified) = %q", got)
	}
	if got := PITRStatusText(DatabaseRestoreCapabilityLogicalRestoreOnly); got != "不支持 PITR" {
		t.Fatalf("PITRStatusText(logical) = %q", got)
	}
}
