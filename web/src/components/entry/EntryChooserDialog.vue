<template>
  <el-dialog
    v-model="visible"
    :title="title"
    :width="width"
    :close-on-click-modal="false"
    @update:model-value="onClose"
  >
    <!-- 顶部搜索框 — 对标 phpLDAPadmin entry_chooser.php 的过滤 -->
    <div class="ec-toolbar">
      <el-input
        v-model="filter"
        :placeholder="filterPlaceholder"
        clearable
        class="ec-filter"
      >
        <template #prefix>
          <el-icon :size="14"><Search /></el-icon>
        </template>
      </el-input>
      <button class="refresh-btn" @click="refreshRoot" title="刷新根节点">
        <el-icon :size="14"><Refresh /></el-icon>
      </button>
    </div>

    <!-- 条目树 — 懒加载 + 过滤 -->
    <div class="ec-tree-wrap">
      <el-tree
        ref="treeRef"
        :data="treeData"
        :props="treeProps"
        :load="loadNode"
        lazy
        show-checkbox
        :node-key="nodeKey"
        :default-expand-all="false"
        :filter-node-method="filterNode"
        :check-strictly="true"
        empty-text="（无子节点）"
        highlight-current
        @node-click="handleNodeClick"
      >
        <template #default="{ data }">
          <span class="ec-node" :class="getNodeClass(data)">
            <el-icon :size="14" class="ec-ico"><component :is="getIcon(data)" /></el-icon>
            <span class="ec-label">{{ data.label }}</span>
            <code v-if="data.isLeaf" class="ec-dn font-mono">{{ data.dn }}</code>
          </span>
        </template>
      </el-tree>
    </div>

    <!-- 底部已选 DN 展示 -->
    <div class="ec-footer">
      <div class="selected-area">
        <div v-if="selectedDN" class="selected-chip">
          <el-icon :size="13"><LocationInformation /></el-icon>
          <code class="font-mono">{{ selectedDN }}</code>
        </div>
        <div v-else class="selected-empty">尚未选择条目，请在树中点击或勾选</div>
      </div>
    </div>

    <template #footer>
      <el-button @click="onClose">取消</el-button>
      <el-button type="primary" :disabled="!selectedDN" @click="confirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
/**
 * EntryChooserDialog — 弹窗式条目选择器（对标 phpLDAPadmin entry_chooser.php）
 *
 * 用途：
 *   - move dialog 选择新父级 DN
 *   - copy dialog 选择目标位置
 *   - group member picker 选择要添加的用户
 *
 * 特性：
 *   - el-tree 懒加载（调用 getChildren 接口）
 *   - 前端过滤输入框
 *   - 点击/勾选皆可选，单返回值
 *
 * Props:
 *   modelValue  控制显示（v-model）
 *   title        弹窗标题
 *   baseDN       根节点 DN（默认使用 serverStore.baseDN）
 *   filterPlaceholder  搜索框占位文本
 *
 * Emits:
 *   update:modelValue  关闭
 *   confirm            返回所选 DN 字符串
 */
import { ref, watch, nextTick } from 'vue'
import { Search, Refresh, LocationInformation, Folder, User, UserFilled, OfficeBuilding, Grid } from '@element-plus/icons-vue'
import { getChildren } from '@/api/entry'
import { useServerStore } from '@/store/server'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  width?: string
  baseDN?: string
  filterPlaceholder?: string
}>(), {
  title: '选择条目',
  width: '560px',
  filterPlaceholder: '按 cn/uid/name 过滤（前端实时）',
})

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  'confirm': [dn: string]
}>()

const serverStore = useServerStore()

const visible = ref(false)
watch(() => props.modelValue, v => { visible.value = v; if (v) init() })

function onClose() { emit('update:modelValue', false) }

const filter = ref('')
const selectedDN = ref('')
const treeRef = ref()

const treeData = ref<any[]>([])
const treeProps = {
  label: 'label',
  children: 'children',
  isLeaf: 'isLeaf',
}
const nodeKey = 'dn'

/** 根节点 — 用当前 server 的 baseDN */
async function init() {
  selectedDN.value = ''
  filter.value = ''
  const serverBaseDN = serverStore.currentServer()?.base_dn || ''
  treeData.value = [{
    dn: props.baseDN || serverBaseDN,
    label: props.baseDN || serverBaseDN || '（未设置 baseDN）',
    isRoot: true,
  }]
  await nextTick()
  treeRef.value?.store.setLoaded(treeData.value[0], false)
  treeRef.value?.loadNode(treeData.value[0])
}

