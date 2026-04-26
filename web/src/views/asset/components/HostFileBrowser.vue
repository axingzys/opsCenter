<template>
  <el-dialog
    v-model="dialogVisible"
    :title="`文件管理 - ${hostName}`"
    width="1000px"
    :close-on-click-modal="false"
    @close="handleClose"
    class="file-browser-dialog"
  >
    <div v-if="loading" class="loading-container">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <span class="loading-text">加载中...</span>
    </div>

    <div v-else class="file-browser">
      <!-- 路径导航 -->
      <div class="breadcrumb-card">
        <div class="breadcrumb-header">
          <el-icon class="location-icon"><Location /></el-icon>
          <span class="breadcrumb-title">当前位置</span>
        </div>
        <el-breadcrumb separator="/" class="path-breadcrumb">
          <el-breadcrumb-item @click="navigateToRoot" class="breadcrumb-home">
            <el-icon><HomeFilled /></el-icon>
            <span>{{ rootLabel }}</span>
          </el-breadcrumb-item>
          <el-breadcrumb-item
            v-for="(segment, index) in pathSegments"
            :key="index"
            @click="navigateToSegment(index)"
            class="breadcrumb-segment"
          >
            {{ segment }}
          </el-breadcrumb-item>
        </el-breadcrumb>
        <div class="current-path-display">
          <el-icon class="path-icon"><FolderOpened /></el-icon>
          <el-input
            v-model="pathInput"
            :placeholder="pathPlaceholder"
            class="path-input"
            @keyup.enter="handlePathInput"
            clearable
          >
            <template #prefix>
              <code class="path-prefix">路径:</code>
            </template>
          </el-input>
        </div>
      </div>

      <!-- 工具栏 -->
      <div class="toolbar-actions">
        <div class="toolbar-left">
          <el-button size="default" @click="refreshFiles" :loading="loading" class="toolbar-btn">
            <el-icon><Refresh /></el-icon>
            <span>刷新</span>
          </el-button>
          <el-button
            size="default"
            @click="navigateUp"
            :disabled="isAtRoot"
            class="toolbar-btn"
          >
            <el-icon><Back /></el-icon>
            <span>返回上级</span>
          </el-button>
        </div>
        <div class="toolbar-right">
          <el-upload
            :action="uploadUrl"
            :headers="uploadHeaders"
            :data="uploadData"
            :show-file-list="false"
            :http-request="handleCustomUpload"
            :before-upload="beforeUpload"
            :disabled="!canUpload"
          >
            <el-button size="default" :loading="uploading" :disabled="!canUpload" class="upload-btn">
              <el-icon><Upload /></el-icon>
              <span>上传文件</span>
            </el-button>
          </el-upload>
        </div>
      </div>

      <!-- 上传进度条 -->
      <div v-if="uploading" class="upload-progress-container">
        <div class="upload-info">
          <el-icon class="upload-icon"><Upload /></el-icon>
          <span class="upload-filename">{{ uploadingFileName }}</span>
          <span class="upload-status">{{ uploadStatusText }}</span>
        </div>
        <el-progress
          :percentage="uploadProgress"
          :stroke-width="8"
          :status="uploadProgress === 100 ? 'success' : undefined"
          :indeterminate="isProcessing"
        />
      </div>

      <!-- 文件列表 -->
      <div class="file-list-container">
        <el-table
          :data="files"
          class="file-table"
          v-loading="loading"
          stripe
          :header-cell-style="{ background: '#f5f7fa', color: '#606266', fontWeight: '600' }"
        >
          <el-table-column width="60" align="center">
            <template #default="{ row }">
              <div class="file-icon-wrapper">
                <el-icon :size="24" :class="getFileIconClass(row)">
                  <Folder v-if="row.isDir" />
                  <Document v-else />
                </el-icon>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="名称" min-width="280">
            <template #default="{ row }">
              <div
                class="file-name-cell"
                :class="{ 'is-directory': row.isDir }"
                @click="handleFileClick(row)"
              >
                <span class="file-name-text">{{ row.name }}</span>
                <el-tag v-if="row.isDir" size="small" type="info" class="dir-tag">目录</el-tag>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="大小" width="120" align="right">
            <template #default="{ row }">
              <span class="file-size">{{ row.isDir ? '-' : formatSize(row.size) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="permissionColumnLabel" width="130" align="center">
            <template #default="{ row }">
              <el-tag class="permission-tag" size="small">{{ row.mode || '-' }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="修改时间" width="180">
            <template #default="{ row }">
              <div class="time-cell">
                <el-icon class="time-icon"><Clock /></el-icon>
                <span>{{ row.modTime }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="180" align="center" fixed="right">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button
                  type="primary"
                  link
                  size="small"
                  @click="downloadFile(row)"
                  :loading="downloadingFiles[getDownloadKey(row)]"
                  class="action-btn"
                >
                  <el-icon><Download /></el-icon>
                  <span>{{ row.isDir ? '下载目录' : '下载' }}</span>
                </el-button>
                <el-popconfirm
                  title="确定删除此文件吗?"
                  @confirm="deleteFile(row)"
                  v-if="!row.isDir"
                  width="200"
                >
                  <template #reference>
                    <el-button
                      type="danger"
                      link
                      size="small"
                      :loading="deletingFiles[row.name]"
                      class="action-btn"
                    >
                      <el-icon><Delete /></el-icon>
                      <span>删除</span>
                    </el-button>
                  </template>
                </el-popconfirm>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <el-empty
          v-if="!loading && files.length === 0"
          description="此目录为空"
          :image-size="120"
        >
          <template #image>
            <el-icon :size="80" color="#c0c4cc"><FolderOpened /></el-icon>
          </template>
        </el-empty>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Loading,
  HomeFilled,
  Refresh,
  Back,
  Folder,
  Document,
  Download,
  Upload,
  Delete,
  Clock,
  Location,
  FolderOpened
} from '@element-plus/icons-vue'
import { listHostFiles, downloadHostFile, deleteHostFile } from '@/api/host'

interface FileInfo {
  name: string
  path?: string
  size: number
  mode: string
  isDir: boolean
  modTime: string
}

const props = defineProps<{
  visible: boolean
  hostId: number
  hostName: string
  hostOsType?: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const loading = ref(false)
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadingFileName = ref('')
const isProcessing = ref(false) // 是否在服务器处理中
const downloadingFiles = ref<Record<string, boolean>>({})
const deletingFiles = ref<Record<string, boolean>>({})
const files = ref<FileInfo[]>([])
const currentPath = ref('')
const pathInput = ref('')

const isWindowsHost = computed(() => props.hostOsType === 'windows')
const rootPath = computed(() => isWindowsHost.value ? '' : '~')
const rootLabel = computed(() => isWindowsHost.value ? '驱动器' : '主目录')
const pathPlaceholder = computed(() => isWindowsHost.value ? '输入 Windows 路径，如 C:\\Users\\axing' : '输入路径后按回车跳转')
const permissionColumnLabel = computed(() => isWindowsHost.value ? '属性' : '权限')
const isAtRoot = computed(() => isWindowsHost.value ? currentPath.value === '' : currentPath.value === '~' || currentPath.value === '/')
const canUpload = computed(() => !(isWindowsHost.value && currentPath.value === ''))

// 计算上传状态文本
const uploadStatusText = computed(() => {
  if (isProcessing.value) {
    return '服务器处理中...'
  } else if (uploadProgress.value < 100) {
    return `上传中 ${uploadProgress.value}%`
  } else {
    return '上传完成'
  }
})

const pathSegments = computed(() => {
  if (isAtRoot.value) return []
  if (isWindowsHost.value) {
    const normalized = normalizeWindowsPath(currentPath.value)
    return normalized ? normalized.split('\\').filter(Boolean) : []
  }
  const path = currentPath.value.startsWith('/') ? currentPath.value.slice(1) : currentPath.value.replace(/^~\/?/, '')
  return path ? path.split('/').filter(p => p) : []
})

// 上传相关
const uploadUrl = computed(() => {
  return `/api/v1/hosts/${props.hostId}/files/upload`
})

const uploadHeaders = computed(() => {
  const token = localStorage.getItem('token')
  return {
    Authorization: `Bearer ${token}`
  }
})

const uploadData = computed(() => {
  return {
    path: currentPath.value
  }
})

const getFileIconClass = (file: FileInfo) => {
  if (file.isDir) {
    return 'icon-directory'
  }
  return 'icon-file'
}

const formatSize = (size: number): string => {
  if (!size || size === 0) return '-'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(size) / Math.log(k))
  return Math.round((size / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

const loadFiles = async (path: string = rootPath.value) => {
  loading.value = true
  try {
    const response = await listHostFiles(props.hostId, path)
    // 响应拦截器已经返回了 data，所以直接使用 response
    files.value = response || []
  } catch (error: any) {
    ElMessage.error('获取文件列表失败: ' + (error.message || '未知错误'))
  } finally {
    loading.value = false
  }
}

const refreshFiles = () => {
  loadFiles(currentPath.value)
}

const navigateTo = (path: string) => {
  const nextPath = isWindowsHost.value ? normalizeWindowsPath(path) : path
  currentPath.value = nextPath
  pathInput.value = nextPath
  loadFiles(nextPath)
}

const navigateToRoot = () => {
  navigateTo(rootPath.value)
}

const handlePathInput = () => {
  if (!pathInput.value || pathInput.value.trim() === '') {
    if (isWindowsHost.value) {
      navigateToRoot()
      return
    }
    ElMessage.warning('请输入有效的路径')
    return
  }
  navigateTo(pathInput.value.trim())
}

const navigateUp = () => {
  if (isAtRoot.value) return

  if (isWindowsHost.value) {
    navigateTo(getWindowsParentPath(currentPath.value))
    return
  }

  const segments = currentPath.value.split('/').filter(s => s && s !== '~')

  if (segments.length === 0) {
    // 如果没有分段了，返回主目录或根目录
    navigateTo(currentPath.value.startsWith('/') ? '/' : '~')
    return
  }

  segments.pop()

  if (currentPath.value.startsWith('/')) {
    // 绝对路径
    const parentPath = segments.length > 0 ? '/' + segments.join('/') : '/'
    navigateTo(parentPath)
  } else {
    // 相对路径（主目录）
    const parentPath = segments.length > 0 ? '~/' + segments.join('/') : '~'
    navigateTo(parentPath)
  }
}

const navigateToSegment = (index: number) => {
  if (isWindowsHost.value) {
    const segments = pathSegments.value.slice(0, index + 1)
    if (segments.length === 0) {
      navigateToRoot()
      return
    }
    const [drive, ...rest] = segments
    const path = rest.length > 0 ? `${drive}\\${rest.join('\\')}` : `${drive}\\`
    navigateTo(path)
    return
  }

  const segments = pathSegments.value.slice(0, index + 1)
  const path = currentPath.value.startsWith('/')
    ? '/' + segments.join('/')
    : '~/' + segments.join('/')
  navigateTo(path)
}

const handleFileClick = (file: FileInfo) => {
  if (file.isDir) {
    navigateTo(resolveFilePath(file))
  }
}

const beforeUpload = (file: File) => {
  uploading.value = true
  uploadProgress.value = 0
  uploadingFileName.value = file.name
  isProcessing.value = false
  return true
}

const handleCustomUpload = async (options: any) => {
  const { file } = options

  const formData = new FormData()
  formData.append('file', file)
  formData.append('path', currentPath.value)

  const token = localStorage.getItem('token')

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()

    // 监听上传进度
    xhr.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) {
        // 限制在95%，剩余5%表示服务器处理
        const percentComplete = Math.min(Math.round((event.loaded / event.total) * 95), 95)
        uploadProgress.value = percentComplete
      }
    })

    // 上传完成，开始处理
    xhr.upload.addEventListener('load', () => {
      uploadProgress.value = 95
      isProcessing.value = true
    })

    // 监听完成
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        let responseCode = 0
        let responseMessage = ''
        try {
          const response = JSON.parse(xhr.responseText)
          responseCode = response.code ?? 0
          responseMessage = response.message || ''
        } catch (e) {
          responseCode = 0
        }

        if (responseCode !== 0) {
          uploading.value = false
          uploadProgress.value = 0
          uploadingFileName.value = ''
          isProcessing.value = false
          ElMessage.error('文件上传失败: ' + (responseMessage || '未知错误'))
          reject(new Error(responseMessage || '未知错误'))
          return
        }

        // 服务器处理完成
        isProcessing.value = false
        uploadProgress.value = 100

        setTimeout(() => {
          uploading.value = false
          uploadProgress.value = 0
          uploadingFileName.value = ''
          ElMessage.success('文件上传成功')
          refreshFiles()
          resolve(xhr.response)
        }, 500)
      } else {
        uploading.value = false
        uploadProgress.value = 0
        uploadingFileName.value = ''
        isProcessing.value = false

        let errorMsg = '未知错误'
        try {
          const response = JSON.parse(xhr.responseText)
          errorMsg = response.message || errorMsg
        } catch (e) {
          errorMsg = xhr.statusText || errorMsg
        }

        ElMessage.error('文件上传失败: ' + errorMsg)
        reject(new Error(errorMsg))
      }
    })

    // 监听错误
    xhr.addEventListener('error', () => {
      uploading.value = false
      uploadProgress.value = 0
      uploadingFileName.value = ''
      isProcessing.value = false
      ElMessage.error('文件上传失败: 网络错误')
      reject(new Error('网络错误'))
    })

    // 监听中止
    xhr.addEventListener('abort', () => {
      uploading.value = false
      uploadProgress.value = 0
      uploadingFileName.value = ''
      isProcessing.value = false
      ElMessage.warning('文件上传已取消')
      reject(new Error('上传已取消'))
    })

    // 打开请求
    xhr.open('POST', `/api/v1/hosts/${props.hostId}/files/upload`)

    // 设置请求头
    if (token) {
      xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    }

    // 发送请求
    xhr.send(formData)
  })
}

