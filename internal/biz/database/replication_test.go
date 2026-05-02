package database

import "testing"

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
	flags := mysqlReplicaRiskFlags("Yes", "No", 601, 3600, 0, 0)
	assertContainsFlag(t, flags, "SQL apply 线程异常")
	assertContainsFlag(t, flags, "复制延迟过大")
	assertContainsFlag(t, flags, "延迟副本已追上")
	assertContainsFlag(t, flags, "来源主库未匹配")
	if got := replicaHealthFromRiskFlags(flags); got != DatabaseReplicaHealthCritical {
		t.Fatalf("replicaHealthFromRiskFlags() = %q, want %q", got, DatabaseReplicaHealthCritical)
	}
}

func TestRedactReplicaValue(t *testing.T) {
	raw := "user=opshub password=secret host=127.0.0.1"
	got := redactReplicaValue("conninfo", raw)
	if got == raw || got != "user=opshub password=****** host=127.0.0.1" {
		t.Fatalf("redactReplicaValue() = %q", got)
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
