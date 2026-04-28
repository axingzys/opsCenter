# 主机管理连接入口与 Windows 远程桌面改造方案

## 背景

主机管理里原有的“终端”能力本质是 SSH Web 终端，后端接口 `/api/v1/asset/terminal/:id` 会直接使用主机的 SSH 用户、SSH 端口和 SSH 凭据创建 `ssh.Client`。

现在 Windows 主机已经增加远程桌面能力，链路为：

```text
Browser -> OpsHub -> Guacamole -> guacd -> RDP -> Windows
```

因此主机管理里不能再把“终端”作为所有主机的统一连接入口。Linux 主机的交互入口是 SSH 终端，Windows 主机的主要交互入口应是 RDP 远程桌面。Windows 的 Agent、WinRM、文件管理和采集能力属于管理通道，不应混进 SSH 终端入口。

## 当前代码观察

### 前端

- `web/src/views/asset/Hosts.vue`
  - 页面顶部存在“终端”按钮，打开 `/terminal`。
  - 主机表单已经支持 Windows 管理方式：`Agent / WinRM / SSH / 仅桌面`。
  - Windows 主机在 `desktopEnabled=true` 且有桌面权限时，列表操作列会显示“桌面连接”按钮。
  - 主机详情中已经展示 `managementModeText`，但列表页对连接能力的表达还不够明确。

- `web/src/views/asset/Terminal.vue`
  - 当前终端页加载所有主机。
  - 双击任意主机都会连接 `/api/v1/asset/terminal/:id`。
  - 没有过滤 Windows 非 SSH 主机，也没有提示“这里只是 SSH 终端”。

- `web/src/utils/permission.ts`
  - 已经拆分 `TERMINAL=8` 和 `DESKTOP=64`。
  - 这说明 SSH 终端和 Windows 桌面应该保持两个独立权限，不需要混用。

### 后端

- `internal/server/asset/terminal.go`
  - `TerminalManager.CreateSession` 是纯 SSH 实现。
  - 使用 `hostVO.CredentialID`、`hostVO.SSHUser`、`hostVO.Port` 创建 SSH 会话。
  - 当前缺少对 Windows 非 SSH 管理方式的明确拒绝。

- `internal/biz/asset/desktop.go`
  - `DesktopSessionUseCase.CreateLaunch` 已经强校验：
    - 仅 Windows 主机支持桌面连接
    - 主机必须启用桌面访问
    - 仅支持 RDP
    - 桌面凭据必须为 RDP 密码凭据

- `internal/biz/asset/host.go`
  - 主机模型已经包含 `osType`、`managementMode`、`desktopEnabled`、`desktopCredentialId` 等字段。
  - 当前模型足够支撑入口语义拆分。

## 设计原则

1. 不再把 Windows 主机当成“可选 SSH 的 Linux 主机”。
2. SSH 终端和 Windows 远程桌面是两个独立能力：
   - SSH 终端：Linux 默认入口，Windows 仅在 `managementMode=ssh` 时作为兼容能力。
   - 远程桌面：Windows RDP 入口，走 `DESKTOP` 权限。
3. Windows 管理通道和桌面通道解耦：
   - `Agent` 用于长期采集、文件管理和状态上报。
   - `WinRM` 用于无 Agent 场景的采集或自动部署辅助。
   - `仅桌面` 只提供 RDP，不提供资源采集。
4. 前端入口必须表达真实能力，后端必须做兜底校验。
5. 一期不新增权限位、不重构主机模型、不新增大页面，先把误导性入口修正。

## 推荐交互方案

### 主机列表

主机列表保留现有行内操作，但明确连接能力：

- Linux 主机：
  - 显示 `Linux / SSH终端 / 采集状态`
  - SSH 终端入口继续使用 `/terminal`

- Windows 主机：
  - 显示 `Windows / 管理方式 / 采集状态`
  - 启用桌面时显示 `RDP桌面`
  - 行内保留“桌面连接”按钮
  - 只有 `managementMode=ssh` 的 Windows 主机才属于 SSH 终端可连接范围

### 顶部入口

将顶部“终端”改名为“SSH终端”。

原因：

- 现有 `/terminal` 页面和后端都是 SSH 能力。
- “终端”这个名称会误导用户以为 Windows 也能进入。
- 改名后，Windows 用户会自然使用行内“桌面连接”按钮。

### SSH 终端页

`/terminal` 页面只展示支持 SSH 的主机：

