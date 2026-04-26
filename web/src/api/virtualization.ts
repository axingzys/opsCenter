import request from '@/utils/request'

export interface VirtualizationPlatformPayload {
  id?: number
  name: string
  provider: 'esxi' | 'pve'
  endpoint: string
  port: number
  username: string
  password?: string
  insecureSkipVerify: boolean
  status: 'enabled' | 'disabled'
  description?: string
}

export interface VirtualizationGuestQuery {
  page?: number
  pageSize?: number
  keyword?: string
  platformId?: number
  clusterId?: number
  powerState?: string
  bound?: 'all' | 'bound' | 'unbound'
}

export interface VirtualizationGuestPowerPayload {
  action: 'power_on' | 'power_off' | 'reboot'
  reason: string
  confirmText: string
}

export interface VirtualizationGuestConsoleLink {
  auditLogId: number
  guestId: number
  guestName: string
  provider: 'esxi' | 'pve'
  providerText?: string
  mode?: string
  url: string
  requiresLogin: boolean
  message?: string
  requestedAt?: string
  expiresAt?: string
}

export interface VirtualizationGuestSnapshot {
  name: string
  description?: string
  createdAt?: string
  parentName?: string
  current: boolean
  includeMemory: boolean
  quiesced: boolean
  powerState?: string
  children: number
}

export interface VirtualizationGuestSnapshotCreatePayload {
  name: string
  description?: string
  includeMemory?: boolean
  quiesce?: boolean
  reason: string
}

export interface VirtualizationGuestSnapshotActionPayload {
  snapshotName: string
  reason: string
  confirmText?: string
}

export interface VirtualizationActionLogQuery {
  page?: number
  pageSize?: number
  platformId?: number
  guestId?: number
  action?: string
  status?: string
  keyword?: string
}

export interface VirtualizationGuestPrecheck {
  guestId: number
  guestName: string
  primaryIp?: string
  conflictPolicy?: 'strict' | 'warn'
  riskLevel: 'none' | 'low' | 'medium' | 'high'
  currentBinding?: {
    id: number
    name: string
    ip?: string
  }
  selectedHost?: {
    id: number
    name: string
    ip?: string
  }
  suggestedHost?: {
    id: number
    name: string
    ip?: string
    matchType?: string
  }
  candidateHosts?: Array<{
    id: number
    name: string
    ip?: string
    matchType?: string
  }>
  checks: Array<{
    code: string
    level: 'low' | 'medium' | 'high'
    message: string
  }>
}

export interface VirtualizationPlatformTrendPoint {
  timestamp: number
  time: string
  guestTotal: number
  poweredOnGuests: number
  poweredOffGuests: number
  suspendedGuests: number
  boundGuests: number
  onlineGuests: number
  offlineGuests: number
  notConfiguredGuests: number
  unknownGuests: number
}

export interface VirtualizationPlatformTrend {
  scopeType?: 'platform' | 'cluster'
  scopeId?: number
  scopeName?: string
  platformId: number
  clusterId?: number
  range: '24h' | '7d' | '15d'
  start: string
  end: string
  points: VirtualizationPlatformTrendPoint[]
}

export const listVirtualizationPlatforms = (params: {
  page?: number
  pageSize?: number
  keyword?: string
}) => request.get('/api/v1/virtualization/platforms', { params })

export const getVirtualizationPlatform = (id: number) =>
  request.get(`/api/v1/virtualization/platforms/${id}`)

export const createVirtualizationPlatform = (data: VirtualizationPlatformPayload) =>
  request.post('/api/v1/virtualization/platforms', data)

export const updateVirtualizationPlatform = (id: number, data: VirtualizationPlatformPayload) =>
  request.put(`/api/v1/virtualization/platforms/${id}`, data)

export const deleteVirtualizationPlatform = (id: number) =>
  request.delete(`/api/v1/virtualization/platforms/${id}`)

export const testVirtualizationPlatform = (id: number) =>
  request.post(`/api/v1/virtualization/platforms/${id}/test`)

export const syncVirtualizationPlatform = (id: number) =>
  request.post(`/api/v1/virtualization/platforms/${id}/sync`)

export const listVirtualizationSyncJobs = (id: number, params: { page?: number; pageSize?: number }) =>
  request.get(`/api/v1/virtualization/platforms/${id}/sync-jobs`, { params })

export const getVirtualizationPlatformTrend = (id: number, params?: { range?: '24h' | '7d' | '15d' }) =>
  request.get(`/api/v1/virtualization/platforms/${id}/trend`, { params })

export const getVirtualizationClusterTrend = (id: number, params?: { range?: '24h' | '7d' | '15d' }) =>
  request.get(`/api/v1/virtualization/clusters/${id}/trend`, { params })

export const getVirtualizationTopology = (platformId?: number) =>
  request.get('/api/v1/virtualization/topology', { params: platformId ? { platformId } : {} })

export const listVirtualizationGuests = (params: VirtualizationGuestQuery) =>
  request.get('/api/v1/virtualization/guests', { params })

export const getVirtualizationGuest = (id: number) =>
  request.get(`/api/v1/virtualization/guests/${id}`)

export const precheckVirtualizationGuestOnboard = (id: number, params?: { assetHostId?: number }) =>
  request.get(`/api/v1/virtualization/guests/${id}/precheck`, { params })

export interface VirtualizationSettings {
  conflictPolicy: 'strict' | 'warn'
  conflictPolicyText?: string
  writeOperationsEnabled: boolean
  writeOperationsEnabledText?: string
}

export const getVirtualizationSettings = () =>
  request.get('/api/v1/virtualization/settings')

export const updateVirtualizationSettings = (data: VirtualizationSettings) =>
  request.put('/api/v1/virtualization/settings', data)

export const onboardVirtualizationGuest = (id: number, data: {
  assetHostId: number
  bindingType?: 'manual' | 'auto'
  bindingNote?: string
}) => request.post(`/api/v1/virtualization/guests/${id}/onboard`, data)

export const unbindVirtualizationGuest = (id: number, data?: { bindingNote?: string }) =>
  request.post(`/api/v1/virtualization/guests/${id}/unbind`, data || {})

export const powerVirtualizationGuest = (id: number, data: VirtualizationGuestPowerPayload) =>
  request.post(`/api/v1/virtualization/guests/${id}/power`, data)

export const createVirtualizationGuestConsoleLink = (id: number) =>
  request.post(`/api/v1/virtualization/guests/${id}/console-link`)

export const listVirtualizationGuestSnapshots = (id: number) =>
  request.get(`/api/v1/virtualization/guests/${id}/snapshots`)

export const createVirtualizationGuestSnapshot = (id: number, data: VirtualizationGuestSnapshotCreatePayload) =>
  request.post(`/api/v1/virtualization/guests/${id}/snapshots`, data)

export const rollbackVirtualizationGuestSnapshot = (id: number, data: VirtualizationGuestSnapshotActionPayload) =>
  request.post(`/api/v1/virtualization/guests/${id}/snapshots/rollback`, data)

export const deleteVirtualizationGuestSnapshot = (id: number, data: VirtualizationGuestSnapshotActionPayload) =>
  request.post(`/api/v1/virtualization/guests/${id}/snapshots/delete`, data)

export const listVirtualizationActionLogs = (params: VirtualizationActionLogQuery) =>
  request.get('/api/v1/virtualization/action-logs', { params })
