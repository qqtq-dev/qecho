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
        <div class="ai-source-label muted">已发送的题干</div>
        <div class="ai-source-stem">{{ stem }}</div>
      </div>

      <div v-for="(turn, idx) in turns" :key="idx" class="ai-turn">
        <div class="ai-q">
          <span class="ai-role">你</span>
          <span class="ai-q-text">{{ turn.question }}</span>
        </div>
        <div class="ai-a">
          <div class="ai-a-head">
            <span class="ai-role ai-role-bot" :class="{ 'is-ima': usedProvider === 'imaweb' }">{{ providerBadge }}</span>
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
        <a-select
          v-if="kbVisible"
          v-model:value="kbId"
          size="small"
          class="ai-kb-select"
          show-search
          option-filter-prop="label"
          :dropdown-match-select-width="260"
          @change="onKbChange"
        >
          <a-select-option value="">不限定知识库</a-select-option>
          <a-select-option v-for="kb in kbOptions" :key="kb.id" :value="kb.id" :label="kb.name">
            {{ kb.name }}
          </a-select-option>
        </a-select>
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
import { aiApi, type AiProvider, type KnowledgeBase } from '@/api/ai'

interface Turn {
  question: string
  answer: string
  state: 'thinking' | 'streaming' | 'done' | 'error'
  error: string
  thinking: string
  thinkOpen: boolean
  startedAt: number
  endedAt: number
}

const MODE_KEY = 'quiz-ai-mode'

const props = defineProps<{
  open: boolean
  questionId: number
  stem: string
  options: string[]
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
// 本次答疑实际走的接入方式：打开时取配置，提问后以引擎 meta 帧为准（两者不一致说明分派有变）。
const usedProvider = ref<AiProvider | ''>('')
// 回答者头像上的字：按本次实际接入方式显示 bic / ima，未加载到配置前退回 AI。
const providerBadge = computed(() => {
  if (usedProvider.value === 'imaweb') return 'ima'
  if (usedProvider.value === 'bicqa') return 'bic'
  return 'AI'
})
// 知识库：只有 ima 网页版才可选，选定后本次答疑会先在该库里检索。
const kbVisible = ref(false)
const kbOptions = ref<KnowledgeBase[]>([])
const kbId = ref('')
const kbLoaded = ref(false)
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

/** 打开抽屉时读一次配置：当前接入方式、是不是 ima 网页版、默认选的是哪个知识库。 */
async function loadKbContext() {
  try {
    const cfg = await aiApi.config()
    // 每次打开都刷新，避免在设置页切换接入方式后这里仍显示旧值。
    usedProvider.value = cfg.provider
    kbVisible.value = cfg.provider === 'imaweb' && cfg.hasCookie
    if (!kbVisible.value || kbLoaded.value) return
    kbId.value = cfg.kbId || ''
    const res = await aiApi.knowledgeBases()
    kbOptions.value = res.items ?? []
    kbLoaded.value = true
  } catch {
    /* 引擎未就绪或未配置时静默，入口不显示即可 */
  }
}

/** 换知识库要开新会话：会话是建库时就绑定了机器人的。 */
function onKbChange() {
  conversationId.value = ''
}

function buildInitialQuestion() {
  const lines = ['请分析这道题：正确答案是哪几个选项？', '']
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
  conversationId.value = ''
  input.value = ''
  version.value = 0
  stopTimer()
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
    thinkOpen: true,
    startedAt: Date.now(),
    endedAt: 0,
  })
  turns.value.push(turn)
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
        // 只在 ima 网页版下传：空串表示这次走通用问答，覆盖设置里的知识库。
        ...(kbVisible.value ? { kbId: kbId.value } : {}),
      },
      {
        onMeta: (id, provider) => {
          conversationId.value = id
          usedProvider.value = provider
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
    if (open) void loadKbContext()
  },
  { immediate: true },
)

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
/* ima 网页版用蓝色，和 BIC-QA 的绿色一眼区分 */
.ai-role-bot.is-ima { background: var(--info-soft); color: var(--info-text); }
.ai-q-text { white-space: pre-wrap; max-height: 88px; overflow: auto; }
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
  max-height: 44px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  margin-top: 6px;
}
.ai-think.is-open .ai-think-body {
  max-height: 300px;
  overflow: auto;
  justify-content: flex-start;
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
.ai-kb-select { width: 150px; }
.ai-input-foot { display: flex; align-items: center; margin-top: 8px; gap: 10px; }
.ai-hint { font-size: 11.5px; flex: 1; }
</style>
