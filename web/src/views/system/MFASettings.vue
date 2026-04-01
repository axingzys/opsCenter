<template>
  <div class="mfa-settings-container">
    <!-- 强制设置提示 -->
    <el-alert
      v-if="!mfaStatus.isEnabled"
      title="系统已开启强制MFA"
      type="warning"
      description="为了保障账户安全，您需要先设置两步验证才能继续使用系统"
      :closable="false"
      show-icon
      style="margin-bottom: 20px"
    />

    <div class="mfa-header">
      <div class="mfa-icon">
        <el-icon><Key /></el-icon>
      </div>
      <div class="mfa-title">
        <h2>两步验证</h2>
        <p>为您的账户添加额外的安全保护</p>
      </div>
    </div>

    <div class="mfa-content">
      <!-- MFA未启用状态 -->
      <div v-if="!mfaStatus.isEnabled" class="mfa-status-card disabled">
        <div class="status-icon">
          <el-icon><WarningFilled /></el-icon>
        </div>
        <div class="status-info">
          <h3>两步验证未启用</h3>
          <p>启用两步验证后，登录时需要输入验证器应用中的动态验证码</p>
        </div>
        <el-button type="primary" @click="handleSetup" :loading="setupLoading">
          启用两步验证
        </el-button>
      </div>

      <!-- MFA已启用状态 -->
      <div v-else class="mfa-status-card enabled">
        <div class="status-icon success">
          <el-icon><CircleCheckFilled /></el-icon>
        </div>
        <div class="status-info">
          <h3>两步验证已启用</h3>
          <p>您的账户已受两步验证保护</p>
        </div>
        <div class="status-actions">
          <el-button @click="showRegenerateDialog = true">
            重新生成备用码
          </el-button>
          <el-button type="danger" plain @click="showDisableDialog = true">
            禁用两步验证
          </el-button>
        </div>
      </div>

      <!-- MFA设置说明 -->
      <div class="mfa-info-card">
        <h4>如何使用两步验证？</h4>
        <ol>
          <li>在手机上安装验证器应用（如 Google Authenticator、Microsoft Authenticator）</li>
          <li>点击"启用两步验证"按钮，使用验证器应用扫描二维码</li>
          <li>输入验证器应用显示的6位验证码完成设置</li>
          <li>以后每次登录都需要输入验证器应用中的验证码</li>
        </ol>
        <div class="mfa-warning">
          <el-icon><WarningFilled /></el-icon>
          <span>请妥善保存备用码，当无法使用验证器应用时，可以使用备用码登录</span>
        </div>
      </div>
    </div>

    <!-- 设置MFA对话框 -->
    <el-dialog
      v-model="showSetupDialog"
      title="设置两步验证"
      width="500px"
      :close-on-click-modal="false"
    >
      <div class="setup-content">
        <div class="setup-step">
          <h4>第一步：扫描二维码</h4>
          <p>使用验证器应用扫描下方二维码</p>
          <div class="qr-code-container">
            <img v-if="setupData.qrCodeUrl" :src="setupData.qrCodeUrl" alt="MFA QR Code" class="qr-code" />
            <div v-else class="qr-loading">
              <el-icon class="loading-icon"><Loading /></el-icon>
              <span>生成中...</span>
            </div>
          </div>
          <div class="manual-code">
            <span>手动输入密钥：</span>
            <code>{{ setupData.manualCode }}</code>
          </div>
        </div>

        <div class="setup-step">
          <h4>第二步：输入验证码</h4>
          <p>输入验证器应用显示的6位验证码</p>
          <el-input
            v-model="verifyCode"
            placeholder="请输入6位验证码"
            maxlength="6"
            size="large"
            class="verify-input"
          />
        </div>
      </div>

      <template #footer>
        <el-button @click="showSetupDialog = false">取消</el-button>
        <el-button type="primary" @click="handleEnable" :loading="enableLoading" :disabled="verifyCode.length !== 6">
          验证并启用
        </el-button>
      </template>
    </el-dialog>

    <!-- 禁用MFA对话框 -->
    <el-dialog
      v-model="showDisableDialog"
      title="禁用两步验证"
      width="400px"
    >
      <div class="disable-content">
        <el-icon class="warning-icon"><WarningFilled /></el-icon>
        <p>禁用两步验证将降低账户安全性，确定要继续吗？</p>
        <el-input
          v-model="disableCode"
          placeholder="请输入验证码确认"
          maxlength="6"
        />
      </div>
      <template #footer>
        <el-button @click="showDisableDialog = false">取消</el-button>
        <el-button type="danger" @click="handleDisable" :loading="disableLoading" :disabled="disableCode.length !== 6">
          确认禁用
        </el-button>
      </template>
    </el-dialog>

    <!-- 重新生成备用码对话框 -->
    <el-dialog
      v-model="showRegenerateDialog"
      title="重新生成备用码"
      width="500px"
    >
      <div class="regenerate-content">
        <p>请输入当前验证码以重新生成备用码</p>
        <el-input
          v-model="regenerateCode"
          placeholder="请输入验证码"
          maxlength="6"
        />
      </div>
      <template #footer>
        <el-button @click="showRegenerateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleRegenerate" :loading="regenerateLoading" :disabled="regenerateCode.length !== 6">
          重新生成
        </el-button>
      </template>
    </el-dialog>

    <!-- 显示备用码对话框 -->
    <el-dialog
      v-model="showBackupCodesDialog"
      title="备用码"
      width="500px"
    >
      <div class="backup-codes-content">
        <el-icon class="backup-icon"><Document /></el-icon>
        <p>请保存以下备用码，每个备用码只能使用一次</p>
        <div class="backup-codes-list">
          <div v-for="(code, index) in backupCodes" :key="index" class="backup-code-item">
            {{ code }}
          </div>
        </div>
        <el-button type="primary" plain @click="copyBackupCodes">
          <el-icon><CopyDocument /></el-icon>
          复制所有备用码
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showBackupCodesDialog = false">
          我已保存备用码
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Key, WarningFilled, CircleCheckFilled, Document, CopyDocument, Loading } from '@element-plus/icons-vue'
import {
  getMFAStatus,
  setupMFA,
  enableMFA,
  disableMFA,
  regenerateBackupCodes,
  type MFAStatusResponse,
  type MFASetupResponse
} from '@/api/mfa'

