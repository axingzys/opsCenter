# OpsHub Windows 主机接入与远程终端技术方案

## 1. 结论

当前项目不能算正式支持 Windows 主机。

原因不是“没有 Windows 页面”，而是资产模块的核心链路都默认了 Linux + SSH：

- 主机模型使用 `SSHUser`、`Port`，前端表单也固定写成“SSH 用户名”“SSH 端口”。
- Web 终端通过 SSH 建立伪终端。
- 文件管理通过 SFTP 实现，默认路径使用 `~` 和 `echo $HOME`。
- 主机信息采集器执行的是 `uname`、`free -b`、`top`、`lscpu`、`/etc/os-release` 这类 Linux 命令。

但是，从架构上看，项目可以扩展为支持 Windows 主机远程接入。

推荐路线分两步：

1. 第一阶段先支持 `Windows + OpenSSH Server`。
   这条路线最接近当前实现，能够较快获得“像 Linux 一样的 Web 终端”体验。
2. 第二阶段再补 `Windows + WinRM/PowerShell Remoting`。
   这条路线更贴近企业 Windows 环境，但实现复杂度明显更高，尤其是交互式终端能力不如 SSH 自然。

如果你的“远程 Win 主机”指的是图形桌面，那个属于 `RDP 代理/桌面网关` 范畴，不属于当前 SSH Web 终端的直接延伸，建议单独作为后续特性评估。

## 2. 当前实现现状

以下模块都已经与 SSH/Linux 耦合：

### 2.1 主机与凭证模型

当前主机模型定义在 `internal/biz/asset/host.go`，关键字段如下：

- `SSHUser`
- `Port`
- `CredentialID`
- `OS`

问题：

- 没有 `osType` 字段区分 `linux/windows`。
- 没有 `connectionProtocol` 字段区分 `ssh/winrm`。
- `SSHUser` 这个命名本身已经把协议写死。

当前凭证模型也只支持：

- `password`
- `key`

这可以覆盖 Linux SSH，但不够表达 Windows 原生远程接入所需的信息，例如：

- WinRM 认证方式
- 域账号
- 是否启用 HTTPS
- 证书校验策略

### 2.2 Web 终端

当前终端服务位于 `internal/server/asset/terminal.go`，终端能力完全基于 SSH：

- 使用 `golang.org/x/crypto/ssh`
- `ssh.Dial`
- `RequestPty`
- `session.Shell()`

这意味着：

- Linux 主机终端是原生支持的。
- Windows 如果安装并启用了 OpenSSH Server，理论上可以复用这条链路。
- 如果走 WinRM，则需要重新实现终端会话适配层。

### 2.3 文件管理

当前文件管理位于 `internal/biz/asset/host_usecase.go` 与 `pkg/ssh/client.go`：

- 依赖 SFTP
- 依赖 `echo $HOME`
- 路径默认使用 `~`

这会导致 Windows 即使通过 SSH 连上，文件管理也不完整：

- `~` 的语义未必和 Linux 一致
- `echo $HOME` 对 PowerShell/CMD 并不可靠
- 路径分隔符、盘符、根目录展示都需要单独处理

### 2.4 主机信息采集

当前采集器位于 `pkg/collector/collector.go`，依赖的命令是 Linux 命令集：

- `uname -r`
- `uname -m`
- `free -b`
- `top -bn1`
- `lscpu`
- `cat /proc/cpuinfo`
- `. /etc/os-release`

因此当前采集器对 Windows 不可用。

### 2.5 前端页面与接口

当前资产前端位于：

- `web/src/views/asset/Hosts.vue`
- `web/src/views/asset/Credentials.vue`
- `web/src/api/host.ts`

现状问题：

- 主机表单固定为“SSH 端口”“SSH 用户名”。
- 终端欢迎语明确写成“SSH Web 终端”。
- 文件接口默认路径是 `~` / `~/`。
- 凭证页面文案明确写成“管理 SSH 认证凭证”。

所以当前前端也没有“Windows 主机”的建模空间。

## 3. 目标与非目标

### 3.1 目标

本方案的目标是支持以下能力：

- 可新增 Windows 主机资产
- 可测试连通性
- 可通过浏览器打开 Windows 远程终端
- 可审计终端会话
- 可进行文件浏览、上传、下载、删除
- 可采集基础主机信息
- 不影响现有 Linux 主机能力

### 3.2 非目标

本方案首期不包含：

