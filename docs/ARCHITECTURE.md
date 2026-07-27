# KAZEUSA（旧dlctool） v2.0 架构白皮书

> 本文档是 v2.0 开发的"宪法"，开发时应当遵循此守则
>
> 最后更新：2026-07-27

---

## 一、项目定位

**是一个 Steam DLC 清单包管理器（MOD 盒子）。**

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
| Steam 库安装后自动刷新                               | `Hooks_SteamUI.cpp` + `Hooks_Package.cpp`       | 安装文案可写"已添加到库" |
| 卸载时 `IsOwned()` 的条目会被跳过移除                | 2026-07-27 实测 `steamui.log`                   | 卸载文案需提示可能重启   |
| Manifest 下载全自动（三级回退）                      | `ManifestClient.cpp`                            | 不需要 manifest 相关功能 |
| addappid 第二参数被忽略                              | `LuaConfig.cpp:lua_addappid()` + 实测           | 填 1 即可，无语义        |
| 函数名大小写无关                                     | `LuaConfig.cpp:case_insensitive_global_index()` | 生成 Lua 用全小写即可    |
| 环境检测 = 3 个 DLL 存在                             | OST 加载链分析                                  | detector 实现确定        |
| 文件名含非 ASCII 字符会令 OST `abort()`              | 2026-07-27 实测 `package.log`                   | 文件名必须清洗为纯 ASCII |

---

## 二、技术栈

| 层级     | 技术       | 版本  |
| -------- | ---------- | ----- |
| 后端     | Go         | 1.23  |
| 桌面框架 | Wails      | v2.11 |
| 前端框架 | Vue        | 3.4+  |
| 前端语言 | TypeScript | 5.3+  |
| 构建工具 | Vite       | 5.x   |
| Lua 解析 | gopher-lua | 1.1.2 |

### 技术栈优势

- **单 exe 分发**：零依赖，拖出来就跑，用户不需要额外装环境
- **包体积小**：15MB左右
- **前端自由度**：Web 技术栈，CSS/动画/主题随便玩
- **Lua VM 解析**：格式免疫，不靠正则猜
- **Go 编译型语言**：静态类型兜底重构安全，交叉编译简单

---

## 三、模块清单与职责

```plain

├── main.go ← Wails 应用装配入口
├── app.go ← 前端 API 编排层（所有暴露给前端的方法）
├── config.go ← 配置持久化（读/写/原子落盘）
├── fileutil.go ← 公共文件基建（原子写入 / 临时目录清理 / 路径工具）
├── deployer.go ← 部署目标接口（抽象"把文件放到哪里"）
├── deployer_ost.go ← OST 部署器实现（放到 config/lua/）
├── detector.go ← 注入器环境检测接口
├── detector_ost.go ← OST 环境检测实现
├── repo_client.go ← 清单仓库客户端（codeload 下载 + 镜像回退）
├── store_client.go ← Steam 商店元数据（搜索 / 详情 / 封面）
├── history.go ← 安装历史管理
├── package_store.go ← 清单解析结果的落盘与读取（packages/*.json）
├── lua_parser.go ← Lua VM 解析器（核心资产，从 v1.4 保留）
├── lua_match.go ← Lua 脚本轻量文本匹配
├── steam.go ← 清单包解压与已部署状态检测
├── constants.go ← 路径常量
├── types.go ← 前后端共享 DTO
├── logger.go ← 日志系统（轮转 + 路径迁移）
└── frontend/ ← Vue3 + TypeScript 前端

```

### 各模块说明

