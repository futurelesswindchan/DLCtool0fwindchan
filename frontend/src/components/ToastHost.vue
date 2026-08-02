<script setup lang="ts">
/**
 * Toast 宿主
 *
 * 挂在 App 根部一次，全站共用。位于右下角——右上角会与标题栏的环境
 * 指示争抢注意力，而提示信息不应盖住状态。
 */

import { useUiStore } from '../stores/ui'

const ui = useUiStore()
</script>

<template>
  <TransitionGroup name="toast" tag="div" class="toast-host">
    <div
      v-for="t in ui.toasts"
      :key="t.id"
      class="toast"
      :class="`toast--${t.kind}`"
      role="status"
      @click="ui.dismiss(t.id)"
    >
      {{ t.message }}
    </div>
  </TransitionGroup>
</template>

<style scoped>
.toast-host {
  position: fixed;
  right: var(--space-4);
  bottom: var(--space-4);
  z-index: 900;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--space-2);
  pointer-events: none;
}

.toast {
  max-width: 380px;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-left-width: 3px;
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);
  box-shadow: var(--elev-2), var(--hairline-top);
  font-size: var(--text-base);
  cursor: pointer;
  pointer-events: auto;
  white-space: pre-line;
}

/*
  ⚠️ info 态用 --color-accent 是本项目允许的例外，与对比表那处不同。
     区别在于 Toast 是**当前唯一在动的东西**，不与其他元素争夺注意力；
     而对比表里若某一行用主色，就等于在多个平级源之间做了推荐。
     速查第 6 条针对的是后者。
*/
.toast--success { border-left-color: var(--state-ok); }
.toast--warn { border-left-color: var(--state-warn); }
.toast--error { border-left-color: var(--state-danger); }
.toast--info { border-left-color: var(--color-accent); }

/* 只过渡 transform 与 opacity，二者由合成器处理，不触发布局重算 */
.toast-enter-active,
.toast-leave-active {
  transition: transform var(--dur-fast) var(--ease-decelerate),
    opacity var(--dur-fast) var(--ease-decelerate);
}

.toast-enter-from,
.toast-leave-to {
  transform: translateX(24px);
  opacity: 0;
}
</style>
