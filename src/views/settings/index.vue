<template>
  <div class="page">
    <PageHeader :title="$t('settings.title')" :subtitle="$t('settings.subtitle')">
      <template #actions>
        <a-button :loading="loading" @click="loadStatus">{{ $t('settings.recheck') }}</a-button>
      </template>
    </PageHeader>

    <div class="stack">
      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.appearance') }}</h3>
        </div>
        <div class="panel-body">
          <div class="row">
            <div>
              <div class="row-title">{{ $t('settings.darkMode') }}</div>
              <div class="muted">{{ $t('settings.darkModeDesc') }}</div>
            </div>
            <a-switch v-model:checked="dark" />
          </div>
          <div class="row">
            <div>
              <div class="row-title">{{ $t('settings.language') }}</div>
              <div class="muted">{{ $t('settings.languageDesc') }}</div>
            </div>
            <a-select v-model:value="lang" :options="localeOptions" class="lang-select" />
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.security') }}</h3>
        </div>
        <div class="panel-body">
          <div class="row">
            <div>
              <div class="row-title">{{ auth.username }}</div>
              <div class="muted">
                {{ auth.isAdmin ? $t('settings.roleAdmin') : $t('settings.roleUser') }} ·
                {{ $t('settings.pwdUsage') }}
              </div>
            </div>
            <a-button @click="openPasswordModal">{{ $t('settings.changePassword') }}</a-button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.engine') }}</h3>
          <a-space>
            <a-button v-if="auth.isAdmin" size="small" @click="openLogs">{{ $t('settings.diagLogs') }}</a-button>
            <a-tag :color="healthy ? 'green' : 'red'">
              {{ healthy ? $t('settings.healthy') : $t('settings.unhealthy') }}
            </a-tag>
          </a-space>
        </div>
        <div class="panel-body">
          <dl class="kv">
            <div><dt>{{ $t('settings.version') }}</dt><dd>{{ status?.version ?? appStore.health?.version ?? '—' }}</dd></div>
            <div><dt>{{ $t('settings.dataDir') }}</dt><dd class="mono">{{ status?.dataDir ?? '—' }}</dd></div>
            <div><dt>{{ $t('settings.maintenance') }}</dt><dd>{{ status?.maintenance ? $t('settings.on') : $t('settings.off') }}</dd></div>
            <div><dt>{{ $t('settings.apiBase') }}</dt><dd class="mono">{{ apiBase }}</dd></div>
          </dl>
          <p class="note">{{ $t('settings.localNote') }}</p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.practicePanel') }}</h3>
          <a-button v-if="auth.isAdmin" type="primary" :loading="settingsSaving" @click="saveSettings">
            {{ $t('settings.saveSettings') }}
          </a-button>
        </div>
        <div class="panel-body">
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.defaultCount') }}</div>
              <div class="muted">{{ $t('settings.defaultCountDesc') }}</div>
            </div>
            <a-input-number v-model:value="form.practiceCount" :min="5" :max="500" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.shuffleQuestions') }}</div>
              <div class="muted">{{ $t('settings.shuffleQuestionsDesc') }}</div>
            </div>
            <a-switch v-model:checked="form.practiceShuffleQuestions" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.shuffleOptions') }}</div>
              <div class="muted">{{ $t('settings.shuffleOptionsDesc') }}</div>
            </div>
            <a-switch v-model:checked="form.practiceShuffleOptions" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.wrongThreshold') }}</div>
              <div class="muted">{{ $t('settings.wrongThresholdDesc') }}</div>
            </div>
            <a-input-number
              v-model:value="form.wrongMasterThreshold"
              :min="1"
              :max="10"
              :disabled="!auth.isAdmin"
            />
          </div>
          <p v-if="!auth.isAdmin" class="note">{{ $t('settings.adminOnly') }}</p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.ai') }}</h3>
          <a-space>
            <a-button :loading="aiTesting" :disabled="!aiReady" @click="testAi">{{ $t('settings.testConn') }}</a-button>
            <a-button type="primary" :loading="aiSaving" @click="saveAi">{{ $t('common.save') }}</a-button>
          </a-space>
        </div>
        <div class="panel-body">
          <p class="note ai-intro">
            {{ $t('settings.aiIntro') }}
          </p>
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.enable') }}</div>
              <div class="muted">{{ $t('settings.enableDesc') }}</div>
            </div>
            <a-switch v-model:checked="cur.enabled" />
          </div>

          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.provider') }}</div>
              <div class="muted">{{ $t('settings.providerDesc') }}</div>
            </div>
            <a-select
              v-model:value="ai.active"
              class="ai-key-input"
              :options="providerOptions"
              @change="onProviderChange"
            />
          </div>

          <div v-if="ai.active === 'openai'" class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.preset') }}</div>
              <div class="muted">{{ $t('settings.presetDesc') }}</div>
            </div>
            <a-select
              v-model:value="aiPreset"
              class="ai-key-input"
              :options="presetOptions"
              :placeholder="$t('settings.presetPlaceholder')"
              allow-clear
              @change="onPresetChange"
            />
          </div>

          <div class="setting-row ai-key-row">
            <div>
              <div class="row-title">{{ $t('settings.apiKey') }}</div>
              <div class="muted">
                <template v-if="cur.hasKey">{{ $t('settings.keySaved', { mask: cur.keyMask }) }}</template>
                <template v-else>{{ keyHint }}</template>
              </div>
            </div>
            <a-input-password
              v-model:value="ai.apiKey"
              class="ai-key-input"
              :placeholder="cur.hasKey ? cur.keyMask : 'sk-...'"
              autocomplete="off"
            />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.baseUrl') }}</div>
              <div class="muted">
                {{
                  ai.active === 'openai'
                    ? $t('settings.baseUrlDescOpenai')
                    : $t('settings.baseUrlDescBicqa')
                }}
              </div>
            </div>
            <a-input v-model:value="cur.baseUrl" class="ai-key-input" :placeholder="baseUrlPlaceholder" />
          </div>

          <div v-if="ai.active === 'openai'" class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.model') }}</div>
              <div class="muted">{{ $t('settings.modelDesc') }}</div>
            </div>
            <a-input v-model:value="cur.model" class="ai-key-input" placeholder="deepseek-chat" />
          </div>

          <div class="setting-row">
            <div>
              <div class="row-title">{{ $t('settings.timeout') }}</div>
              <div class="muted">{{ $t('settings.timeoutDesc') }}</div>
            </div>
            <a-input-number v-model:value="cur.timeoutSec" :min="30" :max="900" :step="30" />
          </div>
          <div class="ai-actions">
            <a-button v-if="cur.hasKey" danger size="small" @click="clearAiKey">
              {{ $t('settings.clearKey') }}
            </a-button>
            <span v-if="aiState" class="muted ai-state">{{ aiState }}</span>
          </div>
          <p class="note">
            {{ $t('settings.privacyNote') }}
          </p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.closeBehavior') }}</h3>
        </div>
        <div class="panel-body">
          <a-radio-group v-model:value="closeBehavior">
            <a-radio value="minimize">{{ $t('settings.minimizeToTray') }}</a-radio>
            <a-radio value="exit">{{ $t('settings.exitApp') }}</a-radio>
          </a-radio-group>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('settings.about') }}</h3>
        </div>
        <div class="panel-body">
          <dl class="kv">
            <div><dt>{{ $t('settings.app') }}</dt><dd>Qecho</dd></div>
            <div><dt>{{ $t('settings.stack') }}</dt><dd>Tauri 2 · Vue 3 · Go · SQLite</dd></div>
            <div><dt>{{ $t('settings.version') }}</dt><dd>{{ status?.version ?? appStore.health?.version ?? '—' }}</dd></div>
          </dl>
        </div>
      </section>
    </div>

    <a-drawer v-model:open="logsOpen" :title="$t('settings.logsTitle')" :width="680" placement="right">
      <div class="diag">
        <a-descriptions :column="1" size="small" bordered>
          <a-descriptions-item :label="$t('settings.logsEngineVersion')">
            {{ logs?.version ?? status?.version ?? '—' }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('settings.logsPort')">{{ logs?.port ?? '—' }}</a-descriptions-item>
          <a-descriptions-item :label="$t('settings.logsDataDir')">
            {{ logs?.dataDir ?? status?.dataDir ?? '—' }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('settings.logsLineCount')">{{ logs?.lines.length ?? 0 }}</a-descriptions-item>
        </a-descriptions>
        <div class="log-toolbar">
          <span class="muted">{{ $t('settings.logsRecent', { n: logs?.lines.length ?? 0 }, logs?.lines.length ?? 0) }}</span>
          <a-space>
            <a-button size="small" :loading="logsLoading" @click="loadLogs">{{ $t('common.refresh') }}</a-button>
            <a-button size="small" :disabled="!logText" @click="copyLogs">{{ $t('common.copy') }}</a-button>
          </a-space>
        </div>
        <pre class="log-box">{{ logText || $t('settings.logsEmpty') }}</pre>
        <p class="note">{{ $t('settings.logsNote') }}</p>
      </div>
    </a-drawer>

    <a-modal
      v-model:open="pwdOpen"
      :title="$t('settings.pwdTitle')"
      :confirm-loading="pwdSaving"
      :ok-text="$t('common.save')"
      :cancel-text="$t('common.cancel')"
      :width="420"
      @ok="savePassword"
    >
      <a-form layout="vertical">
        <a-form-item :label="$t('settings.pwdCurrent')" :extra="auth.username === 'local' ? $t('settings.pwdCurrentExtra') : undefined">
          <a-input-password v-model:value="pwdForm.oldPassword" autocomplete="current-password" />
        </a-form-item>
        <a-form-item :label="$t('settings.pwdNew')" :extra="$t('settings.pwdNewExtra')">
          <a-input-password v-model:value="pwdForm.newPassword" autocomplete="new-password" />
        </a-form-item>
        <a-form-item :label="$t('settings.pwdConfirm')">
          <a-input-password v-model:value="pwdForm.confirm" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>

  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { authApi } from '@/api/auth'
import { engineApi, type EngineStatus, type SystemLogs } from '@/api/engine'
import { settingsApi, type Settings } from '@/api/settings'
import { aiApi, AI_PRESETS, type AiConfig, type AiProvider } from '@/api/ai'
import { getApiBase } from '@/api/request'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isDark, toggleTheme } from '@/theme'
import { localeCode, localeOptions, setLocale } from '@/i18n'

const { t } = useI18n()

const appStore = useAppStore()
const auth = useAuthStore()
const status = ref<EngineStatus | null>(null)
const loading = ref(false)
const apiBase = getApiBase()
const healthy = computed(() => Boolean(status.value) || appStore.health?.status === 'ok')

const settingsSaving = ref(false)

// —— 智能助手：BIC-QA 与通用大模型两条通道，各自独立保存，同时只生效一条 ——
const DEFAULT_AI_BASE_URL = 'https://api.bic-qa.com'

/** 服务商下拉。选中的那条就是当前生效通道，另一条只是存着不参与请求。 */
const providerOptions = computed(() => [
  { value: 'bicqa', label: t('settings.providerBicqa') },
  { value: 'openai', label: t('settings.providerOpenai') },
])

/** 通用模型服务商预设表在 src/api/ai.ts（AI_PRESETS），答疑面板的标题也用它。 */
const presetOptions = AI_PRESETS.map((p) => ({ value: p.value, label: p.label }))

interface AiChannelForm {
  enabled: boolean
  hasKey: boolean
  keyMask: string
  baseUrl: string
  model: string
  timeoutSec: number
}

function emptyChannel(baseUrl: string): AiChannelForm {
  return { enabled: false, hasKey: false, keyMask: '', baseUrl, model: '', timeoutSec: 300 }
}

const ai = reactive({
  active: 'bicqa' as AiProvider,
  bicqa: emptyChannel(DEFAULT_AI_BASE_URL),
  openai: emptyChannel(''),
  /** 仅当前通道的 Key 输入框。已保存的 Key 从不回填明文。 */
  apiKey: '',
})
const aiPreset = ref('')
const aiSaving = ref(false)
const aiTesting = ref(false)
const aiState = ref('')

/** 当前生效通道的表单，模板统一读它。 */
const cur = computed(() => (ai.active === 'openai' ? ai.openai : ai.bicqa))

/** 填了 Key 才能测连通性；是否真正启用由后端 Ready 再判一次。 */
const aiReady = computed(() => cur.value.hasKey)

const keyHint = computed(() =>
  ai.active === 'openai' ? t('settings.keyHintOpenai') : t('settings.keyHintBicqa'),
)

const baseUrlPlaceholder = computed(() =>
  ai.active === 'openai' ? 'https://api.deepseek.com/v1' : DEFAULT_AI_BASE_URL,
)

const providerLabel = computed(() =>
  ai.active === 'openai' ? t('settings.providerShortOpenai') : t('settings.providerShortBicqa'),
)

function applyAiConfig(cfg: AiConfig) {
  ai.active = cfg.active ?? 'bicqa'
  Object.assign(ai.bicqa, cfg.bicqa)
  Object.assign(ai.openai, cfg.openai)
  ai.apiKey = ''
  aiPreset.value = AI_PRESETS.find((p) => p.baseUrl === cur.value.baseUrl)?.value ?? ''
}

/** 切换生效通道：只换 active，两条通道的配置都原样留着，切回来不用重填。 */
function onProviderChange(value: string) {
  ai.active = value === 'openai' ? 'openai' : 'bicqa'
  ai.apiKey = ''
  aiState.value = ''
  aiPreset.value = AI_PRESETS.find((p) => p.baseUrl === cur.value.baseUrl)?.value ?? ''
}

function onPresetChange(value: string) {
  const preset = AI_PRESETS.find((p) => p.value === value)
  if (!preset) return
  cur.value.baseUrl = preset.baseUrl
  cur.value.model = preset.model
}

async function loadAiConfig() {
  try {
    applyAiConfig(await aiApi.config())
  } catch {
    /* 引擎未就绪时忽略，进入页面后可重试 */
  }
}

async function saveAi() {
  aiSaving.value = true
  aiState.value = ''
  const ch = cur.value
  try {
    const cfg = await aiApi.saveConfig({
      active: ai.active,
      channel: {
        enabled: ch.enabled,
        baseUrl: ch.baseUrl,
        apiKey: ai.apiKey.trim() || undefined,
        model: ch.model,
        timeoutSec: ch.timeoutSec,
      },
    })
    applyAiConfig(cfg)
    aiState.value = ch.enabled ? t('settings.msg.aiSavedOn') : t('settings.msg.aiSavedOff')
    message.success(t('settings.msg.aiSaved'))
  } catch {
    /* 请求层已提示 */
  } finally {
    aiSaving.value = false
  }
}

async function testAi() {
  aiTesting.value = true
  aiState.value = t('settings.msg.testing', { provider: providerLabel.value })
  try {
    const res = await aiApi.test()
    aiState.value = res.model
      ? t('settings.msg.testOkModel', { model: res.model, url: res.baseUrl })
      : t('settings.msg.testOk', { url: res.baseUrl })
    message.success(t('settings.msg.testOkMsg', { provider: providerLabel.value }))
  } catch (err) {
    aiState.value =
      err instanceof Error
        ? t('settings.msg.testFailWith', { msg: err.message })
        : t('settings.msg.testFail')
  } finally {
    aiTesting.value = false
  }
}

async function clearAiKey() {
  aiSaving.value = true
  try {
    applyAiConfig(await aiApi.saveConfig({ channel: { clearKey: true, enabled: false } }))
    aiState.value = t('settings.msg.aiClearState')
    message.success(t('settings.msg.aiCleared'))
  } catch {
    /* 请求层已提示 */
  } finally {
    aiSaving.value = false
  }
}

const form = reactive<Settings>({
  practiceCount: 20,
  practiceShuffleQuestions: true,
  practiceShuffleOptions: false,
  wrongMasterThreshold: 2,
  backupDaily: 7,
  backupPreImport: 5,
  backupPreMigrate: 3,
})

async function loadSettings() {
  try {
    const data = await settingsApi.get()
    Object.assign(form, data)
  } catch {
    /* 引擎尚未就绪时保留默认值 */
  }
}

async function saveSettings() {
  settingsSaving.value = true
  try {
    const data = await settingsApi.update({ ...form })
    Object.assign(form, data)
    message.success(t('settings.msg.saved'))
  } finally {
    settingsSaving.value = false
  }
}

const pwdOpen = ref(false)
const pwdSaving = ref(false)
const pwdForm = reactive({ oldPassword: '', newPassword: '', confirm: '' })

function openPasswordModal() {
  pwdForm.oldPassword = ''
  pwdForm.newPassword = ''
  pwdForm.confirm = ''
  pwdOpen.value = true
}

async function savePassword() {
  if (pwdForm.newPassword.length < 8 || pwdForm.newPassword.length > 72) {
    message.warning(t('settings.msg.pwdTooShort'))
    return
  }
  if (pwdForm.newPassword !== pwdForm.confirm) {
    message.warning(t('settings.msg.pwdMismatch'))
    return
  }
  pwdSaving.value = true
  try {
    await authApi.changePassword(pwdForm.oldPassword, pwdForm.newPassword)
    message.success(t('settings.msg.pwdUpdated'))
    pwdOpen.value = false
  } finally {
    pwdSaving.value = false
  }
}

const dark = computed({
  get: () => isDark.value,
  set: (v: boolean) => {
    if (v !== isDark.value) toggleTheme()
  },
})

/** 语言下拉：选项来自语言注册表，新增语言不用改这里 */
const lang = computed({
  get: () => localeCode.value,
  set: (v: string) => {
    void setLocale(v)
  },
})

const closeBehavior = computed({
  get: () => (localStorage.getItem('quiz-close-behavior') === 'exit' ? 'exit' : 'minimize'),
  set: (v: string) => {
    localStorage.setItem('quiz-close-behavior', v)
    syncCloseBehavior(v)
  },
})

async function loadStatus() {
  loading.value = true
  try {
    status.value = await engineApi.status()
  } catch {
    status.value = null
    message.error(t('settings.msg.statusFailed'))
  } finally {
    loading.value = false
    appStore.refresh()
  }
}

const logsOpen = ref(false)
const logsLoading = ref(false)
const logs = ref<SystemLogs | null>(null)
const logText = computed(() => (logs.value?.lines ?? []).join('\n'))

function openLogs() {
  logsOpen.value = true
  void loadLogs()
}

async function loadLogs() {
  logsLoading.value = true
  try {
    logs.value = await engineApi.logs()
  } finally {
    logsLoading.value = false
  }
}

async function copyLogs() {
  const text = logText.value
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    message.success(t('settings.msg.copied'))
    return
  } catch {
    /* 回退到 execCommand */
  }
  const area = document.createElement('textarea')
  area.value = text
  area.style.position = 'fixed'
  area.style.opacity = '0'
  document.body.appendChild(area)
  area.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(area)
  if (ok) message.success(t('settings.msg.copied'))
  else message.error(t('settings.msg.copyFailed'))
}

