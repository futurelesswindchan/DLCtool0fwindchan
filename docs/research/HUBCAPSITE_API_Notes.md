# Hubcap Manifest 接入指南

# Hubcap Manifest 接入指南

> 实测日期：2026-07-28　样本：ARK: Survival Ascended (2399830)、Street Fighter 6 (1364780)
> 账户档位：Discord 认证登录后免费档 25/天。国内访问需代理。
>
> 本文中的事实按三级标注：**实测**（已实际请求验证）、**仅文档**（只见于该站文档页，
> 未实际调用）、**未知**（尚未研究）。区分这三级是为了避免把推测当依据写进代码。

## 一、定位

它是本工具目前能获取的最完整清单来源，代价是需要用户自备凭据。

架构地位**与本地导入等同**——可选增强，不是底层主逻辑。未配置凭据时整条链路静默跳过，
MAU 仍是默认路径。这条边界不能松动：免凭据可用是底线。

## 二、认证

```http
Authorization: Bearer smm_xxxxxxxx...
```

凭据形如 `smm_` + 96 位十六进制。

文档另称支持 `X-API-Key` 头与 `api_key` 查询参数。**只用 `Authorization` 头**——
查询参数会进入服务端与各级代理的访问日志。

### 401 可区分两种情形（实测）

| detail                                                  | 含义               | 应给出的引导       |
| :------------------------------------------------------ | :----------------- | :----------------- |
| `API key required. Provide via Authorization header...` | 未配置             | 引导填写           |
| `API key not found or expired`                          | 已失效 / 已 revoke | 引导续期或重新生成 |

这个区分决定文案：两种情形的处置完全不同，只看状态码写不出正确提示。

## 三、端点清单

### 实测确认

| 端点                        | 额度     | 返回                                                                              |
| :-------------------------- | :------- | :-------------------------------------------------------------------------------- |
| `/api/v1/health`            | 免       | 三组件（redis / database / manifest_storage）状态                                 |
| `/api/v1/user/stats`        | 免       | 见下                                                                              |
| `/api/v1/status/{app_id}`   | 免       | 见下                                                                              |
| `/api/v1/search?q=&limit=`  | 免       | `game_id` / `game_name` / `header_image` / `manifest_available` / `uploaded_date` |
| `/api/v1/lua/{app_id}`      | **计 1** | `text/plain` 裸 lua，**`setManifestid` 数量为 0**                                 |
| `/api/v1/manifest/{app_id}` | **计 1** | `application/zip`，含 lua + 全部 manifest                                         |

> 反直觉但关键：取 lua 要用 `/manifest` 而非 `/lua`。`/lua` 返回的脚本缺 `setManifestid`，
> 不满足格式契约第二条。

`/user/stats` 关键字段：

```json
{
  "username": "...",
  "daily_usage": 3,
  "daily_limit": 25,
  "role_daily_limit": 25,
  "custom_api_limit": null,
  "using_custom_api_limit": false,
  "can_make_requests": true,
  "api_key_usage_count": 0,
  "api_key_expires_at": "2026-08-03T15:50:50.528769"
}
```

`/status/{app_id}` 关键字段：

```json
{
  "app_id": "2399830",
  "game_name": "ARK: Survival Ascended",
  "status": "available",
  "manifest_file_exists": true,
  "file_size": 11865729,
  "file_modified": "2026-07-25T15:35:45.396818",
  "file_age_days": 1.9767,
  "needs_update": false,
  "update_reason": "manifest_current",
  "update_in_progress": false,
  "auto_update_enabled": true
}
```

### 仅文档（未实测）

