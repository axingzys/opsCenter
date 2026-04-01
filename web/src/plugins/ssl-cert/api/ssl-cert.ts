import request from '@/utils/request'

// 证书管理 API

// 获取证书列表
export function getCertificates(params?: { page?: number; page_size?: number; domain?: string; status?: string; source_type?: string }) {
  return request({
    url: '/api/v1/plugins/ssl-cert/certificates',
    method: 'get',
    params
  })
}

// 获取证书详情
export function getCertificate(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}`,
    method: 'get'
  })
}

// 申请证书
export function createCertificate(data: {
  name: string
  domain: string
  san_domains?: string[]
  acme_email?: string
  source_type: string
  ca_provider?: string
  key_algorithm?: string
  dns_provider_id?: number
  cloud_account_id?: number
  auto_renew?: boolean
  renew_days_before?: number
}) {
  return request({
    url: '/api/v1/plugins/ssl-cert/certificates',
    method: 'post',
    data
  })
}

// 导入证书
export function importCertificate(data: {
  name: string
  domain: string
  san_domains?: string[]
  certificate: string
  private_key: string
  cert_chain?: string
  auto_renew?: boolean
  renew_days_before?: number
}) {
  return request({
    url: '/api/v1/plugins/ssl-cert/certificates/import',
    method: 'post',
    data
  })
}

// 更新证书配置
export function updateCertificate(id: number, data: {
  name?: string
  auto_renew?: boolean
  renew_days_before?: number
  dns_provider_id?: number
  acme_email?: string
}) {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}`,
    method: 'put',
    data
  })
}

// 删除证书
export function deleteCertificate(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}`,
    method: 'delete'
  })
}

// 手动续期证书
export function renewCertificate(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}/renew`,
    method: 'post'
  })
}

// 同步云证书状态
export function syncCertificate(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}/sync`,
    method: 'post'
  })
}

// 下载证书
export function downloadCertificate(id: number, format: string = 'pem') {
  return request({
    url: `/api/v1/plugins/ssl-cert/certificates/${id}/download`,
    method: 'get',
    params: { format }
  })
}

// 获取证书统计
export function getCertificateStats() {
  return request({
    url: '/api/v1/plugins/ssl-cert/certificates/stats',
    method: 'get'
  })
}

// 获取云账号列表(用于云厂商证书申请)
export function getCloudAccounts(provider?: string) {
  return request({
    url: '/api/v1/plugins/ssl-cert/certificates/cloud-accounts',
    method: 'get',
    params: provider ? { provider } : undefined
  })
}

// DNS Provider API

// 获取DNS Provider列表
export function getDNSProviders(params?: { page?: number; page_size?: number; name?: string; provider?: string }) {
  return request({
    url: '/api/v1/plugins/ssl-cert/dns-providers',
    method: 'get',
    params
  })
}

// 获取所有启用的DNS Provider
export function getAllDNSProviders() {
  return request({
    url: '/api/v1/plugins/ssl-cert/dns-providers/all',
    method: 'get'
  })
}

// 获取DNS Provider详情
export function getDNSProvider(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/dns-providers/${id}`,
    method: 'get'
  })
}

// 获取DNS Provider完整详情(用于编辑)
export function getDNSProviderDetail(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/dns-providers/${id}/detail`,
    method: 'get'
  })
}

// 创建DNS Provider
export function createDNSProvider(data: {
  name: string
  provider: string
  config: object
  email?: string
  phone?: string
  enabled?: boolean
}) {
  return request({
    url: '/api/v1/plugins/ssl-cert/dns-providers',
    method: 'post',
    data
  })
}

// 更新DNS Provider
export function updateDNSProvider(id: number, data: {
  name?: string
  config?: object
  email?: string
  phone?: string
  enabled?: boolean
}) {
  return request({
    url: `/api/v1/plugins/ssl-cert/dns-providers/${id}`,
    method: 'put',
    data
  })
}

// 删除DNS Provider
export function deleteDNSProvider(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/dns-providers/${id}`,
    method: 'delete'
  })
}

// 测试DNS Provider连接
export function testDNSProvider(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/dns-providers/${id}/test`,
    method: 'post'
  })
}

// 部署配置 API

// 获取部署配置列表
export function getDeployConfigs(params?: { page?: number; page_size?: number; certificate_id?: number; deploy_type?: string }) {
  return request({
    url: '/api/v1/plugins/ssl-cert/deploy-configs',
    method: 'get',
    params
  })
}

// 获取部署配置详情
export function getDeployConfig(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/deploy-configs/${id}`,
    method: 'get'
  })
}

// 创建部署配置
export function createDeployConfig(data: {
  certificate_id: number
  name: string
  deploy_type: string
  target_config: object
  auto_deploy?: boolean
  enabled?: boolean
}) {
  return request({
    url: '/api/v1/plugins/ssl-cert/deploy-configs',
    method: 'post',
    data
  })
}

// 更新部署配置
export function updateDeployConfig(id: number, data: {
  name?: string
  target_config?: object
  auto_deploy?: boolean
  enabled?: boolean
}) {
  return request({
    url: `/api/v1/plugins/ssl-cert/deploy-configs/${id}`,
    method: 'put',
    data
  })
}

// 删除部署配置
export function deleteDeployConfig(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/deploy-configs/${id}`,
    method: 'delete'
  })
}

// 执行部署
export function executeDeploy(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/deploy-configs/${id}/deploy`,
    method: 'post'
  })
}

// 测试部署配置
export function testDeployConfig(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/deploy-configs/${id}/test`,
    method: 'post'
  })
}

// 任务 API

// 获取任务列表
export function getTasks(params?: { page?: number; page_size?: number; certificate_id?: number; task_type?: string; status?: string; trigger_type?: string }) {
  return request({
    url: '/api/v1/plugins/ssl-cert/tasks',
    method: 'get',
    params
  })
}

// 获取任务详情
export function getTask(id: number) {
  return request({
    url: `/api/v1/plugins/ssl-cert/tasks/${id}`,
    method: 'get'
  })
}
