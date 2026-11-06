<template>
  <AuthShell>
    <template #title>{{ $t('auth.loginTitle') }}</template>
    <template #subtitle>
      {{ isDesktop ? $t('auth.loginSubDesktop') : $t('auth.loginSubWeb') }}
    </template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item :label="$t('auth.username')">
        <a-input
          v-model:value="username"
          :placeholder="isDesktop ? $t('auth.placeholderLocalUser') : $t('auth.placeholderUser')"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="15" /></template>
        </a-input>
      </a-form-item>
      <a-form-item :label="$t('auth.password')">
        <a-input-password
          v-model:value="password"
          :placeholder="isDesktop ? $t('auth.placeholderLocalPwd') : $t('auth.placeholderPwd')"
          autocomplete="current-password"
          @press-enter="submit"
        >
          <template #prefix><LockKeyhole :size="15" /></template>
        </a-input-password>
      </a-form-item>
      <a-button
        type="primary"
        block
        html-type="submit"
        class="submit"
        :loading="loading"
      >
        {{ loading ? $t('auth.loginLoading') : $t('auth.login') }}
      </a-button>

      <div class="tip">
        <template v-if="isDesktop">{{ $t('auth.desktopTip') }}</template>
        <template v-else>
          {{ $t('auth.noAccount') }}<router-link to="/register">{{ $t('auth.registerLink') }}</router-link>{{ $t('auth.noAccountTail') }}
        </template>
      </div>
    </a-form>
  </AuthShell>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { LockKeyhole, UserRound } from 'lucide-vue-next'
import AuthShell from './AuthShell.vue'
import { useAuthStore } from '@/stores/auth'
import { isTauri } from '@/api/request'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()

// 桌面壳之外，开发时用浏览器直连本机引擎（vite dev server 代理到 127.0.0.1:54310）
// 也是本机账号：引擎以 --desktop 启动时同样允许 local 空密码登录，这里放宽提交校验。
// 构建产物中 import.meta.env.DEV 恒为 false，线上行为不变（且后端仍会拒绝非回环请求）。
const isDesktop =
  isTauri() ||
  (import.meta.env.DEV && ['127.0.0.1', 'localhost'].includes(window.location.hostname))
const username = ref(isDesktop ? 'local' : '')
const password = ref('')
const loading = ref(false)

const canSubmit = computed(() => !!username.value.trim() && (!!password.value || isDesktop))

async function submit() {
  if (!canSubmit.value) {
    message.warning(isDesktop ? t('auth.needUsername') : t('auth.needUsernamePwd'))
    return
  }
  if (loading.value) return
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    message.success(t('auth.loginOk'))
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    router.replace(redirect)
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false
  }
}
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
.form :deep(.submit) {
  height: 40px;
  margin-top: 4px;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 4px;
  background: #24945f;
  border-color: #24945f;
  box-shadow: none;
}
.form :deep(.submit:not(:disabled):hover) {
  background: #1f8756;
  border-color: #1f8756;
}
.tip {
  margin-top: 16px;
  padding: 8px 12px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--text2);
  border: 1px solid var(--ok-border);
  background: var(--ok-soft);
  border-radius: 6px;
}
.tip a {
  color: var(--primary);
  font-weight: 600;
  /* 中文标点自带间距，英文「?」后面需要补一点，两种语言都自然 */
  margin-left: 3px;
}
/* 字距只为「登 录」这种两字按钮拉开；:lang 按 <html lang> 生效，多语言都适用 */
.form :deep(.submit:lang(en)) { letter-spacing: 0.2px; }
</style>
