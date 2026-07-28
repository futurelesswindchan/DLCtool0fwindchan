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

  const env = useEnvStore()
  if (!env.checked) return true

  guarded = true
  if (!env.ready) return { name: 'setup' }
  return true
})
