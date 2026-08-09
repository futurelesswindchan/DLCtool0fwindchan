<script setup lang="ts">
/**
 * 装饰花纹
 *
 * ⚠️ 为什么是内联 SVG 而不是 mask-image：
 *    WebView2 对 mask-image + 本地 SVG URL 的组合不生效（与 LogoMark 同病）。
 *    LogoMark 切换方案时已确认根因——mask URL 在整个 WebView2 环境加载失败。
 *    故本组件同样弃用 mask，改为内联 SVG + stroke="currentColor"，
 *    颜色由 --pattern-ink 令牌经 color 属性给。
 *
 * 一份 SVG 资产吃两套主题：SVG 路径本身不含颜色，currentColor 由外部
 * CSS 变量决定。换主题、调浓淡都不用碰图形数据。
 *
 * ⚠️ 每屏最多一处装饰主体（宪法 7.1 节）。侧栏有了角落纹样，
 *    内容区就不再放。这条无法由组件强制，只能在 review 时看。
 *
 * 禁区（不得投放）：对比表、DLC 列表、等宽字体区、警示横幅、顶栏。
 */

import { computed } from 'vue'

interface Props {
  /** 图案选择 */
  pattern: 'beans' | 'ear'
  /**
   * 角色决定尺寸与定位策略（宪法 7.2 节）：
   *   tile    无缝微平铺，铺满容器
   *   corner  角落纹样，从边角切出画面外，只露一部分
   *   divider 分隔纹，替代 1px 直线（暂未实现）
   */
  role?: 'tile' | 'corner' | 'divider'
  /** corner 模式下 SVG 的显示尺寸 */
  size?: string
  /**
   * 浓度覆盖。留空用令牌默认值。
   * 上限 8%（分隔纹除外）——
   * 判定标准：截图缩到 25% 还能看出「这里有花纹」，就是太浓了。
   */
  alpha?: number
  /** 角落纹样的贴靠方位 */
  corner?: 'tl' | 'tr' | 'bl' | 'br'
}

const props = withDefaults(defineProps<Props>(), {
  role: 'tile',
  corner: 'br',
})

const style = computed(() => {
  const s: Record<string, string> = {}
  if (props.alpha != null) s.opacity = String(props.alpha)
  if (props.size) {
    s.width = props.size
    s.height = props.size
  }
  return s
})
</script>

<template>
  <!--
    外壳 span：只承载颜色与定位。SVG 描边走 currentColor，颜色由 span 的
    color 属性注入——它拿的是 --pattern-ink（浅/深双套，令牌已定义）。
  -->
  <span
    class="orn"
    :class="[`orn--${role}`, role === 'corner' && `orn--${corner}`]"
    :style="style"
    aria-hidden="true"
  >
    <!-- ─── 咖啡豆微平铺 ─── -->
    <svg
      v-if="pattern === 'beans'"
      class="orn__svg orn__svg--tile"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <defs>
        <!--
          patternUnits="userSpaceOnUse"：用 viewBox 坐标而非百分比，
          与原始 beans.svg 的 48×48 栅格直接对应，无需换算。
          ⚠️ id 取 pattern="beans"，不设随机后缀——单页复用不是问题，
              因为 SVG <pattern> 的 id 只在引用它的 <rect> 所在 SVG 子树里可见，
              不污染全局。
        -->
        <pattern id="orn-beans" width="48" height="48" patternUnits="userSpaceOnUse">
          <g transform="rotate(-20 13 13)">
            <ellipse cx="13" cy="13" rx="4.2" ry="6"/>
            <path d="M13 7.6 C11.6 10 14.4 12 13 14.4 C11.8 16.4 13 17.6 13 18.4"/>
          </g>
          <g transform="rotate(28 34 33)">
            <ellipse cx="34" cy="33" rx="4.2" ry="6"/>
            <path d="M34 27.6 C32.6 30 35.4 32 34 34.4 C32.8 36.4 34 37.6 34 38.4"/>
          </g>
          <rect x="34.5" y="9.5" width="7" height="7" rx="2"/>
          <rect x="8.5" y="33.5" width="5" height="5" rx="2"/>
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#orn-beans)" />
    </svg>

    <!-- ─── 兔耳角落纹样 ─── -->
    <svg
      v-else-if="pattern === 'ear'"
      class="orn__svg orn__svg--corner"
      viewBox="0 0 96 96"
      fill="none"
      stroke="currentColor"
      stroke-width="6"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M30 78 C22 66 20 46 26 32 C29 25 34 22 38 26 C43 31 44 52 41 66 C39.5 73 36 78 33 79 Z"/>
      <path d="M62 76 C56 64 55 44 60 31 C63 24 68 22 71 27 C75 33 74 53 70 65 C68 71 65 76 63 77 Z"/>
    </svg>
  </span>
</template>

<style scoped>
.orn {
  position: absolute;
  /* 必须。否则装饰会挡住底下的点击，而它完全不承载信息 */
  pointer-events: none;

  /* 颜色由令牌给，SVG 描边走 currentColor，故在此注入 color */
  color: var(--pattern-ink);
  /* 透明度同样走令牌，但调用方可经 :alpha 覆盖 */
  opacity: var(--pattern-alpha);
}

.orn__svg {
  display: block;
}

/* 微平铺：铺满容器 */
.orn--tile {
  inset: 0;
}

.orn__svg--tile {
  width: 100%;
  height: 100%;
}

/* 角落纹样：从边角切出画面外。
   露出局部比完整摆中间高级得多——它暗示画面之外还有东西。
   故刻意用负偏移让图形被容器裁掉一部分。 */
.orn--corner {
  width: 140px;
  height: 140px;
}

.orn__svg--corner {
  width: 100%;
  height: 100%;
}

.orn--tl {
  top: -36px;
  left: -36px;
}
.orn--tr {
  top: -36px;
  right: -36px;
}
.orn--bl {
  bottom: -36px;
  left: -36px;
}
.orn--br {
  bottom: -36px;
  right: -36px;
}

/* 分隔纹：替代 1px 直线。成本极低但精致感回报最高，
   因为分隔线数量多、出现频繁。故它是唯一允许较高浓度的角色。 */
.orn--divider {
  position: relative;
  display: block;
  width: 100%;
  height: 8px;
  opacity: 0.08;
}

/* 静态花纹永不动画（宪法 7.6 节）。
   唯一例外是等待态意象，那由业务侧单独实现，不走本组件。 */
</style>
