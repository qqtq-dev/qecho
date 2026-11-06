<template>
  <AuthShell>
    <template #title>{{ $t('auth.registerTitle') }}</template>
    <template #subtitle>{{ $t('auth.registerSub') }}</template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item :label="$t('auth.username')">
        <a-input
          v-model:value="username"
          :placeholder="$t('auth.usernameRule')"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="15" /></template>
        </a-input>
      </a-form-item>
      <a-form-item :label="$t('auth.password')">
        <a-input-password
          v-model:value="password"
          :placeholder="$t('auth.passwordRule')"
          autocomplete="new-password"
          @press-enter="submit"
        >
          <template #prefix><LockKeyhole :size="15" /></template>
        </a-input-password>
      </a-form-item>
      <a-form-item :label="$t('auth.captcha')">
        <div class="captcha-row">
          <a-input
            v-model:value="captchaAnswer"
            :placeholder="$t('auth.captchaPlaceholder')"
            :maxlength="4"
            autocomplete="off"
            @press-enter="submit"
          >
            <template #prefix><ShieldCheck :size="15" /></template>
          </a-input>
          <button type="button" class="captcha-box" :title="$t('auth.captchaRefresh')" @click="loadCaptcha">
            <img v-if="captcha" :src="captcha.image" :alt="$t('auth.captchaAlt')" />
            <span v-else class="captcha-loading">{{ $t('auth.captchaLoading') }}</span>
          </button>
        </div>
      </a-form-item>
      <div class="actions">
        <button type="submit" class="action">
          {{ loading ? $t('auth.registerLoading') : $t('auth.register') }}
        </button>
        <button type="button" class="action" @click="router.push('/login')">
          {{ $t('auth.backToLogin') }}
        </button>
      </div>
    </a-form>
  </AuthShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { LockKeyhole, ShieldCheck, UserRound } from 'lucide-vue-next'
import AuthShell from './AuthShell.vue'
import { authApi, type Captcha } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()

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
    message.warning(t('auth.captchaFailed'))
    return
  }
  if (!canSubmit.value) {
    message.warning(t('auth.fillAll'))
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
    message.success(t('auth.registerOk'))
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
  color: var(--text);
}
.form :deep(.ant-input-affix-wrapper) {
  height: 40px;
  padding: 0 12px;
  border-color: var(--border-strong);
  border-radius: 8px;
}
.form :deep(.ant-input-affix-wrapper:focus-within) {
  border-color: #24945f;
  box-shadow: 0 0 0 2px rgba(36, 148, 95, 0.12);
}
.form :deep(.ant-input-affix-wrapper .anticon),
.form :deep(.ant-input-prefix) {
  color: var(--text2);
  margin-right: 8px;
}
.captcha-row { display: flex; align-items: center; gap: 10px; }
.captcha-box {
  flex: 0 0 auto;
  height: 40px;
  padding: 0 4px;
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  background: var(--surface-2);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.captcha-box:hover { border-color: var(--primary); }
.captcha-box img { height: 100%; width: auto; display: block; }
.captcha-loading { font-size: 12px; color: var(--text2); padding: 0 8px; }
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
/* 字距只为「注 册」这种两字按钮拉开；:lang 按 <html lang> 生效，多语言都适用 */
.action:lang(en) { letter-spacing: 0.2px; }
</style>
