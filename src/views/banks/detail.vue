<template>
  <div class="page">
    <div class="back">
      <a-button type="link" size="small" @click="router.push('/banks')">
        {{ $t('bankDetail.back') }}
      </a-button>
    </div>
    <PageHeader
      :title="bank?.name || $t('bankDetail.titleFallback')"
      :subtitle="bank?.description || $t('bankDetail.subtitleFallback')"
    >
      <template #actions>
        <template v-if="auth.mayExportImport">
          <a-button :loading="exporting" :disabled="total === 0" @click="exportBank">
            {{ $t('bankDetail.exportQuestions') }}
          </a-button>
          <a-button :loading="exportingImages" @click="exportImages">
            {{ $t('bankDetail.exportImages') }}
          </a-button>
        </template>
        <template v-if="canManage">
          <a-button @click="openEditBank">{{ $t('bankDetail.editBank') }}</a-button>
          <a-button v-if="bank && !bank.isPublic" @click="sharingOpen = true">{{ $t('bankDetail.share') }}</a-button>
        </template>
        <template v-if="canEdit">
          <a-button v-if="auth.mayExportImport" @click="importOpen = true">
            {{ $t('bankDetail.importQuestions') }}
          </a-button>
          <a-button type="primary" @click="openCreateQuestion">{{ $t('bankDetail.addQuestion') }}</a-button>
        </template>
        <a-tag v-if="!canEdit" color="default">{{ $t('bankDetail.readOnlyBank') }}</a-tag>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('bankDetail.questions') }}</h3>
        <span class="muted">{{ $t('bankDetail.count', { n: total }) }}</span>
      </div>
      <div class="filters">
        <a-input-search
          v-model:value="filters.keyword"
          :placeholder="$t('bankDetail.searchPlaceholder')"
          allow-clear
          style="width: 240px"
          @search="applyFilters"
        />
        <a-select v-model:value="filters.type" style="width: 110px" @change="applyFilters">
          <a-select-option value="">{{ $t('bankDetail.allTypes') }}</a-select-option>
          <a-select-option value="single">{{ $t('bankDetail.single') }}</a-select-option>
          <a-select-option value="multiple">{{ $t('bankDetail.multiple') }}</a-select-option>
        </a-select>
        <a-select v-model:value="filters.difficulty" style="width: 136px" @change="applyFilters">
          <a-select-option :value="0">{{ $t('difficulty.all') }}</a-select-option>
          <a-select-option v-for="d in DIFFICULTY_LEVELS" :key="d.value" :value="d.value">
            {{ difficultyLabel(d.value) }}
          </a-select-option>
        </a-select>
        <a-select
          v-model:value="filters.tag"
          style="width: 150px"
          :placeholder="$t('bankDetail.allTags')"
          allow-clear
          @change="applyFilters"
        >
          <a-select-option v-for="tag in bankTags" :key="tag.id" :value="tag.name">
            {{ $t('bankDetail.tagOption', { name: tag.name, n: tag.questionCount }) }}
          </a-select-option>
        </a-select>
        <a-button v-if="canManage" type="link" size="small" @click="openTagManager">
          {{ $t('bankDetail.tagManager') }}
        </a-button>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && items.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && items.length === 0" :description="$t('bankDetail.empty')">
            <a-button v-if="canEdit" type="primary" ghost @click="openCreateQuestion">
              {{ $t('bankDetail.addFirst') }}
            </a-button>
          </a-empty>
          <template v-else>
            <a-table
              :data-source="items"
              :columns="columns"
              :pagination="false"
              row-key="id"
              size="middle"
            >
              <template #bodyCell="{ column, record, index }">
                <template v-if="column.key === 'index'">
                  <span class="muted">{{ (page - 1) * size + index + 1 }}</span>
                </template>
                <template v-else-if="column.key === 'type'">
                  <a-tag :color="record.type === 'multiple' ? 'purple' : 'blue'">
                    {{ record.type === 'multiple' ? $t('bankDetail.multiple') : $t('bankDetail.single') }}
                  </a-tag>
                </template>
                <template v-else-if="column.key === 'stem'">
                  <div class="stem">{{ record.stem }}</div>
                  <div v-if="record.source" class="muted">
                    {{ $t('bankDetail.source', { source: record.source }) }}
                  </div>
                  <div v-if="hasImage(record)" class="muted">{{ $t('bankDetail.hasImage') }}</div>
                </template>
                <template v-else-if="column.key === 'answer'">
                  <span class="mono answer">{{ record.answer.join('') || '—' }}</span>
                </template>
                <template v-else-if="column.key === 'tags'">
                  <a-space v-if="record.tags?.length" :size="4" wrap>
                    <a-tag v-for="tag in record.tags" :key="tag" class="tag-chip">{{ tag }}</a-tag>
                  </a-space>
                  <span v-else class="muted">—</span>
                </template>
                <template v-else-if="column.key === 'difficulty'">
                  <span class="muted">{{ difficultyLabel(record.difficulty) }}</span>
                </template>
                <template v-else-if="column.key === 'actions' && canEdit">
                  <a-space>
                    <a-button type="link" size="small" @click="openEditQuestion(record)">{{ $t('bankDetail.edit') }}</a-button>
                    <a-popconfirm
                      :title="$t('bankDetail.removeConfirm')"
                      :ok-text="$t('bankDetail.remove')"
                      :cancel-text="$t('common.cancel')"
                      @confirm="removeQuestion(record)"
                    >
                      <a-button type="link" size="small" danger>{{ $t('bankDetail.remove') }}</a-button>
                    </a-popconfirm>
                  </a-space>
                </template>
              </template>
            </a-table>
            <div class="pager">
              <a-pagination
                v-model:current="page"
                :total="total"
                :page-size="size"
                :show-size-changer="false"
                size="small"
                @change="loadQuestions"
              />
            </div>
          </template>
        </a-spin>
      </div>
    </section>

    <section v-if="importJobs.length" class="panel">
      <div class="panel-head">
        <h3>{{ $t('bankDetail.importHistory') }}</h3>
        <span class="muted">{{ $t('bankDetail.importHistoryDesc', { n: importJobs.length }) }}</span>
      </div>
      <div class="panel-body">
        <a-table
          :data-source="importJobs"
          :columns="importColumns"
          :pagination="false"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'createdAt'">
              <span class="muted">{{ formatTime(record.createdAt) }}</span>
            </template>
            <template v-else-if="column.key === 'source'">
              <div class="import-file">{{ record.sourceName }}</div>
              <div class="muted small">{{ record.format?.toUpperCase() }}</div>
            </template>
            <template v-else-if="column.key === 'summary'">
              <span>{{ $t('bankDetail.inserted', { n: record.inserted }) }}</span>
              <span class="muted"> · {{ $t('bankDetail.skipped', { n: record.skipped }) }}</span>
              <span v-if="record.failed" class="danger"> · {{ $t('bankDetail.failed', { n: record.failed }) }}</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="record.status === 'reverted' ? 'default' : record.failed ? 'orange' : 'green'">
                {{
                  record.status === 'reverted'
                    ? $t('bankDetail.statusReverted')
                    : record.failed
                      ? $t('bankDetail.statusPartial')
                      : $t('bankDetail.statusDone')
                }}
              </a-tag>
              <span v-if="record.status !== 'reverted'" class="muted small">
                · {{ $t('bankDetail.remaining', { n: record.remaining }) }}
              </span>
            </template>
            <template v-else-if="column.key === 'actions'">
              <a-space>
                <a-button
                  v-if="record.failed"
                  type="link"
                  size="small"
                  @click="downloadErrors(record)"
                >
                  {{ $t('bankDetail.errorDetail') }}
                </a-button>
                <a-popconfirm
                  v-if="record.canRevert"
                  :title="$t('bankDetail.revertConfirm')"
                  :ok-text="$t('bankDetail.revert')"
                  :cancel-text="$t('common.cancel')"
                  @confirm="revertJob(record)"
                >
                  <a-button type="link" size="small" danger>{{ $t('bankDetail.revert') }}</a-button>
                </a-popconfirm>
                <span v-else-if="record.status !== 'reverted'" class="muted small">{{ $t('bankDetail.cannotRevert') }}</span>
              </a-space>
            </template>
          </template>
        </a-table>
      </div>
    </section>

    <a-modal
      v-model:open="tagModalOpen"
      :title="$t('bankDetail.tagManager')"
      :footer="null"
      :width="620"
    >
      <div class="tag-manage">
        <a-empty v-if="allTags.length === 0" :description="$t('bankDetail.tagEmpty')" />
        <a-table
          v-else
          :data-source="allTags"
          :columns="tagColumns"
          row-key="id"
          size="small"
          :pagination="false"
          :scroll="{ y: 360 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <a-input
                v-if="editingTagId === record.id"
                v-model:value="editingTagName"
                size="small"
                :maxlength="20"
                @press-enter="saveTagRename(record)"
              />
              <span v-else>{{ record.name }}</span>
            </template>
            <template v-else-if="column.key === 'questionCount'">
              <span class="muted">{{ $t('bankDetail.count', { n: record.questionCount }) }}</span>
            </template>
            <template v-else-if="column.key === 'actions'">
              <a-space>
                <template v-if="editingTagId === record.id">
                  <a-button type="link" size="small" @click="saveTagRename(record)">{{ $t('common.save') }}</a-button>
                  <a-button type="link" size="small" @click="cancelTagRename">{{ $t('common.cancel') }}</a-button>
                </template>
                <template v-else>
                  <a-button type="link" size="small" @click="startTagRename(record)">{{ $t('bankDetail.rename') }}</a-button>
                  <a-button
                    type="link"
                    size="small"
                    :disabled="allTags.length < 2"
                    @click="openMerge(record)"
                  >
                    {{ $t('bankDetail.merge') }}
                  </a-button>
                  <a-button type="link" size="small" danger @click="removeTag(record)">{{ $t('bankDetail.remove') }}</a-button>
                </template>
              </a-space>
            </template>
          </template>
        </a-table>
      </div>
    </a-modal>

    <a-modal
      v-model:open="mergeModalOpen"
      :title="$t('bankDetail.mergeTitle', { name: mergeSource?.name ?? '' })"
      :confirm-loading="mergeSaving"
      :ok-text="$t('bankDetail.mergeOk')"
      :cancel-text="$t('common.cancel')"
      :width="420"
      @ok="confirmMerge"
    >
      <p class="muted merge-tip">{{ $t('bankDetail.mergeTip') }}</p>
      <a-select v-model:value="mergeTargetId" style="width: 100%" :placeholder="$t('bankDetail.mergePlaceholder')">
        <a-select-option v-for="t in mergeTargets" :key="t.id" :value="t.id">
          {{ $t('bankDetail.mergeOption', { name: t.name, n: t.questionCount }) }}
        </a-select-option>
      </a-select>
    </a-modal>

    <a-modal
      v-model:open="bankModalOpen"
      :title="$t('bankDetail.editBank')"
      :confirm-loading="bankSaving"
      :ok-text="$t('common.save')"
      :cancel-text="$t('common.cancel')"
      :width="460"
      @ok="saveBank"
    >
      <a-form layout="vertical">
        <a-form-item :label="$t('bankDetail.name')" required>
          <a-input v-model:value="bankForm.name" :maxlength="60" />
        </a-form-item>
        <a-form-item :label="$t('bankDetail.description')">
          <a-textarea v-model:value="bankForm.description" :rows="3" :maxlength="200" />
        </a-form-item>
      </a-form>
    </a-modal>

    <QuestionEditModal
      v-model:open="qModalOpen"
      :bank-id="bankId"
      :question="editingQuestion"
      @saved="onQuestionSaved"
    />

    <BankSharingDrawer
      v-model:open="sharingOpen"
      :bank-id="bankId"
      :bank-name="bank?.name"
      @updated="loadBank"
    />

    <ImportQuestionsModal
      v-model:open="importOpen"
      :bank-id="bankId"
      :bank-name="bank?.name"
      @imported="onImported"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import ImportQuestionsModal from '@/components/ImportQuestionsModal.vue'
