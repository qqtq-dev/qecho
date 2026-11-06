<template>
  <div class="page">
    <PageHeader :title="$t('exams.title')" :subtitle="$t('exams.subtitle')">
      <template #actions>
        <a-button @click="openShare()">{{ $t('exams.shareSettings') }}</a-button>
        <a-button type="primary" @click="openCreate">{{ $t('exams.create') }}</a-button>
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile :label="$t('exams.metricRunning')" :value="runningCount" :hint="$t('exams.metricRunningHint')" />
      <MetricTile :label="$t('exams.metricParticipants')" :value="participantCount" :hint="$t('exams.metricParticipantsHint')" />
      <MetricTile :label="$t('exams.metricSubmitted')" :value="submittedCount" :hint="$t('exams.metricSubmittedHint')" />
      <MetricTile :label="$t('exams.metricAvgScore')" :value="avgScoreText" :hint="$t('exams.metricAvgScoreHint')" />
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('exams.list') }}</h3>
        <span class="muted">{{ $t('exams.count', { n: exams.length }) }}</span>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && exams.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && exams.length === 0" :description="$t('exams.empty')">
            <a-button type="primary" ghost @click="openCreate">{{ $t('exams.createFirst') }}</a-button>
          </a-empty>
          <a-table
            v-else
            :data-source="exams"
            :columns="columns"
            :pagination="false"
            row-key="id"
            size="middle"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'title'">
                <div class="exam-title">{{ record.title }}</div>
                <div class="muted">
                  {{
                    $t('exams.meta', {
                      bank: record.bankName || $t('exams.bankDeleted'),
                      total: record.totalCount,
                      minutes: Math.round(record.timeLimitSec / 60),
                    })
                  }}
                </div>
                <div v-if="record.shuffleOptions" class="muted">{{ $t('exams.shuffleOptions') }}</div>
              </template>
              <template v-else-if="column.key === 'window'">
                <div>{{ formatTime(record.openAt) }}</div>
                <div class="muted">{{ $t('exams.until', { time: formatTime(record.closeAt) }) }}</div>
              </template>
              <template v-else-if="column.key === 'status'">
                <a-tag :color="statusOf(record).color">{{ $t(statusOf(record).key) }}</a-tag>
              </template>
              <template v-else-if="column.key === 'progress'">
                <span>{{ record.submittedCount }}/{{ record.participantCount }}</span>
                <span class="muted submitted-label">{{ $t('exams.statusSubmitted') }}</span>
              </template>
              <template v-else-if="column.key === 'avgScore'">
                {{ record.submittedCount ? record.avgScore.toFixed(1) : '—' }}
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space>
                  <a-button type="link" size="small" @click="openShare(record)">{{ $t('exams.link') }}</a-button>
                  <a-button type="link" size="small" @click="openResults(record)">{{ $t('exams.results') }}</a-button>
                  <a-popconfirm
                    v-if="record.status === 'published'"
                    :title="$t('exams.closeConfirm')"
                    :ok-text="$t('exams.closeOk')"
                    :cancel-text="$t('common.cancel')"
                    @confirm="closeExam(record)"
                  >
                    <a-button type="link" size="small" danger>{{ $t('exams.close') }}</a-button>
                  </a-popconfirm>
                  <a-popconfirm
                    :title="$t('exams.removeConfirm')"
                    :ok-text="$t('exams.remove')"
                    :cancel-text="$t('common.cancel')"
                    @confirm="removeExam(record)"
                  >
                    <a-button type="link" size="small" danger>{{ $t('exams.remove') }}</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-spin>
      </div>
    </section>

    <a-modal
      v-model:open="createOpen"
      :title="$t('exams.createTitle')"
      :confirm-loading="creating"
      :ok-text="$t('exams.createOk')"
      :cancel-text="$t('common.cancel')"
      :width="560"
      :mask-closable="false"
      @ok="createExam"
    >
      <a-form layout="vertical">
        <a-form-item :label="$t('exams.examName')" required>
          <a-input v-model:value="form.title" :maxlength="80" :placeholder="$t('exams.examNamePlaceholder')" />
        </a-form-item>
        <div class="form-row">
          <a-form-item :label="$t('exams.chooseBank')" required class="grow">
            <a-select
              v-model:value="form.bankId"
              :placeholder="$t('exams.chooseBankPlaceholder')"
              @change="onBankChange"
            >
              <a-select-option v-for="b in banks" :key="b.id" :value="b.id">
                {{ $t('exams.bankOption', { name: b.name, n: b.questionCount }) }}
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item :label="$t('exams.questionCount')" required>
            <a-input-number v-model:value="form.count" :min="1" :max="500" style="width: 120px" />
          </a-form-item>
        </div>
        <a-form-item v-if="banks.length === 0" class="no-bank">
          {{ $t('exams.noBankBefore') }}
          <a @click="goBanks">{{ $t('exams.noBankLink') }}</a>
          {{ $t('exams.noBankAfter') }}
        </a-form-item>
        <div class="form-row">
          <a-form-item :label="$t('exams.duration')" required>
            <a-input-number v-model:value="form.minutes" :min="1" :max="360" style="width: 140px" />
          </a-form-item>
          <a-form-item :label="$t('exams.password')" required class="grow">
            <a-input v-model:value="form.password" :maxlength="32" :placeholder="$t('exams.passwordPlaceholder')" />
          </a-form-item>
        </div>
        <a-form-item :label="$t('exams.windowLabel')" required>
          <a-range-picker
            v-model:value="form.range"
            show-time
            format="YYYY-MM-DD HH:mm"
            :placeholder="[$t('exams.startTime'), $t('exams.endTime')]"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item>
          <a-checkbox v-model:checked="form.shuffle">{{ $t('exams.shuffleQuestions') }}</a-checkbox>
          <a-checkbox v-model:checked="form.shuffleOptions" class="shuffle-option">
            {{ $t('exams.shuffleOptionsCheck') }}
          </a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="shareOpen"
      :title="$t('exams.shareTitle')"
      :footer="null"
      :width="620"
    >
      <a-spin :spinning="shareLoading">
        <div v-if="shareExam" class="share-exam">
          <div class="share-exam-title">{{ shareExam.title }}</div>
          <div class="muted">
            {{
              $t('exams.shareMeta', {
                total: shareExam.totalCount,
                minutes: Math.round(shareExam.timeLimitSec / 60),
                open: formatTime(shareExam.openAt),
                close: formatTime(shareExam.closeAt),
              })
            }}
          </div>
        </div>

        <template v-if="share">
          <div class="share-label">{{ $t('exams.lanLabel') }}</div>
          <a-empty
            v-if="shareBases.length === 0"
            :description="$t('exams.lanEmpty')"
            :image-style="{ height: '40px' }"
          />
          <div v-else class="link-list">
            <div v-for="base in shareBases" :key="base" class="link-row">
              <span class="link-text">{{ base }}{{ shareExam ? '/e/' + shareExam.token : '' }}</span>
              <a-button size="small" type="primary" ghost @click="copy(base + (shareExam ? '/e/' + shareExam.token : ''))">
                {{ $t('exams.copy') }}
              </a-button>
            </div>
          </div>
          <div class="notice">
            {{ $t('exams.firewallNotice') }}
            <div style="margin-top: 8px">
              <a-button size="small" type="primary" ghost :loading="lanRequesting" @click="requestLan">
                {{ $t('exams.allowLan') }}
              </a-button>
            </div>
          </div>

          <div class="share-label">{{ $t('exams.publicLabel') }}</div>
          <div class="public-row">
            <a-input
              v-model:value="publicBaseInput"
              :placeholder="$t('exams.publicPlaceholder')"
              allow-clear
            />
            <a-button :loading="shareSaving" @click="savePublicBase">{{ $t('common.save') }}</a-button>
          </div>
          <div class="muted" style="margin-top: 6px">
            {{ $t('exams.publicHint') }}
          </div>
        </template>
      </a-spin>
    </a-modal>

    <a-drawer
      v-model:open="resultsOpen"
      :title="
        resultsExam
          ? $t('exams.resultsTitleNamed', { name: resultsExam.title })
          : $t('exams.resultsTitle')
      "
      :width="760"
    >
      <a-spin :spinning="resultsLoading">
        <a-empty v-if="!resultsLoading && attempts.length === 0" :description="$t('exams.noParticipants')" />
        <a-table
          v-else
          :data-source="attempts"
          :columns="attemptColumns"
          :pagination="false"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'score'">
              <span v-if="record.score !== null" class="score">{{ record.score }}</span>
              <span v-else class="muted">—</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="record.status === 'submitted' ? 'green' : 'orange'">
                {{ record.status === 'submitted' ? $t('exams.statusSubmitted') : $t('exams.statusAnswering') }}
              </a-tag>
            </template>
            <template v-else-if="column.key === 'progress'">
              {{ record.answeredCount }}/{{ record.totalCount }}
            </template>
            <template v-else-if="column.key === 'submittedAt'">
              <span class="muted">{{ record.submittedAt ? formatTime(record.submittedAt) : '—' }}</span>
            </template>
          </template>
        </a-table>
      </a-spin>

      <!-- 抽屉页脚：左汇总、右操作，固定在底部不随表格滚动 -->
      <template #footer>
        <div class="result-foot">
          <span class="muted">
            <template v-if="attempts.length">
              {{ $t('exams.footSummary', { n: attempts.length, submitted: submittedTotal }) }}
            </template>
            <template v-else>{{ $t('exams.footEmpty') }}</template>
          </span>
          <a-space :size="8">
            <a-button
              :loading="exportingResults"
              :disabled="!attempts.length"
              @click="exportResults"
            >{{ $t('exams.exportExcel') }}</a-button>
            <a-button @click="refreshResults">{{ $t('common.refresh') }}</a-button>
          </a-space>
        </div>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import { banksApi, type Bank } from '@/api/banks'
