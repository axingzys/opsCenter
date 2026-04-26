package database

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
)

const (
	redisMetadataSampleKeyLimit = 200
	redisMetadataScanCount      = 200
	redisPreviewMaxLength       = 256
)

type redisAccess struct {
	single         *redis.Client
	cluster        *redis.ClusterClient
	clusterEnabled bool
}

func openRedisAccess(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*redisAccess, error) {
	client, err := openRedisClient(item, credential)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := client.Ping(probeCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	clusterInfo, err := client.Info(probeCtx, "cluster").Result()
	if err != nil || parseRedisInfo(clusterInfo)["cluster_enabled"] != "1" {
		return &redisAccess{single: client}, nil
	}
	clusterClient, err := openRedisClusterClient(item, credential)
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := clusterClient.Ping(probeCtx).Err(); err != nil {
		clusterClient.Close()
		client.Close()
		return nil, fmt.Errorf("连接 Redis Cluster 失败: %w", err)
	}
	return &redisAccess{
		single:         client,
		cluster:        clusterClient,
		clusterEnabled: true,
	}, nil
}

func (r *redisAccess) Close() {
	if r == nil {
		return
	}
	if r.cluster != nil {
		_ = r.cluster.Close()
	}
	if r.single != nil {
		_ = r.single.Close()
	}
}

func (r *redisAccess) readServerInfo(ctx context.Context) (map[string]string, error) {
	if r == nil || r.single == nil {
		return nil, fmt.Errorf("Redis 连接未初始化")
	}
	infoText, err := r.single.Info(ctx, "server").Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis 服务端信息失败: %w", err)
	}
	return parseRedisInfo(infoText), nil
}

func openRedisClusterClient(item *DatabaseInstance, credential *ConnectionCredential) (*redis.ClusterClient, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}
	options := &redis.ClusterOptions{
		Addrs:        []string{net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))},
		Username:     strings.TrimSpace(credential.Username),
		Password:     credential.Password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if item.TLSEnabled {
		options.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
	}
	return redis.NewClusterClient(options), nil
}

func openRedisClientWithDB(item *DatabaseInstance, credential *ConnectionCredential, dbIndex int) (*redis.Client, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}
	if settings, err := parseRedisSentinelSettingsFromParams(item, credential, params); err != nil {
		return nil, err
	} else if settings != nil {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       settings.MasterName,
			SentinelAddrs:    settings.SentinelAddrs,
			Username:         settings.Username,
			Password:         settings.Password,
			SentinelUsername: settings.SentinelUsername,
			SentinelPassword: settings.SentinelPassword,
			DB:               dbIndex,
			DialTimeout:      5 * time.Second,
			ReadTimeout:      10 * time.Second,
			WriteTimeout:     10 * time.Second,
			TLSConfig:        settings.TLSConfig,
		}), nil
	}
	options := &redis.Options{
		Addr:         net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port)),
		Username:     strings.TrimSpace(credential.Username),
		Password:     credential.Password,
		DB:           dbIndex,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if item.TLSEnabled {
		options.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
	}
	return redis.NewClient(options), nil
}

func openRedisNodeClientByAddr(address string, item *DatabaseInstance, credential *ConnectionCredential, dbIndex int) (*redis.Client, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}
	options := &redis.Options{
		Addr:         strings.TrimSpace(address),
		Username:     strings.TrimSpace(credential.Username),
		Password:     credential.Password,
		DB:           dbIndex,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if item.TLSEnabled {
		options.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
	}
	return redis.NewClient(options), nil
}

func resolveRedisDBIndex(item *DatabaseInstance, schemaName string, clusterEnabled bool) (int, error) {
	dbIndex, err := parseRedisDBIndex(schemaName)
	if err == nil {
		if clusterEnabled && dbIndex != 0 {
			return 0, fmt.Errorf("Redis Cluster 仅支持 db0")
		}
		return dbIndex, nil
	}
	if schemaName != "" {
		return 0, err
	}
	if item != nil && strings.TrimSpace(item.DefaultDatabase) != "" {
		index, parseErr := parseRedisDBIndex(item.DefaultDatabase)
		if parseErr != nil {
			return 0, parseErr
		}
		if clusterEnabled && index != 0 {
			return 0, fmt.Errorf("Redis Cluster 仅支持 db0")
		}
		return index, nil
	}
	params, parseErr := parseConnectionParams(item)
	if parseErr == nil {
		index := connectionParamInt(params, 0, "db", "database")
		if clusterEnabled && index != 0 {
			return 0, fmt.Errorf("Redis Cluster 仅支持 db0")
		}
		return index, nil
	}
	return 0, nil
}

func parseRedisDBIndex(value string) (int, error) {
	text := strings.TrimSpace(strings.ToLower(value))
	if text == "" {
		return 0, fmt.Errorf("Redis 逻辑 DB 不能为空")
	}
	if strings.HasPrefix(text, "db") {
		text = strings.TrimPrefix(text, "db")
	}
	index, err := strconv.Atoi(text)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("Redis 逻辑 DB 格式错误")
	}
	return index, nil
}

func redisSchemaName(dbIndex int) string {
	return fmt.Sprintf("db%d", dbIndex)
}

func formatRedisTTL(ttlMillis int64) string {
	switch {
	case ttlMillis == -2:
		return "已删除"
	case ttlMillis == -1:
		return "永不过期"
	case ttlMillis <= 0:
		return "0 ms"
	}
	duration := time.Duration(ttlMillis) * time.Millisecond
	switch {
	case duration >= 24*time.Hour:
		return fmt.Sprintf("%.1f 天", duration.Hours()/24)
	case duration >= time.Hour:
		return fmt.Sprintf("%.1f 小时", duration.Hours())
	case duration >= time.Minute:
		return fmt.Sprintf("%.1f 分钟", duration.Minutes())
	case duration >= time.Second:
		return fmt.Sprintf("%.1f 秒", duration.Seconds())
	default:
		return fmt.Sprintf("%d ms", ttlMillis)
	}
}

func sanitizeRedisPreview(value string) string {
	if value == "" {
		return ""
	}
	if !utf8.ValidString(value) {
		return fmt.Sprintf("<binary %d bytes>", len(value))
	}
	text := strings.ReplaceAll(value, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	if len([]rune(text)) <= redisPreviewMaxLength {
		return text
	}
	runes := []rune(text)
	return string(runes[:redisPreviewMaxLength]) + "..."
}
