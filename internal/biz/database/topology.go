package database

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type DatabaseTopologyCardVO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type DatabaseTopologyNodeVO struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Role      string            `json:"role"`
	RoleText  string            `json:"roleText"`
	Address   string            `json:"address"`
	State     string            `json:"state"`
	Version   string            `json:"version"`
	Slots     string            `json:"slots"`
	LagBytes  int64             `json:"lagBytes"`
	LagText   string            `json:"lagText"`
	Message   string            `json:"message"`
	Metrics   map[string]string `json:"metrics,omitempty"`
	UpdatedAt string            `json:"updatedAt"`
}

type DatabaseTopologyLinkVO struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
	State  string `json:"state"`
}

type DatabaseShardVO struct {
	Index      string `json:"index"`
	Shard      string `json:"shard"`
	Primary    bool   `json:"primary"`
	State      string `json:"state"`
	Node       string `json:"node"`
	Address    string `json:"address"`
	Docs       int64  `json:"docs"`
	StoreBytes int64  `json:"storeBytes"`
}

type DatabaseTopologyVO struct {
	InstanceID       uint                      `json:"instanceId"`
	InstanceName     string                    `json:"instanceName"`
	DBType           string                    `json:"dbType"`
	DBTypeText       string                    `json:"dbTypeText"`
	TopologyType     string                    `json:"topologyType"`
	TopologyTypeText string                    `json:"topologyTypeText"`
	CollectedAt      string                    `json:"collectedAt"`
	Cards            []*DatabaseTopologyCardVO `json:"cards"`
	Nodes            []*DatabaseTopologyNodeVO `json:"nodes"`
	Links            []*DatabaseTopologyLinkVO `json:"links"`
	Shards           []*DatabaseShardVO        `json:"shards"`
	Message          string                    `json:"message"`
}

func (uc *UseCase) GetTopology(ctx context.Context, instanceID uint, operator QueryOperator) (*DatabaseTopologyVO, error) {
	item, err := uc.getDiagnosableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	audit := uc.startTopologyAudit(ctx, item, operator)
	credential, err := uc.resolveDiagnosableCredential(ctx, item)
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, 0, err.Error())
		return nil, err
	}

	start := time.Now()
	result, err := collectDatabaseTopology(ctx, item, credential)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusFailed, 0, duration, err.Error())
		return nil, err
	}
	rowCount := len(result.Nodes) + len(result.Shards)
	uc.finishQueryAudit(ctx, audit, DatabaseQueryStatusSuccess, rowCount, duration, "")
	return result, nil
}

func (uc *UseCase) startTopologyAudit(ctx context.Context, item *DatabaseInstance, operator QueryOperator) *DatabaseQueryAudit {
	if uc.auditRepo == nil || item == nil {
		return nil
	}
	auditSQLText := "SHOW TOPOLOGY"
	audit := &DatabaseQueryAudit{
		InstanceID:     item.ID,
		OperatorID:     operator.ID,
		OperatorName:   trimText(operator.Username, 100),
		AuditAction:    DatabaseAuditActionTopologyView,
		SQLText:        auditSQLText,
		SQLFingerprint: sqlFingerprint(auditSQLText),
		SQLType:        "TOPOLOGY",
		RiskLevel:      DatabaseQueryRiskLow,
		Status:         DatabaseQueryStatusPending,
		ClientIP:       trimText(operator.ClientIP, 64),
	}
	if err := uc.auditRepo.Create(ctx, audit); err != nil {
		return nil
	}
	return audit
}

func collectDatabaseTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeRedis:
		return collectRedisTopology(ctx, item, credential)
	case DBTypeMongoDB:
		return collectMongoDBTopology(ctx, item, credential)
	case DBTypeElasticsearch, DBTypeOpenSearch:
		return collectSearchTopology(ctx, item, credential)
	default:
		return nil, fmt.Errorf("%s 拓扑视图将在四期后续批次接入", DBTypeText(item.DBType))
	}
}

func testSearchConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	var root map[string]any
	if err := searchGetJSON(ctx, item, credential, "/", &root); err != nil {
		return "", err
	}
	version := stringFromMap(root, "version.number")
	if version == "" {
		version = stringFromMap(root, "version.distribution")
	}
	if version == "" {
		return DBTypeText(item.DBType), nil
	}
	return DBTypeText(item.DBType) + " " + version, nil
}

func collectRedisTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	if settings, err := parseRedisSentinelSettings(item, credential); err != nil {
		return nil, err
	} else if settings != nil {
		return collectRedisSentinelTopology(ctx, item, credential)
	}
	client, err := openRedisClient(item, credential)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(queryCtx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	if clusterInfo, err := client.ClusterInfo(queryCtx).Result(); err == nil && strings.Contains(clusterInfo, "cluster_state:") {
		return collectRedisClusterTopology(queryCtx, client, item, clusterInfo)
	}
	return collectRedisStandaloneTopology(queryCtx, client, item)
}

func testRedisConnection(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (string, error) {
	client, err := openRedisClient(item, credential)
	if err != nil {
		return "", err
	}
	defer client.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(queryCtx).Err(); err != nil {
		return "", fmt.Errorf("连接 Redis 失败: %w", err)
	}

	info, err := client.Info(queryCtx, "server").Result()
	if err != nil {
		return "Redis", nil
	}
	serverInfo := parseRedisInfo(info)
	if version := strings.TrimSpace(serverInfo["redis_version"]); version != "" {
		return version, nil
	}
	return "Redis", nil
}

func collectRedisClusterTopology(ctx context.Context, client *redis.Client, item *DatabaseInstance, clusterInfo string) (*DatabaseTopologyVO, error) {
	info := parseRedisInfo(clusterInfo)
	nodesText, err := client.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis Cluster 节点失败: %w", err)
	}
	nodes, links, nodeByAddr := parseRedisClusterNodes(nodesText)
	slots, err := client.ClusterSlots(ctx).Result()
	if err == nil {
		applyRedisClusterSlots(nodes, nodeByAddr, slots)
	}
	sortTopologyNodes(nodes)

	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     "redis_cluster",
		TopologyTypeText: "Redis Cluster",
		CollectedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Cards: []*DatabaseTopologyCardVO{
			{Key: "cluster_state", Label: "集群状态", Value: valueOrDefault(info["cluster_state"], "-"), Description: "Redis Cluster 当前状态"},
			{Key: "known_nodes", Label: "已知节点", Value: valueOrDefault(info["cluster_known_nodes"], strconv.Itoa(len(nodes))), Description: "Cluster 已知节点数"},
			{Key: "slots_assigned", Label: "已分配 Slots", Value: valueOrDefault(info["cluster_slots_assigned"], "-"), Description: "已分配的 slot 数量"},
			{Key: "slots_ok", Label: "健康 Slots", Value: valueOrDefault(info["cluster_slots_ok"], "-"), Description: "状态正常的 slot 数量"},
		},
		Nodes:   nodes,
		Links:   links,
		Message: "Redis Cluster 拓扑读取成功",
	}, nil
}

