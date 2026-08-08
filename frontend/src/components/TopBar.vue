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

import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnvStore } from '../stores/env'
import { useWindowControls } from '../composables/useWindowControls'
import { LogoMark } from './ui'

const env = useEnvStore()
const router = useRouter()
const route = useRoute()

// 解构而非整体持有：模板里 maximised 直接写名字即可自动解包，
// 若经 win.maximised 访问则需手写 .value，读起来像是漏了什么。
const { maximised, minimise, toggleMaximise, quit } = useWindowControls()

const tabs = [
  { name: 'search', label: '搜索' },
  { name: 'library', label: '已安装' },
  { name: 'settings', label: '设置' },
] as const

/**
 * 常驻下划线指示器的位置与宽度。
 *
 * 为何是一条常驻元素而非每个页签各自的 `::after`：
 * 各自的伪元素之间没有连续性，切页签时旧的消失、新的出现，只能做淡入淡出
 * 这类「新元素登场」动效（原实现即如此，且被迫用 animation 而非 transition）。
 * 抽成一条常驻元素后，切换变成同一个物体的位移，这才能用 transition，
 * 也才能在动画中途被再次点击打断——宪法 5.4 要求动效可中断。
 *
 * 为何测量真实 DOM 而非按索引算：
 * 页签宽度由文字长度决定（「搜索」两字与「已安装」三字不等宽），
 * 且字体加载完成、窗口缩放都会改变布局。算不出来，只能量。
 */
const navEl = ref<HTMLElement | null>(null)
const indicator = ref({ left: 0, width: 0 })
/**
 * 首次定位前不显示指示器。
 *
 * 否则它会从 left:0 width:0 滑到正确位置，让人以为界面加载时有个东西飞过。
 * 首次是「出现」，之后才是「移动」。
 */
const indicatorReady = ref(false)

/** 量出当前激活页签的位置，写进 indicator。找不到激活项时保持原位不动。 */
function measureIndicator() {
  const nav = navEl.value
  if (!nav) return
  const active = nav.querySelector<HTMLElement>('.nav-tab--active')
  if (!active) return

  indicator.value = {
    // offsetLeft 相对 nav（nav 是定位父级），无需两次 getBoundingClientRect 相减
    left: active.offsetLeft,
    width: active.offsetWidth,
  }
  indicatorReady.value = true
}

/**
 * 路由变化后重新定位。
 *
 * 必须等 nextTick：active-class 由 RouterLink 在 DOM 更新时才写上，
 * watch 回调触发的那一刻量到的还是上一个激活项。
 */
watch(
  () => route.name,
  () => nextTick(measureIndicator),
)

/**
 * 窗口尺寸变化时重量。
 *
 * ⚠️ 用 ResizeObserver 而非 window resize 事件：顶栏宽度不只随窗口变，
 *    环境指示灯的文案在状态切换时会变长变短（「OST 就绪」→「Steam 路径未设置」），
 *    那会挤压 nav 的位置而完全不触发 window resize。
 */
let ro: ResizeObserver | null = null

onMounted(() => {
  measureIndicator()
  // 字体加载完成会改变页签宽度，量到的旧值会偏。document.fonts 不是所有
  // 环境都有（WebView2 有，但类型上是可选的），故做存在判断。
  document.fonts?.ready.then(measureIndicator)

  if (navEl.value) {
    ro = new ResizeObserver(measureIndicator)
    ro.observe(navEl.value)
  }
})

onBeforeUnmount(() => {
  ro?.disconnect()
  ro = null
})

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
      <LogoMark class="topbar__logo" />
      <span class="topbar__name">风兔盒</span>
    </div>

    <nav ref="navEl" class="topbar__nav">
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="nav-tab"
        active-class="nav-tab--active"
      >
        {{ t.label }}
      </RouterLink>

      <!--
        常驻指示器。放在页签之后而非之前：两者都在同一定位上下文里，
        后者天然叠在上层，无需给谁写 z-index。
      -->
      <span
        v-show="indicatorReady"
        class="nav-ind"
        :style="{
          transform: `translateX(${indicator.left}px)`,
          width: `${indicator.width}px`,
        }"
        aria-hidden="true"
      />
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

/*
  ⚠️ 经 font-size 驱动尺寸，而非写 width/height。

  LogoMark 内部是 `width: 1em; height: 1em`，故给字号即给尺寸。
  这里刻意不写 width/height：那两条会与组件内 scoped 的同名声明**同优先级**
  （双方都是「类 + 属性选择器」），胜负取决于产物里两段样式谁在后面，
  是构建顺序决定的，不是咱们决定的。改字号则完全无冲突。
*/
.topbar__logo {
  /* 1.1rem(17.6px) -> --text-lg(19)。它是品牌位，按语义归页面标题档 */
  font-size: var(--text-lg);
  line-height: 1;
}

.topbar__nav {
  /*
    必须。指示器 .nav-ind 用 absolute 定位，而 script 里量的是页签相对 nav 的
    offsetLeft——两者基准必须是同一个元素，否则下划线会偏到别处去。
  */
  position: relative;
  display: flex;
  gap: var(--space-1);
}

/*
  圆角取 ctrl 档而非账本给的 chip 档：页签是可点的按钮性元素，
  而 chip 档（4px）按宪法 4.4 是「角标、小徽章」用的。
  4px 配在 8/12 内边距上会显得几乎没圆角，与同屏其他按钮不同源。
*/
.nav-tab {
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

/*
  常驻下划线指示器。位置与宽度由 script 测量后经内联 style 给。

  这里没有 animation、也没有 @keyframes——切页签是同一个物体的位移，
  有明确的起点与终点，故能用 transition（宪法 5.4：动效须可中断）。
  连点两个页签时它会从当前位置就地转向，不会先走完再重新出发。

  ⚠️ 只对 transform 与 width 做过渡，不要图省事写 `transition: all`：
     那会把 background 的主题切换也纳入过渡，换主题时下划线颜色慢半拍。
*/
.nav-ind {
  position: absolute;
  bottom: 2px;
  /* left 恒为 0，位移全交给 transform——改 left 会触发重排，transform 不会 */
  left: 0;
  height: 2px;
  border-radius: 1px;
  background: var(--color-accent);
  pointer-events: none;
  transition: transform var(--dur-fast) var(--ease-decelerate),
    width var(--dur-fast) var(--ease-decelerate);
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
