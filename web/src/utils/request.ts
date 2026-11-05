/**
 * HTTP 请求封装
 *
 * 基于 axios 的统一请求实例：
 * - baseURL 指向 /api（由 Go 端静态托管或反向代理转发）
 * - 请求拦截器：自动携带 Token 与 TraceID（全链路追踪）
 * - 响应拦截器：解包统一响应结构 { code, message, data, trace_id }，
 *   code=0 视为成功；code=401 清理本地 token 并跳转登录页
 *   （登录页内不重复跳转，避免无限重定向循环）
 */
import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

const service: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 请求拦截器：附加认证 Token 与 TraceID
service.interceptors.request.use(
  (config) => {
    const token = sessionStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    // 生成 TraceID：优先复用已有值，否则按 trace-时间戳-随机串 生成
    config.headers['X-Trace-ID'] =
      sessionStorage.getItem('traceId') ||
      `trace-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
    return config
  },
  (error) => Promise.reject(error),
)

// 判断当前是否已在登录页，防止 401 循环重定向
function isOnLoginPage() {
  return window.location.pathname.startsWith('/login')
}

// 响应拦截器：统一解包响应结构，处理业务错误与 401
service.interceptors.response.use(
  (response) => {
    const res = response.data
    // 统一响应结构 { code, message, data, trace_id }
    if (res && typeof res === 'object' && 'code' in res) {
      if (res.code === 0) {
        // 业务成功：直接返回 data 部分
        return res.data
      }
      // 401 但已在登录页，不再重复跳转，避免无限循环
      if (res.code === 401 && !isOnLoginPage()) {
        sessionStorage.removeItem('token')
        ElMessage.error(res.message || '登录已过期，请重新登录')
        window.location.href = '/login'
        return Promise.reject(new Error(res.message || 'Unauthorized'))
      }
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || 'Error'))
    }
    // 非统一结构（如 blob 下载）原样返回
    return res
  },
  (error) => {
    const status = error.response?.status
    if (status === 401 && !isOnLoginPage()) {
      // HTTP 401：清理 token 并跳转登录页（登录页内不跳转）
      sessionStorage.removeItem('token')
      window.location.href = '/login'
      ElMessage.error('登录已过期，请重新登录')
    } else if (status !== 401) {
      ElMessage.error(error.response?.data?.message || error.message || '网络请求失败')
    }
    return Promise.reject(error)
  },
)

/**
 * 发起 HTTP 请求
 *
 * 泛型封装：统一响应结构已在拦截器中解包，T 即业务 data 的类型。
 *
 * @param config axios 请求配置（url、method、params/data 等）
 * @returns 业务数据
 */
export function request<T = unknown>(config: AxiosRequestConfig): Promise<T> {
  return service.request<unknown, T>(config) as unknown as Promise<T>
}

export default service
