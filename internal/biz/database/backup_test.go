package database

import "testing"

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
