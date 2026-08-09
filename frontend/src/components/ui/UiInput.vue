<script setup lang="ts">
/**
 * 单行输入框
 *
 * 现状欠账：设置页的 API key 与 Steam 路径用原生 input，
 * 其焦点环、内边距、圆角全部来自浏览器默认值，与设计语言不同源。
 *
 * mono 形态用于路径、AppID、密钥这类「要被复制的字符串」——
 * 等宽字体能让人一眼看出字符边界，粘错时也更容易发现。
 * 这类内容还必须允许文本选中，故 user-select 在此不做限制。
 *
 * invalid 由调用方给出，组件不自行校验：校验规则属业务，
 * 塞进原语就会让 ui/ 依赖业务知识，破坏「复制到另一个项目就能跑」（宪法 11.1）。
 */

interface Props {
  modelValue?: string
  type?: 'text' | 'password' | 'search'
  placeholder?: string
  disabled?: boolean
  readonly?: boolean
  /** 校验失败态。视觉由原语管，规则由调用方判 */
  invalid?: boolean
  /** 等宽形态。路径 / AppID / 密钥用 */
  mono?: boolean
  size?: 'sm' | 'md'
  /**
   * 挂载后自动聚焦。
   *
   * 必须做成 prop 而非依赖 attrs 透传：本组件的根元素是 `<div>` 外壳，
   * 透传来的 `autofocus` 会落在那个 div 上——而 div 不可聚焦，该属性
   * 静默失效，模板里写了却没有效果。凡「只对特定元素有意义的原生属性」，
   * 在带外壳的原语上都得显式转发到内部元素。
   */
  autofocus?: boolean
}

withDefaults(defineProps<Props>(), { type: 'text', size: 'md' })

const emit = defineEmits<{
  'update:modelValue': [value: string]
  enter: []
}>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}
</script>

<template>
  <div class="ipt" :class="[`ipt--${size}`, { 'ipt--invalid': invalid }]">
    <span v-if="$slots.prefix" class="ipt__affix">
      <slot name="prefix" />
    </span>

    <input
      class="ipt__el"
      :class="{ 'ipt__el--mono': mono }"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :autofocus="autofocus"
      :aria-invalid="invalid || undefined"
      @input="onInput"
      @keydown.enter="emit('enter')"
    />

    <span v-if="$slots.suffix" class="ipt__affix">
      <slot name="suffix" />
    </span>
  </div>
</template>

<style scoped>
.ipt {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;

  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);

  transition: border-color var(--dur-instant) var(--ease-standard),
    box-shadow var(--dur-instant) var(--ease-standard);
}

.ipt--sm {
  padding: 3px 8px;
  min-height: 28px;
}

.ipt--md {
  padding: 5px 10px;
  min-height: 32px;
}

.ipt:hover:not(:has(.ipt__el:disabled)) {
  border-color: var(--color-border-str);
}

/* 焦点态挂在外壳上。原生 input 的 outline 已被下方 outline: none 去掉，
   改由外壳的边框加光环表达——这样前后缀图标也一起被框住，视觉上是一个整体。 */
.ipt:focus-within {
  border-color: var(--color-accent);
  /* 用 box-shadow 而非 outline：outline 不跟随圆角，
     在 8px 圆角上会露出方角，正是那种说不出的廉价感 */
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-accent) 28%, transparent);
}

.ipt--invalid {
  border-color: var(--state-danger);
}

.ipt--invalid:focus-within {
  border-color: var(--state-danger);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--state-danger) 28%, transparent);
}

.ipt__el {
  flex: 1 1 auto;
  min-width: 0;

  border: none;
  background: transparent;
  color: var(--color-text);

  font-family: inherit;
  font-size: var(--text-sm);
  line-height: var(--leading-tight);

  /* 外壳已接管焦点表达，这里必须去掉，否则会出现双层焦点环 */
  outline: none;
}

.ipt__el--mono {
  font-family: var(--font-mono);
  /* 路径与密钥要被复制，等宽数字让字符边界更清晰 */
  font-variant-numeric: tabular-nums;
}

.ipt__el::placeholder {
  color: var(--color-text-dim);
}

.ipt__el:disabled {
  cursor: not-allowed;
  color: var(--color-text-dim);
}

/* 清掉 search 类型在 WebKit 下自带的清除按钮——它是原生控件，
   带着系统自己的样式，属于宪法第 6 章要消灭的对象 */
.ipt__el::-webkit-search-cancel-button {
  appearance: none;
}

.ipt__affix {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  color: var(--color-text-dim);
  font-size: var(--text-sm);
}
</style>
