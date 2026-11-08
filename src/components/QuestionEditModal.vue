<template>
  <a-modal
    :open="open"
    :title="question ? $t('bankDetail.editQuestion') : $t('bankDetail.addQuestionTitle')"
    :confirm-loading="saving"
    :ok-text="$t('common.save')"
    :cancel-text="$t('common.cancel')"
    :width="720"
    :mask-closable="false"
    @ok="save"
    @update:open="(v: boolean) => emit('update:open', v)"
  >
    <a-form layout="vertical">
      <div class="form-row">
        <a-form-item :label="$t('bankDetail.type')" required>
          <a-radio-group v-model:value="form.type" @change="onTypeChange">
            <a-radio-button value="single">{{ $t('bankDetail.singleQuestion') }}</a-radio-button>
            <a-radio-button value="multiple">{{ $t('bankDetail.multipleQuestion') }}</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="$t('bankDetail.difficulty')">
          <a-select v-model:value="form.difficulty" style="width: 120px">
            <a-select-option v-for="d in DIFFICULTY_LEVELS" :key="d.value" :value="d.value">
              {{ difficultyLabel(d.value) }}
            </a-select-option>
          </a-select>
        </a-form-item>
      </div>

      <a-form-item :label="$t('bankDetail.stem')" required>
        <a-textarea
          v-model:value="form.stem"
          :rows="3"
          :maxlength="2000"
          :placeholder="$t('bankDetail.stemPlaceholder')"
        />
      </a-form-item>

      <a-form-item :label="$t('bankDetail.stemImage')">
        <ImageUploadField v-model="form.stemImage" :hint="$t('bankDetail.imageHint')" />
      </a-form-item>

      <a-form-item :label="$t('bankDetail.options')" required>
        <div v-for="(opt, idx) in form.options" :key="opt.key" class="opt-row">
          <span class="opt-key">{{ opt.key }}</span>
          <div class="opt-main">
            <a-input
              v-model:value="opt.content"
              :placeholder="$t('bankDetail.optionPlaceholder', { key: opt.key })"
              :maxlength="500"
            />
            <ImageUploadField v-model="opt.image" />
          </div>
          <a-button
            type="text"
            danger
            size="small"
            :disabled="form.options.length <= 2"
            @click="removeOption(idx)"
          >
            {{ $t('bankDetail.removeOption') }}
          </a-button>
        </div>
        <a-button type="dashed" size="small" :disabled="form.options.length >= 8" @click="addOption">
          {{ $t('bankDetail.addOption') }}
        </a-button>
      </a-form-item>

      <a-form-item
        :label="form.type === 'single' ? $t('bankDetail.answerSingle') : $t('bankDetail.answerMultiple')"
        required
      >
        <a-radio-group v-if="form.type === 'single'" v-model:value="singleAnswer">
          <a-radio v-for="opt in form.options" :key="opt.key" :value="opt.key">{{ opt.key }}</a-radio>
        </a-radio-group>
        <a-checkbox-group v-else v-model:value="form.answer">
          <a-checkbox v-for="opt in form.options" :key="opt.key" :value="opt.key">{{ opt.key }}</a-checkbox>
        </a-checkbox-group>
      </a-form-item>

      <a-form-item :label="$t('bankDetail.analysis')">
        <a-textarea
          v-model:value="form.analysis"
          :rows="2"
          :maxlength="1000"
          :placeholder="$t('bankDetail.analysisPlaceholder')"
        />
      </a-form-item>

      <a-form-item :label="$t('bankDetail.sourceLabel')">
        <a-input
          v-model:value="form.source"
          :maxlength="200"
          :placeholder="$t('bankDetail.sourcePlaceholder')"
        />
      </a-form-item>

      <a-form-item :label="$t('bankDetail.tags')">
        <a-select
          v-model:value="form.tags"
          mode="tags"
          :max-tag-count="6"
          :placeholder="$t('bankDetail.tagsPlaceholder')"
          :options="tagOptions"
        />
        <div class="muted tag-hint">{{ $t('bankDetail.tagsHint') }}</div>
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup lang="ts">
// 题目新增/编辑弹窗：题库详情页与练习页共用。
// 编辑态由 question 传入；新增态由 bankId 指定题库。保存成功后 emit('saved', 保存后的完整题目)。
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import ImageUploadField from '@/components/ImageUploadField.vue'
import { banksApi, type Question, type QuestionInput, type QuestionOption, type Tag } from '@/api/banks'
import { DIFFICULTY_LEVELS, difficultyLabel } from '@/utils/difficulty'

