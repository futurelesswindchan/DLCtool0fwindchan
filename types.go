// types.go
//
// 本文件集中定义前后端共享的数据结构（struct）。
// 这些结构体通过 Wails 框架自动序列化为 JSON 传递给前端，
// 前端的 TypeScript interface 应与此处字段保持一一对应。
//
// 维护须知：
//   - 新增或修改字段后，需同步检查 frontend/src/wailsjs/go/main/models.ts
//     是否自动生成了对应的类型定义。
//   - json tag 使用 camelCase 风格，与前端命名习惯保持一致。

package main

// DepotInfo 表示一个 Steam Depot 的完整信息。
//
// Depot 是 Steam 内容分发系统的基本单元，每个 Depot 对应一组游戏文件。
// 本工具只将密钥与 manifest ID 写入生成的 Lua 脚本，由注入器接管后续流程——
// 既不写 config.vdf，也不向 depotcache 复制 manifest 文件。
//
// 字段说明：
//   - DepotID:       Depot 的唯一数字标识符（如 "1234567"）
//   - DecryptionKey: 用于解密 Depot 内容的十六进制密钥字符串
//   - ManifestID:    当前版本的 manifest 标识符，须与密钥成对写出
//   - FileSize:      内容大小（字节）
//
// XXX: FileSize 的语义随解析路径而异。Lua 路径下取自 setManifestid 的第三参数，
// 即 depot 内容总大小；MAU 路径下无此信息，只能退而取 manifest 文件自身大小，
// 二者相差几个数量级。故界面不得将此字段当作「下载体积」展示。
type DepotInfo struct {
	DepotID       string `json:"depotID"`
	DecryptionKey string `json:"decryptionKey"`
	ManifestID    string `json:"manifestID"`
	FileSize      int64  `json:"fileSize"`
}

// DLCInfo 表示一个 DLC（可下载内容）的信息。
//
// DLC 通过 addappid 注册到生成的清单脚本中。自带独立 Depot 的 DLC
// （HasKey=true，如 ARK 的各地图）需写出三行：注册 App 身份、注册 Depot
// 密钥、绑定 manifest；无独立 Depot 者内容随本体下载，单行注册即可。
// 详见 deployer_ost.go 的 formatDLCLine。
//
// 字段说明：
//   - AppID:         DLC 的唯一数字标识符
//   - Name:          DLC 的显示名称（从 Lua 注释中提取，可能为空）
//   - HasKey:        是否携带解密密钥
//   - DecryptionKey: 解密密钥（仅当 HasKey=true 时有效）
//   - IsInstalled:   当前系统中是否已安装该 DLC（由检测逻辑填充）
//   - ManifestID:    该 DLC 自带 Depot 的 manifest 标识符。带独立 Depot 的
//     DLC（如 ARK 的各地图）必须连同 setManifestid 一起写出，
//     否则 OST 无法确定下载哪个版本
//   - FileSize:      manifest 声明的内容大小（字节）
type DLCInfo struct {
	AppID         string `json:"appID"`
	Name          string `json:"name"`
	HasKey        bool   `json:"hasKey"`
	DecryptionKey string `json:"decryptionKey"`
	IsInstalled   bool   `json:"isInstalled"`
	ManifestID    string `json:"manifestID"`
	FileSize      int64  `json:"fileSize"`
}

// GamePackage 表示从 Lua 压缩包中解析出的完整游戏数据包。
//
// 一个 GamePackage 对应用户上传的一个 zip 文件，包含：
//   - 主游戏的 AppID 和名称
//   - 所有关联的 Depot 信息（含解密密钥和 manifest）
//   - 所有可安装的 DLC 列表
//   - 原始 Lua 文件内容（用于调试和回溯）
//   - 解压后的 manifest 文件路径列表
//
// 字段说明：
//   - MainAppID:     主游戏的 AppID（Lua 文件中第一个 addappid 调用的参数）
//   - MainKey:       主游戏自身的解密密钥。必须原样透传到生成的脚本中——
//     早期版本丢失此字段，导致 OST 无法解密主 App，
//     已安装本体的游戏会让 Steam 在校验时崩溃
//   - GameName:      游戏名称（从 Lua 注释中启发式提取）
//   - Depots:        所有有效 Depot 的列表（必须同时具备密钥和 manifest）
//   - DLCs:          所有可安装 DLC 的列表
//   - LuaContent:    原始 Lua 文件的完整文本内容
//   - ManifestFiles: 解压后 manifest 文件的本地临时路径列表
//   - Source:        清单的来源，取 RepoSource.Name 或 SourceLocalImport。
//     由产出该清单包的路径负责填充，部署时透传至 GameRecord
type GamePackage struct {
	MainAppID     string      `json:"mainAppID"`
	MainKey       string      `json:"mainKey"`
	GameName      string      `json:"gameName"`
	Depots        []DepotInfo `json:"depots"`
	DLCs          []DLCInfo   `json:"dlcs"`
	LuaContent    string      `json:"luaContent"`
	ManifestFiles []string    `json:"manifestFiles"`
	Source        string      `json:"source"`
}

// DeployedEntry 表示注入器目录中的一个清单文件及其归属判定。
//
// 由 App.ScanDeployed 产出，用于让界面反映磁盘的真实状态而非本工具的
// 记忆。注入器会加载目录内全部 .lua 的并集，故未被记录的文件同样在生效。
//
// 字段说明：
//   - FileName:   文件名（不含目录）
//   - MainAppID:  主游戏 AppID。取自文件名，取不到时退用内容中首个声明
//   - AppIDs:     该文件声明的全部 AppID，含主游戏与各 DLC
//   - IsExternal: 是否为外部文件，即不符合本工具命名格式者
//   - InHistory:  该主游戏是否存在于安装历史中
//
// IsExternal 与 InHistory 并非互补，四种组合各有含义：
//
//	!IsExternal && InHistory   常态，本工具部署且有记录
//	!IsExternal && !InHistory  历史丢失或被清空，文件仍在
//	IsExternal && !InHistory   典型的外部清单，用户手动放置或他工具产生
//	IsExternal && InHistory    同一游戏被两处声明，卸载将不彻底
//
// NOTE: 界面对 IsExternal 为真的条目不应提供 DLC 勾选功能——本工具没有
// 对应的 packages 数据，只能还原 AppID 集合而无法得知可选项全貌。
type DeployedEntry struct {
	FileName   string   `json:"fileName"`
	MainAppID  string   `json:"mainAppID"`
	AppIDs     []string `json:"appIDs"`
	IsExternal bool     `json:"isExternal"`
	InHistory  bool     `json:"inHistory"`
}

// OperationResult 表示一次安装或卸载操作的执行结果。
//
// 该结构体用于向前端返回操作状态，前端根据 Success 字段
// 决定展示成功提示还是错误信息。
//
// 字段说明：
//   - Success: 操作是否成功完成
//   - Message: 面向用户的结果描述文本（成功时为提示，失败时为错误原因）
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
