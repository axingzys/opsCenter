package main

import (
	"testing"
	"time"
)

func TestResolveDatabaseArchiverConfigDerivesBaseURL(t *testing.T) {
	cfg := &agentConfig{
		ReportURL: "https://opshub.example.com/api/v1/public/agents/report",
		DatabaseArchiver: databaseArchiverConfig{
			Enabled:    true,
			RunnerID:   "runner-host-7",
			RunnerAuth: "runner-auth",
		},
	}
	resolved, err := resolveDatabaseArchiverConfig(cfg)
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.OpsHubBaseURL != "https://opshub.example.com" {
		t.Fatalf("unexpected base url: %s", resolved.OpsHubBaseURL)
	}
	if resolved.EndpointBaseURL != "https://opshub.example.com/api/v1/public/databases/runner-agents/runner-host-7" {
		t.Fatalf("unexpected endpoint base: %s", resolved.EndpointBaseURL)
	}
	if resolved.Interval != defaultDatabaseArchiverIntervalSeconds*time.Second {
		t.Fatalf("unexpected interval: %s", resolved.Interval)
	}
}

func TestSelectAgentBinlogsForArchiveSkipsCurrentByDefault(t *testing.T) {
	logs := []agentMySQLBinaryLog{
		{Name: "binlog.000001", Size: 10},
		{Name: "binlog.000002", Size: 20},
		{Name: "binlog.000003", Size: 30},
	}
	items, err := selectAgentBinlogsForArchive(logs, "binlog.000001", 10, false)
	if err != nil {
		t.Fatalf("select binlogs: %v", err)
	}
	if len(items) != 1 || items[0].FileName != "binlog.000002" {
		t.Fatalf("unexpected selected items: %#v", items)
	}
	if items[0].Previous != "binlog.000001" || items[0].Next != "binlog.000003" {
		t.Fatalf("unexpected chain metadata: %#v", items[0])
	}
}

func TestSelectAgentBinlogsForArchiveDetectsPurgedCursor(t *testing.T) {
	logs := []agentMySQLBinaryLog{
		{Name: "binlog.000004", Size: 40},
		{Name: "binlog.000005", Size: 50},
	}
	if _, err := selectAgentBinlogsForArchive(logs, "binlog.000003", 10, false); err == nil {
		t.Fatalf("expected purge gap error")
	}
}

func TestSelectAgentBinlogsForArchiveCanIncludeCurrent(t *testing.T) {
	logs := []agentMySQLBinaryLog{
		{Name: "binlog.000001", Size: 10},
		{Name: "binlog.000002", Size: 20},
	}
	items, err := selectAgentBinlogsForArchive(logs, "", 10, true)
	if err != nil {
		t.Fatalf("select binlogs: %v", err)
	}
	if len(items) != 2 || items[1].FileName != "binlog.000002" {
		t.Fatalf("unexpected selected items: %#v", items)
	}
}

func TestParseAgentMySQLBinlogEventTime(t *testing.T) {
	value := parseAgentMySQLBinlogEventTime("#260429  1:23:45 server id 1  end_log_pos 123 CRC32")
	if value == nil {
		t.Fatalf("expected parsed time")
	}
	if value.Year() != 2026 || value.Month() != time.April || value.Day() != 29 || value.Hour() != 1 || value.Minute() != 23 || value.Second() != 45 {
		t.Fatalf("unexpected parsed time: %s", value.Format(time.RFC3339))
	}
}

func TestDatabaseArchiverCredentialMatching(t *testing.T) {
	cfg := &resolvedDatabaseArchiverConfig{
		Credentials: []databaseArchiverCredential{
			{InstanceID: 8, Username: "instance-user", Password: "x"},
			{StreamID: 9, Username: "stream-user", Password: "x"},
		},
	}
	item := databaseArchiverAssignedStream{
		Stream: databaseArchiverStream{ID: 9, InstanceID: 8},
		SourceInstance: databaseArchiverSource{
			ID:   8,
			Host: "127.0.0.1",
			Port: 3306,
		},
	}
	credential, err := cfg.credentialForStream(item)
	if err != nil {
		t.Fatalf("match credential: %v", err)
	}
	if credential.Username != "stream-user" {
		t.Fatalf("stream credential should win, got %s", credential.Username)
	}
}
