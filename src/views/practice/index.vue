<template>
  <div class="page">
    <PageHeader title="练习" subtitle="按题库、标签、题型与难度组卷，作答即时判分并自动保存进度">
      <template #actions>
        <a-button v-if="!session" type="primary" :loading="starting" @click="start()">开始练习</a-button>
      </template>
    </PageHeader>

    <template v-if="session">
      <div class="quiz-layout">
        <section class="panel quiz">
          <div class="quiz-head">
            <div>
              <a-tag v-if="session.source === 'wrong'" color="red">错题复习</a-tag>
              <a-tag v-else color="green">{{ session.bankName }}</a-tag>
              <span class="muted">第 {{ index + 1 }} / {{ session.totalCount }} 题</span>
              <span class="muted progress-hint">已答 {{ answeredCount }}</span>
            </div>
            <div class="head-right">
              <a-button v-if="current" size="small" :type="isMarked ? 'primary' : 'default'" @click="toggleMark">
                <template #icon><Flag :size="12" :fill="isMarked ? 'currentColor' : 'none'" /></template>
                {{ isMarked ? '已标记' : '标记本题' }}
              </a-button>
            </div>
          </div>

          <template v-if="current">
            <div class="q-type">{{ current.type === 'multiple' ? '多选题' : '单选题' }}</div>
            <div class="q-stem">{{ current.stem }}</div>
            <QuestionImage v-if="current.stemImage" :name="current.stemImage" class="q-img" />

            <div class="options">
              <div
                v-for="opt in current.options"
                :key="opt.key"
                class="opt"
                :class="optionClass(opt)"
                @click="toggleOption(opt.key)"
              >
                <span class="opt-key">{{ opt.label }}</span>
                <span v-if="opt.image" class="opt-thumb" @click.stop>
                  <QuestionImage :name="opt.image" :thumb="true" :size="72" />
                </span>
                <span class="opt-text">{{ opt.content }}</span>
              </div>
            </div>

            <div v-if="current.type === 'multiple' && !currentAnswer" class="multi-submit">
              <a-button
                type="primary"
                :loading="submitting"
                :disabled="chosen.length === 0"
                @click="submitAnswer"
              >
                确认答案
              </a-button>
            </div>

            <div v-if="currentAnswer" class="feedback" :class="currentAnswer.result.isCorrect ? 'ok' : 'bad'">
              <div class="feedback-line">
                <strong>{{ currentAnswer.result.isCorrect ? '回答正确' : '回答错误' }}</strong>
                <span class="muted">正确答案：{{ currentAnswer.result.answer.join('') || '—' }}</span>
              </div>
              <div v-if="!currentAnswer.result.isCorrect" class="feedback-line muted">
                已计入错题本（累计答错 {{ currentAnswer.result.wrongCount }} 次）
              </div>
              <div v-else-if="currentAnswer.result.correctStreak" class="feedback-line muted">
                连续答对 {{ currentAnswer.result.correctStreak }} 次{{ currentAnswer.result.mastered ? '，已移出错题本' : '' }}
              </div>
            </div>

            <div class="analysis-box">
              <div class="analysis-title">解析</div>
              <template v-if="currentAnswer">
                <div v-if="currentAnswer.result.analysis" class="analysis-text">{{ currentAnswer.result.analysis }}</div>
                <div v-else class="muted">本题暂无解析</div>
              </template>
              <div v-else class="muted">作答后显示解析</div>
            </div>

            <div class="quiz-actions">
              <a-space>
                <a-button :disabled="index === 0" @click="go(index - 1)">上一题</a-button>
                <a-button :disabled="index >= session.totalCount - 1" @click="go(index + 1)">下一题</a-button>
              </a-space>
              <a-popconfirm
                :title="`确定完成练习？已答 ${answeredCount} / ${session.totalCount} 题`"
                ok-text="完成"
                cancel-text="继续作答"
                @confirm="finish"
              >
                <a-button type="primary">完成练习</a-button>
              </a-popconfirm>
            </div>
            <div class="kbd-hint muted">
              快捷键：A–H / 1–8 选择选项 · Enter 确认或下一题 · ← → 切换题目
            </div>
          </template>
        </section>

        <aside class="panel answer-card">
          <div class="side-head">
            <h3>答题卡</h3>
            <span class="muted">{{ answeredCount }} / {{ session.totalCount }}</span>
          </div>
          <div class="side-legend">
            <span><i class="dot ok"></i>答对</span>
            <span><i class="dot bad"></i>答错</span>
            <span><i class="dot none"></i>未答</span>
            <span><i class="dot mark"></i>标记</span>
          </div>
          <div class="q-grid">
            <button
              v-for="(q, i) in questions"
              :key="q.id"
              type="button"
              class="q-cell"
              :class="cellClass(i)"
              @click="go(i)"
            >
              {{ i + 1 }}
            </button>
          </div>
          <div class="side-foot muted">答对 {{ correctCount }} 题</div>
        </aside>
      </div>
    </template>

    <template v-else-if="finished && summary">
      <section class="panel result">
        <a-result
          status="success"
          :title="`本组练习完成：答对 ${summary.correctCount} / ${summary.totalCount} 题`"
          :sub-title="`用时 ${formatDuration(summary.durationSec)} · 错题本当前待复习 ${wrongStore.stats.pending} 道`"
        >
          <template #extra>
            <a-space>
              <a-button type="primary" @click="start()">再来一组</a-button>
              <a-button @click="backToSettings">调整设置</a-button>
              <a-button @click="router.push('/wrong')">查看错题本</a-button>
            </a-space>
          </template>
        </a-result>
      </section>
    </template>

    <div v-else class="grid">
      <section class="panel">
        <div class="panel-head">
          <h3>练习设置</h3>
        </div>
        <div class="panel-body">
          <a-form layout="vertical">
            <a-form-item label="题库" required>
              <a-select
                v-model:value="form.bankIds"
                mode="multiple"
                placeholder="请选择题库（可多选）"
                :loading="loadingBanks"
                :options="bankOptions"
                :max-tag-count="2"
                allow-clear
              />
            </a-form-item>
            <a-form-item label="题目顺序">
              <a-radio-group v-model:value="form.order">
                <a-radio-button value="random">随机</a-radio-button>
                <a-radio-button value="fixed">按题序</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item label="标签">
              <a-select
                v-model:value="form.tags"
                mode="multiple"
                placeholder="全部标签（可多选，匹配任一标签）"
                :loading="loadingTags"
                :options="tagOptions"
                :max-tag-count="3"
                allow-clear
                :disabled="form.bankIds.length === 0"
              />
            </a-form-item>
            <a-form-item label="题型">
              <a-radio-group v-model:value="form.type">
                <a-radio-button value="">全部</a-radio-button>
                <a-radio-button value="single">单选</a-radio-button>
                <a-radio-button value="multiple">多选</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item label="难度">
              <a-select v-model:value="form.difficulty" :options="difficultyOptions" style="width: 100%" />
            </a-form-item>
            <a-form-item label="题量">
              <a-select v-model:value="form.count" :options="countOptions" style="width: 100%" />
            </a-form-item>
            <a-form-item label="选项乱序">
              <a-switch v-model:checked="form.shuffleOptions" />
              <span class="muted switch-hint">同一组练习内顺序固定，续答不重新洗牌</span>
            </a-form-item>
            <a-empty v-if="!loadingBanks && banks.length === 0" description="还没有题库">
              <a-button type="primary" ghost @click="router.push('/banks')">去创建题库</a-button>
            </a-empty>
          </a-form>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>最近练习</h3>
          <span class="muted">进度自动保存，可随时继续</span>
        </div>
        <div class="panel-body" :class="{ 'empty-state': sessions.length === 0 }">
          <a-empty v-if="sessions.length === 0" description="还没有练习记录">
            <a-button type="primary" ghost @click="router.push('/banks')">先去题库看看</a-button>
          </a-empty>
          <a-list v-else :data-source="sessions" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="recent-row">
                  <div class="recent-main">
                    <div class="recent-title">
                      {{ item.source === 'wrong' ? '错题复习' : item.bankName }}
                      <a-tag :color="item.status === 'ongoing' ? 'processing' : 'default'" class="recent-tag">
                        {{ item.status === 'ongoing' ? '进行中' : '已完成' }}
                      </a-tag>
                    </div>
                    <div class="muted">
                      {{ formatTime(item.updatedAt || item.createdAt) }} · {{ item.totalCount }} 道 ·
                      {{ item.order === 'random' ? '随机' : '按题序' }}
                      <template v-if="item.shuffleOptions"> · 选项乱序</template>
                    </div>
                  </div>
                  <div class="recent-right">
                    <div class="recent-score">
                      <span>{{ item.answeredCount }}/{{ item.totalCount }}</span>
                      <span class="muted">
                        答对 {{ item.correctCount }}<template v-if="item.durationSec"> · {{ formatDuration(item.durationSec) }}</template>
                      </span>
                    </div>
                    <a-space :size="0">
                      <a-button
                        v-if="item.status === 'ongoing'"
                        type="link"
                        size="small"
                        class="recent-continue"
                        @click="resume(item.id)"
                      >
                        继续
                      </a-button>
                      <a-popconfirm
                        title="删除这条练习记录？"
                        ok-text="删除"
                        cancel-text="取消"
                        @confirm="removeSession(item.id)"
                      >
                        <a-button type="link" size="small" danger>删除</a-button>
                      </a-popconfirm>
                    </a-space>
                  </div>
                </div>
              </a-list-item>
            </template>
          </a-list>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Flag } from 'lucide-vue-next'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import QuestionImage from '@/components/QuestionImage.vue'
