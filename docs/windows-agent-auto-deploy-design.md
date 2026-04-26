# OpsHub Windows Agent 自动部署方案

## 1. 目标

这份文档只回答一个问题：

- Windows Agent 能不能做成像 Linux `Agent管理` 那样，在页面里选主机后直接部署。

结论先行：

- **可以做**
- 但前提不是“完全无通道直装”
- 而是目标 Windows 主机至少要具备一种首装通道：
  - `WinRM`
  - `SSH`
  - 已安装旧版 Agent
  - 企业分发通道

对你现在给的样板主机 `192.168.1.9`，结论是：

- **可以作为 Windows 自动部署样板机**
- 因为它的 `WinRM(5985)` 已开启，并且账号 `axing / 123456` 已实测可执行远程 PowerShell

---

## 2. 这次实测结果

实测时间：

- `2026-04-07 15:13:40 +0800` 起

样板主机：

- `192.168.1.9`

提供账号：

- 用户名：`axing`
- 密码：`123456`

### 2.1 网络连通性

实测结果：

- `ping` 可达
- `5985` 打开
- `3389` 打开
- `22` 关闭
- `19100` 关闭

这说明当前这台机子：

- 可以走 `WinRM`
- 可以走 `RDP`
- 不能走 `SSH`
- 当前没有 Linux 那种本地 `Agent /metrics` 监听端口

### 2.2 WinRM 可用性

我用 `axing / 123456` 分别测试了：

- `NTLM`
- `Basic`

两种方式都能执行 PowerShell，并返回：

- `Microsoft Windows 11 专业版`

这说明：

- Windows 自动部署的“远程执行通道”已经具备
- 这台主机非常适合做第一台 WinRM 自动部署样板机

### 2.3 当前旧版 Windows Agent 状态

数据库中的当前状态是：

- 主机 `id=2`
- `ip=192.168.1.9`
- `os_type=windows`
- `management_mode=agent`
- `management_port=19100`
- `agent_version=powershell-agent/0.1.0`
- `agent_last_heartbeat_at=2026-04-07 15:12:08 UTC`
- `agent_last_report_at=2026-04-07 15:12:08 UTC`

远端 Windows 主机上的实测结果：

- `C:\ProgramData\OpsHubAgent\config.json` 存在
- `C:\ProgramData\OpsHubAgent\report.ps1` 存在
- 计划任务 `OpsHubAgent-Host-2` 存在
- 当前 `reportUrl` 是：
  - `http://192.168.1.12:9876/api/v1/public/agents/report`
- 当前版本是：
  - `powershell-agent/0.1.0`

计划任务最近运行状态也正常：

- `Last Run Time: 2026/4/7 15:17:01`
- `Status: Ready`

### 2.4 当前旧版 Agent 的问题

虽然旧版 Windows Agent 不是“没跑起来”，但它和 Linux 版不是一回事。

当前旧版本质上是：

- `PowerShell 上报脚本 + 计划任务`

它的问题是：

1. 没有本地 `19100 /metrics`
2. `19100` 端口当前没监听
3. 不能被 Prometheus 抓取
4. 不能进入现在 Linux 那套完整趋势与告警链路
5. `asset_host_inventory` 里 `top_processes_json / listening_ports_json / private_ips_json` 当前是 `null`
6. 当前 `Agent管理` 页面和后端自动部署逻辑都只支持 Linux

所以之前那版不是“完全失败”，而是：

- **只能算手工安装的过渡版**

---

## 3. 当前代码边界

截至当前仓库状态，Windows 和 Linux 的 Agent 生命周期能力不对等。

### 3.1 已有能力

已经有的部分：

- Windows 主机支持 `managementMode=agent`
- 主机管理里支持“生成安装命令”
- 后端支持：
  - `install.ps1`
  - `register`
  - `report`
- Windows 旧版 Agent 可以手工安装并周期上报

### 3.2 缺失能力

缺失的部分：

- `Agent管理` 的部署列表只筛 `linux`
- 后端 `deployOne()` 明确限制：
  - `仅 Linux 主机支持自动部署 Agent`
- 后端 `uninstallOne()` 明确限制：
  - `仅 Linux 主机支持自动卸载 Agent`
- 当前 Docker 构建只打 Linux Agent bundle
- 没有 Windows 二进制 Agent 包
- 没有 Windows Service 安装/升级/卸载闭环

### 3.3 当前页面为什么不能直接点 Windows 部署

不是因为页面少个按钮，而是当前底层没有 Windows 自动部署实现。

Linux 当前是：

1. 页面选主机
2. 后端走 SSH
3. 远程执行安装命令
4. 二进制常驻运行
5. Prometheus 抓取 `19100`

Windows 当前只有：

1. 主机详情里生成手工命令
2. 人工去 Windows 主机执行 PowerShell
3. 建计划任务周期上报

所以“把 Windows 放进 Agent管理”不是纯前端活，必须补后端部署链路。

---

## 4. 能不能做成像 Linux 一样

答案是：

