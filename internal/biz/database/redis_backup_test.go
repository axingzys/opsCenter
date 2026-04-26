package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeRedisBackupTTL(t *testing.T) {
	if got := normalizeRedisBackupTTL(5 * time.Second); got != 5000 {
		t.Fatalf("normalizeRedisBackupTTL(5s) = %d, want 5000", got)
	}
	if got := normalizeRedisBackupTTL(-1 * time.Millisecond); got != -1 {
		t.Fatalf("normalizeRedisBackupTTL(-1ms) = %d, want -1", got)
	}
	if got := normalizeRedisBackupTTL(-2 * time.Millisecond); got != -2 {
		t.Fatalf("normalizeRedisBackupTTL(-2ms) = %d, want -2", got)
	}
}

func TestValidateRedisLogicalBackupHeader(t *testing.T) {
	valid := &redisLogicalBackupHeader{
		Kind:         redisLogicalBackupKindHeader,
		Format:       redisLogicalBackupFormat,
		Version:      redisLogicalBackupVersion,
		SourceDBType: DBTypeRedis,
	}
	if err := validateRedisLogicalBackupHeader(valid); err != nil {
		t.Fatalf("validateRedisLogicalBackupHeader(valid) error = %v", err)
	}

	invalid := *valid
	invalid.Format = "bad-format"
	if err := validateRedisLogicalBackupHeader(&invalid); err == nil {
		t.Fatalf("expected invalid header to be rejected")
	}
}

func TestWriteAndOpenRedisLogicalBackupFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.redis.json.gz")
	_, err := writeRedisLogicalBackupFile(path, func(enc *json.Encoder) error {
		if err := enc.Encode(&redisLogicalBackupHeader{
			Kind:         redisLogicalBackupKindHeader,
			Format:       redisLogicalBackupFormat,
			Version:      redisLogicalBackupVersion,
			SourceDBType: DBTypeRedis,
			DBIndexes:    []int{0},
		}); err != nil {
			return err
		}
		return enc.Encode(&redisLogicalBackupEntry{
			Kind:       redisLogicalBackupKindKey,
			DBIndex:    0,
			KeyName:    "test:key",
			TTLMillis:  0,
			DumpBase64: "ZHVtcA==",
		})
	})
	if err != nil {
		t.Fatalf("writeRedisLogicalBackupFile() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected backup file exists, stat error = %v", err)
	}
	header, decoder, cleanup, err := openRedisLogicalBackupReader(path)
	if err != nil {
		t.Fatalf("openRedisLogicalBackupReader() error = %v", err)
	}
	defer cleanup()
	if header.Kind != redisLogicalBackupKindHeader {
		t.Fatalf("unexpected header kind: %s", header.Kind)
	}
	var entry redisLogicalBackupEntry
	if err := decoder.Decode(&entry); err != nil {
		t.Fatalf("decode redis backup entry error = %v", err)
	}
	if entry.KeyName != "test:key" {
		t.Fatalf("unexpected redis backup key: %s", entry.KeyName)
	}
}
