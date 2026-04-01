<template>
  <div class="mfa-verify-container">
    <div class="mfa-verify-card">
      <div class="mfa-header">
        <div class="mfa-icon">
          <el-icon><Key /></el-icon>
        </div>
        <h2>两步验证</h2>
        <p class="mfa-subtitle">请输入认证器应用中的6位验证码</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" class="mfa-form">
        <el-form-item prop="code">
          <el-input
            v-model="form.code"
            placeholder="请输入6位验证码"
            maxlength="6"
            size="large"
            class="code-input"
            @keyup.enter="handleVerify"
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </el-form-item>

        <el-form-item>
          <el-checkbox v-model="form.rememberDevice" class="remember-checkbox">
            记住此设备，下次无需验证
          </el-checkbox>
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            @click="handleVerify"
            :loading="loading"
            class="verify-button"
          >
            验证
          </el-button>
        </el-form-item>
      </el-form>

      <div class="mfa-footer">
        <p>没有验证器？请联系管理员重置MFA</p>
        <router-link to="/login" class="back-link">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, FormInstance } from 'element-plus'
import { Key, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { mfaLogin } from '@/api/mfa'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const formRef = ref<FormInstance>()
const loading = ref(false)
const mfaToken = ref('')

const form = reactive({
  code: '',
  rememberDevice: false
})

const rules = {
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { len: 6, message: '验证码必须是6位', trigger: 'blur' }
  ]
}

const handleVerify = async () => {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  if (!mfaToken.value) {
    ElMessage.error('MFA Token缺失，请重新登录')
    router.replace('/login')
    return
  }

  loading.value = true
  try {
    const res = await mfaLogin({
      mfaToken: mfaToken.value,
      code: form.code,
      rememberDevice: form.rememberDevice
    })

    // 保存token和用户信息
    userStore.setToken(res.token)
    if (res.user) {
      userStore.setUserInfo(res.user)
    }

    // 清除sessionStorage中的mfaToken
    sessionStorage.removeItem('mfaToken')

    // 更新MFA缓存，标记用户已完成MFA验证
    const { updateMFACache } = await import('@/router')
    updateMFACache(true)

    ElMessage.success('登录成功')

    // 检查是否有重定向URL
    const redirectUrl = route.query.redirect as string
    if (redirectUrl) {
      // 如果是外部URL，直接跳转
      if (redirectUrl.startsWith('http://') || redirectUrl.startsWith('https://')) {
        window.location.href = redirectUrl
      } else {
        // 内部路由
        await router.replace(redirectUrl)
      }
    } else {
      await router.replace('/')
    }
  } catch (error: any) {
    const message = error?.response?.data?.message || error?.message || '验证失败'
    ElMessage.error(message)
    form.code = ''
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 从sessionStorage或URL query中获取mfaToken
  const token = sessionStorage.getItem('mfaToken') || (route.query.mfaToken as string)
  if (!token) {
    ElMessage.warning('请先登录后再进行MFA验证')
    router.replace('/login')
    return
  }
  mfaToken.value = token
})
</script>

<style scoped>
.mfa-verify-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
  padding: 20px;
}

.mfa-verify-card {
  background: #ffffff;
  border-radius: 16px;
  padding: 48px 40px;
  width: 100%;
  max-width: 420px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.mfa-header {
  text-align: center;
  margin-bottom: 40px;
}

.mfa-icon {
  width: 72px;
  height: 72px;
  background: linear-gradient(135deg, #D4AF37 0%, #FFD700 100%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 24px;
}

.mfa-icon .el-icon {
  font-size: 32px;
  color: #1a1a1a;
}

.mfa-header h2 {
  font-size: 28px;
  font-weight: 600;
  color: #1a1a1a;
  margin: 0 0 12px;
}

.mfa-subtitle {
  font-size: 15px;
  color: #666;
  margin: 0;
}

.mfa-form {
  margin-bottom: 24px;
}

.code-input :deep(.el-input__wrapper) {
  padding: 14px 16px;
  border-radius: 10px;
  font-size: 18px;
  letter-spacing: 8px;
  text-align: center;
}

.code-input :deep(.el-input__inner) {
  text-align: center;
}

.verify-button {
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 500;
  border-radius: 10px;
  background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
  border: 2px solid #D4AF37;
  color: #D4AF37;
  transition: all 0.3s;
}

.remember-checkbox {
  color: #666;
}

.remember-checkbox :deep(.el-checkbox__label) {
  font-size: 14px;
  color: #666;
}

.verify-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.15), 0 0 20px rgba(212, 175, 55, 0.3);
  background: linear-gradient(135deg, #D4AF37 0%, #FFD700 100%);
  color: #1a1a1a;
}

.mfa-footer {
  text-align: center;
  padding-top: 24px;
  border-top: 1px solid #eee;
}

.mfa-footer p {
  font-size: 14px;
  color: #999;
  margin: 0 0 12px;
}

.back-link {
  font-size: 14px;
  color: #D4AF37;
  text-decoration: none;
  transition: color 0.3s;
}

.back-link:hover {
  color: #FFD700;
}
</style>
