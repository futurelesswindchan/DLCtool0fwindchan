import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import './styles/index.css'
import { bootTheme } from './styles/theme-boot'

/**
 * 主题必须在挂载前落地。
 *
 * 主题的权威值在后端配置里，而拉取是异步的——若等到 config.refresh()
 * 完成才应用，凡是选了非默认主题的用户每次启动都会看到一次闪色。
 * bootTheme 从 localStorage 同步读出上次生效的档位先行落地，
 * 配置拉回后若不一致再覆盖（只发生在换设备或清缓存之后）。
 */
bootTheme()

createApp(App).use(createPinia()).use(router).mount('#app')
