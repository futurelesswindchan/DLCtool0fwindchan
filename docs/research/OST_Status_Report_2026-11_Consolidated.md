# OpenSteamTool 项目近况调研报告 · 整合版（2026-11）

> **定位**：以第三方视角记录 OpenSteamTool（下称 OST）截至 2026-11-06 的维护状态、运行故障、签名供应链与社区替代来源，供下游集成方评估依赖风险。
> **底稿**：本报告整合并取代以下三份文稿：`OST_Status_Report_2026-11.md`（第一版，数据截止 11-04）、`OST_Status_Report_2026-11_Supplement.md`（补充篇一）、`OST_Status_Report_2026-11_Supplement2.md`（补充篇二，未完稿）。三份文稿与本报告不一致的地方，以本报告为准，逐条更正列于 §10。
> **前置阅读**：`docs/research/OST_Architecture_Analysis.md`（源码架构）
> **数据截止**：2026-11-06（UTC）

**证据标记**：本报告对关键陈述标注证据等级。

| 标记     | 含义                                                                        |
| -------- | --------------------------------------------------------------------------- |
| 〔核实〕 | 调研者直接读取的一手数据：GitHub API 字段、仓库文件、提交内容、Release 页面 |
| 〔转述〕 | 社区成员在 issue 评论或 README 中的陈述，调研者未复现                       |
| 〔推断〕 | 调研者依据上述材料作出的推理，推理依据随文给出                              |

---

## 1. 摘要

1. **主仓库处于事实上的无人维护状态**。最后一次合并为 2026-07-06（PR #153），最后一个 Release 为 2026-06-13 的 1.4.8。截至 11-06，主仓库没有新推送，可见记录中也没有维护者的回复〔核实〕。
2. **签名供应链存在两层断点**。官方签名仓库 `OpenSteam001/steam-monitor` 最后一次推送是 2026-09-26〔核实〕。社区的自动解析仓库 `hazre/steam-monitor` 从 10-21 起，因为当前 stable 构建 1792618285 有 5 个条目解析失败，按"任一失败即不发布"的策略连续 7 次没有发布〔核实〕。官方停更早于 hazre 第一次被拦截，所以 hazre 的拦截不能解释官方停更〔推断〕。
3. **社区镜像部分填补了缺口，但它与校验结论存在分歧**。`JYM34/steam-monitor` 于 10-22 发布了 stable 1792618285 的完整签名，其中包括 hazre 判定失败的 5 个条目〔核实〕。两者哪一方正确，本报告无法判定。10-22 之后的 beta 构建在所有已知来源中都没有签名〔核实〕。
4. **故障共有三类，另有一项相关的服务端变化**：① 签名 / IPC 规格缺失；② `CNetPacket` 结构体布局变化导致启动崩溃；③ manifest request code 只下发给拥有者账户；此外，家庭共享改由服务端强制校验许可〔转述〕。①② 可以在客户端修复，相关补丁以 PR 形式提交后一直未合并。③ 和家庭共享的变化发生在服务端。
5. **社区替代来源的信任边界差异很大**。isxlan0 的 Release 对应 PR #218 中可追溯的提交，作者声明只做了兼容修复〔核实〕；该 PR 的 diff 有 20 个文件、+752/−721 行，调研者没有逐行审阅。BetterSteamTools 默认开启自动更新编译选项、`[donate]`（用用户账户为他人申请 manifest code）和自定义 URI 处理器〔核实，README 与发布说明〕。SteamDaddy-Dll 关闭了 Issues，默认 manifest 上游指向未知域名，推荐以管理员权限执行远程脚本，并在主仓库 issue 区批量推广〔核实〕。
6. **Steam beta 渠道的 IPC 方法表已经变化**。beta 1792626604 中有 3 个方法的 `method_index` 加 1，4 个 `funcHash` 改变〔核实，JYM34 文件〕。这批改动进入 stable 后，依赖"沿用旧值再重新扫描"的社区手工方法会失效〔推断〕。

---

## 2. 调研范围、方法与局限

### 2.1 数据来源

| 对象                                           | 获取方式                            | 覆盖范围                                                                                                                    |
| ---------------------------------------------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `OpenSteam001/OpenSteamTool` issue / PR 元数据 | GitHub REST API，未认证只读         | 全部 203 条（152 issue + 51 PR），快照时间 11-04                                                                            |
| 同仓库 commits / releases / branches           | GitHub REST API                     | 最近 25 条提交、10 个 Release、全部分支                                                                                     |
| 议题评论                                       | GitHub REST API；API 失败时改读网页 | 通读：#195（32/32）、#205（7/7）、#223、#225、#226。部分：#208（前 14 条）。仅正文：#224、#227。抽样（11-04）：其余约 30 条 |
| 三个 steam-monitor 仓库                        | API 与网页                          | 仓库元数据；hazre issue 列表及 #41、#47 正文；JYM34 两次提交的完整 diff                                                     |
| 社区 fork                                      | 网页与 API                          | isxlan0 Release 页与 PR #218 页；BetterSteamTools Releases 页与 README；SteamDaddy-Dll 元数据、Release、README、部分提交    |
| 账户                                           | API                                 | `Contrary-v2`                                                                                                               |

### 2.2 局限

- 调研者没有下载、解压或运行任何二进制、附件和安装脚本。
- 社区成员的技术结论（结构体偏移、签名正确性、Valve 服务端行为）均未复现，文中标为〔转述〕。
- #208 第 15 条以后的评论，以及 #224、#227 的评论，均抓取失败。#194、#197、#209 和 PR #200 / #201 / #204 / #212 的评论，11-04 之后没有重读。
- SteamDaddy-Dll 只读了 6 条提交，没有与上游做源码比对。
- 没有查证 `20770407.xyz`、`993499094.xyz`、`stats.luastools.xyz`、`generator.ryuu.lol` 的归属与行为。
- 没有查证 `hazre/steam-monitor` 的解析结果发布到何处，也没有查证它与 `OpenSteam001/steam-monitor` 的关系。
- BetterSteamTools Releases 页的抓取结果不含发布日期，本报告只引用各版本的发布说明。
- 主仓库的按月统计与标签分布是 11-04 的快照；其余数据截至 11-06。

---

## 3. 维护状态

### 3.1 总体指标〔核实〕

| 指标                         | 11-04                                     | 11-06      |
| ---------------------------- | ----------------------------------------- | ---------- |
| Star / Fork                  | 2562 / 439                                | 2577 / 443 |
| `open_issues_count`（含 PR） | 80（68 issue + 12 PR）                    | 80         |
| Issue 总数                   | 152（open 68 / closed 84）                | 未重新统计 |
| PR 总数                      | 51（open 12 / merged 23 / 关闭未合并 16） | 未重新统计 |
| 最后一次 push                | 2026-07-06T09:28:57Z                      | 不变       |
| 最后一次合并                 | PR #153，2026-07-06                       | 不变       |
| 最后一个 Release             | 1.4.8，2026-06-13                         | 不变       |
| 是否归档                     | 否                                        | 否         |

### 3.2 按月活动（11-04 快照）〔核实〕

| 月份                | 新增 issue | 关闭 issue | 当月新增且至今 open | 新增 PR | 合并 PR |
| ------------------- | ---------- | ---------- | ------------------- | ------- | ------- |
| 2026-05             | 58         | 49         | 5                   | 23      | 15      |
| 2026-06             | 44         | 26         | 19                  | 17      | 7       |
| 2026-07             | 17         | 4          | 15                  | 2       | 1       |
| 2026-08             | 10         | 3          | 8                   | 2       | 0       |
| 2026-09             | 7          | 1          | 6                   | 5       | 0       |
| 2026-10             | 15         | 1          | 14                  | 2       | 0       |
| 2026-11（至 04 日） | 1          | 0          | 1                   | 0       | 0       |

