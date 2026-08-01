<script setup lang="ts">
/**
 * 按钮
 *
 * 取代全局 .btn（styles/shim.css）。相对 shim 补了三件事：
 *   1. 按下 scale(0.97) 微反馈（宪法 5.3 节）
 *   2. loading 态：禁用点击但保留尺寸，避免按钮宽度跳变
 *   3. icon 形态：正方形，可点区仍不小于 28px
 *
 * 不做 size='lg'。桌面工具里大按钮只会显得虚张声势，
 * 需要强调时用 variant='primary' 而非放大尺寸。
 *
 * NOTE: 全屏只允许一个 primary（宪法 4.1 节点缀色硬规则）——
 * 一屏出现两个粉色按钮等于没有指引。这条无法由组件强制，
 * 只能在 review 时看。
 */

interface Props {
  variant?: 'default' | 'primary' | 'danger' | 'ghost'
  size?: 'sm' | 'md'
  /** 图标按钮：正方形，无内边距差异 */
  icon?: boolean
  disabled?: boolean
  /** 进行中。视觉上降透明度并禁用，但不改变尺寸 */
  loading?: boolean
  type?: 'button' | 'submit'
}

withDefaults(defineProps<Props>(), {
  variant: 'default',
  size: 'md',
  type: 'button',
})
</script>

<template>
  <button
    :type="type"
    class="btn-x"
    :class="[
      `btn-x--${variant}`,
      `btn-x--${size}`,
      { 'btn-x--icon': icon, 'btn-x--loading': loading },
    ]"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
  >
    <slot />
  </button>
</template>

<style scoped>
.btn-x {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);

  /* 可点区下限。视觉高度可以更小，但热区不行（宪法 6.3 节） */
  min-height: 28px;

  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);
  color: var(--color-text);

  /* font-family: inherit 是必须的。省掉则 Windows 上回落到系统 UI 字体，
     与正文不同源——「界面是拼出来的」那种感觉的来源之一 */
  font-family: inherit;
  font-weight: var(--weight-medium);
  line-height: 1;

  cursor: pointer;
  user-select: none;

  transition: background var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard),
    transform var(--dur-instant) var(--ease-standard),
    opacity var(--dur-instant) var(--ease-standard);
}

.btn-x--sm {
  padding: 4px 10px;
  font-size: var(--text-xs);
}

.btn-x--md {
  padding: 6px 12px;
  font-size: var(--text-sm);
}

.btn-x--icon {
  padding: 0;
  width: 28px;
}

.btn-x:hover:not(:disabled) {
  background: var(--color-surface-2);
  border-color: var(--color-border-str);
}

/* 按下微反馈。transform 由合成器处理，不触发重排 */
.btn-x:active:not(:disabled) {
  transform: scale(0.97);
}

.btn-x:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* loading 与 disabled 视觉一致，但语义不同：
   前者是「正在做」，后者是「不能做」。光标据此区分。 */
.btn-x--loading {
  cursor: progress;
}

.btn-x--primary {
  background: var(--color-accent);
  color: var(--color-on-accent);
  border-color: var(--color-accent);
}

.btn-x--primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
}

.btn-x--danger {
  border-color: var(--state-danger);
  color: var(--state-danger);
  background: transparent;
}

.btn-x--danger:hover:not(:disabled) {
  background: var(--state-danger);
  color: var(--color-on-accent);
}

/* ghost：无边框无底色，用于顶栏、卡片角落等不宜再加边框的位置 */
.btn-x--ghost {
  border-color: transparent;
  background: transparent;
  color: var(--color-text-muted);
}

.btn-x--ghost:hover:not(:disabled) {
  background: var(--color-surface-2);
  border-color: transparent;
  color: var(--color-text);
}

/* 自绘控件必须自己实现焦点环。省掉就等于砸了无障碍（宪法 6.3 节）——
   而这件事在鼠标测试中永远暴露不出来。 */
.btn-x:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
</style>
