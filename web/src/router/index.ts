/**
 * 路由配置
 *
 * 应用全部页面路由定义与全局路由守卫：
 * - /login 登录页（独立布局）
 * - / 主布局下的功能页（目录管理、服务器管理、搜索、Schema 等）
 * - 全局守卫基于 localStorage token 做登录态校验，未登录跳转 /login
 */
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    // 登录页：无需登录即可访问
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/Login.vue'),
    meta: { title: '登录' },
  },
  {
    // 主布局：所有功能页面挂载在 MainLayout 下，默认重定向到目录管理
    path: '/',
    component: () => import('@/views/layout/MainLayout.vue'),
    redirect: '/entry',
    children: [
      {
        path: 'entry',
        name: 'Entry',
        component: () => import('@/views/entry/EntryView.vue'),
        meta: { title: '目录管理' },
      },
      {
        path: 'servers',
        name: 'Servers',
        component: () => import('@/views/servers/ServerManager.vue'),
        meta: { title: '服务器管理' },
      },
      {
        path: 'search',
        name: 'Search',
        component: () => import('@/views/search/AdvancedSearch.vue'),
        meta: { title: '高级搜索' },
      },
      {
        path: 'schema',
        name: 'Schema',
        component: () => import('@/views/schema/SchemaBrowser.vue'),
        meta: { title: 'Schema 浏览' },
      },
      {
        path: 'bookmarks',
        name: 'Bookmarks',
        component: () => import('@/views/bookmarks/BookmarkList.vue'),
        meta: { title: '书签' },
      },
      {
        path: 'monitor',
        name: 'Monitor',
        component: () => import('@/views/monitor/ServerMonitor.vue'),
        meta: { title: '服务器监控' },
      },
      {
        path: 'server-info',
        name: 'ServerInfo',
        component: () => import('@/views/monitor/ServerInfo.vue'),
        meta: { title: '服务器信息' },
      },
      {
        path: 'import',
        name: 'Import',
        component: () => import('@/views/import/LdifImport.vue'),
        meta: { title: '批量导入' },
      },
      {
        path: 'logs',
        name: 'Logs',
        component: () => import('@/views/logs/OperationLogs.vue'),
        meta: { title: '操作日志' },
      },
    ],
  },
  {
    // 兜底路由：未匹配路径统一重定向到主页
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

/**
 * 全局路由守卫（登录态校验）
 *
 * - 访问非 /login 页面且无 token：重定向到 /login
 * - 已登录访问 /login：重定向到主页（避免重复登录）
 */
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (to.path !== '/login' && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/')
  } else {
    next()
  }
})

export default router
