# KAZEUSA（旧dlctool） v2.0 架构白皮书

> 本文档是 v2.0 开发的"宪法"，开发时应当遵循此守则
>
> 最后更新：2026-07-28

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
├── repo_client.go ← 清单仓库客户端（多源查找 + 镜像回退）
├── repo_package.go ← MAU 形态清单包解析器（无 lua 的包）
├── msite_client.go ← 认证型源客户端（Hubcap Manifest）
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

| 模块               | 职责                                                                            |
| ------------------ | ------------------------------------------------------------------------------- |
| `app.go`           | 前端能调用的所有方法都在这里，纯编排不做业务                                    |
| `config.go`        | 管理 `~/.kazeusa/config.json`，启动读取、变更时原子写入                         |
| `fileutil.go`      | `atomicWriteFile` 原子写入、WebView2 目录解析、临时目录兜底清理                 |
| `deployer.go`      | 定义"部署"接口：把 Lua 文件放到注入器能读的目录                                 |
| `deployer_ost.go`  | OST 实现：写入 `<Steam>/config/lua/<GameName>_<AppID>.lua`，tmp+rename 原子写入 |
| `detector.go`      | 定义"检测"接口：注入器是否安装就绪                                              |
| `detector_ost.go`  | OST 实现：检查 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll` 是否存在    |
| `repo_client.go`   | 多源查找与下载清单包，产出 `GamePackage`，与本地导入同出口                      |
| `store_client.go`  | Steam 商店元数据查询，仅认 AppID，不知仓库存在                                  |
| `history.go`       | 管理 `~/.kazeusa/history.json`，记录安装/卸载操作                               |
| `package_store.go` | 持久化 `GamePackage` 至 `packages/{appID}.json`，供卸载与增装 DLC 时复用        |
| `lua_parser.go`    | 嵌入式 Lua VM 执行清单脚本，提取 AppID/密钥/manifest 信息                       |
| `lua_match.go`     | 正则判断某 AppID 是否已在脚本中，用于不值得启动 VM 的轻量场景                   |
| `steam.go`         | 清单包解压（含路径遍历防护）、读取部署产物以标记 DLC 安装状态                   |
| `logger.go`        | 统一日志，支持轮转（5MB/3份）、级别标记、文件+控制台双输出                      |

---

## 四、数据持久化

### 存储位置

Release 的数据目录位于 **exe 同级**，使删除文件夹即等同彻底卸载：

```plain
<exe 所在目录>/
├── kazeusa.exe
└── .kazeusa/
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

#### 目录选址顺序

| 优先级 | 条件           | 目录                |
| :----- | :------------- | :------------------ |
| 1      | 开发构建       | `~/.kazeusa`        |
| 2      | 默认           | exe 同级 `.kazeusa` |
| 3      | exe 目录不可写 | 回退 `~/.kazeusa`   |

开发期走 home 目录，避免 `wails dev` 在工作区内生成缓存干扰 git 状态，
更避免数据随构建输出目录清理而消失。

**判据是 Go 构建标签 `dev`，而非环境变量**——Wails v2.11 的 `wails.json`
并无注入环境变量的字段，且构建标签由构建方式决定，`wails dev` 必然携带，
无从遗漏或误设。实现为 `buildmode_dev.go` / `buildmode_prod.go` 两个互斥文件
各定义 `isDevBuild` 常量。不以路径特征推断环境。

可写性判定不止于 `MkdirAll` 成功：目录可能已存在却拒绝写入文件（Program Files
下的 UAC 虚拟化即如此），故额外写入一个探针文件并立即删除。

不实现旧数据迁移。v1.4（时称 dlctool）不产生任何本地数据文件，其操作直接
作用于 Steam，故「迁移旧数据」并无对象；新位置为空即视为全新安装。

`packages/` 是卸载与增装 DLC 的数据来源。压缩包全程在 `%TEMP%` 内处理、用后即删，
不予保留——`GamePackage` 序列化后体积小两三个数量级，且已是可直接使用的状态。

此外仅有两处落盘：`<Steam>/config/lua/<游戏名>_<AppID>.lua`（部署产物），
以及 `%TEMP%/dlctool_*`（解压临时目录，启动时清理超 24 小时的残留）。

### config.json 结构

```json
{
  "steamPath": "C:\\Program Files (x86)\\Steam",
  "theme": "dark",
  "wallpaperPath": "",
  "lastZipDir": "D:\\Downloads",
  "autoDetect": true,
  "checkUpdate": true,
  "skippedVersion": "",
  "lastUpdateCheck": "2026-07-28T10:00:00+08:00",
  "repoSources": [
    {
      "name": "Hubcap Manifest",
      "kind": "api-zip",
      "repo": "https://hubcapmanifest.com",
      "enabled": true
    },
    {
      "name": "MAU",
      "kind": "github-branch",
      "repo": "Auiowu/ManifestAutoUpdate",
      "enabled": true
    }
  ]
}
```

`repoSources` 在 v2.0 由 `defaultRepoSources` 初始化，设置页只读展示，不提供编辑入口。
列表为空时（旧版配置或人工误删）自动回填内置源——否则在线功能彻底失效而用户
从界面上无从修复。

`RepoSource.token` 仅 `api-zip` 形态使用，为空时该源整体跳过。该字段带 `omitempty`，
未配置时不出现于 JSON 中。**凭据以明文存储**，故日志中绝不输出其内容。
`skippedVersion` 与 `lastUpdateCheck` 服务于检查更新的跳过与节流。

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

**输入侧比输出侧宽松**。上述契约约束的是本工具**生成**的脚本；**解析**时需容忍
社区各源的形态差异，现已知两处：

| 差异                              | 出现于           | 处理方式                     |
| :-------------------------------- | :--------------- | :--------------------------- |
| `setManifestid` 只有两个参数      | ManifestHub 快照 | `FileSize` 记 0              |
| 主游戏的 `addappid` 不带密钥      | ManifestHub 快照 | `MainKey` 留空，部署时记警告 |

