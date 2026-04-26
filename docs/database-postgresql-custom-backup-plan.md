# PostgreSQL Custom 备份方案

## 背景

当前数据库备份任务已经有 `backup_type` 字段，前端新增 / 编辑备份任务时也展示了“备份类型”。但现有后端只允许 `logical`：

1. `DatabaseBackupTask` 和 `DatabaseBackupRecord` 都已有 `backup_type` 字段。
2. `validateBackupTaskRequest` 目前只放行 `DatabaseBackupTypeLogical`，其他值会报“当前仅支持 logical 逻辑备份”。
3. PostgreSQL 备份执行在 `buildBackupCommandSpec` 中硬编码 `pg_dump --format=plain`，输出统一写成 `.sql.gz`。
4. 恢复演练当前只校验 `logical` 备份，并对 PostgreSQL 使用 `psql` 从 SQL 文本回放。

所以“新增 custom 备份方法”不需要重新设计一套备份任务模型，但需要把 `backup_type` 从单值扩展为可选枚举，并同步补齐备份执行、文件后缀、下载展示和恢复演练链路。

## 目标

1. PostgreSQL 备份任务支持两种逻辑备份格式：
   - `logical`：现有 Plain SQL，继续使用 `pg_dump --format=plain`，输出 `.sql.gz`。
   - `logical_custom`：新增 Custom，使用 `pg_dump --format=custom`，输出 `.dump`。
2. MySQL / MariaDB / Redis 保持现有行为，只允许 `logical`。
3. 新增能力不影响历史任务、历史备份记录和现有定时调度。
4. PostgreSQL custom 备份记录可以下载，也可以发起非生产恢复演练。
5. 前端在 PostgreSQL 实例下提供备份类型选择；非 PostgreSQL 实例仍只展示 / 提交 `logical`。

## 设计选择

### 1. 复用 `backup_type`，新增 `logical_custom`

不新增表字段，继续使用已有 `backup_type varchar(30)`。

推荐枚举：

| 值 | 含义 | 支持数据库 | 备份命令 | 输出 |
| --- | --- | --- | --- | --- |
| `logical` | 逻辑备份 Plain SQL | MySQL / MariaDB / PostgreSQL / Redis | PostgreSQL: `pg_dump --format=plain` | PostgreSQL: `.sql.gz` |
| `logical_custom` | 逻辑备份 Custom archive | PostgreSQL | `pg_dump --format=custom` | `.dump` |

不建议直接用 `custom` 作为值，因为从平台语义看它仍然是逻辑备份，只是 PostgreSQL 的归档格式不同。`logical_custom` 能保留“逻辑备份”的大类语义，也方便后续再扩展物理备份。

### 2. Custom 输出不再二次 gzip

现有 `runBackupCommand` 会把所有命令 stdout 写入 gzip，因此 plain SQL 得到 `.sql.gz`。PostgreSQL custom archive 本身适合由 `pg_restore` 读取，首版建议输出原始 `.dump` 文件：

1. 避免 `.dump.gz` 带来的双层压缩和恢复分支复杂度。
2. 方便 `pg_restore` 直接读取文件。
3. 文件类型从后缀即可判断，记录和下载展示更明确。

需要给 `backupCommandSpec` 增加输出模式，例如：

```go
type backupCommandSpec struct {
    Commands     []string
    Args         []string
    Env          []string
    DatabaseName string
    FileExt      string
    OutputMode   string // gzip / raw
    Runner       func(ctx context.Context, outputPath string) (int64, error)
}
```

Plain SQL 使用 `OutputMode: "gzip"`，custom 使用 `OutputMode: "raw"`。Redis 仍走已有 `Runner`，不受影响。

### 3. 恢复演练按备份类型分流

PostgreSQL plain 备份继续使用 `psql`：

```bash
psql \
  --host <host> \
  --port <port> \
  --username <user> \
  --dbname <target_db> \
  --no-psqlrc \
  --single-transaction \
  --set ON_ERROR_STOP=on
```

PostgreSQL custom 备份改用 `pg_restore`：

```bash
pg_restore \
  --host <host> \
  --port <port> \
  --username <user> \
  --dbname <target_db> \
  --no-owner \
  --no-privileges \
  --single-transaction \
  --exit-on-error \
  <backup.dump>
```

首版不自动加 `--clean --if-exists`，避免恢复演练误清理目标库对象。目标库应由用户选择非生产实例，并按现有规则禁止选择来源实例和生产环境。

## 后端改造点

### 1. 常量和展示文本

文件：`internal/biz/database/model.go`、`internal/biz/database/usecase.go`

1. 新增常量：

```go
DatabaseBackupTypeLogicalCustom = "logical_custom"
```

2. 调整 `BackupTypeText`：

```go
case DatabaseBackupTypeLogical:
    return "逻辑备份"
case DatabaseBackupTypeLogicalCustom:
    return "逻辑备份（Custom）"
```

历史记录中的 `logical` 继续能正常展示。

