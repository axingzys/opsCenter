# 虚拟化拓扑视图功能完善与前端美化方案

更新时间：2026-04-26

## 1. 背景

当前拓扑视图已经从早期卡片式拓扑切换为接近 Proxmox VE 的服务器视图：

- 左侧为资源树：Datacenter / Cluster / Node / VM。
- 右侧为对象工作区：概要、资源清单、趋势图、同步任务。
- 后端拓扑接口已经返回宿主机基础资源字段和宿主机下 VM 列表。
- 终端入口不放在拓扑页，由已有终端能力承接。

这版已经能解决“旧拓扑不好看、层级不清晰”的第一层问题，但离可长期使用的运维视图还有差距。后续重点不是继续堆卡片，而是把它做成一个稳定、高密度、可操作、可排障的虚拟化资源工作台。

## 2. 当前问题

### 2.1 功能问题

1. 资源树只有服务器视图，缺少按状态、按纳管、按类型、按标签/资源池等视角。
2. 概要 Tab 目前只是视觉元素，`监控 / 资源 / 任务` 尚未切换出独立内容。
3. VM 详情仍偏基础，只展示 CPU、内存、电源、IP、纳管状态，缺少磁盘、网卡、系统、Tools/Agent、快照、绑定资产等信息。
4. Node 详情缺少 PVE 常见信息，例如运行时间、平均负载、CPU 型号、内核版本、PVE 版本、磁盘空间、网络流量、Swap、KSM、存储状态。
5. 趋势图目前来自平台/集群级虚机数量快照，不能表达 PVE 截图里的 CPU、内存、负载、网络、磁盘 IO 等监控。
6. 同步任务只展示最近记录，没有和资源变更、操作审计、Provider Task 关联。
7. 搜索只在前端过滤当前拓扑数据，缺少高亮、定位、搜索结果数量和空状态引导。
8. ESXi/vCenter 还没有专属 inventory 视图，后续不能强行套 PVE Node/VM 模型。

### 2.2 体验问题

1. 页面视觉已接近 PVE，但还有明显“仿 PVE 外壳 + OpsHub 内容”的割裂感。
2. 左侧树宽度、行高、状态点、类型文本还可以更紧凑、更清晰。
3. 右侧图表在数据点少时显得空，缺少“当前值 + 趋势 + 数据不足”的组合表达。
4. VM 选中后工作区仍偏大块布局，信息密度可以提高。
5. 资源清单没有排序、筛选、列宽控制，也没有快速定位到纳管/异常 VM。
6. 任务区域在页面底部，不够像 PVE 的底部任务栏；长页面时可见性不足。
7. 页面没有明确展示“数据来自最近一次同步”还是“实时采集/监控数据”。

### 2.3 数据问题

1. 当前 topology API 主要是 inventory 数据，不是实时监控数据。
2. PVE `cluster/resources` 可拿到资源现状，但很多节点/VM 详情需要继续调用节点状态、VM status/current、storage、network、task 等 API。
3. 当前数据库表还没有 datastore/storage、network、tag、pool、snapshot、provider task 等清单模型。
4. 图表指标缺少统一 metric schema，后续 PVE 和 vSphere 的指标需要归一化。

## 3. 目标

### 3.1 产品目标

1. 做成虚拟化平台的日常入口，而不是只读拓扑图。
2. PVE 平台优先对齐 Proxmox VE 的使用习惯。
3. ESXi/vCenter 单独走 vSphere inventory，不和 PVE 混用展示模型。
4. 页面能同时服务三类场景：
   - 快速查看平台资源状态。
   - 定位异常节点/虚机。
   - 从虚机进入纳管、审计、快照、操作等工作流。

### 3.2 技术目标

1. 拓扑视图组件继续从大页面拆分，避免 `VirtualizationPlatforms.vue` 再次膨胀。
2. 拓扑数据分为 inventory、metrics、tasks、audit 四类接口。
3. 前端按 provider 分层：PVE 使用 PVE workspace，vSphere 使用 vSphere workspace。
4. 先兼容现有表和接口，再逐步新增 storage/network/task/metric 表。
5. 所有新 UI 保持响应式，但优先保证桌面端运维体验。

## 4. 信息架构

### 4.1 页面总结构