| 端点                                              | 说明                                       | 备注                      |
| :------------------------------------------------ | :----------------------------------------- | :------------------------ |
| `/api/v1/library?limit=&offset=&search=&sort_by=` | 免额度，分页浏览全库                       | **禁止使用**，见第七节    |
| `/api/v1/lua/basegame/{app_id}`                   | 只取本体段                                 | 各计 1 次，取全量反而更省 |
| `/api/v1/lua/dlc/{app_id}`                        | 只取 DLC 段                                | 同上                      |
| `/api/v1/depot-keys`                              | 免额度，只返回 depot ID 列表，**不含密钥** | 价值有限                  |
| `/api/v1/manifest/{app_id}?force_update=true`     | 服务端先刷新再返回                         | 或对「检查更新」有用      |
| `/api/v1/manifest/{app_id}?content=`              | 「备用内容选择器」                         | 语义不明                  |

### 未知

文档的 **Generation 标签页**尚未研究。网页端有 Single Manifest / App Bundle /
Workshop Item 三种生成器，是否有对应 API 未知。**Workshop 清单**这条线完全没探过——
若将来要支持创意工坊内容，这里是入口。

## 四、与 MAU 的能力对照

| 本工具的需求                      | Hubcap                           | MAU                |
| :-------------------------------- | :------------------------------- | :----------------- |
| 完整 DLC 列表                     | ARK **19 个**                    | 4 个               |
| `setManifestid`（格式契约第二条） | **13 个齐全**                    | 1 个（仅主 Depot） |
| 独立 Depot 的判定                 | 注释段落显式分组                 | `packagedlcs` 字段 |
| 游戏名                            | lua 注释第 2 行                  | 需查商店           |
| DLC 名称                          | lua 行尾注释                     | 只有数字 ID        |
| 收录检测                          | 专用免额度端点                   | HEAD 猜            |
| 清单新鲜度                        | `file_age_days` + `needs_update` | 无                 |
| 清单获取时间                      | `file_modified`                  | 无                 |

最后两项落实了「过期检测改按需检查」那条决策——原先只能含糊说「仓库有更新」，
现在能说「清单为 2 天前生成，已是最新」。

## 五、`/manifest` 产出的 zip 结构

```
2399830_ARK%3A_Survival_Ascended.zip   ← Content-Disposition 用 filename*=utf-8''
├── 2399830.lua                           (3948 B)
├── 2399831_2402962259560058676.manifest  (9.7 MB)  ← 主 Depot
├── 2827030_534670506882454568.manifest
├── 228989_5753583882400741046.manifest             ← 共享 Depot（VC Redist）
└── ...共 13 个 manifest
```

manifest 文件名规律与 MAU 一致：`<depotID>_<manifestID>.manifest`。

**包内 manifest 解析完即可丢弃**——OST 会自行下载（见 OST_Integration_Notes.md
第三节的三级回退），v2.0 已解耦这一环。留着只是浪费磁盘。

### lua 的注释结构（`buildDLCNameMap` 依赖它取名称）

```lua
-- 2399830's Lua and Manifest Created by Hubcap Manifest
-- ARK: Survival Ascended                    ← 游戏名在第 2 行
-- Created: July 25, 2026 at 16:35:44 EDT
-- Website: https://hubcapmanifest.com/
-- Total Depots: 13
-- Total DLCs: 19
-- Shared Depots: 2

-- MAIN APPLICATION
addappid(2399830, 1, "b1ea599f...") -- ARK: Survival Ascended
-- MAIN APP DEPOTS
addappid(2399831, 1, "320e0bcc...") -- Depot 2399831
setManifestid(2399831, "2402962259560058676", 227596549042)
-- SHARED DEPOTS (from other apps)
addappid(228989, 1, "ad69276e...") -- VC 2022 Redist (Shared from App 228980)
setManifestid(228989, "5753583882400741046", 25674515)
-- DLCS WITH DEDICATED DEPOTS
-- ARK The Center Ascended (AppID: 2827030)
addappid(2827030)
addappid(2827030, 1, "ca47571a...") -- ARK The Center Ascended - Depot 2827030
setManifestid(2827030, "534670506882454568", 9667911831)
-- DLCS WITHOUT DEDICATED DEPOTS
addappid(2881150) -- ARK Bobs Tall Tales
```

