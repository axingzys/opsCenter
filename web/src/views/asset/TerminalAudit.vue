<template>
  <div class="terminal-audit-container">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><Monitor /></el-icon>
        </div>
        <div>
          <h2 class="page-title">会话审计</h2>
          <p class="page-subtitle">查看和管理 SSH 终端与 Windows 桌面会话录制</p>
        </div>
      </div>
    </div>

    <div class="audit-tabs-card">
      <el-tabs v-model="activeTab" class="audit-tabs">
        <el-tab-pane label="SSH终端" name="terminal">
          <div class="search-bar">
            <div class="search-inputs">
              <el-input
                v-model="searchKeyword"
                placeholder="搜索主机名、IP或用户名..."
                clearable
                class="search-input"
                @keyup.enter="loadSessions"
                @clear="loadSessions"
              >
                <template #prefix>
                  <el-icon class="search-icon"><Search /></el-icon>
                </template>
              </el-input>
            </div>

            <div class="search-actions">
              <el-button class="reset-btn" @click="handleRefresh">
                <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                重置
              </el-button>
            </div>
          </div>

          <div class="table-wrapper">
            <el-table
              :data="filteredSessions"
              v-loading="loading"
              class="modern-table"
              :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
            >
              <el-table-column prop="id" label="ID" width="80" align="center" />

              <el-table-column label="主机信息" min-width="220">
                <template #default="{ row }">
                  <div class="host-info">
                    <div class="host-name">
                      <el-icon><Monitor /></el-icon>
                      <span>{{ row.hostName }}</span>
                    </div>
                    <div class="host-ip">{{ row.hostIp }}</div>
                  </div>
                </template>
              </el-table-column>

              <el-table-column prop="username" label="操作用户" min-width="150" align="center">
                <template #default="{ row }">
                  <el-tooltip :content="row.username" placement="top">
                    <el-tag type="info" class="username-tag">
                      <el-icon><User /></el-icon>
                      <span class="username-text">{{ row.username }}</span>
                    </el-tag>
                  </el-tooltip>
                </template>
              </el-table-column>

              <el-table-column prop="durationText" label="时长" min-width="100" align="center" />

              <el-table-column prop="fileSizeText" label="文件大小" min-width="110" align="center" />

              <el-table-column label="录屏状态" min-width="160" align="center">
                <template #default="{ row }">
                  <el-tooltip :content="row.recordingAvailable ? '录屏文件可播放' : row.recordingIssue || '录屏不可用'" placement="top">
                    <el-tag :type="row.recordingAvailable ? 'success' : 'danger'">
                      {{ row.recordingAvailable ? '可播放' : '不可用' }}
                    </el-tag>
                  </el-tooltip>
                </template>
              </el-table-column>

              <el-table-column label="风险记录" min-width="170" align="center">
                <template #default="{ row }">
                  <div style="display: flex; gap: 6px; justify-content: center; flex-wrap: wrap;">
                    <el-tag v-if="row.highRiskCount > 0" type="danger">高危 {{ row.highRiskCount }}</el-tag>
                    <el-tag v-if="row.mediumRiskCount > 0" type="warning">中危 {{ row.mediumRiskCount }}</el-tag>
                    <span v-if="row.highRiskCount === 0 && row.mediumRiskCount === 0" style="color: #909399;">无</span>
                  </div>
                </template>
              </el-table-column>

              <el-table-column prop="statusText" label="状态" min-width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="getStatusType(row.status)">{{ row.statusText }}</el-tag>
                </template>
              </el-table-column>

              <el-table-column prop="startedAtText" label="开始时间" min-width="180" align="center" />

              <el-table-column prop="endedAtText" label="结束时间" min-width="180" align="center" />

              <el-table-column label="操作" width="240" align="center" fixed="right">
                <template #default="{ row }">
                  <div class="action-buttons">
                    <el-tooltip :content="row.recordingAvailable ? '播放' : (row.recordingIssue || '录屏不可用')" placement="top">
                      <el-button
                        link
                        class="action-btn action-play"
                        @click="handlePlay(row)"
                        :loading="playingSession === row.id"
                        :disabled="!row.recordingAvailable"
                      >
                        <el-icon><VideoPlay /></el-icon>
                      </el-button>
                    </el-tooltip>
                    <el-tooltip :content="row.recordingAvailable ? '下载' : (row.recordingIssue || '录屏不可用')" placement="top">
                      <el-button
                        link
                        class="action-btn action-download"
                        @click="handleDownload(row)"
                        :loading="downloadingSession === row.id"
                        :disabled="!row.recordingAvailable"
                      >
                        <el-icon><Download /></el-icon>
                      </el-button>
                    </el-tooltip>
                    <el-tooltip content="查看风险记录" placement="top">
                      <el-button
                        link
                        class="action-btn action-risk"
                        @click="handleOpenRiskRecords(row)"
                        :loading="loadingRiskSession === row.id"
                      >
                        <el-icon><Warning /></el-icon>
                      </el-button>
                    </el-tooltip>
                    <el-tooltip content="删除" placement="top">
                      <el-button
                        link
                        class="action-btn action-delete"
                        @click="handleDeleteClick(row)"
                        :loading="deletingSession === row.id"
                      >
                        <el-icon><Delete /></el-icon>
                      </el-button>
                    </el-tooltip>
                  </div>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-container">
              <el-pagination
                v-model:current-page="page"
                v-model:page-size="pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="total"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="handleSizeChange"
                @current-change="handlePageChange"
              />
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="Windows桌面" name="desktop" lazy>
          <DesktopSessionAuditTab />
        </el-tab-pane>
      </el-tabs>
    </div>

    <el-dialog
      v-model="playerVisible"
      :title="`终端回放 - ${currentSession?.hostName}`"
      width="80%"
      top="5vh"
      class="terminal-player-dialog responsive-dialog"
      :close-on-click-modal="false"
      @close="handlePlayerClose"
    >
      <AsciinemaPlayer
        v-if="recordingUrl && playerVisible"
        :src="recordingUrl"
        :autoplay="true"
        @error="handlePlayerError"
      />
    </el-dialog>

    <el-dialog
      v-model="riskDialogVisible"
      :title="`风险记录 - ${riskSession?.hostName || ''}`"
      width="860px"
      top="8vh"
      :close-on-click-modal="false"
    >
      <div v-if="riskSession" style="margin-bottom: 16px; display: flex; gap: 8px; flex-wrap: wrap;">
        <el-tag type="danger">高危 {{ riskSummary.highRiskCount }}</el-tag>
        <el-tag type="warning">中危 {{ riskSummary.mediumRiskCount }}</el-tag>
        <el-tag type="info">主机 {{ riskSession.hostIp }}</el-tag>
        <el-tag type="info">用户 {{ riskSession.username }}</el-tag>
      </div>

      <el-table :data="riskEvents" v-loading="riskLoading" empty-text="暂无高危或中危命令记录">
        <el-table-column prop="executedAtText" label="执行时间" min-width="170" align="center" />
        <el-table-column label="等级" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getRiskTagType(row.riskLevel)">{{ row.riskLevelText }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="commandText" label="命令" min-width="280" show-overflow-tooltip />
        <el-table-column prop="ruleName" label="命中规则" min-width="180" show-overflow-tooltip />
        <el-table-column prop="ruleDescription" label="说明" min-width="220" show-overflow-tooltip />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Delete,
  Download,
  Monitor,
  RefreshLeft,
  Search,
  User,
  VideoPlay,
  Warning,
} from '@element-plus/icons-vue'
import {
  deleteTerminalSession,
  downloadTerminalSession,
  getTerminalSessionEvents,
  getTerminalSessions,
  playTerminalSession,
} from '@/api/terminal'
import AsciinemaPlayer from '@/components/AsciinemaPlayer.vue'
import DesktopSessionAuditTab from '@/views/asset/components/DesktopSessionAuditTab.vue'