从 2026-07 起，每月合并的 PR 降到 1 个，8 月以后为 0。2026-10 新增 issue 回升到 15 条，时间上与 §4.1 所列 Steam 构建的更新一致，但当月只关闭了 1 条。

### 3.3 维护者活动〔核实〕

- 23 个已合并 PR 中，12 个由仓库所有者 `OpenSteam001` 本人提交。
- 在可见的事件记录里，所有者最后一次主动操作是 2026-07-06 合并 PR #153。2026-09-09 在 #169 有一条 `mentioned` 事件，属于被他人提及，不是所有者的操作。
- 11-04 抓取的全仓库最近 100 条评论中，没有 OWNER、MEMBER 或 COLLABORATOR 身份的评论。11-04 至 11-06 新增的评论也都来自 `author_association` 为 NONE 的账户。
- 2026-07-01 之后共关闭 9 条 issue。抽查了其中 5 条（#209、#202、#187、#155、#136）的 `closed_by`：4 条由提问者本人关闭；#136 的作者账户已注销（显示为 ghost），关闭者字段为空。
- 官方签名仓库 `OpenSteam001/steam-monitor` 的 `pushed_at` 为 2026-09-26，晚于主仓库停更约 11 周。

调研者据此判断，主仓库从 2026-07 中旬起处于事实上的无人维护状态〔推断〕。没有找到维护者对此的公开声明，停更原因不明。

### 3.4 议题标签分布（11-04 快照）〔核实〕

| 标签          | open | closed | 合计 |
| ------------- | ---- | ------ | ---- |
| bug           | 43   | 46     | 89   |
| enhancement   | 18   | 15     | 33   |
| documentation | 3    | 4      | 7    |
| 无标签        | 4    | 19     | 23   |

另有 3 条议题同时带 `duplicate` 和 `bug` 标签，均已关闭，已计入 bug 行。

---

## 4. 故障分析

### 4.1 故障 ①：签名与 IPC 规格缺失

**机制**〔核实，README〕：OST 每次启动时计算 `steamclient64.dll` 和 `steamui.dll` 的 SHA-256，再按哈希获取对应的 pattern TOML 和 IPC TOML。README 写明的查找顺序是：GitHub raw（`OpenSteam001/steam-monitor`）→ jsDelivr CDN → 本地缓存。远端返回 404 且本地没有缓存时，OST 弹窗提示，并禁用这个 DLL 关联的全部钩子。用户可以用 `[remote] url_template` 换成其他镜像。

上述查找顺序引自 BetterSteamTools 的 README。上游 OST 的 README 本次没有重读，两者是否一致没有核对。另据 #226 评论（sergeyrenamed-coder，11-02），获取地址中还有 `generator.ryuu.lol`，该评论者所在网络访问这个域名时，TLS 握手被重置〔转述〕。README 中没有出现这个域名，它在哪个版本中使用，本报告没有查明。

**影响**：`addappid` 等核心功能失效（#224）〔转述〕。

**受影响的构建**：Steam 的 BuildID 就是该客户端构建的 Unix 时间戳。JYM34 README 中各构建的 "Updated (UTC)" 列与 BuildID 换算出的时间逐秒相同〔核实〕，下表的"构建时间"按这种方法换算。

| BuildID    | 构建时间（UTC） | 渠道                                          | 官方签名                        | 相关议题               |
| ---------- | --------------- | --------------------------------------------- | ------------------------------- | ---------------------- |
| 1788652215 | 09-05 23:50     | 未确认                                        | 有（`pattern` 分支 09-06 提交） | #197、#205             |
| 1791918634 | 10-13 19:10     | beta〔核实，JYM34 记录〕                      | 无                              | #208、#209、#213       |
| 1792618285 | 10-21 21:31     | stable〔核实，JYM34 记录与 hazre issue 标题〕 | 无                              | #223、#225、#226、#227 |
| 1792626604 | 10-21 23:50     | beta〔核实，JYM34 记录〕                      | 无                              | 无                     |
| 1793235415 | 10-29 00:56     | beta〔推断〕                                  | 无                              | #224                   |

1793235415 被判为 beta 的依据是：hazre 的 bot 在 10-21 至 11-06 开出的 7 个 issue 中，stable 哈希始终是 1792618285 的哈希，所以 stable 渠道在这段时间里没有换过构建〔推断〕。

#205 报告的是 1788652215 缺签名，但官方仓库已经有这个构建的签名，#197 也记录了它经 jsDelivr 正常加载。#205 的提交者使用的是社区构建 `1.4.8-fix117`，原因可能在于该构建的签名源配置，本报告没有核实。

签名的生成、拦截和镜像覆盖情况见 §5。

### 4.2 故障 ②：`CNetPacket` 结构体布局变化

**机制**〔转述，PR #212〕：从 Steam 构建 1791918634 起，`CNetPacket::m_pubData` 之前多出 8 字节，后续字段整体后移 0x08。

| 字段                 | 旧偏移 | 新偏移 |
| -------------------- | ------ | ------ |
| `m_pubData`          | +0x08  | +0x10  |
| `m_cubData`          | +0x10  | +0x18  |
| `m_cRef`             | +0x14  | +0x1C  |
| `m_pubNetworkBuffer` | +0x18  | +0x20  |
| `m_pNext`            | +0x20  | +0x28  |

`RecvPkt` 钩子按旧偏移读取，触发访问违例，Steam 启动时崩溃。PR 作者给出两种验证方法：反汇编 steamclient 内部的包处理函数；用透传探针记录真实数据包。

**佐证**〔转述，#208，matheeshapathirana，10-15〕：在未修改的 1.4.8 上加载补齐的 TOML，只要不含 `RecvPkt` 条目，Steam 可以启动，但会弹窗提示缺少 `RecvPkt`，依赖它的功能保持关闭。加入 `RecvPkt` 条目后，必须使用合入 #212 的 DLL，否则启动即崩溃。

**与签名供应的交互**〔推断〕：`JYM34/steam-monitor` 发布的 1792618285 steamclient 文件包含 `RecvPkt`（rva `0x5BCA50`）〔核实〕。按上一段的结论，未修补的 1.4.8 加载这份完整文件时，会在启动阶段崩溃。所以 #226 推荐的镜像方案只适用于已经修复偏移的构建。

**修复状态**〔核实〕：PR #212（10-15）和 PR #218（10-22）都包含修复，均未合并。isxlan0 以 PR #218 的头提交 `7e83f76` 发布了 Release，见 §7.2。

**评估**〔推断〕：OST 的远端元数据只描述函数位置（RVA / sig），结构体布局和字段偏移仍然硬编码在源码中。Steam 修改内部数据结构后，只有发布新的二进制才能修复。在无人维护的情况下，这类修复只能经由 fork 分发。

### 4.3 故障 ③：manifest request code 只下发给拥有者

**现象**〔转述，#195〕：从 2026-09-09 起，自动获取 manifest 失败。Steam 日志显示 `Failed to get manifest request code, 'Access Denied'`，界面提示 "No Internet Connection"。#195 有 32 条评论，是全仓库评论最多的议题。

**关于机制的社区陈述**〔均为转述〕：

| 来源                           | 陈述                                                                                         |
| ------------------------------ | -------------------------------------------------------------------------------------------- |
| #195，nomi541，09-10           | Steam 改为先校验账户是否拥有该游戏，再下发 manifest                                          |
| #195，Berkecann，09-13         | Steam 09-09 的更新使"匿名获取 manifest request code"失效                                     |
| #194 评论                      | 内置上游返回的 request code 无法通过校验；官方接口签发的 code 有效，但要求账户拥有对应 depot |
| #195，juliocoliveira，09-10    | 本地已有 manifest 的游戏，没有所有权也能下载；需要新 manifest 的更新则失败                   |
| #195，TotalRealityCheck，09-13 | 预计下载环节也会加入认证（预测）                                                             |

