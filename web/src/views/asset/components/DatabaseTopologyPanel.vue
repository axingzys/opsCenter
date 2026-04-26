<template>
  <div class="topology-panel">
    <el-alert
      title="四期第 1 批支持 Redis Cluster / MongoDB ReplicaSet / Elasticsearch / OpenSearch 的只读拓扑查询，并写入统一审计。"
      type="info"
      show-icon
      :closable="false"
    />

    <div class="metadata-toolbar">
      <div class="metadata-selector">
        <span class="toolbar-label">数据库实例</span>
        <el-select
          :model-value="instanceId"
          placeholder="请选择拓扑实例"
          filterable
          class="metadata-instance-select"
          @update:model-value="emit('update:instanceId', $event)"
          @change="emit('instanceChange')"
        >
          <el-option
            v-for="item in instances"
            :key="item.id"
            :label="`${item.name}（${item.dbTypeText || item.dbType} / ${item.endpoint || `${item.host}:${item.port}`}）`"
            :value="item.id"
          />
        </el-select>
        <el-tag v-if="currentInstance?.dbTypeText" type="success">
          {{ currentInstance.dbTypeText }}
        </el-tag>
      </div>
      <el-button type="primary" :disabled="!instanceId" :loading="loading" @click="emit('load')">
        刷新拓扑
      </el-button>
    </div>

    <div v-if="!instanceId" class="metadata-empty">
      <el-empty description="请先选择 Redis、MongoDB、Elasticsearch 或 OpenSearch 实例" :image-size="82" />
    </div>
    <div v-else class="topology-content" v-loading="loading">
      <el-empty v-if="!result" description="点击刷新拓扑后查看结果" :image-size="82" />
      <template v-else>
        <div class="table-overview-cards diagnosis-cards">
          <div v-for="card in result.cards || []" :key="card.key" class="table-overview-card">
            <span class="summary-label">{{ card.label }}</span>
            <strong>{{ card.value || '-' }}</strong>
            <div class="diagnosis-card-desc">{{ card.description }}</div>
          </div>
        </div>

        <el-alert
          v-if="result.message"
          :title="result.message"
          type="success"
          show-icon
          :closable="false"
          class="topology-message"
        />

        <div class="topology-grid">
          <div class="topology-section">
            <div class="panel-title">
              <span>拓扑节点</span>
              <el-tag size="small" type="info">{{ result.nodes?.length || 0 }}</el-tag>
            </div>
            <el-table :data="result.nodes || []" stripe height="360" class="modern-table">
              <el-table-column label="节点" min-width="150" show-overflow-tooltip>
                <template #default="{ row }">{{ row.name || row.id || '-' }}</template>
              </el-table-column>
              <el-table-column label="角色" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="topologyRoleTag(row.role)">{{ row.roleText || row.role || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="地址" min-width="170" show-overflow-tooltip>
                <template #default="{ row }"><span class="mono">{{ row.address || '-' }}</span></template>
              </el-table-column>
              <el-table-column label="状态" width="110">
                <template #default="{ row }">
                  <el-tag size="small" :type="topologyStateTag(row.state)">{{ row.state || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="Slots / 延迟" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.slots || row.lagText || '-' }}</template>
              </el-table-column>
              <el-table-column label="版本" width="130">
                <template #default="{ row }">{{ row.version || '-' }}</template>
              </el-table-column>
              <el-table-column label="备注" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.message || '-' }}</template>
              </el-table-column>
            </el-table>
          </div>

          <div class="topology-section">
            <div class="panel-title">
              <span>复制关系</span>
              <el-tag size="small" type="info">{{ result.links?.length || 0 }}</el-tag>
            </div>
            <el-table :data="result.links || []" stripe height="360" class="modern-table">
              <el-table-column label="源节点" prop="source" min-width="160" show-overflow-tooltip />
              <el-table-column label="目标节点" prop="target" min-width="160" show-overflow-tooltip />
              <el-table-column label="关系" width="120">
                <template #default="{ row }">{{ row.label || '-' }}</template>
              </el-table-column>
              <el-table-column label="状态" width="120">
                <template #default="{ row }">{{ row.state || '-' }}</template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <div v-if="result.shards?.length" class="topology-section">
          <div class="panel-title">
            <span>索引分片</span>
            <el-tag size="small" type="warning">{{ result.shards.length }}</el-tag>
          </div>
          <el-table :data="result.shards" stripe height="420" class="modern-table">
            <el-table-column label="索引" prop="index" min-width="220" show-overflow-tooltip />
            <el-table-column label="分片" prop="shard" width="90" align="center" />
            <el-table-column label="主副" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.primary ? 'success' : 'info'">{{ row.primary ? '主' : '副' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag size="small" :type="topologyStateTag(row.state)">{{ row.state || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="节点" prop="node" min-width="160" show-overflow-tooltip />
            <el-table-column label="地址" prop="address" min-width="140" show-overflow-tooltip />
            <el-table-column label="文档数" width="120" align="right">
              <template #default="{ row }">{{ formatNumber(row.docs) }}</template>
            </el-table-column>
            <el-table-column label="容量" width="120" align="right">
              <template #default="{ row }">{{ formatBytes(row.storeBytes) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
type InstanceOption = {
  id: number
  name: string
  dbType?: string
  dbTypeText?: string
  endpoint?: string
  host?: string
  port?: number | string
}

type TopologyResult = {
  message?: string
  cards?: any[]
  nodes?: any[]
  links?: any[]
  shards?: any[]
}

defineProps<{
  instanceId?: number
  instances: InstanceOption[]
  currentInstance?: InstanceOption
  result?: TopologyResult
  loading: boolean
}>()

const emit = defineEmits<{
  'update:instanceId': [value: number | undefined]
  instanceChange: []
  load: []
}>()

const topologyRoleTag = (role: string) => {
  const normalized = String(role || '').toLowerCase()
  if (['master', 'primary'].includes(normalized)) return 'success'
  if (['replica', 'slave', 'secondary'].includes(normalized)) return 'primary'
  if (normalized === 'arbiter') return 'warning'
  return 'info'
}

const topologyStateTag = (state: string) => {
  const normalized = String(state || '').toLowerCase()
  if (['ok', 'online', 'connected', 'healthy', 'started', 'green'].includes(normalized)) return 'success'
  if (['yellow', 'recovering', 'initializing', 'relocating'].includes(normalized)) return 'warning'
  if (['unhealthy', 'fail', 'failed', 'red', 'disconnected', 'down'].includes(normalized)) return 'danger'
  return 'info'
}

const formatNumber = (value?: number) => {
  return Number(value || 0).toLocaleString()
}

const formatBytes = (value?: number) => {
  const size = Number(value || 0)
  if (size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}
</script>
