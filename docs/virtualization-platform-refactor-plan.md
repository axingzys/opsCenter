# 虚拟化平台模块重构方案

更新时间：2026-04-26

## 1. 背景和目标

当前虚拟化平台模块已经具备 PVE / ESXi 的平台接入、手动同步、拓扑展示、虚机列表、虚机纳管、开关机、快照、控制台链接和操作审计雏形，但实现上存在几个明显问题：

- 平台管理、同步、资源清单、纳管、操作审计集中在一个大 UseCase 和一个大 Vue 页面中，后续维护成本高。
- 平台数据依赖用户手动同步，拓扑和虚机列表容易滞后。
- 同步过程在 HTTP 请求内直接执行，缺少后台任务、并发锁、重试、超时、stale 标记和同步策略。
- 拓扑视图只是平台、集群、宿主机的卡片堆叠，缺少 Proxmox VE / vSphere 这种高效的资源树和对象工作区。
- PVE 和 ESXi / vCenter 的资源模型并不完全相同，当前展示方式没有体现 provider 差异。
- 纳管目前偏向“绑定已有资产”，没有完整覆盖“从虚机创建资产并绑定”的场景。
- 操作审计已有基础表，但写操作还没有统一任务化，难以追踪 provider 侧 task id 和操作前后状态。
- 平台凭据仍以 password/token 字段保存，后续需要迁移到加密密文和 auth_type 模型。

重构目标：

- 数据自动同步：平台接入后默认自动同步，页面无需依赖人工刷新。
- Provider 分层：PVE 使用 Proxmox VE 风格资源树，vCenter / ESXi 使用 vSphere inventory 风格。
- 拓扑可用：默认视图适合日常运维，辅助视图适合大规模排障和关系探索。
- 纳管完整：支持绑定已有主机资产，也支持基于虚机创建资产并绑定。
- 审计可靠：所有高风险动作进入统一 operation job / action log 流水。
- 渐进落地：先用现有数据表和 provider adapter 落地低风险能力，再逐步升级模型。

## 2. 外部平台参考

### 2.1 Proxmox VE

参考链接：https://pve.proxmox.com/pve-docs/chapter-pve-gui.html

可借鉴点：

- 左侧 Resource Tree 是主入口。
- 默认 Server View 按 Datacenter -> Node -> Guest / Storage / Pool 展示。
- 支持 Folder View、Pool View、Tag View，用不同组织方式查看同一批资源。
- 选中资源树对象后，右侧 Content Panel 展示该对象的 Summary、配置、状态、任务等。
- 底部 Log Panel 展示集群实时任务，用户能看到后台正在发生什么。

落地到 OpsHub：

- PVE 平台默认使用 Proxmox 风格的资源树。
- 左侧可切换 `服务器视图`、`资源类型视图`、`资源池视图`、`标签视图`。
- 中间工作区根据选中对象切换：平台总览、节点详情、虚机详情、存储/池/标签聚合。
- 底部或右侧固定展示最近同步任务、操作任务和审计日志。

### 2.2 VMware vCenter / vSphere

参考链接：https://www.vmware.com/docs/vmware-cloud-on-aws-vcenter-architecture

可借鉴点：

- vCenter inventory 以 Datacenter、Cluster、Host、VM 为核心。
- 同时需要 Resource Pool、Datastore、Network 这些对象来解释资源和权限边界。
- Cluster 表达 HA / DRS / 资源池能力，Host 表达 ESXi 物理承载能力，VM 表达运行实例。

落地到 OpsHub：

- vCenter / ESXi 不强行套 PVE Node / Guest 结构。
- vCenter 模式展示：vCenter -> Datacenter -> Cluster -> Host -> VM。
- 单 ESXi 模式展示：ESXi -> Host -> VM，同时保留 datastore / network 信息。
- 中间工作区优先展示 Cluster / Host 容量、VM 分布、连接状态、维护状态。

