import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import './styles/index.css'
import { bootTheme } from './styles/theme-boot'
import { installFileDropGuard } from './guards/fileDropGuard'

/**
 * 主题必须在挂载前落地。
 *
 * 主题的权威值在后端配置里，而拉取是异步的——若等到 config.refresh()
 * 完成才应用，凡是选了非默认主题的用户每次启动都会看到一次闪色。
 * bootTheme 从 localStorage 同步读出上次生效的档位先行落地，
 * 配置拉回后若不一致再覆盖（只发生在换设备或清缓存之后）。
 */
bootTheme()

/**
 * 拖放防线要早于挂载装上。
 *
 * 挂载之后才装会留下一个窗口期——启动瞬间用户就能拖文件进来，而那正是
 * WebView2 崩溃路径的入口。这类兜底没有「概率很低所以无所谓」的说法：
 * 它的代价是整个进程倒掉。
 */
installFileDropGuard()

createApp(App).use(createPinia()).use(router).mount('#app')
