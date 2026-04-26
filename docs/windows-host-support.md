# OpsHub Windows 主机接入与状态采集技术方案

## 1. 背景

当前项目对 Windows 已经具备一条可用的图形桌面链路：

- 浏览器 -> OpsHub -> Guacamole -> guacd -> RDP -> Windows

但主机资源采集仍然是 Linux/SSH 模型，Windows 主机即使已经能通过 RDP 打开桌面，也无法像 Linux 主机一样稳定显示：

- CPU
- 内存
- 磁盘
- 主机名
- 在线状态
- 运行时长

这不是前端展示问题，而是后端采集链路的能力边界。

## 2. 当前现状与根因

### 2.1 后端采集链路仍然是 SSH

当前 `CollectHostInfo()` 固定执行这条流程：

1. 读取主机 `credentialId`
2. 解密凭据
3. 创建 SSH 客户端
4. 调用采集器执行系统命令
5. 把结果写回 `hosts`

代码位置：

- `internal/biz/asset/host_usecase.go`
- `pkg/ssh`
- `pkg/collector/collector.go`

### 2.2 采集器执行的是 Linux 命令

当前采集器直接执行这些命令：

- `uname -r`
- `uname -m`
- `hostname`
- `lscpu`
- `top -bn1`
- `free -b`
- `df -B1 /`

这意味着它天然假设目标机满足两个条件：

- 能走 SSH
- 能执行 Linux 命令

Windows 主机即使配置了 RDP，只要没有 OpenSSH 或 SSH 凭据，就不可能沿用这条链路。

### 2.3 前端也已经默认承认 Windows 不走自动采集

当前主机编辑页对 Windows 的 SSH 字段已经降成“可选”：

- `SSH端口(可选)`
- `SSH用户名(可选)`
- `SSH凭据(可选)`

并且创建/更新主机后，前端只会对 `osType !== 'windows'` 的主机自动触发采集。

代码位置：

- `web/src/views/asset/Hosts.vue`

这说明当前实现事实上已经把 Windows 视为“有桌面，但没有资源采集”的状态。

## 3. 需求澄清

这次需求不是“让 Windows 也支持 SSH”。

真正的需求是：

1. Windows 主机不依赖 SSH，也能显示 CPU/内存/磁盘等资源信息。
2. Windows 主机的桌面接入和资源采集要解耦。
3. 资源采集要支持手动触发和周期刷新。
4. 方案要适配“大多数 Windows 主机默认没有 SSH”的现实。

结论先行：

- 不应继续沿用 Linux 主机的 SSH 采集方案。
- 也不应试图从 RDP 桌面链路里“顺手拿指标”。
- 应该为 Windows 单独建设“管理/采集通道”。

## 4. 方案对比

| 方案 | 是否依赖目标机改造 | 网络要求 | 复杂度 | 适配度 | 结论 |
| --- | --- | --- | --- | --- | --- |
| 继续复用 SSH | 需要安装并启用 OpenSSH | 入站 22 | 低 | 低 | 不推荐 |
| 从 RDP 链路取指标 | 无 | 入站 3389 | 高 | 极低 | 不可行 |
| WMI/DCOM 直连 | 不装 Agent | RPC/DCOM 动态端口 | 高 | 中低 | 不推荐 |
| WinRM 直连 | 启用 WinRM/PowerShell Remoting | 入站 5985/5986 | 中 | 中高 | 可做第一版 |
| Windows Agent | 安装轻量服务 | 仅需出站 443 | 中高 | 高 | 推荐长期标准方案 |

### 4.1 为什么不推荐继续复用 SSH

因为这条路线只解决“少量安装了 OpenSSH 的 Windows 主机”，无法覆盖大多数目标机。

如果把 Windows 支持建立在 SSH 上，最终会出现两个问题：

- 用户误以为“Windows 已支持”，实际大多数机器不可用
- 后续所有字段、测试连接、错误信息都会继续被 Linux 语义污染

### 4.2 为什么不推荐 WMI/DCOM