async function syncCloseBehavior(behavior: string) {
  if (!(window as unknown as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__) return
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('set_close_behavior', { behavior })
  } catch {
    /* 非 Tauri 环境忽略 */
  }
}

onMounted(() => {
  loadStatus()
  loadSettings()
  loadAiConfig()
  syncCloseBehavior(closeBehavior.value)
})
</script>

<style scoped>
.stack { display: grid; gap: 12px; }
.row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.row + .row { margin-top: 14px; }
.lang-select { width: 168px; flex: none; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.ai-intro { margin: 0 0 8px; }
.ai-key-row { align-items: flex-start; }
.ai-key-input { width: 300px; flex: none; }
.ai-actions { display: flex; align-items: center; gap: 12px; padding: 10px 0 2px; }
.ai-state { font-size: 12.5px; }
.setting-row:last-child { border-bottom: 0; padding-bottom: 0; }
.setting-row:first-child { padding-top: 0; }
.row-title { font-size: 13px; font-weight: 600; margin-bottom: 2px; }
.kv { margin: 0; display: grid; gap: 10px; }
.kv > div { display: flex; justify-content: space-between; gap: 16px; }
.kv dt { color: var(--text2); font-size: 12px; flex: 0 0 auto; }
.kv dd { margin: 0; font-size: 12px; text-align: right; word-break: break-all; }
.note { margin: 14px 0 0; font-size: 11px; color: var(--text2); }
.diag { display: grid; gap: 14px; }
.log-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.log-box {
  margin: 0;
  min-height: 240px;
  max-height: 52vh;
  overflow: auto;
  padding: 12px 14px;
  border-radius: 8px;
  background: #101713;
  color: #cfe8da;
  font-family: Consolas, "Cascadia Mono", monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
