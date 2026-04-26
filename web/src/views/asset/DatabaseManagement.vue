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
        <DatabaseInstancePermissionsPanel
          :query="permissionQuery"
          :role-options="roleOptions"
          :instances="instances"
          :rows="permissionRows"
          :loading="permissionLoading"
          :total="permissionTotal"
          :can-manage="canManageInstancePermissions"
          :permission-options="databasePermissionOptions"
          @load="loadInstancePermissions"
          @add="openPermissionDialog()"
          @edit="openPermissionDialog"
          @delete="handleDeletePermission"
        />
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
          />

          <div class="query-toolbar">
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
            <el-select v-model="querySchemaName" :placeholder="querySchemaPlaceholder" clearable filterable class="query-select">
              <el-option v-for="item in querySchemas" :key="item.schemaName" :label="item.schemaName" :value="item.schemaName" />
            </el-select>
            <span class="query-option-label">最大行数</span>
            <el-input-number v-model="queryLimit" :min="1" :max="500" :step="50" class="query-number" />
            <span class="query-option-label">超时秒数</span>
            <el-input-number v-model="queryTimeoutSeconds" :min="1" :max="30" class="query-number" />
            <span class="query-option-label">导出上限</span>
            <el-input-number v-model="queryExportLimit" :min="1" :max="5000" :step="100" class="query-number" />
            <el-button type="primary" :loading="queryRunning" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="executeQuery">
              {{ isRedisQueryInstance ? '执行命令' : '执行查询' }}
            </el-button>
            <el-button :loading="queryFormatting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleFormatQuery">
              {{ isRedisQueryInstance ? '格式化命令' : '格式化 SQL' }}
            </el-button>
            <el-button v-if="!isRedisQueryInstance" :loading="queryExplaining" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.QUERY, 'queryEnabled')" @click="handleExplainQuery">
              执行计划
            </el-button>
            <el-button v-if="!isRedisQueryInstance" type="warning" plain :loading="queryWriteChecking" :disabled="!queryInstanceId || !canWriteCurrentQueryInstance" @click="handleValidateWriteQuery">
              写前检查
            </el-button>
            <el-button v-if="!isRedisQueryInstance" type="danger" plain :loading="queryWritePreparing" :disabled="!queryInstanceId || !canWriteCurrentQueryInstance" @click="handlePrepareWriteExecute">
              受控写入
            </el-button>
            <el-button type="success" plain :loading="queryExporting" :disabled="!queryInstanceId || !canUseDatabaseFeature(currentQueryInstance, DATABASE_PERMISSION.EXPORT, 'queryEnabled')" @click="handleExportQuery">
              <el-icon style="margin-right: 4px;"><Download /></el-icon>
              导出结果
            </el-button>
            <el-button type="primary" plain :loading="queryHistoryLoading" @click="openQueryHistory">
              最近历史
            </el-button>
            <el-button @click="resetQueryConsole">清空</el-button>
          </div>

          <el-input
            v-model="querySQL"
            type="textarea"
            :rows="10"
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
              <el-tag size="small" type="primary">SQL 格式化</el-tag>
              <el-tag size="small" type="warning">执行计划 / CSV 导出</el-tag>
              <el-tag size="small" type="warning">禁止多语句</el-tag>
              <el-tag size="small" type="warning">原因 / 高风险确认</el-tag>
            </template>
            <span>当前实例：{{ currentQueryInstance?.name || '-' }}</span>
          </div>

          <div v-if="writeResult" class="query-result">
            <div class="result-summary">
              <div class="summary-card">
                <span class="summary-label">影响行数</span>
                <strong>{{ writeResult.rowsAffected }}</strong>
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
                <span class="summary-label">影响阈值</span>
                <strong>{{ writeResult.rowsAffectedLimit }}</strong>
              </div>
            </div>
            <el-alert
              :title="writeResult.message || '写操作执行完成'"
              type="success"
              show-icon
              :closable="false"
            />
            <el-alert
              v-if="writeResult.executedSql"
              :title="`实际执行 SQL：${writeResult.executedSql}`"
              type="warning"
              show-icon
              :closable="false"
              class="executed-sql-alert"
            />
            <div class="write-meta-tags">
              <el-tag :type="riskLevelTag(writeResult.riskLevel)">{{ writeResult.riskLevelText }}</el-tag>
              <el-tag v-if="writeResult.reason" type="info">原因：{{ writeResult.reason }}</el-tag>
              <el-tag v-if="writeResult.confirmRequired" :type="writeResult.confirmed ? 'success' : 'warning'">
                {{ writeResult.confirmed ? '已确认执行' : '未确认' }}
              </el-tag>
            </div>
            <div v-if="writeResult.rollbackSql" class="audit-sql-block">
              <div class="audit-sql-title">回滚 SQL / 恢复提示</div>
              <pre>{{ writeResult.rollbackSql }}</pre>
            </div>
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
            <div class="write-meta-tags">
              <el-tag :type="riskLevelTag(writeCheckResult.riskLevel)">{{ writeCheckResult.riskLevelText }}</el-tag>
              <el-tag :type="writeCheckResult.reasonRequired ? 'warning' : 'info'">
                {{ writeCheckResult.reasonRequired ? '执行时必须填写原因' : '原因非必填' }}
              </el-tag>
              <el-tag :type="writeCheckResult.confirmRequired ? 'danger' : 'success'">
                {{ writeCheckResult.confirmRequired ? '执行前需二次确认' : '无需二次确认' }}
              </el-tag>
            </div>
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
              <el-tag v-if="queryResult.cellTruncated" type="warning">字段已截断</el-tag>
              <el-tag v-if="queryResult.cellsMasked" type="info">敏感字段已脱敏</el-tag>
              <el-tag v-if="queryResult.binaryPreviewed" type="info">二进制已预览</el-tag>
            </div>
            <el-alert
              v-if="queryResult.executedSql"
              :title="`${isRedisQueryInstance ? '实际执行命令' : '实际执行 SQL'}：${queryResult.executedSql}`"
              type="success"
              show-icon
              :closable="false"
              class="executed-sql-alert"
            />
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
                  <span class="query-cell">{{ formatQueryCell(row[column]) }}</span>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <el-empty v-else :description="isRedisQueryInstance ? '执行 Redis 只读命令后在这里查看结果' : '执行只读 SQL、写前检查或受控写入后在这里查看结果'" :image-size="80" />
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
            title="四期第 1 批支持 Redis Cluster / MongoDB ReplicaSet / Elasticsearch / OpenSearch 的只读拓扑查询，并写入统一审计。"
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
            <el-button type="primary" :disabled="!topologyInstanceId" :loading="topologyLoading" @click="loadTopology">
              刷新拓扑
            </el-button>
          </div>

          <div v-if="!topologyInstanceId" class="metadata-empty">
            <el-empty description="请先选择 Redis、MongoDB、Elasticsearch 或 OpenSearch 实例" :image-size="82" />
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
                  </el-table>
                </div>

                <div class="topology-section">
                  <div class="panel-title">
                    <span>复制关系</span>
                    <el-tag size="small" type="info">{{ topologyResult.links?.length || 0 }}</el-tag>
                  </div>
                  <el-table :data="topologyResult.links || []" stripe height="360" class="modern-table">
                    <el-table-column label="源节点" prop="source" min-width="160" show-overflow-tooltip />
                    <el-table-column label="目标节点" prop="target" min-width="160" show-overflow-tooltip />
                    <el-table-column label="关系" width="120">
                      <template #default="{ row }">{{ row.label || '-' }}</template>
                    </el-table-column>
                    <el-table-column label="状态" width="120">
                      <template #default="{ row }">{{ row.state || '-' }}</template>
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

      <el-tab-pane label="备份任务" name="backup">
        <div class="backup-panel">
          <el-alert
            title="当前已支持 MySQL / MariaDB / PostgreSQL / Redis 逻辑备份执行、定时调度、保留策略清理、文件下载和统一审计。PostgreSQL 支持 Plain SQL 与 Custom 两种格式，备份文件默认写入系统配置中的数据库备份目录。"
            type="info"
            show-icon
            :closable="false"
          />

          <div class="backup-card">
            <div class="panel-title">
              <span>备份任务</span>
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
              <el-table-column label="类型" width="120" align="center">
                <template #default="{ row }">{{ row.backupTypeText || row.backupType || '-' }}</template>
              </el-table-column>
              <el-table-column label="计划" min-width="140">
                <template #default="{ row }">{{ row.schedule || '仅手动' }}</template>
              </el-table-column>
              <el-table-column label="保留天数" width="100" align="right">
                <template #default="{ row }">{{ row.retentionDays }}</template>
              </el-table-column>
              <el-table-column label="最近执行" width="170">
                <template #default="{ row }">{{ row.lastRunAt || '-' }}</template>
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
                  <el-option label="执行中" value="running" />
                  <el-option label="成功" value="success" />
                  <el-option label="失败" value="failed" />
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
                </el-select>
              </div>
              <div class="backup-toolbar-group">
                <el-button @click="resetBackupRecordQuery">
                  <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
                  重置
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
              <el-table-column label="类型" width="140" align="center">
                <template #default="{ row }">{{ row.backupTypeText || row.backupType || '-' }}</template>
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
              <el-table-column label="校验" width="120" align="center">
                <template #default="{ row }">
                  <el-tooltip v-if="row.checksumSha256" :content="row.checksumSha256" placement="top">
                    <el-tag size="small" type="success">{{ row.checksumSha256.slice(0, 8) }}</el-tag>
                  </el-tooltip>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="耗时" width="100" align="right">
                <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
              </el-table-column>
              <el-table-column label="结果" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">{{ row.message || '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="150" align="center" fixed="right">
                <template #default="{ row }">
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
                    @click="openRestoreDialog(row)"
                  >
                    演练
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
              <el-table-column label="操作者" width="120">
                <template #default="{ row }">{{ row.operatorName || '-' }}</template>
              </el-table-column>
              <el-table-column label="耗时" width="100" align="right">
                <template #default="{ row }">{{ row.durationMs ? `${row.durationMs} ms` : '-' }}</template>
              </el-table-column>
              <el-table-column label="结果" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">{{ row.message || '-' }}</template>
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
        <DatabaseInspectionReportsPanel
          :form="inspectionForm"
          :query="inspectionQuery"
          :instances="diagnosisInstances"
          :reports="inspectionReports"
          :generating="inspectionGenerating"
          :loading="inspectionLoading"
          :total="inspectionTotal"
          @generate="handleGenerateInspectionReport"
          @load="loadInspectionReports"
          @reset="resetInspectionQuery"
          @detail="openInspectionDetail"
        />
      </el-tab-pane>

      <el-tab-pane label="查询审计" name="audit">
        <DatabaseQueryAuditPanel
          :query="auditQuery"
          :instances="instances"
          :audit-actions="auditActions"
          :sql-types="sqlTypes"
          :rows="queryAudits"
          :loading="auditLoading"
          :exporting="auditExporting"
          :total="auditTotal"
          @load="loadQueryAudits"
          @reset="resetAuditQuery"
          @export="handleExportAudits"
          @detail="openAuditDetail"
        />
      </el-tab-pane>
    </el-tabs>

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
        <el-descriptions-item label="影响阈值">{{ currentAudit.rowsAffectedLimit || 0 }}</el-descriptions-item>
        <el-descriptions-item label="影响行">{{ currentAudit.rowsAffected }}</el-descriptions-item>
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
      width="720px"
      @close="resetBackupTaskForm"
    >
      <el-alert
        title="当前支持 MySQL / MariaDB / PostgreSQL / Redis 的逻辑备份任务配置。PostgreSQL 可选择 Plain SQL 或 Custom 格式；Custom 会生成 .dump 文件，恢复演练使用 pg_restore。"
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
          <el-col :span="12">
            <el-form-item label="备份类型">
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
          <el-col :span="12">
            <el-form-item label="存储类型">
              <el-input v-model="backupTaskForm.storageType" disabled />
            </el-form-item>
          </el-col>
        </el-row>
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
        <el-form-item label="存储配置">
          <el-input
            v-model="backupTaskForm.storageConfig"
            type="textarea"
            :rows="3"
            placeholder="可选，预留给后续本地目录或对象存储扩展"
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
              v-for="item in instances"
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
import DatabaseInspectionReportsPanel from './components/DatabaseInspectionReportsPanel.vue'
import DatabaseInstancePermissionsPanel from './components/DatabaseInstancePermissionsPanel.vue'
import DatabaseQueryAuditPanel from './components/DatabaseQueryAuditPanel.vue'
import {
  DATABASE_PERMISSION,
  createDatabaseBackupTask,
  createDatabaseInstance,
  collectDatabaseCapacitySnapshot,
  deleteDatabaseBackupTask,
  deleteDatabaseInstance,
  deleteDatabaseInstancePermission,
  disableDatabaseInstance,
  downloadDatabaseBackupRecord,
  explainDatabaseQuery,
  enableDatabaseInstance,
  executeDatabaseQuery,
  executeDatabaseWriteQuery,
  generateDatabaseInspectionReport,
  getDatabaseCapacityTrend,
  getDatabaseDiagnosisMetrics,
  getDatabaseInspectionReport,
  getDatabaseTopology,
  getDatabaseUIPermissions,
  exportDatabaseQueryResult,
  exportDatabaseQueryAudits,
  exportDatabaseTableDictionary,
  formatDatabaseQuery,
  listDatabaseBackupRecords,
  listDatabaseBackupTasks,
  listDatabaseDiagnosisSessions,
  listDatabaseInspectionReports,
  listDatabaseRestoreJobs,
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
  runDatabaseBackupTask,
  runDatabaseRestoreDryRun,
  syncDatabaseMetadata,
  testDatabaseInstance,
  updateDatabaseBackupTask,
  updateDatabaseInstance,
  upsertDatabaseInstancePermission,
  type DatabaseBackupRecordResult,
  type DatabaseBackupRunResult,
  type DatabaseBackupTaskPayload,
  type DatabaseBackupTaskResult,
  type DatabaseCapacityCollectResult,
  type DatabaseCapacityTrendResult,
  type DatabaseInspectionSection,
  type DatabaseInspectionReportResult,
  type DatabaseInstancePayload,
  type DatabaseQueryPayload,
  type DatabaseRestoreDryRunPayload,
  type DatabaseRestoreJobResult,
  type DatabaseSupportedType,
  type DatabaseTopologyResult,
  type DatabaseWriteExecuteResult,
  type DatabaseWriteValidateResult,
  validateDatabaseWriteQuery
} from '@/api/database'
import { getAllRoles } from '@/api/role'

