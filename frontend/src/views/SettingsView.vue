<script setup lang="ts">
/**
 * 设置页
 *
 * 四区：环境、清单源、外观、关于。
 *
 * NOTE: 后端的 CheckUpdate / OpenURL / GetAppVersion 尚未实现，因此本页
 * 暂不提供「检查更新」与外链按钮，版本号取自前端构建常量。
 * TODO(后端): 上述三个方法实现后，在「关于」区补上对应入口。
 */

import { ref, onMounted } from 'vue'
import { useEnvStore } from '../stores/env'
import { useConfigStore, type Theme } from '../stores/config'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import {
  selectDirectory,
  getLogPath,
  openDataDir,
  setRepoToken,
  getMSiteStats,
  clearHistory,
  type MSiteStats,
} from '../api'
import EnvBanner from '../components/EnvBanner.vue'

const env = useEnvStore()
const config = useConfigStore()
const toast = useToast()
const confirm = useConfirm()

const logPath = ref('')
const msite = ref<MSiteStats | null>(null)
const tokenInput = ref('')
const savingToken = ref(false)

/** 认证型源的名称。与后端内置源配置保持一致。 */
const M_SITE = 'Hubcap Manifest'

onMounted(async () => {
  logPath.value = await getLogPath()
  await refreshMSite()
})

async function refreshMSite() {
  try {
    msite.value = await getMSiteStats()
  } catch {
    // 未配置凭据或查询失败都表现为无额度信息，不必打扰用户
    msite.value = null
  }
}

/* ─── 环境 ─── */

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

/* ─── 清单源凭据 ─── */

/**
 * 保存 M 站凭据。
 *
 * 输入框内容保存后立即清空，不回填已存的凭据——凭据属敏感信息，界面上
 * 无需也不应长期显示。是否已配置由额度信息的存在与否间接表达。
 */
async function onSaveToken() {
  savingToken.value = true
  try {
    await setRepoToken(M_SITE, tokenInput.value.trim())
    tokenInput.value = ''
    await refreshMSite()
    await config.refresh()
    toast.success('凭据已保存')
  } catch (e) {
    toast.fromError(e, '保存凭据失败')
  } finally {
    savingToken.value = false
  }
}

async function onClearToken() {
  const ok = await confirm({
    title: '清除 Hubcap Manifest 凭据？',
    body: '清除后该源将被跳过，其余两个源不受影响。',
    confirmText: '清除',
    danger: true,
  })
  if (!ok) return

  try {
    await setRepoToken(M_SITE, '')
    msite.value = null
    toast.success('凭据已清除')
  } catch (e) {
    toast.fromError(e, '清除凭据失败')
  }
}

/* ─── 杂项 ─── */

async function onOpenDataDir() {
  try {
    await openDataDir()
  } catch (e) {
    toast.fromError(e, '打开数据目录失败')
  }
}

/**
 * 清空安装记录。
 *
 * 只动账本，不动已部署的文件——这一点必须在确认文案里讲清楚，否则用户
 * 会以为清空记录等于卸载全部游戏。
 */
async function onClearHistory() {
  const ok = await confirm({
    title: '清空安装记录？',
    body: [
      '仅清空本工具的账本，已部署到 Steam 的清单文件不会被删除。',
      '清空后这些游戏会出现在「已安装」页的对账异常区里。',
    ],
    confirmText: '清空记录',
    danger: true,
  })
  if (!ok) return

  try {
    toast.success(await clearHistory())
  } catch (e) {
    toast.fromError(e, '清空记录失败')
  }
}

const themes: { value: Theme; label: string }[] = [
  { value: 'dark', label: '深色' },
  { value: 'light', label: '浅色' },
  { value: 'system', label: '跟随系统' },
]

/**
 * 把 RepoSource.Kind 译成用户能理解的说法。
 *
 * 后端的 kind 是访问形态的技术标识（决定走哪套下载逻辑），直接显示
 * 「githubBranch」对用户没有意义，只需让他知道「这个源要不要自己配凭据」。
 *
 * NOTE: 不渲染 RepoSource.token。该字段随配置一同跨界到前端，但凭据不该
 * 出现在界面上——是否已配置由额度信息间接表达即可。
 */
function kindLabel(kind: string): string {
  switch (kind) {
    case 'api-zip':
      return '需自备凭据'
    case 'github-branch':
    case 'zip-template':
      return '公开源'
    default:
      return kind
  }
}
</script>

