# OpenSteamTool 源码架构分析书

---

## 1. 项目概览

| 维度     | 内容                                                 |
| -------- | ---------------------------------------------------- |
| 语言     | C++20 (MSVC)                                         |
| 构建     | CMake 3.20+ / Visual Studio 2022                     |
| 平台     | Windows only (Linux 预留目录但未实现)                |
| 许可     | GPL-3.0                                              |
| 核心机制 | DLL 劫持 + Detours API Hook + Protobuf 网络拦截      |
| 配置层   | TOML (静态配置) + Lua (动态游戏配置)                 |
| 外部依赖 | Microsoft Detours, Lua 5.x, Protobuf, spdlog, toml++ |
| 仓库     | https://github.com/OpenSteam001/OpenSteamTool        |

---

## 2. 整体架构分层

```plaintext
┌──────────────────────────────────────────────────────────────────┐
│            Entry Points ）DLL 劫持入口层                           │
│  dwmapi.dll / xinput1_4.dll → LoadLibrary("OpenSteamTool.dll")   │
└────────────────────────────┬─────────────────────────────────────┘
                             │ DllMain → Worker Thread
┌────────────────────────────▼─────────────────────────────────────┐
│               Core Init (dllmain.cpp)                            │
│  Config::Load → PatternLoader::Load → IPCLoader::Load            │
│  → LuaConfig::ParseDirectory → Hook Install → Pipe Start         │
└──┬───────┬───────┬───────┬───────┬───────┬───────────────────────┘
   │       │       │       │       │       │
   ▼       ▼       ▼       ▼       ▼       ▼
┌─────┐┌─────┐┌──────┐┌─────┐┌───────┐┌──────────┐
│Hook ││Pipe ││Utils ││Steam││OST    ││Proto/    │
│Layer││Layer││Layer ││Types││Platform│Codegen   │
└─────┘└─────┘└──────┘└─────┘└───────┘└──────────┘
```

---

## 3. 入口机制 (Entry Points)

### 3.1 DLL 劫持原理

OST 提供两个代理 DLL：`dwmapi.dll` 和 `xinput1_4.dll`。放入 Steam 根目录后，Windows 的 DLL 搜索顺序使 `steam.exe` 优先加载本地版本而非 System32 原版。

### 3.2 dwmapi.dll

- **纯转发代理**：通过 `#pragma comment(linker, "/EXPORT:...")` 将 ~100 个导出函数静态转发至真实 `DWMAPI.DLL`
- **注入逻辑**：`DllMain(DLL_PROCESS_ATTACH)` 中调用 `OpenSteamToolLoad()`
- **安全检查**：仅当宿主进程为 `steam.exe`（不区分大小写）时才加载 `OpenSteamTool.dll`

### 3.3 xinput1_4.dll

- **动态包装器**：运行时从 System32 加载真实 `xinput1_4.dll`，逐个获取函数指针
- **完整 API 覆盖**：7 个标准导出 + 6 个未公开 Ordinal（100-104, 108）
- **Steam 兼容**：未公开 Ordinal 对 Big Picture / Guide Button 必要
- **同样的注入逻辑**：`OpenSteamToolLoad()` 检查进程名后加载核心 DLL

### 3.4 OpenSteamTool.dll (dllmain.cpp)

核心 DLL 的 `DllMain` **只做一件事**：启动 Worker Thread，避免 Loader Lock 下执行文件 I/O 和 Detour。

**InitThread 时序**：

