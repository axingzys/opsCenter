<template>
  <div class="desktop-page">
    <input
      ref="fileInputRef"
      class="desktop-file-input"
      type="file"
      multiple
      @change="handleFileInputChange"
    />

    <div class="desktop-header">
      <div class="desktop-title-group">
        <div class="desktop-title">{{ hostName || '远程桌面' }}</div>
        <div class="desktop-subtitle">
          <span>RDP / Guacamole</span>
          <span class="desktop-upload-tip">文件会上传到映射盘根目录</span>
        </div>
      </div>
      <div class="desktop-actions">
        <el-button :loading="uploading" @click="openFilePicker">上传文件</el-button>
        <el-button @click="handleFullscreen">全屏</el-button>
        <el-button type="danger" @click="handleClose">关闭</el-button>
      </div>
    </div>

    <div v-if="uploading" class="desktop-upload-banner">
      <div class="desktop-upload-banner-text">
        正在上传 {{ uploadingFileName }}
        <span v-if="uploadQueueTotal > 1">（{{ uploadQueueIndex }}/{{ uploadQueueTotal }}）</span>
      </div>
      <el-progress :percentage="uploadProgress" :stroke-width="6" />
    </div>

    <div v-if="errorMessage" class="desktop-error">
      <el-result icon="error" title="桌面启动失败" :sub-title="errorMessage">
        <template #extra>
          <el-button @click="handleClose">关闭页面</el-button>
        </template>
      </el-result>
    </div>

    <div v-else class="desktop-body">
      <iframe
        v-if="launchUrl"
        ref="iframeRef"
        class="desktop-iframe"
        :src="launchUrl"
        @load="handleIframeLoad"
        frameborder="0"
        allow="clipboard-read; clipboard-write"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { closeDesktopSession, heartbeatDesktopSession, uploadDesktopSessionFile } from '@/api/host'

const route = useRoute()
const router = useRouter()
const iframeRef = ref<HTMLIFrameElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const launchUrl = ref('')
const errorMessage = ref('')
const closeSubmitted = ref(false)
const uploading = ref(false)
const uploadingFileName = ref('')
const uploadProgress = ref(0)
const uploadQueueIndex = ref(0)
const uploadQueueTotal = ref(0)
const GUAC_AUTH_TOKEN_KEY = 'GUAC_AUTH_TOKEN'
let detachIframeDropGuards: (() => void) | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null

type FileSystemEntryLike = {
  isDirectory: boolean
}

type TransferItemWithEntry = DataTransferItem & {
  webkitGetAsEntry?: () => FileSystemEntryLike | null
  getAsEntry?: () => FileSystemEntryLike | null
}

const sessionId = computed(() => Number(route.query.sessionId || 0))
const hostName = computed(() => String(route.query.hostName || ''))

const buildCloseUrl = () => {
  const token = localStorage.getItem('token')
  return token && sessionId.value
    ? `/api/v1/desktop-sessions/${sessionId.value}/close?token=${encodeURIComponent(token)}`
    : ''
}

const stopHeartbeat = () => {
  if (!heartbeatTimer) return
  clearInterval(heartbeatTimer)
  heartbeatTimer = null
}

const sendHeartbeat = async () => {
  if (!sessionId.value || closeSubmitted.value) return
  try {
    await heartbeatDesktopSession(sessionId.value)
  } catch {
    // 心跳失败交给后端过期扫描处理，避免干扰桌面操作
  }
}

const startHeartbeat = () => {
  stopHeartbeat()
  void sendHeartbeat()
  heartbeatTimer = setInterval(() => {
    void sendHeartbeat()
  }, 30_000)
}

const submitCloseDuringUnload = () => {
  const closeUrl = buildCloseUrl()
  if (!closeUrl) return false

  if (navigator.sendBeacon) {
    try {
      if (navigator.sendBeacon(closeUrl, new Blob([], { type: 'text/plain;charset=UTF-8' }))) {
        return true
      }
    } catch {
      // 继续尝试 keepalive fetch
    }
  }

  try {
    fetch(closeUrl, {
      method: 'POST',
      credentials: 'include',
      keepalive: true
    }).catch(() => {})
    return true
  } catch {
    return false
  }
}

