package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRunnerAgentAuthMatchesHashOnly(t *testing.T) {
	sum := sha256.Sum256([]byte("runner-auth-value"))
	config := `{"runnerAuthSha256":"` + hex.EncodeToString(sum[:]) + `"}`
	if !runnerAgentAuthMatches(config, "runner-auth-value") {
		t.Fatalf("expected runner auth to match sha256 config")
	}
	if runnerAgentAuthMatches(config, "wrong") {
		t.Fatalf("unexpected runner auth match")
	}
	if runnerAgentAuthMatches(`{"runnerAuthSha256":""}`, "runner-auth-value") {
		t.Fatalf("empty hash should not match")
	}
	if strings.Contains(strings.ToLower(config), "token") || strings.Contains(strings.ToLower(config), "secret") {
		t.Fatalf("test config should use non-sensitive key naming: %s", config)
	}
}

func TestBuildRunnerAgentConfigJSONUsesLocalSecretPlaceholders(t *testing.T) {
	host := &DatabaseRunnerHost{
		Host:             "192.168.1.30",
		Port:             22,
		WorkDir:          "/var/lib/opshub/database-runner",
		StorageMountPath: "/backup/opshub",
	}
	req := &DatabaseRunnerAgentLifecycleRequest{
		ServerURL:               "http://192.168.1.12:9876/",
		ServiceName:             "opshub-agent-runner-12",
		ListenAddr:              "0.0.0.0:19100",
		IntervalSeconds:         60,
		DatabaseArchiverEnabled: true,
	}
	config := buildRunnerAgentConfigJSON(host, req, "plain-runner-auth")
	var payload map[string]any
	if err := json.Unmarshal([]byte(config), &payload); err != nil {
		t.Fatalf("config is not json: %v\n%s", err, config)
	}
	archiver, _ := payload["databaseArchiver"].(map[string]any)
	if archiver["runnerId"] != "ssh:192.168.1.30:22" {
		t.Fatalf("unexpected runnerId: %#v", archiver["runnerId"])
	}
	if archiver["baseUrl"] != "http://192.168.1.12:9876" || archiver["runnerAuth"] != "plain-runner-auth" || archiver["storageRoot"] != "/backup/opshub" {
		t.Fatalf("unexpected archiver config: %#v", archiver)
	}
	storage, _ := archiver["storage"].(map[string]any)
	if storage["secretKey"] != "<local_secret>" {
		t.Fatalf("object storage secret should stay local placeholder: %#v", storage["secretKey"])
	}
	credentials, _ := archiver["credentials"].([]any)
	firstCredential, _ := credentials[0].(map[string]any)
	if firstCredential["password"] != "<local_secret>" {
		t.Fatalf("database password should stay local placeholder: %#v", firstCredential["password"])
	}
}

func TestApplyRunnerAgentHeartbeat(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	host := &DatabaseRunnerHost{Status: DatabaseRunnerHostStatusPending, LastError: "old"}
	applyRunnerAgentHeartbeat(host, &DatabaseRunnerAgentHeartbeatRequest{Status: DatabaseRunnerHostStatusOnline}, now)
	if host.Status != DatabaseRunnerHostStatusOnline || host.LastHeartbeatAt == nil || !host.LastHeartbeatAt.Equal(now) || host.LastError != "" {
		t.Fatalf("unexpected online heartbeat: %#v", host)
	}

	applyRunnerAgentHeartbeat(host, &DatabaseRunnerAgentHeartbeatRequest{Status: DatabaseRunnerHostStatusFailed, Message: "disk full"}, now.Add(time.Minute))
	if host.Status != DatabaseRunnerHostStatusFailed || host.LastError != "disk full" {
		t.Fatalf("unexpected failed heartbeat: %#v", host)
	}
}

func TestApplyRunnerAgentCheckpointRunningAndLagDegraded(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	lastEvent := now.Add(-2 * time.Minute)
	stream := &DatabaseLogArchiveStream{
		RPOTargetSeconds: 30,
		LeaseOwner:       "agent:runner-a",
		DesiredState:     DatabaseLogArchiveDesiredStateRunning,
	}
	applyRunnerAgentCheckpoint(stream, &DatabaseRunnerAgentCheckpointRequest{
		DaemonStatus:      DatabaseLogArchiveDaemonStatusRunning,
		CursorFile:        "binlog.000010",
		CursorPos:         1200,
		LastSourceFile:    "binlog.000011",
		LastSourcePos:     88,
		ArchiveLagSeconds: 90,
	}, &lastEvent, now, "agent:runner-a")

	if stream.CursorFile != "binlog.000010" || stream.CursorPos != 1200 {
		t.Fatalf("cursor not applied: %#v", stream)
	}
	if stream.Status != DatabaseLogArchiveStreamStatusDegraded || stream.DaemonStatus != DatabaseLogArchiveDaemonStatusDegraded {
		t.Fatalf("lag should degrade stream, status=%s daemon=%s", stream.Status, stream.DaemonStatus)
	}
	if stream.LeaseExpiresAt == nil || !stream.LeaseExpiresAt.After(now) {
		t.Fatalf("lease was not renewed")
	}
}

