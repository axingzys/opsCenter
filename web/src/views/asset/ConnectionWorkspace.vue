<template>
  <div class="connection-workspace">
    <div class="workspace-header">
      <div>
        <h2 class="workspace-title">连接工作台</h2>
        <p class="workspace-subtitle">SSH 终端与 Windows 桌面统一入口</p>
      </div>
      <div class="workspace-actions">
        <el-button @click="goHostManagement">
          <el-icon><Monitor /></el-icon>
          主机管理
        </el-button>
        <el-button class="black-button" :loading="loading" @click="loadHosts">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="workspace-filters">
      <el-input
        v-model="filters.keyword"
        placeholder="搜索主机、IP、分组或标签"
        clearable
        class="filter-search"
        @keyup.enter="loadHosts"
        @clear="loadHosts"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <el-select v-model="filters.osType" placeholder="系统" class="filter-select">
        <el-option label="全部系统" value="" />
        <el-option label="Linux" value="linux" />
        <el-option label="Windows" value="windows" />
      </el-select>

      <el-select v-model="filters.capability" placeholder="连接方式" class="filter-select">
        <el-option label="全部连接" value="" />
        <el-option label="SSH终端" value="ssh" />
        <el-option label="Windows桌面" value="rdp" />
        <el-option label="Agent可用" value="agent" />
        <el-option label="无可用连接" value="none" />
      </el-select>

      <el-select v-model="filters.status" placeholder="状态" class="filter-select" @change="loadHosts">
        <el-option label="全部状态" value="" />
        <el-option label="在线" value="1" />
        <el-option label="离线" value="0" />
        <el-option label="未知" value="-1" />
      </el-select>

      <el-button @click="resetFilters">重置</el-button>
    </div>

    <div class="workspace-layout">
      <aside class="workspace-sidebar">
        <div class="panel-title-row">
          <div class="panel-title">
            <el-icon><FolderOpened /></el-icon>
            <span>资产分组</span>
          </div>
        </div>
        <button
          class="all-groups-button"
          :class="{ active: selectedGroupId === null }"
          @click="selectAllGroups"
        >
          <span>全部分组</span>
          <span>{{ hostList.length }}</span>
        </button>
        <el-tree
          v-loading="groupLoading"
          :data="groupTree"
          :props="treeProps"
          node-key="id"
          :highlight-current="true"
          class="group-tree"
          @node-click="handleGroupSelect"
        >
          <template #default="{ data }">
            <div class="group-node">
              <span class="group-name">{{ data.name }}</span>
              <span class="group-count">
                {{ getGroupSummary(data).total }}
              </span>
            </div>
          </template>
        </el-tree>
      </aside>

      <main class="workspace-main">
        <el-tabs v-model="activeTab" class="connection-tabs">
          <el-tab-pane :label="`全部连接 ${tabCounts.all}`" name="all" />
          <el-tab-pane :label="`SSH终端 ${tabCounts.ssh}`" name="ssh" />
          <el-tab-pane :label="`Windows桌面 ${tabCounts.rdp}`" name="desktop" />
          <el-tab-pane :label="`最近连接 ${recentConnections.length}`" name="recent" />
          <el-tab-pane :label="`我的收藏 ${favoriteIds.length}`" name="favorites" />
        </el-tabs>

        <el-table
          v-loading="loading"
          :data="filteredHosts"
          class="connections-table"
          :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: '600' }"
          height="calc(100vh - 260px)"
          empty-text="暂无连接资产"
        >
          <el-table-column label="主机" min-width="230" fixed>
            <template #default="{ row }">
              <div class="host-cell">
                <div class="host-icon" :class="`host-status-${row.status}`">
                  <el-icon><Monitor /></el-icon>
                </div>
                <div class="host-text">
                  <div class="host-name-line">
                    <span class="host-name">{{ row.name }}</span>
                    <el-button link class="favorite-button" @click="toggleFavorite(row)">
                      <el-icon>
                        <StarFilled v-if="isFavorite(row.id)" />
                        <Star v-else />
                      </el-icon>
                    </el-button>
                  </div>
                  <div class="host-meta">{{ row.ip }} · {{ row.groupName || '未分组' }}</div>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="连接能力" min-width="210">
            <template #default="{ row }">
              <div class="capability-tags">
                <el-tag size="small" :type="row.osType === 'windows' ? 'warning' : 'success'">
                  {{ row.osType === 'windows' ? 'Windows' : 'Linux' }}
                </el-tag>
                <el-tag v-if="isSSHCapable(row)" size="small" type="info">SSH终端</el-tag>
                <el-tag v-if="isRDPCapable(row)" size="small" type="success">RDP桌面</el-tag>
                <el-tag v-if="isAgentAvailable(row)" size="small" type="primary">Agent在线</el-tag>
                <el-tag v-if="isConnectionEmpty(row)" size="small" type="danger">无可用连接</el-tag>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="状态" min-width="170">
            <template #default="{ row }">
              <div class="status-stack">
                <el-tag size="small" :type="getHostStatusType(row.status)">
                  {{ row.statusText || getHostStatusText(row.status) }}
                </el-tag>
                <span class="status-subtext">
                  {{ row.collectStatusText || getCollectStatusText(row.collectStatus) }}
                </span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="管理方式" min-width="130">
            <template #default="{ row }">
              <el-tag size="small" :type="getManagementModeTagType(row.managementMode)">
                {{ row.managementModeText || getManagementModeText(row.managementMode) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column label="最近连接" min-width="160">
            <template #default="{ row }">
              <span class="last-connection">{{ getLastConnectionText(row.id) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="230" fixed="right" align="center">
            <template #default="{ row }">
              <div class="table-actions">
                <el-tooltip :content="getSSHDisabledReason(row) || '打开 SSH 终端'" placement="top">
                  <el-button
                    link
                    class="action-button"
                    :disabled="!!getSSHDisabledReason(row)"
                    @click="openSSH(row)"
                  >
                    <el-icon><Connection /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="getDesktopDisabledReason(row) || '打开 Windows 桌面'" placement="top">
                  <el-button
                    link
                    class="action-button action-desktop"
                    :disabled="!!getDesktopDisabledReason(row)"
                    :loading="connectLoadingKey === `rdp:${row.id}`"
                    @click="openDesktop(row)"
                  >
                    <el-icon><Operation /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="主机详情" placement="top">
                  <el-button
                    link
                    class="action-button"
                    :disabled="!hasHostPermission(row.id, PERMISSION.VIEW)"
                    @click="openHostDetail(row)"
                  >
                    <el-icon><View /></el-icon>
                  </el-button>
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </main>

      <aside class="workspace-insights">
        <section class="insight-section">
          <div class="insight-title">连接统计</div>
          <div class="metric-grid">
            <div class="metric-item">
              <span class="metric-value">{{ tabCounts.ssh }}</span>
              <span class="metric-label">SSH</span>
            </div>
            <div class="metric-item">
              <span class="metric-value">{{ tabCounts.rdp }}</span>
              <span class="metric-label">RDP</span>
            </div>
            <div class="metric-item">
              <span class="metric-value">{{ tabCounts.agent }}</span>
              <span class="metric-label">Agent</span>
            </div>
            <div class="metric-item">
              <span class="metric-value">{{ tabCounts.none }}</span>
              <span class="metric-label">未配置</span>
            </div>
          </div>
        </section>

        <section class="insight-section">
          <div class="insight-title">最近连接</div>
          <div v-if="recentConnections.length" class="recent-list">
            <button
              v-for="item in recentConnections.slice(0, 6)"
              :key="`${item.type}-${item.hostId}-${item.connectedAt}`"
              class="recent-item"
              @click="reconnect(item)"
            >
              <span class="recent-host">{{ item.hostName }}</span>
              <span class="recent-meta">{{ getConnectionTypeText(item.type) }} · {{ formatRecentTime(item.connectedAt) }}</span>
            </button>
          </div>
          <el-empty v-else description="暂无记录" :image-size="56" />
        </section>

        <section class="insight-section">
          <div class="insight-title">我的收藏</div>
          <div v-if="favoriteHosts.length" class="favorite-list">
            <button
              v-for="host in favoriteHosts.slice(0, 6)"
              :key="host.id"
              class="favorite-item"
              @click="focusHost(host)"
            >
              <span>{{ host.name }}</span>
              <el-tag size="small" :type="host.osType === 'windows' ? 'warning' : 'success'">
                {{ host.osType === 'windows' ? 'Windows' : 'Linux' }}
              </el-tag>
            </button>
          </div>
          <el-empty v-else description="暂无收藏" :image-size="56" />
        </section>

        <section class="insight-section">
          <div class="insight-title">配置提示</div>
          <div v-if="configurationIssues.length" class="issue-list">
            <div v-for="item in configurationIssues.slice(0, 5)" :key="item.id" class="issue-item">
              <el-icon><WarningFilled /></el-icon>
              <span>{{ item.name }}：{{ getConfigurationIssue(item) }}</span>
            </div>
          </div>
          <el-empty v-else description="暂无提示" :image-size="56" />
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Connection,
  FolderOpened,
  Monitor,
  Operation,
  Refresh,
  Search,
  Star,
  StarFilled,
  View,
  WarningFilled
} from '@element-plus/icons-vue'
import { createDesktopSession, getHostList } from '@/api/host'
import { getGroupTree } from '@/api/assetGroup'
import { getUserHostPermissions } from '@/api/assetPermission'
import { hasPermission, PERMISSION } from '@/utils/permission'
import { useUserStore } from '@/stores/user'

interface HostItem {
  id: number
  name: string
  ip: string
  groupId?: number
  groupName?: string
  osType?: string
  status?: number
  statusText?: string
  managementMode?: string
  managementModeText?: string
  collectStatus?: string
  collectStatusText?: string
  desktopEnabled?: boolean
  desktopPort?: number
  desktopCredentialId?: number
  tags?: string[]
}

interface GroupNode {
  id: number
  name: string
  children?: GroupNode[]
}

interface RecentConnection {
  hostId: number
  hostName: string
  ip: string
  type: 'ssh' | 'rdp'
  connectedAt: string
}

const router = useRouter()
const userStore = useUserStore()

const FAVORITE_KEY = 'opshub:connection-workspace:favorites'
const RECENT_KEY = 'opshub:connection-workspace:recent'

const loading = ref(false)
const groupLoading = ref(false)
const connectLoadingKey = ref('')
const activeTab = ref('all')
const selectedGroupId = ref<number | null>(null)
const hostList = ref<HostItem[]>([])
const groupTree = ref<GroupNode[]>([])
const hostPermissions = ref<Map<number, number>>(new Map())
const favoriteIds = ref<number[]>(loadFavoriteIds())
const recentConnections = ref<RecentConnection[]>(loadRecentConnections())

const filters = reactive({
  keyword: '',
  osType: '',
  capability: '',
  status: ''
})

const treeProps = {
  label: 'name',
  children: 'children'
}

const isAdmin = computed(() => {
  const roles = userStore.userInfo?.roles || []
  return roles.some((role: any) => role.code === 'admin')
})

const scopedHosts = computed(() => {
  return hostList.value.filter(host => {
    if (filters.osType && host.osType !== filters.osType) {
      return false
    }
    if (filters.capability && !matchesCapability(host, filters.capability)) {
      return false
    }
    if (activeTab.value === 'ssh' && !isSSHCapable(host)) {
      return false
    }
    if (activeTab.value === 'desktop' && host.osType !== 'windows') {
      return false
    }
    if (activeTab.value === 'recent' && !recentConnections.value.some(item => item.hostId === host.id)) {
      return false
    }
    if (activeTab.value === 'favorites' && !isFavorite(host.id)) {
      return false
    }
    return true
  })
})

const filteredHosts = computed(() => {
  return scopedHosts.value
})

const favoriteHosts = computed(() => {
  return hostList.value.filter(host => isFavorite(host.id))
})

const tabCounts = computed(() => {
  return {
    all: hostList.value.length,
    ssh: hostList.value.filter(isSSHCapable).length,
    rdp: hostList.value.filter(isRDPCapable).length,
    agent: hostList.value.filter(isAgentAvailable).length,
    none: hostList.value.filter(isConnectionEmpty).length
  }
})

const configurationIssues = computed(() => {
  return hostList.value.filter(host => getConfigurationIssue(host))
})

function loadFavoriteIds() {
  try {
    const parsed = JSON.parse(localStorage.getItem(FAVORITE_KEY) || '[]')
    return Array.isArray(parsed) ? parsed.map(Number).filter(Boolean) : []
  } catch {
    return []
  }
}

function loadRecentConnections() {
  try {
    const parsed = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]')
    return Array.isArray(parsed) ? parsed.slice(0, 20) : []
  } catch {
    return []
  }
}

const saveFavoriteIds = () => {
  localStorage.setItem(FAVORITE_KEY, JSON.stringify(favoriteIds.value))
}

const saveRecentConnections = () => {
  localStorage.setItem(RECENT_KEY, JSON.stringify(recentConnections.value.slice(0, 20)))
}

const loadGroups = async () => {
  groupLoading.value = true
  try {
    groupTree.value = await getGroupTree()
  } finally {
    groupLoading.value = false
  }
}

const loadHosts = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: 1,
      pageSize: 10000,
      keyword: filters.keyword || undefined
    }
    if (selectedGroupId.value) {
      params.groupId = selectedGroupId.value
    }
    if (filters.status !== '') {
      params.status = Number(filters.status)
    }

    const res = await getHostList(params)
    hostList.value = res.list || []
    await loadHostPermissions()
  } catch (error: any) {
    ElMessage.error(error.message || '加载连接资产失败')
  } finally {
    loading.value = false
  }
}

