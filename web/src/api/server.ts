/**
 * 服务器与认证相关 API
 *
 * 对应后端 /api/auth、/api/servers 接口：
 * - 登录 / 登出：绑定 LDAP 并将连接凭据缓存到服务端会话
 * - 服务器配置：LDAP 连接的增删改查管理
 */
import { request } from '@/utils/request'
import type { LoginRequest, LoginResponse, LdapServer } from '@/types'

/**
 * 用户登录
 *
 * 使用 LDAP bind DN 与密码进行认证，成功后服务端缓存连接凭据，
 * 后续业务接口通过会话凭据访问 LDAP。
 *
 * @param data 登录请求（主机、端口、Bind DN、密码等）
 * @returns 登录响应（用户信息、可访问的服务器列表等）
 */
export function login(data: LoginRequest) {
  return request<LoginResponse>({
    url: '/auth/login',
    method: 'post',
    data,
  })
}

/**
 * 用户登出
 *
 * 销毁服务端缓存的 LDAP 连接凭据与本地会话。
 */
export function logout() {
  return request({ url: '/auth/logout', method: 'post' })
}

/** 登录页可展示的最小服务器信息（不含 bind_dn/base_dn） */
export interface PublicServerInfo {
  id: number
  name: string
  host: string
  port: number
  use_tls: boolean
  use_ssl: boolean
}

/**
 * 获取登录页可用的服务器列表
 *
 * 无需认证，仅返回连接目标与名称；不包含管理员绑定 DN、BaseDN 等敏感字段。
 *
 * @returns 最小服务器信息列表
 */
export function getPublicServers() {
  return request<PublicServerInfo[]>({ url: '/servers/public', method: 'get' })
}

/**
 * 获取服务器配置列表（仅管理员可访问）
 *
 * @returns 已配置的 LDAP 服务器列表
 */
export function getServers() {
  return request<LdapServer[]>({ url: '/servers', method: 'get' })
}

/**
 * 创建服务器配置
 *
 * @param data 服务器配置字段（host、port、base_dn 等）
 * @returns 创建成功后的完整服务器配置
 */
export function createServer(data: Partial<LdapServer>) {
  return request<LdapServer>({ url: '/servers', method: 'post', data })
}

/**
 * 更新服务器配置
 *
 * @param id 服务器配置 ID
 * @param data 需要更新的配置字段
 * @returns 更新后的完整服务器配置
 */
export function updateServer(id: number, data: Partial<LdapServer>) {
  return request<LdapServer>({ url: `/servers/${id}`, method: 'put', data })
}

/**
 * 删除服务器配置
 *
 * @param id 服务器配置 ID
 */
export function deleteServer(id: number) {
  return request({ url: `/servers/${id}`, method: 'delete' })
}

/**
 * 测试 LDAP 连接
 *
 * 使用给定配置（可携带临时的 bind_dn/password）尝试连接目标 LDAP 服务器，
 * 不保存配置，仅用于连通性与凭据校验。
 *
 * @param data 服务器配置 + 可选的临时绑定凭据
 * @returns connected 是否连接成功，message 结果说明
 */
export function testConnection(data: Partial<LdapServer> & { bind_dn?: string; password?: string }) {
  return request<{ connected: boolean; message: string }>({
    url: '/servers/test',
    method: 'post',
    data,
  })
}

/** RootDSE 元信息类型 */
export interface RootDSEInfo {
  namingContexts: string[]
  supportedLDAPVersion: string[]
  supportedSASLMechanisms: string[]
  supportedBindMethods: string[]
  vendorName: string
  vendorVersion: string
  serverName: string
  subschemaSubentry: string
}

/**
 * 获取 LDAP 服务器 rootDSE 元信息（对标 phpLDAPadmin server_info.php）
 * 展示 namingContexts / supportedSASLMechanisms / vendorName 等
 */
export function getRootDSE() {
  return request<RootDSEInfo>({ url: '/rootdse', method: 'get' })
}
