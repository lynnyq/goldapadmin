<template>
  <el-container class="main-layout">
    <!-- Top Bar -->
    <el-header class="topbar" height="56px">
      <!-- Left cluster -->
      <div class="top-left">
        <div class="app-mark" @click="$router.push('/')">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <rect x="1"  y="1"  width="6" height="6" rx="1.5" fill="var(--accent-primary)" />
            <rect x="11" y="1"  width="6" height="6" rx="1.5" fill="var(--green-600)" />
            <rect x="1"  y="11" width="6" height="6" rx="1.5" fill="var(--amber-600)" />
            <rect x="11" y="11" width="6" height="6" rx="1.5" fill="var(--purple-500)" />
          </svg>
          <span class="app-name">GoLDAPAdmin</span>
        </div>

        <div class="top-sep" />

        <el-select
          v-model="currentServerId"
          size="small"
          class="svr-picker"
          placeholder="选择服务器"
          @change="onServerChange"
        >
          <template #prefix>
            <span class="svr-dot" :class="{ online: !!authStore.serverName }" />
          </template>
          <el-option
            v-for="s in serverStore.servers"
            :key="s.id"
            :label="s.name"
            :value="s.id"
          >
            <div class="svr-pick-opt">
              <span class="sp-name">{{ s.name }}</span>
              <span class="sp-addr font-mono">{{ s.use_ssl ? 'ldaps' : 'ldap' }}://{{ s.host }}:{{ s.port }}</span>
            </div>
          </el-option>
        </el-select>
      </div>

      <!-- Center search -->
      <div class="top-center">
        <div class="search-wrap">
          <el-icon :size="14" class="search-ic"><Search /></el-icon>
          <input
            v-model="searchKeyword"
            class="search-in"
            placeholder="搜索 DN 或属性值..."
            @keyup.enter="goSearch"
          />
          <kbd class="kbd" v-if="!searchKeyword">⌘K</kbd>
        </div>
      </div>

      <!-- Right cluster -->
      <div class="top-right">
        <nav class="nav-rail">
          <el-tooltip content="高级搜索" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/search') }" @click="$router.push('/search')">
              <el-icon :size="17"><Search /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="书签" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/bookmarks') }" @click="$router.push('/bookmarks')">
              <el-icon :size="17"><Star /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="监控" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/monitor') }" @click="$router.push('/monitor')">
              <el-icon :size="17"><DataLine /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="Schema 浏览器" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/schema') }" @click="$router.push('/schema')">
              <el-icon :size="17"><Collection /></el-icon>
            </button>
          </el-tooltip>
          <el-tooltip content="操作日志" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/logs') }" @click="$router.push('/logs')">
              <el-icon :size="17"><Document /></el-icon>
            </button>
          </el-tooltip>

          <div class="nav-sep" />

          <el-tooltip content="服务器管理" placement="bottom">
            <button class="nav-btn" :class="{ active: isActive('/servers') }" @click="$router.push('/servers')">
              <el-icon :size="17"><Setting /></el-icon>
            </button>
          </el-tooltip>
        </nav>

        <el-dropdown trigger="click" @command="handleCommand">
          <div class="user-chip">
            <div class="avatar">
              <el-icon :size="13"><User /></el-icon>
            </div>
            <span class="chip-name">{{ shortUserName }}</span>
            <el-icon :size="11" class="chip-chev"><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <div class="dd-user-block">
                <span class="dd-label">连接身份</span>
                <span class="dd-dn font-mono">{{ authStore.userDN }}</span>
              </div>
              <el-dropdown-item divided command="logout">
                <el-icon><SwitchButton /></el-icon>
                <span>断开连接</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-header>

    <!-- Three-column body -->
    <el-container class="main-body">
      <!-- Left: tree -->
      <el-aside :width="leftCollapsed ? '44px' : '260px'" class="left-pane">
        <div class="pane-head">
          <span v-if="!leftCollapsed" class="pane-title">
            <span class="pt-bar" />
            目录树
          </span>
          <button class="collapse-btn" @click="leftCollapsed = !leftCollapsed">
            <el-icon :size="14"><Fold v-if="!leftCollapsed" /><Expand v-else /></el-icon>
          </button>
        </div>
        <ServerTree v-if="!leftCollapsed" />
        <div v-else class="collapsed-hint">
          <el-icon :size="16"><Folder /></el-icon>
        </div>
      </el-aside>

      <!-- Center -->
      <el-main class="center-pane">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>

      <!-- Right -->
      <el-aside :width="rightCollapsed ? '0px' : '280px'" class="right-pane">
        <SidePanel v-if="!rightCollapsed" @close="rightCollapsed = true" />
      </el-aside>

      <!-- Right expand handle -->
      <button
        v-if="rightCollapsed"
        class="right-handle"
        @click="rightCollapsed = false"
      >
        <el-icon :size="14"><Expand /></el-icon>
      </button>
    </el-container>

    <!-- Status bar -->
    <el-footer class="statusbar" height="30px">
      <div class="sb-left">
        <span class="sb-item">
          <span class="sb-dot" :class="authStore.serverName ? 'online' : 'offline'" />
          <span>{{ authStore.serverName || '未连接' }}</span>
        </span>
        <span class="sb-sep" />
        <span class="sb-item font-mono">
          <el-icon :size="12"><Connection /></el-icon>
          {{ entryStore.selectedDN ? truncate(entryStore.selectedDN, 48) : '未选中条目' }}
        </span>
      </div>
      <div class="sb-right">
        <span class="sb-item font-mono tabular-nums">{{ currentTime }}</span>
      </div>
    </el-footer>
  </el-container>