WMI/DCOM 理论上能获取指标，但它的运维代价很差：

- 依赖 RPC/DCOM
- 端口范围复杂
- 防火墙策略麻烦
- 跨网段、跨域、零信任环境下体验很差

它适合内网老系统，不适合作为新标准方案。

### 4.3 WinRM 的定位

WinRM 是 Windows 原生远程管理方案，适合作为“无 Agent 第一版”。

优点：

- 不依赖 SSH
- 可以直接执行 PowerShell
- 可以获取系统、CPU、内存、磁盘、服务等信息
- 与当前“凭据 + 主机直连”的资产模型比较接近

缺点：

- 目标机要启用 WinRM
- 需要处理认证模式、TLS、域环境/工作组环境差异
- 对跨网段、跨环境的适配不如 Agent

### 4.4 Agent 的定位

Agent 不是“技术炫技”，而是最适合 Windows 的长期方案。

优点：

- 不需要 SSH
- 不需要开放 WinRM 入站端口
- 只要 Windows 主机能主动访问 OpsHub，就能上报
- 心跳、在线状态、实时刷新都更稳定
- 后续可扩展服务、进程、补丁、事件日志、软件清单

缺点：

- 需要安装一个轻量 Windows 服务
- 首版开发量大于 WinRM

## 5. 推荐结论

推荐采用“双通道”架构：

- 桌面通道：继续使用 `RDP + Guacamole`
- 采集通道：新增 Windows 管理链路，不再依赖 SSH

平台能力上，建议保留多种采集能力共存：

1. 后端同时保留 `AgentCollector` 和 `WinRMCollector`
2. 数据模型新增 `managementMode`
3. Windows 前端管理方式提供：
   - `Agent（推荐）`
   - `WinRM`
   - `SSH（兼容）`
   - `仅桌面`

产品默认上，建议明确站到 Agent 一侧：

1. 新增 Windows 主机时，默认选择 `Agent`
2. UI 上将 `Agent` 标记为“推荐”
3. 后续安装文档、运维手册、部署说明都以 `Agent` 为主路径

迁移阶段上，WinRM 继续保留：

1. 对暂时无法安装 Agent 的机器，用 `WinRM` 顶上
2. 对受管内网、能批量启用 WinRM 的环境，`WinRM` 很适合作为第一版交付路径
3. `SSH` 仅作为兼容模式保留，不再作为 Windows 默认采集通道

运行策略上，建议采用单主模式，而不是双活：

1. 一台 Windows 主机正式运行时，只保留一个主采集模式
2. 迁移期间可以临时切换测试 `Agent / WinRM / SSH`
3. 但不建议长期并行双写同一台主机的状态字段

原因：

- `lastSeen`
- 在线状态
- 采集错误
- 最后采集时间

这些字段如果由两个采集器同时写入，很容易互相覆盖，最终让状态失真

也就是说：

- Windows 桌面连不连，和资源采集不应绑定
- Windows 资源采集不应建立在 SSH 是否存在之上
- 平台能力可以共存，但产品默认应收敛到 `Agent`
- Windows 主机的采集模式应当是“单主模式”，不是“双活模式”

## 6. 推荐架构

### 6.1 总体架构

```text
                +--------------------+
                |   OpsHub Frontend  |
                +--------------------+
                         |
                         v
                +--------------------+
                |   OpsHub Backend   |
                +--------------------+
                  |               |
                  |               |
       RDP Desktop Path     Metrics / Management Path
                  |               |
                  v               v
        Guacamole / guacd   WinRM Collector or Agent Gateway
                  |               |
                  v               v
             Windows Host    Windows Host
```

### 6.2 原则

1. RDP 只负责桌面，不负责资源采集。
2. 资源采集通道必须独立于 RDP。
3. Windows 与 Linux 要共用资产模型，但不强行共用同一采集协议。
4. Windows 采集能力允许多通道共存，但单台主机只允许一个主采集模式长期生效。

## 7. Windows 采集方案设计

