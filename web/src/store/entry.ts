/**
 * 条目状态 Store
 *
 * 维护目录树当前选中条目、条目详情缓存与目录树刷新计数。
 * refreshTree 通过递增 treeKey 触发目录树组件重新加载。
 * refreshNodes 维护需局部刷新 children 的节点 DN 集合。
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'

/** 条目详情结构（DN + objectClass + 属性列表） */
interface EntryDetail {
  dn: string
  object_classes: string[]
  attributes: { name: string; values: string[] }[]
}

export const useEntryStore = defineStore('entry', () => {
  /** 当前选中条目 DN */
  const selectedDN = ref<string>('')
  /** 当前选中条目的详情缓存 */
  const selectedEntry = ref<EntryDetail | null>(null)
  /** 目录树刷新计数：递增以触发目录树组件重建（全量刷新） */
  const treeKey = ref<number>(0)
  /** 刷新后需自动展开的 DN 路径（如新建条目后展开其父节点） */
  const expandPath = ref<string[]>([])
  /** 需局部刷新 children 的节点 DN 集合（增删改条目后调用） */
  const pendingRefreshNodes = ref<string[]>([])

  /**
   * 选中条目
   *
   * @param dn 条目完整 DN
   */
  function selectEntry(dn: string) {
    selectedDN.value = dn
  }

  /**
   * 缓存条目详情
   *
   * @param entry 条目详情对象（传 null 表示清空）
   */
  function setEntry(entry: EntryDetail | null) {
    selectedEntry.value = entry
  }

  /**
   * 触发目录树全量刷新（递增 treeKey 让 el-tree 重建）
   *
   * @param expandDn 刷新后需自动展开的 DN（如新建条目的父 DN），传空则仅刷新不展开
   */
  function refreshTree(expandDn?: string) {
    treeKey.value++
    if (expandDn) {
      expandPath.value = buildDnPath(expandDn)
    } else {
      expandPath.value = []
    }
    pendingRefreshNodes.value = []
  }

  /**
   * 触发指定节点的 children 局部刷新（增量刷新，避免重建整棵树）
   *
   * 典型场景：新建条目后只刷新父节点、移动条目后只刷新原父和新父节点。
   * ServerTree 组件 watch 此数组并对每个 DN 调 el-tree 的 updateChildren。
   *
   * @param parentDN 需要刷新的父节点 DN；传空自动从 selectedDN 推父
   */
  function refreshNode(parentDN?: string) {
    const target = parentDN || parentOf(selectedDN.value)
    if (!target) return
    if (!pendingRefreshNodes.value.includes(target)) {
      pendingRefreshNodes.value.push(target)
    }
  }

  /** 清除待局部刷新队列（ServerTree 消费后调用） */
  function clearPendingRefreshNodes() {
    pendingRefreshNodes.value = []
  }

  return {
    selectedDN, selectedEntry, treeKey, expandPath, pendingRefreshNodes,
    selectEntry, setEntry, refreshTree, refreshNode, clearPendingRefreshNodes,
  }
})

/**
 * 构建 DN 的完整祖先路径（含自身），用于 lazy 目录树逐层展开
 *
 * @param dn 完整 DN，如 "uid=alice,ou=Users,dc=example,dc=com"
 * @returns 从根到自身的 DN 数组，如 ["dc=example,dc=com", "ou=Users,dc=example,dc=com", "uid=alice,ou=Users,dc=example,dc=com"]
 */
function buildDnPath(dn: string): string[] {
  const parts = dn.split(',').map((p) => p.trim())
  const path: string[] = []
  for (let i = parts.length - 1; i >= 0; i--) {
    path.push(parts.slice(i).join(','))
  }
  return path
}

/** 从子 DN 推导父 DN */
function parentOf(dn: string): string {
  const idx = dn.indexOf(',')
  if (idx < 0) return ''
  return dn.substring(idx + 1)
}
