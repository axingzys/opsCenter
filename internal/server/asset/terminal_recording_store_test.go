package asset

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ydcloud-dy/opshub/internal/conf"
)

func TestTerminalRecordingStoreValidate(t *testing.T) {
	dir := t.TempDir()
	store := newTerminalRecordingStore(conf.TerminalConfig{RecordingPath: dir})

	validPath := filepath.Join(dir, "valid.cast")
	if err := os.WriteFile(validPath, []byte("{\"version\":2,\"width\":80,\"height\":24}\n[0,\"o\",\"ok\"]\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.Validate(validPath); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalidPath := filepath.Join(dir, "invalid.cast")
	if err := os.WriteFile(invalidPath, []byte("{\"code\":404,\"message\":\"录屏文件不存在\"}\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.Validate(invalidPath); !errors.Is(err, ErrTerminalRecordingInvalid) {
		t.Fatalf("Validate() error = %v, want ErrTerminalRecordingInvalid", err)
	}
}
