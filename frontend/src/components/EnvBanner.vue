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
      <button
        v-if="needSetPath"
        type="button"
        class="btn btn--primary"
        @click="$emit('setPath')"
      >
        设置 Steam 路径
      </button>
      <button
        type="button"
        class="btn"
        :disabled="loading"
        @click="$emit('recheck')"
      >
        重新检测
      </button>
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
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
}

/* 仅用左边框色区分状态，不改整块背景——满屏色块会压过内容。 */
.banner--ready {
  border-left-color: var(--color-success);
}

.banner--missing {
  border-left-color: var(--color-danger);
}

.banner--unknown {
  border-left-color: var(--color-warning);
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
  font-size: 0.8rem;
  font-weight: 700;
  background: var(--color-bg-hover);
}

.banner--ready .banner__icon {
  color: var(--color-success);
}

.banner--missing .banner__icon {
  color: var(--color-danger);
}

.banner--unknown .banner__icon {
  color: var(--color-warning);
}

.banner__body {
  flex: 1;
  min-width: 0;
}

.banner__message {
  margin: 0;
  font-weight: 500;
}

.banner__detail {
  margin: var(--space-1) 0 0;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-text-dim);

  /* 路径可能很长，允许换行而非撑破布局 */
  word-break: break-all;
}

.banner__missing {
  margin: var(--space-2) 0 0;
  padding-left: var(--space-4);
  font-size: 0.8rem;
  color: var(--color-text-muted);
}

.banner__actions {
  display: flex;
  flex-shrink: 0;
  gap: var(--space-2);
}
</style>
