<script setup lang="ts">
/**
 * 下拉选择
 *
 * 这是自绘清单里最必要的一个：**深色主题下 Windows 原生 select 会弹出
 * 白色列表，直接把主题打穿**——用户点一下就看到一块刺眼的白，
 * 而这件事在浅色主题下测不出来。
 *
 * 用原生 popover 属性白捡三个行为：顶层渲染（不被 overflow 裁切）、
 * 点击外部关闭、Esc 关闭。这三样自己实现都要小心处理事件顺序与焦点归还，
 * 而浏览器已经做对了。
 *
 * 定位交给 useAnchoredLayer（约 90 行），不引 floating-ui——
 * 理由见该文件顶部注释。
 *
 * ⚠️ frameless 窗口下浮层是否被窗口边界裁切，需实机验证一次
 *    （宪法第 13 章风险表）。若被裁，退路是把 popover 换成
 *    手动 position: fixed + 自行处理点外关闭。
 */

import { ref, computed, nextTick, onUnmounted } from 'vue'
import { useAnchoredLayer } from '../../composables/useAnchoredLayer'

import type { SelectOption } from './types'

interface Props {
  modelValue?: string | number
  options: readonly SelectOption[]
  placeholder?: string
  disabled?: boolean
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  placeholder: '请选择',
})

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

const open = ref(false)
const anchorEl = ref<HTMLElement | null>(null)
const layerEl = ref<HTMLElement | null>(null)

const { style, update, bind, unbind } = useAnchoredLayer({ matchWidth: true })

const selected = computed(() =>
  props.options.find((o) => o.value === props.modelValue),
)

function recompute() {
  update(anchorEl.value, layerEl.value)
}

async function toggle() {
  if (props.disabled) return

  open.value = !open.value
  if (!open.value) {
    unbind(recompute)
    return
  }

  // 必须等浮层挂载后再测量：仍处于 v-if 未渲染时 getBoundingClientRect
  // 全返回 0，向上翻转的判断会失效
  await nextTick()
  recompute()
  bind(recompute)
}

function pick(o: SelectOption) {
  if (o.disabled) return
  emit('update:modelValue', o.value)
  open.value = false
  unbind(recompute)
}

/**
 * 键盘操作。原生 select 自带这些行为，自绘后必须补回来，
 * 否则键盘用户拿不到这个控件（宪法 6.3 节第一条）。
 */
function onKeydown(e: KeyboardEvent) {
  if (props.disabled) return

  if (e.key === 'Escape' && open.value) {
    open.value = false
    unbind(recompute)
    return
  }

  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    void toggle()
    return
  }

  // 上下键在闭合状态下直接切换选项，与原生 select 行为一致
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const list = props.options.filter((o) => !o.disabled)
    if (!list.length) return

    const i = list.findIndex((o) => o.value === props.modelValue)
    const next =
      e.key === 'ArrowDown'
        ? list[Math.min(i + 1, list.length - 1)]
        : list[Math.max(i - 1, 0)]
    emit('update:modelValue', next.value)
  }
}

// 组件卸载时若浮层仍开着，监听必须解掉，否则留下悬空回调
onUnmounted(() => unbind(recompute))
</script>

