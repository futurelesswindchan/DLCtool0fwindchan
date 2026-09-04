# 决策日志

> 记录每次重大架构/技术决策。
> 格式：日期 + 决策标题 + 背景 + 结论

---

## 2026-09-04：v2.0.0 -> v2.1.0 体验优化技术方案与架构评估

### 背景

v2.0.0 正式版发布后，收集了许多的优化条目与改进方向。

本决策记录架构层面的全面评估结果与最终技术方案。

---

## 核心决策摘要

### ✅ 已确认可立即实施（21 项）

**第一批：快速胜利（1-2 天）**

- Bug #1: GamePane 切换闪烁修复
- Bug #2: 阶段式进度显示（纯前端）
- A1.1: DLC 搜索框
- A1.2: DLC 全选/反选
- C1.2: Hover 微浮起
- C1.4: 加载态点点点动画
- A3.1: 搜索历史
- D1: 首次扫描提示

**第二批：体验完善（3-5 天）**

- C1.1: 成功微庆祝动画
- A4: 批量异常处理
- A5.1: 全局快捷键
- A6: 侧栏折叠态搜索（简化为跳转）
- C2.1: 骨架屏占位
- C3: 状态色增加"进行中蓝"

**第三批：锦上添花（排期待定）**

- C1.3: 数字滚动动画（渐进增强）
- A5.2: 侧栏箭头导航
- C4.3: 空态聚光效果
- A3.2: AppID 识别提示

### ⚠️ 需后端配合（2 项）

- A2.2: 各源实时结果展示（需新增 `source:result` 事件）
- A2.3: 真实进度百分比（需 `download:progress` 增加字段）

### ❌ 不实施

- **A1.3: DLC 分组展示** - 分类标准不明确，A1.1 搜索框已解决核心痛点
- **A3.3: 模糊搜索建议** - 需后端编辑距离算法，收益不足
- **A1.4: DLC 虚拟滚动** - 先实测性能瓶颈再决定

---

## 技术方案详述

### 🚨 紧急修复：两个技术债务

#### Bug #1: GamePane 切换闪烁

**根因：**
`GamePane.html:104` 的 `viewState` 计算时，切换游戏瞬间 `pkg` 被清空但新数据未到，`viewState` 跳到错误状态导致闪烁。

**方案：增加切换标记**

```typescript
// setup 顶层增加
const switching = ref(false);

// watch 回调修改
watch(appID, async (newID, oldID) => {
  if (!newID || newID === oldID) return;

  switching.value = true; // 🆕 置位
  pkg.value = null;
  // ... 其余清理逻辑

  await load();
  switching.value = false; // 🆕 复位
});

// viewState 判据修改
const viewState = computed<GameViewState>(() => {
  if (switching.value) return "loading"; // 🆕 最优先
  if (pkg.value) return "package";
  if (storedLoading.value) return "loading";
  if (!installed.value || reacquiring.value) return "sources";
  return "noPackage";
});
```

**工时：** 1 小时  
**风险：** 🟢 低（单文件改动，不改变既有状态语义）

---

#### Bug #2: 进度条预留但无功能

**现状：** `UiProgress.html` 已实现但从未使用，后端已推送 `download:progress` 事件

**阶段 1：纯前端阶段式进度（立即实施）**

`GamePane.html` 已监听事件（第 169 行），只需在模板显示：

```html
<!-- 源对比表上方插入 -->
<UiProgress
  v-if="looking && progressText"
  :label="progressText"
  class="trial-progress"
/>
```

**工时：** 30 分钟  
**风险：** 🟢 低

**阶段 2：真实进度百分比（需后端）**

后端需提供：

```go
type ProgressPayload struct {
    AppID   string
    Source  string
    Current int  // 当前第几个源
    Total   int  // 总共几个源
}
```

前端改动：

```typescript
const downloadProgress = ref<number>();
const progressLabel = computed(() => {
  if (!progressCurrent || !progressTotal) return progressText.value;
  return `[${progressCurrent}/${progressTotal}] ${progressText.value}`;
});
```

**工时：** 前端 2h + 后端待评估  
**优先级：** 阶段 1 先做，阶段 2 等后端有空

---

### 📊 第一批：快速胜利

#### A1.1 - DLC 搜索框

