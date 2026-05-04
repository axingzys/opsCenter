package database

import (
	"encoding/json"
	"testing"
	"time"

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

func TestApplyRedisClusterNodeVersions(t *testing.T) {
	nodes := []*DatabaseTopologyNodeVO{
		{ID: "master-a", Address: "10.0.0.1:6379@16379"},
		{ID: "master-b", Address: "10.0.0.2:6379"},
	}
	applyRedisClusterNodeVersions(nodes, map[string]string{
		"10.0.0.1:6379": "7.2.5",
	}, "7.0.15")
	if nodes[0].Version != "7.2.5" {
		t.Fatalf("node version = %q, want 7.2.5", nodes[0].Version)
	}
	if nodes[1].Version != "7.0.15" {
		t.Fatalf("fallback version = %q, want 7.0.15", nodes[1].Version)
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

func TestSearchEndpointTLSVerificationDefault(t *testing.T) {
	endpoint, insecureSkipVerify, err := searchEndpoint(&DatabaseInstance{
		DBType:     DBTypeElasticsearch,
		Host:       "search.example.com",
		Port:       9200,
		TLSEnabled: true,
	})
	if err != nil {
		t.Fatalf("searchEndpoint() error = %v", err)
	}
	if endpoint != "https://search.example.com:9200" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	if insecureSkipVerify {
		t.Fatalf("expected TLS certificate verification by default")
	}
}

func TestSearchEndpointAllowsExplicitInsecureSkipVerify(t *testing.T) {
	_, insecureSkipVerify, err := searchEndpoint(&DatabaseInstance{
		DBType:           DBTypeOpenSearch,
		Host:             "search.example.com",
		Port:             9200,
		TLSEnabled:       true,
		ConnectionParams: `{"insecureSkipVerify":true}`,
	})
	if err != nil {
		t.Fatalf("searchEndpoint() error = %v", err)
	}
	if !insecureSkipVerify {
		t.Fatalf("expected explicit insecureSkipVerify=true to be honored")
	}
}

func TestBuildReplicationTopologyLinkForMySQLDelayedReplica(t *testing.T) {
	primary := &DatabaseTopologyNodeVO{ID: "instance:1", Name: "mysql-primary"}
	replicaNode := &DatabaseTopologyNodeVO{ID: "instance:2", Name: "mysql-replica"}
	replica := &DatabaseInstanceReplica{
		Engine:                 DBTypeMySQL,
		ReplicaRole:            DatabaseReplicaRoleDelayed,
		ConfiguredDelaySeconds: 3600,
	}
	check := &DatabaseReplicationCheck{
		Engine:                 DBTypeMySQL,
		RoleDetected:           DatabaseReplicationRoleReplica,
		HealthStatus:           DatabaseReplicaHealthWarning,
		ReplicaIORunning:       "Yes",
		ReplicaSQLRunning:      "Yes",
		SecondsBehindSource:    12,
		ConfiguredDelaySeconds: 3600,
		RemainingDelaySeconds:  0,
	}
	link := buildReplicationTopologyLink(primary, replicaNode, replica, check)
	if link.Source != primary.ID || link.Target != replicaNode.ID {
		t.Fatalf("unexpected link endpoints: %#v", link)
	}
	if link.Label != "delayed replication" {
		t.Fatalf("label = %q, want delayed replication", link.Label)
	}
	if link.LagText != "12s" {
		t.Fatalf("lag = %q, want 12s", link.LagText)
	}
	if link.Metrics["io"] != "Yes" || link.Metrics["sql"] != "Yes" {
		t.Fatalf("missing mysql thread metrics: %#v", link.Metrics)
	}
}

func TestBuildReplicationTopologyLinkForMySQLZeroLag(t *testing.T) {
	primary := &DatabaseTopologyNodeVO{ID: "instance:1", Name: "mysql-primary", Role: DatabaseReplicationRolePrimary, State: DatabaseReplicaHealthHealthy}
	replicaNode := &DatabaseTopologyNodeVO{ID: "instance:2", Name: "mysql-replica", Role: DatabaseReplicationRoleReplica, State: DatabaseReplicaHealthHealthy}
	check := &DatabaseReplicationCheck{
		Engine:              DBTypeMySQL,
		RoleDetected:        DatabaseReplicationRoleReplica,
		HealthStatus:        DatabaseReplicaHealthHealthy,
		ReplicaIORunning:    "Yes",
		ReplicaSQLRunning:   "Yes",
		SecondsBehindSource: 0,
	}
	link := buildReplicationTopologyLink(primary, replicaNode, nil, check)
	if link.LagText != "0s" {
		t.Fatalf("lag = %q, want 0s", link.LagText)
	}
	if link.Metrics["seconds_behind_source"] != "0" {
		t.Fatalf("seconds_behind_source metric = %q, want 0", link.Metrics["seconds_behind_source"])
	}
	cards := buildReplicationTopologyCards(&DatabaseInstance{DBType: DBTypeMySQL}, nil, []*DatabaseTopologyNodeVO{primary, replicaNode}, []*DatabaseTopologyLinkVO{link})
	if got := topologyCardValue(cards, "max_lag"); got != "0s" {
		t.Fatalf("max_lag card = %q, want 0s", got)
	}
}

func TestBuildReplicationTopologyUsesRawMySQLZeroLagForLegacyCheck(t *testing.T) {
	check := &DatabaseReplicationCheck{
		Engine:              DBTypeMySQL,
		RoleDetected:        DatabaseReplicationRoleReplica,
		HealthStatus:        DatabaseReplicaHealthHealthy,
		SecondsBehindSource: -1,
		RawStatusJSON: marshalReplicaJSON(map[string]string{
			"Seconds_Behind_Source": "0",
		}),
	}
	if got := replicationTopologyLagText(check); got != "0s" {
		t.Fatalf("lag = %q, want 0s", got)
	}
	link := buildReplicationTopologyLink(&DatabaseTopologyNodeVO{ID: "instance:1"}, &DatabaseTopologyNodeVO{ID: "instance:2"}, nil, check)
	if link.Metrics["seconds_behind_source"] != "0" {
		t.Fatalf("seconds_behind_source metric = %q, want 0", link.Metrics["seconds_behind_source"])
	}
	cards := buildReplicationTopologyCards(&DatabaseInstance{DBType: DBTypeMySQL}, nil, nil, []*DatabaseTopologyLinkVO{link})
	if got := topologyCardValue(cards, "max_lag"); got != "0s" {
		t.Fatalf("max_lag card = %q, want 0s", got)
	}
}

func TestBuildReplicationTopologyCardsKeepsUnknownLagEmpty(t *testing.T) {
	link := &DatabaseTopologyLinkVO{
		Source:  "instance:1",
		Target:  "instance:2",
		State:   DatabaseReplicaHealthHealthy,
		Metrics: map[string]string{},
	}
	cards := buildReplicationTopologyCards(&DatabaseInstance{DBType: DBTypeMySQL}, nil, nil, []*DatabaseTopologyLinkVO{link})
	if got := topologyCardValue(cards, "max_lag"); got != "-" {
		t.Fatalf("max_lag card = %q, want -", got)
	}
}

func TestAppendTopologyFindingsForCheck(t *testing.T) {
	raw, _ := json.Marshal([]string{"SQL apply 线程异常", "来源主库未匹配"})
	check := &DatabaseReplicationCheck{
		HealthStatus:  DatabaseReplicaHealthCritical,
		RiskFlagsJSON: string(raw),
	}
	var findings []*DatabaseTopologyFindingVO
	appendTopologyFindingsForCheck(&findings, "instance:2", check, nil)
	if len(findings) != 2 {
		t.Fatalf("findings len = %d, want 2", len(findings))
	}
	if findings[0].Level != DatabaseReplicaHealthCritical || findings[0].NodeID != "instance:2" {
		t.Fatalf("unexpected finding: %#v", findings[0])
	}
	if findings[1].Suggestion == "" {
		t.Fatalf("expected suggestion for unmatched source")
	}
}

func topologyCardValue(cards []*DatabaseTopologyCardVO, key string) string {
	for _, card := range cards {
		if card != nil && card.Key == key {
			return card.Value
		}
	}
	return ""
}

func TestAppendPostgreSQLPrimaryRuntimeTopology(t *testing.T) {
	now := time.Now()
	raw := marshalReplicaJSON(map[string]any{
		"role": "primary",
		"pg_stat_replication": []map[string]string{
			{
				"application_name": "standby-01",
				"client_addr":      "10.0.0.2",
				"state":            "streaming",
				"sync_state":       "async",
				"replay_lag":       "2s",
			},
		},
	})
	check := &DatabaseReplicationCheck{
		Engine:        DBTypePostgreSQL,
		RoleDetected:  DatabaseReplicationRolePrimary,
		HealthStatus:  DatabaseReplicaHealthHealthy,
		CheckedAt:     &now,
		RawStatusJSON: raw,
	}
	primary := &DatabaseTopologyNodeVO{ID: "instance:1", Name: "pg-primary"}
	nodes := map[string]*DatabaseTopologyNodeVO{primary.ID: primary}
	links := map[string]*DatabaseTopologyLinkVO{}
	uc := &UseCase{}
	uc.appendPostgreSQLPrimaryRuntimeTopology(nodes, links, primary, check)
	if len(nodes) != 2 {
		t.Fatalf("nodes len = %d, want 2", len(nodes))
	}
	if len(links) != 1 {
		t.Fatalf("links len = %d, want 1", len(links))
	}
	var standby *DatabaseTopologyNodeVO
	for _, node := range nodes {
		if node.ID != primary.ID {
			standby = node
		}
	}
	if standby == nil || standby.Name != "standby-01" || standby.LagText != "2s" {
		t.Fatalf("unexpected standby node: %#v", standby)
	}
}
