import request from '@/utils/request'

// 获取部门树
export const getDepartmentTree = () => {
  return request.get('/api/v1/departments/tree')
}

// 获取部门详情
export const getDepartment = (id: number) => {
  return request.get(`/api/v1/departments/${id}`)
}

// 创建部门
export const createDepartment = (data: any) => {
  return request.post('/api/v1/departments', data)
}

// 更新部门
export const updateDepartment = (id: number, data: any) => {
  return request.put(`/api/v1/departments/${id}`, data)
}

// 删除部门
export const deleteDepartment = (id: number) => {
  return request.delete(`/api/v1/departments/${id}`)
}

// 获取父级部门选项（用于级联选择器）
export const getParentOptions = () => {
  return request.get('/api/v1/departments/parent-options')
}