第三参数即 depot 内容总大小，OST 不读取它（只用 GID 去上游换取 manifest request
code），故记 0 不影响产物有效性。但**界面不得将 `FileSize` 当作下载体积展示**——
该值现有三种来源：M 站的精确值、`appdetails` 的近似值、缺失时的 0，语义互不相同。

#### 同游戏的旧文件清理

部署文件名取自 `GamePackage.GameName`，而该字段随解析路径而异：MAU 路径拿到的
是中文名（非 ASCII 清洗后可能落为 `unknown`），Lua 路径拿到的是英文名。同一
游戏换源重新获取时，会产生两个文件名不同却都声明同一 AppID 的清单。

故 `Deploy` 写入前调用 `removeStaleOwnFiles` 清理自身产物：

| 规则                                 | 理由                                     |
| :----------------------------------- | :--------------------------------------- |
| 判定范围与 `Remove` 一致（后缀匹配） | 只清自己的产物，绝不触碰外部文件         |
| 与本次目标同名者跳过                 | 它即将被覆写，先删会让注入器多收一次事件 |
| 失败只记警告，不中止部署             | 用户拿到清单比清掉残留文件重要           |

不清理的后果是 07-28 记录的那类不可观测冲突：注入器取并集加载，其中一份的密钥
被静默覆盖，症状要到 Steam 下载时解密失败才显现。

#### 多文件共存的加载语义

以下五条经 2026-07-28 实机验证（样本 ARK 2399830，OST Debug 版 trace 日志）。
它们共同决定了部署与卸载**不能只考虑盒子自己的文件**。

| 事实                                    | 依据                                            |
| :-------------------------------------- | :---------------------------------------------- |
| 全部 `.lua` 按**并集去重**加载          | `adding 8 apps` 精确等于两文件 AppID 去重后数量 |
| 不存在文件级优先权                      | 全局 map 同 key 后写覆盖，Parse 顺序不可控      |
| 共享 AppID 的许可证不随单文件删除而移除 | refCount 由 2 减 1 未归零                       |
| 删除文件不触发存活文件重新解析          | `processing 0 additions`                        |
| 密钥冲突无任何日志痕迹                  | 无警告输出，且 OST 从不记录密钥值               |

**为何 `LuaConfig` 是全局的**：`DepotKeySet` / `ManifestOverrides` 等容器为整个
进程共有，`ParseDirectory` 遍历目录并将各文件的解析结果合并写入同一批 map。
文件不构成独立作用域，仅通过 `g_depotRefCount` 记录「有几个文件声明了此 AppID」。

**引用计数造成的卸载缺口**，实测日志：

```plain
UnloadFile:Ref count for AppId 2399830 is 2   ← 另一文件仍持有
UnloadFile: removed 7 depots from ...lua      ← 文件账本注销 7 个
NotifyLicenseChanged: 0 added, 5 removed      ← 许可证层仅移除 5 个
```

差额的 2 个即被两份文件共同声明者。其 refCount 未归零，故许可证保留，
游戏仍在 Steam 库中且重启不消失。

**对部署器与卸载逻辑的要求**：

- 卸载前扫描监控目录中其他 `.lua` 是否声明同一 mainAppID。若有，**不得报告
  「已卸载」**，须告知游戏可能仍留在库中并指出具体文件名
- 定位某 AppID 的部署文件时**按内容匹配 `addappid(<appID>`，不得依赖文件名**。
  外部文件可为任意命名，靠 `_<AppID>.lua` 后缀扫描会漏判
- 不要试图通过命名前缀争取加载优先级——无此机制
- 密钥冲突落盘后即不可观测，症状为「一切正常，直到下载时解密失败」。
  **部署前的主动检测是唯一的发现时机**

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

#### 搜索结果只保留游戏本体

`storesearch` 会把 DLC、试玩版、原声音轨与本体一并返回，而清单包以游戏本体为
单位组织，对这些条目查清单必然落空。故搜索后逐条查 `appdetails` 的 `type`
字段过滤，判定函数为 `isMainGame`。

两级判据：

| 级别 | 判据                             | 排除对象               |
| :--- | :------------------------------- | :--------------------- |
| 一   | `type != "game"`                 | dlc / demo / music 等  |
| 二   | `is_free` 为真且名称含衍生品标记 | 独立上架的序章、试玩版 |

第二级的存在是因为「序章」这类内容常被作为独立免费游戏上架，`type` 同样是
`game`（实测 The Riftbreaker 序章 AppID 1293860 即如此）。该判据**刻意收窄到
仅在免费前提下生效**，以免误杀名字带「序章」的付费正片。

实现上的三条约束：

- **`appdetails` 不支持批量查询**。实测 `appids=a,b,c` 返回空，10 条结果需发
  10 个请求。并发上限 5，并复用已有的 7 天详情缓存
- **失败或超时一律放行**。误杀远比漏放危险——搜不到会让用户以为工具不支持该
  游戏，多一条干扰项只是稍显杂乱
- **超时为部分生效而非整批放行**。已收到的判定仍然作数，否则慢网下过滤完全失效

`GameDetail` 因此新增 `Type` 与 `IsFree` 两字段，随详情缓存一同落盘。

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

    // KindAPIZip 需 Bearer 凭据的 API，返回含 .lua 与 .manifest 的 zip。
    // 不走镜像链（认证请求经第三方代理转发等于交出凭据），
    // 且收录检测有专用的免额度端点。
    KindAPIZip RepoKind = "api-zip"
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

**检测结果为三态，而非「收录 / 未收录」**：

| 结果           | 触发条件                        | 对下载阶段的影响       |
| :------------- | :------------------------------ | :--------------------- |
| `probeHit`     | HTTP 200                        | 确认可用               |
| `probeMiss`    | HTTP 404                        | **唯一可排除的情形**   |
| `probeUnknown` | 超时、连接重置、5xx、429、403   | 保留候选，交镜像链兜底 |

