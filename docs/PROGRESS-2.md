# 开发进度追踪 · 第二卷

> 承接 `PROGRESS.md`。第一卷超过 1000 行后另起本卷，**历史阶段记录仍在第一卷**
> ——查往期实绩、已知残留、OST 源码研究摘要请去那边。
>
> 本卷只记 2026-08-03 之后的进度。每次开发结束时更新。
>
> 最后更新：2026-08-08（第 6 步完工）

---

## ✅ 当前阶段：UI 第 6 步已完工（分支 `feat/ui-v2`）

路线第 5 项（TopBar 指示器 + Logo + 花纹）与第 6 项（术语帮助 + 键盘可达性

- 搜索文案）均于 08-08 完工。当前 ahead 4，待 push。

以下为第 5 步收尾与第 6 步的完整记录，按时间倒序。

### 第 5 步收尾（08-08 早）

08-02 暴露的四个实机问题**全部结清**：

| #   | 问题                           | 结局                                        |
| :-- | :----------------------------- | :------------------------------------------ |
| 1   | 切换游戏详情闪状态 C           | ✅ 08-03 修（连带挖出第五个同源缺陷）       |
| 2   | 切页丢失页面状态与进行中的工作 | ✅ 08-03/04 修（搜索 store + 请求生命周期） |
| 3   | 库界面回不到库概览             | ✅ 08-03 修（入口移位，未新增）             |
| 4   | 搜索频繁超时                   | ✅ 已定性为线路偶发，降级为文案议题         |

## ✅ 08-03/08-04 完工内容

### 搜索状态与请求生命周期移入 store

新建 `stores/search.ts`。原 `SearchPane` 把 `term` / `results` / `searching` /
`searched` 全放组件内 `ref`，切页即销毁，且在途 promise 落到已销毁实例上被
静默丢弃。文件顶部「搜索结果不进 store：属会话级临时数据」的注释已被推翻并
保留备忘。

状态改四值枚举 `SearchStatus`（idle / searching / done / failed）。原因是
`searched` 只在成功路径置真——失败时页面退回初始空态，把「没搜成」伪装成
「没搜过」；且错误只进 toast，切页回来无处可查。**这两条才是「搜索失败体验
差」的结构性根源：没有地方能承载「失败了」，文案无处可落。**

`run()` 带请求序号，过期响应丢弃。新增搜索期间「切页不中断」旁注与失败提示
两处文案。搜索期间输入框一并禁用。

### 库页在本项无欠账（与原计划不同）

路线原写「搜索与库页面状态缓存」并成一项。实读后发现库页是搭便车的：只有
`LibraryShell` 一个 `filter` 是本地状态，而壳在库内切 pane、换 appID 时都不
重建，筛选词本来就保得住；库数据早在 `stores/library.ts` 里。真问题只在搜索。

### 全局文件拖放防线（安全级）

拖任意文件到 `DropZone` 以外的地方会唤出 Windows 透明弹窗，文本类文件还会开
Edge 原生查看器，**其中某些按钮点击后令 WebView2 崩溃、宿主进程一并倒掉**。
发行版同样如此。新建 `guards/fileDropGuard.ts`，window 捕获阶段全拒，
`main.ts` 于 mount 前装上。详见 DECISIONS-3。

### GamePane 四状态改单一枚举裁决

08-03 那次修复自己留下的缺陷：状态 A 是独立 `v-if`，与状态 B/C 那条链互不
知情，两条链同时为真时**两个状态一起渲染**（实测对比表下面跟着一段「本地没有
这个游戏的清单内容」）。改 `GameViewState` 枚举后四分支合入同一条链，
「同时出现两个」在结构上不可能。

顺带保住类型收窄：新增 `activePkg` computed，**没有用 `pkg!` 消音**。

### 详情与留存并发取

`load()` 原先先 await 详情再读留存，读取态遂挂满整个详情请求——实测库页来回
切两个已入库游戏时「正在读取本地清单…」疯狂闪烁。改 `Promise.all`。

