<template>
  <!--
    登录页 —— 双栏 Flex 布局（非 Grid，避免固定宽度试探）
    左右各 flex:1 自然分流，内容区用 max-width 约束但不压扁
    表单 label-position="top" 让标签自然左对齐 → 与 section-head 对齐
  -->
  <div class="login-shell">

    <!-- ============ 左侧品牌区 ============ -->
    <aside class="brand">
      <!-- Logo -->
      <div class="brand-logo">
        <svg width="32" height="32" viewBox="0 0 44 44" fill="none" aria-hidden="true">
          <rect x="3"  y="3"  width="16" height="16" rx="4" fill="var(--accent-primary)" />
          <rect x="25" y="3"  width="16" height="16" rx="4" fill="var(--green-600)" />
          <rect x="3"  y="25" width="16" height="16" rx="4" fill="var(--amber-600)" />
          <rect x="25" y="25" width="16" height="16" rx="4" fill="var(--purple-500)" />
        </svg>
        <span class="logo-text">GoLDAPAdmin</span>
      </div>

      <!-- 主标题 + 简介 -->
      <div class="brand-copy">
        <h1 class="brand-title">更智能的 LDAP 目录管理方式</h1>
        <p class="brand-desc">专业的目录服务管理平台，提供直观的条目浏览、搜索、Schema 编辑和多服务器管理能力。</p>
      </div>

      <!-- 功能列表 -->
      <ul class="brand-list">
        <li><span class="bl-num">01</span><span class="bl-text">条目级增删改查与属性编辑</span></li>
        <li><span class="bl-num">02</span><span class="bl-text">高级 LDAP 过滤器与批量操作</span></li>
        <li><span class="bl-num">03</span><span class="bl-text">多服务器统一管理与 TLS 加密</span></li>
      </ul>

      <!-- 底部 -->
      <div class="brand-foot">v1.0.0 · 单二进制部署</div>
    </aside>

    <!-- ============ 右侧表单区 ============ -->
    <section class="form-wrap">
      <div class="form-body">
        <!-- 头部 -->
        <header class="form-head">
          <span class="head-eyebrow">连接到目录服务</span>
          <h2 class="head-title">登录目录服务器</h2>
          <p class="head-desc">选择已保存的服务器，或手动填写连接参数。</p>
        </header>

        <!-- 表单主体 -->
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          size="large"
          @keyup.enter="handleLogin"
          class="form-main"
        >
          <!-- Section 1: 快速连接（仅存在已保存的服务器时显示） -->
          <div v-if="servers.length > 0" class="form-sec">
            <div class="sec-title">
              <span class="sec-num">01</span>
              <span>快速连接</span>
              <span class="sec-rule" />
            </div>
            <el-form-item label="已保存的服务器" prop="server_id">
              <el-select
                v-model="form.server_id"
                placeholder="选择一个已保存的服务器..."
                clearable
                filterable
                class="full"
                @change="onServerChange"
              >
                <el-option
                  v-for="s in servers"
                  :key="s.id"
                  :label="s.name"
                  :value="s.id"
                >
                  <div class="svr-opt">
                    <span>{{ s.name }}</span>
                    <span class="svr-addr font-mono">{{ s.use_ssl ? 'ldaps' : 'ldap' }}://{{ s.host }}:{{ s.port }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </div>

          <!-- Section 2: 手动填写（无保存连接时为默认模式；有保存连接时清除选择后展开） -->
          <div v-if="!form.server_id" class="form-sec">
            <div class="sec-title">
              <span class="sec-num">{{ servers.length > 0 ? '02' : '01' }}</span>
              <span>手动填写连接参数</span>
              <span class="sec-rule" />
            </div>
            <div class="two-col">
              <el-form-item label="主机地址">
                <el-input v-model="form.host" placeholder="192.168.1.10 或 ldap.example.com" />
              </el-form-item>
              <el-form-item label="基准 DN">
                <el-input v-model="form.base_dn" placeholder="dc=example,dc=com" />
              </el-form-item>
            </div>
          </div>

          <!-- Section 3: 认证 -->
          <div class="form-sec">
            <div class="sec-title">
              <span class="sec-num">{{ !form.server_id && servers.length > 0 ? '03' : '02' }}</span>
              <span>认证信息</span>
              <span class="sec-rule" />
            </div>
            <el-form-item label="绑定 DN" prop="bind_dn">
              <el-input v-model="form.bind_dn" placeholder="cn=admin,dc=example,dc=com" class="font-mono" />
            </el-form-item>
            <el-form-item label="密码" prop="password">
              <el-input v-model="form.password" type="password" show-password placeholder="请输入绑定密码">
                <template #prefix>
                  <el-icon class="prefix-ic"><Lock /></el-icon>
                </template>
              </el-input>
            </el-form-item>

            <!-- 复选框选项（inline 但不挤） -->
            <div class="form-checks">
              <el-checkbox v-model="form.use_tls">使用 StartTLS 加密连接</el-checkbox>
              <el-checkbox v-model="form.remember">记住此服务器</el-checkbox>
            </div>
          </div>

          <!-- 提交 -->
          <el-button
            type="primary"
            class="btn-submit"
            :loading="loading"
            @click="handleLogin"
          >
            <span>连接到目录服务器</span>
            <el-icon class="btn-arrow"><ArrowRight /></el-icon>
          </el-button>
        </el-form>

        <!-- 底部 -->
        <footer class="form-foot">
          <el-icon :size="12"><Connection /></el-icon>
          <span>使用 LDAP v3 协议 · 连接信息仅保存在本地</span>
        </footer>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
