<script setup lang="ts">
/**
 * 装饰花纹
 *
 * 一份 SVG 资产吃两套主题：用 mask-image 让 SVG 只描述「形」，
 * 颜色与浓度全由令牌给（--pattern-ink / --pattern-alpha）。
 * 换主题、调浓淡都不用碰资产文件——**双主题不会走样，
 * 因为它们本来就是同一个文件**（宪法 7.4 节）。
 *
 * ⚠️ 资产路径由调用方经 src 传入，组件不内置任何 import。
 *    原因：花纹资产第 5 步才投放，组件内 import 不存在的文件会让构建直接失败。
 *    调用方写 `import beans from '@/assets/patterns/beans.svg'` 再传进来。
 *
 * ⚠️ 禁止在花纹中使用 SVG filter、blur、渐变网格（宪法 7.4 节）——
 *    这些会让合成器逐帧重算，在长列表上直接掉帧。
 *    花纹是纯静态的填色形状，一步都不能多。
 *
 * ⚠️ 每屏最多一处装饰主体（宪法 7.1 节）。侧栏有了角落纹样，
 *    内容区就不再放。这条无法由组件强制，只能在 review 时看。
 *
 * 禁区（不得投放）：对比表、DLC 列表、等宽字体区、警示横幅、顶栏。
 */

import { computed } from 'vue'

interface Props {
  /** SVG 资产 URL，由调用方 import 后传入 */
  src: string
  /**
   * 角色决定尺寸与定位策略（宪法 7.2 节四种角色）：
   *   tile    无缝微平铺，48~64px
   *   corner  角落纹样，从边角切出画面外，只露一部分
   *   divider 分隔纹，替代 1px 直线
   */
  role?: 'tile' | 'corner' | 'divider'
  /** 平铺尺寸或纹样尺寸 */
  size?: string
  /**
   * 浓度覆盖。留空用令牌默认值。
   * 上限 8%（分隔纹除外，它本身就该看得见）——
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

const style = computed(() => ({
  maskImage: `url(${props.src})`,
  WebkitMaskImage: `url(${props.src})`,
  maskSize: props.size,
  WebkitMaskSize: props.size,
  opacity: props.alpha,
}))
</script>

<template>
  <span
    class="orn"
    :class="[`orn--${role}`, role === 'corner' && `orn--${corner}`]"
    :style="style"
    aria-hidden="true"
  />
</template>

<style scoped>
.orn {
  position: absolute;
  /* 必须。否则装饰会挡住底下的点击，而它完全不承载信息 */
  pointer-events: none;

  /* 颜色由令牌给，SVG 本身不含颜色 */
  background-color: var(--pattern-ink);
  opacity: var(--pattern-alpha);

  mask-repeat: no-repeat;
  -webkit-mask-repeat: no-repeat;
}

/* 微平铺：铺满容器。元素要小且稀疏，密了立刻变「壁纸」 */
.orn--tile {
  inset: 0;
  mask-repeat: repeat;
  -webkit-mask-repeat: repeat;
  mask-size: 56px;
  -webkit-mask-size: 56px;
}

/* 角落纹样：单个较大图形从边角切出画面外。
   露出局部比完整摆中间高级得多——它暗示画面之外还有东西。
   故刻意用负偏移让图形被容器裁掉一部分。 */
.orn--corner {
  width: 140px;
  height: 140px;
  mask-position: center;
  -webkit-mask-position: center;
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
  mask-repeat: repeat-x;
  -webkit-mask-repeat: repeat-x;
  mask-position: center;
  -webkit-mask-position: center;
}

/* 静态花纹永不动画（宪法 7.6 节）。
   唯一例外是等待态意象，那由业务侧单独实现，不走本组件。 */
</style>
