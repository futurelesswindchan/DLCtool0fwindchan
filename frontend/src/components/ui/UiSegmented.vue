<script setup lang="ts">
/**
 * 分段控件
 *
 * 用于 2~4 个平级选项的即时切换，如主题「深色 / 浅色 / 跟随系统」。
 * 超过 4 项应改用 UiSelect——分段控件一宽就会把每段挤到读不清。
 *
 * 与 UiRadio 的分工：语义相同，形态不同。选项少且需要一眼看全时用本组件，
 * 选项多或带较长说明时用 UiRadio 竖排。
 *
 * 指示器用 transform 滑移而非切换各段底色，为的是让眼睛能跟住
 * 「选择从哪移到哪」（宪法 5.2 节）。闪现式切换会丢掉这个信息。
 */

import { computed } from 'vue'

import type { SegmentedOption } from './types'

interface Props {
  modelValue?: string | number
  options: readonly SegmentedOption[]
  disabled?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

const activeIndex = computed(() =>
  Math.max(
    0,
    props.options.findIndex((o) => o.value === props.modelValue),
  ),
)

/**
 * 指示器的位置与宽度由段数均分得出，故各段必须等宽（下方 flex: 1）。
 * 不做按内容宽度自适应：那样指示器就得逐段测量 DOM，
 * 而等宽在选项文字长度接近时反而更整齐。
 */
const indicatorStyle = computed(() => ({
  width: `calc((100% - ${props.options.length - 1} * 2px) / ${props.options.length})`,
  transform: `translateX(calc(${activeIndex.value} * (100% + 2px)))`,
}))
</script>

<template>
  <div class="seg" :class="{ 'seg--disabled': disabled }" role="tablist">
    <span class="seg__indicator" :style="indicatorStyle" aria-hidden="true" />

    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      class="seg__item"
      :class="{ 'seg__item--active': o.value === modelValue }"
      role="tab"
      :aria-selected="o.value === modelValue"
      :disabled="disabled"
      @click="emit('update:modelValue', o.value)"
    >
      {{ o.label }}
    </button>
  </div>
</template>

<style scoped>
.seg {
  position: relative;

  /* 用 grid 而非 flex：grid 的 1fr 会让所有列等于「最宽内容」的宽度，
     flex 的 1 只是「等分剩余空间」，在各项内容宽度不同时做不到真正等宽。

     这正是指示器错位的根因所在——指示器位置按均分计算，
     故各段必须真的等宽，而不是「看起来差不多宽」。 */
  display: inline-grid;
  grid-auto-flow: column;
  grid-auto-columns: 1fr;

  gap: 2px;
  padding: 2px;

  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface-2);
}

.seg--disabled {
  opacity: 0.5;
}

.seg__item {
  position: relative;
  /* 指示器在下方，文字必须在其之上 */
  z-index: 1;

  padding: 4px 12px;
  min-height: 24px;

  border: none;
  /* 同心：外层 8px − 内边距 2px = 6px */
  border-radius: var(--radius-inner);
  background: transparent;
  color: var(--color-text-muted);

  font-family: inherit;
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  white-space: nowrap;

  cursor: pointer;
  transition: color var(--dur-instant) var(--ease-standard);
}

.seg__item:hover:not(:disabled) {
  color: var(--color-text);
}

.seg__item--active {
  color: var(--color-text);
}

.seg__item:disabled {
  cursor: not-allowed;
}

.seg__item:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: -2px;
}

/* 滑移指示器。只过渡 transform，由合成器处理，不触发重排。
 *
 * 配色取 --color-raised 而非直接用 --color-surface：深色主题下
 * --color-surface(#232128) 比轨道 --color-surface-2(#2a2731) 更暗，
 * 而「凸起的东西比底更暗」在物理上不成立，观感就是没凸起。
 * 浅色主题恰好相反，surface 本就比 surface-2 亮。
 * 故抽成语义令牌由主题各自给值，组件侧不必知道这层差异。
 */
.seg__indicator {
  position: absolute;
  top: 2px;
  bottom: 2px;
  left: 2px;

  border-radius: var(--radius-inner);
  background: var(--color-raised);
  box-shadow: var(--elev-1), var(--hairline-top);

  transition: transform var(--dur-fast) var(--ease-standard);
}
</style>
