<template>
  <div class="database-page">
    <div class="page-header">
      <div class="page-title-group">
        <div class="page-title-icon">
          <el-icon><DataLine /></el-icon>
        </div>
        <div>
          <h2 class="page-title">数据库管理</h2>
          <p class="page-subtitle">统一纳管数据库实例，支持结构发现、只读查询和审计</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button class="black-button" @click="openInstanceDialog()">
          <el-icon style="margin-right: 6px;"><Plus /></el-icon>
          新增实例
        </el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="main-tabs">
      <el-tab-pane label="实例管理" name="instances">
        <div class="search-bar">
          <div class="search-inputs">
            <el-input
              v-model="query.keyword"
              placeholder="搜索实例名称、地址、默认库、业务系统..."
              clearable
              class="search-input"
              @keyup.enter="loadInstances"
              @clear="loadInstances"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>

            <el-select v-model="query.dbType" placeholder="数据库类型" clearable class="search-input" @change="loadInstances">
              <el-option v-for="item in supportedTypes" :key="item.type" :label="item.name" :value="item.type" />
            </el-select>

            <el-select v-model="query.status" placeholder="状态" clearable class="search-input" @change="loadInstances">
              <el-option label="启用" value="enabled" />
              <el-option label="禁用" value="disabled" />
            </el-select>

            <el-select v-model="query.environment" placeholder="环境" clearable class="search-input" @change="loadInstances">
              <el-option label="生产" value="prod" />
              <el-option label="预发" value="staging" />
              <el-option label="测试" value="test" />
              <el-option label="开发" value="dev" />
            </el-select>
          </div>
          <div class="search-actions">
            <el-button class="reset-btn" @click="resetQuery">
              <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
              重置
            </el-button>
          </div>
        </div>

        <div class="table-wrapper">
          <el-table
            :data="instances"
            v-loading="loading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="实例名称" min-width="170">
              <template #default="{ row }">
                <div class="instance-name">
                  <span>{{ row.name }}</span>
                  <el-tag v-if="row.environment" size="small" type="info">{{ environmentText(row.environment) }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="130" align="center">
              <template #default="{ row }">
                <el-tag :type="dbTypeTag(row.dbType)">
                  {{ row.dbTypeText || row.dbType }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="地址" min-width="220">
              <template #default="{ row }">
                <span class="mono">{{ row.endpoint || `${row.host}:${row.port}` }}</span>
              </template>
            </el-table-column>
            <el-table-column label="默认库" min-width="130">
              <template #default="{ row }">
                <span>{{ row.defaultDatabase || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="业务系统" min-width="140">
              <template #default="{ row }">
                <span>{{ row.businessSystem || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="负责人" min-width="110">
              <template #default="{ row }">
                <span>{{ row.owner || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'enabled' ? 'success' : 'info'">
                  {{ row.statusText || (row.status === 'enabled' ? '启用' : '禁用') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="最近测试" width="170" align="center">
              <template #default="{ row }">
                {{ row.lastTestAt || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="最近同步" width="170" align="center">
              <template #default="{ row }">
                {{ row.lastSyncAt || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="容量增长" width="170" align="center">
              <template #default="{ row }">
                <div v-if="row.capacitySizeText" class="capacity-summary-cell">
                  <span>{{ row.capacitySizeText }}</span>
                  <el-tag size="small" :type="Number(row.capacityGrowthPercent || 0) >= 50 ? 'warning' : 'info'">
                    {{ row.capacityGrowthText || '0 B' }}
                  </el-tag>
                </div>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="260" align="center" fixed="right">
              <template #default="{ row }">
                <el-tooltip :content="canUseDatabaseFeature(row, DATABASE_PERMISSION.MANAGE, 'testEnabled') ? '连接测试' : '无连接测试权限或该类型暂未接入'" placement="top">
                  <el-button link type="primary" :loading="testingId === row.id" :disabled="!canUseDatabaseFeature(row, DATABASE_PERMISSION.MANAGE, 'testEnabled')" @click="handleTest(row)">
                    <el-icon><Connection /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="canUseDatabaseFeature(row, DATABASE_PERMISSION.MANAGE, 'metadataEnabled') ? '同步结构' : '无同步权限或该类型暂未接入'" placement="top">
                  <el-button link type="success" :loading="syncingId === row.id" :disabled="!canUseDatabaseFeature(row, DATABASE_PERMISSION.MANAGE, 'metadataEnabled')" @click="handleSync(row)">
                    <el-icon><Refresh /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip :content="row.status === 'enabled' ? '禁用' : '启用'" placement="top">
                  <el-button link :type="row.status === 'enabled' ? 'warning' : 'success'" :disabled="!hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)" @click="toggleInstanceStatus(row)">
                    <el-icon><Switch /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="编辑" placement="top">
                  <el-button link type="primary" :disabled="!hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)" @click="openInstanceDialog(row)">
                    <el-icon><Edit /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="删除" placement="top">
                  <el-button link type="danger" :disabled="!hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)" @click="handleDelete(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </el-tooltip>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="query.page"
              v-model:page-size="query.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="total"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadInstances"
              @current-change="loadInstances"
            />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane v-if="canManageInstancePermissions" label="实例权限" name="permissions">
        <div class="backup-card">
          <el-alert
            :title="permissionModeAlertTitle"
            :type="permissionMode === 'whitelist' ? 'success' : 'warning'"
            :closable="false"
            show-icon
            class="permission-mode-alert"
          />
          <div class="backup-toolbar">
            <div class="backup-toolbar-group">
              <el-input
                v-model="permissionQuery.keyword"
                placeholder="搜索角色或实例"
                clearable
                class="audit-search-input"
                @keyup.enter="loadInstancePermissions"
                @clear="loadInstancePermissions"
              >
                <template #prefix>
                  <el-icon><Search /></el-icon>
                </template>
              </el-input>
              <el-select v-model="permissionQuery.roleId" placeholder="角色" clearable filterable class="audit-select" @change="loadInstancePermissions">
                <el-option v-for="role in roleOptions" :key="role.id" :label="role.name" :value="role.id" />
              </el-select>
              <el-select v-model="permissionQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadInstancePermissions">
                <el-option v-for="item in instanceOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </div>
            <el-button v-if="canManageInstancePermissions" type="primary" @click="openPermissionDialog()">
              <el-icon style="margin-right: 6px;"><Plus /></el-icon>
              添加权限
            </el-button>
          </div>

          <el-table
            :data="permissionRows"
            v-loading="permissionLoading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="角色" min-width="180">
              <template #default="{ row }">
                <div class="instance-name">
                  <span>{{ row.roleName || '-' }}</span>
                  <el-tag v-if="row.roleCode" size="small" type="info">{{ row.roleCode }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="数据库实例" min-width="200">
              <template #default="{ row }">{{ row.instanceName || '-' }}</template>
            </el-table-column>
            <el-table-column label="权限" min-width="360">
              <template #default="{ row }">
                <div class="permission-tag-list">
                  <el-tag
                    v-for="item in databasePermissionOptions.filter(option => hasPermissionMask(row.permissions, option.value))"
                    :key="item.value"
                    size="small"
                    type="primary"
                  >
                    {{ item.label }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" width="170" align="center">
              <template #default="{ row }">{{ row.updatedAt || '-' }}</template>
            </el-table-column>
            <el-table-column v-if="canManageInstancePermissions" label="操作" width="120" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openPermissionDialog(row)">编辑</el-button>
                <el-button link type="danger" @click="handleDeletePermission(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="permissionQuery.page"
              v-model:page-size="permissionQuery.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="permissionTotal"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadInstancePermissions"
              @current-change="loadInstancePermissions"
            />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="结构浏览" name="schemas">
        <div class="metadata-browser">
          <div class="metadata-toolbar">
            <div class="metadata-selector">
              <span class="toolbar-label">数据库实例</span>
              <el-select
                v-model="metadataInstanceId"
                placeholder="请选择实例"
                filterable
                class="metadata-instance-select"
                @change="handleMetadataInstanceChange"
              >
                <el-option
                  v-for="item in metadataInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                  :value="item.id"
                />
              </el-select>
              <el-tag v-if="currentMetadataInstance?.lastSyncAt" type="success">
                最近同步：{{ currentMetadataInstance.lastSyncAt }}
              </el-tag>
              <el-tag v-else type="info">未同步</el-tag>
            </div>
            <el-button
              type="primary"
              :disabled="!metadataInstanceId || !canUseDatabaseFeature(currentMetadataInstance, DATABASE_PERMISSION.MANAGE, 'metadataEnabled')"
              :loading="syncingId === metadataInstanceId"
              @click="handleSync()"
            >
              <el-icon style="margin-right: 6px;"><Refresh /></el-icon>
              同步元数据
            </el-button>
          </div>

          <div v-if="!metadataInstanceId" class="metadata-empty">
            <el-empty description="请先选择一个数据库实例" :image-size="82" />
          </div>
          <div v-else class="metadata-content">
            <div class="schema-panel">
              <div class="panel-title">
                <el-icon><Coin /></el-icon>
                <span>{{ isRedisMetadataInstance ? '逻辑 DB' : '库 / Schema' }}</span>
              </div>
              <el-tree
                v-loading="schemasLoading"
                :data="schemaTree"
                node-key="id"
                default-expand-all
                highlight-current
                class="schema-tree"
                @node-click="handleSchemaClick"
              >
                <template #default="{ data }">
                  <span class="schema-node">
                    <span class="schema-node-name">{{ data.label }}</span>
                    <el-tag size="small" type="info">{{ data.tableCount }}</el-tag>
                  </span>
                </template>
              </el-tree>
              <el-empty v-if="!schemasLoading && schemas.length === 0" description="暂无元数据，请先同步" :image-size="64" />
            </div>

            <div class="table-panel">
              <div class="panel-title">
                <el-icon><Grid /></el-icon>
                <span>{{ isRedisMetadataInstance ? 'Key 列表' : '表 / 视图' }}</span>
              </div>
              <el-table
                :data="tables"
                v-loading="tablesLoading"
                stripe
                height="520"
                highlight-current-row
                class="metadata-table"
                @row-click="handleTableClick"
                @row-contextmenu="handleTableContextMenu"
              >
                <template v-if="isRedisMetadataInstance">
                  <el-table-column label="Key" min-width="240" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="table-name-cell">
                        <span>{{ row.tableName }}</span>
                        <el-tag v-if="row.tableType" size="small" type="success">{{ row.tableType }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="TTL" width="120">
                    <template #default="{ row }">{{ row.ttlText || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="内存" width="110" align="right">
                    <template #default="{ row }">{{ formatBytes(row.dataSizeBytes) }}</template>
                  </el-table-column>
                  <el-table-column label="长度" width="100" align="right">
                    <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
                  </el-table-column>
                  <el-table-column label="节点" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.nodeAddress || '-' }}</template>
                  </el-table-column>
                </template>
                <template v-else>
                <el-table-column label="名称" min-width="190">
                  <template #default="{ row }">
                    <div class="table-name-cell">
                      <span>{{ row.tableName }}</span>
                      <el-tag v-if="row.tableType" size="small" type="info">{{ row.tableType }}</el-tag>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="引擎" width="100">
                  <template #default="{ row }">{{ row.engine || '-' }}</template>
                </el-table-column>
                <el-table-column label="行数估算" width="110" align="right">
                  <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
                </el-table-column>
                <el-table-column label="容量" width="110" align="right">
                  <template #default="{ row }">{{ formatBytes((row.dataSizeBytes || 0) + (row.indexSizeBytes || 0)) }}</template>
                </el-table-column>
                </template>
              </el-table>
            </div>

            <div class="detail-panel">
              <div class="panel-title detail-panel-title">
                <div class="detail-panel-heading">
                  <el-icon><Tickets /></el-icon>
                  <span>{{ selectedTable?.tableName || (isRedisMetadataInstance ? 'Key 属性' : '字段 / 索引') }}</span>
                </div>
                <div v-if="selectedTable && !isRedisMetadataInstance" class="detail-panel-actions">
                  <el-button link type="primary" :loading="ddlLoading" @click="handlePreviewDDL">
                    查看 DDL
                  </el-button>
                  <el-button link type="primary" :loading="dictionaryExporting" :disabled="!hasDatabasePermission(currentMetadataInstance, DATABASE_PERMISSION.EXPORT)" @click="handleExportDictionary">
                    <el-icon style="margin-right: 4px;"><Download /></el-icon>
                    导出字典
                  </el-button>
                </div>
              </div>
              <el-empty v-if="!selectedTable" :description="isRedisMetadataInstance ? '请选择一个 Key 查看属性' : '请选择一张表查看字段和索引'" :image-size="72" />
              <div v-else>
                <div class="table-overview-cards">
                  <div class="table-overview-card">
                    <span class="summary-label">对象类型</span>
                    <strong>{{ selectedTable.tableType || '-' }}</strong>
                  </div>
                  <div class="table-overview-card">
                    <span class="summary-label">{{ isRedisMetadataInstance ? 'TTL' : '行数估算' }}</span>
                    <strong>{{ isRedisMetadataInstance ? (selectedTable.ttlText || '-') : formatNumber(selectedTable.rowCount) }}</strong>
                  </div>
                  <div class="table-overview-card">
                    <span class="summary-label">{{ isRedisMetadataInstance ? '内存占用' : '数据容量' }}</span>
                    <strong>{{ formatBytes(selectedTable.dataSizeBytes) }}</strong>
                  </div>
                  <div class="table-overview-card">
                    <span class="summary-label">{{ isRedisMetadataInstance ? '编码 / 节点' : '索引容量' }}</span>
                    <strong>{{ isRedisMetadataInstance ? (selectedTable.encoding || selectedTable.nodeAddress || '-') : formatBytes(selectedTable.indexSizeBytes) }}</strong>
                  </div>
                </div>
                <div v-if="isRedisMetadataInstance">
                  <el-table :data="columns" v-loading="detailsLoading" stripe height="470" class="metadata-table">
                    <el-table-column label="属性" prop="columnName" width="140" />
                    <el-table-column label="值" min-width="220" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.defaultValue || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="说明" min-width="180" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.comment || '-' }}</template>
                    </el-table-column>
                  </el-table>
                </div>
                <el-tabs v-else v-model="detailTab" class="detail-tabs">
                  <el-tab-pane label="字段" name="columns">
                  <el-table :data="columns" v-loading="detailsLoading" stripe height="470" class="metadata-table">
                    <el-table-column label="#" prop="ordinalPosition" width="54" align="center" />
                    <el-table-column label="字段" min-width="150">
                      <template #default="{ row }">
                        <div class="column-name-cell">
                          <span>{{ row.columnName }}</span>
                          <el-tag v-if="row.isSensitive" size="small" type="danger">敏感</el-tag>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column label="类型" prop="dataType" min-width="150" />
                    <el-table-column label="可空" width="70" align="center">
                      <template #default="{ row }">{{ row.isNullable ? '是' : '否' }}</template>
                    </el-table-column>
                    <el-table-column label="键" width="80" align="center">
                      <template #default="{ row }">{{ row.columnKey || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="默认值" min-width="120">
                      <template #default="{ row }">{{ row.defaultValue || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="注释" min-width="160">
                      <template #default="{ row }">{{ row.comment || '-' }}</template>
                    </el-table-column>
                  </el-table>
                  </el-tab-pane>
                  <el-tab-pane label="索引" name="indexes">
                  <el-table :data="indexes" v-loading="detailsLoading" stripe height="470" class="metadata-table">
                    <el-table-column label="索引名" prop="indexName" min-width="150" />
                    <el-table-column label="字段" prop="columns" min-width="180" />
                    <el-table-column label="类型" width="110">
                      <template #default="{ row }">{{ row.indexType || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="唯一" width="80" align="center">
                      <template #default="{ row }">
                        <el-tag :type="row.isUnique ? 'success' : 'info'" size="small">{{ row.isUnique ? '是' : '否' }}</el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column label="基数" width="100" align="right">
                      <template #default="{ row }">{{ formatNumber(row.cardinality) }}</template>
                    </el-table-column>
                    <el-table-column label="注释" min-width="140">
                      <template #default="{ row }">{{ row.comment || '-' }}</template>
                    </el-table-column>
                  </el-table>
                  </el-tab-pane>
                </el-tabs>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="SQL 控制台" name="query">
        <div class="query-console">
          <el-alert
            :title="queryConsoleAlert"
            type="info"
            show-icon
            :closable="false"
            class="query-safety-alert"
          />

          <div class="query-context-panel">
            <div class="query-context-selects">
              <div class="query-field query-instance-field">
                <span class="query-field-label">实例</span>
                <el-select
                  v-model="queryInstanceId"
                  placeholder="请选择实例"
                  filterable
                  class="query-select"
                  @change="handleQueryInstanceChange"
                >
                  <el-option
                    v-for="item in queryInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                    :value="item.id"
                  />
                </el-select>
              </div>
              <div class="query-field">
                <span class="query-field-label">{{ isRedisQueryInstance ? '逻辑 DB' : 'Schema' }}</span>
                <el-select v-model="querySchemaName" :placeholder="querySchemaPlaceholder" clearable filterable class="query-select query-schema-select">
                  <el-option v-for="item in querySchemas" :key="item.schemaName" :label="item.schemaName" :value="item.schemaName" />
                </el-select>
              </div>
            </div>
            <div class="query-context-tags">
              <el-tag size="small" :type="dbTypeTag(currentQueryInstance?.dbType || '')">
                {{ currentQueryInstance?.dbTypeText || currentQueryInstance?.dbType || '未选择' }}
              </el-tag>
              <el-tag v-if="currentQueryInstance?.environment" size="small" :type="currentQueryInstance.environment === 'prod' ? 'danger' : 'info'">
                {{ environmentText(currentQueryInstance.environment) }}
              </el-tag>
              <span class="query-endpoint">
                {{ currentQueryInstance?.endpoint || (currentQueryInstance ? `${currentQueryInstance.host}:${currentQueryInstance.port}` : '-') }}
              </span>
              <el-tag v-if="canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.EXPORT, 'queryEnabled')" size="small" type="success">可导出</el-tag>
              <el-tag v-if="canUseQueryUnlimitedRows" size="small" type="danger">可不限行数</el-tag>
            </div>
          </div>

          <div class="query-workbench">
            <div class="query-editor-panel">
              <div class="query-panel-header">
                <div>
                  <span class="query-section-eyebrow">{{ isRedisQueryInstance ? 'COMMAND' : 'SQL' }}</span>
                  <h3>编辑器</h3>
                </div>
                <div class="query-mode-control">
                  <el-radio-group v-if="!isRedisQueryInstance" v-model="queryConsoleMode" size="small">
                    <el-radio-button label="read">查询</el-radio-button>
                    <el-radio-button label="write" :disabled="!canWriteCurrentQueryInstance">写入</el-radio-button>
                    <el-radio-button label="ddl" :disabled="!canDDLCurrentQueryInstance">DDL</el-radio-button>
                  </el-radio-group>
                  <el-tag :type="queryConsoleModeTagType">{{ queryConsoleModeLabel }}</el-tag>
                </div>
              </div>

              <el-alert
                v-if="isProductionQueryInstance"
                :title="queryConsoleMode === 'read' ? '当前连接生产实例，请确认查询范围和返回行数。' : '当前连接生产实例，写入或 DDL 操作必须确认影响范围、原因和回滚方案。'"
                type="error"
                show-icon
                :closable="false"
                class="query-prod-alert"
              />

              <div class="query-action-bar">
                <template v-if="isRedisQueryInstance || queryConsoleMode === 'read'">
                  <el-button type="primary" :loading="queryRunning" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="executeQuery()">
                    {{ isRedisQueryInstance ? '执行命令' : '执行查询' }}
                  </el-button>
                  <el-button :loading="queryFormatting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleFormatQuery">
                    {{ isRedisQueryInstance ? '格式化命令' : '格式化 SQL' }}
                  </el-button>
                  <el-button v-if="!isRedisQueryInstance" :loading="queryExplaining" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleExplainQuery">
                    执行计划
                  </el-button>
                  <el-button type="success" plain :loading="queryExporting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.EXPORT, 'queryEnabled')" @click="handleExportQuery">
                    <el-icon style="margin-right: 4px;"><Download /></el-icon>
                    导出结果
                  </el-button>
                </template>
                <template v-else-if="queryConsoleMode === 'write'">
                  <el-button :loading="queryFormatting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleFormatQuery">
                    格式化 SQL
                  </el-button>
                  <el-button :loading="queryExplaining" :disabled="!queryInstanceId || !canUseWriteExplainCurrentQueryInstance" @click="handleExplainQuery">
                    写 SQL 计划
                  </el-button>
                  <el-button type="warning" plain :loading="queryWriteChecking" :disabled="!queryInstanceId || !canWriteCurrentQueryInstance" @click="handleValidateWriteQuery">
                    写前检查
                  </el-button>
                  <el-button type="danger" plain :loading="queryWritePreparing" :disabled="!queryInstanceId || !canWriteCurrentQueryInstance" @click="handlePrepareWriteExecute">
                    受控写入
                  </el-button>
                </template>
                <template v-else>
                  <el-button :loading="queryFormatting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleFormatQuery">
                    格式化 SQL
                  </el-button>
                  <el-button type="warning" plain :loading="queryDDLChecking" :disabled="!queryInstanceId || !canDDLCurrentQueryInstance" @click="handleValidateDDLQuery">
                    DDL 检查
                  </el-button>
                  <el-button type="danger" :loading="queryDDLPreparing" :disabled="!queryInstanceId || !canDDLCurrentQueryInstance" @click="handlePrepareDDLExecute">
                    DDL 执行
                  </el-button>
                </template>
                <el-button type="primary" plain :loading="queryHistoryLoading" @click="openQueryHistory">
                  最近历史
                </el-button>
                <el-button plain :disabled="!querySQL.trim()" @click="saveCurrentQueryFavorite">
                  收藏 SQL
                </el-button>
                <el-button @click="resetQueryConsole">清空</el-button>
              </div>

              <el-input
                v-model="querySQL"
                type="textarea"
                :rows="13"
                class="sql-editor"
                :placeholder="queryEditorPlaceholder"
              />

              <div class="query-hints">
                <template v-if="isRedisQueryInstance">
                  <el-tag size="small">只读命令</el-tag>
                  <el-tag size="small" type="success">SCAN 自动 COUNT</el-tag>
                  <el-tag size="small" type="primary">命令格式化</el-tag>
                  <el-tag size="small" type="warning">CSV 导出</el-tag>
                  <el-tag size="small" type="danger">禁止写命令</el-tag>
                </template>
                <template v-else>
                  <el-tag size="small">只读</el-tag>
                  <el-tag size="small" type="danger">受控写入</el-tag>
                  <el-tag size="small" type="success">自动 LIMIT</el-tag>
                  <el-tag v-if="canUseQueryUnlimitedRows" size="small" type="danger">可不限行数</el-tag>
                  <el-tag size="small" type="primary">SQL 格式化</el-tag>
                  <el-tag size="small" type="warning">执行计划 / CSV 导出</el-tag>
                  <el-tag size="small" type="warning">禁止多语句</el-tag>
                  <el-tag size="small" type="warning">原因 / 高风险确认</el-tag>
                </template>
              </div>
            </div>

            <aside class="query-side-panel">
              <div class="query-side-card">
                <div class="query-side-title">执行参数</div>
                <div class="query-param-list">
                  <div class="query-param-item">
                    <span class="query-option-label">最大行数</span>
                    <el-input-number v-model="queryLimit" :min="1" :max="500" :step="50" class="query-number" :disabled="queryUnlimitedRows" />
                  </div>
                  <div class="query-param-item">
                    <span class="query-option-label">超时秒数</span>
                    <el-input-number v-model="queryTimeoutSeconds" :min="1" :max="30" class="query-number" />
                  </div>
                  <div class="query-param-item">
                    <span class="query-option-label">导出上限</span>
                    <el-input-number v-model="queryExportLimit" :min="1" :max="5000" :step="100" class="query-number" />
                  </div>
                  <div v-if="!isRedisQueryInstance && canUseQueryUnlimitedRows" class="query-param-item query-param-switch">
                    <span class="query-option-label">行数限制</span>
                    <el-switch
                      v-model="queryUnlimitedRows"
                      active-text="不限"
                      inactive-text="限制"
                    />
                  </div>
                </div>
              </div>

              <div v-if="!isRedisQueryInstance" class="query-side-card query-policy-card">
                <div class="query-policy-header">
                  <div class="query-side-title">安全策略</div>
                  <el-button link type="primary" :loading="databaseWriteConfigLoading" @click="loadDatabaseWriteConfig">
                    刷新
                  </el-button>
                </div>
                <div class="query-policy-list">
                  <div class="query-policy-row">
                    <span>写操作</span>
                    <el-switch
                      v-model="databaseConfig.writeEnabled"
                      :loading="databaseWriteConfigLoading || databaseWriteConfigSaving"
                      :disabled="!canManageInstancePermissions"
                      active-text="开"
                      inactive-text="关"
                      :before-change="handleBeforeDatabaseWriteToggle"
                    />
                  </div>
                  <el-tag :type="databaseConfig.writeEnabled ? 'success' : 'warning'">
                    {{ databaseConfig.writeEnabled ? '可预检查' : '统一拦截' }}
                  </el-tag>
                  <div class="query-policy-row">
                    <span>写 SQL 计划</span>
                    <el-switch
                      v-model="databaseConfig.writeExplainEnabled"
                      :loading="databaseWriteConfigLoading || databaseWriteConfigSaving"
                      :disabled="!canManageInstancePermissions"
                      active-text="开"
                      inactive-text="关"
                      :before-change="handleBeforeDatabaseWriteExplainToggle"
                    />
                  </div>
                  <el-tag :type="databaseConfig.writeExplainEnabled ? 'success' : 'info'">
                    {{ databaseConfig.writeExplainEnabled ? '可查看计划' : '计划关闭' }}
                  </el-tag>
                  <div class="query-policy-row">
                    <span>DDL 变更</span>
                    <el-switch
                      v-model="databaseConfig.ddlEnabled"
                      :loading="databaseWriteConfigLoading || databaseWriteConfigSaving"
                      :disabled="!canManageInstancePermissions"
                      active-text="开"
                      inactive-text="关"
                      :before-change="handleBeforeDatabaseDDLToggle"
                    />
                  </div>
                  <el-tag :type="databaseConfig.ddlEnabled ? 'danger' : 'info'">
                    {{ databaseConfig.ddlEnabled ? 'DDL 可执行' : 'DDL 关闭' }}
                  </el-tag>
                  <div class="query-policy-tags">
                    <el-tag v-if="!canManageInstancePermissions" type="info">仅管理员可切换</el-tag>
                    <el-tag :type="databaseConfig.highRiskRequiresConfirm ? 'warning' : 'info'">
                      {{ databaseConfig.highRiskRequiresConfirm ? '高风险确认' : '确认非强制' }}
                    </el-tag>
                    <el-tag :type="databaseConfig.operationReasonRequired ? 'warning' : 'info'">
                      {{ databaseConfig.operationReasonRequired ? '原因必填' : '原因选填' }}
                    </el-tag>
                    <el-tag type="info">阈值 {{ formatNumber(databaseConfig.maxAffectedRows) }} 行</el-tag>
                  </div>
                </div>
              </div>

              <div class="query-side-card">
                <div class="query-side-header">
                  <div class="query-side-title">收藏 SQL</div>
                  <el-button link type="primary" :disabled="!querySQL.trim()" @click="saveCurrentQueryFavorite">收藏当前</el-button>
                </div>
                <div v-if="queryFavorites.length" class="query-side-list">
                  <div v-for="item in queryFavorites.slice(0, 5)" :key="item.id" class="query-side-list-item">
                    <div class="query-side-item-main" @click="applyQueryFavorite(item)">
                      <strong>{{ item.title }}</strong>
                      <span>{{ item.sqlText }}</span>
                    </div>
                    <el-button link type="danger" @click.stop="removeQueryFavorite(item.id)">删除</el-button>
                  </div>
                </div>
                <el-empty v-else description="暂无收藏" :image-size="48" />
              </div>

              <div class="query-side-card">
                <div class="query-side-header">
                  <div class="query-side-title">最近 SQL</div>
                  <div>
                    <el-button link type="primary" :loading="queryHistoryLoading" @click="loadQueryHistory">刷新</el-button>
                    <el-button link type="primary" :loading="queryHistoryLoading" @click="openQueryHistory">更多</el-button>
                  </div>
                </div>
                <div v-if="queryHistoryItems.length" v-loading="queryHistoryLoading" class="query-side-list">
                  <div v-for="item in queryHistoryItems.slice(0, 5)" :key="`${item.id || item.auditId || item.createdAt}-${item.sqlText}`" class="query-side-list-item">
                    <div class="query-side-item-main" @click="applyHistoryQuery(item)">
                      <strong>{{ item.sqlType || '-' }} / {{ item.statusText || item.status || '-' }}</strong>
                      <span>{{ item.sqlSummary || item.sqlText || '-' }}</span>
                    </div>
                    <el-button link type="primary" @click.stop="saveHistoryQueryFavorite(item)">收藏</el-button>
                  </div>
                </div>
                <el-empty v-else description="暂无历史" :image-size="48" />
              </div>
            </aside>
          </div>

          <div class="query-result-panel">
            <div class="query-result-header">
              <div>
                <span class="query-section-eyebrow">RESULT</span>
                <h3>执行结果</h3>
              </div>
              <div class="query-result-status">
                <el-tag v-if="writeResult" :type="riskLevelTag(writeResult.riskLevel)">{{ writeResult.riskLevelText }}</el-tag>
                <el-tag v-if="ddlCheckResult" :type="riskLevelTag(ddlCheckResult.riskLevel)">{{ ddlCheckResult.riskLevelText }}</el-tag>
                <el-tag v-if="writeCheckResult" :type="riskLevelTag(writeCheckResult.riskLevel)">{{ writeCheckResult.riskLevelText }}</el-tag>
                <el-tag v-if="queryResult?.truncated" type="warning">结果已截断</el-tag>
                <el-tag v-if="queryResult?.cellsMasked" type="info">敏感字段已脱敏</el-tag>
              </div>
            </div>

            <div v-if="writeResult" class="query-result">
              <div class="result-summary">
                <div class="summary-card">
                  <span class="summary-label">{{ isDDLWriteResult ? '结构变更' : '影响行数' }}</span>
                  <strong>{{ isDDLWriteResult ? '已执行' : writeResult.rowsAffected }}</strong>
                </div>
              <div class="summary-card">
                <span class="summary-label">耗时</span>
                <strong>{{ writeResult.durationMs }} ms</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">SQL 类型</span>
                <strong>{{ writeResult.sqlType }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">风险等级</span>
                <strong>{{ writeResult.riskLevelText }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">审计 ID</span>
                <strong>{{ writeResult.auditId }}</strong>
              </div>
                <div class="summary-card">
                  <span class="summary-label">{{ isDDLWriteResult ? '执行通道' : '影响阈值' }}</span>
                  <strong>{{ isDDLWriteResult ? 'DDL' : writeResult.rowsAffectedLimit }}</strong>
                </div>
              </div>
              <el-alert
                :title="writeResult.message || (isDDLWriteResult ? 'DDL 结构变更执行完成' : '写操作执行完成')"
                type="success"
                show-icon
                :closable="false"
              />
              <el-tabs v-model="queryResultTab" class="query-result-tabs">
                <el-tab-pane v-if="writeResult.executedSql" label="实际执行 SQL" name="sql">
                  <div class="audit-sql-block">
                    <div class="audit-sql-title">
                      <span>实际执行 SQL</span>
                      <el-button link type="primary" @click="copyText(writeResult.executedSql || '', '实际执行 SQL')">复制</el-button>
                    </div>
                    <pre>{{ writeResult.executedSql }}</pre>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="审计信息" name="audit">
                  <el-descriptions :column="3" border class="query-audit-descriptions">
                    <el-descriptions-item label="审计 ID">{{ writeResult.auditId }}</el-descriptions-item>
                    <el-descriptions-item label="SQL 类型">{{ writeResult.sqlType }}</el-descriptions-item>
                    <el-descriptions-item label="风险等级">{{ writeResult.riskLevelText }}</el-descriptions-item>
                    <el-descriptions-item label="耗时">{{ writeResult.durationMs }} ms</el-descriptions-item>
                    <el-descriptions-item label="原因">{{ writeResult.reason || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="确认状态">{{ writeResult.confirmRequired ? (writeResult.confirmed ? '已确认' : '未确认') : '无需确认' }}</el-descriptions-item>
                  </el-descriptions>
                </el-tab-pane>
                <el-tab-pane v-if="writeResult.rollbackSql" label="回滚提示" name="rollback">
                  <div class="audit-sql-block">
                    <div class="audit-sql-title">
                      <span>回滚 SQL / 恢复提示</span>
                      <el-button link type="primary" @click="copyText(writeResult.rollbackSql || '', '回滚提示')">复制</el-button>
                    </div>
                    <pre>{{ writeResult.rollbackSql }}</pre>
                  </div>
                </el-tab-pane>
              </el-tabs>
            </div>
            <div v-else-if="ddlCheckResult" class="query-result">
              <div class="result-summary">
              <div class="summary-card">
                <span class="summary-label">DDL 检查</span>
                <strong>{{ ddlCheckResult.allowed ? '通过' : '未通过' }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">SQL 类型</span>
                <strong>{{ ddlCheckResult.sqlType }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">风险等级</span>
                <strong>{{ ddlCheckResult.riskLevelText }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">备份提示</span>
                <strong>{{ ddlCheckResult.backupRequired ? '需要' : '不要求' }}</strong>
              </div>
            </div>
            <el-alert
              :title="ddlCheckResult.message || 'DDL 结构变更检查完成'"
              :type="ddlCheckResult.allowed ? 'success' : 'warning'"
              show-icon
              :closable="false"
            />
              <el-tabs v-model="queryResultTab" class="query-result-tabs">
                <el-tab-pane label="检查详情" name="check">
                  <div class="write-meta-tags">
                    <el-tag :type="riskLevelTag(ddlCheckResult.riskLevel)">{{ ddlCheckResult.riskLevelText }}</el-tag>
                    <el-tag :type="ddlCheckResult.reasonRequired ? 'warning' : 'info'">
                      {{ ddlCheckResult.reasonRequired ? '执行时必须填写原因' : '原因非必填' }}
                    </el-tag>
                    <el-tag :type="ddlCheckResult.confirmRequired ? 'danger' : 'success'">
                      {{ ddlCheckResult.confirmRequired ? '执行前需二次确认' : '无需二次确认' }}
                    </el-tag>
                    <el-tag v-if="ddlCheckResult.backupRequired" type="warning">建议先确认备份</el-tag>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="待执行 SQL" name="sql">
                  <div class="audit-sql-block">
                    <div class="audit-sql-title">
                      <span>待执行 DDL</span>
                      <el-button link type="primary" @click="copyText(querySQL, '待执行 DDL')">复制</el-button>
                    </div>
                    <pre>{{ querySQL }}</pre>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="审计信息" name="audit">
                  <el-descriptions :column="3" border class="query-audit-descriptions">
                    <el-descriptions-item label="实例">{{ ddlCheckResult.instanceName || currentQueryInstance?.name || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="Schema">{{ ddlCheckResult.schemaName || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="SQL 类型">{{ ddlCheckResult.sqlType || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="风险等级">{{ ddlCheckResult.riskLevelText || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="备份提示">{{ ddlCheckResult.backupRequired ? '建议确认备份' : '不要求' }}</el-descriptions-item>
                    <el-descriptions-item label="确认要求">{{ ddlCheckResult.confirmRequired ? '需要二次确认' : '无需二次确认' }}</el-descriptions-item>
                  </el-descriptions>
                </el-tab-pane>
              </el-tabs>
            </div>
            <div v-else-if="writeCheckResult" class="query-result">
              <div class="result-summary">
              <div class="summary-card">
                <span class="summary-label">预检查结果</span>
                <strong>{{ writeCheckResult.allowed ? '通过' : '未通过' }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">SQL 类型</span>
                <strong>{{ writeCheckResult.sqlType }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">风险等级</span>
                <strong>{{ writeCheckResult.riskLevelText }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">影响阈值</span>
                <strong>{{ writeCheckResult.rowsAffectedLimit }}</strong>
              </div>
            </div>
            <el-alert
              :title="writeCheckResult.message || '写操作预检查完成'"
              :type="writeCheckResult.allowed ? 'success' : 'warning'"
              show-icon
              :closable="false"
            />
              <el-tabs v-model="queryResultTab" class="query-result-tabs">
                <el-tab-pane label="检查详情" name="check">
                  <div class="write-meta-tags">
                    <el-tag :type="riskLevelTag(writeCheckResult.riskLevel)">{{ writeCheckResult.riskLevelText }}</el-tag>
                    <el-tag :type="writeCheckResult.reasonRequired ? 'warning' : 'info'">
                      {{ writeCheckResult.reasonRequired ? '执行时必须填写原因' : '原因非必填' }}
                    </el-tag>
                    <el-tag :type="writeCheckResult.confirmRequired ? 'danger' : 'success'">
                      {{ writeCheckResult.confirmRequired ? '执行前需二次确认' : '无需二次确认' }}
                    </el-tag>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="待执行 SQL" name="sql">
                  <div class="audit-sql-block">
                    <div class="audit-sql-title">
                      <span>待执行 SQL</span>
                      <el-button link type="primary" @click="copyText(querySQL, '待执行 SQL')">复制</el-button>
                    </div>
                    <pre>{{ querySQL }}</pre>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="审计信息" name="audit">
                  <el-descriptions :column="3" border class="query-audit-descriptions">
                    <el-descriptions-item label="实例">{{ writeCheckResult.instanceName || currentQueryInstance?.name || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="Schema">{{ writeCheckResult.schemaName || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="SQL 类型">{{ writeCheckResult.sqlType || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="风险等级">{{ writeCheckResult.riskLevelText || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="影响阈值">{{ writeCheckResult.rowsAffectedLimit }}</el-descriptions-item>
                    <el-descriptions-item label="确认要求">{{ writeCheckResult.confirmRequired ? '需要二次确认' : '无需二次确认' }}</el-descriptions-item>
                  </el-descriptions>
                </el-tab-pane>
              </el-tabs>
            </div>
            <div v-else-if="queryResult" class="query-result">
              <div class="result-summary">
              <div class="summary-card">
                <span class="summary-label">返回行数</span>
                <strong>{{ queryResult.rowsReturned }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">耗时</span>
                <strong>{{ queryResult.durationMs }} ms</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">{{ isRedisQueryInstance ? '命令类型' : 'SQL 类型' }}</span>
                <strong>{{ queryResult.sqlType }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">审计 ID</span>
                <strong>{{ queryResult.auditId }}</strong>
              </div>
              <el-tag v-if="queryResult.truncated" type="warning">结果已截断</el-tag>
              <el-tag v-if="queryUnlimitedRows" type="danger">不限行数</el-tag>
              <el-tag v-if="queryResult.cellTruncated" type="warning">字段已截断</el-tag>
              <el-tag v-if="queryResult.cellsMasked" type="info">敏感字段已脱敏</el-tag>
              <el-tag v-if="queryResult.binaryPreviewed" type="info">二进制已预览</el-tag>
            </div>
              <el-tabs v-model="queryResultTab" class="query-result-tabs">
                <el-tab-pane label="结果集" name="result">
                  <div class="query-result-toolbar">
                    <span>{{ queryResult.columns?.length || 0 }} 列 / {{ queryResult.rowsReturned || 0 }} 行</span>
                    <span>点击单元格可复制内容</span>
                  </div>
                  <el-table :data="queryResult.rows || []" border stripe height="420" class="query-result-table">
                    <el-table-column
                      v-for="column in queryResult.columns || []"
                      :key="column"
                      :prop="column"
                      :label="column"
                      min-width="150"
                      show-overflow-tooltip
                    >
                      <template #default="{ row }">
                        <span class="query-cell query-cell-copyable" @click="copyQueryCell(row[column], column)">{{ formatQueryCell(row[column]) }}</span>
                      </template>
                    </el-table-column>
                  </el-table>
                </el-tab-pane>
                <el-tab-pane v-if="queryResult.executedSql" :label="isRedisQueryInstance ? '实际执行命令' : '实际执行 SQL'" name="sql">
                  <div class="audit-sql-block">
                    <div class="audit-sql-title">
                      <span>{{ isRedisQueryInstance ? '实际执行命令' : '实际执行 SQL' }}</span>
                      <el-button link type="primary" @click="copyText(queryResult.executedSql || '', isRedisQueryInstance ? '实际执行命令' : '实际执行 SQL')">复制</el-button>
                    </div>
                    <pre>{{ queryResult.executedSql }}</pre>
                  </div>
                </el-tab-pane>
                <el-tab-pane label="审计信息" name="audit">
                  <el-descriptions :column="3" border class="query-audit-descriptions">
                    <el-descriptions-item label="审计 ID">{{ queryResult.auditId }}</el-descriptions-item>
                    <el-descriptions-item :label="isRedisQueryInstance ? '命令类型' : 'SQL 类型'">{{ queryResult.sqlType }}</el-descriptions-item>
                    <el-descriptions-item label="耗时">{{ queryResult.durationMs }} ms</el-descriptions-item>
                    <el-descriptions-item label="返回行数">{{ queryResult.rowsReturned }}</el-descriptions-item>
                    <el-descriptions-item label="截断">{{ queryResult.truncated ? '是' : '否' }}</el-descriptions-item>
                    <el-descriptions-item label="导出上限">{{ formatNumber(queryExportLimit) }} 行</el-descriptions-item>
                  </el-descriptions>
                </el-tab-pane>
              </el-tabs>
            </div>
            <el-empty v-else :description="isRedisQueryInstance ? '执行 Redis 只读命令后查看结果' : '执行查询、预检查或受控操作后查看结果'" :image-size="72" />
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="诊断" name="diagnosis">
        <div class="diagnosis-panel">
          <div class="metadata-toolbar">
            <div class="metadata-selector">
              <span class="toolbar-label">数据库实例</span>
              <el-select
                v-model="diagnosisInstanceId"
                placeholder="请选择实例"
                filterable
                class="metadata-instance-select"
                @change="handleDiagnosisInstanceChange"
              >
                <el-option
                  v-for="item in diagnosisInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                  :value="item.id"
                />
              </el-select>
              <span class="toolbar-label">会话条数</span>
              <el-input-number v-model="diagnosisQuery.sessionLimit" :min="5" :max="50" :step="5" class="query-number" />
              <span class="toolbar-label">{{ isRedisDiagnosisInstance ? '慢日志条数' : '慢 SQL 条数' }}</span>
              <el-input-number v-model="diagnosisQuery.slowLimit" :min="5" :max="50" :step="5" class="query-number" />
              <span class="toolbar-label">容量范围</span>
              <el-select v-model="capacityRange" class="audit-select" @change="handleCapacityRangeChange">
                <el-option label="近 24 小时" value="24h" />
                <el-option label="近 7 天" value="7d" />
                <el-option label="近 30 天" value="30d" />
              </el-select>
              <el-tag v-if="currentDiagnosisInstance?.dbTypeText" type="success">
                {{ currentDiagnosisInstance.dbTypeText }}
              </el-tag>
            </div>
            <div class="backup-toolbar-group">
              <el-button :disabled="!diagnosisInstanceId || !hasDatabasePermission(currentDiagnosisInstance, DATABASE_PERMISSION.DIAGNOSIS)" :loading="capacityCollecting" @click="handleCollectCapacity">
                采集容量
              </el-button>
              <el-button type="primary" :disabled="!diagnosisInstanceId || !hasDatabasePermission(currentDiagnosisInstance, DATABASE_PERMISSION.DIAGNOSIS)" :loading="diagnosisLoading" @click="loadDiagnosisData">
                刷新诊断
              </el-button>
            </div>
          </div>

          <div v-if="!diagnosisInstanceId" class="metadata-empty">
            <el-empty description="请先选择一个数据库实例" :image-size="82" />
          </div>
          <div v-else class="diagnosis-content" v-loading="diagnosisLoading">
            <div class="diagnosis-section capacity-section" v-loading="capacityLoading">
              <div class="panel-title">
                <span>{{ isRedisDiagnosisInstance ? '内存趋势' : '容量趋势' }}</span>
                <el-tag size="small" type="info">{{ capacityTrend?.rangeText || '-' }}</el-tag>
              </div>
              <el-alert
                  v-if="capacityTrend?.message && !capacityTrendPoints.length"
                  :title="capacityTrend.message"
                  type="info"
                  show-icon
                  :closable="false"
                  class="diagnosis-alert"
                />
                <div v-if="capacityTrend" class="table-overview-cards diagnosis-cards capacity-summary-cards">
                  <div class="table-overview-card">
                    <span class="summary-label">{{ isRedisDiagnosisInstance ? '当前内存' : '当前容量' }}</span>
                    <strong>{{ capacityTrend.latestSizeText || '-' }}</strong>
                    <div class="diagnosis-card-desc">{{ isRedisDiagnosisInstance ? '最近一次 Redis 内存快照' : '最近一次容量快照' }}</div>
                  </div>
                  <div class="table-overview-card">
                    <span class="summary-label">区间增长</span>
                    <strong>{{ capacityTrend.growthText || '0 B' }}</strong>
                    <div class="diagnosis-card-desc">{{ Number(capacityTrend.growthPercent || 0).toFixed(2) }}%</div>
                  </div>
                  <div class="table-overview-card">
                    <span class="summary-label">采样点</span>
                    <strong>{{ capacityTrend.points?.length || 0 }}</strong>
                    <div class="diagnosis-card-desc">后台定时采集或手动采集</div>
                  </div>
                </div>
                <div class="capacity-chart-panel">
                  <div class="capacity-chart-header">
                    <div class="capacity-chart-title">
                      <span>{{ currentDiagnosisInstance?.name || (isRedisDiagnosisInstance ? '内存趋势' : '容量趋势') }}</span>
                      <span class="capacity-chart-meta">
                        {{ isRedisDiagnosisInstance ? '时间从左到右递增，展示 Redis used_memory 变化' : '时间从左到右递增，最新采样位于右侧' }}
                      </span>
                    </div>
                    <span class="capacity-chart-meta">{{ capacityTrend?.points?.length || 0 }} 个采样点</span>
                  </div>
                  <div class="capacity-chart-body">
                    <div
                      :key="`${diagnosisInstanceId || 0}-${capacityRange}`"
                      ref="capacityChartRef"
                      class="capacity-chart"
                    ></div>
                    <div v-if="!capacityTrendPoints.length" class="capacity-chart-empty">
                      <el-empty description="暂无容量趋势数据" :image-size="72" />
                    </div>
                  </div>
                </div>
                <div v-if="capacityTrend?.topTables?.length" class="topology-section">
                  <div class="panel-title">
                    <span>{{ isRedisDiagnosisInstance ? 'Top Key 样本' : '容量 Top 表' }}</span>
                    <el-tag size="small" type="warning">{{ capacityTrend.topTables.length }}</el-tag>
                  </div>
                  <el-alert
                    v-if="isRedisDiagnosisInstance"
                    title="Top Key 列表基于容量采样时的 Key 样本，不代表全量 Key 排名。"
                    type="info"
                    show-icon
                    :closable="false"
                    class="diagnosis-alert"
                  />
                  <el-table :data="capacityTrend.topTables" stripe height="260" class="modern-table">
                    <template v-if="isRedisDiagnosisInstance">
                      <el-table-column label="逻辑 DB" prop="schemaName" min-width="120" show-overflow-tooltip />
                      <el-table-column label="Key" prop="tableName" min-width="220" show-overflow-tooltip />
                      <el-table-column label="长度" width="110" align="right">
                        <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
                      </el-table-column>
                      <el-table-column label="估算内存" width="130" align="right">
                        <template #default="{ row }">{{ row.totalSizeText || formatBytes(row.totalSizeBytes) }}</template>
                      </el-table-column>
                      <el-table-column label="采样时间" width="170">
                        <template #default="{ row }">{{ row.collectedAt || '-' }}</template>
                      </el-table-column>
                    </template>
                    <template v-else>
                      <el-table-column label="Schema" prop="schemaName" min-width="140" show-overflow-tooltip />
                      <el-table-column label="表" prop="tableName" min-width="180" show-overflow-tooltip />
                      <el-table-column label="行数" width="120" align="right">
                        <template #default="{ row }">{{ formatNumber(row.rowCount) }}</template>
                      </el-table-column>
                      <el-table-column label="数据容量" width="130" align="right">
                        <template #default="{ row }">{{ formatBytes(row.dataSizeBytes) }}</template>
                      </el-table-column>
                      <el-table-column label="索引容量" width="130" align="right">
                        <template #default="{ row }">{{ formatBytes(row.indexSizeBytes) }}</template>
                      </el-table-column>
                      <el-table-column label="总容量" width="130" align="right">
                        <template #default="{ row }">{{ row.totalSizeText || formatBytes(row.totalSizeBytes) }}</template>
                      </el-table-column>
                    </template>
                  </el-table>
                </div>
            </div>

            <div v-if="diagnosisMetrics?.cards?.length" class="table-overview-cards diagnosis-cards diagnosis-metric-cards">
              <div v-for="card in diagnosisMetrics?.cards || []" :key="card.key" class="table-overview-card">
                <span class="summary-label">{{ card.label }}</span>
                <strong>{{ card.value }}</strong>
                <div class="diagnosis-card-desc">{{ card.description }}</div>
              </div>
            </div>

            <div class="diagnosis-grid">
              <div class="diagnosis-section">
                <div class="panel-title">
                  <span>{{ isRedisDiagnosisInstance ? '客户端会话' : '活跃会话' }}</span>
                  <el-tag size="small" type="info">{{ diagnosisSessions.length }}</el-tag>
                </div>
                <el-table
                  v-if="diagnosisSessions.length > 0"
                  :data="diagnosisSessions"
                  stripe
                  height="360"
                  class="modern-table"
                >
                  <el-table-column label="会话 ID" prop="sessionId" width="110" />
                  <el-table-column label="用户" prop="user" width="120" />
                  <el-table-column :label="isRedisDiagnosisInstance ? '逻辑 DB' : '库 / Schema'" min-width="130">
                    <template #default="{ row }">{{ row.databaseName || '-' }}</template>
                  </el-table-column>
                  <el-table-column v-if="isRedisDiagnosisInstance" label="命令" min-width="130">
                    <template #default="{ row }">{{ row.command || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="状态" width="120">
                    <template #default="{ row }">{{ row.state || row.command || '-' }}</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '标记 / 空闲' : '等待事件'" min-width="130">
                    <template #default="{ row }">{{ row.waitEvent || '-' }}</template>
                  </el-table-column>
                  <el-table-column v-if="isRedisDiagnosisInstance" label="空闲时长" width="100" align="right">
                    <template #default="{ row }">{{ row.idleSeconds || 0 }}s</template>
                  </el-table-column>
                  <el-table-column label="持续时长" width="100" align="right">
                    <template #default="{ row }">{{ row.durationSeconds || 0 }}s</template>
                  </el-table-column>
                  <el-table-column label="客户端" min-width="150">
                    <template #default="{ row }">{{ row.clientAddr || '-' }}</template>
                  </el-table-column>
                  <el-table-column v-if="isRedisDiagnosisInstance" label="节点" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.nodeAddress || '-' }}</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '详情' : 'SQL'" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">
                      <span class="mono">{{ row.sqlText || '-' }}</span>
                    </template>
                  </el-table-column>
                </el-table>
                <el-empty v-else :description="isRedisDiagnosisInstance ? '暂无客户端会话' : '暂无活跃会话'" :image-size="64" />
              </div>

              <div class="diagnosis-section">
                <div class="panel-title">
                  <span>{{ isRedisDiagnosisInstance ? '慢日志' : '慢 SQL' }}</span>
                  <el-tag size="small" type="warning">{{ diagnosisSlowQueries.length }}</el-tag>
                </div>
                <el-alert
                  v-if="diagnosisSlowMessage"
                  :title="diagnosisSlowMessage"
                  type="info"
                  show-icon
                  :closable="false"
                  class="diagnosis-alert"
                />
                <el-table
                  v-if="diagnosisSlowQueries.length > 0"
                  :data="diagnosisSlowQueries"
                  stripe
                  height="360"
                  class="modern-table"
                >
                  <el-table-column :label="isRedisDiagnosisInstance ? '逻辑 DB' : '库 / Schema'" min-width="130">
                    <template #default="{ row }">{{ row.schemaName || '-' }}</template>
                  </el-table-column>
                  <el-table-column v-if="isRedisDiagnosisInstance" label="节点" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.nodeAddress || '-' }}</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '命令摘要' : 'SQL 摘要'" min-width="280" show-overflow-tooltip>
                    <template #default="{ row }">
                      <span class="mono">{{ row.sqlSummary || row.sqlText || '-' }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '慢日志 ID' : '执行次数'" width="100" align="right">
                    <template #default="{ row }">{{ formatNumber(row.execCount) }}</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '耗时' : '平均耗时'" width="110" align="right">
                    <template #default="{ row }">{{ Number(row.avgDurationMs || 0).toFixed(2) }} ms</template>
                  </el-table-column>
                  <el-table-column v-if="!isRedisDiagnosisInstance" label="最大耗时" width="110" align="right">
                    <template #default="{ row }">{{ Number(row.maxDurationMs || 0).toFixed(2) }} ms</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '参数数' : '行数指标'" width="110" align="right">
                    <template #default="{ row }">{{ formatNumber(row.rowsMetric) }}</template>
                  </el-table-column>
                  <el-table-column :label="isRedisDiagnosisInstance ? '记录时间' : '最近观测'" width="170">
                    <template #default="{ row }">{{ row.lastSeen || '-' }}</template>
                  </el-table-column>
                </el-table>
                <el-empty v-else-if="!diagnosisSlowMessage" :description="isRedisDiagnosisInstance ? '暂无慢日志数据' : '暂无慢 SQL 数据'" :image-size="64" />
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="拓扑" name="topology">
        <div class="topology-panel">
          <el-alert
            title="拓扑支持 Redis、MongoDB、Elasticsearch / OpenSearch 以及 MySQL / PostgreSQL 主从关系的只读采集，所有查询写入统一审计。"
            type="info"
            show-icon
            :closable="false"
          />

          <div class="metadata-toolbar">
            <div class="metadata-selector">
              <span class="toolbar-label">数据库实例</span>
              <el-select
                v-model="topologyInstanceId"
                placeholder="请选择拓扑实例"
                filterable
                class="metadata-instance-select"
                @change="handleTopologyInstanceChange"
              >
                <el-option
                  v-for="item in topologyInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.dbTypeText || item.dbType} / ${item.endpoint || `${item.host}:${item.port}`}）`"
                  :value="item.id"
                />
              </el-select>
              <el-tag v-if="currentTopologyInstance?.dbTypeText" type="success">
                {{ currentTopologyInstance.dbTypeText }}
              </el-tag>
            </div>
            <div class="topology-toolbar-actions">
              <el-button
                :disabled="!topologyInstanceId"
                :loading="topologyCollectingId === topologyInstanceId"
                @click="handleCheckCurrentTopology"
              >
                采集当前实例
              </el-button>
              <el-button
                type="warning"
                plain
                :disabled="!topologyResult?.nodes?.length"
                :loading="topologyCollectingAll"
                @click="handleCheckTopologyRelated"
              >
                采集相关实例
              </el-button>
              <el-button type="primary" :disabled="!topologyInstanceId" :loading="topologyLoading" @click="loadTopology">
                刷新拓扑
              </el-button>
            </div>
          </div>

          <div v-if="!topologyInstanceId" class="metadata-empty">
            <el-empty description="请选择已开通拓扑权限和拓扑能力的数据库实例" :image-size="82" />
          </div>
          <div v-else class="topology-content" v-loading="topologyLoading">
            <el-empty v-if="!topologyResult" description="点击刷新拓扑后查看结果" :image-size="82" />
            <template v-else>
              <div class="table-overview-cards diagnosis-cards">
                <div v-for="card in topologyResult.cards || []" :key="card.key" class="table-overview-card">
                  <span class="summary-label">{{ card.label }}</span>
                  <strong>{{ card.value || '-' }}</strong>
                  <div class="diagnosis-card-desc">{{ card.description }}</div>
                </div>
              </div>

              <el-alert
                v-if="topologyResult.message"
                :title="topologyResult.message"
                type="success"
                show-icon
                :closable="false"
                class="topology-message"
              />

              <div v-if="topologyResult.findings?.length" class="topology-section topology-findings-section">
                <div class="panel-title">
                  <span>拓扑风险</span>
                  <el-tag size="small" type="warning">{{ topologyResult.findings.length }}</el-tag>
                </div>
                <div class="topology-findings">
                  <div v-for="item in topologyResult.findings" :key="`${item.level}-${item.category}-${item.title}-${item.nodeId || item.linkId || ''}`" class="topology-finding-item">
                    <el-tag size="small" :type="topologyFindingTag(item.level)">{{ topologyFindingLevelText(item.level) }}</el-tag>
                    <div class="topology-finding-body">
                      <strong>{{ item.title }}</strong>
                      <span>{{ item.description || '-' }}</span>
                      <span class="muted-text">{{ item.suggestion || '-' }}</span>
                    </div>
                    <el-button link type="primary" @click="handleTopologyFindingAction(item)">处理</el-button>
                  </div>
                </div>
              </div>

              <div v-if="topologyResult.nodes?.length" class="topology-section topology-graph-section">
                <div class="panel-title">
                  <span>关系图</span>
                  <el-tag size="small" type="info">{{ topologyResult.topologyTypeText || topologyResult.topologyType || '-' }}</el-tag>
                </div>
                <div ref="topologyChartRef" class="topology-chart"></div>
              </div>

              <div class="topology-grid">
                <div class="topology-section">
                  <div class="panel-title">
                    <span>拓扑节点</span>
                    <el-tag size="small" type="info">{{ topologyResult.nodes?.length || 0 }}</el-tag>
                  </div>
                  <el-table :data="topologyResult.nodes || []" stripe height="360" class="modern-table">
                    <el-table-column label="节点" min-width="150" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.name || row.id || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="角色" width="130">
                      <template #default="{ row }">
                        <el-tag size="small" :type="topologyRoleTag(row.role)">{{ row.roleText || row.role || '-' }}</el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column label="地址" min-width="170" show-overflow-tooltip>
                      <template #default="{ row }"><span class="mono">{{ row.address || '-' }}</span></template>
                    </el-table-column>
                    <el-table-column label="状态" width="110">
                      <template #default="{ row }">
                        <el-tag size="small" :type="topologyStateTag(row.state)">{{ row.state || '-' }}</el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column label="Slots / 延迟" min-width="180" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.slots || row.lagText || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="版本" width="130">
                      <template #default="{ row }">{{ row.version || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="备注" min-width="180" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.message || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="操作" width="100" fixed="right">
                      <template #default="{ row }">
                        <el-button link type="primary" @click="openTopologyNodeDetail(row)">详情</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>

                <div class="topology-section">
                  <div class="panel-title">
                    <span>复制关系</span>
                    <el-tag size="small" type="info">{{ topologyResult.links?.length || 0 }}</el-tag>
                  </div>
                  <el-table :data="topologyResult.links || []" stripe height="360" class="modern-table">
                    <el-table-column label="源节点" min-width="160" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.sourceName || topologyNodeName(row.source) || row.source || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="目标节点" min-width="160" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.targetName || topologyNodeName(row.target) || row.target || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="关系" min-width="150" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.label || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="延迟" width="110" align="right">
                      <template #default="{ row }">{{ row.lagText || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="状态" width="120">
                      <template #default="{ row }">
                        <el-tag size="small" :type="topologyStateTag(row.state)">{{ row.state || '-' }}</el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column label="备注" min-width="160" show-overflow-tooltip>
                      <template #default="{ row }">{{ row.message || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="操作" width="100" fixed="right">
                      <template #default="{ row }">
                        <el-button link type="primary" @click="openTopologyLinkDetail(row)">详情</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>
              </div>

              <div v-if="topologyResult.shards?.length" class="topology-section">
                <div class="panel-title">
                  <span>索引分片</span>
                  <el-tag size="small" type="warning">{{ topologyResult.shards.length }}</el-tag>
                </div>
                <el-table :data="topologyResult.shards" stripe height="420" class="modern-table">
                  <el-table-column label="索引" prop="index" min-width="220" show-overflow-tooltip />
                  <el-table-column label="分片" prop="shard" width="90" align="center" />
                  <el-table-column label="主副" width="90" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="row.primary ? 'success' : 'info'">{{ row.primary ? '主' : '副' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="状态" width="120">
                    <template #default="{ row }">
                      <el-tag size="small" :type="topologyStateTag(row.state)">{{ row.state || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="节点" prop="node" min-width="160" show-overflow-tooltip />
                  <el-table-column label="地址" prop="address" min-width="140" show-overflow-tooltip />
                  <el-table-column label="文档数" width="120" align="right">
                    <template #default="{ row }">{{ formatNumber(row.docs) }}</template>
                  </el-table-column>
                  <el-table-column label="容量" width="120" align="right">
                    <template #default="{ row }">{{ formatBytes(row.storeBytes) }}</template>
                  </el-table-column>
                </el-table>
              </div>
            </template>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="副本治理" name="replication">
        <div class="backup-panel">
          <el-alert
            title="P4.4 支持对已确认 replica/standby 执行固定白名单 pause/resume apply 命令；不支持 promote、failover 或任意 SQL。执行前会自动复查角色并写入审计。"
            type="warning"
            show-icon
            :closable="false"
          />

          <div class="backup-card">
            <div class="section-title">
              <span>误操作保护窗口</span>
              <el-tag size="small" type="info">{{ replicationProtectionTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select v-model="replicationProtectionQuery.instanceId" placeholder="主库实例" clearable filterable class="audit-search-input" @change="loadReplicationProtections">
                  <el-option
                    v-for="item in replicationInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                    :value="item.id"
                  />
                </el-select>
                <el-select v-model="replicationProtectionQuery.engine" placeholder="引擎" clearable class="audit-select" @change="loadReplicationProtections">
                  <el-option label="MySQL" value="mysql" />
                  <el-option label="MariaDB" value="mariadb" />
                  <el-option label="PostgreSQL" value="postgresql" />
                </el-select>
                <el-select v-model="replicationProtectionQuery.protectionStatus" placeholder="保护状态" clearable class="audit-select" @change="loadReplicationProtections">
                  <el-option label="有保护窗口" value="protected" />
                  <el-option label="保护降级" value="degraded" />
                  <el-option label="无延迟保护" value="unprotected" />
                  <el-option label="未知" value="unknown" />
                </el-select>
                <el-select v-model="replicationProtectionQuery.riskLevel" placeholder="风险等级" clearable class="audit-select" @change="loadReplicationProtections">
                  <el-option label="健康" value="healthy" />
                  <el-option label="警告" value="warning" />
                  <el-option label="异常" value="critical" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button type="primary" plain :loading="replicationProtectionLoading" @click="loadReplicationProtections">刷新保护窗口</el-button>
                <el-button type="warning" :loading="replicationCheckLoading" @click="handleCheckAllReplication">重新采集</el-button>
              </div>
            </div>

            <el-table :data="replicationProtections" v-loading="replicationProtectionLoading" stripe class="modern-table">
              <el-table-column label="主库" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.primaryInstanceName || `#${row.primaryInstanceId}` }}</span>
                    <span class="muted-text">{{ row.primaryEndpoint || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="保护状态" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicationProtectionStatusTag(row.protectionStatus)">{{ row.protectionStatusText || row.protectionStatus || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="最佳延迟副本" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.preferredReplicaInstanceName || (row.hasDelayedReplica ? `#${row.preferredReplicaInstanceId}` : '无延迟副本') }}</span>
                    <span class="muted-text">{{ row.preferredReplicaEndpoint || `延迟副本 ${row.delayedReplicaCount || 0} 个` }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="配置延迟" width="110" align="right">
                <template #default="{ row }">{{ replicationDelayText(row.configuredDelaySeconds) }}</template>
              </el-table-column>
              <el-table-column label="剩余窗口" width="130" align="right">
                <template #default="{ row }">
                  <span>{{ replicationDelayText(row.remainingDelaySeconds) }}</span>
                  <el-tag v-if="row.remainingDelayEstimated" size="small" type="info" effect="plain" class="ml-1">估算</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="Apply / Replay" width="170" show-overflow-tooltip>
                <template #default="{ row }">{{ row.applyTime || (row.applyLagSeconds ? `${replicationDelayText(row.applyLagSeconds)} 前` : '-') }}</template>
              </el-table-column>
              <el-table-column label="风险" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <el-tag size="small" :type="replicationHealthTag(row.riskLevel)">{{ row.riskLevelText || row.riskLevel || '-' }}</el-tag>
                    <span class="muted-text">{{ replicationRiskText(row.riskMessages) }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="最近检查" width="170">
                <template #default="{ row }">{{ row.lastCheckedAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="230" fixed="right">
                <template #default="{ row }">
                  <el-button v-if="row.preferredReplicaInstanceId" link type="primary" :loading="replicationCheckingId === row.preferredReplicaInstanceId" @click="handleCheckReplication(row.preferredReplicaInstanceId)">采集</el-button>
                  <el-button v-if="uiPermissions.replicaIncidentGuide" link type="warning" @click="handleOpenReplicaIncidentGuide(row)">生成事故指引</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="replicationProtectionQuery.page"
                v-model:page-size="replicationProtectionQuery.pageSize"
                :total="replicationProtectionTotal"
                :page-sizes="[6, 10, 20]"
                layout="total, sizes, prev, pager, next"
                @size-change="loadReplicationProtections"
                @current-change="loadReplicationProtections"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="section-title">
              <span>事故指引</span>
              <el-tag size="small" type="info">{{ replicaIncidentGuideTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select v-model="replicaIncidentGuideQuery.instanceId" placeholder="事故实例" clearable filterable class="audit-search-input" @change="loadReplicaIncidentGuides">
                  <el-option
                    v-for="item in replicationInstances"
                    :key="item.id"
                    :label="`${item.name} (${item.dbType})`"
                    :value="item.id"
                  />
                </el-select>
                <el-select v-model="replicaIncidentGuideQuery.incidentType" placeholder="事故类型" clearable class="audit-select" @change="loadReplicaIncidentGuides">
                  <el-option label="误删" value="delete" />
                  <el-option label="误更新" value="update" />
                  <el-option label="错误发布" value="release" />
                  <el-option label="其他" value="other" />
                </el-select>
                <el-select v-model="replicaIncidentGuideQuery.canIntercept" placeholder="截停机会" clearable class="audit-select" @change="loadReplicaIncidentGuides">
                  <el-option label="可能可截停" value="true" />
                  <el-option label="不可截停/未知" value="false" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button type="primary" plain :loading="replicaIncidentGuideLoading" @click="loadReplicaIncidentGuides">刷新指引</el-button>
              </div>
            </div>
            <el-table :data="replicaIncidentGuides" v-loading="replicaIncidentGuideLoading" stripe class="modern-table">
              <el-table-column label="事故实例" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.instanceName || `#${row.instanceId}` }}</span>
                    <span class="muted-text">{{ row.instanceEndpoint || row.engineText || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="事故类型" width="100">
                <template #default="{ row }">
                  <el-tag size="small" type="warning" effect="plain">{{ row.incidentTypeText || row.incidentType }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="事故时间" width="170">
                <template #default="{ row }">{{ row.incidentTime || '-' }}</template>
              </el-table-column>
              <el-table-column label="推荐延迟副本" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.preferredReplicaName || (row.preferredReplicaInstanceId ? `#${row.preferredReplicaInstanceId}` : '无') }}</span>
                    <span class="muted-text">{{ row.preferredReplicaEndpoint || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="截停机会" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.canIntercept ? 'success' : 'danger'">{{ row.canIntercept ? '可能可截停' : '不可截停/未知' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="剩余窗口" width="110" align="right">
                <template #default="{ row }">{{ replicationDelayText(row.remainingDelaySeconds) }}</template>
              </el-table-column>
              <el-table-column label="影响摘要" min-width="220" show-overflow-tooltip prop="affectedSummary" />
              <el-table-column label="生成时间" width="170">
                <template #default="{ row }">{{ row.createdAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="handleViewReplicaIncidentGuide(row)">查看指引</el-button>
                  <el-button
                    v-if="uiPermissions.replicaIncidentGuide"
                    link
                    type="danger"
                    :loading="replicaIncidentGuideDeletingId === row.id"
                    @click="handleDeleteReplicaIncidentGuide(row)"
                  >
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="replicaIncidentGuideQuery.page"
                v-model:page-size="replicaIncidentGuideQuery.pageSize"
                :total="replicaIncidentGuideTotal"
                :page-sizes="[5, 10, 20]"
                layout="total, sizes, prev, pager, next"
                @size-change="loadReplicaIncidentGuides"
                @current-change="loadReplicaIncidentGuides"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="section-title">
              <span>副本关系</span>
              <el-tag size="small" type="info">{{ replicationReplicaTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select v-model="replicationReplicaQuery.instanceId" placeholder="实例" clearable filterable class="audit-search-input" @change="loadReplicationReplicas">
                  <el-option
                    v-for="item in replicationInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                    :value="item.id"
                  />
                </el-select>
                <el-select v-model="replicationReplicaQuery.engine" placeholder="引擎" clearable class="audit-select" @change="loadReplicationReplicas">
                  <el-option label="MySQL" value="mysql" />
                  <el-option label="MariaDB" value="mariadb" />
                  <el-option label="PostgreSQL" value="postgresql" />
                </el-select>
                <el-select v-model="replicationReplicaQuery.replicaRole" placeholder="角色" clearable class="audit-select" @change="loadReplicationReplicas">
                  <el-option label="实时副本" value="realtime_replica" />
                  <el-option label="延迟副本" value="delayed_replica" />
                  <el-option label="Standby" value="standby" />
                  <el-option label="未知" value="unknown" />
                </el-select>
                <el-select v-model="replicationReplicaQuery.status" placeholder="健康状态" clearable class="audit-select" @change="loadReplicationReplicas">
                  <el-option label="健康" value="healthy" />
                  <el-option label="警告" value="warning" />
                  <el-option label="异常" value="critical" />
                  <el-option label="未知" value="unknown" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button type="primary" plain :loading="replicationLoading" @click="loadReplicationReplicas">刷新关系</el-button>
                <el-button type="warning" :loading="replicationCheckLoading" @click="handleCheckAllReplication">全量采集</el-button>
              </div>
            </div>

            <el-table :data="replicationReplicas" v-loading="replicationLoading" stripe class="modern-table">
              <el-table-column label="主库" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.primaryInstanceName || '来源未匹配' }}</span>
                    <span class="muted-text">{{ row.primaryEndpoint || `${row.sourceHost || '-'}:${row.sourcePort || '-'}` }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="副本" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.replicaInstanceName || `#${row.replicaInstanceId}` }}</span>
                    <span class="muted-text">{{ row.replicaEndpoint || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="引擎" width="120">
                <template #default="{ row }">
                  <el-tag size="small" :type="dbTypeTag(row.engine)">{{ row.engineText || row.engine || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="角色" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicationRoleTag(row.replicaRole)">{{ row.replicaRoleText || row.replicaRole || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="110">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicationHealthTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="Apply状态" width="120">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicaApplyStateTag(row.applyState)">{{ row.applyStateText || row.applyState || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="配置延迟" width="110" align="right">
                <template #default="{ row }">{{ replicationDelayText(row.configuredDelaySeconds) }}</template>
              </el-table-column>
              <el-table-column label="发现来源" width="120">
                <template #default="{ row }">{{ row.discoverySourceText || row.discoverySource || '-' }}</template>
              </el-table-column>
              <el-table-column label="最近检查" width="170">
                <template #default="{ row }">{{ row.lastCheckedAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="最近错误" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ row.lastError || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="170" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :loading="replicationCheckingId === row.replicaInstanceId" @click="handleCheckReplication(row.replicaInstanceId)">采集</el-button>
                  <el-button link type="info" @click="handleViewReplicationStatus(row.replicaInstanceId)">详情</el-button>
                  <el-button v-if="uiPermissions.replicaPauseApply && !row.applyPaused" link type="danger" @click="handleOpenReplicaAction(row, 'pause')">暂停</el-button>
                  <el-button v-if="uiPermissions.replicaResumeApply && row.applyPaused" link type="success" @click="handleOpenReplicaAction(row, 'resume')">恢复</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="replicationReplicaQuery.page"
                v-model:page-size="replicationReplicaQuery.pageSize"
                :total="replicationReplicaTotal"
                :page-sizes="[10, 20, 50]"
                layout="total, sizes, prev, pager, next"
                @size-change="loadReplicationReplicas"
                @current-change="loadReplicationReplicas"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="section-title">
              <span>Apply 操作记录</span>
              <el-tag size="small" type="info">{{ replicaActionTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select v-model="replicaActionQuery.instanceId" placeholder="实例" clearable filterable class="audit-search-input" @change="loadReplicaActions">
                  <el-option
                    v-for="item in replicationInstances"
                    :key="item.id"
                    :label="`${item.name} (${item.dbType})`"
                    :value="item.id"
                  />
                </el-select>
                <el-select v-model="replicaActionQuery.action" placeholder="动作" clearable class="audit-select" @change="loadReplicaActions">
                  <el-option label="暂停 apply" value="pause_apply" />
                  <el-option label="恢复 apply" value="resume_apply" />
                </el-select>
                <el-select v-model="replicaActionQuery.status" placeholder="状态" clearable class="audit-select" @change="loadReplicaActions">
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
                  <el-option label="待执行" value="pending" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button type="primary" plain :loading="replicaActionLoading" @click="loadReplicaActions">刷新记录</el-button>
              </div>
            </div>
            <el-table :data="replicaActions" v-loading="replicaActionLoading" stripe class="modern-table">
              <el-table-column label="动作" width="120">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicaActionTag(row.action)">{{ row.actionText || row.action }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="副本" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.replicaInstanceName || `#${row.replicaInstanceId}` }}</span>
                    <span class="muted-text">{{ row.replicaEndpoint || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="事故编号" width="150" prop="incidentNo" show-overflow-tooltip />
              <el-table-column label="命令" min-width="180" prop="commandTemplate" show-overflow-tooltip />
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="auditStatusTag(row.status)">{{ row.statusText || row.status }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="原因" min-width="220" prop="reason" show-overflow-tooltip />
              <el-table-column label="错误" min-width="220" prop="errorMessage" show-overflow-tooltip />
              <el-table-column label="操作人" width="120" prop="operatorName" show-overflow-tooltip />
              <el-table-column label="时间" width="170">
                <template #default="{ row }">{{ row.createdAt || '-' }}</template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="replicaActionQuery.page"
                v-model:page-size="replicaActionQuery.pageSize"
                :total="replicaActionTotal"
                :page-sizes="[5, 10, 20]"
                layout="total, sizes, prev, pager, next"
                @size-change="loadReplicaActions"
                @current-change="loadReplicaActions"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="section-title">
              <span>最近采集</span>
              <el-tag size="small" type="info">{{ replicationCheckTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select v-model="replicationCheckQuery.instanceId" placeholder="实例" clearable filterable class="audit-search-input" @change="loadReplicationChecks">
                  <el-option
                    v-for="item in replicationInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                    :value="item.id"
                  />
                </el-select>
                <el-select v-model="replicationCheckQuery.roleDetected" placeholder="检测角色" clearable class="audit-select" @change="loadReplicationChecks">
                  <el-option label="主库" value="primary" />
                  <el-option label="从库" value="replica" />
                  <el-option label="Standby" value="standby" />
                  <el-option label="未知" value="unknown" />
                </el-select>
                <el-select v-model="replicationCheckQuery.healthStatus" placeholder="健康状态" clearable class="audit-select" @change="loadReplicationChecks">
                  <el-option label="健康" value="healthy" />
                  <el-option label="警告" value="warning" />
                  <el-option label="异常" value="critical" />
                  <el-option label="未知" value="unknown" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button type="primary" plain :loading="replicationCheckLoading" @click="loadReplicationChecks">刷新采集</el-button>
              </div>
            </div>

            <el-table :data="replicationChecks" v-loading="replicationCheckLoading" stripe class="modern-table">
              <el-table-column label="实例" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="backup-name-cell">
                    <span class="backup-name">{{ row.instanceName || `#${row.instanceId}` }}</span>
                    <span class="muted-text">{{ row.instanceEndpoint || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="引擎" width="120">
                <template #default="{ row }">
                  <el-tag size="small" :type="dbTypeTag(row.engine)">{{ row.engineText || row.engine || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="检测角色" width="120">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicationRoleTag(row.roleDetected)">{{ row.roleDetectedText || row.roleDetected || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="来源实例" min-width="140" show-overflow-tooltip>
                <template #default="{ row }">{{ row.sourceInstanceName || '未匹配' }}</template>
              </el-table-column>
              <el-table-column label="健康" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="replicationHealthTag(row.healthStatus)">{{ row.healthStatusText || row.healthStatus || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="lag" width="120" align="right">
                <template #default="{ row }">
                  <span v-if="row.engine === 'postgresql'">{{ row.pgReplayLagMs ? `${row.pgReplayLagMs} ms` : '-' }}</span>
                  <span v-else>{{ replicationDelayText(row.secondsBehindSource) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="remaining" width="120" align="right">
                <template #default="{ row }">{{ replicationDelayText(row.remainingDelaySeconds) }}</template>
              </el-table-column>
              <el-table-column label="IO / SQL" width="130">
                <template #default="{ row }">
                  <span class="muted-text">{{ row.replicaIoRunning || '-' }} / {{ row.replicaSqlRunning || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Replay LSN / 时间" min-width="190" show-overflow-tooltip>
                <template #default="{ row }">{{ row.pgLastWalReplayLsn || row.pgLastXactReplayTimestamp || '-' }}</template>
              </el-table-column>
              <el-table-column label="检查时间" width="170">
                <template #default="{ row }">{{ row.checkedAt || row.createdAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="错误/风险" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ row.errorMessage || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="90" fixed="right">
                <template #default="{ row }">
                  <el-button link type="info" @click="handleViewReplicationRaw(row)">原始</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-wrapper">
              <el-pagination
                v-model:current-page="replicationCheckQuery.page"
                v-model:page-size="replicationCheckQuery.pageSize"
                :total="replicationCheckTotal"
                :page-sizes="[10, 20, 50]"
                layout="total, sizes, prev, pager, next"
                @size-change="loadReplicationChecks"
                @current-change="loadReplicationChecks"
              />
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="备份与恢复" name="backup">
        <div class="backup-panel">
          <el-alert
            title="P1 已支持外部物理备份、binlog/WAL 归档登记和 PITR 恢复计划预校验。定时备份执行仍以逻辑全量为主，大库生产主链路应改造为物理备份 + 日志连续归档。"
            type="warning"
            show-icon
            :closable="false"
          />
          <el-alert
            v-if="backupLargeWarnings.length"
            :title="`发现 ${backupLargeWarnings.length} 个大库逻辑全量风险任务，建议后续按专项方案改造为物理备份 + binlog/WAL 连续归档。`"
            type="error"
            show-icon
            :closable="false"
          >
            <div class="backup-risk-list">
              <span v-for="item in backupLargeWarnings.slice(0, 3)" :key="item.id">
                {{ item.name }}：{{ item.largeDataWarningText }}
              </span>
            </div>
          </el-alert>

          <div class="backup-card">
            <div class="panel-title">
              <span>逻辑备份任务</span>
              <el-tag size="small" type="info">{{ backupTaskTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-input
                  v-model="backupTaskQuery.keyword"
                  placeholder="搜索任务名称..."
                  clearable
                  class="audit-search-input"
                  @keyup.enter="loadBackupTasks"
                  @clear="loadBackupTasks"
                >
                  <template #prefix>
                    <el-icon><Search /></el-icon>
                  </template>
                </el-input>
                <el-select
                  v-model="backupTaskQuery.instanceId"
                  placeholder="实例"
                  clearable
                  filterable
                  class="audit-select"
                  @change="loadBackupTasks"
                >
                  <el-option
                    v-for="item in supportedBackupInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.dbTypeText || item.dbType}）`"
                    :value="item.id"
                  />
                </el-select>
                <el-select
                  v-model="backupTaskQuery.enabled"
                  placeholder="状态"
                  clearable
                  class="audit-select"
                  @change="loadBackupTasks"
                >
                  <el-option label="启用" value="enabled" />
                  <el-option label="禁用" value="disabled" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button @click="resetBackupTaskQuery">
                  <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                  重置
                </el-button>
                <el-button type="primary" @click="openBackupTaskDialog()">
                  <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                  新增任务
                </el-button>
              </div>
            </div>

            <el-table
              :data="backupTasks"
              v-loading="backupTaskLoading"
              stripe
              class="modern-table"
              :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
            >
              <el-table-column label="任务名称" min-width="180">
                <template #default="{ row }">
                  <div class="table-name-cell">
                    <span>{{ row.name }}</span>
                    <el-tag v-if="row.enabled" size="small" type="success">启用</el-tag>
                    <el-tag v-else size="small" type="info">禁用</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="实例" min-width="170">
                <template #default="{ row }">
                  <div class="instance-name">
                    <span>{{ row.instanceName || `#${row.instanceId}` }}</span>
                    <el-tag size="small" :type="dbTypeTag(row.instanceDbType)">{{ row.instanceDbTypeText || row.instanceDbType || '-' }}</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="策略" min-width="190">
                <template #default="{ row }">
                  <div class="backup-task-meta">
                    <span>{{ row.strategyText || row.backupTypeText || row.backupType || '-' }}</span>
                    <el-tag size="small" type="info">{{ row.restoreCapabilityText || '仅逻辑恢复' }}</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="PITR" width="130" align="center">
                <template #default="{ row }">
                  <el-tag :type="row.pitrSupported ? 'success' : 'info'" size="small">
                    {{ row.pitrStatusText || '不支持 PITR' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="容量提示" min-width="170">
                <template #default="{ row }">
                  <el-tooltip v-if="row.largeDataWarning" :content="row.largeDataWarningText" placement="top">
                    <el-tag type="danger" size="small">{{ row.instanceCapacitySizeText || '大库风险' }}</el-tag>
                  </el-tooltip>
                  <span v-else>{{ row.instanceCapacitySizeText || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="计划" min-width="150">
                <template #default="{ row }">{{ row.schedule || '仅手动' }}</template>
              </el-table-column>
              <el-table-column label="下次执行" width="170">
                <template #default="{ row }">{{ row.nextRunAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="保留天数" width="100" align="right">
                <template #default="{ row }">{{ row.retentionDays }}</template>
              </el-table-column>
              <el-table-column label="最近执行" width="170">
                <template #default="{ row }">{{ row.lastRunAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="最近成功" width="170">
                <template #default="{ row }">{{ row.lastSuccessAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="最近状态" width="110" align="center">
                <template #default="{ row }">
                  <el-tag :type="backupStatusTag(row.lastStatus)" size="small">
                    {{ row.lastStatusText || (row.lastStatus || '-') }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="最近结果" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ row.lastMessage || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="180" align="center" fixed="right">
                <template #default="{ row }">
                  <el-button link type="warning" :loading="runningBackupTaskId === row.id" @click="handleRunBackupTask(row)">
                    手动触发
                  </el-button>
                  <el-button link type="primary" @click="openBackupTaskDialog(row)">编辑</el-button>
                  <el-button link type="danger" @click="handleDeleteBackupTask(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-container">
              <el-pagination
                v-model:current-page="backupTaskQuery.page"
                v-model:page-size="backupTaskQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="backupTaskTotal"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="loadBackupTasks"
                @current-change="loadBackupTasks"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="panel-title">
              <span>备份记录</span>
              <el-tag size="small" type="info">{{ backupRecordTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select
                  v-model="backupRecordQuery.taskId"
                  placeholder="任务"
                  clearable
                  filterable
                  class="audit-search-input"
                  @change="loadBackupRecords"
                >
                  <el-option
                    v-for="item in backupTaskOptions"
                    :key="item.id"
                    :label="item.name"
                    :value="item.id"
                  />
                </el-select>
                <el-select
                  v-model="backupRecordQuery.instanceId"
                  placeholder="实例"
                  clearable
                  filterable
                  class="audit-select"
                  @change="loadBackupRecords"
                >
                  <el-option
                    v-for="item in supportedBackupInstances"
                    :key="item.id"
                    :label="item.name"
                    :value="item.id"
                  />
                </el-select>
                <el-select
                  v-model="backupRecordQuery.status"
                  placeholder="状态"
                  clearable
                  class="audit-select"
                  @change="loadBackupRecords"
                >
                  <el-option label="待执行" value="pending" />
                  <el-option label="队列中" value="queued" />
                  <el-option label="执行中" value="running" />
                  <el-option label="清理中" value="cleaning" />
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
                  <el-option label="已过期" value="expired" />
                </el-select>
                <el-select
                  v-model="backupRecordQuery.triggerType"
                  placeholder="触发方式"
                  clearable
                  class="audit-select"
                  @change="loadBackupRecords"
                >
                  <el-option label="手动触发" value="manual" />
                  <el-option label="定时触发" value="schedule" />
                  <el-option label="手动重试" value="manual_retry" />
                  <el-option label="外部登记" value="external" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button @click="resetBackupRecordQuery">
                  <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                  重置
                </el-button>
                <el-button type="success" plain @click="openExternalBackupDialog">
                  <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                  登记外部备份
                </el-button>
                <el-button type="primary" plain :loading="backupRecordLoading" @click="loadBackupRecords">
                  刷新记录
                </el-button>
              </div>
            </div>

            <el-table
              :data="backupRecords"
              v-loading="backupRecordLoading"
              stripe
              class="modern-table"
              :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
            >
              <el-table-column label="创建时间" prop="createdAt" width="170" />
              <el-table-column label="任务" min-width="170">
                <template #default="{ row }">{{ row.taskName || `#${row.taskId}` }}</template>
              </el-table-column>
              <el-table-column label="实例" min-width="150">
                <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
              </el-table-column>
              <el-table-column label="触发方式" width="110" align="center">
                <template #default="{ row }">{{ row.triggerTypeText || row.triggerType || '-' }}</template>
              </el-table-column>
              <el-table-column label="链路" min-width="170">
                <template #default="{ row }">
                  <div class="backup-task-meta">
                    <span>{{ row.backupMethodText || row.backupTypeText || row.backupType || '-' }}</span>
                    <div class="backup-inline-tags">
                      <el-tag size="small" type="info">{{ row.backupLevelText || row.backupLevel || '-' }}</el-tag>
                      <el-tag v-if="row.backupEngine" size="small" type="primary">{{ row.backupEngine }}</el-tag>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="backupStatusTag(row.status)" size="small">
                    {{ row.statusText || row.status || '-' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="文件" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.fileName || '-' }}</template>
              </el-table-column>
              <el-table-column label="文件大小" width="120" align="right">
                <template #default="{ row }">{{ row.fileSize ? formatBytes(row.fileSize) : '-' }}</template>
              </el-table-column>
              <el-table-column label="可恢复窗口" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ recoverableWindowText(row) }}</template>
              </el-table-column>
              <el-table-column label="校验" width="150" align="center">
                <template #default="{ row }">
                  <el-tooltip v-if="row.checksumSha256 || row.verifyMessage" :content="row.verifyMessage || row.checksumSha256" placement="top">
                    <el-tag size="small" :type="backupVerifyStatusTag(row.verifyStatus)">
                      {{ row.verifyStatusText || (row.checksumSha256 ? row.checksumSha256.slice(0, 8) : '-') }}
                    </el-tag>
                  </el-tooltip>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="过期时间" width="170" align="center">
                <template #default="{ row }">{{ row.expiresAt || '-' }}</template>
              </el-table-column>
              <el-table-column label="恢复演练" width="130" align="center">
                <template #default="{ row }">
                  <el-tag v-if="row.restoreTestStatus" size="small" :type="backupStatusTag(row.restoreTestStatus)">
                    {{ row.restoreTestStatusText || row.restoreTestStatus }}
                  </el-tag>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="耗时" width="100" align="right">
                <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
              </el-table-column>
              <el-table-column label="结果" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">{{ row.message || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="190" align="center" fixed="right">
                <template #default="{ row }">
                  <el-button
                    v-if="row.status === 'success' && row.fileName"
                    link
                    type="success"
                    :loading="verifyingBackupRecordId === row.id"
                    @click="handleVerifyBackupRecord(row)"
                  >
                    校验
                  </el-button>
                  <el-button
                    v-if="row.status === 'success' && row.fileName"
                    link
                    type="primary"
                    @click="handleDownloadBackupRecord(row)"
                  >
                    下载
                  </el-button>
                  <el-button
                    v-if="row.status === 'success' && row.fileName"
                    link
                    type="warning"
                    @click="openBackupRecordDrill(row)"
                  >
                    {{ isPgBaseBackupFullRecord(row) ? 'PITR演练' : '演练' }}
                  </el-button>
                  <span v-if="row.status !== 'success' || !row.fileName">-</span>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-container">
              <el-pagination
                v-model:current-page="backupRecordQuery.page"
                v-model:page-size="backupRecordQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="backupRecordTotal"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="loadBackupRecords"
                @current-change="loadBackupRecords"
              />
            </div>
          </div>

          <div class="backup-card">
            <div class="panel-title">
              <span>备份与恢复</span>
              <el-tag size="small" type="success">P5</el-tag>
            </div>
            <el-alert
              title="默认进入保护概览；底层备份策略、归档流、Runner、Barman、WAL 等资源已收敛到高级资源中。"
              type="info"
              show-icon
              :closable="false"
            />
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-button type="primary" plain @click="openProtectionWizardDialog()">
                  <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                  启用数据库保护
                </el-button>
                <el-button type="success" plain @click="openProtectionWizardDialog()">
                  保护向导
                </el-button>
                <el-button type="primary" plain @click="openPostgresBarmanWizardDialog()">
                  PostgreSQL Barman 向导
                </el-button>
                <el-button type="warning" plain @click="openRestorePlanDialog">
                  恢复演练
                </el-button>
              </div>
              <div class="backup-toolbar-group">
                <el-button :loading="protectionProfileLoading || protectionRiskLoading || storageProfileLoading || runnerHostLoading || runnerJobLoading || barmanServerLoading || backupPolicyLoading || logArchiveStreamLoading || logArchiveLoading || logArchiveEventLoading || restorePlanLoading" @click="refreshPITRState">
                  刷新保护状态
                </el-button>
              </div>
            </div>

            <el-tabs v-model="backupPitrTab" class="pitr-tabs">
              <el-tab-pane label="风险中心" name="protectionRisks">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="protectionRiskQuery.keyword"
                      placeholder="搜索实例、主机、负责人"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadProtectionRisks"
                      @clear="loadProtectionRisks"
                    />
                    <el-select v-model="protectionRiskQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadProtectionRisks">
                      <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="`${item.name}（${item.dbTypeText || item.dbType}）`" :value="item.id" />
                    </el-select>
                    <el-select v-model="protectionRiskQuery.engine" placeholder="引擎" clearable class="audit-select" @change="loadProtectionRisks">
                      <el-option label="MySQL" value="mysql" />
                      <el-option label="MariaDB" value="mariadb" />
                      <el-option label="PostgreSQL" value="postgresql" />
                    </el-select>
                    <el-select v-model="protectionRiskQuery.riskLevel" placeholder="风险" clearable class="audit-select" @change="loadProtectionRisks">
                      <el-option label="中" value="medium" />
                      <el-option label="高" value="high" />
                      <el-option label="严重" value="critical" />
                    </el-select>
                    <el-select v-model="protectionRiskQuery.issueType" placeholder="问题类型" clearable class="audit-search-input" @change="loadProtectionRisks">
                      <el-option label="缺少全量基线" value="missing_full_backup" />
                      <el-option label="增量链异常" value="incremental_chain_broken" />
                      <el-option label="日志链缺口" value="log_chain_gap" />
                      <el-option label="Runner 不可用" value="runner_offline" />
                      <el-option label="Runner 工具异常" value="runner_tool_missing" />
                      <el-option label="存储姿态异常" value="storage_posture_failed" />
                      <el-option label="缺少恢复演练" value="restore_drill_missing" />
                      <el-option label="恢复演练失败" value="restore_drill_failed" />
                      <el-option label="延迟副本不可用" value="replica_delay_unavailable" />
                      <el-option label="归档延迟过高" value="archive_lag_high" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetProtectionRiskQuery">重置</el-button>
                    <el-button type="primary" plain :loading="protectionRiskLoading" @click="loadProtectionRisks">刷新</el-button>
                  </div>
                </div>
                <el-table :data="protectionRisks" v-loading="protectionRiskLoading" stripe class="modern-table">
                  <el-table-column label="实例" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.instanceName || `#${row.instanceId}` }}</span>
                        <el-tag size="small" :type="dbTypeTag(row.engine)">{{ row.engineText || row.engine || '-' }}</el-tag>
                      </div>
                      <div class="muted-text">{{ row.endpoint || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="风险" min-width="180">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="riskLevelTag(row.riskLevel)">{{ row.riskLevelText || row.riskLevel || '-' }}</el-tag>
                        <el-tag size="small" type="warning">{{ row.issueTypeText || row.issueType || '-' }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="说明" min-width="320" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.message || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="保护状态" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>{{ row.protectionModeText || row.protectionMode || '-' }}</div>
                      <div class="muted-text">{{ row.protectionLevelText || row.protectionLevel || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="最近证明" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>Full：{{ row.lastFullAt || '-' }}</div>
                      <div class="muted-text">日志：{{ row.lastLogArchiveAt || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="建议动作" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.actionText || row.action || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="140" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="handleOpenProtectionRisk(row)">查看/修复</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="protectionRiskQuery.page"
                    v-model:page-size="protectionRiskQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="protectionRiskTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadProtectionRisks"
                    @current-change="loadProtectionRisks"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="保护概览" name="protectionOverview">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="protectionProfileQuery.keyword"
                      placeholder="搜索实例、主机、负责人"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadProtectionProfiles"
                      @clear="loadProtectionProfiles"
                    />
                    <el-select v-model="protectionProfileQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadProtectionProfiles">
                      <el-option
                        v-for="item in pitrBackupInstances"
                        :key="item.id"
                        :label="`${item.name}（${item.dbTypeText || item.dbType}）`"
                        :value="item.id"
                      />
                    </el-select>
                    <el-select v-model="protectionProfileQuery.engine" placeholder="引擎" clearable class="audit-select" @change="loadProtectionProfiles">
                      <el-option label="MySQL" value="mysql" />
                      <el-option label="MariaDB" value="mariadb" />
                      <el-option label="PostgreSQL" value="postgresql" />
                    </el-select>
                    <el-select v-model="protectionProfileQuery.protectionLevel" placeholder="保护等级" clearable class="audit-select" @change="loadProtectionProfiles">
                      <el-option label="未保护" value="none" />
                      <el-option label="仅备份" value="backup_only" />
                      <el-option label="PITR 可用" value="pitr_capable" />
                      <el-option label="PITR 已演练" value="pitr_verified" />
                      <el-option label="HA + PITR 已演练" value="ha_and_pitr_verified" />
                    </el-select>
                    <el-select v-model="protectionProfileQuery.riskLevel" placeholder="风险" clearable class="audit-select" @change="loadProtectionProfiles">
                      <el-option label="低" value="low" />
                      <el-option label="中" value="medium" />
                      <el-option label="高" value="high" />
                      <el-option label="严重" value="critical" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetProtectionProfileQuery">重置</el-button>
                    <el-button type="primary" plain :loading="protectionProfileLoading" @click="loadProtectionProfiles">刷新</el-button>
                  </div>
                </div>

                <el-table :data="protectionProfiles" v-loading="protectionProfileLoading" stripe class="modern-table">
                  <el-table-column label="实例" min-width="230">
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.instanceName || `#${row.instanceId}` }}</span>
                        <el-tag size="small" :type="dbTypeTag(row.engine)">{{ row.engineText || row.engine || '-' }}</el-tag>
                      </div>
                      <div class="muted-text">{{ row.endpoint || '-' }}<span v-if="row.version"> / {{ row.version }}</span></div>
                    </template>
                  </el-table-column>
                  <el-table-column label="保护模式" min-width="190">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="protectionModeTag(row.protectionMode)">{{ row.protectionModeText || row.protectionMode || '-' }}</el-tag>
                        <el-tag size="small" :type="protectionLevelTag(row.protectionLevel)">{{ row.protectionLevelText || row.protectionLevel || '-' }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="可恢复窗口" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ protectionRecoverableWindowText(row) }}</template>
                  </el-table-column>
                  <el-table-column label="最近备份" min-width="230" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>Full：{{ row.lastFullAt || '-' }}</div>
                      <div class="muted-text">Inc：{{ row.lastIncrementalAt || '-' }}</div>
                      <div class="muted-text">Synthetic：{{ row.lastSyntheticAt || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="日志归档" min-width="210" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>{{ row.lastLogArchiveAt || '-' }}</div>
                      <div class="muted-text">RPO：{{ protectionRPOText(row) }} / {{ row.logChainStatusText || '-' }}</div>
                      <div v-if="row.engine === 'postgresql'" class="muted-text">
                        TL：{{ row.timelineId || '-' }} / WAL缺口：{{ row.walGapCount || 0 }}
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="Runner / 存储" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>
                        <el-tag size="small" :type="runnerStatusTag(row.runnerStatus)">{{ row.runnerStatusText || row.runnerStatus || '-' }}</el-tag>
                        <span class="muted-text"> {{ row.runnerHost?.name || row.logArchiveStream?.runnerHostName || '-' }}</span>
                      </div>
                      <div class="muted-text">存储：{{ row.storageStatusText || row.storageStatus || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="恢复演练 / 副本" min-width="210" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>
                        <el-tag size="small" :type="restoreDrillStatusTag(row.restoreDrillStatus)">{{ row.restoreDrillStatusText || '-' }}</el-tag>
                        <span class="muted-text"> {{ row.lastRestoreDrillAt || '-' }}</span>
                      </div>
                      <div class="muted-text">副本：{{ row.replicaProtectionText || row.replicaProtectionStatus || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="风险" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="riskLevelTag(row.riskLevel)">{{ row.riskLevelText || row.riskLevel || '-' }}</el-tag>
                        <span class="muted-text">{{ (row.riskMessages || []).slice(0, 2).join('；') || '链路状态正常' }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="300" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="warning" :disabled="!row.backupPolicy" @click="handleRunProtectionProfileBackup(row, 'full')">立即备份</el-button>
                      <el-button link type="success" :disabled="!row.backupPolicy" @click="handleRunProtectionProfileBackup(row, 'incremental')">增量</el-button>
                      <el-button link type="primary" @click="openProtectionRestoreDrillDialog(row)">恢复演练</el-button>
                      <el-button link type="info" :loading="validatingProtectionProfileId === row.profileId" @click="handleValidateProtectionProfile(row)">校验</el-button>
                      <el-button link type="primary" @click="openProtectionProfileResources(row)">详情</el-button>
                      <el-button link type="danger" @click="openProtectionWizardDialog(row)">修复</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="protectionProfileQuery.page"
                    v-model:page-size="protectionProfileQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="protectionProfileTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadProtectionProfiles"
                    @current-change="loadProtectionProfiles"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="高级资源" name="advancedResources">
                <el-alert
                  title="这些是备份与恢复的底层资源。日常使用建议通过保护概览、保护策略向导和恢复演练入口操作。"
                  type="info"
                  show-icon
                  :closable="false"
                />
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-button type="primary" plain @click="openLogArchiveStreamDialog">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增归档流
                    </el-button>
                    <el-button type="success" plain @click="openLogArchiveDialog">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      登记日志归档
                    </el-button>
                    <el-button type="warning" plain @click="openRestorePlanDialog">
                      生成恢复计划
                    </el-button>
                    <el-button type="primary" plain @click="openBackupPolicyDialog()">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增备份策略
                    </el-button>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button :loading="storageProfileLoading || runnerHostLoading || runnerJobLoading || barmanServerLoading || backupPolicyLoading || logArchiveStreamLoading || logArchiveLoading || logArchiveEventLoading || restorePlanLoading" @click="refreshPITRState">
                      刷新高级资源
                    </el-button>
                  </div>
                </div>
                <el-tabs v-model="backupAdvancedTab" class="pitr-tabs">
              <el-tab-pane label="备份策略" name="backupPolicies">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="backupPolicyQuery.keyword"
                      placeholder="搜索策略名称"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadBackupPolicies"
                      @clear="loadBackupPolicies"
                    />
                    <el-select v-model="backupPolicyQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadBackupPolicies">
                      <el-option
                        v-for="item in mysqlPhysicalBackupPolicyInstances"
                        :key="item.id"
                        :label="`${item.name}（${item.dbTypeText || item.dbType}）`"
                        :value="item.id"
                      />
                    </el-select>
                    <el-select v-model="backupPolicyQuery.status" placeholder="策略状态" clearable class="audit-select" @change="loadBackupPolicies">
                      <el-option label="启用" value="active" />
                      <el-option label="降级" value="degraded" />
                      <el-option label="失败" value="failed" />
                      <el-option label="禁用" value="disabled" />
                    </el-select>
                    <el-select v-model="backupPolicyQuery.enabled" placeholder="启用状态" clearable class="audit-select" @change="loadBackupPolicies">
                      <el-option label="启用" value="enabled" />
                      <el-option label="禁用" value="disabled" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetBackupPolicyQuery">重置</el-button>
                    <el-button type="primary" plain @click="openBackupPolicyDialog()">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增策略
                    </el-button>
                    <el-button type="primary" plain :loading="backupPolicyLoading" @click="loadBackupPolicies">刷新</el-button>
                  </div>
                </div>

                <el-table :data="backupPolicies" v-loading="backupPolicyLoading" stripe class="modern-table">
                  <el-table-column label="策略" min-width="210">
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.name }}</span>
                        <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
                      </div>
                      <div class="muted-text">{{ row.instanceName || `#${row.instanceId}` }} / {{ row.instanceDbType || row.engine || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="执行源 / Runner" min-width="230" show-overflow-tooltip>
                    <template #default="{ row }">
                      {{ row.sourceRole || 'primary' }}
                      <span v-if="row.sourceInstanceId" class="muted-text"> / source #{{ row.sourceInstanceId }}</span>
                      <span class="muted-text"> / {{ row.runnerHostName || `Runner #${row.runnerHostId}` }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="工具 / binlog" min-width="210" show-overflow-tooltip>
                    <template #default="{ row }">
	                      {{ row.backupEngine || '-' }}
	                      <span class="muted-text"> / {{ row.binlogStreamId ? `binlog流 #${row.binlogStreamId}` : '未绑定binlog流' }}</span>
	                      <div class="muted-text">{{ row.toolExecutionMode === 'container_tools' ? `容器工具：${row.toolImage || '-'}` : '宿主机工具' }}</div>
	                    </template>
                  </el-table-column>
                  <el-table-column label="计划" min-width="230" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>Full：{{ row.fullSchedule || '仅手动' }}</div>
                      <div class="muted-text">Incremental：{{ row.incrementalSchedule || '仅手动' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="链状态" min-width="180">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="backupPolicyStatusTag(row.chain?.status || row.status)">
                          {{ row.chain?.statusText || row.statusText || row.status || '-' }}
                        </el-tag>
                        <span class="muted-text">base #{{ row.chain?.currentBaseRecordId || '-' }} / latest #{{ row.chain?.latestRecordId || '-' }}</span>
                        <span class="muted-text">增量 {{ row.chain?.incrementalCount || 0 }} 条</span>
                        <span v-if="backupPolicySyntheticRuleText(row)" class="muted-text">{{ backupPolicySyntheticRuleText(row) }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="下次运行" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>Full：{{ row.nextFullRunAt || '-' }}</div>
                      <div class="muted-text">Incremental：{{ row.nextIncrementalRunAt || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="最近结果" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div>
                        <el-tag size="small" :type="backupStatusTag(row.lastStatus || row.status)">{{ row.lastStatusText || row.lastStatus || '-' }}</el-tag>
                        <span class="muted-text"> {{ row.lastRunAt || '-' }}</span>
                      </div>
                      <div class="muted-text">{{ row.lastMessage || row.lastError || '-' }}</div>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="520" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="warning" :loading="runningBackupPolicyId === row.id && runningBackupPolicyLevel === 'full'" @click="handleRunBackupPolicy(row, 'full')">跑Full</el-button>
                      <el-button link type="success" :loading="runningBackupPolicyId === row.id && runningBackupPolicyLevel === 'incremental'" @click="handleRunBackupPolicy(row, 'incremental')">跑增量</el-button>
                      <el-button link type="primary" :loading="validatingBackupPolicyId === row.id" @click="handleValidateBackupPolicyChain(row)">校验链</el-button>
                      <el-button link type="warning" :disabled="!row.syntheticEnabled" :loading="previewingSyntheticPolicyId === row.id" @click="openSyntheticFullPreview(row)">合成预览</el-button>
                      <el-button link type="danger" :disabled="!row.syntheticEnabled" :loading="runningSyntheticPolicyId === row.id" @click="handleRunSyntheticFull(row)">合成Full</el-button>
                      <el-button link type="warning" :disabled="!row.syntheticEnabled" :loading="previewingPurgePolicyId === row.id" @click="openBackupPolicyPurgePreview(row)">清理预览</el-button>
                      <el-button link type="danger" :disabled="!row.syntheticEnabled" :loading="runningPurgePolicyId === row.id" @click="handleRunBackupPolicyPurge(row)">清理旧链</el-button>
                      <el-button link type="primary" @click="openBackupPolicyDialog(row)">编辑</el-button>
                      <el-button link type="danger" @click="handleDeleteBackupPolicy(row)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="backupPolicyQuery.page"
                    v-model:page-size="backupPolicyQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="backupPolicyTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadBackupPolicies"
                    @current-change="loadBackupPolicies"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="存储配置" name="storageProfiles">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="storageProfileQuery.keyword"
                      placeholder="搜索名称、bucket、前缀"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadStorageProfiles"
                      @clear="loadStorageProfiles"
                    />
                    <el-select v-model="storageProfileQuery.storageType" placeholder="类型" clearable class="audit-select" @change="loadStorageProfiles">
                      <el-option label="S3" value="s3" />
                      <el-option label="MinIO" value="minio" />
                      <el-option label="本地" value="local" />
                      <el-option label="NFS" value="nfs" />
                      <el-option label="外部" value="external" />
                    </el-select>
                    <el-select v-model="storageProfileQuery.status" placeholder="状态" clearable class="audit-select" @change="loadStorageProfiles">
                      <el-option label="待配置" value="pending" />
                      <el-option label="启用" value="enabled" />
                      <el-option label="禁用" value="disabled" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetStorageProfileQuery">重置</el-button>
                    <el-button type="primary" plain @click="openStorageProfileDialog">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增存储
                    </el-button>
                    <el-button type="primary" plain :loading="storageProfileLoading" @click="loadStorageProfiles">刷新</el-button>
                  </div>
                </div>
                <el-table :data="storageProfiles" v-loading="storageProfileLoading" stripe class="modern-table">
                  <el-table-column label="名称" min-width="160">
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.name }}</span>
                        <el-tag size="small" type="info">{{ row.storageTypeText || row.storageType }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="Bucket / 端点" min-width="240" show-overflow-tooltip>
                    <template #default="{ row }">
                      {{ row.bucket || '-' }}
                      <span v-if="row.endpoint" class="muted-text"> / {{ row.endpoint }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="路径前缀" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.pathPrefix || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="登记安全值" min-width="240" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="row.versioningEnabled ? 'success' : 'info'">版本化 {{ row.versioningEnabled ? '开' : '未要求' }}</el-tag>
                        <el-tag size="small" :type="row.immutabilityEnabled ? 'success' : 'info'">不可变 {{ row.immutabilityEnabled ? '开' : '未要求' }}</el-tag>
                        <el-tag v-if="row.kmsKeyId" size="small" type="success">KMS</el-tag>
                        <el-tag v-if="row.retentionLockDays" size="small" type="warning">{{ row.retentionLockDays }}天</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="检测姿态" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="storagePostureStatusTag(row.postureStatus)">
                        {{ row.postureStatusText || '未检测' }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="最近检测" width="170">
                    <template #default="{ row }">{{ row.lastPostureCheckAt || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="差异/风险摘要" min-width="300" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.postureSummary || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="150" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openStoragePostureDialog(row)">检测</el-button>
                      <el-button link type="info" :disabled="!row.postureJson" @click="openStoragePostureDetail(row)">详情</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="storageProfileQuery.page"
                    v-model:page-size="storageProfileQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="storageProfileTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadStorageProfiles"
                    @current-change="loadStorageProfiles"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="Runner主机" name="runnerHosts">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="runnerHostQuery.keyword"
                      placeholder="搜索 Runner / 主机"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadRunnerHosts"
                      @clear="loadRunnerHosts"
                    />
                    <el-select v-model="runnerHostQuery.runnerType" placeholder="类型" clearable class="audit-select" @change="loadRunnerHosts">
                      <el-option label="SSH Runner" value="ssh" />
                      <el-option label="本地 Runner" value="local" />
                      <el-option label="Agent Runner" value="agent" />
                    </el-select>
                    <el-select v-model="runnerHostQuery.status" placeholder="状态" clearable class="audit-select" @change="loadRunnerHosts">
                      <el-option label="待测试" value="pending" />
                      <el-option label="在线" value="online" />
                      <el-option label="失败" value="failed" />
                      <el-option label="已禁用" value="disabled" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button plain @click="openRunnerToolOfflinePackageDialog">离线包</el-button>
                    <el-button @click="resetRunnerHostQuery">重置</el-button>
                    <el-button type="primary" plain @click="openRunnerHostDialog">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增 Runner
                    </el-button>
                    <el-button type="primary" plain :loading="runnerHostLoading" @click="loadRunnerHosts">刷新</el-button>
                  </div>
                </div>
                <el-table :data="runnerHosts" v-loading="runnerHostLoading" stripe class="modern-table">
                  <el-table-column label="名称" min-width="160">
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.name }}</span>
                        <el-tag size="small" type="info">{{ row.runnerTypeText || row.runnerType }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="地址" min-width="170" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.host ? `${row.host}:${row.port || 22}` : '-' }}</template>
                  </el-table-column>
                  <el-table-column label="凭据" width="100" align="center">
                    <template #default="{ row }">{{ row.credentialId ? `#${row.credentialId}` : '-' }}</template>
                  </el-table-column>
                  <el-table-column label="工作目录" min-width="230" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.workDir || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="挂载点" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.storageMountPath || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="并发/超时" width="120" align="center">
                    <template #default="{ row }">{{ row.maxConcurrentJobs || 1 }} / {{ row.timeoutMinutes || 30 }}m</template>
                  </el-table-column>
                  <el-table-column label="状态" width="100" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="runnerStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="最近测试" width="170">
                    <template #default="{ row }">{{ row.lastTestAt || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="错误" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.lastError || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="430" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openRunnerHostDialog(row)">编辑</el-button>
                      <el-button link type="primary" @click="openRunnerAgentConfigDialog(row)">配置</el-button>
                      <el-button link type="info" @click="openLogArchiveEventsForRunner(row)">事件</el-button>
                      <el-button link type="success" :loading="runnerHostTestingId === row.id" @click="handleTestRunnerHost(row)">测试</el-button>
                      <el-button link type="warning" :loading="runnerToolProbingId === row.id" @click="handleProbeRunnerTools(row)">巡检</el-button>
                      <el-button link type="info" @click="openRunnerToolProfileDialog(row)">画像</el-button>
                      <el-button link type="primary" @click="openRunnerToolInstallScriptDialog(row)">脚本</el-button>
                      <el-button link type="danger" @click="handleDeleteRunnerHost(row)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="runnerHostQuery.page"
                    v-model:page-size="runnerHostQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="runnerHostTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadRunnerHosts"
                    @current-change="loadRunnerHosts"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="Runner任务" name="runnerJobs">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select v-model="runnerJobQuery.runnerHostId" placeholder="Runner" clearable filterable class="audit-search-input" @change="loadRunnerJobs">
                      <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select v-model="runnerJobQuery.jobType" placeholder="任务类型" clearable class="audit-select" @change="loadRunnerJobs">
                      <el-option label="Runner 探测" value="runner_probe" />
	                      <el-option label="Runner 工具巡检" value="runner_tool_probe" />
	                      <el-option label="Runner 工具安装" value="runner_tool_install" />
	                      <el-option label="Runner Agent 安装" value="runner_agent_install" />
	                      <el-option label="Runner Agent 升级" value="runner_agent_upgrade" />
	                      <el-option label="Runner Agent 重启" value="runner_agent_restart" />
	                      <el-option label="物理备份" value="physical_backup" />
                      <el-option label="MySQL 合成全量" value="mysql_synthetic_full" />
                      <el-option label="binlog 归档" value="binlog_archive" />
                      <el-option label="物理恢复" value="physical_restore" />
                      <el-option label="Barman 检查" value="barman_check" />
                      <el-option label="Barman Catalog 同步" value="barman_catalog_sync" />
                      <el-option label="Barman WAL 同步" value="barman_wal_sync" />
                      <el-option label="Barman 备份" value="barman_backup" />
                      <el-option label="Barman 恢复" value="barman_restore" />
                      <el-option label="pg_basebackup" value="pg_basebackup" />
                      <el-option label="pg_basebackup 恢复" value="pg_basebackup_restore" />
                    </el-select>
                    <el-select v-model="runnerJobQuery.status" placeholder="状态" clearable class="audit-select" @change="loadRunnerJobs">
                      <el-option label="排队中" value="queued" />
                      <el-option label="运行中" value="running" />
                      <el-option label="成功" value="success" />
                      <el-option label="失败" value="failed" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetRunnerJobQuery">重置</el-button>
                    <el-button type="primary" plain :loading="runnerJobLoading" @click="loadRunnerJobs">刷新</el-button>
                  </div>
                </div>
                <el-table :data="runnerJobs" v-loading="runnerJobLoading" stripe class="modern-table">
                  <el-table-column label="创建时间" prop="createdAt" width="170" />
                  <el-table-column label="Runner" min-width="150">
                    <template #default="{ row }">{{ row.runnerHostName || row.runnerId || `#${row.runnerHostId}` }}</template>
                  </el-table-column>
                  <el-table-column label="类型" width="120" align="center">
                    <template #default="{ row }">{{ row.jobTypeText || row.jobType || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="状态" width="100" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="runnerJobStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="命令类别" width="130">
                    <template #default="{ row }">{{ row.allowedCommand || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="摘要" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.commandSummary || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="耗时" width="110" align="right">
                    <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
                  </el-table-column>
                  <el-table-column label="退出码" width="90" align="center">
                    <template #default="{ row }">{{ row.exitCode ?? '-' }}</template>
                  </el-table-column>
                  <el-table-column label="输出" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ runnerJobOutputSummary(row) }}</template>
                  </el-table-column>
                  <el-table-column label="错误" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.errorMessage || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="90" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openRunnerJobDetail(row)">详情</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="runnerJobQuery.page"
                    v-model:page-size="runnerJobQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="runnerJobTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadRunnerJobs"
                    @current-change="loadRunnerJobs"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="Barman Server" name="barmanServers">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-input
                      v-model="barmanServerQuery.keyword"
                      placeholder="搜索名称 / Barman server"
                      clearable
                      class="audit-search-input"
                      @keyup.enter="loadBarmanServers"
                      @clear="loadBarmanServers"
                    />
                    <el-select v-model="barmanServerQuery.sourceInstanceId" placeholder="PostgreSQL实例" clearable filterable class="audit-search-input" @change="loadBarmanServers">
                      <el-option v-for="item in postgresqlBackupInstances" :key="item.id" :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`" :value="item.id" />
                    </el-select>
                    <el-select v-model="barmanServerQuery.runnerHostId" placeholder="Runner" clearable filterable class="audit-search-input" @change="loadBarmanServers">
                      <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select v-model="barmanServerQuery.status" placeholder="状态" clearable class="audit-select" @change="loadBarmanServers">
                      <el-option label="待检测" value="pending" />
                      <el-option label="健康" value="healthy" />
                      <el-option label="降级" value="degraded" />
                      <el-option label="失败" value="failed" />
                      <el-option label="已禁用" value="disabled" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetBarmanServerQuery">重置</el-button>
                    <el-button type="primary" plain @click="openBarmanServerDialog">
                      <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                      新增 Barman
                    </el-button>
                    <el-button type="primary" plain :loading="barmanServerLoading" @click="loadBarmanServers">刷新</el-button>
                  </div>
                </div>
                <el-table :data="barmanServers" v-loading="barmanServerLoading" stripe class="modern-table">
                  <el-table-column label="名称" min-width="180">
                    <template #default="{ row }">
                      <div class="backup-name-cell">
                        <span class="backup-name">{{ row.name }}</span>
                        <el-tag size="small" type="info">{{ row.barmanServerName }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="PostgreSQL 实例" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.sourceInstanceName || `#${row.sourceInstanceId}` }}</template>
                  </el-table-column>
                  <el-table-column label="Runner" min-width="160" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.runnerHostName || `#${row.runnerHostId}` }}</template>
                  </el-table-column>
                  <el-table-column label="保留/方法" min-width="190" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.retentionPolicy || '-' }} / {{ row.backupMethod || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="WAL归档" width="150">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="row.archiverEnabled ? 'success' : 'info'">archive {{ row.archiverEnabled ? '开' : '关' }}</el-tag>
                        <el-tag size="small" :type="row.streamingArchiverEnabled ? 'success' : 'info'">stream {{ row.streamingArchiverEnabled ? '开' : '关' }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="PG元数据" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      {{ row.pgSystemIdentifier || '-' }}
                      <span v-if="row.pgVersion" class="muted-text"> / {{ row.pgVersion }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="状态" width="100" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="barmanStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="最近检查" width="170">
                    <template #default="{ row }">{{ row.lastCheckAt || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="最近同步" width="190">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <span>Catalog {{ row.lastCatalogSyncAt || '-' }}</span>
                        <span>WAL {{ row.lastWalSyncAt || '-' }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="错误" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.lastError || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="360" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openBarmanServerDialog(row)">编辑</el-button>
                      <el-button link type="success" :loading="barmanCheckingId === row.id" @click="handleCheckBarmanServer(row)">检查</el-button>
                      <el-button link type="warning" :loading="barmanCatalogSyncingId === row.id" @click="handleSyncBarmanCatalog(row)">同步</el-button>
                      <el-button link type="warning" :loading="barmanWalSyncingId === row.id" @click="handleSyncBarmanWAL(row)">同步WAL</el-button>
                      <el-button link type="danger" :loading="barmanBackingUpId === row.id" @click="handleBackupBarmanServer(row)">备份</el-button>
                      <el-button link type="danger" @click="handleDeleteBarmanServer(row)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="barmanServerQuery.page"
                    v-model:page-size="barmanServerQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="barmanServerTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadBarmanServers"
                    @current-change="loadBarmanServers"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="归档流" name="streams">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select v-model="logArchiveStreamQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadLogArchiveStreams">
                      <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-select v-model="logArchiveStreamQuery.archiveType" placeholder="类型" clearable class="audit-select" @change="loadLogArchiveStreams">
                      <el-option label="binlog" value="binlog" />
                      <el-option label="WAL" value="wal" />
                    </el-select>
                    <el-select v-model="logArchiveStreamQuery.status" placeholder="状态" clearable class="audit-select" @change="loadLogArchiveStreams">
                      <el-option label="待配置" value="pending" />
                      <el-option label="运行中" value="running" />
                      <el-option label="已暂停" value="paused" />
                      <el-option label="降级" value="degraded" />
                      <el-option label="失败" value="failed" />
                      <el-option label="禁用" value="disabled" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetLogArchiveStreamQuery">重置</el-button>
                    <el-button type="primary" plain :loading="logArchiveStreamLoading" @click="loadLogArchiveStreams">刷新</el-button>
                  </div>
                </div>
                <el-table :data="logArchiveStreams" v-loading="logArchiveStreamLoading" stripe class="modern-table">
                  <el-table-column label="实例" min-width="160">
                    <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
                  </el-table-column>
                  <el-table-column label="来源" min-width="140">
                    <template #default="{ row }">{{ row.sourceInstanceName || row.sourceInstanceId || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="日志类型" width="110" align="center">
                    <template #default="{ row }">{{ row.archiveTypeText || row.archiveType || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="归档引擎" min-width="140">
                    <template #default="{ row }">{{ row.archiveEngine || row.archiveModeText || row.archiveMode || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="Runner" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.runnerHostName || (row.runnerHostId ? `#${row.runnerHostId}` : '-') }}</template>
                  </el-table-column>
                  <el-table-column label="RPO" width="90" align="right">
                    <template #default="{ row }">{{ row.rpoTargetSeconds || 0 }}s</template>
                  </el-table-column>
                  <el-table-column label="状态" width="105" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="logArchiveStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="期望/守护" min-width="150">
                    <template #default="{ row }">
                      <div class="pitr-state-stack">
                        <el-tag size="small" :type="logArchiveDesiredStateTag(row.desiredState)">{{ row.desiredStateText || row.desiredState || '-' }}</el-tag>
                        <el-tag size="small" :type="logArchiveDaemonStatusTag(row.daemonStatus)">{{ row.daemonStatusText || row.daemonStatus || '-' }}</el-tag>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="Cursor" min-width="190" show-overflow-tooltip>
                    <template #default="{ row }">{{ formatLogArchiveCursor(row) }}</template>
                  </el-table-column>
                  <el-table-column label="延迟/心跳" min-width="170" show-overflow-tooltip>
                    <template #default="{ row }">
                      {{ formatLogArchiveLag(row) }} / {{ row.lastHeartbeatAt || '-' }}
                    </template>
                  </el-table-column>
                  <el-table-column label="最近归档" min-width="170">
                    <template #default="{ row }">{{ row.lastArchivedAt || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="最近文件" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.lastArchiveName || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="错误" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.lastError || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="320" align="center" fixed="right">
                    <template #default="{ row }">
                      <el-button
                        link
                        type="success"
                        :disabled="row.archiveType !== 'binlog' || !row.enabled || row.desiredState === 'running'"
                        @click="openStartLogArchiveStreamDialog(row)"
                      >
                        启动
                      </el-button>
                      <el-button
                        link
                        type="warning"
                        :disabled="!row.enabled || row.desiredState !== 'running'"
                        @click="pauseLogArchiveStream(row)"
                      >
                        暂停
                      </el-button>
                      <el-button
                        link
                        type="primary"
                        :disabled="!row.enabled || row.desiredState !== 'paused'"
                        @click="resumeLogArchiveStream(row)"
                      >
                        恢复
                      </el-button>
                      <el-button
                        link
                        type="danger"
                        :disabled="row.desiredState === 'stopped'"
                        @click="stopLogArchiveStream(row)"
                      >
                        停止
                      </el-button>
                      <el-button
                        link
                        type="primary"
                        :disabled="row.archiveType !== 'binlog' || !row.enabled"
                        @click="openRunLogArchiveOnceDialog(row)"
                      >
                        归档一次
                      </el-button>
                      <el-button
                        link
                        type="warning"
                        :disabled="row.archiveType !== 'binlog' || !row.enabled"
                        @click="openRunLogArchiveCatchUpDialog(row)"
                      >
                        追平
                      </el-button>
                      <el-button link type="info" @click="openLogArchiveEventsForStream(row)">事件</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="logArchiveStreamQuery.page"
                    v-model:page-size="logArchiveStreamQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="logArchiveStreamTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadLogArchiveStreams"
                    @current-change="loadLogArchiveStreams"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="日志归档" name="archives">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select v-model="logArchiveQuery.streamId" placeholder="归档流" clearable filterable class="audit-search-input" @change="loadLogArchives">
                      <el-option v-for="item in logArchiveStreamOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select v-model="logArchiveQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadLogArchives">
                      <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-select v-model="logArchiveQuery.archiveType" placeholder="类型" clearable class="audit-select" @change="loadLogArchives">
                      <el-option label="binlog" value="binlog" />
                      <el-option label="WAL" value="wal" />
                    </el-select>
                    <el-select v-model="logArchiveQuery.status" placeholder="状态" clearable class="audit-select" @change="loadLogArchives">
                      <el-option label="已归档" value="archived" />
                      <el-option label="缺失" value="missing" />
                      <el-option label="校验失败" value="checksum_failed" />
                      <el-option label="已过期" value="expired" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetLogArchiveQuery">重置</el-button>
                    <el-button type="primary" plain :loading="logArchiveLoading" @click="loadLogArchives">刷新</el-button>
                  </div>
                </div>
                <el-table :data="logArchives" v-loading="logArchiveLoading" stripe class="modern-table">
                  <el-table-column label="时间范围" min-width="310">
                    <template #default="{ row }">{{ row.firstEventTime || '-' }} 至 {{ row.lastEventTime || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="实例" min-width="150">
                    <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
                  </el-table-column>
                  <el-table-column label="类型" width="100" align="center">
                    <template #default="{ row }">{{ row.archiveTypeText || row.archiveType || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="文件" min-width="200" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.fileName || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="PG WAL" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">
                      <span v-if="row.archiveType === 'wal'">
                        TLI {{ row.timelineId || '-' }} / #{{ row.segmentNo || '-' }}
                      </span>
                      <span v-else>-</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="大小" width="110" align="right">
                    <template #default="{ row }">{{ row.fileSize ? formatBytes(row.fileSize) : '-' }}</template>
                  </el-table-column>
                  <el-table-column label="状态" width="110" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="logArchiveStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="存储" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.storageUri || '-' }}</template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="logArchiveQuery.page"
                    v-model:page-size="logArchiveQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="logArchiveTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadLogArchives"
                    @current-change="loadLogArchives"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="Agent事件" name="events">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select v-model="logArchiveEventQuery.streamId" placeholder="归档流" clearable filterable class="audit-search-input" @change="loadLogArchiveEvents">
                      <el-option v-for="item in logArchiveStreamOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select v-model="logArchiveEventQuery.runnerHostId" placeholder="Runner" clearable filterable class="audit-select" @change="loadLogArchiveEvents">
                      <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select v-model="logArchiveEventQuery.level" placeholder="级别" clearable class="audit-select" @change="loadLogArchiveEvents">
                      <el-option label="信息" value="info" />
                      <el-option label="警告" value="warning" />
                      <el-option label="错误" value="error" />
                    </el-select>
                    <el-select v-model="logArchiveEventQuery.eventType" placeholder="事件类型" clearable class="audit-select" @change="loadLogArchiveEvents">
                      <el-option label="租约获取" value="lease_acquired" />
                      <el-option label="Checkpoint" value="checkpoint" />
                      <el-option label="状态变更" value="state_changed" />
                      <el-option label="归档成功" value="archive_success" />
                      <el-option label="归档失败" value="archive_failed" />
                      <el-option label="Spool 更新" value="spool_updated" />
                      <el-option label="日志断链" value="purge_gap" />
                      <el-option label="Agent 消息" value="agent_message" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetLogArchiveEventQuery">重置</el-button>
                    <el-button type="primary" plain :loading="logArchiveEventLoading" @click="loadLogArchiveEvents">刷新</el-button>
                  </div>
                </div>
                <el-table :data="logArchiveEvents" v-loading="logArchiveEventLoading" stripe class="modern-table">
                  <el-table-column label="时间" prop="occurredAt" width="170" />
                  <el-table-column label="级别" width="90" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="logArchiveEventLevelTag(row.level)">{{ row.levelText || row.level || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="类型" width="120">
                    <template #default="{ row }">{{ row.eventTypeText || row.eventType || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="实例/Runner" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.instanceName || `#${row.instanceId || '-'}` }} / {{ row.runnerHostName || row.runnerId || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="文件" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.fileName || row.activeFile || row.cursorFile || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="游标/延迟" min-width="160" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.cursorFile ? `${row.cursorFile}:${row.cursorPos || 0}` : '-' }} / {{ row.archiveLagSeconds || 0 }}s</template>
                  </el-table-column>
                  <el-table-column label="消息" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.message || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="摘要" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.payloadJson || '-' }}</template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="logArchiveEventQuery.page"
                    v-model:page-size="logArchiveEventQuery.pageSize"
                    :page-sizes="[20, 50, 100]"
                    :total="logArchiveEventTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadLogArchiveEvents"
                    @current-change="loadLogArchiveEvents"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="恢复计划" name="plans">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select v-model="restorePlanQuery.sourceInstanceId" placeholder="来源实例" clearable filterable class="audit-select" @change="loadRestorePlans">
                      <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-select v-model="restorePlanQuery.targetInstanceId" placeholder="目标实例" clearable filterable class="audit-select" @change="loadRestorePlans">
                      <el-option v-for="item in pitrRestoreTargetInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-select v-model="restorePlanQuery.validationStatus" placeholder="预校验" clearable class="audit-select" @change="loadRestorePlans">
                      <el-option label="通过" value="passed" />
                      <el-option label="警告" value="warning" />
                      <el-option label="失败" value="failed" />
                      <el-option label="待校验" value="pending" />
                    </el-select>
                    <el-select v-model="restorePlanQuery.restoreStatus" placeholder="恢复状态" clearable class="audit-select" @change="loadRestorePlans">
                      <el-option label="已规划" value="planned" />
                      <el-option label="执行中" value="running" />
                      <el-option label="已恢复" value="restored" />
                      <el-option label="已验证" value="verified" />
                      <el-option label="失败" value="failed" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetRestorePlanQuery">重置</el-button>
                    <el-button type="primary" plain :loading="restorePlanLoading" @click="loadRestorePlans">刷新</el-button>
                  </div>
                </div>
                <el-table :data="restorePlans" v-loading="restorePlanLoading" stripe class="modern-table">
                  <el-table-column label="创建时间" prop="createdAt" width="170" />
                  <el-table-column label="来源实例" min-width="150">
                    <template #default="{ row }">{{ row.sourceInstanceName || `#${row.sourceInstanceId}` }}</template>
                  </el-table-column>
                  <el-table-column label="目标实例" min-width="150">
                    <template #default="{ row }">{{ row.targetInstanceName || (row.targetInstanceId ? `#${row.targetInstanceId}` : '-') }}</template>
                  </el-table-column>
                  <el-table-column label="目标时间" min-width="170">
                    <template #default="{ row }">{{ row.restoreTargetValue || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="备份链" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreValidationStatusTag(row.backupChainStatus)">{{ row.backupChainStatusText || row.backupChainStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="日志链" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreValidationStatusTag(row.logChainStatus)">{{ row.logChainStatusText || row.logChainStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="存储" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreValidationStatusTag(row.storageStatus)">{{ row.storageStatusText || row.storageStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="Artifact" width="120" align="center">
                    <template #default="{ row }">
                      <el-tooltip :content="restorePlanArtifactReadiness(row).message" placement="top">
                        <el-tag size="small" :type="restorePlanArtifactReadiness(row).tag">{{ restorePlanArtifactReadiness(row).text }}</el-tag>
                      </el-tooltip>
                    </template>
                  </el-table-column>
                  <el-table-column label="工具" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreValidationStatusTag(row.toolStatus)">{{ row.toolStatusText || row.toolStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="预校验" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreValidationStatusTag(row.validationStatus)">{{ row.validationStatusText || row.validationStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="恢复状态" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="restoreStatusTag(row.restoreStatus)">{{ row.restoreStatusText || row.restoreStatus || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="Runner" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.runnerHostName || (row.runnerHostId ? `#${row.runnerHostId}` : '-') }}</template>
                  </el-table-column>
                  <el-table-column label="结果" min-width="260" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.message || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="170" fixed="right">
                    <template #default="{ row }">
                      <el-button
                        size="small"
                        type="warning"
                        link
                        :disabled="row.validationStatus !== 'passed' || ['queued', 'running'].includes(row.restoreStatus) || restorePlanArtifactReadiness(row).blocking"
                        @click="openRunRestorePlanDialog(row)"
                      >
                        执行恢复
                      </el-button>
                      <el-button size="small" type="primary" link :disabled="!row.proofJson" @click="viewRestoreProof(row)">
                        Proof
                      </el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="restorePlanQuery.page"
                    v-model:page-size="restorePlanQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="restorePlanTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadRestorePlans"
                    @current-change="loadRestorePlans"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="Barman Catalog" name="barmanCatalog">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select
                      v-model="barmanCatalogQuery.instanceId"
                      placeholder="PostgreSQL实例"
                      clearable
                      filterable
                      class="audit-search-input"
                      @change="loadBarmanCatalogRecords"
                    >
                      <el-option v-for="item in postgresqlBackupInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                    <el-select
                      v-model="barmanCatalogQuery.backupEngine"
                      placeholder="备份引擎"
                      class="audit-select"
                      @change="loadBarmanCatalogRecords"
                    >
                      <el-option label="Barman" value="barman" />
                      <el-option label="pg_basebackup" value="pg_basebackup" />
                      <el-option label="WAL-G (external)" value="walg" />
                      <el-option label="pgBackRest (legacy)" value="pgbackrest" />
                    </el-select>
                    <el-select
                      v-model="barmanCatalogQuery.status"
                      placeholder="状态"
                      clearable
                      class="audit-select"
                      @change="loadBarmanCatalogRecords"
                    >
                      <el-option label="成功" value="success" />
                      <el-option label="执行中" value="running" />
                      <el-option label="失败" value="failed" />
                      <el-option label="已过期" value="expired" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetBarmanCatalogQuery">重置</el-button>
                    <el-button type="primary" plain :loading="barmanCatalogLoading" @click="loadBarmanCatalogRecords">刷新</el-button>
                  </div>
                </div>
                <el-alert
                  v-if="!barmanCatalogLoading && barmanCatalogRecords.length === 0"
                  title="暂未同步到任何 PostgreSQL 物理备份记录。请在 Barman Server 页签触发『检查』和『同步』，或登记外部 WAL-G/pgBackRest 备份。"
                  type="info"
                  show-icon
                  :closable="false"
                  style="margin-bottom: 12px;"
                />
                <el-table :data="barmanCatalogRecords" v-loading="barmanCatalogLoading" stripe class="modern-table">
                  <el-table-column label="备份ID" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="pitr-catalog-meta">
                        <span class="restore-step-name">{{ row.externalBackupId || `#${row.id}` }}</span>
                        <span v-if="row.externalServerName" class="muted-text">{{ row.externalServerName }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="实例" min-width="160">
                    <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
                  </el-table-column>
                  <el-table-column label="引擎 / 范围" min-width="160">
                    <template #default="{ row }">
                      <div class="pitr-catalog-meta">
                        <el-tag size="small" type="primary">{{ row.backupEngine || '-' }}</el-tag>
                        <span class="muted-text">{{ row.backupScope || 'cluster' }} / {{ row.backupLevelText || row.backupLevel || '-' }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="状态" width="100" align="center">
                    <template #default="{ row }">
                      <el-tag :type="backupStatusTag(row.status)" size="small">{{ row.statusText || row.status || '-' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="System Identifier" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">{{ row.pgSystemIdentifier || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="Timeline" width="110" align="center">
                    <template #default="{ row }">{{ row.timelineId || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="LSN 范围" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="pitr-catalog-meta">
                        <span>{{ row.startLsn || '-' }} → {{ row.endLsn || '-' }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="WAL 范围" min-width="320" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="pitr-catalog-meta">
                        <span>{{ row.walStart || '-' }}</span>
                        <span class="muted-text">→ {{ row.walEnd || '-' }}</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="Manifest checksum" min-width="180" show-overflow-tooltip>
                    <template #default="{ row }">
                      <span v-if="row.backupManifestChecksum" class="muted-text">{{ row.backupManifestChecksum }}</span>
                      <span v-else class="muted-text">-</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="可恢复窗口" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ recoverableWindowText(row) }}</template>
                  </el-table-column>
                  <el-table-column label="同步状态" width="120" align="center">
                    <template #default="{ row }">
                      <el-tag size="small" :type="barmanCatalogSyncTag(row).tag">{{ barmanCatalogSyncTag(row).text }}</el-tag>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="pagination-container">
                  <el-pagination
                    v-model:current-page="barmanCatalogQuery.page"
                    v-model:page-size="barmanCatalogQuery.pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    :total="barmanCatalogTotal"
                    layout="total, sizes, prev, pager, next, jumper"
                    @size-change="loadBarmanCatalogRecords"
                    @current-change="loadBarmanCatalogRecords"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane label="WAL 状态" name="walStatus">
                <div class="backup-toolbar pitr-sub-toolbar">
                  <div class="backup-toolbar-group">
                    <el-select
                      v-model="walStatusQuery.streamId"
                      placeholder="归档流"
                      clearable
                      filterable
                      class="audit-search-input"
                      @change="loadWalStatusArchives"
                    >
                      <el-option v-for="item in walLogArchiveStreamOptions" :key="item.id" :label="item.label" :value="item.id" />
                    </el-select>
                    <el-select
                      v-model="walStatusQuery.instanceId"
                      placeholder="实例"
                      clearable
                      filterable
                      class="audit-select"
                      @change="loadWalStatusArchives"
                    >
                      <el-option v-for="item in postgresqlBackupInstances" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                  </div>
                  <div class="backup-toolbar-group">
                    <el-button @click="resetWalStatusQuery">重置</el-button>
                    <el-button type="primary" plain :loading="walStatusLoading" @click="loadWalStatusArchives">刷新</el-button>
                  </div>
                </div>
                <el-alert
                  title="本视图按归档流分组展示已登记 WAL segment 链路；segment 序号不连续会标记『缺口』，timeline 切换会突出显示。WAL 文件的实际同步动作仍在 Barman Server 页签触发。"
                  type="info"
                  show-icon
                  :closable="false"
                  style="margin-bottom: 12px;"
                />
                <el-empty
                  v-if="!walStatusLoading && walStatusGroups.length === 0"
                  class="pitr-wal-stream-empty"
                  description="暂无 WAL 归档记录"
                />
                <div v-else>
                  <div
                    v-for="group in walStatusGroups"
                    :key="group.streamId"
                    class="pitr-wal-stream-card"
                    v-loading="walStatusLoading"
                  >
                    <div class="pitr-wal-stream-header">
                      <div class="pitr-wal-stream-title">
                        <span>归档流 #{{ group.streamId }}</span>
                        <el-tag v-if="group.instanceName" size="small" type="info">{{ group.instanceName }}</el-tag>
                        <el-tag v-if="group.timelineCount > 1" size="small" type="warning">timeline 切换 ×{{ group.timelineCount - 1 }}</el-tag>
                        <el-tag v-if="group.gapCount > 0" size="small" type="danger">缺口 ×{{ group.gapCount }}</el-tag>
                        <el-tag v-else size="small" type="success">链路连续</el-tag>
                      </div>
                      <div class="pitr-wal-stream-stats">
                        <span>共 {{ group.segments.length }} 段</span>
                        <span>时间窗口：{{ group.timeRange }}</span>
                        <span v-if="group.lsnRange">LSN：{{ group.lsnRange }}</span>
                      </div>
                    </div>
                    <el-table :data="group.segments" stripe size="small">
                      <el-table-column label="WAL 文件" min-width="240" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.fileName || '-' }}</template>
                      </el-table-column>
                      <el-table-column label="Timeline" width="100" align="center">
                        <template #default="{ row }">{{ row.timelineId || '-' }}</template>
                      </el-table-column>
                      <el-table-column label="Segment" width="120" align="right">
                        <template #default="{ row }">#{{ row.segmentNo || '-' }}</template>
                      </el-table-column>
                      <el-table-column label="LSN 范围" min-width="220" show-overflow-tooltip>
                        <template #default="{ row }">
                          <span>{{ row.startLsn || '-' }} → {{ row.endLsn || '-' }}</span>
                        </template>
                      </el-table-column>
                      <el-table-column label="时间范围" min-width="280" show-overflow-tooltip>
                        <template #default="{ row }">{{ row.firstEventTime || '-' }} → {{ row.lastEventTime || '-' }}</template>
                      </el-table-column>
                      <el-table-column label="大小" width="110" align="right">
                        <template #default="{ row }">{{ row.fileSize ? formatBytes(row.fileSize) : '-' }}</template>
                      </el-table-column>
                      <el-table-column label="状态" width="120" align="center">
                        <template #default="{ row }">
                          <el-tag size="small" :type="logArchiveStatusTag(row.status)">{{ row.statusText || row.status || '-' }}</el-tag>
                          <el-tag v-if="row.gapBefore" size="small" type="danger" style="margin-left: 4px;">缺口</el-tag>
                          <el-tag v-if="row.timelineSwitch" size="small" type="warning" style="margin-left: 4px;">TL切换</el-tag>
                        </template>
                      </el-table-column>
                    </el-table>
                  </div>
                </div>
              </el-tab-pane>
            </el-tabs>
              </el-tab-pane>
            </el-tabs>
          </div>

          <div class="backup-card">
            <div class="panel-title">
              <span>恢复演练记录</span>
              <el-tag size="small" type="info">{{ restoreJobTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select
                  v-model="restoreJobQuery.sourceInstanceId"
                  placeholder="来源实例"
                  clearable
                  filterable
                  class="audit-select"
                  @change="loadRestoreJobs"
                >
                  <el-option
                    v-for="item in supportedBackupInstances"
                    :key="item.id"
                    :label="item.name"
                    :value="item.id"
                  />
                </el-select>
                <el-select
                  v-model="restoreJobQuery.targetInstanceId"
                  placeholder="目标实例"
                  clearable
                  filterable
                  class="audit-select"
                  @change="loadRestoreJobs"
                >
                  <el-option
                    v-for="item in restoreTargetInstances"
                    :key="item.id"
                    :label="item.name"
                    :value="item.id"
                  />
                </el-select>
                <el-select
                  v-model="restoreJobQuery.status"
                  placeholder="状态"
                  clearable
                  class="audit-select"
                  @change="loadRestoreJobs"
                >
                  <el-option label="待执行" value="pending" />
                  <el-option label="执行中" value="running" />
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button @click="resetRestoreJobQuery">
                  <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                  重置
                </el-button>
                <el-button type="primary" plain :loading="restoreJobLoading" @click="loadRestoreJobs">
                  刷新记录
                </el-button>
              </div>
            </div>

            <el-table
              :data="restoreJobs"
              v-loading="restoreJobLoading"
              stripe
              class="modern-table"
              :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
            >
              <el-table-column label="创建时间" prop="createdAt" width="170" />
              <el-table-column label="来源实例" min-width="150">
                <template #default="{ row }">{{ row.sourceInstanceName || `#${row.sourceInstanceId}` }}</template>
              </el-table-column>
              <el-table-column label="目标实例" min-width="170">
                <template #default="{ row }">
                  {{ row.targetInstanceName || `#${row.targetInstanceId}` }}
                  <el-tag v-if="row.targetEnvironment" size="small" type="info" style="margin-left: 6px;">
                    {{ row.targetEnvironment }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="Runner" min-width="150" show-overflow-tooltip>
                <template #default="{ row }">{{ row.runnerHostName || (row.runnerHostId ? `#${row.runnerHostId}` : '-') }}</template>
              </el-table-column>
              <el-table-column label="模式" width="110" align="center">
                <template #default="{ row }">{{ row.restoreModeText || row.restoreMode || '-' }}</template>
              </el-table-column>
              <el-table-column label="策略" width="150" align="center">
                <template #default="{ row }">{{ row.restoreStrategyText || row.restoreStrategy || '-' }}</template>
              </el-table-column>
              <el-table-column label="状态" width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="backupStatusTag(row.status)" size="small">
                    {{ row.statusText || row.status || '-' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="文件" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.fileName || '-' }}</template>
              </el-table-column>
              <el-table-column label="隔离库" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  <span v-if="row.listenPort">{{ row.listenHost || '127.0.0.1' }}:{{ row.listenPort }}</span>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="容器" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ row.containerName || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作者" width="120">
                <template #default="{ row }">{{ row.operatorName || '-' }}</template>
              </el-table-column>
              <el-table-column label="耗时" width="100" align="right">
                <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
              </el-table-column>
              <el-table-column label="结果" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">{{ row.message || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="220" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" link @click="openRestoreJobDetail(row)">详情</el-button>
                  <el-button size="small" type="primary" link :disabled="!row.proofJson" @click="viewRestoreProof(row)">Proof</el-button>
                  <el-button
                    size="small"
                    type="warning"
                    link
                    :disabled="row.status === 'running' || row.cleanupStatus === 'cleaned'"
                    @click="cleanupRestoreJob(row)"
                  >
                    清理
                  </el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-container">
              <el-pagination
                v-model:current-page="restoreJobQuery.page"
                v-model:page-size="restoreJobQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="restoreJobTotal"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="loadRestoreJobs"
                @current-change="loadRestoreJobs"
              />
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="巡检报告" name="inspection">
        <div class="backup-panel">
          <el-alert
            title="巡检报告会聚合容量趋势、性能诊断、安全审计和备份状态，首批支持手动生成并落库留痕。"
            type="info"
            show-icon
            :closable="false"
          />

          <div class="backup-card">
            <div class="panel-title">
              <span>生成报告</span>
              <el-tag size="small" type="success">manual</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <span class="toolbar-label">数据库实例</span>
                <el-select
                  v-model="inspectionForm.instanceId"
                  placeholder="请选择实例"
                  filterable
                  class="metadata-instance-select"
                >
                  <el-option
                    v-for="item in diagnosisInstances"
                    :key="item.id"
                    :label="`${item.name}（${item.dbTypeText || item.dbType} / ${item.endpoint || `${item.host}:${item.port}`}）`"
                    :value="item.id"
                  />
                </el-select>
              </div>
              <el-button type="primary" :disabled="!inspectionForm.instanceId" :loading="inspectionGenerating" @click="handleGenerateInspectionReport">
                生成巡检报告
              </el-button>
            </div>
          </div>

          <div class="backup-card">
            <div class="panel-title">
              <span>报告记录</span>
              <el-tag size="small" type="info">{{ inspectionTotal }}</el-tag>
            </div>
            <div class="backup-toolbar">
              <div class="backup-toolbar-group">
                <el-select
                  v-model="inspectionQuery.instanceId"
                  placeholder="实例"
                  clearable
                  filterable
                  class="audit-select"
                  @change="loadInspectionReports"
                >
                  <el-option v-for="item in diagnosisInstances" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
                <el-select
                  v-model="inspectionQuery.riskLevel"
                  placeholder="风险"
                  clearable
                  class="audit-select"
                  @change="loadInspectionReports"
                >
                  <el-option label="低" value="low" />
                  <el-option label="中" value="medium" />
                  <el-option label="高" value="high" />
                  <el-option label="严重" value="critical" />
                </el-select>
                <el-select
                  v-model="inspectionQuery.status"
                  placeholder="状态"
                  clearable
                  class="audit-select"
                  @change="loadInspectionReports"
                >
                  <el-option label="执行中" value="running" />
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button @click="resetInspectionQuery">
                  <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                  重置
                </el-button>
                <el-button type="primary" plain :loading="inspectionLoading" @click="loadInspectionReports">
                  刷新报告
                </el-button>
              </div>
            </div>

            <el-table
              :data="inspectionReports"
              v-loading="inspectionLoading"
              stripe
              class="modern-table"
              :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
            >
              <el-table-column label="生成时间" prop="generatedAt" width="170" />
              <el-table-column label="实例" min-width="160">
                <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
              </el-table-column>
              <el-table-column label="健康分" width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="healthScoreTag(row.healthScore)" size="small">{{ row.healthScore }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="风险" width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="riskLevelTag(row.riskLevel)" size="small">
                    {{ row.riskLevelText || row.riskLevel || '-' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="backupStatusTag(row.status)" size="small">
                    {{ row.statusText || row.status || '-' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="摘要" min-width="320" show-overflow-tooltip>
                <template #default="{ row }">{{ row.summary || row.errorMessage || '-' }}</template>
              </el-table-column>
              <el-table-column label="耗时" width="100" align="right">
                <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="100" align="center" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openInspectionDetail(row)">详情</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-container">
              <el-pagination
                v-model:current-page="inspectionQuery.page"
                v-model:page-size="inspectionQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="inspectionTotal"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="loadInspectionReports"
                @current-change="loadInspectionReports"
              />
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="查询审计" name="audit">
        <div class="audit-panel">
          <div class="audit-search-bar">
            <el-input
              v-model="auditQuery.keyword"
              placeholder="搜索 SQL、操作者、Schema、客户端 IP..."
              clearable
              class="audit-search-input"
              @keyup.enter="loadQueryAudits"
              @clear="loadQueryAudits"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="auditQuery.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="loadQueryAudits">
              <el-option v-for="item in instanceOptions" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
            <el-select v-model="auditQuery.status" placeholder="状态" clearable class="audit-select" @change="loadQueryAudits">
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
              <el-option label="已拦截" value="denied" />
              <el-option label="执行中" value="pending" />
            </el-select>
            <el-select v-model="auditQuery.action" placeholder="审计动作" clearable class="audit-select" @change="loadQueryAudits">
              <el-option v-for="item in auditActions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
            <el-select v-model="auditQuery.riskLevel" placeholder="风险" clearable class="audit-select" @change="loadQueryAudits">
              <el-option label="低" value="low" />
              <el-option label="中" value="medium" />
              <el-option label="高" value="high" />
              <el-option label="严重" value="critical" />
            </el-select>
            <el-select v-model="auditQuery.sqlType" placeholder="SQL 类型" clearable class="audit-select" @change="loadQueryAudits">
              <el-option v-for="item in sqlTypes" :key="item" :label="item" :value="item" />
            </el-select>
            <el-button @click="resetAuditQuery">
              <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
              重置
            </el-button>
            <el-button type="primary" plain :loading="auditExporting" @click="handleExportAudits">
              <el-icon style="margin-right: 4px;"><Download /></el-icon>
              导出 CSV
            </el-button>
          </div>

          <el-table
            :data="queryAudits"
            v-loading="auditLoading"
            stripe
            class="modern-table"
            :header-cell-style="{ background: '#fafbfc', color: '#606266', fontWeight: '600' }"
          >
            <el-table-column label="执行时间" prop="createdAt" width="170" />
            <el-table-column label="实例" min-width="150">
              <template #default="{ row }">
                <span>{{ row.instanceName || `#${row.instanceId}` }}</span>
              </template>
            </el-table-column>
            <el-table-column label="Schema" min-width="120">
              <template #default="{ row }">{{ row.schemaName || '-' }}</template>
            </el-table-column>
            <el-table-column label="动作" width="120" align="center">
              <template #default="{ row }">
                <el-tag size="small" type="info">{{ row.actionText || row.action || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="SQL" min-width="260" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="mono">{{ row.sqlSummary || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small">{{ row.sqlType || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="风险" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="riskLevelTag(row.riskLevel)" size="small">
                  {{ row.riskLevelText || row.riskLevel }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="auditStatusTag(row.status)" size="small">
                  {{ row.statusText || row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="返回行" prop="rowsReturned" width="90" align="right" />
            <el-table-column label="耗时" width="100" align="right">
              <template #default="{ row }">{{ row.durationMs }} ms</template>
            </el-table-column>
            <el-table-column label="操作者" min-width="110">
              <template #default="{ row }">{{ row.operatorName || '-' }}</template>
            </el-table-column>
            <el-table-column label="客户端 IP" min-width="130">
              <template #default="{ row }">{{ row.clientIp || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openAuditDetail(row)">详情</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-container">
            <el-pagination
              v-model:current-page="auditQuery.page"
              v-model:page-size="auditQuery.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="auditTotal"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadQueryAudits"
              @current-change="loadQueryAudits"
            />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <div
      v-if="tableContextMenu.visible"
      class="metadata-context-menu"
      :style="{ left: `${tableContextMenu.x}px`, top: `${tableContextMenu.y}px` }"
      @click.stop
      @contextmenu.prevent
    >
      <button
        type="button"
        class="metadata-context-menu-item"
        :disabled="!canPreviewContextTable"
        @click="handlePreviewContextTable(false)"
      >
        查看表内容
      </button>
      <button
        v-if="canUseContextTableUnlimited"
        type="button"
        class="metadata-context-menu-item"
        @click="handlePreviewContextTable(true)"
      >
        查看全部内容
      </button>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="form.id ? '编辑数据库实例' : '新增数据库实例'"
      width="680px"
      @close="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
        <el-form-item label="实例名称" prop="name">
          <el-input v-model="form.name" placeholder="如：生产-核心MySQL" />
        </el-form-item>
        <el-form-item label="数据库类型" prop="dbType">
          <el-select v-model="form.dbType" placeholder="请选择数据库类型" style="width: 100%;" @change="handleTypeChange">
            <el-option v-for="item in supportedTypes" :key="item.type" :label="item.name" :value="item.type">
              <div class="type-option">
                <span>{{ item.name }}</span>
                <el-tag size="small" :type="item.testEnabled ? 'success' : 'info'">
                  {{ item.phase }}
                </el-tag>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="14">
            <el-form-item label="主机地址" prop="host">
              <el-input v-model="form.host" placeholder="IP、域名或内网地址" />
            </el-form-item>
          </el-col>
          <el-col :span="10">
            <el-form-item label="端口" prop="port" label-width="56px">
              <el-input-number v-model="form.port" :min="1" :max="65535" class="port-input" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="defaultDatabaseLabel">
          <el-input v-model="form.defaultDatabase" :placeholder="defaultDatabasePlaceholder" />
        </el-form-item>
        <el-form-item label="连接参数">
          <el-input
            v-model="form.connectionParams"
            type="textarea"
            :rows="3"
            :placeholder="connectionParamsPlaceholder"
          />
          <div class="field-tip">{{ connectionParamsTip }}</div>
        </el-form-item>
        <el-form-item label="连接凭据" prop="credentialId">
          <el-select v-model="form.credentialId" placeholder="请选择凭据" filterable style="width: 100%;">
            <el-option
              v-for="item in credentials"
              :key="item.id"
              :label="`${item.name}（${item.username || '-'}）`"
              :value="item.id"
            />
          </el-select>
          <div class="field-tip">{{ credentialFieldTip }}</div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="环境">
              <el-select v-model="form.environment" clearable placeholder="环境" style="width: 100%;">
                <el-option label="生产" value="prod" />
                <el-option label="预发" value="staging" />
                <el-option label="测试" value="test" />
                <el-option label="开发" value="dev" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="业务系统">
              <el-input v-model="form.businessSystem" placeholder="业务系统" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="负责人">
              <el-input v-model="form.owner" placeholder="负责人" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio value="enabled">启用</el-radio>
            <el-radio value="disabled">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="TLS">
          <el-switch v-model="form.tlsEnabled" active-text="启用" inactive-text="关闭" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="form.tags" placeholder="多个标签用逗号分隔" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" placeholder="实例说明、维护窗口、注意事项等" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="auditDetailVisible" title="数据库审计详情" width="820px">
      <el-descriptions v-if="currentAudit" :column="2" border>
        <el-descriptions-item label="审计 ID">{{ currentAudit.id }}</el-descriptions-item>
        <el-descriptions-item label="执行时间">{{ currentAudit.createdAt }}</el-descriptions-item>
        <el-descriptions-item label="实例">{{ currentAudit.instanceName || `#${currentAudit.instanceId}` }}</el-descriptions-item>
        <el-descriptions-item label="Schema">{{ currentAudit.schemaName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="操作者">{{ currentAudit.operatorName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="客户端 IP">{{ currentAudit.clientIp || '-' }}</el-descriptions-item>
        <el-descriptions-item label="审计动作">{{ currentAudit.actionText || currentAudit.action || '-' }}</el-descriptions-item>
        <el-descriptions-item label="SQL 类型">{{ currentAudit.sqlType || '-' }}</el-descriptions-item>
        <el-descriptions-item label="风险">{{ currentAudit.riskLevelText || currentAudit.riskLevel }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ currentAudit.statusText || currentAudit.status }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ currentAudit.durationMs }} ms</el-descriptions-item>
	        <el-descriptions-item label="返回行">{{ currentAudit.rowsReturned }}</el-descriptions-item>
	        <el-descriptions-item :label="isDDLAudit(currentAudit) ? '变更类型' : '影响阈值'">
	          {{ isDDLAudit(currentAudit) ? 'DDL' : (currentAudit.rowsAffectedLimit || 0) }}
	        </el-descriptions-item>
	        <el-descriptions-item :label="isDDLAudit(currentAudit) ? '执行结果' : '影响行'">
	          {{ isDDLAudit(currentAudit) ? (currentAudit.statusText || currentAudit.status) : currentAudit.rowsAffected }}
	        </el-descriptions-item>
        <el-descriptions-item label="原因">{{ currentAudit.reason || '-' }}</el-descriptions-item>
        <el-descriptions-item label="确认状态">
          {{ currentAudit.confirmRequired ? (currentAudit.confirmed ? '需要确认，已确认' : '需要确认，未确认') : '无需确认' }}
        </el-descriptions-item>
        <el-descriptions-item label="错误信息" :span="2">{{ currentAudit.errorMessage || '-' }}</el-descriptions-item>
        <el-descriptions-item label="SQL 指纹" :span="2">
          <span class="mono">{{ currentAudit.sqlFingerprint || '-' }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <div v-if="currentAudit" class="audit-sql-block">
        <div class="audit-sql-title">SQL 原文</div>
        <pre>{{ currentAudit.sqlText }}</pre>
      </div>
      <div v-if="currentAudit?.rollbackSql" class="audit-sql-block">
        <div class="audit-sql-title">回滚 SQL / 恢复提示</div>
        <pre>{{ currentAudit.rollbackSql }}</pre>
      </div>
      <template #footer>
        <el-button @click="auditDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ddlDialogVisible" title="表 DDL 预览" width="900px">
      <div v-if="currentDDL">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="实例">{{ currentDDL.instanceName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="数据库类型">{{ currentDDL.dbTypeText || currentDDL.dbType }}</el-descriptions-item>
          <el-descriptions-item label="Schema">{{ currentDDL.schemaName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="对象">{{ currentDDL.tableName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="对象类型">{{ currentDDL.tableType || '-' }}</el-descriptions-item>
          <el-descriptions-item label="引擎">{{ currentDDL.engine || '-' }}</el-descriptions-item>
          <el-descriptions-item label="字段数">{{ currentDDL.columnsCount }}</el-descriptions-item>
          <el-descriptions-item label="索引数">{{ currentDDL.indexesCount }}</el-descriptions-item>
          <el-descriptions-item label="数据容量">{{ formatBytes(currentDDL.dataSizeBytes) }}</el-descriptions-item>
          <el-descriptions-item label="索引容量">{{ formatBytes(currentDDL.indexSizeBytes) }}</el-descriptions-item>
          <el-descriptions-item label="表注释" :span="2">{{ currentDDL.comment || '-' }}</el-descriptions-item>
          <el-descriptions-item label="生成时间" :span="2">{{ currentDDL.generatedAt }}</el-descriptions-item>
        </el-descriptions>
        <div class="audit-sql-block ddl-block">
          <div class="audit-sql-title">DDL 预览</div>
          <pre>{{ currentDDL.ddl }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="ddlDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="explainDialogVisible" title="SQL 执行计划" width="960px">
      <div v-if="explainResult">
        <div class="result-summary explain-summary">
          <div class="summary-card">
            <span class="summary-label">返回行数</span>
            <strong>{{ explainResult.rowsReturned || 0 }}</strong>
          </div>
          <div class="summary-card">
            <span class="summary-label">耗时</span>
            <strong>{{ explainResult.durationMs || 0 }} ms</strong>
          </div>
          <div class="summary-card">
            <span class="summary-label">SQL 类型</span>
            <strong>{{ explainResult.sqlType || 'EXPLAIN' }}</strong>
          </div>
          <div class="summary-card">
            <span class="summary-label">审计 ID</span>
            <strong>{{ explainResult.auditId || '-' }}</strong>
          </div>
        </div>
        <el-alert
          v-if="explainResult.executedSql"
          :title="`实际执行 SQL：${explainResult.executedSql}`"
          type="info"
          show-icon
          :closable="false"
          class="executed-sql-alert"
        />
        <el-table
          :data="explainResult.rows || []"
          border
          stripe
          height="420"
          class="query-result-table"
        >
          <el-table-column
            v-for="column in explainResult.columns || []"
            :key="column"
            :prop="column"
            :label="column"
            min-width="150"
            show-overflow-tooltip
          >
            <template #default="{ row }">
              <span class="query-cell">{{ formatQueryCell(row[column]) }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <el-empty v-else description="暂无执行计划" :image-size="72" />
      <template #footer>
        <el-button @click="explainDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="writeConfirmVisible" title="写操作执行确认" width="760px" @close="resetWriteConfirmForm">
      <div v-if="writeCheckResult" class="write-confirm-dialog">
        <el-alert
          :title="writeCheckResult.message || '写操作预检查通过'"
          :type="writeCheckResult.allowed ? 'success' : 'warning'"
          show-icon
          :closable="false"
        />
        <el-descriptions :column="2" border class="write-confirm-desc">
          <el-descriptions-item label="实例">{{ currentQueryInstance?.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Schema">{{ writeCheckResult.schemaName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="SQL 类型">{{ writeCheckResult.sqlType || '-' }}</el-descriptions-item>
          <el-descriptions-item label="风险等级">
            <el-tag :type="riskLevelTag(writeCheckResult.riskLevel)">{{ writeCheckResult.riskLevelText }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="影响阈值">{{ writeCheckResult.rowsAffectedLimit }}</el-descriptions-item>
          <el-descriptions-item label="确认要求">
            {{ writeCheckResult.confirmRequired ? '需要二次确认' : '无需二次确认' }}
          </el-descriptions-item>
        </el-descriptions>
        <el-form label-width="100px" class="write-confirm-form">
          <el-form-item :label="writeCheckResult.reasonRequired ? '操作原因' : '操作说明'" :required="writeCheckResult.reasonRequired">
            <el-input
              v-model="writeConfirmForm.reason"
              type="textarea"
              :rows="3"
              :placeholder="writeCheckResult.reasonRequired ? '请填写本次写操作原因' : '可选，建议填写变更背景或工单号'"
            />
          </el-form-item>
          <el-form-item v-if="writeCheckResult.confirmRequired" label="执行确认" required>
            <el-checkbox v-model="writeConfirmForm.confirmed">
              我已确认该 SQL 的影响范围和风险，继续执行写操作
            </el-checkbox>
          </el-form-item>
        </el-form>
        <div class="audit-sql-block">
          <div class="audit-sql-title">待执行 SQL</div>
          <pre>{{ querySQL }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="writeConfirmVisible = false">取消</el-button>
        <el-button type="danger" :loading="queryWriteSubmitting" @click="handleExecuteWrite">确认执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ddlConfirmVisible" title="DDL 结构变更确认" width="760px" @close="resetDDLConfirmForm">
      <div v-if="ddlCheckResult" class="write-confirm-dialog">
        <el-alert
          :title="ddlCheckResult.message || 'DDL 结构变更检查通过'"
          :type="ddlCheckResult.allowed ? 'success' : 'warning'"
          show-icon
          :closable="false"
        />
        <el-alert
          v-if="ddlCheckResult.backupRequired"
          title="DDL 可能改变数据库结构，执行前请确认已有可用备份或回滚方案。"
          type="warning"
          show-icon
          :closable="false"
          class="executed-sql-alert"
        />
        <el-descriptions :column="2" border class="write-confirm-desc">
          <el-descriptions-item label="实例">{{ currentQueryInstance?.name || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Schema">{{ ddlCheckResult.schemaName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="SQL 类型">{{ ddlCheckResult.sqlType || '-' }}</el-descriptions-item>
          <el-descriptions-item label="风险等级">
            <el-tag :type="riskLevelTag(ddlCheckResult.riskLevel)">{{ ddlCheckResult.riskLevelText }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="备份提示">
            {{ ddlCheckResult.backupRequired ? '建议先确认备份' : '不要求' }}
          </el-descriptions-item>
          <el-descriptions-item label="确认要求">
            {{ ddlCheckResult.confirmRequired ? '需要二次确认' : '无需二次确认' }}
          </el-descriptions-item>
        </el-descriptions>
        <el-form label-width="100px" class="write-confirm-form">
          <el-form-item :label="ddlCheckResult.reasonRequired ? '变更原因' : '变更说明'" :required="ddlCheckResult.reasonRequired">
            <el-input
              v-model="ddlConfirmForm.reason"
              type="textarea"
              :rows="3"
              :placeholder="ddlCheckResult.reasonRequired ? '请填写本次 DDL 结构变更原因' : '可选，建议填写变更背景或工单号'"
            />
          </el-form-item>
          <el-form-item v-if="ddlCheckResult.confirmRequired" label="执行确认" required>
            <el-checkbox v-model="ddlConfirmForm.confirmed">
              我已确认该 DDL 的影响范围、备份状态和回滚方案，继续执行结构变更
            </el-checkbox>
          </el-form-item>
        </el-form>
        <div class="audit-sql-block">
          <div class="audit-sql-title">待执行 DDL</div>
          <pre>{{ querySQL }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="ddlConfirmVisible = false">取消</el-button>
        <el-button type="danger" :loading="queryDDLSubmitting" @click="handleExecuteDDL">确认执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="queryHistoryDialogVisible" title="最近 SQL 历史" width="980px">
      <div class="query-history-toolbar">
        <el-input
          v-model="queryHistoryQuery.keyword"
          placeholder="搜索最近 SQL..."
          clearable
          class="query-history-input"
          @keyup.enter="loadQueryHistory"
          @clear="loadQueryHistory"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="query-option-label">历史条数</span>
        <el-input-number v-model="queryHistoryQuery.limit" :min="5" :max="50" :step="5" class="query-number" />
        <el-switch
          v-model="queryHistoryQuery.currentInstanceOnly"
          active-text="当前实例"
          inactive-text="全部实例"
          @change="loadQueryHistory"
        />
        <el-button type="primary" plain :loading="queryHistoryLoading" @click="loadQueryHistory">
          刷新
        </el-button>
      </div>
      <el-table :data="queryHistoryItems" v-loading="queryHistoryLoading" stripe class="modern-table">
        <el-table-column label="执行时间" prop="createdAt" width="170" />
        <el-table-column label="实例" min-width="150">
          <template #default="{ row }">{{ row.instanceName || `#${row.instanceId}` }}</template>
        </el-table-column>
        <el-table-column label="Schema" min-width="120">
          <template #default="{ row }">{{ row.schemaName || '-' }}</template>
        </el-table-column>
        <el-table-column label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.sqlType || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="auditStatusTag(row.status)" size="small">
              {{ row.statusText || row.status || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="SQL" min-width="320" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.sqlSummary || row.sqlText || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" align="center" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="applyHistoryQuery(row)">
              复用 SQL
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="queryHistoryQuery.currentInstanceOnly && queryInstanceId" class="query-history-tip">
        当前按实例 {{ currentQueryInstance?.name || `#${queryInstanceId}` }} 过滤，若已选择 Schema 也会一起过滤。
      </div>
      <template #footer>
        <el-button @click="queryHistoryDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="backupTaskDialogVisible"
      :title="backupTaskForm.id ? '编辑备份任务' : '新增备份任务'"
      width="min(1180px, calc(100vw - 48px))"
      class="backup-task-dialog"
      top="5vh"
      @close="resetBackupTaskForm"
    >
      <el-alert
        title="MySQL / MariaDB 物理备份使用 XtraBackup / mariadb-backup；PostgreSQL 物理备份第一版仅支持 Barman，且固定为 cluster 级，需要在范围配置中填写 barmanServerId。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="backupTaskFormRef" :model="backupTaskForm" :rules="backupTaskRules" label-width="110px">
        <el-form-item label="数据库实例" prop="instanceId">
          <el-select v-model="backupTaskForm.instanceId" placeholder="请选择实例" filterable style="width: 100%;">
            <el-option
              v-for="item in supportedBackupInstances"
              :key="item.id"
              :label="`${item.name}（${item.dbTypeText || item.dbType}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="backupTaskForm.name" placeholder="如：app-db-nightly" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="备份方法">
              <el-select v-model="backupTaskForm.backupMethod" style="width: 100%;">
                <el-option
                  v-for="item in availableBackupMethodOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="isPhysicalBackupTaskForm ? '备份类型' : '备份格式'">
              <el-select v-model="backupTaskForm.backupType" style="width: 100%;">
                <el-option
                  v-for="item in availableBackupTypeOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="级别">
              <el-select v-model="backupTaskForm.backupLevel" :disabled="!isPhysicalBackupTaskForm" style="width: 100%;">
                <el-option
                  v-for="item in availableBackupLevelOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
	        </el-row>
	        <el-row :gutter="16">
	          <el-col :span="8">
	            <el-form-item label="工具执行">
	              <el-select v-model="backupPolicyForm.toolExecutionMode" style="width: 100%;">
	                <el-option label="宿主机工具" value="host_tools" />
	                <el-option label="容器化工具" value="container_tools" />
	              </el-select>
	            </el-form-item>
	          </el-col>
	          <el-col :span="8">
	            <el-form-item label="工具镜像">
	              <el-input v-model="backupPolicyForm.toolImage" :disabled="backupPolicyForm.toolExecutionMode !== 'container_tools'" placeholder="如 opshub-runner-tools:mysql80" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="8">
	            <el-form-item label="镜像 Digest">
	              <el-input v-model="backupPolicyForm.toolImageDigest" :disabled="backupPolicyForm.toolExecutionMode !== 'container_tools'" placeholder="可选 sha256:..." />
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-row v-if="backupPolicyForm.toolExecutionMode === 'container_tools'" :gutter="16">
	          <el-col :span="8">
	            <el-form-item label="datadir挂载">
	              <el-input v-model="backupPolicyForm.containerDatadirPath" placeholder="/var/lib/mysql 或只读挂载点" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="8">
	            <el-form-item label="工作目录挂载">
	              <el-input v-model="backupPolicyForm.containerWorkdirPath" placeholder="可选，默认 Runner 工作目录/仓库挂载" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="4">
	            <el-form-item label="网络">
	              <el-input v-model="backupPolicyForm.containerNetworkMode" placeholder="host" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="4">
	            <el-form-item label="datadir只读">
	              <el-switch v-model="backupPolicyForm.containerDatadirRo" active-text="是" inactive-text="否" />
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-alert
	          v-if="backupPolicyForm.toolExecutionMode === 'container_tools'"
	          title="容器化模式会在 Runner 上用 Docker 运行固定镜像；MySQL/MariaDB 物理备份必须能把生产 datadir 只读挂载给容器。"
	          type="warning"
	          show-icon
	          :closable="false"
	          class="backup-dialog-alert compact-alert"
	        />
	        <el-row :gutter="16">
          <el-col v-if="isPhysicalBackupTaskForm" :span="12">
            <el-form-item label="备份引擎">
              <el-select v-model="backupTaskForm.backupEngine" style="width: 100%;">
                <el-option
                  v-for="item in availablePhysicalBackupEngineOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="isPhysicalBackupTaskForm ? 6 : 12">
            <el-form-item label="备份范围">
              <el-input v-model="backupTaskForm.backupScope" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="isPhysicalBackupTaskForm ? 6 : 12">
            <el-form-item label="存储类型">
              <el-input v-model="backupTaskForm.storageType" disabled />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="isPhysicalBackupTaskForm" label="范围配置">
          <el-input
            v-model="backupTaskForm.scopeConfig"
            type="textarea"
            :rows="4"
            :placeholder="selectedBackupTaskDbType === 'postgresql' ? 'Barman: barmanServerId=1；pg_basebackup: runnerHostId=1, extraArgs=[--wal-method=stream]' : '全量可留空；增量配置可填写 incrementalBaseDir 和 extraArgs'"
          />
          <div class="field-tip">物理备份按实例/datadir 级别执行；extraArgs 不允许包含 password/secret。增量备份必须提供上一备份目录 incrementalBaseDir。</div>
          <div v-if="selectedBackupTaskDbType === 'postgresql'" class="field-tip">PostgreSQL 物理备份是 cluster 级。Barman 使用 {"barmanServerId":1}；pg_basebackup 使用 {"runnerHostId":1}，full base backup 会下发受控 SSH Runner 任务。</div>
        </el-form-item>
        <el-form-item label="执行计划">
          <el-input
            v-model="backupTaskForm.schedule"
            placeholder="Cron 表达式，留空表示仅手动执行，如 0 2 * * *"
          />
          <div class="field-tip">当前校验标准 5 段 Cron，启用任务后会由后端调度器轮询执行。</div>
        </el-form-item>
        <el-form-item label="保留天数" prop="retentionDays">
          <el-input-number v-model="backupTaskForm.retentionDays" :min="1" :max="3650" class="query-number" />
        </el-form-item>
        <el-form-item label="最长运行">
          <el-input-number v-model="backupTaskForm.maxDurationMinutes" :min="1" :max="10080" class="query-number" />
          <div class="field-tip">单位分钟。超过该时长仍未完成的备份会被判定为失败，默认 1440 分钟。</div>
        </el-form-item>
        <el-form-item label="存储配置">
          <el-input
            v-model="backupTaskForm.storageConfig"
            type="textarea"
            :rows="3"
            placeholder="可选，预留给后续本地目录或对象存储扩展"
          />
          <div class="field-tip">P0 仅允许非敏感配置；对象存储密钥、数据库密码、SSH 私钥等后续必须接入 secret_profile 后再保存。</div>
        </el-form-item>
        <el-form-item v-if="selectedBackupTaskInstance?.capacitySizeText" label="容量提示">
          <el-alert
            :title="backupCapacityTipTitle"
            :type="backupCapacityTipType"
            show-icon
            :closable="false"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="backupTaskForm.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="backupTaskDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="backupTaskSubmitting" @click="submitBackupTaskForm">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="backupPolicyDialogVisible"
      :title="backupPolicyForm.id ? '编辑备份策略' : '新增备份策略'"
      width="980px"
      @close="resetBackupPolicyForm"
    >
      <el-alert
        title="策略会由后端校验 full/incremental 物理备份链，增量自动选择上一条成功记录作为基线；synthetic full 可把基础全量与最早一段增量合成为新的全量基线。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="backupPolicyFormRef" :model="backupPolicyForm" :rules="backupPolicyRules" label-width="125px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="数据库实例" prop="instanceId">
              <el-select v-model="backupPolicyForm.instanceId" placeholder="请选择 MySQL/MariaDB 实例" filterable style="width: 100%;">
                <el-option
                  v-for="item in mysqlPhysicalBackupPolicyInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.dbTypeText || item.dbType}${item.version ? ` / ${item.version}` : ''}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="策略名称" prop="name">
              <el-input v-model="backupPolicyForm.name" placeholder="如：opshub-mysql-monthly-full-daily-inc" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="备份工具" prop="backupEngine">
              <el-select v-model="backupPolicyForm.backupEngine" style="width: 100%;">
                <el-option
                  v-for="item in availableBackupPolicyEngineOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="来源角色">
              <el-select v-model="backupPolicyForm.sourceRole" style="width: 100%;">
                <el-option label="主库" value="primary" />
                <el-option label="实时从库" value="replica" />
                <el-option label="延迟从库" value="delayed_replica" />
                <el-option label="备份从库" value="backup_replica" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="来源实例">
              <el-select v-model="backupPolicyForm.sourceInstanceId" placeholder="默认同主实例" clearable filterable style="width: 100%;">
                <el-option
                  v-for="item in mysqlPhysicalBackupPolicyInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.dbTypeText || item.dbType}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="Runner" prop="runnerHostId">
              <el-select v-model="backupPolicyForm.runnerHostId" placeholder="请选择 Runner" filterable style="width: 100%;">
                <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="存储配置">
              <el-select v-model="backupPolicyForm.storageProfileId" placeholder="可选；默认 runner:// 本地" clearable filterable style="width: 100%;">
                <el-option
                  v-for="item in storageProfiles"
                  :key="item.id"
                  :label="`${item.name}（${item.storageTypeText || item.storageType}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="binlog归档流">
              <el-select v-model="backupPolicyForm.binlogStreamId" placeholder="建议绑定连续归档流" clearable filterable style="width: 100%;">
                <el-option v-for="item in backupPolicyBinlogStreamOptions" :key="item.id" :label="item.label" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Full Cron">
              <el-input v-model="backupPolicyForm.fullSchedule" placeholder="如 0 2 1 * *，留空仅手动" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="增量 Cron">
              <el-input v-model="backupPolicyForm.incrementalSchedule" placeholder="如 0 3 * * *，留空仅手动" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="Synthetic Full">
              <el-switch v-model="backupPolicyForm.syntheticEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="恢复演练门禁">
              <el-switch v-model="backupPolicyForm.restoreDrillRequired" active-text="要求" inactive-text="不要求" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-switch v-model="backupPolicyForm.enabled" active-text="启用" inactive-text="禁用" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="Synthetic规则">
          <el-input v-model="backupPolicyForm.syntheticRuleJson" type="textarea" :rows="6" placeholder="rolling_synthetic_full 自动合成规则 JSON" />
          <div v-if="backupPolicyForm.syntheticEnabled" class="field-tip">
            {{ backupPolicyFormSyntheticRuleTip }}
          </div>
          <el-alert
            v-if="backupPolicyFormSyntheticRuleWarning"
            :title="backupPolicyFormSyntheticRuleWarning"
            type="warning"
            show-icon
            :closable="false"
            class="backup-dialog-alert compact-alert"
          />
        </el-form-item>
        <el-form-item label="保留策略">
          <el-input v-model="backupPolicyForm.retentionJson" type="textarea" :rows="4" placeholder="JSON，仅保存非敏感保留规则" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="backupPolicyDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="backupPolicySubmitting" @click="submitBackupPolicyForm">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="syntheticPreviewVisible"
      title="合成全量预览"
      width="980px"
    >
      <template v-if="syntheticPreview">
        <el-alert
          :title="syntheticPreview.statusText || syntheticPreview.status"
          :type="syntheticPreview.blockingReasons?.length ? 'error' : (syntheticPreview.warnings?.length ? 'warning' : 'success')"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-descriptions :column="3" border size="small" class="backup-summary-descriptions">
          <el-descriptions-item label="策略">{{ syntheticPreviewPolicy?.name || `#${syntheticPreview.policyId}` }}</el-descriptions-item>
          <el-descriptions-item label="基础记录">#{{ syntheticPreview.selectedBaseRecordId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="合成截止">#{{ syntheticPreview.newSyntheticFullAfterRecordId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="合并增量">{{ syntheticPreview.selectedIncrementalRecordIds?.length || 0 }} / {{ syntheticPreview.mergeIncrementalCount }}</el-descriptions-item>
          <el-descriptions-item label="输入估算">{{ syntheticPreview.estimatedInputSizeText || formatBytes(syntheticPreview.estimatedInputSize) }}</el-descriptions-item>
          <el-descriptions-item label="工作目录估算">{{ syntheticPreview.estimatedWorkdirSizeText || formatBytes(syntheticPreview.estimatedWorkdirSize) }}</el-descriptions-item>
          <el-descriptions-item label="恢复证明">{{ syntheticPreview.requiresRestoreProof ? '要求' : '不强制' }}</el-descriptions-item>
          <el-descriptions-item label="检查时间">{{ syntheticPreview.checkedAt || '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ syntheticPreview.statusText || syntheticPreview.status }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="syntheticPreview.blockingReasons?.length" class="backup-preview-section">
          <div class="section-title">阻断原因</div>
          <el-alert
            v-for="item in syntheticPreview.blockingReasons"
            :key="item"
            :title="item"
            type="error"
            show-icon
            :closable="false"
            class="backup-inline-alert"
          />
        </div>
        <div v-if="syntheticPreview.warnings?.length" class="backup-preview-section">
          <div class="section-title">风险提示</div>
          <el-alert
            v-for="item in syntheticPreview.warnings"
            :key="item"
            :title="item"
            type="warning"
            show-icon
            :closable="false"
            class="backup-inline-alert"
          />
        </div>
        <el-table :data="syntheticPreview.selectedRecords || []" stripe class="modern-table backup-preview-table">
          <el-table-column label="记录" width="90">
            <template #default="{ row }">#{{ row.id }}</template>
          </el-table-column>
          <el-table-column label="级别" width="120">
            <template #default="{ row }">
              <el-tag size="small" :type="row.backupLevel === 'full' ? 'primary' : 'success'">{{ row.backupLevelText || row.backupLevel }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="LSN" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.checkpointFromLsn || '-' }} -> {{ row.checkpointToLsn || '-' }}</template>
          </el-table-column>
          <el-table-column label="文件" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{ row.fileName || row.storageUri || '-' }}</template>
          </el-table-column>
          <el-table-column label="大小" width="110">
            <template #default="{ row }">{{ formatBytes(row.fileSize) }}</template>
          </el-table-column>
          <el-table-column label="可恢复到" width="170">
            <template #default="{ row }">{{ row.recoverableUntil || row.finishedAt || '-' }}</template>
          </el-table-column>
        </el-table>
      </template>
      <template #footer>
        <el-button @click="syntheticPreviewVisible = false">关闭</el-button>
        <el-button
          type="danger"
          :disabled="!!syntheticPreview?.blockingReasons?.length || !syntheticPreviewPolicy"
          :loading="runningSyntheticPolicyId === syntheticPreviewPolicy?.id"
          @click="syntheticPreviewPolicy && handleRunSyntheticFull(syntheticPreviewPolicy)"
        >
          确认合成Full
        </el-button>
      </template>
    </el-dialog>
    <el-dialog
      v-model="purgePreviewVisible"
      title="Synthetic Full 旧链清理预览"
      width="1080px"
    >
      <template v-if="purgePreview">
        <el-alert
          :title="purgePreview.blockingReasons?.length ? '旧链暂不可清理' : '旧链清理门禁通过'"
          :type="purgePreview.blockingReasons?.length ? 'error' : (purgePreview.warnings?.length ? 'warning' : 'success')"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-descriptions :column="3" border size="small" class="backup-summary-descriptions">
          <el-descriptions-item label="策略">{{ purgePreviewPolicy?.name || `#${purgePreview.policyId}` }}</el-descriptions-item>
          <el-descriptions-item label="Synthetic Full">#{{ purgePreview.syntheticRecordId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="恢复证明">{{ purgePreview.proofStatusText || purgePreview.proofStatus || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Binlog 链">{{ purgePreview.binlogCoverageText || purgePreview.binlogCoverageStatus || '-' }}</el-descriptions-item>
          <el-descriptions-item label="保护天数">{{ purgePreview.retentionDays }}</el-descriptions-item>
          <el-descriptions-item label="无 Proof 禁删">{{ purgePreview.neverDeleteWithoutProof ? '开启' : '关闭' }}</el-descriptions-item>
          <el-descriptions-item label="可清理">{{ purgePreview.eligibleRecordIds?.length || 0 }} 条</el-descriptions-item>
          <el-descriptions-item label="阻断">{{ purgePreview.blockedRecordIds?.length || 0 }} 条</el-descriptions-item>
          <el-descriptions-item label="检查时间">{{ purgePreview.checkedAt || '-' }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="purgePreview.blockingReasons?.length" class="backup-preview-section">
          <div class="section-title">阻断原因</div>
          <el-alert
            v-for="item in purgePreview.blockingReasons"
            :key="item"
            :title="item"
            type="error"
            show-icon
            :closable="false"
            class="backup-inline-alert"
          />
        </div>
        <div v-if="purgePreview.warnings?.length" class="backup-preview-section">
          <div class="section-title">风险提示</div>
          <el-alert
            v-for="item in purgePreview.warnings"
            :key="item"
            :title="item"
            type="warning"
            show-icon
            :closable="false"
            class="backup-inline-alert"
          />
        </div>
        <div class="backup-preview-section">
          <div class="section-title">清理计划</div>
          <el-table :data="purgePreview.storageDeletePlan || []" stripe class="modern-table backup-preview-table" empty-text="暂无可清理记录">
            <el-table-column label="记录" width="90">
              <template #default="{ row }">#{{ row.recordId }}</template>
            </el-table-column>
            <el-table-column label="级别" width="110">
              <template #default="{ row }">{{ row.backupLevel || '-' }}</template>
            </el-table-column>
            <el-table-column label="文件" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">{{ row.fileName || row.storageUri || row.filePath || '-' }}</template>
            </el-table-column>
            <el-table-column label="大小" width="110">
              <template #default="{ row }">{{ row.fileSizeText || formatBytes(row.fileSize) }}</template>
            </el-table-column>
            <el-table-column label="Checksum" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ row.checksumSha256 || '-' }}</template>
            </el-table-column>
            <el-table-column label="清理窗口" width="170">
              <template #default="{ row }">{{ row.purgeEligibleAt || '-' }}</template>
            </el-table-column>
          </el-table>
        </div>
        <div v-if="purgePreview.blockedRecords?.length" class="backup-preview-section">
          <div class="section-title">受保护记录</div>
          <el-table :data="purgePreview.blockedRecords || []" stripe class="modern-table backup-preview-table">
            <el-table-column label="记录" width="90">
              <template #default="{ row }">#{{ row.recordId }}</template>
            </el-table-column>
            <el-table-column label="文件" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">{{ row.fileName || row.storageUri || row.filePath || '-' }}</template>
            </el-table-column>
            <el-table-column label="原因" min-width="320" show-overflow-tooltip>
              <template #default="{ row }">{{ row.blockingReason || '-' }}</template>
            </el-table-column>
          </el-table>
        </div>
      </template>
      <template #footer>
        <el-button @click="purgePreviewVisible = false">关闭</el-button>
        <el-button
          type="danger"
          :disabled="!!purgePreview?.blockingReasons?.length || !(purgePreview?.eligibleRecordIds?.length) || !purgePreviewPolicy"
          :loading="runningPurgePolicyId === purgePreviewPolicy?.id"
          @click="purgePreviewPolicy && handleRunBackupPolicyPurge(purgePreviewPolicy)"
        >
          标记清理旧链
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="externalBackupDialogVisible"
      title="登记外部备份记录"
      width="1080px"
      @close="resetExternalBackupForm"
    >
      <el-alert
        title="用于登记 XtraBackup、mariadb-backup、Barman、WAL-G、pg_basebackup 等外部工具已经生成的备份元数据；OpsHub P1 只记录链路和做恢复预校验。"
        type="info"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="externalBackupFormRef" :model="externalBackupForm" :rules="externalBackupRules" label-width="120px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="数据库实例" prop="instanceId">
              <el-select v-model="externalBackupForm.instanceId" placeholder="请选择实例" filterable style="width: 100%;">
                <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="`${item.name}（${item.dbTypeText || item.dbType}）`" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="来源角色">
              <el-select v-model="externalBackupForm.sourceRole" style="width: 100%;">
                <el-option label="外部登记" value="external" />
                <el-option label="主库" value="primary" />
                <el-option label="实时从库" value="replica" />
                <el-option label="延迟从库" value="delayed_replica" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="备份方法" prop="backupMethod">
              <el-select v-model="externalBackupForm.backupMethod" style="width: 100%;">
                <el-option label="物理备份" value="physical" />
                <el-option label="外部引擎" value="external" />
                <el-option label="逻辑备份" value="logical" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备份级别" prop="backupLevel">
              <el-select v-model="externalBackupForm.backupLevel" style="width: 100%;">
                <el-option label="全量" value="full" />
                <el-option label="增量" value="incremental" />
                <el-option label="差异" value="differential" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备份引擎" prop="backupEngine">
              <el-select
                v-model="externalBackupForm.backupEngine"
                filterable
                allow-create
                default-first-option
                style="width: 100%;"
                placeholder="选择或输入外部备份引擎"
              >
                <el-option
                  v-for="item in externalBackupEngineOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-alert
          v-if="isPgBackRestExternalBackup"
          title="pgBackRest 已按 legacy external 纳管：只登记已有链路、演练结果和风险提示，不作为新建 PostgreSQL 策略默认推荐，也不由 OpsHub 自动执行恢复。"
          type="warning"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="外部备份ID">
              <el-input v-model="externalBackupForm.externalBackupId" placeholder="WAL-G backup name / pgBackRest stanza backup ID" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="外部Server">
              <el-input v-model="externalBackupForm.externalServerName" placeholder="Barman server / WAL-G profile / pgBackRest stanza" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="链路 ID">
              <el-input v-model="externalBackupForm.chainId" placeholder="同一全量+增量链路的稳定 ID，可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="工具名">
              <el-input v-model="externalBackupForm.toolName" placeholder="可选，如 wal-g / pgbackrest" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="基础记录">
              <el-input-number v-model="externalBackupForm.baseRecordId" :min="0" class="form-number-full" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="父记录">
              <el-input-number v-model="externalBackupForm.parentRecordId" :min="0" class="form-number-full" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="存储 URI" prop="storageUri">
          <el-input v-model="externalBackupForm.storageUri" placeholder="s3://bucket/path/base_20260428 或 /backup/mysql/base_20260428" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="文件名" prop="fileName">
              <el-input v-model="externalBackupForm.fileName" placeholder="base_20260428.tar.zst" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="文件大小">
              <el-input-number v-model="externalBackupForm.fileSize" :min="0" class="form-number-full" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="状态">
              <el-select v-model="externalBackupForm.status" style="width: 100%;">
                <el-option label="成功" value="success" />
                <el-option label="失败" value="failed" />
                <el-option label="已过期" value="expired" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="SHA256">
          <el-input v-model="externalBackupForm.checksumSha256" placeholder="可选，用于恢复前校验" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="可恢复起点" prop="recoverableFrom">
              <el-date-picker v-model="externalBackupForm.recoverableFrom" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择时间" style="width: 100%;" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="可恢复终点" prop="recoverableUntil">
              <el-date-picker v-model="externalBackupForm.recoverableUntil" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择时间" style="width: 100%;" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="MySQL 起点">
              <el-input v-model="externalBackupForm.backupBinlogFile" placeholder="binlog.000123，可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="GTID 集合">
              <el-input v-model="externalBackupForm.backupGtidSet" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="PG System ID">
              <el-input v-model="externalBackupForm.pgSystemIdentifier" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="Timeline">
              <el-input v-model="externalBackupForm.timelineId" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="LSN 范围">
              <el-input v-model="externalBackupForm.startLsn" placeholder="start_lsn，可选" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="externalBackupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="externalBackupSubmitting" @click="submitExternalBackup">登记</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="storageProfileDialogVisible"
      title="新增存储配置"
      width="820px"
      @close="resetStorageProfileForm"
    >
      <el-alert
        title="存储配置只登记目标位置和期望安全能力，不保存对象存储明文密钥。安全姿态检测时请临时输入 access key / secret key。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="storageProfileFormRef" :model="storageProfileForm" :rules="storageProfileRules" label-width="120px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="storageProfileForm.name" placeholder="如：minio-backup-prod" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="存储类型" prop="storageType">
              <el-select v-model="storageProfileForm.storageType" style="width: 100%;">
                <el-option label="S3" value="s3" />
                <el-option label="MinIO" value="minio" />
                <el-option label="本地" value="local" />
                <el-option label="NFS" value="nfs" />
                <el-option label="外部" value="external" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Endpoint">
              <el-input v-model="storageProfileForm.endpoint" placeholder="MinIO 示例：http://192.168.1.30:9000；AWS S3 可留空" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Region">
              <el-input v-model="storageProfileForm.region" placeholder="如 us-east-1" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Bucket">
              <el-input v-model="storageProfileForm.bucket" placeholder="opshub-backup" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="路径前缀">
              <el-input v-model="storageProfileForm.pathPrefix" placeholder="opshub/database-archives" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="要求版本化">
              <el-switch v-model="storageProfileForm.versioningEnabled" active-text="是" inactive-text="否" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="要求不可变">
              <el-switch v-model="storageProfileForm.immutabilityEnabled" active-text="是" inactive-text="否" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="保留天数">
              <el-input-number v-model="storageProfileForm.retentionLockDays" :min="0" :max="3650" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="KMS Key">
          <el-input v-model="storageProfileForm.kmsKeyId" placeholder="可选；登记后检测会对比默认加密 KMS Key" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="storageProfileForm.status" style="width: 180px;">
            <el-option label="待配置" value="pending" />
            <el-option label="启用" value="enabled" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="storageProfileDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="storageProfileSubmitting" @click="submitStorageProfile">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="storagePostureDialogVisible"
      title="对象存储安全姿态检测"
      width="760px"
      @close="resetStoragePostureForm"
    >
      <el-alert
        title="检测会调用只读 Bucket API，临时 access key / secret key 仅随本次请求发送，不会写入存储配置。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-descriptions :column="2" border class="backup-detail-descriptions">
        <el-descriptions-item label="配置">{{ currentStoragePostureProfile?.name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ currentStoragePostureProfile?.storageTypeText || currentStoragePostureProfile?.storageType || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Bucket">{{ currentStoragePostureProfile?.bucket || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Endpoint">{{ currentStoragePostureProfile?.endpoint || '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-form ref="storagePostureFormRef" :model="storagePostureForm" :rules="storagePostureRules" label-width="130px" class="write-confirm-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Access Key" prop="accessKey">
              <el-input v-model="storagePostureForm.accessKey" autocomplete="off" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Secret Key" prop="secretKey">
              <el-input v-model="storagePostureForm.secretKey" type="password" show-password autocomplete="new-password" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="Session Token">
          <el-input v-model="storagePostureForm.sessionToken" type="textarea" :rows="2" placeholder="可选，STS 临时凭据使用" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="HTTPS">
              <el-switch v-model="storagePostureForm.useSsl" active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="Path Style">
              <el-switch v-model="storagePostureForm.usePathStyle" active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="跳过 TLS 校验">
              <el-switch v-model="storagePostureForm.insecureSkipVerify" active-text="开" inactive-text="关" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <div v-if="currentStoragePostureProfile?.postureSummary" class="storage-posture-current">
        <strong>上次结果：</strong>{{ currentStoragePostureProfile.postureSummary }}
      </div>
      <template #footer>
        <el-button @click="storagePostureDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="storagePostureSubmitting" @click="submitStoragePostureCheck">开始检测</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="storagePostureDetailVisible" title="对象存储姿态详情" width="900px">
      <div v-if="currentStoragePostureDetail" class="storage-posture-detail">
        <el-descriptions :column="2" border class="backup-detail-descriptions">
          <el-descriptions-item label="配置">{{ currentStoragePostureDetail.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="storagePostureStatusTag(currentStoragePostureDetail.postureStatus)">
              {{ currentStoragePostureDetail.postureStatusText || currentStoragePostureDetail.postureStatus || '-' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="最近检测">{{ currentStoragePostureDetail.lastPostureCheckAt || '-' }}</el-descriptions-item>
          <el-descriptions-item label="摘要">{{ currentStoragePostureDetail.postureSummary || '-' }}</el-descriptions-item>
        </el-descriptions>
        <el-table :data="storagePostureChecks" stripe class="modern-table storage-posture-checks">
          <el-table-column label="检查项" prop="label" min-width="150" />
          <el-table-column label="登记值" prop="expected" min-width="180" show-overflow-tooltip />
          <el-table-column label="检测值" prop="actual" min-width="180" show-overflow-tooltip />
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="storagePostureStatusTag(row.status)">{{ storagePostureStatusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="说明" prop="message" min-width="260" show-overflow-tooltip />
        </el-table>
        <div class="audit-sql-block">
          <div class="audit-sql-title">原始检测 JSON</div>
          <pre>{{ formattedStoragePostureJson }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="storagePostureDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="logArchiveStreamDialogVisible"
      title="新增日志归档流"
      width="720px"
      @close="resetLogArchiveStreamForm"
    >
      <el-form ref="logArchiveStreamFormRef" :model="logArchiveStreamForm" :rules="logArchiveStreamRules" label-width="120px">
        <el-form-item label="数据库实例" prop="instanceId">
          <el-select v-model="logArchiveStreamForm.instanceId" placeholder="请选择实例" filterable style="width: 100%;">
            <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="`${item.name}（${item.dbTypeText || item.dbType}）`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档类型" prop="archiveType">
              <el-select v-model="logArchiveStreamForm.archiveType" style="width: 100%;">
                <el-option label="MySQL/MariaDB binlog" value="binlog" />
                <el-option label="PostgreSQL WAL" value="wal" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="归档模式" prop="archiveMode">
              <el-select v-model="logArchiveStreamForm.archiveMode" style="width: 100%;">
                <el-option label="外部登记" value="external" />
                <el-option label="轮询归档" value="polling" />
                <el-option label="Streaming" value="streaming" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档引擎" prop="archiveEngine">
              <el-select
                v-model="logArchiveStreamForm.archiveEngine"
                filterable
                allow-create
                default-first-option
                style="width: 100%;"
                placeholder="选择或输入归档引擎"
              >
                <el-option
                  v-for="item in logArchiveEngineOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Runner 主机">
              <el-select v-model="logArchiveStreamForm.runnerHostId" placeholder="可选，启动时可再选择" clearable filterable style="width: 100%;">
                <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="RPO 秒">
              <el-input-number v-model="logArchiveStreamForm.rpoTargetSeconds" :min="1" :max="86400" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="保留天数">
              <el-input-number v-model="logArchiveStreamForm.retentionDays" :min="1" :max="3650" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-alert
          v-if="isPgBackRestLogArchiveStream"
          title="pgBackRest WAL 归档流仅按 legacy external 元数据纳管，用于已有链路登记和风险提示，不作为新建 PostgreSQL 默认方案。"
          type="warning"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-form-item label="配置 JSON">
          <el-input v-model="logArchiveStreamForm.configJson" type="textarea" :rows="3" placeholder="可选，只保存非敏感配置。密钥后续接 secret_profile。" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="logArchiveStreamForm.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="logArchiveStreamDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="logArchiveStreamSubmitting" @click="submitLogArchiveStream">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="startLogArchiveStreamDialogVisible"
      title="启动长期归档流"
      width="560px"
      @close="resetStartLogArchiveStreamForm"
    >
      <el-alert
        type="warning"
        show-icon
        :closable="false"
        title="启动后归档流会等待 Runner Agent 通过公开接口拉取并续租；backend 不直接运行长期 mysqlbinlog 进程。"
        class="backup-risk-alert"
      />
      <el-form ref="startLogArchiveStreamFormRef" :model="startLogArchiveStreamForm" :rules="startLogArchiveStreamRules" label-width="120px">
        <el-form-item label="归档流">
          <el-input :model-value="startLogArchiveStreamStream?.instanceName || '-'" disabled />
        </el-form-item>
        <el-form-item label="Runner 主机" prop="runnerHostId">
          <el-select v-model="startLogArchiveStreamForm.runnerHostId" placeholder="请选择 Runner 主机" filterable style="width: 100%;">
            <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="归档模式" prop="archiveMode">
          <el-select v-model="startLogArchiveStreamForm.archiveMode" style="width: 100%;">
            <el-option label="轮询归档" value="polling" />
            <el-option label="Streaming" value="streaming" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="startLogArchiveStreamDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="startLogArchiveStreamSubmitting" @click="submitStartLogArchiveStream">启动</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="logArchiveDialogVisible"
      title="登记日志归档文件"
      width="820px"
      @close="resetLogArchiveForm"
    >
      <el-form ref="logArchiveFormRef" :model="logArchiveForm" :rules="logArchiveRules" label-width="120px">
        <el-form-item label="归档流" prop="streamId">
          <el-select v-model="logArchiveForm.streamId" placeholder="请选择归档流" filterable style="width: 100%;">
            <el-option v-for="item in logArchiveStreamOptions" :key="item.id" :label="item.label" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="文件名" prop="fileName">
              <el-input v-model="logArchiveForm.fileName" placeholder="binlog.000123 / 0000000100000000000000A1" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="文件大小">
              <el-input-number v-model="logArchiveForm.fileSize" :min="0" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="状态">
              <el-select v-model="logArchiveForm.status" style="width: 100%;">
                <el-option label="已归档" value="archived" />
                <el-option label="缺失" value="missing" />
                <el-option label="校验失败" value="checksum_failed" />
                <el-option label="已过期" value="expired" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="存储 URI" prop="storageUri">
          <el-input v-model="logArchiveForm.storageUri" placeholder="s3://bucket/binlog/binlog.000123 或 /archive/wal/..." />
        </el-form-item>
        <el-form-item label="SHA256">
          <el-input v-model="logArchiveForm.checksumSha256" placeholder="可选" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="起始时间" prop="firstEventTime">
              <el-date-picker v-model="logArchiveForm.firstEventTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择时间" style="width: 100%;" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="结束时间" prop="lastEventTime">
              <el-date-picker v-model="logArchiveForm.lastEventTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择时间" style="width: 100%;" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="Server UUID">
              <el-input v-model="logArchiveForm.serverUuid" placeholder="MySQL 可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="起始位置">
              <el-input-number v-model="logArchiveForm.startPos" :min="0" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="结束位置">
              <el-input-number v-model="logArchiveForm.endPos" :min="0" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="PG System ID">
              <el-input v-model="logArchiveForm.pgSystemIdentifier" placeholder="PostgreSQL 可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="Timeline">
              <el-input v-model="logArchiveForm.timelineId" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="LSN 起点">
              <el-input v-model="logArchiveForm.startLsn" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="logArchiveDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="logArchiveSubmitting" @click="submitLogArchive">登记</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runLogArchiveOnceDialogVisible"
      title="一次性 binlog 归档"
      width="640px"
      @close="resetRunLogArchiveOnceForm"
    >
      <el-alert
        title="该操作只通过 Runner 拉取一个 binlog 文件并登记元数据，不会常驻运行，也不会修改源库 binlog 保留策略。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="runLogArchiveOnceFormRef" :model="runLogArchiveOnceForm" :rules="runLogArchiveOnceRules" label-width="120px">
        <el-form-item label="归档流">
          <el-input
            :model-value="runLogArchiveOnceStream ? `${runLogArchiveOnceStream.instanceName || `#${runLogArchiveOnceStream.instanceId}`} / ${runLogArchiveOnceStream.archiveTypeText || runLogArchiveOnceStream.archiveType}` : '-'"
            disabled
          />
        </el-form-item>
        <el-form-item label="Runner 主机" prop="runnerHostId">
          <el-select v-model="runLogArchiveOnceForm.runnerHostId" placeholder="请选择 Runner" filterable style="width: 100%;">
            <el-option
              v-for="item in runnerHosts.filter(host => host.enabled)"
              :key="item.id"
              :label="`${item.name}（${item.statusText || item.status}${item.host ? ` / ${item.host}:${item.port || 22}` : ''}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="指定文件">
          <el-input v-model="runLogArchiveOnceForm.fileName" placeholder="可选，例如 binlog.000123；为空则自动选择下一份或最新 binlog" />
          <div class="field-tip">文件名必须存在于源库当前 binlog 列表中。为空时后端根据归档流最近文件自动选择。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="runLogArchiveOnceDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="runLogArchiveOnceSubmitting" @click="submitRunLogArchiveOnce">下发归档任务</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runLogArchiveCatchUpDialogVisible"
      title="binlog 追平归档"
      width="640px"
      @close="resetRunLogArchiveCatchUpForm"
    >
      <el-alert
        title="追平归档会按归档流最近文件向后拉取多份已轮转 binlog；默认不包含当前活跃 binlog，也不会常驻运行或修改源库保留策略。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form
        ref="runLogArchiveCatchUpFormRef"
        :model="runLogArchiveCatchUpForm"
        :rules="runLogArchiveCatchUpRules"
        label-width="120px"
      >
        <el-form-item label="归档流">
          <el-input
            :model-value="runLogArchiveCatchUpStream ? `${runLogArchiveCatchUpStream.instanceName || `#${runLogArchiveCatchUpStream.instanceId}`} / ${runLogArchiveCatchUpStream.archiveTypeText || runLogArchiveCatchUpStream.archiveType}` : '-'"
            disabled
          />
        </el-form-item>
        <el-form-item label="Runner 主机" prop="runnerHostId">
          <el-select v-model="runLogArchiveCatchUpForm.runnerHostId" placeholder="请选择 Runner" filterable style="width: 100%;">
            <el-option
              v-for="item in runnerHosts.filter(host => host.enabled)"
              :key="item.id"
              :label="`${item.name}（${item.statusText || item.status}${item.host ? ` / ${item.host}:${item.port || 22}` : ''}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="最大文件数" prop="maxFiles">
              <el-input-number v-model="runLogArchiveCatchUpForm.maxFiles" :min="1" :max="20" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="包含活跃文件">
              <el-switch v-model="runLogArchiveCatchUpForm.includeCurrent" active-text="包含" inactive-text="不包含" />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="field-tip">如果归档流最近文件已经被源库 purge，后端会拒绝追平，以避免静默断链。</div>
      </el-form>
      <template #footer>
        <el-button @click="runLogArchiveCatchUpDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="runLogArchiveCatchUpSubmitting" @click="submitRunLogArchiveCatchUp">下发追平任务</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="protectionWizardDialogVisible"
      title="启用 MySQL/MariaDB 物理 PITR 保护"
      width="920px"
      @close="resetProtectionWizardForm"
    >
      <el-alert
        title="向导会自动创建或复用 binlog 归档流和物理备份策略，默认立即 Full 一次、每天 03:00 增量、每 5 条增量自动合成新全量基线。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="protectionWizardFormRef" :model="protectionWizardForm" :rules="protectionWizardRules" label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="数据库实例" prop="instanceId">
              <el-select v-model="protectionWizardForm.instanceId" placeholder="请选择 MySQL/MariaDB 实例" filterable style="width: 100%;" @change="handleProtectionWizardInstanceChange">
                <el-option
                  v-for="item in mysqlPhysicalBackupPolicyInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.dbTypeText || item.dbType}${item.version ? ` / ${item.version}` : ''}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="保护模板">
              <el-select v-model="protectionWizardForm.templateKey" style="width: 100%;">
                <el-option label="物理 PITR - 滚动合成全量" value="rolling_synthetic_full" />
                <el-option label="物理 PITR - 保守策略" value="conservative_pitr" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Runner 主机" prop="runnerHostId">
              <el-select v-model="protectionWizardForm.runnerHostId" placeholder="请选择 Runner" filterable style="width: 100%;">
                <el-option
                  v-for="item in runnerHosts.filter(host => host.enabled !== false)"
                  :key="item.id"
                  :label="`${item.name}（${item.statusText || item.status}${item.host ? ` / ${item.host}:${item.port || 22}` : ''}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="存储配置">
              <el-select v-model="protectionWizardForm.storageProfileId" placeholder="可选；默认 runner:// 本地" clearable filterable style="width: 100%;">
                <el-option v-for="item in storageProfiles" :key="item.id" :label="`${item.name}（${item.storageTypeText || item.storageType}）`" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="备份工具" prop="backupEngine">
              <el-select v-model="protectionWizardForm.backupEngine" style="width: 100%;">
                <el-option v-for="item in protectionWizardBackupEngineOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="工具模式">
              <el-select v-model="protectionWizardForm.toolExecutionMode" style="width: 100%;">
                <el-option label="宿主机工具" value="host_tools" />
                <el-option label="容器化工具" value="container_tools" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="protectionWizardForm.toolExecutionMode === 'container_tools'" :gutter="16">
          <el-col :span="8">
            <el-form-item label="工具镜像">
              <el-input v-model="protectionWizardForm.toolImage" placeholder="如 opshub-runner-tools:mysql80" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="datadir 挂载">
              <el-input v-model="protectionWizardForm.containerDatadirPath" placeholder="/var/lib/mysql" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="网络模式">
              <el-input v-model="protectionWizardForm.containerNetworkMode" placeholder="host" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="立即 Full">
              <el-switch v-model="protectionWizardForm.runInitialFullNow" active-text="启用后执行" inactive-text="只保存策略" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="Full Cron">
              <el-input v-model="protectionWizardForm.fullSchedule" placeholder="留空表示不定期原生 Full" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="增量 Cron" prop="incrementalSchedule">
              <el-input v-model="protectionWizardForm.incrementalSchedule" placeholder="0 3 * * *" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="归档模式">
              <el-select v-model="protectionWizardForm.binlogArchiveMode" style="width: 100%;">
                <el-option label="polling" value="polling" />
                <el-option label="streaming" value="streaming" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="RPO 秒">
              <el-input-number v-model="protectionWizardForm.binlogRpoTargetSeconds" :min="30" :max="86400" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="binlog 保留">
              <el-input-number v-model="protectionWizardForm.binlogRetentionDays" :min="1" :max="3650" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="自动合成">
              <el-switch v-model="protectionWizardForm.syntheticEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="触发增量数">
              <el-input-number v-model="protectionWizardForm.syntheticTriggerAfterIncrementals" :min="1" :max="365" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="合并增量数">
              <el-input-number v-model="protectionWizardForm.syntheticMergeOldestIncrementals" :min="1" :max="365" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <div v-if="protectionWizardPreview" class="wizard-preview">
        <el-alert
          v-if="protectionWizardPreview.blockingReasons?.length"
          :title="protectionWizardPreview.blockingReasons.join('；')"
          type="error"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-alert
          v-else-if="protectionWizardPreview.warnings?.length"
          :title="protectionWizardPreview.warnings.join('；')"
          type="warning"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-table :data="protectionWizardPreview.actions || []" size="small" stripe>
          <el-table-column label="动作" width="100">
            <template #default="{ row }">{{ row.action }}</template>
          </el-table-column>
          <el-table-column label="资源" min-width="160">
            <template #default="{ row }">{{ row.resourceType }}<span v-if="row.resourceId"> #{{ row.resourceId }}</span></template>
          </el-table-column>
          <el-table-column label="名称" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="说明" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{ row.message || '-' }}</template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="protectionWizardDialogVisible = false">取消</el-button>
        <el-button :loading="protectionWizardPreviewing" @click="previewProtectionWizard">预览</el-button>
        <el-button type="primary" :disabled="!!protectionWizardPreview && !protectionWizardPreview.canApply" :loading="protectionWizardSubmitting" @click="applyProtectionWizard">确认启用</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="postgresBarmanWizardDialogVisible"
      title="启用 PostgreSQL Barman PITR 保护"
      width="min(1180px, calc(100vw - 48px))"
      class="postgres-barman-wizard-dialog"
      top="5vh"
      @close="resetPostgresBarmanWizardForm"
    >
      <el-alert
        title="向导会登记或复用 Barman Server，并可同时下发 check、catalog sync、WAL sync 和可选 cluster 级备份；不会在 backend 容器内直接执行 Barman。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="postgresBarmanWizardFormRef" :model="postgresBarmanWizardForm" :rules="postgresBarmanWizardRules" label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="PostgreSQL 实例" prop="instanceId">
              <el-select v-model="postgresBarmanWizardForm.instanceId" placeholder="请选择 PostgreSQL 实例" filterable style="width: 100%;" @change="handlePostgresBarmanWizardInstanceChange">
                <el-option
                  v-for="item in postgresqlBackupInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Runner 主机" prop="runnerHostId">
              <el-select v-model="postgresBarmanWizardForm.runnerHostId" placeholder="请选择 SSH Runner" filterable style="width: 100%;">
                <el-option
                  v-for="item in runnerHosts.filter(host => host.runnerType === 'ssh' && host.enabled !== false)"
                  :key="item.id"
                  :label="`${item.name}（${item.statusText || item.status}${item.host ? ` / ${item.host}:${item.port || 22}` : ''}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="显示名称">
              <el-input v-model="postgresBarmanWizardForm.name" placeholder="如 pg-prod-Barman-PITR保护" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Barman server" prop="barmanServerName">
              <el-input v-model="postgresBarmanWizardForm.barmanServerName" placeholder="barman.conf 中的 server name" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="复用 Server">
              <el-select v-model="postgresBarmanWizardForm.reuseBarmanServerId" placeholder="可选；自动匹配当前实例" clearable filterable style="width: 100%;">
                <el-option
                  v-for="item in barmanServers.filter(server => !postgresBarmanWizardForm.instanceId || server.sourceInstanceId === postgresBarmanWizardForm.instanceId)"
                  :key="item.id"
                  :label="`${item.name} / ${item.barmanServerName}`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="保留策略">
              <el-input v-model="postgresBarmanWizardForm.retentionPolicy" placeholder="RECOVERY WINDOW OF 30 DAYS" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="Barman home">
              <el-input v-model="postgresBarmanWizardForm.barmanHome" placeholder="/var/lib/barman，可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="配置文件">
              <el-input v-model="postgresBarmanWizardForm.configPath" placeholder="/etc/barman.conf，可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备份方法">
              <el-input v-model="postgresBarmanWizardForm.backupMethod" placeholder="postgres / rsync" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="archive">
              <el-switch v-model="postgresBarmanWizardForm.archiverEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="streaming">
              <el-switch v-model="postgresBarmanWizardForm.streamingArchiverEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="slot">
              <el-input v-model="postgresBarmanWizardForm.slotName" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16" class="postgres-barman-wizard-switch-row">
          <el-col :span="12">
            <el-form-item label="Barman check">
              <el-switch v-model="postgresBarmanWizardForm.runCheckNow" active-text="下发" inactive-text="跳过" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Catalog sync">
              <el-switch v-model="postgresBarmanWizardForm.syncCatalogNow" active-text="下发" inactive-text="跳过" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="WAL sync">
              <el-switch v-model="postgresBarmanWizardForm.syncWalNow" active-text="下发" inactive-text="跳过" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="立即备份">
              <el-switch v-model="postgresBarmanWizardForm.runInitialBackupNow" active-text="下发" inactive-text="跳过" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="配置 JSON">
          <el-input
            v-model="postgresBarmanWizardForm.configJson"
            type="textarea"
            :rows="3"
            placeholder="可选；只保存非敏感摘要，密钥仍走 Runner/凭据配置"
          />
        </el-form-item>
      </el-form>
      <div v-if="postgresBarmanWizardPreview" class="wizard-preview">
        <el-alert
          v-if="postgresBarmanWizardPreview.blockingReasons?.length"
          :title="postgresBarmanWizardPreview.blockingReasons.join('；')"
          type="error"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-alert
          v-else-if="postgresBarmanWizardPreview.warnings?.length"
          :title="postgresBarmanWizardPreview.warnings.join('；')"
          type="warning"
          show-icon
          :closable="false"
          class="backup-dialog-alert"
        />
        <el-table :data="postgresBarmanWizardPreview.actions || []" size="small" stripe>
          <el-table-column label="动作" width="100">
            <template #default="{ row }">{{ row.action }}</template>
          </el-table-column>
          <el-table-column label="资源" min-width="160">
            <template #default="{ row }">{{ row.resourceType }}<span v-if="row.resourceId"> #{{ row.resourceId }}</span></template>
          </el-table-column>
          <el-table-column label="名称" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="说明" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{ row.message || '-' }}</template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="postgresBarmanWizardDialogVisible = false">取消</el-button>
        <el-button :loading="postgresBarmanWizardPreviewing" @click="previewPostgresBarmanWizard">预览</el-button>
        <el-button type="primary" :disabled="!!postgresBarmanWizardPreview && !postgresBarmanWizardPreview.canApply" :loading="postgresBarmanWizardSubmitting" @click="applyPostgresBarmanWizard">确认启用</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="protectionRestoreDrillDialogVisible"
      :title="`恢复演练${protectionRestoreDrillProfile ? ` - ${protectionRestoreDrillProfile.instanceName}` : ''}`"
      width="800px"
      @close="resetProtectionRestoreDrillForm"
    >
      <el-alert
        title="该入口会自动生成恢复计划并下发隔离恢复 Runner 任务；不会覆盖生产库，也不会切换业务连接。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="protectionRestoreDrillFormRef" :model="protectionRestoreDrillForm" :rules="protectionRestoreDrillRules" label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="目标类型" prop="restoreTargetType">
              <el-select v-model="protectionRestoreDrillForm.restoreTargetType" style="width: 100%;">
                <el-option label="按时间点" value="time" />
                <el-option v-if="protectionRestoreDrillProfile?.engine === 'postgresql'" label="按 LSN" value="lsn" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="protectionRestoreDrillForm.restoreTargetType === 'lsn' ? '目标 LSN' : '目标时间'" prop="restoreTargetValue">
              <el-date-picker
                v-if="protectionRestoreDrillForm.restoreTargetType !== 'lsn'"
                v-model="protectionRestoreDrillForm.restoreTargetValue"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                placeholder="选择恢复目标时间"
                style="width: 100%;"
              />
              <el-input v-else v-model="protectionRestoreDrillForm.restoreTargetValue" placeholder="如：A/18000098" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Runner 主机" prop="runnerHostId">
              <el-select v-model="protectionRestoreDrillForm.runnerHostId" placeholder="请选择 SSH Runner" filterable style="width: 100%;">
                <el-option
                  v-for="item in runnerHosts.filter(host => host.runnerType === 'ssh' && host.enabled !== false)"
                  :key="item.id"
                  :label="`${item.name}（${item.host || item.runnerType}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="容器镜像">
              <el-input v-model="protectionRestoreDrillForm.containerImage" placeholder="留空使用默认 mysql/mariadb/postgres 镜像" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="监听端口">
              <el-input-number v-model="protectionRestoreDrillForm.listenPort" :min="0" :max="65535" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="保留小时" prop="expiresInHours">
              <el-input-number v-model="protectionRestoreDrillForm.expiresInHours" :min="1" :max="168" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="额外校验 SQL">
          <el-input
            v-model="protectionRestoreDrillForm.validationSqlText"
            type="textarea"
            :rows="4"
            placeholder="每行一条只读 SQL；断言校验可在底层恢复任务详情继续查看 proof"
          />
        </el-form-item>
        <el-form-item label="断言校验">
          <div class="restore-assertions">
            <div class="restore-assertions-header">
              <span class="field-tip">expectedRows 校验行数；expectedScalar 校验首行首列；expectedContains 校验输出中包含固定文本。</span>
              <el-button size="small" type="primary" plain @click="addProtectionRestoreDrillAssertion">添加断言</el-button>
            </div>
            <div v-if="!protectionRestoreDrillForm.validationAssertions?.length" class="restore-empty-tip">未配置断言时，只记录校验 SQL 输出和执行状态。</div>
            <div
              v-for="(item, index) in protectionRestoreDrillForm.validationAssertions"
              :key="index"
              class="restore-assertion-item"
            >
              <div class="restore-assertion-toolbar">
                <span>断言 {{ index + 1 }}</span>
                <el-button link type="danger" @click="removeProtectionRestoreDrillAssertion(index)">删除</el-button>
              </div>
              <el-input
                v-model="item.sql"
                type="textarea"
                :rows="2"
                placeholder="只允许只读 SQL，例如 SELECT COUNT(*) FROM orders"
              />
              <el-row :gutter="10" class="restore-assertion-fields">
                <el-col :span="8">
                  <el-input-number v-model="item.expectedRows" :min="0" :max="1000000000" placeholder="expectedRows" class="assertion-number" />
                </el-col>
                <el-col :span="8">
                  <el-input v-model="item.expectedScalar" placeholder="expectedScalar" clearable />
                </el-col>
                <el-col :span="8">
                  <el-input v-model="item.expectedContains" placeholder="expectedContains" clearable />
                </el-col>
              </el-row>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="风险确认" prop="confirmIsolated">
          <el-checkbox v-model="protectionRestoreDrillForm.confirmIsolated">我确认本次只恢复到 Runner 隔离环境，不覆盖生产数据。</el-checkbox>
        </el-form-item>
      </el-form>
      <el-alert
        v-if="protectionRestoreDrillResult"
        :title="protectionRestoreDrillResult.message || protectionRestoreDrillResult.status"
        :type="protectionRestoreDrillResult.job ? 'success' : protectionRestoreDrillResult.status === 'plan_failed' ? 'error' : 'warning'"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <template #footer>
        <el-button @click="protectionRestoreDrillDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="protectionRestoreDrillSubmitting" @click="submitProtectionRestoreDrill">生成并执行</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="restorePlanDialogVisible"
      title="生成 PITR 恢复计划"
      width="680px"
      @close="resetRestorePlanForm"
    >
      <el-alert
        title="恢复计划会检查备份链、日志链、存储对象和工具兼容状态。PostgreSQL 支持 Barman / pg_basebackup 的 target time / target LSN 预校验；从 pg_basebackup full 记录进入时会固定 base。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="restorePlanFormRef" :model="restorePlanForm" :rules="restorePlanRules" label-width="120px">
        <el-form-item label="来源实例" prop="sourceInstanceId">
          <el-select v-model="restorePlanForm.sourceInstanceId" placeholder="请选择来源实例" filterable style="width: 100%;">
            <el-option v-for="item in pitrBackupInstances" :key="item.id" :label="`${item.name}（${item.dbTypeText || item.dbType}）`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="restorePlanForm.baseRecordId" label="指定 Base">
          <el-input :model-value="restorePlanBaseRecordLabel" disabled />
          <div class="field-tip">从 pg_basebackup full 备份记录发起时会固定该 base；后端仍会重新校验 WAL、timeline 和增量链。</div>
        </el-form-item>
        <el-form-item label="目标实例">
          <el-select v-model="restorePlanForm.targetInstanceId" placeholder="可选，建议选择隔离恢复库" clearable filterable style="width: 100%;">
            <el-option
              v-for="item in pitrRestoreTargetInstances.filter(target => target.id !== restorePlanForm.sourceInstanceId)"
              :key="item.id"
              :label="`${item.name}（${item.dbTypeText || item.dbType}${item.environment ? ` / ${item.environment}` : ''}）`"
              :value="item.id"
            />
          </el-select>
          <div class="field-tip">目标实例必须是非生产库；P1 只记录计划，不执行恢复。</div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="恢复模式">
              <el-input v-model="restorePlanForm.restoreMode" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="目标类型" prop="restoreTargetType">
              <el-select v-model="restorePlanForm.restoreTargetType" style="width: 100%;">
                <el-option label="按时间点" value="time" />
                <el-option v-if="restorePlanSourceDbType === 'postgresql'" label="按 LSN" value="lsn" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="restorePlanForm.restoreTargetType === 'lsn' ? '目标 LSN' : '目标时间'" prop="restoreTargetValue">
          <el-date-picker
            v-if="restorePlanForm.restoreTargetType !== 'lsn'"
            v-model="restorePlanForm.restoreTargetValue"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="选择恢复目标时间"
            style="width: 100%;"
          />
          <el-input v-else v-model="restorePlanForm.restoreTargetValue" placeholder="如：A/18000098" />
        </el-form-item>
        <el-form-item v-if="restorePlanSourceDbType === 'postgresql'" label="目标 Timeline">
          <el-input v-model="restorePlanForm.targetTimelineId" placeholder="可选，如 1 或 00000001；为空时使用 base backup timeline" />
        </el-form-item>
        <el-form-item label="包含目标点">
          <el-switch v-model="restorePlanForm.restoreTargetInclusive" active-text="包含" inactive-text="不包含" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="restorePlanDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="restorePlanSubmitting" @click="submitRestorePlan">生成计划</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="restorePlanRunDialogVisible"
      title="执行隔离恢复"
      width="760px"
      @close="resetRestorePlanRunForm"
    >
      <el-alert
        :title="isPostgreSQLRestoreRun ? postgresqlRestoreRunAlertTitle : 'P2.7 只恢复到 Runner 主机上的隔离容器，不覆盖生产库、不切换业务连接、不自动回填数据。'"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="restorePlanRunFormRef" :model="restorePlanRunForm" :rules="restorePlanRunRules" label-width="130px">
        <el-form-item label="恢复计划">
          <el-input :model-value="restorePlanRunSource ? `#${restorePlanRunSource.id} / ${restorePlanRunSource.restoreTargetValue}` : '-'" disabled />
        </el-form-item>
        <el-form-item label="Runner 主机" prop="runnerHostId">
          <el-select v-model="restorePlanRunForm.runnerHostId" placeholder="请选择 SSH Runner" filterable style="width: 100%;">
            <el-option
              v-for="item in runnerHosts.filter(host => host.runnerType === 'ssh' && host.enabled !== false)"
              :key="item.id"
              :label="`${item.name}（${item.host || item.runnerType}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="容器镜像">
              <el-input v-model="restorePlanRunForm.containerImage" :placeholder="isPostgreSQLRestoreRun ? 'postgres:16 / postgres:15 / postgres:latest' : 'mysql:8.0 / mysql:8.4 / mariadb:latest'" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="监听端口">
              <el-input-number v-model="restorePlanRunForm.listenPort" :min="0" :max="65535" class="query-number" />
              <div class="field-tip">0 表示由后端自动分配本机端口。</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="isPostgreSQLRestoreRun" :gutter="16">
          <el-col :span="6">
            <el-form-item label="启动实例">
              <el-switch v-model="restorePlanRunForm.postgresStartInstance" active-text="启动" inactive-text="仅目录" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="目标 Timeline">
              <el-input v-model="restorePlanRunForm.targetTimelineId" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="目标动作">
              <el-select v-model="restorePlanRunForm.targetAction" style="width: 100%;">
                <el-option label="pause" value="pause" />
                <el-option label="shutdown" value="shutdown" />
                <el-option label="promote" value="promote" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="isBarmanRestoreRun" :span="6">
            <el-form-item label="Get WAL">
              <el-switch v-model="restorePlanRunForm.barmanGetWal" active-text="--get-wal" inactive-text="--no-get-wal" />
            </el-form-item>
          </el-col>
          <el-col v-else-if="isPgBaseBackupRestoreRun" :span="6">
            <el-form-item label="WAL 来源">
              <el-tag type="info">恢复计划选择</el-tag>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="保留小时" prop="expiresInHours">
              <el-input-number v-model="restorePlanRunForm.expiresInHours" :min="1" :max="168" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="失败后清理">
              <el-switch
                v-model="restorePlanRunForm.cleanupOnFailure"
                :active-text="isPostgreSQLRestoreRun ? '清理目录/容器' : '清理容器'"
                inactive-text="保留现场"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="!isPostgreSQLRestoreRun || restorePlanRunForm.postgresStartInstance !== false" label="额外校验 SQL">
          <el-input
            v-model="restorePlanRunForm.validationSqlText"
            type="textarea"
            :rows="5"
            :placeholder="isPostgreSQLRestoreRun ? '每行一条 PostgreSQL 只读 SQL，例如 SELECT COUNT(*) FROM public.orders' : '每行一条，只允许 SELECT / SHOW / DESC / DESCRIBE / EXPLAIN'"
          />
        </el-form-item>
        <el-form-item v-if="!isPostgreSQLRestoreRun || restorePlanRunForm.postgresStartInstance !== false" label="断言校验">
          <div class="restore-assertions">
            <div class="restore-assertions-header">
              <span class="field-tip">expectedRows 校验结果行数；expectedScalar 校验首行首列；expectedContains 校验输出中包含固定文本。</span>
              <el-button size="small" type="primary" plain @click="addRestoreValidationAssertion">添加断言</el-button>
            </div>
            <div v-if="!restorePlanRunForm.validationAssertions?.length" class="restore-empty-tip">未配置断言时，只记录校验 SQL 输出和执行状态。</div>
            <div
              v-for="(item, index) in restorePlanRunForm.validationAssertions"
              :key="index"
              class="restore-assertion-item"
            >
              <div class="restore-assertion-toolbar">
                <span>断言 {{ index + 1 }}</span>
                <el-button link type="danger" @click="removeRestoreValidationAssertion(index)">删除</el-button>
              </div>
              <el-input
                v-model="item.sql"
                type="textarea"
                :rows="2"
                placeholder="只允许只读 SQL，例如 SELECT COUNT(*) FROM orders"
              />
              <el-row :gutter="10" class="restore-assertion-fields">
                <el-col :span="8">
                  <el-input-number v-model="item.expectedRows" :min="0" :max="1000000000" placeholder="expectedRows" class="assertion-number" />
                </el-col>
                <el-col :span="8">
                  <el-input v-model="item.expectedScalar" placeholder="expectedScalar" clearable />
                </el-col>
                <el-col :span="8">
                  <el-input v-model="item.expectedContains" placeholder="expectedContains" clearable />
                </el-col>
              </el-row>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="风险确认" prop="confirmIsolated">
          <el-checkbox v-model="restorePlanRunForm.confirmIsolated">
            {{ isPostgreSQLRestoreRun ? '我确认本次只恢复到 Runner 隔离目录或隔离 PostgreSQL 实例，不覆盖生产数据。' : '我确认本次只恢复到隔离库，不覆盖生产数据。' }}
          </el-checkbox>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="restorePlanRunDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="restorePlanRunSubmitting" @click="submitRunRestorePlan">下发恢复任务</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="restoreProofDialogVisible" :title="restoreProofTitle || '恢复证明'" width="860px">
      <pre class="audit-detail-pre">{{ restoreProofContent }}</pre>
      <template #footer>
        <el-button @click="restoreProofDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="restoreJobDetailVisible"
      :title="restoreJobDetailTitle || '恢复任务详情'"
      width="960px"
      :destroy-on-close="true"
    >
      <div v-if="restoreJobDetailRow" class="restore-job-detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="任务ID">#{{ restoreJobDetailRow.id }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="backupStatusTag(restoreJobDetailRow.status)" size="small">
              {{ restoreJobDetailRow.statusText || restoreJobDetailRow.status || '-' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="来源实例">
            {{ restoreJobDetailRow.sourceInstanceName || `#${restoreJobDetailRow.sourceInstanceId}` }}
          </el-descriptions-item>
          <el-descriptions-item label="目标实例">
            {{ restoreJobDetailRow.targetInstanceName || (restoreJobDetailRow.targetInstanceId ? `#${restoreJobDetailRow.targetInstanceId}` : '-') }}
          </el-descriptions-item>
          <el-descriptions-item label="Runner">
            {{ restoreJobDetailRow.runnerHostName || (restoreJobDetailRow.runnerHostId ? `#${restoreJobDetailRow.runnerHostId}` : '-') }}
          </el-descriptions-item>
          <el-descriptions-item label="模式 / 策略">
            {{ restoreJobDetailRow.restoreModeText || restoreJobDetailRow.restoreMode || '-' }}
            <span v-if="restoreJobDetailRow.restoreStrategyText || restoreJobDetailRow.restoreStrategy" class="muted-text">
              / {{ restoreJobDetailRow.restoreStrategyText || restoreJobDetailRow.restoreStrategy }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="恢复目标">
            {{ restoreJobDetailRow.restoreTargetType || '-' }}
            <span v-if="restoreJobDetailRow.restoreTargetValue" class="muted-text"> / {{ restoreJobDetailRow.restoreTargetValue }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="隔离库">
            <span v-if="restoreJobDetailRow.listenPort">
              {{ restoreJobDetailRow.listenHost || '127.0.0.1' }}:{{ restoreJobDetailRow.listenPort }}
            </span>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="容器">{{ restoreJobDetailRow.containerName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="镜像">{{ restoreJobDetailRow.containerImage || '-' }}</el-descriptions-item>
          <el-descriptions-item label="工作目录" :span="2">{{ restoreJobDetailRow.workDir || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Prepared datadir" :span="2">{{ restoreJobDetailRow.preparedDatadir || '-' }}</el-descriptions-item>
          <el-descriptions-item label="日志路径" :span="2">{{ restoreJobDetailRow.logPath || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Artifact" :span="2">{{ restoreJobDetailRow.artifactUri || '-' }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ restoreJobDetailRow.startedAt || '-' }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ restoreJobDetailRow.finishedAt || '-' }}</el-descriptions-item>
          <el-descriptions-item label="总耗时">{{ formatRestoreJobDuration(restoreJobDetailRow.durationMs) }}</el-descriptions-item>
          <el-descriptions-item label="操作者">{{ restoreJobDetailRow.operatorName || '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="restoreJobDetailRow.message" label="结果摘要" :span="2">
            {{ restoreJobDetailRow.message }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="restore-job-detail-section">
          <div class="panel-title">
            <span>步骤时间线</span>
            <el-tag size="small" type="info">{{ restoreJobSteps.length }}</el-tag>
          </div>
          <el-empty v-if="restoreJobSteps.length === 0" description="暂无步骤记录" :image-size="64" />
          <el-timeline v-else class="restore-job-timeline">
            <el-timeline-item
              v-for="(step, index) in restoreJobSteps"
              :key="`${step.name}-${index}`"
              :type="restoreStepTimelineType(step.status)"
              :timestamp="step.finishedAt || step.startedAt || ''"
              placement="top"
            >
              <div class="restore-step-row">
                <span class="restore-step-name">{{ step.label }}</span>
                <el-tag size="small" :type="restoreStepTagType(step.status)">{{ step.statusText }}</el-tag>
                <span v-if="step.durationMs !== null" class="muted-text">耗时 {{ formatRestoreJobDuration(step.durationMs) }}</span>
              </div>
              <div v-if="step.startedAt && step.finishedAt && step.startedAt !== step.finishedAt" class="muted-text">
                {{ step.startedAt }} → {{ step.finishedAt }}
              </div>
              <div v-else-if="step.startedAt && !step.finishedAt" class="muted-text">开始于 {{ step.startedAt }}</div>
            </el-timeline-item>
          </el-timeline>
        </div>

        <div v-if="restoreJobValidations.length > 0" class="restore-job-detail-section">
          <div class="panel-title">
            <span>校验 SQL 结果</span>
            <el-tag size="small" type="info">{{ restoreJobValidations.length }}</el-tag>
          </div>
          <el-table :data="restoreJobValidations" stripe size="small">
            <el-table-column label="#" prop="index" width="50" align="center" />
            <el-table-column label="SQL" prop="sql" min-width="240" show-overflow-tooltip />
            <el-table-column label="执行" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">
                  {{ row.status === 'success' ? '成功' : '失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="断言" width="120" align="center">
              <template #default="{ row }">
                <el-tag v-if="row.assertionStatus" size="small" :type="restoreAssertionTagType(row.assertionStatus)">
                  {{ restoreAssertionStatusText(row.assertionStatus) }}
                </el-tag>
                <span v-else class="muted-text">-</span>
              </template>
            </el-table-column>
            <el-table-column label="期望" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ formatRestoreAssertionExpected(row) }}</template>
            </el-table-column>
            <el-table-column label="实际" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ formatRestoreAssertionActual(row) }}</template>
            </el-table-column>
            <el-table-column label="说明 / 输出预览" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">{{ row.assertionMessage || row.outputPreview || '-' }}</template>
            </el-table-column>
          </el-table>
        </div>
      </div>
      <template #footer>
        <el-button v-if="restoreJobDetailRow?.proofJson" type="primary" plain @click="viewRestoreJobDetailProof">查看 Proof</el-button>
        <el-button @click="restoreJobDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerHostDialogVisible"
      :title="runnerHostForm.id ? '编辑 Runner 主机' : '新增 Runner 主机'"
      width="860px"
      @close="resetRunnerHostForm"
    >
      <el-alert
        title="P2.2 首版只执行内置探测脚本，不开放自定义 shell；凭据复用资产凭据，Runner 配置中不能保存密码、Token、私钥或对象存储密钥。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="runnerHostFormRef" :model="runnerHostForm" :rules="runnerHostRules" label-width="120px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="runnerHostForm.name" placeholder="如：backup-runner-192.168.1.15" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Runner 类型" prop="runnerType">
              <el-select v-model="runnerHostForm.runnerType" style="width: 100%;">
                <el-option label="SSH Runner" value="ssh" />
                <el-option label="本地 Runner（后续）" value="local" disabled />
                <el-option label="Agent Runner" value="agent" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="14">
            <el-form-item label="主机地址" prop="host">
              <el-input v-model="runnerHostForm.host" placeholder="192.168.1.15" />
            </el-form-item>
          </el-col>
          <el-col :span="10">
            <el-form-item label="SSH 端口">
              <el-input-number v-model="runnerHostForm.port" :min="1" :max="65535" class="query-number" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="SSH 凭据" prop="credentialId">
          <el-select v-model="runnerHostForm.credentialId" placeholder="请选择 SSH 凭据" filterable style="width: 100%;">
            <el-option
              v-for="item in sshCredentialOptions"
              :key="item.id"
              :label="`${item.name} (${item.username || '无用户名'})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="工作目录">
              <el-input v-model="runnerHostForm.workDir" placeholder="/var/lib/opshub/database-runner" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="仓库挂载点">
              <el-input v-model="runnerHostForm.storageMountPath" placeholder="/backup/opshub，可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="最大并发">
              <el-input-number v-model="runnerHostForm.maxConcurrentJobs" :min="1" :max="100" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="超时分钟">
              <el-input-number v-model="runnerHostForm.timeoutMinutes" :min="1" :max="1440" class="query-number" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-switch v-model="runnerHostForm.enabled" active-text="启用" inactive-text="禁用" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="CPU 策略">
              <el-input v-model="runnerHostForm.cpuLimit" placeholder="如 nice=10" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="IO 策略">
              <el-input v-model="runnerHostForm.ioLimit" placeholder="如 ionice=be:7" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="带宽策略">
              <el-input v-model="runnerHostForm.bandwidthLimit" placeholder="如 50MB/s" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="配置 JSON">
          <el-input
            v-model="runnerHostForm.configJson"
            type="textarea"
            :rows="3"
            placeholder='可选，仅保存非敏感摘要，例如 {"labels":["mysql-backup"],"network":"lan"}'
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="runnerHostDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="runnerHostSubmitting" @click="submitRunnerHost">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerToolProfileDialogVisible"
      title="Runner 工具画像"
      width="980px"
    >
      <el-alert
        title="工具画像来自最近一次 SSH 巡检；这里只展示可见能力和版本，不保存任何密钥。安装脚本需要基于画像生成，不会自动执行。"
        type="info"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-skeleton v-if="runnerToolProfileLoading" :rows="6" animated />
      <template v-else-if="runnerToolProfile">
        <el-descriptions :column="3" border class="backup-detail-descriptions">
          <el-descriptions-item label="Runner">{{ runnerToolProfile.runnerName || `#${runnerToolProfile.runnerHostId}` }}</el-descriptions-item>
          <el-descriptions-item label="OS">{{ runnerToolProfile.osPrettyName || `${runnerToolProfile.osFamily || '-'} ${runnerToolProfile.osVersion || ''}` }}</el-descriptions-item>
          <el-descriptions-item label="架构">{{ runnerToolProfile.arch || '-' }}</el-descriptions-item>
          <el-descriptions-item label="包管理器">{{ runnerToolProfile.packageManager || '-' }}</el-descriptions-item>
          <el-descriptions-item label="权限">{{ runnerToolProfile.isRoot ? 'root' : (runnerToolProfile.hasSudo ? 'sudo 可用' : '需要安装权限') }}</el-descriptions-item>
          <el-descriptions-item label="最近巡检">{{ runnerToolProfile.lastProbeAt || '-' }}</el-descriptions-item>
        </el-descriptions>
        <div class="runner-tool-section">
          <div class="runner-config-title">
            <span>能力摘要</span>
          </div>
          <div class="runner-tool-tags">
            <el-tag v-for="item in runnerToolProfile.capability?.available || []" :key="`ok-${item}`" type="success" size="small">{{ item }}</el-tag>
            <el-tag v-for="item in runnerToolProfile.capability?.missing || []" :key="`miss-${item}`" type="info" size="small">{{ item }} 缺失</el-tag>
          </div>
          <el-alert
            v-for="item in runnerToolWarnings"
            :key="item"
            :title="item"
            type="warning"
            show-icon
            :closable="false"
            class="backup-dialog-alert compact-alert"
          />
        </div>
        <el-table :data="runnerToolRows" stripe class="modern-table">
          <el-table-column prop="name" label="工具" width="170" />
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.installed ? 'success' : 'info'">{{ row.installed ? '已安装' : '缺失' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="path" label="路径" min-width="220" show-overflow-tooltip />
          <el-table-column prop="version" label="版本" min-width="260" show-overflow-tooltip />
        </el-table>
      </template>
      <el-empty v-else description="暂无工具画像，请先点击 Runner 主机列表里的“巡检”" />
      <template #footer>
        <el-button @click="runnerToolProfileDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerToolScriptDialogVisible"
      title="生成 Runner 工具安装脚本"
      width="1040px"
    >
      <el-alert
        title="可先生成脚本审阅；点击执行安装会创建 Runner 工具安装任务，只运行白名单安装流程并写入审计。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form label-width="130px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="Runner">
              <el-input :model-value="runnerToolScriptHost?.name || '-'" readonly />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="安装模式">
              <el-select v-model="runnerToolScriptForm.installMode" style="width: 100%;">
                <el-option label="在线仓库" value="online" />
                <el-option label="离线清单" value="offline" />
              </el-select>
            </el-form-item>
          </el-col>
	          <el-col :span="6">
	            <el-form-item label="Dry Run">
	              <el-switch v-model="runnerToolScriptForm.dryRun" active-text="是" inactive-text="否" />
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-row :gutter="16">
	          <el-col :span="6">
	            <el-form-item label="执行模式">
	              <el-select v-model="runnerToolScriptForm.executionMode" style="width: 100%;">
	                <el-option label="宿主机安装" value="host_tools" />
	                <el-option label="容器化工具" value="container_tools" />
	              </el-select>
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="工具镜像">
	              <el-input v-model="runnerToolScriptForm.toolImage" :disabled="runnerToolScriptForm.executionMode !== 'container_tools'" placeholder="opshub-runner-tools:mysql80" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="datadir挂载">
	              <el-input v-model="runnerToolScriptForm.datadirMount" :disabled="runnerToolScriptForm.executionMode !== 'container_tools'" placeholder="/var/lib/mysql" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="网络/只读">
	              <div class="inline-control-row">
	                <el-input v-model="runnerToolScriptForm.networkMode" :disabled="runnerToolScriptForm.executionMode !== 'container_tools'" placeholder="host" />
	                <el-switch v-model="runnerToolScriptForm.readOnlyDatadir" :disabled="runnerToolScriptForm.executionMode !== 'container_tools'" active-text="RO" inactive-text="RW" />
	              </div>
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-form-item label="工具 Profile">
          <el-select v-model="runnerToolScriptForm.profiles" multiple filterable style="width: 100%;">
            <el-option label="MySQL 8.0 物理备份（XtraBackup 8.0）" value="mysql_80_physical" />
            <el-option label="MySQL 8.4 物理备份（XtraBackup 8.4）" value="mysql_84_physical" />
            <el-option label="MySQL 5.7 遗留物理备份（XtraBackup 2.4）" value="mysql_57_physical" />
            <el-option label="MySQL/MariaDB binlog 归档" value="mysql_binlog_archiver" />
            <el-option label="MariaDB 物理备份" value="mariadb_physical" />
            <el-option label="PostgreSQL Barman" value="postgres_barman" />
            <el-option label="PostgreSQL pg_basebackup" value="postgres_native_pg_basebackup" />
            <el-option label="隔离恢复容器 Runner" value="restore_runner" />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="MySQL版本">
              <el-input v-model="runnerToolScriptForm.mysqlVersion" placeholder="如 8.0.44 / 8.4，可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="PostgreSQL版本">
              <el-input v-model="runnerToolScriptForm.postgresqlVersion" placeholder="如 16 / 17 / 18，可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="runnerToolScriptForm.installMode === 'offline'" :gutter="16">
          <el-col :span="24">
            <el-form-item label="离线包">
              <el-select v-model="runnerToolScriptForm.offlinePackageId" placeholder="选择已登记离线包" clearable filterable style="width: 100%;">
                <el-option
                  v-for="item in runnerToolOfflinePackages"
                  :key="item.id"
                  :label="`${item.name} / ${item.osFamily || '*'} ${item.osVersion || ''} / ${item.arch || '*'} / ${item.fileName}`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="安装原因">
          <el-input v-model="runnerToolScriptForm.reason" type="textarea" :rows="2" placeholder="必填，用于审计和二次确认" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="执行确认">
              <el-checkbox v-model="runnerToolScriptForm.confirmInstall">确认在该 Runner 上执行工具安装任务</el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="包范围确认">
              <el-checkbox v-model="runnerToolScriptForm.confirmPackages">已审阅安装包/Profile 范围</el-checkbox>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <div class="runner-tool-script-actions">
        <el-button type="primary" :loading="runnerToolScriptGenerating" @click="handleGenerateRunnerToolScript">生成脚本</el-button>
        <el-button type="danger" plain :loading="runnerToolInstalling" @click="handleInstallRunnerTools">执行安装</el-button>
        <el-button :disabled="!runnerToolScriptResult?.script" @click="copyText(runnerToolScriptResult?.script || '', '安装脚本')">复制脚本</el-button>
      </div>
      <template v-if="runnerToolScriptResult">
        <el-alert
          v-for="item in runnerToolScriptResult.warnings || []"
          :key="`warn-${item}`"
          :title="item"
          type="warning"
          show-icon
          :closable="false"
          class="backup-dialog-alert compact-alert"
        />
        <el-alert
          v-if="runnerToolScriptResult.unsupported?.length"
          :title="`未支持项：${runnerToolScriptResult.unsupported.join('、')}`"
          type="error"
          show-icon
          :closable="false"
          class="backup-dialog-alert compact-alert"
        />
        <el-input :model-value="runnerToolScriptResult.script" type="textarea" :rows="22" readonly class="runner-tool-script-textarea" />
      </template>
      <template #footer>
        <el-button @click="runnerToolScriptDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerToolOfflinePackageDialogVisible"
      title="Runner 工具离线包"
      width="1080px"
    >
      <el-alert
        title="离线包用于 P2.17 的 SSH Runner 离线安装：上传时校验 SHA256，执行时还会检查 OS、架构、包管理器和 Profile。"
        type="info"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form label-width="120px">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="名称">
              <el-input v-model="runnerToolOfflinePackageForm.name" placeholder="如 ubuntu22-pg-client" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="版本">
              <el-input v-model="runnerToolOfflinePackageForm.packageVersion" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="SHA256">
              <el-input v-model="runnerToolOfflinePackageForm.checksumSha256" placeholder="可选，填了会严格校验" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="6">
            <el-form-item label="OS">
              <el-input v-model="runnerToolOfflinePackageForm.osFamily" placeholder="ubuntu / rocky" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="OS版本">
              <el-input v-model="runnerToolOfflinePackageForm.osVersion" placeholder="如 22.04，可空" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="架构">
              <el-input v-model="runnerToolOfflinePackageForm.arch" placeholder="x86_64 / amd64" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="包管理器">
              <el-input v-model="runnerToolOfflinePackageForm.packageManager" placeholder="apt / yum / dnf" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="Profile">
          <el-select v-model="runnerToolOfflinePackageForm.profiles" multiple filterable style="width: 100%;">
            <el-option label="MySQL 8.0 物理备份（XtraBackup 8.0）" value="mysql_80_physical" />
            <el-option label="MySQL 8.4 物理备份（XtraBackup 8.4）" value="mysql_84_physical" />
            <el-option label="MySQL/MariaDB binlog 归档" value="mysql_binlog_archiver" />
            <el-option label="MariaDB 物理备份" value="mariadb_physical" />
            <el-option label="PostgreSQL Barman" value="postgres_barman" />
            <el-option label="PostgreSQL pg_basebackup" value="postgres_native_pg_basebackup" />
            <el-option label="隔离恢复容器 Runner" value="restore_runner" />
          </el-select>
        </el-form-item>
        <el-form-item label="离线包文件">
          <input type="file" @change="handleRunnerToolOfflineFileChange" />
          <span class="muted-text" style="margin-left: 12px;">{{ runnerToolOfflinePackageFile?.name || '未选择文件' }}</span>
        </el-form-item>
        <el-form-item label="Manifest">
          <el-input v-model="runnerToolOfflinePackageForm.manifestJson" type="textarea" :rows="3" placeholder="可选，保存离线包清单摘要" />
        </el-form-item>
      </el-form>
      <div class="runner-tool-script-actions">
        <el-button type="primary" :loading="runnerToolOfflinePackageUploading" @click="submitRunnerToolOfflinePackage">上传离线包</el-button>
        <el-button :loading="runnerToolOfflinePackageLoading" @click="loadRunnerToolOfflinePackages">刷新</el-button>
      </div>
      <el-table :data="runnerToolOfflinePackages" v-loading="runnerToolOfflinePackageLoading" stripe class="modern-table">
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column label="系统" min-width="170">
          <template #default="{ row }">{{ row.osFamily || '*' }} {{ row.osVersion || '' }} / {{ row.arch || '*' }} / {{ row.packageManager || '*' }}</template>
        </el-table-column>
        <el-table-column label="Profile" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">{{ (row.profiles || []).join(', ') || '-' }}</template>
        </el-table-column>
        <el-table-column prop="fileName" label="文件" min-width="180" show-overflow-tooltip />
        <el-table-column label="大小" width="110" align="right">
          <template #default="{ row }">{{ formatBytes(row.fileSize || 0) }}</template>
        </el-table-column>
        <el-table-column label="SHA256" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.checksumSha256 || '-' }}</template>
        </el-table-column>
        <el-table-column prop="uploadedAt" label="上传时间" width="170" />
        <el-table-column label="操作" width="100" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click="downloadRunnerToolOfflinePackage(row)">下载</el-button>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="runnerToolOfflinePackageDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerJobDetailVisible"
      title="Runner 任务详情"
      width="1040px"
    >
      <template v-if="runnerJobDetail">
        <el-descriptions :column="3" border class="backup-detail-descriptions">
          <el-descriptions-item label="任务">{{ runnerJobDetail.jobTypeText || runnerJobDetail.jobType }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ runnerJobDetail.statusText || runnerJobDetail.status }}</el-descriptions-item>
          <el-descriptions-item label="退出码">{{ runnerJobDetail.exitCode ?? '-' }}</el-descriptions-item>
          <el-descriptions-item label="Runner">{{ runnerJobDetail.runnerHostName || runnerJobDetail.runnerId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="日志">{{ runnerJobParsedResult.logPath || runnerJobDetail.logPath || '-' }}</el-descriptions-item>
          <el-descriptions-item label="Manifest">{{ runnerJobParsedResult.manifestPath || '-' }}</el-descriptions-item>
        </el-descriptions>
        <el-table v-if="runnerJobInstallSteps.length" :data="runnerJobInstallSteps" stripe class="modern-table">
          <el-table-column prop="name" label="阶段" width="180" />
          <el-table-column prop="status" label="状态" width="120" />
          <el-table-column prop="message" label="说明" min-width="420" show-overflow-tooltip />
        </el-table>
        <el-row :gutter="16" style="margin-top: 12px;">
          <el-col :span="12">
            <div class="runner-config-title"><span>stdout</span></div>
            <el-input :model-value="runnerJobParsedResult.stdout || ''" type="textarea" :rows="10" readonly />
          </el-col>
          <el-col :span="12">
            <div class="runner-config-title"><span>stderr</span></div>
            <el-input :model-value="runnerJobParsedResult.stderr || ''" type="textarea" :rows="10" readonly />
          </el-col>
        </el-row>
      </template>
      <template #footer>
        <el-button @click="runnerJobDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="barmanServerDialogVisible"
      :title="barmanServerForm.id ? '编辑 Barman Server' : '新增 Barman Server'"
      width="920px"
      @close="resetBarmanServerForm"
    >
      <el-alert
        title="Barman Server 通过 Runner 主机上的既有 Barman 配置执行受控命令：check、catalog 同步、WAL 同步、cluster 级 backup 和 restore 到隔离目录；隔离实例启动在 P3.7 继续做。"
        type="info"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="barmanServerFormRef" :model="barmanServerForm" :rules="barmanServerRules" label-width="135px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="PostgreSQL实例" prop="sourceInstanceId">
              <el-select v-model="barmanServerForm.sourceInstanceId" placeholder="请选择 PostgreSQL 实例" filterable style="width: 100%;">
                <el-option
                  v-for="item in postgresqlBackupInstances"
                  :key="item.id"
                  :label="`${item.name}（${item.endpoint || `${item.host}:${item.port}`}）`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Runner主机" prop="runnerHostId">
              <el-select v-model="barmanServerForm.runnerHostId" placeholder="请选择 Runner 主机" filterable style="width: 100%;">
                <el-option v-for="item in runnerHostOptions" :key="item.id" :label="item.label" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="barmanServerForm.name" placeholder="如：pg-prod-barman" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Barman server" prop="barmanServerName">
              <el-input v-model="barmanServerForm.barmanServerName" placeholder="barman.conf 里的 server name" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="配置文件">
              <el-input v-model="barmanServerForm.configPath" placeholder="/etc/barman.conf，可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Barman home">
              <el-input v-model="barmanServerForm.barmanHome" placeholder="/var/lib/barman，可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="保留策略">
              <el-input v-model="barmanServerForm.retentionPolicy" placeholder="REDUNDANCY 2 / RECOVERY WINDOW" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="备份方法">
              <el-input v-model="barmanServerForm.backupMethod" placeholder="postgres / rsync" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="复制 Slot">
              <el-input v-model="barmanServerForm.slotName" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="archive_command">
              <el-switch v-model="barmanServerForm.archiverEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="streaming">
              <el-switch v-model="barmanServerForm.streamingArchiverEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-select v-model="barmanServerForm.status" style="width: 100%;">
                <el-option label="待检测" value="pending" />
                <el-option label="健康" value="healthy" />
                <el-option label="降级" value="degraded" />
                <el-option label="失败" value="failed" />
                <el-option label="已禁用" value="disabled" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="配置 JSON">
          <el-input
            v-model="barmanServerForm.configJson"
            type="textarea"
            :rows="3"
            placeholder='可选，只保存非敏感摘要，例如 {"notes":"catalog only","environment":"prod"}'
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="barmanServerDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="barmanServerSubmitting" @click="submitBarmanServer">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="runnerAgentConfigDialogVisible"
      title="Agent 归档配置"
      width="920px"
    >
      <el-alert
        title="认证明文只显示在本次生成结果中；runnerAuthSha256 写入 Runner 主机配置 JSON，runnerAuth 写入 Agent 本地配置。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
	      <el-descriptions :column="2" border class="backup-detail-descriptions">
	        <el-descriptions-item label="Runner">{{ runnerAgentConfigHost?.name || '-' }}</el-descriptions-item>
	        <el-descriptions-item label="Runner ID">{{ runnerAgentConfigHost ? runnerAgentIDForHost(runnerAgentConfigHost) : '-' }}</el-descriptions-item>
	      </el-descriptions>
	      <el-form label-width="120px" class="runner-config-section">
	        <el-row :gutter="16">
	          <el-col :span="12">
	            <el-form-item label="OpsHub地址">
	              <el-input v-model="runnerAgentLifecycleForm.serverUrl" placeholder="Runner 可访问的 OpsHub 地址，如 http://192.168.1.12:8080" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="安装目录">
	              <el-input v-model="runnerAgentLifecycleForm.installPath" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="服务名">
	              <el-input v-model="runnerAgentLifecycleForm.serviceName" />
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-row :gutter="16">
	          <el-col :span="6">
	            <el-form-item label="监听地址">
	              <el-input v-model="runnerAgentLifecycleForm.listenAddr" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="上报间隔">
	              <el-input-number v-model="runnerAgentLifecycleForm.intervalSeconds" :min="5" :max="3600" controls-position="right" style="width: 100%;" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="归档器">
	              <el-switch v-model="runnerAgentLifecycleForm.databaseArchiverEnabled" active-text="启用" inactive-text="关闭" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="Dry Run">
	              <el-switch v-model="runnerAgentLifecycleForm.dryRun" active-text="是" inactive-text="否" />
	            </el-form-item>
	          </el-col>
	        </el-row>
	        <el-row :gutter="16">
	          <el-col :span="12">
	            <el-form-item label="操作原因">
	              <el-input v-model="runnerAgentLifecycleForm.reason" placeholder="安装/升级/重启时必填" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="重置认证">
	              <el-switch v-model="runnerAgentLifecycleForm.regenerateAuth" active-text="是" inactive-text="否" />
	            </el-form-item>
	          </el-col>
	          <el-col :span="6">
	            <el-form-item label="执行确认">
	              <el-checkbox v-model="runnerAgentLifecycleForm.confirm">确认执行</el-checkbox>
	            </el-form-item>
	          </el-col>
	        </el-row>
	      </el-form>
	      <div class="runner-tool-script-actions">
	        <el-button type="primary" :loading="runnerAgentLifecycleLoading" @click="handleGenerateRunnerAgentConfig">生成配置</el-button>
	        <el-button type="success" plain :loading="runnerAgentLifecycleLoading" @click="handleRunnerAgentLifecycle('install')">安装 Agent</el-button>
	        <el-button type="warning" plain :loading="runnerAgentLifecycleLoading" @click="handleRunnerAgentLifecycle('upgrade')">升级 Agent</el-button>
	        <el-button type="info" plain :loading="runnerAgentLifecycleLoading" @click="handleRunnerAgentLifecycle('restart')">重启 Agent</el-button>
	        <el-button :loading="runnerAgentLogsLoading" @click="handleFetchRunnerAgentLogs">查看日志</el-button>
	      </div>
	      <div class="runner-config-section">
	        <div class="runner-config-title">
	          <span>Runner 主机 configJson</span>
          <el-button size="small" @click="copyText(JSON.stringify({ runnerAuthSha256: runnerAgentAuthSha256 }, null, 2), 'runnerAuthSha256')">复制</el-button>
        </div>
        <el-input
          :model-value="JSON.stringify({ runnerAuthSha256: runnerAgentAuthSha256 }, null, 2)"
          type="textarea"
          :rows="3"
          readonly
        />
      </div>
      <div class="runner-config-section">
        <div class="runner-config-title">
          <span>Agent 本地配置片段</span>
          <el-button size="small" @click="copyText(runnerAgentConfigJson, 'Agent 配置')">复制</el-button>
	        </div>
	        <el-input v-model="runnerAgentConfigJson" type="textarea" :rows="16" readonly />
	      </div>
	      <div v-if="runnerAgentLogs" class="runner-config-section">
	        <div class="runner-config-title">
	          <span>最近 Agent 日志</span>
	          <el-button size="small" @click="copyText(`${runnerAgentLogs.stdout || ''}\n${runnerAgentLogs.stderr || ''}`, 'Agent 日志')">复制</el-button>
	        </div>
	        <el-input :model-value="`${runnerAgentLogs.stdout || ''}\n${runnerAgentLogs.stderr || ''}`" type="textarea" :rows="10" readonly />
	      </div>
      <template #footer>
        <el-button @click="runnerAgentConfigDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="restoreDialogVisible"
      title="发起恢复演练"
      width="640px"
      @close="resetRestoreForm"
    >
      <el-alert
        title="恢复演练会把成功备份导入到非生产目标实例。MySQL / MariaDB / PostgreSQL 可选择仅覆盖备份对象或清空目标库后导入；Redis 会覆盖同名 Key，但不会清空目标实例中的无关数据。"
        type="warning"
        show-icon
        :closable="false"
        class="backup-dialog-alert"
      />
      <el-form ref="restoreFormRef" :model="restoreForm" :rules="restoreRules" label-width="110px">
        <el-form-item label="备份文件">
          <el-input :model-value="restoreSourceRecord?.fileName || '-'" disabled />
        </el-form-item>
        <el-form-item label="来源实例">
          <el-input :model-value="restoreSourceRecord?.instanceName || '-'" disabled />
        </el-form-item>
        <el-form-item label="目标实例" prop="targetInstanceId">
          <el-select v-model="restoreForm.targetInstanceId" placeholder="请选择非生产目标实例" filterable style="width: 100%;">
            <el-option
              v-for="item in restoreTargetOptions"
              :key="item.id"
              :label="`${item.name}（${item.dbTypeText || item.dbType}${item.environment ? ` / ${item.environment}` : ''}）`"
              :value="item.id"
            />
          </el-select>
          <div class="field-tip">目标实例必须已启用，不能与来源实例相同，且环境不能是 prod / production / prd / 生产。</div>
        </el-form-item>
        <el-form-item label="演练模式">
          <el-input v-model="restoreForm.restoreMode" disabled />
        </el-form-item>
        <el-form-item label="目标处理" prop="restoreStrategy">
          <el-radio-group v-model="restoreForm.restoreStrategy">
            <el-radio-button
              v-for="item in availableRestoreStrategyOptions"
              :key="item.value"
              :label="item.value"
            >
              {{ item.label }}
            </el-radio-button>
          </el-radio-group>
          <div class="field-tip">{{ restoreStrategyTip }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="restoreDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="restoreSubmitting" @click="submitRestoreDryRun">
          开始演练
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="inspectionDetailVisible" title="巡检报告详情" width="1040px">
      <div v-if="currentInspectionReport" class="inspection-detail">
        <div class="table-overview-cards diagnosis-cards">
          <div class="table-overview-card">
            <span class="summary-label">健康分</span>
            <strong>{{ currentInspectionReport.healthScore }}</strong>
            <div class="diagnosis-card-desc">{{ currentInspectionReport.riskLevelText || currentInspectionReport.riskLevel }}</div>
          </div>
          <div class="table-overview-card">
            <span class="summary-label">实例</span>
            <strong>{{ currentInspectionReport.instanceName || `#${currentInspectionReport.instanceId}` }}</strong>
            <div class="diagnosis-card-desc">{{ currentInspectionReport.dbTypeText || currentInspectionReport.dbType }}</div>
          </div>
          <div class="table-overview-card">
            <span class="summary-label">关注项</span>
            <strong>{{ currentInspectionReport.findings?.length || 0 }}</strong>
            <div class="diagnosis-card-desc">{{ currentInspectionReport.generatedAt || '-' }}</div>
          </div>
        </div>
        <el-alert
          :title="currentInspectionReport.summary || '-'"
          :type="healthScoreAlertType(currentInspectionReport.healthScore)"
          show-icon
          :closable="false"
          class="diagnosis-alert"
        />

        <div class="inspection-section-grid">
          <div
            v-for="section in inspectionSections(currentInspectionReport)"
            :key="section.key"
            class="diagnosis-section"
          >
            <div class="panel-title">
              <span>{{ section.label }}</span>
              <el-tag size="small" :type="sectionStatusTag(section.status)">{{ section.summary || '-' }}</el-tag>
            </div>
            <el-table :data="section.metrics || []" stripe class="modern-table">
              <el-table-column label="指标" prop="label" min-width="130" />
              <el-table-column label="值" prop="value" min-width="130" />
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="sectionStatusTag(row.status)">{{ row.status || '-' }}</el-tag>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <div class="diagnosis-section">
          <div class="panel-title">
            <span>异常与关注项</span>
            <el-tag size="small" type="warning">{{ currentInspectionReport.findings?.length || 0 }}</el-tag>
          </div>
          <el-table :data="currentInspectionReport.findings || []" stripe class="modern-table">
            <el-table-column label="级别" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="findingSeverityTag(row.severity)">{{ row.severity || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="分类" prop="category" width="110" />
            <el-table-column label="标题" prop="title" min-width="160" show-overflow-tooltip />
            <el-table-column label="说明" prop="message" min-width="280" show-overflow-tooltip />
            <el-table-column label="资源" min-width="140">
              <template #default="{ row }">{{ row.resourceType || '-' }} {{ row.resourceId || '' }}</template>
            </el-table-column>
          </el-table>
        </div>
      </div>
      <template #footer>
        <el-button @click="inspectionDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="topologyDetailVisible" :title="topologyDetailTitle" size="520px">
      <template v-if="selectedTopologyNode">
        <el-descriptions :column="1" border class="topology-detail-descriptions">
          <el-descriptions-item label="节点">{{ selectedTopologyNode.name || selectedTopologyNode.id || '-' }}</el-descriptions-item>
          <el-descriptions-item label="实例 ID">{{ topologyDetailInstanceId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="地址"><span class="mono">{{ selectedTopologyNode.address || '-' }}</span></el-descriptions-item>
          <el-descriptions-item label="角色">
            <el-tag size="small" :type="topologyRoleTag(selectedTopologyNode.role)">{{ selectedTopologyNode.roleText || selectedTopologyNode.role || '-' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag size="small" :type="topologyStateTag(selectedTopologyNode.state)">{{ selectedTopologyNode.state || '-' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="延迟">{{ selectedTopologyNode.lagText || selectedTopologyNode.slots || '-' }}</el-descriptions-item>
          <el-descriptions-item label="最近采集">{{ selectedTopologyNode.metrics?.last_checked_at || selectedTopologyNode.updatedAt || '-' }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ selectedTopologyNode.message || '-' }}</el-descriptions-item>
        </el-descriptions>
      </template>
      <template v-else-if="selectedTopologyLink">
        <el-descriptions :column="1" border class="topology-detail-descriptions">
          <el-descriptions-item label="源节点">{{ selectedTopologyLink.sourceName || topologyNodeName(selectedTopologyLink.source) || selectedTopologyLink.source || '-' }}</el-descriptions-item>
          <el-descriptions-item label="目标节点">{{ selectedTopologyLink.targetName || topologyNodeName(selectedTopologyLink.target) || selectedTopologyLink.target || '-' }}</el-descriptions-item>
          <el-descriptions-item label="关系">{{ selectedTopologyLink.label || '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag size="small" :type="topologyStateTag(selectedTopologyLink.state)">{{ selectedTopologyLink.state || '-' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="延迟">{{ selectedTopologyLink.lagText || '-' }}</el-descriptions-item>
          <el-descriptions-item label="最近采集">{{ selectedTopologyLink.metrics?.last_checked_at || '-' }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ selectedTopologyLink.message || '-' }}</el-descriptions-item>
        </el-descriptions>
      </template>

      <div class="topology-detail-actions">
        <el-button
          type="primary"
          :disabled="!topologyDetailInstanceId"
          :loading="topologyCollectingId === topologyDetailInstanceId"
          @click="handleTopologyDetailCheck"
        >
          采集该实例
        </el-button>
        <el-button :disabled="!topologyDetailInstanceId" @click="handleTopologyDetailReplication">副本治理</el-button>
        <el-button :disabled="!topologyDetailInstanceId" @click="handleTopologyDetailRaw">原始采集</el-button>
      </div>

      <div class="topology-detail-metrics">
        <div class="panel-title">
          <span>关键指标</span>
          <el-tag size="small" type="info">{{ topologyDetailMetricEntries.length }}</el-tag>
        </div>
        <el-table :data="topologyDetailMetricEntries" stripe max-height="420" class="modern-table">
          <el-table-column label="指标" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ topologyMetricLabel(row.key) }}</template>
          </el-table-column>
          <el-table-column label="值" min-width="220" show-overflow-tooltip>
            <template #default="{ row }"><span class="mono">{{ row.value || '-' }}</span></template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>

    <el-dialog v-model="replicationRawDialogVisible" :title="replicationRawDialogTitle" width="860px">
      <el-input
        v-model="replicationRawDialogContent"
        type="textarea"
        :rows="22"
        readonly
        class="mono-textarea"
      />
      <template #footer>
        <el-button @click="replicationRawDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="replicaIncidentGuideDialogVisible"
      title="生成误操作事故指引"
      width="820px"
      @close="resetReplicaIncidentGuideForm"
    >
      <el-alert
        title="P4.3 只生成处置指引和审计，不会自动暂停 apply/replay，也不会执行任何数据库命令。"
        type="warning"
        show-icon
        :closable="false"
        class="mb-3"
      />
      <el-form ref="replicaIncidentGuideFormRef" :model="replicaIncidentGuideForm" :rules="replicaIncidentGuideRules" label-width="120px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="事故实例" prop="instanceId">
              <el-select v-model="replicaIncidentGuideForm.instanceId" placeholder="请选择实例" filterable class="w-full">
                <el-option
                  v-for="item in replicationInstances"
                  :key="item.id"
                  :label="`${item.name} (${item.dbType})`"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="事故时间" prop="incidentTime">
              <el-date-picker
                v-model="replicaIncidentGuideForm.incidentTime"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                placeholder="选择事故时间"
                class="w-full"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="事故类型" prop="incidentType">
              <el-select v-model="replicaIncidentGuideForm.incidentType" placeholder="请选择类型" class="w-full">
                <el-option label="误删" value="delete" />
                <el-option label="误更新" value="update" />
                <el-option label="错误发布" value="release" />
                <el-option label="其他" value="other" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="期望恢复" prop="expectedRecoveryMethod">
              <el-select v-model="replicaIncidentGuideForm.expectedRecoveryMethod" placeholder="请选择方式" class="w-full">
                <el-option label="导出回填" value="export_backfill" />
                <el-option label="整库回滚" value="full_rollback" />
                <el-option label="暂不确定" value="unknown" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="影响摘要" prop="affectedSummary">
          <el-input
            v-model="replicaIncidentGuideForm.affectedSummary"
            type="textarea"
            :rows="4"
            maxlength="4000"
            show-word-limit
            placeholder="填写影响库、表、SQL 摘要或业务对象"
          />
        </el-form-item>
        <el-form-item label="事故原因" prop="incidentReason">
          <el-input
            v-model="replicaIncidentGuideForm.incidentReason"
            type="textarea"
            :rows="4"
            maxlength="4000"
            show-word-limit
            placeholder="必须填写事故原因，便于审计和后续复盘"
          />
        </el-form-item>
        <el-form-item label="安全确认" prop="confirmNoAutoPause">
          <el-checkbox v-model="replicaIncidentGuideForm.confirmNoAutoPause">
            我确认本阶段只生成指引，不自动暂停 apply/replay，后续命令需人工确认目标是 replica/standby 后执行
          </el-checkbox>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="replicaIncidentGuideDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="replicaIncidentGuideSubmitting" @click="submitReplicaIncidentGuide">生成指引</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="replicaIncidentGuideDetailVisible" :title="replicaIncidentGuideDetailTitle" width="920px">
      <el-input
        v-model="replicaIncidentGuideDetailMarkdown"
        type="textarea"
        :rows="26"
        readonly
        class="mono-textarea"
      />
      <template #footer>
        <el-button @click="replicaIncidentGuideDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="replicaActionDialogVisible"
      :title="replicaActionMode === 'pause' ? '暂停副本 Apply' : '恢复副本 Apply'"
      width="760px"
      @close="resetReplicaActionForm"
    >
      <el-alert
        title="该操作会在目标副本上执行固定白名单命令。提交前系统会自动采集副本状态，只有确认是 replica/standby 时才会继续。"
        type="error"
        show-icon
        :closable="false"
        class="mb-3"
      />
      <el-descriptions :column="2" border class="backup-detail-descriptions">
        <el-descriptions-item label="主库">{{ replicaActionTarget?.primaryInstanceName || replicaActionTarget?.primaryEndpoint || '-' }}</el-descriptions-item>
        <el-descriptions-item label="副本">{{ replicaActionTarget?.replicaInstanceName || replicaActionTarget?.replicaEndpoint || '-' }}</el-descriptions-item>
        <el-descriptions-item label="角色">{{ replicaActionTarget?.replicaRoleText || replicaActionTarget?.replicaRole || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Apply状态">{{ replicaActionTarget?.applyStateText || replicaActionTarget?.applyState || '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-form ref="replicaActionFormRef" :model="replicaActionForm" :rules="replicaActionRules" label-width="120px" class="mt-3">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="事故编号" prop="incidentNo">
              <el-input v-model="replicaActionForm.incidentNo" maxlength="120" placeholder="例如 INC-20260503-001" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="关联指引ID">
              <el-input-number v-model="replicaActionForm.incidentGuideId" :min="0" controls-position="right" class="w-full" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="执行原因" prop="reason">
          <el-input
            v-model="replicaActionForm.reason"
            type="textarea"
            :rows="3"
            maxlength="1000"
            show-word-limit
            placeholder="必须填写暂停或恢复原因"
          />
        </el-form-item>
        <el-form-item label="影响确认" prop="confirmImpact">
          <el-input
            v-model="replicaActionForm.confirmImpact"
            type="textarea"
            :rows="3"
            maxlength="1000"
            show-word-limit
            placeholder="说明确认的目标、副本角色、影响范围和回滚/恢复安排"
          />
        </el-form-item>
        <el-form-item label="二次确认" prop="confirmed">
          <el-checkbox v-model="replicaActionForm.confirmed">
            我确认目标是副本/standby，理解暂停或恢复 apply/replay 对误删拦截窗口和复制状态的影响
          </el-checkbox>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="replicaActionDialogVisible = false">取消</el-button>
        <el-button :type="replicaActionMode === 'pause' ? 'danger' : 'success'" :loading="replicaActionSubmitting" @click="submitReplicaAction">
          {{ replicaActionMode === 'pause' ? '确认暂停' : '确认恢复' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="permissionDialogVisible"
      :title="permissionForm.id ? '编辑实例权限' : '添加实例权限'"
      width="720px"
      @close="resetPermissionForm"
    >
      <el-form ref="permissionFormRef" :model="permissionForm" :rules="permissionRules" label-width="100px">
        <el-form-item label="角色" prop="roleId">
          <el-select v-model="permissionForm.roleId" placeholder="请选择角色" filterable style="width: 100%;">
            <el-option v-for="role in roleOptions" :key="role.id" :label="`${role.name}（${role.code}）`" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="实例" prop="instanceId">
          <el-select v-model="permissionForm.instanceId" placeholder="请选择数据库实例" filterable style="width: 100%;">
            <el-option
              v-for="item in instanceOptions"
              :key="item.id"
              :label="`${item.name}（${item.dbTypeText || item.dbType} / ${item.endpoint || `${item.host}:${item.port}`}）`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="权限" prop="permissions">
          <el-checkbox-group v-model="permissionForm.permissions" class="permission-checkbox-grid">
            <el-checkbox v-for="item in databasePermissionOptions" :key="item.value" :label="item.value">
              {{ item.label }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="permissionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="permissionSubmitting" @click="submitPermissionForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Coin,
  DataLine,
  Delete,
  Download,
  Edit,
  Connection,
  Grid,
  Plus,
  Refresh,
  RefreshLeft,
  Search,
  Switch,
  Tickets
} from '@element-plus/icons-vue'
import { getCredentials } from '@/api/host'
import {
  DATABASE_PERMISSION,
  applyDatabaseMySQLPITRWizard,
  backupDatabaseBarmanServer,
  checkDatabaseBarmanServer,
  checkDatabaseReplication,
  checkDatabaseStorageProfilePosture,
  cleanupDatabaseRestoreJob,
  createDatabaseBarmanServer,
  createDatabaseBackupPolicy,
  createDatabaseLogArchiveStream,
  createDatabaseReplicaIncidentGuide,
  createDatabaseRestorePlan,
  createDatabaseRunnerHost,
  createDatabaseStorageProfile,
  createDatabaseBackupTask,
  createDatabaseInstance,
  collectDatabaseCapacitySnapshot,
  deleteDatabaseBarmanServer,
  deleteDatabaseBackupPolicy,
  deleteDatabaseBackupTask,
  deleteDatabaseInstance,
  deleteDatabaseInstancePermission,
  deleteDatabaseReplicaIncidentGuide,
  deleteDatabaseRunnerHost,
  disableDatabaseInstance,
  downloadDatabaseBackupRecord,
  downloadDatabaseRunnerToolOfflinePackage,
  executeDatabaseDDLQuery,
  explainDatabaseQuery,
  enableDatabaseInstance,
  executeDatabaseQuery,
  executeDatabaseWriteQuery,
  explainDatabaseWriteQuery,
	  generateDatabaseInspectionReport,
	  generateDatabaseRunnerAgentConfigSnippet,
	  generateDatabaseRunnerToolInstallScript,
	  getDatabaseRunnerAgentLogs,
  installDatabaseRunnerTools,
  installDatabaseRunnerAgent,
  applyDatabasePostgresBarmanPITRWizard,
  previewDatabaseBackupPolicyPurge,
  previewDatabaseBackupPolicySyntheticFull,
  previewDatabaseMySQLPITRWizard,
  previewDatabasePostgresBarmanPITRWizard,
  getDatabaseCapacityTrend,
  getDatabaseDiagnosisMetrics,
  getDatabaseInspectionReport,
  getDatabaseReplicationStatus,
  getDatabaseReplicaIncidentGuide,
  getDatabaseRestoreJobProof,
  getDatabaseRunnerToolProfile,
  getDatabaseTopology,
  getDatabaseUIPermissions,
  exportDatabaseQueryResult,
  exportDatabaseQueryAudits,
  exportDatabaseTableDictionary,
  formatDatabaseQuery,
  listDatabaseBackupRecords,
  listDatabaseBackupPolicies,
  listDatabaseProtectionProfiles,
  listDatabaseProtectionRisks,
  listDatabaseBackupTasks,
  listDatabaseBarmanServers,
  listDatabaseDiagnosisSessions,
  listDatabaseInspectionReports,
  listDatabaseLogArchiveEvents,
  listDatabaseLogArchives,
  listDatabaseLogArchiveStreams,
  listDatabaseRestoreJobs,
  listDatabaseRestorePlans,
  listDatabaseRunnerHosts,
  listDatabaseRunnerJobs,
  listDatabaseRunnerToolOfflinePackages,
  listDatabaseStorageProfiles,
  getDatabaseSupportedTypes,
  getDatabaseTableDDL,
  listDatabaseColumns,
  listDatabaseQueryHistory,
  listDatabaseInstances,
  listDatabaseInstancePermissions,
  listDatabaseIndexes,
  listDatabaseQueryAudits,
  listDatabaseSchemas,
  listDatabaseSlowQueries,
  listDatabaseTables,
  listDatabaseReplicas,
  listDatabaseReplicationChecks,
  listDatabaseReplicaProtections,
  listDatabaseReplicaIncidentGuides,
  listDatabaseReplicaActions,
  pauseDatabaseLogArchiveStream,
  pauseDatabaseReplicaApply,
  probeDatabaseRunnerTools,
  registerExternalDatabaseBackupRecord,
  registerExternalDatabaseLogArchive,
  resumeDatabaseLogArchiveStream,
	  resumeDatabaseReplicaApply,
	  restartDatabaseRunnerAgent,
  runDatabaseBackupPolicyPurge,
  runDatabaseBackupPolicySyntheticFull,
  runDatabaseBackupPolicyFull,
  runDatabaseBackupPolicyIncremental,
  runDatabaseBackupTask,
  runDatabaseLogArchiveCatchUp,
  runDatabaseLogArchiveOnce,
  runDatabaseProtectionRestoreDrill,
  runDatabaseRestoreDryRun,
  runDatabaseRestorePlan,
  startDatabaseLogArchiveStream,
  stopDatabaseLogArchiveStream,
  syncDatabaseBarmanCatalog,
  syncDatabaseBarmanWAL,
  syncDatabaseMetadata,
  testDatabaseInstance,
  testDatabaseRunnerHost,
  updateDatabaseBackupPolicy,
  updateDatabaseBackupTask,
  updateDatabaseBarmanServer,
  updateDatabaseInstance,
	  updateDatabaseRunnerHost,
	  upgradeDatabaseRunnerAgent,
  uploadDatabaseRunnerToolOfflinePackage,
  upsertDatabaseInstancePermission,
  validateDatabaseBackupPolicyChain,
  validateDatabaseProtectionProfile,
  validateDatabaseDDLQuery,
  verifyDatabaseBackupRecord,
  type DatabaseBackupPolicyChainValidationResult,
  type DatabaseBackupPolicyPurgePreviewResult,
  type DatabaseBackupPolicyPurgeRunResult,
  type DatabaseBackupRecordResult,
  type DatabaseBackupPolicyPayload,
  type DatabaseBackupPolicyResult,
  type DatabaseMySQLPITRWizardPayload,
  type DatabaseMySQLPITRWizardResult,
  type DatabasePostgresBarmanPITRWizardPayload,
  type DatabasePostgresBarmanPITRWizardResult,
  type DatabaseProtectionProfileResult,
  type DatabaseProtectionRiskResult,
  type DatabaseProtectionRestoreDrillPayload,
  type DatabaseProtectionRestoreDrillResult,
  type DatabaseBackupRunResult,
  type DatabaseBackupTaskPayload,
  type DatabaseBackupTaskResult,
  type DatabaseSyntheticFullPreviewResult,
  type DatabaseBarmanServerPayload,
  type DatabaseBarmanServerResult,
  type DatabaseCapacityCollectResult,
  type DatabaseCapacityTrendResult,
  type DatabaseDDLValidateResult,
  type DatabaseExternalBackupRecordPayload,
  type DatabaseExternalLogArchivePayload,
  type DatabaseLogArchiveEventResult,
  type DatabaseLogArchiveStreamControlPayload,
  type DatabaseLogArchiveResult,
  type DatabaseLogArchiveStreamPayload,
  type DatabaseLogArchiveStreamResult,
  type DatabaseInspectionSection,
  type DatabaseInspectionReportResult,
  type DatabaseInstancePayload,
  type DatabaseInstanceReplicaResult,
  type DatabaseReplicaActionPayload,
  type DatabaseReplicaActionResult,
  type DatabaseQueryPayload,
  type DatabaseReplicationCheckResult,
  type DatabaseReplicaProtectionResult,
  type DatabaseReplicaIncidentGuidePayload,
  type DatabaseReplicaIncidentGuideResult,
  type DatabaseReplicationStatusResult,
  type DatabaseRestoreDryRunPayload,
  type DatabaseRestoreJobResult,
  type DatabaseRestorePlanPayload,
  type DatabaseRestorePlanRunPayload,
  type DatabaseRestorePlanResult,
  type DatabaseRestoreValidationAssertionPayload,
  type DatabaseRunLogArchiveCatchUpPayload,
  type DatabaseRunLogArchiveOncePayload,
  type DatabaseRunnerHostPayload,
	  type DatabaseRunnerHostResult,
	  type DatabaseRunnerAgentConfigSnippetResult,
	  type DatabaseRunnerAgentLifecyclePayload,
	  type DatabaseRunnerAgentLogsResult,
	  type DatabaseRunnerJobResult,
  type DatabaseRunnerToolInstallPayload,
  type DatabaseRunnerToolInstallScriptPayload,
  type DatabaseRunnerToolInstallScriptResult,
  type DatabaseRunnerToolOfflinePackageResult,
  type DatabaseRunnerToolProfileResult,
  type DatabaseStorageProfilePayload,
  type DatabaseStorageProfilePostureCheckPayload,
  type DatabaseStorageProfileResult,
  type DatabaseSupportedType,
  type DatabaseTopologyFinding,
  type DatabaseTopologyLink,
  type DatabaseTopologyNode,
  type DatabaseTopologyResult,
  type DatabaseWriteExecuteResult,
  type DatabaseWriteValidateResult,
  validateDatabaseWriteQuery
} from '@/api/database'
import {
  getDatabaseConfig as getSystemDatabaseConfig,
  saveDatabaseConfig as saveSystemDatabaseConfig,
  type DatabaseConfig as SystemDatabaseConfig
} from '@/api/system'
import { getAllRoles } from '@/api/role'

const activeTab = ref('instances')
const loading = ref(false)
const submitting = ref(false)
const testingId = ref(0)
const syncingId = ref(0)
const dialogVisible = ref(false)
const instances = ref<any[]>([])
const instanceOptions = ref<any[]>([])
const supportedTypes = ref<DatabaseSupportedType[]>([])
const credentials = ref<any[]>([])
const roleOptions = ref<any[]>([])
const uiPermissions = ref<Record<string, boolean>>({})
const total = ref(0)

type DatabaseCapabilityKey = 'metadataEnabled' | 'queryEnabled' | 'testEnabled' | 'topologyEnabled'

const supportedTypeMap = computed(() => {
  const result = new Map<string, DatabaseSupportedType>()
  supportedTypes.value.forEach(item => result.set(item.type, item))
  return result
})

const hasDatabaseCapability = (dbType: string | undefined, capability: DatabaseCapabilityKey) => {
  if (!dbType) return false
  return !!supportedTypeMap.value.get(dbType)?.[capability]
}

const hasInstanceCapability = (item: any, capability: DatabaseCapabilityKey) =>
  hasDatabaseCapability(item?.dbType, capability)

const hasDatabasePermission = (item: any, permission: number) =>
  (Number(item?.permissions || 0) & permission) > 0

const canUseDatabaseFeature = (item: any, permission: number, capability?: DatabaseCapabilityKey) =>
  !!item && hasDatabasePermission(item, permission) && (!capability || hasInstanceCapability(item, capability))

const canManageInstancePermissions = computed(() => uiPermissions.value.instancePermissionManage === true)
const permissionModeAlertTitle = computed(() => {
  if (permissionMode.value === 'whitelist') {
    return permissionRulesEnabled.value
      ? '实例对象权限处于白名单模式：非 admin 用户只能访问已授权实例。'
      : '实例对象权限处于白名单模式：当前没有授权规则，非 admin 用户默认无法访问数据库实例。'
  }
  if (permissionModeEnforced.value) {
    return '实例对象权限处于兼容模式，但已存在授权规则，非 admin 用户会按实例权限收敛。'
  }
  return '实例对象权限处于兼容模式：当前没有授权规则，非 admin 用户不会被实例级权限收敛。'
})

const hasPermissionMask = (mask: number | undefined, permission: number) =>
  (Number(mask || 0) & permission) > 0

const databasePermissionOptions = [
  { label: '查看', value: DATABASE_PERMISSION.VIEW },
  { label: '查询', value: DATABASE_PERMISSION.QUERY },
  { label: '导出', value: DATABASE_PERMISSION.EXPORT },
  { label: '写入', value: DATABASE_PERMISSION.WRITE },
  { label: '备份', value: DATABASE_PERMISSION.BACKUP },
  { label: '恢复', value: DATABASE_PERMISSION.RESTORE },
  { label: '诊断', value: DATABASE_PERMISSION.DIAGNOSIS },
  { label: '拓扑', value: DATABASE_PERMISSION.TOPOLOGY },
  { label: '管理', value: DATABASE_PERMISSION.MANAGE },
  { label: '不限行数', value: DATABASE_PERMISSION.QUERY_UNLIMITED },
  { label: '写SQL计划', value: DATABASE_PERMISSION.WRITE_EXPLAIN },
  { label: 'DDL变更', value: DATABASE_PERMISSION.DDL }
]

const formRef = ref<FormInstance>()
const metadataInstanceId = ref<number>()
const schemasLoading = ref(false)
const tablesLoading = ref(false)
const detailsLoading = ref(false)
const ddlLoading = ref(false)
const schemas = ref<any[]>([])
const tables = ref<any[]>([])
const columns = ref<any[]>([])
const indexes = ref<any[]>([])
const selectedSchema = ref('')
const selectedTable = ref<any>()
const detailTab = ref('columns')
const ddlDialogVisible = ref(false)
const currentDDL = ref<any>()
const dictionaryExporting = ref(false)
const tableContextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  row: undefined as any
})
const queryInstanceId = ref<number>()
const querySchemaName = ref('')
const querySchemas = ref<any[]>([])
const querySQL = ref('SELECT 1')
type QueryConsoleMode = 'read' | 'write' | 'ddl'
const queryConsoleMode = ref<QueryConsoleMode>('read')
type QueryResultTab = 'result' | 'sql' | 'audit' | 'rollback' | 'check'
const queryResultTab = ref<QueryResultTab>('result')
interface QuerySQLFavorite {
  id: string
  title: string
  sqlText: string
  schemaName: string
  dbType: string
  createdAt: string
}
const queryFavorites = ref<QuerySQLFavorite[]>([])
const queryFavoriteStorageKey = 'opshub.database.query.favorites'
const queryLimit = ref(500)
const queryUnlimitedRows = ref(false)
const queryTimeoutSeconds = ref(30)
const queryExportLimit = ref(1000)
const queryRunning = ref(false)
const queryFormatting = ref(false)
const queryExplaining = ref(false)
const queryExporting = ref(false)
const queryWriteChecking = ref(false)
const queryWritePreparing = ref(false)
const queryWriteSubmitting = ref(false)
const queryDDLChecking = ref(false)
const queryDDLPreparing = ref(false)
const queryDDLSubmitting = ref(false)
const queryResult = ref<any>()
const writeCheckResult = ref<DatabaseWriteValidateResult>()
const ddlCheckResult = ref<DatabaseDDLValidateResult>()
const writeResult = ref<DatabaseWriteExecuteResult>()
const ddlSQLTypes = new Set(['CREATE', 'ALTER', 'DROP', 'TRUNCATE', 'RENAME'])
const writeConfirmVisible = ref(false)
const ddlConfirmVisible = ref(false)
const writeConfirmForm = reactive({
  reason: '',
  confirmed: false
})
const ddlConfirmForm = reactive({
  reason: '',
  confirmed: false
})
const explainDialogVisible = ref(false)
const explainResult = ref<any>()
const queryHistoryDialogVisible = ref(false)
const queryHistoryLoading = ref(false)
const queryHistoryItems = ref<any[]>([])
const databaseWriteConfigLoading = ref(false)
const databaseWriteConfigSaving = ref(false)
const databaseConfig = reactive<SystemDatabaseConfig>({
  writeEnabled: false,
  writeExplainEnabled: false,
  ddlEnabled: false,
  ddlHighRiskRequiresConfirm: true,
  ddlReasonRequired: true,
  ddlRequireBackupHint: true,
  highRiskRequiresConfirm: true,
  operationReasonRequired: true,
  maxAffectedRows: 1000,
  defaultBackupRetentionDays: 7,
  backupStoragePath: './data/database-backups',
  instancePermissionMode: 'compat'
})
const diagnosisInstanceId = ref<number>()
const diagnosisLoading = ref(false)
const diagnosisMetrics = ref<any>()
const diagnosisSessions = ref<any[]>([])
const diagnosisSlowQueries = ref<any[]>([])
const diagnosisSlowMessage = ref('')
const capacityLoading = ref(false)
const capacityCollecting = ref(false)
const capacityRange = ref('7d')
const capacityTrend = ref<DatabaseCapacityTrendResult>()
const capacityChartRef = ref<HTMLElement>()
const topologyChartRef = ref<HTMLElement>()
const topologyInstanceId = ref<number>()
const topologyLoading = ref(false)
const topologyResult = ref<DatabaseTopologyResult>()
const topologyCollectingId = ref(0)
const topologyCollectingAll = ref(false)
const topologyDetailVisible = ref(false)
const selectedTopologyNode = ref<DatabaseTopologyNode>()
const selectedTopologyLink = ref<DatabaseTopologyLink>()
const replicationLoading = ref(false)
const replicationProtectionLoading = ref(false)
const replicationCheckLoading = ref(false)
const replicationCheckingId = ref(0)
const replicationProtections = ref<DatabaseReplicaProtectionResult[]>([])
const replicationProtectionTotal = ref(0)
const replicationReplicas = ref<DatabaseInstanceReplicaResult[]>([])
const replicationReplicaTotal = ref(0)
const replicationChecks = ref<DatabaseReplicationCheckResult[]>([])
const replicationCheckTotal = ref(0)
const replicationStatusDetail = ref<DatabaseReplicationStatusResult>()
const replicationRawDialogVisible = ref(false)
const replicationRawDialogTitle = ref('')
const replicationRawDialogContent = ref('')
const replicaIncidentGuideLoading = ref(false)
const replicaIncidentGuideSubmitting = ref(false)
const replicaIncidentGuideDeletingId = ref(0)
const replicaIncidentGuides = ref<DatabaseReplicaIncidentGuideResult[]>([])
const replicaIncidentGuideTotal = ref(0)
const replicaIncidentGuideDialogVisible = ref(false)
const replicaIncidentGuideDetailVisible = ref(false)
const replicaIncidentGuideDetailId = ref(0)
const replicaIncidentGuideDetailTitle = ref('')
const replicaIncidentGuideDetailMarkdown = ref('')
const replicaIncidentGuideFormRef = ref<FormInstance>()
const replicaActionLoading = ref(false)
const replicaActionSubmitting = ref(false)
const replicaActions = ref<DatabaseReplicaActionResult[]>([])
const replicaActionTotal = ref(0)
const replicaActionDialogVisible = ref(false)
const replicaActionMode = ref<'pause' | 'resume'>('pause')
const replicaActionTarget = ref<DatabaseInstanceReplicaResult>()
const replicaActionFormRef = ref<FormInstance>()
const backupTaskLoading = ref(false)
const backupTaskSubmitting = ref(false)
const backupTaskDialogVisible = ref(false)
const runningBackupTaskId = ref(0)
const backupTasks = ref<DatabaseBackupTaskResult[]>([])
const backupTaskTotal = ref(0)
const backupTaskFormRef = ref<FormInstance>()
const protectionProfileLoading = ref(false)
const validatingProtectionProfileId = ref('')
const protectionWizardDialogVisible = ref(false)
const protectionWizardSubmitting = ref(false)
const protectionWizardPreviewing = ref(false)
const protectionWizardPreview = ref<DatabaseMySQLPITRWizardResult>()
const protectionWizardFormRef = ref<FormInstance>()
const postgresBarmanWizardDialogVisible = ref(false)
const postgresBarmanWizardSubmitting = ref(false)
const postgresBarmanWizardPreviewing = ref(false)
const postgresBarmanWizardPreview = ref<DatabasePostgresBarmanPITRWizardResult>()
const postgresBarmanWizardFormRef = ref<FormInstance>()
const protectionRestoreDrillDialogVisible = ref(false)
const protectionRestoreDrillSubmitting = ref(false)
const protectionRestoreDrillFormRef = ref<FormInstance>()
const protectionRestoreDrillProfile = ref<DatabaseProtectionProfileResult>()
const protectionRestoreDrillResult = ref<DatabaseProtectionRestoreDrillResult>()
const protectionProfiles = ref<DatabaseProtectionProfileResult[]>([])
const protectionProfileTotal = ref(0)
const protectionRiskLoading = ref(false)
const protectionRisks = ref<DatabaseProtectionRiskResult[]>([])
const protectionRiskTotal = ref(0)
const backupPolicyLoading = ref(false)
const backupPolicySubmitting = ref(false)
const backupPolicyDialogVisible = ref(false)
const runningBackupPolicyId = ref(0)
const runningBackupPolicyLevel = ref('')
const validatingBackupPolicyId = ref(0)
const previewingSyntheticPolicyId = ref(0)
const runningSyntheticPolicyId = ref(0)
const previewingPurgePolicyId = ref(0)
const runningPurgePolicyId = ref(0)
const syntheticPreviewVisible = ref(false)
const syntheticPreviewPolicy = ref<DatabaseBackupPolicyResult>()
const syntheticPreview = ref<DatabaseSyntheticFullPreviewResult>()
const purgePreviewVisible = ref(false)
const purgePreviewPolicy = ref<DatabaseBackupPolicyResult>()
const purgePreview = ref<DatabaseBackupPolicyPurgePreviewResult>()
const backupPolicies = ref<DatabaseBackupPolicyResult[]>([])
const backupPolicyTotal = ref(0)
const backupPolicyFormRef = ref<FormInstance>()
const permissionFormRef = ref<FormInstance>()
const permissionLoading = ref(false)
const permissionSubmitting = ref(false)
const permissionDialogVisible = ref(false)
const permissionRows = ref<any[]>([])
const permissionTotal = ref(0)
const permissionMode = ref<'compat' | 'whitelist'>('compat')
const permissionModeEnforced = ref(false)
const permissionRulesEnabled = ref(false)
const backupRecordLoading = ref(false)
const verifyingBackupRecordId = ref(0)
const backupRecords = ref<DatabaseBackupRecordResult[]>([])
const backupRecordTotal = ref(0)
const externalBackupDialogVisible = ref(false)
const externalBackupSubmitting = ref(false)
const externalBackupFormRef = ref<FormInstance>()
const storageProfileLoading = ref(false)
const storageProfileSubmitting = ref(false)
const storageProfileDialogVisible = ref(false)
const storageProfileFormRef = ref<FormInstance>()
const storageProfiles = ref<DatabaseStorageProfileResult[]>([])
const storageProfileTotal = ref(0)
const storagePostureDialogVisible = ref(false)
const storagePostureSubmitting = ref(false)
const storagePostureFormRef = ref<FormInstance>()
const currentStoragePostureProfile = ref<DatabaseStorageProfileResult>()
const storagePostureDetailVisible = ref(false)
const currentStoragePostureDetail = ref<DatabaseStorageProfileResult>()
const logArchiveStreamLoading = ref(false)
const logArchiveStreams = ref<DatabaseLogArchiveStreamResult[]>([])
const logArchiveStreamTotal = ref(0)
const logArchiveStreamDialogVisible = ref(false)
const logArchiveStreamSubmitting = ref(false)
const logArchiveStreamFormRef = ref<FormInstance>()
const startLogArchiveStreamDialogVisible = ref(false)
const startLogArchiveStreamSubmitting = ref(false)
const startLogArchiveStreamFormRef = ref<FormInstance>()
const startLogArchiveStreamStream = ref<DatabaseLogArchiveStreamResult>()
const logArchiveLoading = ref(false)
const logArchives = ref<DatabaseLogArchiveResult[]>([])
const logArchiveTotal = ref(0)
const logArchiveDialogVisible = ref(false)
const logArchiveSubmitting = ref(false)
const logArchiveFormRef = ref<FormInstance>()
const logArchiveEventLoading = ref(false)
const logArchiveEvents = ref<DatabaseLogArchiveEventResult[]>([])
const logArchiveEventTotal = ref(0)
const runLogArchiveOnceDialogVisible = ref(false)
const runLogArchiveOnceSubmitting = ref(false)
const runLogArchiveOnceFormRef = ref<FormInstance>()
const runLogArchiveOnceStream = ref<DatabaseLogArchiveStreamResult>()
const runLogArchiveCatchUpDialogVisible = ref(false)
const runLogArchiveCatchUpSubmitting = ref(false)
const runLogArchiveCatchUpFormRef = ref<FormInstance>()
const runLogArchiveCatchUpStream = ref<DatabaseLogArchiveStreamResult>()
const restorePlanLoading = ref(false)
const restorePlans = ref<DatabaseRestorePlanResult[]>([])
const restorePlanTotal = ref(0)
const barmanCatalogLoading = ref(false)
const barmanCatalogRecords = ref<DatabaseBackupRecordResult[]>([])
const barmanCatalogTotal = ref(0)
const walStatusLoading = ref(false)
const walStatusArchives = ref<DatabaseLogArchiveResult[]>([])
const restorePlanDialogVisible = ref(false)
const restorePlanSubmitting = ref(false)
const restorePlanFormRef = ref<FormInstance>()
const restorePlanBaseRecordLabel = ref('')
const restorePlanRunDialogVisible = ref(false)
const restorePlanRunSubmitting = ref(false)
const restorePlanRunFormRef = ref<FormInstance>()
const restorePlanRunSource = ref<DatabaseRestorePlanResult>()
const restoreProofDialogVisible = ref(false)
const restoreProofTitle = ref('')
const restoreProofContent = ref('')
const restoreJobDetailVisible = ref(false)
const restoreJobDetailTitle = ref('')
const restoreJobDetailRow = ref<DatabaseRestoreJobResult | null>(null)
interface RestoreJobStepRow {
  name: string
  label: string
  status: string
  statusText: string
  startedAt: string
  finishedAt: string
  durationMs: number | null
}
interface RestoreJobValidationRow {
  index: number
  sql?: string
  status?: string
  outputPreview?: string
  expectedRows?: number
  expectedContains?: string
  expectedScalar?: string
  actualRows?: number | null
  actualScalar?: string
  assertionStatus?: string
  assertionMessage?: string
}
const restoreJobSteps = ref<RestoreJobStepRow[]>([])
const restoreJobValidations = ref<RestoreJobValidationRow[]>([])
const runnerHostLoading = ref(false)
const runnerHostSubmitting = ref(false)
const runnerHostDialogVisible = ref(false)
const runnerHostTestingId = ref(0)
const runnerToolProbingId = ref(0)
const runnerToolProfileLoading = ref(false)
const runnerToolProfileDialogVisible = ref(false)
const runnerToolProfile = ref<DatabaseRunnerToolProfileResult>()
const runnerToolScriptDialogVisible = ref(false)
const runnerToolScriptGenerating = ref(false)
const runnerToolInstalling = ref(false)
const runnerToolScriptHost = ref<DatabaseRunnerHostResult>()
const runnerToolScriptResult = ref<DatabaseRunnerToolInstallScriptResult>()
const runnerToolOfflinePackageDialogVisible = ref(false)
const runnerToolOfflinePackageLoading = ref(false)
const runnerToolOfflinePackageUploading = ref(false)
const runnerToolOfflinePackages = ref<DatabaseRunnerToolOfflinePackageResult[]>([])
const runnerToolOfflinePackageFile = ref<File | null>(null)
const runnerJobDetailVisible = ref(false)
const runnerJobDetail = ref<DatabaseRunnerJobResult | null>(null)
const runnerHostFormRef = ref<FormInstance>()
const runnerHosts = ref<DatabaseRunnerHostResult[]>([])
const runnerHostTotal = ref(0)
const runnerAgentConfigDialogVisible = ref(false)
const runnerAgentConfigHost = ref<DatabaseRunnerHostResult>()
const runnerAgentAuthSha256 = ref('')
const runnerAgentConfigJson = ref('')
const runnerAgentLifecycleLoading = ref(false)
const runnerAgentLogsLoading = ref(false)
const runnerAgentLogs = ref<DatabaseRunnerAgentLogsResult>()
const runnerAgentLifecycleForm = reactive<DatabaseRunnerAgentLifecyclePayload>({
  serverUrl: '',
  installPath: '/opt/opshub-agent',
  serviceName: '',
  listenAddr: '0.0.0.0:19100',
  intervalSeconds: 60,
  databaseArchiverEnabled: true,
  dryRun: false,
  regenerateAuth: false,
  confirm: false,
  reason: ''
})
const runnerJobLoading = ref(false)
const runnerJobs = ref<DatabaseRunnerJobResult[]>([])
const runnerJobTotal = ref(0)
const barmanServerLoading = ref(false)
const barmanServerSubmitting = ref(false)
const barmanServerDialogVisible = ref(false)
const barmanServerFormRef = ref<FormInstance>()
const barmanServers = ref<DatabaseBarmanServerResult[]>([])
const barmanServerTotal = ref(0)
const barmanCheckingId = ref(0)
const barmanCatalogSyncingId = ref(0)
const barmanWalSyncingId = ref(0)
const barmanBackingUpId = ref(0)
const backupPitrTab = ref('protectionOverview')
const backupAdvancedTab = ref('backupPolicies')
const restoreJobLoading = ref(false)
const restoreSubmitting = ref(false)
const restoreDialogVisible = ref(false)
const restoreJobs = ref<DatabaseRestoreJobResult[]>([])
const restoreJobTotal = ref(0)
const restoreSourceRecord = ref<DatabaseBackupRecordResult>()
const restoreFormRef = ref<FormInstance>()
const inspectionLoading = ref(false)
const inspectionGenerating = ref(false)
const inspectionDetailVisible = ref(false)
const inspectionReports = ref<DatabaseInspectionReportResult[]>([])
const inspectionTotal = ref(0)
const currentInspectionReport = ref<DatabaseInspectionReportResult>()
const auditLoading = ref(false)
const auditExporting = ref(false)
const queryAudits = ref<any[]>([])
const auditTotal = ref(0)
const auditDetailVisible = ref(false)
const currentAudit = ref<any>()
const sqlTypes = ['SELECT', 'SHOW', 'DESC', 'DESCRIBE', 'EXPLAIN', 'WITH', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'ALTER', 'DROP', 'TRUNCATE', 'REPLACE', 'RENAME', 'GRANT', 'REVOKE', 'MERGE', 'EXPORT', 'DIAGNOSIS', 'BACKUP', 'RESTORE', 'CAPACITY', 'INSPECTION', 'PERMISSION', 'UNKNOWN']
const auditActions = [
  { label: '只读查询', value: 'query' },
  { label: '执行计划', value: 'explain' },
  { label: '写 SQL 执行计划', value: 'write_explain' },
  { label: '查询结果导出', value: 'query_export' },
  { label: '写操作执行', value: 'change_execute' },
  { label: 'DDL 结构变更', value: 'ddl_execute' },
  { label: '逻辑备份执行', value: 'backup_run' },
  { label: '备份文件下载', value: 'backup_download' },
  { label: '备份文件校验', value: 'backup_verify' },
  { label: '拓扑查看', value: 'topology_view' },
  { label: '恢复演练', value: 'restore_dry_run' },
  { label: '容量趋势查看', value: 'capacity_view' },
  { label: '巡检报告生成', value: 'inspection_generate' },
  { label: '副本状态查看', value: 'replica_status_view' },
  { label: '副本状态采集', value: 'replica_check_run' },
  { label: '副本事故指引', value: 'replica_incident_guide' },
  { label: '暂停副本 Apply', value: 'replica_pause_apply' },
  { label: '恢复副本 Apply', value: 'replica_resume_apply' },
  { label: '实例权限保存', value: 'instance_permission_upsert' },
  { label: '实例权限删除', value: 'instance_permission_delete' },
  { label: '数据字典导出', value: 'metadata_export' },
  { label: '诊断指标查看', value: 'diagnosis_metrics' },
  { label: '活跃会话查看', value: 'diagnosis_sessions' },
  { label: '慢 SQL 查看', value: 'diagnosis_slow_queries' }
]

const isDDLAudit = (item: any) =>
  item?.action === 'ddl_execute' || item?.auditAction === 'ddl_execute'

const isDDLWriteResult = computed(() =>
  writeResult.value?.auditAction === 'ddl_execute' || ddlSQLTypes.has(String(writeResult.value?.sqlType || '').toUpperCase())
)

const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  dbType: '',
  status: '',
  environment: ''
})

const auditQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  instanceId: undefined as number | undefined,
  action: '',
  status: '',
  riskLevel: '',
  sqlType: ''
})

const queryHistoryQuery = reactive({
  keyword: '',
  limit: 20,
  currentInstanceOnly: true
})

const diagnosisQuery = reactive({
  sessionLimit: 20,
  slowLimit: 10
})

const backupTaskQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  instanceId: undefined as number | undefined,
  enabled: ''
})

const protectionProfileQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  instanceId: undefined as number | undefined,
  engine: '',
  protectionLevel: '',
  riskLevel: ''
})

const protectionRiskQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  instanceId: undefined as number | undefined,
  engine: '',
  riskLevel: '',
  issueType: '',
  productionOnly: ''
})

const protectionWizardForm = reactive<DatabaseMySQLPITRWizardPayload>({
  instanceId: 0,
  sourceInstanceId: undefined,
  sourceRole: 'primary',
  runnerHostId: 0,
  storageProfileId: undefined,
  secretProfileId: undefined,
  templateKey: 'rolling_synthetic_full',
  policyName: '',
  backupEngine: '',
  toolExecutionMode: 'host_tools',
  toolImage: '',
  toolImageDigest: '',
  containerDatadirPath: '',
  containerWorkdirPath: '',
  containerNetworkMode: 'host',
  containerDatadirRo: true,
  fullSchedule: '',
  incrementalSchedule: '0 3 * * *',
  runInitialFullNow: true,
  binlogArchiveMode: 'polling',
  binlogRpoTargetSeconds: 300,
  binlogRetentionDays: 45,
  syntheticEnabled: true,
  syntheticAutoRun: true,
  syntheticTriggerAfterIncrementals: 5,
  syntheticMergeOldestIncrementals: 5,
  syntheticRequireRestoreProof: true,
  syntheticNeverDeleteWithoutProof: true,
  syntheticMarkSupersededAfterProof: true,
  syntheticSupersededKeepDays: 7,
  restoreDrillRequired: true,
  retentionFullKeepMonths: 6,
  retentionIncrementalKeepDays: 45,
  retentionBinlogKeepDays: 45,
  retentionNeverDeleteWithoutProof: true,
  archiveConfigJson: ''
})

const postgresBarmanWizardForm = reactive<DatabasePostgresBarmanPITRWizardPayload>({
  instanceId: 0,
  runnerHostId: 0,
  reuseBarmanServerId: undefined,
  name: '',
  barmanServerName: '',
  barmanHome: '',
  configPath: '',
  retentionPolicy: 'RECOVERY WINDOW OF 30 DAYS',
  backupMethod: 'postgres',
  streamingArchiverEnabled: true,
  archiverEnabled: true,
  slotName: '',
  configJson: '',
  runCheckNow: true,
  syncCatalogNow: true,
  syncWalNow: true,
  runInitialBackupNow: false
})

const protectionRestoreDrillForm = reactive<DatabaseProtectionRestoreDrillPayload & { validationSqlText?: string; confirmIsolated?: boolean }>({
  restoreTargetType: 'time',
  restoreTargetValue: '',
  targetTimelineId: '',
  restoreTargetInclusive: true,
  runnerHostId: undefined,
  containerImage: '',
  listenPort: undefined,
  expiresInHours: 24,
  validationSql: [],
  validationAssertions: [],
  validationSqlText: '',
  cleanupOnFailure: false,
  postgresStartInstance: true,
  targetAction: 'pause',
  barmanGetWal: true,
  confirmIsolated: false
})

const backupPolicyQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  instanceId: undefined as number | undefined,
  status: '',
  enabled: ''
})

const permissionQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  roleId: undefined as number | undefined,
  instanceId: undefined as number | undefined
})

const backupRecordQuery = reactive({
  page: 1,
  pageSize: 10,
  taskId: undefined as number | undefined,
  instanceId: undefined as number | undefined,
  status: '',
  triggerType: ''
})

const storageProfileQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  storageType: '',
  status: ''
})

const logArchiveStreamQuery = reactive({
  page: 1,
  pageSize: 10,
  instanceId: undefined as number | undefined,
  archiveType: '',
  status: ''
})

const logArchiveQuery = reactive({
  page: 1,
  pageSize: 10,
  streamId: undefined as number | undefined,
  instanceId: undefined as number | undefined,
  archiveType: '',
  status: ''
})

const logArchiveEventQuery = reactive({
  page: 1,
  pageSize: 20,
  streamId: undefined as number | undefined,
  instanceId: undefined as number | undefined,
  runnerHostId: undefined as number | undefined,
  level: '',
  eventType: ''
})

const restorePlanQuery = reactive({
  page: 1,
  pageSize: 10,
  sourceInstanceId: undefined as number | undefined,
  targetInstanceId: undefined as number | undefined,
  validationStatus: '',
  restoreStatus: ''
})

const barmanCatalogQuery = reactive({
  page: 1,
  pageSize: 10,
  instanceId: undefined as number | undefined,
  backupMethod: 'physical',
  backupEngine: 'barman',
  backupScope: 'cluster',
  status: ''
})

const walStatusQuery = reactive({
  page: 1,
  pageSize: 100,
  streamId: undefined as number | undefined,
  instanceId: undefined as number | undefined
})

const replicationReplicaQuery = reactive({
  page: 1,
  pageSize: 10,
  instanceId: undefined as number | undefined,
  engine: '',
  replicaRole: '',
  status: ''
})

const replicationProtectionQuery = reactive({
  page: 1,
  pageSize: 6,
  instanceId: undefined as number | undefined,
  engine: '',
  protectionStatus: '',
  riskLevel: ''
})

const replicationCheckQuery = reactive({
  page: 1,
  pageSize: 10,
  instanceId: undefined as number | undefined,
  engine: '',
  roleDetected: '',
  healthStatus: ''
})

const replicaIncidentGuideQuery = reactive({
  page: 1,
  pageSize: 5,
  instanceId: undefined as number | undefined,
  incidentType: '',
  status: '',
  canIntercept: ''
})

const replicaActionQuery = reactive({
  page: 1,
  pageSize: 5,
  instanceId: undefined as number | undefined,
  replicaId: undefined as number | undefined,
  action: '',
  status: ''
})

const replicaIncidentGuideForm = reactive<DatabaseReplicaIncidentGuidePayload>({
  instanceId: 0,
  incidentTime: '',
  incidentType: 'delete',
  affectedSummary: '',
  incidentReason: '',
  expectedRecoveryMethod: 'export_backfill',
  confirmNoAutoPause: false
})

const replicaActionForm = reactive<DatabaseReplicaActionPayload>({
  incidentGuideId: undefined,
  incidentNo: '',
  reason: '',
  confirmImpact: '',
  confirmed: false,
  maxCheckAgeSeconds: 60
})

const replicaIncidentGuideRules: FormRules = {
  instanceId: [{ required: true, message: '请选择事故实例', trigger: 'change' }],
  incidentType: [{ required: true, message: '请选择事故类型', trigger: 'change' }],
  incidentTime: [{ required: true, message: '请选择事故时间', trigger: 'change' }],
  affectedSummary: [{ required: true, message: '请填写影响范围或 SQL 摘要', trigger: 'blur' }],
  incidentReason: [{ required: true, message: '请填写事故原因', trigger: 'blur' }],
  confirmNoAutoPause: [
    {
      validator: (_rule, value, callback) => {
        if (value === true) callback()
        else callback(new Error('必须确认本阶段不自动暂停 apply/replay'))
      },
      trigger: 'change'
    }
  ]
}

const replicaActionRules: FormRules = {
  incidentNo: [{ required: true, message: '请填写事故编号', trigger: 'blur' }],
  reason: [{ required: true, message: '请填写执行原因', trigger: 'blur' }],
  confirmImpact: [{ required: true, message: '请填写影响范围确认', trigger: 'blur' }],
  confirmed: [
    {
      validator: (_rule, value, callback) => {
        if (value === true) callback()
        else callback(new Error('必须完成二次确认'))
      },
      trigger: 'change'
    }
  ]
}

const runnerHostQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  runnerType: '',
  status: '',
  enabled: ''
})

const runnerJobQuery = reactive({
  page: 1,
  pageSize: 10,
  runnerHostId: undefined as number | undefined,
  jobType: '',
  status: ''
})

const barmanServerQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  sourceInstanceId: undefined as number | undefined,
  runnerHostId: undefined as number | undefined,
  status: ''
})

const restoreJobQuery = reactive({
  page: 1,
  pageSize: 10,
  sourceInstanceId: undefined as number | undefined,
  targetInstanceId: undefined as number | undefined,
  status: ''
})

const inspectionQuery = reactive({
  page: 1,
  pageSize: 10,
  instanceId: undefined as number | undefined,
  status: '',
  riskLevel: ''
})

const form = reactive<DatabaseInstancePayload & { id?: number }>({
  id: undefined,
  name: '',
  dbType: 'mysql',
  host: '',
  port: 3306,
  defaultDatabase: '',
  credentialId: 0,
  tlsEnabled: false,
  connectionParams: '',
  status: 'enabled',
  environment: '',
  businessSystem: '',
  owner: '',
  tags: '',
  remark: ''
})

const rules: FormRules = {
  name: [{ required: true, message: '请输入实例名称', trigger: 'blur' }],
  dbType: [{ required: true, message: '请选择数据库类型', trigger: 'change' }],
  host: [{ required: true, message: '请输入主机地址', trigger: 'blur' }],
  port: [{ required: true, message: '请输入端口', trigger: 'change' }],
  credentialId: [{ required: true, message: '请选择连接凭据', trigger: 'change' }]
}

const backupTaskForm = reactive<DatabaseBackupTaskPayload & { id?: number }>({
  id: undefined,
  instanceId: 0,
  name: '',
  backupType: 'logical',
  backupMethod: 'logical',
  backupLevel: 'full',
  backupEngine: 'logical',
  backupScope: 'database',
  scopeConfig: '',
  schedule: '',
  storageType: 'local',
  storageConfig: '',
  retentionDays: 7,
  maxDurationMinutes: 1440,
  compression: 'gzip',
  encryptionEnabled: false,
  enabled: true
})
const backupTaskFormInstanceDbType = ref('')

const defaultBackupPolicySyntheticRuleJson = '{\n  "mode": "rolling_synthetic_full",\n  "autoRun": true,\n  "triggerAfterIncrementals": 5,\n  "mergeOldestIncrementals": 5,\n  "requireRestoreProof": true,\n  "neverDeleteWithoutProof": true,\n  "markSupersededAfterProof": true,\n  "supersededKeepDaysAfterProof": 7\n}'

const backupPolicyForm = reactive<DatabaseBackupPolicyPayload & { id?: number }>({
  id: undefined,
  instanceId: 0,
  sourceInstanceId: undefined,
  sourceRole: 'primary',
  name: '',
  backupEngine: 'xtrabackup_8_0',
  toolExecutionMode: 'host_tools',
  toolImage: '',
  toolImageDigest: '',
  containerDatadirPath: '',
  containerWorkdirPath: '',
  containerNetworkMode: 'host',
  containerDatadirRo: true,
  runnerHostId: 0,
  storageProfileId: undefined,
  secretProfileId: undefined,
  binlogStreamId: undefined,
  fullSchedule: '0 2 1 * *',
  incrementalSchedule: '0 3 * * *',
  syntheticEnabled: false,
  syntheticRuleJson: defaultBackupPolicySyntheticRuleJson,
  restoreDrillRequired: true,
  retentionJson: '{\n  "fullKeepMonths": 6,\n  "incrementalKeepDays": 45,\n  "binlogKeepDays": 45,\n  "neverDeleteWithoutProof": true\n}',
  enabled: true
})

const externalBackupForm = reactive<DatabaseExternalBackupRecordPayload>({
  instanceId: 0,
  sourceInstanceId: undefined,
  sourceRole: 'external',
  chainId: '',
  baseRecordId: undefined,
  parentRecordId: undefined,
  backupMethod: 'physical',
  backupLevel: 'full',
  backupEngine: 'external',
  externalBackupId: '',
  externalServerName: '',
  toolName: '',
  toolVersion: '',
  storageProfileId: undefined,
  storageUri: '',
  manifestJson: '',
  prepareStatus: '',
  fileName: '',
  fileSize: undefined,
  checksumSha256: '',
  compression: '',
  encrypted: false,
  recoverableFrom: '',
  recoverableUntil: '',
  startedAt: '',
  finishedAt: '',
  status: 'success',
  verifyStatus: 'success',
  serverUuid: '',
  backupBinlogFile: '',
  backupBinlogPos: undefined,
  backupGtidSet: '',
  pgSystemIdentifier: '',
  timelineId: '',
  startLsn: '',
  endLsn: '',
  walStart: '',
  walEnd: ''
})

const storageProfileForm = reactive<DatabaseStorageProfilePayload>({
  name: '',
  storageType: 'minio',
  endpoint: '',
  bucket: '',
  region: 'us-east-1',
  pathPrefix: '',
  secretProfileId: undefined,
  versioningEnabled: true,
  immutabilityEnabled: false,
  kmsKeyId: '',
  retentionLockDays: 0,
  status: 'enabled'
})

const storagePostureForm = reactive<DatabaseStorageProfilePostureCheckPayload>({
  accessKey: '',
  secretKey: '',
  sessionToken: '',
  useSsl: false,
  usePathStyle: true,
  insecureSkipVerify: false
})

const logArchiveStreamForm = reactive<DatabaseLogArchiveStreamPayload>({
  instanceId: 0,
  sourceInstanceId: undefined,
  engine: '',
  archiveType: '',
  archiveMode: 'external',
  archiveEngine: 'external',
  runnerHostId: undefined,
  storageProfileId: undefined,
  secretProfileId: undefined,
  rpoTargetSeconds: 300,
  retentionDays: 30,
  enabled: true,
  configJson: ''
})

const startLogArchiveStreamForm = reactive<DatabaseLogArchiveStreamControlPayload>({
  runnerHostId: undefined,
  archiveMode: 'polling',
  reason: ''
})

const logArchiveForm = reactive<DatabaseExternalLogArchivePayload>({
  streamId: 0,
  fileName: '',
  storageUri: '',
  fileSize: undefined,
  checksumSha256: '',
  firstEventTime: '',
  lastEventTime: '',
  status: 'archived',
  serverUuid: '',
  startPos: undefined,
  endPos: undefined,
  startGtidSet: '',
  endGtidSet: '',
  previousFileName: '',
  nextFileName: '',
  pgSystemIdentifier: '',
  timelineId: '',
  startLsn: '',
  endLsn: '',
  segmentNo: '',
  timelineHistoryUri: ''
})

const runLogArchiveOnceForm = reactive<DatabaseRunLogArchiveOncePayload>({
  runnerHostId: 0,
  fileName: ''
})

const runLogArchiveCatchUpForm = reactive<DatabaseRunLogArchiveCatchUpPayload>({
  runnerHostId: 0,
  maxFiles: 5,
  includeCurrent: false
})

const restorePlanForm = reactive<DatabaseRestorePlanPayload>({
  sourceInstanceId: 0,
  targetInstanceId: undefined,
  baseRecordId: undefined,
  restoreMode: 'isolated_restore',
  restoreTargetType: 'time',
  restoreTargetValue: '',
  targetTimelineId: '',
  restoreTargetInclusive: true
})

const restorePlanRunForm = reactive<DatabaseRestorePlanRunPayload & { validationSqlText?: string; confirmIsolated?: boolean }>({
  runnerHostId: 0,
  containerImage: '',
  listenPort: undefined,
  expiresInHours: 24,
  validationSql: [],
  validationAssertions: [],
  validationSqlText: '',
  cleanupOnFailure: false,
  postgresStartInstance: true,
  targetTimelineId: '',
  targetAction: 'pause',
  barmanGetWal: true,
  confirmIsolated: false
})

const runnerHostForm = reactive<DatabaseRunnerHostPayload & { id?: number }>({
  id: undefined,
  name: '',
  runnerType: 'ssh',
  host: '',
  port: 22,
  credentialId: undefined,
  workDir: '/var/lib/opshub/database-runner',
  storageMountPath: '',
  maxConcurrentJobs: 1,
  cpuLimit: '',
  ioLimit: '',
  bandwidthLimit: '',
  timeoutMinutes: 30,
  enabled: true,
  configJson: ''
})

const runnerToolScriptForm = reactive({
  profiles: ['mysql_80_physical', 'mysql_binlog_archiver', 'postgres_barman', 'postgres_native_pg_basebackup', 'restore_runner'] as string[],
  installMode: 'online',
  executionMode: 'host_tools',
  toolImage: '',
  toolImageDigest: '',
  datadirMount: '',
  workdirMount: '',
  networkMode: 'host',
  readOnlyDatadir: true,
  dryRun: true,
  mysqlVersion: '',
  postgresqlVersion: '',
  reason: '',
  confirmInstall: false,
  confirmPackages: false,
  offlinePackageId: undefined as number | undefined
})

const runnerToolOfflinePackageForm = reactive({
  name: '',
  packageVersion: '',
  osFamily: '',
  osVersion: '',
  arch: '',
  packageManager: '',
  profiles: ['restore_runner'] as string[],
  checksumSha256: '',
  manifestJson: ''
})

const barmanServerForm = reactive<DatabaseBarmanServerPayload & { id?: number }>({
  id: undefined,
  sourceInstanceId: 0,
  runnerHostId: 0,
  name: '',
  barmanServerName: '',
  barmanHome: '',
  configPath: '',
  retentionPolicy: '',
  backupMethod: '',
  streamingArchiverEnabled: false,
  archiverEnabled: false,
  slotName: '',
  status: 'pending',
  configJson: ''
})

const permissionForm = reactive({
  id: undefined as number | undefined,
  roleId: undefined as number | undefined,
  instanceId: undefined as number | undefined,
  permissions: [DATABASE_PERMISSION.VIEW] as number[]
})

const backupTaskRules: FormRules = {
  instanceId: [{ required: true, message: '请选择数据库实例', trigger: 'change' }],
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  retentionDays: [{ required: true, message: '请输入保留天数', trigger: 'change' }]
}

const backupPolicyRules: FormRules = {
  instanceId: [{ required: true, message: '请选择数据库实例', trigger: 'change' }],
  name: [{ required: true, message: '请输入策略名称', trigger: 'blur' }],
  backupEngine: [{ required: true, message: '请选择物理备份工具', trigger: 'change' }],
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }]
}

const protectionWizardRules: FormRules = {
  instanceId: [{ required: true, message: '请选择 MySQL/MariaDB 实例', trigger: 'change' }],
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  backupEngine: [{ required: true, message: '请选择物理备份工具', trigger: 'change' }],
  incrementalSchedule: [{ required: true, message: '请输入增量 Cron', trigger: 'blur' }]
}

const postgresBarmanWizardRules: FormRules = {
  instanceId: [{ required: true, message: '请选择 PostgreSQL 实例', trigger: 'change' }],
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  barmanServerName: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      const text = String(value || '').trim()
      if (!text) {
        callback(new Error('请输入 Barman server name'))
        return
      }
      if (!/^[A-Za-z0-9_.:-]+$/.test(text)) {
        callback(new Error('只能包含字母、数字、下划线、点、冒号和短横线'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }]
}

const protectionRestoreDrillRules: FormRules = {
  restoreTargetType: [{ required: true, message: '请选择恢复目标类型', trigger: 'change' }],
  restoreTargetValue: [{ required: true, message: '请选择恢复目标', trigger: 'change' }],
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  expiresInHours: [{ required: true, message: '请输入保留小时数', trigger: 'change' }],
  confirmIsolated: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (!value) {
        callback(new Error('请确认本次只恢复到隔离环境'))
        return
      }
      callback()
    },
    trigger: 'change'
  }]
}

const externalBackupRules: FormRules = {
  instanceId: [{ required: true, message: '请选择数据库实例', trigger: 'change' }],
  backupMethod: [{ required: true, message: '请选择备份方法', trigger: 'change' }],
  backupLevel: [{ required: true, message: '请选择备份级别', trigger: 'change' }],
  backupEngine: [{ required: true, message: '请输入备份引擎', trigger: 'blur' }],
  storageUri: [{ required: true, message: '请输入存储 URI', trigger: 'blur' }],
  fileName: [{ required: true, message: '请输入备份文件名', trigger: 'blur' }],
  recoverableFrom: [{ required: true, message: '请选择可恢复起点', trigger: 'change' }],
  recoverableUntil: [{ required: true, message: '请选择可恢复终点', trigger: 'change' }]
}

const storageProfileRules: FormRules = {
  name: [{ required: true, message: '请输入存储配置名称', trigger: 'blur' }],
  storageType: [{ required: true, message: '请选择存储类型', trigger: 'change' }]
}

function isObjectStorageProfile(profile?: { storageType?: string }) {
  return ['s3', 'minio'].includes(String(profile?.storageType || '').toLowerCase())
}

const storagePostureRules: FormRules = {
  accessKey: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (isObjectStorageProfile(currentStoragePostureProfile.value) && !String(value || '').trim()) {
        callback(new Error('请输入临时 Access Key'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  secretKey: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (isObjectStorageProfile(currentStoragePostureProfile.value) && !String(value || '').trim()) {
        callback(new Error('请输入临时 Secret Key'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }]
}

const logArchiveStreamRules: FormRules = {
  instanceId: [{ required: true, message: '请选择数据库实例', trigger: 'change' }],
  archiveType: [{ required: true, message: '请选择归档类型', trigger: 'change' }],
  archiveMode: [{ required: true, message: '请输入归档模式', trigger: 'blur' }],
  archiveEngine: [{ required: true, message: '请输入归档引擎', trigger: 'blur' }]
}

const startLogArchiveStreamRules: FormRules = {
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  archiveMode: [{ required: true, message: '请选择归档模式', trigger: 'change' }]
}

const logArchiveRules: FormRules = {
  streamId: [{ required: true, message: '请选择归档流', trigger: 'change' }],
  fileName: [{ required: true, message: '请输入日志文件名', trigger: 'blur' }],
  storageUri: [{ required: true, message: '请输入存储 URI', trigger: 'blur' }],
  firstEventTime: [{ required: true, message: '请选择起始时间', trigger: 'change' }],
  lastEventTime: [{ required: true, message: '请选择结束时间', trigger: 'change' }]
}

const runLogArchiveOnceRules: FormRules = {
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }]
}

const runLogArchiveCatchUpRules: FormRules = {
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  maxFiles: [{ required: true, message: '请输入最大文件数', trigger: 'change' }]
}

const restorePlanRules: FormRules = {
  sourceInstanceId: [{ required: true, message: '请选择来源实例', trigger: 'change' }],
  restoreTargetType: [{ required: true, message: '请选择目标类型', trigger: 'change' }],
  restoreTargetValue: [{ required: true, message: '请选择恢复目标时间', trigger: 'change' }]
}

const restorePlanRunRules: FormRules = {
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  expiresInHours: [{ required: true, message: '请输入保留小时数', trigger: 'change' }],
  confirmIsolated: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (!value) {
        callback(new Error('请确认本次只恢复到隔离库'))
        return
      }
      callback()
    },
    trigger: 'change'
  }]
}

const runnerHostRules: FormRules = {
  name: [{ required: true, message: '请输入 Runner 名称', trigger: 'blur' }],
  runnerType: [{ required: true, message: '请选择 Runner 类型', trigger: 'change' }],
  host: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (runnerHostForm.runnerType === 'ssh' && !String(value || '').trim()) {
        callback(new Error('请输入 SSH 主机地址'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  credentialId: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      if (runnerHostForm.runnerType === 'ssh' && !value) {
        callback(new Error('请选择 SSH 凭据'))
        return
      }
      callback()
    },
    trigger: 'change'
  }]
}

const barmanServerRules: FormRules = {
  sourceInstanceId: [{ required: true, message: '请选择 PostgreSQL 实例', trigger: 'change' }],
  runnerHostId: [{ required: true, message: '请选择 Runner 主机', trigger: 'change' }],
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  barmanServerName: [{
    validator: (_rule: any, value: any, callback: (error?: Error) => void) => {
      const text = String(value || '').trim()
      if (!text) {
        callback(new Error('请输入 Barman server name'))
        return
      }
      if (!/^[A-Za-z0-9_.:-]+$/.test(text)) {
        callback(new Error('只能包含字母、数字、下划线、点、冒号和短横线'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }]
}

const permissionRules: FormRules = {
  roleId: [{ required: true, message: '请选择角色', trigger: 'change' }],
  instanceId: [{ required: true, message: '请选择数据库实例', trigger: 'change' }],
  permissions: [{ type: 'array', required: true, min: 1, message: '请选择权限', trigger: 'change' }]
}

const restoreForm = reactive<DatabaseRestoreDryRunPayload>({
  targetInstanceId: 0,
  restoreMode: 'dry_run',
  restoreStrategy: 'object_replace'
})

const inspectionForm = reactive({
  instanceId: undefined as number | undefined
})

const restoreRules: FormRules = {
  targetInstanceId: [{ required: true, message: '请选择目标实例', trigger: 'change' }],
  restoreStrategy: [{ required: true, message: '请选择目标处理策略', trigger: 'change' }]
}

const currentMetadataInstance = computed(() =>
  instanceOptions.value.find(item => item.id === metadataInstanceId.value)
)

const metadataInstances = computed(() =>
  instanceOptions.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.VIEW, 'metadataEnabled'))
)

const currentQueryInstance = computed(() =>
  instanceOptions.value.find(item => item.id === queryInstanceId.value)
)

const queryInstances = computed(() =>
  instanceOptions.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.QUERY, 'queryEnabled'))
)

const isRedisMetadataInstance = computed(() =>
  currentMetadataInstance.value?.dbType === 'redis'
)

const isRedisQueryInstance = computed(() =>
  currentQueryInstance.value?.dbType === 'redis'
)

const canWriteCurrentQueryInstance = computed(() =>
  ['mysql', 'mariadb', 'postgresql'].includes(currentQueryInstance.value?.dbType || '') &&
  hasDatabasePermission(currentQueryInstance.value, DATABASE_PERMISSION.WRITE)
)

const canDDLCurrentQueryInstance = computed(() =>
  ['mysql', 'mariadb', 'postgresql'].includes(currentQueryInstance.value?.dbType || '') &&
  uiPermissions.value.queryDDL === true &&
  hasDatabasePermission(currentQueryInstance.value, DATABASE_PERMISSION.DDL)
)

const canUseQueryUnlimitedRows = computed(() =>
  !isRedisQueryInstance.value &&
  canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled') &&
  hasDatabasePermission(currentQueryInstance.value, DATABASE_PERMISSION.QUERY_UNLIMITED)
)

const canUseWriteExplainCurrentQueryInstance = computed(() =>
  !isRedisQueryInstance.value &&
  uiPermissions.value.queryWriteExplain === true &&
  canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled') &&
  hasDatabasePermission(currentQueryInstance.value, DATABASE_PERMISSION.WRITE_EXPLAIN)
)

const canPreviewContextTable = computed(() =>
  !!tableContextMenu.row &&
  !isRedisMetadataInstance.value &&
  canUseDatabaseFeature(currentMetadataInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled')
)

const canUseContextTableUnlimited = computed(() =>
  canPreviewContextTable.value &&
  hasDatabasePermission(currentMetadataInstance.value, DATABASE_PERMISSION.QUERY_UNLIMITED)
)

const queryConsoleAlert = computed(() =>
  isRedisQueryInstance.value
    ? '命令控制台支持 Redis 白名单只读命令。当前支持 GET / MGET / HGET / HGETALL / LRANGE / SMEMBERS / ZRANGE / XRANGE / SCAN / INFO / DBSIZE / MEMORY USAGE / CLUSTER INFO|NODES|SLOTS，所有动作都会写入统一审计。'
    : 'SQL 控制台支持只读查询、执行计划、受控 DML 和独立 DDL 结构变更。只读链路仅允许 SELECT / SHOW / DESC / DESCRIBE / EXPLAIN / WITH；DML 仅支持 INSERT / UPDATE / DELETE；DDL 由独立开关和权限控制，所有动作都会写入统一审计。'
)

const querySchemaPlaceholder = computed(() =>
  isRedisQueryInstance.value ? '逻辑 DB' : '默认库 / Schema'
)

const queryEditorPlaceholder = computed(() =>
  isRedisQueryInstance.value
    ? '请输入 Redis 只读命令，例如：SCAN 0 MATCH user:* COUNT 50 或 GET app:config'
    : '请输入 SQL。只读查询可直接执行；写操作请先做预检查，再走受控确认执行。'
)

const queryConsoleModeLabel = computed(() => {
  if (isRedisQueryInstance.value) return 'Redis 只读'
  if (queryConsoleMode.value === 'write') return '受控写入'
  if (queryConsoleMode.value === 'ddl') return 'DDL 变更'
  return '只读查询'
})

const queryConsoleModeTagType = computed(() => {
  if (isRedisQueryInstance.value) return 'primary'
  if (queryConsoleMode.value === 'write') return 'warning'
  if (queryConsoleMode.value === 'ddl') return 'danger'
  return 'primary'
})

const isProductionQueryInstance = computed(() =>
  currentQueryInstance.value?.environment === 'prod'
)

const currentDiagnosisInstance = computed(() =>
  instanceOptions.value.find(item => item.id === diagnosisInstanceId.value)
)

const diagnosisInstances = computed(() =>
  instanceOptions.value.filter(item => hasDatabasePermission(item, DATABASE_PERMISSION.DIAGNOSIS))
)

const isRedisDiagnosisInstance = computed(() =>
  currentDiagnosisInstance.value?.dbType === 'redis'
)

const currentTopologyInstance = computed(() =>
  instanceOptions.value.find(item => item.id === topologyInstanceId.value)
)

const topologyDetailTitle = computed(() =>
  selectedTopologyNode.value
    ? `拓扑节点 - ${selectedTopologyNode.value.name || selectedTopologyNode.value.id || '-'}`
    : `复制关系 - ${selectedTopologyLink.value?.sourceName || topologyNodeName(selectedTopologyLink.value?.source) || '-'} -> ${selectedTopologyLink.value?.targetName || topologyNodeName(selectedTopologyLink.value?.target) || '-'}`
)

const topologyDetailInstanceId = computed(() => {
  if (selectedTopologyNode.value) {
    return topologyNodeInstanceId(selectedTopologyNode.value)
  }
  if (selectedTopologyLink.value) {
    return topologyLinkInstanceId(selectedTopologyLink.value)
  }
  return 0
})

const topologyDetailMetricEntries = computed(() => {
  const metrics = selectedTopologyNode.value?.metrics || selectedTopologyLink.value?.metrics || {}
  return Object.entries(metrics)
    .filter(([, value]) => String(value || '').trim() !== '')
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({ key, value }))
})

const capacityTrendPoints = computed(() =>
  [...(capacityTrend.value?.points || [])].sort((left, right) => left.collectedAt.localeCompare(right.collectedAt))
)

let capacityChart: echarts.ECharts | null = null
let topologyChart: echarts.ECharts | null = null
let diagnosisRequestSeq = 0
let capacityTrendRequestSeq = 0

const topologyInstances = computed(() =>
  instanceOptions.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.TOPOLOGY, 'topologyEnabled'))
)

const replicationInstances = computed(() =>
  instanceOptions.value.filter(item => ['mysql', 'mariadb', 'postgresql'].includes(item.dbType) && hasDatabasePermission(item, DATABASE_PERMISSION.TOPOLOGY))
)

const supportedBackupInstances = computed(() =>
  instanceOptions.value.filter(item => ['mysql', 'mariadb', 'postgresql', 'redis'].includes(item.dbType) && hasDatabasePermission(item, DATABASE_PERMISSION.BACKUP))
)

const pitrBackupInstances = computed(() =>
  instanceOptions.value.filter(item => ['mysql', 'mariadb', 'postgresql'].includes(item.dbType) && hasDatabasePermission(item, DATABASE_PERMISSION.BACKUP))
)

const mysqlPhysicalBackupPolicyInstances = computed(() =>
  instanceOptions.value.filter(item => ['mysql', 'mariadb'].includes(item.dbType) && hasDatabasePermission(item, DATABASE_PERMISSION.BACKUP))
)

const selectedProtectionWizardInstance = computed(() =>
  instanceOptions.value.find(item => item.id === protectionWizardForm.instanceId)
)

const protectionWizardBackupEngineOptions = computed(() => {
  if (selectedProtectionWizardInstance.value?.dbType === 'mariadb') {
    return [{ label: 'mariadb-backup', value: 'mariadb_backup' }]
  }
  return [
    { label: 'XtraBackup 8.0（MySQL 8.0.x）', value: 'xtrabackup_8_0' },
    { label: 'XtraBackup 8.4（MySQL 8.4.x）', value: 'xtrabackup_8_4' },
    { label: 'XtraBackup 2.4 legacy（MySQL 5.7）', value: 'xtrabackup_2_4' }
  ]
})

const defaultProtectionWizardBackupEngine = () => {
  const instance = selectedProtectionWizardInstance.value
  if (instance?.dbType === 'mariadb') return 'mariadb_backup'
  const version = String(instance?.version || '')
  const match = version.match(/(\d+)\.(\d+)/)
  const major = match ? Number(match[1]) : 0
  const minor = match ? Number(match[2]) : 0
  if (major === 5) return 'xtrabackup_2_4'
  if (major === 8 && minor >= 4) return 'xtrabackup_8_4'
  return 'xtrabackup_8_0'
}

const postgresqlBackupInstances = computed(() =>
  instanceOptions.value.filter(item => item.dbType === 'postgresql' && hasDatabasePermission(item, DATABASE_PERMISSION.BACKUP))
)

const restorePlanSourceInstance = computed(() =>
  instanceOptions.value.find(item => item.id === restorePlanForm.sourceInstanceId)
)

const restorePlanSourceDbType = computed(() =>
  restorePlanSourceInstance.value?.dbType || ''
)

const restoreRunSourceInstance = computed(() =>
  instanceOptions.value.find(item => item.id === restorePlanRunSource.value?.sourceInstanceId)
)

const restoreRunSourceDbType = computed(() =>
  restoreRunSourceInstance.value?.dbType || ''
)

const isPostgreSQLRestoreRun = computed(() =>
  restoreRunSourceDbType.value === 'postgresql'
)

const restoreRunPlanPayload = computed(() => {
  if (!restorePlanRunSource.value?.planJson) return {} as Record<string, any>
  try {
    return JSON.parse(restorePlanRunSource.value.planJson) as Record<string, any>
  } catch {
    return {} as Record<string, any>
  }
})

const restoreRunBackupEngine = computed(() =>
  String(restoreRunPlanPayload.value?.backupEngine || restoreRunPlanPayload.value?.baseBackup?.backupEngine || '').trim()
)

const isBarmanRestoreRun = computed(() =>
  isPostgreSQLRestoreRun.value && restoreRunBackupEngine.value === 'barman'
)

const isPgBaseBackupRestoreRun = computed(() =>
  isPostgreSQLRestoreRun.value && restoreRunBackupEngine.value === 'pg_basebackup'
)

const normalizeToolEngineValue = (value?: string) =>
  String(value || '').trim().toLowerCase().replace(/[-.]/g, '_')

const isPgBackRestEngineValue = (value?: string) =>
  ['pgbackrest', 'pg_backrest', 'pg_back_rest'].includes(normalizeToolEngineValue(value))

const externalBackupEngineOptions = [
  { label: '外部引擎', value: 'external' },
  { label: 'XtraBackup', value: 'xtrabackup' },
  { label: 'mariadb-backup', value: 'mariadb_backup' },
  { label: 'Barman', value: 'barman' },
  { label: 'WAL-G external', value: 'walg' },
  { label: 'pg_basebackup', value: 'pg_basebackup' },
  { label: 'pgBackRest legacy external', value: 'pgbackrest' }
]

const logArchiveEngineOptions = computed(() => {
  if (logArchiveStreamForm.archiveType === 'wal') {
    return [
      { label: '外部 WAL', value: 'external_wal' },
      { label: 'Barman WAL', value: 'barman' },
      { label: 'WAL-G external WAL', value: 'walg' },
      { label: 'pg_receivewal', value: 'pg_receivewal' },
      { label: 'pgBackRest legacy WAL', value: 'pgbackrest' }
    ]
  }
  return [
    { label: '外部 binlog', value: 'external_binlog' },
    { label: 'mysqlbinlog polling', value: 'mysqlbinlog_polling' },
    { label: 'mysqlbinlog streaming', value: 'mysqlbinlog_streaming' }
  ]
})

const isPgBackRestExternalBackup = computed(() =>
  isPgBackRestEngineValue(externalBackupForm.backupEngine)
)

const isPgBackRestLogArchiveStream = computed(() =>
  isPgBackRestEngineValue(logArchiveStreamForm.archiveEngine)
)

const normalizeLogArchiveEngineForType = () => {
  if (!logArchiveEngineOptions.value.some(item => item.value === logArchiveStreamForm.archiveEngine)) {
    logArchiveStreamForm.archiveEngine = logArchiveStreamForm.archiveType === 'wal' ? 'external_wal' : 'external_binlog'
  }
}

const postgresqlRestoreRunAlertTitle = computed(() => {
  if (isPgBaseBackupRestoreRun.value) {
    return 'P3.8 可执行 pg_basebackup artifact 恢复到 Runner 隔离目录，也可结合计划中的 WAL 启动隔离 PostgreSQL 实例做校验；不会覆盖生产库。'
  }
  return 'P3.7 可执行 Barman restore 到 Runner 隔离目录，并可启动隔离 PostgreSQL 实例做校验；不会覆盖生产库。'
})

const selectedBackupTaskInstance = computed(() =>
  instanceOptions.value.find(item => item.id === backupTaskForm.instanceId)
)

const selectedBackupPolicyInstance = computed(() =>
  instanceOptions.value.find(item => item.id === backupPolicyForm.instanceId)
)

const selectedBackupPolicyDbType = computed(() =>
  selectedBackupPolicyInstance.value?.dbType || ''
)

const availableBackupPolicyEngineOptions = computed(() => {
  if (selectedBackupPolicyDbType.value === 'mariadb') {
    return [
      { label: 'mariadb-backup', value: 'mariadb_backup' }
    ]
  }
  return [
    { label: 'XtraBackup 8.0（MySQL 8.0.x）', value: 'xtrabackup_8_0' },
    { label: 'XtraBackup 8.4（MySQL 8.4.x）', value: 'xtrabackup_8_4' },
    { label: 'XtraBackup 2.4 legacy（MySQL 5.7）', value: 'xtrabackup_2_4' }
  ]
})

const defaultPhysicalBackupPolicyEngine = () => {
  if (selectedBackupPolicyDbType.value === 'mariadb') return 'mariadb_backup'
  const version = String(selectedBackupPolicyInstance.value?.version || '')
  const match = version.match(/(\d+)\.(\d+)/)
  const major = match ? Number(match[1]) : 0
  const minor = match ? Number(match[2]) : 0
  if (major === 5) return 'xtrabackup_2_4'
  if (major === 8 && minor >= 4) return 'xtrabackup_8_4'
  return 'xtrabackup_8_0'
}

const backupPolicyBinlogStreamOptions = computed(() =>
  logArchiveStreams.value
    .filter(item => item.archiveType === 'binlog' && (!backupPolicyForm.instanceId || item.instanceId === backupPolicyForm.instanceId))
    .map(item => ({
      id: item.id,
      label: `${item.instanceName || `#${item.instanceId}`} / ${item.archiveEngine || item.archiveMode || 'binlog'} / ${item.statusText || item.status || '-'}`
    }))
)

type SyntheticRuleSummary = {
  valid: boolean
  mode: string
  autoRun: boolean
  triggerAfterIncrementals: number
  mergeOldestIncrementals: number
  requireRestoreProof: boolean
}

const parseBackupPolicySyntheticRule = (raw?: string): SyntheticRuleSummary => {
  const defaults: SyntheticRuleSummary = {
    valid: true,
    mode: 'rolling_synthetic_full',
    autoRun: false,
    triggerAfterIncrementals: 5,
    mergeOldestIncrementals: 5,
    requireRestoreProof: true
  }
  if (!raw || !raw.trim()) return defaults
  try {
    const parsed = JSON.parse(raw)
    const trigger = Number(parsed.triggerAfterIncrementals || defaults.triggerAfterIncrementals)
    const merge = Number(parsed.mergeOldestIncrementals || trigger || defaults.mergeOldestIncrementals)
    return {
      valid: true,
      mode: String(parsed.mode || defaults.mode),
      autoRun: parsed.autoRun === true,
      triggerAfterIncrementals: Number.isFinite(trigger) && trigger > 0 ? trigger : defaults.triggerAfterIncrementals,
      mergeOldestIncrementals: Number.isFinite(merge) && merge > 0 ? merge : defaults.mergeOldestIncrementals,
      requireRestoreProof: parsed.requireRestoreProof !== false
    }
  } catch {
    return {
      ...defaults,
      valid: false
    }
  }
}

const backupPolicySyntheticRuleText = (row: DatabaseBackupPolicyResult) => {
  if (!row.syntheticEnabled) return ''
  const rule = parseBackupPolicySyntheticRule(row.syntheticRuleJson)
  if (!rule.valid) return 'Synthetic规则解析失败'
  if (!rule.autoRun) return '自动合成：关闭'
  const count = row.chain?.incrementalCount || 0
  const remain = Math.max(rule.triggerAfterIncrementals - count, 0)
  return remain > 0
    ? `自动合成 ${count}/${rule.triggerAfterIncrementals} 条，还差 ${remain} 条`
    : `自动合成已达阈值 ${count}/${rule.triggerAfterIncrementals} 条`
}

const protectionModeTag = (mode?: string) => {
  if (!mode || mode === 'none') return 'info'
  if (mode === 'logical_backup') return 'warning'
  if (mode.includes('pitr')) return 'success'
  return 'primary'
}

const protectionLevelTag = (level?: string) => {
  switch (level) {
    case 'ha_and_pitr_verified':
    case 'pitr_verified':
      return 'success'
    case 'pitr_capable':
      return 'primary'
    case 'backup_only':
      return 'warning'
    case 'none':
      return 'danger'
    default:
      return 'info'
  }
}

const protectionHealthTag = (status?: string) => {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'warning':
      return 'warning'
    case 'critical':
      return 'danger'
    default:
      return 'info'
  }
}

const restoreDrillStatusTag = (status?: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'stale':
      return 'warning'
    default:
      return 'info'
  }
}

const protectionRecoverableWindowText = (row: DatabaseProtectionProfileResult) => {
  if (!row.recoverableFrom && !row.recoverableUntil) return '-'
  return `${row.recoverableFrom || '-'} -> ${row.recoverableUntil || '-'}`
}

const protectionRPOText = (row: DatabaseProtectionProfileResult) => {
  if (row.rpoLagSeconds === undefined || row.rpoLagSeconds === null || row.rpoLagSeconds < 0) return '-'
  if (row.rpoLagSeconds < 60) return `${row.rpoLagSeconds}s`
  return `${Math.round(row.rpoLagSeconds / 60)}m`
}

const backupPolicyFormSyntheticRuleSummary = computed(() => parseBackupPolicySyntheticRule(backupPolicyForm.syntheticRuleJson))

const backupPolicyFormSyntheticRuleTip = computed(() => {
  const rule = backupPolicyFormSyntheticRuleSummary.value
  if (!rule.valid) return '规则 JSON 解析失败，后端会拒绝或按安全默认值处理。'
  return `自动合成：${rule.autoRun ? '开启' : '关闭'}；触发增量数：${rule.triggerAfterIncrementals}；合并增量数：${rule.mergeOldestIncrementals}；恢复证明：${rule.requireRestoreProof ? '要求' : '不要求'}。`
})

const backupPolicyFormSyntheticRuleWarning = computed(() => {
  const rule = backupPolicyFormSyntheticRuleSummary.value
  if (!rule.valid) return 'Synthetic 规则不是有效 JSON。'
  if (!rule.autoRun) return ''
  if (!backupPolicyForm.binlogStreamId) return '自动 Synthetic Full 需要绑定 binlog 归档流；未绑定时后端会阻止自动执行。'
  if (rule.mergeOldestIncrementals !== rule.triggerAfterIncrementals) {
    return '第一版自动合成建议整段合并：mergeOldestIncrementals 必须等于 triggerAfterIncrementals，否则后端会阻止自动执行。'
  }
  return ''
})

const selectedBackupTaskDbType = computed(() =>
  selectedBackupTaskInstance.value?.dbType || backupTaskFormInstanceDbType.value
)

const isMySQLFamilyBackupTask = computed(() =>
  ['mysql', 'mariadb'].includes(selectedBackupTaskDbType.value)
)

const isPhysicalBackupTaskForm = computed(() =>
  backupTaskForm.backupMethod === 'physical'
)

const backupLargeWarnings = computed(() =>
  backupTasks.value.filter(item => item.largeDataWarning)
)

const availableBackupMethodOptions = computed(() => {
  const options = [
    { label: '逻辑备份', value: 'logical' }
  ]
  if (isMySQLFamilyBackupTask.value || selectedBackupTaskDbType.value === 'postgresql') {
    options.push({ label: '物理备份', value: 'physical' })
  }
  return options
})

const availableBackupTypeOptions = computed(() => {
  if (isPhysicalBackupTaskForm.value) {
    return [
      { label: '物理备份', value: 'physical' }
    ]
  }
  if (selectedBackupTaskDbType.value === 'postgresql') {
    return [
      { label: '逻辑备份（Plain SQL）', value: 'logical' },
      { label: '逻辑备份（Custom）', value: 'logical_custom' }
    ]
  }
  return [
    { label: '逻辑备份', value: 'logical' }
  ]
})

const availableBackupLevelOptions = computed(() => {
  if (isPhysicalBackupTaskForm.value) {
    if (selectedBackupTaskDbType.value === 'postgresql' && backupTaskForm.backupEngine === 'pg_basebackup') {
      return [
        { label: '全量', value: 'full' }
      ]
    }
    return [
      { label: '全量', value: 'full' },
      { label: '增量', value: 'incremental' }
    ]
  }
  return [
    { label: '全量', value: 'full' }
  ]
})

const defaultPhysicalBackupEngine = () => {
  if (selectedBackupTaskDbType.value === 'postgresql') return 'barman'
  if (selectedBackupTaskDbType.value === 'mariadb') return 'mariadb_backup'
  const version = String(selectedBackupTaskInstance.value?.version || '')
  const match = version.match(/(\d+)\.(\d+)/)
  const major = match ? Number(match[1]) : 0
  const minor = match ? Number(match[2]) : 0
  if (major === 5) return 'xtrabackup_2_4'
  if (major === 8 && minor >= 4) return 'xtrabackup_8_4'
  return 'xtrabackup_8_0'
}

const availablePhysicalBackupEngineOptions = computed(() => {
  if (selectedBackupTaskDbType.value === 'postgresql') {
    return [
      { label: 'Barman（cluster 级）', value: 'barman' },
      { label: 'pg_basebackup（轻量原生 full）', value: 'pg_basebackup' }
    ]
  }
  if (selectedBackupTaskDbType.value === 'mariadb') {
    return [
      { label: 'mariadb-backup', value: 'mariadb_backup' }
    ]
  }
  return [
    { label: 'XtraBackup 8.0（MySQL 8.0.x）', value: 'xtrabackup_8_0' },
    { label: 'XtraBackup 8.4（MySQL 8.4.x）', value: 'xtrabackup_8_4' },
    { label: 'XtraBackup 2.4 legacy（MySQL 5.7）', value: 'xtrabackup_2_4' }
  ]
})

const backupCapacityTipTitle = computed(() => {
  const sizeText = selectedBackupTaskInstance.value?.capacitySizeText || '-'
  if (isPhysicalBackupTaskForm.value) {
    if (selectedBackupTaskDbType.value === 'postgresql') {
      return `当前实例容量 ${sizeText}；PostgreSQL 物理备份固定为 cluster 级，PITR 还需要配套 WAL 归档和恢复演练。`
    }
    return `当前实例容量 ${sizeText}；物理备份适合作为大库主链路，PITR 还需要配套 binlog 归档和恢复演练。`
  }
  return `当前实例容量 ${sizeText}，逻辑全量备份只适合作为小库、临时导出或演练能力。`
})

const backupCapacityTipType = computed(() => {
  if (isPhysicalBackupTaskForm.value) return 'warning'
  return Number(selectedBackupTaskInstance.value?.capacitySizeBytes || 0) >= 50 * 1024 * 1024 * 1024 ? 'error' : 'info'
})

const normalizeBackupTaskFormBackupType = () => {
  if (!availableBackupMethodOptions.value.some(item => item.value === backupTaskForm.backupMethod)) {
    backupTaskForm.backupMethod = 'logical'
  }
  if (isPhysicalBackupTaskForm.value) {
    backupTaskForm.backupType = 'physical'
    backupTaskForm.backupScope = selectedBackupTaskDbType.value === 'postgresql' ? 'cluster' : 'instance'
    backupTaskForm.compression = backupTaskForm.compression || 'gzip'
    if (!availableBackupLevelOptions.value.some(item => item.value === backupTaskForm.backupLevel)) {
      backupTaskForm.backupLevel = 'full'
    }
    if (!availablePhysicalBackupEngineOptions.value.some(item => item.value === backupTaskForm.backupEngine)) {
      backupTaskForm.backupEngine = defaultPhysicalBackupEngine()
    }
    return
  }
  if (!availableBackupTypeOptions.value.some(item => item.value === backupTaskForm.backupType)) {
    backupTaskForm.backupType = 'logical'
  }
  backupTaskForm.backupMethod = 'logical'
  backupTaskForm.backupLevel = 'full'
  backupTaskForm.backupEngine = 'logical'
  backupTaskForm.backupScope = 'database'
}

const restoreTargetInstances = computed(() =>
  instanceOptions.value.filter(item =>
    ['mysql', 'mariadb', 'postgresql', 'redis'].includes(item.dbType) &&
    hasDatabasePermission(item, DATABASE_PERMISSION.RESTORE) &&
    item.status === 'enabled' &&
    !isProductionEnvironment(item.environment)
  )
)

const pitrRestoreTargetInstances = computed(() =>
  restoreTargetInstances.value.filter(item => ['mysql', 'mariadb', 'postgresql'].includes(item.dbType))
)

const logArchiveStreamOptions = computed(() =>
  logArchiveStreams.value.map(item => ({
    id: item.id,
    label: `${item.instanceName || `#${item.instanceId}`} / ${item.archiveTypeText || item.archiveType} / ${item.archiveEngine || item.archiveMode || 'external'}`
  }))
)

const walLogArchiveStreamOptions = computed(() =>
  logArchiveStreams.value
    .filter(item => item.archiveType === 'wal')
    .map(item => ({
      id: item.id,
      label: `${item.instanceName || `#${item.instanceId}`} / ${item.archiveTypeText || item.archiveType} / ${item.archiveEngine || item.archiveMode || 'external'}`
    }))
)

const runnerHostOptions = computed(() =>
  runnerHosts.value.map(item => ({
    id: item.id,
    label: `${item.name}（${item.runnerTypeText || item.runnerType}${item.host ? ` / ${item.host}:${item.port || 22}` : ''}）`
  }))
)

const runnerToolRows = computed(() =>
  Object.values(runnerToolProfile.value?.tools || {}).sort((a, b) => a.name.localeCompare(b.name))
)

const runnerToolWarnings = computed(() => [
  ...(runnerToolProfile.value?.capability?.warnings || []),
  ...(runnerToolProfile.value?.compatibility?.warnings || [])
])
const runnerJobParsedResult = computed<Record<string, any>>(() => {
  if (!runnerJobDetail.value?.resultJson) return {}
  try {
    return JSON.parse(runnerJobDetail.value.resultJson) || {}
  } catch (_err) {
    return {}
  }
})
const runnerJobInstallSteps = computed(() => Array.isArray(runnerJobParsedResult.value.steps) ? runnerJobParsedResult.value.steps : [])

const storagePostureChecks = computed(() => {
  const raw = currentStoragePostureDetail.value?.postureJson || ''
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed.checks) ? parsed.checks : []
  } catch {
    return []
  }
})

const formattedStoragePostureJson = computed(() => {
  const raw = currentStoragePostureDetail.value?.postureJson || ''
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})

const formatJSONText = (raw: string) => {
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

const sshCredentialOptions = computed(() =>
  credentials.value.filter((item: any) => (item.protocol || 'ssh') === 'ssh')
)

const restoreTargetOptions = computed(() => {
  const sourceType = restoreSourceRecord.value
    ? instanceOptions.value.find(item => item.id === restoreSourceRecord.value?.instanceId)?.dbType
    : ''
  return restoreTargetInstances.value.filter(item =>
    item.id !== restoreSourceRecord.value?.instanceId && isRestoreCompatibleType(sourceType, item.dbType)
  )
})

const restoreSourceDbType = computed(() =>
  restoreSourceRecord.value
    ? instanceOptions.value.find(item => item.id === restoreSourceRecord.value?.instanceId)?.dbType || ''
    : ''
)

const availableRestoreStrategyOptions = computed(() => {
  if (restoreSourceDbType.value === 'postgresql' && restoreSourceRecord.value?.backupType === 'logical') {
    return [
      { label: '清空库', value: 'database_clean' }
    ]
  }
  if (['mysql', 'mariadb', 'postgresql'].includes(restoreSourceDbType.value)) {
    return [
      { label: '覆盖对象', value: 'object_replace' },
      { label: '清空库', value: 'database_clean' }
    ]
  }
  return [
    { label: '覆盖对象', value: 'object_replace' }
  ]
})

const restoreStrategyTip = computed(() =>
  restoreForm.restoreStrategy === 'database_clean'
    ? '先删除目标库中的业务对象再导入备份；目标库额外对象会被删除。'
    : '仅替换备份中包含的对象；目标库额外对象会保留，同名对象按备份内容覆盖。'
)

const backupTaskOptions = computed(() =>
  backupTasks.value.map(item => ({
    id: item.id,
    name: item.name
  }))
)

const defaultDatabaseLabel = computed(() => {
  switch (form.dbType) {
    case 'oracle':
      return '服务名'
    case 'mongodb':
      return '认证库'
    case 'elasticsearch':
    case 'opensearch':
      return '集群名'
    default:
      return '默认库'
  }
})

const defaultDatabasePlaceholder = computed(() => {
  switch (form.dbType) {
    case 'mysql':
    case 'mariadb':
      return '可选，如 opshub'
    case 'postgresql':
      return '如 postgres'
    case 'sqlserver':
      return '如 master / appdb'
    case 'clickhouse':
      return '如 default'
    case 'oracle':
      return '如 XEPDB1'
    case 'tidb':
      return '可选，如 test'
    case 'oceanbase':
      return 'MySQL 模式租户下的库名'
    case 'opengauss':
      return '如 postgres'
    case 'kingbase':
      return '如 test'
    case 'dameng':
      return '可选，后续专用驱动接入'
    case 'mongodb':
      return '如 admin'
    case 'elasticsearch':
    case 'opensearch':
      return '可选，用于标记集群'
    default:
      return '可选'
  }
})

const connectionParamsPlaceholder = computed(() => {
  switch (form.dbType) {
    case 'sqlserver':
      return '{"encrypt":"true","trustServerCertificate":"true","applicationIntent":"ReadOnly"}'
    case 'clickhouse':
      return '{"protocol":"native","readTimeoutSeconds":10}'
    case 'oracle':
      return '{"serviceName":"XEPDB1"} 或 {"sid":"XE"}'
    case 'redis':
      return '{"db":0,"insecureSkipVerify":false} 或 {"masterName":"mymaster","sentinelAddrs":["10.0.0.1:26379","10.0.0.2:26379"]}'
    case 'mongodb':
      return '{"authSource":"admin","replicaSet":"rs0","insecureSkipVerify":false}'
    case 'elasticsearch':
    case 'opensearch':
      return '{"scheme":"https","insecureSkipVerify":false} 或 {"url":"https://search.example.com:9200"}'
    case 'tidb':
    case 'oceanbase':
      return '兼容 MySQL 协议，可选 JSON 格式'
    case 'opengauss':
    case 'kingbase':
      return '兼容 PostgreSQL 协议，可选 JSON 格式'
    default:
      return '可选，JSON 格式'
  }
})

const connectionParamsTip = computed(() => {
  switch (form.dbType) {
    case 'sqlserver':
      return 'SQL Server 可填写 encrypt、trustServerCertificate、applicationIntent、appName。'
    case 'clickhouse':
      return 'ClickHouse 可填写 protocol=http/native、readTimeoutSeconds、insecureSkipVerify。'
    case 'oracle':
      return 'Oracle 建议填写 serviceName；如果走 SID，可填 sid，也可追加 server=DEDICATED。'
    case 'redis':
      return 'Redis 可填写 db、masterName、sentinelAddrs；TLS 场景可填写 insecureSkipVerify。Cluster 拓扑会读取 CLUSTER NODES / SLOTS，Sentinel 拓扑会读取 SENTINEL masters / replicas / sentinels。'
    case 'mongodb':
      return 'MongoDB 可填写 authSource、replicaSet；拓扑会优先读取 replSetGetStatus。'
    case 'elasticsearch':
    case 'opensearch':
      return '搜索集群可填写 scheme 或完整 url；拓扑会读取 cluster health、cat nodes 和 cat shards。'
    case 'tidb':
    case 'oceanbase':
      return '当前按 MySQL 兼容协议接入，差异项后续批次补齐。'
    case 'opengauss':
    case 'kingbase':
      return '当前按 PostgreSQL 兼容协议接入，差异项后续批次补齐。'
    case 'dameng':
      return '达梦本批先完成实例纳管和类型注册，连接与元数据待专用驱动接入。'
    default:
      return '连接参数为可选 JSON，未填写时使用系统默认连接策略。'
  }
})

const credentialFieldTip = computed(() => {
  if (form.dbType === 'redis') {
    return 'Redis 复用资产凭据模块；支持密码认证、只填密码，以及空用户名空密码的无认证凭据。'
  }
  if (['mongodb', 'elasticsearch', 'opensearch'].includes(form.dbType)) {
    return '该类型复用资产凭据模块；如果实例未开启认证，可选择空用户名空密码的无认证凭据。'
  }
  return '一期复用资产凭据模块，数据库密码使用凭据中的用户名和密码。'
})

const schemaTree = computed(() =>
  schemas.value.map(item => ({
    id: `${item.instanceId}-${item.schemaName}`,
    label: item.schemaName,
    schemaName: item.schemaName,
    tableCount: item.tableCount || 0
  }))
)

const loadSupportedTypes = async () => {
  supportedTypes.value = await getDatabaseSupportedTypes()
}

const loadCredentials = async () => {
  credentials.value = await getCredentials()
}

const loadUIPermissions = async () => {
  try {
    const res: any = await getDatabaseUIPermissions()
    uiPermissions.value = res || {}
    if (activeTab.value === 'permissions' && !canManageInstancePermissions.value) {
      activeTab.value = 'instances'
    }
  } catch (error: any) {
    uiPermissions.value = {}
    ElMessage.warning('加载数据库页面权限失败')
  }
}

const applyDatabaseWriteConfig = (res: Partial<SystemDatabaseConfig> | undefined) => {
  if (!res) return
  databaseConfig.writeEnabled = !!res.writeEnabled
  databaseConfig.writeExplainEnabled = !!res.writeExplainEnabled
  databaseConfig.ddlEnabled = !!res.ddlEnabled
  databaseConfig.ddlHighRiskRequiresConfirm = res.ddlHighRiskRequiresConfirm !== false
  databaseConfig.ddlReasonRequired = res.ddlReasonRequired !== false
  databaseConfig.ddlRequireBackupHint = res.ddlRequireBackupHint !== false
  databaseConfig.highRiskRequiresConfirm = res.highRiskRequiresConfirm !== false
  databaseConfig.operationReasonRequired = res.operationReasonRequired !== false
  databaseConfig.maxAffectedRows = res.maxAffectedRows || 1000
  databaseConfig.defaultBackupRetentionDays = res.defaultBackupRetentionDays || 7
  databaseConfig.backupStoragePath = res.backupStoragePath || './data/database-backups'
  databaseConfig.instancePermissionMode = res.instancePermissionMode === 'whitelist' ? 'whitelist' : 'compat'
}

const loadDatabaseWriteConfig = async () => {
  databaseWriteConfigLoading.value = true
  try {
    const res: any = await getSystemDatabaseConfig()
    applyDatabaseWriteConfig(res)
  } catch {
    ElMessage.warning('加载数据库写操作配置失败')
  } finally {
    databaseWriteConfigLoading.value = false
  }
}

const handleBeforeDatabaseWriteToggle = async () => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权切换数据库写操作总开关')
    return false
  }
  if (databaseWriteConfigSaving.value) {
    return false
  }
  const nextEnabled = !databaseConfig.writeEnabled
  if (nextEnabled) {
    try {
      await ElMessageBox.confirm(
        '开启后，拥有写入权限的用户可通过预检查和受控确认执行数据库写操作。确认开启？',
        '开启数据库写操作总开关',
        {
          type: 'warning',
          confirmButtonText: '开启',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return false
    }
  }
  databaseWriteConfigSaving.value = true
  try {
    await saveSystemDatabaseConfig({
      ...databaseConfig,
      writeEnabled: nextEnabled
    })
    clearWriteConsoleState()
    ElMessage.success(nextEnabled ? '数据库写操作总开关已开启' : '数据库写操作总开关已关闭')
    return true
  } catch {
    return false
  } finally {
    databaseWriteConfigSaving.value = false
  }
}

const handleBeforeDatabaseWriteExplainToggle = async () => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权切换写 SQL 执行计划开关')
    return false
  }
  if (databaseWriteConfigSaving.value) {
    return false
  }
  const nextEnabled = !databaseConfig.writeExplainEnabled
  if (nextEnabled) {
    try {
      await ElMessageBox.confirm(
        '开启后，拥有写 SQL 计划权限的用户可以对 INSERT / UPDATE / DELETE 等语句查看 EXPLAIN。确认开启？',
        '开启写 SQL 执行计划',
        {
          type: 'warning',
          confirmButtonText: '开启',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return false
    }
  }
  databaseWriteConfigSaving.value = true
  try {
    await saveSystemDatabaseConfig({
      ...databaseConfig,
      writeExplainEnabled: nextEnabled
    })
    clearWriteConsoleState()
    ElMessage.success(nextEnabled ? '写 SQL 执行计划已开启' : '写 SQL 执行计划已关闭')
    return true
  } catch {
    return false
  } finally {
    databaseWriteConfigSaving.value = false
  }
}

const handleBeforeDatabaseDDLToggle = async () => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权切换 DDL 结构变更开关')
    return false
  }
  if (databaseWriteConfigSaving.value) {
    return false
  }
  const nextEnabled = !databaseConfig.ddlEnabled
  if (nextEnabled) {
    try {
      await ElMessageBox.confirm(
        '开启后，拥有 DDL 实例权限的用户可执行受控 CREATE TABLE / CREATE INDEX。确认开启？',
        '开启 DDL 结构变更',
        {
          type: 'warning',
          confirmButtonText: '开启',
          cancelButtonText: '取消'
        }
      )
    } catch {
      return false
    }
  }
  databaseWriteConfigSaving.value = true
  try {
    await saveSystemDatabaseConfig({
      ...databaseConfig,
      ddlEnabled: nextEnabled
    })
    clearWriteConsoleState()
    ElMessage.success(nextEnabled ? 'DDL 结构变更开关已开启' : 'DDL 结构变更开关已关闭')
    return true
  } catch {
    return false
  } finally {
    databaseWriteConfigSaving.value = false
  }
}

const loadRoles = async () => {
  const res: any = await getAllRoles()
  roleOptions.value = (res || []).map((item: any) => ({
    ...item,
    id: item.id || item.ID,
    name: item.name || item.Name,
    code: item.code || item.Code
  }))
}

const loadInstances = async () => {
  loading.value = true
  try {
    const res: any = await listDatabaseInstances(query)
    instances.value = res.list || []
    total.value = res.total || 0
    if (res.page) query.page = res.page
    if (res.pageSize) query.pageSize = res.pageSize
  } finally {
    loading.value = false
  }
}

const pruneSelectedInstanceRefs = () => {
  const hasInstanceOption = (id?: number) =>
    !!id && instanceOptions.value.some(item => item.id === id)

  if (metadataInstanceId.value && !metadataInstances.value.some(item => item.id === metadataInstanceId.value)) {
    metadataInstanceId.value = undefined
    resetMetadataSelection()
  }
  if (queryInstanceId.value && !queryInstances.value.some(item => item.id === queryInstanceId.value)) {
    queryInstanceId.value = undefined
    querySchemaName.value = ''
    querySchemas.value = []
    queryUnlimitedRows.value = false
    clearQueryConsoleState()
  }
  if (queryUnlimitedRows.value && !canUseQueryUnlimitedRows.value) {
    queryUnlimitedRows.value = false
  }
  if (diagnosisInstanceId.value && !diagnosisInstances.value.some(item => item.id === diagnosisInstanceId.value)) {
    diagnosisInstanceId.value = undefined
    diagnosisMetrics.value = undefined
    diagnosisSessions.value = []
    diagnosisSlowQueries.value = []
    diagnosisSlowMessage.value = ''
    capacityTrend.value = undefined
  }
  if (topologyInstanceId.value && !topologyInstances.value.some(item => item.id === topologyInstanceId.value)) {
    topologyInstanceId.value = undefined
    topologyResult.value = undefined
  }
  if (replicationReplicaQuery.instanceId && !replicationInstances.value.some(item => item.id === replicationReplicaQuery.instanceId)) {
    replicationReplicaQuery.instanceId = undefined
  }
  if (replicationProtectionQuery.instanceId && !replicationInstances.value.some(item => item.id === replicationProtectionQuery.instanceId)) {
    replicationProtectionQuery.instanceId = undefined
  }
  if (replicationCheckQuery.instanceId && !replicationInstances.value.some(item => item.id === replicationCheckQuery.instanceId)) {
    replicationCheckQuery.instanceId = undefined
  }
  if (replicaIncidentGuideQuery.instanceId && !replicationInstances.value.some(item => item.id === replicaIncidentGuideQuery.instanceId)) {
    replicaIncidentGuideQuery.instanceId = undefined
  }
  if (replicaActionQuery.instanceId && !replicationInstances.value.some(item => item.id === replicaActionQuery.instanceId)) {
    replicaActionQuery.instanceId = undefined
  }
  if (backupTaskQuery.instanceId && !supportedBackupInstances.value.some(item => item.id === backupTaskQuery.instanceId)) {
    backupTaskQuery.instanceId = undefined
  }
  if (backupRecordQuery.instanceId && !supportedBackupInstances.value.some(item => item.id === backupRecordQuery.instanceId)) {
    backupRecordQuery.instanceId = undefined
  }
  if (logArchiveStreamQuery.instanceId && !pitrBackupInstances.value.some(item => item.id === logArchiveStreamQuery.instanceId)) {
    logArchiveStreamQuery.instanceId = undefined
  }
  if (logArchiveQuery.instanceId && !pitrBackupInstances.value.some(item => item.id === logArchiveQuery.instanceId)) {
    logArchiveQuery.instanceId = undefined
  }
  if (barmanServerQuery.sourceInstanceId && !postgresqlBackupInstances.value.some(item => item.id === barmanServerQuery.sourceInstanceId)) {
    barmanServerQuery.sourceInstanceId = undefined
  }
  if (restorePlanQuery.sourceInstanceId && !hasInstanceOption(restorePlanQuery.sourceInstanceId)) {
    restorePlanQuery.sourceInstanceId = undefined
  }
  if (restorePlanQuery.targetInstanceId && !hasInstanceOption(restorePlanQuery.targetInstanceId)) {
    restorePlanQuery.targetInstanceId = undefined
  }
  if (restoreJobQuery.sourceInstanceId && !hasInstanceOption(restoreJobQuery.sourceInstanceId)) {
    restoreJobQuery.sourceInstanceId = undefined
  }
  if (restoreJobQuery.targetInstanceId && !hasInstanceOption(restoreJobQuery.targetInstanceId)) {
    restoreJobQuery.targetInstanceId = undefined
  }
  if (inspectionQuery.instanceId && !diagnosisInstances.value.some(item => item.id === inspectionQuery.instanceId)) {
    inspectionQuery.instanceId = undefined
  }
  if (backupTaskForm.instanceId && !supportedBackupInstances.value.some(item => item.id === backupTaskForm.instanceId)) {
    backupTaskForm.instanceId = 0
  }
  if (externalBackupForm.instanceId && !pitrBackupInstances.value.some(item => item.id === externalBackupForm.instanceId)) {
    externalBackupForm.instanceId = 0
  }
  if (logArchiveStreamForm.instanceId && !pitrBackupInstances.value.some(item => item.id === logArchiveStreamForm.instanceId)) {
    logArchiveStreamForm.instanceId = 0
  }
  if (barmanServerForm.sourceInstanceId && !postgresqlBackupInstances.value.some(item => item.id === barmanServerForm.sourceInstanceId)) {
    barmanServerForm.sourceInstanceId = 0
  }
  if (restorePlanForm.sourceInstanceId && !pitrBackupInstances.value.some(item => item.id === restorePlanForm.sourceInstanceId)) {
    restorePlanForm.sourceInstanceId = 0
  }
  if (inspectionForm.instanceId && !diagnosisInstances.value.some(item => item.id === inspectionForm.instanceId)) {
    inspectionForm.instanceId = undefined
  }
}

const loadInstanceOptions = async () => {
  const pageSize = 100
  const options: any[] = []
  let page = 1
  let totalCount = 0

  while (page <= 100) {
    const res: any = await listDatabaseInstances({ page, pageSize })
    const list = Array.isArray(res?.list) ? res.list : []
    options.push(...list)
    totalCount = Number(res?.total || options.length)
    if (list.length === 0 || options.length >= totalCount) {
      break
    }
    page += 1
  }

  const seen = new Set<number>()
  instanceOptions.value = options.filter(item => {
    const id = Number(item?.id || 0)
    if (!id || seen.has(id)) return false
    seen.add(id)
    return true
  })
  pruneSelectedInstanceRefs()
}

const ensureMetadataInstance = async () => {
  if (!metadataInstanceId.value && metadataInstances.value.length > 0) {
    metadataInstanceId.value = metadataInstances.value[0].id
  }
  if (metadataInstanceId.value) {
    await loadSchemas()
  }
}

const resetMetadataSelection = () => {
  schemas.value = []
  tables.value = []
  columns.value = []
  indexes.value = []
  selectedSchema.value = ''
  selectedTable.value = undefined
  currentDDL.value = undefined
  ddlDialogVisible.value = false
}

const loadSchemas = async () => {
  if (!metadataInstanceId.value) {
    resetMetadataSelection()
    return
  }
  schemasLoading.value = true
  try {
    schemas.value = await listDatabaseSchemas(metadataInstanceId.value)
    selectedSchema.value = schemas.value[0]?.schemaName || ''
    selectedTable.value = undefined
    columns.value = []
    indexes.value = []
    if (selectedSchema.value) {
      await loadTables(selectedSchema.value)
    } else {
      tables.value = []
    }
  } finally {
    schemasLoading.value = false
  }
}

const loadTables = async (schemaName: string) => {
  if (!metadataInstanceId.value) return
  tablesLoading.value = true
  try {
    tables.value = await listDatabaseTables(metadataInstanceId.value, { schemaName })
    selectedTable.value = tables.value[0]
    if (selectedTable.value) {
      await loadTableDetails(selectedTable.value)
    } else {
      columns.value = []
      indexes.value = []
    }
  } finally {
    tablesLoading.value = false
  }
}

const loadTableDetails = async (table: any) => {
  if (!metadataInstanceId.value || !table?.tableName) return
  detailsLoading.value = true
  try {
    const params = {
      schemaName: table.schemaName,
      tableName: table.tableName
    }
    const [columnRes, indexRes] = await Promise.all([
      listDatabaseColumns(metadataInstanceId.value, params),
      listDatabaseIndexes(metadataInstanceId.value, params)
    ])
    columns.value = columnRes as any[]
    indexes.value = indexRes as any[]
  } finally {
    detailsLoading.value = false
  }
}

const ensureQueryInstance = async () => {
  if (!queryInstanceId.value && queryInstances.value.length > 0) {
    queryInstanceId.value = queryInstances.value[0].id
  }
  if (queryUnlimitedRows.value && !canUseQueryUnlimitedRows.value) {
    queryUnlimitedRows.value = false
  }
  if (queryInstanceId.value) {
    await loadQuerySchemas()
    await loadQueryHistory()
  }
}

const loadQuerySchemas = async () => {
  if (!queryInstanceId.value) {
    querySchemas.value = []
    querySchemaName.value = ''
    return
  }
  querySchemas.value = await listDatabaseSchemas(queryInstanceId.value)
  const schemaNames = querySchemas.value.map(item => item.schemaName)
  if (!querySchemaName.value || !schemaNames.includes(querySchemaName.value)) {
    querySchemaName.value = defaultQuerySchemaName()
  }
}

const resetWriteConfirmForm = () => {
  writeConfirmForm.reason = ''
  writeConfirmForm.confirmed = false
}

const resetDDLConfirmForm = () => {
  ddlConfirmForm.reason = ''
  ddlConfirmForm.confirmed = false
}

const clearReadOnlyConsoleState = () => {
  queryResult.value = undefined
  explainResult.value = undefined
}

const clearWriteConsoleState = () => {
  writeCheckResult.value = undefined
  ddlCheckResult.value = undefined
  writeResult.value = undefined
  writeConfirmVisible.value = false
  ddlConfirmVisible.value = false
  resetWriteConfirmForm()
  resetDDLConfirmForm()
}

const clearQueryConsoleState = () => {
  clearReadOnlyConsoleState()
  clearWriteConsoleState()
  queryResultTab.value = 'result'
}

const resetDiagnosisState = () => {
  diagnosisMetrics.value = undefined
  diagnosisSessions.value = []
  diagnosisSlowQueries.value = []
  diagnosisSlowMessage.value = ''
  capacityTrend.value = undefined
}

const ensureDiagnosisInstance = async () => {
  if (!diagnosisInstanceId.value && diagnosisInstances.value.length > 0) {
    diagnosisInstanceId.value = diagnosisInstances.value[0].id
  }
  if (diagnosisInstanceId.value) {
    await loadDiagnosisData()
  }
}

const loadDiagnosisData = async () => {
  const instanceId = diagnosisInstanceId.value
  if (!instanceId) {
    resetDiagnosisState()
    return
  }
  const requestSeq = ++diagnosisRequestSeq
  const capacitySeq = ++capacityTrendRequestSeq
  const range = capacityRange.value
  diagnosisLoading.value = true
  capacityLoading.value = true
  try {
    const [metricsRes, sessionsRes, slowRes, capacityRes] = await Promise.all([
      getDatabaseDiagnosisMetrics(instanceId),
      listDatabaseDiagnosisSessions(instanceId, { limit: diagnosisQuery.sessionLimit }),
      listDatabaseSlowQueries(instanceId, { limit: diagnosisQuery.slowLimit }),
      getDatabaseCapacityTrend(instanceId, {
        range,
        topLimit: 10
      }) as Promise<DatabaseCapacityTrendResult>
    ])
    if (
      requestSeq !== diagnosisRequestSeq ||
      diagnosisInstanceId.value !== instanceId ||
      capacitySeq !== capacityTrendRequestSeq ||
      capacityRange.value !== range
    ) {
      return
    }
    diagnosisMetrics.value = metricsRes
    diagnosisSessions.value = (sessionsRes as any)?.items || []
    diagnosisSlowQueries.value = (slowRes as any)?.items || []
    diagnosisSlowMessage.value = (slowRes as any)?.message || ''
    capacityTrend.value = capacityRes
  } finally {
    if (requestSeq === diagnosisRequestSeq) {
      diagnosisLoading.value = false
    }
    if (capacitySeq === capacityTrendRequestSeq) {
      capacityLoading.value = false
    }
  }
}

const loadCapacityTrend = async (instanceId = diagnosisInstanceId.value) => {
  if (!instanceId) {
    capacityTrend.value = undefined
    capacityLoading.value = false
    return
  }
  const range = capacityRange.value
  const requestSeq = ++capacityTrendRequestSeq
  capacityLoading.value = true
  try {
    const result = await getDatabaseCapacityTrend(instanceId, {
      range,
      topLimit: 10
    }) as DatabaseCapacityTrendResult
    if (
      requestSeq !== capacityTrendRequestSeq ||
      diagnosisInstanceId.value !== instanceId ||
      capacityRange.value !== range
    ) {
      return
    }
    capacityTrend.value = result
    await renderCapacityChart()
  } finally {
    if (requestSeq === capacityTrendRequestSeq) {
      capacityLoading.value = false
    }
  }
}

const handleCollectCapacity = async () => {
  const instanceId = diagnosisInstanceId.value
  if (!instanceId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  const instanceName = currentDiagnosisInstance.value?.name || `#${instanceId}`
  capacityCollecting.value = true
  try {
    const res = await collectDatabaseCapacitySnapshot(instanceId) as DatabaseCapacityCollectResult
    ElMessage.success(`${res.instanceName || instanceName} ${res.message || `容量快照已采集：${res.snapshotsCount || 0} 条`}`)
    if (diagnosisInstanceId.value === instanceId) {
      await loadCapacityTrend(instanceId)
    }
  } finally {
    capacityCollecting.value = false
  }
}

const ensureTopologyInstance = async () => {
  if (!topologyInstanceId.value && topologyInstances.value.length > 0) {
    topologyInstanceId.value = topologyInstances.value[0].id
  }
  if (topologyInstanceId.value) {
    await loadTopology()
  }
}

const loadTopology = async () => {
  if (!topologyInstanceId.value) {
    topologyResult.value = undefined
    return
  }
  topologyLoading.value = true
  try {
    topologyResult.value = await getDatabaseTopology(topologyInstanceId.value) as DatabaseTopologyResult
  } finally {
    topologyLoading.value = false
  }
}

const topologyInstanceIdFromNodeId = (nodeId?: string) => {
  const match = String(nodeId || '').match(/^instance:(\d+)$/)
  return match ? Number(match[1]) : 0
}

const topologyNodeInstanceId = (node?: DatabaseTopologyNode) => {
  const metricId = Number(node?.metrics?.instance_id || 0)
  if (metricId > 0) return metricId
  return topologyInstanceIdFromNodeId(node?.id)
}

const topologyLinkInstanceId = (link?: DatabaseTopologyLink) => {
  const metricReplicaId = Number(link?.metrics?.replica_instance_id || 0)
  if (metricReplicaId > 0) return metricReplicaId
  return topologyInstanceIdFromNodeId(link?.target) || topologyInstanceIdFromNodeId(link?.source)
}

const topologyRelatedInstanceIds = () => {
  const ids = new Set<number>()
  ;(topologyResult.value?.nodes || []).forEach((node) => {
    const id = topologyNodeInstanceId(node)
    if (id > 0 && replicationInstances.value.some(item => item.id === id)) {
      ids.add(id)
    }
  })
  if (topologyInstanceId.value && replicationInstances.value.some(item => item.id === topologyInstanceId.value)) {
    ids.add(topologyInstanceId.value)
  }
  return [...ids]
}

const collectTopologyReplicationChecks = async (ids: number[]) => {
  if (!ids.length) {
    ElMessage.warning('当前拓扑没有可采集的 MySQL / MariaDB / PostgreSQL 实例')
    return
  }
  let success = 0
  let failed = 0
  try {
    for (const id of ids) {
      topologyCollectingId.value = id
      try {
        await checkDatabaseReplication(id)
        success += 1
      } catch {
        failed += 1
      }
    }
    await loadTopology()
    if (failed > 0) {
      ElMessage.warning(`拓扑相关实例采集完成：成功 ${success}，失败 ${failed}`)
    } else {
      ElMessage.success(`拓扑相关实例采集完成：成功 ${success}`)
    }
  } finally {
    topologyCollectingId.value = 0
  }
}

const handleCheckCurrentTopology = async () => {
  const id = topologyInstanceId.value || 0
  if (!id) {
    ElMessage.warning('请先选择拓扑实例')
    return
  }
  await collectTopologyReplicationChecks([id])
}

const handleCheckTopologyRelated = async () => {
  topologyCollectingAll.value = true
  try {
    await collectTopologyReplicationChecks(topologyRelatedInstanceIds())
  } finally {
    topologyCollectingAll.value = false
  }
}

const openTopologyNodeDetail = (row: DatabaseTopologyNode) => {
  selectedTopologyNode.value = row
  selectedTopologyLink.value = undefined
  topologyDetailVisible.value = true
}

const openTopologyLinkDetail = (row: DatabaseTopologyLink) => {
  selectedTopologyLink.value = row
  selectedTopologyNode.value = undefined
  topologyDetailVisible.value = true
}

const handleTopologyDetailCheck = async () => {
  if (!topologyDetailInstanceId.value) {
    ElMessage.warning('该拓扑对象未匹配到已纳管实例')
    return
  }
  await collectTopologyReplicationChecks([topologyDetailInstanceId.value])
}

const openTopologyReplication = async (instanceId?: number) => {
  const shouldRefreshImmediately = activeTab.value === 'replication'
  if (instanceId) {
    replicationReplicaQuery.instanceId = instanceId
    replicationProtectionQuery.instanceId = instanceId
    replicationCheckQuery.instanceId = instanceId
    replicationReplicaQuery.page = 1
    replicationProtectionQuery.page = 1
    replicationCheckQuery.page = 1
  }
  activeTab.value = 'replication'
  topologyDetailVisible.value = false
  if (shouldRefreshImmediately) {
    await nextTick()
    await refreshReplicationState()
  }
}

const handleTopologyDetailReplication = async () => {
  if (!topologyDetailInstanceId.value) {
    ElMessage.warning('该拓扑对象未匹配到已纳管实例')
    return
  }
  await openTopologyReplication(topologyDetailInstanceId.value)
}

const handleTopologyDetailRaw = async () => {
  if (!topologyDetailInstanceId.value) {
    ElMessage.warning('该拓扑对象未匹配到已纳管实例')
    return
  }
  topologyDetailVisible.value = false
  await handleViewReplicationStatus(topologyDetailInstanceId.value)
}

const handleTopologyFindingAction = async (item: DatabaseTopologyFinding) => {
  const nodeId = item.nodeId || ''
  const link = item.linkId
    ? (topologyResult.value?.links || []).find(row => `${row.source}->${row.target}` === item.linkId)
    : undefined
  const node = nodeId
    ? (topologyResult.value?.nodes || []).find(row => row.id === nodeId)
    : undefined
  const instanceId = topologyNodeInstanceId(node) || topologyLinkInstanceId(link)
  await openTopologyReplication(instanceId || topologyInstanceId.value)
}

const loadReplicationReplicas = async () => {
  replicationLoading.value = true
  try {
    const res: any = await listDatabaseReplicas(replicationReplicaQuery)
    replicationReplicas.value = res.list || []
    replicationReplicaTotal.value = res.total || 0
    if (res.page) replicationReplicaQuery.page = res.page
    if (res.pageSize) replicationReplicaQuery.pageSize = res.pageSize
  } finally {
    replicationLoading.value = false
  }
}

const loadReplicationProtections = async () => {
  replicationProtectionLoading.value = true
  try {
    const res: any = await listDatabaseReplicaProtections(replicationProtectionQuery)
    replicationProtections.value = res.list || []
    replicationProtectionTotal.value = res.total || 0
    if (res.page) replicationProtectionQuery.page = res.page
    if (res.pageSize) replicationProtectionQuery.pageSize = res.pageSize
  } finally {
    replicationProtectionLoading.value = false
  }
}

const loadReplicationChecks = async () => {
  replicationCheckLoading.value = true
  try {
    const res: any = await listDatabaseReplicationChecks(replicationCheckQuery)
    replicationChecks.value = res.list || []
    replicationCheckTotal.value = res.total || 0
    if (res.page) replicationCheckQuery.page = res.page
    if (res.pageSize) replicationCheckQuery.pageSize = res.pageSize
  } finally {
    replicationCheckLoading.value = false
  }
}

const loadReplicaIncidentGuides = async () => {
  replicaIncidentGuideLoading.value = true
  try {
    const res: any = await listDatabaseReplicaIncidentGuides(replicaIncidentGuideQuery)
    replicaIncidentGuides.value = res.list || []
    replicaIncidentGuideTotal.value = res.total || 0
    if (res.page) replicaIncidentGuideQuery.page = res.page
    if (res.pageSize) replicaIncidentGuideQuery.pageSize = res.pageSize
  } finally {
    replicaIncidentGuideLoading.value = false
  }
}

const loadReplicaActions = async () => {
  replicaActionLoading.value = true
  try {
    const res: any = await listDatabaseReplicaActions(replicaActionQuery)
    replicaActions.value = res.list || []
    replicaActionTotal.value = res.total || 0
    if (res.page) replicaActionQuery.page = res.page
    if (res.pageSize) replicaActionQuery.pageSize = res.pageSize
  } finally {
    replicaActionLoading.value = false
  }
}

const refreshReplicationState = async () => {
  await Promise.all([loadReplicationProtections(), loadReplicationReplicas(), loadReplicationChecks(), loadReplicaIncidentGuides(), loadReplicaActions()])
}

const handleCheckReplication = async (instanceId?: number) => {
  if (!instanceId) return
  replicationCheckingId.value = instanceId
  try {
    await checkDatabaseReplication(instanceId)
    ElMessage.success('副本状态采集完成')
    await refreshReplicationState()
  } finally {
    replicationCheckingId.value = 0
  }
}

const handleCheckAllReplication = async () => {
  if (!replicationInstances.value.length) {
    ElMessage.warning('当前没有可采集的 MySQL / MariaDB / PostgreSQL 实例')
    return
  }
  replicationCheckLoading.value = true
  let success = 0
  let failed = 0
  try {
    for (const item of replicationInstances.value) {
      try {
        replicationCheckingId.value = item.id
        await checkDatabaseReplication(item.id)
        success += 1
      } catch {
        failed += 1
      }
    }
    await refreshReplicationState()
    if (failed > 0) {
      ElMessage.warning(`副本状态采集完成：成功 ${success}，失败 ${failed}`)
    } else {
      ElMessage.success(`副本状态采集完成：成功 ${success}`)
    }
  } finally {
    replicationCheckingId.value = 0
    replicationCheckLoading.value = false
  }
}

const handleViewReplicationStatus = async (instanceId?: number) => {
  if (!instanceId) return
  replicationCheckLoading.value = true
  try {
    replicationStatusDetail.value = await getDatabaseReplicationStatus(instanceId) as DatabaseReplicationStatusResult
    const last = replicationStatusDetail.value?.lastCheck
    replicationRawDialogTitle.value = `副本状态详情 - ${replicationStatusDetail.value?.instance?.name || instanceId}`
    replicationRawDialogContent.value = last?.rawStatusJson ? formatJSONText(last.rawStatusJson) : JSON.stringify(replicationStatusDetail.value, null, 2)
    replicationRawDialogVisible.value = true
  } finally {
    replicationCheckLoading.value = false
  }
}

const handleViewReplicationRaw = (row: DatabaseReplicationCheckResult) => {
  replicationRawDialogTitle.value = `副本状态原始采集 #${row.id}`
  replicationRawDialogContent.value = row.rawStatusJson ? formatJSONText(row.rawStatusJson) : '{}'
  replicationRawDialogVisible.value = true
}

const resetReplicaIncidentGuideForm = () => {
  replicaIncidentGuideForm.instanceId = 0
  replicaIncidentGuideForm.incidentTime = ''
  replicaIncidentGuideForm.incidentType = 'delete'
  replicaIncidentGuideForm.affectedSummary = ''
  replicaIncidentGuideForm.incidentReason = ''
  replicaIncidentGuideForm.expectedRecoveryMethod = 'export_backfill'
  replicaIncidentGuideForm.confirmNoAutoPause = false
  replicaIncidentGuideFormRef.value?.clearValidate()
}

const handleOpenReplicaIncidentGuide = (row?: DatabaseReplicaProtectionResult) => {
  resetReplicaIncidentGuideForm()
  replicaIncidentGuideForm.instanceId = row?.primaryInstanceId || replicationProtectionQuery.instanceId || 0
  replicaIncidentGuideForm.incidentTime = formatDateTimeInput()
  replicaIncidentGuideForm.affectedSummary = row?.primaryInstanceName ? `实例 ${row.primaryInstanceName} 发生误操作，影响范围待补充` : ''
  replicaIncidentGuideDialogVisible.value = true
}

const submitReplicaIncidentGuide = async () => {
  await replicaIncidentGuideFormRef.value?.validate()
  replicaIncidentGuideSubmitting.value = true
  try {
    const result = await createDatabaseReplicaIncidentGuide(replicaIncidentGuideForm) as DatabaseReplicaIncidentGuideResult
    ElMessage.success('事故指引已生成')
    replicaIncidentGuideDialogVisible.value = false
    replicaIncidentGuideDetailId.value = result.id
    replicaIncidentGuideDetailTitle.value = `事故指引 #${result.id} - ${result.instanceName || result.instanceId}`
    replicaIncidentGuideDetailMarkdown.value = result.guideMarkdown || ''
    replicaIncidentGuideDetailVisible.value = true
    await loadReplicaIncidentGuides()
  } finally {
    replicaIncidentGuideSubmitting.value = false
  }
}

const handleViewReplicaIncidentGuide = async (row: DatabaseReplicaIncidentGuideResult) => {
  replicaIncidentGuideLoading.value = true
  try {
    const result = await getDatabaseReplicaIncidentGuide(row.id) as DatabaseReplicaIncidentGuideResult
    replicaIncidentGuideDetailId.value = result.id
    replicaIncidentGuideDetailTitle.value = `事故指引 #${result.id} - ${result.instanceName || result.instanceId}`
    replicaIncidentGuideDetailMarkdown.value = result.guideMarkdown || ''
    replicaIncidentGuideDetailVisible.value = true
  } finally {
    replicaIncidentGuideLoading.value = false
  }
}

const handleDeleteReplicaIncidentGuide = async (row: DatabaseReplicaIncidentGuideResult) => {
  await ElMessageBox.confirm(
    `确定删除事故指引 #${row.id} 吗？该操作只删除指引记录，不会执行任何数据库恢复或副本动作。`,
    '删除事故指引',
    {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    }
  )
  const shouldBackPage = replicaIncidentGuides.value.length <= 1 && replicaIncidentGuideQuery.page > 1
  replicaIncidentGuideDeletingId.value = row.id
  try {
    await deleteDatabaseReplicaIncidentGuide(row.id)
    ElMessage.success('事故指引已删除')
    if (replicaIncidentGuideDetailId.value === row.id) {
      replicaIncidentGuideDetailVisible.value = false
      replicaIncidentGuideDetailId.value = 0
      replicaIncidentGuideDetailMarkdown.value = ''
    }
    if (shouldBackPage) {
      replicaIncidentGuideQuery.page -= 1
    }
    await loadReplicaIncidentGuides()
  } finally {
    replicaIncidentGuideDeletingId.value = 0
  }
}

const resetReplicaActionForm = () => {
  replicaActionForm.incidentGuideId = undefined
  replicaActionForm.incidentNo = ''
  replicaActionForm.reason = ''
  replicaActionForm.confirmImpact = ''
  replicaActionForm.confirmed = false
  replicaActionForm.maxCheckAgeSeconds = 60
  replicaActionFormRef.value?.clearValidate()
}

const handleOpenReplicaAction = (row: DatabaseInstanceReplicaResult, mode: 'pause' | 'resume') => {
  resetReplicaActionForm()
  replicaActionMode.value = mode
  replicaActionTarget.value = row
  replicaActionForm.incidentNo = `INC-${formatDateTimeInput().replace(/[-:\s]/g, '')}`
  replicaActionForm.reason = mode === 'pause' ? '误操作应急，暂停副本 apply/replay 以保留恢复窗口' : '误操作处理完成，恢复副本 apply/replay'
  replicaActionForm.confirmImpact = `目标副本：${row.replicaInstanceName || row.replicaEndpoint || row.replicaInstanceId}；当前角色：${row.replicaRoleText || row.replicaRole || '未知'}；当前 apply 状态：${row.applyStateText || row.applyState || '未知'}`
  replicaActionDialogVisible.value = true
}

const submitReplicaAction = async () => {
  await replicaActionFormRef.value?.validate()
  const target = replicaActionTarget.value
  if (!target?.id || !target.replicaInstanceId) return
  replicaActionSubmitting.value = true
  try {
    await checkDatabaseReplication(target.replicaInstanceId)
    if (replicaActionMode.value === 'pause') {
      await pauseDatabaseReplicaApply(target.id, replicaActionForm)
      ElMessage.success('已暂停副本 apply/replay')
    } else {
      await resumeDatabaseReplicaApply(target.id, replicaActionForm)
      ElMessage.success('已恢复副本 apply/replay')
    }
    replicaActionDialogVisible.value = false
    await refreshReplicationState()
  } finally {
    replicaActionSubmitting.value = false
  }
}

const loadBackupTasks = async () => {
  backupTaskLoading.value = true
  try {
    const res: any = await listDatabaseBackupTasks(backupTaskQuery)
    backupTasks.value = res.list || []
    backupTaskTotal.value = res.total || 0
    if (res.page) backupTaskQuery.page = res.page
    if (res.pageSize) backupTaskQuery.pageSize = res.pageSize
  } finally {
    backupTaskLoading.value = false
  }
}

const loadProtectionProfiles = async () => {
  protectionProfileLoading.value = true
  try {
    const res: any = await listDatabaseProtectionProfiles(protectionProfileQuery)
    protectionProfiles.value = res.list || []
    protectionProfileTotal.value = res.total || 0
    if (res.page) protectionProfileQuery.page = res.page
    if (res.pageSize) protectionProfileQuery.pageSize = res.pageSize
  } finally {
    protectionProfileLoading.value = false
  }
}

const loadProtectionRisks = async () => {
  protectionRiskLoading.value = true
  try {
    const res: any = await listDatabaseProtectionRisks(protectionRiskQuery)
    protectionRisks.value = res.list || []
    protectionRiskTotal.value = res.total || 0
    if (res.page) protectionRiskQuery.page = res.page
    if (res.pageSize) protectionRiskQuery.pageSize = res.pageSize
  } finally {
    protectionRiskLoading.value = false
  }
}

const loadBackupPolicies = async () => {
  backupPolicyLoading.value = true
  try {
    const res: any = await listDatabaseBackupPolicies(backupPolicyQuery)
    backupPolicies.value = res.list || []
    backupPolicyTotal.value = res.total || 0
    if (res.page) backupPolicyQuery.page = res.page
    if (res.pageSize) backupPolicyQuery.pageSize = res.pageSize
  } finally {
    backupPolicyLoading.value = false
  }
}

const loadBackupRecords = async () => {
  backupRecordLoading.value = true
  try {
    const res: any = await listDatabaseBackupRecords(backupRecordQuery)
    backupRecords.value = res.list || []
    backupRecordTotal.value = res.total || 0
    if (res.page) backupRecordQuery.page = res.page
    if (res.pageSize) backupRecordQuery.pageSize = res.pageSize
  } finally {
    backupRecordLoading.value = false
  }
}

const loadStorageProfiles = async () => {
  storageProfileLoading.value = true
  try {
    const res: any = await listDatabaseStorageProfiles(storageProfileQuery)
    storageProfiles.value = res.list || []
    storageProfileTotal.value = res.total || 0
    if (res.page) storageProfileQuery.page = res.page
    if (res.pageSize) storageProfileQuery.pageSize = res.pageSize
  } finally {
    storageProfileLoading.value = false
  }
}

const loadLogArchiveStreams = async () => {
  logArchiveStreamLoading.value = true
  try {
    const res: any = await listDatabaseLogArchiveStreams(logArchiveStreamQuery)
    logArchiveStreams.value = res.list || []
    logArchiveStreamTotal.value = res.total || 0
    if (res.page) logArchiveStreamQuery.page = res.page
    if (res.pageSize) logArchiveStreamQuery.pageSize = res.pageSize
  } finally {
    logArchiveStreamLoading.value = false
  }
}

const loadLogArchives = async () => {
  logArchiveLoading.value = true
  try {
    const res: any = await listDatabaseLogArchives(logArchiveQuery)
    logArchives.value = res.list || []
    logArchiveTotal.value = res.total || 0
    if (res.page) logArchiveQuery.page = res.page
    if (res.pageSize) logArchiveQuery.pageSize = res.pageSize
  } finally {
    logArchiveLoading.value = false
  }
}

const loadLogArchiveEvents = async () => {
  logArchiveEventLoading.value = true
  try {
    const res: any = await listDatabaseLogArchiveEvents(logArchiveEventQuery)
    logArchiveEvents.value = res.list || []
    logArchiveEventTotal.value = res.total || 0
    if (res.page) logArchiveEventQuery.page = res.page
    if (res.pageSize) logArchiveEventQuery.pageSize = res.pageSize
  } finally {
    logArchiveEventLoading.value = false
  }
}

const loadRestorePlans = async () => {
  restorePlanLoading.value = true
  try {
    const res: any = await listDatabaseRestorePlans(restorePlanQuery)
    restorePlans.value = res.list || []
    restorePlanTotal.value = res.total || 0
    if (res.page) restorePlanQuery.page = res.page
    if (res.pageSize) restorePlanQuery.pageSize = res.pageSize
  } finally {
    restorePlanLoading.value = false
  }
}

const loadBarmanCatalogRecords = async () => {
  barmanCatalogLoading.value = true
  try {
    const res: any = await listDatabaseBackupRecords(barmanCatalogQuery)
    barmanCatalogRecords.value = res.list || []
    barmanCatalogTotal.value = res.total || 0
    if (res.page) barmanCatalogQuery.page = res.page
    if (res.pageSize) barmanCatalogQuery.pageSize = res.pageSize
  } finally {
    barmanCatalogLoading.value = false
  }
}

const loadWalStatusArchives = async () => {
  walStatusLoading.value = true
  try {
    const res: any = await listDatabaseLogArchives({
      page: walStatusQuery.page,
      pageSize: walStatusQuery.pageSize,
      streamId: walStatusQuery.streamId,
      instanceId: walStatusQuery.instanceId,
      archiveType: 'wal'
    })
    walStatusArchives.value = res.list || []
  } finally {
    walStatusLoading.value = false
  }
}

const loadRunnerHosts = async () => {
  runnerHostLoading.value = true
  try {
    const res: any = await listDatabaseRunnerHosts(runnerHostQuery)
    runnerHosts.value = res.list || []
    runnerHostTotal.value = res.total || 0
    if (res.page) runnerHostQuery.page = res.page
    if (res.pageSize) runnerHostQuery.pageSize = res.pageSize
  } finally {
    runnerHostLoading.value = false
  }
}

const loadRunnerJobs = async () => {
  runnerJobLoading.value = true
  try {
    const res: any = await listDatabaseRunnerJobs(runnerJobQuery)
    runnerJobs.value = res.list || []
    runnerJobTotal.value = res.total || 0
    if (res.page) runnerJobQuery.page = res.page
    if (res.pageSize) runnerJobQuery.pageSize = res.pageSize
  } finally {
    runnerJobLoading.value = false
  }
}

const loadBarmanServers = async () => {
  barmanServerLoading.value = true
  try {
    const res: any = await listDatabaseBarmanServers(barmanServerQuery)
    barmanServers.value = res.list || []
    barmanServerTotal.value = res.total || 0
    if (res.page) barmanServerQuery.page = res.page
    if (res.pageSize) barmanServerQuery.pageSize = res.pageSize
  } finally {
    barmanServerLoading.value = false
  }
}

const refreshPITRState = async () => {
  await Promise.all([loadProtectionProfiles(), loadProtectionRisks(), loadStorageProfiles(), loadRunnerHosts(), loadRunnerJobs(), loadBarmanServers(), loadBackupPolicies(), loadLogArchiveStreams(), loadLogArchives(), loadLogArchiveEvents(), loadRestorePlans(), loadRestoreJobs(), loadBarmanCatalogRecords(), loadWalStatusArchives()])
}

const loadRestoreJobs = async () => {
  restoreJobLoading.value = true
  try {
    const res: any = await listDatabaseRestoreJobs(restoreJobQuery)
    restoreJobs.value = res.list || []
    restoreJobTotal.value = res.total || 0
    if (res.page) restoreJobQuery.page = res.page
    if (res.pageSize) restoreJobQuery.pageSize = res.pageSize
  } finally {
    restoreJobLoading.value = false
  }
}

const loadQueryAudits = async () => {
  auditLoading.value = true
  try {
    const res: any = await listDatabaseQueryAudits(auditQuery)
    queryAudits.value = res.list || []
    auditTotal.value = res.total || 0
    if (res.page) auditQuery.page = res.page
    if (res.pageSize) auditQuery.pageSize = res.pageSize
  } finally {
    auditLoading.value = false
  }
}

const resetBackupTaskForm = () => {
  backupTaskForm.id = undefined
  backupTaskForm.instanceId = 0
  backupTaskForm.name = ''
  backupTaskForm.backupType = 'logical'
  backupTaskForm.backupMethod = 'logical'
  backupTaskForm.backupLevel = 'full'
  backupTaskForm.backupEngine = 'logical'
  backupTaskForm.backupScope = 'database'
  backupTaskForm.scopeConfig = ''
  backupTaskForm.schedule = ''
  backupTaskForm.storageType = 'local'
  backupTaskForm.storageConfig = ''
  backupTaskForm.retentionDays = 7
  backupTaskForm.maxDurationMinutes = 1440
  backupTaskForm.compression = 'gzip'
  backupTaskForm.encryptionEnabled = false
  backupTaskForm.enabled = true
  backupTaskFormInstanceDbType.value = ''
  backupTaskFormRef.value?.clearValidate()
}

const resetBackupPolicyForm = () => {
  backupPolicyForm.id = undefined
  backupPolicyForm.instanceId = mysqlPhysicalBackupPolicyInstances.value[0]?.id || 0
  backupPolicyForm.sourceInstanceId = undefined
  backupPolicyForm.sourceRole = 'primary'
  backupPolicyForm.name = ''
  backupPolicyForm.backupEngine = defaultPhysicalBackupPolicyEngine()
  backupPolicyForm.toolExecutionMode = 'host_tools'
  backupPolicyForm.toolImage = ''
  backupPolicyForm.toolImageDigest = ''
  backupPolicyForm.containerDatadirPath = ''
  backupPolicyForm.containerWorkdirPath = ''
  backupPolicyForm.containerNetworkMode = 'host'
  backupPolicyForm.containerDatadirRo = true
  backupPolicyForm.runnerHostId = runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id || 0
  backupPolicyForm.storageProfileId = undefined
  backupPolicyForm.secretProfileId = undefined
  backupPolicyForm.binlogStreamId = undefined
  backupPolicyForm.fullSchedule = '0 2 1 * *'
  backupPolicyForm.incrementalSchedule = '0 3 * * *'
  backupPolicyForm.syntheticEnabled = false
  backupPolicyForm.syntheticRuleJson = defaultBackupPolicySyntheticRuleJson
  backupPolicyForm.restoreDrillRequired = true
  backupPolicyForm.retentionJson = '{\n  "fullKeepMonths": 6,\n  "incrementalKeepDays": 45,\n  "binlogKeepDays": 45,\n  "neverDeleteWithoutProof": true\n}'
  backupPolicyForm.enabled = true
  backupPolicyFormRef.value?.clearValidate()
}

const resetRestoreForm = () => {
  restoreSourceRecord.value = undefined
  restoreForm.targetInstanceId = 0
  restoreForm.restoreMode = 'dry_run'
  restoreForm.restoreStrategy = 'object_replace'
  restoreFormRef.value?.clearValidate()
}

const formatDateTimeInput = (date = new Date()) => {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const archiveTypeForInstance = (instanceId?: number) => {
  const dbType = instanceOptions.value.find(item => item.id === instanceId)?.dbType || ''
  if (dbType === 'postgresql') return 'wal'
  if (['mysql', 'mariadb'].includes(dbType)) return 'binlog'
  return ''
}

const resetExternalBackupForm = () => {
  externalBackupForm.instanceId = pitrBackupInstances.value[0]?.id || 0
  externalBackupForm.sourceInstanceId = undefined
  externalBackupForm.sourceRole = 'external'
  externalBackupForm.chainId = ''
  externalBackupForm.baseRecordId = undefined
  externalBackupForm.parentRecordId = undefined
  externalBackupForm.backupMethod = 'physical'
  externalBackupForm.backupLevel = 'full'
  externalBackupForm.backupEngine = 'external'
  externalBackupForm.externalBackupId = ''
  externalBackupForm.externalServerName = ''
  externalBackupForm.toolName = ''
  externalBackupForm.toolVersion = ''
  externalBackupForm.storageProfileId = undefined
  externalBackupForm.storageUri = ''
  externalBackupForm.manifestJson = ''
  externalBackupForm.prepareStatus = ''
  externalBackupForm.fileName = ''
  externalBackupForm.fileSize = undefined
  externalBackupForm.checksumSha256 = ''
  externalBackupForm.compression = ''
  externalBackupForm.encrypted = false
  externalBackupForm.recoverableFrom = ''
  externalBackupForm.recoverableUntil = ''
  externalBackupForm.startedAt = ''
  externalBackupForm.finishedAt = ''
  externalBackupForm.status = 'success'
  externalBackupForm.verifyStatus = 'success'
  externalBackupForm.serverUuid = ''
  externalBackupForm.backupBinlogFile = ''
  externalBackupForm.backupBinlogPos = undefined
  externalBackupForm.backupGtidSet = ''
  externalBackupForm.pgSystemIdentifier = ''
  externalBackupForm.timelineId = ''
  externalBackupForm.startLsn = ''
  externalBackupForm.endLsn = ''
  externalBackupForm.walStart = ''
  externalBackupForm.walEnd = ''
  externalBackupFormRef.value?.clearValidate()
}

const resetStorageProfileForm = () => {
  storageProfileForm.name = ''
  storageProfileForm.storageType = 'minio'
  storageProfileForm.endpoint = ''
  storageProfileForm.bucket = ''
  storageProfileForm.region = 'us-east-1'
  storageProfileForm.pathPrefix = ''
  storageProfileForm.secretProfileId = undefined
  storageProfileForm.versioningEnabled = true
  storageProfileForm.immutabilityEnabled = false
  storageProfileForm.kmsKeyId = ''
  storageProfileForm.retentionLockDays = 0
  storageProfileForm.status = 'enabled'
  storageProfileFormRef.value?.clearValidate()
}

const resetStoragePostureForm = () => {
  storagePostureForm.accessKey = ''
  storagePostureForm.secretKey = ''
  storagePostureForm.sessionToken = ''
  const profile = currentStoragePostureProfile.value
  const endpoint = String(profile?.endpoint || '').toLowerCase()
  storagePostureForm.useSsl = endpoint.startsWith('https://') || (!endpoint.startsWith('http://') && profile?.storageType === 's3')
  storagePostureForm.usePathStyle = profile?.storageType === 'minio' || !!profile?.endpoint
  storagePostureForm.insecureSkipVerify = false
  storagePostureFormRef.value?.clearValidate()
}

const resetLogArchiveStreamForm = () => {
  logArchiveStreamForm.instanceId = pitrBackupInstances.value[0]?.id || 0
  logArchiveStreamForm.sourceInstanceId = undefined
  logArchiveStreamForm.engine = ''
  logArchiveStreamForm.archiveType = archiveTypeForInstance(logArchiveStreamForm.instanceId)
  logArchiveStreamForm.archiveMode = 'external'
  logArchiveStreamForm.archiveEngine = logArchiveStreamForm.archiveType === 'wal' ? 'external_wal' : 'external_binlog'
  logArchiveStreamForm.runnerHostId = undefined
  logArchiveStreamForm.storageProfileId = undefined
  logArchiveStreamForm.secretProfileId = undefined
  logArchiveStreamForm.rpoTargetSeconds = 300
  logArchiveStreamForm.retentionDays = 30
  logArchiveStreamForm.enabled = true
  logArchiveStreamForm.configJson = ''
  logArchiveStreamFormRef.value?.clearValidate()
}

const resetStartLogArchiveStreamForm = () => {
  startLogArchiveStreamStream.value = undefined
  startLogArchiveStreamForm.runnerHostId = undefined
  startLogArchiveStreamForm.archiveMode = 'polling'
  startLogArchiveStreamForm.reason = ''
  startLogArchiveStreamFormRef.value?.clearValidate()
}

const resetLogArchiveForm = () => {
  logArchiveForm.streamId = logArchiveStreams.value[0]?.id || 0
  logArchiveForm.fileName = ''
  logArchiveForm.storageUri = ''
  logArchiveForm.fileSize = undefined
  logArchiveForm.checksumSha256 = ''
  logArchiveForm.firstEventTime = ''
  logArchiveForm.lastEventTime = ''
  logArchiveForm.status = 'archived'
  logArchiveForm.serverUuid = ''
  logArchiveForm.startPos = undefined
  logArchiveForm.endPos = undefined
  logArchiveForm.startGtidSet = ''
  logArchiveForm.endGtidSet = ''
  logArchiveForm.previousFileName = ''
  logArchiveForm.nextFileName = ''
  logArchiveForm.pgSystemIdentifier = ''
  logArchiveForm.timelineId = ''
  logArchiveForm.startLsn = ''
  logArchiveForm.endLsn = ''
  logArchiveForm.segmentNo = ''
  logArchiveForm.timelineHistoryUri = ''
  logArchiveFormRef.value?.clearValidate()
}

const resetRunLogArchiveOnceForm = () => {
  runLogArchiveOnceStream.value = undefined
  runLogArchiveOnceForm.runnerHostId = runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id || 0
  runLogArchiveOnceForm.fileName = ''
  runLogArchiveOnceFormRef.value?.clearValidate()
}

const resetRunLogArchiveCatchUpForm = () => {
  runLogArchiveCatchUpStream.value = undefined
  runLogArchiveCatchUpForm.runnerHostId = runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id || 0
  runLogArchiveCatchUpForm.maxFiles = 5
  runLogArchiveCatchUpForm.includeCurrent = false
  runLogArchiveCatchUpFormRef.value?.clearValidate()
}

const defaultEnabledRunnerHostId = () =>
  runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id ||
  runnerHosts.value.find(item => item.enabled !== false)?.id ||
  0

const resetProtectionWizardForm = () => {
  protectionWizardForm.instanceId = mysqlPhysicalBackupPolicyInstances.value[0]?.id || 0
  protectionWizardForm.sourceInstanceId = undefined
  protectionWizardForm.sourceRole = 'primary'
  protectionWizardForm.runnerHostId = defaultEnabledRunnerHostId()
  protectionWizardForm.storageProfileId = undefined
  protectionWizardForm.secretProfileId = undefined
  protectionWizardForm.templateKey = 'rolling_synthetic_full'
  protectionWizardForm.policyName = selectedProtectionWizardInstance.value?.name ? `${selectedProtectionWizardInstance.value.name}-物理PITR保护` : ''
  protectionWizardForm.backupEngine = defaultProtectionWizardBackupEngine()
  protectionWizardForm.toolExecutionMode = 'host_tools'
  protectionWizardForm.toolImage = ''
  protectionWizardForm.toolImageDigest = ''
  protectionWizardForm.containerDatadirPath = ''
  protectionWizardForm.containerWorkdirPath = ''
  protectionWizardForm.containerNetworkMode = 'host'
  protectionWizardForm.containerDatadirRo = true
  protectionWizardForm.reuseLogArchiveStreamId = undefined
  protectionWizardForm.reuseBackupPolicyId = undefined
  protectionWizardForm.fullSchedule = ''
  protectionWizardForm.incrementalSchedule = '0 3 * * *'
  protectionWizardForm.runInitialFullNow = true
  protectionWizardForm.binlogArchiveMode = 'polling'
  protectionWizardForm.binlogRpoTargetSeconds = 300
  protectionWizardForm.binlogRetentionDays = 45
  protectionWizardForm.syntheticEnabled = true
  protectionWizardForm.syntheticAutoRun = true
  protectionWizardForm.syntheticTriggerAfterIncrementals = 5
  protectionWizardForm.syntheticMergeOldestIncrementals = 5
  protectionWizardForm.syntheticRequireRestoreProof = true
  protectionWizardForm.syntheticNeverDeleteWithoutProof = true
  protectionWizardForm.syntheticMarkSupersededAfterProof = true
  protectionWizardForm.syntheticSupersededKeepDays = 7
  protectionWizardForm.restoreDrillRequired = true
  protectionWizardForm.retentionFullKeepMonths = 6
  protectionWizardForm.retentionIncrementalKeepDays = 45
  protectionWizardForm.retentionBinlogKeepDays = 45
  protectionWizardForm.retentionNeverDeleteWithoutProof = true
  protectionWizardForm.archiveConfigJson = ''
  protectionWizardPreview.value = undefined
  protectionWizardFormRef.value?.clearValidate()
}

const defaultPostgresBarmanServerName = (instanceId?: number) => {
  const instance = instanceOptions.value.find(item => item.id === instanceId)
  const name = String(instance?.name || 'postgres').toLowerCase().replace(/[^a-z0-9_.:-]+/g, '_').replace(/^[_ .:-]+|[_ .:-]+$/g, '')
  return `pg_${instanceId || 'server'}_${name || 'postgres'}`.slice(0, 120)
}

const resetPostgresBarmanWizardForm = () => {
  const firstInstanceId = postgresqlBackupInstances.value[0]?.id || 0
  postgresBarmanWizardForm.instanceId = firstInstanceId
  postgresBarmanWizardForm.runnerHostId = defaultEnabledRunnerHostId()
  postgresBarmanWizardForm.reuseBarmanServerId = undefined
  postgresBarmanWizardForm.name = firstInstanceId ? `${postgresqlBackupInstances.value.find(item => item.id === firstInstanceId)?.name || 'PostgreSQL'}-Barman-PITR保护` : ''
  postgresBarmanWizardForm.barmanServerName = defaultPostgresBarmanServerName(firstInstanceId)
  postgresBarmanWizardForm.barmanHome = ''
  postgresBarmanWizardForm.configPath = ''
  postgresBarmanWizardForm.retentionPolicy = 'RECOVERY WINDOW OF 30 DAYS'
  postgresBarmanWizardForm.backupMethod = 'postgres'
  postgresBarmanWizardForm.streamingArchiverEnabled = true
  postgresBarmanWizardForm.archiverEnabled = true
  postgresBarmanWizardForm.slotName = ''
  postgresBarmanWizardForm.configJson = ''
  postgresBarmanWizardForm.runCheckNow = true
  postgresBarmanWizardForm.syncCatalogNow = true
  postgresBarmanWizardForm.syncWalNow = true
  postgresBarmanWizardForm.runInitialBackupNow = false
  postgresBarmanWizardPreview.value = undefined
  postgresBarmanWizardFormRef.value?.clearValidate()
}

const resetProtectionRestoreDrillForm = () => {
  protectionRestoreDrillProfile.value = undefined
  protectionRestoreDrillResult.value = undefined
  protectionRestoreDrillForm.restoreTargetType = 'time'
  protectionRestoreDrillForm.restoreTargetValue = formatDateTimeInput()
  protectionRestoreDrillForm.targetTimelineId = ''
  protectionRestoreDrillForm.restoreTargetInclusive = true
  protectionRestoreDrillForm.runnerHostId = defaultEnabledRunnerHostId() || undefined
  protectionRestoreDrillForm.containerImage = ''
  protectionRestoreDrillForm.listenPort = undefined
  protectionRestoreDrillForm.expiresInHours = 24
  protectionRestoreDrillForm.validationSql = []
  protectionRestoreDrillForm.validationAssertions = []
  protectionRestoreDrillForm.validationSqlText = ''
  protectionRestoreDrillForm.cleanupOnFailure = false
  protectionRestoreDrillForm.postgresStartInstance = true
  protectionRestoreDrillForm.targetAction = 'pause'
  protectionRestoreDrillForm.barmanGetWal = true
  protectionRestoreDrillForm.confirmIsolated = false
  protectionRestoreDrillFormRef.value?.clearValidate()
}

const resetRestorePlanForm = () => {
  restorePlanForm.sourceInstanceId = pitrBackupInstances.value[0]?.id || 0
  restorePlanForm.targetInstanceId = pitrRestoreTargetInstances.value.find(item => item.id !== restorePlanForm.sourceInstanceId)?.id
  restorePlanForm.baseRecordId = undefined
  restorePlanForm.restoreMode = 'isolated_restore'
  restorePlanForm.restoreTargetType = 'time'
  restorePlanForm.restoreTargetValue = formatDateTimeInput()
  restorePlanForm.targetTimelineId = ''
  restorePlanForm.restoreTargetInclusive = true
  restorePlanBaseRecordLabel.value = ''
  restorePlanFormRef.value?.clearValidate()
}

const resetRunnerHostForm = () => {
  runnerHostForm.id = undefined
  runnerHostForm.name = ''
  runnerHostForm.runnerType = 'ssh'
  runnerHostForm.host = ''
  runnerHostForm.port = 22
  runnerHostForm.credentialId = sshCredentialOptions.value[0]?.id
  runnerHostForm.workDir = '/var/lib/opshub/database-runner'
  runnerHostForm.storageMountPath = ''
  runnerHostForm.maxConcurrentJobs = 1
  runnerHostForm.cpuLimit = ''
  runnerHostForm.ioLimit = ''
  runnerHostForm.bandwidthLimit = ''
  runnerHostForm.timeoutMinutes = 30
  runnerHostForm.enabled = true
  runnerHostForm.configJson = ''
  runnerHostFormRef.value?.clearValidate()
}

const resetBarmanServerForm = () => {
  barmanServerForm.id = undefined
  barmanServerForm.sourceInstanceId = postgresqlBackupInstances.value[0]?.id || 0
  barmanServerForm.runnerHostId = runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id || 0
  barmanServerForm.name = ''
  barmanServerForm.barmanServerName = ''
  barmanServerForm.barmanHome = ''
  barmanServerForm.configPath = ''
  barmanServerForm.retentionPolicy = ''
  barmanServerForm.backupMethod = ''
  barmanServerForm.streamingArchiverEnabled = false
  barmanServerForm.archiverEnabled = false
  barmanServerForm.slotName = ''
  barmanServerForm.status = 'pending'
  barmanServerForm.configJson = ''
  barmanServerFormRef.value?.clearValidate()
}

const normalizeRestoreFormStrategy = () => {
  const options = availableRestoreStrategyOptions.value
  if (!options.some(item => item.value === restoreForm.restoreStrategy)) {
    restoreForm.restoreStrategy = options[0]?.value || 'object_replace'
  }
}

const openBackupTaskDialog = (row?: DatabaseBackupTaskResult) => {
  resetBackupTaskForm()
  if (row?.id) {
    backupTaskForm.id = row.id
    backupTaskForm.instanceId = row.instanceId
    backupTaskForm.name = row.name || ''
    backupTaskForm.backupType = row.backupType || 'logical'
    backupTaskForm.backupMethod = row.backupMethod || 'logical'
    backupTaskForm.backupLevel = row.backupLevel || 'full'
    backupTaskForm.backupEngine = row.backupEngine || 'logical'
    backupTaskForm.backupScope = row.backupScope || 'database'
    backupTaskForm.scopeConfig = row.scopeConfig || ''
    backupTaskFormInstanceDbType.value = row.instanceDbType || ''
    backupTaskForm.schedule = row.schedule || ''
    backupTaskForm.storageType = row.storageType || 'local'
    backupTaskForm.storageConfig = row.storageConfig || ''
    backupTaskForm.retentionDays = row.retentionDays || 7
    backupTaskForm.maxDurationMinutes = row.maxDurationMinutes || 1440
    backupTaskForm.compression = row.compression || 'gzip'
    backupTaskForm.encryptionEnabled = !!row.encryptionEnabled
    backupTaskForm.enabled = !!row.enabled
  } else if (supportedBackupInstances.value.length > 0) {
    backupTaskForm.instanceId = supportedBackupInstances.value[0].id
    backupTaskFormInstanceDbType.value = supportedBackupInstances.value[0].dbType || ''
  }
  normalizeBackupTaskFormBackupType()
  backupTaskDialogVisible.value = true
}

const submitBackupTaskForm = async () => {
  if (!backupTaskFormRef.value) return
  await backupTaskFormRef.value.validate()
  normalizeBackupTaskFormBackupType()
  backupTaskSubmitting.value = true
  try {
    const payload: DatabaseBackupTaskPayload = {
      instanceId: backupTaskForm.instanceId,
      name: backupTaskForm.name.trim(),
      backupType: backupTaskForm.backupType || 'logical',
      backupMethod: backupTaskForm.backupMethod || 'logical',
      backupLevel: backupTaskForm.backupLevel || 'full',
      backupEngine: backupTaskForm.backupEngine || 'logical',
      backupScope: backupTaskForm.backupScope || 'database',
      scopeConfig: backupTaskForm.scopeConfig?.trim(),
      schedule: (backupTaskForm.schedule || '').trim(),
      storageType: backupTaskForm.storageType || 'local',
      storageConfig: backupTaskForm.storageConfig?.trim(),
      retentionDays: backupTaskForm.retentionDays,
      maxDurationMinutes: backupTaskForm.maxDurationMinutes,
      compression: backupTaskForm.compression,
      encryptionEnabled: backupTaskForm.encryptionEnabled,
      enabled: backupTaskForm.enabled
    }
    if (backupTaskForm.id) {
      await updateDatabaseBackupTask(backupTaskForm.id, payload)
      ElMessage.success('备份任务已更新')
    } else {
      await createDatabaseBackupTask(payload)
      ElMessage.success('备份任务已创建')
    }
    backupTaskDialogVisible.value = false
    await Promise.all([loadBackupTasks(), loadBackupRecords()])
  } finally {
    backupTaskSubmitting.value = false
  }
}

const openBackupPolicyDialog = async (row?: DatabaseBackupPolicyResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!logArchiveStreams.value.length) {
    await loadLogArchiveStreams()
  }
  resetBackupPolicyForm()
  if (row?.id) {
    backupPolicyForm.id = row.id
    backupPolicyForm.instanceId = row.instanceId
    backupPolicyForm.sourceInstanceId = row.sourceInstanceId || undefined
    backupPolicyForm.sourceRole = row.sourceRole || 'primary'
    backupPolicyForm.name = row.name || ''
    backupPolicyForm.backupEngine = row.backupEngine || defaultPhysicalBackupPolicyEngine()
    backupPolicyForm.toolExecutionMode = row.toolExecutionMode || 'host_tools'
    backupPolicyForm.toolImage = row.toolImage || ''
    backupPolicyForm.toolImageDigest = row.toolImageDigest || ''
    backupPolicyForm.containerDatadirPath = row.containerDatadirPath || ''
    backupPolicyForm.containerWorkdirPath = row.containerWorkdirPath || ''
    backupPolicyForm.containerNetworkMode = row.containerNetworkMode || 'host'
    backupPolicyForm.containerDatadirRo = row.containerDatadirRo !== false
    backupPolicyForm.runnerHostId = row.runnerHostId || 0
    backupPolicyForm.storageProfileId = row.storageProfileId || undefined
    backupPolicyForm.secretProfileId = row.secretProfileId || undefined
    backupPolicyForm.binlogStreamId = row.binlogStreamId || undefined
    backupPolicyForm.fullSchedule = row.fullSchedule || ''
    backupPolicyForm.incrementalSchedule = row.incrementalSchedule || ''
    backupPolicyForm.syntheticEnabled = !!row.syntheticEnabled
    backupPolicyForm.syntheticRuleJson = row.syntheticRuleJson || ''
    backupPolicyForm.restoreDrillRequired = !!row.restoreDrillRequired
    backupPolicyForm.retentionJson = row.retentionJson || ''
    backupPolicyForm.enabled = !!row.enabled
  } else if (!backupPolicyForm.name && selectedBackupPolicyInstance.value?.name) {
    backupPolicyForm.name = `${selectedBackupPolicyInstance.value.name}-物理增量策略`
  }
  backupPolicyDialogVisible.value = true
}

const submitBackupPolicyForm = async () => {
  if (!backupPolicyFormRef.value) return
  await backupPolicyFormRef.value.validate()
  backupPolicySubmitting.value = true
  try {
    const payload: DatabaseBackupPolicyPayload = {
      instanceId: backupPolicyForm.instanceId,
      sourceInstanceId: backupPolicyForm.sourceInstanceId || undefined,
      sourceRole: backupPolicyForm.sourceRole || 'primary',
      name: backupPolicyForm.name.trim(),
      backupEngine: backupPolicyForm.backupEngine || defaultPhysicalBackupPolicyEngine(),
      toolExecutionMode: backupPolicyForm.toolExecutionMode || 'host_tools',
      toolImage: backupPolicyForm.toolImage?.trim() || '',
      toolImageDigest: backupPolicyForm.toolImageDigest?.trim() || '',
      containerDatadirPath: backupPolicyForm.containerDatadirPath?.trim() || '',
      containerWorkdirPath: backupPolicyForm.containerWorkdirPath?.trim() || '',
      containerNetworkMode: backupPolicyForm.containerNetworkMode?.trim() || 'host',
      containerDatadirRo: backupPolicyForm.containerDatadirRo !== false,
      runnerHostId: backupPolicyForm.runnerHostId,
      storageProfileId: backupPolicyForm.storageProfileId || undefined,
      secretProfileId: backupPolicyForm.secretProfileId || undefined,
      binlogStreamId: backupPolicyForm.binlogStreamId || undefined,
      fullSchedule: (backupPolicyForm.fullSchedule || '').trim(),
      incrementalSchedule: (backupPolicyForm.incrementalSchedule || '').trim(),
      syntheticEnabled: backupPolicyForm.syntheticEnabled === true,
      syntheticRuleJson: (backupPolicyForm.syntheticRuleJson || '').trim(),
      restoreDrillRequired: backupPolicyForm.restoreDrillRequired === true,
      retentionJson: (backupPolicyForm.retentionJson || '').trim(),
      enabled: backupPolicyForm.enabled !== false
    }
    if (backupPolicyForm.id) {
      await updateDatabaseBackupPolicy(backupPolicyForm.id, payload)
      ElMessage.success('备份策略已更新')
    } else {
      await createDatabaseBackupPolicy(payload)
      ElMessage.success('备份策略已创建')
    }
    backupPolicyDialogVisible.value = false
    await Promise.all([loadProtectionProfiles(), loadBackupPolicies()])
  } finally {
    backupPolicySubmitting.value = false
  }
}

const openExternalBackupDialog = () => {
  resetExternalBackupForm()
  externalBackupDialogVisible.value = true
}

const submitExternalBackup = async () => {
  if (!externalBackupFormRef.value) return
  await externalBackupFormRef.value.validate()
  externalBackupSubmitting.value = true
  try {
    const payload: DatabaseExternalBackupRecordPayload = {
      ...externalBackupForm,
      sourceInstanceId: externalBackupForm.sourceInstanceId || undefined,
      baseRecordId: externalBackupForm.baseRecordId || undefined,
      parentRecordId: externalBackupForm.parentRecordId || undefined,
      storageProfileId: externalBackupForm.storageProfileId || undefined,
      fileSize: externalBackupForm.fileSize || undefined,
      backupBinlogPos: externalBackupForm.backupBinlogPos || undefined,
      status: externalBackupForm.status || 'success',
      verifyStatus: externalBackupForm.verifyStatus || 'success'
    }
    await registerExternalDatabaseBackupRecord(payload)
    externalBackupDialogVisible.value = false
    ElMessage.success('外部备份记录已登记')
    await Promise.all([loadBackupRecords(), loadBackupTasks()])
  } finally {
    externalBackupSubmitting.value = false
  }
}

const openStorageProfileDialog = () => {
  resetStorageProfileForm()
  storageProfileDialogVisible.value = true
}

const submitStorageProfile = async () => {
  if (!storageProfileFormRef.value) return
  await storageProfileFormRef.value.validate()
  storageProfileSubmitting.value = true
  try {
    await createDatabaseStorageProfile({
      ...storageProfileForm,
      name: storageProfileForm.name.trim(),
      endpoint: storageProfileForm.endpoint?.trim(),
      bucket: storageProfileForm.bucket?.trim(),
      region: storageProfileForm.region?.trim(),
      pathPrefix: storageProfileForm.pathPrefix?.trim(),
      kmsKeyId: storageProfileForm.kmsKeyId?.trim(),
      secretProfileId: storageProfileForm.secretProfileId || undefined
    })
    storageProfileDialogVisible.value = false
    ElMessage.success('存储配置已创建')
    await loadStorageProfiles()
  } finally {
    storageProfileSubmitting.value = false
  }
}

const openStoragePostureDialog = async (row: DatabaseStorageProfileResult) => {
  currentStoragePostureProfile.value = row
  if (!isObjectStorageProfile(row)) {
    storagePostureSubmitting.value = true
    try {
      const result: any = await checkDatabaseStorageProfilePosture(row.id, {})
      currentStoragePostureDetail.value = result
      storagePostureDetailVisible.value = true
      ElMessage.warning(result?.postureSummary || '该存储类型暂不支持自动检测')
      await loadStorageProfiles()
    } finally {
      storagePostureSubmitting.value = false
    }
    return
  }
  resetStoragePostureForm()
  storagePostureDialogVisible.value = true
}

const submitStoragePostureCheck = async () => {
  if (!storagePostureFormRef.value || !currentStoragePostureProfile.value?.id) return
  await storagePostureFormRef.value.validate()
  storagePostureSubmitting.value = true
  try {
    const result: any = await checkDatabaseStorageProfilePosture(currentStoragePostureProfile.value.id, {
      accessKey: storagePostureForm.accessKey?.trim(),
      secretKey: storagePostureForm.secretKey,
      sessionToken: storagePostureForm.sessionToken?.trim(),
      useSsl: storagePostureForm.useSsl,
      usePathStyle: storagePostureForm.usePathStyle,
      insecureSkipVerify: storagePostureForm.insecureSkipVerify
    })
    storagePostureDialogVisible.value = false
    currentStoragePostureDetail.value = result
    storagePostureDetailVisible.value = true
    ElMessage.success(result?.postureSummary || '对象存储安全姿态检测完成')
    await loadStorageProfiles()
  } finally {
    storagePostureSubmitting.value = false
  }
}

const openStoragePostureDetail = (row: DatabaseStorageProfileResult) => {
  currentStoragePostureDetail.value = row
  storagePostureDetailVisible.value = true
}

const openLogArchiveStreamDialog = () => {
  resetLogArchiveStreamForm()
  logArchiveStreamDialogVisible.value = true
}

const submitLogArchiveStream = async () => {
  if (!logArchiveStreamFormRef.value) return
  await logArchiveStreamFormRef.value.validate()
  logArchiveStreamSubmitting.value = true
  try {
    const payload: DatabaseLogArchiveStreamPayload = {
      ...logArchiveStreamForm,
      sourceInstanceId: logArchiveStreamForm.sourceInstanceId || undefined,
      runnerHostId: logArchiveStreamForm.runnerHostId || undefined,
      storageProfileId: logArchiveStreamForm.storageProfileId || undefined,
      secretProfileId: logArchiveStreamForm.secretProfileId || undefined,
      archiveType: logArchiveStreamForm.archiveType || archiveTypeForInstance(logArchiveStreamForm.instanceId)
    }
    await createDatabaseLogArchiveStream(payload)
    logArchiveStreamDialogVisible.value = false
    ElMessage.success('日志归档流已创建')
    await loadLogArchiveStreams()
  } finally {
    logArchiveStreamSubmitting.value = false
  }
}

const openStartLogArchiveStreamDialog = async (row: DatabaseLogArchiveStreamResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!runnerHosts.value.some(item => item.enabled)) {
    ElMessage.warning('请先配置并启用 Runner 主机')
    return
  }
  resetStartLogArchiveStreamForm()
  startLogArchiveStreamStream.value = row
  startLogArchiveStreamForm.runnerHostId = row.runnerHostId || runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id
  startLogArchiveStreamForm.archiveMode = ['polling', 'streaming'].includes(row.archiveMode) ? row.archiveMode : 'polling'
  startLogArchiveStreamDialogVisible.value = true
}

const submitStartLogArchiveStream = async () => {
  if (!startLogArchiveStreamFormRef.value || !startLogArchiveStreamStream.value?.id) return
  await startLogArchiveStreamFormRef.value.validate()
  startLogArchiveStreamSubmitting.value = true
  try {
    await startDatabaseLogArchiveStream(startLogArchiveStreamStream.value.id, {
      runnerHostId: startLogArchiveStreamForm.runnerHostId,
      archiveMode: startLogArchiveStreamForm.archiveMode
    })
    startLogArchiveStreamDialogVisible.value = false
    ElMessage.success('归档流启动指令已保存，等待 Runner Agent 接管')
    await loadLogArchiveStreams()
  } finally {
    startLogArchiveStreamSubmitting.value = false
  }
}

const pauseLogArchiveStream = async (row: DatabaseLogArchiveStreamResult) => {
  try {
    await ElMessageBox.confirm(`确定暂停归档流「${row.instanceName || `#${row.instanceId}`}」吗？`, '暂停归档流确认', { type: 'warning' })
  } catch (_err) {
    return
  }
  await pauseDatabaseLogArchiveStream(row.id, { reason: '手动暂停' })
  ElMessage.success('归档流已标记暂停')
  await loadLogArchiveStreams()
}

const resumeLogArchiveStream = async (row: DatabaseLogArchiveStreamResult) => {
  if (!row.runnerHostId && !runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  const runnerHostId = row.runnerHostId || runnerHosts.value.find(item => item.enabled && item.status === 'online')?.id || runnerHosts.value.find(item => item.enabled)?.id
  if (!runnerHostId) {
    ElMessage.warning('请先配置并启用 Runner 主机')
    return
  }
  await resumeDatabaseLogArchiveStream(row.id, {
    runnerHostId,
    archiveMode: ['polling', 'streaming'].includes(row.archiveMode) ? row.archiveMode : 'polling'
  })
  ElMessage.success('归档流恢复指令已保存，等待 Runner Agent 接管')
  await loadLogArchiveStreams()
}

const stopLogArchiveStream = async (row: DatabaseLogArchiveStreamResult) => {
  try {
    await ElMessageBox.confirm(`确定停止归档流「${row.instanceName || `#${row.instanceId}`}」吗？`, '停止归档流确认', { type: 'warning' })
  } catch (_err) {
    return
  }
  await stopDatabaseLogArchiveStream(row.id, { reason: '手动停止' })
  ElMessage.success('归档流已停止')
  await loadLogArchiveStreams()
}

const openLogArchiveDialog = async () => {
  if (!logArchiveStreams.value.length) {
    await loadLogArchiveStreams()
  }
  if (!logArchiveStreams.value.length) {
    ElMessage.warning('请先创建日志归档流')
    return
  }
  resetLogArchiveForm()
  logArchiveDialogVisible.value = true
}

const submitLogArchive = async () => {
  if (!logArchiveFormRef.value) return
  await logArchiveFormRef.value.validate()
  logArchiveSubmitting.value = true
  try {
    const payload: DatabaseExternalLogArchivePayload = {
      ...logArchiveForm,
      fileSize: logArchiveForm.fileSize || undefined,
      startPos: logArchiveForm.startPos || undefined,
      endPos: logArchiveForm.endPos || undefined,
      status: logArchiveForm.status || 'archived'
    }
    await registerExternalDatabaseLogArchive(payload)
    logArchiveDialogVisible.value = false
    ElMessage.success('日志归档文件已登记')
    await Promise.all([loadLogArchives(), loadLogArchiveStreams()])
  } finally {
    logArchiveSubmitting.value = false
  }
}

const openRunLogArchiveOnceDialog = async (row: DatabaseLogArchiveStreamResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!runnerHosts.value.some(item => item.enabled)) {
    ElMessage.warning('请先配置并启用 Runner 主机')
    return
  }
  resetRunLogArchiveOnceForm()
  runLogArchiveOnceStream.value = row
  runLogArchiveOnceDialogVisible.value = true
}

const submitRunLogArchiveOnce = async () => {
  if (!runLogArchiveOnceFormRef.value || !runLogArchiveOnceStream.value?.id) return
  await runLogArchiveOnceFormRef.value.validate()
  try {
    await ElMessageBox.confirm(
      `确定通过 Runner 拉取归档流「${runLogArchiveOnceStream.value.instanceName || `#${runLogArchiveOnceStream.value.instanceId}`}」的一份 binlog 吗？`,
      '一次性 binlog 归档确认',
      { type: 'warning' }
    )
  } catch (_err) {
    return
  }
  runLogArchiveOnceSubmitting.value = true
  try {
    const payload: DatabaseRunLogArchiveOncePayload = {
      runnerHostId: runLogArchiveOnceForm.runnerHostId,
      fileName: runLogArchiveOnceForm.fileName?.trim() || undefined
    }
    await runDatabaseLogArchiveOnce(runLogArchiveOnceStream.value.id, payload)
    runLogArchiveOnceDialogVisible.value = false
    ElMessage.success('binlog 归档任务已下发')
    await Promise.all([loadRunnerJobs(), loadLogArchiveStreams(), loadLogArchives()])
    window.setTimeout(() => {
      loadRunnerJobs()
      loadLogArchiveStreams()
      loadLogArchives()
    }, 3000)
  } finally {
    runLogArchiveOnceSubmitting.value = false
  }
}

const openRunLogArchiveCatchUpDialog = async (row: DatabaseLogArchiveStreamResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!runnerHosts.value.some(item => item.enabled)) {
    ElMessage.warning('请先配置并启用 Runner 主机')
    return
  }
  resetRunLogArchiveCatchUpForm()
  runLogArchiveCatchUpStream.value = row
  runLogArchiveCatchUpDialogVisible.value = true
}

const submitRunLogArchiveCatchUp = async () => {
  if (!runLogArchiveCatchUpFormRef.value || !runLogArchiveCatchUpStream.value?.id) return
  await runLogArchiveCatchUpFormRef.value.validate()
  try {
    await ElMessageBox.confirm(
      `确定通过 Runner 追平归档流「${runLogArchiveCatchUpStream.value.instanceName || `#${runLogArchiveCatchUpStream.value.instanceId}`}」的 binlog 吗？默认不会拉取当前活跃 binlog。`,
      'binlog 追平归档确认',
      { type: 'warning' }
    )
  } catch (_err) {
    return
  }
  runLogArchiveCatchUpSubmitting.value = true
  try {
    const payload: DatabaseRunLogArchiveCatchUpPayload = {
      runnerHostId: runLogArchiveCatchUpForm.runnerHostId,
      maxFiles: runLogArchiveCatchUpForm.maxFiles || 5,
      includeCurrent: !!runLogArchiveCatchUpForm.includeCurrent
    }
    await runDatabaseLogArchiveCatchUp(runLogArchiveCatchUpStream.value.id, payload)
    runLogArchiveCatchUpDialogVisible.value = false
    ElMessage.success('binlog 追平归档任务已下发')
    await Promise.all([loadRunnerJobs(), loadLogArchiveStreams(), loadLogArchives()])
    window.setTimeout(() => {
      loadRunnerJobs()
      loadLogArchiveStreams()
      loadLogArchives()
    }, 3000)
  } finally {
    runLogArchiveCatchUpSubmitting.value = false
  }
}

const defaultRestoreImageForInstance = (instance?: { dbType?: string; version?: string }) => {
  const version = String(instance?.version || '')
  if (instance?.dbType === 'postgresql') {
    const match = version.match(/(\d+)/)
    return match ? `postgres:${match[1]}` : 'postgres:latest'
  }
  if (instance?.dbType === 'mariadb') {
    const versionTag = version.split('-')[0]?.split(' ')[0] || ''
    return versionTag ? `mariadb:${versionTag}` : 'mariadb:latest'
  }
  if (/8\.4/.test(version)) return 'mysql:8.4'
  if (/5\.7/.test(version)) return 'mysql:5.7'
  return 'mysql:8.0'
}

const parseProtectionWizardRule = (raw?: string) => {
  if (!raw || !raw.trim()) return
  try {
    const parsed = JSON.parse(raw)
    protectionWizardForm.syntheticAutoRun = parsed.autoRun !== false
    protectionWizardForm.syntheticTriggerAfterIncrementals = Number(parsed.triggerAfterIncrementals || 5)
    protectionWizardForm.syntheticMergeOldestIncrementals = Number(parsed.mergeOldestIncrementals || protectionWizardForm.syntheticTriggerAfterIncrementals || 5)
    protectionWizardForm.syntheticRequireRestoreProof = parsed.requireRestoreProof !== false
    protectionWizardForm.syntheticNeverDeleteWithoutProof = parsed.neverDeleteWithoutProof !== false
    protectionWizardForm.syntheticMarkSupersededAfterProof = parsed.markSupersededAfterProof !== false
    protectionWizardForm.syntheticSupersededKeepDays = Number(parsed.supersededKeepDaysAfterProof || parsed.supersededKeepDays || 7)
  } catch (_err) {
    // 保留向导默认值，后端预览会重新生成安全 JSON。
  }
}

const parseProtectionWizardRetention = (raw?: string) => {
  if (!raw || !raw.trim()) return
  try {
    const parsed = JSON.parse(raw)
    protectionWizardForm.retentionFullKeepMonths = Number(parsed.fullKeepMonths || 6)
    protectionWizardForm.retentionIncrementalKeepDays = Number(parsed.incrementalKeepDays || 45)
    protectionWizardForm.retentionBinlogKeepDays = Number(parsed.binlogKeepDays || 45)
    protectionWizardForm.retentionNeverDeleteWithoutProof = parsed.neverDeleteWithoutProof !== false
  } catch (_err) {
    // 保留向导默认值，后端预览会重新生成安全 JSON。
  }
}

const handleProtectionWizardInstanceChange = () => {
  protectionWizardForm.sourceInstanceId = undefined
  protectionWizardForm.reuseLogArchiveStreamId = undefined
  protectionWizardForm.reuseBackupPolicyId = undefined
  protectionWizardForm.backupEngine = defaultProtectionWizardBackupEngine()
  protectionWizardForm.policyName = selectedProtectionWizardInstance.value?.name ? `${selectedProtectionWizardInstance.value.name}-物理PITR保护` : ''
  protectionWizardPreview.value = undefined
}

const openProtectionWizardDialog = async (row?: DatabaseProtectionProfileResult) => {
  if (row?.engine === 'postgresql') {
    await openPostgresBarmanWizardDialog(row)
    return
  }
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!storageProfiles.value.length) {
    await loadStorageProfiles()
  }
  if (!logArchiveStreams.value.length) {
    await loadLogArchiveStreams()
  }
  resetProtectionWizardForm()
  if (row?.instanceId) {
    const policy = row.backupPolicy
    const stream = row.logArchiveStream
    protectionWizardForm.instanceId = row.instanceId
    protectionWizardForm.sourceInstanceId = policy?.sourceInstanceId || stream?.sourceInstanceId || undefined
    protectionWizardForm.sourceRole = policy?.sourceRole || 'primary'
    protectionWizardForm.runnerHostId = policy?.runnerHostId || stream?.runnerHostId || row.runnerHost?.id || defaultEnabledRunnerHostId()
    protectionWizardForm.storageProfileId = policy?.storageProfileId || stream?.storageProfileId || row.storageProfile?.id || undefined
    protectionWizardForm.secretProfileId = policy?.secretProfileId || stream?.secretProfileId || undefined
    protectionWizardForm.policyName = policy?.name || `${row.instanceName}-物理PITR保护`
    protectionWizardForm.backupEngine = policy?.backupEngine || defaultProtectionWizardBackupEngine()
    protectionWizardForm.toolExecutionMode = policy?.toolExecutionMode || 'host_tools'
    protectionWizardForm.toolImage = policy?.toolImage || ''
    protectionWizardForm.toolImageDigest = policy?.toolImageDigest || ''
    protectionWizardForm.containerDatadirPath = policy?.containerDatadirPath || ''
    protectionWizardForm.containerWorkdirPath = policy?.containerWorkdirPath || ''
    protectionWizardForm.containerNetworkMode = policy?.containerNetworkMode || 'host'
    protectionWizardForm.containerDatadirRo = policy?.containerDatadirRo !== false
    protectionWizardForm.reuseLogArchiveStreamId = stream?.id || policy?.binlogStreamId || undefined
    protectionWizardForm.reuseBackupPolicyId = policy?.id || undefined
    protectionWizardForm.fullSchedule = policy?.fullSchedule || ''
    protectionWizardForm.incrementalSchedule = policy?.incrementalSchedule || '0 3 * * *'
    protectionWizardForm.binlogArchiveMode = stream?.archiveMode && stream.archiveMode !== 'external' ? stream.archiveMode : 'polling'
    protectionWizardForm.binlogRpoTargetSeconds = stream?.rpoTargetSeconds || 300
    protectionWizardForm.binlogRetentionDays = stream?.retentionDays || 45
    protectionWizardForm.syntheticEnabled = policy?.syntheticEnabled !== false
    protectionWizardForm.restoreDrillRequired = policy?.restoreDrillRequired !== false
    parseProtectionWizardRule(policy?.syntheticRuleJson)
    parseProtectionWizardRetention(policy?.retentionJson)
  }
  protectionWizardDialogVisible.value = true
}

const handlePostgresBarmanWizardInstanceChange = () => {
  const instance = postgresqlBackupInstances.value.find(item => item.id === postgresBarmanWizardForm.instanceId)
  postgresBarmanWizardForm.reuseBarmanServerId = undefined
  postgresBarmanWizardForm.name = instance?.name ? `${instance.name}-Barman-PITR保护` : ''
  postgresBarmanWizardForm.barmanServerName = defaultPostgresBarmanServerName(postgresBarmanWizardForm.instanceId)
  postgresBarmanWizardPreview.value = undefined
}

const openPostgresBarmanWizardDialog = async (row?: DatabaseProtectionProfileResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  if (!barmanServers.value.length) {
    await loadBarmanServers()
  }
  resetPostgresBarmanWizardForm()
  if (row?.instanceId) {
    const server = row.barmanServer || barmanServers.value.find(item => item.sourceInstanceId === row.instanceId)
    postgresBarmanWizardForm.instanceId = row.instanceId
    postgresBarmanWizardForm.runnerHostId = server?.runnerHostId || row.runnerHost?.id || defaultEnabledRunnerHostId()
    postgresBarmanWizardForm.reuseBarmanServerId = server?.id || undefined
    postgresBarmanWizardForm.name = server?.name || `${row.instanceName}-Barman-PITR保护`
    postgresBarmanWizardForm.barmanServerName = server?.barmanServerName || defaultPostgresBarmanServerName(row.instanceId)
    postgresBarmanWizardForm.barmanHome = server?.barmanHome || ''
    postgresBarmanWizardForm.configPath = server?.configPath || ''
    postgresBarmanWizardForm.retentionPolicy = server?.retentionPolicy || 'RECOVERY WINDOW OF 30 DAYS'
    postgresBarmanWizardForm.backupMethod = server?.backupMethod || 'postgres'
    postgresBarmanWizardForm.streamingArchiverEnabled = server?.streamingArchiverEnabled !== false
    postgresBarmanWizardForm.archiverEnabled = server?.archiverEnabled !== false
    postgresBarmanWizardForm.slotName = server?.slotName || ''
    postgresBarmanWizardForm.configJson = server?.configJson || ''
  }
  postgresBarmanWizardDialogVisible.value = true
}

const buildPostgresBarmanWizardPayload = (): DatabasePostgresBarmanPITRWizardPayload => ({
  instanceId: postgresBarmanWizardForm.instanceId,
  runnerHostId: postgresBarmanWizardForm.runnerHostId,
  reuseBarmanServerId: postgresBarmanWizardForm.reuseBarmanServerId || undefined,
  name: postgresBarmanWizardForm.name?.trim() || undefined,
  barmanServerName: postgresBarmanWizardForm.barmanServerName?.trim() || undefined,
  barmanHome: postgresBarmanWizardForm.barmanHome?.trim() || undefined,
  configPath: postgresBarmanWizardForm.configPath?.trim() || undefined,
  retentionPolicy: postgresBarmanWizardForm.retentionPolicy?.trim() || undefined,
  backupMethod: postgresBarmanWizardForm.backupMethod?.trim() || undefined,
  streamingArchiverEnabled: postgresBarmanWizardForm.streamingArchiverEnabled !== false,
  archiverEnabled: postgresBarmanWizardForm.archiverEnabled !== false,
  slotName: postgresBarmanWizardForm.slotName?.trim() || undefined,
  configJson: postgresBarmanWizardForm.configJson?.trim() || undefined,
  runCheckNow: postgresBarmanWizardForm.runCheckNow !== false,
  syncCatalogNow: postgresBarmanWizardForm.syncCatalogNow !== false,
  syncWalNow: postgresBarmanWizardForm.syncWalNow !== false,
  runInitialBackupNow: postgresBarmanWizardForm.runInitialBackupNow === true
})

const previewPostgresBarmanWizard = async () => {
  if (!postgresBarmanWizardFormRef.value) return
  await postgresBarmanWizardFormRef.value.validate()
  postgresBarmanWizardPreviewing.value = true
  try {
    const res = await previewDatabasePostgresBarmanPITRWizard(buildPostgresBarmanWizardPayload()) as DatabasePostgresBarmanPITRWizardResult
    postgresBarmanWizardPreview.value = res
    if (res.blockingReasons?.length) {
      ElMessage.error(res.blockingReasons[0] || 'PostgreSQL Barman 向导预检未通过')
    } else if (res.warnings?.length) {
      ElMessage.warning(res.warnings[0] || 'PostgreSQL Barman 向导预检有提醒')
    } else {
      ElMessage.success('PostgreSQL Barman 向导预检通过')
    }
  } finally {
    postgresBarmanWizardPreviewing.value = false
  }
}

const applyPostgresBarmanWizard = async () => {
  if (!postgresBarmanWizardFormRef.value) return
  await postgresBarmanWizardFormRef.value.validate()
  if (postgresBarmanWizardPreview.value && !postgresBarmanWizardPreview.value.canApply) {
    ElMessage.error(postgresBarmanWizardPreview.value.blockingReasons?.[0] || 'PostgreSQL Barman 向导预检未通过')
    return
  }
  postgresBarmanWizardSubmitting.value = true
  try {
    const res = await applyDatabasePostgresBarmanPITRWizard(buildPostgresBarmanWizardPayload()) as DatabasePostgresBarmanPITRWizardResult
    postgresBarmanWizardPreview.value = res
    const jobs = [res.checkJob, res.catalogSyncJob, res.walSyncJob, res.initialBackupJob].filter(Boolean).length
    if (res.warnings?.length) {
      ElMessage.warning(`${res.warnings[0]}${jobs ? `；已下发 ${jobs} 个 Runner Job` : ''}`)
    } else {
      ElMessage.success(`PostgreSQL Barman PITR 保护已启用${jobs ? `，已下发 ${jobs} 个 Runner Job` : ''}`)
    }
    postgresBarmanWizardDialogVisible.value = false
    await Promise.all([loadProtectionProfiles(), loadProtectionRisks(), loadBarmanServers(), loadBarmanCatalogRecords(), loadWalStatusArchives(), loadLogArchiveStreams(), loadRunnerJobs()])
  } finally {
    postgresBarmanWizardSubmitting.value = false
  }
}

const buildProtectionWizardPayload = (): DatabaseMySQLPITRWizardPayload => ({
  instanceId: protectionWizardForm.instanceId,
  sourceInstanceId: protectionWizardForm.sourceInstanceId || undefined,
  sourceRole: protectionWizardForm.sourceRole || 'primary',
  runnerHostId: protectionWizardForm.runnerHostId,
  storageProfileId: protectionWizardForm.storageProfileId || undefined,
  secretProfileId: protectionWizardForm.secretProfileId || undefined,
  templateKey: protectionWizardForm.templateKey || 'rolling_synthetic_full',
  policyName: protectionWizardForm.policyName?.trim() || undefined,
  backupEngine: protectionWizardForm.backupEngine || defaultProtectionWizardBackupEngine(),
  toolExecutionMode: protectionWizardForm.toolExecutionMode || 'host_tools',
  toolImage: protectionWizardForm.toolImage?.trim() || undefined,
  toolImageDigest: protectionWizardForm.toolImageDigest?.trim() || undefined,
  containerDatadirPath: protectionWizardForm.containerDatadirPath?.trim() || undefined,
  containerWorkdirPath: protectionWizardForm.containerWorkdirPath?.trim() || undefined,
  containerNetworkMode: protectionWizardForm.containerNetworkMode?.trim() || undefined,
  containerDatadirRo: protectionWizardForm.containerDatadirRo !== false,
  reuseLogArchiveStreamId: protectionWizardForm.reuseLogArchiveStreamId || undefined,
  reuseBackupPolicyId: protectionWizardForm.reuseBackupPolicyId || undefined,
  fullSchedule: protectionWizardForm.fullSchedule?.trim() || '',
  incrementalSchedule: protectionWizardForm.incrementalSchedule?.trim() || '0 3 * * *',
  runInitialFullNow: protectionWizardForm.runInitialFullNow === true,
  binlogArchiveMode: protectionWizardForm.binlogArchiveMode || 'polling',
  binlogRpoTargetSeconds: protectionWizardForm.binlogRpoTargetSeconds || 300,
  binlogRetentionDays: protectionWizardForm.binlogRetentionDays || 45,
  syntheticEnabled: protectionWizardForm.syntheticEnabled === true,
  syntheticAutoRun: protectionWizardForm.syntheticAutoRun !== false,
  syntheticTriggerAfterIncrementals: protectionWizardForm.syntheticTriggerAfterIncrementals || 5,
  syntheticMergeOldestIncrementals: protectionWizardForm.syntheticMergeOldestIncrementals || 5,
  syntheticRequireRestoreProof: protectionWizardForm.syntheticRequireRestoreProof !== false,
  syntheticNeverDeleteWithoutProof: protectionWizardForm.syntheticNeverDeleteWithoutProof !== false,
  syntheticMarkSupersededAfterProof: protectionWizardForm.syntheticMarkSupersededAfterProof !== false,
  syntheticSupersededKeepDays: protectionWizardForm.syntheticSupersededKeepDays || 7,
  restoreDrillRequired: protectionWizardForm.restoreDrillRequired !== false,
  retentionFullKeepMonths: protectionWizardForm.retentionFullKeepMonths || 6,
  retentionIncrementalKeepDays: protectionWizardForm.retentionIncrementalKeepDays || 45,
  retentionBinlogKeepDays: protectionWizardForm.retentionBinlogKeepDays || 45,
  retentionNeverDeleteWithoutProof: protectionWizardForm.retentionNeverDeleteWithoutProof !== false,
  archiveConfigJson: protectionWizardForm.archiveConfigJson?.trim() || undefined
})

const previewProtectionWizard = async () => {
  if (!protectionWizardFormRef.value) return
  await protectionWizardFormRef.value.validate()
  protectionWizardPreviewing.value = true
  try {
    const res = await previewDatabaseMySQLPITRWizard(buildProtectionWizardPayload()) as DatabaseMySQLPITRWizardResult
    protectionWizardPreview.value = res
    if (res.blockingReasons?.length) {
      ElMessage.error(res.blockingReasons[0] || '保护向导预检未通过')
    } else if (res.warnings?.length) {
      ElMessage.warning(res.warnings[0] || '保护向导预检有提醒')
    } else {
      ElMessage.success('保护向导预检通过')
    }
  } finally {
    protectionWizardPreviewing.value = false
  }
}

const applyProtectionWizard = async () => {
  if (!protectionWizardFormRef.value) return
  await protectionWizardFormRef.value.validate()
  if (protectionWizardPreview.value && !protectionWizardPreview.value.canApply) {
    ElMessage.error(protectionWizardPreview.value.blockingReasons?.[0] || '保护向导预检未通过')
    return
  }
  protectionWizardSubmitting.value = true
  try {
    const res = await applyDatabaseMySQLPITRWizard(buildProtectionWizardPayload()) as DatabaseMySQLPITRWizardResult
    protectionWizardPreview.value = res
    const fullText = res.initialFullRun?.runnerJobId ? `，初始 Full 已下发 Runner Job #${res.initialFullRun.runnerJobId}` : ''
    if (res.warnings?.length) {
      ElMessage.warning(`${res.warnings[0]}${fullText}`)
    } else {
      ElMessage.success(`MySQL/MariaDB PITR 保护已启用${fullText}`)
    }
    protectionWizardDialogVisible.value = false
    await Promise.all([loadProtectionProfiles(), loadBackupPolicies(), loadLogArchiveStreams(), loadRunnerJobs()])
  } finally {
    protectionWizardSubmitting.value = false
  }
}

const defaultRestoreImageForProfile = (row?: DatabaseProtectionProfileResult) => {
  const instance = instanceOptions.value.find(item => item.id === row?.instanceId)
  return defaultRestoreImageForInstance(instance || { dbType: row?.engine, version: row?.version })
}

const openProtectionRestoreDrillDialog = async (row: DatabaseProtectionProfileResult) => {
  if (!runnerHosts.value.length) {
    await loadRunnerHosts()
  }
  resetProtectionRestoreDrillForm()
  protectionRestoreDrillProfile.value = row
  protectionRestoreDrillForm.restoreTargetType = 'time'
  protectionRestoreDrillForm.restoreTargetValue = row.recoverableUntil || formatDateTimeInput()
  protectionRestoreDrillForm.runnerHostId = row.runnerHost?.id || row.backupPolicy?.runnerHostId || defaultEnabledRunnerHostId() || undefined
  protectionRestoreDrillForm.containerImage = defaultRestoreImageForProfile(row)
  protectionRestoreDrillForm.postgresStartInstance = row.engine === 'postgresql' ? true : undefined
  protectionRestoreDrillForm.targetAction = row.engine === 'postgresql' ? 'pause' : undefined
  protectionRestoreDrillForm.barmanGetWal = row.engine === 'postgresql' ? true : undefined
  protectionRestoreDrillDialogVisible.value = true
}

const addProtectionRestoreDrillAssertion = () => {
  if (!protectionRestoreDrillForm.validationAssertions) protectionRestoreDrillForm.validationAssertions = []
  protectionRestoreDrillForm.validationAssertions.push({
    sql: '',
    expectedRows: undefined,
    expectedContains: '',
    expectedScalar: ''
  })
}

const removeProtectionRestoreDrillAssertion = (index: number) => {
  protectionRestoreDrillForm.validationAssertions?.splice(index, 1)
}

const submitProtectionRestoreDrill = async () => {
  if (!protectionRestoreDrillFormRef.value || !protectionRestoreDrillProfile.value?.profileId) return
  await protectionRestoreDrillFormRef.value.validate()
  protectionRestoreDrillSubmitting.value = true
  try {
    const validationSql = String(protectionRestoreDrillForm.validationSqlText || '')
      .split('\n')
      .map(item => item.trim())
      .filter(Boolean)
    const validationAssertions = (protectionRestoreDrillForm.validationAssertions || [])
      .map((item): DatabaseRestoreValidationAssertionPayload => ({
        sql: String(item.sql || '').trim(),
        expectedRows: typeof item.expectedRows === 'number' ? item.expectedRows : undefined,
        expectedContains: String(item.expectedContains || '').trim() || undefined,
        expectedScalar: String(item.expectedScalar || '').trim() || undefined
      }))
      .filter(item => Boolean(item.sql) && (
        typeof item.expectedRows === 'number' ||
        Boolean(item.expectedContains) ||
        item.expectedScalar !== undefined
      ))
    const isPostgreSQL = protectionRestoreDrillProfile.value.engine === 'postgresql'
    const payload: DatabaseProtectionRestoreDrillPayload = {
      restoreTargetType: protectionRestoreDrillForm.restoreTargetType || 'time',
      restoreTargetValue: protectionRestoreDrillForm.restoreTargetValue,
      targetTimelineId: protectionRestoreDrillForm.targetTimelineId?.trim() || undefined,
      restoreTargetInclusive: protectionRestoreDrillForm.restoreTargetInclusive !== false,
      runnerHostId: protectionRestoreDrillForm.runnerHostId || undefined,
      containerImage: protectionRestoreDrillForm.containerImage?.trim() || undefined,
      listenPort: protectionRestoreDrillForm.listenPort || undefined,
      expiresInHours: protectionRestoreDrillForm.expiresInHours || 24,
      validationSql,
      validationAssertions,
      cleanupOnFailure: protectionRestoreDrillForm.cleanupOnFailure === true,
      postgresStartInstance: isPostgreSQL ? protectionRestoreDrillForm.postgresStartInstance !== false : undefined,
      targetAction: isPostgreSQL ? (protectionRestoreDrillForm.targetAction || 'pause') : undefined,
      barmanGetWal: isPostgreSQL ? protectionRestoreDrillForm.barmanGetWal !== false : undefined
    }
    const res = await runDatabaseProtectionRestoreDrill(protectionRestoreDrillProfile.value.profileId, payload) as DatabaseProtectionRestoreDrillResult
    protectionRestoreDrillResult.value = res
    if (res.job) {
      ElMessage.success(`恢复演练任务已下发 Runner Job #${res.job.id}`)
    } else if (res.status === 'plan_failed' || res.status === 'run_failed') {
      ElMessage.error(res.message || '恢复演练预检失败')
    } else {
      ElMessage.warning(res.message || '恢复计划已生成，请补充 Runner 后执行')
    }
    await Promise.all([loadProtectionProfiles(), loadRestorePlans(), loadRestoreJobs(), loadRunnerJobs()])
  } finally {
    protectionRestoreDrillSubmitting.value = false
  }
}

const openRestorePlanDialog = () => {
  resetRestorePlanForm()
  restorePlanDialogVisible.value = true
}

const isPgBaseBackupFullRecord = (row?: DatabaseBackupRecordResult) => {
  return row?.status === 'success' &&
    row.backupMethod === 'physical' &&
    row.backupLevel === 'full' &&
    row.backupEngine === 'pg_basebackup'
}

const openRestorePlanDialogFromBackupRecord = (row: DatabaseBackupRecordResult) => {
  resetRestorePlanForm()
  restorePlanForm.sourceInstanceId = row.instanceId
  restorePlanForm.baseRecordId = row.id
  restorePlanForm.restoreMode = 'isolated_restore'
  restorePlanForm.restoreTargetType = 'time'
  restorePlanForm.restoreTargetValue = row.recoverableUntil || formatDateTimeInput()
  restorePlanForm.targetTimelineId = ''
  restorePlanForm.restoreTargetInclusive = true
  restorePlanForm.targetInstanceId = pitrRestoreTargetInstances.value.find(item => item.id !== row.instanceId)?.id
  restorePlanBaseRecordLabel.value = `#${row.id} ${row.fileName || row.externalBackupId || 'pg_basebackup full'}`
  restorePlanDialogVisible.value = true
}

const openBackupRecordDrill = (row: DatabaseBackupRecordResult) => {
  if (isPgBaseBackupFullRecord(row)) {
    openRestorePlanDialogFromBackupRecord(row)
    return
  }
  openRestoreDialog(row)
}

const submitRestorePlan = async () => {
  if (!restorePlanFormRef.value) return
  await restorePlanFormRef.value.validate()
  restorePlanSubmitting.value = true
  try {
    const payload: DatabaseRestorePlanPayload = {
      ...restorePlanForm,
      targetInstanceId: restorePlanForm.targetInstanceId || undefined,
      baseRecordId: restorePlanForm.baseRecordId || undefined,
      restoreMode: restorePlanForm.restoreMode || 'isolated_restore',
      restoreTargetType: restorePlanForm.restoreTargetType || 'time',
      restoreTargetInclusive: restorePlanForm.restoreTargetInclusive !== false
    }
    const res = await createDatabaseRestorePlan(payload) as DatabaseRestorePlanResult
    restorePlanDialogVisible.value = false
    ElMessage.success(res.message || '恢复计划预校验已生成')
    await loadRestorePlans()
  } finally {
    restorePlanSubmitting.value = false
  }
}

const defaultRestoreImageForPlan = (row?: DatabaseRestorePlanResult) => {
  return defaultRestoreImageForInstance(instanceOptions.value.find(item => item.id === row?.sourceInstanceId))
}

const restorePlanTargetTimeline = (row?: DatabaseRestorePlanResult) => {
  if (!row?.planJson) return ''
  try {
    const payload = JSON.parse(row.planJson)
    return String(payload?.target?.timelineId || payload?.targetTimelineId || '')
  } catch {
    return ''
  }
}

const openRunRestorePlanDialog = (row: DatabaseRestorePlanResult) => {
  restorePlanRunSource.value = row
  restorePlanRunForm.runnerHostId = row.runnerHostId || runnerHosts.value.find(item => item.runnerType === 'ssh' && item.enabled !== false)?.id || 0
  restorePlanRunForm.containerImage = defaultRestoreImageForPlan(row)
  restorePlanRunForm.listenPort = undefined
  restorePlanRunForm.expiresInHours = 24
  restorePlanRunForm.targetTimelineId = restorePlanTargetTimeline(row)
  restorePlanRunForm.targetAction = 'pause'
  restorePlanRunForm.barmanGetWal = true
  restorePlanRunForm.validationSqlText = ''
  restorePlanRunForm.validationSql = []
  restorePlanRunForm.validationAssertions = []
  restorePlanRunForm.cleanupOnFailure = false
  restorePlanRunForm.postgresStartInstance = true
  restorePlanRunForm.confirmIsolated = false
  restorePlanRunDialogVisible.value = true
}

const resetRestorePlanRunForm = () => {
  restorePlanRunSource.value = undefined
  restorePlanRunForm.runnerHostId = 0
  restorePlanRunForm.containerImage = ''
  restorePlanRunForm.listenPort = undefined
  restorePlanRunForm.expiresInHours = 24
  restorePlanRunForm.targetTimelineId = ''
  restorePlanRunForm.targetAction = 'pause'
  restorePlanRunForm.barmanGetWal = true
  restorePlanRunForm.validationSqlText = ''
  restorePlanRunForm.validationSql = []
  restorePlanRunForm.validationAssertions = []
  restorePlanRunForm.cleanupOnFailure = false
  restorePlanRunForm.postgresStartInstance = true
  restorePlanRunForm.confirmIsolated = false
  restorePlanRunFormRef.value?.clearValidate()
}

const addRestoreValidationAssertion = () => {
  if (!restorePlanRunForm.validationAssertions) restorePlanRunForm.validationAssertions = []
  restorePlanRunForm.validationAssertions.push({
    sql: '',
    expectedRows: undefined,
    expectedContains: '',
    expectedScalar: ''
  })
}

const removeRestoreValidationAssertion = (index: number) => {
  restorePlanRunForm.validationAssertions?.splice(index, 1)
}

const submitRunRestorePlan = async () => {
  if (!restorePlanRunFormRef.value || !restorePlanRunSource.value) return
  await restorePlanRunFormRef.value.validate()
  restorePlanRunSubmitting.value = true
  try {
    const validationSql = String(restorePlanRunForm.validationSqlText || '')
      .split('\n')
      .map(item => item.trim())
      .filter(Boolean)
    const validationAssertions = (restorePlanRunForm.validationAssertions || [])
      .map((item): DatabaseRestoreValidationAssertionPayload => ({
        sql: String(item.sql || '').trim(),
        expectedRows: typeof item.expectedRows === 'number' ? item.expectedRows : undefined,
        expectedContains: String(item.expectedContains || '').trim() || undefined,
        expectedScalar: String(item.expectedScalar || '').trim() || undefined
      }))
      .filter(item => Boolean(item.sql) && (
        typeof item.expectedRows === 'number' ||
        Boolean(item.expectedContains) ||
        item.expectedScalar !== undefined
      ))
    const payload: DatabaseRestorePlanRunPayload = {
      runnerHostId: restorePlanRunForm.runnerHostId,
      containerImage: (restorePlanRunForm.containerImage || undefined),
      listenPort: restorePlanRunForm.listenPort || undefined,
      expiresInHours: restorePlanRunForm.expiresInHours || 24,
      validationSql: (!isPostgreSQLRestoreRun.value || restorePlanRunForm.postgresStartInstance !== false) ? validationSql : [],
      validationAssertions: (!isPostgreSQLRestoreRun.value || restorePlanRunForm.postgresStartInstance !== false) ? validationAssertions : [],
      postgresStartInstance: isPostgreSQLRestoreRun.value ? restorePlanRunForm.postgresStartInstance !== false : undefined,
      targetTimelineId: isPostgreSQLRestoreRun.value ? (restorePlanRunForm.targetTimelineId || undefined) : undefined,
      targetAction: isPostgreSQLRestoreRun.value ? (restorePlanRunForm.targetAction || 'pause') : undefined,
      barmanGetWal: isBarmanRestoreRun.value ? restorePlanRunForm.barmanGetWal !== false : undefined,
      cleanupOnFailure: restorePlanRunForm.cleanupOnFailure === true
    }
    await runDatabaseRestorePlan(restorePlanRunSource.value.id, payload)
    restorePlanRunDialogVisible.value = false
    ElMessage.success('隔离恢复任务已下发 Runner')
    await Promise.all([loadRestorePlans(), loadRestoreJobs(), loadRunnerJobs()])
  } finally {
    restorePlanRunSubmitting.value = false
  }
}

const viewRestoreProof = async (row: DatabaseRestoreJobResult | DatabaseRestorePlanResult) => {
  restoreProofTitle.value = `恢复证明 #${row.id}`
  let proof = ''
  if ('proofJson' in row && row.proofJson) {
    proof = row.proofJson
  }
  if (!proof && 'restorePlanId' in row && row.id) {
    const res: any = await getDatabaseRestoreJobProof(row.id)
    proof = res?.proofJson || ''
  }
  if (!proof) {
    ElMessage.warning('恢复证明尚未生成')
    return
  }
  try {
    restoreProofContent.value = JSON.stringify(JSON.parse(proof), null, 2)
  } catch (_err) {
    restoreProofContent.value = proof
  }
  restoreProofDialogVisible.value = true
}

const restoreStepLabelMap: Record<string, string> = {
  prepare_restore_directory: '准备恢复目录',
  extract_base_backup: '解压全量备份',
  fetch_base_backup: '拉取全量备份',
  verify_base_checksum: '校验全量 checksum',
  fetch_incremental_chain: '拉取增量链',
  verify_incremental_checksums: '校验增量 checksum',
  prepare_physical_backup: '物理备份 prepare',
  fetch_binlog_chain: '拉取 binlog 链',
  verify_binlog_chain: '校验 binlog 链',
  pg_basebackup: '执行 pg_basebackup',
  package_backup: '打包备份',
  pg_combinebackup: '合成增量 (pg_combinebackup)',
  configure_recovery: '配置 recovery',
  barman_restore: 'Barman 恢复',
  verify_pgdata: '校验 PGDATA',
  start_isolated_instance: '启动隔离实例',
  start_isolated_postgres: '启动隔离 PostgreSQL',
  apply_binlog_to_target: '回放 binlog 到目标点',
  run_validation_sql: '执行校验 SQL',
  generate_proof: '生成恢复证明',
  mark_success_or_failed: '更新最终状态',
  tool_check: '工具检测'
}

const restoreStepStatusTextMap: Record<string, string> = {
  running: '进行中',
  success: '成功',
  failed: '失败',
  skipped: '跳过',
  warning: '告警'
}

const restoreStepLabel = (name: string) => restoreStepLabelMap[name] || name
const restoreStepStatusText = (status: string) => restoreStepStatusTextMap[status] || status || '-'

const restoreStepTagType = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'primary'
    case 'warning':
      return 'warning'
    case 'skipped':
      return 'info'
    default:
      return 'info'
  }
}

const restoreStepTimelineType = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'primary'
    case 'warning':
      return 'warning'
    default:
      return 'info'
  }
}

const restoreAssertionStatusText = (status: string) => {
  switch (status) {
    case 'passed':
      return '通过'
    case 'failed':
      return '失败'
    case 'not_configured':
      return '未配置'
    case 'unknown':
      return '未知'
    default:
      return status || '-'
  }
}

const restoreAssertionTagType = (status: string) => {
  switch (status) {
    case 'passed':
      return 'success'
    case 'failed':
      return 'danger'
    case 'not_configured':
      return 'info'
    default:
      return 'warning'
  }
}

const formatRestoreAssertionExpected = (row: RestoreJobValidationRow) => {
  const parts: string[] = []
  if (row.expectedRows !== undefined && row.expectedRows !== null) {
    parts.push(`rows=${row.expectedRows}`)
  }
  if (row.expectedScalar) {
    parts.push(`scalar=${row.expectedScalar}`)
  }
  if (row.expectedContains) {
    parts.push(`contains=${row.expectedContains}`)
  }
  return parts.length > 0 ? parts.join(' / ') : '-'
}

const formatRestoreAssertionActual = (row: RestoreJobValidationRow) => {
  const parts: string[] = []
  if (row.actualRows !== undefined && row.actualRows !== null) {
    parts.push(`rows=${row.actualRows}`)
  }
  if (row.actualScalar) {
    parts.push(`scalar=${row.actualScalar}`)
  }
  return parts.length > 0 ? parts.join(' / ') : '-'
}

const formatRestoreJobDuration = (ms: number | null | undefined) => {
  if (ms === null || ms === undefined || ms <= 0) return '-'
  if (ms < 1000) return `${ms} ms`
  const seconds = ms / 1000
  if (seconds < 60) return `${seconds.toFixed(1)} s`
  const minutes = Math.floor(seconds / 60)
  const restSeconds = Math.round(seconds - minutes * 60)
  if (minutes < 60) return `${minutes}m ${restSeconds}s`
  const hours = Math.floor(minutes / 60)
  const restMinutes = minutes - hours * 60
  return `${hours}h ${restMinutes}m`
}

const parseStepOccurredAt = (value: string): number | null => {
  if (!value) return null
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const ts = Date.parse(normalized)
  return Number.isNaN(ts) ? null : ts
}

const parseRestoreJobSteps = (stepJson: string | undefined): RestoreJobStepRow[] => {
  if (!stepJson) return []
  let raw: any
  try {
    raw = JSON.parse(stepJson)
  } catch (_err) {
    return []
  }
  if (!Array.isArray(raw)) return []
  const rows: RestoreJobStepRow[] = []
  const indexByName = new Map<string, number>()
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const name: string = String(item.name || '').trim()
    const status: string = String(item.status || '').trim()
    const occurredAt: string = String(item.occurredAt || '').trim()
    if (!name) continue
    if (status === 'running') {
      const row: RestoreJobStepRow = {
        name,
        label: restoreStepLabel(name),
        status,
        statusText: restoreStepStatusText(status),
        startedAt: occurredAt,
        finishedAt: '',
        durationMs: null
      }
      rows.push(row)
      indexByName.set(name, rows.length - 1)
      continue
    }
    const existingIdx = indexByName.get(name)
    if (existingIdx !== undefined) {
      const row = rows[existingIdx]
      if (!row) continue
      row.status = status || row.status
      row.statusText = restoreStepStatusText(row.status)
      row.finishedAt = occurredAt || row.finishedAt
      const startTs = parseStepOccurredAt(row.startedAt)
      const endTs = parseStepOccurredAt(row.finishedAt)
      row.durationMs = startTs !== null && endTs !== null && endTs >= startTs ? endTs - startTs : null
      indexByName.delete(name)
      continue
    }
    rows.push({
      name,
      label: restoreStepLabel(name),
      status: status || 'success',
      statusText: restoreStepStatusText(status || 'success'),
      startedAt: '',
      finishedAt: occurredAt,
      durationMs: null
    })
  }
  return rows
}

const parseRestoreJobValidations = (validationJson: string | undefined): RestoreJobValidationRow[] => {
  if (!validationJson) return []
  let raw: any
  try {
    raw = JSON.parse(validationJson)
  } catch (_err) {
    return []
  }
  if (!Array.isArray(raw)) return []
  return raw
    .filter((item: any) => item && typeof item === 'object')
    .map((item: any, index: number) => ({
      index: typeof item.index === 'number' ? item.index : index + 1,
      sql: item.sql,
      status: item.status,
      outputPreview: item.outputPreview,
      expectedRows: item.expectedRows,
      expectedContains: item.expectedContains,
      expectedScalar: item.expectedScalar,
      actualRows: item.actualRows,
      actualScalar: item.actualScalar,
      assertionStatus: item.assertionStatus,
      assertionMessage: item.assertionMessage
    }))
}

const openRestoreJobDetail = (row: DatabaseRestoreJobResult) => {
  restoreJobDetailRow.value = row
  restoreJobDetailTitle.value = `恢复任务 #${row.id}`
  restoreJobSteps.value = parseRestoreJobSteps(row.stepJson)
  restoreJobValidations.value = parseRestoreJobValidations(row.validationJson)
  restoreJobDetailVisible.value = true
}

const viewRestoreJobDetailProof = () => {
  if (!restoreJobDetailRow.value?.proofJson) return
  viewRestoreProof(restoreJobDetailRow.value)
}

const cleanupRestoreJob = async (row: DatabaseRestoreJobResult) => {
  await ElMessageBox.confirm('将停止隔离容器并删除 Runner 工作目录，确认清理？', '清理恢复环境', {
    confirmButtonText: '清理',
    cancelButtonText: '取消',
    type: 'warning'
  })
  await cleanupDatabaseRestoreJob(row.id)
  ElMessage.success('恢复环境已清理')
  await loadRestoreJobs()
}

const openRunnerHostDialog = (row?: DatabaseRunnerHostResult) => {
  resetRunnerHostForm()
  if (row?.id) {
    runnerHostForm.id = row.id
    runnerHostForm.name = row.name || ''
    runnerHostForm.runnerType = row.runnerType || 'ssh'
    runnerHostForm.host = row.host || ''
    runnerHostForm.port = row.port || 22
    runnerHostForm.credentialId = row.credentialId || undefined
    runnerHostForm.workDir = row.workDir || '/var/lib/opshub/database-runner'
    runnerHostForm.storageMountPath = row.storageMountPath || ''
    runnerHostForm.maxConcurrentJobs = row.maxConcurrentJobs || 1
    runnerHostForm.cpuLimit = row.cpuLimit || ''
    runnerHostForm.ioLimit = row.ioLimit || ''
    runnerHostForm.bandwidthLimit = row.bandwidthLimit || ''
    runnerHostForm.timeoutMinutes = row.timeoutMinutes || 30
    runnerHostForm.enabled = !!row.enabled
    runnerHostForm.configJson = row.configJson || ''
  }
  runnerHostDialogVisible.value = true
}

const submitRunnerHost = async () => {
  if (!runnerHostFormRef.value) return
  await runnerHostFormRef.value.validate()
  runnerHostSubmitting.value = true
  try {
    const payload: DatabaseRunnerHostPayload = {
      name: runnerHostForm.name.trim(),
      runnerType: runnerHostForm.runnerType || 'ssh',
      host: runnerHostForm.host?.trim(),
      port: runnerHostForm.runnerType === 'ssh' ? (runnerHostForm.port || 22) : undefined,
      credentialId: runnerHostForm.runnerType === 'ssh' ? (runnerHostForm.credentialId || undefined) : undefined,
      workDir: runnerHostForm.workDir?.trim(),
      storageMountPath: runnerHostForm.storageMountPath?.trim(),
      maxConcurrentJobs: runnerHostForm.maxConcurrentJobs || 1,
      cpuLimit: runnerHostForm.cpuLimit?.trim(),
      ioLimit: runnerHostForm.ioLimit?.trim(),
      bandwidthLimit: runnerHostForm.bandwidthLimit?.trim(),
      timeoutMinutes: runnerHostForm.timeoutMinutes || 30,
      enabled: runnerHostForm.enabled,
      configJson: runnerHostForm.configJson?.trim()
    }
    if (runnerHostForm.id) {
      await updateDatabaseRunnerHost(runnerHostForm.id, payload)
      ElMessage.success('Runner 主机已更新')
    } else {
      await createDatabaseRunnerHost(payload)
      ElMessage.success('Runner 主机已创建')
    }
    runnerHostDialogVisible.value = false
    await loadRunnerHosts()
  } finally {
    runnerHostSubmitting.value = false
  }
}

const handleTestRunnerHost = async (row: DatabaseRunnerHostResult) => {
  runnerHostTestingId.value = row.id
  try {
    await testDatabaseRunnerHost(row.id)
    ElMessage.success('Runner 探测任务已下发')
    await Promise.all([loadRunnerHosts(), loadRunnerJobs()])
    window.setTimeout(() => {
      loadRunnerHosts()
      loadRunnerJobs()
    }, 2500)
  } finally {
    runnerHostTestingId.value = 0
  }
}

const handleDeleteRunnerHost = async (row: DatabaseRunnerHostResult) => {
  await ElMessageBox.confirm(
    `确认删除 Runner 主机「${row.name || row.host || row.id}」？删除前后端会检查备份策略、归档流、Barman Server、运行中任务和 runner:// 本地 artifact 引用；存在引用时会拒绝删除。`,
    '删除 Runner 主机',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  await deleteDatabaseRunnerHost(row.id)
  ElMessage.success('Runner 主机已删除')
  await Promise.all([loadRunnerHosts(), loadRunnerJobs()])
}

const openRunnerToolProfileDialog = async (row: DatabaseRunnerHostResult) => {
  runnerToolProfile.value = undefined
  runnerToolProfileDialogVisible.value = true
  runnerToolProfileLoading.value = true
  try {
    const res: any = await getDatabaseRunnerToolProfile(row.id)
    runnerToolProfile.value = res || undefined
  } finally {
    runnerToolProfileLoading.value = false
  }
}

const handleProbeRunnerTools = async (row: DatabaseRunnerHostResult) => {
  runnerToolProbingId.value = row.id
  try {
    await probeDatabaseRunnerTools(row.id)
    ElMessage.success('Runner 工具巡检任务已下发')
    await loadRunnerJobs()
    window.setTimeout(() => {
      loadRunnerHosts()
      loadRunnerJobs()
    }, 3000)
  } finally {
    runnerToolProbingId.value = 0
  }
}

const openRunnerToolInstallScriptDialog = async (row: DatabaseRunnerHostResult) => {
  runnerToolScriptHost.value = row
  runnerToolScriptResult.value = undefined
  runnerToolScriptForm.profiles = ['mysql_80_physical', 'mysql_binlog_archiver', 'postgres_barman', 'postgres_native_pg_basebackup', 'restore_runner']
  runnerToolScriptForm.installMode = 'online'
  runnerToolScriptForm.executionMode = 'host_tools'
  runnerToolScriptForm.toolImage = ''
  runnerToolScriptForm.toolImageDigest = ''
  runnerToolScriptForm.datadirMount = ''
  runnerToolScriptForm.workdirMount = ''
  runnerToolScriptForm.networkMode = 'host'
  runnerToolScriptForm.readOnlyDatadir = true
  runnerToolScriptForm.dryRun = true
  runnerToolScriptForm.mysqlVersion = ''
  runnerToolScriptForm.postgresqlVersion = ''
  runnerToolScriptForm.reason = ''
  runnerToolScriptForm.confirmInstall = false
  runnerToolScriptForm.confirmPackages = false
  runnerToolScriptForm.offlinePackageId = undefined
  await loadRunnerToolOfflinePackages()
  runnerToolScriptDialogVisible.value = true
}

const handleGenerateRunnerToolScript = async () => {
  if (!runnerToolScriptHost.value?.id) return
  if (!runnerToolScriptForm.profiles.length) {
    ElMessage.warning('请至少选择一个工具 Profile')
    return
  }
  runnerToolScriptGenerating.value = true
  try {
    const targetDbVersions: Record<string, string> = {}
    if (runnerToolScriptForm.mysqlVersion.trim()) targetDbVersions.mysql = runnerToolScriptForm.mysqlVersion.trim()
    if (runnerToolScriptForm.postgresqlVersion.trim()) targetDbVersions.postgresql = runnerToolScriptForm.postgresqlVersion.trim()
	    const payload: DatabaseRunnerToolInstallScriptPayload = {
	      profiles: runnerToolScriptForm.profiles,
	      installMode: runnerToolScriptForm.installMode,
	      executionMode: runnerToolScriptForm.executionMode,
	      toolImage: runnerToolScriptForm.toolImage.trim(),
	      toolImageDigest: runnerToolScriptForm.toolImageDigest.trim(),
	      datadirMount: runnerToolScriptForm.datadirMount.trim(),
	      workdirMount: runnerToolScriptForm.workdirMount.trim(),
	      networkMode: runnerToolScriptForm.networkMode.trim() || 'host',
	      readOnlyDatadir: runnerToolScriptForm.readOnlyDatadir,
	      dryRun: runnerToolScriptForm.dryRun,
      targetDbVersions
    }
    const res: any = await generateDatabaseRunnerToolInstallScript(runnerToolScriptHost.value.id, payload)
    runnerToolScriptResult.value = res
    ElMessage.success('安装脚本已生成')
  } finally {
    runnerToolScriptGenerating.value = false
  }
}

const handleInstallRunnerTools = async () => {
  if (!runnerToolScriptHost.value?.id) return
  if (!runnerToolScriptForm.profiles.length) {
    ElMessage.warning('请至少选择一个工具 Profile')
    return
  }
	  if (runnerToolScriptForm.executionMode !== 'container_tools' && runnerToolScriptForm.installMode === 'offline' && !runnerToolScriptForm.offlinePackageId) {
	    ElMessage.warning('离线安装需要选择离线包')
	    return
	  }
	  if (runnerToolScriptForm.executionMode === 'container_tools' && !runnerToolScriptForm.toolImage.trim()) {
	    ElMessage.warning('容器化工具模式需要填写工具镜像')
	    return
	  }
  if (!runnerToolScriptForm.reason.trim()) {
    ElMessage.warning('请填写安装原因')
    return
  }
  if (!runnerToolScriptForm.confirmInstall || !runnerToolScriptForm.confirmPackages) {
    ElMessage.warning('请勾选执行确认和包范围确认')
    return
  }
  await ElMessageBox.confirm(
    `确认在 Runner「${runnerToolScriptHost.value.name}」上执行工具安装任务？`,
    'Runner 工具安装确认',
    { type: 'warning', confirmButtonText: '确认执行', cancelButtonText: '取消' }
  )
  runnerToolInstalling.value = true
  try {
    const targetDbVersions: Record<string, string> = {}
    if (runnerToolScriptForm.mysqlVersion.trim()) targetDbVersions.mysql = runnerToolScriptForm.mysqlVersion.trim()
    if (runnerToolScriptForm.postgresqlVersion.trim()) targetDbVersions.postgresql = runnerToolScriptForm.postgresqlVersion.trim()
	    const payload: DatabaseRunnerToolInstallPayload = {
	      profiles: runnerToolScriptForm.profiles,
	      installMode: runnerToolScriptForm.installMode,
	      executionMode: runnerToolScriptForm.executionMode,
	      toolImage: runnerToolScriptForm.toolImage.trim(),
	      toolImageDigest: runnerToolScriptForm.toolImageDigest.trim(),
	      datadirMount: runnerToolScriptForm.datadirMount.trim(),
	      workdirMount: runnerToolScriptForm.workdirMount.trim(),
	      networkMode: runnerToolScriptForm.networkMode.trim() || 'host',
	      readOnlyDatadir: runnerToolScriptForm.readOnlyDatadir,
	      dryRun: runnerToolScriptForm.dryRun,
      targetDbVersions,
      confirmInstall: runnerToolScriptForm.confirmInstall,
      confirmPackages: runnerToolScriptForm.confirmPackages,
      reason: runnerToolScriptForm.reason,
      offlinePackageId: runnerToolScriptForm.offlinePackageId
    }
    await installDatabaseRunnerTools(runnerToolScriptHost.value.id, payload)
    ElMessage.success('Runner 工具安装任务已下发')
    await loadRunnerJobs()
    window.setTimeout(() => {
      loadRunnerHosts()
      loadRunnerJobs()
    }, 3000)
  } finally {
    runnerToolInstalling.value = false
  }
}

const openRunnerToolOfflinePackageDialog = async () => {
  runnerToolOfflinePackageDialogVisible.value = true
  await loadRunnerToolOfflinePackages()
}

const loadRunnerToolOfflinePackages = async () => {
  runnerToolOfflinePackageLoading.value = true
  try {
    const res: any = await listDatabaseRunnerToolOfflinePackages({ page: 1, pageSize: 100 })
    runnerToolOfflinePackages.value = res.list || []
  } finally {
    runnerToolOfflinePackageLoading.value = false
  }
}

const handleRunnerToolOfflineFileChange = (event: Event) => {
  const input = event.target as HTMLInputElement
  runnerToolOfflinePackageFile.value = input.files?.[0] || null
  if (runnerToolOfflinePackageFile.value && !runnerToolOfflinePackageForm.name) {
    runnerToolOfflinePackageForm.name = runnerToolOfflinePackageFile.value.name
  }
}

const submitRunnerToolOfflinePackage = async () => {
  if (!runnerToolOfflinePackageFile.value) {
    ElMessage.warning('请选择离线包文件')
    return
  }
  if (!runnerToolOfflinePackageForm.profiles.length) {
    ElMessage.warning('请选择离线包支持的 Profile')
    return
  }
  runnerToolOfflinePackageUploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', runnerToolOfflinePackageFile.value)
    formData.append('name', runnerToolOfflinePackageForm.name)
    formData.append('packageVersion', runnerToolOfflinePackageForm.packageVersion)
    formData.append('osFamily', runnerToolOfflinePackageForm.osFamily)
    formData.append('osVersion', runnerToolOfflinePackageForm.osVersion)
    formData.append('arch', runnerToolOfflinePackageForm.arch)
    formData.append('packageManager', runnerToolOfflinePackageForm.packageManager)
    formData.append('profiles', runnerToolOfflinePackageForm.profiles.join(','))
    formData.append('checksumSha256', runnerToolOfflinePackageForm.checksumSha256)
    formData.append('manifestJson', runnerToolOfflinePackageForm.manifestJson)
    await uploadDatabaseRunnerToolOfflinePackage(formData)
    ElMessage.success('离线包已上传')
    runnerToolOfflinePackageFile.value = null
    await loadRunnerToolOfflinePackages()
  } finally {
    runnerToolOfflinePackageUploading.value = false
  }
}

const downloadRunnerToolOfflinePackage = async (row: DatabaseRunnerToolOfflinePackageResult) => {
  const blob = await downloadDatabaseRunnerToolOfflinePackage(row.id) as Blob
  downloadBlob(blob, row.fileName || row.name || `runner-tool-package-${row.id}`)
}

const openRunnerJobDetail = (row: DatabaseRunnerJobResult) => {
  runnerJobDetail.value = row
  runnerJobDetailVisible.value = true
}

const openBarmanServerDialog = (row?: DatabaseBarmanServerResult) => {
  resetBarmanServerForm()
  if (row?.id) {
    barmanServerForm.id = row.id
    barmanServerForm.sourceInstanceId = row.sourceInstanceId || 0
    barmanServerForm.runnerHostId = row.runnerHostId || 0
    barmanServerForm.name = row.name || ''
    barmanServerForm.barmanServerName = row.barmanServerName || ''
    barmanServerForm.barmanHome = row.barmanHome || ''
    barmanServerForm.configPath = row.configPath || ''
    barmanServerForm.retentionPolicy = row.retentionPolicy || ''
    barmanServerForm.backupMethod = row.backupMethod || ''
    barmanServerForm.streamingArchiverEnabled = !!row.streamingArchiverEnabled
    barmanServerForm.archiverEnabled = !!row.archiverEnabled
    barmanServerForm.slotName = row.slotName || ''
    barmanServerForm.status = row.status || 'pending'
    barmanServerForm.configJson = row.configJson || ''
  }
  barmanServerDialogVisible.value = true
}

const submitBarmanServer = async () => {
  if (!barmanServerFormRef.value) return
  await barmanServerFormRef.value.validate()
  barmanServerSubmitting.value = true
  try {
    const payload: DatabaseBarmanServerPayload = {
      sourceInstanceId: barmanServerForm.sourceInstanceId,
      runnerHostId: barmanServerForm.runnerHostId,
      name: barmanServerForm.name.trim(),
      barmanServerName: barmanServerForm.barmanServerName.trim(),
      barmanHome: barmanServerForm.barmanHome?.trim(),
      configPath: barmanServerForm.configPath?.trim(),
      retentionPolicy: barmanServerForm.retentionPolicy?.trim(),
      backupMethod: barmanServerForm.backupMethod?.trim(),
      streamingArchiverEnabled: !!barmanServerForm.streamingArchiverEnabled,
      archiverEnabled: !!barmanServerForm.archiverEnabled,
      slotName: barmanServerForm.slotName?.trim(),
      status: barmanServerForm.status || 'pending',
      configJson: barmanServerForm.configJson?.trim()
    }
    if (barmanServerForm.id) {
      await updateDatabaseBarmanServer(barmanServerForm.id, payload)
      ElMessage.success('Barman Server 已更新')
    } else {
      await createDatabaseBarmanServer(payload)
      ElMessage.success('Barman Server 已创建')
    }
    barmanServerDialogVisible.value = false
    await loadBarmanServers()
  } finally {
    barmanServerSubmitting.value = false
  }
}

const handleCheckBarmanServer = async (row: DatabaseBarmanServerResult) => {
  barmanCheckingId.value = row.id
  try {
    await checkDatabaseBarmanServer(row.id)
    ElMessage.success('Barman 检查任务已下发')
    await Promise.all([loadBarmanServers(), loadRunnerJobs()])
    window.setTimeout(() => {
      loadBarmanServers()
      loadRunnerJobs()
    }, 2500)
  } finally {
    barmanCheckingId.value = 0
  }
}

const handleSyncBarmanCatalog = async (row: DatabaseBarmanServerResult) => {
  barmanCatalogSyncingId.value = row.id
  try {
    await syncDatabaseBarmanCatalog(row.id)
    ElMessage.success('Barman catalog 同步任务已下发')
    await Promise.all([loadBarmanServers(), loadRunnerJobs(), loadBackupRecords()])
    window.setTimeout(() => {
      loadBarmanServers()
      loadRunnerJobs()
      loadBackupRecords()
    }, 3000)
  } finally {
    barmanCatalogSyncingId.value = 0
  }
}

const handleSyncBarmanWAL = async (row: DatabaseBarmanServerResult) => {
  barmanWalSyncingId.value = row.id
  try {
    await syncDatabaseBarmanWAL(row.id)
    ElMessage.success('Barman WAL 同步任务已下发')
    await Promise.all([loadBarmanServers(), loadRunnerJobs(), loadLogArchiveStreams(), loadLogArchives()])
    window.setTimeout(() => {
      loadBarmanServers()
      loadRunnerJobs()
      loadLogArchiveStreams()
      loadLogArchives()
    }, 3000)
  } finally {
    barmanWalSyncingId.value = 0
  }
}

const handleBackupBarmanServer = async (row: DatabaseBarmanServerResult) => {
  await ElMessageBox.confirm(
    `确认通过 Runner 触发 Barman cluster 级物理备份「${row.barmanServerName}」？该动作会在 Barman server 上执行受控 barman backup。`,
    '触发 Barman 备份',
    {
      confirmButtonText: '触发备份',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  barmanBackingUpId.value = row.id
  try {
    await backupDatabaseBarmanServer(row.id)
    ElMessage.success('Barman 备份任务已下发')
    await Promise.all([loadBarmanServers(), loadRunnerJobs(), loadBackupRecords()])
    window.setTimeout(() => {
      loadBarmanServers()
      loadRunnerJobs()
      loadBackupRecords()
    }, 5000)
  } finally {
    barmanBackingUpId.value = 0
  }
}

const handleDeleteBarmanServer = async (row: DatabaseBarmanServerResult) => {
  await ElMessageBox.confirm(`确认删除 Barman Server「${row.name}」？已同步的备份记录不会被删除。`, '删除 Barman Server', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning'
  })
  await deleteDatabaseBarmanServer(row.id)
  ElMessage.success('Barman Server 已删除')
  await loadBarmanServers()
}

const runnerAgentIDForHost = (row: DatabaseRunnerHostResult) => `runner-host-${row.id}`

const openRunnerAgentConfigDialog = async (row: DatabaseRunnerHostResult) => {
  if (!logArchiveStreams.value.length) {
    await loadLogArchiveStreams()
  }
  runnerAgentConfigHost.value = row
  runnerAgentAuthSha256.value = ''
  runnerAgentConfigJson.value = ''
  runnerAgentLogs.value = undefined
  runnerAgentLifecycleForm.serverUrl = window.location.origin
  runnerAgentLifecycleForm.installPath = '/opt/opshub-agent'
  runnerAgentLifecycleForm.serviceName = `opshub-agent-runner-${row.id}`
  runnerAgentLifecycleForm.listenAddr = '0.0.0.0:19100'
  runnerAgentLifecycleForm.intervalSeconds = 60
  runnerAgentLifecycleForm.databaseArchiverEnabled = true
  runnerAgentLifecycleForm.dryRun = false
  runnerAgentLifecycleForm.regenerateAuth = false
  runnerAgentLifecycleForm.confirm = false
  runnerAgentLifecycleForm.reason = ''
  runnerAgentConfigDialogVisible.value = true
}

const runnerAgentLifecyclePayload = (): DatabaseRunnerAgentLifecyclePayload => ({
  serverUrl: (runnerAgentLifecycleForm.serverUrl || '').trim(),
  installPath: (runnerAgentLifecycleForm.installPath || '').trim(),
  serviceName: (runnerAgentLifecycleForm.serviceName || '').trim(),
  listenAddr: (runnerAgentLifecycleForm.listenAddr || '').trim(),
  intervalSeconds: Number(runnerAgentLifecycleForm.intervalSeconds || 60),
  databaseArchiverEnabled: runnerAgentLifecycleForm.databaseArchiverEnabled === true,
  dryRun: runnerAgentLifecycleForm.dryRun === true,
  regenerateAuth: runnerAgentLifecycleForm.regenerateAuth === true,
  confirm: runnerAgentLifecycleForm.confirm === true,
  reason: (runnerAgentLifecycleForm.reason || '').trim()
})

const handleGenerateRunnerAgentConfig = async () => {
  if (!runnerAgentConfigHost.value?.id) return
  runnerAgentLifecycleLoading.value = true
  try {
    const res = await generateDatabaseRunnerAgentConfigSnippet(runnerAgentConfigHost.value.id, runnerAgentLifecyclePayload()) as DatabaseRunnerAgentConfigSnippetResult
    runnerAgentAuthSha256.value = res.runnerAuthSha256 || ''
    runnerAgentConfigJson.value = res.configJson || ''
    ElMessage.success(res.message || 'Runner Agent 配置已生成')
    await loadRunnerHosts()
  } finally {
    runnerAgentLifecycleLoading.value = false
  }
}

const handleRunnerAgentLifecycle = async (action: 'install' | 'upgrade' | 'restart') => {
  if (!runnerAgentConfigHost.value?.id) return
  const actionText = action === 'install' ? '安装' : action === 'upgrade' ? '升级' : '重启'
  const payload = runnerAgentLifecyclePayload()
  if (!payload.reason) {
    ElMessage.warning('请填写操作原因')
    return
  }
  if (!payload.confirm) {
    ElMessage.warning('请勾选执行确认')
    return
  }
  await ElMessageBox.confirm(`确认${actionText} Runner「${runnerAgentConfigHost.value.name}」上的 OpsHub Agent？`, `Runner Agent ${actionText}`, {
    type: 'warning',
    confirmButtonText: '确认执行',
    cancelButtonText: '取消'
  })
  runnerAgentLifecycleLoading.value = true
  try {
    if (action === 'install') {
      await installDatabaseRunnerAgent(runnerAgentConfigHost.value.id, payload)
    } else if (action === 'upgrade') {
      await upgradeDatabaseRunnerAgent(runnerAgentConfigHost.value.id, payload)
    } else {
      await restartDatabaseRunnerAgent(runnerAgentConfigHost.value.id, payload)
    }
    ElMessage.success(`Runner Agent ${actionText}任务已下发`)
    await Promise.all([loadRunnerHosts(), loadRunnerJobs()])
  } finally {
    runnerAgentLifecycleLoading.value = false
  }
}

const handleFetchRunnerAgentLogs = async () => {
  if (!runnerAgentConfigHost.value?.id) return
  runnerAgentLogsLoading.value = true
  try {
    runnerAgentLogs.value = await getDatabaseRunnerAgentLogs(runnerAgentConfigHost.value.id, { lines: 300 }) as DatabaseRunnerAgentLogsResult
  } finally {
    runnerAgentLogsLoading.value = false
  }
}

const copyText = async (value: string, label: string) => {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      ElMessage.success(`${label}已复制`)
      return
    } catch {
      // fall back to textarea copy below
    }
  }
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
  ElMessage.success(`${label}已复制`)
}

const loadQueryFavorites = () => {
  try {
    const raw = window.localStorage.getItem(queryFavoriteStorageKey)
    const parsed = raw ? JSON.parse(raw) : []
    queryFavorites.value = Array.isArray(parsed) ? parsed.slice(0, 20) : []
  } catch {
    queryFavorites.value = []
  }
}

const persistQueryFavorites = () => {
  try {
    window.localStorage.setItem(queryFavoriteStorageKey, JSON.stringify(queryFavorites.value.slice(0, 20)))
  } catch {
    ElMessage.warning('浏览器本地存储不可用，收藏未持久化')
  }
}

const buildQueryFavoriteTitle = (sqlText: string) => {
  const normalized = sqlText.replace(/\s+/g, ' ').trim()
  return normalized.length > 36 ? `${normalized.slice(0, 36)}...` : normalized
}

const addQueryFavorite = (sqlText: string, schemaName = querySchemaName.value) => {
  const normalizedSQL = sqlText.trim()
  if (!normalizedSQL) {
    ElMessage.warning('请输入 SQL')
    return
  }
  const duplicatedIndex = queryFavorites.value.findIndex(item =>
    item.sqlText === normalizedSQL &&
    item.schemaName === (schemaName || '') &&
    item.dbType === (currentQueryInstance.value?.dbType || '')
  )
  const favorite: QuerySQLFavorite = {
    id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    title: buildQueryFavoriteTitle(normalizedSQL),
    sqlText: normalizedSQL,
    schemaName: schemaName || '',
    dbType: currentQueryInstance.value?.dbType || '',
    createdAt: new Date().toISOString()
  }
  if (duplicatedIndex >= 0) {
    queryFavorites.value.splice(duplicatedIndex, 1)
  }
  queryFavorites.value.unshift(favorite)
  queryFavorites.value = queryFavorites.value.slice(0, 20)
  persistQueryFavorites()
  ElMessage.success('SQL 已收藏')
}

const saveCurrentQueryFavorite = () => {
  addQueryFavorite(querySQL.value)
}

const saveHistoryQueryFavorite = (row: any) => {
  if (!row?.sqlText) return
  addQueryFavorite(row.sqlText, row.schemaName || querySchemaName.value)
}

const applyQueryFavorite = (item: QuerySQLFavorite) => {
  querySQL.value = item.sqlText
  if (item.schemaName) {
    querySchemaName.value = item.schemaName
  }
  clearQueryConsoleState()
  ElMessage.success('已填入 SQL 编辑器')
}

const removeQueryFavorite = (id: string) => {
  queryFavorites.value = queryFavorites.value.filter(item => item.id !== id)
  persistQueryFavorites()
  ElMessage.success('收藏已删除')
}

const copyQueryCell = (value: any, column: string) => {
  copyText(formatQueryCell(value), `字段 ${column}`)
}

const openLogArchiveEventsForStream = async (row: DatabaseLogArchiveStreamResult) => {
  backupPitrTab.value = 'advancedResources'
  backupAdvancedTab.value = 'events'
  logArchiveEventQuery.page = 1
  logArchiveEventQuery.streamId = row.id
  logArchiveEventQuery.instanceId = undefined
  logArchiveEventQuery.runnerHostId = undefined
  await loadLogArchiveEvents()
}

const openLogArchiveEventsForRunner = async (row: DatabaseRunnerHostResult) => {
  backupPitrTab.value = 'advancedResources'
  backupAdvancedTab.value = 'events'
  logArchiveEventQuery.page = 1
  logArchiveEventQuery.streamId = undefined
  logArchiveEventQuery.instanceId = undefined
  logArchiveEventQuery.runnerHostId = row.id
  await loadLogArchiveEvents()
}

const loadInstancePermissions = async () => {
  permissionLoading.value = true
  try {
    const res: any = await listDatabaseInstancePermissions(permissionQuery)
    permissionRows.value = res.list || []
    permissionTotal.value = res.total || 0
    permissionMode.value = res.permissionMode === 'whitelist' ? 'whitelist' : 'compat'
    permissionModeEnforced.value = !!res.permissionModeEnforced
    permissionRulesEnabled.value = !!res.permissionRulesEnabled
    if (res.page) permissionQuery.page = res.page
    if (res.pageSize) permissionQuery.pageSize = res.pageSize
  } finally {
    permissionLoading.value = false
  }
}

const permissionMaskToBits = (mask: number | undefined) =>
  databasePermissionOptions
    .filter(item => hasPermissionMask(mask, item.value))
    .map(item => item.value)

const permissionBitsToMask = (bits: number[]) =>
  bits.reduce((mask, item) => mask | item, 0)

const resetPermissionForm = () => {
  permissionForm.id = undefined
  permissionForm.roleId = undefined
  permissionForm.instanceId = undefined
  permissionForm.permissions = [DATABASE_PERMISSION.VIEW]
  permissionFormRef.value?.clearValidate()
}

const openPermissionDialog = (row?: any) => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权配置实例权限')
    return
  }
  resetPermissionForm()
  if (row?.id) {
    permissionForm.id = row.id
    permissionForm.roleId = row.roleId
    permissionForm.instanceId = row.instanceId
    permissionForm.permissions = permissionMaskToBits(row.permissions)
  }
  permissionDialogVisible.value = true
}

const submitPermissionForm = async () => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权配置实例权限')
    return
  }
  if (!permissionFormRef.value) return
  await permissionFormRef.value.validate()
  if (
    permissionForm.permissions.includes(DATABASE_PERMISSION.QUERY_UNLIMITED) &&
    !permissionForm.permissions.includes(DATABASE_PERMISSION.QUERY)
  ) {
    ElMessage.warning('不限行数需要同时授予查询权限')
    return
  }
  if (
    permissionForm.permissions.includes(DATABASE_PERMISSION.WRITE_EXPLAIN) &&
    !permissionForm.permissions.includes(DATABASE_PERMISSION.QUERY)
  ) {
    ElMessage.warning('写 SQL 计划需要同时授予查询权限')
    return
  }
  if (
    permissionForm.permissions.includes(DATABASE_PERMISSION.DDL) &&
    !permissionForm.permissions.includes(DATABASE_PERMISSION.QUERY)
  ) {
    ElMessage.warning('DDL 变更需要同时授予查询权限')
    return
  }
  permissionSubmitting.value = true
  try {
    await upsertDatabaseInstancePermission({
      roleId: permissionForm.roleId!,
      instanceId: permissionForm.instanceId!,
      permissions: permissionBitsToMask(permissionForm.permissions)
    })
    ElMessage.success('实例权限已保存')
    permissionDialogVisible.value = false
    await Promise.all([loadInstancePermissions(), loadInstances(), loadInstanceOptions()])
  } finally {
    permissionSubmitting.value = false
  }
}

const handleDeletePermission = async (row: any) => {
  if (!canManageInstancePermissions.value) {
    ElMessage.warning('无权配置实例权限')
    return
  }
  await ElMessageBox.confirm(`确定删除「${row.roleName || '-'}」对「${row.instanceName || '-'}」的实例权限吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await deleteDatabaseInstancePermission(row.id)
  ElMessage.success('实例权限已删除')
  await Promise.all([loadInstancePermissions(), loadInstances(), loadInstanceOptions()])
}

const resetQuery = () => {
  query.page = 1
  query.pageSize = 10
  query.keyword = ''
  query.dbType = ''
  query.status = ''
  query.environment = ''
  loadInstances()
}

const resetForm = () => {
  form.id = undefined
  form.name = ''
  form.dbType = 'mysql'
  form.host = ''
  form.port = 3306
  form.defaultDatabase = ''
  form.credentialId = 0
  form.tlsEnabled = false
  form.connectionParams = ''
  form.status = 'enabled'
  form.environment = ''
  form.businessSystem = ''
  form.owner = ''
  form.tags = ''
  form.remark = ''
  formRef.value?.clearValidate()
}

const openInstanceDialog = (row?: any) => {
  if (row?.id && !hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)) {
    ElMessage.warning('无实例管理权限')
    return
  }
  resetForm()
  if (row?.id) {
    form.id = row.id
    form.name = row.name || ''
    form.dbType = row.dbType || 'mysql'
    form.host = row.host || ''
    form.port = row.port || defaultPort(form.dbType)
    form.defaultDatabase = row.defaultDatabase || ''
    form.credentialId = row.credentialId || 0
    form.tlsEnabled = !!row.tlsEnabled
    form.connectionParams = row.connectionParams || ''
    form.status = row.status || 'enabled'
    form.environment = row.environment || ''
    form.businessSystem = row.businessSystem || ''
    form.owner = row.owner || ''
    form.tags = row.tags || ''
    form.remark = row.remark || ''
  }
  dialogVisible.value = true
}

const submitForm = async () => {
  if (!formRef.value) return
  await formRef.value.validate()
  submitting.value = true
  try {
    const payload: DatabaseInstancePayload = {
      name: form.name,
      dbType: form.dbType,
      host: form.host,
      port: form.port,
      defaultDatabase: form.defaultDatabase,
      credentialId: form.credentialId,
      tlsEnabled: form.tlsEnabled,
      connectionParams: form.connectionParams,
      status: form.status,
      environment: form.environment,
      businessSystem: form.businessSystem,
      owner: form.owner,
      tags: form.tags,
      remark: form.remark
    }
    if (form.id) {
      await updateDatabaseInstance(form.id, payload)
      ElMessage.success('更新成功')
    } else {
      await createDatabaseInstance(payload)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await Promise.all([loadInstances(), loadInstanceOptions()])
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: any) => {
  if (!hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)) {
    ElMessage.warning('无实例管理权限')
    return
  }
  await ElMessageBox.confirm(`确定删除数据库实例「${row.name}」吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await deleteDatabaseInstance(row.id)
  ElMessage.success('删除成功')
  await Promise.all([loadInstances(), loadInstanceOptions()])
}

const toggleInstanceStatus = async (row: any) => {
  if (!hasDatabasePermission(row, DATABASE_PERMISSION.MANAGE)) {
    ElMessage.warning('无实例管理权限')
    return
  }
  if (row.status === 'enabled') {
    await disableDatabaseInstance(row.id)
    ElMessage.success('已禁用')
  } else {
    await enableDatabaseInstance(row.id)
    ElMessage.success('已启用')
  }
  await Promise.all([loadInstances(), loadInstanceOptions()])
}

const handleTest = async (row: any) => {
  if (!canUseDatabaseFeature(row, DATABASE_PERMISSION.MANAGE, 'testEnabled')) {
    ElMessage.warning('无连接测试权限或该数据库类型暂未接入')
    return
  }
  testingId.value = row.id
  try {
    const res: any = await testDatabaseInstance(row.id)
    ElMessage.success(`${res.message || '连接测试成功'}${res.version ? `，版本：${res.version}` : ''}`)
    await Promise.all([loadInstances(), loadInstanceOptions()])
  } finally {
    testingId.value = 0
  }
}

const handleSync = async (row?: any) => {
  const id = row?.id || metadataInstanceId.value
  if (!id) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  const instance = row || instanceOptions.value.find(item => item.id === id)
  if (!canUseDatabaseFeature(instance, DATABASE_PERMISSION.MANAGE, 'metadataEnabled')) {
    ElMessage.warning('无元数据同步权限或该数据库类型暂未接入')
    return
  }
  syncingId.value = id
  try {
    const res: any = await syncDatabaseMetadata(id)
    if (instance?.dbType === 'redis') {
      ElMessage.success(
        `${res.message || '同步成功'}：${res.schemasCount || 0} 个逻辑 DB，${res.tablesCount || 0} 个 Key 样本`
      )
    } else {
      ElMessage.success(
        `${res.message || '同步成功'}：${res.schemasCount || 0} 个库，${res.tablesCount || 0} 张表，${res.columnsCount || 0} 个字段`
      )
    }
    await Promise.all([loadInstances(), loadInstanceOptions()])
    if (metadataInstanceId.value === id || !metadataInstanceId.value) {
      metadataInstanceId.value = id
      await loadSchemas()
    }
  } finally {
    syncingId.value = 0
  }
}

const handleMetadataInstanceChange = async () => {
  resetMetadataSelection()
  await loadSchemas()
}

const handleSchemaClick = async (data: any) => {
  selectedSchema.value = data.schemaName
  selectedTable.value = undefined
  columns.value = []
  indexes.value = []
  await loadTables(data.schemaName)
}

const handleTableClick = async (row: any) => {
  selectedTable.value = row
  currentDDL.value = undefined
  await loadTableDetails(row)
}

const closeTableContextMenu = () => {
  tableContextMenu.visible = false
  tableContextMenu.row = undefined
}

const handleTableContextMenu = async (row: any, _column: any, event: MouseEvent) => {
  event.preventDefault()
  event.stopPropagation()
  if (!row) return
  selectedTable.value = row
  currentDDL.value = undefined
  tableContextMenu.row = row
  tableContextMenu.x = event.clientX
  tableContextMenu.y = event.clientY
  tableContextMenu.visible = true
  await loadTableDetails(row)
}

const quoteQueryIdentifier = (dbType: string | undefined, value: string) => {
  const text = String(value || '').trim()
  if (!text) return text
  switch (dbType) {
    case 'postgresql':
    case 'opengauss':
    case 'kingbase':
    case 'oracle':
      return `"${text.replace(/"/g, '""')}"`
    case 'sqlserver':
      return `[${text.replace(/]/g, ']]')}]`
    default:
      return `\`${text.replace(/`/g, '``')}\``
  }
}

const qualifiedQueryTableName = (dbType: string | undefined, schemaName: string, tableName: string) => {
  const quotedTable = quoteQueryIdentifier(dbType, tableName)
  if (!String(schemaName || '').trim()) return quotedTable
  return `${quoteQueryIdentifier(dbType, schemaName)}.${quotedTable}`
}

const buildTablePreviewSQL = (instance: any, table: any, unlimitedRows: boolean) => {
  const dbType = instance?.dbType || ''
  const qualifiedName = qualifiedQueryTableName(dbType, table?.schemaName || '', table?.tableName || '')
  if (unlimitedRows) {
    return `SELECT * FROM ${qualifiedName}`
  }
  if (dbType === 'sqlserver') {
    return `SELECT TOP (100) * FROM ${qualifiedName}`
  }
  if (dbType === 'oracle') {
    return `SELECT * FROM ${qualifiedName} FETCH FIRST 100 ROWS ONLY`
  }
  return `SELECT * FROM ${qualifiedName} LIMIT 100`
}

const handlePreviewContextTable = async (unlimitedRows: boolean) => {
  const table = tableContextMenu.row
  closeTableContextMenu()
  if (!metadataInstanceId.value || !table?.tableName) {
    ElMessage.warning('请先选择一张表')
    return
  }
  if (isRedisMetadataInstance.value) {
    ElMessage.warning('Redis Key 暂不支持表内容预览')
    return
  }
  if (!canUseDatabaseFeature(currentMetadataInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled')) {
    ElMessage.warning('无查询权限或该数据库类型暂未接入查询控制台')
    return
  }
  if (unlimitedRows && !hasDatabasePermission(currentMetadataInstance.value, DATABASE_PERMISSION.QUERY_UNLIMITED)) {
    ElMessage.warning('无不限行数查询权限')
    return
  }
  if (unlimitedRows) {
    try {
      await ElMessageBox.confirm('该操作可能返回大量数据，确认继续？', '不限行数查询确认', {
        type: 'warning',
        confirmButtonText: '继续',
        cancelButtonText: '取消'
      })
    } catch {
      return
    }
  }

  queryInstanceId.value = metadataInstanceId.value
  await loadQuerySchemas()
  querySchemaName.value = table.schemaName || ''
  queryLimit.value = 100
  queryUnlimitedRows.value = unlimitedRows
  querySQL.value = buildTablePreviewSQL(currentMetadataInstance.value, table, unlimitedRows)
  clearQueryConsoleState()
  activeTab.value = 'query'
  await executeQuery({ skipUnlimitedConfirm: true })
}

const handlePreviewDDL = async () => {
  if (!metadataInstanceId.value || !selectedTable.value?.tableName) {
    ElMessage.warning('请先选择一张表')
    return
  }
  ddlLoading.value = true
  try {
    currentDDL.value = await getDatabaseTableDDL(metadataInstanceId.value, {
      schemaName: selectedTable.value.schemaName,
      tableName: selectedTable.value.tableName
    })
    ddlDialogVisible.value = true
  } finally {
    ddlLoading.value = false
  }
}

const handleExportDictionary = async () => {
  if (!metadataInstanceId.value || !selectedTable.value?.tableName) {
    ElMessage.warning('请先选择一张表')
    return
  }
  if (!hasDatabasePermission(currentMetadataInstance.value, DATABASE_PERMISSION.EXPORT)) {
    ElMessage.warning('无数据字典导出权限')
    return
  }
  dictionaryExporting.value = true
  try {
    const blob = await exportDatabaseTableDictionary(metadataInstanceId.value, {
      schemaName: selectedTable.value.schemaName,
      tableName: selectedTable.value.tableName
    }) as Blob
    downloadBlob(
      blob,
      `database-dictionary-${selectedTable.value.schemaName || 'default'}-${selectedTable.value.tableName}-${formatFileTimestamp()}.csv`
    )
  } finally {
    dictionaryExporting.value = false
  }
}

const handleQueryInstanceChange = async () => {
  querySchemaName.value = ''
  queryUnlimitedRows.value = false
  clearQueryConsoleState()
  await loadQuerySchemas()
  await loadQueryHistory()
}

const handleDiagnosisInstanceChange = async () => {
  resetDiagnosisState()
  await loadDiagnosisData()
  await renderCapacityChart()
}

const handleCapacityRangeChange = async () => {
  await loadCapacityTrend()
  await renderCapacityChart()
}

const handleTopologyInstanceChange = async () => {
  topologyResult.value = undefined
  await loadTopology()
}

const buildQueryPayload = (limit: number, options?: { unlimitedRows?: boolean }): DatabaseQueryPayload => ({
  schemaName: querySchemaName.value,
  sqlText: querySQL.value,
  limit: options?.unlimitedRows ? undefined : limit,
  timeoutSeconds: queryTimeoutSeconds.value,
  unlimitedRows: !!options?.unlimitedRows
})

const buildWriteValidatePayload = () => ({
  schemaName: querySchemaName.value,
  sqlText: querySQL.value
})

type ExplainRoute = 'readonly' | 'write' | 'ddl' | 'analyze' | 'unsupported'

const sqlKeywordsForRouting = (sqlText: string) => {
  const masked = sqlText
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/--.*$/gm, ' ')
    .replace(/'(?:''|[^'])*'/g, ' ')
    .replace(/"(?:\\"|[^"])*"/g, ' ')
    .replace(/`(?:``|[^`])*`/g, ' ')
  return (masked.match(/[a-zA-Z_][a-zA-Z0-9_]*/g) || []).map(item => item.toLowerCase())
}

const classifyExplainRoute = (sqlText: string): ExplainRoute => {
  const keywords = sqlKeywordsForRouting(sqlText)
  if (keywords.length === 0) return 'unsupported'
  if (keywords[0] === 'explain' && keywords[1] === 'analyze') return 'analyze'
  const target = keywords[0] === 'explain' ? keywords[1] : keywords[0]
  if (!target) return 'unsupported'
  if (['select', 'with'].includes(target)) return 'readonly'
  if (['insert', 'update', 'delete', 'replace', 'merge'].includes(target)) return 'write'
  if (['create', 'alter', 'drop', 'truncate', 'rename', 'grant', 'revoke'].includes(target)) return 'ddl'
  return 'unsupported'
}

const executeQuery = async (options?: { skipUnlimitedConfirm?: boolean }) => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled')) {
    ElMessage.warning('无查询权限或该数据库类型暂未接入查询控制台')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning(isRedisQueryInstance.value ? '请输入 Redis 命令' : '请输入 SQL')
    return
  }
  if (queryUnlimitedRows.value && !canUseQueryUnlimitedRows.value) {
    queryUnlimitedRows.value = false
    ElMessage.warning('无不限行数查询权限')
    return
  }
  if (queryUnlimitedRows.value && !options?.skipUnlimitedConfirm) {
    try {
      await ElMessageBox.confirm('该操作可能返回大量数据，确认继续？', '不限行数查询确认', {
        type: 'warning',
        confirmButtonText: '继续',
        cancelButtonText: '取消'
      })
    } catch {
      return
    }
  }
  queryRunning.value = true
  try {
    clearWriteConsoleState()
    queryResult.value = await executeDatabaseQuery(queryInstanceId.value, buildQueryPayload(queryLimit.value, {
      unlimitedRows: queryUnlimitedRows.value
    }))
    queryResultTab.value = 'result'
    ElMessage.success(
      isRedisQueryInstance.value
        ? `命令执行成功，返回 ${queryResult.value?.rowsReturned || 0} 行`
        : `查询成功，返回 ${queryResult.value?.rowsReturned || 0} 行`
    )
  } finally {
    queryRunning.value = false
  }
}

const handleFormatQuery = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled')) {
    ElMessage.warning('无查询权限或该数据库类型暂未接入查询控制台')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryFormatting.value = true
  try {
    const res: any = await formatDatabaseQuery(queryInstanceId.value, { sqlText: querySQL.value })
    querySQL.value = res.sqlText || querySQL.value
    ElMessage.success('SQL 已格式化')
  } finally {
    queryFormatting.value = false
  }
}

const handleExplainQuery = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.QUERY, 'queryEnabled')) {
    ElMessage.warning('无查询权限或该数据库类型暂未接入执行计划')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  const explainRoute = classifyExplainRoute(querySQL.value)
  if (explainRoute === 'analyze') {
    ElMessage.warning('执行计划仅支持 EXPLAIN，不允许 EXPLAIN ANALYZE')
    return
  }
  if (explainRoute === 'ddl') {
    ElMessage.warning('DDL 不支持执行计划，请使用 DDL 检查')
    return
  }
  if (explainRoute === 'write') {
    if (!databaseConfig.writeExplainEnabled) {
      ElMessage.warning('写 SQL 执行计划开关未开启')
      return
    }
    if (!canUseWriteExplainCurrentQueryInstance.value) {
      ElMessage.warning('无写 SQL 执行计划权限')
      return
    }
  }
  queryExplaining.value = true
  try {
    clearWriteConsoleState()
    explainResult.value = explainRoute === 'write'
      ? await explainDatabaseWriteQuery(queryInstanceId.value, buildQueryPayload(queryLimit.value))
      : await explainDatabaseQuery(queryInstanceId.value, buildQueryPayload(queryLimit.value))
    explainDialogVisible.value = true
  } finally {
    queryExplaining.value = false
  }
}

const handleExportQuery = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canUseDatabaseFeature(currentQueryInstance.value, DATABASE_PERMISSION.EXPORT, 'queryEnabled')) {
    ElMessage.warning('无导出权限或该数据库类型暂未接入查询导出')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryExporting.value = true
  try {
    clearWriteConsoleState()
    const blob = await exportDatabaseQueryResult(queryInstanceId.value, buildQueryPayload(queryExportLimit.value)) as Blob
    downloadBlob(blob, `database-query-result-${formatFileTimestamp()}.csv`)
    ElMessage.success('查询结果已导出')
  } finally {
    queryExporting.value = false
  }
}

const handleValidateWriteQuery = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canWriteCurrentQueryInstance.value) {
    ElMessage.warning('该数据库类型暂未接入受控写操作')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryWriteChecking.value = true
  try {
    clearReadOnlyConsoleState()
    writeResult.value = undefined
    ddlCheckResult.value = undefined
    writeCheckResult.value = await validateDatabaseWriteQuery(queryInstanceId.value, buildWriteValidatePayload())
    queryResultTab.value = 'check'
    if (writeCheckResult.value?.allowed) {
      ElMessage.success(writeCheckResult.value.message || '写操作预检查通过')
      return
    }
    ElMessage.warning(writeCheckResult.value?.message || '写操作预检查未通过')
  } finally {
    queryWriteChecking.value = false
  }
}

const handlePrepareWriteExecute = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canWriteCurrentQueryInstance.value) {
    ElMessage.warning('该数据库类型暂未接入受控写操作')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryWritePreparing.value = true
  try {
    clearReadOnlyConsoleState()
    writeResult.value = undefined
    ddlCheckResult.value = undefined
    const result = await validateDatabaseWriteQuery(queryInstanceId.value, buildWriteValidatePayload())
    writeCheckResult.value = result
    queryResultTab.value = 'check'
    if (!result?.allowed) {
      ElMessage.warning(result?.message || '写操作预检查未通过')
      return
    }
    resetWriteConfirmForm()
    writeConfirmForm.confirmed = !result.confirmRequired
    writeConfirmVisible.value = true
  } finally {
    queryWritePreparing.value = false
  }
}

const handleExecuteWrite = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canWriteCurrentQueryInstance.value) {
    ElMessage.warning('该数据库类型暂未接入受控写操作')
    return
  }
  if (!writeCheckResult.value?.allowed) {
    ElMessage.warning('请先完成写操作预检查')
    return
  }
  if (writeCheckResult.value.reasonRequired && !writeConfirmForm.reason.trim()) {
    ElMessage.warning('请填写操作原因')
    return
  }
  if (writeCheckResult.value.confirmRequired && !writeConfirmForm.confirmed) {
    ElMessage.warning('请确认高风险写操作')
    return
  }
  queryWriteSubmitting.value = true
  try {
    writeResult.value = await executeDatabaseWriteQuery(queryInstanceId.value, {
      schemaName: querySchemaName.value,
      sqlText: querySQL.value,
      reason: writeConfirmForm.reason.trim(),
      confirmed: writeConfirmForm.confirmed,
      timeoutSeconds: queryTimeoutSeconds.value
    })
    queryResultTab.value = writeResult.value?.executedSql ? 'sql' : 'audit'
    writeConfirmVisible.value = false
    await loadQueryAudits()
    ElMessage.success(writeResult.value?.message || '写操作执行成功')
  } finally {
    queryWriteSubmitting.value = false
  }
}

const handleValidateDDLQuery = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canDDLCurrentQueryInstance.value) {
    ElMessage.warning('无 DDL 权限或该数据库类型暂未接入结构变更')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryDDLChecking.value = true
  try {
    clearReadOnlyConsoleState()
    writeCheckResult.value = undefined
    writeResult.value = undefined
    ddlCheckResult.value = await validateDatabaseDDLQuery(queryInstanceId.value, buildWriteValidatePayload())
    queryResultTab.value = 'check'
    if (ddlCheckResult.value?.allowed) {
      ElMessage.success(ddlCheckResult.value.message || 'DDL 结构变更检查通过')
      return
    }
    ElMessage.warning(ddlCheckResult.value?.message || 'DDL 结构变更检查未通过')
  } finally {
    queryDDLChecking.value = false
  }
}

const handlePrepareDDLExecute = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canDDLCurrentQueryInstance.value) {
    ElMessage.warning('无 DDL 权限或该数据库类型暂未接入结构变更')
    return
  }
  if (!databaseConfig.ddlEnabled) {
    ElMessage.warning('DDL 结构变更开关未开启')
    return
  }
  if (!querySQL.value.trim()) {
    ElMessage.warning('请输入 SQL')
    return
  }
  queryDDLPreparing.value = true
  try {
    clearReadOnlyConsoleState()
    writeCheckResult.value = undefined
    writeResult.value = undefined
    const result = await validateDatabaseDDLQuery(queryInstanceId.value, buildWriteValidatePayload())
    ddlCheckResult.value = result
    queryResultTab.value = 'check'
    if (!result?.allowed) {
      ElMessage.warning(result?.message || 'DDL 结构变更检查未通过')
      return
    }
    resetDDLConfirmForm()
    ddlConfirmForm.confirmed = !result.confirmRequired
    ddlConfirmVisible.value = true
  } finally {
    queryDDLPreparing.value = false
  }
}

const handleExecuteDDL = async () => {
  if (!queryInstanceId.value) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!canDDLCurrentQueryInstance.value) {
    ElMessage.warning('无 DDL 权限或该数据库类型暂未接入结构变更')
    return
  }
  if (!ddlCheckResult.value?.allowed) {
    ElMessage.warning('请先完成 DDL 结构变更检查')
    return
  }
  if (ddlCheckResult.value.reasonRequired && !ddlConfirmForm.reason.trim()) {
    ElMessage.warning('请填写 DDL 结构变更原因')
    return
  }
  if (ddlCheckResult.value.confirmRequired && !ddlConfirmForm.confirmed) {
    ElMessage.warning('请确认 DDL 结构变更风险')
    return
  }
  queryDDLSubmitting.value = true
  try {
    writeResult.value = await executeDatabaseDDLQuery(queryInstanceId.value, {
      schemaName: querySchemaName.value,
      sqlText: querySQL.value,
      reason: ddlConfirmForm.reason.trim(),
      confirmed: ddlConfirmForm.confirmed,
      timeoutSeconds: queryTimeoutSeconds.value
    })
    queryResultTab.value = writeResult.value?.executedSql ? 'sql' : 'audit'
    ddlConfirmVisible.value = false
    ddlCheckResult.value = undefined
    await loadQueryAudits()
    ElMessage.success(writeResult.value?.message || 'DDL 结构变更执行成功')
  } finally {
    queryDDLSubmitting.value = false
  }
}

const loadQueryHistory = async () => {
  queryHistoryLoading.value = true
  try {
    const params: any = {
      keyword: queryHistoryQuery.keyword,
      limit: queryHistoryQuery.limit
    }
    if (queryHistoryQuery.currentInstanceOnly && queryInstanceId.value) {
      params.instanceId = queryInstanceId.value
      if (querySchemaName.value) {
        params.schemaName = querySchemaName.value
      }
    }
    const res: any = await listDatabaseQueryHistory(params)
    queryHistoryItems.value = res.list || []
    if (res.limit) queryHistoryQuery.limit = res.limit
  } finally {
    queryHistoryLoading.value = false
  }
}

const openQueryHistory = async () => {
  queryHistoryDialogVisible.value = true
  await loadQueryHistory()
}

const applyHistoryQuery = async (row: any) => {
  if (!row?.sqlText) return
  if (row.instanceId && row.instanceId !== queryInstanceId.value) {
    queryInstanceId.value = row.instanceId
    await loadQuerySchemas()
  }
  if (row.schemaName) {
    querySchemaName.value = row.schemaName
  } else if (!querySchemaName.value) {
    querySchemaName.value = defaultQuerySchemaName()
  }
  querySQL.value = row.sqlText
  clearQueryConsoleState()
  queryHistoryDialogVisible.value = false
  ElMessage.success('已填入 SQL 编辑器')
}

const resetQueryConsole = () => {
  querySQL.value = ''
  queryUnlimitedRows.value = false
  clearQueryConsoleState()
}

const resetAuditQuery = () => {
  auditQuery.page = 1
  auditQuery.pageSize = 10
  auditQuery.keyword = ''
  auditQuery.instanceId = undefined
  auditQuery.action = ''
  auditQuery.status = ''
  auditQuery.riskLevel = ''
  auditQuery.sqlType = ''
  loadQueryAudits()
}

const resetBackupTaskQuery = () => {
  backupTaskQuery.page = 1
  backupTaskQuery.pageSize = 10
  backupTaskQuery.keyword = ''
  backupTaskQuery.instanceId = undefined
  backupTaskQuery.enabled = ''
  loadBackupTasks()
}

const resetProtectionProfileQuery = () => {
  protectionProfileQuery.page = 1
  protectionProfileQuery.pageSize = 10
  protectionProfileQuery.keyword = ''
  protectionProfileQuery.instanceId = undefined
  protectionProfileQuery.engine = ''
  protectionProfileQuery.protectionLevel = ''
  protectionProfileQuery.riskLevel = ''
  loadProtectionProfiles()
}

const resetProtectionRiskQuery = () => {
  protectionRiskQuery.page = 1
  protectionRiskQuery.pageSize = 10
  protectionRiskQuery.keyword = ''
  protectionRiskQuery.instanceId = undefined
  protectionRiskQuery.engine = ''
  protectionRiskQuery.riskLevel = ''
  protectionRiskQuery.issueType = ''
  protectionRiskQuery.productionOnly = ''
  loadProtectionRisks()
}

const resetBackupPolicyQuery = () => {
  backupPolicyQuery.page = 1
  backupPolicyQuery.pageSize = 10
  backupPolicyQuery.keyword = ''
  backupPolicyQuery.instanceId = undefined
  backupPolicyQuery.status = ''
  backupPolicyQuery.enabled = ''
  loadBackupPolicies()
}

const resetBackupRecordQuery = () => {
  backupRecordQuery.page = 1
  backupRecordQuery.pageSize = 10
  backupRecordQuery.taskId = undefined
  backupRecordQuery.instanceId = undefined
  backupRecordQuery.status = ''
  backupRecordQuery.triggerType = ''
  loadBackupRecords()
}

const resetStorageProfileQuery = () => {
  storageProfileQuery.page = 1
  storageProfileQuery.pageSize = 10
  storageProfileQuery.keyword = ''
  storageProfileQuery.storageType = ''
  storageProfileQuery.status = ''
  loadStorageProfiles()
}

const resetLogArchiveStreamQuery = () => {
  logArchiveStreamQuery.page = 1
  logArchiveStreamQuery.pageSize = 10
  logArchiveStreamQuery.instanceId = undefined
  logArchiveStreamQuery.archiveType = ''
  logArchiveStreamQuery.status = ''
  loadLogArchiveStreams()
}

const resetLogArchiveQuery = () => {
  logArchiveQuery.page = 1
  logArchiveQuery.pageSize = 10
  logArchiveQuery.streamId = undefined
  logArchiveQuery.instanceId = undefined
  logArchiveQuery.archiveType = ''
  logArchiveQuery.status = ''
  loadLogArchives()
}

const resetLogArchiveEventQuery = () => {
  logArchiveEventQuery.page = 1
  logArchiveEventQuery.pageSize = 20
  logArchiveEventQuery.streamId = undefined
  logArchiveEventQuery.instanceId = undefined
  logArchiveEventQuery.runnerHostId = undefined
  logArchiveEventQuery.level = ''
  logArchiveEventQuery.eventType = ''
  loadLogArchiveEvents()
}

const resetRestorePlanQuery = () => {
  restorePlanQuery.page = 1
  restorePlanQuery.pageSize = 10
  restorePlanQuery.sourceInstanceId = undefined
  restorePlanQuery.targetInstanceId = undefined
  restorePlanQuery.validationStatus = ''
  restorePlanQuery.restoreStatus = ''
  loadRestorePlans()
}

const resetBarmanCatalogQuery = () => {
  barmanCatalogQuery.page = 1
  barmanCatalogQuery.pageSize = 10
  barmanCatalogQuery.instanceId = undefined
  barmanCatalogQuery.backupMethod = 'physical'
  barmanCatalogQuery.backupEngine = 'barman'
  barmanCatalogQuery.backupScope = 'cluster'
  barmanCatalogQuery.status = ''
  loadBarmanCatalogRecords()
}

const resetWalStatusQuery = () => {
  walStatusQuery.page = 1
  walStatusQuery.pageSize = 100
  walStatusQuery.streamId = undefined
  walStatusQuery.instanceId = undefined
  loadWalStatusArchives()
}

const barmanCatalogSyncTag = (row: DatabaseBackupRecordResult) => {
  const engine = normalizeToolEngineValue(row.backupEngine)
  if (engine === 'pg_basebackup') {
    return { tag: 'primary' as const, text: '本地' }
  }
  if (engine === 'walg' || engine === 'wal_g' || isPgBackRestEngineValue(engine)) {
    return { tag: 'warning' as const, text: 'external' }
  }
  if (engine === 'barman' && row.externalBackupId) {
    return { tag: 'success' as const, text: '已同步' }
  }
  if (row.externalBackupId) {
    return { tag: 'warning' as const, text: '已登记' }
  }
  return { tag: 'info' as const, text: '未同步' }
}

interface WalStatusSegmentRow extends DatabaseLogArchiveResult {
  gapBefore?: boolean
  timelineSwitch?: boolean
}

interface WalStatusGroup {
  streamId: number
  instanceName: string
  segments: WalStatusSegmentRow[]
  gapCount: number
  timelineCount: number
  timeRange: string
  lsnRange: string
}

const walStatusGroups = computed<WalStatusGroup[]>(() => {
  const grouped = new Map<number, WalStatusSegmentRow[]>()
  for (const item of walStatusArchives.value) {
    const sid = item.streamId || 0
    if (!grouped.has(sid)) grouped.set(sid, [])
    grouped.get(sid)!.push({ ...item })
  }
  const result: WalStatusGroup[] = []
  for (const [streamId, list] of grouped.entries()) {
    list.sort((a, b) => {
      const tlA = String(a.timelineId || '')
      const tlB = String(b.timelineId || '')
      if (tlA !== tlB) return tlA.localeCompare(tlB)
      const segA = Number(a.segmentNo) || 0
      const segB = Number(b.segmentNo) || 0
      return segA - segB
    })
    let gapCount = 0
    const timelineSet = new Set<string>()
    let prevSegment: number | null = null
    let prevTimeline: string | null = null
    for (const segment of list) {
      const timeline = String(segment.timelineId || '')
      if (timeline) timelineSet.add(timeline)
      const segNo = Number(segment.segmentNo)
      if (timeline && prevTimeline !== null && timeline !== prevTimeline) {
        segment.timelineSwitch = true
        prevSegment = null
      } else if (Number.isFinite(segNo) && prevSegment !== null && segNo - prevSegment > 1) {
        segment.gapBefore = true
        gapCount += 1
      }
      if (Number.isFinite(segNo)) prevSegment = segNo
      if (timeline) prevTimeline = timeline
    }
    const first = list[0]
    const last = list[list.length - 1]
    const timeRange = first || last
      ? `${first?.firstEventTime || '-'} → ${last?.lastEventTime || '-'}`
      : '-'
    let lsnRange = ''
    const startLsn = first?.startLsn
    const endLsn = last?.endLsn
    if (startLsn || endLsn) {
      lsnRange = `${startLsn || '-'} → ${endLsn || '-'}`
    }
    result.push({
      streamId,
      instanceName: first?.instanceName || '',
      segments: list,
      gapCount,
      timelineCount: timelineSet.size,
      timeRange,
      lsnRange
    })
  }
  result.sort((a, b) => a.streamId - b.streamId)
  return result
})

const resetRunnerHostQuery = () => {
  runnerHostQuery.page = 1
  runnerHostQuery.pageSize = 10
  runnerHostQuery.keyword = ''
  runnerHostQuery.runnerType = ''
  runnerHostQuery.status = ''
  runnerHostQuery.enabled = ''
  loadRunnerHosts()
}

const resetRunnerJobQuery = () => {
  runnerJobQuery.page = 1
  runnerJobQuery.pageSize = 10
  runnerJobQuery.runnerHostId = undefined
  runnerJobQuery.jobType = ''
  runnerJobQuery.status = ''
  loadRunnerJobs()
}

const resetBarmanServerQuery = () => {
  barmanServerQuery.page = 1
  barmanServerQuery.pageSize = 10
  barmanServerQuery.keyword = ''
  barmanServerQuery.sourceInstanceId = undefined
  barmanServerQuery.runnerHostId = undefined
  barmanServerQuery.status = ''
  loadBarmanServers()
}

const resetRestoreJobQuery = () => {
  restoreJobQuery.page = 1
  restoreJobQuery.pageSize = 10
  restoreJobQuery.sourceInstanceId = undefined
  restoreJobQuery.targetInstanceId = undefined
  restoreJobQuery.status = ''
  loadRestoreJobs()
}

const loadInspectionReports = async () => {
  inspectionLoading.value = true
  try {
    const res: any = await listDatabaseInspectionReports(inspectionQuery)
    inspectionReports.value = res.list || []
    inspectionTotal.value = res.total || 0
    if (res.page) inspectionQuery.page = res.page
    if (res.pageSize) inspectionQuery.pageSize = res.pageSize
  } finally {
    inspectionLoading.value = false
  }
}

const resetInspectionQuery = () => {
  inspectionQuery.page = 1
  inspectionQuery.pageSize = 10
  inspectionQuery.instanceId = undefined
  inspectionQuery.status = ''
  inspectionQuery.riskLevel = ''
  loadInspectionReports()
}

const ensureInspectionDefaultInstance = () => {
  if (!inspectionForm.instanceId && diagnosisInstances.value.length > 0) {
    inspectionForm.instanceId = diagnosisInstances.value.find(item => item.status === 'enabled')?.id || diagnosisInstances.value[0].id
  }
}

const handleGenerateInspectionReport = async () => {
  ensureInspectionDefaultInstance()
  if (!inspectionForm.instanceId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  inspectionGenerating.value = true
  try {
    const res = await generateDatabaseInspectionReport({ instanceId: inspectionForm.instanceId }) as DatabaseInspectionReportResult
    currentInspectionReport.value = res
    inspectionDetailVisible.value = true
    ElMessage.success(res.summary || '巡检报告已生成')
    await Promise.all([loadInspectionReports(), loadQueryAudits()])
  } finally {
    inspectionGenerating.value = false
  }
}

const openInspectionDetail = async (row: DatabaseInspectionReportResult) => {
  currentInspectionReport.value = row
  inspectionDetailVisible.value = true
  if (!row?.id) return
  inspectionLoading.value = true
  try {
    currentInspectionReport.value = await getDatabaseInspectionReport(row.id) as DatabaseInspectionReportResult
  } finally {
    inspectionLoading.value = false
  }
}

const inspectionSections = (report?: DatabaseInspectionReportResult): DatabaseInspectionSection[] =>
  [
    report?.capacitySummary,
    report?.performanceSummary,
    report?.securitySummary,
    report?.backupSummary
  ].filter((item): item is DatabaseInspectionSection => !!item)

const formatCapacityBytes = (value?: number | null) => {
  const size = Number(value || 0)
  if (size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatCapacityXAxisLabel = (_value: string | number, index: number) => {
  const point = capacityTrendPoints.value[index]
  if (!point) return ''
  if (capacityRange.value === '24h') {
    return point.collectedAt.slice(11, 16)
  }
  if (capacityRange.value === '30d') {
    return point.collectedAt.slice(5, 10)
  }
  const previous = index > 0 ? capacityTrendPoints.value[index - 1] : null
  if (!previous || previous.collectedAt.slice(0, 10) !== point.collectedAt.slice(0, 10)) {
    return point.collectedAt.slice(5, 10)
  }
  return point.collectedAt.slice(11, 16)
}

const buildCapacityTooltip = (params: any) => {
  const items = Array.isArray(params) ? params : [params]
  const item = items[0]
  if (!item || typeof item.dataIndex !== 'number') return ''
  const point = capacityTrendPoints.value[item.dataIndex]
  if (!point) return ''
  return [
    point.collectedAt,
    `${item.marker || ''}${isRedisDiagnosisInstance.value ? '内存' : '容量'}: ${formatCapacityBytes(point.totalSizeBytes)}`
  ].join('<br/>')
}

const renderCapacityChart = async () => {
  await nextTick()
  if (activeTab.value !== 'diagnosis' || !capacityChartRef.value) {
    capacityChart?.dispose()
    capacityChart = null
    return
  }

  if (!capacityTrendPoints.value.length) {
    capacityChart?.clear()
    return
  }

  if (capacityChart && capacityChart.getDom() !== capacityChartRef.value) {
    capacityChart.dispose()
    capacityChart = null
  }

  if (!capacityChart) {
    capacityChart = echarts.init(capacityChartRef.value)
  }

  capacityChart.setOption({
    animationDuration: 320,
    grid: {
      left: 64,
      right: 24,
      top: 28,
      bottom: 54
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      padding: [10, 12],
      textStyle: {
        color: '#f8fafc'
      },
      formatter: buildCapacityTooltip
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: capacityTrendPoints.value.map(point => point.collectedAt),
      axisLine: {
        lineStyle: {
          color: '#cbd5e1'
        }
      },
      axisLabel: {
        color: '#64748b',
        interval: 'auto',
        formatter: formatCapacityXAxisLabel
      }
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLine: {
        show: false
      },
      axisTick: {
        show: false
      },
      axisLabel: {
        color: '#64748b',
        formatter: (value: number) => formatCapacityBytes(value)
      },
      splitLine: {
        lineStyle: {
          color: '#e2e8f0',
          type: 'dashed'
        }
      }
    },
    series: [
      {
        name: isRedisDiagnosisInstance.value ? '内存' : '容量',
        type: 'line',
        smooth: true,
        showSymbol: capacityTrendPoints.value.length <= 12,
        symbolSize: 7,
        data: capacityTrendPoints.value.map(point => point.totalSizeBytes),
        lineStyle: {
          width: 3,
          color: '#2563eb'
        },
        itemStyle: {
          color: '#1d4ed8',
          borderColor: '#dbeafe',
          borderWidth: 2
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(37, 99, 235, 0.24)' },
            { offset: 1, color: 'rgba(37, 99, 235, 0.03)' }
          ])
        }
      }
    ]
  }, { notMerge: true })

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      capacityChart?.resize()
    })
  })
}

const resizeCapacityChart = () => {
  capacityChart?.resize()
  topologyChart?.resize()
}

const topologyNodeName = (nodeId?: string) =>
  topologyResult.value?.nodes?.find(item => item.id === nodeId)?.name || ''

const topologyMetricLabel = (key: string) => {
  const labels: Record<string, string> = {
    instance_id: '实例 ID',
    replica_id: '副本关系 ID',
    check_id: '采集 ID',
    last_check_id: '最近采集 ID',
    role_detected: '检测角色',
    health_status: '健康状态',
    primary_instance_id: '主库实例 ID',
    replica_instance_id: '从库实例 ID',
    replica_role: '副本角色',
    discovery_source: '发现来源',
    source_host: '来源主机',
    source_port: '来源端口',
    source_server_uuid: '来源 Server UUID',
    seconds_behind_source: '复制延迟秒',
    configured_delay_seconds: '配置延迟秒',
    remaining_delay_seconds: '剩余延迟秒',
    replica_io_running: 'IO 线程',
    replica_sql_running: 'SQL 线程',
    io: 'IO 线程',
    sql: 'SQL 线程',
    server_id: 'server_id',
    server_uuid: 'server_uuid',
    version: '版本',
    read_only: 'read_only',
    super_read_only: 'super_read_only',
    log_bin: 'log_bin',
    gtid_mode: 'GTID 模式',
    binlog_format: 'binlog_format',
    binlog_row_image: 'binlog_row_image',
    last_checked_at: '最近采集时间',
    check_age_seconds: '采集年龄秒',
    check_freshness: '采集新鲜度',
    pg_write_lag_ms: 'PG write lag ms',
    pg_flush_lag_ms: 'PG flush lag ms',
    pg_replay_lag_ms: 'PG replay lag ms',
    pg_last_wal_replay_lsn: 'PG replay LSN',
    wal_receiver_status: 'WAL receiver',
    pg_is_wal_replay_paused: 'WAL replay paused',
    recovery_min_apply_delay: 'PG apply delay',
    report_host: '主库报告 Host',
    report_port: '主库报告 Port'
  }
  return labels[key] || key
}

const topologyFindingTag = (level?: string) => {
  switch (String(level || '').toLowerCase()) {
    case 'critical':
      return 'danger'
    case 'warning':
      return 'warning'
    case 'healthy':
      return 'success'
    default:
      return 'info'
  }
}

const topologyFindingLevelText = (level?: string) => {
  switch (String(level || '').toLowerCase()) {
    case 'critical':
      return '异常'
    case 'warning':
      return '警告'
    case 'healthy':
      return '健康'
    default:
      return '提示'
  }
}

const topologyGraphStateColor = (state?: string) => {
  switch (String(state || '').toLowerCase()) {
    case 'healthy':
    case 'ok':
    case 'online':
    case 'connected':
    case 'green':
      return '#16a34a'
    case 'warning':
    case 'yellow':
    case 'recovering':
    case 'initializing':
    case 'relocating':
      return '#d97706'
    case 'critical':
    case 'unhealthy':
    case 'fail':
    case 'failed':
    case 'red':
    case 'disconnected':
    case 'down':
      return '#dc2626'
    default:
      return '#94a3b8'
  }
}

const topologyGraphRoleColor = (node: DatabaseTopologyNode) => {
  const role = String(node.role || '').toLowerCase()
  if (['primary', 'master'].includes(role)) return '#15803d'
  if (['delayed_replica', 'delayed_standby'].includes(role)) return '#ea580c'
  if (['replica', 'slave', 'secondary', 'standby'].includes(role)) return '#2563eb'
  if (['sentinel', 'arbiter'].includes(role)) return '#ca8a04'
  return '#64748b'
}

const topologyGraphSymbol = (node: DatabaseTopologyNode) => {
  const role = String(node.role || '').toLowerCase()
  if (['primary', 'master'].includes(role)) return 'roundRect'
  if (['delayed_replica', 'delayed_standby'].includes(role)) return 'diamond'
  if (['sentinel', 'arbiter'].includes(role)) return 'triangle'
  return 'circle'
}

const isRelationalTopologyResult = (result?: DatabaseTopologyResult) =>
  ['mysql', 'mariadb', 'postgresql'].includes(String(result?.dbType || '').toLowerCase())

const topologyGraphPosition = (
  node: DatabaseTopologyNode,
  allNodes: DatabaseTopologyNode[],
  width: number,
  height: number
) => {
  const role = String(node.role || '').toLowerCase()
  const isPrimary = ['primary', 'master'].includes(role)
  const isDelayed = ['delayed_replica', 'delayed_standby'].includes(role)
  const isReplica = ['replica', 'slave', 'secondary', 'standby'].includes(role)
  const group = allNodes.filter((item) => {
    const itemRole = String(item.role || '').toLowerCase()
    if (isPrimary) return ['primary', 'master'].includes(itemRole)
    if (isDelayed) return ['delayed_replica', 'delayed_standby'].includes(itemRole)
    if (isReplica) return ['replica', 'slave', 'secondary', 'standby'].includes(itemRole)
    return !['primary', 'master', 'replica', 'slave', 'secondary', 'standby', 'delayed_replica', 'delayed_standby'].includes(itemRole)
  })
  const index = Math.max(group.findIndex(item => item.id === node.id), 0)
  const stepY = (top: number, bottom: number) => {
    if (group.length <= 1) return (top + bottom) / 2
    return top + ((bottom - top) * index) / (group.length - 1)
  }
  if (isPrimary) return { x: width * 0.28, y: height * 0.5 }
  if (isDelayed) return { x: width * 0.7, y: stepY(height * 0.58, height * 0.82) }
  if (isReplica) return { x: width * 0.7, y: stepY(height * 0.22, height * 0.46) }
  return { x: width * 0.48, y: stepY(height * 0.18, height * 0.82) }
}

const formatSecondsText = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) return '-'
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
}

const topologyGraphEdgeLabel = (link: DatabaseTopologyLink) => {
  const parts: string[] = []
  const label = String(link.label || '').toLowerCase()
  if (label.includes('delayed')) {
    parts.push('delayed')
  } else if (label.includes('streaming')) {
    parts.push('streaming')
  } else if (label.includes('async')) {
    parts.push('async')
  } else if (link.label) {
    parts.push(link.label)
  }
  if (link.lagText) {
    parts.push(link.lagText)
  } else if (link.metrics?.seconds_behind_source) {
    parts.push(`${link.metrics.seconds_behind_source}s`)
  }
  if (link.metrics?.remaining_delay_seconds) {
    parts.push(`remain ${formatSecondsText(Number(link.metrics.remaining_delay_seconds))}`)
  }
  const io = link.metrics?.io || link.metrics?.replica_io_running
  const sql = link.metrics?.sql || link.metrics?.replica_sql_running
  if (io || sql) {
    parts.push(`IO ${io || '-'} / SQL ${sql || '-'}`)
  }
  if (link.metrics?.wal_receiver_status) {
    parts.push(link.metrics.wal_receiver_status)
  }
  return parts.filter(Boolean).slice(0, 4).join(' · ')
}

const topologyGraphTooltip = (params: any) => {
  const data = params?.data || {}
  if (params?.dataType === 'edge') {
    return [
      `<strong>${data.sourceName || topologyNodeName(data.source) || data.source}</strong> -> <strong>${data.targetName || topologyNodeName(data.target) || data.target}</strong>`,
      data.label ? `关系：${data.label}` : '',
      data.state ? `状态：${data.state}` : '',
      data.lagText ? `延迟：${data.lagText}` : '',
      data.message ? `说明：${data.message}` : ''
    ].filter(Boolean).join('<br/>')
  }
  return [
    `<strong>${data.displayName || data.name}</strong>`,
    data.roleText ? `角色：${data.roleText}` : '',
    data.address ? `地址：${data.address}` : '',
    data.state ? `状态：${data.state}` : '',
    data.lagText ? `延迟：${data.lagText}` : '',
    data.message ? `说明：${data.message}` : ''
  ].filter(Boolean).join('<br/>')
}

const renderTopologyChart = async () => {
  await nextTick()
  if (activeTab.value !== 'topology' || !topologyChartRef.value) {
    topologyChart?.dispose()
    topologyChart = null
    return
  }
  const result = topologyResult.value
  const nodes = result?.nodes || []
  if (!nodes.length) {
    topologyChart?.clear()
    return
  }
  if (topologyChart && topologyChart.getDom() !== topologyChartRef.value) {
    topologyChart.dispose()
    topologyChart = null
  }
  if (!topologyChart) {
    topologyChart = echarts.init(topologyChartRef.value)
  }
  const chartWidth = topologyChartRef.value.clientWidth || 900
  const chartHeight = topologyChartRef.value.clientHeight || 340
  const useRelationalLayout = isRelationalTopologyResult(result)
  const graphNodes = nodes.map((node) => {
    const stateColor = topologyGraphStateColor(node.state)
    const position = useRelationalLayout ? topologyGraphPosition(node, nodes, chartWidth, chartHeight) : undefined
    return {
      name: node.id,
      displayName: node.name || node.id,
      id: node.id,
      roleText: node.roleText || node.role,
      address: node.address,
      state: node.state,
      lagText: node.lagText,
      message: node.message,
      metrics: node.metrics || {},
      x: position?.x,
      y: position?.y,
      fixed: useRelationalLayout,
      symbol: topologyGraphSymbol(node),
      symbolSize: ['primary', 'master'].includes(String(node.role || '').toLowerCase()) ? 72 : 58,
      itemStyle: {
        color: topologyGraphRoleColor(node),
        borderColor: stateColor,
        borderWidth: ['warning', 'critical'].includes(String(node.state || '').toLowerCase()) ? 4 : 2
      },
      label: {
        show: true,
        formatter: () => `${node.name || node.id}\n${node.roleText || node.role || '-'}`,
        color: '#0f172a',
        fontSize: 12,
        lineHeight: 16
      }
    }
  })
  const graphLinks = (result?.links || []).map((link: DatabaseTopologyLink) => ({
    source: link.source,
    target: link.target,
    sourceName: link.sourceName,
    targetName: link.targetName,
    label: link.label,
    state: link.state,
    lagText: link.lagText,
    message: link.message,
    metrics: link.metrics || {},
    displayLabel: topologyGraphEdgeLabel(link),
    lineStyle: {
      color: topologyGraphStateColor(link.state),
      width: String(link.state || '').toLowerCase() === 'critical' ? 3 : 2,
      curveness: 0.16
    },
    labelLayout: {
      hideOverlap: true
    }
  }))
  topologyChart.setOption({
    animationDuration: 320,
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      padding: [10, 12],
      textStyle: {
        color: '#f8fafc'
      },
      formatter: topologyGraphTooltip
    },
    series: [
      {
        type: 'graph',
        layout: useRelationalLayout ? 'none' : 'force',
        roam: true,
        draggable: true,
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 9],
        data: graphNodes,
        links: graphLinks,
        force: useRelationalLayout ? undefined : {
          repulsion: 460,
          edgeLength: [120, 220],
          gravity: 0.08
        },
        lineStyle: {
          opacity: 0.86
        },
        edgeLabel: {
          show: true,
          color: '#475569',
          fontSize: 11,
          formatter: (params: any) => params.data?.displayLabel || params.data?.lagText || params.data?.label || ''
        },
        emphasis: {
          focus: 'adjacency',
          lineStyle: {
            width: 4
          }
        }
      }
    ]
  }, { notMerge: true })
  topologyChart.off('click')
  topologyChart.on('click', (params: any) => {
    if (params?.dataType === 'edge') {
      const link = (result?.links || []).find(item => item.source === params.data?.source && item.target === params.data?.target)
      if (link) openTopologyLinkDetail(link)
      return
    }
    const node = nodes.find(item => item.id === params?.data?.name || item.id === params?.data?.id)
    if (node) openTopologyNodeDetail(node)
  })
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      topologyChart?.resize()
    })
  })
}

const openAuditDetail = (row: any) => {
  currentAudit.value = row
  auditDetailVisible.value = true
}

const handleDeleteBackupTask = async (row: DatabaseBackupTaskResult) => {
  await ElMessageBox.confirm(`确定删除备份任务「${row.name}」吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await deleteDatabaseBackupTask(row.id)
  if (backupRecordQuery.taskId === row.id) {
    backupRecordQuery.taskId = undefined
  }
  ElMessage.success('备份任务已删除')
  await Promise.all([loadBackupTasks(), loadBackupRecords()])
}

const handleDeleteBackupPolicy = async (row: DatabaseBackupPolicyResult) => {
  await ElMessageBox.confirm(`确定删除备份策略「${row.name}」吗？备份记录不会被删除，但后续自动增量链不会再由该策略调度。`, '删除备份策略', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await deleteDatabaseBackupPolicy(row.id)
  ElMessage.success('备份策略已删除')
  await Promise.all([loadProtectionProfiles(), loadBackupPolicies()])
}

const handleRunBackupPolicy = async (row: DatabaseBackupPolicyResult, level: 'full' | 'incremental') => {
  const actionText = level === 'full' ? '全量备份' : '增量备份'
  await ElMessageBox.confirm(
    `确定手动触发策略「${row.name}」的${actionText}吗？该操作会下发 Runner 执行 MySQL/MariaDB 物理备份，并更新策略链状态。`,
    `触发${actionText}`,
    {
      type: 'warning',
      confirmButtonText: '触发',
      cancelButtonText: '取消'
    }
  )
  runningBackupPolicyId.value = row.id
  runningBackupPolicyLevel.value = level
  try {
    const reason = `manual ${level} from backup policy page`
    const res: any = level === 'full'
      ? await runDatabaseBackupPolicyFull(row.id, { reason })
      : await runDatabaseBackupPolicyIncremental(row.id, { reason })
    ElMessage.success(res?.message || `${actionText}已下发 Runner`)
    await Promise.all([loadProtectionProfiles(), loadBackupPolicies(), loadBackupRecords(), loadRunnerJobs()])
    window.setTimeout(() => {
      loadProtectionProfiles()
      loadBackupPolicies()
      loadBackupRecords()
      loadRunnerJobs()
    }, 3000)
  } finally {
    runningBackupPolicyId.value = 0
    runningBackupPolicyLevel.value = ''
  }
}

const handleValidateBackupPolicyChain = async (row: DatabaseBackupPolicyResult) => {
  validatingBackupPolicyId.value = row.id
  try {
    const res = await validateDatabaseBackupPolicyChain(row.id) as DatabaseBackupPolicyChainValidationResult
    if (res.blockingReasons?.length) {
      ElMessage.error(res.blockingReasons[0] || '备份链校验失败')
    } else if (res.warnings?.length) {
      ElMessage.warning(res.warnings[0] || '备份链校验有风险')
    } else {
      ElMessage.success(res.messages?.[0] || '备份链校验通过')
    }
    await Promise.all([loadProtectionProfiles(), loadBackupPolicies()])
  } finally {
    validatingBackupPolicyId.value = 0
  }
}

const handleValidateProtectionProfile = async (row: DatabaseProtectionProfileResult) => {
  if (!row.profileId) return
  validatingProtectionProfileId.value = row.profileId
  try {
    const res = await validateDatabaseProtectionProfile(row.profileId) as DatabaseProtectionProfileResult
    if (res.riskLevel === 'critical' || res.riskLevel === 'high') {
      ElMessage.warning(res.riskMessages?.[0] || '保护状态存在风险')
    } else {
      ElMessage.success('保护状态校验完成')
    }
    await loadProtectionProfiles()
  } finally {
    validatingProtectionProfileId.value = ''
  }
}

const handleRunProtectionProfileBackup = async (row: DatabaseProtectionProfileResult, level: 'full' | 'incremental') => {
  if (!row.backupPolicy?.id) {
    ElMessage.warning('当前保护策略没有可执行的物理备份策略，请先进入高级资源创建或修复策略')
    return
  }
  await handleRunBackupPolicy(row.backupPolicy, level)
}

const openProtectionProfileResources = async (row: DatabaseProtectionProfileResult) => {
  backupPitrTab.value = 'advancedResources'
  backupAdvancedTab.value = row.engine === 'postgresql' ? 'barmanServers' : row.backupPolicy ? 'backupPolicies' : 'streams'
  backupPolicyQuery.instanceId = row.instanceId
  logArchiveStreamQuery.instanceId = row.instanceId
  logArchiveQuery.instanceId = row.instanceId
  barmanServerQuery.sourceInstanceId = row.engine === 'postgresql' ? row.instanceId : undefined
  barmanCatalogQuery.instanceId = row.engine === 'postgresql' ? row.instanceId : undefined
  walStatusQuery.instanceId = row.engine === 'postgresql' ? row.instanceId : undefined
  restorePlanQuery.sourceInstanceId = row.instanceId
  await Promise.all([loadBackupPolicies(), loadLogArchiveStreams(), loadLogArchives(), loadRestorePlans(), loadBarmanServers(), loadBarmanCatalogRecords(), loadWalStatusArchives()])
}

const protectionProfileFromRisk = async (row: DatabaseProtectionRiskResult): Promise<DatabaseProtectionProfileResult> => {
  let profile = protectionProfiles.value.find(item => item.profileId === row.profileId || item.instanceId === row.instanceId)
  if (!profile) {
    protectionProfileQuery.instanceId = row.instanceId
    await loadProtectionProfiles()
    profile = protectionProfiles.value.find(item => item.profileId === row.profileId || item.instanceId === row.instanceId)
  }
  return profile || ({
    profileId: row.profileId,
    instanceId: row.instanceId,
    instanceName: row.instanceName,
    engine: row.engine,
    engineText: row.engineText,
    version: '',
    endpoint: row.endpoint,
    environment: row.environment,
    businessSystem: row.businessSystem,
    owner: row.owner,
    protectionMode: row.protectionMode,
    protectionModeText: row.protectionModeText,
    protectionLevel: row.protectionLevel,
    protectionLevelText: row.protectionLevelText,
    healthStatus: '',
    healthStatusText: '',
    riskLevel: row.riskLevel,
    riskLevelText: row.riskLevelText,
    riskMessages: [row.message],
    recoverableFrom: '',
    recoverableUntil: row.recoverableUntil,
    rpoLagSeconds: -1,
    lastFullAt: row.lastFullAt,
    lastIncrementalAt: '',
    lastSyntheticAt: '',
    lastLogArchiveAt: row.lastLogArchiveAt,
    lastRestoreDrillAt: '',
    restoreDrillStatus: '',
    restoreDrillStatusText: '',
    runnerStatus: '',
    runnerStatusText: '',
    storageStatus: '',
    storageStatusText: '',
    backupChainStatus: '',
    backupChainStatusText: '',
    logChainStatus: '',
    logChainStatusText: '',
    replicaProtectionStatus: '',
    replicaProtectionText: '',
    recommendedActions: [],
    validatedAt: row.checkedAt
  } as DatabaseProtectionProfileResult)
}

const handleOpenProtectionRisk = async (row: DatabaseProtectionRiskResult) => {
  const profile = await protectionProfileFromRisk(row)
  switch (row.issueType) {
    case 'missing_full_backup':
      if (profile.engine === 'postgresql') await openPostgresBarmanWizardDialog(profile)
      else await openProtectionWizardDialog(profile)
      return
    case 'restore_drill_missing':
    case 'restore_drill_failed':
      openProtectionRestoreDrillDialog(profile)
      return
    case 'runner_offline':
    case 'runner_tool_missing':
      backupPitrTab.value = 'advancedResources'
      backupAdvancedTab.value = 'runnerHosts'
      runnerHostQuery.keyword = profile.runnerHost?.name || ''
      await loadRunnerHosts()
      return
    case 'storage_posture_failed':
      backupPitrTab.value = 'advancedResources'
      backupAdvancedTab.value = 'storageProfiles'
      await loadStorageProfiles()
      return
    case 'log_chain_gap':
    case 'archive_lag_high':
      backupPitrTab.value = 'advancedResources'
      backupAdvancedTab.value = profile.engine === 'postgresql' ? 'walStatus' : 'streams'
      logArchiveStreamQuery.instanceId = row.instanceId
      logArchiveQuery.instanceId = row.instanceId
      walStatusQuery.instanceId = row.instanceId
      await Promise.all([loadLogArchiveStreams(), loadLogArchives(), loadWalStatusArchives()])
      return
    case 'replica_delay_unavailable':
      activeTab.value = 'replication'
      replicationProtectionQuery.instanceId = row.instanceId
      await loadReplicationProtections()
      return
    default:
      await openProtectionProfileResources(profile)
  }
}

const openProtectionProfileRestorePlan = (row: DatabaseProtectionProfileResult) => {
  resetRestorePlanForm()
  restorePlanForm.sourceInstanceId = row.instanceId
  restorePlanForm.targetInstanceId = pitrRestoreTargetInstances.value.find(item => item.id !== row.instanceId)?.id
  restorePlanForm.restoreTargetType = 'time'
  restorePlanForm.restoreTargetValue = row.recoverableUntil || formatDateTimeInput()
  restorePlanForm.restoreMode = 'isolated_restore'
  restorePlanDialogVisible.value = true
}

const openSyntheticFullPreview = async (row: DatabaseBackupPolicyResult) => {
  syntheticPreviewPolicy.value = row
  syntheticPreview.value = undefined
  previewingSyntheticPolicyId.value = row.id
  try {
    const res = await previewDatabaseBackupPolicySyntheticFull(row.id) as DatabaseSyntheticFullPreviewResult
    syntheticPreview.value = res
    syntheticPreviewVisible.value = true
  } finally {
    previewingSyntheticPolicyId.value = 0
  }
}

const handleRunSyntheticFull = async (row: DatabaseBackupPolicyResult) => {
  await ElMessageBox.confirm(
    `确定手动触发策略「${row.name}」的合成全量吗？Runner 会读取当前备份链 artifact，完成 prepare/merge 后生成新的 synthetic full 基线。`,
    '触发合成全量',
    {
      type: 'warning',
      confirmButtonText: '触发',
      cancelButtonText: '取消'
    }
  )
  runningSyntheticPolicyId.value = row.id
  try {
    const res: any = await runDatabaseBackupPolicySyntheticFull(row.id, { reason: 'manual synthetic full from backup policy page' })
    ElMessage.success(res?.message || '合成全量已下发 Runner')
    syntheticPreviewVisible.value = false
    await Promise.all([loadBackupPolicies(), loadBackupRecords(), loadRunnerJobs()])
    window.setTimeout(() => {
      loadBackupPolicies()
      loadBackupRecords()
      loadRunnerJobs()
    }, 3000)
  } finally {
    runningSyntheticPolicyId.value = 0
  }
}

const openBackupPolicyPurgePreview = async (row: DatabaseBackupPolicyResult) => {
  purgePreviewPolicy.value = row
  purgePreview.value = undefined
  previewingPurgePolicyId.value = row.id
  try {
    const res = await previewDatabaseBackupPolicyPurge(row.id) as DatabaseBackupPolicyPurgePreviewResult
    purgePreview.value = res
    purgePreviewVisible.value = true
    if (res.blockingReasons?.length) {
      ElMessage.warning(res.blockingReasons[0] || '旧链暂不可清理')
    }
    await Promise.all([loadBackupPolicies(), loadBackupRecords()])
  } finally {
    previewingPurgePolicyId.value = 0
  }
}

const handleRunBackupPolicyPurge = async (row: DatabaseBackupPolicyResult) => {
  if (!purgePreview.value || purgePreview.value.policyId !== row.id) {
    await openBackupPolicyPurgePreview(row)
  }
  if (purgePreview.value?.blockingReasons?.length) {
    ElMessage.error(purgePreview.value.blockingReasons[0] || '旧链暂不可清理')
    return
  }
  if (!purgePreview.value?.eligibleRecordIds?.length) {
    ElMessage.warning('没有到达清理窗口的旧链记录')
    return
  }
  await ElMessageBox.confirm(
    `确定将策略「${row.name}」的 ${purgePreview.value.eligibleRecordIds.length} 条旧链记录标记为 expired/purged 吗？本操作不会在 backend 中硬删除 runner 或对象存储 artifact，但会改变备份记录状态。`,
    '清理旧链确认',
    {
      type: 'warning',
      confirmButtonText: '标记清理',
      cancelButtonText: '取消'
    }
  )
  runningPurgePolicyId.value = row.id
  try {
    const res = await runDatabaseBackupPolicyPurge(row.id, { reason: 'manual synthetic purge from backup policy page' }) as DatabaseBackupPolicyPurgeRunResult
    ElMessage.success(res.message || `已标记清理 ${res.purgedRecordIds?.length || 0} 条旧链记录`)
    purgePreviewVisible.value = false
    await Promise.all([loadBackupPolicies(), loadBackupRecords()])
  } finally {
    runningPurgePolicyId.value = 0
  }
}

const handleRunBackupTask = async (row: DatabaseBackupTaskResult) => {
  await ElMessageBox.confirm(
    `确定手动触发备份任务「${row.name}」吗？系统会创建一条队列记录并在后台执行逻辑全量备份，所有动作会写入统一审计。`,
    '手动触发确认',
    {
      type: 'warning',
      confirmButtonText: '触发',
      cancelButtonText: '取消'
    }
  )
  runningBackupTaskId.value = row.id
  try {
    const res = await runDatabaseBackupTask(row.id) as DatabaseBackupRunResult
    ElMessage.success(res.message || '备份任务已进入队列，可在备份记录中刷新查看进度')
  } finally {
    runningBackupTaskId.value = 0
    await Promise.all([loadBackupTasks(), loadBackupRecords(), loadQueryAudits()])
  }
}

const handleDownloadBackupRecord = async (row: DatabaseBackupRecordResult) => {
  const blob = await downloadDatabaseBackupRecord(row.id) as Blob
  downloadBlob(blob, row.fileName || `database-backup-${row.id}.sql.gz`)
  ElMessage.success('备份文件已开始下载')
}

const handleVerifyBackupRecord = async (row: DatabaseBackupRecordResult) => {
  verifyingBackupRecordId.value = row.id
  try {
    await verifyDatabaseBackupRecord(row.id)
    ElMessage.success('备份文件校验通过')
  } finally {
    verifyingBackupRecordId.value = 0
    await Promise.all([loadBackupRecords(), loadQueryAudits()])
  }
}

const openRestoreDialog = (row: DatabaseBackupRecordResult) => {
  restoreSourceRecord.value = row
  restoreForm.targetInstanceId = 0
  restoreForm.restoreMode = 'dry_run'
  restoreForm.restoreStrategy = row.backupType === 'logical' && restoreSourceDbType.value === 'postgresql'
    ? 'database_clean'
    : 'object_replace'
  normalizeRestoreFormStrategy()
  const firstTarget = restoreTargetOptions.value.find(item => item.id !== row.instanceId)
  if (firstTarget) {
    restoreForm.targetInstanceId = firstTarget.id
  }
  restoreDialogVisible.value = true
}

const submitRestoreDryRun = async () => {
  if (!restoreFormRef.value || !restoreSourceRecord.value?.id) return
  await restoreFormRef.value.validate()
  if (!restoreForm.targetInstanceId) {
    ElMessage.warning('请选择目标实例')
    return
  }
  normalizeRestoreFormStrategy()
  const target = instanceOptions.value.find(item => item.id === restoreForm.targetInstanceId)
  const strategyText = availableRestoreStrategyOptions.value.find(item => item.value === restoreForm.restoreStrategy)?.label || restoreForm.restoreStrategy
  await ElMessageBox.confirm(
    `确定把备份「${restoreSourceRecord.value.fileName}」恢复演练到「${target?.name || restoreForm.targetInstanceId}」吗？目标处理：${strategyText}。`,
    '恢复演练确认',
    {
      type: 'warning',
      confirmButtonText: '开始演练',
      cancelButtonText: '取消'
    }
  )
  restoreSubmitting.value = true
  try {
    const res = await runDatabaseRestoreDryRun(restoreSourceRecord.value.id, {
      targetInstanceId: restoreForm.targetInstanceId,
      restoreMode: restoreForm.restoreMode,
      restoreStrategy: restoreForm.restoreStrategy
    }) as DatabaseRestoreJobResult
    restoreDialogVisible.value = false
    ElMessage.success(res.message || '恢复演练已完成')
  } finally {
    restoreSubmitting.value = false
    await Promise.all([loadRestoreJobs(), loadQueryAudits()])
  }
}

const handleExportAudits = async () => {
  auditExporting.value = true
  try {
    const blob = await exportDatabaseQueryAudits(auditQuery) as Blob
    downloadBlob(blob, `database-query-audits-${formatFileTimestamp()}.csv`)
  } finally {
    auditExporting.value = false
  }
}

const handleTypeChange = (dbType: string) => {
  form.port = defaultPort(dbType)
}

const defaultPort = (dbType: string) => {
  const item = supportedTypes.value.find(type => type.type === dbType)
  return item?.defaultPort || 0
}

const isProductionEnvironment = (environment?: string) =>
  ['prod', 'production', 'prd', '生产', '生产环境'].includes((environment || '').trim().toLowerCase())

const isRestoreCompatibleType = (sourceType?: string, targetType?: string) => {
  const source = (sourceType || '').trim().toLowerCase()
  const target = (targetType || '').trim().toLowerCase()
  if (!source || !target) return false
  if (source === 'postgresql' || target === 'postgresql') {
    return source === 'postgresql' && target === 'postgresql'
  }
  return ['mysql', 'mariadb'].includes(source) && ['mysql', 'mariadb'].includes(target)
}

const defaultQuerySchemaName = () => {
  if (currentQueryInstance.value?.dbType === 'redis') {
    return querySchemas.value[0]?.schemaName || currentQueryInstance.value?.defaultDatabase || 'db0'
  }
  if (['postgresql', 'sqlserver', 'oracle'].includes(currentQueryInstance.value?.dbType || '')) {
    return querySchemas.value[0]?.schemaName || ''
  }
  return currentQueryInstance.value?.defaultDatabase || querySchemas.value[0]?.schemaName || ''
}

const dbTypeTag = (dbType: string) => {
  switch (dbType) {
    case 'mysql':
    case 'mariadb':
      return 'primary'
    case 'postgresql':
      return 'success'
    case 'sqlserver':
      return 'warning'
    case 'clickhouse':
      return 'success'
    case 'oracle':
      return 'danger'
    case 'redis':
      return 'danger'
    case 'mongodb':
      return 'warning'
    case 'elasticsearch':
    case 'opensearch':
      return 'success'
    case 'tidb':
    case 'oceanbase':
      return 'primary'
    case 'opengauss':
    case 'kingbase':
      return 'success'
    case 'dameng':
      return 'warning'
    default:
      return 'info'
  }
}

const topologyRoleTag = (role: string) => {
  const normalized = String(role || '').toLowerCase()
  if (['master', 'primary'].includes(normalized)) return 'success'
  if (['replica', 'slave', 'secondary', 'standby'].includes(normalized)) return 'primary'
  if (['delayed_replica', 'delayed_standby'].includes(normalized)) return 'warning'
  if (normalized === 'arbiter') return 'warning'
  return 'info'
}

const topologyStateTag = (state: string) => {
  const normalized = String(state || '').toLowerCase()
  if (['ok', 'online', 'connected', 'healthy', 'started', 'green'].includes(normalized)) return 'success'
  if (['warning', 'yellow', 'recovering', 'initializing', 'relocating'].includes(normalized)) return 'warning'
  if (['critical', 'unhealthy', 'fail', 'failed', 'red', 'disconnected', 'down'].includes(normalized)) return 'danger'
  return 'info'
}

const replicationHealthTag = (status?: string) => {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'warning':
      return 'warning'
    case 'critical':
      return 'danger'
    default:
      return 'info'
  }
}

const replicationRoleTag = (role?: string) => {
  switch (role) {
    case 'delayed_replica':
      return 'warning'
    case 'realtime_replica':
    case 'replica':
    case 'standby':
      return 'primary'
    case 'primary':
      return 'success'
    default:
      return 'info'
  }
}

const replicaApplyStateTag = (state?: string) => {
  switch (state) {
    case 'running':
      return 'success'
    case 'paused':
      return 'danger'
    default:
      return 'info'
  }
}

const replicaActionTag = (action?: string) => {
  switch (action) {
    case 'pause_apply':
      return 'danger'
    case 'resume_apply':
      return 'success'
    default:
      return 'info'
  }
}

const replicationProtectionStatusTag = (status?: string) => {
  switch (status) {
    case 'protected':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'unprotected':
      return 'danger'
    default:
      return 'info'
  }
}

const replicationRiskText = (messages?: string[]) => {
  if (!messages || !messages.length) return '当前无明显风险'
  return messages.join('；')
}

const replicationDelayText = (value?: number) => {
  const seconds = Number(value ?? -1)
  if (seconds < 0) return '-'
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  if (minutes < 60) return rest ? `${minutes}m ${rest}s` : `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const minuteRest = minutes % 60
  return minuteRest ? `${hours}h ${minuteRest}m` : `${hours}h`
}

const auditStatusTag = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'denied':
      return 'warning'
    case 'pending':
      return 'info'
    default:
      return 'info'
  }
}

const backupStatusTag = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'verified':
      return 'success'
    case 'restored':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    case 'queued':
      return 'info'
    case 'planned':
      return 'info'
    case 'cancelled':
      return 'info'
    case 'cleaning':
      return 'warning'
    case 'cleaned':
      return 'success'
    case 'pending':
      return 'info'
    default:
      return 'info'
  }
}

const backupPolicyStatusTag = (status?: string) => {
  switch (status) {
    case 'active':
    case 'healthy':
      return 'success'
    case 'degraded':
    case 'consolidating':
      return 'warning'
    case 'failed':
    case 'broken':
      return 'danger'
    case 'disabled':
      return 'info'
    default:
      return 'info'
  }
}

const backupVerifyStatusTag = (status?: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'expired':
      return 'info'
    case 'pending':
      return 'warning'
    default:
      return 'info'
  }
}

const storagePostureStatusTag = (status?: string) => {
  switch (status) {
    case 'passed':
      return 'success'
    case 'warning':
    case 'unsupported':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}

const storagePostureStatusText = (status?: string) => {
  switch (status) {
    case 'passed':
      return '通过'
    case 'warning':
      return '有风险'
    case 'failed':
      return '失败'
    case 'unsupported':
      return '需人工核验'
    default:
      return '未检测'
  }
}

const logArchiveStatusTag = (status?: string) => {
  switch (status) {
    case 'archived':
    case 'running':
    case 'passed':
    case 'success':
      return 'success'
    case 'missing':
    case 'checksum_failed':
    case 'failed':
      return 'danger'
    case 'degraded':
    case 'warning':
      return 'warning'
    case 'disabled':
    case 'paused':
    case 'expired':
      return 'info'
    default:
      return 'info'
  }
}

const logArchiveEventLevelTag = (level?: string) => {
  switch (level) {
    case 'error':
      return 'danger'
    case 'warning':
      return 'warning'
    default:
      return 'info'
  }
}

const logArchiveDesiredStateTag = (state?: string) => {
  switch (state) {
    case 'running':
      return 'success'
    case 'paused':
      return 'warning'
    case 'stopped':
      return 'info'
    default:
      return 'info'
  }
}

const logArchiveDaemonStatusTag = (status?: string) => {
  switch (status) {
    case 'running':
      return 'success'
    case 'starting':
    case 'paused':
    case 'degraded':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}

const formatLogArchiveCursor = (row: DatabaseLogArchiveStreamResult) => {
  if (!row.cursorFile && !row.cursorPos) return '-'
  return `${row.cursorFile || '-'}:${row.cursorPos || 0}`
}

const formatLogArchiveLag = (row: DatabaseLogArchiveStreamResult) => {
  if (row.archiveLagSeconds === undefined || row.archiveLagSeconds === null) return '-'
  return `${row.archiveLagSeconds}s`
}

const restoreValidationStatusTag = (status?: string) => {
  switch (status) {
    case 'passed':
    case 'complete':
    case 'available':
    case 'compatible':
    case 'verified':
      return 'success'
    case 'failed':
    case 'missing_base':
    case 'missing_incremental':
    case 'broken_chain':
    case 'missing_binlog':
    case 'missing_wal':
    case 'timeline_gap':
    case 'gtid_gap':
    case 'time_range_gap':
    case 'missing_object':
    case 'checksum_failed':
    case 'incompatible_version':
    case 'missing_tool':
    case 'permission_denied':
      return 'danger'
    case 'warning':
    case 'unsupported':
      return 'warning'
    case 'running':
    case 'planned':
    case 'pending':
    case 'queued':
      return 'info'
    default:
      return 'info'
  }
}

const restorePlanArtifactReadiness = (row?: DatabaseRestorePlanResult) => {
  const fallback = { status: 'unknown', text: '-', tag: 'info', message: '恢复计划尚未生成 artifact 可读性矩阵', blocking: false }
  if (!row?.requiredArtifactJson) return fallback
  let artifacts: any[] = []
  try {
    const parsed = JSON.parse(row.requiredArtifactJson)
    artifacts = Array.isArray(parsed) ? parsed : []
  } catch {
    return { ...fallback, message: 'artifact 可读性矩阵 JSON 格式异常', blocking: true, tag: 'danger', text: '异常' }
  }
  if (!artifacts.length) return fallback
  const statuses = artifacts.map(item => String(item.readinessStatus || '').trim()).filter(Boolean)
  if (!statuses.length) return fallback
  const blockingStatuses = new Set(['missing', 'unreadable', 'metadata_only'])
  const blocking = artifacts.filter(item => blockingStatuses.has(String(item.readinessStatus || '').trim()))
  if (blocking.length) {
    return {
      status: 'blocked',
      text: `不可读 ${blocking.length}`,
      tag: 'danger',
      blocking: true,
      message: blocking.map(item => `${item.fileName || item.id || '-'}: ${item.readinessMessage || item.readinessStatus}`).join('；')
    }
  }
  const pending = artifacts.filter(item => String(item.readinessStatus || '').trim() === 'pending_download')
  if (pending.length) {
    return {
      status: 'pending_download',
      text: `待拉取 ${pending.length}`,
      tag: 'warning',
      blocking: true,
      message: pending.map(item => `${item.fileName || item.id || '-'}: ${item.readinessMessage || '对象存储 artifact 尚未拉取到 Runner'}`).join('；')
    }
  }
  const unknown = artifacts.filter(item => String(item.readinessStatus || '').trim() === 'unknown')
  if (unknown.length) {
    return {
      status: 'unknown',
      text: `待确认 ${unknown.length}`,
      tag: 'info',
      blocking: false,
      message: unknown.map(item => `${item.fileName || item.id || '-'}: ${item.readinessMessage || '执行时确认 Runner'}`).join('；')
    }
  }
  const ready = artifacts.filter(item => String(item.readinessStatus || '').trim() === 'ready')
  if (ready.length === artifacts.length) {
    return { status: 'ready', text: `就绪 ${ready.length}`, tag: 'success', blocking: false, message: '所有 artifact 已登记为 Runner 可直接读取' }
  }
  return {
    status: 'managed',
    text: '托管',
    tag: 'info',
    blocking: false,
    message: artifacts.map(item => `${item.fileName || item.id || '-'}: ${item.readinessMessage || item.readinessStatus || '托管 artifact'}`).join('；')
  }
}

const runnerStatusTag = (status?: string) => {
  switch (status) {
    case 'online':
      return 'success'
    case 'failed':
      return 'danger'
    case 'disabled':
      return 'info'
    default:
      return 'warning'
  }
}

const runnerJobStatusTag = (status?: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    default:
      return 'info'
  }
}

const barmanStatusTag = (status?: string) => {
  switch (status) {
    case 'healthy':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'failed':
      return 'danger'
    case 'disabled':
      return 'info'
    default:
      return 'warning'
  }
}

const restoreStatusTag = (status?: string) => {
  switch (status) {
    case 'verified':
    case 'restored':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
    case 'queued':
      return 'warning'
    case 'cancelled':
      return 'info'
    default:
      return 'info'
  }
}

const runnerJobOutputSummary = (row: DatabaseRunnerJobResult) => {
  if (!row?.resultJson) return '-'
  try {
    const parsed = JSON.parse(row.resultJson)
    if (Array.isArray(parsed.binlogs) && parsed.binlogs.length) {
      const first = parsed.binlogs[0]
      const last = parsed.binlogs[parsed.binlogs.length - 1]
      const range = last?.fileName && last.fileName !== first?.fileName ? `${first?.fileName || '-'} - ${last.fileName}` : first?.fileName || '-'
      return `${parsed.binlogs.length} 个文件 / ${range}`
    }
    if (parsed.binlog?.fileName) {
      const parts = [parsed.binlog.fileName]
      if (parsed.binlog.fileSize) parts.push(formatBytes(Number(parsed.binlog.fileSize)))
      if (parsed.binlog.checksumSha256) parts.push(String(parsed.binlog.checksumSha256).slice(0, 12))
      return parts.join(' / ')
    }
    const stdout = String(parsed.stdout || '').trim().replace(/\s+/g, ' ')
    const stderr = String(parsed.stderr || '').trim().replace(/\s+/g, ' ')
    return stdout || stderr || row.resultJson
  } catch (_err) {
    return row.resultJson
  }
}

const recoverableWindowText = (row: DatabaseBackupRecordResult) => {
  if (!row.recoverableFrom && !row.recoverableUntil) return '-'
  return `${row.recoverableFrom || '-'} 至 ${row.recoverableUntil || '-'}`
}

const riskLevelTag = (riskLevel?: string) => {
  switch (riskLevel) {
    case 'critical':
      return 'danger'
    case 'high':
      return 'warning'
    case 'medium':
      return 'primary'
    case 'low':
      return 'success'
    default:
      return 'info'
  }
}

const healthScoreTag = (score?: number) => {
  const value = Number(score || 0)
  if (value >= 90) return 'success'
  if (value >= 80) return 'primary'
  if (value >= 60) return 'warning'
  return 'danger'
}

const healthScoreAlertType = (score?: number) => {
  const value = Number(score || 0)
  if (value >= 90) return 'success'
  if (value >= 80) return 'info'
  if (value >= 60) return 'warning'
  return 'error'
}

const sectionStatusTag = (status?: string) => {
  switch (status) {
    case 'success':
    case 'healthy':
    case 'normal':
      return 'success'
    case 'warning':
    case 'high':
      return 'warning'
    case 'critical':
    case 'danger':
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}

const findingSeverityTag = (severity?: string) => {
  switch (severity) {
    case 'critical':
      return 'danger'
    case 'warning':
    case 'high':
      return 'warning'
    case 'info':
      return 'info'
    default:
      return 'primary'
  }
}

const environmentText = (value: string) => {
  const map: Record<string, string> = {
    prod: '生产',
    staging: '预发',
    test: '测试',
    dev: '开发'
  }
  return map[value] || value
}

const formatNumber = (value?: number) => {
  return Number(value || 0).toLocaleString()
}

const formatBytes = (value?: number) => {
  const size = Number(value || 0)
  if (size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const formatQueryCell = (value: any) => {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const formatFileTimestamp = () => {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
}

const downloadBlob = (blob: Blob, filename: string) => {
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const handleDocumentClick = () => {
  closeTableContextMenu()
}

watch(
  () => [backupTaskForm.instanceId, selectedBackupTaskInstance.value?.dbType || ''],
  () => {
    const instanceDbType = selectedBackupTaskInstance.value?.dbType || ''
    if (instanceDbType || !backupTaskForm.instanceId) {
      backupTaskFormInstanceDbType.value = instanceDbType
    }
    normalizeBackupTaskFormBackupType()
  }
)

watch(
  () => backupTaskForm.backupMethod,
  () => normalizeBackupTaskFormBackupType()
)

watch(
  () => backupTaskForm.backupEngine,
  () => normalizeBackupTaskFormBackupType()
)

watch(
  () => [backupPolicyForm.instanceId, selectedBackupPolicyDbType.value],
  () => {
    if (!availableBackupPolicyEngineOptions.value.some(item => item.value === backupPolicyForm.backupEngine)) {
      backupPolicyForm.backupEngine = defaultPhysicalBackupPolicyEngine()
    }
    if (backupPolicyForm.binlogStreamId && !backupPolicyBinlogStreamOptions.value.some(item => item.id === backupPolicyForm.binlogStreamId)) {
      backupPolicyForm.binlogStreamId = undefined
    }
    if (!backupPolicyForm.name && selectedBackupPolicyInstance.value?.name) {
      backupPolicyForm.name = `${selectedBackupPolicyInstance.value.name}-物理增量策略`
    }
  }
)

watch(
  () => logArchiveStreamForm.instanceId,
  (instanceId) => {
    logArchiveStreamForm.archiveType = archiveTypeForInstance(instanceId)
    normalizeLogArchiveEngineForType()
  }
)

watch(
  () => logArchiveStreamForm.archiveType,
  () => normalizeLogArchiveEngineForType()
)

watch(
  () => restorePlanForm.sourceInstanceId,
  (sourceInstanceId) => {
    if (restorePlanForm.targetInstanceId === sourceInstanceId) {
      restorePlanForm.targetInstanceId = pitrRestoreTargetInstances.value.find(item => item.id !== sourceInstanceId)?.id
    }
    if (restorePlanSourceDbType.value !== 'postgresql' && restorePlanForm.restoreTargetType === 'lsn') {
      restorePlanForm.restoreTargetType = 'time'
      restorePlanForm.restoreTargetValue = formatDateTimeInput()
      restorePlanForm.targetTimelineId = ''
    }
  }
)

watch(
  () => restorePlanForm.restoreTargetType,
  (targetType) => {
    if (targetType === 'lsn') {
      restorePlanForm.restoreTargetValue = ''
      return
    }
    if (!restorePlanForm.restoreTargetValue || /^[0-9A-Fa-f]+\/[0-9A-Fa-f]+$/.test(String(restorePlanForm.restoreTargetValue))) {
      restorePlanForm.restoreTargetValue = formatDateTimeInput()
    }
  }
)

watch(activeTab, async (tab) => {
  if (tab === 'schemas') {
    await ensureMetadataInstance()
  }
  if (tab === 'query') {
    await ensureQueryInstance()
  }
  if (tab === 'diagnosis') {
    await ensureDiagnosisInstance()
  }
  if (tab === 'topology') {
    await ensureTopologyInstance()
  }
  if (tab === 'replication') {
    await refreshReplicationState()
  }
  if (tab === 'backup') {
    await Promise.all([
      loadProtectionProfiles(),
      loadBackupTasks(),
      loadBackupRecords(),
      loadStorageProfiles(),
      loadRunnerHosts(),
      loadRunnerJobs(),
      loadBarmanServers(),
      loadBackupPolicies(),
      loadLogArchiveStreams(),
      loadLogArchives(),
      loadLogArchiveEvents(),
      loadRestorePlans(),
      loadRestoreJobs(),
      loadBarmanCatalogRecords(),
      loadWalStatusArchives()
    ])
  }
  if (tab === 'permissions') {
    if (!canManageInstancePermissions.value) {
      activeTab.value = 'instances'
      return
    }
    await Promise.all([loadRoles(), loadInstancePermissions()])
  }
  if (tab === 'inspection') {
    ensureInspectionDefaultInstance()
    await loadInspectionReports()
  }
  if (tab === 'audit') {
    await loadQueryAudits()
  }
})

watch(canUseQueryUnlimitedRows, (canUse) => {
  if (!canUse && queryUnlimitedRows.value) {
    queryUnlimitedRows.value = false
  }
})

watch([isRedisQueryInstance, canWriteCurrentQueryInstance, canDDLCurrentQueryInstance], ([isRedis, canWrite, canDDL]) => {
  if (isRedis && queryConsoleMode.value !== 'read') {
    queryConsoleMode.value = 'read'
    return
  }
  if (queryConsoleMode.value === 'write' && !canWrite) {
    queryConsoleMode.value = 'read'
    return
  }
  if (queryConsoleMode.value === 'ddl' && !canDDL) {
    queryConsoleMode.value = 'read'
  }
})

watch(
  () => [
    diagnosisInstanceId.value,
    capacityRange.value,
    capacityTrend.value?.collectedAt || '',
    capacityTrendPoints.value.length,
    capacityLoading.value,
    activeTab.value
  ],
  () => {
    if (!capacityLoading.value) {
      renderCapacityChart()
    }
  },
  { flush: 'post' }
)

watch(
  () => [
    topologyInstanceId.value,
    topologyResult.value?.collectedAt || '',
    topologyResult.value?.nodes?.length || 0,
    topologyResult.value?.links?.length || 0,
    topologyLoading.value,
    activeTab.value
  ],
  () => {
    if (!topologyLoading.value) {
      renderTopologyChart()
    }
  },
  { flush: 'post' }
)

onMounted(async () => {
  window.addEventListener('resize', resizeCapacityChart)
  document.addEventListener('click', handleDocumentClick)
  loadQueryFavorites()
  await Promise.all([loadSupportedTypes(), loadCredentials(), loadRoles(), loadUIPermissions(), loadDatabaseWriteConfig()])
  await Promise.all([loadInstances(), loadInstanceOptions()])
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCapacityChart)
  document.removeEventListener('click', handleDocumentClick)
  capacityChart?.dispose()
  capacityChart = null
  topologyChart?.dispose()
  topologyChart = null
})
</script>

<style scoped>
.database-page {
  padding: 24px;
  background: #f5f7fa;
  min-height: 100vh;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  background: #ffffff;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
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
  border-radius: 12px;
  background: linear-gradient(135deg, #0f766e, #0ea5e9);
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

.header-actions {
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

.main-tabs,
.search-bar,
.table-wrapper {
  background: #ffffff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}

.main-tabs {
  padding: 0 24px 24px;
}

.search-bar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 18px;
  margin-bottom: 16px;
}

.search-inputs {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  flex: 1;
}

.search-input {
  width: 260px;
}

.search-actions {
  display: flex;
  align-items: flex-start;
}

.reset-btn {
  background: #ffffff;
  border-color: #dcdfe6;
  color: #606266;
}

.table-wrapper {
  padding: 18px;
}

.modern-table {
  width: 100%;
}

.pagination-container {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.instance-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: #1f2937;
}

.capacity-summary-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 0;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  color: #374151;
}

.field-tip {
  margin-top: 6px;
  line-height: 1.5;
  font-size: 12px;
  color: #909399;
}

.type-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.port-input {
  width: 100%;
  min-width: 170px;
}

.metadata-browser {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.metadata-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.metadata-selector {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.toolbar-label {
  font-size: 13px;
  font-weight: 600;
  color: #4b5563;
}

.metadata-instance-select {
  width: 360px;
}

.metadata-empty {
  padding: 52px 0;
  background: #ffffff;
  border: 1px dashed #d1d5db;
  border-radius: 8px;
}

.metadata-content {
  display: grid;
  grid-template-columns: 260px minmax(360px, 1fr) minmax(420px, 1.2fr);
  gap: 16px;
}

.schema-panel,
.table-panel,
.detail-panel {
  min-height: 600px;
  padding: 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 700;
  color: #111827;
}

.detail-panel-heading,
.detail-panel-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.schema-tree {
  min-height: 120px;
}

.schema-tree :deep(.el-tree-node__content) {
  height: 34px;
  border-radius: 6px;
}

.schema-node,
.table-name-cell,
.column-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.schema-node {
  width: 100%;
  justify-content: space-between;
  padding-right: 8px;
}

.schema-node-name,
.table-name-cell span:first-child,
.column-name-cell span:first-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metadata-table :deep(.el-table__row) {
  cursor: pointer;
}

.metadata-context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 150px;
  padding: 6px;
  background: #ffffff;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.16);
}

.metadata-context-menu-item {
  display: block;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #111827;
  font-size: 13px;
  line-height: 1.4;
  text-align: left;
  cursor: pointer;
}

.metadata-context-menu-item:hover:not(:disabled) {
  background: #eff6ff;
  color: #2563eb;
}

.metadata-context-menu-item:disabled {
  color: #9ca3af;
  cursor: not-allowed;
}

.detail-tabs {
  margin-top: -8px;
}

.table-overview-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.table-overview-card {
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.table-overview-card strong {
  color: #111827;
  font-size: 16px;
}

.query-console {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.query-safety-alert :deep(.el-alert__title) {
  line-height: 1.5;
}

.query-context-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
  padding: 14px 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.query-context-selects {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  flex: 1;
  min-width: 0;
}

.query-field {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.query-instance-field {
  flex: 1;
  min-width: 320px;
}

.query-field-label {
  flex: 0 0 auto;
  color: #4b5563;
  font-size: 13px;
  font-weight: 600;
}

.query-select {
  width: 300px;
}

.query-instance-field .query-select {
  width: min(520px, 100%);
}

.query-schema-select {
  width: 240px;
}

.query-context-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.query-endpoint {
  max-width: 360px;
  overflow: hidden;
  color: #374151;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.query-workbench {
  position: relative;
  display: block;
  padding-right: 336px;
}

.query-editor-panel,
.query-side-card,
.query-result-panel {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.query-editor-panel,
.query-result-panel {
  padding: 16px;
}

.query-side-panel {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: 320px;
}

.query-side-card {
  padding: 14px;
}

.query-panel-header,
.query-result-header,
.query-policy-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.query-panel-header,
.query-result-header {
  margin-bottom: 12px;
}

.query-panel-header h3,
.query-result-header h3 {
  margin: 2px 0 0;
  color: #111827;
  font-size: 16px;
  font-weight: 650;
}

.query-section-eyebrow {
  color: #6b7280;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0;
}

.query-mode-control,
.query-result-status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.query-action-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  padding: 10px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.query-prod-alert {
  margin-bottom: 12px;
}

.query-number {
  width: 130px;
}

.query-option-label {
  color: #4b5563;
  font-size: 13px;
  font-weight: 600;
}

.sql-editor :deep(.el-textarea__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  line-height: 1.6;
  color: #111827;
  background: #fbfdff;
  border-color: #d7dde6;
  min-height: 312px;
}

.mono-textarea :deep(.el-textarea__inner) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  line-height: 1.6;
  color: #111827;
}

.query-hints {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
  color: #6b7280;
  font-size: 13px;
}

.query-side-title {
  color: #111827;
  font-size: 14px;
  font-weight: 650;
}

.query-side-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.query-param-list,
.query-policy-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 12px;
}

.query-param-item,
.query-policy-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
}

.query-param-switch {
  justify-content: flex-start;
}

.query-policy-card {
  background: #fffaf3;
  border-color: #fed7aa;
}

.query-policy-row span {
  color: #374151;
  font-size: 13px;
  font-weight: 600;
}

.query-policy-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 2px;
}

.query-side-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}

.query-side-list-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.query-side-item-main {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

.query-side-item-main strong,
.query-side-item-main span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.query-side-item-main strong {
  color: #111827;
  font-size: 12px;
  font-weight: 650;
}

.query-side-item-main span {
  margin-top: 4px;
  color: #6b7280;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.query-result-panel {
  margin-right: 336px;
  min-height: 180px;
}

.query-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.write-meta-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.query-result-tabs {
  margin-top: 2px;
}

.query-result-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
  color: #6b7280;
  font-size: 13px;
}

.query-audit-descriptions {
  width: 100%;
}

.explain-summary {
  margin-bottom: 12px;
}

.result-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.summary-card {
  min-width: 118px;
  padding: 12px 14px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.summary-label {
  display: block;
  margin-bottom: 4px;
  font-size: 12px;
  color: #6b7280;
}

.summary-card strong {
  color: #111827;
  font-size: 16px;
}

.executed-sql-alert {
  word-break: break-all;
}

.query-result-table {
  width: 100%;
}

.query-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.query-cell-copyable {
  cursor: pointer;
}

.query-cell-copyable:hover {
  color: #2563eb;
}

.query-history-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}

.query-history-input {
  width: 320px;
}

.query-history-tip {
  margin-top: 12px;
  color: #6b7280;
  font-size: 13px;
}

.diagnosis-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.diagnosis-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.diagnosis-cards {
  margin-bottom: 0;
}

.diagnosis-metric-cards {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.capacity-summary-cards {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.diagnosis-card-desc {
  margin-top: 6px;
  color: #6b7280;
  font-size: 12px;
  line-height: 1.5;
}

.diagnosis-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.diagnosis-section {
  min-height: 460px;
  padding: 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.capacity-section {
  min-height: 0;
}

.capacity-chart-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 12px;
  margin-bottom: 12px;
}

.capacity-chart-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.capacity-chart-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
}

.capacity-chart-meta {
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
}

.capacity-chart-body {
  min-height: 320px;
  padding: 8px 4px 0;
  background: linear-gradient(180deg, #f8fbff 0%, #ffffff 100%);
  border: 1px solid #e2e8f0;
  border-radius: 10px;
}

.capacity-chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}

.capacity-chart {
  width: 100%;
  height: 320px;
}

.diagnosis-alert {
  margin-bottom: 12px;
}

.inspection-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.inspection-section-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.topology-panel,
.topology-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.topology-toolbar-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.topology-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.8fr);
  gap: 16px;
}

.topology-section {
  padding: 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.topology-graph-section {
  min-height: 390px;
}

.topology-chart {
  width: 100%;
  height: 340px;
  margin-top: 10px;
  background: linear-gradient(180deg, #f8fbff 0%, #ffffff 100%);
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.topology-findings-section {
  border-color: #fed7aa;
  background: #fffaf3;
}

.topology-findings {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 12px;
}

.topology-finding-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  background: #ffffff;
  border: 1px solid #fde7c7;
  border-radius: 8px;
}

.topology-finding-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  color: #4b5563;
  font-size: 13px;
  line-height: 1.5;
}

.topology-finding-body strong {
  color: #111827;
  font-size: 13px;
}

.topology-message {
  margin-bottom: 0;
}

.topology-detail-descriptions {
  margin-bottom: 16px;
}

.topology-detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 18px;
}

.topology-detail-metrics {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.backup-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.backup-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.backup-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.backup-toolbar-group {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.backup-risk-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
  line-height: 1.5;
}

.backup-task-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.backup-inline-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.pitr-tabs {
  width: 100%;
}

.pitr-sub-toolbar {
  margin-bottom: 12px;
  padding: 12px;
}

.permission-tag-list {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.permission-checkbox-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(120px, 1fr));
  gap: 8px 16px;
  width: 100%;
}

.backup-dialog-alert {
  margin-bottom: 16px;
}

.backup-dialog-alert.compact-alert {
  margin-top: 8px;
  margin-bottom: 0;
}

.backup-task-dialog :deep(.el-dialog__body) {
  max-height: calc(100vh - 180px);
  overflow-y: auto;
}

.postgres-barman-wizard-dialog :deep(.el-dialog__body) {
  max-height: calc(100vh - 180px);
  overflow-y: auto;
}

.postgres-barman-wizard-switch-row :deep(.el-form-item__content) {
  min-width: 180px;
}

.backup-summary-descriptions,
.backup-preview-table {
  margin-bottom: 14px;
}

.backup-preview-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 14px 0;
}

.backup-inline-alert {
  margin: 0;
}

.restore-assertions {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.restore-assertions-header,
.restore-assertion-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.restore-empty-tip {
  color: #909399;
  font-size: 12px;
}

.restore-assertion-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.restore-assertion-toolbar {
  color: #4b5563;
  font-size: 13px;
  font-weight: 600;
}

.restore-assertion-fields {
  width: 100%;
}

.assertion-number {
  width: 100%;
}

.form-number-full {
  width: 100%;
}

.permission-mode-alert {
  margin-bottom: 14px;
}

.audit-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.audit-search-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.audit-search-input {
  width: 320px;
}

.audit-select {
  width: 150px;
}

.audit-sql-block {
  margin-top: 16px;
}

.write-confirm-dialog {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.write-confirm-desc {
  margin-top: 4px;
}

.write-confirm-form {
  max-width: none;
}

.audit-sql-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 700;
  color: #111827;
}

.audit-sql-block pre {
  max-height: 320px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  background: #111827;
  border-radius: 8px;
  color: #e5e7eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  line-height: 1.6;
}

.ddl-block pre {
  max-height: 460px;
}

.pitr-state-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.muted-text {
  color: #6b7280;
}

.storage-posture-current {
  margin-top: 8px;
  padding: 10px 12px;
  color: #4b5563;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  line-height: 1.6;
}

.storage-posture-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.storage-posture-checks {
  margin-top: 4px;
}

.runner-config-section {
  margin-top: 16px;
}

.runner-config-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 700;
  color: #1f2937;
}

.runner-tool-section {
  margin: 16px 0;
}

.runner-tool-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.runner-tool-script-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 4px 0 14px;
}

.runner-tool-script-textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.restore-job-detail {
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-height: 70vh;
  overflow-y: auto;
}

.restore-job-detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.restore-job-timeline {
  padding-top: 6px;
}

.restore-step-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.restore-step-name {
  font-weight: 600;
  color: #1f2937;
}

.pitr-catalog-meta,
.pitr-wal-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.45;
  font-size: 12px;
  color: #4b5563;
}

.pitr-wal-stream-card {
  margin-bottom: 16px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #ffffff;
}

.pitr-wal-stream-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.pitr-wal-stream-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: #1f2937;
}

.pitr-wal-stream-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #4b5563;
}

.pitr-wal-stream-empty {
  padding: 24px 0;
}

.inline-control-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.inline-control-row .el-input {
  flex: 1;
  min-width: 0;
}

@media (max-width: 900px) {
  .page-header,
  .search-bar,
  .metadata-toolbar,
  .backup-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .search-input,
  .metadata-instance-select,
  .query-select,
  .query-number,
  .query-history-input,
  .audit-search-input,
  .audit-select,
  .port-input {
    width: 100%;
    min-width: 0;
  }

  .metadata-content {
    grid-template-columns: 1fr;
  }

  .diagnosis-grid,
  .inspection-section-grid,
  .topology-grid,
  .table-overview-cards,
  .diagnosis-metric-cards,
  .capacity-summary-cards {
    grid-template-columns: 1fr;
  }

  .capacity-chart-body,
  .capacity-chart {
    min-height: 260px;
    height: 260px;
  }

  .query-workbench {
    padding-right: 0;
  }

  .query-side-panel {
    position: static;
    width: 100%;
    margin-top: 16px;
  }

  .query-result-panel {
    margin-right: 0;
  }

  .query-context-panel,
  .query-panel-header,
  .query-result-header,
  .query-policy-header {
    align-items: stretch;
    flex-direction: column;
  }

  .query-context-selects,
  .query-field,
  .query-instance-field,
  .query-mode-control,
  .query-result-status {
    width: 100%;
  }

  .query-field {
    align-items: stretch;
    flex-direction: column;
  }

  .query-context-tags,
  .query-mode-control,
  .query-result-status {
    justify-content: flex-start;
  }

  .query-param-item,
  .query-policy-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