/**
 * Login —— LDAP 登录页
 * 双栏 Flex 布局 + label-position="top" 让 label 自然左对齐
 * 表单分三组：快速连接 / 手动填写 / 认证信息
 */
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Lock, ArrowRight, Connection } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'
import { useServerStore } from '@/store/server'
import { getServers } from '@/api/server'
import type { LoginRequest, LdapServer } from '@/types'

const router = useRouter()
const authStore = useAuthStore()
const serverStore = useServerStore()

const formRef = ref<FormInstance>()    // 表单实例引用，用于触发整体校验
const loading = ref(false)             // 登录请求进行中，控制提交按钮 loading 态
const servers = ref<LdapServer[]>([])  // 后端已保存的服务器配置列表

// 表单数据
const form = ref<LoginRequest>({
  server_id: undefined,
  host: '', port: 389, base_dn: '',
  bind_dn: '', password: '',
  use_tls: false, remember: false,
})

// 校验规则（中文）
const rules: FormRules = {
  bind_dn:  [{ required: true, message: '请输入绑定 DN', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

/** 页面挂载后加载已保存的服务器列表；有则默认选中第一个并自动填充连接参数 */
onMounted(async () => {
  try {
    servers.value = await getServers()
    // 有保存的连接 → 默认选中第一个并自动填充连接参数
    if (servers.value.length > 0) {
      onServerChange(servers.value[0].id)
    }
    // 无保存的连接 → server_id 保持空，表单进入手动填写模式
  } catch { /* 忽略 */ }
})

/**
 * 选择服务器 → 自动填充连接参数（host/port/base_dn/bind_dn/TLS）
 * @param id 选中的服务器配置 ID
 * @returns 无返回值
 */
function onServerChange(id: number) {
  const s = servers.value.find(s => s.id === id)
  if (s) {
    form.value.server_id = s.id
    form.value.host      = s.host
    form.value.port      = s.port
    form.value.base_dn   = s.base_dn
    form.value.bind_dn   = s.bind_dn   // 补上：DB 存了 bind_dn，自动带出
    form.value.use_tls   = s.use_tls
    form.value.password  = ''          // 强制重新输入（密码不存 DB）
  }
}

/**
 * 校验并提交登录请求（LDAP bind DN + 密码认证），成功后记录当前服务器并跳转首页
 * @returns 无返回值
 */
async function handleLogin() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      const res = await authStore.login(form.value)
      // 将本次登录的服务器设为全局当前服务器，供浏览/搜索等页面共用
      serverStore.setCurrentServer(res.server?.id || 0)
      ElMessage.success('连接成功')
      router.push('/')
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped lang="scss">
/*
 * 布局骨架（Flex 代替 Grid，避免固定宽度试探）
 * - 父容器 display:flex + stretch 让两列自然等高
 * - 左右各 flex:1 占一半，但各自内容用 max-width 约束
 * - 不再用 width 固定数值，而是让 flex-grow 自然分配
 */

.login-shell {
  display: flex;
  height: 100vh;
  // 关键：允许溢出滚动，不再 overflow:hidden 裁掉按钮
  overflow: auto;
}

/* ===== 左侧品牌（flex:1 自然占 50%，内容限 max-width） ===== */
.brand {
  flex: 1;
  background: var(--surface-page);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  padding: 64px 72px;
  position: relative;

  // 左侧装饰光晕
  &::before {
    content: '';
    position: absolute;
    top: -120px; left: -120px;
    width: 400px; height: 400px;
    border-radius: 50%;
    background: radial-gradient(circle, var(--accent-primary-soft) 0%, transparent 70%);
    pointer-events: none;
  }
}

// Logo
.brand-logo {
  display: flex; align-items: center; gap: 12px;
  position: relative; z-index: 1;

  .logo-text {
    font-size: 15px; font-weight: 700;
    letter-spacing: -0.005em;
  }
}

// 标题 + 简介
.brand-copy {
  position: relative; z-index: 1;
  margin-top: 96px;
  max-width: 480px;

  .brand-title {
    font-size: clamp(32px, 4vw, 48px);
    line-height: 1.15;
    font-weight: 700;
    letter-spacing: -0.025em;
    color: var(--text-primary);
    margin: 0 0 20px;
  }

  .brand-desc {
    font-size: 15px;
    line-height: 1.7;
    color: var(--text-secondary);
    margin: 0;
  }
}

// 功能列表
.brand-list {
  position: relative; z-index: 1;
  list-style: none;
  padding: 0; margin: 0;
  margin-top: 64px;
  padding-top: 28px;
  border-top: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; gap: 22px;
  max-width: 420px;

  li {
    display: flex; align-items: center; gap: 16px;
  }

  .bl-num {
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 600;
    color: var(--accent-primary);
    background: var(--accent-primary-soft);
    width: 32px; height: 32px;
    border-radius: var(--radius-2);
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }

  .bl-text {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-secondary);
  }
}

// 底部版本
.brand-foot {
  position: relative; z-index: 1;
  margin-top: auto;
  font-size: var(--fs-caption);
  color: var(--text-muted);
  font-family: var(--font-mono);
}

/* ===== 右侧表单（flex:1，也占 50%） ===== */
.form-wrap {
  flex: 1;
  background: var(--surface-base);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  // 压缩 padding 让内容更多露出
  padding: 40px 72px 24px;
  position: relative;
  overflow-y: auto;
}

.form-body {
  width: 100%;
  max-width: 520px;
}

// 头部
.form-head {
  margin-bottom: 20px;

  .head-eyebrow {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    color: var(--accent-primary);
    display: block;
    margin-bottom: 6px;
  }

  .head-title {
    font-size: 26px;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--text-primary);
    margin: 0 0 6px;
  }

  .head-desc {
    font-size: 13px;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

// ===== 表单节奏 —— 紧凑到一屏内 =====
.form-main {
  display: flex;
  flex-direction: column;
  // section 间距：16px
  gap: 16px;

  :deep(.el-form-item) {
    margin-bottom: 10px;  // ← 14→10

    .el-form-item__label {
      font-size: 11px;
      font-weight: 500;
      color: var(--text-secondary);
      text-align: left !important;
      padding-bottom: 3px !important;  // ← 4→3
    }

    .el-form-item__content {
      margin-left: 0 !important;
    }

    .el-input__wrapper,
    .el-select__wrapper {
      min-height: 38px !important;  // ← 42→38 一屏关键！
      border-radius: var(--radius-2) !important;
      box-shadow: 0 0 0 1px var(--border-default) inset !important;
      background: var(--surface-panel) !important;

      &:hover { box-shadow: 0 0 0 1px var(--border-strong) inset !important; }
      &.is-focus {
        box-shadow: 0 0 0 2px var(--accent-primary) inset,
                    0 0 0 4px var(--accent-primary-ring) !important;
      }

      .el-input__inner { font-size: 14px; }
    }
  }
}

// 双列并排（主机地址 + 基准 DN 一行）
.two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;

  // 双列内 form-item 也保持紧凑
  :deep(.el-form-item) { margin-bottom: 10px; }
}

// 每个 section 的分组标题
.form-sec {
  display: flex; flex-direction: column;
}

.sec-title {
  display: flex; align-items: center; gap: 10px;
  margin-bottom: 8px;  // ← 10→8
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);

  .sec-num {
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    color: var(--accent-primary);
    background: var(--accent-primary-soft);
    width: 20px; height: 20px;
    border-radius: var(--radius-1);
    display: flex; align-items: center; justify-content: center;
  }

  .sec-rule {
    flex: 1; height: 1px;
    background: var(--border-subtle);
  }
}

// 复选框
.form-checks {
  display: flex;
  gap: 24px;
  margin-top: 2px;
  padding: 0;
}

.prefix-ic { color: var(--text-muted); }

.full { width: 100%; }

.svr-opt {
  display: flex; flex-direction: column; gap: 2px;
  font-weight: 500;
  .svr-addr { font-size: var(--fs-caption); color: var(--text-muted); }
}

// ===== 提交按钮 =====
.btn-submit {
  width: 100% !important;
  height: 42px !important;  // ← 46→42
  margin-top: 4px;
  font-size: 14px !important;
  font-weight: 600 !important;
  border-radius: var(--radius-2) !important;
  display: flex; align-items: center; justify-content: center; gap: 8px;
  box-shadow: var(--elevation-2) !important;
  transition: all 0.18s ease !important;

  &:hover:not(.is-disabled) {
    box-shadow: var(--elevation-3) !important;
    transform: translateY(-1px);
  }
  &:active:not(.is-disabled) { transform: translateY(0); }
}

.btn-arrow { transition: transform 0.15s ease; }
.btn-submit:hover .btn-arrow { transform: translateX(2px); }

// ===== 底部 =====
.form-foot {
  margin-top: 14px;  // ← 20→14
  display: flex; align-items: center; justify-content: center; gap: 6px;
  font-size: var(--fs-caption);
  color: var(--text-muted);
}

// ===== 响应式 =====
@media (max-width: 900px) {
  .login-shell { flex-direction: column; overflow: auto; }
  .brand {
    padding: 32px 24px;
    border-right: none;
    border-bottom: 1px solid var(--border-subtle);
  }
  .brand-copy { margin-top: 32px; }
  .brand-list { display: none; }
  .form-wrap { padding: 32px 24px; }
  .form-body { min-width: auto; }
}
</style>
