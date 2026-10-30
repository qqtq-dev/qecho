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

export const engineApi = {
  health: () => http.get<unknown, EngineHealth>('/health'),
  status: () => http.get<unknown, EngineStatus>('/system/status'),
}
