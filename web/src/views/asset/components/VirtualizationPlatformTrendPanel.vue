<template>
  <div class="virtualization-trend-panel">
    <div class="trend-toolbar">
      <div class="trend-summary">
        <div class="trend-title-row">
          <span class="trend-title">{{ title || '平台状态趋势' }}</span>
          <el-tag size="small" effect="plain">{{ scopeText }}</el-tag>
        </div>
        <span class="trend-meta">
          <template v-if="trend?.end">截止 {{ trend.end }}</template>
          <template v-else>等待同步数据</template>
        </span>
      </div>
      <el-radio-group :model-value="range" size="small" @update:model-value="handleRangeChange">
        <el-radio-button label="24h">近 24 小时</el-radio-button>
        <el-radio-button label="7d">近 7 天</el-radio-button>
        <el-radio-button label="15d">近 15 天</el-radio-button>
      </el-radio-group>
    </div>

    <div class="metric-filter-row">
      <div class="metric-filter-title">展示指标</div>
      <el-checkbox-group :model-value="selectedMetrics" @update:model-value="handleMetricChange">
        <el-checkbox
          v-for="item in metricOptions"
          :key="item.value"
          :label="item.value"
          class="metric-checkbox"
        >
          <span class="metric-dot" :style="{ background: item.color || metricColorMap[item.value] || '#2563eb' }" />
          {{ item.label }}
        </el-checkbox>
      </el-checkbox-group>
    </div>

    <div v-if="latestPoint" class="metric-summary-grid">
      <div v-for="item in visibleMetricOptions" :key="item.value" class="metric-summary-card">
        <div class="metric-summary-label">{{ item.label }}</div>
        <div class="metric-summary-value">{{ latestPoint[item.value] }}</div>
      </div>
    </div>

    <div v-loading="loading" class="trend-body">
      <div v-if="hasTrendData && visibleMetricOptions.length > 0" ref="chartRef" class="trend-chart"></div>
      <el-empty v-else description="暂无趋势数据或未选择展示指标" :image-size="80" />
    </div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

export type TrendRange = '24h' | '7d' | '15d'
export type TrendMetricKey =
  | 'guestTotal'
  | 'poweredOnGuests'
  | 'poweredOffGuests'
  | 'suspendedGuests'
  | 'boundGuests'
  | 'onlineGuests'
  | 'offlineGuests'
  | 'notConfiguredGuests'
  | 'unknownGuests'

interface VirtualizationPlatformTrendPoint {
  timestamp: number
  time: string
  guestTotal: number
  poweredOnGuests: number
  poweredOffGuests: number
  suspendedGuests: number
  boundGuests: number
  onlineGuests: number
  offlineGuests: number
  notConfiguredGuests: number
  unknownGuests: number
}

interface VirtualizationPlatformTrendData {
  scopeType?: 'platform' | 'cluster'
  scopeId?: number
  scopeName?: string
  platformId: number
  clusterId?: number
  range: TrendRange
  start: string
  end: string
  points: VirtualizationPlatformTrendPoint[]
}

interface MetricOption {
  label: string
  value: TrendMetricKey
  color?: string
}

const props = withDefaults(defineProps<{
  trend: VirtualizationPlatformTrendData | null
  loading?: boolean
  range: TrendRange
  title?: string
  selectedMetrics: TrendMetricKey[]
  metricOptions: MetricOption[]
}>(), {
  trend: null,
  loading: false,
  title: '',
  selectedMetrics: () => [],
  metricOptions: () => []
})

const emit = defineEmits<{
  (e: 'change-range', value: TrendRange): void
  (e: 'change-metrics', value: TrendMetricKey[]): void
}>()

const chartRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

const metricColorMap: Record<TrendMetricKey, string> = {
  guestTotal: '#111827',
  poweredOnGuests: '#2563eb',
  poweredOffGuests: '#94a3b8',
  suspendedGuests: '#f59e0b',
  boundGuests: '#16a34a',
  onlineGuests: '#059669',
  offlineGuests: '#dc2626',
  notConfiguredGuests: '#7c3aed',
  unknownGuests: '#475569'
}

const hasTrendData = computed(() => (props.trend?.points || []).length > 0)
const latestPoint = computed(() => {
  const points = props.trend?.points || []
  return points.length > 0 ? points[points.length - 1] : null
})
const visibleMetricOptions = computed(() => {
  const selected = new Set(props.selectedMetrics)
  return (props.metricOptions || []).filter(item => selected.has(item.value))
})
const scopeText = computed(() => {
  if (props.trend?.scopeType === 'cluster') return '集群趋势'
  return '平台趋势'
})

