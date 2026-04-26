import request from '@/utils/request'

// 终端审计API

/**
 * 获取终端会话列表
 */
export const getTerminalSessions = (params: {
  page: number
  pageSize: number
  keyword?: string
}) => {
  return request.get('/api/v1/terminal-sessions', { params })
}

/**
 * 播放终端会话录制
 */
export const playTerminalSession = (id: number) => {
  return request.get(`/api/v1/terminal-sessions/${id}/play`, {
    responseType: 'text'
  })
}

/**
 * 下载终端会话录制
 */
export const downloadTerminalSession = (id: number) => {
  return request.get(`/api/v1/terminal-sessions/${id}/download`, {
    responseType: 'blob'
  })
}

/**
 * 获取终端会话高危命令记录
 */
export const getTerminalSessionEvents = (id: number) => {
  return request.get(`/api/v1/terminal-sessions/${id}/events`)
}

/**
 * 删除终端会话
 */
export const deleteTerminalSession = (id: number) => {
  return request.delete(`/api/v1/terminal-sessions/${id}`)
}
