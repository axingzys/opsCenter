import request from '@/utils/request'

// 获取所有配置
export const getAllConfig = () => {
  return request.get('/api/v1/system/config')
}

// 获取基础配置
export const getBasicConfig = () => {
  return request.get('/api/v1/system/config/basic')
}

// 保存基础配置
export const saveBasicConfig = (data: {
  systemName: string
  systemLogo: string
  systemDescription: string
}) => {
  return request.put('/api/v1/system/config/basic', data)
}

// 获取安全配置
export const getSecurityConfig = () => {
  return request.get('/api/v1/system/config/security')
}

// 保存安全配置
export const saveSecurityConfig = (data: {
  passwordMinLength: number
  sessionTimeout: number
  enableCaptcha: boolean
  maxLoginAttempts: number
  lockoutDuration: number
  // MFA配置
  mfaEnabled: boolean
  mfaEnforced: boolean
  mfaType: string
  mfaSkipDuration: number
}) => {
  return request.put('/api/v1/system/config/security', data)
}

// 上传Logo
export const uploadLogo = (file: File) => {
  const formData = new FormData()
  formData.append('file', file)
  return request.post('/api/v1/system/config/logo', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}

// 获取公开配置（无需认证）
export const getPublicConfig = () => {
  return request.get('/api/v1/public/config')
}

// ============ LDAP 配置 ============

// LDAP 配置类型
export interface LDAPConfig {
  enabled: boolean
  host: string
  port: number
  useTls: boolean
  startTls: boolean
  skipVerify: boolean
  bindDn: string
  bindPassword: string
  baseDn: string
  userFilter: string
  attrUsername: string
  attrEmail: string
  attrRealName: string
  attrPhone: string
  defaultRoleId: number
  defaultDeptId: number
  autoCreateUser: boolean
}

// 获取LDAP配置
export const getLDAPConfig = () => {
  return request.get('/api/v1/system/config/ldap')
}

// 保存LDAP配置
export const saveLDAPConfig = (data: LDAPConfig) => {
  return request.put('/api/v1/system/config/ldap', data)
}

// 测试LDAP连接
export const testLDAPConnection = (data: LDAPConfig) => {
  return request.post('/api/v1/system/config/ldap/test', data)
}