import { banksApi, type Bank, type Tag } from '@/api/banks'
import { settingsApi } from '@/api/settings'
import {
  practiceApi,
  type PracticeAnswerResult,
  type PracticeDetail,
  type PracticeItem,
  type PracticeOption,
  type PracticeSession,
  type PracticeSummary,
} from '@/api/practice'
import { useWrongStore } from '@/stores/wrong'

const route = useRoute()
const router = useRouter()
const wrongStore = useWrongStore()

const banks = ref<Bank[]>([])
const availableTags = ref<Tag[]>([])
const loadingBanks = ref(false)
const loadingTags = ref(false)
const starting = ref(false)
const sessions = ref<PracticeSession[]>([])
let tagLoadSeq = 0

const form = reactive({
  bankIds: [] as number[],
  order: 'random' as 'random' | 'fixed',
  type: '' as '' | 'single' | 'multiple',
  difficulty: 0,
  tags: [] as string[],
  count: 20,
  shuffleOptions: false,
})

const tagOptions = computed(() =>
  availableTags.value.map((t) => ({
    label: `${t.name}（${t.questionCount}）`,
    value: t.name,
  })),
)

async function loadFormTags() {
  const ids = [...form.bankIds]
  const seq = ++tagLoadSeq
  if (ids.length === 0) {
    availableTags.value = []
    form.tags = []
    loadingTags.value = false
    return
  }
  loadingTags.value = true
  try {
    const results = await Promise.all(ids.map((id) => banksApi.bankTags(id)))
    if (seq !== tagLoadSeq) return
    const merged = new Map<string, Tag>()
    for (const res of results) {
      for (const tag of res.items) {
        const prev = merged.get(tag.name)
        if (prev) prev.questionCount += tag.questionCount
        else merged.set(tag.name, { ...tag })
      }
    }
    availableTags.value = [...merged.values()].sort(
      (a, b) => b.questionCount - a.questionCount || a.name.localeCompare(b.name, 'zh'),
    )
    const names = new Set(availableTags.value.map((t) => t.name))
    form.tags = form.tags.filter((t) => names.has(t))
  } catch {
    if (seq === tagLoadSeq) availableTags.value = []
  } finally {
    if (seq === tagLoadSeq) loadingTags.value = false
  }
}

