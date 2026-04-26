# 会话录制生命周期修复方案

本文档覆盖会话审计中 Windows 桌面录屏和 SSH 终端录制的第一、二阶段修复方案。目标是解决页面关闭、网络中断、并发连接等场景下，审计记录长时间停留在录制中或进行中的问题。

## 背景问题

当前 Windows 桌面会话创建后直接记录为 `active`，会话结束主要依赖前端页面关闭时调用 `/api/v1/desktop-sessions/:id/close`。浏览器在 `beforeunload` 阶段可能取消异步请求，导致平台记录无法闭合。后端也没有定时兜底扫描，所以孤儿会话会持续显示进行中，时长不断增长。

当前 SSH 终端会话由 WebSocket 驱动关闭，正常情况下 WebSocket 读失败后关闭 SSH 会话并回写审计记录。但实现里存在秒级会话 ID、秒级录制文件名、关闭流程持有全局锁等问题。在同一台主机短时间并发打开终端时，内存会话可能互相覆盖，录制文件也可能被覆盖，最终留下状态为 `recording` 的审计记录。

## 第一阶段：止血修复

第一阶段不引入大规模协议改造，优先保证现有产品行为可恢复、可闭合、并发安全。

### 1. Windows 桌面关闭信号增强

前端桌面页增加稳定关闭路径：

- 保留主动点击关闭按钮时的正常 API 调用。
- 页面卸载场景使用 `pagehide` 和 `beforeunload` 触发关闭。
- 卸载场景优先使用 `navigator.sendBeacon`，失败时退回 `fetch(..., { keepalive: true })`。
- 关闭标记只在请求成功提交或排队后设置，避免一次失败后阻断后续重试。
- 清理 Guacamole 本地 token 和心跳定时器，避免已关闭页面继续产生状态更新。

预期效果：常规关闭标签页、刷新、浏览器导航离开时，即使 axios 请求被取消，也有更大概率把关闭事件提交到后端。

### 2. Windows 桌面后端心跳与孤儿会话兜底

增加桌面会话心跳接口，并以 `updated_at` 作为轻量心跳时间：

- 桌面页打开后每 30 秒调用 `/api/v1/desktop-sessions/:id/heartbeat`。
- 后端校验会话归属，只允许会话创建者续约。
- 心跳只更新 `updated_at`，不改变会话状态和结束原因。
- 后端启动定时扫描 `active`、`creating` 会话。
- 如果会话超过心跳阈值仍未更新，则标记为 `timeout`，写入 `ended_at` 和 `close_reason=heartbeat_timeout`。

预期效果：即使关闭请求完全丢失，后端也能在心跳过期后自动闭合平台审计记录。

### 3. SSH 会话 ID 唯一化

SSH 内存会话 ID 从 `hostID + Unix 秒` 改为 UUID：

- 同一用户或多个用户在同一台主机一秒内打开多个终端，不会互相覆盖 `TerminalManager.sessions`。
- 关闭指定标签页时，只关闭对应 WebSocket/SSH 会话。

预期效果：并发打开终端不会导致前一个审计记录丢失关闭回写机会。

### 4. SSH 录制文件名唯一化

Asciinema 录制文件名从秒级时间戳改为纳秒时间戳加 UUID：

- 避免同一秒多个会话写入同一个 `.cast` 文件。
- 保留时间前缀，便于人工排查文件。

预期效果：每个终端审计记录对应独立录制文件，播放和下载不会串线。

### 5. SSH 关闭流程缩小锁粒度

调整 `TerminalManager.CloseSession`：

- 全局 map 锁只负责查找和删除会话。
- 录制器关闭、SSH 连接关闭、数据库更新、风险事件写入都放到锁外执行。
- 单个会话内部使用 `sync.Once` 防止重复关闭。

预期效果：某个会话关闭时即使文件或数据库操作较慢，也不会阻塞其他会话创建、查询和关闭。

## 第二阶段：生命周期重构

第二阶段解决更深层的连接感知和审计准确性问题，适合在第一阶段稳定后继续推进。

### 1. SSH WebSocket 单写通道

当前 stdout 和 stderr goroutine 都会直接写 WebSocket。第二阶段改为：

- stdout/stderr 只读取 SSH 输出并投递到 channel。
- 单独 writer goroutine 串行调用 `WriteMessage`。
- 每次写入设置 `SetWriteDeadline`。
- 写失败时取消上下文并触发会话关闭。

收益：消除 gorilla/websocket 并发写风险，避免写阻塞导致 handler 卡在 `WaitGroup.Wait()`。

### 2. SSH 连接心跳与取消链路

为 SSH WebSocket 增加明确的上下文取消模型：

- 后端定时发送 ping。
- pong handler 刷新读 deadline。
- 前端可选发送应用层心跳或依赖 WebSocket pong。
- 任一读写错误、心跳超时、SSH 管道 EOF 都统一触发 cancel。
- cancel 后关闭 SSH session/client、关闭 writer channel、回写审计记录。

收益：断网、关页、代理断连后能更快且一致地结束录制。

### 3. SSH 审计时间字段补齐

当前 SSH 审计列表用 `created_at` 和 `duration` 推算开始/结束时间，语义不准确。第二阶段补齐：

- `ssh_terminal_sessions.started_at`
- `ssh_terminal_sessions.ended_at`
- `ssh_terminal_sessions.close_reason`

同步调整：

- 创建会话时写 `started_at`。
- 关闭会话时写 `ended_at` 和关闭原因。
- 列表、详情和导出使用真实字段。
- 提供迁移脚本兼容历史数据。

收益：审计时间线准确，便于排查责任和回放定位。

### 4. 服务启动历史孤儿会话修复

服务启动时处理旧状态：

- 内存会话为空，但数据库中可能有历史 `recording` 或 `active`。
- 启动后扫描这些记录。
- 对 SSH 记录检查录制文件是否存在，尽量补充文件大小和持续时间。
- 对桌面记录按心跳和录屏文件状态标记为 `timeout` 或 `closed`。

收益：重启服务后不会遗留永久录制中的历史记录。

### 5. Guacamole 真实断开事件接入

第一阶段用前端关闭和后端心跳兜底。第二阶段可进一步提升准确性：

- 方案 A：后端代理 Guacamole tunnel，在服务端感知连接建立和断开。
- 方案 B：接入 Guacamole 历史/事件数据源，如果部署方式支持，则按真实连接事件更新 OpsHub 会话。
- 方案 C：保留现状，只将心跳兜底阈值调优为生产默认方案。

收益：桌面会话结束时间更接近真实 RDP/Guacamole 断开时间。

## 验证清单

第一阶段完成后至少验证：

- Windows 桌面打开后点击关闭按钮，会话变为 `closed`。
- Windows 桌面打开后直接关闭标签页，会话在心跳过期后变为 `timeout` 或通过 beacon 变为 `closed`。
- Windows 桌面打开后刷新页面，不产生永久 `active` 会话。
- 同一台主机一秒内打开多个 SSH 终端，所有会话 ID 不重复。
- 同一秒多个 SSH 录制文件不会覆盖。
- 关闭任意一个 SSH 标签页，只结束对应审计记录。
- SSH 录制文件仍可播放和下载。

第二阶段完成后追加验证：

- WebSocket 断网、代理断开、浏览器崩溃后会话能按超时闭合。
- stdout/stderr 大量输出时没有并发写 panic 或数据竞争。
- SSH 审计开始时间、结束时间、持续时长与实际操作一致。