### 事件监听移到 load() 之前

`EventsOn('download:progress')` 原本注册在 `await load()` 之后，而未入库游戏
的 load 会走 lookup()（首次可达 41 秒），进度事件恰在这段期间推送——**首屏
那段时间一条进度都收不到**。已修，实测进度文案正常显示且随源切换。

### 待落盘改动记下归属（数据正确性）

`useDlcSelection` 新增 `pendingPkg`。原防护只在 `onUnmounted`，而库页
master-detail 换游戏**根本不触发卸载**——「勾一下就切走」这条主路径上，
800ms 防抖到点时 `pkgRef` 已被置空，改动静默丢失。三处出口（防抖到点、换包、
组件卸载）现各自都能补交。

另查实：原 `onUnmounted` 里的 `void flush()` 并不「立即」，防抖函数再调一次是
重设计时器。它没丢数据纯属侥幸（setTimeout 不随销毁取消）。

### 勾选还原改直读后端

`loadStored()` 原用 `libItem.value?.record ?? await findHistory(...)`，而
`libItem` 来自 library store 缓存——库页切游戏不刷、落盘也不刷，于是取消勾选
后切走再回来读到旧记录。**磁盘上的 lua 与 history.json 一直是对的，是界面读错
了地方。** 已改为一律问后端，并在落盘成功后刷新 library store（侧栏「N 个
DLC」等计数也读它）。

实机复验通过：`history.json` 与 lua 均正确，界面显示正确。

# <<<<<<< Updated upstream

### 类型探查批次超时 12s → 5s

搜索最坏路径 `storeHTTPTimeout`(15s) + `searchTypeProbeTimeout`(12s) = 27 秒，
现降至 20 秒。依据不是「概率低」而是**失败的形态**：341 次采样里故障是块状的
（29 块中 26 块长于 2 分钟），处在块内多等 7 秒同样探不出一条，不在块内则
p95=642ms、两批也才 1.3 秒，这 7 秒从不被触及。**两种情形下长超时都不产生
收益。**

按「改超时要沿请求链把串联的都数一遍」过了三处，另两个不动：
`storeHTTPTimeout` 是单请求上限且注释有实测依据，调小会**改变请求成败**；
`lookupTimeout` 属 `repo_client` 打 codeload，不在本链上，且那批采样一次都
没打过 codeload，无权拿来改它。

本次改的只是「还等不等剩下那几条的判定」，不改变任何一次请求的成败——这是
它可以放心调小的根本原因。后端 117 条 PASS 不变。

## ✅ 08-05 完工内容（UI 第 5 步控件自绘）

### SearchPane 换原语，本页无原生表单控件

原生 `<input>` + 两个原生 `<button>` 换成 `UiInput size="md"` + 两个
`UiButton`（搜索为 primary，清空为 default）。三段原生控件样式整体删除。

字号有个真冲突：原为 `--text-md`(15px)，而 `UiInput` 两档都固定
`--text-sm`(12px)。核实后确认这不是原语疏漏——`UiSelect` 同构，**输入框里是
用户自己的内容，属正文，不随控件外形缩放**。宪法 11.5 又定死「只两档，
不做 lg」。已定为**接受 12px、一致性优先**，「主入口更醒目」改由位置、全宽、
旁边唯一一个 primary 按钮承担（宪法 4.1）。详见 DECISIONS-3。

### GameCard 删 layout prop 与四段 grid 样式

grid 形态自第 3 步改 master-detail 后零调用点。**连 prop 一起删**而非只删
样式：一个两值枚举只被传过一个值，它就不是枚举；留着「接受 grid 但没有对应
样式」比没有这个参数更糟——签名承诺了一种它实现不了的形态。
`.card--row` 的两条并入 `.card`。

### DlcList 换 UiCheckbox（性能实测待跑）

整体替换而非往里塞：原 `.row__label` 自己就是 `<label>`，而 `UiCheckbox`
内部也是 `<label>`，**嵌套 label 是非法 HTML**。

