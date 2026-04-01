# LDAP 集成指南

本文档介绍如何将 OpsHub 与企业 LDAP/Active Directory 集成，实现统一身份认证。

## 前提条件

1. OpsHub 已部署并正常运行
2. 企业 LDAP/AD 服务器可访问
3. 拥有 LDAP 管理员账号或具有查询权限的服务账号
4. 了解 LDAP 目录结构（Base DN、用户 DN 等）

## LDAP 集成概述

OpsHub 支持通过 LDAP/Active Directory 进行用户认证，主要特性包括：

- **统一认证**：用户使用企业 LDAP 账号登录 OpsHub
- **自动创建用户**：首次登录时自动在 OpsHub 中创建本地用户
- **信息同步**：自动同步用户的邮箱、姓名、电话等信息
- **角色分配**：为 LDAP 用户自动分配默认角色和部门
- **混合认证**：支持 LDAP 用户和本地用户同时存在

## 第一步：获取 LDAP 服务器信息

在配置之前，需要从 IT 管理员处获取以下信息：

### OpenLDAP 示例

| 参数 | 示例值 | 说明 |
|------|--------|------|
| LDAP 服务器地址 | `ldap.example.com` | LDAP 服务器主机名或 IP |
| 端口 | `389` (LDAP) 或 `636` (LDAPS) | 默认端口 |
| Base DN | `dc=example,dc=com` | 搜索根目录 |
| Bind DN | `cn=admin,dc=example,dc=com` | 管理员或服务账号 DN |
| Bind Password | `admin_password` | 管理员密码 |
| 用户过滤器 | `(uid=%s)` | 用户搜索过滤器 |
| 用户名属性 | `uid` | 用户名字段 |
| 邮箱属性 | `mail` | 邮箱字段 |
| 姓名属性 | `cn` | 姓名字段 |
| 电话属性 | `telephoneNumber` | 电话字段 |

### Active Directory 示例

| 参数 | 示例值 | 说明 |
|------|--------|------|
| AD 服务器地址 | `ad.example.com` | AD 域控制器地址 |
| 端口 | `389` (LDAP) 或 `636` (LDAPS) | 默认端口 |
| Base DN | `dc=example,dc=com` | 搜索根目录 |
| Bind DN | `cn=ldap_service,ou=Service Accounts,dc=example,dc=com` | 服务账号 DN |
| Bind Password | `service_password` | 服务账号密码 |
| 用户过滤器 | `(sAMAccountName=%s)` | AD 用户搜索过滤器 |
| 用户名属性 | `sAMAccountName` | AD 用户名字段 |
| 邮箱属性 | `mail` | 邮箱字段 |
| 姓名属性 | `displayName` 或 `cn` | 姓名字段 |
| 电话属性 | `telephoneNumber` | 电话字段 |

## 第二步：测试 LDAP 连接

在配置 OpsHub 之前，建议先测试 LDAP 连接是否正常。

### 使用 ldapsearch 测试（Linux/Mac）

```bash
# 测试连接和认证
ldapsearch -x -H ldap://ldap.example.com:389 \
  -D "cn=admin,dc=example,dc=com" \
  -w "admin_password" \
  -b "dc=example,dc=com" \
  "(uid=testuser)"

# 测试 LDAPS（TLS）
ldapsearch -x -H ldaps://ldap.example.com:636 \
  -D "cn=admin,dc=example,dc=com" \
  -w "admin_password" \
  -b "dc=example,dc=com" \
  "(uid=testuser)"
```

### 使用 PowerShell 测试（Windows AD）

```powershell
# 测试 AD 连接
$ldap = New-Object System.DirectoryServices.DirectoryEntry("LDAP://ad.example.com/dc=example,dc=com", "ldap_service@example.com", "service_password")
$ldap.distinguishedName

# 搜索用户
$searcher = New-Object System.DirectoryServices.DirectorySearcher($ldap)
$searcher.Filter = "(sAMAccountName=testuser)"
$searcher.FindOne()
```

## 第三步：在 OpsHub 中配置 LDAP

### 3.1 登录管理后台

1. 使用管理员账号登录 OpsHub
2. 进入 **系统管理** → **系统配置**
3. 切换到 **LDAP 配置** 标签页

### 3.2 填写 LDAP 配置

#### 基础配置

| 字段 | 说明 | 示例（OpenLDAP） | 示例（AD） |
|------|------|------------------|------------|
| 启用 LDAP | 是否启用 LDAP 认证 | ✓ | ✓ |
| LDAP 服务器 | 服务器地址（不含协议） | `ldap.example.com` | `ad.example.com` |
| 端口 | LDAP 端口 | `389` | `389` |
| 使用 LDAPS | 使用 SSL/TLS 加密连接 | ☐ (端口 636) | ☐ (端口 636) |
| 使用 StartTLS | 在普通连接上启用 TLS | ☐ | ☐ |
| 跳过证书验证 | 开发环境可启用，生产环境不建议 | ☐ | ☐ |

