# OpsHub Linux Agent 与主机监控集成设计方案

## 1. 背景

当前 OpsHub 的主机资源信息主要还是依赖手动点击“采集信息”后，由后端通过 SSH 进入目标机执行命令，再把 CPU、内存、磁盘等摘要写回 `hosts` 表。

这条链路有三个明显问题：

1. 主机列表数据不是自动刷新，用户感知上就是“必须手动点一下才有数据”。
2. 只能稳定覆盖 SSH 可达场景，无法自然扩展到 Agent 生命周期管理。
3. 没有 Prometheus 时序存储，后续做历史趋势、告警、主机监控总览都比较吃力。

你的目标已经很明确：

1. 先只做 Linux。
2. 增加一个类似截图的“Agent 管理”页面。
3. 在 Agent 管理页选择 Linux 主机后，可以把 Agent 自动部署到目标机。
4. 部署成功后，主机管理页不再依赖手动采集，而是自动显示：
   - 内网 IP / 公网 IP
   - CPU
   - 内存
   - 磁盘
   - 进程
   - 端口
   - 主机配置摘要
5. 把 Prometheus 作为监控时序后端接入项目。

本方案只覆盖 Linux Agent 与主机监控集成，Windows 暂不纳入本阶段。

## 2. 当前项目现状

### 2.1 已有能力

当前仓库已经具备下面这些基础：

1. 主机资产模型已经扩展出：
   - `management_mode`
   - `collect_status`
   - `collect_error`
   - `last_collect_at`
   - `agent_id`
   - `agent_version`
   - `agent_last_heartbeat_at`
   - CPU / 内存 / 磁盘摘要字段
2. 后端已经有 `HostCollector` 抽象，区分：
   - `ssh`
   - `winrm`
   - `agent`
3. 前端主机页已经有 Windows 的 Agent/WinRM/SSH 管理方式 UI 雏形。
4. 后端已经有一个“公共 Agent 注册/上报”入口雏形。

### 2.2 当前缺口

但离 Linux Agent + Prometheus 的完整能力还差以下关键部分：

1. Linux 主机编辑页还没有完整暴露 Agent 作为主采集通道的交互。
2. 现有 Agent 引导逻辑仍然是 Windows 导向，不适合直接复用到 Linux。
3. 项目当前 `docker-compose.yml` 里没有 Prometheus 服务。
4. 现有 Monitor 插件只覆盖域名监控、告警通道、告警日志，不包含主机时序监控。
5. 没有独立的 Agent 生命周期表，也没有批量部署/卸载/重启/查看进度的后端模型。
6. 没有“主机进程/端口/配置摘要”的最新快照存储模型。

### 2.3 当前链路为什么只能“手动点采集”

本质原因不是前端，而是架构：

1. 主机列表展示读取的是数据库里的摘要字段。
2. 摘要字段只有在执行 `/api/v1/hosts/:id/collect` 或批量采集后才会被更新。
3. 没有 Agent 定时上报，也没有 Prometheus 定时抓取后的回写逻辑。

所以如果不引入“主动上报”或“定时抓取 + 回写摘要”，主机页就会继续停留在手动刷新模式。

## 3. 对 AutoOps 的调研结论

本次设计参考了 `AutoOps` 仓库的 Agent 与监控实现。

参考仓库：

- https://github.com/opsre/AutoOps

重点参考点：

1. `README.md`
2. `api/api/monitor/model/agent.go`
3. `api/api/monitor/service/agent.go`
4. `api/api/monitor/service/monitorService.go`
5. `api/common/agent/agent.go`
6. `api/router/monitor/monitor.go`
7. `api/common/templates/07-prometheus/README.md`
8. `docker/prometheus/prometheus.yml`

### 3.1 AutoOps 值得借鉴的点

`AutoOps` 的方向是对的，尤其有三点值得借：

1. Agent 生命周期独立建模。
   - 有版本、状态、监听端口、安装进度、最后心跳等字段。
2. Agent 管理和主机监控拆成两条能力线。
   - Agent 管理负责部署、重启、卸载、状态。
   - 主机监控负责查 Prometheus、展示 CPU/内存/磁盘/端口/进程。
3. 主机趋势数据不是直接查数据库，而是查 Prometheus。

### 3.2 AutoOps 不适合直接照搬的点

不能直接照抄，主要有四个原因：

