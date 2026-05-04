package database

import "testing"

func TestPostgresBarmanWizardServerName(t *testing.T) {
	instance := &DatabaseInstance{Name: "PG Prod/主库"}
	instance.ID = 42
	got := postgresBarmanWizardServerName(&DatabasePostgresBarmanPITRWizardRequest{}, instance)
	if got != "pg_42_pg_prod" {
		t.Fatalf("server name = %q", got)
	}
	if got := postgresBarmanWizardServerName(&DatabasePostgresBarmanPITRWizardRequest{BarmanServerName: "pg-prod"}, instance); got != "pg-prod" {
		t.Fatalf("explicit server name = %q", got)
	}
}

func TestPostgresProtectionModeFromBackupEngine(t *testing.T) {
	tests := map[string]string{
		"barman":        DatabaseProtectionModePostgresBarmanPITR,
		"pg-basebackup": DatabaseProtectionModePostgresPgBaseBackupPITR,
		"wal-g":         DatabaseProtectionModeExternalPITR,
	}
	for input, want := range tests {
		if got := postgresProtectionModeFromBackupEngine(input); got != want {
			t.Fatalf("%s => %s, want %s", input, got, want)
		}
	}
}