⚠️ 施工中自己引入又修掉一处回归：顺手写的 `:disabled="readonly"` 会让
`.cb--disabled` 把整行降到 50% 透明度，而只读态恰恰是「用户只想读这份列表」
的场景。已改为只读态不渲染勾选框（本组件文案自己写着「因此不提供勾选」）。

### 修掉三处原语自身的缺陷

都不是本页专属，均由「换掉第一个真实调用方」才暴露：

| 原语         | 缺陷                                           | 症状                                          |
| :----------- | :--------------------------------------------- | :-------------------------------------------- |
| `UiInput`    | `autofocus` 靠 attrs 透传，落在 `<div>` 外壳上 | 不报错也不生效                                |
| `UiCheckbox` | `.cb__label` 缺 `min-width: 0`                 | 插槽内 `ellipsis` 永不生效，长 DLC 名撑破整行 |
| `UiButton`   | 缺 `white-space: nowrap`                       | 窄窗口下标签折行，`line-height: 1` 使折行叠字 |

**只有预览页在用的原语，等于没被验证过**——预览页给的是理想输入（短标签、
宽容器、默认档位）。

### 预览页补三格

发现预览页自己的盲区：「输入类」四个 `UiInput` 全是默认 md 档，于是「两档差
在哪」在本页看不出来，而这一页的存在理由正是「一眼看全」。已补：

1. `size="sm"` / `size="md"` 并排（把「两档字号相同是刻意的」变成可见）
2. 长文案截断（窄容器 + 超长 DLC 名），看守 `.cb__label` 那条修复
3. **压力档 · UiCheckbox × 200**，默认关。取 200 是照 MHW(582010) 的实测
   DLC 数定的；刻意不加 `content-visibility`，否则测的是被跳过渲染的行

## 🔧 08-06 修正与实测结清

### UiCheckbox 性能疑虑结清（实机）

MHW(582010) 200 个 DLC：滚动与单条勾选均无明显掉帧；切到 Intel 集成显卡
同样无问题。每行 6→12 元素、整表约 800 个 SVG 节点这个量级不构成性能问题，
预备的退路（svg 换伪元素 / 仅选中行画勾）**无需动用**。

### 压力档白屏——咱自己写反的判据

预览页那个开关一打开就白屏（DOM 完整，纯渲染层失败），而真实 MHW 页无事。
根因是原注释「刻意不加 `content-visibility`」的理由反了：`DlcList` 永远带
那条属性，压力档不加就是在测一个**永不发行的配置**，200 行让区块涨到约
8000px，而 `.gal__sec` 带 `box-shadow`。已补齐该属性与
`contain-intrinsic-size`。详见 DECISIONS-3。

### 发现 `download:progress` 是死代码，08-04 那条结论已被推翻

后端零处 `runtime.EventsEmit`。详情见上方 08-04 段落里的更正块与
DECISIONS-3。**这直接解释了 `UiProgress` 为何至今零调用点。**

## ✅ 08-08 完工内容（UI 第 5 步收尾）

### 品牌标识位（宪法 7.7）

两处品牌位的 emoji `🐰` 替换为 LogoMark 组件——内联 SVG，`stroke="currentColor"`，
颜色随文字色走，双主题一份资产。

⚠️ LogoMark 走了内联 SVG 而非宪法 7.4 推荐的 mask-image 方案，
原因见 DECISIONS-3「WebView2 不支持 mask-image + 本地 SVG URL」，
以及下方的 🩹 新增坑。

### 顶栏页签常驻指示器

`::after` 伪元素 + `@keyframes tab-in`（animation）→ 单一 `.nav-ind` 元素 +
`transform: translateX()` + `transition`（可中断）。

测量逻辑：`navEl` ref → `querySelector('.nav-tab--active')` → `offsetLeft` /
`offsetWidth` → 写入内联 style。`ResizeObserver` 兜住窗口缩放与字体加载后
的重测，`v-show` 保证首次加载指示器不飞入（直接出现在正确位置）。