### 2. 备份类型校验

文件：`internal/biz/database/backup.go`

新增按数据库类型判断的 helper：

```go
func supportsBackupType(dbType, backupType string) bool {
    switch normalizeDBType(dbType) {
    case DBTypePostgreSQL:
        return backupType == DatabaseBackupTypeLogical ||
            backupType == DatabaseBackupTypeLogicalCustom
    case DBTypeMySQL, DBTypeMariaDB, DBTypeRedis:
        return backupType == DatabaseBackupTypeLogical
    default:
        return false
    }
}
```

`validateBackupTaskRequest` 从“只允许 logical”改成：

1. 先判断实例是否支持备份任务。
2. 再用 `supportsBackupType(instance.DBType, normalizeBackupType(req.BackupType))` 校验。
3. PostgreSQL 以外提交 `logical_custom` 返回清晰错误，例如“当前数据库类型不支持 Custom 备份”。

### 3. 备份命令生成

文件：`internal/biz/database/backup_execution.go`、`internal/biz/database/backup_runtime.go`

将调用从：

```go
spec, err := buildBackupCommandSpec(instance, credential, databaseName)
```

改为：

```go
spec, err := buildBackupCommandSpec(instance, credential, databaseName, task.BackupType)
```

PostgreSQL 分支按 `backupType` 生成不同命令：

Plain SQL：

```go
Args: []string{
    "--host", host,
    "--port", port,
    "--username", username,
    "--dbname", databaseName,
    "--format=plain",
    "--encoding=UTF8",
    "--no-owner",
    "--no-privileges",
},
FileExt: ".sql.gz",
OutputMode: "gzip",
```

Custom：

```go
Args: []string{
    "--host", host,
    "--port", port,
    "--username", username,
    "--dbname", databaseName,
    "--format=custom",
    "--encoding=UTF8",
    "--no-owner",
    "--no-privileges",
},
FileExt: ".dump",
OutputMode: "raw",
```

`runBackupCommand` 根据 `OutputMode` 写文件：

1. `gzip`：保持现有 gzip writer 行为。
2. `raw`：`cmd.Stdout = file`，直接写 `.partial`，成功后 rename。
3. `Runner != nil`：保持 Redis 逻辑备份现状。

### 4. 下载和内容类型

文件：`internal/biz/database/backup_runtime.go`

`detectBackupContentType` 增加 `.dump` 分支：

```go
case strings.HasSuffix(fileName, ".dump"):
    return "application/octet-stream"
```

`application/octet-stream` 足够保守，浏览器会按附件下载。

### 5. 恢复演练

文件：`internal/biz/database/restore.go`、`internal/biz/database/restore_runtime.go`

1. `validateRestoreDryRunRecord` 从只允许 `logical` 改为允许：
   - `logical`
   - `logical_custom`

2. 恢复命令构建需要感知备份类型：

```go
spec, err := buildRestoreCommandSpec(target, credential, databaseName, record.BackupType)
```

3. `restoreCommandSpec` 增加输入模式：

```go
type restoreCommandSpec struct {
    Commands     []string
    Args         []string
    Env          []string
    DatabaseName string
    InputMode    string // stdin / file_arg
    Runner       func(ctx context.Context, inputPath string) error
}
```

4. PostgreSQL plain：
   - `Commands: []string{"psql"}`
   - `InputMode: "stdin"`
   - 沿用当前 gzip 自动解压 stdin 逻辑。

5. PostgreSQL custom：
   - `Commands: []string{"pg_restore"}`
   - `InputMode: "file_arg"`
   - `runRestoreCommand` 执行时把 `record.FilePath` 追加到 args，不走 stdin。

6. MySQL / MariaDB / Redis 恢复逻辑保持不变。

## 前端改造点

文件：`web/src/views/asset/DatabaseManagement.vue`

### 1. 备份类型从禁用输入框改成选择框

当前备份任务弹窗中“备份类型”是 disabled input。改成 `el-select`：

```vue
<el-select v-model="backupTaskForm.backupType" style="width: 100%;">
  <el-option
    v-for="item in availableBackupTypeOptions"
    :key="item.value"
    :label="item.label"
    :value="item.value"
  />
</el-select>
```

### 2. 按实例类型动态给选项

```ts
const availableBackupTypeOptions = computed(() => {
  const instance = instances.value.find(item => item.id === backupTaskForm.instanceId)
  if (instance?.dbType === 'postgresql') {
    return [
      { label: '逻辑备份（Plain SQL）', value: 'logical' },
      { label: '逻辑备份（Custom）', value: 'logical_custom' }
    ]
  }
  return [
    { label: '逻辑备份', value: 'logical' }
  ]
})
```

### 3. 切换实例时修正不兼容类型

当用户从 PostgreSQL 切换到 MySQL / Redis 时，如果当前值是 `logical_custom`，自动重置为 `logical`，避免提交后端再报错。

### 4. 展示和提示

