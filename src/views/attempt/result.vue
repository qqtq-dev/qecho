<template>
  <div class="attempt">
    <header class="attempt-bar" data-tauri-drag-region>
      <button class="back" @click="exit">← 返回</button>
      <span class="title">答题结果</span>
      <WindowControls />
    </header>
    <main class="attempt-body">
      <a-result
        status="info"
        title="暂无成绩"
        sub-title="成绩页需要一次已提交的练习或考试，答题流程将在下一阶段接入。"
      >
        <template #extra>
          <a-button type="primary" @click="exit">返回概览</a-button>
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
