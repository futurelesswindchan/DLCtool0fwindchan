# 开发进度追踪

> 每次开发结束时更新本文件，下次开发接力时快速定位当前进度。
>
> 最后更新：2026-07-30

---

## 当前阶段：封测前置项施工中（分支 `feat/ui-v2`）

目标已从「打 v2.0.0」调整为「先打 `2.0.0-rc.1` 做封闭测试」。
理由：v2 自设计至今整月闭门开发，功能类缺陷已趋于收敛，而体验类问题
开发者本人极难发现——07-30 暴露的四个缺陷中有三个属此类。

| #   | 事项                                     | 性质         |
| :-- | :--------------------------------------- | :----------- |
| 1   | 试下载对比表（缺陷③+议题⑤）              | 最大块，未开工 |
| 2   | 前端视觉细化（配色、间距、动效、空状态） | 未开工       |
| 3   | 教程补 M 站 API key 申请指引             | 未开工       |

### ✅ 封测前置项：诊断链路与构建身份（2026-07-30）

四个缺陷中的三个 + 两项封测基础设施。全量测试由 82 增至 **93 条 PASS**。

**缺陷① `OpenDataDir` 完全失效**（点击无反应，控制台 `Invalid URL shell
metacharacters not allowed`）。根因在上游：Wails v2.11 给 `BrowserOpenURL`
加了 `internal/frontend/utils/urlValidator.go`，两道关卡**各自独立地**否决
本地路径——`scheme == ""`（Windows 裸路径无 scheme）与反斜杠命中 shell
危险字符黑名单。改用 `exec.Command("explorer", dir)`，两处要点：

- **不能据退出码判成败**。explorer 成功打开窗口时同样返回退出码 1，
  故只 `Start()` 不 `Wait()`
- **路径作为独立参数传入**，不拼进命令字符串。用户的 Steam 路径与主目录
  都可能含空格，字符串拼接在含引号或 `&` 的路径下会变成注入面

`app.go` 中「Wails 的 BrowserOpenURL 对本地路径同样有效」这句注释在 v2.11
下已不成立，一并删除。`OpenURL` 走同一函数但只放行 http(s) 且 URL 无反斜杠，
不受影响，实机已验证可用。

**新增诊断包导出**（`ExportDiagnostics`）。这不是便利功能而是安全出口：
数据目录里的 `config.json` 明文存着用户自行申请的 API 凭据，而报障场合
最常见的一句话是「把日志发上来」。若不提供更省事的脱敏通道，用户就会
手动翻目录、把原始配置发到群里——凭据随即归群友所有。

脱敏采**白名单式投影**而非黑名单擦除：独立声明 `maskedConfig` /
`maskedRepoSource` 只含已知安全字段，而非拷贝 `AppConfig` 再置空 Token。
差别在日后新增字段时显现——黑名单方式下新增的敏感字段会默认泄露，
且没有任何编译期或运行期信号提醒补规则。

凭据只描述存在性（`TokenState`），**连 `maskSecret` 的前 8 位也不给**。
日志里出现前 8 位可接受（用于比对是否同一密钥），但诊断包流传范围更广，
且此处没有任何需要比对密钥的场景。

包内不含 `history.json` 与 `packages/`：前者是用户装过哪些游戏，后者是清单
内容本体，都与「工具为何出错」无关，而清单文件流入群聊会使性质从工具测试
变成内容分发。这条边界是刻意划下的。

**新增构建身份注入**（`GetBuildInfo`）。封测期同一版本号会被反复重新构建
（修一个 bug 打一次包，但 tag 未动），此时版本号无法确定用户手里是哪次构建，
而报障最需要确定的恰是这一点——同样的现象在修复前后的包上表现相同。

哈希打在**文件名与程序内部两处**。文件名便于分发时肉眼识别，但会在用户
重命名、解压后只留 exe、或转发他人时丢失，而报障往往发生在下载数日之后。

`appDirty` 标记比哈希本身更重要：带 dirty 的包所对应的代码**在仓库里根本
不存在**，据其哈希 checkout 会得到另一份代码。不标出来则排障时会照着错误
的代码找原因，且完全察觉不到。测试特意锁定「仅注入值为 `true` 时成立」——
写成「非空即真」会把每个正常包都标成已修改，使警告彻底失效。

新增 `build/release.ps1`：采集 git 信息、经 ldflags 注入、产出
`kazeusa-<版本>-<哈希>.exe`。版本号在前、哈希在后，因群聊中人们按版本号
口头指代。

**缺陷② 搜索改按钮驱动**。原为输入即搜（400ms 防抖）。改动理由是网络现实
而非交互偏好：商店接口在国内常以 `wsarecv: An existing connection was
forcibly closed` 中断，输入即搜会把「打一个词」放大成多次失败——实测输入
`monster` 期间日志出现 5 次搜索失败。用户看到连续 5 条报错，合理的结论是
「工具坏了」，而实际只是自己还没打完字。

原记录担忧的「AppID 直查需与搜索入口时机一致」**不存在**：纯数字判断在后端
`SearchGames` 内部，前端只有一个入口。同时补了「清空」按钮——自动搜索取消后，
清空输入不再自动收起结果，需要显式出口。

