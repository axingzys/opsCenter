package database

import (
	"strings"
	"testing"
)

func TestBuildGeneratedDDLForMySQLTable(t *testing.T) {
	table := &DatabaseTable{
		SchemaName: "app",
		Name:       "users",
		TableType:  "BASE TABLE",
		Engine:     "InnoDB",
		Comment:    "用户表",
	}
	columns := []*DatabaseColumn{
		{ColumnName: "id", DataType: "bigint", IsNullable: false, ColumnKey: "PRI", Comment: "主键"},
		{ColumnName: "email", DataType: "varchar(255)", IsNullable: false, DefaultValue: "'a@example.com'"},
	}
	indexes := []*DatabaseIndex{
		{IndexName: "PRIMARY", IndexType: "PRIMARY", Columns: "id", IsUnique: true},
		{IndexName: "uniq_email", IndexType: "BTREE", Columns: "email", IsUnique: true},
	}

	ddl := buildGeneratedDDL(DBTypeMySQL, table, columns, indexes)
	expectations := []string{
		"CREATE TABLE `app`.`users` (",
		"`id` bigint NOT NULL",
		"PRIMARY KEY (`id`)",
		"UNIQUE KEY `uniq_email` (`email`)",
		"-- Engine: InnoDB",
	}
	for _, expected := range expectations {
		if !strings.Contains(ddl, expected) {
			t.Fatalf("expected DDL to contain %q, got:\n%s", expected, ddl)
		}
	}
}

func TestBuildGeneratedDDLForPostgreSQLTable(t *testing.T) {
	table := &DatabaseTable{
		SchemaName: "public",
		Name:       "orders",
		TableType:  "BASE TABLE",
	}
	columns := []*DatabaseColumn{
		{ColumnName: "order_id", DataType: "uuid", IsNullable: false, ColumnKey: "PRI"},
		{ColumnName: "created_at", DataType: "timestamp", IsNullable: false, DefaultValue: "CURRENT_TIMESTAMP"},
	}

	ddl := buildGeneratedDDL(DBTypePostgreSQL, table, columns, nil)
	if !strings.Contains(ddl, `CREATE TABLE "public"."orders" (`) {
		t.Fatalf("expected postgres quoted table name, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, `"created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP`) {
		t.Fatalf("expected postgres column definition, got:\n%s", ddl)
	}
}

func TestBuildGeneratedDDLForSQLServerTable(t *testing.T) {
	table := &DatabaseTable{
		SchemaName: "dbo",
		Name:       "audit_log",
		TableType:  "BASE TABLE",
	}
	columns := []*DatabaseColumn{
		{ColumnName: "id", DataType: "bigint", IsNullable: false, ColumnKey: "PRI"},
		{ColumnName: "created_at", DataType: "datetime2(7)", IsNullable: false},
	}
	indexes := []*DatabaseIndex{
		{IndexName: "PRIMARY", IndexType: "PRIMARY", Columns: "[id]", IsUnique: true},
	}

	ddl := buildGeneratedDDL(DBTypeSQLServer, table, columns, indexes)
	if !strings.Contains(ddl, "CREATE TABLE [dbo].[audit_log] (") {
		t.Fatalf("expected sqlserver qualified name, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, "PRIMARY KEY ([id])") {
		t.Fatalf("expected sqlserver primary key, got:\n%s", ddl)
	}
}

func TestBuildGeneratedDDLForOracleTable(t *testing.T) {
	table := &DatabaseTable{
		SchemaName: "APP",
		Name:       "ORDERS",
		TableType:  "BASE TABLE",
	}
	columns := []*DatabaseColumn{
		{ColumnName: "ORDER_ID", DataType: "NUMBER(20,0)", IsNullable: false, ColumnKey: "PRI"},
	}

	ddl := buildGeneratedDDL(DBTypeOracle, table, columns, nil)
	if !strings.Contains(ddl, `CREATE TABLE "APP"."ORDERS" (`) {
		t.Fatalf("expected oracle quoted table name, got:\n%s", ddl)
	}
}

func TestBuildGeneratedDDLForViewFallback(t *testing.T) {
	table := &DatabaseTable{
		SchemaName: "public",
		Name:       "active_users",
		TableType:  "VIEW",
	}
	columns := []*DatabaseColumn{
		{ColumnName: "id", DataType: "bigint"},
		{ColumnName: "name", DataType: "varchar(64)"},
	}

	ddl := buildGeneratedDDL(DBTypePostgreSQL, table, columns, nil)
	if !strings.Contains(ddl, "当前对象为 VIEW") {
		t.Fatalf("expected view fallback message, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, "--   id bigint") {
		t.Fatalf("expected view columns in fallback output, got:\n%s", ddl)
	}
}

func TestBuildIndexSummaryMap(t *testing.T) {
	indexes := []*DatabaseIndex{
		{IndexName: "PRIMARY", Columns: "id", IndexType: "PRIMARY", IsUnique: true},
		{IndexName: "idx_user_email", Columns: "email", IndexType: "BTREE"},
		{IndexName: "uniq_account", Columns: "tenant_id, account_id", IndexType: "BTREE", IsUnique: true},
	}

	summary := buildIndexSummaryMap(indexes)
	if len(summary["tenant_id"]) != 1 || !strings.Contains(summary["tenant_id"][0], "uniq_account") {
		t.Fatalf("expected tenant_id summary to include uniq_account, got: %#v", summary["tenant_id"])
	}
	if len(summary["email"]) != 1 || !strings.Contains(summary["email"][0], "idx_user_email") {
		t.Fatalf("expected email summary to include idx_user_email, got: %#v", summary["email"])
	}
}