1. `AutoOps` 的默认实现里存在硬编码心跳 URL、Token、Pushgateway 地址，不适合产品化接入。
2. 它在部署时动态生成并编译 Agent，可用但不够稳，不适合生产环境的可追溯发布。
3. 它把 Pushgateway 和 Prometheus 混在一条链路里，复杂度偏高，不适合作为你项目的第一版。
4. 它虽然有 Linux Agent 生命周期模型，但 Windows 在 README 里仍是“规划中”，说明它本身也不是你当前需求的完整现成答案。

### 3.3 本项目应该怎么借

建议采取“借架构，不借实现细节”的策略：

1. 借它的 Agent 生命周期页面思路。
2. 借它的“Prometheus 查历史，数据库存最新摘要”的分层。
3. 不借它的硬编码 URL/Token。
4. 不借它的部署时即时编译模式。
5. 不把 Pushgateway 作为第一版的主路径。

## 4. 为什么“只加 Prometheus 容器”不够

这是这次方案里最关键的判断。

Prometheus 只是时序数据库和抓取器，不是主机部署系统，也不是 CMDB 采集器。

如果只加一个 Prometheus 容器，仍然解决不了下面这些问题：

1. 谁把主机 CPU/内存/磁盘暴露给 Prometheus？
   - 必须有 `node_exporter` 或自定义 Agent。
2. 谁负责 Agent 的安装、重启、卸载、版本展示？
   - Prometheus 不做这个。
3. 谁负责最后心跳、部署进度、错误原因？
   - Prometheus 不适合存这些生命周期数据。
4. 谁负责“当前内网 IP、公网 IP、进程列表、监听端口、配置摘要”这种快照信息？
   - Prometheus 更适合时序指标，不适合做完整最新快照的主存储。

结论：

1. Prometheus 必须加。
2. 但 Prometheus 只能是监控后端，不是 Agent 方案的替代品。
3. 第一版要做的是“Linux Agent + Prometheus”，而不是“Prometheus alone”。

## 5. 方案对比

| 方案 | 是否满足 Agent 管理页 | 是否支持自动展示主机摘要 | 是否支持趋势历史 | 是否支持进程/端口/配置摘要 | 实施复杂度 | 结论 |
| --- | --- | --- | --- | --- | --- | --- |
| 继续用 SSH 手动采集 | 否 | 否 | 否 | 弱 | 低 | 不推荐 |
| Prometheus + node_exporter | 否 | 部分 | 是 | 弱 | 中 | 不够 |
| 自定义 Linux Agent + Prometheus 拉取 | 是 | 是 | 是 | 是 | 中 | 推荐 |
| 自定义 Linux Agent + Pushgateway | 是 | 是 | 是 | 是 | 中高 | 暂不作为第一版 |

### 推荐结论

第一版明确采用：

**OpsHub Linux Agent + Prometheus Scrape**

也就是：

1. Linux 主机部署自定义 `opshub-agent`
2. Agent 定时把最新快照上报给 OpsHub
3. Agent 同时暴露 `/metrics`
4. Prometheus 定时抓取 Agent 指标
5. 主机列表看数据库最新摘要
6. 历史趋势和未来告警看 Prometheus

这样既满足“Agent 管理”，又满足“主机自动展示”，还给后续历史图表和告警留了标准扩展位。

## 6. 目标范围

### 6.1 本阶段包含

1. Linux Agent 生命周期管理
2. Agent 管理页
3. 批量部署 / 重装 / 重启 / 卸载
4. Agent 注册、心跳、快照上报
5. Prometheus 容器接入
6. 主机管理页自动展示最新 CPU / 内存 / 磁盘 / IP 摘要
7. 主机详情页展示：
   - 进程列表
   - 监听端口
   - 主机配置摘要
8. Monitor 插件新增主机监控趋势接口

### 6.2 本阶段不包含

1. Windows Agent
2. WinRM 优化
3. Grafana 面板打包
4. Alertmanager 接入
5. 应用级 exporter 编排
6. 任意业务配置文件的深度采集与差异比对

这里的“配置”在第一版中定义为：

- 操作系统版本
- 内核
- 架构
- 主机名
- 时区
- systemd/容器运行时摘要
- 磁盘挂载摘要
- 网络接口摘要

不直接扩展到 Nginx、MySQL、Redis 等业务配置文件全文采集。

## 7. 总体架构

### 7.1 逻辑分层

