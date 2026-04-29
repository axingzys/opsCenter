package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

const (
	agentBinlogMagic          = "\xfe\x62\x69\x6e"
	agentBinlogEventHeaderLen = 19
)

type agentBinlogValidationResult struct {
	FileSize        int64
	EventCount      int
	LastCompletePos int64
	ChecksumMode    string
	ChecksumEvents  int
}

type agentBinlogAppendPlan struct {
	PayloadOffset int64
	PayloadBytes  int64
	FirstStartPos int64
	LastEndPos    int64
	ChecksumMode  string
}

func validateAgentBinlogFile(path string) (agentBinlogValidationResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return agentBinlogValidationResult{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return agentBinlogValidationResult{}, err
	}
	result := agentBinlogValidationResult{FileSize: info.Size(), ChecksumMode: "none_or_unknown"}
	if info.Size() < int64(len(agentBinlogMagic)+agentBinlogEventHeaderLen) {
		return result, fmt.Errorf("binlog 文件过小: %d", info.Size())
	}
	magic := make([]byte, len(agentBinlogMagic))
	if _, err := io.ReadFull(file, magic); err != nil {
		return result, err
	}
	if string(magic) != agentBinlogMagic {
		return result, errors.New("binlog magic header 不匹配")
	}
	offset := int64(len(agentBinlogMagic))
	crcMode := false
	for offset < info.Size() {
		event, err := readAgentBinlogEvent(file, offset, info.Size())
		if err != nil {
			return result, err
		}
		if result.EventCount == 0 {
			matches, err := agentBinlogEventCRC32Matches(file, offset, event.EventSize)
			if err == nil && matches {
				crcMode = true
				result.ChecksumMode = "crc32"
			}
		}
		if crcMode {
			matches, err := agentBinlogEventCRC32Matches(file, offset, event.EventSize)
			if err != nil {
				return result, err
			}
			if !matches {
				return result, fmt.Errorf("binlog event checksum 校验失败: offset=%d", offset)
			}
			result.ChecksumEvents++
		}
		offset += int64(event.EventSize)
		result.EventCount++
		result.LastCompletePos = offset
	}
	if result.EventCount == 0 {
		return result, errors.New("binlog 不包含事件")
	}
	if result.LastCompletePos != info.Size() {
		return result, fmt.Errorf("binlog 末尾不是完整事件边界: complete=%d size=%d", result.LastCompletePos, info.Size())
	}
	return result, nil
}

func validateAgentBinlogAppendCandidate(existingPath, candidatePath string) (agentBinlogAppendPlan, error) {
	existing, err := validateAgentBinlogFile(existingPath)
	if err != nil {
		return agentBinlogAppendPlan{}, fmt.Errorf("existing binlog 校验失败: %w", err)
	}
	candidateFile, err := os.Open(candidatePath)
	if err != nil {
		return agentBinlogAppendPlan{}, err
	}
	defer candidateFile.Close()
	candidateInfo, err := candidateFile.Stat()
	if err != nil {
		return agentBinlogAppendPlan{}, err
	}
	if candidateInfo.Size() < agentBinlogEventHeaderLen {
		return agentBinlogAppendPlan{}, fmt.Errorf("resume candidate 文件过小: %d", candidateInfo.Size())
	}
	payloadOffset := int64(0)
	if candidateInfo.Size() >= int64(len(agentBinlogMagic)+agentBinlogEventHeaderLen) {
		magic := make([]byte, len(agentBinlogMagic))
		if _, err := io.ReadFull(candidateFile, magic); err != nil {
			return agentBinlogAppendPlan{}, err
		}
		if string(magic) == agentBinlogMagic {
			payloadOffset = int64(len(agentBinlogMagic))
		}
	}
	event, err := readAgentBinlogEvent(candidateFile, payloadOffset, candidateInfo.Size())
	if err != nil {
		return agentBinlogAppendPlan{}, err
	}
	firstStart := int64(event.EndLogPos) - int64(event.EventSize)
	if firstStart != existing.LastCompletePos {
		return agentBinlogAppendPlan{}, fmt.Errorf("resume candidate 起始 position 不连续: want=%d got=%d", existing.LastCompletePos, firstStart)
	}
	if payloadOffset == int64(len(agentBinlogMagic)) {
		tmpPath := candidatePath + ".payload-check"
		if err := copyAgentFileRange(candidatePath, tmpPath, payloadOffset, 0o600); err != nil {
			return agentBinlogAppendPlan{}, err
		}
		defer os.Remove(tmpPath)
		if _, err := validateAgentBinlogEventStream(tmpPath, existing.LastCompletePos); err != nil {
			return agentBinlogAppendPlan{}, err
		}
	} else if _, err := validateAgentBinlogEventStream(candidatePath, existing.LastCompletePos); err != nil {
		return agentBinlogAppendPlan{}, err
	}
	combinedPath := candidatePath + ".combined-check"
	if err := combineAgentBinlogAppend(existingPath, candidatePath, payloadOffset, combinedPath); err != nil {
		return agentBinlogAppendPlan{}, err
	}
	defer os.Remove(combinedPath)
	combined, err := validateAgentBinlogFile(combinedPath)
	if err != nil {
		return agentBinlogAppendPlan{}, err
	}
	if combined.LastCompletePos <= existing.LastCompletePos {
		return agentBinlogAppendPlan{}, fmt.Errorf("resume candidate 没有新增完整事件: before=%d after=%d", existing.LastCompletePos, combined.LastCompletePos)
	}
	return agentBinlogAppendPlan{
		PayloadOffset: payloadOffset,
		PayloadBytes:  candidateInfo.Size() - payloadOffset,
		FirstStartPos: firstStart,
		LastEndPos:    combined.LastCompletePos,
		ChecksumMode:  combined.ChecksumMode,
	}, nil
}

