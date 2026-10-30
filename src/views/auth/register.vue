<template>
  <AuthShell>
    <template #title>注册账号</template>
    <template #subtitle>注册后可创建自己的题库，并与公共题库一起练习</template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item label="用户名">
        <a-input
          v-model:value="username"
          placeholder="2–20 位字母、数字、汉字或 _ . -"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="16" /></template>
        </a-input>
      </a-form-item>
      <a-form-item label="密码">
        <a-input-password
          v-model:value="password"
          placeholder="至少 8 位"
          autocomplete="new-password"
        >
          <template #prefix><LockKeyhole :size="16" /></template>
        </a-input-password>
      </a-form-item>
      <a-form-item label="确认密码">
        <a-input-password
          v-model:value="confirm"
          placeholder="再次输入密码"
          autocomplete="new-password"
        >
          <template #prefix><ShieldCheck :size="16" /></template>
        </a-input-password>
      </a-form-item>
      <a-form-item label="验证码">
        <div class="captcha-row">
          <a-input
            v-model:value="captchaAnswer"
            placeholder="计算结果"
            :maxlength="4"
            @press-enter="submit"
          />
          <button type="button" class="captcha-box" title="点击刷新" @click="loadCaptcha">
            <span v-if="captcha">{{ captcha.question }}</span>
            <span v-else class="muted">加载中</span>
          </button>
        </div>
      </a-form-item>
      <a-button
        type="primary"
        block
        html-type="submit"
        class="submit"
        :loading="loading"
        :disabled="!canSubmit"
      >
        注册并登录
      </a-button>
    </a-form>

    <template #footer>
      已有账号？<router-link to="/login">去登录</router-link>
    </template>
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
const confirm = ref('')
const captcha = ref<Captcha | null>(null)
const captchaAnswer = ref('')
const loading = ref(false)

const canSubmit = computed(
  () =>
    !!username.value.trim() &&
    password.value.length >= 8 &&
    confirm.value === password.value &&
    !!captcha.value &&
    !!captchaAnswer.value.trim(),
)

async function loadCaptcha() {
  try {
    captcha.value = await authApi.captcha()
    captchaAnswer.value = ''
  } catch {
    captcha.value = null
  }
}

async function submit() {
  if (!canSubmit.value || !captcha.value) return
  if (password.value !== confirm.value) {
    message.warning('两次输入的密码不一致')
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
.form :deep(.ant-form-item-label > label) {
  font-size: 13px;
  color: #4a544f;
}
.form :deep(.ant-input-affix-wrapper) {
  height: 44px;
  border-radius: 8px;
}
.form :deep(.ant-input-affix-wrapper .anticon),
.form :deep(.ant-input-prefix) {
  color: #9aa5a0;
  margin-right: 8px;
}
.form :deep(.ant-form-item) { margin-bottom: 14px; }
.captcha-row { display: flex; gap: 10px; }
.captcha-box {
  flex: 0 0 128px;
  height: 44px;
  border: 1px solid #d6dde0;
  border-radius: 8px;
  background: #f6faf8;
  color: #1c7a4e;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 1px;
  cursor: pointer;
}
.captcha-box:hover { border-color: #24945f; }
.submit {
  height: 46px;
  border-radius: 8px;
  font-size: 15.5px;
  font-weight: 600;
  letter-spacing: 1px;
  margin-top: 4px;
  box-shadow: 0 8px 20px rgba(36, 148, 95, 0.25);
}
</style>
