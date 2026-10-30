import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { wrongApi } from '@/api/wrong'
import type { WrongStats } from '@/api/practice'

export const useWrongStore = defineStore('wrong', () => {
  const stats = ref<WrongStats>({ pending: 0, mastered: 0, total: 0 })
  // 练习作答后自增，错题本页面监听它做实时刷新。
  const revision = ref(0)

  const pending = computed(() => stats.value.pending)

  function setStats(next: WrongStats) {
    stats.value = next
    revision.value += 1
  }

  // 列表接口已带回统计时使用，避免触发 revision 造成重复拉取。
  function syncStats(next: WrongStats) {
    stats.value = next
  }

  async function refresh() {
    const next = await wrongApi.stats()
    setStats(next)
    return next
  }

  function touch() {
    revision.value += 1
  }

  return { stats, revision, pending, setStats, syncStats, refresh, touch }
})