watch(() => form.bankIds.join(','), loadFormTags)

const countOptions = computed(() => {
  const options = [
    { label: '10 题', value: 10 },
    { label: '20 题', value: 20 },
    { label: '50 题', value: 50 },
    { label: '100 题', value: 100 },
    { label: '全部', value: 0 },
  ]
  if (!options.some((o) => o.value === form.count)) {
    options.splice(options.length - 1, 0, { label: `${form.count} 题`, value: form.count })
  }
  return options
})

const difficultyOptions = [
  { label: '全部难度', value: 0 },
  { label: '难度 1', value: 1 },
  { label: '难度 2', value: 2 },
  { label: '难度 3', value: 3 },
  { label: '难度 4', value: 4 },
  { label: '难度 5', value: 5 },
]

const bankOptions = computed(() =>
  banks.value.map((b) => ({
    label: `${b.name}（${b.questionCount} 道）`,
    value: b.id,
    disabled: b.questionCount === 0,
  })),
)

const session = ref<PracticeSession | null>(null)
const questions = ref<PracticeItem[]>([])
const index = ref(0)
const chosen = ref<string[]>([])
const submitting = ref(false)
const finished = ref(false)
const summary = ref<PracticeSummary | null>(null)
const markedIds = ref<number[]>([])
let questionStartedAt = Date.now()

