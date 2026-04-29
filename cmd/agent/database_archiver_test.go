package main

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	if resolved.MaxConcurrentStreams != defaultDatabaseArchiverMaxConcurrent {
		t.Fatalf("unexpected max concurrency: %d", resolved.MaxConcurrentStreams)
	}
	if resolved.FailureBackoff != defaultDatabaseArchiverBackoffSeconds*time.Second || resolved.MaxFailureBackoff != defaultDatabaseArchiverMaxBackoffSec*time.Second {
		t.Fatalf("unexpected backoff: %s/%s", resolved.FailureBackoff, resolved.MaxFailureBackoff)
	}
	if resolved.StopNeverEnabled {
		t.Fatalf("stop-never should require explicit opt-in")
	}
	if resolved.StreamingLogMaxBytes != defaultDatabaseArchiverLogMaxBytes || resolved.StreamingLogMaxFiles != defaultDatabaseArchiverLogMaxFiles {
		t.Fatalf("unexpected streaming log settings: %d/%d", resolved.StreamingLogMaxBytes, resolved.StreamingLogMaxFiles)
	}
}

func TestResolveDatabaseArchiverConfigClampsConcurrencyAndBackoff(t *testing.T) {
	cfg := &agentConfig{
		ReportURL: "https://opshub.example.com/api/v1/public/agents/report",
		DatabaseArchiver: databaseArchiverConfig{
			Enabled:                  true,
			RunnerID:                 "runner-host-7",
			RunnerAuth:               "runner-auth",
			MaxConcurrentStreams:     99,
			FailureBackoffSeconds:    1,
			MaxFailureBackoffSeconds: 2,
			StreamingLogMaxBytes:     1024,
			StreamingLogMaxFiles:     99,
		},
	}
	resolved, err := resolveDatabaseArchiverConfig(cfg)
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if resolved.MaxConcurrentStreams != 16 {
		t.Fatalf("unexpected max concurrency: %d", resolved.MaxConcurrentStreams)
	}
	if resolved.FailureBackoff != 5*time.Second || resolved.MaxFailureBackoff != 5*time.Second {
		t.Fatalf("unexpected backoff clamp: %s/%s", resolved.FailureBackoff, resolved.MaxFailureBackoff)
	}
	if resolved.StreamingLogMaxBytes != 1024*1024 || resolved.StreamingLogMaxFiles != 50 {
		t.Fatalf("unexpected log clamp: %d/%d", resolved.StreamingLogMaxBytes, resolved.StreamingLogMaxFiles)
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
	_, err := selectAgentBinlogsForArchive(logs, "binlog.000003", 10, false)
	if err == nil {
		t.Fatalf("expected purge gap error")
	}
	var gap *agentBinlogPurgeGapError
	if !errors.As(err, &gap) {
		t.Fatalf("expected typed purge gap error, got %T", err)
	}
	if gap.LastArchived != "binlog.000003" || gap.FirstAvailable != "binlog.000004" || gap.LastAvailable != "binlog.000005" {
		t.Fatalf("unexpected purge gap metadata: %#v", gap)
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

func TestDatabaseArchiverEventPayloadJSONIsBounded(t *testing.T) {
	payload := databaseArchiverEventPayloadJSON(map[string]any{"sourceFile": "binlog.000001", "sourcePos": 42})
	if payload == "" || len(payload) > 4000 {
		t.Fatalf("unexpected payload: %q", payload)
	}
	if got := databaseArchiverEventPayloadJSON(string(make([]byte, 5000))); len(got) > 4000 {
		t.Fatalf("payload should be bounded")
	}
}

func TestResolveDatabaseArchiverStorageDefaultsLocal(t *testing.T) {
	storage, err := resolveDatabaseArchiverStorage(databaseArchiverStorage{})
	if err != nil {
		t.Fatalf("resolve storage: %v", err)
	}
	if storage.Type != "local" {
		t.Fatalf("unexpected storage type: %s", storage.Type)
	}
}

func TestResolveDatabaseArchiverStorageNormalizesMinIO(t *testing.T) {
	storage, err := resolveDatabaseArchiverStorage(databaseArchiverStorage{
		Type:       "minio",
		Endpoint:   "192.168.1.30:9000/",
		Bucket:     "opshub",
		AccessKey:  "minioadmin",
		SecretKey:  "minioadmin",
		PathPrefix: "/prod/binlogs/",
	})
	if err != nil {
		t.Fatalf("resolve storage: %v", err)
	}
	if storage.Endpoint != "http://192.168.1.30:9000" {
		t.Fatalf("unexpected endpoint: %s", storage.Endpoint)
	}
	if storage.PathPrefix != "prod/binlogs" || storage.StagingPrefix != "prod/binlogs/.staging" {
		t.Fatalf("unexpected prefixes: %s %s", storage.PathPrefix, storage.StagingPrefix)
	}
	if !storage.UsePathStyle {
		t.Fatalf("minio should use path-style addressing")
	}
}

func TestAgentObjectStorageKeys(t *testing.T) {
	cfg := &resolvedDatabaseArchiverConfig{
		Storage: databaseArchiverStorage{
			Type:          "minio",
			Bucket:        "opshub-backup",
			PathPrefix:    "opshub/database-archives",
			StagingPrefix: "opshub/database-archives/.staging",
		},
	}
	item := databaseArchiverAssignedStream{Stream: databaseArchiverStream{ID: 9, InstanceID: 8}}
	key := agentObjectStorageFinalKey(cfg, item, "binlog.000001")
	if key != "opshub/database-archives/mysql-binlog/instance-8/stream-9/finalized/binlog.000001" {
		t.Fatalf("unexpected final key: %s", key)
	}
	uri := agentObjectStorageURI(cfg, item, "binlog.000001")
	if uri != "minio://opshub-backup/"+key {
		t.Fatalf("unexpected storage uri: %s", uri)
	}
}

func TestDatabaseArchiverFailureBackoff(t *testing.T) {
	app := &agentApp{}
	cfg := &resolvedDatabaseArchiverConfig{
		FailureBackoff:    10 * time.Second,
		MaxFailureBackoff: 25 * time.Second,
	}
	if remaining := app.databaseArchiverBackoffRemaining(3); remaining != 0 {
		t.Fatalf("unexpected initial backoff: %s", remaining)
	}
	if delay := app.databaseArchiverMarkFailure(cfg, 3); delay != 10*time.Second {
		t.Fatalf("unexpected first backoff: %s", delay)
	}
	if remaining := app.databaseArchiverBackoffRemaining(3); remaining <= 0 || remaining > 10*time.Second {
		t.Fatalf("unexpected remaining backoff: %s", remaining)
	}
	if delay := app.databaseArchiverMarkFailure(cfg, 3); delay != 20*time.Second {
		t.Fatalf("unexpected second backoff: %s", delay)
	}
	if delay := app.databaseArchiverMarkFailure(cfg, 3); delay != 25*time.Second {
		t.Fatalf("unexpected capped backoff: %s", delay)
	}
	app.databaseArchiverMarkSuccess(3)
	if remaining := app.databaseArchiverBackoffRemaining(3); remaining != 0 {
		t.Fatalf("backoff should be cleared, got %s", remaining)
	}
}

func TestSpoolAgentActiveBinlogReusesUnchangedPartial(t *testing.T) {
	root := t.TempDir()
	cfg := &resolvedDatabaseArchiverConfig{StorageRoot: root}
	item := databaseArchiverAssignedStream{Stream: databaseArchiverStream{ID: 2, InstanceID: 1}}
	spoolDir := agentBinlogSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatalf("mkdir spool: %v", err)
	}
	target := filepath.Join(spoolDir, "binlog.000001.partial")
	data := testAgentBinlogFile(true, []byte("format"), []byte("rows"))
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	result, err := spoolAgentActiveBinlog(context.Background(), cfg, item, databaseArchiverCredential{}, agentMySQLServerIdentity{ServerUUID: "uuid-1", ServerID: "11"}, "missing-mysqlbinlog", "binlog.000001", int64(len(data)))
	if err != nil {
		t.Fatalf("spool should reuse unchanged partial without invoking tool: %v", err)
	}
	if !result.Reused || result.Size != int64(len(data)) || result.SourceSize != int64(len(data)) || result.LastCompletePos != int64(len(data)) {
		t.Fatalf("unexpected spool result: %#v", result)
	}
	manifest, err := readAgentPartialManifest(target)
	if err != nil {
		t.Fatalf("read partial manifest: %v", err)
	}
	if manifest.FileName != "binlog.000001" || manifest.ServerUUID != "uuid-1" || manifest.ServerID != "11" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestValidateAgentBinlogFileCRCAndTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binlog.000001")
	data := testAgentBinlogFile(true, []byte("format"), []byte("rows"))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write binlog: %v", err)
	}
	result, err := validateAgentBinlogFile(path)
	if err != nil {
		t.Fatalf("validate binlog: %v", err)
	}
	if result.EventCount != 2 || result.ChecksumMode != "crc32" || result.LastCompletePos != int64(len(data)) {
		t.Fatalf("unexpected validation result: %#v", result)
	}
	truncated := filepath.Join(t.TempDir(), "binlog.000001")
	if err := os.WriteFile(truncated, data[:len(data)-2], 0o600); err != nil {
		t.Fatalf("write truncated binlog: %v", err)
	}
	if _, err := validateAgentBinlogFile(truncated); err == nil {
		t.Fatalf("expected truncated binlog to fail validation")
	}
}

func TestValidateAgentBinlogAppendCandidateWithMagicHeader(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "binlog.000001.partial")
	first := testAgentBinlogFile(true, []byte("format"))
	if err := os.WriteFile(existing, first, 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	candidate := filepath.Join(root, "binlog.000001")
	second := append([]byte(agentBinlogMagic), testAgentBinlogEvent(30, uint32(len(first)), []byte("next"), true)...)
	if err := os.WriteFile(candidate, second, 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	plan, err := validateAgentBinlogAppendCandidate(existing, candidate)
	if err != nil {
		t.Fatalf("validate append candidate: %v", err)
	}
	if plan.PayloadOffset != int64(len(agentBinlogMagic)) || plan.FirstStartPos != int64(len(first)) || plan.PayloadBytes <= 0 {
		t.Fatalf("unexpected append plan: %#v", plan)
	}
	badCandidate := filepath.Join(root, "bad-binlog.000001")
	bad := append([]byte(agentBinlogMagic), testAgentBinlogEvent(30, 4, []byte("bad"), true)...)
	if err := os.WriteFile(badCandidate, bad, 0o600); err != nil {
		t.Fatalf("write bad candidate: %v", err)
	}
	if _, err := validateAgentBinlogAppendCandidate(existing, badCandidate); err == nil {
		t.Fatalf("expected non-contiguous candidate to fail")
	}
}

func TestSpoolAgentActiveBinlogResumeAppendsValidatedCandidate(t *testing.T) {
	root := t.TempDir()
	cfg := &resolvedDatabaseArchiverConfig{
		StorageRoot:          root,
		WorkDir:              filepath.Join(root, "work"),
		SpoolResumeEnabled:   true,
		Interval:             30 * time.Second,
		StreamingLogMaxBytes: 1024 * 1024,
		StreamingLogMaxFiles: 2,
	}
	item := databaseArchiverAssignedStream{
		Stream:         databaseArchiverStream{ID: 2, InstanceID: 1},
		SourceInstance: databaseArchiverSource{ID: 1, Host: "127.0.0.1", Port: 3306},
	}
	spoolDir := agentBinlogSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatalf("mkdir spool: %v", err)
	}
	existing := testAgentBinlogFile(true, []byte("format"))
	target := filepath.Join(spoolDir, "binlog.000001.partial")
	if err := os.WriteFile(target, existing, 0o600); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	candidatePath := filepath.Join(root, "candidate.binlog")
	candidate := append([]byte(agentBinlogMagic), testAgentBinlogEvent(30, uint32(len(existing)), []byte("next"), true)...)
	if err := os.WriteFile(candidatePath, candidate, 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	tool := filepath.Join(root, "fake-mysqlbinlog")
	script := "#!/bin/sh\nout=''\nfor arg in \"$@\"; do case \"$arg\" in --result-file=*) out=${arg#--result-file=} ;; esac; done\ncp " + candidatePath + " \"$out/binlog.000001\"\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake mysqlbinlog: %v", err)
	}
	result, err := spoolAgentActiveBinlog(context.Background(), cfg, item, databaseArchiverCredential{Username: "u"}, agentMySQLServerIdentity{}, tool, "binlog.000001", int64(len(existing)+len(candidate)-len(agentBinlogMagic)))
	if err != nil {
		t.Fatalf("resume spool: %v", err)
	}
	if !result.Resumed || result.ResumeFrom != int64(len(existing)) || result.AppendedBytes != int64(len(candidate)-len(agentBinlogMagic)) {
		t.Fatalf("unexpected resume result: %#v", result)
	}
	combined, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read combined: %v", err)
	}
	expected := append(append([]byte{}, existing...), candidate[len(agentBinlogMagic):]...)
	if string(combined) != string(expected) {
		t.Fatalf("combined binlog mismatch")
	}
	if _, err := validateAgentBinlogFile(target); err != nil {
		t.Fatalf("combined validation: %v", err)
	}
	manifest, err := readAgentPartialManifest(target)
	if err != nil {
		t.Fatalf("read partial manifest: %v", err)
	}
	if manifest.LastCompletePos != int64(len(expected)) || manifest.FileName != "binlog.000001" {
		t.Fatalf("unexpected manifest after resume: %#v", manifest)
	}
}

