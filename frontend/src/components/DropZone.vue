<template>
  <div class="drop-zone-container" :class="{ 'has-file': !!fileName }">
    <!-- Steam 路径配置面板 (重构) -->
    <div class="steam-config flat-block">
      <div class="steam-header">
        <div class="steam-status" :class="steamPath ? 'status-ok' : 'status-error'">
          <div class="status-dot"></div>
          <span class="status-text">{{ steamPath ? 'Steam 已就绪' : '未找到 Steam' }}</span>
        </div>
        <button class="btn-text" @click="$emit('set-steam-path')">
          {{ steamPath ? '修改' : '手动指定' }}
        </button>
      </div>
      <div class="steam-path-detail" :title="steamPath || '需要手动指定'">
        {{ steamPath || '请手动配置 Steam 安装目录...' }}
      </div>
    </div>

    <!-- 拖拽核心区 -->
    <div 
      class="drag-area flat-block"
      :class="{ 'is-dragover': isDragOver }"
      @dragover.prevent="isDragOver = true"
      @dragleave.prevent="isDragOver = false"
      @drop.prevent="handleDrop"
    >
      <div class="drag-content">
        <div class="icon-wrapper">
          <span class="icon">{{ fileName ? '📦' : '📥' }}</span>
        </div>
        
        <h3 class="title">
          {{ fileName ? fileName : '拖拽 DLC 解锁包至此' }}
        </h3>
        
        <p class="subtitle">
          {{ fileName ? '随时拖入新 .zip 无缝替换' : '支持 .zip 格式压缩包' }}
        </p>

        <button 
          class="btn-select flat-block" 
          @click="$emit('file-select')"
          :disabled="!steamPath || isProcessing"
        >
          手动选择文件
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  steamPath: string
  isProcessing: boolean
  fileName: string | null
}>()

const emit = defineEmits<{
  (e: 'file-drop', file: File): void
  (e: 'file-select'): void
  (e: 'set-steam-path'): void
}>()

const isDragOver = ref(false)

const handleDrop = (event: DragEvent) => {
  isDragOver.value = false
  const files = event.dataTransfer?.files
  if (files && files.length > 0) {
    emit('file-drop', files[0])
  }
}
</script>

<style scoped>
.drop-zone-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 1.5rem;
  gap: 1.5rem;
}

/* --- Steam 配置区 --- */
.steam-config {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background-color: var(--bg-card);
}

.steam-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.steam-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  font-weight: 600;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-ok { color: var(--accent-success); }
.status-ok .status-dot { background-color: var(--accent-success); box-shadow: 0 0 8px var(--accent-success); }

.status-error { color: var(--accent-danger); }
.status-error .status-dot { background-color: var(--accent-danger); box-shadow: 0 0 8px var(--accent-danger); }

.btn-text {
  background: transparent;
  border: none;
  color: var(--accent-primary);
  font-size: 0.8rem;
  font-weight: 500;
  cursor: pointer;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.btn-text:hover {
  background-color: rgba(59, 130, 246, 0.1);
}

.steam-path-detail {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-family: ui-monospace, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  background-color: rgba(0, 0, 0, 0.1);
  padding: 0.4rem 0.6rem;
  border-radius: 4px;
}
body.light-theme .steam-path-detail {
  background-color: rgba(0, 0, 0, 0.03);
}

/* --- 拖拽区 --- */
.drag-area {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed var(--border-light);
  background-color: transparent;
  transition: all 0.3s var(--anim-smooth);
}

.drag-area.is-dragover {
  border-color: var(--accent-primary);
  background-color: rgba(59, 130, 246, 0.05);
  transform: scale(1.02);
}

.drag-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  z-index: 1;
  padding: 1rem;
  width: 100%;
}

.icon-wrapper {
  font-size: 3rem;
  margin-bottom: 0.5rem;
  transition: transform 0.3s var(--anim-smooth);
}

.drag-area:hover .icon-wrapper {
  transform: translateY(-5px);
}

.title {
  font-size: 1.125rem;
  color: var(--text-main);
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 1rem;
}

.subtitle {
  font-size: 0.875rem;
  color: var(--text-muted);
}

.btn-select {
  margin-top: 1rem;
  padding: 0.75rem 1.5rem;
  color: var(--text-main);
  font-weight: 600;
  cursor: pointer;
  border: 1px solid var(--border-light);
}

.btn-select:hover:not(:disabled) {
  background-color: var(--bg-card-hover);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

.btn-select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
