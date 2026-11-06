<template>
  <div class="page">
    <PageHeader
      :title="$t('wrong.title')"
      :subtitle="$t('wrong.subtitle', { n: threshold })"
    >
      <template #actions>
        <a-button :disabled="wrongStore.stats.pending === 0" @click="startReview">
          {{ $t('wrong.startReview') }}
        </a-button>
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile
        :label="$t('wrong.metricPending')"
        :value="wrongStore.stats.pending"
        :hint="$t('wrong.metricPendingHint', { n: threshold })"
      />
      <MetricTile
        :label="$t('wrong.metricMastered')"
        :value="wrongStore.stats.mastered"
        :hint="$t('wrong.metricMasteredHint', { n: threshold })"
      />
      <MetricTile
        :label="$t('wrong.metricTotal')"
        :value="wrongStore.stats.total"
        :hint="$t('wrong.metricTotalHint')"
      />
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('wrong.list') }}</h3>
        <a-radio-group v-model:value="status" size="small" button-style="solid" @change="reload">
          <a-radio-button value="pending">{{ $t('wrong.tabPending') }}</a-radio-button>
          <a-radio-button value="mastered">{{ $t('wrong.tabMastered') }}</a-radio-button>
          <a-radio-button value="all">{{ $t('wrong.tabAll') }}</a-radio-button>
        </a-radio-group>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && items.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && items.length === 0" :description="$t('wrong.empty')">
            <a-button type="primary" ghost @click="router.push('/practice')">
              {{ $t('wrong.goPractice') }}
            </a-button>
          </a-empty>
          <template v-else>
            <a-table
              :data-source="items"
              :columns="columns"
              :pagination="false"
              row-key="questionId"
              size="middle"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'stem'">
                  <div class="stem-row">
                    <QuestionImage v-if="record.stemImage" :name="record.stemImage" :thumb="true" :size="40" />
                    <div>
                      <div class="stem">{{ record.stem }}</div>
                      <div class="muted">{{ record.bankName }}</div>
                    </div>
                  </div>
                </template>
                <template v-else-if="column.key === 'type'">
                  <a-tag :color="record.type === 'multiple' ? 'purple' : 'blue'">
                    {{ record.type === 'multiple' ? $t('wrong.typeMultiple') : $t('wrong.typeSingle') }}
                  </a-tag>
                </template>
                <template v-else-if="column.key === 'wrongCount'">
                  <a-tag color="red">{{ $t('wrong.wrongTimes', { n: record.wrongCount }) }}</a-tag>
                </template>
                <template v-else-if="column.key === 'streak'">
                  <span>{{ record.correctStreak }} / {{ threshold }}</span>
                </template>
                <template v-else-if="column.key === 'status'">
                  <a-tag :color="record.mastered ? 'green' : 'orange'">
                    {{ record.mastered ? $t('wrong.statusMastered') : $t('wrong.statusPending') }}
                  </a-tag>
                </template>
                <template v-else-if="column.key === 'lastWrongAt'">
                  <span class="muted">{{ formatTime(record.lastWrongAt) }}</span>
                </template>
                <template v-else-if="column.key === 'actions'">
                  <a-space>
                    <a-button type="link" size="small" @click="startSingle(record)">
                      {{ $t('wrong.practiceThis') }}
                    </a-button>
                    <a-popconfirm
                      :title="$t('wrong.removeConfirm')"
                      :ok-text="$t('wrong.remove')"
                      :cancel-text="$t('common.cancel')"
                      @confirm="remove(record)"
                    >
                      <a-button type="link" size="small" danger>{{ $t('wrong.remove') }}</a-button>
                    </a-popconfirm>
                  </a-space>
                </template>
              </template>
              <template #expandedRowRender="{ record }">
                <div class="detail">
                  <QuestionImage
                    v-if="record.stemImage"
                    :name="record.stemImage"
                    class="detail-stem-img"
                  />
                  <div v-for="opt in record.options" :key="opt.key" class="detail-opt">
                    <span class="opt-key" :class="{ right: record.answer.includes(opt.key) }">{{ opt.key }}</span>
                    <QuestionImage v-if="opt.image" :name="opt.image" :thumb="true" :size="48" />
                    <span>{{ opt.content }}</span>
                  </div>
                  <div class="muted" style="margin-top: 8px">
                    {{ $t('wrong.correctAnswer', { answer: record.answer.join('') || '—' }) }}
                  </div>
                  <div v-if="record.analysis" class="analysis">{{ record.analysis }}</div>
                </div>
              </template>
            </a-table>
            <div class="pager">
              <a-pagination
                v-model:current="page"
                :total="total"
                :page-size="size"
                :show-size-changer="false"
                size="small"
                @change="load"
              />
            </div>
          </template>
        </a-spin>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import QuestionImage from '@/components/QuestionImage.vue'
