import request from '@/utils/request'

// MFA状态响应
export interface MFAStatusResponse {
  isEnabled: boolean
  mfaType: string
  hasBackup: boolean
  verifiedAt?: string
}

// MFA设置响应
export interface MFASetupResponse {
  secret: string
  qrCodeUrl: string
  manualCode: string
}

// MFA登录请求
export interface MFALoginRequest {
  mfaToken: string
  code: string
  rememberDevice?: boolean
}

// MFA登录响应
export interface MFALoginResponse {
  token: string
  user: any
}

// 获取MFA状态
export const getMFAStatus = () => {
  return request.get<MFAStatusResponse>('/api/v1/mfa/status')
}

// 设置MFA（获取二维码和密钥）
export const setupMFA = () => {
  return request.post<MFASetupResponse>('/api/v1/mfa/setup')
}

// 启用MFA
export const enableMFA = (code: string) => {
  return request.post('/api/v1/mfa/enable', { code })
}

// 禁用MFA
export const disableMFA = (code: string) => {
  return request.post('/api/v1/mfa/disable', { code })
}

// 重新生成备用码
export const regenerateBackupCodes = (code: string) => {
  return request.post<{ backupCodes: string[] }>('/api/v1/mfa/regenerate-backup', { code })
}

// MFA登录
export const mfaLogin = (data: MFALoginRequest) => {
  return request.post<MFALoginResponse>('/api/v1/public/auth/mfa/login', data)
}
