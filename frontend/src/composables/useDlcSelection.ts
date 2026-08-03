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
 *
 * ⚠️ 由此产生一条必须守住的不变式：**待落盘的改动必须记下它属于哪个清单
 * 包**（`pendingPkg`）。状态可以随界面切换归零，但已经答应用户的落盘不能
 * ——防抖窗口结束时用户可能早已切走，此时读 `pkgRef` 拿到的是另一个游戏。
 * 三处出口（防抖到点、换包、组件卸载）各自都要能把改动交出去。
 */

import { ref, computed, watch, onUnmounted, type Ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { installDLCs, type GamePackage, type DLCInfo } from '../api'
import { useToast } from './useToast'
import { useConfirm } from './useConfirm'
import { useLibraryStore } from '../stores/library'

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
  const library = useLibraryStore()

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

  /**
   * 待落盘改动所属的清单包。
   *
   * 记「属于哪个包」而非只记一个 dirty 布尔：防抖窗口内可能已经切走游戏，
   * 届时必须分辨这笔改动是给谁的。用对象同一性比对，比存 appID 更直接
   * ——落盘调用要的正是这个包本身。
   *
   * 非响应式：只在逻辑内部做归属判定，界面读的是 syncState。
   *
   * 声明必须在下方 watch 之前：虽然闭包引用不会立刻求值，靠的却是
   * 「这两行之间 pkgRef 不会变」这个巧合，不是语言保证。
   */
  let pendingPkg: GamePackage | null = null

  // 清单包换成另一个游戏时重置勾选，否则会把上一个游戏的 AppID 带过去。
  //
  // XXX: 必须用 flush: 'sync'。默认的 pre 时机是异步的，调用方在赋值
  // pkg 后紧接着调 selectAll() 时，本回调会晚于 selectAll 执行并把刚
  // 选好的集合清空，导致防抖窗口结束后落盘一个空列表。
  watch(
    pkgRef,
    () => {
      // 换包前先把上一个包的待落盘改动交出去，否则它必然丢失：防抖窗口
      // 到点时 pkgRef 已经是新包（或 null），旧包的勾选也已被下面清空。
      //
      // 这条路径比 onUnmounted 那条更要紧。库页是 master-detail，
      // PaneTransition 刻意复用实例（宪法 5.2），侧栏换游戏根本不触发
      // 卸载——即勾一下就切走的最常见操作恰好走的是这里。
      if (pendingPkg) void syncDetached(pendingPkg, [...selected.value])

      selected.value = new Set()
      syncState.value = 'idle'
    },
    { flush: 'sync' },
  )

  /**
   * 落盘并把过程反馈到 syncState。仅用于「用户正在看着的那个包」。
   */
  async function syncActive(pkg: GamePackage, ids: string[]) {
    pendingPkg = null
    syncState.value = 'syncing'
    try {
      await installDLCs(pkg, ids)
      await refreshLibrary()
      syncState.value = 'done'
      // 2 秒后回到静默态，让「已同步」提示自然淡出而不长期占位
      window.setTimeout(() => {
        if (syncState.value === 'done') syncState.value = 'idle'
      }, 2000)
    } catch (e) {
      syncState.value = 'idle'
      toast.fromError(e, '同步失败')
    }
  }

  /**
   * 补交已经切走的包的改动，不碰 syncState。
   *
   * 为何不反馈状态：此刻 syncState 已经归属界面上的另一个游戏，往里写
   * 「正在同步」会把 A 的进度显示在 B 的页面上——那是拿一个状态承载两个
   * 游戏的处境。失败仍要 toast，因为 toast 是全局的，且落盘失败必须让
   * 用户知道，否则界面显示的勾选与实际部署不一致。
   */
  async function syncDetached(pkg: GamePackage, ids: string[]) {
    pendingPkg = null
    try {
      await installDLCs(pkg, ids)
      await refreshLibrary()
    } catch (e) {
      toast.fromError(e, '同步失败')
    }
  }

  /**
   * 落盘成功后刷新库缓存。
   *
   * 部署改变了 DLC 数与安装时间，而侧栏条目的「N 个 DLC」、库概览的统计、
   * 状态 C 的计数全都读 library store。不刷新则它们停在上次刷新时的数值，
   * 且没有任何报错——只是数字不对。
   *
   * 放在这里而非组件里：此处是唯一知道「落盘刚刚成功」的地方。组件侧只看到
   * syncState 变化，而 syncDetached 压根不动 syncState。
   *
   * 失败只记不抛：缓存陈旧比落盘失败轻得多，不该盖掉后者的错误提示。
   */
  async function refreshLibrary() {
    try {
      await library.refresh()
    } catch {
      // 界面数字可能暂时偏旧，下次进入库页会自行修正
    }
  }

  const flush = useDebounceFn(async () => {
    const pkg = pkgRef.value
    // 归属校验：窗口内切过游戏的话，这笔改动已由下方 watch 补交完毕，
    // 此处再跑一次会拿当前包配上当前勾选重复部署一遍。
    if (!pkg || pkg !== pendingPkg) return

    await syncActive(pkg, [...selected.value])
  }, DEBOUNCE_MS)

  function markDirty() {
    pendingPkg = pkgRef.value
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
    // 与 syncState 一并归零：还原是「读回已有状态」，不产生待落盘改动。
    // 漏掉这行会让上一轮的归属标记搭便车，把刚还原的集合当成用户的新改动。
    pendingPkg = null
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

  /*
    组件卸载时若仍有待落盘的改动，立即发出。否则用户在 800ms 窗口内切走
    页面，最后一次勾选会静默丢失，界面与实际部署产生偏差。

    XXX: 原实现是 `void flush()`，而 flush 是防抖包装后的函数——再调一次
    等于「清掉旧计时器、重设一个新的」，即又等 800ms 而非立即。它没丢数据
    纯属侥幸：setTimeout 不随组件销毁取消，闭包里的包引用还活着，所以那笔
    改动最终仍会发出，只是比预期晚。既然要的是「立即」，就直接调落盘本体。

    用 syncDetached：组件正在销毁，写 syncState 没有任何人会读到。
  */
  onUnmounted(() => {
    if (pendingPkg) void syncDetached(pendingPkg, [...selected.value])
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
