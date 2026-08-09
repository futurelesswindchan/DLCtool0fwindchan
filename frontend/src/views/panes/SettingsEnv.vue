<script setup lang="ts">
/**
 * 设置 · 环境
 *
 * Steam 路径是整条部署链路的唯一权威（`DECISIONS` 07-27），故它在设置页
 * 排第一位，且提供手动与自动两条设置途径——自动识别在多库、非默认盘、
 * 便携版 Steam 下都可能失手，此时手动选择是唯一出路。
 *
 * 本 Pane 不提供注入器的安装或修复入口。那是项目三条铁律之一
 * （不负责安装/修复注入器），引导页只做检测与文字说明。
 */

import { useEnvStore } from '../../stores/env'
import { useToast } from '../../composables/useToast'
import { selectDirectory } from '../../api'
import EnvBanner from '../../components/EnvBanner.vue'
import { UiButton, UiHelpBadge } from '../../components/ui'
import { glossary } from '../../glossary'

const env = useEnvStore()
const toast = useToast()

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
    const path = await env.autoDetect()
    toast.success(`已识别到 ${path}`)
  } catch (e) {
    toast.fromError(e, '自动识别失败')
  }
}
</script>

<template>
  <section class="pane">
    <h2 class="set-block__title">
      环境
      <UiHelpBadge :content="glossary.injector.short" aria-label="了解「注入器 / OST」" />
    </h2>

    <EnvBanner
      :result="env.result"
      :loading="env.loading"
      @recheck="env.refresh()"
      @set-path="onPickSteamPath"
    />

    <dl class="set-kv">
      <dt>Steam 路径</dt>
      <dd>
        <code>{{ env.steamPath || '未设置' }}</code>
        <span class="set-kv__btns">
          <UiButton size="sm" @click="onPickSteamPath">手动选择</UiButton>
          <UiButton size="sm" @click="onAutoDetect">自动识别</UiButton>
        </span>
      </dd>

      <dt>清单写入目录</dt>
      <dd><code>{{ env.deployDir || '—' }}</code></dd>
    </dl>
  </section>
</template>

<style scoped>
.pane {
  /*
    760px 比 ContentPane 的 1040 更窄，这是刻意的，不是漏改。
    设置项以说明性长文本为主，行宽超过约 75 字符后眼睛回行会找错行。
    ContentPane 那 1040 是「上限」，页面可以更窄，但不该更宽。
  */
  max-width: 760px;
}
</style>
