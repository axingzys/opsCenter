# 数据库拓扑 MySQL / PostgreSQL 主从增强方案

## 1. 背景

当前数据库管理已经具备拓扑页签，后端提供统一拓扑接口：

- `GET /api/v1/databases/instances/{id}/topology`
- 后端拓扑对象位于 `internal/biz/database/topology.go`
- 前端拓扑页位于 `web/src/views/asset/DatabaseManagement.vue`
- 前端 API 类型位于 `web/src/api/database.ts`

现有拓扑能力主要覆盖：

1. Redis Cluster / 单机 / 主从 / Sentinel。
2. MongoDB ReplicaSet / 单机基础信息。
3. Elasticsearch / OpenSearch 集群节点和分片。

当前拓扑后端数据结构已经比较通用：

1. `Cards`：摘要指标。
2. `Nodes`：拓扑节点。
3. `Links`：节点关系。
4. `Shards`：搜索类数据库的分片信息。
5. `Message`：采集说明。

这套结构天然适合承载 MySQL / MariaDB / PostgreSQL 的主从和 standby 关系。但是当前 `collectDatabaseTopology` 只分发 Redis、MongoDB、Elasticsearch、OpenSearch，MySQL / MariaDB / PostgreSQL 还没有接入拓扑。

同时，副本治理模块已经具备 MySQL / MariaDB / PostgreSQL 的副本状态采集能力：

1. MySQL / MariaDB：
   - `SHOW REPLICA STATUS`
   - `SHOW SLAVE STATUS`
   - `Seconds_Behind_Source`
   - `Replica_IO_Running`
   - `Replica_SQL_Running`
   - 延迟副本 `SQL_Delay` 和 `SQL_Remaining_Delay`
2. PostgreSQL：
   - `pg_is_in_recovery()`
   - `pg_stat_replication`
   - `pg_stat_wal_receiver`
   - `pg_is_wal_replay_paused()`
   - `pg_last_wal_receive_lsn()`
   - `pg_last_wal_replay_lsn()`
   - `pg_last_xact_replay_timestamp()`
   - `recovery_min_apply_delay`

因此 MySQL / PostgreSQL 主从拓扑不应从零实现，而应复用副本治理已有采集能力和已归并的副本关系。

## 2. 结论

MySQL / MariaDB / PostgreSQL 主从显示值得优先加入拓扑页。

原因：

1. 这是日常数据库运维最高频的拓扑需求。
2. 当前 Redis / MongoDB / 搜索集群拓扑偏复杂类型展示，MySQL / PostgreSQL 主从才是大多数生产环境最常用视图。
3. 后端已有副本采集和副本关系表，接入成本可控。
4. 拓扑页可以把“实例视角”和“副本治理视角”统一起来，减少用户在多个页签之间来回切换。
5. 对误操作保护、延迟副本、PITR、事故指引都有直接价值。

## 3. 设计目标

### 3.1 功能目标

1. 拓扑页支持选择 MySQL、MariaDB、PostgreSQL 实例。
2. 能展示主库、从库、standby、延迟副本之间的关系。
3. 能展示复制健康状态、复制延迟、Apply / Replay 状态。
4. 能识别来源主库未匹配、复制中断、延迟过大、保护窗口消失等风险。
5. 能从拓扑页跳转到副本治理页查看详情或执行治理动作。
6. 拓扑查询继续写入统一审计。

### 3.2 体验目标

1. 拓扑页不只是表格，要提供图形化关系视图。
2. 图形视图用于快速定位主从结构，表格视图用于精确排障。
3. 节点颜色表达健康状态：
   - 绿色：健康。
   - 黄色：警告或延迟偏高。
   - 红色：异常或复制中断。
   - 灰色：未知或未匹配。
4. 边上展示关键复制指标，例如 lag、sync_state、IO / SQL 状态。
5. 点击节点打开详情抽屉，避免主页面堆太多列。

### 3.3 非目标

第一批不做以下能力：

1. 不在拓扑页直接做 failover、promote、主从切换。
2. 不在拓扑页直接执行任意 SQL。
3. 不强制要求所有主从关系都实时自动发现。
4. 不新增长期拓扑快照表。
5. 不把 pause / resume apply 放到拓扑首屏直接执行。

高风险动作仍放在“副本治理”页，拓扑页只提供跳转入口。

## 4. 当前代码现状

### 4.1 后端拓扑

文件：

- `internal/biz/database/topology.go`

当前 `DatabaseTopologyVO` 已经包含：

```go
type DatabaseTopologyVO struct {
    InstanceID       uint
    InstanceName     string
    DBType           string
    DBTypeText       string
    TopologyType     string
    TopologyTypeText string
    CollectedAt      string
    Cards            []*DatabaseTopologyCardVO
    Nodes            []*DatabaseTopologyNodeVO
    Links            []*DatabaseTopologyLinkVO
    Shards           []*DatabaseShardVO
    Message          string
}
```

当前 `collectDatabaseTopology` 支持：

1. Redis。
2. MongoDB。
3. Elasticsearch。
4. OpenSearch。

MySQL / MariaDB / PostgreSQL 当前未接入。

### 4.2 前端拓扑页

文件：

- `web/src/views/asset/DatabaseManagement.vue`

当前前端拓扑页具备：

1. 实例选择。
2. 刷新拓扑。
3. 摘要卡片。
4. 节点表。
5. 复制关系表。
6. 搜索类数据库分片表。

当前实例筛选依赖后端能力标记：

```ts
const topologyInstances = computed(() =>
  instanceOptions.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.TOPOLOGY, 'topologyEnabled'))
)
```

因此只要后端 `SupportedTypes()` 给 MySQL / MariaDB / PostgreSQL 打开 `TopologyEnabled`，前端实例列表就能自然出现这些实例。

### 4.3 副本治理

文件：

- `internal/biz/database/replication.go`
- `internal/biz/database/model.go`
- `internal/data/database/replication_repository.go`
- `web/src/api/database.ts`
- `web/src/views/asset/DatabaseManagement.vue`

已有副本关系表：

