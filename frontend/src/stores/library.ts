/**
 * 已入库清单
 *
 * 内容是 GetHistory 与 ScanDeployed 两路结果的合并视图。搜索结果卡片的
 * 「已入库」标记、已安装页的网格、游戏页的状态分支都读取此处。
 *
 * 为何必须合并：历史记录是本工具的账本，部署目录才是事实。二者可能不一致
 * ——用户手动放入的 lua、被清空的历史、他工具产生的文件都会造成偏差。
 * 只读账本会漏报，只扫目录则拿不到游戏名与勾选集合。
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getHistory,
  scanDeployed,
  removeDLCs as apiRemoveDLCs,
  type GameRecord,
  type DeployedEntry,
} from '../api'

/**
 * 合并后的单条入库记录。
 *
 * 四种来源组合的呈现差异：
 *   - 有 record、非外部：常态，可勾选 DLC
 *   - 有 record、有外部同名声明：conflicted 为真，卸载不彻底，需警示
 *   - 无 record、非外部：历史丢失，文件仍在，可删但无法还原勾选项
 *   - 无 record、外部：典型外部清单，只读且只提供删除
 */
export interface LibraryItem {
  mainAppID: string
  /** 外部清单拿不到游戏名，退回展示 AppID */
  gameName: string
  /** 部署文件名，可能有多个（本工具的 + 外部的） */
  fileNames: string[]
  /** 历史记录，无账本时为 null */
  record: GameRecord | null
  /** 该 AppID 存在非本工具命名格式的声明文件 */
  hasExternal: boolean
  /** 同一游戏被两处声明：卸载将不彻底 */
  conflicted: boolean
}

export const useLibraryStore = defineStore('library', () => {
  const records = ref<GameRecord[]>([])
  const deployed = ref<DeployedEntry[]>([])
  const loading = ref(false)

  /** 已入库的主游戏 AppID 集合，供搜索结果卡片做 O(1) 标记判定。 */
  const installedIDs = computed(
    () => new Set(items.value.map((i) => i.mainAppID)),
  )

  const items = computed<LibraryItem[]>(() => {
    const byID = new Map<string, LibraryItem>()

    for (const r of records.value) {
      byID.set(r.mainAppID, {
        mainAppID: r.mainAppID,
        gameName: r.gameName || r.mainAppID,
        fileNames: r.luaFileName ? [r.luaFileName] : [],
        record: r,
        hasExternal: false,
        conflicted: false,
      })
    }

    for (const d of deployed.value) {
      // 部署目录里的文件可能没有可识别的主 AppID（如自定义命名的外部脚本），
      // 此时以文件名作为标识，保证条目不会互相吞并。
      const key = d.mainAppID || d.fileName
      const item = byID.get(key)

      if (!item) {
        byID.set(key, {
          mainAppID: d.mainAppID || key,
          gameName: d.mainAppID || d.fileName,
          fileNames: [d.fileName],
          record: null,
          hasExternal: d.isExternal,
          conflicted: false,
        })
        continue
      }

      if (!item.fileNames.includes(d.fileName)) item.fileNames.push(d.fileName)
      if (d.isExternal) {
        item.hasExternal = true
        // 账本里有这个游戏，目录里又存在一份外部声明——卸载时本工具只能
        // 删掉自己的文件，游戏仍会因外部声明留在库中。
        if (item.record) item.conflicted = true
      }
    }

    return [...byID.values()].sort((a, b) => {
      const ta = a.record?.installedAt ?? ''
      const tb = b.record?.installedAt ?? ''
      return tb.localeCompare(ta)
    })
  })

  /** 存在需用户知情的对账异常，供界面决定是否提示。 */
  const hasAnomaly = computed(() =>
    items.value.some((i) => i.conflicted || (i.hasExternal && !i.record)),
  )

  async function refresh() {
    loading.value = true
    try {
      const [h, d] = await Promise.all([getHistory(), scanDeployed()])
      records.value = h
      deployed.value = d
    } finally {
      loading.value = false
    }
  }

  function find(mainAppID: string): LibraryItem | undefined {
    return items.value.find((i) => i.mainAppID === mainAppID)
  }

  /**
   * 彻底卸载。
   *
   * NOTE: 存在外部声明时后端返回失败，此处不吞掉——异常会带着「需手动
   * 处理的文件名」抛给调用方。但无论成败都要刷新，因为本工具自己的文件
   * 已经删掉了，界面必须反映这一点。
   */
  async function remove(mainAppID: string) {
    try {
      return await apiRemoveDLCs(mainAppID)
    } finally {
      await refresh()
    }
  }

  return {
    records,
    deployed,
    loading,
    items,
    installedIDs,
    hasAnomaly,
    refresh,
    find,
    remove,
  }
})