`#topbar__nav` 补 `position: relative`。`.nav-tab` 原先那条 `position: relative`
已失去用途（`::after` 被删），一并清理。

### 花纹资产与投放

三张 SVG 资产（beans / ear / logo-rabbit）制作完成，图形词汇统一
（栅格 24px · 线宽 1.5px · 圆头 · 无尖角）。投放两处：

| 位置       | 花纹                | 效果                           |
| :--------- | :------------------ | :----------------------------- |
| 侧栏品牌区 | `ear.svg` corner br | 兔耳尖从右下角切出画面外，极淡 |
| 库空态     | `beans.svg` tile    | 咖啡豆+方糖微平铺满铺背景      |

Ornament 组件重写：`mask-image` → 内联 SVG，`src` prop → `pattern` prop
（`'beans' | 'ear'`）。颜色走 `--pattern-ink` 令牌（经 `color` 属性注入），
浓度仍由 `--pattern-alpha` 控制。

空态同时加了居中撑满：`.empty-full`（`flex: 1; align-items: center;
justify-content: center`）让 `UiEmptyState` 垂直居中而非贴顶。

### 预览页

`#/dev/ui` 新增两节：

- **LogoMark · 品牌标识位**：五档尺寸 + 三色境对照（正文色/次要色/主色）
- **Ornament · 装饰花纹**：可调浓度滑条（0.01~0.14）+ 四种角色对照 +
  真实落点预演（空状态 + 角落兔耳）

浓度滑条用的原生 `<input type="range">` 是已知例外（十三个原语里没有 slider，
预览页不进封测包），已在注释标注 `XXX`。

### 🩹 新增坑 · WebView2 mask-image 整体不生效

与本轮 Logo 黑方块 + Ornament 灰方块属同一病根：**WebView2 不支持 `mask-image`
与本地 SVG URL 的组合**（`npm run dev` 在浏览器中正常）。

LogoMark 与 Ornament 统一改走内联 SVG + `stroke="currentColor"`。
参见 DECISIONS-3 08-08 条目。

⚠️ 这对验证流程有实际影响——**视觉效果类改动必须包含 `wails dev` 实跑验收，
仅看浏览器是不够的。** `npm run dev` 验证通过的 mask 效果，在 WebView2 中
完全不呈现。

---

## ✅ 08-08 完工内容（UI 第 5 步剩余三项）

**第 5 步至此整体完工。** UI 第 6 步（术语帮助系统、键盘可达性、搜索空态文案
的 `describeError` 细化）与 `UiProgress` 去留（路线第 7 项）留给后续。

### LOGO 替换（TopBar + HomeShell）

两处品牌位原先的 emoji `🐰` 换成 `LogoMark` 组件（`components/ui/LogoMark.vue`）。
走 mask-image + `currentColor`，字号即尺寸（内部 `width: 1em`），
双主题自动成立。配色由调用方经 `color` 属性控制。

### TopBar 页签常驻指示器

原实现 `.nav-tab--active::after` 用 `animation: tab-in`（scaleX 0→1），
不可中断——宪法 5.4 要求动效可中断。改为常驻 `.nav-ind` 元素，
`transform: translateX() + width` + `transition`，页签切换时是同一物体的
位移而非新元素的登场动画。`ResizeObserver` 兜窗口缩放与字体加载，
`document.fonts.ready` 修正初始测量。

### 花纹资产与投放

三张 SVG 资产落地（`assets/icons/logo-rabbit.svg`、`assets/patterns/beans.svg`、
`assets/patterns/ear.svg`）。投放两处：

- **HomeShell 侧栏品牌区**：兔耳角落纹样（`Ornament role="corner" corner="br"`），
  `overflow: hidden` 裁掉下半部分，只露耳尖
- **LibraryOverviewPane 库空态**：咖啡豆微平铺 + 居中撑满（`.empty-full` 容器
  `flex: 1; align-items: center; justify-content: center`）

预览页 `#/dev/ui` 加两个验证节：LogoMark 五档尺寸 + 三色境对照，
Ornament 浓度滑条 + 四种角色样品 + 真实落点预演（空状态背后放角纹）。

