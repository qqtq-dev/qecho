import { http } from './request'

export type BankVisibility = 'private' | 'public'
export type BankLevel = '' | 'viewer' | 'editor' | 'owner' | 'admin'

export interface Bank {
  id: number
  name: string
  description: string
  questionCount: number
  answeredCount: number
  correctRate: number
  lastPracticedAt: string
  ownerId: number | null
  ownerName: string
  isPublic: boolean
  /** private 仅所有者与共享成员可见；public 所有登录用户可读 */
  visibility: BankVisibility
  /** 当前登录者在该题库中的身份 */
  memberLevel: BankLevel
  /** 可增删改题目（所有者、管理员、共享编辑者） */
  canEdit: boolean
  /** 可改题库信息、删除、共享（所有者与管理员） */
  canManage: boolean
  createdAt: string
  updatedAt: string
}

export interface BankMember {
  userId: number
  username: string
  level: 'viewer' | 'editor'
  createdAt: string
}

export interface BankSharing {
  bankId: number
  visibility: BankVisibility
  ownerId: number | null
  ownerName: string
  members: BankMember[]
}

export interface QuestionOption {
  key: string
  content: string
  image: string
}

export interface Question {
  id: number
  bankId: number
  type: 'single' | 'multiple'
  stem: string
  stemImage: string
  answer: string[]
  analysis: string
  source: string
  difficulty: number
  tags: string[]
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
  source: string
  difficulty: number
  tags: string[]
  options: QuestionOption[]
}

export interface Tag {
  id: number
  name: string
  questionCount: number
}

/** 标签改名/合并/删除的影响范围；others 指不属于自己的题库。 */
export interface TagImpact {
  questions: number
  banks: number
  otherQuestions: number
  otherBanks: number
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
  sheets: string[]
  headerRow: number
  headers: string[]
  mapping: Record<string, string>
  columns: ImportColumn[]
  total: number
  valid: number
  duplicate: number
  invalid: number
  sample: ImportPreviewQuestion[]
  errors: { row: number; message: string }[]
  warnings: string[]
  imageUploaded: number
  imageMatched: number
}

export interface ImportOptions {
  sheet?: string
  headerRow?: number
  mapping?: Record<string, string>
}

export interface ImportResult {
  jobId: number
  inserted: number
  skipped: number
  failed: number
  errors: { row: number; message: string }[]
}

export interface ImportJob {
  id: number
  sourceName: string
  format: string
  total: number
  inserted: number
  skipped: number
  failed: number
  status: string
  duplicatePolicy: 'skip' | 'keep' | string
  createdAt: string
  revertedAt: string
  remaining: number
  revertible: number
  canRevert: boolean
}

export interface QuestionQuery {
  page?: number
  size?: number
  keyword?: string
  type?: '' | 'single' | 'multiple'
  tag?: string
  difficulty?: number
}

export const banksApi = {
  list: () => http.get<unknown, { items: Bank[] }>('/banks'),
  get: (id: number) => http.get<unknown, Bank>(`/banks/${id}`),
  create: (data: { name: string; description: string; public?: boolean }) =>
    http.post<unknown, Bank>('/banks', data),
  update: (id: number, data: { name: string; description: string }) =>
    http.put<unknown, Bank>(`/banks/${id}`, data),
  remove: (id: number) => http.delete<unknown, void>(`/banks/${id}`),
  sharing: (id: number) => http.get<unknown, BankSharing>(`/banks/${id}/sharing`),
  updateSharing: (id: number, visibility: BankVisibility) =>
    http.put<unknown, BankSharing>(`/banks/${id}/sharing`, { visibility }),
  saveMember: (id: number, userId: number, level: 'viewer' | 'editor') =>
    http.put<unknown, BankSharing>(`/banks/${id}/members`, { userId, level }),
  removeMember: (id: number, userId: number) =>
    http.delete<unknown, BankSharing>(`/banks/${id}/members/${userId}`),
  questions: (id: number, query: QuestionQuery = {}) =>
    http.get<unknown, PageResult<Question>>(`/banks/${id}/questions`, { params: query }),
  createQuestion: (id: number, data: QuestionInput) =>
    http.post<unknown, Question>(`/banks/${id}/questions`, data),
  getQuestion: (qid: number) => http.get<unknown, Question>(`/questions/${qid}`),
  updateQuestion: (qid: number, data: QuestionInput) =>
    http.put<unknown, Question>(`/questions/${qid}`, data),
  removeQuestion: (qid: number) => http.delete<unknown, void>(`/questions/${qid}`),
  bankTags: (id: number) => http.get<unknown, { items: Tag[] }>(`/banks/${id}/tags`),
  tags: () => http.get<unknown, { items: Tag[] }>('/tags'),
  renameTag: (id: number, name: string) => http.put<unknown, void>(`/tags/${id}`, { name }),
  tagImpact: (id: number) => http.get<unknown, TagImpact>(`/tags/${id}/impact`),
  mergeTag: (id: number, targetId: number) =>
    http.post<unknown, void>(`/tags/${id}/merge`, { targetId }),
  removeTag: (id: number) => http.delete<unknown, void>(`/tags/${id}`),
  importPreview: (id: number, file: File, options?: ImportOptions, images: File[] = []) => {
    const form = new FormData()
    form.append('file', file)
    for (const image of images) form.append('images', image)
    if (options && (options.sheet || options.headerRow || options.mapping)) {
      form.append('options', JSON.stringify(options))
    }
    return http.post<unknown, ImportPreview>(`/banks/${id}/imports/preview`, form, {
      timeout: 300000,
    })
  },
  importCommit: (id: number, token: string, policy: 'skip' | 'keep' = 'skip') =>
    http.post<unknown, ImportResult>(`/banks/${id}/imports`, { token, policy }),
  importJobs: (id: number, limit = 20) =>
    http.get<unknown, { items: ImportJob[] }>(`/banks/${id}/imports`, { params: { limit } }),
  importErrorsCsv: (jobId: number) =>
    http.get<unknown, Blob>(`/imports/${jobId}/errors`, {
      params: { format: 'csv' },
      responseType: 'blob',
    }),
  revertImport: (jobId: number) =>
    http.post<unknown, { reverted: number; kept: number }>(`/imports/${jobId}/revert`),
  exportTo: (id: number, path: string) =>
    http.post<unknown, { path: string; count: number; name: string }>(`/banks/${id}/export`, {
      path,
    }),
  exportFile: (id: number) => http.get<unknown, Blob>(`/banks/${id}/export`, { responseType: 'blob' }),
  exportImagesTo: (id: number, path: string) =>
    http.post<unknown, { path: string; count: number }>(`/banks/${id}/export-images`, { path }),
  exportImagesFile: (id: number) =>
    http.get<unknown, Blob>(`/banks/${id}/export-images`, { responseType: 'blob' }),
  importTemplateTo: (path: string) =>
    http.post<unknown, { path: string }>('/import-template', { path }),
  importTemplateFile: () =>
    http.get<unknown, Blob>('/import-template', { responseType: 'blob' }),
}