**文件：** `DlcList.html`

**方案：**

```html
<script setup>
  const filterText = ref("");
  const filteredDlcs = computed(() => {
    if (!filterText.value) return props.dlcs;
    const q = filterText.value.toLowerCase();
    return props.dlcs.filter(
      (d) => d.name?.toLowerCase().includes(q) || d.appID.includes(q),
    );
  });
</script>

<template>
  <div class="dlc__controls">
    <UiInput
      v-model="filterText"
      type="search"
      size="sm"
      placeholder="搜索 DLC 名称或 AppID"
    />
  </div>

  <ul class="dlc__list">
    <li v-for="d in filteredDlcs" :key="d.appID">
      <!-- 现有渲染 -->
    </li>
  </ul>
</template>
```

**工时：** 1 小时  
**风险：** 🟢 低

---

#### A1.2 - DLC 全选/反选

**文件：** `DlcList.html`

**方案：**

```html
<div class="dlc__controls">
  <UiInput v-model="filterText" ... />
  <div class="dlc__bulk">
    <UiButton size="sm" @click="selectAll">全选</UiButton>
    <UiButton size="sm" @click="selectNone">全不选</UiButton>
  </div>
</div>
```

```typescript
function selectAll() {
  filteredDlcs.value.forEach((d) => {
    if (!isSelected(d.appID)) emit("toggle", d);
  });
}

function selectNone() {
  filteredDlcs.value.forEach((d) => {
    if (isSelected(d.appID)) emit("toggle", d);
  });
}
```

**重点：** 作用于 `filteredDlcs`，搜索时只全选可见项

**工时：** 1 小时  
**依赖：** A1.1

---

#### C1.2 - Hover 微浮起

**文件：** 新建 `styles/tokens/interaction.css`

**方案：**

```css
/* tokens/interaction.css */
.u-hover-lift {
  transition:
    transform var(--dur-instant) var(--ease-decelerate),
    box-shadow var(--dur-instant) var(--ease-decelerate);
}

.u-hover-lift:hover {
  transform: translateY(-2px);
  box-shadow: var(--elev-2);
}
```

**应用位置：**

- `GameCard.html` - 游戏卡片
- `UiButton` 的 secondary/ghost 变体

**工时：** 1.5 小时  
**风险：** 🟢 低

---

#### C1.4 - 加载态点点点动画

**文件：** 新建 `components/ui/LoadingDots.html`

**方案：**

```html
<template>
  <span class="dots" aria-label="加载中">
    <span class="dots__dot" />
    <span class="dots__dot" />
    <span class="dots__dot" />
  </span>
</template>

<style scoped>
  .dots {
    display: inline-flex;
    gap: 3px;
  }

  .dots__dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
    animation: dot-bounce 1.2s var(--ease-standard) infinite;
  }

  .dots__dot:nth-child(2) {
    animation-delay: 0.15s;
  }
  .dots__dot:nth-child(3) {
    animation-delay: 0.3s;
  }

  @keyframes dot-bounce {
    0%,
    60%,
    100% {
      opacity: 0.3;
      transform: translateY(0);
    }
    30% {
      opacity: 1;
      transform: translateY(-4px);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .dots__dot {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
```

**应用位置：**

- `UiButton` loading 态
- `GamePane` 试下载进度区

**工时：** 2 小时

---

#### A3.1 - 搜索历史

**文件：** `stores/search.ts` + `SearchPane.html`

**方案：**

```typescript
// stores/search.ts 新增
const MAX_HISTORY = 5;
const history = ref<string[]>(
  JSON.parse(localStorage.getItem("search-history") || "[]"),
);

function addToHistory(term: string) {
  history.value = [term, ...history.value.filter((t) => t !== term)].slice(
    0,
    MAX_HISTORY,
  );
  localStorage.setItem("search-history", JSON.stringify(history.value));
}

// 在 run() 成功后调用
if (mySeq === seq && list.length > 0) {
  addToHistory(q);
}
```

```html
<!-- SearchPane.html -->
<div v-if="search.status === 'idle' && search.history.length" class="history">
  <span class="history__label">最近搜索：</span>
  <button
    v-for="term in search.history"
    :key="term"
    class="history__item"
    @click="search.term = term; runSearch()"
  >
    {{ term }}
  </button>
</div>
```

