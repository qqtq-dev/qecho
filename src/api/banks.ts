import { http } from './request'

export interface Bank {
  id: number
  name: string
  description: string
  questionCount: number
  ownerId: number | null
  ownerName: string
  isPublic: boolean
  createdAt: string
  updatedAt: string
}

export interface QuestionOption {
  key: string
  content: string
}

export interface Question {
  id: number
  bankId: number
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  answer: string[]
  analysis: string
  difficulty: number
  options: QuestionOption[]
  createdAt: string
  updatedAt: string
}

export interface QuestionInput {
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  answer: string[]
  analysis: string
  difficulty: number
  options: QuestionOption[]
}

export interface PageResult<T> {
  items: T[]
  total: number
}

export interface ImportColumn {
  field: string
  header: string
}

export interface ImportPreviewQuestion {
  row: number
  type: 'single' | 'multiple'
  stem: string
  options: QuestionOption[]
  answer: string[]
}

export interface ImportPreview {
  token: string
  fileName: string
  format: string
  sheet: string
  columns: ImportColumn[]
  total: number
  valid: number
  duplicate: number
  invalid: number
  sample: ImportPreviewQuestion[]
  errors: { row: number; message: string }[]
  warnings: string[]
}

export interface ImportResult {
  jobId: number
  inserted: number
  skipped: number
  failed: number
  errors: { row: number; message: string }[]
}

export const banksApi = {
  list: () => http.get<unknown, { items: Bank[] }>('/banks'),
  get: (id: number) => http.get<unknown, Bank>(`/banks/${id}`),
  create: (data: { name: string; description: string; public?: boolean }) =>
    http.post<unknown, Bank>('/banks', data),
  update: (id: number, data: { name: string; description: string }) =>
    http.put<unknown, Bank>(`/banks/${id}`, data),
  remove: (id: number) => http.delete<unknown, void>(`/banks/${id}`),
  questions: (id: number, page: number, size: number) =>
    http.get<unknown, PageResult<Question>>(`/banks/${id}/questions`, { params: { page, size } }),
  createQuestion: (id: number, data: QuestionInput) =>
    http.post<unknown, Question>(`/banks/${id}/questions`, data),
  updateQuestion: (qid: number, data: QuestionInput) =>
    http.put<unknown, Question>(`/questions/${qid}`, data),
  removeQuestion: (qid: number) => http.delete<unknown, void>(`/questions/${qid}`),
  importPreview: (id: number, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<unknown, ImportPreview>(`/banks/${id}/imports/preview`, form, {
      timeout: 120000,
    })
  },
  importCommit: (id: number, token: string) =>
    http.post<unknown, ImportResult>(`/banks/${id}/imports`, { token }),
  revertImport: (jobId: number) =>
    http.post<unknown, { reverted: number }>(`/imports/${jobId}/revert`),
  exportTo: (id: number, path: string) =>
    http.post<unknown, { path: string; count: number; name: string }>(`/banks/${id}/export`, {
      path,
    }),
  exportFile: (id: number) => http.get<unknown, Blob>(`/banks/${id}/export`, { responseType: 'blob' }),
}
