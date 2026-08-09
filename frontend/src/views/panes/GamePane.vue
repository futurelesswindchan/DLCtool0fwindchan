<script setup lang="ts">
/**
 * 游戏详情 Pane
 *
 * 在两棵路由树下注册为同一组件（`game` 与 `library-game`），
 * 渲染进「用户来的那个列表」的内容区（宪法 3.7）。
 *
 * ⚠️ 依赖一个必须守住的前提：外层 `PaneTransition` 不给 `RouterView` 加 key。
 *    加了会强制重建组件，下方 `watch(appID, load)` 永远不触发且不报错。
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

import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
// wailsjs 位于 frontend/ 下而非 src/ 内，故比其他 import 多一级
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
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
} from "../../api";
import { useLibraryStore } from "../../stores/library";
import { useEnvStore } from "../../stores/env";
import { useToast } from "../../composables/useToast";
import { useConfirm } from "../../composables/useConfirm";
import { useDlcSelection } from "../../composables/useDlcSelection";
import DlcList from "../../components/DlcList.vue";
import { UiButton } from "../../components/ui";

const route = useRoute();
const router = useRouter();
const library = useLibraryStore();
const env = useEnvStore();
const toast = useToast();
const confirm = useConfirm();

const appID = computed(() => String(route.params.appID));

const detail = ref<GameDetail | null>(null);

/**
 * 各源的试下载结果。替代原先只有源名的字符串列表。
 *
 * 改动的根因是「收录」与「有用」是两件事：probe 只能回答该源存在这个游戏
 * 的文件，而实测同一个游戏 7 个源全报收录、实得从 200 个 DLC 到 0 个。
 * 只给源名的话，用户唯一的选择方式是盲猜。
 */
const report = ref<TrialReport | null>(null);
const looking = ref(false);
const downloading = ref(false);

/** 正在手动试的认证型源名，空表示无。用于只禁用那一行的按钮。 */
const quotaTrying = ref("");
/** 下载进度事件推送的当前尝试源，用于让用户看到「正在做什么」 */
const progressText = ref("");

/** 当前加载的清单包。来自留存文件或本次会话的下载。 */
const pkg = ref<GamePackage | null>(null);

/** 清单的获取时刻，RFC 3339 字符串。空表示本次会话刚下载的。 */
const savedAt = ref("");

/** 清单的来源源名。空表示本地导入。 */
const pkgSource = ref("");

const libItem = computed(() => library.find(appID.value));
const installed = computed(() => !!libItem.value);

/**
 * 是否正在读取留存清单。
 *
 * 存在理由：状态 C 的判据是 `installed && !pkg`，而这个组合在「已入库游戏
 * 的留存读取尚未返回」期间同样成立——切到已入库游戏时 `installed` 立刻为
 * true（来自同步的 library.find），`pkg` 还是 null，于是状态 C 被真实渲染，
 * 而它的文案写着「本地没有这个游戏的清单内容」。
 *
 * 那不是闪一帧的瑕疵，是在读取期间对用户断言了一件尚未查明的事——与后端
 * 静默改写枚举值同形状：把「还不知道」伪装成「已确认没有」。
 *
 * 故三个状态的判据都要排除读取中，另给一个明确的读取态。
 *
 * 初值取 installed 而非 false：以 URL 直入已入库游戏时，首屏渲染发生在
 * onMounted 的 load() 之前，那一帧同样会命中状态 C。
 */
const storedLoading = ref(installed.value);

/**
 * 用户是否主动发起了「重新获取」。
 *
 * 状态 A（源对比表）原先的判据是 `!installed && !pkg`，其中 `!installed`
 * 同时承担了两个含义：「这游戏没入库」与「该把源对比表摆出来」。这两件事
 * 在重新获取时会分道扬镳——reacquire() 把 pkg 置空但 installed 仍为 true，
 * 于是表格无处可去，页面落回状态 C，而 reacquire 的注释写的却是
 * 「把对比表摆出来让用户自己挑」。意图与渲染不符。
 *
 * 不用「report 非空」反推意图：那是拿数据猜语义，读代码的人看不出来，
 * 且一旦将来有别的路径填了 report 就会误触发。意图只有发起处知道。
 */
const reacquiring = ref(false);

