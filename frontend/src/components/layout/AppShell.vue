<script setup lang="ts">
/**
 * 应用外壳：顶栏 + 全局横幅 + 三板斧的承载区
 *
 * 从 `App.vue` 拆出来的理由不是文件太长，而是职责分层——`App.vue` 现在
 * 只管启动编排（拉后端状态、挂反馈宿主），骨架长什么样归本组件。
 *
 * 本组件**不含侧栏**。侧栏归各 Shell 所有，因为三页的侧栏内容完全不同，
 * 且它必须与内容区同处一个不重建的父级（见 UI-ARCHITECTURE 11.4）。
 * 本组件只保证「顶栏在上、其余撑满」这一层。
 */

import { useUiStore } from '../../stores/ui'
import TopBar from '../TopBar.vue'

const ui = useUiStore()
</script>

<template>
  <div class="shell">
    <TopBar />

    <div v-if="ui.banner" class="banner" :class="`banner--${ui.banner.kind}`">
      <span class="banner__text">{{ ui.banner.message }}</span>
      <button
        v-if="ui.banner.actionText"
        class="banner__action"
        @click="ui.banner.onAction?.()"
      >
        {{ ui.banner.actionText }}
      </button>
      <button class="banner__close" aria-label="关闭" @click="ui.clearBanner()">
        ✕
      </button>
    </div>

    <!--
      三板斧的承载区。这里刻意不加 padding 也不加 overflow——
      侧栏须贴边且自行滚动，内容区的留白与滚动由 ContentPane 负责。
      在此加 padding 会让侧栏浮在半空，那是最容易露馅的一处。
    -->
    <div class="shell__body">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  /* 子级各自滚动，外壳永不滚——否则顶栏会跟着内容一起滑走 */
  overflow: hidden;
}

.shell__body {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
}

.banner {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--color-border);
  font-size: var(--text-xs);
  flex: 0 0 auto;
}

.banner--warn {
  background: color-mix(in srgb, var(--color-warning) 16%, var(--color-bg));
  color: var(--color-warning);
}

.banner--error {
  background: color-mix(in srgb, var(--color-danger) 16%, var(--color-bg));
  color: var(--color-danger);
}

.banner--info,
.banner--success {
  background: color-mix(in srgb, var(--color-accent) 14%, var(--color-bg));
  color: var(--color-accent);
}

.banner__text {
  flex: 1 1 auto;
}

.banner__action,
.banner__close {
  border: none;
  background: transparent;
  color: inherit;
  font-family: inherit;
  font-size: inherit;
  cursor: pointer;
}

.banner__action {
  text-decoration: underline;
}
</style>
