# OpsHub 单机 Docker Compose 一键部署方案

## 1. 目标

目标是在一台标准 Linux 主机上，以 `docker compose` 方式完成 OpsHub 的一键部署，满足以下要求：

- 前端、后端先打包为可交付镜像，不依赖目标主机本地编译
- 数据库初始化 SQL 可独立导出、版本化保存，便于后续脚本直接导入
- 通过一个脚本完成环境检查、依赖启动、数据库导入、业务启动和健康检查
- 部署结果可重复执行，适合测试环境和轻量生产环境

说明：

- `docker compose` 场景中没有 Kubernetes `Pod` 概念，下文中的“前后端 pod 打包”统一落为“前后端容器镜像打包”

## 2. 当前仓库现状

仓库内已经具备单机部署的基础材料，可以直接复用：

- 后端镜像构建文件：`Dockerfile`
- 前端镜像构建文件：`Dockerfile.frontend`
- 现有 Compose 编排：`docker-compose.yml`
- Nginx 反向代理配置：`nginx.conf`
- 数据库初始化脚本：`migrations/init.sql`
- 镜像构建脚本：`build-images.sh`

核对后的现状和问题如下：

1. 根目录 `docker-compose.yml` 已可被 `docker compose config` 正常解析，但更偏“开发态/演示态”
   - 直接使用 `build:`，不适合作为目标主机上的标准交付方式
   - 内置了 `prometheus`、`guacamole` 等扩展服务，单机一键部署时应区分必需项和可选项
   - 仍带有过时的 `version` 字段，虽不影响运行，但建议清理

2. 前后端镜像命名目前不统一
   - 根目录 `docker-compose.yml` 使用 `opshub-api:local` / `opshub-web:local`
   - Helm 默认使用 `opshub-api` / `opshub-web`
   - `build-images.sh` 当前输出的是 `opshub-backend` / `opshub-frontend`
   - 需要统一镜像命名，否则后续脚本、文档、CI 会长期不一致

3. 数据库初始化口径不统一
   - `migrations/init.sql` 已包含完整表结构和初始数据
   - 但 `migrations/README.md` 中的表数量统计已明显过期
   - 后端启动时还会执行 `AutoMigrate` 和默认数据初始化
   - 这意味着“SQL 初始化”和“应用启动自动建表”同时存在，容易导致后续版本漂移

4. 运行依赖不止前后端
   - 后端依赖 MySQL 和 Redis
   - 如需桌面能力，还依赖 `guacd` 和 `guacamole`
   - 如需监控插件默认可用，还需要 Prometheus

结论：

- 现有仓库不需要推倒重来
- 需要把“开发态 compose”整理为“交付态 compose”
- 需要把数据库基线导出流程标准化，而不是继续手工维护一份随时间漂移的 `init.sql`

## 3. 目标部署架构

单机部署建议分为“核心服务”和“可选服务”两层。

### 3.1 核心服务

- `mysql`
- `redis`
- `backend`
- `frontend`

这是最小可运行集合，也是“一键部署脚本”的默认启动目标。

### 3.2 可选服务

- `guacd`
- `guacamole`
- `prometheus`

建议通过以下方式二选一：

- 方案 A：默认全部启动，保证现有功能开箱即用
- 方案 B：通过 Compose `profiles` 做成可选能力，默认只起核心服务

推荐方案：`方案 B`

原因：

- 单机部署的首要目标是稳定启动和简化交付
- 桌面和 Prometheus 属于扩展能力，不应阻塞基础登录和管理能力
- 当前代码已经支持通过环境变量开关部分能力，适合在 `.env` 中按需开启

## 4. 总体交付物设计

建议新增一个独立的单机交付目录，不与现有根目录开发文件混用。

建议目录：

```text
deploy/compose/
├── docker-compose.yml
├── .env.example
├── deploy.sh
├── sql/
│   ├── bootstrap.sql
│   ├── schema.sql
│   └── seed.sql
└── README.md
```

说明：

- `deploy/compose/docker-compose.yml`
  - 面向目标主机使用
  - 只引用镜像，不再本地构建
- `deploy/compose/.env.example`
  - 统一管理端口、密码、镜像版本、外部访问地址