func collectRedisStandaloneTopology(ctx context.Context, client *redis.Client, item *DatabaseInstance) (*DatabaseTopologyVO, error) {
	infoText, err := client.Info(ctx, "server", "replication").Result()
	if err != nil {
		return nil, fmt.Errorf("读取 Redis 信息失败: %w", err)
	}
	info := parseRedisInfo(infoText)
	role := valueOrDefault(info["role"], "standalone")
	version := info["redis_version"]
	address := net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
	node := &DatabaseTopologyNodeVO{
		ID:        address,
		Name:      address,
		Role:      role,
		RoleText:  redisRoleText(role),
		Address:   address,
		State:     "connected",
		Version:   version,
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Metrics: map[string]string{
			"connected_slaves": valueOrDefault(info["connected_slaves"], "0"),
			"master_host":      info["master_host"],
			"master_link":      info["master_link_status"],
		},
	}
	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     "redis_standalone",
		TopologyTypeText: "Redis 单机 / 主从",
		CollectedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Cards: []*DatabaseTopologyCardVO{
			{Key: "role", Label: "角色", Value: node.RoleText, Description: "Redis 当前实例角色"},
			{Key: "version", Label: "版本", Value: valueOrDefault(version, "-"), Description: "Redis 服务端版本"},
			{Key: "connected_slaves", Label: "从节点", Value: valueOrDefault(info["connected_slaves"], "0"), Description: "主从复制连接数"},
		},
		Nodes:   []*DatabaseTopologyNodeVO{node},
		Message: "当前 Redis 未开启 Cluster，已返回单机 / 主从信息",
	}, nil
}

func openRedisClient(item *DatabaseInstance, credential *ConnectionCredential) (*redis.Client, error) {
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
			DB:               settings.DB,
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
		DB:           connectionParamInt(params, 0, "db", "database"),
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if item.TLSEnabled {
		options.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, true, "insecureSkipVerify", "insecure_skip_verify")}
	}
	return redis.NewClient(options), nil
}

func parseRedisClusterNodes(nodesText string) ([]*DatabaseTopologyNodeVO, []*DatabaseTopologyLinkVO, map[string]*DatabaseTopologyNodeVO) {
	nodes := make([]*DatabaseTopologyNodeVO, 0)
	links := make([]*DatabaseTopologyLinkVO, 0)
	byID := make(map[string]*DatabaseTopologyNodeVO)
	byAddr := make(map[string]*DatabaseTopologyNodeVO)
	masters := make(map[string]string)
	now := time.Now().Format("2006-01-02 15:04:05")

	for _, line := range strings.Split(strings.TrimSpace(nodesText), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 8 {
			continue
		}
		id := fields[0]
		address := normalizeRedisClusterAddress(fields[1])
		flags := fields[2]
		role := redisClusterRole(flags)
		node := &DatabaseTopologyNodeVO{
			ID:        id,
			Name:      shortNodeID(id),
			Role:      role,
			RoleText:  redisRoleText(role),
			Address:   address,
			State:     fields[7],
			Slots:     strings.Join(fields[8:], " "),
			Message:   flags,
			UpdatedAt: now,
		}
		nodes = append(nodes, node)
		byID[id] = node
		byAddr[address] = node
		if role == "replica" && fields[3] != "-" {
			masters[id] = fields[3]
		}
	}

	for replicaID, masterID := range masters {
		replica := byID[replicaID]
		master := byID[masterID]
		if replica == nil || master == nil {
			continue
		}
		links = append(links, &DatabaseTopologyLinkVO{
			Source: replica.ID,
			Target: master.ID,
			Label:  "replicates",
			State:  replica.State,
		})
	}
	return nodes, links, byAddr
}

func applyRedisClusterSlots(nodes []*DatabaseTopologyNodeVO, nodeByAddr map[string]*DatabaseTopologyNodeVO, slots []redis.ClusterSlot) {
	if len(slots) == 0 {
		return
	}
	slotRanges := make(map[string][]string)
	for _, slot := range slots {
		if len(slot.Nodes) == 0 {
			continue
		}
		masterAddr := normalizeRedisClusterAddress(slot.Nodes[0].Addr)
		node := nodeByAddr[masterAddr]
		if node == nil {
			continue
		}
		slotRanges[node.ID] = append(slotRanges[node.ID], fmt.Sprintf("%d-%d", slot.Start, slot.End))
	}
	for _, node := range nodes {
		if ranges := slotRanges[node.ID]; len(ranges) > 0 {
			node.Slots = strings.Join(ranges, " ")
		}
	}
}

func collectMongoDBTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	client, err := openMongoClient(ctx, item, credential)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(queryCtx, readpref.PrimaryPreferred()); err != nil {
		return nil, fmt.Errorf("连接 MongoDB 失败: %w", err)
	}

	var status bson.M
	if err := client.Database("admin").RunCommand(queryCtx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&status); err == nil {
		return buildMongoReplicaSetTopology(item, status), nil
	}

	var hello bson.M
	if err := client.Database("admin").RunCommand(queryCtx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil {
		return nil, fmt.Errorf("读取 MongoDB 拓扑失败: %w", err)
	}
	return buildMongoStandaloneTopology(item, hello), nil
}

func openMongoClient(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*mongo.Client, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return nil, err
	}
	host := net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
	opts := options.Client().ApplyURI("mongodb://" + host)
	opts.SetConnectTimeout(5 * time.Second)
	if credential != nil && (strings.TrimSpace(credential.Username) != "" || credential.Password != "") {
		authSource := connectionParamString(params, "authSource", "auth_source")
		if authSource == "" {
			authSource = strings.TrimSpace(item.DefaultDatabase)
		}
		if authSource == "" {
			authSource = "admin"
		}
		opts.SetAuth(options.Credential{
			AuthSource: authSource,
			Username:   strings.TrimSpace(credential.Username),
			Password:   credential.Password,
		})
	}
	if replicaSet := connectionParamString(params, "replicaSet", "replica_set"); replicaSet != "" {
		opts.SetReplicaSet(replicaSet)
	}
	if item.TLSEnabled {
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: connectionParamBool(params, true, "insecureSkipVerify", "insecure_skip_verify")})
	}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("创建 MongoDB 连接失败: %w", err)
	}
	return client, nil
}

func buildMongoReplicaSetTopology(item *DatabaseInstance, status bson.M) *DatabaseTopologyVO {
	setName := anyToString(status["set"])
	members := anyToSlice(status["members"])
	nodes := make([]*DatabaseTopologyNodeVO, 0, len(members))
	links := make([]*DatabaseTopologyLinkVO, 0)
	var primaryID string
	now := time.Now()

	for _, member := range members {
		data, ok := member.(bson.M)
		if !ok {
			if asMap, ok := member.(map[string]any); ok {
				data = bson.M(asMap)
			} else {
				continue
			}
		}
		id := anyToString(data["_id"])
		name := anyToString(data["name"])
		role := strings.ToLower(anyToString(data["stateStr"]))
		if role == "" {
			role = strings.ToLower(anyToString(data["state"]))
		}
		optimeDate := anyToTime(data["optimeDate"])
		lagSeconds := int64(0)
		if !optimeDate.IsZero() {
			lagSeconds = int64(now.Sub(optimeDate).Seconds())
			if lagSeconds < 0 {
				lagSeconds = 0
			}
		}
		node := &DatabaseTopologyNodeVO{
			ID:        id,
			Name:      valueOrDefault(name, id),
			Role:      role,
			RoleText:  mongoRoleText(role),
			Address:   name,
			State:     mongoHealthText(data["health"]),
			LagBytes:  lagSeconds,
			LagText:   formatSeconds(lagSeconds),
			Message:   anyToString(data["lastHeartbeatMessage"]),
			UpdatedAt: now.Format("2006-01-02 15:04:05"),
		}
		nodes = append(nodes, node)
		if strings.EqualFold(role, "primary") {
			primaryID = id
		}
	}
	for _, node := range nodes {
		if primaryID != "" && node.ID != primaryID && node.Role != "arbiter" {
			links = append(links, &DatabaseTopologyLinkVO{
				Source: node.ID,
				Target: primaryID,
				Label:  "replicates",
				State:  node.State,
			})
		}
	}
	sortTopologyNodes(nodes)
	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     "mongodb_replicaset",
		TopologyTypeText: "MongoDB ReplicaSet",
		CollectedAt:      now.Format("2006-01-02 15:04:05"),
		Cards: []*DatabaseTopologyCardVO{
			{Key: "replica_set", Label: "副本集", Value: valueOrDefault(setName, "-"), Description: "ReplicaSet 名称"},
			{Key: "members", Label: "成员数", Value: strconv.Itoa(len(nodes)), Description: "副本集成员数量"},
			{Key: "primary", Label: "Primary", Value: valueOrDefault(primaryID, "-"), Description: "当前 PRIMARY 成员 ID"},
		},
		Nodes:   nodes,
		Links:   links,
		Message: "MongoDB ReplicaSet 拓扑读取成功",
	}
}

