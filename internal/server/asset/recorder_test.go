package asset

import (
	"os"
	"testing"
)

func TestAsciinemaRecorderUsesUniqueFilenames(t *testing.T) {
	dir := t.TempDir()
	paths := make(map[string]bool)

	for i := 0; i < 5; i++ {
		recorder, err := NewAsciinemaRecorder(dir, 120, 30)
		if err != nil {
			t.Fatalf("NewAsciinemaRecorder() error = %v", err)
		}
		path := recorder.GetRecordingPath()
		if paths[path] {
			t.Fatalf("recording path reused: %s", path)
		}
		paths[path] = true
		if err := recorder.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recording file was not created: %v", err)
		}
	}
}
