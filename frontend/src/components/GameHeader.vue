<template>
  <div class="game-header flat-block">
    <div class="game-title-row">
      <h2 class="game-name">{{ gameData?.gameName || "载入中..." }}</h2>
      <span class="game-id">AppID: {{ gameData?.mainAppID || "---" }}</span>
    </div>

    <div class="stats-row">
      <div class="stat-item">
        <span class="stat-label">总 DLC 数</span>
        <span class="stat-value">{{ gameData?.dlcs.length || 0 }}</span>
      </div>
      <div class="divider"></div>
      <div class="stat-item">
        <span class="stat-label">已安装</span>
        <span class="stat-value text-success">{{ installedCount }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { main } from "../../wailsjs/go/models";

defineProps<{
  gameData: main.GamePackage | null;
  installedCount: number;
}>();
</script>

<style scoped>
.game-header {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border-radius: 0 0 var(--radius-md) var(--radius-md);
  border-top: none;
  border-left: none;
  border-right: none;
  background-color: var(--bg-panel);
  z-index: 10;
}

.game-title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}

.game-name {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text-main);
  margin: 0;
}

.game-id {
  font-size: 0.875rem;
  color: var(--text-muted);
  font-family: ui-monospace, monospace;
}

.stats-row {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  background-color: var(--bg-card);
  padding: 0.75rem 1.25rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-light);
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.stat-label {
  font-size: 0.875rem;
  color: var(--text-muted);
}

.stat-value {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-main);
}

.text-success {
  color: var(--accent-success);
}

.divider {
  width: 1px;
  height: 20px;
  background-color: var(--border-light);
}
</style>
