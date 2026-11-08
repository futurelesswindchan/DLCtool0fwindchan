# v2.1.0 体验优化进度追踪

> 本文档记录 DECISIONS.md 中 v2.1.0 优化方案的实施进度
>
> 开始日期：2026-09-04  
> 施工分支：feat/v2.1.0-ux-improvements

---

## 🚨 紧急修复

- [x] Bug #1: GamePane 切换闪烁修复
  - 文件：`frontend/src/views/panes/GamePane.vue`
  - 工时：1h
  - 说明：增加 `switching` 标记防止切换瞬间状态跳变
  - 完成时间：2026-09-04
  - 额外：只修了一小部分，还没有完全解决闪烁问题，后续可能需要进一步优化

- [x] Bug #2: 实时多源状态追踪
  - 文件：`app_trial_run.go`, `frontend/src/views/panes/GamePane.vue`
  - 工时：3h（实际比预估多，因为需要实现完整的事件推送机制）
  - 说明：后端推送 `source:progress` 事件，前端实时显示各源状态（waiting → trying → success/failed）
  - 完成时间：2026-09-04
  - 实施细节：
    - 后端：在 `TrialSources` 初始化时推送所有源 `waiting` 状态
    - 后端：在 `trialOne` 中推送 `trying` / `success` / `failed` 状态
    - 前端：新增 `sourceStates` 响应式变量追踪实时状态
    - 前端：模板优先显示实时状态（⏳ ⌛ 🔄 ✅ ❌）
    - 样式：`trial--trying` 带呼吸动画，`trial--waiting` 半透明
  - 额外：目前只是能跑，实机效果太丑了，需要进一步设计排版与SVG资产

---

## 🎯 第一批：快速胜利（Day 1-2）

- [ ] A1.1: DLC 搜索框
  - 文件：`frontend/src/components/DlcList.vue`
  - 工时：1h
  - 说明：过滤 DLC 名称或 AppID

- [ ] A1.2: DLC 全选/反选
  - 文件：`frontend/src/components/DlcList.vue`
  - 工时：1h
  - 依赖：A1.1
  - 说明：全选/全不选作用于过滤后的列表

- [ ] C1.2: Hover 微浮起
  - 文件：`frontend/src/styles/tokens/interaction.css`
  - 工时：1.5h
  - 说明：GameCard 和 Button 添加 hover 效果

- [ ] C1.4: 加载态点点点动画
  - 文件：`frontend/src/components/ui/LoadingDots.vue`
  - 工时：2h
  - 说明：替代静态加载文本

- [ ] A3.1: 搜索历史
  - 文件：`frontend/src/stores/search.ts` + `frontend/src/views/SearchPane.vue`
  - 工时：1.5h
  - 说明：localStorage 存储最近 5 条搜索

- [ ] D1: 首次扫描提示
  - 文件：`frontend/src/views/panes/LibraryOverviewPane.vue`
  - 工时：0.5h
  - 说明：首次加载时显示友好提示

- [ ] A3.2: AppID 识别提示
  - 文件：`frontend/src/views/SearchPane.vue`
  - 工时：0.5h
  - 说明：纯数字输入显示提示

---

## 🆕 DLC 名称批量查询服务（Day 2）

- [ ] 后端：新增 `BatchGetDLCNames()` 方法
  - 文件：`store_client.go`
  - 工时：2h
  - 说明：并发查询 DLC 名称，8 秒超时

- [ ] 后端：MAU 路径集成
  - 文件：`repo_package.go`, `repo_client.go`
  - 工时：2h
  - 说明：收集需要查名称的 AppID，调用查询并回填

- [ ] 后端：Lua 路径兜底（可选）
  - 文件：`lua_parser.go`
  - 工时：0.5h
  - 说明：Hubcap 注释不全时兜底

- [ ] 测试：MAU 源 + Hubcap 源
  - 工时：1.5h
  - 验收：200 个 DLC 显示完整名称，非 "DLC {appID}"

---

## 🌟 第二批：体验完善（Day 3-5）

- [ ] C1.1: 成功微庆祝动画
  - 文件：`frontend/src/components/ToastHost.vue`
  - 工时：2h

