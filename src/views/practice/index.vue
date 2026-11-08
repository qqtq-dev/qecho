<template>
  <div class="page" :class="{ 'page-quiz': !!session }">
    <PageHeader :title="$t('practice.title')" :subtitle="$t('practice.subtitle')">
      <template #actions>
        <a-spin v-if="!session && !bootReady" size="small" />
        <a-button v-else-if="!session" type="primary" :loading="starting" @click="start()">
          {{ $t('practice.start') }}
        </a-button>
      </template>
    </PageHeader>

    <template v-if="session">
      <div class="quiz-layout" :class="{ 'ai-open': aiOpen }">
        <section class="panel quiz">
          <div class="quiz-head">
            <div>
              <a-tag v-if="session.source === 'wrong'" color="red">{{ $t('practice.wrongReview') }}</a-tag>
              <a-tag v-else color="green">{{ session.bankName }}</a-tag>
              <span class="muted">
                {{ $t('practice.progress', { current: index + 1, total: session.totalCount }) }}
              </span>
              <span class="muted progress-hint">{{ $t('practice.answeredBadge', { n: answeredCount }) }}</span>
            </div>
            <div class="head-right">
              <a-tooltip v-if="editAvailable" :title="$t('practice.editQuestion')" placement="bottomRight">
                <a-button
                  size="small"
                  class="mark-btn"
                  :aria-label="$t('practice.editQuestion')"
                  @click="openEditQuestion"
                >
                  <template #icon><Pencil :size="12" /></template>
                </a-button>
              </a-tooltip>
              <a-tooltip v-if="current" :title="isMarked ? $t('practice.unmark') : $t('practice.mark')"
                placement="bottomRight">
                <a-button
                  size="small"
                  class="mark-btn"
                  :class="{ marked: isMarked }"
                  :aria-label="isMarked ? $t('practice.unmark') : $t('practice.mark')"
                  :aria-pressed="isMarked"
                  @click="toggleMark"
                >
                  <template #icon><Flag :size="12" :fill="isMarked ? 'currentColor' : 'none'" /></template>
                </a-button>
              </a-tooltip>
            </div>
          </div>

          <template v-if="current">
            <div ref="bodyRef" class="quiz-body">
              <div class="q-type">{{ current.type === 'multiple' ? $t('practice.multiple') : $t('practice.single') }}</div>
              <div class="q-stem">{{ current.stem }}</div>
              <QuestionImage v-if="current.stemImage" :name="current.stemImage" class="q-img" />

              <div class="options">
                <div
                  v-for="opt in current.options"
                  :key="opt.key"
                  class="opt"
                  :class="optionClass(opt)"
                  @click="toggleOption(opt.key)"
                >
                  <span class="opt-key">{{ opt.label }}</span>
                  <span v-if="opt.image" class="opt-thumb" @click.stop>
                    <QuestionImage :name="opt.image" :thumb="true" :size="72" />
                  </span>
                  <span class="opt-text">{{ opt.content }}</span>
                </div>
              </div>

              <div v-if="current.type === 'multiple' && !currentAnswer" class="multi-submit">
                <a-button
                  type="primary"
                  :loading="submitting"
                  :disabled="chosen.length === 0"
                  @click="submitAnswer"
                >
                  {{ $t('practice.confirmAnswer') }}
                </a-button>
              </div>

              <div class="analysis-box">
                <div class="analysis-head">
                  <div class="analysis-title">{{ $t('practice.analysis') }}</div>
                  <div class="analysis-actions">
                    <!-- 编辑后原位置变成保存按钮（样式与「问 AI」一致，只是图标化） -->
                    <template v-if="analysisEditing">
                      <a-tooltip :title="$t('common.save')">
                        <a-button
                          size="small"
                          type="primary"
                          class="ai-ask-btn icon-act"
                          :loading="analysisSaving"
                          @click="saveAnalysis"
                        >
                          <template #icon><Check :size="12" /></template>
                        </a-button>
                      </a-tooltip>
                      <a-tooltip :title="$t('common.cancel')">
                        <a-button size="small" class="ai-ask-btn icon-act" @click="cancelAnalysisEdit">
                          <template #icon><X :size="12" /></template>
                        </a-button>
                      </a-tooltip>
                    </template>
                    <a-tooltip v-else-if="currentAnswer && canEditAnalysis" :title="$t('practice.editAnalysis')">
                      <a-button
                        size="small"
                        class="ai-ask-btn icon-act"
                        :aria-label="$t('practice.editAnalysis')"
                        @click="startAnalysisEdit"
                      >
                        <template #icon><Pencil :size="12" /></template>
                      </a-button>
                    </a-tooltip>
                    <!-- 未配置（配置读取完成且 ready=false）时在按钮原位显示引导；null=还在读取，不闪 -->
                    <a-button
                      v-if="aiReady === false"
                      size="small"
                      class="ai-setup-hint"
                      @click="router.push('/settings')"
                    >
                      {{ $t('practice.setupAi') }}
                    </a-button>
                    <!-- 面板开着时按钮常驻，保证任何时刻都能收起；没开面板才要求先作答 -->
                    <a-button
                      v-else-if="aiReady && (aiOpen || currentAnswer)"
                      size="small"
                      class="ai-ask-btn"
                      @click="aiOpen = !aiOpen"
                    >
                      {{ aiOpen ? $t('practice.collapseAi') : $t('practice.askAi') }}
                    </a-button>
                  </div>
                </div>
                <template v-if="currentAnswer">
                  <a-textarea
                    v-if="analysisEditing"
                    v-model:value="analysisDraft"
                    :auto-size="{ minRows: 1 }"
                    :maxlength="20000"
                    class="analysis-editor"
                  />
                  <template v-else>
                    <div v-if="currentAnswer.result.analysis" class="analysis-text">{{ currentAnswer.result.analysis }}</div>
                    <div v-else class="muted">{{ $t('practice.noAnalysis') }}</div>
                  </template>
                </template>
                <div v-else class="muted">{{ $t('practice.analysisAfter') }}</div>
              </div>
            </div>

            <div class="quiz-actions">
              <a-space>
                <a-button :disabled="index === 0" @click="go(index - 1)">{{ $t('practice.prev') }}</a-button>
                <a-button :disabled="index >= session.totalCount - 1" @click="go(index + 1)">
                  {{ $t('practice.next') }}
                </a-button>
              </a-space>
              <a-space :size="8">
                <a-button @click="exitSession">{{ $t('practice.exitSession') }}</a-button>
                <a-popconfirm
                  :title="$t('practice.finishConfirm', { answered: answeredCount, total: session.totalCount })"
                  :ok-text="$t('practice.finishOk')"
                  :cancel-text="$t('practice.continueAnswering')"
                  @confirm="finish"
                >
                  <a-button type="primary">{{ $t('practice.finish') }}</a-button>
                </a-popconfirm>
              </a-space>
            </div>
            <div class="kbd-hint muted">
              {{ $t('practice.kbdHint') }}
            </div>
          </template>
        </section>

        <aside class="panel side-col">
          <!-- 两个面板常驻挂载（v-show），AI 会话不因收起而丢；流式回答收起后继续走 -->
          <div v-show="!aiOpen" class="answer-card">
            <div class="side-head">
              <h3>{{ $t('practice.answerCard') }}</h3>
              <span class="muted">{{ answeredCount }} / {{ session.totalCount }}</span>
            </div>
            <div class="side-legend">
              <span><i class="dot ok"></i>{{ $t('practice.legendOk') }}</span>
              <span><i class="dot bad"></i>{{ $t('practice.legendBad') }}</span>
              <span><i class="dot none"></i>{{ $t('practice.legendNone') }}</span>
              <span><i class="dot mark"></i>{{ $t('practice.legendMark') }}</span>
            </div>
            <div class="q-grid">
              <button
                v-for="(q, i) in questions"
                :key="q.id"
                type="button"
                class="q-cell"
                :class="cellClass(i)"
                @click="go(i)"
              >
                {{ i + 1 }}
              </button>
            </div>
            <div class="side-foot muted">{{ $t('practice.correctCount', { n: correctCount }) }}</div>
          </div>
          <AiChatPanel
            v-show="aiOpen"
            class="ai-host"
            :open="aiOpen"
            :question-id="current?.id ?? 0"
            :stem="current?.stem ?? ''"
            :options="aiOptions"
            :type="current?.type ?? 'single'"
          />
        </aside>
      </div>
    </template>

    <template v-else-if="finished && summary">
      <section class="panel result">
        <a-result
          status="success"
          :title="
            $t('practice.resultTitle', {
              correct: summary.correctCount,
              total: summary.totalCount,
            })
          "
          :sub-title="
            $t('practice.resultSub', {
              duration: formatDuration(summary.durationSec),
              pending: wrongStore.stats.pending,
            })
          "
        >
          <template #extra>
            <a-space>
              <a-button type="primary" @click="start()">{{ $t('practice.again') }}</a-button>
              <a-button @click="backToSettings">{{ $t('practice.adjust') }}</a-button>
              <a-button @click="router.push('/wrong')">{{ $t('practice.viewWrong') }}</a-button>
            </a-space>
          </template>
        </a-result>
      </section>
    </template>

    <div v-else-if="bootReady" class="grid">
      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('practice.settings') }}</h3>
        </div>
        <div class="panel-body">
          <a-form layout="vertical">
            <a-form-item :label="$t('practice.bank')" required>
              <a-select
                v-model:value="form.bankIds"
                mode="multiple"
                :placeholder="$t('practice.bankPlaceholder')"
                :loading="loadingBanks"
                :options="bankOptions"
                :max-tag-count="2"
                allow-clear
              />
            </a-form-item>
            <a-form-item :label="$t('practice.questionType')">
              <a-radio-group v-model:value="form.type">
                <a-radio-button value="">{{ $t('practice.all') }}</a-radio-button>
                <a-radio-button value="single">{{ $t('bankDetail.single') }}</a-radio-button>
                <a-radio-button value="multiple">{{ $t('bankDetail.multiple') }}</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <a-form-item :label="$t('practice.order')">
              <a-radio-group v-model:value="form.order">
                <a-radio-button value="random">{{ $t('practice.random') }}</a-radio-button>
                <a-radio-button value="fixed">{{ $t('practice.fixed') }}</a-radio-button>
              </a-radio-group>
            </a-form-item>
            <div class="form-row">
              <a-form-item :label="$t('practice.difficulty')" class="field-narrow">
                <a-select
                  v-model:value="form.difficulty"
                  :options="difficultyOptions"
                  style="width: 100%"
                />
              </a-form-item>
              <a-form-item :label="$t('practice.tags')" class="field-wide">
                <a-select
                  v-model:value="form.tags"
                  mode="multiple"
                  :placeholder="$t('practice.tagsPlaceholder')"
                  :loading="loadingTags"
                  :options="tagOptions"
                  :max-tag-count="3"
                  allow-clear
                  :disabled="form.bankIds.length === 0"
                />
              </a-form-item>
            </div>
            <div class="form-row">
              <a-form-item :label="$t('practice.rangeLabel')" class="field-range">
                <div class="range-row">
                  <a-input-number
                    v-model:value="form.startSeq"
                    :min="1"
                    :max="candidateTotal || 99999"
                    :precision="0"
                    placeholder="1"
                    class="range-input"
                  />
                  <span class="range-sep">—</span>
                  <a-input-number
                    v-model:value="form.endSeq"
                    :min="form.startSeq || 1"
                    :max="candidateTotal || 99999"
                    :precision="0"
                    :placeholder="$t('practice.rangeEndPlaceholder')"
                    class="range-input"
                  />
                  <span class="muted range-unit">{{ $t('practice.rangeUnit') }}</span>
                  <span v-if="form.order !== 'fixed'" class="count-field">
                    <span class="muted count-label">{{ $t('practice.countLabel') }}</span>
                    <a-select
                      v-model:value="form.count"
                      :options="countOptions"
                      :title="$t('practice.countTitle')"
                      class="count-select"
                      @change="onCountChange"
                    />
                  </span>
                </div>
                <div class="range-hint muted">
                  <template v-if="loadingCandidates">{{ $t('practice.counting') }}</template>
                  <template v-else-if="candidateTotal > 0">
                    {{ rangeActive ? rangeSummary : $t('practice.allCount', { n: candidateTotal }) }}
                    <span v-if="!form.startSeq">{{ $t('practice.rangeFromFirst') }}</span>
                  </template>
                  <template v-else>{{ $t('practice.chooseBankFirst') }}</template>
                </div>
                <div v-if="filterActive" class="range-hint muted">
                  {{ $t('practice.filteredCount', { n: candidateTotal }) }}
                </div>
                <div v-if="nextRangeStart" class="range-hint">
                  <a-button type="link" size="small" class="range-next" @click="applyNextRange">
                    {{ $t('practice.nextRange', { last: nextRangeStart - 1, next: nextRangeStart }) }}
                  </a-button>
                </div>
                <div v-if="pickSummary" class="range-hint muted">{{ pickSummary }}</div>
              </a-form-item>
            </div>
            <a-form-item :label="$t('practice.shuffleOptions')">
              <a-switch v-model:checked="form.shuffleOptions" />
              <span class="muted switch-hint">{{ $t('practice.shuffleHint') }}</span>
            </a-form-item>
            <a-empty v-if="!loadingBanks && banks.length === 0" :description="$t('practice.noBanks')">
              <a-button type="primary" ghost @click="router.push('/banks')">{{ $t('practice.goCreateBank') }}</a-button>
            </a-empty>
          </a-form>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>{{ $t('practice.recent') }}</h3>
          <span class="muted">{{ $t('practice.recentDesc') }}</span>
        </div>
        <div class="panel-body" :class="{ 'empty-state': sessions.length === 0 }">
          <a-empty v-if="sessions.length === 0" :description="$t('practice.noSessions')">
            <a-button type="primary" ghost @click="router.push('/banks')">{{ $t('practice.browseBanks') }}</a-button>
          </a-empty>
          <a-list v-else :data-source="sessions" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="recent-row">
                  <div class="recent-main">
                    <div class="recent-title">
                      {{ item.source === 'wrong' ? $t('practice.wrongReview') : item.bankName }}
                      <a-tag :color="item.status === 'ongoing' ? 'processing' : 'default'" class="recent-tag">
                        {{ item.status === 'ongoing' ? $t('practice.ongoing') : $t('practice.completed') }}
                      </a-tag>
                    </div>
                    <div class="muted">
                      {{
                        $t('practice.sessionMeta', {
                          time: formatTime(item.updatedAt || item.createdAt),
                          count: item.totalCount,
                          order: orderLabel(item),
                        })
                      }}
                      <template v-if="item.shuffleOptions"> · {{ $t('practice.optionShuffled') }}</template>
                    </div>
                  </div>
                  <div class="recent-right">
                    <div class="recent-score">
                      <span>{{ item.answeredCount }}/{{ item.totalCount }}</span>
                      <span class="muted">
                        {{ $t('practice.correctShort', { n: item.correctCount }) }}<template v-if="item.durationSec"> · {{ formatDuration(item.durationSec) }}</template>
                      </span>
                    </div>
                    <a-space :size="0">
                      <a-button
                        v-if="item.status === 'ongoing'"
                        type="link"
                        size="small"
                        class="recent-continue"
                        @click="resume(item.id)"
                      >
                        {{ $t('practice.resume') }}
                      </a-button>
                      <a-button
                        v-if="item.status === 'finished' && item.source === 'bank'"
                        type="link"
                        size="small"
                        class="recent-continue"
                        :disabled="starting"
                        @click="retrySession(item)"
                      >
                        {{ $t('practice.retry') }}
                      </a-button>
                      <a-popconfirm
                        :title="$t('practice.removeConfirm')"
                        :ok-text="$t('banks.remove')"
                        :cancel-text="$t('common.cancel')"
                        @confirm="removeSession(item.id)"
                      >
                        <a-button type="link" size="small" danger>{{ $t('banks.remove') }}</a-button>
                      </a-popconfirm>
                    </a-space>
                  </div>
                </div>
              </a-list-item>
            </template>
          </a-list>
        </div>
      </section>
    </div>

    <QuestionEditModal
      v-model:open="editOpen"
      :bank-id="session?.bankId ?? 0"
      :question="editTarget"
      @saved="onQuestionSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Check, Flag, Pencil, X } from 'lucide-vue-next'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import PageHeader from '@/components/PageHeader.vue'
