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

import type { DLCInfo } from "../api";
import type { SyncState } from "../composables/useDlcSelection";
import { UiCheckbox, UiHelpBadge, UiTooltip } from "./ui";
import { glossary } from "../glossary";

defineProps<{
  dlcs: DLCInfo[];
  isSelected: (appID: string) => boolean;
  syncState: SyncState;
  /** Steam 未运行时，同步完成的文案改为「下次启动 Steam 后生效」 */
  steamRunning?: boolean;
  /** 只读模式：外部清单无对应的清单包数据，无法得知可选项全貌 */
  readonly?: boolean;
}>();

const emit = defineEmits<{ toggle: [dlc: DLCInfo] }>();

const syncText: Record<SyncState, string> = {
  idle: "",
  pending: "⋯ 待同步",
  syncing: "🔄 同步中",
  done: "✓ 已同步",
};
</script>

<template>
  <section class="dlc">
    <header class="dlc__head">
      <h2 class="dlc__title">
        DLC 列表（{{ dlcs.length }}）
        <UiHelpBadge
          :content="glossary['auto-save'].short"
          aria-label="了解「勾选即保存」"
        />
      </h2>
      <Transition name="sync">
        <span v-if="syncState !== 'idle'" class="dlc__sync">
          {{
            syncState === "done" && steamRunning === false
              ? "✓ 下次启动 Steam 后生效"
              : syncText[syncState]
          }}
        </span>
      </Transition>
    </header>

    <!-- 图例：⚑ 的含义不能只靠一个符号传达，多数用户不会去悬停看 title -->
    <div class="legend">
      <p class="legend__row">
        <span class="legend__mark">⚑</span>
        <span>
          这代表此 DLC 含独立的内容分支，勾选后需由
          <strong>Steam 另行下载文件</strong>
          才能玩到内容。取消勾选时 Steam 可能删除已下载的本地文件。
        </span>
      </p>
      <p class="legend__row">
        <span class="legend__mark legend__mark--none">—</span>
        <span>
          这代表此 DLC 内容已包含在游戏本体里，勾选后
          <strong>无需额外下载</strong>，也不占用额外磁盘空间，可以直接游玩~
        </span>
      </p>
    </div>

    <ul class="dlc__list">
      <li v-for="d in dlcs" :key="d.appID" class="row">
        <!--
          整体换成 UiCheckbox 而非往里塞：原 .row__label 自己就是 <label>，
          而 UiCheckbox 内部也是 <label>，嵌套 label 是非法 HTML
          （点击行为在浏览器间不一致，读屏也会读错）。

          勾选值仍由父组件的 isSelected 单向决定，本组件不留本地状态——
          事件只上报「这一条被点了」，落盘与还原归 useDlcSelection。
        -->
        <UiCheckbox
          v-if="!readonly"
          class="row__check"
          :model-value="isSelected(d.appID)"
          @update:model-value="emit('toggle', d)"
        >
          <span class="row__name">{{ d.name || d.appID }}</span>
        </UiCheckbox>

        <!--
          只读态压根不渲染勾选框，而非渲染一个禁用的。

          两个理由。其一是本组件底部那句文案自己说的「因此不提供勾选」——
          摆一个灰掉的勾选框与那句话矛盾，它读起来像「满足某个条件就能勾」。

          其二更要紧：`disabled` 会让 UiCheckbox 整体降到 50% 透明度（原语
          对普通禁用项的正确行为），而只读态恰恰是「用户只想读这份列表」的
          场景，两百行半透明就没法读了。原生 checkbox 此前只灰掉方框、名字
          仍全亮，换原语时若照搬 disabled 就会悄悄吃掉可读性。
        -->
        <span v-else class="row__name row__name--plain">
          {{ d.name || d.appID }}
        </span>

        <span class="row__appid">{{ d.appID }}</span>

        <!--
          depot 标记。文案原先写死在 title 属性里，已移入 glossary——
          它解释的是术语（「含独立内容」与「纯许可证」的区别），
          按宪法 22 条术语文案唯一来源是 glossary.ts。

          用 UiTooltip 而非 UiHelpBadge：两百行列表里每行放一个可点徽章
          会把列表变成按钮墙，且这两条解释是同一维度的两个取值，
          用户扫列时要的是「这一行属哪种」，不是逐行展开阅读。
        -->
        <UiTooltip
          :content="
            d.manifestID
              ? glossary['depot-branch'].short
              : glossary['depot-license'].short
          "
        >
          <span
            class="row__depot"
            :class="{ 'row__depot--none': !d.manifestID }"
          >
            {{ d.manifestID ? "⚑" : "—" }}
          </span>
        </UiTooltip>
      </li>
    </ul>

    <p v-if="readonly" class="dlc__readonly">
      该清单不是本工具部署的，无法得知可选项全貌，因此不提供勾选。
    </p>

    <!-- 生效链路说明。用户最常见的困惑是「点了但 Steam 里没变化」，
         而每一环的实际状态都在别处（清单文件 / 注入器 / Steam），
         界面无从代为确认，只能把链路讲明白让用户自行判断到哪一步。 -->
    <details v-if="!readonly" class="howto">
      <summary class="howto__summary">怎样算 DLC 已经生效？</summary>
      <ol class="howto__steps">
        <li>
          <strong>勾选写入清单文件。</strong>
          勾选后约 1 秒，本工具会把选中的 DLC 写进 Steam 目录下的清单脚本。
          右上角出现「✓ 已同步」即代表这一步完成。
        </li>
        <li>
          <strong>注入器读取清单。</strong>
          OpenSteamTool 监视该目录，文件变动后会自动重新加载，无需手动操作。
          前提是它已随 Steam 启动。
        </li>
        <li>
          <strong>Steam 库中出现条目。</strong>
          通常几秒内刷新。若 Steam 当前未运行，改动会在下次启动 Steam 时生效。
        </li>
        <li>
          <strong>带 ⚑ 的还需下载。</strong>
          在 Steam 的游戏属性 → DLC 中确认已勾选，Steam 会开始下载对应内容。
          不带 ⚑ 的到上一步就已经可以玩了。
        </li>
      </ol>
      <p class="howto__note">
        取消勾选后 Steam 库中的条目可能仍显示一段时间——注入器会跳过你正版
        拥有的内容以免误删，重启 Steam 即可恢复正常显示。
      </p>
    </details>
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
  font-size: var(--text-md);
  font-weight: var(--weight-medium);
}