### 2.3 Datadog Host Map

参考链接：https://docs.datadoghq.com/infrastructure/hostmap/

可借鉴点：

- 使用 host map / heatmap 快速观察大量资源。
- 颜色表示 CPU、内存、错误、健康状态。
- 大小表示容量、错误数、虚机数等。
- 支持按标签、区域、集群等分组。

落地到 OpsHub：

- 增加 `资源热力图` 辅助视图。
- 宿主机热力图：颜色表示 CPU / 内存 / 同步状态 / 告警，大小表示虚机数量或资源总量。
- 虚机热力图：颜色表示电源状态 / 纳管状态 / 运行健康，大小表示 CPU / 内存。
- 按平台、集群、标签、资源池、纳管状态分组。

### 2.4 Grafana Node Graph

参考链接：https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/visualizations/node-graph/

可借鉴点：

- 适合展示对象之间的关系和依赖。
- 支持节点详情、边详情、缩放、平移、网格布局。
- 不适合做默认资源管理入口，适合作为选中对象后的探索视图。

落地到 OpsHub：

- 增加 `关系图` 辅助视图。
- 默认从选中 VM / Host 展开，不一次性渲染全量对象。
- 关系包括 VM -> Host -> Cluster -> Datastore -> Network -> Bound Asset Host。
- 大规模资源场景默认隐藏低价值节点，通过点击展开。

### 2.5 Proxmox Datacenter Manager Views

参考链接：https://pdm.proxmox.com/docs/views.html

可借鉴点：

- include / exclude 过滤系统。
- 支持按 resource-type、resource-pool、tag、remote、resource-id 过滤。
- 可以把权限授予某个 view。

落地到 OpsHub：

- 平台同步策略和拓扑视图都支持 include / exclude 过滤。
- 后续可以将视图和 RBAC 绑定，例如只允许某个团队查看某些平台、集群、标签或资源池。

## 3. 目标架构

建议将虚拟化模块拆成以下边界：

```text
internal/biz/virtualization
  platform      平台配置、凭据、连接探测、能力识别
  sync          自动同步、增量同步、同步锁、stale 标记
  inventory     平台资源清单：datacenter、cluster、host、guest、datastore、network、tag
  topology      PVE/vSphere/heatmap/relationship 的聚合数据
  onboarding    纳管预检查、绑定、创建主机资产、解绑
  operation     开关机、快照、控制台、异步任务
  audit         同步日志、操作审计、资源变更日志

internal/data/virtualization
  repository    虚拟化模块独立数据访问层

internal/provider/virtualization
  pve           PVE API adapter
  vmware        govmomi adapter，覆盖 vCenter / ESXi

internal/service/virtualization
  handler       HTTP API 层

web/src/views/asset/virtualization
  PlatformManagement.vue
  TopologyExplorer.vue
  GuestInventory.vue
  OnboardingDialog.vue
  OperationDrawer.vue
  AuditLogTable.vue
```

第一阶段不直接大搬迁目录，先在现有模块内落地自动同步基础和前端拓扑改造；等行为稳定后再拆包，减少一次性改动风险。

## 4. 自动同步设计

### 4.1 同步策略

平台默认启用自动同步。同步策略分为全局默认和平台覆盖：

```text
global:
  enabled: true
  interval_seconds: 60
  initial_delay_seconds: 15
  platform_timeout_seconds: 300
  stale_after_minutes: 10
  archive_after_days: 7

platform override:
  enabled: true/false
  interval_seconds: 30/60/300
  include_filters
  exclude_filters
```

当前阶段先支持全局配置；后续增加每个平台独立 sync_policy。

### 4.2 同步触发

触发来源：

- `schedule`：后台调度器周期触发。
- `manual`：用户点击立即同步。
- `operation`：开关机、快照、纳管完成后触发一次快速同步。
- `event`：vCenter EventManager / PropertyCollector 事件触发。

第一阶段：

