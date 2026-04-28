package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
)

type testBackupRecordRepo struct {
	items   []*DatabaseBackupRecord
	updated []*DatabaseBackupRecord
}

func (r *testBackupRecordRepo) Create(context.Context, *DatabaseBackupRecord) error { return nil }

func (r *testBackupRecordRepo) Update(_ context.Context, item *DatabaseBackupRecord) error {
	cloned := *item
	r.updated = append(r.updated, &cloned)
	return nil
}

func (r *testBackupRecordRepo) GetByID(context.Context, uint) (*DatabaseBackupRecord, error) {
	return nil, nil
}

func (r *testBackupRecordRepo) List(context.Context, *DatabaseBackupRecordListRequest) ([]*DatabaseBackupRecord, int64, error) {
	return nil, 0, nil
}

func (r *testBackupRecordRepo) ListExpiredSuccessByTask(_ context.Context, taskID uint, before time.Time) ([]*DatabaseBackupRecord, error) {
	result := make([]*DatabaseBackupRecord, 0, len(r.items))
	for _, item := range r.items {
		if item == nil || item.TaskID != taskID || item.Status != DatabaseBackupStatusSuccess {
			continue
		}
		if item.FinishedAt != nil && item.FinishedAt.Before(before) {
			result = append(result, item)
			continue
		}
		if item.FinishedAt == nil && item.CreatedAt.Before(before) {
			result = append(result, item)
		}
	}
	return result, nil
}

func TestCleanupExpiredBackupFilesByTask(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "expired.sql.gz")
	if err := os.WriteFile(filePath, []byte("backup"), 0o644); err != nil {
		t.Fatalf("write temp backup file: %v", err)
	}

	finishedAt := time.Now().AddDate(0, 0, -10)
	recordRepo := &testBackupRecordRepo{
		items: []*DatabaseBackupRecord{
			{
				Model:        gormModelForTest(1),
				TaskID:       1,
				Status:       DatabaseBackupStatusSuccess,
				FilePath:     filePath,
				FileName:     "expired.sql.gz",
				FinishedAt:   &finishedAt,
				ErrorMessage: buildBackupSuccessMessage("expired.sql.gz", 128),
			},
		},
	}
	uc := &UseCase{
		backupRecordRepo: recordRepo,
		backupPolicyResolver: func(context.Context) (*DatabaseBackupPolicy, error) {
			return &DatabaseBackupPolicy{DefaultRetentionDays: 7, StoragePath: dir}, nil
		},
	}
	task := &DatabaseBackupTask{RetentionDays: 7}
	task.ID = 1
	task.CreatedAt = time.Now()

	cleanedCount, err := uc.cleanupExpiredBackupFilesByTask(context.Background(), task)
	if err != nil {
		t.Fatalf("cleanupExpiredBackupFilesByTask() error = %v", err)
	}
	if cleanedCount != 1 {
		t.Fatalf("cleanupExpiredBackupFilesByTask() = %d, want 1", cleanedCount)
	}
	if _, statErr := os.Stat(filePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected backup file removed, stat error = %v", statErr)
	}
	if len(recordRepo.updated) != 1 {
		t.Fatalf("expected one updated record, got %d", len(recordRepo.updated))
	}
	if recordRepo.updated[0].FilePath != "" {
		t.Fatalf("expected pruned record file path empty, got %q", recordRepo.updated[0].FilePath)
	}
	if recordRepo.updated[0].Status != DatabaseBackupStatusExpired {
		t.Fatalf("expected pruned record status expired, got %q", recordRepo.updated[0].Status)
	}
	if recordRepo.updated[0].ErrorMessage == "" {
		t.Fatalf("expected pruned record message")
	}
}

func TestReconcileStaleBackupRecords(t *testing.T) {
	startedAt := time.Now().Add(-time.Hour)
	recordRepo := &testBackupRecordRepo{
		items: []*DatabaseBackupRecord{
			{
				Model:        gormModelForTest(2),
				TaskID:       1,
				Status:       DatabaseBackupStatusRunning,
				FileName:     "running.sql.gz",
				StartedAt:    &startedAt,
				ErrorMessage: backupRunningMessage,
			},
		},
	}
	uc := &UseCase{
		backupRecordRepo: recordRepo,
		startedAt:        time.Now(),
	}

	uc.reconcileStaleBackupRecords(context.Background(), recordRepo.items)

	if len(recordRepo.updated) != 1 {
		t.Fatalf("expected one updated stale record, got %d", len(recordRepo.updated))
	}
	updated := recordRepo.updated[0]
	if updated.Status != DatabaseBackupStatusFailed {
		t.Fatalf("stale record status = %q, want failed", updated.Status)
	}
	if updated.FinishedAt == nil || updated.DurationMs <= 0 {
		t.Fatalf("expected stale record finished metadata, got finishedAt=%v duration=%d", updated.FinishedAt, updated.DurationMs)
	}
	if updated.ErrorMessage != backupStaleMessage {
		t.Fatalf("stale record message = %q", updated.ErrorMessage)
	}
}

func TestReconcileStaleBackupRecordsKeepsCurrentRun(t *testing.T) {
	startedAt := time.Now()
	recordRepo := &testBackupRecordRepo{
		items: []*DatabaseBackupRecord{
			{
				Model:     gormModelForTest(3),
				TaskID:    1,
				Status:    DatabaseBackupStatusRunning,
				StartedAt: &startedAt,
			},
		},
	}
	uc := &UseCase{
		backupRecordRepo: recordRepo,
		startedAt:        time.Now().Add(-time.Minute),
	}

	uc.reconcileStaleBackupRecords(context.Background(), recordRepo.items)

	if len(recordRepo.updated) != 0 {
		t.Fatalf("expected current running record to stay untouched, got %d updates", len(recordRepo.updated))
	}
}

func TestIsBackupRecordStaleByHeartbeatTimeout(t *testing.T) {
	startedAt := time.Now().Add(-10 * time.Minute)
	lastHeartbeatAt := time.Now().Add(-backupHeartbeatTimeout - time.Second)
	record := &DatabaseBackupRecord{
		Status:          DatabaseBackupStatusRunning,
		StartedAt:       &startedAt,
		LastHeartbeatAt: &lastHeartbeatAt,
	}
	task := &DatabaseBackupTask{MaxDurationMinutes: defaultBackupMaxDurationMinutes}

	if !isBackupRecordStale(record, task, time.Now().Add(-time.Hour), time.Now()) {
		t.Fatalf("expected running record with stale heartbeat to be stale")
	}
}

func gormModelForTest(id uint) gorm.Model {
	return gorm.Model{ID: id, CreatedAt: time.Now()}
}
