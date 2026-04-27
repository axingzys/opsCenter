<template>
  <div class="mq-page">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><Connection /></el-icon>
        </div>
        <div>
          <h2 class="page-title">消息队列管理</h2>
          <p class="page-subtitle">统一纳管 RabbitMQ、Kafka、RocketMQ、ActiveMQ、Pulsar，优先提供只读诊断和审计</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button class="black-button" :disabled="!uiPermissions.instanceCreate" @click="openInstanceDialog()">
          <el-icon style="margin-right: 6px;"><Plus /></el-icon>
          新增实例
        </el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="main-tabs">
      <el-tab-pane label="实例管理" name="instances">
        <div class="search-bar">
          <div class="search-inputs">
            <el-input v-model="instanceQuery.keyword" placeholder="搜索实例名称、地址、业务系统、负责人..." clearable class="search-input" @keyup.enter="loadInstances" @clear="loadInstances">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-select v-model="instanceQuery.mqType" placeholder="MQ类型" clearable class="search-select" @change="loadInstances">
              <el-option v-for="item in supportedTypes" :key="item.type" :label="item.name" :value="item.type" />
            </el-select>
            <el-select v-model="instanceQuery.status" placeholder="状态" clearable class="search-select" @change="loadInstances">
              <el-option label="启用" value="enabled" />
              <el-option label="禁用" value="disabled" />
            </el-select>
            <el-select v-model="instanceQuery.healthStatus" placeholder="健康" clearable class="search-select" @change="loadInstances">
              <el-option label="健康" value="healthy" />
              <el-option label="警告" value="warning" />
              <el-option label="异常" value="critical" />
              <el-option label="未知" value="unknown" />
            </el-select>
          </div>
          <el-button class="reset-btn" @click="resetInstanceQuery">
            <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
            重置
          </el-button>
        </div>

        <div class="table-wrapper">
          <el-table :data="instances" v-loading="instanceLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="实例名称" min-width="180">
              <template #default="{ row }">
                <div class="name-cell">
                  <span>{{ row.name }}</span>
                  <el-tag v-if="row.environment" size="small" type="info">{{ environmentText(row.environment) }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="130" align="center">
              <template #default="{ row }">
                <el-tag :type="mqTypeTag(row.mqType)">{{ row.mqTypeText || row.mqType }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="连接地址" min-width="240" show-overflow-tooltip>
              <template #default="{ row }"><span class="mono">{{ row.endpoint }}</span></template>
            </el-table-column>
            <el-table-column label="管理地址" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">{{ row.managementUrl || '-' }}</template>
            </el-table-column>
            <el-table-column label="业务系统" min-width="140">
              <template #default="{ row }">{{ row.businessSystem || '-' }}</template>
            </el-table-column>
            <el-table-column label="负责人" width="120">
              <template #default="{ row }">{{ row.owner || '-' }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'enabled' ? 'success' : 'info'">{{ row.statusText || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="健康" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="healthTag(row.healthStatus)">{{ row.healthText || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="最近同步" width="170" align="center">
              <template #default="{ row }">{{ row.lastSyncAt || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="270" align="center" fixed="right">
              <template #default="{ row }">
                <el-tooltip content="连接测试" placement="top">
                  <el-button link type="primary" :loading="testingId === row.id" :disabled="!canUse(row, MQ_PERMISSION.MANAGE, 'connectionTest')" @click="handleTest(row)">
                    <el-icon><Connection /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="同步元数据" placement="top">
                  <el-button link type="success" :loading="syncingId === row.id" :disabled="!canUse(row, MQ_PERMISSION.MANAGE, 'metadataSync')" @click="handleSync(row)">
                    <el-icon><Refresh /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="查看概览" placement="top">
                  <el-button link type="info" :disabled="!canUse(row, MQ_PERMISSION.DIAGNOSE, 'diagnosisView')" @click="openOverview(row)">
                    <el-icon><DataBoard /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="row.status === 'enabled' ? '禁用' : '启用'" placement="top">
                  <el-button link :type="row.status === 'enabled' ? 'warning' : 'success'" :disabled="!canUse(row, MQ_PERMISSION.MANAGE, 'instanceStatus')" @click="toggleStatus(row)">
                    <el-icon><Switch /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="编辑" placement="top">
                  <el-button link type="primary" :disabled="!canUse(row, MQ_PERMISSION.MANAGE, 'instanceUpdate')" @click="openInstanceDialog(row)">
                    <el-icon><Edit /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="删除" placement="top">
                  <el-button link type="danger" :disabled="!canUse(row, MQ_PERMISSION.MANAGE, 'instanceDelete')" @click="handleDelete(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </el-tooltip>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination v-model:current-page="instanceQuery.page" v-model:page-size="instanceQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="instanceTotal" layout="total, sizes, prev, pager, next, jumper" @size-change="loadInstances" @current-change="loadInstances" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="资源管理" name="resources">
        <div class="workspace-toolbar">
          <el-select v-model="selectedInstanceId" placeholder="选择实例" filterable class="instance-select" @change="handleResourceInstanceChange">
            <el-option v-for="item in viewableInstances" :key="item.id" :label="`${item.name} (${item.mqTypeText || item.mqType})`" :value="item.id" />
          </el-select>
          <el-input v-model="resourceQuery.keyword" placeholder="搜索资源名称、命名空间..." clearable class="search-input" @keyup.enter="loadResources" @clear="loadResources">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="resourceQuery.resourceType" placeholder="资源类型" clearable class="search-select" @change="loadResources">
            <el-option label="Topic" value="topic" />
            <el-option label="Queue" value="queue" />
            <el-option label="Exchange" value="exchange" />
            <el-option label="VHost" value="vhost" />
            <el-option label="Namespace" value="namespace" />
            <el-option label="Tenant" value="tenant" />
          </el-select>
          <el-checkbox v-model="resourceHasBacklog" @change="loadResources">仅看堆积</el-checkbox>
          <el-button type="primary" :disabled="!canOperateSelectedInstance" @click="openOperationDialog()">
            <el-icon style="margin-right: 6px;"><Plus /></el-icon>
            资源操作
          </el-button>
        </div>

        <div class="metric-grid" v-if="overview">
          <div class="metric-item">
            <span class="metric-label">Broker</span>
            <strong>{{ overview.onlineBrokerCount }}/{{ overview.brokerCount }}</strong>
          </div>
          <div class="metric-item">
            <span class="metric-label">资源</span>
            <strong>{{ overview.resourceCount }}</strong>
          </div>
          <div class="metric-item">
            <span class="metric-label">Backlog</span>
            <strong>{{ overview.backlog }}</strong>
          </div>
          <div class="metric-item">
            <span class="metric-label">Lag</span>
            <strong>{{ overview.lag }}</strong>
          </div>
        </div>

        <div class="table-wrapper">
          <el-table :data="resources" v-loading="resourceLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="资源名称" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="name-cell"><span>{{ row.name }}</span><el-tag size="small">{{ row.resourceTypeText || row.resourceType }}</el-tag></div>
              </template>
            </el-table-column>
            <el-table-column label="命名空间" min-width="170" show-overflow-tooltip>
              <template #default="{ row }">{{ row.namespace || '-' }}</template>
            </el-table-column>
            <el-table-column label="分区" width="90" align="center" prop="partitionCount" />
            <el-table-column label="副本" width="90" align="center" prop="replicaCount" />
            <el-table-column label="消息数" width="120" align="right" prop="messageCount" />
            <el-table-column label="Backlog" width="120" align="right">
              <template #default="{ row }"><span :class="{ danger: row.backlog > 0 }">{{ row.backlog }}</span></template>
            </el-table-column>
            <el-table-column label="生产/s" width="120" align="right">
              <template #default="{ row }">{{ formatRate(row.producedRate) }}</template>
            </el-table-column>
            <el-table-column label="消费/s" width="120" align="right">
              <template #default="{ row }">{{ formatRate(row.consumedRate) }}</template>
            </el-table-column>
            <el-table-column label="消费者" width="100" align="center" prop="consumerCount" />
          </el-table>
          <div class="pagination-container">
            <el-pagination v-model:current-page="resourceQuery.page" v-model:page-size="resourceQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="resourceTotal" layout="total, sizes, prev, pager, next, jumper" @size-change="loadResources" @current-change="loadResources" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="消费诊断" name="diagnosis">
        <div class="workspace-toolbar">
          <el-select v-model="selectedInstanceId" placeholder="选择实例" filterable class="instance-select" @change="handleDiagnosisInstanceChange">
            <el-option v-for="item in diagnosableInstances" :key="item.id" :label="`${item.name} (${item.mqTypeText || item.mqType})`" :value="item.id" />
          </el-select>
          <el-input v-model="consumerQuery.keyword" placeholder="搜索消费组、订阅或资源..." clearable class="search-input" @keyup.enter="loadConsumerGroups" @clear="loadConsumerGroups">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-checkbox v-model="consumerHasLag" @change="loadConsumerGroups">仅看Lag</el-checkbox>
          <el-button :loading="metricCollecting" :disabled="!selectedInstanceId || !uiPermissions.diagnosisView" @click="handleCollectMetrics">
            <el-icon style="margin-right: 6px;"><DataBoard /></el-icon>
            采集指标
          </el-button>
          <el-button type="primary" :loading="inspectionLoading" :disabled="!selectedInstanceId || !uiPermissions.diagnosisView" @click="handleGenerateInspection">
            <el-icon style="margin-right: 6px;"><View /></el-icon>
            生成巡检
          </el-button>
        </div>

        <div v-if="overview" class="diagnosis-summary">
          <div class="summary-tags">
            <el-tag :type="healthTag(overview.healthStatus)">{{ overview.healthText || overview.healthStatus || '未知' }}</el-tag>
            <el-tag v-for="item in overview.anomalyTags || []" :key="item" type="warning">{{ item }}</el-tag>
            <span class="summary-time">最近指标：{{ overview.lastMetricAt || '-' }}</span>
          </div>
          <div v-if="overview.healthReasons?.length" class="health-reasons">
            <span v-for="item in overview.healthReasons" :key="item">{{ item }}</span>
          </div>
        </div>

        <div v-if="inspectionReport" class="inspection-panel">
          <div class="inspection-head">
            <div>
              <div class="panel-title">巡检结果</div>
              <div class="inspection-summary">{{ inspectionReport.summary }}</div>
            </div>
            <div class="score-box" :class="inspectionReport.riskLevel">
              <strong>{{ inspectionReport.score }}</strong>
              <span>{{ inspectionReport.generatedAt }}</span>
            </div>
          </div>
          <div class="inspection-sections">
            <div v-for="section in inspectionReport.sections || []" :key="section.key" class="inspection-section">
              <div class="section-title">
                <span>{{ section.title }}</span>
                <el-tag size="small" :type="severityTag(section.status)">{{ sectionStatusText(section.status) }}</el-tag>
              </div>
              <div class="section-summary">{{ section.summary }}</div>
              <div class="section-metrics">
                <span v-for="metric in section.metrics || []" :key="metric.name">
                  {{ metric.name }}：{{ formatDiffValue(metric.value) }}
                </span>
              </div>
            </div>
          </div>
          <el-table v-if="inspectionReport.findings?.length" :data="inspectionReport.findings" size="small" class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="级别" width="90">
              <template #default="{ row }"><el-tag :type="severityTag(row.severity)">{{ sectionStatusText(row.severity) }}</el-tag></template>
            </el-table-column>
            <el-table-column prop="title" label="问题" min-width="160" />
            <el-table-column prop="description" label="说明" min-width="260" show-overflow-tooltip />
            <el-table-column prop="category" label="分类" width="110" />
          </el-table>
        </div>

        <div class="split-layout">
          <div class="panel">
            <div class="panel-title">消费组 / 订阅</div>
            <el-table :data="consumerGroups" v-loading="consumerLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
              <el-table-column label="名称" min-width="180" prop="groupName" show-overflow-tooltip />
              <el-table-column label="资源" min-width="180" prop="resourceName" show-overflow-tooltip />
              <el-table-column label="消费者" width="90" align="center" prop="consumerCount" />
              <el-table-column label="Lag" width="100" align="right">
                <template #default="{ row }"><span :class="{ danger: row.lag > 0 }">{{ row.lag }}</span></template>
              </el-table-column>
              <el-table-column label="Backlog" width="110" align="right">
                <template #default="{ row }"><span :class="{ danger: row.backlog > 0 }">{{ row.backlog }}</span></template>
              </el-table-column>
            </el-table>
          </div>
          <div class="panel">
            <div class="panel-title">分区 / 队列维度</div>
            <el-table :data="partitions" v-loading="partitionLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
              <el-table-column label="资源" min-width="170" prop="resourceName" show-overflow-tooltip />
              <el-table-column label="分区" width="80" align="center" prop="partitionId" />
              <el-table-column label="Leader" min-width="150" prop="leader" show-overflow-tooltip />
              <el-table-column label="状态" width="90" align="center">
                <template #default="{ row }"><el-tag size="small" :type="row.status === 'online' ? 'success' : 'warning'">{{ row.status || '-' }}</el-tag></template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="消息查看" name="messages">
        <div class="message-sampler">
          <el-form :model="sampleForm" label-width="120px" class="sampler-form">
            <el-form-item label="实例">
              <el-select v-model="sampleInstanceId" placeholder="选择实例" filterable>
                <el-option v-for="item in sampleReadableInstances" :key="item.id" :label="`${item.name} (${item.mqTypeText || item.mqType})`" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="资源类型">
              <el-select v-model="sampleForm.resourceType" placeholder="资源类型">
                <el-option label="Topic" value="topic" />
                <el-option label="Queue" value="queue" />
              </el-select>
            </el-form-item>
            <el-form-item label="资源名称">
              <el-input v-model="sampleForm.resourceName" placeholder="Kafka topic 或其他资源名称" />
            </el-form-item>
            <el-form-item label="Partition">
              <el-input-number v-model="sampleForm.partitionId" :min="0" :max="100000" />
            </el-form-item>
            <el-form-item label="Offset">
              <el-input-number v-model="sampleForm.offset" :min="0" :max="9007199254740991" />
            </el-form-item>
            <el-form-item label="条数">
              <el-input-number v-model="sampleForm.limit" :min="1" :max="10" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="sampleLoading" :disabled="!uiPermissions.messageRead || !sampleInstanceId || !sampleForm.resourceName || sampleReadableInstances.length === 0" @click="handleSampleMessages">
                <el-icon style="margin-right: 6px;"><View /></el-icon>
                采样
              </el-button>
            </el-form-item>
          </el-form>

          <div class="sample-result">
            <div class="panel-title">采样结果</div>
            <el-empty v-if="messageSamples.length === 0" description="暂无消息样本" :image-size="72" />
            <div v-else class="sample-list">
              <div v-for="item in messageSamples" :key="`${item.partitionId}-${item.offset}`" class="sample-item">
                <div class="sample-meta">
                  <el-tag size="small">partition {{ item.partitionId }}</el-tag>
                  <span>offset {{ item.offset }}</span>
                  <span>{{ item.timestamp || '-' }}</span>
                  <span v-if="item.truncated" class="danger">已截断</span>
                </div>
                <pre>{{ item.payload }}</pre>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="任务中心" name="jobs">
        <div class="workspace-toolbar">
          <el-input v-model="jobQuery.keyword" placeholder="搜索实例、阶段、操作人、链路ID..." clearable class="search-input" @keyup.enter="loadJobs" @clear="loadJobs">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="jobQuery.instanceId" placeholder="实例" clearable filterable class="search-select" @change="loadJobs">
            <el-option v-for="item in diagnosableInstances" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select v-model="jobQuery.jobType" placeholder="任务类型" clearable class="search-select" @change="loadJobs">
            <el-option label="元数据同步" value="sync" />
            <el-option label="指标采集" value="metric_collect" />
            <el-option label="巡检" value="inspection" />
            <el-option label="操作后刷新" value="operation_refresh" />
            <el-option label="导出" value="export" />
          </el-select>
          <el-select v-model="jobQuery.status" placeholder="状态" clearable class="search-select" @change="loadJobs">
            <el-option label="待执行" value="pending" />
            <el-option label="执行中" value="running" />
            <el-option label="成功" value="success" />
            <el-option label="部分成功" value="partial_success" />
            <el-option label="失败" value="failed" />
            <el-option label="超时" value="timeout" />
          </el-select>
          <el-button :loading="jobLoading" @click="loadJobs">
            <el-icon style="margin-right: 6px;"><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
        <div class="table-wrapper">
          <el-table :data="jobs" v-loading="jobLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="任务" min-width="150">
              <template #default="{ row }">
                <div class="name-cell">
                  <span>{{ row.jobTypeText || row.jobType }}</span>
                  <el-tag size="small" :type="mqTypeTag(row.mqType)">{{ row.mqTypeText || row.mqType }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="实例" min-width="160" prop="instanceName" show-overflow-tooltip />
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }"><el-tag :type="jobStatusTag(row.status)">{{ row.statusText || row.status }}</el-tag></template>
            </el-table-column>
            <el-table-column label="进度" width="160">
              <template #default="{ row }">
                <el-progress :percentage="row.progressPercent || 0" :status="row.status === 'failed' ? 'exception' : row.status === 'success' ? 'success' : undefined" />
              </template>
            </el-table-column>
            <el-table-column label="当前阶段" min-width="170" prop="currentStage" show-overflow-tooltip />
            <el-table-column label="操作人" width="120" prop="operatorName" />
            <el-table-column label="耗时(ms)" width="110" align="right" prop="durationMs" />
            <el-table-column label="消息" min-width="220" prop="message" show-overflow-tooltip />
            <el-table-column label="链路ID" min-width="190" prop="correlationId" show-overflow-tooltip />
            <el-table-column label="时间" width="170" prop="createdAt" />
          </el-table>
          <div class="pagination-container">
            <el-pagination v-model:current-page="jobQuery.page" v-model:page-size="jobQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="jobTotal" layout="total, sizes, prev, pager, next, jumper" @size-change="loadJobs" @current-change="loadJobs" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="操作审计" name="audits">
        <div class="workspace-toolbar">
          <el-input v-model="auditQuery.keyword" placeholder="搜索实例、资源、操作人、消息..." clearable class="search-input" @keyup.enter="loadAudits" @clear="loadAudits">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="auditQuery.instanceId" placeholder="实例" clearable filterable class="search-select" @change="loadAudits">
            <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select v-model="auditQuery.status" placeholder="状态" clearable class="search-select" @change="loadAudits">
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
            <el-option label="执行中" value="pending" />
          </el-select>
          <el-radio-group v-model="auditType" @change="loadAudits">
            <el-radio-button label="operation">操作审计</el-radio-button>
            <el-radio-button label="message">消息审计</el-radio-button>
          </el-radio-group>
        </div>
        <div class="table-wrapper">
          <el-table :data="audits" v-loading="auditLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="动作" width="150" prop="actionText" />
            <el-table-column label="实例" min-width="160" prop="instanceName" show-overflow-tooltip />
            <el-table-column label="资源" min-width="200" prop="resourceName" show-overflow-tooltip />
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }"><el-tag :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'warning'">{{ row.statusText || row.status }}</el-tag></template>
            </el-table-column>
            <el-table-column label="操作人" width="120" prop="operatorName" />
            <el-table-column label="耗时(ms)" width="110" align="right" prop="durationMs" />
            <el-table-column label="消息" min-width="240" prop="message" show-overflow-tooltip />
            <el-table-column label="时间" width="170" prop="createdAt" />
          </el-table>
          <div class="pagination-container">
            <el-pagination v-model:current-page="auditQuery.page" v-model:page-size="auditQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="auditTotal" layout="total, sizes, prev, pager, next, jumper" @size-change="loadAudits" @current-change="loadAudits" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane v-if="uiPermissions.permissionManage" label="实例权限" name="permissions">
        <div class="workspace-toolbar">
          <el-input v-model="permissionQuery.keyword" placeholder="搜索角色或实例" clearable class="search-input" @keyup.enter="loadPermissions" @clear="loadPermissions">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-button type="primary" @click="openPermissionDialog()">
            <el-icon style="margin-right: 6px;"><Plus /></el-icon>
            添加权限
          </el-button>
        </div>
        <div class="table-wrapper">
          <el-table :data="permissionRows" v-loading="permissionLoading" stripe class="modern-table" :header-cell-style="tableHeaderStyle">
            <el-table-column label="角色" min-width="180">
              <template #default="{ row }">{{ row.roleName || '-' }} <el-tag v-if="row.roleCode" size="small">{{ row.roleCode }}</el-tag></template>
            </el-table-column>
            <el-table-column label="实例" min-width="180" prop="instanceName" />
            <el-table-column label="权限" min-width="420">
              <template #default="{ row }">
                <div class="permission-tag-list">
                  <el-tag v-for="item in permissionOptions.filter(option => hasPermissionMask(row.permissions, option.value))" :key="item.value" size="small">{{ item.label }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120" align="center">
              <template #default="{ row }">
                <el-button link type="primary" @click="openPermissionDialog(row)">编辑</el-button>
                <el-button link type="danger" @click="handleDeletePermission(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pagination-container">
            <el-pagination v-model:current-page="permissionQuery.page" v-model:page-size="permissionQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="permissionTotal" layout="total, sizes, prev, pager, next, jumper" @size-change="loadPermissions" @current-change="loadPermissions" />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="instanceDialogVisible" :title="instanceForm.id ? '编辑MQ实例' : '新增MQ实例'" width="720px">
      <el-form ref="instanceFormRef" :model="instanceForm" :rules="instanceRules" label-width="120px">
        <el-form-item label="实例名称" prop="name"><el-input v-model="instanceForm.name" /></el-form-item>
        <el-form-item label="MQ类型" prop="mqType">
          <el-select v-model="instanceForm.mqType" placeholder="选择类型" @change="applyTypeDefaults">
            <el-option v-for="item in supportedTypes" :key="item.type" :label="item.name" :value="item.type" />
          </el-select>
        </el-form-item>
        <el-form-item label="连接地址" prop="endpoint"><el-input v-model="instanceForm.endpoint" placeholder="host:port，Kafka可用逗号分隔多个bootstrap" /></el-form-item>
        <el-form-item label="管理地址"><el-input v-model="instanceForm.managementUrl" placeholder="RabbitMQ/Pulsar/ActiveMQ 管理API地址，可选" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="instanceForm.port" :min="0" :max="65535" /></el-form-item>
        <el-form-item label="凭据"><el-select v-model="instanceForm.credentialId" clearable filterable placeholder="可选">
          <el-option v-for="item in credentials" :key="item.id" :label="`${item.name} (${item.username || '无用户名'})`" :value="item.id" />
        </el-select></el-form-item>
        <el-form-item label="TLS"><el-switch v-model="instanceForm.tlsEnabled" /></el-form-item>
        <el-form-item label="环境"><el-select v-model="instanceForm.environment" clearable>
          <el-option label="生产" value="prod" /><el-option label="预发" value="staging" /><el-option label="测试" value="test" /><el-option label="开发" value="dev" />
        </el-select></el-form-item>
        <el-form-item label="业务系统"><el-input v-model="instanceForm.businessSystem" /></el-form-item>
        <el-form-item label="负责人"><el-input v-model="instanceForm.owner" /></el-form-item>
        <el-form-item label="连接参数"><el-input v-model="instanceForm.connectionParams" type="textarea" :rows="3" placeholder='JSON，如 {"sasl":"plain"}' /></el-form-item>
        <el-form-item label="备注"><el-input v-model="instanceForm.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="instanceDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="instanceSubmitting" @click="submitInstance">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="operationDialogVisible" title="MQ资源操作" width="760px">
      <el-form :model="operationForm" label-width="120px">
        <el-form-item label="实例">
          <el-select v-model="operationForm.instanceId" filterable placeholder="选择实例" @change="handleOperationInstanceChange">
            <el-option v-for="item in resourceManageInstances" :key="item.id" :label="`${item.name} (${item.mqTypeText || item.mqType})`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="操作">
          <el-select v-model="operationForm.action" placeholder="选择操作" @change="applyOperationTemplate">
            <el-option v-for="item in operationOptions" :key="item.value" :label="item.displayLabel" :value="item.value" :disabled="item.disabled" />
          </el-select>
        </el-form-item>
        <el-form-item label="资源名称">
          <el-input v-model="operationForm.resourceName" placeholder="Topic、Queue、Exchange 或 Namespace" />
        </el-form-item>
        <el-form-item label="命名空间">
          <el-input v-model="operationForm.namespace" placeholder="RabbitMQ vhost 或 Pulsar tenant/namespace，可选" />
        </el-form-item>
        <el-form-item label="操作原因">
          <el-input v-model="operationForm.reason" type="textarea" :rows="2" placeholder="记录到操作审计，可选" />
        </el-form-item>
        <el-form-item label="参数JSON">
          <el-input v-model="operationForm.paramsText" type="textarea" :rows="8" class="mono-textarea" />
        </el-form-item>
      </el-form>

      <div v-if="operationValidation" class="operation-validation">
        <div class="validation-header">
          <el-tag :type="operationValidation.supported ? 'success' : 'danger'">{{ operationValidation.supported ? '支持执行' : '暂不支持' }}</el-tag>
          <el-tag :type="riskTag(operationValidation.riskLevel)">{{ riskText(operationValidation.riskLevel) }}</el-tag>
          <el-tag v-if="operationValidation.metadataStale" type="warning">元数据过期</el-tag>
          <span>{{ operationValidation.message }}</span>
        </div>
        <div v-if="operationValidation.impacts?.length" class="validation-list">
          <span v-for="item in operationValidation.impacts" :key="item">{{ item }}</span>
        </div>
        <el-table v-if="operationValidation.diff?.length" :data="operationValidation.diff" size="small" class="operation-diff-table">
          <el-table-column prop="key" label="变更项" min-width="160" />
          <el-table-column label="当前值" min-width="180">
            <template #default="{ row }">{{ formatDiffValue(row.before) }}</template>
          </el-table-column>
          <el-table-column label="目标值" min-width="180">
            <template #default="{ row }">{{ formatDiffValue(row.after) }}</template>
          </el-table-column>
          <el-table-column label="风险" width="100">
            <template #default="{ row }"><el-tag :type="riskTag(row.risk)">{{ riskText(row.risk) }}</el-tag></template>
          </el-table-column>
        </el-table>
        <el-alert v-for="item in operationValidation.warnings || []" :key="item" :title="item" type="warning" :closable="false" show-icon />
      </div>

      <template #footer>
        <el-button @click="operationDialogVisible = false">取消</el-button>
        <el-button :loading="operationValidating" @click="validateOperation">校验</el-button>
        <el-button type="primary" :loading="operationSubmitting" :disabled="!operationValidation?.supported" @click="executeOperation">确认执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="permissionDialogVisible" title="MQ实例权限" width="560px">
      <el-form :model="permissionForm" label-width="100px">
        <el-form-item label="角色">
          <el-select v-model="permissionForm.roleId" filterable placeholder="选择角色">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="实例">
          <el-select v-model="permissionForm.instanceId" filterable placeholder="选择实例">
            <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="权限">
          <el-checkbox-group v-model="permissionForm.permissionValues">
            <el-checkbox v-for="item in permissionOptions" :key="item.value" :label="item.value">{{ item.label }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="permissionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="permissionSubmitting" @click="submitPermission">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Connection, DataBoard, Delete, Edit, Plus, Refresh, RefreshLeft, Search, Switch, View } from '@element-plus/icons-vue'
import { getCredentials } from '@/api/host'
import { getAllRoles } from '@/api/role'
import {
  MQ_PERMISSION,
  collectMQMetricSnapshot,
  createMQInstance,
  deleteMQInstance,
  deleteMQInstancePermission,
  disableMQInstance,
  enableMQInstance,
  executeMQResourceOperation,
  generateMQInspectionReport,
  getMQOverview,
  getMQSupportedTypes,
  getMQUIPermissions,
  listMQConsumerGroups,
  listMQInstances,
  listMQInstancePermissions,
  listMQJobs,
  listMQMessageAudits,
  listMQOperationAudits,
  listMQPartitions,
  listMQResources,
  sampleMQMessages,
  syncMQMetadata,
  testMQInstance,
  updateMQInstance,
  upsertMQInstancePermission,
  validateMQResourceOperation,
  type MQInstancePayload,
  type MQResourceOperationPayload,
  type MQSupportedType
} from '@/api/messagequeue'

const tableHeaderStyle = { background: '#fafbfc', color: '#606266', fontWeight: '600' }
const activeTab = ref('instances')
const supportedTypes = ref<MQSupportedType[]>([])
const credentials = ref<any[]>([])
const roles = ref<any[]>([])
const uiPermissions = reactive<Record<string, boolean>>({
  instanceCreate: false,
  instanceUpdate: false,
  instanceDelete: false,
  instanceStatus: false,
  connectionTest: false,
  metadataSync: false,
  diagnosisView: false,
  messageRead: false,
  messageExport: false,
  messageWrite: false,
  resourceManage: false,
  highRisk: false,
  auditView: false,
  auditExport: false,
  permissionManage: false
})

const instances = ref<any[]>([])
const instanceTotal = ref(0)
const instanceLoading = ref(false)
const testingId = ref(0)
const syncingId = ref(0)
const instanceQuery = reactive({ page: 1, pageSize: 10, keyword: '', mqType: '', status: '', healthStatus: '', environment: '' })

const selectedInstanceId = ref<number | null>(null)
const overview = ref<any>(null)
const resources = ref<any[]>([])
const resourceTotal = ref(0)
const resourceLoading = ref(false)
const resourceHasBacklog = ref(false)
const resourceQuery = reactive({ page: 1, pageSize: 10, keyword: '', resourceType: '', namespace: '' })

const consumerGroups = ref<any[]>([])
const partitions = ref<any[]>([])
const consumerLoading = ref(false)
const partitionLoading = ref(false)
const metricCollecting = ref(false)
const inspectionLoading = ref(false)
const inspectionReport = ref<any>(null)
const consumerHasLag = ref(false)
const consumerQuery = reactive({ page: 1, pageSize: 20, keyword: '', resourceName: '', namespace: '' })

const sampleInstanceId = ref<number | null>(null)
const sampleLoading = ref(false)
const messageSamples = ref<any[]>([])
const sampleForm = reactive<any>({ resourceType: 'topic', resourceName: '', partitionId: 0, offset: null, limit: 5, maxBytes: 65536 })

const auditType = ref('operation')
const audits = ref<any[]>([])
const auditTotal = ref(0)
const auditLoading = ref(false)
const auditQuery = reactive<any>({ page: 1, pageSize: 10, keyword: '', instanceId: undefined, status: '', action: '' })

const jobs = ref<any[]>([])
const jobTotal = ref(0)
const jobLoading = ref(false)
const jobQuery = reactive<any>({ page: 1, pageSize: 10, keyword: '', instanceId: undefined, mqType: '', jobType: '', status: '' })

const permissionRows = ref<any[]>([])
const permissionTotal = ref(0)
const permissionLoading = ref(false)
const permissionQuery = reactive({ page: 1, pageSize: 10, keyword: '' })

const instanceDialogVisible = ref(false)
const instanceSubmitting = ref(false)
const instanceFormRef = ref<FormInstance>()
const instanceForm = reactive<any>({
  id: 0,
  name: '',
  mqType: 'rabbitmq',
  endpoint: '',
  managementUrl: '',
  port: 5672,
  credentialId: undefined,
  tlsEnabled: false,
  connectionParams: '',
  status: 'enabled',
  environment: '',
  businessSystem: '',
  owner: '',
  tags: '',
  remark: ''
})
const instanceRules: FormRules = {
  name: [{ required: true, message: '请输入实例名称', trigger: 'blur' }],
  mqType: [{ required: true, message: '请选择MQ类型', trigger: 'change' }],
  endpoint: [{ required: true, message: '请输入连接地址', trigger: 'blur' }]
}

const permissionDialogVisible = ref(false)
const permissionSubmitting = ref(false)
const permissionForm = reactive<any>({ id: 0, roleId: undefined, instanceId: undefined, permissionValues: [MQ_PERMISSION.VIEW, MQ_PERMISSION.DIAGNOSE] })

const operationDialogVisible = ref(false)
const operationValidating = ref(false)
const operationSubmitting = ref(false)
const operationValidation = ref<any>(null)
const operationForm = reactive<any>({ instanceId: undefined, action: '', resourceType: '', namespace: '', resourceName: '', reason: '', confirmText: '', idempotencyKey: '', paramsText: '{}' })

const permissionOptions = [
  { label: '查看', value: MQ_PERMISSION.VIEW },
  { label: '诊断', value: MQ_PERMISSION.DIAGNOSE },
  { label: '消息查看', value: MQ_PERMISSION.MESSAGE_READ },
  { label: '消息导出', value: MQ_PERMISSION.MESSAGE_EXPORT },
  { label: '消息写入', value: MQ_PERMISSION.MESSAGE_WRITE },
  { label: '资源管理', value: MQ_PERMISSION.RESOURCE_MANAGE },
  { label: '高危操作', value: MQ_PERMISSION.HIGH_RISK },
  { label: '审计', value: MQ_PERMISSION.AUDIT },
  { label: '管理', value: MQ_PERMISSION.MANAGE }
]

type OperationTemplate = { label: string; value: string; resourceType: string; params: Record<string, any>; highRisk?: boolean }
type OperationOption = OperationTemplate & { displayLabel: string; disabled: boolean }

const operationTemplateMap: Record<string, OperationTemplate[]> = {
  rabbitmq: [
    { label: '创建/更新 Queue', value: 'rabbitmq_queue_upsert', resourceType: 'queue', params: { durable: true, autoDelete: false, arguments: {} } },
    { label: '创建/更新 Exchange', value: 'rabbitmq_exchange_upsert', resourceType: 'exchange', params: { type: 'direct', durable: true, autoDelete: false, internal: false, arguments: {} } },
    { label: '创建 Binding', value: 'rabbitmq_binding_upsert', resourceType: 'binding', params: { source: 'amq.direct', destinationType: 'queue', routingKey: '', arguments: {} } },
    { label: '清空 Queue', value: 'rabbitmq_queue_purge', resourceType: 'queue', highRisk: true, params: {} },
    { label: '删除 Queue', value: 'rabbitmq_queue_delete', resourceType: 'queue', highRisk: true, params: { ifUnused: false, ifEmpty: false } },
    { label: '删除 Exchange', value: 'rabbitmq_exchange_delete', resourceType: 'exchange', highRisk: true, params: { ifUnused: false, force: false } }
  ],
  kafka: [
    { label: '创建 Topic', value: 'kafka_topic_create', resourceType: 'topic', params: { partitions: 1, replicationFactor: 1, configs: {} } },
    { label: 'Topic 分区扩容', value: 'kafka_partitions_expand', resourceType: 'topic', params: { partitions: 3 } },
    { label: '更新 Topic 配置', value: 'kafka_topic_config_update', resourceType: 'topic', params: { configs: { 'retention.ms': '604800000' } } },
    { label: '删除 Topic', value: 'kafka_topic_delete', resourceType: 'topic', highRisk: true, params: {} }
  ],
  pulsar: [
    { label: '更新 Namespace Retention', value: 'pulsar_namespace_retention_update', resourceType: 'namespace', params: { retentionTimeInMinutes: 1440, retentionSizeInMB: 1024 } },
    { label: '更新 Namespace TTL', value: 'pulsar_namespace_ttl_update', resourceType: 'namespace', params: { messageTTLInSeconds: 86400 } },
    { label: '删除 Topic', value: 'pulsar_topic_delete', resourceType: 'topic', highRisk: true, params: { force: false } },
    { label: '跳过 Subscription 积压', value: 'pulsar_subscription_skip', resourceType: 'subscription', highRisk: true, params: { topic: 'persistent://public/default/topic', subscription: '' } },
    { label: '重置 Subscription Cursor', value: 'pulsar_subscription_reset', resourceType: 'subscription', highRisk: true, params: { topic: 'persistent://public/default/topic', subscription: '', timestampMs: Date.now() } }
  ]
}

const supportedTypeMap = computed(() => new Map(supportedTypes.value.map(item => [item.type, item])))
const viewableInstances = computed(() => instances.value.filter(item => hasObjectPermission(item, MQ_PERMISSION.VIEW)))
const diagnosableInstances = computed(() => instances.value.filter(item => hasObjectPermission(item, MQ_PERMISSION.DIAGNOSE)))
const messageReadableInstances = computed(() => instances.value.filter(item => hasObjectPermission(item, MQ_PERMISSION.MESSAGE_READ)))
const sampleReadableInstances = computed(() => messageReadableInstances.value.filter(item => supportedTypeMap.value.get(item.mqType)?.messageSampleEnabled))
const resourceManageInstances = computed(() => instances.value.filter(item => hasObjectPermission(item, MQ_PERMISSION.RESOURCE_MANAGE)))
const selectedResourceInstance = computed(() => instances.value.find(item => item.id === selectedInstanceId.value))
const selectedOperationInstance = computed(() => instances.value.find(item => item.id === operationForm.instanceId))
const selectedOperationHasHighRisk = computed(() => !!uiPermissions.highRisk && hasObjectPermission(selectedOperationInstance.value, MQ_PERMISSION.HIGH_RISK))
const operationOptions = computed<OperationOption[]>(() => {
  const options = operationTemplateMap[selectedOperationInstance.value?.mqType || ''] || []
  return options.map(item => ({
    ...item,
    displayLabel: item.highRisk ? `${item.label}（高危）` : item.label,
    disabled: !!item.highRisk && !selectedOperationHasHighRisk.value
  }))
})
const canOperateSelectedInstance = computed(() => !!uiPermissions.resourceManage && resourceManageInstances.value.length > 0)

const loadSupportedTypes = async () => {
  supportedTypes.value = await getMQSupportedTypes()
}

const loadUIPermissions = async () => {
  const data = await getMQUIPermissions()
  Object.assign(uiPermissions, data || {})
}

const loadCredentials = async () => {
  credentials.value = await getCredentials()
}

const loadRoles = async () => {
  roles.value = await getAllRoles()
}

const loadInstances = async () => {
  instanceLoading.value = true
  try {
    const res = await listMQInstances({ ...instanceQuery })
    instances.value = res.list || []
    instanceTotal.value = res.total || 0
    if (!selectedInstanceId.value && instances.value.length > 0) {
      selectedInstanceId.value = instances.value[0].id
    }
    if (!sampleInstanceId.value || !sampleReadableInstances.value.some(item => item.id === sampleInstanceId.value)) {
      sampleInstanceId.value = sampleReadableInstances.value[0]?.id || null
    }
  } finally {
    instanceLoading.value = false
  }
}

const resetInstanceQuery = () => {
  Object.assign(instanceQuery, { page: 1, keyword: '', mqType: '', status: '', healthStatus: '', environment: '' })
  loadInstances()
}

const loadOverview = async () => {
  if (!selectedInstanceId.value) return
  overview.value = await getMQOverview(selectedInstanceId.value)
}

const loadResources = async () => {
  if (!selectedInstanceId.value) return
  resourceLoading.value = true
  try {
    const res = await listMQResources(selectedInstanceId.value, { ...resourceQuery, hasBacklog: resourceHasBacklog.value ? 'true' : '' })
    resources.value = res.list || []
    resourceTotal.value = res.total || 0
  } finally {
    resourceLoading.value = false
  }
}

const loadConsumerGroups = async () => {
  if (!selectedInstanceId.value) return
  consumerLoading.value = true
  try {
    const res = await listMQConsumerGroups(selectedInstanceId.value, { ...consumerQuery, hasLag: consumerHasLag.value ? 'true' : '' })
    consumerGroups.value = res.list || []
  } finally {
    consumerLoading.value = false
  }
}

const loadPartitions = async () => {
  if (!selectedInstanceId.value) return
  partitionLoading.value = true
  try {
    const res = await listMQPartitions(selectedInstanceId.value, { page: 1, pageSize: 100 })
    partitions.value = res.list || []
  } finally {
    partitionLoading.value = false
  }
}

const loadDiagnosis = async () => {
  await Promise.all([loadOverview(), loadConsumerGroups(), loadPartitions()])
}

const handleDiagnosisInstanceChange = async () => {
  inspectionReport.value = null
  consumerQuery.page = 1
  await loadDiagnosis()
}

const handleResourceInstanceChange = async () => {
  resourceQuery.page = 1
  await Promise.all([loadOverview(), loadResources()])
}

const handleCollectMetrics = async () => {
  if (!selectedInstanceId.value) return
  metricCollecting.value = true
  try {
    const res = await collectMQMetricSnapshot(selectedInstanceId.value)
    ElMessage.success(res.message || '指标采集完成')
    await Promise.all([loadOverview(), loadInstances(), loadAudits(), loadJobs()])
  } finally {
    metricCollecting.value = false
  }
}

const handleGenerateInspection = async () => {
  if (!selectedInstanceId.value) return
  inspectionLoading.value = true
  try {
    inspectionReport.value = await generateMQInspectionReport(selectedInstanceId.value)
    ElMessage.success('巡检完成')
    await Promise.all([loadAudits(), loadJobs()])
  } finally {
    inspectionLoading.value = false
  }
}

const loadAudits = async () => {
  auditLoading.value = true
  try {
    const res = auditType.value === 'message'
      ? await listMQMessageAudits({ ...auditQuery })
      : await listMQOperationAudits({ ...auditQuery })
    audits.value = res.list || []
    auditTotal.value = res.total || 0
  } finally {
    auditLoading.value = false
  }
}

const loadJobs = async () => {
  if (!uiPermissions.diagnosisView) return
  jobLoading.value = true
  try {
    const res = await listMQJobs({ ...jobQuery })
    jobs.value = res.list || []
    jobTotal.value = res.total || 0
  } finally {
    jobLoading.value = false
  }
}

const loadPermissions = async () => {
  if (!uiPermissions.permissionManage) return
  permissionLoading.value = true
  try {
    const res = await listMQInstancePermissions({ ...permissionQuery })
    permissionRows.value = res.list || []
    permissionTotal.value = res.total || 0
  } finally {
    permissionLoading.value = false
  }
}

const openInstanceDialog = (row?: any) => {
  if (row) {
    Object.assign(instanceForm, {
      id: row.id,
      name: row.name,
      mqType: row.mqType,
      endpoint: row.endpoint,
      managementUrl: row.managementUrl || '',
      port: row.port || supportedTypeMap.value.get(row.mqType)?.defaultPort || 0,
      credentialId: row.credentialId || undefined,
      tlsEnabled: !!row.tlsEnabled,
      connectionParams: row.connectionParams || '',
      status: row.status || 'enabled',
      environment: row.environment || '',
      businessSystem: row.businessSystem || '',
      owner: row.owner || '',
      tags: row.tags || '',
      remark: row.remark || ''
    })
  } else {
    Object.assign(instanceForm, {
      id: 0,
      name: '',
      mqType: 'rabbitmq',
      endpoint: '',
      managementUrl: '',
      port: 5672,
      credentialId: undefined,
      tlsEnabled: false,
      connectionParams: '',
      status: 'enabled',
      environment: '',
      businessSystem: '',
      owner: '',
      tags: '',
      remark: ''
    })
  }
  instanceDialogVisible.value = true
}

const applyTypeDefaults = () => {
  const item = supportedTypeMap.value.get(instanceForm.mqType)
  if (item) {
    instanceForm.port = item.defaultPort
  }
}

const submitInstance = async () => {
  if (!instanceFormRef.value) return
  await instanceFormRef.value.validate()
  instanceSubmitting.value = true
  try {
    const payload: MQInstancePayload = { ...instanceForm, credentialId: instanceForm.credentialId || 0 }
    if (instanceForm.id) {
      await updateMQInstance(instanceForm.id, payload)
      ElMessage.success('更新成功')
    } else {
      await createMQInstance(payload)
      ElMessage.success('创建成功')
    }
    instanceDialogVisible.value = false
    await loadInstances()
  } finally {
    instanceSubmitting.value = false
  }
}

const handleTest = async (row: any) => {
  testingId.value = row.id
  try {
    const res = await testMQInstance(row.id)
    ElMessage.success(res.message || '连接测试成功')
    await loadInstances()
  } finally {
    testingId.value = 0
  }
}

const handleSync = async (row: any) => {
  syncingId.value = row.id
  try {
    const res = await syncMQMetadata(row.id)
    ElMessage.success(res.message || '同步成功')
    await Promise.all([loadInstances(), loadResources(), loadDiagnosis(), loadAudits(), loadJobs()])
  } finally {
    syncingId.value = 0
  }
}

const openOverview = async (row: any) => {
  selectedInstanceId.value = row.id
  activeTab.value = 'resources'
  await handleResourceInstanceChange()
}

const toggleStatus = async (row: any) => {
  if (row.status === 'enabled') {
    await disableMQInstance(row.id)
    ElMessage.success('禁用成功')
  } else {
    await enableMQInstance(row.id)
    ElMessage.success('启用成功')
  }
  await loadInstances()
}

const handleDelete = async (row: any) => {
  await ElMessageBox.confirm(`确认删除 MQ 实例「${row.name}」？`, '删除确认', { type: 'warning' })
  await deleteMQInstance(row.id)
  ElMessage.success('删除成功')
  await loadInstances()
}

const handleSampleMessages = async () => {
  if (!sampleInstanceId.value) return
  sampleLoading.value = true
  try {
    const payload = { ...sampleForm }
    if (payload.offset === null || payload.offset === undefined) delete payload.offset
    const res = await sampleMQMessages(sampleInstanceId.value, payload)
    messageSamples.value = res.samples || []
    ElMessage.success(res.message || '采样完成')
    await loadAudits()
  } finally {
    sampleLoading.value = false
  }
}

const openOperationDialog = () => {
  const current = selectedResourceInstance.value && hasObjectPermission(selectedResourceInstance.value, MQ_PERMISSION.RESOURCE_MANAGE)
    ? selectedResourceInstance.value
    : resourceManageInstances.value[0]
  Object.assign(operationForm, {
    instanceId: current?.id,
    action: '',
    resourceType: '',
    namespace: '',
    resourceName: '',
    reason: '',
    confirmText: '',
    idempotencyKey: '',
    paramsText: '{}'
  })
  operationValidation.value = null
  operationDialogVisible.value = true
  handleOperationInstanceChange()
}

const handleOperationInstanceChange = () => {
  const first = operationOptions.value.find(item => !item.disabled) || operationOptions.value[0]
  operationForm.action = first?.value || ''
  applyOperationTemplate()
}

const applyOperationTemplate = () => {
  const option = operationOptions.value.find(item => item.value === operationForm.action)
  operationValidation.value = null
  operationForm.confirmText = ''
  operationForm.idempotencyKey = ''
  if (!option || option.disabled) {
    if (option?.disabled) ElMessage.warning('当前实例缺少高危操作权限')
    operationForm.resourceType = ''
    operationForm.paramsText = '{}'
    return
  }
  operationForm.resourceType = option.resourceType
  operationForm.paramsText = JSON.stringify(option.params, null, 2)
}

const buildOperationPayload = (confirmed = false): MQResourceOperationPayload | null => {
  if (!operationForm.instanceId || !operationForm.action) {
    ElMessage.warning('请选择实例和操作')
    return null
  }
  let params: Record<string, any> = {}
  try {
    params = operationForm.paramsText ? JSON.parse(operationForm.paramsText) : {}
  } catch (error: any) {
    ElMessage.error(`参数JSON格式错误: ${error.message || error}`)
    return null
  }
  return {
    action: operationForm.action,
    resourceType: operationForm.resourceType,
    namespace: operationForm.namespace,
    resourceName: operationForm.resourceName,
    reason: operationForm.reason,
    confirmText: operationForm.confirmText,
    idempotencyKey: operationForm.idempotencyKey,
    confirmed,
    params
  }
}

const validateOperation = async () => {
  const payload = buildOperationPayload(false)
  if (!payload || !operationForm.instanceId) return
  operationValidating.value = true
  try {
    const res = await validateMQResourceOperation(operationForm.instanceId, payload)
    operationValidation.value = res
    if (res.normalizedParams) {
      operationForm.paramsText = JSON.stringify(res.normalizedParams, null, 2)
    }
    if (res.namespace) operationForm.namespace = res.namespace
    if (res.resourceName) operationForm.resourceName = res.resourceName
    ElMessage.success('校验完成')
  } finally {
    operationValidating.value = false
  }
}

const executeOperation = async () => {
  if (!operationValidation.value?.supported) {
    await validateOperation()
  }
  if (!operationValidation.value?.supported) return
  const highRisk = isHighRisk(operationValidation.value.riskLevel)
  if (highRisk && !String(operationForm.reason || '').trim()) {
    ElMessage.warning('高危操作必须填写操作原因')
    return
  }
  const confirmText = highRisk
    ? `确认执行高危操作「${operationValidation.value.actionText || operationForm.action}」？该操作会记录审计并可能造成消息或资源不可恢复。`
    : `确认执行「${operationValidation.value.actionText || operationForm.action}」？`
  await ElMessageBox.confirm(confirmText, highRisk ? '高危操作确认' : '操作确认', { type: highRisk ? 'error' : 'warning' })
  if (highRisk && operationValidation.value.resourceName) {
    const { value } = await ElMessageBox.prompt(`请输入资源名 ${operationValidation.value.resourceName} 以确认执行`, '资源名确认', {
      inputValue: '',
      inputPattern: new RegExp(`^${escapeRegExp(operationValidation.value.resourceName)}$`),
      inputErrorMessage: '输入的资源名不一致',
      type: 'error'
    })
    operationForm.confirmText = value
  }
  if (!operationForm.idempotencyKey) {
    operationForm.idempotencyKey = createIdempotencyKey()
  }
  const payload = buildOperationPayload(true)
  if (!payload || !operationForm.instanceId) return
  operationSubmitting.value = true
  try {
    const res = await executeMQResourceOperation(operationForm.instanceId, payload)
    ElMessage.success(res.message || '执行成功')
    operationDialogVisible.value = false
    await Promise.all([loadResources(), loadAudits(), loadJobs()])
  } finally {
    operationSubmitting.value = false
  }
}

const openPermissionDialog = (row?: any) => {
  if (row) {
    Object.assign(permissionForm, {
      id: row.id,
      roleId: row.roleId,
      instanceId: row.instanceId,
      permissionValues: permissionOptions.filter(item => hasPermissionMask(row.permissions, item.value)).map(item => item.value)
    })
  } else {
    Object.assign(permissionForm, { id: 0, roleId: undefined, instanceId: undefined, permissionValues: [MQ_PERMISSION.VIEW, MQ_PERMISSION.DIAGNOSE] })
  }
  permissionDialogVisible.value = true
}

const submitPermission = async () => {
  if (!permissionForm.roleId || !permissionForm.instanceId) {
    ElMessage.warning('请选择角色和实例')
    return
  }
  const permissions = permissionForm.permissionValues.reduce((sum: number, value: number) => sum | value, 0)
  permissionSubmitting.value = true
  try {
    await upsertMQInstancePermission({ roleId: permissionForm.roleId, instanceId: permissionForm.instanceId, permissions })
    ElMessage.success('保存成功')
    permissionDialogVisible.value = false
    await Promise.all([loadPermissions(), loadInstances()])
  } finally {
    permissionSubmitting.value = false
  }
}

const handleDeletePermission = async (row: any) => {
  await ElMessageBox.confirm('确认删除该权限配置？', '删除确认', { type: 'warning' })
  await deleteMQInstancePermission(row.id)
  ElMessage.success('删除成功')
  await Promise.all([loadPermissions(), loadInstances()])
}

const hasPermissionMask = (permissions: number, value: number) => (Number(permissions || 0) & value) > 0
const hasObjectPermission = (row: any, permission: number) => !row || row.permissions === undefined || hasPermissionMask(row.permissions, permission)
const canUse = (row: any, permission: number, uiKey: string) => !!uiPermissions[uiKey] && hasObjectPermission(row, permission)
const mqTypeTag = (type: string) => type === 'rabbitmq' ? 'success' : type === 'kafka' ? 'warning' : type === 'pulsar' ? 'primary' : 'info'
const healthTag = (status: string) => status === 'healthy' ? 'success' : status === 'critical' ? 'danger' : status === 'warning' ? 'warning' : 'info'
const jobStatusTag = (status: string) => status === 'success' ? 'success' : status === 'failed' || status === 'timeout' ? 'danger' : status === 'partial_success' ? 'warning' : status === 'running' ? 'primary' : 'info'
const environmentText = (value: string) => ({ prod: '生产', staging: '预发', test: '测试', dev: '开发' } as Record<string, string>)[value] || value
const formatRate = (value: number) => Number(value || 0).toFixed(2)
const isHighRisk = (riskLevel: string) => ['high', 'critical'].includes(String(riskLevel || '').toLowerCase())
const riskTag = (riskLevel: string) => riskLevel === 'critical' ? 'danger' : riskLevel === 'high' ? 'danger' : riskLevel === 'medium' ? 'warning' : 'info'
const riskText = (riskLevel: string) => ({ low: '低风险', medium: '中风险', high: '高风险', critical: '严重风险' } as Record<string, string>)[riskLevel] || riskLevel || '-'
const severityTag = (status: string) => ['critical', 'danger', 'failed'].includes(String(status || '').toLowerCase()) ? 'danger' : status === 'warning' ? 'warning' : status === 'info' ? 'info' : 'success'
const sectionStatusText = (status: string) => ({ success: '正常', healthy: '健康', warning: '警告', critical: '异常', info: '提示' } as Record<string, string>)[status] || status || '-'
const formatDiffValue = (value: any) => {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}
const createIdempotencyKey = () => {
  const cryptoObj = window.crypto as Crypto | undefined
  if (cryptoObj?.randomUUID) return cryptoObj.randomUUID()
  return `mqop-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
const escapeRegExp = (value: string) => String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

watch(activeTab, async tab => {
  if (tab === 'resources') await handleResourceInstanceChange()
  if (tab === 'diagnosis') await loadDiagnosis()
  if (tab === 'jobs') await loadJobs()
  if (tab === 'audits') await loadAudits()
  if (tab === 'permissions') await loadPermissions()
})

onMounted(async () => {
  await Promise.all([loadSupportedTypes(), loadUIPermissions(), loadCredentials(), loadRoles()])
  await loadInstances()
  await Promise.all([loadResources(), loadDiagnosis(), loadAudits(), loadJobs(), loadPermissions()])
})
</script>

<style scoped>
.mq-page {
  padding: 24px;
  background: #f5f7fa;
  min-height: 100vh;
}

.page-header,
.main-tabs,
.search-bar,
.table-wrapper,
.panel,
.message-sampler {
  background: #ffffff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 24px;
}

.page-title-group {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: linear-gradient(135deg, #0f766e, #2563eb);
  color: #ffffff;
  font-size: 24px;
}

.page-title {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 600;
  color: #1f2937;
}

.page-subtitle {
  margin: 0;
  color: #6b7280;
  font-size: 14px;
}

.header-actions,
.search-bar,
.workspace-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.black-button {
  background: #111827;
  border-color: #111827;
  color: #ffffff;
}

.black-button:hover,
.black-button:focus {
  background: #374151;
  border-color: #374151;
  color: #ffffff;
}

.main-tabs {
  padding: 0 24px 24px;
}

.search-bar,
.workspace-toolbar {
  justify-content: space-between;
  margin-bottom: 16px;
  padding: 16px;
  box-shadow: none;
  border: 1px solid #eef0f3;
}

.search-inputs {
  display: flex;
  flex: 1;
  gap: 12px;
  min-width: 0;
}

.search-input {
  width: 320px;
}

.search-select {
  width: 160px;
}

.instance-select {
  width: 300px;
}

.table-wrapper {
  padding: 16px;
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(140px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.metric-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  background: #ffffff;
}

.metric-label {
  color: #6b7280;
  font-size: 13px;
}

.metric-item strong {
  color: #111827;
  font-size: 20px;
}

.split-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
}

.diagnosis-summary,
.inspection-panel {
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  background: #ffffff;
}

.summary-tags,
.health-reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.summary-time {
  color: #6b7280;
  font-size: 13px;
}

.health-reasons {
  margin-top: 10px;
}

.health-reasons span {
  padding: 4px 8px;
  border-radius: 6px;
  background: #f8fafc;
  color: #475569;
  font-size: 13px;
}

.inspection-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.inspection-summary {
  color: #6b7280;
  font-size: 13px;
}

.score-box {
  min-width: 140px;
  text-align: right;
}

.score-box strong {
  display: block;
  color: #111827;
  font-size: 32px;
  line-height: 1;
}

.score-box span {
  color: #6b7280;
  font-size: 12px;
}

.score-box.warning strong {
  color: #d97706;
}

.score-box.critical strong {
  color: #dc2626;
}

.inspection-sections {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.inspection-section {
  padding: 12px;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  background: #fafbfc;
}

.section-title {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  font-weight: 600;
  color: #1f2937;
}

.section-summary,
.section-metrics {
  color: #6b7280;
  font-size: 13px;
}

.section-metrics {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}

.panel {
  padding: 16px;
}

.panel-title {
  margin-bottom: 12px;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.message-sampler {
  display: grid;
  grid-template-columns: 420px minmax(0, 1fr);
  gap: 20px;
  padding: 20px;
}

.sampler-form {
  border-right: 1px solid #eef0f3;
  padding-right: 20px;
}

.sample-result {
  min-width: 0;
}

.sample-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sample-item {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}

.sample-meta {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 10px 12px;
  background: #f8fafc;
  color: #475569;
  font-size: 13px;
}

.sample-item pre {
  margin: 0;
  padding: 12px;
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: #111827;
  font-size: 13px;
}

.permission-tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mono-textarea :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.operation-validation {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  background: #fafbfc;
}

.validation-header,
.validation-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.validation-list span {
  padding: 4px 8px;
  border-radius: 6px;
  background: #ffffff;
  color: #475569;
  font-size: 13px;
}

.operation-diff-table {
  width: 100%;
}

.danger {
  color: #dc2626;
  font-weight: 600;
}

@media (max-width: 1200px) {
  .page-header,
  .search-bar,
  .workspace-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .search-inputs {
    flex-wrap: wrap;
  }

  .split-layout,
  .message-sampler,
  .metric-grid,
  .inspection-sections {
    grid-template-columns: 1fr;
  }

  .sampler-form {
    border-right: 0;
    border-bottom: 1px solid #eef0f3;
    padding-right: 0;
    padding-bottom: 16px;
  }
}
</style>
