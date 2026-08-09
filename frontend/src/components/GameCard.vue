<script setup lang="ts">
/**
 * 游戏卡片（横向形态，用于搜索结果列表）
 *
 * 原有 `layout: 'row' | 'grid'` 已于第 5 步删除。grid 形态只服务过已安装页的
 * 卡片网格，而第 3 步把那里改成 master-detail 后，网格随 `LibraryView` 一同
 * 退场，此后零调用点。
 *
 * 删 prop 而非只删样式：一个两值枚举若只被传过一个值，它就不是枚举。留着
 * 「接受 grid 但没有对应样式」的参数，比没有这个参数更糟——签名承诺了一种
 * 它实现不了的形态。
 *
 * NOTE: 若日后搜索结果要做网格视图，从 git 历史里取回那四段样式即可
 * （`.card--grid` / `--grid .card__cover` / `--grid .card__name` /
 * `--grid .card__badge`），不必现在替一个没人用的形态占位。
 */

import { UiTooltip } from './ui'

interface Props {
  appID: string
  name: string
  /** 横版封面 URL，为空时显示占位 */
  cover?: string
  /** 已入库标记 */
  installed?: boolean
  /** 副标题，如「12/19 DLC · 07-28 获取」 */
  subtitle?: string
  /** 需用户知情的对账异常，显示警示角标 */
  warning?: boolean
}

const props = defineProps<Props>()

/**
 * Steam CDN 的横版封面地址。
 *
 * 搜索接口已给出 headerImage，但已安装页的记录里没有该字段，此时按
 * AppID 拼出约定路径——Steam 对所有上架应用都提供此路径。
 */
const coverUrl = () =>
  props.cover ||
  `https://cdn.cloudflare.steamstatic.com/steam/apps/${props.appID}/header.jpg`
</script>

<template>
  <button class="card" type="button">
    <div class="card__cover">
      <img :src="coverUrl()" :alt="name" loading="lazy" />
      <UiTooltip v-if="warning" content="存在需处理的清单冲突" class="card__warn-anchor">
        <span class="card__warn">⚠</span>
      </UiTooltip>
    </div>

    <div class="card__info">
      <div class="card__name">{{ name }}</div>
      <div class="card__meta">
        <span class="card__appid">AppID {{ appID }}</span>
        <span v-if="subtitle" class="card__sub">{{ subtitle }}</span>
      </div>
    </div>

    <span v-if="installed" class="card__badge">已入库</span>
  </button>
</template>

<style scoped>
.card {
  display: flex;
  /* 原 .card--row 的两条并入此处：只剩一种形态，修饰类没有存在意义 */
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-1);
  color: var(--color-text);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  /* 仅过渡合成器属性，长列表下不产生重排 */
  transition: transform var(--dur-instant) var(--ease-standard),
    border-color var(--dur-instant) var(--ease-standard);
}

.card:hover {
  border-color: var(--color-accent);
  transform: translateY(-2px);
}

.card__cover {
  position: relative;
  flex: 0 0 auto;
  width: 138px;
  height: 64px;
  overflow: hidden;
  /* 同心圆角：卡片 12px、内边距 12px，故封面取 inner 档（6px）。
     账本方向的 chip 档（4px）是给角标用的，配在这个尺寸的图上偏小 */
  border-radius: var(--radius-inner);
  background: var(--color-surface-2);
}

.card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

/*
  绝对定位挪到锚点上：警示角标现在包在 UiTooltip 里，
  而 UiTooltip 的锚点是 inline-flex 外壳——定位若留在内层 span 上，
  它会相对锚点定位而非封面，角标就跑到左上角去了。
*/
.card__warn-anchor {
  position: absolute;
  top: 4px;
  right: 4px;
}

.card__warn {
  padding: 0 5px;
  border-radius: var(--radius-chip);
  background: var(--state-warn);
  /* #000 换令牌：纯黑不在两套主题的色板里（4.1 节均不含纯白纯黑），
     且压在浅色主题那个偏暗的警示黄上对比过强，显得像贴纸 */
  color: var(--color-bg);
  font-size: var(--text-xs);
}

.card__info {
  flex: 1 1 auto;
  min-width: 0;
}

.card__name {
  /* 0.92rem(14.7px) -> --text-md(15)。它是卡片内的标题 */
  font-size: var(--text-md);
  font-weight: var(--weight-medium);
}

.card__meta {
  display: flex;
  gap: var(--space-3);
  color: var(--color-text-dim);
  font-size: var(--text-xs);
}

.card__appid {
  font-family: var(--font-mono);
  /* AppID 是要被纵向对比与复制的数字（速查第 9 条） */
  font-variant-numeric: tabular-nums;
}

.card__badge {
  flex: 0 0 auto;
  align-self: center;
  padding: 2px var(--space-2);
  border: 1px solid var(--state-ok);
  border-radius: var(--radius-chip);
  color: var(--state-ok);
  font-size: var(--text-xs);
}
</style>
