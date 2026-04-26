<template>
  <div class="pve-topology">
    <div class="pve-toolbar">
      <div class="toolbar-left">
        <el-select v-model="viewMode" class="view-mode-select">
          <el-option label="服务器视图" value="server" />
        </el-select>
        <el-select
          v-model="selectedPlatformId"
          placeholder="全部平台"
          clearable
          class="platform-select"
        >
          <el-option v-for="item in platformOptions" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
        <el-input v-model="treeKeyword" clearable placeholder="搜索节点 / VM / IP" class="tree-search">
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>
      <div class="toolbar-actions">
        <div class="live-sync">
          <span class="live-dot" />
          <span>后台自动同步</span>
          <span class="freshness-text">{{ dataFreshnessText }}</span>
        </div>
        <el-radio-group :model-value="trendRange" size="small" @update:model-value="handleRangeChange">
          <el-radio-button label="24h">小时</el-radio-button>
          <el-radio-button label="7d">7 天</el-radio-button>
          <el-radio-button label="15d">15 天</el-radio-button>
        </el-radio-group>
        <el-button class="black-button" :loading="loading" @click="emit('refresh')">
          <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="pve-console" v-loading="loading">
      <el-empty v-if="platforms.length === 0" description="暂无拓扑数据，请等待自动同步或手动同步平台" />
      <template v-else>
        <aside class="resource-tree">
          <div class="tree-header">
            <div class="tree-title-row">
              <span class="tree-title">资源树</span>
              <el-tag size="small" effect="plain">{{ treeBadgeText }}</el-tag>
            </div>
            <div class="tree-subtitle">
              Datacenter / Node / VM
              <template v-if="treeKeyword.trim()"> · 已过滤</template>
            </div>
          </div>

          <div class="tree-scroll">
            <section v-for="platform in filteredPlatforms" :key="platform.id" class="tree-platform">
              <button
                type="button"
                class="tree-row tree-row-platform"
                :class="{ active: isTopologyNodeActive('platform', platform.id) }"
                @click="selectTopologyPlatform(platform)"
              >
                <span class="tree-caret">▾</span>
                <el-icon><Grid /></el-icon>
                <span class="tree-name">数据中心</span>
                <span class="tree-muted">{{ platform.name }}</span>
              </button>

              <div v-for="cluster in platform.clusters || []" :key="`${platform.id}-${cluster.id}`" class="tree-branch">
                <button
                  type="button"
                  class="tree-row tree-row-cluster"
                  :class="{ active: isTopologyNodeActive('cluster', platform.id, cluster.id) }"
                  @click="selectTopologyCluster(platform, cluster)"
                >
                  <span class="tree-caret">▾</span>
                  <el-icon><FolderOpened /></el-icon>
                  <span class="tree-name">{{ cluster.name }}</span>
                  <span class="tree-count">{{ cluster.hostCount || (cluster.hosts || []).length }}</span>
                </button>

                <div v-for="host in cluster.hosts || []" :key="`${platform.id}-${cluster.id}-${host.id}`" class="tree-branch">
                  <button
                    type="button"
                    class="tree-row tree-row-host"
                    :class="{ active: isTopologyNodeActive('host', platform.id, cluster.id, host.id) }"
                    @click="selectTopologyHost(platform, cluster, host)"
                  >
                    <span class="tree-status" :class="`status-${host.status || 'unknown'}`" />
                    <el-icon><Monitor /></el-icon>
                    <span class="tree-name">{{ host.name }}</span>
                    <span class="tree-count">{{ hostGuestCount(host) }}</span>
                  </button>

                  <button
                    v-for="guest in hostGuests(host)"
                    :key="`${platform.id}-${cluster.id}-${host.id}-${guest.id}`"
                    type="button"
                    class="tree-row tree-row-guest"
                    :class="{ active: isTopologyNodeActive('guest', platform.id, cluster.id, host.id, guest.id) }"
                    @click="selectTopologyGuest(platform, cluster, host, guest)"
                  >
                    <span class="guest-state" :class="`power-${guest.powerState || 'unknown'}`" />
                    <el-icon><Cpu /></el-icon>
                    <span class="tree-name">{{ guestTreeLabel(guest) }}</span>
                    <span class="tree-muted">{{ guestTypeLabel(guest.externalId) }}</span>
                  </button>
                </div>
              </div>
            </section>

            <el-empty v-if="filteredPlatforms.length === 0" description="没有匹配的资源" :image-size="64" />
          </div>
        </aside>

        <main class="server-view">
          <section class="object-header">
            <div class="object-title-block">
              <div class="object-title-row">
                <el-icon class="object-icon"><Monitor /></el-icon>
                <div>
                  <h3>{{ objectTitle }}</h3>
                  <p>{{ objectSubtitle }}</p>
                  <div class="object-path">
                    <span v-for="(segment, index) in objectPathSegments" :key="`${segment}-${index}`">{{ segment }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="object-actions">
              <el-tag v-for="item in objectMetaItems" :key="item.label" size="small" effect="plain">
                {{ item.label }}：{{ item.value }}
              </el-tag>
              <el-tag :type="objectStatusType" effect="plain">{{ objectStatusText }}</el-tag>
              <el-button class="reset-btn" @click="openGuestsFromTopology">
                查看虚机
              </el-button>
              <el-button class="reset-btn" @click="showCurrentTrend">
                查看趋势
              </el-button>
            </div>
          </section>

          <nav class="object-tabs">
            <button
              v-for="tab in topologyTabs"
              :key="tab.value"
              class="tab-button"
              :class="{ active: activeObjectTab === tab.value }"
              type="button"
              @click="setActiveObjectTab(tab.value)"
            >
              {{ tab.label }}
            </button>
          </nav>

          <div v-if="activeObjectTab === 'overview'" class="summary-grid">
            <section class="pve-card object-summary">
              <header class="card-header">
                <div>
                  <h4>{{ summaryTitle }}</h4>
                  <span>{{ summaryMeta }}</span>
                </div>
                <el-tag size="small" :type="selectedTopologyPlatform?.provider === 'pve' ? 'warning' : 'success'">
                  {{ selectedTopologyPlatform?.providerText || selectedTopologyPlatform?.provider || '-' }}
                </el-tag>
              </header>

              <div class="meter-list">
                <div v-for="item in meterRows" :key="item.label" class="meter-row">
                  <div class="meter-label">
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value }}</strong>
                  </div>
                  <div class="meter-track">
                    <span :style="{ width: `${item.percent}%` }" />
                  </div>
                  <div class="meter-help">{{ item.help }}</div>
                </div>
              </div>

              <div class="detail-table">
                <div v-for="row in detailRows" :key="row.label" class="detail-row">
                  <span>{{ row.label }}</span>
                  <strong>{{ row.value }}</strong>
                </div>
              </div>
            </section>

            <section class="pve-card resource-list">
              <header class="card-header">
                <div>
                  <h4>{{ resourceListTitle }}</h4>
                  <span>{{ resourceListSubtitle }}</span>
                </div>
                <el-tag size="small" effect="plain">{{ contextStats.hostCount }} Host / {{ contextStats.guestCount }} VM</el-tag>
              </header>

              <div v-if="currentGuestRows.length > 0" class="compact-table">
                <div class="compact-table-head guest-grid">
                  <span>VMID</span>
                  <span>名称</span>
                  <span>状态</span>
                  <span>CPU / 内存</span>
                  <span>IP</span>
                  <span>纳管</span>
                </div>
                <button
                  v-for="guest in currentGuestRows"
                  :key="guest.id"
                  type="button"
                  class="compact-table-row guest-grid"
                  :class="{ active: selectedTopologyGuest?.id === guest.id }"
                  @click="selectGuestFromContext(guest)"
                >
                  <span>{{ guestResourceId(guest.externalId) }}</span>
                  <strong>{{ guest.name }}</strong>
                  <el-tag size="small" :type="powerStateType(guest.powerState)">
                    {{ powerStateText(guest.powerState) }}
                  </el-tag>
                  <span>{{ guest.cpuCount || 0 }} vCPU / {{ formatMb(guest.memoryMb) }}</span>
                  <span>{{ guest.primaryIp || '-' }}</span>
                  <el-tag size="small" :type="bindingStatusType(guest.bindingStatus)">
                    {{ bindingStatusText(guest.bindingStatus) }}
                  </el-tag>
                </button>
              </div>

              <div v-else class="compact-table">
                <div class="compact-table-head host-grid">
                  <span>节点</span>
                  <span>状态</span>
                  <span>CPU</span>
                  <span>内存</span>
                  <span>虚机</span>
                </div>
                <button
                  v-for="host in currentHostRows"
                  :key="host.id"
                  type="button"
                  class="compact-table-row host-grid"
                  :class="{ active: selectedTopologyHost?.id === host.id }"
                  @click="selectHostFromContext(host)"
                >
                  <strong>{{ host.name }}</strong>
                  <el-tag size="small" :type="hostStatusType(host.status)">
                    {{ hostStatusText(host.status) }}
                  </el-tag>
                  <span>{{ host.cpuCores || 0 }} Core</span>
                  <span>{{ formatMemoryPair(host.memoryUsedMb, host.memoryTotalMb) }}</span>
                  <span>{{ hostGuestCount(host) }}</span>
                </button>
              </div>
            </section>
          </div>

          <template v-else-if="activeObjectTab === 'monitor'">
            <section class="monitor-summary-grid">
              <div v-for="item in monitorSummaryRows" :key="item.label" class="monitor-summary-card">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
                <small>{{ item.help }}</small>
              </div>
            </section>
            <section class="chart-grid" v-loading="trendLoading">
              <article v-for="panel in chartPanels" :key="panel.key" class="pve-card chart-card">
                <header class="chart-header">
                  <div>
                    <a>{{ panel.title }}</a>
                    <strong class="chart-current">{{ panel.currentValue }}</strong>
                  </div>
                  <div class="chart-legend">
                    <span v-for="series in panel.series" :key="series.name">
                      <i :style="{ background: series.color }" />
                      {{ series.name }}
                    </span>
                  </div>
                </header>
                <div v-if="hasTrendData" :ref="setChartRef(panel.key)" class="metric-chart" />
                <div v-else class="chart-empty">
                  <el-empty description="暂无趋势数据，等待同步快照生成" :image-size="58" />
                </div>
              </article>
            </section>
          </template>

          <section v-else-if="activeObjectTab === 'resources'" class="pve-card resource-list resource-list-wide">
            <header class="card-header">
              <div>
                <h4>{{ resourceListTitle }}</h4>
                <span>{{ resourceListSubtitle }}</span>
              </div>
              <el-tag size="small" effect="plain">{{ contextStats.hostCount }} Host / {{ contextStats.guestCount }} VM</el-tag>
            </header>

            <div v-if="currentGuestRows.length > 0" class="compact-table">
              <div class="compact-table-head guest-grid">
                <span>VMID</span>
                <span>名称</span>
                <span>状态</span>
                <span>CPU / 内存</span>
                <span>IP</span>
                <span>纳管</span>
              </div>
              <button
                v-for="guest in currentGuestRows"
                :key="guest.id"
                type="button"
                class="compact-table-row guest-grid"
                :class="{ active: selectedTopologyGuest?.id === guest.id }"
                @click="selectGuestFromContext(guest)"
              >
                <span>{{ guestResourceId(guest.externalId) }}</span>
                <strong>{{ guest.name }}</strong>
                <el-tag size="small" :type="powerStateType(guest.powerState)">
                  {{ powerStateText(guest.powerState) }}
                </el-tag>
                <span>{{ guest.cpuCount || 0 }} vCPU / {{ formatMb(guest.memoryMb) }}</span>
                <span>{{ guest.primaryIp || '-' }}</span>
                <el-tag size="small" :type="bindingStatusType(guest.bindingStatus)">
                  {{ bindingStatusText(guest.bindingStatus) }}
                </el-tag>
              </button>
            </div>

            <div v-else class="compact-table">
              <div class="compact-table-head host-grid">
                <span>节点</span>
                <span>状态</span>
                <span>CPU</span>
                <span>内存</span>
                <span>虚机</span>
              </div>
              <button
                v-for="host in currentHostRows"
                :key="host.id"
                type="button"
                class="compact-table-row host-grid"
                :class="{ active: selectedTopologyHost?.id === host.id }"
                @click="selectHostFromContext(host)"
              >
                <strong>{{ host.name }}</strong>
                <el-tag size="small" :type="hostStatusType(host.status)">
                  {{ hostStatusText(host.status) }}
                </el-tag>
                <span>{{ host.cpuCores || 0 }} Core</span>
                <span>{{ formatMemoryPair(host.memoryUsedMb, host.memoryTotalMb) }}</span>
                <span>{{ hostGuestCount(host) }}</span>
              </button>
            </div>
          </section>

          <section v-else class="task-panel task-panel-main">
            <header class="task-header">
              <div>
                <h4>任务</h4>
                <span>{{ taskPanelSubtitle }}</span>
              </div>
              <el-button class="reset-btn" size="small" :loading="syncJobsLoading" @click="emit('load-sync-jobs', selectedTopologyPlatform?.id)">
                <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
                刷新任务
              </el-button>
            </header>
            <div class="task-table" v-loading="syncJobsLoading">
              <el-empty v-if="taskRows.length === 0" description="暂无同步任务记录" :image-size="64" />
              <div v-else class="task-table-head task-grid">
                <span>开始时间</span>
                <span>触发</span>
                <span>状态</span>
                <span>条目</span>
                <span>完成时间 / 错误</span>
              </div>
              <div v-for="job in taskRows" :key="job.id" class="task-row task-grid">
                <span>{{ job.startedAt || job.createTime || '-' }}</span>
                <span>{{ syncTriggerText(job.triggerType) }}</span>
                <el-tag size="small" :type="syncStatusType(job.status)">
                  {{ syncStatusText(job.status) }}
                </el-tag>
                <span>{{ job.itemsTotal || 0 }} / +{{ job.itemsCreated || 0 }} / ~{{ job.itemsUpdated || 0 }}</span>
                <span :class="{ error: !!job.failureReason }">{{ job.failureReason || job.finishedAt || '-' }}</span>
              </div>
            </div>
          </section>

          <section v-if="activeObjectTab === 'overview'" class="chart-grid chart-grid-docked" v-loading="trendLoading">
            <article v-for="panel in chartPanels" :key="panel.key" class="pve-card chart-card">
              <header class="chart-header">
                <div>
                  <a>{{ panel.title }}</a>
                  <strong class="chart-current">{{ panel.currentValue }}</strong>
                </div>
                <div class="chart-legend">
                  <span v-for="series in panel.series" :key="series.name">
                    <i :style="{ background: series.color }" />
                    {{ series.name }}
                  </span>
                </div>
              </header>
              <div v-if="hasTrendData" :ref="setChartRef(panel.key)" class="metric-chart" />
              <div v-else class="chart-empty">
                <el-empty description="暂无趋势数据" :image-size="58" />
              </div>
            </article>
          </section>

          <section
            v-if="activeObjectTab !== 'tasks'"
            class="task-panel task-panel-docked"
            :class="{ collapsed: taskDockCollapsed }"
          >
            <header class="task-header">
              <div>
                <h4>任务</h4>
                <span>{{ taskPanelSubtitle }}</span>
              </div>
              <div class="task-actions">
                <el-button class="reset-btn" size="small" :loading="syncJobsLoading" @click="emit('load-sync-jobs', selectedTopologyPlatform?.id)">
                  <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
                  刷新任务
                </el-button>
                <el-button class="reset-btn" size="small" @click="taskDockCollapsed = !taskDockCollapsed">
                  {{ taskDockCollapsed ? '展开' : '收起' }}
                </el-button>
              </div>
            </header>
            <div v-show="!taskDockCollapsed" class="task-table" v-loading="syncJobsLoading">
              <el-empty v-if="taskRows.length === 0" description="暂无同步任务记录" :image-size="64" />
              <div v-else class="task-table-head task-grid">
                <span>开始时间</span>
                <span>触发</span>
                <span>状态</span>
                <span>条目</span>
                <span>完成时间 / 错误</span>
              </div>
              <div v-for="job in taskRows" :key="job.id" class="task-row task-grid">
                <span>{{ job.startedAt || job.createTime || '-' }}</span>
                <span>{{ syncTriggerText(job.triggerType) }}</span>
                <el-tag size="small" :type="syncStatusType(job.status)">
                  {{ syncStatusText(job.status) }}
                </el-tag>
                <span>{{ job.itemsTotal || 0 }} / +{{ job.itemsCreated || 0 }} / ~{{ job.itemsUpdated || 0 }}</span>
                <span :class="{ error: !!job.failureReason }">{{ job.failureReason || job.finishedAt || '-' }}</span>
              </div>
            </div>
          </section>
        </main>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type ComponentPublicInstance } from 'vue'
