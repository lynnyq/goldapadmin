/**
 * 服务监控相关 API
 *
 * 对应后端 /api/v1/monitor 接口，用于展示 LDAP 服务器运行状态信息
 * （对标 phpLDAPadmin 的服务器监控页）。
 */
import { request } from '@/utils/request'
import type { MonitorStats } from '@/types'

/**
 * 获取监控统计信息
 *
 * @returns LDAP 服务运行指标（版本、条目数、连接数等）
 */
export function getMonitorStats() {
  return request<MonitorStats>({ url: '/monitor/stats', method: 'get' })
}
