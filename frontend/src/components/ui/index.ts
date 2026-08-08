/**
 * ui/ 原语统一出口
 *
 * ⚠️ 本层是纯的：不得 import stores、api、wailsjs、router（宪法 11.1 节）。
 *    判定方式——ui/ 下任一组件，复制到另一个项目就能跑。做不到就说明漏了依赖。
 *
 *    这条不是洁癖。等自绘控件一多，就会出现「一个复选框里塞了 store 调用」
 *    这类事，之后想复用就搬不动了。
 *
 * 经此文件统一导出，调用方写
 *   import { UiButton, UiInput } from '@/components/ui'
 * 而不必逐个记路径。
 */

export { default as UiButton } from './UiButton.vue'
export { default as UiCheckbox } from './UiCheckbox.vue'
export { default as UiRadio } from './UiRadio.vue'
export { default as UiSwitch } from './UiSwitch.vue'
export { default as UiInput } from './UiInput.vue'
export { default as UiSelect } from './UiSelect.vue'
export { default as UiSegmented } from './UiSegmented.vue'
export { default as UiProgress } from './UiProgress.vue'
export { default as UiTooltip } from './UiTooltip.vue'
export { default as UiHelpBadge } from './UiHelpBadge.vue'
export { default as UiEmptyState } from './UiEmptyState.vue'
export { default as UiScrollArea } from './UiScrollArea.vue'
export { default as Ornament } from './Ornament.vue'
export { default as LogoMark } from './LogoMark.vue'

export type { SelectOption, SegmentedOption } from './types'