**排查证据**〔转述〕：#197 的提交者换用 4 个来源不同的 manifest GID，结果相同。JYM34 于 09-11 报告 `manifest.opensteamtool.com` 从多个网络访问都返回 Cloudflare 403。

**影响**：OST 内置的三个上游（`opensteamtool`、`steamrun`、`wudrm`）在 #195 的讨论中都被报告失效〔转述〕。依赖自动获取 manifest 的下载和更新流程因此中断。

**社区应对**：

| 方式                        | 内容                                                                                                                         | 状态                                                                                                                                                             |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 手动导入                    | 把 `.manifest` 放进 `<Steam>/depotcache`（borgox，09-09）                                                                    | 有人确认有效（WholeHorse），也有人称无效（juliocoliveira）。GuiMasTerz7-dev 称需要先清除 Steam 下载缓存。DaKaRTheWind 称该目录的文件在重启后会被删除〔均为转述〕 |
| manifest 下载站             | hubcapmanifest（需加入 Discord、通过验证、生成 API key，其客户端要求手动删除 OST DLL）；steamtools.site（需经过广告页）      | 第三方站点，来源与内容无法核实                                                                                                                                   |
| ManifestDeX                 | Berkecann 称找到绕过方法，通过 `manifest.manifestdex.com` 提供；PR #200 把它接入为可选上游；称已在构建 1788652215 上测试成功 | 后端闭源；评论区多次报告服务不可用                                                                                                                               |
| BetterSteamTools `[donate]` | 用用户自己的账户，为该账户拥有的 depot 向 Steam 申请 request code，供其他用户下载；**默认开启**                              | 见 §7.3                                                                                                                                                          |
| SteamDaddy-Dll              | 默认上游改为 `20770407.xyz`，备选 `steamapi.993499094.xyz`                                                                   | 见 §7.4                                                                                                                                                          |

**评估**〔推断〕：如果 nomi541 与 #194 的描述成立，有效的 request code 只能由拥有对应 depot 的账户取得。那么任何仍然可用的上游，最终都要依赖拥有者账户：要么是服务运营者自己的账户，要么是像 `[donate]` 那样由用户账户贡献。这一推断还解释了 bilibili8888 的观察：hubcapmanifest 上冷门游戏大多没有 manifest。在这种情况下，请求量和由此带来的账户风险被转移到了贡献 code 的账户上。PR #201（自定义上游模板）和 PR #204（多上游自动切换）能提高容错能力，但它们不能产生有效的 code。

### 4.4 相关变化：家庭共享改由服务端校验

**BetterSteamTools v1.0.8 的发布说明**〔转述〕：Valve 已在服务端强制许可锁。账户运行一个通过 `addappid` 添加、自己并没有许可的游戏时，CM 服务器先发送 `SharedLibraryStopPlaying(9406)`，再发送 `ClientLoggedOff(757)`，宽限期过后终止会话。客户端的两种应对都无效：清除 9406 只能让游戏窗口继续开着，会话仍然会被终止；屏蔽 757 只能把踢出变成约 6 秒的重连，因为服务器还会关闭 socket。该版本因此移除了家庭共享绕过。

**SteamDaddy-Dll 的提交说明**〔转述〕：提交 `d43abed`（11-04）称恢复了真实的 lender ID，并修复了"在别处登录"导致的掉线。

两种说法不一致。BetterSteamTools 各版本的发布日期没有抓取到，所以两者的先后顺序无法确定。调研者也无法验证任何一方的实际效果。

---

## 5. 签名供应链

### 5.1 三个 steam-monitor 仓库〔核实〕

| 仓库                       | 创建       | 最后推送   | Star / Fork | 说明                                                                                                                    |
| -------------------------- | ---------- | ---------- | ----------- | ----------------------------------------------------------------------------------------------------------------------- |
| OpenSteam001/steam-monitor | 2026-05-23 | 2026-09-26 | 50 / 23     | 官方签名仓库，默认分支 `protobuf`；`pattern` 分支最后更新 09-06，`ipc` 分支最后更新 09-26                               |
| hazre/steam-monitor        | 2026-09-12 | 2026-10-21 | 20 / 7      | Python 项目；用 GitHub Actions 自动解析签名，失败时由 bot 开 issue                                                      |
| JYM34/steam-monitor        | 2026-09-24 | 2026-10-22 | 1 / 1       | 提交者为 github-actions bot；文件头标注 `MACHINE-GENERATED by tools/resolve_patterns.py`，IPC 文件标注 `resolve_ipc.py` |

- 三个仓库的 `fork` 字段都是 false，在 GitHub 上彼此没有 fork 关系。
- JYM34 账户也是 09-11 在 #195 报告 `manifest.opensteamtool.com` 返回 403 的账户。
- sergeyrenamed-coder 称 JYM34 与官方使用"相同的"生成脚本〔转述〕。调研者只核实了文件头里的脚本名，没有比对脚本内容。

### 5.2 hazre/steam-monitor 的拦截记录〔核实〕

截至 11-06，hazre 仓库有 7 个 open issue，都由 github-actions bot 创建，标题格式为 "Pattern resolve blocked: stable …, beta …"。

| issue | 创建日期 | stable 哈希（steamclient / steamui）    | beta steamclient 哈希（前 8 位）    |
| ----- | -------- | --------------------------------------- | ----------------------------------- |
| #41   | 10-21    | `253e0a11…` / `cd9226db…`（1792618285） | 无                                  |
| #42   | 10-22    | 同上                                    | `891378c0`（JYM34 记为 1792626604） |
| #43   | 10-23    | 同上                                    | `844afeae`                          |
| #44   | 10-27    | 同上                                    | `676e04ad`                          |
| #45   | 10-29    | 同上                                    | `5aa9bbc2`                          |
| #46   | 11-04    | 同上                                    | `eeae2113`                          |
| #47   | 11-06    | 同上                                    | `b09e96b9`                          |

调研者读了 #41 和 #47 的正文，内容都是："Nothing was published (block-on-failure). Triage the skipped entries by hand, then re-run with workflow_dispatch."

#41 列出的失败项如下。#47 的失败项与之相同，另加一条 beta 失败：`KeyValues_FindOrCreateKey: 0 FindOrCreate cands`。

| 组件                | 条目                                            | 失败原因                          |
| ------------------- | ----------------------------------------------- | --------------------------------- |
| steamclient pattern | `CUtlBufferEnsureCapacity`                      | 0 referencing funcs               |
| steamclient pattern | `OptedInMask`                                   | 0 referencing funcs               |
| steamui pattern     | `GetAppByID`                                    | 0 GetAppByID cands                |
| IPC                 | `IClientUser.GetAppOwnershipTicketExtendedData` | wrapper 0x76aad0 fails hash check |
| IPC                 | `IClientUtils.GetAppID`                         | wrapper 0x769830 fails hash check |

**解读**〔推断〕：

- 在 10-21 至 11-06 期间，stable 构建一直是 1792618285。每次运行都因为同一组 stable 失败项而整体不发布，同一次运行里的新 beta 构建即使解析成功，也被一并拦下。
- issue 页面没有人工参与的记录，仓库最后一次推送停在 10-21。
- 7 个 issue 的创建间隔为 1–6 天，说明这段时间 beta 渠道频繁更新。#45 的创建日期与 1793235415 的构建时间（10-29 00:56）在同一天，两者可能对应同一构建，但没有用哈希核实。
- hazre 仓库创建于 09-12，晚于官方 `pattern` 分支最后一次更新（09-06）。所以 hazre 的拦截只能解释 10-21 以后的断供，不能解释官方仓库为什么停更。

