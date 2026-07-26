// deployer.go
//
// 本文件定义「部署」这一行为的抽象接口。
//
// 部署是本工具的核心职责：把清单脚本放到注入器能读到的位置，仅此而已。
// 抽象成接口的意义在于隔离注入器的具体形态——当前实现是 OpenSteamTool，
// 若将来 OST 停更或出现更好的替代品，只需新增一个实现文件，
// app.go 与前端代码无需改动。
//
// 职责边界（架构铁律，不可越界）：
//   - 只写清单脚本本身，不写 config.vdf
//   - 不写注入器自身的配置文件
//   - 不负责安装、更新或修复注入器
//   - 不参与 manifest 下载与 depot 密钥管理（OST 全自动处理）

package main

import "fmt"

// Deployer 定义将清单脚本部署到注入器监控目录的接口。
//
// 实现须保证写入的原子性：注入器通常以文件系统事件驱动重载，
// 若读到半截内容会解析失败。
type Deployer interface {
	// Deploy 将指定游戏的清单脚本写入注入器可读目录。
	//
	// 参数 selectedIDs 是用户勾选的 DLC AppID 列表；未勾选的
	// DLC 不会出现在生成的脚本中。传入空列表表示只注册主游戏。
	//
	// 返回部署后的文件完整路径，供调用方记入历史与日志。
	Deploy(gp *GamePackage, selectedIDs []string) (string, error)

	// Remove 从注入器监控目录中移除指定游戏的清单脚本。
	//
	// 目标文件不存在时应视为成功（幂等），因为用户可能已手动删除。
	Remove(mainAppID string) error

	// DeployDir 返回当前部署目标目录的完整路径。
	//
	// 供前端展示「文件将被放到哪里」，以及日志排障使用。
	DeployDir() string

	// Name 返回该部署器对应的注入器名称，用于日志与界面展示。
	Name() string
}

// ErrEmptyPackage 表示传入的清单包缺少必要信息，无法生成有效脚本。
var ErrEmptyPackage = fmt.Errorf("清单包为空或缺少主游戏 AppID")

// sanitizeFileName 将字符串清洗为可安全用作 Windows 文件名的形式。
//
// 游戏名来自清单脚本的注释，内容不可控——可能含有冒号（如
// "Marvel's Spider-Man: Miles Morales"）、斜杠或其他 Windows
// 保留字符，直接拼进文件名会导致创建失败。
//
// 处理规则：
//   - 保留字符 \ / : * ? " < > | 一律替换为下划线
//   - 控制字符（ASCII < 0x20）同样替换
//   - 首尾空格与点号被裁剪（Windows 不允许以点结尾的文件名）
//   - 清洗后为空时返回 "unknown"，避免生成以下划线开头的怪异文件名
//
// NOTE: 不做长度截断——调用方拼接 AppID 后总长仍远低于
// Windows 的 255 字符上限。
func sanitizeFileName(name string) string {
	const reserved = `\/:*?"<>|`

	runes := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r < 0x20:
			runes = append(runes, '_')
		case containsRune(reserved, r):
			runes = append(runes, '_')
		default:
			runes = append(runes, r)
		}
	}

	cleaned := trimDotsAndSpaces(string(runes))
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

// containsRune 判断字符串中是否包含指定字符。
//
// 用 rune 级比较而非 strings.ContainsRune，是为了让调用点语义更直白，
// 同时避免为一处判断引入 strings 依赖。
func containsRune(s string, target rune) bool {
	for _, r := range s {
		if r == target {
			return true
		}
	}
	return false
}

// trimDotsAndSpaces 裁剪字符串首尾的空格与点号。
//
// Windows 会静默丢弃文件名末尾的点与空格，导致程序以为写入了
// "游戏名 .lua" 而实际文件是 "游戏名.lua"，后续删除时对不上。
func trimDotsAndSpaces(s string) string {
	start, end := 0, len(s)

	for start < end && (s[start] == ' ' || s[start] == '.') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '.') {
		end--
	}

	return s[start:end]
}
