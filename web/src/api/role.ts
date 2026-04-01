import request from '@/utils/request'

// 角色列表
export const getRoleList = (params: any) => {
  return request.get('/api/v1/roles', { params })
}

// 获取所有角色（不分页）
export const getAllRoles = () => {
  return request.get('/api/v1/roles/all')
}

// 获取角色详情
export const getRole = (id: number) => {
  return request.get(`/api/v1/roles/${id}`)
}

// 创建角色
export const createRole = (data: any) => {
  return request.post('/api/v1/roles', data)
}

// 更新角色
export const updateRole = (id: number, data: any) => {
  return request.put(`/api/v1/roles/${id}`, data)
}

// 删除角色
export const deleteRole = (id: number) => {
  return request.delete(`/api/v1/roles/${id}`)
}

// 获取角色菜单
export const getRoleMenus = (id: number) => {
  return request.get(`/api/v1/roles/${id}`)
}

// 分配角色菜单
export const assignRoleMenus = (id: number, menuIds: number[]) => {
  return request.post(`/api/v1/roles/${id}/menus`, { menuIds })
}
