package database

import (
	"strings"
	"testing"
	"time"
)

func TestSelectMySQLBinlogForArchive(t *testing.T) {
	logs := []mysqlBinaryLogFile{
		{Name: "binlog.000001", Size: 100},
		{Name: "binlog.000002", Size: 200},
		{Name: "binlog.000003", Size: 300},
	}
	selected, err := selectMySQLBinlogForArchive(logs, "", "binlog.000001")
	if err != nil {
		t.Fatalf("selectMySQLBinlogForArchive error = %v", err)
	}
	if selected.FileName != "binlog.000002" || selected.Previous != "binlog.000001" || selected.Next != "binlog.000003" {
		t.Fatalf("unexpected selection: %#v", selected)
	}
	selected, err = selectMySQLBinlogForArchive(logs, "", "binlog.000003")
	if err != nil {
		t.Fatalf("select latest again error = %v", err)
	}
	if selected.FileName != "binlog.000003" || selected.Previous != "binlog.000002" || selected.Next != "" {
		t.Fatalf("unexpected latest selection: %#v", selected)
	}
	selected, err = selectMySQLBinlogForArchive(logs, "binlog.000001", "")
	if err != nil {
		t.Fatalf("select requested error = %v", err)
	}
	if selected.FileName != "binlog.000001" || selected.Previous != "" || selected.Next != "binlog.000002" {
		t.Fatalf("unexpected requested selection: %#v", selected)
	}
}

func TestSelectMySQLBinlogForArchiveRejectsUnsafeName(t *testing.T) {
	_, err := selectMySQLBinlogForArchive([]mysqlBinaryLogFile{{Name: "binlog.000001"}}, "../binlog.000001", "")
	if err == nil {
		t.Fatalf("expected unsafe filename error")
	}
}

func TestSelectMySQLBinlogsForCatchUp(t *testing.T) {
	logs := []mysqlBinaryLogFile{
		{Name: "binlog.000001", Size: 100},
		{Name: "binlog.000002", Size: 200},
		{Name: "binlog.000003", Size: 300},
		{Name: "binlog.000004", Size: 400},
	}
	selected, err := selectMySQLBinlogsForCatchUp(logs, "binlog.000001", 2, false)
	if err != nil {
		t.Fatalf("select catch-up error = %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected len = %d", len(selected))
	}
	if selected[0].FileName != "binlog.000002" || selected[0].Previous != "binlog.000001" || selected[0].Next != "binlog.000003" {
		t.Fatalf("unexpected first selection: %#v", selected[0])
	}
	if selected[1].FileName != "binlog.000003" || selected[1].Previous != "binlog.000002" || selected[1].Next != "binlog.000004" {
		t.Fatalf("unexpected second selection: %#v", selected[1])
	}

	selected, err = selectMySQLBinlogsForCatchUp(logs, "binlog.000003", 5, true)
	if err != nil {
		t.Fatalf("select current catch-up error = %v", err)
	}
	if len(selected) != 1 || selected[0].FileName != "binlog.000004" {
		t.Fatalf("unexpected current selection: %#v", selected)
	}
}

func TestSelectMySQLBinlogsForCatchUpRejectsPurgedLastArchive(t *testing.T) {
	logs := []mysqlBinaryLogFile{
		{Name: "binlog.000002", Size: 200},
		{Name: "binlog.000003", Size: 300},
	}
	_, err := selectMySQLBinlogsForCatchUp(logs, "binlog.000001", 5, false)
	if err == nil {
		t.Fatalf("expected purged last archive error")
	}
	if !strings.Contains(err.Error(), "人工确认日志链") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectMySQLBinlogsForCatchUpSkipsCurrentByDefault(t *testing.T) {
	_, err := selectMySQLBinlogsForCatchUp([]mysqlBinaryLogFile{{Name: "binlog.000001", Size: 100}}, "", 5, false)
	if err == nil {
		t.Fatalf("expected no rotated binlog error")
	}
	if !strings.Contains(err.Error(), "已轮转") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseMySQLBinlogEventTime(t *testing.T) {
	got := parseMySQLBinlogEventTime("#260429  1:02:03 server id 1  end_log_pos 123 CRC32")
	if got == nil {
		t.Fatalf("expected parsed time")
	}
	if got.Format("2006-01-02 15:04:05") != "2026-04-29 01:02:03" {
		t.Fatalf("time = %s", got.Format("2006-01-02 15:04:05"))
	}
}

func TestParseMySQLBinlogArchiveOutputsMultiple(t *testing.T) {
	started := time.Date(2026, 4, 29, 1, 0, 0, 0, time.Local)
	finished := time.Date(2026, 4, 29, 1, 10, 0, 0, time.Local)
	stdout := strings.Join([]string{
		"noise",
		"OPSHUB_BINLOG_BEGIN=binlog.000001",
		"OPSHUB_BINLOG_FILE=binlog.000001",
		"OPSHUB_STORAGE_PATH=/backup/binlog.000001",
		"OPSHUB_FILE_SIZE=123",
		"OPSHUB_SHA256=aaa",
		"OPSHUB_FIRST_EVENT_LINE=#260429  1:02:03 server id 1 end_log_pos 4",
		"OPSHUB_LAST_EVENT_LINE=#260429  1:03:03 server id 1 end_log_pos 123",
		"OPSHUB_BINLOG_END=binlog.000001",
		"OPSHUB_BINLOG_BEGIN=binlog.000002",
		"OPSHUB_BINLOG_FILE=binlog.000002",
		"OPSHUB_STORAGE_PATH=/backup/binlog.000002",
		"OPSHUB_FILE_SIZE=456",
		"OPSHUB_SHA256=bbb",
		"OPSHUB_FIRST_EVENT_LINE=#260429  1:04:03 server id 1 end_log_pos 4",
		"OPSHUB_LAST_EVENT_LINE=#260429  1:05:03 server id 1 end_log_pos 456",
		"OPSHUB_BINLOG_END=binlog.000002",
	}, "\n")
	outputs := parseMySQLBinlogArchiveOutputs(stdout, started, finished)
	if len(outputs) != 2 {
		t.Fatalf("outputs len = %d: %#v", len(outputs), outputs)
	}
	if outputs[0].FileName != "binlog.000001" || outputs[0].FileSize != 123 || outputs[0].ChecksumSHA256 != "aaa" {
		t.Fatalf("unexpected first output: %#v", outputs[0])
	}
	if outputs[1].FileName != "binlog.000002" || outputs[1].FileSize != 456 || outputs[1].ChecksumSHA256 != "bbb" {
		t.Fatalf("unexpected second output: %#v", outputs[1])
	}
	if outputs[1].LastEventTime != "2026-04-29 01:05:03" {
		t.Fatalf("last event time = %s", outputs[1].LastEventTime)
	}
}

func TestBuildMySQLBinlogArchiveOnceScriptUsesOptionFile(t *testing.T) {
	script, err := buildMySQLBinlogArchiveOnceScript(mysqlBinlogArchiveScriptInput{
		WorkDir:          "/tmp/opshub runner",
		StorageMountPath: "/backup/opshub",
		StreamID:         9,
		DBHost:           "127.0.0.1",
		DBPort:           3306,
		DBUser:           "backup",
		DBPassword:       "secret-value",
		BinlogFile:       "binlog.000123",
	})
	if err != nil {
		t.Fatalf("build script error = %v", err)
	}
	for _, forbidden := range []string{"MYSQL_PWD", "--password", "--password="} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("script contains forbidden token %q: %s", forbidden, script)
		}
	}
	for _, want := range []string{"--defaults-extra-file", "--read-from-remote-server", "--raw", "OPSHUB_SHA256", "OPSHUB_FIRST_EVENT_LINE"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %s", want, script)
		}
	}
}

