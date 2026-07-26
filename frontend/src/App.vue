<script setup lang="ts">
/**
 * 应用根组件 · v2.0 最小骨架
 *
 * v1.4 的界面已整体移除，此处仅保留一个可编译、可启动的空壳，
 * 用于在后端模块全部完成并联调通过后再进行界面设计。
 *
 * NOTE: 后端 API 的绑定文件位于 wailsjs/go/main/App.js，
 * 由 Wails 在构建时自动生成——后端方法签名变更后需重新生成，
 * 不要手动编辑那些文件。
 */
</script>

<template>
  <div class="app">
    <main class="placeholder">
      <h1 class="placeholder__title">风兔盒</h1>
      <p class="placeholder__subtitle">请问您今天要来点 DLC 吗？</p>
      <p class="placeholder__hint">v2.0 界面施工中</p>
    </main>
  </div>
</template>

<style scoped>
.app {
  display: flex;
  height: 100vh;
  align-items: center;
  justify-content: center;
}

.placeholder {
  text-align: center;
  user-select: none;
}

.placeholder__title {
  margin: 0;
  font-size: 2.5rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: var(--color-text);
}

.placeholder__subtitle {
  margin: 0.75rem 0 0;
  font-size: 1rem;
  color: var(--color-text-muted);
}

.placeholder__hint {
  margin: 2rem 0 0;
  font-size: 0.8rem;
  color: var(--color-text-dim);
}
</style>
