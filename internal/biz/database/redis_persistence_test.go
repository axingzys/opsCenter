package database

import (
	"strconv"
	"testing"
	"time"
)

func TestBuildRedisPersistenceNodeState(t *testing.T) {
	now := time.Date(2026, 4, 25, 21, 0, 0, 0, time.Local)
	state := buildRedisPersistenceNodeState(
		"127.0.0.1:6379",
		"master",
		map[string]string{
			"aof_enabled":               "1",
			"rdb_last_save_time":        strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10),
			"rdb_last_bgsave_status":    "ok",
			"aof_last_bgrewrite_status": "ok",
			"aof_last_write_status":     "ok",
		},
		map[string]string{
			"appendonly":  "yes",
			"appendfsync": "everysec",
			"save":        "900 1 300 10",
		},
		now,
	)

	if !state.aofEnabled {
		t.Fatalf("expected aofEnabled")
	}
	if !state.rdbEnabled {
		t.Fatalf("expected rdbEnabled")
	}
	if state.appendFsyncNo {
		t.Fatalf("expected appendFsyncNo false")
	}
	if state.staleRDBOnly {
		t.Fatalf("expected staleRDBOnly false")
	}
}

func TestBuildRedisPersistenceNodeStateStaleRDBOnly(t *testing.T) {
	now := time.Date(2026, 4, 25, 21, 0, 0, 0, time.Local)
	state := buildRedisPersistenceNodeState(
		"127.0.0.1:6379",
		"master",
		map[string]string{
			"aof_enabled":            "0",
			"rdb_last_save_time":     strconv.FormatInt(now.Add(-48*time.Hour).Unix(), 10),
			"rdb_last_bgsave_status": "ok",
		},
		map[string]string{
			"appendonly": "no",
			"save":       "900 1",
		},
		now,
	)

	if state.aofEnabled {
		t.Fatalf("expected aofEnabled false")
	}
	if !state.rdbEnabled {
		t.Fatalf("expected rdbEnabled true")
	}
	if !state.staleRDBOnly {
		t.Fatalf("expected staleRDBOnly true")
	}
}

func TestRedisRDBPersistenceEnabled(t *testing.T) {
	if !redisRDBPersistenceEnabled("", map[string]string{"rdb_last_save_time": "123"}) {
		t.Fatalf("expected last save time to imply rdb enabled")
	}
	if redisRDBPersistenceEnabled("", map[string]string{}) {
		t.Fatalf("expected empty config without save history to be disabled")
	}
}