```plaintext
1. Log::Init()                    — 初始化日志系统
2. InitializeSteamComponents()    — 解析路径，加载 steamclient64.dll / steamui.dll
3. Config::Load()                 — 解析 opensteamtool.toml
4. Log::InitModules()             — 按配置初始化各模块日志
5. SteamDiagnostics::Initialize() — 准备诊断工具
6. PatternLoader::Load() ×2       — 为 steamui / steamclient 加载 pattern TOML
7. IPCLoader::Load()              — 加载 IPC 方法元数据
8. LuaConfig::ParseDirectory()    — 解析所有 .lua 配置文件
9. LuaFileWatcher::Start()        — 启动 Lua 热重载监视
10. ConfigFileWatcher::Start()    — 启动 TOML 热重载监视
11. SteamUI::CoreHook()           — 安装 SteamUI 钩子
12. SteamClient::CoreHook()       — 安装 SteamClient 全部钩子
13. PatternLoader::ReportMissing()— 报告未找到的函数
14. CloudRedirectHost::Init()     — 可选云重定向
```

**DLL_PROCESS_DETACH 清理**：停止 FileWatcher → 卸载钩子 → 关闭 CloudRedirect

---

## 4. 平台抽象层 (OSTPlatform)

位于 `src/OSTPlatform/`，为上层提供跨平台（目前仅 Windows）的基础设施：

| 模块                   | 职责                                                                   |
| ---------------------- | ---------------------------------------------------------------------- |
| `ByteSearch`           | 内存字节模式扫描                                                       |
| `Detour`               | Microsoft Detours 的 C++ 封装（BeginTransaction/Attach/Detach/Commit） |
| `Dialog`               | Win32 弹窗（警告/错误）                                                |
| `DirectoryWatch`       | ReadDirectoryChangesW 封装                                             |
| `DynamicLibrary`       | LoadLibrary/GetProcAddress 抽象                                        |
| `Encoding`             | UTF-8 ↔ Wide 转换                                                      |
| `Hash`                 | SHA-256 计算                                                           |
| `Http`                 | WinHTTP 封装（GET/POST）                                               |
| `Memory`               | 模块镜像获取（base + size）                                            |
| `PE`                   | PE 文件格式解析                                                        |
| `Process`              | 进程信息查询（PID, 创建时间）                                          |
| `RemoteProcess`        | 远程进程操作（架构检测, DLL 注入）                                     |
| `SteamCredentialStore` | Windows 注册表凭据存储                                                 |
| `Thread`               | 线程启动封装                                                           |
| `Trap`                 | 调试陷阱                                                               |

---

## 5. 配置系统

### 5.1 TOML 配置 (Config)

文件：`opensteamtool.toml`（Steam 根目录）

结构体 `Config::Snapshot` 包含所有配置项：

- `[manifest]` — provider 选择 + HTTP 超时
- `[log]` — 日志级别
- `[lua]` — 额外 Lua 路径
- `[remote]` — 镜像 URL 模板
- `[stats]` — stats API 开关
- `[inject]` — 游戏进程注入设置
- `[cloud]` — 云存档重定向

**热重载**：`ConfigFileWatcher` 监视文件变更，自动重新加载。

### 5.2 Lua 配置 (LuaConfig)

核心数据结构：

- `DepotKeySet<AppId_t, string>` — 解密密钥映射
- `AccessTokenSet<AppId_t, uint64_t>` — 访问令牌
- `ManifestOverrides<uint64_t, ManifestOverride>` — manifest 绑定
- `StatSteamIdSet<AppId_t, uint64_t>` — 成就 SteamID 覆盖
- `OwnedAppIdSet` — 已拥有游戏标记（排除自身）

**注册的 Lua 函数**：
| 函数 | 作用 |
|---------|---------|
| `addappid(id [,0, key])` | 注册 depot + 可选解密密钥 |
| `addtoken(id, token)` | 注册 PICS 访问令牌 |
| `setmanifestid(depot, gid [,size])` | 绑定 manifest |
| `setappticket(id, hex)` | 写入 AppTicket 凭据 |
| `seteticket(id, hex)` | 写入 ETicket 凭据 |
| `setstat(id, steamid)` | 成就 SteamID 覆盖 |
| `http_get(url [,headers])` | HTTP GET |
| `http_post(url, body [,headers])` | HTTP POST |
| `fetch_manifest_code(gid)` | 自定义 manifest 码获取 |
| `fetch_manifest_code_ex(app,depot,gid)` | 扩展版 |

