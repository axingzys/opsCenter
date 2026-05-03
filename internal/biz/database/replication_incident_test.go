package database

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestReplicaIncidentCanInterceptRequiresDelayedReplicaAndRemainingWindow(t *testing.T) {
	protection := &DatabaseReplicaProtectionVO{
		HasDelayedReplica:          true,
		PreferredReplicaInstanceID: 42,
		PreferredReplicaStatus:     DatabaseReplicaHealthHealthy,
		RiskLevel:                  DatabaseReplicaHealthWarning,
		RemainingDelaySeconds:      30,
	}
	check := &DatabaseReplicationCheck{Model: gorm.Model{ID: 1}, HealthStatus: DatabaseReplicaHealthHealthy}
	if !replicaIncidentCanIntercept(protection, check) {
		t.Fatalf("expected incident guide to mark an available remaining window as interceptable")
	}

	protection.RemainingDelaySeconds = 0
	if replicaIncidentCanIntercept(protection, check) {
		t.Fatalf("expected zero remaining delay to be non-interceptable")
	}

	protection.RemainingDelaySeconds = 30
	protection.RiskLevel = DatabaseReplicaHealthCritical
	if replicaIncidentCanIntercept(protection, check) {
		t.Fatalf("expected critical replica risk to be non-interceptable")
	}
}

func TestBuildReplicaIncidentMarkdownContainsReadOnlyBoundary(t *testing.T) {
	incidentAt := time.Date(2026, 5, 3, 15, 30, 0, 0, time.Local)
	markdown := buildReplicaIncidentMarkdown(
		&DatabaseInstance{Model: gorm.Model{ID: 40}, Name: "mysql-primary", DBType: DBTypeMySQL, Host: "127.0.0.1", Port: 3306},
		&DatabaseReplicaProtectionVO{
			HasDelayedReplica:            true,
			PreferredReplicaInstanceID:   42,
			PreferredReplicaInstanceName: "mysql-delayed",
			PreferredReplicaEndpoint:     "127.0.0.1:3307",
			LastCheckID:                  73,
			ConfiguredDelaySeconds:       120,
			RemainingDelaySeconds:        30,
			ProtectionStatusText:         "降级",
			RiskLevelText:                "警告",
		},
		&DatabaseReplicationCheck{Model: gorm.Model{ID: 73}, HealthStatus: DatabaseReplicaHealthHealthy},
		ReplicaIncidentTypeDelete,
		"orders rows",
		"test incident",
		ReplicaIncidentRecoveryExportBackfill,
		&incidentAt,
		true,
		QueryOperator{Username: "admin"},
	)
	required := []string{
		"OpsHub P4.3 只生成指引和审计，不自动执行",
		"STOP REPLICA SQL_THREAD;",
		"STOP SLAVE SQL_THREAD;",
		"SELECT pg_wal_replay_pause();",
		"PITR",
	}
	for _, item := range required {
		if !strings.Contains(markdown, item) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", item, markdown)
		}
	}
}
