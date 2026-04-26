# OpsHub Windows Agent 操作文档

## 1. 文档范围

这份文档描述的是当前仓库**已经落地并验证通过**的 Windows Agent 路线。

当前结论：

- Windows 主机已经可以进入 `插件管理 -> Agent管理`
- 支持像 Linux 一样从平台页面发起**自动部署**和**自动卸载**
- Windows 自动部署当前走的是：
  - `WinRM 远程执行`
  - 远端拉取 `install.ps1`
  - 下载 `opshub-agent-windows-amd64`
  - 安装为 `Windows Service`
  - 本地暴露 `19100 /metrics`

当前落地的是：

- `opshub-agent/0.1.0`
- `Windows Service` 常驻运行版

---

## 2. 当前实现状态

截至 `2026-04-07`，Windows Agent 当前具备以下能力：

- `Agent管理` 页面支持筛选和部署 Windows 主机
- Windows 自动部署依赖 `WinRM` 密码凭据
- Windows 自动部署完成后会：
  - 写入 `config.json`
  - 写入 `opshub-agent.exe`
  - 注册 `Windows Service`
  - 自动启动服务
  - 首次注册并立即上报
  - 暴露本地 `19100 /metrics`
- Windows 自动卸载会删除：
  - Windows Service `OpsHubAgent-Host-{hostId}`
  - 目录 `C:\ProgramData\OpsHubAgent`
- 卸载后主机仍保留：
  - `managementMode=agent`
  - WinRM 端口
  - WinRM 凭据
- 所以 Windows 主机可以直接再次在 `Agent管理` 页面重装

当前 Windows Agent 部署完成后，目标机上会生成：

- `C:\ProgramData\OpsHubAgent\config.json`
- `C:\ProgramData\OpsHubAgent\opshub-agent.exe`
- Windows Service `OpsHubAgent-Host-{hostId}`

---

## 3. 自动部署前提

Windows 自动部署不是“无通道直装”。

要能在平台点击“部署Agent”，目标 Windows 主机必须满足：

1. 主机类型为 `windows`
2. 主机管理方式为 `Agent`
3. 已配置 `WinRM` 密码凭据
4. WinRM 端口可达：
   - 通常是 `5985`
   - HTTPS 时可以是 `5986`
5. 目标 Windows 主机可以访问 OpsHub 后端地址
6. WinRM 账号具备足够权限执行管理员 PowerShell

如果不满足这些前提：

- 平台无法自动首装
- 只能走手工安装

---

## 4. 页面操作步骤

### 4.1 准备 WinRM 凭据

在 `资产管理 -> 凭证管理` 中创建一条：

- 协议：`winrm`
- 认证方式：`password`
- 用户名：Windows 登录用户
- 密码：对应密码
- 如有域环境，可填写 `domain`

注意：

- 当前 Windows 自动部署只支持 `WinRM + 密码`
- `SSH` 不作为 Windows Agent 管理页自动部署的主路径

### 4.2 新增或编辑 Windows 主机

在 `资产管理 -> 主机管理` 中：

1. 选择或新建一台 Windows 主机
2. `操作系统` 选择 `Windows`
3. `管理方式` 选择 `Agent（推荐）`
4. 填写：
   - `WinRM端口`
   - `WinRM凭据`
5. 保存主机

当前页面已经支持：

- Windows 选择 `Agent` 时保留 WinRM 端口和凭据
- 不再像之前那样切到 `Agent` 就把 WinRM 信息清空

### 4.3 在 Agent 管理页发起自动部署

进入：

- `插件管理 -> Agent管理`

点击：

- `部署Agent`

部署弹窗中：

- Linux 主机走 `SSH`
- Windows 主机走 `WinRM`

Windows 主机会进入部署候选列表的条件是：

- `osType=windows`
- `managementMode=agent`
- 已配置 `managementCredentialId`
- 当前没有 `agentId`

选择目标 Windows 主机后点击：

- `开始部署`

### 4.4 平台自动执行的动作

点击部署后，平台会：

1. 为该主机生成注册令牌
2. 生成短安装命令
3. 通过 `WinRM` 到目标主机执行安装命令
4. 目标主机再去拉取：
   - `/api/v1/public/agents/install.ps1`
5. 远端脚本写入：
   - `config.json`
   - `opshub-agent.exe`
6. 创建并启动 Windows Service
7. 立即执行注册与上报
8. 平台更新：
   - `hosts`
   - `asset_agents`
   - `asset_host_inventory`

---

## 5. 自动卸载步骤

进入：

- `插件管理 -> Agent管理`

支持两种方式：

- 单台点击卸载图标
- 勾选后批量卸载