- **可以做成“页面里选主机 -> 点击部署 -> 自动安装”**
- 但底层实现不应该照搬 Linux 的 SSH 逻辑

Windows 正确的做法应该是：

- `Linux`：继续走 `SSH 自动部署`
- `Windows`：走 `WinRM 自动部署`

也就是说：

- 页面体验可以尽量一致
- 底层部署协议必须分开

---

## 5. 推荐实现路线

我建议分两条路线看，不要混在一起。

## 5.1 路线 A：快速接入版

目标：

- 尽快让 Windows 出现在 `插件管理 -> Agent管理` 中
- 页面上可以像 Linux 一样勾选并点击部署

做法：

1. `Agent管理` 的部署弹窗增加 Windows 主机
2. 仅筛选满足以下条件的 Windows 主机：
   - `osType=windows`
   - `managementMode=agent`
   - 有 WinRM 凭证
   - `5985` 或 `5986` 可达
   - 当前未安装 Agent
3. 后端部署逻辑分支：
   - Linux 走 SSH
   - Windows 走 WinRM
4. Windows WinRM 部署时，远程执行当前已有的 PowerShell Bootstrap 命令
5. 平台继续使用当前 `powershell-agent/0.1.0` 注册上报

### 优点

- 开发量最小
- 复用现有 `install.ps1`
- 你给的 `192.168.1.9` 可以直接作为首台样板机
- 可以先把“手工部署”改成“平台点部署”

### 缺点

- 仍然是旧版 PowerShell Agent
- 仍然没有本地 `19100 /metrics`
- 监控趋势和 Prometheus 链路仍然不完整
- 仍然没有正式 Windows Service 生命周期

### 适用定位

这条路线适合：

- 先把产品体验做通
- 先消灭“手工复制命令”这一步

但它不适合当最终方案。

## 5.2 路线 B：正式版

目标：

- Windows 真的进入和 Linux 相同等级的 Agent 生命周期体系

做法：

1. 新增 Windows Agent bundle：
   - `opshub-agent-windows-amd64.exe`
   - 视需要再补 `arm64`
2. 复用现有 `cmd/agent` 作为跨平台主程序
3. 为 Windows 增加 Service 安装能力
4. `install.ps1` 不再创建计划任务
5. 改为：
   - 下载 `exe`
   - 写配置
   - 安装 Windows Service
   - 启动服务
6. 本地监听：
   - `0.0.0.0:19100`
7. Prometheus 抓取 Windows Agent
8. 后续升级/卸载改成：
   - `WinRM 首装`
   - `Agent 自管理任务`

### 优点

- 才是真正的 Windows Agent
- 能进入完整监控趋势和告警链路
- 生命周期闭环更完整
- 和 Linux 的产品体验能真正对齐

### 缺点

- 开发量更大
- 需要补 Windows bundle、Service、任务机制

### 适用定位

这条路线适合作为：

- 正式交付方案
- 后续长期标准方案

---

## 6. 推荐的交付策略

不建议一口气直接做完整正式版再上线。

我建议按两阶段推进：

### Phase WA-1：先做 Windows 自动部署

目标：

- 先让 `Agent管理` 支持 Windows
- 让 Windows 首装从“手工执行命令”改为“页面点部署”

实现方式：

- 用 WinRM 远程执行当前已有 `install.ps1`

这一步完成后，产品层面会得到：

1. Windows 主机也能在 `Agent管理` 里选中部署
2. 部署体验接近 Linux
3. 不再依赖人工登录 Windows 复制命令

### Phase WA-2：再把旧版 PowerShell Agent 换成正式 Windows Service Agent

目标：

- 把 `powershell-agent/0.1.0` 替换为 `opshub-agent/0.1.0-windows`

这一步完成后，技术层面会得到：

1. 本地 `19100 /metrics`
2. Prometheus 抓取
3. 监控趋势完整
4. 进程/端口/网络/磁盘 IO 全部入库
5. 升级/重启/卸载闭环

---

## 7. Phase WA-1 详细设计

这一阶段只解决自动部署，不急着重写 Windows Agent 主体。

### 7.1 页面改造

改造 `插件管理 -> Agent管理`：

1. 部署弹窗增加 Windows 主机
2. 增加 `OS类型` 字段或图标
3. 部署候选主机过滤规则改成：

Linux：

- `osType=linux`
- 有 SSH 用户
- 有 SSH 凭证
- 未安装 Agent

Windows：

- `osType=windows`
- `managementMode=agent`
- 有 WinRM 凭证
- 未安装 Agent

### 7.2 部署动作

用户点击“部署Agent”后：

1. 页面提交 `hostIds`
2. 后端逐台判断主机类型
3. Linux 走现有 SSH 分支
4. Windows 走新增 WinRM 分支

### 7.3 Windows WinRM 部署步骤

Windows 单机部署流程建议如下：

1. 读取主机与凭证
2. 校验：
   - `osType=windows`
   - `managementMode=agent`
   - WinRM 凭证存在
3. 测试 WinRM 连接
4. 生成当前主机的 Bootstrap
5. 通过 WinRM 远程执行：

