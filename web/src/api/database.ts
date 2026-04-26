import request from '@/utils/request'

export interface DatabaseSupportedType {
  type: string
  name: string
  defaultPort: number
  metadataEnabled: boolean
  queryEnabled: boolean
  testEnabled: boolean
  topologyEnabled?: boolean
  phase: string
}

export interface DatabaseInstancePayload {
  name: string
  dbType: string
  host: string
  port?: number
  defaultDatabase?: string
  credentialId: number
  tlsEnabled?: boolean
  connectionParams?: string
  status?: 'enabled' | 'disabled'
  environment?: string
  businessSystem?: string
  owner?: string
  tags?: string
  remark?: string
}

export interface DatabaseQueryPayload {
  schemaName?: string
  sqlText: string
  limit?: number
  timeoutSeconds?: number
}

export interface DatabaseQueryFormatPayload {
  sqlText: string
}

export interface DatabaseWriteValidatePayload {
  schemaName?: string
  sqlText: string
}

export interface DatabaseWriteValidateResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  schemaName: string
  sqlType: string
  riskLevel: string
  riskLevelText: string
  allowed: boolean
  confirmRequired: boolean
  reasonRequired: boolean
  rowsAffectedLimit: number
  message: string
}

export interface DatabaseWriteExecutePayload {
  schemaName?: string
  sqlText: string
  reason?: string
  confirmed?: boolean
  timeoutSeconds?: number
}

export interface DatabaseWriteExecuteResult {
  auditId: number
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  schemaName: string
  sqlType: string
  riskLevel: string
  riskLevelText: string
  executedSql: string
  rowsAffected: number
  rowsAffectedLimit: number
  reason: string
  confirmRequired: boolean
  confirmed: boolean
  rollbackSql: string
  durationMs: number
  message: string
  executedAt: string
}

export interface DatabaseBackupTaskPayload {
  instanceId: number
  name: string
  backupType?: string
  schedule?: string
  storageType?: string
  storageConfig?: string
  retentionDays?: number
  enabled: boolean
}