import QuestionImage from '@/components/QuestionImage.vue'
import AiChatPanel from '@/components/AiChatPanel.vue'
import QuestionEditModal from '@/components/QuestionEditModal.vue'
import { aiApi } from '@/api/ai'
import { banksApi, type Bank, type Question, type Tag } from '@/api/banks'
import { settingsApi } from '@/api/settings'
import {
  practiceApi,
  type PracticeAnswerResult,
  type PracticeDetail,
  type PracticeItem,
  type PracticeOption,
  type PracticeSession,
  type PracticeSummary,
} from '@/api/practice'
import { useWrongStore } from '@/stores/wrong'
import { difficultyFilterOptions } from '@/utils/difficulty'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const wrongStore = useWrongStore()

const banks = ref<Bank[]>([])
const availableTags = ref<Tag[]>([])
const loadingBanks = ref(false)
const loadingTags = ref(false)
const starting = ref(false)
const sessions = ref<PracticeSession[]>([])
let tagLoadSeq = 0

const form = reactive({
  bankIds: [] as number[],
  order: 'random' as 'random' | 'fixed',
  type: '' as '' | 'single' | 'multiple',
  difficulty: 0,
  tags: [] as string[],
  count: 20,
  // 按题序时的题序范围：起始必填，结束留空表示练到末尾
  startSeq: 1,
  endSeq: null as number | null,
  shuffleOptions: false,
})

