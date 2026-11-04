<template>
  <div class="page">
    <div class="page-header">
      <h2>Schema 浏览器</h2>
    </div>

    <!-- 双标签页：对象类定义 / 属性类型定义 -->
    <el-tabs v-model="activeTab">
      <el-tab-pane label="Object Classes" name="oc">
        <el-input
          v-model="ocFilter"
          placeholder="搜索 objectClass..."
          style="width: 300px; margin-bottom: 12px"
          clearable
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-table :data="filteredOCs" border v-loading="loading" height="calc(100vh - 240px)">
          <el-table-column prop="name" label="名称" width="220" />
          <el-table-column prop="description" label="描述" min-width="240" show-overflow-tooltip />
          <el-table-column label="类型" width="120">
            <template #default="{ row }">
              <el-tag size="small" :type="ocTypeColor(row.type)">{{ row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-button link type="primary" @click="viewOC(row as ObjectClassDef)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="Attribute Types" name="at">
        <el-input
          v-model="atFilter"
          placeholder="搜索 attribute..."
          style="width: 300px; margin-bottom: 12px"
          clearable
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-table :data="filteredATs" border v-loading="loading" height="calc(100vh - 240px)">
          <el-table-column prop="name" label="名称" width="220" />
          <el-table-column prop="description" label="描述" min-width="240" show-overflow-tooltip />
          <el-table-column prop="syntax" label="语法" width="160" show-overflow-tooltip />
          <el-table-column label="单值" width="80" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.single_value" size="small" type="success">是</el-tag>
              <el-tag v-else size="small">否</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- ObjectClass 详情 -->
    <el-dialog v-model="showOCDetail" :title="selectedOC?.name" width="640px">
      <div v-if="selectedOC">
        <p><strong>OID：</strong>{{ selectedOC.oid }}</p>
        <p><strong>描述：</strong>{{ selectedOC.description }}</p>
        <p><strong>类型：</strong>{{ selectedOC.type }}</p>
        <p v-if="selectedOC.superior?.length">
          <strong>父类：</strong>{{ selectedOC.superior.join(', ') }}
        </p>
        <el-divider />
        <h4>必填属性 (MUST)</h4>
        <div class="tag-row">
          <el-tag v-for="a in selectedOC.must" :key="a" size="small" type="danger">{{ a }}</el-tag>
        </div>
        <h4>可选属性 (MAY)</h4>
        <div class="tag-row">
          <el-tag v-for="a in selectedOC.may" :key="a" size="small" type="success">{{ a }}</el-tag>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * Schema 浏览器组件 — 展示 LDAP 服务器的对象类和属性类型定义
 * 提供 Object Classes 和 Attribute Types 两个标签页，
 * 支持按名称搜索过滤，以及查看对象类的详细属性定义
 */
import { ref, computed, onMounted } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { getObjectClasses, getAttributeTypes } from '@/api/schema'
import type { ObjectClassDef, AttributeTypeDef } from '@/types'

// ==================== 响应式状态 ====================
const activeTab = ref('oc')                    // 当前激活标签页：oc=对象类, at=属性类型
const loading = ref(false)                     // 数据加载状态
const ocFilter = ref('')                       // 对象类搜索关键词
const atFilter = ref('')                       // 属性类型搜索关键词
const objectClasses = ref<ObjectClassDef[]>([])       // 所有对象类定义
const attributeTypes = ref<AttributeTypeDef[]>([])    // 所有属性类型定义
const showOCDetail = ref(false)                // 对象类详情对话框显示
const selectedOC = ref<ObjectClassDef | null>(null)  // 当前选中的对象类

/**
 * 根据搜索关键词过滤对象类列表（按名称忽略大小写匹配）
 * @returns 过滤后的对象类定义数组
 */
const filteredOCs = computed(() =>
  objectClasses.value.filter((oc) =>
    oc.name.toLowerCase().includes(ocFilter.value.toLowerCase()),
  ),
)

/**
 * 根据搜索关键词过滤属性类型列表（按名称忽略大小写匹配）
 * @returns 过滤后的属性类型定义数组
 */
const filteredATs = computed(() =>
  attributeTypes.value.filter((at) =>
    at.name.toLowerCase().includes(atFilter.value.toLowerCase()),
  ),
)

// ==================== 初始化 ====================
/** 挂载后并发拉取对象类与属性类型定义 */
onMounted(async () => {
  loading.value = true
  try {
    const [ocs, ats] = await Promise.all([getObjectClasses(), getAttributeTypes()])
    objectClasses.value = ocs
    attributeTypes.value = ats
  } finally {
    loading.value = false
  }
})

/**
 * 根据对象类类型返回 el-tag 组件的 type 属性（用于类型列着色）
 * @param type 对象类类型（structural/auxiliary/abstract 等）
 * @returns el-tag 类型：structural=primary，auxiliary=success，其他=warning
 */
function ocTypeColor(type: string): 'primary' | 'success' | 'warning' {
  if (type === 'structural') return 'primary'
  if (type === 'auxiliary') return 'success'
  return 'warning'
}

/**
 * 打开对象类详情对话框（展示 OID、父类及 MUST/MAY 属性）
 * @param row 要查看详情的对象类定义
 * @returns 无返回值
 */
function viewOC(row: ObjectClassDef) {
  selectedOC.value = row
  showOCDetail.value = true
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

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
}
</style>