const downloadFile = async (file: FileInfo) => {
  const downloadKey = getDownloadKey(file)
  downloadingFiles.value[downloadKey] = true
  try {
    if (file.isDir) {
      ElMessage.info('目录正在打包中，请稍候...')
    }

    const response = await downloadHostFile(props.hostId, resolveFilePath(file))
    const downloadName = file.isDir ? ensureArchiveName(file.name) : file.name
    const blob = response instanceof Blob ? response : new Blob([response])

    // 创建下载链接
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', downloadName)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    ElMessage.success(file.isDir ? '目录下载成功' : '文件下载成功')
  } catch (error: any) {
    let errorMessage = error?.message || '未知错误'
    const errorBlob = error?.response?.data
    if (errorBlob instanceof Blob) {
      try {
        const text = (await errorBlob.text()).trim()
        if (text) {
          try {
            const payload = JSON.parse(text)
            errorMessage = payload.message || payload.msg || text
          } catch {
            errorMessage = text
          }
        }
      } catch {
        // ignore blob parse error and keep original message
      }
    }

    const isTimeout = error?.code === 'ECONNABORTED' || /timeout|超时/i.test(errorMessage)
    if (file.isDir && isTimeout) {
      ElMessage.error('目录打包下载超时，请缩小目录范围后重试')
      return
    }

    ElMessage.error((file.isDir ? '目录下载失败: ' : '文件下载失败: ') + errorMessage)
  } finally {
    downloadingFiles.value[downloadKey] = false
  }
}

