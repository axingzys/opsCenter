package database

import "testing"

func testWritePolicy(enabled bool) *DatabaseWritePolicy {
	return &DatabaseWritePolicy{
		WriteEnabled:            enabled,
		HighRiskRequiresConfirm: true,
		OperationReasonRequired: true,
		MaxAffectedRows:         1000,
	}
}

func TestAnalyzeReadOnlySQLAllowsReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		sqlText string
		sqlType string
	}{
		{name: "select", sqlText: "select * from users", sqlType: "SELECT"},
		{name: "show", sqlText: "show tables", sqlType: "SHOW"},
		{name: "explain", sqlText: "explain select * from users", sqlType: "EXPLAIN"},
		{name: "with", sqlText: "with t as (select 1) select * from t", sqlType: "WITH"},
		{name: "trailing semicolon", sqlText: "select 1;", sqlType: "SELECT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeReadOnlySQL(tt.sqlText, 500)
			if !result.Allowed {
				t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
			}
			if result.SQLType != tt.sqlType {
				t.Fatalf("expected type %s, got %s", tt.sqlType, result.SQLType)
			}
		})
	}
}

func TestAnalyzeReadOnlySQLRejectsUnsafeSQL(t *testing.T) {
	tests := []string{
		"update users set name = 'x'",
		"select * from users; drop table users",
		"explain delete from users where id = 1",
		"select * from users where id in (delete from t)",
		"use mysql",
		"select * from users into outfile '/tmp/users.csv'",
		"select * from users into dumpfile '/tmp/users.bin'",
		"select load_file('/etc/passwd')",
		"select sleep(10)",
		"select benchmark(1000000, md5('x'))",
	}
	for _, sqlText := range tests {
		t.Run(sqlText, func(t *testing.T) {
			result := AnalyzeReadOnlySQL(sqlText, 500)
			if result.Allowed {
				t.Fatalf("expected SQL to be rejected: %s", sqlText)
			}
		})
	}
}

func TestAnalyzeReadOnlySQLIgnoresKeywordsInsideStrings(t *testing.T) {
	result := AnalyzeReadOnlySQL("select 'drop table x; update y' as text", 500)
	if !result.Allowed {
		t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
	}
}

func TestAnalyzeReadOnlySQLAppendsLimit(t *testing.T) {
	result := AnalyzeReadOnlySQL("select * from users", 200)
	if !result.Allowed {
		t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
	}
	if result.SQLText != "select * from users LIMIT 200" {
		t.Fatalf("unexpected SQL after limit rewrite: %s", result.SQLText)
	}
}

func TestAnalyzeReadOnlySQLRawDoesNotAppendLimit(t *testing.T) {
	result := AnalyzeReadOnlySQLRaw("select * from users")
	if !result.Allowed {
		t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
	}
	if result.SQLText != "select * from users" {
		t.Fatalf("unexpected SQL after raw analysis: %s", result.SQLText)
	}
}

func TestAnalyzeExplainSQL(t *testing.T) {
	tests := []struct {
		name      string
		sqlText   string
		wantSQL   string
		shouldErr bool
	}{
		{name: "select to explain", sqlText: "select * from users", wantSQL: "EXPLAIN select * from users"},
		{name: "with to explain", sqlText: "with t as (select 1) select * from t", wantSQL: "EXPLAIN with t as (select 1) select * from t"},
		{name: "explicit explain", sqlText: "explain select * from users", wantSQL: "explain select * from users"},
		{name: "reject explain analyze", sqlText: "explain analyze select * from users", shouldErr: true},
		{name: "reject show", sqlText: "show tables", shouldErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeExplainSQL(tt.sqlText)
			if tt.shouldErr {
				if result.Allowed {
					t.Fatalf("expected SQL to be rejected: %s", tt.sqlText)
				}
				return
			}
			if !result.Allowed {
				t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
			}
			if result.SQLText != tt.wantSQL {
				t.Fatalf("unexpected explain SQL: %s", result.SQLText)
			}
			if result.SQLType != "EXPLAIN" {
				t.Fatalf("unexpected SQL type: %s", result.SQLType)
			}
		})
	}
}

func TestAnalyzeReadOnlySQLByDBDoesNotAppendLimitForSQLServerAndOracle(t *testing.T) {
	tests := []struct {
		dbType string
		want   string
	}{
		{dbType: DBTypeSQLServer, want: "select * from users"},
		{dbType: DBTypeOracle, want: "select * from users"},
		{dbType: DBTypeClickHouse, want: "select * from users LIMIT 200"},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			result := AnalyzeReadOnlySQLByDB(tt.dbType, "select * from users", 200)
			if !result.Allowed {
				t.Fatalf("expected SQL to be allowed, got: %s", result.Message)
			}
			if result.SQLText != tt.want {
				t.Fatalf("unexpected SQL after rewrite: %s", result.SQLText)
			}
		})
	}
}