**缺陷④ 彻底卸载后留在原页**。原实现跳回库列表，问题在于卸载常是「装错了源、
想换一个」的中间步骤而非终点，跳走后用户需重新搜索、重进详情页，且刚看过的
源信息全部丢失。改为 `resetToUninstalled()` 复位为状态 A，**并重新 lookup**——
状态 A 的界面依赖 `sources`，不重查会显示一个空页面，与「卸载把源也弄坏了」
难以区分。

**「三源」旧措辞清理**。界面 4 处仍写「三个清单源」，源已扩至 7 个。同时补上
缺陷③ 的轻量前置：明确告知「收录只表示该源存在这个游戏的文件，不代表内容
完整」，并说明拿到 DLC 少应换源而非工具出错。完整的试下载对比表另行设计。

`reacquire()` 取首位源的写法留 `FIXME` 标注，附 Kingdom Rush Vengeance 反例
（首位源 2 DLC，快照源 4 DLC），待对比表落地后改为用户选择。

**顺带完成 `app.go` 拆分的一小块**：诊断类方法迁出至 `app_diag.go`，
`app.go` 由 1218 行降至 1192 行。

### ✅ Frameless 启用与三个元信息方法（2026-07-30）

两件卡在 v2.0.0 门口的事一并做完。测试新增 `app_meta_test.go`（3 个测试函数、
含 12 个子用例），全量 `go test -v` 现有 **70 条 PASS**（含父项；叶子用例 58 个）。

> 口径说明：此前文档记的「35 / 50 个用例」未注明统计方式，与本次的
> 「PASS 行数」不可直接相减。今后统一记 `go test -count=1 -v .` 的 PASS 行数，
> 便于复核。

**Frameless 启用**。此前一直推迟，卡在「Aero Snap 是否保留」这一未验证的疑问上。
本轮直接读 Wails v2.11 源码定论——`startDrag()` 实现为
`ReleaseCapture()` + `PostMessage(WM_NCLBUTTONDOWN, HTCAPTION)`，即把拖动过程
整个交还系统的标题栏循环，**Aero Snap 与边缘缩放均零成本继承**。
Snap Layouts 确认不可用（需 `WM_NCHITTEST` 返回 `HTMAXBUTTON`，自绘按钮在
WebView 客户区内触达不到），接受。

落地三处：`main.go` / `wails.json` 开 `Frameless`、`TopBar.vue` 自绘标题栏与窗口
控制、新增 `composables/useWindowControls.ts` 承担控制与状态。

施工中处理的两个易漏点：

- **最大化状态以 `resize` 事件同步**，而非点击时翻转本地布尔值。Aero Snap 与
  `Win+↑` 同样改变窗口状态却不产生点击事件，只翻本地值必然与真实形态错位。
  同步调用加 120ms 防抖——`WindowIsMaximised` 是跨边界异步调用，缩放期间不防抖
  会打出上百次 IPC
- **`no-drag` 用宽泛选择器兜住**（`.topbar button, a, input, select`）而非逐个类名
  列举。按钮点击被识别为拖拽起始是 frameless 最高频的翻车点，宽泛选择器使日后
  新增控件默认即被覆盖，不依赖施工者记得补规则

**三个元信息方法**（新增 `app_meta.go`，不入已逾 1170 行的 `app.go`）：

| 方法 | 要点 |
| :--- | :--- |
| `GetAppVersion` | 经 `-ldflags -X main.appVersion=` 注入，默认 `dev` |
| `OpenURL` | **仅放行 http 与 https** |
| `CheckUpdate` | 查 `/releases/latest`，返回 `error` 而非 `OperationResult` |
| `GetReleasePageURL` | 发布页地址，检查失败时的兜底跳转 |

`OpenURL` 的 scheme 限制不是过度防御：`BrowserOpenURL` 在 Windows 上最终交给
`ShellExecute`，会执行 `file:` 指向的程序、按注册表处理任意自定义协议。当前前端
只传常量链接，但这个方法一旦存在便是通用出口。

`CheckUpdate` 失败按「暂时查不到」就地提示而非弹错——国内访问 `api.github.com`
经常直接超时，这是常态而非故障，弹错会让用户误以为工具坏了。

原设计的 `SkipVersion` **取消**：其前提是启动时自动弹更新提示，而现行设计中检查
更新只由用户主动点击，没有需要跳过的东西。

**待实机验证**：frameless 下的拖动、Aero Snap、边缘缩放、双击最大化四项行为，
以及三个方法的真实网络路径。

### ✅ 清单源扩容与探测策略修正（2026-07-30）

源从 3 个增至 7 个（1 个认证型 + 6 个免凭据），并修正三处在扩容后才会
显现的缺陷。测试从 35 个增至 50 个用例。

**新增四个源**：

| 源 | 形态 | 分支数（实测 07-29） |
| :--- | :--- | :--- |
| `bingyu50/ManifestAutoUpdate` | MAU 形态 | 13131 |
| `hansaes/ManifestAutoUpdate` | MAU 形态 | 6336 |
| `tymolu233/ManifestAutoUpdate` | MAU 形态 | 3140 |
| `SSMGAlt/ManifestHub2` | lua 形态 | 62288 |

