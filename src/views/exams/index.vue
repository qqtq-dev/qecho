<template>
  <div class="page">
    <PageHeader title="考试" subtitle="限时答题、外联链接与成绩汇总">
      <template #actions>
        <a-button @click="openShare()">分享设置</a-button>
        <a-button type="primary" @click="openCreate">新建考试</a-button>
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile label="进行中" :value="runningCount" hint="当前可参加的考试" />
      <MetricTile label="参与人次" :value="participantCount" hint="含未交卷" />
      <MetricTile label="已交卷" :value="submittedCount" hint="全部考试合计" />
      <MetricTile label="平均得分" :value="avgScoreText" hint="按已交卷统计" />
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>考试列表</h3>
        <span class="muted">共 {{ exams.length }} 场</span>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && exams.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && exams.length === 0" description="暂无考试">
            <a-button type="primary" ghost @click="openCreate">新建一场考试</a-button>
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
                <div class="muted">{{ record.bankName || '题库已删除' }} · {{ record.totalCount }} 道题 · {{ Math.round(record.timeLimitSec / 60) }} 分钟</div>
                <div v-if="record.shuffleOptions" class="muted">选项乱序</div>
              </template>
              <template v-else-if="column.key === 'window'">
                <div>{{ formatTime(record.openAt) }}</div>
                <div class="muted">至 {{ formatTime(record.closeAt) }}</div>
              </template>
              <template v-else-if="column.key === 'status'">
                <a-tag :color="statusOf(record).color">{{ statusOf(record).text }}</a-tag>
              </template>
              <template v-else-if="column.key === 'progress'">
                <span>{{ record.submittedCount }}/{{ record.participantCount }}</span>
                <span class="muted"> 已交卷</span>
              </template>
              <template v-else-if="column.key === 'avgScore'">
                {{ record.submittedCount ? record.avgScore.toFixed(1) : '—' }}
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space>
                  <a-button type="link" size="small" @click="openShare(record)">链接</a-button>
                  <a-button type="link" size="small" @click="openResults(record)">成绩</a-button>
                  <a-popconfirm
                    v-if="record.status === 'published'"
                    title="关闭后参与者将无法再进入考试，确定吗？"
                    ok-text="关闭考试"
                    cancel-text="取消"
                    @confirm="closeExam(record)"
                  >
                    <a-button type="link" size="small" danger>关闭</a-button>
                  </a-popconfirm>
                  <a-popconfirm
                    title="删除后考试及参与记录将一并移除，且不可恢复，确定吗？"
                    ok-text="删除"
                    cancel-text="取消"
                    @confirm="removeExam(record)"
                  >
                    <a-button type="link" size="small" danger>删除</a-button>
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
      title="新建考试"
      :confirm-loading="creating"
      ok-text="创建并获取链接"
      cancel-text="取消"
      :width="560"
      :mask-closable="false"
      @ok="createExam"
    >
      <a-form layout="vertical">
        <a-form-item label="考试名称" required>
          <a-input v-model:value="form.title" :maxlength="80" placeholder="例如：10 月安全规程周测" />
        </a-form-item>
        <div class="form-row">
          <a-form-item label="选择题库" required class="grow">
            <a-select
              v-model:value="form.bankId"
              placeholder="请选择题库"
              @change="onBankChange"
            >
              <a-select-option v-for="b in banks" :key="b.id" :value="b.id">
                {{ b.name }}（{{ b.questionCount }} 道）
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="抽题数量" required>
            <a-input-number v-model:value="form.count" :min="1" :max="500" style="width: 120px" />
          </a-form-item>
        </div>
        <a-form-item v-if="banks.length === 0" class="no-bank">
          还没有题库，请先到
          <a @click="goBanks">题库管理</a>
          添加题目。
        </a-form-item>
        <div class="form-row">
          <a-form-item label="考试时长（分钟）" required>
            <a-input-number v-model:value="form.minutes" :min="1" :max="360" style="width: 140px" />
          </a-form-item>
          <a-form-item label="考试密码" required class="grow">
            <a-input v-model:value="form.password" :maxlength="32" placeholder="至少 4 位，参与者进入时填写" />
          </a-form-item>
        </div>
        <a-form-item label="开考时间段" required>
          <a-range-picker
            v-model:value="form.range"
            show-time
            format="YYYY-MM-DD HH:mm"
            :placeholder="['开始时间', '结束时间']"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item>
          <a-checkbox v-model:checked="form.shuffle">打乱题目顺序</a-checkbox>
          <a-checkbox v-model:checked="form.shuffleOptions" class="shuffle-option">打乱选项顺序</a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="shareOpen"
      title="考试链接"
      :footer="null"
      :width="620"
    >
      <a-spin :spinning="shareLoading">
        <div v-if="shareExam" class="share-exam">
          <div class="share-exam-title">{{ shareExam.title }}</div>
          <div class="muted">{{ shareExam.totalCount }} 道题 · {{ Math.round(shareExam.timeLimitSec / 60) }} 分钟 · {{ formatTime(shareExam.openAt) }} 至 {{ formatTime(shareExam.closeAt) }}</div>
        </div>

        <template v-if="share">
          <div class="share-label">局域网链接（同一 Wi-Fi / 局域网设备可打开）</div>
          <a-empty v-if="shareBases.length === 0" description="未检测到局域网地址，请检查网络连接" :image-style="{ height: '40px' }" />
          <div v-else class="link-list">
            <div v-for="base in shareBases" :key="base" class="link-row">
              <span class="link-text">{{ base }}{{ shareExam ? '/e/' + shareExam.token : '' }}</span>
              <a-button size="small" type="primary" ghost @click="copy(base + (shareExam ? '/e/' + shareExam.token : ''))">复制</a-button>
            </div>
          </div>
          <div class="notice">
            首次分享时 Windows 可能提示防火墙拦截，请点击下方按钮放行；未放行时其他设备打不开链接。
            <div style="margin-top: 8px">
              <a-button size="small" type="primary" ghost :loading="lanRequesting" @click="requestLan">
                允许局域网访问
              </a-button>
            </div>
          </div>

          <div class="share-label">公网地址（可选，配置隧道或反向代理后填写）</div>
          <div class="public-row">
            <a-input
              v-model:value="publicBaseInput"
              placeholder="例如 https://exam.example.com"
              allow-clear
            />
            <a-button :loading="shareSaving" @click="savePublicBase">保存</a-button>
          </div>
          <div class="muted" style="margin-top: 6px">
            填写后，考试链接将优先使用公网地址；参与者页面由本机引擎直接提供。
          </div>
        </template>
      </a-spin>
    </a-modal>

    <a-drawer
      v-model:open="resultsOpen"
      :title="resultsExam ? `${resultsExam.title} · 成绩` : '成绩'"
      :width="760"
    >
      <a-spin :spinning="resultsLoading">
        <a-empty v-if="!resultsLoading && attempts.length === 0" description="还没有人参加这场考试" />
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
                {{ record.status === 'submitted' ? '已交卷' : '答题中' }}
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
              共 {{ attempts.length }} 人参加 · 已交卷 {{ submittedTotal }} 人
            </template>
            <template v-else>暂无参与记录</template>
          </span>
          <a-space :size="8">
            <a-button
              :loading="exportingResults"
              :disabled="!attempts.length"
              @click="exportResults"
            >导出 Excel</a-button>
            <a-button @click="refreshResults">刷新</a-button>
          </a-space>
        </div>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import { banksApi, type Bank } from '@/api/banks'
