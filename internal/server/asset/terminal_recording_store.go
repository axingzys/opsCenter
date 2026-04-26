package asset

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ydcloud-dy/opshub/internal/conf"
)

var (
	ErrTerminalRecordingPathEmpty = errors.New("未找到录屏文件路径")
	ErrTerminalRecordingMissing   = errors.New("录屏文件不存在，可能已因容器重建或清理丢失")
	ErrTerminalRecordingInvalid   = errors.New("录屏文件格式无效")
)

type terminalRecordingStore struct {
	rootDir string
}

func newTerminalRecordingStore(cfg conf.TerminalConfig) *terminalRecordingStore {
	rootDir := strings.TrimSpace(cfg.GetRecordingPath())
	if rootDir == "" {
		rootDir = "./data/terminal-recordings"
	}
	if abs, err := filepath.Abs(rootDir); err == nil {
		rootDir = abs
	}
	return &terminalRecordingStore{rootDir: filepath.Clean(rootDir)}
}

func (s *terminalRecordingStore) RootDir() string {
	return s.rootDir
}

func (s *terminalRecordingStore) CreateRecorder(cols, rows int) (*AsciinemaRecorder, error) {
	return NewAsciinemaRecorder(s.rootDir, cols, rows)
}

func (s *terminalRecordingStore) NormalizeForSave(recordingPath string) string {
	cleaned := strings.TrimSpace(recordingPath)
	if cleaned == "" {
		return ""
	}
	if abs, err := filepath.Abs(cleaned); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(cleaned)
}

func (s *terminalRecordingStore) Resolve(recordingPath string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(recordingPath))
	if cleaned == "" || cleaned == "." {
		return "", ErrTerminalRecordingPathEmpty
	}

	candidates := make([]string, 0, 4)
	pushCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		candidate = filepath.Clean(candidate)
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	if filepath.IsAbs(cleaned) {
		pushCandidate(cleaned)
	} else {
		if abs, err := filepath.Abs(cleaned); err == nil {
			pushCandidate(abs)
		}
	}

	if s.rootDir != "" {
		if strings.Contains(filepath.ToSlash(cleaned), "terminal-recordings/") {
			parts := strings.SplitN(filepath.ToSlash(cleaned), "terminal-recordings/", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				pushCandidate(filepath.Join(s.rootDir, filepath.FromSlash(parts[1])))
			}
		}
		if base := filepath.Base(cleaned); base != "" && base != "." && base != string(filepath.Separator) {
			pushCandidate(filepath.Join(s.rootDir, base))
		}
	}

	for _, candidate := range candidates {
		fileInfo, err := os.Stat(candidate)
		if err == nil {
			if fileInfo.IsDir() {
				return "", ErrTerminalRecordingInvalid
			}
			return candidate, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}

	return "", ErrTerminalRecordingMissing
}

func (s *terminalRecordingStore) ReadValidated(recordingPath string) ([]byte, string, error) {
	resolvedPath, err := s.Resolve(recordingPath)
	if err != nil {
		return nil, "", err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrTerminalRecordingMissing
		}
		return nil, "", err
	}
	if err := validateAsciinemaCast(content); err != nil {
		return nil, "", err
	}

	return content, resolvedPath, nil
}

func validateAsciinemaCast(content []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	if !scanner.Scan() {
		return ErrTerminalRecordingInvalid
	}

	var header struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		return fmt.Errorf("%w: %v", ErrTerminalRecordingInvalid, err)
	}
	if header.Version != 2 || header.Width <= 0 || header.Height <= 0 {
		return ErrTerminalRecordingInvalid
	}
	return nil
}
