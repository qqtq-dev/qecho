import { http } from './request'

export interface UserItem {
  id: number
  username: string
  role: 'admin' | 'user'
  createdAt: string
  lastLoginAt: string
  canCreateBank: boolean
  canExportImport: boolean
}

export interface UserBrief {
  id: number
  username: string
  role: 'admin' | 'user'
}

export const usersApi = {
  list: () => http.get<unknown, { items: UserItem[] }>('/users'),
  search: (keyword: string) =>
    http.get<unknown, { items: UserBrief[] }>('/users/search', { params: { keyword } }),
  updateRole: (id: number, role: 'admin' | 'user') =>
    http.put<unknown, void>(`/users/${id}/role`, { role }),
  updatePermissions: (id: number, data: { canCreateBank: boolean; canExportImport: boolean }) =>
    http.put<unknown, void>(`/users/${id}/permissions`, data),
  resetPassword: (id: number, password: string) =>
    http.post<unknown, void>(`/users/${id}/password`, { password }),
  remove: (id: number) => http.delete<unknown, void>(`/users/${id}`),
}
