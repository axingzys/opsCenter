<template>
  <div class="topology-explorer">
    <div class="topology-toolbar">
      <div class="topology-toolbar-left">
        <el-select
          v-model="selectedPlatformId"
          placeholder="选择平台"
          clearable
          class="topology-platform-select"
        >
          <el-option v-for="item in platformOptions" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </div>
      <div class="topology-toolbar-actions">
        <div class="topology-sync-indicator">
          <span class="topology-live-dot" />
          后台自动同步
        </div>
        <el-button class="black-button" @click="emit('refresh')" :loading="loading">
          <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
          刷新拓扑
        </el-button>
      </div>
    </div>

    <div class="topology-panel" v-loading="loading">
      <el-empty v-if="platforms.length === 0" description="暂无拓扑数据，请先执行平台同步" />
      <template v-else>
        <div class="topology-workspace">
          <aside class="topology-resource-tree">
            <div class="resource-tree-header">
              <div>
                <div class="resource-tree-title">资源树</div>
                <div class="resource-tree-subtitle">PVE 按节点组织，ESXi/vCenter 按 vSphere 层级组织</div>
              </div>
              <el-tag size="small" effect="plain">{{ topologyStats.guestCount }} VM</el-tag>
            </div>
            <div class="resource-tree-list">
              <section
                v-for="platform in platforms"
                :key="`tree-platform-${platform.id}`"
                class="resource-tree-platform"
              >
                <button
                  type="button"
                  class="resource-tree-node resource-tree-node-platform"
                  :class="{ 'resource-tree-node-active': isTopologyNodeActive('platform', platform.id) }"
                  @click="selectTopologyPlatform(platform)"
                >
                  <span class="resource-node-chevron">▾</span>
                  <span class="resource-node-kind">{{ topologyProviderShortLabel(platform.provider) }}</span>
                  <span class="resource-node-name">{{ platform.name }}</span>
                  <span class="resource-node-count">{{ platformGuestCount(platform) }}</span>
                </button>
                <div
                  v-for="cluster in platform.clusters || []"
                  :key="`tree-cluster-${platform.id}-${cluster.id}`"
                  class="resource-tree-branch"
                >
                  <button
                    type="button"
                    class="resource-tree-node resource-tree-node-cluster"
                    :class="{ 'resource-tree-node-active': isTopologyNodeActive('cluster', platform.id, cluster.id) }"
                    @click="selectTopologyCluster(platform, cluster)"
                  >
                    <span class="resource-node-chevron">▾</span>
                    <span class="resource-node-kind">{{ platform.provider === 'pve' ? 'NODES' : 'CLUSTER' }}</span>
                    <span class="resource-node-name">{{ cluster.name }}</span>
                    <span class="resource-node-count">{{ cluster.guestCount || 0 }}</span>
                  </button>
                  <button
                    v-for="host in cluster.hosts || []"
                    :key="`tree-host-${platform.id}-${cluster.id}-${host.id}`"
                    type="button"
                    class="resource-tree-node resource-tree-node-host"
                    :class="{ 'resource-tree-node-active': isTopologyNodeActive('host', platform.id, cluster.id, host.id) }"
                    @click="selectTopologyHost(platform, cluster, host)"
                  >
                    <span class="resource-status-dot" :class="`status-${host.status || 'unknown'}`" />
                    <span class="resource-node-kind">{{ platform.provider === 'pve' ? 'NODE' : 'HOST' }}</span>
                    <span class="resource-node-name">{{ host.name }}</span>
                    <span class="resource-node-count">{{ host.guestCount || 0 }}</span>
                  </button>
                </div>
              </section>
            </div>
          </aside>

          <main class="topology-main">
            <div class="topology-summary">
              <div class="summary-card">
                <div class="summary-label">平台</div>
                <div class="summary-value">{{ topologyStats.platformCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">集群</div>
                <div class="summary-value">{{ topologyStats.clusterCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">宿主机</div>
                <div class="summary-value">{{ topologyStats.hostCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">虚机</div>
                <div class="summary-value">{{ topologyStats.guestCount }}</div>
              </div>
            </div>

            <div class="trend-scope-toolbar">
              <div class="trend-scope-left">
                <div class="trend-scope-label">趋势视角</div>
                <el-radio-group v-model="localTrendScopeType" size="small">
                  <el-radio-button label="platform">平台</el-radio-button>
                  <el-radio-button label="cluster">集群</el-radio-button>
                </el-radio-group>
              </div>
              <div class="trend-scope-right">
                <el-select
                  v-if="localTrendScopeType === 'cluster'"
                  v-model="localTrendClusterId"
                  placeholder="选择集群"
                  clearable
                  class="trend-cluster-select"
                >
                  <el-option
                    v-for="item in trendClusterOptions"
                    :key="item.id"
                    :value="item.id"
                    :label="`${item.name} · ${item.guestCount || 0} 虚机`"
                  />
                </el-select>
              </div>
            </div>

            <VirtualizationPlatformTrendPanel
              v-if="currentTrendPlatformId"
              class="topology-trend-panel"
              :title="trendPanelTitle"
              :trend="trend"
              :loading="trendLoading"
              :range="trendRange"
              :selected-metrics="selectedTrendMetrics"
              :metric-options="trendMetricOptions"
              @change-range="(range) => emit('change-trend-range', range)"
              @change-metrics="(values) => emit('change-trend-metrics', values)"
            />

            <div class="topology-provider-list">
              <section v-for="platform in platforms" :key="platform.id" class="topology-provider-section">
                <header class="topology-provider-header">
                  <div>
                    <div class="topology-provider-title">{{ platform.name }}</div>
                    <div class="topology-provider-meta">
                      <el-tag :type="platform.provider === 'pve' ? 'warning' : 'success'" size="small">
                        {{ platform.providerText || platform.provider }}
                      </el-tag>
                      <span>{{ topologyHierarchyLabel(platform.provider) }}</span>
                    </div>
                  </div>
                  <div class="platform-sync-time">
                    最近同步：{{ platform.lastSyncAt || '-' }}
                  </div>
                </header>

                <div class="cluster-lane-list">
                  <section
                    v-for="cluster in platform.clusters || []"
                    :key="`${platform.id}-${cluster.id}-${cluster.name}`"
                    class="cluster-lane"
                    :class="{ 'cluster-lane-active': isTopologyNodeActive('cluster', platform.id, cluster.id) }"
                    @click="selectTopologyCluster(platform, cluster)"
                  >
                    <div class="cluster-lane-header">
                      <div>
                        <div class="cluster-title">{{ cluster.name }}</div>
                        <div class="cluster-subtitle">{{ cluster.datacenter || topologyProviderLabel(platform.provider) }}</div>
                      </div>
                      <div class="cluster-metrics">
                        <span>{{ cluster.hostCount || 0 }} 宿主机</span>
                        <span>{{ cluster.guestCount || 0 }} 虚机</span>
                      </div>
                    </div>
                    <div class="host-tile-grid">
                      <div v-if="!cluster.hosts || cluster.hosts.length === 0" class="empty-host">暂无宿主机</div>
                      <button
                        v-for="host in cluster.hosts || []"
                        :key="host.id"
                        type="button"
                        class="host-tile"
                        :class="[
                          `host-${host.status || 'unknown'}`,
                          { 'host-tile-active': isTopologyNodeActive('host', platform.id, cluster.id, host.id) }
                        ]"
                        @click.stop="selectTopologyHost(platform, cluster, host)"
                      >
                        <div class="host-tile-top">
                          <span class="host-status-pill">
                            <span class="host-dot" />
                            {{ hostStatusText(host.status) }}
                          </span>
                          <span class="host-vm-count">{{ host.guestCount || 0 }} VM</span>
                        </div>
                        <div class="host-tile-name">{{ host.name }}</div>
                        <div class="host-tile-ip">{{ host.managementIp || '-' }}</div>
                        <div class="host-density-bar">
                          <span :style="{ width: `${hostGuestDensity(host, platform)}%` }" />
                        </div>
                        <div class="host-vm-dots">
                          <span v-for="dot in hostVmDots(host)" :key="dot" class="vm-dot" />
                          <span v-if="hostVmOverflow(host) > 0" class="vm-dot-overflow">+{{ hostVmOverflow(host) }}</span>
                        </div>
                      </button>
                    </div>
                  </section>
                </div>
              </section>
            </div>

            <section class="topology-task-panel">
              <header class="task-panel-header">
                <div>
                  <div class="task-panel-title">最近同步任务</div>
                  <div class="task-panel-subtitle">{{ selectedTopologyPlatform?.name || '当前平台' }}</div>
                </div>
                <el-button class="reset-btn" size="small" :loading="syncJobsLoading" @click="emit('load-sync-jobs', selectedTopologyPlatform?.id)">
                  <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
                  刷新任务
                </el-button>
              </header>
              <div class="sync-task-list" v-loading="syncJobsLoading">
                <el-empty v-if="syncJobs.length === 0" description="暂无同步任务记录" :image-size="64" />
                <div v-for="job in syncJobs.slice(0, 6)" :key="job.id" class="sync-task-row">
                  <div class="sync-task-main">
                    <el-tag size="small" :type="syncStatusType(job.status)">
                      {{ syncStatusText(job.status) }}
                    </el-tag>
                    <span class="sync-task-trigger">{{ syncTriggerText(job.triggerType) }}</span>
                    <span class="sync-task-time">{{ job.startedAt || job.createTime || '-' }}</span>
                  </div>
                  <div class="sync-task-meta">
                    <span>条目 {{ job.itemsTotal || 0 }}</span>
                    <span v-if="job.finishedAt">完成 {{ job.finishedAt }}</span>
                    <span v-if="job.failureReason" class="sync-task-error">{{ job.failureReason }}</span>
                  </div>
                </div>
              </div>
            </section>
          </main>

          <aside class="topology-detail-panel">
            <div class="detail-panel-header">
              <div>
                <div class="detail-panel-title">{{ topologyDetailTitle }}</div>
                <div class="detail-panel-subtitle">{{ topologyDetailSubtitle }}</div>
              </div>
            </div>

            <template v-if="selectedTopologyHost">
              <div class="detail-kv-list">
                <div class="detail-kv">
                  <span>宿主机</span>
                  <strong>{{ selectedTopologyHost.name }}</strong>
                </div>
                <div class="detail-kv">
                  <span>管理 IP</span>
                  <strong>{{ selectedTopologyHost.managementIp || '-' }}</strong>
                </div>
                <div class="detail-kv">
                  <span>状态</span>
                  <strong>{{ hostStatusText(selectedTopologyHost.status) }}</strong>
                </div>
                <div class="detail-kv">
                  <span>虚机数</span>
                  <strong>{{ selectedTopologyHost.guestCount || 0 }}</strong>
                </div>
              </div>
              <div class="detail-actions">
                <el-button class="black-button" @click="openGuestsFromTopology">
                  查看虚机
                </el-button>
                <el-button class="reset-btn" @click="showSelectedClusterTrend">
                  查看趋势
                </el-button>
              </div>
            </template>
            <template v-else-if="selectedTopologyCluster">
              <div class="detail-kv-list">
                <div class="detail-kv">
                  <span>集群</span>
                  <strong>{{ selectedTopologyCluster.name }}</strong>
                </div>
                <div class="detail-kv">
                  <span>数据中心</span>
                  <strong>{{ selectedTopologyCluster.datacenter || '-' }}</strong>
                </div>
                <div class="detail-kv">
                  <span>宿主机</span>
                  <strong>{{ selectedTopologyCluster.hostCount || 0 }}</strong>
                </div>
                <div class="detail-kv">
                  <span>虚机</span>
                  <strong>{{ selectedTopologyCluster.guestCount || 0 }}</strong>
                </div>
              </div>
              <div class="detail-actions">
                <el-button class="black-button" @click="openGuestsFromTopology">
                  查看虚机
                </el-button>
                <el-button class="reset-btn" @click="showSelectedClusterTrend">
                  查看趋势
                </el-button>
              </div>
            </template>
            <template v-else-if="selectedTopologyPlatform">
              <div class="detail-kv-list">
                <div class="detail-kv">
                  <span>平台</span>
                  <strong>{{ selectedTopologyPlatform.name }}</strong>
                </div>
                <div class="detail-kv">
                  <span>类型</span>
                  <strong>{{ topologyProviderLabel(selectedTopologyPlatform.provider) }}</strong>
                </div>
                <div class="detail-kv">
                  <span>集群</span>
                  <strong>{{ selectedTopologyPlatform.clusters?.length || 0 }}</strong>
                </div>
                <div class="detail-kv">
                  <span>虚机</span>
                  <strong>{{ platformGuestCount(selectedTopologyPlatform) }}</strong>
                </div>
              </div>
              <div class="detail-actions">
                <el-button class="black-button" @click="openGuestsFromTopology">
                  查看虚机
                </el-button>
                <el-button class="reset-btn" @click="emit('refresh')">
                  刷新
                </el-button>
              </div>
            </template>
          </aside>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import VirtualizationPlatformTrendPanel from './VirtualizationPlatformTrendPanel.vue'

type TopologyNodeType = 'platform' | 'cluster' | 'host'
type TrendRange = '24h' | '7d' | '15d'
type TrendScopeType = 'platform' | 'cluster'

interface TopologySelection {
  type: TopologyNodeType
  platformId: number
  clusterId?: number
  hostId?: number
}

const props = defineProps<{
  platformId: number | null
  platformOptions: any[]
  loading: boolean
  platforms: any[]
  currentTrendPlatformId: number | null
  trend: any
  trendLoading: boolean
  trendRange: TrendRange
  trendScopeType: TrendScopeType
  trendClusterId: number | null
  trendClusterOptions: any[]
  trendPanelTitle: string
  selectedTrendMetrics: string[]
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
  (e: 'change-trend-metrics', value: string[]): void
  (e: 'open-guests', value: { platformId: number; clusterId?: number }): void
  (e: 'load-sync-jobs', value?: number): void
}>()

const selectedTopologyNode = ref<TopologySelection | null>(null)

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
  const platforms = props.platforms || []
  let clusterCount = 0
  let hostCount = 0
  let guestCount = 0
  platforms.forEach((platform: any) => {
    const clusters = platform.clusters || []
    clusterCount += clusters.length
    clusters.forEach((cluster: any) => {
      hostCount += (cluster.hosts || []).length
      guestCount += Number(cluster.guestCount || 0)
    })
  })
  return {
    platformCount: platforms.length,
    clusterCount,
    hostCount,
    guestCount
  }
})

const selectedTopologyPlatform = computed<any | null>(() => {
  const selection = selectedTopologyNode.value
  if (!selection) return props.platforms[0] || null
  return props.platforms.find((item: any) => item.id === selection.platformId) || null
})

const selectedTopologyCluster = computed<any | null>(() => {
  const selection = selectedTopologyNode.value
  const platform = selectedTopologyPlatform.value
  if (!selection || !platform || !selection.clusterId) return null
  return (platform.clusters || []).find((item: any) => item.id === selection.clusterId) || null
})

const selectedTopologyHost = computed<any | null>(() => {
  const selection = selectedTopologyNode.value
  const cluster = selectedTopologyCluster.value
  if (!selection || !cluster || !selection.hostId) return null
  return (cluster.hosts || []).find((item: any) => item.id === selection.hostId) || null
})

const topologyDetailTitle = computed(() => {
  if (selectedTopologyHost.value) return selectedTopologyHost.value.name
  if (selectedTopologyCluster.value) return selectedTopologyCluster.value.name
  if (selectedTopologyPlatform.value) return selectedTopologyPlatform.value.name
  return '拓扑详情'
})

const topologyDetailSubtitle = computed(() => {
  if (selectedTopologyHost.value) return '宿主机详情'
  if (selectedTopologyCluster.value) return selectedTopologyPlatform.value?.provider === 'pve' ? 'PVE 节点组' : 'vSphere Cluster'
  if (selectedTopologyPlatform.value) return topologyHierarchyLabel(selectedTopologyPlatform.value.provider)
  return '选择左侧资源查看详情'
})

const ensureTopologySelection = () => {
  if (isCurrentTopologySelectionValid()) return
  const platform = props.platforms[0]
  if (!platform) {
    selectedTopologyNode.value = null
    return
  }
  selectedTopologyNode.value = {
    type: 'platform',
    platformId: platform.id
  }
}

const isCurrentTopologySelectionValid = () => {
  const selection = selectedTopologyNode.value
  if (!selection) return false
  const platform = props.platforms.find((item: any) => item.id === selection.platformId)
  if (!platform) return false
  if (selection.type === 'platform') return true
  const cluster = (platform.clusters || []).find((item: any) => item.id === selection.clusterId)
  if (!cluster) return false
  if (selection.type === 'cluster') return true
  return (cluster.hosts || []).some((item: any) => item.id === selection.hostId)
}

const selectTopologyPlatform = (platform: any) => {
  selectedTopologyNode.value = {
    type: 'platform',
    platformId: platform.id
  }
  if (props.platformId !== platform.id) {
    emit('update:platformId', platform.id)
  }
  emit('update:trendScopeType', 'platform')
}

const selectTopologyCluster = (platform: any, cluster: any) => {
  selectedTopologyNode.value = {
    type: 'cluster',
    platformId: platform.id,
    clusterId: cluster.id
  }
  if (props.platformId !== platform.id) {
    emit('update:platformId', platform.id)
  }
  emit('update:trendScopeType', 'cluster')
  emit('update:trendClusterId', cluster.id)
}

const selectTopologyHost = (platform: any, cluster: any, host: any) => {
  selectedTopologyNode.value = {
    type: 'host',
    platformId: platform.id,
    clusterId: cluster.id,
    hostId: host.id
  }
  if (props.platformId !== platform.id) {
    emit('update:platformId', platform.id)
  }
  emit('update:trendScopeType', 'cluster')
  emit('update:trendClusterId', cluster.id)
}

const isTopologyNodeActive = (type: TopologyNodeType, platformId: number, clusterId?: number, hostId?: number) => {
  const selection = selectedTopologyNode.value
  if (!selection || selection.type !== type || selection.platformId !== platformId) return false
  if (type === 'platform') return true
  if (selection.clusterId !== clusterId) return false
  if (type === 'cluster') return true
  return selection.hostId === hostId
}

const topologyProviderLabel = (provider: string) => {
  if (provider === 'pve') return 'Proxmox VE'
  return 'VMware vSphere'
}

const topologyProviderShortLabel = (provider: string) => {
  if (provider === 'pve') return 'PVE'
  return 'ESXi'
}

const topologyHierarchyLabel = (provider: string) => {
  if (provider === 'pve') return 'Datacenter / Node / QEMU / LXC'
  return 'vCenter / Datacenter / Cluster / Host / VM'
}

const hostStatusText = (status?: string) => {
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  if (status === 'maintenance') return '维护'
  return '未知'
}

const platformGuestCount = (platform: any) => {
  return (platform?.clusters || []).reduce((total: number, cluster: any) => total + Number(cluster.guestCount || 0), 0)
}

const platformMaxHostGuestCount = (platform: any) => {
  const counts = (platform?.clusters || [])
    .flatMap((cluster: any) => cluster.hosts || [])
    .map((host: any) => Number(host.guestCount || 0))
  return Math.max(1, ...counts)
}

const hostGuestDensity = (host: any, platform: any) => {
  return Math.min(100, Math.round((Number(host?.guestCount || 0) / platformMaxHostGuestCount(platform)) * 100))
}

const hostVmDots = (host: any) => {
  return Array.from({ length: Math.min(18, Number(host?.guestCount || 0)) }, (_, index) => index + 1)
}

const hostVmOverflow = (host: any) => {
  return Math.max(0, Number(host?.guestCount || 0) - 18)
}

const openGuestsFromTopology = () => {
  const platform = selectedTopologyPlatform.value
  if (!platform) return
  emit('open-guests', {
    platformId: platform.id,
    clusterId: selectedTopologyCluster.value?.id || undefined
  })
}

const showSelectedClusterTrend = () => {
  const platform = selectedTopologyPlatform.value
  const cluster = selectedTopologyCluster.value
  if (!platform || !cluster) return
  if (props.platformId !== platform.id) {
    emit('update:platformId', platform.id)
  }
  emit('update:trendScopeType', 'cluster')
  emit('update:trendClusterId', cluster.id)
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
  if (trigger === 'manual') return '手动同步'
  if (trigger === 'operation') return '操作触发'
  if (trigger === 'event') return '事件触发'
  return trigger || '-'
}

watch(
  () => props.platforms,
  () => {
    ensureTopologySelection()
  },
  { immediate: true, deep: true }
)
</script>

<style scoped>
.topology-explorer {
  display: grid;
  gap: 12px;
}

.black-button {
  background: #111827;
  color: #fff;
  border: 1px solid #111827;
}

.black-button:hover {
  background: #1f2937;
  border-color: #1f2937;
  color: #fff;
}

.reset-btn {
  background: #f3f4f6;
  border-color: #e5e7eb;
}

.topology-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.topology-toolbar-left,
.topology-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.topology-platform-select {
  width: 280px;
}

.topology-panel {
  background: #f7f9fb;
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  min-height: 520px;
  padding: 12px;
}

.topology-sync-indicator {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: #3f4b5f;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.topology-live-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #18a058;
  box-shadow: 0 0 0 3px rgba(24, 160, 88, 0.14);
}

.topology-workspace {
  display: grid;
  grid-template-columns: minmax(230px, 270px) minmax(0, 1fr) minmax(280px, 320px);
  gap: 12px;
  align-items: start;
}

.topology-resource-tree,
.topology-detail-panel {
  min-height: 496px;
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  background: #ffffff;
}

.topology-resource-tree {
  overflow: hidden;
}

.resource-tree-header,
.detail-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border-bottom: 1px solid #edf1f6;
}

.resource-tree-title,
.detail-panel-title {
  color: #111827;
  font-size: 14px;
  font-weight: 700;
}

.resource-tree-subtitle,
.detail-panel-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

.resource-tree-list {
  max-height: 680px;
  overflow: auto;
  padding: 8px;
}

.resource-tree-platform + .resource-tree-platform {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #edf1f6;
}

.resource-tree-branch {
  margin-top: 4px;
}

.resource-tree-node {
  width: 100%;
  min-height: 32px;
  display: grid;
  grid-template-columns: 14px 54px minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  border: 0;
  border-radius: 6px;
  padding: 5px 8px;
  background: transparent;
  color: #334155;
  cursor: pointer;
  text-align: left;
}

.resource-tree-node:hover {
  background: #f3f6fa;
}

.resource-tree-node-active {
  background: #e8f1ff;
  color: #0f4c9c;
}

.resource-tree-node-cluster {
  padding-left: 18px;
}

.resource-tree-node-host {
  grid-template-columns: 14px 54px minmax(0, 1fr) auto;
  padding-left: 34px;
}

.resource-node-chevron {
  color: #94a3b8;
  font-size: 10px;
}

.resource-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  justify-self: center;
  background: #94a3b8;
}

.resource-node-kind {
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
}

.resource-node-name {
  overflow: hidden;
  color: inherit;
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-node-count {
  min-width: 22px;
  border-radius: 999px;
  padding: 1px 6px;
  background: #eef2f7;
  color: #475569;
  font-size: 11px;
  text-align: center;
}

.topology-main {
  min-width: 0;
}

.topology-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.topology-trend-panel {
  margin-bottom: 12px;
}

.trend-scope-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #dfe5ee;
  background: #ffffff;
}

.trend-scope-left,
.trend-scope-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.trend-scope-label {
  color: #475569;
  font-size: 12px;
  font-weight: 600;
}

.trend-cluster-select {
  width: 260px;
}

.summary-card {
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  padding: 10px 12px;
  background: #fff;
}

.summary-label {
  color: #64748b;
  font-size: 12px;
}

.summary-value {
  margin-top: 6px;
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}

.topology-provider-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.topology-provider-section,
.topology-task-panel {
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  background: #ffffff;
  padding: 12px;
}

.topology-provider-header,
.task-panel-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.topology-provider-title,
.task-panel-title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.topology-provider-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
}

