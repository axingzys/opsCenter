package database

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveBackupDatabaseName(t *testing.T) {
	tests := []struct {
		name       string
		item       *DatabaseInstance
		credential *ConnectionCredential
		want       string
		wantErr    bool
	}{
		{
			name: "mysql uses default database",
			item: &DatabaseInstance{
				DBType:          DBTypeMySQL,
				DefaultDatabase: "appdb",
			},
			credential: &ConnectionCredential{Username: "root"},
			want:       "appdb",
		},
		{
			name: "postgres falls back to username",
			item: &DatabaseInstance{
				DBType: DBTypePostgreSQL,
			},
			credential: &ConnectionCredential{Username: "postgres"},
			want:       "postgres",
		},
		{
			name: "mysql without default database fails",
			item: &DatabaseInstance{
				DBType: DBTypeMariaDB,
			},
			credential: &ConnectionCredential{Username: "root"},
			wantErr:    true,
		},
		{
			name: "redis uses all dbs label",
			item: &DatabaseInstance{
				DBType: DBTypeRedis,
			},
			credential: &ConnectionCredential{},
			want:       "all-dbs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBackupDatabaseName(tt.item, tt.credential)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveBackupDatabaseName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveBackupDatabaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildBackupCommandSpec(t *testing.T) {
	mysqlSpec, err := buildBackupCommandSpec(&DatabaseInstance{
		DBType: DBTypeMySQL,
		Host:   "127.0.0.1",
		Port:   3306,
	}, &ConnectionCredential{
		Username: "root",
		Password: "secret",
	}, "appdb", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildBackupCommandSpec(mysql) error = %v", err)
	}
	if len(mysqlSpec.Commands) == 0 || mysqlSpec.Commands[0] != "mysqldump" {
		t.Fatalf("unexpected mysql commands: %#v", mysqlSpec.Commands)
	}
	if !containsString(mysqlSpec.Env, "MYSQL_PWD=secret") {
		t.Fatalf("expected MYSQL_PWD env, got %#v", mysqlSpec.Env)
	}
	if !containsString(mysqlSpec.Args, "appdb") {
		t.Fatalf("expected database name in args, got %#v", mysqlSpec.Args)
	}

	pgSpec, err := buildBackupCommandSpec(&DatabaseInstance{
		DBType:     DBTypePostgreSQL,
		Host:       "127.0.0.1",
		Port:       5432,
		TLSEnabled: true,
	}, &ConnectionCredential{
		Username: "postgres",
		Password: "secret",
	}, "postgres", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildBackupCommandSpec(postgresql) error = %v", err)
	}
	if len(pgSpec.Commands) != 1 || pgSpec.Commands[0] != "pg_dump" {
		t.Fatalf("unexpected postgresql commands: %#v", pgSpec.Commands)
	}
	if !containsString(pgSpec.Env, "PGPASSWORD=secret") {
		t.Fatalf("expected PGPASSWORD env, got %#v", pgSpec.Env)
	}
	if !containsString(pgSpec.Env, "PGSSLMODE=require") {
		t.Fatalf("expected PGSSLMODE=require, got %#v", pgSpec.Env)
	}
	if !containsString(pgSpec.Args, "--format=plain") {
		t.Fatalf("expected plain pg_dump format, got %#v", pgSpec.Args)
	}
	if pgSpec.FileExt != ".sql.gz" || pgSpec.OutputMode != backupOutputModeGzip {
		t.Fatalf("unexpected postgresql plain output: ext=%s mode=%s", pgSpec.FileExt, pgSpec.OutputMode)
	}

	pgCustomSpec, err := buildBackupCommandSpec(&DatabaseInstance{
		DBType: DBTypePostgreSQL,
		Host:   "127.0.0.1",
		Port:   5432,
	}, &ConnectionCredential{
		Username: "postgres",
		Password: "secret",
	}, "postgres", DatabaseBackupTypeLogicalCustom)
	if err != nil {
		t.Fatalf("buildBackupCommandSpec(postgresql custom) error = %v", err)
	}
	if !containsString(pgCustomSpec.Args, "--format=custom") {
		t.Fatalf("expected custom pg_dump format, got %#v", pgCustomSpec.Args)
	}
	if pgCustomSpec.FileExt != ".dump" || pgCustomSpec.OutputMode != backupOutputModeRaw {
		t.Fatalf("unexpected postgresql custom output: ext=%s mode=%s", pgCustomSpec.FileExt, pgCustomSpec.OutputMode)
	}

	redisSpec, err := buildBackupCommandSpec(&DatabaseInstance{
		DBType: DBTypeRedis,
		Host:   "127.0.0.1",
		Port:   6379,
	}, &ConnectionCredential{}, "all-dbs", DatabaseBackupTypeLogical)
	if err != nil {
		t.Fatalf("buildBackupCommandSpec(redis) error = %v", err)
	}
	if redisSpec.Runner == nil {
		t.Fatalf("expected redis backup runner")
	}
	if redisSpec.FileExt != redisLogicalBackupFileExt {
		t.Fatalf("unexpected redis backup file ext: %s", redisSpec.FileExt)
	}
}

func TestRunBackupCommandOutputModes(t *testing.T) {
	originalLookPath := backupCommandLookPath
	originalFactory := backupCommandFactory
	defer func() {
		backupCommandLookPath = originalLookPath
		backupCommandFactory = originalFactory
	}()

	backupCommandLookPath = func(command string) (string, error) {
		return command, nil
	}
	backupCommandFactory = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf 'backup-data'")
	}

	tempDir := t.TempDir()
	gzipPath := filepath.Join(tempDir, "backup.sql.gz")
	gzipSize, err := runBackupCommand(context.Background(), &backupCommandSpec{
		Commands:   []string{"fake-dump"},
		OutputMode: backupOutputModeGzip,
	}, gzipPath)
	if err != nil {
		t.Fatalf("runBackupCommand(gzip) error = %v", err)
	}
	if gzipSize <= 0 {
		t.Fatalf("expected gzip file size")
	}
	gzipFile, err := os.Open(gzipPath)
	if err != nil {
		t.Fatalf("open gzip backup: %v", err)
	}
	defer gzipFile.Close()
	gzipReader, err := gzip.NewReader(gzipFile)
	if err != nil {
		t.Fatalf("new gzip reader: %v", err)
	}
	defer gzipReader.Close()
	gzipContent, err := io.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("read gzip backup: %v", err)
	}
	if string(gzipContent) != "backup-data" {
		t.Fatalf("unexpected gzip backup content: %q", string(gzipContent))
	}

	rawPath := filepath.Join(tempDir, "backup.dump")
	rawSize, err := runBackupCommand(context.Background(), &backupCommandSpec{
		Commands:   []string{"fake-dump"},
		OutputMode: backupOutputModeRaw,
	}, rawPath)
	if err != nil {
		t.Fatalf("runBackupCommand(raw) error = %v", err)
	}
	if rawSize != int64(len("backup-data")) {
		t.Fatalf("unexpected raw file size: %d", rawSize)
	}
	rawContent, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw backup: %v", err)
	}
	if string(rawContent) != "backup-data" {
		t.Fatalf("unexpected raw backup content: %q", string(rawContent))
	}
}

