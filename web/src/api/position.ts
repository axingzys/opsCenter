import request from '@/utils/request'

// 获取岗位列表
export const getPositionList = (params: { page?: number; pageSize?: number; postCode?: string; postName?: string }) => {
  return request.get('/api/v1/positions', { params })
}

// 获取岗位详情
export const getPosition = (id: number) => {
  return request.get(`/api/v1/positions/${id}`)
}

// 创建岗位
export const createPosition = (data: any) => {
  return request.post('/api/v1/positions', data)
}

// 更新岗位
export const updatePosition = (id: number, data: any) => {
  return request.put(`/api/v1/positions/${id}`, data)
}

// 删除岗位
export const deletePosition = (id: number) => {
  return request.delete(`/api/v1/positions/${id}`)
}

// 获取岗位下的用户列表
export const getPositionUsers = (postId: number, params?: { page?: number; pageSize?: number }) => {
  return request.get(`/api/v1/positions/${postId}/users`, { params })
}

// 分配用户到岗位
export const assignUsersToPosition = (postId: number, userIds: number[]) => {
  return request.post(`/api/v1/positions/${postId}/users`, { userIds })
}

// 移除岗位下的用户
export const removeUserFromPosition = (postId: number, userId: number) => {
  return request.delete(`/api/v1/positions/${postId}/users/${userId}`)
}