**大小写无关**：通过 `_G` 元表 `__index` 实现全局函数名 case-insensitive 查找。

**热重载机制**：

- 文件级引用计数 (`g_depotRefCount`)
- UnloadFile 时减计数，计数归零才真正移除 depot
- 增量通知：`TakePendingRemovals()` / `TakePendingAdditions()`

---

## 6. Pattern 动态适配系统

### 6.1 设计哲学

OST **不在 DLL 中硬编码任何字节签名**。每次 Steam 启动时：

`SHA-256(steamclient64.dll) → 查远端 TOML → 获取该版本的函数签名`

### 6.2 RemoteToml 获取链

```plaintext
1. 计算 DLL SHA-256
2. 构造缓存路径: <Steam>/opensteamtool/pattern/<component>/<sha256>.toml
3. 尝试远端下载（镜像链）:
   GitHub raw → jsDelivr CDN → 自定义镜像
4. 成功: 写入缓存，返回 body
5. 远端 404: 立即停止（所有镜像同内容）
6. 远端失败: 回退本地缓存
7. 全部失败: 弹窗提示，禁用该模块钩子
```

### 6.3 Pattern TOML 格式

```toml
[0x82428E37]        # FNV-1a hash of function name
name = "BBuildAndAsyncSendFrame"
rva = "0x1234ABCD"  # 优先使用 RVA（精确）
sig = "48 89 5C 24 ?? 57 48 83 EC 20"  # 回退使用 IDA-style 签名
```

### 6.4 FindPattern 解析流程

1. 查找 `g_moduleMaps[module]` 中 `Fnv1aHash(funcName)` 对应条目
2. 若有 `rva` → 模块基址 + rva → 直接返回
3. 若有 `sig` → ParseSig 转字节/掩码 → ScanModule 线性扫描
4. 未找到 → 记录至 `g_missingFunctions`

---

## 7. Hook 系统

### 7.1 宏体系 (HookMacros.h)

| 宏                                    | 作用                                |
| ------------------------------------- | ----------------------------------- |
| `HOOK_FUNC(name, ret, ...)`           | 声明 typedef + 原始指针 + hook 函数 |
| `HOOK_BEGIN()` / `HOOK_END()`         | Detour 事务边界                     |
| `INSTALL_HOOK(module, name)`          | FindPattern → Attach                |
| `INSTALL_HOOK_C(name)`                | steamclient 模块简写                |
| `INSTALL_HOOK_U(name)`                | steamui 模块简写                    |
| `RESOLVE_FUNC(name, ret, ...)`        | 仅解析不 hook（调用用）             |
| `RESOLVE_C(name)` / `RESOLVE_U(name)` | 模块简写                            |

### 7.2 HookManager 编排

```cpp
SteamClient::CoreHook() {
    Hooks_CallBack::Install();     // 回调系统
    Hooks_Decryption::Install();   // 解密密钥注入
    Hooks_IPC::Install();          // IPC 消息拦截
    Hooks_Manifest::Install();     // Manifest 绑定
    Hooks_Misc::Install();         // 杂项（AppId 捕获等）
    Hooks_NetPacket::Install();    // 网络包拦截
    Hooks_Package::Install();      // 包/许可证注入（核心解锁）
}

SteamUI::CoreHook() {
    Hooks_SteamUI::Install();      // UI 层钩子
}
```

注意 `Hooks_KeyValues` 被注释掉了——表明已被 `Hooks_Manifest` 的 `BuildDepotDependency` 方案取代。

---

## 8. 核心解锁机制 (Hooks_Package)

这是 OST 最关键的模块。

### 8.1 原理

Steam 通过 `CheckAppOwnership()` 判断用户是否拥有某 app。OST hook 此函数：

