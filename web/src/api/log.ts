/**
 * 操作日志相关 API
 *
 * 对应后端 /api/v1/logs 接口，用于查询用户在系统内执行的写操作审计记录。
 */
import { request } from '@/utils/request'
import type { OperationLog } from '@/types'

/**
 * 分页查询操作日志
 *
 * @param params.page 页码（从 1 开始）
 * @param params.page_size 每页条数
 * @param params.action 按操作类型过滤（如 create/modify/delete 等，可选）
 * @returns total 总条数；list 当前页日志列表
 */
export function getLogs(params: { page?: number; page_size?: number; action?: string }) {
  return request<{ total: number; list: OperationLog[] }>({
    url: '/logs',
    method: 'get',
    params,
  })
}