import { isTauri } from '@/api/request'
import { examsApi, type AttemptSummary, type Exam, type ShareInfo } from '@/api/exams'

const { t } = useI18n()
const router = useRouter()

const exams = ref<Exam[]>([])
const banks = ref<Bank[]>([])
const loading = ref(false)

const columns = computed(() => [
  { title: t('exams.colTitle'), key: 'title', dataIndex: 'title' },
  { title: t('exams.colWindow'), key: 'window', width: 190 },
  { title: t('exams.colStatus'), key: 'status', width: 90 },
  { title: t('exams.colProgress'), key: 'progress', width: 130 },
  { title: t('exams.colAvgScore'), key: 'avgScore', width: 90 },
  { title: t('exams.colActions'), key: 'actions', width: 220 },
])

const attemptColumns = computed(() => [
  { title: t('exams.colName'), key: 'name', dataIndex: 'name', width: 120 },
  { title: t('exams.colStudentNo'), key: 'studentNo', dataIndex: 'studentNo', width: 130 },
  { title: t('exams.colStatus'), key: 'status', width: 90 },
  { title: t('exams.colScore'), key: 'score', width: 80 },
  { title: t('exams.colProgressRatio'), key: 'progress', width: 80 },
  { title: t('exams.colSubmittedAt'), key: 'submittedAt', width: 160 },
])

