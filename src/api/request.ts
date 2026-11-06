import axios, { type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { message } from 'ant-design-vue'
import { translate } from '@/i18n'

declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}

export interface ApiEnvelope<T = unknown> {
  code: number
  message: string
  data: T
}

const TOKEN_KEY = 'quiz-token'
let apiBase = '/api/v1'

export function isTauri(): boolean {
  return !!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
}

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string): void {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export function getApiBase(): string {
  return apiBase
}

export function resolveImageUrl(name?: null | string): string {
  if (!name) return ''
  if (/^https?:\/\//.test(name)) return name
  return `${apiBase}/images/${encodeURIComponent(name)}`
}

/**
 * - Web 开发：走 Vite 代理（/api → 127.0.0.1:54310）
 * - Tauri：从壳层读取 sidecar 动态端口
 */
export async function initApiBase(): Promise<void> {
  if (!isTauri()) return
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    apiBase = await invoke<string>('engine_base_url')
  } catch {
    apiBase = 'http://127.0.0.1:54310/api/v1'
  }
}

export const http = axios.create({ timeout: 15000 })

http.interceptors.request.use((config) => {
  const url = config.url ?? ''
  if (!/^https?:\/\//.test(url)) {
    config.url = apiBase + url
  }
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) config.headers['X-Token'] = token
  return config
})

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const unwrap = (resp: AxiosResponse): any => {
  const body = resp.data as ApiEnvelope
  if (body && typeof body.code === 'number' && body.code !== 0) {
    if (resp.config.silent !== true) message.error(body.message || translate('error.requestFailed'))
    return Promise.reject(new Error(body.message))
  }
  return body && body.data !== undefined ? body.data : body
}

async function silentLogin(): Promise<string> {
  const resp = await axios.post<ApiEnvelope<{ token: string }>>(
    `${apiBase}/auth/login`,
    { username: 'local', password: '' },
    { timeout: 5000 },
  )
  const token = resp.data?.data?.token
  if (!token) throw new Error(translate('error.localLoginFailed'))
  localStorage.setItem(TOKEN_KEY, token)
  return token
}

http.interceptors.response.use(
  unwrap as unknown as (value: AxiosResponse) => AxiosResponse | Promise<AxiosResponse>,
  async (error) => {
    const resp = error?.response
    const silent = error?.config?.silent === true
    const apiMessage: string | undefined = resp?.data?.message

    if (resp?.status === 401) {
      const url: string = resp?.config?.url ?? ''
      if (url.includes('/auth/login') || url.includes('/auth/register')) {
        if (!silent) message.error(apiMessage || translate('error.loginFailed'))
      } else if (!isTauri()) {
        setToken('')
        if (!location.hash.startsWith('#/login')) location.hash = '#/login'
        if (!silent) message.error(apiMessage || translate('error.sessionExpired'))
      } else {
        const cfg = error.config as (InternalAxiosRequestConfig & { __retried?: boolean }) | undefined
        if (cfg && !cfg.__retried) {
          cfg.__retried = true
          try {
            const token = await silentLogin()
            cfg.headers = cfg.headers ?? {}
            cfg.headers['X-Token'] = token
            return http.request(cfg)
          } catch {
            localStorage.removeItem(TOKEN_KEY)
          }
        }
        if (!silent) message.error(translate('error.engineDown'))
      }
    } else if (!silent) {
      message.error(apiMessage || error?.message || translate('error.network'))
    }
    return Promise.reject(error)
  },
)
