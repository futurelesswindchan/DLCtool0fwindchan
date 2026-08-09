<script setup lang="ts">
/**
 * 首页壳：获取清单的两个入口
 *
 * 侧栏把「本地导入」提到与在线搜索平级（宪法 3.4）。这不是新决定，
 * 而是把既有决策落到视觉上——该站网页端额度是 API 的 4~60 倍，
 * 本地导入对重度用户是主路径而非退路，而现状它躺在搜索页底部折叠区，
 * 视觉上就是个退路。
 *
 * 环境状态**不在**此处：它已收归 TopBar 唯一权威。
 * 原先侧栏底部也有一份，但那一份只在异常时出现（就绪时留空），
 * 而一个只在出事时才存在的东西无法让用户建立「去哪看状态」的预期；
 * 且它只有本页有，Library 与 Settings 两页没有，而状态是全局的。
 * TopBar 的指示灯三态全覆盖、跨页常驻、同样可点跳转，故留它一个。
 */

import { useLibraryStore } from '../../stores/library'
import { Ornament } from '../../components/ui'
import Sidebar from '../../components/layout/Sidebar.vue'
import SidebarSection from '../../components/layout/SidebarSection.vue'
import SidebarItem from '../../components/layout/SidebarItem.vue'
import PaneTransition from '../../components/layout/PaneTransition.vue'

const library = useLibraryStore()
</script>

<template>
  <Sidebar brand-icon="star">
    <template #brand>
      <!--
        LOGO 与「风兔盒」已由 TopBar 承担，此处不再重复（下轮待办 1.1）。
        留下的是 slogan 与署名——它们在 TopBar 放不下，且属品牌标识，
        不是装饰性俏皮话，故常驻于此。
      -->
      <div class="brand">
        <Ornament pattern="ear" role="corner" corner="br" />
        <div class="brand__text">
          <span class="brand__slogan">请问您今天要来点 DLC 吗？</span>
          <span class="brand__by">POWERED BY futureless windchan</span>
        </div>
      </div>
    </template>

    <SidebarSection title="获取方式">
      <SidebarItem
        :to="{ name: 'search' }"
        label="在线搜索"
        meta="从社区源查找"
        icon="search"
        exact
      />
      <SidebarItem
        :to="{ name: 'import' }"
        label="本地导入"
        meta="已有清单包"
        icon="package"
      />
    </SidebarSection>

    <SidebarSection title="已入库" :count="library.items.length">
      <SidebarItem
        :to="{ name: 'library' }"
        label="查看全部"
        icon="library"
        :warning="library.hasAnomaly"
      />
    </SidebarSection>

  </Sidebar>

  <PaneTransition />
</template>

<style scoped>
/*
  侧栏品牌区。overflow: hidden 让角落纹样从右下切出画面外，
  不是歪在角上的一个完整图形——「露局部比完整摆中间高级」。
  position: relative 为 Ornament 的 absolute 定位提供基准。
*/
.brand {
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.brand__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* slogan 是品牌资产，给正文色与稍重的字重——它是这块区域的主角 */
.brand__slogan {
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  color: var(--color-text);
}

/*
  署名。字距放开一点让全大写读起来像标记而非句子，
  与 SidebarSection 标题同一手法。
*/
.brand__by {
  font-size: 10px;
  letter-spacing: 0.04em;
  color: var(--color-text-dim);
}
</style>
