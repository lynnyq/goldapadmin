<template>
  <div class="server-tree" :class="{ dragging: isDragging }">
    <div class="tree-toolbar">
      <div class="filter-box">
        <el-icon :size="13" class="filter-ic"><Search /></el-icon>
        <input v-model="filterText" class="filter-in" placeholder="过滤 RDN..." />
        <button v-if="filterText" class="filter-clear" @click="filterText = ''">
          <el-icon :size="12"><Close /></el-icon>
        </button>
      </div>
      <div class="toolbar-btns">
        <el-tooltip content="展开全部" placement="top">
          <button class="tb-btn" @click="expandAllNodes"><el-icon :size="14"><Plus /></el-icon></button>
        </el-tooltip>
        <el-tooltip content="收起全部" placement="top">
          <button class="tb-btn" @click="collapseAllNodes"><el-icon :size="14"><Minus /></el-icon></button>
        </el-tooltip>
        <el-tooltip content="刷新" placement="top">
          <button class="tb-btn" @click="entryStore.refreshTree()" :class="{ spin: refreshAnim }">
            <el-icon :size="14"><Refresh /></el-icon>
          </button>
        </el-tooltip>
      </div>
    </div>

    <div class="tree-wrap" @click="closeContextMenu" @contextmenu.prevent>
      <el-tree
        v-if="baseDn"
        ref="treeRef"
        :key="entryStore.treeKey"
        :data="treeData"
        :props="treeProps"
        node-key="dn"
        lazy
        :load="loadNode"
        :filter-node-method="filterNode"
        :expand-on-click-node="false"
        :default-expanded-keys="defaultExpandedKeys"
        :allow-drag="allowDrag"
        :allow-drop="allowDrop"
        highlight-current
        class="dir-tree"
        :class="{ 'has-error': loadError }"
        @node-click="handleNodeClick"
        @node-expand="handleNodeExpand"
        @node-collapse="handleNodeCollapse"
        @node-drag-start="handleDragStart"
        @node-drop="handleDrop"
      >
        <template #default="{ node, data }">
          <div
            class="t-node"
            :class="['ti-' + getType(data), { disabled: data.disabled, 'is-drop-target': isDropTarget(node) }]"
            :draggable="data.entry_type !== 'domain'"
            @contextmenu.stop.prevent="showContextMenu($event, data, node)"
            @dblclick="handleNodeDoubleClick(node)"
          >
            <div class="t-icon">
              <span v-if="data.disabled" class="disabled-badge">🚫</span>
              <component :is="getIcon(data)" />
            </div>
            <span class="t-label">{{ node.label }}</span>
            <span v-if="data.entry_type && data.entry_type !== 'unknown' && data.entry_type !== 'folder'" class="t-type">
              {{ getTypeLabel(data.entry_type) }}
            </span>
            <span v-if="data.child_count" class="t-count">({{ data.child_count }})</span>
          </div>
        </template>
        <template #empty>
          <div v-if="loadError" class="tree-empty">
            <el-icon :size="24" class="err-ic"><WarningFilled /></el-icon>
            <p>{{ loadError }}</p>
            <el-button size="small" @click="retryLoad">重试</el-button>
          </div>
          <div v-else-if="loadingInitial" class="tree-empty">
            <el-icon :size="20" class="spin-ic"><Loading /></el-icon>
            <p>加载目录树...</p>
          </div>
          <div v-else class="tree-empty">
            <el-icon :size="24" class="empty-ic"><Folder /></el-icon>
            <p>暂无子条目</p>
          </div>
        </template>
      </el-tree>
    </div>

    <!-- 右键上下文菜单 -->
    <teleport to="body">
      <div
        v-if="ctxMenu.visible"
        class="ctx-menu"
        :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
        @click.stop
      >
        <div class="ctx-menu-header">
          <span class="ctx-title">{{ getRDNValue(ctxMenu.data?.dn || '') }}</span>
          <el-tooltip content="复制完整 DN" placement="right">
            <button class="ctx-copy" @click="copyDN(ctxMenu.data?.dn)">
              <el-icon :size="12"><DocumentCopy /></el-icon>
            </button>
          </el-tooltip>
        </div>
        <div class="ctx-divider"></div>
        <button class="ctx-item" @click="ctxAction('view')">
          <el-icon><View /></el-icon><span>查看详情</span>
        </button>
        <button class="ctx-item" v-if="ctxMenu.data?.has_children" @click="ctxAction('expand')">
          <el-icon><Expand /></el-icon><span>展开子节点</span>
        </button>
        <div class="ctx-divider"></div>
        <button class="ctx-item" @click="ctxAction('create')">
          <el-icon><Plus /></el-icon><span>在此新建</span>
        </button>
        <button class="ctx-item" @click="ctxAction('copy')">
          <el-icon><CopyDocument /></el-icon><span>复制</span>
        </button>
        <button class="ctx-item" @click="ctxAction('move')">
          <el-icon><Aim /></el-icon><span>移动</span>
        </button>
        <button class="ctx-item" @click="ctxAction('rename')">
          <el-icon><Edit /></el-icon><span>重命名</span>
        </button>
        <button class="ctx-item danger" @click="ctxAction('delete')">
          <el-icon><Delete /></el-icon><span>删除</span>
        </button>
        <div class="ctx-divider"></div>
        <button class="ctx-item" @click="ctxAction('bookmark')">
          <el-icon><Star /></el-icon><span>收藏书签</span>
        </button>
        <button class="ctx-item" @click="ctxAction('export')">
          <el-icon><Download /></el-icon><span>导出 LDIF</span>
        </button>
        <button class="ctx-item" @click="ctxAction('tree')">
          <el-icon><Connection /></el-icon><span>加载整棵子树</span>
        </button>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Close, Refresh, User, UserFilled, Folder,
  OfficeBuilding, Connection, Document, WarningFilled, Loading,
  Plus, Minus, DocumentCopy, View, Expand, CopyDocument, Aim, Edit,
  Delete, Star, Download, Box, Stamp, Flag,
} from '@element-plus/icons-vue'
import { getChildren, copyEntry, renameEntry, deleteEntry } from '@/api/entry'
import { exportLdif } from '@/api/ldif'
import { useEntryStore } from '@/store/entry'
import { useServerStore } from '@/store/server'
import { getRDNValue, getEntryIconType } from '@/utils/dn'
import { addBookmark } from '@/api/bookmark'
import type { TreeNode } from '@/types'