import { wrongApi, type WrongItem } from '@/api/wrong'
import { practiceApi } from '@/api/practice'
import { settingsApi } from '@/api/settings'
import { useWrongStore } from '@/stores/wrong'

const { t } = useI18n()
const router = useRouter()
const wrongStore = useWrongStore()
const threshold = ref(2)

const items = ref<WrongItem[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const loading = ref(false)
const status = ref<'pending' | 'mastered' | 'all'>('pending')

const columns = computed(() => [
  { title: t('wrong.colQuestion'), key: 'stem', dataIndex: 'stem' },
  { title: t('wrong.colType'), key: 'type', width: 76 },
  { title: t('wrong.colWrongCount'), key: 'wrongCount', width: 96 },
  { title: t('wrong.colStreak'), key: 'streak', width: 96 },
  { title: t('wrong.colStatus'), key: 'status', width: 96 },
  { title: t('wrong.colLastWrong'), key: 'lastWrongAt', width: 150 },
  { title: t('wrong.colActions'), key: 'actions', width: 140 },
])

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('YYYY-MM-DD HH:mm') : '—'
}

async function load() {
  loading.value = true
  try {
    const res = await wrongApi.list({ page: page.value, size, status: status.value })
    items.value = res.items
    total.value = res.total
    wrongStore.syncStats(res.stats)
  } finally {
    loading.value = false
  }
}

async function reload() {
  page.value = 1
  await load()
}

async function remove(item: WrongItem) {
  await wrongApi.remove(item.questionId)
  if (items.value.length === 1 && page.value > 1) page.value -= 1
  await load()
}

function startReview() {
  router.push('/practice?source=wrong')
}

const startingId = ref(0)

async function startSingle(item: WrongItem) {
  if (startingId.value) return
  startingId.value = item.questionId
  try {
    const res = await practiceApi.start({
      questionIds: [item.questionId],
      count: 0,
      order: 'fixed',
      source: 'wrong',
    })
    router.push({ path: '/practice', query: { session: String(res.session.id) } })
  } catch {
    /* 失败原因由请求层提示 */
  } finally {
    startingId.value = 0
  }
}

// 练习页面作答后会递增 revision，这里即时重新拉取，保证跨页面同步。
watch(
  () => wrongStore.revision,
  () => {
    if (!loading.value) load()
  },
)

async function loadSettings() {
  try {
    const s = await settingsApi.get()
    threshold.value = s.wrongMasterThreshold
  } catch {
    /* 读取失败时按默认阈值展示 */
  }
}

function onVisible() {
  if (document.visibilityState === 'visible') load()
}

onMounted(() => {
  load()
  loadSettings()
  document.addEventListener('visibilitychange', onVisible)
})
onBeforeUnmount(() => document.removeEventListener('visibilitychange', onVisible))
</script>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.stem {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  max-width: 620px;
}
.stem-row { display: flex; align-items: flex-start; gap: 10px; }
.detail { padding: 4px 8px 8px; }
.detail-stem-img { display: block; margin-bottom: 8px; }
.detail-opt { display: flex; align-items: center; gap: 8px; padding: 3px 0; }
.opt-key {
  flex: none; width: 22px; height: 22px; border-radius: 50%;
  background: var(--chip-bg); color: var(--chip-text); font-size: 12px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
}
.opt-key.right { background: #24945f; color: #fff; }
.analysis {
  margin-top: 8px; padding: 8px 10px; background: var(--surface-3);
  border-radius: 6px; font-size: 13px; color: var(--text); white-space: pre-wrap;
}
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
.panel-body :deep(.ant-spin-nested-loading) { min-height: 40px; }
@media (max-width: 900px) {
  .metrics { grid-template-columns: 1fr; }
}
</style>