- 图形化远程桌面（RDP 画面转发）
- 剪贴板、文件拖拽、打印机映射等桌面特性
- 域控、Kerberos、CredSSP 的完整企业级集成
- Windows 运维自动化编排体系重构

## 4. 技术路线比较

| 方案 | 终端体验 | 文件管理 | 改造成本 | 风险 | 推荐度 |
|:-----|:---------|:---------|:---------|:-----|:-------|
| Windows over SSH（OpenSSH Server） | 最接近现有 Linux Web 终端 | 可继续复用 SFTP | 低到中 | 主要是命令/路径兼容 | 高 |
| Windows over WinRM | 更贴近 Windows 企业环境 | 需单独实现文件传输 | 中到高 | 交互式终端能力较弱、协议适配复杂 | 中 |
| Windows over RDP | 图形桌面 | 与终端无关 | 高 | 需要桌面代理与网关体系 | 低 |

### 4.1 推荐结论

推荐先做：

- `Windows + SSH(OpenSSH Server)` 作为 MVP

然后再做：

- `Windows + WinRM` 作为增强版

不建议首版直接做：

- `RDP 图形桌面`

原因很直接：

- 用户要求是“像 Linux 终端那样”，本质是文本终端，不是图形桌面。
- 当前系统的 WebSocket + xterm + SSH 链路已经成熟，复用价值最高。
- WinRM 更适合远程命令和自动化，不适合作为第一版交互终端的唯一基础。

## 5. 推荐方案设计

### 5.1 总体设计

将当前“SSH 主机”抽象为“远程主机”，新增两个核心维度：

- `osType`: `linux` / `windows`
- `connectionProtocol`: `ssh` / `winrm`

同时把当前 SSH 专用字段逐步泛化：

- `SSHUser` -> `loginUser`
- `Port` 保留，但其默认值由协议决定
- 终端、文件管理、采集逻辑都改为“按协议 + 按操作系统分发”

### 5.1.1 推荐抽象层

建议新增以下接口：

```go
type RemoteClient interface {
    Connect(ctx context.Context) error
    Close() error
    TestConnection(ctx context.Context) error
    Execute(ctx context.Context, command string) (string, error)
}

type RemoteTerminal interface {
    Start(ctx context.Context, cols, rows uint16) error
    Resize(cols, rows uint16) error
    Stdin() io.WriteCloser
    Stdout() io.Reader
    Stderr() io.Reader
    Close() error
}

type RemoteFileClient interface {
    ListDir(ctx context.Context, path string) ([]FileInfo, error)
    Upload(ctx context.Context, path string, reader io.Reader) error
    Download(ctx context.Context, path string, writer io.Writer) error
    Remove(ctx context.Context, path string) error
    ResolveHome(ctx context.Context) (string, error)
}

type HostCollector interface {
    CollectAll(ctx context.Context) (*SystemInfo, error)
}
```

实现层按协议/系统拆分：

- `SSHClient`
- `SSHRemoteTerminal`
- `SFTPFileClient`
- `LinuxCollector`
- `WindowsSSHCollector`
- `WinRMClient`
- `WinRMFileClient`
- `WindowsWinRMCollector`

### 5.2 数据模型改造

### 5.2.1 Host 表

建议在主机表新增字段：

| 字段 | 类型 | 说明 |
|:-----|:-----|:-----|
| `os_type` | varchar(20) | `linux` / `windows` |
| `connection_protocol` | varchar(20) | `ssh` / `winrm` |
| `login_user` | varchar(100) | 通用登录用户 |
| `shell_type` | varchar(20) | `bash` / `sh` / `powershell` / `cmd` |
| `default_directory` | varchar(255) | 默认目录 |

兼容策略：

- 现有 Linux 主机数据回填：
  - `os_type = linux`
  - `connection_protocol = ssh`
  - `login_user = ssh_user`
  - `shell_type = bash`
- 老字段 `ssh_user` 第一阶段保留，代码内部逐步切换到 `login_user`

### 5.2.2 Credential 表

建议扩展凭证模型：

| 字段 | 类型 | 说明 |
|:-----|:-----|:-----|
| `protocol` | varchar(20) | `ssh` / `winrm` |
| `auth_type` | varchar(20) | `password` / `key` |
| `username` | varchar(100) | 用户名 |
| `domain` | varchar(100) | Windows 域名，可选 |
| `use_https` | tinyint | WinRM 是否走 HTTPS |
| `verify_server_cert` | tinyint | 是否校验证书 |

兼容策略：

- 现有凭证默认回填 `protocol = ssh`
- `key` 仅在 SSH 下可选
- `winrm` 首版只支持 `password`

