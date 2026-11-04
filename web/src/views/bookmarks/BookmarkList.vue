<template>
  <div class="page">
    <div class="page-header">
      <h2>书签管理</h2>
    </div>

    <el-table :data="bookmarks" border v-loading="loading">
      <el-table-column prop="name" label="名称" width="200" />
      <el-table-column prop="dn" label="DN" min-width="400" show-overflow-tooltip />
      <el-table-column prop="created_at" label="创建时间" width="180" />
      <el-table-column label="操作" width="160" align="center">
        <template #default="{ row }">
          <el-button link type="primary" @click="jumpTo(row as Bookmark)">跳转</el-button>
          <el-button link type="danger" @click="handleDelete(row as Bookmark)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
/**
 * 书签管理组件 — 展示和管理用户保存的 LDAP 条目书签
 * 支持跳转到书签指向的条目，以及删除书签
 */
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getBookmarks, deleteBookmark } from '@/api/bookmark'
import type { Bookmark } from '@/types'

// ==================== 路由初始化 ====================
const router = useRouter()

// ==================== 响应式状态 ====================
const loading = ref(false)               // 列表加载状态
const bookmarks = ref<Bookmark[]>([])    // 书签列表

// ==================== 初始化 ====================
onMounted(loadBookmarks)

/**
 * 从后端加载当前用户的所有书签并填充列表
 * @returns 无返回值
 */
async function loadBookmarks() {
  loading.value = true
  try {
    bookmarks.value = await getBookmarks()
  } finally {
    loading.value = false
  }
}

/**
 * 跳转到书签指向的条目详情页
 * @param row 目标书签（携带条目 DN）
 * @returns 无返回值
 */
function jumpTo(row: Bookmark) {
  router.push({ path: '/entry', query: { dn: row.dn } })
}

/**
 * 删除指定书签（需用户确认，成功后刷新列表）
 * @param row 待删除的书签
 * @returns 无返回值
 */
async function handleDelete(row: Bookmark) {
  await ElMessageBox.confirm(`确定删除书签 "${row.name}" 吗？`, '确认', { type: 'warning' })
  try {
    await deleteBookmark(row.id)
    ElMessage.success('删除成功')
    loadBookmarks()
  } catch {
    // ignore
  }
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
</style>
