/**
 * Promise 化的确认弹窗
 *
 * 使调用方能按线性顺序书写决策逻辑：
 *
 *   if (!(await confirm({ title: '...' }))) return
 *   doTheThing()
 *
 * 若改用事件总线或回调，同一段决策会被割裂到两处，取消分支尤其难写。
 *
 * 实现上用模块级单例而非 store：弹窗同时只可能存在一个，且其状态（含
 * resolve 函数）不适合放进 Pinia——函数不可序列化，devtools 会报错。
 */

import { ref, readonly } from 'vue'

export interface ConfirmOptions {
  title: string
  /** 正文，支持多段。数组每项渲染为一个段落 */
  body?: string | string[]
  confirmText?: string
  cancelText?: string
  /** 危险操作：确认按钮改用警示配色 */
  danger?: boolean
}

interface ConfirmState extends ConfirmOptions {
  visible: boolean
}

const state = ref<ConfirmState>({ visible: false, title: '' })
let resolver: ((ok: boolean) => void) | null = null

/** 供 ConfirmDialog 组件读取的只读状态。 */
export const confirmState = readonly(state)

/**
 * 由 ConfirmDialog 调用以关闭弹窗。
 *
 * 按 Esc 或点击遮罩均应传 false——用户以模糊方式退出时，默认取消
 * 而非确认，是不可省的安全默认值。
 */
export function resolveConfirm(ok: boolean) {
  state.value.visible = false
  resolver?.(ok)
  resolver = null
}

export function useConfirm() {
  return (opts: ConfirmOptions): Promise<boolean> => {
    // 前一个弹窗尚未关闭时直接判否，避免 resolver 被覆盖导致上一个
    // Promise 永久挂起。正常交互下不会触发，属防御性处置。
    if (resolver) resolver(false)

    state.value = { ...opts, visible: true }
    return new Promise<boolean>((res) => {
      resolver = res
    })
  }
}
