<template>
  <div class="page">
    <PageHeader title="题库管理" subtitle="维护本机题库与题目，考试按题库抽题">
      <template #actions>
        <a-tooltip :title="auth.mayExportImport ? '' : '管理员未开放导出与导入题库的权限'">
          <a-button :disabled="!auth.mayExportImport" @click="importOpen = true">导入题库</a-button>
        </a-tooltip>
        <a-tooltip :title="auth.mayCreateBank ? '' : '管理员未开放新建题库的权限'">
          <a-button type="primary" :disabled="!auth.mayCreateBank" @click="openCreate">新建题库</a-button>
        </a-tooltip>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>题库</h3>
        <span class="muted">共 {{ banks.length }} 个</span>
      </div>
      <div class="tabs">
        <a-radio-group v-model:value="tab" button-style="solid" size="small">
          <a-radio-button value="all">全部 {{ banks.length }}</a-radio-button>
          <a-radio-button value="mine">我的 {{ counts.mine }}</a-radio-button>
          <a-radio-button value="shared">共享给我 {{ counts.shared }}</a-radio-button>
          <a-radio-button value="public">公共 {{ counts.public }}</a-radio-button>
        </a-radio-group>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && visibleBanks.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && visibleBanks.length === 0" :description="emptyText">
            <a-button
              v-if="tab !== 'shared' && tab !== 'public' && auth.mayCreateBank"
              type="primary"
              ghost
              @click="openCreate"
            >
              新建题库
            </a-button>
          </a-empty>
          <a-table
            v-else
            :data-source="visibleBanks"
            :columns="columns"
            :pagination="false"
            row-key="id"
            size="middle"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <a class="bank-link" @click="router.push(`/banks/${record.id}`)">{{ record.name }}</a>
                <div v-if="record.description" class="muted ellipsis">{{ record.description }}</div>
              </template>
              <template v-else-if="column.key === 'questionCount'">
                <a-tag color="green">{{ record.questionCount }} 道</a-tag>
              </template>
              <template v-else-if="column.key === 'practice'">
                <template v-if="record.answeredCount > 0">
                  <span class="mono">{{ formatRate(record.correctRate) }}</span>
                  <div class="muted small">作答 {{ record.answeredCount }} 次</div>
                </template>
                <span v-else class="muted">—</span>
              </template>
              <template v-else-if="column.key === 'lastPracticedAt'">
                <span class="muted">{{ formatTime(record.lastPracticedAt) }}</span>
              </template>
              <template v-else-if="column.key === 'owner'">
                <a-tag v-if="record.isPublic" color="blue">公共</a-tag>
                <a-tag v-else-if="record.memberLevel === 'owner'" color="green">我的</a-tag>
                <a-tag v-else-if="record.memberLevel === 'admin'" color="purple">管理员</a-tag>
                <a-tag v-else-if="record.memberLevel === 'editor'" color="blue">共享 · 可编辑</a-tag>
                <a-tag v-else-if="record.memberLevel === 'viewer'">共享 · 只读</a-tag>
                <a-tag v-else-if="record.visibility === 'public'">公开 · 只读</a-tag>
                <a-tag v-else>其他</a-tag>
                <div v-if="record.memberLevel !== 'owner' && record.ownerName" class="muted small">
                  {{ record.ownerName }}
                </div>
              </template>
              <template v-else-if="column.key === 'updatedAt'">
                <span class="muted">{{ formatTime(record.updatedAt) }}</span>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space :size="2">
                  <a-button type="link" size="small" @click="router.push(`/banks/${record.id}`)">题目</a-button>
                  <template v-if="record.canManage">
                    <a-button
                      v-if="!record.isPublic"
                      type="link"
                      size="small"
                      @click="openSharing(record)"
                    >
                      共享
                    </a-button>
                    <a-button type="link" size="small" @click="openEdit(record)">编辑</a-button>
                    <a-popconfirm
                      title="删除后该题库的题目也会一并移除，确定吗？"
                      ok-text="删除"
                      cancel-text="取消"
                      @confirm="removeBank(record)"
                    >
                      <a-button type="link" size="small" danger>删除</a-button>
                    </a-popconfirm>
                  </template>
                  <a-tag v-else-if="record.canEdit" color="blue">可编辑题目</a-tag>
                  <span v-else class="muted readonly">只读</span>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-spin>
      </div>
    </section>

    <a-modal
      v-model:open="modalOpen"
      :title="editing ? '编辑题库' : '新建题库'"
      :confirm-loading="saving"
      ok-text="保存"
      cancel-text="取消"
      :width="460"
      @ok="saveBank"
    >
      <a-form layout="vertical">
        <a-form-item label="题库名称" required>
          <a-input v-model:value="form.name" :maxlength="60" placeholder="例如：2026 安全规程题库" />
        </a-form-item>
        <a-form-item v-if="auth.isAdmin && !editing" label="公共题库">
          <a-switch v-model:checked="form.public" />
          <span class="muted switch-hint">开启后作为公共题库，所有登录用户都能看到并使用</span>
        </a-form-item>
        <a-form-item label="说明">
          <a-textarea
            v-model:value="form.description"
            :rows="3"
            :maxlength="200"
            placeholder="可选，方便区分题库用途"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <BankSharingDrawer
      v-model:open="sharingOpen"
      :bank-id="sharingBank?.id ?? null"
      :bank-name="sharingBank?.name"
      @updated="load"
    />

    <ImportQuestionsModal
      v-model:open="importOpen"
      :banks="banks"
      @imported="load"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import ImportQuestionsModal from '@/components/ImportQuestionsModal.vue'