interface Answered {
  chosen: string[]
  result: PracticeAnswerResult
}

const answers = reactive<Record<number, Answered>>({})

const current = computed(() => questions.value[index.value] ?? null)
const currentAnswer = computed(() => (current.value ? answers[current.value.id] : undefined))
const isMarked = computed(() => !!current.value && markedIds.value.includes(current.value.id))
const answeredCount = computed(() => Object.keys(answers).length)
const correctCount = computed(() => Object.values(answers).filter((a) => a.result.isCorrect).length)

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('MM-DD HH:mm') : '—'
}

function formatDuration(seconds: number) {
  const sec = Math.max(0, Math.round(seconds || 0))
  const minutes = Math.floor(sec / 60)
  if (minutes >= 60) return `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分`
  return `${minutes} 分 ${String(sec % 60).padStart(2, '0')} 秒`
}

function clearAnswers() {
  for (const id of Object.keys(answers)) delete answers[Number(id)]
}

function applySession(detail: PracticeDetail) {
  clearAnswers()
  session.value = detail.session
  questions.value = detail.questions
  for (const a of detail.answers) {
    answers[a.questionId] = {
      chosen: [...a.chosen],
      result: {
        isCorrect: a.isCorrect,
        answer: a.answer,
        analysis: a.analysis,
        wrongCount: a.wrongCount,
        correctStreak: a.correctStreak,
        mastered: a.mastered,
        timeSpentMs: a.timeSpentMs,
        durationSec: detail.session.durationSec,
        stats: detail.stats,
      },
    }
  }
  markedIds.value = [...detail.marked]
  const maxIndex = Math.max(questions.value.length - 1, 0)
  index.value = Math.min(Math.max(detail.lastSeq - 1, 0), maxIndex)
  finished.value = false
  summary.value = null
  const item = current.value
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  questionStartedAt = Date.now()
  wrongStore.setStats(detail.stats)
}

async function loadBanks() {
  loadingBanks.value = true
  try {
    const res = await banksApi.list()
    banks.value = res.items
    if (form.bankIds.length === 0) {
      const preferred = banks.value.find((b) => b.questionCount > 0)
      if (preferred) form.bankIds = [preferred.id]
    }
  } finally {
    loadingBanks.value = false
  }
}

async function loadSessions() {
  const res = await practiceApi.sessions()
  sessions.value = res.items
}

