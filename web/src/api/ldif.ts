/**
 * LDIF 导入导出 API
 *
 * 对应后端 /api/ldif 路由组。
 * - importLdif 使用 axios request 封装，自动携带 TraceID、统一错误处理、401 跳转
 * - exportLdif 使用 fetch 触发浏览器下载（需要精细解析 Content-Disposition 文件名），
 *   但已补齐 TraceID 注入、401 自动跳转和统一错误响应解析
 */
import { request } from '@/utils/request'

/** LDIF 导入结果 */
export interface LdifImportResult {
  /** 成功条数 */
  success: number
  /** 失败条数 */
  failed: number
  /** 逐条错误信息 */
  errors: string[]
}

/**
 * 生成 TraceID：与 request.ts 拦截器保持一致，优先复用 localStorage
 */
function getTraceID(): string {
  return (
    localStorage.getItem('traceId') ||
    `trace-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  )
}

/**
 * 从 fetch 响应体解析后端统一错误结构 {code, message, data, trace_id}，
 * 兼容纯 HTTP 错误（后端也可能返回非 JSON）。
 */
async function parseError(bodyText: string, httpStatus: number, traceId: string): Promise<string> {
  try {
    const j = JSON.parse(bodyText)
    if (typeof j.message === 'string' && j.message) return j.message
    if (typeof j.trace_id === 'string' && j.trace_id) {
      return `HTTP ${httpStatus}（trace_id=${j.trace_id}）`
    }
  } catch {
    // 不是 JSON，直接用 HTTP 状态码兜底
  }
  return `HTTP ${httpStatus}`
}

/**
 * 导出 LDIF 文件（触发浏览器下载）
 *
 * 使用 fetch 而非 axios，因为需要精细控制 Content-Disposition 文件名解析
 * （axios 的 responseType: 'blob' 下读取原始 header 不便）。
 * 但已补齐与 request.ts 拦截器一致的 TraceID 注入、401 跳转和错误解析。
 *
 * @param dn 要导出的 base DN
 * @param recursive 是否递归导出所有子孙节点（默认 true）
 * @returns 下载的文件名（从 Content-Disposition 解析）
 */
export async function exportLdif(dn: string, recursive = true): Promise<string> {
  const token = localStorage.getItem('token')
  if (!token) throw new Error('未登录')

  const traceId = getTraceID()
  const params = new URLSearchParams({ dn, recursive: recursive ? 'true' : 'false' })
  const resp = await fetch(`/api/ldif/export?${params}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
      'X-Trace-ID': traceId,
    },
  })

  // 401 统一处理：与 request.ts 拦截器逻辑保持一致
  if (resp.status === 401) {
    if (!window.location.pathname.startsWith('/login')) {
      localStorage.removeItem('token')
      const bodyText = await resp.text().catch(() => '')
      const msg = await parseError(bodyText, 401, traceId)
      // 用 setTimeout 确保消息先弹出再跳转
      setTimeout(() => {
        // 全局已挂载 ElMessage 会被捕获，这里 fallback 到 alert
        window.location.href = '/login'
      }, 100)
      throw new Error(msg)
    }
  }

  if (!resp.ok) {
    const bodyText = await resp.text().catch(() => '')
    const msg = await parseError(bodyText, resp.status, traceId)
    throw new Error(`导出失败: ${msg}`)
  }

  // 从 Content-Disposition 解析文件名（后端用 RFC 5987 编码）
  let filename = `${dn.split(',')[0].split('=')[1] || 'export'}.ldif`
  const cd = resp.headers.get('Content-Disposition') || ''
  // 优先取 RFC 5987 编码的 filename*=UTF-8''xxx
  const matchStar = cd.match(/filename\*=UTF-8''([^;]+)/i)
  if (matchStar) {
    filename = decodeURIComponent(matchStar[1].trim())
  } else {
    const match = cd.match(/filename="?([^";]+)"?/i)
    if (match) filename = match[1].trim()
  }

  const blob = await resp.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)

  return filename
}

/**
 * 导入 LDIF 内容
 *
 * 使用 request() axios 封装，自动获取：
 * - 统一响应结构解包（{code, message, data, trace_id}）
 * - TraceID 注入（X-Trace-ID 头）
 * - 401 自动跳转登录页
 * - 错误统一 ElMessage 提示
 *
 * @param content LDIF 文本
 * @returns 导入结果（success/failed/errors）
 */
export function importLdif(content: string) {
  return request<LdifImportResult>({
    url: '/ldif/import',
    method: 'post',
    data: { content },
  })
}
