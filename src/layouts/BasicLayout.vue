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
            {{ online ? '服务正常' : '服务异常' }}
          </span>
        </a-tooltip>
        <a-tooltip title="收起 / 展开侧栏">
          <button class="ghost-btn" @click="collapsed = !collapsed">
            <PanelLeftOpen v-if="collapsed" :size="15" />
            <PanelLeftClose v-else :size="15" />
          </button>
        </a-tooltip>
        <a-tooltip :title="isDark ? '切换到浅色' : '切换到深色'">
          <button class="ghost-btn" @click="toggleTheme">
            <Moon v-if="!isDark" :size="15" />
            <Sun v-else :size="15" />
          </button>
        </a-tooltip>
        <a-dropdown placement="bottomRight">
          <button class="ghost-btn user-btn">
            <UserRound :size="15" />
            <span class="user-name">{{ auth.username || '未登录' }}</span>
          </button>
          <template #overlay>
            <a-menu>
              <a-menu-item key="role" disabled>
                {{ auth.isAdmin ? '管理员' : '普通用户' }}
              </a-menu-item>
              <a-menu-item v-if="desktop" key="desktop" disabled>本机账号</a-menu-item>
              <a-menu-divider />
              <a-menu-item key="logout" @click="onLogout">退出登录</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
        <a-tooltip title="设置">
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
        <div v-if="!collapsed" class="sider-foot">© 2026 Qecho · 版权所有</div>
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
import type { MenuProps } from 'ant-design-vue'
import {
  BookX,
  ChartColumn,
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
} from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isTauri } from '@/api/request'
import { isDark, toggleTheme } from '@/theme'
import WindowControls from '@/components/WindowControls.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const auth = useAuthStore()
const desktop = isTauri()

const collapsed = ref(localStorage.getItem('quiz-sider-collapsed') === '1')
watch(collapsed, (v) => localStorage.setItem('quiz-sider-collapsed', v ? '1' : '0'))

const menuItems = computed<MenuProps['items']>(() => [
  { key: '/dashboard', icon: () => h(LayoutDashboard, { size: 15 }), label: '概览' },
  { key: '/banks', icon: () => h(Library, { size: 15 }), label: '题库管理' },
  { key: '/practice', icon: () => h(SquarePen, { size: 15 }), label: '练习' },
  { key: '/exams', icon: () => h(Timer, { size: 15 }), label: '考试' },
  { key: '/wrong', icon: () => h(BookX, { size: 15 }), label: '错题本' },
  { key: '/stats', icon: () => h(ChartColumn, { size: 15 }), label: '统计' },
  { key: '/settings', icon: () => h(Settings, { size: 15 }), label: '设置' },
])

const selectedKey = computed(() => (route.path.startsWith('/banks/') ? '/banks' : route.path))
const online = computed(() => appStore.health?.status === 'ok')
const healthTitle = computed(() =>
  online.value
    ? `引擎正常 · v${appStore.health?.version ?? '-'}`
    : '引擎未就绪，请检查诊断日志',
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

onMounted(() => {
  appStore.refresh()
  timer = window.setInterval(() => appStore.refresh(), 30_000)
  resizeMain()
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
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
  height: 46px; flex: 0 0 46px;
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
  background: transparent; color: var(--text);
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
