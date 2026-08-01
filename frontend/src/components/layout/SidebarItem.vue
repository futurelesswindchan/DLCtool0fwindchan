<script setup lang="ts">
/**
 * 侧栏条目：一个可选中的导航项
 *
 * 宪法 11.2 的组件清单里没有它，这是刻意补的一件。理由：三页若各自手写
 * 条目标记，选中态、悬停、焦点环、折叠态的表现一定会分头长歪，
 * 而宪法 3.1 要求三页侧栏语义一致——一致性靠约定守不住，得靠同一个组件。
 *
 * 它是 layout 层而非 ui 层，因为它 import 了 router（`RouterLink`）。
 * `ui/` 必须保持「复制到别的项目就能跑」（宪法 11.1）。
 *
 * 选中态由 `RouterLink` 的 `active-class` 给出，不自己比对路由——
 * 手写比对在嵌套路由下极易漏掉尾斜杠与子路径两种情形。
 */

import { useUiStore } from '../../stores/ui'
import type { RouteLocationRaw } from 'vue-router'

const props = defineProps<{
  /** 目标路由。给了就渲染成 RouterLink，否则是普通按钮 */
  to?: RouteLocationRaw
  /** 主文案 */
  label: string
  /** 次要文案，折叠态下隐藏。用于「12 个 DLC」这类条目自身的属性 */
  meta?: string
  /** 折叠态下显示的单字符或 emoji。无图标系统前的占位手段 */
  icon?: string
  /** 需用户知情的异常标记，右侧显示一个点 */
  warning?: boolean
  /** 精确匹配。用于 '' 空子路由，否则父路径会一直高亮 */
  exact?: boolean
}>()

defineEmits<{ click: [] }>()

const ui = useUiStore()

/** 折叠态下 label 不可见，须把完整信息挪到 title 上，否则图标条无法辨认。 */
const titleText = () =>
  props.meta ? `${props.label} · ${props.meta}` : props.label

/**
 * `exact` 的实现要点：两个 class 不能同时给同一个值。
 *
 * RouterLink 的 activeClass 在**子路径也匹配**时生效。若两者都给
 * `item--active`，activeClass 会先命中，exactActiveClass 形同虚设——
 * 于是「在线搜索」（空子路由 `''`）在用户进到 `/app/123` 时仍然高亮，
 * 侧栏出现两个选中项。
 *
 * 故 exact 为真时必须把 activeClass 让空，只留 exactActiveClass。
 */
const activeClass = () => {
  if (!props.to) return undefined
  return props.exact ? '' : 'item--active'
}

const exactActiveClass = () => {
  if (!props.to) return undefined
  return props.exact ? 'item--active' : ''
}
</script>

<template>
  <component
    :is="to ? 'RouterLink' : 'button'"
    :to="to"
    :type="to ? undefined : 'button'"
    class="item"
    :class="{ 'item--collapsed': ui.sidebarCollapsed }"
    :active-class="activeClass()"
    :exact-active-class="exactActiveClass()"
    :title="titleText()"
    @click="$emit('click')"
  >
    <span v-if="icon" class="item__icon" aria-hidden="true">{{ icon }}</span>

    <span v-if="!ui.sidebarCollapsed" class="item__text">
      <span class="item__label u-truncate">{{ label }}</span>
      <span v-if="meta" class="item__meta u-truncate">{{ meta }}</span>
    </span>

    <span
      v-if="warning"
      class="item__warn"
      :title="'该条目需要注意'"
      aria-hidden="true"
    ></span>
  </component>
</template>

<style scoped>
.item {
  /* 选中竖条以伪元素绝对定位，故容器须建立定位上下文 */
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  /* 可点区不低于 28px（宪法 15 章） */
  min-height: 30px;
  padding: var(--space-2);
  border: none;
  /* 同心圆角：侧栏内边距 8px，故条目取 inner 档而非 ctrl 档 */
  border-radius: var(--radius-inner);
  background: transparent;
  color: var(--color-text-muted);
  font-family: inherit;
  font-size: var(--text-base);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
  /* 只过渡颜色，不过渡尺寸——条目数量可能上百 */
  transition: background var(--dur-instant) var(--ease-standard),
    color var(--dur-instant) var(--ease-standard);
}

.item--collapsed {
  justify-content: center;
  padding-inline: 0;
}

.item:hover {
  background: var(--color-surface-2);
  color: var(--color-text);
}

.item:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: -2px;
}

/*
  选中态用 --color-raised 而非主色底。
  宪法 4.1 硬规则：每屏饱和主色只允许出现在「下一步该点的那一个」上。
  侧栏选中项是「已经点过的那个」，抢主色会让真正的行动点失焦。
  主色只以左侧 2px 竖条出现，那是定位标记不是强调。
*/
.item--active {
  background: var(--color-raised);
  color: var(--color-text);
  font-weight: var(--weight-medium);
}

/*
  主色只以左侧 2px 竖条出现——那是定位标记，不是强调。
  用 translateY 而非 top 计算居中：条目高度随 meta 有无而变（30px 或更高），
  写死 top 会让两种条目的竖条不在同一相对位置上。
*/
.item--active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 2px;
  height: 16px;
  border-radius: 1px;
  background: var(--color-accent);
}

.item__icon {
  flex: 0 0 auto;
  width: 16px;
  text-align: center;
  font-size: var(--text-sm);
  line-height: 1;
}

.item__text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  /* flex 项默认 min-width:auto，不加这条 u-truncate 不生效 */
  min-width: 0;
  flex: 1 1 auto;
}

.item__meta {
  color: var(--color-text-dim);
  font-size: var(--text-xs);
  font-weight: var(--weight-normal);
}

.item__warn {
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--state-warn);
}
</style>
