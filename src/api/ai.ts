import { http, getApiBase, getToken } from './request'

/** 两套接口：bicqa 是 BIC-QA 私有协议，openai 是通用大模型的兼容协议。 */
export type AiProvider = 'bicqa' | 'openai'

/** 两条通道各自独立的配置。切换服务商时互不覆盖。 */
export interface AiChannel {
  enabled: boolean
  hasKey: boolean
  keyMask: string
  baseUrl: string
  model: string
  timeoutSec: number
}

export interface AiConfig {
  /** 当前生效的通道。两条只能同时生效一条。 */
  active: AiProvider
  ready: boolean
  bicqa: AiChannel
  openai: AiChannel
}

export interface AiTestResult {
  ok: boolean
  conversationId?: string
  baseUrl: string
  model?: string
}

/** 只提交当前生效通道的改动，另一条由服务端原样保留。 */
export interface AiConfigInput {
  active?: AiProvider
  channel?: {
    enabled?: boolean
    baseUrl?: string
    apiKey?: string
    model?: string
    timeoutSec?: number
    clearKey?: boolean
  }
}

/** 一问一答的上下文单元。 */
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface ChatHandlers {
  onMeta?: (conversationId: string) => void
  onThinking?: (text: string) => void
  onDelta?: (text: string) => void
  onError?: (message: string) => void
  onDone?: () => void
}

export interface ChatInput {
  question: string
  conversationId?: string
  title?: string
  /** 回答长度：brief 简答（默认，思考更快）、full 详答。 */
  mode?: 'brief' | 'full'
  /**
   * 多轮上下文。BIC-QA 把历史存在服务端，会忽略该字段；
   * 通用大模型无状态，靠它维持多轮，不传就变成单轮问答。
   */
  history?: ChatMessage[]
}

