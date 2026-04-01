# Jenkins SSO 集成指南

本文档介绍如何将 Jenkins 与 OpsHub 统一认证系统集成，实现单点登录。

## 前提条件

1. OpsHub 已部署并正常运行
2. Jenkins 已安装并可访问
3. OpsHub 和 Jenkins 网络互通

## 部署模式

### 生产环境（Docker Compose）

OpsHub 通过 docker-compose 部署，nginx 统一代理：
- 前端：http://服务器IP:80
- API：http://服务器IP:80/api/
- OAuth2：http://服务器IP:80/oauth2/

### 本地开发

前端通过 Vite 开发服务器运行，后端直接运行：
- 前端：http://localhost:5173
- 后端：http://localhost:9876
- OAuth2：通过 Vite 代理 http://localhost:5173/oauth2/

**重要**：本地开发时，Jenkins 的 OIDC 配置必须使用 `http://localhost:5173` 作为 OpsHub 地址（而非 9876），
因为 session cookie 存储在前端域名下，OAuth2 授权流程需要携带 cookie。

## 重要说明

**OAuth2/OIDC 端点必须通过 nginx 代理（端口 80）访问**，不能直接访问后端端口 9876。

- 正确：`http://10.122.24.67/oauth2/.well-known/openid-configuration`
- 错误：`http://10.122.24.67:9876/oauth2/.well-known/openid-configuration`

## 第一步：配置 OpsHub 外部访问 URL

OAuth2/OIDC 需要正确的 issuer URL，必须配置 OpsHub 的外部访问地址。

### Docker Compose 部署

在 `docker-compose.yml` 所在目录创建 `.env` 文件：

```bash
# 替换为你的服务器 IP（使用前端端口 80，而非后端端口 9876）
OPSHUB_SERVER_EXTERNAL_URL=http://10.122.24.67
```

重启服务：

```bash
docker-compose down
docker-compose up -d
```

### Kubernetes/Helm 部署

在 `values.yaml` 中设置：

```yaml
server:
  externalURL: "http://opshub.example.com"
```

或通过 `--set` 参数：

```bash
helm upgrade opshub ./charts/opshub --set server.externalURL=http://opshub.example.com
```

### 验证配置

访问 OIDC 发现端点，确认 `issuer` 正确：

```bash
# 通过 nginx 代理访问（端口 80）
curl http://10.122.24.67/oauth2/.well-known/openid-configuration
```

应返回类似：

```json
{
  "issuer": "http://10.122.24.67",
  "authorization_endpoint": "http://10.122.24.67/oauth2/authorize",
  "token_endpoint": "http://10.122.24.67/oauth2/token",
  ...
}
```

## 第二步：在 OpsHub 中创建 SSO 应用

1. 登录 OpsHub 管理后台
2. 进入 **身份认证** → **SSO应用**
3. 点击 **添加应用**，填写以下信息：

| 字段 | 值 |
|------|-----|
| 应用名称 | Jenkins |
| 应用编码 | jenkins |
| 分类 | CI/CD |
| SSO类型 | OAuth2 |
| 应用URL | `http://<JENKINS_IP>:8080/` |
| 图标URL | `https://www.jenkins.io/images/logos/jenkins/jenkins.svg`（可选） |
| 描述 | Jenkins CI/CD 平台 |
| 启用 | 开启 |

4. **SSO配置**（JSON格式）：

```json
{
  "client_secret": "your-secret-key-here",
  "redirect_uri": "http://<JENKINS_IP>:8080/securityRealm/finishLogin"
}
```

**示例**（本地开发）：
```json
{
  "client_secret": "jenkins-secret-2024",
  "redirect_uri": "http://localhost:8080/securityRealm/finishLogin"
}
```

5. 保存后，**应用编码**（如 `jenkins`）就是 **Client ID**，SSO配置中的 `client_secret` 就是 **Client Secret**

## 第三步：安装 Jenkins OIDC 插件

1. 进入 Jenkins 管理界面
2. 点击 **Manage Jenkins** → **Plugins** → **Available plugins**
3. 搜索 `OpenId Connect Authentication`
4. 安装插件并重启 Jenkins

## 第四步：配置 Jenkins OIDC

1. 进入 **Manage Jenkins** → **Security**
2. 在 **Security Realm** 中选择 **Login with Openid Connect**
3. 填写以下配置：

### 基础配置

| 字段 | 值 |
|------|-----|
| Client ID | OpsHub 中生成的 Client ID |
| Client Secret | OpsHub 中生成的 Client Secret |

