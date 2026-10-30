<template>
  <div class="metric-tile">
    <div class="mt-top">
      <span class="mt-label">{{ label }}</span>
      <span v-if="$slots.icon" class="mt-icon"><slot name="icon" /></span>
    </div>
    <div class="mt-value">{{ value }}</div>
    <div v-if="hint" class="mt-hint">{{ hint }}</div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ label: string; value: string | number; hint?: string }>()
</script>

<style scoped>
.metric-tile {
  padding: 16px 18px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
}
.mt-top { display: flex; align-items: center; justify-content: space-between; }
.mt-label { font-size: 12px; color: var(--text2); }
.mt-icon { color: var(--primary); display: inline-flex; }
.mt-value { margin-top: 8px; font-size: 26px; font-weight: 650; letter-spacing: 0.2px; }
.mt-hint { margin-top: 4px; font-size: 11px; color: var(--text2); }
</style>