func TestValidateAgentPartialManifestRejectsIdentityDrift(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "binlog.000001.partial")
	data := testAgentBinlogFile(true, []byte("format"))
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	validation, err := validateAgentBinlogFile(target)
	if err != nil {
		t.Fatalf("validate partial: %v", err)
	}
	if err := writeAgentPartialManifest(target, "binlog.000001", int64(len(data)), validation, agentMySQLServerIdentity{ServerUUID: "server-a", ServerID: "7"}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := validateAgentPartialManifestForResume(target, "binlog.000001", agentMySQLServerIdentity{ServerUUID: "server-a", ServerID: "7"}); err != nil {
		t.Fatalf("matching identity should pass: %v", err)
	}
	if err := validateAgentPartialManifestForResume(target, "binlog.000001", agentMySQLServerIdentity{ServerUUID: "server-b", ServerID: "7"}); err == nil {
		t.Fatalf("expected server_uuid drift to fail")
	}
	if err := validateAgentPartialManifestForResume(target, "binlog.000002", agentMySQLServerIdentity{ServerUUID: "server-a", ServerID: "7"}); err == nil {
		t.Fatalf("expected filename drift to fail")
	}
}

func TestArchiveAgentStreamingSpoolFileFinalizesValidRotatedFile(t *testing.T) {
	root := t.TempDir()
	cfg := &resolvedDatabaseArchiverConfig{StorageRoot: root}
	item := databaseArchiverAssignedStream{
		Stream: databaseArchiverStream{ID: 3, InstanceID: 9},
		Runner: databaseArchiverRunnerConfig{ID: 5},
	}
	spoolDir := agentBinlogStreamingSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatalf("mkdir spool: %v", err)
	}
	data := testAgentBinlogFile(true, []byte("format"), []byte("rows"))
	spoolPath := filepath.Join(spoolDir, "binlog.000010")
	if err := os.WriteFile(spoolPath, data, 0o600); err != nil {
		t.Fatalf("write spool: %v", err)
	}
	artifact, err := archiveAgentStreamingSpoolFile(context.Background(), cfg, item, "missing-mysqlbinlog", agentBinlogArchiveSelection{
		FileName: "binlog.000010",
		FileSize: int64(len(data)),
		Previous: "binlog.000009",
		Next:     "binlog.000011",
	})
	if err != nil {
		t.Fatalf("finalize streaming spool: %v", err)
	}
	if artifact.FileName != "binlog.000010" || artifact.FileSize != int64(len(data)) || artifact.ChecksumSHA256 == "" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	if _, err := os.Stat(filepath.Join(agentBinlogFinalDir(cfg, item), "binlog.000010")); err != nil {
		t.Fatalf("finalized file missing: %v", err)
	}
	if _, err := os.Stat(spoolPath); !os.IsNotExist(err) {
		t.Fatalf("spool file should be removed after finalize, err=%v", err)
	}
}