const LS_EXPANDED = 'goldapadmin.tree.expanded'
const LS_VERSION = 1

const router = useRouter()
const entryStore = useEntryStore()
const serverStore = useServerStore()

const treeRef = ref()
const filterText = ref('')
const treeData = ref<TreeNode[]>([])
const refreshAnim = ref(false)
const loadingInitial = ref(false)
const loadError = ref<string>('')

// 拖拽状态
const isDragging = ref(false)
const dropTarget = ref<string>('')

// 折叠状态持久化
const savedExpanded = ref<string[]>([])

// 右键菜单
interface CtxMenu {
  visible: boolean; x: number; y: number
  data: TreeNode | null
  node: any | null
}
const ctxMenu = ref<CtxMenu>({ visible: false, x: 0, y: 0, data: null, node: null })

const treeProps = { label: 'rdn', children: 'children', isLeaf: 'leaf' }
const baseDn = computed(() => serverStore.currentServer()?.base_dn || '')

// 加载持久化折叠状态
function loadExpandedKeys(): string[] {
  try {
    const raw = localStorage.getItem(LS_EXPANDED)
    if (!raw) return []
    const obj = JSON.parse(raw)
    if (obj.v !== LS_VERSION) return []
    const serverId = serverStore.currentServer()?.id
    return obj[`s${serverId}`] || []
  } catch { return [] }
}
function saveExpandedKeys(keys: string[]) {
  try {
    const raw = localStorage.getItem(LS_EXPANDED)
    const obj = raw ? JSON.parse(raw) : { v: LS_VERSION }
    const serverId = serverStore.currentServer()?.id
    obj[`s${serverId}`] = keys
    localStorage.setItem(LS_EXPANDED, JSON.stringify(obj))
  } catch { /* quota */ }
}

const defaultExpandedKeys = computed(() => {
  const keys = new Set<string>()
  if (baseDn.value) keys.add(baseDn.value)
  savedExpanded.value.forEach(k => keys.add(k))
  return Array.from(keys)
})

// ==================== 监听器 ====================
watch(filterText, v => treeRef.value?.filter(v))
watch(() => entryStore.treeKey, () => {
  refreshAnim.value = true
  setTimeout(() => refreshAnim.value = false, 500)
})
watch(() => entryStore.expandPath, (path) => {
  if (path.length > 0) nextTick(() => expandPathNodes([...path]))
}, { deep: true })
watch(baseDn, () => {
  saveExpandedKeys([])
  savedExpanded.value = loadExpandedKeys()
})

