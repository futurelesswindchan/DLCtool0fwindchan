<template>
  <div 
    class="dlc-card flat-block"
    :class="{ 'is-selected': isSelected, 'is-installed': dlc.isInstalled }"
    :style="{ '--card-index': index }"
    @click="$emit('toggle', dlc.appID)"
  >
    <!-- 左侧状态指示器 (替代 Checkbox) -->
    <div class="status-indicator">
      <div class="indicator-inner"></div>
    </div>

    <!-- 中间信息区 -->
    <div class="dlc-info">
      <h4 class="dlc-name" :title="dlc.name">{{ dlc.name }}</h4>
      <span class="dlc-id">{{ dlc.appID }}</span>
    </div>

    <!-- 右侧徽章区 -->
    <div class="dlc-badges">
      <span v-if="dlc.isInstalled" class="badge badge-success">已安装</span>
      <span v-if="dlc.hasKey" class="badge badge-primary">有密钥</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { main } from '../../wailsjs/go/models'

defineProps<{
  dlc: main.DLCInfo
  isSelected: boolean
  index: number
}>()

defineEmits<{
  (e: 'toggle', appID: string): void
}>()
</script>

<style scoped>
.dlc-card {
  display: flex;
  align-items: center;
  padding: 1rem 1.25rem;
  cursor: pointer;
  gap: 1rem;
  
  /* 瀑布流入场动画 */
  animation: slideUpFade 0.5s var(--anim-smooth) both;
  animation-delay: calc(var(--card-index) * 0.04s);
}

.dlc-card:hover {
  background-color: var(--bg-card-hover);
  transform: translateY(-2px);
}

/* 选中状态的高亮反馈 */
.dlc-card.is-selected {
  border-color: var(--accent-primary);
  background-color: rgba(59, 130, 246, 0.05);
}

.dlc-card.is-selected::before {
  /* 选中后保留一丝扫光底色 */
  left: 0;
  background: linear-gradient(120deg, transparent, rgba(59, 130, 246, 0.1), transparent);
}

/* --- 状态指示器 (替代原生 Checkbox) --- */
.status-indicator {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: 2px solid var(--border-light);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s var(--anim-smooth);
  flex-shrink: 0;
  z-index: 1;
}

.indicator-inner {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background-color: var(--accent-primary);
  transform: scale(0);
  transition: transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.dlc-card.is-selected .status-indicator {
  border-color: var(--accent-primary);
}

.dlc-card.is-selected .indicator-inner {
  transform: scale(1);
}

/* --- 信息区 --- */
.dlc-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  z-index: 1;
}

.dlc-name {
  font-size: 0.95rem;
  font-weight: 500;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dlc-id {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* --- 徽章区 --- */
.dlc-badges {
  display: flex;
  gap: 0.5rem;
  z-index: 1;
}

.badge {
  font-size: 0.7rem;
  padding: 0.2rem 0.5rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
}

.badge-success {
  background-color: rgba(16, 185, 129, 0.15);
  color: var(--accent-success);
}

.badge-primary {
  background-color: rgba(59, 130, 246, 0.15);
  color: var(--accent-hover);
}
</style>
