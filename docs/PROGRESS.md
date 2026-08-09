# 开发进度追踪

> 每次开发结束时更新本文件，下次开发接力时快速定位当前进度。
>
> 最后更新：2026-07-29

---

## 当前阶段：后端全部完工，⑧ 前端待施工（分支 `refactor/v2`）

### ✅ 本轮完成（2026-07-26 ~ 07-27）

**地基四件套**

- [x] **① config.go** — 配置持久化（`AppConfig` / `RepoSource` / `ConfigManager`，读-改-写全在锁内，JSON 原子落盘）
- [x] **② logger.go** — 日志迁至 `~/.kazeusa/logs/`（原在 `%TEMP%` 会被系统清理）、5MB × 3 份轮转、并发安全
- [x] **③ deployer.go + deployer_ost.go** — 部署器：生成 Lua 清单并原子写入 `<Steam>/config/lua/`
- [x] **④ detector.go + detector_ost.go** — 环境检测：查三个 DLL，采用 ready/missing/unknown 三态

**核心与整合**

- [x] **⑥ history.go** — 安装历史，按 mainAppID 去重覆盖
- [x] **⑦ app.go 重构** — 注入四模块，移除 `App.steamPath` 双源真相
- [x] **⑨ 旧代码清理** — 删除 `vdf_helper.go` 与 `andygrunwald/vdf` 依赖，`steam.go` 由约 620 行精简至 215 行
- [x] **前端清空** — 移除 v1.4 四个组件，`App.vue` 改为占位骨架，`style.css` 重写为设计令牌系统

**基建与治理**

- [x] **fileutil.go**（新增）— `atomicWriteFile` 原子写入、`webviewDataDir`、`cleanStaleTempDirs`、`fileExists`
- [x] **lua_match.go**（新增）— 承接 `luaContainsAppID`
- [x] **文件卫生治理** — WebView2 数据目录收拢、`OnShutdown` 生命周期补全、临时目录兜底清理、背景色三处对齐
- [x] **命名统一** — exe → `kazeusa.exe`，标题 → `风兔盒 - 请问您今天要来点DLC吗？`
- [x] 架构讨论落档 5 条新决策（事件总线 / 责任链 / 缓存分层 / 纯 CSS / 文件落点）

**实机联调（2026-07-27）**

- [x] **L1.5 测试台界面** — `App.vue` 组装环境横幅 / 拖拽区 / 清单面板 / 历史列表，
      新增 `api/index.ts` 封装层收敛 wailsjs 的 `arg1/arg2` 与 `File` 转换
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

| 路径  | DLC 数 | setManifestid | 耗时   |
| :---- | :----- | :------------ | :----- |
| M 站  | 19     | 13            | 4.5 秒 |
| MAU   | 4      | 1             | 3.9 秒 |

三源实测状态：ManifestHub 已被清空（仅剩 `main` 分支）故默认停用；MAU 可用但
收录不全；M 站数据完整但需用户自备凭据。详见 DECISIONS.md 的 07-28 条目。

### 📋 待开始

- [ ] ⑧ 前端 v2.0 UI 实现（设计已定稿，L1.5 测试台过渡可用）

### ✅ 收尾施工与缺陷修正（2026-07-29）

两个 commit：`chore` 清理遗留待办与数据目录迁移，`fix` 修正清单定位缺陷。
`go build` / `go vet` 在正式与 `-tags dev` 两种模式下均通过，`go test` 全绿。

**数据目录迁至 exe 同级**。`appDataDir` 是唯一收敛点，七处调用皆只依赖其返回值，
故此项为单函数改动。判据改用构建标签而非环境变量，理由见 DECISIONS.md 的
2026-07-29 条。可写性以探针文件验证，不止于 `MkdirAll` 成功。

**清单定位改按内容匹配**。原实现靠 `_<AppID>.lua` 后缀识别，漏判所有外部文件。
新增三个方法：

