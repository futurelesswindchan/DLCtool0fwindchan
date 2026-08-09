<script setup lang="ts">
/**
 * 悬浮提示
 *
 * ⚠️ 不得把唯一一份关键信息放在这里（宪法 8.1 节）。
 *    悬浮提示在触屏上不可达、键盘用户要 tab 到才出现、且无法复制。
 *    它只适合放「补充解释」，不适合放「用户必须知道的事」。
 *
 * 触发方式同时支持 hover 与 focus：只做 hover 会让键盘用户永远看不到。
 *
 * 延迟 300ms 出现、立即消失。理由：鼠标扫过界面时会掠过很多元素，
 * 无延迟会导致提示不停闪；而消失若也有延迟，用户移开后提示还赖着，
 * 会挡住他想点的下一个东西。
 */

import { ref, nextTick, onUnmounted } from 'vue'
import { useAnchoredLayer } from '../../composables/useAnchoredLayer'

interface Props {
  /** 提示文字。为空则不渲染浮层，便于按条件启用 */
  content?: string
  /** 出现延迟，毫秒 */
  delay?: number
}

const props = withDefaults(defineProps<Props>(), { delay: 300 })

const visible = ref(false)
const anchorEl = ref<HTMLElement | null>(null)
const layerEl = ref<HTMLElement | null>(null)

let timer: number | undefined

const { style, update, bind, unbind } = useAnchoredLayer({ gap: 6 })

function recompute() {
  update(anchorEl.value, layerEl.value)
}

async function show() {
  if (!props.content) return

  window.clearTimeout(timer)
  timer = window.setTimeout(async () => {
    visible.value = true
    await nextTick()
    recompute()
    bind(recompute)
  }, props.delay)
}

function hide() {
  window.clearTimeout(timer)
  if (!visible.value) return
  visible.value = false
  unbind(recompute)
}

onUnmounted(() => {
  window.clearTimeout(timer)
  unbind(recompute)
})
</script>

<template>
  <span
    ref="anchorEl"
    class="tip__anchor"
    @mouseenter="show"
    @mouseleave="hide"
    @focusin="show"
    @focusout="hide"
  >
    <slot />

    <Teleport to="body">
      <span
        v-if="visible && content"
        ref="layerEl"
        class="tip"
        role="tooltip"
        :style="{ top: style.top, left: style.left }"
      >
        {{ content }}
      </span>
    </Teleport>
  </span>
</template>

<style scoped>
.tip__anchor {
  display: inline-flex;
  align-items: center;
}
</style>

<style>
/* 浮层经 Teleport 挂到 body，scoped 的 data 属性不会跟过去，
   故这段必须是全局样式。类名加 tip 前缀避免冲突。

   ⚠️ 未入 @layer，故优先级高于 ui 层——这是 Teleport 内容的固有代价，
      调用方若需覆盖得用同等或更高特异性。 */
.tip {
  position: fixed;
  z-index: 995;

  max-width: 260px;
  padding: var(--space-2) var(--space-3);

  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);
  box-shadow: var(--elev-2), var(--hairline-top);

  color: var(--color-text);
  font-size: var(--text-xs);
  line-height: var(--leading-normal);

  /* 提示不该拦住鼠标：若它挡在锚点与目标之间，会造成「移过去就消失」的抖动 */
  pointer-events: none;

  animation: tip-in var(--dur-instant) var(--ease-decelerate);
}

@keyframes tip-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
</style>
