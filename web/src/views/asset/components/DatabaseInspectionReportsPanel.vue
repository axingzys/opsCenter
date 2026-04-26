<template>
  <div class="backup-panel">
    <el-alert
      title="巡检报告会聚合容量趋势、性能诊断、安全审计和备份状态，首批支持手动生成并落库留痕。"
      type="info"
      show-icon
      :closable="false"
    />

    <div class="backup-card">
      <div class="panel-title">
        <span>生成报告</span>
        <el-tag size="small" type="success">manual</el-tag>
      </div>
      <div class="backup-toolbar">
        <div class="backup-toolbar-group">
          <span class="toolbar-label">数据库实例</span>
          <el-select
            v-model="form.instanceId"
            placeholder="请选择实例"
            filterable
            class="metadata-instance-select"
          >
            <el-option
              v-for="item in instances"
              :key="item.id"
              :label="`${item.name}（${item.dbTypeText || item.dbType} / ${item.endpoint || `${item.host}:${item.port}`}）`"
              :value="item.id"
            />
          </el-select>
        </div>
        <el-button type="primary" :disabled="!form.instanceId" :loading="generating" @click="emit('generate')">
          生成巡检报告
        </el-button>
      </div>
    </div>

    <div class="backup-card">
      <div class="panel-title">
        <span>报告记录</span>
        <el-tag size="small" type="info">{{ total }}</el-tag>
      </div>
      <div class="backup-toolbar">
        <div class="backup-toolbar-group">
          <el-select
            v-model="query.instanceId"
            placeholder="实例"
            clearable
            filterable
            class="audit-select"
            @change="emit('load')"
          >
            <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select
            v-model="query.riskLevel"
            placeholder="风险"
            clearable
            class="audit-select"
            @change="emit('load')"
          >
            <el-option label="低" value="low" />
            <el-option label="中" value="medium" />
            <el-option label="高" value="high" />
            <el-option label="严重" value="critical" />
          </el-select>
          <el-select
            v-model="query.status"
            placeholder="状态"
            clearable
            class="audit-select"
            @change="emit('load')"
          >
            <el-option label="执行中" value="running" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
        </div>
        <div class="backup-toolbar-group">
          <el-button @click="emit('reset')">
            <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
            重置
          </el-button>
          <el-button type="primary" plain :loading="loading" @click="emit('load')">
            刷新报告
          </el-button>
        </div>
      </div>

      <el-table
        :data="reports"
        v-loading="loading"
        stripe
        class="modern-table"
        :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
      >
        <el-table-column label="生成时间" prop="generatedAt" width="170" />
        <el-table-column label="实例" min-width="160">
          <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
        </el-table-column>
        <el-table-column label="健康分" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="healthScoreTag(row.healthScore)" size="small">{{ row.healthScore }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="风险" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="riskLevelTag(row.riskLevel)" size="small">
              {{ row.riskLevelText || row.riskLevel || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="inspectionStatusTag(row.status)" size="small">
              {{ row.statusText || row.status || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="摘要" min-width="320" show-overflow-tooltip>
          <template #default="{ row }">{{ row.summary || row.errorMessage || '-' }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="100" align="right">
          <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" align="center" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="emit('detail', row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="emit('load')"
          @current-change="emit('load')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RefreshLeft } from '@element-plus/icons-vue'

type InstanceOption = {
  id: number
  name: string
  dbType?: string
  dbTypeText?: string
  endpoint?: string
  host?: string
  port?: number | string
}

type InspectionForm = {
  instanceId?: number | null
}

type InspectionQuery = {
  page: number
  pageSize: number
  instanceId?: number | null
  riskLevel?: string
  status?: string
}

defineProps<{
  form: InspectionForm
  query: InspectionQuery
  instances: InstanceOption[]
  reports: any[]
  generating: boolean
  loading: boolean
  total: number
}>()

const emit = defineEmits<{
  generate: []
  load: []
  reset: []
  detail: [row: any]
}>()

const inspectionStatusTag = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    case 'queued':
      return 'info'
    case 'cleaning':
      return 'warning'
    case 'pending':
      return 'info'
    default:
      return 'info'
  }
}

const riskLevelTag = (riskLevel: string) => {
  switch (riskLevel) {
    case 'critical':
      return 'danger'
    case 'high':
      return 'warning'
    case 'medium':
      return 'primary'
    case 'low':
      return 'success'
    default:
      return 'info'
  }
}

const healthScoreTag = (score?: number) => {
  const value = Number(score || 0)
  if (value >= 90) return 'success'
  if (value >= 80) return 'primary'
  if (value >= 60) return 'warning'
  return 'danger'
}
</script>
