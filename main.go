// main.go
//
// 本文件是 Wails 应用的装配入口，只做三件事：
//   1. 构造 App 实例
//   2. 声明窗口与运行时选项
//   3. 启动事件循环
//
// 不在此处编写任何业务逻辑——所有前端可调用的方法都在 app.go。
//
// 文件落点约定：
//
//	本工具产生的一切文件都必须位于 ~/.kazeusa/ 之下，
//	包括 WebView2 的用户数据目录（见 WebviewUserDataPath 设置）。
//	唯一的例外是部署到 <Steam>/config/lua/ 的清单脚本，
//	以及 %TEMP% 下用完即删的解压临时目录。
//	验收标准：卸载 = 删 exe + 删 ~/.kazeusa/ 一个文件夹。

package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "风兔盒 - 请问您今天要来点DLC吗？",
		Width:     1200,
		Height:    700,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// 与 wails.json 的 backgroundColour 及前端 styles/tokens/color.css 的
		// 浅色 --color-bg 保持一致，避免界面在首帧渲染前闪出别的颜色。
		// 当前 #f4f2f8（默认浅色主题的冷紫白底），三处共同定义，改一处必须同改另两处。
		//
		// 此值是编译期常量，无法随主题切换。选了深色主题的用户在窗口创建的
		// 那一瞬仍会看到浅底，属 frameless 窗口的固有限制，无绕行方案。
		BackgroundColour: &options.RGBA{R: 244, G: 242, B: 248, A: 1},

		// 隐去系统标题栏，改由前端的 TopBar 组件自绘，令 Logo、导航页签、
		// 环境状态与窗口控制融为一行。
		//
		// 拖动、缩放与 Aero Snap 的保留情况（v2.11 实现）：
		//
		//   - 拖动：CSS 声明 --wails-draggable: drag 后，Wails 收到 drag 消息即
		//     ReleaseCapture() + PostMessage(WM_NCLBUTTONDOWN, HTCAPTION)，
		//     等于把后续过程交还给系统的标题栏拖动循环。Aero Snap（拖至屏幕
		//     边缘吸附）正由该循环处理，因此完整保留，无需自行实现
		//   - 缩放：WM_NCCALCSIZE 返回 0 只隐去标题栏，WS_THICKFRAME 仍在，
		//     故边缘缩放热区照常可用。前端元素不要占据窗口边缘 4~6px
		//   - Snap Layouts（Win11 悬停最大化按钮弹出的分屏浮层）不可用：
		//     它要求 WM_NCHITTEST 对最大化按钮区域返回 HTMAXBUTTON，
		//     而自绘按钮位于 WebView 客户区内，命中测试到不了那里。
		//     属原生标题栏特权，无绕行方案
		//   - 双击最大化：系统行为随标题栏一同消失，已在 TopBar 自行绑定
		//
		// NOTE: 不设 DisableFramelessWindowDecorations。保留默认的 DWM 客户区
		// 扩展可换来系统级的窗口阴影、圆角与边框，关掉它就得自己画，
		// 且自绘的阴影永远与系统主题对不齐。
		Frameless: true,

		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,

			// 将 WebView2 的用户数据（Cookie / LocalStorage / 缓存 / 崩溃转储）
			// 收拢到本工具的数据目录下。
			//
			// 若不指定，WebView2 会在 %APPDATA%\<exe文件名>\ 下自建目录
			// （连 .exe 后缀一起带上），且一旦 exe 改名，旧目录会永久残留
			// 且无人清理——v1.4 就留下了两坨这样的垃圾。
			//
			// 返回空字符串时 Wails 会退回 WebView2 默认行为，
			// 不理想但不影响运行。
			WebviewUserDataPath: webviewDataDir(),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