</template>

<script setup lang="ts">
/**
 * 主布局组件 — 提供三栏式 LDAP 管理界面骨架
 * 顶部栏包含服务器选择、全局搜索和导航按钮
 * 左侧为目录树，中间为路由视图，右侧为属性面板
 * 底部为状态栏，显示连接状态和当前时间
 */
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Search, Star, DataLine, Setting, User, Fold, Expand,
  Collection, Document, ArrowDown, SwitchButton, Folder, Connection,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'
import { useServerStore } from '@/store/server'
import { useEntryStore } from '@/store/entry'
import ServerTree from './ServerTree.vue'
import SidePanel from './SidePanel.vue'

// ==================== 路由和 Store 初始化 ====================
const router  = useRouter()
const route   = useRoute()
const authStore  = useAuthStore()
const serverStore = useServerStore()
const entryStore = useEntryStore()

// ==================== 响应式状态 ====================
const leftCollapsed  = ref(false)   // 左侧目录树折叠状态
const rightCollapsed = ref(false)   // 右侧面板折叠状态
const searchKeyword  = ref('')      // 全局搜索关键词
const currentServerId = ref(serverStore.currentServerId)  // 当前选中的服务器 ID
const currentTime    = ref('')      // 状态栏显示的当前时间

let timer: number | undefined  // 定时更新时间的定时器句柄

/** 从用户 DN 中提取简短的用户名用于显示 */
const shortUserName = computed(() => {
  const dn = authStore.userDN
  if (!dn) return 'anonymous'
  const first = (dn.split(',')[0] || dn).replace(/^(cn|uid|uidNumber)=/i, '')
  return first.slice(0, 24)
})

// ==================== 生命周期 ====================
/** 挂载时加载服务器列表，并启动每秒刷新状态栏时间的定时器 */
onMounted(() => {
  serverStore.loadServers()
  updateTime()
  timer = window.setInterval(updateTime, 1000)
})
/** 卸载时清除时间定时器，防止内存泄漏 */
onUnmounted(() => { if (timer) clearInterval(timer) })

/** 更新状态栏显示的当前时间 */
function updateTime() {
  currentTime.value = new Date().toLocaleTimeString('en-US', { hour12: false })
}
/** 判断当前路由是否匹配指定路径前缀 */
function isActive(p: string) { return route.path.startsWith(p) }
/** 截断长字符串到指定长度（null/undefined 安全） */
function truncate(s: string | null | undefined, n: number) {
  if (!s) return ''
  return s.length <= n ? s : s.slice(0, n - 3) + '...'
}

/** 切换当前连接的服务器 */
function onServerChange(id: number) {
  serverStore.setCurrentServer(id)
  entryStore.refreshTree()
  ElMessage.success('服务器已切换')
}
/** 执行全局搜索，跳转到高级搜索页 */
function goSearch() {
  if (searchKeyword.value) router.push({ path: '/search', query: { q: searchKeyword.value } })
}
/** 处理用户下拉菜单命令，目前支持登出 */
async function handleCommand(cmd: string) {
  if (cmd === 'logout') { await authStore.logout(); router.push('/login') }
}
</script>

<style scoped lang="scss">
.main-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}

/* ===== Top bar ===== */
.topbar {
  background: var(--surface-panel) !important;
  border-bottom: 1px solid var(--border-subtle) !important;
  box-shadow: var(--elevation-1);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  gap: 16px;
  flex-shrink: 0;
}

.top-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.app-mark {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: var(--radius-2);
  cursor: pointer;
  transition: background 0.15s ease;

  &:hover { background: var(--surface-hover); }

  .app-name {
    font-weight: 700;
    font-size: 14px;
    letter-spacing: -0.005em;
    color: var(--text-primary);
  }
}

.top-sep { width: 1px; height: 22px; background: var(--border-subtle); }

.svr-picker {
  width: 184px;

  :deep(.el-select__wrapper) {
    background: var(--surface-base) !important;
    box-shadow: 0 0 0 1px var(--border-subtle) inset !important;
    padding: 4px 8px;
  }

  .svr-dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--neutral-400);
    margin-right: 6px;
    flex-shrink: 0;
    transition: background 0.2s ease;
    box-shadow: 0 0 0 2px transparent;

    &.online {
      background: var(--accent-success);
      box-shadow: 0 0 0 2px var(--accent-success-soft);
    }
  }
}

.svr-pick-opt {
  display: flex; flex-direction: column; gap: 2px;
  .sp-name { font-weight: 600; font-size: 13px; }
  .sp-addr { font-size: var(--fs-caption); color: var(--text-muted); }
}