```go
type DatabaseInstanceReplica struct {
    PrimaryInstanceID      uint
    ReplicaInstanceID      uint
    Engine                 string
    ReplicaRole            string
    SourceHost             string
    SourcePort             int
    SourceServerUUID       string
    PGSystemIdentifier     string
    ApplicationName        string
    ConfiguredDelaySeconds int
    DiscoverySource        string
    Status                 string
    LastCheckID            uint
    LastCheckedAt          *time.Time
    LastError              string
}
```

已有副本检查表：

```go
type DatabaseReplicationCheck struct {
    InstanceID                uint
    ReplicaID                 uint
    Engine                    string
    RoleDetected              string
    SourceInstanceID          uint
    ReplicaIORunning          string
    ReplicaSQLRunning         string
    SecondsBehindSource       int
    ConfiguredDelaySeconds    int
    RemainingDelaySeconds     int
    RelayLogBytes             int64
    PGWriteLagMs              int64
    PGFlushLagMs              int64
    PGReplayLagMs             int64
    PGLastWALReplayLSN        string
    PGLastXactReplayTimestamp *time.Time
    WALBacklogBytes           int64
    HealthStatus              string
    RiskFlagsJSON             string
    RawStatusJSON             string
    CheckedAt                 *time.Time
    ErrorMessage              string
}
```

这些字段已经足够生成第一版 MySQL / PostgreSQL 拓扑。

## 5. 推荐架构

### 5.1 数据来源策略

MySQL / PostgreSQL 拓扑建议使用“实时采集 + 已归并副本关系”的混合方式。

原因：

1. 只查当前实例，无法可靠得到完整主从图。
2. MySQL 主库端 `SHOW REPLICAS` 或 `SHOW SLAVE HOSTS` 依赖从库 `report_host`，很多环境不会配置。
3. PostgreSQL 主库端 `pg_stat_replication` 只能看到当前连接到主库的 standby，不一定能映射到 OpsHub 已纳管实例。
4. 已有 `database_instance_replicas` 能保存从库主动采集后归并出来的关系。
5. 已有 `database_replication_checks` 能补充最近一次健康状态和延迟指标。

建议策略：

1. 对当前选中实例做一次只读实时采集。
2. 读取当前实例相关的 `DatabaseInstanceReplica`：
   - 当前实例是主库时，加载它的从库。
   - 当前实例是从库时，加载它的主库和同主库下其他从库。
3. 读取相关实例最近一次 `DatabaseReplicationCheck`。
4. 将实时采集结果覆盖当前节点状态。
5. 将历史归并关系补成完整拓扑。
6. 对未匹配主库的副本生成虚拟 source 节点。

### 5.2 后端入口

在 `collectDatabaseTopology` 增加分支：

```go
func collectDatabaseTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error) {
    switch normalizeDBType(item.DBType) {
    case DBTypeMySQL, DBTypeMariaDB:
        return uc.collectMySQLReplicationTopology(ctx, item, credential)
    case DBTypePostgreSQL:
        return uc.collectPostgreSQLReplicationTopology(ctx, item, credential)
    case DBTypeRedis:
        return collectRedisTopology(ctx, item, credential)
    case DBTypeMongoDB:
        return collectMongoDBTopology(ctx, item, credential)
    case DBTypeElasticsearch, DBTypeOpenSearch:
        return collectSearchTopology(ctx, item, credential)
    default:
        return nil, fmt.Errorf("%s 拓扑视图将在后续批次接入", DBTypeText(item.DBType))
    }
}
```

注意：当前 `collectDatabaseTopology` 是普通函数，不能直接访问 repo。为了复用副本关系，建议改为 `UseCase` 方法：

```go
func (uc *UseCase) collectDatabaseTopology(ctx context.Context, item *DatabaseInstance, credential *ConnectionCredential) (*DatabaseTopologyVO, error)
```

已有 Redis / MongoDB / Search 采集函数可继续保持纯函数。

### 5.3 能力开关

`SupportedTypes()` 中打开：

```go
{Type: DBTypeMySQL, Name: "MySQL", DefaultPort: 3306, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, TopologyEnabled: true, Phase: "phase1"},
{Type: DBTypeMariaDB, Name: "MariaDB", DefaultPort: 3306, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, TopologyEnabled: true, Phase: "phase1"},
{Type: DBTypePostgreSQL, Name: "PostgreSQL", DefaultPort: 5432, MetadataEnabled: true, QueryEnabled: true, TestEnabled: true, TopologyEnabled: true, Phase: "phase1"},
```

这样前端拓扑实例列表会自动出现 MySQL / MariaDB / PostgreSQL。

## 6. 后端数据设计

### 6.1 复用现有节点结构

`DatabaseTopologyNodeVO` 当前字段已经够第一版使用：

```go
type DatabaseTopologyNodeVO struct {
    ID        string
    Name      string
    Role      string
    RoleText  string
    Address   string
    State     string
    Version   string
    Slots     string
    LagBytes  int64
    LagText   string
    Message   string
    Metrics   map[string]string
    UpdatedAt string
}
```

MySQL / PostgreSQL 节点字段映射建议：

| 字段 | MySQL / MariaDB | PostgreSQL |
| --- | --- | --- |
| `ID` | `instance:{id}` 或 `mysql:{server_uuid}` | `instance:{id}` 或 `pg:{system_identifier}:{host}` |
| `Name` | 实例名或 `host:port` | 实例名或 `host:port` |
| `Role` | `primary` / `replica` / `delayed_replica` / `unknown` | `primary` / `standby` / `delayed_standby` / `unknown` |
| `RoleText` | 主库 / 从库 / 延迟副本 / 未知 | Primary / Standby / 延迟 Standby / 未知 |
| `Address` | `host:port` | `host:port` |
| `State` | `healthy` / `warning` / `critical` / `unknown` | `healthy` / `warning` / `critical` / `unknown` |
| `Version` | `@@version` | `SHOW server_version` |
| `LagText` | `Seconds_Behind_Source` | `replay_lag` 或 replay 时间差 |
| `Message` | 风险摘要 | 风险摘要 |
| `Metrics` | IO/SQL、GTID、read_only 等 | sync_state、WAL receiver、LSN 等 |

### 6.2 建议扩展 Link 字段

当前 `DatabaseTopologyLinkVO` 字段偏少：

```go
type DatabaseTopologyLinkVO struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Label  string `json:"label"`
    State  string `json:"state"`
}
```

