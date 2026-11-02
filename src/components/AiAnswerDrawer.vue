<template>
  <a-drawer
    :open="open"
    placement="right"
    :width="560"
    :body-style="{ padding: '0', display: 'flex', flexDirection: 'column', height: '100%' }"
    @close="close"
  >
    <template #title>
      <div class="ai-title">
        <span>AI 答疑</span>
        <a-tag v-if="active?.state === 'thinking'" color="processing">思考中 {{ elapsedOf(active) }}s</a-tag>
        <a-tag v-else-if="active?.state === 'streaming'" color="processing">回答中</a-tag>
        <a-tag v-else-if="active?.state === 'error'" color="error">出错</a-tag>
        <a-tag v-else-if="active?.state === 'done'" color="success">已完成</a-tag>
      </div>
    </template>

    <div ref="scrollRef" class="ai-scroll">
      <div class="ai-source">
        <div class="ai-source-label muted">
          已发送的题干 · {{ type === 'multiple' ? '多选题' : '单选题' }}
        </div>
        <div class="ai-source-stem">{{ stem }}</div>
      </div>

      <div v-for="(turn, idx) in turns" :key="idx" class="ai-turn">
        <div class="ai-q">
          <span class="ai-role">你</span>
          <div class="ai-q-body">
            <div
              :ref="(el) => setQRef(el as HTMLElement | null, idx)"
              class="ai-q-text"
              :class="{ 'is-clamped': !turn.qOpen }"
            >{{ turn.question }}</div>
            <button
              v-if="turn.qOverflow"
              class="ai-q-more"
              type="button"
              @click="toggleQuestion(turn)"
            >{{ turn.qOpen ? '收起' : '展开全部' }}</button>
          </div>
        </div>
        <div class="ai-a">
          <div class="ai-a-head">
            <span class="ai-role ai-role-bot">AI</span>
            <a-space :size="8">
              <a v-if="turn.answer" class="ai-mini" @click="copy(turn.answer)">复制</a>
              <a
                v-if="idx === turns.length - 1 && busy"
                class="ai-mini"
                @click="stop"
              >停止</a>
            </a-space>
          </div>

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
  </a-drawer>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { aiApi, type ChatMessage } from '@/api/ai'

interface Turn {
  question: string
  answer: string
  state: 'thinking' | 'streaming' | 'done' | 'error'
  error: string
  thinking: string
  thinkOpen: boolean
  /** 长题干默认折叠三行，点「展开全部」才完整显示 */
  qOpen: boolean
  qOverflow: boolean
  startedAt: number
  endedAt: number
}

const MODE_KEY = 'quiz-ai-mode'

const props = defineProps<{
  open: boolean
  questionId: number
  stem: string
  options: string[]
  /** 题型：单选只有一个正确答案，多选可能不止一个，必须告诉 AI，否则容易按多选答。 */
  type: 'single' | 'multiple'
}>()

const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

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

// 题干是否超过三行：折叠状态下量一次，展开时不重复量，否则全文显示会把「收起」按钮带走
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

function close() {
  stop()
  emit('update:open', false)
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    message.success('已复制')
  } catch {
    message.warning('复制失败，请手动选择文本')
  }
}

watch(mode, (value) => {
  localStorage.setItem(MODE_KEY, value)
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      if (turns.value.length === 0) void send(buildInitialQuestion())
    } else {
      stop()
    }
  },
)

watch(
  () => props.questionId,
  () => {
    reset()
    if (props.open) emit('update:open', false)
  },
)
</script>

<style scoped>
.ai-title { display: flex; align-items: center; gap: 8px; font-size: 15px; }
.ai-scroll { flex: 1; min-height: 0; overflow: auto; padding: 14px 16px 8px; }
.ai-source {
  padding: 10px 12px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  background: var(--bg);
}
.ai-source-label { font-size: 11.5px; margin-bottom: 4px; }
.ai-source-stem {
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.ai-turn { margin-top: 16px; }
.ai-q { display: flex; gap: 8px; font-size: 13px; line-height: 1.6; }
.ai-role {
  flex: none;
  height: 20px;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--chip-bg);
  color: var(--chip-text);
  font-size: 11.5px;
  line-height: 20px;
}
.ai-role-bot { background: var(--ok-soft); color: var(--ok-text); }
.ai-q-body { flex: 1; min-width: 0; }
.ai-q-text {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 13px;
  line-height: 1.6;
}
/* 长题干默认折三行，不出现内部滚动条 */
.ai-q-text.is-clamped {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.ai-q-more {
  margin-top: 2px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--primary);
  font-size: 11.5px;
  line-height: 1.6;
  cursor: pointer;
}
.ai-q-more:hover { text-decoration: underline; }
.ai-a { margin-top: 8px; }
.ai-a-head { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.ai-mini { font-size: 12px; }
.ai-wait { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text2); padding: 6px 0; }
.ai-caret { color: var(--primary); font-size: 13px; line-height: 1; }
.ai-alert { margin-top: 8px; }

.ai-think {
  margin: 4px 0 10px;
  padding: 8px 10px;
  border-left: 2px solid var(--border-strong);
  background: var(--surface-2);
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
  padding: 10px 16px 14px;
  border-top: 1px solid var(--border);
  background: var(--surface);
}
.ai-input-foot { display: flex; align-items: center; margin-top: 8px; gap: 10px; }
.ai-hint { font-size: 11.5px; flex: 1; }
</style>