MAU 形态的三个 fork **零解析改动即可接入**——解析器当初为兼容
`Key.vdf` / `config.vdf` 两种命名而写成「只认扩展名与内容结构，不认文件名」，
此处收获红利。

**排序依据是单游戏完整度而非分支数**。ARK(2399830) 实测对照：

| 源 | DLC | setManifestid |
| :--- | :--- | :--- |
| Hubcap | 19 | 13 |
| MAU | 4 | 1 |
| ManifestHub 快照 | 1 | 3 |

快照源广度是 MAU 的 15 倍，单游戏覆盖反而更少。故 MAU 系仍居前，快照源
置末位兜底冷门游戏。

**为接入快照源而修的两处解析差异**：

- `setManifestid` 只有两个参数（无 fileSize）。原实现用 `L.CheckNumber(3)` 取值，
  参数缺失时抛 Lua error 使整个解析失败——若不修，新源接入后 100% 报错
- 主游戏的 `addappid` 不带密钥，使「第一个带 key 的调用即主游戏」这一判据
  把紧随其后的 Depot 误判为主游戏

**扩容暴露的三个隐患**（详见 DECISIONS-2.md）：

- `probe` 的部分失败会永久排除源。已改为三态，仅 404 可排除
- 7 个源无限制并发探测会触发限流。已加并发上限 4
- 老用户配置中已有的源列表不会刷新，新源静默不生效。已加 `mergeNewBuiltinSources`

**顺带修正**：`parseMSiteTime` 原按 UTC 解析该站不带时区的时间戳，实际为美东
时间，使到期时刻偏早 4~5 小时。改用 `America/New_York` 并内嵌 `time/tzdata`
（Windows 无系统 zoneinfo，不内嵌会静默退回 UTC）。

**尚未实机验证**：新增四源全部只经单元测试，未跑过真实下载。

### ✅ ⑧ 前端界面完工（2026-07-29）

按架构文档的分层落地，`feat/ui-v2` 分支：

| 层             | 产出                                                              |
| :------------- | :---------------------------------------------------------------- |
| `api/`         | 24 个方法，`unwrap` 将 `OperationResult` 归一化为异常语义         |
| `router/`      | 5 条路由，hash 模式，`/setup` 守卫                                |
| `stores/`      | `env` / `config` / `library` / `ui`，均为后端状态的镜像           |
| `composables/` | `useToast` / `useConfirm`（Promise 化） / `useDlcSelection`       |
| `components/`  | `TopBar` / `GameCard` / `DlcList` / `ConfirmDialog` / `ToastHost` |
| `views/`       | `Search` / `Game` / `Library` / `Settings` / `Setup`              |

依赖仅增三个：`vue-router` / `pinia` / `@vueuse/core`。样式纯 CSS 变量，
组件全部手写。删除测试台遗留的 `PackagePanel.vue`。

`vue-tsc` 升至 2.x——1.8.22 无法给 TypeScript 5.9 打补丁，类型检查跑不起来。

**施工中规避的两个静默缺陷**（均为肉眼审查难发现的类型）：

- `useDlcSelection` 若在 `watch` 回调里构造，其内部的 `onUnmounted` 注册不到组件
  实例上，切页时最后一次勾选会静默丢失。改为在 setup 顶层接收 `Ref` 一次性构造
- 重置勾选的 `watch` 必须用 `flush: 'sync'`。默认 `pre` 时机是异步的，赋值 pkg 后
  紧接着调 `selectAll()` 时回调会晚于其执行并清空集合，导致 800ms 后落盘一个
  **空列表**——等于什么都没装

`Frameless` 刻意推迟：Aero Snap 保留情况未验证，`no-drag` 遗漏是已知的高频翻车点。

### ✅ 清单留存与搜索过滤（2026-07-29）

实机反馈驱动的两项功能补齐，四个 commit：

**`packages/` 落盘**（`package_store.go`，新增）。架构文档自 07-27 即将其定为
「卸载与增装 DLC 的数据来源」，但此前从未实现——用户重启应用便无法调整已入库
游戏的勾选，只能重新联网下载。现 `InstallDLCs` 时写入、`RemoveDLCs` 时清理、
`GetPackage` 读回。落盘格式为 `StoredPackage`，含 `SavedAt` 与 `Source` 元信息。

**不做过期判定，不引入版本号**。清单源的探索仍在进行，未来出现携带更多元数据的
源时扩展字段即可，不必推翻存储结构。

**搜索只保留本体**。按 `appdetails` 的 `type` 字段过滤，`GameDetail` 新增 `Type`
与 `IsFree`。实测约束三条：批量查询不可用（`appids=a,b,c` 返回空）故并发逐条查；
「序章」的 `type` 也是 `game` 需第二级判据；超时须部分生效否则慢网下过滤完全失效。

**修同游戏多文件名冲突**。MAU 路径与 Lua 路径拿到的游戏名不同（中文名清洗后落为
`unknown`，英文名正常），换源重获会留下两份都声明同一 AppID 的文件——正是 07-28
记录的那类不可观测冲突。现 `Deploy` 前清理自身产物的同 AppID 旧文件。

