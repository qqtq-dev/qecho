<template>
  <div class="page">
    <PageHeader title="统计" subtitle="答题量、正确率与练习趋势">
      <template #actions>
        <a-segmented v-model:value="range" :options="ranges" />
      </template>
    </PageHeader>

    <div class="metrics">
      <MetricTile label="答题总数" :value="0" :hint="rangeHint" />
      <MetricTile label="正确率" value="—" :hint="rangeHint" />
      <MetricTile label="练习次数" :value="0" :hint="rangeHint" />
      <MetricTile label="考试次数" :value="0" :hint="rangeHint" />
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>正确率趋势</h3>
        <span class="muted">按天汇总</span>
      </div>
      <div class="panel-body">
        <div class="chart-empty">
          <a-empty description="完成练习或考试后生成统计图表" />
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import MetricTile from '@/components/MetricTile.vue'

const range = ref('7d')
const ranges = [
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
  { label: '全部', value: 'all' },
]
const rangeHint = computed(() => ranges.find((r) => r.value === range.value)?.label ?? '')
</script>

<style scoped>
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.chart-empty {
  min-height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--border);
  border-radius: 6px;
}
@media (max-width: 1100px) {
  .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
