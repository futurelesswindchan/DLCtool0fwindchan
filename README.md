# DLC 解锁工具 v1.0

一个简单易用的 Steam DLC 一键解锁工具，支持拖拽压缩包、自动识别游戏和 DLC、选择性安装/卸载。

## 功能特性

- 🎮 **傻瓜式操作**：拖拽压缩包即可使用
- 🔍 **自动识别**：自动解析 Lua 文件，识别游戏和 DLC
- ✨ **选择性安装**：可选择要安装的 DLC
- 🗑️ **一键清除**：支持清除所有伪入库 DLC
- 🌙 **深浅主题**：支持深色和浅色主题切换
- 📦 **单个文件**：所有功能集成在一个 EXE 中

## 系统要求

- Windows 7 或更高版本
- 已安装 Steam
- 管理员权限（修改 Steam 配置时需要）

## 使用方法

1. 首先你需要获取 DLC 解锁包（格式：`AppID.zip`）
2. 运行此项目编译出来的 EXE 文件
3. 拖拽压缩包到工具窗口，或点击"选择文件"按钮
4. 工具会自动识别游戏和 DLC
5. 选择要安装的 DLC，或点击"清除所有"
6. 点击"安装选中"或"清除所有"按钮
7. 重启 Steam，DLC 应该已经可用

## 压缩包格式

压缩包应包含以下文件：

```
AppID.zip
├── AppID.lua              # Lua 脚本（包含 AppID 和密钥信息）
├── DepotID_ManifestID.manifest  # Manifest 文件
├── DepotID_ManifestID.manifest  # 其他 Manifest 文件
└── ...
```

## 技术栈

- **前端**：Vue 3 + TypeScript + Vite
- **后端**：Go 1.21+
- **框架**：Wails v2
- **打包**：Wails CLI

## 开发

### 环境要求

- Go 1.21 或更高版本
- Node.js 16 或更高版本
- npm 或 yarn

### 安装依赖

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 安装前端依赖
cd frontend
npm install
cd ..
```

### 开发模式

```bash
wails dev
```

### 构建

```bash
wails build
```

输出文件将在 `build/bin/` 目录中。

## 项目结构

```
DLCtool0fwindchan/
├── app.go                 # Go 应用主逻辑
├── main.go               # Go 入口文件
├── go.mod               # Go 模块定义
├── wails.json           # Wails 配置
├── frontend/            # Vue 前端
│   ├── src/
│   │   ├── App.vue      # 主组件
│   │   ├── main.ts      # 入口文件
│   │   └── style.css    # 全局样式
│   ├── index.html       # HTML 模板
│   ├── package.json     # npm 配置
│   ├── vite.config.ts   # Vite 配置
│   └── tsconfig.json    # TypeScript 配置
└── build/               # 构建输出
```
