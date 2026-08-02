<script setup lang="ts">
/**
 * 确认弹窗
 *
 * 状态与 resolve 逻辑在 composables/useConfirm.ts，本组件只负责呈现。
 *
 * Esc 与遮罩点击均解析为 false——用户以模糊方式退出时默认取消，
 * 这是不可省的安全默认值。
 */

import { watch, onUnmounted } from 'vue'
import { confirmState, resolveConfirm } from '../composables/useConfirm'

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') resolveConfirm(false)
}

// 仅在弹窗可见期间挂键盘监听，避免常驻监听干扰其他页面的快捷键
watch(
  () => confirmState.value.visible,
  (visible) => {
    if (visible) window.addEventListener('keydown', onKey)
    else window.removeEventListener('keydown', onKey)
  },
)

onUnmounted(() => window.removeEventListener('keydown', onKey))

/**
 * 正文统一按段落数组渲染，字符串视为单段。
 *
 * 入参取 readonly 是因为 confirmState 经 Vue 的 readonly() 包装，其数组
 * 属性会被推导为 readonly string[]，不能赋给可变的 string[]。
 */
function paragraphs(body?: string | readonly string[]): readonly string[] {
  if (!body) return []
  return Array.isArray(body) ? body : [body as string]
}
</script>

<template>
  <Transition name="dialog">
    <div
      v-if="confirmState.visible"
      class="mask"
      @click.self="resolveConfirm(false)"
    >
      <div class="dialog" role="dialog" aria-modal="true">
        <h2 class="dialog__title">{{ confirmState.title }}</h2>

        <div v-if="paragraphs(confirmState.body).length" class="dialog__body">
          <p v-for="(p, i) in paragraphs(confirmState.body)" :key="i">{{ p }}</p>
        </div>

        <div class="dialog__actions">
          <button class="btn" @click="resolveConfirm(false)">
            {{ confirmState.cancelText || '取消' }}
          </button>
          <button
            class="btn"
            :class="confirmState.danger ? 'btn--danger' : 'btn--primary'"
            @click="resolveConfirm(true)"
          >
            {{ confirmState.confirmText || '确定' }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 50%);
}

.dialog {
  width: min(440px, calc(100vw - 64px));
  padding: var(--space-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  /* 硬编码的黑色阴影换成令牌：原值在浅色主题下是纯黑大范围投影，
     落在冷紫底上会灰得发脏（elevation.css 已为此把浅色阴影调成带紫的深色）。
     --hairline-top 补上顶边内高光，浮层才有厚度而不是一块贴纸。 */
  box-shadow: var(--elev-2), var(--hairline-top);
}

.dialog__title {
  margin: 0 0 var(--space-3);
  /* 1rem(16px) -> --text-md(15)。它是区块标题级别，不是页面标题 */
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
}

.dialog__body {
  margin: 0 0 var(--space-5);
  color: var(--color-text-muted);
  font-size: var(--text-base);
  line-height: var(--leading-normal);
}

.dialog__body p {
  margin: 0 0 var(--space-2);
}

.dialog__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.dialog-enter-active,
.dialog-leave-active {
  transition: opacity var(--dur-instant) var(--ease-standard);
}

/* 弹窗本体走 base 档：它是「浮层入场」，属 motion.css 里的中等位移。
   遮罩的淡入比它快一档，使背景先暗下来、弹窗随后到位。 */
.dialog-enter-active .dialog,
.dialog-leave-active .dialog {
  transition: transform var(--dur-base) var(--ease-decelerate);
}

.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}

.dialog-enter-from .dialog,
.dialog-leave-to .dialog {
  transform: scale(0.94);
}
</style>
