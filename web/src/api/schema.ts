/**
 * Schema 浏览相关 API
 *
 * 对应后端 /api/v1/schema 接口，读取 LDAP 服务端的 schema 定义
 * （objectClass / attributeType），并支持查询某条目允许的属性集合。
 */
import { request } from '@/utils/request'
import type { ObjectClassDef, AttributeTypeDef } from '@/types'

/**
 * 获取所有 objectClass 定义
 *
 * @returns 服务端 schema 中的全部 objectClass 列表
 */
export function getObjectClasses() {
  return request<ObjectClassDef[]>({ url: '/schema/objectclasses', method: 'get' })
}

/**
 * 获取所有 attributeType 定义
 *
 * @returns 服务端 schema 中的全部属性类型列表
 */
export function getAttributeTypes() {
  return request<AttributeTypeDef[]>({ url: '/schema/attributetypes', method: 'get' })
}

/**
 * 获取某个 objectClass 的详细定义
 *
 * @param name objectClass 名称（如 inetOrgPerson）
 * @returns 该 objectClass 的完整定义（父类、必须/可选属性等）
 */
export function getObjectClass(name: string) {
  return request<ObjectClassDef>({ url: `/schema/objectclass/${name}`, method: 'get' })
}

/**
 * 获取某个属性类型的详细定义
 *
 * @param name 属性名（如 uid）
 * @returns 该属性的完整定义（语法、匹配规则、单/多值等）
 */
export function getAttributeType(name: string) {
  return request<AttributeTypeDef>({ url: `/schema/attribute/${name}`, method: 'get' })
}

/**
 * 获取指定 objectClass 组合允许的属性
 *
 * 创建/编辑条目时，根据条目已选的 objectClass 集合，
 * 返回 schema 推导出的必须属性（must）与可选属性（may）。
 *
 * @param objectClasses 条目的 objectClass 名称列表
 * @returns must 必须属性；may 可选属性
 */
export function getAllowedAttributes(objectClasses: string[]) {
  return request<{ must: string[]; may: string[] }>({
    url: '/schema/allowed-attributes',
    method: 'post',
    data: { object_classes: objectClasses },
  })
}
