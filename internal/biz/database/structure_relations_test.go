package database

import "testing"

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