三点值得注意：

1. **DLC 名称出现两次**——前置注释 `-- <名称> (AppID: xxx)` 与行尾
   `-- <名称> - Depot xxx`。`buildDLCNameMap` 命中任一种即可
2. **有「共享 Depot」概念**——VC Redist、DirectX 这类运行库，标注
   `Shared from App 228980`。它们进 `Depots` 段正常输出，无需特殊处理
3. **密钥值与 ARCHITECTURE 5.1 的示例逐字一致**（`ca47571a...`、
   `setManifestid(2827030, ..., 9667911831)`），说明文档里那些示例当年就出自此源

**该源的 `setManifestid` 是三参数形态**，第三参数为 depot 内容总大小。这点与
GitHub 快照类源（两参数）不同，是解析器必须兼容两种参数个数的原因。

## 六、额度经济学

### API 档位

| 来源                              | 额度                                       |
| :-------------------------------- | :----------------------------------------- |
| Discord 等级（Sophie→Solus 八档） | 25 / 30 / 35 / 40 / 45 / 50 / 55 / 60 每天 |
| 订阅 $5~$25/月                    | 50 / 100 / 200 / 400 / 800 每天            |
| 买断 $125 / $500                  | 1000 每天 / 无限                           |

支付走 Discord 人工，无在线结算，到账约 24 小时。

### 网页端额度远高于 API

| 通道                 | 额度        |
| :------------------- | :---------- |
| 网页 Single Manifest | **1500/天** |
| 网页 App Bundle      | **100/天**  |
| 网页 Workshop Item   | **500/天**  |
| **API（免费档）**    | **25/天**   |

网页 App Bundle 是 API 的 4 倍，Single Manifest 更是 60 倍。这解释了 1.4 时期
「先由个人下载再分发给用户」这一做法的成因——它并非临时权宜，而是额度结构的必然结果。

**由此得出的设计约束：本地导入不该被降级。** 它是「有能力的用户 / 分发者」的落点，
吞吐量远高于 API。设计上应保持它是一等入口，而非「API 失败时的退路」。

### 省额度的原则

- 收录检测、账户状态、健康检查全走免费端点，可放心调用（实测 7 次调用后
  `daily_usage` 仍为 0）
- 计费端点只在用户明确要入库时打，且**只打 `/manifest`**
- 撞额度时用 `can_make_requests` 提前判断，不必等 429

## 七、硬约束

**① 绝不遍历 `/api/v1/library`。**

该站文档页顶部有红框警告：

> **WARNING: DO NOT SCRAPE** — Scraping our database will result in a permanent
> ban without refund.

`/library` 免额度、支持分页、一页 100 条，看着诱人，但十余万条全拉一遍正是它定义的
爬库行为。所幸架构早已移除 `FetchRepoList`——**搜索是唯一在线入口**，本就不需要遍历。

**② 绝不内置共享凭据。** 两条理由各自足够：分发的 exe 里的密钥必然被 `strings`
提取；数千用户共用一份凭据的流量特征与爬库无法区分，被封的会是凭据持有者。

**③ 认证请求不走镜像链。** 把带凭据的请求交给第三方公益代理转发，等于把用户凭据
交给代理运营方。宁可直连失败。

**④ 日志不输出凭据内容。** 只记「已设置 / 已清除」。

## 八、风险与摩擦点

| 风险              | 说明                                                                 | 现状                                       |
| :---------------- | :------------------------------------------------------------------- | :----------------------------------------- |
| **凭据 7 天过期** | 捐助者 90 天。30 天以内的延期自动批准                                | `MSiteStats.ExpiringSoon` 已备好           |
| 额度低            | 免费档 25/天，批量补装历史游戏会撞墙                                 | 错误文案已写明「UTC 零点重置」             |
| 可达性历史不佳    | 旧域名曾被墙（此即内部沿用「M 站」这一称法的由来），现域名可正常访问 | 基址存于配置而非硬编码，域名再变用户可自改 |
| 需 Discord 账户   | 无法匿名使用                                                         | 属该站策略，无从规避                       |
| 凭据明文落盘      | 存于 `config.json`                                                   | 桌面应用常规做法，但日志必须回避           |