const formatTime = (raw: string) => (raw ? dayjs(raw).format('MM-DD HH:mm') : '—')

function statusOf(exam: Exam) {
  if (exam.status === 'closed') return { key: 'exams.statusClosed', color: 'default' }
  const now = dayjs()
  if (now.isBefore(dayjs(exam.openAt))) return { key: 'exams.statusNotStarted', color: 'orange' }
  if (now.isAfter(dayjs(exam.closeAt))) return { key: 'exams.statusEnded', color: 'default' }
  return { key: 'exams.statusRunning', color: 'green' }
}

const runningCount = computed(
  () => exams.value.filter((e) => statusOf(e).key === 'exams.statusRunning').length,
)
const participantCount = computed(() => exams.value.reduce((sum, e) => sum + e.participantCount, 0))
const submittedCount = computed(() => exams.value.reduce((sum, e) => sum + e.submittedCount, 0))
const avgScoreText = computed(() => {
  const submitted = exams.value.filter((e) => e.submittedCount > 0)
  if (!submitted.length) return '—'
  const totalScore = submitted.reduce((sum, e) => sum + e.avgScore * e.submittedCount, 0)
  const totalCount = submitted.reduce((sum, e) => sum + e.submittedCount, 0)
  return (totalScore / totalCount).toFixed(1)
})

