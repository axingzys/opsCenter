# SSL-Cert SSL证书管理插件

<p align="center">
  <img src="https://img.shields.io/badge/Plugin-SSL--Cert-FF6B6B?style=flat&logo=letsencrypt" alt="SSL-Cert">
  <img src="https://img.shields.io/badge/Version-1.0.0-blue?style=flat" alt="Version">
  <img src="https://img.shields.io/badge/Status-Stable-green?style=flat" alt="Status">
</p>

---

## 概述

SSL-Cert 证书管理插件提供完整的 SSL/TLS 证书生命周期管理功能，支持通过 ACME 协议自动申请免费证书（如 Let's Encrypt）、手动导入证书、云厂商证书同步，以及自动续期和部署到 Nginx 或 Kubernetes。

---

## 功能特性

### 证书管理

| 功能 | 描述 |
|:-----|:-----|
| ACME 自动申请 | 支持 Let's Encrypt、ZeroSSL、Google、BuyPass 等 CA |
| 手动导入 | 支持导入已有的 PEM 格式证书和私钥 |
| 云厂商同步 | 从阿里云等云厂商同步证书 |
| 证书详情 | 查看证书域名、有效期、签发机构、指纹等信息 |
| 证书下载 | 下载证书文件（PEM 格式） |

### DNS 验证配置

| 功能 | 描述 |
|:-----|:-----|
| 阿里云 DNS | 通过阿里云 DNS API 自动完成域名验证 |
| 自动验证 | ACME DNS-01 挑战自动完成 |
| 配置测试 | 验证 DNS API 配置是否正确 |

### 部署配置

| 功能 | 描述 |
|:-----|:-----|
| Nginx SSH 部署 | 通过 SSH 将证书部署到 Nginx 服务器 |
| K8s Secret 部署 | 将证书部署为 Kubernetes TLS Secret |
| 自动部署 | 续期后自动部署到配置的目标 |
| 部署测试 | 测试部署配置是否可用 |

### 自动续期

| 功能 | 描述 |
|:-----|:-----|
| 定时检查 | 自动检查证书有效期 |
| 提前续期 | 可配置提前续期天数（默认 30 天） |
| 自动部署 | 续期成功后自动部署 |
| 失败通知 | 续期失败时记录错误信息 |

### 任务记录

| 功能 | 描述 |
|:-----|:-----|
| 签发记录 | 证书首次签发的任务记录 |
| 续期记录 | 证书续期的任务记录 |
| 部署记录 | 证书部署的任务记录 |
| 错误追踪 | 记录任务失败的详细错误信息 |

---

## 安装与启用

### 通过管理界面启用

1. 登录 OpsHub 系统
2. 进入「插件管理」-「插件列表」
3. 找到「SSL-Cert」插件
4. 点击「启用」按钮
5. 刷新页面，左侧菜单出现「SSL证书」

### 环境变量配置

可选的环境变量配置：

| 环境变量 | 说明 | 示例 |
|:---------|:-----|:-----|
| `OPSHUB_ACME_EMAIL` | ACME 注册邮箱 | `admin@example.com` |
| `ACME_EMAIL` | ACME 邮箱（备用） | `admin@example.com` |
| `LETSENCRYPT_EMAIL` | Let's Encrypt 邮箱（备用） | `admin@example.com` |
| `OPSHUB_ACME_STAGING` | 使用测试环境 | `true` |

---

## 使用指南

### 证书管理

#### 申请新证书（ACME）

1. 进入「SSL证书」-「证书管理」
2. 点击「申请证书」按钮
3. 填写证书信息：

| 字段 | 说明 |
|:-----|:-----|
| 证书名称 | 便于识别的名称 |
| 主域名 | 主要域名（如 `example.com`） |
| SAN 域名 | 附加域名（如 `www.example.com`） |
| CA 提供商 | Let's Encrypt / ZeroSSL / Google / BuyPass |
| 密钥算法 | RSA 2048/3072/4096 或 ECDSA P-256/P-384 |
| DNS 服务商 | 选择已配置的 DNS 服务商 |
| 自动续期 | 是否开启自动续期 |
| 提前续期天数 | 提前多少天开始续期（默认 30 天） |