- 增加后台调度器，按固定间隔扫描 enabled 平台并触发同步。
- 保留手动同步接口。
- 同一平台使用同步锁，避免手动同步和自动同步并发覆盖。
- 每个平台同步设置超时，避免一个平台卡死阻塞后续同步。

第二阶段：

- vCenter / ESXi 接入 govmomi PropertyCollector / EventManager。
- PVE 保持短周期轮询，同时拉取 task log 补足操作状态。
- 操作完成后触发单平台快速同步。

### 4.3 同步状态

平台状态字段：

```text
last_sync_at
last_sync_status: idle/running/success/failed
last_sync_message
sync_lag_seconds
next_sync_at
```

同步任务字段：

```text
platform_id
trigger_type: manual/schedule/operation/event
status: pending/running/success/failed/skipped
started_at
finished_at
items_total
items_created
items_updated
items_deleted
failure_reason
provider_task_id
```

### 4.4 数据写入规则

- 同步写入必须按平台隔离，禁止跨平台覆盖。
- upsert key 使用 `(platform_id, external_id)`。
- 缺失资源第一步标记为 `stale`，不立即硬删除。
- 绑定状态和人工维护字段不被同步覆盖。
- 同步过程中产生的资源快照和指标快照需要同一时间戳，方便趋势展示。

## 5. 拓扑视图设计

### 5.1 页面结构

```text
顶部工具栏
  平台选择 / Provider 过滤 / 搜索 / 自动同步状态 / 立即同步

左侧资源树
  根据 provider 展示 PVE 或 vSphere 层级

中间工作区
  Summary
  Resource workspace
  Metrics / heatmap / relationship graph

右侧详情抽屉
  当前选中对象的属性、纳管状态、快捷操作、最近审计

底部任务栏
  最近同步任务 / 操作任务 / provider task
```

### 5.2 PVE 拓扑

PVE 使用 Proxmox 风格：

```text
Datacenter
  Platform / Remote
    Node
      QEMU VM
      LXC CT
    Storage
    Pool
    Tag
```

视图模式：

- 服务器视图：按 Node 分组展示 VM / CT。
- 类型视图：按 VM、LXC、Storage、Node 分组。
- 资源池视图：按 Pool 分组。
- 标签视图：按 Tag 分组。

详情工作区：

- Datacenter / Platform：平台状态、节点数、虚机数、同步状态、资源趋势。
- Node：CPU、内存、磁盘、网络、运行 VM/CT 数、维护状态。
- Guest：电源状态、IP、OS、纳管状态、快照入口、控制台入口。

阶段 2 实际落地的 PVE 服务器视图：

```text
顶部工具栏
  服务器视图 / 平台选择 / 资源搜索 / 自动同步状态 / 趋势时间范围 / 刷新

左侧资源树
  数据中心
    PVE Cluster
      Node
        QEMU VM / LXC CT

右侧概要工作区
  对象标题和状态
  概要页签
  CPU、内存、运行虚机、纳管率
  当前对象基础信息
  当前对象资源清单
  虚机总量、电源状态、纳管状态、运行连通状态趋势
  最近同步任务
```

当前阶段不放终端入口，终端能力由已有模块承接。拓扑接口返回宿主机下的 VM 列表，并补充宿主机 CPU、内存、采集时间、平台地址等基础字段，前端直接以这些字段构造 Proxmox 风格的服务器概要页。

### 5.3 vSphere 拓扑

vCenter / ESXi 使用 vSphere inventory 风格：

```text
vCenter / ESXi
  Datacenter
    Cluster
      Host
        Virtual Machine
    Resource Pool
    Datastore
    Network
```

视图模式：

- Inventory View：Datacenter -> Cluster -> Host -> VM。
- Resource Pool View：按资源池组织 VM。
- Datastore View：按 datastore 展示 VM 文件和容量风险。
- Network View：按 network / port group 展示 VM 网络。

详情工作区：

