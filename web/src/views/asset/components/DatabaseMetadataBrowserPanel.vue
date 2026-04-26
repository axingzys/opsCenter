<template>
  <div class="metadata-browser">
    <div class="metadata-toolbar">
      <div class="metadata-selector">
        <span class="toolbar-label">数据库实例</span>
        <el-select
          :model-value="instanceId"
          placeholder="请选择实例"
          filterable
          class="metadata-instance-select"
          @update:model-value="emit('update:instanceId', $event)"
          @change="emit('instanceChange')"
        >
          <el-option
            v-for="item in instances"
            :key="item.id"
            :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
            :value="item.id"
          />
        </el-select>
        <el-tag v-if="currentInstance?.lastSyncAt" type="success">
          最近同步：{{ currentInstance.lastSyncAt }}
        </el-tag>
        <el-tag v-else type="info">未同步</el-tag>
      </div>
      <el-button
        type="primary"
        :disabled="!instanceId || !canSync"
        :loading="syncing"
        @click="emit('sync')"
      >
        <el-icon style="margin-right: 6px;"><Refresh /></el-icon>
        同步元数据
      </el-button>
    </div>

    <div v-if="!instanceId" class="metadata-empty">
      <el-empty description="请先选择一个数据库实例" :image-size="82" />
    </div>
    <div v-else class="metadata-content">
      <div class="schema-panel">
        <div class="panel-title">
          <el-icon><Coin /></el-icon>
          <span>{{ isRedis ? '逻辑 DB' : '库 / Schema' }}</span>
        </div>
        <el-tree
          v-loading="schemasLoading"
          :data="schemaTree"
          node-key="id"
          default-expand-all
          highlight-current
          class="schema-tree"
          @node-click="emit('schemaClick', $event)"
        >
          <template #default="{ data }">
            <span class="schema-node">
              <span class="schema-node-name">{{ data.label }}</span>
              <el-tag size="small" type="info">{{ data.tableCount }}</el-tag>
            </span>
          </template>
        </el-tree>
        <el-empty v-if="!schemasLoading && schemasCount === 0" description="暂无元数据，请先同步" :image-size="64" />
      </div>

      <div class="table-panel">
        <div class="panel-title">
          <el-icon><Grid /></el-icon>
          <span>{{ isRedis ? 'Key 列表' : '表 / 视图' }}</span>
        </div>
        <el-table
          :data="tables"
          v-loading="tablesLoading"
          stripe
          height="520"
          highlight-current-row
          class="metadata-table"
          @row-click="emit('tableClick', $event)"
        >
          <template v-if="isRedis">
            <el-table-column label="Key" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="table-name-cell">
                  <span>{{ row.tableName }}</span>
                  <el-tag v-if="row.tableType" size="small" type="success">{{ row.tableType }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="TTL" width="120">
              <template #default="{ row }">{{ row.ttlText || '-' }}</template>
            </el-table-column>
            <el-table-column label="内存" width="110" align="right">
              <template #default="{ row }">{{ formatBytes(row.dataSizeBytes) }}</template>
            </el-table-column>
            <el-table-column label="长度" width="100" align="right">
              <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
            </el-table-column>
            <el-table-column label="节点" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.nodeAddress || '-' }}</template>
            </el-table-column>
          </template>
          <template v-else>
            <el-table-column label="名称" min-width="190">
              <template #default="{ row }">
                <div class="table-name-cell">
                  <span>{{ row.tableName }}</span>
                  <el-tag v-if="row.tableType" size="small" type="info">{{ row.tableType }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="引擎" width="100">
              <template #default="{ row }">{{ row.engine || '-' }}</template>
            </el-table-column>
            <el-table-column label="行数估算" width="110" align="right">
              <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
            </el-table-column>
            <el-table-column label="容量" width="110" align="right">
              <template #default="{ row }">{{ formatBytes((row.dataSizeBytes || 0) + (row.indexSizeBytes || 0)) }}</template>
            </el-table-column>
          </template>
        </el-table>
      </div>

      <div class="detail-panel">
        <div class="panel-title detail-panel-title">
          <div class="detail-panel-heading">
            <el-icon><Tickets /></el-icon>
            <span>{{ selectedTable?.tableName || (isRedis ? 'Key 属性' : '字段 / 索引') }}</span>
          </div>
          <div v-if="selectedTable && !isRedis" class="detail-panel-actions">
            <el-button link type="primary" :loading="ddlLoading" @click="emit('previewDdl')">
              查看 DDL
            </el-button>
            <el-button link type="primary" :loading="dictionaryExporting" :disabled="!canExport" @click="emit('exportDictionary')">
              <el-icon style="margin-right: 4px;"><Download /></el-icon>
              导出字典
            </el-button>
          </div>
        </div>
        <el-empty v-if="!selectedTable" :description="isRedis ? '请选择一个 Key 查看属性' : '请选择一张表查看字段和索引'" :image-size="72" />
        <div v-else>
          <div class="table-overview-cards">
            <div class="table-overview-card">
              <span class="summary-label">对象类型</span>
              <strong>{{ selectedTable.tableType || '-' }}</strong>
            </div>
            <div class="table-overview-card">
              <span class="summary-label">{{ isRedis ? 'TTL' : '行数估算' }}</span>
              <strong>{{ isRedis ? (selectedTable.ttlText || '-') : formatNumber(selectedTable.rowCount) }}</strong>
            </div>
            <div class="table-overview-card">
              <span class="summary-label">{{ isRedis ? '内存占用' : '数据容量' }}</span>
              <strong>{{ formatBytes(selectedTable.dataSizeBytes) }}</strong>
            </div>
            <div class="table-overview-card">
              <span class="summary-label">{{ isRedis ? '编码 / 节点' : '索引容量' }}</span>
              <strong>{{ isRedis ? (selectedTable.encoding || selectedTable.nodeAddress || '-') : formatBytes(selectedTable.indexSizeBytes) }}</strong>
            </div>
          </div>
          <div v-if="isRedis">
            <el-table :data="columns" v-loading="detailsLoading" stripe height="470" class="metadata-table">
              <el-table-column label="属性" prop="columnName" width="140" />
              <el-table-column label="值" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ row.defaultValue || '-' }}</template>
              </el-table-column>
              <el-table-column label="说明" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.comment || '-' }}</template>
              </el-table-column>
            </el-table>
          </div>
          <el-tabs v-else :model-value="detailTab" class="detail-tabs" @update:model-value="emit('update:detailTab', String($event))">
            <el-tab-pane label="字段" name="columns">
              <el-table :data="columns" v-loading="detailsLoading" stripe height="470" class="metadata-table">
                <el-table-column label="#" prop="ordinalPosition" width="54" align="center" />
                <el-table-column label="字段" min-width="150">
                  <template #default="{ row }">
                    <div class="column-name-cell">
                      <span>{{ row.columnName }}</span>
                      <el-tag v-if="row.isSensitive" size="small" type="danger">敏感</el-tag>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="类型" prop="dataType" min-width="150" />
                <el-table-column label="可空" width="70" align="center">
                  <template #default="{ row }">{{ row.isNullable ? '是' : '否' }}</template>
                </el-table-column>
                <el-table-column label="键" width="80" align="center">
                  <template #default="{ row }">{{ row.columnKey || '-' }}</template>
                </el-table-column>
                <el-table-column label="默认值" min-width="120">
                  <template #default="{ row }">{{ row.defaultValue || '-' }}</template>
                </el-table-column>
                <el-table-column label="注释" min-width="160">
                  <template #default="{ row }">{{ row.comment || '-' }}</template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane label="索引" name="indexes">
              <el-table :data="indexes" v-loading="detailsLoading" stripe height="470" class="metadata-table">
                <el-table-column label="索引名" prop="indexName" min-width="150" />
                <el-table-column label="字段" prop="columns" min-width="180" />
                <el-table-column label="类型" width="110">
                  <template #default="{ row }">{{ row.indexType || '-' }}</template>
                </el-table-column>
                <el-table-column label="唯一" width="80" align="center">
                  <template #default="{ row }">
                    <el-tag :type="row.isUnique ? 'success' : 'info'" size="small">{{ row.isUnique ? '是' : '否' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="基数" width="100" align="right">
                  <template #default="{ row }">{{ formatNumber(row.cardinality) }}</template>
                </el-table-column>
                <el-table-column label="注释" min-width="140">
                  <template #default="{ row }">{{ row.comment || '-' }}</template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Coin, Download, Grid, Refresh, Tickets } from '@element-plus/icons-vue'

type InstanceOption = {
  id: number
  name: string
  endpoint?: string
  host?: string
  port?: number | string
  lastSyncAt?: string
}

defineProps<{
  instanceId?: number
  detailTab: string
  instances: InstanceOption[]
  currentInstance?: InstanceOption
  canSync: boolean
  canExport: boolean
  syncing: boolean
  schemaTree: any[]
  schemasLoading: boolean
  schemasCount: number
  isRedis: boolean
  tables: any[]
  tablesLoading: boolean
  selectedTable?: any
  columns: any[]
  indexes: any[]
  detailsLoading: boolean
  ddlLoading: boolean
  dictionaryExporting: boolean
}>()

const emit = defineEmits<{
  'update:instanceId': [value: number | undefined]
  'update:detailTab': [value: string]
  instanceChange: []
  sync: []
  schemaClick: [data: any]
  tableClick: [row: any]
  previewDdl: []
  exportDictionary: []
}>()

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
