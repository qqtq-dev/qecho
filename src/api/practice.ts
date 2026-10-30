import { http } from './request'
import type { QuestionOption } from './banks'

export interface WrongStats {
  pending: number
  mastered: number
  total: number
}

export interface PracticeItem {
  id: number
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  difficulty: number
  options: QuestionOption[]
}

export interface PracticeSession {
  id: number
  bankId: number
  bankName: string
  source: 'bank' | 'wrong'
  totalCount: number
  order: 'random' | 'fixed'
  answeredCount: number
  correctCount: number
  createdAt: string
}

export interface PracticeStart {
  session: PracticeSession
  questions: PracticeItem[]
}

export interface PracticeAnswerResult {
  isCorrect: boolean
  answer: string[]
  analysis: string
  wrongCount: number
  correctStreak: number
  mastered: boolean
  stats: WrongStats
}

export interface PracticeStartInput {
  bankId?: number
  count: number
  order: 'random' | 'fixed'
  source?: 'bank' | 'wrong'
  questionType?: 'single' | 'multiple'
}

export const practiceApi = {
  start: (data: PracticeStartInput) =>
    http.post<unknown, PracticeStart>('/practice/sessions', data),
  sessions: () => http.get<unknown, { items: PracticeSession[] }>('/practice/sessions'),
  answer: (sessionId: number, questionId: number, chosen: string[]) =>
    http.post<unknown, PracticeAnswerResult>(`/practice/sessions/${sessionId}/answers`, {
      questionId,
      chosen,
    }),
  remove: (sessionId: number) => http.delete<unknown, null>(`/practice/sessions/${sessionId}`),
}