### 5.3 JYM34/steam-monitor 与 hazre 校验结论的分歧〔核实〕

JYM34 于 2026-10-22 13:11 UTC 提交了 `b53e8d8`（"Update patterns (stable 1792618285, beta 1792626604)"）和 `8e428f3`（"Update IPC …"）。仓库此后没有新推送。

hazre 判定失败的 5 个条目，JYM34 都发布了数值：

| 条目                                | hazre 结论                      | JYM34 发布值       | #227 手工提取      |
| ----------------------------------- | ------------------------------- | ------------------ | ------------------ |
| `CUtlBufferEnsureCapacity`          | 0 referencing funcs             | rva `0xCF4760`     | 未提供             |
| `OptedInMask`                       | 0 referencing funcs             | rva `0x6048B0`     | 未提供             |
| steamui `GetAppByID`                | 0 cands                         | rva `0x61B230`     | 未提供             |
| `GetAppOwnershipTicketExtendedData` | wrapper `0x76aad0` 哈希校验失败 | wrapper `0x76AAD0` | wrapper `0x76AAD0` |
| `IClientUtils.GetAppID`             | wrapper `0x769830` 哈希校验失败 | wrapper `0x769830` | wrapper `0x769830` |

- JYM34 发布的两个 IPC wrapper 地址，就是 hazre 校验失败的那两个地址。
- #227（LotusHirasawaSusumu，11-02）给出的 1792618285 IPC 规格共 8 个地址（2 个 vtable、6 个 wrapper），与 JYM34 的 IPC 文件逐项相同。#227 发布于 JYM34 之后，作者称是用 Ghidra 自行提取的，两者是否独立得出无法确认。
- 数值相同有两种可能的解释〔推断〕：一是 hazre 的哈希校验过严，这些地址其实是正确的；二是 JYM34 的工作流没有执行这项校验，发布了未通过检查的数据。调研者无法区分这两种情况。

### 5.4 各构建的签名覆盖（截至 11-06）〔核实〕

| 构建                             | 渠道         | OpenSteam001 | JYM34       | hazre                |
| -------------------------------- | ------------ | ------------ | ----------- | -------------------- |
| 1791918634                       | beta         | 无           | 有          | 未查                 |
| 1792618285                       | stable       | 无           | 有（10-22） | 被拦截               |
| 1792626604                       | beta         | 无           | 有（10-22） | 被拦截               |
| 1793235415                       | beta〔推断〕 | 无           | 无          | 被拦截（可能是 #45） |
| hazre #43–#47 中的其余 beta 构建 | beta         | 无           | 无          | 被拦截               |

除已知来源外，#208 中 matheeshapathirana 为 1791918634 手工提供了签名（§5.6）。

**结论**〔推断〕：stable 渠道用户目前可以从 JYM34 获取当前构建的签名；beta 渠道从 1792626604 之后的构建起，在所有已知来源中都没有签名。stable 渠道下一次更新后，如果 JYM34 不恢复发布、hazre 也没有人工处理，stable 用户同样会断供。

### 5.5 beta 1792626604 的 IPC 方法表与字段偏移变化〔核实，JYM34 文件〕

| 项目                                                        | stable 1792618285  | beta 1792626604    |
| ----------------------------------------------------------- | ------------------ | ------------------ |
| `GetAppOwnershipTicketExtendedData` method_index / funcHash | 105 / `0xC7E71245` | 106 / `0xC7E71244` |
| `RequestEncryptedAppTicket`                                 | 120 / `0x25D6BB1D` | 121 / `0x25D6BB1C` |
| `GetEncryptedAppTicket`                                     | 121 / `0xE0468CB4` | 122 / `0xE0468CB5` |
| `IClientUtils.GetAppID`                                     | 19 / `0x09607EC4`  | 19 / `0x09607EC2`  |
| `GetAppIDForCurrentPipe` 读取的字段偏移                     | `0x10D0`           | `0x1118`           |
| `SendCallbackToPipe` − `GetAppIDForCurrentPipe`             | `0x5110`           | `0x5260`           |

`ProcessPendingLicenseUpdates` 和 `SendCallbackToPipe` 在 beta 中的函数序言字节也有变化。

**影响**〔推断〕：#208 的手工方法依赖两个前提：IPC 的 `funcHash` / `fencepost` / `argc` / `method_index` 在 5 个历史构建中保持不变；`GetAppIDForCurrentPipe` 与 `SendCallbackToPipe` 之间固定相距 0x5110。这两个前提在 beta 1792626604 上都不再成立。这批改动进入 stable 后，沿用旧值再扫描的手工方法会失效，必须重新逆向。OST 源码中 IPC 方法签名的处理方式（例如是否硬编码参数布局）是否也需要修改，本报告没有核查。

### 5.6 社区手工签名：#208 与 #227

**#208**（matheeshapathirana，10-15，针对 1791918634）〔转述，数据已核实存在〕：

- 内容：steamclient pattern 24 条（不含 `RecvPkt`）、steamui pattern 9 条、IPC 规格（`IClientUser` 4 个方法、`IClientUtils` 2 个方法），另附单独的 `RecvPkt` 条目（rva `0x5BC990`）。
- 方法：用上一构建的签名在新 DLL 中扫描，保留唯一匹配的条目，得到 19/25 与 8/10。其余条目分别用模糊扫描、字符串交叉引用加 `.pdata`、相对 `SendCallbackToPipe` 的固定偏移 0x5110 等方法定位。`CUtlMemoryGrow`、`RepeatedFieldUint32_Add` 存在多个字节相同的副本，取离旧位置最近的一个。`GetTopManager` 因当前源码没有引用而省略。
- 作者自述的限制：只在一台 Windows 11 机器上验证过；不是 bot 生成；建议使用者用自己的 DLL 逐条核对。评论末尾注明由 Claude Code 生成。
- 作者指出，issue 正文里给出的 `RecvPkt` 值（`0x5BC610`）对应另一个函数，他本人没有测试。
- `LoadDepotDecryptionKey` 与 `ConfigStoreGetBinary` 两条的 rva 和 sig 完全相同。JYM34 机器生成的 1792618285 文件（均为 `0x5D95E0`）和 1792626604 文件（均为 `0x5EDBF0`）也是如此〔核实〕。因此这不是手工录入错误，更可能是两个名称在当前构建中解析到同一个函数〔推断〕。

**#227**（LotusHirasawaSusumu，11-02，针对 1792618285）：用 Ghidra 提取了 IPC 规格，数值见 §5.3；同时请求协助提取 pattern〔转述〕。

---

## 6. PR 积压（截至 11-06 均为 open）

### 6.1 兼容性修复

| PR   | 提交日期 | 内容                                                                                                                                 | 观察                                                                                                                        |
| ---- | -------- | ------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------- |
| #212 | 10-15    | 修复 `CNetPacket` 偏移                                                                                                               | 附根因分析与两种验证方法〔转述〕                                                                                            |
| #218 | 10-22    | isxlan0 提交，含 3 个提交：修复本地构建；把 `opensteamtool` 替换为第三方接口；修复结构体崩溃并改为本地已有清单文件时优先使用本地文件 | 20 个文件、+752/−721 行；作者在描述中提示使用第三方仓库有风险；11-06 页面显示 Codex 自动审查状态为 Failed（未完成）〔核实〕 |

### 6.2 manifest 获取

| PR   | 提交日期 | 内容                                                         | 观察                           |
| ---- | -------- | ------------------------------------------------------------ | ------------------------------ |
| #201 | 09-13    | `[manifest] url` 支持自定义 URL 模板和 `format` 解析器       | 向后兼容；在加载配置时校验 URL |
| #204 | 09-23    | 多上游自动切换：失败的源冷却 60 秒，单次请求总时长上限 11 秒 | 附带与编码相关的编译修改       |
| #200 | 09-13    | 新增 ManifestDeX 上游                                        | 引入闭源的外部依赖             |

