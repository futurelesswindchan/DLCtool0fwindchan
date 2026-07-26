// constants.go
//
// 本文件集中定义项目中所有硬编码的路径常量、目录名称和配置前缀。
// 目的是消除散落在各业务函数中的魔法字符串，使后续路径规则变更时
// 只需修改此处即可全局生效，避免遗漏。
//
// 命名规范：
//   - 目录类常量以 Dir 结尾（如 DepotcacheDir）
//   - 文件名类常量以 File 结尾（如 ConfigVDFFile）
//   - 路径片段使用正斜杠，运行时通过 filepath.Join 拼接为系统路径

package main

const (
	// ConfigDir 是 Steam 安装目录下存放配置文件的子目录名称。
	// 完整路径示例：<SteamPath>/config/
	ConfigDir = "config"

	// ConfigVDFFile 是 Steam 的主配置文件名。
	// 该文件使用 Valve Data Format (VDF) 格式，存储 depot 解密密钥等信息。
	// 完整路径示例：<SteamPath>/config/config.vdf
	ConfigVDFFile = "config.vdf"

	// SteamtoolsPluginDir 是 Steamtools 插件的子目录名称（位于 config 下）。
	// 完整路径示例：<SteamPath>/config/stplug-in/
	SteamtoolsPluginDir = "stplug-in"

	// SteamtoolsLuaFile 是 Steamtools 的 Lua 脚本文件名。
	// 该文件包含 addappid() 调用，用于注册 DLC 的 AppID。
	// 完整路径示例：<SteamPath>/config/stplug-in/Steamtools.lua
	SteamtoolsLuaFile = "Steamtools.lua"

	// DepotcacheDir 是 Steam 存放 depot manifest 缓存文件的目录名称。
	// manifest 文件命名格式为 <DepotID>_<ManifestID>.manifest
	// 完整路径示例：<SteamPath>/depotcache/
	DepotcacheDir = "depotcache"

	// TempDirPrefix 是本工具创建临时解压目录时使用的前缀。
	// 用于 os.MkdirTemp("", TempDirPrefix) 调用。
	TempDirPrefix = "dlctool_"

	// BackupSuffix 是配置文件备份时追加的后缀。
	// 示例：config.vdf -> config.vdf.bak
	BackupSuffix = ".bak"

	// BackupRemoveSuffix 是卸载操作备份时追加的后缀。
	// 用于区分安装备份和卸载备份。
	// 示例：config.vdf -> config.vdf.bak.remove
	BackupRemoveSuffix = ".bak.remove"

	// SteamRegistryKey 是 Windows 注册表中 Steam 安装信息的键路径。
	// 位于 HKEY_CURRENT_USER 下。
	SteamRegistryKey = `Software\Valve\Steam`

	// SteamRegistryValueName 是注册表中存储 Steam 安装路径的值名称。
	SteamRegistryValueName = "SteamPath"

	// SteamProcessName 是 Steam 主进程的可执行文件名，用于 taskkill 操作。
	SteamProcessName = "steam.exe"

	// KillSteamWaitDuration 是关闭 Steam 后等待进程完全退出的时间（秒）。
	KillSteamWaitDuration = 2
)

// ============================================================
// v2.0 新增常量
// ============================================================
//
// v2.0 架构下，本工具作为「清单包管理盒子」，只负责把 .lua 文件
// 放到注入器（OpenSteamTool）监控的目录，不再触碰 config.vdf 与 depotcache。
// 下列常量服务于配置持久化、部署器与环境检测三个新模块。

const (
	// AppDataDirName 是本工具在用户主目录下创建的数据目录名。
	// 完整路径示例：C:\Users\<用户名>\.kazeusa\
	AppDataDirName = ".kazeusa"

	// ConfigFileName 是用户配置文件名，存放于 AppDataDirName 下。
	// 完整路径示例：<用户主目录>/.kazeusa/config.json
	ConfigFileName = "config.json"

	// HistoryFileName 是安装历史记录文件名，存放于 AppDataDirName 下。
	// 完整路径示例：<用户主目录>/.kazeusa/history.json
	HistoryFileName = "history.json"

	// LogDirName 是日志文件所在的子目录名，位于 AppDataDirName 下。
	// 完整路径示例：<用户主目录>/.kazeusa/logs/
	LogDirName = "logs"

	// TempFileSuffix 是原子写入过程中使用的临时文件后缀。
	//
	// 所有落盘操作均遵循「先写 .tmp 再 rename」的策略：
	// OST 的 LuaFileWatcher 基于 ReadDirectoryChangesW 事件驱动，
	// 若直接写入目标文件，可能在内容尚未写完时就被读取。
	// 通过 rename 提交可保证 OST 只收到一次事件且拿到完整内容。
	TempFileSuffix = ".tmp"

	// OSTLuaDirName 是 OpenSteamTool 默认监控的 Lua 脚本目录名（位于 config 下）。
	// 完整路径示例：<SteamPath>/config/lua/
	//
	// NOTE: OST 允许通过自身的 toml 配置追加额外监控目录，
	// 但本工具只使用默认目录，不读取也不修改 OST 的配置文件。
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
// 那些属于注入器的内部事务，超出本工具职责边界。
var ostRequiredDLLs = []string{
	"dwmapi.dll",
	"xinput1_4.dll",
	"OpenSteamTool.dll",
}