- `deploy/compose/deploy.sh`
  - 一键部署入口脚本
- `deploy/compose/sql/schema.sql`
  - 纯表结构导出
- `deploy/compose/sql/seed.sql`
  - 初始账号、菜单、插件状态、系统配置等最小种子数据
- `deploy/compose/sql/bootstrap.sql`
  - `schema.sql + seed.sql` 合并产物，供脚本直接导入

## 5. 镜像打包方案

### 5.1 镜像命名规范

建议统一为：

- 后端：`<registry>/opshub-api:<version>`
- 前端：`<registry>/opshub-web:<version>`

不再继续使用 `opshub-backend` / `opshub-frontend` 这组名称。

原因：

- 已与 Helm 默认命名保持一致
- 与当前业务语义更匹配
- 后续 Compose、脚本、CI、发布文档可以完全复用同一套变量

### 5.2 镜像构建方式

直接复用现有 Dockerfile：

- 后端：`Dockerfile`
- 前端：`Dockerfile.frontend`

建议保留根目录 `build-images.sh`，但调整为以下职责：

1. 统一输出标准镜像名
2. 支持自定义仓库地址和版本号
3. 可选导出离线包

建议命令形态：

```bash
./build-images.sh registry.example.com/opshub v1.0.0
```

脚本内部执行：

```bash
docker build -f Dockerfile -t registry.example.com/opshub/opshub-api:v1.0.0 .
docker build -f Dockerfile.frontend -t registry.example.com/opshub/opshub-web:v1.0.0 .
docker push registry.example.com/opshub/opshub-api:v1.0.0
docker push registry.example.com/opshub/opshub-web:v1.0.0
```

### 5.3 离线交付方案

如果目标主机无法直接访问镜像仓库，建议补充离线包交付：

```bash
docker save -o dist/opshub-images-v1.0.0.tar \
  registry.example.com/opshub/opshub-api:v1.0.0 \
  registry.example.com/opshub/opshub-web:v1.0.0
```

目标主机执行：

```bash
docker load -i opshub-images-v1.0.0.tar
```

是否把 MySQL/Redis/Guacamole/Prometheus 也打进离线包，可在实施阶段按网络环境决定。

## 6. 数据库导出与初始化方案

## 6.1 原则

数据库初始化不要继续依赖“手工维护一份越来越大的 SQL 文件”，而应改为：

- 先用当前版本代码把空库初始化成“标准基线库”
- 再从这个标准基线库导出 SQL
- 导出的 SQL 成为后续一键部署的唯一初始化输入

这样可以保证 SQL 与代码版本同步。

## 6.2 推荐产物

建议同时维护三份文件：

1. `schema.sql`
   - 只包含表结构、索引、约束
2. `seed.sql`
   - 只包含最小可用初始化数据
   - 如：admin 用户、角色、菜单、插件状态、系统默认配置
3. `bootstrap.sql`
   - `schema.sql + seed.sql` 的合并文件
   - 部署脚本默认导入这一份

推荐原因：

- 后续如果只想升级结构，可以单独使用 `schema.sql`
- 后续如果只想重置基础数据，可以单独使用 `seed.sql`
- 一键部署仍然只需要导入 `bootstrap.sql`

## 6.3 基线 SQL 的生成方式

建议以“真实运行后的数据库”为导出源，而不是继续直接编辑 `migrations/init.sql`。

推荐流程：

1. 启动一套临时 MySQL、Redis、后端
2. 让后端执行当前版本的 `AutoMigrate` 和默认数据初始化
3. 确认插件相关表、系统表、默认管理员、菜单等均已生成
4. 使用 `mysqldump` 导出

建议命令：

```bash
mysqldump \
  --single-transaction \
  --set-gtid-purged=OFF \
  --default-character-set=utf8mb4 \
  --no-tablespaces \
  -h 127.0.0.1 -P 23306 -u root -p \
  --no-data opshub > deploy/compose/sql/schema.sql
```