<template>
  <div class="page">
    <section class="block">
      <h2 class="block__title">环境</h2>

      <EnvBanner
        :result="env.result"
        :loading="env.loading"
        @recheck="env.refresh()"
        @set-path="onPickSteamPath"
      />

      <dl class="kv">
        <dt>Steam 路径</dt>
        <dd>
          <code>{{ env.steamPath || '未设置' }}</code>
          <span class="kv__btns">
            <button class="btn" @click="onPickSteamPath">手动选择</button>
            <button class="btn" @click="onAutoDetect">自动识别</button>
          </span>
        </dd>

        <dt>清单写入目录</dt>
        <dd><code>{{ env.deployDir || '—' }}</code></dd>
      </dl>
    </section>

    <section class="block">
      <h2 class="block__title">清单源</h2>

      <ul class="sources">
        <li
          v-for="s in config.config?.repoSources ?? []"
          :key="s.name"
          class="source"
        >
          <span class="source__dot" :class="{ 'source__dot--off': !s.enabled }"></span>
          <span class="source__name">{{ s.name }}</span>
          <span class="source__type">{{ kindLabel(s.kind) }}</span>
          <span class="source__state">{{ s.enabled ? '启用' : '已停用' }}</span>
        </li>
      </ul>

      <div class="msite">
        <h3 class="msite__title">{{ M_SITE }} 凭据</h3>
        <p class="hint">
          该源数据最完整，但需自备 API key。凭据默认仅 7 天有效，到期后此源会被静默跳过。
        </p>

        <div v-if="msite" class="msite__stats">
          <span>账户 {{ msite.username }}</span>
          <span>今日 {{ msite.dailyUsage }}/{{ msite.dailyLimit }}</span>
          <span v-if="msite.expiresAt">
            到期 {{ msite.expiresAt.slice(0, 10) }}
          </span>
          <span v-if="msite.expiringSoon" class="msite__warn">即将到期</span>
          <span v-else-if="!msite.canMakeRequests" class="msite__warn">
            额度已用完
          </span>
        </div>
        <p v-else class="hint hint--dim">尚未配置凭据。</p>

        <div class="msite__form">
          <input
            v-model="tokenInput"
            class="input"
            type="password"
            placeholder="粘贴 API key"
            autocomplete="off"
          />
          <button
            class="btn btn--primary"
            :disabled="savingToken || !tokenInput.trim()"
            @click="onSaveToken"
          >
            保存
          </button>
          <button v-if="msite" class="btn btn--danger" @click="onClearToken">
            清除
          </button>
        </div>
      </div>
    </section>

    <section class="block">
      <h2 class="block__title">外观</h2>
      <div class="theme">
        <button
          v-for="t in themes"
          :key="t.value"
          class="btn"
          :class="{ 'btn--primary': config.theme === t.value }"
          @click="config.setTheme(t.value)"
        >
          {{ t.label }}
        </button>
      </div>
    </section>

    <section class="block">
      <h2 class="block__title">关于</h2>
      <dl class="kv">
        <dt>日志文件</dt>
        <dd><code>{{ logPath || '—' }}</code></dd>
        <dt>数据目录</dt>
        <dd>
          <span class="kv__btns">
            <button class="btn" @click="onOpenDataDir">在文件管理器中打开</button>
            <button class="btn btn--danger" @click="onClearHistory">
              清空安装记录
            </button>
          </span>
        </dd>
      </dl>
    </section>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  max-width: 760px;
  margin: 0 auto;
}

.block__title {
  margin: 0 0 var(--space-3);
  padding-bottom: var(--space-2);
  border-bottom: 1px solid var(--color-border);
  font-size: 0.92rem;
  font-weight: 500;
}

.hint {
  margin: 0 0 var(--space-2);
  color: var(--color-text-muted);
  font-size: 0.8rem;
}

.hint--dim {
  color: var(--color-text-dim);
}

.kv {
  display: grid;
  grid-template-columns: 130px 1fr;
  gap: var(--space-2) var(--space-3);
  margin: var(--space-4) 0 0;
  font-size: 0.83rem;
}

.kv dt {
  color: var(--color-text-muted);
}

.kv dd {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}

.kv code {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  word-break: break-all;
}

.kv__btns {
  display: flex;
  gap: var(--space-2);
}

.sources {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.source {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-bg-elevated);
  font-size: 0.82rem;
}

.source + .source {
  border-top: 1px solid var(--color-border);
}

.source__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-success);
}

.source__dot--off {
  background: var(--color-text-dim);
}

.source__name {
  flex: 1 1 auto;
}

.source__type,
.source__state {
  color: var(--color-text-dim);
  font-size: 0.75rem;
}

.msite {
  margin-top: var(--space-4);
}

.msite__title {
  margin: 0 0 var(--space-2);
  font-size: 0.85rem;
  font-weight: 500;
}

.msite__stats {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-2);
  color: var(--color-text-muted);
  font-size: 0.78rem;
}

.msite__warn {
  color: var(--color-warning);
}

.msite__form {
  display: flex;
  gap: var(--space-2);
}

.input {
  flex: 1 1 auto;
  min-width: 0;
  padding: 6px var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg);
  color: var(--color-text);
  font-family: inherit;
  font-size: 0.82rem;
}

.input:focus {
  border-color: var(--color-accent);
  outline: none;
}

.theme {
  display: flex;
  gap: var(--space-2);
}
</style>
