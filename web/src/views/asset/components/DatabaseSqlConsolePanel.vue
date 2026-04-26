<template>
  <div class="query-console">
    <el-alert
      :title="alertTitle"
      type="info"
      show-icon
      :closable="false"
    />

    <div class="query-toolbar">
      <el-select
        :model-value="instanceId"
        placeholder="请选择实例"
        filterable
        class="query-select"
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
      <el-select
        :model-value="schemaName"
        :placeholder="schemaPlaceholder"
        clearable
        filterable
        class="query-select"
        @update:model-value="emit('update:schemaName', $event || '')"
      >
        <el-option v-for="item in schemas" :key="item.schemaName" :label="item.schemaName" :value="item.schemaName" />
      </el-select>
      <span class="query-option-label">最大行数</span>
      <el-input-number :model-value="limit" :min="1" :max="500" :step="50" class="query-number" @update:model-value="emit('update:limit', Number($event || 1))" />
      <span class="query-option-label">超时秒数</span>
      <el-input-number :model-value="timeoutSeconds" :min="1" :max="30" class="query-number" @update:model-value="emit('update:timeoutSeconds', Number($event || 1))" />
      <span class="query-option-label">导出上限</span>
      <el-input-number :model-value="exportLimit" :min="1" :max="5000" :step="100" class="query-number" @update:model-value="emit('update:exportLimit', Number($event || 1))" />
      <el-button type="primary" :loading="running" :disabled="!instanceId || !canQuery" @click="emit('execute')">
        {{ isRedis ? '执行命令' : '执行查询' }}
      </el-button>
      <el-button :loading="formatting" :disabled="!instanceId || !canQuery" @click="emit('formatQuery')">
        {{ isRedis ? '格式化命令' : '格式化 SQL' }}
      </el-button>
      <el-button v-if="!isRedis" :loading="explaining" :disabled="!instanceId || !canQuery" @click="emit('explain')">
        执行计划
      </el-button>
      <el-button v-if="!isRedis" type="warning" plain :loading="writeChecking" :disabled="!instanceId || !canWrite" @click="emit('validateWrite')">
        写前检查
      </el-button>
      <el-button v-if="!isRedis" type="danger" plain :loading="writePreparing" :disabled="!instanceId || !canWrite" @click="emit('prepareWrite')">
        受控写入
      </el-button>
      <el-button type="success" plain :loading="exporting" :disabled="!instanceId || !canExport" @click="emit('exportQuery')">
        <el-icon style="margin-right: 4px;"><Download /></el-icon>
        导出结果
      </el-button>
      <el-button type="primary" plain :loading="historyLoading" @click="emit('openHistory')">
        最近历史
      </el-button>
      <el-button @click="emit('reset')">清空</el-button>
    </div>

    <el-input
      :model-value="sqlText"
      type="textarea"
      :rows="10"
      class="sql-editor"
      :placeholder="editorPlaceholder"
      @update:model-value="emit('update:sqlText', $event)"
    />

    <div class="query-hints">
      <template v-if="isRedis">
        <el-tag size="small">只读命令</el-tag>
        <el-tag size="small" type="success">SCAN 自动 COUNT</el-tag>
        <el-tag size="small" type="primary">命令格式化</el-tag>
        <el-tag size="small" type="warning">CSV 导出</el-tag>
        <el-tag size="small" type="danger">禁止写命令</el-tag>
      </template>
      <template v-else>
        <el-tag size="small">只读</el-tag>
        <el-tag size="small" type="danger">受控写入</el-tag>
        <el-tag size="small" type="success">自动 LIMIT</el-tag>
        <el-tag size="small" type="primary">SQL 格式化</el-tag>
        <el-tag size="small" type="warning">执行计划 / CSV 导出</el-tag>
        <el-tag size="small" type="warning">禁止多语句</el-tag>
        <el-tag size="small" type="warning">原因 / 高风险确认</el-tag>
      </template>
      <span>当前实例：{{ currentInstance?.name || '-' }}</span>
    </div>

    <div v-if="writeResult" class="query-result">
      <div class="result-summary">
        <div class="summary-card">
          <span class="summary-label">影响行数</span>
          <strong>{{ writeResult.rowsAffected }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">耗时</span>
          <strong>{{ writeResult.durationMs }} ms</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">SQL 类型</span>
          <strong>{{ writeResult.sqlType }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">风险等级</span>
          <strong>{{ writeResult.riskLevelText }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">审计 ID</span>
          <strong>{{ writeResult.auditId }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">影响阈值</span>
          <strong>{{ writeResult.rowsAffectedLimit }}</strong>
        </div>
      </div>
      <el-alert
        :title="writeResult.message || '写操作执行完成'"
        type="success"
        show-icon
        :closable="false"
      />
      <el-alert
        v-if="writeResult.executedSql"
        :title="`实际执行 SQL：${writeResult.executedSql}`"
        type="warning"
        show-icon
        :closable="false"
        class="executed-sql-alert"
      />
      <div class="write-meta-tags">
        <el-tag :type="riskLevelTag(writeResult.riskLevel)">{{ writeResult.riskLevelText }}</el-tag>
        <el-tag v-if="writeResult.reason" type="info">原因：{{ writeResult.reason }}</el-tag>
        <el-tag v-if="writeResult.confirmRequired" :type="writeResult.confirmed ? 'success' : 'warning'">
          {{ writeResult.confirmed ? '已确认执行' : '未确认' }}
        </el-tag>
      </div>
      <div v-if="writeResult.rollbackSql" class="audit-sql-block">
        <div class="audit-sql-title">回滚 SQL / 恢复提示</div>
        <pre>{{ writeResult.rollbackSql }}</pre>
      </div>
    </div>
    <div v-else-if="writeCheckResult" class="query-result">
      <div class="result-summary">
        <div class="summary-card">
          <span class="summary-label">预检查结果</span>
          <strong>{{ writeCheckResult.allowed ? '通过' : '未通过' }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">SQL 类型</span>
          <strong>{{ writeCheckResult.sqlType }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">风险等级</span>
          <strong>{{ writeCheckResult.riskLevelText }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">影响阈值</span>
          <strong>{{ writeCheckResult.rowsAffectedLimit }}</strong>
        </div>
      </div>
      <el-alert
        :title="writeCheckResult.message || '写操作预检查完成'"
        :type="writeCheckResult.allowed ? 'success' : 'warning'"
        show-icon
        :closable="false"
      />
      <div class="write-meta-tags">
        <el-tag :type="riskLevelTag(writeCheckResult.riskLevel)">{{ writeCheckResult.riskLevelText }}</el-tag>
        <el-tag :type="writeCheckResult.reasonRequired ? 'warning' : 'info'">
          {{ writeCheckResult.reasonRequired ? '执行时必须填写原因' : '原因非必填' }}
        </el-tag>
        <el-tag :type="writeCheckResult.confirmRequired ? 'danger' : 'success'">
          {{ writeCheckResult.confirmRequired ? '执行前需二次确认' : '无需二次确认' }}
        </el-tag>
      </div>
    </div>
    <div v-else-if="queryResult" class="query-result">
      <div class="result-summary">
        <div class="summary-card">
          <span class="summary-label">返回行数</span>
          <strong>{{ queryResult.rowsReturned }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">耗时</span>
          <strong>{{ queryResult.durationMs }} ms</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">{{ isRedis ? '命令类型' : 'SQL 类型' }}</span>
          <strong>{{ queryResult.sqlType }}</strong>
        </div>
        <div class="summary-card">
          <span class="summary-label">审计 ID</span>
          <strong>{{ queryResult.auditId }}</strong>
        </div>
        <el-tag v-if="queryResult.truncated" type="warning">结果已截断</el-tag>
        <el-tag v-if="queryResult.cellTruncated" type="warning">字段已截断</el-tag>
        <el-tag v-if="queryResult.cellsMasked" type="info">敏感字段已脱敏</el-tag>
        <el-tag v-if="queryResult.binaryPreviewed" type="info">二进制已预览</el-tag>
      </div>
      <el-alert
        v-if="queryResult.executedSql"
        :title="`${isRedis ? '实际执行命令' : '实际执行 SQL'}：${queryResult.executedSql}`"
        type="success"
        show-icon
        :closable="false"
        class="executed-sql-alert"
      />
      <el-table :data="queryResult.rows || []" border stripe height="420" class="query-result-table">
        <el-table-column
          v-for="column in queryResult.columns || []"
          :key="column"
          :prop="column"
          :label="column"
          min-width="150"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <span class="query-cell">{{ formatQueryCell(row[column]) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-empty v-else :description="isRedis ? '执行 Redis 只读命令后在这里查看结果' : '执行只读 SQL、写前检查或受控写入后在这里查看结果'" :image-size="80" />
  </div>
</template>

<script setup lang="ts">
import { Download } from '@element-plus/icons-vue'

type InstanceOption = {
  id: number
  name: string
  endpoint?: string
  host?: string
  port?: number | string
}

type SchemaOption = {
  schemaName: string
}

defineProps<{
  instanceId?: number
  schemaName: string
  limit: number
  timeoutSeconds: number
  exportLimit: number
  sqlText: string
  alertTitle: string
  instances: InstanceOption[]
  schemas: SchemaOption[]
  schemaPlaceholder: string
  editorPlaceholder: string
  currentInstance?: InstanceOption
  isRedis: boolean
  canQuery: boolean
  canWrite: boolean
  canExport: boolean
  running: boolean
  formatting: boolean
  explaining: boolean
  writeChecking: boolean
  writePreparing: boolean
  exporting: boolean
  historyLoading: boolean
  queryResult?: any
  writeCheckResult?: any
  writeResult?: any
}>()

const emit = defineEmits<{
  'update:instanceId': [value: number | undefined]
  'update:schemaName': [value: string]
  'update:limit': [value: number]
  'update:timeoutSeconds': [value: number]
  'update:exportLimit': [value: number]
  'update:sqlText': [value: string]
  instanceChange: []
  execute: []
  formatQuery: []
  explain: []
  validateWrite: []
  prepareWrite: []
  exportQuery: []
  openHistory: []
  reset: []
}>()

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

const formatQueryCell = (value: any) => {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}
</script>
