# 🎮 DLC 库管理工具 (DLCtool0fwindchan)

![Wails](https://img.shields.io/badge/Wails-v2.0+-red.svg?style=flat)
![Vue.js](https://img.shields.io/badge/vuejs-%2335495e.svg?style=flat&logo=vuedotjs&logoColor=%234FC08D)
![TypeScript](https://img.shields.io/badge/typescript-%23007ACC.svg?style=flat&logo=typescript&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat&logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-lightgrey)

> 一个优雅、安全、简单易用的 Steam DLC 本地一键解锁与管理工具 ✨
>
> A simple and elegant Steam DLC local unlocker and manager built with Wails, Vue 3 & Go.

![示例图片1](https://github.com/futurelesswindchan/DLCtool0fwindchan/blob/main/.github/images/preview_light.png)
![示例图片2](https://github.com/futurelesswindchan/DLCtool0fwindchan/blob/main/.github/images/preview_dark.png)

---

## 📖 项目简介 (Introduction)

你好——！👋 欢迎来到「风风的 DLC 魔法工坊」！

这不仅仅是一个枯燥的解锁器，我希望把它打造成一个兼具 **高颜值** 与 **极简交互** 的管理工具。  
整个项目抛弃了传统单调的界面，采用了现代化的双面板设计，从底层解析到前端 UI 都是精心重构的产物！ヽ( ^ω^ ゞ )

- **前端 [Frontend]**: 基于 Vue 3 全家桶，负责所有赏心悦目的视觉效果与顺滑的拖拽交互。
- **后端 [Backend]**: 基于 Go 和 Wails 框架，提供安全、极速的系统级文件操作与 VDF 解析。

## 💖 核心亮点 (Core Features)

- **🎨 现代化沉浸 UI 设计**
  全新双面板设计，支持深浅色模式无缝切换！并且与 Windows 原生标题栏沉浸整合，带来极致的视觉享受！ヾ(_´∀ ˋ_)ﾉ

- **📦 傻瓜式拖拽体验**
  告别繁琐的手动解压！只需将 `.zip` 格式的 DLC 解锁包直接拖入软件，它就能自动把一切安排得明明白白~

- **🔍 智能解析与安全引擎**
  底层重构了 VDF 解析与 Steamtools Lua 注入机制。自动读取 Lua 脚本，精准识别游戏与 DLC，同时拥有极高的容错与幂等性，**绝对不会损坏你的 Steam 配置文件**！

- **⚙️ 精细化状态管理**
  支持对 DLC 进行全选、反选、单选安装，甚至可以一键清除所有伪入库，让你的库干干净净！

---

## 🚀 食用指南 (User Guide)

> **想立刻开始给游戏添加 DLC 吗？或者不知道去哪里找解锁包？**
>
> 考虑到能逛到 GitHub 的大佬们肯定都有魔法基础啦，再加上配合其他神秘工具的进阶玩法比较多，风酱已经把 **最最最详细的保姆级图文教程** 写在博客里啦！(o ゜ ▽ ゜)o☆
>
> 👉 **[点击这里前往风风博客，查看完整使用教程与进阶玩法~](https://qwq.windchan0v0.xyz/articles/topics/ark-dlc-add)**

---

## 🛠️ 源码编译与本地开发 (Build & Develop)

想亲自动手给工具加点料，或者自己从源码编译出绿色的 EXE？欢迎来到本地开发频道！(`・ω・´)

### 开发环境准备

1. 安装 [Go (1.21+)](https://golang.org/doc/install)
2. 安装 [Node.js (16+)](https://nodejs.org/en/)
3. 安装 Wails CLI：
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```
4. 拉取代码并安装前端依赖：
   ```bash
   git clone https://github.com/futurelesswindchan/DLCtool0fwindchan.git
   cd DLCtool0fwindchan/frontend
   npm install
   cd ..
   ```

### 开始施法！(开发指令)

```bash
# 🔮 启动实时开发模式 (支持前端热更新，修改代码实时可见哦！)
wails dev

# 🔨 编译打包最终的 EXE 可执行文件
wails build
```

_(编译输出的独立可执行文件会乖乖躺在 `build/bin/` 目录下~)_

---

## 📂 项目结构 (Project Structure)

```text
DLCtool0fwindchan/
├── main.go               # Go 应用入口
├── app.go                # Wails 生命周期与核心前端接口绑定
├── steam.go              # Steam 路径检测与操作逻辑
├── vdf_helper.go         # VDF 配置文件安全解析与重写引擎
├── lua_parser.go         # Lua 脚本解析器
├── wails.json            # Wails 工程配置
├── frontend/             # Vue 3 前端魔法阵
│   ├── src/
│   │   ├── components/   # 拆分的 UI 积木 (DropZone, DlcCard等)
│   │   ├── App.vue       # 前端主视图
│   │   ├── main.ts       # 前端入口
│   │   └── style.css     # 全局扁平化设计与动画 CSS
│   └── vite.config.ts    # Vite 构建配置
└── build/                # 编译资源与输出目录
```

---

## 📄 使用许可 (License)

本项目采用 [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/deed.zh) 协议进行许可。  
简单来说：欢迎学习、分享和修改，但请 **注明出处**，并且 **绝对不要用于商业用途** 哦 awa！

---

> **Copyright © 2026 没有未来的小风酱 (futurelesswindchan)**
>
> Made with ♡ and lots of —⊂ZZZ⊃.
