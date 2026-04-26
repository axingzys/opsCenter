package database

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestParseRedisClusterNodes(t *testing.T) {
	raw := `
07c37dfeb2352e0b8f1f976313d3f58f3a8c0 master-1:6379@16379 master - 0 0 1 connected 0-5460
2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a replica-1:6379@16379 slave 07c37dfeb2352e0b8f1f976313d3f58f3a8c0 0 0 2 connected
`
	nodes, links, byAddr := parseRedisClusterNodes(raw)
	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if nodes[0].Role != "master" || nodes[0].Slots != "0-5460" {
		t.Fatalf("unexpected master node: %#v", nodes[0])
	}
	if byAddr["master-1:6379"] == nil {
		t.Fatalf("expected address map to contain master-1:6379")
	}
	if links[0].Target != nodes[0].ID {
		t.Fatalf("replica target = %q, want %q", links[0].Target, nodes[0].ID)
	}
}

func TestApplyRedisClusterSlots(t *testing.T) {
	nodes := []*DatabaseTopologyNodeVO{
		{ID: "master-a", Address: "10.0.0.1:6379"},
	}
	byAddr := map[string]*DatabaseTopologyNodeVO{
		"10.0.0.1:6379": nodes[0],
	}
	applyRedisClusterSlots(nodes, byAddr, []redis.ClusterSlot{
		{Start: 0, End: 5460, Nodes: []redis.ClusterNode{{Addr: "10.0.0.1:6379"}}},
		{Start: 5461, End: 10922, Nodes: []redis.ClusterNode{{Addr: "10.0.0.1:6379"}}},
	})
	if nodes[0].Slots != "0-5460 5461-10922" {
		t.Fatalf("slots = %q", nodes[0].Slots)
	}
}

func TestStringFromMap(t *testing.T) {
	data := map[string]any{
		"version": map[string]any{
			"number": "8.12.0",
		},
	}
	if got := stringFromMap(data, "version.number"); got != "8.12.0" {
		t.Fatalf("stringFromMap = %q, want 8.12.0", got)
	}
	if got := stringFromMap(data, "version.missing"); got != "" {
		t.Fatalf("missing path = %q, want empty", got)
	}
}
