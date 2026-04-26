package database

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisMetricsAggregate struct {
	clusterState        string
	role                string
	clusterSlotsOK      int64
	nodeCount           int64
	masterCount         int64
	replicaCount        int64
	connectedClients    int64
	maxClients          int64
	blockedClients      int64
	usedMemory          int64
	usedMemoryPeak      int64
	usedMemoryDataset   int64
	maxMemory           int64
	opsPerSec           int64
	keyspaceHits        int64
	keyspaceMisses      int64
	evictedKeys         int64
	expiredKeys         int64
	rejectedConnections int64
	maxReplicaLagSec    int64
	memFragmentationMax float64
	memFragmentationSum float64
	memFragmentationN   int64
}

func collectRedisMetrics(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseMetricCardVO, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer access.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if access.clusterEnabled {
		return collectRedisClusterMetrics(queryCtx, item, credential, access)
	}
	return collectRedisStandaloneMetrics(queryCtx, access.single)
}

func collectRedisStandaloneMetrics(ctx context.Context, client *redis.Client) ([]*DatabaseMetricCardVO, error) {
	infoText, err := client.Info(ctx, "server", "clients", "memory", "stats", "replication").Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis 指标失败: %w", err)
	}
	info := parseRedisInfo(infoText)
	aggregate := &redisMetricsAggregate{}
	aggregate.addNode(info, strings.TrimSpace(info["role"]))
	return buildRedisMetricCards(aggregate, false), nil
}

func collectRedisClusterMetrics(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess) ([]*DatabaseMetricCardVO, error) {
	if access == nil || access.single == nil {
		return nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	clusterInfoText, err := access.single.ClusterInfo(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 指标失败: %w", err)
	}
	clusterInfo := parseRedisInfo(clusterInfoText)
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	aggregate := &redisMetricsAggregate{
		clusterState:   strings.TrimSpace(clusterInfo["cluster_state"]),
		clusterSlotsOK: parseInt64Default(clusterInfo["cluster_slots_ok"], 0),
	}
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return nil, err
		}
		infoText, infoErr := nodeClient.Info(ctx, "server", "clients", "memory", "stats", "replication").Result()
		nodeClient.Close()
		if infoErr != nil {
			return nil, fmt.Errorf("读取 Redis 节点 %s 指标失败: %w", node.Address, infoErr)
		}
		aggregate.addNode(parseRedisInfo(infoText), node.Role)
	}
	return buildRedisMetricCards(aggregate, true), nil
}

func (a *redisMetricsAggregate) addNode(info map[string]string, role string) {
	if a == nil {
		return
	}
	a.nodeCount++
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	if a.role == "" && normalizedRole != "" {
		a.role = normalizedRole
	}
	switch normalizedRole {
	case "master":
		a.masterCount++
	case "replica", "slave":
		a.replicaCount++
	}
	a.connectedClients += parseInt64Default(info["connected_clients"], 0)
	a.maxClients += parseInt64Default(info["maxclients"], 0)
	a.blockedClients += parseInt64Default(info["blocked_clients"], 0)
	a.usedMemory += parseInt64Default(info["used_memory"], 0)
	a.usedMemoryPeak += parseInt64Default(info["used_memory_peak"], 0)
	a.usedMemoryDataset += parseInt64Default(info["used_memory_dataset"], 0)
	a.maxMemory += parseInt64Default(info["maxmemory"], 0)
	a.opsPerSec += parseInt64Default(info["instantaneous_ops_per_sec"], 0)
	a.keyspaceHits += parseInt64Default(info["keyspace_hits"], 0)
	a.keyspaceMisses += parseInt64Default(info["keyspace_misses"], 0)
	a.evictedKeys += parseInt64Default(info["evicted_keys"], 0)
	a.expiredKeys += parseInt64Default(info["expired_keys"], 0)
	a.rejectedConnections += parseInt64Default(info["rejected_connections"], 0)
	fragmentation := parseRedisFloatDefault(info["mem_fragmentation_ratio"], 0)
	if fragmentation > 0 {
		a.memFragmentationSum += fragmentation
		a.memFragmentationN++
		if fragmentation > a.memFragmentationMax {
			a.memFragmentationMax = fragmentation
		}
	}
	if normalizedRole == "replica" || normalizedRole == "slave" {
		lagSec := parseInt64Default(info["master_last_io_seconds_ago"], 0)
		if linkDown := parseInt64Default(info["master_link_down_since_seconds"], 0); linkDown > lagSec {
			lagSec = linkDown
		}
		if lagSec > a.maxReplicaLagSec {
			a.maxReplicaLagSec = lagSec
		}
	}
}

