import axios from 'axios'
import request from '@/utils/request'

/**
 * 上传头像
 * @param file 头像文件
 */
export const uploadAvatar = (file: File) => {
  const formData = new FormData()
  formData.append('file', file)

  // 直接使用 axios,绕过响应拦截器
  return axios.post('/api/v1/upload/avatar', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    }
  }).then(response => {
    // 返回完整的响应对象,包含 code, message, data
    return response.data
  })
}

/**
 * 更新用户头像
 * @param avatarUrl 头像URL
 */
export const updateUserAvatar = (avatarUrl: string) => {
  return request.put('/api/v1/profile/avatar', { avatar: avatarUrl })
}
