import { ref, watchEffect } from 'vue'
import { theme as antTheme } from 'ant-design-vue'

const KEY = 'quiz-theme'

const saved = localStorage.getItem(KEY)
const isDark = ref(saved === 'dark')

watchEffect(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem(KEY, isDark.value ? 'dark' : 'light')
})

export function toggleTheme() {
  document.documentElement.classList.add('theme-switching')
  isDark.value = !isDark.value
  window.setTimeout(() => document.documentElement.classList.remove('theme-switching'), 200)
}

export function antdAlgorithm() {
  return isDark.value ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm
}

export { isDark }
