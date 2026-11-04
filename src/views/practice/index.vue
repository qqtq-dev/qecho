<template>
  <div class="page" :class="{ 'page-quiz': !!session }">
    <PageHeader title="练习" subtitle="按题库、标签、题型与难度组卷，作答即时判分并自动保存进度">
      <template #actions>
        <a-button v-if="!session" type="primary" :loading="starting" @click="start()">开始练习</a-button>
      </template>
    </PageHeader>

    <template v-if="session">
      <div class="quiz-layout" :class="{ 'ai-open': aiOpen }">
        <section class="panel quiz">
          <div class="quiz-head">
            <div>
              <a-tag v-if="session.source === 'wrong'" color="red">错题复习</a-tag>
              <a-tag v-else color="green">{{ session.bankName }}</a-tag>
              <span class="muted">第 {{ index + 1 }} / {{ session.totalCount }} 题</span>
              <span class="muted progress-hint">已答 {{ answeredCount }}</span>
            </div>
            <div class="head-right">
              <a-tooltip v-if="current" :title="isMarked ? '取消标记' : '标记本题'" placement="bottomRight">
                <a-button
                  size="small"
                  class="mark-btn"
                  :class="{ marked: isMarked }"
                  :aria-label="isMarked ? '取消标记' : '标记本题'"
                  :aria-pressed="isMarked"
                  @click="toggleMark"
                >
                  <template #icon><Flag :size="12" :fill="isMarked ? 'currentColor' : 'none'" /></template>
                </a-button>
              </a-tooltip>
            </div>
          </div>

          <template v-if="current">
            <div ref="bodyRef" class="quiz-body">
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

              <div class="analysis-box">
                <div class="analysis-head">
                  <div class="analysis-title">解析</div>
                  <!-- 未配置（配置读取完成且 ready=false）时在按钮原位显示引导；null=还在读取，不闪 -->
                  <a-button
                    v-if="aiReady === false"
                    size="small"
                    class="ai-setup-hint"
                    @click="router.push('/settings')"
                  >
                    配置 AI 答疑 →
                  </a-button>
                  <!-- 面板开着时按钮常驻，保证任何时刻都能收起；没开面板才要求先作答 -->
                  <a-button
                    v-else-if="aiReady && (aiOpen || currentAnswer)"
                    size="small"
                    class="ai-ask-btn"
                    @click="aiOpen = !aiOpen"
                  >
                    <template #icon><Sparkles :size="12" /></template>
                    {{ aiOpen ? '收起 AI' : '问 AI' }}
                  </a-button>
                </div>
                <template v-if="currentAnswer">
                  <div v-if="currentAnswer.result.analysis" class="analysis-text">{{ currentAnswer.result.analysis }}</div>
                  <div v-else class="muted">本题暂无解析</div>
                </template>
                <div v-else class="muted">作答后显示解析</div>
              </div>
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

        <aside class="panel side-col">
          <!-- 两个面板常驻挂载（v-show），AI 会话不因收起而丢；流式回答收起后继续走 -->
          <div v-show="!aiOpen" class="answer-card">
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
          </div>
          <AiChatPanel
            v-show="aiOpen"
            class="ai-host"
            :open="aiOpen"
            :question-id="current?.id ?? 0"
            :stem="current?.stem ?? ''"
            :options="aiOptions"
            :type="current?.type ?? 'single'"
          />
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
            <a-form-item label="题序范围">
              <div class="range-presets">
                <a-button
                  v-for="p in rangePresets"
                  :key="p.value"
                  size="small"
                  :type="isPresetActive(p.value) ? 'primary' : 'default'"
                  :disabled="p.value > 0 && candidateTotal > 0 && p.value > candidateTotal"
                  @click="applyRangePreset(p.value)"
                >
                  {{ p.label }}
                </a-button>
              </div>
              <div class="range-row">
                <a-input-number
                  v-model:value="form.startSeq"
                  :min="1"
                  :max="candidateTotal || 99999"
                  :precision="0"
                  placeholder="起始"
                  class="range-input"
                />
                <span class="range-sep">—</span>
                <a-input-number
                  v-model:value="form.endSeq"
                  :min="form.startSeq || 1"
                  :max="candidateTotal || 99999"
                  :precision="0"
                  placeholder="末题"
                  class="range-input"
                />
                <span class="muted range-unit">题</span>
              </div>
              <div class="range-hint muted">
                <template v-if="loadingCandidates">正在统计题数…</template>
                <template v-else-if="candidateTotal > 0">
                  当前筛选共 {{ candidateTotal }} 道 · {{ rangeSummary }}
                </template>
                <template v-else>请先选择题库</template>
              </div>
              <div v-if="filterActive" class="range-hint muted">
                题号按上方筛选结果编号，改筛选会重新排
              </div>
              <div v-if="nextRangeStart" class="range-hint">
                <a-button type="link" size="small" class="range-next" @click="applyNextRange">
                  上次练到第 {{ nextRangeStart - 1 }} 题，从第 {{ nextRangeStart }} 题接着练
                </a-button>
              </div>
            </a-form-item>
            <a-form-item label="题目顺序">
              <a-radio-group v-model:value="form.order">
                <a-radio-button value="random">随机</a-radio-button>
                <a-radio-button value="fixed">按题序</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item v-if="form.order !== 'fixed'" label="题量">
              <a-select
                v-model:value="form.count"
                :options="countOptions"
                style="width: 100%"
                @change="onCountChange"
              />
              <div v-if="pickSummary" class="range-hint muted">{{ pickSummary }}</div>
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
                      {{ orderLabel(item) }}
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
import { Flag, Sparkles } from 'lucide-vue-next'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import QuestionImage from '@/components/QuestionImage.vue'
import AiChatPanel from '@/components/AiChatPanel.vue'
import { aiApi } from '@/api/ai'
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
  // 按题序时的题序范围：起始必填，结束留空表示练到末尾
  startSeq: 1,
  endSeq: null as number | null,
  shuffleOptions: false,
})

