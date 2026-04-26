<template>
  <div class="host-alert-rules-container">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><Bell /></el-icon>
        </div>
        <div>
          <h2 class="page-title">主机告警</h2>
          <p class="page-subtitle">为 Agent 主机配置阈值告警和离线告警，支持 Linux / Windows</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="handleAdd">
          <el-icon style="margin-right: 6px;"><Plus /></el-icon>
          新增规则
        </el-button>
        <el-button @click="loadData">
          <el-icon style="margin-right: 6px;"><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-icon stat-icon-primary"><el-icon><Histogram /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">规则总数</div>
          <div class="stat-value">{{ stats.total }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-success"><el-icon><CircleCheck /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">已启用</div>
          <div class="stat-value">{{ stats.enabled }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-muted"><el-icon><SwitchButton /></el-icon></div>
        <div class="stat-content">
          <div class="stat-label">已停用</div>
          <div class="stat-value">{{ stats.disabled }}</div>
        </div>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        v-loading="loading"
        :data="tableData"
        class="modern-table"
        :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
      >
        <el-table-column label="规则名称" prop="name" min-width="180" />

        <el-table-column label="作用主机" min-width="180">
          <template #default="{ row }">
            <span>{{ getHostName(row.hostId) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="告警指标" width="150">
          <template #default="{ row }">
            <el-tag>{{ getMetricText(row.metric) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="告警通道" min-width="180">
          <template #default="{ row }">
            <span>{{ getChannelNames(row.channelIds) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="阈值" width="130">
          <template #default="{ row }">{{ formatThreshold(row.metric, row.threshold) }}</template>
        </el-table-column>

        <el-table-column label="告警间隔" width="130">
          <template #default="{ row }">{{ row.alertInterval || 0 }} 秒</template>
        </el-table-column>

        <el-table-column label="级别" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.severity === 'critical' ? 'danger' : 'warning'">
              {{ row.severity === 'critical' ? '严重' : '告警' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="handleToggle(row, $event)"
            />
          </template>
        </el-table-column>

        <el-table-column label="更新时间" prop="updatedAt" width="180" />

        <el-table-column label="操作" width="140" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button link type="danger" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="640px"
      destroy-on-close
      @closed="handleDialogClosed"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="110px">
        <el-form-item label="规则名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入规则名称" />
        </el-form-item>

        <el-form-item label="作用主机">
          <div class="host-scope-field">
            <el-radio-group v-model="hostScope">
              <el-radio label="all">全部 Agent 主机</el-radio>
              <el-radio label="single">指定主机</el-radio>
            </el-radio-group>
            <div v-if="hostScope === 'single'" class="host-picker-panel">
              <el-input
                v-model="hostKeyword"
                clearable
                placeholder="搜索主机名 / IP"
              />
              <div class="host-picker-scroll">
                <el-radio-group v-model="form.hostId" class="host-picker-group">
                  <el-radio
                    v-for="item in filteredHostOptions"
                    :key="item.id"
                    :value="item.id"
                    class="host-picker-option"
                  >
                    <div class="host-picker-option-main">
                      <div class="host-picker-option-head">
                        <span class="host-picker-option-name">{{ item.name }}</span>
                        <el-tag
                          size="small"
                          effect="plain"
                          :type="item.osType === 'windows' ? 'primary' : 'success'"
                        >
                          {{ item.osType === 'windows' ? 'Windows' : 'Linux' }}
                        </el-tag>
                      </div>
                      <span class="host-picker-option-meta">{{ item.primaryPrivateIp || item.ip }}</span>
                    </div>
                  </el-radio>
                </el-radio-group>
                <el-empty
                  v-if="!filteredHostOptions.length"
                  description="暂无匹配主机"
                  :image-size="40"
                />
              </div>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="告警通道">
          <el-select
            v-model="form.channelIds"
            multiple
            collapse-tags
            collapse-tags-tooltip
            placeholder="留空表示全部已启用通道"
            style="width: 100%;"
          >
            <el-option
              v-for="item in channelOptions"
              :key="item.id"
              :label="item.name"
              :value="item.id"
              :disabled="!item.enabled"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="告警指标" prop="metric">
          <el-select v-model="form.metric" placeholder="请选择告警指标" style="width: 100%;">
            <el-option label="CPU 使用率" value="cpu_usage" />
            <el-option label="内存使用率" value="memory_usage" />
            <el-option label="磁盘使用率" value="disk_usage" />
            <el-option label="Agent 离线" value="agent_offline" />
          </el-select>
        </el-form-item>

        <el-form-item :label="form.metric === 'agent_offline' ? '超时阈值' : '告警阈值'" prop="threshold">
          <el-input-number
            v-model="form.threshold"
            :min="1"
            :max="999999"
            :precision="form.metric === 'agent_offline' ? 0 : 2"
            style="width: 180px;"
          />
          <span class="form-hint">{{ form.metric === 'agent_offline' ? '秒' : '%' }}</span>
        </el-form-item>

        <el-form-item label="告警间隔" prop="alertInterval">
          <el-input-number v-model="form.alertInterval" :min="60" :max="86400" :precision="0" style="width: 180px;" />
          <span class="form-hint">秒</span>
        </el-form-item>

        <el-form-item label="严重级别" prop="severity">
          <el-radio-group v-model="form.severity">
            <el-radio label="warning">告警</el-radio>
            <el-radio label="critical">严重</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="是否启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="可选说明" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Bell,
  CircleCheck,
  Delete,
  Edit,
  Histogram,
  Plus,
  Refresh,
  SwitchButton
} from '@element-plus/icons-vue'
import {
  createHostAlertRule,
  deleteHostAlertRule,
  getHostAlertRules,
  getHostAlertRuleStats,
  listAssignableMonitorHosts,
  updateHostAlertRule,
  type AssignableMonitorHost,
  type HostAlertRule
} from '@/api/monitor-host'
import { getAlertChannels, type AlertChannel } from '@/api/alert-config'

const loading = ref(false)
const dialogVisible = ref(false)
const dialogTitle = ref('新增规则')
const submitting = ref(false)
const formRef = ref<FormInstance>()
const hostScope = ref<'all' | 'single'>('all')
const hostKeyword = ref('')

const stats = reactive({
  total: 0,
  enabled: 0,
  disabled: 0
})

const tableData = ref<HostAlertRule[]>([])
const hostOptions = ref<AssignableMonitorHost[]>([])
const channelOptions = ref<AlertChannel[]>([])

const normalizeAssignableHosts = (hosts: AssignableMonitorHost[] = []) => {
  return hosts
    .map((item) => {
      const id = Number(item.id ?? item.ID ?? 0)
      return {
        ...item,
        id
      }
    })
    .filter(item => item.id > 0)
}

const filteredHostOptions = computed(() => {
  const keyword = hostKeyword.value.trim().toLowerCase()
  if (!keyword) {
    return hostOptions.value
  }
  return hostOptions.value.filter((item) => {
    return [item.name, item.ip, item.primaryPrivateIp]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword))
  })
})

const createDefaultForm = () => ({
  id: undefined as number | undefined,
  name: '',
  hostId: 0,
  channelIds: [] as number[],
  metric: 'cpu_usage' as HostAlertRule['metric'],
  threshold: 80,
  alertInterval: 600,
  severity: 'warning' as HostAlertRule['severity'],
  enabled: true,
  description: ''
})

const form = reactive(createDefaultForm())

const rules: FormRules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
  metric: [{ required: true, message: '请选择告警指标', trigger: 'change' }],
  threshold: [{ required: true, message: '请输入阈值', trigger: 'blur' }],
  alertInterval: [{ required: true, message: '请输入告警间隔', trigger: 'blur' }]
}

const getMetricText = (metric: string) => {
  const metricMap: Record<string, string> = {
    cpu_usage: 'CPU 使用率',
    memory_usage: '内存使用率',
    disk_usage: '磁盘使用率',
    agent_offline: 'Agent 离线'
  }
  return metricMap[metric] || metric
}

const getHostName = (hostId?: number | null) => {
  if (!hostId) return '全部 Agent 主机'
  const host = hostOptions.value.find(item => item.id === hostId)
  return host ? `${host.name} (${host.primaryPrivateIp || host.ip})` : `主机 #${hostId}`
}

const getChannelNames = (channelIds?: number[]) => {
  const selected = (channelIds || []).filter(Boolean)
  if (!selected.length) {
    return '全部已启用通道'
  }
  const names = selected
    .map(channelId => channelOptions.value.find(item => item.id === channelId)?.name)
    .filter(Boolean)
  return names.length ? names.join('、') : '已选通道'
}

const formatThreshold = (metric: string, threshold?: number) => {
  if (threshold === undefined || threshold === null) return '-'
  if (metric === 'agent_offline') return `${threshold} 秒`
  return `${Number(threshold).toFixed(2)}%`
}

const loadData = async () => {
  loading.value = true
  try {
    const [rulesData, statsData, hostsData, channelsData] = await Promise.all([
      getHostAlertRules(),
      getHostAlertRuleStats(),
      listAssignableMonitorHosts(),
      getAlertChannels()
    ])
    tableData.value = rulesData || []
    hostOptions.value = normalizeAssignableHosts(hostsData || [])
    channelOptions.value = channelsData || []
    Object.assign(stats, statsData || { total: 0, enabled: 0, disabled: 0 })
  } catch (error: any) {
    ElMessage.error('加载主机告警规则失败')
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  dialogTitle.value = '新增主机告警规则'
  Object.assign(form, createDefaultForm())
  hostScope.value = 'all'
  hostKeyword.value = ''
  dialogVisible.value = true
}

const handleEdit = (row: HostAlertRule) => {
  dialogTitle.value = '编辑主机告警规则'
  Object.assign(form, {
    id: row.id,
    name: row.name,
    hostId: row.hostId || 0,
    channelIds: row.channelIds || [],
    metric: row.metric,
    threshold: row.threshold,
    alertInterval: row.alertInterval,
    severity: row.severity,
    enabled: row.enabled,
    description: row.description || ''
  })
  hostScope.value = row.hostId ? 'single' : 'all'
  hostKeyword.value = ''
  dialogVisible.value = true
}

const handleToggle = async (row: HostAlertRule, value: string | number | boolean) => {
  try {
    await updateHostAlertRule(row.id!, {
      ...row,
      enabled: Boolean(value)
    })
    ElMessage.success('状态已更新')
    loadData()
  } catch (error: any) {
    ElMessage.error('更新状态失败')
  }
}

const handleDelete = async (row: HostAlertRule) => {
  try {
    await ElMessageBox.confirm(`确定删除规则“${row.name}”吗？`, '提示', { type: 'warning' })
    await deleteHostAlertRule(row.id!)
    ElMessage.success('删除成功')
    loadData()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      if (hostScope.value === 'single' && !form.hostId) {
        ElMessage.error('请选择主机')
        return
      }
      const payload: HostAlertRule = {
        name: form.name,
        hostId: hostScope.value === 'single' ? (form.hostId || null) : null,
        channelIds: form.channelIds || [],
        metric: form.metric,
        threshold: form.threshold,
        alertInterval: form.alertInterval,
        severity: form.severity,
        enabled: form.enabled,
        description: form.description
      }
      if (form.id) {
        await updateHostAlertRule(form.id, payload)
        ElMessage.success('更新成功')
      } else {
        await createHostAlertRule(payload)
        ElMessage.success('创建成功')
      }
      dialogVisible.value = false
      loadData()
    } catch (error: any) {
      ElMessage.error(error?.response?.data?.message || '保存失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleDialogClosed = () => {
  formRef.value?.clearValidate()
  Object.assign(form, createDefaultForm())
  hostScope.value = 'all'
  hostKeyword.value = ''
}

watch(hostScope, (value) => {
  if (value === 'all') {
    form.hostId = 0
  }
})

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.host-alert-rules-container {
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
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}

.host-scope-field {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.host-picker-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.host-picker-scroll {
  max-height: 220px;
  overflow-y: auto;
  padding: 10px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}

.host-picker-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.host-picker-option {
  margin-right: 0;
  width: 100%;
}

.host-picker-option-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.4;
}

.host-picker-option-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.host-picker-option-name {
  color: #111827;
  font-weight: 600;
}

.host-picker-option-meta {
  color: #6b7280;
  font-size: 12px;
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

.table-wrapper {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.modern-table {
  width: 100%;
}

.form-hint {
  margin-left: 10px;
  color: #909399;
  font-size: 12px;
}

@media (max-width: 1100px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }

  .page-header {
    flex-direction: column;
    gap: 12px;
  }
}
</style>
