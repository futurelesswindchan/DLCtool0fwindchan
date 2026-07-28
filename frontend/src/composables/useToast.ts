/**
 * Toast 提示
 *
 * 对 uiStore 的薄封装，让组件侧写 toast.success(...) 而非
 * uiStore.toast(..., 'success')，调用点更短。
 */

import { useUiStore } from '../stores/ui'
import { ApiError } from '../api'

export function useToast() {
  const ui = useUiStore()

  return {
    info: (m: string) => ui.toast(m, 'info'),
    success: (m: string) => ui.toast(m, 'success'),
    warn: (m: string) => ui.toast(m, 'warn'),
    error: (m: string) => ui.toast(m, 'error'),

    /**
     * 呈现捕获到的异常。
     *
     * ApiError 携带的是后端给出的面向用户文案，可直接展示；其他异常
     * （网络中断、绑定层报错）文案对用户无意义，故加前缀说明来源，
     * 避免用户误以为是自己操作有误。
     */
    fromError(e: unknown, fallbackPrefix = '操作失败') {
      if (e instanceof ApiError) {
        ui.toast(e.message, 'error')
        return
      }
      const detail = e instanceof Error ? e.message : String(e)
      ui.toast(`${fallbackPrefix}：${detail}`, 'error')
    },
  }
}