import { Cpu, FolderOpened, Grid, Monitor, Refresh, Search } from '@element-plus/icons-vue'

type TopologyNodeType = 'platform' | 'cluster' | 'host' | 'guest'
type TrendRange = '24h' | '7d' | '15d'
type TrendScopeType = 'platform' | 'cluster'
type TopologyTab = 'overview' | 'monitor' | 'resources' | 'tasks'
type TrendMetricKey =
  | 'guestTotal'
  | 'poweredOnGuests'
  | 'poweredOffGuests'
  | 'suspendedGuests'
  | 'boundGuests'
  | 'onlineGuests'
  | 'offlineGuests'
  | 'notConfiguredGuests'
  | 'unknownGuests'

interface TopologySelection {
  type: TopologyNodeType
  platformId: number
  clusterId?: number
  hostId?: number
  guestId?: number
}

interface TopologyGuestNode {
  id: number
  name: string
  clusterId?: number
  hostId?: number
  externalId?: string
  powerState?: string
  cpuCount?: number
  memoryMb?: number
  primaryIp?: string
  toolsStatus?: string
  bindingStatus?: string
}

interface TopologyHostNode {
  id: number
  name: string
  clusterId?: number
  externalId?: string
  managementIp?: string
  cpuModel?: string
  cpuCores?: number
  memoryTotalMb?: number
  memoryUsedMb?: number
  status?: string
  guestCount?: number
  lastCollectedAt?: string
  guests?: TopologyGuestNode[]
}

