<template>
  <div class="metric-overview">
    <div class="metric-overview-grid">
      <div
        v-for="card in metricCards"
        :key="card.key"
        class="metric-card"
      >
        <div class="metric-card-header">
          <div class="metric-card-label">
            <span class="metric-card-icon" :class="`metric-card-icon-${card.tone}`">
              <el-icon><component :is="card.icon" /></el-icon>
            </span>
            <span>{{ card.label }}</span>
          </div>
          <span class="metric-card-badge" :class="`metric-card-badge-${card.tone}`">{{ card.badge }}</span>
        </div>
        <div class="metric-card-value">{{ card.value }}</div>
        <div class="metric-card-meta">{{ card.meta }}</div>
        <div class="metric-card-progress">
          <el-progress
            :percentage="card.progress"
            :stroke-width="8"
            :show-text="false"
            :color="card.progressColor"
          />
        </div>
      </div>
    </div>

    <div v-if="showLegend" class="metric-overview-legend">
      <span class="legend-item"><span class="legend-dot legend-low"></span>正常 &lt;70%</span>
      <span class="legend-item"><span class="legend-dot legend-high"></span>繁忙 ≥70%</span>
      <span class="legend-item"><span class="legend-dot legend-critical"></span>严重 ≥90%</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Coin, Cpu, DataLine, Download, Files, Upload } from '@element-plus/icons-vue'

interface HostMetricSummary {
  cpuCores?: number
  cpuUsage?: number
  memoryTotal?: number
  memoryUsed?: number
  memoryUsage?: number
  diskTotal?: number
  diskUsed?: number
  diskUsage?: number
}

interface HostMetricTrendPoint {
  load1?: number | null
  networkRecvRate?: number | null
  networkSendRate?: number | null
  diskReadRate?: number | null
  diskWriteRate?: number | null
}

interface HostMetricTrendData {
  points?: HostMetricTrendPoint[]
}

type MetricTone = 'cpu' | 'memory' | 'disk' | 'load' | 'network' | 'io'

interface MetricCard {
  key: string
  label: string
  icon: any
  tone: MetricTone
  badge: string
  value: string
  meta: string
  progress: number
  progressColor: string
}

const props = withDefaults(defineProps<{
  summary?: HostMetricSummary | null
  trend?: HostMetricTrendData | null
  showLegend?: boolean
}>(), {
  summary: null,
  trend: null,
  showLegend: false
})

const summaryData = computed<HostMetricSummary>(() => props.summary || {})

const trendPoints = computed<HostMetricTrendPoint[]>(() => props.trend?.points || [])

const latestPoint = computed<HostMetricTrendPoint | null>(() => {
  for (let index = trendPoints.value.length - 1; index >= 0; index -= 1) {
    const point = trendPoints.value[index]
    if (
      point.load1 !== null && point.load1 !== undefined ||
      point.networkRecvRate !== null && point.networkRecvRate !== undefined ||
      point.networkSendRate !== null && point.networkSendRate !== undefined ||
      point.diskReadRate !== null && point.diskReadRate !== undefined ||
      point.diskWriteRate !== null && point.diskWriteRate !== undefined
    ) {
      return point
    }
  }
  return null
})

const getMetricPeak = (field: keyof HostMetricTrendPoint) => {
  let max = 0
  for (const point of trendPoints.value) {
    const value = point[field]
    if (typeof value === 'number' && Number.isFinite(value) && value > max) {
      max = value
    }
  }
  return max
}

const normalizePercent = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(Number(value))) {
    return 0
  }
  return Math.min(Math.max(Number(value), 0), 100)
}

const scaleRelativePercent = (value?: number | null, peak?: number | null) => {
  if (value === undefined || value === null) {
    return 0
  }
  const baseline = peak && peak > 0 ? peak : value
  if (!baseline || baseline <= 0) {
    return 0
  }
  return normalizePercent(value / baseline * 100)
}

const getUsageColor = (usage?: number | null) => {
  const current = Number(usage || 0)
  if (current >= 90) return '#f56c6c'
  if (current >= 70) return '#e6a23c'
  return '#67c23a'
}

const formatPercent = (value?: number | null) => {
  if (value === undefined || value === null) return '-'
  return `${Number(value).toFixed(1)}%`
}