const loadHostPermissions = async () => {
  const permissionsMap = new Map<number, number>()
  if (isAdmin.value) {
    hostList.value.forEach(host => permissionsMap.set(host.id, PERMISSION.ALL))
    hostPermissions.value = permissionsMap
    return
  }

  const results = await Promise.allSettled(
    hostList.value.map(host => getUserHostPermissions(host.id))
  )
  results.forEach((result, index) => {
    const host = hostList.value[index]
    if (!host) {
      return
    }
    if (result.status === 'fulfilled' && result.value?.permissions !== undefined) {
      permissionsMap.set(host.id, result.value.permissions)
    } else {
      permissionsMap.set(host.id, 0)
    }
  })
  hostPermissions.value = permissionsMap
}

const handleGroupSelect = async (group: GroupNode) => {
  selectedGroupId.value = group.id
  await loadHosts()
}

const selectAllGroups = async () => {
  selectedGroupId.value = null
  await loadHosts()
}

const resetFilters = async () => {
  filters.keyword = ''
  filters.osType = ''
  filters.capability = ''
  filters.status = ''
  selectedGroupId.value = null
  activeTab.value = 'all'
  await loadHosts()
}

const matchesCapability = (host: HostItem, capability: string) => {
  switch (capability) {
    case 'ssh':
      return isSSHCapable(host)
    case 'rdp':
      return isRDPCapable(host)
    case 'agent':
      return isAgentAvailable(host)
    case 'none':
      return isConnectionEmpty(host)
    default:
      return true
  }
}

