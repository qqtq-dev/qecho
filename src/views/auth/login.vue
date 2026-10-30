<template>
  <AuthShell>
    <template #title>登录</template>
    <template #subtitle>
      {{ isDesktop ? '本机账号默认无需密码，直接登录即可' : '登录后可管理题库、发起练习与考试' }}
    </template>

    <a-form layout="vertical" class="form" @submit.prevent="submit">
      <a-form-item label="用户名">
        <a-input
          v-model:value="username"
          placeholder="请输入用户名"
          autocomplete="username"
          :maxlength="20"
        >
          <template #prefix><UserRound :size="16" /></template>
        </a-input>
      </a-form-item>
      <a-form-item label="密码">
        <a-input-password
          v-model:value="password"
          :placeholder="isDesktop ? '本机账号可留空' : '请输入密码'"
          autocomplete="current-password"
        >
          <template #prefix><LockKeyhole :size="16" /></template>
        </a-input-password>
      </a-form-item>
      <a-button
        type="primary"
        block
        html-type="submit"
        class="submit"
        :loading="loading"
        :disabled="!canSubmit"
      >
        登 录
      </a-button>
    </a-form>

    <template #footer>
      <template v-if="isDesktop">
        本机模式 · 数据保存在这台电脑
      </template>
      <template v-else>
        还没有账号？<router-link to="/register">立即注册</router-link>
      </template>
    </template>
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
  if (!canSubmit.value) return
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
.form :deep(.ant-form-item) { margin-bottom: 20px; }
.submit {
  height: 46px;
  border-radius: 8px;
  font-size: 15.5px;
  font-weight: 600;
  letter-spacing: 2px;
  box-shadow: 0 8px 20px rgba(36, 148, 95, 0.25);
}
</style>