export interface DatabaseBackupTaskResult {
  id: number
  instanceId: number
  instanceName: string
  instanceDbType: string
  instanceDbTypeText: string
  name: string
  backupType: string
  backupTypeText: string
  schedule: string
  storageType: string
  storageTypeText: string
  storageConfig?: string
  retentionDays: number
  enabled: boolean
  lastRunAt: string
  lastStatus: string
  lastStatusText: string
  lastMessage: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupRecordResult {
  id: number
  taskId: number
  taskName: string
  instanceId: number
  instanceName: string
  triggerType: string
  triggerTypeText: string
  backupType: string
  backupTypeText: string
  storageType: string
  storageTypeText: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  startedAt: string
  finishedAt: string
  durationMs: number
  message: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseBackupRunResult {
  taskId: number
  taskName: string
  recordId: number
  instanceId: number
  instanceName: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  durationMs: number
  message: string
  triggeredAt: string
}

export interface DatabaseRestoreDryRunPayload {
  targetInstanceId: number
  restoreMode?: string
}

export interface DatabaseRestoreJobResult {
  id: number
  backupRecordId: number
  sourceInstanceId: number
  sourceInstanceName: string
  targetInstanceId: number
  targetInstanceName: string
  targetEnvironment: string
  restoreMode: string
  restoreModeText: string
  status: string
  statusText: string
  fileName: string
  fileSize: number
  operatorId: number
  operatorName: string
  startedAt: string
  finishedAt: string
  durationMs: number
  message: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseCapacityPoint {
  collectedAt: string
  schemaCount: number
  tableCount: number
  rowCount: number
  dataSizeBytes: number
  indexSizeBytes: number
  totalSizeBytes: number
  totalSizeText: string
}

export interface DatabaseCapacityObject {
  objectType: string
  schemaName: string
  tableName: string
  tableCount: number
  rowCount: number
  dataSizeBytes: number
  indexSizeBytes: number
  totalSizeBytes: number
  totalSizeText: string
  collectedAt: string
}

export interface DatabaseCapacityTrendResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  range: string
  rangeText: string
  collectedAt: string
  latestSizeBytes: number
  latestSizeText: string
  growthBytes: number
  growthText: string
  growthPercent: number
  points: DatabaseCapacityPoint[]
  topSchemas: DatabaseCapacityObject[]
  topTables: DatabaseCapacityObject[]
  message: string
}

export interface DatabaseCapacityCollectResult {
  instanceId: number
  instanceName: string
  snapshotsCount: number
  schemaCount: number
  tableCount: number
  totalSizeBytes: number
  totalSizeText: string
  collectedAt: string
  message: string
}

export interface DatabaseInspectionMetric {
  key: string
  label: string
  value: string
  status: string
}

export interface DatabaseInspectionSection {
  key: string
  label: string
  status: string
  summary: string
  metrics: DatabaseInspectionMetric[]
}

export interface DatabaseInspectionFinding {
  severity: string
  category: string
  title: string
  message: string
  resourceType: string
  resourceId: string
}

export interface DatabaseInspectionReportResult {
  id: number
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  reportType: string
  status: string
  statusText: string
  healthScore: number
  riskLevel: string
  riskLevelText: string
  summary: string
  capacitySummary: DatabaseInspectionSection
  performanceSummary: DatabaseInspectionSection
  securitySummary: DatabaseInspectionSection
  backupSummary: DatabaseInspectionSection
  findings: DatabaseInspectionFinding[]
  operatorId: number
  operatorName: string
  generatedAt: string
  durationMs: number
  errorMessage: string
  createdAt: string
  updatedAt: string
}

export interface DatabaseDiagnosisListParams {
  limit?: number
}

export interface DatabaseTopologyCard {
  key: string
  label: string
  value: string
  description: string
}

export interface DatabaseTopologyNode {
  id: string
  name: string
  role: string
  roleText: string
  address: string
  state: string
  version: string
  slots: string
  lagBytes: number
  lagText: string
  message: string
  metrics?: Record<string, string>
  updatedAt: string
}

export interface DatabaseTopologyLink {
  source: string
  target: string
  label: string
  state: string
}

export interface DatabaseShard {
  index: string
  shard: string
  primary: boolean
  state: string
  node: string
  address: string
  docs: number
  storeBytes: number
}

export interface DatabaseTopologyResult {
  instanceId: number
  instanceName: string
  dbType: string
  dbTypeText: string
  topologyType: string
  topologyTypeText: string
  collectedAt: string
  cards: DatabaseTopologyCard[]
  nodes: DatabaseTopologyNode[]
  links: DatabaseTopologyLink[]
  shards: DatabaseShard[]
  message: string
}

export interface DatabaseTableDDLPayload {
  schemaName?: string
  tableName: string
}

export const getDatabaseSupportedTypes = () =>
  request.get('/api/v1/databases/supported-types')

export const listDatabaseInstances = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  dbType?: string
  status?: string
  environment?: string
}) => request.get('/api/v1/databases/instances', { params })

export const listDatabaseQueryAudits = (params: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  action?: string
  status?: string
  riskLevel?: string
  sqlType?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/databases/query-audits', { params })

export const listDatabaseQueryHistory = (params?: {
  instanceId?: number
  schemaName?: string
  keyword?: string
  limit?: number
}) => request.get('/api/v1/databases/query-history', { params })

export const exportDatabaseQueryAudits = (params: {
  keyword?: string
  instanceId?: number
  action?: string
  status?: string
  riskLevel?: string
  sqlType?: string
  startTime?: string
  endTime?: string
}) => request.get('/api/v1/databases/query-audits/export', {
  params,
  responseType: 'blob'
})

export const listDatabaseBackupTasks = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  instanceId?: number
  enabled?: string
}) => request.get('/api/v1/databases/backup-tasks', { params })

export const createDatabaseBackupTask = (data: DatabaseBackupTaskPayload) =>
  request.post('/api/v1/databases/backup-tasks', data)

export const updateDatabaseBackupTask = (id: number, data: DatabaseBackupTaskPayload) =>
  request.put(`/api/v1/databases/backup-tasks/${id}`, data)

export const deleteDatabaseBackupTask = (id: number) =>
  request.delete(`/api/v1/databases/backup-tasks/${id}`)

export const runDatabaseBackupTask = (id: number) =>
  request.post(`/api/v1/databases/backup-tasks/${id}/run`)

export const listDatabaseBackupRecords = (params?: {
  page?: number
  pageSize?: number
  taskId?: number
  instanceId?: number
  status?: string
  triggerType?: string
  dateFrom?: string
  dateTo?: string
}) => request.get('/api/v1/databases/backup-records', { params })

