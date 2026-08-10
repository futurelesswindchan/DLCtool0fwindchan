# 🐰 风兔盒 (KAZEUSA)

![Wails](https://img.shields.io/badge/Wails-v2.11+-red.svg?style=flat&logo=wails)
![Vue.js](https://img.shields.io/badge/Vue-3.4+-%2335495e.svg?style=flat&logo=vuedotjs&logoColor=%234FC08D)
![TypeScript](https://img.shields.io/badge/TypeScript-5.3+-%23007ACC.svg?style=flat&logo=typescript&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg?style=flat&logo=go&logoColor=white)
![Powered by OST](https://img.shields.io/badge/Powered_by-OpenSteamTool-2ea44f.svg?style=flat&logo=steam&logoColor=white)
![License](https://img.shields.io/badge/license-CC%20BY--NC--SA%204.0-lightgrey.svg?style=flat)

---

> 极为优雅的 Steam 游戏 DLC 清单/解密密钥在线管理与调度中心 🚀
>
> An elegant, secure, and decoupled Steam Game DLC manifest & decryption key package manager.

<div align="center">
  <table>
    <tr>
      <td><img width="445" alt="preview_light" src="https://github.com/futurelesswindchan/DLCtool0fwindchan/blob/main/.github/images/preview_light.png" /></td>
      <td><img width="445" alt="preview_dark" src="https://github.com/futurelesswindchan/DLCtool0fwindchan/blob/main/.github/images/preview_dark.png" /></td>
    </tr>
    <tr>
      <td align="center">☀️ 浅色模式（默认）</td>
      <td align="center">🌙 深色模式</td>
    </tr>
  </table>
</div>

---

## 📖 重新向世界介绍自己

你好呀——！👋 欢迎来到风风的 DLC 魔法工坊 v2.0 时代！

在 V2 版本中，本作正式更名为 **「风兔盒 (kazeusa)」**。  
它不再是过去那个包揽一切的“注入器配置工具”，而是进化成为了一个强大且克制的 **Steam 游戏 DLC 清单/解密密钥管理器**。

为了绝对的稳定与安全，咱们设计了严谨的 **三层解耦架构**：

```text
🌐 在线清单与密钥仓库 (社区维护)
       ↓
       ↓ 获取、探查、解析、比对
       ↓
📦 风兔盒 kazeusa (我们的领地！)
       ↓
       ↓ 将清洗后的 .lua 清单精准放入 config/lua 目录
       ↓
🔧 注入器层 (OpenSteamTool)
```

⚠️ **不可逾越的三条铁律**：
为了不与 Steam 客户端及底层解锁器“神仙打架”，风兔盒做出了以下承诺：

1. **绝不写入或修改 `config.vdf` 等 Steam 客户端相关文件**。
2. **绝不写入或干涉注入器自身的配置文件**。
3. **绝不负责安装、更新或修复注入器本身**。

如果你遇到了 Steam 崩溃或闪退，那绝对不是风兔盒的锅哦！我们只是一个乖巧的文件搬运工~ 📦

---

## ✨ 核心亮点 (Core Features)

- 🍻 **干净、小巧、便携**\
  风兔盒会将所有运行时以及数据文件放在 `.exe` 同级的 `.kazeusa` 文件夹中，完全不依赖注册表和系统目录。不会在你的系统里留下任何痕迹>w<！且得益于 Wails 和 GO 的强大，风兔盒编译产物为仅 **10+ MB** 的单文件 EXE，下载即用，删除即走~
- 🌍 **魔法寻源：清单/密钥在线对撞**\
  还在到处求 DLC 的清单文件？输入游戏昵称或 AppID ，风兔盒会瞬间并发探查多个内置在线仓库，直接生成实力对比表，告诉你能装上多少个 DLC。选最优的那个，一键入库！
- ⚡ **无感热重载：0.5 秒魔法生效**\
  得益于 OST 底层强大的文件监听机制，在盒子中勾选完想要的 DLC，切回 Steam 即可在 500ms 内瞬间完成识别！告别过去反复重启 Steam 的痛苦折磨。
- 📦 **精细化留存管控**\
  获取清单后，不想全装？DLC 列表提供单页细粒度勾选，修改自动落盘记录，你的选择会被永久记住。当然，如果你更喜欢手动导入本地 `.zip` 压缩包，我们依然提供丝滑拖拽支持~
- 🎨 **强迫症级别的纯手工 UI**\
  拒绝臃肿的组件库！前端基于 Vue 3，从按钮到单选框全部都是原生自绘的。现代扁平化视觉、无边框沉浸设计、双重色彩主题、所有动效全程可中断计算~(๑•̀ㅂ•́)و✧

---

## 🚀 食用指南 (Quick Start)

> **不知道怎么开始？没有任何基础？**
>
> 考虑到长图文教程塞在 GitHub 里不太方便，所以风酱打算在博客为大家准备 **最最最新版保姆级图文教程** 啦！
>
> 👉 **[风兔盒（KAZEUSA）食用教程](https://qwq.windchan0v0.xyz/admin/dashboard/editor/frontend/kazeusa)**

---

## 💬 发现 BUG 啦？ (Issues)

- 🐞&💡 工具层面报错、界面卡死、白屏，或者有更好的改进点子？
- 👉 [欢迎在 Issue 里向灵感许愿池投币~ ](https://github.com/futurelesswindchan/DLCtool0fwindchan/issues)
- ⚠️ 注意：**Steam 崩溃 / 游戏进不去 / 装了也没效果**，请移步 OST 、或清单源的社区进行提问和求助，这真的不归本盒子管哦！

---

## 🛠️ 执剑指南 (本地开发与源码编译)

如果你想自己从源码编译出纯绿色的 EXE，或者想研究下这个严密运转的工程，欢迎加入本地开发序列！

### 环境准备

1. 配置 [Go (1.23+)](https://golang.org/doc/install) 环境。
2. 配置 [Node.js (18+)](https://nodejs.org/en/) (前端构建使用)。
3. 安装 Wails 开发脚手架：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 施法指令

```bash
# 克隆仓库与安装前端依赖
git clone https://github.com/futurelesswindchan/DLCtool0fwindchan.git
cd DLCtool0fwindchan/frontend
npm i

# 退回根目录
cd ..

# 🔮 启动实时开发模式 (支持前端增量热更与后端重载哦！)
wails dev

# 🔨 编译发布，在 build/bin/ 中召唤最终免安装 EXE
wails build -clean
```

---

## 💖 鸣谢 (Special Thanks)

风兔盒 (Kazeusa) 能够以如此轻盈且优雅的姿态运行，离不开开源社区的伟大贡献。

向以下在幕后默默发光发热的神仙项目与开发者们致以最深的敬意！（单方面疯狂贴贴 awa）

<div align="center">
  <table>
    <tr>
      <td align="center" width="45%">
        <br/>
        <a href="https://github.com/OpenSteam001/OpenSteamTool">
          <img src="https://raw.githubusercontent.com/OpenSteam001/OpenSteamTool/main/docs/logo-animated.svg" width="180" alt="OpenSteamTool Logo">
          <br/>
          <h3>OpenSteamTool</h3>
        </a>
        强大的新一代 Steam 开源底层解锁引擎<br/>
        （本项目的核心动力源泉 🚀）
        <br/><br/>
      </td>
      <td align="center" width="55%">
        <br/>
        <a href="https://wails.io/">
          <!-- Wails 的官方 Logo -->
          <img src="https://wails.io/img/wails-logo-horizontal.svg" width="360" alt="Wails Logo">
          <br/>
          <h3>Wails</h3>
        </a>
        无比优雅的 Go + Web 桌面应用构建框架<br/>
        （赋予了风兔盒轻盈的体态与绝美的容颜 🎨）
        <br/><br/>
      </td>
    </tr>
  </table>
</div>

## 📄 使用许可 (License)

本项目采用 [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/deed.zh) 协议进行许可。  
简单来说：欢迎学习、分享和修改，但请 **注明出处**，并且 **绝对不要用于商业用途** 哦 awa！

_Made with love and magic by [futurelesswindchan](https://github.com/futurelesswindchan)_