interface TerminalSession {
  id: number
  hostId: number
  hostName: string
  hostIp: string
  userId: number
  username: string
  duration: number
  durationText: string
  fileSize: number
  fileSizeText: string
  status: string
  statusText: string
  recordingAvailable: boolean
  recordingIssue: string
  highRiskCount: number
  mediumRiskCount: number
  topRiskLevel: string
  topRiskLevelText: string
  createdAt: string
  createdAtText: string
  startedAt: string
  startedAtText: string
  endedAt: string
  endedAtText: string
  closeReason?: string
}

interface TerminalCommandEvent {
  id: number
  sessionId: number
  commandText: string
  normalizedCommand: string
  riskLevel: string
  riskLevelText: string
  ruleCode: string
  ruleName: string
  ruleDescription: string
  source: string
  confidence: string
  executedAt: string
  executedAtText: string
}

const loading = ref(false)
const sessions = ref<TerminalSession[]>([])
const searchKeyword = ref('')
const activeTab = ref('terminal')
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const playerVisible = ref(false)
const recordingUrl = ref('')
const currentSession = ref<TerminalSession | null>(null)
const playingSession = ref(0)
const downloadingSession = ref(0)

const riskDialogVisible = ref(false)
const riskLoading = ref(false)
const loadingRiskSession = ref(0)
const riskSession = ref<TerminalSession | null>(null)
const riskEvents = ref<TerminalCommandEvent[]>([])
const riskSummary = ref({ highRiskCount: 0, mediumRiskCount: 0 })