- [ ] A4: 批量异常处理
  - 文件：`frontend/src/views/panes/LibraryOverviewPane.vue`
  - 工时：2h

- [ ] C3: 状态色"进行中蓝"
  - 文件：`frontend/src/styles/tokens/color.css`
  - 工时：1.5h

- [ ] A5.1: 全局快捷键
  - 文件：`frontend/src/composables/useKeyboard.ts`
  - 工时：3h

- [ ] C2.1: 骨架屏占位
  - 文件：`frontend/src/components/ui/UiSkeleton.vue`
  - 工时：3h

- [ ] A6: 侧栏折叠态搜索
  - 文件：`frontend/src/components/layout/Sidebar.vue`
  - 工时：1h

- [ ] C4.3: 空态聚光效果
  - 文件：`frontend/src/components/ui/UiEmptyState.vue`
  - 工时：1h

---

## 🎨 第三批：锦上添花（待排期）

- [ ] C1.3: 数字滚动动画（渐进增强）
- [ ] A5.2: 侧栏箭头导航
- [ ] 其他...

---

## ⚠️ 需后端配合（待联调）

- [ ] A2.2: 各源实时结果展示
  - 需新增 `source:result` 事件
- [ ] A2.3: 真实进度百分比
  - 需 `download:progress` 增加 `current`/`total` 字段

---

## 📊 进度统计

- 总计划项：26 项
- 已完成：2 项 ✅
- 进行中：0 项
- 待开始：24 项

---

## 📝 施工日志

### 2026-09-05

**上午-下午**

- ✅ Bug #1 深度修复：延迟清空数据 + 过期响应检测
  - 实现方案 A：切换时不立即清空 `pkg`，保持视觉连续性
  - 为所有异步加载函数添加过期响应检测机制
  - 修改涉及：`load()`, `loadDetail()`, `loadStored()`, `lookup()` 及所有调用点
  - 新增调试日志追踪切换流程
  
- ✅ Hero 区域布局优化：固定高度防跳动
  - 问题：不同游戏简介长度不同，导致 hero 区域高度跳变，封面图被拉伸
  - 方案：
    - 固定 `.hero__info` 高度与封面对齐（~108px）
    - 标题和 meta 限制一行，超出省略号
    - 描述区固定 3 行高度（`min-height`），即使内容不足也占位
    - 移除 `v-if`，让 `hero__desc` 始终渲染
  - 修改文件：`frontend/src/views/panes/GamePane.vue`

**待实施**

- 方案 B：路由守卫预加载（需测试方案 A 效果后再决定）
- 清理调试日志（待主人确认效果后）

### 2026-09-04

**上午**

- ✅ 创建 PROGRESS.md
- ✅ Bug #1: GamePane 切换闪烁修复（初步）
  - 增加 `switching` 标记，在 `watch(appID)` 中设置，防止切换瞬间 `viewState` 跳到错误状态
- ✅ Bug #2: 实时多源状态追踪
  - 后端：`app_trial_run.go` 添加 `runtime.EventsEmit` 推送逻辑
  - 前端：`GamePane.vue` 监听 `source:progress` 事件并实时更新 UI
  - 添加 `trial--waiting` 和 `trial--trying` 样式，trying 带呼吸动画

**待测试**

- 需要主人实机测试 Bug #1 和 Bug #2 的效果，确认无误后提交 commit
 项
- 待开始：24 项

---

## 📝 施工日志

### 2026-09-04

**上午**

- ✅ 创建 PROGRESS.md
- ✅ Bug #1: GamePane 切换闪烁修复
  - 增加 `switching` 标记，在 `watch(appID)` 中设置，防止切换瞬间 `viewState` 跳到错误状态
- ✅ Bug #2: 实时多源状态追踪
  - 后端：`app_trial_run.go` 添加 `runtime.EventsEmit` 推送逻辑
  - 前端：`GamePane.vue` 监听 `source:progress` 事件并实时更新 UI
  - 添加 `trial--waiting` 和 `trial--trying` 样式，trying 带呼吸动画

**待测试**

- 需要主人实机测试 Bug #1 和 Bug #2 的效果，确认无误后提交 commit