const candidateTotal = ref(0)
const loadingCandidates = ref(false)
let candidateSeq = 0

async function loadCandidates() {
  const seq = ++candidateSeq
  if (form.bankIds.length === 0) {
    candidateTotal.value = 0
    loadingCandidates.value = false
    return
  }
  loadingCandidates.value = true
  try {
    const res = await practiceApi.preview({
      bankIds: form.bankIds,
      questionType: form.type || undefined,
      difficulty: form.difficulty || undefined,
      tags: form.tags.length ? form.tags : undefined,
    })
    if (seq === candidateSeq) candidateTotal.value = res.candidates
  } catch {
    if (seq === candidateSeq) candidateTotal.value = 0
  } finally {
    if (seq === candidateSeq) loadingCandidates.value = false
  }
}

watch(
  () => [form.bankIds.join(','), form.type, form.difficulty, form.tags.join(',')].join('|'),
  () => {
    void loadCandidates()
  },
)

// 题序范围摘要：显示实际会练到哪几道；起始 1 + 末题留空视为“全库”
const rangeSummary = computed(() => {
  const total = candidateTotal.value
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = total > 0 ? Math.min(rawEnd || total, total) : rawEnd
  if (end < start) return t('practice.rangeEndBeforeStart')
  const span = end - start + 1
  if (rawEnd === 0) {
    return start <= 1
      ? t('practice.rangeAll', { n: span })
      : t('practice.rangeToEnd', { start, n: span })
  }
  return t('practice.rangeSpan', { start, end, n: span })
})

// 是否真的限定了范围（起始 1 + 末题留空 = 全库，不算）
const rangeActive = computed(() => {
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  return Math.max(1, form.startSeq || 1) > 1 || rawEnd > 0
})

// 范围里实际有多少道（没限定范围时等于候选总数）
const rangeSpan = computed(() => {
  if (!rangeActive.value) return candidateTotal.value
  const total = candidateTotal.value
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = total > 0 ? Math.min(rawEnd || total, total) : rawEnd
  if (end < start) return 0
  return end - start + 1
})

// 随机时实际会做多少道：题量选“全部”就是整段
const pickCount = computed(() => {
  const span = rangeSpan.value
  if (form.count <= 0) return span
  return span > 0 ? Math.min(form.count, span) : form.count
})

// 把“范围 + 题量”翻译成一句人话，免得用户自己算
const pickSummary = computed(() => {
  const total = candidateTotal.value
  if (form.order === 'fixed' || total <= 0) return ''
  // 题量选“全部”时范围摘要已经说清了整段，不重复占一行
  if (form.count <= 0) return ''
  const n = pickCount.value
  if (n <= 0) return ''
  const start = Math.max(1, form.startSeq || 1)
  const rawEnd = form.endSeq && form.endSeq > 0 ? form.endSeq : 0
  const end = Math.min(rawEnd || total, total)
  if (!rangeActive.value) {
    return form.count > 0
      ? t('practice.pickAll', { total, n })
      : t('practice.pickShuffleAll', { total })
  }
  return form.count > 0
    ? t('practice.pickSpan', { start, end, n })
    : t('practice.pickSpanAll', { start, end, n })
})

