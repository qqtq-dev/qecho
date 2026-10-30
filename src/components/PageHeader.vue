<template>
  <div class="page-header">
    <div class="ph-main">
      <div class="ph-title">{{ title }}</div>
      <div v-if="subtitle" class="ph-sub">{{ subtitle }}</div>
    </div>
    <div class="ph-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<style scoped>
.page-header {
  display: flex; align-items: flex-start; justify-content: space-between;
  gap: 16px; margin-bottom: 14px;
}
.ph-title { font-size: 22px; font-weight: 650; line-height: 1.3; }
.ph-sub { margin-top: 3px; font-size: 12px; color: var(--text2); }
.ph-actions { display: flex; align-items: center; gap: 8px; flex: 0 0 auto; }
</style>
