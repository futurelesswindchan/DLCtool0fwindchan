/**
 * 在线搜索的会话状态与请求生命周期
 *
 * 为何从组件搬到这里（推翻 SearchPane 原注释「属会话级临时数据」）：
 * 那句话只考虑了「有没有别的页面要读」，漏了「组件销毁时正在飞的请求
 * 怎么办」。实机表现是切页再回来，关键词、结果、失败提示全没了，而且
 * `await searchGames(q)` 的 promise 落在已销毁的实例上，结果被静默丢弃
 * ——用户以为搜索没生效，实际是搜到了没人接。
 *
 * 请求活在 store 里，组件只负责触发与读取，切页便不再打断工作。
 *
 * NOTE: 不做「关键词 -> 结果」的多组缓存。搜索是一次性动作，用户回到
 * 本页要的是「上次那一屏」，不是历史记录。多组缓存要额外回答何时失效，
 * 收益却只有重复搜同一词时省一次请求。
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { searchGames, ApiError, type GameSearchResult } from '../api'

/**
 * 搜索的四种状态。
 *
 * 刻意用单一枚举而非几个并列的布尔量：`searching` / `searched` 那种组合
 * 里，「失败」没有自己的位置——原实现失败时 `searched` 仍为 false，页面
 * 于是退回初始空态，用户刚点过搜索却看到「还没搜过」的样子。
 *
 * 枚举则强制每种情形都得有个名字，空态与错误态各自成立，不必靠
 * 「结果为空」反推是哪一种。
 */
export type SearchStatus = 'idle' | 'searching' | 'done' | 'failed'

export const useSearchStore = defineStore('search', () => {
  /** 输入框的当前内容。存在此处才能让切页回来时输入框保持原样。 */
  const term = ref('')
  /** 上一次成功搜索所用的关键词，供结果区标注「以下是 xxx 的结果」。 */
  const executedTerm = ref('')
  const results = ref<GameSearchResult[]>([])
  const status = ref<SearchStatus>('idle')
  /** 失败原因的面向用户文案。非 failed 状态时为空串。 */
  const errorMessage = ref('')

  /**
   * 请求序号。
   *
   * 每次发起自增，resolve 时比对自己是否仍是最新一发，过期响应直接丢弃。
   * 虽然界面同时只允许一次搜索（按钮会禁用），但「禁用」是表现层的约定，
   * 而正确性不该依赖表现层——回车键、以后可能加的搜索历史点击、乃至
   * 热重载都可能并发触发。序号让每个响应带上身份，`status` 也由它派生，
   * 而不是自己当一个可能与实际请求脱节的裸布尔。
   */
  let seq = 0

  const searching = computed(() => status.value === 'searching')

  /** 关键词非空且不在搜索中，才允许发起。 */
  const canSearch = computed(() => !!term.value.trim() && !searching.value)

  /**
   * 已确认无结果。
   *
   * 显式排除 searching 与 failed：这两种情形下 `results` 同样是空的，
   * 若只看长度就会在尚未查明（或压根没查成）时对用户断言「没有这个游戏」。
   */
  const isEmptyResult = computed(
    () => status.value === 'done' && results.value.length === 0,
  )

  /**
   * 发起搜索。
   *
   * 不抛异常：失败写进 `errorMessage` 由界面呈现。调用方（组件）可能在
   * 响应回来前就已销毁，抛出去没人接得住。
   *
   * @param keyword 省略时取输入框当前内容
   */
  async function run(keyword?: string) {
    const q = (keyword ?? term.value).trim()
    if (!q || searching.value) return

    term.value = q
    const mySeq = ++seq
    status.value = 'searching'
    errorMessage.value = ''

    try {
      const list = await searchGames(q)
      if (mySeq !== seq) return
      results.value = list
      executedTerm.value = q
      status.value = 'done'
    } catch (e) {
      if (mySeq !== seq) return
      // 失败不清空上一轮结果：用户可能只是想再搜一次，把已有内容擦掉
      // 等于惩罚了一次网络波动。错误提示与旧结果同屏并不矛盾——
      // 前者说的是「这次没成」，后者标着自己属于哪个关键词。
      errorMessage.value = describeError(e)
      status.value = 'failed'
    }
  }

  /** 清空关键词与结果，回到初始态。 */
  function clear() {
    // 自增使在途响应失效，否则清空后旧结果仍会回填。
    seq++
    term.value = ''
    executedTerm.value = ''
    results.value = []
    errorMessage.value = ''
    status.value = 'idle'
  }

  return {
    term,
    executedTerm,
    results,
    status,
    errorMessage,
    searching,
    canSearch,
    isEmptyResult,
    run,
    clear,
  }
})

/**
 * 把异常转成面向用户的一句话。
 *
 * ApiError 带的是后端已经写好的用户文案，直接用；其余（绑定层、网络栈）
 * 的原文对用户无意义，给一句兜底。
 *
 * TODO(路线第 10 项): 按错误类型分支细化，复用 tools/netprobe 的 classify
 * 七分支，并说清「这不是盒子坏了」。此处只保证失败有话可说。
 */
function describeError(e: unknown): string {
  if (e instanceof ApiError) return e.message
  return '搜索没能完成，可能是网络波动。稍等一两分钟再试通常就好。'
}