type CloseMode = 'normal' | 'unload'

const closeSession = async (mode: CloseMode = 'normal') => {
  if (closeSubmitted.value || !sessionId.value) return
  stopHeartbeat()

  if (mode === 'unload' && submitCloseDuringUnload()) {
    closeSubmitted.value = true
    localStorage.removeItem(GUAC_AUTH_TOKEN_KEY)
    return
  }

  try {
    await closeDesktopSession(sessionId.value)
    closeSubmitted.value = true
  } catch {
    // 页面关闭时忽略关闭失败
  }

  localStorage.removeItem(GUAC_AUTH_TOKEN_KEY)
}

const hasDirectoryDrop = (dataTransfer: DataTransfer | null) => {
  if (!dataTransfer?.items?.length) return false

  for (const item of Array.from(dataTransfer.items)) {
    if (item.kind !== 'file') continue

    const itemWithEntry = item as TransferItemWithEntry
    const getEntry = itemWithEntry.webkitGetAsEntry || itemWithEntry.getAsEntry
    if (typeof getEntry !== 'function') continue

    const entry = getEntry.call(itemWithEntry)
    if (entry?.isDirectory) return true
  }

  return false
}

const hasFileDrop = (dataTransfer: DataTransfer | null) => {
  if (dataTransfer?.files?.length) return true
  if (!dataTransfer?.items?.length) return false
  return Array.from(dataTransfer.items).some((item) => item.kind === 'file')
}

const getDroppedFiles = (dataTransfer: DataTransfer | null) => {
  if (!dataTransfer?.files?.length) return []
  return Array.from(dataTransfer.files)
}

const stopDropEvent = (event: DragEvent) => {
  event.preventDefault()
  event.stopPropagation()
  if (typeof event.stopImmediatePropagation === 'function') {
    event.stopImmediatePropagation()
  }
}

const clearGuacamoleDropState = (iframeDocument: Document) => {
  iframeDocument.querySelectorAll('.drop-pending').forEach((element) => {
    element.classList.remove('drop-pending')
  })
}

const openFilePicker = () => {
  if (!sessionId.value) {
    ElMessage.error('桌面会话不存在，无法上传文件')
    return
  }
  fileInputRef.value?.click()
}

const uploadFiles = async (files: File[]) => {
  if (!files.length) return
  if (!sessionId.value) {
    ElMessage.error('桌面会话不存在，无法上传文件')
    return
  }
  if (uploading.value) {
    ElMessage.warning('已有文件正在上传，请稍候')
    return
  }

  uploading.value = true
  uploadQueueTotal.value = files.length

  try {
    for (const [index, file] of files.entries()) {
      uploadQueueIndex.value = index + 1
      uploadingFileName.value = file.name
      uploadProgress.value = 0

      await uploadDesktopSessionFile(sessionId.value, file, (event) => {
        const total = event.total || file.size || 0
        if (!total) return
        uploadProgress.value = Math.max(1, Math.min(100, Math.round((event.loaded / total) * 100)))
      })

      uploadProgress.value = 100
    }

    ElMessage.success('文件已上传到映射盘根目录，请在 Windows 资源管理器中刷新查看')
  } catch {
    // 请求层已统一提示错误，这里只负责重置上传状态
  } finally {
    uploading.value = false
    uploadingFileName.value = ''
    uploadProgress.value = 0
    uploadQueueIndex.value = 0
    uploadQueueTotal.value = 0
  }
}

const handleFileInputChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  await uploadFiles(files)
}