### 6.3 功能与其他修复

| PR   | 提交日期 | 内容                                                                   |
| ---- | -------- | ---------------------------------------------------------------------- |
| #148 | 06-26    | 修复 Denuvo 88500012 的三种失败模式，新能力需在 Lua 中显式开启         |
| #146 | 06-25    | 注入 API 支持多库与条件注入；OnlineFix 支持已拥有的游戏；修复 P2P 游戏 |
| #144 | 06-25    | 向好友广播未拥有游戏的 Rich Presence                                   |
| #121 | 06-15    | ProtectionScan 不再对主程序套用 80 MB 大小下限（对应 #120）            |
| #178 | 08-05    | 递归加载 Lua 子目录，目录监视覆盖整个子树                              |
| #74  | 05-28    | 便携模式，注入器部分未完成                                             |
| #173 | 07-25    | README 补充云存档配置说明                                              |

**评估**〔推断〕：积压的 PR 中有几个附带完整的问题描述、根因分析和验证记录（#212、#121、#201、#204）。但没有维护者合并，这些修复只能在各个 fork 中分别发布。不同 fork 合入的补丁组合不同，用户无法依据统一的版本号判断自己用的是哪一组修复。

---

## 7. 社区替代来源

### 7.1 总览

| 类型             | 来源                                                         | 在主仓库的出处                    | 本报告的覆盖                              |
| ---------------- | ------------------------------------------------------------ | --------------------------------- | ----------------------------------------- |
| 兼容修复 fork    | isxlan0/OpenSteamTool                                        | PR #218；#225、#226 评论（11-04） | Release 页、PR 页（§7.2）                 |
| 功能扩展 fork    | madoiscool/BetterSteamTools                                  | #205 评论（10-14、11-04）         | README、发布说明（§7.3）                  |
| 改品牌的衍生项目 | Contrary-v2/SteamDaddy-Dll；Contrary7/SteamDaddy-Backup      | #223、#225、#226 评论（11-05）    | 元数据、Release、README、6 条提交（§7.4） |
| 被引用的 fork    | huanyuejue/OpenSteamTool、mmxlyo/OpenSteamTool               | SteamDaddy-Dll README Credits     | 未查看                                    |
| 被提及的 fork    | AllRange1337 等                                              | 11-04 抽样评论                    | 未查看                                    |
| 签名来源         | JYM34/steam-monitor、hazre/steam-monitor                     | #226 评论                         | §5                                        |
| 签名来源         | madoiscool/steam-monitor                                     | 11-04 抽样评论                    | 未查看                                    |
| 评论区签名数据   | matheeshapathirana（#208）、LotusHirasawaSusumu（#227）      | 评论正文                          | §5.6                                      |
| 直接分发的压缩包 | SilverIyan（#208）、DaKaRTheWind（#195）                     | 评论附件                          | §7.5                                      |
| manifest 来源    | ManifestDeX、hubcapmanifest、steamtools.site、`20770407.xyz` | #195 评论；SteamDaddy README      | §4.3                                      |

### 7.2 isxlan0/OpenSteamTool〔核实〕

- Release "OpenSteamTool v1.4.8 fix"，发布于 10-22 05:23 UTC。tag `v1.4.8` 指向提交 `7e83f76`，也就是 PR #218 的头提交。
- 资产：`Release.7z`（796 KB，SHA-256 `148bf983…d067`），以及 GitHub 自动生成的源码包。
- 发布说明（中英文）："仅更新使其适配最新版本 steam 使其可用，没有修改其他内容"；"不放心成品二进制请自行下载源代码构建"。
- 改动内容见 §6.1。这个 fork 把签名或清单的获取地址换成了第三方来源，具体是哪个地址，调研者没有在 diff 中核对。

**观察**〔推断〕：在已查看的替代来源中，它是唯一一个 Release 与公开 PR 的提交一一对应、并且明确提示用户自行构建的。二进制和源码是否一致，调研者没有验证。

### 7.3 madoiscool/BetterSteamTools

**定位**〔核实，README〕：README 标题为 "BetterSteamTools - a fork of OpenSteamTools!"。产物文件名仍然是 `OpenSteamTool.dll`、`dwmapi.dll`、`xinput1_4.dll`，配置文件仍然是 `opensteamtool.toml`。

**发布说明摘要**〔转述，各版本 Changes 段〕：

| 版本   | 内容                                                                                                                                                                                                                    |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| v1.0.0 | 新增 "Tokeer" `bst://` URI 处理器：启动时在 HKCU 注册该协议；网站打开 `bst://redeem/…` 时，由 DLL 独立完成兑换码的 HTTP 兑换，把 ticket 写入 Steam 凭据存储，并提示启动游戏。服务端基址由编译选项 `OST_TOKEER_URL` 指定 |
| v1.0.1 | `-realappid` 启动参数：在 `SpawnProcess` 时传递真实 appid，需要和 `-onlinefix` 一起使用                                                                                                                                 |
| v1.0.3 | "update da toml rq"（没有进一步说明）                                                                                                                                                                                   |
| v1.0.4 | 新增编译选项 `OST_ENABLE_UPDATER`，**默认 ON**；设为 OFF 可以编译掉自动更新代码，避免固定版本的构建在运行时被替换                                                                                                       |
| v1.0.5 | 伪造 ownership ticket 时，只采用 SteamID 与当前登录账户一致的 `localconfig` ticket 作为基底，避免 Denuvo 报错 54                                                                                                        |
| v1.0.6 | stats API 端点改为 `stats.luastools.xyz`                                                                                                                                                                                |
| v1.0.7 | 已有 `sha_schema` 时仍然伪装 GetUserStats 的 SteamID，使新增的成就能下发                                                                                                                                                |
| v1.0.8 | 移除家庭共享绕过和对 757 登出消息的屏蔽（§4.4）                                                                                                                                                                         |

**README 中的默认配置**〔核实〕：

- `[manifest] url = "opensteamtool"`，与上游相同，而这个上游已被报告失效。
- `[stats] enable_api = true`，请求发往 `https://stats.luastools.xyz/{appid}`。
- `[donate] enabled = true`：用当前账户为它拥有、且归档方需要的 depot 向 Steam 申请 manifest request code。示例配置里有请求间隔、单周期上限等参数，`max_mints_per_session = 0` 表示不限次数。
- 签名查找顺序描述的仍然是 `OpenSteam001/steam-monitor`（§4.1）。v1.0.3 的 "update da toml" 是否改了这一点，没有核实。

**用户反馈**〔转述〕：#205 中，aryakavin25-byte 于 10-14 推荐并贴出 v1.0.4 的下载链接；ailaoyuan 于 11-04 称换用新文件后恢复正常。

**评估**〔推断〕：BetterSteamTools 在上游之外增加了三个默认生效的对外行为：自动更新（编译默认开启）、`[donate]` 以用户账户向 Steam 申请 code、`bst://` 协议注册。这些行为不仅影响本机，也涉及用户账户与外部服务的交互。用户在 README 中可以找到说明，但需要主动关闭。

### 7.4 Contrary-v2/SteamDaddy-Dll

**仓库与 Release**〔核实〕：

