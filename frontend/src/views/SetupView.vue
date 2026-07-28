<script setup lang="ts">
/**
 * 引导页
 *
 * 注入器未安装是流失率最高的环节，故给出明确的三步引导而非一句错误提示。
 *
 * 本工具不负责安装或修复注入器（三条铁律之一），因此这里只做说明与检测，
 * 实际的下载与放置由用户自行完成。文案需说清「要把什么放到哪里」，
 * 而不是含糊地说「请安装 OpenSteamTool」。
 */

import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useEnvStore } from '../stores/env'
import { useToast } from '../composables/useToast'
import { selectDirectory } from '../api'

const router = useRouter()
const env = useEnvStore()
const toast = useToast()

/** 第一步是否已完成：Steam 路径已确定 */
const step1Done = computed(() => !!env.steamPath)

/** 第二步是否已完成：三个 DLL 均在位 */
const step2Done = computed(() => env.ready)

async function onPickSteamPath() {
  const dir = await selectDirectory()
  if (!dir) return
  try {
    toast.success(await env.setSteamPath(dir))
  } catch (e) {
    toast.fromError(e, '设置 Steam 路径失败')
  }
}

async function onAutoDetect() {
  try {
    toast.success(`已识别到 ${await env.autoDetect()}`)
  } catch (e) {
    toast.fromError(e, '自动识别失败')
  }
}

async function onRecheck() {
  await env.refresh()
  if (env.ready) toast.success('注入器已就绪，可以开始入库了')
  else toast.warn('仍未检测到注入器，请确认文件已放入 Steam 根目录')
}
</script>

<template>
  <div class="page">
    <header class="intro">
      <h1 class="intro__title">开始之前，需要三步准备</h1>
      <p class="intro__desc">
        风兔盒负责把清单文件放到正确位置，实际让 Steam 认账的是注入器
        OpenSteamTool。二者分工明确，因此注入器需要由你自行安装一次，
        之后就不用再管了。
      </p>
    </header>

    <ol class="steps">
      <!-- 步骤一：Steam 路径 -->
      <li class="step" :class="{ 'step--done': step1Done }">
        <span class="step__no">{{ step1Done ? '✓' : '1' }}</span>
        <div class="step__body">
          <h2 class="step__title">确定 Steam 安装位置</h2>
          <p class="step__desc">
            通常能自动识别。若你的 Steam 安装在非默认位置，请手动选择。
          </p>
          <p v-if="env.steamPath" class="step__path">
            <code>{{ env.steamPath }}</code>
          </p>
          <div class="step__actions">
            <button class="btn" @click="onAutoDetect">自动识别</button>
            <button class="btn" @click="onPickSteamPath">手动选择</button>
          </div>
        </div>
      </li>

      <!-- 步骤二：放入注入器 -->
      <li class="step" :class="{ 'step--done': step2Done }">
        <span class="step__no">{{ step2Done ? '✓' : '2' }}</span>
        <div class="step__body">
          <h2 class="step__title">把注入器文件放进 Steam 根目录</h2>
          <p class="step__desc">
            从 OpenSteamTool 的发布页取得压缩包，把其中的三个 DLL 放到 Steam
            根目录下（与 <code>steam.exe</code> 同一层）。
            <code>.lib</code> 与 <code>.exp</code> 之类的文件不需要。
          </p>

          <ul v-if="env.missingFiles.length" class="step__missing">
            <li v-for="f in env.missingFiles" :key="f">
              缺少 <code>{{ f }}</code>
            </li>
          </ul>

          <p class="step__warn">
            同时只能启用一个入库工具。若此前用过 SteamTools，请先停用——两者
            共用同名的 <code>dwmapi.dll</code>，混用会互相覆盖。
          </p>
        </div>
      </li>

      <!-- 步骤三：重启 Steam -->
      <li class="step">
        <span class="step__no">3</span>
        <div class="step__body">
          <h2 class="step__title">重启 Steam</h2>
          <p class="step__desc">
            注入器需要随 Steam 启动才会生效。之后每次入库都会自动同步，
            无需再重启。
          </p>
          <p class="step__desc">
            准备就绪后的流程是：搜索游戏 → 入库 → 勾选想要的 DLC。勾选约 1 秒后
            自动写入，Steam 库会在几秒内出现对应条目；部分 DLC 还需在 Steam 里
            另行下载内容。
          </p>
        </div>
      </li>
    </ol>

    <div class="foot">
      <button class="btn btn--primary" :disabled="env.loading" @click="onRecheck">
        {{ env.loading ? '检测中…' : '重新检测' }}
      </button>
      <button
        v-if="env.ready"
        class="btn"
        @click="router.push({ name: 'search' })"
      >
        开始使用 →
      </button>
      <button v-else class="btn" @click="router.push({ name: 'search' })">
        先看看界面
      </button>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  max-width: 700px;
  margin: 0 auto;
}

.intro__title {
  margin: 0 0 var(--space-2);
  font-size: 1.15rem;
}

.intro__desc {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.85rem;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
  counter-reset: step;
}

.step {
  display: flex;
  gap: var(--space-4);
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-left-width: 3px;
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  transition: border-color var(--duration-base) var(--ease-out);
}

.step--done {
  border-left-color: var(--color-success);
}

.step__no {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--color-bg-hover);
  font-size: 0.8rem;
  font-weight: 700;
}

.step--done .step__no {
  color: var(--color-success);
}

.step__body {
  min-width: 0;
}

.step__title {
  margin: 0 0 var(--space-1);
  font-size: 0.92rem;
  font-weight: 500;
}

.step__desc {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.82rem;
}

.step__path {
  margin: var(--space-2) 0 0;
  font-size: 0.75rem;
  word-break: break-all;
}

.step__missing {
  margin: var(--space-2) 0 0;
  padding-left: var(--space-4);
  color: var(--color-warning);
  font-size: 0.8rem;
}

.step__warn {
  margin: var(--space-2) 0 0;
  color: var(--color-warning);
  font-size: 0.8rem;
}

.step__actions {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-3);
}

.foot {
  display: flex;
  gap: var(--space-2);
}
</style>
