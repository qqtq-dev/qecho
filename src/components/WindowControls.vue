<template>
  <div v-if="isTauri" class="win-controls">
    <button v-if="minimize" class="wc" title="最小化" @click="min">
      <Minus :size="14" />
    </button>
    <button v-if="maximize" class="wc" title="最大化 / 还原" @click="toggleMax">
      <Square :size="13" />
    </button>
    <button v-if="close" class="wc close" title="关闭" @click="close">
      <X :size="14" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { Minus, Square, X } from 'lucide-vue-next'
withDefaults(defineProps<{ minimize?: boolean; maximize?: boolean; close?: boolean }>(), {
  minimize: true,
  maximize: true,
  close: true,
})

const isTauri = !!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__

async function win() {
  return (await import('@tauri-apps/api/window')).getCurrentWindow()
}
const min = async () => (await win()).minimize()
const toggleMax = async () => (await win()).toggleMaximize()
const close = async () => (await win()).close()
</script>

<style scoped>
.win-controls { display: flex; align-items: center; gap: 2px; }
.wc {
  width: 30px; height: 28px; border: none; border-radius: 6px;
  background: transparent; color: inherit;
  display: inline-flex; align-items: center; justify-content: center;
  cursor: pointer; transition: background-color 0.12s;
}
.wc:hover { background: var(--hover); }
.wc.close:hover { background: #dc2626; color: #fff; }
</style>