import BankSharingDrawer from '@/components/BankSharingDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import { banksApi, type Bank } from '@/api/banks'

const router = useRouter()
const auth = useAuthStore()
const banks = ref<Bank[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const importOpen = ref(false)
const saving = ref(false)
const editing = ref<Bank | null>(null)
const form = reactive({ name: '', description: '', public: false })
const tab = ref<'all' | 'mine' | 'shared' | 'public'>('all')

const sharingOpen = ref(false)
const sharingBank = ref<Bank | null>(null)

function groupOf(bank: Bank): 'mine' | 'shared' | 'public' | 'other' {
  if (bank.memberLevel === 'owner') return 'mine'
  if (bank.memberLevel === 'viewer' || bank.memberLevel === 'editor') return 'shared'
  if (bank.isPublic || bank.visibility === 'public') return 'public'
  // 管理员看到的他人私有题库
  return 'other'
}

const counts = computed(() => {
  const out = { mine: 0, shared: 0, public: 0 }
  for (const bank of banks.value) {
    const group = groupOf(bank)
    if (group === 'mine') out.mine += 1
    else if (group === 'shared') out.shared += 1
    else if (group === 'public') out.public += 1
  }
  return out
})

const visibleBanks = computed(() => {
  if (tab.value === 'all') return banks.value
  return banks.value.filter((bank) => groupOf(bank) === tab.value)
})

const emptyText = computed(() => {
  if (tab.value === 'mine') return '你还没有自己的题库'
  if (tab.value === 'shared') return '还没有人共享题库给你'
  if (tab.value === 'public') return '还没有公共题库'
  return '还没有题库'
})

const columns = [
  { title: '题库', key: 'name', dataIndex: 'name' },
  { title: '题目数', key: 'questionCount', dataIndex: 'questionCount', width: 100 },
  { title: '练习正确率', key: 'practice', width: 130 },
  { title: '最近练习', key: 'lastPracticedAt', width: 150 },
  { title: '归属', key: 'owner', width: 130 },
  { title: '更新时间', key: 'updatedAt', dataIndex: 'updatedAt', width: 160 },
  { title: '操作', key: 'actions', width: 230 },
]

function formatRate(rate: number) {
  return `${(Math.round(rate * 1000) / 10).toFixed(1)}%`
}

function formatTime(raw: string) {
  if (!raw) return '—'
  return dayjs(raw).format('YYYY-MM-DD HH:mm')
}

async function load() {
  loading.value = true
  try {
    const res = await banksApi.list()
    banks.value = res.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  if (!auth.mayCreateBank) {
    message.warning('管理员未开放新建题库的权限')
    return
  }
  editing.value = null
  form.name = ''
  form.description = ''
  form.public = false
  modalOpen.value = true
}

function openEdit(bank: Bank) {
  editing.value = bank
  form.name = bank.name
  form.description = bank.description
  form.public = bank.isPublic
  modalOpen.value = true
}

function openSharing(bank: Bank) {
  sharingBank.value = bank
  sharingOpen.value = true
}

async function saveBank() {
  const name = form.name.trim()
  if (!name) {
    message.warning('请填写题库名称')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await banksApi.update(editing.value.id, { name, description: form.description.trim() })
      message.success('题库已更新')
    } else {
      await banksApi.create({
        name,
        description: form.description.trim(),
        public: auth.isAdmin && form.public,
      })
      message.success('题库已创建')
    }
    modalOpen.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function removeBank(bank: Bank) {
  await banksApi.remove(bank.id)
  message.success('题库已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.bank-link { font-weight: 600; }
.ellipsis {
  max-width: 520px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.panel-body :deep(.ant-spin-nested-loading) { min-height: 40px; }
.switch-hint { margin-left: 10px; font-size: 12px; }
.readonly { font-size: 12px; }
.small { font-size: 12px; }
.tabs { padding: 12px 16px 0; }
</style>
