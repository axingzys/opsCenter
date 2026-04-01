import request from '@/utils/request'

// 操作日志相关接口
export const getOperationLogList = (params: {
  page?: number
  pageSize?: number
  username?: string
  module?: string
  action?: string
  startTime?: string
  endTime?: string
}) => {
  return request.get('/api/v1/audit/operation-logs', { params })
}

export const getOperationLogDetail = (id: number) => {
  return request.get(`/api/v1/audit/operation-logs/${id}`)
}

export const deleteOperationLog = (id: number) => {
  return request.delete(`/api/v1/audit/operation-logs/${id}`)
}

export const deleteOperationLogsBatch = (ids: number[]) => {
  return request.post('/api/v1/audit/operation-logs/batch-delete', { ids })
}

// 登录日志相关接口
export const getLoginLogList = (params: {
  page?: number
  pageSize?: number
  username?: string
  loginType?: string
  loginStatus?: string
  startTime?: string
  endTime?: string
}) => {
  return request.get('/api/v1/audit/login-logs', { params })
}

export const getLoginLogDetail = (id: number) => {
  return request.get(`/api/v1/audit/login-logs/${id}`)
}

export const deleteLoginLog = (id: number) => {
  return request.delete(`/api/v1/audit/login-logs/${id}`)
}

export const deleteLoginLogsBatch = (ids: number[]) => {
  return request.post('/api/v1/audit/login-logs/batch-delete', { ids })
}

// 数据日志相关接口
export const getDataLogList = (params: {
  page?: number
  pageSize?: number
  username?: string
  tableName?: string
  action?: string
  startTime?: string
  endTime?: string
}) => {
  return request.get('/api/v1/audit/data-logs', { params })
}

export const getDataLogDetail = (id: number) => {
  return request.get(`/api/v1/audit/data-logs/${id}`)
}

export const deleteDataLog = (id: number) => {
  return request.delete(`/api/v1/audit/data-logs/${id}`)
}

export const deleteDataLogsBatch = (ids: number[]) => {
  return request.post('/api/v1/audit/data-logs/batch-delete', { ids })
}
