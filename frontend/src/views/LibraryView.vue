<script setup lang="ts">
/**
 * 已安装页
 *
 * 内容是历史账本与部署目录扫描结果的合并视图，因此这里能看到三类条目：
 * 本工具正常部署的、账本丢失但文件仍在的、以及完全来自外部的清单。
 *
 * 对账异常单列一区而非混在网格里——正确性问题需要用户主动处理，
 * 混在正常条目中会被忽略。
 */

import { onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useLibraryStore } from '../stores/library'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import GameCard from '../components/GameCard.vue'

const router = useRouter()
const library = useLibraryStore()
const toast = useToast()
const confirm = useConfirm()

onMounted(() => library.refresh())

/** 本工具管理的条目：有账本记录，可点进去编辑 */
const managed = computed(() => library.items.filter((i) => i.record))

/** 需用户知情的条目：外部清单，或账本丢失 */
const anomalies = computed(() => library.items.filter((i) => !i.record))

function subtitleOf(item: (typeof library.items)[number]) {
  const r = item.record
  if (!r) return ''
  const when = r.installedAt ? r.installedAt.slice(0, 10) : ''
  return `${r.dlcCount} 个 DLC${when ? ` · ${when} 获取` : ''}`
}

async function onUninstall(mainAppID: string, name: string) {
  const ok = await confirm({
    title: `彻底卸载「${name}」？`,
    body: [
      '将删除本工具部署的清单文件与安装记录。',
      'Steam 库中的条目可能需要重启 Steam 后才消失。',
    ],
    confirmText: '彻底卸载',
    danger: true,
  })
  if (!ok) return

  try {
    toast.success(await library.remove(mainAppID))
  } catch (e: any) {
    // 存在外部声明导致的不彻底卸载，后端文案已列出需手动处理的文件名
    toast.warn(e?.message ?? '卸载未完全成功')
  }
}
</script>

<template>
  <div class="page">
    <header class="head">
      <h1 class="head__title">已安装（{{ managed.length }}）</h1>
      <button class="btn" :disabled="library.loading" @click="library.refresh()">
        {{ library.loading ? '扫描中…' : '重新扫描' }}
      </button>
    </header>

    <p v-if="!library.items.length && !library.loading" class="hint">
      还没有入库任何游戏。到搜索页找一个吧。
    </p>

    <div v-if="managed.length" class="grid">
      <div v-for="item in managed" :key="item.mainAppID" class="grid__cell">
        <GameCard
          layout="grid"
          :app-i-d="item.mainAppID"
          :name="item.gameName"
          :subtitle="subtitleOf(item)"
          :warning="item.conflicted"
          @click="router.push({ name: 'game', params: { appID: item.mainAppID } })"
        />
        <p v-if="item.conflicted" class="cell__warn">
          该游戏同时被外部清单声明，卸载后可能仍留在库中
        </p>
      </div>
    </div>

    <!-- 对账异常：部署目录里存在本工具账本之外的清单 -->
    <section v-if="anomalies.length" class="block">
      <h2 class="block__title">部署目录中的其他清单（{{ anomalies.length }}）</h2>
      <p class="hint">
        这些文件位于清单目录中，但不在本工具的安装记录里——可能是手动放入的，
        或由其他工具产生。其中可能含特意配置的内容，本工具不会自动清理。
      </p>

      <ul class="anomaly">
        <li v-for="item in anomalies" :key="item.mainAppID" class="anomaly__row">
          <div class="anomaly__main">
            <span class="anomaly__file">{{ item.fileNames.join('、') }}</span>
            <span class="anomaly__tag">
              {{ item.hasExternal ? '外部清单' : '记录丢失' }}
            </span>
          </div>
          <button
            v-if="!item.hasExternal"
            class="btn btn--danger"
            @click="onUninstall(item.mainAppID, item.gameName)"
          >
            删除
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  max-width: 980px;
  margin: 0 auto;
}

.head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.head__title {
  margin: 0;
  font-size: 1.05rem;
}

.head .btn {
  margin-left: auto;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--space-3);
  content-visibility: auto;
}

.grid__cell :deep(.card) {
  width: 100%;
}

.cell__warn {
  margin: var(--space-1) 0 0;
  color: var(--color-warning);
  font-size: 0.74rem;
}

.block__title {
  margin: 0 0 var(--space-2);
  font-size: 0.9rem;
  font-weight: 500;
}

.hint {
  margin: 0 0 var(--space-3);
  color: var(--color-text-muted);
  font-size: 0.8rem;
}

.anomaly {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.anomaly__row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-bg-elevated);
  font-size: 0.83rem;
}

.anomaly__row + .anomaly__row {
  border-top: 1px solid var(--color-border);
}

.anomaly__main {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1 1 auto;
  min-width: 0;
}

.anomaly__file {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.anomaly__tag {
  flex: 0 0 auto;
  padding: 1px var(--space-2);
  border: 1px solid var(--color-warning);
  border-radius: var(--radius-sm);
  color: var(--color-warning);
  font-size: 0.7rem;
}
</style>