为了图形化展示复制边，建议兼容性扩展：

```go
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
```

兼容性：

1. 旧前端仍能读取 `source`、`target`、`label`、`state`。
2. 新前端可以使用 `lagText` 和 `metrics` 展示边上的状态。

### 6.3 建议增加 Findings

当前拓扑只有 `Message`，不能结构化展示异常。建议增加：

```go
type DatabaseTopologyFindingVO struct {
    Level       string `json:"level"`
    Category    string `json:"category"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Suggestion  string `json:"suggestion"`
    NodeID      string `json:"nodeId,omitempty"`
    LinkID      string `json:"linkId,omitempty"`
}
```

然后扩展：

```go
type DatabaseTopologyVO struct {
    ...
    Findings []*DatabaseTopologyFindingVO `json:"findings,omitempty"`
}
```

第一版 findings 来源：

1. 主库未匹配。
2. MySQL IO 线程异常。
3. MySQL SQL apply 线程异常。
4. MySQL 复制延迟超过阈值。
5. MySQL 延迟副本已追上，保护窗口消失。
6. PostgreSQL WAL receiver 异常。
7. PostgreSQL replay paused。
8. PostgreSQL replay lag 超过阈值。
9. 当前主库没有任何登记副本。
10. 最近采集时间过旧。

## 7. MySQL / MariaDB 拓扑采集设计

### 7.1 当前节点采集

对当前实例执行只读查询：

```sql
SELECT
  @@server_uuid AS server_uuid,
  @@hostname AS hostname,
  @@port AS port,
  @@version AS version,
  @@read_only AS read_only,
  @@super_read_only AS super_read_only,
  @@log_bin AS log_bin,
  @@gtid_mode AS gtid_mode;
```

兼容 MariaDB 时部分变量可能不存在，采集失败的单项变量不应导致整个拓扑失败。可拆分为多个 `SHOW VARIABLES LIKE ...` 或容错读取。

然后执行：

```sql
SHOW REPLICA STATUS;
```

失败或语法不支持时降级：

```sql
SHOW SLAVE STATUS;
```

如果返回空行，当前节点按 primary 或未配置 replica 处理。

如果返回数据，当前节点按 replica / delayed replica 处理。

### 7.2 MySQL 节点 Metrics

建议放入 `node.metrics`：

```json
{
  "server_uuid": "xxx",
  "read_only": "ON",
  "super_read_only": "ON",
  "log_bin": "ON",
  "gtid_mode": "ON",
  "replica_io_running": "Yes",
  "replica_sql_running": "Yes",
  "seconds_behind_source": "3",
  "sql_delay": "3600",
  "sql_remaining_delay": "1800",
  "source_host": "10.0.0.1",
  "source_port": "3306"
}
```

### 7.3 MySQL Cards

建议摘要卡：

1. `拓扑类型`：MySQL Replication / MariaDB Replication。
2. `当前角色`：主库 / 从库 / 延迟副本 / 未知。
3. `副本数量`：已归并从库数量。
4. `复制健康`：健康 / 警告 / 异常 / 未知。
5. `最大延迟`：当前拓扑内最大 lag。
6. `保护窗口`：存在延迟副本时显示剩余窗口。

### 7.4 MySQL Link

复制关系建议方向：

```text
primary -> replica
```

而 Redis 当前代码是 `replica -> master`。为了前端图形理解一致，MySQL/PG 建议使用 `primary -> replica`。如果担心不同数据库方向不一致，可以在前端通过 `label` 解释，不要求所有类型强行统一。

Link 示例：

```json
{
  "source": "instance:1",
  "target": "instance:2",
  "sourceName": "mysql-primary",
  "targetName": "mysql-replica-01",
  "label": "async replication",
  "state": "healthy",
  "lagText": "3s",
  "message": "IO/SQL running",
  "metrics": {
    "io": "Yes",
    "sql": "Yes",
    "delay": "0"
  }
}
```

### 7.5 MySQL 风险规则

第一版使用现有副本治理风险判断即可：

1. `Replica_IO_Running != Yes`：critical。
2. `Replica_SQL_Running != Yes`：critical。
3. `Seconds_Behind_Source >= critical threshold`：critical。
4. `Seconds_Behind_Source >= warning threshold`：warning。
5. `SQL_Delay > 0 && SQL_Remaining_Delay == 0`：warning，延迟副本保护窗口消失。
6. `SourceInstanceID == 0`：warning，来源主库未匹配。
7. 最近采集超过阈值：warning。

## 8. PostgreSQL 拓扑采集设计

### 8.1 当前节点角色判断

执行：

```sql
SELECT pg_is_in_recovery();
```

结果：

1. `false`：当前为 primary。
2. `true`：当前为 standby。

### 8.2 PostgreSQL Primary 采集

Primary 上执行：

```sql
SELECT
  application_name,
  COALESCE(client_addr::text, '') AS client_addr,
  COALESCE(state, '') AS state,
  COALESCE(sync_state, '') AS sync_state,
  COALESCE(write_lag::text, '') AS write_lag,
  COALESCE(flush_lag::text, '') AS flush_lag,
  COALESCE(replay_lag::text, '') AS replay_lag,
  sent_lsn::text AS sent_lsn,
  write_lsn::text AS write_lsn,
  flush_lsn::text AS flush_lsn,
  replay_lsn::text AS replay_lsn
FROM pg_stat_replication;
```

用途：

1. 生成 primary 节点。
2. 生成连接到该 primary 的 standby 虚拟节点或匹配已纳管实例。
3. 生成 primary -> standby 链路。
4. 展示 sync_state、state、write/flush/replay lag。

### 8.3 PostgreSQL Standby 采集

Standby 上执行：

```sql
SELECT pg_is_wal_replay_paused();
```

```sql
SELECT
  COALESCE(status, '') AS status,
  COALESCE(receive_start_lsn::text, '') AS receive_start_lsn,
  COALESCE(written_lsn::text, '') AS received_lsn,
  COALESCE(flushed_lsn::text, '') AS flushed_lsn,
  COALESCE(latest_end_lsn::text, '') AS latest_end_lsn,
  COALESCE(latest_end_time::text, '') AS latest_end_time,
  COALESCE(conninfo, '') AS conninfo
