<template>
  <div class="auth-page" :class="{ 'is-desktop': isTauri }">
    <div class="auth-shell">
      <aside class="side" data-tauri-drag-region>
        <div class="side-top">
          <div class="brand">
            <span class="logo-mark">Q</span>
            <span class="brand-name">Qecho</span>
          </div>
          <h2 class="hero">题库 · 练习 · 考试<br />都在一个应用里</h2>
          <p class="side-desc">
            离线可用的本机题库工具，支持 Excel 导入、随机练习、错题复习与局域网在线考试。
          </p>
          <div class="chips">
            <span>题库导入</span>
            <span>随机练习</span>
            <span>错题本</span>
            <span>在线考试</span>
          </div>
        </div>
        <p class="side-foot">© 2026 Qecho · 版权所有</p>
      </aside>

      <section class="main">
        <WindowControls class="win-controls" :maximize="false" />
        <div class="main-inner">
          <div class="head">
            <h1><slot name="title" /></h1>
            <p class="sub"><slot name="subtitle" /></p>
          </div>
          <slot />
          <div v-if="$slots.footer" class="foot"><slot name="footer" /></div>
        </div>
        <p class="main-foot">© 2026 Qecho · 版权所有</p>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import WindowControls from '@/components/WindowControls.vue'

const isTauri = !!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__

onMounted(async () => {
  if (!isTauri) return
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('set_window_size', { width: 840, height: 500, resizable: true })
  } catch {
    /* 尺寸调整失败不影响登录 */
  }
})
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: radial-gradient(circle at 18% 12%, #e6f2ec 0, #eef2f0 42%, #e9eeeb 100%);
}
.auth-shell {
  width: 100%;
  max-width: 920px;
  height: min(580px, calc(100vh - 64px));
  min-height: 540px;
  display: flex;
  background: #fff;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 22px 60px rgba(15, 40, 30, 0.16);
}
.auth-page.is-desktop { padding: 0; }
.auth-page.is-desktop .auth-shell {
  max-width: none;
  width: 100%;
  height: 100vh;
  min-height: 0;
  border-radius: 0;
  box-shadow: none;
}
.auth-page.is-desktop .side { flex: 0 0 34%; max-width: 420px; }
.auth-page.is-desktop .main-inner { max-width: 300px; }

.side {
  position: relative;
  flex: 0 0 40%;
  max-width: 372px;
  padding: 26px 30px 18px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  color: #fff;
  background:
    radial-gradient(circle at 85% 8%, rgba(36, 148, 95, 0.35), transparent 48%),
    radial-gradient(circle at 10% 95%, rgba(36, 148, 95, 0.18), transparent 45%),
    linear-gradient(158deg, #0c1626 0%, #0f1d30 55%, #0b1a24 100%);
}
.side::after {
  content: '';
  position: absolute;
  inset: auto -60px -80px auto;
  width: 220px;
  height: 220px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.06);
}
.brand { display: flex; align-items: center; gap: 10px; }
.logo-mark {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: linear-gradient(160deg, #2fb273, #187a4b);
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 5px 14px rgba(36, 148, 95, 0.4);
}
.brand-name { font-size: 16px; font-weight: 700; letter-spacing: 1px; }
.hero {
  margin: 28px 0 0;
  font-size: 22px;
  line-height: 1.4;
  font-weight: 700;
  letter-spacing: 0.5px;
}
.side-desc {
  margin: 12px 0 0;
  max-width: 270px;
  font-size: 11.5px;
  line-height: 1.75;
  color: rgba(255, 255, 255, 0.62);
}
.chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 18px; }
.chips span {
  padding: 3px 9px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 6px;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.78);
  background: rgba(255, 255, 255, 0.04);
}
.side-foot {
  margin: 0;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.4);
}

.main {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px 40px 36px;
  background: #fff;
}
.win-controls {
  position: absolute;
  top: 12px;
  right: 14px;
  color: #5b6661;
}
.win-controls :deep(.wc:hover) { background: rgba(15, 40, 30, 0.06); }
.win-controls :deep(.wc.close:hover) { background: #dc2626; color: #fff; }
.main-inner { width: 100%; max-width: 300px; }
.head h1 { margin: 0; font-size: 20px; font-weight: 700; color: #17211c; letter-spacing: 0.5px; }
.head .sub { margin: 5px 0 18px; font-size: 12px; color: #6b7570; line-height: 1.6; }
.main-inner :deep(.ant-form-item) { margin-bottom: 14px; }
.main-inner :deep(.ant-form-item-label > label) { font-size: 12.5px; height: 24px; }
.foot {
  margin-top: 14px;
  text-align: center;
  font-size: 12px;
  color: #6b7570;
}
.foot :deep(a) { color: #24945f; font-weight: 600; margin-left: 4px; }
.main-foot {
  display: none;
  margin: 24px 0 0;
  text-align: center;
  font-size: 11.5px;
  color: #a3ada8;
}

@media (max-width: 760px) {
  .auth-page { padding: 16px; }
  .side { display: none; }
  .auth-shell { max-width: 460px; height: auto; min-height: 0; border-radius: 16px; }
  .main { padding: 52px 28px 36px; }
  .main-foot { display: block; }
}
</style>