- Linux 主机
- Windows 且 `managementMode=ssh` 的主机

不展示：

- Windows `managementMode=agent`
- Windows `managementMode=winrm`
- Windows `managementMode=none`

页面文案统一改为“SSH终端”，空态提示改为“左侧仅显示支持 SSH 的主机”。

### 后端兜底

`/api/v1/asset/terminal/:id` 需要做明确保护：

- Windows 且 `managementMode != ssh` 时拒绝，并提示使用远程桌面。
- SSH 用户为空时拒绝。
- 凭据协议不是 `ssh` 时拒绝。

这样即使前端绕过过滤，后端也不会误用 RDP/WinRM 凭据创建 SSH 连接。

## 分阶段计划

### 一期：入口语义修正与保护

目标：避免 Windows 主机误入 SSH 终端，明确 Linux/Windows 的连接入口。

范围：

1. 主机管理顶部按钮改名：
   - `终端` -> `SSH终端`

2. 主机列表增加轻量连接能力展示：
   - 操作系统
   - SSH 终端能力
   - Windows RDP 桌面能力
   - 管理方式
   - 采集状态

3. `/terminal` 页面过滤主机：
   - 只显示 SSH 能力主机
   - 页面文案改成 `SSH终端`
   - 从其他入口传入非 SSH 主机时跳过并提示

4. 后端 SSH 终端兜底校验：
   - Windows 非 SSH 管理方式拒绝
   - 非 SSH 凭据拒绝
   - 缺少 SSH 用户拒绝

5. 不改数据库结构，不新增权限位。

验收标准：

- Linux 主机仍可进入 SSH 终端。
- Windows `managementMode=agent/winrm/none` 不会出现在 SSH 终端页。
- Windows `managementMode=ssh` 仍可作为兼容模式进入 SSH 终端。
- Windows RDP 桌面按钮仍可打开远程桌面。
- 直接访问 SSH WebSocket 连接 Windows 非 SSH 主机时，后端返回清晰错误。

### 二期：Windows 管理抽屉（暂不实施）

二期原本规划为 Windows 主机行内“管理抽屉”，用于收敛桌面、Agent、采集、文件和审计能力。当前判断是：主机管理列表不需要继续承载更多操作面板，优先建设独立连接工作台。

处理结论：

- 不保留 Windows 管理抽屉代码。
- 不新增二期相关后端接口或审计过滤参数。
- 主机管理继续保留一期能力：
  - `SSH终端` 顶部入口
  - Windows 行内 `桌面连接`
  - 连接能力列
  - Windows 非 SSH 主机禁止进入 SSH 终端

二期不影响三期。三期连接工作台直接复用已有 SSH 终端、RDP 桌面、主机列表、分组树和权限数据。

### 三期：连接工作台

目标：新增独立连接中心，主机管理负责资产配置，连接工作台负责日常登录操作。

页面路径：

```text
/asset/connections
```

页面定位：

- 快速找到主机。
- 明确区分 `SSH终端` 和 `Windows桌面`。
- 支持按分组、系统、连接方式、状态过滤。
- 支持最近连接和收藏，但不在第一版强依赖后端新表。
- 连接动作复用现有 `/terminal` 和 `/desktop` 能力。
- 审计继续保持 SSH 终端审计、Windows 桌面审计分离。

页面结构：

1. 顶部工具栏
   - 搜索主机名、IP、标签、分组。
   - 操作系统筛选：全部 / Linux / Windows。
   - 连接方式筛选：全部 / SSH终端 / Windows桌面 / Agent可用 / 无可用连接。
   - 状态筛选：全部 / 在线 / 离线 / 未知。
   - 刷新按钮。

2. 左侧分组树
   - 复用资产分组树。
   - 点击分组过滤右侧连接列表。
   - 展示当前分组下主机数量和可连接能力数量。

3. 中间连接列表
   - 展示所有可连接资产。
   - 每行展示主机名、IP、系统、分组、连接能力、管理方式、采集状态、最近连接、收藏状态。
   - 行内动作：
     - Linux 或 SSH 兼容 Windows：`打开SSH`
     - Windows 且启用桌面：`打开桌面`
     - `主机详情`
     - `收藏`

4. 右侧辅助面板
   - 当前筛选统计。
   - 最近连接。
   - 我的收藏。
   - 配置缺失提示，例如未配置桌面、无 SSH 能力、Agent 离线。

Tab 设计：