func TestApplyRunnerAgentCheckpointReleaseLease(t *testing.T) {
	now := time.Date(2026, 4, 29, 2, 0, 0, 0, time.Local)
	leaseExpiresAt := now.Add(time.Minute)
	stream := &DatabaseLogArchiveStream{
		Enabled:        true,
		Status:         DatabaseLogArchiveStreamStatusRunning,
		DesiredState:   DatabaseLogArchiveDesiredStateRunning,
		DaemonStatus:   DatabaseLogArchiveDaemonStatusRunning,
		LeaseOwner:     "agent:runner-a",
		LeaseExpiresAt: &leaseExpiresAt,
	}
	applyRunnerAgentCheckpoint(stream, &DatabaseRunnerAgentCheckpointRequest{ReleaseLease: true}, nil, now, "agent:runner-a")
	if stream.LeaseOwner != "" || stream.LeaseExpiresAt != nil {
		t.Fatalf("lease not released: %#v", stream)
	}
	if stream.DaemonStatus != DatabaseLogArchiveDaemonStatusStopped || stream.Status != DatabaseLogArchiveStreamStatusPending {
		t.Fatalf("unexpected stopped state: %s/%s", stream.DaemonStatus, stream.Status)
	}
}

func TestNormalizeLogArchiveEvent(t *testing.T) {
	if got := normalizeLogArchiveEventLevel("ERROR"); got != DatabaseLogArchiveEventLevelError {
		t.Fatalf("unexpected level: %s", got)
	}
	if got := normalizeLogArchiveEventLevel("unknown"); got != DatabaseLogArchiveEventLevelInfo {
		t.Fatalf("unexpected default level: %s", got)
	}
	if got := normalizeLogArchiveEventType("spool_updated"); got != DatabaseLogArchiveEventSpoolUpdated {
		t.Fatalf("unexpected event type: %s", got)
	}
	if got := normalizeLogArchiveEventType("custom"); got != DatabaseLogArchiveEventAgentMessage {
		t.Fatalf("unexpected default event type: %s", got)
	}
	if LogArchiveEventLevelText(DatabaseLogArchiveEventLevelWarning) != "警告" {
		t.Fatalf("unexpected level text")
	}
	if LogArchiveEventTypeText(DatabaseLogArchiveEventArchiveFailed) != "归档失败" {
		t.Fatalf("unexpected event type text")
	}
}

func TestArchiveEventPayloadIsBoundedJSON(t *testing.T) {
	payload := archiveEventPayload(map[string]any{"storageUri": "runner://runner-host-1/binlog.000001", "fileSize": 42})
	if !strings.Contains(payload, "runner://runner-host-1/binlog.000001") {
		t.Fatalf("payload missing storage uri: %s", payload)
	}
	if len(archiveEventPayload(strings.Repeat("x", 5000))) > 4000 {
		t.Fatalf("payload should be bounded")
	}
}