async function load() {
  loading.value = true
  try {
    const [examRes, bankRes] = await Promise.all([examsApi.list(), banksApi.list()])
    exams.value = examRes.items
    banks.value = bankRes.items
  } finally {
    loading.value = false
  }
}

const createOpen = ref(false)
const creating = ref(false)
const form = reactive({
  title: '',
  bankId: undefined as number | undefined,
  count: 20,
  minutes: 30,
  password: '',
  shuffle: true,
  shuffleOptions: true,
  range: null as [Dayjs, Dayjs] | null,
})

function openCreate() {
  form.title = ''
  form.bankId = banks.value[0]?.id
  form.count = Math.min(banks.value[0]?.questionCount || 20, 50) || 20
  form.minutes = 30
  form.password = ''
  form.shuffle = true
  form.shuffleOptions = true
  form.range = [dayjs().add(10, 'minute').second(0), dayjs().add(70, 'minute').second(0)]
  createOpen.value = true
}

function onBankChange(id: number) {
  const bank = banks.value.find((b) => b.id === id)
  if (bank && bank.questionCount > 0) {
    form.count = Math.min(bank.questionCount, 50)
  }
}

function goBanks() {
  createOpen.value = false
  router.push('/banks')
}

async function createExam() {
  if (!form.title.trim()) return message.warning(t('exams.msg.nameRequired'))
  if (!form.bankId) return message.warning(t('exams.msg.bankRequired'))
  if (!form.range) return message.warning(t('exams.msg.rangeRequired'))
  if (form.password.trim().length < 4) return message.warning(t('exams.msg.passwordTooShort'))
  creating.value = true
  try {
    const exam = await examsApi.create({
      title: form.title.trim(),
      bankId: form.bankId,
      count: form.count,
      timeLimitSec: form.minutes * 60,
      openAt: form.range[0].toISOString(),
      closeAt: form.range[1].toISOString(),
      password: form.password.trim(),
      shuffleQuestions: form.shuffle,
      shuffleOptions: form.shuffleOptions,
    })
    createOpen.value = false
    await load()
    message.success(t('exams.msg.created'))
    openShare(exam)
  } finally {
    creating.value = false
  }
}

async function closeExam(exam: Exam) {
  await examsApi.close(exam.id)
  message.success(t('exams.msg.closed'))
  await load()
}

async function removeExam(exam: Exam) {
  await examsApi.remove(exam.id)
  message.success(t('exams.msg.deleted'))
  await load()
}

const shareOpen = ref(false)
const shareLoading = ref(false)
const shareSaving = ref(false)
const share = ref<ShareInfo | null>(null)
const shareExam = ref<Exam | null>(null)
const publicBaseInput = ref('')

const lanRequesting = ref(false)

async function requestLan() {
  const w = window as unknown as { __TAURI_INTERNALS__?: unknown }
  if (!w.__TAURI_INTERNALS__) {
    message.info(t('exams.msg.desktopOnly'))
    return
  }
  lanRequesting.value = true
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('request_lan_access')
    message.success(t('exams.msg.lanRequested'))
  } catch (err) {
    message.error(t('exams.msg.lanFailed', { msg: (err as Error).message }))
  } finally {
    lanRequesting.value = false
  }
}

const shareBases = computed(() => {
  if (!share.value) return []
  if (share.value.publicBaseUrl) return [share.value.publicBaseUrl]
  return share.value.lanAddresses.map((ip) => `http://${ip}:${share.value?.port}`)
})

async function openShare(exam?: Exam) {
  shareExam.value = exam ?? null
  shareOpen.value = true
  shareLoading.value = true
  try {
    share.value = await examsApi.share()
    publicBaseInput.value = share.value.publicBaseUrl
  } finally {
    shareLoading.value = false
  }
}

