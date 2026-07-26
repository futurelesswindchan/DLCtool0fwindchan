# 决策日志

> 记录每次重大架构/技术决策。
> 格式：日期 + 决策标题 + 背景 + 结论

---

## 2026-07-07: 项目定位为"MOD 盒子管理器"，与底层工具完全解耦

**背景**：v1.4 硬耦合 SteamTools（直接写 config.vdf + Steamtools.lua + 复制 manifest），导致底层工具停更时整个工具链断裂。

**结论**：

- 盒子不写 config.vdf
- 盒子不写注入器自身的配置文件
- 盒子只负责把 Lua 清单文件放到注入器的监控目录
- 注入器环境仅做"检测 + 状态提示"，不帮安装不帮修复（方案 A）
- 用户通过博客上的教程自行安装注入器

---

## 2026-07-07: 全面放弃 SteamTools，转向 OpenSteamTool

**背景**：SteamTools 自 2026.01 起停更半年，与 V 社夏促后的新版 Steam 客户端冲突，导致 Steam 崩溃重启、游戏闪退。论坛已炸，也许已经遗憾离场awa。

**结论**：

- v2.0 不保留 ST 兼容
- 适配对象为 OpenSteamTool（活跃维护的开源替代品）
- 旧用户自行卸载 ST + 清理残留（有教程指引）

---

## 2026-07-07: 在线仓库拉取为 v2.0 杀手级新功能

**背景**：用户获取清单包需要翻墙访问特定网站，门槛高、体验差，大量用户因此找不到包而私信求助。

**结论**：

- v2.0 必须支持在线仓库拉取
- 仓库源方案待定（需研究可用的 GitHub 清单仓库）
- 设计上支持多仓库源 + 镜像回退

---

## 2026-07-07: 使用 JSON 文件做持久化，不引入 SQLite

**背景**：SQLite 引入 CGO 依赖，会让 Wails 交叉编译变复杂；本项目数据量小（配置 + 几十条历史记录）。

**结论**：

- 用户配置：`~/.kazeusa/config.json`
- 安装历史：`~/.kazeusa/history.json`
- 写入策略：先写 .tmp 再 rename（原子写入）
- 零外部依赖，保持打包清爽

---

## 2026-07-07: 每游戏一个独立 Lua 文件

**背景**：OST 支持 `config/lua/` 目录下多文件热重载。相比 ST 的单文件追加，独立文件便于管理和清理。

**结论**：

- 每个游戏生成独立文件，命名格式：`<GameName>_<MainAppID>.lua`
- 安装 = 放文件，卸载 = 删文件
- 干净利落，不影响其他游戏

---

## 2026-07-07: v1.4 → v2.0 不提供自动迁移

**背景**：ST 有自己的卸载程序，config.vdf 删了 Steam 会重新生成，自动迁移逻辑复杂但收益低。

**结论**：

- 博客出详细迁移教程
- 用户手动完成：卸载 ST → 删配置 → 装 OST → 装新版盒子
- v2.0 代码中不保留任何 ST 相关的 patch/unpatch 逻辑

---

## 2026-07-07: Lua VM 解析器作为核心资产保留

**背景**：lua_parser.go 使用嵌入式 Lua 解释器执行清单脚本，天然免疫格式变化，是相比竞品（正则解析）的技术护城河。

**结论**：

- `lua_parser.go` 原封保留
- 无论清单包来源（M 站/GitHub 仓库/其他），只要是合法 Lua 就能解析
- 可能需要扩展支持 OST 新增的函数（addtoken / setAppTicket 等），注册为空操作回调即可

---

## 2026-07-07: 接力开发方案

**背景**：项目越来越大却无应有的文档，使得开发工作较为困难（其实是风酱自己都忘了昨天写了啥awa）

**结论**：

- 维护 `docs/` 目录作为文档目录
- 初次维护阅读 `ARCHITECTURE.md` + `PROGRESS.md`
- 每次重大决策追加到 `DECISIONS.md`
- 每次开发进展更新 `PROGRESS.md`