4. 点击「提交」
5. 系统自动完成 DNS 验证和证书签发

#### 导入证书

1. 点击「导入证书」按钮
2. 填写证书信息：

| 字段 | 说明 |
|:-----|:-----|
| 证书名称 | 便于识别的名称 |
| 证书内容 | PEM 格式的证书内容 |
| 私钥内容 | PEM 格式的私钥内容 |

3. 点击「提交」
4. 系统自动解析证书信息

#### 手动续期

1. 在证书列表找到需要续期的证书
2. 点击「续期」按钮
3. 确认续期操作
4. 等待续期完成

### DNS 验证配置

#### 添加阿里云 DNS 配置

1. 进入「SSL证书」-「DNS配置」
2. 点击「新增配置」
3. 填写配置信息：

| 字段 | 说明 |
|:-----|:-----|
| 配置名称 | 便于识别的名称 |
| DNS 服务商 | 选择「阿里云」 |
| AccessKey ID | 阿里云 AccessKey ID |
| AccessKey Secret | 阿里云 AccessKey Secret |
| 联系邮箱 | 联系人邮箱（可选） |
| 联系电话 | 联系人电话（可选） |

4. 点击「测试」验证配置
5. 点击「保存」

> **注意**：建议为 DNS API 创建专用的 RAM 子账号，仅授予 DNS 相关权限。

#### 阿里云 RAM 权限配置