## 7.1 路线 A：WinRM 直连采集

这是最快能落地的第一版。

### 7.1.1 采集流程

1. 用户点击“采集信息”
2. 后端读取主机的 Windows 管理方式
3. 如果是 `winrm`，就用 WinRM 连接 Windows
4. 执行 PowerShell 采集脚本
5. 脚本输出统一 JSON
6. 后端解析 JSON，写回 `hosts`

### 7.1.2 采集内容

建议通过 PowerShell + CIM / Counter 采集以下信息：

- OS：`Win32_OperatingSystem.Caption`
- Kernel/Build：`Version` / `BuildNumber`
- Hostname：`CSName`
- Arch：`OSArchitecture`
- Uptime：`LastBootUpTime`
- CPU 核数：`Win32_Processor.NumberOfLogicalProcessors`
- CPU 使用率：`Get-Counter '\\Processor(_Total)\\% Processor Time'`
- Memory：`Win32_OperatingSystem.TotalVisibleMemorySize / FreePhysicalMemory`
- Disk：`Win32_LogicalDisk`，仅统计 `DriveType = 3` 的固定磁盘

### 7.1.3 适用场景

- 机器由统一运维控制
- 可批量启用 WinRM
- 网络上允许 5985/5986
- 希望尽快做出第一版

### 7.1.4 风险

- 工作组环境下认证策略较繁琐
- 没开 WinRM 的机器仍然采不到
- 在线状态本质上仍是“连得上 WinRM”

## 7.2 路线 B：Windows Agent

这是推荐的长期标准方案。

### 7.2.1 采集流程

1. 在 Windows 主机安装 `OpsHub Agent`
2. Agent 以 Windows Service 方式运行
3. Agent 定时采集本机指标
4. Agent 主动通过 HTTPS 上报到 OpsHub
5. OpsHub 更新 `hosts` 资源字段和最后心跳时间

### 7.2.2 Agent 采集内容

与 WinRM 版保持同一份逻辑和输出结构：

- 系统名称、版本、Build、架构
- 主机名、开机时间、运行时长
- CPU 核数、CPU 使用率
- 内存总量、已用、使用率
- 固定磁盘总量、已用、使用率

建议 Agent 输出结构与现有 `SystemInfo` 对齐，这样后端落库逻辑可以复用。

### 7.2.3 Agent 的优势

- 目标机只需要访问 OpsHub，不需要暴露管理端口
- 可以做心跳，在线状态更准确
- 更适合跨网段、NAT、办公网、零信任场景
- 后续扩展能力最好

### 7.2.4 Agent 的职责边界

Agent 只做这些事：

- 本机指标采集
- 心跳上报
- 可选的即时刷新

首版不建议把它做成“远控代理”或“大而全的运维客户端”。

## 8. 最终建议：平台能力共存，产品默认 Agent，迁移保留 WinRM

建议拆成三个层面理解：

### 8.1 平台能力

- 后端同时支持 `AgentCollector` 与 `WinRMCollector`
- Windows 主机通过 `managementMode` 选择主采集通道
- `SSH` 继续保留，但只作为兼容模式

### 8.2 产品默认

- 新增 Windows 主机时默认 `managementMode = agent`
- UI 明确标注 `Agent（推荐）`
- 未来的文档、安装引导、运维手册均以 Agent 为主

### 8.3 迁移阶段

- 工程交付可以先落 `WinRM`
- 但产品路线不应停在 `WinRM`
- 一旦 Agent 可用，应作为 Windows 主机默认与推荐方案

原因：

- WinRM 能更快验证业务闭环
- Agent 才能真正覆盖“大多数没有 SSH 的 Windows 主机”
- 两者共存能兼顾短期交付和长期标准化

## 9. 数据模型改造建议

当前 `Host` 模型里的 `credentialId/sshUser/port` 对 Windows 来说语义已经不准确。

建议新增“管理通道”字段，而不是继续把 Windows 塞进 SSH 字段。

### 9.1 Host 建议新增字段

