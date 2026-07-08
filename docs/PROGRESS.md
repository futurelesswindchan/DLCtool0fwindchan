# 开发进度追踪

> 每次开发结束时更新本文件，下次开发接力时快速定位当前进度。
>
> 最后更新：2026-07-08

---

## 当前阶段：OST 源码研究完成，待研究项已全部确认，准备进入施工

### ✅ 已完成

- [x] v1.4 全部代码审查（Go 后端 + Vue3 前端）
- [x] 竞品分析（Fluent Install — Python + PyQt6）
- [x] OpenSteamTool 文档研究与适配方案设计
- [x] 架构白皮书定稿（三层解耦 / 模块划分 / 接口契约）
- [x] 决策日志建立
- [x] B 站大更新预告动态撰写
- [x] **OST 源码完整架构分析**（产出：`OST_Architecture_Analysis.md`）
- [x] **OST 热重载机制研究**：事件驱动 + 500ms 防抖，文件落盘即生效
- [x] **OST SteamUI 刷新机制研究**：安装/卸载后库自动更新，无需重启
- [x] **OST ManifestClient 研究**：全自动三级回退，kazeusa 无需参与

### ✅ 待研究项——全部已确认

- [x] OST 的 Lua 目录：默认 `<Steam>/config/lua/`，可通过 toml `[lua].paths` 添加额外目录
- [x] OST 自动下载 manifest：全自动，拦截网络包 + 上游 API 获取码
- [x] addappid 第二参数：**被完全忽略**，代码只看第1参数(AppId)和第3参数(64字符key)
- [x] OST 环境检测指标：检查 Steam 根目录是否存在 `dwmapi.dll` + `xinput1_4.dll` + `OpenSteamTool.dll`
- [x] M 站 Lua 与 OST 格式差异：无差异，OST 的 addappid 兼容任何第二参数
- [x] 在线仓库的具体源：待定（技术上无阻塞，属于产品决策）

### 📋 待开始（地基阶段）

- [ ] config.go — 配置持久化
- [ ] logger.go — 日志增强（轮转 + 路径迁移）
- [ ] deployer.go + deployer_ost.go — 部署器
- [ ] detector.go + detector_ost.go — 环境检测
- [ ] repo_client.go — 在线仓库客户端
- [ ] history.go — 安装历史
- [ ] app.go 重构 — 接入新架构
- [ ] 前端 v2.0 UI 设计与实现
- [ ] 旧代码清理（移除 vdf_helper / steam.go 中的 patch 逻辑）

---

## 版本里程碑

| 版本       | 目标                                  | 状态 |
| ---------- | ------------------------------------- | ---- |
| v2.0-alpha | 地基完成：配置/日志/部署器/检测器     | 🔜   |
| v2.0-beta  | 核心功能：在线仓库 + 历史管理 + 新 UI | 📋   |
| v2.0-rc    | 全功能可用，博客教程完成              | 📋   |
| v2.0       | 正式发布                              | 📋   |

---

## OST 源码研究关键发现摘要

> 详见 `docs/OST_Architecture_Analysis.md`

| 发现                               | 对 kazeusa 的影响                             |
| ---------------------------------- | --------------------------------------------- |
| 热重载 500ms 防抖，事件驱动        | deployer 写文件即可，推荐 tmp+rename 原子写入 |
| 安装后 Steam 库自动刷新            | UX 文案可以直接写"已添加到库"                 |
| Manifest 全自动获取                | 不需要 manifest fallback 功能                 |
| .lua 文件 mtime 作为 PurchasedTime | 不要修改文件时间戳，保持当前时间即可          |
| OST 函数名大小写无关               | 生成的 Lua 用全小写即可                       |
