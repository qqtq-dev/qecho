import { http } from './request'

export interface OverviewPractice {
  id: number
  bankName: string
  source: 'bank' | 'wrong'
  answeredCount: number
  totalCount: number
  createdAt: string
}

export interface OverviewExam {
  id: number
  title: string
  totalCount: number
  closeAt: string
  participantCount: number
}

export interface Overview {
  questionCount: number
  todayAnswered: number
  answeredTotal: number
  correctTotal: number
  wrongPending: number
  wrongMastered: number
  unfinishedCount: number
  practices: OverviewPractice[]
  exams: OverviewExam[]
}

export const overviewApi = {
  get: () => http.get<unknown, Overview>('/overview'),
}
