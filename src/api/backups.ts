import { http } from './request'

export type BackupKind = 'manual' | 'daily' | 'pre-import' | 'pre-migrate' | 'pre-restore'

export interface BackupItem {
  name: string
  kind: BackupKind
  size: number
  createdAt: string
  schemaVersion: number
  auto: boolean
}

export interface BackupOverview {
  items: BackupItem[]
  totalSize: number
  autoSize: number
  manualSize: number
  capBytes: number
  retention: Record<string, number>
  backupsDir: string
  pendingRestore: boolean
}

export const backupsApi = {
  list: () => http.get<unknown, BackupOverview>('/backups'),
  create: () => http.post<unknown, BackupItem>('/backups'),
  remove: (name: string) => http.delete<unknown, null>('/backups', { params: { name } }),
  restore: (name: string) => http.post<unknown, { pending: boolean }>('/backups/restore', { name }),
  cancelRestore: () => http.delete<unknown, null>('/backups/restore'),
}
