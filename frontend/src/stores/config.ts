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
import { applyTheme, type ThemeName } from '../styles/theme-boot'

/**
 * 主题档位。
 *
 * 实现在 styles/theme-boot.ts——它必须能在 Vue 挂载前独立运行，
 * 故不能住在 store 里。此处只做类型再导出，调用方无需知道这层分工。
 */
export type Theme = ThemeName

export const useConfigStore = defineStore('config', () => {
  const config = ref<AppConfig | null>(null)

  /** 已启用的清单源。ManifestHub 仓库已清空，默认为禁用状态。 */
  const enabledSources = computed(
    () => config.value?.repoSources?.filter((s) => s.enabled) ?? [],
  )

  /**
   * 当前主题。默认浅色，须与后端 defaultTheme 保持一致。
   *
   * 两处默认值必须同改：后端管首次运行落盘的值，此处管配置尚未拉到时的
   * 临时值。不一致会让首屏先按一种主题渲染、拉到配置后再跳一次。
   */
  const theme = computed<Theme>(() => (config.value?.theme as Theme) ?? 'light')

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


