<template>
  <a-modal
    :open="open"
    :title="result ? $t('importModal.resultTitle') : $t('importModal.title')"
    :width="840"
    :footer="null"
    :mask-closable="false"
    @cancel="close"
  >
    <template v-if="!result">
      <div v-if="!preview" class="import-form">
        <div class="target-row">
          <template v-if="bankId">
            <span class="muted">{{ $t('importModal.target') }}</span>
            <b>{{ bankName || $t('importModal.currentBank') }}</b>
          </template>
          <template v-else>
            <span class="muted">{{ $t('importModal.target') }}</span>
            <a-select v-model:value="mode" style="width: 132px">
              <a-select-option value="existing" :disabled="!banks?.length">
                {{ $t('importModal.modeExisting') }}
              </a-select-option>
              <a-select-option value="new" :disabled="!auth.mayCreateBank">
                {{ $t('importModal.modeNew') }}
              </a-select-option>
            </a-select>
            <a-select
              v-if="mode === 'existing'"
              v-model:value="selectedBankId"
              style="width: 240px"
              :placeholder="$t('importModal.selectBank')"
            >
              <a-select-option v-for="bank in writableBanks" :key="bank.id" :value="bank.id">
                {{ $t('importModal.bankOption', { name: bank.name, n: bank.questionCount }) }}
              </a-select-option>
            </a-select>
            <template v-else>
              <a-input
                v-model:value="newBankName"
                style="width: 240px"
                :maxlength="60"
                :placeholder="$t('importModal.newBankName')"
              />
              <a-checkbox v-if="auth.isAdmin" v-model:checked="newBankPublic">
                {{ $t('importModal.publicBank') }}
              </a-checkbox>
              <span v-if="auth.isAdmin && mode === 'new'" class="muted small">
                {{ $t('importModal.publicHint') }}
              </span>
            </template>
          </template>
        </div>

        <div class="template-row">
          <a-button size="small" :loading="downloadingTemplate" :disabled="!!busy" @click="downloadTemplate">
            {{ $t('importModal.downloadTemplate') }}
          </a-button>
          <span class="muted small">{{ $t('importModal.templateHint') }}</span>
        </div>

        <div
          class="dropzone"
          :class="{ active: dragActive, busy: busy === 'preview' }"
          @click="pickFile"
          @dragover.prevent="dragActive = true"
          @dragleave.prevent="dragActive = false"
          @drop.prevent="onDrop"
        >
          <input ref="fileInput" type="file" accept=".xlsx,.xlsm,.csv" hidden @change="onFileChange" />
          <div class="dz-mark">⇪</div>
          <div v-if="!file" class="dz-title">{{ $t('importModal.dropTitle') }}</div>
          <div v-else class="dz-title">
            {{ file.name }}
            <span class="muted">（{{ sizeText }}）</span>
          </div>
          <div class="muted small">{{ $t('importModal.dropHint') }}</div>
        </div>

        <div class="hint">{{ $t('importModal.formatHint') }}</div>

        <div class="image-row">
          <input
            ref="imageInput"
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            multiple
            hidden
            @change="onImagesChange"
          />
          <a-button size="small" :disabled="!!busy" @click="pickImages">
            {{ $t('importModal.pickImages') }}
          </a-button>
          <template v-if="images.length === 0">
            <span class="muted small">{{ $t('importModal.imageHint') }}</span>
          </template>
          <template v-else>
            <a-tag color="green">{{ $t('importModal.imageCount', { n: images.length }) }}</a-tag>
            <span class="muted small image-names">{{ imageNamesText }}</span>
            <a-button type="link" size="small" @click="clearImages">
              {{ $t('importModal.clear') }}
            </a-button>
          </template>
        </div>

        <div class="footer">
          <a-button @click="close">{{ $t('importModal.cancel') }}</a-button>
          <a-button
            type="primary"
            :loading="busy === 'preview'"
            :disabled="!file"
            @click="startPreview"
          >
            {{ $t('importModal.parse') }}
          </a-button>
        </div>
      </div>

      <div v-else class="preview">
        <div class="summary">
          <div class="file-line">
            <b>{{ preview.fileName }}</b>
            <span v-if="preview.sheet" class="muted">
              {{ $t('importModal.sheetBadge', { name: preview.sheet }) }}
            </span>
          </div>
          <div class="stats">
            <a-tag>{{ $t('importModal.tagTotal', { n: preview.total }) }}</a-tag>
            <a-tag color="green">{{ $t('importModal.tagValid', { n: preview.valid }) }}</a-tag>
            <a-tag v-if="preview.duplicate" color="orange">
              {{ $t('importModal.tagDuplicate', { n: preview.duplicate }) }}
            </a-tag>
            <a-tag v-if="preview.invalid" color="red">
              {{ $t('importModal.tagInvalid', { n: preview.invalid }) }}
            </a-tag>
            <a-tag v-if="preview.imageUploaded" color="geekblue">
              {{
                $t('importModal.tagImages', {
                  matched: preview.imageMatched,
                  uploaded: preview.imageUploaded,
                })
              }}
            </a-tag>
          </div>
        </div>

        <div class="mapping">
          <span v-for="col in preview.columns" :key="col.field" class="map-chip">
            {{ col.header }} → {{ colLabel(col.field) }}
          </span>
        </div>

        <a-collapse ghost class="mapping-editor">
          <a-collapse-panel key="mapping" :header="$t('importModal.mappingHeader')">
            <div class="map-form">
              <div class="map-line">
                <label>{{ $t('importModal.sheetField') }}</label>
                <a-select
                  v-if="preview.sheets?.length > 1"
                  v-model:value="sheet"
                  style="width: 200px"
                >
                  <a-select-option v-for="name in preview.sheets" :key="name" :value="name">
                    {{ name }}
                  </a-select-option>
                </a-select>
                <span v-else class="muted">{{ preview.sheet || $t('importModal.defaultSheet') }}</span>
                <label class="map-gap">{{ $t('importModal.headerRow') }}</label>
                <a-input-number v-model:value="headerRow" :min="1" :max="20" style="width: 80px" />
              </div>
              <div v-for="field in mappingFields" :key="field.key" class="map-line">
                <label>{{ field.label }}</label>
                <a-select
                  v-model:value="mappingDraft[field.key]"
                  style="width: 220px"
                  allow-clear
                  :placeholder="$t('importModal.noColumn')"
                >
                  <a-select-option
                    v-for="(header, index) in preview.headers"
                    :key="index"
                    :value="columnName(index)"
                  >
                    {{ columnName(index) }} · {{ header || $t('importModal.emptyHeader') }}
                  </a-select-option>
                </a-select>
              </div>
              <div class="map-actions">
                <a-button size="small" :loading="busy === 'preview'" @click="applyMapping">
                  {{ $t('importModal.applyMapping') }}
                </a-button>
                <span class="muted small">{{ $t('importModal.mappingHint') }}</span>
              </div>
            </div>
          </a-collapse-panel>
        </a-collapse>

        <a-alert
          v-for="warn in preview.warnings"
          :key="warn"
          type="warning"
          :message="warn"
          show-icon
          class="warn"
        />

        <a-table
          v-if="preview.sample.length"
          :data-source="preview.sample"
          :columns="sampleColumns"
          :pagination="false"
          row-key="row"
          size="small"
          :scroll="{ x: 760 }"
          class="sample"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'type'">
              <a-tag :color="record.type === 'multiple' ? 'purple' : 'blue'">
                {{ record.type === 'multiple' ? $t('importModal.typeMultiple') : $t('importModal.typeSingle') }}
              </a-tag>
            </template>
            <template v-else-if="column.key === 'stem'">
              <div class="sample-stem">{{ record.stem }}</div>
            </template>
            <template v-else-if="column.key === 'options'">
              <div v-for="opt in record.options" :key="opt.key" class="opt-line">
                {{ opt.key }}. {{ opt.content }}
              </div>
            </template>
            <template v-else-if="column.key === 'answer'">
              <span class="mono">{{ record.answer.join('') }}</span>
            </template>
          </template>
        </a-table>

        <a-collapse v-if="preview.errors.length" ghost class="errors">
          <a-collapse-panel
            :header="$t('importModal.errorsHeader', { n: preview.errors.length })"
            key="errors"
          >
            <div class="err-list">
              <div v-for="item in preview.errors" :key="item.row + item.message">
                {{ $t('importModal.errorRow', { n: item.row, msg: item.message }) }}
              </div>
            </div>
          </a-collapse-panel>
        </a-collapse>

        <div class="footer">
          <a-button @click="backToForm">{{ $t('importModal.backToForm') }}</a-button>
          <a-radio-group v-model:value="policy" size="small" class="policy">
            <a-radio-button value="skip">{{ $t('importModal.policySkip') }}</a-radio-button>
            <a-radio-button value="keep">{{ $t('importModal.policyKeep') }}</a-radio-button>
          </a-radio-group>
          <a-button
            type="primary"
            :loading="busy === 'commit'"
            :disabled="preview.valid === 0 && policy === 'skip'"
            @click="commit"
          >
            {{ commitLabel }}
          </a-button>
        </div>
      </div>
    </template>

    <div v-else class="result">
      <a-result
        status="success"
        :title="$t('importModal.done')"
        :sub-title="
          $t('importModal.doneSubtitle', {
            inserted: result.inserted,
            skipped: result.skipped,
            failed: result.failed,
          })
        "
      />
      <div v-if="result.errors.length" class="err-list">
        <div v-for="item in result.errors" :key="item.row + item.message">
          {{ $t('importModal.errorRow', { n: item.row, msg: item.message }) }}
        </div>
      </div>
      <div class="footer">
        <a-popconfirm
          v-if="!reverted"
          :title="$t('importModal.undoConfirm')"
          :ok-text="$t('importModal.undo')"
          :cancel-text="$t('importModal.cancel')"
          @confirm="undo"
        >
          <a-button danger ghost :loading="busy === 'revert'">
            {{ $t('importModal.undoBtn') }}
          </a-button>
        </a-popconfirm>
        <span v-else class="muted">{{ $t('importModal.undone') }}</span>
        <a-space>
          <a-button v-if="result.failed" @click="downloadErrors">
            {{ $t('importModal.downloadErrors') }}
          </a-button>
          <a-button type="primary" @click="close">{{ $t('importModal.finish') }}</a-button>
        </a-space>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { banksApi, type Bank, type ImportPreview, type ImportResult } from '@/api/banks'
