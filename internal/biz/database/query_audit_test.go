package database

import "testing"

func TestNormalizeAuditAction(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{name: "empty defaults to query", input: "", output: DatabaseAuditActionQuery},
		{name: "query export", input: "QUERY_EXPORT", output: DatabaseAuditActionQueryExport},
		{name: "diagnosis slow", input: "diagnosis_slow_queries", output: DatabaseAuditActionDiagnosisSlowQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAuditAction(tt.input); got != tt.output {
				t.Fatalf("normalizeAuditAction(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestQueryAuditActionText(t *testing.T) {
	if got := QueryAuditActionText(DatabaseAuditActionMetadataExport); got != "数据字典导出" {
		t.Fatalf("unexpected action text: %s", got)
	}
	if got := QueryAuditActionText(DatabaseAuditActionChangeExecute); got != "写操作执行" {
		t.Fatalf("unexpected change action text: %s", got)
	}
	if got := QueryAuditActionText(""); got != "只读查询" {
		t.Fatalf("expected empty action to fallback to query text, got %s", got)
	}
}

func TestQueryRiskLevelText(t *testing.T) {
	if got := QueryRiskLevelText(DatabaseQueryRiskMedium); got != "中" {
		t.Fatalf("unexpected medium risk text: %s", got)
	}
	if got := QueryRiskLevelText(DatabaseQueryRiskCritical); got != "严重" {
		t.Fatalf("unexpected critical risk text: %s", got)
	}
}

func TestBuildDiagnosisAuditSQL(t *testing.T) {
	if got := buildDiagnosisAuditSQL(DatabaseAuditActionDiagnosisSessions, 25); got != "SHOW SESSIONS LIMIT 25" {
		t.Fatalf("unexpected diagnosis audit sql: %s", got)
	}
	if got := buildMetadataExportAuditSQL("audit", "events"); got != "EXPORT DICTIONARY audit.events" {
		t.Fatalf("unexpected metadata audit sql: %s", got)
	}
}
