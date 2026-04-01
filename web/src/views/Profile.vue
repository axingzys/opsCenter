<template>
  <div class="profile-container">
    <!-- 页面标题 -->
    <div class="page-header">
      <h2 class="page-title">个人信息</h2>
    </div>

    <el-tabs v-model="activeTab" class="profile-tabs">
      <!-- 基本信息标签页 -->
      <el-tab-pane label="基本信息" name="basic">
        <div class="tab-content">
          <el-form
            :model="profileForm"
            :rules="profileRules"
            ref="profileFormRef"
            label-width="100px"
            class="profile-form"
          >
            <el-form-item label="头像">
              <div class="avatar-section">
                <el-avatar :size="100" :src="avatarUrl" :key="avatarKey">
                  <el-icon><UserFilled /></el-icon>
                </el-avatar>
                <el-upload
                  class="avatar-uploader"
                  :show-file-list="false"
                  :before-upload="beforeAvatarUpload"
                  :http-request="handleAvatarUpload"
                  accept="image/*"
                >
                  <el-button class="black-button" :loading="uploadLoading" style="margin-left: 20px;">
                    {{ uploadLoading ? '上传中...' : '更换头像' }}
                  </el-button>
                </el-upload>
              </div>
              <div class="avatar-tip">支持 JPG、PNG 格式,文件大小不超过 2MB</div>
            </el-form-item>

            <el-form-item label="用户名">
              <el-input v-model="profileForm.username" disabled />
            </el-form-item>

            <el-form-item label="真实姓名" prop="realName">
              <el-input v-model="profileForm.realName" placeholder="请输入真实姓名" />
            </el-form-item>

            <el-form-item label="邮箱" prop="email">
              <el-input v-model="profileForm.email" placeholder="请输入邮箱" />
            </el-form-item>

            <el-form-item label="手机号" prop="phone">
              <el-input v-model="profileForm.phone" placeholder="请输入手机号" />
            </el-form-item>

            <el-form-item>
              <el-button class="black-button" @click="handleUpdateProfile" :loading="updateLoading">
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- 修改密码标签页 -->
      <el-tab-pane label="修改密码" name="password" :disabled="isLDAPUser">
        <div class="tab-content">
          <!-- LDAP用户提示 -->
          <el-alert
            v-if="isLDAPUser"
            title="LDAP用户无法修改密码"
            type="info"
            description="您的账号通过LDAP认证，密码由LDAP服务器管理，无法在此修改。请联系系统管理员修改密码。"
            :closable="false"
            show-icon
            style="margin-bottom: 20px"
          />

          <el-form
            v-if="!isLDAPUser"
            :model="passwordForm"
            :rules="passwordRules"
            ref="passwordFormRef"
            label-width="100px"
            class="profile-form"
            style="max-width: 600px"
          >
            <el-form-item label="原密码" prop="oldPassword">
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                show-password
                placeholder="请输入原密码"
              />
            </el-form-item>

            <el-form-item label="新密码" prop="newPassword">
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                show-password
                placeholder="请输入新密码（至少6位）"
              />
            </el-form-item>

            <el-form-item label="确认密码" prop="confirmPassword">
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                show-password
                placeholder="请再次输入新密码"
              />
            </el-form-item>

            <el-form-item>
              <el-button class="black-button" @click="handleUpdatePassword" :loading="passwordLoading">
                修改密码
              </el-button>
              <el-button @click="handleResetPassword">重置</el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- MFA设置标签页 -->
      <el-tab-pane label="两步验证" name="mfa">
        <div class="tab-content">
          <div class="mfa-section">
            <!-- MFA未启用状态 -->
            <div v-if="!mfaStatus.isEnabled" class="mfa-status-card disabled">
              <div class="status-icon">
                <el-icon><WarningFilled /></el-icon>
              </div>
              <div class="status-info">
                <h3>两步验证未启用</h3>
                <p>启用两步验证后，登录时需要输入验证器应用中的动态验证码</p>
              </div>
              <el-button type="primary" @click="handleSetupMFA" :loading="mfaSetupLoading">
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
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
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
          <img v-if="mfaSetupData.qrCodeUrl" :src="mfaSetupData.qrCodeUrl" alt="MFA QR Code" class="qr-code" />
          <div v-else class="qr-loading">
            <span>生成中...</span>
          </div>
        </div>
        <div class="manual-code">
          <span>手动输入密钥：</span>
          <code>{{ mfaSetupData.manualCode }}</code>
        </div>
      </div>

      <div class="setup-step">
        <h4>第二步:输入验证码</h4>
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
      <el-button type="primary" @click="handleEnableMFA" :disabled="verifyCode.length !== 6">
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
      <el-button type="danger" @click="handleDisableMFA" :disabled="disableCode.length !== 6">
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
      <el-button type="primary" @click="handleRegenerateBackupCodes" :disabled="regenerateCode.length !== 6">
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
      <p>请保存以下备用码，每个备用码只能使用一次</p>
      <div class="backup-codes-list">
        <div v-for="(code, index) in backupCodes" :key="index" class="backup-code-item">
          {{ code }}
        </div>
      </div>
      <el-button type="primary" plain @click="copyBackupCodes">
        复制所有备用码
      </el-button>
    </div>
    <template #footer>
      <el-button type="primary" @click="showBackupCodesDialog = false">
        我已保存备用码
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, nextTick } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { UserFilled, WarningFilled, CircleCheckFilled } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { updateUser, changePassword } from '@/api/user'
import { uploadAvatar, updateUserAvatar } from '@/api/upload'
import {
  getMFAStatus,
  setupMFA,
  enableMFA,
  disableMFA,
  regenerateBackupCodes
} from '@/api/mfa'
import type { UploadProps } from 'element-plus'