const formatBytes = (value?: number | null) => {
  if (value === undefined || value === null || value <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = Number(value)
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatRate = (value?: number | null) => {
  if (value === undefined || value === null) return '-'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let current = Number(value)
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatLoad = (value?: number | null) => {
  if (value === undefined || value === null) return '-'
  return Number(value).toFixed(2)
}

const latestMetrics = computed(() => ({
  load1: latestPoint.value?.load1 ?? null,
  networkRecvRate: latestPoint.value?.networkRecvRate ?? null,
  networkSendRate: latestPoint.value?.networkSendRate ?? null,
  diskReadRate: latestPoint.value?.diskReadRate ?? null,
  diskWriteRate: latestPoint.value?.diskWriteRate ?? null
}))

const loadPeak = computed(() => {
  const coreBaseline = Number(summaryData.value.cpuCores || 0)
  const trendPeak = getMetricPeak('load1')
  return Math.max(coreBaseline, trendPeak, 1)
})

const metricCards = computed<MetricCard[]>(() => {
  const summary = summaryData.value
  const metrics = latestMetrics.value
  const cards: MetricCard[] = [
    {
      key: 'cpu',
      label: 'CPU',
      icon: Cpu,
      tone: 'cpu',
      badge: formatPercent(summary.cpuUsage),
      value: summary.cpuCores ? `${summary.cpuCores}核` : '-',
      meta: summary.cpuUsage !== undefined && summary.cpuUsage !== null ? '当前 CPU 使用率' : '暂无 CPU 使用率',
      progress: normalizePercent(summary.cpuUsage),
      progressColor: getUsageColor(summary.cpuUsage)
    },
    {
      key: 'memory',
      label: '内存',
      icon: Coin,
      tone: 'memory',
      badge: formatPercent(summary.memoryUsage),
      value: formatBytes(summary.memoryTotal),
      meta: summary.memoryTotal ? `${formatBytes(summary.memoryUsed)} / ${formatBytes(summary.memoryTotal)}` : '暂无内存数据',
      progress: normalizePercent(summary.memoryUsage),
      progressColor: getUsageColor(summary.memoryUsage)
    },
    {
      key: 'disk',
      label: '磁盘',
      icon: Files,
      tone: 'disk',
      badge: formatPercent(summary.diskUsage),
      value: formatBytes(summary.diskTotal),
      meta: summary.diskTotal ? `${formatBytes(summary.diskUsed)} / ${formatBytes(summary.diskTotal)}` : '暂无磁盘数据',
      progress: normalizePercent(summary.diskUsage),
      progressColor: getUsageColor(summary.diskUsage)
    },
    {
      key: 'load',
      label: 'Load(1m)',
      icon: DataLine,
      tone: 'load',
      badge: formatLoad(metrics.load1),
      value: summary.cpuCores ? `${summary.cpuCores} 核基线` : '1 分钟平均负载',
      meta: metrics.load1 !== null ? `按 ${loadPeak.value.toFixed(2)} 基线缩放` : '暂无负载数据',
      progress: scaleRelativePercent(metrics.load1, loadPeak.value),
      progressColor: '#6366f1'
    },
    {
      key: 'network-recv',
      label: '网络接收',
      icon: Download,
      tone: 'network',
      badge: formatRate(metrics.networkRecvRate),
      value: `峰值 ${formatRate(getMetricPeak('networkRecvRate'))}`,
      meta: metrics.networkRecvRate !== null ? '按可见区间峰值归一化' : '暂无网络接收数据',
      progress: scaleRelativePercent(metrics.networkRecvRate, getMetricPeak('networkRecvRate')),
      progressColor: '#0ea5e9'
    },
    {
      key: 'network-send',
      label: '网络发送',
      icon: Upload,
      tone: 'network',
      badge: formatRate(metrics.networkSendRate),
      value: `峰值 ${formatRate(getMetricPeak('networkSendRate'))}`,
      meta: metrics.networkSendRate !== null ? '按可见区间峰值归一化' : '暂无网络发送数据',
      progress: scaleRelativePercent(metrics.networkSendRate, getMetricPeak('networkSendRate')),
      progressColor: '#0284c7'
    },
    {
      key: 'disk-read',
      label: '磁盘读取',
      icon: Download,
      tone: 'io',
      badge: formatRate(metrics.diskReadRate),
      value: `峰值 ${formatRate(getMetricPeak('diskReadRate'))}`,
      meta: metrics.diskReadRate !== null ? '按可见区间峰值归一化' : '暂无磁盘读取数据',
      progress: scaleRelativePercent(metrics.diskReadRate, getMetricPeak('diskReadRate')),
      progressColor: '#8b5cf6'
    },
    {
      key: 'disk-write',
      label: '磁盘写入',
      icon: Upload,
      tone: 'io',
      badge: formatRate(metrics.diskWriteRate),
      value: `峰值 ${formatRate(getMetricPeak('diskWriteRate'))}`,
      meta: metrics.diskWriteRate !== null ? '按可见区间峰值归一化' : '暂无磁盘写入数据',
      progress: scaleRelativePercent(metrics.diskWriteRate, getMetricPeak('diskWriteRate')),
      progressColor: '#a855f7'
    }
  ]

  return cards
})
</script>

<style scoped>
.metric-overview {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.metric-overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.metric-card {
  padding: 16px 18px;
  border-radius: 14px;
  border: 1px solid #e5e7eb;
  background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.metric-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.metric-card-label {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
}

.metric-card-icon {
  width: 24px;
  height: 24px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}

.metric-card-icon-cpu {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.1);
}

.metric-card-icon-memory {
  color: #16a34a;
  background: rgba(22, 163, 74, 0.12);
}

.metric-card-icon-disk {
  color: #d97706;
  background: rgba(217, 119, 6, 0.12);
}

.metric-card-icon-load {
  color: #6366f1;
  background: rgba(99, 102, 241, 0.12);
}

.metric-card-icon-network {
  color: #0284c7;
  background: rgba(2, 132, 199, 0.12);
}

.metric-card-icon-io {
  color: #9333ea;
  background: rgba(147, 51, 234, 0.12);
}

.metric-card-badge {
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.metric-card-badge-cpu {
  color: #2563eb;
}

.metric-card-badge-memory {
  color: #16a34a;
}

.metric-card-badge-disk {
  color: #d97706;
}

.metric-card-badge-load {
  color: #6366f1;
}

.metric-card-badge-network {
  color: #0284c7;
}

.metric-card-badge-io {
  color: #9333ea;
}

.metric-card-value {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.metric-card-meta {
  min-height: 18px;
  font-size: 12px;
  color: #64748b;
}

.metric-card-progress :deep(.el-progress-bar__outer) {
  background: #e8eef5;
}

.metric-overview-legend {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 18px;
  padding: 12px 14px;
  border-radius: 12px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #64748b;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

.legend-low {
  background: #67c23a;
}

.legend-high {
  background: #e6a23c;
}

.legend-critical {
  background: #f56c6c;
}

@media (max-width: 1440px) {
  .metric-overview-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .metric-overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .metric-overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