func buildMongoStandaloneTopology(item *DatabaseInstance, hello bson.M) *DatabaseTopologyVO {
	now := time.Now().Format("2006-01-02 15:04:05")
	address := net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
	role := "standalone"
	if anyToBool(hello["isWritablePrimary"]) {
		role = "primary"
	} else if anyToBool(hello["secondary"]) {
		role = "secondary"
	}
	node := &DatabaseTopologyNodeVO{
		ID:        address,
		Name:      address,
		Role:      role,
		RoleText:  mongoRoleText(role),
		Address:   address,
		State:     "ok",
		Version:   anyToString(hello["maxWireVersion"]),
		UpdatedAt: now,
	}
	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     "mongodb_standalone",
		TopologyTypeText: "MongoDB 单机",
		CollectedAt:      now,
		Cards: []*DatabaseTopologyCardVO{
			{Key: "role", Label: "角色", Value: node.RoleText, Description: "MongoDB 当前节点角色"},
			{Key: "set", Label: "副本集", Value: valueOrDefault(anyToString(hello["setName"]), "-"), Description: "为空表示未检测到副本集"},
		},
		Nodes:   []*DatabaseTopologyNodeVO{node},
		Message: "当前 MongoDB 未返回 ReplicaSet 状态，已返回 hello 基础信息",
	}
}

func collectSearchTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	var root map[string]any
	if err := searchGetJSON(ctx, item, credential, "/", &root); err != nil {
		return nil, err
	}
	var health map[string]any
	if err := searchGetJSON(ctx, item, credential, "/_cluster/health", &health); err != nil {
		return nil, err
	}

	nodes, nodesErr := collectSearchNodes(ctx, item, credential)
	shards, shardsErr := collectSearchShards(ctx, item, credential)
	message := "搜索集群拓扑读取成功"
	if nodesErr != nil || shardsErr != nil {
		var messages []string
		if nodesErr != nil {
			messages = append(messages, "节点读取失败: "+nodesErr.Error())
		}
		if shardsErr != nil {
			messages = append(messages, "分片读取失败: "+shardsErr.Error())
		}
		message = strings.Join(messages, "；")
	}
	version := stringFromMap(root, "version.number")
	status := strings.ToLower(anyToString(health["status"]))
	topologyType := normalizeDBType(item.DBType) + "_cluster"
	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     topologyType,
		TopologyTypeText: DBTypeText(item.DBType) + " Cluster",
		CollectedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Cards: []*DatabaseTopologyCardVO{
			{Key: "cluster_name", Label: "集群", Value: valueOrDefault(anyToString(root["cluster_name"]), "-"), Description: "集群名称"},
			{Key: "status", Label: "健康状态", Value: valueOrDefault(status, "-"), Description: "Cluster health status"},
			{Key: "nodes", Label: "节点数", Value: valueOrDefault(anyToString(health["number_of_nodes"]), strconv.Itoa(len(nodes))), Description: "集群节点数量"},
			{Key: "shards", Label: "分片数", Value: valueOrDefault(anyToString(health["active_shards"]), strconv.Itoa(len(shards))), Description: "活跃分片数量"},
			{Key: "version", Label: "版本", Value: valueOrDefault(version, "-"), Description: "服务端版本"},
		},
		Nodes:   nodes,
		Shards:  shards,
		Message: message,
	}, nil
}

