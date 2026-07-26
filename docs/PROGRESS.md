# 开发进度追踪

> 每次开发结束时更新本文件，下次开发接力时快速定位当前进度。
>
> 最后更新：2026-07-26

---

## 当前阶段：后端全部就绪，等待实机验证与前端设计（分支 `refactor/v2`）

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

### 📋 待开始

- [ ] ⑤ repo_client.go — 在线仓库客户端（L1 顺序回退 + L3 缓存）**← 被仓库源决策阻塞**
- [ ] ⑧ 前端 v2.0 UI 设计与实现（骨架已就绪，待设计）

### ⚠️ 尚未验证（重要）

当前后端**仅通过编译验证，未经实机运行**。下列行为需实测确认：

- [ ] `~/.kazeusa/` 能否正确创建，`config.json` 与 `logs/` 是否正常落盘
- [ ] WebView2 数据目录是否确实收拢至 `~/.kazeusa/webview2/`（而非旧的 `%APPDATA%\<exe名>\`）
- [ ] 环境检测能否正确识别 OST 的三个 DLL
- [ ] 部署器写出的 `.lua` 文件 OST 能否正常解析并刷新 Steam 库
- [ ] 清单包解析 → 部署 → 历史记录的完整闭环

前两项跑一次 `wails dev` 即可验证；后三项需 OST 环境与真实清单包。

### ⚠️ 遗留待办

- [ ] 在线仓库源的具体地址仍未确定（属产品决策，可参考竞品做法）
- [ ] 主人本机的 v1.4 残留需手动删除：`%APPDATA%\DLC入库工具.exe`、`%APPDATA%\DLC入库工具v1.4.exe`
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
- [x] addappid 第二参数：**被完全忽略**，代码只看第1参数(AppId)和第3参数(64字符key)
- [x] OST 环境检测指标：检查 Steam 根目录是否存在 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll`
- [x] M 站 Lua 与 OST 格式差异：无差异，OST 的 addappid 兼容任何第二参数
- [x] 在线仓库的具体源：待定（技术上无阻塞，属于产品决策）

---

## 版本里程碑

| 版本       | 目标                                  | 状态 |
| ---------- | ------------------------------------- | ---- |
| v2.0-alpha | 地基完成：配置/日志/部署器/检测器     | 🔜   |
| v2.0-beta  | 核心功能：在线仓库 + 历史管理 + 新 UI | 📋   |
| v2.0-rc    | 全功能可用，博客教程完成              | 📋   |
| v2.0       | 正式发布                              | 📋   |

---

## OST 源码研究关键发现摘要

> 详见 `docs/research/OST_Architecture_Analysis.md`

| 发现                               | 对 kazeusa 的影响                             |
| ---------------------------------- | --------------------------------------------- |
| 热重载 500ms 防抖，事件驱动        | deployer 写文件即可，推荐 tmp+rename 原子写入 |
| 安装后 Steam 库自动刷新            | UX 文案可以直接写"已添加到库"                 |
| Manifest 全自动获取                | 不需要 manifest fallback 功能                 |
| .lua 文件 mtime 作为 PurchasedTime | 不要修改文件时间戳，保持当前时间即可          |
| OST 函数名大小写无关               | 生成的 Lua 用全小写即可                       |