func TestQuarantineAgentStreamingSpoolFileMovesInvalidFile(t *testing.T) {
	root := t.TempDir()
	cfg := &resolvedDatabaseArchiverConfig{StorageRoot: root}
	item := databaseArchiverAssignedStream{Stream: databaseArchiverStream{ID: 4, InstanceID: 10}}
	spoolDir := agentBinlogStreamingSpoolDir(cfg, item)
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		t.Fatalf("mkdir spool: %v", err)
	}
	spoolPath := filepath.Join(spoolDir, "binlog.000011")
	if err := os.WriteFile(spoolPath, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write spool: %v", err)
	}
	quarantinePath, err := quarantineAgentStreamingSpoolFile(cfg, item, "binlog.000011")
	if err != nil {
		t.Fatalf("quarantine spool: %v", err)
	}
	if quarantinePath == "" {
		t.Fatalf("expected quarantine path")
	}
	if _, err := os.Stat(spoolPath); !os.IsNotExist(err) {
		t.Fatalf("source spool should be moved, err=%v", err)
	}
	if _, err := os.Stat(quarantinePath); err != nil {
		t.Fatalf("quarantine file missing: %v", err)
	}
}

func TestAgentUploadRetryQueuePersistsAndValidatesArtifact(t *testing.T) {
	root := t.TempDir()
	retryEnabled := true
	cfg := &resolvedDatabaseArchiverConfig{
		StorageRoot:        root,
		RunnerID:           "runner-a",
		UploadRetryEnabled: retryEnabled,
		UploadRetryBase:    time.Second,
		UploadRetryMax:     10 * time.Second,
		Storage: databaseArchiverStorage{
			Type:       "minio",
			Bucket:     "backup",
			PathPrefix: "archives",
		},
	}
	item := databaseArchiverAssignedStream{
		Stream: databaseArchiverStream{ID: 8, InstanceID: 3, SourceInstanceID: 2},
		Runner: databaseArchiverRunnerConfig{ID: 9},
	}
	filePath := filepath.Join(root, "binlog.000001")
	data := testAgentBinlogFile(true, []byte("format"))
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	checksum, err := sha256File(filePath)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	artifact := agentBinlogArtifact{
		FileName:       "binlog.000001",
		Path:           filePath,
		StorageURI:     "s3://backup/archives/binlog.000001",
		FileSize:       int64(len(data)),
		ChecksumSHA256: checksum,
		FirstEventTime: time.Now(),
		LastEventTime:  time.Now(),
	}
	if err := enqueueAgentUploadRetry(cfg, item, artifact, errors.New("temporary unavailable")); err != nil {
		t.Fatalf("enqueue upload retry: %v", err)
	}
	entries, err := os.ReadDir(agentUploadQueueDir(cfg))
	if err != nil {
		t.Fatalf("read queue dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one queue item, got %d", len(entries))
	}
	queueItem, err := readAgentUploadQueueItem(filepath.Join(agentUploadQueueDir(cfg), entries[0].Name()))
	if err != nil {
		t.Fatalf("read queue item: %v", err)
	}
	if queueItem.StreamID != 8 || queueItem.FileName != "binlog.000001" || queueItem.ChecksumSHA256 != checksum || queueItem.NextAttemptAt == "" {
		t.Fatalf("unexpected queue item: %#v", queueItem)
	}
	if err := validateAgentQueuedArtifact(artifact); err != nil {
		t.Fatalf("validate queued artifact: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}
	if err := validateAgentQueuedArtifact(artifact); err == nil {
		t.Fatalf("expected tampered artifact validation to fail")
	}
}

func TestAgentRollingLogWriterRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mysqlbinlog.stderr.log")
	writer, err := newAgentRollingLogWriter(path, 10, 3)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	if _, err := writer.Write([]byte("1234567890abc")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(current) != "abc" {
		t.Fatalf("unexpected current log: %q", current)
	}
	rotated, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("read rotated: %v", err)
	}
	if string(rotated) != "1234567890" {
		t.Fatalf("unexpected rotated log: %q", rotated)
	}
}

func TestDatabaseArchiverStreamingProcessLifecycle(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "fake-mysqlbinlog")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":null}`))
	}))
	defer server.Close()
	app := &agentApp{
		httpClient:                    server.Client(),
		databaseArchiverProcesses:     map[uint]*agentStreamingProcess{},
		databaseArchiverProcessStarts: map[uint]int{},
	}
	cfg := &resolvedDatabaseArchiverConfig{
		EndpointBaseURL:      server.URL,
		RunnerAuth:           "runner-auth",
		WorkDir:              filepath.Join(root, "work"),
		StorageRoot:          filepath.Join(root, "storage"),
		StreamingLogMaxBytes: 1024 * 1024,
		StreamingLogMaxFiles: 2,
	}
	item := databaseArchiverAssignedStream{
		Stream:         databaseArchiverStream{ID: 7, InstanceID: 3},
		SourceInstance: databaseArchiverSource{ID: 3, Host: "127.0.0.1", Port: 3306},
	}
	credential := databaseArchiverCredential{Username: "replica", Password: "secret"}
	status, err := app.ensureDatabaseArchiverStreamingProcess(context.Background(), cfg, item, credential, tool, "binlog.000001")
	if err != nil {
		t.Fatalf("ensure streaming process: %v", err)
	}
	if !status.Started || !status.Running || status.PID <= 0 {
		t.Fatalf("unexpected status after start: %#v", status)
	}
	if _, err := os.Stat(agentBinlogStreamingSpoolDir(cfg, item)); err != nil {
		t.Fatalf("streaming spool dir missing: %v", err)
	}
	second, err := app.ensureDatabaseArchiverStreamingProcess(context.Background(), cfg, item, credential, tool, "binlog.000001")
	if err != nil {
		t.Fatalf("ensure existing process: %v", err)
	}
	if !second.Running || second.Started || second.PID != status.PID {
		t.Fatalf("expected existing process to be reused, got %#v", second)
	}
	app.stopDatabaseArchiverStreamingProcess(context.Background(), cfg, item.Stream.ID, "test stop")
	if len(app.databaseArchiverStreamingProcessIDs()) != 0 {
		t.Fatalf("streaming process should be detached")
	}
}