```text
顶部页面区
  标题 / 策略 / 写操作 / 新增平台

拓扑 Tab
  拓扑工具栏
    视图模式
    平台选择
    Provider 过滤
    搜索
    数据新鲜度
    时间范围
    刷新

  主工作台
    左侧资源树
    中间对象工作区
    右侧上下文面板（可折叠，后续）
    底部任务栏（可固定，后续）
```

### 4.2 PVE 服务器视图

```text
Datacenter
  PVE Platform
    Cluster
      Node
        QEMU VM
        LXC CT
      Storage
      Network
      Pool
      Tag
```

第一轮继续只展示 Node / QEMU / LXC；第二轮补 Storage / Network；第三轮补 Pool / Tag。

### 4.3 vSphere 视图

```text
vCenter / ESXi
  Datacenter
    Cluster
      Host
        Virtual Machine
    Resource Pool
    Datastore
    Network / Port Group
```

vSphere 需要单独组件，不复用 PVE 的 Node/CT 文案：

- `Node` 改为 `Host`。
- `QEMU/LXC` 改为 `Virtual Machine`。
- 增加 `Datastore`、`Network`、`Resource Pool`。
- 后续补 HA/DRS、Maintenance Mode、Tools、Snapshot 等字段。

## 5. 视图模式设计

### 5.1 服务器视图

用途：默认视图，适合日常运维。

展示：

- 平台 / 集群 / 节点 / 虚机层级。
- 节点显示状态、VM 数、CPU/内存压力。
- VM 显示电源状态、类型、纳管状态。

### 5.2 资源类型视图

用途：按对象类型快速定位。

PVE：

```text
Node
QEMU VM
LXC CT
Storage
Network
```

vSphere：

```text
Cluster
Host
Virtual Machine
Datastore
Network
Resource Pool
```

### 5.3 状态视图

用途：异常排查。

分组：

- 运行中
- 已停止
- 挂起
- 离线
- 未纳管
- 同步异常
- 数据过期

### 5.4 纳管视图

用途：批量推进虚机纳管。

分组：

- 已纳管
- 未纳管
- 冲突
- 建议自动匹配
- 缺少 IP

该视图需要和“虚机纳管”Tab 打通：在树中选中 VM 后，可以直接打开纳管预检查弹窗。

### 5.5 标签/资源池视图

用途：按业务组织资源。

PVE 可从 tag/pool 读取；vSphere 可从 folder/resource pool/tag 读取。当前阶段先预留入口，等数据模型补齐后启用。

## 6. 对象工作区设计

### 6.1 Platform / Datacenter 工作区

概要：

- 平台类型、地址、账号、同步状态、最近同步时间。
- Cluster、Node/Host、VM、已纳管 VM 数。
- 开机/关机/挂起比例。
- 在线/离线/未配置比例。
- 同步任务状态。

监控：

- 虚机总量趋势。
- 电源状态趋势。
- 纳管状态趋势。
- 同步耗时/失败趋势。

资源：

- 节点/宿主机列表。
- VM Top N：CPU/内存/磁盘占用最高。
- 未纳管 VM 列表。

任务：

- 最近同步任务。
- 最近操作审计。
- Provider Task（后续）。

### 6.2 Node / Host 工作区

PVE Node 需要展示：

- 节点名、管理 IP、状态、运行时间。
- CPU 型号、核心数、CPU 使用率、平均负载。
- 内存总量、使用量、Swap。
- 磁盘空间、存储状态。
- 网络流量、磁盘 IO。
- PVE 版本、内核版本。
- VM/CT 清单。

vSphere Host 需要展示：

- 连接状态、维护模式。
- CPU/内存容量和使用率。
- Datastore 挂载状态。
- Network/Port Group。
- VM 清单。

### 6.3 VM / CT 工作区

概要：

- VMID / Instance UUID / MoID。
- 名称、类型、宿主节点、状态。
- CPU、内存、磁盘、网卡。
- IP、OS、Tools/Agent 状态。
- 纳管状态、绑定资产主机。

监控：

- CPU 使用率。
- 内存使用率。
- 磁盘 IO。
- 网络流量。

资源：

- 磁盘列表。
- 网卡列表。
- 快照列表。
- 标签/资源池。

任务：

- 电源操作记录。
- 快照操作记录。
- 控制台访问审计。
- 纳管/解绑记录。

## 7. 数据接口规划

### 7.1 保留现有接口

