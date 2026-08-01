<script setup lang="ts">
/**
 * 游戏页
 *
 * 同一路由承担三种状态，由是否已有该游戏的清单包决定：
 *   A. 未入库            → 详情 + 三源查找进展 + 入库按钮
 *   B. 已入库且有留存清单 → DLC 勾选列表，勾选即生效
 *   C. 已入库但无留存清单 → 只能重新获取或彻底卸载
 *
 * 状态 C 现在只在少数情形出现：留存文件被手动删除、留存内容损坏，或该
 * 游戏是在留存功能上线前入库的。正常路径下入库即落盘，重启应用后直接
 * 进入状态 B。
 *
 * 清单时效的处理原则：只展示获取时间，不判定「过期」。清单旧不等于无效，
 * 多数情况下几个月前的清单依然可用；是否重新获取交由用户自行决定。
 */

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import {
  getGameDetail,
  trialSources,
  trialOneSource,
  installFromTrial,
  getPackage,
  findHistory,
  type GameDetail,
  type GamePackage,
  type TrialReport,
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

/**
 * 各源的试下载结果。替代原先只有源名的字符串列表。
 *
 * 改动的根因是「收录」与「有用」是两件事：probe 只能回答该源存在这个游戏
 * 的文件，而实测同一个游戏 7 个源全报收录、实得从 200 个 DLC 到 0 个。
 * 只给源名的话，用户唯一的选择方式是盲猜。
 */
const report = ref<TrialReport | null>(null)
const looking = ref(false)
const downloading = ref(false)

/** 正在手动试的认证型源名，空表示无。用于只禁用那一行的按钮。 */
const quotaTrying = ref('')
/** 下载进度事件推送的当前尝试源，用于让用户看到「正在做什么」 */
const progressText = ref('')

/** 当前加载的清单包。来自留存文件或本次会话的下载。 */
const pkg = ref<GamePackage | null>(null)

/** 清单的获取时刻，RFC 3339 字符串。空表示本次会话刚下载的。 */
const savedAt = ref('')

/** 清单的来源源名。空表示本地导入。 */
const pkgSource = ref('')

const libItem = computed(() => library.find(appID.value))
const installed = computed(() => !!libItem.value)

/**
 * 清单获取时间的人性化表述。
 *
 * 有意不说「已过期」——清单旧不等于无效，多数情况下几个月前的清单依然
 * 可用。只陈述事实，让用户自行判断是否值得重新获取。
 */
const savedAtText = computed(() => {
  if (!savedAt.value) return ''

  const then = new Date(savedAt.value)
  if (Number.isNaN(then.getTime())) return ''

  const days = Math.floor((Date.now() - then.getTime()) / 86_400_000)
  const when =
    days <= 0 ? '今天' : days === 1 ? '昨天' : `${days} 天前`
  return `清单获取于${when}（${savedAt.value.slice(0, 10)}）`
})

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

/**
 * 落盘完成后读回权威的获取时间。
 *
 * 不在前端自行填 Date.now()：清单留存由后端在部署时写入，前端猜的时间
 * 与实际落盘时刻可能不符。等 syncState 转 done 再读，可确保文件已存在。
 */
watch(
  () => selection.syncState.value,
  async (state) => {
    if (state !== 'done') return
    try {
      const stored = await getPackage(appID.value)
      if (stored?.savedAt) savedAt.value = stored.savedAt
      if (stored?.source !== undefined) pkgSource.value = stored.source
    } catch {
      // 读不回来只影响「获取于 X 天前」的显示，不值得打扰用户
    }
  },
)

async function load() {
  detail.value = null
  report.value = null
  progressText.value = ''
  savedAt.value = ''
  pkgSource.value = ''

  try {
    detail.value = await getGameDetail(appID.value)
  } catch (e) {
    toast.fromError(e, '获取游戏详情失败')
  }

  if (installed.value) await loadStored()
  else await lookup()
}

/**
 * 读取留存清单，还原上次的勾选状态。
 *
 * 勾选来自安装历史的 installedIDs 而非清单包本身——清单包记录的是「有哪些
 * DLC 可选」，历史记录的才是「用户选了哪些」。二者混淆会导致用户取消过的
 * DLC 在重启后又变回选中。
 *
 * 留存不存在时静默停留在状态 C，不弹错——那是正常处境（留存功能上线前
 * 入库的游戏），提示用户重新获取即可。
 */
async function loadStored() {
  try {
    const stored = await getPackage(appID.value)
    if (!stored?.package) return

    pkg.value = stored.package
    savedAt.value = stored.savedAt ?? ''
    pkgSource.value = stored.source ?? ''

    const record = libItem.value?.record ?? (await findHistory(appID.value))
    selection.restore(record?.installedIDs ?? [])
  } catch (e) {
    toast.fromError(e, '读取本地清单失败')
  }
}

/**
 * 对各源做试下载，得出实得 DLC 数的对比。失败不阻断页面，仅提示。
 *
 * 比原先的收录查询慢得多（要真下载并解析），但换来的是用户能看见差异。
 * 等待成本由缓存与「选定源后免二次下载」两处摊平。
 *
 * @param refresh 为真时忽略缓存强制重查
 */
async function lookup(refresh = false) {
  looking.value = true
  try {
    report.value = await trialSources(appID.value, refresh)
  } catch (e) {
    toast.fromError(e, '查询清单源失败')
  } finally {
    looking.value = false
  }
}

/**
 * 手动试某个认证型源。
 *
 * 独立成一个动作是因为它消耗用户自己申请的 API 额度——这笔开销必须由一次
 * 明确的点击表达，不能夹在自动流程里替用户花掉。
 */
async function tryQuotaSource(source: string) {
  if (!report.value) return

  const idx = report.value.trials.findIndex((t) => t.source === source)
  if (idx < 0) return

  quotaTrying.value = source
  try {
    const t = await trialOneSource(appID.value, source)
    // 就地替换该行，不整表重查——重查会把其他源也再跑一遍
    report.value.trials[idx] = t
    recomputeBest()
  } catch (e) {
    toast.fromError(e, `试 ${source} 失败`)
  } finally {
    quotaTrying.value = ''
  }
}

/**
 * 手动试完认证源后重算「实得最多」。
 *
 * 前端自行重算而非回后端取：后端的汇总是在整表试下载时算的，此处只多了
 * 一行数据，为它再走一次跨边界调用不值得。
 */
function recomputeBest() {
  if (!report.value) return

  let best = ''
  let max = 0
  for (const t of report.value.trials) {
    // 严格大于，与后端 summarizeTrials 保持一致：并列时保留在前者，
    // 使推荐结果稳定可复现
    if (t.status === 'ok' && t.dlcCount > max) {
      max = t.dlcCount
      best = t.source
    }
  }
  report.value.bestSource = best
  report.value.maxDLC = max
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
  progressText.value = `正在从 ${sourceName} 入库…`
  try {
    // 走 installFromTrial 而非 downloadFromRepo：试下载的产物已缓存，
    // 命中时零网络请求。这是那一轮等待的收益端，缺了它试下载就只是变慢。
    pkg.value = await installFromTrial(appID.value, sourceName)
    // 下载得到的包默认全选，与本地导入路径保持一致的语义。
    // selectAll 会触发防抖落盘，完成首次部署。
    selection.selectAll()
    await library.refresh()

    // 先乐观填上来源，让界面立即有信息可显示。权威的 savedAt 要等防抖
    // 落盘真正完成后才存在，故交由 watch 在同步状态转 done 时读回。
    pkgSource.value = sourceName
    savedAt.value = ''

    toast.success(`已获取 ${pkg.value.gameName || appID.value} 的清单`)
  } catch (e) {
    toast.fromError(e, '获取清单失败')
  } finally {
    downloading.value = false
    progressText.value = ''
  }
}

/**
 * 重新获取：用于状态 C，或用户想覆盖为新版清单。
 *
 * 不自动选源，只把对比表摆出来让用户自己挑。原实现取列表首位，等同于写死
 * 优先级，而实测存在反例——Kingdom Rush Vengeance (1367550) 上通常最优的
 * Hubcap 只给 2 个 DLC，快照源给 4 个。既然「最优源」因游戏而异，就没有
 * 任何固定顺序是对的。
 */
async function reacquire() {
  pkg.value = null
  await lookup(true)

  if (!report.value?.usableCount) {
    toast.warn('没有源能提供该游戏的清单，可尝试本地导入清单包')
  }
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
    toast.success(msg)
    await resetToUninstalled()
  } catch (e: any) {
    // 外部声明导致的不彻底卸载，文案已由后端组装好，原样呈现
    toast.warn(e?.message ?? '卸载未完全成功')
    await resetToUninstalled()
  }
}