const activeTab = ref('instances')
const loading = ref(false)
const submitting = ref(false)
const testingId = ref(0)
const syncingId = ref(0)
const dialogVisible = ref(false)
const instances = ref<any[]>([])
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
  { label: '管理', value: DATABASE_PERMISSION.MANAGE }
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
const queryInstanceId = ref<number>()
const querySchemaName = ref('')
const querySchemas = ref<any[]>([])
const querySQL = ref('SELECT 1')
const queryLimit = ref(500)
const queryTimeoutSeconds = ref(30)
const queryExportLimit = ref(1000)
const queryRunning = ref(false)
const queryFormatting = ref(false)
const queryExplaining = ref(false)
const queryExporting = ref(false)
const queryWriteChecking = ref(false)
const queryWritePreparing = ref(false)
const queryWriteSubmitting = ref(false)
const queryResult = ref<any>()
const writeCheckResult = ref<DatabaseWriteValidateResult>()
const writeResult = ref<DatabaseWriteExecuteResult>()
const writeConfirmVisible = ref(false)
const writeConfirmForm = reactive({
  reason: '',
  confirmed: false
})
const explainDialogVisible = ref(false)
const explainResult = ref<any>()
const queryHistoryDialogVisible = ref(false)
const queryHistoryLoading = ref(false)
const queryHistoryItems = ref<any[]>([])
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
const topologyInstanceId = ref<number>()
const topologyLoading = ref(false)
const topologyResult = ref<DatabaseTopologyResult>()
const backupTaskLoading = ref(false)
const backupTaskSubmitting = ref(false)
const backupTaskDialogVisible = ref(false)
const runningBackupTaskId = ref(0)
const backupTasks = ref<DatabaseBackupTaskResult[]>([])
const backupTaskTotal = ref(0)
const backupTaskFormRef = ref<FormInstance>()
const permissionFormRef = ref<FormInstance>()
const permissionLoading = ref(false)
const permissionSubmitting = ref(false)
const permissionDialogVisible = ref(false)
const permissionRows = ref<any[]>([])
const permissionTotal = ref(0)
const backupRecordLoading = ref(false)
const backupRecords = ref<DatabaseBackupRecordResult[]>([])
const backupRecordTotal = ref(0)
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
  { label: '查询结果导出', value: 'query_export' },
  { label: '写操作执行', value: 'change_execute' },
  { label: '逻辑备份执行', value: 'backup_run' },
  { label: '备份文件下载', value: 'backup_download' },
  { label: '拓扑查看', value: 'topology_view' },
  { label: '恢复演练', value: 'restore_dry_run' },
  { label: '容量趋势查看', value: 'capacity_view' },
  { label: '巡检报告生成', value: 'inspection_generate' },
  { label: '实例权限保存', value: 'instance_permission_upsert' },
  { label: '实例权限删除', value: 'instance_permission_delete' },
  { label: '数据字典导出', value: 'metadata_export' },
  { label: '诊断指标查看', value: 'diagnosis_metrics' },
  { label: '活跃会话查看', value: 'diagnosis_sessions' },
  { label: '慢 SQL 查看', value: 'diagnosis_slow_queries' }
]

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
  schedule: '',
  storageType: 'local',
  storageConfig: '',
  retentionDays: 7,
  enabled: true
})
const backupTaskFormInstanceDbType = ref('')

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
  instances.value.find(item => item.id === metadataInstanceId.value)
)

