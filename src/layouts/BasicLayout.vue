<template>
  <a-layout class="layout">
    <header class="topbar" data-tauri-drag-region>
      <div class="tb-left">
        <div class="logo">
          <span class="logo-mark">Q</span>
          <span class="logo-text">Qecho</span>
        </div>
      </div>
      <div class="tb-right">
        <a-tooltip :title="healthTitle">
          <span class="engine-tag" :class="{ off: !online }">
            <span class="dot"></span>
            {{ online ? $t('topbar.online') : $t('topbar.offline') }}
          </span>
        </a-tooltip>
        <a-popover
          :open="noticeOpen"
          placement="bottomRight"
          trigger="click"
          :overlay-style="{ width: '320px' }"
          @open-change="onNoticeOpenChange"
        >
          <template #content>
            <!-- key 绑内容指纹：异步数据（升级卡片/通知列表）到达后节点重建，antd 重新测量定位，
                 否则气泡按弹出时的矮内容定位，卡片插进来会底部截断/箭头错位 -->
            <div :key="`${hasUpdate ? 1 : 0}-${notices.length}`" class="notice-wrap">
              <div v-if="hasUpdate" class="update-card">
                <div class="update-top">
                  <Download :size="13" />
                  <span class="update-title">{{ $t('topbar.newVersion') }}</span>
                  <span class="update-ver">v{{ updateInfo?.latest }}</span>
                </div>
                <div v-if="updateInfo?.notes" class="update-notes">{{ updateInfo.notes }}</div>
                <div class="update-foot">
                  <span class="update-cur">{{ $t('topbar.currentVersion', { version: updateInfo?.current }) }}</span>
                  <a-button size="small" type="primary" @click="openRelease">{{ $t('topbar.viewUpdate') }}</a-button>
                </div>
              </div>
              <div class="notice-box">
                <a-empty v-if="!notices.length" :description="$t('topbar.noNotices')" :image="false" />
                <div v-for="item in notices" :key="item.id" class="notice-item">
                  <div class="notice-text">{{ item.text }}</div>
                  <div class="notice-time">{{ formatNoticeTime(item.createdAt) }}</div>
                </div>
              </div>
            </div>
          </template>
          <a-badge :count="unread" :offset="[-2, 2]" size="small">
            <button class="ghost-btn" :class="{ 'dot-new': updateDot }" :title="bellTitle">
              <Bell :size="15" />
            </button>
          </a-badge>
        </a-popover>
        <a-tooltip :title="$t('topbar.toggleSider')">
          <button class="ghost-btn" @click="collapsed = !collapsed">
            <PanelLeftOpen v-if="collapsed" :size="15" />
            <PanelLeftClose v-else :size="15" />
          </button>
        </a-tooltip>
        <a-tooltip :title="isDark ? $t('topbar.toLight') : $t('topbar.toDark')">
          <button class="ghost-btn" @click="toggleTheme">
            <Moon v-if="!isDark" :size="15" />
            <Sun v-else :size="15" />
          </button>
        </a-tooltip>
        <a-dropdown placement="bottomRight">
          <button class="ghost-btn user-btn">
            <UserRound :size="15" />
            <span class="user-name">{{ auth.username || $t('topbar.notLoggedIn') }}</span>
          </button>
          <template #overlay>
            <a-menu>
              <a-menu-item key="role" disabled>
                {{ auth.isAdmin ? $t('topbar.admin') : $t('topbar.user') }}
              </a-menu-item>
              <a-menu-item v-if="desktop" key="desktop" disabled>{{ $t('topbar.localAccount') }}</a-menu-item>
              <a-menu-divider />
              <a-menu-item key="logout" @click="onLogout">{{ $t('topbar.logout') }}</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
        <!-- 右上角贴着窗口边缘：气泡改成右对齐，否则默认居中会把浮层顶出视口，
             连带着出现横向滚动条、整页跟着位移 -->
        <a-tooltip :title="$t('topbar.settings')" placement="bottomRight">
          <button class="ghost-btn" @click="go('/settings')"><Settings :size="15" /></button>
        </a-tooltip>
        <WindowControls />
      </div>
    </header>

    <div class="body">
      <aside class="sider" :class="{ collapsed }">
        <nav class="side-nav">
          <a-menu
            mode="inline"
            :theme="isDark ? 'dark' : 'light'"
            :selected-keys="[selectedKey]"
            :inline-collapsed="collapsed"
            :items="menuItems"
            @click="onMenuClick"
          />
        </nav>
        <div v-if="!collapsed" class="sider-foot">{{ $t('app.copyright') }}</div>
      </aside>
      <main class="content">
        <router-view />
      </main>
    </div>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { MenuProps } from 'ant-design-vue'