func TestBuildBackupOutputPath(t *testing.T) {
	startedAt := time.Date(2026, 4, 24, 12, 30, 45, 0, time.UTC)
	outputPath, fileName, err := buildBackupOutputPath("/tmp/backups", &DatabaseInstance{Name: "生产/MySQL"}, &DatabaseBackupTask{Name: "nightly main"}, "app-db", startedAt, ".sql.gz")
	if err != nil {
		t.Fatalf("buildBackupOutputPath() error = %v", err)
	}
	if !strings.Contains(outputPath, "/tmp/backups") {
		t.Fatalf("expected storage root in path, got %s", outputPath)
	}
	if !strings.Contains(fileName, "nightly_main") || !strings.HasSuffix(fileName, ".sql.gz") {
		t.Fatalf("unexpected file name: %s", fileName)
	}
}

func TestBuildBackupOutputPathRedis(t *testing.T) {
	startedAt := time.Date(2026, 4, 24, 12, 30, 45, 0, time.UTC)
	_, fileName, err := buildBackupOutputPath("/tmp/backups", &DatabaseInstance{Name: "redis-cluster"}, &DatabaseBackupTask{Name: "redis nightly"}, "all-dbs", startedAt, redisLogicalBackupFileExt)
	if err != nil {
		t.Fatalf("buildBackupOutputPath(redis) error = %v", err)
	}
	if !strings.HasSuffix(fileName, redisLogicalBackupFileExt) {
		t.Fatalf("unexpected redis backup file name: %s", fileName)
	}
}

func TestBuildBackupRecordMessage(t *testing.T) {
	success := buildBackupRecordMessage(&DatabaseBackupRecord{
		Status:   DatabaseBackupStatusSuccess,
		FileName: "test.sql.gz",
		FilePath: "/tmp/test.sql.gz",
	})
	if success != "逻辑备份完成，文件已生成" {
		t.Fatalf("unexpected success message: %s", success)
	}

	failed := buildBackupRecordMessage(&DatabaseBackupRecord{
		Status:       DatabaseBackupStatusFailed,
		ErrorMessage: "permission denied",
	})
	if failed != "permission denied" {
		t.Fatalf("unexpected failed message: %s", failed)
	}

	pruned := buildBackupRecordMessage(&DatabaseBackupRecord{
		Status:   DatabaseBackupStatusSuccess,
		FileName: "expired.sql.gz",
		FilePath: "",
	})
	if !strings.Contains(pruned, "已按保留策略清理") {
		t.Fatalf("unexpected pruned message: %s", pruned)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
