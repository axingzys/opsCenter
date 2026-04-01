import type { Plugin, PluginMenuConfig, PluginRouteConfig } from '../types'
import { pluginManager } from '../manager'

/**
 * SSL证书管理插件
 * 提供SSL证书申请、续期、部署等功能
 */
class SSLCertPlugin implements Plugin {
  name = 'ssl-cert'
  prettyName = 'SSL证书'
  description = 'SSL证书自动续期插件，支持Let\'s Encrypt和云厂商证书服务，自动部署到Nginx和K8s'
  version = '1.0.0'
  author = 'J'

  /**
   * 安装插件
   */
  async install() {
    // 初始化操作
  }

  /**
   * 卸载插件
   */
  async uninstall() {
    // 清理资源
  }

  /**
   * 获取插件菜单配置
   */
  getMenus(): PluginMenuConfig[] {
    const parentPath = '/ssl-cert'

    return [
      {
        name: 'SSL证书',
        path: parentPath,
        icon: 'Key',
        sort: 25,
        hidden: false,
        parentPath: '',
      },
      {
        name: '证书管理',
        path: '/ssl-cert/certificates',
        icon: 'Document',
        sort: 1,
        hidden: false,
        parentPath: parentPath,
      },
      {
        name: 'DNS验证配置',
        path: '/ssl-cert/dns-providers',
        icon: 'Connection',
        sort: 2,
        hidden: false,
        parentPath: parentPath,
      },
      {
        name: '部署配置',
        path: '/ssl-cert/deploy-configs',
        icon: 'Upload',
        sort: 3,
        hidden: false,
        parentPath: parentPath,
      },
      {
        name: '任务记录',
        path: '/ssl-cert/tasks',
        icon: 'List',
        sort: 4,
        hidden: false,
        parentPath: parentPath,
      },
    ]
  }

  /**
   * 获取插件路由配置
   */
  getRoutes(): PluginRouteConfig[] {
    return [
      {
        path: '/ssl-cert',
        name: 'SSLCert',
        component: () => import('./components/CertificateList.vue'),
        redirect: '/ssl-cert/certificates',
        meta: { title: 'SSL证书' },
      },
      {
        path: '/ssl-cert/certificates',
        name: 'CertificateList',
        component: () => import('./components/CertificateList.vue'),
        meta: { title: '证书管理' },
      },
      {
        path: '/ssl-cert/dns-providers',
        name: 'DNSProviderList',
        component: () => import('./components/DNSProviderList.vue'),
        meta: { title: 'DNS验证配置' },
      },
      {
        path: '/ssl-cert/deploy-configs',
        name: 'DeployConfigList',
        component: () => import('./components/DeployConfigList.vue'),
        meta: { title: '部署配置' },
      },
      {
        path: '/ssl-cert/tasks',
        name: 'TaskList',
        component: () => import('./components/TaskList.vue'),
        meta: { title: '任务记录' },
      },
    ]
  }
}

// 创建并注册插件实例
const plugin = new SSLCertPlugin()
pluginManager.register(plugin)

export default plugin