/**
 * 当前该渲染哪一个状态。
 *
 * 四个状态互斥，故由**一个值**裁决，而不是让四组布尔条件各自去判断。
 *
 * 上一轮（08-03）修状态 C 误报时用的是「往每个判据上补排除条件」，那治好了
 * 「状态 C 何时该出现」，却漏了另一半：状态 A 是独立的 `v-if`，与状态 B/C
 * 那条 `v-if/v-else-if` 链互不知情。两条链都为真时**两个状态同时渲染**——
 * 实测重新获取时可见源对比表下面跟着一段「本地没有这个游戏的清单内容」，
 * 点「取消」后对比表收起，只剩那段错文案留在原地。
 *
 * 教训是判据分散在模板里就无法保证互斥：每加一个状态，都得指望作者记得去
 * 改另外几处不相邻的条件。改成单一枚举后，「同时出现两个」在结构上不可能，
 * 而非依赖作者的记性。
 *
 * 顺序即优先级，从上到下第一个成立者胜出：
 *   1. pkg 在手 → 状态 B，无论其他标记如何
 *   2. 正在读留存 → 读取态，必须先于状态 C（否则把「还不知道」说成「没有」）
 *   3. 未入库，或已入库但用户主动要换源 → 状态 A（源对比表）
 *   4. 已入库且确认无留存 → 状态 C
 */
type GameViewState = "package" | "loading" | "sources" | "noPackage";

const viewState = computed<GameViewState>(() => {
  if (pkg.value) return "package";
  if (storedLoading.value) return "loading";
  if (!installed.value || reacquiring.value) return "sources";
  return "noPackage";
});

/**
 * 状态 B 分支内使用的清单包，已收窄为非空。
 *
 * 存在理由是类型收窄：模板判据从 `v-if="pkg"` 换成 `viewState === 'package'`
 * 后，TS 不再知道二者等价，`pkg.dlcs` 遂报可能为 null。
 *
 * 不写 `pkg!`：那是把「此处一定非空」这个理由从代码里抹掉，只留一个感叹号
 * ——日后若 viewState 的判据改动，感叹号不会报错，而这里会。
 */
const activePkg = computed(() =>
  viewState.value === "package" ? pkg.value : null,
);

/**
 * 清单获取时间的人性化表述。
 *
 * 有意不说「已过期」——清单旧不等于无效，多数情况下几个月前的清单依然
 * 可用。只陈述事实，让用户自行判断是否值得重新获取。
 */
const savedAtText = computed(() => {
  if (!savedAt.value) return "";

  const then = new Date(savedAt.value);
  if (Number.isNaN(then.getTime())) return "";

  const days = Math.floor((Date.now() - then.getTime()) / 86_400_000);
  const when = days <= 0 ? "今天" : days === 1 ? "昨天" : `${days} 天前`;
  return `清单获取于${when}（${savedAt.value.slice(0, 10)}）`;
});

/**
 * 勾选控制器。
 *
 * 在 setup 顶层一次性构造，pkg 为 null 时各项操作自行短路——因为
 * useDlcSelection 内部注册了 onUnmounted，放进 watch 回调会注册不到
 * 当前组件实例上，导致待落盘的改动在切页时静默丢失。
 */
const selection = useDlcSelection(pkg);

onMounted(() => {
  // 必须先注册再 load()。原实现是 `await load()` 之后才注册，而未入库游戏
  // 的 load 会走 lookup()——真下载并解析各源，首次实测可达 41 秒，而
  // download:progress 恰恰是这段期间推送的。等它跑完才装监听器，等于首屏
  // 那 41 秒里一条进度都收不到，progressText 始终为空。
  //
  // 「进度可见的等待不叫等待，叫围观」——最需要围观的正是最慢的第一次。
  EventsOn("download:progress", (payload: any) => {
    if (payload?.appID && payload.appID !== appID.value) return;
    const src = payload?.source ?? "";
    progressText.value = src ? `正在尝试 ${src}…` : "正在获取…";
  });

  void load();
});

// 切页面不清理会导致重复注册，同一事件收到多份
onUnmounted(() => EventsOff("download:progress"));

/** 路由参数变化时重新加载（用户可能从已安装页直接跳到另一个游戏） */
watch(appID, () => {
  // 三行必须同步完成再让渲染发生：pkg 置空的同一刻，installed 已因
  // library.find(appID) 变为 true。若 storedLoading 慢一步（例如只在
  // load() 体内置位），中间就存在 `installed && !pkg && !storedLoading`
  // 的可观测状态，恰好命中状态 C。
  pkg.value = null;
  storedLoading.value = installed.value;
  reacquiring.value = false;
  void load();
});

