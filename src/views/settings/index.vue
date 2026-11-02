<template>
  <div class="page">
    <PageHeader title="设置" subtitle="外观、账号安全、本地引擎、练习设置与 AI 答疑">
      <template #actions>
        <a-button :loading="loading" @click="loadStatus">重新检测</a-button>
      </template>
    </PageHeader>

    <div class="stack">
      <section class="panel">
        <div class="panel-head">
          <h3>外观</h3>
        </div>
        <div class="panel-body">
          <div class="row">
            <div>
              <div class="row-title">深色模式</div>
              <div class="muted">切换后立即生效，并记住选择</div>
            </div>
            <a-switch v-model:checked="dark" />
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>账号安全</h3>
        </div>
        <div class="panel-body">
          <div class="row">
            <div>
              <div class="row-title">{{ auth.username }}</div>
              <div class="muted">
                {{ auth.isAdmin ? '管理员账号' : '普通账号' }} · 密码用于网页版登录与重新登录
              </div>
            </div>
            <a-button @click="openPasswordModal">修改密码</a-button>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>本地引擎</h3>
          <a-space>
            <a-button v-if="auth.isAdmin" size="small" @click="openLogs">诊断日志</a-button>
            <a-tag :color="healthy ? 'green' : 'red'">{{ healthy ? '正常' : '异常' }}</a-tag>
          </a-space>
        </div>
        <div class="panel-body">
          <dl class="kv">
            <div><dt>版本</dt><dd>{{ status?.version ?? appStore.health?.version ?? '—' }}</dd></div>
            <div><dt>数据目录</dt><dd class="mono">{{ status?.dataDir ?? '—' }}</dd></div>
            <div><dt>维护模式</dt><dd>{{ status?.maintenance ? '开启' : '关闭' }}</dd></div>
            <div><dt>接口地址</dt><dd class="mono">{{ apiBase }}</dd></div>
          </dl>
          <p class="note">数据全部保存在本机，不上传、不联网。</p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>练习与复习</h3>
          <a-button v-if="auth.isAdmin" type="primary" :loading="settingsSaving" @click="saveSettings">
            保存设置
          </a-button>
        </div>
        <div class="panel-body">
          <div class="setting-row">
            <div>
              <div class="row-title">默认题量</div>
              <div class="muted">打开练习页时预填的题目数量</div>
            </div>
            <a-input-number v-model:value="form.practiceCount" :min="5" :max="500" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">默认随机题目顺序</div>
              <div class="muted">关闭后默认按题库题序练习</div>
            </div>
            <a-switch v-model:checked="form.practiceShuffleQuestions" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">默认选项乱序</div>
              <div class="muted">每组练习内选项顺序固定，答案按选项内容判定</div>
            </div>
            <a-switch v-model:checked="form.practiceShuffleOptions" :disabled="!auth.isAdmin" />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">错题掌握阈值</div>
              <div class="muted">连续答对多少次后移出错题本（1–10 次）</div>
            </div>
            <a-input-number
              v-model:value="form.wrongMasterThreshold"
              :min="1"
              :max="10"
              :disabled="!auth.isAdmin"
            />
          </div>
          <p v-if="!auth.isAdmin" class="note">练习设置由管理员统一配置。</p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>智能助手</h3>
          <a-space>
            <a-button :loading="aiTesting" :disabled="!aiReady" @click="testAi">测试连接</a-button>
            <a-button type="primary" :loading="aiSaving" @click="saveAi">保存</a-button>
          </a-space>
        </div>
        <div class="panel-body">
          <p class="note ai-intro">
            接入后练习作答完可在题目旁直接追问知识点。每个账号使用自己的凭据，互不影响；
            未启用时不会发起任何外部请求。
          </p>
          <div class="setting-row">
            <div>
              <div class="row-title">启用</div>
              <div class="muted">关闭后练习页不显示答疑入口</div>
            </div>
            <a-switch v-model:checked="ai.enabled" />
          </div>

          <div class="setting-row ai-key-row">
            <div>
              <div class="row-title">API Key</div>
              <div class="muted">
                <template v-if="ai.hasKey">已保存 {{ ai.keyMask }}，留空表示不修改</template>
                <template v-else>sk- 开头，在 BIC-QA 个人中心申请</template>
              </div>
            </div>
            <a-input-password
              v-model:value="ai.apiKey"
              class="ai-key-input"
              :placeholder="ai.hasKey ? ai.keyMask : 'sk-...'"
              autocomplete="off"
            />
          </div>
          <div class="setting-row">
            <div>
              <div class="row-title">服务地址</div>
              <div class="muted">默认官方接口，一般无需修改</div>
            </div>
            <a-input v-model:value="ai.baseUrl" class="ai-key-input" placeholder="https://api.bic-qa.com" />
          </div>

          <div class="setting-row">
            <div>
              <div class="row-title">回答超时</div>
              <div class="muted">流式回答的最长等待时间（30–900 秒）</div>
            </div>
            <a-input-number v-model:value="ai.timeoutSec" :min="30" :max="900" :step="30" />
          </div>
          <div class="ai-actions">
            <a-button v-if="ai.hasKey" danger size="small" @click="clearAiKey">
              清除已保存的 Key
            </a-button>
            <span v-if="aiState" class="muted ai-state">{{ aiState }}</span>
          </div>
          <p class="note">
            你的凭据只对本人账号生效，保存在本机数据目录的 bicqa 文件夹，不会写入数据库、不会随备份导出，
            也不会回传到界面；提问内容只包含题干与选项，不包含答案与解析。
          </p>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>关闭窗口时</h3>
        </div>
        <div class="panel-body">
          <a-radio-group v-model:value="closeBehavior">
            <a-radio value="minimize">最小化到托盘</a-radio>
            <a-radio value="exit">退出程序</a-radio>
          </a-radio-group>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>关于</h3>
        </div>
        <div class="panel-body">
          <dl class="kv">
            <div><dt>应用</dt><dd>Qecho</dd></div>
            <div><dt>技术栈</dt><dd>Tauri 2 · Vue 3 · Go · SQLite</dd></div>
            <div><dt>版本</dt><dd>{{ status?.version ?? appStore.health?.version ?? '—' }}</dd></div>
          </dl>
        </div>
      </section>
    </div>

    <a-drawer v-model:open="logsOpen" title="诊断日志" :width="680" placement="right">
      <div class="diag">
        <a-descriptions :column="1" size="small" bordered>
          <a-descriptions-item label="引擎版本">
            {{ logs?.version ?? status?.version ?? '—' }}
          </a-descriptions-item>
          <a-descriptions-item label="监听端口">{{ logs?.port ?? '—' }}</a-descriptions-item>
          <a-descriptions-item label="数据目录">
            {{ logs?.dataDir ?? status?.dataDir ?? '—' }}
          </a-descriptions-item>
          <a-descriptions-item label="日志行数">{{ logs?.lines.length ?? 0 }}</a-descriptions-item>
        </a-descriptions>
        <div class="log-toolbar">
          <span class="muted">最近 {{ logs?.lines.length ?? 0 }} 行 · 含慢请求与错误记录</span>
          <a-space>
            <a-button size="small" :loading="logsLoading" @click="loadLogs">刷新</a-button>
            <a-button size="small" :disabled="!logText" @click="copyLogs">复制</a-button>
          </a-space>
        </div>
        <pre class="log-box">{{ logText || '暂时没有日志' }}</pre>
        <p class="note">完整日志按天保存在数据目录的 logs 文件夹，可连同截图一起反馈问题。</p>
      </div>
    </a-drawer>

    <a-modal
      v-model:open="pwdOpen"
      title="修改密码"
      :confirm-loading="pwdSaving"
      ok-text="保存"
      cancel-text="取消"
      :width="420"
      @ok="savePassword"
    >
      <a-form layout="vertical">
        <a-form-item label="当前密码" :extra="auth.username === 'local' ? '本机账号尚未设置密码时可留空' : undefined">
          <a-input-password v-model:value="pwdForm.oldPassword" autocomplete="current-password" />
        </a-form-item>
        <a-form-item label="新密码" extra="8–72 位">
          <a-input-password v-model:value="pwdForm.newPassword" autocomplete="new-password" />
        </a-form-item>
        <a-form-item label="确认新密码">
          <a-input-password v-model:value="pwdForm.confirm" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>

  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { authApi } from '@/api/auth'
