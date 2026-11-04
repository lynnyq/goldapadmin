<template>
  <div class="server-info">
    <!-- Header -->
    <div class="si-head">
      <div class="si-main">
        <h2>服务器信息</h2>
        <p>LDAP rootDSE 元信息（对标 phpLDAPadmin server_info.php）</p>
      </div>
      <button class="tb" :disabled="loading" @click="load">
        <el-icon :size="14"><Refresh /></el-icon>{{ loading ? '加载中…' : '刷新' }}
      </button>
    </div>

    <!-- Error -->
    <div v-if="error" class="si-error">
      <el-alert type="error" :closable="false">{{ error }}</el-alert>
    </div>

    <!-- RootDSE 属性卡片 -->
    <div v-if="info" class="si-grid">
      <!-- Naming Contexts -->
      <div class="si-card">
        <div class="sc-title">
          <span class="sc-bar" />
          <span>Naming Contexts</span>
          <code class="sc-attr font-mono">namingContexts</code>
        </div>
        <div class="sc-body">
          <div v-if="info.namingContexts?.length" class="chip-list">
            <el-tag v-for="nc in info.namingContexts" :key="nc" size="large" class="nc-chip">{{ nc }}</el-tag>
          </div>
          <div v-else class="empty-hint">服务器未返回 namingContexts</div>
        </div>
      </div>

      <!-- Supported LDAP Version -->
      <div class="si-card">
        <div class="sc-title">
          <span class="sc-bar" /><span>协议版本</span>
          <code class="sc-attr font-mono">supportedLDAPVersion</code>
        </div>
        <div class="sc-body">
          <div v-if="info.supportedLDAPVersion?.length" class="ver-list">
            <span v-for="v in info.supportedLDAPVersion" :key="v" class="ver-chip font-mono">v{{ v }}</span>
          </div>
          <div v-else class="empty-hint">—</div>
        </div>
      </div>

      <!-- SASL 机制 -->
      <div class="si-card">
        <div class="sc-title">
          <span class="sc-bar" /><span>SASL 机制</span>
          <code class="sc-attr font-mono">supportedSASLMechanisms</code>
        </div>
        <div class="sc-body">
          <div v-if="info.supportedSASLMechanisms?.length" class="chip-list">
            <el-tag v-for="m in info.supportedSASLMechanisms" :key="m" effect="plain" class="sasl-chip">{{ m }}</el-tag>
          </div>
          <div v-else class="empty-hint">服务器未声明 SASL 支持</div>
        </div>
      </div>

      <!-- 服务器厂商 / 名称 -->
      <div class="si-card">
        <div class="sc-title">
          <span class="sc-bar" /><span>服务器身份</span>
        </div>
        <div class="sc-body info-list">
          <div class="info-row">
            <span class="ir-key font-mono">vendorName</span>
            <span class="ir-val">{{ info.vendorName || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="ir-key font-mono">vendorVersion</span>
            <span class="ir-val">{{ info.vendorVersion || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="ir-key font-mono">serverName</span>
            <span class="ir-val">{{ info.serverName || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="ir-key font-mono">subschemaSubentry</span>
            <span class="ir-val">{{ info.subschemaSubentry || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="ir-key font-mono">supportedBindMethods</span>
            <span class="ir-val">{{ info.supportedBindMethods?.join(', ') || '—' }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading skeleton -->
    <div v-else-if="loading" class="si-grid">
      <div v-for="i in 4" :key="i" class="si-card skeleton">
        <div class="sc-title"><div class="sk-bar" /><div class="sk-titleline" /></div>
        <div class="sc-body"><div class="sk-block" /><div class="sk-block short" /></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { getRootDSE } from '@/api/server'

const loading = ref(false)
const error = ref('')
const info = ref<{
  namingContexts: string[]
  supportedLDAPVersion: string[]
  supportedSASLMechanisms: string[]
  supportedBindMethods: string[]
  vendorName: string
  vendorVersion: string
  serverName: string
  subschemaSubentry: string
} | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    info.value = await getRootDSE()
  } catch (e: any) {
    error.value = e?.message || '加载 rootDSE 失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.server-info {
  padding: 20px 24px;
  height: 100%;
  overflow-y: auto;
}

.si-head {
  display: flex; justify-content: space-between; align-items: flex-start;
  margin-bottom: 20px;

  h2 { margin: 0 0 4px; font-size: 18px; font-weight: 600; }
  p { margin: 0; font-size: 13px; color: var(--text-muted); }
}

.tb {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 13px;
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  font-size: 13px; color: var(--text-secondary);
  cursor: pointer;
  transition: all .15s;
  &:hover:not(:disabled) { color: var(--accent-primary); border-color: var(--accent-primary); }
  &:disabled { opacity: .6; cursor: not-allowed; }
}

.si-error { margin-bottom: 16px; }

.si-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
  gap: 14px;
}

.si-card {
  background: var(--surface-panel);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 14px 16px;
}

.sc-title {
  display: flex; align-items: center; gap: 8px; margin-bottom: 12px;
  font-size: 13px; font-weight: 600; color: var(--text-primary);

  .sc-bar {
    width: 3px; height: 14px; border-radius: 2px; background: var(--accent-primary);
  }
  .sc-attr {
    margin-left: auto; font-size: 11px; font-weight: 400; color: var(--text-muted);
  }
}

.sc-body { font-size: 13px; }

.chip-list { display: flex; flex-wrap: wrap; gap: 6px; }

.nc-chip {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  background: color-mix(in oklab, var(--accent-primary) 8%, white);
  border-color: color-mix(in oklab, var(--accent-primary) 22%, white);
  color: var(--accent-primary);
}

.sasl-chip {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
}

.ver-list { display: flex; gap: 6px; }
.ver-chip {
  padding: 4px 10px; border-radius: 4px;
  background: var(--surface-base); border: 1px solid var(--border-subtle);
  font-size: 12px; color: var(--text-primary);
}

.info-list { display: flex; flex-direction: column; gap: 8px; }
.info-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  padding: 6px 10px;
  background: var(--surface-base); border: 1px solid var(--border-subtle); border-radius: 6px;

  .ir-key { font-size: 12px; color: var(--text-muted); }
  .ir-val { font-size: 13px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}

.empty-hint { font-size: 12px; color: var(--text-muted); }

/* Skeleton */
.skeleton { .sk-bar { width: 8px; height: 14px; border-radius: 2px; background: var(--border-subtle); }
  .sk-titleline { flex: 1; height: 12px; background: var(--border-subtle); border-radius: 3px; margin-left: 8px; }
  .sk-block { height: 12px; background: var(--border-subtle); border-radius: 3px; margin-bottom: 8px; }
  .sk-block.short { width: 60%; }
}
</style>