import {
  Bell,
  BookX,
  ChartColumn,
  DatabaseBackup,
  Download,
  LayoutDashboard,
  Library,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  SquarePen,
  Sun,
  Timer,
  UserRound,
  Users,
} from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isTauri } from '@/api/request'
import { noticesApi, type Notice } from '@/api/notices'
import { updateApi, type UpdateInfo } from '@/api/update'
import { isDark, toggleTheme } from '@/theme'
import WindowControls from '@/components/WindowControls.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const auth = useAuthStore()
const desktop = isTauri()
const { t, locale } = useI18n()

const collapsed = ref(localStorage.getItem('quiz-sider-collapsed') === '1')
watch(collapsed, (v) => localStorage.setItem('quiz-sider-collapsed', v ? '1' : '0'))

const menuItems = computed<MenuProps['items']>(() => {
  // 读一次 locale，语言切换时让这份菜单跟着重算
  void locale.value
  const items: NonNullable<MenuProps['items']> = [
    { key: '/dashboard', icon: () => h(LayoutDashboard, { size: 15 }), label: t('nav.dashboard') },
    { key: '/banks', icon: () => h(Library, { size: 15 }), label: t('nav.banks') },
    { key: '/practice', icon: () => h(SquarePen, { size: 15 }), label: t('nav.practice') },
    { key: '/exams', icon: () => h(Timer, { size: 15 }), label: t('nav.exams') },
    { key: '/wrong', icon: () => h(BookX, { size: 15 }), label: t('nav.wrong') },
    { key: '/stats', icon: () => h(ChartColumn, { size: 15 }), label: t('nav.stats') },
  ]
  // 用户管理与数据备份只对管理员开放，普通用户看不到入口。
  if (auth.isAdmin) {
    items.push(
      { key: '/users', icon: () => h(Users, { size: 15 }), label: t('nav.users') },
      { key: '/backups', icon: () => h(DatabaseBackup, { size: 15 }), label: t('nav.backups') },
    )
  }
  items.push({ key: '/settings', icon: () => h(Settings, { size: 15 }), label: t('nav.settings') })
  return items
})

const selectedKey = computed(() => (route.path.startsWith('/banks/') ? '/banks' : route.path))
const online = computed(() => appStore.health?.status === 'ok')
const healthTitle = computed(() =>
  online.value
    ? t('topbar.tipOnline', { version: appStore.health?.version ?? '-' })
    : t('topbar.tipOffline'),
)

function onMenuClick({ key }: { key: string | number }) {
  go(String(key))
}

function go(path: string) {
  if (route.path !== path) router.push(path)
}

async function onLogout() {
  await auth.logout()
  router.replace('/login')
}

let timer: number | undefined

// —— 通知 ——
const notices = ref<Notice[]>([])
const unread = ref(0)
const noticeOpen = ref(false)

// —— 版本更新 ——
const updateInfo = ref<UpdateInfo | null>(null)
const hasUpdate = computed(() => !!updateInfo.value?.hasUpdate)
// 未读通知的数字优先占用角标，此时版本更新不再额外画点
const updateDot = computed(() => hasUpdate.value && unread.value === 0)
const bellTitle = computed(() =>
  hasUpdate.value ? t('topbar.bellWithUpdate', { version: updateInfo.value?.latest }) : t('topbar.bell'),
)

async function checkUpdate(force = false) {
  if (!auth.loggedIn) return
  try {
    updateInfo.value = await updateApi.check(force)
  } catch {
    /* 拿不到版本信息不影响使用，静默失败 */
  }
}

