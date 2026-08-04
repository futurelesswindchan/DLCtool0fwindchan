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

/**
 * 压力档开关。
 *
 * 默认关：这一页平时是「一眼看全」用的，常驻两百行会让它自己变卡，
 * 反过来污染其他原语的观感判断。
 */
const stress = ref(false)

/** 压力档的勾选集合。用 Set 而非每行一个 ref，与 DlcList 的真实形态一致。 */
const stressPicked = ref(new Set<number>())

function toggleStress(i: number) {
  const s = new Set(stressPicked.value)
  s.has(i) ? s.delete(i) : s.add(i)
  stressPicked.value = s
}

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

      <!-- 长文案截断：DlcList 的 DLC 名可以很长，而 .cb__label 若不带
           min-width: 0，flex 项就不会收缩到内容宽度以下，插槽里的 ellipsis
           永远不生效、整行被撑破。这一格就是那条修复的看守。 -->
      <div class="gal__truncbox">
        <UiCheckbox v-model="checked">
          <span class="gal__trunc">
            超长 DLC 名称测试 · Monster Hunter World Iceborne Master Edition
            Deluxe Kit Complete Bundle
          </span>
        </UiCheckbox>
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

      <!--
        两档必须并排：原先四个 UiInput 全是默认 md 档，于是「sm 与 md 到底
        差在哪」在本页看不出来——而这一页的存在理由正是「一眼看全」。
        缺了对照的档位差异无从发现，与速查第 37 条（判据本身也要有判据）
        是同一形状。

        实际差异只有内边距与最小高度（28px / 32px），字号两档同为
        --text-sm：输入框里是用户自己的内容，属正文，不随控件尺寸缩放。
        UiSelect 同构。这是刻意的，不是漏改。
      -->
      <div class="gal__col">
        <UiInput v-model="text" size="sm" placeholder="size=sm，高 28px" />
        <UiInput v-model="text" size="md" placeholder="size=md，高 32px" />
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

    <!--
      压力档：UiCheckbox 在 DlcList 那种规模下的实测场。

      为什么需要它：换掉原生 checkbox 后每行元素数由 6 涨到 12，其中 4 个是
      SVG 节点（勾与横线各含一条 path）。MHW(582010) 有 200 个 DLC，即整表
      2400 个元素、800 个 SVG——这个量级不该靠估。

      取 200 行是照实际最坏样本定的，不是随手取的整数。

      NOTE: 本段刻意不加 content-visibility。DlcList 的 .dlc__list 有那条
      属性护着，若这里也加上，测的就是「被跳过渲染的行」，等于把要测的东西
      屏蔽掉了——判据本身也要有判据（速查第 37 条）。
    -->
    <section class="gal__sec">
      <h2>压力档 · UiCheckbox × 200</h2>
      <div class="gal__row">
        <UiSwitch v-model="stress" label="渲染 200 行（默认关，避免本页自己变卡）" />
        <span v-if="stress" class="gal__note u-tnum">
          已选 {{ stressPicked.size }} 项
        </span>
      </div>
      <ul v-if="stress" class="gal__stress">
        <li v-for="i in 200" :key="i" class="gal__stressrow">
          <UiCheckbox
            class="gal__stresscheck"
            :model-value="stressPicked.has(i)"
            @update:model-value="toggleStress(i)"
          >
            <span class="gal__trunc">
              第 {{ i }} 个 DLC · Monster Hunter World Iceborne 追加内容包
            </span>
          </UiCheckbox>
          <span class="gal__stressid u-tnum">{{ 2000000 + i * 13 }}</span>
        </li>
      </ul>
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

/* 刻意窄，好让截断真的发生——容器够宽的话这一格就验不出任何东西 */
.gal__truncbox {
  display: flex;
  width: 260px;
  padding: var(--space-2);
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-ctrl);
}

.gal__trunc {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.gal__note {
  color: var(--color-text-dim);
  font-size: var(--text-sm);
}

/* 压力档列表。刻意复刻 DlcList 的行结构（勾选框吃剩余宽 + 右侧等宽 AppID），
   否则测出来的不是那一页的实际形态 */
.gal__stress {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  overflow: hidden;
}

.gal__stressrow {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  font-size: var(--text-base);
}

.gal__stressrow + .gal__stressrow {
  border-top: 1px solid var(--color-border);
}

.gal__stressrow:hover {
  background: var(--color-surface-2);
}

.gal__stresscheck {
  flex: 1 1 auto;
  min-width: 0;
}

.gal__stressid {
  flex: 0 0 auto;
  color: var(--color-text-dim);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
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