```powershell
powershell.exe -ExecutionPolicy Bypass -NoProfile -Command "Invoke-RestMethod -UseBasicParsing -Uri '<install.ps1 url>' | Invoke-Expression"
```

6. 等待注册结果
7. 更新任务状态

### 7.4 卸载动作

如果这一阶段仍然沿用旧版 PowerShell Agent，那么 Windows 卸载也应该先做 WinRM 版本。

建议卸载步骤：

1. 删除计划任务：
   - `schtasks /Delete /TN "OpsHubAgent-Host-{id}" /F`
2. 删除目录：
   - `C:\ProgramData\OpsHubAgent`
3. 平台清理：
   - `asset_agents`
   - `asset_host_inventory`
   - 主机 `agent_id / agent_version / heartbeat`

### 7.5 任务状态定义

Windows 部署任务建议显示：

- `checking`
- `connecting`
- `installing`
- `waiting_register`
- `running`
- `failed`

这样 Agent 管理页和 Linux 展示就能保持一致。

### 7.6 这阶段的局限

这一阶段完成后，依然要接受以下现实：

- 监控趋势仍然不完整
- `19100` 仍然不会监听
- Prometheus 仍然抓不到 Windows Agent
- 仍然只是“自动部署旧版 Agent”

所以这一步只是：

- **产品体验补齐**

不是：

- **技术方案最终收口**

---

## 8. Phase WA-2 详细设计

这一阶段把 Windows 从旧版脚本 Agent 升到正式版。

### 8.1 核心目标

把 Windows Agent 从：

- `PowerShell 脚本 + 计划任务`

升级成：

- `Go 二进制 + Windows Service + /metrics`

### 8.2 构建产物

后端必须新增 Windows bundle：

- `opshub-agent-windows-amd64.exe`

建议先只做：

- `amd64`

因为现网 Windows 机器绝大多数先满足这个架构。

### 8.3 Bootstrap 脚本改造

新的 `install.ps1` 建议做这些事：

1. 创建目录：
   - `C:\Program Files\OpsHubAgent`
   - `C:\ProgramData\OpsHubAgent`
2. 下载 Windows Agent `exe`
3. 写入 `config.json`
4. 安装 Windows Service
5. 启动服务
6. 校验 `19100` 监听

### 8.4 服务建议

推荐使用：

- `kardianos/service`

不要继续使用：

- 计划任务作为正式运行主体

### 8.5 指标与监控

正式版 Windows Agent 必须补齐：

- CPU
- 内存
- 磁盘
- 网络接收
- 网络发送
- 磁盘读取
- 磁盘写入
- 进程
- 监听端口
- 网卡信息

关于 `Load`：

- Windows 没有 Linux 语义下的 `Load Average`
- 第一版应显示为 `N/A`
- 不要为了对齐界面硬造假值

### 8.6 Prometheus 约束

正式版如果要进监控中心，必须满足：

- Prometheus 可访问 `192.168.1.9:19100`

所以 Windows 样板机后续还要确认：

- 防火墙是否允许 `19100`
- Prometheus 所在机器是否能入站访问

如果这条网络不通，就只能做到：

- 摘要上报

做不到：

- 趋势与告警

---

## 9. 为什么你这台样板机适合先开做

`192.168.1.9` 现在正好具备 Windows 自动部署试点的条件：

1. 机器在线
2. `WinRM 5985` 已开启
3. `RDP 3389` 可用
4. 用户 `axing / 123456` 能远程执行 PowerShell
5. 当前已经有一套手工旧 Agent，可作为升级/替换样板

这意味着我们后续可以在这台机上依次验证：

1. WinRM 自动安装旧版 Agent
2. WinRM 自动卸载旧版 Agent
3. WinRM 自动安装正式版 Windows Service Agent
4. Prometheus 抓取 `19100`

---

## 10. 这次建议的最终结论

如果你的目标是：

- 先把 Windows 也放进 `Agent管理`
- 先做到“点部署”

那么答案是：

- **现在就可以做**
- 并且 `192.168.1.9` 能直接作为第一台验证主机

但如果你的目标是：

- Windows 和 Linux 一样完整进入趋势、告警、库存、卸载、升级闭环

那就不能只停在“WinRM 调 install.ps1”。

必须继续做：

1. Windows bundle
2. Windows Service
3. 正式 `19100 /metrics`
4. Prometheus 抓取
5. 升级/卸载任务闭环

---

## 11. 推荐下一步

推荐按这个顺序推进：

1. 先把 `Agent管理` 的 Windows 自动部署做出来
   - 用 WinRM 远程执行现有安装脚本
2. 在 `192.168.1.9` 上完成首轮自动部署验证
3. 再把 Windows 正式版二进制 Agent 和 Service 接进去
4. 最后再补 Windows 卸载、升级、监控趋势完整闭环

也就是说：

- **先解决“能在页面里点部署”**
- **再解决“和 Linux 完全同级”**

这个节奏最稳，也最符合你当前的产品目标。
