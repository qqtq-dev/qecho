<template>
  <div class="page">
    <div class="back">
      <a-button type="link" size="small" @click="router.push('/banks')">← 返回题库列表</a-button>
    </div>
    <PageHeader :title="bank?.name || '题库详情'" :subtitle="bank?.description || '维护题目、答案与解析'">
      <template #actions>
        <template v-if="auth.mayExportImport">
          <a-button :loading="exporting" :disabled="total === 0" @click="exportBank">导出题目</a-button>
          <a-button :loading="exportingImages" @click="exportImages">导出图片</a-button>
        </template>
        <template v-if="canManage">
          <a-button @click="openEditBank">编辑题库</a-button>
          <a-button v-if="bank && !bank.isPublic" @click="sharingOpen = true">共享</a-button>
        </template>
        <template v-if="canEdit">
          <a-button v-if="auth.mayExportImport" @click="importOpen = true">导入题目</a-button>
          <a-button type="primary" @click="openCreateQuestion">添加题目</a-button>
        </template>
        <a-tag v-if="!canEdit" color="default">只读题库</a-tag>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>题目</h3>
        <span class="muted">共 {{ total }} 道</span>
      </div>
      <div class="filters">
        <a-input-search
          v-model:value="filters.keyword"
          placeholder="搜索题干或解析"
          allow-clear
          style="width: 240px"
          @search="applyFilters"
        />
        <a-select v-model:value="filters.type" style="width: 110px" @change="applyFilters">
          <a-select-option value="">全部题型</a-select-option>
          <a-select-option value="single">单选</a-select-option>
          <a-select-option value="multiple">多选</a-select-option>
        </a-select>
        <a-select v-model:value="filters.difficulty" style="width: 110px" @change="applyFilters">
          <a-select-option :value="0">全部难度</a-select-option>
          <a-select-option v-for="d in 5" :key="d" :value="d">难度 {{ d }}</a-select-option>
        </a-select>
        <a-select
          v-model:value="filters.tag"
          style="width: 150px"
          placeholder="全部标签"
          allow-clear
          @change="applyFilters"
        >
          <a-select-option v-for="tag in bankTags" :key="tag.id" :value="tag.name">
            {{ tag.name }}（{{ tag.questionCount }}）
          </a-select-option>
        </a-select>
        <a-button v-if="canManage" type="link" size="small" @click="openTagManager">标签管理</a-button>
      </div>
      <div class="panel-body" :class="{ 'empty-state': !loading && items.length === 0 }">
        <a-spin :spinning="loading">
          <a-empty v-if="!loading && items.length === 0" description="这个题库还没有题目">
            <a-button v-if="canEdit" type="primary" ghost @click="openCreateQuestion">添加第一道题</a-button>
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
                    {{ record.type === 'multiple' ? '多选' : '单选' }}
                  </a-tag>
                </template>
                <template v-else-if="column.key === 'stem'">
                  <div class="stem">{{ record.stem }}</div>
                  <div v-if="record.source" class="muted">来源：{{ record.source }}</div>
                  <div v-if="hasImage(record)" class="muted">含图片</div>
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
                    <a-button type="link" size="small" @click="openEditQuestion(record)">编辑</a-button>
                    <a-popconfirm
                      title="确定删除这道题吗？"
                      ok-text="删除"
                      cancel-text="取消"
                      @confirm="removeQuestion(record)"
                    >
                      <a-button type="link" size="small" danger>删除</a-button>
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
        <h3>导入记录</h3>
        <span class="muted">最近 {{ importJobs.length }} 次，7 天内可撤销</span>
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
              <span>新增 {{ record.inserted }}</span>
              <span class="muted"> · 跳过 {{ record.skipped }}</span>
              <span v-if="record.failed" class="danger"> · 失败 {{ record.failed }}</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="record.status === 'reverted' ? 'default' : record.failed ? 'orange' : 'green'">
                {{ record.status === 'reverted' ? '已撤销' : record.failed ? '部分失败' : '已完成' }}
              </a-tag>
              <span v-if="record.status !== 'reverted'" class="muted small">
                · 现存 {{ record.remaining }}
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
                  错误明细
                </a-button>
                <a-popconfirm
                  v-if="record.canRevert"
                  title="撤销将删除本次导入且未编辑过的题目，确定吗？"
                  ok-text="撤销"
                  cancel-text="取消"
                  @confirm="revertJob(record)"
                >
                  <a-button type="link" size="small" danger>撤销</a-button>
                </a-popconfirm>
                <span v-else-if="record.status !== 'reverted'" class="muted small">不可撤销</span>
              </a-space>
            </template>
          </template>
        </a-table>
      </div>
    </section>

    <a-modal
      v-model:open="tagModalOpen"
      title="标签管理"
      :footer="null"
      :width="620"
    >
      <div class="tag-manage">
        <a-empty v-if="allTags.length === 0" description="还没有标签，可在编辑题目时添加" />
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
              <span class="muted">{{ record.questionCount }} 道</span>
            </template>
            <template v-else-if="column.key === 'actions'">
              <a-space>
                <template v-if="editingTagId === record.id">
                  <a-button type="link" size="small" @click="saveTagRename(record)">保存</a-button>
                  <a-button type="link" size="small" @click="cancelTagRename">取消</a-button>
                </template>
                <template v-else>
                  <a-button type="link" size="small" @click="startTagRename(record)">重命名</a-button>
                  <a-button
                    type="link"
                    size="small"
                    :disabled="allTags.length < 2"
                    @click="openMerge(record)"
                  >
                    合并
                  </a-button>
                  <a-popconfirm
                    :title="`删除标签「${record.name}」？只解除题目标签，不会删除题目。`"
                    ok-text="删除"
                    cancel-text="取消"
                    @confirm="removeTag(record)"
                  >
                    <a-button type="link" size="small" danger>删除</a-button>
                  </a-popconfirm>
                </template>
              </a-space>
            </template>
          </template>
        </a-table>
      </div>
    </a-modal>

    <a-modal
      v-model:open="mergeModalOpen"
      :title="`合并标签「${mergeSource?.name ?? ''}」`"
      :confirm-loading="mergeSaving"
      ok-text="确认合并"
      cancel-text="取消"
      :width="420"
      @ok="confirmMerge"
    >
      <p class="muted merge-tip">合并后该标签下的题目会转到目标标签，原标签将被删除。</p>
      <a-select v-model:value="mergeTargetId" style="width: 100%" placeholder="选择合并到的标签">
        <a-select-option v-for="t in mergeTargets" :key="t.id" :value="t.id">
          {{ t.name }}（{{ t.questionCount }} 道）
        </a-select-option>
      </a-select>
    </a-modal>

    <a-modal
      v-model:open="bankModalOpen"
      title="编辑题库"
      :confirm-loading="bankSaving"
      ok-text="保存"
      cancel-text="取消"
      :width="460"
      @ok="saveBank"
    >
      <a-form layout="vertical">
        <a-form-item label="题库名称" required>
          <a-input v-model:value="bankForm.name" :maxlength="60" />
        </a-form-item>
        <a-form-item label="说明">
          <a-textarea v-model:value="bankForm.description" :rows="3" :maxlength="200" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="qModalOpen"
      :title="editingQuestion ? '编辑题目' : '添加题目'"
      :confirm-loading="qSaving"
      ok-text="保存"
      cancel-text="取消"
      :width="720"
      :mask-closable="false"
      @ok="saveQuestion"
    >
      <a-form layout="vertical">
        <div class="form-row">
          <a-form-item label="题型" required>
            <a-radio-group v-model:value="qForm.type" @change="onTypeChange">
              <a-radio-button value="single">单选题</a-radio-button>
              <a-radio-button value="multiple">多选题</a-radio-button>
            </a-radio-group>
          </a-form-item>
          <a-form-item label="难度">
            <a-select v-model:value="qForm.difficulty" style="width: 120px">
              <a-select-option v-for="d in 5" :key="d" :value="d">难度 {{ d }}</a-select-option>
            </a-select>
          </a-form-item>
        </div>

        <a-form-item label="题干" required>
          <a-textarea
            v-model:value="qForm.stem"
            :rows="3"
            :maxlength="2000"
            placeholder="请输入题干内容"
          />
        </a-form-item>

        <a-form-item label="题目图片">
          <ImageUploadField
            v-model="qForm.stemImage"
            hint="支持 PNG / JPEG / WebP / GIF，单张不超过 8 MB"
          />
        </a-form-item>

        <a-form-item label="选项" required>
          <div v-for="(opt, idx) in qForm.options" :key="opt.key" class="opt-row">
            <span class="opt-key">{{ opt.key }}</span>
            <div class="opt-main">
              <a-input
                v-model:value="opt.content"
                :placeholder="`选项 ${opt.key} 内容`"
                :maxlength="500"
              />
              <ImageUploadField v-model="opt.image" />
            </div>
            <a-button
              type="text"
              danger
              size="small"
              :disabled="qForm.options.length <= 2"
              @click="removeOption(idx)"
            >
              移除
            </a-button>
          </div>
          <a-button
            type="dashed"
            size="small"
            :disabled="qForm.options.length >= 8"
            @click="addOption"
          >
            + 添加选项
          </a-button>
        </a-form-item>

        <a-form-item :label="qForm.type === 'single' ? '正确答案（单选）' : '正确答案（至少两个）'" required>
          <a-radio-group v-if="qForm.type === 'single'" v-model:value="singleAnswer">
            <a-radio v-for="opt in qForm.options" :key="opt.key" :value="opt.key">
              {{ opt.key }}
            </a-radio>
          </a-radio-group>
          <a-checkbox-group v-else v-model:value="qForm.answer">
            <a-checkbox v-for="opt in qForm.options" :key="opt.key" :value="opt.key">
              {{ opt.key }}
            </a-checkbox>
          </a-checkbox-group>
        </a-form-item>

        <a-form-item label="解析">
          <a-textarea
            v-model:value="qForm.analysis"
            :rows="2"
            :maxlength="1000"
            placeholder="可选，交卷后向参与者展示"
          />
        </a-form-item>

        <a-form-item label="来源">
          <a-input
            v-model:value="qForm.source"
            :maxlength="200"
            placeholder="可选，例如：2026 年安全规程考试真题"
          />
        </a-form-item>

        <a-form-item label="标签">
          <a-select
            v-model:value="qForm.tags"
            mode="tags"
            :max-tag-count="6"
            placeholder="输入标签后回车，例如：第一章、重点"
            :options="bankTags.map((t) => ({ value: t.name, label: t.name }))"
          />
          <div class="muted tag-hint">最多 10 个标签，可在题库列表中按标签筛选</div>
        </a-form-item>
      </a-form>
    </a-modal>

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
import dayjs from 'dayjs'
import { message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import ImportQuestionsModal from '@/components/ImportQuestionsModal.vue'
import BankSharingDrawer from '@/components/BankSharingDrawer.vue'
import ImageUploadField from '@/components/ImageUploadField.vue'
import { banksApi, type Bank, type ImportJob, type Question, type QuestionOption, type Tag } from '@/api/banks'
import { isTauri } from '@/api/request'
import { useAuthStore } from '@/stores/auth'

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
    { title: '题型', key: 'type', width: 76 },
    { title: '题干', key: 'stem', dataIndex: 'stem' },
    { title: '标签', key: 'tags', width: 150 },
    { title: '答案', key: 'answer', width: 90 },
    { title: '难度', key: 'difficulty', width: 90 },
  ]
  if (canEdit.value) base.push({ title: '操作', key: 'actions', width: 130 })
  return base
})

