/**
 * 应用入口
 *
 * 初始化 Vue 应用并注册全局插件：
 * - Pinia：状态管理
 * - Vue Router：路由
 * - Element Plus：UI 组件库（中文语言包）
 */
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

import App from './App.vue'
import router from './router'
import './styles/global.scss'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

app.mount('#app')
