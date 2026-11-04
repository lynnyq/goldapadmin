<template>
  <div class="page">
    <div class="page-header">
      <h2>批量导入 (LDIF)</h2>
    </div>

    <!-- 导入内容区：支持拖拽上传 .ldif 文件或直接粘贴文本 -->
    <el-card>
      <el-upload
        :auto-upload="false"
        :on-change="onFile"
        accept=".ldif,.txt"
        drag
        style="margin-bottom: 16px"
      >
        <el-icon :size="48" class="upload-icon"><UploadFilled /></el-icon>
        <p>将 LDIF 文件拖到此处，或<em>点击选择</em></p>
      </el-upload>

      <el-input
        v-model="content"
        type="textarea"
        :rows="16"
        placeholder="或直接粘贴 LDIF 内容..."
      />

      <div class="actions">
        <el-button @click="content = ''">清空</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">
          <el-icon><Upload /></el-icon>执行导入
        </el-button>
      </div>
    </el-card>

    <!-- 导入结果区：成功/失败统计与错误详情 -->
    <el-card v-if="result" header="导入结果" style="margin-top: 16px">
      <el-result
        :icon="result.failed === 0 ? 'success' : 'warning'"
        :title="`成功 ${result.success} 条，失败 ${result.failed} 条`"
      />
      <div v-if="result.errors?.length" class="errors">
        <h4>错误详情：</h4>
        <ul>
          <li v-for="(e, i) in result.errors" :key="i" class="error-item">{{ e }}</li>
        </ul>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
/**
 * LDIF 批量导入组件 — 支持从文件上传或文本粘贴导入 LDAP 条目
 * 导入完成后展示成功/失败统计和错误详情
 */
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Upload, UploadFilled } from '@element-plus/icons-vue'
import { importLdif } from '@/api/ldif'

// ==================== 响应式状态 ====================
const content = ref('')       // LDIF 文件内容或粘贴文本
const importing = ref(false)  // 导入进行中
const result = ref<{ success: number; failed: number; errors: string[] } | null>(null)  // 导入结果统计

/**
 * 处理文件选择，异步读取 LDIF 文件文本内容并填入编辑区
 * @param file el-upload 的 on-change 回调文件对象（raw 为原始 File）
 * @returns 无返回值
 */
function onFile(file: any) {
  const reader = new FileReader()
  reader.onload = (e) => {
    content.value = e.target?.result as string
  }
  reader.readAsText(file.raw)
}

/**
 * 校验内容非空后提交 LDIF 导入请求，按失败条数给出提示并展示统计结果
 * @returns 无返回值
 */
async function doImport() {
  if (!content.value.trim()) {
    ElMessage.warning('请输入 LDIF 内容')
    return
  }
  importing.value = true
  result.value = null
  try {
    result.value = await importLdif(content.value)
    if (result.value.failed === 0) {
      ElMessage.success('导入成功')
    } else {
      ElMessage.warning('部分条目导入失败')
    }
  } finally {
    importing.value = false
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

.upload-icon {
  color: var(--accent-primary);
}

.actions {
  margin-top: 16px;
  text-align: right;
}

.errors {
  margin-top: 12px;

  h4 {
    margin-bottom: 8px;
    color: var(--accent-danger);
  }

  .error-item {
    color: var(--accent-danger);
    font-size: 13px;
    padding: 4px 0;
    font-family: var(--font-mono);
  }
}
</style>