const buildNativeDownloadUrl = (path: string) => {
  const token = localStorage.getItem('token')
  if (!token) {
    return path
  }
  const params = new URLSearchParams({ token })
  return `${path}?${params.toString()}`
}

export const getDatabaseBackupRecordDownloadUrl = (id: number) =>
  buildNativeDownloadUrl(`/api/v1/databases/backup-records/${id}/download`)

export const runDatabaseRestoreDryRun = (id: number, data: DatabaseRestoreDryRunPayload) =>
  request.post(`/api/v1/databases/backup-records/${id}/restore-dry-run`, data)

export const listDatabaseRestoreJobs = (params?: {
  page?: number
  pageSize?: number
  backupRecordId?: number
  sourceInstanceId?: number
  targetInstanceId?: number
  status?: string
}) => request.get('/api/v1/databases/restore-jobs', { params })

export const listDatabaseInspectionReports = (params?: {
  page?: number
  pageSize?: number
  instanceId?: number
  status?: string
  riskLevel?: string
}) => request.get('/api/v1/databases/inspection-reports', { params })

export const getDatabaseInspectionReport = (id: number) =>
  request.get(`/api/v1/databases/inspection-reports/${id}`)

export const generateDatabaseInspectionReport = (data: { instanceId: number }) =>
  request.post('/api/v1/databases/inspection-reports', data)

export const createDatabaseInstance = (data: DatabaseInstancePayload) =>
  request.post('/api/v1/databases/instances', data)

export const getDatabaseInstance = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}`)

export const updateDatabaseInstance = (id: number, data: DatabaseInstancePayload) =>
  request.put(`/api/v1/databases/instances/${id}`, data)

export const deleteDatabaseInstance = (id: number) =>
  request.delete(`/api/v1/databases/instances/${id}`)

export const enableDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/enable`)

export const disableDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/disable`)

export const testDatabaseInstance = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/test`)

export const syncDatabaseMetadata = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/sync-metadata`)

export const listDatabaseSchemas = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/schemas`)

export const listDatabaseTables = (id: number, params?: { schemaName?: string }) =>
  request.get(`/api/v1/databases/instances/${id}/tables`, { params })

export const listDatabaseColumns = (id: number, params: { schemaName?: string; tableName: string }) =>
  request.get(`/api/v1/databases/instances/${id}/columns`, { params })

export const listDatabaseIndexes = (id: number, params: { schemaName?: string; tableName: string }) =>
  request.get(`/api/v1/databases/instances/${id}/indexes`, { params })

export const getDatabaseTableDDL = (id: number, params: DatabaseTableDDLPayload) =>
  request.get(`/api/v1/databases/instances/${id}/ddl`, { params })

export const exportDatabaseTableDictionary = (id: number, params: DatabaseTableDDLPayload) =>
  request.get(`/api/v1/databases/instances/${id}/dictionary/export`, {
    params,
    responseType: 'blob'
  })

export const getDatabaseDiagnosisMetrics = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/metrics`)

export const listDatabaseDiagnosisSessions = (id: number, params?: DatabaseDiagnosisListParams) =>
  request.get(`/api/v1/databases/instances/${id}/sessions`, { params })

export const listDatabaseSlowQueries = (id: number, params?: DatabaseDiagnosisListParams) =>
  request.get(`/api/v1/databases/instances/${id}/slow-queries`, { params })

export const getDatabaseTopology = (id: number) =>
  request.get(`/api/v1/databases/instances/${id}/topology`)

export const getDatabaseCapacityTrend = (id: number, params?: { range?: string; topLimit?: number }) =>
  request.get(`/api/v1/databases/instances/${id}/capacity-trend`, { params })

export const collectDatabaseCapacitySnapshot = (id: number) =>
  request.post(`/api/v1/databases/instances/${id}/capacity-snapshots`)

export const formatDatabaseQuery = (id: number, data: DatabaseQueryFormatPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/format`, data)

export const validateDatabaseWriteQuery = (id: number, data: DatabaseWriteValidatePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/write/validate`, data)

export const executeDatabaseWriteQuery = (id: number, data: DatabaseWriteExecutePayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/write`, data)

export const executeDatabaseQuery = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query`, data)

export const explainDatabaseQuery = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/explain`, data)

export const exportDatabaseQueryResult = (id: number, data: DatabaseQueryPayload) =>
  request.post(`/api/v1/databases/instances/${id}/query/export`, data, {
    responseType: 'blob'
  })