**工时：** 1.5 小时

---

#### D1 - 首次扫描提示

**文件：** `LibraryOverviewPane.html`

**方案：**

```html
<div v-if="library.loading && !library.items.length" class="first-scan">
  <p class="first-scan__text">正在扫描 Steam 目录中的已部署清单…</p>
  <p class="first-scan__hint">首次启动可能需要几秒钟，之后会很快哦~</p>
</div>
```

**工时：** 30 分钟

---

### 🟡 第二批：体验完善

#### C1.1 - 成功微庆祝动画

**文件：** `ToastHost.html`

**方案：**

```css
.toast--success {
  animation:
    toast-bounce-in var(--dur-base) var(--ease-decelerate),
    toast-glow 0.8s ease-out;
}

@keyframes toast-bounce-in {
  from {
    opacity: 0;
    transform: translateY(-12px) scale(0.92);
  }
  60% {
    transform: translateY(2px) scale(1.02);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes toast-glow {
  from {
    box-shadow: 0 0 0 0 var(--state-ok);
  }
  to {
    box-shadow: 0 0 0 8px transparent;
  }
}
```

**工时：** 2 小时

---

#### A4 - 批量异常处理

**文件：** `LibraryOverviewPane.html`

**方案：**

```html
<section v-if="lostRecord.length" class="warn warn--danger">
  <h2>{{ lostRecord.length }} 个条目的记录丢失</h2>
  <UiButton variant="danger" size="sm" @click="batchRemoveLost">
    批量清理这 {{ lostRecord.length }} 个条目
  </UiButton>
</section>
```

```typescript
async function batchRemoveLost() {
  const count = lostRecord.value.length;
  const ok = await confirm({
    title: `批量删除 ${count} 个记录丢失的条目？`,
    body: ["这些条目没有记录，删除后无法还原。"],
    confirmText: "全部删除",
    danger: true,
  });
  if (!ok) return;

  let success = 0;
  for (const item of lostRecord.value) {
    try {
      await library.remove(item.mainAppID);
      success++;
    } catch (e) {
      console.error(`删除失败:`, e);
    }
  }

  toast.success(`已删除 ${success} / ${count} 个条目`);
}
```

**工时：** 2 小时

---

#### A5.1 - 全局快捷键

**文件：** 新建 `composables/useKeyboard.ts`

**方案：**

```typescript
export function useKeyboard() {
  const router = useRouter();
  const search = useSearchStore();

  onMounted(() => {
    window.addEventListener("keydown", handleKey);
  });

  onUnmounted(() => {
    window.removeEventListener("keydown", handleKey);
  });

  function handleKey(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey;

    // Ctrl+K: 聚焦搜索
    if (mod && e.key === "k") {
      e.preventDefault();
      router.push({ name: "search" });
      nextTick(() => {
        document
          .querySelector<HTMLInputElement>(".search__field input")
          ?.focus();
      });
    }

    // Ctrl+L: 已安装库
    if (mod && e.key === "l") {
      e.preventDefault();
      router.push({ name: "library" });
    }

    // Esc: 清空搜索
    if (e.key === "Escape" && router.currentRoute.value.name === "search") {
      search.clear();
    }
  }
}
```

**工时：** 3 小时

---

#### A6 - 侧栏折叠态搜索

**方案澄清：** 折叠时显示搜索图标，点击**跳转到搜索页**，而非弹出输入框

**理由：** 避免违反 UI-ARCHITECTURE.md 3.1 节"侧栏只承载选择"

```html
<SidebarItem
  v-if="ui.sidebarCollapsed"
  icon="search"
  label="搜索"
  @click="router.push({ name: 'search' })"
/>
```

**工时：** 1 小时

---

#### C2.1 - 骨架屏占位

**文件：** 新建 `components/ui/UiSkeleton.html`

**方案：**

```html
<template>
  <div
    class="skeleton"
    :class="`skeleton--${variant}`"
    :style="{ width, height }"
  />
</template>

<style scoped>
  .skeleton {
    background: linear-gradient(
      90deg,
      var(--color-surface-2) 0%,
      var(--color-surface) 50%,
      var(--color-surface-2) 100%
    );
    background-size: 200% 100%;
    animation: skeleton-shimmer 1.5s ease-in-out infinite;
    border-radius: var(--radius-ctrl);
  }

  @keyframes skeleton-shimmer {
    from {
      background-position: -200% 0;
    }
    to {
      background-position: 200% 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .skeleton {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
```

