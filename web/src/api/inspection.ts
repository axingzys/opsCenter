import request from '@/utils/request'

// ==================== 类型定义 ====================

export interface InspectionOptions {
  checkCluster: boolean
  checkNodes: boolean
  checkWorkloads: boolean
  checkNetwork: boolean
  checkStorage: boolean
  checkSecurity: boolean
  checkConfig: boolean
  checkCapacity: boolean
  checkEvents: boolean
}

export interface StartInspectionRequest {
  clusterIds: number[]
  options?: InspectionOptions
}

export interface InspectionProgress {
  inspectionId: number
  status: string
  progress: number
  currentStep: string
  completedClusters: number
  totalClusters: number
}

export interface CheckItem {
  category: string
  name: string
  status: 'success' | 'warning' | 'error'
  value: string
  expected: string
  detail: string
  suggestion: string
}

export interface ClusterInfoResult {
  version: string
  platform: string
  gitVersion: string
  goVersion: string
  buildDate: string
  connectionState: string
  connectionDelay: number
  items: CheckItem[]
}

export interface NodeResource {
  name: string
  cpuCapacity: string
  cpuUsed: string
  cpuUsagePercent: number
  memoryCapacity: string
  memoryUsed: string
  memoryPercent: number
  podCount: number
  podCapacity: number
  status: string
}

export interface NodeHealthResult {
  totalNodes: number
  readyNodes: number
  notReadyNodes: number
  pressureNodes: number
  taintedNodes: number
  nodeUtilization: NodeResource[]
  items: CheckItem[]
}

export interface ComponentStatus {
  name: string
  namespace: string
  status: string
  ready: string
  restarts: number
  age: string
}

export interface ComponentsResult {
  controlPlane: ComponentStatus[]
  addons: ComponentStatus[]
  items: CheckItem[]
}

export interface WorkloadInfo {
  kind: string
  namespace: string
  name: string
  ready: string
  status: string
  reason: string
}

export interface WorkloadsResult {
  totalDeployments: number
  healthyDeployments: number
  totalDaemonSets: number
  healthyDaemonSets: number
  totalStatefulSets: number
  healthyStatefulSets: number
  totalPods: number
  runningPods: number
  pendingPods: number
  failedPods: number
  highRestartPods: number
  imagePullErrors: number
  podsByPhase: Record<string, number>
  items: CheckItem[]
  unhealthyWorkloads: WorkloadInfo[]
}

export interface NetworkResult {
  totalServices: number
  clusterIPServices: number
  nodePortServices: number
  loadBalancerServices: number
  noEndpointServices: number
  totalIngresses: number
  networkPolicies: number
  items: CheckItem[]
}

export interface StorageResult {
  totalPVs: number
  availablePVs: number
  boundPVs: number
  releasedPVs: number
  failedPVs: number
  totalPVCs: number
  boundPVCs: number
  pendingPVCs: number
  storageClasses: number
  defaultSC: string
  items: CheckItem[]
}

export interface SecurityResult {
  serviceAccounts: number
  roles: number
  clusterRoles: number
  roleBindings: number
  clusterRoleBindings: number
  privilegedPods: number
  hostNetworkPods: number
  rootUserContainers: number
  clusterAdminBindings: number
  items: CheckItem[]
}

export interface ConfigResult {
  totalConfigMaps: number
  totalSecrets: number
  namespaceCount: number
  resourceQuotaCount: number
  limitRangeCount: number
  noRequestLimitPods: number
  items: CheckItem[]
}

export interface CapacityResult {
  totalCPU: string
  allocatedCPU: string
  cpuAllocatePercent: number
  totalMemory: string
  allocatedMemory: string
  memoryAllocatePercent: number
  totalPodCapacity: number
  currentPodCount: number
  podDensityPercent: number
  items: CheckItem[]
}

export interface EventInfo {
  type: string
  reason: string
  message: string
  object: string
  namespace: string
  count: number
  lastSeen: string
}

export interface EventsResult {
  warningEvents: number
  errorEvents: number
  recentEvents: EventInfo[]
  highFreqEvents: EventInfo[]
  items: CheckItem[]
}

export interface InspectionSummary {
  totalChecks: number
  passedChecks: number
  warningChecks: number
  failedChecks: number
  duration: number
}

export interface InspectionResult {
  clusterId: number
  clusterName: string
  score: number
  summary: InspectionSummary
  clusterInfo: ClusterInfoResult
  nodeHealth: NodeHealthResult
  components: ComponentsResult
  workloads: WorkloadsResult
  network: NetworkResult
  storage: StorageResult
  security: SecurityResult
  config: ConfigResult
  capacity: CapacityResult
  events: EventsResult
}

export interface ClusterInspection {
  id: number
  clusterId: number
  clusterName: string
  status: string
  score: number
  checkCount: number
  passCount: number
  warningCount: number
  failCount: number
  duration: number
  reportData: string
  userId: number
  startTime: string
  endTime: string
  createdAt: string
  updatedAt: string
}

export interface InspectionHistoryItem {
  id: number
  clusterId: number
  clusterName: string
  score: number
  status: string
  checkCount: number
  passCount: number
  warningCount: number
  failCount: number
  duration: number
  createdAt: string
}

// ==================== API 接口 ====================

/**
 * 开始集群巡检
 */
export function startInspection(data: StartInspectionRequest) {
  return request<{ inspectionId: number }>({
    url: '/api/v1/plugins/kubernetes/inspection/start',
    method: 'post',
    data
  })
}

/**
 * 获取巡检进度
 */
export function getInspectionProgress(inspectionId: number) {
  return request<InspectionProgress>({
    url: `/api/v1/plugins/kubernetes/inspection/progress/${inspectionId}`,
    method: 'get'
  })
}

/**
 * 获取巡检结果
 */
export function getInspectionResult(inspectionId: number) {
  return request<{
    inspection: ClusterInspection
    result: InspectionResult
  }>({
    url: `/api/v1/plugins/kubernetes/inspection/result/${inspectionId}`,
    method: 'get'
  })
}

/**
 * 获取巡检历史
 */
export function getInspectionHistory(params: {
  clusterId?: number
  page?: number
  pageSize?: number
}) {
  return request<{
    total: number
    list: InspectionHistoryItem[]
    page: number
    pageSize: number
  }>({
    url: '/api/v1/plugins/kubernetes/inspection/history',
    method: 'get',
    params
  })
}

/**
 * 删除巡检记录
 */
export function deleteInspection(inspectionId: number) {
  return request({
    url: `/api/v1/plugins/kubernetes/inspection/${inspectionId}`,
    method: 'delete'
  })
}

/**
 * 导出巡检报告
 */
export function exportInspection(inspectionId: number, format: 'excel' = 'excel') {
  return request({
    url: `/api/v1/plugins/kubernetes/inspection/export/${inspectionId}`,
    method: 'get',
    params: { format },
    responseType: 'blob'
  })
}
