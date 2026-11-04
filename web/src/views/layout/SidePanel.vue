<template>
  <div class="side-panel">
    <div class="sp-head">
      <div class="sp-tabs">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          class="sp-tab"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          <el-icon :size="13"><component :is="tab.icon" /></el-icon>
          {{ tab.label }}
        </button>
      </div>
      <button class="sp-close" @click="$emit('close')">
        <el-icon :size="14"><Close /></el-icon>
      </button>
    </div>

    <div class="sp-body">
      <!-- Schema -->
      <div v-show="activeTab === 'schema'">
        <div v-if="entryStore.selectedEntry" class="schema">
          <div class="schema-block">
            <div class="block-title">
              <span class="bt-bar" /> 对象类
            </div>
            <div class="oc-list">
              <button
                v-for="oc in entryStore.selectedEntry.object_classes"
                :key="oc"
                class="oc-chip"
                :class="{ active: currentOC?.name === oc }"
                @click="loadObjectClass(oc)"
              >{{ oc }}</button>
            </div>
          </div>

          <div v-if="currentOC" class="oc-card">
            <div class="oc-card-head">
              <code class="oc-oid">{{ currentOC.oid }}</code>
              <span class="oc-type" :class="'t-' + currentOC.type">{{ currentOC.type }}</span>
            </div>
            <p v-if="currentOC.description" class="oc-desc">{{ currentOC.description }}</p>

            <div class="attr-group must">
              <h5>MUST</h5>
              <div class="attr-chips">
                <span v-for="a in currentOC.must" :key="a" class="attr-chip">{{ a }}</span>
                <span v-if="!currentOC.must.length" class="empty-note">无</span>
              </div>
            </div>

            <div class="attr-group may">
              <h5>MAY</h5>
              <div class="attr-chips">
                <span v-for="a in currentOC.may" :key="a" class="attr-chip">{{ a }}</span>
                <span v-if="!currentOC.may.length" class="empty-note">无</span>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="panel-empty">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
          </svg>
          <p>请选择条目以查看其 Schema 信息</p>
        </div>
      </div>

      <!-- History -->
      <div v-show="activeTab === 'history'">
        <div v-if="recentLogs.length" class="timeline">
          <div
            v-for="(log, idx) in recentLogs"
            :key="log.id"
            class="tl-item"
            :class="{ failed: log.status !== 'success' }"
          >
            <div class="tl-rail">
              <span class="tl-dot" :class="log.status === 'success' ? 'ok' : 'err'" />
              <span v-if="idx < recentLogs.length - 1" class="tl-line" />
            </div>
            <div class="tl-content">
              <div class="tl-row">
                <span class="tl-action">{{ formatAction(log.action) }}</span>
                <span class="tl-time font-mono">{{ formatTime(log.created_at) }}</span>
              </div>
              <code class="tl-target">{{ log.target_dn }}</code>
              <p v-if="log.status !== 'success'" class="tl-error">{{ log.error_msg }}</p>
            </div>
          </div>
        </div>

        <div v-else class="panel-empty">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <polyline points="12 6 12 12 16 14"/>
          </svg>
          <p>暂无最近操作记录</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 右侧属性面板组件 — 提供两个标签页
 * Schema 标签：展示当前选中条目的对象类详情（MUST/MAY 属性）
 * 操作历史标签：展示最近的操作日志时间线
 */
import { ref, watch } from 'vue'
import { Close, Document, Clock } from '@element-plus/icons-vue'
import { getObjectClass } from '@/api/schema'
import { useEntryStore } from '@/store/entry'
import type { ObjectClassDef, OperationLog } from '@/types'

/** 事件声明：close — 用户点击面板关闭按钮时通知父组件收起面板 */
const emit = defineEmits<{ close: [] }>()

// ==================== Store 初始化 ====================
const entryStore = useEntryStore()

// ==================== 标签页配置 ====================
const tabs = [
  { id: 'schema', label: 'Schema',  icon: Document },
  { id: 'history', label: '操作历史', icon: Clock },
]

// ==================== 响应式状态 ====================
const activeTab = ref('schema')                // 当前激活的标签页
const currentOC = ref<ObjectClassDef | null>(null)  // 当前选中的对象类定义
const recentLogs = ref<OperationLog[]>([])      // 最近操作日志列表

/** 监听选中条目变化，自动加载其第一个对象类详情 */
watch(
  () => entryStore.selectedEntry?.object_classes,
  async ocs => {
    currentOC.value = null
    if (ocs?.length) await loadObjectClass(ocs[0])
  },
  { immediate: true },
)

/** 根据对象类名称从后端获取详细定义 */
async function loadObjectClass(name: string) {
  try { currentOC.value = await getObjectClass(name) } catch { currentOC.value = null }
}