import BankSharingDrawer from '@/components/BankSharingDrawer.vue'
import QuestionEditModal from '@/components/QuestionEditModal.vue'
import { banksApi, type Bank, type ImportJob, type Question, type Tag, type TagImpact } from '@/api/banks'
import { isTauri } from '@/api/request'
import { DIFFICULTY_LEVELS, difficultyLabel } from '@/utils/difficulty'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const bankId = Number(route.params.id)

const bank = ref<Bank | null>(null)
const items = ref<Question[]>([])
const total = ref(0)
const page = ref(1)
const size = 20
const loading = ref(false)
const importOpen = ref(false)
const sharingOpen = ref(false)
const bankTags = ref<Tag[]>([])
const filters = reactive({
  keyword: '',
  type: '' as '' | 'single' | 'multiple',
  tag: undefined as string | undefined,
  difficulty: 0,
})

const canEdit = computed(() => bank.value?.canEdit ?? false)
const canManage = computed(() => bank.value?.canManage ?? false)

const columns = computed(() => {
  const base = [
    { title: '#', key: 'index', width: 56 },
    { title: t('bankDetail.colType'), key: 'type', width: 76 },
    { title: t('bankDetail.colStem'), key: 'stem', dataIndex: 'stem' },
    { title: t('bankDetail.colTags'), key: 'tags', width: 150 },
    { title: t('bankDetail.colAnswer'), key: 'answer', width: 90 },
    { title: t('bankDetail.colDifficulty'), key: 'difficulty', width: 90 },
  ]
  if (canEdit.value) base.push({ title: t('bankDetail.colActions'), key: 'actions', width: 130 })
  return base
})

