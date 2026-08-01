<script setup lang="ts">
/**
 * 侧栏容器
 *
 * 三页共用同一个容器，内容由各 Shell 经插槽填入。
 *
 * ⚠️ 语义铁律（宪法 3.1）：**侧栏只承载「选择」，永不承载最终信息。**
 *    往这里塞「共 12 个 DLC」这类结论性内容会破坏用户对侧栏的稳定预期——
 *    三页若各有各的规矩，三板斧就只是三个巧合。
 *    运行状态摘要是允许的例外，因为它是「入口」（点了去修环境），不是结论。
 *
 * 插槽：
 *   brand   顶部品牌位／标题，折叠态下隐藏
 *   default 主体，可滚动
 *   footer  底部常驻区，不参与滚动
 */

import { useUiStore } from '../../stores/ui'

const ui = useUiStore()
</script>

<template>
  <aside
    class="sidebar"
    :class="{ 'sidebar--collapsed': ui.sidebarCollapsed }"
    :aria-label="'侧边导航'"
  >
    <div v-if="!ui.sidebarCollapsed && $slots.brand" class="sidebar__brand">
      <slot name="brand" />
    </div>

    <!--
      主体滚动区。content-visibility 交给内部长列表自己声明——
      挂在滚动容器上会让滚动条高度反复跳动。
    -->
    <div class="sidebar__body">
      <slot />
    </div>

    <div v-if="$slots.footer" class="sidebar__footer">
      <slot name="footer" />
    </div>

    <!--
      折叠开关常驻底部。放底部而非顶部：顶部紧邻顶栏的品牌位，
      两个视觉重心挨着会打架；且底部是「不常用但要找得到」的惯例位置。
    -->
    <button
      class="sidebar__toggle"
      :title="ui.sidebarCollapsed ? '展开侧栏' : '折叠侧栏'"
      :aria-label="ui.sidebarCollapsed ? '展开侧栏' : '折叠侧栏'"
      :aria-expanded="!ui.sidebarCollapsed"
      @click="ui.toggleSidebar()"
    >
      <svg viewBox="0 0 16 16" aria-hidden="true">
        <path :d="ui.sidebarCollapsed ? 'M6 4l4 4-4 4' : 'M10 4L6 8l4 4'" />
      </svg>
      <span v-if="!ui.sidebarCollapsed" class="sidebar__toggle-text">折叠</span>
    </button>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  flex: 0 0 auto;
  width: var(--sidebar-w);
  min-height: 0;
  border-right: 1px solid var(--color-border);
  background: var(--color-surface);
  /*
    宽度过渡是宪法 14 章允许的唯一重排例外之一（与侧栏高度同类）。
    折叠只在用户点击或窗口跨阈值时发生，不是高频动作。
  */
  transition: width var(--dur-base) var(--ease-standard);
}

.sidebar--collapsed {
  width: var(--sidebar-w-collapsed);
}

.sidebar__brand {
  flex: 0 0 auto;
  padding: var(--space-4) var(--space-3) var(--space-3);
  border-bottom: 1px solid var(--color-border);
}

.sidebar__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-2) var(--space-2) var(--space-3);
}

.sidebar__footer {
  flex: 0 0 auto;
  padding: var(--space-2);
  border-top: 1px solid var(--color-border);
}

.sidebar__toggle {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 0 0 auto;
  /* 可点区不低于 28px（宪法 15 章第 3 条） */
  min-height: 28px;
  padding: var(--space-2) var(--space-3);
  border: none;
  border-top: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-dim);
  font-family: inherit;
  font-size: var(--text-xs);
  cursor: pointer;
  transition: color var(--dur-instant) var(--ease-standard),
    background var(--dur-instant) var(--ease-standard);
}

.sidebar--collapsed .sidebar__toggle {
  justify-content: center;
  padding-inline: 0;
}

.sidebar__toggle:hover {
  background: var(--color-surface-2);
  color: var(--color-text-muted);
}

.sidebar__toggle:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: -2px;
}

.sidebar__toggle svg {
  width: 12px;
  height: 12px;
  flex: 0 0 auto;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