// 切到“按题序”时，若还没填范围，就用题量兜一个默认段，保持原来的直觉
watch(
  () => form.order,
  (order) => {
    if (order !== 'fixed' || rangeActive.value) return
    form.startSeq = 1
    form.endSeq = form.count > 0 ? form.count : null
  },
)

// 用户亲手改过题量后就不再自动跟随范围
let countTouched = false
function onCountChange() {
  countTouched = true
}

// 范围一填，题量默认切到“全部”（这段全做）；清空范围则回到 20 题
watch(
  () => [form.startSeq, form.endSeq].join('|'),
  () => {
    if (form.order !== 'random' || countTouched) return
    form.count = rangeActive.value ? 0 : 20
  },
)

// 上次同题库“按题序”练习的结束位置 + 1，用于一键接着往下练
const nextRangeStart = computed(() => {
  if (form.order !== 'fixed' || form.bankIds.length !== 1) return 0
  const last = sessions.value.find(
    (s) => s.bankId === form.bankIds[0] && s.order === 'fixed' && s.source === 'bank',
  )
  if (!last) return 0
  const start = last.rangeStart > 0 ? last.rangeStart : 1
  const end = last.rangeEnd > 0 ? last.rangeEnd : start + last.totalCount - 1
  const next = end + 1
  if (candidateTotal.value > 0 && next > candidateTotal.value) return 0
  if (next <= (form.startSeq || 1) && form.endSeq !== null) return 0
  return next
})

function applyNextRange() {
  const next = nextRangeStart.value
  if (!next) return
  form.startSeq = next
  // 保留上次跨度，方便一段段往下练
  const last = sessions.value.find(
    (s) => s.bankId === form.bankIds[0] && s.order === 'fixed' && s.source === 'bank',
  )
  const span = last ? last.totalCount : 0
  const total = candidateTotal.value
  const end = span > 0 ? next + span - 1 : 0
  form.endSeq = end > 0 && (total === 0 || end <= total) ? end : null
}

// 范围的常用档位：前 N 题 / 全库，省得每次都敲数字
// 是否加了筛选：题号是按筛选结果算的，得说清楚
const filterActive = computed(
  () => form.type !== '' || form.difficulty > 0 || form.tags.length > 0,
)

const tagOptions = computed(() =>
  availableTags.value.map((tag) => ({
    label: t('practice.tagOption', { name: tag.name, n: tag.questionCount }),
    value: tag.name,
  })),
)

async function loadFormTags() {
  const ids = [...form.bankIds]
  const seq = ++tagLoadSeq
  if (ids.length === 0) {
    availableTags.value = []
    form.tags = []
    loadingTags.value = false
    return
  }
  loadingTags.value = true
  try {
    const results = await Promise.all(ids.map((id) => banksApi.bankTags(id)))
    if (seq !== tagLoadSeq) return
    const merged = new Map<string, Tag>()
    for (const res of results) {
      for (const tag of res.items) {
        const prev = merged.get(tag.name)
        if (prev) prev.questionCount += tag.questionCount
        else merged.set(tag.name, { ...tag })
      }
    }
    availableTags.value = [...merged.values()].sort(
      (a, b) => b.questionCount - a.questionCount || a.name.localeCompare(b.name, locale.value),
    )
    const names = new Set(availableTags.value.map((tag) => tag.name))
    form.tags = form.tags.filter((t) => names.has(t))
  } catch {
    if (seq === tagLoadSeq) availableTags.value = []
  } finally {
    if (seq === tagLoadSeq) loadingTags.value = false
  }
}

watch(() => form.bankIds.join(','), loadFormTags)

const countOptions = computed(() => {
  const options = [
    { label: t('practice.countOption', { n: 10 }), value: 10 },
    { label: t('practice.countOption', { n: 20 }), value: 20 },
    { label: t('practice.countOption', { n: 50 }), value: 50 },
    { label: t('practice.countOption', { n: 100 }), value: 100 },
    { label: t('practice.countAll'), value: 0 },
  ]
  // “全部”在当前范围下就是整段，把实际道数标出来
  if (rangeActive.value && rangeSpan.value > 0) {
    options[options.length - 1].label = t('practice.countAllSpan', { n: rangeSpan.value })
  }
  if (!options.some((o) => o.value === form.count)) {
    options.splice(options.length - 1, 0, {
      label: t('practice.countOption', { n: form.count }),
      value: form.count,
    })
  }
  return options
})

const difficultyOptions = computed(() => difficultyFilterOptions())

const bankOptions = computed(() =>
  banks.value.map((b) => ({
    label: t('practice.bankOption', { name: b.name, n: b.questionCount }),
    value: b.id,
    disabled: b.questionCount === 0,
  })),
)

const session = ref<PracticeSession | null>(null)
const questions = ref<PracticeItem[]>([])
const index = ref(0)
const chosen = ref<string[]>([])
const bodyRef = ref<HTMLElement | null>(null)
const submitting = ref(false)
const finished = ref(false)
const summary = ref<PracticeSummary | null>(null)
const markedIds = ref<number[]>([])
// 启动门：练习页挂载后要先决定「恢复答题 / 显示设置面板」，完成前不渲染设置区，避免闪跳
const bootReady = ref(false)
let questionStartedAt = Date.now()

interface Answered {
  chosen: string[]
  result: PracticeAnswerResult
}

const answers = reactive<Record<number, Answered>>({})

const current = computed(() => questions.value[index.value] ?? null)
const currentAnswer = computed(() => (current.value ? answers[current.value.id] : undefined))
const isMarked = computed(() => !!current.value && markedIds.value.includes(current.value.id))
const answeredCount = computed(() => Object.keys(answers).length)
const correctCount = computed(() => Object.values(answers).filter((a) => a.result.isCorrect).length)

// 编辑题目：仅题库练习（错题回顾跨多个题库，不提供入口）且当前用户可编辑该题库时显示
const editOpen = ref(false)
const editTarget = ref<Question | null>(null)
const bankCanEdit = ref(false)
const editAvailable = computed(
  () => session.value?.source === 'bank' && bankCanEdit.value && !!current.value && !finished.value,
)

async function refreshEditAccess(detail: PracticeDetail) {
  bankCanEdit.value = false
  if (detail.session.source !== 'bank') return
  try {
    const bank = await banksApi.get(detail.session.bankId)
    bankCanEdit.value = bank.canEdit
  } catch {
    bankCanEdit.value = false
  }
}

// 就地编辑解析：解析框内直接改文本，不弹窗；只更新当前题的 analysis 字段
const analysisEditing = ref(false)
const analysisDraft = ref('')
const analysisSaving = ref(false)
const canEditAnalysis = computed(() => session.value?.source === 'bank' && bankCanEdit.value)

function startAnalysisEdit() {
  if (!current.value || !currentAnswer.value || analysisEditing.value) return
  analysisDraft.value = currentAnswer.value.result.analysis ?? ''
  analysisEditing.value = true
}

