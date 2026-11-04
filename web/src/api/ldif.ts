/**
 * LDIF 导入导出 API
 *
 * 对应后端 /api/ldif 路由组。
 * ExportLdif 使用 fetch + Blob 触发浏览器下载（不是 request 封装，因为需要处理原始 Content-Disposition）。
 */
import { useAuthStore } from '@/store/auth'

function getToken(): string {
  // 优先从 Pinia store 取，store 未初始化时从 localStorage 兜底
  try {
    return useAuthStore().token
  } catch {
    return localStorage.getItem('token') || ''
  }
}

/**
 * 导出 LDIF 文件（触发浏览器下载）
 *
 * @param dn 要导出的 base DN
 * @param recursive 是否递归导出所有子孙节点（默认 true）
 * @returns 下载的文件名（从 Content-Disposition 解析）
 */
export async function exportLdif(dn: string, recursive = true): Promise<string> {
  const token = getToken()
  if (!token) throw new Error('未登录')

  const params = new URLSearchParams({ dn, recursive: recursive ? 'true' : 'false' })
  const resp = await fetch(`/api/ldif/export?${params}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!resp.ok) {
    const body = await resp.text()
    let msg = `导出失败: HTTP ${resp.status}`
    try {
      const j = JSON.parse(body)
      if (j.message) msg = j.message
    } catch { /* ignore */ }
    throw new Error(msg)
  }

  // 从 Content-Disposition 解析文件名
  let filename = `${dn.split(',')[0].split('=')[1] || 'export'}.ldif`
  const cd = resp.headers.get('Content-Disposition') || ''
  // 优先取 RFC 5987 编码的 filename*=
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
 * @param content LDIF 文本
 * @returns 导入结果（success/failed/errors）
 */
export function importLdif(content: string) {
  return fetch('/api/ldif/import', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify({ content }),
  }).then(async (r) => {
    const j = await r.json()
    if (j.code !== 0) throw new Error(j.message || '导入失败')
    return j.data as { success: number; failed: number; errors: string[] }
  })
}