/**
 * 卸载后把本页复位为「未入库」形态，而非跳转到库页面。
 *
 * 原实现跳回库列表，问题在于卸载常常是「装错了源、想换一个」的中间步骤，
 * 而不是终点。跳走之后用户得重新搜索、重新进详情页才能换源，且刚看过的
 * 源对比信息全部丢失。留在原页则卸载与重装是连续动作。
 *
 * 必须重新 lookup：状态 A 的界面依赖 report，而它在进入状态 B 后不会被
 * 刷新。不重查的话用户会看到一个「没有任何可用源」的空页面，与「卸载把
 * 源也弄坏了」难以区分。
 *
 * 不传 refresh：卸载后立刻换源是高频操作，此时试下载缓存通常仍有效，
 * 复用它可让「卸载 → 换源重装」几乎无等待。
 */
async function resetToUninstalled() {
  pkg.value = null
  savedAt.value = ''
  pkgSource.value = ''
  await lookup()
}
</script>

<template>
  <!--
    「← 返回」已于第 3 步删除。三板斧下侧栏常驻，用户点侧栏下一个候选即可，
    不必先退回列表（宪法 3.7）；且嵌套路由下 router.back() 会退出整个 Shell，
    行为与用户预期相反。
  -->
  <div class="page">
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

      <template v-if="looking">
        <p class="hint">正在逐个试取清单，这一步会实际下载并解析…</p>
        <p class="hint">
          比单纯查询「有没有」慢一些，但能看出各源实际能给多少 DLC。
          结果会缓存 30 分钟，稍后再来不必重等。
        </p>
      </template>

      <template v-else-if="report?.trials?.length">
        <p class="hint">
          已试 {{ report.trials.length }} 个源，其中
          <strong>{{ report.usableCount }}</strong> 个可用<template
            v-if="report.bestSource"
          >
            ，实得最多的是
            <strong>{{ report.bestSource }}</strong>（{{ report.maxDLC }} 个 DLC）</template
          >。
        </p>
        <p class="hint">
          各源内容差距可以很大，DLC 数少通常是那个源本身收录得少，不是本工具
          出错。下面的数字就是实际能装上的数量，按需自行选择。
        </p>

        <!-- 免额度源：已自动试取，直接给结果 -->
        <ul class="trials">
          <li
            v-for="t in report.trials.filter((x) => !x.needsQuota)"
            :key="t.source"
            class="trial"
            :class="`trial--${t.status}`"
          >
            <span class="trial__name">{{ t.source }}</span>
            <span class="trial__num">
              <template v-if="t.status === 'ok'">{{ t.dlcCount }} DLC</template>
              <template v-else-if="t.status === 'empty'">仅本体</template>
              <template v-else>—</template>
            </span>
            <span class="trial__msg">
              {{ t.message }}
              <template v-if="t.cached">（缓存）</template>
            </span>
            <button
              v-if="t.status === 'ok' || t.status === 'empty'"
              class="btn btn--primary trial__btn"
              :disabled="downloading"
              @click="install(t.source)"
            >
              用这个入库
            </button>
            <span v-else class="trial__btn trial__btn--none">不可用</span>
          </li>
        </ul>

        <!-- 认证型源：单列，需用户主动花额度 -->
        <template v-if="report.quotaSources.length">
          <h3 class="subtitle">需要 API 额度的源</h3>
          <p class="hint">
            这类源通常收录得最全，但每次获取都会消耗你自己申请的额度，
            因此不会自动试取。若上面的结果不满意，再来试这里。
          </p>
          <ul class="trials">
            <li
              v-for="t in report.trials.filter((x) => x.needsQuota)"
              :key="t.source"
              class="trial"
              :class="`trial--${t.status}`"
            >
              <span class="trial__name">{{ t.source }}</span>
              <span class="trial__num">
                <template v-if="t.status === 'ok'">{{ t.dlcCount }} DLC</template>
                <template v-else-if="t.status === 'empty'">仅本体</template>
                <template v-else>?</template>
              </span>
              <span class="trial__msg">{{ t.message }}</span>
              <button
                v-if="t.status === 'skipped' || t.status === 'failed'"
                class="btn trial__btn"
                :disabled="quotaTrying === t.source"
                @click="tryQuotaSource(t.source)"
              >
                {{ quotaTrying === t.source ? '获取中…' : '试这个源' }}
              </button>
              <button
                v-else-if="t.status === 'ok' || t.status === 'empty'"
                class="btn btn--primary trial__btn"
                :disabled="downloading"
                @click="install(t.source)"
              >
                用这个入库
              </button>
              <span v-else class="trial__btn trial__btn--none">不可用</span>
            </li>
          </ul>
        </template>

        <div class="actions">
          <button class="btn" :disabled="looking" @click="lookup(true)">
            全部重新试取
          </button>
          <button class="btn" @click="router.push({ name: 'search' })">
            改用本地导入
          </button>
        </div>
      </template>

      <template v-else>
        <p class="hint hint--warn">所有已启用的清单源都没有该游戏。</p>
        <p class="hint">
          这通常意味着社区尚未收录它，而非本工具或网络出错。若查询过程中
          出现过网络报错，也可以先点「重新查询」再看一次。
        </p>
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

    <!-- 状态 B：已入库且有留存清单 -->
    <template v-if="pkg">
      <div class="meta-bar">
        <span v-if="savedAtText">{{ savedAtText }}</span>
        <span v-if="pkgSource">来源 {{ pkgSource }}</span>
        <span v-else-if="pkg">来源 本地导入</span>
      </div>

      <div class="actions">
        <button class="btn" @click="selection.selectAll()">全选</button>
        <button class="btn" @click="selection.selectNone()">全不选</button>
        <button class="btn" :disabled="downloading" @click="reacquire()">
          重新获取清单
        </button>
        <button class="btn btn--danger" @click="uninstall()">彻底卸载</button>
      </div>

      <p class="hint hint--dim">
        勾选后约 1 秒自动写入，无需点保存。「重新获取清单」会从源下载最新版本
        并覆盖本地这一份——若当前一切正常，通常不必操作。
      </p>

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
        本地没有这个游戏的清单内容（可能是早期版本入库的，或留存文件已被删除），
        因此暂时无法调整 DLC 勾选。重新获取一次即可恢复编辑，之后就会一直保留。
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
/* 限宽与居中已上移至 layout/ContentPane（放宽至 1040px，宪法 3.9） */
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
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