| 字段                  | 值                                                                              |
| --------------------- | ------------------------------------------------------------------------------- |
| 描述                  | "Dlls for SteamDaddy"                                                           |
| GitHub fork 关系      | parent 为 `OpenSteam001/OpenSteamTool`                                          |
| 创建 / 最后推送       | 2026-10-28T12:27:04Z / 2026-11-04T18:39:31Z                                     |
| Star / Fork（11-06）  | 3 / 0                                                                           |
| 许可                  | GPL-3.0                                                                         |
| Issues / Discussions  | 均已关闭（`has_issues: false`）                                                 |
| Release               | 只有 v4.0.6（11-04T18:39:31Z），正文为空                                        |
| 资产（下载量，11-06） | `SteamDaddy.Dll-v4.0.6-Release.zip`，1.63 MB（18）；`…-Debug.zip`，3.94 MB（5） |

版本号 v4.0.6 与上游 1.4.8 不连续，公开记录中找不到 4.x 之前的版本。仓库没有可见的 CI 构建记录，所以无法从公开信息确认二进制是由公开源码编译的。

**已读的提交**〔核实〕：作者与提交者均为 Contrary-v2，均未签名。

| 时间（UTC）    | 提交      | 说明                                                                                          |
| -------------- | --------- | --------------------------------------------------------------------------------------------- |
| 10-28 21:30:11 | `b1041f2` | 远端 TOML 镜像轮询总时长上限设为 30 秒，避免 InitThread 长时间阻塞                            |
| 11-04 18:01:50 | `9aad2d6` | 换用 `SteamDaddy.rc` 资源定义，构建信息改名，目标改为 `SteamDaddy.dll`                        |
| 11-04 18:02:04 | `bde3abd` | 代理加载器（version.dll / dwmapi.dll）启动加固                                                |
| 11-04 18:02:21 | `886d275` | 从 `localconfig.vdf` 解析 "donor" AppTicket；改进 Denuvo 检测；IPC 拦截只作用于配置过的 appid |
| 11-04 18:04:21 | `d43abed` | 家庭共享：恢复真实 lender ID，修复"在别处登录"导致的掉线                                      |
| 11-04 18:04:41 | `b2b9e87` | 允许刷新成就 schema；stats 端点双路容错                                                       |

11-04 的 5 条提交集中在 2 分 51 秒内。这种分布更像是在本地整理后一次性推送，不能据此还原实际的开发过程〔推断〕。

**README 要点**〔核实〕：

- 项目改名为 "SteamDaddy"，配置文件为 `steamdaddy.toml`，找不到时回退读取 `opensteamtool.toml`。README 把本仓库定位为底层 DLL，"主项目"是另一个账户名下的 `Contrary7/SteamDaddy-Backup`，声称提供 GUI、一键解锁、ContraryCDN API 和 Millennium UI 集成。
- Credits 列出的上游：OpenSteam001/OpenSteamTool、huanyuejue/OpenSteamTool、mmxlyo/OpenSteamTool、Selectively11/CloudRedirect。
- 默认 manifest 上游按优先级为：`20770407.xyz` → `steamapi.993499094.xyz`（SDM）→ `manifest.manifestdex.com` → `gmrc.wudrm.com`（**明文 HTTP**）→ `manifest.steam.run` → `manifest.opensteamtool.com`。前两个域名在主仓库的讨论中从未出现。
- 成就 / 统计 API 为 `stats.luastools.xyz`，默认开启。签名元数据经 `gh-proxy.com` 代理获取，默认 `order = "gh-proxy-first"`。
- 推荐的安装方式是以管理员权限执行 `irm https://raw.githubusercontent.com/Contrary7/SteamDaddy-Backup/main/install_b.ps1 | iex`。脚本内容没有查看。
- `extract_tickets.exe` 的 `--online` 模式声称不需要本地 Steam 客户端，可以 "with QR-code or saved credentials" 直接连接 Valve CM 服务器。

**与 BetterSteamTools 的对应关系**〔核实为文本对应；来源关系为推断〕：

| SteamDaddy                                                                           | BetterSteamTools                                                                 |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------- |
| `b2b9e87`：stats 端点；成就 schema 刷新                                              | v1.0.6 端点改为 `stats.luastools.xyz`；v1.0.7 已有 `sha_schema` 时仍伪装 SteamID |
| `886d275`：从 `localconfig.vdf` 取 donor AppTicket                                   | v1.0.5 从 `localconfig` 选取伪造 ticket 的基底                                   |
| README：SteamDRMP "off-by-four" 漏洞、`extract_tickets`、Lua 目录 `config/stplug-in` | README 中有相同的表述、工具与目录                                                |

SteamDaddy 的 Credits 没有列出 BetterSteamTools。这些功能也可能来自 Credits 中列出的 huanyuejue 或 mmxlyo 的 fork，或者来自双方共同的上游，调研者无法判断代码的来源方向。家庭共享一项双方说法相反，见 §4.4。

**推广行为**〔核实〕：11-05 14:15:50 至 14:16:25 UTC，Contrary-v2 在 35 秒内向 #226、#225、#223 发出 4 条正文相同的评论（#223 两条），称该仓库 "updated with the latest forks and PRs from the developers and is actively maintained"，"Hopefully, this release fixes all your pattern-related and achievement issues"。评论没有说明修复了哪个 Steam 构建，也没有说明是否处理了 `CNetPacket` 偏移。明细见附录 C。

**账户**〔核实〕：Contrary-v2，显示名 "Contrary"，bio 为 "hmm,, here we go again!"，注册于 2025-11-10，公开仓库 4 个，粉丝 4 人。账户名带 `-v2`，README 又引用 `Contrary7` 账户，两者是否属于同一人没有核实。

### 7.5 评论区直接分发的压缩包〔转述〕

| 议题 | 发布者 / 日期       | 内容                                                                                                   | 后续反馈                                                                  |
| ---- | ------------------- | ------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| #208 | SilverIyan，10-15   | `OpenSteamTool-Binaries.zip`，说明为"解压到 Steam 根目录"；被问到家庭共享联机时回答 "I don't know bro" | hg94692208、FizarFR 于 10-29 确认可用，FizarFR 的环境是 stable 1792618285 |
| #195 | DaKaRTheWind，09-18 | `Steam_Fix_Manifests.zip`，内含会覆盖文件的 `install.bat`                                              | 反应 ❤️ 2、👍 1、👎 1                                                     |

这两个压缩包都没有构建说明或对应的源码提交，内容无法从公开信息核实。

### 7.6 替代来源的信任边界对比

| 项目                       | isxlan0                                             | BetterSteamTools                                   | SteamDaddy-Dll                                          |
| -------------------------- | --------------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------- |
| Release 与公开提交对应     | 是（`7e83f76`）                                     | 未核实                                             | 无法确认（无 CI，Release 正文为空）                     |
| 可在仓库内反馈问题         | 未核实                                              | 未核实                                             | 否（Issues 已关闭）                                     |
| 是否处理 `CNetPacket` 偏移 | 是（PR #218 描述）                                  | 未核实                                             | 未说明                                                  |
| 默认 manifest 上游         | 第三方接口（具体地址未核对）                        | `opensteamtool`（已报告失效）                      | `20770407.xyz` 等未知域名                               |
| 默认开启的对外行为         | 未发现                                              | 自动更新、`[donate]`、`bst://` 协议注册、stats API | stats API、`gh-proxy.com` 代理                          |
| 安装方式                   | 未核实（发布 `Release.7z`，未见安装说明）           | 手动复制 3 个 DLL（发布说明）                      | 推荐以管理员权限执行远程脚本                            |
| 涉及账户凭据的工具         | 未发现                                              | `extract_tickets`（需本地已登录的 Steam）          | `extract_tickets --online`（声称可用已保存凭据直连 CM） |
| 推广方式                   | 作者在 #225、#226 各回复一次，附 PR 与 Release 链接 | 由第三方用户在 #205 推荐                           | 35 秒内 4 条相同评论                                    |

表中"未核实"表示调研者没有读到相关信息，不表示该项不存在。

