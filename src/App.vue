<template>
  <a-config-provider
    :locale="zhCN"
    :theme="{
      algorithm: antdAlgorithm(),
      token: { colorPrimary: '#24945f', colorSuccess: '#24945f', borderRadius: 6 },
    }"
  >
    <router-view />
  </a-config-provider>
</template>

<script setup lang="ts">
import zhCN from 'ant-design-vue/es/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import { antdAlgorithm } from '@/theme'

dayjs.locale('zh-cn')
</script>

<style>
:root {
  --bg: #f4f6f7;
  --surface: #ffffff;
  --border: #e4e8e6;
  --text: #1f2329;
  --text2: #6f7680;
  --hover: #f0f5f2;
  --primary: #24945f;
  --primary-soft: #e8f4ee;
  --zebra: #fafcfb;
  /* 次级表面与语义色：深色模式统一在此覆盖，组件内不再写死浅色 */
  --surface-2: #fafcfb;
  --surface-3: #f6f8f7;
  --border-strong: #e2e6e4;
  --chip-bg: #eef2f0;
  --chip-text: #4b5563;
  --ok-soft: #e8f5ee;
  --ok-border: #cfe8da;
  --ok-text: #1b7a4d;
  --bad-soft: #fde6e0;
  --bad-border: #ffd4d1;
  --bad-text: #e5352b;
  --warn-soft: #fff8e6;
  --warn-text: #8a6100;
  /* 标记（待回顾）语义色：图例、答题卡角标、顶栏旗子共用同一组 */
  --mark: #fa8c16;
  --mark-soft: #faf9de;
  --mark-border: #ffd08f;
  --mark-text: #ad5200;
  /* 滚动条滑块：细、浅绿，轨道透明 */
  --scroll-thumb: rgba(36, 148, 95, .35);
  --scroll-thumb-hover: rgba(36, 148, 95, .6);
  --code-bg: #f1f3f2;
  --auth-bg: radial-gradient(circle at 18% 12%, #e6f2ec 0, #eef2f0 42%, #e9eeeb 100%);
  /* 顶部导航栏高度：抽屉、顶部浮层统一让出这段空间 */
  --topbar-h: 46px;
}
html.dark {
  --bg: #0f1216;
  --surface: #161b22;
  --border: #2a313b;
  --text: #e6e8eb;
  --text2: #98a1ac;
  --hover: #1d232c;
  --primary: #35b07a;
  --primary-soft: #173326;
  --zebra: #1a2029;
  --surface-2: #1a2029;
  --surface-3: #1e242d;
  --border-strong: #39424f;
  --chip-bg: #242b34;
  --chip-text: #a3acb8;
  --ok-soft: #173325;
  --ok-border: #2f6b4c;
  --ok-text: #63c894;
  --bad-soft: #3a1a18;
  --bad-border: #a8403a;
  --bad-text: #ff7466;
  --warn-soft: #2e2612;
  --warn-text: #e3b558;
  --mark: #f0a13c;
  --mark-soft: #33270f;
  --mark-border: #7d5820;
  --mark-text: #ffc069;
  --scroll-thumb: rgba(83, 196, 145, .42);
  --scroll-thumb-hover: rgba(83, 196, 145, .7);
  --code-bg: #232b35;
  --auth-bg: radial-gradient(circle at 18% 12%, #16241d 0, #12171c 42%, #0f1216 100%);
}
html, body, #app { height: 100%; margin: 0; }
html { color-scheme: light; }
html.dark { color-scheme: dark; }
html.theme-switching *,
html.theme-switching *::before,
html.theme-switching *::after {
  transition: none !important;
  animation: none !important;
}
body {
  font-family: 'Segoe UI', 'Microsoft YaHei', system-ui, sans-serif;
  background: var(--bg);
  color: var(--text);
}
.text2 { color: var(--text2); }

.page { max-width: none; }
.panel { background: var(--surface); border: 1px solid var(--border); border-radius: 6px; }
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--border);
}
.panel-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.panel-body { padding: 16px; }
.panel .empty-state {
  min-height: 320px; display: flex; align-items: center; justify-content: center;
}
.muted { color: var(--text2); font-size: 12px; }
.mono { font-family: Consolas, 'Courier New', monospace; }

.ant-table-thead > tr > th:last-child,
.ant-table-tbody > tr > td:last-child {
  white-space: nowrap;
}

/* 全局消息提示：卡片化，颜色跟随主题；错误态与答题卡同一套红 */
.ant-message { top: 60px; }
.ant-message .ant-message-notice .ant-message-notice-content {
  padding: 11px 15px;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--border-strong);
  box-shadow: 0 10px 28px rgba(16, 28, 22, .12);
  color: var(--text);
  font-size: 13.5px;
  line-height: 1.55;
  text-align: left;
}
.ant-message .ant-message-custom-content { align-items: center; }
.ant-message .ant-message-custom-content .anticon { font-size: 15px; }
.ant-message .ant-message-notice .ant-message-notice-content:has(.ant-message-warning),
.ant-message .ant-message-notice .ant-message-notice-content:has(.ant-message-error) {
  background: var(--bad-soft);
  border-color: var(--bad-border);
}
.ant-message .ant-message-notice .ant-message-notice-content:has(.ant-message-warning) .anticon,
.ant-message .ant-message-notice .ant-message-notice-content:has(.ant-message-error) .anticon { color: var(--bad-text); }
.ant-message .ant-message-notice .ant-message-notice-content:has(.ant-message-success) .anticon { color: var(--ok-text); }
html.dark .ant-message .ant-message-notice .ant-message-notice-content { box-shadow: 0 12px 32px rgba(0, 0, 0, .5); }
</style>
