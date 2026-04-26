package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func calculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("读取备份文件失败: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算备份文件 checksum 失败: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (uc *UseCase) verifyBackupRecordFile(ctx context.Context, record *DatabaseBackupRecord, filePath string) (os.FileInfo, error) {
	if record == nil {
		return nil, fmt.Errorf("备份记录不存在")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		var err error
		filePath, err = uc.secureBackupFilePath(ctx, record.FilePath)
		if err != nil {
			return nil, fmt.Errorf("备份文件路径无效: %w", err)
		}
	}

	info, err := os.Stat(filePath)
	if err != nil || info == nil || info.IsDir() {
		return nil, fmt.Errorf("备份文件不存在")
	}
	if record.FileSize > 0 && info.Size() != record.FileSize {
		return nil, fmt.Errorf("备份文件大小校验失败：记录 %d 字节，实际 %d 字节", record.FileSize, info.Size())
	}
	expectedChecksum := strings.ToLower(strings.TrimSpace(record.ChecksumSHA256))
	if expectedChecksum == "" {
		return nil, fmt.Errorf("备份记录缺少 checksum，请重新执行备份")
	}
	actualChecksum, err := calculateFileSHA256(filePath)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(expectedChecksum, actualChecksum) {
		return nil, fmt.Errorf("备份文件 checksum 校验失败")
	}
	return info, nil
}
