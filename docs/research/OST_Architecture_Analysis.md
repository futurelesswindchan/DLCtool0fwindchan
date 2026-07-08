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