import { isTauri } from '@/api/request'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  open: boolean
  bankId?: number
  bankName?: string
  banks?: Bank[]
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'imported', bankId: number): void
}>()

const { t } = useI18n()

const fileInput = ref<HTMLInputElement | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const images = ref<File[]>([])
const mode = ref<'existing' | 'new'>('existing')
const selectedBankId = ref<number | undefined>()
const newBankName = ref('')
const newBankPublic = ref(false)
// 记录本次导入临时建出的题库，供失败/取消时回收。
const createdBankId = ref<number | null>(null)
const committed = ref(false)
const file = ref<File | null>(null)
const preview = ref<ImportPreview | null>(null)
const result = ref<ImportResult | null>(null)
const busy = ref<'' | 'preview' | 'commit' | 'revert'>('')
const downloadingTemplate = ref(false)
const dragActive = ref(false)
const targetId = ref<number | undefined>(props.bankId)
const policy = ref<'skip' | 'keep'>('skip')
const sheet = ref<string | undefined>()
const headerRow = ref(1)
const mappingDraft = reactive<Record<string, string | undefined>>({})
const auth = useAuthStore()

type MappingField = { key: string; labelKey: string; letter?: string }

const MAPPING_FIELDS: MappingField[] = [
  { key: 'type', labelKey: 'importModal.fieldType' },
  { key: 'stem', labelKey: 'importModal.fieldStem' },
  { key: 'options', labelKey: 'importModal.fieldOptions' },
  { key: 'answer', labelKey: 'importModal.fieldAnswer' },
  { key: 'analysis', labelKey: 'importModal.fieldAnalysis' },
  { key: 'difficulty', labelKey: 'importModal.fieldDifficulty' },
  { key: 'tags', labelKey: 'importModal.fieldTags' },
  { key: 'source', labelKey: 'importModal.fieldSource' },
  { key: 'stemimage', labelKey: 'importModal.fieldStemImage' },
  ...['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'].map((letter) => ({
    key: `option_${letter}`,
    labelKey: 'importModal.fieldOption',
    letter,
  })),
]

