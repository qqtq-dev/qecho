import { translate } from '@/i18n'

/**
 * 题目难度五档，与数据库 questions.difficulty 的 1–5 一一对应。
 * 全站显示、筛选、编辑统一用这里的 key，文案放在词条 difficulty.* 里，
 * 跟着界面语言切换，各页面不必各写一套。
 */
export const DIFFICULTY_LEVELS = [
  { value: 1, key: 'difficulty.easy' },
  { value: 2, key: 'difficulty.easier' },
  { value: 3, key: 'difficulty.medium' },
  { value: 4, key: 'difficulty.harder' },
  { value: 5, key: 'difficulty.hard' },
] as const

/** 带「全部难度」的筛选选项，value 0 表示不限。调用方放在 computed 里即可跟随语言变化 */
export function difficultyFilterOptions() {
  return [
    { value: 0, label: translate('difficulty.all') },
    ...DIFFICULTY_LEVELS.map((d) => ({ value: d.value as number, label: translate(d.key) })),
  ]
}

/** 难度值转文案；越界值显示破折号，不硬套某一档 */
export function difficultyLabel(value: number): string {
  const hit = DIFFICULTY_LEVELS.find((d) => d.value === value)
  return hit ? translate(hit.key) : '—'
}
