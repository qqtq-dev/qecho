import { http } from './request'

export interface UpdateInfo {
  ok: boolean
  current: string
  latest: string
  hasUpdate: boolean
  /** 发布方标记为重要更新时，前端用更醒目的样式 */
  force: boolean
  notes: string
  publishedAt: string
  url: string
  downloadUrl: string
  checkedAt: string
  message?: string
}

export const updateApi = {
  /** force 为 true 时绕过引擎缓存，用户主动点开铃铛时用 */
  check: (force = false) =>
    http.get<unknown, UpdateInfo>(`/update/check${force ? '?force=1' : ''}`, { silent: true }),
}