### 🚨 WebView2 下 mask-image 不兼容（本轮最重要的发现）

**结论：WebView2 完全不支持 `mask-image` + 本地 SVG URL。**

现象：在浏览器 `npm run dev` 中一切正常，但在 `wails dev` / `wails build`
中，`mask-image` 的遮罩完全不生效——元素表现为 `background-color` 的
实色填充矩形（LogoMark 是黑色方块，Ornament 是灰色矩形）。

排查过程：

- LogoMark 先被发现黑方块（`currentColor` → 黑），怀疑是 `currentColor`
  继承问题
- 改为内联 SVG（`stroke="currentColor"`）后 Logo 正常，但报告 Ornament
  也出灰矩形——说明不是颜色问题，是 **`mask-image` 整体不生效**
- 两者统一改为内联 SVG：LogoMark 内联路径，Ornament 内联 `<svg>` +
  `<pattern>` 元素

**工程影响**：宪法 7.4 节「一份资产吃两套主题」中推荐的 mask-image 方案
**对 WebView2 不可用**，需改走内联 SVG + `currentColor`。
独立 `.svg` 文件保留为设计源文件，运行时不再加载。

详见 DECISIONS-3 新增的决策 18。

### UI 速查更新

速查从 67 条增至 68 条（新增第 68 条见 UI-ARCHITECTURE.md 底部变更记录）。

---

## 📏 已知残留（更新）

| 项目                                        | 说明                                                                                                                        |
| :------------------------------------------ | :-------------------------------------------------------------------------------------------------------------------------- |
| `ear.svg` / `beans.svg` / `logo-rabbit.svg` | 现为**设计源文件，运行时不再加载**。LogoMark 与 Ornament 均走内联 SVG。若日后改设计，改源文件后需同步更新组件内的内联路径。 |

### ✅ UI 第 6 步 完工（08-08 晚）

#### 术语帮助系统

**新建 `frontend/src/glossary.ts`**——全部界面术语文案的单一事实来源
（宪法 8.1 节）。结构 `key→{term,short,long?}`，含 12 条词条覆盖宪法
列举的六个优先术语：仅本体/不可用/未试取、源、API 额度、注入器/OST、
清单包/本地导入、勾选即保存。额外 `g()` 快捷取值函数。

**投放 `UiHelpBadge` 至四处优先位置**（该组件已于第 2 步完工）：

- `SettingsSources`：标题「清单源」→ glossary.source、凭据标题 → glossary.quota
- `SettingsEnv`：标题「环境」→ glossary.injector（解释盒子与 OST 分工）
- `ImportPane`：标题「本地导入」→ glossary['import-package']
- `DlcList`：标题「DLC 列表」→ glossary['auto-save']

对比表状态（仅本体/不可用/未试取）的 badge 留待后续轮次投放——
`glossary.empty` / `glossary.unsupported` / `glossary.skipped` 三个词条已备好。

#### 键盘可达性审计

逐件确认剩余四件原语：**UiRadio** / **UiSwitch** / **UiSegmented** / **UiInput**
全部通过——real semantic HTML + `:focus-visible` 焦点环 + ≥28px 热区。
UiSegmented `min-height: 24px` 略低 4px，但水平排列的 `<button>` 组宽度充裕，
形态合理。此前已确认的原语（UiButton/UiCheckbox/UiSelect/UiHelpBadge）同样
达标。Tab 序自然：TopBar 页签 → 侧栏条目 → 内容区。

#### 搜索空态与错误文案

**`describeError` 七分支细化**（`stores/search.ts`）：复用 `tools/netprobe` 的
`classify` 逻辑，把 Wails 绑定层透传的 Go 网络错误按七种原因分支——
等响应头超时 / TLS 握手超时 / DNS 失败 / 强制关闭 / TCP 建连 / EOF / 兜底。
每条都说清「是否与本工具有关」（宪法速查第 50 条）。

