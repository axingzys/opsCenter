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
