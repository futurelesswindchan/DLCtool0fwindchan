# OST 集成备忘录

> 定位：为 kazeusa 开发提供快速参考的 OST 行为细节备查手册
> 前置阅读：`docs/research/OST_Architecture_Analysis.md`

---

## 一、热重载机制（LuaFileWatcher）

### 源码位置

`src/Utils/Config/LuaFileWatcher.cpp`

### 工作模式

**事件驱动**（非轮询）。底层使用 Windows `ReadDirectoryChangesW` API。

### 时序流程

```plain
文件变更发生
│
▼
ReadDirectoryChangesW 触发事件
│
▼
WaitAny() 返回 Signaled
│
▼
进入防抖窗口（500ms）
│ ← 期间所有后续事件合并到同一批次
▼
防抖超时，批量处理：
├─ 仅处理 .lua 后缀文件（大小写无关）
├─ Added / Modified → LuaConfig::ParseFile(path)
├─ Removed → LuaConfig::UnloadFile(path)
└─ 最终触发：
├─ Hooks\_Package::NotifyLicenseChanged()  → Steam 许可证刷新
└─ CloudRedirectHost::SyncAppSet()        → 云存档同步
```

### 关键参数

| 参数            | 值          | 含义                         |
| --------------- | ----------- | ---------------------------- |
| `kDebounceMs`   | 500ms       | 防抖合并窗口                 |
| WaitAny timeout | 1000ms      | 主循环心跳（仅检查退出标志） |
| 缓冲区大小      | 65536 bytes | ReadDirectoryChangesW 缓冲区 |

### 事件映射

| 平台原始事件     | OST 内部映射 | 处理动作   |
| ---------------- | ------------ | ---------- |
| `Added`          | Added        | ParseFile  |
| `RenamedNewName` | Added        | ParseFile  |
| `Modified`       | Modified     | ParseFile  |
| `Removed`        | Removed      | UnloadFile |
| `RenamedOldName` | Removed      | UnloadFile |

### 防抖合并规则

- 同一文件在 500ms 窗口内多次变更 → 只保留**最后一次** action
- 最终按首次出现顺序处理（保序不重复）
- 示例：文件 A 先 Added 后 Modified → 只执行一次 ParseFile

### 对 kazeusa deployer 的指导

| 做法                        | 结果                                                                    | 推荐？          |
| --------------------------- | ----------------------------------------------------------------------- | --------------- |
| 直接写 .lua                 | 可能先 Added(空) 再 Modified(有内容)，但防抖合并后只 Parse 一次完整内容 | ⚠️ 可行但不完美 |
| 先写 .tmp 再 rename 为 .lua | OST 只收到一次 RenamedNewName，拿到完整文件                             | ✅ 推荐         |
| 先创建空文件，延迟写内容    | 可能被 Parse 到空文件                                                   | ❌ 避免         |

---

## 二、SteamUI 库界面刷新（Hooks_SteamUI）

### 源码位置

`src/Hook/Hooks_SteamUI.cpp`

### 三个钩子

| 钩子函数                         | 目标        | 作用                             |
| -------------------------------- | ----------- | -------------------------------- |
| `FillInAppOverview`              | steamui.dll | 填充游戏概览时注入 PurchasedTime |
| `CSteamUIAppControllerRunFrame`  | steamui.dll | UI 主循环帧——消费移除队列        |
| `BuildCompleteAppOverviewChange` | steamui.dll | 全量快照构建后追加 removed_appid |

### 安装游戏时的刷新链路

```plain
.lua 文件写入 config/lua/
→ LuaFileWatcher 检测（500ms 内）
→ LuaConfig::ParseFile() 注册 depot
→ Hooks\_Package::NotifyLicenseChanged()
→ CUtlMemoryGrow 扩展 Package 0 的 AppIdVec
→ MarkLicenseAsChanged(packageId=0, true)
→ ProcessPendingLicenseUpdates()
→ Steam 重新查询 CheckAppOwnership → hook 返回"已拥有"
→ SteamUI 拉取 AppOverview
→ FillInAppOverview hook 注入 PurchasedTime（= .lua 文件 mtime）
→ 游戏出现在 Steam 库"最近添加"列表 ✓

```

**延迟估计**：文件写入后 ~500ms（防抖）+ 几十毫秒（Steam 内部处理）≈ **不到 1 秒**