#### 认证配置

| 字段 | 说明 | 示例（OpenLDAP） | 示例（AD） |
|------|------|------------------|------------|
| Bind DN | 管理员或服务账号 DN | `cn=admin,dc=example,dc=com` | `cn=ldap_service,ou=Service Accounts,dc=example,dc=com` |
| Bind 密码 | 管理员密码 | `admin_password` | `service_password` |
| Base DN | 用户搜索根目录 | `dc=example,dc=com` | `ou=Users,dc=example,dc=com` |
| 用户过滤器 | 用户搜索过滤器，`%s` 会被替换为用户名 | `(uid=%s)` | `(sAMAccountName=%s)` |

**用户过滤器高级示例**：

```
# 只允许特定组的用户
(&(uid=%s)(memberOf=cn=opshub_users,ou=Groups,dc=example,dc=com))

# AD 中只允许启用的用户
(&(sAMAccountName=%s)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))

# 只允许特定 OU 下的用户
(&(uid=%s)(ou=Engineering))
```

#### 属性映射

| 字段 | 说明 | OpenLDAP | Active Directory |
|------|------|----------|------------------|
| 用户名属性 | LDAP 中的用户名字段 | `uid` | `sAMAccountName` |
| 邮箱属性 | 邮箱字段 | `mail` | `mail` |
| 姓名属性 | 真实姓名字段 | `cn` | `displayName` 或 `cn` |
| 电话属性 | 电话号码字段 | `telephoneNumber` | `telephoneNumber` 或 `mobile` |

#### 用户管理

| 字段 | 说明 | 建议值 |
|------|------|--------|
| 自动创建用户 | 首次登录时自动创建本地用户 | ✓ 启用 |
| 默认角色 | LDAP 用户的默认角色 | 选择一个普通用户角色 |
| 默认部门 | LDAP 用户的默认部门 | 选择一个部门（可选） |

### 3.3 保存并测试

1. 点击 **保存配置**
2. 点击 **测试连接** 按钮验证配置是否正确
3. 如果测试成功，会显示 "LDAP 连接测试成功"

## 第四步：测试 LDAP 登录

### 4.1 退出当前账号

退出管理员账号，返回登录页面。

### 4.2 使用 LDAP 账号登录

1. 在登录页面输入 LDAP 用户名和密码
2. 点击登录
3. 系统会自动：
   - 连接 LDAP 服务器验证用户名和密码
   - 查询用户信息（邮箱、姓名、电话）
   - 在 OpsHub 中创建本地用户（如果不存在）
   - 分配默认角色和部门
   - 完成登录

### 4.3 验证用户信息

登录成功后：
1. 进入 **个人中心**，查看用户信息是否正确同步
2. 进入 **系统管理** → **用户管理**，查看用户列表
3. LDAP 用户的 **来源** 字段会显示为 "LDAP"

## 第五步：用户管理

### 5.1 LDAP 用户特性

- **密码管理**：LDAP 用户的密码由 LDAP 服务器管理，无法在 OpsHub 中修改
- **信息同步**：每次登录时会自动同步 LDAP 中的用户信息
- **角色管理**：可以在 OpsHub 中为 LDAP 用户分配角色和权限
- **用户来源**：用户列表中会标识用户来源为 "LDAP"

### 5.2 为 LDAP 用户分配角色

1. 进入 **系统管理** → **用户管理**
2. 找到 LDAP 用户，点击 **编辑**
3. 在 **角色分配** 中选择角色
4. 保存

### 5.3 禁用 LDAP 用户

如果需要禁止某个 LDAP 用户访问 OpsHub：

1. 进入 **系统管理** → **用户管理**
2. 找到该用户，点击 **禁用**
3. 该用户将无法登录，即使 LDAP 认证成功

## 配置示例

### 示例 1：OpenLDAP 配置

```json
{
  "enabled": true,
  "host": "ldap.example.com",
  "port": 389,
  "useTls": false,
  "startTls": false,
  "skipVerify": false,
  "bindDn": "cn=admin,dc=example,dc=com",
  "bindPassword": "admin_password",
  "baseDn": "dc=example,dc=com",
  "userFilter": "(uid=%s)",
  "attrUsername": "uid",
  "attrEmail": "mail",
  "attrRealName": "cn",
  "attrPhone": "telephoneNumber",
  "defaultRoleId": 2,
  "defaultDeptId": 0,
  "autoCreateUser": true
}
```

