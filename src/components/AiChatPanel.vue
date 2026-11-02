<template>
  <section class="ai-panel">
    <div class="ai-head">
      <span class="ai-title">{{ channelTitle }}</span>
      <span v-if="active?.state === 'thinking'" class="ai-state is-busy">思考中 {{ elapsedOf(active) }}s</span>
      <span v-else-if="active?.state === 'streaming'" class="ai-state is-busy">回答中</span>
      <span v-else-if="active?.state === 'error'" class="ai-state is-error">出错</span>
      <span v-else-if="active?.state === 'done' && turns.length" class="ai-state is-done">已完成</span>
    </div>

    <div ref="scrollRef" class="ai-scroll">
      <div v-for="(turn, idx) in turns" :key="idx" class="ai-turn">
        <!-- 我：靠右，气泡底色；长提问默认折三行（末尾自带 …），点气泡本身展开/收起 -->
        <div class="ai-row ai-row-me">
          <div
            class="ai-bubble mine"
            :class="{ 'is-toggleable': turn.qOverflow }"
            :title="turn.qOverflow ? (turn.qOpen ? '点击收起' : '点击展开全文') : undefined"
            @click="turn.qOverflow && toggleQuestion(turn)"
          >
            <div
              :ref="(el) => setQRef(el as HTMLElement | null, idx)"
              class="ai-q-text"
              :class="{ 'is-clamped': !turn.qOpen }"
            >{{ turn.question }}</div>
          </div>
          <span class="ai-role">我</span>
        </div>

        <!-- AI：靠左，浅底气泡，推理与正文都在气泡里 -->
        <div class="ai-row ai-row-bot">
          <span class="ai-role ai-role-bot">AI</span>
          <div class="ai-bubble bot">
            <div v-if="turn.state === 'thinking' && !turn.thinking" class="ai-wait">
              <a-spin size="small" />
              <span>正在连接…</span>
            </div>

            <div v-if="turn.thinking" class="ai-think" :class="{ 'is-open': turn.thinkOpen }">
              <button class="ai-think-head" type="button" @click="turn.thinkOpen = !turn.thinkOpen">
                <a-spin v-if="turn.state === 'thinking'" size="small" />
                <span class="ai-think-title">{{ turn.state === 'thinking' ? '正在思考' : '思考过程' }}</span>
                <span class="muted ai-think-meta">{{ thinkLen(turn) }} 字 · {{ elapsedOf(turn) }}s</span>
                <span class="ai-think-toggle">{{ turn.thinkOpen ? '收起' : '展开' }}</span>
              </button>
              <div class="ai-think-body"><div class="ai-think-text">{{ turn.thinking }}</div></div>
            </div>

            <div v-if="turn.answer" class="ai-md" v-html="rendered[idx]" />
            <div v-if="turn.state === 'streaming'" class="ai-caret">▍</div>
            <a-alert
              v-if="turn.state === 'error'"
              type="error"
              show-icon
              :message="turn.error || 'AI 调用失败'"
              class="ai-alert"
            />
            <div v-if="idx === turns.length - 1 && busy" class="ai-stop-row">
              <a class="ai-mini" @click="stop">停止</a>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="ai-input">
      <a-textarea
        v-model:value="input"
        :rows="2"
        :disabled="busy"
        placeholder="继续追问，例如：这个选项为什么错？/ 能给个 SQL 例子吗？"
        @press-enter.exact.prevent="ask"
      />
      <div class="ai-input-foot">
        <a-radio-group v-model:value="mode" size="small">
          <a-radio-button value="brief">简答</a-radio-button>
          <a-radio-button value="full">详答</a-radio-button>
        </a-radio-group>
        <span class="muted ai-hint">
          {{ mode === 'brief' ? '约 200 字，通常 10–20 秒' : '逐项讲解，通常 30–90 秒' }}
        </span>
        <a-button type="primary" size="small" :loading="busy" :disabled="!input.trim()" @click="ask">
          发送
        </a-button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { aiApi, aiChannelTitle, type ChatMessage } from '@/api/ai'

interface Turn {
  question: string
  answer: string
  state: 'thinking' | 'streaming' | 'done' | 'error'
  error: string
  thinking: string
  thinkOpen: boolean
  /** 长提问默认折三行，点气泡本身展开 */
  qOpen: boolean
  qOverflow: boolean
  startedAt: number
  endedAt: number
}