| 方法                     | 职责                                       |
| :----------------------- | :----------------------------------------- |
| `findLuaFilesDeclaring`  | 返回声明某 AppID 的全部文件名               |
| `externalDeclarations`   | 滤出其中非本工具命名格式者                  |
| `ScanDeployed`           | 全目录对账，产出 `[]DeployedEntry`          |

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

| 议题       | 结论                                                          |
| :--------- | :------------------------------------------------------------ |
| 数据目录   | 改为 exe 同级 `.kazeusa/`，`KAZEUSA_DEV=1` 时走 home           |
| 常量清理   | 删除 8 个 v1.4 遗留常量（vdf / ST / depotcache / killSteam）   |
| 检查更新   | 302 重定向探测免配额，**不实现自更新**，仅提示 + 开浏览器       |
| 设置页     | 四项必备 + 主题壁纸，清单源只读展示                            |
| 边界场景   | 路径识别失败、lua 目录缺失、Steam 未运行、密钥截断、写入串行化 |

发布流程约定：**蓝奏云上传先行，GitHub Release 作为最后一步**。Release 一发布
在线客户端即开始提示新版，若蓝奏云未就绪用户会撞上失效链接。

### ✅ 设计定稿（2026-07-28 第四轮 · UI 与前端架构）

⑧ 的设计取向敲定，落档 4 条决策：

| 议题     | 结论                                                        |
| :------- | :---------------------------------------------------------- |
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
- 数据目录选址不得依赖路径特征推断，只认环境变量
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

| 问题                       | 结论                                                              |
| :------------------------- | :---------------------------------------------------------------- |
| 多文件如何加载             | **并集去重**。`adding 8 apps` 精确等于两文件 AppID 去重后的数量    |
| 是否存在文件级优先权       | **不存在**。全局 map 的同 key 后写覆盖，Parse 顺序不可控           |
| 删除其一后共享 AppID 的命运 | **许可证保留**。refCount 由 2 减 1 未归零，游戏仍在 Steam 库中     |
| 存活文件的声明是否恢复      | **不恢复**。`processing 0 additions`，旧值残留至重启 Steam         |
| 密钥冲突是否可观测          | **完全不可观测**。无警告日志，且 OST 从不输出密钥值                |

**对实现的三条硬约束**：

1. 卸载前必须扫描部署目录中其他 Lua 是否声明同一 mainAppID。若有，不得报
   「已卸载」，须如实告知游戏可能仍留在 Steam 库中，并指出具体文件名
2. 靠命名前缀为盒子产物抢占加载优先级的思路无效，此方向已排除
3. 盒子是唯一能在事前发现密钥冲突的环节——落盘之后线索链完全断裂，
   症状将表现为「一切正常，直到下载时解密失败」

**顺带发现**：SteamTools 与 OST 共用 `dwmapi.dll` / `xinput1_4.dll` 两个劫持入口，
社区流传的 ST 卸载步骤会连带删除 OST 的加载链。教程须明确卸载与安装的顺序。

### 📌 用户体验缺口（已识别，待设计）

主线路径已顺，但异常路径尚有空白。按影响排序：

| 缺口                     | 影响                                             |
| :----------------------- | :----------------------------------------------- |
| 首次启动时注入器未安装   | **流失率最高环节**。仅挂红条不足以引导，需引导页 |
| OST 监控目录中存在未被记录的 Lua | **正确性问题**。见下方说明，会令卸载功能失效 |
| 三源均未收录该游戏       | 目前是死路，应就近引导至本地导入                 |
| 下载过程无进展反馈       | 只转圈会促使用户关窗，应显示当前尝试的源         |
| ST 残留与 OST 并存       | 两个注入器同时作用于同一 AppID，需检测并告知     |

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

| 版本       | 目标                                  | 状态 |
| ---------- | ------------------------------------- | ---- |
| v2.0-alpha | 地基完成：配置/日志/部署器/检测器     | ✅   |
| v2.0-beta  | 核心功能：在线仓库 + 历史管理 + 新 UI | 🔜 设计已定稿 |
| v2.0-rc    | 全功能可用，博客教程完成              | 📋   |
| v2.0       | 正式发布                              | 📋   |

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