function cancelAnalysisEdit() {
  analysisEditing.value = false
  analysisDraft.value = ''
}

async function saveAnalysis() {
  if (!current.value || analysisSaving.value) return
  analysisSaving.value = true
  try {
    const q = await banksApi.updateQuestionAnalysis(current.value.id, analysisDraft.value)
    const entry = answers[current.value.id]
    if (entry) entry.result.analysis = q.analysis ?? ''
    cancelAnalysisEdit()
    message.success(t('practice.msg.analysisSaved'))
  } catch {
    message.error(t('practice.msg.analysisSaveFailed'))
  } finally {
    analysisSaving.value = false
  }
}

// 换题或重开会话时退出编辑态，避免把草稿带到别的题上
watch(
  () => current.value?.id,
  () => {
    if (analysisEditing.value) cancelAnalysisEdit()
  },
)

async function openEditQuestion() {
  if (!current.value || editOpen.value) return
  try {
    editTarget.value = await banksApi.getQuestion(current.value.id)
    editOpen.value = true
  } catch {
    message.error(t('practice.msg.loadQuestionFailed'))
  }
}

// 保存后重拉会话详情：选项可能增删，乱序模式的显示字母要由服务端重算；
// 仍停在用户正在看的那一题。
async function onQuestionSaved() {
  if (!session.value) return
  const keep = index.value
  try {
    const detail = await practiceApi.detail(session.value.id)
    applySession(detail)
    const maxIndex = Math.max(questions.value.length - 1, 0)
    index.value = Math.min(keep, maxIndex)
    const item = current.value
    chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  } catch {
    /* 拉取失败时保持现状，下次进入会话即拿到新数据 */
  }
}

// AI 答疑：仅在服务端已启用并配置凭据时显示入口
// null=配置还在读取（引导不闪），false=未配置，true=已配置
const aiReady = ref<boolean | null>(null)
const aiOpen = ref(false)
const aiOptions = computed(() =>
  (current.value?.options ?? []).map((o) => `${o.label}. ${o.content}`),
)

async function loadAiConfig() {
  try {
    aiReady.value = (await aiApi.config()).ready
  } catch {
    aiReady.value = false
  }
}

function formatTime(raw: string) {
  return raw ? dayjs(raw).format('MM-DD HH:mm') : '—'
}

function formatDuration(seconds: number) {
  const sec = Math.max(0, Math.round(seconds || 0))
  const minutes = Math.floor(sec / 60)
  if (minutes >= 60) {
    return t('practice.durationHour', { h: Math.floor(minutes / 60), m: minutes % 60 })
  }
  return t('practice.durationMin', { m: minutes, s: String(sec % 60).padStart(2, '0') })
}

function orderLabel(item: PracticeSession) {
  const start = item.rangeStart > 0 ? item.rangeStart : 0
  const end = start > 0 ? (item.rangeEnd > 0 ? item.rangeEnd : start + item.totalCount - 1) : 0
  if (item.order === 'random') {
    return start > 0 ? t('practice.orderRandomRange', { start, end }) : t('practice.orderRandom')
  }
  return start > 0 ? t('practice.orderFixedRange', { start, end }) : t('practice.orderFixed')
}

function clearAnswers() {
  for (const id of Object.keys(answers)) delete answers[Number(id)]
}

function applySession(detail: PracticeDetail) {
  clearAnswers()
  // 新练习/续练都算新开始：AI 面板复位，避免上一组的展开状态带到第一题、挂载即自动发首问
  aiOpen.value = false
  session.value = detail.session
  questions.value = detail.questions
  for (const a of detail.answers) {
    answers[a.questionId] = {
      chosen: [...a.chosen],
      result: {
        isCorrect: a.isCorrect,
        answer: a.answer,
        analysis: a.analysis,
        wrongCount: a.wrongCount,
        correctStreak: a.correctStreak,
        mastered: a.mastered,
        timeSpentMs: a.timeSpentMs,
        durationSec: detail.session.durationSec,
        stats: detail.stats,
      },
    }
  }
  markedIds.value = [...detail.marked]
  void refreshEditAccess(detail)
  const maxIndex = Math.max(questions.value.length - 1, 0)
  index.value = Math.min(Math.max(detail.lastSeq - 1, 0), maxIndex)
  finished.value = false
  summary.value = null
  const item = current.value
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  questionStartedAt = Date.now()
  wrongStore.setStats(detail.stats)
}

async function loadBanks() {
  loadingBanks.value = true
  try {
    const res = await banksApi.list()
    banks.value = res.items
    if (form.bankIds.length === 0) {
      const preferred = banks.value.find((b) => b.questionCount > 0)
      if (preferred) form.bankIds = [preferred.id]
    }
  } finally {
    loadingBanks.value = false
  }
}

async function loadSessions() {
  const res = await practiceApi.sessions()
  sessions.value = res.items
}

async function loadSettings() {
  try {
    const s = await settingsApi.get()
    form.count = s.practiceCount
    form.order = s.practiceShuffleQuestions ? 'random' : 'fixed'
    form.shuffleOptions = s.practiceShuffleOptions
  } catch {
    /* 读取失败时沿用默认值 */
  }
  form.startSeq = 1
  form.endSeq = form.count > 0 ? form.count : null
}

async function removeSession(id: number) {
  await practiceApi.remove(id)
  sessions.value = sessions.value.filter((s) => s.id !== id)
  message.success(t('practice.msg.sessionDeleted'))
}

async function start(source: 'bank' | 'wrong' = 'bank', bankIdsOverride?: number[]) {
  if (starting.value) return
  // 题序范围对两种顺序都生效：按题序是顺序练这一段，随机是只在这段里抽题
  const useRange = source === 'bank' && rangeActive.value
  if (useRange) {
    const start = Math.max(1, form.startSeq || 1)
    if (form.endSeq && form.endSeq > 0 && form.endSeq < start) {
      message.warning(t('practice.msg.endBeforeStart'))
      return
    }
  }
  starting.value = true
  try {
    const res = await practiceApi.start({
      bankIds: source === 'wrong' ? bankIdsOverride : form.bankIds,
      // 按题序时由题序范围决定题量；随机时由题量决定抽几道
      count: source === 'wrong' || form.order === 'fixed' ? 0 : form.count,
      order: form.order,
      startSeq: useRange ? Math.max(1, form.startSeq || 1) : undefined,
      endSeq: useRange ? form.endSeq || 0 : undefined,
      source,
      questionType: form.type || undefined,
      difficulty: form.difficulty || undefined,
      tags: form.tags.length ? form.tags : undefined,
      shuffleOptions: form.shuffleOptions,
    })
    applySession({
      session: res.session,
      questions: res.questions,
      answers: [],
      marked: [],
      lastSeq: 1,
      stats: wrongStore.stats,
    })
    // 会话写进地址栏：F5 刷新时凭 query 恢复答题界面
    router.replace({ path: '/practice', query: { session: String(res.session.id) } })
    if (res.truncated) message.info(t('practice.msg.truncated'))
  } finally {
    starting.value = false
  }
}