interface TopologyClusterNode {
  id: number
  name: string
  datacenter?: string
  status?: string
  hostCount?: number
  guestCount?: number
  hosts?: TopologyHostNode[]
}

interface TopologyPlatformNode {
  id: number
  name: string
  provider: string
  providerText?: string
  endpoint?: string
  port?: number
  status?: string
  lastSyncAt?: string
  clusters?: TopologyClusterNode[]
}

interface TrendPoint {
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

interface ChartSeries {
  name: string
  color: string
  data: number[]
  area?: boolean
}

interface ChartPanel {
  key: string
  title: string
  unit: string
  currentValue: string
  series: ChartSeries[]
}

const props = defineProps<{
  platformId: number | null
  platformOptions: any[]
  loading: boolean
  platforms: TopologyPlatformNode[]
  currentTrendPlatformId: number | null
  trend: any
  trendLoading: boolean
  trendRange: TrendRange
  trendScopeType: TrendScopeType
  trendClusterId: number | null
  trendClusterOptions: any[]
  trendPanelTitle: string
  selectedTrendMetrics: TrendMetricKey[]
  trendMetricOptions: readonly any[]
  syncJobs: any[]
  syncJobsLoading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:platformId', value: number | null): void
  (e: 'update:trendScopeType', value: TrendScopeType): void
  (e: 'update:trendClusterId', value: number | null): void
  (e: 'refresh'): void
  (e: 'change-trend-range', value: TrendRange): void
  (e: 'change-trend-metrics', value: TrendMetricKey[]): void
  (e: 'open-guests', value: { platformId: number; clusterId?: number }): void
  (e: 'load-sync-jobs', value?: number): void
}>()

const viewMode = ref('server')
const treeKeyword = ref('')
const activeObjectTab = ref<TopologyTab>('overview')
const taskDockCollapsed = ref(false)
const selectedTopologyNode = ref<TopologySelection | null>(null)
const chartRefs = new Map<string, HTMLElement>()
const charts = new Map<string, echarts.ECharts>()

const topologyTabs: Array<{ label: string; value: TopologyTab }> = [
  { label: '概要', value: 'overview' },
  { label: '监控', value: 'monitor' },
  { label: '资源', value: 'resources' },
  { label: '任务', value: 'tasks' }
]

const selectedPlatformId = computed({
  get: () => props.platformId,
  set: (value: number | null) => emit('update:platformId', value)
})

const localTrendScopeType = computed({
  get: () => props.trendScopeType,
  set: (value: TrendScopeType) => emit('update:trendScopeType', value)
})

const localTrendClusterId = computed({
  get: () => props.trendClusterId,
  set: (value: number | null) => emit('update:trendClusterId', value)
})

const topologyStats = computed(() => {
  let clusterCount = 0
  let hostCount = 0
  let guestCount = 0
  props.platforms.forEach((platform) => {
    const clusters = platform.clusters || []
    clusterCount += clusters.length
    clusters.forEach((cluster) => {
      const hosts = cluster.hosts || []
      hostCount += hosts.length
      guestCount += hosts.reduce((total, host) => total + hostGuestCount(host), 0)
    })
  })
  return {
    platformCount: props.platforms.length,
    clusterCount,
    hostCount,
    guestCount
  }
})

const filteredTreeStats = computed(() => {
  let clusterCount = 0
  let hostCount = 0
  let guestCount = 0
  filteredPlatforms.value.forEach((platform) => {
    const clusters = platform.clusters || []
    clusterCount += clusters.length
    clusters.forEach((cluster) => {
      const hosts = cluster.hosts || []
      hostCount += hosts.length
      guestCount += hosts.reduce((total, host) => total + hostGuestCount(host), 0)
    })
  })
  return {
    platformCount: filteredPlatforms.value.length,
    clusterCount,
    hostCount,
    guestCount
  }
})

const treeBadgeText = computed(() => {
  const keyword = normalizeKeyword(treeKeyword.value)
  if (!keyword) return `${topologyStats.value.guestCount} VM`
  return `${filteredTreeStats.value.guestCount} / ${topologyStats.value.guestCount} VM`
})

const filteredPlatforms = computed(() => {
  const keyword = normalizeKeyword(treeKeyword.value)
  if (!keyword) return props.platforms

  return props.platforms
    .map((platform) => {
      const platformMatched = matchesKeyword(platform.name, keyword) || matchesKeyword(platform.providerText, keyword)
      const clusters = (platform.clusters || [])
        .map((cluster) => {
          const clusterMatched = matchesKeyword(cluster.name, keyword) || matchesKeyword(cluster.datacenter, keyword)
          const hosts = (cluster.hosts || [])
            .map((host) => {
              const guests = hostGuests(host).filter((guest) => isGuestMatched(guest, keyword))
              if (clusterMatched || platformMatched || isHostMatched(host, keyword) || guests.length > 0) {
                return {
                  ...host,
                  guests: guests.length > 0 || isHostMatched(host, keyword) || clusterMatched || platformMatched ? guests : []
                }
              }
              return null
            })
            .filter(Boolean) as TopologyHostNode[]
          if (platformMatched || clusterMatched || hosts.length > 0) {
            return {
              ...cluster,
              hosts
            }
          }
          return null
        })
        .filter(Boolean) as TopologyClusterNode[]
      if (platformMatched || clusters.length > 0) {
        return {
          ...platform,
          clusters
        }
      }
      return null
    })
    .filter(Boolean) as TopologyPlatformNode[]
})

const selectedTopologyPlatform = computed<TopologyPlatformNode | null>(() => {
  const selection = selectedTopologyNode.value
  if (!selection) return props.platforms[0] || null
  return props.platforms.find((item) => item.id === selection.platformId) || null
})

const selectedTopologyCluster = computed<TopologyClusterNode | null>(() => {
  const selection = selectedTopologyNode.value
  const platform = selectedTopologyPlatform.value
  if (!selection || !platform || !selection.clusterId) return null
  return (platform.clusters || []).find((item) => item.id === selection.clusterId) || null
})

const selectedTopologyHost = computed<TopologyHostNode | null>(() => {
  const selection = selectedTopologyNode.value
  const cluster = selectedTopologyCluster.value
  if (!selection || !cluster || !selection.hostId) return null
  return (cluster.hosts || []).find((item) => item.id === selection.hostId) || null
})

const selectedTopologyGuest = computed<TopologyGuestNode | null>(() => {
  const selection = selectedTopologyNode.value
  const host = selectedTopologyHost.value
  if (!selection || !host || !selection.guestId) return null
  return hostGuests(host).find((item) => item.id === selection.guestId) || null
})

const contextHosts = computed(() => {
  if (selectedTopologyHost.value) return [selectedTopologyHost.value]
  if (selectedTopologyCluster.value) return selectedTopologyCluster.value.hosts || []
  if (selectedTopologyPlatform.value) return platformHosts(selectedTopologyPlatform.value)
  return []
})

const contextGuests = computed(() => {
  if (selectedTopologyGuest.value) return [selectedTopologyGuest.value]
  return contextHosts.value.flatMap((host) => hostGuests(host))
})

const currentHostRows = computed(() => contextHosts.value)
const currentGuestRows = computed(() => {
  if (selectedTopologyGuest.value) return [selectedTopologyGuest.value]
  if (selectedTopologyHost.value) return hostGuests(selectedTopologyHost.value)
  return []
})

const contextStats = computed(() => {
  const hosts = contextHosts.value
  const guests = contextGuests.value
  const poweredOn = guests.filter((guest) => guest.powerState === 'powered_on').length
  const poweredOff = guests.filter((guest) => guest.powerState === 'powered_off').length
  const bound = guests.filter((guest) => guest.bindingStatus === 'bound').length
  const online = guests.filter((guest) => guest.powerState === 'powered_on' || guest.toolsStatus === 'guestToolsRunning').length
  const cpuCores = hosts.reduce((total, host) => total + Number(host.cpuCores || 0), 0)
  const allocatedCpu = guests.reduce((total, guest) => total + Number(guest.cpuCount || 0), 0)
  const memoryTotal = hosts.reduce((total, host) => total + Number(host.memoryTotalMb || 0), 0)
  const memoryUsed = hosts.reduce((total, host) => total + Number(host.memoryUsedMb || 0), 0)
  const allocatedMemory = guests.reduce((total, guest) => total + Number(guest.memoryMb || 0), 0)
  return {
    hostCount: hosts.length,
    guestCount: guests.length,
    poweredOn,
    poweredOff,
    bound,
    online,
    cpuCores,
    allocatedCpu,
    memoryTotal,
    memoryUsed,
    allocatedMemory
  }
})

const dataFreshnessText = computed(() => {
  if (selectedTopologyHost.value?.lastCollectedAt) return `采集 ${selectedTopologyHost.value.lastCollectedAt}`
  if (selectedTopologyPlatform.value?.lastSyncAt) return `同步 ${selectedTopologyPlatform.value.lastSyncAt}`
  return '等待同步'
})

const objectTitle = computed(() => {
  if (selectedTopologyGuest.value) return `${guestResourceId(selectedTopologyGuest.value.externalId)} (${selectedTopologyGuest.value.name})`
  if (selectedTopologyHost.value) return `节点 '${selectedTopologyHost.value.name}'`
  if (selectedTopologyCluster.value) return selectedTopologyCluster.value.name
  return selectedTopologyPlatform.value?.name || '服务器视图'
})

const objectSubtitle = computed(() => {
  const platform = selectedTopologyPlatform.value
  if (selectedTopologyGuest.value) {
    return `${guestTypeLabel(selectedTopologyGuest.value.externalId)} · ${selectedTopologyHost.value?.name || '-'} · ${platform?.name || '-'}`
  }
  if (selectedTopologyHost.value) {
    return `${platform?.provider === 'pve' ? 'Proxmox VE Node' : 'vSphere Host'} · ${selectedTopologyCluster.value?.name || '-'}`
  }
  if (selectedTopologyCluster.value) {
    return `${platform?.provider === 'pve' ? 'PVE Cluster' : 'vSphere Cluster'} · ${selectedTopologyCluster.value.datacenter || '-'}`
  }
  return platform ? `${topologyProviderLabel(platform.provider)} · ${platform.endpoint || '-'}:${platform.port || '-'}` : '选择左侧资源查看概要'
})

const objectStatusText = computed(() => {
  if (selectedTopologyGuest.value) return powerStateText(selectedTopologyGuest.value.powerState)
  if (selectedTopologyHost.value) return hostStatusText(selectedTopologyHost.value.status)
  return selectedTopologyPlatform.value?.status === 'enabled' ? '启用' : selectedTopologyPlatform.value?.status || '未知'
})

const objectStatusType = computed(() => {
  if (selectedTopologyGuest.value) return powerStateType(selectedTopologyGuest.value.powerState)
  if (selectedTopologyHost.value) return hostStatusType(selectedTopologyHost.value.status)
  return selectedTopologyPlatform.value?.status === 'enabled' ? 'success' : 'info'
})

const objectPathSegments = computed(() => {
  const segments: string[] = []
  const platform = selectedTopologyPlatform.value
  const cluster = selectedTopologyCluster.value
  const host = selectedTopologyHost.value
  const guest = selectedTopologyGuest.value
  if (platform) segments.push(platform.provider === 'pve' ? 'Datacenter' : 'vCenter', platform.name)
  if (cluster) segments.push(cluster.name)
  if (host) segments.push(host.name)
  if (guest) segments.push(`${guestTypeLabel(guest.externalId)} ${guestResourceId(guest.externalId)}`)
  return segments
})

const objectMetaItems = computed(() => {
  const stats = contextStats.value
  const scope = selectedTopologyGuest.value
    ? '虚机'
    : selectedTopologyHost.value
      ? '节点'
      : selectedTopologyCluster.value
        ? '集群'
        : '平台'
  const trendWindow = props.trendRange === '24h' ? '小时' : props.trendRange === '7d' ? '7 天' : '15 天'
  return [
    { label: '范围', value: scope },
    { label: 'Host', value: `${stats.hostCount}` },
    { label: 'VM', value: `${stats.guestCount}` },
    { label: '趋势', value: trendWindow }
  ]
})

const summaryTitle = computed(() => {
  if (selectedTopologyGuest.value) return '虚机概要'
  if (selectedTopologyHost.value) return `${selectedTopologyHost.value.name} 概要`
  if (selectedTopologyCluster.value) return '集群概要'
  return '数据中心概要'
})

const summaryMeta = computed(() => {
  if (selectedTopologyHost.value?.lastCollectedAt) return `采集时间 ${selectedTopologyHost.value.lastCollectedAt}`
  if (selectedTopologyPlatform.value?.lastSyncAt) return `最近同步 ${selectedTopologyPlatform.value.lastSyncAt}`
  return '等待同步数据'
})

const meterRows = computed(() => {
  const stats = contextStats.value
  const cpuPercent = percentage(stats.allocatedCpu, stats.cpuCores)
  const memoryPercent = percentage(stats.memoryUsed || stats.allocatedMemory, stats.memoryTotal)
  const powerPercent = percentage(stats.poweredOn, stats.guestCount)
  const boundPercent = percentage(stats.bound, stats.guestCount)
  return [
    {
      label: 'CPU',
      value: stats.cpuCores > 0 ? `${stats.allocatedCpu} / ${stats.cpuCores} vCPU` : `${stats.allocatedCpu} vCPU`,
      percent: cpuPercent,
      help: stats.cpuCores > 0 ? `已分配 ${formatPercent(cpuPercent)}` : '暂无物理核心数据'
    },
    {
      label: '内存',
      value: stats.memoryTotal > 0 ? formatMemoryPair(stats.memoryUsed || stats.allocatedMemory, stats.memoryTotal) : formatMb(stats.allocatedMemory),
      percent: memoryPercent,
      help: stats.memoryTotal > 0 ? `使用率 ${formatPercent(memoryPercent)}` : '暂无宿主机内存总量'
    },
    {
      label: '运行虚机',
      value: `${stats.poweredOn} / ${stats.guestCount}`,
      percent: powerPercent,
      help: `关机 ${stats.poweredOff} 台`
    },
    {
      label: '纳管',
      value: `${stats.bound} / ${stats.guestCount}`,
      percent: boundPercent,
      help: `未纳管 ${Math.max(0, stats.guestCount - stats.bound)} 台`
    }
  ]
})

const detailRows = computed(() => {
  const platform = selectedTopologyPlatform.value
  const cluster = selectedTopologyCluster.value
  const host = selectedTopologyHost.value
  const guest = selectedTopologyGuest.value
  if (guest) {
    return [
      { label: 'VMID', value: guestResourceId(guest.externalId) },
      { label: '类型', value: guestTypeLabel(guest.externalId) },
      { label: '宿主节点', value: host?.name || '-' },
      { label: '电源状态', value: powerStateText(guest.powerState) },
      { label: 'CPU', value: `${guest.cpuCount || 0} vCPU` },
      { label: '内存', value: formatMb(guest.memoryMb) },
      { label: '主 IP', value: guest.primaryIp || '-' },
      { label: '纳管状态', value: bindingStatusText(guest.bindingStatus) }
    ]
  }
  if (host) {
    return [
      { label: '节点', value: host.name },
      { label: '状态', value: hostStatusText(host.status) },
      { label: '管理 IP', value: host.managementIp || '-' },
      { label: 'CPU', value: host.cpuCores ? `${host.cpuCores} Core` : '-' },
      { label: 'CPU 型号', value: host.cpuModel || '-' },
      { label: '内存', value: formatMemoryPair(host.memoryUsedMb, host.memoryTotalMb) },
      { label: '虚机数量', value: `${hostGuestCount(host)} 台` },
      { label: '平台', value: platform?.name || '-' }
    ]
  }
  if (cluster) {
    return [
      { label: '集群', value: cluster.name },
      { label: '数据中心', value: cluster.datacenter || '-' },
      { label: '状态', value: cluster.status || '-' },
      { label: '宿主机', value: `${contextStats.value.hostCount} 台` },
      { label: '虚机', value: `${contextStats.value.guestCount} 台` },
      { label: '平台', value: platform?.name || '-' }
    ]
  }
  return [
    { label: '平台', value: platform?.name || '-' },
    { label: '类型', value: platform ? topologyProviderLabel(platform.provider) : '-' },
    { label: '地址', value: platform ? `${platform.endpoint || '-'}:${platform.port || '-'}` : '-' },
    { label: '状态', value: platform?.status === 'enabled' ? '启用' : platform?.status || '-' },
    { label: '集群', value: `${platform?.clusters?.length || 0} 个` },
    { label: '宿主机', value: `${contextStats.value.hostCount} 台` },
    { label: '虚机', value: `${contextStats.value.guestCount} 台` },
    { label: '最近同步', value: platform?.lastSyncAt || '-' }
  ]
})

const resourceListTitle = computed(() => {
  if (selectedTopologyGuest.value) return '当前虚机'
  if (selectedTopologyHost.value) return '节点虚机清单'
  if (selectedTopologyCluster.value) return '集群资源清单'
  return '平台资源清单'
})

const resourceListSubtitle = computed(() => {
  if (selectedTopologyHost.value) return `${selectedTopologyHost.value.name} 下的 QEMU / LXC 实例`
  if (selectedTopologyCluster.value) return `${selectedTopologyCluster.value.name} 下的节点和虚机`
  return '按服务器视图汇总当前对象资源'
})

const trendPoints = computed<TrendPoint[]>(() => props.trend?.points || [])
const hasTrendData = computed(() => trendPoints.value.length > 0)
const chartLabels = computed(() => trendPoints.value.map(formatTrendPointLabel))
const latestTrendPoint = computed(() => trendPoints.value[trendPoints.value.length - 1] || null)

const monitorSummaryRows = computed(() => {
  const stats = contextStats.value
  const latest = latestTrendPoint.value
  return [
    {
      label: 'VM 总数',
      value: `${latest?.guestTotal ?? stats.guestCount}`,
      help: hasTrendData.value ? '最近趋势快照' : '当前拓扑汇总'
    },
    {
      label: '开机',
      value: `${latest?.poweredOnGuests ?? stats.poweredOn}`,
      help: `关机 ${latest?.poweredOffGuests ?? stats.poweredOff}`
    },
    {
      label: '已纳管',
      value: `${latest?.boundGuests ?? stats.bound}`,
      help: `未纳管 ${Math.max(0, (latest?.guestTotal ?? stats.guestCount) - (latest?.boundGuests ?? stats.bound))}`
    },
    {
      label: '在线',
      value: `${latest?.onlineGuests ?? stats.online}`,
      help: `离线 ${latest?.offlineGuests ?? Math.max(0, stats.guestCount - stats.online)}`
    }
  ]
})

const chartPanels = computed<ChartPanel[]>(() => {
  const points = trendPoints.value
  const stats = contextStats.value
  const latest = latestTrendPoint.value
  const total = latest?.guestTotal ?? stats.guestCount
  const poweredOn = latest?.poweredOnGuests ?? stats.poweredOn
  const poweredOff = latest?.poweredOffGuests ?? stats.poweredOff
  const bound = latest?.boundGuests ?? stats.bound
  const online = latest?.onlineGuests ?? stats.online
  const offline = latest?.offlineGuests ?? Math.max(0, stats.guestCount - stats.online)
  return [
    {
      key: 'guest-total',
      title: '虚机总量',
      unit: '台',
      currentValue: `${total} 台`,
      series: [
        { name: '总数', color: '#89a61f', data: points.map((item) => item.guestTotal), area: true }
      ]
    },
    {
      key: 'power-state',
      title: '电源状态',
      unit: '台',
      currentValue: `开机 ${poweredOn} / 关机 ${poweredOff}`,
      series: [
        { name: '开机', color: '#89a61f', data: points.map((item) => item.poweredOnGuests), area: true },
        { name: '关机', color: '#5b8cc0', data: points.map((item) => item.poweredOffGuests) },
        { name: '挂起', color: '#d39a22', data: points.map((item) => item.suspendedGuests) }
      ]
    },
    {
      key: 'onboard-state',
      title: '纳管状态',
      unit: '台',
      currentValue: `已纳管 ${bound} / 未纳管 ${Math.max(0, total - bound)}`,
      series: [
        { name: '已纳管', color: '#20a162', data: points.map((item) => item.boundGuests), area: true },
        { name: '未纳管', color: '#9aa0a6', data: points.map((item) => Math.max(0, item.guestTotal - item.boundGuests)) }
      ]
    },
    {
      key: 'runtime-state',
      title: '运行连通状态',
      unit: '台',
      currentValue: `在线 ${online} / 离线 ${offline}`,
      series: [
        { name: '在线', color: '#2e8b57', data: points.map((item) => item.onlineGuests), area: true },
        { name: '离线', color: '#c2410c', data: points.map((item) => item.offlineGuests) },
        { name: '未配置', color: '#6d6f75', data: points.map((item) => item.notConfiguredGuests + item.unknownGuests) }
      ]
    }
  ]
})

const taskRows = computed(() => (props.syncJobs || []).slice(0, 8))

const taskPanelSubtitle = computed(() => {
  const platform = selectedTopologyPlatform.value
  if (!platform) return '当前平台最近同步记录'
  return `${platform.name} 最近同步记录`
})

const normalizeKeyword = (value?: string) => (value || '').trim().toLowerCase()
const matchesKeyword = (value: unknown, keyword: string) => String(value || '').toLowerCase().includes(keyword)

const isHostMatched = (host: TopologyHostNode, keyword: string) => {
  return matchesKeyword(host.name, keyword)
    || matchesKeyword(host.managementIp, keyword)
    || matchesKeyword(host.cpuModel, keyword)
    || matchesKeyword(host.externalId, keyword)
}

const isGuestMatched = (guest: TopologyGuestNode, keyword: string) => {
  return matchesKeyword(guest.name, keyword)
    || matchesKeyword(guest.primaryIp, keyword)
    || matchesKeyword(guest.externalId, keyword)
    || matchesKeyword(guestResourceId(guest.externalId), keyword)
}

const hostGuests = (host?: TopologyHostNode | null) => Array.isArray(host?.guests) ? host!.guests! : []
const hostGuestCount = (host?: TopologyHostNode | null) => hostGuests(host).length || Number(host?.guestCount || 0)
const platformHosts = (platform: TopologyPlatformNode) => (platform.clusters || []).flatMap((cluster) => cluster.hosts || [])

const ensureTopologySelection = () => {
  if (isCurrentTopologySelectionValid()) return
  const platform = props.platforms[0]
  if (!platform) {
    selectedTopologyNode.value = null
    return
  }
  const cluster = platform.clusters?.[0]
  const host = cluster?.hosts?.[0]
  if (host && cluster) {
    selectedTopologyNode.value = {
      type: 'host',
      platformId: platform.id,
      clusterId: cluster.id,
      hostId: host.id
    }
    emit('update:trendScopeType', 'cluster')
    emit('update:trendClusterId', cluster.id)
    return
  }
  if (cluster) {
    selectedTopologyNode.value = {
      type: 'cluster',
      platformId: platform.id,
      clusterId: cluster.id
    }
    emit('update:trendScopeType', 'cluster')
    emit('update:trendClusterId', cluster.id)
    return
  }
  selectedTopologyNode.value = {
    type: 'platform',
    platformId: platform.id
  }
  emit('update:trendScopeType', 'platform')
}

const isCurrentTopologySelectionValid = () => {
  const selection = selectedTopologyNode.value
  if (!selection) return false
  const platform = props.platforms.find((item) => item.id === selection.platformId)
  if (!platform) return false
  if (selection.type === 'platform') return true
  const cluster = (platform.clusters || []).find((item) => item.id === selection.clusterId)
  if (!cluster) return false
  if (selection.type === 'cluster') return true
  const host = (cluster.hosts || []).find((item) => item.id === selection.hostId)
  if (!host) return false
  if (selection.type === 'host') return true
  return hostGuests(host).some((item) => item.id === selection.guestId)
}

const selectTopologyPlatform = (platform: TopologyPlatformNode) => {
  selectedTopologyNode.value = {
    type: 'platform',
    platformId: platform.id
  }
  if (props.platformId !== platform.id) emit('update:platformId', platform.id)
  localTrendScopeType.value = 'platform'
}

const selectTopologyCluster = (platform: TopologyPlatformNode, cluster: TopologyClusterNode) => {
  selectedTopologyNode.value = {
    type: 'cluster',
    platformId: platform.id,
    clusterId: cluster.id
  }
  if (props.platformId !== platform.id) emit('update:platformId', platform.id)
  localTrendScopeType.value = 'cluster'
  localTrendClusterId.value = cluster.id
}

const selectTopologyHost = (platform: TopologyPlatformNode, cluster: TopologyClusterNode, host: TopologyHostNode) => {
  selectedTopologyNode.value = {
    type: 'host',
    platformId: platform.id,
    clusterId: cluster.id,
    hostId: host.id
  }
  if (props.platformId !== platform.id) emit('update:platformId', platform.id)
  localTrendScopeType.value = 'cluster'
  localTrendClusterId.value = cluster.id
}

const selectTopologyGuest = (
  platform: TopologyPlatformNode,
  cluster: TopologyClusterNode,
  host: TopologyHostNode,
  guest: TopologyGuestNode
) => {
  selectedTopologyNode.value = {
    type: 'guest',
    platformId: platform.id,
    clusterId: cluster.id,
    hostId: host.id,
    guestId: guest.id
  }
  if (props.platformId !== platform.id) emit('update:platformId', platform.id)
  localTrendScopeType.value = 'cluster'
  localTrendClusterId.value = cluster.id
}

const selectHostFromContext = (host: TopologyHostNode) => {
  const platform = selectedTopologyPlatform.value
  const cluster = (platform?.clusters || []).find((item) => (item.hosts || []).some((child) => child.id === host.id))
  if (!platform || !cluster) return
  selectTopologyHost(platform, cluster, host)
}

const selectGuestFromContext = (guest: TopologyGuestNode) => {
  const platform = selectedTopologyPlatform.value
  if (!platform) return
  for (const cluster of platform.clusters || []) {
    for (const host of cluster.hosts || []) {
      if (hostGuests(host).some((item) => item.id === guest.id)) {
        selectTopologyGuest(platform, cluster, host, guest)
        return
      }
    }
  }
}

const isTopologyNodeActive = (type: TopologyNodeType, platformId: number, clusterId?: number, hostId?: number, guestId?: number) => {
  const selection = selectedTopologyNode.value
  if (!selection || selection.type !== type || selection.platformId !== platformId) return false
  if (type === 'platform') return true
  if (selection.clusterId !== clusterId) return false
  if (type === 'cluster') return true
  if (selection.hostId !== hostId) return false
  if (type === 'host') return true
  return selection.guestId === guestId
}

const openGuestsFromTopology = () => {
  const platform = selectedTopologyPlatform.value
  if (!platform) return
  emit('open-guests', {
    platformId: platform.id,
    clusterId: selectedTopologyCluster.value?.id || undefined
  })
}

const showCurrentTrend = () => {
  const platform = selectedTopologyPlatform.value
  if (!platform) return
  if (props.platformId !== platform.id) emit('update:platformId', platform.id)
  if (selectedTopologyCluster.value) {
    localTrendScopeType.value = 'cluster'
    localTrendClusterId.value = selectedTopologyCluster.value.id
    return
  }
  localTrendScopeType.value = 'platform'
}

const setActiveObjectTab = (tab: TopologyTab) => {
  activeObjectTab.value = tab
  if (tab === 'overview' || tab === 'monitor') {
    showCurrentTrend()
  }
}

const handleRangeChange = (value: string | number | boolean) => {
  emit('change-trend-range', value as TrendRange)
}

const topologyProviderLabel = (provider?: string) => {
  if (provider === 'pve') return 'Proxmox VE'
  return 'VMware vSphere'
}

const hostStatusText = (status?: string) => {
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  if (status === 'maintenance') return '维护'
  return '未知'
}

const hostStatusType = (status?: string) => {
  if (status === 'online') return 'success'
  if (status === 'offline') return 'danger'
  if (status === 'maintenance') return 'warning'
  return 'info'
}

const powerStateText = (status?: string) => {
  if (status === 'powered_on') return '运行中'
  if (status === 'powered_off') return '已停止'
  if (status === 'suspended') return '挂起'
  return '未知'
}

const powerStateType = (status?: string) => {
  if (status === 'powered_on') return 'success'
  if (status === 'powered_off') return 'info'
  if (status === 'suspended') return 'warning'
  return 'info'
}

const bindingStatusText = (status?: string) => {
  if (status === 'bound') return '已纳管'
  if (status === 'conflict') return '冲突'
  return '未纳管'
}

const bindingStatusType = (status?: string) => {
  if (status === 'bound') return 'success'
  if (status === 'conflict') return 'danger'
  return 'info'
}

const syncStatusType = (status: string) => {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'info'
}

const syncStatusText = (status: string) => {
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'running') return '运行中'
  if (status === 'pending') return '等待'
  return status || '未知'
}