### 自动配置（推荐）

勾选 **Automatic configuration**，填写：

| 字段 | 值 |
|------|-----|
| Well-known configuration endpoint | `http://<OPSHUB_URL>/oauth2/.well-known/openid-configuration` |

**重要**：这里的 `<OPSHUB_URL>` 必须与 OpsHub 配置的 `EXTERNAL_URL` 一致：
- 生产环境（docker-compose）：`http://10.122.24.67/oauth2/.well-known/openid-configuration`
- 本地开发：`http://localhost:5173/oauth2/.well-known/openid-configuration`

### 用户名映射

| 字段 | 值 |
|------|-----|
| User name field name | `preferred_username` 或 `sub` |
| Full name field name | `name` |
| Email field name | `email` |

### 高级配置（可选）

| 字段 | 值 |
|------|-----|
| Scopes | `openid profile email` |
| Logout from OpenID Provider | 勾选（可实现单点登出） |

4. 点击 **Save** 保存配置

## 第五步：配置授权策略

1. 在 Jenkins **Security** 页面
2. **Authorization** 选择合适的策略：
   - **Logged-in users can do anything**：登录用户拥有所有权限
   - **Matrix-based security**：基于矩阵的细粒度权限控制

3. 确保至少有一个管理员用户有完整权限

## 第六步：测试登录

1. 打开 Jenkins 登录页面
2. 应该看到 **Login with Openid Connect** 按钮
3. 点击后跳转到 OpsHub 登录页面
4. 使用 OpsHub 账号登录
5. 授权后自动跳转回 Jenkins，完成登录

## 通过应用门户访问

配置完成后，用户可以通过 OpsHub 应用门户一键访问 Jenkins：

1. 登录 OpsHub
2. 进入 **身份认证** → **应用门户**
3. 找到 Jenkins 应用，点击即可自动登录跳转

## 故障排查

### 问题1：跳转到 localhost:8080

**原因**：未配置 `OPSHUB_SERVER_EXTERNAL_URL`

**解决**：按照第一步配置外部访问 URL 并重启服务

### 问题2：403 Forbidden

**原因**：访问了错误的端口或地址

**解决**：
- 确认 OpsHub API 端口（默认 9876）
- 确认 Jenkins 和 OpsHub 网络互通
- 检查防火墙规则

### 问题3：Invalid redirect_uri

**原因**：OpsHub 中配置的回调地址与 Jenkins 实际地址不匹配

**解决**：
- 检查 OpsHub SSO 应用中的回调地址
- 确保格式正确：`http://<JENKINS_IP>:8080/securityRealm/finishLogin`

### 问题4：用户登录后没有权限

**原因**：Jenkins 授权策略未正确配置

**解决**：
1. 使用本地管理员账户登录 Jenkins
2. 检查 **Authorization** 配置
3. 为 OIDC 用户分配适当权限

### 问题5：无法获取用户信息

**原因**：Scopes 配置不正确

**解决**：确保 Scopes 包含 `openid profile email`

## 相关端点

**生产环境**（通过 nginx 代理，端口 80）：

| 端点 | URL |
|------|-----|
| OIDC 发现 | `http://<OPSHUB_IP>/oauth2/.well-known/openid-configuration` |
| 授权端点 | `http://<OPSHUB_IP>/oauth2/authorize` |
| Token 端点 | `http://<OPSHUB_IP>/oauth2/token` |
| 用户信息 | `http://<OPSHUB_IP>/oauth2/userinfo` |
| JWKS | `http://<OPSHUB_IP>/oauth2/jwks` |

**本地开发**（通过 Vite 代理，端口 5173）：

| 端点 | URL |
|------|-----|
| OIDC 发现 | `http://localhost:5173/oauth2/.well-known/openid-configuration` |
| 授权端点 | `http://localhost:5173/oauth2/authorize` |
| Token 端点 | `http://localhost:5173/oauth2/token` |
| 用户信息 | `http://localhost:5173/oauth2/userinfo` |
| JWKS | `http://localhost:5173/oauth2/jwks` |

## 安全建议

1. **生产环境使用 HTTPS**：配置 SSL 证书，使用 HTTPS 访问
2. **定期轮换 Client Secret**：定期更新 SSO 应用的 Client Secret
3. **最小权限原则**：只给用户必要的 Jenkins 权限
4. **审计日志**：在 OpsHub 认证日志中查看登录记录