```bash
mysqldump \
  --single-transaction \
  --set-gtid-purged=OFF \
  --default-character-set=utf8mb4 \
  --no-tablespaces \
  -h 127.0.0.1 -P 23306 -u root -p \
  opshub \
  sys_department sys_role sys_menu sys_role_menu sys_user sys_user_role \
  sys_config plugin_states > deploy/compose/sql/seed.sql
```

再生成：

```bash
cat deploy/compose/sql/schema.sql deploy/compose/sql/seed.sql > deploy/compose/sql/bootstrap.sql
```

## 6.4 是否继续保留 `migrations/init.sql`

建议保留，但角色调整为：

- `migrations/init.sql`：开发态或历史兼容文件
- `deploy/compose/sql/bootstrap.sql`：交付态标准基线文件

不建议继续让根目录 `migrations/init.sql` 同时承担：

- 文档示例
- 本地开发初始化
- 生产交付初始化
- 版本基线归档

这四种职责。

## 6.5 与 `AutoMigrate` 的关系

当前后端启动时会执行 `AutoMigrate`，短期可以保留，但部署口径要明确：

- 一键部署时以 `bootstrap.sql` 为主
- `AutoMigrate` 只作为补充兜底，不作为主初始化入口

原因：

- 只依赖 `AutoMigrate`，初始化结果不够显式
- 只依赖 SQL，又可能遗漏某些运行时新增对象
- 先导入基线 SQL，再允许应用做幂等补齐，是当前版本最稳妥的方式

中期建议：

- 后续增加迁移版本控制机制
- 最终把“生产初始化”从 `AutoMigrate` 中解耦

## 7. 一键部署脚本方案

建议新增 `deploy/compose/deploy.sh` 作为唯一入口。

### 7.1 脚本职责

脚本需要完成以下步骤：

1. 环境检查
   - 检查 `docker` 和 `docker compose`
   - 检查 `.env` 是否存在
   - 检查 `bootstrap.sql` 是否存在

2. 参数处理
   - 支持 `--with-desktop`
   - 支持 `--with-monitoring`
   - 支持 `--force-reinit-db`
   - 支持 `--image-tag`

3. 预启动依赖
   - 启动 `mysql`、`redis`
   - 等待 MySQL 和 Redis 健康

4. 数据库初始化
   - 若数据库为空，则导入 `bootstrap.sql`
   - 若数据库非空，则跳过导入
   - 若指定 `--force-reinit-db`，则先清空后重新导入

5. 启动业务服务
   - 启动 `backend`、`frontend`
   - 按参数决定是否启动 `guacd`、`guacamole`、`prometheus`

6. 健康检查
   - 检查 `/health`
   - 输出前端访问地址
   - 输出默认管理员信息提示

### 7.2 推荐实现方式

数据库导入不建议完全依赖 Compose 里的单次 `db-init` 容器，推荐由 `deploy.sh` 统一控制。

推荐原因：

- 更容易做“仅首次导入”的判断
- 更容易支持 `--force-reinit-db`
- 更容易输出清晰日志
- 避免 Compose 生命周期和初始化脚本相互耦合

推荐导入逻辑：

```bash
if mysql 中不存在 sys_user 表; then
  导入 bootstrap.sql
else
  跳过
fi
```

表存在性判断建议使用：

```sql
SELECT COUNT(*) 
FROM information_schema.tables
WHERE table_schema='opshub' AND table_name='sys_user';
```

## 8. Compose 文件改造方案

建议新增 `deploy/compose/docker-compose.yml`，与根目录现有文件并行存在。

### 8.1 改造原则

1. 交付态 Compose 不再包含 `build:`
2. 只引用固定镜像
3. 服务名尽量保持与现有 Nginx 代理配置一致
4. MySQL、Redis 保留持久化卷
5. 可选服务使用 `profiles`

### 8.2 核心配置建议

- `mysql`
  - 官方 MySQL 8.0 镜像
  - 持久化卷 `mysql-data`
- `redis`
  - 官方 Redis 7 镜像
  - 持久化卷 `redis-data`
- `backend`
  - 镜像：`opshub-api`
  - 通过环境变量覆盖 `config/config.yaml`
- `frontend`
  - 镜像：`opshub-web`
  - 继续复用 `nginx.conf` 做反代

### 8.3 可选服务配置建议