const candidateTotal = ref(0)
const loadingCandidates = ref(false)
let candidateSeq = 0

async function loadCandidates() {
  const seq = ++candidateSeq
  if (form.bankIds.length === 0) {
    candidateTotal.value = 0
    loadingCandidates.value = false
    return
  }
  loadingCandidates.value = true
  try {
    const res = await practiceApi.preview({
      bankIds: form.bankIds,
      questionType: form.type || undefined,
      difficulty: form.difficulty || undefined,
      tags: form.tags.length ? form.tags : undefined,
    })
    if (seq === candidateSeq) candidateTotal.value = res.candidates
  } catch {
    if (seq === candidateSeq) candidateTotal.value = 0
  } finally {
    if (seq === candidateSeq) loadingCandidates.value = false
  }
}

watch(
  () => [form.bankIds.join(','), form.type, form.difficulty, form.tags.join(',')].join('|'),
  () => {
    void loadCandidates()
  },
)

// 题序范围摘要：显示实际会练到哪几道；起始 1 + 末题留空视为“全库”
const rangeSummary = computed(() => {
  const total = candidateTotal.value
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = total > 0 ? Math.min(rawEnd || total, total) : rawEnd
  if (end < start) return '结束题号小于起始题号'
  const span = end - start + 1
  if (rawEnd === 0) {
    return start <= 1 ? `全库 ${span} 道` : `第 ${start} 题到末题（共 ${span} 道）`
  }
  return `第 ${start}–${end} 题（共 ${span} 道）`
})

// 是否真的限定了范围（起始 1 + 末题留空 = 全库，不算）
const rangeActive = computed(() => {
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  return Math.max(1, form.startSeq || 1) > 1 || rawEnd > 0
})

// 范围里实际有多少道（没限定范围时等于候选总数）
const rangeSpan = computed(() => {
  if (!rangeActive.value) return candidateTotal.value
  const total = candidateTotal.value
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = total > 0 ? Math.min(rawEnd || total, total) : rawEnd
  if (end < start) return 0
  return end - start + 1
})

// 随机时实际会做多少道：题量选“全部”就是整段
const pickCount = computed(() => {
  const span = rangeSpan.value
  if (form.count <= 0) return span
  return span > 0 ? Math.min(form.count, span) : form.count
})

