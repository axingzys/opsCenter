package database

import (
	"strings"
	"testing"
)

func testWritePolicy(enabled bool) *DatabaseWritePolicy {
	return &DatabaseWritePolicy{
		WriteEnabled:            enabled,
		HighRiskRequiresConfirm: true,
		OperationReasonRequired: true,
		DDLEnabled:              enabled,
		DDLHighRiskConfirm:      true,
		DDLReasonRequired:       true,
		DDLRequireBackupHint:    true,
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
		"with deleted as (delete from users where id = 1 returning *) select * from deleted",
		"with seed as (select 1) update users set name = 'x' where id = 1",
		"with seed as (select 1) delete from users where id = 1",
		"select 1create table test_table (id bigint primary key)",
		"/*!50000 update users set name = 'x' */ select 1",
		"select /*!50000 sleep(10) */ 1",
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

func TestAnalyzeReadOnlySQLIgnoresExecutableCommentMarkerInsideStrings(t *testing.T) {
	result := AnalyzeReadOnlySQL("select '/*!50000 update users set name = x */' as text", 500)
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

func TestAnalyzeWriteExplainSQLByDB(t *testing.T) {
	enabledPolicy := testWritePolicy(true)
	enabledPolicy.WriteExplainEnabled = true
	disabledPolicy := testWritePolicy(true)
	disabledPolicy.WriteExplainEnabled = false

	tests := []struct {
		name    string
		dbType  string
		sqlText string
		policy  *DatabaseWritePolicy
		wantSQL string
		allowed bool
		message string
	}{
		{
			name:    "update to explain",
			dbType:  DBTypeMySQL,
			sqlText: "update users set name = 'x' where id = 1",
			policy:  enabledPolicy,
			wantSQL: "EXPLAIN update users set name = 'x' where id = 1",
			allowed: true,
		},
		{
			name:    "explicit explain update",
			dbType:  DBTypeMySQL,
			sqlText: "explain update users set name = 'x' where id = 1",
			policy:  enabledPolicy,
			wantSQL: "explain update users set name = 'x' where id = 1",
			allowed: true,
		},
		{
			name:    "postgres merge",
			dbType:  DBTypePostgreSQL,
			sqlText: "merge into users using staging on users.id = staging.id when matched then update set name = staging.name",
			policy:  enabledPolicy,
			wantSQL: "EXPLAIN merge into users using staging on users.id = staging.id when matched then update set name = staging.name",
			allowed: true,
		},
		{
			name:    "disabled switch",
			dbType:  DBTypeMySQL,
			sqlText: "update users set name = 'x' where id = 1",
			policy:  disabledPolicy,
			allowed: false,
			message: "开关未开启",
		},
		{
			name:    "reject explain analyze",
			dbType:  DBTypeMySQL,
			sqlText: "explain analyze update users set name = 'x' where id = 1",
			policy:  enabledPolicy,
			allowed: false,
			message: "EXPLAIN ANALYZE",
		},
		{
			name:    "reject ddl without forbidden keyword noise",
			dbType:  DBTypeMySQL,
			sqlText: "create table test_table (updated_at datetime on update current_timestamp)",
			policy:  enabledPolicy,
			allowed: false,
			message: "DDL 检查",
		},
		{
			name:    "reject unsupported db",
			dbType:  DBTypeOracle,
			sqlText: "update users set name = 'x' where id = 1",
			policy:  enabledPolicy,
			allowed: false,
			message: "暂不支持写 SQL 执行计划",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeWriteExplainSQLByDB(tt.dbType, tt.sqlText, tt.policy)
			if result.Allowed != tt.allowed {
				t.Fatalf("allowed=%v, want %v, message=%s", result.Allowed, tt.allowed, result.Message)
			}
			if tt.allowed {
				if result.SQLText != tt.wantSQL {
					t.Fatalf("unexpected explain SQL: %s", result.SQLText)
				}
				if result.SQLType != "EXPLAIN" {
					t.Fatalf("unexpected SQL type: %s", result.SQLType)
				}
				return
			}
			if tt.message != "" && !strings.Contains(result.Message, tt.message) {
				t.Fatalf("expected message to contain %q, got %q", tt.message, result.Message)
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
			message:   "当前受控写入仅支持 INSERT / UPDATE / DELETE，DDL 请使用 DDL 检查 / DDL 执行",
			confirm:   true,
		},
		{
			name:      "create routed to ddl",
			dbType:    DBTypeMySQL,
			sqlText:   "create table test_table (id bigint primary key)",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "CREATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "当前受控写入仅支持 INSERT / UPDATE / DELETE，DDL 请使用 DDL 检查 / DDL 执行",
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
		{
			name:      "reject executable comment",
			dbType:    DBTypeMySQL,
			sqlText:   "/*!50000 update users set name = 'x' where id = 1 */",
			policy:    testWritePolicy(true),
			allowed:   false,
			sqlType:   "UNKNOWN",
			riskLevel: DatabaseQueryRiskLow,
			message:   "禁止使用数据库可执行注释",
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

func TestAnalyzeDDLSQLByDB(t *testing.T) {
	enabledPolicy := testWritePolicy(true)
	disabledPolicy := testWritePolicy(true)
	disabledPolicy.DDLEnabled = false

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
			name:      "create table allowed",
			dbType:    DBTypeMySQL,
			sqlText:   "create table test_table (id bigint primary key)",
			policy:    enabledPolicy,
			allowed:   true,
			sqlType:   "CREATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "通过 DDL 结构变更检查，执行前需二次确认",
			confirm:   true,
		},
		{
			name:      "create unique index allowed",
			dbType:    DBTypePostgreSQL,
			sqlText:   "create unique index idx_users_name on users(name)",
			policy:    enabledPolicy,
			allowed:   true,
			sqlType:   "CREATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "通过 DDL 结构变更检查，执行前需二次确认",
			confirm:   true,
		},
		{
			name:      "ddl switch disabled",
			dbType:    DBTypeMySQL,
			sqlText:   "create table test_table (id bigint primary key)",
			policy:    disabledPolicy,
			allowed:   false,
			sqlType:   "CREATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "数据库 DDL 结构变更开关未开启",
			confirm:   true,
		},
		{
			name:      "drop denied",
			dbType:    DBTypeMySQL,
			sqlText:   "drop table users",
			policy:    enabledPolicy,
			allowed:   false,
			sqlType:   "DROP",
			riskLevel: DatabaseQueryRiskCritical,
			message:   "默认禁止 DROP / TRUNCATE",
			confirm:   true,
		},
		{
			name:      "alter not opened yet",
			dbType:    DBTypeMySQL,
			sqlText:   "alter table users add column memo varchar(100)",
			policy:    enabledPolicy,
			allowed:   false,
			sqlType:   "ALTER",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "当前 DDL 执行仅支持 CREATE TABLE / CREATE INDEX",
			confirm:   true,
		},
		{
			name:      "create database denied",
			dbType:    DBTypeMySQL,
			sqlText:   "create database app",
			policy:    enabledPolicy,
			allowed:   false,
			sqlType:   "CREATE",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "当前 DDL 执行仅支持 CREATE TABLE / CREATE INDEX",
			confirm:   true,
		},
		{
			name:      "reject unsupported db",
			dbType:    DBTypeRedis,
			sqlText:   "create table test_table (id bigint primary key)",
			policy:    enabledPolicy,
			allowed:   false,
			sqlType:   "UNKNOWN",
			riskLevel: DatabaseQueryRiskHigh,
			message:   "Redis 暂不支持 DDL 结构变更检查",
			confirm:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeDDLSQLByDB(tt.dbType, tt.sqlText, tt.policy)
			if result.Allowed != tt.allowed {
				t.Fatalf("expected allowed=%v, got %v, message=%s", tt.allowed, result.Allowed, result.Message)
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
		})
	}
}
