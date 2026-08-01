<script setup lang="ts">
/**
 * 侧栏分组
 *
 * 只负责「一个可选的小标题 + 一组条目」。折叠态下标题隐藏但分隔线保留——
 * 图标条模式下若连分隔都没有，几个图标会糊成一列，用户读不出分组。
 *
 * 计数是「有几个可选项」，属选择的辅助信息，不算宪法 3.1 禁止的结论性内容。
 */

import { useUiStore } from '../../stores/ui'

defineProps<{
  /** 分组标题。折叠态下不显示 */
  title?: string
  /** 可选计数，附在标题后 */
  count?: number
}>()

const ui = useUiStore()
</script>

<template>
  <section class="sec" :class="{ 'sec--collapsed': ui.sidebarCollapsed }">
    <h2 v-if="title && !ui.sidebarCollapsed" class="sec__title">
      {{ title }}
      <span v-if="count !== undefined" class="sec__count u-tnum">{{ count }}</span>
    </h2>

    <div class="sec__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.sec + .sec {
  margin-top: var(--space-4);
  padding-top: var(--space-3);
  border-top: 1px solid var(--color-border);
}

.sec__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin: 0 0 var(--space-2);
  padding-inline: var(--space-2);
  color: var(--color-text-dim);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  /* 分组标题是唯一允许字距放开的地方，它需要读起来像「标签」而非句子 */
  letter-spacing: 0.04em;
}

.sec__count {
  color: var(--color-text-dim);
  font-weight: var(--weight-normal);
}

.sec__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
</style>