```text
                +----------------------+
                |   OpsHub Frontend    |
                +----------------------+
                           |
                           v
                +----------------------+
                |   OpsHub Backend     |
                |  Asset + Monitor API |
                +----------------------+
                   |              |
                   |              |
                   |              v
                   |      +------------------+
                   |      |   Prometheus     |
                   |      +------------------+
                   |                ^
                   |                |
                   v                |
        +--------------------+      |
        | SSH Deploy Worker  |      |
        +--------------------+      |
                   |                |
                   v                |
             +-----------+  /metrics|
             | Linux     |----------+
             | Agent     |
             +-----------+
                   |
                   | register / heartbeat / report
                   v
             OpsHub Backend
```

### 7.2 责任边界

#### 资产模块负责

1. Agent 部署、重启、卸载
2. Agent 生命周期状态
3. 最新主机摘要回写
4. 最新进程 / 端口 / 配置快照存储

#### Monitor 插件负责

1. Prometheus 配置与接入
2. 查询主机时序指标
3. 历史趋势接口
4. 未来告警扩展

#### Linux Agent 负责

1. 注册
2. 心跳
3. 主机快照上报
4. 暴露 Prometheus 指标

## 8. 核心设计原则

1. 主机列表不要逐行实时查 Prometheus。
   - 列表页应读数据库里的最新摘要，否则页面越大越慢。
2. 历史趋势不要存数据库。
   - Prometheus 更适合时序。
3. 生命周期和时序指标不要混存。
   - 生命周期进数据库，时序进 Prometheus。
4. 进程和端口要区分“最新快照”和“指标标签”。
   - 最新快照用于详情页。
   - 指标只保留受控的 Top N，避免高基数炸库。
5. Linux Agent 第一版必须使用预编译包，不在部署时临时编译。
6. 不依赖手动“点击采集”才能看到数据。

## 9. 数据模型设计

### 9.1 沿用 `hosts` 表的字段

当前 `hosts` 表里已有以下字段，可以继续保留并作为主机列表的摘要来源：

1. `management_mode`
2. `collect_status`
3. `collect_error`
4. `last_collect_at`
5. `agent_id`
6. `agent_version`
7. `agent_last_heartbeat_at`
8. `cpu_cores`
9. `cpu_usage`
10. `memory_total`
11. `memory_used`
12. `memory_usage`
13. `disk_total`
14. `disk_used`
15. `disk_usage`
16. `os`
17. `kernel`
18. `arch`
19. `uptime`
20. `hostname`

### 9.2 建议新增到 `hosts` 表的字段

为了让主机列表直接显示内外网 IP 和 Agent 摘要，建议给 `hosts` 增加：

1. `primary_private_ip`
2. `primary_public_ip`
3. `agent_status`
4. `agent_port`
5. `agent_last_report_at`
6. `agent_last_error`

原因很简单：

1. 主机列表是高频页面。
2. 主机列表不应该每次都 join 一堆 JSON 快照表。
3. 常用字段做扁平化摘要，体验和性能都更稳。

### 9.3 新增 `asset_agents` 表

建议单独建立 Agent 生命周期表。

推荐字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `host_id` | 主机 ID，唯一 |
| `agent_id` | Agent 唯一 ID |
| `version` | Agent 版本 |
| `status` | `pending/deploying/running/offline/error/uninstalling/uninstalled` |
| `listen_port` | Agent 监听端口 |
| `install_path` | 安装目录，默认 `/opt/opshub-agent` |
| `service_name` | systemd 服务名，默认 `opshub-agent` |
| `install_progress` | 0-100 |
| `install_stage` | 当前阶段，如 `uploading/starting/waiting_register` |
| `last_heartbeat_at` | 最后心跳 |
| `last_report_at` | 最后快照上报 |
| `last_error` | 最近错误 |
| `deployed_by` | 部署人 |
| `deployed_at` | 部署时间 |
| `updated_at` | 更新时间 |

### 9.4 新增 `asset_host_inventory` 表

建议存“最新完整快照”，而不是把所有细项都塞进 `hosts`。

推荐字段：

| 字段 | 说明 |
| --- | --- |
| `host_id` | 主机 ID，唯一 |
| `private_ips_json` | 内网 IP 列表 |
| `public_ips_json` | 公网 IP 列表 |
| `interfaces_json` | 网卡摘要 |
| `disks_json` | 磁盘分区摘要 |
| `top_processes_json` | Top 进程快照 |
| `listening_ports_json` | 监听端口快照 |
| `config_summary_json` | 主机配置摘要 |
| `collected_at` | 快照时间 |
| `updated_at` | 更新时间 |