// 把“范围 + 题量”翻译成一句人话，免得用户自己算
const pickSummary = computed(() => {
  const total = candidateTotal.value
  if (form.order === 'fixed' || total <= 0) return ''
  const n = pickCount.value
  if (n <= 0) return ''
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = Math.min(rawEnd || total, total)
  if (!rangeActive.value) {
    return form.count > 0 ? `从全库 ${total} 道中随机抽 ${n} 道` : `全库 ${total} 道全部打乱`
  }
  return form.count > 0
    ? `从第 ${start}–${end} 题中随机抽 ${n} 道，不重复`
    : `第 ${start}–${end} 题全部打乱，共 ${n} 道`
})

// 切到“按题序”时，若还没填范围，就用题量兜一个默认段，保持原来的直觉
watch(
  () => form.order,
  (order) => {
    if (order !== 'fixed' || rangeActive.value) return
    form.startSeq = 1
    form.endSeq = form.count > 0 ? form.count : null
  },
)

// 用户亲手改过题量后就不再自动跟随范围
let countTouched = false
function onCountChange() {
  countTouched = true
}

// 范围一填，题量默认切到“全部”（这段全做）；清空范围则回到 20 题
watch(
  () => [form.startSeq, form.endSeq].join('|'),
  () => {
    if (form.order !== 'random' || countTouched) return
    form.count = rangeActive.value ? 0 : 20
  },
)

// 上次同题库“按题序”练习的结束位置 + 1，用于一键接着往下练
const nextRangeStart = computed(() => {
  if (form.order !== 'fixed' || form.bankIds.length !== 1) return 0
  const last = sessions.value.find(
    (s) => s.bankId === form.bankIds[0] && s.order === 'fixed' && s.source === 'bank',
  )
  if (!last) return 0
  const start = last.rangeStart > 0 ? last.rangeStart : 1
  const end = last.rangeEnd > 0 ? last.rangeEnd : start + last.totalCount - 1
  const next = end + 1
  if (candidateTotal.value > 0 && next > candidateTotal.value) return 0
  if (next <= (form.startSeq || 1) && form.endSeq !== null) return 0
  return next
})

function applyNextRange() {
  const next = nextRangeStart.value
  if (!next) return
  form.startSeq = next
  // 保留上次跨度，方便一段段往下练
  const last = sessions.value.find(
    (s) => s.bankId === form.bankIds[0] && s.order === 'fixed' && s.source === 'bank',
  )
  const span = last ? last.totalCount : 0
  const total = candidateTotal.value
  const end = span > 0 ? next + span - 1 : 0
  form.endSeq = end > 0 && (total === 0 || end <= total) ? end : null
}

// 范围的常用档位：前 N 题 / 全库，省得每次都敲数字
const rangePresets = [
  { label: '前 20', value: 20 },
  { label: '前 50', value: 50 },
  { label: '前 100', value: 100 },
  { label: '全库', value: 0 },
]

function isPresetActive(value: number) {
  if (value <= 0) return !rangeActive.value
  return (form.startSeq || 1) <= 1 && form.endSeq === value
}

function applyRangePreset(value: number) {
  const total = candidateTotal.value
  form.startSeq = 1
  form.endSeq = value > 0 ? (total > 0 ? Math.min(value, total) : value) : null
  // 选了档位等于重新定范围，题量交回自动跟随
  countTouched = false
}

// 是否加了筛选：题号是按筛选结果算的，得说清楚
const filterActive = computed(
  () => form.type !== '' || form.difficulty > 0 || form.tags.length > 0,
)

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
  // “全部”在当前范围下就是整段，把实际道数标出来
  if (rangeActive.value && rangeSpan.value > 0) {
    options[options.length - 1].label = `全部（${rangeSpan.value} 道）`
  }
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
const bodyRef = ref<HTMLElement | null>(null)
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

// AI 答疑：仅在服务端已启用并配置凭据时显示入口
// null=配置还在读取（引导不闪），false=未配置，true=已配置
const aiReady = ref<boolean | null>(null)
const aiOpen = ref(false)
const aiOptions = computed(() =>
  (current.value?.options ?? []).map((o) => `${o.label}. ${o.content}`),
)