const deletingSession = ref(0)

const filteredSessions = computed(() => {
  if (!searchKeyword.value) {
    return sessions.value
  }

  const keyword = searchKeyword.value.toLowerCase()
  return sessions.value.filter(item =>
    item.hostName?.toLowerCase().includes(keyword) ||
    item.hostIp?.toLowerCase().includes(keyword) ||
    item.username?.toLowerCase().includes(keyword)
  )
})

const extractErrorMessage = (error: any, fallback: string) => {
  const responseData = error?.response?.data
  if (typeof responseData === 'object' && responseData?.message) {
    return responseData.message
  }
  if (typeof responseData === 'string' && responseData.trim()) {
    try {
      const parsed = JSON.parse(responseData)
      if (parsed?.message) {
        return parsed.message
      }
    } catch {
      return responseData
    }
  }
  if (error?.message) {
    return error.message
  }
  return fallback
}

const cleanupRecordingUrl = () => {
  if (recordingUrl.value) {
    URL.revokeObjectURL(recordingUrl.value)
    recordingUrl.value = ''
  }
}

const loadSessions = async () => {
  loading.value = true
  try {
    const response = await getTerminalSessions({
      page: page.value,
      pageSize: pageSize.value,
      keyword: searchKeyword.value
    })
    sessions.value = response.list || []
    total.value = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载会话列表失败: ' + extractErrorMessage(error, '未知错误'))
  } finally {
    loading.value = false
  }
}

const handlePlay = async (session: TerminalSession) => {
  if (!session.recordingAvailable) {
    ElMessage.warning(session.recordingIssue || '录屏文件不可用')
    return
  }

  playingSession.value = session.id
  try {
    cleanupRecordingUrl()
    const response = await playTerminalSession(session.id)
    const blob = new Blob([response], { type: 'text/plain;charset=utf-8' })
    recordingUrl.value = URL.createObjectURL(blob)
    currentSession.value = session
    playerVisible.value = true
  } catch (error: any) {
    ElMessage.error('加载录制文件失败: ' + extractErrorMessage(error, '未知错误'))
  } finally {
    playingSession.value = 0
  }
}

const handleDownload = async (session: TerminalSession) => {
  if (!session.recordingAvailable) {
    ElMessage.warning(session.recordingIssue || '录屏文件不可用')
    return
  }

  downloadingSession.value = session.id
  try {
    const response = await downloadTerminalSession(session.id)
    const blobUrl = URL.createObjectURL(response)
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = buildDownloadName(session)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(blobUrl)
    ElMessage.success('录制文件已开始下载')
  } catch (error: any) {
    ElMessage.error('下载录制文件失败: ' + extractErrorMessage(error, '未知错误'))
  } finally {
    downloadingSession.value = 0
  }
}

