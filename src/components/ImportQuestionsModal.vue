<template>
  <a-modal
    :open="open"
    :title="result ? '导入结果' : '导入题目'"
    :width="840"
    :footer="null"
    :mask-closable="false"
    @cancel="close"
  >
    <template v-if="!result">
      <div v-if="!preview" class="import-form">
        <div class="target-row">
          <template v-if="bankId">
            <span class="muted">导入到</span>
            <b>{{ bankName || '当前题库' }}</b>
          </template>
          <template v-else>
            <span class="muted">导入到</span>
            <a-select v-model:value="mode" style="width: 132px">
              <a-select-option value="existing" :disabled="!banks?.length">已有题库</a-select-option>
              <a-select-option value="new">新建题库</a-select-option>
            </a-select>
            <a-select
              v-if="mode === 'existing'"
              v-model:value="selectedBankId"
              style="width: 240px"
              placeholder="请选择题库"
            >
              <a-select-option v-for="bank in writableBanks" :key="bank.id" :value="bank.id">
                {{ bank.name }}（{{ bank.questionCount }} 题）
              </a-select-option>
            </a-select>
            <a-input
              v-else
              v-model:value="newBankName"
              style="width: 240px"
              :maxlength="60"
              placeholder="新题库名称"
            />
          </template>
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
          <div v-if="!file" class="dz-title">点击选择或拖拽 Excel / CSV 文件到此处</div>
          <div v-else class="dz-title">
            {{ file.name }}
            <span class="muted">（{{ sizeText }}）</span>
          </div>
          <div class="muted small">支持 .xlsx / .xlsm / .csv，单文件不超过 20 MB</div>
        </div>

        <div class="hint">
          按表头识别列：类别/题型、题目/题干、选项（A— 分行，或选项A…选项H 分列）、答案、解析、难度、标签。
          判断题自动转为单选；导入前会列出列映射，可手动调整；默认跳过与题库内已有题目重复的行。
        </div>

        <div class="footer">
          <a-button @click="close">取消</a-button>
          <a-button
            type="primary"
            :loading="busy === 'preview'"
            :disabled="!file"
            @click="startPreview"
          >
            解析文件
          </a-button>
        </div>
      </div>

      <div v-else class="preview">
        <div class="summary">
          <div class="file-line">
            <b>{{ preview.fileName }}</b>
            <span v-if="preview.sheet" class="muted">· 工作表「{{ preview.sheet }}」</span>
          </div>
          <div class="stats">
            <a-tag>共 {{ preview.total }} 行</a-tag>
            <a-tag color="green">可导入 {{ preview.valid }}</a-tag>
            <a-tag v-if="preview.duplicate" color="orange">重复跳过 {{ preview.duplicate }}</a-tag>
            <a-tag v-if="preview.invalid" color="red">错误 {{ preview.invalid }}</a-tag>
          </div>
        </div>

        <div class="mapping">
          <span v-for="col in preview.columns" :key="col.field" class="map-chip">
            {{ col.header }} → {{ colLabel(col.field) }}
          </span>
        </div>

        <a-collapse ghost class="mapping-editor">
          <a-collapse-panel key="mapping" header="列映射不合适？手动调整后重新解析">
            <div class="map-form">
              <div class="map-line">
                <label>工作表</label>
                <a-select
                  v-if="preview.sheets?.length > 1"
                  v-model:value="sheet"
                  style="width: 200px"
                >
                  <a-select-option v-for="name in preview.sheets" :key="name" :value="name">
                    {{ name }}
                  </a-select-option>
                </a-select>
                <span v-else class="muted">{{ preview.sheet || '默认' }}</span>
                <label class="map-gap">表头行</label>
                <a-input-number v-model:value="headerRow" :min="1" :max="20" style="width: 80px" />
              </div>
              <div v-for="field in mappingFields" :key="field.key" class="map-line">
                <label>{{ field.label }}</label>
                <a-select
                  v-model:value="mappingDraft[field.key]"
                  style="width: 220px"
                  allow-clear
                  placeholder="不使用此列"
                >
                  <a-select-option
                    v-for="(header, index) in preview.headers"
                    :key="index"
                    :value="columnName(index)"
                  >
                    {{ columnName(index) }} · {{ header || '（空表头）' }}
                  </a-select-option>
                </a-select>
              </div>
              <div class="map-actions">
                <a-button size="small" :loading="busy === 'preview'" @click="applyMapping">
                  应用映射并重新解析
                </a-button>
                <span class="muted small">题干与答案是必填列；选项可选合并列或至少两个分列</span>
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
                {{ record.type === 'multiple' ? '多选' : '单选' }}
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
          <a-collapse-panel :header="`错误明细（${preview.errors.length} 条，错误行不会入库）`" key="errors">
            <div class="err-list">
              <div v-for="item in preview.errors" :key="item.row + item.message">
                第 {{ item.row }} 行：{{ item.message }}
              </div>
            </div>
          </a-collapse-panel>
        </a-collapse>

        <div class="footer">
          <a-button @click="backToForm">返回重选</a-button>
          <a-radio-group v-model:value="policy" size="small" class="policy">
            <a-radio-button value="skip">跳过重复</a-radio-button>
            <a-radio-button value="keep">保留重复</a-radio-button>
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
        title="导入完成"
        :sub-title="`新增 ${result.inserted} 道 · 跳过重复 ${result.skipped} 道 · 失败 ${result.failed} 道`"
      />
      <div v-if="result.errors.length" class="err-list">
        <div v-for="item in result.errors" :key="item.row + item.message">
          第 {{ item.row }} 行：{{ item.message }}
        </div>
      </div>
      <div class="footer">
        <a-popconfirm
          v-if="!reverted"
          title="撤销只删除本次导入且未被编辑的题目，确定撤销吗？"
          ok-text="撤销"
          cancel-text="取消"
          @confirm="undo"
        >
          <a-button danger ghost :loading="busy === 'revert'">撤销本次导入</a-button>
        </a-popconfirm>
        <span v-else class="muted">本次导入已撤销</span>
        <a-space>
          <a-button v-if="result.failed" @click="downloadErrors">下载错误明细</a-button>
          <a-button type="primary" @click="close">完成</a-button>
        </a-space>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { banksApi, type Bank, type ImportPreview, type ImportResult } from '@/api/banks'
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

