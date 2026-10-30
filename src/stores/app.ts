import { defineStore } from 'pinia'
import { ref } from 'vue'
import { engineApi, type EngineHealth } from '@/api/engine'

export const useAppStore = defineStore('app', () => {
  const health = ref<EngineHealth | null>(null)

  async function refresh() {
    try {
      health.value = await engineApi.health()
    } catch {
      health.value = null
    }
  }

  return { health, refresh }
})
