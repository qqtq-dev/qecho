import { http, getApiBase, getToken } from './request'

/** 接入方式：bicqa = 官方 Open API（API Key）；imaweb = ima 网页版（登录 Cookie）。 */
export type AiProvider = 'bicqa' | 'imaweb'

export interface AiConfig {
  provider: AiProvider
  enabled: boolean
  hasKey: boolean
  keyMask: string
  hasCookie: boolean
  cookieMask: string
  baseUrl: string
  model: string
  timeoutSec: number
  ready: boolean
}

export interface AiTestResult {
  ok: boolean
  provider: AiProvider
  conversationId: string
  baseUrl: string
}

export interface AiConfigInput {
  provider?: AiProvider
  enabled?: boolean
  baseUrl?: string
  apiKey?: string
  model?: string
  timeoutSec?: number
  /** ima 网页版登录态；留空表示沿用已保存的 Cookie。 */
  cookie?: string
  clearKey?: boolean
  clearCookie?: boolean
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
