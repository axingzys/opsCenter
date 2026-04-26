package database

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisLogicalBackupFileExt     = ".redis.json.gz"
	redisLogicalBackupFormat      = "redis-logical-backup"
	redisLogicalBackupVersion     = 1
	redisLogicalBackupKindHeader  = "header"
	redisLogicalBackupKindKey     = "key"
	redisBackupScanCount          = 1000
	redisBackupDatabaseName       = "all-dbs"
	redisBackupStandaloneTopology = "standalone"
	redisBackupClusterTopology    = "cluster"
	redisBackupSentinelTopology   = "sentinel"
)

type redisLogicalBackupHeader struct {
	Kind              string `json:"kind"`
	Format            string `json:"format"`
	Version           int    `json:"version"`
	SourceDBType      string `json:"sourceDbType"`
	SourceTopology    string `json:"sourceTopology"`
	DatabaseName      string `json:"databaseName"`
	ClusterEnabled    bool   `json:"clusterEnabled"`
	DBIndexes         []int  `json:"dbIndexes"`
	EstimatedKeyCount int64  `json:"estimatedKeyCount"`
	GeneratedAt       string `json:"generatedAt"`
}

type redisLogicalBackupEntry struct {
	Kind        string `json:"kind"`
	DBIndex     int    `json:"dbIndex"`
	KeyName     string `json:"keyName"`
	TTLMillis   int64  `json:"ttlMillis"`
	DumpBase64  string `json:"dumpBase64"`
	NodeID      string `json:"nodeId,omitempty"`
	NodeAddress string `json:"nodeAddress,omitempty"`
}

func buildRedisBackupCommandSpec(item *DatabaseInstance, credential *ConnectionCredential, databaseName string) (*backupCommandSpec, error) {
	if item == nil {
		return nil, fmt.Errorf("数据库实例不存在")
	}
	if credential == nil {
		return nil, fmt.Errorf("凭据不存在")
	}
	if strings.TrimSpace(databaseName) == "" {
		databaseName = redisBackupDatabaseName
	}
	return &backupCommandSpec{
		DatabaseName: databaseName,
		FileExt:      redisLogicalBackupFileExt,
		Runner: func(ctx context.Context, outputPath string) (int64, error) {
			return runRedisLogicalBackup(ctx, item, credential, outputPath)
		},
	}, nil
}

func buildRedisRestoreCommandSpec(item *DatabaseInstance, credential *ConnectionCredential, databaseName string) (*restoreCommandSpec, error) {
	if item == nil {
		return nil, fmt.Errorf("目标数据库实例不存在")
	}
	if credential == nil {
		return nil, fmt.Errorf("目标实例凭据不存在")
	}
	if strings.TrimSpace(databaseName) == "" {
		databaseName = redisBackupDatabaseName
	}
	return &restoreCommandSpec{
		DatabaseName: databaseName,
		Runner: func(ctx context.Context, inputPath string) error {
			return runRedisLogicalRestore(ctx, item, credential, inputPath)
		},
	}, nil
}

func runRedisLogicalBackup(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, outputPath string) (int64, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return 0, err
	}
	defer access.Close()

	header, err := buildRedisLogicalBackupHeader(ctx, item, credential, access)
	if err != nil {
		return 0, err
	}

	return writeRedisLogicalBackupFile(outputPath, func(enc *json.Encoder) error {
		if err := enc.Encode(header); err != nil {
			return fmt.Errorf("写入 Redis 备份头失败: %w", err)
		}
		if access.clusterEnabled {
			return encodeRedisClusterBackupEntries(ctx, item, credential, access, enc)
		}
		return encodeRedisStandaloneBackupEntries(ctx, item, credential, header.DBIndexes, enc)
	})
}

func buildRedisLogicalBackupHeader(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess) (*redisLogicalBackupHeader, error) {
	if access == nil || access.single == nil {
		return nil, fmt.Errorf("Redis 连接未初始化")
	}
	header := &redisLogicalBackupHeader{
		Kind:           redisLogicalBackupKindHeader,
		Format:         redisLogicalBackupFormat,
		Version:        redisLogicalBackupVersion,
		SourceDBType:   DBTypeRedis,
		DatabaseName:   redisBackupDatabaseName,
		ClusterEnabled: access.clusterEnabled,
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}
	if access.clusterEnabled {
		header.SourceTopology = redisBackupClusterTopology
		header.DBIndexes = []int{0}
		nodesText, err := access.single.ClusterNodes(ctx).Result()
		if err != nil {
			return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
		}
		nodes, _, _ := parseRedisClusterNodes(nodesText)
		for _, node := range nodes {
			if strings.ToLower(strings.TrimSpace(node.Role)) != "master" {
				continue
			}
			if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
				continue
			}
			nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
			if err != nil {
				return nil, err
			}
			infoText, infoErr := nodeClient.Info(ctx, "keyspace").Result()
			nodeClient.Close()
			if infoErr != nil {
				return nil, fmt.Errorf("读取 Redis Cluster keyspace 信息失败: %w", infoErr)
			}
			stats := parseRedisKeyspaceStats(parseRedisInfo(infoText))
			if stat, ok := stats[0]; ok {
				header.EstimatedKeyCount += stat.keyCount
			}
		}
		return header, nil
	}

	infoText, err := access.single.Info(ctx, "keyspace").Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis keyspace 信息失败: %w", err)
	}
	stats := parseRedisKeyspaceStats(parseRedisInfo(infoText))
	if len(stats) == 0 {
		defaultDB, resolveErr := resolveRedisDBIndex(item, "", false)
		if resolveErr != nil {
			return nil, resolveErr
		}
		header.DBIndexes = []int{defaultDB}
	} else {
		header.DBIndexes = make([]int, 0, len(stats))
		for dbIndex, stat := range stats {
			header.DBIndexes = append(header.DBIndexes, dbIndex)
			header.EstimatedKeyCount += stat.keyCount
		}
		sort.Ints(header.DBIndexes)
	}
	if settings, _ := parseRedisSentinelSettings(item, credential); settings != nil {
		header.SourceTopology = redisBackupSentinelTopology
	} else {
		header.SourceTopology = redisBackupStandaloneTopology
	}
	return header, nil
}

