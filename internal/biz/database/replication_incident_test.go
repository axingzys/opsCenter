package database

import (
	"context"
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

func TestDeleteReplicaIncidentGuideDeletesExistingGuide(t *testing.T) {
	repo := &replicaIncidentGuideRepoForDeleteTest{
		item: &DatabaseReplicaIncidentGuide{
			Model:        gorm.Model{ID: 9},
			InstanceID:   40,
			IncidentType: ReplicaIncidentTypeDelete,
			CanIntercept: true,
		},
	}
	uc := &UseCase{replicaIncidentRepo: repo}

	err := uc.DeleteReplicaIncidentGuide(context.Background(), 9, QueryOperator{Username: "admin"})
	if err != nil {
		t.Fatalf("delete incident guide: %v", err)
	}
	if repo.deletedID != 9 {
		t.Fatalf("expected delete id 9, got %d", repo.deletedID)
	}
}

func TestDeleteReplicaIncidentGuideRejectsMissingGuide(t *testing.T) {
	repo := &replicaIncidentGuideRepoForDeleteTest{}
	uc := &UseCase{replicaIncidentRepo: repo}

	err := uc.DeleteReplicaIncidentGuide(context.Background(), 9, QueryOperator{Username: "admin"})
	if err == nil {
		t.Fatalf("expected missing guide error")
	}
	if repo.deletedID != 0 {
		t.Fatalf("expected no delete call, got %d", repo.deletedID)
	}
}

type replicaIncidentGuideRepoForDeleteTest struct {
	item      *DatabaseReplicaIncidentGuide
	deletedID uint
}

func (r *replicaIncidentGuideRepoForDeleteTest) Create(context.Context, *DatabaseReplicaIncidentGuide) error {
	return nil
}

func (r *replicaIncidentGuideRepoForDeleteTest) Delete(_ context.Context, id uint) error {
	r.deletedID = id
	return nil
}

func (r *replicaIncidentGuideRepoForDeleteTest) GetByID(_ context.Context, id uint) (*DatabaseReplicaIncidentGuide, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}

func (r *replicaIncidentGuideRepoForDeleteTest) List(context.Context, *DatabaseReplicaIncidentGuideListRequest) ([]*DatabaseReplicaIncidentGuide, int64, error) {
	return nil, 0, nil
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