func buildRedisMetricCards(aggregate *redisMetricsAggregate, cluster bool) []*DatabaseMetricCardVO {
	if aggregate == nil {
		aggregate = &redisMetricsAggregate{}
	}
	hitRate := redisHitRate(aggregate.keyspaceHits, aggregate.keyspaceMisses)
	fragmentation := aggregate.memFragmentationMax
	if fragmentation <= 0 && aggregate.memFragmentationN > 0 {
		fragmentation = aggregate.memFragmentationSum / float64(aggregate.memFragmentationN)
	}
	memoryPressure := redisMemoryPressure(aggregate.usedMemory, aggregate.maxMemory)
	clientsValue := fmt.Sprintf("%d", aggregate.connectedClients)
	if aggregate.maxClients > 0 {
		clientsValue = fmt.Sprintf("%d / %d", aggregate.connectedClients, aggregate.maxClients)
	}
	roleCard := &DatabaseMetricCardVO{
		Key:         "redis_role_state",
		Label:       "实例角色",
		Value:       redisRoleText(valueOrDefault(aggregate.role, "standalone")),
		Description: "非 Cluster 模式下读取当前节点角色和运行指标",
	}
	if cluster {
		roleCard = &DatabaseMetricCardVO{
			Key:   "redis_cluster_state",
			Label: "集群状态",
			Value: redisClusterStateText(aggregate.clusterState),
			Description: fmt.Sprintf(
				"%d 节点，%d 主 %d 从，slots ok %d",
				aggregate.nodeCount,
				aggregate.masterCount,
				aggregate.replicaCount,
				aggregate.clusterSlotsOK,
			),
		}
	}
	return []*DatabaseMetricCardVO{
		roleCard,
		{
			Key:         "redis_clients",
			Label:       "连接客户端",
			Value:       clientsValue,
			Description: "connected_clients / maxclients",
		},
		{
			Key:         "redis_blocked_clients",
			Label:       "阻塞客户端",
			Value:       fmt.Sprintf("%d", aggregate.blockedClients),
			Description: "BLPOP / XREAD 等阻塞中的客户端数量",
		},
		{
			Key:         "redis_used_memory",
			Label:       "已用内存",
			Value:       humanizeBytes(aggregate.usedMemory),
			Description: fmt.Sprintf("峰值 %s，数据集 %s", humanizeBytes(aggregate.usedMemoryPeak), humanizeBytes(aggregate.usedMemoryDataset)),
		},
		{
			Key:         "redis_memory_pressure",
			Label:       "内存压力",
			Value:       memoryPressure,
			Description: "used_memory / maxmemory",
		},
		{
			Key:         "redis_ops",
			Label:       "实时 Ops/s",
			Value:       fmt.Sprintf("%d", aggregate.opsPerSec),
			Description: "instantaneous_ops_per_sec",
		},
		{
			Key:         "redis_hit_rate",
			Label:       "缓存命中率",
			Value:       fmt.Sprintf("%.2f%%", hitRate),
			Description: fmt.Sprintf("hits %d / misses %d", aggregate.keyspaceHits, aggregate.keyspaceMisses),
		},
		{
			Key:         "redis_fragmentation",
			Label:       "内存碎片率",
			Value:       fmt.Sprintf("%.2f", fragmentation),
			Description: "mem_fragmentation_ratio",
		},
		{
			Key:         "redis_replica_lag",
			Label:       "最大复制延迟",
			Value:       fmt.Sprintf("%ds", aggregate.maxReplicaLagSec),
			Description: "replica master_last_io_seconds_ago 最大值",
		},
		{
			Key:         "redis_evicted_rejected",
			Label:       "驱逐 / 拒绝",
			Value:       fmt.Sprintf("%d / %d", aggregate.evictedKeys, aggregate.rejectedConnections),
			Description: fmt.Sprintf("累计过期键 %d", aggregate.expiredKeys),
		},
	}
}