const syncTriggerText = (trigger: string) => {
  if (trigger === 'schedule') return '自动同步'
  if (trigger === 'operation') return '操作触发'
  if (trigger === 'event') return '事件触发'
  return '手动同步'
}

const guestResourceId = (externalId?: string) => {
  const parts = String(externalId || '').split('/')
  return parts.length === 2 ? parts[1] : externalId || '-'
}

const guestTypeLabel = (externalId?: string) => {
  if (String(externalId || '').startsWith('qemu/')) return 'QEMU'
  if (String(externalId || '').startsWith('lxc/')) return 'LXC'
  return 'VM'
}

const guestTreeLabel = (guest: TopologyGuestNode) => {
  const id = guestResourceId(guest.externalId)
  return id && id !== '-' ? `${id} (${guest.name})` : guest.name
}

const percentage = (used: number, total: number) => {
  if (!total || total <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((used / total) * 100)))
}

const formatPercent = (value: number) => `${Math.max(0, Math.min(100, value)).toFixed(0)}%`

const formatMb = (value?: number) => {
  const mb = Number(value || 0)
  if (mb <= 0) return '0 MiB'
  if (mb >= 1024 * 1024) return `${(mb / 1024 / 1024).toFixed(2)} TiB`
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GiB`
  return `${mb} MiB`
}

const formatMemoryPair = (used?: number, total?: number) => {
  const totalValue = Number(total || 0)
  const usedValue = Number(used || 0)
  if (totalValue <= 0) return usedValue > 0 ? formatMb(usedValue) : '-'
  return `${formatMb(usedValue)} / ${formatMb(totalValue)}`
}

const formatTrendPointLabel = (point: TrendPoint) => {
  const date = new Date(point.timestamp * 1000)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hour = `${date.getHours()}`.padStart(2, '0')
  const minute = `${date.getMinutes()}`.padStart(2, '0')
  if (props.trendRange === '24h') return `${hour}:${minute}`
  return `${month}-${day} ${hour}:${minute}`
}

const setChartRef = (key: string) => (el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) {
    chartRefs.set(key, el)
  } else {
    chartRefs.delete(key)
  }
}

const renderCharts = async () => {
  await nextTick()
  const activeKeys = new Set(chartPanels.value.map((panel) => panel.key))
  charts.forEach((chart, key) => {
    if (!activeKeys.has(key) || !chartRefs.has(key) || !hasTrendData.value) {
      chart.dispose()
      charts.delete(key)
    }
  })

  if (!hasTrendData.value) return

  chartPanels.value.forEach((panel) => {
    const el = chartRefs.get(panel.key)
    if (!el) return

    let chart = charts.get(panel.key)
    if (!chart) {
      chart = echarts.init(el)
      charts.set(panel.key, chart)
    }

    chart.setOption({
      color: panel.series.map((item) => item.color),
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(38, 38, 38, 0.94)',
        borderWidth: 0,
        textStyle: { color: '#fff' },
        formatter(params: any[]) {
          const point = trendPoints.value[params?.[0]?.dataIndex || 0]
          const header = point?.time || '-'
          const lines = params.map((item) => `${item.marker}${item.seriesName}: ${item.value} ${panel.unit}`)
          return [header, ...lines].join('<br/>')
        }
      },
      legend: { show: false },
      grid: {
        left: 42,
        right: 12,
        top: 12,
        bottom: 28
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: chartLabels.value,
        axisLabel: {
          color: '#555',
          fontSize: 11,
          hideOverlap: true
        },
        axisLine: { lineStyle: { color: '#cfcfcf' } },
        axisTick: { show: false },
        splitLine: { show: true, lineStyle: { color: '#e7e7e7' } }
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
        axisLabel: {
          color: '#555',
          fontSize: 11
        },
        axisLine: { show: true, lineStyle: { color: '#cfcfcf' } },
        axisTick: { show: false },
        splitLine: { show: true, lineStyle: { color: '#e7e7e7' } }
      },
      series: panel.series.map((item) => ({
        name: item.name,
        type: 'line',
        data: item.data,
        smooth: false,
        symbol: 'circle',
        symbolSize: 4,
        lineStyle: { width: 1.4 },
        areaStyle: item.area
          ? {
              opacity: 0.55
            }
          : undefined
      }))
    }, true)
  })
}

const resizeCharts = () => {
  charts.forEach((chart) => chart.resize())
}

watch(() => props.platforms, ensureTopologySelection, { deep: true, immediate: true, flush: 'post' })
watch(() => selectedTopologyPlatform.value?.id, (id) => {
  if (id) emit('load-sync-jobs', id)
})
watch(() => props.trend, renderCharts, { deep: true, flush: 'post' })
watch(() => props.trendRange, renderCharts, { flush: 'post' })
watch(hasTrendData, renderCharts, { flush: 'post' })
watch(activeObjectTab, renderCharts, { flush: 'post' })

onMounted(() => {
  window.addEventListener('resize', resizeCharts)
  renderCharts()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts)
  charts.forEach((chart) => chart.dispose())
  charts.clear()
  chartRefs.clear()
})
</script>

<style scoped>
.pve-topology {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 760px;
  color: #262626;
}

.pve-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid #cfcfcf;
  background: linear-gradient(#f7f7f7, #e8e8e8);
}

.toolbar-left,
.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.view-mode-select {
  width: 150px;
}

.platform-select {
  width: 220px;
}

.tree-search {
  width: 240px;
}

.live-sync {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #4b5563;
  font-size: 12px;
  white-space: nowrap;
}

.live-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #7aa80f;
  box-shadow: 0 0 0 3px rgba(122, 168, 15, 0.18);
}

.freshness-text {
  max-width: 180px;
  overflow: hidden;
  color: #6b7280;
  text-overflow: ellipsis;
}

.black-button {
  background: #111827;
  border-color: #111827;
  color: #fff;
}

.black-button:hover {
  background: #1f2937;
  border-color: #1f2937;
  color: #fff;
}

.reset-btn {
  color: #374151;
  background: #fff;
  border-color: #cfd4dc;
}

.pve-console {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  min-height: 720px;
  border: 1px solid #bfc4cb;
  background: #d7d7d7;
}

.resource-tree {
  min-width: 0;
  border-right: 1px solid #b8bec7;
  background: #f4f4f4;
}

.tree-header {
  padding: 10px;
  border-bottom: 1px solid #d5d8dd;
  background: #ededed;
}

.tree-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tree-title {
  font-size: 13px;
  font-weight: 700;
}

.tree-subtitle {
  margin-top: 4px;
  color: #6b7280;
  font-size: 12px;
}

.tree-scroll {
  height: 668px;
  overflow: auto;
  padding: 4px 0 12px;
}

.tree-platform {
  margin: 0;
}

.tree-branch {
  margin: 0;
}

.tree-row {
  width: 100%;
  min-height: 26px;
  display: grid;
  grid-template-columns: 14px 16px minmax(0, 1fr) auto;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border: 0;
  border-left: 3px solid transparent;
  background: transparent;
  color: #1f2937;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.tree-row:hover {
  background: #e7f1fb;
}

.tree-row.active {
  background: #cfe7fb;
  border-left-color: #1d75bd;
}

.tree-row-platform {
  font-weight: 700;
}

.tree-row-cluster {
  padding-left: 20px;
}

.tree-row-host {
  grid-template-columns: 14px 16px minmax(0, 1fr) auto;
  padding-left: 34px;
}

.tree-row-guest {
  grid-template-columns: 14px 16px minmax(0, 1fr) auto;
  padding-left: 52px;
}

.tree-caret {
  color: #6b7280;
  font-size: 11px;
}

.tree-name,
.tree-muted {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.tree-muted {
  color: #6b7280;
  font-size: 11px;
}

.tree-count {
  min-width: 20px;
  padding: 1px 5px;
  border-radius: 8px;
  background: #e0e4e8;
  color: #4b5563;
  font-size: 11px;
  text-align: center;
}

.tree-status,
.guest-state {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  justify-self: center;
}

.status-online,
.power-powered_on {
  background: #1aa053;
}

.status-offline,
.power-powered_off {
  background: #9aa0a6;
}

.status-maintenance,
.power-suspended {
  background: #d39a22;
}

.status-unknown,
.power-unknown {
  background: #6b7280;
}

.server-view {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px;
  overflow: hidden;
}

.object-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #cfcfcf;
  background: #f7f7f7;
}

.object-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.object-icon {
  color: #1d75bd;
  font-size: 22px;
  flex: 0 0 auto;
}

.object-title-row h3 {
  margin: 0;
  color: #1d75bd;
  font-size: 15px;
  font-weight: 600;
}

.object-title-row p {
  margin: 3px 0 0;
  color: #606266;
  font-size: 12px;
}

.object-path {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 5px;
  color: #6b7280;
  font-size: 11px;
}

.object-path span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.object-path span + span::before {
  color: #9aa0a6;
  content: '/';
}

.object-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.object-tabs {
  display: flex;
  align-items: center;
  gap: 0;
  border: 1px solid #cfcfcf;
  background: #eeeeee;
}

.tab-button {
  height: 30px;
  min-width: 78px;
  padding: 0 14px;
  border: 0;
  border-right: 1px solid #cfcfcf;
  background: transparent;
  color: #374151;
  font-size: 12px;
  cursor: pointer;
}

.tab-button.active {
  background: #fff;
  color: #1d75bd;
  font-weight: 700;
}

.summary-grid {
  display: grid;
  grid-template-columns: minmax(360px, 0.95fr) minmax(430px, 1.35fr);
  gap: 8px;
}

.pve-card {
  min-width: 0;
  border: 1px solid #cfcfcf;
  background: #fff;
}

.card-header,
.chart-header,
.task-header {
  min-height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 9px;
  border-bottom: 1px solid #d7dbe0;
  background: #f8f8f8;
}

.card-header h4,
.task-header h4 {
  margin: 0;
  color: #1d75bd;
  font-size: 13px;
  font-weight: 600;
}

.card-header span,
.task-header span {
  color: #6b7280;
  font-size: 12px;
}

.meter-list {
  padding: 10px 12px 2px;
}

.meter-row {
  margin-bottom: 10px;
}

.meter-label {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  color: #1f2937;
  font-size: 12px;
}

.meter-label strong {
  font-weight: 600;
}

.meter-track {
  height: 8px;
  margin-top: 5px;
  border: 1px solid #cad0d7;
  background: #edf1f5;
}

.meter-track span {
  display: block;
  height: 100%;
  background: #b6d36c;
}

.meter-help {
  margin-top: 3px;
  color: #6b7280;
  font-size: 11px;
}

.detail-table {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
  padding: 8px 12px 12px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  min-height: 28px;
  padding: 6px 8px;
  border-bottom: 1px solid #eef0f2;
  color: #4b5563;
  font-size: 12px;
}

.detail-row strong {
  min-width: 0;
  overflow: hidden;
  color: #1f2937;
  font-weight: 600;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-table {
  padding: 8px;
  overflow: auto;
}

.compact-table-head,
.compact-table-row {
  display: grid;
  align-items: center;
  gap: 8px;
  min-width: 640px;
  min-height: 32px;
  padding: 0 8px;
  border: 0;
  border-bottom: 1px solid #e5e7eb;
  background: #fff;
  color: #374151;
  font-size: 12px;
  text-align: left;
}

.compact-table-head {
  background: #f4f5f6;
  color: #6b7280;
  font-weight: 700;
}

.compact-table-row {
  width: 100%;
  cursor: pointer;
}

.compact-table-row:hover,
.compact-table-row.active {
  background: #edf6ff;
}

.guest-grid {
  grid-template-columns: 70px minmax(130px, 1fr) 82px 135px minmax(100px, 0.8fr) 82px;
}

.host-grid {
  grid-template-columns: minmax(140px, 1fr) 90px 90px minmax(140px, 1fr) 70px;
}

.resource-list-wide {
  min-height: 410px;
}

.monitor-summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.monitor-summary-card {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #cfcfcf;
  background: #fff;
}

.monitor-summary-card span,
.monitor-summary-card small {
  display: block;
  overflow: hidden;
  color: #6b7280;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-summary-card strong {
  display: block;
  margin: 6px 0 3px;
  color: #111827;
  font-size: 22px;
  line-height: 1.1;
}

.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.chart-card {
  min-height: 244px;
}

.chart-header a {
  color: #1d75bd;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
}

.chart-current {
  display: block;
  margin-top: 3px;
  color: #374151;
  font-size: 12px;
  font-weight: 600;
}

.chart-legend {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
  color: #4b5563;
  font-size: 11px;
}

.chart-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.chart-legend i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.metric-chart {
  width: 100%;
  height: 198px;
}

.chart-empty {
  height: 198px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.chart-grid-docked .chart-card {
  min-height: 224px;
}

.chart-grid-docked .metric-chart,
.chart-grid-docked .chart-empty {
  height: 178px;
}

.task-panel {
  border: 1px solid #cfcfcf;
  background: #fff;
}

.task-panel-main {
  min-height: 410px;
}

.task-panel-docked.collapsed .task-header {
  border-bottom: 0;
}

.task-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-table {
  min-height: 94px;
  overflow: auto;
}

.task-grid {
  display: grid;
  grid-template-columns: 170px 90px 90px 150px minmax(180px, 1fr);
  align-items: center;
  gap: 8px;
  min-width: 780px;
  min-height: 30px;
  padding: 0 10px;
  border-bottom: 1px solid #e5e7eb;
  font-size: 12px;
}

.task-table-head {
  background: #f4f5f6;
  color: #6b7280;
  font-weight: 700;
}

.task-row {
  color: #374151;
}

.task-row .error {
  color: #b42318;
}

@media (max-width: 1280px) {
  .pve-console {
    grid-template-columns: 250px minmax(0, 1fr);
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 980px) {
  .pve-toolbar,
  .object-header {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-left,
  .toolbar-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .view-mode-select,
  .platform-select,
  .tree-search {
    width: 100%;
  }

  .pve-console {
    grid-template-columns: 1fr;
  }

  .resource-tree {
    border-right: 0;
    border-bottom: 1px solid #b8bec7;
  }

  .tree-scroll {
    height: 320px;
  }

  .chart-grid {
    grid-template-columns: 1fr;
  }

  .monitor-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-table {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .monitor-summary-grid {
    grid-template-columns: 1fr;
  }

  .task-actions {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
</style>
