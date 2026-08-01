<script setup lang="ts">
/**
 * 库概览：已安装页未选中任何游戏时的内容区
 *
 * ⚠️ 宪法 3.5 第 3 条：**未选中时内容区不得空白。空态是引导位，不是留白位。**
 *
 * 故这里分两种形态：
 *   - 库里有东西：给出总量、异常项与最近获取时间，并提示去侧栏选一个
 *   - 库是空的：这才是真正的空态，直接给「去搜索」的行动点
 *
 * 冲突项在此**必须**可见。它关系到卸载不彻底（07-28 实测三条硬约束），
 * 而用户不会主动逐个点开检查——侧栏的警示点只说明「这条有事」，
 * 「有几条、是什么事」得在内容区讲清楚。
 * 这不违反宪法 3.1：结论性信息正是内容区该承担的。
 */

import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useLibraryStore } from '../../stores/library'
import { UiButton, UiEmptyState } from '../../components/ui'

const router = useRouter()
const library = useLibraryStore()

const managed = computed(() => library.items.filter((i) => i.record))
const conflicted = computed(() => library.items.filter((i) => i.conflicted))
const orphaned = computed(() =>
  library.items.filter((i) => !i.record && i.hasExternal),
)
const lostRecord = computed(() =>
  library.items.filter((i) => !i.record && !i.hasExternal),
)

/** DLC 总数只统计本工具管理的条目——外部清单拿不到准确的 DLC 数。 */
const totalDlc = computed(() =>
  managed.value.reduce((n, i) => n + (i.record?.dlcCount ?? 0), 0),
)

/** 最近一次获取时间。空账本时为空字符串，模板据此隐藏该格。 */
const latest = computed(() => {
  const times = managed.value
    .map((i) => i.record?.installedAt ?? '')
    .filter(Boolean)
    .sort()
  return times.length ? times[times.length - 1].slice(0, 10) : ''
})
</script>

<template>
  <div class="pane">
    <!-- 真空态：库里什么都没有 -->
    <UiEmptyState
      v-if="!library.items.length && !library.loading"
      title="还没有入库任何游戏"
      description="到搜索页找一个游戏，或从本地导入已有的清单包。"
    >
      <template #action>
        <!-- UiButton 是纯 button，不接 to。导航由外层给回调，
             原语不 import router（宪法 11.1） -->
        <UiButton variant="primary" @click="router.push({ name: 'search' })">
          去搜索
        </UiButton>
      </template>
    </UiEmptyState>

    <template v-else>
      <header class="head">
        <h1 class="head__title">库概览</h1>
        <UiButton
          size="sm"
          :loading="library.loading"
          @click="library.refresh()"
        >
          {{ library.loading ? '扫描中' : '重新扫描' }}
        </UiButton>
      </header>

      <!-- 三格摘要。数字全部 tabular-nums（宪法速查第 9 条） -->
      <dl class="stats">
        <div class="stat">
          <dt class="stat__k">本工具管理</dt>
          <dd class="stat__v u-tnum">{{ managed.length }}</dd>
        </div>
        <div class="stat">
          <dt class="stat__k">已部署 DLC</dt>
          <dd class="stat__v u-tnum">{{ totalDlc }}</dd>
        </div>
        <div v-if="latest" class="stat">
          <dt class="stat__k">最近获取</dt>
          <dd class="stat__v stat__v--date u-tnum">{{ latest }}</dd>
        </div>
      </dl>

      <p class="hint">从左侧选一个游戏可查看详情并调整 DLC 勾选。</p>

      <!--
        异常区。三类分开讲，因为处置方式完全不同：
        冲突要手动删外部文件、孤立外部清单不归本工具管、记录丢失可直接删。
        混成一句「有 3 个异常」用户无从下手。
      -->
      <section v-if="conflicted.length" class="warn warn--attention">
        <h2 class="warn__title">
          {{ conflicted.length }} 个游戏同时被外部清单声明
        </h2>
        <p class="warn__body">
          卸载这些游戏时本工具只能删掉自己那一份文件，游戏会继续留在 Steam 库里。
          彻底移除需手动删除对应的外部清单文件。
        </p>
        <ul class="warn__list">
          <li v-for="i in conflicted" :key="i.mainAppID">
            {{ i.gameName }}
            <span class="warn__files">{{ i.fileNames.join('、') }}</span>
          </li>
        </ul>
      </section>

      <section v-if="orphaned.length" class="warn">
        <h2 class="warn__title">
          部署目录中有 {{ orphaned.length }} 份外部清单
        </h2>
        <p class="warn__body">
          这些文件不在本工具的记录里，可能是手动放入或由其他工具产生。
          注入器会加载目录内全部清单的并集，因此它们同样在生效中。
          其中可能含特意配置的内容，本工具不会自动清理。
        </p>
      </section>

      <section v-if="lostRecord.length" class="warn">
        <h2 class="warn__title">
          {{ lostRecord.length }} 个条目的安装记录已丢失
        </h2>
        <p class="warn__body">
          清单文件仍在生效，但本工具的账本里没有它们，因此无法还原当初的
          DLC 勾选项。可以在侧栏选中后删除，或重新获取一次。
        </p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
}

.head__title {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}

.stats {
  display: flex;
  gap: var(--space-3);
  margin: 0;
}

.stat {
  flex: 1 1 0;
  /* flex 项默认 min-width:auto，不写这条长数字会把格子撑破 */
  min-width: 0;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-1);
}

.stat__k {
  margin: 0 0 var(--space-1);
  color: var(--color-text-dim);
  font-size: var(--text-xs);
}

.stat__v {
  margin: 0;
  color: var(--color-text);
  font-size: var(--text-xl);
  font-weight: var(--weight-semibold);
  line-height: var(--leading-tight);
}

/* 日期不是「量」，不该和计数一样醒目 */
.stat__v--date {
  font-size: var(--text-md);
  font-weight: var(--weight-medium);
}

.hint {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.warn {
  padding: var(--space-4);
  /* 同心圆角：内边距 16px 的面板，内部元素圆角须相应收窄 */
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
}

/*
  只有「冲突」这一类淡染。
  另两类是需知情的事实而非待处置的问题，同样淡染会让三块一样吵，
  用户就分不出哪个真要动手——宪法 4.1 的点缀色纪律同理适用于状态色。
*/
.warn--attention {
  border-color: color-mix(in srgb, var(--state-warn) 40%, var(--color-border));
  background: color-mix(in srgb, var(--state-warn) var(--state-wash), var(--color-surface));
}

.warn__title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-md);
  font-weight: var(--weight-medium);
}

.warn__body {
  margin: 0;
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.warn__list {
  margin: var(--space-3) 0 0;
  padding-left: var(--space-4);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
}

.warn__files {
  display: block;
  color: var(--color-text-dim);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}
</style>