const userStore = useUserStore()
const activeTab = ref('basic')
const updateLoading = ref(false)
const passwordLoading = ref(false)
const uploadLoading = ref(false)

// 检测是否为LDAP用户
const isLDAPUser = computed(() => {
  return userStore.userInfo?.source === 'ldap'
})

// 用于强制刷新头像组件的 key
const avatarKey = ref(Date.now())

const profileFormRef = ref<FormInstance>()
const passwordFormRef = ref<FormInstance>()

const profileForm = reactive({
  id: 0,
  username: '',
  realName: '',
  email: '',
  phone: '',
  avatar: ''
})

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const profileRules = {
  realName: [{ required: true, message: '请输入真实姓名', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' }
  ],
  phone: [{ pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' }]
}

const validateConfirmPassword = (_rule: any, value: any, callback: any) => {
  if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules = {
  oldPassword: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

// 加载用户信息
const loadUserInfo = async () => {
  try {
    // 如果 store 中已有用户信息，先显示出来
    if (userStore.userInfo) {
      profileForm.id = userStore.userInfo.ID || userStore.userInfo.id
      profileForm.username = userStore.userInfo.username
      profileForm.realName = userStore.userInfo.realName || ''
      profileForm.email = userStore.userInfo.email || ''
      profileForm.phone = userStore.userInfo.phone || ''
      profileForm.avatar = userStore.userInfo.avatar || ''
    }

    // 然后异步刷新最新数据
    await userStore.getProfile()

    // 刷新后再次更新表单，确保数据最新
    if (userStore.userInfo) {
      profileForm.id = userStore.userInfo.ID || userStore.userInfo.id
      profileForm.username = userStore.userInfo.username
      profileForm.realName = userStore.userInfo.realName || ''
      profileForm.email = userStore.userInfo.email || ''
      profileForm.phone = userStore.userInfo.phone || ''
      profileForm.avatar = userStore.userInfo.avatar || ''
    }
  } catch (error) {
    ElMessage.error('获取用户信息失败')
  }
}

// 头像URL - 添加时间戳破坏缓存
const avatarUrl = computed(() => {
  const avatar = userStore.userInfo?.avatar || ''
  if (!avatar) return ''

  // 如果是base64图片，直接返回
  if (avatar.startsWith('data:')) return avatar

  // 添加时间戳参数破坏浏览器缓存（使用 store 中的时间戳）
  const separator = avatar.includes('?') ? '&' : '?'
  return `${avatar}${separator}t=${userStore.avatarTimestamp}`
})

// 上传前校验
const beforeAvatarUpload: UploadProps['beforeUpload'] = (file) => {
  const isImage = file.type.startsWith('image/')
  const isLt2M = file.size / 1024 / 1024 < 2

  if (!isImage) {
    ElMessage.error('只能上传图片文件!')
    return false
  }
  if (!isLt2M) {
    ElMessage.error('图片大小不能超过 2MB!')
    return false
  }
  return true
}

// 处理头像上传
const handleAvatarUpload = async (options: any) => {
  const { file } = options
  uploadLoading.value = true

  try {
    // 上传图片到服务器
    const uploadRes: any = await uploadAvatar(file)

    if (uploadRes.code === 0 && uploadRes.data) {
      // 获取服务器返回的头像路径
      const serverPath = uploadRes.data.url || uploadRes.data

      // 更新用户头像到服务器（保存相对路径）
      await updateUserAvatar(serverPath)

      // 立即更新 store 中的头像，触发所有组件更新
      userStore.updateAvatar(serverPath)

      // 等待 DOM 更新
      await nextTick()

      // 强制刷新组件（通过改变 key）
      avatarKey.value = Date.now()

      ElMessage.success('头像上传成功')

      // 延迟刷新完整的用户信息（避免覆盖刚更新的头像）
      setTimeout(() => {
        userStore.getProfile().then(() => {
        })
      }, 500)
    } else {
      throw new Error(uploadRes.message || '上传失败')
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.message || error.message || '头像上传失败'
    ElMessage.error(errorMsg)
  } finally {
    uploadLoading.value = false
  }
}

// 更新基本信息
const handleUpdateProfile = async () => {
  if (!profileFormRef.value) return

  await profileFormRef.value.validate(async (valid) => {
    if (valid) {
      updateLoading.value = true
      try {
        await updateUser(profileForm.id, {
          realName: profileForm.realName,
          email: profileForm.email,
          phone: profileForm.phone
        })
        ElMessage.success('保存成功')
        // 重新获取用户信息
        await userStore.getProfile()
        // 更新表单数据
        if (userStore.userInfo) {
          profileForm.id = userStore.userInfo.ID || userStore.userInfo.id
          profileForm.username = userStore.userInfo.username
          profileForm.realName = userStore.userInfo.realName || ''
          profileForm.email = userStore.userInfo.email || ''
          profileForm.phone = userStore.userInfo.phone || ''
          profileForm.avatar = userStore.userInfo.avatar || ''
        }
      } catch (error) {
        ElMessage.error('保存失败')
      } finally {
        updateLoading.value = false
      }
    }
  })
}

// 修改密码
const handleUpdatePassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate(async (valid) => {
    if (valid) {
      passwordLoading.value = true
      try {
        await changePassword(passwordForm.oldPassword, passwordForm.newPassword)
        ElMessage.success('密码修改成功，请重新登录')
        handleResetPassword()
        // 延迟后跳转到登录页
        setTimeout(() => {
          userStore.logout()
          window.location.href = '/login'
        }, 1500)
      } catch (error: any) {
        const errorMsg = error.response?.data?.message || error.message || '修改密码失败'
        ElMessage.error(errorMsg)
      } finally {
        passwordLoading.value = false
      }
    }
  })
}

// 重置密码表单
const handleResetPassword = () => {
  passwordFormRef.value?.resetFields()
}

// MFA相关
const mfaStatus = ref({ isEnabled: false })
const mfaSetupLoading = ref(false)
const showSetupDialog = ref(false)
const showDisableDialog = ref(false)
const showRegenerateDialog = ref(false)
const showBackupCodesDialog = ref(false)
const mfaSetupData = ref({ qrCodeUrl: '', manualCode: '' })
const verifyCode = ref('')
const disableCode = ref('')
const regenerateCode = ref('')
const backupCodes = ref<string[]>([])

// 加载MFA状态
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

// 设置MFA
const handleSetupMFA = async () => {
  mfaSetupLoading.value = true
  try {
    const res = await setupMFA()
    if (res) {
      mfaSetupData.value = res
      showSetupDialog.value = true
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '获取MFA设置信息失败')
  } finally {
    mfaSetupLoading.value = false
  }
}

// 启用MFA
const handleEnableMFA = async () => {
  if (verifyCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

  try {
    await enableMFA(verifyCode.value)
    ElMessage.success('两步验证已启用')
    showSetupDialog.value = false
    verifyCode.value = ''
    await loadMFAStatus()
  } catch (error: any) {
    ElMessage.error(error?.message || '启用失败')
  }
}

// 禁用MFA
const handleDisableMFA = async () => {
  if (disableCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

  try {
    await disableMFA(disableCode.value)
    ElMessage.success('两步验证已禁用')
    showDisableDialog.value = false
    disableCode.value = ''
    await loadMFAStatus()
  } catch (error: any) {
    ElMessage.error(error?.message || '禁用失败')
  }
}

// 重新生成备用码
const handleRegenerateBackupCodes = async () => {
  if (regenerateCode.value.length !== 6) {
    ElMessage.warning('请输入6位验证码')
    return
  }

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
  }
}

// 复制备用码
const copyBackupCodes = () => {
  const text = backupCodes.value.join('\n')
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('备用码已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

onMounted(() => {
  loadUserInfo()
  loadMFAStatus()
})
</script>

<style scoped>
.profile-container {
  padding: 20px;
  background-color: #fff;
  min-height: 100%;
}

.page-header {
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e6e6e6;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 500;
  color: #303133;
}

.profile-tabs {
  background-color: transparent;
}

.tab-content {
  padding-top: 20px;
}

.profile-form {
  max-width: 800px;
}

.avatar-section {
  display: flex;
  align-items: center;
}

.avatar-section :deep(.el-avatar) {
  background-color: #FFAF35;
  border: 3px solid rgba(255, 255, 255, 0.2);
}

.avatar-section :deep(.el-icon) {
  font-size: 50px;
  color: #fff;
}

.avatar-uploader {
  display: inline-block;
}

.avatar-tip {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}

/* 黑色按钮样式 */
.black-button {
  background-color: #000000 !important;
  color: #ffffff !important;
  border-color: #000000 !important;
}

.black-button:hover {
  background-color: #333333 !important;
  border-color: #333333 !important;
}

.black-button:focus {
  background-color: #000000 !important;
  border-color: #000000 !important;
}

/* MFA相关样式 */
.mfa-section {
  max-width: 800px;
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

.mfa-status-card.enabled{
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

/* MFA设置对话框样式 */
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

/* 禁用对话框样式 */
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

/* 备用码对话框样式 */
.backup-codes-content {
  text-align: center;
  padding: 20px 0;
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
</style>
