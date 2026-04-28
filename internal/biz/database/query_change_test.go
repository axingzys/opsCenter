package database

import "testing"

func TestSupportsWriteExecution(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{dbType: DBTypeMySQL, want: true},
		{dbType: DBTypeMariaDB, want: true},
		{dbType: DBTypePostgreSQL, want: true},
		{dbType: DBTypeSQLServer, want: false},
		{dbType: DBTypeClickHouse, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			if got := supportsWriteExecution(tt.dbType); got != tt.want {
				t.Fatalf("supportsWriteExecution(%q) = %v, want %v", tt.dbType, got, tt.want)
			}
		})
	}
}

func TestIsSupportedWriteExecuteType(t *testing.T) {
	tests := []struct {
		sqlType string
		want    bool
	}{
		{sqlType: "INSERT", want: true},
		{sqlType: "UPDATE", want: true},
		{sqlType: "DELETE", want: true},
		{sqlType: "CREATE", want: false},
		{sqlType: "ALTER", want: false},
		{sqlType: "DROP", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.sqlType, func(t *testing.T) {
			if got := isSupportedWriteExecuteType(tt.sqlType); got != tt.want {
				t.Fatalf("isSupportedWriteExecuteType(%q) = %v, want %v", tt.sqlType, got, tt.want)
			}
		})
	}
}

func TestSupportsDDLExecution(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{dbType: DBTypeMySQL, want: true},
		{dbType: DBTypeMariaDB, want: true},
		{dbType: DBTypePostgreSQL, want: true},
		{dbType: DBTypeSQLServer, want: false},
		{dbType: DBTypeRedis, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			if got := supportsDDLExecution(tt.dbType); got != tt.want {
				t.Fatalf("supportsDDLExecution(%q) = %v, want %v", tt.dbType, got, tt.want)
			}
		})
	}
}

func TestIsSupportedDDLExecuteType(t *testing.T) {
	tests := []struct {
		sqlType string
		want    bool
	}{
		{sqlType: "CREATE", want: true},
		{sqlType: "ALTER", want: false},
		{sqlType: "DROP", want: false},
		{sqlType: "TRUNCATE", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.sqlType, func(t *testing.T) {
			if got := isSupportedDDLExecuteType(tt.sqlType); got != tt.want {
				t.Fatalf("isSupportedDDLExecuteType(%q) = %v, want %v", tt.sqlType, got, tt.want)
			}
		})
	}
}

func TestBuildWriteRollbackSQLHint(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		sqlType string
		want    string
	}{
		{
			name:    "insert hint",
			dbType:  DBTypeMySQL,
			sqlType: "INSERT",
			want:    "暂不支持自动生成 INSERT 回滚 SQL，请基于主键或唯一键手工确认删除语句。",
		},
		{
			name:    "update hint",
			dbType:  DBTypePostgreSQL,
			sqlType: "UPDATE",
			want:    "暂不支持自动生成 UPDATE / DELETE 回滚 SQL，请优先结合逻辑备份、事务日志或变更前快照恢复。",
		},
		{
			name:    "unsupported empty",
			dbType:  DBTypeSQLServer,
			sqlType: "ALTER",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildWriteRollbackSQLHint(tt.dbType, tt.sqlType); got != tt.want {
				t.Fatalf("buildWriteRollbackSQLHint(%q, %q) = %q, want %q", tt.dbType, tt.sqlType, got, tt.want)
			}
		})
	}
}

func TestEnforceRowsAffectedLimit(t *testing.T) {
	tests := []struct {
		name         string
		rowsAffected int64
		limit        int64
		wantErr      bool
	}{
		{name: "below limit", rowsAffected: 9, limit: 10, wantErr: false},
		{name: "equals limit", rowsAffected: 10, limit: 10, wantErr: false},
		{name: "above limit", rowsAffected: 11, limit: 10, wantErr: true},
		{name: "no limit", rowsAffected: 100, limit: 0, wantErr: false},
		{name: "negative rows normalized", rowsAffected: -1, limit: 0, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforceRowsAffectedLimit(tt.rowsAffected, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("enforceRowsAffectedLimit(%d, %d) error = %v, wantErr %v", tt.rowsAffected, tt.limit, err, tt.wantErr)
			}
		})
	}
}
