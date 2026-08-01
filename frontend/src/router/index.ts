/**
 * 路由表
 *
 * 使用 hash 模式：Wails 经自定义协议提供页面，history 模式在部分情形下
 * 刷新会 404。
 *
 * 全站扁平，无嵌套路由。/game/:appID 一个路由承担「未入库」与「已入库」
 * 两种状态，由组件内部据本地是否已有该游戏的清单包分支渲染——拆成两个
 * 路由的话，入库成功后需跳转，过渡动画与返回逻辑都会变复杂。
 */

import { createRouter, createWebHashHistory } from 'vue-router'
import { useEnvStore } from '../stores/env'

const routes = [
  { path: '/', name: 'search', component: () => import('../views/SearchView.vue') },
  { path: '/game/:appID', name: 'game', component: () => import('../views/GameView.vue') },
  { path: '/library', name: 'library', component: () => import('../views/LibraryView.vue') },
  { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
  { path: '/setup', name: 'setup', component: () => import('../views/SetupView.vue') },
]

/**
 * 原语预览页，仅开发构建注册。
 *
 * 它的作用不只是「看一眼」：第 2 步不改任何页面，若无此页则十三个原语
 * 全部无人引用、被 tree-shake 掉——构建通过但一行都没真正跑过。
 *
 * import.meta.env.DEV 由 Vite 静态替换，生产构建下整个分支连同
 * 那句 import 一起被摇掉，不会进封测包。
 *
 * 访问路径：开发模式下打开 #/dev/ui
 */
if (import.meta.env.DEV) {
  routes.push({
    path: '/dev/ui',
    name: 'dev-ui',
    component: () => import('../views/UiGalleryView.vue'),
  })
}

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

/**
 * 环境未就绪时引导至 /setup。
 *
 * 两处例外不拦截：
 *   - 目标本身是 /setup 或 /settings（否则用户无从修好环境）
 *   - 检测尚未完成（envStore.checked 为 false），此时状态未知，
 *     贸然重定向会让首屏闪一下引导页
 *
 * 仅在应用启动后的首次导航拦截。用户主动离开引导页视为知情选择，
 * 不反复弹回——否则界面会变成无法逃离的死循环。
 */
let guarded = false

router.beforeEach((to) => {
  if (guarded) return true
  if (to.name === 'setup' || to.name === 'settings') return true
  // 预览页与环境状态无关，被引导拦走反而没法验证原语
  if (to.name === 'dev-ui') return true

  const env = useEnvStore()
  if (!env.checked) return true

  guarded = true
  if (!env.ready) return { name: 'setup' }
  return true
})
