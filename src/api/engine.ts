import { http } from './request'

export interface EngineHealth {
  app?: string
  status: string
  sqlite: boolean
  version: string
}

export interface EngineStatus {
  version: string
  dataDir: string
  maintenance: boolean
}

export interface SystemLogs {
  version: string
  dataDir: string
  port: number
  lines: string[]
}

export const engineApi = {
  health: () => http.get<unknown, EngineHealth>('/health'),
  status: () => http.get<unknown, EngineStatus>('/system/status'),
  logs: () => http.get<unknown, SystemLogs>('/system/logs'),
}