const isSSHCapable = (host: HostItem) => {
  return host.osType !== 'windows' || host.managementMode === 'ssh'
}

const isRDPCapable = (host: HostItem) => {
  return host.osType === 'windows' && !!host.desktopEnabled
}

const isAgentAvailable = (host: HostItem) => {
  return host.osType === 'windows' && host.managementMode === 'agent' && host.collectStatus === 'online'
}

const isConnectionEmpty = (host: HostItem) => {
  return !isSSHCapable(host) && !isRDPCapable(host)
}

const hasHostPermission = (hostId: number, permission: number) => {
  if (isAdmin.value) {
    return true
  }
  return hasPermission(hostPermissions.value.get(hostId) || 0, permission)
}

const getSSHDisabledReason = (host: HostItem) => {
  if (!isSSHCapable(host)) {
    return '该主机不支持 SSH 终端'
  }
  if (!hasHostPermission(host.id, PERMISSION.TERMINAL)) {
    return '无 SSH 终端权限'
  }
  return ''
}

const getDesktopDisabledReason = (host: HostItem) => {
  if (host.osType !== 'windows') {
    return '仅 Windows 主机支持桌面'
  }
  if (!host.desktopEnabled) {
    return '未启用桌面访问'
  }
  if (!hasHostPermission(host.id, PERMISSION.DESKTOP)) {
    return '无桌面连接权限'
  }
  return ''
}

