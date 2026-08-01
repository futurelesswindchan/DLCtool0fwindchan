<script setup lang="ts">
/**
 * 原语预览页（仅开发构建可达）
 *
 * 存在理由：第 2 步只建原语、不改任何页面，若无此页则十三个组件全部无人引用，
 * 会被 tree-shake 掉——**构建通过但实际一行都没验证过**。
 *
 * 这一页让「原语可用」这个验证要点变成看得见的东西，
 * 同时也是双主题、焦点环、浮层裁切三项风险的实机验证场。
 *
 * ⚠️ 路由仅在开发构建注册（router/index.ts 按 import.meta.env.DEV 判断），
 *    不会进封测包。第 6 步完成后可整体删除，或保留作为回归自查页——
 *    倾向保留：它的维护成本几乎为零，而每次改原语都需要一个地方一眼看全。
 */

import { ref } from 'vue'
import {
  UiButton,
  UiCheckbox,
  UiRadio,
  UiSwitch,
  UiInput,
  UiSelect,
  UiSegmented,
  UiProgress,
  UiTooltip,
  UiHelpBadge,
  UiEmptyState,
  UiScrollArea,
  type SelectOption,
} from '../components/ui'

const checked = ref(true)
const partial = ref(false)
const radioVal = ref<string | number>('a')
const switched = ref(true)
const text = ref('')
const mono = ref('D:\\steam\\config\\lua')
const selectVal = ref<string | number>('hubcap')
const segVal = ref<string | number>('dark')
const progress = ref(42)

const sources: SelectOption[] = [
  { label: 'Hubcap', value: 'hubcap', hint: 'api-zip' },
  { label: 'MAU', value: 'mau', hint: 'github-branch' },
  { label: 'tymolu233', value: 'tymolu', hint: 'github-branch' },
  { label: '已停用的源', value: 'dead', hint: '不可用', disabled: true },
]

const themes = [
  { label: '深色', value: 'dark' },
  { label: '浅色', value: 'light' },
  { label: '跟随系统', value: 'auto' },
]

/** 切主题不走 store，直接改 data-theme——本页是独立验证场，不该有副作用 */
function toggleTheme() {
  const el = document.documentElement
  el.dataset.theme = el.dataset.theme === 'light' ? 'dark' : 'light'
}
</script>

<template>
  <div class="gal">
    <header class="gal__head">
      <h1>原语预览</h1>
      <UiButton variant="ghost" @click="toggleTheme">切换主题</UiButton>
    </header>

    <section class="gal__sec">
      <h2>UiButton</h2>
      <div class="gal__row">
        <UiButton>默认</UiButton>
        <UiButton variant="primary">主操作</UiButton>
        <UiButton variant="danger">彻底卸载</UiButton>
        <UiButton variant="ghost">幽灵</UiButton>
        <UiButton disabled>禁用</UiButton>
        <UiButton loading>进行中</UiButton>
        <UiButton size="sm">小号</UiButton>
        <UiButton icon aria-label="关闭">×</UiButton>
      </div>
    </section>

    <section class="gal__sec">
      <h2>选择类</h2>
      <div class="gal__row">
        <UiCheckbox v-model="checked" label="已勾选" />
        <UiCheckbox v-model="partial" indeterminate label="部分选中" />
        <UiCheckbox disabled label="禁用" />
        <UiSwitch v-model="switched" label="启用此源" />
      </div>
      <div class="gal__row">
        <UiRadio v-model="radioVal" name="demo" value="a" label="选项 A" />
        <UiRadio v-model="radioVal" name="demo" value="b" label="选项 B" />
        <UiRadio v-model="radioVal" name="demo" value="c" disabled label="禁用" />
      </div>
      <div class="gal__row">
        <UiSegmented v-model="segVal" :options="themes" />
      </div>
    </section>

    <section class="gal__sec">
      <h2>输入类</h2>
      <div class="gal__col">
        <UiInput v-model="text" placeholder="搜索游戏名或 AppID" />
        <UiInput v-model="mono" mono readonly />
        <UiInput v-model="text" invalid placeholder="校验失败态" />
        <UiInput v-model="text" disabled placeholder="禁用" />
        <UiSelect v-model="selectVal" :options="sources" />
      </div>
    </section>

    <section class="gal__sec">
      <h2>UiProgress</h2>
      <div class="gal__col">
        <UiProgress :value="progress" label="正在试下载 3 / 7 个源" />
        <UiProgress label="正在查询收录情况" />
        <UiProgress :value="100" tone="ok" label="完成" />
        <UiProgress :value="64" tone="danger" label="部分源失败" />
      </div>
      <div class="gal__row">
        <UiButton size="sm" @click="progress = Math.max(0, progress - 20)">
          −20
        </UiButton>
        <UiButton size="sm" @click="progress = Math.min(100, progress + 20)">
          +20
        </UiButton>
      </div>
    </section>

    <section class="gal__sec">
      <h2>提示类</h2>
      <div class="gal__row">
        <UiTooltip content="这段文字只是补充解释，不放关键信息">
          <UiButton size="sm">悬停我</UiButton>
        </UiTooltip>
        <span class="gal__term">
          manifest
          <UiHelpBadge
            content="清单文件。记录某个 depot 在特定版本下包含哪些文件及其校验值，OST 可据 GID 自行下载，故源只需提供密钥与 GID 两串文本。"
          />
        </span>
      </div>
    </section>

    <section class="gal__sec">
      <h2>UiEmptyState</h2>
      <UiEmptyState
        title="社区还没收录这个游戏"
        :description="[
          '这不是你操作错了——已试过的源都没有这个 AppID 的清单。',
          '可以换个源再试，或者从本地导入已有的 lua 文件。',
        ]"
      >
        <template #action>
          <UiButton variant="primary">本地导入</UiButton>
          <UiButton>重试全部源</UiButton>
        </template>
      </UiEmptyState>

      <UiEmptyState
        tone="error"
        size="compact"
        title="连接源失败：TLS 握手被中断"
        description="国内网络环境下偶发。稍后重试通常可恢复。"
      >
        <template #action>
          <UiButton size="sm">导出诊断包</UiButton>
        </template>
      </UiEmptyState>
    </section>

    <section class="gal__sec">
      <h2>UiScrollArea</h2>
      <div class="gal__scrollbox">
        <UiScrollArea long-list>
          <div v-for="i in 40" :key="i" class="gal__item u-tnum">
            第 {{ i }} 项 · AppID {{ 1000000 + i * 137 }}
          </div>
        </UiScrollArea>
      </div>
    </section>
  </div>
</template>

<style scoped>
.gal {
  height: 100%;
  overflow-y: auto;
  padding: var(--space-5);
  max-width: var(--content-max-w);
  margin: 0 auto;
}

.gal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-5);
}

.gal__head h1 {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
}

.gal__sec {
  margin-bottom: var(--space-6);
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--elev-1), var(--hairline-top);
}

.gal__sec h2 {
  margin: 0 0 var(--space-3);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.gal__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-3);
}

.gal__row:last-child {
  margin-bottom: 0;
}

.gal__col {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  max-width: 420px;
  margin-bottom: var(--space-3);
}

.gal__term {
  display: inline-flex;
  align-items: center;
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}

.gal__scrollbox {
  display: flex;
  height: 180px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-ctrl);
}

.gal__item {
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--color-border);
  font-size: var(--text-sm);
}
</style>