const deleteFile = async (file: FileInfo) => {
  deletingFiles.value[file.name] = true
  try {
    await deleteHostFile(props.hostId, resolveFilePath(file))
    ElMessage.success('文件删除成功')
    refreshFiles()
  } catch (error: any) {
    ElMessage.error('文件删除失败: ' + (error.message || '未知错误'))
  } finally {
    deletingFiles.value[file.name] = false
  }
}

const handleClose = () => {
  emit('update:visible', false)
}

const resolveFilePath = (file: FileInfo) => {
  if (file.path && file.path.trim()) {
    return file.path.trim()
  }
  if (isWindowsHost.value) {
    if (currentPath.value === '') {
      return normalizeWindowsPath(file.name)
    }
    return normalizeWindowsPath(`${currentPath.value}\\${file.name}`)
  }
  if (currentPath.value === '~' || currentPath.value === '/') {
    return (currentPath.value === '~' ? '~/' : '/') + file.name
  }
  return currentPath.value + '/' + file.name
}

const getDownloadKey = (file: FileInfo) => {
  return resolveFilePath(file)
}

const ensureArchiveName = (fileName: string) => {
  const trimmed = (fileName || '').trim()
  if (!trimmed) {
    return 'download.zip'
  }
  if (trimmed.toLowerCase().endsWith('.zip')) {
    return trimmed
  }
  return `${trimmed}.zip`
}