.task-panel-subtitle,
.platform-sync-time {
  color: #64748b;
  font-size: 12px;
  margin-top: 3px;
}

.cluster-lane-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.cluster-lane {
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  padding: 12px;
  background: #fbfcfe;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.cluster-lane:hover {
  border-color: #b8c4d6;
}

.cluster-lane-active {
  border-color: #2f6fed;
  box-shadow: 0 0 0 3px rgba(47, 111, 237, 0.1);
}

.cluster-lane-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 10px;
}

.cluster-title {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
}

.cluster-subtitle {
  margin-top: 3px;
  color: #64748b;
  font-size: 12px;
}

.cluster-metrics {
  display: flex;
  gap: 8px;
  color: #64748b;
  font-size: 12px;
}

.host-tile-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(188px, 1fr));
  gap: 10px;
}

.empty-host {
  color: #94a3b8;
  font-size: 12px;
}

.host-tile {
  min-height: 132px;
  border: 1px solid #dfe5ee;
  border-radius: 8px;
  padding: 10px;
  background: #fff;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.host-tile:hover {
  border-color: #b8c4d6;
  transform: translateY(-1px);
}

.host-tile-active {
  border-color: #2f6fed;
  box-shadow: 0 0 0 3px rgba(47, 111, 237, 0.1);
}