```plaintext
CheckAppOwnership(appId) 被调用:
├─ 若 appId 在 Lua 配置中 且 用户实际不拥有:
│   → 伪造 AppOwnership 结构:
│       PackageId = 0 (注入包)
│       ReleaseState = Released
│       bOwnsLicense = true
│       bFreeLicense = false
│   → return true
├─ 若 appId 在 Lua 配置中 且 用户实际拥有:
│   → MarkOwned(appId)，后续排除
│   → 正常返回
└─ 其他情况:
    → 调用原始函数
```

### 8.2 假许可证注入

OST 创建一个"假的" PackageId=0 许可证，将所有 Lua 配置的 AppId 注入其 AppIdVec：

```plaintext
InitFakeLicenseOnce():
1. GetPackageInfo(packageId=0)       获取包信息结构
2. CUtlMemoryGrow()                  扩展 AppId 数组
3. 批量写入所有 Lua 配置的 AppId
4. MarkLicenseAsChanged(packageId=0) 触发 Steam 刷新
5. ProcessPendingLicenseUpdates()    处理更新
```

### 8.3 热重载增量更新

`NotifyLicenseChanged()` 处理 Lua 文件变更后的增量更新：

- 移除 unload 的 depot → `FindAndFastRemove()`
- 添加新 depot → `CUtlMemoryGrow()` + 写入
- 触发 Steam 刷新许可证
- 通知 SteamUI 层更新/移除界面元素

---

## 9. IPC 拦截系统

### 9.1 架构

```plaintext
IPCProcessMessage hook
├─ HandleHandshake()      — 捕获客户端 PID
└─ ResolveDispatch()      — 解析 IPC 消息类型
    ├─ 非目标消息: 透传
    └─ 匹配 handler:
        1. handler->pre()   修改请求
        2. 原始处理
        3. handler->post()  修改响应
```

### 9.2 Handler 注册机制

- 各子模块 (ISteamUser, ISteamUtils) 注册 `IPCHandlerEntry` 数组
- `RegisterHandlers()` 将 entry 与 `IPCLoader::Method` 元数据关联
- 运行时通过 `(interfaceID, funcHash)` 快速查找 handler

### 9.3 IPCLoader

从远端 TOML（同 Pattern 机制）加载 IPC 方法元数据：

- `interfaceID` — Steam 内部接口枚举
- `funcHash` — 方法哈希
- `fencepost` — 栅栏值
- `argc` — 参数数量

由 `tools/ipc_codegen/` 生成 `IPCMessages.gen.h`（枚举/解析器）。

---

## 10. 网络包拦截 (Hooks_NetPacket)

### 10.1 数据包格式

```plaintext
┌────────────────┬──────────────┬──────────────┐
│ MsgHdr (8B)    │ Proto Header │ Proto Body   │
│ eMsg|flag      │ (variable)   │ (variable)   │
│ headerLength   │              │              │
└────────────────┴──────────────┴──────────────┘
```

`kMsgHdrProtoFlag` 标识 Protobuf 格式消息。

### 10.2 拦截的关键消息

| EMsg / Job Name                                   | 方向 | 作用              |
| ------------------------------------------------- | ---- | ----------------- |
| `CMsgClientPICSProductInfoRequest` (8903)         | 出站 | 注入访问令牌      |
| `FamilyGroupsClient.NotifyRunningApps#1`          | 入站 | 家庭共享绕过      |
| `Player.GetUserStats#1`                           | 入站 | 成就 SteamID 欺骗 |
| `ContentServerDirectory.GetManifestRequestCode#1` | 入站 | Manifest 码获取   |

### 10.3 Ring Buffer Pool

使用固定大小 (8 槽) 的环形缓冲区池管理修改后的数据包，避免动态分配：

- `g_RecvPacketPool[8][65536+1024+8]` — 入站
- `g_SendPacketPool[8][同上]` — 出站

---

## 11. Pipe 系统

### 11.1 作用

Steam 通过命名管道与游戏进程通信。OST 在 handshake 阶段介入：

```plaintext
游戏进程启动 → Steam 收到 Handshake IPC
→ PipeManager::OnHandshake()
  → ProcessInspector 检查进程信息
  → 缓存 ProcessSnapshot
  → DenuvoAuth::Apply()    — Denuvo 授权流程
  → Injection::Apply()      — 可选 DLL 注入
```

