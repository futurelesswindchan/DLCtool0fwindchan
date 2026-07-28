// lua_match.go
//
// 本文件提供对 Lua 清单脚本做文本匹配的工具函数。
//
// 与 lua_parser.go 的分工：
//   - lua_parser.go 用嵌入式 Lua VM 执行脚本，提取完整的结构化数据，
//     用于解析用户导入的清单包
//   - 本文件只做轻量的文本判断，用于「某个 AppID 是否已在脚本中」
//     这类不值得启动 VM 的场景
//
// 之所以不统一用 VM：检测安装状态时可能要对几十个 AppID 逐个判断，
// 为此反复执行脚本代价过高；而正则在这个场景下足够可靠。

package main

import (
	"fmt"
	"regexp"
)

// luaContainsAppID 检查 Lua 脚本中是否存在指定 AppID 的 addappid 调用。
//
// 相比 strings.Contains，本函数能容忍以下格式变体：
//   - addappid 与左括号之间的空格
//   - AppID 前后的空格
//   - 参数后紧跟逗号（三参数形式）或右括号（单参数形式）
//
// 同时会跳过被注释掉的调用——清单包常在文件尾部用注释列出
// 「已排除的 DLC」，若不区分注释就会把它们误判为已安装。
//
// 参数：
//   - content: Lua 脚本的完整文本
//   - appID:   待检查的 AppID
//
// 返回值：
//   - bool: 是否存在有效（未被注释）的匹配调用
//
// XXX: 行首至 addappid 之间不允许出现连字符，这是排除注释行的手段。
// 代价是形如 `x = a-b; addappid(123)` 这样的行会被漏判，
// 但清单脚本中不会出现这种写法，可以接受。
//
// NOTE: 字符类须排除 \n 且量词须非贪婪，理由见 declaredAppIDPattern 的说明。
// 此处仅判断有无匹配，跨行吞噬不影响布尔结果，但不应依赖这一巧合。
func luaContainsAppID(content, appID string) bool {
	pattern := fmt.Sprintf(`(?m)^[^-\n]*?addappid\(\s*%s\s*[,)]`, regexp.QuoteMeta(appID))
	return regexp.MustCompile(pattern).MatchString(content)
}

// luaDeclaredAppIDs 提取脚本中全部被有效注册的 AppID。
//
// 与 luaContainsAppID 的区别在于方向：后者回答「这个 AppID 在不在」，
// 本函数回答「这个文件声明了哪些」。用于扫描注入器目录时无从预知
// 目标 AppID 的场景，如识别未被记录的外部清单文件。
//
// 参数：
//   - content: Lua 脚本的完整文本
//
// 返回值：
//   - []string: 去重后的 AppID 列表，顺序不保证。无匹配时返回空切片
//
// 同样跳过被注释的调用：清单包常在文件尾部用注释列出已排除的 DLC，
// 若不区分便会把它们当作已声明。
//
// NOTE: 返回空切片而非 nil。Wails 会把 nil 切片序列化为 JSON 的 null，
// 前端遍历时报错。
func luaDeclaredAppIDs(content string) []string {
	matches := declaredAppIDPattern.FindAllStringSubmatch(content, -1)

	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		id := m[1]
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// declaredAppIDPattern 捕获未被注释的 addappid 调用中的 AppID。
//
// 在包级预编译而非每次调用时构造：扫描注入器目录需对每个文件执行一次，
// 而 MustCompile 的开销远大于匹配本身。
//
// XXX: 字符类必须同时排除换行符。`[^-]` 在 Go 的正则中会匹配 \n，
// 贪婪量词便跨行吞噬整段文本，使 FindAll 只返回最后一处匹配——表现为
// 「文件明明声明了 20 个 AppID，却只识别出 1 个」。此缺陷由测试捕获。
// 非贪婪量词同样不可省，否则单行内若有多次调用仍会漏判。
//
// XXX: 与 luaContainsAppID 共用「行首至 addappid 之间不含连字符」这一
// 排除注释的手段，局限相同——形如 `x = a-b; addappid(123)` 会被漏判。
// 清单脚本中不存在此类写法，可以接受。
var declaredAppIDPattern = regexp.MustCompile(`(?m)^[^-\n]*?addappid\(\s*(\d+)\s*[,)]`)