1. 任务列表、记录列表继续使用后端返回的 `backupTypeText`。
2. 弹窗说明文案补充 PostgreSQL Custom 会生成 `.dump`，恢复演练使用 `pg_restore`。
3. 下载按钮不需要改，仍使用现有下载接口。

## API 和兼容性

接口不需要新增，仍然使用现有字段：

```json
{
  "instanceId": 12,
  "name": "pg-custom-nightly",
  "backupType": "logical_custom",
  "schedule": "0 2 * * *",
  "storageType": "local",
  "retentionDays": 7,
  "enabled": true
}
```

兼容策略：

1. 历史任务和历史记录中的 `backupType=logical` 不变。
2. 数据库结构不需要迁移，因为 `backup_type` 已是 varchar。
3. 如果有接口文档或 Swagger 生成物，需要把 `backupType` 的说明补充为 `logical / logical_custom`。
4. `storageType` 仍只支持 `local`，custom 不引入对象存储变更。

## 测试计划

### 后端单元测试

1. `supportsBackupType`
   - PostgreSQL 支持 `logical` 和 `logical_custom`。
   - MySQL / MariaDB / Redis 只支持 `logical`。
   - 其他数据库仍不支持备份。

2. `buildBackupCommandSpec`
   - PostgreSQL `logical` 包含 `--format=plain`，后缀 `.sql.gz`，输出模式 `gzip`。
   - PostgreSQL `logical_custom` 包含 `--format=custom`，后缀 `.dump`，输出模式 `raw`。
   - TLS 场景仍设置 `PGSSLMODE=require`。

3. `runBackupCommand`
   - gzip 模式能生成 gzip 文件。
   - raw 模式能生成原始文件，不经过 gzip。
   - 命令失败时删除 `.partial`。

4. `buildRestoreCommandSpec`
   - PostgreSQL plain 使用 `psql` 和 stdin。
   - PostgreSQL custom 使用 `pg_restore` 和 file arg。

5. `validateRestoreDryRunRecord`
   - 成功 local `logical` 和 `logical_custom` 都允许恢复演练。
   - 非成功状态、非 local、文件不存在仍拒绝。

### 前端验证

1. 新增 PostgreSQL 备份任务时能选择 Plain SQL / Custom。
2. 新增 MySQL / MariaDB / Redis 备份任务时只显示逻辑备份。
3. 切换实例后不保留不兼容的 `logical_custom`。
4. 编辑历史 `logical` 任务默认显示 Plain SQL。
5. 任务列表和记录列表展示后端返回的中文类型。

### 集成验证

1. PostgreSQL Plain SQL 手动备份仍生成 `.sql.gz`，下载正常。
2. PostgreSQL Custom 手动备份生成 `.dump`，下载正常。
3. 定时 PostgreSQL Custom 任务能被调度器执行并生成记录。
4. PostgreSQL Custom 备份可以恢复演练到非生产 PostgreSQL 实例。
5. 非 PostgreSQL 实例提交 `logical_custom` 返回明确错误。

## 验收标准

1. 不改请求路径，不新增用户操作入口，只扩展备份类型选项。
2. 现有 Plain SQL 备份和恢复演练不回归。
3. PostgreSQL Custom 备份记录能完整展示任务、类型、状态、文件名、文件大小和耗时。
4. 备份失败时记录为 failed，并保留 pg_dump / pg_restore 的关键错误信息。
5. Custom 备份文件按保留策略正常清理。
6. Docker 镜像内的 `postgresql-client` 能提供 `pg_dump`、`psql`、`pg_restore`；如果运行环境缺少 `pg_restore`，恢复演练应返回“未安装恢复客户端命令: pg_restore”。

## 风险和注意事项

1. PostgreSQL custom archive 不能当 SQL 文本直接查看或用 `psql` 恢复，必须使用 `pg_restore`。
2. `pg_dump` / `pg_restore` 客户端版本最好不低于目标 PostgreSQL 服务端大版本，否则可能出现兼容性错误。这个风险 plain 和 custom 都存在，但 custom 恢复会更明显。
3. 首版不自动清空目标库；如果目标库已有同名对象，`pg_restore` 可能失败。这与当前 plain SQL 恢复演练的保守策略一致。
4. Custom 文件首版不做外层 gzip，文件后缀为 `.dump`。如后续需要压缩级别、目录格式或并行恢复，可以再通过 `storageConfig` 或新增字段扩展。
5. `backup_type` 目前承担了用户可选备份格式的职责。若未来要引入物理备份、WAL 归档、增量备份，建议再拆分为 `backup_type` 和 `backup_format`。

## 推荐实施顺序

1. 后端枚举、校验和展示文本。
2. PostgreSQL custom `pg_dump` 命令和 raw 输出模式。
3. 下载内容类型和备份记录消息回归。
4. PostgreSQL custom 恢复演练 `pg_restore` 分流。
5. 前端备份类型选择框和实例切换联动。
6. 单元测试、前端手工验证、PostgreSQL 实库集成验证。