### 9.5 新增 `asset_agent_jobs` 表

要做“部署进度”“批量部署”“失败原因”，就需要任务表。

推荐字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `job_type` | `deploy/reinstall/restart/uninstall` |
| `host_id` | 主机 ID |
| `status` | `pending/running/success/failed` |
| `progress` | 0-100 |
| `stage` | 当前阶段 |
| `message` | 当前提示 |
| `error` | 错误信息 |
| `operator_id` | 操作人 |
| `started_at` | 开始时间 |
| `finished_at` | 结束时间 |

## 10. Agent 设计

### 10.1 Agent 形态

推荐采用单一 Go 静态二进制：

1. `opshub-agent-linux-amd64`
2. `opshub-agent-linux-arm64`

运行方式：

1. systemd service
2. 默认安装目录 `/opt/opshub-agent`
3. 默认配置目录 `/etc/opshub-agent`
4. 默认数据目录 `/var/lib/opshub-agent`
5. 默认监听端口 `19100/tcp`

不建议把默认端口设成 `9100`，原因是：

1. 很多环境已经把 `9100` 留给 `node_exporter`
2. 后续如果用户还要并存 `node_exporter`，会直接冲突

### 10.2 Agent 负责采集的内容

#### 摘要类

1. OS
2. Kernel
3. Arch
4. Hostname
5. Uptime
6. CPU 核数与实时使用率
7. 内存总量、已用量、使用率
8. 磁盘总量、已用量、使用率

#### 详情类

1. 内网 IP 列表
2. 公网 IP 列表
3. 网卡列表
4. 分区列表
5. Top N 进程
6. 监听端口列表
7. 主机配置摘要

#### 时序类

1. CPU 使用率
2. Memory 使用率
3. Disk 使用率
4. Load
5. Network RX/TX
6. Disk IO
7. Top N 进程 CPU / 内存
8. 监听端口状态
9. Agent 自身状态

### 10.3 配置摘要的范围

第一版的 `config_summary_json` 建议只覆盖以下内容：

1. `os_release`
2. `kernel`
3. `arch`
4. `timezone`
5. `service_manager`
6. `container_runtime`
7. `mounts`
8. `agent_version`

不要在第一版里把它做成“采集任意配置文件内容”。

### 10.4 心跳与快照周期

推荐默认值：

1. `heartbeat_interval = 30s`
2. `report_interval = 60s`
3. `prometheus_scrape_interval = 15s`

### 10.5 注册与鉴权

建议使用两段式 Token：

1. **引导 Token**
   - 一次性或短时有效
   - 默认 30 分钟过期
   - 用于首次注册
2. **Agent Access Token**
   - 注册成功后下发
   - 用于 heartbeat / report
   - 需要可轮换

必须满足：

1. 不能硬编码 URL
2. 不能硬编码 Token
3. 所有 Agent 访问地址都从 `server.external_url` 或明确配置项生成

## 11. Prometheus 设计

### 11.1 接入方式

第一版采用 **Prometheus 主动拉取 Agent**。

理由：

1. 模型简单
2. 便于后续主机监控图表
3. 符合 Prometheus 标准习惯
4. 比 Pushgateway 更适合持续运行的 Agent

### 11.2 Target 管理方式

推荐使用 `file_sd_configs`。

原因：

1. 实现简单
2. 不需要额外做服务发现服务
3. 后端在 Agent 注册、卸载、IP 变更时只需要重写一个 JSON 文件

示例：

```yaml
scrape_configs:
  - job_name: 'opshub-agent'
    scrape_interval: 15s
    metrics_path: /metrics
    file_sd_configs:
      - files:
          - /etc/prometheus/file_sd/opshub-agents.json
```

`opshub-agents.json` 由 OpsHub 后端生成，内容示例：

```json
[
  {
    "targets": ["10.0.1.23:19100"],
    "labels": {
      "host_id": "12",
      "host_name": "prod-api-01",
      "group_id": "3",
      "os_type": "linux"
    }
  }
]
```

### 11.3 Prometheus 在项目里的角色

Prometheus 只负责：

1. 存时间序列
2. 提供查询接口
3. 为未来告警提供基础

Prometheus 不负责：

1. Agent 部署
2. 生命周期状态
3. 最新进程/端口快照
4. 主机资产管理