func encodeRedisStandaloneBackupEntries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, dbIndexes []int, enc *json.Encoder) error {
	if len(dbIndexes) == 0 {
		dbIndexes = []int{0}
	}
	address := resolveRedisStandaloneAddress(ctx, item, credential)
	for _, dbIndex := range dbIndexes {
		dbClient, err := openRedisClientWithDB(item, credential, dbIndex)
		if err != nil {
			return err
		}
		err = encodeRedisBackupEntries(ctx, dbClient, dbIndex, "", address, enc)
		dbClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeRedisClusterBackupEntries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess, enc *json.Encoder) error {
	if access == nil || access.single == nil {
		return fmt.Errorf("Redis Cluster 连接未初始化")
	}
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	sortTopologyNodes(nodes)
	for _, node := range nodes {
		if strings.ToLower(strings.TrimSpace(node.Role)) != "master" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return err
		}
		err = encodeRedisBackupEntries(ctx, nodeClient, 0, strings.TrimSpace(node.ID), strings.TrimSpace(node.Address), enc)
		nodeClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeRedisBackupEntries(ctx context.Context, client redis.Cmdable, dbIndex int, nodeID, nodeAddress string, enc *json.Encoder) error {
	var cursor uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		keys, nextCursor, err := client.Scan(ctx, cursor, "", int64(redisBackupScanCount)).Result()
		if err != nil {
			return fmt.Errorf("扫描 Redis Key 失败: %w", err)
		}
		for _, keyName := range keys {
			dumpValue, ttlMillis, err := readRedisDumpEntry(ctx, client, keyName)
			if err != nil {
				return err
			}
			if dumpValue == nil {
				continue
			}
			entry := &redisLogicalBackupEntry{
				Kind:        redisLogicalBackupKindKey,
				DBIndex:     dbIndex,
				KeyName:     keyName,
				TTLMillis:   ttlMillis,
				DumpBase64:  base64.StdEncoding.EncodeToString(dumpValue),
				NodeID:      nodeID,
				NodeAddress: nodeAddress,
			}
			if err := enc.Encode(entry); err != nil {
				return fmt.Errorf("写入 Redis 备份条目失败: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func readRedisDumpEntry(ctx context.Context, client redis.Cmdable, keyName string) ([]byte, int64, error) {
	dumpValue, err := client.Dump(ctx, keyName).Result()
	if err == redis.Nil {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("读取 Redis Key %s 序列化内容失败: %w", keyName, err)
	}
	ttlDuration, ttlErr := client.PTTL(ctx, keyName).Result()
	if ttlErr == redis.Nil {
		return nil, 0, nil
	}
	if ttlErr != nil {
		return nil, 0, fmt.Errorf("读取 Redis Key %s TTL 失败: %w", keyName, ttlErr)
	}
	return []byte(dumpValue), normalizeRedisBackupTTL(ttlDuration), nil
}

func normalizeRedisBackupTTL(ttlDuration time.Duration) int64 {
	ttlMillis := int64(ttlDuration / time.Millisecond)
	switch {
	case ttlMillis < -1:
		return -2
	case ttlMillis < 0:
		return -1
	default:
		return ttlMillis
	}
}

func writeRedisLogicalBackupFile(outputPath string, writer func(enc *json.Encoder) error) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建备份目录失败: %w", err)
	}
	tmpPath := outputPath + ".partial"
	file, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("创建 Redis 备份文件失败: %w", err)
	}

	gzipWriter := gzip.NewWriter(file)
	encoder := json.NewEncoder(gzipWriter)
	encoder.SetEscapeHTML(false)

	writeErr := writer(encoder)
	closeErr := gzipWriter.Close()
	fileCloseErr := file.Close()
	if writeErr != nil || closeErr != nil || fileCloseErr != nil {
		_ = os.Remove(tmpPath)
		if writeErr != nil {
			return 0, writeErr
		}
		if closeErr != nil {
			return 0, fmt.Errorf("写入 Redis 备份压缩文件失败: %w", closeErr)
		}
		return 0, fmt.Errorf("关闭 Redis 备份文件失败: %w", fileCloseErr)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("写入 Redis 备份文件失败: %w", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return 0, fmt.Errorf("读取 Redis 备份文件失败: %w", err)
	}
	return info.Size(), nil
}

func runRedisLogicalRestore(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, inputPath string) error {
	header, reader, cleanup, err := openRedisLogicalBackupReader(inputPath)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := validateRedisLogicalBackupHeader(header); err != nil {
		return err
	}

	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return err
	}
	defer access.Close()

	if access.clusterEnabled {
		for _, dbIndex := range header.DBIndexes {
			if dbIndex != 0 {
				return fmt.Errorf("Redis Cluster 仅支持恢复 db0，当前备份包含 db%d", dbIndex)
			}
		}
	}

	dbClients := make(map[int]*redis.Client)
	defer closeRedisClients(dbClients)

	for {
		var entry redisLogicalBackupEntry
		if err := reader.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("解析 Redis 备份条目失败: %w", err)
		}
		if entry.Kind != redisLogicalBackupKindKey {
			return fmt.Errorf("Redis 备份条目格式错误")
		}
		if strings.TrimSpace(entry.KeyName) == "" {
			return fmt.Errorf("Redis 备份条目缺少 Key 名称")
		}
		payload, err := base64.StdEncoding.DecodeString(entry.DumpBase64)
		if err != nil {
			return fmt.Errorf("解析 Redis Key %s 序列化内容失败: %w", entry.KeyName, err)
		}
		if err := restoreRedisBackupEntry(ctx, access, item, credential, dbClients, &entry, payload); err != nil {
			return err
		}
	}
	return nil
}

func openRedisLogicalBackupReader(inputPath string) (*redisLogicalBackupHeader, *json.Decoder, func(), error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("打开备份文件失败: %w", err)
	}

	var reader io.Reader = file
	var gzipReader *gzip.Reader
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(inputPath)), ".gz") {
		gzipReader, err = gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, nil, nil, fmt.Errorf("读取 Redis 备份压缩文件失败: %w", err)
		}
		reader = gzipReader
	}

	decoder := json.NewDecoder(reader)
	var header redisLogicalBackupHeader
	if err := decoder.Decode(&header); err != nil {
		if gzipReader != nil {
			gzipReader.Close()
		}
		file.Close()
		return nil, nil, nil, fmt.Errorf("解析 Redis 备份头失败: %w", err)
	}
	return &header, decoder, func() {
		if gzipReader != nil {
			_ = gzipReader.Close()
		}
		_ = file.Close()
	}, nil
}

