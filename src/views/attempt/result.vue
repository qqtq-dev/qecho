<template>
  <div class="attempt">
    <header class="attempt-bar" data-tauri-drag-region>
      <button class="back" @click="exit">{{ $t('attempt.back') }}</button>
      <span class="title">{{ $t('attempt.resultTitle') }}</span>
      <WindowControls />
    </header>
    <main class="attempt-body">
      <a-result
        status="info"
        :title="$t('attempt.noScoreTitle')"
        :sub-title="$t('attempt.noScoreDesc')"
      >
        <template #extra>
          <a-button type="primary" @click="exit">{{ $t('attempt.backOverview') }}</a-button>
        </template>
      </a-result>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import WindowControls from '@/components/WindowControls.vue'

const router = useRouter()
const exit = () => router.push('/dashboard')
</script>

<style scoped>
.attempt { height: 100vh; display: flex; flex-direction: column; background: var(--bg); }
.attempt-bar {
  height: 46px; flex: 0 0 46px;
  display: flex; align-items: center; gap: 12px;
  padding: 0 14px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
.back {
  border: none; background: transparent; color: var(--text);
  font-size: 13px; cursor: pointer; padding: 4px 6px; border-radius: 6px;
}
.back:hover { background: var(--hover); }
.title { flex: 1; font-size: 13px; font-weight: 600; }
.attempt-body { flex: 1; display: flex; align-items: center; justify-content: center; overflow: auto; }
</style>
