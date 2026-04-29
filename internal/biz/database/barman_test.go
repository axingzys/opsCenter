package database

import (
	"encoding/base64"
	"testing"
	"time"
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

func TestCollectBarmanWALCatalogRecordsUsesListFilesForGapDetection(t *testing.T) {
	showJSON := `{
		"pg-main": {
			"backup_id": "20260429T010203",
			"status": "DONE",
			"begin_time": "2026-04-29 01:02:03",
			"end_time": "2026-04-29 01:12:03",
			"system_identifier": "7392211849478099211",
			"begin_wal": "000000010000000000000001",
			"end_wal": "000000010000000000000003",
			"begin_lsn": "0/1000000",
			"end_lsn": "0/3000000"
		}
	}`
	filesOutput := "000000010000000000000001\n000000010000000000000003\n"
	stdout := "OPSHUB_BARMAN_SHOW_BACKUP=20260429T010203|0|" +
		base64.StdEncoding.EncodeToString([]byte(showJSON)) + "|\n" +
		"OPSHUB_BARMAN_LIST_FILES=20260429T010203|0|" +
		base64.StdEncoding.EncodeToString([]byte(filesOutput)) + "|\n"
	records := collectBarmanWALCatalogRecords(&DatabaseBarmanServer{
		BarmanServerName:   "pg-main",
		PGSystemIdentifier: "7392211849478099211",
		WALSegmentSize:     16777216,
	}, stdout)
	if len(records) != 2 {
		t.Fatalf("expected list-files records without fallback fill, got %d: %+v", len(records), records)
	}
	status, _ := classifyBarmanWALCatalog(records)
	if status != DatabaseLogChainStatusMissingWAL {
		t.Fatalf("expected missing_wal, got %s", status)
	}
}

func TestCollectBarmanWALCatalogRecordsFallsBackToShowBackupRange(t *testing.T) {
	showJSON := `{
		"pg-main": {
			"backup_id": "20260429T010203",
			"status": "DONE",
			"begin_time": "2026-04-29 01:02:03",
			"end_time": "2026-04-29 01:12:03",
			"begin_wal": "000000010000000000000001",
			"end_wal": "000000010000000000000003"
		}
	}`
	stdout := "OPSHUB_BARMAN_SHOW_BACKUP=20260429T010203|0|" +
		base64.StdEncoding.EncodeToString([]byte(showJSON)) + "|\n" +
		"OPSHUB_BARMAN_LIST_FILES=20260429T010203|1||" +
		base64.StdEncoding.EncodeToString([]byte("list-files unsupported")) + "\n"
	records := collectBarmanWALCatalogRecords(&DatabaseBarmanServer{BarmanServerName: "pg-main", WALSegmentSize: 16777216}, stdout)
	if len(records) != 3 {
		t.Fatalf("expected fallback range records, got %d: %+v", len(records), records)
	}
	status, _ := classifyBarmanWALCatalog(records)
	if status != DatabaseLogChainStatusComplete {
		t.Fatalf("expected complete, got %s", status)
	}
}

func TestClassifyBarmanWALCatalogTimelineHistoryGap(t *testing.T) {
	records := []barmanWALCatalogRecord{{
		FileName:   "000000030000000000000001",
		TimelineID: "00000003",
		SegmentNo:  "1",
	}}
	status, timelineStatus := classifyBarmanWALCatalog(records)
	if status != DatabaseLogChainStatusComplete || timelineStatus != DatabaseLogChainStatusTimelineGap {
		t.Fatalf("unexpected status: %s/%s", status, timelineStatus)
	}
}

func TestParseBarmanBackupRunnerResult(t *testing.T) {
	showJSON := `{"pg-main":{"backup_id":"20260429T010203","status":"DONE"}}`
	stdout := "OPSHUB_BARMAN_BACKUP_EXIT=0\n" +
		"OPSHUB_BARMAN_BACKUP_ID=20260429T010203\n" +
		"OPSHUB_BARMAN_SHOW_BACKUP=20260429T010203|0|" +
		base64.StdEncoding.EncodeToString([]byte(showJSON)) + "|\n"
	result := parseBarmanBackupRunnerResult(
		&DatabaseBarmanServer{BarmanServerName: "pg-main"},
		&DatabaseRunnerHost{Host: "runner", Port: 22},
		stdout,
		"",
		0,
		nil,
		time.Now(),
		time.Now(),
	)
	if result.BackupExitCode != 0 || result.BackupID != "20260429T010203" || result.ShowJSON != showJSON {
		t.Fatalf("unexpected result: %+v", result)
	}
}