async function savePublicBase() {
  shareSaving.value = true
  try {
    const res = await examsApi.updateShare(publicBaseInput.value.trim())
    publicBaseInput.value = res.publicBaseUrl
    if (share.value) share.value = { ...share.value, publicBaseUrl: res.publicBaseUrl }
    message.success(t('exams.msg.publicSaved'))
  } finally {
    shareSaving.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    message.success(t('exams.msg.copied'))
  } catch {
    const node = document.createElement('textarea')
    node.value = text
    node.style.position = 'fixed'
    node.style.opacity = '0'
    document.body.appendChild(node)
    node.select()
    document.execCommand('copy')
    document.body.removeChild(node)
    message.success(t('exams.msg.copied'))
  }
}

const resultsOpen = ref(false)
const resultsLoading = ref(false)
const resultsExam = ref<Exam | null>(null)
const attempts = ref<AttemptSummary[]>([])

async function openResults(exam: Exam) {
  resultsExam.value = exam
  resultsOpen.value = true
  await refreshResults()
}

async function refreshResults() {
  if (!resultsExam.value) return
  resultsLoading.value = true
  try {
    const res = await examsApi.attempts(resultsExam.value.id)
    attempts.value = res.items
  } finally {
    resultsLoading.value = false
  }
}

const exportingResults = ref(false)
const submittedTotal = computed(
  () => attempts.value.filter((a) => a.status === 'submitted').length,
)

function resultsFileName() {
  const fallback = t('exams.fileExamFallback')
  const name = (resultsExam.value?.title || fallback).replace(/[\\/:*?"<>|]/g, '_').trim() || fallback
  return `${name}-${t('exams.fileResultsLabel')}-${dayjs().format('YYYYMMDD')}.xlsx`
}

async function exportResults() {
  if (!resultsExam.value || exportingResults.value) return
  exportingResults.value = true
  try {
    const fileName = resultsFileName()
    if (isTauri()) {
      const { invoke } = await import('@tauri-apps/api/core')
      const path = await invoke<string | null>('pick_export_path', { fileName })
      if (!path) return
      const res = await examsApi.exportAttemptsTo(resultsExam.value.id, path)
      message.success(t('exams.msg.exported', { count: res.count, path: res.path }))
      return
    }
    const blob = await examsApi.exportAttemptsFile(resultsExam.value.id)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success(t('exams.msg.downloadStarted'))
  } catch (err) {
    message.error(t('exams.msg.exportFailed', { msg: (err as Error).message || t('exams.msg.retryLater') }))
  } finally {
    exportingResults.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
@media (max-width: 1000px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
.exam-title { font-weight: 600; }
.form-row { display: flex; gap: 16px; align-items: flex-start; }
.form-row .grow { flex: 1; }
.shuffle-option { margin-left: 16px; }
.no-bank { color: var(--warn-text); font-size: 13px; margin-top: -8px; }
.share-exam { padding: 10px 12px; background: var(--ok-soft); border: 1px solid var(--ok-border); border-radius: 8px; }
.share-exam-title { font-weight: 600; }
.share-label { margin: 16px 0 8px; font-size: 13px; font-weight: 600; color: var(--text); }
.link-list { display: flex; flex-direction: column; gap: 8px; }
.link-row { display: flex; align-items: center; gap: 10px; }
.link-text {
  flex: 1;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12.5px;
  word-break: break-all;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 7px 10px;
}
.notice {
  margin-top: 10px;
  padding: 8px 12px;
  background: var(--warn-soft);
  color: var(--warn-text);
  border-radius: 6px;
  font-size: 12.5px;
}
.public-row { display: flex; gap: 8px; }
/* 抽屉页脚：左汇总、右按钮，一行放下 */
.result-foot {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; text-align: left; font-size: 12.5px;
}
.score { font-weight: 700; color: #24945f; }
/* 已交卷标签紧跟比例，留一个间距；用 margin 而不是空格，避免被行内空白折叠 */
.submitted-label { margin-left: 4px; }
.panel-body :deep(.ant-spin-nested-loading) { min-height: 40px; }
</style>
