<template>
  <div class="action-bar flat-block">
    <!-- 进度条模式（当正在处理时展示） -->
    <div v-if="isProcessing || notification" class="progress-mode">
      <div v-if="isProcessing" class="progress-track">
        <div
          class="progress-fill"
          :style="{ width: progressPercent + '%' }"
        ></div>
      </div>
      <div class="status-message" :class="notification?.type">
        {{ notification?.message || "正在处理..." }}
      </div>
    </div>

    <!-- 正常操作模式 -->
    <div v-else class="actions-mode">
      <div class="left-actions">
        <button class="btn-text" @click="$emit('select-all')">全选</button>
        <span class="divider"></span>
        <button class="btn-text" @click="$emit('select-none')">全不选</button>
      </div>

      <div class="right-actions">
        <!-- 内联确认：清除按钮 -->
        <button
          class="btn-block btn-danger"
          :class="{ 'is-confirming': confirmClear }"
          @click="handleClearClick"
        >
          {{ confirmClear ? "点击确认清除" : `清除所有 (${totalCount})` }}
        </button>

        <button
          class="btn-block btn-primary"
          @click="handleInstallClick"
          :disabled="selectedCount === 0"
        >
          安装选中 ({{ selectedCount }})
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{
  selectedCount: number;
  totalCount: number;
  isProcessing: boolean;
  progressPercent: number;
  notification: { type: string; message: string } | null;
}>();

const emit = defineEmits<{
  (e: "select-all"): void;
  (e: "select-none"): void;
  (e: "clear-all"): void;
  (e: "install-selected"): void;
}>();

const confirmClear = ref(false);
let clearTimer: number | null = null;

const handleClearClick = () => {
  if (confirmClear.value) {
    emit("clear-all");
    confirmClear.value = false;
    if (clearTimer) clearTimeout(clearTimer);
  } else {
    confirmClear.value = true;
    clearTimer = window.setTimeout(() => {
      confirmClear.value = false;
    }, 3000);
  }
};

const handleInstallClick = () => {
  emit("install-selected");
};

// 离开处理状态时，重置确认状态
watch(
  () => props.isProcessing,
  (newVal) => {
    if (newVal) confirmClear.value = false;
  },
);
</script>

<style scoped>
.action-bar {
  padding: 1rem 1.5rem;
  border-radius: var(--radius-md) var(--radius-md) 0 0;
  border-bottom: none;
  border-left: none;
  border-right: none;
  background-color: var(--bg-panel);
  z-index: 10;
  min-height: 80px;
  display: flex;
  align-items: center;
}

.actions-mode {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.left-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.divider {
  width: 1px;
  height: 14px;
  background-color: var(--border-light);
}

.btn-text {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.5rem;
  font-size: 0.875rem;
  transition: color 0.2s;
}

.btn-text:hover {
  color: var(--text-main);
}

.right-actions {
  display: flex;
  gap: 1rem;
}

.btn-block {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.3s var(--anim-smooth);
}

.btn-block:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-block:active:not(:disabled) {
  transform: scale(0.95);
}

.btn-primary {
  background-color: var(--accent-primary);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background-color: var(--accent-hover);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.btn-danger {
  background-color: var(--bg-card);
  color: var(--accent-danger);
  border: 1px solid var(--border-light);
}

.btn-danger:hover {
  background-color: rgba(239, 68, 68, 0.1);
  border-color: var(--accent-danger);
}

.btn-danger.is-confirming {
  background-color: var(--accent-danger);
  color: white;
  animation: pulseGlow 1.5s infinite;
}

/* --- 进度条模式 --- */
.progress-mode {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  justify-content: center;
  animation: fadeIn 0.3s;
}

.progress-track {
  width: 100%;
  height: 6px;
  background-color: var(--bg-card);
  border-radius: 3px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(
    90deg,
    var(--accent-primary),
    var(--accent-success)
  );
  border-radius: 3px;
  transition: width 0.3s ease;
}

.status-message {
  font-size: 0.875rem;
  text-align: center;
  font-weight: 500;
}

.status-message.success {
  color: var(--accent-success);
}
.status-message.error {
  color: var(--accent-danger);
}
.status-message.info,
.status-message.progress {
  color: var(--text-main);
}
</style>
