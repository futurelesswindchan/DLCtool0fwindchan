<script setup lang="ts">
/**
 * 清单包内容与 DLC 选择列表
 *
 * 展示解析结果，并让用户勾选要入库的 DLC。
 *
 * 性能考量：DLC 数量可达数百，列表容器使用 content-visibility: auto
 * 让浏览器跳过视口外条目的渲染，效果等同虚拟滚动但无需引入库。
 */

import { computed } from 'vue'
import type { GamePackage } from '../api'

const props = defineProps<{
  pkg: GamePackage
  selected: string[]
  busy: boolean
}>()

const emit = defineEmits<{
  'update:selected': [ids: string[]]
  install: []
  remove: []
}>()

/** 已勾选数量，用于按钮文案与全选状态判断。 */
const selectedCount = computed(() => props.selected.length)

const allSelected = computed(
  () =>
    props.pkg.dlcs.length > 0 &&
    props.selected.length === props.pkg.dlcs.length,
)

/** 已在系统中检测到的 DLC 数量，反映当前实际入库状态。 */
const installedCount = computed(
  () => props.pkg.dlcs.filter((d) => d.isInstalled).length,
)

function toggle(appID: string) {
  const next = props.selected.includes(appID)
    ? props.selected.filter((id) => id !== appID)
    : [...props.selected, appID]
  emit('update:selected', next)
}

function toggleAll() {
  emit(
    'update:selected',
    allSelected.value ? [] : props.pkg.dlcs.map((d) => d.appID),
  )
}
</script>

<template>
  <section class="package">
    <header class="package__header">
      <div>
        <h2 class="package__name">{{ pkg.gameName }}</h2>
        <p class="package__meta">
          AppID {{ pkg.mainAppID }} · DLC {{ pkg.dlcs.length }} 项 ·
          Depot {{ pkg.depots.length }} 项 ·
          已入库 {{ installedCount }} 项
        </p>
      </div>

      <div class="package__actions">
        <button
          type="button"
          class="btn btn--primary"
          :disabled="busy"
          @click="emit('install')"
        >
          入库 {{ selectedCount }} 项
        </button>
        <button
          type="button"
          class="btn btn--danger"
          :disabled="busy"
          @click="emit('remove')"
        >
          移除本游戏
        </button>
      </div>
    </header>

    <div v-if="pkg.dlcs.length" class="package__toolbar">
      <label class="check">
        <input
          type="checkbox"
          :checked="allSelected"
          :disabled="busy"
          @change="toggleAll"
        />
        <span>全选</span>
      </label>
    </div>

    <ul v-if="pkg.dlcs.length" class="dlc-list">
      <li v-for="dlc in pkg.dlcs" :key="dlc.appID" class="dlc">
        <label class="check check--block">
          <input
            type="checkbox"
            :checked="selected.includes(dlc.appID)"
            :disabled="busy"
            @change="toggle(dlc.appID)"
          />
          <span class="dlc__name">{{ dlc.name }}</span>
        </label>

        <span class="dlc__tags">
          <span class="tag tag--id">{{ dlc.appID }}</span>
          <span v-if="dlc.hasKey" class="tag tag--key">含密钥</span>
          <span v-if="dlc.isInstalled" class="tag tag--installed">已入库</span>
        </span>
      </li>
    </ul>

    <p v-else class="package__empty">
      此清单包未包含可选 DLC，入库将只注册主游戏。
    </p>
  </section>
</template>

<style scoped>
.package {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
}

.package__header {
  display: flex;
  gap: var(--space-4);
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
}

.package__name {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.package__meta {
  margin: var(--space-1) 0 0;
  font-size: 0.8rem;
  color: var(--color-text-muted);
}

.package__actions {
  display: flex;
  gap: var(--space-2);
}

.package__toolbar {
  padding-bottom: var(--space-2);
  border-bottom: 1px solid var(--color-border);
}

.package__empty {
  margin: 0;
  padding: var(--space-4);
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  color: var(--color-text-muted);
  font-size: 0.85rem;
}

.dlc-list {
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-y: auto;
  min-height: 0;

  /* 让浏览器跳过视口外条目的渲染工作。
     配合 contain-intrinsic-size 给出高度估值，避免滚动条抖动。 */
  content-visibility: auto;
  contain-intrinsic-size: auto 40px;
}

.dlc {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
}

.dlc:hover {
  background: var(--color-bg-hover);
}

.dlc__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dlc__tags {
  display: flex;
  flex-shrink: 0;
  gap: var(--space-1);
}

.tag {
  padding: 1px var(--space-2);
  border-radius: var(--radius-sm);
  font-size: 0.7rem;
  font-family: var(--font-mono);
  background: var(--color-bg-hover);
  color: var(--color-text-dim);
}

.tag--key {
  color: var(--color-warning);
}

.tag--installed {
  color: var(--color-success);
}
</style>
