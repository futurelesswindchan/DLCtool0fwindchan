# release.ps1
#
# 封测/正式版打包脚本。做三件事：
#   1. 从 git 取提交哈希与工作树状态
#   2. 经 ldflags 把版本、哈希、构建时刻注入程序自身
#   3. 产出带版本与哈希的文件名，便于分发时肉眼识别
#
# 为何哈希要打两处（文件名 + 程序内部）：
#   文件名便于在群里一眼看出「你用的是哪个包」，但它会在用户重命名、
#   解压后只留 exe、或转发他人时丢失。而报障往往发生在下载数日之后。
#   注入程序内部则跟随 exe 本身，诊断包里永远带得出来。
#
# 用法：
#   powershell -ExecutionPolicy Bypass -File build\release.ps1 -Version 2.0.0-rc.1
#
# 注意：本脚本只构建，不上传、不打 tag、不动 git 状态。

param(
    # 版本号，不带 v 前缀。封测建议用 2.0.0-rc.1 这类形式，
    # 与正式版明确区分——用户报障时第一眼就能看出手里是测试包。
    [Parameter(Mandatory = $true)]
    [string]$Version,

    # 跳过前端构建。仅在刚构建过前端、只想重新编译 Go 时使用。
    [switch]$SkipFrontend
)

$ErrorActionPreference = 'Stop'

# 切到项目根目录（本脚本位于 build/ 下），使后续相对路径不依赖调用位置
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

Write-Host "项目根目录: $root" -ForegroundColor Cyan

# ── 1. 采集 git 信息 ─────────────────────────────────────────

$commit = (git rev-parse --short HEAD).Trim()

# 工作树是否有未提交改动。这一项必须记录：带 dirty 标记的包所对应的
# 代码在仓库里根本不存在，据其哈希 checkout 会得到另一份代码。
$status = git status --porcelain
$dirty = if ([string]::IsNullOrWhiteSpace($status)) { 'false' } else { 'true' }

# 构建时刻用 UTC 并带 Z 后缀，避免跨时区阅读歧义
$builtAt = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

Write-Host "版本号  : $Version"
Write-Host "提交    : $commit"
Write-Host "工作树  : $(if ($dirty -eq 'true') { '有未提交改动' } else { '干净' })"
Write-Host "构建时刻: $builtAt"

if ($dirty -eq 'true') {
    Write-Host ''
    Write-Host '⚠ 工作树有未提交改动。' -ForegroundColor Yellow
    Write-Host '  产出的包会带 [已修改] 标记，其对应代码无法从仓库还原。' -ForegroundColor Yellow
    Write-Host '  正式发布前请先提交。封测包可以接受，但要清楚这一点。' -ForegroundColor Yellow
    Write-Host ''
}

# ── 2. 构建 ──────────────────────────────────────────────────

# -X 的目标必须是 main 包中的包级字符串变量，见 app_meta.go
$ldflags = @(
    "-X main.appVersion=$Version"
    "-X main.appCommit=$commit"
    "-X main.appBuiltAt=$builtAt"
    "-X main.appDirty=$dirty"
    # -w -s 去掉调试信息与符号表，减小体积。
    # 不影响 panic 栈的函数名，故排障能力不受损。
    '-w'
    '-s'
) -join ' '

$wailsArgs = @('build', '-clean', '-ldflags', $ldflags)
if ($SkipFrontend) {
    # -s 跳过前端构建
    $wailsArgs += '-s'
}

Write-Host ''
Write-Host '开始构建…' -ForegroundColor Cyan
& wails @wailsArgs
if ($LASTEXITCODE -ne 0) {
    throw "wails build 失败，退出码 $LASTEXITCODE"
}

# ── 3. 重命名产物 ────────────────────────────────────────────

$src = Join-Path $root 'build\bin\kazeusa.exe'
if (-not (Test-Path -LiteralPath $src)) {
    throw "未找到构建产物: $src"
}

$suffix = if ($dirty -eq 'true') { '-dirty' } else { '' }
$outName = "kazeusa-$Version-$commit$suffix.exe"
$dst = Join-Path $root "build\bin\$outName"

# 同名产物直接覆盖：同一版本同一提交的重复构建理应等价，
# 保留多份只会让人分不清该发哪个
if (Test-Path -LiteralPath $dst) {
    Remove-Item -LiteralPath $dst -Force
}
Move-Item -LiteralPath $src -Destination $dst

$sizeMB = [math]::Round((Get-Item -LiteralPath $dst).Length / 1MB, 2)

Write-Host ''
Write-Host '构建完成' -ForegroundColor Green
Write-Host "  产物: $dst"
Write-Host "  体积: $sizeMB MB"
Write-Host ''
Write-Host '发布前请自查：' -ForegroundColor Cyan
Write-Host '  1. 启动后到设置页确认「构建标识」显示的版本与哈希与预期一致'
Write-Host '  2. 点一次「导出诊断包」，确认能生成且自动打开文件夹'
Write-Host '  3. 确认分发目录内不含 .lib / .exp / lua_static.lib，只保留三个 DLL'
Write-Host '  4. 清理测试机 config\lua\ 下的遗留清单（内含假密钥）'
