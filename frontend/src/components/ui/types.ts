/**
 * 原语的公共契约类型
 *
 * 为什么不写在各自的 .vue 里：`<script setup>` 中的 `export interface`
 * 无法被别处再导出（vue-tsc 报 TS2459），故凡是调用方需要引用的类型
 * 都得住在真正的 .ts 文件里。
 *
 * 这个限制反而指向了更好的结构——**契约与实现分离**：
 * 调用方只需 import 类型即可构造数据，不必知道是哪个组件在消费它。
 */

/** UiSelect 的选项 */
export interface SelectOption {
  label: string;
  value: string | number;
  /**
   * 次要说明，显示在标签右侧。
   * 用于「源名 + 形态标注」这类场景，如「MAU · github-branch」。
   */
  hint?: string;
  disabled?: boolean;
}

/** UiSegmented 的选项。刻意不支持 hint——分段控件一挤就读不清 */
export interface SegmentedOption {
  label: string;
  value: string | number;
}

/**
 * UiIcon 支持的图标名。
 *
 * 收成联合类型而非放开 string：拼错图标名会渲染出一个空 svg，
 * 而空 svg 不报错也不留痕——只有类型能在编译期抓住它。
 *
 * 加图标要两处同改：本类型 + UiIcon.vue 的模板分支。
 */
export type IconName =
  | "search"
  | "package"
  | "library"
  | "chart"
  | "plug"
  | "globe"
  | "palette"
  | "info"
  | "warn"
  | "star"
  | "filter"
  | "gear";