## 12. 指标命名建议

建议使用统一前缀，避免和未来 `node_exporter` 混淆。

推荐：

1. `opshub_host_cpu_usage_percent`
2. `opshub_host_memory_usage_percent`
3. `opshub_host_disk_usage_percent`
4. `opshub_host_load`
5. `opshub_host_net_receive_bytes_per_second`
6. `opshub_host_net_send_bytes_per_second`
7. `opshub_host_disk_read_bytes_per_second`
8. `opshub_host_disk_write_bytes_per_second`
9. `opshub_process_cpu_percent`
10. `opshub_process_memory_percent`
11. `opshub_port_listening`
12. `opshub_agent_up`
13. `opshub_agent_last_report_timestamp`

### 12.1 高基数控制

这是第一版必须明确控制的点。

不要无限制上报：

1. 所有进程
2. 所有网络连接
3. 所有随机标签

建议规则：

1. Prometheus 指标只保留 Top 10 进程
2. 端口只保留监听端口
3. 详情页需要的完整列表放数据库快照，不全放 Prometheus

## 13. 部署链路设计

### 13.1 主路径：通过 SSH 自动部署

这条链路最贴合当前 OpsHub 现状，因为 Linux 主机本身就已经有 SSH 凭据模型。

流程：

1. 在 Agent 管理页选择一个或多个 Linux 主机
2. 后端校验：
   - 主机存在
   - `osType=linux`
   - 存在 SSH 用户名与 SSH 凭据
3. 创建 `asset_agent_jobs`
4. 后端通过 SSH/SFTP 上传：
   - Agent 二进制
   - 配置文件
   - systemd unit 文件
5. 在目标机执行：
   - `mkdir -p /opt/opshub-agent /etc/opshub-agent /var/lib/opshub-agent`
   - `chmod +x /opt/opshub-agent/opshub-agent`
   - `systemctl daemon-reload`
   - `systemctl enable --now opshub-agent`
6. Agent 首次注册
7. 后端更新：
   - `asset_agents`
   - `hosts`
   - `asset_host_inventory`
8. 生成 Prometheus target 文件并 reload

### 13.2 备用路径：生成手动安装命令

这条路径建议保留，但不作为主路径。

适用场景：

1. SSH 不可达
2. 目标机必须人工审批安装
3. 需要跳板机外安装

命令形式建议：

```bash
curl -fsSL https://<opshub>/api/v1/public/agents/install.sh?token=<bootstrap-token> | bash
```

### 13.3 为什么不用“部署时即时编译”

不推荐直接复用 AutoOps 的编译式部署，原因：

1. 生产环境后端容器不应依赖完整 Go 编译链
2. 批量部署时编译耗时和日志都更难控制
3. 版本不可追溯
4. amd64 / arm64 管理会更混乱

推荐做法：

1. 在 CI 里预编译 Agent
2. 后端只分发对应架构的版本包

## 14. 运行链路设计

### 14.1 注册

首次安装完成后：

1. Agent 用 bootstrap token 调用 `/api/v1/public/agents/register`
2. 后端生成 `agent_id`
3. 后端下发 access token 和基础配置

### 14.2 心跳

Agent 每 30 秒调用 `/api/v1/public/agents/heartbeat`：

1. 更新 `asset_agents.last_heartbeat_at`
2. 更新 `hosts.agent_last_heartbeat_at`
3. 刷新 `agent_status`

### 14.3 快照上报

Agent 每 60 秒调用 `/api/v1/public/agents/report`：

1. 回写 `hosts` 摘要字段
2. 更新 `asset_host_inventory`
3. 更新 `asset_agents.last_report_at`
4. 清理错误状态

### 14.4 Prometheus 抓取

Prometheus 每 15 秒抓取 `/metrics`：

1. 时序数据进入 Prometheus
2. Monitor 插件从 Prometheus 查历史

## 15. IP 采集策略

### 15.1 内网 IP

由 Agent 从本机网卡直接采集，过滤掉：

1. `lo`
2. Docker bridge
3. link-local

### 15.2 公网 IP

公网 IP 不能简单等同于“网卡 IP”，因为很多主机在 NAT 后面。

建议优先级：

1. 云主机导入时已有公网 IP，则直接沿用
2. 如果 Agent 注册请求的来源地址是公网地址，则可记为候选公网 IP
3. 如果开启了公网探测配置，可由 Agent 调用回显接口获取
4. 如果都拿不到，则为空

