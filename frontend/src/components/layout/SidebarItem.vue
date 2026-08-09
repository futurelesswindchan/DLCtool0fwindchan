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

import { computed } from "vue";
import { useUiStore } from "../../stores/ui";
import { UiIcon, UiTooltip } from "../ui";
import type { IconName } from "../ui";
import type { RouteLocationRaw } from "vue-router";

const props = defineProps<{
  /** 目标路由。给了就渲染成 RouterLink，否则是普通按钮 */
  to?: RouteLocationRaw;
  /** 主文案 */
  label: string;
  /** 次要文案。用于「12 个 DLC」这类条目自身的属性。
   *  折叠态下文字不销毁（只透隐），以保持 Y 轴高度不变。 */
  meta?: string;
  /**
   * 图标名。折叠态下它是唯一可见的辨识物，故有 to 的条目都该给。
   *
   * 原先此处是 emoji 字符串，已换为 UiIcon 的图标名——emoji 在不同
   * Windows 版本与字体回退下渲染差异很大（尺寸、基线、彩色与否都不定），
   * 而侧栏图标要与 LogoMark、Ornament 属同一套图形词汇。
   */
  icon?: IconName;
  /**
   * 首字头像的取字来源，通常传游戏名。与 icon 二选一，icon 优先。
   *
   * 为何不给每个游戏一个统一图标：折叠态下十个游戏会长得一模一样，
   * 等于没有信息。首字虽不保证唯一（「怪物猎人」与「怪物猎人：崛起」
   * 首字相同），但配合悬停提示已足够辨认，且零外部依赖。
   *
   * 刻意不用 Steam 封面图：Library 页离线也必须可用（用户可能正是来
   * 卸载 DLC 的），而封面图要联网、要缓存、要处理失败回退。
   */
  avatar?: string;
  /** 需用户知情的异常标记，右侧显示一个点 */
  warning?: boolean;
  /** 精确匹配。用于 '' 空子路由，否则父路径会一直高亮 */
  exact?: boolean;
}>();

defineEmits<{ click: [] }>();

const ui = useUiStore();

/**
 * 悬停提示内容。
 *
 * 原先走原生 `title` 属性，已换成 `UiTooltip`——宪法 13 条不允许出现
 * 未接管的原生 UI，而原生 tooltip 的外观（灰底小框、跟随鼠标、延迟不可控）
 * 完全不受控，与自绘的浮层同屏出现时观感割裂。
 *
 * 它承载的是**数据**（这一行是哪个游戏），不是术语——故不进 glossary，
 * 也不该换成 UiHelpBadge。两者的分工：HelpBadge 解释概念（全项目一份），
 * 此处呈现条目自身的值（每行都不同）。
 *
 * 警示含义并入这里，不再由警示点自己挂 title——那个点是 aria-hidden 的，
 * 给它挂标签自相矛盾（对读屏器隐藏却又提供文字）。
 */
const tipText = computed(() => {
  const base = props.meta ? `${props.label} · ${props.meta}` : props.label;
  return props.warning ? `${base}（需要注意）` : base;
});

/**
 * 首字。中文取首个汉字，拉丁字母转大写。
 *
 * 用 `Array.from` 而非 `[0]`：后者按 UTF-16 码元取，
 * 遇到 emoji 或某些扩展汉字会截出半个代理对，渲染成乱码方块。
 */
const avatarChar = computed(() => {
  const s = props.avatar?.trim();
  if (!s) return "";
  return (Array.from(s)[0] ?? "").toUpperCase();
});

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
  if (!props.to) return undefined;
  return props.exact ? "" : "item--active";
};

const exactActiveClass = () => {
  if (!props.to) return undefined;
  return props.exact ? "item--active" : "";
};
</script>

<template>
  <UiTooltip :content="tipText" class="item__anchor">
    <component
      :is="to ? 'RouterLink' : 'button'"
      :to="to"
      :type="to ? undefined : 'button'"
      class="item"
      :class="{ 'item--collapsed': ui.sidebarCollapsed }"
      :active-class="activeClass()"
      :exact-active-class="exactActiveClass()"
      @click="$emit('click')"
    >
      <UiIcon v-if="icon" :name="icon" class="item__icon" />

      <!-- 首字头像。尺寸与图标一致，使两类条目在折叠态下对齐同一竖列 -->
      <span v-else-if="avatarChar" class="item__avatar" aria-hidden="true">{{
        avatarChar
      }}</span>

      <span class="item__text">
        <span class="item__label u-truncate">{{ label }}</span>
        <span v-if="meta" class="item__meta u-truncate">{{ meta }}</span>
      </span>

      <!--
        异常标记只在展开态出现。折叠态下它紧贴 56px 窄条右缘，
        而那时用户连是哪个条目都读不出来，一个孤零零的黄点只是噪声——
        它无法回答「哪里出了问题」，反而让人以为是界面故障。

        不挂 title：本元素 aria-hidden，给它挂标签自相矛盾。
        警示含义已并入条目自身的悬停提示（见 tipText）。
      -->
      <span
        v-if="warning && !ui.sidebarCollapsed"
        class="item__warn"
        aria-hidden="true"
      ></span>
    </component>
  </UiTooltip>