const MODE_KEY = 'quiz-ai-mode'

const props = defineProps<{
  /** 面板常驻挂载（父组件用 v-show 切换显示），open 表示当前是否可见，决定要不要发起首问 */
  open?: boolean
  questionId: number
  stem: string
  options: string[]
  /** 题型：单选只有一个正确答案，多选可能不止一个，必须告诉 AI，否则容易按多选答。 */
  type: 'single' | 'multiple'
}>()

/** 收起由父组件的「收起 AI」按钮控制（aiOpen），面板自身不再发 update:open */

marked.setOptions({ gfm: true, breaks: true })

const turns = ref<Turn[]>([])
const conversationId = ref('')
const input = ref('')
const busy = ref(false)
const mode = ref<'brief' | 'full'>(
  localStorage.getItem(MODE_KEY) === 'full' ? 'full' : 'brief',
)
const scrollRef = ref<HTMLElement | null>(null)
let controller: AbortController | null = null
let lastBump = 0
const version = ref(0)
const tick = ref(0)
let timer: number | null = null

const active = computed(() => turns.value[turns.value.length - 1])

const rendered = computed(() => {
  void version.value
  return turns.value.map((t) =>
    t.answer ? DOMPurify.sanitize(marked.parse(t.answer, { async: false }) as string) : '',
  )
})

function bump(force = false) {
  const now = Date.now()
  if (force || now - lastBump > 120) {
    lastBump = now
    version.value += 1
  }
}