async function startWrong() {
  const bankId = route.query.bankId ? Number(route.query.bankId) : undefined
  try {
    await start('wrong', bankId ? [bankId] : [])
  } catch {
    message.info(t('practice.msg.noWrong'))
    router.replace('/wrong')
  }
}

async function resume(id: number) {
  try {
    const detail = await practiceApi.detail(id)
    if (detail.session.status !== 'ongoing') {
      message.info(t('practice.msg.alreadyFinished'))
      await loadSessions()
      return
    }
    applySession(detail)
    // 会话写进地址栏：F5 刷新时凭 query 恢复答题界面
    router.replace({ path: '/practice', query: { session: String(id) } })
  } catch {
    message.error(t('practice.msg.resumeFailed'))
    await loadSessions()
  }
}

// 退出本次练习：进度已实时保存在服务端，仅回到练习列表页
async function exitSession() {
  if (!session.value) return
  savePosition()
  session.value = null
  summary.value = null
  try {
    await loadSessions()
  } catch {
    /* 列表刷新失败不阻断返回 */
  }
  router.replace({ path: '/practice' })
}

// 重练：把已完成的题库练习原样再做一遍（同一批题目，题序/选项乱序沿用原设置）
async function retrySession(item: PracticeSession) {
  if (starting.value) return
  starting.value = true
  try {
    const detail = await practiceApi.detail(item.id)
    const ids = detail.questions.map((q) => q.id)
    if (ids.length === 0) {
      message.warning(t('practice.msg.retryNoQuestions'))
      return
    }
    const res = await practiceApi.start({
      bankIds: item.bankId > 0 ? [item.bankId] : undefined,
      questionIds: ids,
      count: 0,
      order: item.order,
      shuffleOptions: item.shuffleOptions,
      // 透传原会话的题序区间，仅供列表展示（引擎对指定题目模式不做范围筛选）
      startSeq: item.rangeStart > 0 ? item.rangeStart : undefined,
      endSeq: item.rangeEnd > 0 ? item.rangeEnd : undefined,
    })
    applySession({
      session: res.session,
      questions: res.questions,
      answers: [],
      marked: [],
      lastSeq: 1,
      stats: wrongStore.stats,
    })
    router.replace({ path: '/practice', query: { session: String(res.session.id) } })
  } catch {
    message.error(t('practice.msg.retryFailed'))
  } finally {
    starting.value = false
  }
}

function savePosition() {
  if (!session.value) return
  practiceApi.position(session.value.id, index.value + 1, [...markedIds.value]).catch(() => {})
}

function optionClass(opt: PracticeOption) {
  const cls: Record<string, boolean> = { selected: chosen.value.includes(opt.key) }
  const answer = currentAnswer.value
  if (answer) {
    const correct = answer.result.answer.includes(opt.label)
    cls.correct = correct
    cls.wrong = chosen.value.includes(opt.key) && !correct
  }
  return cls
}

function cellClass(i: number) {
  const q = questions.value[i]
  const answer = q ? answers[q.id] : undefined
  const marked = !!q && markedIds.value.includes(q.id)
  return {
    current: i === index.value,
    ok: !!answer?.result.isCorrect,
    bad: !!answer && !answer.result.isCorrect,
    marked,
  }
}

function toggleMark() {
  if (!current.value) return
  const id = current.value.id
  const pos = markedIds.value.indexOf(id)
  if (pos >= 0) markedIds.value.splice(pos, 1)
  else markedIds.value.push(id)
  savePosition()
}

function toggleOption(key: string) {
  if (submitting.value || currentAnswer.value || !current.value) return
  if (current.value.type === 'multiple') {
    const pos = chosen.value.indexOf(key)
    if (pos >= 0) chosen.value.splice(pos, 1)
    else chosen.value.push(key)
    chosen.value.sort()
  } else {
    chosen.value = [key]
    void submitAnswer()
  }
}

async function submitAnswer() {
  if (!session.value || !current.value || chosen.value.length === 0) return
  submitting.value = true
  try {
    const spent = Math.min(Math.max(Date.now() - questionStartedAt, 0), 30 * 60 * 1000)
    const result = await practiceApi.answer(session.value.id, current.value.id, chosen.value, spent)
    answers[current.value.id] = { chosen: [...chosen.value], result }
    wrongStore.setStats(result.stats)
    session.value.answeredCount = answeredCount.value
    session.value.correctCount = correctCount.value
    session.value.durationSec = result.durationSec
    // 答对保持安静，只有答错才提示并说明已计入错题本
    if (!result.isCorrect) {
      message.warning(
        t('practice.msg.wrongAnswer', {
          answer: result.answer.join('') || '—',
          n: result.wrongCount,
        }),
      )
    }
  } finally {
    submitting.value = false
  }
}

function isTypingTarget(target: EventTarget | null) {
  const el = target as HTMLElement | null
  if (!el) return false
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable
}

function hasOpenLayer() {
  return !!document.querySelector('.ant-modal-wrap, .ant-popover:not(.ant-popover-hidden)')
}

function onKeydown(e: KeyboardEvent) {
  if (!session.value || !current.value || submitting.value) return
  if (e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target) || hasOpenLayer()) return
  const q = current.value
  const upper = e.key.length === 1 ? e.key.toUpperCase() : e.key
  const letterIdx =
    e.key.length === 1 && upper >= 'A' && upper <= 'H' ? upper.charCodeAt(0) - 65 : -1
  const numberIdx = e.key.length === 1 && e.key >= '1' && e.key <= '8' ? Number(e.key) - 1 : -1
  const optIdx = letterIdx >= 0 ? letterIdx : numberIdx
  if (optIdx >= 0) {
    const opt = q.options[optIdx]
    if (!opt) return
    e.preventDefault()
    toggleOption(opt.key)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    if (currentAnswer.value) go(index.value + 1)
    else if (q.type === 'multiple' && chosen.value.length > 0) void submitAnswer()
    return
  }
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    go(index.value - 1)
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    go(index.value + 1)
  }
}

function go(next: number) {
  if (!session.value) return
  if (next < 0 || next >= session.value.totalCount) return
  // 切题即收起 AI 面板：会话已随切题作废，留着只会挡住答题卡并在切题瞬间自动发新首问
  aiOpen.value = false
  index.value = next
  const item = questions.value[next]
  chosen.value = item && answers[item.id] ? [...answers[item.id].chosen] : []
  questionStartedAt = Date.now()
  // 换题后把题目区滚回顶部，避免停在上一题的解析位置
  if (bodyRef.value) bodyRef.value.scrollTop = 0
  savePosition()
}

async function finish() {
  if (!session.value) return
  try {
    summary.value = await practiceApi.submit(session.value.id)
    finished.value = true
    session.value = null
    questions.value = []
    chosen.value = []
    await loadSessions()
  } catch {
    message.error(t('practice.msg.finishFailed'))
  }
}

