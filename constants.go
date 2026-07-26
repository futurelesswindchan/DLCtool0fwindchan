// constants.go
//
// 本文件集中定义项目中所有硬编码的路径常量与目录名称，
// 目的是消除散落在业务代码中的魔法字符串——路径规则变更时
// 只需改动此处即可全局生效。
//
// 命名规范：
//   - 目录类常量以 Dir 或 DirName 结尾
//   - 文件名类常量以 File 或 FileName 结尾
//   - 路径片段不含分隔符，运行时统一由 filepath.Join 拼接

package main

// ============================================================
// Steam 相关
// ============================================================

const (
	// ConfigDir 是 Steam 安装目录下存放配置文件的子目录名。
	// 完整路径示例：<SteamPath>/config/
	ConfigDir = "config"

	// SteamRegistryKey 是 Windows 注册表中 Steam 安装信息的键路径，
	// 位于 HKEY_CURRENT_USER 下。
	SteamRegistryKey = `Software\Valve\Steam`

	// SteamRegistryValueName 是注册表中存储 Steam 安装路径的值名称。
	SteamRegistryValueName = "SteamPath"
)

// ============================================================
// 本工具的数据目录
// ============================================================
//
// 落盘位置约定：本工具产生的一切文件都必须位于 AppDataDirName 之下。
// 唯一的外部写入是部署到 <Steam>/config/lua/ 的清单脚本，
// 以及 %TEMP% 下用完即删的解压临时目录。
// 验收标准：卸载 = 删 exe + 删 ~/.kazeusa 一个文件夹。

const (
	// AppDataDirName 是本工具在用户主目录下创建的数据目录名。
	// 完整路径示例：C:\Users\<用户名>\.kazeusa\
	AppDataDirName = ".kazeusa"

	// ConfigFileName 是用户配置文件名，位于 AppDataDirName 下。
	ConfigFileName = "config.json"

	// HistoryFileName 是安装历史记录文件名，位于 AppDataDirName 下。
	HistoryFileName = "history.json"

	// LogDirName 是日志文件所在的子目录名，位于 AppDataDirName 下。
	LogDirName = "logs"

	// WebviewDirName 是 WebView2 用户数据的子目录名，位于 AppDataDirName 下。
	//
	// 显式指定此目录是为了避免 WebView2 在 %APPDATA%\<exe文件名>\ 下
	// 自建目录——那样一旦 exe 改名，旧目录便永久残留且无人清理。
	WebviewDirName = "webview2"

	// TempDirPrefix 是创建临时解压目录时使用的前缀，
	// 用于 os.MkdirTemp("", TempDirPrefix)。
	//
	// NOTE: 启动时的残留清理依赖此前缀来识别自己的产物，
	// 修改它会导致旧版本留下的临时目录再也不会被回收。
	TempDirPrefix = "dlctool_"

	// TempFileSuffix 是原子写入过程中使用的临时文件后缀。
	//
	// 所有落盘操作均遵循「先写 .tmp 再 rename」的策略：
	// OST 的 LuaFileWatcher 基于文件系统事件驱动，若直接写入目标文件，
	// 可能在内容尚未写完时就被读取。经 rename 提交可保证注入器
	// 只收到一次事件，且此刻内容已完整。
	TempFileSuffix = ".tmp"
)

// ============================================================
// 注入器（OpenSteamTool）相关
// ============================================================

const (
	// OSTLuaDirName 是 OpenSteamTool 默认监控的 Lua 脚本目录名，位于 config 下。
	// 完整路径示例：<SteamPath>/config/lua/
	//
	// NOTE: OST 允许通过自身的 toml 配置追加额外监控目录，
	// 但本工具只使用默认目录——读取注入器的配置文件属于越界行为。
	OSTLuaDirName = "lua"

	// LuaFileExt 是清单脚本文件的扩展名。
	LuaFileExt = ".lua"
)

// ostRequiredDLLs 列出判定 OpenSteamTool 环境就绪所需的文件。
//
// 这三个文件均位于 Steam 根目录下，构成 OST 的 DLL 劫持加载链：
// dwmapi.dll 与 xinput1_4.dll 是两个代理入口，二者任一被 Steam 加载后
// 都会转而载入核心的 OpenSteamTool.dll。
//
// NOTE: 仅检测文件是否存在，不校验版本、不检查 OST 自身的 toml 配置——
// 那些属于注入器的内部事务，超出本工具的职责边界。
var ostRequiredDLLs = []string{
	"dwmapi.dll",
	"xinput1_4.dll",
	"OpenSteamTool.dll",
}
