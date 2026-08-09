<script setup lang="ts">
/**
 * 开关
 *
 * 内部是真 <input type="checkbox"> 加 role="switch"——语义上开关就是复选框，
 * 差别只在「立即生效」这层含义。
 *
 * 与 UiCheckbox 的分工（用错了会误导用户）：
 *   UiCheckbox  用于「选中若干项，稍后一起提交」，如 DLC 勾选
 *   UiSwitch    用于「拨一下立刻生效」，如源启用、主题切换
 *
 * 拨杆是本项目唯一允许出现全圆形状的控件之二（另一个是 UiRadio）：
 * 圆形拨杆是开关的通行视觉语言，改成方的反而会让人认不出。
 */

interface Props {
  modelValue?: boolean
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
  <label class="sw" :class="{ 'sw--disabled': disabled }">
    <input
      class="sw__input"
      type="checkbox"
      role="switch"
      :checked="modelValue"
      :disabled="disabled"
      @change="onChange"
    />

    <span class="sw__track" aria-hidden="true">
      <span class="sw__knob" />
    </span>

    <span v-if="label || $slots.default" class="sw__label">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

<style scoped>
.sw {
  display: inline-flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 28px;
  cursor: pointer;
  user-select: none;
}

.sw--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sw__input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  margin: 0;
  pointer-events: none;
}

.sw__track {
  position: relative;
  flex: 0 0 auto;
  width: 34px;
  height: 18px;
  border-radius: 9px;
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-str);
  transition: background var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard);
}

.sw__knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--color-text-muted);

  /* 只过渡 transform，不过渡 left——后者会触发重排（宪法 14 节） */
  transition: transform var(--dur-instant) var(--ease-standard),
    background var(--dur-instant) var(--ease-standard);
}

.sw__input:checked + .sw__track {
  background: var(--color-accent);
  border-color: var(--color-accent);
}

.sw__input:checked + .sw__track .sw__knob {
  transform: translateX(16px);
  background: var(--color-on-accent);
}

.sw:hover:not(.sw--disabled) .sw__track {
  border-color: var(--color-accent);
}

.sw__label {
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
}

.sw__input:focus-visible + .sw__track {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}
</style>