func TestAgentObjectStorageMinIOIntegration(t *testing.T) {
	endpoint := os.Getenv("OPSHUB_TEST_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("set OPSHUB_TEST_MINIO_ENDPOINT to run MinIO integration test")
	}
	accessKey := firstNonEmptyString(os.Getenv("OPSHUB_TEST_MINIO_ACCESS_KEY"), "minioadmin")
	secretKey := firstNonEmptyString(os.Getenv("OPSHUB_TEST_MINIO_SECRET_KEY"), "minioadmin")
	bucket := firstNonEmptyString(os.Getenv("OPSHUB_TEST_MINIO_BUCKET"), "opshub-p265-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	storage, err := resolveDatabaseArchiverStorage(databaseArchiverStorage{
		Type:      "minio",
		Endpoint:  endpoint,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
	if err != nil {
		t.Fatalf("resolve storage: %v", err)
	}
	client := newAgentObjectStorageS3Client(storage)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil && !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") && !strings.Contains(err.Error(), "BucketAlreadyExists") {
		t.Fatalf("create bucket: %v", err)
	}
	defer func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String("tests/binlog.000001")})
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(defaultDatabaseArchiverObjectPrefix + "/mysql-binlog/instance-12/stream-34/binlog.000001")})
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(defaultDatabaseArchiverObjectPrefix + "/mysql-binlog/instance-12/stream-34/binlog.000001.sha256")})
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(defaultDatabaseArchiverObjectPrefix + "/mysql-binlog/instance-12/stream-34/binlog.000001.manifest.json")})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	}()
	filePath := filepath.Join(t.TempDir(), "binlog.000001")
	if err := os.WriteFile(filePath, []byte("opshub-binlog-object-storage-test"), 0o600); err != nil {
		t.Fatalf("write temp artifact: %v", err)
	}
	checksum, err := sha256File(filePath)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	key := "tests/binlog.000001"
	if err := uploadAgentObjectFile(ctx, client, bucket, key, filePath, map[string]string{"opshub-sha256": checksum}); err != nil {
		t.Fatalf("upload object: %v", err)
	}
	if err := verifyAgentObject(ctx, client, bucket, key, int64(len("opshub-binlog-object-storage-test")), checksum); err != nil {
		t.Fatalf("verify object: %v", err)
	}
	exists, err := agentObjectExistsAndMatches(ctx, client, bucket, key, int64(len("opshub-binlog-object-storage-test")), checksum)
	if err != nil {
		t.Fatalf("exists and matches: %v", err)
	}
	if !exists {
		t.Fatalf("expected object to exist")
	}
	cfg := &resolvedDatabaseArchiverConfig{Storage: storage}
	item := databaseArchiverAssignedStream{
		Stream: databaseArchiverStream{ID: 34, InstanceID: 12, SourceInstanceID: 12},
	}
	artifact := agentBinlogArtifact{
		FileName:       "binlog.000001",
		Path:           filePath,
		StorageURI:     agentObjectStorageURI(cfg, item, "binlog.000001"),
		FileSize:       int64(len("opshub-binlog-object-storage-test")),
		ChecksumSHA256: checksum,
		FirstEventTime: time.Now(),
		LastEventTime:  time.Now(),
	}
	if err := writeAgentBinlogSidecars(artifact); err != nil {
		t.Fatalf("write sidecars: %v", err)
	}
	if err := publishAgentBinlogArtifact(ctx, cfg, item, artifact); err != nil {
		t.Fatalf("publish binlog artifact: %v", err)
	}
	finalKey := agentObjectStorageFinalKey(cfg, item, artifact.FileName)
	if err := verifyAgentObject(ctx, client, bucket, finalKey, artifact.FileSize, artifact.ChecksumSHA256); err != nil {
		t.Fatalf("verify published final object: %v", err)
	}
	stagingKey := agentObjectStorageStagingKey(cfg, item, artifact.FileName)
	if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(stagingKey)}); err == nil {
		t.Fatalf("staging object should be removed after publish")
	}
}

