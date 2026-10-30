<template>
  <div class="page">
    <PageHeader title="练习" subtitle="按题库抽题，作答后立即判分并同步错题本">
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
            </div>
            <div class="head-right">
              <span v-if="remainingSec > 0" class="timer" :class="{ low: timeLow }">
                <Clock :size="13" />
                {{ remainingText }}
              </span>
              <a-button v-if="current" size="small" :type="isMarked ? 'primary' : 'default'" @click="toggleMark">
                <template #icon><Flag :size="12" :fill="isMarked ? 'currentColor' : 'none'" /></template>
                {{ isMarked ? '已标记' : '标记本题' }}
              </a-button>
            </div>
          </div>

          <template v-if="current">
            <div class="q-type">{{ current.type === 'multiple' ? '多选题' : '单选题' }}</div>
            <div class="q-stem">{{ current.stem }}</div>
            <img v-if="current.stemImage" :src="current.stemImage" class="q-img" alt="题目图片" />

            <div class="options">
              <div
                v-for="opt in current.options"
                :key="opt.key"
                class="opt"
                :class="optionClass(opt.key)"
                @click="toggleOption(opt.key)"
              >
                <span class="opt-key">{{ opt.key }}</span>
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
              <a-space>
                <a-button v-if="currentAnswer" @click="retryQuestion">重新作答</a-button>
                <a-popconfirm
                  :title="`确定交卷？已答 ${answeredCount} / ${session.totalCount} 题`"
                  ok-text="交卷"
                  cancel-text="继续作答"
                  @confirm="finish"
                >
                  <a-button type="primary">交卷</a-button>
                </a-popconfirm>
              </a-space>
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

    <template v-else-if="finished">
      <section class="panel result">
        <a-result
          status="success"
          :title="`本组练习完成：答对 ${correctCount} / ${sessionTotal} 题`"
          :sub-title="`错题本当前待复习 ${wrongStore.stats.pending} 道`"
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
                v-model:value="form.bankId"
                placeholder="请选择题库"
                :loading="loadingBanks"
                :options="bankOptions"
                @change="onBankChange"
              />
            </a-form-item>
            <a-form-item label="顺序">
              <a-radio-group v-model:value="form.order">
                <a-radio-button value="random">随机</a-radio-button>
                <a-radio-button value="fixed">按题序</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item label="题型">
              <a-radio-group v-model:value="form.type">
                <a-radio-button value="">全部</a-radio-button>
                <a-radio-button value="single">单选</a-radio-button>
                <a-radio-button value="multiple">多选</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item label="题量">
              <a-input-number v-model:value="form.count" :min="1" :max="100" style="width: 100%" />
            </a-form-item>
            <a-form-item label="限时（分钟）">
              <a-input-number v-model:value="form.timeLimit" :min="0" :max="300" style="width: 100%" />
              <div class="muted" style="margin-top: 4px">0 表示不限时，时间到自动交卷</div>
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
          <span class="muted">每组练习自动记录</span>
        </div>
        <div class="panel-body" :class="{ 'empty-state': sessions.length === 0 }">
          <a-empty v-if="sessions.length === 0" description="还没有练习记录">
            <a-button type="primary" ghost @click="router.push('/banks')">先去题库看看</a-button>
          </a-empty>
          <a-list v-else :data-source="sessions" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="recent-row">
                  <div>
                    <div class="recent-title">
                      {{ item.source === 'wrong' ? '错题复习' : item.bankName }}
                      <a-tag v-if="item.source === 'wrong'" color="red" style="margin-left: 6px">错题</a-tag>
                    </div>
                    <div class="muted">
                      {{ formatTime(item.createdAt) }} · {{ item.totalCount }} 道 ·
                      {{ item.order === 'random' ? '随机' : '按题序' }}
                    </div>
                  </div>
                  <div class="recent-score">
                    <span>{{ item.answeredCount }}/{{ item.totalCount }}</span>
                    <span class="muted">答对 {{ item.correctCount }}</span>
                  </div>
                  <a-popconfirm
                    title="删除这条练习记录？"
                    ok-text="删除"
                    cancel-text="取消"
                    @confirm="removeSession(item.id)"
                  >
                    <a-button type="link" size="small" danger class="recent-del">删除</a-button>
                  </a-popconfirm>
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
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Clock, Flag } from 'lucide-vue-next'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import { banksApi, type Bank } from '@/api/banks'
import {
  practiceApi,
  type PracticeAnswerResult,
  type PracticeItem,
  type PracticeSession,
} from '@/api/practice'
import { useWrongStore } from '@/stores/wrong'

const route = useRoute()
const router = useRouter()
const wrongStore = useWrongStore()

const banks = ref<Bank[]>([])
const loadingBanks = ref(false)
const starting = ref(false)
const sessions = ref<PracticeSession[]>([])

const form = reactive({
  bankId: undefined as number | undefined,
  order: 'random' as 'random' | 'fixed',
  type: '' as '' | 'single' | 'multiple',
  count: 20,
  timeLimit: 0,
})

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
const sessionTotal = ref(0)
const markedIds = ref<number[]>([])

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

const remainingSec = ref(0)
let timer: number | undefined
const remainingText = computed(() => {
  const sec = Math.max(0, remainingSec.value)
  return `${String(Math.floor(sec / 60)).padStart(2, '0')}:${String(sec % 60).padStart(2, '0')}`
})
const timeLow = computed(() => remainingSec.value > 0 && remainingSec.value <= 60)