- `guacd`
  - profile: `desktop`
- `guacamole`
  - profile: `desktop`
- `prometheus`
  - profile: `monitoring`

对应 `.env` 默认值建议：

- `OPSHUB_DESKTOP_ENABLED=false`
- `OPSHUB_MONITORING_PROMETHEUS_ENABLED=false`

当脚本启用相关 profile 时，再同步打开对应环境变量。

## 9. 配置文件方案

建议交付态配置全部收敛到 `deploy/compose/.env`。

建议包含以下变量：

```dotenv
IMAGE_REGISTRY=registry.example.com/opshub
IMAGE_TAG=v1.0.0

FRONTEND_PORT=80
BACKEND_PORT=9876
MYSQL_PORT=23306
REDIS_PORT=26390

MYSQL_DATABASE=opshub
MYSQL_USERNAME=root
MYSQL_ROOT_PASSWORD=change-me
REDIS_PASSWORD=change-me

OPSHUB_SERVER_MODE=release
OPSHUB_SERVER_JWT_SECRET=change-me
OPSHUB_SERVER_EXTERNAL_URL=http://<host-ip>:9876
OPSHUB_SERVER_FRONTEND_URL=http://<host-ip>

OPSHUB_DESKTOP_ENABLED=false
OPSHUB_MONITORING_PROMETHEUS_ENABLED=false
```

说明：

- 后端当前支持通过 `OPSHUB_*` 环境变量覆盖 `config.yaml`
- 因此不需要在目标主机单独维护一份后端配置文件

## 10. 推荐实施顺序

### 第一阶段：整理交付基础

1. 统一镜像命名
2. 改造 `build-images.sh`
3. 新增 `deploy/compose/` 目录
4. 拆分并生成 `schema.sql`、`seed.sql`、`bootstrap.sql`

### 第二阶段：交付态部署文件

1. 新增交付态 `docker-compose.yml`
2. 新增 `.env.example`
3. 编写 `deploy.sh`
4. 增加 `README.md`

### 第三阶段：验证

1. 在干净 Linux 主机执行一键部署
2. 验证前端可访问
3. 验证默认管理员可登录
4. 验证重跑脚本不会重复初始化数据库
5. 验证 `--force-reinit-db` 可重建环境

## 11. 验收标准

满足以下条件即可认为方案落地成功：

1. 在干净主机执行一条命令可完成部署

```bash
./deploy/compose/deploy.sh
```

2. 脚本自动完成数据库初始化，无需人工进入 MySQL 执行导入
3. 前端可通过浏览器访问
4. 后端 `/health` 返回正常
5. 默认管理员 `admin / 123456` 可登录
6. 第二次执行部署脚本时，不会重复覆盖已有数据库
7. 若指定强制重置参数，可以重新导入基线 SQL

## 12. 风险与注意事项

1. `migrations/init.sql` 与运行时 `AutoMigrate` 并存
   - 这是当前最大的初始化漂移风险
   - 必须以导出的基线 SQL 为准，避免双重维护

2. 桌面和监控功能有额外依赖
   - 若默认关闭，可降低首发复杂度
   - 若默认开启，需要同步校验 Guacamole 和 Prometheus 的可用性

3. 镜像仓库访问策略要提前定
   - 在线拉取还是离线导入，会影响脚本设计

4. 默认账号密码仅适用于初始化
   - 部署文档必须明确要求首次登录后立即修改

5. 数据目录需要持久化
   - 至少包括 MySQL、Redis、Guacamole 录屏目录、Prometheus 数据目录

## 13. 本次方案结论

本项目已经具备单机 `docker compose` 部署的基础条件，当前不需要重构应用架构，重点是把现有材料整理成“交付态”：

- 镜像交付统一成 `opshub-api` / `opshub-web`
- 数据库初始化改为“标准基线导出 + 脚本导入”
- 单独维护 `deploy/compose/` 目录，避免和开发态文件混用
- 用 `deploy.sh` 串起启动、导入、健康检查，形成真正的一键部署入口

按这个方案推进，后续可以分两步完成：

1. 先落部署目录、SQL 导出和脚本
2. 再补离线安装包和可选服务 profile
