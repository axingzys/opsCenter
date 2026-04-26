package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type redisCapacityData struct {
	usedMemory        int64
	usedMemoryDataset int64
	maxMemory         int64
	keyspaces         map[int]*redisKeyspaceAggregate
	keys              []*DatabaseRedisKeySample
}

func (uc *UseCase) collectRedisCapacitySnapshot(ctx context.Context, instance *DatabaseInstance) (*DatabaseCapacityCollectVO, error) {
	if uc.capacitySnapshotRepo == nil {
		return nil, fmt.Errorf("容量快照仓库未配置")
	}
	credential, err := uc.resolveDiagnosableCredential(ctx, instance)
	if err != nil {
		return nil, err
	}
	data, err := collectRedisCapacityData(ctx, instance, credential)
	if err != nil {
		return nil, err
	}
	collectedAt := time.Now()
	snapshots := buildRedisCapacitySnapshots(instance, data, collectedAt)
	if err := uc.capacitySnapshotRepo.CreateBatch(ctx, snapshots); err != nil {
		return nil, fmt.Errorf("写入容量快照失败: %w", err)
	}
	instanceSnapshot := findInstanceCapacitySnapshot(snapshots)
	return &DatabaseCapacityCollectVO{
		InstanceID:     instance.ID,
		InstanceName:   instance.Name,
		SnapshotsCount: len(snapshots),
		SchemaCount:    instanceSnapshot.SchemaCount,
		TableCount:     instanceSnapshot.TableCount,
		TotalSizeBytes: instanceSnapshot.TotalSizeBytes,
		TotalSizeText:  humanizeBytes(instanceSnapshot.TotalSizeBytes),
		CollectedAt:    collectedAt.Format("2006-01-02 15:04:05"),
		Message:        "Redis 容量快照采集完成",
	}, nil
}

func collectRedisCapacityData(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*redisCapacityData, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer access.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if access.clusterEnabled {
		return collectRedisClusterCapacityData(queryCtx, item, credential, access)
	}
	return collectRedisStandaloneCapacityData(queryCtx, item, credential)
}

func collectRedisStandaloneCapacityData(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*redisCapacityData, error) {
	baseClient, err := openRedisClient(item, credential)
	if err != nil {
		return nil, err
	}
	defer baseClient.Close()

	infoText, err := baseClient.Info(ctx, "memory", "keyspace").Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis 容量指标失败: %w", err)
	}
	info := parseRedisInfo(infoText)
	keyspaces := parseRedisKeyspaceStats(info)
	if len(keyspaces) == 0 {
		defaultDB, resolveErr := resolveRedisDBIndex(item, "", false)
		if resolveErr != nil {
			return nil, resolveErr
		}
		keyspaces[defaultDB] = &redisKeyspaceAggregate{dbIndex: defaultDB}
	}

	keys := make([]*DatabaseRedisKeySample, 0)
	dbIndexes := make([]int, 0, len(keyspaces))
	for dbIndex := range keyspaces {
		dbIndexes = append(dbIndexes, dbIndex)
	}
	sort.Ints(dbIndexes)

	remaining := redisMetadataSampleKeyLimit
	address := resolveRedisStandaloneAddress(ctx, item, credential)
	for _, dbIndex := range dbIndexes {
		dbClient, err := openRedisClientWithDB(item, credential, dbIndex)
		if err != nil {
			return nil, err
		}
		samples := make([]*DatabaseRedisKeySample, 0)
		if remaining > 0 {
			samples, err = scanRedisKeySamples(ctx, dbClient, dbIndex, remaining, "", address, false)
			if err != nil {
				dbClient.Close()
				return nil, err
			}
		}
		dbClient.Close()
		keys = append(keys, samples...)
		remaining -= len(samples)
		ensureRedisKeyspaceStat(keyspaces, dbIndex).sampledKeys += len(samples)
	}
	return &redisCapacityData{
		usedMemory:        parseInt64Default(info["used_memory"], 0),
		usedMemoryDataset: normalizeRedisDatasetMemory(parseInt64Default(info["used_memory"], 0), parseInt64Default(info["used_memory_dataset"], 0)),
		maxMemory:         parseInt64Default(info["maxmemory"], 0),
		keyspaces:         keyspaces,
		keys:              keys,
	}, nil
}

