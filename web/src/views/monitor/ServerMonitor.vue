<template>
  <div class="page">
    <div class="page-header">
      <h2>服务器监控</h2>
      <el-button @click="loadStats" :loading="loading">
        <el-icon><Refresh /></el-icon>刷新
      </el-button>
    </div>

    <!-- 核心指标卡片：连接状态 / 活跃连接 / 总操作数 / 平均响应 -->
    <el-row :gutter="16" class="stats-row">
      <el-col :span="6">
        <el-card class="stat-card" :class="{ online: stats?.connected }">
          <div class="stat-label">连接状态</div>
          <div class="stat-value">
            <el-tag :type="stats?.connected ? 'success' : 'danger'">
              {{ stats?.connected ? '已连接' : '未连接' }}
            </el-tag>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-label">活跃连接</div>
          <div class="stat-value">{{ stats?.active_connections || 0 }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-label">总操作数</div>
          <div class="stat-value">{{ stats?.total_operations || 0 }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-label">平均响应</div>
          <div class="stat-value">{{ stats?.avg_response_ms || 0 }} ms</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表区：ECharts 渲染，数据由定时刷新驱动 -->
    <el-row :gutter="16" class="chart-row">
      <el-col :span="12">
        <el-card header="操作类型分布">
          <div ref="typeChartRef" class="chart" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card header="响应时间趋势">
          <div ref="timeChartRef" class="chart" />
        </el-card>
      </el-col>
    </el-row>

    <el-card header="详细统计" style="margin-top: 16px">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="成功操作">{{ stats?.success_count || 0 }}</el-descriptions-item>
        <el-descriptions-item label="失败操作">{{ stats?.failed_count || 0 }}</el-descriptions-item>
        <el-descriptions-item label="运行时间">
          {{ formatUptime(stats?.uptime_seconds || 0) }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
/**
 * 服务器监控组件 — 展示 LDAP 服务器的实时运行状态和性能指标
 * 使用 ECharts 绘制操作类型分布饼图和响应时间趋势折线图，
 * 每 5 秒自动刷新数据
 */
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { getMonitorStats } from '@/api/monitor'
import type { MonitorStats } from '@/types'

// ==================== 响应式状态 ====================
const loading = ref(false)                          // 数据加载状态
const stats = ref<MonitorStats | null>(null)        // 监控统计数据
const typeChartRef = ref<HTMLElement>()             // 操作类型饼图容器
const timeChartRef = ref<HTMLElement>()             // 响应时间折线图容器
let typeChart: echarts.ECharts | null = null         // 操作类型饼图实例
let timeChart: echarts.ECharts | null = null         // 响应时间折线图实例
let timer: number | undefined                        // 定时刷新定时器句柄

// ==================== 生命周期 ====================
/** 挂载后首次拉取监控数据，并启动 5 秒定时自动刷新 */
onMounted(async () => {
  await loadStats()
  timer = window.setInterval(loadStats, 5000)  // 每 5 秒自动刷新
})

/** 组件卸载前清理定时器与 ECharts 实例，避免内存泄漏 */
onUnmounted(() => {
  if (timer) clearInterval(timer)
  typeChart?.dispose()
  timeChart?.dispose()
})

/**
 * 从后端获取最新的监控统计数据，等待 DOM 更新后重绘图表
 * @returns 无返回值
 */
async function loadStats() {
  loading.value = true
  try {
    stats.value = await getMonitorStats()
    await nextTick()
    renderCharts()
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
}

/**
 * 根据最新统计数据重新渲染 ECharts 图表
 * 首次渲染时惰性 init 实例，后续仅 setOption 增量更新
 * @returns 无返回值
 */
function renderCharts() {
  if (!stats.value) return

  // 操作类型分布饼图
  if (typeChartRef.value) {
    if (!typeChart) typeChart = echarts.init(typeChartRef.value)
    const data = Object.entries(stats.value.operations_by_type || {}).map(([k, v]) => ({ name: k, value: v }))
    typeChart.setOption({
      tooltip: { trigger: 'item' },
      legend: { bottom: 0 },
      series: [{
        type: 'pie',
        radius: ['40%', '70%'],
        data,
        label: { show: true, formatter: '{b}: {c}' },
      }],
    })
  }

  // 响应时间趋势折线图
  if (timeChartRef.value) {
    if (!timeChart) timeChart = echarts.init(timeChartRef.value)
    const times = stats.value.response_times || []
    timeChart.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: times.map((t) => t.time) },
      yAxis: { type: 'value', name: 'ms' },
      series: [{
        type: 'line',
        smooth: true,
        data: times.map((t) => t.avg_ms),
        areaStyle: {},
      }],
    })
  }
}

/**
 * 将服务器运行秒数格式化为可读的 时:分:秒 字符串
 * @param sec 服务运行时长（秒）
 * @returns 形如 "1时23分45秒" 的可读字符串
 */
function formatUptime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  return `${h}时${m}分${s}秒`
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

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
  border-radius: var(--radius-3) !important;

  &.online {
    border-left: 3px solid var(--accent-success) !important;
  }

  .stat-label {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    margin-bottom: 10px;
  }

  .stat-value {
    font-size: 28px;
    font-weight: 700;
    color: var(--text-primary);
    font-family: var(--font-mono);
    letter-spacing: -0.02em;
  }
}

.chart-row {
  margin-bottom: 20px;
}

.chart {
  height: 300px;
}
</style>
