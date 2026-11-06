import type { MessageSchema } from './schema'

type ZhAntdModule = typeof import('ant-design-vue/es/locale/zh_CN')
export type AntdLocale = ZhAntdModule['default']

/**
 * 语言注册表。**新增一门语言只需要改这里 + 加一个词条文件**，其它代码不用动：
 *
 *   1. 复制 locales/zh-CN.ts 为 locales/xx-XX.ts，按 `satisfies MessageSchema` 翻译；
 *   2. 在本文件 LOCALES 里追加一条，写清 code / 原生名 / 文字方向 / Intl 标签 / dayjs 包名；
 *   3. load() 里把词条、antd 语言包、dayjs 语言包一起 import。
 *
 * 词条与语言包都走动态 import，不打进首屏包；未加载前不会白屏。
 */
export interface LocaleMeta {
  /** BCP 47 标签，也是 i18n 实例里的 locale 值 */
  code: string
  /** 原生名称，语言下拉里直接显示（不随界面语言变化） */
  name: string
  /** 英文名，给以后的命令行/URL 参数做提示用 */
  enName: string
  /** 文字方向，阿拉伯语、希伯来语等从右到左的语言填 rtl */
  dir: 'ltr' | 'rtl'
  /** Intl / toLocaleString 用的标签 */
  intl: string
  /** dayjs 语言包注册名 */
  dayjs: string
  /** 懒加载该语言的词条与组件库语言包 */
  load: () => Promise<{ messages: MessageSchema; antd: AntdLocale }>
}

export const LOCALES: LocaleMeta[] = [
  {
    code: 'zh-CN',
    name: '简体中文',
    enName: 'Simplified Chinese',
    dir: 'ltr',
    intl: 'zh-CN',
    dayjs: 'zh-cn',
    async load() {
      const [messages, antd] = await Promise.all([
        import('./zh-CN'),
        import('ant-design-vue/es/locale/zh_CN'),
        import('dayjs/locale/zh-cn'),
      ])
      return { messages: messages.default, antd: antd.default }
    },
  },
  {
    code: 'en-US',
    name: 'English',
    enName: 'English',
    dir: 'ltr',
    intl: 'en-US',
    dayjs: 'en',
    async load() {
      const [messages, antd] = await Promise.all([
        import('./en-US'),
        import('ant-design-vue/es/locale/en_US'),
        import('dayjs/locale/en'),
      ])
      return { messages: messages.default, antd: antd.default }
    },
  },
]

/** 兜底语言：任何语言缺词条时回落到它，保证界面上不出现原始 key */
export const DEFAULT_LOCALE = 'zh-CN'

export function findLocale(code: string | null | undefined): LocaleMeta | undefined {
  if (!code) return undefined
  const lower = code.toLowerCase()
  return (
    LOCALES.find((l) => l.code.toLowerCase() === lower) ??
    // 允许只传语言部分（en、zh），方便 URL 参数和后期的浏览器语言匹配
    LOCALES.find((l) => l.code.toLowerCase().split('-')[0] === lower.split('-')[0])
  )
}

export type { MessageSchema }
