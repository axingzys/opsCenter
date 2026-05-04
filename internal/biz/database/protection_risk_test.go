package database

import "testing"

func TestProtectionIssueTypeForRisk(t *testing.T) {
	tests := []struct {
		name    string
		message string
		action  DatabaseProtectionActionVO
		want    string
	}{
		{
			name:    "missing full baseline",
			message: "当前物理备份链缺少可用全量基线",
			action:  DatabaseProtectionActionVO{Action: "run_full_backup"},
			want:    DatabaseProtectionIssueMissingFullBackup,
		},
		{
			name:    "wal gap",
			message: "PostgreSQL WAL 链存在 1 处缺口",
			action:  DatabaseProtectionActionVO{Action: "sync_barman_wal"},
			want:    DatabaseProtectionIssueLogChainGap,
		},
		{
			name:    "restore drill failed",
			message: "最近一次恢复演练失败",
			action:  DatabaseProtectionActionVO{Action: "run_restore_drill"},
			want:    DatabaseProtectionIssueRestoreDrillFailed,
		},
		{
			name:    "archive lag",
			message: "日志归档延迟超过目标 RPO",
			action:  DatabaseProtectionActionVO{Action: "inspect_archive_lag"},
			want:    DatabaseProtectionIssueArchiveLagHigh,
		},
		{
			name:    "barman check",
			message: "Barman Server 检查失败",
			action:  DatabaseProtectionActionVO{Action: "check_barman_server"},
			want:    DatabaseProtectionIssueRunnerToolMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := protectionIssueTypeForRisk(tt.message, tt.action); got != tt.want {
				t.Fatalf("issue type = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExpandProtectionRisksFiltersIssueType(t *testing.T) {
	profile := &DatabaseProtectionProfileVO{
		ProfileID:          "instance-1",
		InstanceID:         1,
		InstanceName:       "pg-prod",
		Engine:             DBTypePostgreSQL,
		RiskLevel:          DatabaseQueryRiskCritical,
		RiskMessages:       []string{"PostgreSQL WAL 链存在 1 处缺口", "尚未完成恢复演练 proof"},
		RecommendedActions: []DatabaseProtectionActionVO{protectionAction(DatabaseQueryRiskCritical, "sync_barman_wal", "同步 WAL", true), protectionAction(DatabaseQueryRiskMedium, "run_restore_drill", "演练", false)},
	}
	risks := expandProtectionRisks(profile, DatabaseProtectionIssueLogChainGap)
	if len(risks) != 1 {
		t.Fatalf("risks len = %d, want 1", len(risks))
	}
	if risks[0].IssueType != DatabaseProtectionIssueLogChainGap {
		t.Fatalf("issue = %s", risks[0].IssueType)
	}
}