---

## 2026-07-08: OST 源码研究确认——kazeusa 完全不需要参与 manifest 流程

**背景**：研究 OST 源码时确认了 ManifestClient 的三级回退机制（Lua 自定义 → 内置 Provider API），以及 Hooks_NetPacket 自动注入访问令牌的流程。

**结论**：

- kazeusa 不需要提供 manifest 下载/部署/fallback 功能
- kazeusa 不需要管 depotcache 目录
- 用户只要把 .lua 放对位置，OST 全自动搞定后续
- 如果上游 API 全部不可达，那是 OST/网络问题，不是 kazeusa 的责任

---

## 2026-07-08: 部署策略——推荐 tmp+rename 原子写入

**背景**：OST 的 LuaFileWatcher 使用 ReadDirectoryChangesW 事件驱动 + 500ms 防抖窗口。如果先创建空文件再写内容，可能被中途触发解析。

**结论**：

- deployer_ost.go 写入 .lua 时，先写 `<filename>.tmp` 再 `os.Rename` 为 `.lua`
- 这样 OST 只收到一次 RenamedNewName 事件，拿到完整内容
- 不需要加锁、不需要通知 OST、不需要等待确认

---

## 2026-07-08: OST 环境检测方案确定

**背景**：通过 OST 源码确认了加载链：dwmapi.dll / xinput1_4.dll 作为 DLL 劫持入口，加载 OpenSteamTool.dll。

**结论**：

- detector_ost.go 检测逻辑：在 Steam 根目录检查以下三个文件是否存在
  - `dwmapi.dll`（入口代理 A）
  - `xinput1_4.dll`（入口代理 B）
  - `OpenSteamTool.dll`（核心 DLL）
- 三者全部存在 → Available
- 缺少任一 → Not Available，Message 提示缺少哪个文件
- 不检测版本、不检测 pattern 缓存、不检测 toml 配置（那些是 OST 自己的事）

---

## 2026-07-26: 模块间用接口解耦，不自建事件总线

**背景**：考虑引入事件总线做模块间松耦合通信。

**结论**：不引入。后端仅 6 个模块，调用关系是一条无环无交叉的直线，事件总线的收益不足以抵偿代价——可追溯性归零（无法通过 IDE 跳转找到调用方）、类型安全丧失（`any` + 断言把编译期检查推迟到运行时）、错误传播需另造回执机制。

- 模块间解耦交由 `Deployer` / `Detector` 接口承担，这已是恰当的粒度
- **唯一使用事件的场景是跨进程边界**：Go → Vue 的单向进度通知，直接用 Wails 内置的 `runtime.EventsEmit`，不自造第三套总线
- 判据：模块数量增长到出现环形依赖时再重新评估

---

## 2026-07-26: 多源回退用 slice 顺序遍历实现，暂不引入策略模式

**背景**：考虑用责任链模式 + 策略模式组合来组织资源管理。

**结论**：责任链的**精神**采用，**样板代码**不采用；策略模式暂缓。

- `repo_client.go` 的多源回退用 `for range sources` + 首个成功者返回实现，约十行，不需要 `setNext()` 链表结构与 interface 层级
- 策略模式暂不引入：当前每个操作都只有一种合法算法（解析只有 Lua VM、部署只有 tmp+rename、检测只有查三个 DLL），找不到「多算法可互换」的落点。过早抽象比重复代码更贵
- `Deployer` / `Detector` 接口本质上已是策略模式（OST 是一种实现，未来其他注入器是另一种），无需再叠一层

---

## 2026-07-26: 在线仓库采用分层缓存，健康度探测改为手动触发

**背景**：v2.0 的多源下载需要决定「智能」到什么程度。

**结论**：分三层，其中两层必做、一层延后且改为手动。