## 九、尚未验证 / 已知不精确

以下各条**未经验证**或**已知有偏差**，不可当作事实依据。

**① 时间戳不是 UTC，而代码按 UTC 解析。**

返回的时间形如 `2026-08-03T15:50:50.528769`，**无时区标识**。lua 注释头写的是 `EDT`。
据凭据生成时刻反推，服务端时间约为 UTC-5，而 `parseMSiteTime`
（`msite_client.go:254`）三个 layout 全部走 `time.ParseInLocation(..., time.UTC)`，
故算出的到期时刻比真实值**早约 5 小时**。

对「剩余不足 3 天」这个阈值判断无实质影响（只是略早提醒），但**若界面要展示精确
到期时刻，必须先修**。

**② DLC 名称解析已确认可命中（2026-07-29 复核）。**

`buildDLCNameMap`（`lua_parser.go:142`）两条正则与本源实际输出对照：

- 格式 1 `--\s*(.+?)\s*\(AppID:\s*(\d+)\)` → 命中 `-- ARK The Center Ascended (AppID: 2827030)`
- 格式 2 `addappid\((\d+)\)\s*--\s*(.+)$` → 命中 `addappid(2881150) -- ARK Bobs Tall Tales`

两种形态均覆盖，且格式 1 优先。实际使用中界面确有正确显示 DLC 名称，与代码分析一致。
原「未验证」的标注可撤销。

> 但格式 2 有一处需留意：它跳过所有以 `--` 开头的行以避开 EXCLUDED DLCS 区域，
> 这依赖「被排除的 DLC 整行被注释」这一形态。若该站改变排除写法，会误收。

**③ `/status` 的 `needs_update` 判据未知。** 它基于什么比较得出、`update_reason`
有哪些取值，都只见过 `manifest_current` 一种。

**④ `force_update=true` 未测。** 是否额外计额度、是否同步等待刷新完成，均不清楚。

## 十、代码接入现状

已实现于 `msite_client.go`：

| 成员                | 职责                                              |
| :------------------ | :------------------------------------------------ |
| `MSiteStats`        | 额度与凭据状态 DTO，含 `ExpiringSoon`             |
| `msiteStatus`       | `/status`，区分「未收录」与「已知但尚未生成清单」 |
| `MSiteAccountStats` | `/user/stats`，未配置凭据时返回 `nil, nil`        |
| `msiteAuthError`    | 401 / 429 / 404 → 可操作的中文说明                |
| `parseMSiteTime`    | 多 layout 兜底解析（**见第九节①的偏差**）         |

`repo_client.go` 中的 `KindAPIZip` 有三处分派：`enabledSources` 跳过无凭据的源、
`probe` 走专用端点、`Fetch` 不走镜像链。

`app.go` 暴露 `SetRepoToken(sourceName, token)` 与 `GetMSiteStats()`。

前端待做：凭据输入框、额度展示、到期横幅。

## 十一、结论

该源的价值在于「完整」，风险在于「有条件」。数据质量上它是唯一能满足格式契约的源
（MAU 连 ARK 的地图 DLC 密钥都没有），但它需要凭据、有额度、会过期、历史上被墙过。

故三源的分工顺序恒定： **MAU 保证「能用」，Hubcap 提供「好用」，本地导入兜住「一定能用」。**
任何让 Hubcap 变成必需品的改动，都在削弱前两条。

配套的用户引导文档（如何申请凭据）是必要投入——门槛只存在于「不知道怎么弄」这一步，
跨过之后收益显著。