const fileInput = ref<HTMLInputElement | null>(null)
const mode = ref<'existing' | 'new'>('existing')
const selectedBankId = ref<number | undefined>()
const newBankName = ref('')
const file = ref<File | null>(null)
const preview = ref<ImportPreview | null>(null)
const result = ref<ImportResult | null>(null)
const busy = ref<'' | 'preview' | 'commit' | 'revert'>('')
const dragActive = ref(false)
const targetId = ref<number | undefined>(props.bankId)
const policy = ref<'skip' | 'keep'>('skip')
const sheet = ref<string | undefined>()
const headerRow = ref(1)
const mappingDraft = reactive<Record<string, string | undefined>>({})
const auth = useAuthStore()

const mappingFields = [
  { key: 'type', label: '题型' },
  { key: 'stem', label: '题干' },
  { key: 'options', label: '选项（合并列）' },
  { key: 'answer', label: '答案' },
  { key: 'analysis', label: '解析' },
  { key: 'difficulty', label: '难度' },
  { key: 'tags', label: '标签' },
  { key: 'stemimage', label: '题干图片' },
  { key: 'option_A', label: '选项 A 列' },
  { key: 'option_B', label: '选项 B 列' },
  { key: 'option_C', label: '选项 C 列' },
  { key: 'option_D', label: '选项 D 列' },
  { key: 'option_E', label: '选项 E 列' },
  { key: 'option_F', label: '选项 F 列' },
  { key: 'option_G', label: '选项 G 列' },
  { key: 'option_H', label: '选项 H 列' },
]

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
  if (!preview.value) return '导入'
  const count = policy.value === 'keep'
    ? preview.value.valid + preview.value.duplicate
    : preview.value.valid
  return count > 0 ? `开始导入 ${count} 道` : '没有可导入的题目'
})

const writableBanks = computed(() =>
  (props.banks ?? []).filter(
    (b) => auth.isAdmin || (b.ownerId !== null && b.ownerId !== undefined),
  ),
)
const reverted = ref(false)

const sizeText = computed(() => {
  if (!file.value) return ''
  const mb = file.value.size / 1024 / 1024
  return mb >= 1 ? `${mb.toFixed(1)} MB` : `${Math.max(1, Math.round(file.value.size / 1024))} KB`
})