| 模块              | 职责                                                                            |
| ----------------- | ------------------------------------------------------------------------------- |
| `app.go`          | 前端能调用的所有方法都在这里，纯编排不做业务                                    |
| `config.go`       | 管理 `~/.kazeusa/config.json`，启动读取、变更时原子写入                         |
| `fileutil.go`     | `atomicWriteFile` 原子写入、WebView2 目录解析、临时目录兜底清理                 |
| `deployer.go`     | 定义"部署"接口：把 Lua 文件放到注入器能读的目录                                 |
| `deployer_ost.go` | OST 实现：写入 `<Steam>/config/lua/<GameName>_<AppID>.lua`，tmp+rename 原子写入 |
| `detector.go`     | 定义"检测"接口：注入器是否安装就绪                                              |
| `detector_ost.go` | OST 实现：检查 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll` 是否存在    |
| `repo_client.go`  | 多源查找与下载清单包，产出 `GamePackage`，与本地导入同出口                      |
| `store_client.go` | Steam 商店元数据查询，仅认 AppID，不知仓库存在                                  |
| `history.go`      | 管理 `~/.kazeusa/history.json`，记录安装/卸载操作                               |
| `package_store.go` | 持久化 `GamePackage` 至 `packages/{appID}.json`，供卸载与增装 DLC 时复用       |
| `lua_parser.go`   | 嵌入式 Lua VM 执行清单脚本，提取 AppID/密钥/manifest 信息                       |
| `lua_match.go`    | 正则判断某 AppID 是否已在脚本中，用于不值得启动 VM 的轻量场景                   |
| `steam.go`        | 清单包解压（含路径遍历防护）、读取部署产物以标记 DLC 安装状态                   |
| `logger.go`       | 统一日志，支持轮转（5MB/3份）、级别标记、文件+控制台双输出                      |

---

## 四、数据持久化

### 存储位置

```plain
%USERPROFILE%/.kazeusa/
├── config.json ← 用户配置
├── history.json ← 安装历史记录
├── logs/
│   ├── kazeusa.log ← 当前日志
│   ├── kazeusa.log.1 ← 轮转备份
│   └── kazeusa.log.2
├── cache/ ← 在线元数据缓存（⑤ 阶段建立）
│   ├── detail/{appID}.json ← 商店详情，7 天有效
│   └── image/{appID}_header.jpg ← 封面图，永久
├── packages/ ← 清单解析结果，{mainAppID}.json
└── webview2/ ← WebView2 运行时数据

