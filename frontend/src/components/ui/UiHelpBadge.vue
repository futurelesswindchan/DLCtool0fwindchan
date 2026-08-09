<script setup lang="ts">
/**
 * 术语帮助徽章
 *
 * 挂在术语旁边的那颗「?」。承载宪法铁律四：
 * **不降低世界的复杂度，只降低理解它的成本**——
 * 术语照实写（manifest、depot、GID 就叫这个名字），但给出口。
 *
 * ⚠️ 必须是真 <button>（宪法 8.1 节）。
 *    做成 <span> 则键盘用户无法触达，而需要看解释的人恰恰是最没经验的人。
 *
 * ⚠️ 文案唯一来源是 glossary.ts（宪法 8.1 节），第 6 步铺开。
 *    本组件只负责呈现，不内置任何术语文本——内置就会出现
 *    同一个术语在三处解释得不一样。
 *
 * 两种触发分工：
 *   悬停 → 显示 ariaLabel（短提示，引导点击）
 *   点击 → 钉住展开面板，显示完整 glossary 解释（可选中复制）
 *
 * 这样做的理由：hover 给全答案会让用户不点击——
 * 而只有点击才能钉住面板、复制文字、去搜索引擎查。
 */

import { ref, onUnmounted } from "vue";
import UiTooltip from "./UiTooltip.vue";

interface Props {
  /** 解释文字。调用方从 glossary 取，组件不内置 */
  content: string;
  /** 悬浮提示文字与无障碍标签，默认「查看说明」。
   *  同时也是 hover 时 tooltip 显示的内容——短提示引导点击。 */
  ariaLabel?: string;
}

const props = withDefaults(defineProps<Props>(), { ariaLabel: "查看说明" });

/** 点击后钉住的展开态。与悬停提示互不干扰 */
const pinned = ref(false);

function onDocClick() {
  pinned.value = false;
}

function toggle(e: MouseEvent) {
  // 阻止冒泡，否则本次点击会立刻被下方的关闭监听吃掉
  e.stopPropagation();

  pinned.value = !pinned.value;
  if (pinned.value) {
    // 延到下一帧再挂监听，同样是为了避开本次点击
    window.setTimeout(() => document.addEventListener("click", onDocClick), 0);
  } else {
    document.removeEventListener("click", onDocClick);
  }
}

onUnmounted(() => document.removeEventListener("click", onDocClick));
</script>

<template>
  <span class="hb__wrap">
    <UiTooltip :content="pinned ? '' : props.ariaLabel">
      <button
        type="button"
        class="hb"
        :class="{ 'hb--pinned': pinned }"
        :aria-label="ariaLabel"
        :aria-expanded="pinned"
        @click="toggle"
      >
        ?
      </button>
    </UiTooltip>

    <!-- 钉住态就地展开，不用浮层：它可以被选中复制，
         且不会因鼠标移动而消失 -->
    <span v-if="pinned" class="hb__panel">{{ content }}</span>
  </span>
</template>

<style scoped>
.hb__wrap {
  display: inline;
}

.hb {
  display: inline-grid;
  place-items: center;

  /* 视觉 14px，但热区靠 padding 撑到 28px（宪法 6.3 节第二条）。
     用 outline-offset 负值让焦点环仍贴着可见圆圈。 */
  width: 14px;
  height: 14px;
  padding: 2px;
  box-sizing: border-box;

  border: 1px solid var(--color-border-str);
  border-radius: 50%;
  background: transparent;
  color: var(--color-text-dim);

  font-family: inherit;
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  line-height: 1;

  cursor: pointer;
  vertical-align: middle;
  transition:
    color var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard);
}

.hb:hover,
.hb--pinned {
  color: var(--color-accent);
  border-color: var(--color-accent);
}

.hb:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: -6px;
}

/* 就地展开面板。作为 block 插入，会把下方内容推开——
   这是有意的：它比浮层更诚实，用户能看清它属于哪个术语。 */
.hb__panel {
  display: block;
  margin-top: var(--space-2);
  padding: var(--space-2) var(--space-3);

  border-left: 2px solid var(--color-accent);
  border-radius: var(--radius-chip);
  background: var(--color-surface-2);

  color: var(--color-text-muted);
  font-size: var(--text-xs);
  line-height: var(--leading-normal);

  /* 术语解释常要被复制去搜索，必须允许选中 */
  user-select: text;
}
</style>