const getConfigurationIssue = (host: HostItem) => {
  if (host.osType === 'windows' && !host.desktopEnabled && host.managementMode !== 'ssh') {
    return '未配置桌面或 SSH 兼容入口'
  }
  if (host.osType === 'windows' && host.desktopEnabled && !host.desktopCredentialId) {
    return '缺少 RDP 凭据'
  }
  if (host.osType !== 'windows' && !hasHostPermission(host.id, PERMISSION.TERMINAL)) {
    return '缺少 SSH 终端权限'
  }
  return ''
}

const openSSH = (host: HostItem) => {
  const reason = getSSHDisabledReason(host)
  if (reason) {
    ElMessage.warning(reason)
    return
  }
  const pendingHosts = JSON.parse(sessionStorage.getItem('dblClickHosts') || '[]')
  pendingHosts.push(host)
  sessionStorage.setItem('dblClickHosts', JSON.stringify(pendingHosts))
  recordConnection(host, 'ssh')
  window.open(`${window.location.origin}/terminal`, '_blank')
}

const openDesktop = async (host: HostItem) => {
  const reason = getDesktopDisabledReason(host)
  if (reason) {
    ElMessage.warning(reason)
    return
  }

  connectLoadingKey.value = `rdp:${host.id}`
  try {
    const width = Math.max(window.innerWidth - 80, 1280)
    const height = Math.max(window.innerHeight - 160, 720)
    const res = await createDesktopSession(host.id, {
      width,
      height,
      dpi: window.devicePixelRatio ? Math.round(window.devicePixelRatio * 96) : 96
    })
    localStorage.setItem(`desktop-launch:${res.sessionId}`, res.launchUrl)
    recordConnection(host, 'rdp')
    const desktopUrl = `${window.location.origin}/desktop?sessionId=${res.sessionId}&hostName=${encodeURIComponent(host.name)}`
    window.open(desktopUrl, '_blank')
  } catch (error: any) {
    ElMessage.error(error.message || '桌面连接创建失败')
  } finally {
    connectLoadingKey.value = ''
  }
}

