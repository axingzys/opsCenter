# GitLab SSO 集成指南

本文档介绍如何将 GitLab 与 OpsHub 统一认证系统集成，实现单点登录。

## 前提条件

1. OpsHub 已部署并正常运行
2. GitLab 已安装并可访问（自建 GitLab 实例）
3. OpsHub 和 GitLab 网络互通

## 第一步：配置 OpsHub 外部访问 URL

确保 OpsHub 的 `external_url` 配置正确（与 Jenkins 集成相同）。

在 `config/config.yaml` 中设置：

```yaml
server:
  external_url: "http://10.122.28.13:5173"  # OpsHub 外部访问地址
```

重启 OpsHub 服务。

## 第二步：在 OpsHub 中创建 GitLab SSO 应用

1. 登录 OpsHub 管理后台
2. 进入 **身份认证** → **应用管理**
3. 点击 **新增应用** 或选择 GitLab 模板，填写以下信息：

| 字段 | 值 |
|------|-----|
| 应用名称 | GitLab |
| 应用编码 | `gitlab` (必须是这个值) |
| 分类 | 代码管理 |
| SSO类型 | OAuth2 或 OIDC |
| 应用URL | `http://<GITLAB_IP>/` |
| 图标URL | `https://about.gitlab.com/images/press/logo/svg/gitlab-icon-rgb.svg` |
| 描述 | 代码托管和 DevOps 平台 |
| 启用 | 开启 |

4. **SSO配置**（JSON格式）：

```json
{
  "client_id": "gitlab",
  "client_secret": "your-gitlab-secret-here",
  "redirect_uri": "http://<GITLAB_IP>/users/auth/openid_connect/callback"
}
```

**示例**：
```json
{
  "client_id": "gitlab",
  "client_secret": "gitlab-secret-2024",
  "redirect_uri": "http://10.122.28.14/users/auth/openid_connect/callback"
}
```

5. 保存应用

## 第三步：配置 GitLab OIDC

### 3.1 编辑 GitLab 配置文件

SSH 登录到 GitLab 服务器，编辑配置文件：

```bash
sudo vim /etc/gitlab/gitlab.rb
```

### 3.2 添加 OIDC 配置

在配置文件中添加以下内容：

```ruby
# 启用 OmniAuth
gitlab_rails['omniauth_enabled'] = true

# 允许单点登录
gitlab_rails['omniauth_allow_single_sign_on'] = ['openid_connect']

# 不阻止自动创建用户（允许新用户通过 SSO 注册）
gitlab_rails['omniauth_block_auto_created_users'] = false

# 自动关联已有用户（通过邮箱匹配）
gitlab_rails['omniauth_auto_link_user'] = ['openid_connect']

# 从提供商同步用户信息
gitlab_rails['omniauth_sync_profile_from_provider'] = ['openid_connect']
gitlab_rails['omniauth_sync_profile_attributes'] = ['email', 'name']

# 配置 OIDC 提供商
gitlab_rails['omniauth_providers'] = [
  {
    name: 'openid_connect',
    label: 'OpsHub SSO',
    args: {
      name: 'openid_connect',
      scope: ['openid', 'profile', 'email'],
      response_type: 'code',
      issuer: 'http://<OPSHUB_URL>',
      discovery: true,
      client_auth_method: 'query',
      uid_field: 'preferred_username',
      send_scope_to_token_endpoint: 'false',
      client_options: {
        identifier: 'gitlab',
        secret: 'your-gitlab-secret-here',
        redirect_uri: 'http://<GITLAB_IP>/users/auth/openid_connect/callback'
      }
    }
  }
]
```

**重要参数说明**：
- `issuer`: OpsHub 的外部访问地址（不带 `/oauth2` 后缀）
- `identifier`: 必须与 OpsHub 中应用的 `code` 一致
- `secret`: 必须与 OpsHub 中 SSO 配置的 `client_secret` 一致
- `redirect_uri`: GitLab 的回调地址
- `uid_field`: 使用 `preferred_username` 作为用户标识

**示例配置**：
```ruby
gitlab_rails['omniauth_providers'] = [
  {
    name: 'openid_connect',
    label: 'OpsHub SSO',
    args: {
      name: 'openid_connect',
      scope: ['openid', 'profile', 'email'],
      response_type: 'code',
      issuer: 'http://10.122.28.13:5173',
      discovery: true,
      client_auth_method: 'query',
      uid_field: 'preferred_username',
      send_scope_to_token_endpoint: 'false',
      client_options: {
        identifier: 'gitlab',
        secret: 'gitlab-secret-2024',
        redirect_uri: 'http://10.122.28.14/users/auth/openid_connect/callback'
      }
    }
  }
]
```

### 3.3 重新配置 GitLab

```bash
sudo gitlab-ctl reconfigure
```

## 第四步：验证 OIDC 配置

在 GitLab 服务器上验证 OpsHub 的 OIDC 发现端点：

```bash
curl http://<OPSHUB_URL>/oauth2/.well-known/openid-configuration
```

应该返回类似：

```json
{
  "issuer": "http://10.122.28.13:5173",
  "authorization_endpoint": "http://10.122.28.13:5173/oauth2/authorize",
  "token_endpoint": "http://10.122.28.13:5173/oauth2/token",
  "userinfo_endpoint": "http://10.122.28.13:5173/oauth2/userinfo",
  "jwks_uri": "http://10.122.28.13:5173/oauth2/jwks",
  ...
}
```