**应用位置：**

- `GamePane.html` - 详情加载态
- `SearchPane.html` - 搜索结果加载态

**工时：** 3 小时

---

#### C3 - 状态色"进行中蓝"

**文件：** `styles/tokens/color.css`

**方案：**

```css
/* 深色主题 */
:root[data-theme="dark"] {
  --state-processing: #6eb6ff;
}

/* 浅色主题 */
:root,
:root[data-theme="light"] {
  --state-processing: #3b6dff;
}
```

**应用位置：**

- `UiProgress` 新增 `tone="processing"`
- `DlcList` 同步状态
- `GamePane` 试下载进行中

**验证：** `npm run verify` 确保令牌被使用

**工时：** 1.5 小时

---

### 🟢 第三批：锦上添花

#### C1.3 - 数字滚动动画

**浏览器兼容性：**

- ✅ Chrome/Edge 85+
- ✅ Safari 15.4+
- ❌ Firefox 不支持 `@property`

**方案：渐进增强**

```css
@supports (background: paint(something)) {
  @property --stat-num {
    syntax: "<integer>";
    inherits: false;
    initial-value: 0;
  }

  .stat__v {
    counter-reset: num var(--stat-num);
    transition: --stat-num 0.8s var(--ease-decelerate);
  }

  .stat__v::after {
    content: counter(num);
  }
}
```

**降级：** Firefox 直接显示数字，无动画

**工时：** 4 小时

---

#### A5.2 - 侧栏箭头导航

**文件：** `composables/useKeyboard.ts`

**方案：**

```typescript
// 上下箭头：在库列表中导航
if (route.name === "library" && ["ArrowUp", "ArrowDown"].includes(e.key)) {
  e.preventDefault();
  const items = library.items;
  const current = items.findIndex((i) => i.mainAppID === route.params.appID);
  const next = e.key === "ArrowDown" ? items[current + 1] : items[current - 1];
  if (next) {
    router.push({ name: "library-game", params: { appID: next.mainAppID } });
  }
}
```

**工时：** 2 小时  
**依赖：** A5.1

---

#### C4.3 - 空态聚光效果

**文件：** `UiEmptyState.html`

**方案：**

```css
.empty-state::before {
  content: "";
  position: absolute;
  top: 50%;
  left: 50%;
  width: 400px;
  height: 400px;
  background: radial-gradient(circle, var(--color-accent) 0%, transparent 70%);
  opacity: 0.08;
  transform: translate(-50%, -50%);
  pointer-events: none;
}
```

**工时：** 1 小时

---

#### A3.2 - AppID 识别提示

**文件：** `SearchPane.html`

**方案：**

```html
<p v-if="/^\d+$/.test(search.term)" class="hint hint--accent">
  💡 检测到 AppID，将直接查询游戏详情
</p>
```

**工时：** 30 分钟

---

### ⏳ 第四批：需跨端协作

#### A2.2 - 各源实时结果展示

**需要后端提供：**

```go
type SourceResultPayload struct {
    AppID   string
    Source  string
    Status  string // "trying" | "success" | "failed"
    DLCCount int   // 成功时提供
}
```

**前端改动：**

```typescript
const sourceStates = ref<Record<string, SourceState>>({});

EventsOn("source:result", (payload) => {
  if (payload.appID !== appID.value) return;
  sourceStates.value[payload.source] = {
    status: payload.status,
    dlcCount: payload.dlcCount,
  };
});
```

**工时：** 前端 3h + 后端待评估

---

#### A2.3 - 真实进度百分比

**（已在 Bug #2 阶段 2 详述）**

---

## 🆕 新增决策：DLC 名称通用查询服务

### 背景

**问题识别：**

- Hubcap Manifest 源的 DLC 有详细名称（从 Lua 注释提取）
- MAU 系仓库的 DLC 只有 AppID，显示为 "DLC 2224460"
- 200 个纯数字 AppID 的列表完全无法使用

