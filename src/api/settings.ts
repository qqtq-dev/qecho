import { http } from './request'

export interface Settings {
  practiceCount: number
  practiceShuffleQuestions: boolean
  practiceShuffleOptions: boolean
  wrongMasterThreshold: number
  backupDaily: number
  backupPreImport: number
  backupPreMigrate: number
}

export const settingsApi = {
  get: () => http.get<unknown, Settings>('/settings'),
  update: (data: Settings) => http.put<unknown, Settings>('/settings', data),
}
