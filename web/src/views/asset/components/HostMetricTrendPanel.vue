<template>
  <div class="host-trend-panel">
    <div class="trend-toolbar">
      <div class="trend-summary">
        <span class="trend-summary-title">近时段资源趋势</span>
        <span class="trend-summary-meta">
          {{ stepLabel }}
          <template v-if="trend?.end"> · 截止 {{ trend.end }}</template>
        </span>
      </div>
      <el-radio-group :model-value="range" size="small" @update:model-value="handleRangeChange">
        <el-radio-button label="1h">近 1 小时</el-radio-button>
        <el-radio-button label="24h">近 24 小时</el-radio-button>
        <el-radio-button label="7d">近 7 天</el-radio-button>
        <el-radio-button label="15d">近 15 天</el-radio-button>
      </el-radio-group>
    </div>

    <div v-loading="loading" class="trend-body" :style="trendBodyStyle">
      <div v-if="hasTrendData" ref="chartRef" class="trend-chart" :style="trendChartStyle"></div>
      <el-empty v-else description="暂无趋势数据" :image-size="80" />
    </div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type TrendRange = '1h' | '24h' | '7d' | '15d'

interface HostMetricTrendPoint {
  timestamp: number
  time: string
  cpuUsage: number | null
  memoryUsage: number | null
  diskUsage: number | null
  load1?: number | null
  networkRecvRate?: number | null
  networkSendRate?: number | null
  diskReadRate?: number | null
  diskWriteRate?: number | null
}

interface HostMetricTrendData {
  hostId: number
  range: TrendRange
  stepSeconds: number
  start: string
  end: string
  points: HostMetricTrendPoint[]
}

const props = withDefaults(defineProps<{
  trend: HostMetricTrendData | null
  loading?: boolean
  range: TrendRange
}>(), {
  trend: null,
  loading: false
})

const emit = defineEmits<{
  (e: 'change-range', value: TrendRange): void
}>()

const chartRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null

type TrendMetricField =
  | 'cpuUsage'
  | 'memoryUsage'
  | 'diskUsage'
  | 'load1'
  | 'networkRecvRate'
  | 'networkSendRate'
  | 'diskReadRate'
  | 'diskWriteRate'

interface TrendSeriesMeta {
  name: string
  field: TrendMetricField
  xAxisIndex: number
  yAxisIndex: number
  color: string
  area?: boolean
  unit: 'percent' | 'load' | 'rate'
}

const trendSeries: TrendSeriesMeta[] = [
  { name: 'CPU', field: 'cpuUsage', xAxisIndex: 0, yAxisIndex: 0, color: '#2563eb', area: true, unit: 'percent' },
  { name: '内存', field: 'memoryUsage', xAxisIndex: 0, yAxisIndex: 0, color: '#16a34a', unit: 'percent' },
  { name: '磁盘', field: 'diskUsage', xAxisIndex: 0, yAxisIndex: 0, color: '#f59e0b', unit: 'percent' },
  { name: 'Load(1m)', field: 'load1', xAxisIndex: 1, yAxisIndex: 1, color: '#6366f1', unit: 'load' },
  { name: '网络接收', field: 'networkRecvRate', xAxisIndex: 2, yAxisIndex: 2, color: '#0ea5e9', unit: 'rate' },
  { name: '网络发送', field: 'networkSendRate', xAxisIndex: 2, yAxisIndex: 2, color: '#0891b2', unit: 'rate' },
  { name: '磁盘读取', field: 'diskReadRate', xAxisIndex: 2, yAxisIndex: 2, color: '#8b5cf6', unit: 'rate' },
  { name: '磁盘写入', field: 'diskWriteRate', xAxisIndex: 2, yAxisIndex: 2, color: '#c026d3', unit: 'rate' }
]

interface TrendChartLayout {
  chartHeight: number
  bodyMinHeight: number
  titleTops: [number, number, number]
  gridTops: [number, number, number]
  gridHeights: [number, number, number]
  gridBottom: number
  xAxisLabelInterval: number | 'auto'
  percentSplitNumber: number
  loadSplitNumber: number
  rateSplitNumber: number
}

