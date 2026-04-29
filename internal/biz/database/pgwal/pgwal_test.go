package pgwal

import "testing"

func TestParseSegmentName(t *testing.T) {
	segment, err := ParseSegmentName("000000030000000A00000010", DefaultSegmentSize)
	if err != nil {
		t.Fatalf("ParseSegmentName() error = %v", err)
	}
	if segment.TimelineID != "00000003" || segment.Log != 10 || segment.Segment != 16 {
		t.Fatalf("unexpected segment: %+v", segment)
	}
	if segment.SegmentNo != 2576 {
		t.Fatalf("unexpected segment no: %d", segment.SegmentNo)
	}
}

func TestSegmentRange(t *testing.T) {
	segments, err := SegmentRange("0000000100000000000000FE", "000000010000000100000001", DefaultSegmentSize, 10)
	if err != nil {
		t.Fatalf("SegmentRange() error = %v", err)
	}
	expected := []string{
		"0000000100000000000000FE",
		"0000000100000000000000FF",
		"000000010000000100000000",
		"000000010000000100000001",
	}
	if len(segments) != len(expected) {
		t.Fatalf("unexpected segments: %+v", segments)
	}
	for i, name := range expected {
		if segments[i].Name != name {
			t.Fatalf("unexpected segment at %d: %s", i, segments[i].Name)
		}
	}
}

func TestTimelineHistoryFile(t *testing.T) {
	if !IsTimelineHistoryFile("00000003.history") {
		t.Fatalf("expected timeline history file")
	}
	if TimelineFromHistoryFile("00000003.history") != "00000003" {
		t.Fatalf("unexpected timeline")
	}
}

func TestNormalizeTimelineID(t *testing.T) {
	cases := map[string]string{
		"3":        "00000003",
		"10":       "0000000A",
		"A":        "0000000A",
		"0xA":      "0000000A",
		"00000010": "00000010",
	}
	for input, expected := range cases {
		if actual := NormalizeTimelineID(input); actual != expected {
			t.Fatalf("NormalizeTimelineID(%q)=%s, want %s", input, actual, expected)
		}
	}
}

func TestLSNHelpers(t *testing.T) {
	value, err := ParseLSN("A/18000098")
	if err != nil {
		t.Fatalf("ParseLSN() error = %v", err)
	}
	if FormatLSN(value) != "A/18000098" {
		t.Fatalf("unexpected formatted LSN: %s", FormatLSN(value))
	}
	if cmp, err := CompareLSN("A/18000098", "A/18000099"); err != nil || cmp != -1 {
		t.Fatalf("unexpected compare: %d/%v", cmp, err)
	}
	segment := SegmentForLSN("3", value, DefaultSegmentSize)
	if segment.TimelineID != "00000003" || segment.Name != "000000030000000A00000018" {
		t.Fatalf("unexpected segment for LSN: %+v", segment)
	}
	start, end := SegmentLSNRange(segment, DefaultSegmentSize)
	if value < start || value >= end {
		t.Fatalf("LSN not covered by segment range: %X not in [%X,%X)", value, start, end)
	}
}
