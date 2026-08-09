# net-diag.ps1 -- Steam store API connectivity diagnosis
#
# Pure ASCII on purpose: PowerShell 5.1 reads .ps1 as ANSI(GBK) by default,
# so any non-ASCII byte makes the parser report errors at unrelated lines.
#
# Usage (run in repo root):
#   powershell -ExecutionPolicy Bypass -File tools\net-diag.ps1 -Tag baseline
#
# Run it once per network state and keep the Tag distinct, e.g.
#   -Tag no-accel        (everything off)
#   -Tag uu-on           (UU booster running)
#   -Tag leishen-on      (Leishen booster running)
#   -Tag proxy-sys       (proxy app, system-proxy mode)
#   -Tag proxy-tun       (proxy app, TUN mode)
#
# Output goes to tools\diag-<Tag>.txt

param([string]$Tag = "default")

$ErrorActionPreference = "Continue"
$out = Join-Path $PSScriptRoot ("diag-" + $Tag + ".txt")
$lines = New-Object System.Collections.ArrayList

function Add-Line($text) { [void]$lines.Add($text) }

function Add-Section($title) {
    Add-Line ""
    Add-Line ("=== " + $title + " ===")
}

# Same endpoint the app uses. Chinese term is URL-encoded to keep this
# file pure ASCII: term = "deep rock galactic" in Chinese.
$cnTerm = "https://store.steampowered.com/api/storesearch/?cc=CN&l=schinese&term=%E6%B7%B1%E5%B2%A9%E9%93%B6%E6%B2%B3"
$enTerm = "https://store.steampowered.com/api/storesearch/?cc=CN&l=schinese&term=deep+rock"

Add-Line ("Tag       : " + $Tag)
Add-Line ("Timestamp : " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))

# ---------------------------------------------------------------
# 1. Proxy configuration as seen by each layer
# ---------------------------------------------------------------
Add-Section "WinINET system proxy (what browsers use)"
$reg = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings"
try {
    $s = Get-ItemProperty -Path $reg -ErrorAction Stop
    Add-Line ("ProxyEnable   : " + $s.ProxyEnable)
    Add-Line ("ProxyServer   : " + $s.ProxyServer)
    Add-Line ("AutoConfigURL : " + $s.AutoConfigURL)
} catch {
    Add-Line ("read failed: " + $_.Exception.Message)
}

Add-Section "Environment variables (the ONLY thing Go default Transport reads)"
foreach ($n in @("HTTP_PROXY","HTTPS_PROXY","NO_PROXY","http_proxy","https_proxy","no_proxy")) {
    $v = [Environment]::GetEnvironmentVariable($n)
    if ([string]::IsNullOrEmpty($v)) { $v = "(unset)" }
    Add-Line ($n.PadRight(12) + ": " + $v)
}

# ---------------------------------------------------------------
# 2. DNS and interfaces -- tells TUN-mode boosters apart
# ---------------------------------------------------------------
Add-Section "DNS resolution of store.steampowered.com"
try {
    $ips = [System.Net.Dns]::GetHostAddresses("store.steampowered.com")
    foreach ($ip in $ips) { Add-Line ("  " + $ip.IPAddressToString) }
} catch {
    Add-Line ("resolve failed: " + $_.Exception.Message)
}

Add-Section "Active network adapters (look for virtual/TAP/TUN devices)"
try {
    Get-NetAdapter | Where-Object { $_.Status -eq "Up" } | ForEach-Object {
        Add-Line ("  " + $_.Name + " | " + $_.InterfaceDescription)
    }
} catch {
    Add-Line "Get-NetAdapter unavailable"
}

Add-Section "Route to the resolved address (first hop only)"
try {
    $first = ([System.Net.Dns]::GetHostAddresses("store.steampowered.com"))[0].IPAddressToString
    $r = Find-NetRoute -RemoteIPAddress $first -ErrorAction Stop
    Add-Line ("  via interface : " + $r[0].InterfaceAlias)
    Add-Line ("  local address : " + $r[0].IPAddress)
} catch {
    Add-Line ("Find-NetRoute failed: " + $_.Exception.Message)
}

# ---------------------------------------------------------------
# 3. Three request paths, timed
# ---------------------------------------------------------------
function Test-Curl($label, $url, $extraArgs) {
    Add-Line ""
    Add-Line ("-- " + $label)
    $curl = Join-Path $env:SystemRoot "System32\curl.exe"
    if (-not (Test-Path $curl)) { Add-Line "curl.exe not found"; return }

    $args = @("-s","-o","NUL","-w","http=%{http_code} dns=%{time_namelookup}s connect=%{time_connect}s total=%{time_total}s","--max-time","20")
    if ($extraArgs) { $args += $extraArgs }
    $args += $url

    $sw = [Diagnostics.Stopwatch]::StartNew()
    $res = & $curl $args 2>&1
    $sw.Stop()
    Add-Line ("  " + ($res -join " "))
    Add-Line ("  wall clock : " + $sw.ElapsedMilliseconds + " ms")
}

Add-Section "Path A: forced direct (bypasses every proxy setting)"
Test-Curl "CN term" $cnTerm @("--noproxy","*")
Test-Curl "EN term" $enTerm @("--noproxy","*")

Add-Section "Path B: curl default (honours env vars, ignores WinINET)"
Add-Line "This is the closest analogue to the app's current behaviour."
Test-Curl "CN term" $cnTerm $null

Add-Section "Path C: WinINET (what the browser does)"
$sw = [Diagnostics.Stopwatch]::StartNew()
try {
    $r = Invoke-WebRequest -Uri $cnTerm -TimeoutSec 20 -UseBasicParsing
    $sw.Stop()
    Add-Line ("  status     : " + $r.StatusCode)
    Add-Line ("  bytes      : " + $r.RawContentLength)
    Add-Line ("  wall clock : " + $sw.ElapsedMilliseconds + " ms")
} catch {
    $sw.Stop()
    Add-Line ("  FAILED after " + $sw.ElapsedMilliseconds + " ms: " + $_.Exception.Message)
}

# ---------------------------------------------------------------
# 4. Raw TCP reachability -- separates TLS problems from routing
# ---------------------------------------------------------------
Add-Section "Raw TCP 443 handshake (no TLS, no HTTP)"
$sw = [Diagnostics.Stopwatch]::StartNew()
try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $ok = $tcp.ConnectAsync("store.steampowered.com", 443).Wait(10000)
    $sw.Stop()
    if ($ok) { Add-Line ("  connected in " + $sw.ElapsedMilliseconds + " ms") }
    else { Add-Line ("  TIMED OUT after " + $sw.ElapsedMilliseconds + " ms") }
    $tcp.Close()
} catch {
    $sw.Stop()
    Add-Line ("  FAILED: " + $_.Exception.Message)
}

# ---------------------------------------------------------------
Set-Content -Path $out -Value $lines -Encoding UTF8
Write-Host ""
Write-Host ("Report written to: " + $out)
Write-Host "Please paste its contents back."
Write-Host ""
$lines | ForEach-Object { Write-Host $_ }
