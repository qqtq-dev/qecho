import { http } from './request'

export interface LoginResult {
  token: string
  userId: number
  username: string
  role: 'admin' | 'user'
}

export interface Profile {
  id: number
  username: string
  role: 'admin' | 'user'
  desktop: boolean
}

export interface Captcha {
  id: string
  question: string
}

export const authApi = {
  login: (username: string, password = '') =>
    http.post<unknown, LoginResult>('/auth/login', { username, password }),
  register: (data: {
    username: string
    password: string
    captchaId: string
    captchaAnswer: string
  }) => http.post<unknown, LoginResult>('/auth/register', data),
  captcha: () => http.get<unknown, Captcha>('/auth/captcha'),
  logout: () => http.post<unknown, void>('/auth/logout'),
  me: () => http.get<unknown, Profile>('/auth/me'),
}
