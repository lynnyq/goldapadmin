<template>
  <div class="page">
    <div class="page-header">
      <h2>服务器管理</h2>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>新建服务器
      </el-button>
    </div>

    <!-- 服务器配置列表 -->
    <el-table :data="servers" border v-loading="loading">
      <el-table-column prop="name" label="名称" width="160" />
      <el-table-column label="地址" min-width="200">
        <template #default="{ row }">
          {{ row.use_ssl ? 'ldaps' : 'ldap' }}://{{ row.host }}:{{ row.port }}
        </template>
      </el-table-column>
      <el-table-column prop="base_dn" label="Base DN" min-width="200" show-overflow-tooltip />
      <el-table-column prop="bind_dn" label="绑定 DN" min-width="200" show-overflow-tooltip />
      <el-table-column label="TLS" width="80" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.use_tls" size="small" type="success">开启</el-tag>
          <el-tag v-else size="small">关闭</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" align="center">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row as LdapServer)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row as LdapServer)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建/编辑弹窗 -->
    <el-dialog v-model="showDialog" :title="isEdit ? '编辑服务器' : '新建服务器'" width="560px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="服务器显示名称" />
        </el-form-item>
        <el-form-item label="主机" prop="host">
          <el-input v-model="form.host" placeholder="192.168.1.10" />
        </el-form-item>
        <el-form-item label="端口" prop="port">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="Base DN" prop="base_dn">
          <el-input v-model="form.base_dn" placeholder="dc=example,dc=com" />
        </el-form-item>
        <el-form-item label="绑定 DN" prop="bind_dn">
          <el-input v-model="form.bind_dn" placeholder="cn=admin,dc=example,dc=com" />
        </el-form-item>
        <el-form-item label="协议">
          <el-checkbox v-model="form.use_tls">TLS (StartTLS)</el-checkbox>
          <el-checkbox v-model="form.use_ssl">SSL (ldaps://)</el-checkbox>
        </el-form-item>
        <el-form-item label="超时(秒)">
          <el-input-number v-model="form.timeout" :min="1" :max="300" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * 服务器管理组件 — 管理多个 LDAP 服务器连接配置
 * 支持新建、编辑、删除服务器配置（主机、端口、Base DN、绑定凭据、TLS/SSL 等）
 */
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useServerStore } from '@/store/server'
import { createServer, updateServer, deleteServer } from '@/api/server'
import type { LdapServer } from '@/types'

// ==================== Store 和状态 ====================
const serverStore = useServerStore()
const loading = ref(false)              // 列表加载状态
const showDialog = ref(false)           // 新建/编辑对话框显示
const isEdit = ref(false)                // 是否为编辑模式
const saving = ref(false)                // 表单保存加载状态
const formRef = ref<FormInstance>()      // 表单实例引用

const servers = ref<LdapServer[]>([])    // 服务器列表

// ==================== 表单数据 ====================
const form = reactive<Partial<LdapServer>>({
  name: '',           // 服务器显示名称
  host: '',           // LDAP 主机地址
  port: 389,          // LDAP 端口（默认 389）
  base_dn: '',        // 基础 DN
  bind_dn: '',        // 绑定 DN
  use_tls: false,     // 是否启用 StartTLS
  use_ssl: false,     // 是否使用 LDAPS（636端口）
  timeout: 30,        // 连接超时秒数
})

// ==================== 表单校验规则 ====================
const rules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  host: [{ required: true, message: '请输入主机', trigger: 'blur' }],
  base_dn: [{ required: true, message: '请输入 Base DN', trigger: 'blur' }],
}

// ==================== 初始化 ====================
onMounted(loadServers)

/**
 * 通过 Store 拉取所有服务器配置并刷新本地列表
 * @returns 无返回值
 */
async function loadServers() {
  loading.value = true
  try {
    await serverStore.loadServers()
    servers.value = serverStore.servers
  } finally {
    loading.value = false
  }
}

/**
 * 打开新建服务器对话框，重置表单为默认值
 * @returns 无返回值
 */
function openCreate() {
  isEdit.value = false
  Object.assign(form, {
    name: '', host: '', port: 389, base_dn: '', bind_dn: '',
    use_tls: false, use_ssl: false, timeout: 30,
  })
  showDialog.value = true
}

/**
 * 打开编辑服务器对话框，将当前行数据预填到表单
 * @param row 待编辑的服务器配置
 * @returns 无返回值
 */
function openEdit(row: LdapServer) {
  isEdit.value = true
  Object.assign(form, row)
  showDialog.value = true
}

/**
 * 校验并保存服务器配置，依据 isEdit 分别调用新建或更新接口，成功后刷新列表
 * @returns 无返回值
 */
async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (isEdit.value && form.id) {
        await updateServer(form.id, form)
        ElMessage.success('更新成功')
      } else {
        await createServer(form)
        ElMessage.success('创建成功')
      }
      showDialog.value = false
      loadServers()
    } finally {
      saving.value = false
    }
  })
}

/**
 * 删除指定服务器配置（需用户确认，成功后刷新列表）
 * @param row 待删除的服务器配置
 * @returns 无返回值
 */
async function handleDelete(row: LdapServer) {
  await ElMessageBox.confirm(`确定删除服务器 "${row.name}" 吗？`, '确认', { type: 'warning' })
  try {
    await deleteServer(row.id)
    ElMessage.success('删除成功')
    loadServers()
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
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }
}
</style>