const metadataInstances = computed(() =>
  instances.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.VIEW, 'metadataEnabled'))
)

const currentQueryInstance = computed(() =>
  instances.value.find(item => item.id === queryInstanceId.value)
)

const queryInstances = computed(() =>
  instances.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.QUERY, 'queryEnabled'))
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

const queryConsoleAlert = computed(() =>
  isRedisQueryInstance.value
    ? '命令控制台支持 Redis 白名单只读命令。当前支持 GET / MGET / HGET / HGETALL / LRANGE / SMEMBERS / ZRANGE / XRANGE / SCAN / INFO / DBSIZE / MEMORY USAGE / CLUSTER INFO|NODES|SLOTS，所有动作都会写入统一审计。'
    : 'SQL 控制台支持只读查询和受控写操作。只读链路仅允许 SELECT / SHOW / DESC / DESCRIBE / EXPLAIN / WITH；写操作请先做预检查，再按风险确认后执行，所有动作都会写入统一审计。'
)

const querySchemaPlaceholder = computed(() =>
  isRedisQueryInstance.value ? '逻辑 DB' : '默认库 / Schema'
)

const queryEditorPlaceholder = computed(() =>
  isRedisQueryInstance.value
    ? '请输入 Redis 只读命令，例如：SCAN 0 MATCH user:* COUNT 50 或 GET app:config'
    : '请输入 SQL。只读查询可直接执行；写操作请先做预检查，再走受控确认执行。'
)

