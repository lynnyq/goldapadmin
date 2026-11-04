/**
 * 搜索相关 API
 *
 * 对应后端 /api/v1/search 接口，提供 LDAP 高级搜索能力
 * （自定义 base DN、filter、范围与返回属性）。
 */
import { request } from '@/utils/request'
import type { LdapEntry } from '@/types'

/**
 * 高级搜索
 *
 * @param params.base_dn 搜索起始 DN
 * @param params.filter LDAP 过滤表达式（如 (objectClass=*)）
 * @param params.scope 搜索范围：base 仅本条目 / one 单层 / sub 子树（默认 sub）
 * @param params.attributes 仅返回的属性列表（可选，缺省返回全部）
 * @param params.size_limit 返回条目数上限（可选）
 * @returns 命中的条目列表
 */
export function search(params: {
  base_dn: string
  filter: string
  scope?: 'base' | 'one' | 'sub'
  attributes?: string[]
  size_limit?: number
}) {
  const { attributes, ...rest } = params
  return request<LdapEntry[]>({
    url: '/search',
    method: 'get',
    params: rest,
    // 数组用重复键格式 attributes=cn&attributes=gidNumber，
    // 否则 Gin 的 QueryArray 无法正确解析（默认序列化会带方括号）
    paramsSerializer: {
      serialize: (p) => {
        const usp = new URLSearchParams()
        for (const [k, v] of Object.entries(p)) {
          if (v === undefined || v === null || v === '') continue
          usp.append(k, String(v))
        }
        if (attributes) {
          for (const a of attributes) usp.append('attributes', a)
        }
        return usp.toString()
      },
    },
  })
}
