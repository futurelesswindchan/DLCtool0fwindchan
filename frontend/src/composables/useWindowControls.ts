/**
 * 无边框窗口的控制与状态
 *
 * frameless 模式下系统标题栏消失，最小化 / 最大化 / 关闭三个动作需由前端
 * 自行绘制并调用 runtime API。本 composable 收拢这三个动作与「当前是否已
 * 最大化」这一状态。
 *
 * NOTE: 必须在组件 setup 的同步作用域内调用——内部注册了 onUnmounted，
 * 若在异步回调或 watch 内构造，清理逻辑会挂不到组件实例上（此坑已在
 * useDlcSelection 上踩过一次）。
 */

import { onMounted, onUnmounted, ref } from 'vue'
import {
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
} from '../../wailsjs/runtime/runtime'

export function useWindowControls() {
  /** 窗口当前是否处于最大化状态，决定按钮图标与圆角的呈现。 */
  const maximised = ref(false)

  /**
   * 同步最大化状态。
   *
   * 之所以不能只在点击按钮时翻转本地布尔值：Aero Snap（拖到屏幕顶端）与
   * 系统快捷键 Win+↑ 同样会改变窗口状态，此时前端收不到任何点击事件。
   * 一旦本地状态与真实状态错位，圆角与图标就会跟窗口形态相反。
   */
  async function sync() {
    try {
      maximised.value = await WindowIsMaximised()
    } catch {
      // 状态查询失败只影响图标呈现，不值得打扰用户
    }
  }

  /**
   * 以 resize 事件驱动状态同步。
   *
   * 相比定时轮询，resize 能覆盖全部改变窗口尺寸的途径（按钮、Aero Snap、
   * 快捷键、边缘拖拽），且静止时零开销。
   *
   * NOTE: WindowIsMaximised 是跨边界的异步调用，resize 在拖拽缩放期间会
   * 高频触发，故加 120ms 防抖。缺了防抖会在缩放窗口时打出上百次 IPC。
   */
  let timer: number | undefined

  function onResize() {
    if (timer !== undefined) window.clearTimeout(timer)
    timer = window.setTimeout(sync, 120)
  }

  onMounted(() => {
    void sync()
    window.addEventListener('resize', onResize)
  })

  onUnmounted(() => {
    if (timer !== undefined) window.clearTimeout(timer)
    window.removeEventListener('resize', onResize)
  })

  function minimise() {
    WindowMinimise()
  }

  /**
   * 切换最大化。
   *
   * 立即翻转本地状态而不等 sync：IPC 往返有可见延迟，等结果回来再改图标
   * 会让按钮显得迟钝。resize 触发的 sync 随后会纠正任何偏差。
   */
  function toggleMaximise() {
    WindowToggleMaximise()
    maximised.value = !maximised.value
  }

  /**
   * 退出应用。
   *
   * 直接调 Quit 而不做二次确认：本工具的所有操作都已即时落盘，关闭窗口
   * 不会丢失任何东西，多一层确认只是徒增摩擦。
   */
  function quit() {
    Quit()
  }

  return { maximised, minimise, toggleMaximise, quit }
}