### 卸载游戏时的刷新链路

```plain
.lua 文件删除
→ LuaFileWatcher 检测
→ LuaConfig::UnloadFile() 移除 depot
→ Hooks_Package::NotifyLicenseChanged()
→ FindAndFastRemove(appId) 从 Package 0 移除
→ Hooks_SteamUI::QueueRemoval(appId) 入队
→ MarkLicenseAsChangedAndProcessUpdates()
→ 下一帧 CSteamUIAppControllerRunFrame：
├─ 取出 g_pendingRemovals（mutex + swap）
├─ 跳过 IsOwned() 的（用户真实拥有的不动）
├─ GetAppByID → 设置 OwnershipFlags = None
├─ 若 AppState == Uninstalled → 加入 g_removedAppIds
└─ MarkAppChange(appId, AppInfoOrConfig)
→ BuildCompleteAppOverviewChange：
→ 将 g_removedAppIds 追加到 protobuf removed_appid 字段
→ 游戏从 Steam 库消失 ✓

```

### PurchasedTime 机制

```cpp
// FillInAppOverview 中：
pApp->PurchasedTime = LuaConfig::GetPurchaseTime(appId);
```

`GetPurchaseTime` 返回的是**该 AppId 对应所有 .lua 文件中最大的 mtime**（Unix epoch 秒数）。

**对 kazeusa 的意义**：

- 不要刻意修改文件时间戳
- 让系统自动赋予当前时间 → 新安装的游戏自然排在"最近添加"顶部

### 线程安全模型

```plain
FileWatcher 线程 ──QueueRemoval()──→ g_pendingRemovals
                                         │ (std::mutex)
SteamUI 主线程 ←──RunFrame 消费────────────┘

CancelRemoval() 用于处理竞态：
  文件替换(overwrite) = UnloadFile + ParseFile
  → 先 queue removal → 再 cancel removal（因为又 add 回来了）
```

### 对 kazeusa UX 的指导

| 场景     | 用户看到什么                                          | kazeusa 应该提示什么                |
| :------- | :---------------------------------------------------- | :---------------------------------- |
| 安装 DLC | 游戏 <1 秒后出现在 Steam 库                           | "✓ 已安装！游戏已添加到 Steam 库。" |
| 卸载 DLC | 游戏在下一帧从库消失                                  | "✓ 已卸载。"                        |
| 覆盖安装 | 先短暂消失再出现（但实际上 CancelRemoval 会阻止消失） | "✓ 已更新！"                        |

**不需要**提示"请重启 Steam"。

---

## 三、Manifest 下载链路（ManifestClient）

### 源码位置

- `src/Utils/SteamMetadata/ManifestClient.cpp`
- `src/Hook/Hooks_NetPacket.cpp`（拦截 GetManifestRequestCode 网络包）

### 三级回退机制

```plain
FetchManifestRequestCode(manifestGid, appId, depotId)
│
├─ 优先级 1: Lua fetch_manifest_code_ex(app, depot, gid)
│   用户可通过 Lua 脚本完全接管（自定义 API 端点）
│   返回 nil → 降级
│
├─ 优先级 2: Lua fetch_manifest_code(gid)
│   简化版（仅传 gid）
│   返回 nil → 降级
│
└─ 优先级 3: 内置 Provider HTTP API
    由 opensteamtool.toml [manifest].url 选择
    请求失败 → 返回 false → Steam 显示下载错误
```

### 内置 Provider 表

| 名称                    | URL                                             | 协议  | 响应格式 | Parser                           |
| :---------------------- | :---------------------------------------------- | :---- | :------- | :------------------------------- |
| `opensteamtool`（默认） | `https://manifest.opensteamtool.com/{gid}`      | HTTPS | 纯数字   | from_chars                       |
| `wudrm`                 | `http://gmrc.wudrm.com/manifest/{gid}`          | HTTP  | 纯数字   | from_chars                       |
| `steamrun`              | `https://manifest.steam.run/api/manifest/{gid}` | HTTPS | JSON     | 字符串搜索提取 `"content":"..."` |

### HTTP 超时配置

来自 `opensteamtool.toml`，可热重载：

```toml
[manifest]
timeout_resolve_ms = 5000
timeout_connect_ms = 5000
timeout_send_ms    = 10000
timeout_recv_ms    = 10000
```