// 局部刷新：对 pendingRefreshNodes 中的每个 DN，用 el-tree.updateChildren 刷新其子节点
watch(() => entryStore.pendingRefreshNodes.length, async (len) => {
  if (len === 0) return
  const dnsToRefresh = [...entryStore.pendingRefreshNodes]
  entryStore.clearPendingRefreshNodes()
  for (const dn of dnsToRefresh) {
    await refreshChildrenAt(dn)
  }
})

// ==================== 核心：懒加载 ====================
async function loadNode(node: any, resolve: (d: TreeNode[]) => void) {
  if (node.level === 0) {
    let srv = serverStore.currentServer()
    if (!srv) {
      try { await serverStore.loadServers() } catch { /* ignore */ }
      srv = serverStore.currentServer()
    }
    if (!srv?.base_dn) return resolve([])
    const root: TreeNode = {
      dn: srv.base_dn, rdn: getRDNValue(srv.base_dn),
      object_classes: [], has_children: true, leaf: false, entry_type: 'domain',
    }
    treeData.value = [root]
    return resolve([root])
  }

  const dn = node.data?.dn
  if (!dn) return resolve([])
  loadingInitial.value = true
  loadError.value = ''
  try {
    const children = await getChildren(dn)
    // 后端已同时返回 is_leaf 和 has_children，is_leaf 直接给 el-tree 用
    resolve(children)
  } catch (e: any) {
    loadError.value = e?.message || '加载失败'
    ElMessage.error(loadError.value)
    resolve([])
  } finally {
    loadingInitial.value = false
  }
}

function retryLoad() {
  loadError.value = ''
  entryStore.refreshTree()
}

/**
 * 局部刷新指定 DN 节点的 children
 * 找到节点 → 重新拉 getChildren → 用 el-tree 的 store.updateChildren 更新
 * 懒加载节点（lazy=true）需要手动设置 loaded=true 让 el-tree 不重新调 load
 */
async function refreshChildrenAt(dn: string) {
  const tree = treeRef.value
  if (!tree) return
  const node: any = tree.store.getNode(dn)
  if (!node) return

  try {
    const children = await getChildren(dn)
    // 给每个 child 标记 leaf 让 lazy 模式正确渲染展开箭头
    const enriched = children.map((c) => ({ ...c, leaf: c.is_leaf ?? !c.has_children }))
    // 设置 store 层数据 + 标记已加载
    node.childNodes = []
    const updateFn = (tree.store as any).updateChildren?.bind(tree.store)
    if (updateFn) {
      updateFn(node.data, enriched)
    } else {
      // 兜底：直接替换 childNodes + 置 loaded=true
      ;(node as any).childNodes = enriched as any
    }
    ;(node as any).loaded = true
  } catch (e: any) {
    ElMessage.warning(`刷新 ${dn} 失败: ${e?.message || e}`)
  }
}

async function expandPathNodes(path: string[]) {
  const tree = treeRef.value
  if (!tree) return
  for (const dn of path) {
    let node: any
    for (let i = 0; i < 30 && !(node = tree.store.getNode(dn)); i++) {
      await new Promise(r => setTimeout(r, 100))
    }
    if (!node) continue
    if (!node.expanded) node.expand()
    for (let i = 0; i < 30 && !node.loaded; i++) {
      await new Promise(r => setTimeout(r, 100))
    }
  }
}

// ==================== 展开/收起事件（持久化） ====================
function handleNodeExpand(data: TreeNode) {
  savedExpanded.value = Array.from(new Set([...savedExpanded.value, data.dn]))
  saveExpandedKeys(savedExpanded.value)
}
function handleNodeCollapse(data: TreeNode) {
  savedExpanded.value = savedExpanded.value.filter(d => d !== data.dn)
  saveExpandedKeys(savedExpanded.value)
}

// ==================== 展开/收起全部 ====================
function expandAllNodes() { treeRef.value?.store?.setAllExpandable(true, true) }
function collapseAllNodes() { treeRef.value?.store?.setAllExpandable(true, false) }

