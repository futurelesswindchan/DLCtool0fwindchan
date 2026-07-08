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