const mappingFields = computed(() =>
  MAPPING_FIELDS.map((field) => ({
    key: field.key,
    label: field.letter ? t(field.labelKey, { letter: field.letter }) : t(field.labelKey),
  })),
)

function columnName(index: number) {
  let n = index + 1
  let out = ''
  while (n > 0) {
    n -= 1
    out = String.fromCharCode(65 + (n % 26)) + out
    n = Math.floor(n / 26)
  }
  return out
}

const commitLabel = computed(() => {
  if (!preview.value) return t('importModal.commit')
  const count = policy.value === 'keep'
    ? preview.value.valid + preview.value.duplicate
    : preview.value.valid
  return count > 0 ? t('importModal.commitCount', { n: count }) : t('importModal.commitEmpty')
})

const writableBanks = computed(() => (props.banks ?? []).filter((b) => b.canEdit))
const reverted = ref(false)

const sizeText = computed(() => {
  if (!file.value) return ''
  const mb = file.value.size / 1024 / 1024
  return mb >= 1 ? `${mb.toFixed(1)} MB` : `${Math.max(1, Math.round(file.value.size / 1024))} KB`
})

const imageNamesText = computed(() => {
  const names = images.value.slice(0, 3).map((f) => f.name)
  const more = images.value.length > 3 ? t('importModal.imageNameMore') : ''
  return names.join(t('importModal.imageNameJoin')) + more
})

