<script setup lang="ts">
/**
 * 在线搜索（首页默认 Pane）
 *
 * 搜索状态与请求生命周期住在 `stores/search.ts`，本组件只触发与读取。
 *
 * ⚠️ 原注释写着「搜索结果不进 store：属会话级临时数据」，已被实机推翻。
 * 那个判断只问了「有没有别的页面要读」，漏了「组件销毁时正在飞的请求
 * 怎么办」——切页即丢关键词与结果，且在途 promise 落到已销毁实例上，
 * 搜到的结果被静默丢弃。判断留在此处备忘，避免有人日后又搬回来。
 *
 * 本地导入已于第 3 步搬去 `ImportPane.vue`，并在侧栏获得与在线搜索平级的
 * 常驻入口（宪法 3.4）。它并非退路——该站网页端额度是 API 的 4~60 倍，
 * 对重度用户而言手动下载再导入反而是更划算的主路径，
 * 而躺在本页底部折叠区时它「视觉上就是个退路」。
 *
 * 滚动、内边距与最大宽度由 `layout/ContentPane` 提供，本组件不再自己限宽。
 *
 * 控件已于第 5 步全部换为原语（`UiInput` / `UiButton`），本页无原生表单控件。
 *
 * 搜索框字号随原语走 `--text-sm`(12px)，不再是原先刻意放大的 `--text-md`(15px)。
 * 「主入口该更醒目」这个意图仍然成立，只是改由**位置、全宽、旁边唯一一个
 * primary 按钮**承担——宪法 4.1 规定每屏饱和主色只给「下一步该点的那一个」，
 * 那个色块本身就是最强的强调手段，不必再叠一层字号。
 */

import { useRouter } from "vue-router";
import { useLibraryStore } from "../../stores/library";
import { useSearchStore } from "../../stores/search";
import { openURL } from "../../api";
import GameCard from "../../components/GameCard.vue";
import { UiButton, UiInput } from "../../components/ui";

const router = useRouter();
const library = useLibraryStore();
const search = useSearchStore();

/** 外部链接。后端只放行 http/https，此处统一由 openURL 代理。 */
const LINKS = [
  {
    label: "GitHub",
    url: "https://github.com/futurelesswindchan/DLCtool0fwindchan",
  },
  { label: "博客", url: "https://qwq.windchan0v0.xyz/home" },
  { label: "B 站", url: "https://space.bilibili.com/273245032" },
  { label: "爱发电", url: "https://ifdian.net/a/wind-chan" },
] as const;

function onOpenLink(url: string) {
  void openURL(url);
}

/**
 * 触发搜索。由按钮点击或回车调用，不随输入自动执行。
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
 *
 * 不 await：请求归 store 管，本组件即使被销毁也不影响它跑完。
 */
function runSearch() {
  void search.run();
}

/**
 * 清空关键词与结果。
 *
 * 自动搜索取消后，清空输入不再自动收起结果列表，需要一个显式出口——
 * 否则用户想回到初始状态只能刷新页面。
 */
function clearSearch() {
  search.clear();
}

function openGame(appID: string) {
  router.push({ name: "game", params: { appID } });
}
</script>