func testAgentBinlogFile(withChecksum bool, payloads ...[]byte) []byte {
	result := []byte(agentBinlogMagic)
	pos := uint32(len(result))
	for index, payload := range payloads {
		eventType := byte(30)
		if index == 0 {
			eventType = 15
		}
		event := testAgentBinlogEvent(eventType, pos, payload, withChecksum)
		result = append(result, event...)
		pos += uint32(len(event))
	}
	return result
}

func testAgentBinlogEvent(eventType byte, startPos uint32, payload []byte, withChecksum bool) []byte {
	eventSize := agentBinlogEventHeaderLen + len(payload)
	if withChecksum {
		eventSize += 4
	}
	event := make([]byte, agentBinlogEventHeaderLen+len(payload))
	binary.LittleEndian.PutUint32(event[0:4], uint32(time.Now().Unix()))
	event[4] = eventType
	binary.LittleEndian.PutUint32(event[5:9], 1)
	binary.LittleEndian.PutUint32(event[9:13], uint32(eventSize))
	binary.LittleEndian.PutUint32(event[13:17], startPos+uint32(eventSize))
	binary.LittleEndian.PutUint16(event[17:19], 0)
	copy(event[agentBinlogEventHeaderLen:], payload)
	if withChecksum {
		sum := crc32.ChecksumIEEE(event)
		checksum := make([]byte, 4)
		binary.LittleEndian.PutUint32(checksum, sum)
		event = append(event, checksum...)
	}
	return event
}
