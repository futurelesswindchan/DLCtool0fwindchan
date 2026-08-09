# release.ps1 -- build a release/beta package for kazeusa
#
# This script is intentionally ASCII-only. Windows PowerShell 5.1 reads .ps1
# files as ANSI (GBK on zh-CN systems) unless the file carries a UTF-8 BOM.
# Non-ASCII bytes then get misread and can swallow string terminators, which
# surfaces as parse errors on unrelated lines. Keeping this file ASCII-only
# removes the whole class of problem and keeps the script portable to machines
# with different regional settings.
#
# What it does:
#   1. Collect git commit hash and working-tree cleanliness
#   2. Inject version / commit / build time / dirty flag via -ldflags
#   3. Emit an executable named with the version and commit
#
# Why the commit hash is stamped in two places (filename AND binary):
#   The filename makes it easy to tell packages apart when sharing them, but
#   it is lost when users rename the file, keep only the exe after unzipping,
#   or forward it to someone else. Bug reports typically arrive days after
#   download, when the archive is long gone. The value injected into the
#   binary travels with the exe itself and always shows up in the diagnostics
#   bundle.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File build\release.ps1 -Version 2.0.0-rc.1
#
# This script only builds. It does not upload, tag, or modify git state --
# a build script that mutates the repository makes failures far harder to
# roll back.

param(
    # Version string without a leading "v". For closed beta use something like
    # 2.0.0-rc.1 so beta packages are distinguishable from the final release:
    # otherwise a leaked beta gets reported as if it were the stable build.
    [Parameter(Mandatory = $true)]
    [string]$Version,

    # Skip the frontend build. Only useful right after a successful frontend
    # build when iterating on Go code alone.
    [switch]$SkipFrontend
)

$ErrorActionPreference = 'Stop'

# This script lives in build/, so the project root is one level up.
# Resolving it explicitly means the script works regardless of the caller's
# current directory.
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

Write-Host "Project root : $root" -ForegroundColor Cyan

# --- 1. Collect git information ---------------------------------------------

$commit = (git rev-parse --short HEAD).Trim()

# Whether the working tree has uncommitted changes. This matters more than the
# hash itself: for a dirty build, the corresponding source does NOT exist in
# the repository, so checking out that hash yields different code. Without an
# explicit marker, debugging proceeds against the wrong source silently.
$status = git status --porcelain
$dirty = if ([string]::IsNullOrWhiteSpace($status)) { 'false' } else { 'true' }

# UTC with a Z suffix, so the timestamp reads unambiguously across timezones.
$builtAt = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

Write-Host "Version      : $Version"
Write-Host "Commit       : $commit"
Write-Host "Working tree : $(if ($dirty -eq 'true') { 'DIRTY (uncommitted changes)' } else { 'clean' })"
Write-Host "Build time   : $builtAt"

if ($dirty -eq 'true') {
    Write-Host ''
    Write-Host 'WARNING: the working tree has uncommitted changes.' -ForegroundColor Yellow
    Write-Host '  The output will be marked as modified, and its exact source' -ForegroundColor Yellow
    Write-Host '  cannot be restored from the repository. Do not distribute it.' -ForegroundColor Yellow
    Write-Host '  Commit first for anything you plan to hand to testers.' -ForegroundColor Yellow
    Write-Host ''
}

# --- 2. Build ---------------------------------------------------------------

# Each -X target must be a package-level string variable in package main.
# See app_meta.go for the declarations and the rationale behind each one.
$ldflags = @(
    "-X main.appVersion=$Version"
    "-X main.appCommit=$commit"
    "-X main.appBuiltAt=$builtAt"
    "-X main.appDirty=$dirty"
    # -w -s strip debug info and the symbol table to reduce size. Function
    # names in panic traces are unaffected, so troubleshooting still works.
    '-w'
    '-s'
) -join ' '

$wailsArgs = @('build', '-clean', '-ldflags', $ldflags)
if ($SkipFrontend) {
    $wailsArgs += '-s'
}

Write-Host ''
Write-Host 'Building...' -ForegroundColor Cyan
& wails @wailsArgs
if ($LASTEXITCODE -ne 0) {
    throw "wails build failed with exit code $LASTEXITCODE"
}

# --- 3. Rename the artifact -------------------------------------------------

$src = Join-Path $root 'build\bin\kazeusa.exe'
if (-not (Test-Path -LiteralPath $src)) {
    throw "Build artifact not found: $src"
}

$suffix = if ($dirty -eq 'true') { '-dirty' } else { '' }
$outName = "kazeusa-$Version-$commit$suffix.exe"
$dst = Join-Path $root "build\bin\$outName"

# Overwrite an existing artifact with the same name: rebuilding the same
# version at the same commit should be equivalent, and keeping several copies
# only makes it unclear which one to ship.
if (Test-Path -LiteralPath $dst) {
    Remove-Item -LiteralPath $dst -Force
}
Move-Item -LiteralPath $src -Destination $dst

$sizeMB = [math]::Round((Get-Item -LiteralPath $dst).Length / 1MB, 2)

Write-Host ''
Write-Host 'Build complete' -ForegroundColor Green
Write-Host "  Output : $dst"
Write-Host "  Size   : $sizeMB MB"
Write-Host ''
Write-Host 'Pre-release checklist:' -ForegroundColor Cyan
Write-Host '  1. Launch the app, open Settings > About, and confirm the build'
Write-Host '     identifier shows the expected version and commit hash.'
Write-Host '  2. Click "export diagnostics" once. Confirm the zip is created,'
Write-Host '     the folder opens, and config.masked.json contains no token.'
Write-Host '  3. Strip .lib / .exp / lua_static.lib from the distribution'
Write-Host '     folder. Ship only the three DLLs.'
Write-Host '  4. Clean leftover manifests from the test machine at'
Write-Host '     <Steam>\config\lua\ -- they contain fake keys. Restart Steam'
Write-Host '     to clear keys still held in the injector memory.'
