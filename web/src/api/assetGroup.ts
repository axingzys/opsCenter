import request from '@/utils/request'

// 获取分组树
export const getGroupTree = () => {
  return request.get('/api/v1/asset-groups/tree')
}

// 获取分组详情
export const getGroup = (id: number) => {
  return request.get(`/api/v1/asset-groups/${id}`)
}

// 创建分组
export const createGroup = (data: any) => {
  return request.post('/api/v1/asset-groups', data)
}

// 更新分组
export const updateGroup = (id: number, data: any) => {
  return request.put(`/api/v1/asset-groups/${id}`, data)
}

// 删除分组
export const deleteGroup = (id: number) => {
  return request.delete(`/api/v1/asset-groups/${id}`)
}

// 获取父级分组选项（用于级联选择器）
export const getParentOptions = () => {
  return request.get('/api/v1/asset-groups/parent-options')
}
