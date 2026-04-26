# 虚拟化平台管理一期：表结构与接口草案

## 一期目标
1. 接入 `vCenter` / `独立 ESXi` / `PVE` 的只读资产发现。
2. 支持平台、集群、宿主机、虚拟节点的持久化和展示。
3. 支持虚拟节点纳管为现有主机资产并保持映射关系。

## 数据库表结构草案

## 1. `virtualization_platforms`
用途：虚拟化平台连接与同步配置。

```sql
CREATE TABLE IF NOT EXISTS virtualization_platforms (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  name              VARCHAR(100) NOT NULL COMMENT '平台名称',
  provider          VARCHAR(20)  NOT NULL COMMENT 'vcenter/esxi/pve',
  endpoint          VARCHAR(255) NOT NULL COMMENT '平台地址（IP/域名）',
  port              INT          NOT NULL DEFAULT 443,
  auth_type         VARCHAR(20)  NOT NULL DEFAULT 'password' COMMENT 'password/token',
  username          VARCHAR(128) DEFAULT NULL,
  secret_ciphertext TEXT         DEFAULT NULL COMMENT '加密密文（密码或token）',
  skip_tls_verify   TINYINT      NOT NULL DEFAULT 0,
  datacenter_filter VARCHAR(255) DEFAULT NULL COMMENT '可选，逗号分隔',
  cluster_filter    VARCHAR(255) DEFAULT NULL COMMENT '可选，逗号分隔',
  node_filter       VARCHAR(255) DEFAULT NULL COMMENT '可选，逗号分隔',
  status            VARCHAR(20)  NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  last_sync_at      DATETIME     DEFAULT NULL,
  last_sync_status  VARCHAR(20)  DEFAULT NULL COMMENT 'success/failed/running',
  last_sync_error   TEXT         DEFAULT NULL,
  created_by        BIGINT UNSIGNED DEFAULT NULL,
  updated_by        BIGINT UNSIGNED DEFAULT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_provider (provider),
  KEY idx_status (status),
  KEY idx_deleted_at (deleted_at),
  UNIQUE KEY uk_provider_endpoint_port (provider, endpoint, port, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 2. `virtualization_clusters`
用途：平台中的数据中心/集群逻辑层。

```sql
CREATE TABLE IF NOT EXISTS virtualization_clusters (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  platform_id       BIGINT UNSIGNED NOT NULL,
  external_id       VARCHAR(191) NOT NULL COMMENT '平台侧唯一ID',
  name              VARCHAR(191) NOT NULL,
  cluster_type      VARCHAR(20)  NOT NULL DEFAULT 'cluster' COMMENT 'datacenter/cluster/pool',
  parent_external_id VARCHAR(191) DEFAULT NULL,
  path              VARCHAR(500) DEFAULT NULL COMMENT '拓扑路径',
  metadata_json     JSON DEFAULT NULL,
  synced_at         DATETIME NOT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_platform_id (platform_id),
  KEY idx_deleted_at (deleted_at),
  UNIQUE KEY uk_platform_external (platform_id, external_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 3. `virtualization_hosts`
用途：平台中的宿主机节点。

```sql
CREATE TABLE IF NOT EXISTS virtualization_hosts (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  platform_id       BIGINT UNSIGNED NOT NULL,
  cluster_id        BIGINT UNSIGNED DEFAULT NULL,
  external_id       VARCHAR(191) NOT NULL,
  name              VARCHAR(191) NOT NULL,
  management_ip     VARCHAR(64)  DEFAULT NULL,
  hypervisor_type   VARCHAR(20)  NOT NULL COMMENT 'esxi/pve',
  cpu_cores         INT          DEFAULT NULL,
  memory_bytes      BIGINT       DEFAULT NULL,
  status            VARCHAR(20)  DEFAULT NULL COMMENT 'online/offline/unknown',
  metadata_json     JSON DEFAULT NULL,
  synced_at         DATETIME NOT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_platform_id (platform_id),
  KEY idx_cluster_id (cluster_id),
  KEY idx_deleted_at (deleted_at),
  UNIQUE KEY uk_platform_external (platform_id, external_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 4. `virtualization_guests`
用途：虚拟机 / 容器资产清单。

```sql
CREATE TABLE IF NOT EXISTS virtualization_guests (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  platform_id       BIGINT UNSIGNED NOT NULL,
  cluster_id        BIGINT UNSIGNED DEFAULT NULL,
  host_id           BIGINT UNSIGNED DEFAULT NULL,
  external_id       VARCHAR(191) NOT NULL COMMENT '平台侧唯一ID（UUID/VMID）',
  guest_type        VARCHAR(20)  NOT NULL COMMENT 'vm/lxc',
  name              VARCHAR(191) NOT NULL,
  power_state       VARCHAR(30)  DEFAULT NULL COMMENT 'running/stopped/suspended/...',
  os_name           VARCHAR(255) DEFAULT NULL,
  guest_ip          VARCHAR(64)  DEFAULT NULL COMMENT '主IP（为空允许）',
  guest_ips_json    JSON DEFAULT NULL COMMENT 'IP数组',
  cpu_cores         INT          DEFAULT NULL,
  memory_bytes      BIGINT       DEFAULT NULL,
  disk_bytes        BIGINT       DEFAULT NULL,
  is_template       TINYINT      NOT NULL DEFAULT 0,
  metadata_json     JSON DEFAULT NULL,
  synced_at         DATETIME NOT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_platform_id (platform_id),
  KEY idx_cluster_id (cluster_id),
  KEY idx_host_id (host_id),
  KEY idx_guest_type (guest_type),
  KEY idx_power_state (power_state),
  KEY idx_guest_ip (guest_ip),
  KEY idx_deleted_at (deleted_at),
  UNIQUE KEY uk_platform_external (platform_id, external_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 5. `virtualization_guest_bindings`
用途：虚拟节点与主机资产的纳管映射。

```sql
CREATE TABLE IF NOT EXISTS virtualization_guest_bindings (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  guest_id          BIGINT UNSIGNED NOT NULL,
  host_asset_id     BIGINT UNSIGNED NOT NULL COMMENT 'hosts.id',
  bind_status       VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT 'active/unbound/conflict',
  bind_source       VARCHAR(20) NOT NULL DEFAULT 'manual' COMMENT 'manual/auto',
  bind_note         VARCHAR(500) DEFAULT NULL,
  created_by        BIGINT UNSIGNED DEFAULT NULL,
  updated_by        BIGINT UNSIGNED DEFAULT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_guest_id (guest_id),
  KEY idx_host_asset_id (host_asset_id),
  KEY idx_deleted_at (deleted_at),
  UNIQUE KEY uk_guest_active (guest_id, bind_status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 6. `virtualization_sync_jobs`
用途：记录同步任务历史和结果。

```sql
CREATE TABLE IF NOT EXISTS virtualization_sync_jobs (
  id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  platform_id       BIGINT UNSIGNED NOT NULL,
  trigger_type      VARCHAR(20) NOT NULL COMMENT 'manual/schedule',
  status            VARCHAR(20) NOT NULL COMMENT 'pending/running/success/failed',
  started_at        DATETIME DEFAULT NULL,
  finished_at       DATETIME DEFAULT NULL,
  summary_json      JSON DEFAULT NULL COMMENT '新增/更新/删除统计',
  error_message     TEXT DEFAULT NULL,
  created_by        BIGINT UNSIGNED DEFAULT NULL,
  created_at        DATETIME NOT NULL,
  updated_at        DATETIME NOT NULL,
  deleted_at        DATETIME DEFAULT NULL,
  KEY idx_platform_id (platform_id),
  KEY idx_status (status),
  KEY idx_created_at (created_at),
  KEY idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 同步策略草案
1. 平台同步采用 `全量拉取 + upsert`。
2. 每次同步写一条 `virtualization_sync_jobs`。
3. 对本轮未出现的对象打 `stale`（通过 `synced_at` 判定），不立即硬删除。
4. 纳管绑定表不随同步任务删除，只在节点彻底失效后标记冲突。

## 接口草案（REST）

## 1. 平台管理
1. `GET /api/v1/virtualization/platforms`
2. `POST /api/v1/virtualization/platforms`
3. `GET /api/v1/virtualization/platforms/{id}`
4. `PUT /api/v1/virtualization/platforms/{id}`
5. `DELETE /api/v1/virtualization/platforms/{id}`
6. `POST /api/v1/virtualization/platforms/{id}/test`
7. `POST /api/v1/virtualization/platforms/{id}/sync`
8. `GET /api/v1/virtualization/platforms/{id}/sync-jobs`

### `POST /api/v1/virtualization/platforms` 请求示例
```json
{
  "name": "生产-PVE",
  "provider": "pve",
  "endpoint": "10.0.10.12",
  "port": 8006,
  "authType": "token",
  "username": "root@pam",
  "secret": "<token_or_password>",
  "skipTlsVerify": true,
  "clusterFilter": "cluster-a,cluster-b",
  "status": "enabled"
}
```

## 2. 拓扑与清单
1. `GET /api/v1/virtualization/topology?platformId={id}`
2. `GET /api/v1/virtualization/guests`
3. `GET /api/v1/virtualization/guests/{id}`

### `GET /api/v1/virtualization/guests` 查询参数
1. `platformId`
2. `clusterId`
3. `hostId`
4. `guestType` (`vm`/`lxc`)
5. `powerState`
6. `managed` (`true`/`false`)
7. `keyword`（名称、IP、外部ID）
8. `page`、`pageSize`

### 清单返回示例
```json
{
  "total": 2,
  "list": [
    {
      "id": 101,
      "platformId": 1,
      "platformName": "生产-PVE",
      "clusterName": "cluster-a",
      "hostName": "pve-node-01",
      "externalId": "qemu/105",
      "guestType": "vm",
      "name": "order-api-01",
      "powerState": "running",
      "osName": "Ubuntu 22.04",
      "guestIp": "10.10.1.25",
      "managed": true,
      "hostAssetId": 88,
      "syncedAt": "2026-04-19 10:20:30"
    }
  ]
}
```

## 3. 纳管接口
1. `POST /api/v1/virtualization/guests/{id}/onboard`
2. `POST /api/v1/virtualization/guests/batch-onboard`
3. `POST /api/v1/virtualization/guests/{id}/unbind`

### 单个纳管请求示例
```json
{
  "groupId": 12,
  "credentialId": 32,
  "sshUser": "root",
  "port": 22,
  "nameOverride": "order-api-01",
  "onConflict": "skip"
}
```

### 单个纳管响应示例
```json
{
  "guestId": 101,
  "hostAssetId": 88,
  "bindStatus": "active"
}
```

## 4. 错误码草案
1. `VIRT_PLATFORM_UNREACHABLE`：平台不可达。
2. `VIRT_AUTH_FAILED`：认证失败。
3. `VIRT_SYNC_RUNNING`：同步任务正在执行。
4. `VIRT_GUEST_NO_IP`：虚拟节点缺少可纳管IP。
5. `VIRT_ONBOARD_CONFLICT`：纳管冲突（IP重复、已有绑定等）。

## 适配层接口草案（后端内部）
```go
type VirtualizationProviderAdapter interface {
    TestConnection(ctx context.Context, cfg PlatformConfig) error
    ListClusters(ctx context.Context, cfg PlatformConfig) ([]ClusterDTO, error)
    ListHosts(ctx context.Context, cfg PlatformConfig) ([]HostDTO, error)
    ListGuests(ctx context.Context, cfg PlatformConfig) ([]GuestDTO, error)
}
```

## 约束与边界
1. 一期只保证发现和纳管，不提供开关机、快照、迁移等写操作。
2. `guestIp` 允许为空；纳管时必须校验可用 IP。
3. 同步不会覆盖主机资产中的人工字段（描述、标签、分组等）。
4. 平台凭据必须加密存储，不在任何接口明文回传。
