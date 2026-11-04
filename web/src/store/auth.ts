/**
 * 认证状态 Store
 *
 * 管理登录令牌、当前用户 DN 与服务器名称，并同步持久化到 localStorage，
 * 保证页面刷新后登录态不丢失。注意：密码不落盘，仅服务端会话缓存凭据。
 */
import { defineStore } from 'pinia'
import { login as apiLogin, logout as apiLogout } from '@/api/server'
import type { LoginRequest } from '@/types'

/** 认证状态结构 */
interface AuthState {
  /** 访问令牌（JWT） */
  token: string
  /** 当前登录用户的 Bind DN */
  userDN: string
  /** 当前登录的 LDAP 服务器名称 */
  serverName: string
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: localStorage.getItem('token') || '',
    userDN: localStorage.getItem('userDN') || '',
    serverName: localStorage.getItem('serverName') || '',
  }),
  actions: {
    /**
     * 执行登录
     *
     * 调用登录接口，成功后将 token、用户 DN、服务器名称
     * 写入 state 与 localStorage。
     *
     * @param data 登录请求参数（主机、端口、Bind DN、密码等）
     * @returns 登录响应（含 token 与用户信息）
     */
    async login(data: LoginRequest) {
      const res = await apiLogin(data)
      this.token = res.token
      this.userDN = res.user_dn
      this.serverName = res.server?.name || ''
      localStorage.setItem('token', res.token)
      localStorage.setItem('userDN', res.user_dn)
      localStorage.setItem('serverName', this.serverName)
      return res
    },
    /**
     * 执行登出
     *
     * 先调用登出接口销毁服务端凭据缓存（失败不阻断本地清理），
     * 再清空本地 state 与 localStorage。
     */
    async logout() {
      try {
        await apiLogout()
      } catch {
        // 登出接口失败时忽略错误，仍然完成本地状态清理
      }
      this.token = ''
      this.userDN = ''
      this.serverName = ''
      localStorage.removeItem('token')
      localStorage.removeItem('userDN')
      localStorage.removeItem('serverName')
    },
  },
})