async function loadAiConfig() {
  try {
    aiReady.value = (await aiApi.config()).ready
  } catch {
    aiReady.value = false
  }
}

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('MM-DD HH:mm') : '—'
}

function formatDuration(seconds: number) {
  const sec = Math.max(0, Math.round(seconds || 0))
  const minutes = Math.floor(sec / 60)
  if (minutes >= 60) return `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分`
  return `${minutes} 分 ${String(sec % 60).padStart(2, '0')} 秒`
}

function orderLabel(item: PracticeSession) {
  const start = item.rangeStart > 0 ? item.rangeStart : 0
  const end = start > 0 ? (item.rangeEnd > 0 ? item.rangeEnd : start + item.totalCount - 1) : 0
  if (item.order === 'random') {
    return start > 0 ? `随机 · 第 ${start}–${end} 题` : '随机'
  }
  return start > 0 ? `按题序 ${start}–${end}` : '按题序'
}

function clearAnswers() {
  for (const id of Object.keys(answers)) delete answers[Number(id)]
}

function applySession(detail: PracticeDetail) {
  clearAnswers()
  // 新练习/续练都算新开始：AI 面板复位，避免上一组的展开状态带到第一题、挂载即自动发首问
  aiOpen.value = false
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
  form.startSeq = 1
  form.endSeq = form.count > 0 ? form.count : null
}

async function removeSession(id: number) {
  await practiceApi.remove(id)
  sessions.value = sessions.value.filter((s) => s.id !== id)
  message.success('已删除练习记录')
}

