import type zhCN from './zh-CN'

/** 把中文原文的字面量类型放宽成 string，英文词条才不会被要求等于「保存」 */
type DeepString<T> = {
  [K in keyof T]: T[K] extends string ? string : DeepString<T[K]>
}

/**
 * 词条结构的唯一标准：由 zh-CN 推导。
 * 新增语言用 `satisfies MessageSchema` 约束，漏翻或拼错 key 会直接编译报错。
 */
export type MessageSchema = DeepString<typeof zhCN>