func TestBuildMySQLBinlogArchiveOnceScriptSupportsMultipleFiles(t *testing.T) {
	script, err := buildMySQLBinlogArchiveOnceScript(mysqlBinlogArchiveScriptInput{
		WorkDir:          "/tmp/opshub-runner",
		StorageMountPath: "/backup/opshub",
		StreamID:         9,
		DBHost:           "127.0.0.1",
		DBPort:           3306,
		DBUser:           "backup",
		DBPassword:       "secret-value",
		BinlogFiles:      []string{"binlog.000001", "binlog.000002"},
	})
	if err != nil {
		t.Fatalf("build script error = %v", err)
	}
	if !strings.Contains(script, "for BINLOG_FILE in 'binlog.000001' 'binlog.000002'; do") {
		t.Fatalf("script missing multi-file loop: %s", script)
	}
	for _, want := range []string{"OPSHUB_BINLOG_BEGIN", "OPSHUB_BINLOG_END", "OPSHUB_SHA256"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %s", want, script)
		}
	}
}

func TestBinlogArchiveRequestJSONDoesNotExposeSecrets(t *testing.T) {
	payload := binlogArchiveRequestJSON(
		&DatabaseLogArchiveStream{Model: gormModelForTest(1), LastArchiveName: "binlog.000001"},
		&DatabaseRunnerHost{Model: gormModelForTest(2), RunnerType: DatabaseRunnerTypeSSH, Host: "192.168.1.15", CredentialID: 3},
		&mysqlBinlogArchiveSelection{FileName: "binlog.000002", Previous: "binlog.000001"},
		QueryOperator{ID: 4, Username: "admin"},
	)
	for _, forbidden := range []string{"password", "secret", "token", "credential"} {
		if strings.Contains(strings.ToLower(payload), forbidden) {
			t.Fatalf("request json exposes %q: %s", forbidden, payload)
		}
	}
	if !strings.Contains(payload, DatabaseRunnerAllowedCommandBinlogArchiveOnce) {
		t.Fatalf("request json missing allowed command: %s", payload)
	}
}

func TestBinlogArchiveCatchUpRequestJSONDoesNotExposeSecrets(t *testing.T) {
	payload := binlogArchiveCatchUpRequestJSON(
		&DatabaseLogArchiveStream{Model: gormModelForTest(1), LastArchiveName: "binlog.000001"},
		&DatabaseRunnerHost{Model: gormModelForTest(2), RunnerType: DatabaseRunnerTypeSSH, Host: "192.168.1.15", CredentialID: 3},
		[]mysqlBinlogArchiveSelection{
			{FileName: "binlog.000002", Previous: "binlog.000001", Next: "binlog.000003"},
			{FileName: "binlog.000003", Previous: "binlog.000002"},
		},
		&DatabaseRunLogArchiveCatchUpRequest{RunnerHostID: 2, MaxFiles: 5, IncludeCurrent: false},
		QueryOperator{ID: 4, Username: "admin"},
	)
	for _, forbidden := range []string{"password", "secret", "token", "credential"} {
		if strings.Contains(strings.ToLower(payload), forbidden) {
			t.Fatalf("request json exposes %q: %s", forbidden, payload)
		}
	}
	for _, want := range []string{DatabaseRunnerAllowedCommandBinlogArchiveCatchUp, "binlog.000002", "binlog.000003"} {
		if !strings.Contains(payload, want) {
			t.Fatalf("request json missing %q: %s", want, payload)
		}
	}
}
