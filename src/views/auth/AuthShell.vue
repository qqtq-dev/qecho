<template>
  <div class="auth-page" :class="{ 'is-desktop': isTauri }">
    <div class="auth-shell">
      <aside class="side" data-tauri-drag-region>
        <div class="side-top">
          <div class="brand">
            <span class="logo-mark">Q</span>
            <span class="brand-name">Qecho</span>
          </div>
          <h2 class="hero">题库导入、随机练习、在线考试全都有<br />一个应用全部搞定</h2>
          <p class="side-desc">
            Excel 批量导入题库，按题型、难度、标签筛选组卷；随机练习逐题判分，错题自动收录、连对自动移出；局域网在线考试连手机就能答。数据全部保存在本机，离线也能用。
          </p>
          <p class="side-desc">
            题库、题图与练习记录都可整体导出，换台电脑导入即可接着练习。
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
    await invoke('set_window_size', { width: 800, height: 500, resizable: true })
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
  background: var(--auth-bg);
}
.auth-shell {
  width: 100%;
  max-width: 800px;
  height: min(500px, calc(100vh - 48px));
  display: flex;
  background: var(--surface);
  border-radius: 20px;
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
.auth-page.is-desktop .main-inner { max-width: 340px; }

.side {
  position: relative;
  flex: 0 0 47%;
  max-width: 380px;
  padding: 38px 26px 16px;
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
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(160deg, #2fb273, #187a4b);
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 5px 14px rgba(36, 148, 95, 0.4);
}
.brand-name { font-size: 15px; font-weight: 700; letter-spacing: 1px; }
.hero {
  margin: 18px 0 0;
  font-size: 18px;
  line-height: 1.4;
  font-weight: 700;
  letter-spacing: 0.5px;
}
.side-desc {
  margin: 12px 0 0;
  font-size: 12.5px;
  line-height: 1.75;
  color: rgba(255, 255, 255, 0.68);
}
.chips { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 16px; }
.chips span {
  padding: 4px 11px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 7px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.82);
  background: rgba(255, 255, 255, 0.05);
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
  padding: 28px 40px;
  background: var(--surface);
}
.win-controls {
  position: absolute;
  top: 12px;
  right: 14px;
  color: var(--text2);
}
.win-controls :deep(.wc:hover) { background: var(--hover); }
.win-controls :deep(.wc.close:hover) { background: #dc2626; color: #fff; }
.main-inner { width: 100%; max-width: 340px; }
.head { margin-bottom: 0; }
.head h1 { margin: 0 0 6px; font-size: 20px; font-weight: 600; color: var(--text); letter-spacing: 0; }
.head .sub { margin: 0 0 24px; font-size: 13px; color: var(--text2); line-height: 1.6; }
.main-inner :deep(.ant-form-item) { margin-bottom: 14px; }
.main-inner :deep(.ant-form-item-label) { padding: 0 0 2px; }
.main-inner :deep(.ant-form-item-label > label) { font-size: 12.5px; height: 22px; }
.foot {
  margin-top: 14px;
  text-align: center;
  font-size: 12px;
  color: var(--text2);
}
.foot :deep(a) { color: var(--primary); font-weight: 600; margin-left: 4px; }
.main-foot {
  display: none;
  margin: 24px 0 0;
  text-align: center;
  font-size: 11.5px;
  color: var(--text2);
}

@media (max-width: 760px) {
  .auth-page { padding: 16px; }
  .side { display: none; }
  .auth-shell { max-width: 460px; height: auto; min-height: 0; border-radius: 16px; }
  .main { padding: 52px 28px 36px; }
  .main-foot { display: block; }
}
</style>
