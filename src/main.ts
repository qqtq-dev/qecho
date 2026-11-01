import { createApp } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'
import App from './App.vue'
import router from './router'
import { initApiBase } from './api/request'
import { ensureSession } from './stores/auth'

function isTauri() {
  return !!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
}

async function showWindow() {
  if (!isTauri()) return
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('set_window_size', { width: 840, height: 500, resizable: true })
  } catch {
    /* 显示失败不阻塞后续 */
  }
}

function renderBootError(err: unknown) {
  document.getElementById('boot-splash')?.remove()
  const root = document.getElementById('app')
  if (!root) return
  root.innerHTML = ''
  const wrap = document.createElement('div')
  wrap.style.cssText =
    'height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;background:#f4f6f7;color:#1f2329;font-family:"Segoe UI","Microsoft YaHei",sans-serif;'
  const title = document.createElement('div')
  title.textContent = '本地服务未启动'
  title.style.cssText = 'font-size:20px;font-weight:600;'
  const detail = document.createElement('div')
  detail.textContent = err instanceof Error ? err.message : String(err || '未知错误')
  detail.style.cssText = 'font-size:13px;color:#6f7680;max-width:560px;text-align:center;'
  const retry = document.createElement('button')
  retry.textContent = '重新连接'
  retry.style.cssText =
    'margin-top:8px;padding:7px 18px;border:0;border-radius:6px;background:#24945f;color:#fff;cursor:pointer;font-size:13px;'
  retry.onclick = () => location.reload()
  wrap.append(title, detail, retry)
  root.appendChild(wrap)
}

async function bootstrap() {
  await initApiBase()

  const pinia = createPinia()
  setActivePinia(pinia)

  try {
    await ensureSession(pinia)
  } catch (err) {
    console.error('[bootstrap] session failed', err)
    if (isTauri()) {
      await showWindow()
      renderBootError(err)
      return
    }
  }

  const app = createApp(App)
  app.use(pinia)
  app.use(router)
  app.use(Antd)
  app.mount('#app')
  document.getElementById('boot-splash')?.remove()

  if (isTauri()) {
    try {
      const { invoke } = await import('@tauri-apps/api/core')
      const behavior = localStorage.getItem('quiz-close-behavior') ?? 'minimize'
      await invoke('set_close_behavior', { behavior })
    } catch {
      /* 忽略 */
    }
  }
}

bootstrap()
