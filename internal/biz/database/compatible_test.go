package database

import (
	"strings"
	"testing"
)

func TestTrimVersionText(t *testing.T) {
	raw := "Release Version: v7.5.0\nEdition: Community\nGit Commit Hash: abc123\n"
	got := trimVersionText(raw)
	if strings.Contains(got, "\n") {
		t.Fatalf("trimVersionText should flatten newlines, got %q", got)
	}
	if !strings.Contains(got, "Release Version") || !strings.Contains(got, "Git Commit Hash") {
		t.Fatalf("trimVersionText lost content: %q", got)
	}
}

func TestJoinVersionParts(t *testing.T) {
	got := joinVersionParts("OceanBase MySQL", "5.7.25", "OceanBase 4.3.0")
	for _, expected := range []string{"OceanBase MySQL", "5.7.25", "OceanBase 4.3.0"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("joinVersionParts missing %q in %q", expected, got)
		}
	}
}

func TestJoinVersionPartsDeduplicates(t *testing.T) {
	got := joinVersionParts("TiDB", "TiDB")
	if got != "TiDB" {
		t.Fatalf("joinVersionParts duplicate result = %q", got)
	}
}
