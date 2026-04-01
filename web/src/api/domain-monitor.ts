import request from '@/utils/request'

export interface DomainMonitor {
  id?: number
  domain: string
  status: string
  responseTime: number
  sslValid: boolean
  sslExpiry: string
  checkInterval: number
  enableSSL: boolean
  enableAlert: boolean
  responseThreshold?: number
  sslExpiryDays?: number
  lastCheck: string
  nextCheck?: string
  createdAt?: string
  updatedAt?: string
}

export interface DomainCheckHistory {
  id: number
  domainId: number
  domain: string
  status: string
  responseTime: number
  sslValid: boolean
  sslExpiry?: string
  statusCode: number
  errorMessage?: string
  checkedAt: string
  createdAt: string
}

// 获取域名监控列表
export const getDomainMonitors = () => {
  return request.get('/api/v1/plugins/monitor/domains')
}

// 获取域名监控详情
export const getDomainMonitor = (id: number) => {
  return request.get(`/api/v1/plugins/monitor/domains/${id}`)
}

// 创建域名监控
export const createDomainMonitor = (data: DomainMonitor) => {
  return request.post('/api/v1/plugins/monitor/domains', data)
}

// 更新域名监控
export const updateDomainMonitor = (id: number, data: DomainMonitor) => {
  return request.put(`/api/v1/plugins/monitor/domains/${id}`, data)
}

// 删除域名监控
export const deleteDomainMonitor = (id: number) => {
  return request.delete(`/api/v1/plugins/monitor/domains/${id}`)
}

// 立即检查域名
export const checkDomain = (id: number) => {
  return request.post(`/api/v1/plugins/monitor/domains/${id}/check`)
}

// 获取域名统计数据
export const getDomainStats = () => {
  return request.get('/api/v1/plugins/monitor/domains/stats')
}

// 获取域名检查历史
export const getDomainCheckHistory = (id: number, page: number = 1, pageSize: number = 20) => {
  return request.get(`/api/v1/plugins/monitor/domains/${id}/history`, {
    params: { page, pageSize }
  })
}