**根因：**

- `lua_parser.go:124-161` 的 `buildDLCNameMap()` 从 Hubcap 注释提取名称
- `repo_package.go:168-201` 的 MAU 路径硬编码 `Name: "DLC " + appID`
- config.json 只有 AppID 列表，无名称字段

### 决策

**✅ 实施方案：通用 DLC 名称批量查询服务**

#### 核心思路

利用 Steam 官方 `appdetails` 接口查询每个 DLC 的名称，作为所有源的通用回退方案。

#### 技术方案

**1. 新增后端方法（`store_client.go`）**

```go
// BatchGetDLCNames 批量获取 DLC 名称。
//
// 设计要点：
//   1. 并发查询（并发数 5），复用 Detail() 的缓存机制（7 天 TTL）
//   2. 单个失败不影响整体，失败的保持 "DLC {appID}"
//   3. 总超时 8 秒，超时后已查到的名称仍然生效
//
// 返回值：AppID → 名称的映射，查询失败的条目不在 map 中
func (s *StoreClient) BatchGetDLCNames(appIDs []string) map[string]string {
    const (
        concurrency = 5
        timeout     = 8 * time.Second
    )

    type result struct {
        appID string
        name  string
    }

    ch := make(chan result, len(appIDs))
    sem := make(chan struct{}, concurrency)

    for _, appID := range appIDs {
        go func(id string) {
            sem <- struct{}{}
            defer func() { <-sem }()

            detail, err := s.Detail(id)
            if err == nil && detail.Name != "" && detail.Name != id {
                ch <- result{appID: id, name: detail.Name}
            }
        }(appID)
    }

    names := make(map[string]string)
    deadline := time.After(timeout)
    received := 0

collect:
    for range appIDs {
        select {
        case r := <-ch:
            names[r.appID] = r.name
            received++
        case <-deadline:
            s.log("批量查询 DLC 名称超时，已获取 %d/%d", received, len(appIDs))
            break collect
        }
    }

    return names
}
```

**2. MAU 路径调用（`repo_package.go`）**

修改 `parseMAUPackage()` 返回值，增加待查名称的 AppID 列表：

```go
func parseMAUPackage(dir string) (*GamePackage, []string, []string, error) {
    // ... 现有逻辑

    // 收集需要查名称的 DLC AppID
    var needNames []string
    for _, dlc := range gp.DLCs {
        if dlc.Name == "DLC "+dlc.AppID {
            needNames = append(needNames, dlc.AppID)
        }
    }

    return gp, pending, needNames, nil  // 🆕 增加返回值
}
```

**3. RepoClient 补齐名称（`repo_client.go`）**

```go
// 在 fromZip() 或相应调用链中
pkg, pending, needNames, err := parseMAUPackage(dir)
if err != nil {
    return nil, err
}

// 补齐密钥与 manifest（现有逻辑）
if len(pending) > 0 {
    enrichPackageDLCs(pkg, pending)
}

// 🆕 补齐 DLC 名称
if len(needNames) > 0 && r.storeClient != nil {
    names := r.storeClient.BatchGetDLCNames(needNames)
    for i := range pkg.DLCs {
        if name, ok := names[pkg.DLCs[i].AppID]; ok {
            pkg.DLCs[i].Name = name
        }
    }
    r.log("已为 %d/%d 个 DLC 补齐名称", len(names), len(needNames))
}

return pkg, nil
```

**4. Lua 路径兜底（可选，`lua_parser.go`）**

为 Hubcap 注释不全的情况提供兜底：

```go
// ParseLua() 返回后检查未命名 DLC
var needNames []string
for _, dlc := range pkg.DLCs {
    if dlc.Name == "" || dlc.Name == "DLC "+dlc.AppID {
        needNames = append(needNames, dlc.AppID)
    }
}

if len(needNames) > 0 && storeClient != nil {
    names := storeClient.BatchGetDLCNames(needNames)
    for i := range pkg.DLCs {
        if name, ok := names[pkg.DLCs[i].AppID]; ok {
            pkg.DLCs[i].Name = name
        }
    }
}
```

#### 关键设计决策

**为何放在 `store_client.go`：**

- 职责就是"查询 Steam 商店元数据"
- 复用现有 HTTP 客户端、缓存机制（7 天）、日志系统
- 与 `Detail()` 代码复用度高

