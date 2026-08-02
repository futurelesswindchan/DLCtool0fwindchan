<script setup lang="ts">
/**
 * 游戏卡片
 *
 * 两种形态共用一个组件：搜索结果用横向（容纳完整游戏名），已安装页用
 * 网格（封面为主视觉，名称可截断）。差异仅在布局，信息构成一致，故不拆分。
 */

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
  /**
   * 形态。
   *
   * TODO(第 5 步): `grid` 已无调用点。它原先只服务已安装页的卡片网格，
   *   而第 3 步把那里改成了 master-detail（侧栏列表 + 详情），网格随
   *   `LibraryView` 一同退场。判断是否删除时的依据：搜索结果若将来要做
   *   网格视图才有必要留，否则连同 `.card--grid` 那四段样式一起删。
   *   标在这里是为了别在第 5 步花时间打磨一个没人用的形态。
   */
  layout?: 'row' | 'grid'
}

const props = withDefaults(defineProps<Props>(), { layout: 'row' })

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
  <button class="card" :class="`card--${layout}`" type="button">
    <div class="card__cover">
      <img :src="coverUrl()" :alt="name" loading="lazy" />
      <span v-if="warning" class="card__warn" title="存在需处理的清单冲突">⚠</span>
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

.card--row {
  align-items: center;
  gap: var(--space-4);
}

.card--grid {
  flex-direction: column;
  gap: var(--space-2);
}

.card__cover {
  position: relative;
  flex: 0 0 auto;
  overflow: hidden;
  /* 同心圆角：卡片 12px、内边距 12px，故封面取 inner 档（6px）。
     账本方向的 chip 档（4px）是给角标用的，配在这个尺寸的图上偏小 */
  border-radius: var(--radius-inner);
  background: var(--color-surface-2);
}

.card--row .card__cover {
  width: 138px;
  height: 64px;
}

.card--grid .card__cover {
  width: 100%;
  aspect-ratio: 460 / 215;
}

.card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.card__warn {
  position: absolute;
  top: 4px;
  right: 4px;
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

/* 网格形态下名称可截断，横向形态允许换行以显示完整名称 */
.card--grid .card__name {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
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

.card--grid .card__badge {
  align-self: flex-start;
}
</style>