- Datacenter：Cluster、Host、VM、Datastore、Network 总览。
- Cluster：HA / DRS / 主机容量 / VM 分布。
- Host：连接状态、维护模式、CPU/内存、VM 列表。
- VM：电源、IP、Tools、快照、纳管和控制台操作。

### 5.4 热力图

用途：

- 大规模资源快速巡检。
- 按异常状态、资源水位、纳管状态定位问题。

交互：

- fill by：CPU、内存、同步状态、运行状态、纳管状态。
- size by：虚机数、CPU 核数、内存、告警数。
- group by：平台、集群、宿主机、资源池、标签。

### 5.5 关系图

用途：

- 从单个 VM / Host 出发看依赖关系。
- 排查某台虚机所在宿主、集群、存储、网络、绑定资产。

节点：

```text
platform
datacenter
cluster
host
guest
datastore
network
asset_host
```

边：

```text
contains
runs_on
uses_storage
attached_to_network
bound_to_asset
```

## 6. 平台管理重构

平台表最终需要支持：

```text
provider: pve/vcenter/esxi
provider_mode: pve_cluster/single_esxi/vcenter
endpoint
port
tls_verify
proxy
auth_type: password/api_token/session/certificate
secret_ciphertext
secret_version
capabilities_json
sync_policy_json
include_filters_json
exclude_filters_json
status
last_probe_at
last_sync_at
last_error
```

平台操作：

- 创建平台时执行真实认证探测，而不是只做 TCP dial。
- 探测后写入 capabilities，例如是否支持 snapshot、console、event、storage、network。
- 平台禁用后停止自动同步，不再触发操作。
- 删除平台前检查是否存在 active 纳管绑定。
- 更新凭据属于高风险审计动作。

## 7. 资源模型重构

现有模型已有 platform、cluster、host、guest、binding、sync_job、metric、action_log。后续建议补足：

```text
virtualization_datacenters
virtualization_resource_pools
virtualization_datastores
virtualization_networks
virtualization_tags
virtualization_inventory_edges
virtualization_operation_jobs
virtualization_resource_changes
```

资源通用字段：

```text
platform_id
external_id
name
resource_type
status
metadata_json
last_seen_at
stale_at
deleted_at
```

保留原则：

- provider 原始字段进入 metadata_json。
- 常用筛选字段提升为结构化列。
- 不同 provider 的差异不能丢失，但前端不直接读取原始 payload。

## 8. 虚机纳管重构

### 8.1 纳管模式

支持两种纳管：

- 绑定已有主机资产：guest -> existing host。
- 创建主机资产并绑定：guest -> new host -> binding。

### 8.2 预检查

预检查项：

- guest 是否存在。
- guest 是否已绑定 active asset host。
- guest 是否无 IP 或多 IP。
- primary_ip 是否已存在资产。
- hostname / guest name 是否已存在资产。
- provider external_id 是否已绑定历史资产。
- 是否 template。
- 是否 powered_off。
- PVE LXC 与 QEMU 差异。
- vCenter instance_uuid / bios_uuid 稳定性。

预检查结果：

```text
risk_level: low/medium/high/blocker
conflicts
recommendations
can_bind_existing
can_create_asset
default_action
```

### 8.3 绑定关系

binding 应支持历史：

```text
guest_id
asset_host_id
binding_type: manual/auto/import
status: active/inactive
bound_by
bound_at
unbound_by
unbound_at
unbind_reason
source
```

唯一性：

- 同一 guest 只能有一个 active binding。
- 同一 asset host 只能有一个 active virtualization binding，除非明确允许一主机多实例。

## 9. 操作审计重构

所有写操作统一进入 operation job：

```text
power_on
power_off
reboot
shutdown
snapshot_create
snapshot_delete
snapshot_revert
console_link
platform_probe
platform_sync
platform_credential_update
onboard_bind
onboard_create_asset
onboard_unbind
```

operation job 字段：

