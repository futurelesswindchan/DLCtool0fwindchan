# tools/ 诊断工具

排障用的一次性工具，不进产物、不参与主构建。

## 背景

08-02 实机暴露搜索频繁超时（`context deadline exceeded ... awaiting headers`），
而同一 URL 浏览器可秒开。当时的假设是
「Go 默认 Transport 不读 WinINET 系统代理」——**这个假设被这两个工具推翻了**。
结论见 `docs/DECISIONS-2.md` 的 08-02 第三条。

## netprobe

用 Go 自己的 HTTP 栈实测四种传输配置。**必须用 Go 而非只用 curl 对照**：
curl 读环境变量、不读 WinINET，行为「类似」盒子但不等于盒子；
若加速器走 WFP 驱动做进程白名单，`curl.exe` 与 `kazeusa.exe`
拿到的待遇可能完全不同。

### 一次性对照

```
go run .\tools\netprobe            默认每种配置跑 3 次
go run .\tools\netprobe -n 5       改次数
go run .\tools\netprobe -term deep+rock
```

四种配置各自回答一个问题：

| 配置 | 回答什么 |
| :--- | :--- |
| 当前实现 | 与 `store_client.go` 完全一致，是基准 |
| 强制直连 | 代理是否反而是障碍 |
| 系统代理 | 读 WinINET，即浏览器走的那条路 |
| 调优直连 | 是不是 HTTP/2 或握手参数的问题 |

### 长时间采样（间歇性故障必用）

```
go run .\tools\netprobe -watch 30s
go run .\tools\netprobe -watch 1m -log tools\night.log
```

Ctrl+C 结束并打印汇总（成功率、耗时分位、失败分类）。

**为什么需要它**：五轮对照测试每轮仅约 10 秒、四配置全部成功，
而一小时前同一套代码是四配置全败。
**对间歇性故障，短窗口采样的阴性结果几乎没有信息量。**

采样模式只测「当前实现」一种配置——要回答的是「盒子现在通不通」，
四种全测会把单次采样拉长到近一分钟，反而降低时间分辨率。

失败时会额外快照 DNS 解析结果与代理状态。这是关键设计：
故障时最想知道的不是「失败了」，而是「失败那一刻环境是什么样」，
而那个状态事后无法复原。解析结果落在 fake-ip 网段
（`198.18.0.0/15` 等）时会附一句提示——它是 TUN 模式代理的标志，
「假 IP + 请求失败」同时出现即「虚拟网卡接住了但没转发出去」。

## net-diag.ps1

PowerShell 侧的环境快照：WinINET 注册表值、环境变量、DNS、
虚拟网卡列表、到目标地址的路由首跳，以及三条请求路径的耗时对照
（强制直连 / curl 默认 / WinINET）。

```
powershell -ExecutionPolicy Bypass -File tools\net-diag.ps1 -Tag baseline
```

报告写到 `tools\diag-<Tag>.txt`。每种网络状态各跑一次并保持 Tag 不同：
`baseline` / `uu-on` / `leishen-on` / `proxy-sys` / `proxy-tun`。

⚠️ 本文件一律**纯 ASCII**：PowerShell 5.1 默认按 ANSI(GBK) 读 `.ps1`，
无 BOM 的 UTF-8 中文会被误读，且报错行号指向完全无关的位置。

## 已实测的结论摘要

| 事实 | 证据 |
| :--- | :--- |
| 加速器覆盖本进程，进程白名单担忧不成立 | 雷神启用后出现 `utun233` 网卡、路由经它、TCP 建连 7ms（基线 107ms） |
| UU 与雷神机制不同 | UU 不建虚拟网卡，改 DNS 解析结果导向更近的 Akamai 边缘 |
| 系统代理是最慢的一条路 | 直连 240ms 对系统代理 784ms~1.8s；浏览器 WinINET 1.6~2.8s |
| 15s 超时过长 | 成功耗时实测上限 2.5s |

`diag-*.txt` 与 `netwatch.log` 是采样产物，可随时删。
