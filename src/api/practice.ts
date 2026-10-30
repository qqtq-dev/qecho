import { http } from './request'

export interface WrongStats {
  pending: number
  mastered: number
  total: number
}

export interface PracticeOption {
  key: string
  label: string
  content: string
  image: string
}

export interface PracticeItem {
  id: number
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  difficulty: number
  options: PracticeOption[]
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
  status: 'ongoing' | 'finished'
  lastSeq: number
  durationSec: number
  shuffleOptions: boolean
  createdAt: string
  updatedAt: string
}

export interface PracticeStart {
  session: PracticeSession
  questions: PracticeItem[]
  truncated: boolean
}

export interface PracticeAnswerResult {
  isCorrect: boolean
  answer: string[]
  analysis: string
  wrongCount: number
  correctStreak: number
  mastered: boolean
  timeSpentMs: number
  durationSec: number
  stats: WrongStats
}

export interface PracticeAnswerState {
  questionId: number
  chosen: string[]
  isCorrect: boolean
  answer: string[]
  analysis: string
  wrongCount: number
  correctStreak: number
  mastered: boolean
  timeSpentMs: number
  answeredAt: string
}

export interface PracticeDetail {
  session: PracticeSession
  questions: PracticeItem[]
  answers: PracticeAnswerState[]
  marked: number[]
  lastSeq: number
  stats: WrongStats
}

export interface PracticeSummary {
  sessionId: number
  totalCount: number
  answeredCount: number
  correctCount: number
  durationSec: number
  status: 'finished'
}

export interface PracticeStartInput {
  bankIds?: number[]
  questionIds?: number[]
  count: number
  order: 'random' | 'fixed'
  source?: 'bank' | 'wrong'
  questionType?: 'single' | 'multiple'
  difficulty?: number
  shuffleOptions?: boolean
}

export const practiceApi = {
  start: (data: PracticeStartInput) =>
    http.post<unknown, PracticeStart>('/practice/sessions', data),
  sessions: () => http.get<unknown, { items: PracticeSession[] }>('/practice/sessions'),
  detail: (sessionId: number) =>
    http.get<unknown, PracticeDetail>(`/practice/sessions/${sessionId}`),
  answer: (sessionId: number, questionId: number, chosen: string[], timeSpentMs: number) =>
    http.post<unknown, PracticeAnswerResult>(`/practice/sessions/${sessionId}/answers`, {
      questionId,
      chosen,
      timeSpentMs,
    }),
  position: (sessionId: number, seq: number, marked: number[]) =>
    http.post<unknown, null>(`/practice/sessions/${sessionId}/position`, { seq, marked }),
  submit: (sessionId: number) =>
    http.post<unknown, PracticeSummary>(`/practice/sessions/${sessionId}/submit`),
  remove: (sessionId: number) => http.delete<unknown, null>(`/practice/sessions/${sessionId}`),
}
