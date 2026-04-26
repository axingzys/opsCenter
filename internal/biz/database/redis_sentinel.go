package database

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisSentinelSettings struct {
	MasterName       string
	SentinelAddrs    []string
	Username         string
	Password         string
	SentinelUsername string
	SentinelPassword string
	DB               int
	TLSConfig        *tls.Config
}

func parseRedisSentinelSettings(item *DatabaseInstance, credential *ConnectionCredential) (*redisSentinelSettings, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}
	return parseRedisSentinelSettingsFromParams(item, credential, params)
}

func parseRedisSentinelSettingsFromParams(item *DatabaseInstance, credential *ConnectionCredential, params map[string]any) (*redisSentinelSettings, error) {
	masterName := connectionParamString(params, "masterName", "master_name", "sentinelMasterName", "sentinel_master_name")
	addrs := connectionParamStringList(params, "sentinelAddrs", "sentinel_addrs", "sentinels", "sentinelNodes", "sentinel_nodes")
	sentinelUsername := connectionParamString(params, "sentinelUsername", "sentinel_username")
	sentinelPassword := connectionParamString(params, "sentinelPassword", "sentinel_password")
	explicit := masterName != "" || len(addrs) > 0 || sentinelUsername != "" || sentinelPassword != ""
	if !explicit {
		return nil, nil
	}
	if masterName == "" {
		return nil, fmt.Errorf("Redis Sentinel 缺少 masterName")
	}
	seedAddr := net.JoinHostPort(strings.TrimSpace(item.Host), strconv.Itoa(item.Port))
	combinedAddrs := make([]string, 0, len(addrs)+1)
	seen := make(map[string]struct{})
	for _, addr := range append([]string{seedAddr}, addrs...) {
		text := strings.TrimSpace(addr)
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		combinedAddrs = append(combinedAddrs, text)
	}
	if len(combinedAddrs) == 0 {
		return nil, fmt.Errorf("Redis Sentinel 缺少 sentinelAddrs")
	}
	username := ""
	password := ""
	if credential != nil {
		username = strings.TrimSpace(credential.Username)
		password = credential.Password
	}
	if sentinelUsername == "" {
		sentinelUsername = username
	}
	if sentinelPassword == "" {
		sentinelPassword = password
	}
	settings := &redisSentinelSettings{
		MasterName:       masterName,
		SentinelAddrs:    combinedAddrs,
		Username:         username,
		Password:         password,
		SentinelUsername: strings.TrimSpace(sentinelUsername),
		SentinelPassword: sentinelPassword,
		DB:               connectionParamInt(params, 0, "db", "database"),
	}
	if item != nil && item.TLSEnabled {
		settings.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
	}
	return settings, nil
}

func openRedisSentinelClient(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*redis.SentinelClient, string, *redisSentinelSettings, error) {
	settings, err := parseRedisSentinelSettings(item, credential)
	if err != nil {
		return nil, "", nil, err
	}
	if settings == nil {
		return nil, "", nil, fmt.Errorf("Redis Sentinel 未配置")
	}
	var lastErr error
	for _, addr := range settings.SentinelAddrs {
		client := redis.NewSentinelClient(&redis.Options{
			Addr:         addr,
			Username:     settings.SentinelUsername,
			Password:     settings.SentinelPassword,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			TLSConfig:    settings.TLSConfig,
		})
		probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := client.Ping(probeCtx).Err()
		cancel()
		if err == nil {
			return client, addr, settings, nil
		}
		lastErr = err
		_ = client.Close()
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的 Sentinel 节点")
	}
	return nil, "", settings, fmt.Errorf("连接 Redis Sentinel 失败: %w", lastErr)
}

func collectRedisSentinelTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	sentinelClient, seedAddr, settings, err := openRedisSentinelClient(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer sentinelClient.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	masterInfo, err := sentinelClient.Master(queryCtx, settings.MasterName).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Sentinel master 信息失败: %w", err)
	}
	replicas, err := sentinelClient.Replicas(queryCtx, settings.MasterName).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Sentinel replicas 信息失败: %w", err)
	}
	sentinels, err := sentinelClient.Sentinels(queryCtx, settings.MasterName).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Sentinel sentinels 信息失败: %w", err)
	}

	nodes := make([]*DatabaseTopologyNodeVO, 0, 2+len(replicas)+len(sentinels))
	links := make([]*DatabaseTopologyLinkVO, 0, len(replicas)+len(sentinels)+1)
	now := time.Now().Format("2006-01-02 15:04:05")

	masterAddr := redisSentinelAddress(masterInfo)
	masterNodeID := "redis-master:" + masterAddr
	nodes = append(nodes, &DatabaseTopologyNodeVO{
		ID:        masterNodeID,
		Name:      valueOrDefault(settings.MasterName, masterAddr),
		Role:      "master",
		RoleText:  redisRoleText("master"),
		Address:   masterAddr,
		State:     redisSentinelState(masterInfo),
		Message:   valueOrDefault(masterInfo["flags"], "-"),
		UpdatedAt: now,
		Metrics: map[string]string{
			"quorum":              valueOrDefault(masterInfo["quorum"], "-"),
			"num-slaves":          valueOrDefault(masterInfo["num-slaves"], strconv.Itoa(len(replicas))),
			"num-other-sentinels": valueOrDefault(masterInfo["num-other-sentinels"], strconv.Itoa(len(sentinels))),
			"failover-timeout":    valueOrDefault(masterInfo["failover-timeout"], "-"),
			"down-after-millis":   valueOrDefault(masterInfo["down-after-milliseconds"], "-"),
			"parallel-syncs":      valueOrDefault(masterInfo["parallel-syncs"], "-"),
		},
	})

	for idx, replica := range replicas {
		address := redisSentinelAddress(replica)
		id := valueOrDefault(replica["runid"], fmt.Sprintf("replica-%d", idx))
		nodes = append(nodes, &DatabaseTopologyNodeVO{
			ID:        "redis-replica:" + id,
			Name:      shortNodeID(id),
			Role:      "replica",
			RoleText:  redisRoleText("replica"),
			Address:   address,
			State:     redisSentinelState(replica),
			Message:   valueOrDefault(replica["flags"], "-"),
			UpdatedAt: now,
			Metrics: map[string]string{
				"master-link-status": valueOrDefault(replica["master-link-status"], "-"),
				"slave-priority":     valueOrDefault(replica["slave-priority"], "-"),
			},
		})
		links = append(links, &DatabaseTopologyLinkVO{
			Source: "redis-replica:" + id,
			Target: masterNodeID,
			Label:  "replicates",
			State:  redisSentinelState(replica),
		})
	}

	sentinelNodeIDs := make(map[string]struct{})
	appendSentinelNode := func(info map[string]string, fallbackAddr string) {
		address := redisSentinelAddress(info)
		if address == "" {
			address = strings.TrimSpace(fallbackAddr)
		}
		id := valueOrDefault(info["runid"], address)
		nodeID := "redis-sentinel:" + id
		if _, ok := sentinelNodeIDs[nodeID]; ok {
			return
		}
		sentinelNodeIDs[nodeID] = struct{}{}
		nodes = append(nodes, &DatabaseTopologyNodeVO{
			ID:        nodeID,
			Name:      shortNodeID(id),
			Role:      "sentinel",
			RoleText:  redisRoleText("sentinel"),
			Address:   address,
			State:     redisSentinelState(info),
			Message:   valueOrDefault(info["flags"], "sentinel"),
			UpdatedAt: now,
			Metrics: map[string]string{
				"master-name": valueOrDefault(info["name"], settings.MasterName),
			},
		})
		links = append(links, &DatabaseTopologyLinkVO{
			Source: nodeID,
			Target: masterNodeID,
			Label:  "monitors",
			State:  redisSentinelState(info),
		})
	}
	appendSentinelNode(map[string]string{
		"name":  settings.MasterName,
		"runid": seedAddr,
	}, seedAddr)
	for _, sentinel := range sentinels {
		appendSentinelNode(sentinel, "")
	}

	sortTopologyNodes(nodes)
	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     "redis_sentinel",
		TopologyTypeText: "Redis Sentinel",
		CollectedAt:      now,
		Cards: []*DatabaseTopologyCardVO{
			{Key: "master_name", Label: "监控主库", Value: settings.MasterName, Description: "Sentinel 监控的主库名称"},
			{Key: "master_state", Label: "主库状态", Value: redisSentinelState(masterInfo), Description: "Sentinel 对主库的当前判定"},
			{Key: "replicas", Label: "从节点", Value: strconv.Itoa(len(replicas)), Description: "Sentinel 已发现的从节点数量"},
			{Key: "sentinels", Label: "哨兵节点", Value: strconv.Itoa(len(sentinelNodeIDs)), Description: "当前可见的 Sentinel 节点数量"},
			{Key: "quorum", Label: "法定票数", Value: valueOrDefault(masterInfo["quorum"], "-"), Description: "触发主观下线 / 故障转移需要的票数"},
		},
		Nodes:   nodes,
		Links:   links,
		Message: "Redis Sentinel 拓扑读取成功",
	}, nil
}

func resolveRedisStandaloneAddress(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) string {
	fallback := netJoinRedisAddress(item.Host, item.Port)
	settings, err := parseRedisSentinelSettings(item, credential)
	if err != nil || settings == nil {
		return fallback
	}
	sentinelClient, _, _, err := openRedisSentinelClient(ctx, item, credential)
	if err != nil {
		return fallback
	}
	defer sentinelClient.Close()
	values, err := sentinelClient.GetMasterAddrByName(ctx, settings.MasterName).Result()
	if err != nil || len(values) < 2 {
		return fallback
	}
	return net.JoinHostPort(strings.TrimSpace(values[0]), strings.TrimSpace(values[1]))
}

func redisSentinelAddress(info map[string]string) string {
	if info == nil {
		return ""
	}
	ip := strings.TrimSpace(info["ip"])
	port := strings.TrimSpace(info["port"])
	if ip != "" && port != "" {
		return net.JoinHostPort(ip, port)
	}
	name := strings.TrimSpace(info["name"])
	if name != "" && strings.Contains(name, ":") {
		return normalizeRedisClusterAddress(name)
	}
	return name
}

func redisSentinelState(info map[string]string) string {
	flags := strings.ToLower(strings.TrimSpace(info["flags"]))
	switch {
	case strings.Contains(flags, "s_down"):
		return "s_down"
	case strings.Contains(flags, "o_down"):
		return "o_down"
	case flags != "":
		return flags
	default:
		return "connected"
	}
}
