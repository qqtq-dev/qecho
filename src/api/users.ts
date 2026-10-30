import { http } from './request'

export interface UserItem {
  id: number
  username: string
  role: 'admin' | 'user'
  createdAt: string
  lastLoginAt: string
}

export const usersApi = {
  list: () => http.get<unknown, { items: UserItem[] }>('/users'),
  updateRole: (id: number, role: 'admin' | 'user') =>
    http.put<unknown, void>(`/users/${id}/role`, { role }),
  resetPassword: (id: number, password: string) =>
    http.post<unknown, void>(`/users/${id}/password`, { password }),
}
