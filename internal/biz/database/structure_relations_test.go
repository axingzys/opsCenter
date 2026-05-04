package database

import (
	"strings"
	"testing"
)

func TestBuildInferredTableRelationsInfersUserID(t *testing.T) {
	tables := []*DatabaseTable{
		{SchemaName: "public", Name: "users"},
		{SchemaName: "public", Name: "orders"},
	}
	columns := []*DatabaseColumn{
		{SchemaName: "public", Table: "users", ColumnName: "id", ColumnKey: "PRI"},
		{SchemaName: "public", Table: "orders", ColumnName: "id", ColumnKey: "PRI"},
		{SchemaName: "public", Table: "orders", ColumnName: "user_id"},
	}
	indexes := []*DatabaseIndex{
		{SchemaName: "public", Table: "users", IndexName: "PRIMARY", IndexType: "PRIMARY", Columns: "id", IsUnique: true},
	}

	relations := buildInferredTableRelations(tables, columns, indexes, nil)
	if len(relations) != 1 {
		t.Fatalf("expected one inferred relation, got %d", len(relations))
	}
	relation := relations[0]
	if relation.Table != "orders" || relation.ColumnName != "user_id" {
		t.Fatalf("unexpected source relation: %#v", relation)
	}
	if relation.ReferencedTableName != "users" || relation.ReferencedColumnName != "id" {
		t.Fatalf("unexpected target relation: %#v", relation)
	}
	if relation.RelationType != DatabaseTableRelationTypeInferred {
		t.Fatalf("expected inferred relation, got %s", relation.RelationType)
	}
}

func TestBuildInferredTableRelationsSkipsExistingForeignKey(t *testing.T) {
	tables := []*DatabaseTable{
		{SchemaName: "public", Name: "users"},
		{SchemaName: "public", Name: "orders"},
	}
	columns := []*DatabaseColumn{
		{SchemaName: "public", Table: "users", ColumnName: "id", ColumnKey: "PRI"},
		{SchemaName: "public", Table: "orders", ColumnName: "user_id"},
	}
	existing := []*DatabaseTableRelation{
		{
			SchemaName:           "public",
			Table:                "orders",
			ColumnName:           "user_id",
			ReferencedSchemaName: "public",
			ReferencedTableName:  "users",
			ReferencedColumnName: "id",
			RelationType:         DatabaseTableRelationTypeForeignKey,
		},
	}

	relations := buildInferredTableRelations(tables, columns, nil, existing)
	if len(relations) != 0 {
		t.Fatalf("expected existing foreign key to suppress inferred relation, got %d", len(relations))
	}
}

func TestBuildTableRelationSQLSnippetsUseDatabaseQuoting(t *testing.T) {
	relation := &DatabaseTableRelation{
		SchemaName:           "app",
		Table:                "orders",
		ColumnName:           "user_id",
		ReferencedSchemaName: "app",
		ReferencedTableName:  "users",
		ReferencedColumnName: "id",
	}

	mysqlJoin := buildTableRelationJoinSQL(relation, DBTypeMySQL)
	if mysqlJoin != "LEFT JOIN `app`.`users` ref ON src.`user_id` = ref.`id`" {
		t.Fatalf("unexpected mysql join sql: %s", mysqlJoin)
	}

	postgresOrphan := buildTableRelationOrphanCheckSQL(relation, DBTypePostgreSQL)
	if !containsAll(postgresOrphan, `"app"."orders" src`, `"app"."users" ref`, `LIMIT 100`) {
		t.Fatalf("unexpected postgres orphan sql: %s", postgresOrphan)
	}

	sqlServerOrphan := buildTableRelationOrphanCheckSQL(relation, DBTypeSQLServer)
	if !containsAll(sqlServerOrphan, `SELECT TOP (100) src.*`, `[app].[orders] src`, `[app].[users] ref`) {
		t.Fatalf("unexpected sqlserver orphan sql: %s", sqlServerOrphan)
	}
}

func TestBuildTableRelationImpact(t *testing.T) {
	relation := &DatabaseTableRelation{
		SchemaName:           "app",
		Table:                "orders",
		ColumnName:           "user_id",
		ReferencedSchemaName: "app",
		ReferencedTableName:  "users",
		ReferencedColumnName: "id",
		RelationType:         DatabaseTableRelationTypeForeignKey,
		Confidence:           100,
	}

	level, text := buildTableRelationImpact(relation, "incoming")
	if level != "high" || text == "" {
		t.Fatalf("expected high incoming impact, got level=%s text=%s", level, text)
	}

	relation.RelationType = DatabaseTableRelationTypeInferred
	relation.Confidence = 70
	level, text = buildTableRelationImpact(relation, "outgoing")
	if level != "info" || text == "" {
		t.Fatalf("expected info low-confidence outgoing impact, got level=%s text=%s", level, text)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