const xAxisLabels = computed(() => {
  return (props.trend?.points || []).map((point) => {
    const date = new Date(point.timestamp * 1000)
    const month = `${date.getMonth() + 1}`.padStart(2, '0')
    const day = `${date.getDate()}`.padStart(2, '0')
    const hour = `${date.getHours()}`.padStart(2, '0')
    const minute = `${date.getMinutes()}`.padStart(2, '0')
    if (props.range === '24h') {
      return `${hour}:${minute}`
    }
    return `${month}-${day}`
  })
})

const buildSeries = (field: TrendMetricKey) => {
  return (props.trend?.points || []).map(point => point[field] as number)
}

const handleRangeChange = (value: string | number | boolean) => {
  emit('change-range', value as TrendRange)
}

const handleMetricChange = (value: Array<string | number | boolean>) => {
  emit('change-metrics', value as TrendMetricKey[])
}

const renderChart = async () => {
  if (!chartRef.value || !hasTrendData.value || visibleMetricOptions.value.length === 0) {
    if (chart) {
      chart.dispose()
      chart = null
    }
    return
  }

  await nextTick()
  if (!chartRef.value) return

  if (!chart) {
    chart = echarts.init(chartRef.value)
  }

  const series = visibleMetricOptions.value.map((item, index) => {
    const isPrimary = index === 0
    return {
      name: item.label,
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: isPrimary ? 6 : 5,
      data: buildSeries(item.value),
      lineStyle: item.value === 'offlineGuests' ? { type: 'dashed' } : undefined,
      areaStyle: isPrimary
        ? {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: `${item.color || metricColorMap[item.value]}33` },
              { offset: 1, color: `${item.color || metricColorMap[item.value]}08` }
            ])
          }
        : undefined
    }
  })

  chart.setOption({
    color: visibleMetricOptions.value.map(item => item.color || metricColorMap[item.value] || '#2563eb'),
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(15, 23, 42, 0.94)',
      borderWidth: 0,
      textStyle: { color: '#f8fafc' },
      formatter(params: any[]) {
        const point = props.trend?.points?.[params?.[0]?.dataIndex || 0]
        const header = point?.time || '-'
        const lines = params.map(item => `${item.marker}${item.seriesName}：${item.value}`)
        return [header, ...lines].join('<br/>')
      }
    },
    legend: {
      top: 8,
      left: 12,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: {
        color: '#4b5563',
        fontSize: 12
      }
    },
    grid: {
      top: 58,
      left: 42,
      right: 18,
      bottom: 28
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: xAxisLabels.value,
      axisLabel: {
        color: '#6b7280',
        fontSize: 11
      },
      axisLine: {
        lineStyle: {
          color: '#d1d5db'
        }
      }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: {
        color: '#6b7280',
        fontSize: 11
      },
      splitLine: {
        lineStyle: {
          color: '#eef2f7'
        }
      }
    },
    series
  })

  chart.resize()
}

const handleResize = () => {
  chart?.resize()
}

watch(
  () => [props.trend, props.loading, props.range, props.selectedMetrics],
  () => {
    renderChart()
  },
  { deep: true }
)

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  if (chart) {
    chart.dispose()
    chart = null
  }
})
</script>

<style scoped>
.virtualization-trend-panel {
  background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
  border: 1px solid #e5e7eb;
  border-radius: 16px;
  padding: 16px 18px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.trend-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.trend-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.trend-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.trend-title {
  color: #111827;
  font-size: 15px;
  font-weight: 700;
}

.trend-meta {
  color: #6b7280;
  font-size: 12px;
}

.metric-filter-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid #e5e7eb;
  margin-bottom: 14px;
}

.metric-filter-title {
  color: #475569;
  font-size: 12px;
  font-weight: 600;
  padding-top: 6px;
}

.metric-checkbox {
  margin-right: 14px;
  margin-bottom: 8px;
}

.metric-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}

.metric-summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.metric-summary-card {
  border-radius: 12px;
  padding: 12px;
  border: 1px solid #e5e7eb;
  background: #fff;
}

.metric-summary-label {
  color: #64748b;
  font-size: 12px;
}

.metric-summary-value {
  margin-top: 8px;
  color: #0f172a;
  font-size: 24px;
  font-weight: 700;
}

.trend-body {
  min-height: 320px;
}

.trend-chart {
  height: 320px;
}

@media (max-width: 768px) {
  .trend-chart {
    height: 360px;
  }

  .metric-filter-row {
    padding: 10px;
  }
}
</style>
