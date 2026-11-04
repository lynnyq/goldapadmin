/**
 * 服务器状态 Store
 *
 * 管理已配置的 LDAP 服务器列表与当前选中的服务器，
 * 当前服务器 ID 持久化到 localStorage，刷新后自动恢复。
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getServers } from '@/api/server'
import type { LdapServer } from '@/types'

export const useServerStore = defineStore('server', () => {
  /** 服务器配置列表 */
  const servers = ref<LdapServer[]>([])
  /** 当前选中服务器 ID（从 localStorage 恢复） */
  const currentServerId = ref<number>(
    Number(localStorage.getItem('currentServerId')) || 0,
  )

  /**
   * 加载服务器列表
   *
   * 从后端拉取全部服务器配置并写入 servers。
   * 保证赋值结果是数组，即使后端返回 null 也降级为空数组。
   */
  async function loadServers() {
    const list = await getServers()
    servers.value = Array.isArray(list) ? list : []
  }

  /**
   * 设置当前选中的服务器
   *
   * @param id 服务器配置 ID
   */
  function setCurrentServer(id: number) {
    currentServerId.value = id
    localStorage.setItem('currentServerId', String(id))
  }

  /**
   * 获取当前选中的服务器配置对象
   *
   * @returns 当前服务器配置；未选中或不存在时返回 undefined
   */
  const currentServer = () =>
    servers.value.find((s) => s.id === currentServerId.value)

  return { servers, currentServerId, loadServers, setCurrentServer, currentServer }
})
