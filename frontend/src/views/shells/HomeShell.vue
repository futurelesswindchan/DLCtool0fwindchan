<script setup lang="ts">
/**
 * 首页壳：获取清单的两个入口
 *
 * 侧栏把「本地导入」提到与在线搜索平级（宪法 3.4）。这不是新决定，
 * 而是把既有决策落到视觉上——该站网页端额度是 API 的 4~60 倍，
 * 本地导入对重度用户是主路径而非退路，而现状它躺在搜索页底部折叠区，
 * 视觉上就是个退路。
 *
 * 运行状态摘要放在侧栏底部：它需要长期可见，且点击能去到修复它的地方,
 * 属「入口」不属「结论」，不违反宪法 3.1。
 */

import { useEnvStore } from '../../stores/env'
import { useLibraryStore } from '../../stores/library'
import Sidebar from '../../components/layout/Sidebar.vue'
import SidebarSection from '../../components/layout/SidebarSection.vue'
import SidebarItem from '../../components/layout/SidebarItem.vue'
import PaneTransition from '../../components/layout/PaneTransition.vue'

const env = useEnvStore()
const library = useLibraryStore()
</script>

<template>
  <Sidebar>
    <template #brand>
      <div class="brand">
        <!-- LOGO 占位。第 5 步换真资产（宪法 7.7） -->
        <span class="brand__mark" aria-hidden="true">🐰</span>
        <div class="brand__text">
          <span class="brand__name">风兔盒</span>
          <span class="brand__tag">找清单、放对位置</span>
        </div>
      </div>
    </template>

    <SidebarSection title="获取方式">
      <SidebarItem
        :to="{ name: 'search' }"
        label="在线搜索"
        meta="从社区源查找"
        icon="🔍"
        exact
      />
      <SidebarItem
        :to="{ name: 'import' }"
        label="本地导入"
        meta="已有清单包"
        icon="📦"
      />
    </SidebarSection>

    <SidebarSection title="已入库" :count="library.items.length">
      <SidebarItem
        :to="{ name: 'library' }"
        label="查看全部"
        icon="📚"
        :warning="library.hasAnomaly"
      />
    </SidebarSection>

    <template #footer>
      <SidebarItem
        v-if="env.indicator !== 'ready'"
        :to="{ name: env.indicator === 'missing' ? 'setup' : 'settings' }"
        :label="env.indicator === 'missing' ? '未检测到 OST' : 'Steam 路径未设置'"
        meta="点此处理"
        icon="⚠"
        warning
      />
    </template>
  </Sidebar>

  <PaneTransition />
</template>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.brand__mark {
  font-size: var(--text-lg);
  line-height: 1;
}

.brand__text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.brand__name {
  font-size: var(--text-base);
  font-weight: var(--weight-semibold);
  color: var(--color-text);
}

.brand__tag {
  font-size: var(--text-xs);
  color: var(--color-text-dim);
}
</style>