### 访问令牌注入（Hooks_NetPacket 协作）

```plain
Steam 发出 CMsgClientPICSProductInfoRequest (eMsg 8903)
  → Hooks_NetPacket 出站拦截
  → 遍历请求中的 app 列表
  → 若 app 在 LuaConfig 中且有 addtoken() 配置的令牌
  → 注入 access_token 字段到 Protobuf 消息
  → 修改后的请求发给 Steam 服务器
  → Steam 服务器返回受保护游戏的完整信息（包含 depot 列表）
```

**对 kazeusa 的意义**：

- `addtoken(appid, "token")` 在 .lua 中的作用是让 Steam 能获取到受保护游戏的 depot 信息
- kazeusa 只需原样保留清单包中的 addtoken 调用即可
- 不需要理解 token 的含义，不需要验证 token 是否有效

### Manifest 码的含义

- Manifest 码（ManifestRequestCode）是一个**一次性授权码**，用于从 Steam CDN 下载 depot 的 manifest 文件
- 码有时效性，所以 OST 不缓存，每次下载时实时获取
- kazeusa 完全不需要参与这个流程

### 全部不可达时的行为

```plain
Lua 两个函数都返回 nil / 不存在
  + 内置 Provider API 请求失败（网络问题/API 挂了）
  → FetchManifestRequestCode 返回 false
  → Steam 内部报告下载错误
  → 用户在 Steam 下载界面看到错误提示
```

**这不是 kazeusa 的问题**。kazeusa 的职责到"把 .lua 放对位置"就结束了。

---

## 四、综合结论——kazeusa 的边界

```plain
kazeusa 做什么：
━━━━━━━━━━━━━━
✓ 帮用户获取 .lua 清单文件（在线仓库 / 本地 zip）
✓ 解析 .lua 内容展示 DLC 列表
✓ 把 .lua 原子写入 <Steam>/config/lua/（tmp+rename）
✓ 管理已安装游戏列表
✓ 检测 OST 是否安装（3 个 DLL 存在检查）
✓ 删除 .lua 实现卸载

kazeusa 不做什么：
━━━━━━━━━━━━━━━━
✗ 不下载 manifest
✗ 不管 depotcache 目录
✗ 不写 opensteamtool.toml
✗ 不通知 OST "我放了文件"（OST 自己检测）
✗ 不等待 OST 确认"已加载"
✗ 不负责 API 不可达的情况
✗ 不需要重启 Steam 的逻辑
✗ 不需要 manifest 版本管理
```

---

## 五、lua_parser.go 需要注册的 OST 函数

kazeusa 的 Lua VM 解析器需要识别这些函数（注册为空操作回调，仅提取参数）：

| 函数                                    | 参数                      | kazeusa 需要提取的信息                      |
| :-------------------------------------- | :------------------------ | :------------------------------------------ |
| `addappid(id [,ignored, key])`          | AppId + 可选 64 字符密钥  | AppId（展示用）+ 密钥（原样保留到部署文件） |
| `addtoken(id, token)`                   | AppId + uint64 字符串     | 原样保留到部署文件                          |
| `setmanifestid(depot, gid [,size])`     | DepotId + GID 字符串      | 原样保留到部署文件                          |
| `setappticket(id, hex)`                 | AppId + 十六进制字符串    | 原样保留到部署文件                          |
| `seteticket(id, hex)`                   | AppId + 十六进制字符串    | 原样保留到部署文件                          |
| `setstat(id, steamid)`                  | AppId + SteamID 字符串    | 原样保留到部署文件                          |
| `fetch_manifest_code(gid)`              | GID                       | 仅识别，部署时原样复制函数定义              |
| `fetch_manifest_code_ex(app,depot,gid)` | 三参数                    | 仅识别，部署时原样复制函数定义              |
| `http_get(url [,headers])`              | URL + 可选 headers 表     | 仅识别，部署时原样复制                      |
| `http_post(url, body [,headers])`       | URL + body + 可选 headers | 仅识别，部署时原样复制                      |

**核心原则**：kazeusa 的 lua_parser 是**提取信息用的**，不是执行逻辑用的。所有函数调用和定义在部署时**原样输出到 .lua 文件**。

---

> 被太阳烤化掉的风酱 于 2026-07-08 进行了最后的编辑