**修「全不选」绕过 depot 确认**。逐个 `toggle` 会拦、批量清空却放行，一次点击
即可取消全部带独立 Depot 的 DLC，Steam 随即删除已下载内容。

**补齐操作说明**。实机反馈集中在「不知道点了之后发生了什么」：`⚑` 的下载含义、
DLC 生效的四步链路、外部清单为何同样在生效、清空记录只清账本不删文件。

测试增至 35 个用例（新增 `package_store_test.go` 5 个、`store_filter_test.go` 12 个）。

### 📌 已知残留（不阻塞发布）

| 项目                       | 说明                                                         |
| :------------------------- | :----------------------------------------------------------- |
| 中文搜索命中率低           | `欧洲卡车模拟` 返回 0 条，`storesearch` 自身局限，非过滤所致 |
| riftbreaker 仍漏一至两条   | 疑与 `Type` 字段引入前的旧缓存有关，影响极小                 |
| `packages/` 丢失时的降级读 | 设计已定（读已部署 Lua 还原 AppID 集合），未实现，属功能退化 |
| `gofmt -l` 列出全部文件    | 既有的 CRLF/LF 差异，不影响功能                              |
| 主游戏无密钥是否需补密钥   | MAU 路径有 `fallbackMainKey`，快照源路径没有。两处不一致，取舍依据未经实测，见 DECISIONS-2.md |
| 新增四源未实机验证         | 仅经单元测试，未跑过真实下载。验证要点见下                   |

**新源实机验证要点**（下次开工优先做）：

1. 搜一个 MAU 有而 Hubcap 没有的游戏，确认新 fork 能命中
2. 搜一个只有 `ManifestHub 快照` 收录的冷门游戏，确认两参数 lua 能走通到部署
3. 看日志中「排除 N 个明确未收录的源」是否符合预期
4. 老配置迁移：启动后源列表应变为 7 个，且原有 Token 与停用状态保持不变
5. 七源并发探测的实际耗时，确认并发上限 4 未使等待过长

### 🗂️ 项目结构议题（已讨论，暂不执行）

26 个 Go 文件散在根目录，单一 `package main`。评估结论：

- **Go 与 Wails 都不阻止分包**，但把 DTO 挪出 `main` 会使前端类型引用从
  `main.GamePackage` 变为 `types.GamePackage`，`models.ts` 的 namespace 也随之变化，
  24 处 api 方法需同步修改
- 现有耦合（`types.go` / `logger.go` / `constants.go` / `fileutil.go` 被广泛引用，
  `repo_package.go` 反向依赖 `store`）会使拆包立刻面对循环依赖与归属重判
- 现实工作量 1~2 天，收益仅为「文件列表更整齐」；构建速度、运行时性能、可测试性
  均不改变（单包内测试私有函数反而更方便）

**决定**：v2.0.0 发布后再做，届时功能已冻结、有干净 baseline 可对照。
在此之前继续靠文件名前缀分组（`store_*` / `repo_*` / `deployer_*` / `detector_*`）。

**更值得先做的是拆 `app.go`**（已超 1170 行，混了配置、部署编排、在线获取、
对账扫描四类职责）。按职责拆为 `app_config.go` / `app_package.go` / `app_online.go`
/ `app_scan.go` 属同包内改动，Wails 完全感知不到，零风险。

---

## 历史进度

### ✅ 地基与核心（2026-07-26 ~ 07-27）

**地基四件套**

- [x] **① config.go** — 配置持久化（`AppConfig` / `RepoSource` / `ConfigManager`，读-改-写全在锁内，JSON 原子落盘）
- [x] **② logger.go** — 日志迁至数据目录、5MB × 3 份轮转、并发安全
- [x] **③ deployer.go + deployer_ost.go** — 部署器：生成 Lua 清单并原子写入 `<Steam>/config/lua/`
- [x] **④ detector.go + detector_ost.go** — 环境检测：查三个 DLL，采用 ready/missing/unknown 三态

**核心与整合**

- [x] **⑥ history.go** — 安装历史，按 mainAppID 去重覆盖
- [x] **⑦ app.go 重构** — 注入四模块，移除 `App.steamPath` 双源真相
- [x] **⑨ 旧代码清理** — 删除 `vdf_helper.go` 与 `andygrunwald/vdf` 依赖，`steam.go` 由约 620 行精简至 215 行
- [x] **前端清空** — 移除 v1.4 四个组件，`style.css` 重写为设计令牌系统

**基建与治理**

- [x] **fileutil.go**（新增）— `atomicWriteFile` 原子写入、`webviewDataDir`、`cleanStaleTempDirs`、`fileExists`
- [x] **lua_match.go**（新增）— 承接 `luaContainsAppID`
- [x] **文件卫生治理** — WebView2 数据目录收拢、`OnShutdown` 生命周期补全、临时目录兜底清理、背景色三处对齐
- [x] **命名统一** — exe → `kazeusa.exe`，标题 → `风兔盒 - 请问您今天要来点DLC吗？`
- [x] 架构讨论落档 5 条新决策（事件总线 / 责任链 / 缓存分层 / 纯 CSS / 文件落点）

**实机联调（2026-07-27）**

