<template>
  <div class="page">
    <div class="page-header">
      <h2>操作日志</h2>
    </div>

    <!-- 审计日志列表：操作类型按类别着色，底部为分页器 -->
    <el-card>
      <el-table :data="logs" border v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="action" label="操作" width="120">
          <template #default="{ row }">
            <el-tag :type="actionType(row.action)" size="small">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_dn" label="目标 DN" min-width="280" show-overflow-tooltip />
        <el-table-column prop="user_dn" label="操作用户" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="100" align="center">
          <template #default="{ row }">{{ row.duration_ms }}ms</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="error_msg" label="错误信息" min-width="200" show-overflow-tooltip />
      </el-table>

      <el-pagination
        style="margin-top: 16px; justify-content: flex-end"
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadLogs"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
/**
 * 操作日志组件 — 展示所有 LDAP 操作的审计日志列表
 * 支持分页浏览，记录操作类型、目标 DN、操作用户、执行状态、耗时和错误信息
 */
import { ref, onMounted } from 'vue'
import { getLogs } from '@/api/log'
import type { OperationLog } from '@/types'

// ==================== 响应式状态 ====================
const loading = ref(false)             // 列表加载状态
const logs = ref<OperationLog[]>([])   // 日志记录列表
const total = ref(0)                  // 日志总数（用于分页）
const page = ref(1)                   // 当前页码
const pageSize = ref(20)              // 每页条数

// ==================== 初始化 ====================
onMounted(loadLogs)

/**
 * 按当前页码从后端加载操作日志，并同步总条数供分页器使用
 * @returns 无返回值
 */
async function loadLogs() {
  loading.value = true
  try {
    const res = await getLogs({ page: page.value, page_size: pageSize.value })
    logs.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

/**
 * 根据操作类型返回 el-tag 组件的 type 属性（用于操作列着色）
 * @param action 操作类型名称（如 create/add/delete/modify 等）
 * @returns el-tag 类型：新增=success，删除=danger，修改=warning，其他=primary
 */
function actionType(action: string): 'primary' | 'success' | 'warning' | 'danger' {
  if (action.includes('create') || action.includes('add')) return 'success'
  if (action.includes('delete')) return 'danger'
  if (action.includes('modify')) return 'warning'
  return 'primary'
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
