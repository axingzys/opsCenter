package pgwal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	DefaultSegmentSize int64 = 16 * 1024 * 1024
)

var (
	segmentPattern = regexp.MustCompile(`^[0-9A-Fa-f]{24}$`)
	historyPattern = regexp.MustCompile(`^[0-9A-Fa-f]{8}\.history$`)
)

type Segment struct {
	Name       string
	TimelineID string
	Log        uint64
	Segment    uint64
	SegmentNo  uint64
}

func IsSegmentName(value string) bool {
	return segmentPattern.MatchString(strings.TrimSpace(value))
}

func IsTimelineHistoryFile(value string) bool {
	return historyPattern.MatchString(strings.TrimSpace(value))
}

func TimelineFromHistoryFile(value string) string {
	value = strings.TrimSpace(value)
	if !IsTimelineHistoryFile(value) {
		return ""
	}
	return strings.ToUpper(strings.TrimSuffix(value, ".history"))
}

func ParseSegmentName(value string, segmentSize int64) (Segment, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !IsSegmentName(value) {
		return Segment{}, fmt.Errorf("invalid PostgreSQL WAL segment name: %s", value)
	}
	if segmentSize <= 0 {
		segmentSize = DefaultSegmentSize
	}
	timelineText := value[:8]
	logText := value[8:16]
	segmentText := value[16:24]
	logID, err := strconv.ParseUint(logText, 16, 64)
	if err != nil {
		return Segment{}, err
	}
	segmentID, err := strconv.ParseUint(segmentText, 16, 64)
	if err != nil {
		return Segment{}, err
	}
	segmentsPerLog := uint64(0x100000000 / segmentSize)
	if segmentsPerLog == 0 {
		segmentsPerLog = 256
	}
	return Segment{
		Name:       value,
		TimelineID: timelineText,
		Log:        logID,
		Segment:    segmentID,
		SegmentNo:  logID*segmentsPerLog + segmentID,
	}, nil
}

func SegmentRange(start, end string, segmentSize int64, maxSegments int) ([]Segment, error) {
	startSegment, err := ParseSegmentName(start, segmentSize)
	if err != nil {
		return nil, err
	}
	endSegment, err := ParseSegmentName(end, segmentSize)
	if err != nil {
		return nil, err
	}
	if startSegment.TimelineID != endSegment.TimelineID {
		return nil, fmt.Errorf("WAL segment timeline mismatch: %s -> %s", startSegment.TimelineID, endSegment.TimelineID)
	}
	if endSegment.SegmentNo < startSegment.SegmentNo {
		return nil, fmt.Errorf("WAL segment range is reversed: %s -> %s", start, end)
	}
	if maxSegments <= 0 {
		maxSegments = 10000
	}
	count := int(endSegment.SegmentNo-startSegment.SegmentNo) + 1
	if count > maxSegments {
		return nil, fmt.Errorf("WAL segment range too large: %d > %d", count, maxSegments)
	}
	result := make([]Segment, 0, count)
	current := startSegment
	for {
		result = append(result, current)
		if current.SegmentNo == endSegment.SegmentNo {
			break
		}
		current = nextSegment(current, segmentSize)
	}
	return result, nil
}

func nextSegment(current Segment, segmentSize int64) Segment {
	if segmentSize <= 0 {
		segmentSize = DefaultSegmentSize
	}
	segmentsPerLog := uint64(0x100000000 / segmentSize)
	if segmentsPerLog == 0 {
		segmentsPerLog = 256
	}
	next := current
	next.Segment++
	if next.Segment >= segmentsPerLog {
		next.Segment = 0
		next.Log++
	}
	next.SegmentNo++
	next.Name = fmt.Sprintf("%s%08X%08X", next.TimelineID, next.Log, next.Segment)
	return next
}
