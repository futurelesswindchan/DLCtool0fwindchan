/**
 * 路由表
 *
 * 使用 hash 模式：Wails 经自定义协议提供页面，history 模式在部分情形下
 * 刷新会 404。
 *
 * ── 为什么是嵌套的 ──
 *
 * 嵌套不是为了整齐，而是侧栏常驻的**技术前提**（宪法 11.4）。
 * `:appID` 变化时父级 Shell 不重建，于是侧栏保持不动、列表滚动位置不丢、
 * 选中态指示器可以平滑滑移——5.2 节那个动效不需要任何额外机制就成立。
 *
 * ⚠️ 与之配套的硬约束：**不要给 `<RouterView>` 加 `:key`**。
 *    任何随 appID 变化的 key 都会强制重建组件，使 GameView 里那段实机
 *    验证过的 `watch(appID, load)` 永远不触发，且不报任何错。
 *    详见 components/layout/PaneTransition.vue 的注释。
 *
 * ── 游戏详情为何在两棵树下各注册一次 ──
 *
 * 详情不再是独立页面，而是渲染进「用户来的那个列表」的内容区（宪法 3.7）：
 *   从搜索结果进入 -> 侧栏保留搜索入口
 *   从已安装进入   -> 侧栏保留已安装列表
 * 同一个 `GameView` 组件，两个路由名。收益是用户试了一个源不满意，
 * 不必返回就能点侧栏的下一个候选。
 *
 * `/game/:appID` 保留为重定向：它在 07-29 之前的版本里是正式路径，
 * 且 GameView 内部与部分 toast 行动按钮仍以 `name: 'game'` 跳转。
 */

import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useEnvStore } from '../stores/env'

const routes: RouteRecordRaw[] = [
  /**
   * 引导页不套三板斧，保持独立全屏（宪法 3.2）——
   * 它是一次性流程，侧栏在此只会分散注意。
   */
  {
    path: '/setup',
    name: 'setup',
    component: () => import('../views/SetupView.vue'),
  },

  {
    path: '/',
    component: () => import('../views/shells/HomeShell.vue'),
    children: [
      {
        path: '',
        name: 'search',
        component: () => import('../views/SearchView.vue'),
      },
      {
        path: 'import',
        name: 'import',
        component: () => import('../views/panes/ImportPane.vue'),
      },
      {
        path: 'app/:appID',
        name: 'game',
        component: () => import('../views/GameView.vue'),
      },
    ],
  },

  {
    path: '/library',
    component: () => import('../views/shells/LibraryShell.vue'),
    children: [
      {
        path: '',
        name: 'library',
        component: () => import('../views/panes/LibraryOverviewPane.vue'),
      },
      /**
       * 已安装树下的详情。与 `game` 同组件不同名——
       * 两个名字的存在使「从哪来」这件事由路由自身记住，
       * 组件内不必再判断上下文。
       */
      {
        path: ':appID',
        name: 'library-game',
        component: () => import('../views/GameView.vue'),
      },
    ],
  },

  {
    path: '/settings',
    component: () => import('../views/shells/SettingsShell.vue'),
    children: [
      {
        path: '',
        name: 'settings',
        component: () => import('../views/SettingsView.vue'),
      },
    ],
  },

  /**
   * 旧路径兼容。v2 开发期内部曾以此为正式路径，
   * 且改造前的调用点若有遗漏，重定向能让它们继续工作而不是白屏。
   */
  {
    path: '/game/:appID',
    redirect: (to) => ({ name: 'game', params: to.params }),
  },
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
 * 不套三板斧：它要验的是原语本身，套上壳反而多一层干扰。
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