```text
GET /api/v1/virtualization/topology
GET /api/v1/virtualization/platforms/{id}/trend
GET /api/v1/virtualization/clusters/{id}/trend
GET /api/v1/virtualization/platforms/{id}/sync-jobs
GET /api/v1/virtualization/action-logs
GET /api/v1/virtualization/guests
```

### 7.2 新增拓扑详情接口

```text
GET /api/v1/virtualization/topology/objects/{type}/{id}
```

用途：点击资源树对象后获取详情，避免 topology 一次返回过大。

返回对象：

- `platform`
- `cluster`
- `host`
- `guest`
- `storage`
- `network`
- `pool`
- `tag`

### 7.3 新增实时/近实时指标接口

```text
GET /api/v1/virtualization/hosts/{id}/metrics?range=1h
GET /api/v1/virtualization/guests/{id}/metrics?range=1h
```

指标字段：

```text
cpu_usage_percent
memory_usage_percent
memory_used_bytes
memory_total_bytes
disk_read_bytes
disk_write_bytes
network_in_bytes
network_out_bytes
load_1m
load_5m
load_15m
```

### 7.4 新增资源清单接口

```text
GET /api/v1/virtualization/hosts/{id}/storages
GET /api/v1/virtualization/hosts/{id}/networks
GET /api/v1/virtualization/guests/{id}/disks
GET /api/v1/virtualization/guests/{id}/nics
GET /api/v1/virtualization/guests/{id}/snapshots
```

快照接口已有基础能力，后续需要统一展示字段。

### 7.5 新增 Provider Task 接口

```text
GET /api/v1/virtualization/platforms/{id}/provider-tasks
GET /api/v1/virtualization/guests/{id}/provider-tasks
```

PVE 可读取 task log；vSphere 可接 EventManager / TaskManager。

## 8. 数据模型规划

### 8.1 建议新增表

```text
virtualization_storages
virtualization_networks
virtualization_guest_disks
virtualization_guest_nics
virtualization_provider_tasks
virtualization_host_metrics
virtualization_guest_metrics
virtualization_resource_tags
virtualization_resource_pools
```

### 8.2 同步策略

分三类同步：

1. Inventory 同步：平台、集群、宿主机、VM、存储、网络。
2. Metrics 采集：CPU、内存、磁盘、网络、负载。
3. Task/Event 同步：Provider task、操作审计、资源变更事件。

建议频率：

- Inventory：60 秒。
- Metrics：30 秒或 60 秒。
- Task/Event：15 秒或操作后立即拉取。

## 9. 前端组件拆分

当前组件：

```text
web/src/views/asset/components/VirtualizationTopologyExplorer.vue
```

建议拆分为：

```text
web/src/views/asset/virtualization/topology/
  VirtualizationTopologyExplorer.vue
  PveTopologyWorkspace.vue
  VsphereTopologyWorkspace.vue
  TopologyToolbar.vue
  TopologyResourceTree.vue
  TopologyObjectHeader.vue
  TopologySummaryPanel.vue
  TopologyResourceTable.vue
  TopologyMetricGrid.vue
  TopologyTaskBar.vue
  TopologyEmptyState.vue
  composables/
    useTopologySelection.ts
    useTopologyTree.ts
    useTopologyMetrics.ts
    useTopologyCharts.ts
  types.ts
```

拆分原则：

- `VirtualizationTopologyExplorer` 只负责数据装配和 provider 分发。
- `PveTopologyWorkspace` 管 PVE 视图。
- `VsphereTopologyWorkspace` 管 vSphere 视图。
- 资源树、图表、任务栏独立，便于后续复用。

## 10. 前端视觉美化方向

### 10.1 整体风格

保持 PVE 的高密度、工具型风格，但不要完全照搬旧 ExtJS 质感。

设计原则：

- 少装饰，多信息。
- 面板边界清晰。
- 字号克制。
- 色彩用于状态，不用于大面积装饰。
- 表格和树必须适合长时间运维使用。

### 10.2 色彩

建议状态色：

```text
运行中 / 在线：#2f9e44
停止 / 未知：#8a939d
警告 / 挂起：#d98c00
错误 / 离线：#d64545
选中：#1976d2
PVE 趋势主色：#9dbb3c
背景：#f2f4f7
面板：#ffffff
边框：#cfd4dc
```

### 10.3 左侧资源树

优化项：

