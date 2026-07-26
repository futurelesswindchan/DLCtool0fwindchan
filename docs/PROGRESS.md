# 开发进度追踪

> 每次开发结束时更新本文件，下次开发接力时快速定位当前进度。
>
> 最后更新：2026-07-27

---

## 当前阶段：后端已通过实机端到端验证，等待正式 UI 设计（分支 `refactor/v2`）

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

### 📋 待开始

- [ ] ⑤ repo_client.go — 在线仓库客户端（L1 顺序回退 + L3 缓存）**← 被仓库源决策阻塞**
- [ ] ⑧ 前端 v2.0 UI 设计与实现（L1.5 测试台可用，正式界面待设计）

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

- [ ] 在线仓库源的具体地址仍未确定（属产品决策，可参考竞品做法）
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
- [x] 在线仓库的具体源：待定（技术上无阻塞，属于产品决策）

---

## 版本里程碑

| 版本       | 目标                                  | 状态 |
| ---------- | ------------------------------------- | ---- |
| v2.0-alpha | 地基完成：配置/日志/部署器/检测器     | ✅   |
| v2.0-beta  | 核心功能：在线仓库 + 历史管理 + 新 UI | 📋   |
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
