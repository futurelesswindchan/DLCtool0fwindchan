<script setup lang="ts">
/**
 * 游戏页
 *
 * 同一路由承担三种状态，由本地是否已有该游戏的清单包决定：
 *   A. 未入库          → 展示详情 + 三源查找进展 + 入库按钮
 *   B. 已入库且本会话有清单包 → DLC 勾选列表，勾选即生效
 *   C. 已入库但无清单包 → 只能重新获取或彻底卸载（见下方说明）
 *
 * 状态 C 的由来：后端 GetPackage 尚未实现，packages/ 目录不落盘，因此
 * 重启应用后无法还原已入库游戏的 DLC 可选项全貌。此处如实告知用户
 * 「重新获取以编辑」，而不是伪造一份不完整的列表让用户以为可以随意勾选。
 * TODO(后端 GetPackage): 该方法实现后，状态 C 可并入 B。
 */

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  getGameDetail,
  lookupRepos,
  downloadFromRepo,
  type GameDetail,
  type GamePackage,
} from '../api'
import { useLibraryStore } from '../stores/library'
import { useEnvStore } from '../stores/env'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import { useDlcSelection } from '../composables/useDlcSelection'
import DlcList from '../components/DlcList.vue'

const route = useRoute()
const router = useRouter()
const library = useLibraryStore()
const env = useEnvStore()
const toast = useToast()
const confirm = useConfirm()

const appID = computed(() => String(route.params.appID))

const detail = ref<GameDetail | null>(null)
const sources = ref<string[]>([])
const looking = ref(false)
const downloading = ref(false)
/** 下载进度事件推送的当前尝试源，用于让用户看到「正在做什么」 */
const progressText = ref('')

/** 本会话内已加载的清单包。仅当用户刚下载或刚导入时才有值。 */
const pkg = ref<GamePackage | null>(null)

const libItem = computed(() => library.find(appID.value))
const installed = computed(() => !!libItem.value)

/**
 * 勾选控制器。
 *
 * 在 setup 顶层一次性构造，pkg 为 null 时各项操作自行短路——因为
 * useDlcSelection 内部注册了 onUnmounted，放进 watch 回调会注册不到
 * 当前组件实例上，导致待落盘的改动在切页时静默丢失。
 */
const selection = useDlcSelection(pkg)

onMounted(async () => {
  await load()

  EventsOn('download:progress', (payload: any) => {
    if (payload?.appID && payload.appID !== appID.value) return
    const src = payload?.source ?? ''
    progressText.value = src ? `正在尝试 ${src}…` : '正在获取…'
  })
})

// 切页面不清理会导致重复注册，同一事件收到多份
onUnmounted(() => EventsOff('download:progress'))

/** 路由参数变化时重新加载（用户可能从已安装页直接跳到另一个游戏） */
watch(appID, () => {
  pkg.value = null
  void load()
})

async function load() {
  detail.value = null
  sources.value = []
  progressText.value = ''

  try {
    detail.value = await getGameDetail(appID.value)
  } catch (e) {
    toast.fromError(e, '获取游戏详情失败')
  }

  if (!installed.value) await lookup()
}

/** 查询三源收录情况。失败不阻断页面，仅提示。 */
async function lookup() {
  looking.value = true
  try {
    sources.value = await lookupRepos(appID.value)
  } catch (e) {
    toast.fromError(e, '查询清单源失败')
  } finally {
    looking.value = false
  }
}

/**
 * 从指定源入库。
 *
 * 部署由 useDlcSelection 在 pkg 就位后自动触发首次同步——下载得到的包
 * 默认全选，与本地导入路径保持一致的语义。
 */
async function install(sourceName: string) {
  if (!env.ready) {
    toast.warn('注入器尚未就绪，请先完成环境配置')
    router.push({ name: 'setup' })
    return
  }

  downloading.value = true
  progressText.value = `正在尝试 ${sourceName}…`
  try {
    pkg.value = await downloadFromRepo(appID.value, sourceName)
    // 下载得到的包默认全选，与本地导入路径保持一致的语义。
    // selectAll 会触发防抖落盘，完成首次部署。
    selection.selectAll()
    await library.refresh()
    toast.success(`已获取 ${pkg.value.gameName || appID.value} 的清单`)
  } catch (e) {
    toast.fromError(e, '获取清单失败')
  } finally {
    downloading.value = false
    progressText.value = ''
  }
}

/** 重新获取：用于状态 C，或用户想覆盖为新版清单。 */
async function reacquire() {
  if (!sources.value.length) await lookup()
  if (!sources.value.length) {
    toast.warn('三个清单源均未收录该游戏，可尝试本地导入清单包')
    return
  }
  await install(sources.value[0])
}

/**
 * 彻底卸载。
 *
 * NOTE: 存在外部 lua 也声明了同一 AppID 时，后端返回失败。这不是异常，
 * 而是如实告知「本工具的文件已删，但游戏可能仍留在库中」。故此处以
 * warn 而非 error 呈现，并保留后端列出的文件名。
 */