const dailyTrendLayout = (labelInterval?: number | 'auto'): TrendChartLayout => {
  const pointCount = props.trend?.points?.length || 0
  const interval = labelInterval ?? (pointCount > 0 ? Math.max(Math.ceil(pointCount / 8) - 1, 0) : 0)
  return {
    chartHeight: 664,
    bodyMinHeight: 664,
    titleTops: [46, 266, 468],
    gridTops: [80, 300, 502],
    gridHeights: [140, 106, 138],
    gridBottom: 18,
    xAxisLabelInterval: interval,
    percentSplitNumber: 4,
    loadSplitNumber: 3,
    rateSplitNumber: 4
  }
}

const hourlyTrendLayout = (): TrendChartLayout => ({
  chartHeight: 576,
  bodyMinHeight: 576,
  titleTops: [42, 235, 400],
  gridTops: [72, 265, 430],
  gridHeights: [128, 88, 128],
  gridBottom: 16,
  xAxisLabelInterval: 'auto',
  percentSplitNumber: 5,
  loadSplitNumber: 4,
  rateSplitNumber: 5
})

const isLongRange = computed(() => props.range === '7d' || props.range === '15d')

const chartLayout = computed<TrendChartLayout>(() => {
  if (props.range === '1h') {
    return hourlyTrendLayout()
  }
  if (isLongRange.value) {
    return dailyTrendLayout(0)
  }
  return dailyTrendLayout()
})

const trendBodyStyle = computed(() => ({
  minHeight: `${chartLayout.value.bodyMinHeight}px`
}))

const trendChartStyle = computed(() => ({
  height: `${chartLayout.value.chartHeight}px`
}))

const hasTrendData = computed(() => {
  const points = props.trend?.points || []
  return points.some(point => (
    point.cpuUsage !== null ||
    point.memoryUsage !== null ||
    point.diskUsage !== null ||
    point.load1 !== null ||
    point.networkRecvRate !== null ||
    point.networkSendRate !== null ||
    point.diskReadRate !== null ||
    point.diskWriteRate !== null
  ))
})

const stepLabel = computed(() => {
  const seconds = props.trend?.stepSeconds || 0
  if (seconds >= 86400) {
    return `粒度 ${Math.round(seconds / 86400)}d`
  }
  if (seconds >= 3600) {
    return `粒度 ${Math.round(seconds / 3600)}h`
  }
  if (seconds >= 60) {
    return `粒度 ${Math.round(seconds / 60)}m`
  }
  return seconds > 0 ? `粒度 ${seconds}s` : '粒度 -'
})

const buildSeriesData = (field: keyof HostMetricTrendPoint) => {
  return (props.trend?.points || []).map(point => {
    const value = point[field]
    return typeof value === 'number' ? Number(value.toFixed(2)) : null
  })
}

const padNumber = (value: number) => `${value}`.padStart(2, '0')

const formatMonthDay = (timestamp: number) => {
  const date = new Date(timestamp * 1000)
  return `${padNumber(date.getMonth() + 1)}-${padNumber(date.getDate())}`
}

const formatHourMinute = (timestamp: number) => {
  const date = new Date(timestamp * 1000)
  return `${padNumber(date.getHours())}:${padNumber(date.getMinutes())}`
}

const formatFullDateTime = (timestamp: number) => {
  const date = new Date(timestamp * 1000)
  return `${date.getFullYear()}-${padNumber(date.getMonth() + 1)}-${padNumber(date.getDate())} ${padNumber(date.getHours())}:${padNumber(date.getMinutes())}`
}

const isSameCalendarDay = (current: HostMetricTrendPoint, previous: HostMetricTrendPoint) => {
  const currentDate = new Date(current.timestamp * 1000)
  const previousDate = new Date(previous.timestamp * 1000)
  return currentDate.getFullYear() === previousDate.getFullYear() &&
    currentDate.getMonth() === previousDate.getMonth() &&
    currentDate.getDate() === previousDate.getDate()
}