### 11.2 ProcessInspector

- 通过 PID 获取进程快照（可执行路径、创建时间、AppId）
- 使用 `(PID, CreationTime)` 作为 ProcessKey 防止 PID 重用问题
- 结果缓存，同一进程多管道复用

### 11.3 DenuvoAuth

针对 Denuvo 加密游戏的授权流程：

```plaintext
Stage::None → (检测到 Denuvo) → Stage::Authorizing
→ (handshake 计数 >= 2) → Stage::EndAuthorization
   → WriteSteamIdOnEndAuthorization()  持久化 SteamID
```

- `ProtectionScan` 检测游戏进程是否使用 Denuvo
- 授权窗口内的管道可以使用伪造身份
- SteamID 从注册表活跃用户获取并写入凭据存储

### 11.4 Injection

可选的游戏进程 DLL 注入：

- 由 `[inject]` 配置控制
- 检测目标架构 (x64/x86) 选择对应 DLL
- 使用 `RemoteProcess::InjectLibrary()` 注入
- ProcessKey 去重防止重复注入

---

## 12. 解密密钥注入 (Hooks_Decryption)

Hook `ConfigStoreGetBinary`，拦截 Steam 读取 depot 解密密钥的请求：

```plaintext
请求 KeyName = "...\<DepotId>\DecryptionKey"
├─ DepotId 在 Lua 配置中有密钥:
│   → 直接返回配置的密钥字节
└─ 否则:
    → 调用原始函数
```

同时提供 `GetCacheAppOwnershipTicket()` 用于从 ConfigStore 读取缓存的 AppTicket。

---

## 13. Manifest 绑定 (Hooks_Manifest)

Hook `BuildDepotDependency`——Steam 构建 depot 下载列表时调用：

```plaintext
原始函数返回后:
├─ 遍历 pDepotInfo 中每个 DepotEntry
├─ 若 DepotId 在 ManifestOverrides 中:
│   → 替换 ManifestGid 和 ManifestSize
└─ 返回
```

这使 Steam 下载用户指定版本的 manifest，实现版本锁定。

---

## 14. 构建系统与工具链

### 14.1 CMake 模块

| 模块                 | 职责                         |
| -------------------- | ---------------------------- |
| `Detours.cmake`      | FetchContent 拉取 Detours    |
| `Lua.cmake`          | FetchContent 拉取 Lua        |
| `Protobuf.cmake`     | FetchContent 拉取 Protobuf   |
| `Spdlog.cmake`       | FetchContent 拉取 spdlog     |
| `Tomlplusplus.cmake` | FetchContent 拉取 toml++     |
| `LogMacros.cmake`    | 从 LogModules.def 生成日志宏 |

### 14.2 IPC Codegen

`tools/ipc_codegen/` 读取 `Steam/IPCMessages.steamd`（自定义 DSL），生成：

- `IPCMessages.gen.h` — 接口枚举、消息解析器

### 14.3 extract_tickets

独立工具，从已登录 Steam 提取 AppTicket/ETicket 用于 Denuvo 游戏配置。

---

## 15. 数据流全景

```plaintext
                    ┌─────────────┐
                    │  .lua 文件   │
                    └──────┬──────┘
                           │ ParseFile
                           ▼
              ┌──────────────────────────┐
              │   LuaConfig (内存状态)    │
              │  DepotKeySet / Tokens /  │
              │  ManifestOverrides /...  │
              └────┬────────┬────────┬──┘
                   │        │        │
     ┌─────────────┘        │        └──────────────────┐
     ▼                      ▼                           ▼
┌─────────────┐    ┌────────────────┐         ┌────────────────┐
│Hooks_Package│    │Hooks_Decryption│         │Hooks_NetPacket │
│ 包注入       │    │ 解密密钥提供     │         │ 令牌注入/成就    │
│ 所有权伪造    │    │                │         │ 欺骗/manifest   │
└─────────────┘    └────────────────┘         └────────────────┘
     │                                                  │
     └──────────────────┐    ┌──────────────────────────┘
                        ▼    ▼
              ┌──────────────────────────┐
              │      Steam 内部逻辑       │
              │  (steamclient64.dll)     │
              └──────────────────────────┘
```