async function uninstall() {
  const ok = await confirm({
    title: `彻底卸载「${detail.value?.name || appID.value}」？`,
    body: [
      '将删除本工具部署的清单文件与安装记录。',
      'Steam 库中的条目可能需要重启 Steam 后才消失。',
    ],
    confirmText: '彻底卸载',
    danger: true,
  })
  if (!ok) return

  try {
    const msg = await library.remove(appID.value)
    pkg.value = null
    toast.success(msg)
    router.push({ name: 'library' })
  } catch (e: any) {
    // 外部声明导致的不彻底卸载，文案已由后端组装好，原样呈现
    toast.warn(e?.message ?? '卸载未完全成功')
    pkg.value = null
  }
}
</script>

<template>
  <div class="page">
    <button class="back" @click="router.back()">← 返回</button>

    <header class="hero">
      <img
        class="hero__cover"
        :src="detail?.headerImage || `https://cdn.cloudflare.steamstatic.com/steam/apps/${appID}/header.jpg`"
        :alt="detail?.name || appID"
      />
      <div class="hero__info">
        <h1 class="hero__name">{{ detail?.name || appID }}</h1>
        <p class="hero__meta">
          AppID {{ appID }}
          <template v-if="detail?.releaseDate"> · {{ detail.releaseDate }}</template>
          <template v-if="detail?.developers?.length">
            · {{ detail.developers.join('、') }}
          </template>
        </p>
        <p v-if="detail?.description" class="hero__desc">{{ detail.description }}</p>
      </div>
    </header>

    <!-- 状态 A：未入库 -->
    <section v-if="!installed && !pkg" class="block">
      <h2 class="block__title">清单源</h2>

      <p v-if="looking" class="hint">正在查询三个清单源…</p>

      <template v-else-if="sources.length">
        <p class="hint">{{ sources.length }} 个源收录了该游戏</p>
        <div class="actions">
          <button
            v-for="s in sources"
            :key="s"
            class="btn btn--primary"
            :disabled="downloading"
            @click="install(s)"
          >
            从 {{ s }} 入库
          </button>
        </div>
      </template>

      <template v-else>
        <p class="hint hint--warn">三个清单源均未收录该游戏。</p>
        <p class="hint">
          若已从其他渠道拿到清单包，可回到搜索页用底部的本地导入功能。
        </p>
        <div class="actions">
          <button class="btn" @click="lookup()">重新查询</button>
          <button class="btn" @click="router.push({ name: 'search' })">
            去本地导入
          </button>
        </div>
      </template>

      <p v-if="progressText" class="hint">{{ progressText }}</p>
    </section>

    <!-- 状态 B：已入库且本会话有清单包 -->
    <template v-if="pkg">
      <div class="actions">
        <button class="btn" @click="selection.selectAll()">全选</button>
        <button class="btn" @click="selection.selectNone()">全不选</button>
        <button class="btn" :disabled="downloading" @click="reacquire()">
          替换清单
        </button>
        <button class="btn btn--danger" @click="uninstall()">彻底卸载</button>
      </div>

      <DlcList
        :dlcs="pkg.dlcs"
        :is-selected="selection.isSelected"
        :sync-state="selection.syncState.value"
        @toggle="selection.toggle"
      />
    </template>

    <!-- 状态 C：已入库但本会话没有清单包 -->
    <section v-else-if="installed && !pkg" class="block">
      <h2 class="block__title">已入库</h2>
      <p class="hint">
        已部署清单文件：{{ libItem?.fileNames.join('、') }}
      </p>
      <p v-if="libItem?.record" class="hint">
        {{ libItem.record.dlcCount }} 个 DLC · 获取于
        {{ libItem.record.installedAt.slice(0, 16).replace('T', ' ') }}
      </p>
      <p v-if="libItem?.conflicted" class="hint hint--warn">
        该游戏同时被本工具之外的清单文件声明，卸载后可能仍留在库中。
      </p>
      <p class="hint">
        本次会话未加载清单内容，如需调整 DLC 勾选，请重新获取一次清单。
      </p>
      <div class="actions">
        <button class="btn btn--primary" :disabled="downloading" @click="reacquire()">
          重新获取以编辑
        </button>
        <button class="btn btn--danger" @click="uninstall()">彻底卸载</button>
      </div>
      <p v-if="progressText" class="hint">{{ progressText }}</p>
    </section>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  max-width: 860px;
  margin: 0 auto;
}

.back {
  align-self: flex-start;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  font-family: inherit;
  font-size: 0.82rem;
  cursor: pointer;
  padding: 0;
}

.back:hover {
  color: var(--color-text);
}

.hero {
  display: flex;
  gap: var(--space-4);
}

.hero__cover {
  flex: 0 0 auto;
  width: 230px;
  aspect-ratio: 460 / 215;
  object-fit: cover;
  border-radius: var(--radius-md);
  background: var(--color-bg-hover);
}

.hero__info {
  min-width: 0;
}

.hero__name {
  margin: 0 0 var(--space-1);
  font-size: 1.25rem;
}

.hero__meta {
  margin: 0 0 var(--space-2);
  color: var(--color-text-dim);
  font-size: 0.78rem;
}

.hero__desc {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.85rem;
  /* 简介可能很长，限制三行以免挤压下方的操作区 */
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.block {
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
}

.block__title {
  margin: 0 0 var(--space-3);
  font-size: 0.9rem;
  font-weight: 500;
}

.hint {
  margin: 0 0 var(--space-2);
  color: var(--color-text-muted);
  font-size: 0.82rem;
}

.hint--warn {
  color: var(--color-warning);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
