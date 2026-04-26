<template>
  <div v-loading="loading" class="inventory-container">
    <template v-if="hasInventory">
      <div class="inventory-section">
        <div class="inventory-section-title">
          <el-icon><Link /></el-icon>
          <span>IP地址</span>
        </div>
        <div class="inventory-ip-group">
          <div class="inventory-ip-label">内网</div>
          <div class="inventory-ip-tags">
            <el-tag v-for="item in currentInventory.privateIps" :key="`private-${item}`" type="info">{{ item }}</el-tag>
            <span v-if="!currentInventory.privateIps.length" class="text-muted">-</span>
          </div>
        </div>
        <div class="inventory-ip-group">
          <div class="inventory-ip-label">公网</div>
          <div class="inventory-ip-content">
            <div class="inventory-ip-tags">
              <el-tag v-for="item in currentInventory.publicIps" :key="`public-${item}`" type="success">{{ item }}</el-tag>
              <span v-if="!currentInventory.publicIps.length" class="text-muted">-</span>
            </div>
            <div v-if="currentInventory.publicIpHistory.length" class="inventory-ip-history">
              <div class="inventory-ip-history-title">变更历史</div>
              <div
                v-for="(item, index) in currentInventory.publicIpHistory"
                :key="`${item.ip}-${item.firstSeenAt}-${index}`"
                class="inventory-ip-history-item"
              >
                <div class="inventory-ip-history-main">
                  <el-tag :type="item.isCurrent ? 'success' : 'info'" effect="plain">{{ item.ip }}</el-tag>
                  <el-tag v-if="item.isCurrent" type="success">当前</el-tag>
                  <span class="inventory-ip-history-meta">首次 {{ item.firstSeenAt || '-' }}</span>
                  <span class="inventory-ip-history-meta">最近 {{ item.lastSeenAt || '-' }}</span>
                  <span class="inventory-ip-history-meta">上报 {{ item.seenCount || 0 }} 次</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="inventory-section">
        <div class="inventory-section-title">
          <el-icon><Setting /></el-icon>
          <span>配置摘要</span>
        </div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="系统">{{ currentInventory.configSummary.osRelease || '-' }}</el-descriptions-item>
          <el-descriptions-item label="内核">{{ currentInventory.configSummary.kernel || '-' }}</el-descriptions-item>
          <el-descriptions-item label="架构">{{ currentInventory.configSummary.arch || '-' }}</el-descriptions-item>
          <el-descriptions-item label="时区">{{ currentInventory.configSummary.timezone || '-' }}</el-descriptions-item>
          <el-descriptions-item label="服务管理">{{ currentInventory.configSummary.serviceManager || '-' }}</el-descriptions-item>
          <el-descriptions-item label="容器运行时">{{ currentInventory.configSummary.containerRuntime || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Agent版本">{{ currentInventory.configSummary.agentVersion || '-' }}</el-descriptions-item>
          <el-descriptions-item label="采集时间">{{ currentInventory.collectedAt || '-' }}</el-descriptions-item>
        </el-descriptions>
      </div>

      <div class="inventory-section">
        <div class="inventory-section-title">
          <el-icon><Coin /></el-icon>
          <span>磁盘</span>
        </div>
        <el-table :data="currentInventory.disks" size="small">
          <el-table-column label="设备" prop="device" min-width="120" />
          <el-table-column label="挂载点" prop="mountPoint" min-width="140" />
          <el-table-column label="文件系统" prop="fstype" width="120" />
          <el-table-column label="总容量" min-width="110">
            <template #default="{ row }">{{ formatBytes(row.total) }}</template>
          </el-table-column>
          <el-table-column label="已用" min-width="110">
            <template #default="{ row }">{{ formatBytes(row.used) }}</template>
          </el-table-column>
          <el-table-column label="使用率" width="120">
            <template #default="{ row }">{{ formatPercent(row.usage) }}</template>
          </el-table-column>
        </el-table>
      </div>

      <div class="inventory-grid">
        <div class="inventory-section">
          <div class="inventory-section-title">
            <el-icon><Cpu /></el-icon>
            <span>进程</span>
          </div>
          <el-table :data="currentInventory.topProcesses" size="small" max-height="320">
            <el-table-column label="PID" prop="pid" width="80" />
            <el-table-column label="名称" prop="name" min-width="140" show-overflow-tooltip />
            <el-table-column label="CPU" width="90">
              <template #default="{ row }">{{ formatPercent(row.cpuPercent) }}</template>
            </el-table-column>
            <el-table-column label="内存" width="90">
              <template #default="{ row }">{{ formatPercent(row.memoryPercent) }}</template>
            </el-table-column>
          </el-table>
        </div>

        <div class="inventory-section">
          <div class="inventory-section-title">
            <el-icon><Memo /></el-icon>
            <span>监听端口</span>
          </div>
          <el-table :data="currentInventory.listeningPorts" size="small" max-height="320">
            <el-table-column label="协议" prop="protocol" width="80" />
            <el-table-column label="地址" prop="listenAddr" min-width="120" />
            <el-table-column label="端口" prop="port" width="90" />
            <el-table-column label="进程" prop="processName" min-width="120" show-overflow-tooltip />
          </el-table>
        </div>
      </div>
    </template>

    <el-empty v-else description="暂无 Agent 采集数据" :image-size="72" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Cpu, Coin, Link, Memo, Setting } from '@element-plus/icons-vue'

interface InventorySummary {
  osRelease?: string
  kernel?: string
  arch?: string
  timezone?: string
  serviceManager?: string
  containerRuntime?: string
  agentVersion?: string
}

interface InventoryState {
  hostId?: number
  privateIps?: string[]
  publicIps?: string[]
  publicIpHistory?: Array<{
    ip?: string
    source?: string
    firstSeenAt?: string
    lastSeenAt?: string
    seenCount?: number
    isCurrent?: boolean
  }>
  disks?: any[]
  topProcesses?: any[]
  listeningPorts?: any[]
  configSummary?: InventorySummary
  collectedAt?: string
}

const props = withDefaults(defineProps<{
  inventory?: InventoryState
  loading?: boolean
}>(), {
  inventory: () => ({}),
  loading: false
})

const currentInventory = computed(() => ({
  hostId: props.inventory?.hostId || 0,
  privateIps: props.inventory?.privateIps || [],
  publicIps: props.inventory?.publicIps || [],
  publicIpHistory: props.inventory?.publicIpHistory || [],
  disks: props.inventory?.disks || [],
  topProcesses: props.inventory?.topProcesses || [],
  listeningPorts: props.inventory?.listeningPorts || [],
  configSummary: props.inventory?.configSummary || {},
  collectedAt: props.inventory?.collectedAt || ''
}))

const hasInventory = computed(() => {
  const inventory = currentInventory.value
  return Boolean(
    inventory.collectedAt ||
    inventory.privateIps.length ||
    inventory.publicIps.length ||
    inventory.publicIpHistory.length ||
    inventory.disks.length ||
    inventory.topProcesses.length ||
    inventory.listeningPorts.length ||
    Object.keys(inventory.configSummary || {}).length
  )
})

const formatBytes = (value?: number) => {
  if (!value) {
    return '-'
  }
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
  if (value === undefined || value === null) {
    return '-'
  }
  return `${Number(value).toFixed(1)}%`
}
</script>

<style scoped>
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

.inventory-ip-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
}

.inventory-ip-history {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 12px;
  background: #f8fafc;
  border: 1px solid #ebeef5;
}

.inventory-ip-history-title {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.inventory-ip-history-item {
  display: flex;
  align-items: center;
  min-height: 28px;
}

.inventory-ip-history-main {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.inventory-ip-history-meta {
  font-size: 12px;
  color: #6b7280;
}

.inventory-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.text-muted {
  color: #9ca3af;
}

@media (max-width: 1200px) {
  .inventory-grid {
    grid-template-columns: 1fr;
  }
}
</style>
