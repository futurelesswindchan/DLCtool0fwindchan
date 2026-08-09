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

import { useUiStore } from "../../stores/ui";
import { UiIcon, UiTooltip } from "../ui";
import type { IconName } from "../ui";

defineProps<{
  /**
   * 折叠态下品牌区显示的图标。
   *
   * 展开态的 brand 内容（slogan / 筛选框 / 标题）在 56px 里都放不下，
   * 但那块位置不能空着——它是本页的标识位，空着会让折叠态看起来缺了一截。
   */
  brandIcon?: IconName;
}>();

const ui = useUiStore();
</script>

<template>
  <aside
    class="sidebar"
    :class="{ 'sidebar--collapsed': ui.sidebarCollapsed }"
    :aria-label="'侧边导航'"
  >
    <!--
      品牌区在折叠态下**保留高度但内容透隐**，不用 v-if 销毁。
      三页 brand 内容不同（Logo+文字 / 筛选框 / 标题），若整块移除，
      折叠时三页各丢不同高度，下方条目的起始位置对不上。
    -->
    <div v-if="$slots.brand" class="sidebar__brand">
      <div class="sidebar__brand-inner">
        <slot name="brand" />
      </div>

      <!--
        折叠态图标。与 brand 内容叠放（absolute）而非并列——
        并列会让两者在过渡期间互相挤压，叠放则各自只做透明度变化。
      -->
      <UiIcon v-if="brandIcon" :name="brandIcon" class="sidebar__brand-icon" />
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
    <UiTooltip
      :content="ui.sidebarCollapsed ? '展开侧栏' : '折叠侧栏'"
      class="sidebar__toggle-anchor"
    >
      <button
        class="sidebar__toggle"
        :aria-label="ui.sidebarCollapsed ? '展开侧栏' : '折叠侧栏'"
        :aria-expanded="!ui.sidebarCollapsed"
        @click="ui.toggleSidebar()"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path :d="ui.sidebarCollapsed ? 'M6 4l4 4-4 4' : 'M10 4L6 8l4 4'" />
        </svg>
        <span class="sidebar__toggle-text">折叠</span>
      </button>
    </UiTooltip>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  flex: 0 0 auto;
  width: var(--sidebar-w);
  min-height: 0;
  border-right: 2px solid var(--color-border);
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

/*
  高度写死而非由内容撑开：折叠态要保留同样的高度，而三页 brand 内容不同。
  padding 与 border 在两态都在场，故分隔线位置不动。
*/
.sidebar__brand {
  position: relative;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  height: var(--sidebar-brand-h);
  padding: 0 var(--space-3);
  border-bottom: 1px solid var(--color-border);
  overflow: hidden;
}

/*
  折叠态图标。left 取 20px 让 16px 图标中心落在距左 28px，
  与条目图标（padding-inline: 20px，16px 图标中心 x=28）、
  折叠开关箭头（padding-left: 22px，12px 箭头中心 x=28）同列。
  原先 left: var(--space-3)=12px 使中心在 x=20，偏左 8px。
*/
.sidebar__brand-icon {
  position: absolute;
  left: 20px;
  top: 50%;
  width: 16px;
  height: 16px;
  margin-top: -8px;
  color: var(--color-text-dim);
  opacity: 0;
  pointer-events: none;
  transition: opacity var(--dur-base) var(--ease-standard);
}

.sidebar--collapsed .sidebar__brand-icon {
  opacity: 1;
}

.sidebar__brand-inner {
  width: 100%;
  min-width: 0;
  /* 与侧栏 width 同频，折叠时内容渐隐而非瞬间消失 */
  transition: opacity var(--dur-base) var(--ease-standard);
}

/*
  折叠态：内容透隐且不可交互，但高度由外层保住。
  nowrap 兜住一件事——透隐期间内容仍在布局中，56px 窄条会把文字压成竖排，
  透明度到 0 之前那几帧会看见挤压过程。
*/
.sidebar--collapsed .sidebar__brand {
  padding-inline: 0;
}

.sidebar--collapsed .sidebar__brand-inner {
  opacity: 0;
  pointer-events: none;
  white-space: nowrap;
}

/*
  主体滚动区。content-visibility 交给内部长列表自己声明——
  挂在滚动容器上会让滚动条高度反复跳动。

  横向内边距为 0：条目自身的 padding-inline:20px 足以提供呼吸空间，
  加一层 body padding 会让折叠态下图标中心偏离 28px 对齐线。
  （原先 var(--space-2)=8px 使图标中心跑到 36px，比品牌图标偏右 8px。）
*/
.sidebar__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-2) 0 var(--space-3);
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
  /* 横向 22px 让 12px 箭头中心落在 x=28，与条目图标对中 */
  padding: var(--space-2) 22px;
  border: none;
  border-top: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-dim);
  font-family: inherit;
  font-size: var(--text-xs);
  cursor: pointer;
  /* 「折叠」二字透隐期间仍在布局中，56px 窄条会挤压它，故裁掉溢出 */
  overflow: hidden;
  transition:
    color var(--dur-instant) var(--ease-standard),
    background var(--dur-instant) var(--ease-standard);
}

/*
  折叠开关与条目同病同治：原先 justify-content: center + padding-inline: 0
  会让箭头在宽度收缩期间先跳到右边再左移。
  改为两态同一横向内边距，箭头一动不动。

  22px 让 12px 箭头中心落在 x=28，与条目图标同列。
*/
.sidebar--collapsed .sidebar__toggle {
  padding-inline: 22px;
}

/*
  UiTooltip 的锚点是 inline-flex，在 flex column 中会收缩成内容宽度，
  故显式撑满——否则「折叠」这一条的点击区只有文字那么宽。

  scoped 样式能命中子组件根元素（Vue 会给单根子组件带上父组件的 scope id），
  此处只碰布局不碰外观，符合既定分工。
*/
.sidebar__toggle-anchor {
  display: flex;
  flex: 0 0 auto;
}

.sidebar__toggle-anchor > .sidebar__toggle {
  width: 100%;
}

/* 「折叠」二字与 brand 内容同一手法：透隐不销毁，保住高度与位置 */
.sidebar__toggle-text {
  /*
    始终 nowrap，不只在折叠态。
    若只在折叠态加，展开时移除会触发一帧高度重排，表现为按钮被「顶一下」。
    「折叠」两字永远是一行，写死 nowrap 无副作用。
  */
  white-space: nowrap;
  transition: opacity var(--dur-base) var(--ease-standard);
}

.sidebar--collapsed .sidebar__toggle-text {
  opacity: 0;
  pointer-events: none;
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