这意味着：

1. `primary_private_ip` 必须做
2. `primary_public_ip` 只能做“尽力而为”

## 16. API 设计

### 16.1 资产侧私有 API

建议新增：

1. `GET /api/v1/agents`
2. `GET /api/v1/agents/:hostId`
3. `POST /api/v1/agents/deploy`
4. `POST /api/v1/agents/:hostId/reinstall`
5. `POST /api/v1/agents/:hostId/restart`
6. `DELETE /api/v1/agents/:hostId`
7. `GET /api/v1/agents/jobs/:jobId`
8. `GET /api/v1/hosts/:id/inventory`

### 16.2 Agent 公共 API

建议统一到现有 public agent 路由下：

1. `GET /api/v1/public/agents/install.sh`
2. `POST /api/v1/public/agents/register`
3. `POST /api/v1/public/agents/heartbeat`
4. `POST /api/v1/public/agents/report`

### 16.3 Monitor 插件 API

建议新增主机监控接口：

1. `GET /api/v1/plugins/monitor/hosts/:id/overview`
2. `GET /api/v1/plugins/monitor/hosts/:id/history`
3. `GET /api/v1/plugins/monitor/hosts/:id/processes`
4. `GET /api/v1/plugins/monitor/hosts/:id/ports`

## 17. 页面设计

### 17.1 新增“Agent 管理”页

建议放到：

- `资产管理 > Agent管理`

原因：

1. 这是主机受管能力，不是纯监控视角
2. 它和主机资产、SSH 凭据、部署动作强相关

列表字段建议：

1. 主机名称
2. 内网 IP
3. 公网 IP
4. Agent 版本
5. Agent 状态
6. 监听端口
7. 安装进度
8. 健康状态
9. 最后心跳
10. 更新时间
11. 操作

操作建议：

1. 部署
2. 重装
3. 重启
4. 卸载
5. 查看安装日志
6. 复制手动安装命令

### 17.2 主机管理页改造

主机页需要完成两个变化：

#### 变化一：不再依赖手动采集才有数据

对于 `management_mode=agent` 的 Linux 主机：

1. 列表直接展示数据库里的最新摘要
2. 页面定时刷新即可
3. 不再把“点击采集”当成唯一入口

#### 变化二：详情页增加更多观察维度

建议把主机详情页改成 Tab 结构：

1. `概览`
2. `进程`
3. `端口`
4. `配置`
5. `监控`
6. `Agent`

### 17.3 采集按钮语义调整

对于 Agent 管理模式的 Linux 主机：

1. “采集信息”按钮建议改成“刷新状态”
2. 第一版不强依赖后端主动下发命令
3. 按钮行为只负责刷新页面数据，不承担真正采集触发

真正的采集由 Agent 定时上报完成。

## 18. 与现有代码的落点映射

建议按下面路径落地：

### 18.1 后端资产模块

1. `internal/biz/asset/host.go`
   - 扩展 Linux Agent 相关字段与默认逻辑
2. `internal/biz/asset/host_usecase.go`
   - 区分 `ssh` 手动采集和 `agent` 自动摘要
3. `internal/biz/asset/host_collector.go`
   - 让 Linux Agent 成为正式采集通道
4. `internal/biz/asset/agent.go`
   - 从“Windows Agent 雏形”重构成“通用 Agent 核心”
5. 新增：
   - `internal/biz/asset/agent_deploy_usecase.go`
   - `internal/biz/asset/agent_inventory.go`
   - `internal/biz/asset/agent_job.go`

### 18.2 后端 Service / HTTP

1. `internal/service/asset/agent.go`
   - 扩展部署、重启、卸载、列表查询
2. `internal/server/asset/http.go`
   - 注册 Agent 管理私有路由
   - 复用并扩展公共 Agent 注册/上报路由

### 18.3 数据层

建议新增：

1. `internal/data/asset/agent_repo.go`
2. `internal/data/asset/agent_job_repo.go`
3. `internal/data/asset/host_inventory_repo.go`

### 18.4 前端

1. 新增 `web/src/views/asset/Agents.vue`
2. 新增 `web/src/api/agent.ts`
3. 修改 `web/src/views/asset/Hosts.vue`
4. 修改 `web/src/router/index.ts`
5. 补充权限点与菜单

### 18.5 Monitor 插件

