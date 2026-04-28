package database

import (
	"strings"
	"testing"
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

func TestParseMySQLBinlogEventTime(t *testing.T) {
	got := parseMySQLBinlogEventTime("#260429  1:02:03 server id 1  end_log_pos 123 CRC32")
	if got == nil {
		t.Fatalf("expected parsed time")
	}
	if got.Format("2006-01-02 15:04:05") != "2026-04-29 01:02:03" {
		t.Fatalf("time = %s", got.Format("2006-01-02 15:04:05"))
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