const handleOpenRiskRecords = async (session: TerminalSession) => {
  loadingRiskSession.value = session.id
  riskLoading.value = true
  riskSession.value = session
  riskEvents.value = []
  riskSummary.value = {
    highRiskCount: session.highRiskCount || 0,
    mediumRiskCount: session.mediumRiskCount || 0,
  }
  try {
    const response = await getTerminalSessionEvents(session.id)
    riskEvents.value = response.list || []
    riskSummary.value = {
      highRiskCount: response.highRiskCount || 0,
      mediumRiskCount: response.mediumRiskCount || 0,
    }
    riskDialogVisible.value = true
  } catch (error: any) {
    ElMessage.error('加载风险记录失败: ' + extractErrorMessage(error, '未知错误'))
  } finally {
    riskLoading.value = false
    loadingRiskSession.value = 0
  }
}

const handleDeleteClick = (row: TerminalSession) => {
  ElMessageBox.confirm('确定删除此会话录制吗？相关高危命令记录也会一并删除。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await handleDelete(row.id)
  }).catch(() => {})
}

const handleDelete = async (id: number) => {
  deletingSession.value = id
  try {
    await deleteTerminalSession(id)
    ElMessage.success('删除成功')
    await loadSessions()
  } catch (error: any) {
    ElMessage.error('删除失败: ' + extractErrorMessage(error, '未知错误'))
  } finally {
    deletingSession.value = 0
  }
}

const buildDownloadName = (session: TerminalSession) => {
  const host = sanitizeFileName(session.hostName || session.hostIp || `terminal-session-${session.id}`)
  return `${host}-${session.id}.cast`
}

const sanitizeFileName = (value: string) => value.replace(/[\\/:*?"<>|\s]+/g, '_')

const handleRefresh = () => {
  searchKeyword.value = ''
  page.value = 1
  loadSessions()
}

const handleSizeChange = () => {
  page.value = 1
  loadSessions()
}

const handlePageChange = () => {
  loadSessions()
}

const handlePlayerClose = () => {
  cleanupRecordingUrl()
  currentSession.value = null
}

const handlePlayerError = (message: string) => {
  ElMessage.error(message || '终端播放器初始化失败')
}

const getStatusType = (status: string): 'success' | 'info' | 'warning' | 'danger' => {
  const typeMap: Record<string, 'success' | 'info' | 'warning' | 'danger'> = {
    completed: 'success',
    recording: 'warning',
    failed: 'danger',
    timeout: 'warning'
  }
  return typeMap[status] || 'info'
}

const getRiskTagType = (riskLevel: string): 'danger' | 'warning' | 'info' => {
  if (riskLevel === 'high') {
    return 'danger'
  }
  if (riskLevel === 'medium') {
    return 'warning'
  }
  return 'info'
}

onMounted(() => {
  loadSessions()
})

onBeforeUnmount(() => {
  cleanupRecordingUrl()
})
</script>
<style scoped>
.terminal-audit-container {
  padding: 0;
  background-color: transparent;
}

/* 页面头部 */
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
  line-height: 1.3;
}

.page-subtitle {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: #909399;
  line-height: 1.4;
}

.audit-tabs-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  padding: 0 16px 16px;
}

.audit-tabs-card :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.audit-tabs-card :deep(.el-tabs__nav-wrap::after) {
  background-color: #ebeef5;
}

.audit-tabs-card :deep(.el-tabs__item.is-active) {
  color: #303133;
  font-weight: 600;
}

.audit-tabs-card :deep(.el-tabs__active-bar) {
  background-color: #d4af37;
}