.host-tile-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.host-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #475569;
  font-size: 12px;
  font-weight: 600;
}

.host-vm-count {
  color: #334155;
  font-size: 12px;
  font-weight: 700;
}

.host-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #94a3b8;
}

.host-tile-name {
  overflow: hidden;
  color: #0f172a;
  font-size: 13px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-tile-ip {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.host-density-bar {
  height: 5px;
  margin-top: 10px;
  overflow: hidden;
  border-radius: 999px;
  background: #edf1f6;
}

.host-density-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #2f6fed;
}

.host-vm-dots {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  min-height: 20px;
  margin-top: 10px;
}

.vm-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #94a3b8;
}

.vm-dot-overflow {
  color: #64748b;
  font-size: 11px;
  line-height: 1;
}

.host-online .host-dot,
.status-online {
  background: #16a34a;
}

.host-maintenance .host-dot,
.status-maintenance {
  background: #d97706;
}

.host-offline .host-dot,
.host-unknown .host-dot,
.status-offline,
.status-unknown {
  background: #9ca3af;
}

.host-offline .host-density-bar span,
.host-unknown .host-density-bar span {
  background: #94a3b8;
}

.host-maintenance .host-density-bar span {
  background: #d97706;
}

.topology-task-panel {
  margin-top: 12px;
}

