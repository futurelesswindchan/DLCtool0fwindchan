/**
 * 界面反馈状态
 *
 * 收纳 Toast 队列与顶部横幅。任意组件、任意深度均可触发反馈，
 * 若逐层透传事件会让中间组件承担与自身无关的职责。
 *
 * 确认弹窗不在此处——它需要 Promise 语义，见 composables/useConfirm.ts。
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'info' | 'success' | 'warn' | 'error'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

/** 顶部横幅。与 Toast 的区别是不自动消失，用于需持续可见的状态。 */
export interface Banner {
  kind: ToastKind
  message: string
  /** 可选的行动按钮文案，为空则只展示信息 */
  actionText?: string
  /** 点击行动按钮的回调 */
  onAction?: () => void
}

/** Toast 自动消失时长。错误类停留更久，因为用户需要时间读完原因。 */
const DURATION: Record<ToastKind, number> = {
  info: 2600,
  success: 2600,
  warn: 4000,
  error: 6000,
}

let seq = 0

/** 窗口宽度低于此值时侧栏自动折叠为图标条。与 main.go 的 MinWidth 对齐考虑。 */
const SIDEBAR_COLLAPSE_AT = 900

export const useUiStore = defineStore('ui', () => {
  const toasts = ref<Toast[]>([])
  const banner = ref<Banner | null>(null)

  /**
   * 侧栏折叠态。跨三页共享，故不放各 Shell 内部——否则从首页切到已安装页
   * 会看到侧栏宽度跳一下。
   *
   * 不做「自动隐藏」：侧栏承载导航，完全消失会让用户失去方位（宪法 3.8）。
   * 窄窗口只折叠为图标条，且用户可手动展开覆盖。
   */
  const sidebarCollapsed = ref(window.innerWidth < SIDEBAR_COLLAPSE_AT)

  /**
   * 用户是否手动干预过折叠态。
   *
   * 一旦手动操作过，窗口 resize 就不再自动改写它——否则用户在窄窗口下
   * 特意展开侧栏，随手拖动一下窗口就被收回去，那比不提供这个开关更烦。
   */
  const sidebarPinned = ref(false)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
    sidebarPinned.value = true
  }

  /** 由 AppShell 在 resize 时调用。已手动干预过则不再自动改写。 */
  function syncSidebarToWidth(width: number) {
    if (sidebarPinned.value) return
    sidebarCollapsed.value = width < SIDEBAR_COLLAPSE_AT
  }

  function toast(message: string, kind: ToastKind = 'info') {
    const id = ++seq
    toasts.value.push({ id, kind, message })
    window.setTimeout(() => dismiss(id), DURATION[kind])
  }

  function dismiss(id: number) {
    const i = toasts.value.findIndex((t) => t.id === id)
    if (i !== -1) toasts.value.splice(i, 1)
  }

  function showBanner(b: Banner) {
    banner.value = b
  }

  function clearBanner() {
    banner.value = null
  }

  return {
    toasts,
    banner,
    toast,
    dismiss,
    showBanner,
    clearBanner,
    sidebarCollapsed,
    sidebarPinned,
    toggleSidebar,
    syncSidebarToWidth,
  }
})