const recordConnection = (host: HostItem, type: 'ssh' | 'rdp') => {
  const next: RecentConnection = {
    hostId: host.id,
    hostName: host.name,
    ip: host.ip,
    type,
    connectedAt: new Date().toISOString()
  }
  recentConnections.value = [
    next,
    ...recentConnections.value.filter(item => !(item.hostId === host.id && item.type === type))
  ].slice(0, 20)
  saveRecentConnections()
}

const reconnect = (item: RecentConnection) => {
  const host = hostList.value.find(candidate => candidate.id === item.hostId)
  if (!host) {
    ElMessage.warning('主机不在当前列表中')
    return
  }
  if (item.type === 'ssh') {
    openSSH(host)
  } else {
    openDesktop(host)
  }
}

const openHostDetail = (host: HostItem) => {
  router.push({
    path: '/asset/hosts',
    query: {
      hostId: host.id,
      from: 'connections'
    }
  })
}

const goHostManagement = () => {
  router.push('/asset/hosts')
}

const isFavorite = (hostId: number) => {
  return favoriteIds.value.includes(hostId)
}

const toggleFavorite = (host: HostItem) => {
  if (isFavorite(host.id)) {
    favoriteIds.value = favoriteIds.value.filter(id => id !== host.id)
  } else {
    favoriteIds.value = [host.id, ...favoriteIds.value]
  }
  saveFavoriteIds()
}

const focusHost = (host: HostItem) => {
  filters.keyword = host.name
  activeTab.value = 'all'
}

const getLastConnectionText = (hostId: number) => {
  const item = recentConnections.value.find(record => record.hostId === hostId)
  if (!item) {
    return '-'
  }
  return `${getConnectionTypeText(item.type)} · ${formatRecentTime(item.connectedAt)}`
}

const getConnectionTypeText = (type: 'ssh' | 'rdp') => {
  return type === 'ssh' ? 'SSH' : 'RDP'
}

const formatRecentTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

const getHostStatusType = (status?: number) => {
  if (status === 1) return 'success'
  if (status === 0) return 'danger'
  return 'info'
}

const getHostStatusText = (status?: number) => {
  if (status === 1) return '在线'
  if (status === 0) return '离线'
  return '未知'
}

const getCollectStatusText = (status?: string) => {
  switch (status) {
    case 'online':
      return '采集在线'
    case 'offline':
      return '采集离线'
    case 'not_configured':
      return '未配置采集'
    default:
      return '采集未知'
  }
}

const getManagementModeText = (mode?: string) => {
  switch (mode) {
    case 'agent':
      return 'Agent'
    case 'winrm':
      return 'WinRM'
    case 'none':
      return '仅桌面'
    default:
      return 'SSH'
  }
}

const getManagementModeTagType = (mode?: string) => {
  switch (mode) {
    case 'agent':
      return 'success'
    case 'winrm':
      return 'warning'
    case 'none':
      return 'info'
    default:
      return 'primary'
  }
}

const getGroupSummary = (group: GroupNode) => {
  const groupIds = collectGroupIds(group)
  const groupHosts = hostList.value.filter(host => host.groupId && groupIds.includes(host.groupId))
  return {
    total: groupHosts.length,
    ssh: groupHosts.filter(isSSHCapable).length,
    rdp: groupHosts.filter(isRDPCapable).length
  }
}

