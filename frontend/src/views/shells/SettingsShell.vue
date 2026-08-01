<script setup lang="ts">
/**
 * 设置壳
 *
 * ⚠️ 本步（第 3 步）刻意**不拆** `SettingsView`（627 行、四个 block）。
 *
 * 理由：拆它属宪法第 4 步「三页迁进壳」的工作量，而第 3 步的职责是
 * 结构性的路由改造——把 627 行的拆分与路由嵌套混进同一个 commit，
 * 回退时就分不开了，而第 3 步是六步里唯一有回退风险的一步。
 *
 * 过渡期做法：侧栏用锚点滚动而非子路由。用户能到达四个小节（这是方案 A
 * 的要求：中间态不得功能退化），但地址栏不变。
 * 第 4 步换成 `/settings/:section` 子路由时，只需把 SidebarItem 的
 * `@click` 换成 `to`，侧栏结构一行不动——这是刻意留的接缝。
 *
 * 小节标题 id 与 `SettingsView` 内的 `<section :id>` 对应，改名须同步两处。
 */

import { ref } from 'vue'
import Sidebar from '../../components/layout/Sidebar.vue'
import SidebarSection from '../../components/layout/SidebarSection.vue'
import SidebarItem from '../../components/layout/SidebarItem.vue'
import PaneTransition from '../../components/layout/PaneTransition.vue'

/**
 * 与 SettingsView 中的 `<section :id>` 一一对应。
 *
 * 顺序与现状一致（环境在最前），不按宪法 3.6 的「外观 / 清单源 / 关于 / 调试」
 * 重排——本步不动 SettingsView 内部，此处若重排会导致点第一项跳到页面中段，
 * 观感像是跳错了。重排与「调试」独立成节都归第 4 步。
 */
const sections = [
  { id: 'env', label: '环境', icon: '🔌' },
  { id: 'sources', label: '清单源', icon: '🌐' },
  { id: 'appearance', label: '外观', icon: '🎨' },
  { id: 'about', label: '关于与诊断', icon: 'ℹ' },
] as const

const current = ref<string>('env')

/**
 * 滚动到对应小节。
 *
 * 用 scrollIntoView 而非改 hash：hash 会进历史栈，用户按返回键会在四个
 * 小节间来回跳，而他的意图通常是离开设置页。
 *
 * behavior 跟随系统减弱动效偏好——这是位移类动效，宪法 5.6 要求真正禁用
 * 而非只缩短时长。
 */
function goto(id: string) {
  current.value = id
  const el = document.getElementById(id)
  if (!el) return

  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  el.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' })
}
</script>

<template>
  <Sidebar>
    <template #brand>
      <span class="title">设置</span>
    </template>

    <SidebarSection>
      <SidebarItem
        v-for="s in sections"
        :key="s.id"
        :label="s.label"
        :icon="s.icon"
        :class="{ 'is-current': current === s.id }"
        @click="goto(s.id)"
      />
    </SidebarSection>
  </Sidebar>

  <PaneTransition />
</template>

<style scoped>
.title {
  font-size: var(--text-base);
  font-weight: var(--weight-semibold);
  color: var(--color-text);
}

/*
  锚点态不是路由态，故 SidebarItem 的 item--active 不会生效，
  这里补一份等效表现。第 4 步换成子路由后这段连同 is-current 一起删掉。
  取值必须与 SidebarItem 的 item--active 一致，否则两页选中态长得不一样。
*/
.is-current {
  background: var(--color-raised);
  color: var(--color-text);
  font-weight: var(--weight-medium);
}
</style>
