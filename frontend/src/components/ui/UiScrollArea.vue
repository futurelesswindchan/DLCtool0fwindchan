<script setup lang="ts">
/**
 * 滚动容器
 *
 * 全局已接管 ::-webkit-scrollbar（styles/base/scrollbar.css），
 * 本组件解决的是另外两件事：
 *
 *   1. **顶/底渐隐遮罩**，暗示「上面还有内容」。
 *      长列表滚到中间时若没有这个提示，用户会以为已经到顶了。
 *   2. **长列表性能**，可选开启 content-visibility。
 *
 * ⚠️ content-visibility 与错开入场动效可能冲突（宪法第 13 章风险表）：
 *    视口外元素不参与渲染，也就不参与动画，滚回来时可能出现跳变。
 *    故默认关闭，由调用方按列表长度决定——短列表开了没收益，
 *    只是白担一份跳变风险。
 */

import { ref, computed } from 'vue'

interface Props {
  /** 开启 content-visibility。仅长列表（数十项以上）需要 */
  longList?: boolean
  /** 关掉渐隐遮罩。内容本就不足一屏时遮罩纯属噪音 */
  noFade?: boolean
}

const props = defineProps<Props>()

const el = ref<HTMLElement | null>(null)
const atTop = ref(true)
const atBottom = ref(true)

/**
 * 滚动位置探测。用 1px 容差而非严格相等——
 * 缩放比例非整数时 scrollTop 会出现小数，严格判等会导致遮罩不消失。
 */
function onScroll() {
  const n = el.value
  if (!n) return

  atTop.value = n.scrollTop <= 1
  atBottom.value = n.scrollTop + n.clientHeight >= n.scrollHeight - 1
}

const showTopFade = computed(() => !props.noFade && !atTop.value)
const showBottomFade = computed(() => !props.noFade && !atBottom.value)
</script>

<template>
  <div class="sa">
    <div
      ref="el"
      class="sa__body"
      :class="{ 'sa__body--long': longList }"
      @scroll="onScroll"
    >
      <slot />
    </div>

    <!-- 遮罩不拦鼠标，否则会吃掉顶部第一项的点击 -->
    <span v-if="showTopFade" class="sa__fade sa__fade--top" aria-hidden="true" />
    <span
      v-if="showBottomFade"
      class="sa__fade sa__fade--bottom"
      aria-hidden="true"
    />
  </div>
</template>

<style scoped>
.sa {
  position: relative;
  /* 必须能被 flex 父容器压缩，否则内容一多就把父容器顶开、
     滚动条永远不出现。min-height: 0 是 flex 布局里最常漏的一行。 */
  min-height: 0;
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
}

.sa__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  /* 横向一律不滚：出现横向滚动条基本都意味着某处没做截断，
     属于布局缺陷而非需要滚动 */
  overflow-x: hidden;
}

.sa__body--long > :deep(*) {
  content-visibility: auto;
  contain-intrinsic-size: auto 48px;
}

.sa__fade {
  position: absolute;
  left: 0;
  right: 0;
  height: 24px;
  pointer-events: none;
}

/* 渐隐用当前底色到透明。因为取的是令牌，双主题自动跟随，
   不必写两套（与花纹用 mask-image 是同一个思路）。 */
.sa__fade--top {
  top: 0;
  background: linear-gradient(to bottom, var(--color-bg), transparent);
}

.sa__fade--bottom {
  bottom: 0;
  background: linear-gradient(to top, var(--color-bg), transparent);
}
</style>