import { engineApi, type EngineStatus, type SystemLogs } from '@/api/engine'
import { settingsApi, type Settings } from '@/api/settings'
import { aiApi, type AiConfig } from '@/api/ai'
import { getApiBase } from '@/api/request'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isDark, toggleTheme } from '@/theme'

const appStore = useAppStore()
const auth = useAuthStore()
const status = ref<EngineStatus | null>(null)
const loading = ref(false)
const apiBase = getApiBase()
const healthy = computed(() => Boolean(status.value) || appStore.health?.status === 'ok')

const settingsSaving = ref(false)

// —— 智能助手（BIC-QA 官方 Open API）——
const DEFAULT_AI_BASE_URL = 'https://api.bic-qa.com'

const ai = reactive({
  enabled: false,
  hasKey: false,
  keyMask: '',
  baseUrl: DEFAULT_AI_BASE_URL,
  timeoutSec: 300,
  apiKey: '',
})
const aiSaving = ref(false)
const aiTesting = ref(false)
const aiState = ref('')

/** 填了 Key 才能测连通性；是否真正启用由后端 Ready 再判一次。 */
const aiReady = computed(() => ai.hasKey)

function applyAiConfig(cfg: AiConfig) {
  ai.enabled = cfg.enabled
  ai.hasKey = cfg.hasKey
  ai.keyMask = cfg.keyMask
  ai.baseUrl = cfg.baseUrl
  ai.timeoutSec = cfg.timeoutSec
  ai.apiKey = ''
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
  try {
    const cfg = await aiApi.saveConfig({
      enabled: ai.enabled,
      baseUrl: ai.baseUrl,
      apiKey: ai.apiKey.trim() || undefined,
      timeoutSec: ai.timeoutSec,
    })
    applyAiConfig(cfg)
    aiState.value = ai.enabled ? '已保存，可点「测试连接」验证' : '已保存（当前为关闭状态）'
    message.success('智能助手设置已保存')
  } catch {
    /* 请求层已提示 */
  } finally {
    aiSaving.value = false
  }
}

async function testAi() {
  aiTesting.value = true
  aiState.value = '正在连接 BIC-QA…'
  try {
    const res = await aiApi.test()
    aiState.value = `连接正常 · ${res.baseUrl}`
    message.success('BIC-QA 连接正常')
  } catch (err) {
    aiState.value = err instanceof Error ? `连接失败：${err.message}` : '连接失败'
  } finally {
    aiTesting.value = false
  }
}

async function clearAiKey() {
  aiSaving.value = true
  try {
    applyAiConfig(await aiApi.saveConfig({ clearKey: true, enabled: false }))
    aiState.value = '已清除 API Key，功能已关闭'
    message.success('已清除 API Key')
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
    message.success('设置已保存')
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
    message.warning('新密码长度需为 8–72 位')
    return
  }
  if (pwdForm.newPassword !== pwdForm.confirm) {
    message.warning('两次输入的新密码不一致')
    return
  }
  pwdSaving.value = true
  try {
    await authApi.changePassword(pwdForm.oldPassword, pwdForm.newPassword)
    message.success('密码已更新')
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
    message.error('读取引擎状态失败')
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
    message.success('日志已复制')
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
  if (ok) message.success('日志已复制')
  else message.error('复制失败，请手动选择日志内容')
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