**评估**〔推断〕：OST 的 DLL 注入 `steam.exe` 进程，可以接触 Steam 的会话和凭据存储（见架构分析 §4 `SteamCredentialStore`）。签名 TOML 中的 RVA 会直接用来安装钩子。所以，选用替代来源时，不只是选择一组功能，也是在决定信任谁的二进制、谁的签名和谁的远端服务。这三个来源中，SteamDaddy-Dll 引入的外部信任对象最多，也最难核实。调研者不建议把它视为 OST 的继任版本。

---

## 8. 其他未决议题（11-04 抽样）

| 主题      | 议题                             | 状态                                            |
| --------- | -------------------------------- | ----------------------------------------------- |
| Denuvo    | #139、#162；关闭未合并的 PR #128 | 特定游戏报 88500012，SteamID 偏移修复没有被采纳 |
| OnlineFix | #52、#181、#185、#158            | 邀请好友时崩溃、大厅为空、不支持自定义 appid    |
| 云存档    | #96、#113、#116                  | CloudRedirect 兼容需求与云同步报错              |
| 跨平台    | #142、#166、#165                 | Linux、macOS 均未实现，README 标为预留          |
| 体验      | #160、#152、#127、#124           | 首次启动约 1 分钟后入库游戏才显示、启动卡顿等   |
| 安全      | #222                             | 下载的构建被 Windows Defender 报毒              |
| 版本      | #195（ImmobilePlushie，09-19）   | 询问是否会发布 1.4.9，没有回复                  |

---

## 9. 对下游集成方的风险评估

| 风险项            | 当前状态                                              | 影响   | 说明                                                                  |
| ----------------- | ----------------------------------------------------- | ------ | --------------------------------------------------------------------- |
| 上游代码维护      | 停滞约 4 个月                                         | 高     | 缺陷与兼容问题没有官方修复渠道                                        |
| 签名元数据供应    | 官方停更；自动解析被拦截；只剩一个镜像覆盖当前 stable | 高     | stable 下一次更新后可能全面断供                                       |
| 签名正确性        | 镜像与自动校验结论不一致                              | 中至高 | 错误的 RVA 至少会导致崩溃；恶意构造的签名能造成什么后果，本报告未评估 |
| 结构体布局兼容    | 已发生一次破坏性变化                                  | 高     | 只能改源码并重新编译                                                  |
| IPC 方法表        | beta 已变化                                           | 高     | 进入 stable 后，社区手工方法失效                                      |
| manifest 自动获取 | 服务端只给拥有者下发 code                             | 高     | 可用上游依赖拥有者账户；替代服务闭源或不稳定                          |
| 家庭共享          | 社区报告服务端强制许可锁                              | 中     | 客户端绕过方案被一个 fork 主动移除                                    |
| 替代来源可信度    | 来源分散，信任边界差异大                              | 中至高 | 见 §7.6                                                               |
| 许可              | GPL-3.0                                               | 中     | 分发修改版需公开源码                                                  |

**应对原则**：

1. **锁定版本，自行构建**。从可审查的源码提交构建，记录 commit 和已合入的补丁，不使用他人分发的二进制或压缩包。
2. **签名来源可配置、可校验**。不硬编码单一签名源，允许用户指定镜像，并在加载前检查每个条目的 `rva` 处字节是否与 `sig` 一致。
3. **默认关闭对外行为**。自动更新、code 贡献、统计 API、协议注册这类会与外部服务交互的功能，应当默认关闭，并在界面上说明。
4. **把失效当作常态设计**。检测签名缺失、结构体不兼容、IPC 方法表变化等情况，向用户给出明确、可操作的提示，避免静默失败或崩溃。
5. **manifest 获取与核心功能解耦**。自动获取只作为可选能力，保证手动导入流程始终可用。

---

## 10. 对三份底稿的更正

| #   | 底稿位置          | 原表述                                                                                      | 更正                                                                                                                                                             | 依据       |
| --- | ----------------- | ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 1   | 第一版 §4.1 评估  | 故障的关键在发布环节，单一自动化流程停了就没有备用渠道                                      | 断点至少有两层：官方仓库 09-26 后停更，原因不明；hazre 的自动解析从 10-21 起被 block-on-failure 策略拦截。JYM34 镜像只覆盖到 10-22                               | §5.1、§5.2 |
| 2   | 第一版附录 A      | 10-14 "Steam 构建 1791918634 发布"                                                          | 该构建的时间是 10-13 19:10 UTC，属于 beta 渠道；10-14 是首次被报告的日期                                                                                         | §4.1       |
| 3   | 第一版 §4.1 表    | 1793235415 与 1791918634、1792618285 并列，没有区分渠道                                     | 1791918634 为 beta〔核实〕；1792618285 为 stable〔核实〕；1793235415 为 beta〔推断〕                                                                             | §4.1       |
| 4   | 补充篇一 §1、§4.1 | "签名的获取地址包括 `generator.ryuu.lol`"，并把它作为故障 ① 的第二层原因                    | 这只是一位评论者的说法〔转述〕。BetterSteamTools README 记载的查找顺序是 GitHub raw → jsDelivr → 本地缓存，没有出现该域名。它属于哪个版本的获取链路，没有查明    | §4.1       |
| 5   | 补充篇一 §4.2     | `LoadDepotDecryptionKey` 与 `ConfigStoreGetBinary` 的 rva 和 sig 完全相同，"可能是录入错误" | JYM34 机器生成的两个构建的文件中，这两项也相同，因此不是录入错误，更可能是同一函数〔推断〕                                                                       | §5.6       |
| 6   | 补充篇一 §4.3     | #226 的 JYM34 方案"只补签名，不解决故障 ②"                                                  | 更准确的说法是：JYM34 文件包含 `RecvPkt`，在未修补的 1.4.8 上可能导致启动崩溃〔推断〕；只适用于已修复偏移的构建                                                  | §4.2       |
| 7   | 补充篇二 §1       | hazre 的拦截是"签名断供的直接原因"                                                          | 只能解释 10-21 之后的断供；官方仓库停更更早，hazre 仓库本身也创建于官方 `pattern` 分支最后更新之后                                                               | §5.2       |
| 8   | 补充篇二 §1       | SteamDaddy 的 4 项改动分别对应 BetterSteamTools v1.0.1、v1.0.5、v1.0.6、v1.0.7              | 在已读的 SteamDaddy 提交中，没有找到与 v1.0.1（`-realappid`）对应的内容。可以确认的对应是 v1.0.5、v1.0.6、v1.0.7，以及 README 中的若干表述；代码来源方向无法判断 | §7.4       |
| 9   | 补充篇二 §2.2     | beta 构建"大约每 2–5 天"推送一次                                                            | hazre issue 的创建间隔为 1–6 天；issue 创建日期也不等同于构建时间                                                                                                | §5.2       |
| 10  | 补充篇二 §2.4     | beta 渠道"从 10-29 起"在所有已知来源中都没有签名                                            | 应为"1792626604（10-21 23:50）之后的 beta 构建"；hazre #43（10-23）对应的 beta 就已经没有签名                                                                    | §5.4       |
| 11  | 第一版 §4.3       | OST 内置的三个上游"全部失效"                                                                | 证据等级降为〔转述〕；调研者没有逐一测试                                                                                                                         | §4.3       |

---

## 11. 结论

OST 的设计把易变部分（函数位置、IPC 规格）移到了远端元数据中。在 2026-05 至 06 的活跃期，这种设计支撑了快速迭代。但项目的可用性一直依赖几个外部条件：维护者持续发版，签名有人持续生成和发布，manifest 上游持续可用。截至 2026-11-06，这些条件都已失效或只剩部分可用。