### 5.2.3 终端审计表

当前表名为 `ssh_terminal_sessions`，对 Windows over SSH 没问题，但对 WinRM 语义不准确。

建议分阶段处理：

- 阶段一：
  - 保留现有表
  - 增加 `protocol`、`os_type`、`shell_type` 字段
- 阶段二：
  - 如确实需要支持 WinRM 终端审计，再评估是否迁移为通用表 `terminal_sessions`

这样能降低一次性改造风险。

### 5.3 后端改造方案

### 5.3.1 主机创建与更新

需要调整以下对象：

- `internal/biz/asset/host.go`
- `internal/service/asset/host.go`
- `internal/biz/asset/host_usecase.go`

改造点：

- `HostRequest` 增加 `osType`、`connectionProtocol`、`loginUser`、`shellType`
- 保留对旧参数 `sshUser` 的兼容读取
- 根据协议设置默认端口：
  - SSH: `22`
  - WinRM HTTP: `5985`
  - WinRM HTTPS: `5986`

### 5.3.2 终端接入

保留当前路由：

- `GET /api/v1/asset/terminal/:id`

但内部改为按主机协议分发：

1. 读取主机信息
2. 根据 `connectionProtocol` 选择终端驱动
3. 建立远程终端
4. 复用现有 WebSocket / xterm 前端协议

推荐行为：

- `linux + ssh`：走现有实现
- `windows + ssh`：复用现有 SSH 终端，但默认 shell 为 PowerShell
- `windows + winrm`：第二阶段再接入

### 5.3.3 文件管理

保留现有接口：

- `GET /api/v1/hosts/:id/files`
- `POST /api/v1/hosts/:id/files/upload`
- `GET /api/v1/hosts/:id/files/download`
- `DELETE /api/v1/hosts/:id/files`

但内部逻辑改为：

- 由 `RemoteFileClient` 负责路径解释
- 前端不再默认写死 `~`
- 后端返回标准化路径信息，例如：
  - Linux: `/var/log`
  - Windows: `C:\\Users\\Administrator`

Windows 首版建议：

- `windows + ssh` 继续使用 SFTP
- 默认目录改为远程用户目录
- 路径展示按 Windows 风格输出

### 5.3.4 主机信息采集

采集器必须拆分为两套命令体系：

- Linux 采集器：保留现状
- Windows 采集器：使用 PowerShell 命令收集

Windows 采集建议字段：

- OS 版本
- 主机名
- CPU 核数
- 内存总量/已用量
- 磁盘容量
- 开机时长

建议命令来源：

- `Get-CimInstance Win32_OperatingSystem`
- `Get-CimInstance Win32_ComputerSystem`
- `Get-CimInstance Win32_Processor`
- `Get-PSDrive -PSProvider FileSystem`

### 5.3.5 测试连接

当前 `TestConnection` 只会走 SSH。

改造后逻辑应为：

- `ssh`：执行轻量命令检测
- `winrm`：执行远程空命令或基础 PowerShell 命令检测

返回结果统一为：

- 是否成功
- 延迟
- 失败原因

### 5.4 前端改造方案

### 5.4.1 主机表单

修改 `web/src/views/asset/Hosts.vue`：

- 新增“操作系统”：
  - Linux
  - Windows
- 新增“连接协议”：
  - SSH
  - WinRM
- 根据协议动态显示字段：
  - SSH: 用户名、端口、SSH 凭证
  - WinRM: 用户名、端口、WinRM 凭证、HTTP/HTTPS

动态默认值建议：

- Linux + SSH:
  - 端口 `22`
  - shell `bash`
- Windows + SSH:
  - 端口 `22`
  - shell `powershell`
- Windows + WinRM:
  - 端口 `5985/5986`
  - shell `powershell`

### 5.4.2 凭证表单

修改 `web/src/views/asset/Credentials.vue`：

- 页面文案从“SSH 凭证”改为“远程连接凭证”
- 增加协议类型选择：
  - SSH
  - WinRM
- 认证方式按协议受限：
  - SSH: `password` / `key`
  - WinRM: 首版仅 `password`

### 5.4.3 Web 终端

修改 `web/src/views/asset/Hosts.vue`：

- 欢迎语从“SSH Web 终端”改为“远程 Web 终端”
- 终端顶部展示：
  - 主机名
  - IP
  - 协议
  - 操作系统
  - shell 类型

### 5.4.4 文件管理

修改 `web/src/api/host.ts` 与对应文件管理页面：