<template>
  <div class="sel" :class="`sel--${size}`">
    <button
      ref="anchorEl"
      type="button"
      class="sel__trigger"
      :class="{ 'sel__trigger--open': open }"
      :disabled="disabled"
      role="combobox"
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click="toggle"
      @keydown="onKeydown"
    >
      <span class="sel__value" :class="{ 'sel__value--empty': !selected }">
        {{ selected ? selected.label : placeholder }}
      </span>

      <svg class="sel__arrow" viewBox="0 0 12 12" aria-hidden="true">
        <path d="M3 4.8 6 7.8 9 4.8" />
      </svg>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        ref="layerEl"
        class="sel__layer"
        :class="{ 'sel__layer--flipped': style.flipped }"
        :style="{
          top: style.top,
          left: style.left,
          minWidth: style.minWidth,
        }"
        role="listbox"
      >
        <button
          v-for="o in options"
          :key="o.value"
          type="button"
          class="sel__opt"
          :class="{
            'sel__opt--active': o.value === modelValue,
            'sel__opt--disabled': o.disabled,
          }"
          role="option"
          :aria-selected="o.value === modelValue"
          :disabled="o.disabled"
          @click="pick(o)"
        >
          <span class="sel__opt-label">{{ o.label }}</span>
          <span v-if="o.hint" class="sel__opt-hint">{{ o.hint }}</span>
        </button>
      </div>
    </Teleport>

    <!-- 点击外部关闭。用一层透明遮罩而非 document 监听：
         document 监听要处理「点击触发器本身」的重入，容易出现点一下关了又开。
         遮罩挂在浮层之下、层级之上，天然只捕获外部点击。 -->
    <Teleport to="body">
      <div v-if="open" class="sel__backdrop" @click="toggle" />
    </Teleport>
  </div>
</template>

<style scoped>
.sel {
  display: inline-block;
  width: 100%;
}

.sel__trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  width: 100%;

  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);
  color: var(--color-text);

  font-family: inherit;
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
  text-align: left;

  cursor: pointer;
  transition: border-color var(--dur-instant) var(--ease-standard);
}

.sel--sm .sel__trigger {
  padding: 3px 8px;
  min-height: 28px;
}

.sel--md .sel__trigger {
  padding: 5px 10px;
  min-height: 32px;
}

.sel__trigger:hover:not(:disabled),
.sel__trigger--open {
  border-color: var(--color-accent);
}

.sel__trigger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sel__trigger:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

.sel__value {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.sel__value--empty {
  color: var(--color-text-dim);
}

.sel__arrow {
  flex: 0 0 auto;
  width: 12px;
  height: 12px;
  fill: none;
  stroke: var(--color-text-muted);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform var(--dur-fast) var(--ease-standard);
}

.sel__trigger--open .sel__arrow {
  transform: rotate(180deg);
}

/* ---- 浮层 ---- */

.sel__backdrop {
  position: fixed;
  inset: 0;
  z-index: 980;
}

.sel__layer {
  position: fixed;
  z-index: 990;

  max-height: 280px;
  overflow-y: auto;
  padding: var(--space-1);

  border: 1px solid var(--color-border);
  /* 浮层是 elev-2 层。同心圆角：外层 12px、内边距 4px，
     故选项圆角取 8px（见下方 .sel__opt） */
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-2), var(--hairline-top);

  /* 从锚点方向展开，回答「从哪来」（宪法 5.4 节第一条）。
     transform-origin 随翻转方向切换，向上翻转时从底边长出。 */
  transform-origin: top center;
  animation: layer-in var(--dur-fast) var(--ease-decelerate);
}

.sel__layer--flipped {
  transform-origin: bottom center;
}

/* 这是循环之外唯一用 animation 的地方：入场是一次性的，
   没有「中途被打断需要反向回退」的语义（宪法 11.6 节第三条）。 */
@keyframes layer-in {
  from {
    opacity: 0;
    transform: scale(0.97);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.sel__opt {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  width: 100%;
  padding: 6px 8px;
  min-height: 28px;

  border: none;
  /* 同心：外层 --radius-card(12px) − 内边距 --space-1(4px) = 8px */
  border-radius: var(--radius-ctrl);
  background: transparent;
  color: var(--color-text);

  font-family: inherit;
  font-size: var(--text-sm);
  text-align: left;

  cursor: pointer;
  transition: background var(--dur-instant) var(--ease-standard);
}

.sel__opt:hover:not(:disabled) {
  background: var(--color-surface-2);
}

/* 选中项只用底色与字重区分，不上主色——
   下拉列表里的「已选」是事实陈述，不是「下一步该点这个」的引导 */
.sel__opt--active {
  background: var(--color-surface-2);
  font-weight: var(--weight-medium);
}

.sel__opt--disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.sel__opt:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: -2px;
}

.sel__opt-hint {
  flex: 0 0 auto;
  color: var(--color-text-dim);
  font-size: var(--text-xs);
}
</style>