## 第五步：测试登录

### 方式一：通过 GitLab 登录页

1. 访问 GitLab 登录页面
2. 应该能看到 **OpsHub SSO** 登录按钮
3. 点击按钮，跳转到 OpsHub 登录页面
4. 使用 OpsHub 账号登录
5. 授权后自动跳转回 GitLab，完成登录

### 方式二：通过 OpsHub 应用门户

1. 登录 OpsHub
2. 进入 **身份认证** → **应用门户**
3. 找到 GitLab 应用，点击即可自动登录跳转

## 可选配置

### 禁用 GitLab 内置登录（仅允许 SSO）

```ruby
# 禁用标准登录
gitlab_rails['omniauth_allow_single_sign_on'] = ['openid_connect']

# 强制所有用户使用 SSO
gitlab_rails['omniauth_block_auto_created_users'] = false

# 禁用密码登录
gitlab_rails['password_authentication_enabled'] = false
```

### 配置管理员用户

首次配置时，建议保留一个管理员账号。通过 Rails Console 设置：

```bash
sudo gitlab-rails console
```

```ruby
# 查找通过 SSO 登录的用户
user = User.find_by(username: 'your_username')

# 设置为管理员
user.admin = true
user.save!
```

### 启用调试日志

如果遇到问题，可以启用详细日志：

```ruby
gitlab_rails['omniauth_log'] = true
gitlab_rails['log_level'] = 'debug'
```

然后查看日志：

```bash
sudo gitlab-ctl tail gitlab-rails
```

## 故障排查

### 问题1：看不到 SSO 登录按钮

**原因**：OmniAuth 未正确配置

**解决**：
1. 检查 `gitlab.rb` 中 `omniauth_enabled` 是否为 `true`
2. 运行 `sudo gitlab-ctl reconfigure`
3. 重启 GitLab：`sudo gitlab-ctl restart`

### 问题2：点击 SSO 按钮后报错

**原因**：OIDC 配置不正确

**解决**：
1. 验证 OpsHub OIDC 发现端点可访问
2. 检查 `issuer` 地址是否正确
3. 检查 `client_id` 和 `client_secret` 是否匹配

### 问题3：State mismatch 错误

**原因**：回调地址不匹配

**解决**：
1. 确保 GitLab 配置中的 `redirect_uri` 与 OpsHub 中配置的一致
2. 注意地址末尾不要有多余的 `/`

### 问题4：用户信息同步失败

**原因**：Scope 配置不正确

**解决**：
确保 scope 包含 `openid`、`profile`、`email`：
```ruby
scope: ['openid', 'profile', 'email']
```

### 问题5：无法通过应用门户自动登录

**原因**：应用编码不是 `gitlab`

**解决**：
确保 OpsHub 中 GitLab 应用的**应用编码**字段值是 `gitlab`（小写）

## Docker 部署的 GitLab 配置

如果 GitLab 是通过 Docker 部署的，配置方式略有不同：

### docker-compose.yml

```yaml
version: '3'
services:
  gitlab:
    image: gitlab/gitlab-ce:latest
    environment:
      GITLAB_OMNIBUS_CONFIG: |
        external_url 'http://gitlab.example.com'
        gitlab_rails['omniauth_enabled'] = true
        gitlab_rails['omniauth_allow_single_sign_on'] = ['openid_connect']
        gitlab_rails['omniauth_block_auto_created_users'] = false
        gitlab_rails['omniauth_providers'] = [
          {
            name: 'openid_connect',
            label: 'OpsHub SSO',
            args: {
              name: 'openid_connect',
              scope: ['openid', 'profile', 'email'],
              response_type: 'code',
              issuer: 'http://opshub.example.com',
              discovery: true,
              client_auth_method: 'query',
              uid_field: 'preferred_username',
              client_options: {
                identifier: 'gitlab',
                secret: 'your-secret',
                redirect_uri: 'http://gitlab.example.com/users/auth/openid_connect/callback'
              }
            }
          }
        ]
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - gitlab_config:/etc/gitlab
      - gitlab_logs:/var/log/gitlab
      - gitlab_data:/var/opt/gitlab

volumes:
  gitlab_config:
  gitlab_logs:
  gitlab_data:
```

## 相关端点

| 端点 | URL |
|------|-----|
| OIDC 发现 | `http://<OPSHUB_URL>/oauth2/.well-known/openid-configuration` |
| 授权端点 | `http://<OPSHUB_URL>/oauth2/authorize` |
| Token 端点 | `http://<OPSHUB_URL>/oauth2/token` |
| 用户信息 | `http://<OPSHUB_URL>/oauth2/userinfo` |
| JWKS | `http://<OPSHUB_URL>/oauth2/jwks` |
| GitLab 回调 | `http://<GITLAB_URL>/users/auth/openid_connect/callback` |
| GitLab SSO 入口 | `http://<GITLAB_URL>/users/auth/openid_connect` |

## 安全建议

1. **生产环境使用 HTTPS**：配置 SSL 证书
2. **定期轮换 Client Secret**
3. **限制允许登录的域名**（如果只需要特定用户）
4. **审计日志**：在 OpsHub 认证日志中查看登录记录
