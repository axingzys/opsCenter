package main

import (
	"context"
	"errors"
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
}
