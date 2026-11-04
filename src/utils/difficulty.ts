/**
 * 题目难度五档，与数据库 questions.difficulty 的 1–5 一一对应。
 * 全站显示、筛选、编辑统一用这里的文案，避免各页面各写一套。
 */
export const DIFFICULTY_LEVELS = [
  { value: 1, label: '简单' },
  { value: 2, label: '较易' },
  { value: 3, label: '中等' },
  { value: 4, label: '较难' },
  { value: 5, label: '困难' },
] as const

/** 带“全部难度”的筛选选项，value 0 表示不限 */
export const DIFFICULTY_FILTER_OPTIONS = [
  { value: 0, label: '全部难度' },
  ...DIFFICULTY_LEVELS.map((d) => ({ value: d.value as number, label: d.label })),
]

/** 难度值转文案；越界值显示破折号，不硬套某一档 */
export function difficultyLabel(value: number): string {
  const hit = DIFFICULTY_LEVELS.find((d) => d.value === value)
  return hit ? hit.label : '—'
}