const normalizeWindowsPath = (path: string) => {
  const trimmed = (path || '').trim().split('/').join('\\')
  if (!trimmed) {
    return ''
  }
  if (/^[A-Za-z]:$/.test(trimmed)) {
    return `${trimmed}\\`
  }
  if (/^[A-Za-z]:[^\\]/.test(trimmed)) {
    return `${trimmed.slice(0, 2)}\\${trimmed.slice(2)}`
  }
  return trimmed.replace(/\\{2,}/g, '\\')
}

const getWindowsParentPath = (path: string) => {
  const normalized = normalizeWindowsPath(path).replace(/[\\]+$/, '')
  if (!normalized) {
    return ''
  }

  const segments = normalized.split('\\').filter(Boolean)
  if (segments.length <= 1) {
    return ''
  }

  const [drive, ...rest] = segments
  rest.pop()
  return rest.length > 0 ? `${drive}\\${rest.join('\\')}` : `${drive}\\`
}

const resetBrowserPath = () => {
  currentPath.value = rootPath.value
  pathInput.value = rootPath.value
}

// 监听对话框显示状态
watch(() => props.visible, (visible) => {
  if (visible) {
    resetBrowserPath()
    loadFiles(rootPath.value)
  }
})
</script>

<style scoped lang="scss">
.file-browser-dialog {
  :deep(.el-dialog__header) {
    border-bottom: 1px solid #e4e7ed;
    padding: 20px 24px;
    margin-right: 0;
  }

  :deep(.el-dialog__body) {
    padding: 24px;
  }
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px;
  gap: 16px;

  .loading-text {
    font-size: 14px;
    color: #909399;
  }
}