```text
operation_id
platform_id
target_type
target_id
target_name
action
risk_level
request_payload
before_snapshot
after_snapshot
status
operator_id
operator_name
operator_ip
started_at
finished_at
error_message
provider_task_id
```

行为规则：

- 高风险动作必须有操作原因和二次确认。
- provider task id 必须落库。
- 操作完成后触发单平台快速同步。
- 审计列表支持按平台、对象、动作、状态、操作人、时间筛选。

## 10. API 规划

平台管理：

```text
GET    /api/v1/virtualization/platforms
POST   /api/v1/virtualization/platforms
GET    /api/v1/virtualization/platforms/{id}
PUT    /api/v1/virtualization/platforms/{id}
DELETE /api/v1/virtualization/platforms/{id}
POST   /api/v1/virtualization/platforms/{id}/probe
GET    /api/v1/virtualization/platforms/{id}/capabilities
PUT    /api/v1/virtualization/platforms/{id}/sync-policy
```

同步：

```text
POST /api/v1/virtualization/platforms/{id}/sync
GET  /api/v1/virtualization/sync-runs
GET  /api/v1/virtualization/sync-runs/{id}
GET  /api/v1/virtualization/sync-events
```

资源和拓扑：

```text
GET /api/v1/virtualization/inventory/tree
GET /api/v1/virtualization/topology
GET /api/v1/virtualization/topology/heatmap
GET /api/v1/virtualization/topology/relationships
GET /api/v1/virtualization/guests
GET /api/v1/virtualization/hosts
GET /api/v1/virtualization/datastores
GET /api/v1/virtualization/networks
```

纳管：

```text
POST /api/v1/virtualization/onboarding/precheck
POST /api/v1/virtualization/onboarding/bind-existing
POST /api/v1/virtualization/onboarding/create-asset
POST /api/v1/virtualization/onboarding/unbind
```

操作审计：

```text
POST /api/v1/virtualization/operations
GET  /api/v1/virtualization/operations
GET  /api/v1/virtualization/operations/{id}
GET  /api/v1/virtualization/audit-logs
```

## 11. 前端拆分

当前 `web/src/views/asset/VirtualizationPlatforms.vue` 过大，建议拆分：

```text
web/src/views/asset/virtualization/VirtualizationIndex.vue
web/src/views/asset/virtualization/components/PlatformList.vue
web/src/views/asset/virtualization/components/PlatformFormDialog.vue
web/src/views/asset/virtualization/components/TopologyExplorer.vue
web/src/views/asset/virtualization/components/ProviderResourceTree.vue
web/src/views/asset/virtualization/components/PveResourceTree.vue
web/src/views/asset/virtualization/components/VSphereResourceTree.vue
web/src/views/asset/virtualization/components/ResourceWorkspace.vue
web/src/views/asset/virtualization/components/HeatmapView.vue
web/src/views/asset/virtualization/components/RelationshipGraph.vue
web/src/views/asset/virtualization/components/GuestInventory.vue
web/src/views/asset/virtualization/components/OnboardingDialog.vue
web/src/views/asset/virtualization/components/OperationDrawer.vue
web/src/views/asset/virtualization/components/AuditLogTable.vue
web/src/views/asset/components/VirtualizationPlatformTrendPanel.vue
```

UI 原则：

- 默认是工具型管理界面，不做营销式大卡片。
- 左树、中工作区、右详情、底任务栏是主结构。
- 卡片只用于单个重复资源，不做卡片套卡片。
- 资源数量多时默认折叠和虚拟列表。
- 状态、风险、纳管结果使用稳定色彩，不用单一蓝紫渐变主题。
- 所有操作按钮带明确 icon 和 tooltip。

## 12. 实施阶段

### 阶段 1：自动同步基础和详细方案

目标：

- 新增本方案文档。
- 增加全局虚拟化同步配置。
- 增加后台周期同步调度器。
- 同一平台同步加锁，避免并发覆盖。
- 保留现有手动同步 API。