FROM pg_stat_wal_receiver
LIMIT 1;
```

```sql
SELECT
  pg_last_wal_receive_lsn()::text,
  pg_last_wal_replay_lsn()::text,
  pg_last_xact_replay_timestamp();
```

用途：

1. 判断 WAL receiver 是否正常。
2. 判断 replay 是否暂停。
3. 计算 replay 延迟。
4. 从 `primary_conninfo` 或 `conninfo` 匹配来源主库。

### 8.4 PostgreSQL 节点 Metrics

Primary 节点：

```json
{
  "in_recovery": "false",
  "standby_count": "2",
  "sync_standby_count": "1",
  "async_standby_count": "1"
}
```

Standby 节点：

```json
{
  "in_recovery": "true",
  "wal_receiver_status": "streaming",
  "replay_paused": "false",
  "receive_lsn": "0/5000060",
  "replay_lsn": "0/5000060",
  "last_xact_replay_timestamp": "2026-05-04 16:30:00",
  "primary_host": "10.0.0.1",
  "primary_port": "5432",
  "application_name": "standby-01"
}
```

### 8.5 PostgreSQL Cards

建议摘要卡：

1. `拓扑类型`：PostgreSQL Streaming Replication。
2. `当前角色`：Primary / Standby / 延迟 Standby / 未知。
3. `Standby 数量`：已连接或已登记 standby 数量。
4. `同步模式`：sync / async / mixed。
5. `最大 Replay Lag`：拓扑内最大 replay lag。
6. `Replay 状态`：运行 / 暂停 / 未知。

### 8.6 PostgreSQL 风险规则

1. `wal_receiver.status != streaming`：critical 或 warning。
2. `pg_is_wal_replay_paused = true`：warning，若非计划内则 critical。
3. `replay_lag` 超过阈值：warning / critical。
4. `sourceInstanceId == 0`：warning，来源 primary 未匹配。
5. `primary` 无任何 standby：info 或 warning，按生产环境配置决定。
6. 最近采集过旧：warning。

## 9. 前端设计

### 9.1 拓扑页入口文案更新

当前提示：

```text
四期第 1 批支持 Redis Cluster / MongoDB ReplicaSet / Elasticsearch / OpenSearch 的只读拓扑查询，并写入统一审计。
```

建议改为：

```text
拓扑支持 Redis、MongoDB、Elasticsearch / OpenSearch 以及 MySQL / PostgreSQL 主从关系的只读采集，所有查询写入统一审计。
```

空状态也要从：

```text
请先选择 Redis、MongoDB、Elasticsearch 或 OpenSearch 实例
```

改为：

```text
请选择已开通拓扑权限和拓扑能力的数据库实例
```

### 9.2 图形化关系视图

建议在摘要卡片和表格之间加入拓扑图区域。

实现建议：

1. 复用项目内已经使用的 ECharts，不新增图形依赖。
2. 使用 ECharts `graph` series。
3. 节点使用数据库类型和角色决定样式。
4. 边使用状态决定颜色。
5. 图下保留节点表和链路表。

布局：

```text
拓扑页
  工具栏
  摘要卡片
  风险发现
  拓扑图
  节点表 + 复制关系表
  分片表，仅搜索类数据库显示
