import request from '@/utils/request'

export interface LoginParams {
  username: string
  password: string
  captchaId?: string
  captchaCode?: string
}

export interface RegisterParams {
  username: string
  password: string
  realName?: string
  email: string
  phone?: string
}

export interface LoginResponse {
  token: string
  user: any
}

// 登录
export const login = (params: LoginParams) => {
  return request.post<any, LoginResponse>('/api/v1/public/login', params)
}

// 注册
export const register = (params: RegisterParams) => {
  return request.post('/api/v1/public/register', params)
}

// 获取当前用户信息
export const getProfile = () => {
  return request.get('/api/v1/profile')
}