1. `plugins/monitor/server/router.go`
2. `plugins/monitor/server/handler.go`
3. 新增主机监控 handler / service / model
4. 前端插件页新增：
   - 主机监控总览
   - 主机详情趋势

### 18.6 配置与部署

1. `internal/conf/conf.go`
2. `config/config.yaml.example`
3. `docker-compose.yml`
4. 新增：
   - `deploy/prometheus/prometheus.yml`
   - `deploy/prometheus/file_sd/`
5. 新增 `cmd/agent/main.go`

## 19. 配置项设计

建议新增配置分组：

```yaml
agent:
  enabled: true
  default_listen_port: 19100
  heartbeat_interval_seconds: 30
  report_interval_seconds: 60
  heartbeat_timeout_seconds: 180
  install_path: /opt/opshub-agent
  config_path: /etc/opshub-agent
  service_name: opshub-agent
  package_dir: /app/agent-bundles

monitoring:
  prometheus:
    enabled: true
    url: http://prometheus:9090
    reload_url: http://prometheus:9090/-/reload
    file_sd_path: /etc/prometheus/file_sd/opshub-agents.json
    scrape_interval: 15s
```

## 20. 分阶段实施建议

### Phase 1：Linux Agent 生命周期闭环

目标：

1. Agent 管理页可看列表
2. 支持单台/批量部署
3. Agent 注册、心跳、快照上报跑通
4. 主机管理页自动显示最新摘要

### Phase 2：Prometheus 接入与主机历史趋势

目标：

1. `docker-compose.yml` 增加 Prometheus
2. 自动生成 file_sd targets
3. Monitor 插件新增主机趋势接口与页面

### Phase 3：进程、端口、配置详情完善

目标：

1. 主机详情页展示进程和端口
2. 配置摘要结构稳定
3. 指标高基数控制落地

### Phase 4：告警与大盘

目标：

1. 主机阈值告警
2. 汇总大盘
3. 可选接入 Grafana / Alertmanager

## 21. 验收标准

完成第一版后，应满足：

1. 可以在 Agent 管理页选择一个或多个 Linux 主机并发起部署。
2. 部署后 1 分钟内，主机管理页无需手动点击采集即可显示 CPU / 内存 / 磁盘摘要。
3. Agent 管理页可见：
   - 版本
   - 状态
   - 监听端口
   - 安装进度
   - 最后心跳
4. 主机详情页可见：
   - 内网 IP / 公网 IP
   - Top 进程
   - 监听端口
   - 配置摘要
5. Prometheus target 会在 Agent 注册后自动加入。
6. Monitor 插件可以查询 1 小时内主机 CPU / 内存 / 磁盘趋势。

## 22. 风险与注意事项

### 22.1 网络可达性

Prometheus 必须能访问 Agent 暴露端口。

如果 Prometheus 无法访问目标主机：

1. 最新摘要仍可通过 Agent 上报显示
2. 但历史趋势会缺失

### 22.2 公网 IP 识别并不绝对准确

NAT 场景下，公网 IP 只能尽力识别，不能承诺 100% 真实反映“云厂商绑定 EIP”。

### 22.3 进程/端口指标高基数风险

如果无上限地把所有进程、所有端口都做成 Prometheus labels，会直接带来性能问题。

必须控制：

1. Top N
2. 监听端口限定
3. 快照与指标分离

### 22.4 不要直接在主机列表页查 Prometheus

否则一页几十台主机时，接口 fanout 会非常重，体验会明显下降。

### 22.5 不要把当前 Windows Agent 代码直接强行复用到 Linux

当前仓库里的 Agent 逻辑是 Windows 导向，应重构成：

1. 通用 Agent 核心
2. Linux 安装脚本
3. Windows 安装脚本

而不是继续在一个文件里堆平台分支。

## 23. 最终推荐结论

本项目的正确第一步不是“只上一个 Prometheus 容器”，而是：

**先落 Linux Agent 生命周期，再接 Prometheus 时序存储。**

第一版的最优组合是：

1. Linux 主机继续沿用现有 SSH 凭据体系做部署入口
2. 新增独立 `Agent 管理` 页做批量部署和状态管理
3. Agent 定时把最新快照上报 OpsHub
4. Prometheus 定时抓取 Agent `/metrics`
5. 主机管理页读数据库摘要
6. Monitor 插件读 Prometheus 做历史趋势

这条路线最符合你当前项目的代码基础，也最接近你希望实现的产品形态。