const formatXAxisLabel = (_value: string | number, index: number) => {
  const point = props.trend?.points?.[index]
  if (!point) {
    return ''
  }

  if (props.range === '1h') {
    return formatHourMinute(point.timestamp)
  }

  if (props.range === '24h') {
    const previousPoint = index > 0 ? props.trend?.points?.[index - 1] : null
    if (previousPoint && isSameCalendarDay(point, previousPoint)) {
      return formatHourMinute(point.timestamp)
    }
    return `${formatMonthDay(point.timestamp)} ${formatHourMinute(point.timestamp)}`
  }

  if (!isLongRange.value) {
    return point.time || formatFullDateTime(point.timestamp)
  }

  const previousPoint = props.trend?.points?.[index - 1]
  if (index === 0 || !previousPoint || !isSameCalendarDay(point, previousPoint)) {
    return formatMonthDay(point.timestamp)
  }
  return ''
}

const buildXAxisData = () => {
  return (props.trend?.points || []).map(point => point.timestamp)
}

const formatRate = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s']
  let current = Number(value)
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatLoad = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  return Number(value).toFixed(Number(value) >= 10 ? 1 : 2)
}

const formatLoadAxis = (value: number) => {
  if (!Number.isFinite(value)) return ''
  if (value >= 100) return `${Math.round(value)}`
  if (value >= 10) return value.toFixed(1)
  return value.toFixed(2)
}

const formatTooltipValue = (meta: TrendSeriesMeta, value: number | string) => {
  if (typeof value !== 'number') return '-'
  if (meta.unit === 'percent') {
    return `${value.toFixed(2)}%`
  }
  if (meta.unit === 'load') {
    return formatLoad(value)
  }
  return formatRate(value)
}

const buildTooltip = (params: any) => {
  const items = Array.isArray(params) ? params : [params]
  if (!items.length) return ''

  const dataIndex = typeof items[0]?.dataIndex === 'number' ? items[0].dataIndex : -1
  const headerPoint = dataIndex >= 0 ? props.trend?.points?.[dataIndex] : null
  const header = headerPoint?.time || (headerPoint ? formatFullDateTime(headerPoint.timestamp) : (items[0]?.axisValueLabel || items[0]?.name || ''))
  const lines = items
    .filter(item => item && item.seriesName)
    .map(item => {
      const meta = trendSeries.find(series => series.name === item.seriesName)
      const value = Array.isArray(item.value) ? item.value[1] : item.value
      const formatted = meta ? formatTooltipValue(meta, value) : value
      return `${item.marker}${item.seriesName}: ${formatted}`
    })

  return [header, ...lines].join('<br/>')
}

