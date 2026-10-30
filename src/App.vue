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
</style>