- 去掉前端默认 `~`
- 首次进入时从后端获取默认目录
- Windows 主机展示盘符入口，如：
  - `C:\\`
  - `D:\\`

## 6. 推荐实施阶段

### 6.1 Phase 1: Windows over SSH MVP

目标：

- 能新增 Windows 主机
- 能测试连接
- 能打开 PowerShell 终端
- 能做文件管理
- 能保留终端审计
- 能采集基础主机信息

实施内容：

- 新增 `osType` / `connectionProtocol`
- 抽象 Remote 接口
- 保留现有终端路由
- 增加 Windows SSH 采集器
- 前端增加 Windows/协议选择

上线前提：

- Windows 主机已安装并启用 OpenSSH Server
- Windows 主机允许 SFTP
- 终端默认 shell 配置为 PowerShell

这一步完成后，用户体验会最接近“像 Linux 那样打开一个终端”。

### 6.2 Phase 2: Windows over WinRM

适用场景：

- 企业环境不允许开 SSH
- 需要贴近 AD/Windows 运维规范
- 需要更标准的 PowerShell Remoting

实施内容：

- 新增 WinRM 客户端适配层
- 增加 WinRM 凭证模型
- 实现 WinRM 文件传输
- 新增 WinRM 连通性检测
- 对终端体验做单独 PoC 验证

关键风险：

- WinRM 更适合命令执行，不一定适合作为完整交互式 PTY
- 编码、换行、命令回显与 xterm 交互需要额外处理
- HTTPS、自签证书、域认证策略会提高接入复杂度

因此第二阶段应先做技术验证，再进入正式开发。

### 6.3 Phase 3: 图形化远程桌面（可选）

如果后续需求变成“像堡垒机那样打开 Windows 桌面”，则属于新模块：

- RDP 协议接入
- Web 画面转发
- 鼠标键盘映射
- 剪贴板与会话控制

这不建议与本次“Windows 主机终端化接入”一起做。

## 7. 风险与难点

### 7.1 终端体验差异

即使接入 Windows，终端体验也不可能与 Linux 完全相同：

- PowerShell 输出风格不同
- 路径不同
- 命令不同
- 默认编码与换行不同

### 7.2 文件路径兼容

当前代码大量假设：

- `~`
- `/`
- `$HOME`

Windows 需要统一路径抽象，否则文件管理会持续出兼容问题。

### 7.3 采集命令差异

当前采集器完全是 Linux 命令集，不能复用到 Windows。

### 7.4 审计字段命名债务

当前 `ssh_terminal_sessions` 这个表名会限制未来扩展，需要在 Phase 2 前决定是否泛化。

## 8. 兼容性策略

### 8.1 对现有 Linux 主机的兼容

必须保证以下行为不变：

- 现有 Linux 主机无需重建
- 现有 SSH 凭证继续可用
- 现有终端权限模型不变
- 现有文件管理接口不变

### 8.2 数据迁移策略

建议采用“新增字段 + 数据回填”方式：

1. 给主机表新增 `os_type`、`connection_protocol`、`login_user`、`shell_type`
2. 给凭证表新增 `protocol` 等字段
3. 将旧数据批量回填为 Linux + SSH
4. 代码切换到新字段
5. 最后再评估是否移除 `ssh_user`

## 9. 建议排期

按可控范围估算：

| 阶段 | 内容 | 预估 |
|:-----|:-----|:-----|
| Phase 1 | Windows over SSH MVP | 5 到 8 个工作日 |
| Phase 2 | WinRM 支持与 PoC | 5 到 10 个工作日 |
| Phase 3 | RDP 远程桌面 | 单独评估 |

说明：

- 这里是技术估算，不含联调环境准备时间。
- 如果需要支持域账号、HTTPS 证书校验、企业代理策略，Phase 2 会继续增长。

## 10. 最终建议

如果你的目标是：

- “能把 Windows 主机纳入资产”
- “能远程打开命令行”
- “体验尽量像现在的 Linux 终端”

那么最合理的方案是：

- 第一版只做 `Windows + OpenSSH Server`

如果你的目标是：

- “必须遵守 Windows 企业远程管理规范”
- “不能开 SSH，只能 WinRM”

那么应该：

- 先做 `WinRM 技术验证`
- 再决定是否把 WinRM 作为正式接入协议

如果你的目标是：

- “打开 Windows 图形桌面”

那应该单独立项做：

- `RDP 网关/桌面代理`

不建议把它混进本次终端方案。