- [x] **L1.5 测试台界面** — 已于 ⑧ 完工后废弃，`PackagePanel.vue` 删除
- [x] **Lua 生成器修正** — 补齐主游戏密钥与 `setManifestid`，DLC 独立 Depot 改双行注册，
      Depots 段跳过 DLC 自有 Depot
- [x] **文件名兼容性修正** — `sanitizeFileName` 丢弃非 ASCII 字符并折叠连续下划线
- [x] **依赖清理** — 补跑 `go mod tidy`，移除 `andygrunwald/vdf`
- [x] 实机验证 4 条决策落档

### ✅ ⑤ 完成（2026-07-28）

三条获取路径均已打通，且产出一致的 `GamePackage`：

- [x] **store_client.go** — Steam 商店元数据。`Search` / `Detail`，详情缓存 7 天。
      纯数字输入直接按 AppID 查详情，跳过搜索接口
- [x] **repo_client.go** — 多源查找与下载。`Lookup` 并发 HEAD 检测、`Fetch` 四级镜像回退。
      自动模式先检测再下载，实测 24.4 秒降至 5.5 秒
- [x] **repo_package.go** — MAU 形态解析器。从 `Key.vdf` + `config.json` + manifest
      文件名构建 `GamePackage`，含独立 Depot DLC 的按需补齐（并发限 4 路）
- [x] **msite_client.go** — Hubcap Manifest 接入。`status` / `manifest` / `user-stats`
      三端点，未配置凭据时整条链路静默跳过

`app.go` 新增 8 个前端方法：`SearchGames` / `GetGameDetail` / `LookupRepos` /
`DownloadFromRepo` / `SetRepoToken` / `GetMSiteStats`，及内部的格式分派与名称回填。

**实机验证结果**（样本 ARK 2399830）：

| 路径 | DLC 数 | setManifestid | 耗时   |
| :--- | :----- | :------------ | :----- |
| M 站 | 19     | 13            | 4.5 秒 |
| MAU  | 4      | 1             | 3.9 秒 |

三源实测状态：ManifestHub 已被清空（仅剩 `main` 分支）故默认停用；MAU 可用但
收录不全；M 站数据完整但需用户自备凭据。详见 DECISIONS.md 的 07-28 条目。

### 📋 待开始

- [ ] 接入更多低门槛清单源（下一个话题的专项）
- [ ] 正式 UI 设计与打磨，含 `Frameless` 实现
- [ ] `CheckUpdate` / `OpenURL` / `GetAppVersion` 三个方法

### ✅ 收尾施工与缺陷修正（2026-07-29）

两个 commit：`chore` 清理遗留待办与数据目录迁移，`fix` 修正清单定位缺陷。
`go build` / `go vet` 在正式与 `-tags dev` 两种模式下均通过，`go test` 全绿。

**数据目录迁至 exe 同级**。`appDataDir` 是唯一收敛点，七处调用皆只依赖其返回值，
故此项为单函数改动。判据改用构建标签而非环境变量，理由见 DECISIONS.md 的
2026-07-29 条。可写性以探针文件验证，不止于 `MkdirAll` 成功。

**清单定位改按内容匹配**。原实现靠 `_<AppID>.lua` 后缀识别，漏判所有外部文件。
新增三个方法：

| 方法                    | 职责                               |
| :---------------------- | :--------------------------------- |
| `findLuaFilesDeclaring` | 返回声明某 AppID 的全部文件名      |
| `externalDeclarations`  | 滤出其中非本工具命名格式者         |
| `ScanDeployed`          | 全目录对账，产出 `[]DeployedEntry` |

部署与卸载前均检测冲突。卸载时若存在外部声明则返回**失败**并列出文件名，
不再谎报「已移除」。

**建立测试基线**。新增 `lua_match_test.go` 与 `app_scan_test.go`，18 个用例，
为项目首批测试。测试当即捕获一处正则缺陷：`[^-]` 在 Go 正则中会匹配换行符，
贪婪量词跨行吞噬使 `FindAll` 只返回最后一处匹配。该缺陷靠肉眼审查难以发现。

### ✅ 设计定稿（2026-07-27 第二轮）

调研竞品 Fluent-Install 后确定在线仓库方案，⑤ 的阻塞解除。落档 8 条决策：

| 议题     | 结论                                            |
| :------- | :---------------------------------------------- |
| 仓库源   | 聚合三个 GitHub 分支型社区源，不自建            |
| 认证     | 走 codeload，零 API 配额，**不引入 token 配置** |
| 元数据   | 只用 Steam 官方 storesearch / appdetails / CDN  |
| 入口     | 搜索为唯一在线入口，移除 `FetchRepoList`        |
| 解析结果 | 落盘 `packages/{appID}.json`，压缩包用后即删    |
| 本地导入 | 保留，降为备用入口（唯一离线路径）              |
| 版本判定 | manifest ID 不可比大小，改按需检查而非实时监测  |
| 勾选交互 | 勾选即生效 + 800ms 防抖聚合                     |

### ✅ 设计定稿（2026-07-28 第三轮）

再落档 5 条决策，覆盖数据目录、更新检查与设置页：