async function loadSettings() {
  try {
    const s = await settingsApi.get()
    form.count = s.practiceCount
    form.order = s.practiceShuffleQuestions ? 'random' : 'fixed'
    form.shuffleOptions = s.practiceShuffleOptions
  } catch {
    /* 读取失败时沿用默认值 */
  }
}

async function removeSession(id: number) {
  await practiceApi.remove(id)
  sessions.value = sessions.value.filter((s) => s.id !== id)
  message.success('已删除练习记录')
}

async function start(source: 'bank' | 'wrong' = 'bank', bankIdsOverride?: number[]) {
  if (starting.value) return
  starting.value = true
  try {
    const res = await practiceApi.start({
      bankIds: source === 'wrong' ? bankIdsOverride : form.bankIds,
      count: source === 'wrong' ? 0 : form.count,
      order: form.order,
      source,
      questionType: form.type || undefined,
      difficulty: form.difficulty || undefined,
      tags: form.tags.length ? form.tags : undefined,
      shuffleOptions: form.shuffleOptions,
    })
    applySession({
      session: res.session,
      questions: res.questions,
      answers: [],
      marked: [],
      lastSeq: 1,
      stats: wrongStore.stats,
    })
    if (res.truncated) message.info('候选题目超过 5000 道，已截取前 5000 道')
  } finally {
    starting.value = false
  }
}

async function startWrong() {
  const bankId = route.query.bankId ? Number(route.query.bankId) : undefined
  try {
    await start('wrong', bankId ? [bankId] : [])
  } catch {
    message.info('错题本里还没有待复习的题目')
    router.replace('/wrong')
  }
}

async function resume(id: number) {
  try {
    const detail = await practiceApi.detail(id)
    if (detail.session.status !== 'ongoing') {
      message.info('该练习已完成')
      await loadSessions()
      return
    }
    applySession(detail)
  } catch {
    message.error('无法继续该练习，可能已被删除')
    await loadSessions()
  }
}

function savePosition() {
  if (!session.value) return
  practiceApi.position(session.value.id, index.value + 1, [...markedIds.value]).catch(() => {})
}

function optionClass(opt: PracticeOption) {
  const cls: Record<string, boolean> = { selected: chosen.value.includes(opt.key) }
  const answer = currentAnswer.value
  if (answer) {
    const correct = answer.result.answer.includes(opt.label)
    cls.correct = correct
    cls.wrong = chosen.value.includes(opt.key) && !correct
  }
  return cls
}

function cellClass(i: number) {
  const q = questions.value[i]
  const answer = q ? answers[q.id] : undefined
  const marked = !!q && markedIds.value.includes(q.id)
  return {
    current: i === index.value,
    ok: !!answer?.result.isCorrect,
    bad: !!answer && !answer.result.isCorrect,
    marked,
  }
}

function toggleMark() {
  if (!current.value) return
  const id = current.value.id
  const pos = markedIds.value.indexOf(id)
  if (pos >= 0) markedIds.value.splice(pos, 1)
  else markedIds.value.push(id)
  savePosition()
}

function toggleOption(key: string) {
  if (submitting.value || currentAnswer.value || !current.value) return
  if (current.value.type === 'multiple') {
    const pos = chosen.value.indexOf(key)
    if (pos >= 0) chosen.value.splice(pos, 1)
    else chosen.value.push(key)
    chosen.value.sort()
  } else {
    chosen.value = [key]
    void submitAnswer()
  }
}

async function submitAnswer() {
  if (!session.value || !current.value || chosen.value.length === 0) return
  submitting.value = true
  try {
    const spent = Math.min(Math.max(Date.now() - questionStartedAt, 0), 30 * 60 * 1000)
    const result = await practiceApi.answer(session.value.id, current.value.id, chosen.value, spent)
    answers[current.value.id] = { chosen: [...chosen.value], result }
    wrongStore.setStats(result.stats)
    session.value.answeredCount = answeredCount.value
    session.value.correctCount = correctCount.value
    session.value.durationSec = result.durationSec
    if (result.isCorrect) message.success('回答正确')
    else message.warning(`回答错误，正确答案：${result.answer.join('')}`)
  } finally {
    submitting.value = false
  }
}

