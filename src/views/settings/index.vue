<template>
  <div class="page">
    <PageHeader title="设置" subtitle="外观、本地引擎、数据备份与关闭行为">
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

      <section v-if="auth.isAdmin" class="panel">
        <div class="panel-head">
          <h3>用户管理</h3>
          <a-button size="small" :loading="usersLoading" @click="loadUsers">刷新</a-button>
        </div>
        <div class="panel-body">
          <a-table
            :data-source="users"
            :columns="userColumns"
            :pagination="false"
            row-key="id"
            size="small"
            :loading="usersLoading"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'username'">
                <div class="user-cell">
                  <span>{{ record.username }}</span>
                  <a-tag v-if="record.username === auth.username" color="green">当前账号</a-tag>
                  <a-tag v-if="record.username === 'local'" color="blue">本机</a-tag>
                </div>
              </template>
              <template v-else-if="column.key === 'role'">
                <a-tag :color="record.role === 'admin' ? 'green' : 'default'">
                  {{ record.role === 'admin' ? '管理员' : '普通用户' }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'createdAt'">
                <span class="muted">{{ formatTime(record.createdAt) }}</span>
              </template>
              <template v-else-if="column.key === 'lastLoginAt'">
                <span class="muted">{{ record.lastLoginAt ? formatTime(record.lastLoginAt) : '—' }}</span>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space :size="6">
                  <a-button
                    size="small"
                    :disabled="record.username === 'local' || record.username === auth.username"
                    @click="toggleRole(record)"
                  >
                    {{ record.role === 'admin' ? '设为普通用户' : '设为管理员' }}
                  </a-button>
                  <a-button
                    size="small"
                    :disabled="record.username === 'local'"
                    @click="openReset(record)"
                  >
                    重置密码
                  </a-button>
                </a-space>
              </template>
            </template>
          </a-table>
          <p class="note">重置密码会让该用户已登录的会话失效；本机账号请使用上方“修改密码”。</p>
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
          <div class="setting-row ai-provider-row">
            <div>
              <div class="row-title">接入方式</div>
              <div class="muted">
                两套凭据可以分别保存；打开其中一个会自动关闭另一个，已保存的凭据都保留。
                两个都关闭即停用答疑入口。
              </div>
            </div>
            <div class="ai-provider-list">
              <div class="ai-provider-item" :class="{ on: isProviderOn('bicqa') }">
                <span class="ai-pi-name">BIC-QA 官方</span>
                <span class="ai-pi-state">{{ ai.hasKey ? '已配置' : '未配置' }}</span>
                <a-switch
                  size="small"
                  :checked="isProviderOn('bicqa')"
                  @change="(c: boolean) => toggleProvider('bicqa', c)"
                />
              </div>
              <div class="ai-provider-item" :class="{ on: isProviderOn('imaweb') }">
                <span class="ai-pi-name">ima 网页版</span>
                <span class="ai-pi-state">{{ ai.hasCookie ? '已配置' : '未配置' }}</span>
                <a-switch
                  size="small"
                  :checked="isProviderOn('imaweb')"
                  @change="(c: boolean) => toggleProvider('imaweb', c)"
                />
              </div>
            </div>
          </div>

          <template v-if="ai.provider === 'bicqa'">
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
          </template>

          <template v-else>
            <div class="setting-row ai-key-row">
              <div>
                <div class="row-title">鉴权串（x-ima-cookie）</div>
                <div class="muted">
                  <template v-if="ai.hasCookie">已保存（{{ ai.cookieMask }}），留空表示不修改</template>
                  <template v-else>网页版不发 Cookie 头，鉴权信息放在请求头 x-ima-cookie 里</template>
                </div>
              </div>
              <a-textarea
                v-model:value="ai.cookie"
                class="ai-key-input"
                :rows="3"
                :placeholder="ai.hasCookie ? '留空表示不修改' : 'PLATFORM=H5; IMA-UID=...; IMA-TOKEN=...'"
                autocomplete="off"
              />
            </div>
            <div class="setting-row">
              <div>
                <div class="row-title">知识库</div>
                <div class="muted">
                  选定后提问会先在该库里检索，再结合检索结果作答；不选则走通用问答
                </div>
              </div>
              <div class="ai-kb-picker">
                <a-select
                  v-model:value="ai.kbId"
                  class="ai-key-input"
                  :loading="kbLoading"
                  :disabled="!ai.hasCookie"
                  show-search
                  option-filter-prop="label"
                  :placeholder="ai.hasCookie ? '不限定（通用问答）' : '请先填写鉴权串'"
                  @change="onKbChange"
                >
                  <a-select-option value="">不限定（通用问答）</a-select-option>
                  <a-select-option v-for="kb in kbList" :key="kb.id" :value="kb.id" :label="kb.name">
                    {{ kb.name }}<span v-if="kb.docCount" class="ai-kb-count"> · {{ kb.docCount }} 篇</span>
                  </a-select-option>
                </a-select>
                <a-button size="small" :loading="kbLoading" :disabled="!ai.hasCookie" @click="loadKnowledgeBases">
                  刷新列表
                </a-button>
              </div>
            </div>
            <p v-if="kbError" class="note ai-error">{{ kbError }}</p>
            <details class="ai-howto">
              <summary>怎么拿到这串鉴权信息（约 1 分钟）</summary>
              <ol>
                <li>浏览器打开 <code>https://ima.qq.com</code>，确认右上角是自己的头像（已登录）。</li>
                <li>
                  按 <code>F12</code> 打开开发者工具 → <b>Network / 网络</b> → 筛选框输入
                  <code>ima.qq.com</code> → 按 <code>F5</code> 刷新一次页面，让请求列表有内容。
                </li>
                <li>
                  点开任意一条请求 → <b>Headers / 标头</b> → 在 <b>Request Headers / 请求标头</b> 里找到
                  <code>x-ima-cookie</code> 这一行，复制冒号后面那一整串（通常 700 多字符）。
                </li>
                <li>
                  嫌找起来麻烦就换一种：右键该请求 → <b>Copy → Copy as cURL (bash)</b>，
                  把整条命令直接粘到上面输入框，引擎会自动截取其中的鉴权串。
                </li>
                <li>粘贴后点「保存」，再点「测试连接」；提示连接正常就说明鉴权有效。</li>
              </ol>
            </details>
            <div class="setting-row">
              <div>
                <div class="row-title">服务地址</div>
                <div class="muted">默认 ima 官方域名，一般无需修改</div>
              </div>
              <a-input v-model:value="ai.baseUrl" class="ai-key-input" placeholder="https://ima.qq.com" />
            </div>
            <p class="note ai-warn">
              ima 网页版走的是前端私有接口，不是官方开放平台：接口随时可能调整，鉴权过期后需要重新复制一次
              （界面会提示「登录已失效」）。这串信息等同于你的账号登录凭证，只保存在本机数据目录，请勿分享。
              若追求长期稳定，建议改用 BIC-QA 官方接口。
            </p>
          </template>

          <div class="setting-row">
            <div>
              <div class="row-title">回答超时</div>
              <div class="muted">流式回答的最长等待时间（30–900 秒）</div>
            </div>
            <a-input-number v-model:value="ai.timeoutSec" :min="30" :max="900" :step="30" />
          </div>
          <div class="ai-actions">
            <a-button v-if="ai.hasKey && ai.provider === 'bicqa'" danger size="small" @click="clearAiKey">
              清除已保存的 Key
            </a-button>
            <a-button v-if="ai.hasCookie && ai.provider === 'imaweb'" danger size="small" @click="clearAiCookie">
              清除已保存的鉴权串
            </a-button>
            <span v-if="aiState" class="muted ai-state">{{ aiState }}</span>
          </div>
          <p class="note">
            你的凭据只对本人账号生效，保存在本机数据目录的 bicqa 文件夹，不会写入数据库、不会随备份导出，
            也不会回传到界面；提问内容只包含题干与选项，不包含答案与解析。
          </p>
        </div>
      </section>

      <section v-if="auth.isAdmin" class="panel">
        <div class="panel-head">
          <h3>数据备份</h3>
          <a-button type="primary" :loading="backupBusy" @click="createBackup">立即备份</a-button>
        </div>
        <div class="panel-body">
          <a-alert v-if="pendingRestore" type="warning" show-icon class="restore-alert">
            <template #message>数据恢复已就绪</template>
            <template #description>
              <div class="restore-row">
                <span>重启应用后将用所选备份替换当前数据库，重启前无法写入新数据。</span>
                <a-space>
                  <a-button size="small" type="primary" @click="restartApp">立即重启</a-button>
                  <a-button size="small" :loading="backupBusy" @click="cancelRestore">取消恢复</a-button>
                </a-space>
              </div>
            </template>
          </a-alert>
          <div class="retention-row">
            <span class="muted">自动备份保留</span>
            <label>每日 <a-input-number v-model:value="form.backupDaily" :min="1" :max="30" size="small" style="width: 70px" /> 份</label>
            <label>导入前 <a-input-number v-model:value="form.backupPreImport" :min="1" :max="30" size="small" style="width: 70px" /> 份</label>
            <label>升级前 <a-input-number v-model:value="form.backupPreMigrate" :min="1" :max="30" size="small" style="width: 70px" /> 份</label>
            <a-button size="small" :loading="settingsSaving" @click="saveSettings">保存</a-button>
          </div>
          <div class="backup-summary">
            <span>共 {{ backupItems.length }} 份，占用 {{ formatBytes(backupTotalSize) }}</span>
            <span class="muted">
              自动保留：每日 {{ retention.daily }} 份 · 导入前 {{ retention['pre-import'] }} 份 · 升级前
              {{ retention['pre-migrate'] }} 份；自动备份合计超过 {{ formatBytes(backupCapBytes) }} 时清理最旧。
            </span>
          </div>
          <a-spin :spinning="backupLoading">
            <div class="backup-list">
              <div v-for="item in backupItems" :key="item.name" class="backup-row">
                <a-tag :color="kindColor(item.kind)">{{ kindLabel(item.kind) }}</a-tag>
                <div class="backup-main">
                  <div class="backup-name mono">{{ item.name }}</div>
                  <div class="muted">{{ formatTime(item.createdAt) }} · {{ formatBytes(item.size) }}</div>
                </div>
                <a-space :size="6">
                  <a-button size="small" :disabled="pendingRestore" @click="confirmRestore(item)">恢复</a-button>
                  <a-button size="small" danger :disabled="pendingRestore" @click="confirmDelete(item)">
                    删除
                  </a-button>
                </a-space>
              </div>
              <a-empty v-if="!backupLoading && backupItems.length === 0" description="还没有备份" :image="false" />
            </div>
          </a-spin>
          <p class="note mono">{{ backupsDir }}</p>
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

    <a-modal
      v-model:open="resetOpen"
      :title="`重置密码：${resetUser?.username ?? ''}`"
      :confirm-loading="resetSaving"
      ok-text="重置"
      cancel-text="取消"
      :width="420"
      @ok="saveReset"
    >
      <a-form layout="vertical">
        <a-form-item label="新密码" extra="8–72 位；重置后该用户需用新密码重新登录">
          <a-input-password v-model:value="resetForm.password" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { authApi } from '@/api/auth'
import { usersApi, type UserItem } from '@/api/users'
import { engineApi, type EngineStatus, type SystemLogs } from '@/api/engine'
import { backupsApi, type BackupItem, type BackupOverview } from '@/api/backups'
import { settingsApi, type Settings } from '@/api/settings'
import { aiApi, type AiConfig, type AiProvider, type KnowledgeBase } from '@/api/ai'
import { getApiBase, isTauri } from '@/api/request'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isDark, toggleTheme } from '@/theme'

