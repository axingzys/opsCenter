package database

import (
	"strings"
	"testing"
	"time"
)

func TestParsePostgreSQLDelaySeconds(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{name: "zero", raw: "0", want: 0},
		{name: "seconds", raw: "30s", want: 30},
		{name: "minutes", raw: "5min", want: 300},
		{name: "time", raw: "01:02:03", want: 3723},
		{name: "mixed", raw: "1 hour 2 minutes 3 seconds", want: 3723},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePostgreSQLDelaySeconds(tc.raw); got != tc.want {
				t.Fatalf("parsePostgreSQLDelaySeconds(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestMySQLReplicaRiskFlags(t *testing.T) {
	flags := mysqlReplicaRiskFlags("Yes", "No", 601, 0, -1, 0)
	assertContainsFlag(t, flags, "SQL apply 线程异常")
	assertContainsFlag(t, flags, "复制延迟过大")
	assertContainsFlag(t, flags, "来源主库未匹配")
	if got := replicaHealthFromRiskFlags(flags); got != DatabaseReplicaHealthCritical {
		t.Fatalf("replicaHealthFromRiskFlags() = %q, want %q", got, DatabaseReplicaHealthCritical)
	}
}

func TestMySQLDelayedReplicaRiskFlagsDoNotTreatConfiguredDelayAsLag(t *testing.T) {
	flags := mysqlReplicaRiskFlags("Yes", "Yes", 3603, 3600, 1, 1)
	assertNotContainsFlag(t, flags, "复制延迟过大")
	assertNotContainsFlag(t, flags, "延迟副本已追上")
	if len(flags) != 0 {
		t.Fatalf("flags = %v, want empty for healthy delayed replica", flags)
	}
}

func TestMySQLDelayedReplicaRiskFlagsWarnUnknownRemainingDelay(t *testing.T) {
	flags := mysqlReplicaRiskFlags("Yes", "Yes", 3603, 3600, -1, 1)
	assertContainsFlag(t, flags, "剩余保护窗口未知")
	assertNotContainsFlag(t, flags, "复制延迟过大")
}

func TestMySQLTopologyVariableRiskFlags(t *testing.T) {
	replicaFlags := mysqlTopologyVariableRiskFlags(DatabaseReplicationRoleReplica, map[string]string{
		"server_id":       "2",
		"read_only":       "OFF",
		"super_read_only": "0",
	})
	assertContainsFlag(t, replicaFlags, "从库未开启只读保护")
	assertContainsFlag(t, replicaFlags, "从库未开启 super_read_only")

	primaryFlags := mysqlTopologyVariableRiskFlags(DatabaseReplicationRolePrimary, map[string]string{
		"server_id": "0",
		"log_bin":   "OFF",
	})
	assertContainsFlag(t, primaryFlags, "server_id 配置无效")
	assertContainsFlag(t, primaryFlags, "主库未开启 binlog")
}

func TestRedactReplicaValue(t *testing.T) {
	raw := "user=opshub password=secret host=127.0.0.1"
	got := redactReplicaValue("conninfo", raw)
	if got == raw || got != "user=opshub password=****** host=127.0.0.1" {
		t.Fatalf("redactReplicaValue() = %q", got)
	}
}

func TestPostgreSQLWALReceiverStatusQueryUsesPortableLSNColumns(t *testing.T) {
	query := postgreSQLWALReceiverStatusQuery()
	if strings.Contains(query, "received_lsn::") {
		t.Fatalf("pg_stat_wal_receiver has no received_lsn column on supported PostgreSQL versions")
	}
	if !strings.Contains(query, "written_lsn::text") {
		t.Fatalf("expected query to use written_lsn as the received_lsn compatibility alias")
	}
	if !strings.Contains(query, "flushed_lsn::text") {
		t.Fatalf("expected query to expose flushed_lsn for standby receiver diagnostics")
	}
}

func TestReplicationCheckTriggerSourceText(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		text string
	}{
		{name: "empty defaults to manual", raw: "", want: DatabaseReplicationCheckTriggerManual, text: "手动采集"},
		{name: "batch", raw: DatabaseReplicationCheckTriggerBatch, want: DatabaseReplicationCheckTriggerBatch, text: "批量采集"},
		{name: "scheduler", raw: DatabaseReplicationCheckTriggerScheduler, want: DatabaseReplicationCheckTriggerScheduler, text: "自动调度"},
		{name: "topology", raw: DatabaseReplicationCheckTriggerTopologyAutoRefresh, want: DatabaseReplicationCheckTriggerTopologyAutoRefresh, text: "拓扑自动补采"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeReplicationCheckTriggerSource(tc.raw); got != tc.want {
				t.Fatalf("normalizeReplicationCheckTriggerSource(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			if got := ReplicationCheckTriggerSourceText(tc.raw); got != tc.text {
				t.Fatalf("ReplicationCheckTriggerSourceText(%q) = %q, want %q", tc.raw, got, tc.text)
			}
		})
	}
}

func TestNewerTime(t *testing.T) {
	base := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	older := base.Add(-time.Minute)
	newer := base.Add(time.Minute)
	if newerTime(nil, &base) {
		t.Fatalf("nil candidate should not be newer")
	}
	if newerTime(&older, &base) {
		t.Fatalf("older candidate should not be newer")
	}
	if !newerTime(&newer, &base) {
		t.Fatalf("newer candidate should be newer")
	}
	if !newerTime(&base, nil) {
		t.Fatalf("candidate should be newer than nil current")
	}
}

func TestReplicaProtectionRiskMessagesAcceptsSmallRemainingWindow(t *testing.T) {
	replica := &DatabaseInstanceReplica{Status: DatabaseReplicaHealthHealthy}
	check := &DatabaseReplicationCheck{
		Engine:                 DBTypeMySQL,
		HealthStatus:           DatabaseReplicaHealthHealthy,
		ConfiguredDelaySeconds: 120,
		SecondsBehindSource:    123,
		RemainingDelaySeconds:  0,
	}
	vo := &DatabaseReplicaProtectionVO{RemainingDelaySeconds: 0}
	thresholds := replicaProtectionThresholds{
		lagWarningSeconds:            300,
		lagCriticalSeconds:           1800,
		remainingDelayWarningSeconds: 30,
	}
	messages := replicaProtectionRiskMessages(replica, check, vo, thresholds)
	assertNotContainsFlag(t, messages, "延迟副本已追上，当前没有可截停窗口")
	assertNotContainsFlag(t, messages, "剩余保护窗口偏小")
	if got := replicaProtectionRiskLevel(messages, replica, check); got != DatabaseReplicaHealthHealthy {
		t.Fatalf("replicaProtectionRiskLevel() = %q, want %q", got, DatabaseReplicaHealthHealthy)
	}
}

func TestReplicaProtectionStatus(t *testing.T) {
	if got := replicaProtectionStatus(DatabaseReplicaHealthHealthy, &DatabaseReplicaProtectionVO{HasDelayedReplica: true}); got != DatabaseReplicaProtectionProtected {
		t.Fatalf("replicaProtectionStatus() = %q, want %q", got, DatabaseReplicaProtectionProtected)
	}
	if got := replicaProtectionStatus(DatabaseReplicaHealthWarning, &DatabaseReplicaProtectionVO{HasDelayedReplica: true}); got != DatabaseReplicaProtectionDegraded {
		t.Fatalf("replicaProtectionStatus() = %q, want %q", got, DatabaseReplicaProtectionDegraded)
	}
	if got := replicaProtectionStatus(DatabaseReplicaHealthHealthy, &DatabaseReplicaProtectionVO{}); got != DatabaseReplicaProtectionUnprotected {
		t.Fatalf("replicaProtectionStatus() = %q, want %q", got, DatabaseReplicaProtectionUnprotected)
	}
}

func assertContainsFlag(t *testing.T, flags []string, want string) {
	t.Helper()
	for _, flag := range flags {
		if flag == want {
			return
		}
	}
	t.Fatalf("flags %v does not contain %q", flags, want)
}

func assertNotContainsFlag(t *testing.T, flags []string, want string) {
	t.Helper()
	for _, flag := range flags {
		if flag == want {
			t.Fatalf("flags %v should not contain %q", flags, want)
		}
	}
}
