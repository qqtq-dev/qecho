import { http } from './request'

export interface StatsTrendPoint {
  date: string
  answered: number
  correct: number
}

export interface StatsBank {
  bankId: number
  bankName: string
  answered: number
  correct: number
}

export interface StatsTag {
  tagId: number
  tagName: string
  answered: number
  correct: number
}

export interface Stats {
  range: '7d' | '30d' | 'all'
  answeredTotal: number
  correctTotal: number
  practiceCount: number
  examCount: number
  examScoredCount: number
  examAverageScore: number
  trend: StatsTrendPoint[]
  banks: StatsBank[]
}

export const statsApi = {
  get: (range: string) => http.get<unknown, Stats>('/stats', { params: { range } }),
  tags: (range: string) =>
    http.get<unknown, { items: StatsTag[] }>('/stats/tags', { params: { range } }),
}