- `managementMode`
  - `ssh`
  - `winrm`
  - `agent`
  - `none`
- `managementPort`
- `managementCredentialId`
- `collectStatus`
  - `online`
  - `offline`
  - `unknown`
  - `not_configured`
- `collectError`
- `lastCollectAt`
- `agentId`
- `agentVersion`
- `agentLastHeartbeatAt`

补充约束：

- `managementMode` 是单值字段，不是多选集合
- 一台主机在正式运行时只应存在一个主采集模式
- 如果需要迁移，可在运维操作中切换 `managementMode`
- 不建议让 Agent 与 WinRM 长期同时写入同一台主机状态

### 9.2 对现有字段的处理建议

- Linux 主机：
  - `managementMode = ssh`
  - 现有 `credentialId/port/sshUser` 可平滑映射到管理通道字段
- Windows 主机：
  - 不再把 `SSHUser/Port/CredentialID` 当成默认采集字段
  - RDP 字段继续保留用于桌面访问

### 9.3 Credential 模型建议

现有凭据模型已经支持：

- `protocol`
- `type`
- `username`
- `domain`
- `password`

建议扩展：

- `protocol = winrm`
- `authMode`
  - `ntlm`
  - `kerberos`
  - `basic`
  - `certificate`

这样 RDP 凭据和 WinRM 凭据可以分离。

## 10. 后端设计建议

## 10.1 抽象采集接口

不要再让 `CollectHostInfo()` 直接写死 SSH。

建议抽象出统一接口：

```go
type HostCollector interface {
    Test(ctx context.Context, host *Host, cred *Credential) error
    Collect(ctx context.Context, host *Host, cred *Credential) (*collector.SystemInfo, error)
}
```

实现：

- `SSHCollector`
- `WinRMCollector`
- `AgentCollector`

调度逻辑：

- Linux 默认走 `SSHCollector`
- Windows 根据 `managementMode` 选择 `WinRMCollector` 或 `AgentCollector`
- 同一时刻只允许一个 Collector 作为该主机的主采集器执行落库

## 10.2 采集结果统一落库

无论是 SSH、WinRM 还是 Agent，最终都统一写回当前 `hosts` 表中的这些字段：

- `cpuCores`
- `cpuUsage`
- `memoryTotal`
- `memoryUsed`
- `memoryUsage`
- `diskTotal`
- `diskUsed`
- `diskUsage`
- `os`
- `kernel`
- `arch`
- `hostname`
- `uptime`
- `lastSeen`

这样前端资源卡片无需重写。

## 10.3 状态定义调整

当前 `status` 更像 SSH 在线状态。

建议拆开理解：

- `desktopStatus`：RDP 是否可用
- `collectStatus`：采集链路是否可用
- `status`：主列表聚合状态

聚合逻辑建议：

- 有采集心跳/最近采集成功：显示在线
- 仅桌面可达但无采集链路：显示“桌面可用 / 未配置采集”
- 两者都不可达：离线

## 11. 前端设计建议

## 11.1 Windows 编辑页不应继续以 SSH 为中心

当前 Windows 编辑页虽然把 SSH 字段改成“可选”，但这还不够。

建议改成：

### Linux

- 管理方式：固定 `SSH`
- 展示 SSH 用户、SSH 端口、SSH 凭据

### Windows

- 管理方式：
  - `Agent（推荐）`
  - `WinRM`
  - `SSH（兼容）`
  - `仅桌面`
- 桌面方式：RDP
- 桌面凭据：独立选择

默认策略：

- 新增 Windows 主机时默认选中 `Agent（推荐）`
- 如果用户主动切换到 `WinRM` 或 `SSH（兼容）`，再展示对应字段
- `仅桌面` 只解决远程桌面，不提供资源采集

### Windows 选择不同管理方式时显示不同字段

如果是 `WinRM`：

- WinRM 端口
- HTTP/HTTPS
- 认证方式
- WinRM 凭据

如果是 `Agent`：