const FIELD_LABEL_KEYS: Record<string, string> = {
  type: 'importModal.fieldType',
  stem: 'importModal.fieldStem',
  options: 'importModal.fieldOptions',
  answer: 'importModal.fieldAnswer',
  analysis: 'importModal.fieldAnalysis',
  difficulty: 'importModal.fieldDifficulty',
  tags: 'importModal.fieldTags',
  stemimage: 'importModal.fieldStemImageShort',
  source: 'importModal.fieldSource',
}

function colLabel(field: string) {
  const letter = field.slice(7)
  if (field.startsWith('option_')) return t('importModal.optionLabel', { letter })
  if (field.startsWith('option:')) return t('importModal.optionLabel', { letter })
  const key = FIELD_LABEL_KEYS[field]
  return key ? t(key) : field
}

const sampleColumns = computed(() => [
  { title: t('importModal.colRow'), key: 'row', dataIndex: 'row', width: 56 },
  { title: t('importModal.colType'), key: 'type', width: 64 },
  { title: t('importModal.colStem'), key: 'stem' },
  { title: t('importModal.colOptions'), key: 'options', width: 360 },
  { title: t('importModal.colAnswer'), key: 'answer', width: 64 },
])

function reset() {
  file.value = null
  preview.value = null
  result.value = null
  busy.value = ''
  dragActive.value = false
  reverted.value = false
  newBankName.value = ''
  newBankPublic.value = false
  createdBankId.value = null
  committed.value = false
  policy.value = 'skip'
  sheet.value = undefined
  headerRow.value = 1
  images.value = []
  for (const key of Object.keys(mappingDraft)) delete mappingDraft[key]
  targetId.value = props.bankId
  selectedBankId.value = writableBanks.value[0]?.id
  mode.value = writableBanks.value.length ? 'existing' : 'new'
  if (fileInput.value) fileInput.value.value = ''
  if (imageInput.value) imageInput.value.value = ''
}

watch(
  () => props.open,
  (open) => {
    if (open) reset()
    else void discardEmptyBank()
  },
)

/**
 * 「新建题库」是先生成题库再预览的：预览失败或用户中途关掉弹窗时，
 * 把这个空题库删掉，避免列表里留下一本（可能是公共的）空库。
 */
async function discardEmptyBank() {
  const id = createdBankId.value
  if (!id || committed.value) return
  createdBankId.value = null
  try {
    await banksApi.remove(id)
  } catch {
    /* 回收失败不影响主流程，用户可手动删除 */
  }
}

