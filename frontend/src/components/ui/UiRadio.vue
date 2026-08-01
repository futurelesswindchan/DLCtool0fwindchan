<script setup lang="ts">
/**
 * 单选框
 *
 * 与 UiCheckbox 同构：内部真 <input type="radio">，appearance: none 后自绘。
 * 不合并成一个组件——两者的 v-model 语义不同（布尔 vs 值相等），
 * 合并后 props 会出现互斥组合，反而更难用对。
 *
 * name 必须由调用方给出且同组一致，否则原生的「同组互斥」行为不成立。
 * 这一条无法由组件兜住，故列为必填 prop。
 */

interface Props {
  /** 当前选中值（v-model） */
  modelValue?: string | number
  /** 本项代表的值 */
  value: string | number
  /** 同组必须一致，否则互斥失效 */
  name: string
  disabled?: boolean
  label?: string
}

defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()
</script>

<template>
  <label class="rd" :class="{ 'rd--disabled': disabled }">
    <input
      class="rd__input"
      type="radio"
      :name="name"
      :value="value"
      :checked="modelValue === value"
      :disabled="disabled"
      @change="emit('update:modelValue', value)"
    />

    <span class="rd__ring" aria-hidden="true">
      <span class="rd__dot" />
    </span>

    <span v-if="label || $slots.default" class="rd__label">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

<style scoped>
.rd {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 28px;
  cursor: pointer;
  user-select: none;
}

.rd--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.rd__input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  margin: 0;
  pointer-events: none;
}

.rd__ring {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 15px;
  height: 15px;

  border: 1px solid var(--color-border-str);
  /* 单选框是唯一允许全圆的元素——它表达「互斥」这个语义，
     而全圆是这个语义的通行视觉。禁止全圆的规则针对按钮（宪法 4.4 节）。 */
  border-radius: 50%;
  background: var(--color-surface);

  transition: border-color var(--dur-instant) var(--ease-standard);
}

.rd__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-accent);

  opacity: 0;
  transform: scale(0.4);
  transition: opacity var(--dur-instant) var(--ease-decelerate),
    transform var(--dur-instant) var(--ease-decelerate);
}

.rd__input:checked + .rd__ring {
  border-color: var(--color-accent);
}

.rd__input:checked + .rd__ring .rd__dot {
  opacity: 1;
  transform: scale(1);
}

.rd:hover:not(.rd--disabled) .rd__ring {
  border-color: var(--color-accent);
}

.rd__label {
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
}

.rd__input:focus-visible + .rd__ring {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
</style>