- Agent 安装命令
- 注册令牌
- Agent 状态
- 最后心跳

如果是 `仅桌面`：

- 只显示 RDP 相关字段
- 资源区显示“未配置采集通道”

## 11.2 主机列表建议

建议新增或补充这些展示：

- 采集方式
- 桌面方式
- 采集状态
- 最后采集时间

Windows 主机如果没有采集通道，不应只是显示 `-`，应明确提示：

- `未配置采集`
- `WinRM 未连通`
- `Agent 未在线`

## 11.3 按钮语义建议

现在“测试连接”和“采集信息”对 Windows 容易造成歧义。

建议拆分：

- `测试桌面`
- `测试采集`
- `采集信息`

## 12. Agent 方案的最小实现范围

如果选择 Agent，不建议一上来做太大。

### 12.1 首版只做这些能力

- 安装为 Windows Service
- 启动注册
- 定时心跳
- 周期采集 CPU/内存/磁盘/系统信息
- HTTPS 上报

### 12.2 首版不做这些能力

- 远程命令执行
- 文件分发
- 进程管理
- 补丁管理
- 事件日志拉取

这些可以放到后续阶段。

## 13. 安全要求

## 13.1 WinRM 模式

- 优先 `HTTPS`
- 域环境优先 `Kerberos`
- 不建议把 `Basic over HTTP` 作为默认方案
- 凭据继续沿用现有加密存储机制

## 13.2 Agent 模式

- Agent 与 OpsHub 通过 TLS 通信
- 使用一次性注册令牌或短期注册码
- 服务端为 Agent 分配唯一 `agentId`
- 心跳接口必须鉴权
- 支持服务端吊销 Agent

## 13.3 审计

新增审计事件建议包括：

- 测试采集通道
- 手动触发采集
- Agent 注册
- Agent 心跳异常
- WinRM 认证失败

## 14. 分阶段实施建议

## Phase 1：采集架构解耦

目标：

- 把当前 `SSH = 采集` 的硬编码拆开
- 主机模型支持 `managementMode`
- 前端表单区分 Linux 与 Windows 的采集方式

产出：

- 数据模型调整
- 后端 Collector 抽象
- 前端表单改版
- 明确 `managementMode` 单主模式约束

## Phase 2：WinRM 第一版（迁移方案）

目标：

- Windows 不依赖 SSH，也能手动采集
- 支持 CPU/内存/磁盘/系统信息

产出：

- `WinRMCollector`
- WinRM 凭据与测试连接
- Windows 主机资源卡片可用
- 作为无法安装 Agent 场景下的过渡方案

## Phase 3：Agent 标准方案（产品默认）

目标：

- 支持大多数没有 SSH、也不方便开 WinRM 的 Windows 主机
- 实现心跳和持续上报

产出：

- Windows Agent
- 注册与心跳接口
- Agent 状态展示
- 新增 Windows 主机默认 `Agent（推荐）`

## Phase 4：高级资产能力

可选扩展：

- 每盘符明细
- 网卡与 IP 明细
- 服务列表
- 进程列表
- 补丁信息
- 事件日志

## 15. 最终建议

一句话总结：

- Windows 主机的“桌面接入”与“资源采集”必须分开设计。
- RDP 继续保留为桌面通道。
- 资源采集不应继续依赖 SSH。
- 平台层面保留 Agent 与 WinRM 共存。
- 产品层面默认收敛到 Agent。
- 单台主机正式运行时只保留一个主采集模式。

如果只问“这项目现在怎么改最合理”，我的结论是：

1. 不再把 Windows 主机当成“可选 SSH 的 Linux 变体”
2. 先抽象 `managementMode`
3. 后端同时保留 `AgentCollector` 和 `WinRMCollector`
4. 前端 Windows 管理方式提供 `Agent（推荐） / WinRM / SSH（兼容） / 仅桌面`
5. 新增 Windows 主机默认 `Agent`
6. 迁移阶段保留 `WinRM`
7. 单机正式运行时禁止双活采集
