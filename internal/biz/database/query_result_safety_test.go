package database

import (
	"strings"
	"testing"
)

func TestSanitizeDatabaseQueryResultMasksTruncatesAndPreviewsBinary(t *testing.T) {
	result := &DatabaseQueryResultVO{
		Columns: []string{"id", "password", "profile_json", "payload"},
		Rows: []map[string]any{{
			"id":           1,
			"password":     "plain-secret",
			"profile_json": strings.Repeat("x", 40),
			"payload":      queryRawBytes{0x00, 0xff, 0x01},
		}},
		Message: "查询成功",
	}

	err := sanitizeDatabaseQueryResult(result, queryResultSafetyPolicy{
		maxCellRunes:  24,
		maxTotalBytes: 1024,
	})
	if err != nil {
		t.Fatalf("sanitizeDatabaseQueryResult() error = %v", err)
	}
	if got := result.Rows[0]["password"]; got != "[已脱敏]" {
		t.Fatalf("password not masked: %#v", got)
	}
	if got := result.Rows[0]["profile_json"].(string); !strings.Contains(got, "已截断") {
		t.Fatalf("long text not marked as truncated: %q", got)
	}
	if got := result.Rows[0]["payload"].(string); !strings.Contains(got, "二进制内容") {
		t.Fatalf("binary payload not previewed: %q", got)
	}
	if !result.CellsMasked || !result.CellTruncated || !result.BinaryPreviewed {
		t.Fatalf("unexpected safety flags: masked=%v truncated=%v binary=%v", result.CellsMasked, result.CellTruncated, result.BinaryPreviewed)
	}
}

func TestSanitizeDatabaseQueryResultRejectsOversizedExport(t *testing.T) {
	result := &DatabaseQueryResultVO{
		Columns: []string{"body"},
		Rows: []map[string]any{
			{"body": strings.Repeat("a", 100)},
			{"body": strings.Repeat("b", 100)},
		},
	}

	err := sanitizeDatabaseQueryResult(result, queryResultSafetyPolicy{
		maxCellRunes:     100,
		maxTotalBytes:    64,
		rejectTotalBytes: true,
	})
	if err == nil {
		t.Fatalf("expected oversized export to be rejected")
	}
	if !strings.Contains(err.Error(), "导出结果超过大小限制") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildQueryExportAuditReason(t *testing.T) {
	result := &DatabaseQueryResultVO{
		Columns:         []string{"id", "name"},
		RowsReturned:    2,
		Truncated:       true,
		CellTruncated:   true,
		CellsMasked:     true,
		BinaryPreviewed: true,
		ResultBytes:     128,
	}

	reason := buildQueryExportAuditReason(result)
	for _, expected := range []string{
		`"exportRows":2`,
		`"columns":2`,
		`"truncated":true`,
		`"cellTruncated":true`,
		`"masked":true`,
		`"binaryPreviewed":true`,
		`"resultBytes":128`,
	} {
		if !strings.Contains(reason, expected) {
			t.Fatalf("expected reason %s to contain %s", reason, expected)
		}
	}
}