这个区分是必要的：大陆直连 codeload 超时是常态，若把超时当作未收录，会出现
「明明有清单却提示需要本地导入」，而下载阶段的四级镜像链本可救回。检测是优化
手段，不该拥有否决下载的权力。

认证型源的错误一律为 `probeUnknown`——凭据失效、额度耗尽都与「该游戏有无清单」
无关。

**并发上限 4**。源增至七个后，无限制并发会同时向 codeload 发六个请求，可能触发
限流；而限流响应算作 `probeUnknown`，该源便要在下载阶段白走一遍镜像链，反而更慢。

#### 内置源

八个内置源，形态与状态均不相同（分支数为 2026-07-29 `git ls-remote` 实测）：

| 名称                  | Kind            | 标识                            | 状态                                  |
| :-------------------- | :-------------- | :------------------------------ | :------------------------------------ |
| Hubcap Manifest       | `api-zip`       | `https://hubcapmanifest.com`    | 数据最完整，需用户自备凭据            |
| MAU                   | `github-branch` | `Auiowu/ManifestAutoUpdate`     | 2591 分支，本体自 2026-02 停更        |
| MAU 镜像              | `github-branch` | `Satisl/MAU`                    | 4062 分支，活跃                       |
| MAU fork · bingyu50   | `github-branch` | `bingyu50/ManifestAutoUpdate`   | 13131 分支                            |
| MAU fork · hansaes    | `github-branch` | `hansaes/ManifestAutoUpdate`    | 6336 分支                             |
| MAU fork · tymolu233  | `github-branch` | `tymolu233/ManifestAutoUpdate`  | 3140 分支                             |
| ManifestHub 快照      | `github-branch` | `SSMGAlt/ManifestHub2`          | 62288 分支，lua 形态，数据停在 2025-07 |
| ManifestHub           | `github-branch` | `SteamAutoCracks/ManifestHub`   | **默认停用**——仓库已清空，仅剩 `main` |

顺序即优先级。M 站置首位是因其数据完整度显著更高，但它在未配置凭据时自动跳过，
**MAU 系仍是默认路径**——免凭据可用是底线。

**排序依据是单游戏完整度，而非分支总数**。ARK(2399830) 实测对照：

| 源              | DLC | setManifestid |
| :-------------- | --: | ------------: |
| Hubcap          |  19 |            13 |
| MAU             |   4 |             1 |
| ManifestHub 快照 |   1 |             3 |

快照源的收录广度是 MAU 的 15 倍，但单个游戏的 DLC 覆盖反而更少。广度决定
「找不找得到」，完整度决定「找到了够不够用」——后者对已经找到清单的用户更重要，
故快照源置于末位，仅用于兜住前面各源都没有的冷门游戏。

**各源并非同构**。07-27 曾假定它们结构等价、一套代码全覆盖，该假定已被实测推翻。
现存三种包内形态：

| 形态          | 包内内容                     | 解析路径                    |
| :------------ | :--------------------------- | :-------------------------- |
| Hubcap        | `.lua` + 全部 `.manifest`    | lua 路径（三参数 setManifestid） |
| MAU 系        | `Key.vdf`/`config.vdf` + `.manifest` | MAU 路径（见 5.5）    |
| ManifestHub 快照 | 仅 `.lua`                 | lua 路径（两参数 setManifestid，主游戏行无密钥） |

故解析路径按包内实际内容分派（`zipContainsLua` 按扩展名判断），**不按来源假定**。
这一设计使 MAU 形态的三个 fork 零解析改动即可接入——解析器只认扩展名与内容结构，
不认文件名。

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

### 5.5 两条解析路径

清单包存在两种形态，`DownloadFromRepo` 依据**包内是否存在 `.lua`** 分派，
不按来源假定——上游随时可能调整打包脚本，按源名硬编码会静默失效。

| 形态     | 来源           | 包内容                                              | 解析器            |
| :------- | :------------- | :-------------------------------------------------- | :---------------- |
| Lua 形态 | M 站、本地导入 | `.lua` + `.manifest`                                | `lua_parser.go`   |
| MAU 形态 | MAU 及其镜像   | `Key.vdf` + `config.json` + `.manifest`，**无 lua** | `repo_package.go` |

MAU 形态的 `config.json` 实测样本：

```json
{
  "appId": 1364780,
  "depots": [1364781],
  "dlcs": [2224460, 2224461, 2224462, 2825190, 2825200],
  "packagedlcs": [1792750, 1792751]
}
```

`packagedlcs` 显式标出带独立 Depot 的 DLC，比从 Lua 注释启发式推断可靠——
正是格式契约中必须写三行、且取消勾选需模态框警告的那一类。

**MAU 的密钥分散于各自分支**：主游戏分支的 `Key.vdf` 只含主 Depot 一个密钥，
`packagedlcs` 中各 DLC 的密钥在以其 AppID 命名的独立分支内。故解析主包后需对
缺密钥者各拉一次分支补齐，并发限 4 路。补齐失败则该 DLC 退化为单行注册。

MAU 形态的 `FileSize` 语义与 Lua 路径不同：Lua 中该值为 depot 内容总大小，
MAU 路径下只能取 manifest 文件自身大小，两者差几个数量级。OST 不依赖它校验，
但**界面不得将其作为「下载体积」展示**。

### 5.6 认证型源（Hubcap Manifest）

定位与本地导入等同：可选增强，非底层主逻辑。未配置凭据时整条链路静默跳过。

| 端点                        | 额度 | 用途                                      |
| :-------------------------- | :--- | :---------------------------------------- |
| `/api/v1/status/{app_id}`   | 免   | 收录检测，附 `game_name`、`file_age_days` |
| `/api/v1/manifest/{app_id}` | 计   | 下载含 lua 的 zip                         |
| `/api/v1/user/stats`        | 免   | 额度与凭据到期日                          |

