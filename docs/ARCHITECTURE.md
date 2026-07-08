# DLCtool v2.0 架构白皮书

> 本文档是 v2.0 开发的"宪法"，开发时应当遵循此守则
>
> 最后更新：2026-07-08

---

## 一、项目定位

**DLCtool 是一个 Steam DLC 清单包管理器（MOD 盒子）。**

不是注入器，不是清单包生产工具。是连接在线清单仓库与底层解锁工具之间的桥梁，负责：

1. 从在线仓库拉取/本地导入清单包
2. 解析清单包内容，展示可安装的 DLC 列表
3. 将清单文件部署到底层工具能读取的位置
4. 管理已安装游戏的状态与历史记录
5. 检测底层工具环境是否就绪（仅检测，不安装/不修复）

### 完全解耦的核心设计

```plain
┌─────────────────────────────────────────────────┐
│ 🌐 清单仓库层 (GitHub 仓库 / 镜像) │ ← 社区维护，咱不管内容生产
└─────────────────────┬───────────────────────────┘
│ 拉取/下载
┌─────────────────────▼───────────────────────────┐
│ 📦 盒子层 (kazeusa v2.0)                         │ ← 这是咱
│ "把正确的文件放到正确的地方"                        │
└─────────────────────┬───────────────────────────┘
│ 部署 .lua 文件到 config/lua/
┌─────────────────────▼───────────────────────────┐
│ 🔧 注入器层 (OpenSteamTool)                      │ ← 咱不管、不碰、不集成其内部逻辑
│ 自行热重载 Lua（500ms），自动下载 manifest          │
└─────────────────────────────────────────────────┘
```

**三条铁律：**

- 盒子不写 `config.vdf`（那是注入器的事）
- 盒子不写注入器自身的配置文件
- 盒子不负责安装/更新/修复注入器

### OST 源码研究确认的关键事实

| 事实                                                 | 出处                                            | 对 kazeusa 的意义        |
| ---------------------------------------------------- | ----------------------------------------------- | ------------------------ |
| Lua 目录默认 `<Steam>/config/lua/`，可通过 toml 扩展 | `dllmain.cpp` + `Config.cpp`                    | deployer 目标目录确定    |
| 热重载事件驱动 + 500ms 防抖                          | `LuaFileWatcher.cpp`                            | 文件落盘即生效，无需重启 |
| Steam 库安装/卸载后自动刷新                          | `Hooks_SteamUI.cpp` + `Hooks_Package.cpp`       | UX 文案可写"已添加到库"  |
| Manifest 下载全自动（三级回退）                      | `ManifestClient.cpp`                            | 不需要 manifest 相关功能 |
| addappid 第二参数被忽略                              | `LuaConfig.cpp:lua_addappid()`                  | M 站/社区 Lua 直接兼容   |
| 函数名大小写无关                                     | `LuaConfig.cpp:case_insensitive_global_index()` | 生成 Lua 用全小写即可    |
| 环境检测 = 3 个 DLL 存在                             | OST 加载链分析                                  | detector 实现确定        |

---

## 二、技术栈

| 层级     | 技术             | 版本                     |
| -------- | ---------------- | ------------------------ |
| 后端     | Go               | 1.23                     |
| 桌面框架 | Wails            | v2.11                    |
| 前端框架 | Vue              | 3.4+                     |
| 前端语言 | TypeScript       | 5.3+                     |
| 构建工具 | Vite             | 5.x                      |
| Lua 解析 | gopher-lua       | 1.1.2                    |
| VDF 解析 | andygrunwald/vdf | 1.1.0 (仅遗留兼容时使用) |

### 技术栈优势

- **单 exe 分发**：零依赖，拖出来就跑，用户不需要额外装环境
- **包体积小**：~15MB vs 200MB+
- **前端自由度**：Web 技术栈，CSS/动画/主题随便玩
- **Lua VM 解析**：格式免疫，不靠正则猜
- **Go 编译型语言**：静态类型兜底重构安全，交叉编译简单

---

## 三、模块清单与职责