| 层级          | 内容                                               | 时机                                          |
| :------------ | :------------------------------------------------- | :-------------------------------------------- |
| L1 顺序回退   | 按源列表逐个尝试，失败换下一个                     | v2.0-beta 必做                                |
| L2 健康度探测 | 并发测速后按延迟排序                               | v2.0-rc，且**由用户点击「测速」按钮手动触发** |
| L3 本地缓存   | 索引缓存 1 小时过期；清单包永久缓存 + 提供清空按钮 | v2.0-beta 必做                                |

L2 不做自动后台探测的理由：启动时并发请求全部源，在防火墙或网络受限环境下会让用户看到一片失败标记，制造无谓的焦虑。

仓库源支持内置默认 + 用户自定义，可增删改与「重置为默认」，配置载体为 `config.json` 的 `repoSources` 字段。

---

## 2026-07-26: 前端不引入 UI 组件库，纯 CSS 优先

**背景**：Wails 受 WebView2 约束，无法像 WPF 那样做重度定制渲染，故希望尽量用 CSS 承担表现层。

**结论**：采纳纯 CSS 优先，并明确三条性能纪律。

- **不引入 UI 组件库**（Element Plus / Naive UI 等）：本工具 UI 高度定制（卡片 / 拖拽区 / 列表），可复用的仅按钮与弹窗，而组件库会把 `frontend/dist` 从 ~100KB 推到 1MB+
- **DLC 列表用 `content-visibility: auto`** 让浏览器跳过屏幕外元素渲染，一行 CSS 替代整个虚拟滚动库
- **动画只改 `transform` 与 `opacity`**，二者走 GPU 合成层；禁止对 `width` / `height` / `top` 做过渡（触发重排）
- 主题切换走 CSS 变量 + `:root[data-theme]`，切换时只改一个 attribute，不引发 Vue 重渲染

补充认知修正：渲染瓶颈并非「WebView2 弱于 WPF」——其底层即 Chromium，CSS 动画性能良好；真正的风险点是 Vue 响应式开销与重排重绘。

---

## 2026-07-26: 全部落盘文件收拢至 ~/.kazeusa/，统一命名为 kazeusa

**背景**：v1.4 未设置 `WebviewUserDataPath`，WebView2 遂在 `%APPDATA%\<exe文件名>\` 下自建目录（连 `.exe` 后缀一并带上）。exe 改名后旧目录永久残留且无人清理，实测已产生 `DLC入库工具.exe` 与 `DLC入库工具v1.4.exe` 两坨垃圾。同时排查出日志写在 `%TEMP%`（会被系统清理抹掉，用户报障时无线索）、日志无轮转、`Logger.Close()` 从未被调用、临时解压目录无兜底清理等一系列问题。

**结论**：

命名统一（exe 名务必纯 ASCII，中文 exe 名会让 WebView2 目录与崩溃转储路径都带上中文）：

| 项目              | 取值                                         |
| :---------------- | :------------------------------------------- |
| go module         | `dlctool`（保持不变，改动需重写全部 import） |
| exe 文件名        | `kazeusa.exe`                                |
| 窗口标题 / 显示名 | `风兔盒 - 请问您今天要来点DLC吗？`           |
| 数据目录          | `~/.kazeusa/`                                |

允许的落盘位置**穷举如下**，此外一律禁止：

```plain
%USERPROFILE%\.kazeusa\
├── config.json          用户配置
├── history.json         安装历史
├── logs\                日志（5MB × 3 份轮转）
├── cache\               仓库索引与清单包缓存
└── webview2\            WebView2 运行时数据（本次收拢）

