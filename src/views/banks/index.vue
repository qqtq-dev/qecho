<template>
  <div class="page">
    <PageHeader :title="$t('banks.title')" :subtitle="$t('banks.subtitle')">
      <template #actions>
        <a-tooltip :title="auth.mayExportImport ? '' : $t('banks.noImportPermission')">
          <a-button :disabled="!auth.mayExportImport" @click="importOpen = true">
            {{ $t('banks.import') }}
          </a-button>
        </a-tooltip>
        <a-tooltip :title="auth.mayCreateBank ? '' : $t('banks.noCreatePermission')">
          <a-button type="primary" :disabled="!auth.mayCreateBank" @click="openCreate">
            {{ $t('banks.create') }}
          </a-button>
        </a-tooltip>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('banks.list') }}</h3>
        <span class="muted">{{ $t('banks.count', { n: banks.length }) }}</span>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && banks.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && banks.length === 0" :description="$t('banks.empty')">
            <a-button
              v-if="auth.mayCreateBank"
              type="primary"
              ghost
              @click="openCreate"
            >
              {{ $t('banks.create') }}
            </a-button>
          </a-empty>
          <a-table
            v-else
            :data-source="banks"
            :columns="columns"
            :pagination="false"
            row-key="id"
            size="middle"
            :scroll="{ x: 1200 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <a class="bank-link" @click="router.push(`/banks/${record.id}`)">{{ record.name }}</a>
                <div v-if="record.description" class="muted ellipsis">{{ record.description }}</div>
              </template>
              <template v-else-if="column.key === 'questionCount'">
                <a-tag color="green">{{ $t('banks.questionCount', { n: record.questionCount }) }}</a-tag>
              </template>
              <template v-else-if="column.key === 'practice'">
                <span v-if="record.answeredCount > 0" class="mono">{{ formatRate(record.correctRate) }}</span>
                <span v-else class="muted">—</span>
              </template>
              <template v-else-if="column.key === 'answered'">
                <span v-if="record.answeredCount > 0">
                  {{ $t('banks.answeredTimes', { n: record.answeredCount }) }}
                </span>
                <span v-else class="muted">—</span>
              </template>
              <template v-else-if="column.key === 'lastPracticedAt'">
                <span class="muted">{{ formatTime(record.lastPracticedAt) }}</span>
              </template>
              <template v-else-if="column.key === 'owner'">
                <a-tag v-if="record.isPublic" color="blue">{{ $t('banks.ownerPublic') }}</a-tag>
                <span v-else-if="record.ownerName">{{ record.ownerName }}</span>
                <span v-else class="muted">—</span>
              </template>
              <template v-else-if="column.key === 'access'">
                <a-tag v-if="record.memberLevel === 'owner'" color="green">{{ $t('banks.accessOwner') }}</a-tag>
                <a-tag v-else-if="record.memberLevel === 'admin'" color="purple">{{ $t('banks.accessAdmin') }}</a-tag>
                <a-tag v-else-if="record.memberLevel === 'editor'" color="blue">{{ $t('banks.accessEditor') }}</a-tag>
                <a-tag v-else-if="record.memberLevel === 'viewer'">{{ $t('banks.accessViewer') }}</a-tag>
                <a-tag v-else-if="record.visibility === 'public'" color="cyan">{{ $t('banks.accessPublic') }}</a-tag>
                <a-tag v-else>{{ $t('banks.accessReadonly') }}</a-tag>
              </template>
              <template v-else-if="column.key === 'updatedAt'">
                <span class="muted">{{ formatTime(record.updatedAt) }}</span>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-space :size="2">
                  <a-button type="link" size="small" @click="router.push(`/banks/${record.id}`)">{{ $t('banks.questions') }}</a-button>
                  <template v-if="record.canManage">
                    <a-button
                      v-if="!record.isPublic"
                      type="link"
                      size="small"
                      @click="openSharing(record)"
                    >
                      {{ $t('banks.share') }}
                    </a-button>
                    <a-button type="link" size="small" @click="openEdit(record)">{{ $t('banks.edit') }}</a-button>
                    <a-popconfirm
                      :title="$t('banks.removeConfirm')"
                      :ok-text="$t('banks.remove')"
                      :cancel-text="$t('common.cancel')"
                      @confirm="removeBank(record)"
                    >
                      <a-button type="link" size="small" danger>{{ $t('banks.remove') }}</a-button>
                    </a-popconfirm>
                  </template>
                  <a-tag v-else-if="record.canEdit" color="blue">{{ $t('banks.canEditQuestions') }}</a-tag>
                  <span v-else class="muted readonly">{{ $t('banks.accessReadonly') }}</span>
                </a-space>
              </template>
            </template>
          </a-table>
        </a-spin>
      </div>
    </section>

    <a-modal
      v-model:open="modalOpen"
      :title="editing ? $t('banks.editTitle') : $t('banks.createTitle')"
      :confirm-loading="saving"
      :ok-text="$t('common.save')"
      :cancel-text="$t('common.cancel')"
      :width="460"
      @ok="saveBank"
    >
      <a-form layout="vertical">
        <a-form-item :label="$t('banks.name')" required>
          <a-input v-model:value="form.name" :maxlength="60" :placeholder="$t('banks.namePlaceholder')" />
        </a-form-item>
        <a-form-item v-if="auth.isAdmin && !editing" :label="$t('banks.publicBank')">
          <a-switch v-model:checked="form.public" />
          <span class="muted switch-hint">{{ $t('banks.publicHint') }}</span>
        </a-form-item>
        <a-form-item :label="$t('banks.description')">
          <a-textarea
            v-model:value="form.description"
            :rows="3"
            :maxlength="200"
            :placeholder="$t('banks.descriptionPlaceholder')"
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
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import ImportQuestionsModal from '@/components/ImportQuestionsModal.vue'
import BankSharingDrawer from '@/components/BankSharingDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import { banksApi, type Bank } from '@/api/banks'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const banks = ref<Bank[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const importOpen = ref(false)
const saving = ref(false)
const editing = ref<Bank | null>(null)
const form = reactive({ name: '', description: '', public: false })