const appStore = useAppStore()
const auth = useAuthStore()
const status = ref<EngineStatus | null>(null)
const loading = ref(false)
const apiBase = getApiBase()
const healthy = computed(() => Boolean(status.value) || appStore.health?.status === 'ok')

const backupOverview = ref<BackupOverview | null>(null)
const backupLoading = ref(false)
const backupBusy = ref(false)
const pendingRestore = ref(false)
const backupItems = computed(() => backupOverview.value?.items ?? [])
const backupTotalSize = computed(() => backupOverview.value?.totalSize ?? 0)
const backupCapBytes = computed(() => backupOverview.value?.capBytes ?? 0)
const retention = computed(() => backupOverview.value?.retention ?? {})
const backupsDir = computed(() => backupOverview.value?.backupsDir ?? '')

const settingsSaving = ref(false)

// —— 智能助手（BIC-QA / ima 网页版）——
const AI_BASE_URL: Record<AiProvider, string> = {
  bicqa: 'https://api.bic-qa.com',
  imaweb: 'https://ima.qq.com',
}

const ai = reactive({
  provider: 'bicqa' as AiProvider,
  enabled: false,
  hasKey: false,
  keyMask: '',
  hasCookie: false,
  cookieMask: '',
  baseUrl: AI_BASE_URL.bicqa,
  timeoutSec: 300,
  apiKey: '',
  cookie: '',
  kbId: '',
  kbName: '',
})
const aiSaving = ref(false)
const aiTesting = ref(false)
const aiState = ref('')
const kbList = ref<KnowledgeBase[]>([])
const kbLoading = ref(false)
const kbError = ref('')

