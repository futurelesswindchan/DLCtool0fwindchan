<script setup lang="ts">
/**
 * L1.5 测试台 · 最小可用界面
 *
 * 目标：跑通 EnvBanner + DropZone + PackagePanel 三个组件，
 * 验证后端 API 实际可用。样式粗糙无所谓，正式界面后续重做。
 */

import { ref, onMounted } from 'vue'
import {
  detectEnvironment,
  autoDetectSteamPath,
  setSteamPath,
  selectDirectory,
  selectZipFile,
  processZipFile,
  processDroppedFile,
  installDLCs,
  removeDLCs,
  getHistory,
  getDeployDir,
  openDataDir,
} from './api'
import type { DetectorResult, GamePackage, GameRecord } from './api'
import EnvBanner from './components/EnvBanner.vue'
import DropZone from './components/DropZone.vue'
import PackagePanel from './components/PackagePanel.vue'

/* ─── 状态 ─── */

const envResult = ref<DetectorResult | null>(null)
const envLoading = ref(false)

const pkg = ref<GamePackage | null>(null)
const selectedIDs = ref<string[]>([])
const busy = ref(false)

const history = ref<GameRecord[]>([])
const deployDir = ref('')
const statusMsg = ref('')

/* ─── 环境检测 ─── */

async function checkEnv() {
  envLoading.value = true
  try {
    envResult.value = await detectEnvironment()
  } catch (e: any) {
    statusMsg.value = `检测失败: ${e.message ?? e}`
  } finally {
    envLoading.value = false
  }
}

async function onSetPath() {
  const dir = await selectDirectory()
  if (!dir) return
  const res = await setSteamPath(dir)
  statusMsg.value = res.message
  if (res.success) await checkEnv()
}

/* ─── 清单包处理 ─── */

async function onPickFile() {
  busy.value = true
  try {
    const path = await selectZipFile()
    if (!path) return
    pkg.value = await processZipFile(path)
    selectedIDs.value = pkg.value.dlcs.map((d) => d.appID)
  } catch (e: any) {
    statusMsg.value = `解析失败: ${e.message ?? e}`
  } finally {
    busy.value = false
  }
}

async function onDropFile(file: File) {
  busy.value = true
  try {
    pkg.value = await processDroppedFile(file)
    selectedIDs.value = pkg.value.dlcs.map((d) => d.appID)
  } catch (e: any) {
    statusMsg.value = `解析失败: ${e.message ?? e}`
  } finally {
    busy.value = false
  }
}

/* ─── 部署 / 移除 ─── */

async function onInstall() {
  if (!pkg.value) return
  busy.value = true
  try {
    const res = await installDLCs(pkg.value, selectedIDs.value)
    statusMsg.value = res.message
    if (res.success) await refreshHistory()
  } catch (e: any) {
    statusMsg.value = `部署失败: ${e.message ?? e}`
  } finally {
    busy.value = false
  }
}

async function onRemove() {
  if (!pkg.value) return
  busy.value = true
  try {
    const res = await removeDLCs(pkg.value.mainAppID)
    statusMsg.value = res.message
    if (res.success) {
      pkg.value = null
      selectedIDs.value = []
      await refreshHistory()
    }
  } catch (e: any) {
    statusMsg.value = `移除失败: ${e.message ?? e}`
  } finally {
    busy.value = false
  }
}

/* ─── 历史与辅助 ─── */

async function refreshHistory() {
  history.value = await getHistory()
}

onMounted(async () => {
  await checkEnv()
  deployDir.value = await getDeployDir()
  await refreshHistory()
})
</script>

<template>
  <div class="app">
    <!-- 状态消息条 -->
    <div v-if="statusMsg" class="toast" @click="statusMsg = ''">
      {{ statusMsg }}
    </div>

    <header class="header">
      <h1 class="header__title">风兔盒 <small>L1.5 测试台</small></h1>
      <button class="btn" @click="openDataDir()">打开数据目录</button>
    </header>

    <main class="main">
      <!-- 环境检测横幅 -->
      <EnvBanner
        :result="envResult"
        :loading="envLoading"
        @recheck="checkEnv"
        @set-path="onSetPath"
      />

      <!-- 部署目录提示 -->
      <p v-if="deployDir" class="deploy-hint">
        清单将写入：<code>{{ deployDir }}</code>
      </p>

      <!-- 拖拽/选择清单包 -->
      <DropZone
        :busy="busy"
        @drop-file="onDropFile"
        @pick-file="onPickFile"
      />

      <!-- 清单包解析结果 -->
      <PackagePanel
        v-if="pkg"
        :pkg="pkg"
        :selected="selectedIDs"
        :busy="busy"
        @update:selected="selectedIDs = $event"
        @install="onInstall"
        @remove="onRemove"
      />

      <!-- 历史记录（简单列表） -->
      <section v-if="history.length" class="history">
        <h2 class="history__title">安装历史 ({{ history.length }})</h2>
        <ul class="history__list">
          <li v-for="rec in history" :key="rec.mainAppID" class="history__item">
            <strong>{{ rec.gameName }}</strong>
            <span class="history__meta">
              AppID {{ rec.mainAppID }} · {{ rec.dlcCount }} DLC ·
              {{ rec.installedAt }}
            </span>
          </li>
        </ul>
      </section>
    </main>
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100vh;
  padding: var(--space-4);
  gap: var(--space-4);
  overflow-y: auto;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header__title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
}

.header__title small {
  font-size: 0.75rem;
  color: var(--color-text-dim);
  font-weight: 400;
  margin-left: var(--space-2);
}

.main {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  flex: 1;
  min-height: 0;
}

.deploy-hint {
  margin: 0;
  font-size: 0.8rem;
  color: var(--color-text-muted);
}

.deploy-hint code {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-text-dim);
  word-break: break-all;
}

.toast {
  position: fixed;
  top: var(--space-4);
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  font-size: 0.85rem;
  cursor: pointer;
  max-width: 80%;
  text-align: center;
}

.history__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.history__list {
  margin: var(--space-2) 0 0;
  padding: 0;
  list-style: none;
}

.history__item {
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--color-border);
}

.history__meta {
  display: block;
  font-size: 0.75rem;
  color: var(--color-text-dim);
  margin-top: var(--space-1);
}

/* 通用按钮（测试台简化版） */
.btn {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg-elevated);
  color: var(--color-text);
  font-size: 0.8rem;
  cursor: pointer;
}

.btn:hover {
  background: var(--color-bg-hover);
}
</style>
