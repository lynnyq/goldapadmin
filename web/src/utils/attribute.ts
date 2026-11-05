/**
 * LDAP 属性元数据工具
 *
 * 提供属性友好名称映射，对标 phpLDAPadmin 的字段标签展示，
 * 避免用户直接面对 uidNumber/gidNumber 等原始属性名。
 */

/** phpLDAPadmin 风格的属性友好名称映射（key 全小写） */
const attrLabels: Record<string, string> = {
  cn: '全称 (Common Name)',
  sn: '姓氏 (Surname)',
  uid: '用户名',
  uidnumber: '用户 ID',
  gidnumber: '组编号 (GID Number)',
  homedirectory: '主目录 (Home Directory)',
  loginshell: '登录 Shell',
  gecos: '用户信息 (GECOS)',
  userpassword: '用户密码',
  memberuid: '成员 UID',
  member: '成员 DN',
  uniquemember: '唯一成员 DN',
  description: '描述',
  mail: '邮箱',
  telephonenumber: '电话',
  ou: '组织单位',
  o: '组织',
  dc: '域名组件',
  seealso: '参见',
  l: '城市 (Locality)',
  st: '省/州 (State)',
  postalcode: '邮政编码',
  c: '国家代码',
  givenname: '名 (Given Name)',
  displayname: '显示名',
  title: '职位',
  departmentnumber: '部门编号',
  employeenumber: '员工编号',
  employeetype: '员工类型',
  shadowlastchange: '密码最后修改日期',
  shadowmin: '密码最小使用天数',
  shadowmax: '密码最大使用天数',
  shadowwarning: '密码过期警告天数',
  shadowinactive: '密码过期后宽限天数',
  shadowexpire: '账号过期日期',
  shadowflag: 'shadow 标志位',
  labeleduri: '主页 URL',
  mobile: '手机号',
  homephone: '家庭电话',
  pager: '寻呼机',
  facsimiletelephonenumber: '传真',
  roomnumber: '房间号',
  carlicense: '车牌号',
  businesscategory: '业务类别',
  destinationindicator: '目的地指示符',
  dnqualifier: 'DN 限定符',
  preferredlanguage: '首选语言',
  initials: '姓名首字母',
  jpegphoto: '照片',
}

/**
 * 获取属性的友好显示名称
 *
 * @param name LDAP 属性名（大小写不敏感）
 * @returns 友好名称；未映射时返回原始属性名
 */
export function attrLabel(name: string): string {
  if (!name) return name
  return attrLabels[name.toLowerCase()] || name
}

/**
 * 判断是否为密码类属性（需脱敏展示 + 哈希 helper）
 */
export function isPasswordAttr(name: string): boolean {
  const n = name.toLowerCase()
  return n === 'userpassword' || n === 'unicodepwd' || n === 'lmpassword' || n === 'ntpassword'
}

/**
 * 判断是否为成员类属性（memberUid/member/uniqueMember），
 * 可展示成员选择器与逐个移除功能
 */
export function isMemberAttr(name: string): boolean {
  const n = name.toLowerCase()
  return n === 'memberuid' || n === 'member' || n === 'uniquemember'
}

/**
 * 判断是否为数值型属性（uidNumber/gidNumber/shadow*），
 * 需整数校验
 */
const numberAttrs = new Set([
  'uidnumber', 'gidnumber', 'shadowlastchange', 'shadowmin', 'shadowmax',
  'shadowwarning', 'shadowinactive', 'shadowexpire', 'shadowflag',
])
export function isNumberAttr(name: string): boolean {
  return numberAttrs.has(name.toLowerCase())
}
