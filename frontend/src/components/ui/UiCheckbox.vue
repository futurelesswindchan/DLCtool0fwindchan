<script setup lang="ts">
/**
 * 复选框
 *
 * ⚠️ 内部必须是真 <input type="checkbox">，只是 appearance: none 后自绘视觉。
 *    不要改成 div + role="checkbox"——那样键盘、表单、读屏全部要手写，
 *    且几乎必然做不全（宪法 6.3 节）。
 *
 * 现状欠账：DlcList 用的原生 checkbox 可点区只有 13px。
 * 本组件把热区撑到 28px（整个 label 可点），视觉方框仍是 15px。
 * 密集列表尤其需要这条——DLC 列表动辄两百行，13px 的靶子会让人频繁点空。
 *
 * indeterminate 用于「全选」处于部分选中的状态。它无法由 HTML 属性表达，
 * 只能经 DOM property 设置，故用 :indeterminate 绑定而非 attribute。
 */

interface Props {
  modelValue?: boolean
  /** 部分选中态。优先于 modelValue 的视觉呈现 */
  indeterminate?: boolean
  disabled?: boolean
  label?: string
}

defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function onChange(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).checked)
}
</script>

<template>
  <label class="cb" :class="{ 'cb--disabled': disabled }">
    <input
      class="cb__input"
      type="checkbox"
      :checked="modelValue"
      :indeterminate="indeterminate"
      :disabled="disabled"
      @change="onChange"
    />

    <span class="cb__box" aria-hidden="true">
      <!-- 勾与横线共用一个 svg，靠 CSS 切换显隐。
           分成两个元素会在 indeterminate 切换时闪一下。 -->
      <svg class="cb__check" viewBox="0 0 12 12">
        <path d="M2.5 6.2 4.8 8.5 9.5 3.8" />
      </svg>
      <svg class="cb__dash" viewBox="0 0 12 12">
        <path d="M3 6h6" />
      </svg>
    </span>

    <span v-if="label || $slots.default" class="cb__label">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

<style scoped>
.cb {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);

  /* 热区下限 28px。视觉方框只有 15px，差额由这里补足——
     整个 label 都可点，不必精确瞄准小方块。 */
  min-height: 28px;

  cursor: pointer;
  user-select: none;
}

.cb--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 真 input 仍在 DOM 里，只是视觉上让位给自绘方框。
   不用 display: none——那会让它从 tab 序列里消失，键盘用户直接失去这个控件。 */
.cb__input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  margin: 0;
  pointer-events: none;
}

.cb__box {
  position: relative;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 15px;
  height: 15px;

  border: 1px solid var(--color-border-str);
  /* 15px 方框配 4px 圆角。再大就开始像圆点，再小就显得生硬 */
  border-radius: var(--radius-chip);
  background: var(--color-surface);

  transition: background var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard);
}

.cb__check,
.cb__dash {
  position: absolute;
  width: 12px;
  height: 12px;
  fill: none;
  stroke: var(--color-on-accent);
  stroke-width: 1.8;
  /* 端点圆头，与统一图形词汇一致（宪法 7.7 节） */
  stroke-linecap: round;
  stroke-linejoin: round;

  opacity: 0;
  transform: scale(0.6);
  transition: opacity var(--dur-instant) var(--ease-decelerate),
    transform var(--dur-instant) var(--ease-decelerate);
}

.cb__input:checked + .cb__box,
.cb__input:indeterminate + .cb__box {
  background: var(--color-accent);
  border-color: var(--color-accent);
}

.cb__input:checked + .cb__box .cb__check,
.cb__input:indeterminate + .cb__box .cb__dash {
  opacity: 1;
  transform: scale(1);
}

/* indeterminate 优先于 checked：全选框在部分选中时应显示横线而非勾，
   即使 modelValue 恰好为 true */
.cb__input:indeterminate + .cb__box .cb__check {
  opacity: 0;
  transform: scale(0.6);
}

.cb:hover:not(.cb--disabled) .cb__box {
  border-color: var(--color-accent);
}

.cb__label {
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
}

/* 焦点环挂在自绘方框上而非隐藏的 input 上 */
.cb__input:focus-visible + .cb__box {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
</style>
