import { http } from './request'

export interface Notice {
  id: number
  kind: string
  text: string
  bankId: number | null
  createdAt: string
  read: boolean
}

export const noticesApi = {
  list: () => http.get<unknown, { items: Notice[]; unread: number }>('/notices'),
  readAll: () => http.post<unknown, void>('/notices/read'),
}