---

## 16. 对 kazeusa（咱 DLCTool v2.0） 的启示

| 维度        | OST 的做法                    | kazeusa 可借鉴/需注意                      |
| ----------- | ----------------------------- | ------------------------------------------ |
| 配置格式    | Lua DSL (灵活但需学习)        | 考虑提供 GUI 生成 Lua 或更友好的 JSON/YAML |
| 版本适配    | 远端 Pattern TOML + SHA-256   | 依赖上游 steam-monitor 仓库更新节奏        |
| 热重载      | FileWatcher + 增量更新        | 极佳 UX，kazeusa 可直接受益                |
| 注入方式    | DLL 劫持 (无需管理员)         | 轻量且稳定，无 anticheat 兼容性问题        |
| IPC 架构    | 中间人模式 (pre/post handler) | 可扩展性强，适合添加新功能                 |
| Denuvo 支持 | 授权窗口 + 凭据持久化         | 复杂但有效，需要用户配合提取 ticket        |

---

## 附录 : 文件索引

| 路径                                        | 职责               |
| ------------------------------------------- | ------------------ |
| `src/dllmain.cpp`                           | 核心初始化         |
| `src/dwmapi/dwmapi.cpp`                     | DLL 劫持入口 A     |
| `src/xinput1_4/xinput1_4.cpp`               | DLL 劫持入口 B     |
| `src/Hook/HookManager.cpp`                  | Hook 编排          |
| `src/Hook/HookMacros.h`                     | Hook 宏基础设施    |
| `src/Hook/Hooks_Package.cpp`                | **核心解锁逻辑**   |
| `src/Hook/Hooks_IPC.cpp`                    | IPC 拦截框架       |
| `src/Hook/Hooks_NetPacket.cpp`              | 网络包拦截         |
| `src/Hook/Hooks_Manifest.cpp`               | Manifest 绑定      |
| `src/Hook/Hooks_Decryption.cpp`             | 解密密钥注入       |
| `src/Pipe/PipeManager.cpp`                  | 管道管理核心       |
| `src/Pipe/Features/DenuvoAuth/`             | Denuvo 授权        |
| `src/Pipe/Features/Injection/`              | 游戏进程注入       |
| `src/Utils/Config/Config.cpp`               | TOML 配置          |
| `src/Utils/Config/LuaConfig.cpp`            | Lua 配置引擎       |
| `src/Utils/SteamMetadata/PatternLoader.cpp` | Pattern 动态适配   |
| `src/Utils/SteamMetadata/RemoteToml.cpp`    | 远端 TOML 获取     |
| `src/Utils/SteamMetadata/IPCLoader.h`       | IPC 元数据加载     |
| `src/OSTPlatform/`                          | 平台抽象层         |
| `src/Steam/`                                | Steam 内部类型定义 |

---

> 被太阳烤化掉的风酱 于 2026-07-08 进行了最后的编辑

---

## 附录 B : 2026-11 社区现状补充（issue / PR 调研）

> 数据来源：2026-11-04 通过 GitHub REST API 只读拉取 OpenSteam001/OpenSteamTool 全部 203 条 issue / PR，以及 OpenSteam001/steam-monitor 各分支提交记录。正文 §1–§16 的源码分析不受影响，本附录只补充运行时现状与架构层面的教训。

### B.1 仓库状态

