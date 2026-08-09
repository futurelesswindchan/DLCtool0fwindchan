<script setup lang="ts">
/**
 * 进度条
 *
 * 两种形态，对应两种诚实程度：
 *   确定态（给了 value）  按比例填充，并可显示「第几个 / 共几个」
 *   不确定态（value 为空）跑一道循环高光，只表达「在动」不谎报进度
 *
 * 不确定态刻意不做成「假进度」——PCL2 值得借鉴的一条正是
 * 「总在解释自己正在做什么」，而假进度条是明确的不诚实：
 * 它一旦走到 90% 卡住，用户对整个软件的信任都会打折。
 *
 * label 存在时会显示在进度条上方右侧，用于写「3 / 7 个源」这类文字。
 * 试下载 41 秒的等待正是它的落点（宪法 5.5 节）——
 * 进度可见的等待不叫等待，叫围观。
 */

import { computed } from 'vue'

interface Props {
  /** 0~100。留空为不确定态 */
  value?: number
  /** 进度条上方的说明文字 */
  label?: string
  /** 语义色。默认用主色，失败时传 danger */
  tone?: 'accent' | 'ok' | 'warn' | 'danger'
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), { tone: 'accent', size: 'md' })

const indeterminate = computed(() => props.value === undefined)

const clamped = computed(() =>
  props.value === undefined ? 0 : Math.min(100, Math.max(0, props.value)),
)
</script>

<template>
  <div class="pg" :class="`pg--${size}`">
    <div v-if="label" class="pg__head">
      <span class="pg__label">{{ label }}</span>
      <span v-if="!indeterminate" class="pg__pct u-tnum">
        {{ Math.round(clamped) }}%
      </span>
    </div>

    <div
      class="pg__track"
      role="progressbar"
      :aria-valuenow="indeterminate ? undefined : Math.round(clamped)"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div
        class="pg__fill"
        :class="[
          `pg__fill--${tone}`,
          { 'pg__fill--indeterminate': indeterminate },
        ]"
        :style="indeterminate ? undefined : { width: `${clamped}%` }"
      />
    </div>
  </div>
</template>

<style scoped>
.pg {
  width: 100%;
}

.pg__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-1);
}

.pg__label {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.pg__pct {
  color: var(--color-text-muted);
  font-size: var(--text-xs);
}

.pg__track {
  position: relative;
  overflow: hidden;
  width: 100%;
  border-radius: 2px;
  background: var(--color-surface-2);
}

.pg--sm .pg__track {
  height: 3px;
}

.pg--md .pg__track {
  height: 5px;
}

.pg__fill {
  height: 100%;
  border-radius: 2px;

  /* 确定态过渡 width 是本项目允许的少数重排之一：
     进度条宽度很小、且每次变化只影响自身，代价可忽略。
     换成 transform: scaleX 会让圆角被拉扁。 */
  transition: width var(--dur-base) var(--ease-standard);
}

.pg__fill--accent {
  background: var(--color-accent);
}
.pg__fill--ok {
  background: var(--state-ok);
}
.pg__fill--warn {
  background: var(--state-warn);
}
.pg__fill--danger {
  background: var(--state-danger);
}

/* 不确定态：一道高光来回跑。这里用 animation 是正确的——
   它是循环动画，不是交互过渡（宪法 11.6 节第三条）。 */
.pg__fill--indeterminate {
  width: 40%;
  animation: pg-sweep 1.2s var(--ease-standard) infinite;
}

@keyframes pg-sweep {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(250%);
  }
}

/* 减少动态效果时停下循环并铺满轨道——
   必须真正禁用而非只是变快（宪法 5.6 节）。
   铺满是为了仍然表达「有事在进行」，只是不再用运动表达。 */
@media (prefers-reduced-motion: reduce) {
  .pg__fill--indeterminate {
    animation: none;
    width: 100%;
    opacity: 0.5;
  }
}
</style>