.search-bar {
  margin-bottom: 12px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.search-inputs {
  display: flex;
  gap: 12px;
  flex: 1;
}

.search-input {
  width: 280px;
}

.search-actions {
  display: flex;
  gap: 10px;
}

.reset-btn {
  background: #f5f7fa;
  border-color: #dcdfe6;
  color: #606266;
}

.reset-btn:hover {
  background: #e6e8eb;
  border-color: #c0c4cc;
}

.search-bar :deep(.el-input__wrapper) {
  border-radius: 8px;
  border: 1px solid #dcdfe6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  background-color: #fff;
}

.search-bar :deep(.el-input__wrapper:hover) {
  border-color: #d4af37;
  box-shadow: 0 2px 8px rgba(212, 175, 55, 0.15);
}

.search-bar :deep(.el-input__wrapper.is-focus) {
  border-color: #d4af37;
  box-shadow: 0 2px 12px rgba(212, 175, 55, 0.25);
}

.search-icon {
  color: #d4af37;
}

/* 表格容器 */
.table-wrapper {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  overflow: hidden;
}

.modern-table {
  width: 100%;
}

.modern-table :deep(.el-table__body-wrapper) {
  border-radius: 0 0 12px 12px;
}

.modern-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.modern-table :deep(.el-table__row:hover) {
  background-color: #f8fafc !important;
}

/* 主机信息 */
.host-info {
  .host-name {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 500;
    color: #303133;
    margin-bottom: 4px;

    :deep(.el-icon) {
      color: #409eff;
    }
  }

  .host-ip {
    font-size: 12px;
    color: #909399;
    font-family: 'Consolas', 'Monaco', monospace;
  }
}

/* 用户名标签 */
.username-tag {
  min-width: 120px;
  max-width: 130px;
  display: inline-flex !important;
  flex-direction: row !important;
  align-items: center !important;
  justify-content: center;
  gap: 6px;
  padding: 0 12px;

  :deep(.el-icon) {
    font-size: 14px;
    display: inline-block;
    vertical-align: middle;
    flex-shrink: 0;
  }

  .username-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }
}

:deep(.username-tag .el-tag__content) {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
  width: 100%;
  overflow: hidden;
}

/* 操作按钮 */
.action-buttons {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
}

.action-btn {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.action-btn :deep(.el-icon) {
  font-size: 16px;
}

.action-btn:hover {
  transform: scale(1.1);
}

.action-play:hover {
  background-color: #e8f4ff;
  color: #409eff;
}

.action-download:hover {
  background-color: #ecf5ff;
  color: #409eff;
}

.action-risk:hover {
  background-color: #fff7e6;
  color: #e6a23c;
}

.action-delete:hover {
  background-color: #fee;
  color: #f56c6c;
}

/* 分页 */
.pagination-container {
  padding: 12px 20px;
  background: #fff;
  border-top: 1px solid #f0f0f0;
  border-radius: 0 0 12px 12px;
  display: flex;
  justify-content: flex-end;
}

/* 播放对话框样式 */
:deep(.terminal-player-dialog) {
  border-radius: 12px;
}

:deep(.terminal-player-dialog .el-dialog__header) {
  padding: 20px 24px 16px;
  border-bottom: 1px solid #f0f0f0;
}

:deep(.terminal-player-dialog .el-dialog__body) {
  padding: 20px;
  background: #000;
}

:deep(.terminal-player-dialog .el-dialog__footer) {
  padding: 16px 24px;
  border-top: 1px solid #f0f0f0;
}

/* 标签样式 */
:deep(.el-tag) {
  border-radius: 6px;
  padding: 4px 10px;
  font-weight: 500;
}

/* 响应式对话框 */
:deep(.responsive-dialog) {
  max-width: 95vw;
  min-width: 500px;
}

@media (max-width: 768px) {
  :deep(.responsive-dialog .el-dialog) {
    width: 95% !important;
    max-width: none;
    min-width: auto;
  }

  .search-input {
    width: auto;
    flex: 1;
    min-width: 200px;
  }

  .search-inputs {
    flex-direction: column;
  }
}
</style>