function backToSettings() {
  finished.value = false
  summary.value = null
  loadSessions()
}

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  await Promise.all([loadBanks(), loadSessions(), loadSettings(), wrongStore.refresh(), loadAiConfig()])
  const sessionId = Number(route.query.session ?? 0)
  if (sessionId > 0) {
    // 地址栏带会话参数（刷新 / 分享链接）：直接恢复答题界面
    await resume(sessionId)
  } else if (route.query.source === 'wrong') {
    await startWrong()
  }
  // 无参数：留在设置页（练习的家），进行中的练习靠「最近练习 → 继续」进入
  bootReady.value = true
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  savePosition()
})
</script>

<style scoped>
.page { min-height: 100%; display: flex; flex-direction: column; }
/* 答题态锁定在一屏内，长内容由题目区内部滚动，不再把底部按钮顶出屏幕 */
.page-quiz { height: 100%; min-height: 0; overflow: hidden; }
.page-quiz > :first-child { flex: none; }
.grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(0, 1.6fr);
  gap: 12px;
}
.panel-body.empty-state { min-height: 300px; }
.quiz-layout {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 200px;
  grid-template-rows: minmax(0, 1fr);
  gap: 12px;
  /* 打开 AI 时右列从答题卡的 200px 展开成聊天列，宽度过渡和内容切换同步 */
  transition: grid-template-columns 0.28s ease;
}
.quiz-layout.ai-open { grid-template-columns: minmax(0, 1fr) 440px; }
.side-col { display: flex; flex-direction: column; min-height: 0; }
.side-col > * { flex: 1; min-height: 0; }