func collectRedisSessions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSessionVO, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer access.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if access.clusterEnabled {
		return collectRedisClusterSessions(queryCtx, item, credential, access, limit)
	}
	return collectRedisStandaloneSessions(queryCtx, access.single, resolveRedisStandaloneAddress(queryCtx, item, credential), limit)
}

func collectRedisStandaloneSessions(ctx context.Context, client *redis.Client, nodeAddress string, limit int) ([]*DatabaseSessionVO, error) {
	text, err := client.ClientList(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis CLIENT LIST 失败: %w", err)
	}
	items := parseRedisClientList(text, nodeAddress)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func collectRedisClusterSessions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess, limit int) ([]*DatabaseSessionVO, error) {
	if access == nil || access.single == nil {
		return nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	items := make([]*DatabaseSessionVO, 0)
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		nodeClient, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return nil, err
		}
		text, clientErr := nodeClient.ClientList(ctx).Result()
		nodeClient.Close()
		if clientErr != nil {
			return nil, fmt.Errorf("读取 Redis 节点 %s 会话失败: %w", node.Address, clientErr)
		}
		items = append(items, parseRedisClientList(text, node.Address)...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DurationSeconds == items[j].DurationSeconds {
			return items[i].IdleSeconds < items[j].IdleSeconds
		}
		return items[i].DurationSeconds > items[j].DurationSeconds
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func parseRedisClientList(clientListText, nodeAddress string) []*DatabaseSessionVO {
	items := make([]*DatabaseSessionVO, 0)
	for _, line := range strings.Split(clientListText, "\n") {
		fields := parseRedisClientListLine(line)
		if len(fields) == 0 {
			continue
		}
		idleSeconds := parseInt64Default(fields["idle"], 0)
		databaseName := "-"
		if dbIndex := parseInt64Default(fields["db"], -1); dbIndex >= 0 {
			databaseName = redisSchemaName(int(dbIndex))
		}
		command := strings.TrimSpace(fields["cmd"])
		command = strings.ReplaceAll(command, "|", " ")
		if strings.EqualFold(command, "NULL") {
			command = ""
		}
		items = append(items, &DatabaseSessionVO{
			SessionID:       valueOrDefault(fields["id"], "-"),
			DatabaseName:    databaseName,
			User:            valueOrDefault(fields["user"], "-"),
			Command:         valueOrDefault(command, "-"),
			State:           redisClientState(fields),
			WaitEvent:       redisClientWaitEvent(fields),
			ClientAddr:      valueOrDefault(fields["addr"], "-"),
			NodeAddress:     strings.TrimSpace(nodeAddress),
			IdleSeconds:     idleSeconds,
			DurationSeconds: parseInt64Default(fields["age"], 0),
			SQLText:         trimText(redisClientDetail(fields), 2000),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DurationSeconds == items[j].DurationSeconds {
			return items[i].IdleSeconds < items[j].IdleSeconds
		}
		return items[i].DurationSeconds > items[j].DurationSeconds
	})
	return items
}

func parseRedisClientListLine(line string) map[string]string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	fields := make(map[string]string)
	for _, token := range strings.Fields(line) {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		fields[parts[0]] = parts[1]
	}
	return fields
}

func redisClientState(fields map[string]string) string {
	flags := strings.ToLower(strings.TrimSpace(fields["flags"]))
	command := strings.ToLower(strings.TrimSpace(fields["cmd"]))
	idleSeconds := parseInt64Default(fields["idle"], 0)
	switch {
	case strings.Contains(flags, "b"):
		return "阻塞"
	case strings.Contains(flags, "x"):
		return "事务中"
	case strings.Contains(flags, "p"):
		return "PubSub"
	case strings.Contains(flags, "s"):
		return "副本链路"
	case strings.Contains(flags, "o"):
		return "监视器"
	}
	if command == "null" && idleSeconds >= 5 {
		return "空闲"
	}
	if idleSeconds >= 300 {
		return "空闲"
	}
	if command != "" && command != "null" {
		return "活跃"
	}
	return "正常"
}

func redisClientWaitEvent(fields map[string]string) string {
	parts := make([]string, 0, 2)
	if idleSeconds := parseInt64Default(fields["idle"], 0); idleSeconds > 0 {
		parts = append(parts, fmt.Sprintf("idle %ds", idleSeconds))
	}
	if flags := strings.TrimSpace(fields["flags"]); flags != "" {
		parts = append(parts, "flags="+flags)
	}
	return strings.Join(parts, " / ")
}

func redisClientDetail(fields map[string]string) string {
	parts := make([]string, 0, 4)
	if name := strings.TrimSpace(fields["name"]); name != "" {
		parts = append(parts, "name="+name)
	}
	if libName := strings.TrimSpace(fields["lib-name"]); libName != "" {
		parts = append(parts, "lib="+libName)
	}
	if libVer := strings.TrimSpace(fields["lib-ver"]); libVer != "" {
		parts = append(parts, "ver="+libVer)
	}
	if resp := strings.TrimSpace(fields["resp"]); resp != "" {
		parts = append(parts, "resp="+resp)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func collectRedisSlowQueries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, limit int) ([]*DatabaseSlowQueryVO, string, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, "", err
	}
	defer access.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if access.clusterEnabled {
		items, collectErr := collectRedisClusterSlowQueries(queryCtx, item, credential, access, limit)
		return items, "已聚合各主节点的 Redis 慢日志，时长单位为 ms。", collectErr
	}
	dbIndex, err := resolveRedisDBIndex(item, "", false)
	if err != nil {
		return nil, "", err
	}
	items, collectErr := collectRedisStandaloneSlowQueries(queryCtx, access.single, redisSchemaName(dbIndex), resolveRedisStandaloneAddress(queryCtx, item, credential), limit)
	return items, "Redis 慢日志来自实例内存队列，执行 SLOWLOG RESET 或实例重启后历史会清空。", collectErr
}

func collectRedisStandaloneSlowQueries(ctx context.Context, client *redis.Client, schemaName, nodeAddress string, limit int) ([]*DatabaseSlowQueryVO, error) {
	logs, err := client.SlowLogGet(ctx, int64(limit)).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis SLOWLOG 失败: %w", err)
	}
	return redisSlowLogsToVO(logs, schemaName, nodeAddress), nil
}

func collectRedisClusterSlowQueries(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess, limit int) ([]*DatabaseSlowQueryVO, error) {
	if access == nil || access.single == nil {
		return nil, fmt.Errorf("Redis Cluster 连接未初始化")
	}
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	items := make([]*DatabaseSlowQueryVO, 0)
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
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
		logs, slowErr := nodeClient.SlowLogGet(ctx, int64(limit)).Result()
		nodeClient.Close()
		if slowErr != nil {
			return nil, fmt.Errorf("读取 Redis 节点 %s 慢日志失败: %w", node.Address, slowErr)
		}
		items = append(items, redisSlowLogsToVO(logs, "db0", node.Address)...)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastSeen > items[j].LastSeen
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func redisSlowLogsToVO(logs []redis.SlowLog, schemaName, nodeAddress string) []*DatabaseSlowQueryVO {
	items := make([]*DatabaseSlowQueryVO, 0, len(logs))
	for _, item := range logs {
		commandText := formatRedisCommandTokens(item.Args)
		durationMs := float64(item.Duration.Microseconds()) / 1000
		items = append(items, &DatabaseSlowQueryVO{
			SchemaName:    valueOrDefault(schemaName, "-"),
			SQLText:       trimText(commandText, 2000),
			SQLSummary:    trimText(commandText, 200),
			AvgDurationMs: durationMs,
			MaxDurationMs: durationMs,
			ExecCount:     item.ID,
			RowsMetric:    int64(len(item.Args)),
			NodeAddress:   strings.TrimSpace(nodeAddress),
			LastSeen:      item.Time.Format("2006-01-02 15:04:05"),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastSeen > items[j].LastSeen
	})
	return items
}

func redisHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total <= 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

func redisMemoryPressure(usedMemory, maxMemory int64) string {
	if maxMemory <= 0 {
		return "未限制"
	}
	return fmt.Sprintf("%.2f%%", float64(usedMemory)/float64(maxMemory)*100)
}

func redisClusterStateText(state string) string {
	if strings.EqualFold(strings.TrimSpace(state), "ok") {
		return "正常"
	}
	if strings.TrimSpace(state) == "" {
		return "未知"
	}
	return strings.TrimSpace(state)
}

func parseRedisFloatDefault(value string, defaultValue float64) float64 {
	text := strings.TrimSpace(value)
	if text == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}
