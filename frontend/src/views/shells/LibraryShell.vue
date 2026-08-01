<script setup lang="ts">
/**
 * 已安装壳：master-detail
 *
 * 最大收益不是视觉，而是「换游戏」不再需要往返（宪法 3.3）——
 * 切游戏只换右半边，侧栏列表与滚动位置保持不动。
 *
 * 同时让「卸载后留在原页」那条既有决策落得更自然：根本没有「原页」可离开，
 * 复位天然发生在内容区。
 *
 * 侧栏三条约束（宪法 3.5）：
 *   1. 必须可筛选——装上百个游戏时纯列表不可用
 *   2. 必须分组——「本工具管理」与「外部清单」，后者不提供 DLC 编辑
 *   3. 未选中时内容区不得空白——显示库概览，空态是引导位不是留白位
 */

import { onMounted, computed, ref } from 'vue'
import { useLibraryStore } from '../../stores/library'
import Sidebar from '../../components/layout/Sidebar.vue'
import SidebarSection from '../../components/layout/SidebarSection.vue'
import SidebarItem from '../../components/layout/SidebarItem.vue'
import PaneTransition from '../../components/layout/PaneTransition.vue'
import { UiInput } from '../../components/ui'

const library = useLibraryStore()

onMounted(() => library.refresh())

const filter = ref('')

/** 大小写与空白均不敏感——用户从 Steam 复制游戏名常带尾随空格。 */
const matched = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return library.items
  return library.items.filter(
    (i) =>
      i.gameName.toLowerCase().includes(q) || i.mainAppID.includes(q),
  )
})

/** 本工具管理：有账本记录，可点进去编辑 DLC */
const managed = computed(() => matched.value.filter((i) => i.record))

/** 外部清单或账本丢失：只读，只提供删除 */
const external = computed(() => matched.value.filter((i) => !i.record))

function metaOf(item: (typeof library.items)[number]) {
  const r = item.record
  if (!r) return item.fileNames.join('、')
  return `${r.dlcCount} 个 DLC`
}
</script>

<template>
  <Sidebar>
    <template #brand>
      <UiInput
        v-model="filter"
        type="search"
        placeholder="筛选游戏名或 AppID"
        size="sm"
      />
    </template>

    <SidebarSection title="本工具管理" :count="managed.length">
      <SidebarItem
        v-for="item in managed"
        :key="item.mainAppID"
        :to="{ name: 'library-game', params: { appID: item.mainAppID } }"
        :label="item.gameName"
        :meta="metaOf(item)"
        :warning="item.conflicted"
      />
      <p v-if="!managed.length" class="none">
        {{ filter ? '没有匹配的游戏' : '还没有入库任何游戏' }}
      </p>
    </SidebarSection>

    <!--
      外部清单单列一组。它们同样在生效中（注入器加载目录内全部文件的并集），
      所以不能不显示；但不提供 DLC 编辑，故条目不可点进详情。
    -->
    <SidebarSection
      v-if="external.length"
      title="外部清单"
      :count="external.length"
    >
      <SidebarItem
        v-for="item in external"
        :key="item.mainAppID"
        :label="item.gameName"
        :meta="metaOf(item)"
        warning
      />
    </SidebarSection>

    <template #footer>
      <SidebarItem
        :to="{ name: 'library' }"
        label="库概览"
        icon="📊"
        exact
      />
    </template>
  </Sidebar>

  <PaneTransition />
</template>

<style scoped>
.none {
  margin: var(--space-2);
  color: var(--color-text-dim);
  font-size: var(--text-xs);
}
</style>