async function start(source: 'bank' | 'wrong' = 'bank', bankIdsOverride?: number[]) {
  if (starting.value) return
  // 题序范围对两种顺序都生效：按题序是顺序练这一段，随机是只在这段里抽题
  const useRange = source === 'bank' && rangeActive.value
  if (useRange) {
    const start = Math.max(1, form.startSeq || 1)
    if (form.endSeq && form.endSeq > 0 && form.endSeq < start) {
      message.warning('结束题号不能小于起始题号')
      return
    }
  }
  starting.value = true
  try {
    const res = await practiceApi.start({
      bankIds: source === 'wrong' ? bankIdsOverride : form.bankIds,
      // 按题序时由题序范围决定题量；随机时由题量决定抽几道
      count: source === 'wrong' || form.order === 'fixed' ? 0 : form.count,
      order: form.order,
      startSeq: useRange ? Math.max(1, form.startSeq || 1) : undefined,
      endSeq: useRange ? form.endSeq || 0 : undefined,
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
    // 答对保持安静，只有答错才提示并说明已计入错题本
    if (!result.isCorrect) {
      message.warning(
        `回答错误，正确答案：${result.answer.join('') || '—'}，已计入错题本（累计答错 ${result.wrongCount} 次）`
      )
    }
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
  // 切题即收起 AI 面板：会话已随切题作废，留着只会挡住答题卡并在切题瞬间自动发新首问
  aiOpen.value = false
  index.value = next
  const item = questions.value[next]
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  questionStartedAt = Date.now()
  // 换题后把题目区滚回顶部，避免停在上一题的解析位置
  if (bodyRef.value) bodyRef.value.scrollTop = 0
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
  await Promise.all([loadBanks(), loadSessions(), loadSettings(), wrongStore.refresh(), loadAiConfig()])
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
/* 答题态锁定在一屏内，长内容由题目区内部滚动，不再把底部按钮顶出屏幕 */
.page-quiz { height: 100%; min-height: 0; overflow: hidden; }
.page-quiz > :first-child { flex: none; }
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
  /* 打开 AI 时右列从答题卡的 200px 展开成聊天列，宽度过渡和内容切换同步 */
  transition: grid-template-columns 0.28s ease;
}
.quiz-layout.ai-open { grid-template-columns: minmax(0, 1fr) 440px; }
.side-col { display: flex; flex-direction: column; min-height: 0; }
.side-col > * { flex: 1; min-height: 0; }

/* v-show 切换：display 翻转时动画重放，效果等同之前的 Transition；两面板常驻不销毁 */
.ai-host { animation: ai-in 0.22s ease; }
.answer-card { animation: card-in 0.12s ease; }
@keyframes ai-in {
  from { opacity: 0; transform: translateX(28px); }
  to { opacity: 1; transform: none; }
}
@keyframes card-in {
  from { opacity: 0; transform: translateY(-8px) scale(0.98); }
  to { opacity: 1; transform: none; }
}
.quiz { display: flex; flex-direction: column; padding: 18px 20px 20px; min-height: 0; overflow: hidden; }
/* 题干、选项、解析在这一层内部滚动，底部操作条永远留在可视区 */
.quiz-body {
  flex: 1; min-height: 0; display: flex; flex-direction: column;
  overflow-y: auto; padding-right: 4px;
}
.quiz-body, .q-grid { scrollbar-width: thin; scrollbar-color: var(--scroll-thumb) transparent; }
.quiz-body::-webkit-scrollbar,
.q-grid::-webkit-scrollbar { width: 6px; height: 6px; }
.quiz-body::-webkit-scrollbar-track,
.q-grid::-webkit-scrollbar-track { background: transparent; }
.quiz-body::-webkit-scrollbar-thumb,
.q-grid::-webkit-scrollbar-thumb { background: var(--scroll-thumb); border-radius: 99px; }
.quiz-body::-webkit-scrollbar-thumb:hover,
.q-grid::-webkit-scrollbar-thumb:hover { background: var(--scroll-thumb-hover); }
.quiz-body::-webkit-scrollbar-corner,
.q-grid::-webkit-scrollbar-corner { background: transparent; }
.quiz-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.quiz-head .muted { margin-left: 8px; font-size: 13px; }
.progress-hint { opacity: .8; }
.head-right { display: flex; align-items: center; gap: 10px; }
.mark-btn {
  width: 26px; height: 24px; padding: 0;
  display: inline-flex; align-items: center; justify-content: center;
  color: var(--text2);
}
.mark-btn :deep(.ant-btn-icon) { display: inline-flex; align-items: center; justify-content: center; line-height: 1; }
.mark-btn:hover,
.mark-btn.ant-btn-default:not(:disabled):hover,
.mark-btn.ant-btn-default:not(:disabled):focus { color: var(--mark); border-color: var(--mark-border); }
.mark-btn.marked {
  color: var(--mark);
  background: var(--mark-soft);
  border-color: var(--mark-border);
}
.mark-btn.marked:hover,
.mark-btn.marked.ant-btn-default:not(:disabled):hover { color: var(--mark); background: var(--mark-soft); border-color: var(--mark); }
.answer-card { display: flex; flex-direction: column; padding: 14px 12px 12px; min-height: 0; }
.side-head { display: flex; align-items: baseline; justify-content: space-between; }
.side-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.side-legend { display: flex; flex-wrap: wrap; gap: 4px 10px; margin: 10px 0; font-size: 11.5px; color: var(--text2); }
.side-legend span { display: inline-flex; align-items: center; gap: 4px; }
.dot { width: 9px; height: 9px; border-radius: 2px; border: 1px solid var(--border); background: var(--surface); }
.dot.ok { background: var(--ok-soft); border-color: var(--ok-text); }
.dot.bad { background: var(--bad-soft); border-color: var(--bad-text); }
.dot.mark { background: var(--mark-soft); border-color: var(--mark); }
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
  transition: border-color .12s, background .12s, color .12s, box-shadow .12s;
}
.q-cell:hover { border-color: var(--primary); color: var(--primary); }
/* 状态：浅色底 + 饱和边框 + 同色数字；声明顺序即叠加优先级（答错 > 标记 > 答对） */
.q-cell.ok { background: var(--ok-soft); border-color: var(--ok-text); color: var(--ok-text); }
.q-cell.marked { background: var(--mark-soft); border-color: var(--mark); color: var(--mark-text); }
.q-cell.bad { background: var(--bad-soft); border-color: var(--bad-text); color: var(--bad-text); }
.q-cell.ok:hover, .q-cell.bad:hover, .q-cell.marked:hover { filter: brightness(.985); }
/* 当前题：一律用主题绿实心填充，与这题是答对、答错还是标记无关 */
.q-cell.current,
.q-cell.current:hover {
  color: var(--surface);
  background: var(--primary);
  border-color: var(--primary);
  font-weight: 600;
}
.side-foot { margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border); text-align: right; }
.q-type { margin-top: 18px; font-size: 12.5px; color: var(--text2); }
.q-stem { margin-top: 6px; font-size: 16px; line-height: 1.6; white-space: pre-wrap; }
.q-img { display: block; margin-top: 10px; }
.opt-thumb { flex: none; }
.options { margin-top: 14px; display: flex; flex-direction: column; gap: 8px; }
.multi-submit { display: flex; justify-content: flex-end; margin-top: 10px; }
.opt {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 11px 12px; border: 1px solid var(--border-strong); border-radius: 8px;
  background: var(--surface-2); color: var(--text); cursor: pointer;
  transition: border-color .12s, background .12s;
}
.opt:hover { background: var(--hover); }
.opt.selected { border-color: #24945f; background: var(--ok-soft); }
.opt.correct { border-color: #24945f; background: var(--ok-soft); }
.opt.wrong { border-color: var(--bad-text); background: var(--bad-soft); }
.opt-key {
  flex: none; width: 24px; height: 24px; border-radius: 50%;
  border: 1.5px solid var(--border-strong); color: var(--text2);
  font-size: 12.5px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
}
.opt.selected .opt-key, .opt.correct .opt-key { background: #24945f; border-color: #24945f; color: #fff; }
.opt.wrong .opt-key { background: var(--bad-text); border-color: var(--bad-text); color: #fff; }
/* 行高与 24px 圆形字母齐平：antd reset 的 html{line-height:1.15} 会让首行只有 16px，
   flex-start 对齐后文字整体比圆圈高 2~3px，这里固定成 22px 让两者视觉居中一致 */
.opt-text { flex: 1; white-space: pre-wrap; line-height: 22px; }
/* 解析框贴内容高度：没解析时不占地方，解析长了自然长高、随 quiz-body 滚动 */
.quiz-body > *:not(.analysis-box) { flex: none; }
.analysis-box {
  flex: none; margin-top: 14px;
  padding: 12px 14px; border: 1px dashed var(--border); border-radius: 8px;
  background: var(--bg); font-size: 13.5px; line-height: 1.7;
}
.analysis-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 6px; }
.analysis-title { font-size: 13px; font-weight: 600; }
.ai-ask-btn { height: 24px; padding: 0 8px; font-size: 12px; }
/* AI 未配置引导：柔和药丸链接样式，与「问 AI」同位同尺寸 */
.ai-setup-hint {
  height: 24px;
  padding: 0 10px;
  font-size: 12px;
  border: none;
  border-radius: 999px;
  background: var(--primary-soft);
  color: var(--primary);
}
.analysis-text { color: var(--text); white-space: pre-wrap; }
.quiz-actions {
  display: flex; justify-content: space-between; align-items: center;
  padding-top: 12px; margin-top: 12px; border-top: 1px solid var(--border);
  flex-wrap: wrap; gap: 10px;
}
.kbd-hint { margin-top: 8px; font-size: 12px; text-align: right; }
.result { margin-top: 0; }
.switch-hint { margin-left: 10px; font-size: 12.5px; }
.range-row { display: flex; align-items: center; gap: 6px; }
.range-presets { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.range-input { width: 88px; flex: none; }
.range-sep { color: var(--text2); }
.range-unit { font-size: 12.5px; }
.range-hint { margin-top: 4px; font-size: 12px; line-height: 1.5; }
.range-next { padding: 0; height: auto; font-size: 12px; }
.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; }
.recent-main { min-width: 0; }
.recent-title { font-weight: 600; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.recent-tag { margin-left: 2px; }
.recent-right { display: flex; align-items: center; gap: 10px; flex: none; }
.recent-score { display: flex; flex-direction: column; align-items: flex-end; font-size: 13px; }
.recent-continue { padding-inline: 4px; }
@media (max-width: 900px) {
  .grid { grid-template-columns: 1fr; }
  .quiz-layout,
  .quiz-layout.ai-open { grid-template-columns: 1fr; grid-template-rows: auto; }
  /* 窄屏下面板纵向堆叠，AI 面板撑起足够高度 */
  .side-col { min-height: 480px; }
  .q-grid { max-height: 170px; }
}
</style>