.file-browser {
  // 面包屑卡片
  .breadcrumb-card {
    background: #ffffff;
    padding: 20px;
    border-radius: 8px;
    margin-bottom: 20px;
    border: 1px solid #e4e7ed;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);

    .breadcrumb-header {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 12px;

      .location-icon {
        font-size: 18px;
        color: #409eff;
      }

      .breadcrumb-title {
        font-size: 14px;
        font-weight: 600;
        color: #303133;
      }
    }

    .path-breadcrumb {
      margin-bottom: 12px;

      :deep(.el-breadcrumb__item) {
        .el-breadcrumb__inner {
          color: #606266;
          font-weight: 500;
          cursor: pointer;
          transition: all 0.3s;
          display: flex;
          align-items: center;
          gap: 6px;

          &:hover {
            color: #409eff;
          }
        }

        .el-breadcrumb__separator {
          color: #c0c4cc;
        }
      }

      .breadcrumb-home {
        :deep(.el-breadcrumb__inner) {
          font-weight: 600;
          color: #409eff;
        }
      }
    }

    .current-path-display {
      display: flex;
      align-items: center;
      gap: 8px;

      .path-icon {
        font-size: 16px;
        color: #409eff;
        flex-shrink: 0;
      }

      .path-input {
        flex: 1;

        :deep(.el-input__wrapper) {
          background: #f5f7fa;
          border-radius: 6px;
          border: 1px solid #e4e7ed;
          box-shadow: none;
          transition: all 0.3s;

          &:hover {
            border-color: #c0c4cc;
          }

          &.is-focus {
            border-color: #409eff;
            background: #ffffff;
          }
        }

        :deep(.el-input__inner) {
          font-family: 'Consolas', 'Monaco', monospace;
          font-size: 13px;
          color: #303133;
          font-weight: 500;
          letter-spacing: 0.5px;
        }

        .path-prefix {
          font-family: 'Consolas', 'Monaco', monospace;
          font-size: 12px;
          color: #909399;
          margin-right: 6px;
        }
      }
    }
  }

  // 工具栏
  .toolbar-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    .toolbar-left,
    .toolbar-right {
      display: flex;
      gap: 12px;
    }

    .toolbar-btn {
      border-radius: 8px;
      padding: 10px 20px;
      font-weight: 500;
      transition: all 0.3s;

      &:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      }
    }

    .upload-btn {
      border-radius: 8px;
      padding: 10px 24px;
      font-weight: 500;
      background-color: #303133;
      border-color: #303133;
      color: #ffffff;
      transition: all 0.3s;

      &:hover {
        background-color: #1d1e1f;
        border-color: #1d1e1f;
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }
    }
  }

  // 上传进度条
  .upload-progress-container {
    margin-bottom: 20px;
    padding: 16px;
    background: #f5f7fa;
    border-radius: 8px;
    border: 1px solid #e4e7ed;

    .upload-info {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 12px;

      .upload-icon {
        font-size: 18px;
        color: #409eff;
        animation: upload-pulse 1.5s ease-in-out infinite;
      }

      .upload-filename {
        flex: 1;
        font-size: 14px;
        color: #303133;
        font-weight: 500;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .upload-percent {
        font-size: 14px;
        font-weight: 600;
        color: #409eff;
        min-width: 45px;
        text-align: right;
      }

      .upload-status {
        font-size: 13px;
        font-weight: 600;
        color: #409eff;
        margin-left: auto;
        white-space: nowrap;
      }
    }

    :deep(.el-progress) {
      .el-progress__text {
        display: none;
      }

      .el-progress-bar__outer {
        background-color: #e4e7ed;
      }

      .el-progress-bar__inner {
        transition: width 0.3s ease;
      }
    }
  }

  @keyframes upload-pulse {
    0%, 100% {
      transform: scale(1);
      opacity: 1;
    }
    50% {
      transform: scale(1.1);
      opacity: 0.8;
    }
  }

  // 文件列表
  .file-list-container {
    .file-table {
      border-radius: 12px;
      overflow: hidden;
      box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

      :deep(.el-table__body tr) {
        transition: all 0.3s;

        &:hover {
          background-color: #f5f7fa;
          transform: scale(1.001);
        }
      }

      .file-icon-wrapper {
        display: flex;
        align-items: center;
        justify-content: center;

        .icon-directory {
          color: #409eff;
          transition: all 0.3s;

          &:hover {
            transform: scale(1.1);
          }
        }

        .icon-file {
          color: #909399;
        }
      }

      .file-name-cell {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 4px 0;

        .file-name-text {
          font-size: 14px;
          color: #303133;
          font-weight: 500;
        }

        &.is-directory {
          cursor: pointer;

          .file-name-text {
            color: #409eff;
          }

          &:hover {
            .file-name-text {
              text-decoration: underline;
            }
          }
        }

        .dir-tag {
          border: none;
          background: #ecf5ff;
          color: #409eff;
          font-size: 12px;
        }
      }

      .file-size {
        font-family: 'Consolas', 'Monaco', monospace;
        font-size: 13px;
        color: #606266;
        font-weight: 500;
      }

      .permission-tag {
        font-family: 'Consolas', 'Monaco', monospace;
        font-size: 12px;
        background: #f4f4f5;
        color: #606266;
        border: 1px solid #e4e7ed;
        border-radius: 6px;
        padding: 4px 10px;
      }

      .time-cell {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 13px;
        color: #606266;

        .time-icon {
          font-size: 14px;
          color: #909399;
        }
      }

      .action-buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 8px;

        .action-btn {
          font-weight: 500;
          transition: all 0.3s;

          &:hover {
            transform: scale(1.05);
          }
        }

        .no-action {
          color: #c0c4cc;
          font-size: 14px;
        }
      }
    }

    :deep(.el-empty) {
      padding: 60px 0;

      .el-empty__image {
        margin-bottom: 20px;
      }

      .el-empty__description {
        margin-top: 16px;
        font-size: 14px;
        color: #909399;
      }
    }
  }
}
</style>