const DIFFICULTY_LABELS = ['', '很简单', '简单', '中等', '困难', '极难']
const difficultyLabel = (v: number) => DIFFICULTY_LABELS[v] || DIFFICULTY_LABELS[3]
const hasImage = (q: Question) => !!q.stemImage || q.options.some((o) => !!o.image)

async function loadBank() {
  bank.value = await banksApi.get(bankId)
}

const exporting = ref(false)
const exportingImages = ref(false)

function exportFileName(suffix = '') {
  const name = (bank.value?.name ?? '题库').replace(/[\\/:*?"<>|]/g, '_').trim() || '题库'
  return `${name}${suffix}-${dayjs().format('YYYYMMDD')}.xlsx`
}

async function exportImages() {
  if (!bank.value || exportingImages.value) return
  exportingImages.value = true
  try {
    const fileName = exportFileName('-图片').replace(/\.xlsx$/, '.zip')
    if (isTauri()) {
      const { invoke } = await import('@tauri-apps/api/core')
      const path = await invoke<string | null>('pick_export_path', { fileName })
      if (!path) return
      const res = await banksApi.exportImagesTo(bankId, path)
      message.success(`已导出 ${res.count} 张图片到 ${res.path}`)
      return
    }
    const blob = await banksApi.exportImagesFile(bankId)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success('已开始下载')
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
      message.success(`已导出 ${res.count} 道题到 ${res.path}`)
      return
    }
    const blob = await banksApi.exportFile(bankId)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    link.click()
    URL.revokeObjectURL(url)
    message.success('已开始下载')
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
const tagColumns = [
  { title: '标签', key: 'name', dataIndex: 'name' },
  { title: '题目数', key: 'questionCount', width: 90 },
  { title: '操作', key: 'actions', width: 200 },
]
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

async function saveTagRename(tag: Tag) {
  const name = editingTagName.value.trim()
  if (!name) {
    message.warning('标签名称不能为空')
    return
  }
  if (name === tag.name) {
    cancelTagRename()
    return
  }
  await banksApi.renameTag(tag.id, name)
  message.success('标签已重命名')
  cancelTagRename()
  await refreshAfterTagChange()
}

function openMerge(tag: Tag) {
  mergeSource.value = tag
  mergeTargetId.value = undefined
  mergeModalOpen.value = true
}

async function confirmMerge() {
  if (!mergeSource.value || !mergeTargetId.value) {
    message.warning('请选择合并到的标签')
    return
  }
  mergeSaving.value = true
  try {
    await banksApi.mergeTag(mergeSource.value.id, mergeTargetId.value)
    message.success('标签已合并')
    mergeModalOpen.value = false
    await refreshAfterTagChange()
  } finally {
    mergeSaving.value = false
  }
}

async function removeTag(tag: Tag) {
  await banksApi.removeTag(tag.id)
  message.success('标签已删除')
  await refreshAfterTagChange()
}

const importJobs = ref<ImportJob[]>([])
const importColumns = [
  { title: '导入时间', key: 'createdAt', width: 150 },
  { title: '文件', key: 'source', dataIndex: 'sourceName' },
  { title: '结果', key: 'summary', width: 180 },
  { title: '状态', key: 'status', width: 170 },
  { title: '操作', key: 'actions', width: 150 },
]

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
  saveBlob(blob, `导入错误-${job.id}.csv`)
}

async function revertJob(job: ImportJob) {
  const res = await banksApi.revertImport(job.id)
  message.success(`已撤销 ${res.reverted} 道题${res.kept ? `，保留 ${res.kept} 道已编辑题目` : ''}`)
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
    message.warning('请填写题库名称')
    return
  }
  bankSaving.value = true
  try {
    bank.value = await banksApi.update(bankId, { name, description: bankForm.description.trim() })
    bankModalOpen.value = false
    message.success('题库已更新')
  } finally {
    bankSaving.value = false
  }
}

const qModalOpen = ref(false)
const qSaving = ref(false)
const editingQuestion = ref<Question | null>(null)
const qForm = reactive({
  type: 'single' as 'single' | 'multiple',
  stem: '',
  stemImage: '',
  answer: [] as string[],
  analysis: '',
  source: '',
  difficulty: 2,
  tags: [] as string[],
  options: [] as QuestionOption[],
})

const singleAnswer = computed({
  get: () => qForm.answer[0] ?? '',
  set: (v: string) => {
    qForm.answer = v ? [v] : []
  },
})

const LETTERS = 'ABCDEFGH'

function resetForm() {
  qForm.type = 'single'
  qForm.stem = ''
  qForm.stemImage = ''
  qForm.answer = []
  qForm.analysis = ''
  qForm.source = ''
  qForm.difficulty = 2
  qForm.tags = []
  qForm.options = [
    { key: 'A', content: '', image: '' },
    { key: 'B', content: '', image: '' },
  ]
}

function openCreateQuestion() {
  editingQuestion.value = null
  resetForm()
  qModalOpen.value = true
}

function openEditQuestion(q: Question) {
  editingQuestion.value = q
  qForm.type = q.type
  qForm.stem = q.stem
  qForm.stemImage = q.stemImage
  qForm.analysis = q.analysis
  qForm.source = q.source ?? ''
  qForm.difficulty = q.difficulty >= 1 && q.difficulty <= 5 ? q.difficulty : 3
  qForm.tags = [...(q.tags ?? [])]
  qForm.options = q.options.map((o) => ({ key: o.key, content: o.content, image: o.image }))
  qForm.answer = [...q.answer]
  qModalOpen.value = true
}

function onTypeChange() {
  if (qForm.type === 'single' && qForm.answer.length > 1) {
    qForm.answer = [qForm.answer[0]]
  }
}

function addOption() {
  if (qForm.options.length >= 8) return
  qForm.options.push({ key: LETTERS[qForm.options.length], content: '', image: '' })
}

function removeOption(index: number) {
  if (qForm.options.length <= 2) return
  const oldKeys = qForm.options.map((o) => o.key)
  const removedKey = oldKeys[index]
  qForm.options.splice(index, 1)
  const remainingOld = oldKeys.filter((_, i) => i !== index)
  const map: Record<string, string> = {}
  qForm.options.forEach((opt, i) => {
    const nextKey = LETTERS[i]
    map[remainingOld[i]] = nextKey
    opt.key = nextKey
  })
  qForm.answer = qForm.answer
    .filter((k) => k !== removedKey)
    .map((k) => map[k])
    .filter((k): k is string => Boolean(k))
}

async function saveQuestion() {
  const stem = qForm.stem.trim()
  if (!stem) {
    message.warning('请填写题干')
    return
  }
  const options = qForm.options.map((o) => ({
    key: o.key,
    content: o.content.trim(),
    image: o.image,
  }))
  if (options.some((o) => !o.content)) {
    message.warning('请填写完整的选项内容')
    return
  }
  if (!qForm.answer.length) {
    message.warning('请设置正确答案')
    return
  }
  if (qForm.type === 'single' && qForm.answer.length !== 1) {
    message.warning('单选题只能有一个正确答案')
    return
  }
  if (qForm.type === 'multiple' && qForm.answer.length < 2) {
    message.warning('多选题至少需要两个正确答案')
    return
  }
  qSaving.value = true
  try {
    const payload = {
      type: qForm.type,
      stem,
      stemImage: qForm.stemImage.trim(),
      answer: qForm.answer,
      analysis: qForm.analysis.trim(),
      source: qForm.source.trim(),
      difficulty: qForm.difficulty,
      tags: qForm.tags,
      options,
    }
    if (editingQuestion.value) {
      await banksApi.updateQuestion(editingQuestion.value.id, payload)
      message.success('题目已更新')
    } else {
      await banksApi.createQuestion(bankId, payload)
      message.success('题目已添加')
    }
    qModalOpen.value = false
    await Promise.all([loadQuestions(), loadBank(), loadTags()])
  } finally {
    qSaving.value = false
  }
}

async function removeQuestion(q: Question) {
  await banksApi.removeQuestion(q.id)
  message.success('题目已删除')
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