```plain

├── main.go ← Wails 应用装配入口
├── app.go ← 前端 API 编排层（所有暴露给前端的方法）
├── config.go ← 配置持久化（读/写/原子落盘）
├── deployer.go ← 部署目标接口（抽象"把文件放到哪里"）
├── deployer_ost.go ← OST 部署器实现（放到 config/lua/）
├── detector.go ← 注入器环境检测接口
├── detector_ost.go ← OST 环境检测实现
├── repo_client.go ← 在线仓库拉取客户端（GitHub API + 镜像回退）
├── history.go ← 安装历史管理
├── lua_parser.go ← Lua VM 解析器（核心资产，从 v1.4 保留）
├── constants.go ← 路径常量
├── types.go ← 前后端共享 DTO
├── logger.go ← 日志系统（轮转 + tag + 路径迁移）
└── frontend/ ← Vue3 + TypeScript 前端

```

### 各模块说明

| 模块              | 职责                                                                         |
| ----------------- | ---------------------------------------------------------------------------- |
| `app.go`          | 前端能调用的所有方法都在这里，纯编排不做业务                                 |
| `config.go`       | 管理 `~/.kazeusa/config.json`，启动读取、变更时原子写入                      |
| `deployer.go`     | 定义"部署"接口：把 Lua 文件放到注入器能读的目录                              |
| `deployer_ost.go` | OST 实现：写入 `<Steam>/config/lua/<GameID>.lua`，使用 tmp+rename 原子写入   |
| `detector.go`     | 定义"检测"接口：注入器是否安装就绪                                           |
| `detector_ost.go` | OST 实现：检查 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll` 是否存在 |
| `repo_client.go`  | 从 GitHub 仓库拉取清单包列表/内容，含镜像回退和缓存                          |
| `history.go`      | 管理 `~/.kazeusa/history.json`，记录安装/卸载操作                            |
| `lua_parser.go`   | 嵌入式 Lua VM 执行清单脚本，提取 AppID/密钥/manifest 信息                    |
| `logger.go`       | 统一日志，支持轮转（5MB/3份）、操作 tag、文件+控制台双输出                   |

---

## 四、数据持久化

### 存储位置

```plain
%USERPROFILE%/.kazeusa/
├── config.json ← 用户配置
├── history.json ← 安装历史记录
└── logs/
├── kazeusa.log ← 当前日志
├── kazeusa.log.1 ← 轮转备份
└── kazeusa.log.2

```

### config.json 结构

```json
{
  "steamPath": "C:\\Program Files\\Steam",
  "theme": "dark",
  "deployDir": "config/lua",
  "lastZipDir": "D:\\Downloads",
  "repoSources": [
    {
      "name": "默认仓库",
      "type": "github",
      "url": "https://github.com/xxx/xxx",
      "mirror": "https://mirror.example.com/xxx"
    }
  ]
}
```

### history.json 结构

```json
[
  {
    "mainAppID": "1361510",
    "gameName": "Monster Hunter Stories",
    "dlcCount": 21,
    "installedIDs": ["1361511", "1361512"],
    "installedAt": "2025-07-07T15:30:00+08:00",
    "luaFileName": "MonsterHunterStories_1361510.lua"
  }
]
```

---

## 五、核心接口契约

### 5.1 Deployer 接口（部署器）

```go
// deployer.go

// Deployer 定义将清单文件部署到注入器监控目录的接口。
type Deployer interface {
    // Deploy 将游戏的 Lua 配置写入注入器可读目录。
    // 使用 tmp+rename 原子写入确保 OST FileWatcher 拿到完整内容。
    // 返回部署后的文件路径。
    Deploy(gp *GamePackage, selectedIDs []string) (string, error)

    // Remove 从注入器监控目录中移除指定游戏的配置。
    // OST 会在 500ms 内自动检测到删除并从 Steam 库移除游戏。
    Remove(mainAppID string) error

    // DeployDir 返回当前部署目标目录的完整路径。
    DeployDir() string
}
```

### 5.2 Detector 接口（环境检测）

```go
// detector.go

// DetectorResult 表示环境检测结果。
type DetectorResult struct {
    Name      string `json:"name"`      // 工具名称
    Available bool   `json:"available"` // 是否可用
    Message   string `json:"message"`   // 状态描述（供前端展示）
}