func TestRecordLogArchiveEventBuildsHourlyRollupAndPrunesHighFrequency(t *testing.T) {
	occurredAt := time.Date(2026, 4, 29, 12, 34, 56, 0, time.Local)
	eventRepo := &testLogArchiveEventRepo{}
	streamRepo := &testLogArchiveStreamRepo{stream: &DatabaseLogArchiveStream{RetentionDays: 30}}
	uc := &UseCase{
		logArchiveEventRepo:  eventRepo,
		logArchiveStreamRepo: streamRepo,
	}

	err := uc.recordLogArchiveEvent(context.Background(), &DatabaseLogArchiveEvent{
		StreamID:          7,
		InstanceID:        3,
		SourceInstanceID:  2,
		RunnerHostID:      9,
		RunnerID:          "runner-a",
		EventType:         DatabaseLogArchiveEventCheckpoint,
		Level:             DatabaseLogArchiveEventLevelWarning,
		Message:           strings.Repeat("m", 1200),
		CursorFile:        "binlog.000010",
		CursorPos:         2048,
		ActiveFile:        "binlog.000011",
		ArchiveLagSeconds: 91,
		PayloadJSON:       `{"pid":123}`,
		OccurredAt:        &occurredAt,
	})
	if err != nil {
		t.Fatalf("recordLogArchiveEvent() error = %v", err)
	}
	if len(eventRepo.created) != 1 {
		t.Fatalf("expected one raw event, got %d", len(eventRepo.created))
	}
	if len(eventRepo.rollups) != 1 {
		t.Fatalf("expected one rollup, got %d", len(eventRepo.rollups))
	}
	rollup := eventRepo.rollups[0]
	if rollup.StreamID != 7 || rollup.EventType != DatabaseLogArchiveEventCheckpoint {
		t.Fatalf("unexpected rollup identity: %#v", rollup)
	}
	if !rollup.BucketStart.Equal(occurredAt.Truncate(time.Hour)) || !rollup.BucketEnd.Equal(occurredAt.Truncate(time.Hour).Add(time.Hour)) {
		t.Fatalf("unexpected rollup bucket: start=%s end=%s", rollup.BucketStart, rollup.BucketEnd)
	}
	if rollup.EventCount != 1 || rollup.WarningCount != 1 || rollup.ErrorCount != 0 || rollup.Level != DatabaseLogArchiveEventLevelWarning {
		t.Fatalf("unexpected rollup counters: %#v", rollup)
	}
	if rollup.LastCursorFile != "binlog.000010" || rollup.LastCursorPos != 2048 || rollup.LastActiveFile != "binlog.000011" {
		t.Fatalf("unexpected rollup cursor fields: %#v", rollup)
	}
	if len(eventRepo.highFrequencyDeletes) != 1 || len(eventRepo.deletes) != 1 {
		t.Fatalf("expected high frequency and normal prune calls, high=%d normal=%d", len(eventRepo.highFrequencyDeletes), len(eventRepo.deletes))
	}
	if eventRepo.highFrequencyDeletes[0].streamID != 7 {
		t.Fatalf("unexpected high-frequency prune stream: %#v", eventRepo.highFrequencyDeletes[0])
	}
	if got := eventRepo.highFrequencyDeletes[0].eventTypes; len(got) != 3 || got[0] != DatabaseLogArchiveEventCheckpoint {
		t.Fatalf("unexpected high-frequency event types: %#v", got)
	}
	if len(eventRepo.created[0].Message) > 1000 {
		t.Fatalf("event message should be trimmed")
	}
}

type testLogArchiveEventRepo struct {
	created []*DatabaseLogArchiveEvent
	rollups []*DatabaseLogArchiveEventRollup
	deletes []struct {
		before   time.Time
		streamID uint
	}
	highFrequencyDeletes []struct {
		before     time.Time
		streamID   uint
		eventTypes []string
	}
}

func (r *testLogArchiveEventRepo) Create(_ context.Context, item *DatabaseLogArchiveEvent) error {
	cloned := *item
	r.created = append(r.created, &cloned)
	return nil
}

func (r *testLogArchiveEventRepo) List(context.Context, *DatabaseLogArchiveEventListRequest) ([]*DatabaseLogArchiveEvent, int64, error) {
	return nil, 0, nil
}

func (r *testLogArchiveEventRepo) UpsertRollup(_ context.Context, item *DatabaseLogArchiveEventRollup) error {
	cloned := *item
	r.rollups = append(r.rollups, &cloned)
	return nil
}

func (r *testLogArchiveEventRepo) DeleteBefore(_ context.Context, before time.Time, streamID uint) (int64, error) {
	r.deletes = append(r.deletes, struct {
		before   time.Time
		streamID uint
	}{before: before, streamID: streamID})
	return 0, nil
}

func (r *testLogArchiveEventRepo) DeleteHighFrequencyBefore(_ context.Context, before time.Time, streamID uint, eventTypes []string) (int64, error) {
	r.highFrequencyDeletes = append(r.highFrequencyDeletes, struct {
		before     time.Time
		streamID   uint
		eventTypes []string
	}{before: before, streamID: streamID, eventTypes: append([]string{}, eventTypes...)})
	return 0, nil
}

type testLogArchiveStreamRepo struct {
	stream *DatabaseLogArchiveStream
}

func (r *testLogArchiveStreamRepo) Create(context.Context, *DatabaseLogArchiveStream) error {
	return nil
}
func (r *testLogArchiveStreamRepo) Update(context.Context, *DatabaseLogArchiveStream) error {
	return nil
}
func (r *testLogArchiveStreamRepo) GetByID(context.Context, uint) (*DatabaseLogArchiveStream, error) {
	return r.stream, nil
}
func (r *testLogArchiveStreamRepo) List(context.Context, *DatabaseLogArchiveStreamListRequest) ([]*DatabaseLogArchiveStream, int64, error) {
	return nil, 0, nil
}
func (r *testLogArchiveStreamRepo) ListRunnableForRunner(context.Context, uint) ([]*DatabaseLogArchiveStream, error) {
	return nil, nil
}
func (r *testLogArchiveStreamRepo) TryAcquireLease(context.Context, uint, uint, string, time.Time, time.Time) (bool, error) {
	return false, nil
}
