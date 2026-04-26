<template>
  <div class="audit-panel">
    <div class="audit-search-bar">
      <el-input
        v-model="query.keyword"
        placeholder="搜索 SQL、操作者、Schema、客户端 IP..."
        clearable
        class="audit-search-input"
        @keyup.enter="emit('load')"
        @clear="emit('load')"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-select v-model="query.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="emit('load')">
        <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
      </el-select>
      <el-select v-model="query.status" placeholder="状态" clearable class="audit-select" @change="emit('load')">
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
        <el-option label="已拦截" value="denied" />
        <el-option label="执行中" value="pending" />
      </el-select>
      <el-select v-model="query.action" placeholder="审计动作" clearable class="audit-select" @change="emit('load')">
        <el-option v-for="item in auditActions" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-select v-model="query.riskLevel" placeholder="风险" clearable class="audit-select" @change="emit('load')">
        <el-option label="低" value="low" />
        <el-option label="中" value="medium" />
        <el-option label="高" value="high" />
        <el-option label="严重" value="critical" />
      </el-select>
      <el-select v-model="query.sqlType" placeholder="SQL 类型" clearable class="audit-select" @change="emit('load')">
        <el-option v-for="item in sqlTypes" :key="item" :label="item" :value="item" />
      </el-select>
      <el-button @click="emit('reset')">
        <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
        重置
      </el-button>
      <el-button type="primary" plain :loading="exporting" @click="emit('export')">
        <el-icon style="margin-right: 4px;"><Download /></el-icon>
        导出 CSV
      </el-button>
    </div>

    <el-table
      :data="rows"
      v-loading="loading"
      stripe
      class="modern-table"
      :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
    >
      <el-table-column label="执行时间" prop="createdAt" width="170" />
      <el-table-column label="实例" min-width="150">
        <template #default="{ row }">
          <span>{{ row.instanceName || `#${row.instanceId}` }}</span>
        </template>
      </el-table-column>
      <el-table-column label="Schema" min-width="120">
        <template #default="{ row }">{{ row.schemaName || '-' }}</template>
      </el-table-column>
      <el-table-column label="动作" width="120" align="center">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ row.actionText || row.action || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="SQL" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="mono">{{ row.sqlSummary || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="100" align="center">
        <template #default="{ row }">
          <el-tag size="small">{{ row.sqlType || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="风险" width="80" align="center">
        <template #default="{ row }">
          <el-tag :type="riskLevelTag(row.riskLevel)" size="small">
            {{ row.riskLevelText || row.riskLevel }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="auditStatusTag(row.status)" size="small">
            {{ row.statusText || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="返回行" prop="rowsReturned" width="90" align="right" />
      <el-table-column label="耗时" width="100" align="right">
        <template #default="{ row }">{{ row.durationMs }} ms</template>
      </el-table-column>
      <el-table-column label="操作者" min-width="110">
        <template #default="{ row }">{{ row.operatorName || '-' }}</template>
      </el-table-column>
      <el-table-column label="客户端 IP" min-width="130">
        <template #default="{ row }">{{ row.clientIp || '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="90" align="center" fixed="right">
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
</template>

<script setup lang="ts">
import { Download, RefreshLeft, Search } from '@element-plus/icons-vue'

type AuditQuery = {
  page: number
  pageSize: number
  keyword?: string
  instanceId?: number
  action?: string
  status?: string
  riskLevel?: string
  sqlType?: string
}

type SelectOption = {
  label: string
  value: string
}

defineProps<{
  query: AuditQuery
  instances: any[]
  auditActions: SelectOption[]
  sqlTypes: string[]
  rows: any[]
  loading: boolean
  exporting: boolean
  total: number
}>()

const emit = defineEmits<{
  load: []
  reset: []
  export: []
  detail: [row: any]
}>()

const auditStatusTag = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'denied':
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
</script>