const hasImage = (q: Question) => !!q.stemImage || q.options.some((o) => !!o.image)

async function loadBank() {
  bank.value = await banksApi.get(bankId)
}

const exporting = ref(false)
const exportingImages = ref(false)

function exportFileName(suffix = '') {
  const fallback = t('banks.defaultName')
  const name = (bank.value?.name || fallback).replace(/[\\/:*?"<>|]/g, '_').trim() || fallback
  return `${name}${suffix}-${dayjs().format('YYYYMMDD')}.xlsx`
}

async function exportImages() {
  if (!bank.value || exportingImages.value) return
  exportingImages.value = true
  try {
    const fileName = exportFileName(t('bankDetail.imageSuffix')).replace(/\.xlsx$/, '.zip')
    if (isTauri()) {
      const { invoke } = await import('@tauri-apps/api/core')
      const path = await invoke<string | null>('pick_export_path', { fileName })
      if (!path) return
      const res = await banksApi.exportImagesTo(bankId, path)
      message.success(t('bankDetail.msg.imagesExported', { count: res.count, path: res.path }))
      return
    }
    const blob = await banksApi.exportImagesFile(bankId)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success(t('bankDetail.msg.downloadStarted'))
  } finally {
    exportingImages.value = false
  }
}

async function exportBank() {
  if (!bank.value) return
  exporting.value = true
  try {
    const fileName = exportFileName()
    if (isTauri()) {
      const { invoke } = await import('@tauri-apps/api/core')
      const path = await invoke<string | null>('pick_export_path', { fileName })
      if (!path) return
      const res = await banksApi.exportTo(bankId, path)
      message.success(t('bankDetail.msg.questionsExported', { count: res.count, path: res.path }))
      return
    }
    const blob = await banksApi.exportFile(bankId)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success(t('bankDetail.msg.downloadStarted'))
  } finally {
    exporting.value = false
  }
}

async function loadQuestions() {
  loading.value = true
  try {
    const res = await banksApi.questions(bankId, {
      page: page.value,
      size,
      keyword: filters.keyword.trim() || undefined,
      type: filters.type || undefined,
      tag: filters.tag,
      difficulty: filters.difficulty || undefined,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  void loadQuestions()
}

async function loadTags() {
  try {
    const res = await banksApi.bankTags(bankId)
    bankTags.value = res.items
  } catch {
    bankTags.value = []
  }
}

const tagModalOpen = ref(false)
const allTags = ref<Tag[]>([])
const tagColumns = computed(() => [
  { title: t('bankDetail.colTagName'), key: 'name', dataIndex: 'name' },
  { title: t('bankDetail.colTagCount'), key: 'questionCount', width: 90 },
  { title: t('bankDetail.colActions'), key: 'actions', width: 200 },
])
const editingTagId = ref<number | null>(null)
const editingTagName = ref('')
const mergeModalOpen = ref(false)
const mergeSource = ref<Tag | null>(null)
const mergeTargetId = ref<number | undefined>(undefined)
const mergeSaving = ref(false)
const mergeTargets = computed(() => allTags.value.filter((t) => t.id !== mergeSource.value?.id))

async function loadAllTags() {
  const res = await banksApi.tags()
  allTags.value = res.items
}

function openTagManager() {
  tagModalOpen.value = true
  void loadAllTags()
}

function startTagRename(tag: Tag) {
  editingTagId.value = tag.id
  editingTagName.value = tag.name
}

function cancelTagRename() {
  editingTagId.value = null
  editingTagName.value = ''
}

async function refreshAfterTagChange() {
  await Promise.all([loadAllTags(), loadTags()])
  if (filters.tag && !bankTags.value.some((t) => t.name === filters.tag)) {
    filters.tag = undefined
  }
  await loadQuestions()
}

/**
 * 标签是全局字典：改名/合并/删除会连带改到别人的题库。
 * 先查影响面，涉及他人题库时把数量写进确认框。
 */
const VERB_KEY = {
  rename: 'bankDetail.verbRename',
  merge: 'bankDetail.verbMerge',
  delete: 'bankDetail.verbDelete',
} as const

async function confirmTagAction(
  tag: Tag,
  verb: keyof typeof VERB_KEY,
  run: () => Promise<void>,
  alwaysConfirm = false,
) {
  let impact: TagImpact | null = null
  try {
    impact = await banksApi.tagImpact(tag.id)
  } catch {
    /* 拿不到影响面就按普通流程处理 */
  }
  const others = impact?.otherBanks ?? 0
  if (!alwaysConfirm && others === 0) {
    await run()
    return
  }
  const verbText = t(VERB_KEY[verb])
  const lines = [
    t('bankDetail.tagImpact', { questions: impact?.questions ?? 0, banks: impact?.banks ?? 0 }),
  ]
  if (others > 0) {
    lines.push(
      t('bankDetail.tagImpactOthers', {
        banks: others,
        questions: impact?.otherQuestions ?? 0,
        verb: verbText.toLowerCase(),
      }),
    )
  }
  Modal.confirm({
    title: t('bankDetail.tagConfirmTitle', { verb: verbText, name: tag.name }),
    content: lines.join(''),
    okText: t('bankDetail.tagConfirmOk', { verb: verbText }),
    cancelText: t('common.cancel'),
    okType: verb === 'delete' ? 'danger' : 'primary',
    async onOk() {
      await run()
    },
  })
}

async function saveTagRename(tag: Tag) {
  const name = editingTagName.value.trim()
  if (!name) {
    message.warning(t('bankDetail.msg.tagNameRequired'))
    return
  }
  if (name === tag.name) {
    cancelTagRename()
    return
  }
  await confirmTagAction(tag, 'rename', async () => {
    await banksApi.renameTag(tag.id, name)
    message.success(t('bankDetail.msg.tagRenamed'))
    cancelTagRename()
    await refreshAfterTagChange()
  })
}

function openMerge(tag: Tag) {
  mergeSource.value = tag
  mergeTargetId.value = undefined
  mergeModalOpen.value = true
}

async function confirmMerge() {
  if (!mergeSource.value || !mergeTargetId.value) {
    message.warning(t('bankDetail.msg.mergeTargetRequired'))
    return
  }
  const source = mergeSource.value
  const targetId = mergeTargetId.value
  mergeSaving.value = true
  try {
    await confirmTagAction(source, 'merge', async () => {
      await banksApi.mergeTag(source.id, targetId)
      message.success(t('bankDetail.msg.tagMerged'))
      mergeModalOpen.value = false
      await refreshAfterTagChange()
    })
  } finally {
    mergeSaving.value = false
  }
}

async function removeTag(tag: Tag) {
  await confirmTagAction(
    tag,
    'delete',
    async () => {
      await banksApi.removeTag(tag.id)
      message.success(t('bankDetail.msg.tagDeleted'))
      await refreshAfterTagChange()
    },
    true,
  )
}

const importJobs = ref<ImportJob[]>([])
const importColumns = computed(() => [
  { title: t('bankDetail.colImportTime'), key: 'createdAt', width: 150 },
  { title: t('bankDetail.colFile'), key: 'source', dataIndex: 'sourceName' },
  { title: t('bankDetail.colResult'), key: 'summary', width: 180 },
  { title: t('bankDetail.colStatus'), key: 'status', width: 170 },
  { title: t('bankDetail.colActions'), key: 'actions', width: 150 },
])

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('YYYY-MM-DD HH:mm') : '—'
}

async function loadImportJobs() {
  try {
    const res = await banksApi.importJobs(bankId)
    importJobs.value = res.items
  } catch {
    importJobs.value = []
  }
}

function saveBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.click()
  URL.revokeObjectURL(url)
}

async function downloadErrors(job: ImportJob) {
  const blob = await banksApi.importErrorsCsv(job.id)
  saveBlob(blob, t('bankDetail.importErrorsFile', { id: job.id }))
}

async function revertJob(job: ImportJob) {
  const res = await banksApi.revertImport(job.id)
  message.success(
    res.kept
      ? t('bankDetail.msg.revertedKeep', { count: res.reverted, kept: res.kept })
      : t('bankDetail.msg.reverted', { count: res.reverted }),
  )
  await Promise.all([loadImportJobs(), loadQuestions(), loadBank(), loadTags()])
}

const bankModalOpen = ref(false)
const bankSaving = ref(false)
const bankForm = reactive({ name: '', description: '' })

function openEditBank() {
  if (!bank.value) return
  bankForm.name = bank.value.name
  bankForm.description = bank.value.description
  bankModalOpen.value = true
}

async function saveBank() {
  const name = bankForm.name.trim()
  if (!name) {
    message.warning(t('bankDetail.msg.bankNameRequired'))
    return
  }
  bankSaving.value = true
  try {
    bank.value = await banksApi.update(bankId, { name, description: bankForm.description.trim() })
    bankModalOpen.value = false
    message.success(t('bankDetail.msg.bankUpdated'))
  } finally {
    bankSaving.value = false
  }
}

const qModalOpen = ref(false)
const editingQuestion = ref<Question | null>(null)

function openCreateQuestion() {
  editingQuestion.value = null
  qModalOpen.value = true
}

function openEditQuestion(q: Question) {
  editingQuestion.value = q
  qModalOpen.value = true
}

// 公共弹窗保存成功后刷新列表；题目对象由弹窗带回
async function onQuestionSaved() {
  await Promise.all([loadQuestions(), loadBank(), loadTags()])
}

async function removeQuestion(q: Question) {
  await banksApi.removeQuestion(q.id)
  message.success(t('bankDetail.msg.questionDeleted'))
  if (items.value.length === 1 && page.value > 1) page.value -= 1
  await Promise.all([loadQuestions(), loadBank(), loadTags()])
}

async function onImported() {
  page.value = 1
  await Promise.all([loadBank(), loadQuestions(), loadTags(), loadImportJobs()])
}

onMounted(async () => {
  await loadBank()
  await Promise.all([loadQuestions(), loadTags(), loadImportJobs()])
})
</script>

<style scoped>
.back { margin: -4px 0 4px; }
.back :deep(.ant-btn) { padding-inline: 0; }
.stem {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  max-width: 640px;
}
.answer { font-weight: 600; color: var(--accent, #24945f); }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
.form-row { display: flex; gap: 24px; align-items: flex-start; }
.opt-row { display: flex; align-items: flex-start; gap: 10px; margin-bottom: 10px; }
.opt-row .opt-key { margin-top: 4px; }
.opt-main { flex: 1; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.opt-key {
  flex: none;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--ok-soft);
  color: var(--ok-text);
  font-weight: 600;
  font-size: 13px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.panel-body :deep(.ant-spin-nested-loading) { min-height: 40px; }
.filters {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  padding: 14px 16px 12px;
}
.import-file {
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.small { font-size: 12px; }
.danger { color: var(--bad-text); }
.tag-chip { margin-inline-end: 0; }
.tag-hint { margin-top: 4px; font-size: 12px; }
.merge-tip { margin: 0 0 12px; }
</style>
