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
 * ⚠️ 一个曾经存在的盲区（2026-08-01 第 4 步收尾时发现并修掉）：
 *
 *   本脚本原先按**目录**收集定义（walk(TOKEN_DIR)），而不看这些文件
 *   是否真的被 styles/index.css 引入。于是出现过这样一个状态——
 *   legacy.css 已从 index.css 摘除、在浏览器里完全失效，但文件还在磁盘上，
 *   于是它定义的 13 个旧令牌仍被算作「已定义」，任何遗漏的引用都被放过。
 *
 *   这正是本脚本存在的意义所反过来咬了自己一口：宪法 12.4 节称它是
 *   「唯一能确认迁移干净的手段」，而它在「文件还在、只是不再引入」这个
 *   中间状态下恰好是瞎的——而那个状态正是删除 legacy.css 的前一步。
 *
 *   修法：只收集**被 index.css 实际 @import 的**令牌文件。
 *   由此得到一条一般判断：**判据本身也要有判据。**
 *   凡是「扫目录得出结论」的检查，都要问一句「目录里的东西是否等于
 *   实际生效的东西」——在有构建步骤的项目里，这两者默认不相等。
 *
 * 局限（刻意不处理，避免脚本自身变成需要维护的东西）：
 *   - 不识别组件内部自定义的局部变量。故约定：组件内的局部变量
 *     必须带组件前缀（如 --sel-xxx）并在同文件内定义，脚本会因
 *     「同文件内有定义」而放过它。
 *   - 不做值合法性校验，只查存在性。
 *   - 只解析 index.css 里形如 @import './tokens/x.css' 的直接引入，
 *     不做递归。令牌层刻意保持扁平，故够用。
 *
 * 用法：node scripts/check-tokens.mjs
 * 退出码非 0 表示发现未定义引用。
 */

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'

const SRC = 'src'
const STYLES = join(SRC, 'styles')
const TOKEN_DIR = join(STYLES, 'tokens')
const STYLE_ENTRY = join(STYLES, 'index.css')

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

/**
 * 找出被 index.css 实际引入的令牌文件。
 *
 * 不能直接 walk(TOKEN_DIR)：磁盘上存在但未被引入的令牌文件，其定义在
 * 浏览器里是不存在的，算进来会让检查放过真正的遗漏引用（见文件头说明）。
 */
function importedTokenFiles() {
  const entry = readFileSync(STYLE_ENTRY, 'utf8')

  // 去掉块注释，避免把注释里提到的路径当成真的 @import
  // （本项目的 index.css 里确实有一段注释在解释 legacy.css 为何被删）
  const code = entry.replace(/\/\*[\s\S]*?\*\//g, '')

  const files = []
  for (const m of code.matchAll(/@import\s+['"]\.\/tokens\/([a-z0-9-]+\.css)['"]/gi)) {
    files.push(join(TOKEN_DIR, m[1]))
  }
  return files
}

/** 收集全局令牌定义 */
const globalTokens = new Set()
const tokenFiles = importedTokenFiles()

for (const file of tokenFiles) {
  const text = readFileSync(file, 'utf8')
  for (const m of text.matchAll(/(--[a-z0-9-]+)\s*:/gi)) {
    globalTokens.add(m[1])
  }
}

if (globalTokens.size === 0) {
  console.error(
    `未从 ${STYLE_ENTRY} 解析到任何被引入的令牌文件，` +
      '检查 @import 写法是否仍为 \'./tokens/x.css\' 形式',
  )
  process.exit(2)
}

/**
 * 磁盘上存在却未被引入的令牌文件，一律报错而不是静默忽略。
 *
 * 这种文件是最危险的形态：它看起来「还在项目里」，但对浏览器不存在。
 * 要么把它引入，要么把它删掉，不允许以这个状态留存。
 */
const orphanTokenFiles = walk(TOKEN_DIR).filter((f) => !tokenFiles.includes(f))
if (orphanTokenFiles.length) {
  console.error('发现未被 index.css 引入的令牌文件：\n')
  for (const f of orphanTokenFiles) console.error('  ' + f)
  console.error(
    '\n这类文件对浏览器不存在，但会让本检查误判「令牌已定义」。' +
      '请将其引入 index.css，或直接删除。',
  )
  process.exit(1)
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