平台会通过 `WinRM` 在目标机执行卸载脚本，具体动作包括：

1. 停止并删除 Windows Service `OpsHubAgent-Host-{hostId}`
2. 删除 `C:\ProgramData\OpsHubAgent`
3. 清理：
   - `asset_agents`
   - `asset_host_inventory`
   - Prometheus target（如果存在）
4. 清空主机当前 Agent 状态
5. 保留 WinRM 端口和 WinRM 凭据，便于后续重装

卸载完成后，主机会回到：

- `managementMode=agent`
- `managementPort=5985`
- `managementCredentialId=<原 WinRM 凭据>`

这意味着：

- 可以直接在 `Agent管理` 中再次部署

---

## 6. 手工安装回退路径

如果 Windows 主机没有 WinRM 或当前不适合让平台远程执行，仍然可以走手工安装。

入口在：

- `资产管理 -> 主机管理 -> 编辑 Windows 主机`

点击：

- `生成安装命令`

然后在目标主机上以管理员 PowerShell 执行。

所以当前 Windows 有两条路：

1. 有 WinRM：
   - 走 `Agent管理` 自动部署
2. 没 WinRM：
   - 走主机详情里的手工安装命令

---

## 7. 部署后的检查项

### 7.1 平台侧检查

在 `插件管理 -> Agent管理` 中确认：

- 版本显示为 `opshub-agent/0.1.0`
- 状态为 `运行中`
- 安装进度为 `100%%`
- 最后心跳、最后上报已有时间

在 `资产管理 -> 主机管理` 中确认：

- CPU / 内存 / 磁盘摘要已经出现
- 主机采集状态为在线

### 7.2 Windows 主机本地检查

在目标 Windows 主机执行：

```powershell
Get-Item 'C:\ProgramData\OpsHubAgent\config.json'
Get-Item 'C:\ProgramData\OpsHubAgent\opshub-agent.exe'
Get-Service -Name 'OpsHubAgent-Host-<主机ID>'
Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:19100/metrics'
```

预期结果：

- 安装目录存在
- `config.json` 存在
- `opshub-agent.exe` 存在
- 服务存在且状态为 `Running`
- `/metrics` 可访问，返回 `opshub_agent_*`

---

## 8. 当前限制

这部分必须单独说明，避免把当前 Windows 方案误认为正式版。

### 8.1 当前自动部署依赖 WinRM

如果目标 Windows 主机：

- 没开 WinRM
- 没配置 WinRM 凭据
- WinRM 端口不通
- 主机无法访问 OpsHub 后端地址

那么：

- `Agent管理` 无法自动部署
- 只能手工安装

### 8.2 当前仍然存在的边界

当前已经是正式的 Windows Service 路线，但仍有两个边界需要说明：

- 首装仍然依赖 `WinRM`，不是“无通道直装”
- 当前默认只验证了 `amd64` Windows 主机

---

## 9. 已完成的真实验证

样板主机：

- `192.168.1.9`

验证账号：

- 用户名：`axing`
- 密码：`123456`

已验证结果：

### 9.1 WinRM 通道

实测：

- `5985` 可达
- 远程 PowerShell 可执行

### 9.2 自动卸载

在 `2026-04-07 17:26:41` 到 `17:26:42` 实测通过。

卸载后远端结果：

- `serviceExists=false`
- `taskExists=false`
- `homeExists=false`
- `configExists=false`
- `binaryExists=false`

平台结果：

- `asset_agents` 已删除
- `asset_host_inventory` 已删除
- 主机回到 `managementPort=5985`

### 9.3 自动重装

在 `2026-04-07 17:26:45` 到 `17:26:52` 实测通过。

重装后远端结果：

- `serviceExists=true`
- `serviceStatus=Running`
- `taskExists=false`
- `homeExists=true`
- `configExists=true`
- `binaryExists=true`
- `metricsOk=true`

平台结果：

- 新 `agentId = e9b22b38-4662-4b8b-a055-f65c08ce3e53`
- 版本：`opshub-agent/0.1.0`
- 状态：`running`
- 本地监听：`19100`
- `asset_host_inventory` 已写入
- Prometheus 可按 `19100` 抓取

这说明当前 Windows 自动部署闭环已经成立：

- 卸载可用
- 重装可用
- 主机摘要回写可用
- Windows Service 常驻可用
- `/metrics` 暴露可用

---

## 10. 下一阶段建议

当前建议的下一步不再是“把计划任务改成服务”，这一步已经完成。

后续更有价值的是：

- 把 Windows Agent 纳入和 Linux 一样的趋势、告警和主机监控口径
- 补更多 Windows 专属库存字段
- 增加 `重装 / 重启 / 查看安装日志` 这些 Agent 管理页操作