function stopTimer() {
  if (timer) window.clearInterval(timer)
  timer = undefined
}

function startTimer(minutes: number) {
  stopTimer()
  remainingSec.value = Math.max(0, Math.round(minutes * 60))
  if (remainingSec.value === 0) return
  timer = window.setInterval(() => {
    remainingSec.value -= 1
    if (remainingSec.value <= 0) {
      stopTimer()
      finish()
      message.warning('时间到，已自动交卷')
    }
  }, 1000)
}

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('MM-DD HH:mm') : '—'
}

async function loadBanks() {
  loadingBanks.value = true
  try {
    const res = await banksApi.list()
    banks.value = res.items
    const preferred = banks.value.find((b) => b.questionCount > 0)
    if (!form.bankId && preferred) form.bankId = preferred.id
    if (preferred) form.count = Math.min(20, preferred.questionCount)
  } finally {
    loadingBanks.value = false
  }
}

function onBankChange(id: number) {
  const bank = banks.value.find((b) => b.id === id)
  if (bank) form.count = Math.min(form.count, bank.questionCount) || 1
}

async function loadSessions() {
  const res = await practiceApi.sessions()
  sessions.value = res.items
}

async function removeSession(id: number) {
  await practiceApi.remove(id)
  sessions.value = sessions.value.filter((s) => s.id !== id)
  message.success('已删除练习记录')
}

async function start(source: 'bank' | 'wrong' = 'bank', bankIdOverride?: number) {
  if (starting.value) return
  starting.value = true
  try {
    const res = await practiceApi.start({
      bankId: source === 'wrong' ? bankIdOverride : form.bankId,
      count: form.count,
      order: form.order,
      source,
      questionType: form.type || undefined,
    })
    session.value = res.session
    sessionTotal.value = res.session.totalCount
    questions.value = res.questions
    index.value = 0
    finished.value = false
    for (const id of Object.keys(answers)) delete answers[Number(id)]
    chosen.value = []
    markedIds.value = []
    startTimer(form.timeLimit)
  } finally {
    starting.value = false
  }
}

async function startWrong() {
  const bankId = route.query.bankId ? Number(route.query.bankId) : undefined
  try {
    await start('wrong', bankId)
  } catch {
    message.info('错题本里还没有待复习的题目')
    router.replace('/wrong')
  }
}

function optionClass(key: string) {
  const cls: Record<string, boolean> = { selected: chosen.value.includes(key) }
  const answer = currentAnswer.value
  if (answer) {
    cls.correct = answer.result.answer.includes(key)
    cls.wrong = chosen.value.includes(key) && !answer.result.answer.includes(key)
  }
  return cls
}

function cellClass(i: number) {
  const q = questions.value[i]
  const answer = q ? answers[q.id] : undefined
  return {
    current: i === index.value,
    ok: !!answer?.result.isCorrect,
    bad: !!answer && !answer.result.isCorrect,
    marked: !!q && markedIds.value.includes(q.id),
  }
}

function toggleMark() {
  if (!current.value) return
  const id = current.value.id
  const pos = markedIds.value.indexOf(id)
  if (pos >= 0) markedIds.value.splice(pos, 1)
  else markedIds.value.push(id)
}

function toggleOption(key: string) {
  if (submitting.value || currentAnswer.value || !current.value) return
  if (current.value.type === 'multiple') {
    const idx = chosen.value.indexOf(key)
    if (idx >= 0) chosen.value.splice(idx, 1)
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
    const result = await practiceApi.answer(session.value.id, current.value.id, chosen.value)
    answers[current.value.id] = { chosen: [...chosen.value], result }
    wrongStore.setStats(result.stats)
    if (result.isCorrect) message.success('回答正确')
    else message.warning(`回答错误，正确答案：${result.answer.join('')}`)
  } finally {
    submitting.value = false
  }
}

function retryQuestion() {
  if (!current.value) return
  delete answers[current.value.id]
  chosen.value = []
}

function go(next: number) {
  if (!session.value) return
  if (next < 0 || next >= session.value.totalCount) return
  index.value = next
  const item = questions.value[next]
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
}

function finish() {
  stopTimer()
  finished.value = true
  session.value = null
  questions.value = []
  loadSessions()
}

function backToSettings() {
  finished.value = false
  loadSessions()
}

onMounted(async () => {
  await Promise.all([loadBanks(), loadSessions(), wrongStore.refresh()])
  if (route.query.source === 'wrong') {
    await startWrong()
  }
})

onUnmounted(stopTimer)
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
.head-right { display: flex; align-items: center; gap: 10px; }
.timer {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 13px; color: var(--text2); font-variant-numeric: tabular-nums;
}
.timer.low { color: #d4380d; font-weight: 600; }
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
.q-img { display: block; max-width: 100%; margin-top: 10px; border-radius: 8px; }
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
.result { margin-top: 0; }
.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; }
.recent-title { font-weight: 600; }
.recent-score { display: flex; flex-direction: column; align-items: flex-end; font-size: 13px; }
@media (max-width: 900px) {
  .grid { grid-template-columns: 1fr; }
  .quiz-layout { grid-template-columns: 1fr; grid-template-rows: auto; }
  .q-grid { max-height: 170px; }
}
</style>