/** 将英文操作类型码转换为中文显示 */
function formatAction(a: string) {
  const m: Record<string, string> = {
    create: '创建', modify: '修改', delete: '删除',
    move: '移动', copy: '复制', search: '搜索', bind: '绑定',
  }
  return m[a] || a
}
/** 将 ISO 时间戳格式化为 HH:mm:ss 格式 */
function formatTime(ts: string) {
  if (!ts) return ''
  try {
    return new Date(ts).toLocaleTimeString('en-US', { hour12: false })
  } catch { return ts }
}
</script>

<style scoped lang="scss">
.side-panel {
  height: 100%;
  display: flex; flex-direction: column;
}

/* Head */
.sp-head {
  height: 42px;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 8px 0 12px;
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.sp-tabs { display: flex; gap: 2px; }

.sp-tab {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 5px 10px;
  background: none; border: none;
  border-radius: var(--radius-2);
  font-size: var(--fs-label);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover { color: var(--text-primary); background: var(--surface-hover); }

  &.active {
    color: var(--accent-primary);
    background: var(--accent-primary-soft);
  }
}

.sp-close {
  width: 28px; height: 28px;
  display: flex; align-items: center; justify-content: center;
  background: none; border: none; border-radius: var(--radius-1);
  color: var(--text-muted); cursor: pointer;
  transition: all 0.15s ease;

  &:hover { color: var(--text-primary); background: var(--surface-hover); }
}

/* Body */
.sp-body {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
}

/* Schema content */
.schema {
  display: flex; flex-direction: column; gap: 14px;
}

.schema-block {
  .block-title {
    display: flex; align-items: center; gap: 8px;
    font-size: var(--fs-label);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
    margin-bottom: 8px;
  }
  .bt-bar {
    width: 3px; height: 10px; border-radius: 2px;
    background: var(--accent-primary);
  }
}

.oc-list { display: flex; flex-wrap: wrap; gap: 5px; }

.oc-chip {
  padding: 4px 10px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  font-size: 12px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.12s ease;

  &:hover { border-color: var(--border-default); color: var(--text-primary); }

  &.active {
    background: var(--accent-primary-soft);
    border-color: var(--accent-primary);
    color: var(--accent-primary);
    font-weight: 500;
  }
}

.oc-card {
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-3);
  padding: 14px;
}

.oc-card-head {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 8px;
}

.oc-oid {
  font-size: var(--fs-caption);
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-1);
  padding: 1px 6px;
  color: var(--text-muted);
}

.oc-type {
  font-size: var(--fs-caption);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 1px 7px;
  border-radius: var(--radius-1);

  &.t-structural { background: color-mix(in oklab, var(--accent-primary) 14%, white); color: var(--accent-primary); }
  &.t-auxiliary  { background: color-mix(in oklab, var(--green-600) 16%, white); color: var(--green-700); }
  &.t-abstract   { background: color-mix(in oklab, var(--purple-500) 14%, white); color: var(--purple-500); }
}

.oc-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-bottom: 14px;
}

.attr-group {
  margin-top: 12px;

  h5 {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-muted);
    margin-bottom: 6px;
  }

  &.must .attr-chip {
    background: color-mix(in oklab, var(--red-500) 14%, white);
    color: var(--red-600);
    border-color: color-mix(in oklab, var(--red-500) 22%, white);
  }

  &.may .attr-chip {
    background: color-mix(in oklab, var(--green-600) 14%, white);
    color: var(--green-700);
    border-color: color-mix(in oklab, var(--green-600) 22%, white);
  }
}

.attr-chips { display: flex; flex-wrap: wrap; gap: 4px; }

.attr-chip {
  font-size: var(--fs-caption);
  font-family: var(--font-mono);
  padding: 2px 7px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-1);
}

.empty-note {
  font-size: var(--fs-caption);
  color: var(--text-muted);
  font-style: italic;
}

/* Timeline */
.timeline { display: flex; flex-direction: column; gap: 0; }

.tl-item { display: flex; gap: 10px; padding: 10px 0; }

.tl-rail {
  width: 14px;
  display: flex; flex-direction: column; align-items: center;
  flex-shrink: 0;
}

.tl-dot {
  width: 8px; height: 8px; border-radius: 50%;
  flex-shrink: 0;

  &.ok  { background: var(--accent-success); }
  &.err { background: var(--accent-danger); }
}

.tl-line {
  width: 1px;
  flex: 1;
  background: var(--border-subtle);
  margin-top: 4px;
}

.tl-content { flex: 1; min-width: 0; }

.tl-row {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 3px;
}

.tl-action {
  font-size: var(--fs-label);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-primary);
}

.tl-time {
  font-size: var(--fs-caption);
  color: var(--text-muted);
}

.tl-target {
  display: block;
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

.tl-error {
  margin-top: 3px;
  font-size: 12px;
  color: var(--accent-danger);
}

/* Empty */
.panel-empty {
  height: calc(100% - 32px);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 10px;
  color: var(--text-muted);

  svg { opacity: 0.4; }
  p { font-size: var(--fs-meta); margin: 0; }
}
</style>
