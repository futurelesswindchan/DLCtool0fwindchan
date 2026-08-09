/**
 * 逐项错开入场
 *
 * JS 只发序号，延迟在 CSS 里算——
 * `animation-delay: calc(var(--stagger-i) * var(--stagger))`。
 * 这样时长口径仍然唯一（令牌），不会出现某个列表偷偷用了别的步长。
 *
 * ⚠️ 上限硬编码在此，不开放为参数的默认值以外的选择。
 *    若 200 条全参与错开，最后一条要等 4.8 秒才出现——**动效就变成了卡顿**。
 *    这是性能纪律，不是美学选择（宪法 5.1 节）。
 */

export interface StaggerOptions {
  /** 参与错开的最大项数，超出的直接入场。默认 8 */
  max?: number
}

export function useStagger(options: StaggerOptions = {}) {
  const max = options.max ?? 8

  /**
   * 给第 i 项（从 0 起）返回内联 CSS 变量。
   *
   * 超过上限的项统一取 max，于是它们与第 max 项同时入场——
   * 不是不参与动画，而是不再继续往后累加延迟。
   * 这比「超出的直接显示」更平滑，也避免了分界处的突变。
   */
  function styleFor(i: number): Record<string, string> {
    return { '--stagger-i': String(Math.min(i, max)) }
  }

  return { styleFor, max }
}
