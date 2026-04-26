# OpsHub Docker Compose 部署

## 文件说明

- `docker-compose.yml`: 单机交付用编排文件
- `.env.example`: 环境变量模板
- `nginx.conf`: 前端容器使用的 Nginx 配置
- `prometheus/prometheus.yml`: Prometheus 配置
- `deploy.sh`: 一键部署脚本
- `install-offline.sh`: 目标主机离线安装脚本
- `package-offline-bundle.sh`: 构建机构建完整离线部署包
- `package-migration-bundle.sh`: 从当前运行中的 OpsHub 实例导出迁移包
- `install-migration-bundle.sh`: 在目标主机恢复数据库并启动容器
- `export-baseline.sh`: 重新导出数据库基线 SQL
- `sql/bootstrap.sql`: 一键部署导入的数据库基线
- `sql/schema.sql`: 表结构导出
- `sql/seed.sql`: 初始化数据导出

## 首次使用

```bash
cp deploy/compose/.env.example deploy/compose/.env
./deploy/compose/deploy.sh
```

可选参数：

```bash
./deploy/compose/deploy.sh --with-desktop --with-monitoring
./deploy/compose/deploy.sh --force-reinit-db
./deploy/compose/deploy.sh --image-tag v1.0.0
```

## 离线部署

如果目标主机能直接拉取 MySQL、Redis 等依赖，或者你只打算手工传前后端镜像：

```bash
./build-images.sh --save
```

把 `deploy/compose/` 目录和镜像包一起传到目标主机后执行：

```bash
./deploy/compose/install-offline.sh \
  --images ./dist/opshub-images-latest.tar \
  --host 192.168.1.10
```

如果目标主机上的镜像不是默认名字，例如带私有仓库前缀，可以显式指定：

```bash
./deploy/compose/install-offline.sh \
  --images ./dist/opshub-images-v1.0.0.tar \
  --backend-image registry.example.com/opshub/opshub-api:v1.0.0 \
  --frontend-image registry.example.com/opshub/opshub-web:v1.0.0
```

如果目标主机完全离线，建议在构建机直接导出完整 bundle：

```bash
./deploy/compose/package-offline-bundle.sh
./deploy/compose/package-offline-bundle.sh --without-monitoring
```

把生成的 `dist/opshub-offline-bundle-<tag>.tar.gz` 传到目标主机，解压后执行：

```bash
tar -xzf opshub-offline-bundle-latest.tar.gz
cd opshub-offline-bundle-latest
./deploy/compose/install-offline.sh \
  --images ./dist/opshub-images-latest.tar \
  --with-desktop \
  --with-monitoring \
  --host 192.168.1.10 \
  --strict-offline
```

如果你把多个 tar 包一起放在 `dist/` 目录，也可以直接让脚本逐个导入：

```bash
./deploy/compose/install-offline.sh --images ./dist --with-desktop --with-monitoring --host 192.168.1.10
```

脚本会自动：

- 导入镜像
- 生成 `deploy/compose/.env`
- 写入后端、前端镜像名
- 可选写入主机地址、端口、密码
- 调用现有 `deploy.sh` 完成数据库初始化和服务启动

推荐做法：

- 如果目标主机能联网拉取 `mysql`、`redis` 等基础镜像，只传 `deploy/compose/` 和你打好的前后端镜像 tar 即可
- 如果目标主机完全离线，用 `package-offline-bundle.sh` 生成完整包最稳，不要只传镜像 tar
- `package-offline-bundle.sh` 默认会把桌面和监控镜像一起打进去；只有在明确不需要时才传 `--without-desktop` 或 `--without-monitoring`
- 打包进 bundle 不等于部署时自动启用；安装时仍要传 `--with-desktop --with-monitoring`

常用参数：

- `--host`: 把前端和后端访问地址写入 `.env`
- `--frontend-port` / `--backend-port`: 覆盖默认端口
- `--mysql-root-password` / `--redis-password`: 覆盖默认密码
- `--strict-offline`: 要求 MySQL、Redis、Guacamole、Prometheus 等镜像都已在本机存在，不允许部署时联网拉取

## 迁移当前运行实例到新节点

如果你的目标不是“全新初始化”，而是把当前节点正在运行的 OpsHub 连同数据库一起迁到另一台机器，使用下面这套脚本。

源节点执行：

```bash
./deploy/compose/package-migration-bundle.sh
```

脚本会直接读取当前运行中的这些容器并打包：

- `opshub-backend`
- `opshub-frontend`
- `opshub-mysql`
- `opshub-redis`
- 如果存在，还会自动带上 `opshub-guacd`、`opshub-guacamole`、`opshub-prometheus`

打包内容包括：

- 当前实际运行的镜像
- 当前 `.env` 中的部署参数
- MySQL 整库导出
- `docker-compose.yml`、`nginx.conf`、Prometheus 配置
- 目标节点恢复脚本 `install-migration-bundle.sh`

把生成的 `dist/opshub-migration-bundle-<timestamp>.tar.gz` 传到目标节点后执行：

```bash
tar -xzf opshub-migration-bundle-<timestamp>.tar.gz
cd opshub-migration-bundle-<timestamp>
./deploy/compose/install-migration-bundle.sh --host 192.168.1.10
```

可选参数：

- `--scheme https`: 对外地址写成 HTTPS
- `--frontend-port` / `--backend-port`: 覆盖默认端口
- `--mysql-root-password` / `--redis-password`: 在目标节点改成新密码
- `--skip-image-load`: 镜像已经提前 `docker load` 过时可跳过

说明：

- 这套迁移脚本不会走旧的 `bootstrap.sql` 初始化逻辑，而是直接导入当前 MySQL 导出的整库数据
- Redis 不做数据迁移，目标节点会直接启动新 Redis；对 OpsHub 来说这通常没问题
- 如果你还要迁移桌面录屏、Prometheus 历史数据之类的卷数据，需要额外复制对应目录或卷

## 导出数据库基线

```bash
./deploy/compose/export-baseline.sh
```

脚本会启动临时 MySQL、Redis 容器，并通过本机 Go 启动当前版本后端，导入当前仓库的初始化 SQL，再让后端执行运行时迁移，最后导出最新基线到 `deploy/compose/sql/`。

说明：

- 新主机做全新安装时，`deploy.sh` 实际导入的是 `deploy/compose/sql/bootstrap.sql`
- 只有当你最近又改了数据库结构、初始化数据、运行时迁移，才需要重新执行 `export-baseline.sh`
- 不建议把线上业务数据直接导成“初始化 SQL”再用于新环境首装，除非你的目标就是复制整库数据
