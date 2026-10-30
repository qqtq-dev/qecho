<template>
  <AuthShell>
    <template #title>账号登录</template>
    <template #subtitle>
      {{ isDesktop ? '本机账号默认免密码，直接登录即可' : '登录后可管理题库、发起练习与考试' }}
    </template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item label="用户名">
        <a-input
          v-model:value="username"
          :placeholder="isDesktop ? '本机账号 local' : '请输入用户名'"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="15" /></template>
        </a-input>
      </a-form-item>
      <a-form-item label="密码">
        <a-input-password
          v-model:value="password"
          :placeholder="isDesktop ? '本机可留空' : '请输入密码'"
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
        {{ loading ? '登录中…' : '登 录' }}
      </a-button>

      <div class="tip">
        <template v-if="isDesktop">本机模式 · 账号数据保存在这台电脑</template>
        <template v-else>
          还没有账号？<router-link to="/register">立即注册</router-link>，注册需填写图片验证码。
        </template>
      </div>
    </a-form>
  </AuthShell>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { LockKeyhole, UserRound } from 'lucide-vue-next'
import AuthShell from './AuthShell.vue'
import { useAuthStore } from '@/stores/auth'
import { isTauri } from '@/api/request'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const isDesktop = isTauri()
const username = ref(isDesktop ? 'local' : '')
const password = ref('')
const loading = ref(false)

const canSubmit = computed(() => !!username.value.trim() && (!!password.value || isDesktop))

async function submit() {
  if (!canSubmit.value) {
    message.warning(isDesktop ? '请输入用户名' : '请输入用户名和密码')
    return
  }
  if (loading.value) return
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    message.success('登录成功')
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
  color: rgba(0, 0, 0, 0.45);
  border: 1px solid #cbe3d6;
  background: #f3faf6;
  border-radius: 6px;
}
.tip a {
  color: #24945f;
  font-weight: 600;
}
</style>
