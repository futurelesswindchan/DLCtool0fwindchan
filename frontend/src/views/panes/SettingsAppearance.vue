<script setup lang="ts">
/**
 * 设置 · 外观
 *
 * 只有主题一项，这不是「没做完」——本项目的设置页方向是**少而准**
 * （宪法第 9 章，与 PCL2 反向），因为本工具的价值就是把复杂性吃掉。
 * 新增任何一项都须先回答「不加会不会卡住用户」。
 *
 * 三档而非两档：「跟随系统」是真需求，Windows 会在日落时自动切换，
 * 缺这一档的话用户每天要手动改一次。
 *
 * ⚠️ 三档的合法集合与 Go 侧 normalize 必须等价。第 2 步实机时踩过一次：
 *    后端静默把 'system' 改写成 'dark'，界面上表现为「这个选项点了没反应」——
 *    静默改写把「不支持」伪装成了「支持但坏了」（`DECISIONS-2` 08-01）。
 */

import { useConfigStore, type Theme } from '../../stores/config'
import { UiSegmented } from '../../components/ui'
import type { SegmentedOption } from '../../components/ui'

const config = useConfigStore()

const themes: SegmentedOption[] = [
  { value: 'dark', label: '深色' },
  { value: 'light', label: '浅色' },
  { value: 'system', label: '跟随系统' },
]

/**
 * 分段控件的 v-model 中转。
 *
 * 不直接 v-model 到 config.theme：store 的 setTheme 还要落盘到后端，
 * 而分段控件只吐一个值。中转一层可以让「切换主题」这个动作保持单一入口。
 */
function onChange(v: string | number) {
  config.setTheme(v as Theme)
}
</script>

<template>
  <section class="pane">
    <h2 class="set-block__title">外观</h2>

    <div class="row">
      <span class="row__label">主题</span>
      <UiSegmented
        :model-value="config.theme"
        :options="themes"
        @update:model-value="onChange"
      />
    </div>

    <p class="set-hint set-hint--dim">
      「跟随系统」会在系统主题变化时即时切换，不需要重启。
    </p>
  </section>
</template>

<style scoped>
.pane {
  max-width: 760px;
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  margin-bottom: var(--space-2);
}

.row__label {
  color: var(--color-text-muted);
  font-size: var(--text-base);
}
</style>