### 示例 2：Active Directory 配置

```json
{
  "enabled": true,
  "host": "ad.example.com",
  "port": 389,
  "useTls": false,
  "startTls": false,
  "skipVerify": false,
  "bindDn": "cn=ldap_service,ou=Service Accounts,dc=example,dc=com",
  "bindPassword": "service_password",
  "baseDn": "ou=Users,dc=example,dc=com",
  "userFilter": "(sAMAccountName=%s)",
  "attrUsername": "sAMAccountName",
  "attrEmail": "mail",
  "attrRealName": "displayName",
  "attrPhone": "telephoneNumber",
  "defaultRoleId": 2,
  "defaultDeptId": 1,
  "autoCreateUser": true
}
```

### 示例 3：使用 LDAPS（SSL/TLS）

```json
{
  "enabled": true,
  "host": "ldaps.example.com",
  "port": 636,
  "useTls": true,
  "startTls": false,
  "skipVerify": false,
  "bindDn": "cn=admin,dc=example,dc=com",
  "bindPassword": "admin_password",
  "baseDn": "dc=example,dc=com",
  "userFilter": "(uid=%s)",
  "attrUsername": "uid",
  "attrEmail": "mail",
  "attrRealName": "cn",
  "attrPhone": "telephoneNumber",
  "defaultRoleId": 2,
  "defaultDeptId": 0,
  "autoCreateUser": true
}
```

## 故障排查

### 问题 1：连接超时

**现象**：测试连接时提示 "LDAP服务器连接失败: dial tcp timeout"

**原因**：
- LDAP 服务器地址或端口错误
- 网络不通或防火墙阻止
- LDAP 服务未启动

**解决方法**：
1. 检查 LDAP 服务器地址和端口是否正确
2. 使用 `telnet` 或 `nc` 测试网络连通性：
   ```bash
   telnet ldap.example.com 389
   # 或
   nc -zv ldap.example.com 389
   ```
3. 检查防火墙规则，确保允许访问 LDAP 端口
4. 确认 LDAP 服务正在运行

### 问题 2：认证失败

**现象**：测试连接时提示 "LDAP管理员绑定失败: Invalid Credentials"

**原因**：
- Bind DN 或密码错误
- Bind DN 格式不正确
- 服务账号权限不足

**解决方法**：
1. 验证 Bind DN 和密码是否正确
2. 确认 Bind DN 格式（OpenLDAP 使用 `cn=admin,dc=example,dc=com`，AD 可能使用 `user@domain.com`）
3. 确保服务账号有查询用户的权限
4. 使用 `ldapsearch` 命令行工具测试认证

### 问题 3：找不到用户

**现象**：登录时提示 "LDAP中未找到用户"

**原因**：
- Base DN 配置错误
- 用户过滤器不正确
- 用户不在搜索范围内

**解决方法**：
1. 检查 Base DN 是否包含目标用户
2. 验证用户过滤器格式：
   - OpenLDAP: `(uid=%s)`
   - AD: `(sAMAccountName=%s)`
3. 使用 `ldapsearch` 测试搜索：
   ```bash
   ldapsearch -x -H ldap://ldap.example.com:389 \
     -D "cn=admin,dc=example,dc=com" \
     -w "password" \
     -b "dc=example,dc=com" \
     "(uid=testuser)"
   ```

### 问题 4：用户信息不完整

**现象**：用户登录成功，但邮箱、姓名等信息为空

**原因**：
- 属性映射配置错误
- LDAP 中用户信息不完整
- 属性名称不匹配

**解决方法**：
1. 使用 `ldapsearch` 查看用户的实际属性：
   ```bash
   ldapsearch -x -H ldap://ldap.example.com:389 \
     -D "cn=admin,dc=example,dc=com" \
     -w "password" \
     -b "dc=example,dc=com" \
     "(uid=testuser)" \
     "*"
   ```
2. 根据实际属性名称调整配置：
   - 邮箱：`mail` 或 `email`
   - 姓名：`cn`、`displayName`、`name`
   - 电话：`telephoneNumber`、`mobile`、`phone`

### 问题 5：TLS/SSL 证书错误

**现象**：使用 LDAPS 时提示 "x509: certificate signed by unknown authority"

**原因**：
- LDAP 服务器使用自签名证书
- 系统未信任 CA 证书

**解决方法**：

**临时方案**（仅开发环境）：
- 启用 "跳过证书验证" 选项