**用 `/manifest` 而非 `/lua`**（此选择反直觉，故记录于此）：两者同耗 1 次额度，
但 `/lua` 返回的脚本内 `setManifestid` 数量为 **0**，而 `/manifest` 包内的 lua
有 13 个（实测 ARK）。多传 11MB 换取数据完整性是值得的——包内 manifest 解析完
即丢，OST 会自行下载。分段端点 `/lua/basegame` 与 `/lua/dlc` 亦不使用，
各计一次额度反而更贵。

**绝不遍历 `/api/v1/library`**。该端点免额度且支持分页，但遍历十余万条正是该站
明令禁止的爬库行为（处罚为永久封禁）。搜索是唯一在线入口，本就无需遍历。

凭据处理的四条约束：

- 不内置任何共享凭据。exe 内的密钥必然被提取，且共享流量特征与爬库无法区分
- 认证请求不走镜像链，宁可直连失败
- 凭据只经 `Authorization` 头传递，不作查询参数（后者会进入各级代理日志）
- 日志中不输出凭据内容，仅记「已设置 / 已清除」

凭据默认有效期仅 7 天（捐助者 90 天），是该源的主要摩擦点。剩余不足 3 天时以
顶部横幅提示；额度耗尽须明确告知「今日额度已用尽，UTC 零点重置」而非笼统报失败。

### 5.7 检查更新

版本源以 GitHub Release 为准，蓝奏云为便利渠道但无可查询接口，不作判定依据。

分两级执行，API 配额仅在确有更新时消耗：

| 阶段         | 手段                                                                        |
| :----------- | :-------------------------------------------------------------------------- |
| 判断有无新版 | `GET github.com/{repo}/releases/latest`，不跟随重定向，读 302 的 `Location` |
| 取更新说明   | `api.github.com/repos/{repo}/releases/latest`，仅当有新版                   |

302 走的是网页路由而非 API，不受 60 次/小时的配额限制。国内失败时复用清单下载
已定的 `gh-proxy` 回退链。

**不实现自更新**，仅提示并打开浏览器。替换运行中的自身需下载并执行可执行文件，
分发链路一旦被劫持即等同任意代码执行；与「不负责安装/修复注入器」的铁律一致。

版本号于编译期注入，不硬编码：

```plain
go build -ldflags "-X main.appVersion=2.0.1"
```

带后缀的预发布版本不计为更新，除非用户当前亦为预发布版。

### 5.8 前端 API（暴露给 wailsjs）

> 以下为 2026-07-27 实际实现的签名。返回 `OperationResult` 的方法不返回 error——
> 失败信息经 `Message` 字段传达，前端只需判断 `Success` 一处，无需同时处理
> 异常与失败结果两套分支。

| 方法                 | 签名                                         | 说明                                |
| :------------------- | :------------------------------------------- | :---------------------------------- |
| `GetConfig`          | `() → AppConfig`                             | 获取当前配置                        |
| `SaveConfig`         | `(AppConfig) → OperationResult`              | 保存配置，路径变更时重建部署器      |
| `GetSteamPath`       | `() → (string, error)`                       | 从注册表识别并写入配置              |
| `SetSteamPath`       | `(string) → OperationResult`                 | 手动指定，校验 config 子目录        |
| `SelectDirectory`    | `() → (string, error)`                       | 文件夹选择对话框                    |
| `SelectZipFile`      | `() → (string, error)`                       | 清单包选择对话框，定位到上次目录    |
| `DetectEnvironment`  | `() → DetectorResult`                        | 检测注入器环境（三态）              |
| `GetDeployDir`       | `() → string`                                | 清单文件将写入的目录                |
| `ProcessZipFile`     | `(string) → (GamePackage, error)`            | 解析本地清单包                      |
| `ProcessDroppedFile` | `(name, data) → (GamePackage, error)`        | 解析拖拽文件                        |
| `InstallDLCs`        | `(GamePackage, []string) → OperationResult`  | 部署清单并记录历史                  |
| `RemoveDLCs`         | `(mainAppID) → OperationResult`              | 先删文件再删记录                    |
| `GetHistory`         | `() → []GameRecord`                          | 全部历史，按时间倒序                |
| `FindHistory`        | `(mainAppID) → GameRecord`                   | 单条查询，用于带出上次勾选          |
| `ClearHistory`       | `() → OperationResult`                       | 仅清空记录，不动已部署文件          |
| `GetLogPath`         | `() → string`                                | 当前日志文件路径                    |
| `OpenDataDir`        | `() → OperationResult`                       | 在文件管理器中打开数据目录          |
| `SearchGames`        | `(term) → ([]GameSearchResult, error)`       | 搜索，纯数字按 AppID 直查，仅留本体 |
| `GetGameDetail`      | `(appID) → (GameDetail, error)`              | 详情，失败时返回降级结果            |
| `LookupRepos`        | `(appID) → ([]string, error)`                | 并发查各源收录情况                  |
| `DownloadFromRepo`   | `(appID, sourceName) → (GamePackage, error)` | 下载并解析，形态自动分派            |
| `SetRepoToken`       | `(sourceName, token) → OperationResult`      | 设置认证型源的凭据，传空即清除      |
| `GetMSiteStats`      | `() → (MSiteStats, error)`                   | 额度与到期日，未配置凭据时返回 nil  |
| `GetPackage`         | `(mainAppID) → (StoredPackage, error)`       | 读 `packages/`，无留存时返回 nil    |
| `ScanDeployed`       | `() → []DeployedEntry`                       | 扫 `config/lua/` 对账，按内容匹配   |
| `CheckUpdate`        | `() → UpdateInfo`                            | **未实现**，302 探测                |
| `SkipVersion`        | `(version) → OperationResult`                | **未实现**，写 `skippedVersion`     |
| `OpenURL`            | `(url) → OperationResult`                    | **未实现**，交外部浏览器打开        |
| `GetAppVersion`      | `() → string`                                | **未实现**，返回编译期注入值        |

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

