<template>
  <div class="backup-card">
    <div class="backup-toolbar">
      <div class="backup-toolbar-group">
        <el-input
          v-model="query.keyword"
          placeholder="搜索角色或实例"
          clearable
          class="audit-search-input"
          @keyup.enter="emit('load')"
          @clear="emit('load')"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="query.roleId" placeholder="角色" clearable filterable class="audit-select" @change="emit('load')">
          <el-option v-for="role in roleOptions" :key="role.id" :label="role.name" :value="role.id" />
        </el-select>
        <el-select v-model="query.instanceId" placeholder="实例" clearable filterable class="audit-select" @change="emit('load')">
          <el-option v-for="item in instances" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </div>
      <el-button v-if="canManage" type="primary" @click="emit('add')">
        <el-icon style="margin-right: 6px;"><Plus /></el-icon>
        添加权限
      </el-button>
    </div>

    <el-table
      :data="rows"
      v-loading="loading"
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
              v-for="item in permissionOptions.filter(option => hasPermissionMask(row.permissions, option.value))"
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
      <el-table-column v-if="canManage" label="操作" width="120" align="center" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="emit('edit', row)">编辑</el-button>
          <el-button link type="danger" @click="emit('delete', row)">删除</el-button>
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
</template>

<script setup lang="ts">
import { Plus, Search } from '@element-plus/icons-vue'

type PermissionOption = {
  label: string
  value: number
}

type PermissionQuery = {
  page: number
  pageSize: number
  keyword?: string
  roleId?: number
  instanceId?: number
}

defineProps<{
  query: PermissionQuery
  roleOptions: any[]
  instances: any[]
  rows: any[]
  loading: boolean
  total: number
  canManage: boolean
  permissionOptions: PermissionOption[]
}>()

const emit = defineEmits<{
  load: []
  add: []
  edit: [row: any]
  delete: [row: any]
}>()

const hasPermissionMask = (mask: number | undefined, permission: number) =>
  (Number(mask || 0) & permission) > 0
</script>