1. 支持折叠/展开状态持久化。
2. 搜索命中高亮。
3. 搜索后自动展开命中路径。
4. 不同对象使用不同图标。
5. VM 行展示 VMID、名称、类型、状态。
6. 异常对象加醒目标识。
7. 右侧计数改为稳定宽度，避免行抖动。

### 10.4 对象头部

展示：

- 对象图标。
- 对象名称。
- 对象路径：Platform / Cluster / Node / VM。
- 状态 Tag。
- 最近采集/同步时间。
- 快捷操作：查看虚机、查看趋势、纳管、快照、审计。

### 10.5 概要面板

优化项：

1. 将 CPU/内存/磁盘/纳管率做成 PVE 风格横向进度条。
2. 当前值与总量右对齐。
3. 数据缺失时显示 `暂无采集数据`，不要显示误导性 0。
4. 高风险指标显示 warning/danger 状态。

### 10.6 图表

优化项：

1. 数据点少时使用柱状/阶梯线，不强行铺满趋势线。
2. 每个图表顶部显示当前值。
3. 支持 `1h / 24h / 7d / 15d`。
4. 鼠标悬浮显示时间、指标、单位。
5. 空数据时说明原因：未同步、未开启采集、数据不足。

### 10.7 任务栏

目标接近 PVE 底部 Log Panel：

- 固定在拓扑工作区底部，可折叠。
- Tab：同步任务、操作审计、Provider Task。
- 显示状态、开始时间、耗时、对象、操作人、失败原因。
- 支持点击任务查看详情。

## 11. 交互设计

### 11.1 选择联动

1. 点击树节点后更新对象工作区。
2. 选中 Cluster/Node 时自动切换趋势 scope。
3. 选中 VM 时展示 VM 详情，但趋势默认仍可显示所属 Node/Cluster。
4. 点击资源清单行时同步选中左侧树节点。

### 11.2 搜索

搜索范围：

- 平台名。
- 集群名。
- 节点名。
- VMID。
- VM 名称。
- IP。
- 资产主机名称。

行为：

- 输入后即时过滤。
- 显示命中数量。
- 支持 Enter 跳转下一个命中。
- 清空搜索恢复展开状态。

### 11.3 快捷操作

对象级操作：

- Platform：同步、查看同步任务、查看审计。
- Cluster：查看虚机、查看趋势。
- Node/Host：查看虚机、查看监控、查看任务。
- VM：纳管、解绑、开机/关机/重启、快照、控制台、审计。

高风险操作继续受写操作总开关和审计规则控制。

## 12. 实施阶段

### 阶段 A：当前 PVE 视图打磨

目标：

- 修正当前页面细节和视觉密度。
- 让概要/资源/监控/任务 Tab 真正可切换。
- 优化左侧资源树和 VM 详情页。

任务：

1. 拆分 `VirtualizationTopologyExplorer.vue`。
2. 增加对象路径、数据新鲜度、采集来源提示。
3. 完成 Tab 内容切换。
4. VM 详情补磁盘、网卡、绑定资产、快照入口占位。
5. 图表空状态和低数据量展示优化。
6. 任务栏固定到底部并支持折叠。

验收：

- PVE 平台 10 到 100 台 VM 场景下页面可用。
- 选中 Platform/Cluster/Node/VM 时内容明确变化。
- 搜索、树选中、表格选中三者联动正常。

### 阶段 B：PVE 数据补齐

目标：

- 补齐 Node/VM 的运维基础信息。
- 支持更接近 PVE Summary 的节点详情。

任务：

1. PVE adapter 增加 node status/current 采集。
2. PVE adapter 增加 qemu/lxc status/current 采集。
3. 增加 storage 清单。
4. 增加 network/nic/disk 清单。
5. 增加 host/guest metrics 存储和查询。

验收：

- Node 页面能看到 CPU、内存、Swap、负载、磁盘、网络。
- VM 页面能看到磁盘、网卡、IP、运行状态、纳管状态。
- 监控图表能展示真实资源指标，而不是只有 VM 数量。

### 阶段 C：纳管视图和异常视图

目标：

- 把拓扑和虚机纳管打通。
- 支持快速找出未纳管、冲突、离线、数据过期对象。

任务：

1. 增加纳管视图模式。
2. 增加状态视图模式。
3. 树节点支持按风险状态着色。
4. VM 行提供纳管/预检查快捷入口。
5. 工作区增加“未纳管 VM 列表”和“建议匹配资产”。

验收：

