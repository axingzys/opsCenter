package database

import "testing"

func TestProtectionProfileInstanceID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint
		wantErr bool
	}{
		{name: "instance prefix", input: "instance-12", want: 12},
		{name: "numeric", input: "34", want: 34},
		{name: "future composite id", input: "mysql:56:78", want: 56},
		{name: "empty", input: "", wantErr: true},
		{name: "bad", input: "instance-abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProtectionProfileInstanceID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProtectionProfileDeriveLevel(t *testing.T) {
	profile := &DatabaseProtectionProfileVO{
		ProtectionMode:        DatabaseProtectionModeMySQLPhysicalPITR,
		RestoreDrillStatus:    DatabaseRestoreDrillStatusSuccess,
		LogArchiveStream:      &DatabaseLogArchiveStreamVO{Enabled: true, Status: DatabaseLogArchiveStreamStatusRunning},
		ReplicaProtection:     &DatabaseReplicaProtectionVO{ProtectionStatus: DatabaseReplicaProtectionProtected},
		BackupPolicy:          &DatabaseBackupPolicyVO{Chain: &DatabaseBackupChainStateVO{CurrentBaseRecordID: 1, Status: DatabaseBackupChainStateHealthy}},
		LatestBackupRecord:    &DatabaseBackupRecordVO{ID: 1},
		LatestLogArchive:      &DatabaseLogArchiveVO{ID: 2},
		RecommendedActions:    []DatabaseProtectionActionVO{},
		ReplicaProtectionText: "受保护",
	}
	if got := profile.deriveProtectionLevel(); got != DatabaseProtectionLevelHAAndPITRVerified {
		t.Fatalf("got %s, want %s", got, DatabaseProtectionLevelHAAndPITRVerified)
	}

	profile.RestoreDrillStatus = DatabaseRestoreDrillStatusNone
	if got := profile.deriveProtectionLevel(); got != DatabaseProtectionLevelPITRCapable {
		t.Fatalf("got %s, want %s", got, DatabaseProtectionLevelPITRCapable)
	}

	profile.LogArchiveStream.Status = DatabaseLogArchiveStreamStatusFailed
	if got := profile.deriveProtectionLevel(); got != DatabaseProtectionLevelBackupOnly {
		t.Fatalf("got %s, want %s", got, DatabaseProtectionLevelBackupOnly)
	}
}
