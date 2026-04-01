import request from '@/utils/request'
import type { PluginInfo } from '@/plugins/types'

/**
 * 获取所有插件列表
 */
export const listPlugins = () => {
  return request.get<any, PluginInfo[]>('/api/v1/plugins')
}

/**
 * 获取插件详情
 */
export const getPlugin = (name: string) => {
  return request.get<any, PluginInfo>(`/api/v1/plugins/${name}`)
}

/**
 * 获取插件菜单配置
 */
export const getPluginMenus = (name: string) => {
  return request.get<any, any>(`/api/v1/plugins/${name}/menus`)
}

/**
 * 启用插件
 */
export const enablePlugin = (name: string) => {
  return request.post(`/api/v1/plugins/${name}/enable`)
}

/**
 * 禁用插件
 */
export const disablePlugin = (name: string) => {
  return request.post(`/api/v1/plugins/${name}/disable`)
}

/**
 * 上传并安装插件
 */
export const uploadPlugin = (file: File) => {
  const formData = new FormData()
  formData.append('file', file)

  return request.post('/api/v1/plugins/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}

/**
 * 卸载插件
 */
export const uninstallPlugin = (name: string) => {
  return request.delete(`/api/v1/plugins/${name}/uninstall`)
}
