<script setup lang="ts">
/**
 * 应用根
 *
 * 第 3 步起职责收窄为三件：启动时拉取一次后端状态、挂载全局反馈宿主、
 * 决定当前路由要不要套三板斧外壳。骨架长什么样归 `layout/AppShell`。
 *
 * 启动顺序有意为之：配置先于环境检测——主题取自配置，先落主题可避免
 * 首屏以默认色渲染后再跳成另一套。
 */

import { onMounted, onUnmounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { watchSystemTheme } from './styles/theme-boot'
import { useEnvStore } from './stores/env'
import { useConfigStore } from './stores/config'
import { useLibraryStore } from './stores/library'
import { useUiStore } from './stores/ui'
import { getMSiteStats } from './api'
import AppShell from './components/layout/AppShell.vue'
import ToastHost from './components/ToastHost.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'

const env = useEnvStore()
const config = useConfigStore()
const library = useLibraryStore()
const ui = useUiStore()
const router = useRouter()
const route = useRoute()

/**
 * 独立全屏的路由：不套顶栏与三板斧。
 *
 * 引导页是一次性流程，侧栏与页签在此只会分散注意（宪法 3.2）；
 * 预览页要验的是原语本身，套壳反而多一层干扰。
 *
 * 判据用路由名而非路径：路径在嵌套改造中会变，名字是稳定契约。
 */
const FULLSCREEN_ROUTES = new Set(['setup', 'dev-ui'])
const fullscreen = computed(() => FULLSCREEN_ROUTES.has(String(route.name)))

/**
 * 跟随系统档下，运行期间系统主题变化需即时生效。
 *
 * 不接这个监听的话，「跟随系统」只在启动那一刻跟随一次，之后要重启才更新——
 * 而 Windows 会在日落时自动切换主题，用户恰好开着工具时就会看到
 * 系统变了而工具没变，那比不提供这个选项更让人困惑。
 *
 * 监听必须在 setup 同步作用域构造（含清理的 composable 的既有约束），
 * 故不能挪进 onMounted。
 */
const unwatchSystemTheme = watchSystemTheme(() => config.theme === 'system')
onUnmounted(unwatchSystemTheme)

/**
 * 侧栏折叠态随窗口宽度同步。
 *
 * 必须走 resize 事件而非 CSS 媒体查询：折叠态是 store 状态，
 * 侧栏内部多个组件（SidebarItem 的文字、SidebarSection 的标题）都要读它，
 * 纯 CSS 做不到「隐藏文字的同时也不渲染它」。
 *
 * 防抖 120ms：与 TopBar 里最大化状态同步用的是同一个量级，
 * Aero Snap 拖拽期间会连续触发 resize。
 */
let resizeTimer = 0
function onResize() {
  window.clearTimeout(resizeTimer)
  resizeTimer = window.setTimeout(
    () => ui.syncSidebarToWidth(window.innerWidth),
    120,
  )
}
window.addEventListener('resize', onResize)
onUnmounted(() => {
  window.clearTimeout(resizeTimer)
  window.removeEventListener('resize', onResize)
})

onMounted(async () => {
  await config.refresh()
  await env.refresh()
  await library.refresh()

  // 环境检测在首次导航之后才完成，路由守卫此时已放行，故在此补一次判定。
  // 只在用户仍停留在首页时跳转，避免打断已经开始的操作。
  if (!env.ready && router.currentRoute.value.name === 'search') {
    router.replace({ name: 'setup' })
  }

  await checkMSiteExpiry()
})

/**
 * M 站凭据到期提醒。
 *
 * 凭据默认仅 7 天有效，到期后整条在线获取链路会静默跳过该源——用户很难
 * 自行察觉，故以横幅主动告知。未配置凭据时后端返回 null，此时无需提醒。
 */
async function checkMSiteExpiry() {
  try {
    const stats = await getMSiteStats()
    if (!stats) return

    if (stats.expiringSoon) {
      ui.showBanner({
        kind: 'warn',
        message: `Hubcap Manifest 凭据即将到期（${stats.expiresAt.slice(0, 10)}），到期后该源将无法使用`,
        actionText: '前往设置',
        onAction: () => router.push({ name: 'settings' }),
      })
    } else if (!stats.canMakeRequests) {
      ui.showBanner({
        kind: 'warn',
        message: `Hubcap Manifest 今日额度已用完（${stats.dailyUsage}/${stats.dailyLimit}），明日恢复`,
      })
    }
  } catch {
    // 额度查询失败不影响主流程，静默处置——弹错只会让用户困惑
  }
}
</script>

<template>
  <!--
    两条渲染路径。
    独立全屏那条直接放 RouterView，不加过渡——引导页与预览页都是「到了就
    停在那」的终点，进出时再来一段位移动效只是噪音。
  -->
  <RouterView v-if="fullscreen" />

  <!--
    三板斧那条。Shell 自己渲染侧栏与 PaneTransition，
    故这里的 RouterView 出口是「哪个壳」，不是「哪个 Pane」。
  -->
  <AppShell v-else>
    <RouterView />
  </AppShell>

  <ToastHost />
  <ConfirmDialog />
</template>
