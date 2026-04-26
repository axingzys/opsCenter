<template>
  <div class="desktop-audit-tab">
    <el-alert
      class="desktop-audit-tip"
      type="info"
      :closable="false"
      show-icon
      title="Windows 桌面当前支持录屏文件下载与在线回放。"
    />

    <div class="search-bar">
      <div class="search-inputs">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索主机名、IP、用户名或会话 UUID..."
          clearable
          class="search-input"
          @keyup.enter="loadSessions"
          @clear="loadSessions"
        >
          <template #prefix>
            <el-icon class="search-icon"><Search /></el-icon>
          </template>
        </el-input>

        <el-select
          v-model="status"
          placeholder="会话状态"
          clearable
          class="status-select"
          @change="handleFilterChange"
        >
          <el-option label="全部状态" value="" />
          <el-option label="创建中" value="creating" />
          <el-option label="进行中" value="active" />
          <el-option label="已关闭" value="closed" />
          <el-option label="失败" value="failed" />
          <el-option label="超时" value="timeout" />
        </el-select>
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
        :data="sessions"
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
                <span>{{ row.hostName || '-' }}</span>
              </div>
              <div class="host-ip">{{ row.hostIp || '-' }}</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="会话信息" min-width="220">
          <template #default="{ row }">
            <div class="session-info">
              <div class="session-uuid">{{ row.sessionUuid }}</div>
              <div class="session-meta">
                <span>{{ row.protocol?.toUpperCase() || 'RDP' }}</span>
                <span>{{ row.resolution || '默认分辨率' }}</span>
              </div>
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

        <el-table-column label="状态" min-width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="时长" min-width="110" align="center">
          <template #default="{ row }">
            {{ formatDuration(row.durationSeconds) }}
          </template>
        </el-table-column>

        <el-table-column label="录屏文件" min-width="120" align="center">
          <template #default="{ row }">
            <span v-if="row.recordingAvailable">{{ formatFileSize(row.fileSize) }}</span>
            <span v-else class="muted-text">无录屏</span>
          </template>
        </el-table-column>

        <el-table-column prop="startedAt" label="开始时间" min-width="180" align="center">
          <template #default="{ row }">
            {{ row.startedAt || '-' }}
          </template>
        </el-table-column>

        <el-table-column prop="endedAt" label="结束时间" min-width="180" align="center">
          <template #default="{ row }">
            {{ row.endedAt || '-' }}
          </template>
        </el-table-column>

        <el-table-column label="操作" width="190" align="center" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-tooltip :content="row.recordingAvailable ? '在线回放' : '暂无录屏文件'" placement="top">
                <el-button
                  link
                  class="action-btn action-play"
                  :disabled="!row.recordingAvailable"
                  @click="handlePlay(row)"
                >
                  <el-icon><VideoPlay /></el-icon>
                </el-button>
              </el-tooltip>

              <el-tooltip :content="row.recordingAvailable ? '下载录屏' : '暂无录屏文件'" placement="top">
                <el-button
                  link
                  class="action-btn action-download"
                  :disabled="!row.recordingAvailable"
                  :loading="downloadingSession === row.id"
                  @click="handleDownload(row)"
                >
                  <el-icon><Download /></el-icon>
                </el-button>
              </el-tooltip>

              <el-tooltip content="删除会话" placement="top">
                <el-button
                  link
                  class="action-btn action-delete"
                  :loading="deletingSession === row.id"
                  @click="handleDelete(row)"
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

    <el-dialog
      v-model="playbackVisible"
      :title="playbackTitle"
      width="90%"
      top="4vh"
      destroy-on-close
      :close-on-click-modal="false"
      class="desktop-playback-dialog responsive-dialog"
      @closed="currentPlaybackSession = null"
    >
      <GuacamoleRecordingPlayer
        v-if="currentPlaybackSession"
        :src="`/api/v1/desktop-sessions/${currentPlaybackSession.id}/recording/play`"
        :autoplay="true"
      />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, Monitor, RefreshLeft, Search, User, VideoPlay } from '@element-plus/icons-vue'
import { deleteDesktopSession, downloadDesktopSessionRecording, getDesktopSessions } from '@/api/host'
import GuacamoleRecordingPlayer from '@/views/asset/components/GuacamoleRecordingPlayer.vue'

interface DesktopSession {
  id: number
  sessionUuid: string
  hostId: number
  hostName: string
  hostIp: string
  userId: number
  username: string
  provider: string
  protocol: string
  status: string
  clientIp: string
  resolution: string
  recordingPath: string
  recordingAvailable: boolean
  fileSize: number
  durationSeconds: number
  startedAt?: string
  endedAt?: string
  closeReason?: string
  createTime: string
  updateTime: string
}

const loading = ref(false)
const downloadingSession = ref(0)
const deletingSession = ref(0)
const sessions = ref<DesktopSession[]>([])
const searchKeyword = ref('')
const status = ref('')
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const playbackVisible = ref(false)
const currentPlaybackSession = ref<DesktopSession | null>(null)

const playbackTitle = computed(() => {
  if (!currentPlaybackSession.value) {
    return '桌面录屏回放'
  }

  return `桌面录屏回放 - ${currentPlaybackSession.value.hostName || currentPlaybackSession.value.hostIp || currentPlaybackSession.value.sessionUuid}`
})

const loadSessions = async () => {
  loading.value = true
  try {
    const response = await getDesktopSessions({
      page: page.value,
      pageSize: pageSize.value,
      keyword: searchKeyword.value || undefined,
      status: status.value || undefined,
      scope: 'all'
    })
    sessions.value = response.list || []
    total.value = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载桌面会话失败: ' + (error.message || '未知错误'))
  } finally {
    loading.value = false
  }
}