.meta-bar {
  display: flex;
  gap: var(--space-4);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  background: var(--color-bg-elevated);
  color: var(--color-text-muted);
  font-size: 0.78rem;
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

/* ─── 源试取对比表 ───
   视觉设计留待下一轮统一重构，此处只保证「三种没结果视觉分家」这一
   功能要求：可用与不可用要一眼分清，否则用户仍会把源的贫瘠误判为故障。 */

.subtitle {
  margin: var(--space-4) 0 var(--space-2);
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--color-text);
}

.trials {
  list-style: none;
  margin: 0 0 var(--space-3);
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.trial {
  display: grid;
  /* 数字列定宽，使各行的 DLC 数上下对齐——对比表的核心就是纵向比较，
     宽度随内容浮动会让「哪个更多」需要逐行读数字才能看出 */
  grid-template-columns: minmax(8em, 1fr) 6em minmax(0, 2fr) auto;
  gap: var(--space-3);
  align-items: center;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-left-width: 3px;
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  font-size: 0.82rem;
}

/* 左边框色承担状态区分。用颜色而非图标是因为它不占宽度，
   且在密集列表里更易形成整体印象 */
.trial--ok {
  border-left-color: var(--color-accent);
}

.trial--empty {
  border-left-color: var(--color-warning);
}

/* unsupported 与 miss 都是「该源没有可用内容」，同色处理——
   对用户而言两者的处置方式相同（换源），无需在颜色上再作区分 */
.trial--unsupported,
.trial--miss {
  border-left-color: var(--color-text-dim);
  opacity: 0.7;
}

.trial--failed {
  border-left-color: var(--color-danger, #c0392b);
}

.trial--skipped {
  border-left-color: var(--color-text-muted);
  border-left-style: dashed;
}

.trial__name {
  font-weight: 500;
  color: var(--color-text);
  overflow-wrap: anywhere;
}

.trial__num {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--color-text);
  text-align: right;
}

.trial__msg {
  color: var(--color-text-muted);
  font-size: 0.78rem;
}

.trial__btn {
  white-space: nowrap;
}

.trial__btn--none {
  color: var(--color-text-dim);
  font-size: 0.78rem;
}

.hint--warn {
  color: var(--color-warning);
}

.hint--dim {
  color: var(--color-text-dim);
  font-size: 0.76rem;
  line-height: 1.7;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