const props = defineProps<{
  open: boolean
  /** 新增模式的目标题库 */
  bankId: number
  /** 传入则为编辑模式，null 为新增 */
  question: Question | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  saved: [question: Question]
}>()

const { t } = useI18n()
const saving = ref(false)

const form = reactive({
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
  get: () => form.answer[0] ?? '',
  set: (v: string) => {
    form.answer = v ? [v] : []
  },
})

const LETTERS = 'ABCDEFGH'

const tagOptions = ref<{ value: string; label: string }[]>([])

async function loadTags() {
  try {
    const res = await banksApi.bankTags(props.bankId)
    tagOptions.value = res.items.map((tag: Tag) => ({ value: tag.name, label: tag.name }))
  } catch {
    tagOptions.value = []
  }
}

function resetForm() {
  form.type = 'single'
  form.stem = ''
  form.stemImage = ''
  form.answer = []
  form.analysis = ''
  form.source = ''
  form.difficulty = 2
  form.tags = []
  form.options = [
    { key: 'A', content: '', image: '' },
    { key: 'B', content: '', image: '' },
  ]
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    void loadTags()
    const q = props.question
    if (!q) {
      resetForm()
      return
    }
    form.type = q.type
    form.stem = q.stem
    form.stemImage = q.stemImage
    form.analysis = q.analysis
    form.source = q.source ?? ''
    form.difficulty = q.difficulty >= 1 && q.difficulty <= 5 ? q.difficulty : 3
    form.tags = [...(q.tags ?? [])]
    form.options = q.options.map((o) => ({ key: o.key, content: o.content, image: o.image }))
    form.answer = [...q.answer]
  },
)

function onTypeChange() {
  if (form.type === 'single' && form.answer.length > 1) {
    form.answer = [form.answer[0]]
  }
}

function addOption() {
  if (form.options.length >= 8) return
  form.options.push({ key: LETTERS[form.options.length], content: '', image: '' })
}

function removeOption(index: number) {
  if (form.options.length <= 2) return
  const oldKeys = form.options.map((o) => o.key)
  const removedKey = oldKeys[index]
  form.options.splice(index, 1)
  const remainingOld = oldKeys.filter((_, i) => i !== index)
  const map: Record<string, string> = {}
  form.options.forEach((opt, i) => {
    const nextKey = LETTERS[i]
    map[remainingOld[i]] = nextKey
    opt.key = nextKey
  })
  form.answer = form.answer
    .filter((k) => k !== removedKey)
    .map((k) => map[k])
    .filter((k): k is string => Boolean(k))
}

async function save() {
  const stem = form.stem.trim()
  if (!stem) {
    message.warning(t('bankDetail.msg.stemRequired'))
    return
  }
  const options = form.options.map((o) => ({
    key: o.key,
    content: o.content.trim(),
    image: o.image,
  }))
  if (options.some((o) => !o.content)) {
    message.warning(t('bankDetail.msg.optionsRequired'))
    return
  }
  if (!form.answer.length) {
    message.warning(t('bankDetail.msg.answerRequired'))
    return
  }
  if (form.type === 'single' && form.answer.length !== 1) {
    message.warning(t('bankDetail.msg.singleAnswerOnly'))
    return
  }
  if (form.type === 'multiple' && form.answer.length < 2) {
    message.warning(t('bankDetail.msg.multipleAnswerMin'))
    return
  }
  saving.value = true
  try {
    const payload: QuestionInput = {
      type: form.type,
      stem,
      stemImage: form.stemImage.trim(),
      answer: form.answer,
      analysis: form.analysis.trim(),
      source: form.source.trim(),
      difficulty: form.difficulty,
      tags: form.tags,
      options,
    }
    const saved = props.question
      ? await banksApi.updateQuestion(props.question.id, payload)
      : await banksApi.createQuestion(props.bankId, payload)
    message.success(
      props.question ? t('bankDetail.msg.questionUpdated') : t('bankDetail.msg.questionAdded'),
    )
    emit('update:open', false)
    emit('saved', saved)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.opt-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 8px;
}
.opt-key {
  font-weight: 600;
  line-height: 30px;
  min-width: 18px;
}
.opt-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.tag-hint {
  font-size: 12px;
  margin-top: 4px;
}
</style>
