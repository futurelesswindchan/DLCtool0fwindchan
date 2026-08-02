<script setup lang="ts">
/**
 * 设置 · 清单源
 *
 * 本 Pane 是「源生态维护是核心长期任务」这条认识在界面上的落点
 * （`DECISIONS-2` 07-31）：源的质量与数量决定了整个工具的上限，
 * 故它值得一整屏，而不是挤在设置页中段。
 *
 * ⚠️ 不渲染 `RepoSource.token`。该字段随配置一同跨界到前端，但凭据不该
 *    出现在界面上——是否已配置由额度信息间接表达即可。
 *
 * TODO(自定义源): 宪法第 9 章已为「用户自定义源入口」定性——它不是增加
 *   复杂度，而是把「等下一个版本」这个无解等待变成可解动作，减少的是
 *   无力感。落点就在本 Pane。当前源列表硬编码在 Go 侧 defaultRepoSources()，
 *   新增源必须改代码发版。
 */

import { ref, onMounted } from 'vue'
import { useConfigStore } from '../../stores/config'
import { useToast } from '../../composables/useToast'
import { useConfirm } from '../../composables/useConfirm'
import { setRepoToken, getMSiteStats, type MSiteStats } from '../../api'
import { UiButton, UiInput } from '../../components/ui'

const config = useConfigStore()
const toast = useToast()
const confirm = useConfirm()

/** 认证型源的名称。与后端内置源配置保持一致。 */
const M_SITE = 'Hubcap Manifest'

const msite = ref<MSiteStats | null>(null)
const tokenInput = ref('')
const savingToken = ref(false)

onMounted(refreshMSite)

async function refreshMSite() {
  try {
    msite.value = await getMSiteStats()
  } catch {
    // 未配置凭据或查询失败都表现为无额度信息，不必打扰用户
    msite.value = null
  }
}

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

/**
 * 把 RepoSource.Kind 译成用户能理解的说法。
 *
 * 后端的 kind 是访问形态的技术标识（决定走哪套下载逻辑），直接显示
 * 「githubBranch」对用户没有意义，只需让他知道「这个源要不要自己配凭据」。
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
  <section class="pane">
    <h2 class="set-block__title">清单源</h2>

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
      <p class="set-hint">
        该源数据最完整，但需自备 API key。凭据默认仅 7 天有效，到期后此源会被静默跳过。
      </p>

      <div v-if="msite" class="msite__stats">
        <span>账户 {{ msite.username }}</span>
        <span class="u-tnum">今日 {{ msite.dailyUsage }}/{{ msite.dailyLimit }}</span>
        <span v-if="msite.expiresAt" class="u-tnum">
          到期 {{ msite.expiresAt.slice(0, 10) }}
        </span>
        <span v-if="msite.expiringSoon" class="set-hint--warn">即将到期</span>
        <span v-else-if="!msite.canMakeRequests" class="set-hint--warn">
          额度已用完
        </span>
      </div>
      <p v-else class="set-hint set-hint--dim">尚未配置凭据。</p>

      <div class="msite__form">
        <UiInput
          v-model="tokenInput"
          type="password"
          placeholder="粘贴 API key"
          mono
        />
        <UiButton
          variant="primary"
          :disabled="!tokenInput.trim()"
          :loading="savingToken"
          @click="onSaveToken"
        >
          保存
        </UiButton>
        <UiButton v-if="msite" variant="danger" @click="onClearToken">
          清除
        </UiButton>
      </div>
    </div>
  </section>
</template>

<style scoped>
.pane {
  max-width: 760px;
}

.sources {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-card);
  /* 子项的方角要被外层圆角裁掉，否则四角会露出直角 */
  overflow: hidden;
}

.source {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  font-size: var(--text-sm);
}

.source + .source {
  border-top: 1px solid var(--color-border);
}

/*
  启用态小圆点，用状态色-可用。

  ⚠️ 宪法 12.4 节原先断言这个旧语义色「扫描无调用点」，而这一处正是反例
     （全量实测 9 处、分布 6 个文件）。若照原账本在删 legacy.css 时直接
     移除它，这个点会变成无效值、渲染为透明，而三道检查全部通过。
     勘误已回填文档 12.4 节。

     （此注释刻意不写出那个旧令牌名，否则会让归零判据出现假阳性。）
*/
.source__dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--state-ok);
}

.source__dot--off {
  background: var(--color-text-dim);
}

.source__name {
  flex: 1 1 auto;
  min-width: 0;
}

.source__type,
.source__state {
  color: var(--color-text-dim);
  font-size: var(--text-xs);
}

.msite {
  margin-top: var(--space-4);
}

.msite__title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-base);
  font-weight: var(--weight-medium);
}

.msite__stats {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-2);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.msite__form {
  display: flex;
  gap: var(--space-2);
}
</style>
