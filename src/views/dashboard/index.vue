<template>
  <div class="page">
    <PageHeader title="概览" subtitle="本机题库、练习与考试的整体情况">
      <template #actions>
        <a-button type="primary" @click="router.push('/practice')">开始练习</a-button>
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile label="题目总数" :value="overview?.questionCount ?? 0" hint="全部题库的有效题目" />
      <MetricTile label="今日答题" :value="overview?.todayAnswered ?? 0" hint="练习与考试作答合计" />
      <MetricTile label="总正确率" :value="accuracyText" :hint="accuracyHint" />
      <MetricTile label="待复习错题" :value="overview?.wrongPending ?? 0" :hint="wrongHint" />
    </div>

    <div class="panels">
      <section class="panel">
        <div class="panel-head">
          <h3>继续学习</h3>
          <span class="muted">进行中的练习与考试</span>
        </div>
        <div class="panel-body" :class="{ empty: !hasOngoing }">
          <a-empty v-if="!hasOngoing" description="暂无进行中的练习或考试">
            <a-button type="primary" ghost @click="router.push('/banks')">先去题库看看</a-button>
          </a-empty>
          <ul v-else class="ongoing">
            <li
              v-for="item in overview?.exams ?? []"
              :key="`exam-${item.id}`"
              @click="router.push('/exams')"
            >
              <span class="dot exam" />
              <div class="oi-main">
                <div class="oi-title">{{ item.title }}</div>
                <div class="oi-meta">
                  考试 · {{ item.totalCount }} 题 · {{ item.participantCount }} 人答题中 ·
                  {{ fmtTime(item.closeAt) }} 截止
                </div>
              </div>
              <span class="oi-arrow">›</span>
            </li>
            <li
              v-for="item in overview?.practices ?? []"
              :key="`practice-${item.id}`"
              @click="router.push('/practice')"
            >
              <span class="dot practice" />
              <div class="oi-main">
                <div class="oi-title">{{ item.bankName }}</div>
                <div class="oi-meta">
                  练习 · 已答 {{ item.answeredCount }} / {{ item.totalCount }} ·
                  {{ fmtTime(item.createdAt) }}
                </div>
              </div>
              <span class="oi-arrow">›</span>
            </li>
          </ul>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>服务状态</h3>
          <a-tag :color="online ? 'green' : 'red'">{{ online ? '正常' : '异常' }}</a-tag>
        </div>
        <div class="panel-body">
          <dl class="status-list">
            <div><dt>引擎版本</dt><dd>{{ appStore.health?.version ?? '—' }}</dd></div>
            <div><dt>SQLite</dt><dd>{{ appStore.health?.sqlite ? '已连接' : '未连接' }}</dd></div>
            <div><dt>接口地址</dt><dd class="mono">{{ apiBase }}</dd></div>
          </dl>
          <a-button size="small" :loading="refreshing" @click="refresh">重新检测</a-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import { useAppStore } from '@/stores/app'
import { getApiBase } from '@/api/request'
import { overviewApi, type Overview } from '@/api/overview'

const router = useRouter()
const appStore = useAppStore()
const refreshing = ref(false)
const online = computed(() => appStore.health?.status === 'ok')
const apiBase = computed(() => getApiBase())
const overview = ref<Overview | null>(null)

const accuracyText = computed(() => {
  const data = overview.value
  if (!data || data.answeredTotal === 0) return '—'
  return `${Math.round((data.correctTotal / data.answeredTotal) * 100)}%`
})

const accuracyHint = computed(() => {
  const data = overview.value
  if (!data || data.answeredTotal === 0) return '完成练习或考试后生成'
  return `已答 ${data.answeredTotal} 题，答对 ${data.correctTotal} 题`
})

const wrongHint = computed(() => {
  const data = overview.value
  if (data && data.wrongMastered > 0) return `已掌握 ${data.wrongMastered} 题`
  return '连续答对 2 次后移出'
})

const hasOngoing = computed(() => {
  const data = overview.value
  return !!data && data.exams.length + data.practices.length > 0
})

function fmtTime(value: string) {
  return dayjs(value).format('MM-DD HH:mm')
}

async function loadOverview() {
  overview.value = await overviewApi.get()
}

async function refresh() {
  refreshing.value = true
  try {
    await Promise.all([appStore.refresh(), loadOverview()])
  } finally {
    refreshing.value = false
  }
}

onMounted(loadOverview)
</script>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.panels {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(280px, 1fr);
  gap: 12px;
  margin-top: 12px;
}
.panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  min-height: 260px;
  display: flex;
  flex-direction: column;
}
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--border);
}
.panel-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.muted { font-size: 11px; color: var(--text2); }
.panel-body { padding: 16px; flex: 1; }
.panel-body.empty { display: flex; align-items: center; justify-content: center; }
.ongoing { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
.ongoing li {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 14px; border: 1px solid var(--border); border-radius: 8px;
  cursor: pointer; transition: background .15s, border-color .15s;
}
.ongoing li:hover { background: var(--bg); border-color: var(--primary); }
.dot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--primary); }
.dot.practice { background: #7a9c8b; }
.oi-main { flex: 1; min-width: 0; }
.oi-title { font-size: 13.5px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.oi-meta { margin-top: 3px; font-size: 11.5px; color: var(--text2); }
.oi-arrow { flex: none; color: var(--text2); font-size: 16px; }
.status-list { margin: 0 0 14px; display: grid; gap: 10px; }
.status-list > div { display: flex; justify-content: space-between; gap: 12px; }
.status-list dt { color: var(--text2); font-size: 12px; }
.status-list dd { margin: 0; font-size: 12px; text-align: right; }
.mono { font-family: Consolas, 'Courier New', monospace; word-break: break-all; }
@media (max-width: 1100px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .panels { grid-template-columns: 1fr; }
}
</style>
