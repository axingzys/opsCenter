<template>
  <div class="host-monitor-container">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><Monitor /></el-icon>
        </div>
        <div>
          <h2 class="page-title">主机监控</h2>
          <p class="page-subtitle">查看 Agent 主机的实时摘要、趋势和库存快照，支持 Linux / Windows</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button @click="loadData">
          <el-icon style="margin-right: 6px;"><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-icon stat-icon-primary"><el-icon><Grid /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">监控主机</div>
          <div class="stat-value">{{ stats.totalHosts }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-success"><el-icon><CircleCheck /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">健康</div>
          <div class="stat-value">{{ stats.healthyHosts }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-warning"><el-icon><Warning /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">告警</div>
          <div class="stat-value">{{ stats.warningHosts }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-danger"><el-icon><Bell /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">严重</div>
          <div class="stat-value">{{ stats.criticalHosts }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-muted"><el-icon><CircleClose /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">离线</div>
          <div class="stat-value">{{ stats.offlineHosts }}</div>
        </div>
      </div>
    </div>

    <div class="search-bar">
      <div class="search-inputs">
        <el-input
          v-model="searchForm.keyword"
          placeholder="搜索主机名/IP"
          clearable
          class="search-input"
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-select
          v-model="searchForm.osType"
          placeholder="操作系统"
          clearable
          class="search-input"
        >
          <el-option label="Linux" value="linux" />
          <el-option label="Windows" value="windows" />
        </el-select>

        <el-select
          v-model="searchForm.health"
          placeholder="健康状态"
          clearable
          class="search-input"
        >
          <el-option label="健康" value="healthy" />
          <el-option label="告警" value="warning" />
          <el-option label="严重" value="critical" />
          <el-option label="离线" value="offline" />
        </el-select>
      </div>

      <div class="search-actions">
        <el-button type="primary" @click="handleSearch">
          <el-icon style="margin-right: 6px;"><Search /></el-icon>
          搜索
        </el-button>
        <el-button @click="handleReset">
          <el-icon style="margin-right: 6px;"><RefreshLeft /></el-icon>
          重置
        </el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        v-loading="loading"
        :data="tableData"
        class="modern-table"
        :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
      >
        <el-table-column label="主机" min-width="220">
          <template #default="{ row }">
            <div class="host-cell">
              <div class="host-name-row">
                <span class="host-name">{{ row.name }}</span>
                <el-tag
                  size="small"
                  effect="plain"
                  :type="row.osType === 'windows' ? 'primary' : 'success'"
                >
                  {{ row.osType === 'windows' ? 'Windows' : 'Linux' }}
                </el-tag>
              </div>
              <div class="host-ip-row">
                <span>{{ row.ip }}</span>
                <span v-if="row.primaryPrivateIp" class="host-ip-extra">内网 {{ row.primaryPrivateIp }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="健康状态" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="getHealthTagType(row.healthStatus)">
              {{ row.healthStatusText || '-' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="版本" width="150" prop="agentVersion" />

        <el-table-column label="CPU" min-width="160">
          <template #default="{ row }">
            <div class="metric-cell">
              <span>{{ row.cpuCores || 0 }} 核</span>
              <span>{{ formatPercent(row.cpuUsage) }}</span>
            </div>
            <el-progress :percentage="normalizePercent(row.cpuUsage)" :stroke-width="8" :show-text="false" />
          </template>
        </el-table-column>

        <el-table-column label="内存" min-width="180">
          <template #default="{ row }">
            <div class="metric-cell">
              <span>{{ formatBytes(row.memoryUsed) }} / {{ formatBytes(row.memoryTotal) }}</span>
              <span>{{ formatPercent(row.memoryUsage) }}</span>
            </div>
            <el-progress :percentage="normalizePercent(row.memoryUsage)" :stroke-width="8" :show-text="false" />
          </template>
        </el-table-column>

        <el-table-column label="磁盘" min-width="180">
          <template #default="{ row }">
            <div class="metric-cell">
              <span>{{ formatBytes(row.diskUsed) }} / {{ formatBytes(row.diskTotal) }}</span>
              <span>{{ formatPercent(row.diskUsage) }}</span>
            </div>
            <el-progress :percentage="normalizePercent(row.diskUsage)" :stroke-width="8" :show-text="false" />
          </template>
        </el-table-column>

        <el-table-column label="触发规则" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="row.triggeredRuleCount > 0 ? 'danger' : 'info'">
              {{ row.triggeredRuleCount || 0 }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="最后上报" prop="lastReportAt" width="180" />

        <el-table-column label="操作" width="120" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">
              <el-icon><View /></el-icon>
              <span style="margin-left: 4px;">详情</span>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </div>

    <el-drawer
      v-model="detailVisible"
      title="主机监控详情"
      size="70%"
      destroy-on-close
    >
      <div v-loading="detailLoading" class="detail-wrapper">
        <template v-if="currentOverview?.host">
          <div class="detail-summary">
            <div class="summary-card">
              <span class="summary-label">主机</span>
              <span class="summary-value">{{ currentOverview.host.name }}</span>
            </div>
            <div class="summary-card">
              <span class="summary-label">目标</span>
              <span class="summary-value">{{ currentOverview.host.primaryPrivateIp || currentOverview.host.ip || '-' }}</span>
            </div>
            <div class="summary-card">
              <span class="summary-label">系统</span>
              <span class="summary-value">{{ currentOverview.host.os || '-' }}</span>
            </div>
            <div class="summary-card">
              <span class="summary-label">最后上报</span>
              <span class="summary-value">{{ currentOverview.host.lastReportAt || '-' }}</span>
            </div>
          </div>

          <div class="detail-section">
            <div class="detail-section-title">资源摘要</div>
            <HostMetricOverviewCards
              :summary="currentOverview.host"
              :trend="currentHistory"
            />
          </div>

          <div class="detail-section">
            <HostMetricTrendPanel
              :trend="currentHistory"
              :loading="historyLoading"
              :range="historyRange"
              @change-range="handleRangeChange"
            />
          </div>

          <div class="detail-section">
            <div class="detail-section-title">Agent 采集详情</div>
            <AgentInventoryPanel :inventory="currentOverview.inventory" :loading="detailLoading" />
          </div>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Bell,
  CircleCheck,
  CircleClose,
  Grid,
  Monitor,
  Refresh,
  RefreshLeft,
  Search,
  View,
  Warning
} from '@element-plus/icons-vue'
import {
  getMonitorHostHistory,
  getMonitorHostOverview,
  getMonitorHosts
} from '@/api/monitor-host'
import AgentInventoryPanel from '@/views/asset/components/AgentInventoryPanel.vue'
import HostMetricOverviewCards from '@/views/asset/components/HostMetricOverviewCards.vue'
import HostMetricTrendPanel from '@/views/asset/components/HostMetricTrendPanel.vue'

const loading = ref(false)
const detailLoading = ref(false)
const historyLoading = ref(false)
const detailVisible = ref(false)
const currentOverview = ref<any>(null)
const currentHistory = ref<any>(null)
const historyRange = ref<'1h' | '24h' | '7d' | '15d'>('1h')
const autoRefreshTimer = ref<number>()

const searchForm = reactive({
  keyword: '',
  osType: '',
  health: ''
})

const stats = reactive({
  totalHosts: 0,
  healthyHosts: 0,
  warningHosts: 0,
  criticalHosts: 0,
  offlineHosts: 0
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const tableData = ref<any[]>([])

const formatBytes = (value?: number) => {
  if (!value) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = value
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatPercent = (value?: number) => {
  if (value === undefined || value === null) return '-'
  return `${Number(value).toFixed(1)}%`
}

const normalizePercent = (value?: number) => {
  if (value === undefined || value === null) return 0
  return Math.min(Math.max(Number(value), 0), 100)
}

const getHealthTagType = (health: string) => {
  switch (health) {
    case 'healthy':
      return 'success'
    case 'warning':
      return 'warning'
    case 'critical':
      return 'danger'
    case 'offline':
      return 'info'
    default:
      return 'info'
  }
}

const loadData = async () => {
  loading.value = true
  try {
    const data = await getMonitorHosts({
      page: pagination.page,
      pageSize: pagination.pageSize,
      keyword: searchForm.keyword || undefined,
      osType: (searchForm.osType || undefined) as 'linux' | 'windows' | undefined,
      health: searchForm.health || undefined
    })
    tableData.value = data?.list || []
    pagination.total = data?.total || 0
    Object.assign(stats, data?.stats || {
      totalHosts: 0,
      healthyHosts: 0,
      warningHosts: 0,
      criticalHosts: 0,
      offlineHosts: 0
    })
  } catch (error: any) {
    ElMessage.error('加载主机监控列表失败')
  } finally {
    loading.value = false
  }
}

const loadHistory = async (hostId: number) => {
  historyLoading.value = true
  try {
    currentHistory.value = await getMonitorHostHistory(hostId, { range: historyRange.value })
  } catch (error: any) {
    currentHistory.value = null
    ElMessage.error('加载主机趋势失败')
  } finally {
    historyLoading.value = false
  }
}

const handleView = async (row: any) => {
  detailVisible.value = true
  detailLoading.value = true
  currentOverview.value = null
  currentHistory.value = null
  historyRange.value = '1h'
  try {
    const [overview, history] = await Promise.all([
      getMonitorHostOverview(row.id),
      getMonitorHostHistory(row.id, { range: '1h' })
    ])
    currentOverview.value = overview
    currentHistory.value = history
  } catch (error: any) {
    ElMessage.error('加载主机监控详情失败')
  } finally {
    detailLoading.value = false
  }
}

const handleRangeChange = async (value: '1h' | '24h' | '7d' | '15d') => {
  historyRange.value = value
  if (currentOverview.value?.host?.id) {
    await loadHistory(currentOverview.value.host.id)
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadData()
}

const handleReset = () => {
  searchForm.keyword = ''
  searchForm.osType = ''
  searchForm.health = ''
  pagination.page = 1
  loadData()
}

onMounted(() => {
  loadData()
  autoRefreshTimer.value = window.setInterval(() => {
    loadData()
  }, 30000)
})

onBeforeUnmount(() => {
  if (autoRefreshTimer.value) {
    window.clearInterval(autoRefreshTimer.value)
  }
})
</script>

<style scoped>
.host-monitor-container {
  padding: 0;
  background-color: transparent;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  padding: 16px 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}

.page-title-group {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.page-title-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, #000 0%, #1a1a1a 100%);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #d4af37;
  font-size: 22px;
  flex-shrink: 0;
  border: 1px solid #d4af37;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.page-subtitle {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: #909399;
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}

.stat-card {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
}

.stat-icon-primary {
  background: linear-gradient(135deg, #000 0%, #1a1a1a 100%);
  color: #d4af37;
  border: 1px solid #d4af37;
}

.stat-icon-success {
  background: linear-gradient(135deg, #4caf50 0%, #45a049 100%);
  color: #fff;
}

.stat-icon-warning {
  background: linear-gradient(135deg, #e6a23c 0%, #d9972c 100%);
  color: #fff;
}

.stat-icon-danger {
  background: linear-gradient(135deg, #f56c6c 0%, #f4534a 100%);
  color: #fff;
}

.stat-icon-muted {
  background: linear-gradient(135deg, #909399 0%, #6b7280 100%);
  color: #fff;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-bottom: 4px;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}

.search-bar {
  margin-bottom: 12px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.search-inputs {
  display: flex;
  gap: 12px;
  flex: 1;
}

.search-input {
  width: 280px;
}

.search-actions {
  display: flex;
  gap: 10px;
}

.table-wrapper {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.modern-table {
  width: 100%;
}

.pagination-container {
  padding: 16px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid #f0f0f0;
}

.host-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.host-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.host-name {
  font-weight: 600;
  color: #303133;
}

.host-ip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: #909399;
}

.host-ip-extra {
  color: #409eff;
}

.metric-cell {
  display: flex;
  justify-content: space-between;
  margin-bottom: 6px;
  font-size: 12px;
  color: #606266;
}

.detail-wrapper {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.summary-card {
  padding: 16px;
  border-radius: 12px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.summary-label {
  font-size: 12px;
  color: #6b7280;
}

.summary-value {
  font-size: 15px;
  font-weight: 600;
  color: #111827;
}

.detail-section {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 16px;
  padding: 18px;
}

.detail-section-title {
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 600;
  color: #111827;
}

@media (max-width: 1440px) {
  .stats-cards {
    grid-template-columns: repeat(3, 1fr);
  }

  .detail-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .stats-cards,
  .detail-summary {
    grid-template-columns: 1fr;
  }

  .search-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .search-inputs {
    flex-direction: column;
  }

  .search-input {
    width: 100%;
  }
}
</style>