| 指标                          | 数值                                                    |
| ----------------------------- | ------------------------------------------------------- |
| Star / Fork                   | 2562 / 439                                              |
| Issue                         | 152 条（68 open / 84 closed）                           |
| PR                            | 51 条（12 open / 23 merged / 16 关闭未合并）            |
| 最后一次合并                  | #153 Cloud RPC 时序修复 + stats 同步，2026-07-06        |
| 最后一个 Release              | 1.4.8，2026-06-13                                       |
| 维护者回复                    | 最近 100 条评论中无 OWNER / MEMBER / COLLABORATOR       |
| steam-monitor `pattern` 分支  | 最后更新 2026-09-06（stable / beta 1788652215）         |
| steam-monitor `ipc` 分支      | 最后更新 2026-09-26（beta 1790380355）                  |

7 月后关闭的 issue 经抽查均由提问者自行关闭，不是维护者处理。

**阶段划分**：

- 2026-05 ~ 06：高速迭代，1.4.0 → 1.4.8 共 10 个 Release，合入热重载删除、远端 Pattern、Denuvo、Injection、stats API、CloudRedirect。
- 2026-07 ~ 08：基本停滞，仅合入 #153，PR 积压。
- 2026-09 至今：下文 B.2 的三类故障相继出现，主仓库无响应。

### B.2 当前三类故障

#### ① Pattern / IPC TOML 断供

steam-monitor 机器人停更后，新 Steam 构建没有对应的 `<sha256>.toml`。表现为启动弹窗 `signature file not found` / `IPC spec missing`，对应 DLL 的钩子被禁用，`addappid` 失效（§6.2 第 7 步的降级路径）。

| Steam BuildID | 首次出现           | 相关 issue        |
| ------------- | ------------------ | ----------------- |
| 1791918634    | 2026-10-14         | #208、#209、#213  |
| 1792618285    | 2026-10-21（stable）| #226、#227        |
| 1793235415    | 2026-10-30         | #224              |

同类报告还有 #205（BuildID 1788652215）、#214、#216、#223。#227 中有用户用 Ghidra 手工逆向出 1792618285 的 IPC spec，steamclient / steamui 的 pattern 仍缺。

#### ② CNetPacket 结构体布局变化导致启动崩溃

自 1791918634 起，`CNetPacket::m_pubData` 前多出 8 字节，其后字段整体后移：

| 字段                 | 旧偏移 | 新偏移 |
| -------------------- | ------ | ------ |
| `m_pubData`          | +0x08  | +0x10  |
| `m_cubData`          | +0x10  | +0x18  |
| `m_cRef`             | +0x14  | +0x1C  |
| `m_pubNetworkBuffer` | +0x18  | +0x20  |
| `m_pNext`            | +0x20  | +0x28  |

`RecvPkt` 钩子按旧偏移读取，触发访问违例。补 TOML 无法修复，必须改源码。PR #212 给出根因与补丁，#218 也包含同一修复，均未合并。

#### ③ 自动 manifest request code 失效

2026-09-09 起 Valve 在服务端对 request code 校验 depot 所有权。内置三个上游（opensteamtool / steamrun / wudrm）全部返回 `Access Denied`，客户端表现为 "No Internet Connection"（#195、#197、#194）。`manifest.opensteamtool.com` 同时返回 Cloudflare 403。社区替代 ManifestDeX（PR #200）后端闭源，且多次因请求过载不可用。

### B.3 未合并的 PR

| PR   | 内容                                                                 | 备注                                             |
| ---- | -------------------------------------------------------------------- | ------------------------------------------------ |
| #218 | 签名源改为第三方仓库 + 修复 CNetPacket 崩溃                          | 作者自述有风险；Codex 自动审查存在未解决问题     |
| #212 | 修复 CNetPacket +8 偏移                                              | 根因分析最完整                                   |
| #204 | manifest 多上游自动切换：失败源冷却 60 s，单次请求总时长上限 11 s    | 上游本身已失效                                   |
| #201 | `[manifest] url` 支持自定义 URL 模板 + `format` 解析器              | 向后兼容                                         |
| #200 | 新增 ManifestDeX 上游                                                | 依赖闭源服务                                     |
| #178 | 递归加载 `config/lua` 子目录（`WatchSubtree=TRUE`）                  | 改动小                                           |
| #148 | 修复 Denuvo 88500012 的三种失败模式                                  | 新能力均需 Lua 显式开启                          |
| #146 | Injection 多库 + 条件注入；OnlineFix 支持已拥有游戏；修复 P2P 游戏  | 作者称仍在修问题                                 |
| #144 | 未拥有游戏的 Rich Presence 广播给好友（借用 appid 480）             | 作者为 CONTRIBUTOR                               |
| #121 | ProtectionScan 不再对主 exe 套用 80 MB 大小下限                      | 对应 #120（狙击精英4 主程序 70.97 MB 被漏检）    |
| #173 | README 补充云存档配置说明                                            | 仅文档                                           |
| #74  | 便携模式                                                             | 注入器部分缺失                                   |

