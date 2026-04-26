package database

import "testing"

func TestConnectionParamStringList(t *testing.T) {
	got := connectionParamStringList(map[string]any{
		"sentinelAddrs": []any{"10.0.0.1:26379", "10.0.0.2:26379"},
		"sentinels":     "10.0.0.2:26379,10.0.0.3:26379\n10.0.0.4:26379",
	}, "sentinelAddrs", "sentinels")

	expected := []string{"10.0.0.1:26379", "10.0.0.2:26379", "10.0.0.3:26379", "10.0.0.4:26379"}
	if len(got) != len(expected) {
		t.Fatalf("length = %d, want %d", len(got), len(expected))
	}
	for idx := range expected {
		if got[idx] != expected[idx] {
			t.Fatalf("index %d = %q, want %q", idx, got[idx], expected[idx])
		}
	}
}

func TestParseRedisSentinelSettingsFromParams(t *testing.T) {
	item := &DatabaseInstance{
		Host:             "192.168.1.10",
		Port:             26379,
		ConnectionParams: `{"masterName":"mymaster","sentinelAddrs":["192.168.1.11:26379","192.168.1.10:26379"],"db":1}`,
	}
	params, err := parseConnectionParams(item)
	if err != nil {
		t.Fatalf("parseConnectionParams() error = %v", err)
	}
	settings, err := parseRedisSentinelSettingsFromParams(item, &ConnectionCredential{Username: "default", Password: "secret"}, params)
	if err != nil {
		t.Fatalf("parseRedisSentinelSettingsFromParams() error = %v", err)
	}
	if settings == nil {
		t.Fatalf("expected settings")
	}
	if settings.MasterName != "mymaster" {
		t.Fatalf("MasterName = %q, want mymaster", settings.MasterName)
	}
	if settings.DB != 1 {
		t.Fatalf("DB = %d, want 1", settings.DB)
	}
	if len(settings.SentinelAddrs) != 2 {
		t.Fatalf("SentinelAddrs length = %d, want 2", len(settings.SentinelAddrs))
	}
	if settings.SentinelAddrs[0] != "192.168.1.10:26379" {
		t.Fatalf("first sentinel addr = %q", settings.SentinelAddrs[0])
	}
	if settings.SentinelUsername != "default" || settings.SentinelPassword != "secret" {
		t.Fatalf("unexpected sentinel credential fallback: %#v", settings)
	}
}

func TestParseRedisSentinelSettingsFromParamsMissingMasterName(t *testing.T) {
	item := &DatabaseInstance{
		Host:             "192.168.1.10",
		Port:             26379,
		ConnectionParams: `{"sentinelAddrs":["192.168.1.11:26379"]}`,
	}
	params, err := parseConnectionParams(item)
	if err != nil {
		t.Fatalf("parseConnectionParams() error = %v", err)
	}
	if _, err := parseRedisSentinelSettingsFromParams(item, nil, params); err == nil {
		t.Fatalf("expected missing masterName error")
	}
}
