<script setup lang="ts">
/**
 * 在线搜索（首页默认 Pane）
 *
 * 搜索结果不进 store：属会话级临时数据，无其他页面需要读取，放在全局
 * 反而要额外考虑何时清理。
 *
 * 本地导入已于第 3 步搬去 `panes/ImportPane.vue`，并在侧栏获得与在线搜索
 * 平级的常驻入口（宪法 3.4）。它并非退路——该站网页端额度是 API 的
 * 4~60 倍，对重度用户而言手动下载再导入反而是更划算的主路径，
 * 而躺在本页底部折叠区时它「视觉上就是个退路」。
 *
 * 滚动、内边距与最大宽度由 `layout/ContentPane` 提供，本组件不再自己限宽。
 */

import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  searchGames,
  type GameSearchResult,
} from '../api'
import { useLibraryStore } from '../stores/library'
import { useToast } from '../composables/useToast'
import GameCard from '../components/GameCard.vue'

const router = useRouter()
const library = useLibraryStore()
const toast = useToast()

const term = ref('')
const results = ref<GameSearchResult[]>([])
const searching = ref(false)
const searched = ref(false)

/** 关键词为空或正在搜索时，禁用搜索按钮。 */
const canSearch = computed(() => !!term.value.trim() && !searching.value)

/**
 * 发起搜索。由按钮点击或回车触发，不再随输入自动执行。
 *
 * 改为显式触发的理由是网络现实而非交互偏好：Steam 商店接口在国内经常
 * 以 `wsarecv: An existing connection was forcibly closed` 中断，而输入
 * 即搜会把「打一个词」放大成多次失败请求——实测输入 monster 期间日志里
 * 出现了 5 次搜索失败。用户看到连续 5 条报错，合理的结论是「工具坏了」，
 * 而实际只是自己还没打完字。
 *
 * 显式触发把请求次数交回用户，失败与操作也就一一对应，归因清晰。
 *
 * NOTE: 纯数字 AppID 的直查分支在后端 SearchGames 内部处理，前端只有这
 * 一个入口，无需为两种输入分别安排触发时机。
 */
async function runSearch() {
  const q = term.value.trim()
  if (!q || searching.value) return

  searching.value = true
  try {
    results.value = await searchGames(q)
    searched.value = true
  } catch (e) {
    toast.fromError(e, '搜索失败')
  } finally {
    searching.value = false
  }
}

/**
 * 清空关键词与结果。
 *
 * 自动搜索取消后，清空输入不再自动收起结果列表，需要一个显式出口——
 * 否则用户想回到初始状态只能刷新页面。
 */
function clearSearch() {
  term.value = ''
  results.value = []
  searched.value = false
}

function openGame(appID: string) {
  router.push({ name: 'game', params: { appID } })
}
</script>

<template>
  <div class="page">
    <div class="search">
      <input
        v-model="term"
        class="search__input"
        type="search"
        placeholder="请搜索游戏本体的简体中文名或 AppID"
        autofocus
        @keydown.enter="runSearch()"
      />
      <button
        class="search__btn"
        type="button"
        :disabled="!canSearch"
        @click="runSearch()"
      >
        {{ searching ? '搜索中…' : '搜索' }}
      </button>
      <button
        v-if="term || searched"
        class="search__clear"
        type="button"
        :disabled="searching"
        title="清空"
        @click="clearSearch()"
      >
        清空
      </button>
    </div>

    <p class="tips">
      结果只列出游戏本体，DLC 与试玩版已自动排除——清单以整个游戏为单位提供，
      单独搜 DLC 名找不到东西。搜索走 Steam 官方接口，
      <strong>大陆网络通常需要开启加速工具</strong>（UU、Steam++ 之类均可）。
    </p>

    <ul v-if="results.length" class="results">
      <li v-for="r in results" :key="r.appID">
        <GameCard
          layout="row"
          :app-i-d="r.appID"
          :name="r.name"
          :cover="r.headerImage"
          :installed="library.installedIDs.has(r.appID)"
          @click="openGame(r.appID)"
        />
      </li>
    </ul>

    <p v-else-if="searched && !searching" class="empty">
      没找到匹配的游戏。可以试试直接输入 AppID，或从左侧的「本地导入」进入。
    </p>

  </div>
</template>

<style scoped>
/* 限宽与居中已上移至 layout/ContentPane，此处不再重复定义 */
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

/* 输入框与按钮同排。改按钮驱动后不再需要 relative 定位的加载指示器，
   加载态直接由按钮文案承担——它就在用户刚点击的位置，比右侧的省略号
   更容易被注意到。 */
.search {
  display: flex;
  gap: var(--space-2);
  align-items: stretch;
}

.search__btn,
.search__clear {
  flex: 0 0 auto;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  color: var(--color-text);
  font-family: inherit;
  font-size: 0.95rem;
  cursor: pointer;
  white-space: nowrap;
}

.search__btn {
  border-color: var(--color-accent);
  background: var(--color-accent);
  color: var(--color-bg);
  min-width: 6em;
}

.search__btn:disabled,
.search__clear:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.search__btn:not(:disabled):hover,
.search__clear:not(:disabled):hover {
  filter: brightness(1.1);
}

.search__input {
  flex: 1 1 auto;
  min-width: 0;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  color: var(--color-text);
  font-family: inherit;
  font-size: 0.95rem;
}

.search__input:focus {
  border-color: var(--color-accent);
  outline: none;
}

.results {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
  /* 跳过视口外条目的渲染工作，足以替代虚拟滚动 */
  content-visibility: auto;
}

.results :deep(.card) {
  width: 100%;
}

.empty {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.85rem;
}

.tips {
  margin: 0;
  color: var(--color-text-dim);
  font-size: 0.76rem;
  line-height: 1.7;
}

.tips strong {
  color: var(--color-warning);
  font-weight: 500;
}

</style>
