<template>
  <div class="page">
    <div class="page-header">
      <h2>高级搜索</h2>
    </div>

    <!-- 搜索条件区：Base DN / 搜索范围 / 过滤器 -->
    <el-card class="search-card">
      <el-form :model="form" label-width="100px" inline>
        <el-form-item label="Base DN">
          <el-input v-model="form.base_dn" style="width: 320px" placeholder="dc=example,dc=com" />
        </el-form-item>
        <el-form-item label="搜索范围">
          <el-select v-model="form.scope" style="width: 140px">
            <el-option label="仅当前节点 (base)" value="base" />
            <el-option label="直接子节点 (one)" value="one" />
            <el-option label="所有子树 (sub)" value="sub" />
          </el-select>
        </el-form-item>
        <el-form-item label="过滤器">
          <el-input
            v-model="form.filter"
            style="width: 400px"
            placeholder="(objectClass=inetOrgPerson)"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="doSearch">
            <el-icon><Search /></el-icon>搜索
          </el-button>
          <el-button @click="form.filter = ''">清空</el-button>
        </el-form-item>
      </el-form>

      <!-- 快速过滤器构建器：属性 + 操作符 + 值，拼接后追加到过滤器 -->
      <div class="filter-builder">
        <span class="builder-label">快速构建：</span>
        <el-select v-model="builder.attr" placeholder="属性" style="width: 160px">
          <el-option v-for="a in commonAttrs" :key="a" :label="a" :value="a" />
        </el-select>
        <el-select v-model="builder.op" style="width: 100px">
          <el-option label="=" value="=" />
          <el-option label="包含" value="=*" />
          <el-option label=">=" value=">=" />
          <el-option label="<=" value="<=" />
        </el-select>
        <el-input v-model="builder.val" style="width: 200px" placeholder="值" />
        <el-button @click="appendFilter">添加到过滤器</el-button>
      </div>
    </el-card>

    <!-- 搜索结果区：支持勾选后批量删除 / 批量修改（对标 phpLDAPadmin mass_delete.php / mass_edit.php） -->
    <el-card class="result-card" v-loading="loading">
      <template #header>
        <div class="result-header">
          <span>搜索结果 ({{ results.length }})</span>
          <div class="batch-actions" v-if="selectedRows.length > 0">
            <el-button
              size="small"
              @click="showBatchEdit = true"
            >批量修改 ({{ selectedRows.length }})</el-button>
            <el-button
              type="danger"
              size="small"
              @click="batchDelete"
            >批量删除 ({{ selectedRows.length }})</el-button>
          </div>
        </div>
      </template>

      <el-table
        :data="results"
        border
        @selection-change="(rows: LdapEntry[]) => (selectedRows = rows)"
      >
        <el-table-column type="selection" width="48" />
        <el-table-column prop="dn" label="DN" min-width="300" show-overflow-tooltip />
        <el-table-column label="objectClass" min-width="200">
          <template #default="{ row }">
            <el-tag
              v-for="oc in row.object_classes"
              :key="oc"
              size="small"
              style="margin: 2px"
            >
              {{ oc }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click="viewEntry(row.dn)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 批量修改对话框 — 对标 phpLDAPadmin mass_edit.php -->
    <el-dialog v-model="showBatchEdit" title="批量修改属性" width="480px">
      <div class="dlg-body">
        <div class="dlg-hint">将对已选中的 {{ selectedRows.length }} 个条目执行同一属性操作</div>
        <div class="dlg-row">
          <span class="dlg-label">操作类型</span>
          <el-select v-model="batchForm.op" style="width: 100%">
            <el-option label="replace — 替换（覆盖原值）" value="replace" />
            <el-option label="add — 添加（追加新值）" value="add" />
            <el-option label="delete — 删除（移除属性）" value="delete" />
          </el-select>
        </div>
        <div class="dlg-row">
          <span class="dlg-label">属性名</span>
          <el-input v-model="batchForm.attr" placeholder="如 telephoneNumber, description" />
        </div>
        <div v-if="batchForm.op !== 'delete'" class="dlg-row">
          <span class="dlg-label">属性值（多个值用换行分隔）</span>
          <el-input v-model="batchForm.valuesText" type="textarea" :rows="3" placeholder="每行一个值" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showBatchEdit = false">取消</el-button>
        <el-button type="primary" :loading="batchWorking" @click="doBatchEdit">执行修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * 高级搜索组件 — 提供灵活的 LDAP 过滤器构建和结果浏览
 * 支持自定义 Base DN、搜索范围（base/one/sub）、LDAP Filter 构造，
 * 以及快速构建器（属性+操作符+值）和批量删除功能
 */
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { search } from '@/api/search'
import { batchDelete as batchDeleteApi, batchModify } from '@/api/entry'
import { useServerStore } from '@/store/server'
import type { LdapEntry } from '@/types'

// ==================== 路由和 Store 初始化 ====================
const route = useRoute()
const router = useRouter()
const serverStore = useServerStore()

// ==================== 响应式状态 ====================
const loading = ref(false)                     // 搜索加载状态
const results = ref<LdapEntry[]>([])           // 搜索结果列表
const selectedRows = ref<LdapEntry[]>([])      // 表格中选中的行（用于批量操作）

/** 常用 LDAP 属性列表，供快速构建器选择 */
const commonAttrs = ['cn', 'sn', 'uid', 'mail', 'ou', 'o', 'objectClass', 'uidNumber', 'gidNumber']

// ==================== 搜索表单 ====================
const form = reactive({
  base_dn: '',
  scope: 'sub',     // 搜索范围：base=仅当前, one=直接子节点, sub=整个子树
  filter: '(objectClass=*)',  // 默认匹配所有条目的过滤器
})

// ==================== 快速过滤器构建器 ====================
const builder = reactive({ attr: '', op: '=', val: '' })

// ==================== 初始化 ====================
onMounted(() => {
  // 自动填入当前服务器的 base_dn
  const server = serverStore.currentServer()
  if (server) form.base_dn = server.base_dn
  // 如果 URL 带有 ?q= 参数，自动构建过滤器并执行搜索
  const q = route.query.q as string
  if (q) {
    form.filter = `(|(cn=*${q}*)(uid=*${q}*)(mail=*${q}*))`
    doSearch()
  }
})

/**
 * 将快速构建器的条件追加到过滤器中，自动用 (&) 包裹
 * @returns 无返回值
 */
function appendFilter() {
  if (!builder.attr || !builder.val) return
  const cond = `(${builder.attr}${builder.op}${builder.val})`
  // 已有自定义过滤器时用 (&) 与新条件取交集；仍是默认过滤器 (objectClass=*) 时直接替换
  if (form.filter && form.filter !== '(objectClass=*)') {
    form.filter = `(&${form.filter}${cond})`
  } else {
    form.filter = cond
  }
}

/**
 * 执行 LDAP 搜索请求，按 base_dn + scope + filter 查询并写入结果列表
 * @returns 无返回值
 */
async function doSearch() {
  if (!form.base_dn || !form.filter) {
    ElMessage.warning('请输入 Base DN 和过滤器')
    return
  }
  loading.value = true
  try {
    results.value = await search({
      base_dn: form.base_dn,
      filter: form.filter,
      scope: form.scope as 'base' | 'one' | 'sub',
    })
  } finally {
    loading.value = false
  }
}

/**
 * 跳转到指定条目的详情页
 * @param dn 要查看的条目 DN
 * @returns 无返回值
 */
function viewEntry(dn: string) {
  router.push({ path: '/entry', query: { dn } })
}

/**
 * 批量删除选中的搜索结果条目（对标 phpLDAPadmin mass_delete.php）
 * 使用后端 batch_delete API 逐条执行并汇总结果，便于日志追踪
 */
async function batchDelete() {
  if (selectedRows.value.length === 0) return
  await ElMessageBox.confirm(
    `确定删除选中的 ${selectedRows.value.length} 个条目吗？子条目不会被自动删除。`,
    '批量删除',
    { type: 'error' },
  )
  const dns = selectedRows.value.map((r) => r.dn)
  const res = await batchDeleteApi(dns, false)
  ElMessage.success(`成功删除 ${res.success}/${res.total} 个条目（失败 ${res.failed}）`)
  doSearch()
}

// ==================== 批量修改状态 — 对标 phpLDAPadmin mass_edit.php ====================
const showBatchEdit = ref(false)
const batchWorking = ref(false)
const batchForm = reactive({
  op: 'replace' as 'add' | 'replace' | 'delete',
  attr: '',
  valuesText: '',
})

async function doBatchEdit() {
  if (!batchForm.attr) { ElMessage.warning('请填写属性名'); return }
  const dns = selectedRows.value.map((r) => r.dn)
  const values = batchForm.op === 'delete'
    ? []
    : batchForm.valuesText.split('\n').map((s) => s.trim()).filter(Boolean)
  if (batchForm.op !== 'delete' && values.length === 0) {
    ElMessage.warning('请至少输入一个属性值')
    return
  }
  batchWorking.value = true
  try {
    const res = await batchModify(dns, batchForm.op, batchForm.attr, values)
    const msg = res.failed > 0
      ? `修改完成：成功 ${res.success}/${res.total}，失败 ${res.failed}`
      : `已成功修改 ${res.total} 个条目的 ${batchForm.attr}`
    ElMessage.success(msg)
    showBatchEdit.value = false
    doSearch()
  } finally { batchWorking.value = false }
}
</script>

<style scoped lang="scss">
.page {
  padding: 24px;
  height: 100%;
  overflow: auto;
}

.page-header {
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }
}

.search-card {
  margin-bottom: 20px;
  border-radius: var(--radius-3) !important;
}

.filter-builder {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;

  .builder-label {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
}

.result-card {
  border-radius: var(--radius-3) !important;

  .result-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .batch-actions { display: flex; gap: 8px; }
}

/* 批量修改对话框样式 */
.dlg-body { display: flex; flex-direction: column; gap: 14px; }
.dlg-row { display: flex; flex-direction: column; gap: 6px; }
.dlg-label { font-size: 12px; font-weight: 500; color: var(--text-secondary); }
.dlg-hint { font-size: 12px; color: var(--text-muted); padding: 8px 12px; background: var(--surface-base); border-radius: 6px; }
</style>