/* v-show 切换：display 翻转时动画重放，效果等同之前的 Transition；两面板常驻不销毁 */
.ai-host { animation: ai-in 0.22s ease; }
.answer-card { animation: card-in 0.12s ease; }
@keyframes ai-in {
  from { opacity: 0; transform: translateX(28px); }
  to { opacity: 1; transform: none; }
}
@keyframes card-in {
  from { opacity: 0; transform: translateY(-8px) scale(0.98); }
  to { opacity: 1; transform: none; }
}
.quiz { display: flex; flex-direction: column; padding: 18px 20px 20px; min-height: 0; overflow: hidden; }
/* 题干、选项、解析在这一层内部滚动，底部操作条永远留在可视区 */
.quiz-body {
  flex: 1; min-height: 0; display: flex; flex-direction: column;
  overflow-y: auto; padding-right: 4px;
  /* 长解析撑出滚动条时预留轨道位，内容不因滚动条出现/消失而左右晃动 */
  scrollbar-gutter: stable;
}
.quiz-body, .q-grid { scrollbar-width: thin; scrollbar-color: var(--scroll-thumb) transparent; }
.quiz-body::-webkit-scrollbar,
.q-grid::-webkit-scrollbar { width: 6px; height: 6px; }
.quiz-body::-webkit-scrollbar-track,
.q-grid::-webkit-scrollbar-track { background: transparent; }
.quiz-body::-webkit-scrollbar-thumb,
.q-grid::-webkit-scrollbar-thumb { background: var(--scroll-thumb); border-radius: 99px; }
.quiz-body::-webkit-scrollbar-thumb:hover,
.q-grid::-webkit-scrollbar-thumb:hover { background: var(--scroll-thumb-hover); }
.quiz-body::-webkit-scrollbar-corner,
.q-grid::-webkit-scrollbar-corner { background: transparent; }
.quiz-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.quiz-head .muted { margin-left: 8px; font-size: 13px; }
.progress-hint { opacity: .8; }
.head-right { display: flex; align-items: center; gap: 10px; }
.mark-btn {
  width: 26px; height: 24px; padding: 0;
  display: inline-flex; align-items: center; justify-content: center;
  color: var(--text2);
}
.mark-btn :deep(.ant-btn-icon) { display: inline-flex; align-items: center; justify-content: center; line-height: 1; }
.mark-btn:hover,
.mark-btn.ant-btn-default:not(:disabled):hover,
.mark-btn.ant-btn-default:not(:disabled):focus { color: var(--mark); border-color: var(--mark-border); }
.mark-btn.marked {
  color: var(--mark);
  background: var(--mark-soft);
  border-color: var(--mark-border);
}
.mark-btn.marked:hover,
.mark-btn.marked.ant-btn-default:not(:disabled):hover { color: var(--mark); background: var(--mark-soft); border-color: var(--mark); }
/* 解析编辑图标按钮：在 ai-ask-btn 配色基础上做图标居中，三个按钮统一 24px 高 */
.icon-act {
  width: 26px; height: 24px; padding: 0;
  display: inline-flex; align-items: center; justify-content: center;
  box-shadow: none;
}
.icon-act.ant-btn-primary:not(:disabled):hover,
.icon-act.ant-btn-primary:not(:disabled):focus { box-shadow: none; }
.icon-act :deep(.ant-btn-icon) { display: inline-flex; align-items: center; justify-content: center; line-height: 1; }
.answer-card { display: flex; flex-direction: column; padding: 14px 12px 12px; min-height: 0; }
.side-head { display: flex; align-items: baseline; justify-content: space-between; }
.side-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.side-legend { display: flex; flex-wrap: wrap; gap: 4px 10px; margin: 10px 0; font-size: 11.5px; color: var(--text2); }
.side-legend span { display: inline-flex; align-items: center; gap: 4px; }
.dot { width: 9px; height: 9px; border-radius: 2px; border: 1px solid var(--border); background: var(--surface); }
.dot.ok { background: var(--ok-soft); border-color: var(--ok-text); }
.dot.bad { background: var(--bad-soft); border-color: var(--bad-text); }
.dot.mark { background: var(--mark-soft); border-color: var(--mark); }
.q-grid {
  flex: 1; min-height: 0;
  overflow-y: auto; overflow-x: hidden;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(24px, 1fr));
  align-content: start; gap: 6px;
  /* 滚动条：常驻预留宽度，出条前后列宽不跳动；
     padding 给格子与滚动条之间留出间距，负 margin 把滚动条往面板右边推、不浪费空白。
     两者等值（8/8）互相抵消：盒子右移但内容区宽度不变，列宽与加 padding 前完全一致。
     想再往右就把这对数值同步加大（如 10/10），列宽不受影响；
     上限是面板 .answer-card 的 padding-right（12px），超出会顶出面板。
     最小列宽 24px 而非 26px：不同平台滚动条宽度不同（6–17px），留出余量避免少一列 */
  padding-right: 8px; margin-right: -8px;
  scrollbar-gutter: stable;
}
.q-cell {
  position: relative;
  height: 26px; padding: 0; border: 1px solid var(--border); border-radius: 4px;
  background: var(--surface); color: var(--text2);
  font-size: 11.5px; line-height: 1; cursor: pointer;
  transition: border-color .12s, background .12s, color .12s, box-shadow .12s;
}
.q-cell:hover { border-color: var(--primary); color: var(--primary); }
/* 状态：浅色底 + 饱和边框 + 同色数字；声明顺序即叠加优先级（答错 > 标记 > 答对） */
.q-cell.ok { background: var(--ok-soft); border-color: var(--ok-text); color: var(--ok-text); }
.q-cell.marked { background: var(--mark-soft); border-color: var(--mark); color: var(--mark-text); }
.q-cell.bad { background: var(--bad-soft); border-color: var(--bad-text); color: var(--bad-text); }
.q-cell.ok:hover, .q-cell.bad:hover, .q-cell.marked:hover { filter: brightness(.985); }
/* 当前题：一律用主题绿实心填充，与这题是答对、答错还是标记无关 */
.q-cell.current,
.q-cell.current:hover {
  color: var(--surface);
  background: var(--primary);
  border-color: var(--primary);
  font-weight: 600;
}
.side-foot { margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border); text-align: right; }
.q-type { margin-top: 18px; font-size: 12.5px; color: var(--text2); }
.q-stem { margin-top: 6px; font-size: 16px; line-height: 1.6; white-space: pre-wrap; }
.q-img { display: block; margin-top: 10px; }
.opt-thumb { flex: none; }
.options { margin-top: 14px; display: flex; flex-direction: column; gap: 8px; }
.multi-submit { display: flex; justify-content: flex-end; margin-top: 10px; }
.opt {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 11px 12px; border: 1px solid var(--border-strong); border-radius: 8px;
  background: var(--surface-2); color: var(--text); cursor: pointer;
  transition: border-color .12s, background .12s;
}
.opt:hover { background: var(--hover); }
.opt.selected { border-color: #24945f; background: var(--ok-soft); }
.opt.correct { border-color: #24945f; background: var(--ok-soft); }
.opt.wrong { border-color: var(--bad-text); background: var(--bad-soft); }
.opt-key {
  flex: none; width: 24px; height: 24px; border-radius: 50%;
  border: 1.5px solid var(--border-strong); color: var(--text2);
  font-size: 12.5px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
}
.opt.selected .opt-key, .opt.correct .opt-key { background: #24945f; border-color: #24945f; color: #fff; }
.opt.wrong .opt-key { background: var(--bad-text); border-color: var(--bad-text); color: #fff; }
/* 行高与 24px 圆形字母齐平：antd reset 的 html{line-height:1.15} 会让首行只有 16px，
   flex-start 对齐后文字整体比圆圈高 2~3px，这里固定成 22px 让两者视觉居中一致 */
.opt-text { flex: 1; white-space: pre-wrap; line-height: 22px; }
/* 解析框贴内容高度：没解析时不占地方，解析长了自然长高、随 quiz-body 滚动 */
.quiz-body > *:not(.analysis-box) { flex: none; }
.analysis-box {
  flex: none; margin-top: 14px;
  padding: 12px 14px; border: 1px dashed var(--border); border-radius: 8px;
  background: var(--bg); font-size: 13.5px; line-height: 1.7;
  overflow-wrap: anywhere; word-break: break-word; min-width: 0;
}
.analysis-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 6px; }
.analysis-title { font-size: 13px; font-weight: 600; }
.ai-ask-btn { height: 24px; padding: 0 8px; font-size: 12px; }
/* 解析框右上角按钮组：显式 flex 垂直居中，图标与文字按钮严格对齐 */
.analysis-actions { display: flex; align-items: center; gap: 4px; }
/* 去掉点击瞬间的按压背景，收起/问 AI 都保持干净 */
.ai-ask-btn.ant-btn-default:not(:disabled):active,
.ai-ask-btn.ant-btn-default:not(:disabled):focus,
.ai-ask-btn.ant-btn-default:not(:disabled):hover { background: transparent; }
/* AI 未配置引导：柔和药丸链接样式，与「问 AI」同位同尺寸 */
.ai-setup-hint {
  height: 24px;
  padding: 0 10px;
  font-size: 12px;
  border: none;
  border-radius: 999px;
  background: var(--primary-soft);
  color: var(--primary);
}
.analysis-text { color: var(--text); white-space: pre-wrap; }
/* 就地编辑解析：去掉输入框的边框和底色，视觉上直接在解析框里改字 */
/* 高度由 auto-size 跟随内容，禁止内部滚动条（内容多时由外层 quiz-body 滚动） */
.analysis-editor,
.analysis-editor:focus,
.analysis-editor:hover {
  padding: 0; border: none; box-shadow: none;
  background: transparent;
  color: var(--text); font-size: 13.5px; line-height: 1.7;
  overflow-y: hidden !important; resize: none;
}
.quiz-actions {
  display: flex; justify-content: space-between; align-items: center;
  padding-top: 12px; margin-top: 12px; border-top: 1px solid var(--border);
  flex-wrap: wrap; gap: 10px;
}
.kbd-hint { margin-top: 8px; font-size: 12px; text-align: right; }
.result { margin-top: 0; }
.switch-hint { margin-left: 10px; font-size: 12.5px; }
.range-row { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
/* 难度与标签并排，省一行高度 */
.form-row { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 24px; }
.form-row :deep(.ant-form-item) { margin-bottom: 0; min-width: 0; }
.form-row .field-narrow { flex: 0 0 38%; }
.form-row .field-wide { flex: 1 1 auto; }
.form-row .field-range { flex: 1 1 auto; }
/* 题量：标签竖排在下拉正上方，样式与表单标签（如“标签”“题序范围”）保持一致 */
.count-field { position: relative; display: inline-flex; align-items: center; margin-left: 10px; flex: none; }
.count-label {
  position: absolute; left: 1px; bottom: calc(100% + 4px);
  font-size: 14px; line-height: 1; color: var(--text);
  white-space: nowrap; pointer-events: none;
}
.count-select { width: 128px; flex: none; }
.range-input { width: 72px; flex: none; }
.range-sep { color: var(--text2); }
.range-unit { font-size: 12.5px; }
.range-hint { margin-top: 4px; font-size: 12px; line-height: 1.5; }
.range-next { padding: 0; height: auto; font-size: 12px; }
.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; }
.recent-main { min-width: 0; }
.recent-title { font-weight: 600; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.recent-tag { margin-left: 2px; }
.recent-right { display: flex; align-items: center; gap: 10px; flex: none; }
.recent-score { display: flex; flex-direction: column; align-items: flex-end; font-size: 13px; }
.recent-continue { padding-inline: 4px; }
@media (max-width: 900px) {
  .grid { grid-template-columns: 1fr; }
  .form-row { flex-wrap: wrap; gap: 0; }
  .form-row .field-narrow { flex: 0 0 100%; margin-bottom: 16px !important; }
  /* 窄屏（手机/分屏）下面板纵向堆叠，改为随文档自然滚动：
     桌面那套「容器定高 + 内部滚动」在堆叠布局下会把题干区压成 0 高，题就没了 */
  .page-quiz { height: auto; min-height: 100%; overflow: visible; }
  .quiz-layout,
  .quiz-layout.ai-open { display: flex; flex-direction: column; }
  .quiz { overflow: visible; }
  .quiz-body { flex: none; overflow-y: visible; }
  .side-col { min-height: 0; }
  .side-col > * { flex: none; min-height: 0; }
  .q-grid { max-height: 170px; }
}
</style>
