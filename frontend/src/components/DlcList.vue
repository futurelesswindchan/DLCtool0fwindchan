<script setup lang="ts">
/**
 * DLC 勾选列表
 *
 * 表意约定（见架构文档「DLC 列表的表意约定」）：
 *   ⚑  带独立 Depot，取消勾选可能导致 Steam 删除本地内容
 *   —  无独立 Depot，纯许可证，不占空间
 *
 * NOTE: 不展示 fileSize 为「下载体积」。该字段语义随解析路径而异——Lua
 * 路径取 depot 内容总大小，MAU 路径只能退取 manifest 文件自身大小，
 * 二者相差几个数量级，当作体积展示必然误导用户。此处仅以 ⚑ / — 表达
 * 「是否占空间」这一确定可靠的信息。
 */

import type { DLCInfo } from '../api'
import type { SyncState } from '../composables/useDlcSelection'

defineProps<{
  dlcs: DLCInfo[]
  isSelected: (appID: string) => boolean
  syncState: SyncState
  /** Steam 未运行时，同步完成的文案改为「下次启动 Steam 后生效」 */
  steamRunning?: boolean
  /** 只读模式：外部清单无对应的清单包数据，无法得知可选项全貌 */
  readonly?: boolean
}>()

const emit = defineEmits<{ toggle: [dlc: DLCInfo] }>()

const syncText: Record<SyncState, string> = {
  idle: '',
  pending: '⋯ 待同步',
  syncing: '🔄 同步中',
  done: '✓ 已同步',
}
</script>

<template>
  <section class="dlc">
    <header class="dlc__head">
      <h2 class="dlc__title">DLC 列表（{{ dlcs.length }}）</h2>
      <Transition name="sync">
        <span v-if="syncState !== 'idle'" class="dlc__sync">
          {{
            syncState === 'done' && steamRunning === false
              ? '✓ 下次启动 Steam 后生效'
              : syncText[syncState]
          }}
        </span>
      </Transition>
    </header>

    <ul class="dlc__list">
      <li v-for="d in dlcs" :key="d.appID" class="row">
        <label class="row__label">
          <input
            type="checkbox"
            :checked="isSelected(d.appID)"
            :disabled="readonly"
            @change.prevent="emit('toggle', d)"
          />
          <span class="row__name">{{ d.name || d.appID }}</span>
        </label>

        <span class="row__appid">{{ d.appID }}</span>

        <span
          class="row__depot"
          :class="{ 'row__depot--none': !d.manifestID }"
          :title="d.manifestID ? '含独立内容分支，取消勾选后 Steam 可能删除本地文件' : '纯许可证，不占用磁盘空间'"
        >
          {{ d.manifestID ? '⚑' : '—' }}
        </span>
      </li>
    </ul>

    <p v-if="readonly" class="dlc__readonly">
      该清单不是本工具部署的，无法得知可选项全貌，因此不提供勾选。
    </p>
  </section>
</template>

<style scoped>
.dlc__head {
  display: flex;
  align-items: baseline;
  gap: var(--space-3);
  margin-bottom: var(--space-2);
}

.dlc__title {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 500;
}

.dlc__sync {
  margin-left: auto;
  color: var(--color-text-muted);
  font-size: 0.78rem;
}

.dlc__list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  content-visibility: auto;
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-bg-elevated);
  font-size: 0.85rem;
}

.row + .row {
  border-top: 1px solid var(--color-border);
}

.row:hover {
  background: var(--color-bg-hover);
}

.row__label {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 1 1 auto;
  min-width: 0;
  cursor: pointer;
}

.row__label:has(input:disabled) {
  cursor: default;
}

.row__name {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.row__appid {
  flex: 0 0 auto;
  color: var(--color-text-dim);
  font-family: var(--font-mono);
  font-size: 0.75rem;
}

.row__depot {
  flex: 0 0 auto;
  width: 18px;
  text-align: center;
  color: var(--color-warning);
}

.row__depot--none {
  color: var(--color-text-dim);
}

.dlc__readonly {
  margin: var(--space-2) 0 0;
  color: var(--color-text-muted);
  font-size: 0.78rem;
}

.sync-enter-active,
.sync-leave-active {
  transition: opacity var(--duration-base) var(--ease-out);
}

.sync-enter-from,
.sync-leave-to {
  opacity: 0;
}
</style>