const installIframeDropGuards = () => {
  detachIframeDropGuards?.()
  detachIframeDropGuards = null

  const iframeWindow = iframeRef.value?.contentWindow
  const iframeDocument = iframeWindow?.document
  if (!iframeWindow || !iframeDocument) return

  const handleDragEnter = (event: DragEvent) => {
    if (!hasFileDrop(event.dataTransfer)) return
    stopDropEvent(event)
    clearGuacamoleDropState(iframeDocument)
  }

  const handleDragOver = (event: DragEvent) => {
    if (!hasFileDrop(event.dataTransfer)) return
    stopDropEvent(event)
    clearGuacamoleDropState(iframeDocument)
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = hasDirectoryDrop(event.dataTransfer) ? 'none' : 'copy'
    }
  }

  const handleDragLeave = (event: DragEvent) => {
    if (!hasFileDrop(event.dataTransfer)) return
    stopDropEvent(event)
    clearGuacamoleDropState(iframeDocument)
  }

  const handleDrop = (event: DragEvent) => {
    if (!hasFileDrop(event.dataTransfer)) return
    stopDropEvent(event)
    clearGuacamoleDropState(iframeDocument)
    if (hasDirectoryDrop(event.dataTransfer)) {
      ElMessage.warning('暂不支持拖拽上传文件夹，请先压缩后再上传')
      return
    }

    const files = getDroppedFiles(event.dataTransfer)
    if (!files.length) return

    void uploadFiles(files)
  }

  iframeDocument.addEventListener('dragenter', handleDragEnter, true)
  iframeDocument.addEventListener('dragover', handleDragOver, true)
  iframeDocument.addEventListener('dragleave', handleDragLeave, true)
  iframeDocument.addEventListener('drop', handleDrop, true)

  detachIframeDropGuards = () => {
    iframeDocument.removeEventListener('dragenter', handleDragEnter, true)
    iframeDocument.removeEventListener('dragover', handleDragOver, true)
    iframeDocument.removeEventListener('dragleave', handleDragLeave, true)
    iframeDocument.removeEventListener('drop', handleDrop, true)
  }
}

const handleClose = async () => {
  await closeSession('normal')
  window.close()
  router.push('/asset/hosts')
}

const handleFullscreen = async () => {
  const element = iframeRef.value
  if (!element) return
  if (document.fullscreenElement) {
    await document.exitFullscreen()
    return
  }
  await element.requestFullscreen()
}

const handleIframeLoad = () => {
  try {
    installIframeDropGuards()
  } catch {
    // Guacamole 页面如果不在同源上下文，无法注入拖拽保护，静默跳过
  }
}

const handleBeforeUnload = () => {
  void closeSession('unload')
}

const handlePageHide = () => {
  void closeSession('unload')
}

onMounted(() => {
  window.addEventListener('beforeunload', handleBeforeUnload)
  window.addEventListener('pagehide', handlePageHide)

  if (!sessionId.value) {
    errorMessage.value = '缺少桌面会话 ID'
    return
  }

  const storageKey = `desktop-launch:${sessionId.value}`
  const storedLaunchUrl = localStorage.getItem(storageKey)
  if (!storedLaunchUrl) {
    errorMessage.value = '桌面启动地址不存在或已过期，请重新发起连接'
    return
  }

  localStorage.removeItem(GUAC_AUTH_TOKEN_KEY)
  launchUrl.value = storedLaunchUrl
  localStorage.removeItem(storageKey)
  startHeartbeat()
})

onBeforeUnmount(() => {
  detachIframeDropGuards?.()
  detachIframeDropGuards = null
  window.removeEventListener('beforeunload', handleBeforeUnload)
  window.removeEventListener('pagehide', handlePageHide)
  void closeSession('unload')
})
</script>

<style scoped>
.desktop-page {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #0f172a;
}

.desktop-header {
  height: 64px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.92);
  color: #e2e8f0;
}

.desktop-title {
  font-size: 16px;
  font-weight: 600;
}

.desktop-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: #94a3b8;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.desktop-actions {
  display: flex;
  gap: 12px;
}

.desktop-upload-tip {
  color: #60a5fa;
}

.desktop-upload-banner {
  padding: 12px 20px;
  background: rgba(15, 23, 42, 0.96);
  border-bottom: 1px solid rgba(148, 163, 184, 0.16);
}

.desktop-upload-banner-text {
  margin-bottom: 8px;
  font-size: 13px;
  color: #cbd5e1;
}

.desktop-body {
  flex: 1;
  min-height: 0;
}

.desktop-iframe {
  width: 100%;
  height: 100%;
  border: 0;
  background: #020617;
}

.desktop-error {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8fafc;
}

.desktop-file-input {
  display: none;
}
</style>
