<script setup lang="ts">
/**
 * 顶栏：Logo + 导航页签 + 环境状态 + 窗口控制
 *
 * frameless 模式下本组件同时充当标题栏。环境状态常驻此处——它需要长期
 * 可见，但不应占用内容区空间。
 *
 * ⚠️ 拖动规则：容器声明 `--wails-draggable: drag`，其内**所有**可交互元素
 * 必须显式声明 `no-drag`，否则点击会被识别为拖拽起始而失灵。新增任何按钮
 * 或链接时都要一并加入下方的 no-drag 选择器列表。
 */

import { useRouter } from 'vue-router'
import { useEnvStore } from '../stores/env'
import { useWindowControls } from '../composables/useWindowControls'

const env = useEnvStore()
const router = useRouter()

// 解构而非整体持有：模板里 maximised 直接写名字即可自动解包，
// 若经 win.maximised 访问则需手写 .value，读起来像是漏了什么。
const { maximised, minimise, toggleMaximise, quit } = useWindowControls()

const tabs = [
  { name: 'search', label: '搜索' },
  { name: 'library', label: '已安装' },
  { name: 'settings', label: '设置' },
] as const

const indicatorText: Record<string, string> = {
  ready: 'OST 就绪',
  missing: '未检测到 OST',
  nopath: 'Steam 路径未设置',
}

/** 点击指示灯跳向能解决问题的页面，就绪时无动作。 */
function onIndicatorClick() {
  if (env.indicator === 'missing') router.push({ name: 'setup' })
  else if (env.indicator === 'nopath') router.push({ name: 'settings' })
}

/**
 * 双击标题栏切换最大化。
 *
 * frameless 下系统不再提供这一行为，须自行绑定。绑在容器上即可——
 * 子元素声明的 no-drag 只影响拖拽识别，不阻止事件冒泡。
 */
function onTitleBarDblClick(e: MouseEvent) {
  // 双击按钮或页签不应触发最大化，用户此时的意图是连点该控件
  if ((e.target as HTMLElement).closest('button, a')) return
  toggleMaximise()
}
</script>

<template>
  <header class="topbar" @dblclick="onTitleBarDblClick">
    <div class="topbar__brand">
      <span class="topbar__logo" aria-hidden="true">🐰</span>
      <span class="topbar__name">风兔盒</span>
    </div>

    <nav class="topbar__nav">
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="nav-tab"
        active-class="nav-tab--active"
      >
        {{ t.label }}
      </RouterLink>
    </nav>

    <button
      class="env"
      :class="`env--${env.indicator}`"
      :disabled="env.indicator === 'ready'"
      :title="env.result?.message || ''"
      @click="onIndicatorClick"
    >
      <span class="env__dot" aria-hidden="true"></span>
      {{ indicatorText[env.indicator] }}
    </button>

    <!--
      窗口控制。图标用 SVG 内联而非字符——字体回退会让 ─ □ ✕ 三者的
      粗细与基线各不相同，在小尺寸下尤其明显。
    -->
    <div class="wctl">
      <button class="wctl__btn" title="最小化" aria-label="最小化" @click="minimise()">
        <svg viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 5h10" />
        </svg>
      </button>

      <button
        class="wctl__btn"
        :title="maximised ? '还原' : '最大化'"
        :aria-label="maximised ? '还原' : '最大化'"
        @click="toggleMaximise()"
      >
        <svg v-if="maximised" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M2.5 0.5h7v7h-7z" />
          <path d="M0.5 2.5h7v7h-7z" />
        </svg>
        <svg v-else viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0.5 0.5h9v9h-9z" />
        </svg>
      </button>

      <button
        class="wctl__btn wctl__btn--close"
        title="关闭"
        aria-label="关闭"
        @click="quit()"
      >
        <svg viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 0l10 10M10 0L0 10" />
        </svg>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  gap: var(--space-5);
  height: 46px;
  /* 右侧不留内边距——窗口控制按钮须贴到窗口边缘，与系统标题栏观感一致 */
  padding: 0 0 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
  flex: 0 0 auto;

  /* frameless 下本条即是标题栏，整体可拖动 */
  --wails-draggable: drag;
}