/** 当前接入方式所需的最小凭据是否已具备（决定「测试连接」能否点）。 */
const aiReady = computed(() => (ai.provider === 'imaweb' ? ai.hasCookie : ai.hasKey))
const aiProviderLabel = computed(() => (ai.provider === 'imaweb' ? 'ima 网页版' : 'BIC-QA'))

function applyAiConfig(cfg: AiConfig) {
  ai.provider = cfg.provider ?? 'bicqa'
  ai.enabled = cfg.enabled
  ai.hasKey = cfg.hasKey
  ai.keyMask = cfg.keyMask
  ai.hasCookie = cfg.hasCookie
  ai.cookieMask = cfg.cookieMask
  ai.baseUrl = cfg.baseUrl
  ai.timeoutSec = cfg.timeoutSec
  ai.kbId = cfg.kbId ?? ''
  ai.kbName = cfg.kbName ?? ''
  ai.apiKey = ''
  ai.cookie = ''
}

/** 拉取 ima 个人知识库列表；只有选了具体库，提问才会带检索参数。 */
async function loadKnowledgeBases() {
  if (!ai.hasCookie) {
    kbError.value = '请先保存鉴权串，再刷新知识库列表'
    return
  }
  kbLoading.value = true
  kbError.value = ''
  try {
    const res = await aiApi.knowledgeBases()
    kbList.value = res.items ?? []
    if (!res.items?.length) kbError.value = '这个账号下没有可用的知识库'
  } catch (err) {
    kbError.value = err instanceof Error ? err.message : '知识库列表加载失败'
  } finally {
    kbLoading.value = false
  }
}