```

### 9.3 节点视觉规则

节点形态：

| 角色 | 颜色 | 说明 |
| --- | --- | --- |
| primary / master | 绿色 | 当前主节点 |
| replica / standby | 蓝色 | 普通副本 |
| delayed_replica / delayed_standby | 橙色 | 延迟副本 |
| sentinel / arbiter | 黄色 | 协调或仲裁节点 |
| unknown | 灰色 | 未识别或未匹配 |
| critical | 红色边框 | 节点异常 |

节点标题：

```text
实例名
角色 · 地址
```

节点副标题：

```text
lag 3s · healthy
```

### 9.4 边视觉规则

边方向：

```text
primary -> replica
```

边文案：

1. MySQL：`async · lag 3s · IO Yes / SQL Yes`
2. PostgreSQL：`streaming · async · replay 12ms`
3. Redis：保留现有关系展示，后续可统一方向。
4. MongoDB：`replicates · healthy`

边颜色：

1. 绿色：healthy。
2. 黄色：warning。
3. 红色：critical / failed / disconnected。
4. 灰色：unknown。

### 9.5 风险发现面板

新增 `topologyResult.findings` 展示：

```text
发现 3 项拓扑风险
```

每条 finding 展示：

1. 风险等级。
2. 标题。
3. 说明。
4. 建议。
5. 关联节点或链路。

示例：

```text
异常：mysql-replica-01 SQL apply 线程异常
说明：Replica_SQL_Running = No，复制 SQL 线程停止。
建议：进入副本治理查看最近采集和错误信息，确认是否需要恢复 apply。
```

### 9.6 节点详情抽屉

点击节点打开抽屉。

抽屉内容：

1. 基本信息：
   - 实例名。
   - 地址。
   - 引擎。
   - 版本。
   - 环境。
   - 负责人。
2. 角色状态：
   - 当前角色。
   - 健康状态。
   - 最近采集时间。
3. 复制指标：
   - MySQL IO/SQL。
   - MySQL seconds behind source。
   - PostgreSQL sync_state。
   - PostgreSQL replay lag。
   - PostgreSQL replay paused。
4. 风险说明。
5. 快捷入口：
   - 查看副本治理。
   - 查看最近采集。
   - 刷新当前实例复制状态。

### 9.7 拓扑页操作

建议保留低风险操作：

1. 刷新拓扑。
2. 重新采集当前实例。
3. 跳转副本治理。
4. 只看异常。
5. 只看延迟副本。
6. 复制拓扑摘要。

不建议放在拓扑首屏的操作：

1. 暂停 apply。
2. 恢复 apply。
3. promote。
4. failover。
5. 任意 SQL 执行。

## 10. API 返回示例

### 10.1 MySQL 主从拓扑

```json
{
  "instanceId": 1,
  "instanceName": "mysql-primary",
  "dbType": "mysql",
  "dbTypeText": "MySQL",
  "topologyType": "mysql_replication",
  "topologyTypeText": "MySQL Replication",
  "collectedAt": "2026-05-04 16:30:00",
  "cards": [
    {
      "key": "role",
      "label": "当前角色",
      "value": "主库",
      "description": "当前选择实例检测到的复制角色"
    },
    {
      "key": "replicas",
      "label": "副本数量",
      "value": "2",
      "description": "已归并到该主库的从库数量"
    },
    {
      "key": "max_lag",
      "label": "最大延迟",
      "value": "5s",
      "description": "拓扑内最大复制延迟"
    }
  ],
  "nodes": [
    {
      "id": "instance:1",
      "name": "mysql-primary",
      "role": "primary",
      "roleText": "主库",
      "address": "10.0.0.1:3306",
      "state": "healthy",
      "version": "8.0.35",
      "lagText": "",
      "message": "",
      "metrics": {
        "server_uuid": "uuid-primary",
        "read_only": "OFF",
        "super_read_only": "OFF",
        "gtid_mode": "ON"
      },
      "updatedAt": "2026-05-04 16:30:00"
    },
    {
      "id": "instance:2",
      "name": "mysql-replica-01",
      "role": "replica",
      "roleText": "从库",
      "address": "10.0.0.2:3306",
      "state": "healthy",
      "version": "8.0.35",
      "lagText": "5s",
      "message": "IO/SQL running",
      "metrics": {
        "replica_io_running": "Yes",
        "replica_sql_running": "Yes",
        "seconds_behind_source": "5"
      },
      "updatedAt": "2026-05-04 16:30:00"
    }
  ],
  "links": [
    {
      "source": "instance:1",
      "target": "instance:2",
      "sourceName": "mysql-primary",
      "targetName": "mysql-replica-01",
      "label": "async replication",
      "state": "healthy",
      "lagText": "5s",
      "message": "IO Yes / SQL Yes"
    }
  ],
  "findings": [],
  "message": "MySQL 主从拓扑读取成功"
}
```

### 10.2 PostgreSQL Primary / Standby 拓扑

```json
{
  "instanceId": 11,
  "instanceName": "pg-primary",
  "dbType": "postgresql",
  "dbTypeText": "PostgreSQL",
  "topologyType": "postgresql_streaming_replication",
  "topologyTypeText": "PostgreSQL Streaming Replication",
  "collectedAt": "2026-05-04 16:30:00",
  "cards": [
    {
      "key": "role",
      "label": "当前角色",
      "value": "Primary",
      "description": "当前选择实例检测到的复制角色"
    },
    {
      "key": "standbys",
      "label": "Standby 数量",
      "value": "2",
      "description": "已连接或已归并 standby 数量"
    },
    {
      "key": "max_replay_lag",
      "label": "最大 Replay Lag",
      "value": "12 ms",
      "description": "拓扑内最大 replay lag"
    }
  ],
  "nodes": [
    {
      "id": "instance:11",
      "name": "pg-primary",
      "role": "primary",
      "roleText": "Primary",
      "address": "10.0.1.1:5432",
      "state": "healthy",
      "version": "15.5",
      "metrics": {
        "in_recovery": "false",
        "standby_count": "2"
      },
      "updatedAt": "2026-05-04 16:30:00"
    },
    {
      "id": "instance:12",
      "name": "pg-standby-01",
      "role": "standby",
      "roleText": "Standby",
      "address": "10.0.1.2:5432",
      "state": "healthy",
      "lagText": "12 ms",
      "metrics": {
        "sync_state": "async",
        "state": "streaming",
        "replay_lag": "12 ms"
      },
      "updatedAt": "2026-05-04 16:30:00"
    }
  ],
  "links": [
    {
      "source": "instance:11",
      "target": "instance:12",
      "sourceName": "pg-primary",
      "targetName": "pg-standby-01",
      "label": "streaming async",
      "state": "healthy",
      "lagText": "12 ms"
    }
  ],
  "findings": [],
  "message": "PostgreSQL Streaming Replication 拓扑读取成功"
}
```

## 11. 实施批次

### 11.1 第一批：MySQL / PostgreSQL 拓扑接入

目标：让 MySQL / MariaDB / PostgreSQL 能进入拓扑页，并展示主从节点和复制关系。

后端：

1. `SupportedTypes()` 打开 MySQL / MariaDB / PostgreSQL 的 `TopologyEnabled`。
2. 将 `collectDatabaseTopology` 改为 `UseCase` 方法。
3. 新增 `collectMySQLReplicationTopology`。
4. 新增 `collectPostgreSQLReplicationTopology`。
5. 复用 `DatabaseInstanceReplica` 和最近 `DatabaseReplicationCheck` 构建完整节点和链路。
6. 扩展 `DatabaseTopologyLinkVO` 可选字段。
7. 增加 `DatabaseTopologyFindingVO`。
8. 增加单元测试：
   - MySQL replica check 转 topology node。
   - PostgreSQL replication rows 转 topology links。
   - 未匹配主库生成虚拟节点。
   - finding 风险生成。

前端：

1. 更新拓扑页提示文案。
2. 更新 API 类型，补充 link 可选字段和 findings。
3. 节点表增加健康、lag、采集时间展示。
4. 复制关系表增加 `lagText`、`message`。
5. 增加风险发现面板。

验收：

1. MySQL / MariaDB / PostgreSQL 出现在拓扑实例选择器。
2. 选择 MySQL 主库能看到从库关系。
3. 选择 MySQL 从库能看到上游主库。
4. 选择 PostgreSQL primary 能看到 standby。
5. 选择 PostgreSQL standby 能看到上游 primary。
6. 拓扑查询写入 `topology_view` 审计。

### 11.2 第二批：图形化拓扑视图

目标：把拓扑从表格升级为“图 + 表”的日常排障视图。

前端：

1. 使用 ECharts graph 渲染 `nodes` 和 `links`。
2. 节点按角色和健康状态着色。
3. 边按健康状态和 lag 着色。
4. 支持只看异常。
5. 支持只看当前链路。
6. 点击节点打开详情抽屉。
7. 点击边展示复制详情。

验收：

1. MySQL / PostgreSQL 主从图可直观看到主从方向。
2. 异常链路红色展示。
3. 延迟副本橙色展示。
4. 点击节点能看到 metrics 明细。
5. 图形为空时有明确空状态。

### 11.3 第三批：副本治理打通

背景：第一批和第二批已经完成后，拓扑页已经能展示 MySQL / MariaDB / PostgreSQL 主从关系、延迟副本、关系图、风险发现和最大延迟。第三批不再继续堆大范围新能力，重点补齐“拓扑可解释性”和“排障动作闭环”。

目标：拓扑页能成为副本排障入口，但不承载高风险动作。用户在拓扑中看到异常后，应能直接知道“哪个节点异常、为什么异常、数据是否新鲜、下一步去哪里处理”。

边界：

1. 拓扑页允许低风险动作：刷新拓扑、采集相关实例、查看副本详情、跳转副本治理、查看原始采集。
2. 拓扑页不直接执行 pause / resume apply，不做 promote、failover、主从切换。
3. 拓扑页不替代副本治理页；副本治理页仍承载动作审批、原始记录、事故指引和操作审计。
4. 不新增长期拓扑快照表；第三批继续复用 `database_instance_replicas` 和 `database_replication_checks`。

#### 11.3.1 节点 / 链路详情抽屉

当前拓扑页已经有图、节点表和复制关系表，但节点细节主要压在 `message`、`lagText` 和 `metrics` 中。第三批建议点击节点或复制关系后打开详情抽屉。

节点详情建议展示：

1. 基础信息：
   - 实例名。
   - 实例 ID。
   - 地址。
   - 数据库类型。
   - 角色。
   - 健康状态。
   - 最近采集时间。
2. 复制状态：
   - MySQL / MariaDB：`Replica_IO_Running`、`Replica_SQL_Running`、`Seconds_Behind_Source`、`SQL_Delay`、`SQL_Remaining_Delay`。
   - PostgreSQL：`wal_receiver_status`、`pg_is_wal_replay_paused`、`pg_replay_lag_ms`、`pg_last_wal_replay_lsn`、`recovery_min_apply_delay`。
3. 来源信息：
   - `source_host`。
   - `source_port`。
   - `source_server_uuid`。
   - `primary_instance_id`。
   - `replica_instance_id`。
4. 排障入口：
   - 查看最近采集。
   - 查看副本治理。
   - 刷新该实例采集。
   - 复制关键指标。

链路详情建议展示：

1. 源节点和目标节点。
2. 复制关系类型：`async replication`、`delayed replication`、`streaming replication`。
3. 当前延迟。
4. 链路状态。
5. IO / SQL 或 WAL receiver 状态。
6. 最近采集 ID。
7. 最近错误摘要。

前端实现建议：

1. 在 `DatabaseManagement.vue` 中新增 `topologyDetailVisible`、`selectedTopologyNode`、`selectedTopologyLink`。
2. ECharts 节点和边增加 click 事件：
   - 点击 node 打开节点详情。
   - 点击 edge 打开链路详情。
3. 节点表和复制关系表增加“详情”操作列。
4. metrics 使用统一格式化函数展示，避免直接把 JSON 平铺成难读文本。

#### 11.3.2 拓扑页联动副本治理

第三批最重要的体验优化是把拓扑风险和副本治理页连接起来。

建议入口：

1. 风险项右侧增加“处理”按钮。
2. 节点详情增加“查看副本治理”按钮。
3. 复制关系详情增加“查看副本关系”按钮。
4. 节点详情增加“查看原始采集”按钮。
5. 节点详情增加“采集该实例”按钮。

跳转行为：

1. 点击“查看副本治理”：
   - `activeTab = 'replication'`
   - 设置 `replicationReplicaQuery.instanceId`
   - 加载副本关系列表。
2. 点击“查看原始采集”：
   - 如果有 `check_id`，打开最近采集详情。
   - 如果没有 `check_id`，提示先采集该实例。
3. 点击“处理风险”：
   - 根据 finding 类型跳到对应区域：
     - `collection`：副本状态采集列表。
     - `replication`：副本关系列表。
     - `replica_relation`：副本关系列表。
   - 自动带上实例筛选。

注意事项：

1. 拓扑页只做跳转和采集，不直接执行暂停 / 恢复。
2. 如果用户没有副本治理权限，按钮置灰并提示权限不足。
3. 跳转后要保留当前拓扑结果，方便用户回来继续看。

#### 11.3.3 采集新鲜度和批量采集相关实例

当前拓扑实时采集当前选中实例，其他相关节点来自最近一次 `database_replication_checks`。如果副本很久没有采集，拓扑可能显示旧状态。

建议增加采集新鲜度能力：

1. 后端计算每个节点最近采集年龄：
   - `last_check_id`
   - `last_checked_at`
   - `check_age_seconds`
   - `check_freshness`: `fresh` / `stale` / `unknown`
2. 默认阈值：
   - 5 分钟内：fresh。
   - 5 到 30 分钟：warning。
   - 超过 30 分钟：stale。
   - 没有采集记录：unknown。
3. 拓扑摘要卡增加：
   - `最近采集`：拓扑内最新采集时间。
   - `过期节点`：超过阈值的节点数量。
4. findings 增加：
   - `副本状态采集已过期`。
   - `副本尚无采集记录`。
5. 前端工具栏增加：
   - `采集当前实例`。
   - `采集相关实例`。

采集相关实例行为：

1. 从当前拓扑结果中提取 `nodes` 的实例 ID。
2. 过滤 MySQL / MariaDB / PostgreSQL 实例。
3. 调用已有 `checkDatabaseReplication(instanceId)`。
4. 成功后自动刷新拓扑。
5. 前端显示成功 / 失败数量。

验收标准：

1. 主库拓扑中，副本节点超过 30 分钟未采集会出现 warning finding。
2. 点击“采集相关实例”后，主库和所有已纳管副本都会重新采集。
3. 采集完成后拓扑自动刷新。

#### 11.3.4 MySQL 角色安全检查

MySQL 从库如果没有启用只读保护，拓扑应提示风险。第三批建议补充 MySQL / MariaDB 节点级变量采集。

后端采集变量：

```sql
SELECT @@server_id;
SELECT @@server_uuid;
SELECT @@version;
SELECT @@read_only;
SELECT @@super_read_only;
SELECT @@log_bin;
SELECT @@gtid_mode;
SELECT @@binlog_format;
SELECT @@binlog_row_image;
```

兼容要求：

1. MariaDB 或旧版本字段不存在时不能导致拓扑失败。
2. 每个变量单独容错采集，失败时写入 metrics 中的 error 摘要。
3. 变量只进入脱敏后的 `metrics`，不写敏感连接信息。

新增风险规则：

1. `role=replica` 且 `read_only=OFF`：warning，提示从库未开启只读保护。
2. `role=replica` 且 `super_read_only=OFF`：info 或 warning，按 MySQL 版本和权限能力提示。
3. `role=primary` 且 `log_bin=OFF`：warning，提示主库未开启 binlog，不利于复制和 PITR。
4. `server_id` 缺失或为 0：warning，提示复制配置不完整。

前端展示：

1. 节点详情抽屉展示 `read_only`、`super_read_only`、`server_id`、`server_uuid`。
2. 风险发现中展示“从库未开启只读保护”。
3. 关系图中不强行把只读风险映射为红色，建议 warning 黄色即可。

#### 11.3.5 延迟副本保护窗口汇总

当前拓扑已经能识别延迟副本，但保护窗口摘要还可以更直接。建议从主库视角汇总所有延迟副本。

后端摘要卡建议：

1. `延迟副本`：延迟副本数量。
2. `最小保护窗口`：所有延迟副本中最小 `remaining_delay_seconds`。
3. `推荐保护副本`：剩余窗口最大且健康的延迟副本。
4. `保护状态`：`受保护` / `降级` / `未保护`。

计算规则：

1. 只统计 `replica_role=delayed_replica` 或 `delayed_standby`。
2. 健康副本优先。
3. `remaining_delay_seconds < 0` 视为未知，不参与最小窗口计算，但生成 warning。
4. `remaining_delay_seconds == 0` 表示延迟副本已追上，生成 warning。
5. 没有延迟副本时，主库保护状态为 `未保护`。

前端展示：

1. 摘要卡突出显示保护状态。
2. 延迟副本节点 label 或 tooltip 中展示剩余窗口。
3. 详情抽屉展示推荐保护副本和当前可用窗口。

#### 11.3.6 MySQL 主库侧辅助发现

MySQL 主库端不能可靠发现所有从库，但可以做辅助发现，减少“从库已经起来但拓扑没显示”的困惑。

建议采集：

```sql
SHOW REPLICAS;
```

兼容降级：

```sql
SHOW SLAVE HOSTS;
```

使用方式：

1. 仅在当前实例被识别为 primary 时执行。
2. 将返回的 host / port / server_id 与 OpsHub 已纳管实例匹配。
3. 匹配成功：补充关系。
4. 匹配失败：创建虚拟未纳管副本节点。
5. 如果没有配置 `report_host` / `report_port`，不生成错误，只在 message 中提示“主库侧发现依赖 report_host/report_port”。

限制：

1. 不能把 `SHOW REPLICAS` 作为唯一数据源。
2. 仍以从库主动采集和 `database_instance_replicas` 为准。
3. 辅助发现的关系应标记 `discovery_source=mysql_primary_reported`。

验收标准：

1. 从库配置了 `report_host` 后，主库拓扑能看到未采集过的从库提示。
2. 未匹配实例显示为灰色虚拟节点。
3. 点击虚拟节点能看到“未纳管 / 未匹配”的说明。

#### 11.3.7 关系图布局优化

当前 ECharts 使用 force layout，节点少时可用，但主从链路变多后位置不稳定。第三批建议对 MySQL / PostgreSQL 使用确定性布局。

布局规则：

1. 主库固定在左侧或中心。
2. 普通从库放右侧上半区。
3. 延迟副本放右侧下半区。
4. 未匹配来源或未纳管节点放灰色边缘区域。
5. PostgreSQL primary 的 runtime standby 和已纳管 standby 尽量合并展示。

边 label 规则：

1. MySQL 普通复制：`async · 0s · IO Yes / SQL Yes`。
2. MySQL 延迟复制：`delayed · lag 12s · remain 58m`。
3. PostgreSQL：`streaming · async · replay 12ms`。
4. 未知延迟：`async · lag unknown`。

验收标准：

1. 1 主 1 从、1 主多从、1 主多普通从 + 延迟从，图形布局稳定。
2. 节点不会互相覆盖。
3. 边 label 不遮挡主要节点。
4. 窄屏下图形仍可滚动缩放。

#### 11.3.8 后端返回字段建议

短期可以继续放在 `metrics` 中，避免 API 破坏性变更。建议统一以下 key：

节点 metrics：

```json
{
  "instance_id": "1",
  "replica_id": "4",
  "check_id": "109",
  "role_detected": "replica",
  "health_status": "healthy",
  "source_host": "192.168.1.12",
  "source_port": "23306",
  "server_id": "2",
  "server_uuid": "xxx",
  "read_only": "ON",
  "super_read_only": "ON",
  "last_checked_at": "2026-05-04 20:52:29",
  "check_age_seconds": "120",
  "check_freshness": "fresh"
}
```

链路 metrics：

```json
{
  "replica_id": "4",
  "check_id": "109",
  "io": "Yes",
  "sql": "Yes",
  "seconds_behind_source": "0",
  "configured_delay_seconds": "3600",
  "remaining_delay_seconds": "1800",
  "discovery_source": "replica_status"
}
```

后续如果前端依赖增多，再考虑将 `replicaId`、`checkId`、`freshness` 提升为一等字段。

#### 11.3.9 第三批实施顺序

建议按以下顺序实施：

1. 采集新鲜度和采集相关实例。
2. 节点 / 链路详情抽屉。
3. 拓扑页跳转副本治理和原始采集。
4. MySQL 角色安全检查。
5. 延迟副本保护窗口汇总。
6. 关系图确定性布局。
7. MySQL 主库侧辅助发现。

优先级理由：

1. 新鲜度和批量采集解决“拓扑是否可信”的问题。
2. 详情抽屉解决“为什么异常”的问题。
3. 跳转副本治理解决“下一步怎么处理”的问题。
4. MySQL 角色安全检查能直接发现生产从库未只读这类高价值风险。
5. 延迟副本保护窗口对误删防护最有价值，但依赖前面采集准确性。
6. 主库侧辅助发现有用，但受 MySQL `report_host` 配置限制，不能作为第一优先级。

第三批验收：

1. 从拓扑节点能打开详情抽屉并看到关键 metrics。
2. 从复制关系能打开链路详情并定位到 `replica_id` 和 `check_id`。
3. 从 finding 能跳转到副本治理，并自动带上实例筛选。
4. 点击“采集相关实例”后，相关 MySQL / MariaDB / PostgreSQL 实例完成副本状态采集并刷新拓扑。
5. 采集过期节点会产生 warning finding。
6. MySQL 从库 `read_only=OFF` 时会产生 warning finding。
7. 延迟副本能在摘要卡看到保护状态和最小保护窗口。
8. 拓扑页没有 pause / resume / promote / failover 直接执行按钮。

#### 11.3.10 第三批落地记录

本次落地范围：

1. 后端拓扑 metrics：
   - 节点和链路补充 `instance_id`、`primary_instance_id`、`replica_instance_id`、`check_id`、`last_check_id`。
   - 节点和链路补充 `last_checked_at`、`check_age_seconds`、`check_freshness`。
   - `check_freshness` 使用 5 分钟 fresh、5 到 30 分钟 warning、超过 30 分钟 stale、无采集 unknown。
2. 后端风险发现：
   - stale / warning / unknown 采集状态生成 collection finding。
   - MySQL / MariaDB 采集 `server_id`、`server_uuid`、`version`、`read_only`、`super_read_only`、`log_bin`、`gtid_mode`、`binlog_format`、`binlog_row_image`。
   - MySQL / MariaDB 增加 `从库未开启只读保护`、`从库未开启 super_read_only`、`主库未开启 binlog`、`server_id 配置无效` 风险提示。
3. 后端摘要卡：
   - 增加 `最近采集`、`过期节点`、`延迟副本`、`最小保护窗口`、`保护状态`、`推荐保护副本`。
   - 从主库视角汇总所有已识别延迟副本的剩余保护窗口。
4. MySQL 主库侧辅助发现：
   - 主库识别为 primary 时读取 `SHOW REPLICAS`，失败后降级 `SHOW SLAVE HOSTS`。
   - 返回结果进入 `reported_replicas`。
   - 拓扑中未匹配到 OpsHub 实例时生成灰色虚拟从库节点，`discovery_source=mysql_primary_reported`。
5. 前端拓扑页：
   - 工具栏增加 `采集当前实例` 和 `采集相关实例`。
   - findings 增加 `处理` 入口，跳转副本治理并带上实例筛选。
   - 节点表和复制关系表增加 `详情` 操作。
   - 点击 ECharts 节点或边打开详情抽屉。
   - 详情抽屉展示基础信息、状态、延迟、最近采集和 metrics，并提供 `采集该实例`、`副本治理`、`原始采集`。
   - MySQL / MariaDB / PostgreSQL 拓扑图使用确定性布局：主库、普通副本、延迟副本分区展示；其他类型仍保留 force layout。

仍保留在后续批次的内容：

1. 拓扑页不执行 pause / resume / promote / failover。
2. 不新增拓扑快照表。
3. 不做人工绑定主从关系。
4. 不在拓扑页直接配置复制链路。

### 11.4 第四批：拓扑历史和人工治理

目标：解决自动发现不完整和拓扑变化追踪。

可选能力：

1. 拓扑快照。
2. 拓扑历史对比。
3. 手工绑定主从关系。
4. 未匹配主库治理。
5. 拓扑导出。
6. 巡检报告引用拓扑 findings。

这一批不是第一优先级，建议等主从拓扑稳定后再做。

## 12. 测试计划

### 12.1 后端单元测试

新增或扩展：

1. `internal/biz/database/topology_test.go`
2. `internal/biz/database/replication_test.go`

测试场景：

1. MySQL primary 无从库。
2. MySQL primary 有两个 replica。
3. MySQL delayed replica 有剩余延迟。
4. MySQL replica IO running = No。
5. MySQL replica SQL running = No。
6. MySQL source host 未匹配 OpsHub 实例。
7. PostgreSQL primary 有 sync standby 和 async standby。
8. PostgreSQL standby WAL receiver streaming。
9. PostgreSQL standby replay paused。
10. PostgreSQL primary_conninfo 未匹配 OpsHub 实例。

### 12.2 前端验证

1. `npm run typecheck`
2. `npm run build`
3. 拓扑页空状态。
4. MySQL 主从拓扑展示。
5. PostgreSQL 主从拓扑展示。
6. findings 展示。
7. 节点详情抽屉。
8. 移动端或窄屏布局不重叠。

### 12.3 集成验证

至少准备：

1. MySQL primary + replica。
2. MySQL delayed replica。
3. PostgreSQL primary + standby。
4. PostgreSQL standby replay paused 的测试环境。
5. 未匹配主库的从库实例。

验收命令：

```bash
go test ./internal/biz/database/...
npm run typecheck
npm run build
```

## 13. 风险和注意事项

1. MySQL 主库端不能可靠发现所有从库，必须结合从库主动采集和归并关系。
2. PostgreSQL `pg_stat_replication` 中的 `application_name` 不一定和 OpsHub 实例名一致，需要通过 host、port、system identifier 辅助匹配。
3. 一些账号权限可能无法读取复制状态，需要明确错误信息并生成 warning finding。
4. MariaDB 与 MySQL 变量和字段名存在差异，采集逻辑要容错。
5. 拓扑图不要替代表格，图用于快速定位，表格用于排障。
6. 不应在拓扑首屏放置高风险操作，避免误触。
7. 拓扑查询会访问数据库实例，仍需写入 `topology_view` 审计。

## 14. 推荐优先级

第一批和第二批完成后，下一步建议进入第三批，优先做“可信度 + 详情 + 跳转”的闭环：

1. 采集新鲜度和“采集相关实例”。
2. 节点 / 链路详情抽屉。
3. findings、节点、链路跳转副本治理。
4. MySQL 角色安全检查：`read_only`、`super_read_only`、`server_id`、`log_bin`。
5. 延迟副本保护窗口汇总。
6. 关系图确定性布局。
7. MySQL 主库侧辅助发现。

这批完成后，拓扑页会从“主从关系展示”升级为“主从排障入口”。用户看到异常后，可以在同一条链路上完成确认、采集、定位和跳转治理，但高风险动作仍保留在副本治理页。