function pickFile() {
  if (busy.value) return
  fileInput.value?.click()
}

function pickImages() {
  if (busy.value) return
  imageInput.value?.click()
}

function onImagesChange(event: Event) {
  const input = event.target as HTMLInputElement
  addImages(Array.from(input.files ?? []))
  input.value = ''
}

function addImages(list: File[]) {
  if (!list.length) return
  const accepted: File[] = []
  for (const f of list) {
    if (!/\.(png|jpe?g|webp|gif)$/i.test(f.name)) continue
    if (f.size > 8 * 1024 * 1024) continue
    if (images.value.some((i) => i.name === f.name && i.size === f.size)) continue
    accepted.push(f)
  }
  if (accepted.length < list.length) {
    message.warning(t('importModal.msg.imageIgnored'))
  }
  if (images.value.length + accepted.length > 300) {
    message.warning(t('importModal.msg.imageTooMany'))
  }
  images.value = images.value.concat(accepted).slice(0, 300)
}

function clearImages() {
  images.value = []
  if (imageInput.value) imageInput.value.value = ''
}

async function downloadTemplate() {
  if (downloadingTemplate.value) return
  downloadingTemplate.value = true
  const fileName = t('importModal.templateFileName')
  try {
    if (isTauri()) {
      const { invoke } = await import('@tauri-apps/api/core')
      const path = await invoke<string | null>('pick_export_path', {
        fileName,
        title: t('importModal.templateSaveTitle'),
      })
      if (!path) return
      const res = await banksApi.importTemplateTo(path)
      message.success(t('importModal.msg.templateSaved', { path: res.path }))
      return
    }
    const blob = await banksApi.importTemplateFile()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success(t('importModal.msg.templateStarted'))
  } finally {
    downloadingTemplate.value = false
  }
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  setFile(input.files?.[0] ?? null)
  input.value = ''
}

function onDrop(event: DragEvent) {
  dragActive.value = false
  setFile(event.dataTransfer?.files?.[0] ?? null)
}

function setFile(f: File | null) {
  if (!f) return
  if (!/\.(xlsx|xlsm|csv)$/i.test(f.name)) {
    message.warning(t('importModal.msg.fileType'))
    return
  }
  if (f.size > 20 * 1024 * 1024) {
    message.warning(t('importModal.msg.fileSize'))
    return
  }
  file.value = f
  preview.value = null
  result.value = null
  sheet.value = undefined
  headerRow.value = 1
  for (const key of Object.keys(mappingDraft)) delete mappingDraft[key]
  if (!props.bankId && mode.value === 'new' && !newBankName.value) {
    newBankName.value = f.name.replace(/\.[^.]+$/, '')
  }
}

async function startPreview() {
  if (!file.value) return
  let bankID = props.bankId
  if (!bankID) {
    if (mode.value === 'existing') {
      if (!selectedBankId.value) {
        message.warning(t('importModal.msg.bankRequired'))
        return
      }
      bankID = selectedBankId.value
    } else {
      const name = newBankName.value.trim() || file.value.name.replace(/\.[^.]+$/, '')
      if (!name) {
        message.warning(t('importModal.msg.bankNameRequired'))
        return
      }
      busy.value = 'preview'
      try {
        const bank = await banksApi.create({
          name,
          description: t('importModal.msg.bankDescription', { name: file.value.name }),
          public: auth.isAdmin && newBankPublic.value,
        })
        createdBankId.value = bank.id
        committed.value = false
        bankID = bank.id
        targetId.value = bank.id
      } catch {
        return
      } finally {
        busy.value = ''
      }
    }
  }
  targetId.value = bankID
  const ok = await runPreview(bankID!)
  if (!ok && createdBankId.value === bankID) {
    await discardEmptyBank()
    targetId.value = props.bankId
    message.warning(t('importModal.msg.previewFailed'))
  }
}

function previewOptions() {
  const mapping: Record<string, string> = {}
  for (const [key, value] of Object.entries(mappingDraft)) {
    if (value) mapping[key] = value
  }
  return {
    sheet: sheet.value,
    headerRow: headerRow.value || undefined,
    mapping: Object.keys(mapping).length ? mapping : undefined,
  }
}