**生产环境方案**：
1. 获取 LDAP 服务器的 CA 证书
2. 将 CA 证书添加到系统信任列表：
   ```bash
   # Linux
   sudo cp ca.crt /usr/local/share/ca-certificates/
   sudo update-ca-certificates

   # macOS
   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt
   ```
3. 重启 OpsHub 服务

### 问题 6：用户登录成功但没有权限

**现象**：LDAP 用户登录成功，但无法访问任何功能

**原因**：
- 未配置默认角色
- 默认角色没有权限

**解决方法**：
1. 检查 LDAP 配置中的 "默认角色" 是否设置
2. 确认默认角色有适当的权限
3. 手动为用户分配角色：
   - 进入 **系统管理** → **用户管理**
   - 编辑 LDAP 用户，分配角色

### 问题 7：用户无法登录（已在 LDAP 中删除）

**现象**：LDAP 中已删除的用户仍然可以在 OpsHub 中看到

**原因**：
- OpsHub 不会自动删除 LDAP 用户的本地记录
- 用户状态未同步

**解决方法**：
1. 手动禁用或删除该用户：
   - 进入 **系统管理** → **用户管理**
   - 找到该用户，点击 **禁用** 或 **删除**
2. 该用户将无法再登录（LDAP 认证会失败）

## 高级配置

### 多 Base DN 搜索

如果用户分布在多个 OU 中，可以使用更高层的 Base DN：

```json
{
  "baseDn": "dc=example,dc=com",
  "userFilter": "(uid=%s)"
}
```

或使用更复杂的过滤器：

```json
{
  "baseDn": "dc=example,dc=com",
  "userFilter": "(|(ou=Engineering)(ou=Sales))(uid=%s)"
}
```

### 组成员过滤

只允许特定组的成员登录：

```json
{
  "userFilter": "(&(uid=%s)(memberOf=cn=opshub_users,ou=Groups,dc=example,dc=com))"
}
```

### AD 用户账户控制

只允许启用的 AD 用户登录：

```json
{
  "userFilter": "(&(sAMAccountName=%s)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
}
```

### 使用 StartTLS

在普通 LDAP 连接上启用 TLS 加密：

```json
{
  "port": 389,
  "useTls": false,
  "startTls": true,
  "skipVerify": false
}
```



## 相关日志

### 查看 LDAP 认证日志

OpsHub 会记录 LDAP 认证的详细日志，包括：
- LDAP 连接状态
- 用户搜索结果
- 认证成功/失败
- 用户创建/更新

日志位置：
- Docker 部署：`docker-compose logs -f opshub`
- Kubernetes 部署：`kubectl logs -f deployment/opshub`
- 本地运行：控制台输出

日志示例：

```
INFO  LDAP认证成功  username=zhangsan email=zhangsan@example.com realName=张三
INFO  自动创建LDAP用户成功  username=zhangsan userId=10
INFO  为LDAP用户分配默认角色成功  username=zhangsan roleId=2
```

## 常见 LDAP 目录结构

### OpenLDAP 典型结构

```
dc=example,dc=com
├── ou=People
│   ├── uid=zhangsan,ou=People,dc=example,dc=com
│   ├── uid=lisi,ou=People,dc=example,dc=com
│   └── ...
├── ou=Groups
│   ├── cn=admins,ou=Groups,dc=example,dc=com
│   └── cn=users,ou=Groups,dc=example,dc=com
└── cn=admin,dc=example,dc=com (管理员)
```

配置：
- Base DN: `ou=People,dc=example,dc=com`
- Bind DN: `cn=admin,dc=example,dc=com`
- User Filter: `(uid=%s)`

### Active Directory 典型结构

```
dc=example,dc=com
├── ou=Users
│   ├── cn=Zhang San,ou=Users,dc=example,dc=com
│   ├── cn=Li Si,ou=Users,dc=example,dc=com
│   └── ...
├── ou=Service Accounts
│   └── cn=ldap_service,ou=Service Accounts,dc=example,dc=com
└── ou=Groups
    ├── cn=Domain Admins,ou=Groups,dc=example,dc=com
    └── cn=Domain Users,ou=Groups,dc=example,dc=com
```

配置：
- Base DN: `ou=Users,dc=example,dc=com`
- Bind DN: `cn=ldap_service,ou=Service Accounts,dc=example,dc=com`
- User Filter: `(sAMAccountName=%s)`

## 参考资料

- [OpenLDAP 官方文档](https://www.openldap.org/doc/)
- [Active Directory LDAP 语法](https://docs.microsoft.com/en-us/windows/win32/adsi/search-filter-syntax)
- [LDAP 过滤器语法](https://ldap.com/ldap-filters/)
- [RFC 4511 - LDAP 协议](https://tools.ietf.org/html/rfc4511)