验收：

- `go test ./internal/biz/asset`
- `go test ./internal/server/asset`
- `npm run build`
- backend / frontend 镜像能构建成功。

### 阶段 2：拓扑前端重构

目标：

- 从大页面中拆出 TopologyExplorer。
- PVE 拓扑使用 Proxmox 风格服务器视图，左侧按 Datacenter / Cluster / Node / VM 展开。
- ESXi / vCenter 使用 vSphere 风格资源树。
- 增加对象概要、资源清单、趋势图和底部任务面板。
- 拓扑 API 补充宿主机基础资源字段和宿主机下 VM 列表。

验收：

- `npm run build`
- `go test ./internal/biz/asset ./internal/server/asset ./internal/service/asset`
- 桌面和移动 viewport 无明显布局重叠。
- PVE / ESXi mock 数据均能正常展示。

### 阶段 3：同步模型增强

目标：

- 增加平台级 sync_policy。
- 增加 stale 标记。
- 增加同步任务 skipped / timeout 状态。
- 同步写入事务化。
- vCenter 优先接入事件增量同步，PVE 保持短轮询。

验收：

- 自动同步不会和手动同步并发覆盖。
- 平台异常时不会阻塞其他平台。
- stale 资源不会被误删绑定关系。

### 阶段 4：纳管重构

目标：

- 增加纳管预检查 API。
- 支持绑定已有资产。
- 支持创建主机资产并绑定。
- binding 支持历史和 active 唯一性。

验收：

- IP 冲突、hostname 冲突、已绑定、无 IP、多 IP 都有明确结果。
- 纳管和解绑均有审计日志。

### 阶段 5：操作任务和审计重构

目标：

- 开关机、快照、控制台链接统一进入 operation job。
- provider task id 落库。
- 操作完成后触发快速同步。
- 审计页面按操作维度重构。

验收：

- 操作失败能保留失败原因。
- 页面能看到 pending/running/success/failed。
- 高风险操作必须有确认和原因。

### 阶段 6：目录拆分和旧代码清理

目标：

- 从 `internal/biz/asset` 拆出 `internal/biz/virtualization`。
- 从 `internal/data/asset/host.go` 拆出 virtualization repository。
- 删除旧拓扑卡片视图和重复逻辑。
- 补齐迁移脚本和升级文档。

验收：

- 全量测试通过。
- 容器构建通过。
- API 兼容清单明确。

## 13. 测试和发布流程

每完成一个阶段：

1. 查看工作区，确认只提交本阶段相关文件。
2. 运行后端测试。
3. 运行前端构建。
4. 构建 backend / frontend 镜像。
5. 必要时用 compose 启动并检查 `/health`。
6. 提交 Git。
7. 推送到当前分支远端。

建议命令：

```bash
go test ./internal/biz/asset ./internal/server/asset
cd web && npm run build
docker build -t opshub-api:dev -f Dockerfile .
docker build -t opshub-web:dev -f Dockerfile.frontend .
git status --short
git add <files>
git commit -m "feat: add virtualization auto sync foundation"
git push origin dev
```

如果某个阶段涉及 DB schema，需要额外验证：

```bash
go test ./...
docker compose up -d mysql redis
docker compose up -d backend frontend
curl -fsS http://127.0.0.1:9876/health
```

## 14. 当前阶段决策

本阶段先做自动同步基础，不立即做完整模型迁移，原因：

- 自动同步是用户最直接的痛点。
- 现有 adapter 已经能 collect PVE / ESXi 数据，可以复用。
- 不改表结构即可先跑通后台同步。
- 拓扑重构可以在已有 topology API 上先做前端改造。

本阶段不做：

- 平台凭据加密迁移。
- provider event stream。
- datastore / network 新表。
- operation job 全量替换。
- 虚机创建资产纳管。

这些进入后续阶段，避免第一阶段改动面过大。