- `全部连接`
  - 展示全部主机。
  - Linux 默认主操作是 `SSH终端`。
  - Windows 默认主操作是 `远程桌面`。
  - Windows `managementMode=ssh` 时额外显示 `SSH终端`。

- `SSH终端`
  - 只展示 Linux 主机和 `managementMode=ssh` 的 Windows 主机。
  - 支持单台打开 SSH。
  - 后续支持多选批量打开 SSH。

- `Windows桌面`
  - 只展示 Windows 主机。
  - `desktopEnabled=true` 且有桌面权限时可连接。
  - 缺少 RDP 配置时显示原因并禁用按钮。

- `最近连接`
  - 聚合最近 SSH 会话和 RDP 会话。
  - 第一版可先使用前端 localStorage 记录从工作台发起的连接。
  - 后续再切到后端会话表聚合。

- `我的收藏`
  - 第一版可先使用 localStorage，按用户浏览器保存。
  - 后续新增后端收藏表实现跨设备同步。

接口规划：

第一版先复用已有接口：

- `GET /api/v1/hosts`
- `GET /api/v1/asset-groups/tree`
- `POST /api/v1/hosts/:id/desktop/sessions`
- `GET /api/v1/asset/terminal/:id`

后续可新增聚合接口：

```http
GET /api/v1/asset/connections
```

建议查询参数：

```text
keyword
groupId
osType
capability
status
page
pageSize
favoriteOnly
recentOnly
```

建议返回结构：

```json
{
  "total": 100,
  "list": [
    {
      "hostId": 1,
      "hostName": "win-prod-01",
      "ip": "10.0.0.12",
      "groupId": 2,
      "groupName": "生产环境",
      "osType": "windows",
      "managementMode": "agent",
      "managementModeText": "Agent",
      "collectStatus": "online",
      "collectStatusText": "在线",
      "desktopEnabled": true,
      "sshCapable": false,
      "rdpCapable": true,
      "agentCapable": true,
      "fileCapable": true,
      "lastConnectedAt": "2026-04-27 10:22:00",
      "lastConnectionType": "rdp",
      "favorite": true,
      "disabledReason": ""
    }
  ]
}
```

后续收藏接口：

```http
POST   /api/v1/asset/connections/:hostId/favorite
DELETE /api/v1/asset/connections/:hostId/favorite
GET    /api/v1/asset/connection-favorites
```

权限规则：

- `SSH终端` 使用 `PERMISSION.TERMINAL`。
- `Windows桌面` 使用 `PERMISSION.DESKTOP`。
- `主机详情` 使用 `PERMISSION.VIEW`。
- 收藏不应提升连接权限，只保存入口偏好。
- 最近连接普通用户只看自己的连接；管理员后续可看全部。

三期第一版 MVP：

- 新增 `/asset/connections` 页面和路由。
- 主机管理顶部增加 `连接工作台` 入口。
- 实现 `全部连接 / SSH终端 / Windows桌面 / 最近连接 / 我的收藏`。
- 实现搜索、分组、系统、能力、状态过滤。
- 实现 SSH 一键打开，复用 `/terminal` 的 `sessionStorage` 传参。
- 实现 RDP 一键打开，复用 `createDesktopSession`。
- 最近连接和收藏先用 localStorage，后续再后端持久化。

三期后续增强：

- 后端聚合连接查询接口。
- 后端收藏表。
- 后端最近连接聚合。
- 多选批量打开 SSH。
- 默认连接方式和快捷键。

## 不建议的方案

不建议只加一个“Windows管理”按钮并直接打开远程桌面。

原因：

- “Windows管理”含义太宽，用户无法判断里面是 RDP、Agent、WinRM 还是文件管理。
- Windows 桌面连接已经是一个明确能力，应直接叫“远程桌面”或“RDP桌面”。
- 管理通道和桌面通道职责不同，混在一个按钮里后续会继续产生歧义。

## 实施记录

### 一期已完成

- `web/src/views/asset/Hosts.vue`
- `web/src/views/asset/Terminal.vue`
- `internal/server/asset/terminal.go`
- `docs/windows-host-access-entry-plan.md`

### 二期已取消

- 二期 Windows 管理抽屉代码已还原。
- 当前实现直接进入三期连接工作台。

### 三期进行中

- `docs/windows-host-access-entry-plan.md`
- `web/src/views/asset/ConnectionWorkspace.vue`
- `web/src/router/index.ts`
- `web/src/views/asset/Hosts.vue`