/*
  拖动豁免清单。
  遗漏任何一项都会让该元素的点击被吞掉——这是 frameless 最高频的翻车点，
  故用「一切按钮与链接」的宽泛选择器兜住，而非逐个类名列举。
*/
.topbar button,
.topbar a,
.topbar input,
.topbar select {
  --wails-draggable: no-drag;
}

.topbar__brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: var(--weight-semibold);
}

.topbar__logo {
  /* 1.1rem(17.6px) -> --text-lg(19)。它是品牌位，按语义归页面标题档 */
  font-size: var(--text-lg);
  line-height: 1;
}

.topbar__nav {
  display: flex;
  gap: var(--space-1);
}

/*
  圆角取 ctrl 档而非账本给的 chip 档：页签是可点的按钮性元素，
  而 chip 档（4px）按宪法 4.4 是「角标、小徽章」用的。
  4px 配在 8/12 内边距上会显得几乎没圆角，与同屏其他按钮不同源。
*/
.nav-tab {
  position: relative;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-ctrl);
  color: var(--color-text-muted);
  font-size: var(--text-base);
  text-decoration: none;
  transition: color var(--dur-instant) var(--ease-standard),
    background var(--dur-instant) var(--ease-standard);
}

.nav-tab:hover {
  background: var(--color-surface-2);
  color: var(--color-text);
}

.nav-tab--active {
  color: var(--color-text);
}

/* 当前项下划线。用 transform 缩放而非改宽度，避免触发重排 */
.nav-tab--active::after {
  content: '';
  position: absolute;
  left: var(--space-3);
  right: var(--space-3);
  bottom: 2px;
  height: 2px;
  border-radius: 1px;
  background: var(--color-accent);
  /*
    ⚠️ 这是 animation 而非 transition，属宪法 5.4 节的例外情形：
    下划线是「新出现的元素」而非「从旧位置移到新位置」，没有可插值的起点。

    第 5 步的改法是把下划线抽成一条常驻的指示器、用 transform 在页签间滑移，
    那时它才能变成可中断的 transition。届时本段连同 @keyframes 一起删。
  */
  animation: tab-in var(--dur-fast) var(--ease-decelerate);
}

@keyframes tab-in {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}

.env {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border);
  /*
    ⚠️ 999px 是全圆胶囊，宪法 4.4 节明令禁止（那是消费级 App 的语言）。
       但此处是**状态指示灯**而非按钮——胶囊形恰好把它与同屏的方角控件
       区分开，让人一眼看出「这不是一个操作」。
       第 5 步若要统一，替代方案是去掉边框只留色点与文字，而不是改成方角。
  */
  border-radius: 999px;
  background: transparent;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  font-family: inherit;
  cursor: pointer;
}

.env:disabled {
  cursor: default;
  opacity: 1;
}

.env:not(:disabled):hover {
  background: var(--color-surface-2);
  color: var(--color-text);
}

.env__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.env--ready { color: var(--state-ok); }
.env--missing { color: var(--state-warn); }
.env--nopath { color: var(--color-text-dim); }

/* ─── 窗口控制 ─── */

.wctl {
  display: flex;
  align-self: stretch;
  margin-left: var(--space-4);
}

/*
  尺寸取 46×46 而非 Windows 原生的 46×32：本标题栏整条高 46px，按钮占满
  全高才不会在其上下留出无法点击的死区。宽度 46 使热区接近正方形。
*/
.wctl__btn {
  display: grid;
  place-items: center;
  width: 46px;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: background var(--dur-instant) var(--ease-standard),
    color var(--dur-instant) var(--ease-standard);
}

.wctl__btn svg {
  width: 10px;
  height: 10px;
  /* 描边绘制而非填充：1px 线宽在任何缩放下都保持视觉一致 */
  fill: none;
  stroke: currentColor;
  stroke-width: 1;
}

.wctl__btn:hover {
  background: var(--color-surface-2);
  color: var(--color-text);
}

/* 关闭键沿用系统的红底白字，用户对这一约定的认知比任何自创配色都强。
   文字色用 --color-on-accent 而非 #fff：两套主题下它都是「压在饱和色上
   的字色」，而纯白压在浅色主题那个偏亮的红上会发飘。 */
.wctl__btn--close:hover {
  background: var(--state-danger);
  color: var(--color-on-accent);
}

.wctl__btn:focus-visible {
  outline: 1px solid var(--color-accent);
  outline-offset: -3px;
}
</style>