func collectSearchNodes(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseTopologyNodeVO, error) {
	var rows []map[string]any
	err := searchGetJSON(ctx, item, credential, "/_cat/nodes?format=json&h=id,name,ip,role,master,heap.percent,ram.percent,cpu,load_1m,node.role,version", &rows)
	if err != nil {
		return nil, err
	}
	nodes := make([]*DatabaseTopologyNodeVO, 0, len(rows))
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, row := range rows {
		id := anyToString(row["id"])
		name := anyToString(row["name"])
		role := anyToString(row["node.role"])
		if role == "" {
			role = anyToString(row["role"])
		}
		nodes = append(nodes, &DatabaseTopologyNodeVO{
			ID:        valueOrDefault(id, name),
			Name:      valueOrDefault(name, id),
			Role:      role,
			RoleText:  searchRoleText(role, anyToString(row["master"])),
			Address:   anyToString(row["ip"]),
			State:     "online",
			Version:   anyToString(row["version"]),
			UpdatedAt: now,
			Metrics: map[string]string{
				"heap_percent": anyToString(row["heap.percent"]),
				"ram_percent":  anyToString(row["ram.percent"]),
				"cpu":          anyToString(row["cpu"]),
				"load_1m":      anyToString(row["load_1m"]),
				"master":       anyToString(row["master"]),
			},
		})
	}
	sortTopologyNodes(nodes)
	return nodes, nil
}

func collectSearchShards(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) ([]*DatabaseShardVO, error) {
	var rows []map[string]any
	err := searchGetJSON(ctx, item, credential, "/_cat/shards?format=json&bytes=b&h=index,shard,prirep,state,docs,store,node,ip", &rows)
	if err != nil {
		return nil, err
	}
	shards := make([]*DatabaseShardVO, 0, len(rows))
	for _, row := range rows {
		shards = append(shards, &DatabaseShardVO{
			Index:      anyToString(row["index"]),
			Shard:      anyToString(row["shard"]),
			Primary:    strings.EqualFold(anyToString(row["prirep"]), "p"),
			State:      anyToString(row["state"]),
			Node:       anyToString(row["node"]),
			Address:    anyToString(row["ip"]),
			Docs:       anyToInt64(row["docs"]),
			StoreBytes: anyToInt64(row["store"]),
		})
		if len(shards) >= 500 {
			break
		}
	}
	return shards, nil
}

func searchGetJSON(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, path string, target any) error {
	endpoint, insecureSkipVerify, err := searchEndpoint(item)
	if err != nil {
		return err
	}
	requestURL := endpoint + path
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(queryCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("创建搜索集群请求失败: %w", err)
	}
	if credential != nil && strings.TrimSpace(credential.Username) != "" {
		req.SetBasicAuth(strings.TrimSpace(credential.Username), credential.Password)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	if strings.HasPrefix(endpoint, "https://") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接搜索集群失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return fmt.Errorf("读取搜索集群响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("搜索集群返回 HTTP %d: %s", resp.StatusCode, trimText(string(body), 200))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("解析搜索集群响应失败: %w", err)
	}
	return nil
}

func searchEndpoint(item *DatabaseInstance) (string, bool, error) {
	params, err := parseConnectionParams(item)
	if err != nil {
		return "", true, err
	}
	if rawURL := connectionParamString(params, "url", "endpoint", "baseUrl", "base_url"); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", true, fmt.Errorf("搜索集群 URL 格式错误")
		}
		return strings.TrimRight(rawURL, "/"), connectionParamBool(params, true, "insecureSkipVerify", "insecure_skip_verify"), nil
	}
	scheme := connectionParamString(params, "scheme", "protocol")
	if scheme == "" {
		if item.TLSEnabled {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if scheme != "http" && scheme != "https" {
		return "", true, fmt.Errorf("搜索集群协议仅支持 http 或 https")
	}
	return scheme + "://" + net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port)), connectionParamBool(params, true, "insecureSkipVerify", "insecure_skip_verify"), nil
}