### B.4 其他长期 open 的主题

- **Denuvo**：#139（88500012）、#162；关闭未合并的 #128（SteamID 偏移）。
- **OnlineFix**：#52、#181（邀请好友崩溃）、#185（自定义 appid）、#158（已拥有游戏无法启用）。
- **云存档**：#96、#113（CloudRedirect 兼容）、#116（云同步报错）。
- **跨平台**：Linux（#142、#166）、macOS（#165）未实现。
- **体验类**：#160 首次启动约 1 分钟后才显示入库游戏；#152 `steam://launch` 卡顿；#127、#124 显示延迟。
- **安全**：#222 下载被 Windows Defender 报毒。

### B.5 社区分叉与安全风险

主仓库停摆后出现大量 fork 与替代签名源（madoiscool/BetterSteamTools、JYM34/steam-monitor、matheeshapathirana netfix2、isxlan0、AllRange1337 等），评论区还有用户直接分发编译好的 DLL 压缩包。这些二进制来源无法验证，且会被注入 `steam.exe`。实测时只使用自行从源码编译的版本，签名 TOML 需人工核对。

### B.6 对正文的修正与补充

1. **§6.2 镜像链**：README 说明配置自定义 `[remote] url_template` 后会**替换**内置的 GitHub raw / jsDelivr，不是追加在其后；本地缓存回退保留。
2. **§6 Pattern 系统的覆盖范围**：远端 TOML 只适配函数地址（RVA / sig）。`CNetPacket` 等结构体布局硬编码在源码中，Steam 改布局即崩溃（B.2 ②）。另有字段偏移被嵌在 sig 字节串里，例如 `GetAppIDForCurrentPipe` 的 `0xD30 → 0x10D0`，sig 一失配就连带失效。
3. **§10.2 `GetManifestRequestCode` 拦截**：客户端拦截链路仍然有效，但上游 request code 已被服务端封堵（B.2 ③），客户端钩子无法解决。
4. **§16「依赖上游 steam-monitor 更新节奏」**：该风险已发生。单一机器人 + 单一仓库构成单点故障。

### B.7 对 kazeusa 的架构启示（追加）

| 维度         | OST 暴露的问题                         | kazeusa 设计建议                                                     |
| ------------ | -------------------------------------- | -------------------------------------------------------------------- |
| 结构体偏移   | 硬编码在源码，Steam 改布局即崩溃       | 偏移与函数签名一起作为远程元数据下发，带版本号与校验                 |
| 元数据来源   | 单一 steam-monitor 仓库，停更即全面失效 | 支持多个签名源按优先级切换，并允许用户本地覆盖                       |
| 失配降级     | 只能弹窗 + 禁用整个 DLL 的钩子         | 按功能粒度降级，读取前校验偏移合理性，失败时拒绝安装钩子而不是崩溃   |
| manifest 获取 | 依赖第三方 request code 服务           | 视为不可控的外部依赖，优先支持本地 `.manifest` 导入流程              |
| 二进制分发   | 社区靠来路不明的 DLL 续命              | 提供可复现构建与发布签名 / 哈希，方便用户自行校验                    |

---

> 架构军师·风风酱 于 2026-11-04 追加附录 B