func TestAnalyzeExplainSQLByDBRejectsUnsupportedEngines(t *testing.T) {
	tests := []string{DBTypeSQLServer, DBTypeOracle}
	for _, dbType := range tests {
		t.Run(dbType, func(t *testing.T) {
			result := AnalyzeExplainSQLByDB(dbType, "select * from users")
			if result.Allowed {
				t.Fatalf("expected explain to be rejected for %s", dbType)
			}
		})
	}
}

func TestAnalyzeWriteSQLByDB(t *testing.T) {
	tests := []struct {
		name      string
		dbType    string
		sqlText   string
		policy    *DatabaseWritePolicy
		allowed   bool
		sqlType   string
		riskLevel string
		message   string
		confirm   bool
	}{
		{
			name:      "insert allowed",
			dbType:    DBTypeMySQL,
			sqlText:   "insert into users(id, name) values (1, 'alice')",
			policy:    testWritePolicy(true),
			allowed:   true,
			sqlType:   "INSERT",
			riskLevel: DatabaseQueryRiskMedium,
			message:   "通过写操作预检查",
			confirm:   false,
		},
		{
			name:      "update requires confirm",
			dbType:    DBTypeMySQL,
			sqlText:   "update users set status = 'disabled' where id = 1",
			policy:    testWritePolicy(true),
			allowed:   true,
			sqlType:   "UPDATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "通过写操作预检查，执行前需二次确认",
			confirm:   true,
		},
		{
			name:      "write switch disabled",
			dbType:    DBTypePostgreSQL,
			sqlText:   "insert into users(id) values (1)",
			policy:    testWritePolicy(false),
			allowed:   false,
			sqlType:   "INSERT",
			riskLevel: DatabaseQueryRiskMedium,
			message:   "数据库写操作总开关未开启",
			confirm:   false,
		},
		{
			name:      "delete without where denied",
			dbType:    DBTypeMySQL,
			sqlText:   "delete from users",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "DELETE",
			riskLevel: DatabaseQueryRiskCritical,
			message:   "默认禁止无 WHERE 的 UPDATE / DELETE",
			confirm:   true,
		},
		{
			name:      "drop denied",
			dbType:    DBTypeMySQL,
			sqlText:   "drop table users",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "DROP",
			riskLevel: DatabaseQueryRiskCritical,
			message:   "默认禁止 DROP / TRUNCATE",
			confirm:   true,
		},
		{
			name:      "reject read only sql",
			dbType:    DBTypeMySQL,
			sqlText:   "select * from users",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "SELECT",
			riskLevel: DatabaseQueryRiskLow,
			message:   "当前接口仅支持写操作预检查",
			confirm:   false,
		},
		{
			name:      "reject unsupported db",
			dbType:    DBTypeRedis,
			sqlText:   "set k v",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "UNKNOWN",
			riskLevel: DatabaseQueryRiskLow,
			message:   "Redis 暂不支持写操作预检查",
			confirm:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeWriteSQLByDB(tt.dbType, tt.sqlText, tt.policy)
			if result.Allowed != tt.allowed {
				t.Fatalf("expected allowed=%v, got %v", tt.allowed, result.Allowed)
			}
			if result.SQLType != tt.sqlType {
				t.Fatalf("expected sqlType=%s, got %s", tt.sqlType, result.SQLType)
			}
			if result.RiskLevel != tt.riskLevel {
				t.Fatalf("expected riskLevel=%s, got %s", tt.riskLevel, result.RiskLevel)
			}
			if result.Message != tt.message {
				t.Fatalf("expected message=%s, got %s", tt.message, result.Message)
			}
			if result.ConfirmRequired != tt.confirm {
				t.Fatalf("expected confirmRequired=%v, got %v", tt.confirm, result.ConfirmRequired)
			}
			if result.ReasonRequired != tt.policy.OperationReasonRequired {
				t.Fatalf("expected reasonRequired=%v, got %v", tt.policy.OperationReasonRequired, result.ReasonRequired)
			}
			if result.RowsAffectedLimit != tt.policy.MaxAffectedRows {
				t.Fatalf("expected rowsAffectedLimit=%d, got %d", tt.policy.MaxAffectedRows, result.RowsAffectedLimit)
			}
		})
	}
}