// ==================== 拖拽移动 ====================
function allowDrag(draggingNode: any) {
  const type = draggingNode.data?.entry_type
  if (type === 'domain') return false
  return true
}
function allowDrop(_draggingNode: any, dropNode: any, type: string) {
  if (type !== 'inner') return false
  if (dropNode.data?.entry_type === 'domain') return false
  if (dropNode.data?.has_children === false) return false
  return true
}
function handleDragStart(srcNode: any) {
  void srcNode // allowDrag 已过滤 domain 根节点
  isDragging.value = true
}
function handleDrop(_draggingNode: any, dropNode: any, dropType: string) {
  isDragging.value = false
  if (dropType !== 'inner') return
  const srcDN = _draggingNode.data.dn
  const newParent = dropNode.data.dn
  if (srcDN === newParent) return
  // 不能拖到自己的子孙里
  if (newParent.includes(srcDN)) {
    ElMessage.warning('不能拖到自己的子目录里')
    return
  }
  ElMessageBox.confirm(
    `确定将\n  ${srcDN}\n移动到\n  ${newParent}\n吗？`,
    '移动条目', { type: 'info', confirmButtonText: '确定', cancelButtonText: '取消' }
  ).then(async () => {
    try {
      // 动态 import 避免循环依赖
      const { moveEntry } = await import('@/api/entry')
      await moveEntry(srcDN, newParent)
      ElMessage.success('移动成功')
      entryStore.refreshTree(newParent)
    } catch { /* cancel / error */ }
  }).catch(() => {})
}
function isDropTarget(node: any): boolean {
  return dropTarget.value === node?.data?.dn
}

// ==================== 右键菜单 ====================
function showContextMenu(e: MouseEvent, data: TreeNode, node: any) {
  // 只允许 domain 外的节点触发（根节点也允许）
  closeContextMenu()
  ctxMenu.value = {
    visible: true, x: e.clientX, y: e.clientY,
    data, node,
  }
  document.addEventListener('click', closeContextMenu, { once: true })
  document.addEventListener('contextmenu', closeContextMenu, { once: true })
}
function closeContextMenu() {
  ctxMenu.value.visible = false
  document.removeEventListener('click', closeContextMenu)
  document.removeEventListener('contextmenu', closeContextMenu)
}

async function ctxAction(action: string) {
  const data = ctxMenu.value.data
  if (!data) return
  closeContextMenu()
  entryStore.selectEntry(data.dn)

  switch (action) {
    case 'view':
      router.push('/entry'); break
    case 'expand':
      await expandPathNodes([data.dn]); break
    case 'create':
      ElMessage.info('打开新建弹窗，父 DN: ' + data.dn);
      entryStore.selectEntry(data.dn); router.push('/entry');
      break
    case 'copy':
      {
        try {
          const { value } = await ElMessageBox.prompt('输入目标父 DN', '复制到', {
            inputPlaceholder: `如 ou=NewParent,${data.dn.split(',').slice(1).join(',')}`,
          })
          if (value) {
            await copyEntry(data.dn, value, false)
            ElMessage.success('已复制')
            entryStore.refreshTree()
          }
        } catch { /* cancel */ }
      }
      break
    case 'move':
      {
        try {
          const { value } = await ElMessageBox.prompt(
            `将 ${data.dn} 移动到哪个父 DN 下？\n⚠ 注意：移动将修改 DN，ACL/引用可能受影响`,
            '移动到', {
              inputPlaceholder: `如 ou=NewParent,${data.dn.split(',').slice(1).join(',')}`,
              inputValue: data.dn.split(',').slice(1).join(','),
            }
          )
          if (value && value !== data.dn.split(',').slice(1).join(',')) {
            const { moveEntry } = await import('@/api/entry')
            await moveEntry(data.dn, value)
            ElMessage.success('已移动')
            entryStore.refreshTree()
          }
        } catch { /* cancel */ }
      }
      break
    case 'rename':
      {
        try {
          const rdn = data.rdn.split('=')[1] || data.rdn
          const { value } = await ElMessageBox.prompt('输入新 RDN 值', '重命名', { inputValue: rdn })
          if (value && value !== rdn) {
            await renameEntry(data.dn, value)
            ElMessage.success('已重命名')
            entryStore.refreshTree()
          }
        } catch { /* cancel */ }
      }
      break
    case 'delete':
      {
        try {
          await ElMessageBox.confirm(`确定删除 ${data.dn}？此操作不可恢复！`, '危险操作', { type: 'error' })
          let recursive = false
          if (data.has_children) {
            recursive = (await ElMessageBox.confirm(
              `该节点有子条目（has_children=true），是否级联删除所有子条目？`,
              '级联删除确认', {
                type: 'warning',
                confirmButtonText: '级联删除',
                cancelButtonText: '取消',
              }).then(() => true).catch(() => false))
          }
          await deleteEntry(data.dn, recursive)
          ElMessage.success('已删除')
          entryStore.refreshTree()
        } catch { /* cancel */ }
      }
      break
    case 'bookmark':
      try {
        await addBookmark({ name: data.rdn, dn: data.dn })
        ElMessage.success('已添加书签')
      } catch {}
      break
    case 'export':
      {
        try {
          const filename = await exportLdif(data.dn, true)
          ElMessage.success(`已下载 ${filename}`)
        } catch (e: any) {
          ElMessage.error(`导出失败: ${e?.message || e}`)
        }
      }
      break
    case 'tree':
      ElMessage.info('整棵子树加载已在后续实现，当前使用懒加载'); break
  }
}