const currentDiagnosisInstance = computed(() =>
  instances.value.find(item => item.id === diagnosisInstanceId.value)
)

const diagnosisInstances = computed(() =>
  instances.value.filter(item => hasDatabasePermission(item, DATABASE_PERMISSION.DIAGNOSIS))
)

const isRedisDiagnosisInstance = computed(() =>
  currentDiagnosisInstance.value?.dbType === 'redis'
)

const currentTopologyInstance = computed(() =>
  instances.value.find(item => item.id === topologyInstanceId.value)
)

const capacityTrendPoints = computed(() =>
  [...(capacityTrend.value?.points || [])].sort((left, right) => left.collectedAt.localeCompare(right.collectedAt))
)

let capacityChart: echarts.ECharts | null = null
let diagnosisRequestSeq = 0
let capacityTrendRequestSeq = 0

const topologyInstances = computed(() =>
  instances.value.filter(item => canUseDatabaseFeature(item, DATABASE_PERMISSION.TOPOLOGY, 'topologyEnabled'))
)

const supportedBackupInstances = computed(() =>
  instances.value.filter(item => ['mysql', 'mariadb', 'postgresql', 'redis'].includes(item.dbType) && hasDatabasePermission(item, DATABASE_PERMISSION.BACKUP))
)

const selectedBackupTaskInstance = computed(() =>
  instances.value.find(item => item.id === backupTaskForm.instanceId)
)