/** 懒加载子节点（对标 ServerTree 的 getChildren 调用） */
async function loadNode(node: any, resolve: (children: any[]) => void) {
  try {
    const children = await getChildren(node.data.dn)
    resolve(children.map((c: any) => ({
      dn: c.dn,
      label: c.label || extractRDN(c.dn),
      isLeaf: c.is_leaf,
      objectClass: c.object_class || [],
    })))
  } catch {
    resolve([])
  }
}

function extractRDN(dn: string): string {
  const idx = dn.indexOf(',')
  return idx === -1 ? dn : dn.slice(0, idx)
}

/** 前端过滤（对标 PLA 的树内搜索） */
watch(filter, (q) => { treeRef.value?.filter(q) })

function filterNode(value: string, data: any) {
  if (!value) return true
  const v = value.toLowerCase()
  return (data.label || '').toLowerCase().includes(v)
    || (data.dn || '').toLowerCase().includes(v)
}

function handleNodeClick(data: any) {
  selectedDN.value = data.dn
  treeRef.value?.setCheckedKeys([data.dn], false)
}

function refreshRoot() { treeData.value = []; init() }

function confirm() {
  if (selectedDN.value) {
    emit('confirm', selectedDN.value)
    onClose()
  }
}

/** 根据 objectClass 返回图标（对标 ServerTree 的 icon 逻辑） */
function getIcon(data: any) {
  const oc: string[] = data.objectClass || []
  const ocStr = oc.join(' ').toLowerCase()
  if (ocStr.includes('groupofnames') || ocStr.includes('groupofuniquenames') || ocStr.includes('posixgroup'))
    return UserFilled
  if (ocStr.includes('organizationalunit')) return OfficeBuilding
  if (ocStr.includes('domain') || ocStr.includes('dcobject')) return Grid
  if (ocStr.includes('person') || ocStr.includes('inetorgperson') || ocStr.includes('uidobject')) return User
  return Folder
}

function getNodeClass(data: any) {
  const oc: string[] = data.objectClass || []
  const s = oc.join(',').toLowerCase()
  return {
    'ec-is-ou': s.includes('organizationalunit'),
    'ec-is-group': s.includes('posixgroup') || s.includes('groupofnames') || s.includes('groupofuniquenames'),
    'ec-is-user': s.includes('person'),
  }
}
</script>

<style scoped lang="scss">
.ec-toolbar {
  display: flex; gap: 8px; margin-bottom: 12px;
  .ec-filter { flex: 1; }
  .refresh-btn {
    width: 32px; height: 32px; display: flex; align-items: center; justify-content: center;
    background: var(--surface-base); border: 1px solid var(--border-subtle); border-radius: 6px;
    cursor: pointer; color: var(--text-muted);
    &:hover { color: var(--accent-primary); border-color: var(--accent-primary); }
  }
}

.ec-tree-wrap {
  height: 380px; overflow-y: auto;
  background: var(--surface-base); border: 1px solid var(--border-subtle); border-radius: 8px;
  padding: 8px;
}

.ec-node {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 2px 6px; border-radius: 4px; width: 100%;

  .ec-ico { color: var(--text-muted); flex-shrink: 0; }
  .ec-label { font-size: 13px; font-weight: 500; color: var(--text-primary); }
  .ec-dn { margin-left: auto; font-size: 11px; color: var(--text-muted); max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  &.ec-is-user { .ec-ico { color: var(--accent-primary); } }
  &.ec-is-group { .ec-ico { color: var(--purple-500, #8b5cf6); } }
  &.ec-is-ou { .ec-ico { color: var(--amber-500, #f59e0b); } }
}

.ec-footer {
  margin-top: 12px; padding: 10px 12px;
  background: var(--surface-base); border: 1px solid var(--border-subtle); border-radius: 6px;
  min-height: 36px;

  .selected-chip {
    display: inline-flex; align-items: center; gap: 6px;
    font-size: 12px; color: var(--text-primary);
    code { font-size: 12px; }
  }
  .selected-empty { font-size: 12px; color: var(--text-muted); }
}
</style>