function scrollToBottom() {
  nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

// 提问是否超过三行：折叠状态下量一次，展开时不重复量，否则全文显示会把「收起」按钮带走
const qEls = new Map<number, HTMLElement>()

function setQRef(el: HTMLElement | null, idx: number) {
  if (el) qEls.set(idx, el)
  else qEls.delete(idx)
}

function measureQuestions() {
  nextTick(() => {
    for (const [idx, el] of qEls) {
      const turn = turns.value[idx]
      if (!turn || turn.qOpen) continue
      const overflow = el.scrollHeight > el.clientHeight + 2
      if (overflow !== turn.qOverflow) turn.qOverflow = overflow
    }
  })
}

function toggleQuestion(turn: Turn) {
  turn.qOpen = !turn.qOpen
  if (turn.qOpen) scrollToBottom()
  else measureQuestions()
}

function thinkLen(turn: Turn) {
  void tick.value
  return turn.thinking.length
}

function elapsedOf(turn?: Turn) {
  void tick.value
  if (!turn?.startedAt) return 0
  return Math.max(0, Math.round(((turn.endedAt || Date.now()) - turn.startedAt) / 1000))
}

function startTimer() {
  if (timer !== null) return
  timer = window.setInterval(() => {
    tick.value += 1
  }, 500)
}

function stopTimer() {
  if (timer !== null) {
    window.clearInterval(timer)
    timer = null
  }
}

function buildInitialQuestion() {
  const single = props.type !== 'multiple'
  const lines = ['请分析这道题：正确答案是哪几个选项？', '']
  // 题型必须显式给出：不说单选，模型常按多选作答，给出多个答案。
  lines.push(single ? '题型：单选题（只有一个正确答案）' : '题型：多选题（正确答案可能不止一个）')
  lines.push(`题干：${props.stem}`)
  if (props.options.length) {
    lines.push('选项：')
    for (const opt of props.options) lines.push(opt)
  }
  return lines.join('\n')
}

function reset() {
  controller?.abort()
  controller = null
  busy.value = false
  turns.value = []
  qEls.clear()
  conversationId.value = ''
  input.value = ''
  version.value = 0
  stopTimer()
}

/**
 * 面板由父组件 v-if 控制挂载，关闭即销毁实例。把已完成的会话按题目缓存在模块层，
 * 同一题关掉再打开时原样恢复，不重发首问；切题时也能找回之前问过的记录。
 * 只留最近 20 题，防止长时间练习内存一直涨。
 */
const CACHE_MAX = 20
const chatCache = new Map<number, { turns: Turn[]; conversationId: string }>()

function saveToCache() {
  if (!turns.value.length) return
  // 在途的轮次定格：有正文就算完成，只有思考没正文就丢弃
  for (const t of turns.value) {
    if (t.state !== 'done' && t.state !== 'error') {
      if (t.answer.trim()) {
        t.state = 'done'
        if (!t.endedAt) t.endedAt = Date.now()
      }
    }
  }
  const kept = turns.value.filter((t) => t.state === 'done' || t.state === 'error')
  if (!kept.length) return
  chatCache.delete(props.questionId)
  chatCache.set(props.questionId, { turns: kept, conversationId: conversationId.value })
  while (chatCache.size > CACHE_MAX) {
    const oldest = chatCache.keys().next().value
    if (oldest === undefined) break
    chatCache.delete(oldest)
  }
}

function restoreFromCache(id: number): boolean {
  const saved = chatCache.get(id)
  if (!saved) return false
  chatCache.delete(id)
  turns.value = saved.turns
  conversationId.value = saved.conversationId
  input.value = ''
  busy.value = false
  version.value = 0
  return true
}

/** 最多带入的已完成轮次。通用模型按 token 计费，太长既贵又挤占回答本身的额度。 */
const HISTORY_TURNS = 10

/**
 * 取最近若干轮已完成的问答作为多轮上下文。BIC-QA 把历史存在服务端，收到也会忽略；
 * 通用大模型是无状态的，不传就退化成单轮，追问「那 B 选项呢」会答不上来。
 */
function buildHistory(): ChatMessage[] {
  const done = turns.value.filter((t) => t.state === 'done' && t.answer.trim())
  return done.slice(-HISTORY_TURNS).flatMap((t) => [
    { role: 'user' as const, content: t.question },
    { role: 'assistant' as const, content: t.answer },
  ])
}

async function send(question: string) {
  if (busy.value) return
  const text = question.trim()
  if (!text) return
  busy.value = true
  const turn = reactive<Turn>({
    question: text,
    answer: '',
    state: 'thinking',
    error: '',
    thinking: '',
    // 思考过程默认收起：标题行已经能看出在思考和耗时，正文不再挤占版面
    thinkOpen: false,
    qOpen: false,
    qOverflow: false,
    startedAt: Date.now(),
    endedAt: 0,
  })
  // 历史要在本轮压进 turns 之前取，否则当前这轮的空答案也会被算进去
  const history = buildHistory()
  turns.value.push(turn)
  measureQuestions()
  startTimer()
  scrollToBottom()

  controller = new AbortController()
  try {
    await aiApi.streamChat(
      {
        question: text,
        conversationId: conversationId.value || undefined,
        title: `题鸣答疑 · ${props.stem.slice(0, 20)}`,
        mode: mode.value,
        history,
      },
      {
        onMeta: (id) => {
          conversationId.value = id
        },
        onThinking: (chunk) => {
          turn.thinking += chunk
          bump()
          scrollToBottom()
        },
        onDelta: (chunk) => {
          turn.answer += chunk
          if (turn.state === 'thinking') {
            turn.state = 'streaming'
            // 正文开始后自动收起思考，避免把答案挤到下面
            turn.thinkOpen = false
          }
          bump()
          scrollToBottom()
        },
        onError: (msg) => {
          turn.state = 'error'
          turn.error = msg
          turn.endedAt = Date.now()
          bump(true)
        },
        onDone: () => {
          if (turn.state !== 'error') turn.state = 'done'
          turn.endedAt = Date.now()
          turn.thinkOpen = false
          bump(true)
          scrollToBottom()
        },
      },
      controller.signal,
    )
  } catch (err) {
    if ((err as Error)?.name === 'AbortError') {
      turn.state = 'done'
    } else {
      turn.state = 'error'
      turn.error = err instanceof Error ? err.message : '请求失败'
      bump(true)
    }
    turn.endedAt = Date.now()
  } finally {
    busy.value = false
    controller = null
    if (turn.state === 'thinking') turn.state = 'done'
    if (!turn.endedAt) turn.endedAt = Date.now()
    stopTimer()
    bump(true)
  }
}

function ask() {
  const text = input.value
  input.value = ''
  void send(text)
}

function stop() {
  controller?.abort()
  controller = null
  busy.value = false
  stopTimer()
}

/** 展开时的入口：有记录直接恢复，没有才对当前题发起首问 */
function openOrRestore() {
  if (turns.value.length || restoreFromCache(props.questionId)) {
    measureQuestions()
    scrollToBottom()
  } else {
    void send(buildInitialQuestion())
  }
}

// 标题跟当前生效通道走：BIC-QA 显示 BIC-QA，通用模型显示服务商名（匹配不到就叫通用大模型）
const channelTitle = ref('AI 答疑')

// 容器尺寸变化时重测折叠（面板刚挂载、宽度过渡中量到的是零尺寸，会误判「没溢出」）
let resizeObserver: ResizeObserver | null = null
onMounted(() => {
  resizeObserver = new ResizeObserver(() => measureQuestions())
  if (scrollRef.value) resizeObserver.observe(scrollRef.value)
  // 常驻挂载：挂载时若已展开（比如从别的页切回来）也要接上会话
  if (props.open) openOrRestore()
  aiApi.config()
    .then((cfg) => { channelTitle.value = aiChannelTitle(cfg) })
    .catch(() => { /* 读不到配置就保持默认标题 */ })
})
onBeforeUnmount(() => {
  saveToCache()
  resizeObserver?.disconnect()
  resizeObserver = null
})

watch(mode, (value) => {
  localStorage.setItem(MODE_KEY, value)
})

// 收起状态里切题：旧题会话存缓存，新题有缓存就静默恢复，等展开时直接看
watch(
  () => props.questionId,
  () => {
    saveToCache()
    reset()
    if (props.open) openOrRestore()
    else restoreFromCache(props.questionId)
  },
)

// 每次展开：空会话才发首问，收起期间后台写完的回答原样呈现
watch(
  () => props.open,
  (visible) => {
    if (visible) openOrRestore()
  },
)
</script>

<style scoped>
.ai-panel { display: flex; flex-direction: column; height: 100%; min-height: 0; }
.ai-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px 10px;
  border-bottom: 1px solid var(--border);
}
.ai-title { font-size: 14.5px; font-weight: 600; }
/* 状态标签：浅色药丸，配色取 antd 标准 soft 色，不用主题里偏灰的软色 */
.ai-state {
  padding: 1px 9px;
  border-radius: 10px;
  border: 1px solid transparent;
  font-size: 12px;
  line-height: 18px;
}
.ai-state.is-busy { background: #e6f4ff; border-color: #91caff; color: #0958d9; }
.ai-state.is-done { background: #f6ffed; border-color: #b7eb8f; color: #389e0d; }
.ai-state.is-error { background: #fff2f0; border-color: #ffccc7; color: #d4380d; }
.ai-scroll { flex: 1; min-height: 0; overflow: auto; padding: 14px 14px 8px; }

.ai-turn { margin-bottom: 18px; }
.ai-turn:last-child { margin-bottom: 6px; }
.ai-row { display: flex; align-items: flex-start; gap: 8px; }
.ai-row-me { justify-content: flex-end; margin-bottom: 12px; }
.ai-role {
  flex: none;
  height: 20px;
  padding: 0 7px;
  border-radius: 10px;
  background: var(--chip-bg);
  color: var(--chip-text);
  font-size: 11.5px;
  line-height: 20px;
}
.ai-role-bot { background: var(--ok-soft); color: var(--ok-text); }

.ai-bubble {
  min-width: 0;
  padding: 8px 11px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.6;
}
/* 我的提问：主色淡底、右上角收尖 */
.ai-bubble.mine {
  max-width: 88%;
  background: var(--primary-soft);
  border-top-right-radius: 3px;
}
/* AI 回答：卡片淡底、左上角收尖 */
.ai-bubble.bot {
  flex: 1;
  min-width: 0;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-top-left-radius: 3px;
}
.ai-q-text { white-space: pre-wrap; word-break: break-word; font-size: 13px; line-height: 1.6; }
/* 长提问默认折三行，不出现内部滚动条 */
.ai-q-text.is-clamped {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
/* 超过三行的提问气泡可点击展开/收起：手型光标 + 悬停微亮做暗示，省略号由 line-clamp 自带 */
.ai-bubble.mine.is-toggleable { cursor: pointer; transition: filter .15s ease; }
.ai-bubble.mine.is-toggleable:hover { filter: brightness(.97); }
.ai-mini { font-size: 12px; }
.ai-stop-row { text-align: right; margin-top: 4px; }
.ai-wait { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text2); padding: 4px 0; }
.ai-caret { color: var(--primary); font-size: 13px; line-height: 1; }
.ai-alert { margin-top: 8px; }

.ai-think {
  margin: 2px 0 10px;
  padding: 8px 10px;
  border-left: 2px solid var(--border-strong);
  background: var(--surface-3);
  border-radius: 0 6px 6px 0;
}
.ai-think.is-open { border-left-color: var(--ok-border); }
.ai-think-head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  text-align: left;
  font-size: 12.5px;
  color: var(--text2);
}
.ai-think-title { font-weight: 500; }
.ai-think-meta { font-size: 11.5px; }
.ai-think-toggle { margin-left: auto; font-size: 11.5px; color: var(--primary); }
.ai-think-body {
  max-height: 0;
  overflow: hidden;
  margin-top: 0;
  transition: max-height 0.18s ease, margin-top 0.18s ease;
}
.ai-think.is-open .ai-think-body {
  max-height: 300px;
  overflow: auto;
  margin-top: 6px;
}
/* 滚动条跟左侧题目区同一套：6px 细条、圆角、透明轨道，去掉系统默认的箭头粗条 */
.ai-scroll,
.ai-think.is-open .ai-think-body { scrollbar-width: thin; scrollbar-color: var(--scroll-thumb) transparent; }
.ai-scroll::-webkit-scrollbar,
.ai-think.is-open .ai-think-body::-webkit-scrollbar { width: 6px; height: 6px; }
.ai-scroll::-webkit-scrollbar-track,
.ai-think.is-open .ai-think-body::-webkit-scrollbar-track { background: transparent; }
.ai-scroll::-webkit-scrollbar-thumb,
.ai-think.is-open .ai-think-body::-webkit-scrollbar-thumb { background: var(--scroll-thumb); border-radius: 99px; }
.ai-scroll::-webkit-scrollbar-thumb:hover,
.ai-think.is-open .ai-think-body::-webkit-scrollbar-thumb:hover { background: var(--scroll-thumb-hover); }
.ai-scroll::-webkit-scrollbar-corner,
.ai-think.is-open .ai-think-body::-webkit-scrollbar-corner { background: transparent; }
.ai-think-text {
  white-space: pre-wrap;
  font-size: 12.5px;
  line-height: 1.65;
  color: var(--text2);
  opacity: 0.9;
}

.ai-md { font-size: 13.5px; line-height: 1.75; word-break: break-word; }
.ai-md :deep(h1),
.ai-md :deep(h2),
.ai-md :deep(h3),
.ai-md :deep(h4) {
  font-size: 14px;
  font-weight: 600;
  margin: 12px 0 6px;
}
.ai-md :deep(p) { margin: 6px 0; }
.ai-md :deep(ul),
.ai-md :deep(ol) { padding-left: 20px; margin: 6px 0; }
.ai-md :deep(li) { margin: 2px 0; }
.ai-md :deep(code) {
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--code-bg);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 12.5px;
}
.ai-md :deep(pre) {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--surface-3);
  border: 1px solid var(--border);
  overflow: auto;
  margin: 8px 0;
}
.ai-md :deep(pre code) { background: none; padding: 0; }
.ai-md :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 8px 0;
  font-size: 12.5px;
}
.ai-md :deep(th),
.ai-md :deep(td) { border: 1px solid var(--border); padding: 5px 8px; text-align: left; }
.ai-md :deep(th) { background: var(--surface-3); font-weight: 600; }
.ai-md :deep(blockquote) {
  margin: 8px 0;
  padding: 2px 10px;
  border-left: 3px solid var(--ok-border);
  color: var(--text2);
}
.ai-input {
  flex: none;
  padding: 10px 14px 12px;
  border-top: 1px solid var(--border);
  background: var(--surface);
}
.ai-input-foot { display: flex; align-items: center; margin-top: 8px; gap: 10px; }
.ai-hint { font-size: 11.5px; flex: 1; }
</style>