func collectRedisClusterCapacityData(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess) (*redisCapacityData, error) {
	if access == nil || access.single == nil {
		return nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	keyspaces := make(map[int]*redisKeyspaceAggregate)
	keys := make([]*DatabaseRedisKeySample, 0)
	remaining := redisMetadataSampleKeyLimit
	usedMemory := int64(0)
	usedMemoryDataset := int64(0)
	maxMemory := int64(0)

	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(node.Role))
		nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return nil, err
		}
		infoText, infoErr := nodeClient.Info(ctx, "memory", "keyspace").Result()
		if infoErr != nil {
			nodeClient.Close()
			return nil, fmt.Errorf("读取 Redis 节点 %s 容量指标失败: %w", node.Address, infoErr)
		}
		info := parseRedisInfo(infoText)
		nodeUsedMemory := parseInt64Default(info["used_memory"], 0)
		usedMemory += nodeUsedMemory
		usedMemoryDataset += normalizeRedisDatasetMemory(nodeUsedMemory, parseInt64Default(info["used_memory_dataset"], 0))
		if currentMaxMemory := parseInt64Default(info["maxmemory"], 0); currentMaxMemory > 0 {
			maxMemory += currentMaxMemory
		}
		if role != "master" {
			nodeClient.Close()
			continue
		}
		mergeRedisKeyspaceStats(keyspaces, parseRedisKeyspaceStats(info))
		if remaining > 0 {
			samples, err := scanRedisKeySamples(ctx, nodeClient, 0, remaining, node.ID, node.Address, true)
			if err != nil {
				nodeClient.Close()
				return nil, err
			}
			keys = append(keys, samples...)
			remaining -= len(samples)
			ensureRedisKeyspaceStat(keyspaces, 0).sampledKeys += len(samples)
		}
		nodeClient.Close()
	}
	if len(keyspaces) == 0 {
		keyspaces[0] = &redisKeyspaceAggregate{dbIndex: 0}
	}
	return &redisCapacityData{
		usedMemory:        usedMemory,
		usedMemoryDataset: normalizeRedisDatasetMemory(usedMemory, usedMemoryDataset),
		maxMemory:         maxMemory,
		keyspaces:         keyspaces,
		keys:              keys,
	}, nil
}

func buildRedisCapacitySnapshots(instance *DatabaseInstance, data *redisCapacityData, collectedAt time.Time) []*DatabaseCapacitySnapshot {
	if data == nil {
		data = &redisCapacityData{keyspaces: map[int]*redisKeyspaceAggregate{}}
	}
	if data.keyspaces == nil {
		data.keyspaces = map[int]*redisKeyspaceAggregate{}
	}
	snapshots := make([]*DatabaseCapacitySnapshot, 0, len(data.keyspaces)+len(data.keys)+1)
	keyspaceSnapshots := make(map[string]*DatabaseCapacitySnapshot)
	dbIndexes := make([]int, 0, len(data.keyspaces))
	totalKeys := int64(0)
	for dbIndex := range data.keyspaces {
		dbIndexes = append(dbIndexes, dbIndex)
	}
	sort.Ints(dbIndexes)
	for _, dbIndex := range dbIndexes {
		stat := data.keyspaces[dbIndex]
		name := redisSchemaName(dbIndex)
		totalKeys += stat.keyCount
		snapshot := &DatabaseCapacitySnapshot{
			InstanceID:  instance.ID,
			ObjectType:  DatabaseCapacityObjectSchema,
			SchemaName:  name,
			TableCount:  int(stat.keyCount),
			RowCount:    stat.keyCount,
			CollectedAt: collectedAt,
		}
		keyspaceSnapshots[name] = snapshot
		snapshots = append(snapshots, snapshot)
	}
	for _, key := range data.keys {
		if key == nil {
			continue
		}
		schemaName := valueOrDefault(key.KeyspaceName, "db0")
		current := keyspaceSnapshots[schemaName]
		if current == nil {
			current = &DatabaseCapacitySnapshot{
				InstanceID:  instance.ID,
				ObjectType:  DatabaseCapacityObjectSchema,
				SchemaName:  schemaName,
				CollectedAt: collectedAt,
			}
			keyspaceSnapshots[schemaName] = current
			snapshots = append(snapshots, current)
		}
		current.DataSizeBytes += key.MemoryUsageBytes
		current.TotalSizeBytes = current.DataSizeBytes
		snapshots = append(snapshots, &DatabaseCapacitySnapshot{
			InstanceID:     instance.ID,
			ObjectType:     DatabaseCapacityObjectTable,
			SchemaName:     schemaName,
			Table:          trimText(key.KeyName, 150),
			RowCount:       key.ValueSize,
			DataSizeBytes:  key.MemoryUsageBytes,
			IndexSizeBytes: 0,
			TotalSizeBytes: key.MemoryUsageBytes,
			CollectedAt:    collectedAt,
		})
	}
	usedMemory := data.usedMemory
	datasetMemory := normalizeRedisDatasetMemory(usedMemory, data.usedMemoryDataset)
	overheadMemory := usedMemory - datasetMemory
	if overheadMemory < 0 {
		overheadMemory = 0
	}
	instanceSnapshot := &DatabaseCapacitySnapshot{
		InstanceID:     instance.ID,
		ObjectType:     DatabaseCapacityObjectInstance,
		SchemaCount:    len(keyspaceSnapshots),
		TableCount:     int(totalKeys),
		RowCount:       totalKeys,
		DataSizeBytes:  datasetMemory,
		IndexSizeBytes: overheadMemory,
		TotalSizeBytes: usedMemory,
		CollectedAt:    collectedAt,
	}
	return append([]*DatabaseCapacitySnapshot{instanceSnapshot}, snapshots...)
}

func normalizeRedisDatasetMemory(usedMemory, datasetMemory int64) int64 {
	switch {
	case usedMemory <= 0:
		return 0
	case datasetMemory <= 0:
		return usedMemory
	case datasetMemory > usedMemory:
		return usedMemory
	default:
		return datasetMemory
	}
}