推荐的最小权限策略：

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "alidns:AddDomainRecord",
        "alidns:DeleteDomainRecord",
        "alidns:DescribeDomainRecords",
        "alidns:DescribeDomains"
      ],
      "Resource": "*"
    }
  ]
}
```

### 部署配置

#### 添加 Nginx SSH 部署配置

1. 进入「SSL证书」-「部署配置」
2. 点击「新增配置」
3. 填写配置信息：

| 字段 | 说明 |
|:-----|:-----|
| 配置名称 | 便于识别的名称 |
| 关联证书 | 选择要部署的证书 |
| 部署类型 | 选择「Nginx SSH」 |
| 目标主机 | 选择资产中的主机 |
| 证书路径 | 如 `/etc/nginx/ssl/cert.pem` |
| 私钥路径 | 如 `/etc/nginx/ssl/key.pem` |
| 备份旧证书 | 是否备份替换的旧证书 |
| 自动部署 | 续期后是否自动部署 |

4. 点击「测试」验证配置
5. 点击「保存」

#### 添加 K8s Secret 部署配置

1. 点击「新增配置」
2. 填写配置信息：

| 字段 | 说明 |
|:-----|:-----|
| 配置名称 | 便于识别的名称 |
| 关联证书 | 选择要部署的证书 |
| 部署类型 | 选择「K8s Secret」 |
| K8s 集群 | 选择目标集群 |
| 命名空间 | Secret 所在命名空间 |
| Secret 名称 | TLS Secret 名称 |
| 证书 Key | 默认 `tls.crt` |
| 私钥 Key | 默认 `tls.key` |
| 触发滚动更新 | 是否触发关联 Deployment 重启 |
| Deployment 列表 | 需要滚动更新的 Deployment |
| 自动部署 | 续期后是否自动部署 |

3. 点击「测试」验证配置
4. 点击「保存」

#### 手动部署

1. 在部署配置列表找到配置
2. 点击「部署」按钮
3. 确认部署操作
4. 等待部署完成

### 查看任务记录

1. 进入「SSL证书」-「任务记录」
2. 查看任务列表：

| 列 | 说明 |
|:---|:-----|
| 任务类型 | issue（签发）/ renew（续期）/ deploy（部署） |
| 关联证书 | 任务关联的证书 |
| 状态 | pending / running / success / failed |
| 触发方式 | auto（自动）/ manual（手动） |
| 开始时间 | 任务开始执行时间 |
| 完成时间 | 任务完成时间 |
| 错误信息 | 失败时的错误详情 |

---

## 数据库表

SSL-Cert 插件使用以下数据库表：

| 表名 | 说明 |
|:-----|:-----|
| `ssl_certificates` | 证书主表 |
| `ssl_dns_providers` | DNS 服务商配置 |
| `ssl_deploy_configs` | 部署配置 |
| `ssl_renew_tasks` | 续期任务记录 |

---

## 证书状态

| 状态 | 说明 | 颜色 |
|:-----|:-----|:-----|
| `pending` | 待申请 | 灰色 |
| `active` | 正常 | 绿色 |
| `expiring` | 即将过期（30天内） | 橙色 |
| `expired` | 已过期 | 红色 |
| `error` | 错误 | 红色 |

---

## CA 提供商

### Let's Encrypt

| 项目 | 说明 |
|:-----|:-----|
| 免费 | 完全免费 |
| 有效期 | 90 天 |
| 限制 | 每周 50 个证书/域名 |
| 推荐 | 最常用的免费 CA |

### ZeroSSL

| 项目 | 说明 |
|:-----|:-----|
| 免费 | 基础版免费 |
| 有效期 | 90 天 |
| 特点 | 提供 Web 管理界面 |

### Google Trust Services

| 项目 | 说明 |
|:-----|:-----|
| 免费 | 免费 |
| 有效期 | 90 天 |
| 特点 | Google 签发的证书 |

### BuyPass

| 项目 | 说明 |
|:-----|:-----|
| 免费 | 免费 |
| 有效期 | 180 天 |
| 特点 | 有效期较长 |

---

## 密钥算法

| 算法 | 说明 | 推荐场景 |
|:-----|:-----|:---------|
| RSA 2048 | 兼容性最好 | 通用场景 |
| RSA 3072 | 更高安全性 | 安全要求较高 |
| RSA 4096 | 最高安全性 | 高安全要求 |
| ECDSA P-256 | 性能好，密钥短 | 现代浏览器 |
| ECDSA P-384 | 更高安全性 | 高安全要求 |

---

## 最佳实践

### 证书管理

| 建议 | 说明 |
|:-----|:-----|
| 使用自动续期 | 避免证书过期导致服务中断 |
| 提前 30 天续期 | 留出充足的处理时间 |
| 使用 ECDSA | 性能更好，安全性不低于 RSA 2048 |
| 定期检查 | 定期检查证书状态和任务记录 |

### DNS 配置

| 建议 | 说明 |
|:-----|:-----|
| 最小权限 | 仅授予必要的 DNS API 权限 |
| 专用账号 | 创建专用的 API 密钥 |
| 定期轮换 | 定期更换 API 密钥 |
| 测试验证 | 添加配置后进行测试 |

### 部署配置

| 建议 | 说明 |
|:-----|:-----|
| 开启备份 | Nginx 部署时开启旧证书备份 |
| 测试部署 | 正式部署前先进行测试 |
| 自动部署 | 开启续期后自动部署 |
| 监控通知 | 配合监控插件设置告警 |

---

## 常见问题

### Q: ACME 申请证书失败？

**A:** 检查以下几点：
1. DNS 配置是否正确（点击测试验证）
2. 域名是否正确解析
3. 阿里云 AccessKey 是否有 DNS 权限
4. 是否超过 CA 的申请频率限制

### Q: 证书部署失败？

**A:** 可能原因：
1. SSH 连接失败（检查主机凭据）
2. 目标路径没有写权限
3. K8s 集群连接失败
4. 命名空间或权限不足

### Q: 自动续期没有生效？

**A:** 检查以下几点：
1. 证书是否开启了自动续期
2. DNS 配置是否关联正确
3. 查看任务记录中的错误信息
4. 检查调度器是否正常运行

### Q: 如何使用测试环境？

**A:** 设置环境变量 `OPSHUB_ACME_STAGING=true`，使用 Let's Encrypt 的测试环境，避免触发正式环境的频率限制。

### Q: 证书链不完整？

**A:**
1. 申请证书时系统会自动获取证书链
2. 导入证书时请填写完整的证书链
3. 部分浏览器需要完整证书链才能正确验证

---

## 相关文档

- [Let's Encrypt 文档](https://letsencrypt.org/docs/)
- [ACME 协议 RFC 8555](https://tools.ietf.org/html/rfc8555)
- [阿里云 DNS API](https://help.aliyun.com/document_detail/29739.html)
- [Kubernetes TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)
- [OpsHub 主文档](../../README.md)
- [部署指南](../deployment.md)
