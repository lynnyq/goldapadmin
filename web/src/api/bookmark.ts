/**
 * 书签相关 API
 *
 * 对应后端 /api/bookmarks 接口，用于收藏常用 DN，便于快速跳转。
 */
import { request } from '@/utils/request'
import type { Bookmark } from '@/types'

/**
 * 获取书签列表
 *
 * @returns 当前用户收藏的书签列表
 */
export function getBookmarks() {
  return request<Bookmark[]>({ url: '/bookmarks', method: 'get' })
}

/**
 * 添加书签
 *
 * @param data name 书签显示名称；dn 收藏的条目 DN
 * @returns 创建成功后的书签对象
 */
export function addBookmark(data: { name: string; dn: string }) {
  return request<Bookmark>({ url: '/bookmarks', method: 'post', data })
}

/**
 * 删除书签
 *
 * @param id 书签 ID
 */
export function deleteBookmark(id: number) {
  return request({ url: `/bookmarks/${id}`, method: 'delete' })
}
