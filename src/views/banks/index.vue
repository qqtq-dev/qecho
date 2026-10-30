<template>
  <div class="page">
    <PageHeader title="题库管理" subtitle="维护本机题库与题目，考试按题库抽题">
      <template #actions>
        <a-button @click="importOpen = true">导入题库</a-button>
        <a-button type="primary" @click="openCreate">新建题库</a-button>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>全部题库</h3>
        <span class="muted">共 {{ banks.length }} 个</span>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && banks.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && banks.length === 0" description="还没有题库">
            <a-button type="primary" ghost @click="openCreate">新建第一个题库</a-button>
          </a-empty>
          <a-table
            v-else
            :data-source="banks"
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
              <template v-else-if="column.key === 'owner'">
                <a-tag v-if="record.isPublic" color="blue">公共</a-tag>
                <a-tag v-else color="default">{{ record.ownerName || '我的' }}</a-tag>
              </template>
              <template v-else-if="column.key === 'updatedAt'">
                <span class="muted">{{ formatTime(record.updatedAt) }}</span>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space>
                  <a-button type="link" size="small" @click="router.push(`/banks/${record.id}`)">题目</a-button>
                  <template v-if="canWrite(record)">
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
        <a-form-item v-if="auth.isAdmin && !editing" label="共享范围">
          <a-switch v-model:checked="form.public" />
          <span class="muted switch-hint">开启后所有登录用户都能看到并使用该题库</span>
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

    <ImportQuestionsModal
      v-model:open="importOpen"
      :banks="banks"
      @imported="load"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import ImportQuestionsModal from '@/components/ImportQuestionsModal.vue'
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

function canWrite(bank: Bank) {
  return auth.isAdmin || (bank.ownerId !== null && bank.ownerId !== undefined)
}

const columns = [
  { title: '题库', key: 'name', dataIndex: 'name' },
  { title: '题目数', key: 'questionCount', dataIndex: 'questionCount', width: 100 },
  { title: '练习正确率', key: 'practice', width: 130 },
  { title: '归属', key: 'owner', width: 110 },
  { title: '更新时间', key: 'updatedAt', dataIndex: 'updatedAt', width: 165 },
  { title: '操作', key: 'actions', width: 190 },
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
</style>
