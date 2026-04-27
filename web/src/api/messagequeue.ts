import request from '@/utils/request'

export const MQ_PERMISSION = {
  VIEW: 1,
  DIAGNOSE: 2,
  MESSAGE_READ: 4,
  MESSAGE_EXPORT: 8,
  MESSAGE_WRITE: 16,
  RESOURCE_MANAGE: 32,
  HIGH_RISK: 64,
  AUDIT: 128,
  MANAGE: 256,
  ALL: 511
} as const

export interface MQSupportedType {
  type: string
  name: string
  defaultPort: number
  defaultManagementPort?: number
  testEnabled: boolean
  metadataEnabled: boolean
  diagnosisEnabled: boolean
  messageSampleEnabled: boolean
  resourceManageEnabled: boolean
  phase: string
}

export interface MQInstancePayload {
  name: string
  mqType: string
  endpoint: string
  managementUrl?: string
  port?: number
  credentialId?: number
  tlsEnabled?: boolean
  connectionParams?: string
  status?: 'enabled' | 'disabled'
  environment?: string
  businessSystem?: string
  owner?: string
  tags?: string
  remark?: string
}

export interface MQInstancePermissionPayload {
  roleId: number
  instanceId: number
  permissions: number
}

export interface MQMessageSamplePayload {
  resourceType?: string
  namespace?: string
  resourceName: string
  groupName?: string
  partitionId?: number
  offset?: number
  key?: string
  limit?: number
  maxBytes?: number
}

export interface MQResourceOperationPayload {
  action: string
  resourceType?: string
  namespace?: string
  resourceName?: string
  reason?: string
  confirmText?: string
  confirmed?: boolean
  idempotencyKey?: string
  params?: Record<string, any>
}

export const getMQSupportedTypes = () =>
  request.get('/api/v1/message-queues/supported-types')

export const getMQUIPermissions = () =>
  request.get('/api/v1/message-queues/ui-permissions')

export const listMQInstances = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  mqType?: string
  status?: string
  healthStatus?: string
  environment?: string
}) => request.get('/api/v1/message-queues/instances', { params })

export const createMQInstance = (data: MQInstancePayload) =>
  request.post('/api/v1/message-queues/instances', data)

export const getMQInstance = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}`)

export const updateMQInstance = (id: number, data: MQInstancePayload) =>
  request.put(`/api/v1/message-queues/instances/${id}`, data)

export const deleteMQInstance = (id: number) =>
  request.delete(`/api/v1/message-queues/instances/${id}`)

export const enableMQInstance = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/enable`)

export const disableMQInstance = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/disable`)

export const testMQInstance = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/test`)

export const syncMQMetadata = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/sync-metadata`)

export const getMQOverview = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/overview`)

export const getMQCapabilities = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/capabilities`)

export const collectMQMetricSnapshot = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/metric-snapshots`)

export const listMQMetricSnapshots = (id: number, params?: {
  page?: number
  pageSize?: number
  resourceType?: string
  resourceName?: string
  startTime?: string
  endTime?: string
}) => request.get(`/api/v1/message-queues/instances/${id}/metric-snapshots`, { params })

export const listMQJobs = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  mqType?: string
  jobType?: string
  status?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/message-queues/jobs', { params })

export const getMQJob = (id: number) =>
  request.get(`/api/v1/message-queues/jobs/${id}`)

export const getMQProductionDashboard = () =>
  request.get('/api/v1/message-queues/dashboard')

export const getMQGovernanceReport = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  severity?: string
  category?: string
  keyword?: string
}) => request.get('/api/v1/message-queues/governance-report', { params })

export const getMQDLQAnalysis = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  keyword?: string
  kind?: string
  hasBacklog?: string
}) => request.get('/api/v1/message-queues/dlq-analysis', { params })

export const upsertMQDLQRecord = (data: {
  instanceId: number
  resourceId?: number
  resourceType: string
  namespace?: string
  resourceName: string
  kind?: string
  handlingStatus: string
  owner?: string
  remark?: string
  lastBacklog?: number
  lastMessage?: string
}) => request.post('/api/v1/message-queues/dlq-records', data)

export const getMQTopology = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/topology`)

export const generateMQInspectionReport = (id: number) =>
  request.post(`/api/v1/message-queues/instances/${id}/inspection-reports`)

export const listMQBrokers = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/brokers`)

export const listMQResources = (id: number, params?: {
  page?: number
  pageSize?: number
  keyword?: string
  resourceType?: string
  namespace?: string
  hasBacklog?: string
}) => request.get(`/api/v1/message-queues/instances/${id}/resources`, { params })

export const listMQBindings = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/bindings`)

export const listMQConsumerGroups = (id: number, params?: {
  page?: number
  pageSize?: number
  keyword?: string
  resourceName?: string
  namespace?: string
  hasLag?: string
}) => request.get(`/api/v1/message-queues/instances/${id}/consumer-groups`, { params })

export const listMQPartitions = (id: number, params?: {
  page?: number
  pageSize?: number
  resourceName?: string
}) => request.get(`/api/v1/message-queues/instances/${id}/partitions`, { params })

export const sampleMQMessages = (id: number, data: MQMessageSamplePayload) =>
  request.post(`/api/v1/message-queues/instances/${id}/messages/sample`, data)

export const prepareMQMessageReplay = (id: number, data: {
  resourceType?: string
  namespace?: string
  resourceName: string
  targetInstanceId?: number
  targetResourceName?: string
  maxMessages?: number
  rateLimitPerSecond?: number
  reason: string
}) => request.post(`/api/v1/message-queues/instances/${id}/messages/replay-requests`, data)

export const validateMQResourceOperation = (id: number, data: MQResourceOperationPayload) =>
  request.post(`/api/v1/message-queues/instances/${id}/operations/validate`, data)

export const listMQOperationActions = (id: number) =>
  request.get(`/api/v1/message-queues/instances/${id}/operations/actions`)

export const executeMQResourceOperation = (id: number, data: MQResourceOperationPayload) =>
  request.post(`/api/v1/message-queues/instances/${id}/operations`, data)

export const listMQOperationAudits = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  mqType?: string
  action?: string
  status?: string
  riskLevel?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/message-queues/operation-audits', { params })

export const listMQMessageAudits = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  mqType?: string
  action?: string
  status?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/message-queues/message-audits', { params })

export const listMQInstancePermissions = (params: {
  page?: number
  pageSize?: number
  roleId?: number
  instanceId?: number
  keyword?: string
}) => request.get('/api/v1/message-queues/instance-permissions', { params })

export const upsertMQInstancePermission = (data: MQInstancePermissionPayload) =>
  request.post('/api/v1/message-queues/instance-permissions', data)

export const deleteMQInstancePermission = (id: number) =>
  request.delete(`/api/v1/message-queues/instance-permissions/${id}`)