<Steam>\config\lua\<游戏名>_<AppID>.lua    唯一的外部写入
%TEMP%\dlctool_*\                          解压临时目录，用完即删
```

配套修改：

- `main.go` 显式设置 `WebviewUserDataPath` 指向 `~/.kazeusa/webview2/`
- `main.go` 补 `OnShutdown` 回调——此前完全缺失，是结构性缺口。日志 flush、未来的下载协程与缓存写入都需要这个退出钩子
- `logger.go` 日志迁至 `~/.kazeusa/logs/`，实现 5MB × 3 份轮转
- `app.go` 的 `startup` 中兜底清理 `%TEMP%` 下超过 24 小时的 `dlctool_*` 残留（设时间门槛以免误删另一实例正在使用的目录）
- `wails.json` 与 `main.go` 的窗口尺寸、背景色对齐（此前 json 写 `#ffffff` 而 main.go 写深灰，会闪白屏；json 中的 `maxWidth/maxHeight` 属死配置，改之无效）
- **不自动删除 v1.4 的 AppData 残留**：删除他人 `AppData` 目录需判断「是否为我所建」，判断失误即误删用户数据，此行为本身就不优雅。改由博客迁移教程指引用户手动清理

**验收标准**：卸载 = 删 exe + 删 `~/.kazeusa/` 一个文件夹。

---

## 2026-07-27: Steam 路径的唯一权威是 config.json，Deployer 改路径后重建

**背景**：v1.4 在 `App.steamPath` 字段与配置中各存一份 Steam 路径，导致用户在设置中改了路径后，部分操作仍使用旧值——这类双源真相引发的不一致极难排查。

同时，`Deployer` 在构造时固定 `steamPath`，需决定路径变更后如何处理。

**结论**：

- **移除 `App.steamPath` 字段**。唯一权威是 `config.json`，需要时经 `a.steamPath()` 读取，且明确约定不缓存返回值
- **Deployer 采用重建而非原地修改**（候选方案见下表）。改 Steam 路径是极低频操作，多数用户一生只做一次，为它污染接口签名不值得

| 方案      | 做法                                                      | 取舍                                                       |
| :-------- | :-------------------------------------------------------- | :--------------------------------------------------------- |
| A（采纳） | 路径变更时 `a.deployer = NewOSTDeployer(newPath, logger)` | 部署器内部无可变状态，也就无竞态；代价是调用方必须记得重建 |
| B         | 接口改为 `Deploy(steamPath, gp, ids)`                     | 无状态，但每个方法签名都要多带一个参数                     |

- 为防止「改了路径忘记重建」，将写配置与重建部署器打包为 `persistSteamPath`，`GetSteamPath` 与 `SetSteamPath` 均经由它执行。新增改路径入口时应沿用此函数

---

## 2026-07-27: GameRecord.InstalledAt 使用 RFC 3339 字符串而非 time.Time

**背景**：`wails generate module` 在生成前端绑定时报 `Not found: time.Time`——Wails 的类型映射器不认识标准库的 `time.Time`，导致前端拿不到该字段的类型定义。

**结论**：

- 字段类型改为 `string`，存储 RFC 3339 格式（`time.Now().Format(time.RFC3339)`）
- 排序直接比较字符串：RFC 3339 的年-月-日在前且位宽固定，字典序与时间顺序一致，无需解析回 `time.Time`
- 前端可用 `new Date(installedAt)` 直接构造 Date 对象
- 一般原则：**跨 Wails 边界的 DTO 只使用基础类型**（string / number / bool / slice / map / 自定义 struct）。引入标准库复合类型会让绑定生成静默丢字段

---

## 2026-07-27: 部署文件名限定为纯 ASCII

**背景**：实机联调时发现，部署 `Street Fighter™ 6_1364780.lua` 会导致 Steam 立即闪退，而同一批次的 `ARK_ Survival Ascended_2399830.lua` 正常生效。前者含全角商标符号，后者为纯 ASCII。

Debug 版 OST 的 `package.log` 定位了崩溃点：

```plain
[05:33:01.406] Processing 1 Lua file change(s)
[05:33:01.406] Lua file added: D:\Steam\config\lua\Street Fighter™ 6_1364780.lua
（日志到此中断，随即弹出 MSVC "abort() has been called"）
```

日志在打印文件名之后、`ParseFile` 结果之前中断，说明 OST 在拿到路径后、解析内容前即已终止。推断为其 `Encoding` 模块在宽字符与 UTF-8 之间转换路径时触发断言。

