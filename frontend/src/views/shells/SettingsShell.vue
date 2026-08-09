<script setup lang="ts">
/**
 * 设置壳
 *
 * 第 3 步用锚点滚动过渡，第 4 步已换为真子路由——`SidebarItem` 的 `@click`
 * 换成 `to` 即可，侧栏结构与样式一行没动。那个接缝是第 3 步刻意留的。
 *
 * 随之退场的三样东西：
 *   - `current` 本地状态（选中态改由 RouterLink 的 active-class 给出）
 *   - `goto()` 与 `scrollIntoView`
 *   - `.is-current` 那份「与 item--active 取值必须一致」的重复样式
 *
 * 最后一样是本步的实际收益：那种「两处取值必须记得同步」的约定，
 * 靠人守是守不住的。
 *
 * 拆开后每屏只讲一件事，且「清单源」终于有自己的地盘——用户自定义源
 * 那个长期议题有地方落了（宪法 3.6 / 第 9 章）。
 */

import Sidebar from '../../components/layout/Sidebar.vue'
import SidebarSection from '../../components/layout/SidebarSection.vue'
import SidebarItem from '../../components/layout/SidebarItem.vue'
import PaneTransition from '../../components/layout/PaneTransition.vue'

/**
 * 顺序即用户遇到问题的先后：环境不通则一切不可用，故排第一；
 * 源决定能拿到什么，排第二；外观是偏好；关于与诊断是出事时才来的地方。
 */
const sections = [
  { name: 'settings-env', label: '环境', icon: 'plug' },
  { name: 'settings-sources', label: '清单源', icon: 'globe' },
  { name: 'settings-appearance', label: '外观', icon: 'palette' },
  { name: 'settings-about', label: '关于与诊断', icon: 'info' },
] as const
</script>

<template>
  <Sidebar brand-icon="gear">
    <template #brand>
      <span class="title">设置</span>
    </template>

    <SidebarSection>
      <SidebarItem
        v-for="s in sections"
        :key="s.name"
        :to="{ name: s.name }"
        :label="s.label"
        :icon="s.icon"
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
</style>
