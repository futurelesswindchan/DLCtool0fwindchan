/**
 * 设计令牌引用自查
 *
 * 存在理由（2026-08-01 实测踩到）：
 *   UiSegmented 引用了从未定义的 --seg-knob，指示器背景因此是透明的，
 *   在两个主题下都不可见。而 `vue-tsc --noEmit` 与 `vite build` 全部通过——
 *   **未定义的 CSS 变量不是错误，它渲染为无效值然后被忽略，不报错也不警告。**
 *
 *   这类失效只能靠肉眼在实机上发现，而且极易被当成「配色没调好」，
 *   排查方向会指向取值而不是「这个变量根本不存在」。
 *
 * 本脚本做两件事：
 *   1. 收集 styles/tokens/ 下所有 --xxx 定义
 *   2. 扫描全部 .css 与 .vue 里的 var(--xxx) 引用，报出未定义者
 *
 * 局限（刻意不处理，避免脚本自身变成需要维护的东西）：
 *   - 不识别组件内部自定义的局部变量。故约定：组件内的局部变量
 *     必须带组件前缀（如 --sel-xxx）并在同文件内定义，脚本会因
 *     「同文件内有定义」而放过它。
 *   - 不做值合法性校验，只查存在性。
 *
 * 用法：node scripts/check-tokens.mjs
 * 退出码非 0 表示发现未定义引用。
 */

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'

const SRC = 'src'
const TOKEN_DIR = join(SRC, 'styles', 'tokens')

/** Wails 与浏览器提供的变量，不由本项目定义 */
const EXTERNAL = new Set(['--wails-draggable'])

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, out)
    else out.push(p)
  }
  return out
}

/** 收集全局令牌定义 */
const globalTokens = new Set()
for (const file of walk(TOKEN_DIR)) {
  const text = readFileSync(file, 'utf8')
  for (const m of text.matchAll(/(--[a-z0-9-]+)\s*:/gi)) {
    globalTokens.add(m[1])
  }
}

if (globalTokens.size === 0) {
  console.error('未收集到任何令牌定义，检查 TOKEN_DIR 路径是否正确')
  process.exit(2)
}

const problems = []

for (const file of walk(SRC)) {
  const ext = extname(file)
  if (ext !== '.css' && ext !== '.vue') continue
  // 令牌文件本身只有定义，跳过
  if (file.startsWith(TOKEN_DIR)) continue

  const text = readFileSync(file, 'utf8')

  // 同文件内定义的局部变量视为合法
  const localTokens = new Set(
    [...text.matchAll(/(--[a-z0-9-]+)\s*:/gi)].map((m) => m[1]),
  )

  // 捕获组 2 为逗号，表示写了兜底值
  for (const m of text.matchAll(/var\(\s*(--[a-z0-9-]+)\s*(,)?/gi)) {
    const name = m[1]
    const hasFallback = Boolean(m[2])

    if (globalTokens.has(name)) continue
    if (localTokens.has(name)) continue
    if (EXTERNAL.has(name)) continue

    // 写了兜底值的引用本就是安全的，且这正是「由 JS 注入的变量」
    // 的正确写法（如 useStagger 注入的 --stagger-i）。
    // 反过来说：由 JS 注入却不写兜底，才是应该被拦住的。
    if (hasFallback) continue

    const line = text.slice(0, m.index).split('\n').length
    problems.push(`${file}:${line}  ${name}`)
  }
}

if (problems.length) {
  console.error('发现未定义的令牌引用：\n')
  for (const p of problems) console.error('  ' + p)
  console.error(
    '\n未定义的 CSS 变量不会报错，只会渲染为无效值——' +
      '这类问题在构建阶段完全静默，必须靠本检查拦住。',
  )
  process.exit(1)
}

console.log(`令牌引用检查通过（已定义 ${globalTokens.size} 个）`)