`DepotInfo.FileSize` / `DLCInfo.FileSize` 的语义随解析路径而异：Lua 路径取自
`setManifestid` 第三参数即 depot 内容总大小，MAU 路径无此信息只能退取 manifest
文件自身大小，二者相差几个数量级。**界面不得将其作为「下载体积」展示。**

#### StoredPackage 与清单时效

`packages/{mainAppID}.json` 的落盘格式在 `GamePackage` 外包一层元信息：

```go
type StoredPackage struct {
    SavedAt string       // 写入时刻，RFC 3339 字符串
    Source  string       // 获取来源的源名称，本地导入时为空
    Package *GamePackage
}
```

包这一层是因为「何时、从哪获取」都是界面需展示的信息，且无法从 `GamePackage`
本身推得。`SavedAt` 用字符串而非 `time.Time`，理由同 `GameRecord.InstalledAt`。

**存取层不做任何过期判定**。清单旧不等于无效，界面表述为「清单获取于 X 天前」
而非「已过期」。是否重新获取由用户自行决定。

返回值的两种「无内容」必须区别对待：

| 返回             | 含义       | 界面应当         |
| :--------------- | :--------- | :--------------- |
| `nil` + 无 error | 没有留存   | 引导用户获取清单 |
| `nil` + error    | 留存已损坏 | 提示重试         |

混为一谈的代价是部署出字段残缺的无效脚本——症状要到 Steam 下载失败时才显现。

#### DeployedEntry 与冲突提示

`ScanDeployed` 返回的每条 `DeployedEntry` 含 `IsExternal`（是否非本工具命名格式）
与 `InHistory`（该主游戏是否在安装历史中）两个判定位。二者并非互补：

| 组合                        | 含义                                   |
| :-------------------------- | :------------------------------------- |
| `!IsExternal && InHistory`  | 常态                                   |
| `!IsExternal && !InHistory` | 历史丢失或被清空，文件仍在             |
| `IsExternal && !InHistory`  | 典型外部清单，用户手动放置或他工具产生 |
| `IsExternal && InHistory`   | 同一游戏被两处声明，**卸载将不彻底**   |

界面约定：

- `IsExternal` 为真的条目**不提供 DLC 勾选**。本工具无对应的 `packages/` 数据，
  只能还原 AppID 集合而无法得知可选项全貌
- 外部清单可查看与删除，但删除须经用户明确发起——其中可能含用户特意配置的内容
- `InstallDLCs` 与 `RemoveDLCs` 在检出外部声明时会改写返回文案。卸载场景返回的是
  **失败**而非成功，前端不应将其视为异常，而应如实呈现「已删除自己的文件，
  但游戏可能仍在库中」并列出需手动处理的文件名

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

## 七之二、界面设计

### 设计取向

以 PCL2（Plain Craft Launcher 2）为参照：实用优先、信息密度高而有序、动效有活力
但不拖延。具体提取为三条原则：

- **不藏功能**。无汉堡菜单、无多级折叠，所有功能两次点击内可达
- **密度靠组织而非留白**。卡片内信息不少，依靠对齐与分组维持秩序
- **状态始终可见**。进度与检测结果摊在明面，不使用户猜测

**不照搬**其一：PCL2 的多主题与自定义色相体系。启动器长期驻留，个性化价值高；
本工具用完即关，配色系统投入产出不成比例。仅做亮/暗/跟随系统三档加一个主题色。

### 窗口形态

采用 `Frameless: true`，标题栏由前端绘制，令 Logo、导航页签与窗口控制融为一体。

```plain
┌──────────────────────────────────────────────────────────────────┐
│ 🐰 风兔盒   ● 搜索  ○ 已安装  ○ 设置        ✓ OST 就绪   ─  □  ✕ │
└──────────────────────────────────────────────────────────────────┘
   Logo       导航页签（当前项带下划线）      环境状态    窗口控制
```

环境状态常驻标题栏——需长期可见但不应占用内容区。三态：

| 显示           | 含义             | 交互         |
| :------------- | :--------------- | :----------- |
| ✓ OST 就绪     | 三个 DLL 均在位  | 无           |
| ⚠ 未检测到 OST | 缺少 DLL         | 点击跳引导页 |
| ? 路径未设置   | Steam 路径未确定 | 点击跳设置页 |

拖动经 CSS 声明，标题栏内所有可交互元素必须显式禁用：

```css
.titlebar {
  --wails-draggable: drag;
}
.titlebar button,
.titlebar .nav-tab {
  --wails-draggable: no-drag;
}
```

窗口控制调 `WindowMinimise` / `WindowToggleMaximise` / `Quit`。

frameless 的已知代价，实现时需留意：

| 事项               | 说明                                             |
| :----------------- | :----------------------------------------------- |
| `no-drag` 遗漏     | 按钮点击会被识别为拖拽起始而失灵，最常见的翻车点 |
| 双击标题栏最大化   | 系统行为丧失，需自行绑定 `dblclick`              |
| Win11 Snap Layouts | 悬停最大化按钮的分屏浮层不可用，属原生标题栏特权 |
| 边缘缩放热区       | 边框变窄，前端元素勿占据边缘，留 4~6px 空隙      |

拖到屏幕边缘的 Aero Snap 在当前 Wails 版本下是否保留**尚未验证**。

### 页面结构

顶层三页加一个详情页与一个引导页，全部扁平，无折叠层级。

```plain
搜索（首页）
  │  搜索框（游戏名 或 AppID）
  │  横向结果卡片：封面 + 名称 + AppID + 已入库标记
  │  底部弱化的拖拽区
  ↓
游戏页 /game/:appID
  │  未入库：封面大图 + 简介 + 三源查找进展 + [入库]
  │  已入库：DLC 勾选列表，勾选即生效
  └  [全选] [全不选] [替换清单] [彻底卸载]

已安装
  │  网格卡片：封面 + 名称 + 已选/总数 + 获取时间 + 来源
  └  [检查清单更新] ← 按需触发，非自动轮询

设置
  │  环境：Steam 路径 / 注入器状态 / 写入目录
  │  清单源：三个内置源与最后成功时间（只读）
  │  外观：主题 / 壁纸
  └  关于：版本 / 检查更新 / 数据目录 / 日志 / 外链

引导 /setup
  └  注入器未安装时的三步引导
```

