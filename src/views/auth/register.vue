<template>
  <AuthShell>
    <template #title>注册账号</template>
    <template #subtitle>注册成功后自动登录，首个账号将成为管理员</template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item label="用户名">
        <a-input
          v-model:value="username"
          placeholder="2–20 位字母、数字、汉字或 _ . -"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="15" /></template>
        </a-input>
      </a-form-item>
      <a-form-item label="密码">
        <a-input-password
          v-model:value="password"
          placeholder="至少 8 位"
          autocomplete="new-password"
          @press-enter="submit"
        >
          <template #prefix><LockKeyhole :size="15" /></template>
        </a-input-password>
      </a-form-item>
      <a-form-item label="验证码">
        <div class="captcha-row">
          <a-input
            v-model:value="captchaAnswer"
            placeholder="输入右侧字符"
            :maxlength="4"
            autocomplete="off"
            @press-enter="submit"
          >
            <template #prefix><ShieldCheck :size="15" /></template>
          </a-input>
          <button type="button" class="captcha-box" title="看不清？点击换一张" @click="loadCaptcha">
            <img v-if="captcha" :src="captcha.image" alt="图片验证码" />
            <span v-else class="captcha-loading">加载中</span>
          </button>
        </div>
      </a-form-item>
      <div class="actions">
        <button type="submit" class="action">
          {{ loading ? '注册中…' : '注 册' }}
        </button>
        <button type="button" class="action" @click="router.push('/login')">
          返回登录
        </button>
      </div>
    </a-form>
  </AuthShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { LockKeyhole, ShieldCheck, UserRound } from 'lucide-vue-next'
import AuthShell from './AuthShell.vue'
import { authApi, type Captcha } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const captcha = ref<Captcha | null>(null)
const captchaAnswer = ref('')
const loading = ref(false)

const canSubmit = computed(
  () =>
    username.value.trim().length >= 2 &&
    password.value.length >= 8 &&
    !!captcha.value &&
    captchaAnswer.value.trim().length > 0,
)

async function loadCaptcha() {
  captchaAnswer.value = ''
  try {
    captcha.value = await authApi.captcha()
  } catch {
    captcha.value = null
  }
}

async function submit() {
  if (loading.value) return
  if (!captcha.value) {
    message.warning('验证码加载失败，请点击图片刷新')
    return
  }
  if (!canSubmit.value) {
    message.warning('请填写用户名、密码和图片验证码')
    return
  }
  loading.value = true
  try {
    await auth.register({
      username: username.value.trim(),
      password: password.value,
      captchaId: captcha.value.id,
      captchaAnswer: captchaAnswer.value.trim(),
    })
    message.success('注册成功，已自动登录')
    router.replace('/dashboard')
  } catch {
    await loadCaptcha()
  } finally {
    loading.value = false
  }
}

onMounted(loadCaptcha)
</script>

<style scoped>
.form :deep(.ant-form-item) { margin-bottom: 16px; }
.form :deep(.ant-form-item-label) { padding: 0 0 6px; }
.form :deep(.ant-form-item-label > label) {
  height: auto;
  font-size: 13px;
  color: rgba(0, 0, 0, 0.65);
}
.form :deep(.ant-input-affix-wrapper) {
  height: 40px;
  padding: 0 12px;
  border-color: #d9d9d9;
  border-radius: 8px;
}
.form :deep(.ant-input-affix-wrapper:focus-within) {
  border-color: #24945f;
  box-shadow: 0 0 0 2px rgba(36, 148, 95, 0.12);
}
.form :deep(.ant-input-affix-wrapper .anticon),
.form :deep(.ant-input-prefix) {
  color: rgba(0, 0, 0, 0.35);
  margin-right: 8px;
}
.captcha-row { display: flex; align-items: center; gap: 10px; }
.captcha-box {
  flex: 0 0 auto;
  height: 40px;
  padding: 0 4px;
  border: 1px solid #d9d9d9;
  border-radius: 8px;
  background: #f6faf8;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.captcha-box:hover { border-color: #24945f; }
.captcha-box img { height: 100%; width: auto; display: block; }
.captcha-loading { font-size: 12px; color: #9aa5a0; padding: 0 8px; }
.actions {
  display: flex;
  gap: 10px;
  margin-top: 4px;
}
.action {
  flex: 1;
  height: 40px;
  border: none;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 3px;
  cursor: pointer;
  background: #24945f;
  color: #fff;
  transition: background 0.15s;
}
.action:hover { background: #1f8756; }
</style>