/**
 * 兜住「入库状态迟到」的情形。
 *
 * 以 URL 直接进入库内游戏时（重载、外部唤起），library.refresh() 往往还没
 * 返回，此刻 installed 为 false，load() 会走未入库分支去试各源。等 store
 * 回填后 installed 翻 true，而那次 load 早已结束——留存读取根本不会发生，
 * 页面停在状态 C 并宣称「本地没有清单内容」。
 *
 * 只在「翻为 true 且尚无 pkg」时补读一次，不做全量重载：详情已经取到了，
 * 没必要再发一次网络请求。
 */
watch(installed, (now, before) => {
  if (now && !before && !pkg.value && !storedLoading.value) {
    storedLoading.value = true;
    void loadStored();
  }
});

/**
 * 落盘完成后读回权威的获取时间。
 *
 * 不在前端自行填 Date.now()：清单留存由后端在部署时写入，前端猜的时间
 * 与实际落盘时刻可能不符。等 syncState 转 done 再读，可确保文件已存在。
 */
watch(
  () => selection.syncState.value,
  async (state) => {
    if (state !== "done") return;
    try {
      const stored = await getPackage(appID.value);
      if (stored?.savedAt) savedAt.value = stored.savedAt;
      if (stored?.source !== undefined) pkgSource.value = stored.source;
    } catch {
      // 读不回来只影响「获取于 X 天前」的显示，不值得打扰用户
    }
  },
);

async function load() {
  detail.value = null;
  report.value = null;
  progressText.value = "";
  savedAt.value = "";
  pkgSource.value = "";
  reacquiring.value = false;

  // 同步置位，且必须在任何 await 之前，否则中间存在
  // `installed && !pkg && !storedLoading` 的可观测状态，命中状态 C。
  storedLoading.value = installed.value;

  // 详情与留存并发取，不再串联。
  //
  // 原实现是先 await 详情、再读留存，于是读取态要挂满整个详情请求——即便
  // 留存清单就在本地、即便详情早已缓存。实测在库页来回切两个已入库游戏时
  // 表现为「正在读取本地清单…」疯狂闪烁，因为每次切换都要重等一遍详情。
  //
  // 这两件事本无依赖关系，串起来只是因为写在同一个函数里的先后位置。
  // 并发之后留存通常先回（读本地文件），状态 B 直接就位，读取态几乎不可见。
  //
  // 未入库分支仍走 lookup()，它本来就是慢操作，进度由 progressText 承担。
  await Promise.all([loadDetail(), installed.value ? loadStored() : lookup()]);
}

/**
 * 取商店详情。失败只提示，不阻断——详情是锦上添花，勾选功能不依赖它。
 */