/* Center search */
.top-center {
  flex: 1;
  max-width: 520px;
  display: flex;
  justify-content: center;
}

.search-wrap {
  position: relative;
  width: 100%;
  display: flex;
  align-items: center;
}

.search-ic {
  position: absolute;
  left: 12px;
  color: var(--text-muted);
  pointer-events: none;
}

.search-in {
  width: 100%;
  height: 36px;
  padding: 0 56px 0 34px;
  background: var(--surface-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  color: var(--text-primary);
  font: inherit;
  font-size: var(--fs-body);
  outline: none;
  transition: all 0.15s ease;

  &::placeholder { color: var(--text-muted); }

  &:hover { border-color: var(--border-default); }

  &:focus {
    border-color: var(--accent-primary);
    box-shadow: var(--focus-ring);
    background: var(--surface-panel);
  }
}

.kbd {
  position: absolute;
  right: 10px;
  padding: 1px 6px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-1);
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Right cluster */
.top-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.nav-rail {
  display: flex;
  align-items: center;
  gap: 2px;
}

.nav-btn {
  width: 34px; height: 34px;
  display: flex;
  align-items: center; justify-content: center;
  background: none; border: none; border-radius: var(--radius-2);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover { background: var(--surface-hover); color: var(--text-primary); }

  &.active {
    color: var(--accent-primary);
    background: var(--accent-primary-soft);
  }
}

.nav-sep { width: 1px; height: 20px; background: var(--border-subtle); margin: 0 4px; }

.user-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 8px 3px 3px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: 999px;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover { border-color: var(--border-default); box-shadow: var(--elevation-1); }

  .avatar {
    width: 26px; height: 26px;
    border-radius: 50%;
    background: linear-gradient(135deg, var(--accent-primary), var(--purple-500));
    color: #fff;
    display: flex; align-items: center; justify-content: center;
  }

  .chip-name {
    font-size: var(--fs-meta);
    font-weight: 500;
    max-width: 160px;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }

  .chip-chev { color: var(--text-muted); }
}

/* ===== Body ===== */
.main-body {
  flex: 1;
  overflow: hidden;
  position: relative;
}

/* Left pane */
.left-pane {
  background: var(--surface-base) !important;
  border-right: 1px solid var(--border-subtle) !important;
  transition: width 0.22s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  overflow: hidden;

  .pane-head {
    height: 42px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .pane-title {
    display: flex; align-items: center; gap: 8px;
    font-size: var(--fs-label);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }

  .pt-bar {
    width: 3px; height: 12px;
    border-radius: 2px;
    background: var(--accent-primary);
  }

  .collapse-btn {
    width: 28px; height: 28px;
    display: flex; align-items: center; justify-content: center;
    background: none; border: none;
    border-radius: var(--radius-1);
    color: var(--text-muted); cursor: pointer;
    transition: all 0.15s ease;

    &:hover { background: var(--surface-hover); color: var(--text-primary); }
  }

  .collapsed-hint {
    flex: 1;
    min-height: 0;
    display: flex; align-items: center; justify-content: center;
    color: var(--text-muted);
  }
}

.center-pane {
  background: var(--surface-page) !important;
  padding: 0 !important;
  overflow: hidden;
}

.right-pane {
  background: var(--surface-base) !important;
  border-left: 1px solid var(--border-subtle) !important;
  transition: width 0.22s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.right-handle {
  position: absolute;
  right: 6px; top: 50%;
  transform: translateY(-50%);
  width: 24px; height: 52px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-2);
  box-shadow: var(--elevation-2);
  color: var(--text-muted);
  display: flex; align-items: center; justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
  z-index: 10;

  &:hover {
    color: var(--accent-primary);
    border-color: var(--accent-primary);
  }
}

/* ===== Status bar ===== */
.statusbar {
  background: var(--surface-panel) !important;
  border-top: 1px solid var(--border-subtle) !important;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 14px;
  flex-shrink: 0;

  .sb-left, .sb-right {
    display: flex; align-items: center; gap: 10px;
  }

  .sb-item {
    display: inline-flex; align-items: center; gap: 6px;
    font-size: var(--fs-caption);
    color: var(--text-secondary);
  }

  .sb-sep { width: 1px; height: 14px; background: var(--border-subtle); }

  .sb-dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--neutral-400);

    &.online { background: var(--accent-success); }
    &.offline { background: var(--accent-danger); }
  }
}

/* ===== Page transition ===== */
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.12s ease;
}
.page-fade-enter-from,
.page-fade-leave-to { opacity: 0; }

/* Dropdown user block */
.dd-user-block {
  padding: 12px 16px;
  background: var(--surface-base);
  margin: -12px -12px 6px;
  border-radius: var(--radius-2) var(--radius-2) 0 0;
  border-bottom: 1px solid var(--border-subtle);

  .dd-label {
    display: block;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .dd-dn {
    font-size: var(--fs-meta);
    color: var(--text-primary);
    word-break: break-all;
  }
}
</style>
