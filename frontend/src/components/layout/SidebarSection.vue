<script setup lang="ts">
/**
 * 侧栏分组
 *
 * 只负责「一个可选的小标题 + 一组条目」。折叠态下标题隐藏但分隔线保留——
 * 图标条模式下若连分隔都没有，几个图标会糊成一列，用户读不出分组。
 *
 * 计数是「有几个可选项」，属选择的辅助信息，不算宪法 3.1 禁止的结论性内容。
 */

import { useUiStore } from "../../stores/ui";

defineProps<{
  /** 分组标题。折叠态下不显示 */
  title?: string;
  /** 可选计数，附在标题后 */
  count?: number;
}>();

const ui = useUiStore();
</script>

<template>
  <section class="sec" :class="{ 'sec--collapsed': ui.sidebarCollapsed }">
    <!--
      标题在折叠态下透隐但保留高度，不用 v-if 销毁——
      否则每个分组丢约 24px，分组一多，折叠前后条目位置差得很远。
    -->
    <h2 v-if="title" class="sec__title">
      <span class="sec__title-text u-truncate">{{ title }}</span>
      <span v-if="count !== undefined" class="sec__count u-tnum">{{
        count
      }}</span>
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

/*
  高度写死：折叠态标题透隐但要占住同样的垂直空间，
  由内容撑开则折叠时高度归零。
*/
.sec__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  box-sizing: border-box;
  height: 18px;
  margin: 0 0 var(--space-2);
  padding-inline: var(--space-2);
  color: var(--color-text-dim);
  font-size: var(--text-xs);
  font-weight: var(--weight-medium);
  /* 分组标题是唯一允许字距放开的地方，它需要读起来像「标签」而非句子 */
  letter-spacing: 0.04em;
  overflow: hidden;
  /* 与侧栏 width 同频 */
  transition: opacity var(--dur-base) var(--ease-standard);
}

/* 折叠态：透隐、不占横向、不可选中，但高度由上方 height 保住 */
.sec--collapsed .sec__title {
  opacity: 0;
  padding-inline: 0;
  pointer-events: none;
  white-space: nowrap;
}

.sec__title-text {
  min-width: 0;
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
