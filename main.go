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
		// 与 wails.json 的 backgroundColour 保持一致，
		// 避免深色界面在首帧渲染前闪出白屏。
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
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