```

`packages/` 是卸载与增装 DLC 的数据来源。压缩包全程在 `%TEMP%` 内处理、用后即删，
不予保留——`GamePackage` 序列化后体积小两三个数量级，且已是可直接使用的状态。

此外仅有两处落盘：`<Steam>/config/lua/<游戏名>_<AppID>.lua`（部署产物），
以及 `%TEMP%/dlctool_*`（解压临时目录，启动时清理超 24 小时的残留）。

### config.json 结构

```json
{
  "steamPath": "C:\\Program Files (x86)\\Steam",
  "theme": "dark",
  "lastZipDir": "D:\\Downloads",
  "autoDetect": true,
  "repoSources": [
    {
      "name": "默认仓库",
      "type": "github",
      "url": "https://github.com/xxx/xxx",
      "mirror": "https://mirror.example.com/xxx",
      "enabled": true
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
    "installedAt": "2026-07-27T00:15:48+08:00",
    "luaFileName": "Monster Hunter Stories_1361510.lua",
    "source": "ManifestHub"
  }
]
```

`installedAt` 为 RFC 3339 字符串而非 `time.Time`，原因见 DECISIONS.md。
以 `mainAppID` 为唯一键去重覆盖：重复部署同一游戏是更新记录，不是追加条目。
`source` 标记来源（源名称或「本地导入」），供界面展示与检查更新时定位回源。

完整的 DLC 清单不存于此处，而在 `packages/{mainAppID}.json`。此分工使卸载与增装
无需用户重新提供清单包。

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

#### 生成脚本的格式契约

以下每一条都有实机验证支撑，违反其中任意一条都会导致 Steam 崩溃或功能静默失效。

| 规则                                         | 违反后果                        |
| :------------------------------------------- | :------------------------------ |
| 主游戏必须带自身密钥输出                     | 已安装本体的游戏会令 Steam 崩溃 |
| 每个 Depot 的密钥与 `setManifestid` 成对出现 | manifest 版本无法钉住           |
| 带独立 Depot 的 DLC 写两行                   | App 与 Depot 身份无法同时成立   |
| Depots 段跳过 DLC 自有的 Depot               | 取消勾选失效，密钥仍被写出      |
| 文件名清洗为纯 ASCII                         | OST 在 `ParseFile` 前 `abort()` |

标准输出形态：

```lua
-- 主游戏：必须带密钥
addappid(1364780, 1, "ab1ae48f...")

-- 本体与共享 Depot：密钥 + manifest 成对
addappid(1364781, 1, "cfec3971...")
setManifestid(1364781, "4741141599989541719", 86803973402)

-- 带独立 Depot 的 DLC：三行
addappid(1792750)                          -- 注册 App 身份
addappid(1792750, 1, "321dd0bd...")        -- 注册 Depot 密钥
setManifestid(1792750, "6884397220835125615", 4643138331)

-- 无独立 Depot 的 DLC：单行
addappid(2224460)
```

`addappid` 第二参数无语义（详见 DECISIONS.md 2026-07-27 条），固定填 `1` 仅为与社区脚本视觉一致。

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

### 5.3 StoreClient 接口（商店元数据）

```go
// store_client.go

// GameSearchResult 是搜索列表项，只含渲染卡片所需的最小字段。
// Available 由 RepoClient 回填，搜索阶段不填充——理由见 DECISIONS.md。
type GameSearchResult struct {
    AppID       string `json:"appID"`
    Name        string `json:"name"`
    HeaderImage string `json:"headerImage"`
    Available   bool   `json:"available"`
}

// GameDetail 是游戏页所需的完整元数据。
type GameDetail struct {
    AppID       string   `json:"appID"`
    Name        string   `json:"name"`
    HeaderImage string   `json:"headerImage"`
    Description string   `json:"description"`
    Developers  []string `json:"developers"`
    Publishers  []string `json:"publishers"`
    ReleaseDate string   `json:"releaseDate"`
    Screenshots []string `json:"screenshots"`
    DLCIDs      []string `json:"dlcIDs"`
}

// StoreClient 提供 Steam 商店元数据查询。
// 仅使用官方公开接口，不接入第三方自建 API（理由见 DECISIONS.md）。
type StoreClient interface {
    // Search 按关键词搜索。纯数字输入直接视为 AppID，跳过搜索接口。
    Search(term string) ([]GameSearchResult, error)

    // Detail 获取游戏详情。
    // 失败时返回仅含 AppID 与封面 URL 的降级结果，保证界面不空白。
    Detail(appID string) (*GameDetail, error)
}
```

封面 URL 由 AppID 直接拼接，不发请求：

```plain
header:  https://cdn.cloudflare.steamstatic.com/steam/apps/{appID}/header.jpg
library: https://cdn.cloudflare.steamstatic.com/steam/apps/{appID}/library_600x900.jpg
```

### 5.4 RepoClient 接口（清单仓库）

```go
// repo_client.go

// RepoKind 区分仓库的访问形态，决定采用哪套下载逻辑。
type RepoKind string

const (
    // KindGitHubBranch 以 AppID 作为分支名，走 codeload 下载分支 zip。
    KindGitHubBranch RepoKind = "github-branch"

    // KindZipTemplate 用 {app_id} 占位符拼出直链 zip。
    // v2.0 无内置源使用此形态，仅为自定义源预留。
    KindZipTemplate RepoKind = "zip-template"
)

// RepoClient 提供清单包的查找与获取。
type RepoClient interface {
    // Lookup 并发询问所有启用的源，返回收录该 AppID 的源名称。
    // 全部未收录时返回空切片而非 nil。
    Lookup(appID string) ([]string, error)

    // Fetch 下载并解析清单包，产出与本地导入一致的 GamePackage。
    // sourceName 为空表示自动选择首个可用源，非空则只尝试指定源。
    Fetch(appID string, sourceName string) (*GamePackage, error)
}
```

下载地址与镜像回退顺序：

```plain
https://codeload.github.com/{repo}/zip/refs/heads/{appID}

回退：gh-proxy.org → cdn.gh-proxy.org → edgeone.gh-proxy.org → 直连
```

收录检测用 HEAD 请求同一地址，不消耗 GitHub API 配额，因此无需 token。
检测只对用户进入详情页的单个 AppID 执行，搜索结果列表不预先标记。

#### 内置源

三个源同为 `github-branch` 形态，互为备份，任一收录即可完成入库：

| 名称        | 仓库                          |
| :---------- | :---------------------------- |
| ManifestHub | `SteamAutoCracks/ManifestHub` |
| MAU         | `Auiowu/ManifestAutoUpdate`   |
| MAU 镜像    | `Satisl/MAU`                  |

v2.0 不提供自定义源的界面入口，但 `RepoSource` 与 `RepoKind` 已按多源设计，
后续开放仅需增加设置页，不动查找与下载逻辑。

#### 缓存分层

| 内容       | 位置                             | 有效期   |
| :--------- | :------------------------------- | :------- |
| 搜索结果   | 内存                             | 单次会话 |
| 商店详情   | `cache/detail/{appID}.json`      | 7 天     |
| 封面图     | `cache/image/{appID}_header.jpg` | 永久     |
| 清单压缩包 | 不缓存                           | 用后即删 |

清单不缓存是有意为之：仓库会更新 manifest 版本，缓存旧包等同于向用户提供过期清单。

### 5.5 前端 API（暴露给 wailsjs）

> 以下为 2026-07-27 实际实现的签名。返回 `OperationResult` 的方法不返回 error——
> 失败信息经 `Message` 字段传达，前端只需判断 `Success` 一处，无需同时处理
> 异常与失败结果两套分支。

| 方法                 | 签名                                         | 说明                             |
| :------------------- | :------------------------------------------- | :------------------------------- |
| `GetConfig`          | `() → AppConfig`                             | 获取当前配置                     |
| `SaveConfig`         | `(AppConfig) → OperationResult`              | 保存配置，路径变更时重建部署器   |
| `GetSteamPath`       | `() → (string, error)`                       | 从注册表识别并写入配置           |
| `SetSteamPath`       | `(string) → OperationResult`                 | 手动指定，校验 config 子目录     |
| `SelectDirectory`    | `() → (string, error)`                       | 文件夹选择对话框                 |
| `SelectZipFile`      | `() → (string, error)`                       | 清单包选择对话框，定位到上次目录 |
| `DetectEnvironment`  | `() → DetectorResult`                        | 检测注入器环境（三态）           |
| `GetDeployDir`       | `() → string`                                | 清单文件将写入的目录             |
| `ProcessZipFile`     | `(string) → (GamePackage, error)`            | 解析本地清单包                   |
| `ProcessDroppedFile` | `(name, data) → (GamePackage, error)`        | 解析拖拽文件                     |
| `InstallDLCs`        | `(GamePackage, []string) → OperationResult`  | 部署清单并记录历史               |
| `RemoveDLCs`         | `(mainAppID) → OperationResult`              | 先删文件再删记录                 |
| `GetHistory`         | `() → []GameRecord`                          | 全部历史，按时间倒序             |
| `FindHistory`        | `(mainAppID) → GameRecord`                   | 单条查询，用于带出上次勾选       |
| `ClearHistory`       | `() → OperationResult`                       | 仅清空记录，不动已部署文件       |
| `GetLogPath`         | `() → string`                                | 当前日志文件路径                 |
| `OpenDataDir`        | `() → OperationResult`                       | 在文件管理器中打开数据目录       |
| `SearchGames`        | `(term) → []GameSearchResult`                | **未实现**（⑤ 阶段）             |
| `GetGameDetail`      | `(appID) → GameDetail`                       | **未实现**（⑤ 阶段）             |
| `LookupRepos`        | `(appID) → []string`                         | **未实现**（⑤ 阶段）             |
| `DownloadFromRepo`   | `(appID, sourceName) → (GamePackage, error)` | **未实现**（⑤ 阶段）             |
| `GetPackage`         | `(mainAppID) → (GamePackage, error)`         | **未实现**，读 `packages/`       |

原设计的 `FetchRepoList`（列出仓库全部收录）已移除，理由见 DECISIONS.md：
`ManifestHub` 有数万个分支，完整列表对用户无使用价值，搜索是唯一合理入口。

#### DTO 约束

跨 Wails 边界的结构体**只使用基础类型**（string / number / bool / slice / map /
自定义 struct）。标准库复合类型会让 `wails generate module` 静默丢字段——
`GameRecord.InstalledAt` 曾用 `time.Time`，导致生成时报 `Not found: time.Time`，
后改为 RFC 3339 字符串。

返回切片的方法必须返回空切片而非 nil：Wails 会把 nil 切片序列化为 JSON `null`，
前端 `v-for` 遍历时报错。

`GamePackage.MainKey` 与 `DLCInfo.ManifestID` / `FileSize` 是生成合法脚本的必要
字段，解析器必须完整填充——缺失时不会报错，只会静默产出无效脚本。

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

| 阶段 | 步骤              | 产出                                 | 依赖 | 状态 |
| :--- | :---------------- | :----------------------------------- | :--- | :--- |
| 地基 | ① 配置持久化      | `config.go` + `fileutil.go`          | 无   | ✅   |
| 地基 | ② 日志增强        | `logger.go` 改造                     | ①    | ✅   |
| 地基 | ③ 部署器接口+实现 | `deployer.go` + `deployer_ost.go`    | ①    | ✅   |
| 地基 | ④ 环境检测        | `detector.go` + `detector_ost.go`    | ①    | ✅   |
| 核心 | ⑤ 在线仓库客户端  | `repo_client.go` + `store_client.go` | ①    | 🔜   |
| 核心 | ⑥ 安装历史        | `history.go`                         | ①    | ✅   |
| 整合 | ⑦ app.go 重构     | 接入新架构                           | ③④⑥  | ✅   |
| 整合 | ⑧ 前端 v2.0       | 全新 UI                              | ⑦    | 🔜   |
| 整合 | ⑨ 旧代码清理      | 移除 ST 遗留逻辑                     | ⑦    | ✅   |

⑤ 的仓库源已于 2026-07-27 决定（聚合三个社区源），阻塞解除。
⑦ 与 ⑨ 实际必须一并完成——删除 ST 路径方法后其调用方立即编译失败。

---

## 七之二、界面结构

```plain
┌ 首页 ──────────────────────────────────┐
│ 搜索框（游戏名 或 AppID）               │
│ 搜索结果卡片：封面 + 名称 + AppID       │
│ ─────────────────────────────           │
│ 或拖入本地清单包 ← 次要但可发现          │
└─────────────────────────────────────────┘
                  ↓
┌ 游戏页 ────────────────────────────────┐
│ 未入库：封面 + 简介 + 可用源 + [入库]    │
│ 已入库：DLC 勾选列表，勾选即生效         │
│ [全选] [全不选] [替换清单] [彻底卸载]   │
└─────────────────────────────────────────┘

┌ 已安装 ────────────────────────────────┐
│ 卡片：封面 + 名称 + DLC 数 + 获取时间   │
│ [检查更新] ← 按需触发，非自动轮询        │
└─────────────────────────────────────────┘
```

「游戏页」是同一组件的两种状态，未入库与已入库共用，用户心智模型统一为
「一个游戏一个页面」。

关键交互约束（详见 DECISIONS.md）：

- 勾选状态即部署状态，无「安装/卸载选中项」按钮
- 勾选先改内存、界面立即响应，800ms 无新操作才落盘
- 取消勾选带独立 Depot 的 DLC 需二次确认，Steam 可能删除本地内容
- 全部取消勾选保留主游戏行，与「彻底卸载」是两件事
- 界面不展示 manifest ID，展示「获取时间」与「来源」

---

## 八、用户迁移策略（v1.4 → v2.0）

v2.0 不提供自动迁移。用户需要：

1. 卸载 SteamTools（使用其自带卸载程序）
2. 删除 `<Steam>/config/stplug-in/` 目录
3. 删除 `<Steam>/config/config.vdf`（Steam 会自动重新生成）
4. 清空 `<Steam>/depotcache/` 中的旧 manifest
5. 按照新教程安装 OpenSteamTool + kazeusa v2.0