const fieldLabels: Record<string, string> = {
  type: '题型',
  stem: '题干',
  options: '选项（合并列）',
  answer: '答案',
  analysis: '解析',
  difficulty: '难度',
  tags: '标签',
  stemimage: '题图',
  source: '来源',
}

function colLabel(field: string) {
  if (field.startsWith('option_')) return `选项 ${field.slice(7)}`
  if (field.startsWith('option:')) return `选项 ${field.slice(7)}`
  return fieldLabels[field] ?? field
}

const sampleColumns = [
  { title: '行', key: 'row', dataIndex: 'row', width: 56 },
  { title: '题型', key: 'type', width: 64 },
  { title: '题干', key: 'stem' },
  { title: '选项', key: 'options', width: 360 },
  { title: '答案', key: 'answer', width: 64 },
]

function reset() {
  file.value = null
  preview.value = null
  result.value = null
  busy.value = ''
  dragActive.value = false
  reverted.value = false
  newBankName.value = ''
  policy.value = 'skip'
  sheet.value = undefined
  headerRow.value = 1
  for (const key of Object.keys(mappingDraft)) delete mappingDraft[key]
  targetId.value = props.bankId
  selectedBankId.value = writableBanks.value[0]?.id
  mode.value = writableBanks.value.length ? 'existing' : 'new'
  if (fileInput.value) fileInput.value.value = ''
}

watch(
  () => props.open,
  (open) => {
    if (open) reset()
  },
)

function pickFile() {
  if (busy.value) return
  fileInput.value?.click()
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
    message.warning('仅支持 .xlsx / .xlsm / .csv 文件')
    return
  }
  if (f.size > 20 * 1024 * 1024) {
    message.warning('文件不能超过 20 MB')
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
        message.warning('请选择导入到哪个题库')
        return
      }
      bankID = selectedBankId.value
    } else {
      const name = newBankName.value.trim() || file.value.name.replace(/\.[^.]+$/, '')
      if (!name) {
        message.warning('请填写新题库名称')
        return
      }
      busy.value = 'preview'
      try {
        const bank = await banksApi.create({ name, description: `由 ${file.value.name} 导入` })
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
  await runPreview(bankID!)
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
  if (!file.value) return
  busy.value = 'preview'
  try {
    const res = await banksApi.importPreview(bankID, file.value, previewOptions())
    preview.value = res
    sheet.value = res.sheet || undefined
    headerRow.value = res.headerRow || 1
    for (const key of Object.keys(mappingDraft)) delete mappingDraft[key]
    for (const [key, value] of Object.entries(res.mapping || {})) mappingDraft[key] = value
  } catch {
    preview.value = null
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
    emit('imported', targetId.value)
    message.success(`已导入 ${result.value.inserted} 道题`)
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
  link.download = `导入错误-${result.value.jobId}.csv`
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
    message.success(`已撤销 ${res.reverted} 道题`)
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
.dropzone {
  border: 1px dashed #cfd8d3;
  border-radius: 10px;
  padding: 26px 20px;
  text-align: center;
  cursor: pointer;
  background: #fafcfb;
  transition: border-color 0.2s, background 0.2s;
}
.dropzone:hover,
.dropzone.active {
  border-color: #24945f;
  background: #f2faf6;
}
.dropzone.busy { pointer-events: none; opacity: 0.7; }
.dz-mark {
  width: 38px;
  height: 38px;
  margin: 0 auto 8px;
  border-radius: 50%;
  background: #e8f5ee;
  color: #1c7a4e;
  font-size: 20px;
  line-height: 38px;
}
.dz-title { font-weight: 600; margin-bottom: 4px; word-break: break-all; }
.small { font-size: 12px; }
.hint {
  margin-top: 14px;
  color: #6b7a73;
  font-size: 12px;
  line-height: 1.7;
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
  background: #eef7f2;
  color: #1c7a4e;
}
.warn { margin-bottom: 8px; }
.mapping-editor { margin: -6px 0 8px; }
.mapping-editor :deep(.ant-collapse-header) { padding-left: 0; font-size: 12px; color: #4a5a52; }
.map-form { display: flex; flex-direction: column; gap: 8px; }
.map-line { display: flex; align-items: center; gap: 10px; }
.map-line label { width: 110px; font-size: 12px; color: #4a5a52; }
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
  color: #4a5a52;
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
  color: #b04a3f;
  background: #fdf6f5;
  border-radius: 6px;
  padding: 8px 10px;
}
.result .footer { justify-content: space-between; }
</style>