**`SearchPane` tips 文案改口径**：从「大陆网络通常需要开启加速工具（UU、Steam++
之类均可）」→「偶发失败与本工具无关，稍等再试通常就好」。加速工具仍有提及但
降为补充建议。实测方向对（雷神让建连从 107ms→7ms）但不完整，多数用户不受影响。

#### 文件变更

```
新建  frontend/src/glossary.ts (12 条术语词条 + g() 函数)
修改  frontend/src/stores/search.ts          (describeError 七分支)
修改  frontend/src/views/panes/SearchPane.vue (tips 文案改口径)
修改  frontend/src/views/panes/SettingsSources.vue (两处 HelpBadge)
修改  frontend/src/views/panes/SettingsEnv.vue (一处 HelpBadge)
修改  frontend/src/views/panes/ImportPane.vue (一处 HelpBadge)
修改  frontend/src/components/DlcList.vue     (一处 HelpBadge)
```

前端 verify 绿（type-check + tokens 59 + build 173 modules / 6.1s）。
体积增量：`glossary.js` 4.56 kB (gzip 3.81 kB)。

### 全局核查

按速查第 13 条扫过 `frontend/src/**/*.vue`：`ui/` 之外已无未接管的原生表单
控件（`<input>` / `<select>` / `<textarea>` 归零）。剩余 9 处原生 `<button>`
均为壳层与窗控元素，且 `base/a11y.css` 有全局 `:focus-visible` 兜住，
第 14 条无欠账。

前端 verify 绿。体积变化：`GamePane.css` 6.26→6.13 kB、`GamePane.js`
15.70→16.68 kB。

> > > > > > > Stashed changes

---

## 🔜 后续路线

1. ✅ 搜索状态缓存与请求生命周期（08-03/04）
2. ✅ 拖放防线、四状态枚举、并发取详情、库概览三分支（08-04）
   <<<<<<< Updated upstream
3. UI 第 5 步剩余：`SearchPane` 原生 input/button 换 `UiInput`/`UiButton`、
   `DlcList` 原生 checkbox 性能实测、`GameCard layout="grid"` 零调用点待删
4. UI 第 5 步剩余：`TopBar` 下划线改常驻指示器（transform 滑移才可中断）、
   花纹投放、LOGO
5. UI 第 6 步：术语帮助系统、键盘可达性、**搜索空态与错误文案**
   （`stores/search.ts` 的 `describeError` 已留 TODO，按 `tools/netprobe` 的
   `classify` 七分支细化，并说清「这不是盒子坏了」）
6. `searchTypeProbeTimeout` 从 12s 砍到 5s——**当前最坏路径是 15+12=27 秒**，
   原计划漏了这一项
   =======
7. ✅ `searchTypeProbeTimeout` 12s → 5s，最坏路径 27s → 20s（08-04）
8. ✅ `SearchPane` 换原语、`GameCard` 删 grid、`DlcList` 换 `UiCheckbox`
   （08-05），性能实测已于 08-06 结清（实机 MHW 200 DLC 无掉帧，集显亦然）
9. ✅ UI 第 5 步收尾（08-08 早）：`TopBar` 常驻指示器（transition 滑移）、
   LogoMark 品牌标识（内联 SVG）、花纹资产与投放（Ornament 内联 SVG 重写）
10. ✅ UI 第 6 步（08-08 晚）：术语帮助系统（glossary.ts + 四处 HelpBadge 投放）、
    键盘可达性审计（四件原语全通过）、搜索空态与错误文案（describeError 七分支
    细化 + tips 改口径）
11. **决定 `UiProgress` 的去向**（跨前后端）：它是十三个原语里唯一零调用点的
    一个，目标落点是宪法 5.5 的「试下载占位行 + 逐条就地替换」，而那需要后端
    按源推送事件——后端至今零处 `EventsEmit`。三条路：①后端加推送，
    `UiProgress` 与 `download:progress` 一起激活 ②不做推送，删掉那个死监听，
    `UiProgress` 保留待用 ③先只做「已试 N/7 个源」这种前端可自算的粗粒度进度
    > > > > > > > Stashed changes