function copyDN(dn?: string) {
  if (!dn) return
  navigator.clipboard?.writeText(dn).then(() => ElMessage.success('已复制 DN'))
  closeContextMenu()
}

function handleNodeClick(data: TreeNode) {
  entryStore.selectEntry(data.dn)
  router.push('/entry')
}
function handleNodeDoubleClick(node: any) {
  if (!node.expanded) node.expand()
}

function filterNode(v: string, data: any): boolean {
  if (!v) return true
  return (data?.rdn || '').toString().toLowerCase().includes(v.toLowerCase())
}

// ==================== 图标 ====================
function getIcon(data: TreeNode) {
  switch (data.entry_type || getEntryIconType(data.object_classes)) {
    case 'user':      return UserFilled
    case 'group':     return User
    case 'folder':    return Folder
    case 'office':    return OfficeBuilding
    case 'domain':    return Connection
    case 'gpo':       return Box
    case 'alias':     return Stamp
    case 'referral':  return Flag
    default:          return Document
  }
}
function getType(data: TreeNode) {
  return data.entry_type || getEntryIconType(data.object_classes)
}
function getTypeLabel(t: string): string {
  const map: Record<string, string> = {
    user: 'User', group: 'Group', folder: 'OU', office: 'Org',
    domain: 'Domain', gpo: 'GPO', alias: 'Alias', referral: 'Ref',
  }
  return map[t] || t
}

// ==================== 生命周期 ====================
onMounted(() => { savedExpanded.value = loadExpandedKeys() })
onUnmounted(() => closeContextMenu())
</script>

<style scoped lang="scss">
.server-tree {
  flex: 1; display: flex; flex-direction: column; overflow: hidden;
  position: relative;
  &.dragging .dir-tree { background: var(--surface-hover, rgba(0,0,0,0.02)); }
}
.tree-toolbar {
  display: flex; align-items: center; gap: 6px; padding: 10px;
  border-bottom: 1px solid var(--border-subtle); flex-shrink: 0;
}
.filter-box {
  flex: 1; display: flex; align-items: center; gap: 6px;
  padding: 5px 10px; background: var(--surface-panel);
  border: 1px solid var(--border-subtle); border-radius: var(--radius-2);
  transition: all 0.15s ease;
  &:focus-within { border-color: var(--accent-primary); box-shadow: var(--focus-ring); }
  .filter-ic { color: var(--text-muted); flex-shrink: 0; }
}
.filter-in {
  flex: 1; background: none; border: none; outline: none;
  font-family: var(--font-ui); font-size: var(--fs-body); color: var(--text-primary);
  &::placeholder { color: var(--text-muted); }
}
.filter-clear {
  background: none; border: none; color: var(--text-muted); cursor: pointer;
  padding: 2px; border-radius: 4px;
  &:hover { color: var(--text-primary); background: var(--surface-hover); }
}
.toolbar-btns { display: flex; gap: 4px; }
.tb-btn {
  width: 28px; height: 28px; display: flex; align-items: center; justify-content: center;
  background: transparent; border: 1px solid var(--border-subtle);
  border-radius: var(--radius-1); color: var(--text-muted); cursor: pointer;
  transition: all 0.15s ease;
  &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
  .spin { animation: spin 0.5s linear; }
}
@keyframes spin { to { transform: rotate(360deg); } }

.tree-wrap { flex: 1; overflow: auto; position: relative; }