<template>
  <div class="page">
    <div class="search">
      <UiInput
        v-model="search.term"
        class="search__field"
        type="search"
        size="md"
        placeholder="请搜索游戏本体的官方中文、英文名或 AppID"
        autofocus
        :disabled="search.searching"
        @enter="runSearch()"
      />
      <UiButton
        class="search__btn"
        variant="primary"
        :disabled="!search.canSearch"
        :loading="search.searching"
        @click="runSearch()"
      >
        {{ search.searching ? "搜索中…" : "搜索" }}
      </UiButton>
      <UiButton
        v-if="search.term || search.status !== 'idle'"
        :disabled="search.searching"
        title="清空"
        @click="clearSearch()"
      >
        清空
      </UiButton>
    </div>

    <!--
      「切页不打断」只在搜索期间出现：这句话要解决的是用户此刻正犹豫
      「能不能走开」，写在静态说明里等于没说——没人会在没疑问的时候
      记住一条承诺。不用 toast 是因为它恰好相反，是在事情发生后才弹。
    -->
    <p v-if="search.searching" class="hint" role="status">
      正在搜索，切到其他页面不会中断，回来还在这儿哦~
    </p>

    <!--
      失败是一等状态而非只发个 toast。toast 是瞬时的，切页回来「刚才失败过」
      就无处可查，用户看到的是初始空态——那等于把「没搜成」伪装成「没搜过」。
    -->
    <p v-else-if="search.status === 'failed'" class="error" role="alert">
      {{ search.errorMessage }}
    </p>

    <p class="tips">
      清单以整个游戏本体为单位提供，请搜索游戏本体>w<！单独去搜游戏 DLC
      名是不行的哦！
      <br />同时，因为搜索走的是 Steam官方接口，<strong
        >偶发失败与本工具无关，稍等再试通常就好。</strong
      >
      如需改善，开启Steam 网络加速工具可降低失败率。
    </p>

    <!--
      公告区：仅在 idle 态（未搜索、无结果）显示，搜索中或有结果时让位。
      三张卡复用书脊色条语言（与 EnvBanner / Toast 同一体系）。

      ┌─ 更新公告时的操作指引 ───────────────────────────────────────────────┐
      │                                                                   │
      │  【公告卡】board__card--accent（主色书脊）                           │
      │    <h2 class="board__title"> 标题文字 </h2>                        │
      │    <p class="board__body"> 段落一 </p>                             │
      │    <p class="board__body"> 段落二 </p>  ← 可多段，最后一段无下边距     │
      │                                                                   │
      │  【链接卡】board__card--neutral（中性书脊）                          │
      │    标题与链接按钮由 LINKS 常量驱动，改链接去 script 里改 LINKS          │
      │                                                                   │
      │  【防倒卖卡】board__card--warn（警示书脊）                            │
      │    <p class="board__warn-text"> 文字，<strong> 强调 </strong> </p> │
      │    固定格式，通常不改                                                │
      │                                                                   │
      │  可用书脊颜色修饰类：                                                │
      │    board__card--accent   主色（公告、亮点）                          │
      │    board__card--neutral  中性灰（次要信息、链接）                     │
      │    board__card--warn     警示橙（注意事项）                          │
      │    board__card--ok       可自行加，--spine-color:var(--state-ok)   │
      │                                                                   │
      │  显示条件：v-if="search.status === 'idle'"                         │
      │  一旦用户开始搜索或有结果，公告区自动让位，无需手动控制                    │
      └───────────────────────────────────────────────────────────────────┘
    -->
    <div v-if="search.status === 'idle'" class="board">
      <!-- 公告卡：v2.0.0 正式版寄语 -->
      <section class="board__card board__card--accent">
        <h2 class="board__title">欢迎使用Kazeusa（风兔盒） v2.0.0</h2>
        <p class="board__body">
          这是V2的第一个正式版本。从最初的想法到现在，感谢每一位陪伴咱走到这里的朋友。
        </p>
        <p class="board__body">
          风兔盒会持续维护，有问题欢迎反馈，有建议欢迎告诉咱。希望它能帮到你，也希望你玩得开心(・ω・)
        </p>
      </section>

      <!-- 链接卡：仓库 / 博客 / B站 / 爱发电 -->
      <section class="board__card board__card--neutral">
        <h2 class="board__title">同时，你可以通过以下途径联系到小风酱！</h2>
        <div class="board__links">
          <button
            v-for="link in LINKS"
            :key="link.url"
            class="board__link"
            type="button"
            @click="onOpenLink(link.url)"
          >
            {{ link.label }}
          </button>
        </div>
      </section>

      <!-- 防倒卖卡 -->
      <section class="board__card board__card--warn">
        <p class="board__warn-text">
          本工具是<strong>开源且免费</strong>的。如果你是花钱买来的...说明被骗了哦QAQ！
        </p>
      </section>
    </div>

    <ul v-if="search.results.length" class="results">
      <li v-for="r in search.results" :key="r.appID">
        <GameCard
          :app-i-d="r.appID"
          :name="r.name"
          :cover="r.headerImage"
          :installed="library.installedIDs.has(r.appID)"
          @click="openGame(r.appID)"
        />
      </li>
    </ul>

    <!--
      判据用 store 的 isEmptyResult，它只在 status 为 done 时成立。
      不写「结果为空」——searching 与 failed 时结果同样是空的，那两种
      情形下说「没找到匹配的游戏」是在尚未查明时替用户下结论。
    -->
    <p v-else-if="search.isEmptyResult" class="empty">
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