## 📌 本卷新增的已知残留

| 项目                         | 说明                                                                                                                                                                                           |
| :--------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 详情页长操作切页             | `install()` / `lookup()` 的 `downloading` / `looking` 仍是组件内 ref。实例复用下本来就活着，故未搬进 store；真卸载路径经实测靠「先缓存再部署」天然安全（复用试下载产物入库，日志确认零新请求） |
| `library.refresh()` 调用频次 | 每次勾选落盘后都刷一次全库（GetHistory + ScanDeployed）。装上百个游戏时可能偏重，尚未实测                                                                                                      |

> 第一卷的已知残留清单仍然有效，本表只列本卷新增项。

---

## 🔜 下轮待办（08-08 列出）

以下由于第 6 步验收后列出。**本轮只记入文档，正式动手在下一轮新对话。**

### 零、全局样式相关

| #   | 项目                       | 说明                                                                                                                                                                                                                                                                        |
| :-- | :------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0.1 | 单左侧边框加厚圆角设计     | 存在于设置页选项、Toast 栏、HelpBadge 展开面板等地。UI 架构书已定性为「几何打架」——3px 直角色条贴在圆角卡片左侧，色条两端是方的、卡片角是圆的，像贴上去的胶带。改为书脊圆角样式（`position: absolute; border-radius: 4px`），并**抽离为独立 UI 组件**，参数支持大小与颜色。 |
| 0.2 | 侧栏 emoji icon            | 目前使用 emoji（📥🔍⚙️ 等），在不同系统上渲染不同且不可控。两条路：①全部换 Font Awesome ②全去掉，仅折叠态保留 icon。待议论决定。                                                                                                                                            |
| 0.3 | 侧栏展开时内容跳动         | 收起→展开时，侧栏内 emoji icon 与文字因为宽度变化，从多行显示慢慢展开为单行。这种跳动非常诡异——必须改掉。                                                                                                                                                                   |
| 0.4 | 侧栏收缩/展开 Y 轴高度固定 | 不管搜索还是展开，每个选项所占的 Y 轴高度应当固定不变，这样操作逻辑才有一致性。                                                                                                                                                                                             |

### 一、Home 页面相关

| #   | 项目                   | 说明                                                                                                                                                                                                             |
| :-- | :--------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1.1 | 侧栏顶部与 TopBar 冲突 | Home 页侧栏顶部有 LOGO + 昵称 + slogan，紧挨着的 TopBar 左侧也有 LOGO + 昵称——严重重复与冲突感。需议论决定修改方式。                                                                                             |
| 1.2 | 搜索栏下方大面积留白   | 选中在线搜索后，右侧只有搜索栏 + 提示文字，中下部大量留白。考虑加入**公告区**：上 70% 写版本更新公告（版本号、日期、更新条款，放 TS 变量不硬编码），下 30% 放防倒卖提示（原作者 + GitHub 链接 + 免费开源提醒）。 |

### 二、Library 页相关

| #   | 项目               | 说明                                                                                                   |
| :-- | :----------------- | :----------------------------------------------------------------------------------------------------- |
| 2.1 | 库空态未占满详情页 | 当前 empty 组件未占满父组件（ContentPane）的全部高度，下半部大面积留白。需让空态占满父组件大部分空间。 |

### 三、HelpBadge 相关

| #   | 项目           | 说明                                                                                                                                    |
| :-- | :------------- | :-------------------------------------------------------------------------------------------------------------------------------------- |
| 3.1 | 点击引导性不足 | 当前图标是 `?` 在圆圈里——用户潜意识里只「悬停」它而不会「点击」它。需在图标设计或 aria-label 上加强引导。                               |
| 3.2 | 悬浮提示 BUG   | 悬浮在 HelpBadge 上时显示的**不是 aria-label，而是正式的 glossary 文案**。需定位是 Tooltip 锚点绑定问题还是 content prop 传递时序问题。 |