import { isTauri } from '@/api/request'
import { examsApi, type AttemptSummary, type Exam, type ShareInfo } from '@/api/exams'

const router = useRouter()

const exams = ref<Exam[]>([])
const banks = ref<Bank[]>([])
const loading = ref(false)

const columns = [
  { title: '考试', key: 'title', dataIndex: 'title' },
  { title: '时间窗', key: 'window', width: 190 },
  { title: '状态', key: 'status', width: 90 },
  { title: '交卷情况', key: 'progress', width: 130 },
  { title: '平均分', key: 'avgScore', width: 90 },
  { title: '操作', key: 'actions', width: 220 },
]

const attemptColumns = [
  { title: '姓名', key: 'name', dataIndex: 'name', width: 120 },
  { title: '学号/工号', key: 'studentNo', dataIndex: 'studentNo', width: 130 },
  { title: '状态', key: 'status', width: 90 },
  { title: '得分', key: 'score', width: 80 },
  { title: '作答', key: 'progress', width: 80 },
  { title: '交卷时间', key: 'submittedAt', width: 160 },
]

const formatTime = (raw: string) => (raw ? dayjs(raw).format('MM-DD HH:mm') : '—')

function statusOf(exam: Exam) {
  if (exam.status === 'closed') return { text: '已关闭', color: 'default' }
  const now = dayjs()
  if (now.isBefore(dayjs(exam.openAt))) return { text: '未开始', color: 'orange' }
  if (now.isAfter(dayjs(exam.closeAt))) return { text: '已结束', color: 'default' }
  return { text: '进行中', color: 'green' }
}

const runningCount = computed(() => exams.value.filter((e) => statusOf(e).text === '进行中').length)
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
  if (!form.title.trim()) return message.warning('请填写考试名称')
  if (!form.bankId) return message.warning('请选择题库')
  if (!form.range) return message.warning('请选择开考时间段')
  if (form.password.trim().length < 4) return message.warning('考试密码至少 4 位')
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
    message.success('考试已创建')
    openShare(exam)
  } finally {
    creating.value = false
  }
}

async function closeExam(exam: Exam) {
  await examsApi.close(exam.id)
  message.success('考试已关闭')
  await load()
}

async function removeExam(exam: Exam) {
  await examsApi.remove(exam.id)
  message.success('考试已删除')
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
    message.info('请在桌面端窗口中点击此按钮')
    return
  }
  lanRequesting.value = true
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('request_lan_access')
    message.success('已请求管理员授权，请在系统弹窗中点击“是”')
  } catch (err) {
    message.error(`授权失败：${(err as Error).message}`)
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
    message.success('公网地址已保存')
  } finally {
    shareSaving.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    message.success('链接已复制')
  } catch {
    const node = document.createElement('textarea')
    node.value = text
    node.style.position = 'fixed'
    node.style.opacity = '0'
    document.body.appendChild(node)
    node.select()
    document.execCommand('copy')
    document.body.removeChild(node)
    message.success('链接已复制')
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
  const name = (resultsExam.value?.title ?? '考试').replace(/[\\/:*?"<>|]/g, '_').trim() || '考试'
  return `${name}-成绩-${dayjs().format('YYYYMMDD')}.xlsx`
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
      message.success(`已导出 ${res.count} 条成绩到 ${res.path}`)
      return
    }
    const blob = await examsApi.exportAttemptsFile(resultsExam.value.id)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success('已开始下载')
  } catch (err) {
    message.error(`导出失败：${(err as Error).message || '请稍后重试'}`)
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
.panel-body :deep(.ant-spin-nested-loading) { min-height: 40px; }
</style>
