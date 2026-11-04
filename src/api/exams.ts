import { http } from './request'

export interface Exam {
  id: number
  title: string
  bankId: number
  bankName: string
  totalCount: number
  timeLimitSec: number
  openAt: string
  closeAt: string
  status: 'published' | 'closed'
  token: string
  shuffleQuestions: boolean
  shuffleOptions: boolean
  participantCount: number
  submittedCount: number
  avgScore: number
  createdAt: string
}

export interface ExamCreateInput {
  title: string
  bankId: number
  count: number
  timeLimitSec: number
  openAt: string
  closeAt: string
  password: string
  shuffleQuestions: boolean
  shuffleOptions: boolean
}

export interface AttemptSummary {
  id: number
  name: string
  studentNo: string
  status: 'ongoing' | 'submitted'
  score: number | null
  startedAt: string
  submittedAt: string
  deadlineAt: string
  answeredCount: number
  totalCount: number
}

export interface ShareInfo {
  port: number
  lanAddresses: string[]
  publicBaseUrl: string
  linkPrefix: string
}

export const examsApi = {
  list: () => http.get<unknown, { items: Exam[] }>('/exams'),
  create: (data: ExamCreateInput) => http.post<unknown, Exam>('/exams', data),
  get: (id: number) => http.get<unknown, Exam>(`/exams/${id}`),
  close: (id: number) => http.post<unknown, Exam>(`/exams/${id}/close`),
  remove: (id: number) => http.delete<unknown, void>(`/exams/${id}`),
  attempts: (id: number) =>
    http.get<unknown, { items: AttemptSummary[] }>(`/exams/${id}/attempts`),
  exportAttemptsTo: (id: number, path: string) =>
    http.post<unknown, { path: string; count: number; title: string }>(
      `/exams/${id}/attempts/export`,
      { path },
    ),
  exportAttemptsFile: (id: number) =>
    http.get<unknown, Blob>(`/exams/${id}/attempts/export`, { responseType: 'blob' }),
  share: () => http.get<unknown, ShareInfo>('/share'),
  updateShare: (publicBaseUrl: string) =>
    http.put<unknown, { publicBaseUrl: string }>('/share', { publicBaseUrl }),
}
