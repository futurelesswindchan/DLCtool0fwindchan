<script setup lang="ts">
/**
 * 顶栏：Logo + 导航页签 + 环境状态
 *
 * 环境状态常驻此处——它需要长期可见，但不应占用内容区空间。
 *
 * NOTE: 当前使用系统标题栏（frameless 留待正式上线前评估），因此本组件
 * 不含窗口控制按钮，也无需处理 --wails-draggable。改为 frameless 时，
 * 须给此处所有可交互元素显式声明 no-drag，否则按钮会失灵。
 */

import { useRouter } from 'vue-router'
import { useEnvStore } from '../stores/env'

const env = useEnvStore()
const router = useRouter()

const tabs = [
  { name: 'search', label: '搜索' },
  { name: 'library', label: '已安装' },
  { name: 'settings', label: '设置' },
] as const

const indicatorText: Record<string, string> = {
  ready: 'OST 就绪',
  missing: '未检测到 OST',
  nopath: 'Steam 路径未设置',
}

/** 点击指示灯跳向能解决问题的页面，就绪时无动作。 */
function onIndicatorClick() {
  if (env.indicator === 'missing') router.push({ name: 'setup' })
  else if (env.indicator === 'nopath') router.push({ name: 'settings' })
}
</script>

<template>
  <header class="topbar">
    <div class="topbar__brand">
      <span class="topbar__logo" aria-hidden="true">🐰</span>
      <span class="topbar__name">风兔盒</span>
    </div>

    <nav class="topbar__nav">
      <RouterLink
        v-for="t in tabs"
        :key="t.name"
        :to="{ name: t.name }"
        class="nav-tab"
        active-class="nav-tab--active"
      >
        {{ t.label }}
      </RouterLink>
    </nav>

    <button
      class="env"
      :class="`env--${env.indicator}`"
      :disabled="env.indicator === 'ready'"
      :title="env.result?.message || ''"
      @click="onIndicatorClick"
    >
      <span class="env__dot" aria-hidden="true"></span>
      {{ indicatorText[env.indicator] }}
    </button>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  gap: var(--space-5);
  height: 46px;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg-elevated);
  flex: 0 0 auto;
}

.topbar__brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: 600;
}

.topbar__logo {
  font-size: 1.1rem;
}

.topbar__nav {
  display: flex;
  gap: var(--space-1);
}

.nav-tab {
  position: relative;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  font-size: 0.85rem;
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out);
}

.nav-tab:hover {
  background: var(--color-bg-hover);
  color: var(--color-text);
}

.nav-tab--active {
  color: var(--color-text);
}

/* 当前项下划线。用 transform 缩放而非改宽度，避免触发重排 */
.nav-tab--active::after {
  content: '';
  position: absolute;
  left: var(--space-3);
  right: var(--space-3);
  bottom: 2px;
  height: 2px;
  border-radius: 1px;
  background: var(--color-accent);
  animation: tab-in var(--duration-base) var(--ease-out);
}

@keyframes tab-in {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}

.env {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 0.78rem;
  font-family: inherit;
  cursor: pointer;
}

.env:disabled {
  cursor: default;
  opacity: 1;
}

.env:not(:disabled):hover {
  background: var(--color-bg-hover);
  color: var(--color-text);
}

.env__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.env--ready { color: var(--color-success); }
.env--missing { color: var(--color-warning); }
.env--nopath { color: var(--color-text-dim); }
</style>
