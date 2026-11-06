import { computed, reactive, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import {
  DEFAULT_LOCALE,
  LOCALES,
  findLocale,
  type AntdLocale,
  type LocaleMeta,
  type MessageSchema,
} from './locales'

const STORAGE_KEY = 'quiz-lang'

/**
 * 语言状态。词条按需异步加载，所以初始 messages 为空，由 initI18n() 填第一门语言。
 * legacy: false 用组合式 API；globalInjection 让模板里直接写 $t('...')。
 */
export const i18n = createI18n({
  legacy: false,
  locale: DEFAULT_LOCALE,
  fallbackLocale: DEFAULT_LOCALE,
  messages: {} as Record<string, MessageSchema>,
  missingWarn: false,
  fallbackWarn: false,
  globalInjection: true,
})

/** 已加载的 antd 语言包，随语言切换给 a-config-provider */
const antdLocales = reactive<Record<string, AntdLocale>>({})
const loaded = new Set<string>()

const fallbackMeta = findLocale(DEFAULT_LOCALE) as LocaleMeta
const currentMeta = ref<LocaleMeta>(fallbackMeta)

/** 当前语言元信息：文字方向、Intl 标签等都在这里取，组件里不要写死语言代码 */
export const localeMeta = computed(() => currentMeta.value)
export const localeCode = computed(() => currentMeta.value.code)
/** 给 a-config-provider :locale 用 */
export const antdLocale = computed(() => antdLocales[currentMeta.value.code])
/** 语言下拉选项，直接来自注册表 */
export const localeOptions = LOCALES.map((l) => ({ value: l.code, label: l.name }))

/** 语言检测优先级：URL ?lang= > 上次选择 > 浏览器语言 > 默认 */
function detectLocale(): string {
  const fromUrl = new URLSearchParams(window.location.search).get('lang')
  if (findLocale(fromUrl)) return findLocale(fromUrl)!.code

  const saved = localStorage.getItem(STORAGE_KEY)
  if (findLocale(saved)) return findLocale(saved)!.code

  for (const tag of navigator.languages ?? [navigator.language]) {
    const hit = findLocale(tag)
    if (hit) return hit.code
  }
  return DEFAULT_LOCALE
}

let loading: Promise<void> | null = null

/**
 * 切换语言：加载词条（首次）→ 同步 i18n / antd / dayjs / <html lang|dir> → 记住选择。
 * 之后的语言只要在注册表里加一条，这里不用改。
 */
export async function setLocale(code: string): Promise<void> {
  const meta = findLocale(code) ?? fallbackMeta

  if (!loaded.has(meta.code)) {
    const { messages, antd } = await meta.load()
    i18n.global.setLocaleMessage(meta.code, messages)
    antdLocales[meta.code] = antd
    loaded.add(meta.code)
  }

  i18n.global.locale.value = meta.code
  dayjs.locale(meta.dayjs)
  const root = document.documentElement
  root.lang = meta.code
  root.dir = meta.dir
  currentMeta.value = meta
  try {
    localStorage.setItem(STORAGE_KEY, meta.code)
  } catch {
    /* 隐私模式写不进去时只影响记忆，不影响当前会话 */
  }
}

/** 应用启动时调用一次，返回检测出来的语言代码 */
export async function initI18n(): Promise<string> {
  loading ??= setLocale(detectLocale()).then(() => undefined)
  await loading
  return currentMeta.value.code
}

/**
 * 组件外取词（拦截器、工具函数）。组件内请用 useI18n() 的 t，才有依赖追踪。
 */
export function translate(key: string, named?: Record<string, unknown>): string {
  return named ? i18n.global.t(key, named) : i18n.global.t(key)
}

/**
 * 按当前语言格式化日期，替代写死的 toLocaleString('zh-CN')。
 * 以后新增语言不用再回来改这些调用点。
 */
export function formatDateTime(
  value: string | number | Date,
  options: Intl.DateTimeFormatOptions = { hour12: false },
): string {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(currentMeta.value.intl, options)
}

/**
 * 开发期自检：缺词条、语言包没注册，控制台直接报出来，
 * 免得新加的语言在某个页面悄悄显示成中文。
 */
if (import.meta.env.DEV) {
  const probe = (schema: unknown, path = ''): string[] =>
    Object.entries(schema as Record<string, unknown>).flatMap(([k, v]) =>
      v && typeof v === 'object' ? probe(v, path ? `${path}.${k}` : k) : [`${path ? `${path}.` : ''}${k}`],
    )

  void Promise.all(
    LOCALES.map(async (l) => {
      const { messages } = await l.load()
      const expected = probe((await import('./locales/zh-CN')).default)
      const actual = new Set(probe(messages))
      const missing = expected.filter((k) => !actual.has(k))
      if (missing.length) {
        console.warn(`[i18n] ${l.code} 缺少 ${missing.length} 条：`, missing.slice(0, 10))
      }
    }),
  )
}
