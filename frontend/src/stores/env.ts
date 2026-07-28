/**
 * 环境状态
 *
 * 内容为后端检测结果的镜像。标题栏的环境指示、部署链路的前置校验、
 * 引导页的路由守卫都读取此处，故置于全局。
 *
 * 唯一真相在 Go：本 store 不推导任何状态，只在明确的时机（应用启动、
 * 用户改路径、用户手动重检）调用后端刷新。
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  detectEnvironment,
  getDeployDir,
  getConfig,
  setSteamPath as apiSetSteamPath,
  autoDetectSteamPath,
  type DetectorResult,
} from '../api'

export const useEnvStore = defineStore('env', () => {
  const result = ref<DetectorResult | null>(null)
  const steamPath = ref('')
  const deployDir = ref('')
  const loading = ref(false)

  /** 是否已完成过至少一次检测。路由守卫据此避免在状态未知时误重定向。 */
  const checked = ref(false)

  /** 注入器就绪，可以部署。 */
  const ready = computed(() => result.value?.available === true)

  /**
   * 标题栏指示灯的三态。
   *
   * 与后端的 status 字段不完全对应：Steam 路径为空时后端仍会返回
   * missing，但对用户而言「没设路径」和「设了路径但缺 DLL」是两件
   * 需要不同处置的事，故在此拆开。
   */
  const indicator = computed<'ready' | 'missing' | 'nopath'>(() => {
    if (!steamPath.value) return 'nopath'
    return ready.value ? 'ready' : 'missing'
  })

  /** 缺失的注入器文件名列表。 */
  const missingFiles = computed(() => result.value?.missingFiles ?? [])

  async function refresh() {
    loading.value = true
    try {
      const cfg = await getConfig()
      steamPath.value = cfg.steamPath ?? ''
      result.value = await detectEnvironment()
      deployDir.value = await getDeployDir()
    } finally {
      loading.value = false
      checked.value = true
    }
  }

  /** 手动指定 Steam 路径。失败时抛 ApiError，由调用方 Toast 呈现。 */
  async function setSteamPath(path: string) {
    const msg = await apiSetSteamPath(path)
    await refresh()
    return msg
  }

  /** 尝试从注册表自动识别。返回识别到的路径，未找到时后端抛错。 */
  async function autoDetect() {
    const path = await autoDetectSteamPath()
    await refresh()
    return path
  }

  return {
    result,
    steamPath,
    deployDir,
    loading,
    checked,
    ready,
    indicator,
    missingFiles,
    refresh,
    setSteamPath,
    autoDetect,
  }
})
