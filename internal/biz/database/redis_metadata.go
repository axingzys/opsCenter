package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisKeyspaceAggregate struct {
	dbIndex      int
	keyCount     int64
	expiringKeys int64
	avgTTLMillis int64
	sampledKeys  int
}

func collectRedisMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, []*DatabaseRedisKeyspace, []*DatabaseRedisKeySample, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return "", nil, nil, err
	}
	defer access.Close()

	info, err := access.readServerInfo(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	version := valueOrDefault(info["redis_version"], "Redis")

	if access.clusterEnabled {
		keyspaces, keys, collectErr := collectRedisClusterMetadata(ctx, item, credential, access)
		return version, keyspaces, keys, collectErr
	}
	keyspaces, keys, collectErr := collectRedisStandaloneMetadata(ctx, item, credential)
	return version, keyspaces, keys, collectErr
}

func collectRedisStandaloneMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseRedisKeyspace, []*DatabaseRedisKeySample, error) {
	baseClient, err := openRedisClient(item, credential)
	if err != nil {
		return nil, nil, err
	}
	defer baseClient.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	infoText, err := baseClient.Info(queryCtx, "keyspace").Result()
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Redis keyspace 信息失败: %w", err)
	}
	stats := parseRedisKeyspaceStats(parseRedisInfo(infoText))
	if len(stats) == 0 {
		defaultDB, resolveErr := resolveRedisDBIndex(item, "", false)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		stats[defaultDB] = &redisKeyspaceAggregate{dbIndex: defaultDB}
	}

	dbIndexes := make([]int, 0, len(stats))
	for dbIndex := range stats {
		dbIndexes = append(dbIndexes, dbIndex)
	}
	sort.Ints(dbIndexes)

	keys := make([]*DatabaseRedisKeySample, 0)
	keyspaces := make([]*DatabaseRedisKeyspace, 0, len(dbIndexes))
	remaining := redisMetadataSampleKeyLimit
	address := resolveRedisStandaloneAddress(queryCtx, item, credential)
	for _, dbIndex := range dbIndexes {
		dbClient, err := openRedisClientWithDB(item, credential, dbIndex)
		if err != nil {
			return nil, nil, err
		}
		samples := make([]*DatabaseRedisKeySample, 0)
		if remaining > 0 {
			samples, err = scanRedisKeySamples(queryCtx, dbClient, dbIndex, remaining, "", address, false)
			if err != nil {
				dbClient.Close()
				return nil, nil, err
			}
		}
		dbClient.Close()
		keys = append(keys, samples...)
		remaining -= len(samples)
		stat := stats[dbIndex]
		stat.sampledKeys = len(samples)
		keyspaces = append(keyspaces, buildRedisKeyspace(dbIndex, stat))
	}
	return keyspaces, keys, nil
}

func collectRedisClusterMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess) ([]*DatabaseRedisKeyspace, []*DatabaseRedisKeySample, error) {
	if access == nil || access.single == nil {
		return nil, nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	nodesText, err := access.single.ClusterNodes(queryCtx).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	sortTopologyNodes(nodes)

	keyspaceStats := make(map[int]*redisKeyspaceAggregate)
	keys := make([]*DatabaseRedisKeySample, 0)
	remaining := redisMetadataSampleKeyLimit
	for _, node := range nodes {
		if strings.ToLower(strings.TrimSpace(node.Role)) != "master" {
			continue
		}
		if strings.Contains(strings.ToLower(node.State), "fail") {
			continue
		}
		nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return nil, nil, err
		}
		infoText, infoErr := nodeClient.Info(queryCtx, "keyspace").Result()
		if infoErr == nil {
			mergeRedisKeyspaceStats(keyspaceStats, parseRedisKeyspaceStats(parseRedisInfo(infoText)))
		}
		if remaining > 0 {
			samples, sampleErr := scanRedisKeySamples(queryCtx, nodeClient, 0, remaining, node.ID, node.Address, true)
			if sampleErr != nil {
				nodeClient.Close()
				return nil, nil, sampleErr
			}
			keys = append(keys, samples...)
			remaining -= len(samples)
			ensureRedisKeyspaceStat(keyspaceStats, 0).sampledKeys += len(samples)
		}
		nodeClient.Close()
	}

	if _, ok := keyspaceStats[0]; !ok {
		keyspaceStats[0] = &redisKeyspaceAggregate{dbIndex: 0, sampledKeys: len(keys)}
	}
	dbIndexes := make([]int, 0, len(keyspaceStats))
	for dbIndex := range keyspaceStats {
		dbIndexes = append(dbIndexes, dbIndex)
	}
	sort.Ints(dbIndexes)
	keyspaces := make([]*DatabaseRedisKeyspace, 0, len(dbIndexes))
	for _, dbIndex := range dbIndexes {
		keyspaces = append(keyspaces, buildRedisKeyspace(dbIndex, keyspaceStats[dbIndex]))
	}
	return keyspaces, keys, nil
}

