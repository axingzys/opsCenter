package database

import (
	"testing"
	"time"
)

func TestShouldTriggerBackupTask(t *testing.T) {
	lastRunAt := time.Date(2026, 4, 24, 2, 0, 0, 0, time.Local)
	task := &DatabaseBackupTask{
		Schedule:  "0 2 * * *",
		LastRunAt: &lastRunAt,
	}

	due, nextRun, err := shouldTriggerBackupTask(task, time.Date(2026, 4, 25, 2, 0, 1, 0, time.Local))
	if err != nil {
		t.Fatalf("shouldTriggerBackupTask() error = %v", err)
	}
	if !due {
		t.Fatalf("expected task due at %s", nextRun)
	}
	if nextRun.Format("2006-01-02 15:04:05") != "2026-04-25 02:00:00" {
		t.Fatalf("unexpected next run: %s", nextRun.Format("2006-01-02 15:04:05"))
	}
}

func TestShouldTriggerBackupTaskNotDue(t *testing.T) {
	createdAt := time.Date(2026, 4, 24, 1, 0, 0, 0, time.Local)
	task := &DatabaseBackupTask{Schedule: "30 3 * * *"}
	task.ID = 1
	task.CreatedAt = createdAt

	due, nextRun, err := shouldTriggerBackupTask(task, time.Date(2026, 4, 24, 3, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("shouldTriggerBackupTask() error = %v", err)
	}
	if due {
		t.Fatalf("expected task not due before %s", nextRun)
	}
	if nextRun.Format("2006-01-02 15:04:05") != "2026-04-24 03:30:00" {
		t.Fatalf("unexpected next run: %s", nextRun.Format("2006-01-02 15:04:05"))
	}
}
