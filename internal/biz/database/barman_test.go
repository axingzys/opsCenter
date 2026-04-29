package database

import (
	"encoding/base64"
	"testing"
)

func TestParseBarmanServerMetadata(t *testing.T) {
	raw := `{
		"pg-main": {
			"retention_policy": "RECOVERY WINDOW OF 14 DAYS",
			"backup_method": "postgres",
			"streaming_archiver": "on",
			"archiver": "off",
			"slot_name": "barman_slot",
			"postgres_version": "16.2",
			"system_identifier": "7392211849478099211",
			"wal_segment_size": 16777216
		}
	}`
	metadata := parseBarmanServerMetadata(raw, "pg-main")
	if metadata.RetentionPolicy != "RECOVERY WINDOW OF 14 DAYS" {
		t.Fatalf("unexpected retention policy: %s", metadata.RetentionPolicy)
	}
	if metadata.BackupMethod != "postgres" || metadata.SlotName != "barman_slot" {
		t.Fatalf("unexpected method/slot: %s/%s", metadata.BackupMethod, metadata.SlotName)
	}
	if !metadata.StreamingArchiverEnabled || !metadata.StreamingArchiverKnown {
		t.Fatalf("expected streaming archiver enabled and known")
	}
	if metadata.ArchiverEnabled || !metadata.ArchiverKnown {
		t.Fatalf("expected archiver disabled but known")
	}
	if metadata.PGSystemIdentifier != "7392211849478099211" || metadata.WALSegmentSize != 16777216 {
		t.Fatalf("unexpected pg metadata: %+v", metadata)
	}
}

func TestParseBarmanBackupCatalog(t *testing.T) {
	raw := `{
		"pg-main": {
			"backup_id": "20260429T010203",
			"status": "DONE",
			"backup_type": "full",
			"begin_time": "2026-04-29 01:02:03",
			"end_time": "2026-04-29 01:12:03",
			"size": "1.5 GB",
			"system_identifier": "7392211849478099211",
			"timeline": "3",
			"begin_wal": "000000030000000A00000010",
			"end_wal": "000000030000000A00000018",
			"begin_lsn": "A/10000028",
			"end_lsn": "A/18000098"
		}
	}`
	record := parseBarmanBackupCatalog("20260429T010203", "pg-main", raw)
	if record.Status != DatabaseBackupStatusSuccess || record.BackupLevel != DatabaseBackupLevelFull {
		t.Fatalf("unexpected status/level: %s/%s", record.Status, record.BackupLevel)
	}
	if record.FileSize != 1610612736 {
		t.Fatalf("unexpected size: %d", record.FileSize)
	}
	if record.StartedAt == nil || record.FinishedAt == nil {
		t.Fatalf("expected begin/end times")
	}
	if record.TimelineID != "3" || record.WALStart == "" || record.WALEnd == "" || record.StartLSN == "" || record.EndLSN == "" {
		t.Fatalf("unexpected WAL metadata: %+v", record)
	}
}

func TestExtractBarmanBackupIDs(t *testing.T) {
	listText := "20260429T010203 - Wed Apr 29\ninvalid/id should-skip\n20260429T020304 - Wed Apr 29\n"
	listJSON := `{"pg-main":[{"backup_id":"20260429T030405"}]}`
	ids := extractBarmanBackupIDs(listJSON, listText)
	expected := []string{"20260429T010203", "20260429T020304", "20260429T030405"}
	if len(ids) != len(expected) {
		t.Fatalf("unexpected ids: %#v", ids)
	}
	for i := range expected {
		if ids[i] != expected[i] {
			t.Fatalf("unexpected id at %d: %s", i, ids[i])
		}
	}
}

func TestParseBarmanShowOutputs(t *testing.T) {
	showJSON := `{"backup_id":"20260429T010203"}`
	showErr := ""
	stdout := "OPSHUB_BARMAN_SHOW_BACKUP=20260429T010203|0|" +
		base64.StdEncoding.EncodeToString([]byte(showJSON)) + "|" +
		base64.StdEncoding.EncodeToString([]byte(showErr)) + "\n"
	outputs := parseBarmanShowOutputs(stdout)
	if len(outputs) != 1 {
		t.Fatalf("unexpected show outputs: %#v", outputs)
	}
	if outputs[0].BackupID != "20260429T010203" || outputs[0].ExitCode != 0 || outputs[0].ShowJSON != showJSON {
		t.Fatalf("unexpected output: %+v", outputs[0])
	}
}