async function openRelease() {
  const url = updateInfo.value?.downloadUrl || updateInfo.value?.url
  if (!url) return
  if (desktop) {
    try {
      const { invoke } = await import('@tauri-apps/api/core')
      await invoke('open_external', { url })
      return
    } catch {
      /* 壳层打不开时退回浏览器方式 */
    }
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

async function loadNotices() {
  if (!auth.loggedIn) return
  try {
    const res = await noticesApi.list()
    notices.value = res.items
    unread.value = res.unread
  } catch {
    /* 通知拉取失败不打断使用 */
  }
}

async function onNoticeOpenChange(open: boolean) {
  noticeOpen.value = open
  if (!open) return
  await loadNotices()
  // 用户主动打开铃铛时绕过引擎缓存，确保刚发布的新版本能立刻看到
  void checkUpdate(true)
  if (unread.value > 0) {
    try {
      await noticesApi.readAll()
      unread.value = 0
      notices.value = notices.value.map((item) => ({ ...item, read: true }))
    } catch {
      /* 标记已读失败时保留未读态 */
    }
  }
}

function formatNoticeTime(raw: string) {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getMonth() + 1}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

// —— 权限位刷新 ——
// 管理员改了「建题库/导出导入」或停用账号后，前端要跟上；切页与窗口聚焦时轻量拉一次。
let lastProfileAt = 0
async function refreshProfile() {
  if (!auth.loggedIn) return
  const now = Date.now()
  if (now - lastProfileAt < 15_000) return
  lastProfileAt = now
  try {
    await auth.refresh()
  } catch {
    /* 会话失效由请求层统一处理 */
  }
}

watch(
  () => route.path,
  () => {
    void refreshProfile()
    void loadNotices()
  },
)

function onWindowFocus() {
  void refreshProfile()
  void loadNotices()
}

onMounted(() => {
  appStore.refresh()
  void loadNotices()
  void checkUpdate()
  timer = window.setInterval(() => {
    appStore.refresh()
    void refreshProfile()
    void loadNotices()
  }, 30_000)
  window.addEventListener('focus', onWindowFocus)
  resizeMain()
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  window.removeEventListener('focus', onWindowFocus)
})

async function resizeMain() {
  if (!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__) return
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('set_window_size', { width: 960, height: 660, resizable: true })
  } catch {
    /* 尺寸调整失败不阻塞使用 */
  }
}
</script>

<style scoped>
.layout { height: 100vh; display: flex; flex-direction: column; }

.topbar {
  height: var(--topbar-h); flex: 0 0 var(--topbar-h);
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 14px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  position: relative; z-index: 5;
}
.tb-left { display: flex; align-items: center; gap: 12px; }
.logo { display: flex; align-items: center; gap: 8px; user-select: none; }
.logo-mark {
  width: 26px; height: 26px; border-radius: 6px; background: var(--primary);
  color: #fff; font-weight: 700; font-size: 14px;
  display: inline-flex; align-items: center; justify-content: center;
}
.logo-text { font-size: 14px; font-weight: 600; }
.tb-right { display: flex; align-items: center; gap: 6px; }
.ghost-btn {
  width: 28px; height: 28px; border: none; border-radius: 6px;
  background: transparent; color: var(--text); position: relative;
  cursor: pointer; display: inline-flex; align-items: center; justify-content: center;
  transition: background-color 0.15s;
}
.ghost-btn:hover { background: var(--hover); }
.user-btn { width: auto; gap: 6px; padding: 0 8px; }
.user-name { font-size: 12px; max-width: 96px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.engine-tag {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 3px 9px; border-radius: 999px;
  background: var(--primary-soft); color: var(--primary);
  font-size: 12px; user-select: none;
}
.engine-tag .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--primary); }
.engine-tag.off { background: var(--bad-soft); color: var(--bad-text); }
.engine-tag.off .dot { background: var(--bad-text); }

.notice-wrap { display: flex; flex-direction: column; gap: 10px; }
.notice-box { max-height: 280px; overflow: auto; }
.notice-item { padding: 8px 4px; border-bottom: 1px solid var(--border); }
.notice-item:last-child { border-bottom: 0; }
.notice-text { font-size: 13px; line-height: 1.5; }
.notice-time { margin-top: 2px; font-size: 11px; color: var(--text2); }

/* 新版本提示：整块卡片置顶，和下面的通知列表分开 */
.update-card {
  padding: 10px 11px;
  border-radius: 8px;
  background: var(--mark-soft);
  border: 1px solid var(--mark-border);
}
.update-top { display: flex; align-items: center; gap: 6px; color: var(--mark-text); }
.update-title { font-size: 12.5px; font-weight: 600; }
.update-ver {
  margin-left: auto; padding: 0 6px; border-radius: 999px;
  background: var(--mark); color: #fff; font-size: 11px; line-height: 17px;
}
.update-notes {
  margin-top: 6px; font-size: 12px; line-height: 1.55; color: var(--text2);
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.update-foot {
  margin-top: 9px; display: flex; align-items: center; justify-content: space-between; gap: 8px;
}
.update-cur { font-size: 11px; color: var(--text2); }

/* 未读数字已经被角标占用时改用小圆点，两者互斥不会重叠 */
.ghost-btn.dot-new::after {
  content: ''; position: absolute; top: 5px; right: 5px;
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--mark);
  box-shadow: 0 0 0 2px var(--surface);
}

.body { flex: 1; display: flex; min-height: 0; }

.sider {
  width: 208px; flex: 0 0 208px;
  display: flex; flex-direction: column;
  background: var(--surface);
  border-right: 1px solid var(--border);
  transition: width 0.18s ease, flex-basis 0.18s ease;
  overflow: hidden;
}
.sider.collapsed { width: 64px; flex-basis: 64px; }
.side-nav { flex: 1; background: var(--surface); overflow: hidden; }
:deep(.side-nav .ant-menu) { background: transparent; border-inline-end: 0 !important; width: 100%; }
:deep(.side-nav .ant-menu-inline-collapsed) { width: 100%; }
.sider-foot {
  flex: 0 0 auto; padding: 12px 16px 14px;
  font-size: 11px; color: var(--text2); user-select: none;
}

.content { flex: 1; overflow: auto; padding: 16px 18px; }
@media (max-width: 720px) {
  .content { min-width: 0; padding: 14px 12px; }
  .topbar { padding-inline: 10px; }
  .tb-right { gap: 2px; }
  .engine-tag { display: none; }
}
</style>