async function runPreview(bankID: number) {
  if (!file.value) return false
  busy.value = 'preview'
  try {
    const res = await banksApi.importPreview(bankID, file.value, previewOptions(), images.value)
    preview.value = res
    sheet.value = res.sheet || undefined
    headerRow.value = res.headerRow || 1
    for (const key of Object.keys(mappingDraft)) delete mappingDraft[key]
    for (const [key, value] of Object.entries(res.mapping || {})) mappingDraft[key] = value
    return true
  } catch {
    preview.value = null
    return false
  } finally {
    busy.value = ''
  }
}

async function applyMapping() {
  if (!preview.value || !targetId.value) return
  await runPreview(targetId.value)
}

function backToForm() {
  preview.value = null
}

async function commit() {
  if (!preview.value || !targetId.value) return
  busy.value = 'commit'
  try {
    result.value = await banksApi.importCommit(targetId.value, preview.value.token, policy.value)
    committed.value = true
    emit('imported', targetId.value)
    message.success(t('importModal.msg.imported', { n: result.value.inserted }))
  } catch {
    preview.value = null
  } finally {
    busy.value = ''
  }
}

async function downloadErrors() {
  if (!result.value) return
  const blob = await banksApi.importErrorsCsv(result.value.jobId)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = t('importModal.errorsFileName', { id: result.value.jobId })
  link.click()
  URL.revokeObjectURL(url)
}

async function undo() {
  if (!result.value) return
  busy.value = 'revert'
  try {
    const res = await banksApi.revertImport(result.value.jobId)
    reverted.value = true
    emit('imported', targetId.value!)
    message.success(t('importModal.msg.undone', { n: res.reverted }))
  } finally {
    busy.value = ''
  }
}

function close() {
  emit('update:open', false)
}
</script>

<style scoped>
.target-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.template-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.dropzone {
  border: 1px dashed var(--border-strong);
  border-radius: 10px;
  padding: 26px 20px;
  text-align: center;
  cursor: pointer;
  background: var(--surface-2);
  transition: border-color 0.2s, background 0.2s;
}
.dropzone:hover,
.dropzone.active {
  border-color: var(--primary);
  background: var(--ok-soft);
}
.dropzone.busy { pointer-events: none; opacity: 0.7; }
.dz-mark {
  width: 38px;
  height: 38px;
  margin: 0 auto 8px;
  border-radius: 50%;
  background: var(--ok-soft);
  color: var(--ok-text);
  font-size: 20px;
  line-height: 38px;
}
.dz-title { font-weight: 600; margin-bottom: 4px; word-break: break-all; }
.small { font-size: 12px; }
.hint {
  margin-top: 14px;
  color: var(--text2);
  font-size: 12px;
  line-height: 1.7;
}
.image-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  background: var(--surface-2);
}
.image-names {
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 20px;
}
.summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.file-line { word-break: break-all; }
.stats { display: flex; gap: 6px; flex-wrap: wrap; }
.mapping { display: flex; flex-wrap: wrap; gap: 6px; margin: 12px 0; }
.map-chip {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--ok-soft);
  color: var(--ok-text);
}
.warn { margin-bottom: 8px; }
.mapping-editor { margin: -6px 0 8px; }
.mapping-editor :deep(.ant-collapse-header) { padding-left: 0; font-size: 12px; color: var(--text2); }
.map-form { display: flex; flex-direction: column; gap: 8px; }
.map-line { display: flex; align-items: center; gap: 10px; }
.map-line label { width: 110px; font-size: 12px; color: var(--text2); }
.map-gap { width: auto !important; margin-left: 10px; }
.map-actions { display: flex; align-items: center; gap: 12px; margin-top: 4px; }
.policy { margin-left: auto; }
.sample { margin-top: 10px; }
.sample-stem {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.opt-line {
  font-size: 12px;
  color: var(--text2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 360px;
}
.errors { margin-top: 4px; }
.err-list {
  max-height: 160px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.8;
  color: var(--bad-text);
  background: var(--bad-soft);
  border-radius: 6px;
  padding: 8px 10px;
}
.result .footer { justify-content: space-between; }
</style>