func validateRedisLogicalBackupHeader(header *redisLogicalBackupHeader) error {
	if header == nil {
		return fmt.Errorf("Redis 备份头不存在")
	}
	if header.Kind != redisLogicalBackupKindHeader {
		return fmt.Errorf("Redis 备份文件格式错误")
	}
	if header.Format != redisLogicalBackupFormat || header.Version != redisLogicalBackupVersion {
		return fmt.Errorf("Redis 备份文件版本不兼容")
	}
	if normalizeDBType(header.SourceDBType) != DBTypeRedis {
		return fmt.Errorf("仅支持 Redis 备份文件恢复演练")
	}
	return nil
}

func restoreRedisBackupEntry(ctx context.Context, access *redisAccess, item *DatabaseInstance, credential *ConnectionCredential, dbClients map[int]*redis.Client, entry *redisLogicalBackupEntry, payload []byte) error {
	if entry == nil {
		return nil
	}
	ttl := time.Duration(0)
	if entry.TTLMillis > 0 {
		ttl = time.Duration(entry.TTLMillis) * time.Millisecond
	}
	if access != nil && access.clusterEnabled {
		if entry.DBIndex != 0 {
			return fmt.Errorf("Redis Cluster 仅支持恢复 db0，当前条目 %s 位于 db%d", entry.KeyName, entry.DBIndex)
		}
		if err := access.cluster.RestoreReplace(ctx, entry.KeyName, ttl, string(payload)).Err(); err != nil {
			return fmt.Errorf("恢复 Redis Key %s 失败: %w", entry.KeyName, err)
		}
		return nil
	}

	dbClient, ok := dbClients[entry.DBIndex]
	if !ok {
		client, err := openRedisClientWithDB(item, credential, entry.DBIndex)
		if err != nil {
			return err
		}
		dbClients[entry.DBIndex] = client
		dbClient = client
	}
	if err := dbClient.RestoreReplace(ctx, entry.KeyName, ttl, string(payload)).Err(); err != nil {
		return fmt.Errorf("恢复 Redis Key %s 失败: %w", entry.KeyName, err)
	}
	return nil
}

func closeRedisClients(clients map[int]*redis.Client) {
	for _, client := range clients {
		if client != nil {
			_ = client.Close()
		}
	}
}