const renderChart = async () => {
  await nextTick()
  if (!chartRef.value || !hasTrendData.value) {
    chart?.dispose()
    chart = null
    return
  }

  if (!chart) {
    chart = echarts.init(chartRef.value)
  }

  const layout = chartLayout.value

  chart.setOption({
    animationDuration: 300,
    title: [
      {
        text: '资源占用',
        left: 16,
        top: layout.titleTops[0],
        textStyle: {
          fontSize: 13,
          fontWeight: 600,
          color: '#334155'
        }
      },
      {
        text: '负载',
        left: 16,
        top: layout.titleTops[1],
        textStyle: {
          fontSize: 13,
          fontWeight: 600,
          color: '#334155'
        }
      },
      {
        text: '吞吐',
        left: 16,
        top: layout.titleTops[2],
        textStyle: {
          fontSize: 13,
          fontWeight: 600,
          color: '#334155'
        }
      }
    ],
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(17, 24, 39, 0.92)',
      borderWidth: 0,
      padding: [10, 12],
      textStyle: {
        color: '#f9fafb'
      },
      formatter: buildTooltip
    },
    legend: {
      top: 0,
      type: 'scroll',
      data: trendSeries.map(series => series.name),
      icon: 'roundRect',
      itemWidth: 12,
      itemHeight: 8,
      textStyle: {
        color: '#475569'
      }
    },
    axisPointer: {
      link: [
        {
          xAxisIndex: [0, 1, 2]
        }
      ]
    },
    grid: [
      {
        left: 56,
        right: 20,
        top: layout.gridTops[0],
        height: layout.gridHeights[0]
      },
      {
        left: 56,
        right: 20,
        top: layout.gridTops[1],
        height: layout.gridHeights[1]
      },
      {
        left: 56,
        right: 20,
        top: layout.gridTops[2],
        height: layout.gridHeights[2],
        bottom: layout.gridBottom
      }
    ],
    xAxis: [
      {
        type: 'category',
        boundaryGap: false,
        gridIndex: 0,
        data: buildXAxisData(),
        axisLine: {
          lineStyle: {
            color: '#dbe4ee'
          }
        },
        axisLabel: {
          show: false
        },
        axisTick: {
          show: false
        }
      },
      {
        type: 'category',
        boundaryGap: false,
        gridIndex: 1,
        data: buildXAxisData(),
        axisLine: {
          lineStyle: {
            color: '#dbe4ee'
          }
        },
        axisLabel: {
          show: false
        },
        axisTick: {
          show: false
        }
      },
      {
        type: 'category',
        boundaryGap: false,
        gridIndex: 2,
        data: buildXAxisData(),
        axisLine: {
          lineStyle: {
            color: '#dbe4ee'
          }
        },
        axisLabel: {
          color: '#64748b',
          hideOverlap: true,
          interval: layout.xAxisLabelInterval,
          formatter: formatXAxisLabel
        },
        axisTick: {
          show: false
        }
      }
    ],
    yAxis: [
      {
        type: 'value',
        gridIndex: 0,
        min: 0,
        max: 100,
        splitNumber: layout.percentSplitNumber,
        axisLabel: {
          color: '#64748b',
          formatter: '{value}%'
        },
        splitLine: {
          lineStyle: {
            color: '#edf2f7'
          }
        }
      },
      {
        type: 'value',
        gridIndex: 1,
        min: 0,
        splitNumber: layout.loadSplitNumber,
        axisLabel: {
          color: '#64748b',
          formatter: formatLoadAxis
        },
        splitLine: {
          lineStyle: {
            color: '#edf2f7'
          }
        }
      },
      {
        type: 'value',
        gridIndex: 2,
        min: 0,
        splitNumber: layout.rateSplitNumber,
        axisLabel: {
          color: '#64748b',
          formatter: (value: number) => formatRate(value)
        },
        splitLine: {
          lineStyle: {
            color: '#edf2f7'
          }
        }
      }
    ],
    series: trendSeries.map(series => ({
      name: series.name,
      type: 'line',
      xAxisIndex: series.xAxisIndex,
      yAxisIndex: series.yAxisIndex,
      smooth: true,
      showSymbol: false,
      data: buildSeriesData(series.field),
      lineStyle: {
        width: 2,
        color: series.color
      },
      itemStyle: {
        color: series.color
      },
      areaStyle: series.area ? {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(37, 99, 235, 0.18)' },
          { offset: 1, color: 'rgba(37, 99, 235, 0.02)' }
        ])
      } : undefined
    }))
  }, { notMerge: true })
}

const resizeChart = () => {
  chart?.resize()
}

const handleRangeChange = (value: string | number | boolean) => {
  if (value === '1h' || value === '24h' || value === '7d' || value === '15d') {
    emit('change-range', value)
  }
}

watch(
  () => [props.trend, props.loading, props.range],
  () => {
    if (!props.loading) {
      renderChart()
    }
  },
  { deep: true }
)

onMounted(() => {
  window.addEventListener('resize', resizeChart)
  renderChart()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeChart)
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.host-trend-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.trend-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.trend-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.trend-summary-title {
  font-size: 15px;
  font-weight: 600;
  color: #0f172a;
}

.trend-summary-meta {
  font-size: 12px;
  color: #64748b;
}

.trend-body {
  min-height: 576px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  padding: 12px;
}

.trend-chart {
  width: 100%;
  height: 576px;
}

@media (max-width: 768px) {
  .trend-body {
    min-height: 520px;
    padding: 8px;
  }

  .trend-chart {
    height: 520px;
  }
}
</style>