const sharingOpen = ref(false)
const sharingBank = ref<Bank | null>(null)

type AccessKey = 'owner' | 'admin' | 'editor' | 'viewer' | 'public' | 'readonly'

function accessKey(bank: Bank): AccessKey {
  if (bank.memberLevel === 'owner') return 'owner'
  if (bank.memberLevel === 'admin') return 'admin'
  if (bank.memberLevel === 'editor') return 'editor'
  if (bank.memberLevel === 'viewer') return 'viewer'
  if (bank.visibility === 'public') return 'public'
  return 'readonly'
}

const accessFilters = computed(() => [
  { text: t('banks.accessOwner'), value: 'owner' },
  { text: t('banks.accessAdmin'), value: 'admin' },
  { text: t('banks.accessEditor'), value: 'editor' },
  { text: t('banks.accessViewer'), value: 'viewer' },
  { text: t('banks.accessPublic'), value: 'public' },
  { text: t('banks.accessReadonly'), value: 'readonly' },
])

const columns = computed(() => [
  { title: t('banks.colName'), key: 'name', dataIndex: 'name' },
  { title: t('banks.colQuestionCount'), key: 'questionCount', dataIndex: 'questionCount', width: 100 },
  { title: t('banks.colAccuracy'), key: 'practice', width: 100 },
  { title: t('banks.colAnswered'), key: 'answered', width: 100 },
  { title: t('banks.colLastPractice'), key: 'lastPracticedAt', width: 150 },
  { title: t('banks.colOwner'), key: 'owner', width: 110 },
  {
    title: t('banks.colAccess'),
    key: 'access',
    width: 130,
    filters: accessFilters.value,
    onFilter: (value: unknown, record: Bank) => accessKey(record) === value,
  },
  { title: t('banks.colUpdated'), key: 'updatedAt', dataIndex: 'updatedAt', width: 160 },
  { title: t('banks.colActions'), key: 'actions', width: 230 },
])

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
    message.warning(t('banks.noCreatePermission'))
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
    message.warning(t('banks.msg.nameRequired'))
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await banksApi.update(editing.value.id, { name, description: form.description.trim() })
      message.success(t('banks.msg.updated'))
    } else {
      await banksApi.create({
        name,
        description: form.description.trim(),
        public: auth.isAdmin && form.public,
      })
      message.success(t('banks.msg.created'))
    }
    modalOpen.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function removeBank(bank: Bank) {
  await banksApi.remove(bank.id)
  message.success(t('banks.msg.deleted'))
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