搜索结果用**横向**卡片以容纳完整游戏名；已安装用**网格**，封面为主视觉、名称可截断。

「游戏页」是同一组件的两种状态，据本地是否存在 `packages/{appID}.json` 分支渲染。
不拆为两个路由——否则入库成功后需路由跳转，过渡动画与返回逻辑均会复杂化。

### DLC 列表的表意约定

```plain
☑  Shadow of the Erdtree                          43.2 GB  ⚑
☐  Original Soundtrack                             2.4 GB
☐  Preorder Bonus Gesture                              —
```

- `⚑` 标记带独立 Depot 的 DLC，取消勾选时触发二次确认
- `—` 表示无独立 Depot，纯许可证，安装不占空间
- 同步状态置于列表右上，三态轮转：`⋯ 待同步` → `🔄 同步中` → `✓ 已同步`（2 秒后淡出）
- Steam 未运行时末态改为 `✓ 下次启动 Steam 后生效`

**符号必须配常驻图例，不可只靠悬停提示**。多数用户不会去悬停一个符号，而
`⚑` 的实际含义（需由 Steam 另行下载）直接影响用户预期。图例文案须说明后果而非
仅描述属性：说「含独立内容分支」不如说「需由 Steam 另行下载才能玩到内容」。

#### 界面须解释操作的实际后果

实机试用的反馈集中在「不知道点了之后发生了什么」，而非功能缺失。故界面须解释
**用户无从自行确认的环节**，而不是重复界面上已有的信息。四类说明：

| 类别         | 内容                                                       |
| :----------- | :--------------------------------------------------------- |
| 符号含义     | `⚑` 需 Steam 另行下载；`—` 已含在本体内                    |
| 生效链路     | 写入清单 → 注入器读取 → Steam 库刷新 → 带 ⚑ 的按需下载     |
| 异常处境成因 | 外部清单同样在生效；卸载为何不彻底；清空记录只清账本       |
| 前置要求     | 需本体名或 AppID；大陆网络需加速工具；文件名须避免非 ASCII |

**生效链路必须逐步讲明，不可只说「已同步」**。该链路横跨本工具、注入器与 Steam
三方，每一环的实际状态都在别处，界面无从代为确认。讲清链路让用户能自行判断卡在
哪一步，比给一个笼统的成功提示有用。

### 关键交互约束

详见 DECISIONS.md，此处汇总：

- 勾选状态即部署状态，无「安装/卸载选中项」按钮
- 勾选先改内存、界面立即响应，800ms 无新操作才落盘
- 取消勾选带独立 Depot 的 DLC 需二次确认，Steam 可能删除本地内容
- **「全不选」的确认强度不得低于逐个取消**。批量入口是「顺手点一下」的位置，
  误触概率反而更高。不逐个弹窗（DLC 可能有数十个），改为一次性列出受影响条目，
  列举上限 8 条以免弹窗高度超出窗口把确认按钮挤出视口
- 全部取消勾选保留主游戏行，与「彻底卸载」是两件事
- 界面不展示 manifest ID，展示「获取时间」与「来源」
- 三源均未收录时给出路而非死路，就近引导至本地导入
- 检查更新的结果表述为「仓库有更新」，不称「本地已过期」

### 动效

PCL2 的活力感来自快速、带过冲、以位移为主的动画。缓动与时长定为设计令牌统一取用：

```css
:root {
  /* 标准过渡：多数状态变化 */
  --ease-standard: cubic-bezier(0.4, 0, 0.2, 1);
  /* 入场：末端轻微过冲，活力感的来源 */
  --ease-enter: cubic-bezier(0.34, 1.4, 0.64, 1);
  /* 退场：快速收走 */
  --ease-exit: cubic-bezier(0.4, 0, 1, 1);

  --dur-fast: 150ms;
  --dur-base: 220ms;
  --dur-slow: 300ms;
}
```

`--ease-enter` 的第二个控制点取 1.4（大于 1）以产生过冲，元素略微越过终点再弹回。

约定：

- 动画时长不超过 300ms
- 以 `transform` 与 `opacity` 驱动，避免触发布局重排
- 列表错峰入场，每项延迟 20~30ms
- **必须尊重 `prefers-reduced-motion`**。过多动效会令前庭功能障碍用户产生眩晕，
  一条媒体查询的成本没有不做的理由

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

### 壁纸的对比度处理

用户可能选择浅色图片，致使暗色主题的浅色文字不可读。壁纸与内容之间垫一层
半透明遮罩，遮罩透明度随主题变化。遮罩承担主要的压暗职责，模糊仅作点缀。

---

## 七之三、反馈体系

四种形态并存，以一条规则划分职责，避免退化为「重要的用弹窗、次要的用 toast」
这类凭手感的判断。

### 划分规则

判据是**用户是否必须做出选择**，而非事情是否重要。信息再重要，只要无需决策，
就不应阻断用户。

```plain
需要用户拍板？
├─ 是 → 后果难挽回？
│        ├─ 是 → 模态弹窗（遮罩 + 模糊）
│        └─ 否 → 内联确认（原地展开，不遮挡）
└─ 否 → 存在持续过程？
         ├─ 是 → 原地状态指示
         └─ 否 → Toast
```

### 场景归属

