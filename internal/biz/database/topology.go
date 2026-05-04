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
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	SourceName string            `json:"sourceName,omitempty"`
	TargetName string            `json:"targetName,omitempty"`
	Label      string            `json:"label"`
	State      string            `json:"state"`
	LagText    string            `json:"lagText,omitempty"`
	Message    string            `json:"message,omitempty"`
	Metrics    map[string]string `json:"metrics,omitempty"`
}

type DatabaseTopologyFindingVO struct {
	Level       string `json:"level"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	NodeID      string `json:"nodeId,omitempty"`
	LinkID      string `json:"linkId,omitempty"`
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
	InstanceID       uint                         `json:"instanceId"`
	InstanceName     string                       `json:"instanceName"`
	DBType           string                       `json:"dbType"`
	DBTypeText       string                       `json:"dbTypeText"`
	TopologyType     string                       `json:"topologyType"`
	TopologyTypeText string                       `json:"topologyTypeText"`
	CollectedAt      string                       `json:"collectedAt"`
	Cards            []*DatabaseTopologyCardVO    `json:"cards"`
	Nodes            []*DatabaseTopologyNodeVO    `json:"nodes"`
	Links            []*DatabaseTopologyLinkVO    `json:"links"`
	Shards           []*DatabaseShardVO           `json:"shards"`
	Findings         []*DatabaseTopologyFindingVO `json:"findings,omitempty"`
	Message          string                       `json:"message"`
}

const (
	topologyCheckFreshSeconds = 5 * 60
	topologyCheckStaleSeconds = 30 * 60
)

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
	result, err := uc.collectDatabaseTopology(ctx, item, credential)
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

func (uc *UseCase) collectDatabaseTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	switch normalizeDBType(item.DBType) {
	case DBTypeMySQL, DBTypeMariaDB, DBTypePostgreSQL:
		return uc.collectRelationalReplicationTopology(ctx, item, credential)
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

func (uc *UseCase) collectRelationalReplicationTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
	engine := normalizeDBType(item.DBType)
	now := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, replicationCheckTimeout)
	defer cancel()

	var (
		currentCheck *DatabaseReplicationCheck
		collectErr   error
	)
	switch engine {
	case DBTypeMySQL, DBTypeMariaDB:
		currentCheck, collectErr = uc.collectMySQLReplicationStatus(checkCtx, item, credential, now)
	case DBTypePostgreSQL:
		currentCheck, collectErr = uc.collectPostgreSQLReplicationStatus(checkCtx, item, credential, now)
	default:
		return nil, fmt.Errorf("%s 拓扑视图将在后续批次接入", DBTypeText(item.DBType))
	}
	if currentCheck == nil {
		currentCheck = &DatabaseReplicationCheck{
			InstanceID:    item.ID,
			Engine:        engine,
			RoleDetected:  DatabaseReplicationRoleUnknown,
			HealthStatus:  DatabaseReplicaHealthUnknown,
			CheckedAt:     &now,
			ErrorMessage:  trimText(errorText(collectErr), 1000),
			RawStatusJSON: marshalReplicaJSON(map[string]any{"error": errorText(collectErr)}),
		}
	}

	instances := map[uint]*DatabaseInstance{item.ID: item}
	replicas := uc.relatedTopologyReplicas(ctx, item, engine)
	checks := uc.latestTopologyChecks(ctx, item.ID, replicas, currentCheck)
	nodesByID := make(map[string]*DatabaseTopologyNodeVO)
	linksByID := make(map[string]*DatabaseTopologyLinkVO)
	findings := make([]*DatabaseTopologyFindingVO, 0)

	currentNode := uc.ensureReplicationTopologyNode(ctx, nodesByID, instances, item.ID, "", currentCheck, nil)
	appendTopologyFindingsForCheck(&findings, currentNode.ID, currentCheck, collectErr)

	for _, replica := range replicas {
		if replica == nil {
			continue
		}
		primaryNode := uc.ensureReplicationPrimaryNode(ctx, nodesByID, instances, replica, checks[replica.PrimaryInstanceID])
		replicaCheck := checks[replica.ReplicaInstanceID]
		replicaNode := uc.ensureReplicationTopologyNode(ctx, nodesByID, instances, replica.ReplicaInstanceID, replica.ReplicaRole, replicaCheck, replica)
		link := buildReplicationTopologyLink(primaryNode, replicaNode, replica, replicaCheck)
		linksByID[replicationTopologyLinkID(link.Source, link.Target)] = link
		appendTopologyFindingsForCheck(&findings, replicaNode.ID, replicaCheck, nil)
		appendTopologyFindingsForFreshness(&findings, replicaNode.ID, replicaCheck, replica)
		appendTopologyFindingsForReplica(&findings, replicaNode.ID, replica)
	}

	if isReplicaDetected(currentCheck.RoleDetected) {
		replica := uc.buildReplicaRelationFromCheck(ctx, item, currentCheck)
		if replica != nil {
			primaryNode := uc.ensureReplicationPrimaryNode(ctx, nodesByID, instances, replica, checks[replica.PrimaryInstanceID])
			replicaNode := uc.ensureReplicationTopologyNode(ctx, nodesByID, instances, item.ID, replica.ReplicaRole, currentCheck, replica)
			link := buildReplicationTopologyLink(primaryNode, replicaNode, replica, currentCheck)
			linksByID[replicationTopologyLinkID(link.Source, link.Target)] = link
		}
	}

	if currentCheck.RoleDetected == DatabaseReplicationRolePrimary && engine == DBTypePostgreSQL {
		uc.appendPostgreSQLPrimaryRuntimeTopology(nodesByID, linksByID, currentNode, currentCheck)
	}
	if currentCheck.RoleDetected == DatabaseReplicationRolePrimary && (engine == DBTypeMySQL || engine == DBTypeMariaDB) {
		uc.appendMySQLPrimaryReportedTopology(nodesByID, linksByID, currentNode, currentCheck)
	}

	nodes := topologyNodeMapValues(nodesByID)
	links := topologyLinkMapValues(linksByID)
	cards := buildReplicationTopologyCards(item, currentCheck, nodes, links)
	message := DBTypeText(item.DBType) + " 主从拓扑读取成功"
	if collectErr != nil {
		message = "拓扑已基于最近副本关系返回，实时采集存在异常: " + collectErr.Error()
	}

	return &DatabaseTopologyVO{
		InstanceID:       item.ID,
		InstanceName:     item.Name,
		DBType:           item.DBType,
		DBTypeText:       DBTypeText(item.DBType),
		TopologyType:     replicationTopologyType(engine),
		TopologyTypeText: replicationTopologyTypeText(engine),
		CollectedAt:      now.Format("2006-01-02 15:04:05"),
		Cards:            cards,
		Nodes:            nodes,
		Links:            links,
		Findings:         dedupeTopologyFindings(findings),
		Message:          message,
	}, nil
}

func (uc *UseCase) relatedTopologyReplicas(ctx context.Context, item *DatabaseInstance, engine string) []*DatabaseInstanceReplica {
	if uc == nil || uc.instanceReplicaRepo == nil || item == nil {
		return nil
	}
	result := make([]*DatabaseInstanceReplica, 0)
	seen := make(map[uint]struct{})
	add := func(list []*DatabaseInstanceReplica) {
		for _, replica := range list {
			if replica == nil {
				continue
			}
			if _, ok := seen[replica.ID]; ok {
				continue
			}
			seen[replica.ID] = struct{}{}
			result = append(result, replica)
		}
	}
	base, _, err := uc.instanceReplicaRepo.List(ctx, &DatabaseInstanceReplicaListRequest{
		Page:       1,
		PageSize:   10000,
		InstanceID: item.ID,
		Engine:     engine,
	})
	if err == nil {
		add(base)
	}
	for _, replica := range append([]*DatabaseInstanceReplica{}, result...) {
		if replica.PrimaryInstanceID == 0 || replica.PrimaryInstanceID == item.ID {
			continue
		}
		siblings, _, err := uc.instanceReplicaRepo.List(ctx, &DatabaseInstanceReplicaListRequest{
			Page:              1,
			PageSize:          10000,
			PrimaryInstanceID: replica.PrimaryInstanceID,
			Engine:            engine,
		})
		if err == nil {
			add(siblings)
		}
	}
	return result
}

func (uc *UseCase) latestTopologyChecks(ctx context.Context, currentInstanceID uint, replicas []*DatabaseInstanceReplica, currentCheck *DatabaseReplicationCheck) map[uint]*DatabaseReplicationCheck {
	checks := make(map[uint]*DatabaseReplicationCheck)
	if currentCheck != nil && currentInstanceID > 0 {
		checks[currentInstanceID] = currentCheck
	}
	for _, replica := range replicas {
		if replica == nil {
			continue
		}
		if replica.PrimaryInstanceID > 0 {
			if _, ok := checks[replica.PrimaryInstanceID]; !ok {
				checks[replica.PrimaryInstanceID] = latestReplicationCheck(ctx, uc.replicationCheckRepo, replica.PrimaryInstanceID)
			}
		}
		if replica.ReplicaInstanceID > 0 {
			if _, ok := checks[replica.ReplicaInstanceID]; !ok {
				checks[replica.ReplicaInstanceID] = latestReplicationCheck(ctx, uc.replicationCheckRepo, replica.ReplicaInstanceID)
			}
		}
	}
	return checks
}

func (uc *UseCase) ensureReplicationPrimaryNode(
	ctx context.Context,
	nodes map[string]*DatabaseTopologyNodeVO,
	instances map[uint]*DatabaseInstance,
	replica *DatabaseInstanceReplica,
	check *DatabaseReplicationCheck,
) *DatabaseTopologyNodeVO {
	if replica == nil {
		return unknownTopologySourceNode(nodes, "", 0)
	}
	if replica.PrimaryInstanceID > 0 {
		return uc.ensureReplicationTopologyNode(ctx, nodes, instances, replica.PrimaryInstanceID, DatabaseReplicationRolePrimary, check, nil)
	}
	return unknownTopologySourceNode(nodes, replica.SourceHost, replica.SourcePort)
}

func (uc *UseCase) ensureReplicationTopologyNode(
	ctx context.Context,
	nodes map[string]*DatabaseTopologyNodeVO,
	instances map[uint]*DatabaseInstance,
	instanceID uint,
	fallbackRole string,
	check *DatabaseReplicationCheck,
	replica *DatabaseInstanceReplica,
) *DatabaseTopologyNodeVO {
	if instanceID == 0 {
		return unknownTopologySourceNode(nodes, "", 0)
	}
	id := topologyInstanceNodeID(instanceID)
	if existing := nodes[id]; existing != nil {
		mergeReplicationTopologyNode(existing, check, replica, fallbackRole)
		return existing
	}
	item := instances[instanceID]
	if item == nil && uc != nil && uc.instanceRepo != nil {
		if loaded, err := uc.instanceRepo.GetByID(ctx, instanceID); err == nil && loaded != nil {
			item = loaded
			instances[instanceID] = loaded
		}
	}
	name := fmt.Sprintf("#%d", instanceID)
	address := "-"
	version := ""
	if item != nil {
		name = item.Name
		address = net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port))
		version = item.Version
	}
	role := replicationTopologyRole(check, replica, fallbackRole)
	state := replicationTopologyState(check, replica)
	node := &DatabaseTopologyNodeVO{
		ID:        id,
		Name:      name,
		Role:      role,
		RoleText:  replicationTopologyRoleText(role),
		Address:   address,
		State:     state,
		Version:   version,
		LagText:   replicationTopologyLagText(check),
		Message:   replicationTopologyMessage(check, replica),
		Metrics:   replicationTopologyMetrics(check, replica),
		UpdatedAt: replicationTopologyUpdatedAt(check, replica),
	}
	nodes[id] = node
	return node
}

func mergeReplicationTopologyNode(node *DatabaseTopologyNodeVO, check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica, fallbackRole string) {
	if node == nil {
		return
	}
	if role := replicationTopologyRole(check, replica, fallbackRole); role != "" && role != DatabaseReplicationRoleUnknown {
		node.Role = role
		node.RoleText = replicationTopologyRoleText(role)
	}
	if state := replicationTopologyState(check, replica); state != DatabaseReplicaHealthUnknown {
		node.State = state
	}
	if lag := replicationTopologyLagText(check); lag != "" {
		node.LagText = lag
	}
	if message := replicationTopologyMessage(check, replica); message != "" {
		node.Message = message
	}
	if updatedAt := replicationTopologyUpdatedAt(check, replica); updatedAt != "" {
		node.UpdatedAt = updatedAt
	}
	if node.Metrics == nil {
		node.Metrics = map[string]string{}
	}
	for key, value := range replicationTopologyMetrics(check, replica) {
		node.Metrics[key] = value
	}
}

func unknownTopologySourceNode(nodes map[string]*DatabaseTopologyNodeVO, host string, port int) *DatabaseTopologyNodeVO {
	address := strings.TrimSpace(host)
	if address == "" {
		address = "unknown-source"
	} else if port > 0 {
		address = net.JoinHostPort(address, fmt.Sprintf("%d", port))
	}
	id := "source:" + address
	if existing := nodes[id]; existing != nil {
		return existing
	}
	node := &DatabaseTopologyNodeVO{
		ID:       id,
		Name:     "来源未匹配",
		Role:     DatabaseReplicationRolePrimary,
		RoleText: "来源主库",
		Address:  address,
		State:    DatabaseReplicaHealthUnknown,
		Message:  "未匹配到 OpsHub 已纳管实例",
		Metrics: map[string]string{
			"matched": "false",
		},
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	nodes[id] = node
	return node
}

func buildReplicationTopologyLink(primaryNode, replicaNode *DatabaseTopologyNodeVO, replica *DatabaseInstanceReplica, check *DatabaseReplicationCheck) *DatabaseTopologyLinkVO {
	if primaryNode == nil {
		primaryNode = &DatabaseTopologyNodeVO{ID: "source:unknown", Name: "来源未匹配"}
	}
	if replicaNode == nil {
		replicaNode = &DatabaseTopologyNodeVO{ID: "replica:unknown", Name: "未知副本"}
	}
	metrics := map[string]string{}
	if replica != nil {
		metrics["replica_id"] = strconv.FormatUint(uint64(replica.ID), 10)
		if replica.PrimaryInstanceID > 0 {
			metrics["primary_instance_id"] = strconv.FormatUint(uint64(replica.PrimaryInstanceID), 10)
		}
		if replica.ReplicaInstanceID > 0 {
			metrics["replica_instance_id"] = strconv.FormatUint(uint64(replica.ReplicaInstanceID), 10)
		}
		metrics["configured_delay_seconds"] = strconv.Itoa(replica.ConfiguredDelaySeconds)
		metrics["discovery_source"] = replica.DiscoverySource
		if replica.SourceHost != "" {
			metrics["source_host"] = replica.SourceHost
		}
		if replica.SourcePort > 0 {
			metrics["source_port"] = strconv.Itoa(replica.SourcePort)
		}
		if replica.LastCheckID > 0 {
			metrics["last_check_id"] = strconv.FormatUint(uint64(replica.LastCheckID), 10)
		}
	}
	if check != nil {
		metrics["health"] = check.HealthStatus
		if check.ID > 0 {
			metrics["check_id"] = strconv.FormatUint(uint64(check.ID), 10)
		}
		if secondsBehind, ok := replicationTopologySecondsBehindSource(check); ok {
			metrics["seconds_behind_source"] = strconv.Itoa(secondsBehind)
		}
		if remainingDelay, ok := replicationTopologyRemainingDelaySeconds(check); ok {
			metrics["remaining_delay_seconds"] = strconv.Itoa(remainingDelay)
		}
		if check.ReplicaIORunning != "" {
			metrics["io"] = check.ReplicaIORunning
		}
		if check.ReplicaSQLRunning != "" {
			metrics["sql"] = check.ReplicaSQLRunning
		}
		if check.PGReplayLagMs > 0 {
			metrics["pg_replay_lag_ms"] = strconv.FormatInt(check.PGReplayLagMs, 10)
		}
	}
	if checkedAt := replicationTopologyCheckedAt(check, replica); checkedAt != nil {
		metrics["last_checked_at"] = formatTime(checkedAt)
		ageSeconds, freshness := topologyCheckFreshness(checkedAt, time.Now())
		metrics["check_age_seconds"] = strconv.FormatInt(ageSeconds, 10)
		metrics["check_freshness"] = freshness
	} else {
		metrics["check_freshness"] = "unknown"
	}
	return &DatabaseTopologyLinkVO{
		Source:     primaryNode.ID,
		Target:     replicaNode.ID,
		SourceName: primaryNode.Name,
		TargetName: replicaNode.Name,
		Label:      replicationTopologyLinkLabel(check, replica),
		State:      replicationTopologyState(check, replica),
		LagText:    replicationTopologyLagText(check),
		Message:    replicationTopologyMessage(check, replica),
		Metrics:    metrics,
	}
}

func (uc *UseCase) appendMySQLPrimaryReportedTopology(
	nodes map[string]*DatabaseTopologyNodeVO,
	links map[string]*DatabaseTopologyLinkVO,
	primaryNode *DatabaseTopologyNodeVO,
	check *DatabaseReplicationCheck,
) {
	if primaryNode == nil || check == nil {
		return
	}
	rows := topologyRawRows(decodeReplicaRawMap(check.RawStatusJSON)["reported_replicas"])
	for _, row := range rows {
		host := firstNonEmpty(toString(row["Host"]), toString(row["host"]))
		port := parseInt(firstNonEmpty(toString(row["Port"]), toString(row["port"])), 0)
		serverID := firstNonEmpty(
			toString(row["Server_id"]),
			toString(row["Server_Id"]),
			toString(row["Server_ID"]),
			toString(row["server_id"]),
			toString(row["Id"]),
		)
		if strings.TrimSpace(host) == "" && strings.TrimSpace(serverID) == "" {
			continue
		}
		metrics := map[string]string{
			"discovery_source": "mysql_primary_reported",
		}
		if strings.TrimSpace(host) != "" {
			metrics["report_host"] = strings.TrimSpace(host)
		}
		if port > 0 {
			metrics["report_port"] = strconv.Itoa(port)
		}
		if strings.TrimSpace(serverID) != "" {
			metrics["server_id"] = strings.TrimSpace(serverID)
		}
		if value := firstNonEmpty(toString(row["Source_UUID"]), toString(row["Master_UUID"]), toString(row["Uuid"]), toString(row["UUID"])); value != "" {
			metrics["source_uuid"] = value
		}

		node := findTopologyNodeByAddress(nodes, host)
		if node == nil {
			nodeID := "mysql-reported-replica:" + valueOrDefault(strings.TrimSpace(host), "unknown")
			if port > 0 {
				nodeID += ":" + strconv.Itoa(port)
			}
			if strings.TrimSpace(serverID) != "" {
				nodeID += ":" + strings.TrimSpace(serverID)
			}
			node = nodes[nodeID]
			if node == nil {
				address := strings.TrimSpace(host)
				if address == "" {
					address = "reported-server-" + valueOrDefault(strings.TrimSpace(serverID), "unknown")
				} else if port > 0 {
					address = net.JoinHostPort(address, strconv.Itoa(port))
				}
				node = &DatabaseTopologyNodeVO{
					ID:        nodeID,
					Name:      "未纳管从库",
					Role:      DatabaseReplicationRoleReplica,
					RoleText:  replicationTopologyRoleText(DatabaseReplicationRoleReplica),
					Address:   address,
					State:     DatabaseReplicaHealthUnknown,
					Message:   "主库侧报告该从库，但未匹配到 OpsHub 已纳管实例",
					Metrics:   metrics,
					UpdatedAt: replicationTopologyUpdatedAt(check, nil),
				}
				nodes[nodeID] = node
			}
		} else {
			if node.Metrics == nil {
				node.Metrics = map[string]string{}
			}
			for key, value := range metrics {
				if strings.TrimSpace(value) != "" {
					node.Metrics[key] = value
				}
			}
		}
		linkID := replicationTopologyLinkID(primaryNode.ID, node.ID)
		if existing := links[linkID]; existing != nil {
			if existing.Metrics == nil {
				existing.Metrics = map[string]string{}
			}
			for key, value := range metrics {
				if strings.TrimSpace(value) != "" {
					existing.Metrics[key] = value
				}
			}
			continue
		}
		links[linkID] = &DatabaseTopologyLinkVO{
			Source:     primaryNode.ID,
			Target:     node.ID,
			SourceName: primaryNode.Name,
			TargetName: node.Name,
			Label:      "async replication",
			State:      node.State,
			Message:    "主库侧 SHOW REPLICAS/SLAVE HOSTS 报告",
			Metrics:    metrics,
		}
	}
}

func (uc *UseCase) appendPostgreSQLPrimaryRuntimeTopology(
	nodes map[string]*DatabaseTopologyNodeVO,
	links map[string]*DatabaseTopologyLinkVO,
	primaryNode *DatabaseTopologyNodeVO,
	check *DatabaseReplicationCheck,
) {
	if primaryNode == nil || check == nil {
		return
	}
	rows := topologyRawRows(decodeReplicaRawMap(check.RawStatusJSON)["pg_stat_replication"])
	for _, row := range rows {
		client := valueOrDefault(toString(row["client_addr"]), "unknown-client")
		appName := valueOrDefault(toString(row["application_name"]), "standby")
		nodeID := "pg-standby:" + client + ":" + appName
		state := topologyStateFromPostgreSQLReplicationRow(row)
		lagText := firstNonEmpty(toString(row["replay_lag"]), toString(row["flush_lag"]), toString(row["write_lag"]))
		metrics := map[string]string{
			"application_name": appName,
			"client_addr":      client,
			"state":            toString(row["state"]),
			"sync_state":       toString(row["sync_state"]),
			"write_lag":        toString(row["write_lag"]),
			"flush_lag":        toString(row["flush_lag"]),
			"replay_lag":       toString(row["replay_lag"]),
		}
		if existing := findTopologyNodeByAddress(nodes, client); existing != nil {
			nodeID = existing.ID
			existing.State = worseReplicaHealth(existing.State, state)
			if lagText != "" {
				existing.LagText = lagText
			}
			if existing.Metrics == nil {
				existing.Metrics = map[string]string{}
			}
			for key, value := range metrics {
				existing.Metrics[key] = value
			}
		} else if nodes[nodeID] == nil {
			nodes[nodeID] = &DatabaseTopologyNodeVO{
				ID:        nodeID,
				Name:      appName,
				Role:      DatabaseReplicationRoleStandby,
				RoleText:  replicationTopologyRoleText(DatabaseReplicationRoleStandby),
				Address:   client,
				State:     state,
				LagText:   lagText,
				Message:   firstNonEmpty(toString(row["state"]), "-"),
				Metrics:   metrics,
				UpdatedAt: replicationTopologyUpdatedAt(check, nil),
			}
		}
		link := &DatabaseTopologyLinkVO{
			Source:     primaryNode.ID,
			Target:     nodeID,
			SourceName: primaryNode.Name,
			TargetName: nodes[nodeID].Name,
			Label:      strings.TrimSpace("streaming " + toString(row["sync_state"])),
			State:      nodes[nodeID].State,
			LagText:    nodes[nodeID].LagText,
			Message:    toString(row["state"]),
			Metrics:    nodes[nodeID].Metrics,
		}
		links[replicationTopologyLinkID(link.Source, link.Target)] = link
	}
}

func findTopologyNodeByAddress(nodes map[string]*DatabaseTopologyNodeVO, address string) *DatabaseTopologyNodeVO {
	address = strings.TrimSpace(address)
	if address == "" || address == "unknown-client" {
		return nil
	}
	for _, node := range nodes {
		if node == nil {
			continue
		}
		host := strings.TrimSpace(node.Address)
		if host == address {
			return node
		}
		if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost == address {
			return node
		}
	}
	return nil
}

func topologyRawRows(value any) []map[string]any {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		switch typed := row.(type) {
		case map[string]any:
			result = append(result, typed)
		case map[string]string:
			item := make(map[string]any, len(typed))
			for key, value := range typed {
				item[key] = value
			}
			result = append(result, item)
		}
	}
	return result
}

func topologyStateFromPostgreSQLReplicationRow(row map[string]any) string {
	state := strings.ToLower(strings.TrimSpace(toString(row["state"])))
	if state == "" || state == "streaming" {
		return DatabaseReplicaHealthHealthy
	}
	if strings.Contains(state, "catch") || strings.Contains(state, "backup") {
		return DatabaseReplicaHealthWarning
	}
	return DatabaseReplicaHealthCritical
}

func buildReplicationTopologyCards(item *DatabaseInstance, currentCheck *DatabaseReplicationCheck, nodes []*DatabaseTopologyNodeVO, links []*DatabaseTopologyLinkVO) []*DatabaseTopologyCardVO {
	replicaCount := 0
	maxLagMs := int64(0)
	maxLagKnown := false
	health := DatabaseReplicaHealthHealthy
	remainingWindow := ""
	latestCheck := ""
	staleNodeCount := 0
	delayedReplicaCount := 0
	minProtectionWindow := int64(0)
	minProtectionKnown := false
	preferredReplicaName := ""
	preferredProtectionWindow := int64(0)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if isReplicationReplicaRole(node.Role) {
			replicaCount++
		}
		health = worseReplicaHealth(health, node.State)
		if value := strings.TrimSpace(node.Metrics["last_checked_at"]); value != "" && value > latestCheck {
			latestCheck = value
		}
		switch strings.TrimSpace(node.Metrics["check_freshness"]) {
		case "stale", "unknown":
			staleNodeCount++
		}
		if isReplicationDelayedRole(node.Role) {
			delayedReplicaCount++
			if remaining, ok := parseMetricInt64OK(node.Metrics, "remaining_delay_seconds"); ok && remaining >= 0 {
				if !minProtectionKnown || remaining < minProtectionWindow {
					minProtectionWindow = remaining
				}
				minProtectionKnown = true
				if node.State == DatabaseReplicaHealthHealthy && remaining > preferredProtectionWindow {
					preferredProtectionWindow = remaining
					preferredReplicaName = node.Name
				}
			}
		}
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		health = worseReplicaHealth(health, link.State)
		if value, ok := parseMetricInt64OK(link.Metrics, "pg_replay_lag_ms"); ok && value >= 0 {
			maxLagKnown = true
			if value > maxLagMs {
				maxLagMs = value
			}
		}
		if value, ok := parseMetricInt64OK(link.Metrics, "seconds_behind_source"); ok && value >= 0 {
			maxLagKnown = true
			if value*1000 > maxLagMs {
				maxLagMs = value * 1000
			}
		}
	}
	if currentCheck != nil && currentCheck.ConfiguredDelaySeconds > 0 {
		if remainingDelay, ok := replicationTopologyRemainingDelaySeconds(currentCheck); ok {
			remainingWindow = formatSeconds(int64(remainingDelay))
		}
	}
	maxLagText := "-"
	if maxLagKnown {
		maxLagText = formatReplicationMaxLagMillis(maxLagMs)
	}
	protectionStatus := "未保护"
	if delayedReplicaCount > 0 {
		protectionStatus = "降级"
		if minProtectionKnown && minProtectionWindow > 0 && preferredReplicaName != "" {
			protectionStatus = "受保护"
		}
	}
	minProtectionText := "-"
	if minProtectionKnown {
		minProtectionText = formatSeconds(minProtectionWindow)
	}
	return []*DatabaseTopologyCardVO{
		{Key: "topology_type", Label: "拓扑类型", Value: replicationTopologyTypeText(normalizeDBType(item.DBType)), Description: "当前实例的复制拓扑类型"},
		{Key: "role", Label: "当前角色", Value: replicationTopologyRoleText(replicationTopologyRole(currentCheck, nil, "")), Description: "实时采集识别出的当前实例角色"},
		{Key: "replicas", Label: "副本数量", Value: strconv.Itoa(replicaCount), Description: "拓扑中已识别的副本节点数量"},
		{Key: "health", Label: "复制健康", Value: ReplicaHealthText(health), Description: "根据节点和链路风险汇总出的健康状态"},
		{Key: "max_lag", Label: "最大延迟", Value: maxLagText, Description: "拓扑中可识别的最大复制延迟"},
		{Key: "latest_check", Label: "最近采集", Value: valueOrDefault(latestCheck, "-"), Description: "拓扑节点中最新一次副本状态采集时间"},
		{Key: "stale_nodes", Label: "过期节点", Value: strconv.Itoa(staleNodeCount), Description: "采集状态为 stale 或 unknown 的节点数量"},
		{Key: "delayed_replicas", Label: "延迟副本", Value: strconv.Itoa(delayedReplicaCount), Description: "拓扑中已识别的延迟副本数量"},
		{Key: "min_protection_window", Label: "最小保护窗口", Value: minProtectionText, Description: "所有延迟副本中可识别的最小剩余保护窗口"},
		{Key: "protection_status", Label: "保护状态", Value: protectionStatus, Description: "基于延迟副本健康状态和剩余窗口汇总"},
		{Key: "preferred_protection_replica", Label: "推荐保护副本", Value: valueOrDefault(preferredReplicaName, "-"), Description: "健康且剩余保护窗口最大的延迟副本"},
		{Key: "protection_window", Label: "保护窗口", Value: valueOrDefault(remainingWindow, "-"), Description: "延迟副本剩余保护窗口，仅在可识别时展示"},
	}
}

func replicationTopologyType(engine string) string {
	switch normalizeDBType(engine) {
	case DBTypePostgreSQL:
		return "postgresql_streaming_replication"
	case DBTypeMariaDB:
		return "mariadb_replication"
	default:
		return "mysql_replication"
	}
}

func replicationTopologyTypeText(engine string) string {
	switch normalizeDBType(engine) {
	case DBTypePostgreSQL:
		return "PostgreSQL Streaming Replication"
	case DBTypeMariaDB:
		return "MariaDB Replication"
	default:
		return "MySQL Replication"
	}
}

func topologyInstanceNodeID(instanceID uint) string {
	return fmt.Sprintf("instance:%d", instanceID)
}

func replicationTopologyLinkID(source, target string) string {
	return source + "->" + target
}

func replicationTopologyRole(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica, fallback string) string {
	if check != nil {
		switch check.RoleDetected {
		case DatabaseReplicationRoleReplica:
			if check.ConfiguredDelaySeconds > 0 {
				return DatabaseReplicaRoleDelayed
			}
			return DatabaseReplicationRoleReplica
		case DatabaseReplicationRoleStandby:
			if check.ConfiguredDelaySeconds > 0 {
				return "delayed_standby"
			}
			return DatabaseReplicationRoleStandby
		case DatabaseReplicationRolePrimary:
			return DatabaseReplicationRolePrimary
		}
	}
	if replica != nil {
		switch replica.ReplicaRole {
		case DatabaseReplicaRoleDelayed:
			if normalizeDBType(replica.Engine) == DBTypePostgreSQL {
				return "delayed_standby"
			}
			return DatabaseReplicaRoleDelayed
		case DatabaseReplicaRoleStandby:
			return DatabaseReplicationRoleStandby
		case DatabaseReplicaRoleRealtime:
			return DatabaseReplicationRoleReplica
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return DatabaseReplicationRoleUnknown
}

func replicationTopologyRoleText(role string) string {
	switch strings.TrimSpace(role) {
	case DatabaseReplicationRolePrimary:
		return "主库"
	case DatabaseReplicationRoleReplica:
		return "从库"
	case DatabaseReplicaRoleDelayed:
		return "延迟副本"
	case DatabaseReplicationRoleStandby:
		return "Standby"
	case "delayed_standby":
		return "延迟 Standby"
	default:
		return "未知"
	}
}

func replicationTopologyState(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) string {
	if check != nil && strings.TrimSpace(check.HealthStatus) != "" {
		return check.HealthStatus
	}
	if replica != nil && strings.TrimSpace(replica.Status) != "" {
		return replica.Status
	}
	return DatabaseReplicaHealthUnknown
}

func replicationTopologyLagText(check *DatabaseReplicationCheck) string {
	if check == nil {
		return ""
	}
	if normalizeDBType(check.Engine) == DBTypePostgreSQL {
		if check.PGReplayLagMs > 0 {
			return formatTopologyMillis(check.PGReplayLagMs)
		}
		if check.PGFlushLagMs > 0 {
			return formatTopologyMillis(check.PGFlushLagMs)
		}
		if check.PGWriteLagMs > 0 {
			return formatTopologyMillis(check.PGWriteLagMs)
		}
		return ""
	}
	if secondsBehind, ok := replicationTopologySecondsBehindSource(check); ok {
		return formatSeconds(int64(secondsBehind))
	}
	return ""
}

func replicationTopologySecondsBehindSource(check *DatabaseReplicationCheck) (int, bool) {
	if check == nil {
		return 0, false
	}
	if check.SecondsBehindSource >= 0 {
		return check.SecondsBehindSource, true
	}
	raw := decodeReplicaRawMap(check.RawStatusJSON)
	return parseReplicaRawNonNegativeInt(raw, "Seconds_Behind_Source", "Seconds_Behind_Master")
}

func replicationTopologyRemainingDelaySeconds(check *DatabaseReplicationCheck) (int, bool) {
	if check == nil {
		return 0, false
	}
	if check.RemainingDelaySeconds >= 0 {
		return check.RemainingDelaySeconds, true
	}
	raw := decodeReplicaRawMap(check.RawStatusJSON)
	return parseReplicaRawNonNegativeInt(raw, "SQL_Remaining_Delay")
}

func parseReplicaRawNonNegativeInt(raw map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value := strings.TrimSpace(toString(raw[key]))
		if value == "" || strings.EqualFold(value, "NULL") {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			continue
		}
		return parsed, true
	}
	return 0, false
}

func replicationTopologyMessage(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) string {
	if check != nil && strings.TrimSpace(check.ErrorMessage) != "" {
		return strings.TrimSpace(check.ErrorMessage)
	}
	if replica != nil && strings.TrimSpace(replica.LastError) != "" {
		return strings.TrimSpace(replica.LastError)
	}
	if check != nil && check.RoleDetected == DatabaseReplicationRoleReplica {
		return strings.TrimSpace(fmt.Sprintf("IO %s / SQL %s", valueOrDefault(check.ReplicaIORunning, "-"), valueOrDefault(check.ReplicaSQLRunning, "-")))
	}
	return ""
}

func replicationTopologyMetrics(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) map[string]string {
	metrics := make(map[string]string)
	if replica != nil {
		metrics["replica_id"] = strconv.FormatUint(uint64(replica.ID), 10)
		metrics["primary_instance_id"] = strconv.FormatUint(uint64(replica.PrimaryInstanceID), 10)
		metrics["replica_instance_id"] = strconv.FormatUint(uint64(replica.ReplicaInstanceID), 10)
		if replica.ReplicaInstanceID > 0 {
			metrics["instance_id"] = strconv.FormatUint(uint64(replica.ReplicaInstanceID), 10)
		}
		metrics["replica_role"] = replica.ReplicaRole
		metrics["discovery_source"] = replica.DiscoverySource
		metrics["configured_delay_seconds"] = strconv.Itoa(replica.ConfiguredDelaySeconds)
		if replica.LastCheckID > 0 {
			metrics["last_check_id"] = strconv.FormatUint(uint64(replica.LastCheckID), 10)
		}
		if replica.SourceHost != "" {
			metrics["source_host"] = replica.SourceHost
		}
		if replica.SourcePort > 0 {
			metrics["source_port"] = strconv.Itoa(replica.SourcePort)
		}
	}
	if check != nil {
		metrics["instance_id"] = strconv.FormatUint(uint64(check.InstanceID), 10)
		if check.ID > 0 {
			metrics["check_id"] = strconv.FormatUint(uint64(check.ID), 10)
			metrics["last_check_id"] = strconv.FormatUint(uint64(check.ID), 10)
		}
		metrics["role_detected"] = check.RoleDetected
		metrics["health_status"] = check.HealthStatus
		if secondsBehind, ok := replicationTopologySecondsBehindSource(check); ok {
			metrics["seconds_behind_source"] = strconv.Itoa(secondsBehind)
		}
		if remainingDelay, ok := replicationTopologyRemainingDelaySeconds(check); ok {
			metrics["remaining_delay_seconds"] = strconv.Itoa(remainingDelay)
		}
		metrics["configured_delay_seconds"] = strconv.Itoa(check.ConfiguredDelaySeconds)
		if check.ReplicaIORunning != "" {
			metrics["replica_io_running"] = check.ReplicaIORunning
		}
		if check.ReplicaSQLRunning != "" {
			metrics["replica_sql_running"] = check.ReplicaSQLRunning
		}
		if check.PGWriteLagMs > 0 {
			metrics["pg_write_lag_ms"] = strconv.FormatInt(check.PGWriteLagMs, 10)
		}
		if check.PGFlushLagMs > 0 {
			metrics["pg_flush_lag_ms"] = strconv.FormatInt(check.PGFlushLagMs, 10)
		}
		if check.PGReplayLagMs > 0 {
			metrics["pg_replay_lag_ms"] = strconv.FormatInt(check.PGReplayLagMs, 10)
		}
		if check.PGLastWALReplayLSN != "" {
			metrics["pg_last_wal_replay_lsn"] = check.PGLastWALReplayLSN
		}
		raw := decodeReplicaRawMap(check.RawStatusJSON)
		if receiver, ok := raw["pg_stat_wal_receiver"].(map[string]any); ok {
			metrics["wal_receiver_status"] = toString(receiver["status"])
			metrics["received_lsn"] = toString(receiver["received_lsn"])
			metrics["flushed_lsn"] = toString(receiver["flushed_lsn"])
			metrics["latest_end_lsn"] = toString(receiver["latest_end_lsn"])
		}
		if value := toString(raw["pg_is_wal_replay_paused"]); value != "" {
			metrics["pg_is_wal_replay_paused"] = value
		}
		if value := toString(raw["recovery_min_apply_delay"]); value != "" {
			metrics["recovery_min_apply_delay"] = value
		}
		for _, key := range []string{"server_id", "server_uuid", "version", "read_only", "super_read_only", "log_bin", "gtid_mode", "binlog_format", "binlog_row_image"} {
			if value := toString(raw[key]); value != "" {
				metrics[key] = value
			}
		}
	}
	if checkedAt := replicationTopologyCheckedAt(check, replica); checkedAt != nil {
		metrics["last_checked_at"] = formatTime(checkedAt)
		ageSeconds, freshness := topologyCheckFreshness(checkedAt, time.Now())
		metrics["check_age_seconds"] = strconv.FormatInt(ageSeconds, 10)
		metrics["check_freshness"] = freshness
	} else {
		metrics["check_freshness"] = "unknown"
	}
	return metrics
}

func replicationTopologyCheckedAt(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) *time.Time {
	if check != nil && check.CheckedAt != nil {
		return check.CheckedAt
	}
	if replica != nil && replica.LastCheckedAt != nil {
		return replica.LastCheckedAt
	}
	return nil
}

func topologyCheckFreshness(checkedAt *time.Time, now time.Time) (int64, string) {
	if checkedAt == nil {
		return -1, "unknown"
	}
	ageSeconds := int64(now.Sub(*checkedAt).Seconds())
	if ageSeconds < 0 {
		ageSeconds = 0
	}
	switch {
	case ageSeconds <= topologyCheckFreshSeconds:
		return ageSeconds, "fresh"
	case ageSeconds <= topologyCheckStaleSeconds:
		return ageSeconds, "warning"
	default:
		return ageSeconds, "stale"
	}
}

func replicationTopologyUpdatedAt(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) string {
	if check != nil && check.CheckedAt != nil {
		return formatTime(check.CheckedAt)
	}
	if replica != nil && replica.LastCheckedAt != nil {
		return formatTime(replica.LastCheckedAt)
	}
	return time.Now().Format("2006-01-02 15:04:05")
}

func replicationTopologyLinkLabel(check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) string {
	engine := ""
	if check != nil {
		engine = check.Engine
	} else if replica != nil {
		engine = replica.Engine
	}
	if normalizeDBType(engine) == DBTypePostgreSQL {
		return "streaming replication"
	}
	if replica != nil && replica.ConfiguredDelaySeconds > 0 {
		return "delayed replication"
	}
	if check != nil && check.ConfiguredDelaySeconds > 0 {
		return "delayed replication"
	}
	return "async replication"
}

func topologyNodeMapValues(nodes map[string]*DatabaseTopologyNodeVO) []*DatabaseTopologyNodeVO {
	result := make([]*DatabaseTopologyNodeVO, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}
	sortTopologyNodes(result)
	return result
}

func topologyLinkMapValues(links map[string]*DatabaseTopologyLinkVO) []*DatabaseTopologyLinkVO {
	result := make([]*DatabaseTopologyLinkVO, 0, len(links))
	for _, link := range links {
		result = append(result, link)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Source == result[j].Source {
			return result[i].Target < result[j].Target
		}
		return result[i].Source < result[j].Source
	})
	return result
}

func appendTopologyFindingsForCheck(findings *[]*DatabaseTopologyFindingVO, nodeID string, check *DatabaseReplicationCheck, collectErr error) {
	if findings == nil {
		return
	}
	if collectErr != nil {
		*findings = append(*findings, &DatabaseTopologyFindingVO{
			Level:       DatabaseReplicaHealthWarning,
			Category:    "collection",
			Title:       "实时拓扑采集异常",
			Description: trimText(collectErr.Error(), 300),
			Suggestion:  "确认账号是否具备复制状态查询权限，并检查实例连接状态。",
			NodeID:      nodeID,
		})
	}
	if check == nil {
		return
	}
	flags := decodeTopologyRiskFlags(check.RiskFlagsJSON)
	if len(flags) == 0 && strings.TrimSpace(check.ErrorMessage) != "" {
		flags = strings.FieldsFunc(check.ErrorMessage, func(r rune) bool {
			return r == '；' || r == ';' || r == '\n'
		})
	}
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)
		if flag == "" {
			continue
		}
		*findings = append(*findings, &DatabaseTopologyFindingVO{
			Level:       topologyFindingLevel(check.HealthStatus, flag),
			Category:    "replication",
			Title:       flag,
			Description: topologyFindingDescription(flag),
			Suggestion:  topologyFindingSuggestion(flag),
			NodeID:      nodeID,
		})
	}
}

func appendTopologyFindingsForReplica(findings *[]*DatabaseTopologyFindingVO, nodeID string, replica *DatabaseInstanceReplica) {
	if findings == nil || replica == nil || strings.TrimSpace(replica.LastError) == "" {
		return
	}
	*findings = append(*findings, &DatabaseTopologyFindingVO{
		Level:       topologyFindingLevel(replica.Status, replica.LastError),
		Category:    "replica_relation",
		Title:       trimText(replica.LastError, 80),
		Description: trimText(replica.LastError, 300),
		Suggestion:  "进入副本治理查看最近采集详情，并确认复制链路是否需要处理。",
		NodeID:      nodeID,
	})
}

func appendTopologyFindingsForFreshness(findings *[]*DatabaseTopologyFindingVO, nodeID string, check *DatabaseReplicationCheck, replica *DatabaseInstanceReplica) {
	if findings == nil {
		return
	}
	checkedAt := replicationTopologyCheckedAt(check, replica)
	ageSeconds, freshness := topologyCheckFreshness(checkedAt, time.Now())
	switch freshness {
	case "unknown":
		*findings = append(*findings, &DatabaseTopologyFindingVO{
			Level:       DatabaseReplicaHealthWarning,
			Category:    "collection",
			Title:       "副本尚无采集记录",
			Description: "该副本关系没有可用的最近采集时间，拓扑只能展示静态关系。",
			Suggestion:  "点击采集相关实例，或进入副本治理采集该实例后再刷新拓扑。",
			NodeID:      nodeID,
		})
	case "warning":
		*findings = append(*findings, &DatabaseTopologyFindingVO{
			Level:       DatabaseReplicaHealthWarning,
			Category:    "collection",
			Title:       "副本状态采集不够新鲜",
			Description: fmt.Sprintf("该副本最近一次状态采集距今约 %s，拓扑可能不是最新状态。", formatSeconds(ageSeconds)),
			Suggestion:  "点击采集相关实例刷新当前拓扑中的主库和副本状态。",
			NodeID:      nodeID,
		})
	case "stale":
		*findings = append(*findings, &DatabaseTopologyFindingVO{
			Level:       DatabaseReplicaHealthWarning,
			Category:    "collection",
			Title:       "副本状态采集已过期",
			Description: fmt.Sprintf("该副本最近一次状态采集距今约 %s，拓扑中的延迟和线程状态可能已经过期。", formatSeconds(ageSeconds)),
			Suggestion:  "点击采集相关实例刷新当前拓扑中的主库和副本状态。",
			NodeID:      nodeID,
		})
	}
}

func decodeTopologyRiskFlags(raw string) []string {
	var flags []string
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		return nil
	}
	return flags
}

func topologyFindingLevel(status, text string) string {
	if strings.TrimSpace(status) == DatabaseReplicaHealthCritical {
		return DatabaseReplicaHealthCritical
	}
	lower := strings.ToLower(text)
	if strings.Contains(text, "异常") || strings.Contains(text, "失败") || strings.Contains(lower, "failed") {
		return DatabaseReplicaHealthCritical
	}
	if strings.TrimSpace(status) == DatabaseReplicaHealthHealthy {
		return "info"
	}
	return DatabaseReplicaHealthWarning
}

func topologyFindingDescription(flag string) string {
	switch {
	case strings.Contains(flag, "来源主库未匹配"):
		return "副本状态中能看到来源地址，但未匹配到 OpsHub 已纳管的主库实例。"
	case strings.Contains(flag, "IO 线程异常"):
		return "MySQL / MariaDB 复制 IO 线程未处于 Yes 状态。"
	case strings.Contains(flag, "SQL apply 线程异常"):
		return "MySQL / MariaDB 复制 SQL apply 线程未处于 Yes 状态。"
	case strings.Contains(flag, "复制延迟过大"):
		return "当前复制延迟超过默认预警阈值。"
	case strings.Contains(flag, "延迟副本已追上"):
		return "延迟副本剩余延迟为 0，当前没有可用于误操作截停的保护窗口。"
	case strings.Contains(flag, "从库未开启只读保护"):
		return "MySQL / MariaDB 从库 read_only 未开启，业务或误操作可能直接写入从库。"
	case strings.Contains(flag, "从库未开启 super_read_only"):
		return "MySQL / MariaDB 从库 super_read_only 未开启，具备高权限的会话仍可能写入。"
	case strings.Contains(flag, "主库未开启 binlog"):
		return "MySQL / MariaDB 主库 log_bin 未开启，不利于复制链路和时间点恢复。"
	case strings.Contains(flag, "server_id 配置无效"):
		return "MySQL / MariaDB server_id 缺失或为 0，复制身份配置不完整。"
	case strings.Contains(flag, "WAL replay 已暂停"):
		return "PostgreSQL standby 当前 WAL replay 处于暂停状态。"
	case strings.Contains(flag, "WAL receiver"):
		return "PostgreSQL standby 的 WAL receiver 状态异常或无法读取。"
	default:
		return flag
	}
}

func topologyFindingSuggestion(flag string) string {
	switch {
	case strings.Contains(flag, "来源主库未匹配"):
		return "确认主库实例已纳管，或检查 source host / primary_conninfo 是否与实例地址一致。"
	case strings.Contains(flag, "IO 线程异常"), strings.Contains(flag, "SQL apply 线程异常"):
		return "进入副本治理查看原始采集结果，确认复制错误后再决定是否恢复 apply。"
	case strings.Contains(flag, "复制延迟过大"):
		return "检查主从网络、从库负载和长事务，必要时查看慢 SQL 与容量趋势。"
	case strings.Contains(flag, "延迟副本已追上"):
		return "确认延迟副本配置是否符合误操作保护目标，必要时重新配置延迟窗口。"
	case strings.Contains(flag, "从库未开启只读保护"):
		return "检查从库配置 read_only=ON，并确认业务连接不会写入从库。"
	case strings.Contains(flag, "从库未开启 super_read_only"):
		return "如版本支持，建议开启 super_read_only=ON，避免高权限账号绕过只读保护。"
	case strings.Contains(flag, "主库未开启 binlog"):
		return "确认主库 binlog 配置；如需复制和 PITR，应按变更流程开启。"
	case strings.Contains(flag, "server_id 配置无效"):
		return "为每个 MySQL / MariaDB 节点配置唯一且非 0 的 server_id。"
	case strings.Contains(flag, "WAL replay 已暂停"):
		return "确认是否为计划内暂停；若非计划内，进入副本治理执行恢复流程。"
	case strings.Contains(flag, "WAL receiver"):
		return "检查 primary_conninfo、复制槽、网络连通性和 PostgreSQL 日志。"
	default:
		return "进入副本治理查看最近采集和原始状态。"
	}
}

func dedupeTopologyFindings(items []*DatabaseTopologyFindingVO) []*DatabaseTopologyFindingVO {
	result := make([]*DatabaseTopologyFindingVO, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		if item == nil {
			continue
		}
		key := item.Level + "|" + item.Category + "|" + item.Title + "|" + item.NodeID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func isReplicationReplicaRole(role string) bool {
	switch strings.TrimSpace(role) {
	case DatabaseReplicationRoleReplica, DatabaseReplicaRoleDelayed, DatabaseReplicationRoleStandby, "delayed_standby":
		return true
	default:
		return false
	}
}

func isReplicationDelayedRole(role string) bool {
	switch strings.TrimSpace(role) {
	case DatabaseReplicaRoleDelayed, "delayed_standby":
		return true
	default:
		return false
	}
}

func worseReplicaHealth(left, right string) string {
	score := func(value string) int {
		switch strings.TrimSpace(value) {
		case DatabaseReplicaHealthCritical:
			return 3
		case DatabaseReplicaHealthWarning:
			return 2
		case DatabaseReplicaHealthUnknown:
			return 1
		case DatabaseReplicaHealthHealthy:
			return 0
		default:
			return 1
		}
	}
	if score(right) > score(left) {
		return right
	}
	return left
}

func parseMetricInt64OK(metrics map[string]string, key string) (int64, bool) {
	if metrics == nil {
		return 0, false
	}
	raw, ok := metrics[key]
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func formatReplicationMaxLagMillis(value int64) string {
	if value <= 0 {
		return "0s"
	}
	return formatTopologyMillis(value)
}

func formatTopologyMillis(value int64) string {
	if value <= 0 {
		return "0 ms"
	}
	if value < 1000 {
		return fmt.Sprintf("%d ms", value)
	}
	return formatSeconds(value / 1000)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
		return collectRedisClusterTopology(queryCtx, client, item, credential, clusterInfo)
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

func collectRedisClusterTopology(ctx context.Context, client *redis.Client, item *DatabaseInstance, credential *ConnectionCredential, clusterInfo string) (*DatabaseTopologyVO, error) {
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
	defaultVersion := readRedisServerVersion(ctx, client)
	nodeVersions := collectRedisClusterNodeVersions(ctx, item, credential, nodes)
	applyRedisClusterNodeVersions(nodes, nodeVersions, defaultVersion)
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

func readRedisServerVersion(ctx context.Context, client *redis.Client) string {
	if client == nil {
		return ""
	}
	infoText, err := client.Info(ctx, "server").Result()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parseRedisInfo(infoText)["redis_version"])
}

func collectRedisClusterNodeVersions(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential, nodes []*DatabaseTopologyNodeVO) map[string]string {
	versions := make(map[string]string)
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.Address) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.State)), "fail") {
			continue
		}
		address := normalizeRedisClusterAddress(node.Address)
		if address == "" {
			continue
		}
		if _, ok := versions[address]; ok {
			continue
		}
		nodeClient, err := openRedisNodeClientByAddr(address, item, credential, 0)
		if err != nil {
			continue
		}
		version := readRedisServerVersion(ctx, nodeClient)
		nodeClient.Close()
		if version != "" {
			versions[address] = version
		}
	}
	return versions
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
		options.TLSConfig = &tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")}
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

func applyRedisClusterNodeVersions(nodes []*DatabaseTopologyNodeVO, versionsByAddr map[string]string, fallbackVersion string) {
	fallbackVersion = strings.TrimSpace(fallbackVersion)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		version := strings.TrimSpace(versionsByAddr[normalizeRedisClusterAddress(node.Address)])
		if version == "" {
			version = fallbackVersion
		}
		node.Version = version
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
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify")})
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
		return strings.TrimRight(rawURL, "/"), connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify"), nil
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
	return scheme + "://" + net.JoinHostPort(strings.TrimSpace(item.Host), fmt.Sprintf("%d", item.Port)), connectionParamBool(params, false, "insecureSkipVerify", "insecure_skip_verify"), nil
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
