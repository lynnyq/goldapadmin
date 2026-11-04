/**
 * 条目操作相关 API
 *
 * 对应后端 /api/entry 接口，覆盖条目的增删改查、复制/移动/重命名，
 * 以及目录树懒加载与自增 ID 分配（对标 phpLDAPadmin 的条目模板能力）。
 */
import { request } from '@/utils/request'
import type { LdapEntry, TreeNode, BatchResultItem } from '@/types'

/**
 * 获取子节点列表（目录树懒加载）
 *
 * @param baseDN 父条目 DN
 * @returns 直接子节点列表（含 DN、RDN、是否有子节点标记等）
 */
export function getChildren(baseDN: string) {
  return request<TreeNode[]>({
    url: '/entry/children',
    method: 'get',
    params: { base_dn: baseDN },
  })
}

/**
 * 递归获取指定 baseDN 下完整目录树（对标 phpLDAPadmin 全量树导出）
 *
 * 与 getChildren（懒加载单层）不同，该接口一次性拉取所有子孙节点
 * 并返回嵌套结构；适合较小规模目录或导出预览场景。
 *
 * @param baseDN 根条目 DN
 * @returns 嵌套树结构（含所有子孙）
 */
export function getTree(baseDN: string) {
  return request<TreeNode>({
    url: '/entry/tree',
    method: 'get',
    params: { base_dn: baseDN },
  })
}

/**
 * 获取条目详情
 *
 * @param dn 条目完整 DN
 * @returns 条目对象（含全部属性与 objectClass）
 */
export function getEntry(dn: string) {
  return request<LdapEntry>({
    url: '/entry/get',
    method: 'get',
    params: { dn },
  })
}

/**
 * 获取下一个可用的 uidNumber/gidNumber（对标 phpLDAPadmin 自增 ID 分配）
 *
 * @param type uid 取最大 uidNumber+1；gid 取最大 gidNumber+1
 * @returns next 下一个可用编号
 */
export function getNextId(type: 'uid' | 'gid') {
  return request<{ next: number }>({
    url: '/entry/next_id',
    method: 'get',
    params: { type },
  })
}

/**
 * 创建条目
 *
 * @param data.dn 新条目完整 DN
 * @param data.attributes 条目属性（objectClass 必须包含）
 * @param data.password_algos 密码属性哈希算法映射（属性名 → 算法），对标 phpLDAPadmin password helper
 */
export function createEntry(data: {
  dn: string
  attributes: Record<string, string[]>
  password_algos?: Record<string, string>
}) {
  return request({ url: '/entry/create', method: 'post', data })
}

/**
 * 修改条目（属性级变更）
 *
 * @param dn 目标条目 DN
 * @param changes 变更列表：op 操作类型（add/replace/delete）、attr 属性名、values 属性值
 * @param passwordAlgos 密码属性哈希算法映射（属性名 → 算法），对标 phpLDAPadmin password helper
 */
export function modifyEntry(
  dn: string,
  changes: Array<{ op: 'add' | 'replace' | 'delete'; attr: string; values?: string[] }>,
  passwordAlgos?: Record<string, string>,
) {
  return request({
    url: '/entry/modify',
    method: 'post',
    data: { dn, changes, password_algos: passwordAlgos },
  })
}

/**
 * 删除条目
 *
 * @param dn 目标条目 DN
 * @param recursive 是否递归删除整个子树（默认 false）
 */
export function deleteEntry(dn: string, recursive = false) {
  return request({
    url: '/entry/delete',
    method: 'delete',
    params: { dn, recursive },
  })
}

/**
 * 复制条目
 *
 * @param srcDN 源条目 DN
 * @param destDN 目标条目 DN（含新 RDN 的新位置）
 * @param recursive 是否递归复制整个子树（默认 false）
 */
export function copyEntry(srcDN: string, destDN: string, recursive = false, targetServerId?: number | null) {
  const data: any = { src_dn: srcDN, dest_dn: destDN, recursive }
  if (targetServerId != null) data.target_server_id = targetServerId
  return request({
    url: '/entry/copy',
    method: 'post',
    data,
  })
}

/**
 * 移动条目（跨父节点移动，可同时改名）
 *
 * @param srcDN 源条目 DN
 * @param newParentDN 新父条目 DN
 * @param newRDN 新 RDN（可选，缺省保留原 RDN）
 */
export function moveEntry(srcDN: string, newParentDN: string, newRDN?: string) {
  return request({
    url: '/entry/move',
    method: 'post',
    data: { src_dn: srcDN, new_parent_dn: newParentDN, new_rdn: newRDN },
  })
}

/**
 * 重命名条目（仅修改 RDN，位置不变）
 *
 * @param dn 目标条目 DN
 * @param newRDN 新 RDN
 */
export function renameEntry(dn: string, newRDN: string) {
  return request({
    url: '/entry/rename',
    method: 'post',
    data: { dn, new_rdn: newRDN },
  })
}

/**
 * 获取二进制属性的 base64 编码值（对标 phpLDAPadmin view_jpeg_photo.php）
 */
export function getBinaryAttr(dn: string, attr: string): Promise<{ base64: string; size: number }> {
  return request({
    url: '/entry/binary',
    method: 'get',
    params: { dn, attr },
  })
}

/**
 * 下载二进制属性原始文件到浏览器（对标 phpLDAPadmin download_binary_attr.php）
 * 使用 axios responseType=blob + Content-Disposition 自动取文件名
 */
export function downloadBinaryAttr(dn: string, attr: string) {
  return request({
    url: '/entry/binary-download',
    method: 'get',
    params: { dn, attr },
    responseType: 'blob',
  }).then((blob: any) => {
    // 后端已设置 Content-Disposition，浏览器可直接 download；这里兜底取 attr 作为文件名
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${attr}.bin`
    a.style.display = 'none'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  })
}

/**
 * 列出 baseDN 下所有子孙条目 DN（递归删除预览用，对标 phpLDAPadmin delete_form.php）
 */
export function listDescendants(baseDN: string): Promise<{ count: number; dns: string[] }> {
  return request({
    url: '/entry/descendants',
    method: 'get',
    params: { base_dn: baseDN },
  })
}

/** 批量操作结果（后端 BatchDelete / BatchModify 返回） */
export interface BatchResult {
  /** 总条目数 */
  total: number
  /** 成功条数 */
  success: number
  /** 失败条数 */
  failed: number
  /** 每条的详细结果（含失败原因） */
  items: BatchResultItem[]
  /** 批量操作总耗时（Go time.Duration，纳秒） */
  duration?: number
}

/**
 * 批量删除条目（对标 phpLDAPadmin mass_delete.php）
 */
export function batchDelete(dns: string[], recursive = false) {
  return request<BatchResult>({
    url: '/entry/batch',
    method: 'delete',
    data: { dns, recursive },
  })
}

/**
 * 批量修改条目同一属性（对标 phpLDAPadmin mass_edit.php / mass_update.php）
 */
export function batchModify(dns: string[], op: 'add' | 'replace' | 'delete', attr: string, values?: string[]) {
  return request<BatchResult>({
    url: '/entry/batch-modify',
    method: 'post',
    data: { dns, op, attr, values: values || [] },
  })
}

/**
 * LDAP Compare 操作（对标 phpLDAPadmin compare.php）
 */
export function compareEntry(dn: string, attr: string, value: string): Promise<{ matched: boolean }> {
  return request({
    url: '/entry/compare',
    method: 'post',
    data: { dn, attr, value },
  })
}