| 场景                        | 形态         | 依据                          |
| :-------------------------- | :----------- | :---------------------------- |
| 取消勾选带独立 Depot 的 DLC | 模态弹窗     | Steam 可能删除数十 GB，不可逆 |
| 替换清单的差异对照          | 模态弹窗     | 需对比两组信息后决策          |
| 彻底卸载游戏                | 模态弹窗     | 破坏性                        |
| 清空历史记录                | 模态弹窗     | 破坏性                        |
| 部署成功 / 检测完成         | Toast        | 无需决策                      |
| 已跳过此版本                | Toast        | 无需决策                      |
| 网络失败可重试              | Toast 带操作 | 不阻断，可忽略                |
| 勾选同步状态                | 原地指示     | 持续过程，绑定于页面          |
| 清单下载进度                | 原地指示     | 时长不确定，需与位置绑定      |
| 三源查找进展                | 原地指示     | 同上                          |
| 发现新版本                  | 顶部横幅     | 不紧急，不应拦路              |

原地指示与顶部横幅是使用频次最高的两类。下载进度若用 Toast，用户切换页面后即
失去进展感知；置于游戏页则返回时仍可见。

更新提示**不使用弹窗**：启动即以模态框遮挡界面仅为告知有新版本，属典型打扰。

### 模态弹窗

```plain
┌──────────────────────────────────────────────────┐
│  ⚠  取消此 DLC 可能删除已下载的内容               │
│                                                  │
│  Shadow of the Erdtree 带有独立 Depot，取消勾选   │
│  后 Steam 可能移除本地已下载的 43.2 GB 内容。     │
│  重新勾选需再次下载。                             │
│                                                  │
│  ☐ 本次会话内不再提示                            │
│                                                  │
│              ┌────────┐  ┌──────────┐            │
│              │ 取消 ● │  │ 仍然取消 │            │
│              └────────┘  └──────────┘            │
└──────────────────────────────────────────────────┘
```

约定：

- **默认焦点置于安全选项**。用户习惯性回车时应落在「不执行危险操作」上，
  危险按钮亦不作主视觉强调
- **给出具体后果与体积**。「43.2 GB」远比「可能删除内容」有力，数字使代价可感
- **「本次会话内不再提示」限定于会话**，不写入配置。批量操作时连续弹窗是折磨，
  但持久化会使用户在很久之前的一次勾选中永久失去保护
- **Esc 与点击遮罩一律等于取消**，永不等于确认。误触的代价必须是「什么都没发生」
- **焦点锁定在弹窗内**，Tab 不得跳至背后界面；关闭后焦点归还触发元素
- **不嵌套弹窗**。多步流程在同一弹窗内切换内容

### Toast

右下角堆叠，自右侧滑入，同时最多三条，超出则挤出最旧者。

- 成功类 3 秒后自动消失
- **警告与错误类不自动消失**，须用户关闭或执行操作——飘走的错误等同于未曾提示
- **所有警告与错误同时写入日志**。Toast 消失后无从追溯，日志是唯一留存

### 性能注意

`backdrop-filter: blur()` 在 WebView2 中由 GPU 加速，但若背后铺有用户壁纸，
全窗口大面积模糊在集显设备上可能掉帧。模糊半径控制在 8~12px，配合半透明暗色
遮罩——遮罩承担主要的压暗效果，模糊仅作点缀。

---

## 七之四、前端架构

### 首要原则：唯一真相在 Go

前端不持有状态所有权，所有 store 均为后端状态的**镜像**而非**所有者**。

违反此原则会立即产生漂移——界面显示已装 3 个 DLC，实际部署文件中却是 5 个。
因此每个 store 都须有明确的刷新时机，而非自行维护推导出的状态。

本项目的数据流因此比一般 SPA 更简单：后端说什么即是什么，前端无需操心一致性。
真正的复杂度在交互打磨，不在数据流。

### 分层

```plain
┌─────────────────────────────────────────────────┐
│  views/         路由级页面组件                   │
│  components/    可复用组件                       │
├─────────────────────────────────────────────────┤
│  composables/   跨页面行为逻辑                   │
│                 useConfirm / useToast /          │
│                 useDlcSelection                  │
├─────────────────────────────────────────────────┤
│  stores/        Pinia，后端状态的镜像            │
├─────────────────────────────────────────────────┤
│  api/           wailsjs 封装层                   │
├─────────────────────────────────────────────────┤
│  Go 后端        唯一真相                         │
└─────────────────────────────────────────────────┘
```

**组件不得直接 import wailsjs**，一律经 `api/` 层。该层吸收生成代码的形态差异
（如 `arg1` / `arg2` 参数名、`File` 转换），Wails 升级改变生成规则时只需改动一处。

实际落地的目录（2026-07-29）：

```plain
frontend/src/
├── main.ts                挂 router + pinia
├── App.vue                外壳：TopBar + RouterView + 反馈宿主
├── style.css              设计令牌与全局样式
├── router/index.ts        5 条路由 + /setup 守卫
├── api/index.ts           24 个方法，含 unwrap 与 ApiError
├── stores/                env / config / library / ui
├── composables/           useToast / useConfirm / useDlcSelection
├── components/            TopBar / GameCard / DlcList / ConfirmDialog
│                          ToastHost / EnvBanner / DropZone
└── views/                 Search / Game / Library / Settings / Setup
```

样式采用纯 CSS 变量，不引入原子类框架；组件全部手写，不引入 UI 组件库——
全站只需按钮 / 复选框 / 卡片 / 弹窗 / Toast 五种，引库反而要与其主题系统打架。

`Frameless` **尚未启用**。Aero Snap 保留情况未经实机验证，且 `no-drag` 遗漏是
已知的高频翻车点，故留待正式上线前单独评估与实现，以便翻车时干净回退。

### 路由

```ts
const routes = [
  { path: "/", component: SearchView },
  { path: "/game/:appID", component: GameView },
  { path: "/library", component: LibraryView },
  { path: "/settings", component: SettingsView },
  { path: "/setup", component: SetupView },
];
```

- **使用 `createWebHashHistory`**。Wails 经自定义协议提供页面，history 模式在
  部分情形下刷新会 404
- `/game/:appID` 单一路由承担未入库与已入库两种状态，据 `packages/{appID}.json`
  是否存在分支渲染