- 用户能在拓扑页直接找出未纳管 VM。
- 用户能从 VM 打开纳管预检查。
- 严格/宽松策略在拓扑快捷操作中生效。

### 阶段 D：vSphere 专属视图

目标：

- 不强行复用 PVE 视图。
- 支持 vCenter / ESXi inventory。

任务：

1. 新增 `VsphereTopologyWorkspace.vue`。
2. 树层级改为 vCenter / Datacenter / Cluster / Host / VM。
3. 增加 Datastore、Network、Resource Pool 占位。
4. Host 页面展示连接状态、维护模式、容量。
5. VM 页面展示 Tools、Snapshot、Datastore、Network。

验收：

- ESXi 单机可正常展示 Host/VM。
- vCenter 多 Datacenter/Cluster 可正常分层。
- PVE 和 vSphere 文案、层级、字段不混用。

### 阶段 E：任务与审计工作台

目标：

- 拓扑页能看到平台正在发生什么。

任务：

1. 底部任务栏固定化。
2. 同步任务、操作审计、Provider Task 三类合并展示。
3. 支持点击任务查看请求、结果、错误、耗时。
4. 操作完成后触发快速同步并刷新当前对象。

验收：

- 用户执行开关机/快照/纳管后，拓扑页能看到审计记录。
- Provider 侧失败原因能展示在任务详情里。

## 13. 优先级

### P0

1. 当前 PVE 页面视觉打磨。
2. Tab 真正切换内容。
3. 左侧树搜索/选中/展开体验。
4. VM 详情补充纳管和资源字段。
5. 任务栏固定与折叠。

### P1

1. PVE Node/VM 实时资源指标。
2. Storage/Network 清单。
3. 纳管视图、状态视图。
4. 图表数据质量和空状态。

### P2

1. vSphere 专属视图。
2. Provider Task。
3. Pool/Tag 视图。
4. 热力图和关系图。

## 14. 验收清单

### 功能验收

1. PVE 服务器视图能展示 Platform/Cluster/Node/VM。
2. 点击任意层级，右侧内容切换正确。
3. VM 详情能展示基础资源、纳管状态和快捷入口。
4. Node 详情能展示基础容量和 VM 清单。
5. 搜索能定位 VMID、名称和 IP。
6. 任务栏能看到最近同步和操作记录。
7. 数据不足时有明确说明，不误导用户。

### UI 验收

1. 1920x1080 下不出现主要区域错位。
2. 1366x768 下左树和工作区仍可用。
3. 表格列不挤压关键字段。
4. 图表不会撑破面板。
5. 状态色和选中态一致。
6. 页面整体更接近 PVE 的高密度工作台，而不是营销式卡片页面。

### 性能验收

1. 100 台 VM 内切换节点无明显卡顿。
2. 500 台 VM 场景下搜索和树渲染仍可接受。
3. topology 首屏接口不返回过重详情。
4. 图表按需加载，不一次性渲染所有对象指标。

## 15. 建议下一步

建议下一轮先做阶段 A：

1. 拆分当前拓扑组件。
2. 完成概要/监控/资源/任务 Tab 切换。
3. 优化左侧树和 VM 详情。
4. 固定底部任务栏。
5. 调整视觉细节，让页面更像一个真正可用的运维工作台。

阶段 A 不需要先改数据库，主要是前端结构和交互完善，风险最低，也能最快改善你当前看到的页面效果。

## 16. 实施记录

### 2026-04-26 阶段 A 第一批

已完成：

- 写入本方案文档。
- 将 `概要 / 监控 / 资源 / 任务` 做成真实可切换内容区。
- 增加对象路径、数据新鲜度、筛选后的 VM 数量、对象范围标签。
- 监控图表增加当前值摘要和数据不足空态。
- 底部任务栏支持折叠。

### 2026-04-26 阶段 A 第二批

已完成：

- 左侧资源树增加状态视图和纳管视图。
- 资源 Tab 增加 VMID / 名称 / IP、电源状态、纳管状态过滤。
- 对象详情增加抽屉，集中展示路径、基础信息、容量与虚机运维占位。
- 同步任务行支持点击查看任务详情和失败原因。

仍需后端数据补齐：

- VM 磁盘、网卡、快照、绑定资产详情。
- PVE Node/VM 实时 CPU、内存、网络、磁盘 IO 指标。
- Provider Task 和操作审计合并展示。