| 议题     | 结论                                                           |
| :------- | :------------------------------------------------------------- |
| 数据目录 | 改为 exe 同级 `.kazeusa/`，开发构建走 home（判据为构建标签）   |
| 常量清理 | 删除 8 个 v1.4 遗留常量（vdf / ST / depotcache / killSteam）   |
| 检查更新 | 302 重定向探测免配额，**不实现自更新**，仅提示 + 开浏览器      |
| 设置页   | 四项必备 + 主题壁纸，清单源只读展示                            |
| 边界场景 | 路径识别失败、lua 目录缺失、Steam 未运行、密钥截断、写入串行化 |

发布流程约定：**蓝奏云上传先行，GitHub Release 作为最后一步**。Release 一发布
在线客户端即开始提示新版，若蓝奏云未就绪用户会撞上失效链接。

### ✅ 设计定稿（2026-07-28 第四轮 · UI 与前端架构）

⑧ 的设计取向敲定，落档 4 条决策：

| 议题     | 结论                                                         |
| :------- | :----------------------------------------------------------- |
| 界面取向 | 参照 PCL2，`Frameless` 自绘标题栏融合导航与窗口控制          |
| 动效     | transform 驱动、≤300ms、`--ease-enter` 过冲取 1.4            |
| 反馈体系 | 四形态并存，以「是否需要用户决策」划分而非按重要程度         |
| 前端架构 | 唯一真相在 Go，store 只作镜像；hash 路由；确认弹窗走 Promise |

反馈体系的四种形态：模态弹窗（不可逆决策）、Toast（无需决策）、
原地状态指示（持续过程）、顶部横幅（不紧急通知）。后两者使用频次最高。

需注意的实现约束：

- 取消勾选带独立 Depot 的 DLC 时 Steam 可能删除本地内容，须二次确认
- 全部取消勾选仍要保留主游戏的 `addappid` 与密钥，否则已装本体的游戏会崩
- 替换清单时按 AppID 保留原勾选状态
- `packages/` 丢失时降级读已部署 Lua，仅能还原 AppID 集合
- 版本号必须编译期注入（`-ldflags -X main.appVersion=`），不得硬编码
- 数据目录选址判据为构建标签，不得依赖路径特征推断
- exe 目录不可写时须回退 home，否则程序在只读位置完全不可用
- frameless 标题栏内所有可交互元素须显式 `--wails-draggable: no-drag`
- 双击标题栏最大化需自行绑定，系统行为已丧失
- 监听 Wails 事件的组件卸载时必须 `EventsOff`，否则重复注册
- `prefers-reduced-motion` 媒体查询必须实现
- 警告与错误类 Toast 不自动消失，且须同时写入日志

### 🔬 待实机验证

- [x] ~~同一 AppID 被多个 Lua 文件重复声明时 OST 的行为~~ **已于 2026-07-28 验证完毕**，
      结论见下方「多文件共存的实测结论」一节，落档 4 条决策
- [ ] **frameless 下拖至屏幕边缘的 Aero Snap 是否保留**。Win11 的 Snap Layouts
      浮层确定不可用（属原生标题栏特权），但边缘吸附未经确认

### ✅ 多文件共存的实测结论（2026-07-28）

以 ARK（2399830）为样本，在 `config/lua/` 同时放置两份均声明主游戏、
DLC 互不重叠的文件，配合 OST Debug 版的 trace 日志完成验证。

| 问题                        | 结论                                                            |
| :-------------------------- | :-------------------------------------------------------------- |
| 多文件如何加载              | **并集去重**。`adding 8 apps` 精确等于两文件 AppID 去重后的数量 |
| 是否存在文件级优先权        | **不存在**。全局 map 的同 key 后写覆盖，Parse 顺序不可控        |
| 删除其一后共享 AppID 的命运 | **许可证保留**。refCount 由 2 减 1 未归零，游戏仍在 Steam 库中  |
| 存活文件的声明是否恢复      | **不恢复**。`processing 0 additions`，旧值残留至重启 Steam      |
| 密钥冲突是否可观测          | **完全不可观测**。无警告日志，且 OST 从不输出密钥值             |

**对实现的三条硬约束**：

1. 卸载前必须扫描部署目录中其他 Lua 是否声明同一 mainAppID。若有，不得报
   「已卸载」，须如实告知游戏可能仍留在 Steam 库中，并指出具体文件名
2. 靠命名前缀为盒子产物抢占加载优先级的思路无效，此方向已排除
3. 盒子是唯一能在事前发现密钥冲突的环节——落盘之后线索链完全断裂，
   症状将表现为「一切正常，直到下载时解密失败」

**顺带发现**：SteamTools 与 OST 共用 `dwmapi.dll` / `xinput1_4.dll` 两个劫持入口，
社区流传的 ST 卸载步骤会连带删除 OST 的加载链。教程须明确卸载与安装的顺序。

### 📌 用户体验缺口

主线路径已顺，异常路径的处置在 ⑧ 阶段大部分补齐。剩余状态：

