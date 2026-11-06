<template>
  <div class="page">
    <PageHeader :title="$t('dashboard.title')" :subtitle="$t('dashboard.subtitle')">
      <template #actions>
        <a-button type="primary" @click="router.push('/practice')">{{ $t('dashboard.startPractice') }}</a-button>
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile
        :label="$t('dashboard.metricQuestions')"
        :value="overview?.questionCount ?? 0"
        :hint="$t('dashboard.metricQuestionsHint')"
      />
      <MetricTile
        :label="$t('dashboard.metricToday')"
        :value="overview?.todayAnswered ?? 0"
        :hint="$t('dashboard.metricTodayHint')"
      />
      <MetricTile :label="$t('dashboard.metricAccuracy')" :value="accuracyText" :hint="accuracyHint" />
      <MetricTile
        :label="$t('dashboard.metricWrong')"
        :value="overview?.wrongPending ?? 0"
        :hint="wrongHint"
      />
    </div>

    <div class="panels">
      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('dashboard.ongoing') }}</h3>
          <span class="muted">{{ $t('dashboard.ongoingDesc') }}</span>
        </div>
        <div class="panel-body" :class="{ empty: !hasOngoing }">
          <a-empty v-if="!hasOngoing" :description="$t('dashboard.noOngoing')">
            <a-button type="primary" ghost @click="router.push('/banks')">
              {{ $t('dashboard.goBanks') }}
            </a-button>
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
                  {{
                    $t('dashboard.examMeta', {
                      total: item.totalCount,
                      participants: item.participantCount,
                      deadline: fmtTime(item.closeAt),
                    })
                  }}
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
                  {{
                    $t('dashboard.practiceMeta', {
                      answered: item.answeredCount,
                      total: item.totalCount,
                      time: fmtTime(item.createdAt),
                    })
                  }}
                </div>
              </div>
              <span class="oi-arrow">›</span>
            </li>
          </ul>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('dashboard.service') }}</h3>
          <a-tag :color="online ? 'green' : 'red'">
            {{ online ? $t('dashboard.ok') : $t('dashboard.abnormal') }}
          </a-tag>
        </div>
        <div class="panel-body">
          <dl class="status-list">
            <div>
              <dt>{{ $t('dashboard.engineVersion') }}</dt>
              <dd>{{ appStore.health?.version ?? '—' }}</dd>
            </div>
            <div>
              <dt>{{ $t('dashboard.sqlite') }}</dt>
              <dd>{{ appStore.health?.sqlite ? $t('dashboard.connected') : $t('dashboard.disconnected') }}</dd>
            </div>
            <div><dt>{{ $t('dashboard.apiBase') }}</dt><dd class="mono">{{ apiBase }}</dd></div>
          </dl>
          <a-button size="small" :loading="refreshing" @click="refresh">
            {{ $t('dashboard.recheck') }}
          </a-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import { useAppStore } from '@/stores/app'
import { getApiBase } from '@/api/request'
import { overviewApi, type Overview } from '@/api/overview'

const { t } = useI18n()
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
  if (!data || data.answeredTotal === 0) return t('dashboard.accuracyEmpty')
  return t('dashboard.accuracyHint', {
    answered: data.answeredTotal,
    correct: data.correctTotal,
  })
})

const wrongHint = computed(() => {
  const data = overview.value
  if (data && data.wrongMastered > 0) return t('dashboard.wrongHintMastered', { n: data.wrongMastered })
  return t('dashboard.wrongHint')
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
