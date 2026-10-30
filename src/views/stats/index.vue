<template>
  <div class="page">
    <PageHeader title="统计" subtitle="答题量、正确率与练习趋势">
      <template #actions>
        <a-segmented v-model:value="range" :options="ranges" />
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile label="答题总数" :value="stats?.answeredTotal ?? 0" :hint="rangeHint" />
      <MetricTile label="正确率" :value="accuracyText" :hint="accuracyHint" />
      <MetricTile label="练习次数" :value="stats?.practiceCount ?? 0" hint="按练习创建时间统计" />
      <MetricTile label="考试次数" :value="stats?.examCount ?? 0" :hint="examHint" />
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>正确率趋势</h3>
        <span class="muted">柱=答题量 · 线=正确率 · 按天汇总</span>
      </div>
      <div class="panel-body chart-body">
        <a-spin v-if="!stats" />
        <a-empty v-else-if="!hasData" description="这段时间还没有答题记录">
          <a-button type="primary" ghost @click="router.push('/practice')">去练习</a-button>
        </a-empty>
        <div v-show="hasData" ref="chartEl" class="chart" />
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>题库正确率</h3>
        <span class="muted">练习与考试合计</span>
      </div>
      <div class="panel-body bank-body" :class="{ empty: stats && !banks.length }">
        <a-empty v-if="stats && !banks.length" description="暂无题库维度数据" />
        <ul v-else class="bank-list">
          <li v-for="bank in banks" :key="bank.bankId">
            <div class="bl-top">
              <span class="bl-name">{{ bank.bankName }}</span>
              <span class="bl-rate">{{ rateOf(bank.correct, bank.answered) }}%</span>
            </div>
            <div class="bl-bar">
              <span :style="{ width: rateOf(bank.correct, bank.answered) + '%' }" />
            </div>
            <div class="bl-meta">共 {{ bank.answered }} 题 · 答对 {{ bank.correct }} 题</div>
          </li>
        </ul>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>标签正确率</h3>
        <span class="muted">找薄弱知识点 · 练习与考试合计</span>
      </div>
      <div class="panel-body bank-body" :class="{ empty: loaded && !tags.length }">
        <a-empty v-if="loaded && !tags.length" description="暂无标签维度数据，给题目打上标签后即可统计" />
        <ul v-else class="bank-list">
          <li v-for="tag in tags" :key="tag.tagId">
            <div class="bl-top">
              <span class="bl-name">{{ tag.tagName }}</span>
              <span class="bl-rate" :class="{ weak: rateOf(tag.correct, tag.answered) < 60 }">
                {{ rateOf(tag.correct, tag.answered) }}%
              </span>
            </div>
            <div class="bl-bar">
              <span
                :class="{ weak: rateOf(tag.correct, tag.answered) < 60 }"
                :style="{ width: rateOf(tag.correct, tag.answered) + '%' }"
              />
            </div>
            <div class="bl-meta">共 {{ tag.answered }} 题 · 答对 {{ tag.correct }} 题</div>
          </li>
        </ul>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'
import { statsApi, type Stats, type StatsTag } from '@/api/stats'
import { isDark } from '@/theme'

