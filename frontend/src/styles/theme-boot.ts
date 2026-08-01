/**
 * 首屏主题落地
 *
 * 必须在 Vue 挂载之前同步执行，故独立成模块而非放进 store。
 *
 * 要解决的问题：主题存在后端配置里，而 `config.refresh()` 是异步的。
 * 于是首屏必然先以 CSS 里裸 `:root` 的那套令牌渲染，等配置拉回来才切换——
 * 凡是选了非默认主题的用户，每次启动都会看到一次闪色。
 *
 * 做法：把上次生效的主题同时写进 localStorage，启动时同步读出并立刻落到
 * `<html data-theme>`。后端配置仍是唯一权威——localStorage 只是一份
 * 「用于提前渲染的猜测」，配置拉回后若不一致会被覆盖，
 * 那种情况下才会闪一次（只发生在用户换过设备或清过缓存之后）。
 *
 * 为什么不把主题存进 localStorage 当权威：配置需要跟着 config.json 走，
 * 用户导出诊断包时主题也应在其中。两份权威必然会不一致。
 */

const KEY = 'kazeusa.theme'

export type ThemeName = 'dark' | 'light' | 'system'

/**
 * 把 system 解析为具体档位。
 *
 * CSS 侧因此只需处理 dark / light 两种情形，不必再写一套
 * prefers-color-scheme 规则——两套规则同时存在时，
 * 「跟随系统」与「显式指定」的优先级会变得很难推理。
 */
export function resolveTheme(t: ThemeName): 'dark' | 'light' {
  if (t !== 'system') return t
  return window.matchMedia('(prefers-color-scheme: light)').matches
    ? 'light'
    : 'dark'
}

export function applyTheme(t: ThemeName) {
  document.documentElement.setAttribute('data-theme', resolveTheme(t))
  try {
    localStorage.setItem(KEY, t)
  } catch {
    // 隐私模式或存储配额耗尽时静默失败。
    // 代价只是下次启动可能闪一次色，不值得打断启动流程。
  }
}

/**
 * 启动时调用。读不到就用默认值。
 *
 * ⚠️ 默认值须与 Go 侧 config.go 的 defaultTheme 一致，
 *    否则首次运行的用户会在配置拉回后看到一次跳变。
 */
export function bootTheme() {
  let saved: string | null = null
  try {
    saved = localStorage.getItem(KEY)
  } catch {
    // 同上，忽略
  }

  const t: ThemeName =
    saved === 'dark' || saved === 'light' || saved === 'system'
      ? saved
      : 'light'

  document.documentElement.setAttribute('data-theme', resolveTheme(t))
}

/**
 * 监听系统主题变化，仅在当前处于 system 档时生效。
 *
 * 返回解绑函数。不自行注册 onUnmounted——本监听的生命周期是整个应用，
 * 由调用方在 App 层管理更清晰。
 */
export function watchSystemTheme(isSystemMode: () => boolean) {
  const mq = window.matchMedia('(prefers-color-scheme: light)')

  const onChange = () => {
    if (!isSystemMode()) return
    document.documentElement.setAttribute(
      'data-theme',
      mq.matches ? 'light' : 'dark',
    )
  }

  mq.addEventListener('change', onChange)
  return () => mq.removeEventListener('change', onChange)
}