</template>

<style scoped>
/*
  UiTooltip 的锚点默认 inline-flex，在侧栏的 flex column 中会收缩成内容宽度，
  导致条目点击区变窄。此处撑满。

  scoped 样式能命中子组件根元素（Vue 给单根子组件带上父组件的 scope id）。
  只碰布局不碰外观——碰边框/字号/焦点环就回到了 shim 模式。
*/
.item__anchor {
  display: block;
}

.item {
  /* 选中竖条以伪元素绝对定位，故容器须建立定位上下文 */
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  /* 可点区不低于 28px（宪法 15 章）。
     44px 匹配带 meta 的两行条目高度，保证折叠/展开态 Y 轴一致。 */
  min-height: 44px;
  /* 横向 20px：让 16px 图标中心落在 x=28，正好是 56px 窄条的视觉重心。
     两态相同值，折叠时图标一动不动（见 .item--collapsed）。 */
  padding: var(--space-2) 20px;
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
  /* 只过渡颜色与文字显隐，不过渡尺寸——条目数量可能上百。
     文字 opacity 与侧栏 width 同频（都走 --dur-base），
     展开时文字渐显而非瞬间弹出。 */
  transition:
    background var(--dur-instant) var(--ease-standard),
    color var(--dur-instant) var(--ease-standard),
    opacity var(--dur-base) var(--ease-standard);
}

/*
  折叠态**不改横向内边距**，故 icon 一动不动。

  原先这里写 justify-content: center + padding-inline: 0，
  两条都是立即生效的，而侧栏 width 要 240ms 才收完——于是 icon 先跳到
  240px 容器的中点，再跟着容器一路漂到左边。
  「居中」在宽度变化期间是个移动靶，不能用它定位。

  20px 让 16px 图标中心落在 x=28，即 56px 窄条的视觉重心，
  展开态 240px 下同样不显偏。
*/
.item--collapsed {
  /* 与展开态同值，此处显式写出以表明「刻意不变」而非漏写 */
  padding-inline: 20px;
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
  用 translateY 而非 top 计算居中：条目高度随 meta 有无而变（44px 起），
  写死 top 会让两种条目的竖条不在同一相对位置上。
*/
.item--active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 2px;
  height: 16px;
  border-radius: 1px;
  background: var(--color-accent);
}

/*
  尺寸写死 16px 而非随字号：条目字号是 --text-base，图标跟着走会偏大，
  且 emoji 时代靠 text-align 居中的手法对 svg 不适用。
*/
.item__icon {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
}

/*
  首字头像。尺寸与 .item__icon 严格一致，两类条目在折叠态才对得齐。

  字号 10px 而非跟随条目字号：16px 方块里放一个汉字，14px 会顶到边。
  chip 档圆角（4px）——它是角标性质的小方块，不是控件。
*/
.item__avatar {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 16px;
  height: 16px;
  border-radius: var(--radius-chip);
  background: var(--color-surface-2);
  color: var(--color-text-dim);
  font-size: 10px;
  font-weight: var(--weight-medium);
  line-height: 1;
}

/* 选中与悬停时头像跟着提亮，否则它在高亮底色上显得是块脏东西 */
.item:hover .item__avatar,
.item--active .item__avatar {
  background: var(--color-raised);
  color: var(--color-text-muted);
}

.item__text {
  display: flex;
  flex-direction: column;
  gap: 1px;
  /* flex 项默认 min-width:auto，不加这条 u-truncate 不生效 */
  min-width: 0;
  flex: 1 1 auto;
  /* 与侧栏 width transition 同频，展开时渐显而非弹入 */
  transition: opacity var(--dur-base) var(--ease-standard);
}

/* 折叠态：文字透隐、不占横向空间，但保持高度——图标居中不受干扰，
   且条目 Y 轴高度与展开态一致。 */
.item--collapsed .item__text {
  opacity: 0;
  max-width: 0;
  overflow: hidden;
  pointer-events: none;
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
