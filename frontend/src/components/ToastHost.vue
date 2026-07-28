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
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  box-shadow: 0 6px 20px rgb(0 0 0 / 35%);
  font-size: 0.85rem;
  cursor: pointer;
  pointer-events: auto;
  white-space: pre-line;
}

.toast--success { border-left-color: var(--color-success); }
.toast--warn { border-left-color: var(--color-warning); }
.toast--error { border-left-color: var(--color-danger); }
.toast--info { border-left-color: var(--color-accent); }

/* 只过渡 transform 与 opacity，二者由合成器处理，不触发布局重算 */
.toast-enter-active,
.toast-leave-active {
  transition: transform var(--duration-base) var(--ease-out),
    opacity var(--duration-base) var(--ease-out);
}

.toast-enter-from,
.toast-leave-to {
  transform: translateX(24px);
  opacity: 0;
}
</style>
