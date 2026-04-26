package database

import "testing"

func TestAnalyzeReadOnlyRedisCommandAllowsScanAndFormatsCount(t *testing.T) {
	result := analyzeReadOnlyRedisCommand("scan 0 match user:* count 1000", 50)
	if !result.Allowed {
		t.Fatalf("expected redis scan to be allowed, got denied: %s", result.Message)
	}
	if result.SQLType != "SCAN" {
		t.Fatalf("expected sqlType SCAN, got %s", result.SQLType)
	}
	if result.SQLText != "SCAN 0 MATCH user:* COUNT 50" {
		t.Fatalf("unexpected formatted command: %s", result.SQLText)
	}
}

func TestAnalyzeReadOnlyRedisCommandRejectsWriteCommands(t *testing.T) {
	result := analyzeReadOnlyRedisCommand("set app:config 1", 50)
	if result.Allowed {
		t.Fatalf("expected write command to be rejected")
	}
}

func TestTokenizeRedisCommandSupportsQuotes(t *testing.T) {
	command, err := parseRedisReadCommand(`GET "app config"`, 20)
	if err != nil {
		t.Fatalf("expected quoted command to parse, got error: %v", err)
	}
	if len(command.Args) != 1 || command.Args[0] != "app config" {
		t.Fatalf("unexpected args: %#v", command.Args)
	}
}

func TestParseRedisKeyspaceStats(t *testing.T) {
	stats := parseRedisKeyspaceStats(map[string]string{
		"db0": "keys=12,expires=5,avg_ttl=1000",
		"db1": "keys=3,expires=0,avg_ttl=0",
	})
	if len(stats) != 2 {
		t.Fatalf("expected 2 keyspaces, got %d", len(stats))
	}
	if stats[0].keyCount != 12 || stats[0].expiringKeys != 5 || stats[0].avgTTLMillis != 1000 {
		t.Fatalf("unexpected db0 stats: %#v", stats[0])
	}
	if stats[1].keyCount != 3 {
		t.Fatalf("unexpected db1 stats: %#v", stats[1])
	}
}