- `/setup` 由路由守卫拦截：环境未就绪且用户非主动前往设置页时重定向至此

游戏页实际有三种状态，第三种是留存缺失时的降级：

| 状态 | 条件                   | 呈现                           |
| :--- | :--------------------- | :----------------------------- |
| A    | 未入库                 | 详情 + 三源查找进展 + 入库按钮 |
| B    | 已入库且有留存清单     | DLC 勾选列表，勾选即生效       |
| C    | 已入库但留存缺失或损坏 | 仅提供重新获取与彻底卸载       |

状态 C 只在留存文件被手动删除、内容损坏，或游戏是在留存功能上线前入库时出现。
此时**不伪造一份不完整的勾选列表**——本工具无从得知可选项全貌，让用户误以为
可以随意勾选比如实告知更糟。

### Store 划分

按「谁需要读它」划分，而非按数据类型：

| Store             | 内容                                             | 为何全局                     |
| :---------------- | :----------------------------------------------- | :--------------------------- |
| `useEnvStore`     | Steam 路径、检测结果、写入目录、Steam 是否运行   | 标题栏与部署链路均需读取     |
| `useConfigStore`  | `config.json` 镜像                               | 主题与壁纸影响全局样式       |
| `useLibraryStore` | 已安装记录（history 与 `ScanDeployed` 合并结果） | 搜索卡片的「已入库」标记需读 |
| `useUiStore`      | Toast 队列、弹窗队列、顶部横幅                   | 任意位置均可触发             |

**不进 store 的两类状态**：

- **搜索结果**——会话级临时数据，无其他页面需读，留在 `SearchView` 内即可
- **DLC 勾选状态**——归属「当前所看的这一个游戏」，置于全局反而需处理切换游戏时的
  清理。封装为页面内的 composable

### DLC 勾选的落盘时机

```ts
// composables/useDlcSelection.ts
//
// 管理单个游戏的 DLC 勾选与落盘。
//
// 勾选先只改内存以保证界面即时响应，800ms 内无新操作才真正部署——该值大于 OST
// FileWatcher 的 500ms 防抖窗口，确保聚合完成后只触发注入器一次。
//
// 参数取 Ref 而非裸对象：本函数内部注册了 onUnmounted，必须在组件 setup 的
// 同步作用域内调用。清单包往往是异步到手的，若要求传裸对象，调用方只能把它塞进
// await 之后或 watch 回调里，此时 onUnmounted 注册不到当前组件实例，待落盘的
// 改动会静默丢失。
export function useDlcSelection(pkgRef: Ref<GamePackage | null>) {
  const selected = ref(new Set<string>());
  const syncState = ref<"idle" | "pending" | "syncing" | "done">("idle");
  // ...
}
```

防抖使用 VueUse 的 `useDebounceFn`，不自行实现。

三条易被忽略的实现约束：

| 约束                                    | 违反后果                                     |
| :-------------------------------------- | :------------------------------------------- |
| `restore` 还原勾选**不得**触发落盘      | 打开一次页面即白部署一次，获取时间被刷成今天 |
| 重置勾选的 `watch` 须用 `flush: 'sync'` | 赋值 pkg 后紧接 `selectAll()` 会落盘空列表   |
| `selectNone` 须与 `toggle` 同等确认     | 一次点击取消全部带独立 Depot 的 DLC          |

第二条的成因是 `watch` 默认的 `pre` 时机为异步：回调会晚于同步调用的
`selectAll` 执行，把刚选好的集合清空，800ms 后落盘一个空列表——等于什么都没装。

### 确认弹窗以 Promise 呈现

```ts
// composables/useConfirm.ts
//
// 以 Promise 形式呈现确认弹窗，使调用方能按线性顺序书写决策逻辑，避免回调嵌套。
// 用户按 Esc 或点击遮罩均解析为 false。
export function useConfirm(): (opts: ConfirmOptions) => Promise<boolean>;
```

调用方因此得以保持可读：

```ts
async function onToggle(dlc: DLCInfo) {
  // 取消勾选带独立 Depot 的 DLC 时 Steam 可能删除本地内容，须先取得确认
  if (isSelected(dlc) && dlc.manifestID && !skipWarnThisSession.value) {
    const ok = await confirm({ /* ... */ danger: true });
    if (!ok) return;
  }
  toggle(dlc.appID);
}
```

若改用事件总线或回调，该逻辑会被割裂为两处。

### 后端推送

下载进度是唯一需后端主动推送的场景，轮询不适用。经 Wails 事件机制：

```go
runtime.EventsEmit(ctx, "download:progress", map[string]any{
    "appID": appID, "source": "ManifestHub", "stage": "downloading",
})
```

**组件卸载时必须 `EventsOff`**。否则切换页面再返回会重复注册，同一事件收到多份，
是 Wails 项目中常见的隐性缺陷。

### OperationResult 的归一化

`OperationResult` 不抛异常，仅携带 `Success` 与 `Message`。由 `api/` 层统一转换为
异常语义，避免每个组件同时书写 `Success` 判断与 `try/catch` 两套分支：

```ts
// api/index.ts
//
// 将 OperationResult 归一化为异常语义：业务失败与运行时异常在调用方视角下一致。
async function unwrap(p: Promise<OperationResult>): Promise<void> {
  const r = await p;
  if (!r.success) throw new ApiError(r.message);
}
```

组件侧统一 `try/catch` 配合 Toast。

---

## 八、用户迁移策略（v1.4 → v2.0）

v2.0 不提供自动迁移。用户需要：

1. 卸载 SteamTools（使用其自带卸载程序）
2. 删除 `<Steam>/config/stplug-in/` 目录
3. 删除 `<Steam>/config/config.vdf`（Steam 会自动重新生成）
4. 清空 `<Steam>/depotcache/` 中的旧 manifest
5. 按照新教程安装 OpenSteamTool + kazeusa v2.0