function dispatchFrame(frame: string, handlers: ChatHandlers) {
  let event = 'message'
  const dataLines: string[] = []
  for (const raw of frame.split('\n')) {
    const line = raw.replace(/\r$/, '')
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (dataLines.length === 0) return
  let payload: Record<string, unknown> = {}
  try {
    payload = JSON.parse(dataLines.join('\n')) as Record<string, unknown>
  } catch {
    return
  }
  const text = typeof payload.text === 'string' ? payload.text : ''
  const message = typeof payload.message === 'string' ? payload.message : ''
  switch (event) {
    case 'meta': {
      // 首帧只用来绑定会话；id 为空时不回调，避免把已有的多轮会话误清空。
      const id = payload.conversationId
      if (typeof id === 'string' && id) handlers.onMeta?.(id)
      break
    }
    case 'thinking':
      if (text) handlers.onThinking?.(text)
      break
    case 'delta':
      if (text) handlers.onDelta?.(text)
      break
    case 'error':
      handlers.onError?.(message || 'AI 返回错误')
      break
    case 'done':
      handlers.onDone?.()
      break
  }
}

/**
 * 流式问答。引擎以 SSE 输出统一的 meta / thinking / delta / error / done 事件，
 * 这里用 fetch + ReadableStream 逐帧解析（axios 不支持流式读取）。
 */
export async function streamChat(
  input: ChatInput,
  handlers: ChatHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const resp = await fetch(`${getApiBase()}/ai/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      'X-Token': getToken(),
    },
    body: JSON.stringify(input),
    signal,
  })
  if (!resp.ok) {
    let msg = `请求失败（HTTP ${resp.status}）`
    try {
      const body = (await resp.json()) as { message?: string }
      if (body?.message) msg = body.message
    } catch {
      /* 非 JSON 错误体，保留默认提示 */
    }
    throw new Error(msg)
  }
  if (!resp.body) throw new Error('当前环境不支持流式响应')

  const reader = resp.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''
  let finished = false
  const wrapped: ChatHandlers = {
    ...handlers,
    onDone: () => {
      finished = true
      handlers.onDone?.()
    },
  }

  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    buffer = buffer.replace(/\r\n/g, '\n')
    let idx = buffer.indexOf('\n\n')
    while (idx >= 0) {
      const frame = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      dispatchFrame(frame, wrapped)
      idx = buffer.indexOf('\n\n')
    }
  }
  if (buffer.trim()) dispatchFrame(buffer, wrapped)
  if (!finished) wrapped.onDone?.()
}

export const aiApi = {
  config: () => http.get<unknown, AiConfig>('/ai/config'),
  saveConfig: (data: AiConfigInput) => http.put<unknown, AiConfig>('/ai/config', data),
  test: () => http.post<unknown, AiTestResult>('/ai/test', {}),
  usage: () => http.get<unknown, Record<string, unknown>>('/ai/usage'),
  streamChat,
}

/** 通用大模型的常用服务商预设。baseUrl 统一填到版本段为止，末尾的 /chat/completions 由程序补。 */
export interface AiPreset {
  value: string
  label: string
  baseUrl: string
  model: string
}

export const AI_PRESETS: AiPreset[] = [
  {
    value: 'dashscope',
    label: '阿里云百炼 · 通义千问',
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    model: 'qwen-plus',
  },
  {
    value: 'deepseek',
    label: 'DeepSeek',
    baseUrl: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
  },
  {
    value: 'zhipu',
    label: '智谱 GLM',
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4',
    model: 'glm-4-flash',
  },
  {
    value: 'moonshot',
    label: '月之暗面 Kimi',
    baseUrl: 'https://api.moonshot.cn/v1',
    model: 'moonshot-v1-8k',
  },
  {
    value: 'siliconflow',
    label: '硅基流动',
    baseUrl: 'https://api.siliconflow.cn/v1',
    model: 'Qwen/Qwen2.5-7B-Instruct',
  },
  {
    value: 'stepfun',
    label: '阶跃星辰',
    // 用实测可用的 step_plan 通道；官方通用段是 https://api.stepfun.com/v1
    baseUrl: 'https://api.stepfun.com/step_plan/v1',
    model: 'step-5-preview',
  },
  {
    value: 'tokenrhythm',
    label: '基元律动',
    baseUrl: 'https://tokenrhythm.studio/v1',
    model: 'deepseek-v4-flash',
  },
  {
    value: 'openai',
    label: 'OpenAI',
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
  },
]

/** 按域名猜服务商。覆盖预设之外常见家，命中即用。 */
const HOST_HINTS: Array<[RegExp, string]> = [
  [/dashscope\.aliyuncs/, '通义千问'],
  [/deepseek/, 'DeepSeek'],
  [/bigmodel\.cn/, '智谱 GLM'],
  [/moonshot/, '月之暗面 Kimi'],
  [/siliconflow/, '硅基流动'],
  [/openai\.com/, 'OpenAI'],
  [/stepfun/, '阶跃星辰'],
  [/minimaxi?\.com/, 'MiniMax'],
  [/hunyuan\.cloud/, '腾讯混元'],
  [/doubao\.com|volces\.com/, '字节豆包'],
  [/baidubce\.com/, '百度千帆'],
  [/aliyuncs/, '阿里云百炼'],
  [/tokenrhythm\.studio/, '基元律动'],
]

/** 域名认不出时按模型名猜（中转站常见：域名是中转，模型名还带着家族特征）。 */
const MODEL_HINTS: Array<[RegExp, string]> = [
  [/qwen/, '通义千问'],
  [/deepseek/, 'DeepSeek'],
  [/glm/, '智谱 GLM'],
  [/kimi|moonshot/, '月之暗面 Kimi'],
  [/step[-_]/, '阶跃星辰'],
  [/doubao|huala/, '字节豆包'],
  [/hunyuan/, '腾讯混元'],
  [/yi-|01-ai|zeroone/, '零一万物'],
  [/gpt|o[134](-|$)|chatgpt/, 'OpenAI'],
]

/** 按当前配置算面板标题：尽量给出具体服务商，全猜不出才叫通用大模型。 */
export function aiChannelTitle(cfg: AiConfig): string {
  if (cfg.active === 'bicqa') return 'BIC-QA'
  const ch = cfg.openai
  const preset = AI_PRESETS.find((p) => p.baseUrl === ch.baseUrl)
  if (preset) return preset.label
  const base = ch.baseUrl.toLowerCase()
  for (const [re, name] of HOST_HINTS) {
    if (re.test(base)) return name
  }
  const model = ch.model.toLowerCase()
  for (const [re, name] of MODEL_HINTS) {
    if (re.test(model)) return name
  }
  // 模型名本身就是最好的标识（如 step-5-preview），直接亮出来
  return ch.model.trim() || '通用大模型'
}
