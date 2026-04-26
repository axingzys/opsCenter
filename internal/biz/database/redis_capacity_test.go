package database

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestNormalizeRedisDatasetMemory(t *testing.T) {
	tests := []struct {
		name          string
		usedMemory    int64
		datasetMemory int64
		expected      int64
	}{
		{name: "empty", usedMemory: 0, datasetMemory: 0, expected: 0},
		{name: "fallback to used", usedMemory: 1024, datasetMemory: 0, expected: 1024},
		{name: "clamp to used", usedMemory: 1024, datasetMemory: 2048, expected: 1024},
		{name: "keep dataset", usedMemory: 2048, datasetMemory: 1536, expected: 1536},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRedisDatasetMemory(tt.usedMemory, tt.datasetMemory); got != tt.expected {
				t.Fatalf("normalizeRedisDatasetMemory() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestBuildRedisCapacitySnapshots(t *testing.T) {
	collectedAt := time.Date(2026, 4, 25, 20, 30, 0, 0, time.Local)
	snapshots := buildRedisCapacitySnapshots(&DatabaseInstance{Model: gorm.Model{ID: 1}}, &redisCapacityData{
		usedMemory:        2048,
		usedMemoryDataset: 1536,
		keyspaces: map[int]*redisKeyspaceAggregate{
			0: {dbIndex: 0, keyCount: 2},
			1: {dbIndex: 1, keyCount: 1},
		},
		keys: []*DatabaseRedisKeySample{
			{KeyspaceName: "db0", KeyName: "user:1", MemoryUsageBytes: 512, ValueSize: 3},
			{KeyspaceName: "db1", KeyName: "config", MemoryUsageBytes: 128, ValueSize: 1},
		},
	}, collectedAt)

	instanceSnapshot := findInstanceCapacitySnapshot(snapshots)
	if instanceSnapshot.TotalSizeBytes != 2048 {
		t.Fatalf("instance total size = %d, want 2048", instanceSnapshot.TotalSizeBytes)
	}
	if instanceSnapshot.DataSizeBytes != 1536 {
		t.Fatalf("instance data size = %d, want 1536", instanceSnapshot.DataSizeBytes)
	}
	if instanceSnapshot.IndexSizeBytes != 512 {
		t.Fatalf("instance overhead size = %d, want 512", instanceSnapshot.IndexSizeBytes)
	}
	if instanceSnapshot.SchemaCount != 2 {
		t.Fatalf("instance schema count = %d, want 2", instanceSnapshot.SchemaCount)
	}
	if instanceSnapshot.TableCount != 3 {
		t.Fatalf("instance key count = %d, want 3", instanceSnapshot.TableCount)
	}
	if len(snapshots) != 5 {
		t.Fatalf("snapshot count = %d, want 5", len(snapshots))
	}
}
