<script setup lang="ts">
/**
 * 环境状态横幅
 *
 * 展示注入器检测结果，三态各对应不同的用户处境与指引：
 *   ready   已就绪，可以入库
 *   missing 已检测但注入器未安装 → 引导用户去装
 *   unknown Steam 路径无效导致无法检测 → 引导用户先设路径
 *
 * 区分后两者很重要：若把「路径没设」也显示成「未安装注入器」，
 * 用户会跑去装注入器，装完发现还是红的。
 */

import { computed } from 'vue'
import type { DetectorResult } from '../api'
import { UiButton } from './ui'

const props = defineProps<{
  result: DetectorResult | null
  loading: boolean
}>()

defineEmits<{
  recheck: []
  setPath: []
}>()

/** 状态对应的视觉修饰类，驱动横幅配色。 */
const statusClass = computed(() => {
  if (props.loading || !props.result) return 'banner--pending'
  switch (props.result.status) {
    case 'ready':
      return 'banner--ready'
    case 'missing':
      return 'banner--missing'
    default:
      return 'banner--unknown'
  }
})

/** 状态图标。用文字符号而非图标字体，避免引入额外依赖。 */
const icon = computed(() => {
  if (props.loading || !props.result) return '…'
  switch (props.result.status) {
    case 'ready':
      return '✓'
    case 'missing':
      return '✕'
    default:
      return '?'
  }
})

/** 仅在 unknown 状态下提示用户去设置路径，其余状态该按钮无意义。 */
const needSetPath = computed(() => props.result?.status === 'unknown')
</script>

<template>
  <section class="banner" :class="statusClass">
    <span class="banner__icon" aria-hidden="true">{{ icon }}</span>

    <div class="banner__body">
      <p class="banner__message">
        {{ loading ? '正在检测注入器环境…' : result?.message ?? '尚未检测' }}
      </p>

      <p v-if="result?.checkedPath" class="banner__detail">
        检查目录：{{ result.checkedPath }}
      </p>

      <ul
        v-if="result?.missingFiles?.length"
        class="banner__missing"
      >
        <li v-for="file in result.missingFiles" :key="file">
          缺少 {{ file }}
        </li>
      </ul>
    </div>

    <div class="banner__actions">
      <UiButton
        v-if="needSetPath"
        variant="primary"
        @click="$emit('setPath')"
      >
        设置 Steam 路径
      </UiButton>
      <!-- 检测中用 loading 而非 disabled：两者都禁用点击，但 loading 的
           cursor: progress 表达的是「正在做」，disabled 表达「不能做」。
           此处按钮随时可用，只是这一刻正忙。 -->
      <UiButton :loading="loading" @click="$emit('recheck')">
        重新检测
      </UiButton>
    </div>
  </section>
</template>

<style scoped>
.banner {
  display: flex;
  gap: var(--space-4);
  align-items: flex-start;
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-left-width: 3px;
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-1);
}

/* 仅用左边框色区分状态，不改整块背景——满屏色块会压过内容。 */
.banner--ready {
  border-left-color: var(--state-ok);
}

.banner--missing {
  border-left-color: var(--state-danger);
}

.banner--unknown {
  border-left-color: var(--state-warn);
}

.banner--pending {
  border-left-color: var(--color-text-dim);
}

.banner__icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  font-size: var(--text-sm);
  /* 700 收到 --weight-semibold(600)：字重只用三档（4.3 节），
     且 600 在 12px 上已经足够醒目，700 只会让字形显得糊 */
  font-weight: var(--weight-semibold);
  background: var(--color-surface-2);
}

.banner--ready .banner__icon {
  color: var(--state-ok);
}

.banner--missing .banner__icon {
  color: var(--state-danger);
}

.banner--unknown .banner__icon {
  color: var(--state-warn);
}

.banner__body {
  flex: 1;
  min-width: 0;
}

.banner__message {
  margin: 0;
  font-weight: var(--weight-medium);
}

.banner__detail {
  margin: var(--space-1) 0 0;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  color: var(--color-text-dim);

  /* 路径可能很长，允许换行而非撑破布局 */
  word-break: break-all;
}

.banner__missing {
  margin: var(--space-2) 0 0;
  padding-left: var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}

.banner__actions {
  display: flex;
  flex-shrink: 0;
  gap: var(--space-2);
}
</style>
