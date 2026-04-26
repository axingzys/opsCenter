<template>
  <div>
    <div class="search-bar">
      <div class="search-inputs">
        <el-input
          v-model="query.keyword"
          placeholder="搜索实例名称、地址、默认库、业务系统..."
          clearable
          class="search-input"
          @keyup.enter="emit('load')"
          @clear="emit('load')"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-select v-model="query.dbType" placeholder="数据库类型" clearable class="search-input" @change="emit('load')">
          <el-option v-for="item in supportedTypes" :key="item.type" :label="item.name" :value="item.type" />
        </el-select>

        <el-select v-model="query.status" placeholder="状态" clearable class="search-input" @change="emit('load')">
          <el-option label="启用" value="enabled" />
          <el-option label="禁用" value="disabled" />
        </el-select>

        <el-select v-model="query.environment" placeholder="环境" clearable class="search-input" @change="emit('load')">
          <el-option label="生产" value="prod" />
          <el-option label="预发" value="staging" />
          <el-option label="测试" value="test" />
          <el-option label="开发" value="dev" />
        </el-select>
      </div>
      <div class="search-actions">
        <el-button class="reset-btn" @click="emit('reset')">
          <el-icon style="margin-right: 4px;"><RefreshLeft /></el-icon>
          重置
        </el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        :data="rows"
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
            <el-tooltip :content="canTest(row) ? '连接测试' : '无连接测试权限或该类型暂未接入'" placement="top">
              <el-button link type="primary" :loading="testingId === row.id" :disabled="!canTest(row)" @click="emit('test', row)">
                <el-icon><Connection /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="canSync(row) ? '同步结构' : '无同步权限或该类型暂未接入'" placement="top">
              <el-button link type="success" :loading="syncingId === row.id" :disabled="!canSync(row)" @click="emit('sync', row)">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="row.status === 'enabled' ? '禁用' : '启用'" placement="top">
              <el-button link :type="row.status === 'enabled' ? 'warning' : 'success'" :disabled="!canManage(row)" @click="emit('toggle', row)">
                <el-icon><Switch /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="编辑" placement="top">
              <el-button link type="primary" :disabled="!canManage(row)" @click="emit('edit', row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="删除" placement="top">
              <el-button link type="danger" :disabled="!canManage(row)" @click="emit('remove', row)">
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
          @size-change="emit('load')"
          @current-change="emit('load')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Connection, Delete, Edit, Refresh, RefreshLeft, Search, Switch } from '@element-plus/icons-vue'

type InstanceQuery = {
  page: number
  pageSize: number
  keyword?: string
  dbType?: string
  status?: string
  environment?: string
}

type SupportedType = {
  type: string
  name: string
}

defineProps<{
  query: InstanceQuery
  supportedTypes: SupportedType[]
  rows: any[]
  loading: boolean
  total: number
  testingId: number
  syncingId: number
  canTest: (row: any) => boolean
  canSync: (row: any) => boolean
  canManage: (row: any) => boolean
}>()

const emit = defineEmits<{
  load: []
  reset: []
  test: [row: any]
  sync: [row: any]
  toggle: [row: any]
  edit: [row: any]
  remove: [row: any]
}>()

const environmentText = (value: string) => {
  const map: Record<string, string> = {
    prod: '生产',
    staging: '预发',
    test: '测试',
    dev: '开发'
  }
  return map[value] || value
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
</script>
