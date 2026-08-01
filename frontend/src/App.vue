<script setup lang="ts">
/**
 * 应用外壳
 *
 * 只负责三件事：启动时拉取一次后端状态、绘制顶栏与路由出口、挂载全局
 * 反馈宿主（Toast 与确认弹窗）。任何业务逻辑都不放在这里。
 *
 * 启动顺序有意为之：配置先于环境检测——主题取自配置，先落主题可避免
 * 首屏以默认暗色渲染后再跳成亮色。
 */

import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { watchSystemTheme } from './styles/theme-boot'
import { useEnvStore } from './stores/env'
import { useConfigStore } from './stores/config'
import { useLibraryStore } from './stores/library'
import { useUiStore } from './stores/ui'
import { getMSiteStats } from './api'
import TopBar from './components/TopBar.vue'
import ToastHost from './components/ToastHost.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'

const env = useEnvStore()
const config = useConfigStore()
const library = useLibraryStore()
const ui = useUiStore()
const router = useRouter()

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
  <div class="shell">
    <TopBar />

    <div v-if="ui.banner" class="banner" :class="`banner--${ui.banner.kind}`">
      <span class="banner__text">{{ ui.banner.message }}</span>
      <button
        v-if="ui.banner.actionText"
        class="banner__action"
        @click="ui.banner.onAction?.()"
      >
        {{ ui.banner.actionText }}
      </button>
      <button class="banner__close" aria-label="关闭" @click="ui.clearBanner()">
        ✕
      </button>
    </div>

    <main class="content">
      <RouterView v-slot="{ Component }">
        <Transition name="page" mode="out-in">
          <component :is="Component" />
        </Transition>
      </RouterView>
    </main>

    <ToastHost />
    <ConfirmDialog />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.content {
  flex: 1 1 auto;
  overflow-y: auto;
  padding: var(--space-5);
}

.banner {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--color-border);
  font-size: 0.82rem;
  flex: 0 0 auto;
}

.banner--warn {
  background: color-mix(in srgb, var(--color-warning) 16%, var(--color-bg));
  color: var(--color-warning);
}

.banner--error {
  background: color-mix(in srgb, var(--color-danger) 16%, var(--color-bg));
  color: var(--color-danger);
}

.banner--info,
.banner--success {
  background: color-mix(in srgb, var(--color-accent) 14%, var(--color-bg));
  color: var(--color-accent);
}

.banner__text {
  flex: 1 1 auto;
}

.banner__action,
.banner__close {
  border: none;
  background: transparent;
  color: inherit;
  font-family: inherit;
  font-size: inherit;
  cursor: pointer;
}

.banner__action {
  text-decoration: underline;
}

/* 页面切换只做位移与淡入，时长压在快档——过渡不该让用户等 */
.page-enter-active,
.page-leave-active {
  transition: opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}

.page-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.page-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