/*
  输入框在这一排里要吃掉剩余宽度。UiInput 自身是 `inline-flex` + `width: 100%`，
  在 flex 容器中仍需显式给伸缩因子，否则 `width: 100%` 相对的是它自己的
  内容宽度而非剩余空间。

  scoped 样式能命中子组件根元素：Vue 会给单根子组件的根节点也带上父组件的
  scope id。此处只调布局（外部关切），不碰 UiInput 内部的边框、字号、焦点环
  ——那些属原语自己的领地，从调用方伸手进去改就是 shim 模式回归。
*/
.search__field {
  flex: 1 1 auto;
  min-width: 0;
}

/*
  按钮文案在「搜索 / 搜索中…」间切换，宽度会跟着跳。定一个下限把位置钉住。

  NOTE: 加载态刻意由文案承担而非只靠 UiButton 的 loading 视觉——它就在
  用户刚点击的位置，比别处的指示器更容易被注意到。loading 仍然传，
  它额外给出 `cursor: progress` 与 `aria-busy`，两者不重复。
*/
.search__btn {
  min-width: 6em;
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

/* 进行中的旁注，视觉重量要低于结果本身——它是陪伴而非主角 */
.hint {
  margin: 0;
  color: var(--color-text-dim);
  font-size: var(--text-sm);
}

/*
  失败提示不用饱和主色：按宪法 4.1，主色只留给「下一步该点的那一个东西」，
  而这里没有可点的东西，重试入口是上方那个搜索按钮。
*/
.error {
  margin: 0;
  color: var(--state-warn);
  font-size: var(--text-base);
  line-height: var(--leading-normal);
}

.empty {
  margin: 0;
  color: var(--color-text-muted);
  /* 0.85rem(13.6px) -> --text-base(13)。这是搜索无果时的主要说明，属正文 */
  font-size: var(--text-base);
}

/*
  ⚠️ 语义与映射表冲突，按宪法 4.3 节「以语义为准」处理：
  0.76rem 按表归 --text-xs(11px)，但 --text-xs 的定义是「角标、图例」，
  而这里是三行带 line-height 1.7 的说明性正文。且第 3 步的 ImportPane
  同名 .tips 已用 --text-sm——两个兄弟 Pane 的提示字号不该不同。
  故归 --text-sm。已回填至宪法 4.3 节映射表。
*/
.tips {
  margin: 0;
  color: var(--color-text-dim);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.tips strong {
  color: var(--state-warn);
  font-weight: var(--weight-medium);
}

/* ─── 公告区 ─── */

.board {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/*
  公告卡基础。书脊色条与 EnvBanner / Toast 同一手法——
  ::before 伪元素独立设圆角，不用 border-left-width 加宽（加宽在圆角处截断是方的）。
  --spine-color 由各状态修饰类下发。
*/
.board__card {
  position: relative;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  overflow: hidden;
}

.board__card::before {
  content: "";
  position: absolute;
  left: 0;
  top: 6px;
  bottom: 6px;
  width: 3px;
  border-radius: 0 2px 2px 0;
  background: var(--spine-color, var(--color-border));
}

/* 公告卡用主色书脊 */
.board__card--accent {
  --spine-color: var(--color-accent);
}
/* 链接卡用中性色书脊 */
.board__card--neutral {
  --spine-color: var(--state-neutral);
}
/* 防倒卖卡用警示色书脊 */
.board__card--warn {
  --spine-color: var(--state-warn);
}

.board__title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-base);
  font-weight: var(--weight-semibold);
  color: var(--color-text);
}

.board__body {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}

.board__body:last-child {
  margin-bottom: 0;
}

/* 链接按钮组：横排，超窄时自动换行 */
.board__links {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

/*
  外链按钮。走 set-linkbtn 同一思路——真 <button> 经 openURL 代理，
  不用 <a href> 以免 WebView2 把整个界面导航走。
  外观用 chip 圆角 + 细边框，视觉重量低于搜索入口（宪法 4.1）。
*/
.board__link {
  padding: var(--space-1) var(--space-3);
  border: 1px solid var(--color-border-str);
  border-radius: var(--radius-chip);
  background: transparent;
  color: var(--color-text-muted);
  font-family: inherit;
  font-size: var(--text-sm);
  cursor: pointer;
  transition:
    background var(--dur-instant) var(--ease-standard),
    color var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard);
}

.board__link:hover {
  background: var(--color-surface-2);
  color: var(--color-text);
  border-color: var(--color-accent);
}

.board__link:focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 1px;
}

/* 防倒卖文案：紧凑单行，无标题 */
.board__warn-text {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}

.board__warn-text strong {
  color: var(--state-warn);
  font-weight: var(--weight-medium);
}
</style>
