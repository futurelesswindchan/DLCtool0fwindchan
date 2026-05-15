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
