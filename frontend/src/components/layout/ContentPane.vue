<script setup lang="ts">
/**
 * 内容区容器
 *
 * 负责滚动、留白与最大宽度。三页共用，故这三件事只在这里定义一次——
 * 现状是 `SearchView` 与 `GameView` 各自写 `max-width: 860px; margin: 0 auto`，
 * 那种「每页自己记得对齐」的做法迟早对不齐。
 *
 * 宽度放宽至 1040px（宪法 3.9）：侧栏已占 240px，仍限 860 会让宽屏右侧空一大块。
 *
 * `wide` 用于确实需要铺满的内容（试下载对比表），它不该被 1040 卡住。
 */

defineProps<{
  /** 取消最大宽度限制，铺满可用区域 */
  wide?: boolean;
  /** 去掉内边距。用于自带边距的内容，避免双层留白 */
  flush?: boolean;
}>();
</script>

<template>
  <main class="pane" :class="{ 'pane--flush': flush }">
    <div class="pane__inner" :class="{ 'pane__inner--wide': wide }">
      <slot />
    </div>
  </main>
</template>

<style scoped>
.pane {
  flex: 1 1 auto;
  /* flex 项默认 min-width:auto，不写这条内容会把侧栏挤窄 */
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-5);
  /*
    打通高度继承链：改为 flex column 容器，让子级 .pane__inner 可以用
    flex:1 撑满整个可视高度。纯 block 容器下 min-height:100% 的百分比
    参照需要父级有明确 height 值，而 overflow-y:auto 的滚动容器里
    子级 100% 会无限循环；改 flex column 后子级直接参与弹性分配，
    规避该问题。
  */
  display: flex;
  flex-direction: column;
}

.pane--flush {
  padding: 0;
}

.pane__inner {
  max-width: var(--content-max-w);
  /* 居中而非靠左：靠左在宽屏下会让内容与窗口右缘拉出极不平衡的空白 */
  margin: 0 auto;
  /*
    flex:1 让 .pane__inner 在 flex-column 父级中占满剩余高度，
    同时 display:flex + flex-direction:column 把弹性语义继续向下传递——
    LibraryOverviewPane 的 .pane（min-height:100%）与 .empty-full（flex:1）
    都依赖这条链路才能正确撑满。
  */
  flex: 1;
  display: flex;
  flex-direction: column;
  /* wide 变体下 max-width:none，其余样式不变 */
  width: 100%;
}

.pane__inner--wide {
  max-width: none;
}
</style>