此前一度误判为生成脚本内容有误（缺少 `setManifestid`），修正内容后症状不变，才转而排查文件名。

**结论**：

- `sanitizeFileName` 丢弃所有码位 ≥ `0x7F` 的字符，并将连续下划线与空格折叠为单个下划线
- 选择**丢弃**而非替换为下划线：中文名游戏（如「原神」）若逐字替换会退化成一串下划线，既难以辨认又容易撞名；丢空后由 `luaFileName` 拼接 AppID 保证唯一性
- 文件名不再具备完整可读性，属可接受的代价——定位文件依靠 `_<AppID>.lua` 后缀，`Remove` 与 `readDeployedLua` 均不依赖游戏名

**教训**：症状出现在「部署之后」时，可疑范围应同时包含文件**内容**与文件**路径**，而非默认前者。

---

## 2026-07-27: addappid 第二参数确认无语义，DLC 独立 Depot 采用双行注册

**背景**：OST 的 README 示例写作 `addappid(id, 0, key)` 并注明「0:Depot 1:App」，而源码分析结论为该参数被完全忽略。此矛盾长期挂为待验证项。

实机比对社区清单包后确认：区分 App 与 Depot 身份靠的是**两次独立调用**，与第二参数无关。

**结论**：

- 第二参数固定填 `1`，仅为与社区脚本保持视觉一致，不承载任何语义
- 带独立 Depot 的 DLC 必须写两行——裸 `addappid(id)` 注册 App 身份，再 `addappid(id, 1, key)` 注册 Depot 密钥
- 每个 Depot 的密钥与 `setManifestid` 必须成对出现
- 主游戏自身密钥必须输出，早期版本仅取 AppID 而丢弃密钥
- README 与实现冲突时以源码及实测为准

---

## 2026-07-27: 卸载后的界面残留归属注入器层，仅在文案中提示

**背景**：卸载已安装本体的游戏（SF6）后，Steam 库中的 DLC 条目不会立即消失；ARK 则表现为「条目仍在但不再显示独立下载 DLC 的体积」。

如下是OST的日志：

`package.log` 显示后端清理完全成功：

```plain
UnloadFile: removed 16 depots
NotifyLicenseChanged: 0 added, 16 removed
queued 16 UI removals, skipped 0 transient removals
```

`steamui.log` 给出未生效的原因：

```plain
RunFrame: appId 1364780 is owned again, skipping removal
RunFrame: appId 1792750 is owned again, skipping removal
RunFrame: appId 1792751 is owned again, skipping removal
```

被跳过的三项恰为主游戏与两个带独立 Depot 的 DLC，均对应磁盘上已安装的内容。OST 的 `CSteamUIAppControllerRunFrame` 在消费移除队列时会检查 `IsOwned()`，跳过用户真实拥有的条目——这是防止误删正版内容的保护机制。

**结论**：

- 许可证层面的移除是彻底的，残留仅限界面显示
- 此行为属注入器层，按「不碰注入器内部逻辑」的铁律不予干预
- `RemoveDLCs` 的返回文案需相应调整，不再无条件声称「已卸载」，而是告知本体已安装时可能需要重启 Steam 才更新显示
- 同理，`InstallDLCs` 沿用「已添加到库」的表述仍然成立——安装方向的刷新经实测正常

---

## 2026-07-27: Steam 界面上的 DLC 日期显示不再追查

**背景**：Steam 的 DLC 管理列表显示日期为 2021-07-09，与清单包内任何时间戳均不吻合。

`steamui.log` 证实 OST 的注入行为正确：

```plain
FillInAppOverview: set PurchasedTime=1785102027 for appId=1364780
```

该时间戳换算为 2026-07-27 05:40:27，与 `.lua` 文件的 mtime 一致。

**结论**：

- `PurchasedTime` 注入链路无误，界面所显示日期取自何字段成因未明
- 不影响功能，不再投入排查
- 此项从待验证清单中移除，不作为已解释项归档——避免为未经证实的推断留下记录
