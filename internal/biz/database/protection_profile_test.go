package database

import (
	"strings"
	"testing"
	"time"
)

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

func TestProtectionProfileInheritedReplicaUsesPrimaryProtection(t *testing.T) {
	profile := &DatabaseProtectionProfileVO{
		BackupRequirement:            DatabaseProtectionBackupRequirementInherited,
		InheritedProtection:          true,
		InheritedProtectionLevel:     DatabaseProtectionLevelPITRCapable,
		InheritedProtectionRiskLevel: DatabaseQueryRiskLow,
		PrimaryInstanceID:            1,
		PrimaryInstanceName:          "opshub-mysql",
		ProtectionMode:               DatabaseProtectionModeInherited,
		ReplicaRole:                  DatabaseReplicaRoleDelayed,
		ReplicaRoleText:              ReplicaRoleText(DatabaseReplicaRoleDelayed),
		ConfiguredDelaySeconds:       3600,
		RecommendedActions:           []DatabaseProtectionActionVO{},
		BackupChainStatus:            DatabaseBackupChainStatusUnsupported,
		LogChainStatus:               DatabaseLogChainStatusUnsupported,
		ReplicaProtectionStatus:      DatabaseReplicaProtectionUnknown,
		RestoreDrillStatus:           DatabaseRestoreDrillStatusNone,
		RemainingDelaySeconds:        -1,
	}

	profile.applyProtectionAssessment(time.Now())

	if profile.RiskLevel != DatabaseQueryRiskLow {
		t.Fatalf("got risk %s, want %s, messages=%v", profile.RiskLevel, DatabaseQueryRiskLow, profile.RiskMessages)
	}
	if profile.ProtectionLevel != DatabaseProtectionLevelPITRCapable {
		t.Fatalf("got level %s, want %s", profile.ProtectionLevel, DatabaseProtectionLevelPITRCapable)
	}
	for _, message := range profile.RiskMessages {
		if strings.Contains(message, "没有启用备份策略") {
			t.Fatalf("inherited replica should not require own backup, got message %q", message)
		}
	}
	if profile.InheritedProtectionText == "" || !strings.Contains(profile.InheritedProtectionText, "opshub-mysql") {
		t.Fatalf("unexpected inherited text: %q", profile.InheritedProtectionText)
	}
}

func TestProtectionProfileInheritedReplicaRequiresProtectedPrimary(t *testing.T) {
	profile := &DatabaseProtectionProfileVO{
		BackupRequirement:       DatabaseProtectionBackupRequirementInherited,
		PrimaryInstanceID:       1,
		PrimaryInstanceName:     "opshub-mysql",
		ProtectionMode:          DatabaseProtectionModeInherited,
		RecommendedActions:      []DatabaseProtectionActionVO{},
		BackupChainStatus:       DatabaseBackupChainStatusUnsupported,
		LogChainStatus:          DatabaseLogChainStatusUnsupported,
		ReplicaProtectionStatus: DatabaseReplicaProtectionUnknown,
		RestoreDrillStatus:      DatabaseRestoreDrillStatusNone,
	}

	profile.applyProtectionAssessment(time.Now())

	if profile.RiskLevel != DatabaseQueryRiskCritical {
		t.Fatalf("got risk %s, want %s", profile.RiskLevel, DatabaseQueryRiskCritical)
	}
	if !containsProtectionRiskMessage(profile.RiskMessages, "来源主库尚未启用") {
		t.Fatalf("expected primary protection risk, got %v", profile.RiskMessages)
	}
	if profile.ProtectionLevel != DatabaseProtectionLevelNone {
		t.Fatalf("got level %s, want %s", profile.ProtectionLevel, DatabaseProtectionLevelNone)
	}
}

func containsProtectionRiskMessage(messages []string, want string) bool {
	for _, message := range messages {
		if strings.Contains(message, want) {
			return true
		}
	}
	return false
}