function onKbChange(value: string) {
  const hit = kbList.value.find((kb) => kb.id === value)
  ai.kbName = hit?.name ?? ''
}

/** 该接入方式是否处于启用状态：总开关开着，且它就是当前选中的那一种。 */
function isProviderOn(p: AiProvider) {
  return ai.enabled && ai.provider === p
}

/** 打开一个就关掉另一个；关掉当前启用的这个等于整体停用（练习页不再显示答疑入口）。 */
function toggleProvider(p: AiProvider, on: boolean) {
  aiState.value = ''
  if (!on) {
    if (ai.provider === p) ai.enabled = false
    return
  }
  const switched = ai.provider !== p
  ai.provider = p
  ai.enabled = true
  onProviderChange()
  if (switched) {
    const label = p === 'imaweb' ? 'ima 网页版' : 'BIC-QA 官方'
    const other = p === 'imaweb' ? 'BIC-QA 官方' : 'ima 网页版'
    message.info(`已启用 ${label}，${other} 自动关闭（凭据仍保留）`)
  }
}

/** 切换启用哪一种接入方式：两套凭据各自保留，切换不会清掉任何一边已保存的内容。 */
function onProviderChange() {
  const other = ai.provider === 'imaweb' ? AI_BASE_URL.bicqa : AI_BASE_URL.imaweb
  if (!ai.baseUrl || ai.baseUrl === other) {
    ai.baseUrl = AI_BASE_URL[ai.provider]
  }
  ai.apiKey = ''
  ai.cookie = ''
  aiState.value = ''
  if (ai.provider === 'imaweb' && ai.hasCookie && kbList.value.length === 0) {
    void loadKnowledgeBases()
  }
}

async function loadAiConfig() {
  try {
    applyAiConfig(await aiApi.config())
    if (ai.provider === 'imaweb' && ai.hasCookie) void loadKnowledgeBases()
  } catch {
    /* 引擎未就绪时忽略，进入页面后可重试 */
  }
}