// Detector 定义注入器环境检测接口。
type Detector interface {
    // Detect 检查注入器是否已安装且环境就绪。
    // OST：检查 Steam 根目录下 dwmapi.dll + xinput1_4.dll + OpenSteamTool.dll
    Detect(steamPath string) *DetectorResult
}
```

### 5.3 前端 API（暴露给 wailsjs）

| 方法                 | 签名                                        | 说明                   |
| :------------------- | :------------------------------------------ | :--------------------- |
| `GetConfig`          | `() → AppConfig`                            | 获取当前配置           |
| `SaveConfig`         | `(AppConfig) → error`                       | 保存配置               |
| `GetSteamPath`       | `() → (string, error)`                      | 从注册表自动识别       |
| `SetSteamPath`       | `(string) → error`                          | 手动指定               |
| `SelectDirectory`    | `() → string`                               | 打开文件夹选择对话框   |
| `SelectZipFile`      | `() → string`                               | 打开 zip 选择对话框    |
| `DetectEnvironment`  | `() → DetectorResult`                       | 检测注入器环境         |
| `ProcessZipFile`     | `(string) → GamePackage`                    | 解析本地 zip           |
| `ProcessDroppedFile` | `(name, data) → GamePackage`                | 解析拖拽文件           |
| `InstallDLCs`        | `(GamePackage, []string) → OperationResult` | 部署清单到注入器目录   |
| `RemoveDLCs`         | `(string) → OperationResult`                | 按 mainAppID 移除      |
| `GetHistory`         | `() → []GameRecord`                         | 获取安装历史           |
| `FetchRepoList`      | `() → []RepoGameEntry`                      | 从在线仓库获取游戏列表 |
| `DownloadFromRepo`   | `(appID) → GamePackage`                     | 从仓库下载并解析       |
| `GetRecentLogs`      | `(n int) → []string`                        | 获取最近 n 条日志      |

---

## 六、与 v1.4 的主要差异

| 维度         | v1.4                              | v2.0                           |
| :----------- | :-------------------------------- | :----------------------------- |
| 底层工具     | SteamTools（已停更）              | OpenSteamTool（活跃维护）      |
| 耦合度       | 硬耦合（直接写 config.vdf + Lua） | 完全解耦（只放文件到监控目录） |
| 清单来源     | 用户手动下载 zip 拖入             | 在线仓库拉取 + 本地导入        |
| 配置持久化   | 无（每次重新识别）                | 有（\~/.kazeusa/config.json）  |
| 安装历史     | 无                                | 有（\~/.kazeusa/history.json） |
| 需要关 Steam | 是（写 config.vdf 前必须）        | 否（OST 热重载，500ms 内生效） |
| Lua 管理     | 追加到单文件                      | 每游戏独立文件                 |
| 部署方式     | 直接写入                          | tmp+rename 原子写入            |

---

## 七、施工顺序（推荐）

| 阶段 | 步骤              | 产出                              | 依赖 |
| :--- | :---------------- | :-------------------------------- | :--- |
| 地基 | ① 配置持久化      | `config.go`                       | 无   |
| 地基 | ② 日志增强        | `logger.go` 改造                  | ①    |
| 地基 | ③ 部署器接口+实现 | `deployer.go` + `deployer_ost.go` | ①    |
| 地基 | ④ 环境检测        | `detector.go` + `detector_ost.go` | ①    |
| 核心 | ⑤ 在线仓库客户端  | `repo_client.go`                  | ①    |
| 核心 | ⑥ 安装历史        | `history.go`                      | ①    |
| 整合 | ⑦ app.go 重构     | 接入新架构                        | ③④⑤⑥ |
| 整合 | ⑧ 前端 v2.0       | 全新 UI                           | ⑦    |

---

## 八、用户迁移策略（v1.4 → v2.0）

v2.0 不提供自动迁移。用户需要：

1. 卸载 SteamTools（使用其自带卸载程序）
2. 删除 `<Steam>/config/stplug-in/` 目录
3. 删除 `<Steam>/config/config.vdf`（Steam 会自动重新生成）
4. 清空 `<Steam>/depotcache/` 中的旧 manifest
5. 按照新教程安装 OpenSteamTool + kazeusa v2.0
