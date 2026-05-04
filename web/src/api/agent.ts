import request from '@/utils/request'

export const getAgentList = (params: any) => {
  return request.get('/api/v1/agents', { params })
}

export const deployAgents = (data: { hostIds: number[] }) => {
  return request.post('/api/v1/agents/deploy', data, {
    headers: {
      'X-OpsHub-Base-URL': window.location.origin
    }
  })
}

export const reinstallAgents = (data: { hostIds: number[] }) => {
  return request.post('/api/v1/agents/reinstall', data, {
    headers: {
      'X-OpsHub-Base-URL': window.location.origin
    }
  })
}

export const uninstallAgents = (data: { hostIds: number[] }) => {
  return request.post('/api/v1/agents/uninstall', data)
}

export const getAgentJob = (id: number) => {
  return request.get(`/api/v1/agents/jobs/${id}`)
}

export const getAgentInventory = (hostId: number) => {
  return request.get(`/api/v1/agents/${hostId}/inventory`)
}