func parseRedisInfo(infoText string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(infoText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func redisClusterRole(flags string) string {
	switch {
	case strings.Contains(flags, "master"):
		return "master"
	case strings.Contains(flags, "slave"):
		return "replica"
	default:
		return "unknown"
	}
}

func redisRoleText(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "master":
		return "主节点"
	case "replica", "slave":
		return "从节点"
	case "sentinel":
		return "哨兵"
	case "standalone":
		return "单机"
	default:
		return valueOrDefault(role, "未知")
	}
}

func mongoRoleText(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "primary":
		return "PRIMARY"
	case "secondary":
		return "SECONDARY"
	case "arbiter":
		return "ARBITER"
	case "startup":
		return "STARTUP"
	case "recovering":
		return "RECOVERING"
	case "standalone":
		return "单机"
	default:
		return valueOrDefault(strings.ToUpper(role), "未知")
	}
}

func searchRoleText(role, master string) string {
	var parts []string
	if strings.Contains(role, "m") {
		parts = append(parts, "master-eligible")
	}
	if strings.Contains(role, "d") {
		parts = append(parts, "data")
	}
	if strings.Contains(role, "i") {
		parts = append(parts, "ingest")
	}
	if master == "*" {
		parts = append(parts, "current-master")
	}
	if len(parts) == 0 {
		return valueOrDefault(role, "-")
	}
	return strings.Join(parts, ", ")
}

func normalizeRedisClusterAddress(address string) string {
	address = strings.Split(address, "@")[0]
	address = strings.Split(address, ",")[0]
	return strings.TrimSpace(address)
}

func sortTopologyNodes(nodes []*DatabaseTopologyNodeVO) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Role == nodes[j].Role {
			return nodes[i].Address < nodes[j].Address
		}
		return nodes[i].Role < nodes[j].Role
	})
}

func shortNodeID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func anyToString(value any) string {
	switch current := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(current)
	case fmt.Stringer:
		return strings.TrimSpace(current.String())
	case int:
		return strconv.Itoa(current)
	case int32:
		return strconv.FormatInt(int64(current), 10)
	case int64:
		return strconv.FormatInt(current, 10)
	case float64:
		if current == float64(int64(current)) {
			return strconv.FormatInt(int64(current), 10)
		}
		return strconv.FormatFloat(current, 'f', 2, 64)
	case bool:
		if current {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", current))
	}
}

func anyToInt64(value any) int64 {
	switch current := value.(type) {
	case int:
		return int64(current)
	case int32:
		return int64(current)
	case int64:
		return current
	case float64:
		return int64(current)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
		return parsed
	default:
		return 0
	}
}

func anyToBool(value any) bool {
	switch current := value.(type) {
	case bool:
		return current
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(current))
		return parsed
	case int, int32, int64, float64:
		return anyToInt64(current) != 0
	default:
		return false
	}
}

func anyToSlice(value any) []any {
	switch current := value.(type) {
	case primitive.A:
		return []any(current)
	case []any:
		return current
	default:
		return nil
	}
}

func anyToTime(value any) time.Time {
	switch current := value.(type) {
	case primitive.DateTime:
		return current.Time()
	case time.Time:
		return current
	default:
		return time.Time{}
	}
}

func mongoHealthText(value any) string {
	if anyToInt64(value) == 1 {
		return "healthy"
	}
	return "unhealthy"
}

func formatSeconds(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
}

func stringFromMap(data map[string]any, path string) string {
	var current any = data
	for _, part := range strings.Split(path, ".") {
		asMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = asMap[part]
	}
	return anyToString(current)
}