func scanRedisKeySamples(ctx context.Context, client redis.Cmdable, dbIndex, limit int, nodeID, nodeAddress string, clusterEnabled bool) ([]*DatabaseRedisKeySample, error) {
	if limit <= 0 {
		return []*DatabaseRedisKeySample{}, nil
	}
	result := make([]*DatabaseRedisKeySample, 0, limit)
	var cursor uint64
	seen := make(map[string]struct{}, limit)
	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, "", int64(redisMetadataScanCount)).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描 Redis Key 失败: %w", err)
		}
		for _, key := range keys {
			if _, ok := seen[key]; ok {
				continue
			}
			sample, sampleErr := collectRedisKeySample(ctx, client, dbIndex, key, nodeID, nodeAddress, clusterEnabled)
			if sampleErr == nil && sample != nil {
				result = append(result, sample)
				seen[key] = struct{}{}
			}
			if len(result) >= limit {
				return result, nil
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return result, nil
}

func collectRedisKeySample(ctx context.Context, client redis.Cmdable, dbIndex int, keyName, nodeID, nodeAddress string, clusterEnabled bool) (*DatabaseRedisKeySample, error) {
	keyType, err := client.Type(ctx, keyName).Result()
	if err != nil {
		return nil, err
	}
	if keyType == "" || keyType == "none" {
		return nil, nil
	}
	ttlResult := client.PTTL(ctx, keyName)
	ttlMillis := int64(ttlResult.Val() / time.Millisecond)
	if pttlErr := ttlResult.Err(); pttlErr != nil && pttlErr != redis.Nil {
		return nil, pttlErr
	}
	memoryUsage, _ := client.MemoryUsage(ctx, keyName).Result()
	encoding, _ := client.ObjectEncoding(ctx, keyName).Result()
	valueSize, previewText, err := readRedisKeyPreview(ctx, client, keyName, keyType)
	if err != nil {
		return nil, err
	}
	slot := 0
	if clusterEnabled {
		slotResult, slotErr := client.ClusterKeySlot(ctx, keyName).Result()
		if slotErr == nil {
			slot = int(slotResult)
		}
	}
	return &DatabaseRedisKeySample{
		DBIndex:          dbIndex,
		KeyspaceName:     redisSchemaName(dbIndex),
		KeyName:          keyName,
		KeyType:          keyType,
		TTLMillis:        ttlMillis,
		MemoryUsageBytes: memoryUsage,
		ValueSize:        valueSize,
		Encoding:         encoding,
		Slot:             slot,
		NodeID:           nodeID,
		NodeAddress:      nodeAddress,
		PreviewText:      previewText,
	}, nil
}

func readRedisKeyPreview(ctx context.Context, client redis.Cmdable, keyName, keyType string) (int64, string, error) {
	switch strings.ToLower(strings.TrimSpace(keyType)) {
	case "string":
		value, err := client.Get(ctx, keyName).Result()
		if err != nil && err != redis.Nil {
			return 0, "", err
		}
		size, _ := client.StrLen(ctx, keyName).Result()
		return size, sanitizeRedisPreview(value), nil
	case "hash":
		size, _ := client.HLen(ctx, keyName).Result()
		values, err := client.HGetAll(ctx, keyName).Result()
		if err != nil {
			return 0, "", err
		}
		return size, sanitizeRedisPreview(marshalPreview(values)), nil
	case "list":
		size, _ := client.LLen(ctx, keyName).Result()
		values, err := client.LRange(ctx, keyName, 0, 4).Result()
		if err != nil {
			return 0, "", err
		}
		return size, sanitizeRedisPreview(marshalPreview(values)), nil
	case "set":
		size, _ := client.SCard(ctx, keyName).Result()
		values, err := client.SRandMemberN(ctx, keyName, 5).Result()
		if err != nil {
			return 0, "", err
		}
		return size, sanitizeRedisPreview(marshalPreview(values)), nil
	case "zset":
		size, _ := client.ZCard(ctx, keyName).Result()
		values, err := client.ZRangeWithScores(ctx, keyName, 0, 4).Result()
		if err != nil {
			return 0, "", err
		}
		return size, sanitizeRedisPreview(marshalPreview(values)), nil
	case "stream":
		size, _ := client.XLen(ctx, keyName).Result()
		values, err := client.XRangeN(ctx, keyName, "-", "+", 3).Result()
		if err != nil {
			return 0, "", err
		}
		return size, sanitizeRedisPreview(marshalPreview(values)), nil
	default:
		return 0, "", nil
	}
}

func marshalPreview(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(body)
}

func parseRedisKeyspaceStats(info map[string]string) map[int]*redisKeyspaceAggregate {
	result := make(map[int]*redisKeyspaceAggregate)
	for key, rawValue := range info {
		dbIndex, err := parseRedisDBIndex(key)
		if err != nil {
			continue
		}
		aggregate := &redisKeyspaceAggregate{dbIndex: dbIndex}
		for _, pair := range strings.Split(rawValue, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.TrimSpace(parts[0]) {
			case "keys":
				aggregate.keyCount = parseInt64Default(parts[1], 0)
			case "expires":
				aggregate.expiringKeys = parseInt64Default(parts[1], 0)
			case "avg_ttl":
				aggregate.avgTTLMillis = parseInt64Default(parts[1], 0)
			}
		}
		result[dbIndex] = aggregate
	}
	return result
}

func mergeRedisKeyspaceStats(target map[int]*redisKeyspaceAggregate, source map[int]*redisKeyspaceAggregate) {
	for dbIndex, incoming := range source {
		current, ok := target[dbIndex]
		if !ok {
			target[dbIndex] = incoming
			continue
		}
		currentWeight := current.expiringKeys
		incomingWeight := incoming.expiringKeys
		current.keyCount += incoming.keyCount
		current.expiringKeys += incoming.expiringKeys
		totalWeight := currentWeight + incomingWeight
		if totalWeight > 0 {
			current.avgTTLMillis = int64((float64(current.avgTTLMillis)*float64(currentWeight) + float64(incoming.avgTTLMillis)*float64(incomingWeight)) / float64(totalWeight))
		}
		current.sampledKeys += incoming.sampledKeys
	}
}

func ensureRedisKeyspaceStat(target map[int]*redisKeyspaceAggregate, dbIndex int) *redisKeyspaceAggregate {
	if current, ok := target[dbIndex]; ok {
		return current
	}
	current := &redisKeyspaceAggregate{dbIndex: dbIndex}
	target[dbIndex] = current
	return current
}

func buildRedisKeyspace(dbIndex int, stat *redisKeyspaceAggregate) *DatabaseRedisKeyspace {
	if stat == nil {
		stat = &redisKeyspaceAggregate{dbIndex: dbIndex}
	}
	return &DatabaseRedisKeyspace{
		DBIndex:      dbIndex,
		KeyspaceName: redisSchemaName(dbIndex),
		KeyCount:     stat.keyCount,
		ExpiringKeys: stat.expiringKeys,
		AvgTTLMillis: stat.avgTTLMillis,
		SampledKeys:  stat.sampledKeys,
	}
}

func parseInt64Default(value string, defaultValue int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func netJoinRedisAddress(host string, port int) string {
	return strings.TrimSpace(fmt.Sprintf("%s:%d", strings.TrimSpace(host), port))
}

func (uc *UseCase) syncRedisMetadata(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, job *DatabaseSyncJob, start time.Time) (*MetadataSyncResultVO, error) {
	if uc.redisMetadataRepo == nil {
		return nil, fmt.Errorf("Redis 元数据仓储未配置")
	}
	version, keyspaces, keys, err := collectRedisMetadata(ctx, item, credential)
	now := time.Now()
	if err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, err.Error(), start, now, 0, 0, 0, 0)
		return nil, err
	}
	for _, keyspace := range keyspaces {
		keyspace.InstanceID = item.ID
		keyspace.LastSyncAt = &now
	}
	for _, key := range keys {
		key.InstanceID = item.ID
		key.LastSyncAt = &now
	}
	if err := uc.redisMetadataRepo.ReplaceAll(ctx, item.ID, keyspaces, keys); err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, "写入 Redis 元数据失败: "+err.Error(), start, now, 0, 0, 0, 0)
		return nil, fmt.Errorf("写入 Redis 元数据失败: %w", err)
	}
	item.Version = version
	item.LastSyncAt = &now
	if strings.TrimSpace(item.Engine) == "" {
		item.Engine = item.DBType
	}
	if err := uc.instanceRepo.Update(ctx, item); err != nil {
		uc.finishSyncJob(ctx, job, DatabaseSyncStatusFailed, "更新实例同步时间失败: "+err.Error(), start, now, 0, 0, 0, 0)
		return nil, err
	}
	message := "Redis 元数据同步成功"
	uc.finishSyncJob(ctx, job, DatabaseSyncStatusSuccess, message, start, now, len(keyspaces), len(keys), len(keys), 0)
	return &MetadataSyncResultVO{
		JobID:        job.ID,
		InstanceID:   item.ID,
		Name:         item.Name,
		Status:       DatabaseSyncStatusSuccess,
		Message:      message,
		Version:      version,
		DurationMs:   now.Sub(start).Milliseconds(),
		SchemasCount: len(keyspaces),
		TablesCount:  len(keys),
		ColumnsCount: len(keys),
		IndexesCount: 0,
		SyncedAt:     now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (uc *UseCase) listRedisSchemas(ctx context.Context, instanceID uint) ([]*DatabaseSchemaVO, error) {
	if uc.redisMetadataRepo == nil {
		return nil, fmt.Errorf("Redis 元数据仓储未配置")
	}
	items, err := uc.redisMetadataRepo.ListKeyspaces(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseSchemaVO, 0, len(items))
	for _, item := range items {
		list = append(list, &DatabaseSchemaVO{
			ID:           item.ID,
			InstanceID:   item.InstanceID,
			SchemaName:   item.KeyspaceName,
			TableCount:   int(item.KeyCount),
			ExpiresCount: item.ExpiringKeys,
			AvgTTLText:   formatRedisTTL(item.AvgTTLMillis),
			LastSyncAt:   formatTime(item.LastSyncAt),
		})
	}
	return list, nil
}

func (uc *UseCase) listRedisKeys(ctx context.Context, instanceID uint, schemaName string) ([]*DatabaseTableVO, error) {
	if uc.redisMetadataRepo == nil {
		return nil, fmt.Errorf("Redis 元数据仓储未配置")
	}
	dbIndex, err := parseRedisDBIndex(schemaName)
	if err != nil {
		return nil, fmt.Errorf("请先选择 Redis 逻辑 DB")
	}
	items, err := uc.redisMetadataRepo.ListKeys(ctx, instanceID, dbIndex)
	if err != nil {
		return nil, err
	}
	list := make([]*DatabaseTableVO, 0, len(items))
	for _, item := range items {
		list = append(list, &DatabaseTableVO{
			ID:            item.ID,
			InstanceID:    item.InstanceID,
			SchemaName:    item.KeyspaceName,
			TableName:     item.KeyName,
			TableType:     strings.ToUpper(item.KeyType),
			RowCount:      item.ValueSize,
			DataSizeBytes: item.MemoryUsageBytes,
			TTLMillis:     item.TTLMillis,
			TTLText:       formatRedisTTL(item.TTLMillis),
			Encoding:      item.Encoding,
			NodeAddress:   item.NodeAddress,
			Slot:          item.Slot,
			Preview:       item.PreviewText,
			Comment:       item.PreviewText,
			LastSyncAt:    formatTime(item.LastSyncAt),
		})
	}
	return list, nil
}

func (uc *UseCase) listRedisKeyDetails(ctx context.Context, instanceID uint, schemaName, tableName string) ([]*DatabaseColumnVO, error) {
	if uc.redisMetadataRepo == nil {
		return nil, fmt.Errorf("Redis 元数据仓储未配置")
	}
	dbIndex, err := parseRedisDBIndex(schemaName)
	if err != nil {
		return nil, err
	}
	item, err := uc.redisMetadataRepo.GetKey(ctx, instanceID, dbIndex, tableName)
	if err != nil {
		return nil, fmt.Errorf("Redis Key 不存在或请先同步元数据")
	}
	rows := []struct {
		name    string
		value   string
		comment string
	}{
		{name: "type", value: strings.ToUpper(item.KeyType), comment: "Redis Key 类型"},
		{name: "ttl", value: formatRedisTTL(item.TTLMillis), comment: "过期时间"},
		{name: "memory_usage", value: humanizeBytes(item.MemoryUsageBytes), comment: "MEMORY USAGE 估算"},
		{name: "length", value: strconv.FormatInt(item.ValueSize, 10), comment: "字符串长度或成员数量"},
		{name: "encoding", value: valueOrDefault(item.Encoding, "-"), comment: "底层编码"},
		{name: "slot", value: strconv.Itoa(item.Slot), comment: "Cluster slot"},
		{name: "node", value: valueOrDefault(item.NodeAddress, "-"), comment: "所属节点"},
		{name: "preview", value: valueOrDefault(item.PreviewText, "-"), comment: "值预览"},
	}
	list := make([]*DatabaseColumnVO, 0, len(rows))
	for index, row := range rows {
		list = append(list, &DatabaseColumnVO{
			InstanceID:      item.InstanceID,
			SchemaName:      item.KeyspaceName,
			TableName:       item.KeyName,
			ColumnName:      row.name,
			OrdinalPosition: index + 1,
			DataType:        "meta",
			DefaultValue:    row.value,
			Comment:         row.comment,
		})
	}
	return list, nil
}