- 主仓库已停更约 4 个月，兼容性补丁无法合并。
- 签名供应从"官方仓库"转到了"自动解析仓库加社区镜像"。前者被自身的校验策略拦截，后者在 10-22 之后停止更新，而且与前者的校验结论存在分歧。
- Steam 方面同时出现了客户端结构变化（`CNetPacket`、IPC 方法表）和服务端策略变化（manifest code、家庭共享许可）。前一类可以靠社区逆向补救；后一类使任何仍然可用的方案都要依赖拥有者账户或闭源服务。
- 社区的技术投入仍然存在（#208、#212、#227），但成果分散在评论、PR 和多个 fork 中。各替代来源的信任边界差异很大，用户获得一个可信构建的成本明显上升。

本报告不预测项目的后续走向。下列信号可用于判断状况是否改变：

- 维护者是否回复，或合并 #212 这类兼容性修复；
- `OpenSteam001/steam-monitor` 是否恢复发布；
- hazre 的拦截项是否得到人工处理，JYM34 是否恢复更新；
- Steam stable 渠道下一次更新是否带入 beta 1792626604 中的 IPC 变化；
- 是否有人对 SteamDaddy-Dll v4.0.6 等二进制做独立分析；
- 是否出现一个被社区普遍接受、Release 与源码对应、并持续维护的 fork。

---

## 附录 A：时间线（UTC）

| 日期       | 事件                                                                                           | 证据                             |
| ---------- | ---------------------------------------------------------------------------------------------- | -------------------------------- |
| 2026-05-01 | 首个 issue #1                                                                                  | 核实                             |
| 2026-05-04 | 发布 v1.4.0                                                                                    | 核实                             |
| 2026-05-23 | `OpenSteam001/steam-monitor` 创建                                                              | 核实                             |
| 2026-05-25 | 合并 PR #55，签名改为从远端 tracker 加载                                                       | 核实                             |
| 2026-05-28 | 发布 v1.4.6；合并 PR #64                                                                       | 核实                             |
| 2026-05-30 | 合并 PR #80，`opensteamtool` manifest 上游设为默认                                             | 核实                             |
| 2026-06-13 | 发布 1.4.8（最后一个 Release）；合并 PR #109                                                   | 核实                             |
| 2026-06-20 | 合并 PR #125（stats API）                                                                      | 核实                             |
| 2026-06-25 | 合并 PR #138（CloudRedirect 支持）                                                             | 核实                             |
| 2026-07-06 | 合并 PR #153（最后一次合并与推送）                                                             | 核实                             |
| 2026-09-05 | Steam 构建 1788652215                                                                          | 核实（BuildID 换算）             |
| 2026-09-06 | 官方 `pattern` 分支最后一次更新                                                                | 核实                             |
| 2026-09-09 | manifest request code 开始失效（#195）                                                         | 转述                             |
| 2026-09-11 | JYM34 报告 `manifest.opensteamtool.com` 返回 403                                               | 转述                             |
| 2026-09-12 | `hazre/steam-monitor` 创建                                                                     | 核实                             |
| 2026-09-13 | PR #200（ManifestDeX）、#201 提交                                                              | 核实                             |
| 2026-09-23 | PR #204 提交                                                                                   | 核实                             |
| 2026-09-24 | `JYM34/steam-monitor` 创建                                                                     | 核实                             |
| 2026-09-26 | 官方 `ipc` 分支最后一次更新                                                                    | 核实                             |
| 2026-10-13 | beta 构建 1791918634                                                                           | 核实（JYM34 记录）               |
| 2026-10-14 | #208、#209 报告 1791918634 缺签名并崩溃                                                        | 核实                             |
| 2026-10-15 | PR #212 提交；#208 出现手工签名                                                                | 核实                             |
| 2026-10-21 | stable 构建 1792618285、beta 1792626604；hazre #41 首次拦截；hazre 最后一次推送                | 核实                             |
| 2026-10-22 | PR #218 提交；isxlan0 发布 v1.4.8 fix；JYM34 发布 1792618285 / 1792626604 签名（最后一次推送） | 核实                             |
| 2026-10-28 | SteamDaddy-Dll 仓库创建                                                                        | 核实                             |
| 2026-10-29 | 构建 1793235415；hazre #45                                                                     | 核实（BuildID 换算、issue 日期） |
| 2026-11-02 | #226 给出 JYM34 镜像方案；#227 手工提取 IPC                                                    | 核实                             |
| 2026-11-04 | SteamDaddy-Dll 发布 v4.0.6；isxlan0 在 #225、#226 推荐自己的 fork；hazre #46                   | 核实                             |
| 2026-11-05 | Contrary-v2 在 35 秒内发出 4 条推广评论                                                        | 核实                             |
| 2026-11-06 | hazre #47，stable 仍为 1792618285                                                              | 核实                             |

## 附录 B：数据口径

- issue 与 PR 依据 GitHub API 是否带 `pull_request` 字段区分。
- "关闭未合并"指 state 为 closed 且 `merged_at` 为空的 PR。
- 按月统计依据 UTC 时间的 `created_at`、`closed_at`、`merged_at`；"当月新增且至今 open"以 11-04 为截止。
- 维护者活动依据 `author_association` 字段和 issue events API。events API 只返回最近 200 条事件，结论只适用于可见范围。
- 构建时间由 BuildID 按 Unix 时间戳换算；渠道依据 JYM34 README 与 hazre issue 标题。
- 文中的哈希只给出前 8 位或首尾片段，完整值见原始来源。

## 附录 C：Contrary-v2 的评论

| 时间（UTC）         | 议题 | 评论 ID    | 反应 |
| ------------------- | ---- | ---------- | ---- |
| 2026-11-05 14:15:50 | #226 | 6660224388 | ❤️ 1 |
| 2026-11-05 14:16:04 | #225 | 6660229557 | 无   |
| 2026-11-05 14:16:09 | #223 | 6660231629 | 无   |
| 2026-11-05 14:16:25 | #223 | 6660237553 | 无   |

4 条评论正文完全相同。

## 附录 D：主要参考

**主仓库**

- https://github.com/OpenSteam001/OpenSteamTool
- #195：https://github.com/OpenSteam001/OpenSteamTool/issues/195
- #197：https://github.com/OpenSteam001/OpenSteamTool/issues/197
- #205：https://github.com/OpenSteam001/OpenSteamTool/issues/205
- #208 手工签名：https://github.com/OpenSteam001/OpenSteamTool/issues/208#issuecomment-6185068668
- #223：https://github.com/OpenSteam001/OpenSteamTool/issues/223
- #224：https://github.com/OpenSteam001/OpenSteamTool/issues/224
- #225：https://github.com/OpenSteam001/OpenSteamTool/issues/225
- #226 镜像方案：https://github.com/OpenSteam001/OpenSteamTool/issues/226#issuecomment-6572179902
- #227：https://github.com/OpenSteam001/OpenSteamTool/issues/227
- PR #200 / #201 / #204 / #212 / #218：https://github.com/OpenSteam001/OpenSteamTool/pulls

**签名仓库**

- https://github.com/OpenSteam001/steam-monitor
- https://github.com/hazre/steam-monitor/issues
- https://github.com/hazre/steam-monitor/issues/41
- https://github.com/hazre/steam-monitor/issues/47
- https://github.com/JYM34/steam-monitor/commit/b53e8d85cbb23586a645e28b3a40188a3c850d6e
- https://github.com/JYM34/steam-monitor/commit/8e428f3562b37c401b5fe52921beb59aa57d8c1d

**社区 fork**

- https://github.com/isxlan0/OpenSteamTool/releases/tag/v1.4.8
- https://github.com/madoiscool/BetterSteamTools
- https://github.com/madoiscool/BetterSteamTools/releases
- https://github.com/Contrary-v2/SteamDaddy-Dll
- https://github.com/Contrary-v2/SteamDaddy-Dll/releases/tag/v4.0.6
