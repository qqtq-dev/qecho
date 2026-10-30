import { http } from './request'
import type { QuestionOption } from './banks'
import type { WrongStats } from './practice'

export interface WrongItem {
  questionId: number
  bankId: number
  bankName: string
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  answer: string[]
  analysis: string
  difficulty: number
  options: QuestionOption[]
  wrongCount: number
  correctStreak: number
  mastered: boolean
  lastWrongAt: string
  updatedAt: string
}

export const wrongApi = {
  list: (params: { page?: number; size?: number; status?: 'all' | 'pending' | 'mastered' }) =>
    http.get<unknown, { items: WrongItem[]; total: number; stats: WrongStats }>('/wrong', {
      params,
    }),
  stats: () => http.get<unknown, WrongStats>('/wrong/stats'),
  remove: (questionId: number) => http.delete<unknown, void>(`/wrong/${questionId}`),
}