**为何并发而非批量接口：**

- Steam `appdetails` 不支持批量（实测 `appids=a,b,c` 返回空）
- 并发 5 个平衡速度与接口压力

**超时策略：**

- 单请求 15 秒（`storeHTTPTimeout`）
- 批次总超时 8 秒（留余量）
- 超时后已查到的名称仍然回填（部分生效）

**失败处理：**

- 单个 DLC 查询失败 → 跳过，保持 "DLC {appID}"
- 整批超时 → 已收到的仍然生效

#### 数据流

```plain
┌─────────────────────────────────────────────────────┐
│ 1. parseMAUPackage() 发现 DLC 名称全是 "DLC {appID}" │
│    → 返回 needNames = [2224460, 2224461, ...]       │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│ 2. storeClient.BatchGetDLCNames() 并发查询          │
│    → {2224460: "地图包：艾岛", 2224461: "音乐包"}     │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│ 3. RepoClient 回填到 pkg.DLCs                        │
│    查不到的保持 "DLC {appID}"                         │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│ 4. 前端收到完整 GamePackage，直接渲染 d.name         │
└─────────────────────────────────────────────────────┘
```

#### 工作量评估

| 改动                      | 文件               | 工时       |
| :------------------------ | :----------------- | :--------- |
| 新增 `BatchGetDLCNames()` | `store_client.go`  | 2h         |
| 修改返回值 + 收集逻辑     | `repo_package.go`  | 1h         |
| 调用 + 回填               | `repo_client.go`   | 1h         |
| Lua 路径兜底（可选）      | `lua_parser.go`    | 0.5h       |
| 测试                      | MAU 源 + Hubcap 源 | 1.5h       |
| **总计**                  |                    | **6 小时** |

#### 风险评估

| 风险项           | 概率  | 缓解方案                  |
| :--------------- | :---- | :------------------------ |
| Steam 接口限流   | 🟡 中 | 超时保护 + 部分生效策略   |
| 单个查询慢       | 🟢 低 | 并发 + 8 秒总超时         |
| 缓存失效高峰卡顿 | 🟢 低 | 7 天缓存 + 降级显示 AppID |
| 破坏现有流程     | 🟢 低 | 查询失败不影响原功能      |

**总体风险：** 🟢 低

#### 验收标准

**功能验收：**

- [ ] MAU 源游戏的 DLC 列表显示完整名称（非 "DLC {appID}"）
- [ ] Hubcap 源游戏的 DLC 名称不变（保持注释提取结果）
- [ ] 单个 DLC 查询失败时，保持 "DLC {appID}"，不影响其他
- [ ] 整批超时时，已查到的名称正常回填
- [ ] 后端日志记录查询成功率

**性能验收：**

- [ ] 200 个 DLC 的游戏，查询耗时 < 10 秒
- [ ] 第二次打开同一游戏，命中缓存，耗时 < 1 秒

**兼容性验收：**

- [ ] 无网络时，降级为 "DLC {appID}"，不报错
- [ ] 本地导入的 Lua 包（已有名称），不触发额外查询

#### 实施优先级

**建议放入第二批（体验完善）或作为第 0.5 批（高价值补漏）**

**理由：**

- 用户价值极高（200 个纯数字列表完全无法使用）
- 技术风险低（失败不破坏原功能）
- 实施成本可控（6 小时纯后端）
- 适用范围广（所有源都受益）

---

## 🚫 不实施项目及理由

### A1.3 - DLC 分组展示

**不实施理由：**

1. **分类标准不明确** - 无元数据，启发式规则准确度低
2. **收益有限** - A1.1 搜索框已解决 200+ DLC 的查找问题
3. **实现成本高** - 需维护中英文关键词库，不同游戏命名习惯差异大

**替代方案：** A1.1 搜索框 + A1.2 全选/反选已足够

---

### A3.3 - 模糊搜索建议

**不实施理由：**

1. **需后端编辑距离算法** - 开发成本高
2. **收益不足** - 用户可自行换英文再搜
3. **误导风险** - 相似度判定不准时反而干扰

---

### A1.4 - DLC 虚拟滚动

**暂不实施理由：**

