<template>
  <div class="virtualization-page">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><DataBoard /></el-icon>
        </div>
        <div>
          <h2 class="page-title">虚拟化平台管理</h2>
          <p class="page-subtitle">统一纳管 ESXi / PVE 平台，展示拓扑并完成虚机纳管</p>
        </div>
      </div>
      <div class="header-actions">
        <el-select v-model="virtualizationSettings.conflictPolicy" class="policy-select">
          <el-option label="严格策略（高风险拦截）" value="strict" />
          <el-option label="宽松策略（仅提示）" value="warn" />
        </el-select>
        <div class="write-switch-group">
          <span class="write-switch-label">写操作</span>
          <el-switch
            v-model="virtualizationSettings.writeOperationsEnabled"
            inline-prompt
            active-text="开"
            inactive-text="关"
          />
        </div>
        <el-button class="reset-btn" :loading="settingsSubmitting" @click="saveVirtualizationSettings">保存策略</el-button>
        <el-button class="black-button" @click="openPlatformDialog()">
          <el-icon style="margin-right: 6px;"><Plus /></el-icon>
          新增平台
        </el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="main-tabs">
      <el-tab-pane label="平台管理" name="platforms">
        <div class="search-bar">
          <div class="search-inputs">
            <el-input
              v-model="platformQuery.keyword"
              placeholder="搜索平台名称、地址、类型..."
              clearable
              class="search-input"
              @keyup.enter="loadPlatforms"
              @clear="loadPlatforms"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>
          <div class="search-actions">
            <el-button class="reset-btn" @click="resetPlatformQuery">
              <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
              重置
            </el-button>
          </div>
        </div>

        <div class="table-wrapper">
          <el-table
            :data="platformList"
            v-loading="platformLoading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="平台名称" min-width="160" prop="name" />
            <el-table-column label="类型" width="140" align="center">
              <template #default="{ row }">
                <el-tag :type="row.provider === 'pve' ? 'warning' : 'success'">
                  {{ row.providerText || row.provider }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="地址" min-width="230">
              <template #default="{ row }">
                <span>{{ row.endpoint }}:{{ row.port }}</span>
              </template>
            </el-table-column>
            <el-table-column label="账号" min-width="140" prop="username" />
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'enabled' ? 'success' : 'info'">
                  {{ row.status === 'enabled' ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="最近同步" min-width="190" align="center">
              <template #default="{ row }">
                <div>{{ row.lastSyncAt || '-' }}</div>
                <el-tag size="small" :type="syncStatusType(row.lastSyncStatus)" style="margin-top: 4px;">
                  {{ row.lastSyncStatus || 'idle' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="260" align="center" fixed="right">
              <template #default="{ row }">
                <el-tooltip content="测试连接" placement="top">
                  <el-button link type="primary" @click="handleTestPlatform(row)" :loading="testingPlatformId === row.id">
                    <el-icon><Connection /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="立即同步" placement="top">
                  <el-button link type="primary" @click="handleSyncPlatform(row)" :loading="syncingPlatformId === row.id">
                    <el-icon><Refresh /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="同步任务" placement="top">
                  <el-button link type="info" @click="openSyncJobDialog(row)">
                    <el-icon><List /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="编辑" placement="top">
                  <el-button link type="primary" @click="openPlatformDialog(row)">
                    <el-icon><Edit /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="删除" placement="top">
                  <el-button link type="danger" @click="handleDeletePlatform(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </el-tooltip>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="platformQuery.page"
              v-model:page-size="platformQuery.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="platformTotal"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadPlatforms"
              @current-change="loadPlatforms"
            />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="拓扑视图" name="topology">
        <div class="search-bar">
          <div class="search-inputs">
            <el-select
              v-model="topologyPlatformId"
              placeholder="选择平台"
              clearable
              class="search-input"
            >
              <el-option v-for="item in platformOptions" :key="item.id" :value="item.id" :label="item.name" />
            </el-select>
          </div>
          <div class="search-actions">
            <el-button class="black-button" @click="loadTopology" :loading="topologyLoading">
              <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
              刷新拓扑
            </el-button>
          </div>
        </div>

        <div class="topology-panel" v-loading="topologyLoading">
          <el-empty v-if="topologyPlatforms.length === 0" description="暂无拓扑数据，请先执行平台同步" />
          <template v-else>
            <div class="topology-summary">
              <div class="summary-card">
                <div class="summary-label">平台</div>
                <div class="summary-value">{{ topologyStats.platformCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">集群</div>
                <div class="summary-value">{{ topologyStats.clusterCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">宿主机</div>
                <div class="summary-value">{{ topologyStats.hostCount }}</div>
              </div>
              <div class="summary-card">
                <div class="summary-label">虚机</div>
                <div class="summary-value">{{ topologyStats.guestCount }}</div>
              </div>
            </div>

            <div class="trend-scope-toolbar">
              <div class="trend-scope-left">
                <div class="trend-scope-label">趋势视角</div>
                <el-radio-group v-model="trendScopeType" size="small">
                  <el-radio-button label="platform">平台</el-radio-button>
                  <el-radio-button label="cluster">集群</el-radio-button>
                </el-radio-group>
              </div>
              <div class="trend-scope-right">
                <el-select
                  v-if="trendScopeType === 'cluster'"
                  v-model="trendClusterId"
                  placeholder="选择集群"
                  clearable
                  class="search-input"
                >
                  <el-option
                    v-for="item in trendClusterOptions"
                    :key="item.id"
                    :value="item.id"
                    :label="`${item.name} · ${item.guestCount || 0} 虚机`"
                  />
                </el-select>
              </div>
            </div>

            <VirtualizationPlatformTrendPanel
              v-if="currentTrendPlatformId"
              class="topology-trend-panel"
              :title="trendPanelTitle"
              :trend="platformTrend"
              :loading="platformTrendLoading"
              :range="platformTrendRange"
              :selected-metrics="selectedTrendMetrics"
              :metric-options="trendMetricOptions"
              @change-range="handlePlatformTrendRangeChange"
              @change-metrics="handleTrendMetricChange"
            />

            <div class="platform-card-list">
              <section v-for="platform in topologyPlatforms" :key="platform.id" class="platform-card">
                <header class="platform-card-header">
                  <div>
                    <div class="platform-name">{{ platform.name }}</div>
                    <div class="platform-meta">
                      <el-tag :type="platform.provider === 'pve' ? 'warning' : 'success'" size="small">
                        {{ platform.providerText || platform.provider }}
                      </el-tag>
                      <el-tag :type="platform.status === 'enabled' ? 'success' : 'info'" size="small">
                        {{ platform.status === 'enabled' ? '启用' : '禁用' }}
                      </el-tag>
                    </div>
                  </div>
                  <div class="platform-sync-time">
                    最近同步：{{ platform.lastSyncAt || '-' }}
                  </div>
                </header>

                <div class="cluster-grid">
                  <article
                    v-for="cluster in platform.clusters || []"
                    :key="`${platform.id}-${cluster.id}-${cluster.name}`"
                    class="cluster-card"
                    :class="{ 'cluster-card-active': trendScopeType === 'cluster' && trendClusterId === cluster.id }"
                    @click="selectTrendCluster(platform.id, cluster.id)"
                  >
                    <div class="cluster-title-row">
                      <div class="cluster-title">{{ cluster.name }}</div>
                      <div class="cluster-metrics">
                        <span>{{ cluster.hostCount || 0 }} 宿主机</span>
                        <span>{{ cluster.guestCount || 0 }} 虚机</span>
                      </div>
                    </div>
                    <div class="host-list">
                      <div v-if="!cluster.hosts || cluster.hosts.length === 0" class="empty-host">暂无宿主机</div>
                      <div
                        v-for="host in cluster.hosts || []"
                        :key="host.id"
                        class="host-chip"
                        :class="`host-${host.status || 'unknown'}`"
                      >
                        <span class="host-dot" />
                        <span class="host-name">{{ host.name }}</span>
                        <span class="host-ip">{{ host.managementIp || '-' }}</span>
                        <span class="host-vm">VM {{ host.guestCount || 0 }}</span>
                      </div>
                    </div>
                  </article>
                </div>
              </section>
            </div>
          </template>
        </div>
      </el-tab-pane>

      <el-tab-pane label="虚机纳管" name="guests">
        <div class="guest-platform-workspace">
          <div class="guest-platform-header">
            <div>
              <div class="workspace-title">平台工作台</div>
              <div class="workspace-subtitle">先选择一个平台，再查看并纳管该平台的虚机，避免多平台混在一张表里。</div>
            </div>
            <el-button class="reset-btn" @click="refreshGuestWorkspace">
              <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
              刷新平台
            </el-button>
          </div>
          <div class="guest-platform-list" v-if="platformOptions.length > 0">
            <button
              v-for="item in platformCatalog"
              :key="item.id"
              type="button"
              class="guest-platform-card"
              :class="{ 'guest-platform-card-active': guestQuery.platformId === item.id }"
              @click="selectGuestPlatform(item.id)"
            >
              <div class="guest-platform-card-top">
                <span class="guest-platform-card-name">{{ item.name }}</span>
                <el-tag size="small" :type="item.provider === 'pve' ? 'warning' : 'success'">
                  {{ item.providerText || item.provider }}
                </el-tag>
              </div>
              <div class="guest-platform-card-meta">{{ item.endpoint }}:{{ item.port }}</div>
              <div class="guest-platform-card-meta">最近同步：{{ item.lastSyncAt || '-' }}</div>
            </button>
          </div>
          <el-empty v-else description="暂无虚拟化平台，请先新增并同步平台" :image-size="72" />
        </div>

        <el-alert
          v-if="!virtualizationSettings.writeOperationsEnabled"
          class="guest-write-alert"
          title="当前未开启虚拟化写操作，仅允许查看、控制台跳转、纳管和解绑；开关机与快照修改已被拦截。"
          type="info"
          :closable="false"
          show-icon
        />

        <div class="search-bar">
          <div class="search-inputs">
            <el-input
              v-model="guestQuery.keyword"
              placeholder="搜索虚机名称、IP、外部ID..."
              clearable
              class="search-input"
              @keyup.enter="loadGuests"
              @clear="loadGuests"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="guestQuery.bound" placeholder="纳管状态" class="search-input">
              <el-option label="全部" value="all" />
              <el-option label="已纳管" value="bound" />
              <el-option label="未纳管" value="unbound" />
            </el-select>
            <el-select v-model="guestQuery.powerState" clearable placeholder="电源状态" class="search-input">
              <el-option label="开机" value="powered_on" />
              <el-option label="关机" value="powered_off" />
              <el-option label="挂起" value="suspended" />
              <el-option label="未知" value="unknown" />
            </el-select>
          </div>
          <div class="search-actions">
            <el-button class="reset-btn" @click="resetGuestQuery">
              <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
              重置
            </el-button>
          </div>
        </div>

        <div v-if="!guestQuery.platformId" class="guest-empty-state">
          <el-empty description="请先选择一个平台，再查看该平台下的虚机列表" :image-size="84" />
        </div>

        <div v-else class="table-wrapper">
          <el-table
            :data="guestList"
            v-loading="guestLoading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="虚机名称" min-width="180" prop="name" />
            <el-table-column label="外部ID" min-width="180" prop="externalId" />
            <el-table-column label="IP" width="150" align="center">
              <template #default="{ row }">
                {{ row.primaryIp || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="系统" min-width="170">
              <template #default="{ row }">
                {{ row.osType || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="powerStatusType(row.powerState)">
                  {{ powerStatusText(row.powerState) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="联动状态" width="130" align="center">
              <template #default="{ row }">
                <template v-if="row.runtimeStatusText">
                  <el-tag :type="runtimeStatusType(row.runtimeStatus)">
                    {{ row.runtimeStatusText }}
                  </el-tag>
                  <div v-if="row.runtimeStatusSource === 'asset_host'" class="runtime-source">资产主机</div>
                </template>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column label="规格" min-width="130" align="center">
              <template #default="{ row }">
                {{ row.cpuCount || 0 }}C / {{ row.memoryMb || 0 }}MB
              </template>
            </el-table-column>
            <el-table-column label="纳管状态" min-width="180" align="center">
              <template #default="{ row }">
                <el-tag :type="row.bindingStatus === 'bound' ? 'success' : 'info'">
                  {{ row.bindingStatus === 'bound' ? '已纳管' : '未纳管' }}
                </el-tag>
                <div class="binding-host" v-if="row.assetHostName">
                  <el-link type="primary" :underline="false" @click="jumpToHostDetail(row)">
                    {{ row.assetHostName }}
                  </el-link>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="420" align="center" fixed="right">
              <template #default="{ row }">
                <template v-if="row.powerState === 'powered_on'">
                  <el-button
                    link
                    type="warning"
                    :loading="powerActionLoadingKey === `${row.id}:reboot`"
                    :disabled="!virtualizationSettings.writeOperationsEnabled"
                    @click="openPowerDialog(row, 'reboot')"
                  >
                    重启
                  </el-button>
                  <el-button
                    link
                    type="danger"
                    :loading="powerActionLoadingKey === `${row.id}:power_off`"
                    :disabled="!virtualizationSettings.writeOperationsEnabled"
                    @click="openPowerDialog(row, 'power_off')"
                  >
                    关机
                  </el-button>
                </template>
                <el-button
                  v-else
                  link
                  type="success"
                  :loading="powerActionLoadingKey === `${row.id}:power_on`"
                  :disabled="!virtualizationSettings.writeOperationsEnabled"
                  @click="openPowerDialog(row, 'power_on')"
                >
                  开机
                </el-button>
                <el-button
                  link
                  type="primary"
                  :loading="consoleActionLoadingKey === String(row.id)"
                  @click="openGuestConsole(row)"
                >
                  控制台
                </el-button>
                <el-button link type="primary" @click="openSnapshotDialog(row)">
                  快照
                </el-button>
                <el-button
                  v-if="row.bindingStatus !== 'bound'"
                  link
                  type="primary"
                  @click="openOnboardDialog(row)"
                >
                  纳管
                </el-button>
                <el-button
                  v-else
                  link
                  type="danger"
                  @click="handleUnbindGuest(row)"
                >
                  解绑
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="guestQuery.page"
              v-model:page-size="guestQuery.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="guestTotal"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadGuests"
              @current-change="loadGuests"
            />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="操作审计" name="audit">
        <div class="search-bar">
          <div class="search-inputs">
            <el-select v-model="actionLogQuery.platformId" clearable placeholder="选择平台" class="search-input">
              <el-option
                v-for="item in platformOptions"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
            </el-select>
            <el-select v-model="actionLogQuery.action" clearable placeholder="操作类型" class="search-input">
              <el-option
                v-for="item in actionLogActionOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
            <el-select v-model="actionLogQuery.status" clearable placeholder="执行状态" class="search-input">
              <el-option
                v-for="item in actionLogStatusOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
            <el-input
              v-model="actionLogQuery.keyword"
              placeholder="搜索目标、操作人、原因、结果..."
              clearable
              class="search-input audit-search-input"
              @keyup.enter="handleActionLogSearch"
              @clear="handleActionLogSearch"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>
          <div class="search-actions">
            <el-button class="reset-btn" @click="resetActionLogQuery">
              <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
              重置
            </el-button>
            <el-button class="black-button" :loading="actionLogLoading" @click="loadActionLogs">
              <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
              刷新日志
            </el-button>
          </div>
        </div>

        <div class="table-wrapper">
          <el-table
            :data="actionLogList"
            v-loading="actionLogLoading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="开始时间" min-width="170" align="center">
              <template #default="{ row }">
                <div>{{ row.startedAt || row.createTime || '-' }}</div>
                <div class="audit-subtext">完成：{{ row.finishedAt || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="平台" min-width="140">
              <template #default="{ row }">
                {{ row.platformName || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="操作对象" min-width="220">
              <template #default="{ row }">
                <div class="audit-target">{{ row.targetName || '-' }}</div>
                <div class="audit-subtext">{{ row.targetType || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="动作" width="120" align="center">
              <template #default="{ row }">
                <el-tag effect="plain">{{ row.actionText || row.action }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="风险" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="auditRiskType(row.riskLevel)">
                  {{ auditRiskText(row.riskLevel) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="auditStatusType(row.status)">
                  {{ row.statusText || auditStatusText(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作人" min-width="120" align="center">
              <template #default="{ row }">
                {{ row.operatorName || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="操作原因" min-width="220">
              <template #default="{ row }">
                <span class="audit-multiline">{{ row.reason || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="结果信息" min-width="220">
              <template #default="{ row }">
                <span class="audit-multiline">{{ row.resultMessage || '-' }}</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="actionLogQuery.page"
              v-model:page-size="actionLogQuery.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="actionLogTotal"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadActionLogs"
              @current-change="loadActionLogs"
            />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog
      v-model="platformDialogVisible"
      :title="platformForm.id ? '编辑虚拟化平台' : '新增虚拟化平台'"
      width="680px"
      @close="resetPlatformForm"
    >
      <el-form ref="platformFormRef" :model="platformForm" :rules="platformRules" label-width="120px">
        <el-form-item label="平台名称" prop="name">
          <el-input v-model="platformForm.name" />
        </el-form-item>
        <el-form-item label="平台类型" prop="provider">
          <el-radio-group v-model="platformForm.provider">
            <el-radio value="esxi">ESXi / vCenter</el-radio>
            <el-radio value="pve">PVE</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="API地址" prop="endpoint">
          <el-input v-model="platformForm.endpoint" placeholder="如 10.0.0.21 或 https://10.0.0.21" />
        </el-form-item>
        <el-form-item label="端口" prop="port">
          <el-input-number v-model="platformForm.port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="platformForm.username"
            :placeholder="platformForm.provider === 'pve' ? '建议填写 root@pam（系统也会自动尝试 realm）' : '如 administrator@vsphere.local'"
          />
          <div v-if="platformForm.provider === 'pve'" class="field-tip">
            PVE 账号建议带 realm，例如 <code>root@pam</code>。若只填写 <code>root</code>，系统会自动尝试 <code>root@pam / root@pve</code>。
          </div>
        </el-form-item>
        <el-form-item :label="platformForm.id ? '密码(留空不改)' : '密码'" prop="password">
          <el-input v-model="platformForm.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="证书校验">
          <el-switch v-model="platformForm.insecureSkipVerify" />
          <span class="switch-text">{{ platformForm.insecureSkipVerify ? '跳过证书校验' : '启用证书校验' }}</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="platformForm.status">
            <el-radio value="enabled">启用</el-radio>
            <el-radio value="disabled">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="platformForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="platformDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="platformSubmitting" @click="submitPlatform">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="syncJobDialogVisible" title="同步任务记录" width="760px">
      <el-table :data="syncJobList" v-loading="syncJobLoading" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="triggerType" label="触发方式" width="100" />
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column prop="startedAt" label="开始时间" min-width="170" />
        <el-table-column prop="finishedAt" label="结束时间" min-width="170" />
        <el-table-column label="条目统计" min-width="180">
          <template #default="{ row }">
            total:{{ row.itemsTotal }} / +{{ row.itemsCreated }} / ~{{ row.itemsUpdated }} / -{{ row.itemsDeleted }}
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="onboardDialogVisible" title="纳管虚机" width="560px">
      <el-form :model="onboardForm" label-width="120px">
        <el-form-item label="虚机">
          <el-input :model-value="currentGuest?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="目标主机" required>
          <el-select v-model="onboardForm.assetHostId" placeholder="选择资产主机" filterable style="width: 100%">
            <el-option v-for="item in hostOptions" :key="item.id" :label="`${item.name} (${item.ip})`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="onboardForm.bindingNote" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>

      <div class="precheck-panel" v-loading="precheckLoading">
        <div class="precheck-header">
          <span>纳管预检查（当前策略：{{ precheckPolicyText(precheckResult?.conflictPolicy) }}）</span>
          <el-tag size="small" :type="precheckRiskType(precheckResult?.riskLevel)">
            {{ precheckRiskText(precheckResult?.riskLevel) }}
          </el-tag>
        </div>
        <div v-if="precheckResult?.suggestedHost" class="precheck-suggest">
          建议主机：{{ precheckResult.suggestedHost.name }} ({{ precheckResult.suggestedHost.ip || '-' }})
          <el-button
            link
            type="primary"
            @click="useSuggestedHost"
            v-if="precheckResult.suggestedHost.id !== onboardForm.assetHostId"
          >
            一键使用
          </el-button>
        </div>
        <div v-if="precheckResult?.checks?.length" class="precheck-check-list">
          <div
            v-for="item in precheckResult.checks"
            :key="item.code"
            class="precheck-item"
            :class="`risk-${item.level}`"
          >
            <el-icon class="precheck-icon"><WarningFilled /></el-icon>
            <span>{{ item.message }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="onboardDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="onboardingGuest" @click="handleOnboardGuest">确认纳管</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="powerDialogVisible"
      :title="`${selectedPowerActionText}虚机`"
      width="620px"
      @close="resetPowerForm"
    >
      <div class="power-dialog-intro">
        <el-alert
          :title="`${selectedPowerActionText}属于${selectedPowerActionRiskText}操作，系统会记录审计日志。`"
          type="warning"
          :closable="false"
          show-icon
        />
      </div>
      <el-form :model="powerForm" label-width="120px">
        <el-form-item label="虚机">
          <el-input :model-value="powerGuest?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="当前状态">
          <el-input :model-value="powerGuest ? powerStatusText(powerGuest.powerState) : '-'" disabled />
        </el-form-item>
        <el-form-item label="目标动作">
          <el-input :model-value="selectedPowerActionText" disabled />
        </el-form-item>
        <el-form-item label="操作原因" required>
          <el-input
            v-model="powerForm.reason"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
            placeholder="请输入本次开机 / 关机 / 重启的原因，审计日志会保留该说明"
          />
        </el-form-item>
        <el-form-item label="二次确认" required>
          <el-input
            v-model="powerForm.confirmText"
            :placeholder="`请输入虚机名 ${powerGuest?.name || ''} 确认操作`"
          />
          <div class="field-tip">
            请输入虚机名 <code>{{ powerGuest?.name || '-' }}</code> 完成二次确认。
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="powerDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="powerSubmitting" @click="submitPowerAction">
          确认{{ selectedPowerActionText }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="snapshotDialogVisible"
      title="虚机快照管理"
      width="980px"
      @close="handleSnapshotDialogClose"
    >
      <div class="snapshot-dialog-layout">
        <section class="snapshot-create-panel">
          <div class="snapshot-panel-title">创建快照</div>
          <el-form :model="snapshotCreateForm" label-width="100px">
            <el-form-item label="虚机">
              <el-input :model-value="snapshotGuest?.name || '-'" disabled />
            </el-form-item>
            <el-form-item label="当前状态">
              <el-input :model-value="snapshotGuest ? powerStatusText(snapshotGuest.powerState) : '-'" disabled />
            </el-form-item>
            <el-form-item label="快照名称" required>
              <el-input v-model="snapshotCreateForm.name" maxlength="80" show-word-limit />
            </el-form-item>
            <el-form-item label="描述">
              <el-input
                v-model="snapshotCreateForm.description"
                type="textarea"
                :rows="3"
                maxlength="500"
                show-word-limit
                placeholder="可选，记录本次快照用途"
              />
            </el-form-item>
            <el-form-item label="内存状态">
              <el-switch v-model="snapshotCreateForm.includeMemory" />
              <span class="switch-text">{{ snapshotCreateForm.includeMemory ? '包含内存' : '仅磁盘快照' }}</span>
            </el-form-item>
            <el-form-item label="文件系统">
              <el-switch v-model="snapshotCreateForm.quiesce" />
              <span class="switch-text">{{ snapshotCreateForm.quiesce ? '静默一致性' : '普通快照' }}</span>
            </el-form-item>
            <el-form-item label="操作原因" required>
              <el-input
                v-model="snapshotCreateForm.reason"
                type="textarea"
                :rows="3"
                maxlength="500"
                show-word-limit
                placeholder="请输入创建快照的原因，审计日志会保留该说明"
              />
            </el-form-item>
          </el-form>
          <div class="snapshot-create-actions">
            <el-button class="reset-btn" @click="resetSnapshotCreateForm">清空</el-button>
            <el-button
              type="primary"
              :loading="snapshotSubmitting"
              :disabled="!virtualizationSettings.writeOperationsEnabled"
              @click="submitSnapshotCreate"
            >
              创建快照
            </el-button>
          </div>
        </section>

        <section class="snapshot-list-panel">
          <div class="snapshot-list-header">
            <div>
              <div class="snapshot-panel-title">快照列表</div>
              <div class="snapshot-panel-subtitle">展示平台侧当前可见的快照记录，支持回滚和删除。</div>
            </div>
            <el-button class="reset-btn" :loading="snapshotLoading" @click="loadSnapshots">
              <el-icon style="margin-right: 4px;"><Refresh /></el-icon>
              刷新快照
            </el-button>
          </div>

          <el-table
            :data="snapshotList"
            v-loading="snapshotLoading"
            stripe
            class="modern-table snapshot-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="快照名称" min-width="150">
              <template #default="{ row }">
                <div class="snapshot-name-cell">
                  <span>{{ row.name }}</span>
                  <el-tag v-if="row.current" size="small" type="success">当前</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="160" align="center">
              <template #default="{ row }">
                {{ row.createdAt || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="父快照" min-width="130" align="center">
              <template #default="{ row }">
                {{ row.parentName || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="特性" min-width="180" align="center">
              <template #default="{ row }">
                <div class="snapshot-feature-tags">
                  <el-tag size="small" :type="row.includeMemory ? 'warning' : 'info'">
                    {{ row.includeMemory ? '含内存' : '仅磁盘' }}
                  </el-tag>
                  <el-tag size="small" :type="row.quiesced ? 'success' : 'info'">
                    {{ row.quiesced ? '静默一致性' : '未静默' }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="powerStatusType(row.powerState)">
                  {{ snapshotPowerStateText(row.powerState) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="描述" min-width="180">
              <template #default="{ row }">
                <span class="audit-multiline">{{ row.description || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150" align="center" fixed="right">
              <template #default="{ row }">
                <template v-if="!row.current">
                  <el-button
                    link
                    type="warning"
                    :disabled="!virtualizationSettings.writeOperationsEnabled"
                    @click="openSnapshotActionDialog(row, 'rollback')"
                  >
                    回滚
                  </el-button>
                  <el-button
                    link
                    type="danger"
                    :disabled="!virtualizationSettings.writeOperationsEnabled"
                    @click="openSnapshotActionDialog(row, 'delete')"
                  >
                    删除
                  </el-button>
                </template>
                <span v-else class="audit-subtext">当前快照</span>
              </template>
            </el-table-column>
          </el-table>
        </section>
      </div>
    </el-dialog>

    <el-dialog
      v-model="snapshotActionDialogVisible"
      :title="selectedSnapshotActionText"
      width="620px"
      @close="handleSnapshotActionDialogClose"
    >
      <div class="power-dialog-intro">
        <el-alert
          :title="`${selectedSnapshotActionText}属于高风险操作，系统会记录审计日志。`"
          type="warning"
          :closable="false"
          show-icon
        />
      </div>
      <el-form :model="snapshotActionForm" label-width="120px">
        <el-form-item label="虚机">
          <el-input :model-value="snapshotGuest?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="快照名称">
          <el-input :model-value="currentSnapshotRecord?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="目标动作">
          <el-input :model-value="selectedSnapshotActionText" disabled />
        </el-form-item>
        <el-form-item label="操作原因" required>
          <el-input
            v-model="snapshotActionForm.reason"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
            placeholder="请输入本次快照回滚 / 删除的原因"
          />
        </el-form-item>
        <el-form-item label="二次确认" required>
          <el-input
            v-model="snapshotActionForm.confirmText"
            :placeholder="`请输入快照名称 ${currentSnapshotRecord?.name || ''} 确认操作`"
          />
          <div class="field-tip">
            请输入快照名称 <code>{{ currentSnapshotRecord?.name || '-' }}</code> 完成二次确认。
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="snapshotActionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="snapshotActionSubmitting" @click="submitSnapshotAction">
          确认{{ selectedSnapshotActionText }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Connection,
  DataBoard,
  Delete,
  Edit,
  List,
  Plus,
  Refresh,
  RefreshLeft,
  Search,
  WarningFilled
} from '@element-plus/icons-vue'
import {
  createVirtualizationGuestSnapshot,
  createVirtualizationGuestConsoleLink,
  createVirtualizationPlatform,
  deleteVirtualizationGuestSnapshot,
  deleteVirtualizationPlatform,
  getVirtualizationClusterTrend,
  getVirtualizationSettings,
  getVirtualizationPlatformTrend,
  listVirtualizationActionLogs,
  listVirtualizationGuestSnapshots,
  listVirtualizationGuests,
  listVirtualizationPlatforms,
  listVirtualizationSyncJobs,
  onboardVirtualizationGuest,
  powerVirtualizationGuest,
  precheckVirtualizationGuestOnboard,
  rollbackVirtualizationGuestSnapshot,
  syncVirtualizationPlatform,
  testVirtualizationPlatform,
  unbindVirtualizationGuest,
  updateVirtualizationSettings,
  updateVirtualizationPlatform,
  getVirtualizationTopology
} from '@/api/virtualization'
import { getHostList } from '@/api/host'
import VirtualizationPlatformTrendPanel from './components/VirtualizationPlatformTrendPanel.vue'

const router = useRouter()
const activeTab = ref('platforms')

const platformLoading = ref(false)
const platformTotal = ref(0)
const platformList = ref<any[]>([])
const platformCatalog = ref<any[]>([])
const platformQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: ''
})

const platformOptions = computed(() => platformCatalog.value.map((item: any) => ({
  id: item.id,
  name: item.name,
  provider: item.provider,
  providerText: item.providerText,
  endpoint: item.endpoint,
  port: item.port,
  lastSyncAt: item.lastSyncAt
})))

const testingPlatformId = ref<number | null>(null)
const syncingPlatformId = ref<number | null>(null)

const platformDialogVisible = ref(false)
const platformSubmitting = ref(false)
const settingsSubmitting = ref(false)
const platformFormRef = ref()
const platformForm = reactive({
  id: 0,
  name: '',
  provider: 'esxi',
  endpoint: '',
  port: 443,
  username: '',
  password: '',
  insecureSkipVerify: true,
  status: 'enabled',
  description: ''
})

const platformRules = computed(() => {
  const rules: any = {
    name: [{ required: true, message: '请输入平台名称', trigger: 'blur' }],
    provider: [{ required: true, message: '请选择平台类型', trigger: 'change' }],
    endpoint: [{ required: true, message: '请输入 API 地址', trigger: 'blur' }],
    port: [{ required: true, message: '请输入端口', trigger: 'change' }],
    username: [{ required: true, message: '请输入用户名', trigger: 'blur' }]
  }
  if (!platformForm.id) {
    rules.password = [{ required: true, message: '请输入密码', trigger: 'blur' }]
  }
  return rules
})

const syncJobDialogVisible = ref(false)
const syncJobLoading = ref(false)
const syncJobList = ref<any[]>([])
const syncJobPlatformId = ref<number | null>(null)

const topologyLoading = ref(false)
const topologyPlatformId = ref<number | null>(null)
const topologyPlatforms = ref<any[]>([])
const platformTrendLoading = ref(false)
const platformTrendRange = ref<'24h' | '7d' | '15d'>('7d')
const platformTrend = ref<any>(null)
const trendScopeType = ref<'platform' | 'cluster'>('platform')
const trendClusterId = ref<number | null>(null)
const trendMetricOptions = [
  { label: '总数', value: 'guestTotal', color: '#111827' },
  { label: '开机', value: 'poweredOnGuests', color: '#2563eb' },
  { label: '关机', value: 'poweredOffGuests', color: '#94a3b8' },
  { label: '挂起', value: 'suspendedGuests', color: '#f59e0b' },
  { label: '已纳管', value: 'boundGuests', color: '#16a34a' },
  { label: '在线', value: 'onlineGuests', color: '#059669' },
  { label: '离线', value: 'offlineGuests', color: '#dc2626' },
  { label: '未配置', value: 'notConfiguredGuests', color: '#7c3aed' },
  { label: '未知', value: 'unknownGuests', color: '#475569' }
] as const
const selectedTrendMetrics = ref<Array<typeof trendMetricOptions[number]['value']>>([
  'guestTotal',
  'poweredOnGuests',
  'boundGuests',
  'onlineGuests',
  'offlineGuests'
])
const topologyStats = computed(() => {
  const platforms = topologyPlatforms.value || []
  let clusterCount = 0
  let hostCount = 0
  let guestCount = 0
  platforms.forEach((platform: any) => {
    const clusters = platform.clusters || []
    clusterCount += clusters.length
    clusters.forEach((cluster: any) => {
      hostCount += (cluster.hosts || []).length
      guestCount += Number(cluster.guestCount || 0)
    })
  })
  return {
    platformCount: platforms.length,
    clusterCount,
    hostCount,
    guestCount
  }
})
const currentTrendPlatformId = computed<number | null>(() => {
  return topologyPlatformId.value || topologyPlatforms.value[0]?.id || platformCatalog.value[0]?.id || null
})
const currentTrendPlatformName = computed(() => {
  const platformID = currentTrendPlatformId.value
  if (!platformID) return ''
  const platform = topologyPlatforms.value.find((item: any) => item.id === platformID)
    || platformCatalog.value.find((item: any) => item.id === platformID)
  return platform?.name || ''
})
const trendClusterOptions = computed(() => {
  const platformID = currentTrendPlatformId.value
  if (!platformID) return []
  const platform = topologyPlatforms.value.find((item: any) => item.id === platformID)
  return platform?.clusters || []
})
const currentTrendCluster = computed(() => {
  return trendClusterOptions.value.find((item: any) => item.id === trendClusterId.value) || null
})
const trendPanelTitle = computed(() => {
  if (trendScopeType.value === 'cluster' && currentTrendCluster.value) {
    return `${currentTrendPlatformName.value} / ${currentTrendCluster.value.name} · 集群趋势`
  }
  return currentTrendPlatformName.value ? `${currentTrendPlatformName.value} · 平台趋势` : '平台状态趋势'
})

const guestLoading = ref(false)
const guestTotal = ref(0)
const guestList = ref<any[]>([])
const guestQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  platformId: undefined as number | undefined,
  clusterId: undefined as number | undefined,
  powerState: '',
  bound: 'all' as 'all' | 'bound' | 'unbound'
})

const hostOptions = ref<any[]>([])
const onboardDialogVisible = ref(false)
const onboardingGuest = ref(false)
const currentGuest = ref<any>(null)
const precheckLoading = ref(false)
const precheckResult = ref<any>(null)
const virtualizationSettings = reactive({
  conflictPolicy: 'strict' as 'strict' | 'warn',
  writeOperationsEnabled: false
})
const onboardForm = reactive({
  assetHostId: undefined as number | undefined,
  bindingType: 'manual' as 'manual' | 'auto',
  bindingNote: ''
})
const powerDialogVisible = ref(false)
const powerSubmitting = ref(false)
const powerActionLoadingKey = ref('')
const consoleActionLoadingKey = ref('')
const powerGuest = ref<any>(null)
const powerForm = reactive({
  action: 'power_on' as 'power_on' | 'power_off' | 'reboot',
  reason: '',
  confirmText: ''
})
const selectedPowerActionText = computed(() => {
  if (powerForm.action === 'power_off') return '关机'
  if (powerForm.action === 'reboot') return '重启'
  return '开机'
})
const selectedPowerActionRiskText = computed(() => {
  if (powerForm.action === 'power_on') return '中风险'
  return '高风险'
})

const snapshotDialogVisible = ref(false)
const snapshotLoading = ref(false)
const snapshotSubmitting = ref(false)
const snapshotActionSubmitting = ref(false)
const snapshotGuest = ref<any>(null)
const snapshotList = ref<any[]>([])
const snapshotCreateForm = reactive({
  name: '',
  description: '',
  includeMemory: false,
  quiesce: false,
  reason: ''
})
const snapshotActionDialogVisible = ref(false)
const snapshotActionForm = reactive({
  action: 'rollback' as 'rollback' | 'delete',
  snapshotName: '',
  reason: '',
  confirmText: ''
})
const currentSnapshotRecord = ref<any>(null)
const selectedSnapshotActionText = computed(() => snapshotActionForm.action === 'delete' ? '删除快照' : '回滚快照')

const actionLogLoading = ref(false)
const actionLogTotal = ref(0)
const actionLogList = ref<any[]>([])
const actionLogActionOptions = [
  { label: '开机', value: 'power_on' },
  { label: '关机', value: 'power_off' },
  { label: '重启', value: 'reboot' },
  { label: '创建快照', value: 'snapshot_create' },
  { label: '回滚快照', value: 'snapshot_rollback' },
  { label: '删除快照', value: 'snapshot_delete' },
  { label: '控制台跳转', value: 'console_link' }
]
const actionLogStatusOptions = [
  { label: '处理中', value: 'pending' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已拒绝', value: 'denied' }
]
const actionLogQuery = reactive({
  page: 1,
  pageSize: 10,
  platformId: undefined as number | undefined,
  action: '',
  status: '',
  keyword: ''
})

const loadPlatforms = async () => {
  platformLoading.value = true
  try {
    const res: any = await listVirtualizationPlatforms({
      page: platformQuery.page,
      pageSize: platformQuery.pageSize,
      keyword: platformQuery.keyword
    })
    platformList.value = res.list || []
    platformTotal.value = res.total || 0
  } finally {
    platformLoading.value = false
  }
}

const loadPlatformCatalog = async () => {
  const res: any = await listVirtualizationPlatforms({
    page: 1,
    pageSize: 200,
    keyword: ''
  })
  platformCatalog.value = res.list || []
  normalizePlatformSelections()
}

const normalizePlatformSelections = () => {
  const ids = new Set(platformCatalog.value.map((item: any) => item.id))
  if (guestQuery.platformId && !ids.has(guestQuery.platformId)) {
    guestQuery.platformId = undefined
  }
  if (topologyPlatformId.value && !ids.has(topologyPlatformId.value)) {
    topologyPlatformId.value = null
  }
  if (!guestQuery.platformId && platformCatalog.value.length > 0) {
    guestQuery.platformId = platformCatalog.value[0].id
  }
}

const loadVirtualizationSettings = async () => {
  try {
    const data: any = await getVirtualizationSettings()
    if (data?.conflictPolicy === 'warn' || data?.conflictPolicy === 'strict') {
      virtualizationSettings.conflictPolicy = data.conflictPolicy
    }
    virtualizationSettings.writeOperationsEnabled = !!data?.writeOperationsEnabled
  } catch (error: any) {
    ElMessage.error(error.message || '加载纳管策略失败')
  }
}

const saveVirtualizationSettings = async () => {
  settingsSubmitting.value = true
  try {
    await updateVirtualizationSettings({
      conflictPolicy: virtualizationSettings.conflictPolicy,
      writeOperationsEnabled: virtualizationSettings.writeOperationsEnabled
    })
    ElMessage.success('冲突策略已更新')
    if (onboardDialogVisible.value && currentGuest.value?.id) {
      await loadGuestPrecheck()
    }
  } catch (error: any) {
    ElMessage.error(error.message || '保存冲突策略失败')
  } finally {
    settingsSubmitting.value = false
  }
}

const resetPlatformQuery = () => {
  platformQuery.page = 1
  platformQuery.pageSize = 10
  platformQuery.keyword = ''
  loadPlatforms()
}

const resetPlatformForm = () => {
  platformForm.id = 0
  platformForm.name = ''
  platformForm.provider = 'esxi'
  platformForm.endpoint = ''
  platformForm.port = 443
  platformForm.username = ''
  platformForm.password = ''
  platformForm.insecureSkipVerify = true
  platformForm.status = 'enabled'
  platformForm.description = ''
  platformFormRef.value?.clearValidate()
}

const openPlatformDialog = (row?: any) => {
  resetPlatformForm()
  if (row) {
    platformForm.id = row.id
    platformForm.name = row.name
    platformForm.provider = row.provider
    platformForm.endpoint = row.endpoint
    platformForm.port = row.port
    platformForm.username = row.username
    platformForm.password = ''
    platformForm.insecureSkipVerify = !!row.insecureSkipVerify
    platformForm.status = row.status || 'enabled'
    platformForm.description = row.description || ''
  }
  platformDialogVisible.value = true
}

watch(() => platformForm.provider, (provider) => {
  if (!platformForm.id) {
    platformForm.port = provider === 'pve' ? 8006 : 443
  }
})

const submitPlatform = async () => {
  const valid = await platformFormRef.value?.validate().catch(() => false)
  if (!valid) return

  platformSubmitting.value = true
  try {
    const payload = {
      name: platformForm.name,
      provider: platformForm.provider as 'esxi' | 'pve',
      endpoint: platformForm.endpoint,
      port: platformForm.port,
      username: platformForm.username,
      password: platformForm.password,
      insecureSkipVerify: platformForm.insecureSkipVerify,
      status: platformForm.status as 'enabled' | 'disabled',
      description: platformForm.description
    }
    if (platformForm.id) {
      await updateVirtualizationPlatform(platformForm.id, payload)
      ElMessage.success('平台更新成功')
    } else {
      await createVirtualizationPlatform(payload)
      ElMessage.success('平台创建成功')
    }
    platformDialogVisible.value = false
    await Promise.all([loadPlatforms(), loadPlatformCatalog()])
  } catch (error: any) {
    ElMessage.error(error.message || '保存失败')
  } finally {
    platformSubmitting.value = false
  }
}

const handleDeletePlatform = (row: any) => {
  ElMessageBox.confirm(`确定删除平台「${row.name}」吗？`, '提示', { type: 'warning' })
    .then(async () => {
      await deleteVirtualizationPlatform(row.id)
      ElMessage.success('删除成功')
      await Promise.all([loadPlatforms(), loadPlatformCatalog()])
      await loadTopology()
      await loadGuests()
    })
}

const handleTestPlatform = async (row: any) => {
  testingPlatformId.value = row.id
  try {
    await testVirtualizationPlatform(row.id)
    ElMessage.success('连接测试成功')
  } catch (error: any) {
    ElMessage.error(error.message || '连接测试失败')
  } finally {
    testingPlatformId.value = null
  }
}

const handleSyncPlatform = async (row: any) => {
  syncingPlatformId.value = row.id
  try {
    await syncVirtualizationPlatform(row.id)
    ElMessage.success('同步完成')
    await Promise.all([loadPlatforms(), loadPlatformCatalog()])
    await loadTopology()
    await loadGuests()
    if (syncJobDialogVisible.value && syncJobPlatformId.value === row.id) {
      await loadSyncJobs()
    }
  } catch (error: any) {
    ElMessage.error(error.message || '同步失败')
  } finally {
    syncingPlatformId.value = null
  }
}

const openSyncJobDialog = async (row: any) => {
  syncJobPlatformId.value = row.id
  syncJobDialogVisible.value = true
  await loadSyncJobs()
}

const loadSyncJobs = async () => {
  if (!syncJobPlatformId.value) return
  syncJobLoading.value = true
  try {
    const res: any = await listVirtualizationSyncJobs(syncJobPlatformId.value, { page: 1, pageSize: 50 })
    syncJobList.value = res.list || []
  } finally {
    syncJobLoading.value = false
  }
}

const loadTopology = async () => {
  topologyLoading.value = true
  try {
    const res: any = await getVirtualizationTopology(topologyPlatformId.value || undefined)
    topologyPlatforms.value = res.platforms || []
    if (trendScopeType.value === 'cluster' && trendClusterId.value) {
      const exists = trendClusterOptions.value.some((item: any) => item.id === trendClusterId.value)
      if (!exists) {
        trendClusterId.value = trendClusterOptions.value[0]?.id || null
      }
    }
    await loadPlatformTrend()
  } finally {
    topologyLoading.value = false
  }
}

const loadPlatformTrend = async () => {
  const platformID = currentTrendPlatformId.value
  if (!platformID) {
    platformTrend.value = null
    return
  }

  platformTrendLoading.value = true
  try {
    if (trendScopeType.value === 'cluster' && trendClusterId.value) {
      platformTrend.value = await getVirtualizationClusterTrend(trendClusterId.value, {
        range: platformTrendRange.value
      })
    } else {
      platformTrend.value = await getVirtualizationPlatformTrend(platformID, {
        range: platformTrendRange.value
      })
    }
  } catch (error: any) {
    platformTrend.value = null
    ElMessage.warning(error.message || '获取平台趋势失败')
  } finally {
    platformTrendLoading.value = false
  }
}

const handlePlatformTrendRangeChange = async (range: '24h' | '7d' | '15d') => {
  platformTrendRange.value = range
  await loadPlatformTrend()
}

const handleTrendMetricChange = (values: Array<typeof trendMetricOptions[number]['value']>) => {
  selectedTrendMetrics.value = values
}

const selectTrendCluster = async (platformId: number, clusterId: number) => {
  trendScopeType.value = 'cluster'
  trendClusterId.value = clusterId
  if (topologyPlatformId.value !== platformId) {
    topologyPlatformId.value = platformId
    return
  }
  await loadPlatformTrend()
}

const loadGuests = async () => {
  if (!guestQuery.platformId) {
    guestList.value = []
    guestTotal.value = 0
    return
  }
  guestLoading.value = true
  try {
    const res: any = await listVirtualizationGuests({
      page: guestQuery.page,
      pageSize: guestQuery.pageSize,
      keyword: guestQuery.keyword || undefined,
      platformId: guestQuery.platformId,
      clusterId: guestQuery.clusterId,
      powerState: guestQuery.powerState || undefined,
      bound: guestQuery.bound
    })
    guestList.value = res.list || []
    guestTotal.value = res.total || 0
  } finally {
    guestLoading.value = false
  }
}

const resetGuestQuery = () => {
  guestQuery.page = 1
  guestQuery.pageSize = 10
  guestQuery.keyword = ''
  guestQuery.clusterId = undefined
  guestQuery.powerState = ''
  guestQuery.bound = 'all'
  loadGuests()
}

const selectGuestPlatform = (platformId: number) => {
  if (guestQuery.platformId === platformId) return
  guestQuery.platformId = platformId
  guestQuery.page = 1
}

const refreshGuestWorkspace = async () => {
  await loadPlatformCatalog()
  await loadGuests()
}

const loadHostOptions = async () => {
  const res: any = await getHostList({
    page: 1,
    pageSize: 1000,
    keyword: ''
  })
  hostOptions.value = res.list || []
}

const openOnboardDialog = async (row: any) => {
  currentGuest.value = row
  onboardForm.assetHostId = undefined
  onboardForm.bindingType = 'manual'
  onboardForm.bindingNote = ''
  precheckResult.value = null
  onboardDialogVisible.value = true
  if (hostOptions.value.length === 0) {
    await loadHostOptions()
  }
  await loadGuestPrecheck()
}

const loadGuestPrecheck = async () => {
  if (!currentGuest.value?.id) return
  precheckLoading.value = true
  try {
    precheckResult.value = await precheckVirtualizationGuestOnboard(currentGuest.value.id, {
      assetHostId: onboardForm.assetHostId
    })
  } finally {
    precheckLoading.value = false
  }
}

const useSuggestedHost = () => {
  const suggestedID = precheckResult.value?.suggestedHost?.id
  if (!suggestedID) return
  onboardForm.assetHostId = suggestedID
}

const jumpToHostDetail = (row: any) => {
  if (!row?.assetHostId) return
  router.push({
    path: '/asset/hosts',
    query: {
      hostId: String(row.assetHostId),
      from: 'virtualization'
    }
  })
}

const handleOnboardGuest = async () => {
  if (!currentGuest.value?.id) return
  if (!onboardForm.assetHostId) {
    ElMessage.warning('请选择目标资产主机')
    return
  }
  onboardingGuest.value = true
  try {
    await onboardVirtualizationGuest(currentGuest.value.id, {
      assetHostId: onboardForm.assetHostId,
      bindingType: onboardForm.bindingType,
      bindingNote: onboardForm.bindingNote
    })
    ElMessage.success('纳管成功')
    onboardDialogVisible.value = false
    await loadGuests()
  } catch (error: any) {
    ElMessage.error(error.message || '纳管失败')
  } finally {
    onboardingGuest.value = false
  }
}

const handleUnbindGuest = async (row: any) => {
  ElMessageBox.confirm(`确定解绑虚机「${row.name}」吗？`, '提示', { type: 'warning' })
    .then(async () => {
      await unbindVirtualizationGuest(row.id, {})
      ElMessage.success('解绑成功')
      await loadGuests()
    })
}

const resetPowerForm = () => {
  powerGuest.value = null
  powerForm.action = 'power_on'
  powerForm.reason = ''
  powerForm.confirmText = ''
}

const openPowerDialog = (row: any, action: 'power_on' | 'power_off' | 'reboot') => {
  powerGuest.value = row
  powerForm.action = action
  powerForm.reason = ''
  powerForm.confirmText = ''
  powerDialogVisible.value = true
}

const openConsolePopup = () => {
  const popup = window.open('about:blank', '_blank')
  if (!popup) {
    return null
  }

  try {
    popup.document.write(`
      <!doctype html>
      <html lang="zh-CN">
        <head>
          <meta charset="utf-8" />
          <title>正在打开控制台</title>
          <style>
            body {
              margin: 0;
              min-height: 100vh;
              display: flex;
              align-items: center;
              justify-content: center;
              font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
              color: #475569;
              background: #f8fafc;
            }
          </style>
        </head>
        <body>正在打开虚机控制台...</body>
      </html>
    `)
    popup.document.close()
    popup.opener = null
  } catch {
    // Ignore browser restrictions on the temporary about:blank page.
  }

  return popup
}

const openGuestConsole = async (row: any) => {
  if (!row?.id) return

  const popup = openConsolePopup()
  consoleActionLoadingKey.value = String(row.id)
  try {
    const res: any = await createVirtualizationGuestConsoleLink(row.id)
    const targetURL = res?.url
    if (!targetURL) {
      throw new Error('未获取到控制台链接')
    }
    if (popup && !popup.closed) {
      popup.location.replace(targetURL)
    } else {
      const fallbackPopup = window.open(targetURL, '_blank')
      if (!fallbackPopup) {
        throw new Error('浏览器阻止了控制台窗口，请允许弹窗后重试')
      }
      try {
        fallbackPopup.opener = null
      } catch {
        // Ignore browser restrictions on the fallback console window.
      }
    }
    if (res?.message) {
      ElMessage.info(res.message)
    }
    await loadActionLogs()
  } catch (error: any) {
    if (popup && !popup.closed) {
      popup.close()
    }
    ElMessage.error(error.message || '打开控制台失败')
  } finally {
    consoleActionLoadingKey.value = ''
  }
}

const submitPowerAction = async () => {
  if (!powerGuest.value?.id) return
  if (!powerForm.reason.trim()) {
    ElMessage.warning('请输入操作原因')
    return
  }
  if (powerForm.confirmText.trim() !== powerGuest.value.name) {
    ElMessage.warning('二次确认未通过，请输入正确的虚机名称')
    return
  }

  powerSubmitting.value = true
  powerActionLoadingKey.value = `${powerGuest.value.id}:${powerForm.action}`
  try {
    const res: any = await powerVirtualizationGuest(powerGuest.value.id, {
      action: powerForm.action,
      reason: powerForm.reason.trim(),
      confirmText: powerForm.confirmText.trim()
    })
    ElMessage.success(res?.message || `${selectedPowerActionText.value}操作已提交`)
    powerDialogVisible.value = false
    await loadGuests()
  } catch (error: any) {
    ElMessage.error(error.message || `${selectedPowerActionText.value}失败`)
  } finally {
    powerSubmitting.value = false
    powerActionLoadingKey.value = ''
    await loadActionLogs()
  }
}

const resetSnapshotCreateForm = () => {
  snapshotCreateForm.name = ''
  snapshotCreateForm.description = ''
  snapshotCreateForm.includeMemory = false
  snapshotCreateForm.quiesce = false
  snapshotCreateForm.reason = ''
}

const resetSnapshotActionForm = () => {
  snapshotActionForm.action = 'rollback'
  snapshotActionForm.snapshotName = ''
  snapshotActionForm.reason = ''
  snapshotActionForm.confirmText = ''
  currentSnapshotRecord.value = null
}

const suggestSnapshotName = () => {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `snap-${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
}

const loadSnapshots = async () => {
  if (!snapshotGuest.value?.id) return
  snapshotLoading.value = true
  try {
    const res: any = await listVirtualizationGuestSnapshots(snapshotGuest.value.id)
    snapshotList.value = res.list || []
  } finally {
    snapshotLoading.value = false
  }
}

const openSnapshotDialog = async (row: any) => {
  snapshotGuest.value = row
  snapshotList.value = []
  resetSnapshotCreateForm()
  snapshotCreateForm.name = suggestSnapshotName()
  snapshotDialogVisible.value = true
  await loadSnapshots()
}

const submitSnapshotCreate = async () => {
  if (!snapshotGuest.value?.id) return
  if (!snapshotCreateForm.name.trim()) {
    ElMessage.warning('请输入快照名称')
    return
  }
  if (!snapshotCreateForm.reason.trim()) {
    ElMessage.warning('请输入操作原因')
    return
  }

  snapshotSubmitting.value = true
  try {
    await createVirtualizationGuestSnapshot(snapshotGuest.value.id, {
      name: snapshotCreateForm.name.trim(),
      description: snapshotCreateForm.description.trim() || undefined,
      includeMemory: snapshotCreateForm.includeMemory,
      quiesce: snapshotCreateForm.quiesce,
      reason: snapshotCreateForm.reason.trim()
    })
    ElMessage.success('快照创建成功')
    resetSnapshotCreateForm()
    snapshotCreateForm.name = suggestSnapshotName()
    await loadSnapshots()
  } catch (error: any) {
    ElMessage.error(error.message || '创建快照失败')
  } finally {
    snapshotSubmitting.value = false
    await loadActionLogs()
  }
}

const handleSnapshotDialogClose = () => {
  snapshotGuest.value = null
  snapshotList.value = []
  resetSnapshotCreateForm()
}

const openSnapshotActionDialog = (row: any, action: 'rollback' | 'delete') => {
  currentSnapshotRecord.value = row
  snapshotActionForm.action = action
  snapshotActionForm.snapshotName = row.name
  snapshotActionForm.reason = ''
  snapshotActionForm.confirmText = ''
  snapshotActionDialogVisible.value = true
}

const handleSnapshotActionDialogClose = () => {
  resetSnapshotActionForm()
}

const submitSnapshotAction = async () => {
  if (!snapshotGuest.value?.id || !currentSnapshotRecord.value?.name) return
  if (!snapshotActionForm.reason.trim()) {
    ElMessage.warning('请输入操作原因')
    return
  }
  if (snapshotActionForm.confirmText.trim() !== currentSnapshotRecord.value.name) {
    ElMessage.warning('二次确认未通过，请输入正确的快照名称')
    return
  }

  snapshotActionSubmitting.value = true
  try {
    const payload = {
      snapshotName: currentSnapshotRecord.value.name,
      reason: snapshotActionForm.reason.trim(),
      confirmText: snapshotActionForm.confirmText.trim()
    }
    if (snapshotActionForm.action === 'delete') {
      await deleteVirtualizationGuestSnapshot(snapshotGuest.value.id, payload)
      ElMessage.success('快照删除成功')
    } else {
      await rollbackVirtualizationGuestSnapshot(snapshotGuest.value.id, payload)
      ElMessage.success('快照回滚成功')
    }
    snapshotActionDialogVisible.value = false
    await Promise.all([loadSnapshots(), loadGuests()])
  } catch (error: any) {
    ElMessage.error(error.message || `${selectedSnapshotActionText.value}失败`)
  } finally {
    snapshotActionSubmitting.value = false
    await loadActionLogs()
  }
}

const loadActionLogs = async () => {
  actionLogLoading.value = true
  try {
    const res: any = await listVirtualizationActionLogs({
      page: actionLogQuery.page,
      pageSize: actionLogQuery.pageSize,
      platformId: actionLogQuery.platformId,
      action: actionLogQuery.action || undefined,
      status: actionLogQuery.status || undefined,
      keyword: actionLogQuery.keyword || undefined
    })
    actionLogList.value = res.list || []
    actionLogTotal.value = res.total || 0
  } finally {
    actionLogLoading.value = false
  }
}

const handleActionLogSearch = () => {
  actionLogQuery.page = 1
  loadActionLogs()
}

const resetActionLogQuery = () => {
  actionLogQuery.page = 1
  actionLogQuery.pageSize = 10
  actionLogQuery.platformId = undefined
  actionLogQuery.action = ''
  actionLogQuery.status = ''
  actionLogQuery.keyword = ''
  loadActionLogs()
}

const precheckRiskType = (riskLevel?: string) => {
  if (riskLevel === 'high') return 'danger'
  if (riskLevel === 'medium') return 'warning'
  if (riskLevel === 'low') return 'info'
  return 'success'
}

const precheckRiskText = (riskLevel?: string) => {
  if (riskLevel === 'high') return '高风险'
  if (riskLevel === 'medium') return '中风险'
  if (riskLevel === 'low') return '低风险'
  return '无风险'
}

const precheckPolicyText = (policy?: string) => {
  if (policy === 'warn') return '宽松（仅提示）'
  return '严格（高风险拦截）'
}

const syncStatusType = (status: string) => {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'info'
}

const powerStatusType = (status: string) => {
  if (status === 'powered_on') return 'success'
  if (status === 'suspended') return 'warning'
  return 'info'
}

const powerStatusText = (status: string) => {
  if (status === 'powered_on') return '开机'
  if (status === 'suspended') return '挂起'
  if (status === 'powered_off') return '关机'
  return '未知'
}

const runtimeStatusType = (status?: string) => {
  if (status === 'online') return 'success'
  if (status === 'offline') return 'danger'
  if (status === 'not_configured') return 'info'
  return 'warning'
}

const snapshotPowerStateText = (status?: string) => {
  if (!status) return '-'
  return powerStatusText(status)
}

const auditRiskType = (riskLevel?: string) => {
  if (riskLevel === 'high') return 'danger'
  if (riskLevel === 'medium') return 'warning'
  if (riskLevel === 'low') return 'info'
  return 'info'
}

const auditRiskText = (riskLevel?: string) => {
  if (riskLevel === 'high') return '高风险'
  if (riskLevel === 'medium') return '中风险'
  if (riskLevel === 'low') return '低风险'
  return '未知'
}

const auditStatusType = (status?: string) => {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'denied') return 'warning'
  return 'info'
}

const auditStatusText = (status?: string) => {
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'denied') return '已拒绝'
  return '处理中'
}

watch(
  () => [guestQuery.platformId, guestQuery.bound, guestQuery.powerState],
  () => {
    guestQuery.page = 1
    loadGuests()
  }
)

watch(topologyPlatformId, () => {
  loadTopology()
})

watch(trendScopeType, async (scopeType) => {
  if (scopeType === 'cluster' && !trendClusterId.value) {
    trendClusterId.value = trendClusterOptions.value[0]?.id || null
    if (!trendClusterId.value) {
      return
    }
  }
  await loadPlatformTrend()
})

watch(trendClusterId, () => {
  if (trendScopeType.value !== 'cluster') return
  loadPlatformTrend()
})

watch(
  () => onboardForm.assetHostId,
  () => {
    if (!onboardDialogVisible.value) return
    loadGuestPrecheck()
  }
)

watch(
  () => [actionLogQuery.platformId, actionLogQuery.action, actionLogQuery.status],
  () => {
    actionLogQuery.page = 1
    if (activeTab.value === 'audit') {
      loadActionLogs()
    }
  }
)

watch(activeTab, (tab) => {
  if (tab === 'audit') {
    loadActionLogs()
  }
})

onMounted(async () => {
  await Promise.all([loadPlatforms(), loadPlatformCatalog(), loadVirtualizationSettings(), loadActionLogs()])
  await Promise.all([loadTopology(), loadGuests()])
})
</script>

<style scoped>
.virtualization-page {
  padding: 20px;
  background: #f6f8fb;
  min-height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 18px;
  gap: 16px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.policy-select {
  width: 220px;
}

.write-switch-group {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
}

.write-switch-label {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.page-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, #111827, #374151);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  box-shadow: 0 8px 18px rgba(17, 24, 39, 0.25);
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #111827;
}

.page-subtitle {
  margin: 6px 0 0;
  color: #6b7280;
}

.black-button {
  background: #111827;
  color: #fff;
  border: 1px solid #111827;
}

.black-button:hover {
  background: #1f2937;
  border-color: #1f2937;
  color: #fff;
}

.main-tabs {
  background: #fff;
  border-radius: 14px;
  padding: 14px 18px;
  box-shadow: 0 10px 30px rgba(17, 24, 39, 0.06);
}

.search-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
  gap: 12px;
  flex-wrap: wrap;
}

.search-inputs {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.search-input {
  width: 230px;
}

.audit-search-input {
  width: 300px;
}

.reset-btn {
  background: #f3f4f6;
  border-color: #e5e7eb;
}

.table-wrapper {
  background: #fff;
  border-radius: 12px;
}

.modern-table {
  width: 100%;
}

.pagination-container {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
  padding-bottom: 4px;
}

.topology-panel {
  background: radial-gradient(1200px 400px at 10% 0%, rgba(15, 23, 42, 0.03), transparent), #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  min-height: 380px;
  padding: 14px;
}

.topology-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.topology-trend-panel {
  margin-bottom: 14px;
}

.trend-scope-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  background: rgba(255, 255, 255, 0.86);
}

.trend-scope-left,
.trend-scope-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.trend-scope-label {
  color: #475569;
  font-size: 12px;
  font-weight: 600;
}

.summary-card {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 10px 12px;
  background: #fff;
}

.summary-label {
  color: #64748b;
  font-size: 12px;
}

.summary-value {
  margin-top: 6px;
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
}

.platform-card-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 14px;
}

.platform-card {
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  padding: 12px;
}

.platform-card-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.platform-name {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.platform-meta {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.platform-sync-time {
  color: #64748b;
  font-size: 12px;
  margin-top: 3px;
}

.cluster-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 10px;
}

.cluster-card {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 10px;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  cursor: pointer;
  transition: all 0.18s ease;
}

.cluster-card:hover {
  border-color: #cbd5e1;
  box-shadow: 0 8px 22px rgba(15, 23, 42, 0.06);
}

.cluster-card-active {
  border-color: #2563eb;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.14);
}

.cluster-title-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 8px;
}

.cluster-title {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
}

.cluster-metrics {
  display: flex;
  gap: 8px;
  color: #64748b;
  font-size: 12px;
}

.host-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.empty-host {
  color: #94a3b8;
  font-size: 12px;
}

.host-chip {
  display: grid;
  grid-template-columns: 8px minmax(120px, 1fr) minmax(70px, auto) minmax(60px, auto);
  align-items: center;
  gap: 8px;
  border-radius: 8px;
  padding: 6px 8px;
  border: 1px solid #e2e8f0;
  background: #fff;
}

.host-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #94a3b8;
}

.host-name {
  color: #0f172a;
  font-size: 12px;
  font-weight: 600;
}

.host-ip {
  color: #64748b;
  font-size: 12px;
}

.host-vm {
  color: #334155;
  font-size: 12px;
  text-align: right;
}

.host-online .host-dot {
  background: #16a34a;
}

.host-maintenance .host-dot {
  background: #d97706;
}

.host-offline .host-dot,
.host-unknown .host-dot {
  background: #9ca3af;
}

.binding-host {
  margin-top: 6px;
  font-size: 12px;
  color: #6b7280;
}

.guest-platform-workspace {
  margin-bottom: 14px;
  padding: 14px;
  border-radius: 14px;
  border: 1px solid #e5e7eb;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
}

.guest-platform-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.workspace-title {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.workspace-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.guest-platform-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.guest-platform-card {
  text-align: left;
  border-radius: 12px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  background: #fff;
  cursor: pointer;
  transition: all 0.18s ease;
}

.guest-platform-card:hover {
  border-color: #cbd5e1;
  transform: translateY(-1px);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.guest-platform-card-active {
  border-color: #111827;
  background: linear-gradient(135deg, rgba(17, 24, 39, 0.04), rgba(37, 99, 235, 0.05));
  box-shadow: 0 12px 28px rgba(17, 24, 39, 0.08);
}

.guest-platform-card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
}

.guest-platform-card-name {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
}

.guest-platform-card-meta {
  margin-top: 8px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.guest-empty-state {
  border: 1px dashed #d1d5db;
  border-radius: 14px;
  background: #fff;
  padding: 30px 12px;
}

.guest-write-alert {
  margin-bottom: 14px;
}

.runtime-source {
  margin-top: 6px;
  font-size: 11px;
  color: #94a3b8;
}

.audit-target {
  color: #0f172a;
  font-weight: 600;
}

.audit-subtext {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 12px;
}

.audit-multiline {
  color: #334155;
  line-height: 1.5;
  white-space: normal;
  word-break: break-word;
}

.switch-text {
  margin-left: 10px;
  color: #6b7280;
  font-size: 12px;
}

.field-tip {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

.power-dialog-intro {
  margin-bottom: 14px;
}

.snapshot-dialog-layout {
  display: grid;
  grid-template-columns: 340px minmax(0, 1fr);
  gap: 16px;
}

.snapshot-create-panel,
.snapshot-list-panel {
  border: 1px solid #e5e7eb;
  border-radius: 14px;
  background: #f8fafc;
  padding: 14px;
}

.snapshot-panel-title {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.snapshot-panel-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.snapshot-create-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.snapshot-list-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.snapshot-table {
  background: #fff;
  border-radius: 12px;
}

.snapshot-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.snapshot-feature-tags {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 6px;
}

.precheck-panel {
  margin-top: 6px;
  border-radius: 10px;
  border: 1px solid #e5e7eb;
  background: #f8fafc;
  padding: 10px;
}

.precheck-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  color: #111827;
  margin-bottom: 8px;
}

.precheck-suggest {
  font-size: 12px;
  color: #475569;
  margin-bottom: 8px;
}

.precheck-check-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.precheck-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.4;
  background: #fff;
  border: 1px solid #e5e7eb;
  color: #334155;
}

.precheck-icon {
  flex-shrink: 0;
}

.risk-high {
  border-color: #fecaca;
  color: #b91c1c;
}

.risk-medium {
  border-color: #fde68a;
  color: #b45309;
}

.risk-low {
  border-color: #bfdbfe;
  color: #1d4ed8;
}

@media (max-width: 900px) {
  .virtualization-page {
    padding: 12px;
  }

  .topology-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .search-input {
    width: 100%;
  }

  .guest-platform-header {
    flex-direction: column;
  }

  .snapshot-dialog-layout {
    grid-template-columns: 1fr;
  }

  .audit-search-input {
    width: 100%;
  }
}
</style>
