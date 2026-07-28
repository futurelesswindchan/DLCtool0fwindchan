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
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  color: var(--color-text);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  /* 仅过渡合成器属性，长列表下不产生重排 */
  transition: transform var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
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
  border-radius: var(--radius-sm);
  background: var(--color-bg-hover);
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
  border-radius: var(--radius-sm);
  background: var(--color-warning);
  color: #000;
  font-size: 0.75rem;
}

.card__info {
  flex: 1 1 auto;
  min-width: 0;
}

.card__name {
  font-size: 0.92rem;
  font-weight: 500;
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
  font-size: 0.75rem;
}

.card__appid {
  font-family: var(--font-mono);
}

.card__badge {
  flex: 0 0 auto;
  align-self: center;
  padding: 2px var(--space-2);
  border: 1px solid var(--color-success);
  border-radius: var(--radius-sm);
  color: var(--color-success);
  font-size: 0.72rem;
}

.card--grid .card__badge {
  align-self: flex-start;
}
</style>