| 缺口                         | 状态                                                |
| :--------------------------- | :-------------------------------------------------- |
| 首次启动时注入器未安装       | ✅ `/setup` 三步引导页已实现，路由守卫拦截          |
| OST 目录中存在未被记录的 Lua | ✅ 已安装页的「其他清单」区呈现，并说明为何不代清理 |
| 三源均未收录                 | ✅ 就近引导至本地导入，不再是死路                   |
| 下载无进展反馈               | ✅ 显示当前尝试的源                                 |
| M 站凭据将到期 / 额度耗尽    | ✅ 顶部横幅，附「前往设置」入口                     |
| ST 残留与 OST 并存           | ⚠️ 引导页已给出文字警示，自动检测未实现             |
| 视觉打磨                     | ⚠️ 当前为功能可用的骨架，待 UI 专项                 |

**第二项的定义与处理方向**（2026-07-28 修正）：

此项指 `<Steam>/config/lua/` 中出现了盒子未记录的 `.lua` 文件，**来源不限**——
可能是用户手动放置、另一份 kazeusa 副本写入（数据目录随 exe 走之后，换个位置
放 exe 即是一份独立的数据目录）、其他工具产生，或旧版本遗留。**盒子无法区分
来源，也不需要区分**，只需如实呈现为「外部清单」。

此项先前被表述为「已部署但不在历史中的游戏」并归类为信任问题，该表述有误：
它隐含「这是我们漏记的自己的东西」，从而暗示应当去认领、去修复记录。实则这些
是来源不明的第三方文件，不该认领，只该如实呈现。

经 2026-07-28 的实机验证，此项的危害远超界面显示不准——外部文件与盒子的部署
共享 AppID 时，OST 的引用计数会使盒子的卸载**仅部分生效**，而界面报告「已卸载」。
处理方向：

- 启动时扫描监控目录，**按文件内容而非文件名**解析各文件声明的 AppID
- 与历史记录对账，多出者标记为「外部清单」，可查看与删除但不可改勾选
  （无 `packages/` 数据）
- 部署与卸载前均须检查目标 AppID 是否被其他文件同时声明，据实提示

**第五项**为 2026-07-28 新增。v1.4 时代的 SteamTools 痕迹写在 Steam 自身，
与 OST 同处一层，二者并存时同一 AppID 会被两套机制各自注入。检测属状态提示，
符合职责边界；代为清理则越界。

### ✅ 实机验证结果（2026-07-27）

使用 OST **Release 1.4.8** 完成首轮端到端验证，测试样本为 ARK: Survival Ascended
（本体未安装，19 DLC）与 Street Fighter 6（本体已安装，12 DLC）。

| 验证项                                         | 结果                              |
| :--------------------------------------------- | :-------------------------------- |
| `~/.kazeusa/` 目录与 `config.json` 落盘        | ✅ 通过                           |
| WebView2 数据目录收拢至 `~/.kazeusa/webview2/` | ✅ 通过，确认生成 `EBWebView/`    |
| 环境检测识别 OST 三个 DLL                      | ✅ 通过，三态判定正确             |
| 清单包解析 → 部署 → 历史记录闭环               | ✅ 通过                           |
| 部署产物被 OST 解析并刷新 Steam 库             | ✅ 通过（修复两处缺陷后）         |
| 卸载后 Steam 库界面即时更新                    | ⚠️ 部分——本体已安装时需重启 Steam |

**排障期间修复的两处缺陷**（详见 DECISIONS.md 2026-07-27 条）：

1. 生成脚本丢失主游戏密钥与全部 `setManifestid`，且 DLC 自有 Depot 被重复输出
2. 文件名保留非 ASCII 字符，导致 OST 在 `ParseFile` 前 `abort()`

第 2 项才是 Steam 闪退的真正原因。第 1 项虽为真缺陷，修正后症状并未改变——
这段弯路的教训已记入 DECISIONS.md。

**调试方法备忘**：OST 的 Release 构建不输出日志，需换用仓库提供的 Debug 版 DLL，
并在 Steam 根目录放置 `opensteamtool.toml` 写入 `[log] level = "trace"`，
日志落于 `<Steam>/opensteamtool/*.log`。其中 `package.log` 与 `steamui.log`
分别覆盖许可证注入与界面刷新两条链路，是排查此类问题的主要依据。

### ⚠️ 已知限制（非缺陷，不修）

- **卸载后界面残留**：许可证层面移除彻底，但 OST 会跳过 `IsOwned()` 为真的条目
  以保护正版内容。表现为条目不消失，或条目仍在但不再显示独立 DLC 体积。
  重启 Steam 即恢复正常。归属注入器层，仅在文案中提示。
- **DLC 日期显示异常**：Steam 界面显示的日期与清单包内任何时间戳均不吻合，
  而 OST 注入的 `PurchasedTime` 经日志确认无误。成因未明，不影响功能，不再追查。

### ⚠️ 遗留待办

- [x] ~~在线仓库源的具体地址仍未确定~~ 已于 2026-07-27 决定，07-28 实测校正
- [x] ~~`readDeployedLua` 按文件名后缀定位~~ 已于 2026-07-29 改为按内容匹配，
      并新增 `findLuaFilesDeclaring` / `externalDeclarations` / `ScanDeployed`
- [x] ~~数据目录迁至 exe 同级~~ 已于 2026-07-29 落地。**判据改为构建标签 `dev`
      而非 `KAZEUSA_DEV` 环境变量**——Wails v2.11 的 `wails.json` 无注入环境变量
      的字段，原决策前提不成立。同时取消了 `KAZEUSA_DATA` 那一层