1. **先做 A1.1 搜索 + A1.3 分组** - 实测性能后再决定
2. **引入第三方库风险** - 需评估与现有样式兼容性
3. **可能是过度优化** - 200 个 `<li>` 未必真卡顿

**决策流程：**

1. 实施 A1.1 搜索框
2. 实测 200 个 DLC 的渲染时间 + 滚动帧率
3. 有瓶颈再引入虚拟滚动

---

## 📊 施工路线图

### 第一周（5 个工作日）

**Day 1：紧急修复 + 快速胜利前半**

- [ ] Bug #1: GamePane 切换闪烁（1h）
- [ ] Bug #2: 阶段式进度显示（0.5h）
- [ ] A1.1: DLC 搜索框（1h）
- [ ] A1.2: DLC 全选/反选（1h）
- [ ] D1: 首次扫描提示（0.5h）
- [ ] A3.2: AppID 识别提示（0.5h）

**Day 2：快速胜利后半 + DLC 名称查询**

- [ ] C1.2: Hover 微浮起（1.5h）
- [ ] C1.4: 点点点动画（2h）
- [ ] A3.1: 搜索历史（1.5h）
- [ ] **🆕 DLC 名称批量查询（后端，2h）**

**Day 3-4：交互完善**

- [ ] **🆕 DLC 名称查询集成（后端，2h）**
- [ ] C1.1: 成功庆祝动画（2h）
- [ ] A4: 批量异常处理（2h）
- [ ] C3: 状态色"进行中蓝"（1.5h）
- [ ] A5.1: 全局快捷键（3h）

**Day 5：视觉打磨**

- [ ] C2.1: 骨架屏占位（3h）
- [ ] A6: 侧栏折叠态搜索（1h）
- [ ] C4.3: 空态聚光效果（1h）
- [ ] **🆕 DLC 名称查询测试（1.5h）**

### 第二周（视觉增强 + 跨端协作）

**Day 6-7：锦上添花**

- [ ] C1.3: 数字滚动动画（4h）
- [ ] A5.2: 侧栏箭头导航（2h）

**Day 8-10：跨端协作（需后端参与）**

- [ ] A2.2: 各源实时结果（前端 3h + 后端联调）
- [ ] A2.3: 真实进度百分比（前端 2h + 后端联调）

---

## 🎯 已确认的技术事实

### Toast 队列机制 ✅

已读 `stores/ui.ts:59-68`，**确认已实现队列**：

```typescript
const toasts = ref<Toast[]>([]);

function toast(message: string, kind: ToastKind = "info") {
  const id = ++seq;
  toasts.value.push({ id, kind, message }); // 推入数组
  window.setTimeout(() => dismiss(id), DURATION[kind]);
}
```

多个 toast 同时触发时会**依次排列**，不会堆叠。

### 进度推送已就位 ✅

已读 `GamePane.html:169-173`，**后端已在推送 `download:progress`**：

```typescript
EventsOn("download:progress", (payload: any) => {
  const src = payload?.source ?? "";
  progressText.value = src ? `正在尝试 ${src}…` : "正在获取…";
});
```

只是没在界面上显示，补个 `<UiProgress>` 即可。

## 2026-09-04：安全漏洞修复与依赖升级

### 背景

收到来自 @begininvoke 的安全报告（PR #3, Issue #4, Issue #5），经过 govulncheck 官方扫描确认，项目存在真实安全漏洞。

### 决策

1. **PostCSS 升级** (commit 54895b1)
   - `postcss@8.5.8` → `8.5.28`
   - 修复 CVE-2026-45623 (HIGH)
   - 前端构建验证通过

2. **Go 升级** (commit 116592b)
   - `go1.25.5` → `go1.27.0`
   - 修复 15 个标准库漏洞 (CRITICAL/HIGH)
   - govulncheck: 0 vulnerabilities affecting code

3. **Wails 升级至 v2.15.0**
   - **Wails v3 尚在 beta**，不适合生产使用
   - 当前版本稳定，无安全问题，等 v3 正式版发布后再继续考虑升级

### 验证结果

- ✅ `npm run verify` - 前端构建通过
- ✅ `go test ./...` - 后端测试通过
- ✅ `govulncheck ./...` - 0 个漏洞影响代码