type agentBinlogEventHeader struct {
	EventType uint8
	EventSize uint32
	EndLogPos uint32
}

func readAgentBinlogEvent(file *os.File, offset, fileSize int64) (agentBinlogEventHeader, error) {
	if offset < 0 || offset+agentBinlogEventHeaderLen > fileSize {
		return agentBinlogEventHeader{}, fmt.Errorf("binlog event header 不完整: offset=%d size=%d", offset, fileSize)
	}
	header := make([]byte, agentBinlogEventHeaderLen)
	if _, err := file.ReadAt(header, offset); err != nil {
		return agentBinlogEventHeader{}, err
	}
	eventSize := binary.LittleEndian.Uint32(header[9:13])
	if eventSize < agentBinlogEventHeaderLen {
		return agentBinlogEventHeader{}, fmt.Errorf("binlog event_size 非法: offset=%d event_size=%d", offset, eventSize)
	}
	if offset+int64(eventSize) > fileSize {
		return agentBinlogEventHeader{}, fmt.Errorf("binlog event 不完整: offset=%d event_size=%d file_size=%d", offset, eventSize, fileSize)
	}
	return agentBinlogEventHeader{
		EventType: header[4],
		EventSize: eventSize,
		EndLogPos: binary.LittleEndian.Uint32(header[13:17]),
	}, nil
}

func agentBinlogEventCRC32Matches(file *os.File, offset int64, eventSize uint32) (bool, error) {
	if eventSize < agentBinlogEventHeaderLen+4 {
		return false, nil
	}
	hash := crc32.NewIEEE()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return false, err
	}
	if _, err := io.CopyN(hash, file, int64(eventSize)-4); err != nil {
		return false, err
	}
	checksumBytes := make([]byte, 4)
	if _, err := io.ReadFull(file, checksumBytes); err != nil {
		return false, err
	}
	expected := binary.LittleEndian.Uint32(checksumBytes)
	return hash.Sum32() == expected, nil
}

func validateAgentBinlogEventStream(path string, expectedStart int64) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	offset := int64(0)
	lastEnd := expectedStart
	for offset < info.Size() {
		event, err := readAgentBinlogEvent(file, offset, info.Size())
		if err != nil {
			return 0, err
		}
		start := int64(event.EndLogPos) - int64(event.EventSize)
		if start != lastEnd {
			return 0, fmt.Errorf("binlog event stream position 不连续: want=%d got=%d", lastEnd, start)
		}
		lastEnd = int64(event.EndLogPos)
		offset += int64(event.EventSize)
	}
	if offset != info.Size() {
		return 0, fmt.Errorf("binlog event stream 末尾不是完整边界: complete=%d size=%d", offset, info.Size())
	}
	if lastEnd <= expectedStart {
		return 0, fmt.Errorf("binlog event stream 没有新增事件: start=%d end=%d", expectedStart, lastEnd)
	}
	return lastEnd, nil
}

func combineAgentBinlogAppend(existingPath, candidatePath string, candidatePayloadOffset int64, targetPath string) error {
	if err := copyAgentFile(existingPath, targetPath, 0o600); err != nil {
		return err
	}
	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	in, err := os.Open(candidatePath)
	if err != nil {
		return err
	}
	defer in.Close()
	if candidatePayloadOffset > 0 {
		if _, err := in.Seek(candidatePayloadOffset, io.SeekStart); err != nil {
			return err
		}
	}
	_, err = io.Copy(out, in)
	return err
}

func copyAgentFile(src, dst string, mode os.FileMode) error {
	return copyAgentFileRange(src, dst, 0, mode)
}

func copyAgentFileRange(src, dst string, offset int64, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if offset > 0 {
		if _, err := in.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}