const router = useRouter()
const route = useRoute()

// 检查是否是强制设置模式（从登录跳转过来）
const isForceSetup = computed(() => {
  return route.query.force === 'true' || !mfaStatus.value.isEnabled
})

const mfaStatus = ref<MFAStatusResponse>({
  isEnabled: false,
  mfaType: 'totp',
  hasBackup: false
})

const setupLoading = ref(false)
const enableLoading = ref(false)
const disableLoading = ref(false)
const regenerateLoading = ref(false)

const showSetupDialog = ref(false)
const showDisableDialog = ref(false)
const showRegenerateDialog = ref(false)
const showBackupCodesDialog = ref(false)

const setupData = ref<MFASetupResponse>({
  secret: '',
  qrCodeUrl: '',
  manualCode: ''
})

const verifyCode = ref('')
const disableCode = ref('')
const regenerateCode = ref('')
const backupCodes = ref<string[]>([])

const loadMFAStatus = async () => {
  try {
    const res = await getMFAStatus()
    if (res) {
      mfaStatus.value = res
    }
  } catch (error) {
    console.error('加载MFA状态失败', error)
  }
}

const handleSetup = async () => {
  setupLoading.value = true
  try {
    const res = await setupMFA()
    if (res) {
      setupData.value = res
      showSetupDialog.value = true
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '获取MFA设置信息失败')
  } finally {
    setupLoading.value = false
  }
}