.dlc__sync {
  margin-left: auto;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.dlc__list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  /* 子行是方角，须被外层圆角裁掉 */
  overflow: hidden;
  content-visibility: auto;
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  font-size: var(--text-base);
}

.row + .row {
  border-top: 1px solid var(--color-border);
}

.row:hover {
  background: var(--color-surface-2);
}

/*
  只给 UiCheckbox 定这一行的伸缩关系（布局属调用方关切），
  其内部的方框尺寸、热区、焦点环一概不碰。

  原 `.row__label:has(input:disabled)` 那条已随只读态改为不渲染勾选框而失效
  ——没有 input 了，也就没有据其禁用态改光标这回事。
*/
.row__check {
  flex: 1 1 auto;
  min-width: 0;
}

/* 只读态的名字：没有勾选框可以借位，得自己吃掉剩余宽度。
   字号与勾选态保持一致（--text-sm，随 UiCheckbox 的 .cb__label 走），
   否则同一份列表在两种模式下字号会不同 */
.row__name--plain {
  flex: 1 1 auto;
  min-width: 0;
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
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
  font-size: var(--text-xs);
  /* 整列 AppID 要能纵向扫读（速查第 9 条） */
  font-variant-numeric: tabular-nums;
}

.row__depot {
  flex: 0 0 auto;
  width: 18px;
  text-align: center;
  color: var(--state-warn);
}

.row__depot--none {
  color: var(--color-text-dim);
}

.dlc__readonly {
  margin: var(--space-2) 0 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.legend {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin-bottom: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-ctrl);
  background: var(--color-surface);
  /* 图例是本组件唯一真正归 --text-xs 的地方——它就是「图例」，
     与那些被改判为 --text-sm 的说明性正文不同 */
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.legend__row {
  display: flex;
  gap: var(--space-2);
  margin: 0;
}

.legend__mark {
  flex: 0 0 auto;
  width: 14px;
  text-align: center;
  color: var(--state-warn);
}

.legend__mark--none {
  color: var(--color-text-dim);
}

.howto {
  margin-top: var(--space-3);
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  font-size: var(--text-sm);
}

.howto__summary {
  cursor: pointer;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.howto__summary:hover {
  color: var(--color-text);
}

.howto__steps {
  margin: var(--space-3) 0 0;
  padding-left: var(--space-5);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}

.howto__steps li + li {
  margin-top: var(--space-2);
}

.howto__steps strong {
  color: var(--color-text);
  font-weight: var(--weight-medium);
}

.howto__note {
  margin: var(--space-3) 0 0;
  color: var(--color-text-dim);
  /* 多行说明正文，同 SearchPane .tips 一样由 --text-xs 改判 --text-sm */
  font-size: var(--text-sm);
}

.sync-enter-active,
.sync-leave-active {
  transition: opacity var(--dur-fast) var(--ease-standard);
}

.sync-enter-from,
.sync-leave-to {
  opacity: 0;
}
</style>