function isTypingTarget(target: EventTarget | null) {
  const el = target as HTMLElement | null
  if (!el) return false
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable
}

function hasOpenLayer() {
  return !!document.querySelector('.ant-modal-wrap, .ant-popover:not(.ant-popover-hidden)')
}

function onKeydown(e: KeyboardEvent) {
  if (!session.value || !current.value || submitting.value) return
  if (e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target) || hasOpenLayer()) return
  const q = current.value
  const upper = e.key.length === 1 ? e.key.toUpperCase() : e.key
  const letterIdx =
    e.key.length === 1 && upper >= 'A' && upper <= 'H' ? upper.charCodeAt(0) - 65 : -1
  const numberIdx = e.key.length === 1 && e.key >= '1' && e.key <= '8' ? Number(e.key) - 1 : -1
  const optIdx = letterIdx >= 0 ? letterIdx : numberIdx
  if (optIdx >= 0) {
    const opt = q.options[optIdx]
    if (!opt) return
    e.preventDefault()
    toggleOption(opt.key)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    if (currentAnswer.value) go(index.value + 1)
    else if (q.type === 'multiple' && chosen.value.length > 0) void submitAnswer()
    return
  }
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    go(index.value - 1)
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    go(index.value + 1)
  }
}

function go(next: number) {
  if (!session.value) return
  if (next < 0 || next >= session.value.totalCount) return
  index.value = next
  const item = questions.value[next]
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  questionStartedAt = Date.now()
  savePosition()
}

async function finish() {
  if (!session.value) return
  try {
    summary.value = await practiceApi.submit(session.value.id)
    finished.value = true
    session.value = null
    questions.value = []
    chosen.value = []
    await loadSessions()
  } catch {
    message.error('完成练习失败，请重试')
  }
}

function backToSettings() {
  finished.value = false
  summary.value = null
  loadSessions()
}

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  await Promise.all([loadBanks(), loadSessions(), loadSettings(), wrongStore.refresh()])
  const sessionId = Number(route.query.session ?? 0)
  if (sessionId > 0) {
    await resume(sessionId)
    router.replace({ path: '/practice' })
  } else if (route.query.source === 'wrong') {
    await startWrong()
  }
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  savePosition()
})
</script>