const handleFilterChange = () => {
  page.value = 1
  loadSessions()
}

const handleRefresh = () => {
  searchKeyword.value = ''
  status.value = ''
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

const handleDownload = async (row: DesktopSession) => {
  if (!row.recordingAvailable) {
    ElMessage.warning('该桌面会话暂无录屏文件')
    return
  }

  downloadingSession.value = row.id
  try {
    const blob = await downloadDesktopSessionRecording(row.id)
    const downloadUrl = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = downloadUrl
    link.download = buildDownloadName(row)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(downloadUrl)
    ElMessage.success('录屏文件已开始下载')
  } catch (error: any) {
    ElMessage.error('下载录屏文件失败: ' + (error.message || '未知错误'))
  } finally {
    downloadingSession.value = 0
  }
}

const handlePlay = (row: DesktopSession) => {
  if (!row.recordingAvailable) {
    ElMessage.warning('该桌面会话暂无录屏文件')
    return
  }

  currentPlaybackSession.value = row
  playbackVisible.value = true
}

const handleDelete = async (row: DesktopSession) => {
  ElMessageBox.confirm('确定删除此桌面会话及其录屏文件吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    deletingSession.value = row.id
    try {
      await deleteDesktopSession(row.id)
      if (currentPlaybackSession.value?.id === row.id) {
        playbackVisible.value = false
        currentPlaybackSession.value = null
      }
      ElMessage.success('桌面会话删除成功')
      loadSessions()
    } catch (error: any) {
      ElMessage.error('删除桌面会话失败: ' + (error.message || '未知错误'))
    } finally {
      deletingSession.value = 0
    }
  }).catch(() => {})
}

const buildDownloadName = (row: DesktopSession) => {
  const host = sanitizeFileName(row.hostName || row.hostIp || `desktop-session-${row.id}`)
  const session = sanitizeFileName(row.sessionUuid || String(row.id))
  return `${host}-${session}.guac`
}

const sanitizeFileName = (value: string) => value.replace(/[\\/:*?"<>|\s]+/g, '_')

const formatDuration = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-'
  }
  if (seconds < 60) {
    return `${seconds}s`
  }
  if (seconds < 3600) {
    const minutes = Math.floor(seconds / 60)
    const remainSeconds = seconds % 60
    return `${minutes}m ${remainSeconds}s`
  }
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${minutes}m`
}

const formatFileSize = (size?: number) => {
  if (!size || size <= 0) {
    return '-'
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

const getStatusText = (sessionStatus: string) => {
  const statusMap: Record<string, string> = {
    creating: '创建中',
    active: '进行中',
    closed: '已关闭',
    failed: '失败',
    timeout: '超时'
  }
  return statusMap[sessionStatus] || sessionStatus || '-'
}

const getStatusType = (sessionStatus: string): 'success' | 'info' | 'warning' | 'danger' => {
  const typeMap: Record<string, 'success' | 'info' | 'warning' | 'danger'> = {
    active: 'success',
    creating: 'warning',
    closed: 'info',
    failed: 'danger',
    timeout: 'warning'
  }
  return typeMap[sessionStatus] || 'info'
}

onMounted(() => {
  loadSessions()
})
</script>

<style scoped>
.desktop-audit-tab {
  padding-top: 4px;
}

.desktop-audit-tip {
  margin-bottom: 12px;
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
  width: 320px;
}

.status-select {
  width: 160px;
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

.search-bar :deep(.el-input__wrapper),
.search-bar :deep(.el-select__wrapper) {
  border-radius: 8px;
  border: 1px solid #dcdfe6;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  background-color: #fff;
}

.search-bar :deep(.el-input__wrapper:hover),
.search-bar :deep(.el-select__wrapper:hover) {
  border-color: #d4af37;
  box-shadow: 0 2px 8px rgba(212, 175, 55, 0.15);
}

.search-bar :deep(.el-input__wrapper.is-focus),
.search-bar :deep(.el-select__wrapper.is-focused) {
  border-color: #d4af37;
  box-shadow: 0 2px 12px rgba(212, 175, 55, 0.25);
}

.search-icon {
  color: #d4af37;
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

.modern-table :deep(.el-table__row:hover) {
  background-color: #f8fafc !important;
}

.host-info,
.session-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.host-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  color: #303133;
}

.host-name :deep(.el-icon) {
  color: #409eff;
}

.host-ip,
.session-meta {
  font-size: 12px;
  color: #909399;
}

.session-uuid {
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}

.session-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.username-tag {
  min-width: 120px;
  max-width: 130px;
  display: inline-flex !important;
  flex-direction: row !important;
  align-items: center !important;
  justify-content: center;
  gap: 6px;
  padding: 0 12px;
}

.username-tag :deep(.el-icon) {
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

:deep(.username-tag .el-tag__content) {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
  width: 100%;
  overflow: hidden;
}

.muted-text {
  color: #909399;
}

.action-btn {
  width: 32px;
  height: 32px;
  border-radius: 6px;
}

.action-buttons {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
}

.action-play {
  color: #67c23a;
}

.action-download {
  color: #409eff;
}

.action-delete {
  color: #f56c6c;
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  padding: 16px 20px;
}

:deep(.desktop-playback-dialog .el-dialog__body) {
  padding: 16px;
}

@media (max-width: 768px) {
  .search-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .search-inputs {
    flex-direction: column;
  }

  .search-input,
  .status-select {
    width: 100%;
  }

  .pagination-container {
    justify-content: center;
    overflow-x: auto;
  }
}
</style>