async function loadDetail() {
  try {
    detail.value = await getGameDetail(appID.value);
  } catch (e) {
    toast.fromError(e, "获取游戏详情失败");
  }
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
  storedLoading.value = true;
  try {
    const stored = await getPackage(appID.value);
    if (!stored?.package) return;

    pkg.value = stored.package;
    savedAt.value = stored.savedAt ?? "";
    pkgSource.value = stored.source ?? "";

    // 勾选集合一律问后端，不走 library store 的缓存。
    //
    // 原实现是 `libItem.value?.record ?? await findHistory(...)`，想省一次
    // 跨边界调用。但 library store 是缓存——只在 LibraryShell 挂载时与
    // 入库/卸载后刷新，库页切游戏不刷、勾选落盘后也不刷。于是「取消勾选后
    // 立刻切走、再切回来」读到的是上次刷新时的旧记录，被取消的那个 DLC
    // 仍显示为勾选，而磁盘上的 lua 与 history.json 其实都是对的。
    //
    // 实测该缺陷只在取消方向可见：缓存存的是「上次刷新时的状态」，勾选方向
    // 上旧值恰好等于正确值，症状被掩盖。又一次「一个方向碰巧对了」。
    //
    // 教训：缓存可以回答「这游戏在不在库里」，不能回答「用户选了哪些」。
    // 前者变化时必然伴随一次刷新，后者不是。省一次本地调用换不来这个风险。
    const record = await findHistory(appID.value);
    selection.restore(record?.installedIDs ?? []);
  } catch (e) {
    toast.fromError(e, "读取本地清单失败");
  } finally {
    storedLoading.value = false;
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
  looking.value = true;
  try {
    report.value = await trialSources(appID.value, refresh);
  } catch (e) {
    toast.fromError(e, "查询清单源失败");
  } finally {
    looking.value = false;
  }
}

/**
 * 手动试某个认证型源。
 *
 * 独立成一个动作是因为它消耗用户自己申请的 API 额度——这笔开销必须由一次
 * 明确的点击表达，不能夹在自动流程里替用户花掉。
 */
async function tryQuotaSource(source: string) {
  if (!report.value) return;

  const idx = report.value.trials.findIndex((t) => t.source === source);
  if (idx < 0) return;

  quotaTrying.value = source;
  try {
    const t = await trialOneSource(appID.value, source);
    // 就地替换该行，不整表重查——重查会把其他源也再跑一遍
    report.value.trials[idx] = t;
    recomputeBest();
  } catch (e) {
    toast.fromError(e, `试 ${source} 失败`);
  } finally {
    quotaTrying.value = "";
  }
}

/**
 * 手动试完认证源后重算「实得最多」。
 *
 * 前端自行重算而非回后端取：后端的汇总是在整表试下载时算的，此处只多了
 * 一行数据，为它再走一次跨边界调用不值得。
 */
function recomputeBest() {
  if (!report.value) return;

  let best = "";
  let max = 0;
  for (const t of report.value.trials) {
    // 严格大于，与后端 summarizeTrials 保持一致：并列时保留在前者，
    // 使推荐结果稳定可复现
    if (t.status === "ok" && t.dlcCount > max) {
      max = t.dlcCount;
      best = t.source;
    }
  }
  report.value.bestSource = best;
  report.value.maxDLC = max;
}

/**
 * 从指定源入库。
 *
 * 部署由 useDlcSelection 在 pkg 就位后自动触发首次同步——下载得到的包
 * 默认全选，与本地导入路径保持一致的语义。
 */
async function install(sourceName: string) {
  if (!env.ready) {
    toast.warn("注入器尚未就绪，请先完成环境配置");
    router.push({ name: "setup" });
    return;
  }

  downloading.value = true;
  progressText.value = `正在从 ${sourceName} 入库…`;
  try {
    // 走 installFromTrial 而非 downloadFromRepo：试下载的产物已缓存，
    // 命中时零网络请求。这是那一轮等待的收益端，缺了它试下载就只是变慢。
    pkg.value = await installFromTrial(appID.value, sourceName);
    // 下载得到的包默认全选，与本地导入路径保持一致的语义。
    // selectAll 会触发防抖落盘，完成首次部署。
    selection.selectAll();
    await library.refresh();

    // 先乐观填上来源，让界面立即有信息可显示。权威的 savedAt 要等防抖
    // 落盘真正完成后才存在，故交由 watch 在同步状态转 done 时读回。
    pkgSource.value = sourceName;
    savedAt.value = "";

    // 意图已达成，撤下标记。留着它不影响渲染（状态 B 的 v-if="pkg" 优先），
    // 但会让「用户还想换源吗」这个问题的答案一直是过期的 true。
    reacquiring.value = false;

    toast.success(`已获取 ${pkg.value.gameName || appID.value} 的清单`);
  } catch (e) {
    toast.fromError(e, "获取清单失败");
  } finally {
    downloading.value = false;
    progressText.value = "";
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
  pkg.value = null;
  reacquiring.value = true;
  await lookup(true);

  if (!report.value?.usableCount) {
    toast.warn("没有源能提供该游戏的清单，可尝试本地导入清单包");
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
      "将删除本工具部署的清单文件与安装记录。",
      "Steam 库中的条目可能需要重启 Steam 后才消失。",
    ],
    confirmText: "彻底卸载",
    danger: true,
  });
  if (!ok) return;

  try {
    const msg = await library.remove(appID.value);
    toast.success(msg);
    await resetToUninstalled();
  } catch (e: any) {
    // 外部声明导致的不彻底卸载，文案已由后端组装好，原样呈现
    toast.warn(e?.message ?? "卸载未完全成功");
    await resetToUninstalled();
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
  pkg.value = null;
  savedAt.value = "";
  pkgSource.value = "";
  await lookup();
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
        :src="
          detail?.headerImage ||
          `https://cdn.cloudflare.steamstatic.com/steam/apps/${appID}/header.jpg`
        "
        :alt="detail?.name || appID"
      />
      <div class="hero__info">
        <h1 class="hero__name">{{ detail?.name || appID }}</h1>
        <p class="hero__meta">
          AppID {{ appID }}
          <template v-if="detail?.releaseDate">
            · {{ detail.releaseDate }}</template
          >
          <template v-if="detail?.developers?.length">
            · {{ detail.developers.join("、") }}
          </template>
        </p>
        <p v-if="detail?.description" class="hero__desc">
          {{ detail.description }}
        </p>
      </div>
    </header>

    <!--
      状态 A：源对比表。

      两种情形都要它：未入库的游戏，以及已入库用户主动点了「重新获取」。
      后者原先落不到这里（`!installed` 为假），表格无处可去——见 reacquiring
      的声明处。判据写成「没有清单包，且（未入库 或 正在重新获取）」，
      两个含义各自显式。
    -->
    <section v-if="viewState === 'sources'" class="block">
      <h2 class="block__title">清单源</h2>

      <template v-if="looking">
        <p class="hint">正在逐个试取清单，这一步会实际下载并解析…</p>
        <p class="hint">
          比单纯查询「有没有」慢一些，但能看出各源实际能给多少 DLC。 结果会缓存
          30 分钟，稍后再来不必重等。
        </p>
      </template>

      <template v-else-if="report?.trials?.length">
        <p class="hint">
          已试 {{ report.trials.length }} 个源，其中
          <strong>{{ report.usableCount }}</strong> 个可用<template
            v-if="report.bestSource"
          >
            ，实得最多的是
            <strong>{{ report.bestSource }}</strong
            >（{{ report.maxDLC }} 个 DLC）</template
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
            <span
              class="trial__num"
              :class="{ 'trial__num--text': t.status !== 'ok' }"
            >
              <template v-if="t.status === 'ok'">{{ t.dlcCount }} DLC</template>
              <template v-else-if="t.status === 'empty'">仅本体</template>
              <template v-else>—</template>
            </span>
            <span class="trial__msg">
              {{ t.message }}
              <template v-if="t.cached">（缓存）</template>
            </span>
            <UiButton
              v-if="t.status === 'ok' || t.status === 'empty'"
              variant="primary"
              class="trial__btn"
              :disabled="downloading"
              @click="install(t.source)"
            >
              用这个入库
            </UiButton>
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
              <span
                class="trial__num"
                :class="{ 'trial__num--text': t.status !== 'ok' }"
              >
                <template v-if="t.status === 'ok'"
                  >{{ t.dlcCount }} DLC</template
                >
                <template v-else-if="t.status === 'empty'">仅本体</template>
                <template v-else>?</template>
              </span>
              <span class="trial__msg">{{ t.message }}</span>
              <!-- quotaTrying 存的是源名而非布尔，故能精确指出是哪一行在忙，
                   这里可以放心用 loading。旁边「用这个入库」用的 downloading
                   是共享布尔，挂 loading 会让所有行一起显示进行中。 -->
              <UiButton
                v-if="t.status === 'skipped' || t.status === 'failed'"
                class="trial__btn"
                :loading="quotaTrying === t.source"
                @click="tryQuotaSource(t.source)"
              >
                {{ quotaTrying === t.source ? "获取中…" : "试这个源" }}
              </UiButton>
              <UiButton
                v-else-if="t.status === 'ok' || t.status === 'empty'"
                variant="primary"
                class="trial__btn"
                :disabled="downloading"
                @click="install(t.source)"
              >
                用这个入库
              </UiButton>
              <span v-else class="trial__btn trial__btn--none">不可用</span>
            </li>
          </ul>
        </template>

        <div class="actions">
          <UiButton :loading="looking" @click="lookup(true)">
            全部重新试取
          </UiButton>
          <UiButton @click="router.push({ name: 'search' })">
            改用本地导入
          </UiButton>
          <!--
            重新获取的退路。已入库用户点开这张表后若不想换源了，没有它就只能
            靠切走再切回——而「卸载」入口在状态 C，同样够不着。
          -->
          <UiButton v-if="reacquiring" @click="reacquiring = false">
            取消，保持现状
          </UiButton>
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
          <UiButton :loading="looking" @click="lookup()">重新查询</UiButton>
          <UiButton @click="router.push({ name: 'search' })">
            去本地导入
          </UiButton>
          <!-- 同上：一个源都没有时更需要退路，否则卸载入口够不着 -->
          <UiButton v-if="reacquiring" @click="reacquiring = false">
            取消，保持现状
          </UiButton>
        </div>
      </template>

      <p v-if="progressText" class="hint">{{ progressText }}</p>
    </section>

    <!-- 状态 B：已入库且有留存清单 -->
    <template v-else-if="activePkg">
      <div class="meta-bar">
        <span v-if="savedAtText">{{ savedAtText }}</span>
        <span v-if="pkgSource">来源 {{ pkgSource }}</span>
        <span v-else>来源 本地导入</span>
      </div>

      <div class="actions">
        <UiButton @click="selection.selectAll()">全选</UiButton>
        <UiButton @click="selection.selectNone()">全不选</UiButton>
        <UiButton :loading="downloading" @click="reacquire()">
          重新获取清单
        </UiButton>
        <UiButton variant="danger" @click="uninstall()">彻底卸载</UiButton>
      </div>

      <p class="hint hint--dim">
        勾选后约 1秒自动写入，无需额外点击保存哦！
        「重新获取清单」会从源下载最新版本并覆盖本地这一份（也就是手动更新啦），若当前一切正常，通常不必操作。
      </p>

      <DlcList
        :dlcs="activePkg.dlcs"
        :is-selected="selection.isSelected"
        :sync-state="selection.syncState.value"
        @toggle="selection.toggle"
      />
    </template>

    <!--
      读取态：已入库、留存尚未读回。

      必须排在状态 C 之前。这里只说「正在读」，不对有没有留存下任何结论——
      读取中与「确认没有留存」是两件事，混在一起就会让用户看到一段错的解释
      并可能据此去点「重新获取」。
    -->
    <section v-else-if="viewState === 'loading'" class="block">
      <h2 class="block__title">已入库</h2>
      <p class="hint">正在读取本地清单…</p>
    </section>

    <!-- 状态 C：已入库但确认没有留存清单 -->
    <section v-else class="block">
      <h2 class="block__title">已入库</h2>
      <p class="hint">已部署清单文件：{{ libItem?.fileNames.join("、") }}</p>
      <p v-if="libItem?.record" class="hint">
        {{ libItem.record.dlcCount }} 个 DLC · 获取于
        {{ libItem.record.installedAt.slice(0, 16).replace("T", " ") }}
      </p>
      <p v-if="libItem?.conflicted" class="hint hint--warn">
        该游戏同时被本工具之外的清单文件声明，卸载后可能仍留在库中。
      </p>
      <p class="hint">
        本地没有这个游戏的清单内容（可能是早期版本入库的，或留存文件已被删除），
        因此暂时无法调整 DLC 勾选。重新获取一次即可恢复编辑，之后就会一直保留。
      </p>
      <div class="actions">
        <UiButton variant="primary" :loading="downloading" @click="reacquire()">
          重新获取以编辑
        </UiButton>
        <UiButton variant="danger" @click="uninstall()">彻底卸载</UiButton>
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
  border-radius: var(--radius-ctrl);
  /* 封面未加载时的占位底色 */
  background: var(--color-surface-2);
}

.hero__info {
  min-width: 0;
}

.meta-bar {
  display: flex;
  gap: var(--space-4);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-chip);
  background: var(--color-surface);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.hero__name {
  margin: 0 0 var(--space-1);
  /* 1.25rem(20px) -> --text-lg(19)。这是页面标题 */
  font-size: var(--text-lg);
}

.hero__meta {
  margin: 0 0 var(--space-2);
  color: var(--color-text-dim);
  font-size: var(--text-sm);
}

.hero__desc {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-base);
  /* 简介可能很长，限制三行以免挤压下方的操作区 */
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.block {
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  /* 面板取 card 档；其内部元素须按同心圆角相应收窄（宪法 4.4） */
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-1);
}