const handleEnable = async () => {
  if (verifyCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

  enableLoading.value = true
  try {
    await enableMFA(verifyCode.value)
    ElMessage.success('两步验证已启用')
    showSetupDialog.value = false
    verifyCode.value = ''
    await loadMFAStatus()

    // 更新MFA缓存状态为已启用，并清除强制标记
    const { updateMFACache } = await import('@/router')
    updateMFACache(true, false)

    // 如果是强制设置模式，设置完成后跳转到首页
    if (isForceSetup.value) {
      ElMessage.success('MFA设置完成，正在跳转...')
      setTimeout(() => {
        router.push('/')
      }, 1000)
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '启用失败')
  } finally {
    enableLoading.value = false
  }
}

const handleDisable = async () => {
  if (disableCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

  disableLoading.value = true
  try {
    await disableMFA(disableCode.value)
    ElMessage.success('两步验证已禁用')
    showDisableDialog.value = false
    disableCode.value = ''
    await loadMFAStatus()
  } catch (error: any) {
    ElMessage.error(error?.message || '禁用失败')
  } finally {
    disableLoading.value = false
  }
}

const handleRegenerate = async () => {
  if (regenerateCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

  regenerateLoading.value = true
  try {
    const res = await regenerateBackupCodes(regenerateCode.value)
    if (res && res.backupCodes) {
      backupCodes.value = res.backupCodes
      showRegenerateDialog.value = false
      showBackupCodesDialog.value = true
      regenerateCode.value = ''
      ElMessage.success('备用码已重新生成')
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '重新生成备用码失败')
  } finally {
    regenerateLoading.value = false
  }
}

const copyBackupCodes = () => {
  const text = backupCodes.value.join('\n')
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('备用码已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

onMounted(() => {
  loadMFAStatus()
})
</script>

<style scoped>
.mfa-settings-container {
  padding: 24px;
  max-width: 800px;
  margin: 0 auto;
}

.mfa-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 32px;
}

.mfa-icon {
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #d4af37;
  font-size: 28px;
  border: 2px solid #d4af37;
}

.mfa-title h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  color: #303133;
}

.mfa-title p {
  margin: 0;
  font-size: 14px;
  color: #909399;
}

.mfa-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.mfa-status-card {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  border: 1px solid #ebeef5;
}

.mfa-status-card.disabled {
  border-color: #e6a23c;
  background: linear-gradient(135deg, #fdf6ec 0%, #fff 100%);
}

.mfa-status-card.enabled {
  border-color: #67c23a;
  background: linear-gradient(135deg, #f0f9eb 0%, #fff 100%);
}

.status-icon {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  background: #fdf6ec;
  color: #e6a23c;
}

.status-icon.success {
  background: #f0f9eb;
  color: #67c23a;
}

.status-info {
  flex: 1;
}

.status-info h3 {
  margin: 0 0 8px 0;
  font-size: 18px;
  color: #303133;
}

.status-info p {
  margin: 0;
  font-size: 14px;
  color: #606266;
}

.status-actions {
  display: flex;
  gap: 12px;
}

.mfa-info-card {
  padding: 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  border: 1px solid #ebeef5;
}

.mfa-info-card h4 {
  margin: 0 0 16px 0;
  font-size: 16px;
  color: #303133;
}

.mfa-info-card ol {
  margin: 0 0 20px 0;
  padding-left: 20px;
}

.mfa-info-card li {
  margin-bottom: 8px;
  color: #606266;
  font-size: 14px;
  line-height: 1.6;
}

.mfa-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #fdf6ec;
  border-radius: 8px;
  font-size: 13px;
  color: #e6a23c;
}

/* 设置对话框 */
.setup-content {
  padding: 20px 0;
}

.setup-step {
  margin-bottom: 32px;
}

.setup-step:last-child {
  margin-bottom: 0;
}

.setup-step h4 {
  margin: 0 0 12px 0;
  font-size: 16px;
  color: #303133;
}

.setup-step p {
  margin: 0 0 16px 0;
  font-size: 14px;
  color: #909399;
}

.qr-code-container {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}

.qr-code {
  width: 200px;
  height: 200px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.qr-loading {
  width: 200px;
  height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  border-radius: 8px;
}

.loading-icon {
  font-size: 32px;
  color: #d4af37;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.manual-code {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 14px;
  color: #606266;
}

.manual-code code {
  padding: 4px 12px;
  background: #f5f7fa;
  border-radius: 4px;
  font-family: monospace;
  font-size: 14px;
  color: #303133;
}

.verify-input {
  width: 100%;
}

.verify-input :deep(.el-input__inner) {
  text-align: center;
  font-size: 20px;
  letter-spacing: 8px;
}

/* 禁用对话框 */
.disable-content {
  text-align: center;
  padding: 20px 0;
}

.warning-icon {
  font-size: 48px;
  color: #f56c6c;
  margin-bottom: 16px;
}

.disable-content p {
  font-size: 14px;
  color: #606266;
  margin-bottom: 20px;
}

/* 备用码对话框 */
.backup-codes-content {
  text-align: center;
  padding: 20px 0;
}

.backup-icon {
  font-size: 48px;
  color: #d4af37;
  margin-bottom: 16px;
}

.backup-codes-list {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin: 20px 0;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
}

.backup-code-item {
  padding: 8px 12px;
  background: #fff;
  border-radius: 4px;
  font-family: monospace;
  font-size: 16px;
  color: #303133;
  border: 1px solid #ebeef5;
}

/* 响应式 */
@media (max-width: 768px) {
  .mfa-settings-container {
    padding: 16px;
  }

  .mfa-status-card {
    flex-direction: column;
    text-align: center;
  }

  .status-actions {
    flex-direction: column;
    width: 100%;
  }

  .status-actions .el-button {
    width: 100%;
  }

  .backup-codes-list {
    grid-template-columns: 1fr;
  }
}
</style>