async function saveAi() {
  aiSaving.value = true
  aiState.value = ''
  try {
    const cfg = await aiApi.saveConfig({
      provider: ai.provider,
      enabled: ai.enabled,
      baseUrl: ai.baseUrl,
      apiKey: ai.apiKey.trim() || undefined,
      cookie: ai.cookie.trim() || undefined,
      // 知识库只属于 ima 一套凭据：在 BIC-QA 下保存时不要动它，否则会误清另一边的选择。
      ...(ai.provider === 'imaweb'
        ? {
            kbId: ai.kbId || undefined,
            kbName: ai.kbId ? ai.kbName : undefined,
            clearKb: !ai.kbId,
          }
        : {}),
      timeoutSec: ai.timeoutSec,
    })
    applyAiConfig(cfg)
    aiState.value = ai.enabled ? '已保存，可点「测试连接」验证' : '已保存（当前为关闭状态）'
    message.success(`${aiProviderLabel.value} 设置已保存`)
    if (ai.provider === 'imaweb' && ai.hasCookie && kbList.value.length === 0) {
      void loadKnowledgeBases()
    }
  } catch {
    /* 请求层已提示 */
  } finally {
    aiSaving.value = false
  }
}

async function testAi() {
  aiTesting.value = true
  aiState.value = `正在连接 ${aiProviderLabel.value}…`
  try {
    const res = await aiApi.test()
    aiState.value = `连接正常 · ${res.baseUrl}`
    message.success(`${aiProviderLabel.value} 连接正常`)
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

async function clearAiCookie() {
  aiSaving.value = true
  try {
    applyAiConfig(await aiApi.saveConfig({ clearCookie: true, enabled: false }))
    aiState.value = '已清除鉴权串，功能已关闭'
    message.success('已清除鉴权串')
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
    if (auth.isAdmin) await loadBackups()
  } finally {
    settingsSaving.value = false
  }
}

const KIND_LABELS: Record<string, string> = {
  manual: '手动',
  daily: '每日自动',
  'pre-import': '导入前自动',
  'pre-migrate': '升级前自动',
  'pre-restore': '恢复前自动',
}

const KIND_COLORS: Record<string, string> = {
  manual: 'green',
  daily: 'blue',
  'pre-import': 'orange',
  'pre-migrate': 'purple',
  'pre-restore': 'cyan',
}

function kindLabel(kind: string): string {
  return KIND_LABELS[kind] ?? kind
}

function kindColor(kind: string): string {
  return KIND_COLORS[kind] ?? 'default'
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

const users = ref<UserItem[]>([])
const usersLoading = ref(false)
const userColumns = [
  { title: '用户', key: 'username', dataIndex: 'username' },
  { title: '角色', key: 'role', width: 100 },
  { title: '注册时间', key: 'createdAt', width: 160 },
  { title: '最近登录', key: 'lastLoginAt', width: 160 },
  { title: '操作', key: 'actions', width: 210 },
]

async function loadUsers() {
  usersLoading.value = true
  try {
    users.value = (await usersApi.list()).items
  } finally {
    usersLoading.value = false
  }
}

function toggleRole(item: UserItem) {
  const target: 'admin' | 'user' = item.role === 'admin' ? 'user' : 'admin'
  Modal.confirm({
    title: target === 'admin' ? '设为管理员' : '取消管理员',
    content:
      target === 'admin'
        ? `将 ${item.username} 设为管理员，可管理用户、共享标签与全局设置。`
        : `将 ${item.username} 设为普通用户，其只能管理自己的题库。`,
    okText: '确定',
    cancelText: '取消',
    async onOk() {
      await usersApi.updateRole(item.id, target)
      message.success('角色已更新')
      await loadUsers()
    },
  })
}

const resetOpen = ref(false)
const resetSaving = ref(false)
const resetUser = ref<UserItem | null>(null)
const resetForm = reactive({ password: '' })

function openReset(item: UserItem) {
  resetUser.value = item
  resetForm.password = ''
  resetOpen.value = true
}

async function saveReset() {
  if (!resetUser.value) return
  if (resetForm.password.length < 8 || resetForm.password.length > 72) {
    message.warning('密码长度需为 8–72 位')
    return
  }
  resetSaving.value = true
  try {
    await usersApi.resetPassword(resetUser.value.id, resetForm.password)
    message.success(`已重置 ${resetUser.value.username} 的密码`)
    resetOpen.value = false
  } finally {
    resetSaving.value = false
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

async function loadBackups() {
  backupLoading.value = true
  try {
    const data = await backupsApi.list()
    backupOverview.value = data
    pendingRestore.value = data.pendingRestore
  } catch {
    backupOverview.value = null
  } finally {
    backupLoading.value = false
  }
}

async function createBackup() {
  backupBusy.value = true
  try {
    const info = await backupsApi.create()
    message.success(`已创建备份 ${info.name}`)
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

function confirmDelete(item: BackupItem) {
  Modal.confirm({
    title: '删除备份',
    content: `确定删除 ${item.name}？删除后无法恢复。`,
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await backupsApi.remove(item.name)
      message.success('备份已删除')
      await loadBackups()
    },
  })
}

function confirmRestore(item: BackupItem) {
  Modal.confirm({
    title: '恢复数据',
    content: `将用 ${item.name} 替换当前数据库，恢复前会自动备份当前数据。恢复需要重启应用后生效。`,
    okText: '准备恢复',
    cancelText: '取消',
    async onOk() {
      backupBusy.value = true
      try {
        await backupsApi.restore(item.name)
        pendingRestore.value = true
        await loadBackups()
        Modal.confirm({
          title: '恢复已就绪',
          content: '重启应用后完成数据恢复；重启前应用不再接受新数据。',
          okText: '立即重启',
          cancelText: '稍后重启',
          onOk() {
            return restartApp()
          },
        })
      } finally {
        backupBusy.value = false
      }
    },
  })
}

async function cancelRestore() {
  backupBusy.value = true
  try {
    await backupsApi.cancelRestore()
    pendingRestore.value = false
    message.success('已取消恢复')
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

async function restartApp() {
  if (!isTauri()) {
    message.info('请关闭并重新启动应用或引擎以完成恢复')
    return
  }
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('restart_app')
  } catch {
    message.error('重启失败，请手动关闭并重新打开应用')
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
  if (auth.isAdmin) {
    loadBackups()
    loadUsers()
  }
})
</script>

<style scoped>
.stack { display: grid; gap: 12px; }
.row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 0; border-bottom: 1px solid var(--border); }
/* 接入方式：每套一个开关，打开一个另一个自动关（凭据都保留） */
.ai-provider-row { align-items: flex-start; }
.ai-provider-list { display: flex; flex-direction: column; gap: 6px; flex: none; width: 264px; }
.ai-provider-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-2);
}
.ai-provider-item.on { border-color: var(--primary); background: var(--primary-soft); }
.ai-pi-name { font-size: 13px; color: var(--text); }
.ai-provider-item.on .ai-pi-name { color: var(--primary); font-weight: 600; }
.ai-pi-state { flex: 1; text-align: right; font-size: 11.5px; color: var(--text2); }
.ai-intro { margin: 0 0 8px; }
.ai-key-row { align-items: flex-start; }
.ai-key-input { width: 300px; flex: none; }
.ai-actions { display: flex; align-items: center; gap: 12px; padding: 10px 0 2px; }
.ai-state { font-size: 12.5px; }
.ai-kb-picker { display: flex; align-items: center; gap: 8px; }
.ai-kb-count { color: var(--muted); font-size: 12px; }
.ai-error { color: var(--bad-text); }
.ai-warn {
  margin: 8px 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--warn-soft);
  color: var(--warn-text);
  font-size: 12.5px;
  line-height: 1.6;
}
.setting-row:last-child { border-bottom: 0; padding-bottom: 0; }
.setting-row:first-child { padding-top: 0; }
.retention-row { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; font-size: 12px; margin-bottom: 10px; }
.retention-row label { display: inline-flex; align-items: center; gap: 6px; color: var(--text2); }
.row-title { font-size: 13px; font-weight: 600; margin-bottom: 2px; }
.user-cell { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.kv { margin: 0; display: grid; gap: 10px; }
.kv > div { display: flex; justify-content: space-between; gap: 16px; }
.kv dt { color: var(--text2); font-size: 12px; flex: 0 0 auto; }
.kv dd { margin: 0; font-size: 12px; text-align: right; word-break: break-all; }
.note { margin: 14px 0 0; font-size: 11px; color: var(--text2); }
.restore-alert { margin-bottom: 14px; }
.restore-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.backup-summary { display: grid; gap: 4px; font-size: 12px; margin-bottom: 8px; }
.backup-list { display: grid; gap: 0; }
.backup-row { display: flex; align-items: center; gap: 10px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.backup-row:last-child { border-bottom: 0; }
.backup-main { flex: 1; min-width: 0; display: grid; gap: 2px; }
.backup-name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