.block__title {
  margin: 0 0 var(--space-3);
  /* 0.9rem(14.4px) -> --text-md(15)。区块标题 */
  font-size: var(--text-md);
  font-weight: var(--weight-medium);
}

.hint {
  margin: 0 0 var(--space-2);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

/* ─── 源试取对比表 ───

   双重编码（宪法 10.2）。原 border-left 方案已废：3px 直角色条贴在圆角
   卡片左侧，色条两端是方的、卡片角是圆的，看起来像贴上去的胶带而非卡片
   自己长出来的——那是几何打架，调色救不回来。

   替代方案两条编码叠加，都不占横向宽度：
     ① 数字自己承担状态色（.trial__num）。对比表的核心动作是纵向扫一列
        数字，状态长在数字上，一眼同时得到「多少」和「行不行」，
        比左边框省一次眼动。
     ② 整行 --state-wash 淡染。单看比色条弱，但覆盖面大，
        形成的整体印象反而更强。

   状态色经 --trial-hue 一个自定义属性下发，五种状态各自只需改这一个值，
   数字色与行底色都从它派生。好处是新增状态时不会漏掉其中一处——
   两处分别写死是那种「改了一半」缺陷的温床。 */

.subtitle {
  margin: var(--space-4) 0 var(--space-2);
  font-size: var(--text-base);
  font-weight: var(--weight-medium);
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
  /* 同心圆角：外层 .block 是 12px card 档、内边距 16px，
     故此处取 ctrl 档（8px）而非同为 card 档 */
  border-radius: var(--radius-ctrl);

  /* 状态色的单一来源。默认取中性，各状态类只覆盖这一个值。
     行底色由它与 surface 混出，浓度走 --state-wash（深色 6% / 浅色 4%，
     浅底对淡染更敏感故取下限）。 */
  --trial-hue: var(--state-neutral);
  background: color-mix(
    in srgb,
    var(--trial-hue) var(--state-wash),
    var(--color-surface)
  );

  font-size: var(--text-sm);
}

/* 状态色不共享 --color-accent（速查第 6 条）。这不是配色偏好——主色是
   「下一步该点这个」的引导，若某个源的「可用」用主色标出，视觉上就等于
   推荐了它。而实测结论明确：Hubcap 并非恒优（KRV 上它只给 2 DLC，
   快照源给 4），任何源相关逻辑都不得写死优先级。 */
.trial--ok {
  --trial-hue: var(--state-ok);
}

.trial--empty {
  --trial-hue: var(--state-warn);
}

/* unsupported 与 miss 都是「该源没有可用内容」，同色处理——
   对用户而言两者的处置方式相同（换源），无需在颜色上再作区分。
   降透明度是第三重编码，表达「这行不用细看」。 */
.trial--unsupported,
.trial--miss {
  --trial-hue: var(--state-neutral);
  opacity: 0.7;
}

.trial--failed {
  --trial-hue: var(--state-danger);
}

/* skipped 是「还没发生」而非「结果不好」，故不给淡染、只用虚线描边——
   有底色会让人以为已经试过了。原方案只虚线化左边框，现在色条已去掉，
   虚线改为整圈：一个未闭合感的轮廓恰好表达「这里还空着」。 */
.trial--skipped {
  --trial-hue: var(--state-neutral);
  background: transparent;
  border-style: dashed;
  border-color: var(--color-border-str);
}

.trial__name {
  font-weight: var(--weight-medium);
  color: var(--color-text);
  overflow-wrap: anywhere;
}

/* 双重编码的第一条：数字自己承担状态。
   放大到 --text-md 并取状态色，使纵向扫这一列时「多少」与「行不行」
   同时到手。tabular-nums 是硬要求（速查第 9 条），
   等宽数字才能让 19 与 200 的位数差一眼看出来。 */
.trial__num {
  font-variant-numeric: tabular-nums;
  font-size: var(--text-md);
  font-weight: var(--weight-semibold);
  color: var(--trial-hue);
  text-align: right;
}

/* 「仅本体」「—」「?」这类占位文案不跟着放大：放大是为了让数字醒目，
   占位符一起放大只是变吵。回落到行内基准字号，颜色仍随状态。 */
.trial__num--text {
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
}

.trial__msg {
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.trial__btn {
  white-space: nowrap;
}

.trial__btn--none {
  color: var(--color-text-dim);
  font-size: var(--text-sm);
}

.hint--warn {
  color: var(--state-warn);
}

.hint--dim {
  /* 0.76rem 按表归 --text-xs，此处同 SearchPane .tips 一样改判 --text-sm：
     它是带 line-height 的多行说明正文，非角标 */
  color: var(--color-text-dim);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
