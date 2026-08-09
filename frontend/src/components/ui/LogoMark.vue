<script setup lang="ts">
/**
 * 品牌标识图形
 *
 * 为何不用 mask-image（宪法 7.4 方案）：
 *   WebView2 下 mask-image 的 url() 解析与浏览器存在差异——asset URL 可以
 *   在 Vite 开发服务器正常加载，但在 Wails 嵌入的 WebView2 中失效，导致
 *   遮罩完全不生效、元素退化为 background-color 的实色填充方块。
 *   浏览器与 WebView2 行为不一致且无可行的 polyfill 方案，故弃用 mask。
 *
 *   当前方案：SVG 路径内联到模板，stroke="currentColor"。
 *   与 mask 方案同一份图形数据、同一种配色机制（currentColor 随文字色走），
 *   区别仅在于渲染路径——内联 SVG 是 WebView2 原生支持的，无兼容性风险。
 *
 *   代价：资产不再是独立 .svg 文件（宪法 7.4 要求），而是与组件代码耦合。
 *   日后若要换正式 LOGO，直接替换 template 里的 path 即可。
 *
 * 尺寸经 CSS 控制（默认 1em 随字号走），颜色经 `color` 继承：
 * ```vue
 * <LogoMark style="width: 28px; height: 28px; color: var(--color-accent)" />
 * ```
 */
</script>

<template>
  <svg
    class="logo-mark"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <!-- 左耳。下端收细至耳根 -->
    <path d="M9.2 12.6 C7.6 10 7.4 5.6 8.8 3.4 C9.6 2.2 10.8 2.2 11.4 3.6 C12.3 5.6 12 10 11 12.4 Z"/>
    <!-- 右耳。略外倾，两耳不平行才显得是一对 -->
    <path d="M14.6 12.4 C13.7 10 13.5 5.6 14.5 3.6 C15.2 2.2 16.4 2.3 17 3.6 C18 5.8 17.4 10.2 15.8 12.6 Z"/>
    <!-- 头部下弧。不闭合成圆，留口让两耳与头读作一体 -->
    <path d="M6.6 13.4 C5.4 16.4 6.6 20 9.4 21 C11.2 21.6 13.6 21.5 15.4 20.8 C18 19.8 19 16.2 17.6 13.3"/>
  </svg>
</template>

<style scoped>
.logo-mark {
  display: inline-block;
  /* 默认跟字号走，调用方可用 width/height 覆盖 */
  width: 1em;
  height: 1em;
  /* 垂直对齐：非 flex 上下文下 middle 让 SVG 与同行文字中线对齐 */
  vertical-align: middle;
  /* flex 容器中不参与伸缩 */
  flex: 0 0 auto;
}
</style>
