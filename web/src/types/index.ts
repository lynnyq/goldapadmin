/**
 * 全局类型定义
 *
 * 与后端 Golang 数据模型一一对应的 TypeScript 类型：
 * 统一响应结构、服务器配置、登录、条目、目录树、Schema、日志、监控、书签。
 */

/** 统一响应结构（code=0 表示业务成功） */
export interface ApiResponse<T = unknown> {
  /** 业务码：0=成功 */
  code: number
  /** 提示信息（错误时为错误说明） */
  message: string
  /** 业务数据 */
  data: T
  /** 全链路追踪 ID */
  trace_id: string
}

/** LDAP 服务器配置 */
export interface LdapServer {
  /** 服务器配置 ID */
  id: number
  /** 显示名称 */
  name: string
  /** 主机地址（不含 ldap:// 前缀与端口） */
  host: string
  /** 端口（如 389/636） */
  port: number
  /** 默认 Base DN */
  base_dn: string
  /** 管理绑定 DN */
  bind_dn: string
  /** 是否启用 StartTLS */
  use_tls: boolean
  /** 是否启用 LDAPS (SSL) */
  use_ssl: boolean
  /** 连接超时（秒） */
  timeout: number
  /** 创建时间 */
  created_at: string
  /** 更新时间 */
  updated_at: string
}

/** 登录请求 */
export interface LoginRequest {
  /** 已保存服务器配置 ID（与 host 二选一） */
  server_id?: number
  /** 手动输入的主机地址 */
  host?: string
  /** 手动输入的端口 */
  port?: number
  /** 手动输入的 Base DN */
  base_dn?: string
  /** 绑定 DN（用户账号） */
  bind_dn: string
  /** 绑定密码（仅传输，不落盘） */
  password: string
  /** 是否启用 StartTLS */
  use_tls?: boolean
  /** 是否启用 LDAPS (SSL) */
  use_ssl?: boolean
  /** 是否记住服务器配置（不记住密码） */
  remember?: boolean
}

/** 登录响应 */
export interface LoginResponse {
  /** 访问令牌（JWT） */
  token: string
  /** 登录使用的服务器配置 */
  server: LdapServer
  /** 登录用户的 Bind DN */
  user_dn: string
}

/** LDAP 条目 */
export interface LdapEntry {
  /** 条目完整 DN */
  dn: string
  /** objectClass 列表 */
  object_classes: string[]
  /** 属性列表 */
  attributes: LdapAttribute[]
  /** 是否存在子节点（用于目录树懒加载） */
  has_children?: boolean
}

/** LDAP 属性（多值） */
export interface LdapAttribute {
  /** 属性名 */
  name: string
  /** 属性值列表 */
  values: string[]
}

/** 目录树节点（与后端 TreeNode 模型对齐） */
export interface TreeNode {
  /** 条目完整 DN */
  dn: string
  /** RDN（相对 DN，作为节点显示名） */
  rdn: string
  /** objectClass 列表 */
  object_classes: string[]
  /** 是否存在子节点（控制展开箭头） */
  has_children: boolean
  /** true 表示无子节点（后端返回，el-tree lazy 模式用） */
  is_leaf?: boolean
  /** 直接子条目数（后端可能省略） */
  child_count?: number
  /** 条目类型（后端 classifyEntryType 结果） */
  entry_type?: string
  /** AD userAccountControl 禁用账户 */
  disabled?: boolean
  /** 已加载的子节点（懒加载填充） */
  children?: TreeNode[]
  /** 是否叶子节点（前端给 el-tree lazy 用） */
  leaf?: boolean
}

/** Schema attributeType 定义 */
export interface AttributeTypeDef {
  /** 属性 OID */
  oid: string
  /** 主名称 */
  name: string
  /** 别名列表 */
  names: string[]
  /** 描述 */
  description: string
  /** 语法 OID（数据类型） */
  syntax: string
  /** 是否单值属性 */
  single_value: boolean
  /** 是否必须属性（在所属 objectClass 中） */
  mandatory: boolean
}

/** Schema objectClass 定义 */
export interface ObjectClassDef {
  /** objectClass OID */
  oid: string
  /** 主名称 */
  name: string
  /** 别名列表 */
  names: string[]
  /** 描述 */
  description: string
  /** 类型：structural 结构型 / auxiliary 辅助型 / abstract 抽象型 */
  type: 'structural' | 'auxiliary' | 'abstract'
  /** 必须属性列表 */
  must: string[]
  /** 可选属性列表 */
  may: string[]
  /** 父类列表 */
  superior: string[]
}

/** 操作日志（写操作审计记录） */
export interface OperationLog {
  /** 日志 ID */
  id: number
  /** 服务器配置 ID */
  server_id: number
  /** 操作用户的 Bind DN */
  user_dn: string
  /** 操作类型（create/modify/delete 等） */
  action: string
  /** 操作目标 DN */
  target_dn: string
  /** 操作详情 */
  detail: string
  /** 执行结果 */
  status: 'success' | 'failed'
  /** 失败原因（成功时为空） */
  error_msg: string
  /** 耗时（毫秒） */
  duration_ms: number
  /** 操作时间 */
  created_at: string
}

/** 监控统计信息 */
export interface MonitorStats {
  /** LDAP 连接是否正常 */
  connected: boolean
  /** 活跃连接数 */
  active_connections: number
  /** 总操作数 */
  total_operations: number
  /** 成功操作数 */
  success_count: number
  /** 失败操作数 */
  failed_count: number
  /** 平均响应时间（毫秒） */
  avg_response_ms: number
  /** 服务运行时长（秒） */
  uptime_seconds: number
  /** 按操作类型分组的次数 */
  operations_by_type: Record<string, number>
  /** 响应时间序列（time 时间点，avg_ms 平均耗时） */
  response_times: { time: string; avg_ms: number }[]
}

/** 书签（收藏的 DN） */
export interface Bookmark {
  /** 书签 ID */
  id: number
  /** 所属服务器配置 ID */
  server_id: number
  /** 显示名称 */
  name: string
  /** 收藏的条目 DN */
  dn: string
  /** 创建时间 */
  created_at: string
}
