<template>
  <div class="agents-page-container">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><Connection /></el-icon>
        </div>
        <div>
          <h2 class="page-title">Agent管理</h2>
          <p class="page-subtitle">面向 Linux / Windows 主机的 Agent 生命周期与监控接入管理</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button class="black-button" @click="openDeployDialog">
          <el-icon style="margin-right: 6px;"><Plus /></el-icon>
          部署Agent
        </el-button>
        <el-button
          type="danger"
          plain
          :disabled="!selectedAgentHostIds.length"
          :loading="uninstallSubmitting"
          @click="handleBatchUninstall"
        >
          <el-icon style="margin-right: 6px;"><Delete /></el-icon>
          批量卸载{{ selectedAgentHostIds.length ? `(${selectedAgentHostIds.length})` : '' }}
        </el-button>
      </div>
    </div>

    <div class="filter-bar">
      <div class="filter-inputs">
        <el-input
          v-model="searchForm.keyword"
          placeholder="搜索主机名 / IP / Agent ID"
          clearable
          class="filter-input"
          @input="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-select
          v-model="searchForm.status"
          placeholder="Agent状态"
          clearable
          class="filter-input"
          @change="handleSearch"
        >
          <el-option label="运行中" value="running" />
          <el-option label="等待注册" value="waiting_register" />
          <el-option label="部署中" value="deploying" />
          <el-option label="离线" value="offline" />
          <el-option label="异常" value="error" />
        </el-select>
      </div>

      <div class="filter-actions">
        <el-button class="reset-btn" @click="handleReset">
          <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
          重置
        </el-button>
        <el-button @click="loadAgentList">
          <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        ref="agentTableRef"
        :data="agentList"
        v-loading="loading"
        class="modern-table"
        :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
        @selection-change="handleAgentSelectionChange"
      >
        <el-table-column type="selection" width="55" fixed="left" />
        <el-table-column label="主机" min-width="220" fixed="left">
          <template #default="{ row }">
            <div class="host-cell">
              <div class="host-avatar" :class="`status-${row.status}`">
                <el-icon><Monitor /></el-icon>
              </div>
              <div class="host-meta">
                <div class="host-name">{{ row.hostName }}</div>
                <div class="host-ip">{{ row.ip }}</div>
                <div class="host-extra">
                  <span v-if="row.primaryPrivateIp">内网 {{ row.primaryPrivateIp }}</span>
                  <span v-if="row.primaryPublicIp">公网 {{ row.primaryPublicIp }}</span>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="版本" width="130" align="center">
          <template #default="{ row }">
            <span>{{ row.version || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" effect="light">
              {{ row.statusText }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="监听端口" width="100" align="center" prop="listenPort" />

        <el-table-column label="安装进度" min-width="180">
          <template #default="{ row }">
            <div class="progress-cell">
              <el-progress
                :percentage="row.installProgress || 0"
                :stroke-width="8"
                :show-text="true"
                :color="progressColor(row.status)"
              />
              <div class="progress-stage">{{ row.installStageText || '-' }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="健康状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="healthTagType(row.healthStatus)" effect="plain">
              {{ healthStatusText(row.healthStatus) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="最后心跳" width="170" prop="lastHeartbeatAt" />
        <el-table-column label="最后上报" width="170" prop="lastReportAt" />
        <el-table-column label="更新时间" width="170" prop="updateTime" />

        <el-table-column label="最近错误" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.lastError" class="error-text">{{ row.lastError }}</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-tooltip content="查看库存" placement="top">
                <el-button link class="action-btn action-view" @click="openInventory(row)">
                  <el-icon><View /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="卸载Agent" placement="top">
                <el-button link class="action-btn action-delete" @click="handleSingleUninstall(row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadAgentList"
          @current-change="loadAgentList"
        />
      </div>
    </div>

    <el-dialog
      v-model="deployDialogVisible"
      title="部署 Agent"
      width="72%"
      class="responsive-dialog"
      @open="handleDeployDialogOpen"
    >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px;"
        title="Linux 主机通过 SSH 自动部署，Windows 主机通过 WinRM 自动部署。未配置对应管理凭据的主机仍需手工安装。"
      />

      <div class="deploy-filter-bar">
        <el-input
          v-model="deploySearchKeyword"
          placeholder="搜索待部署主机..."
          clearable
          class="filter-input"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <el-table
        ref="deployTableRef"
        :data="filteredDeployHosts"
        v-loading="deployLoading"
        @selection-change="handleDeploySelectionChange"
        :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
        max-height="460"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column label="主机" min-width="220">
          <template #default="{ row }">
              <div class="deploy-host-meta">
              <div class="deploy-host-name">{{ row.name }}</div>
              <div class="deploy-host-ip">{{ row.ip }}:{{ deployAccessPort(row) }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="系统" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="success">{{ (row.osType || 'linux').toUpperCase() }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="管理方式" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="row.managementMode === 'agent' ? 'success' : 'info'">
              {{ row.managementModeText || row.managementMode || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="业务分组" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.groupName || '未分组' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="连接账号" width="130">
          <template #default="{ row }">
            <span>{{ row.osType === 'linux' ? (row.sshUser || '-') : 'WinRM凭据' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="当前采集状态" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.collectStatus === 'online' ? 'success' : row.collectStatus === 'offline' ? 'danger' : 'info'">
              {{ row.collectStatusText || row.collectStatus || '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="现有Agent" width="140" align="center">
          <template #default="{ row }">
            <el-tag v-if="isRevokedAgentId(row.agentId)" type="warning">已吊销/可重装</el-tag>
            <el-tag v-else-if="hasActiveAgentIdentity(row)" type="success">已注册</el-tag>
            <span v-else class="text-muted">未部署</span>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <div class="dialog-footer">
          <span class="selection-count">已选择 {{ selectedDeployHostIds.length }} 台主机</span>
          <div>
            <el-button @click="deployDialogVisible = false">取消</el-button>
            <el-button class="black-button" :loading="deploySubmitting" @click="submitDeploy">
              开始部署
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-drawer
      v-model="inventoryDrawerVisible"
      size="56%"
      :title="inventoryTitle"
      destroy-on-close
    >
      <AgentInventoryPanel :inventory="inventory" :loading="inventoryLoading" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Connection,
  Plus,
  Search,
  Refresh,
  RefreshLeft,
  Monitor,
  View,
  Delete
} from '@element-plus/icons-vue'
import { deployAgents, getAgentInventory, getAgentList, uninstallAgents } from '@/api/agent'
import { getHostList } from '@/api/host'
import AgentInventoryPanel from './components/AgentInventoryPanel.vue'

interface AgentListItem {
  hostId: number
  hostName: string
  ip: string
  primaryPrivateIp?: string
  primaryPublicIp?: string
  version?: string
  status: string
  statusText: string
  listenPort: number
  installProgress: number
  installStage?: string
  installStageText?: string
  healthStatus?: string
  lastHeartbeatAt?: string
  lastReportAt?: string
  updateTime?: string
  lastError?: string
  agentId?: string
}

interface HostItem {
  id: number
  name: string
  groupName?: string
  ip: string
  port?: number
  managementPort?: number
  osType: string
  sshUser?: string
  credentialId?: number
  managementCredentialId?: number
  managementMode?: string
  managementModeText?: string
  collectStatus?: string
  collectStatusText?: string
  agentId?: string
}

interface InventoryState {
  hostId: number
  privateIps: string[]
  publicIps: string[]
  publicIpHistory: any[]
  disks: any[]
  topProcesses: any[]
  listeningPorts: any[]
  configSummary: {
    osRelease?: string
    kernel?: string
    arch?: string
    timezone?: string
    serviceManager?: string
    containerRuntime?: string
    agentVersion?: string
  }
  collectedAt?: string
}

const loading = ref(false)
const deployLoading = ref(false)
const deploySubmitting = ref(false)
const uninstallSubmitting = ref(false)
const inventoryLoading = ref(false)

const deployDialogVisible = ref(false)
const inventoryDrawerVisible = ref(false)
const inventoryTitle = ref('主机库存')
const deploySearchKeyword = ref('')

const agentList = ref<AgentListItem[]>([])
const deployHosts = ref<HostItem[]>([])
const selectedDeployHostIds = ref<number[]>([])
const selectedAgentHostIds = ref<number[]>([])
const agentTableRef = ref()
const deployTableRef = ref()

const searchForm = reactive({
  keyword: '',
  status: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const inventory = reactive<InventoryState>({
  hostId: 0,
  privateIps: [],
  publicIps: [],
  publicIpHistory: [],
  disks: [],
  topProcesses: [],
  listeningPorts: [],
  configSummary: {},
  collectedAt: ''
})

const filteredDeployHosts = computed(() => {
  const keyword = deploySearchKeyword.value.trim().toLowerCase()
  return deployHosts.value.filter(item => {
    if (!keyword) {
      return true
    }
    return [item.name, item.ip, item.sshUser, item.osType]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword))
  })
})

const isRevokedAgentId = (agentId?: string) => {
  return String(agentId || '').trim().startsWith('revoked:')
}

const hasActiveAgentIdentity = (item: HostItem) => {
  return !!item.agentId && !isRevokedAgentId(item.agentId)
}

const loadAgentList = async () => {
  loading.value = true
  try {
    const res = await getAgentList({
      page: pagination.page,
      pageSize: pagination.pageSize,
      keyword: searchForm.keyword,
      status: searchForm.status
    })
    agentList.value = res.list || []
    pagination.total = res.total || 0
    selectedAgentHostIds.value = []
    await nextTick()
    agentTableRef.value?.clearSelection?.()
  } finally {
    loading.value = false
  }
}

const loadDeployHosts = async () => {
  deployLoading.value = true
  try {
    const res = await getHostList({
      page: 1,
      pageSize: 500
    })
    const list = (res.list || []) as HostItem[]
    deployHosts.value = list.filter(item => {
      if (hasActiveAgentIdentity(item)) {
        return false
      }

      if (item.osType === 'linux') {
        return !!item.sshUser && !!(item.credentialId || item.managementCredentialId)
      }

      if (item.osType === 'windows') {
        return item.managementMode === 'agent' && !!item.managementCredentialId
      }

      return false
    })
  } finally {
    deployLoading.value = false
  }
}

const deployAccessPort = (row: HostItem) => {
  if (row.osType === 'windows') {
    return row.managementPort || 5985
  }
  return row.port || 22
}

const openDeployDialog = async () => {
  deployDialogVisible.value = true
}

const handleDeployDialogOpen = async () => {
  selectedDeployHostIds.value = []
  deploySearchKeyword.value = ''
  await nextTick()
  deployTableRef.value?.clearSelection?.()
  await loadDeployHosts()
}

const handleDeploySelectionChange = (rows: HostItem[]) => {
  selectedDeployHostIds.value = rows.map(item => item.id)
}

const handleAgentSelectionChange = (rows: AgentListItem[]) => {
  selectedAgentHostIds.value = rows.map(item => item.hostId)
}

const submitDeploy = async () => {
  if (!selectedDeployHostIds.value.length) {
    ElMessage.warning('请选择要部署的主机')
    return
  }

  deploySubmitting.value = true
  try {
    const jobs = await deployAgents({ hostIds: selectedDeployHostIds.value })
    const successCount = Array.isArray(jobs)
      ? jobs.filter((item: any) => item.status !== 'failed').length
      : 0
    const failedCount = Array.isArray(jobs)
      ? jobs.filter((item: any) => item.status === 'failed').length
      : 0

    ElMessage.success(`部署请求已执行，成功 ${successCount} 台，失败 ${failedCount} 台`)
    deployDialogVisible.value = false
    await loadAgentList()
  } finally {
    deploySubmitting.value = false
  }
}

const runUninstall = async (hostIds: number[], successMessage: string) => {
  uninstallSubmitting.value = true
  try {
    const jobs = await uninstallAgents({ hostIds })
    const successCount = Array.isArray(jobs)
      ? jobs.filter((item: any) => item.status !== 'failed').length
      : 0
    const failedCount = Array.isArray(jobs)
      ? jobs.filter((item: any) => item.status === 'failed').length
      : 0

    ElMessage.success(`${successMessage}，成功 ${successCount} 台，失败 ${failedCount} 台`)
    await loadAgentList()
  } finally {
    uninstallSubmitting.value = false
  }
}

const handleBatchUninstall = async () => {
  if (!selectedAgentHostIds.value.length) {
    ElMessage.warning('请选择要卸载的 Agent')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要批量卸载这 ${selectedAgentHostIds.value.length} 台主机上的 Agent 吗？`,
      '批量卸载确认',
      {
        type: 'warning',
        confirmButtonText: '确认卸载',
        cancelButtonText: '取消'
      }
    )

    await runUninstall(selectedAgentHostIds.value, '批量卸载任务已提交')
  } catch {
    return
  }
}

const handleSingleUninstall = async (row: AgentListItem) => {
  try {
    await ElMessageBox.confirm(
      `确定要卸载主机 "${row.hostName}" 上的 Agent 吗？`,
      '卸载确认',
      {
        type: 'warning',
        confirmButtonText: '确认卸载',
        cancelButtonText: '取消'
      }
    )

    await runUninstall([row.hostId], `主机 ${row.hostName} 卸载任务已提交`)
  } catch {
    return
  }
}

const openInventory = async (row: AgentListItem) => {
  inventoryDrawerVisible.value = true
  inventoryTitle.value = `${row.hostName} 库存详情`
  inventoryLoading.value = true
  try {
    const data = await getAgentInventory(row.hostId)
    inventory.hostId = data.hostId || row.hostId
    inventory.privateIps = data.privateIps || []
    inventory.publicIps = data.publicIps || []
    inventory.publicIpHistory = data.publicIpHistory || []
    inventory.disks = data.disks || []
    inventory.topProcesses = data.topProcesses || []
    inventory.listeningPorts = data.listeningPorts || []
    inventory.configSummary = data.configSummary || {}
    inventory.collectedAt = data.collectedAt || ''
  } finally {
    inventoryLoading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadAgentList()
}

const handleReset = () => {
  searchForm.keyword = ''
  searchForm.status = ''
  pagination.page = 1
  loadAgentList()
}

const statusTagType = (status: string) => {
  switch (status) {
    case 'running':
      return 'success'
    case 'waiting_register':
    case 'deploying':
      return 'warning'
    case 'offline':
    case 'error':
      return 'danger'
    default:
      return 'info'
  }
}

const healthTagType = (status?: string) => {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'degraded':
      return 'warning'
    default:
      return 'info'
  }
}

const healthStatusText = (status?: string) => {
  switch (status) {
    case 'healthy':
      return '健康'
    case 'degraded':
      return '告警'
    default:
      return '未知'
  }
}

const progressColor = (status: string) => {
  switch (status) {
    case 'running':
      return '#67c23a'
    case 'error':
    case 'offline':
      return '#f56c6c'
    default:
      return '#409eff'
  }
}

onMounted(() => {
  loadAgentList()
})
</script>

<style scoped>
.agents-page-container {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.page-title-group {
  display: flex;
  align-items: center;
  gap: 14px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.page-title-icon {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0f172a, #1e3a8a);
  color: #fff;
  font-size: 22px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #111827;
}

.page-subtitle {
  margin: 6px 0 0;
  color: #6b7280;
  font-size: 13px;
}

.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  padding: 16px 18px;
  border-radius: 16px;
  background: #fff;
  border: 1px solid #ebeef5;
}

.filter-inputs,
.filter-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filter-input {
  width: 260px;
}

.table-wrapper {
  padding: 18px;
  border-radius: 18px;
  background: #fff;
  border: 1px solid #ebeef5;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.action-btn {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.action-btn:hover {
  transform: translateY(-1px);
}

.action-view:hover {
  background: #eff6ff;
  color: #2563eb;
}

.action-delete:hover {
  background: #fef2f2;
  color: #dc2626;
}

.host-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.host-avatar {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #64748b, #334155);
}

.host-avatar.status-running {
  background: linear-gradient(135deg, #16a34a, #15803d);
}

.host-avatar.status-offline,
.host-avatar.status-error {
  background: linear-gradient(135deg, #ef4444, #dc2626);
}

.host-avatar.status-deploying,
.host-avatar.status-waiting_register {
  background: linear-gradient(135deg, #f59e0b, #d97706);
}

.host-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.host-name {
  font-weight: 600;
  color: #111827;
}

.host-ip {
  color: #4b5563;
  font-size: 13px;
}

.host-extra {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  color: #6b7280;
  font-size: 12px;
}

.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-stage {
  color: #6b7280;
  font-size: 12px;
}

.error-text {
  color: #dc2626;
}

.deploy-filter-bar {
  margin-bottom: 16px;
}

.deploy-host-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.deploy-host-name {
  font-weight: 600;
  color: #111827;
}

.deploy-host-ip {
  color: #6b7280;
  font-size: 13px;
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.selection-count {
  color: #6b7280;
  font-size: 13px;
}

.inventory-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.inventory-section {
  padding: 18px;
  border-radius: 16px;
  background: #fff;
  border: 1px solid #ebeef5;
}

.inventory-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 600;
  color: #111827;
}

.inventory-ip-group {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}

.inventory-ip-label {
  width: 56px;
  color: #6b7280;
  font-size: 13px;
}

.inventory-ip-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.inventory-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.black-button {
  background: #111827;
  border-color: #111827;
  color: #fff;
}

.black-button:hover {
  background: #1f2937;
  border-color: #1f2937;
}

.reset-btn {
  border-color: #d0d5dd;
}

.text-muted {
  color: #9ca3af;
}

@media (max-width: 1200px) {
  .inventory-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .page-header,
  .filter-bar,
  .dialog-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-inputs,
  .filter-actions {
    flex-wrap: wrap;
  }

  .filter-input {
    width: 100%;
  }
}
</style>