<style scoped>
.page { min-height: 100%; display: flex; flex-direction: column; }
.grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(0, 1.6fr);
  gap: 12px;
}
.panel-body.empty-state { min-height: 300px; }
.quiz-layout {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 200px;
  grid-template-rows: minmax(0, 1fr);
  gap: 12px;
}
.quiz { display: flex; flex-direction: column; padding: 18px 20px 20px; }
.quiz-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.quiz-head .muted { margin-left: 8px; font-size: 13px; }
.progress-hint { opacity: .8; }
.head-right { display: flex; align-items: center; gap: 10px; }
.answer-card { display: flex; flex-direction: column; padding: 14px 12px 12px; min-height: 0; }
.side-head { display: flex; align-items: baseline; justify-content: space-between; }
.side-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.side-legend { display: flex; flex-wrap: wrap; gap: 4px 10px; margin: 10px 0; font-size: 11.5px; color: var(--text2); }
.side-legend span { display: inline-flex; align-items: center; gap: 4px; }
.dot { width: 8px; height: 8px; border-radius: 2px; border: 1px solid var(--border); background: var(--surface); }
.dot.ok { background: #e8f5ee; border-color: #24945f; }
.dot.bad { background: #fdece8; border-color: #d4380d; }
.dot.mark { background: #fa8c16; border-color: #fa8c16; }
.q-grid {
  flex: 1; min-height: 0; overflow: auto;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(26px, 1fr));
  align-content: start; gap: 6px;
}
.q-cell {
  position: relative;
  height: 26px; padding: 0; border: 1px solid var(--border); border-radius: 4px;
  background: var(--surface); color: var(--text2);
  font-size: 11.5px; line-height: 1; cursor: pointer;
  transition: border-color .12s, background .12s, color .12s;
}
.q-cell.marked::after {
  content: ''; position: absolute; top: 2px; right: 2px;
  width: 5px; height: 5px; border-radius: 50%; background: #fa8c16;
}
.q-cell:hover { border-color: #24945f; color: #24945f; }
.q-cell.ok { background: #e8f5ee; border-color: #cfe8da; color: #1b7a4d; }
.q-cell.bad { background: #fdece8; border-color: #f5d6cc; color: #b8340f; }
.q-cell.current { border-color: #24945f; box-shadow: 0 0 0 1px #24945f inset; color: #1b7a4d; font-weight: 600; }
.q-cell.bad.current { border-color: #d4380d; box-shadow: 0 0 0 1px #d4380d inset; color: #b8340f; }
.side-foot { margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border); text-align: right; }
.q-type { margin-top: 18px; font-size: 12.5px; color: #6b7570; }
.q-stem { margin-top: 6px; font-size: 16px; line-height: 1.6; white-space: pre-wrap; }
.q-img { display: block; margin-top: 10px; }
.opt-thumb { flex: none; }
.options { margin-top: 14px; display: flex; flex-direction: column; gap: 8px; }
.multi-submit { display: flex; justify-content: flex-end; margin-top: 10px; }
.opt {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 11px 12px; border: 1px solid #e2e6e4; border-radius: 8px;
  background: #fff; cursor: pointer; transition: border-color .12s, background .12s;
}
.opt:hover { background: #f7faf8; }
.opt.selected { border-color: #24945f; background: #e8f5ee; }
.opt.correct { border-color: #24945f; background: #e8f5ee; }
.opt.wrong { border-color: #d4380d; background: #fdece8; }
.opt-key {
  flex: none; width: 24px; height: 24px; border-radius: 50%;
  border: 1.5px solid #c6cfca; color: #6b7570;
  font-size: 12.5px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
}
.opt.selected .opt-key, .opt.correct .opt-key { background: #24945f; border-color: #24945f; color: #fff; }
.opt.wrong .opt-key { background: #d4380d; border-color: #d4380d; color: #fff; }
.opt-text { flex: 1; white-space: pre-wrap; }
.feedback { margin-top: 14px; padding: 12px; border-radius: 8px; font-size: 13.5px; }
.feedback.ok { background: #eef7f2; border: 1px solid #cfe8da; }
.feedback.bad { background: #fdf2ef; border: 1px solid #f5d6cc; }
.feedback-line { display: flex; gap: 12px; align-items: baseline; flex-wrap: wrap; }
.analysis-box {
  flex: 1; min-height: 96px; margin-top: 14px;
  padding: 12px 14px; border: 1px dashed var(--border); border-radius: 8px;
  background: var(--bg); font-size: 13.5px; line-height: 1.7;
}
.analysis-title { margin-bottom: 6px; font-size: 13px; font-weight: 600; }
.analysis-text { color: var(--text); white-space: pre-wrap; }
.quiz-actions { display: flex; justify-content: space-between; margin-top: auto; padding-top: 18px; flex-wrap: wrap; gap: 10px; }
.kbd-hint { margin-top: 10px; font-size: 12px; text-align: right; }
.result { margin-top: 0; }
.switch-hint { margin-left: 10px; font-size: 12.5px; }
.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; }
.recent-main { min-width: 0; }
.recent-title { font-weight: 600; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.recent-tag { margin-left: 2px; }
.recent-right { display: flex; align-items: center; gap: 10px; flex: none; }
.recent-score { display: flex; flex-direction: column; align-items: flex-end; font-size: 13px; }
.recent-continue { padding-inline: 4px; }
@media (max-width: 900px) {
  .grid { grid-template-columns: 1fr; }
  .quiz-layout { grid-template-columns: 1fr; grid-template-rows: auto; }
  .q-grid { max-height: 170px; }
}
</style>
