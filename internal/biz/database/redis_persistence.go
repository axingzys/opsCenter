package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisPersistenceNodeState struct {
	address              string
	role                 string
	aofEnabled           bool
	rdbEnabled           bool
	appendFsync          string
	appendFsyncNo        bool
	rdbSaveInProgress    bool
	aofRewriteInProgress bool
	rdbLastBgsaveStatus  string
	aofLastRewriteStatus string
	aofLastWriteStatus   string
	lastSaveAt           *time.Time
	staleRDBOnly         bool
}

type redisPersistenceAggregate struct {
	now                       time.Time
	cluster                   bool
	inspectedNodes            int64
	masterNodes               int64
	replicaNodes              int64
	aofEnabledNodes           int64
	rdbEnabledNodes           int64
	noPersistenceNodes        int64
	appendFsyncNoNodes        int64
	rdbSaveInProgressNodes    int64
	aofRewriteInProgressNodes int64
	rdbFailureNodes           int64
	aofRewriteFailureNodes    int64
	aofWriteFailureNodes      int64
	staleRDBOnlyNodes         int64
	oldestSuccessfulSaveAt    *time.Time
}

func collectRedisPersistenceAggregate(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*redisPersistenceAggregate, error) {
	access, err := openRedisAccess(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer access.Close()

	aggregate := &redisPersistenceAggregate{now: time.Now(), cluster: access.clusterEnabled}
	queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if access.clusterEnabled {
		if err := collectRedisClusterPersistence(queryCtx, item, credential, access, aggregate); err != nil {
			return nil, err
		}
	} else {
		if err := collectRedisStandalonePersistence(queryCtx, item, credential, aggregate); err != nil {
			return nil, err
		}
	}
	return aggregate, nil
}

func collectRedisStandalonePersistence(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, aggregate *redisPersistenceAggregate) error {
	client, err := openRedisClient(item, credential)
	if err != nil {
		return err
	}
	defer client.Close()

	infoText, err := client.Info(ctx, "persistence", "replication").Result()
	if err != nil {
		return fmt.Errorf("读取 Redis 持久化信息失败: %w", err)
	}
	info := parseRedisInfo(infoText)
	configs := readRedisPersistenceConfig(ctx, client)
	aggregate.addState(buildRedisPersistenceNodeState(
		netJoinRedisAddress(item.Host, item.Port),
		valueOrDefault(info["role"], "standalone"),
		info,
		configs,
		aggregate.now,
	))
	return nil
}

func collectRedisClusterPersistence(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, access *redisAccess, aggregate *redisPersistenceAggregate) error {
	if access == nil || access.single == nil {
		return fmt.Errorf("Redis Cluster 连接未初始化")
	}
	nodesText, err := access.single.ClusterNodes(ctx).Result()
	if err != nil {
		return fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, _, _ := parseRedisClusterNodes(nodesText)
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		client, err := openRedisNodeClientByAddr(node.Address, item, credential, 0)
		if err != nil {
			return err
		}
		infoText, infoErr := client.Info(ctx, "persistence", "replication").Result()
		if infoErr != nil {
			client.Close()
			return fmt.Errorf("读取 Redis 节点 %s 持久化信息失败: %w", node.Address, infoErr)
		}
		configs := readRedisPersistenceConfig(ctx, client)
		client.Close()
		aggregate.addState(buildRedisPersistenceNodeState(node.Address, node.Role, parseRedisInfo(infoText), configs, aggregate.now))
	}
	return nil
}

func readRedisPersistenceConfig(ctx context.Context, client redisConfigGetter) map[string]string {
	values := make(map[string]string)
	if client == nil {
		return values
	}
	for _, key := range []string{"appendonly", "appendfsync", "save"} {
		result, err := client.ConfigGet(ctx, key).Result()
		if err != nil {
			continue
		}
		if value, ok := result[key]; ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}

type redisConfigGetter interface {
	ConfigGet(ctx context.Context, parameter string) *redis.MapStringStringCmd
}

func buildRedisPersistenceNodeState(address, role string, info, configs map[string]string, now time.Time) *redisPersistenceNodeState {
	state := &redisPersistenceNodeState{
		address:              strings.TrimSpace(address),
		role:                 strings.ToLower(strings.TrimSpace(role)),
		appendFsync:          strings.ToLower(strings.TrimSpace(configs["appendfsync"])),
		rdbLastBgsaveStatus:  strings.ToLower(strings.TrimSpace(info["rdb_last_bgsave_status"])),
		aofLastRewriteStatus: strings.ToLower(strings.TrimSpace(info["aof_last_bgrewrite_status"])),
		aofLastWriteStatus:   strings.ToLower(strings.TrimSpace(info["aof_last_write_status"])),
	}
	if state.role == "" {
		state.role = strings.ToLower(strings.TrimSpace(info["role"]))
	}
	if state.role == "" {
		state.role = "standalone"
	}
	state.aofEnabled = strings.TrimSpace(info["aof_enabled"]) == "1" || strings.EqualFold(strings.TrimSpace(configs["appendonly"]), "yes")
	state.rdbEnabled = redisRDBPersistenceEnabled(configs["save"], info)
	state.appendFsyncNo = state.aofEnabled && state.appendFsync == "no"
	state.rdbSaveInProgress = parseInt64Default(info["rdb_bgsave_in_progress"], 0) == 1
	state.aofRewriteInProgress = parseInt64Default(info["aof_rewrite_in_progress"], 0) == 1
	if saveAt := parseInt64Default(info["rdb_last_save_time"], 0); saveAt > 0 {
		lastSaveAt := time.Unix(saveAt, 0)
		state.lastSaveAt = &lastSaveAt
	}
	if state.rdbEnabled && !state.aofEnabled {
		if state.lastSaveAt == nil || now.Sub(*state.lastSaveAt) > 24*time.Hour {
			state.staleRDBOnly = true
		}
	}
	return state
}

func redisRDBPersistenceEnabled(saveConfig string, info map[string]string) bool {
	if strings.TrimSpace(saveConfig) != "" {
		return true
	}
	return parseInt64Default(info["rdb_last_save_time"], 0) > 0 || parseInt64Default(info["rdb_saves"], 0) > 0
}

func (a *redisPersistenceAggregate) addState(state *redisPersistenceNodeState) {
	if a == nil || state == nil {
		return
	}
	a.inspectedNodes++
	switch state.role {
	case "master":
		a.masterNodes++
	case "replica", "slave":
		a.replicaNodes++
	}
	if state.aofEnabled {
		a.aofEnabledNodes++
	}
	if state.rdbEnabled {
		a.rdbEnabledNodes++
	}
	if !state.aofEnabled && !state.rdbEnabled {
		a.noPersistenceNodes++
	}
	if state.appendFsyncNo {
		a.appendFsyncNoNodes++
	}
	if state.rdbSaveInProgress {
		a.rdbSaveInProgressNodes++
	}
	if state.aofRewriteInProgress {
		a.aofRewriteInProgressNodes++
	}
	if state.rdbEnabled && state.rdbLastBgsaveStatus != "" && state.rdbLastBgsaveStatus != "ok" {
		a.rdbFailureNodes++
	}
	if state.aofEnabled && state.aofLastRewriteStatus != "" && state.aofLastRewriteStatus != "ok" {
		a.aofRewriteFailureNodes++
	}
	if state.aofEnabled && state.aofLastWriteStatus != "" && state.aofLastWriteStatus != "ok" {
		a.aofWriteFailureNodes++
	}
	if state.staleRDBOnly {
		a.staleRDBOnlyNodes++
	}
	if state.lastSaveAt != nil {
		if a.oldestSuccessfulSaveAt == nil || state.lastSaveAt.Before(*a.oldestSuccessfulSaveAt) {
			oldest := *state.lastSaveAt
			a.oldestSuccessfulSaveAt = &oldest
		}
	}
}

func (a *redisPersistenceAggregate) nodeScopeText() string {
	if a == nil {
		return "0 节点"
	}
	if a.cluster {
		return fmt.Sprintf("%d 节点（%d 主 %d 从）", a.inspectedNodes, a.masterNodes, a.replicaNodes)
	}
	return fmt.Sprintf("%d 节点", maxInt64(a.inspectedNodes, 1))
}

func (a *redisPersistenceAggregate) persistenceModeText() string {
	if a == nil || a.inspectedNodes == 0 {
		return "-"
	}
	parts := make([]string, 0, 3)
	if a.aofEnabledNodes > 0 {
		parts = append(parts, fmt.Sprintf("AOF %d", a.aofEnabledNodes))
	}
	if a.rdbEnabledNodes > 0 {
		parts = append(parts, fmt.Sprintf("RDB %d", a.rdbEnabledNodes))
	}
	if a.noPersistenceNodes > 0 {
		parts = append(parts, fmt.Sprintf("未启用 %d", a.noPersistenceNodes))
	}
	if len(parts) == 0 {
		return "未启用持久化"
	}
	return strings.Join(parts, " / ")
}

func (a *redisPersistenceAggregate) rdbSaveText() string {
	if a == nil {
		return "-"
	}
	if a.rdbEnabledNodes == 0 {
		return "未启用 RDB"
	}
	if a.oldestSuccessfulSaveAt == nil {
		return "暂无成功落盘记录"
	}
	return fmt.Sprintf("%s（距今 %s）", a.oldestSuccessfulSaveAt.Format("2006-01-02 15:04:05"), humanizeElapsed(a.now.Sub(*a.oldestSuccessfulSaveAt)))
}

func (a *redisPersistenceAggregate) aofStatusText() string {
	if a == nil {
		return "-"
	}
	if a.aofEnabledNodes == 0 {
		return "未启用 AOF"
	}
	parts := []string{fmt.Sprintf("%d 节点启用", a.aofEnabledNodes)}
	if a.aofRewriteInProgressNodes > 0 {
		parts = append(parts, fmt.Sprintf("重写中 %d", a.aofRewriteInProgressNodes))
	}
	if a.appendFsyncNoNodes > 0 {
		parts = append(parts, fmt.Sprintf("appendfsync=no %d", a.appendFsyncNoNodes))
	}
	if failures := a.aofRewriteFailureNodes + a.aofWriteFailureNodes; failures > 0 {
		parts = append(parts, fmt.Sprintf("异常 %d", failures))
	}
	return strings.Join(parts, " / ")
}

func redisPersistenceModeStatus(a *redisPersistenceAggregate) string {
	if a == nil {
		return "info"
	}
	if a.noPersistenceNodes > 0 {
		return "warning"
	}
	return "success"
}

func redisRDBSaveStatus(a *redisPersistenceAggregate) string {
	if a == nil || a.rdbEnabledNodes == 0 {
		return "info"
	}
	if a.rdbFailureNodes > 0 || a.staleRDBOnlyNodes > 0 {
		return "warning"
	}
	return "success"
}

func redisAOFStatus(a *redisPersistenceAggregate) string {
	if a == nil || a.aofEnabledNodes == 0 {
		return "info"
	}
	if a.aofRewriteFailureNodes > 0 || a.aofWriteFailureNodes > 0 || a.appendFsyncNoNodes > 0 {
		return "warning"
	}
	return "success"
}

func humanizeElapsed(duration time.Duration) string {
	if duration < 0 {
		duration = -duration
	}
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
		return fmt.Sprintf("%d ms", duration/time.Millisecond)
	}
}

func maxInt64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}
