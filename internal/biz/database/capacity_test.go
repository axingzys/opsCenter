package database

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestBuildCapacitySnapshotsAggregatesTablesOnce(t *testing.T) {
	collectedAt := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	instance := &DatabaseInstance{Model: gorm.Model{ID: 42}, Name: "main", DBType: DBTypeMySQL}
	schemas := []*DatabaseSchema{
		{InstanceID: 42, SchemaName: "app", TableCount: 99, SizeBytes: 999999},
	}
	tables := []*DatabaseTable{
		{InstanceID: 42, SchemaName: "app", Name: "orders", RowCount: 100, DataSizeBytes: 120, IndexSizeBytes: 30},
		{InstanceID: 42, SchemaName: "app", Name: "users", RowCount: 20, DataSizeBytes: 40, IndexSizeBytes: 10},
	}

	snapshots := buildCapacitySnapshots(instance, schemas, tables, collectedAt)

	if got, want := len(snapshots), 4; got != want {
		t.Fatalf("snapshot count = %d, want %d", got, want)
	}
	instanceSnapshot := findSnapshotByObject(snapshots, DatabaseCapacityObjectInstance, "", "")
	if instanceSnapshot == nil {
		t.Fatal("instance snapshot missing")
	}
	if got, want := instanceSnapshot.TotalSizeBytes, int64(200); got != want {
		t.Fatalf("instance total size = %d, want %d", got, want)
	}
	if got, want := instanceSnapshot.TableCount, 2; got != want {
		t.Fatalf("instance table count = %d, want %d", got, want)
	}
	if got, want := instanceSnapshot.RowCount, int64(120); got != want {
		t.Fatalf("instance row count = %d, want %d", got, want)
	}

	schemaSnapshot := findSnapshotByObject(snapshots, DatabaseCapacityObjectSchema, "app", "")
	if schemaSnapshot == nil {
		t.Fatal("schema snapshot missing")
	}
	if got, want := schemaSnapshot.TotalSizeBytes, int64(200); got != want {
		t.Fatalf("schema total size = %d, want %d", got, want)
	}
	if got, want := schemaSnapshot.TableCount, 2; got != want {
		t.Fatalf("schema table count = %d, want %d", got, want)
	}
}

func TestNormalizeCapacityRange(t *testing.T) {
	cases := []struct {
		name string
		req  *DatabaseCapacityTrendRequest
		want string
	}{
		{name: "default", req: nil, want: "7d"},
		{name: "one day alias", req: &DatabaseCapacityTrendRequest{Range: "1d"}, want: "24h"},
		{name: "thirty days", req: &DatabaseCapacityTrendRequest{Range: "30d"}, want: "30d"},
		{name: "unknown", req: &DatabaseCapacityTrendRequest{Range: "bad"}, want: "7d"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, since := normalizeCapacityRange(tt.req)
			if got != tt.want {
				t.Fatalf("range = %q, want %q", got, tt.want)
			}
			if since.IsZero() {
				t.Fatal("since should not be zero")
			}
		})
	}
}

func findSnapshotByObject(items []*DatabaseCapacitySnapshot, objectType, schemaName, tableName string) *DatabaseCapacitySnapshot {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.ObjectType == objectType && item.SchemaName == schemaName && item.Table == tableName {
			return item
		}
	}
	return nil
}
