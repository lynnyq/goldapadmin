/**
 * DN 解析与处理工具
 *
 * 提供 DN/RDN 的解析、转义、父子关系推导，
 * 以及基于 objectClass 的条目图标推断。
 */

/**
 * 解析 DN 字符串，返回各级 RDN 数组
 * "ou=users,dc=example,dc=com" => [{attr:"ou",value:"users"}, {attr:"dc",value:"example"}, {attr:"dc",value:"com"}]
 *
 * @param dn 待解析的 DN 字符串
 * @returns RDN 数组（attr 属性名，value 属性值）；空 DN 返回空数组
 */
export function parseDN(dn: string): { attr: string; value: string }[] {
  if (!dn) return []
  const parts: { attr: string; value: string }[] = []
  let current = ''
  let escaping = false
  for (let i = 0; i < dn.length; i++) {
    const ch = dn[i]
    if (escaping) {
      current += ch
      escaping = false
    } else if (ch === '\\') {
      current += ch
      escaping = true
    } else if (ch === ',') {
      parts.push(splitRDN(current))
      current = ''
    } else {
      current += ch
    }
  }
  if (current) parts.push(splitRDN(current))
  return parts
}

/**
 * 拆分单个 RDN 为属性名与属性值（按第一个 = 分割，无 = 时整体视为属性名）
 *
 * @param rdn 单个 RDN 字符串
 * @returns attr 属性名；value 属性值
 */
function splitRDN(rdn: string): { attr: string; value: string } {
  const idx = rdn.indexOf('=')
  if (idx === -1) return { attr: rdn, value: '' }
  return { attr: rdn.slice(0, idx).trim(), value: rdn.slice(idx + 1).trim() }
}

/**
 * 从 DN 中提取 RDN 值
 *
 * @param dn 条目 DN
 * @returns 第一级 RDN 的属性值；解析失败时返回原 DN
 */
export function getRDNValue(dn: string): string {
  const parts = parseDN(dn)
  return parts.length > 0 ? parts[0].value : dn
}

/**
 * 获取父 DN
 *
 * @param dn 条目 DN
 * @returns 去掉第一级 RDN 后的父 DN；已是根级别时返回空字符串
 */
export function getParentDN(dn: string): string {
  const parts = parseDN(dn)
  if (parts.length <= 1) return ''
  return parts
    .slice(1)
    .map((p) => `${p.attr}=${p.value}`)
    .join(',')
}

/**
 * 转义 RDN 值中的 DN 特殊字符（RFC 4514）
 * , + " \ < > ; 以及前导 # / 空格、尾部空格都需要转义，
 * 否则拼接出的 DN 会被 LDAP 服务端判定为 Invalid DN Syntax
 */
export function escapeDNValue(value: string): string {
  let v = value.trim()
  // 前导 # 会被解析为十六进制编码值，必须转义
  if (v.startsWith('#')) v = '\\' + v
  return v.replace(/([,+"\\<>;])/g, '\\$1')
}

/**
 * 校验 RDN 值是否为空或仅空白
 *
 * @param value 待校验的 RDN 值
 * @returns 是否有效（非空白）
 */
export function isValidRDNValue(value: string): boolean {
  return value.trim().length > 0
}

/**
 * 根据 objectClass 推断节点图标类型（对标 phpLDAPadmin）
 *
 * 优先级：最具体的 structural objectClass 先判断，
 * 覆盖 phpLDAPadmin 的 domain/org/ou/gpo/group/user/alias/referral
 */
export function getEntryIconType(objectClasses: string[]): string {
  const classes = (objectClasses || []).map((c) => c.toLowerCase())
  const has = (c: string) => classes.includes(c)

  if (has('grouppolicycontainer') || has('grouppolicyobject')) return 'gpo'
  if (has('alias')) return 'alias'
  if (has('referral')) return 'referral'
  if (has('domain') || has('domainrelatedobject')) return 'domain'
  if (has('organizationalunit')) return 'folder'
  if (has('organization')) return 'office'
  if (has('posixgroup') || has('groupofnames') || has('groupofuniquenames')) return 'group'
  if (has('inetorgperson') || has('organizationalperson') || has('person') || has('user')) return 'user'
  return 'file'
}
