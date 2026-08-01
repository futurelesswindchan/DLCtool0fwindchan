import { ref, type Ref } from 'vue'

/**
 * 把浮层定位到锚点元素下方，空间不足时向上翻转。
 *
 * 为什么不引 floating-ui：项目已定「不引 UI 组件库」，理由是可复用的仅按钮
 * 与弹窗，却会把打包体积从约 100KB 推到 1MB 以上。浮层场景当前只有下拉与
 * Tooltip 两个，10KB 依赖换这几十行不划算。
 *
 * ⚠️ 但需诚实记录：若日后浮层场景涨到五六个（尤其出现需要箭头指向、
 *    自动选择四向、虚拟锚点等需求时），这个判断应当推翻。
 *    自己维护一套定位逻辑的成本是隐性的，会在边界情形上慢慢渗出来。
 *
 * 用 position: fixed 而非 absolute：absolute 会被任何祖先的 overflow: hidden
 * 裁掉，而本项目的侧栏与列表容器都有滚动区。fixed 相对视口定位，天然免疫。
 *
 * 本 composable 不注册 onUnmounted，清理责任交给调用方（bind / unbind 成对调用）。
 * 理由：浮层生命周期通常短于组件，等到组件卸载才清理会在浮层反复开合时
 * 累积悬空监听。另注：项目已知「含 onUnmounted 的 composable 必须在 setup
 * 同步作用域构造」，不注册也就绕开了这条限制，可在事件回调里随时构造。
 */

export interface AnchoredLayerOptions {
  /** 浮层与锚点的间距，默认 4px */
  gap?: number
  /** 距窗口边缘的最小留白，默认 8px */
  margin?: number
  /** 浮层宽度是否跟随锚点。下拉框需要，Tooltip 不需要 */
  matchWidth?: boolean
}

export interface LayerStyle {
  position: 'fixed'
  top: string
  left: string
  minWidth?: string
  /** 向上翻转时为 true，供调用方切换入场动画方向 */
  flipped: boolean
}

export function useAnchoredLayer(options: AnchoredLayerOptions = {}) {
  const { gap = 4, margin = 8, matchWidth = false } = options

  const style = ref<LayerStyle>({
    position: 'fixed',
    top: '0px',
    left: '0px',
    flipped: false,
  })

  /**
   * 按锚点与浮层的实测尺寸重算位置。
   *
   * 必须在浮层已挂载且可测量之后调用——浮层若仍是 display: none，
   * getBoundingClientRect 全返回 0，翻转判断会失效。
   */
  function update(anchor: HTMLElement | null, layer: HTMLElement | null) {
    if (!anchor || !layer) return

    const a = anchor.getBoundingClientRect()
    const l = layer.getBoundingClientRect()

    const spaceBelow = window.innerHeight - a.bottom - margin
    // 下方装不下、且上方更宽敞时才翻转。
    // 加「上方更宽敞」这个条件是因为：两边都装不下时，往下展开至少符合预期，
    // 翻上去反而会让用户以为点错了地方。
    const spaceAbove = a.top - margin
    const flipped = l.height > spaceBelow && spaceAbove > spaceBelow

    const top = flipped ? a.top - l.height - gap : a.bottom + gap

    // 水平方向左对齐锚点，右侧溢出时贴右边缘回收
    let left = a.left
    if (left + l.width > window.innerWidth - margin) {
      left = window.innerWidth - margin - l.width
    }
    if (left < margin) left = margin

    style.value = {
      position: 'fixed',
      top: `${Math.round(top)}px`,
      left: `${Math.round(left)}px`,
      minWidth: matchWidth ? `${Math.round(a.width)}px` : undefined,
      flipped,
    }
  }

  /**
   * 绑定「跟随重算」的监听。
   *
   * 滚动用捕获阶段监听：滚动事件不冒泡，若只在 window 上监听，
   * 内层滚动容器的滚动不会被捕捉到，浮层就会与锚点脱开。
   *
   * resize 同时覆盖窗口缩放与最大化。项目已知 Aero Snap 不产生点击事件，
   * 必须靠 resize 同步，此处同理。
   */
  function bind(recompute: () => void) {
    window.addEventListener('scroll', recompute, true)
    window.addEventListener('resize', recompute)
  }

  function unbind(recompute: () => void) {
    window.removeEventListener('scroll', recompute, true)
    window.removeEventListener('resize', recompute)
  }

  return { style: style as Ref<LayerStyle>, update, bind, unbind }
}