- [x] ~~`constants.go` 清理 8 个 v1.4 遗留常量~~ 经核查早已完成，现存均为正当引用
- [x] ~~日志输出中的 depot 密钥截断为前 8 位~~ 已新增 `maskSecret`。现存日志本无
      输出密钥之处，故此项为预防性约定
- [x] ~~`GameRecord` 增补 `Source` 字段~~ 已完成。连带为 `GamePackage` 加同名字段，
      使 `InstallDLCs` 签名不变；`RepoClient.Fetch` 改为一并返回实际命中的源名
- [x] ~~`types.go` 中 `DepotInfo` / `DLCInfo` 的 config.vdf 过期注释~~ 已清理

### 📌 尚未讨论的议题

- [ ] **新压缩包如何覆盖旧版本**。已提出但未展开。`GameRecord.Source` 已按
      「旧文件缺失该字段时视为空值」实现，不做版本号
- [ ] 教程草稿待补 OST 蓝奏云链接、界面截图、在线仓库说明、M 站 API key
      申请指引（ST 残留清理清单与卸载顺序已于 2026-07-29 补入）
- [ ] **ST 残留检测**。决策已定「可检测并挂横幅告知，不得代劳清理」，
      实现顺延至 ⑧ 与环境横幅一并处理
- [ ] `GameRecord.Source` 字段尚未落地。现已有三个来源（M 站 / MAU / 本地导入），
      该字段的价值已实际显现——检查更新时需据此定位回源
- [ ] `RemoveDLCs` 的返回文案需调整，告知本体已安装时可能需重启 Steam
- [ ] 本机的 v1.4 残留需手动删除：`%APPDATA%\DLC入库工具.exe`、`%APPDATA%\DLC入库工具v1.4.exe`
- [ ] `.gitignore` 忽略了 `frontend/dist/`，但 `main.go` 有 `//go:embed all:frontend/dist`——他人 clone 后无法直接 `go build`，待前端阶段处理

---

### ✅ 已完成

- [x] v1.4 全部代码审查（Go 后端 + Vue3 前端）
- [x] 竞品分析（Fluent Install — Python + PyQt6）
- [x] OpenSteamTool 文档研究与适配方案设计
- [x] 架构白皮书定稿（三层解耦 / 模块划分 / 接口契约）
- [x] 决策日志建立
- [x] B 站大更新预告动态撰写
- [x] **OST 源码完整架构分析**（产出：`OST_Architecture_Analysis.md`）
- [x] **OST 热重载机制研究**：事件驱动 + 500ms 防抖，文件落盘即生效
- [x] **OST SteamUI 刷新机制研究**：安装/卸载后库自动更新，无需重启
- [x] **OST ManifestClient 研究**：全自动三级回退，kazeusa 无需参与

### ✅ 待研究项——全部已确认

- [x] OST 的 Lua 目录：默认 `<Steam>/config/lua/`，可通过 toml `[lua].paths` 添加额外目录
- [x] OST 自动下载 manifest：全自动，拦截网络包 + 上游 API 获取码
- [x] addappid 第二参数：**无语义**。区分 App 与 Depot 靠两次独立调用，
      2026-07-27 实测确认，README 的「0:Depot 1:App」与实现不符
- [x] OST 环境检测指标：检查 Steam 根目录是否存在 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll`
- [x] M 站 Lua 与 OST 格式差异：无差异，但生成脚本须遵守格式契约（见 ARCHITECTURE.md 5.1）
- [x] 在线仓库的具体源：聚合 `SteamAutoCracks/ManifestHub`、`Auiowu/ManifestAutoUpdate`、
      `Satisl/MAU` 三个分支型社区源，走 codeload 免配额下载

---

## 版本里程碑

| 版本       | 目标                                  | 状态          |
| ---------- | ------------------------------------- | ------------- |
| v2.0-alpha | 地基完成：配置/日志/部署器/检测器     | ✅            |
| v2.0-beta  | 核心功能：在线仓库 + 历史管理 + 新 UI | 🔜 设计已定稿 |
| v2.0-rc    | 全功能可用，博客教程完成              | 📋            |
| v2.0       | 正式发布                              | 📋            |

---

## OST 源码研究关键发现摘要

> 详见 `docs/research/OST_Architecture_Analysis.md`

| 发现                               | 对 kazeusa 的影响                             |
| ---------------------------------- | --------------------------------------------- |
| 热重载 500ms 防抖，事件驱动        | deployer 写文件即可，推荐 tmp+rename 原子写入 |
| 安装后 Steam 库自动刷新            | 安装文案可以直接写"已添加到库"                |
| 卸载时 `IsOwned()` 条目被跳过      | 卸载文案需提示本体已安装时可能要重启 Steam    |
| Manifest 全自动获取                | 不需要 manifest fallback 功能                 |
| .lua 文件 mtime 作为 PurchasedTime | 不要修改文件时间戳，保持当前时间即可          |
| OST 函数名大小写无关               | 生成的 Lua 用全小写即可                       |
| 路径经宽字符 ↔ UTF-8 转换          | 文件名必须为纯 ASCII，否则 OST `abort()`      |