echarts.use([BarChart, LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const router = useRouter()
const range = ref<'7d' | '30d' | 'all'>('7d')
const ranges = [
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
  { label: '全部', value: 'all' },
]
const rangeHint = computed(() => ranges.find((r) => r.value === range.value)?.label ?? '')

const stats = ref<Stats | null>(null)
const tags = ref<StatsTag[]>([])
const loaded = ref(false)
const banks = computed(() => stats.value?.banks ?? [])
const hasData = computed(() => (stats.value?.answeredTotal ?? 0) > 0)

const accuracyText = computed(() => {
  const data = stats.value
  if (!data || data.answeredTotal === 0) return '—'
  return `${Math.round((data.correctTotal / data.answeredTotal) * 100)}%`
})

const accuracyHint = computed(() => {
  const data = stats.value
  if (!data || data.answeredTotal === 0) return '完成练习或考试后生成'
  return `答对 ${data.correctTotal} / ${data.answeredTotal} 题`
})

const examHint = computed(() => {
  const data = stats.value
  if (!data || data.examScoredCount === 0) return '暂无交卷记录'
  return `已交卷 ${data.examCount} 份 · 平均 ${data.examAverageScore.toFixed(1)} 分`
})

function rateOf(correct: number, answered: number) {
  return answered ? Math.round((correct / answered) * 100) : 0
}

const chartEl = ref<HTMLDivElement | null>(null)
let chart: ReturnType<typeof echarts.init> | null = null

function palette() {
  const css = getComputedStyle(document.documentElement)
  const dark = isDark.value
  return {
    text: css.getPropertyValue('--text2').trim() || (dark ? '#98a1ac' : '#6f7680'),
    border: css.getPropertyValue('--border').trim() || (dark ? '#2a313b' : '#e4e8e6'),
    primary: css.getPropertyValue('--primary').trim() || '#24945f',
    bar: dark ? 'rgba(53, 176, 122, 0.32)' : 'rgba(36, 148, 95, 0.20)',
    surface: css.getPropertyValue('--surface').trim() || (dark ? '#161b22' : '#ffffff'),
  }
}

function chartOption() {
  const p = palette()
  const trend = stats.value?.trend ?? []
  return {
    animationDuration: 300,
    grid: { left: 6, right: 6, top: 34, bottom: 2, containLabel: true },
    legend: {
      top: 0,
      right: 0,
      itemWidth: 10,
      itemHeight: 10,
      itemGap: 14,
      textStyle: { color: p.text, fontSize: 11 },
      data: ['答题量', '正确率'],
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: p.surface,
      borderColor: p.border,
      textStyle: { color: p.text, fontSize: 12 },
      formatter: (params: unknown) => {
        const items = params as Array<{
          axisValue?: string
          seriesName?: string
          value?: number | null
          dataIndex?: number
        }>
        const idx = items[0]?.dataIndex ?? 0
        const point = trend[idx]
        if (!point) return ''
        const rate = point.answered ? `${Math.round((point.correct / point.answered) * 100)}%` : '—'
        return `${point.date}<br/>答题 ${point.answered} 题<br/>答对 ${point.correct} 题<br/>正确率 ${rate}`
      },
    },
    xAxis: {
      type: 'category',
      data: trend.map((d) => d.date.slice(5)),
      axisLine: { lineStyle: { color: p.border } },
      axisTick: { show: false },
      axisLabel: { color: p.text, fontSize: 11, hideOverlap: true },
    },
    yAxis: [
      {
        type: 'value',
        minInterval: 1,
        splitLine: { lineStyle: { color: p.border, type: 'dashed' } },
        axisLabel: { color: p.text, fontSize: 11 },
      },
      {
        type: 'value',
        min: 0,
        max: 100,
        splitLine: { show: false },
        axisLabel: { color: p.text, fontSize: 11, formatter: '{value}%' },
      },
    ],
    series: [
      {
        name: '答题量',
        type: 'bar',
        data: trend.map((d) => d.answered),
        barMaxWidth: 18,
        itemStyle: { color: p.bar, borderRadius: [3, 3, 0, 0] },
      },
      {
        name: '正确率',
        type: 'line',
        yAxisIndex: 1,
        data: trend.map((d) => (d.answered ? Math.round((d.correct / d.answered) * 100) : null)),
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        connectNulls: false,
        lineStyle: { width: 2, color: p.primary },
        itemStyle: { color: p.primary },
      },
    ],
  }
}

async function renderChart() {
  await nextTick()
  if (!hasData.value || !chartEl.value) {
    chart?.dispose()
    chart = null
    return
  }
  if (!chart) chart = echarts.init(chartEl.value)
  chart.setOption(chartOption(), true)
  chart.resize()
}

async function load() {
  const [data, tagData] = await Promise.all([statsApi.get(range.value), statsApi.tags(range.value)])
  stats.value = data
  tags.value = tagData.items
  loaded.value = true
}

function onResize() {
  chart?.resize()
}

watch(range, load)
watch([stats, isDark], renderChart)

onMounted(() => {
  load()
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
})
</script>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
}
.panel + .panel { margin-top: 12px; }
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
}
.panel-head h3 { margin: 0; font-size: 14px; font-weight: 600; }
.muted { font-size: 11px; color: var(--text2); }
.panel-body { padding: 16px; flex: 1; }
.chart-body { display: flex; align-items: center; justify-content: center; min-height: 300px; }
.chart { width: 100%; height: 300px; }
.bank-body.empty { display: flex; align-items: center; justify-content: center; min-height: 160px; }
.bank-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 14px; }
.bl-top { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.bl-name { font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bl-rate { font-size: 13px; font-weight: 700; color: var(--primary); flex: none; }
.bl-rate.weak { color: #d4380d; }
.bl-bar {
  margin-top: 6px;
  height: 6px;
  border-radius: 3px;
  background: var(--hover);
  overflow: hidden;
}
.bl-bar span { display: block; height: 100%; border-radius: 3px; background: var(--primary); }
.bl-bar span.weak { background: #d4380d; }
.bl-meta { margin-top: 5px; font-size: 11px; color: var(--text2); }
@media (max-width: 1100px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
