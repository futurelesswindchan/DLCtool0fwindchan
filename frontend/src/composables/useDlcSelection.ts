/**
 * 单个游戏的 DLC 勾选与落盘
 *
 * 交互模型是「勾选即部署」，没有独立的安装按钮。为此需解决两个问题：
 *
 *   1. 界面必须即时响应，不能等后端往返。故勾选先只改内存。
 *   2. 连续勾选不应触发多次部署。故用 800ms 防抖——该值大于 OST
 *      FileWatcher 的 500ms 防抖窗口，确保用户的一串操作聚合完成后
 *      只惊动注入器一次。
 *
 * 不放进全局 store：勾选状态归属「当前所看的这一个游戏」，全局化反而
 *要额外处理切换游戏时的清理。
 */

import { ref, computed, watch, onUnmounted, type Ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { installDLCs, type GamePackage, type DLCInfo } from '../api'
import { useToast } from './useToast'
import { useConfirm } from './useConfirm'

/** 同步状态。done 态由界面在 2 秒后自行淡出。 */
export type SyncState = 'idle' | 'pending' | 'syncing' | 'done'

const DEBOUNCE_MS = 800

/**
 * 批量确认弹窗中最多列举的 DLC 名称数。
 *
 * 部分游戏的 DLC 多达数十个（实测猎人：荒野的召唤有 70 个），全部列出会
 * 让弹窗高度超出窗口，确认按钮被挤到视口外反而更危险。
 */
const MAX_LISTED_DLCS = 8

/**
 * 参数取响应式引用而非裸对象。
 *
 * 原因：本函数内部注册了 onUnmounted，必须在组件 setup 的同步作用域内
 * 调用。而清单包往往是异步到手的（下载 / 解析之后才有值），若要求传入
 * 裸对象，调用方就只能把本函数塞进 await 之后或 watch 回调里，此时
 * onUnmounted 注册不到当前组件实例上，待落盘的改动会静默丢失。
 */
export function useDlcSelection(pkgRef: Ref<GamePackage | null>) {
  const toast = useToast()
  const confirm = useConfirm()

  const selected = ref(new Set<string>())
  const syncState = ref<SyncState>('idle')

  /** 本次会话内是否已跳过独立 Depot 的取消勾选警告 */
  const skipDepotWarn = ref(false)

  const total = computed(() => pkgRef.value?.dlcs.length ?? 0)
  const selectedCount = computed(() => selected.value.size)
  const allSelected = computed(
    () => total.value > 0 && selectedCount.value === total.value,
  )

  const isSelected = (appID: string) => selected.value.has(appID)

  // 清单包换成另一个游戏时重置勾选，否则会把上一个游戏的 AppID 带过去。
  //
  // XXX: 必须用 flush: 'sync'。默认的 pre 时机是异步的，调用方在赋值
  // pkg 后紧接着调 selectAll() 时，本回调会晚于 selectAll 执行并把刚
  // 选好的集合清空，导致防抖窗口结束后落盘一个空列表。
  watch(
    pkgRef,
    () => {
      selected.value = new Set()
      syncState.value = 'idle'
    },
    { flush: 'sync' },
  )

  const flush = useDebounceFn(async () => {
    const pkg = pkgRef.value
    if (!pkg) return

    syncState.value = 'syncing'
    try {
      await installDLCs(pkg, [...selected.value])
      syncState.value = 'done'
      // 2 秒后回到静默态，让「已同步」提示自然淡出而不长期占位
      window.setTimeout(() => {
        if (syncState.value === 'done') syncState.value = 'idle'
      }, 2000)
    } catch (e) {
      syncState.value = 'idle'
      toast.fromError(e, '同步失败')
    }
  }, DEBOUNCE_MS)

  function markDirty() {
    syncState.value = 'pending'
    void flush()
  }

  /**
   * 切换单个 DLC。
   *
   * 取消勾选带独立 Depot 的 DLC 时需二次确认：Steam 可能因此删除已下载的
   * 本地内容，对大体积 DLC 而言重新下载代价不小。纯许可证型 DLC（无
   * manifestID）不占空间，无需确认。
   */
  async function toggle(dlc: DLCInfo) {
    const removing = isSelected(dlc.appID)

    if (removing && dlc.manifestID && !skipDepotWarn.value) {
      const ok = await confirm({
        title: `取消「${dlc.name || dlc.appID}」？`,
        body: [
          '该 DLC 含独立的内容分支，取消后 Steam 可能删除已下载到本地的文件。',
          '重新勾选可恢复许可证，但内容需要重新下载。',
        ],
        confirmText: '取消勾选',
        danger: true,
      })
      if (!ok) return
    }

    if (removing) selected.value.delete(dlc.appID)
    else selected.value.add(dlc.appID)

    markDirty()
  }

  /**
   * 还原已有的勾选状态，不触发落盘。
   *
   * 用于打开已入库的游戏页时恢复上次的选择。**必须**与 selectAll 等
   * 操作区分开来：若走 markDirty，仅仅打开一次页面就会白部署一次，
   * 既无意义地惊动注入器，也会把 installedAt 刷成当前时间，让「获取于
   * X 天前」永远显示今天。
   */
  function restore(appIDs: string[]) {
    selected.value = new Set(appIDs)
    syncState.value = 'idle'
  }

  function selectAll() {
    for (const d of pkgRef.value?.dlcs ?? []) selected.value.add(d.appID)
    markDirty()
  }

  /**
   * 全不选。
   *
   * 与「彻底卸载」是两件事：此处仍保留主游戏声明，清单文件不删除，
   * 游戏本体依旧在库中。
   *
   * XXX: 必须在此处也做独立 Depot 的确认。逐个 toggle 会拦，批量清空却
   * 直接放行的话，一次点击就能悄悄取消掉全部带 ⚑ 的 DLC——Steam 随即
   * 删除已下载的内容，对几十 GB 的 DLC 而言代价远超逐个取消。此处不逐个
   * 弹窗（数量可能是几十个），改为一次性列出受影响的条目。
   */
  async function selectNone() {
    const affected = (pkgRef.value?.dlcs ?? []).filter(
      (d) => d.manifestID && isSelected(d.appID),
    )

    if (affected.length > 0 && !skipDepotWarn.value) {
      const names = affected
        .slice(0, MAX_LISTED_DLCS)
        .map((d) => d.name || d.appID)
      const more = affected.length - names.length

      const ok = await confirm({
        title: `取消全部 ${selectedCount.value} 个 DLC？`,
        body: [
          `其中 ${affected.length} 个含独立的内容分支，Steam 可能删除已下载到本地的文件：`,
          names.join('、') + (more > 0 ? ` 等 ${affected.length} 个` : ''),
          '重新勾选可恢复许可证，但内容需要重新下载。',
        ],
        confirmText: '全部取消',
        danger: true,
      })
      if (!ok) return
    }

    selected.value.clear()
    markDirty()
  }

  // 组件卸载时若仍有待落盘的改动，立即冲刷。否则用户在 800ms 窗口内
  // 切走页面，最后一次勾选会静默丢失，界面与实际部署产生偏差。
  onUnmounted(() => {
    if (syncState.value === 'pending') void flush()
  })

  return {
    selected,
    syncState,
    skipDepotWarn,
    total,
    selectedCount,
    allSelected,
    isSelected,
    restore,
    toggle,
    selectAll,
    selectNone,
  }
}
