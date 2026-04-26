import request from '@/utils/request'

export const getMonitorHosts = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  health?: string
  osType?: 'linux' | 'windows'
}) => {
  return request.get('/api/v1/plugins/monitor/hosts', { params })
}

export const getMonitorHostOverview = (id: number) => {
  return request.get(`/api/v1/plugins/monitor/hosts/${id}/overview`)
}

export const getMonitorHostHistory = (id: number, params?: { range?: '1h' | '24h' | '7d' | '15d' }) => {
  return request.get(`/api/v1/plugins/monitor/hosts/${id}/history`, { params })
}

export const getMonitorHostProcesses = (id: number) => {
  return request.get(`/api/v1/plugins/monitor/hosts/${id}/processes`)
}

export const getMonitorHostPorts = (id: number) => {
  return request.get(`/api/v1/plugins/monitor/hosts/${id}/ports`)
}

export const listAssignableMonitorHosts = () => {
  return request.get<AssignableMonitorHost[]>('/api/v1/plugins/monitor/hosts/assignable')
}

export interface AssignableMonitorHost {
  id: number
  name: string
  ip: string
  primaryPrivateIp?: string
  primaryPublicIp?: string
  status?: number
  port?: number
  sshUser?: string
  osType?: string
  managementMode?: string
  collectStatus?: string
  desktopEnabled?: boolean
  desktopPort?: number
  os?: string
  ID?: number
}

export interface HostAlertRule {
  id?: number
  name: string
  hostId?: number | null
  channelIds?: number[]
  metric: 'cpu_usage' | 'memory_usage' | 'disk_usage' | 'agent_offline'
  threshold: number
  alertInterval: number
  severity: 'warning' | 'critical'
  enabled: boolean
  description?: string
  createdAt?: string
  updatedAt?: string
}

export const getHostAlertRules = () => {
  return request.get('/api/v1/plugins/monitor/host-alert-rules')
}

export const getHostAlertRule = (id: number) => {
  return request.get(`/api/v1/plugins/monitor/host-alert-rules/${id}`)
}

export const createHostAlertRule = (data: HostAlertRule) => {
  return request.post('/api/v1/plugins/monitor/host-alert-rules', data)
}

export const updateHostAlertRule = (id: number, data: HostAlertRule) => {
  return request.put(`/api/v1/plugins/monitor/host-alert-rules/${id}`, data)
}

export const deleteHostAlertRule = (id: number) => {
  return request.delete(`/api/v1/plugins/monitor/host-alert-rules/${id}`)
}

export const getHostAlertRuleStats = () => {
  return request.get('/api/v1/plugins/monitor/host-alert-rules/stats')
}