// ============ Tree 样式 ============
:deep(.dir-tree) {
  background: transparent !important; padding: 6px 6px 12px;

  .el-tree-node__content {
    height: 28px; margin: 1px 0; border-radius: var(--radius-1);
    padding-right: 4px; transition: background 0.12s ease;
    cursor: default;
  }
  .el-tree-node.is-current > .el-tree-node__content {
    background: var(--accent-primary-soft) !important; color: var(--accent-primary);
  }
  .el-tree-node.is-current > .el-tree-node__content:hover {
    background: color-mix(in oklab, var(--accent-primary-soft) 80%, white 20%) !important;
  }
  .el-tree-node__expand-icon.is-leaf { visibility: hidden; }
}

.t-node {
  display: flex; align-items: center; gap: 6px; width: 100%; padding: 0 2px;
  &.disabled .t-label { text-decoration: line-through; opacity: 0.6; }
  &.is-drop-target { outline: 2px dashed var(--accent-primary); outline-offset: -2px; }
}

.t-icon {
  width: 18px; height: 18px; border-radius: var(--radius-1);
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  font-size: 11px;
}
.disabled-badge { font-size: 8px; position: absolute; margin-left: -18px; margin-top: -14px; }

&.ti-user    .t-icon { background: color-mix(in oklab, var(--accent-primary) 12%, white);   color: var(--accent-primary); }
&.ti-group   .t-icon { background: color-mix(in oklab, var(--amber-600) 14%, white);       color: var(--amber-600); }
&.ti-folder  .t-icon { background: color-mix(in oklab, var(--red-500) 14%, white);         color: var(--red-600); }
&.ti-office  .t-icon { background: color-mix(in oklab, var(--green-600) 14%, white);       color: var(--green-600); }
&.ti-domain  .t-icon { background: color-mix(in oklab, var(--purple-500) 14%, white);      color: var(--purple-500); }
&.ti-gpo     .t-icon { background: color-mix(in oklab, var(--cyan-500) 14%, white);        color: var(--cyan-600); }
&.ti-alias   .t-icon { background: color-mix(in oklab, var(--gray-500) 14%, white);        color: var(--gray-700); }
&.ti-referral.t-icon { background: color-mix(in oklab, var(--orange-500) 14%, white); color: var(--orange-600); }

.t-label {
  font-size: 12.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1; min-width: 0;
}
.t-type {
  font-size: 9px; padding: 0 4px; border-radius: 2px;
  background: var(--surface-hover); color: var(--text-muted); flex-shrink: 0;
}
.t-count {
  font-size: 10px; color: var(--text-muted); flex-shrink: 0;
  font-family: var(--font-mono);
}

// ============ Empty ============
.tree-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 30px 20px; gap: 8px; color: var(--text-muted);
  .spin-ic { animation: spin 1s linear infinite; }
  .err-ic { color: var(--warning, #f59e0b); }
  .empty-ic { color: var(--text-muted); opacity: 0.5; }
  p { font-size: 12px; margin: 0; }
}

// ============ 右键菜单 ============
.ctx-menu {
  position: fixed; z-index: 9999; min-width: 220px;
  background: var(--surface-panel, white);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  box-shadow: 0 8px 24px rgba(0,0,0,0.15);
  padding: 6px; font-size: 12.5px;
  animation: ctx-in 0.12s ease;
}
@keyframes ctx-in { from { opacity: 0; transform: scale(0.96); } to { opacity: 1; transform: scale(1); } }

.ctx-menu-header {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 8px 6px;
  .ctx-title { flex: 1; font-weight: 500; color: var(--text-primary);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px; }
  .ctx-copy {
    background: none; border: none; color: var(--text-muted); cursor: pointer;
    padding: 2px; border-radius: 3px;
    &:hover { color: var(--accent-primary); background: var(--surface-hover); }
  }
}
.ctx-divider { height: 1px; background: var(--border-subtle); margin: 4px 0; }
.ctx-item {
  display: flex; align-items: center; gap: 8px; width: 100%;
  padding: 6px 10px; background: none; border: none; border-radius: var(--radius-1);
  color: var(--text-primary); cursor: pointer; text-align: left; font-size: inherit;
  transition: background 0.1s;
  &:hover { background: var(--surface-hover); }
  &.danger { color: var(--danger, #ef4444);
    &:hover { background: color-mix(in oklab, var(--danger, #ef4444) 10%, white); }
  }
  .el-icon { font-size: 14px; }
}
</style>