const collectGroupIds = (group: GroupNode): number[] => {
  return [
    group.id,
    ...(group.children || []).flatMap(collectGroupIds)
  ]
}

onMounted(async () => {
  await Promise.all([
    loadGroups(),
    loadHosts()
  ])
})
</script>

<style scoped>
.connection-workspace {
  min-height: 100%;
  padding: 20px;
  background: #f5f7fa;
}

.workspace-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.workspace-title {
  margin: 0;
  color: #1f2937;
  font-size: 22px;
  font-weight: 700;
  line-height: 30px;
}

.workspace-subtitle {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.workspace-actions,
.workspace-filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.workspace-filters {
  padding: 12px;
  margin-bottom: 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.filter-search {
  width: 320px;
}

.filter-select {
  width: 150px;
}

.workspace-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr) 280px;
  gap: 16px;
  align-items: stretch;
}

.workspace-sidebar,
.workspace-main,
.workspace-insights {
  min-width: 0;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.workspace-sidebar,
.workspace-insights {
  padding: 14px;
}

.workspace-main {
  padding: 0 14px 14px;
}

.panel-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.panel-title,
.insight-title {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #1f2937;
  font-size: 14px;
  font-weight: 700;
}

.all-groups-button {
  width: 100%;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px;
  margin-bottom: 8px;
  color: #334155;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  cursor: pointer;
}

.all-groups-button.active {
  color: #1d4ed8;
  background: #eff6ff;
  border-color: #bfdbfe;
}

.group-tree {
  background: transparent;
}

.group-node {
  width: 100%;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.group-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-count {
  color: #94a3b8;
  font-size: 12px;
}

.connection-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.connections-table {
  width: 100%;
}

.host-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.host-icon {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  color: #64748b;
  background: #f1f5f9;
}

.host-status-1 {
  color: #16a34a;
  background: #dcfce7;
}

.host-status-0 {
  color: #dc2626;
  background: #fee2e2;
}

.host-text {
  min-width: 0;
  flex: 1;
}

.host-name-line {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.host-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1f2937;
  font-weight: 600;
}

.host-meta,
.status-subtext,
.last-connection {
  color: #64748b;
  font-size: 12px;
}

.favorite-button {
  width: 22px;
  min-width: 22px;
  height: 22px;
  color: #f59e0b;
}

.capability-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.status-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.table-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.action-button {
  width: 28px;
  height: 28px;
  min-width: 28px;
  padding: 0;
  color: #334155;
}

.action-button:hover {
  color: #2563eb;
  background: #eff6ff;
}

.action-desktop:hover {
  color: #16a34a;
  background: #ecfdf5;
}

.workspace-insights {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.insight-section {
  padding-bottom: 16px;
  border-bottom: 1px solid #e5e7eb;
}

.insight-section:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.insight-title {
  margin-bottom: 10px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.metric-item {
  min-height: 58px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 10px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.metric-value {
  color: #111827;
  font-size: 20px;
  font-weight: 700;
  line-height: 24px;
}

.metric-label {
  color: #64748b;
  font-size: 12px;
}

.recent-list,
.favorite-list,
.issue-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.recent-item,
.favorite-item {
  width: 100%;
  min-height: 42px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 2px;
  padding: 8px 10px;
  text-align: left;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  cursor: pointer;
}

.favorite-item {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.recent-item:hover,
.favorite-item:hover {
  border-color: #93c5fd;
  background: #eff6ff;
}

.recent-host {
  max-width: 100%;
  overflow: hidden;
  color: #1f2937;
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-meta {
  color: #64748b;
  font-size: 12px;
}

.issue-item {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  color: #92400e;
  font-size: 12px;
  line-height: 18px;
}

.black-button {
  color: #ffffff;
  background: #111827;
  border-color: #111827;
}

.black-button:hover {
  color: #ffffff;
  background: #374151;
  border-color: #374151;
}

@media (max-width: 1280px) {
  .workspace-layout {
    grid-template-columns: 220px minmax(0, 1fr);
  }

  .workspace-insights {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .insight-section {
    padding-bottom: 0;
    border-bottom: none;
  }
}

@media (max-width: 900px) {
  .connection-workspace {
    padding: 12px;
  }

  .workspace-header {
    flex-direction: column;
  }

  .workspace-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .workspace-insights {
    grid-template-columns: minmax(0, 1fr);
  }

  .filter-search,
  .filter-select {
    width: 100%;
  }
}
</style>
