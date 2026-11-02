import { computed, ref } from 'vue'
import { defineStore, type Pinia } from 'pinia'
import { authApi } from '@/api/auth'
import { setToken as persistToken, getToken, isTauri } from '@/api/request'

const USER_KEY = 'quiz-user'
const ROLE_KEY = 'quiz-role'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(getToken())
  const username = ref<string>(localStorage.getItem(USER_KEY) ?? '')
  const role = ref<'admin' | 'user' | ''>((localStorage.getItem(ROLE_KEY) as 'admin' | 'user') ?? '')
  // 能力位由管理员授予，登录后由 /auth/login 或 /auth/me 下发。
  const canCreateBank = ref(false)
  const canExportImport = ref(false)

  const isAdmin = computed(() => role.value === 'admin')
  const loggedIn = computed(() => !!token.value)
  const mayCreateBank = computed(() => isAdmin.value || canCreateBank.value)
  const mayExportImport = computed(() => isAdmin.value || canExportImport.value)

  function apply(
    tokenValue: string,
    name: string,
    roleValue: 'admin' | 'user',
    createBank = false,
    exportImport = false,
  ) {
    token.value = tokenValue
    username.value = name
    role.value = roleValue
    canCreateBank.value = createBank
    canExportImport.value = exportImport
    persistToken(tokenValue)
    localStorage.setItem(USER_KEY, name)
    localStorage.setItem(ROLE_KEY, roleValue)
  }

  function adopt() {
    token.value = getToken()
    username.value = localStorage.getItem(USER_KEY) ?? username.value
    const stored = localStorage.getItem(ROLE_KEY)
    if (stored === 'admin' || stored === 'user') role.value = stored
  }

  async function login(user: string, password: string) {
    const r = await authApi.login(user, password)
    apply(r.token, r.username, r.role, r.canCreateBank, r.canExportImport)
  }

  async function register(data: {
    username: string
    password: string
    captchaId: string
    captchaAnswer: string
  }) {
    const r = await authApi.register(data)
    apply(r.token, r.username, r.role, r.canCreateBank, r.canExportImport)
  }

  async function refresh() {
    const profile = await authApi.me()
    apply(getToken(), profile.username, profile.role, profile.canCreateBank, profile.canExportImport)
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      /* 忽略 */
    }
    clear()
  }

  function clear() {
    token.value = ''
    username.value = ''
    role.value = ''
    canCreateBank.value = false
    canExportImport.value = false
    persistToken('')
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(ROLE_KEY)
  }

  return {
    token,
    username,
    role,
    isAdmin,
    loggedIn,
    canCreateBank,
    canExportImport,
    mayCreateBank,
    mayExportImport,
    login,
    register,
    refresh,
    logout,
    clear,
    adopt,
  }
})

/**
 * 网页版校验已保存的登录态；桌面壳每次启动都进入登录页（本机账号预填、可留空密码）。
 * 必须在 app.use(router) 之前调用，内部不得使用 useRouter()。
 */
export async function ensureSession(pinia: Pinia): Promise<void> {
  const auth = useAuthStore(pinia)
  if (!isTauri()) {
    if (auth.token) {
      try {
        await auth.refresh()
      } catch {
        auth.clear()
      }
    }
    return
  }
  // 桌面端不静默登录，让用户看到登录页；旧 token 一并清理。
  if (auth.token) auth.clear()
}
