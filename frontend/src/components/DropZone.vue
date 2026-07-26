<script setup lang="ts">
/**
 * 清单包拖拽导入区
 *
 * 同时支持拖拽与点击选择两条导入路径。
 *
 * NOTE: 必须对 dragover 调用 preventDefault，否则 WebView 会把拖入的
 * 文件当作导航请求直接打开，整个界面被文件内容替换。
 */

import { ref } from 'vue'

const props = defineProps<{
  busy: boolean
}>()

const emit = defineEmits<{
  dropFile: [file: File]
  pickFile: []
}>()

/** 是否有文件悬停在区域上方，用于高亮反馈。 */
const isDragging = ref(false)

function onDragOver(event: DragEvent) {
  event.preventDefault()
  if (!props.busy) isDragging.value = true
}

function onDragLeave() {
  isDragging.value = false
}

function onDrop(event: DragEvent) {
  event.preventDefault()
  isDragging.value = false

  if (props.busy) return

  const file = event.dataTransfer?.files?.[0]
  if (file) emit('dropFile', file)
}
</script>

<template>
  <div
    class="dropzone"
    :class="{
      'dropzone--active': isDragging,
      'dropzone--busy': busy,
    }"
    role="button"
    tabindex="0"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
    @click="!busy && emit('pickFile')"
    @keydown.enter="!busy && emit('pickFile')"
    @keydown.space.prevent="!busy && emit('pickFile')"
  >
    <p class="dropzone__title">
      {{ busy ? '正在解析清单包…' : '把清单包拖到这里' }}
    </p>
    <p class="dropzone__hint">或点击选择 .zip 文件</p>
  </div>
</template>

<style scoped>
.dropzone {
  display: grid;
  place-items: center;
  gap: var(--space-2);
  padding: var(--space-7) var(--space-4);
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-bg-elevated);
  cursor: pointer;
  text-align: center;
  user-select: none;

  /* 只过渡 border-color 与 transform：二者不触发重排。
     避免对 padding / height 做动画。 */
  transition:
    border-color var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.dropzone:hover {
  border-color: var(--color-border-strong);
}

.dropzone--active {
  border-color: var(--color-accent);
  transform: scale(1.01);
}

.dropzone--busy {
  cursor: progress;
  opacity: 0.6;
}

.dropzone__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 500;
}

.dropzone__hint {
  margin: 0;
  font-size: 0.8rem;
  color: var(--color-text-dim);
}
</style>
