/**
 * 配置镜像
 *
 * 主题影响全局样式，清单源列表在设置页与下载链路两处读取，故置于全局。
 *
 * 主题的落地方式是改写 <html data-theme>，由 CSS 级联完成重算，
 * 不触发 Vue 重渲染——因此界面上有多少组件都不影响切换开销。
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getConfig, saveConfig, type AppConfig } from '../api'

export type Theme = 'dark' | 'light' | 'system'

export const useConfigStore = defineStore('config', () => {
  const config = ref<AppConfig | null>(null)

  /** 已启用的清单源。ManifestHub 仓库已清空，默认为禁用状态。 */
  const enabledSources = computed(
    () => config.value?.repoSources?.filter((s) => s.enabled) ?? [],
  )

  const theme = computed<Theme>(() => (config.value?.theme as Theme) ?? 'dark')

  async function refresh() {
    config.value = await getConfig()
    applyTheme(theme.value)
  }

  /** 保存整份配置。成功后重新拉取，避免前端持有与后端不一致的副本。 */
  async function save(patch: Partial<AppConfig>) {
    if (!config.value) return
    const next = { ...config.value, ...patch } as AppConfig
    await saveConfig(next)
    await refresh()
  }

  async function setTheme(t: Theme) {
    applyTheme(t)
    await save({ theme: t } as Partial<AppConfig>)
  }

  return { config, enabledSources, theme, refresh, save, setTheme }
})

/**
 * 把主题写到 <html data-theme> 上。
 *
 * system 档不写死值，而是查询系统偏好后落为具体档位——CSS 侧只需处理
 * dark / light 两种情形，不必再写一套 prefers-color-scheme 规则。
 */
function applyTheme(t: Theme) {
  const resolved =
    t === 'system'
      ? window.matchMedia('(prefers-color-scheme: light)').matches
        ? 'light'
        : 'dark'
      : t
  document.documentElement.setAttribute('data-theme', resolved)
}