const selectedBackupTaskDbType = computed(() =>
  selectedBackupTaskInstance.value?.dbType || backupTaskFormInstanceDbType.value
)

const availableBackupTypeOptions = computed(() => {
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

const normalizeBackupTaskFormBackupType = () => {
  if (!availableBackupTypeOptions.value.some(item => item.value === backupTaskForm.backupType)) {
    backupTaskForm.backupType = 'logical'
  }
}

const restoreTargetInstances = computed(() =>
  instances.value.filter(item =>
    ['mysql', 'mariadb', 'postgresql', 'redis'].includes(item.dbType) &&
    hasDatabasePermission(item, DATABASE_PERMISSION.RESTORE) &&
    item.status === 'enabled' &&
    !isProductionEnvironment(item.environment)
  )
)

const restoreTargetOptions = computed(() => {
  const sourceType = restoreSourceRecord.value
    ? instances.value.find(item => item.id === restoreSourceRecord.value?.instanceId)?.dbType
    : ''
  return restoreTargetInstances.value.filter(item =>
    item.id !== restoreSourceRecord.value?.instanceId && isRestoreCompatibleType(sourceType, item.dbType)
  )
})

const restoreSourceDbType = computed(() =>
  restoreSourceRecord.value
    ? instances.value.find(item => item.id === restoreSourceRecord.value?.instanceId)?.dbType || ''
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
    if (metadataInstanceId.value && !metadataInstances.value.some(item => item.id === metadataInstanceId.value)) {
      metadataInstanceId.value = undefined
      resetMetadataSelection()
    }
    if (queryInstanceId.value && !queryInstances.value.some(item => item.id === queryInstanceId.value)) {
      queryInstanceId.value = undefined
      querySchemaName.value = ''
      querySchemas.value = []
      clearQueryConsoleState()
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
    if (backupTaskQuery.instanceId && !supportedBackupInstances.value.some(item => item.id === backupTaskQuery.instanceId)) {
      backupTaskQuery.instanceId = undefined
    }
    if (backupRecordQuery.instanceId && !supportedBackupInstances.value.some(item => item.id === backupRecordQuery.instanceId)) {
      backupRecordQuery.instanceId = undefined
    }
    if (restoreJobQuery.sourceInstanceId && !instances.value.some(item => item.id === restoreJobQuery.sourceInstanceId)) {
      restoreJobQuery.sourceInstanceId = undefined
    }
    if (restoreJobQuery.targetInstanceId && !instances.value.some(item => item.id === restoreJobQuery.targetInstanceId)) {
      restoreJobQuery.targetInstanceId = undefined
    }
    if (inspectionQuery.instanceId && !diagnosisInstances.value.some(item => item.id === inspectionQuery.instanceId)) {
      inspectionQuery.instanceId = undefined
    }
    if (backupTaskForm.instanceId && !supportedBackupInstances.value.some(item => item.id === backupTaskForm.instanceId)) {
      backupTaskForm.instanceId = 0
    }
    if (inspectionForm.instanceId && !diagnosisInstances.value.some(item => item.id === inspectionForm.instanceId)) {
      inspectionForm.instanceId = undefined
    }
  } finally {
    loading.value = false
  }
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
  if (queryInstanceId.value) {
    await loadQuerySchemas()
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

const clearReadOnlyConsoleState = () => {
  queryResult.value = undefined
  explainResult.value = undefined
}

const clearWriteConsoleState = () => {
  writeCheckResult.value = undefined
  writeResult.value = undefined
  writeConfirmVisible.value = false
  resetWriteConfirmForm()
}

const clearQueryConsoleState = () => {
  clearReadOnlyConsoleState()
  clearWriteConsoleState()
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
  backupTaskForm.schedule = ''
  backupTaskForm.storageType = 'local'
  backupTaskForm.storageConfig = ''
  backupTaskForm.retentionDays = 7
  backupTaskForm.enabled = true
  backupTaskFormInstanceDbType.value = ''
  backupTaskFormRef.value?.clearValidate()
}

const resetRestoreForm = () => {
  restoreSourceRecord.value = undefined
  restoreForm.targetInstanceId = 0
  restoreForm.restoreMode = 'dry_run'
  restoreForm.restoreStrategy = 'object_replace'
  restoreFormRef.value?.clearValidate()
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
    backupTaskFormInstanceDbType.value = row.instanceDbType || ''
    backupTaskForm.schedule = row.schedule || ''
    backupTaskForm.storageType = row.storageType || 'local'
    backupTaskForm.storageConfig = row.storageConfig || ''
    backupTaskForm.retentionDays = row.retentionDays || 7
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
      schedule: (backupTaskForm.schedule || '').trim(),
      storageType: backupTaskForm.storageType || 'local',
      storageConfig: backupTaskForm.storageConfig?.trim(),
      retentionDays: backupTaskForm.retentionDays,
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

const loadInstancePermissions = async () => {
  permissionLoading.value = true
  try {
    const res: any = await listDatabaseInstancePermissions(permissionQuery)
    permissionRows.value = res.list || []
    permissionTotal.value = res.total || 0
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
  permissionSubmitting.value = true
  try {
    await upsertDatabaseInstancePermission({
      roleId: permissionForm.roleId!,
      instanceId: permissionForm.instanceId!,
      permissions: permissionBitsToMask(permissionForm.permissions)
    })
    ElMessage.success('实例权限已保存')
    permissionDialogVisible.value = false
    await Promise.all([loadInstancePermissions(), loadInstances()])
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
  await Promise.all([loadInstancePermissions(), loadInstances()])
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
    await loadInstances()
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
  await loadInstances()
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
  await loadInstances()
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
    await loadInstances()
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
  const instance = row || instances.value.find(item => item.id === id)
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
    await loadInstances()
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
  clearQueryConsoleState()
  await loadQuerySchemas()
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

const buildQueryPayload = (limit: number): DatabaseQueryPayload => ({
  schemaName: querySchemaName.value,
  sqlText: querySQL.value,
  limit,
  timeoutSeconds: queryTimeoutSeconds.value
})

const buildWriteValidatePayload = () => ({
  schemaName: querySchemaName.value,
  sqlText: querySQL.value
})

const executeQuery = async () => {
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
  queryRunning.value = true
  try {
    clearWriteConsoleState()
    queryResult.value = await executeDatabaseQuery(queryInstanceId.value, buildQueryPayload(queryLimit.value))
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
  queryExplaining.value = true
  try {
    clearWriteConsoleState()
    explainResult.value = await explainDatabaseQuery(queryInstanceId.value, buildQueryPayload(queryLimit.value))
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
    writeCheckResult.value = await validateDatabaseWriteQuery(queryInstanceId.value, buildWriteValidatePayload())
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
    const result = await validateDatabaseWriteQuery(queryInstanceId.value, buildWriteValidatePayload())
    writeCheckResult.value = result
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
    writeConfirmVisible.value = false
    await loadQueryAudits()
    ElMessage.success(writeResult.value?.message || '写操作执行成功')
  } finally {
    queryWriteSubmitting.value = false
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

const resetBackupRecordQuery = () => {
  backupRecordQuery.page = 1
  backupRecordQuery.pageSize = 10
  backupRecordQuery.taskId = undefined
  backupRecordQuery.instanceId = undefined
  backupRecordQuery.status = ''
  backupRecordQuery.triggerType = ''
  loadBackupRecords()
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

const handleRunBackupTask = async (row: DatabaseBackupTaskResult) => {
  await ElMessageBox.confirm(
    `确定手动触发备份任务「${row.name}」吗？系统会立即执行逻辑备份并写入统一审计。`,
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
    ElMessage.success(res.message || '备份任务已触发')
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
  const target = instances.value.find(item => item.id === restoreForm.targetInstanceId)
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
  if (['replica', 'slave', 'secondary'].includes(normalized)) return 'primary'
  if (normalized === 'arbiter') return 'warning'
  return 'info'
}

const topologyStateTag = (state: string) => {
  const normalized = String(state || '').toLowerCase()
  if (['ok', 'online', 'connected', 'healthy', 'started', 'green'].includes(normalized)) return 'success'
  if (['yellow', 'recovering', 'initializing', 'relocating'].includes(normalized)) return 'warning'
  if (['unhealthy', 'fail', 'failed', 'red', 'disconnected', 'down'].includes(normalized)) return 'danger'
  return 'info'
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
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    case 'queued':
      return 'info'
    case 'cleaning':
      return 'warning'
    case 'pending':
      return 'info'
    default:
      return 'info'
  }
}

const riskLevelTag = (riskLevel: string) => {
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
  if (tab === 'backup') {
    await Promise.all([loadBackupTasks(), loadBackupRecords(), loadRestoreJobs()])
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

onMounted(async () => {
  window.addEventListener('resize', resizeCapacityChart)
  await Promise.all([loadSupportedTypes(), loadCredentials(), loadRoles(), loadUIPermissions()])
  await loadInstances()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCapacityChart)
  capacityChart?.dispose()
  capacityChart = null
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
  gap: 16px;
}

.query-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.query-select {
  width: 300px;
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
}

.query-hints {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  color: #6b7280;
  font-size: 13px;
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

.explain-summary {
  margin-bottom: 12px;
}

.result-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.summary-card {
  min-width: 120px;
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

.topology-message {
  margin-bottom: 0;
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
}
</style>