.sync-task-list {
  min-height: 86px;
}

.sync-task-row {
  display: grid;
  gap: 5px;
  padding: 9px 0;
  border-top: 1px solid #edf1f6;
}

.sync-task-row:first-child {
  border-top: 0;
}

.sync-task-main,
.sync-task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sync-task-trigger {
  color: #111827;
  font-size: 12px;
  font-weight: 700;
}

.sync-task-time,
.sync-task-meta {
  color: #64748b;
  font-size: 12px;
}

.sync-task-error {
  max-width: 360px;
  overflow: hidden;
  color: #b91c1c;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topology-detail-panel {
  position: sticky;
  top: 12px;
  padding-bottom: 12px;
}

.detail-kv-list {
  display: grid;
  gap: 8px;
  padding: 12px;
}

.detail-kv {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid #edf1f6;
}

.detail-kv span {
  color: #64748b;
  font-size: 12px;
}

.detail-kv strong {
  color: #111827;
  font-size: 12px;
  text-align: right;
  word-break: break-word;
}

.detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 4px 12px 0;
}

@media (max-width: 900px) {
  .topology-platform-select,
  .trend-cluster-select {
    width: 100%;
  }

  .topology-workspace {
    grid-template-columns: 1fr;
  }

  .topology-resource-tree,
  .topology-detail-panel {
    min-height: auto;
  }

  .topology-detail-panel {
    position: static;
  }

  .topology-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .topology-provider-header,
  .cluster-lane-header,
  .task-panel-header {
    flex-direction: column;
  }
}
</style>
